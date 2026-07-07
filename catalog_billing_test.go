package nombaone

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

// okClient returns a client whose mock replies 200 {} to everything, plus the mock.
func okClient(t *testing.T) (*Client, *mockTransport) {
	t.Helper()
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")})
	return testClient(t, m), m
}

func TestPlans_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("create", func(t *testing.T) {
		c, m := okClient(t)
		if _, err := c.Plans.Create(ctx, PlanCreateParams{Name: "Pro"}); err != nil {
			t.Fatal(err)
		}
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodPost, "/v1/plans")
		if decodeBody(t, call.Body)["name"] != "Pro" {
			t.Errorf("body = %s", call.Body)
		}
		if !uuidRE.MatchString(call.Header.Get("Idempotency-Key")) {
			t.Error("create must send an Idempotency-Key")
		}
	})

	t.Run("retrieve", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Plans.Retrieve(ctx, "nbo1pln")
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/plans/nbo1pln")
	})

	t.Run("update clears description with null", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Plans.Update(ctx, "nbo1pln", PlanUpdateParams{Description: Null[string]()})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodPatch, "/v1/plans/nbo1pln")
		if call.Body != `{"description":null}` {
			t.Errorf("body = %s", call.Body)
		}
	})

	t.Run("list with status filter", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Plans.List(ctx, PlanListParams{Status: PlanStatusArchived})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodGet, "/v1/plans")
		q, _ := url.ParseQuery(call.Query)
		if q.Get("status") != "archived" {
			t.Errorf("query = %v", q)
		}
	})

	t.Run("archive", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Plans.Archive(ctx, "nbo1pln")
		wantMethodPath(t, lastCall(t, m), http.MethodPost, "/v1/plans/nbo1pln/archive")
	})

	t.Run("nested prices create and list", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Plans.Prices.Create(ctx, "nbo1pln", PriceCreateParams{UnitAmountInKobo: 250_000, Interval: PriceIntervalMonth})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodPost, "/v1/plans/nbo1pln/prices")
		body := decodeBody(t, call.Body)
		if body["unitAmountInKobo"] != float64(250000) || body["interval"] != "month" {
			t.Errorf("body = %v", body)
		}

		c2, m2 := okClient(t)
		_, _ = c2.Plans.Prices.List(ctx, "nbo1pln", PlanPriceListParams{})
		wantMethodPath(t, lastCall(t, m2), http.MethodGet, "/v1/plans/nbo1pln/prices")
	})
}

func TestPrices_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieve", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Prices.Retrieve(ctx, "nbo1prc")
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/prices/nbo1prc")
	})

	t.Run("list uses planRef and active filter names", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Prices.List(ctx, PriceListParams{PlanRef: String("nbo1pln"), Active: Bool(true)})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodGet, "/v1/prices")
		q, _ := url.ParseQuery(call.Query)
		if q.Get("planRef") != "nbo1pln" || q.Get("active") != "true" {
			t.Errorf("query = %v (must use planRef, not planId)", q)
		}
	})

	t.Run("deactivate", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Prices.Deactivate(ctx, "nbo1prc")
		wantMethodPath(t, lastCall(t, m), http.MethodPost, "/v1/prices/nbo1prc/deactivate")
	})
}

func TestInvoices_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("retrieve", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Invoices.Retrieve(ctx, "nbo1inv")
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/invoices/nbo1inv")
	})

	t.Run("list with status and customerId filters", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Invoices.List(ctx, InvoiceListParams{Status: InvoiceStatusOpen, CustomerID: String("nbo1cus")})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodGet, "/v1/invoices")
		q, _ := url.ParseQuery(call.Query)
		if q.Get("status") != "open" || q.Get("customerId") != "nbo1cus" {
			t.Errorf("query = %v", q)
		}
	})

	t.Run("void", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Invoices.Void(ctx, "nbo1inv", InvoiceVoidParams{Comment: String("duplicate")})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodPost, "/v1/invoices/nbo1inv/void")
		if decodeBody(t, call.Body)["comment"] != "duplicate" {
			t.Errorf("body = %s", call.Body)
		}
	})
}

