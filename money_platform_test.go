package nombaone

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestPaymentMethods_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("setup", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.PaymentMethods.Setup(ctx, PaymentMethodSetupParams{CustomerRef: "nbo1cus", AmountInKobo: 5000, CallbackURL: "https://x.co"})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodPost, "/v1/payment-methods/setup")
		body := decodeBody(t, call.Body)
		if body["customerRef"] != "nbo1cus" || body["amountInKobo"] != float64(5000) {
			t.Errorf("body = %v", body)
		}
	})

	t.Run("createVirtualAccount", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.PaymentMethods.CreateVirtualAccount(ctx, PaymentMethodVirtualAccountParams{CustomerRef: "nbo1cus"})
		wantMethodPath(t, lastCall(t, m), http.MethodPost, "/v1/payment-methods/virtual-account")
	})

	t.Run("retrieve", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.PaymentMethods.Retrieve(ctx, "nbo1pmt")
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/payment-methods/nbo1pmt")
	})

	t.Run("list uses customerRef filter name", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.PaymentMethods.List(ctx, PaymentMethodListParams{CustomerRef: String("nbo1cus")})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodGet, "/v1/payment-methods")
		q, _ := url.ParseQuery(call.Query)
		if q.Get("customerRef") != "nbo1cus" {
			t.Errorf("query = %v (must use customerRef, not customerId)", q)
		}
	})

	t.Run("setDefault and remove", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.PaymentMethods.SetDefault(ctx, "nbo1pmt")
		wantMethodPath(t, lastCall(t, m), http.MethodPost, "/v1/payment-methods/nbo1pmt/default")

		c2, m2 := okClient(t)
		_, _ = c2.PaymentMethods.Remove(ctx, "nbo1pmt")
		wantMethodPath(t, lastCall(t, m2), http.MethodDelete, "/v1/payment-methods/nbo1pmt")
	})
}

func TestMandates_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("create", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Mandates.Create(ctx, MandateCreateParams{
			CustomerRef: "nbo1cus", CustomerAccountNumber: "0123456789", BankCode: "058",
			CustomerName: "Ada", CustomerAccountName: "Ada", CustomerPhoneNumber: "+234",
			CustomerAddress: "Lagos", Narration: "sub", MaxAmountInKobo: 500_000,
		})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodPost, "/v1/mandates")
		if !uuidRE.MatchString(call.Header.Get("Idempotency-Key")) {
			t.Error("mandate create must send an Idempotency-Key")
		}
	})

	t.Run("retrieve returns a PaymentMethod (the quirk)", func(t *testing.T) {
		pmJSON := `{"domain":"payment_method","id":"nbo1pmt","customerId":"nbo1cus","kind":"mandate","status":"consent_pending","isDefault":false,"brand":null,"last4":null,"expMonth":null,"expYear":null,"mode":"sandbox","createdAt":"2026-07-04T00:00:00.000Z","updatedAt":"2026-07-04T00:00:00.000Z"}`
		m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(pmJSON)})
		c := testClient(t, m)
		pm, err := c.Mandates.Retrieve(ctx, "nbo1pmt")
		if err != nil {
			t.Fatal(err)
		}
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/mandates/nbo1pmt")
		if pm.Kind != PaymentMethodKindMandate || pm.Status != PaymentMethodStatusConsentPending {
			t.Errorf("payment method = %+v", pm)
		}
	})
}

func TestSettlements_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieve", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Settlements.Retrieve(ctx, "nbo1stl")
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/settlements/nbo1stl")
	})

	t.Run("list with status filter", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Settlements.List(ctx, SettlementListParams{Status: SettlementStatusSettled})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodGet, "/v1/settlements")
		q, _ := url.ParseQuery(call.Query)
		if q.Get("status") != "settled" {
			t.Errorf("query = %v", q)
		}
	})

	t.Run("escrow uses the literal /settlements/escrow path", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Settlements.RetrieveEscrow(ctx)
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/settlements/escrow")
	})

	t.Run("refund", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Settlements.Refund(ctx, "nbo1stl", SettlementRefundParams{AmountInKobo: Int64(100_000)})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodPost, "/v1/settlements/nbo1stl/refund")
		if decodeBody(t, call.Body)["amountInKobo"] != float64(100000) {
			t.Errorf("body = %s", call.Body)
		}
	})

	t.Run("createPayout uses the literal /settlements/payout path", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Settlements.CreatePayout(ctx, PayoutCreateParams{AmountInKobo: 5_000_000, BankCode: "058", AccountNumber: "0123456789"}, WithIdempotencyKey("payout-1"))
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodPost, "/v1/settlements/payout")
		if call.Header.Get("Idempotency-Key") != "payout-1" {
			t.Errorf("payout must honor the explicit idempotency key (merchantTxRef)")
		}
	})
}

