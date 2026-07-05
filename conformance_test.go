package nombaone

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"
)

// The drift alarm. Every SDK method is exercised against a recording transport;
// each emitted METHOD /v1/path must exist in the committed OpenAPI snapshot
// (spec/openapi.json), and every spec operation (minus the explicit
// exclusions) must be emitted by some SDK method. Either direction failing
// names the route.

type specOp struct {
	method   string
	segments []string
	key      string
}

func loadSpecOps(t *testing.T) []specOp {
	t.Helper()
	raw, err := os.ReadFile("spec/openapi.json")
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse spec: %v", err)
	}
	httpMethods := map[string]bool{"get": true, "post": true, "patch": true, "put": true, "delete": true}
	var ops []specOp
	for path, item := range doc.Paths {
		for method := range item {
			if !httpMethods[method] {
				continue
			}
			ops = append(ops, specOp{
				method:   method,
				segments: splitPath(path),
				key:      method + " " + path,
			})
		}
	}
	return ops
}

func splitPath(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// matchSpecOp finds the most-specific structural match: {param} matches any
// segment; literal matches win ties (so /settlements/escrow beats
// /settlements/{id}).
func matchSpecOp(ops []specOp, method, urlPath string) *specOp {
	segments := splitPath(urlPath)
	var best *specOp
	bestLiterals := -1
	for i := range ops {
		op := &ops[i]
		if op.method != method || len(op.segments) != len(segments) {
			continue
		}
		literals := 0
		ok := true
		for j, specSeg := range op.segments {
			if strings.HasPrefix(specSeg, "{") {
				continue
			}
			if specSeg != segments[j] {
				ok = false
				break
			}
			literals++
		}
		if ok && literals > bestLiterals {
			best = op
			bestLiterals = literals
		}
	}
	return best
}

const conformanceEnvelope = `{"success":true,"statusCode":200,"data":null,"pagination":{"limit":20,"hasMore":false,"nextCursor":null},"meta":{"requestId":"req_conf"}}`

// The complete public surface — one exercise per SDK method.
func conformanceExercises() []func(context.Context, *Client) {
	const (
		id       = "nbo000000000001xxx"
		grant    = "nbo000000000002crg"
		delivery = "nbo000000000003whd"
	)
	return []func(context.Context, *Client){
		// customers
		func(ctx context.Context, c *Client) {
			_, _ = c.Customers.Create(ctx, CustomerCreateParams{Email: "a@b.co", Name: "A"})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Customers.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Customers.Update(ctx, id, CustomerUpdateParams{Name: String("B")})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Customers.List(ctx, CustomerListParams{}) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Customers.ApplyDiscount(ctx, id, CustomerApplyDiscountParams{Coupon: "X"})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Customers.RemoveDiscount(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Customers.GrantCredit(ctx, id, CustomerGrantCreditParams{AmountInKobo: 100})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Customers.RetrieveCreditBalance(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.Customers.VoidCredit(ctx, id, grant) },
		// plans (+ nested prices)
		func(ctx context.Context, c *Client) { _, _ = c.Plans.Create(ctx, PlanCreateParams{Name: "Pro"}) },
		func(ctx context.Context, c *Client) { _, _ = c.Plans.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Plans.Update(ctx, id, PlanUpdateParams{Name: String("Pro2")})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Plans.List(ctx, PlanListParams{}) },
		func(ctx context.Context, c *Client) { _, _ = c.Plans.Archive(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Plans.Prices.Create(ctx, id, PriceCreateParams{UnitAmountInKobo: 100, Interval: PriceIntervalMonth})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Plans.Prices.List(ctx, id, PlanPriceListParams{}) },
		// prices
		func(ctx context.Context, c *Client) { _, _ = c.Prices.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.Prices.List(ctx, PriceListParams{}) },
		func(ctx context.Context, c *Client) { _, _ = c.Prices.Deactivate(ctx, id) },
		// subscriptions
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.Create(ctx, SubscriptionCreateParams{CustomerID: id, PriceID: id, PaymentMethodID: String(id)})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Subscriptions.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.Update(ctx, id, SubscriptionUpdateParams{Metadata: Metadata{}})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Subscriptions.List(ctx, SubscriptionListParams{}) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.ListEvents(ctx, id, SubscriptionListEventsParams{})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Subscriptions.Pause(ctx, id, SubscriptionPauseParams{}) },
		func(ctx context.Context, c *Client) { _, _ = c.Subscriptions.Resume(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.Cancel(ctx, id, SubscriptionCancelParams{})
		},
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.Resubscribe(ctx, id, SubscriptionResubscribeParams{})
		},
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.Change(ctx, id, SubscriptionChangeParams{PriceID: String(id)})
		},
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.UpdatePaymentMethod(ctx, id, SubscriptionUpdatePaymentMethodParams{CheckoutToken: String("t")})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Subscriptions.RetrieveUpcomingInvoice(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.ApplyDiscount(ctx, id, SubscriptionApplyDiscountParams{Coupon: "X"})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Subscriptions.RemoveDiscount(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.Schedule.Create(ctx, id, SubscriptionScheduleCreateParams{PriceID: id})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Subscriptions.Schedule.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.Subscriptions.Schedule.Release(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.Subscriptions.Dunning.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Subscriptions.Dunning.ListAttempts(ctx, id, DunningListAttemptsParams{})
		},
		// invoices
		func(ctx context.Context, c *Client) { _, _ = c.Invoices.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.Invoices.List(ctx, InvoiceListParams{}) },
		func(ctx context.Context, c *Client) { _, _ = c.Invoices.Void(ctx, id, InvoiceVoidParams{}) },
		// coupons
		func(ctx context.Context, c *Client) {
			_, _ = c.Coupons.Create(ctx, CouponCreateParams{Code: "X", PercentOff: Int(10), Duration: CouponDurationOnce})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Coupons.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Coupons.Update(ctx, id, CouponUpdateParams{MaxRedemptions: Int(5)})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Coupons.List(ctx, CouponListParams{}) },
		// payment methods
		func(ctx context.Context, c *Client) {
			_, _ = c.PaymentMethods.Setup(ctx, PaymentMethodSetupParams{CustomerRef: id, AmountInKobo: 100, CallbackURL: "https://x.co"})
		},
		func(ctx context.Context, c *Client) {
			_, _ = c.PaymentMethods.CreateVirtualAccount(ctx, PaymentMethodVirtualAccountParams{CustomerRef: id})
		},
		func(ctx context.Context, c *Client) { _, _ = c.PaymentMethods.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.PaymentMethods.List(ctx, PaymentMethodListParams{}) },
		func(ctx context.Context, c *Client) { _, _ = c.PaymentMethods.SetDefault(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.PaymentMethods.Remove(ctx, id) },
		// mandates
		func(ctx context.Context, c *Client) {
			_, _ = c.Mandates.Create(ctx, MandateCreateParams{
				CustomerRef: id, CustomerAccountNumber: "0123456789", BankCode: "058",
				CustomerName: "A", CustomerAccountName: "A", CustomerPhoneNumber: "+234",
				CustomerAddress: "Lagos", Narration: "sub", MaxAmountInKobo: 100,
			})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Mandates.Retrieve(ctx, id) },
		// settlements
		func(ctx context.Context, c *Client) { _, _ = c.Settlements.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.Settlements.List(ctx, SettlementListParams{}) },
		func(ctx context.Context, c *Client) { _, _ = c.Settlements.RetrieveEscrow(ctx) },
		func(ctx context.Context, c *Client) { _, _ = c.Settlements.Refund(ctx, id, SettlementRefundParams{}) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Settlements.CreatePayout(ctx, PayoutCreateParams{AmountInKobo: 100, BankCode: "058", AccountNumber: "01"})
		},
		// webhook endpoints (+ deliveries)
		func(ctx context.Context, c *Client) {
			_, _ = c.WebhookEndpoints.Create(ctx, WebhookEndpointCreateParams{URL: "https://x.co/h"})
		},
		func(ctx context.Context, c *Client) { _, _ = c.WebhookEndpoints.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.WebhookEndpoints.Update(ctx, id, WebhookEndpointUpdateParams{Disabled: Bool(true)})
		},
		func(ctx context.Context, c *Client) { _, _ = c.WebhookEndpoints.List(ctx) },
		func(ctx context.Context, c *Client) { _, _ = c.WebhookEndpoints.Delete(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.WebhookEndpoints.RotateSecret(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.WebhookEndpoints.Deliveries.List(ctx, id, WebhookDeliveryListParams{})
		},
		func(ctx context.Context, c *Client) { _, _ = c.WebhookEndpoints.Deliveries.Retrieve(ctx, id, delivery) },
		func(ctx context.Context, c *Client) { _, _ = c.WebhookEndpoints.Deliveries.Replay(ctx, id, delivery) },
		// events
		func(ctx context.Context, c *Client) { _, _ = c.Events.List(ctx, EventListParams{}) },
		func(ctx context.Context, c *Client) { _, _ = c.Events.Retrieve(ctx, id) },
		func(ctx context.Context, c *Client) { _, _ = c.Events.Catalog(ctx) },
		// organization (+ billing)
		func(ctx context.Context, c *Client) { _, _ = c.Organization.Retrieve(ctx) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Organization.Update(ctx, TenantSettingsUpdateParams{SettlementMode: SettlementModeSplitAtCollection})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Organization.Billing.Retrieve(ctx) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Organization.Billing.Update(ctx, BillingSettingsUpdateParams{CommsEnabled: Bool(true)})
		},
		// metrics
		func(ctx context.Context, c *Client) { _, _ = c.Metrics.Billing(ctx, BillingMetricsParams{}) },
		// sandbox
		func(ctx context.Context, c *Client) {
			_, _ = c.Sandbox.CreatePaymentMethod(ctx, SandboxPaymentMethodParams{CustomerID: id})
		},
		func(ctx context.Context, c *Client) { _, _ = c.Sandbox.AdvanceCycle(ctx, id) },
		func(ctx context.Context, c *Client) {
			_, _ = c.Sandbox.SimulateWebhook(ctx, SandboxSimulateWebhookParams{Type: "invoice.paid"})
		},
	}
}

// excludedOps are routes intentionally not in the SDK surface.
func excludedOps() map[string]bool {
	return map[string]bool{
		"get /v1/health":        true, // infra liveness, not a billing call
		"get /v1/openapi.json":  true, // the spec itself
		"post /v1/examples":     true, // deletable reference scaffold
		"get /v1/examples":      true,
		"get /v1/examples/{id}": true,
	}
}

func TestOpenAPIConformance(t *testing.T) {
	ops := loadSpecOps(t)
	excluded := excludedOps()

	m := newMock(scriptedResponse{status: 200, body: conformanceEnvelope})
	client := testClient(t, m, WithMaxRetries(0))

	ctx := context.Background()
	for _, exercise := range conformanceExercises() {
		exercise(ctx, client)
	}

	// Direction 1: every SDK call matches a spec operation.
	covered := map[string]bool{}
	var unmatched []string
	for _, call := range m.calls {
		method := strings.ToLower(call.Method)
		if match := matchSpecOp(ops, method, call.Path); match != nil {
			covered[match.key] = true
		} else {
			unmatched = append(unmatched, method+" "+call.Path)
		}
	}
	if len(unmatched) > 0 {
		sort.Strings(unmatched)
		t.Errorf("SDK emitted routes that do not exist in the spec:\n  %s", strings.Join(unmatched, "\n  "))
	}

	// Direction 2: every spec operation (minus exclusions) is covered.
	var missing []string
	for _, op := range ops {
		if excluded[op.key] || covered[op.key] {
			continue
		}
		missing = append(missing, op.key)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("spec operations with no SDK method exercising them:\n  %s", strings.Join(missing, "\n  "))
	}

	// Belt-and-braces: each exclusion must name a route that really exists, so
	// a renamed route can't hide behind the exclusion list.
	for key := range excluded {
		found := false
		for _, op := range ops {
			if op.key == key {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("EXCLUDED entry no longer exists in spec: %s", key)
		}
	}
}