func TestCoupons_WireContract(t *testing.T) {
	ctx := context.Background()

	t.Run("create", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Coupons.Create(ctx, CouponCreateParams{
			Code: "LAUNCH20", PercentOff: Int(20), Duration: CouponDurationRepeating, DurationInCycles: Int(3),
		})
		call := lastCall(t, m)
		wantMethodPath(t, call, http.MethodPost, "/v1/coupons")
		body := decodeBody(t, call.Body)
		if body["code"] != "LAUNCH20" || body["percentOff"] != float64(20) || body["duration"] != "repeating" {
			t.Errorf("body = %v", body)
		}
		// amountOffInKobo must be omitted when unset (mutually exclusive with percentOff)
		if _, present := body["amountOffInKobo"]; present {
			t.Error("amountOffInKobo should be omitted when unset")
		}
	})

	t.Run("retrieve/update/list", func(t *testing.T) {
		c, m := okClient(t)
		_, _ = c.Coupons.Retrieve(ctx, "nbo1cpn")
		wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/coupons/nbo1cpn")

		c2, m2 := okClient(t)
		_, _ = c2.Coupons.Update(ctx, "nbo1cpn", CouponUpdateParams{MaxRedemptions: Int(5)})
		wantMethodPath(t, lastCall(t, m2), http.MethodPatch, "/v1/coupons/nbo1cpn")

		c3, m3 := okClient(t)
		_, _ = c3.Coupons.List(ctx, CouponListParams{})
		wantMethodPath(t, lastCall(t, m3), http.MethodGet, "/v1/coupons")
	})
}