func TestWebhookEndpoints_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("create returns the signing secret once", func(t *testing.T) {
		epJSON := `{"domain":"webhook","id":"nbo1whk","url":"https://x.co/h","enabledEvents":["*"],"signingSecretPrefix":"nbo_whsec_ab","disabledAt":null,"createdAt":"2026-07-04T00:00:00.000Z","signingSecret":"nbo_whsec_abcdef0123456789"}`
		m := newMock(scriptedResponse{status: http.StatusCreated, body: okEnvelope(epJSON)})
		c := testClient(t, m)
		ep, err := c.WebhookEndpoints.Create(ctx, WebhookEndpointCreateParams{URL: "https://x.co/h"})
		if err != nil {
			t.Fatal(err)
		}
		wantMethodPath(t, lastCall(t, m), http.MethodPost, "/v1/webhooks")
		if ep.SigningSecret != "nbo_whsec_abcdef0123456789" {
			t.Errorf("SigningSecret = %q", ep.SigningSecret)
		}
		if ep.ID != "nbo1whk" || len(ep.EnabledEvents) != 1 { // embedded fields
			t.Errorf("embedded endpoint = %+v", ep.WebhookEndpoint)
		}
	})

	t.Run("crud + rotate", func(t *testing.T) {
		cases := []struct {
			name   string
			method string
			path   string
			call   func(*Client)
		}{
			{"retrieve", http.MethodGet, "/v1/webhooks/nbo1whk", func(c *Client) { _, _ = c.WebhookEndpoints.Retrieve(ctx, "nbo1whk") }},
			{"update", http.MethodPatch, "/v1/webhooks/nbo1whk", func(c *Client) {
				_, _ = c.WebhookEndpoints.Update(ctx, "nbo1whk", WebhookEndpointUpdateParams{Disabled: Bool(true)})
			}},
			{"list", http.MethodGet, "/v1/webhooks", func(c *Client) { _, _ = c.WebhookEndpoints.List(ctx) }},
			{"delete", http.MethodDelete, "/v1/webhooks/nbo1whk", func(c *Client) { _, _ = c.WebhookEndpoints.Delete(ctx, "nbo1whk") }},
			{"rotateSecret", http.MethodPost, "/v1/webhooks/nbo1whk/rotate-secret", func(c *Client) { _, _ = c.WebhookEndpoints.RotateSecret(ctx, "nbo1whk") }},
			{"deliveries.retrieve", http.MethodGet, "/v1/webhooks/nbo1whk/deliveries/nbo1whd", func(c *Client) { _, _ = c.WebhookEndpoints.Deliveries.Retrieve(ctx, "nbo1whk", "nbo1whd") }},
			{"deliveries.replay", http.MethodPost, "/v1/webhooks/nbo1whk/deliveries/nbo1whd/replay", func(c *Client) { _, _ = c.WebhookEndpoints.Deliveries.Replay(ctx, "nbo1whk", "nbo1whd") }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				c, m := okClient(t)
				tc.call(c)
				wantMethodPath(t, lastCall(t, m), tc.method, tc.path)
			})
		}
	})

	t.Run("deliveries list with status filter", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.WebhookEndpoints.Deliveries.List(ctx, "nbo1whk", WebhookDeliveryListParams{Status: WebhookDeliveryStatusDead})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodGet, "/v1/webhooks/nbo1whk/deliveries")
		q, _ := url.ParseQuery(call.Query)
		if q.Get("status") != "dead" {
			t.Errorf("query = %v", q)
		}
	})
}