func TestSubscriptions_WireContract(t *testing.T) {
	ctx := context.Background()

	// method, path, and (optional) an action performed against the client.
	type check struct {
		name      string
		method    string
		path      string
		call      func(*Client)
		wantBody  map[string]any
		wantIdem  bool
		wantQuery map[string]string
	}
	checks := []check{
		{
			name: "create", method: http.MethodPost, path: "/v1/subscriptions", wantIdem: true,
			call: func(c *Client) {
				_, _ = c.Subscriptions.Create(ctx, SubscriptionCreateParams{CustomerID: "nbo1cus", PriceID: "nbo1prc", PaymentMethodID: String("nbo1pmt")})
			},
			wantBody: map[string]any{"customerId": "nbo1cus", "priceId": "nbo1prc", "paymentMethodId": "nbo1pmt"},
		},
		{name: "retrieve", method: http.MethodGet, path: "/v1/subscriptions/nbo1sub", call: func(c *Client) { _, _ = c.Subscriptions.Retrieve(ctx, "nbo1sub") }},
		{
			name: "update", method: http.MethodPatch, path: "/v1/subscriptions/nbo1sub",
			call: func(c *Client) {
				_, _ = c.Subscriptions.Update(ctx, "nbo1sub", SubscriptionUpdateParams{Metadata: Metadata{"k": "v"}})
			},
			wantBody: map[string]any{},
		},
		{
			name: "list", method: http.MethodGet, path: "/v1/subscriptions",
			call: func(c *Client) {
				_, _ = c.Subscriptions.List(ctx, SubscriptionListParams{CustomerID: String("nbo1cus"), Status: SubscriptionStatusActive})
			},
			wantQuery: map[string]string{"customerId": "nbo1cus", "status": "active"},
		},
		{name: "listEvents", method: http.MethodGet, path: "/v1/subscriptions/nbo1sub/events", call: func(c *Client) { _, _ = c.Subscriptions.ListEvents(ctx, "nbo1sub", SubscriptionListEventsParams{}) }},
		{name: "pause", method: http.MethodPost, path: "/v1/subscriptions/nbo1sub/pause", call: func(c *Client) { _, _ = c.Subscriptions.Pause(ctx, "nbo1sub", SubscriptionPauseParams{}) }},
		{name: "resume", method: http.MethodPost, path: "/v1/subscriptions/nbo1sub/resume", call: func(c *Client) { _, _ = c.Subscriptions.Resume(ctx, "nbo1sub") }},
		{name: "cancel", method: http.MethodPost, path: "/v1/subscriptions/nbo1sub/cancel", call: func(c *Client) {
			_, _ = c.Subscriptions.Cancel(ctx, "nbo1sub", SubscriptionCancelParams{Mode: CancelModeAtPeriodEnd})
		}, wantBody: map[string]any{"mode": "at_period_end"}},
		{name: "resubscribe", method: http.MethodPost, path: "/v1/subscriptions/nbo1sub/resubscribe", call: func(c *Client) { _, _ = c.Subscriptions.Resubscribe(ctx, "nbo1sub", SubscriptionResubscribeParams{}) }},
		{name: "change", method: http.MethodPost, path: "/v1/subscriptions/nbo1sub/change", call: func(c *Client) {
			_, _ = c.Subscriptions.Change(ctx, "nbo1sub", SubscriptionChangeParams{PriceID: String("nbo2prc")})
		}, wantBody: map[string]any{"priceId": "nbo2prc"}},
		{name: "updatePaymentMethod", method: http.MethodPost, path: "/v1/subscriptions/nbo1sub/payment-method", call: func(c *Client) {
			_, _ = c.Subscriptions.UpdatePaymentMethod(ctx, "nbo1sub", SubscriptionUpdatePaymentMethodParams{CheckoutToken: String("tok")})
		}, wantBody: map[string]any{"checkoutToken": "tok"}},
		{name: "retrieveUpcomingInvoice", method: http.MethodGet, path: "/v1/subscriptions/nbo1sub/upcoming-invoice", call: func(c *Client) { _, _ = c.Subscriptions.RetrieveUpcomingInvoice(ctx, "nbo1sub") }},
		{name: "applyDiscount", method: http.MethodPost, path: "/v1/subscriptions/nbo1sub/discount", call: func(c *Client) {
			_, _ = c.Subscriptions.ApplyDiscount(ctx, "nbo1sub", SubscriptionApplyDiscountParams{Coupon: "X"})
		}},
		{name: "removeDiscount", method: http.MethodDelete, path: "/v1/subscriptions/nbo1sub/discount", call: func(c *Client) { _, _ = c.Subscriptions.RemoveDiscount(ctx, "nbo1sub") }},
		{name: "schedule.create", method: http.MethodPost, path: "/v1/subscriptions/nbo1sub/schedule", call: func(c *Client) {
			_, _ = c.Subscriptions.Schedule.Create(ctx, "nbo1sub", SubscriptionScheduleCreateParams{PriceID: "nbo2prc"})
		}},
		{name: "schedule.retrieve", method: http.MethodGet, path: "/v1/subscriptions/nbo1sub/schedule", call: func(c *Client) { _, _ = c.Subscriptions.Schedule.Retrieve(ctx, "nbo1sub") }},
		{name: "schedule.release", method: http.MethodDelete, path: "/v1/subscriptions/nbo1sub/schedule", call: func(c *Client) { _, _ = c.Subscriptions.Schedule.Release(ctx, "nbo1sub") }},
		{name: "dunning.retrieve", method: http.MethodGet, path: "/v1/subscriptions/nbo1sub/dunning", call: func(c *Client) { _, _ = c.Subscriptions.Dunning.Retrieve(ctx, "nbo1sub") }},
		{name: "dunning.listAttempts", method: http.MethodGet, path: "/v1/subscriptions/nbo1sub/dunning/attempts", call: func(c *Client) {
			_, _ = c.Subscriptions.Dunning.ListAttempts(ctx, "nbo1sub", DunningListAttemptsParams{})
		}},
	}

	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			c, m := okClient(t)
			tc.call(c)
			call := lastCall(t, m)
			wantMethodPath(t, call, tc.method, tc.path)
			if tc.wantIdem && !uuidRE.MatchString(call.Header.Get("Idempotency-Key")) {
				t.Errorf("%s must send an Idempotency-Key", tc.name)
			}
			for k, v := range tc.wantBody {
				if got := decodeBody(t, call.Body)[k]; got != v {
					t.Errorf("body[%q] = %v, want %v", k, got, v)
				}
			}
			if tc.wantQuery != nil {
				q, _ := url.ParseQuery(call.Query)
				for k, v := range tc.wantQuery {
					if q.Get(k) != v {
						t.Errorf("query[%q] = %q, want %q", k, q.Get(k), v)
					}
				}
			}
		})
	}
}

func TestSubscriptions_UnmarshalsRichObject(t *testing.T) {
	subJSON := `{"domain":"subscription","id":"nbo1sub","customerId":"nbo1cus","priceId":"nbo1prc","status":"active","collectionMethod":"charge_automatically","currentPeriodIndex":0,"currentPeriodStart":"2026-07-04T00:00:00.000Z","currentPeriodEnd":"2026-08-04T00:00:00.000Z","trialStart":null,"trialEnd":null,"cancelAtPeriodEnd":false,"canceledAt":null,"endedAt":null,"cancellationReason":null,"defaultPaymentMethodId":"nbo1pmt","items":[{"id":"si_1","priceId":"nbo1prc","quantity":1}],"latestInvoiceId":"nbo1inv","currency":"NGN","mode":"sandbox","createdAt":"2026-07-04T00:00:00.000Z"}`
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(subJSON)})
	c := testClient(t, m)

	sub, err := c.Subscriptions.Retrieve(context.Background(), "nbo1sub")
	if err != nil {
		t.Fatal(err)
	}
	if sub.Status != SubscriptionStatusActive || sub.CollectionMethod != CollectionMethodChargeAutomatically {
		t.Errorf("sub = %+v", sub)
	}
	if sub.CancellationReason != nil {
		t.Errorf("CancellationReason = %v, want nil", sub.CancellationReason)
	}
	if len(sub.Items) != 1 || sub.Items[0].Quantity != 1 {
		t.Errorf("items = %+v", sub.Items)
	}
	if sub.DefaultPaymentMethodID == nil || *sub.DefaultPaymentMethodID != "nbo1pmt" {
		t.Errorf("DefaultPaymentMethodID = %v", sub.DefaultPaymentMethodID)
	}
}

// TestSubscriptions_UpdatePaymentMethodReturnsPaymentMethod locks the wire
// truth: this endpoint responds with a PaymentMethod (domain "payment_method",
// id …pmt), not a Subscription — confirmed against the live sandbox.
func TestSubscriptions_UpdatePaymentMethodReturnsPaymentMethod(t *testing.T) {
	pmJSON := `{"domain":"payment_method","id":"nbo132366265063pmt","customerId":"nbo1cus","kind":"card","status":"active","isDefault":true,"brand":"visa","last4":"4242","expMonth":12,"expYear":2030,"mode":"sandbox","createdAt":"2026-07-05T00:00:00.000Z","updatedAt":"2026-07-05T00:00:00.000Z"}`
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(pmJSON)})
	c := testClient(t, m)

	pm, err := c.Subscriptions.UpdatePaymentMethod(context.Background(), "nbo1sub", SubscriptionUpdatePaymentMethodParams{PaymentMethodReference: String("nbo132366265063pmt")})
	if err != nil {
		t.Fatal(err)
	}
	if pm.Domain != "payment_method" {
		t.Errorf("domain = %q, want payment_method", pm.Domain)
	}
	if pm.Kind != PaymentMethodKindCard || pm.Last4 == nil || *pm.Last4 != "4242" {
		t.Errorf("did not decode as a PaymentMethod: %+v", pm)
	}
}