func TestEvents_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("list with type filter", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Events.List(ctx, EventListParams{Type: String("invoice.paid")})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodGet, "/v1/events")
		q, _ := url.ParseQuery(call.Query)
		if q.Get("type") != "invoice.paid" {
			t.Errorf("query = %v", q)
		}
	})

	t.Run("retrieve", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Events.Retrieve(ctx, "nbo1evt")
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/events/nbo1evt")
	})

	t.Run("catalog returns a keyed map", func(t *testing.T) {
		catalogJSON := `{"invoice.paid":{"when":"an invoice is fully collected","payload":["reference"]},"customer.created":{"when":"a customer is created","payload":["reference"]}}`
		m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(catalogJSON)})
		c := testClient(t, m)
		catalog, err := c.Events.Catalog(ctx)
		if err != nil {
			t.Fatal(err)
		}
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/events/catalog")
		if entry, ok := catalog["invoice.paid"]; !ok || entry.When == "" || len(entry.Payload) != 1 {
			t.Errorf("catalog = %+v", catalog)
		}
	})
}

func TestOrganizationAndMetrics_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("organization retrieve/update use PUT", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Organization.Retrieve(ctx)
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/organization")

		c2, m2 := okClient(t)
		_, _ = c2.Organization.Update(ctx, TenantSettingsUpdateParams{SettlementMode: SettlementModeSplitAtCollection})
		call := lastCall(t, m2)
		wantMethodPath(t, call, http.MethodPut, "/v1/organization")
		if decodeBody(t, call.Body)["settlementMode"] != "split_at_collection" {
			t.Errorf("body = %s", call.Body)
		}
	})

	t.Run("organization billing retrieve/update use PUT", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Organization.Billing.Retrieve(ctx)
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/organization/billing")

		c2, m2 := okClient(t)
		_, _ = c2.Organization.Billing.Update(ctx, BillingSettingsUpdateParams{PaydayBiasEnabled: Bool(true), PaydayDays: []int{25, 28, 30}})
		call := lastCall(t, m2)
		wantMethodPath(t, call, http.MethodPut, "/v1/organization/billing")
		if decodeBody(t, call.Body)["paydayBiasEnabled"] != true {
			t.Errorf("body = %s", call.Body)
		}
	})

	t.Run("metrics billing with window", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Metrics.Billing(ctx, BillingMetricsParams{From: String("2026-07-01T00:00:00Z"), To: String("2026-07-31T00:00:00Z")})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodGet, "/v1/metrics/billing")
		q, _ := url.ParseQuery(call.Query)
		if q.Get("from") == "" || q.Get("to") == "" {
			t.Errorf("query = %v", q)
		}
	})
}

func TestSandbox_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("methods hit the sandbox paths", func(t *testing.T) {
		cases := []struct {
			name   string
			method string
			path   string
			call   func(*Client)
		}{
			{"createPaymentMethod", http.MethodPost, "/v1/sandbox/payment-methods", func(c *Client) {
				_, _ = c.Sandbox.CreatePaymentMethod(ctx, SandboxPaymentMethodParams{CustomerID: "nbo1cus"})
			}},
			{"advanceCycle", http.MethodPost, "/v1/sandbox/subscriptions/nbo1sub/advance-cycle", func(c *Client) { _, _ = c.Sandbox.AdvanceCycle(ctx, "nbo1sub") }},
			{"simulateWebhook", http.MethodPost, "/v1/sandbox/webhooks/simulate", func(c *Client) {
				_, _ = c.Sandbox.SimulateWebhook(ctx, SandboxSimulateWebhookParams{Type: "invoice.paid"})
			}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				c, m := okClient(t)
				tc.call(c)
				wantMethodPath(t, lastCall(t, m), tc.method, tc.path)
			})
		}
	})

	t.Run("live key fails locally before any network call", func(t *testing.T) {
		m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")})
		c, err := New(WithAPIKey("nbo_live_key"), WithBaseURL("http://api.test"), WithHTTPClient(m))
		if err != nil {
			t.Fatal(err)
		}

		_, err = c.Sandbox.CreatePaymentMethod(ctx, SandboxPaymentMethodParams{CustomerID: "nbo1cus"})
		if !errors.Is(err, ErrSandboxRequiresSandboxKey) {
			t.Fatalf("err = %v, want ErrSandboxRequiresSandboxKey", err)
		}
		if m.callCount() != 0 {
			t.Errorf("callCount = %d, want 0 (guard must fire before any network call)", m.callCount())
		}
	})
}
