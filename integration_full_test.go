package nombaone_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	nombaone "github.com/nombaone/nombaone-go"
)

// TestIntegrationFullSurface exercises every SDK method against a real sandbox.
// A method passes if it either succeeds or fails with a specific, expected
// typed API error (proving the path/verb/body/wire are correct even when the
// business state is absent). A transport error, a parse error, or an
// unexpected error code fails the method.
//
//	NOMBAONE_INTEGRATION=1 NOMBAONE_API_KEY=nbo_sandbox_… \
//	NOMBAONE_BASE_URL=https://sandbox.api.nombaone.xyz \
//	go test -run TestIntegrationFullSurface -v ./...

// tally accumulates the owner-legible verdict across every method call-site.
type tally struct {
	ok       int // succeeded outright
	expected int // returned a specific, expected typed API error
	defects  int // parse mismatch, crash, transport failure, or wrong error code
}

// surf is the process-wide tally for the full-surface run (the test runs once).
var surf tally

func (ta *tally) report(t *testing.T) {
	t.Helper()
	total := ta.ok + ta.expected + ta.defects
	t.Logf("────────────────────────────────────────────────────────")
	t.Logf("VERDICT: %d method checks across 15 namespaces | ok %d | expected-errors %d | DEFECTS %d",
		total, ta.ok, ta.expected, ta.defects)
	t.Logf("────────────────────────────────────────────────────────")
}

// expectOKOr passes if err is nil or a tolerated API error code; otherwise it
// records a DEFECT. It also records the outcome into the shared verdict tally.
func expectOKOr(t *testing.T, label string, err error, tolerated ...nombaone.ErrorCode) bool {
	t.Helper()
	if err == nil {
		surf.ok++
		t.Logf("✓ %s", label)
		return true
	}
	var apiErr *nombaone.APIError
	if errors.As(err, &apiErr) {
		for _, c := range tolerated {
			if apiErr.Code == c {
				surf.expected++
				t.Logf("✓ %s (expected %s)", label, apiErr.Code)
				return false
			}
		}
		surf.defects++
		t.Errorf("✗ DEFECT %s — unexpected API error %s: %s", label, apiErr.Code, apiErr.Hint)
		return false
	}
	surf.defects++
	t.Errorf("✗ DEFECT %s — non-API error: %v", label, err)
	return false
}

func mustOK(t *testing.T, label string, err error) {
	t.Helper()
	if err != nil {
		surf.defects++
		t.Fatalf("✗ DEFECT %s — %v", label, err)
	}
	surf.ok++
	t.Logf("✓ %s", label)
}

// wantDomain asserts a response object's discriminator matches the type the SDK
// claims — catching a silent wire/model mismatch (Go's json decoder does not
// error on a wrong-but-overlapping shape). A mismatch is a DEFECT.
func wantDomain(t *testing.T, label, got, want string) {
	t.Helper()
	if got != want {
		surf.defects++
		t.Errorf("✗ DEFECT %s — response domain=%q, want %q (wire/model mismatch)", label, got, want)
		return
	}
	t.Logf("  ↳ %s domain=%q ✓", label, got)
}

func fullClient(t *testing.T) *nombaone.Client {
	t.Helper()
	if os.Getenv("NOMBAONE_INTEGRATION") != "1" {
		t.Skip("set NOMBAONE_INTEGRATION=1 (and NOMBAONE_API_KEY) to run the full-surface suite")
	}
	key := os.Getenv("NOMBAONE_API_KEY")
	if key == "" {
		t.Fatal("NOMBAONE_INTEGRATION=1 but NOMBAONE_API_KEY is empty")
	}
	opts := []nombaone.Option{nombaone.WithAPIKey(key), nombaone.WithMaxRetries(4)}
	if base := os.Getenv("NOMBAONE_BASE_URL"); base != "" {
		opts = append(opts, nombaone.WithBaseURL(base))
	}
	c, err := nombaone.New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

func TestIntegrationFullSurface(t *testing.T) {
	client := fullClient(t)
	ctx := context.Background()
	uniq := fmt.Sprintf("go-full-%d", time.Now().UnixNano())

	// Print the owner-legible verdict after every subtest has run.
	t.Cleanup(func() { surf.report(t) })

	// ---- Shared setup: catalog + a subscriber + a chargeable card ----
	plan, err := client.Plans.Create(ctx, nombaone.PlanCreateParams{Name: "Full " + uniq, Description: nombaone.String("full-surface plan")})
	mustOK(t, "plans.Create", err)
	wantDomain(t, "plans.Create", plan.Domain, "plan")
	priceMonthly, err := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{UnitAmountInKobo: 250_000, Interval: nombaone.PriceIntervalMonth})
	mustOK(t, "plans.Prices.Create (monthly)", err)
	wantDomain(t, "plans.Prices.Create", priceMonthly.Domain, "price")
	priceUpgrade, err := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{UnitAmountInKobo: 500_000, Interval: nombaone.PriceIntervalMonth})
	mustOK(t, "plans.Prices.Create (upgrade)", err)

	customer, err := client.Customers.Create(ctx, nombaone.CustomerCreateParams{Email: uniq + "@example.com", Name: "Full Surface"})
	mustOK(t, "customers.Create", err)
	wantDomain(t, "customers.Create", customer.Domain, "customer")

	card, err := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{CustomerID: customer.ID, Behavior: nombaone.SandboxBehaviorSuccess})
	mustOK(t, "sandbox.CreatePaymentMethod", err)
	wantDomain(t, "sandbox.CreatePaymentMethod", card.Domain, "payment_method")
	card2, err := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{CustomerID: customer.ID, Behavior: nombaone.SandboxBehaviorSuccess})
	mustOK(t, "sandbox.CreatePaymentMethod (2nd)", err)

	coupon, err := client.Coupons.Create(ctx, nombaone.CouponCreateParams{Code: "FULL" + uniq[len(uniq)-6:], PercentOff: nombaone.Int(15), Duration: nombaone.CouponDurationOnce})
	mustOK(t, "coupons.Create", err)
	wantDomain(t, "coupons.Create", coupon.Domain, "coupon")

	t.Run("customers", func(t *testing.T) {
		_, err := client.Customers.Retrieve(ctx, customer.ID)
		expectOKOr(t, "customers.Retrieve", err)
		_, err = client.Customers.Update(ctx, customer.ID, nombaone.CustomerUpdateParams{Phone: nombaone.Set("+2348012345678")})
		expectOKOr(t, "customers.Update (set phone)", err)
		_, err = client.Customers.Update(ctx, customer.ID, nombaone.CustomerUpdateParams{Phone: nombaone.Null[string]()})
		expectOKOr(t, "customers.Update (clear phone → null)", err)
		_, err = client.Customers.List(ctx, nombaone.CustomerListParams{Limit: nombaone.Int(3)})
		expectOKOr(t, "customers.List", err)

		grant, err := client.Customers.GrantCredit(ctx, customer.ID, nombaone.CustomerGrantCreditParams{AmountInKobo: 100_000, Source: nombaone.CreditSourceGoodwill})
		expectOKOr(t, "customers.GrantCredit", err)
		_, err = client.Customers.RetrieveCreditBalance(ctx, customer.ID)
		expectOKOr(t, "customers.RetrieveCreditBalance", err)
		if grant != nil {
			_, err = client.Customers.VoidCredit(ctx, customer.ID, grant.ID)
			expectOKOr(t, "customers.VoidCredit", err)
		}

		_, err = client.Customers.ApplyDiscount(ctx, customer.ID, nombaone.CustomerApplyDiscountParams{Coupon: coupon.Code})
		applied := expectOKOr(t, "customers.ApplyDiscount", err, nombaone.ErrCodeCouponAlreadyApplied)
		_, err = client.Customers.RemoveDiscount(ctx, customer.ID)
		expectOKOr(t, "customers.RemoveDiscount", err, nombaone.ErrCodeDiscountNotFound)
		_ = applied
	})

	t.Run("plans_prices", func(t *testing.T) {
		_, err := client.Plans.Retrieve(ctx, plan.ID)
		expectOKOr(t, "plans.Retrieve", err)
		_, err = client.Plans.Update(ctx, plan.ID, nombaone.PlanUpdateParams{Description: nombaone.Null[string]()})
		expectOKOr(t, "plans.Update (clear description → null)", err)
		_, err = client.Plans.List(ctx, nombaone.PlanListParams{Status: nombaone.PlanStatusActive, Limit: nombaone.Int(3)})
		expectOKOr(t, "plans.List", err)

		_, err = client.Prices.Retrieve(ctx, priceMonthly.ID)
		expectOKOr(t, "prices.Retrieve", err)
		_, err = client.Prices.List(ctx, nombaone.PriceListParams{PlanRef: nombaone.String(plan.ID), Active: nombaone.Bool(true)})
		expectOKOr(t, "prices.List", err)

		// Deactivate a throwaway price and archive a throwaway plan (no subscribers).
		throwPlan, err := client.Plans.Create(ctx, nombaone.PlanCreateParams{Name: "Throwaway " + uniq})
		mustOK(t, "plans.Create (throwaway)", err)
		throwPrice, err := client.Plans.Prices.Create(ctx, throwPlan.ID, nombaone.PriceCreateParams{UnitAmountInKobo: 100_00, Interval: nombaone.PriceIntervalYear})
		mustOK(t, "plans.Prices.Create (throwaway)", err)
		_, err = client.Plans.Prices.List(ctx, throwPlan.ID, nombaone.PlanPriceListParams{})
		expectOKOr(t, "plans.Prices.List", err)
		_, err = client.Prices.Deactivate(ctx, throwPrice.ID)
		expectOKOr(t, "prices.Deactivate", err, nombaone.ErrCodePriceAlreadyInactive)
		_, err = client.Plans.Archive(ctx, throwPlan.ID)
		expectOKOr(t, "plans.Archive", err, nombaone.ErrCodePlanAlreadyArchived)
	})

	// Helper: create an active subscription for a state test.
	newActiveSub := func(t *testing.T) *nombaone.Subscription {
		t.Helper()
		sub, err := client.Subscriptions.Create(ctx, nombaone.SubscriptionCreateParams{
			CustomerID: customer.ID, PriceID: priceMonthly.ID, PaymentMethodID: nombaone.String(card.ID),
		})
		mustOK(t, "subscriptions.Create", err)
		wantDomain(t, "subscriptions.Create", sub.Domain, "subscription")
		return sub
	}

	t.Run("subscriptions_reads_and_edits", func(t *testing.T) {
		sub := newActiveSub(t)
		_, err := client.Subscriptions.Retrieve(ctx, sub.ID)
		expectOKOr(t, "subscriptions.Retrieve", err)
		_, err = client.Subscriptions.Update(ctx, sub.ID, nombaone.SubscriptionUpdateParams{Metadata: nombaone.Metadata{"tier": "gold"}})
		expectOKOr(t, "subscriptions.Update", err)
		_, err = client.Subscriptions.List(ctx, nombaone.SubscriptionListParams{CustomerID: nombaone.String(customer.ID)})
		expectOKOr(t, "subscriptions.List", err)
		_, err = client.Subscriptions.ListEvents(ctx, sub.ID, nombaone.SubscriptionListEventsParams{Limit: nombaone.Int(5)})
		expectOKOr(t, "subscriptions.ListEvents", err)
		_, err = client.Subscriptions.RetrieveUpcomingInvoice(ctx, sub.ID)
		expectOKOr(t, "subscriptions.RetrieveUpcomingInvoice", err)
		_, err = client.Subscriptions.Dunning.Retrieve(ctx, sub.ID)
		expectOKOr(t, "subscriptions.Dunning.Retrieve", err)
		_, err = client.Subscriptions.Dunning.ListAttempts(ctx, sub.ID, nombaone.DunningListAttemptsParams{})
		expectOKOr(t, "subscriptions.Dunning.ListAttempts", err)
		_, err = client.Subscriptions.Change(ctx, sub.ID, nombaone.SubscriptionChangeParams{PriceID: nombaone.String(priceUpgrade.ID)})
		expectOKOr(t, "subscriptions.Change (prorated upgrade)", err)
		pm, err := client.Subscriptions.UpdatePaymentMethod(ctx, sub.ID, nombaone.SubscriptionUpdatePaymentMethodParams{PaymentMethodReference: nombaone.String(card2.ID)})
		if expectOKOr(t, "subscriptions.UpdatePaymentMethod", err) {
			// The wire returns a PaymentMethod here, not a Subscription.
			wantDomain(t, "subscriptions.UpdatePaymentMethod", pm.Domain, "payment_method")
		}
		_, err = client.Subscriptions.ApplyDiscount(ctx, sub.ID, nombaone.SubscriptionApplyDiscountParams{Coupon: coupon.Code})
		expectOKOr(t, "subscriptions.ApplyDiscount", err, nombaone.ErrCodeCouponAlreadyApplied)
		_, err = client.Subscriptions.RemoveDiscount(ctx, sub.ID)
		expectOKOr(t, "subscriptions.RemoveDiscount", err, nombaone.ErrCodeDiscountNotFound)
	})

	t.Run("subscriptions_pause_resume", func(t *testing.T) {
		sub := newActiveSub(t)
		_, err := client.Subscriptions.Pause(ctx, sub.ID, nombaone.SubscriptionPauseParams{})
		paused := expectOKOr(t, "subscriptions.Pause", err, nombaone.ErrCodeSubscriptionIllegalTransition)
		if paused {
			_, err = client.Subscriptions.Resume(ctx, sub.ID)
			expectOKOr(t, "subscriptions.Resume", err)
		}
	})

	t.Run("subscriptions_schedule", func(t *testing.T) {
		sub := newActiveSub(t)
		_, err := client.Subscriptions.Schedule.Create(ctx, sub.ID, nombaone.SubscriptionScheduleCreateParams{PriceID: priceUpgrade.ID})
		created := expectOKOr(t, "subscriptions.Schedule.Create", err, nombaone.ErrCodeSubscriptionScheduleConflict)
		_, err = client.Subscriptions.Schedule.Retrieve(ctx, sub.ID)
		expectOKOr(t, "subscriptions.Schedule.Retrieve", err, nombaone.ErrCodeSubscriptionScheduleNotFound)
		if created {
			_, err = client.Subscriptions.Schedule.Release(ctx, sub.ID)
			expectOKOr(t, "subscriptions.Schedule.Release", err, nombaone.ErrCodeSubscriptionScheduleNotFound)
		}
	})

	t.Run("subscriptions_cancel_resubscribe", func(t *testing.T) {
		sub := newActiveSub(t)
		_, err := client.Subscriptions.Cancel(ctx, sub.ID, nombaone.SubscriptionCancelParams{Mode: nombaone.CancelModeNow})
		canceled := expectOKOr(t, "subscriptions.Cancel", err)
		if canceled {
			_, err = client.Subscriptions.Resubscribe(ctx, sub.ID, nombaone.SubscriptionResubscribeParams{})
			expectOKOr(t, "subscriptions.Resubscribe", err, nombaone.ErrCodeSubscriptionNotTerminal)
		}
	})

	t.Run("invoices", func(t *testing.T) {
		sub := newActiveSub(t)
		cycle, err := client.Sandbox.AdvanceCycle(ctx, sub.ID)
		mustOK(t, "sandbox.AdvanceCycle", err)
		wantDomain(t, "sandbox.AdvanceCycle.invoice", cycle.Invoice.Domain, "invoice")
		inv, err := client.Invoices.Retrieve(ctx, cycle.Invoice.ID)
		if expectOKOr(t, "invoices.Retrieve", err) {
			wantDomain(t, "invoices.Retrieve", inv.Domain, "invoice")
		}
		_, err = client.Invoices.List(ctx, nombaone.InvoiceListParams{CustomerID: nombaone.String(customer.ID), Status: nombaone.InvoiceStatusPaid})
		expectOKOr(t, "invoices.List", err)
		_, err = client.Invoices.Void(ctx, cycle.Invoice.ID, nombaone.InvoiceVoidParams{Comment: nombaone.String("test void")})
		expectOKOr(t, "invoices.Void", err,
			nombaone.ErrCodeInvoiceNotVoidable, nombaone.ErrCodeInvoiceAlreadyPaid, nombaone.ErrCodeInvoiceAlreadyFinalized)
	})

	t.Run("coupons", func(t *testing.T) {
		_, err := client.Coupons.Retrieve(ctx, coupon.ID)
		expectOKOr(t, "coupons.Retrieve", err)
		_, err = client.Coupons.Update(ctx, coupon.ID, nombaone.CouponUpdateParams{MaxRedemptions: nombaone.Int(1000)})
		expectOKOr(t, "coupons.Update", err)
		_, err = client.Coupons.List(ctx, nombaone.CouponListParams{Limit: nombaone.Int(3)})
		expectOKOr(t, "coupons.List", err)
	})

	t.Run("payment_methods", func(t *testing.T) {
		_, err := client.PaymentMethods.Setup(ctx, nombaone.PaymentMethodSetupParams{CustomerRef: customer.ID, AmountInKobo: 5_000, CallbackURL: "https://example.com/return"})
		expectOKOr(t, "paymentMethods.Setup", err)
		_, err = client.PaymentMethods.CreateVirtualAccount(ctx, nombaone.PaymentMethodVirtualAccountParams{CustomerRef: customer.ID})
		expectOKOr(t, "paymentMethods.CreateVirtualAccount", err)
		gotPM, err := client.PaymentMethods.Retrieve(ctx, card.ID)
		if expectOKOr(t, "paymentMethods.Retrieve", err) {
			wantDomain(t, "paymentMethods.Retrieve", gotPM.Domain, "payment_method")
		}
		_, err = client.PaymentMethods.List(ctx, nombaone.PaymentMethodListParams{CustomerRef: nombaone.String(customer.ID)})
		expectOKOr(t, "paymentMethods.List", err)
		_, err = client.PaymentMethods.SetDefault(ctx, card.ID)
		expectOKOr(t, "paymentMethods.SetDefault", err)

		// Remove a throwaway method so we don't disturb the shared cards.
		throwCard, err := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{CustomerID: customer.ID})
		mustOK(t, "sandbox.CreatePaymentMethod (throwaway)", err)
		_, err = client.PaymentMethods.Remove(ctx, throwCard.ID)
		expectOKOr(t, "paymentMethods.Remove", err)
	})

	t.Run("mandates_async", func(t *testing.T) {
		// NOTE (backend quirk, deployed sandbox 2026-07-05): POST /v1/mandates
		// returns 504 (NIBSS upstream unavailable in the deployed sandbox). The
		// SDK method is correct — it builds the right request and surfaces a
		// typed ServerError — so we fail fast (no retry) and tolerate the
		// upstream error rather than let the SDK's correct 5xx retries stall.
		failFast := []nombaone.RequestOption{nombaone.WithRequestMaxRetries(0), nombaone.WithRequestTimeout(30 * time.Second)}
		setup, err := client.Mandates.Create(ctx, nombaone.MandateCreateParams{
			CustomerRef: customer.ID, CustomerAccountNumber: "0123456789", BankCode: "058",
			CustomerName: "Full Surface", CustomerAccountName: "Full Surface", CustomerPhoneNumber: "+2348012345678",
			CustomerAddress: "1 Marina, Lagos", Narration: "Full surface sub", MaxAmountInKobo: 500_000,
		}, failFast...)
		created := expectOKOr(t, "mandates.Create (async; backend 504 tolerated)", err,
			nombaone.ErrCodeSystemUpstreamError, nombaone.ErrCodeSystemInternalError)
		if created && setup != nil {
			// retrieve returns a PaymentMethod (the documented quirk)
			pm, err := client.Mandates.Retrieve(ctx, setup.Reference, failFast...)
			if expectOKOr(t, "mandates.Retrieve → PaymentMethod", err,
				nombaone.ErrCodePaymentMethodNotFound, nombaone.ErrCodeSystemUpstreamError) && pm != nil {
				if pm.Kind != nombaone.PaymentMethodKindMandate {
					t.Errorf("mandate retrieve kind = %q, want mandate", pm.Kind)
				}
			}
		}
	})

	t.Run("settlements", func(t *testing.T) {
		// A sandbox org may have no settlement subaccount configured yet, in
		// which case escrow/settlement/payout reads return the typed
		// SETTLEMENT_SUBACCOUNT_NOT_FOUND — a legitimate business state the SDK
		// surfaces correctly, not a client error.
		_, err := client.Settlements.RetrieveEscrow(ctx)
		expectOKOr(t, "settlements.RetrieveEscrow", err, nombaone.ErrCodeSettlementSubaccountNotFound)
		page, err := client.Settlements.List(ctx, nombaone.SettlementListParams{Limit: nombaone.Int(3)})
		expectOKOr(t, "settlements.List", err, nombaone.ErrCodeSettlementSubaccountNotFound)
		if page != nil && len(page.Data) > 0 {
			st := page.Data[0]
			_, err = client.Settlements.Retrieve(ctx, st.ID)
			expectOKOr(t, "settlements.Retrieve", err)
			_, err = client.Settlements.Refund(ctx, st.ID, nombaone.SettlementRefundParams{AmountInKobo: nombaone.Int64(100)})
			expectOKOr(t, "settlements.Refund", err,
				nombaone.ErrCodeRefundAlreadyRefunded, nombaone.ErrCodeRefundAmountExceedsNet, nombaone.ErrCodeSettlementNotFound)
		} else {
			// No settlements in a fresh sandbox — prove the read/write paths reach the API with well-typed errors.
			_, err = client.Settlements.Retrieve(ctx, "nbo000000000000stl")
			expectOKOr(t, "settlements.Retrieve (absent)", err, nombaone.ErrCodeSettlementNotFound)
			_, err = client.Settlements.Refund(ctx, "nbo000000000000stl", nombaone.SettlementRefundParams{})
			expectOKOr(t, "settlements.Refund (absent)", err, nombaone.ErrCodeSettlementNotFound, nombaone.ErrCodeRefundAmountExceedsNet)
		}
		_, err = client.Settlements.CreatePayout(ctx,
			nombaone.PayoutCreateParams{AmountInKobo: 100_000_000, BankCode: "058", AccountNumber: "0123456789"},
			nombaone.WithIdempotencyKey("full-payout-"+uniq))
		expectOKOr(t, "settlements.CreatePayout", err,
			nombaone.ErrCodePayoutExceedsAvailable, nombaone.ErrCodeEscrowLocked, nombaone.ErrCodeSettlementSubaccountNotFound)
	})

	t.Run("webhook_endpoints_and_deliveries", func(t *testing.T) {
		ep, err := client.WebhookEndpoints.Create(ctx, nombaone.WebhookEndpointCreateParams{
			URL: "https://example.com/nombaone/hooks/" + uniq, EnabledEvents: []string{"*"},
		})
		mustOK(t, "webhookEndpoints.Create (secret shown once)", err)
		if len(ep.SigningSecret) < 10 {
			t.Errorf("signing secret too short: %q", ep.SigningSecret)
		}
		_, err = client.WebhookEndpoints.Retrieve(ctx, ep.ID)
		expectOKOr(t, "webhookEndpoints.Retrieve", err)
		_, err = client.WebhookEndpoints.Update(ctx, ep.ID, nombaone.WebhookEndpointUpdateParams{EnabledEvents: []string{"invoice.paid"}})
		expectOKOr(t, "webhookEndpoints.Update", err)
		_, err = client.WebhookEndpoints.List(ctx)
		expectOKOr(t, "webhookEndpoints.List", err)
		_, err = client.WebhookEndpoints.RotateSecret(ctx, ep.ID)
		expectOKOr(t, "webhookEndpoints.RotateSecret (new secret once)", err)

		// Produce a real delivery, then exercise the deliveries sub-namespace.
		_, err = client.Sandbox.SimulateWebhook(ctx, nombaone.SandboxSimulateWebhookParams{Type: "invoice.paid", Payload: map[string]any{"reference": "nbo000000000001inv"}})
		expectOKOr(t, "sandbox.SimulateWebhook", err)

		var deliveryID string
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			page, err := client.WebhookEndpoints.Deliveries.List(ctx, ep.ID, nombaone.WebhookDeliveryListParams{Limit: nombaone.Int(5)})
			if err != nil {
				expectOKOr(t, "webhookEndpoints.Deliveries.List", err)
				break
			}
			if len(page.Data) > 0 {
				deliveryID = page.Data[0].ID
				t.Logf("✓ webhookEndpoints.Deliveries.List (%d deliveries)", len(page.Data))
				break
			}
			time.Sleep(1 * time.Second)
		}
		if deliveryID != "" {
			_, err = client.WebhookEndpoints.Deliveries.Retrieve(ctx, ep.ID, deliveryID)
			expectOKOr(t, "webhookEndpoints.Deliveries.Retrieve", err)
			_, err = client.WebhookEndpoints.Deliveries.Replay(ctx, ep.ID, deliveryID)
			expectOKOr(t, "webhookEndpoints.Deliveries.Replay", err)
		} else {
			t.Logf("… no delivery surfaced within 20s; deliveries.Retrieve/Replay exercised against a synthetic id")
			_, err = client.WebhookEndpoints.Deliveries.Retrieve(ctx, ep.ID, "nbo000000000000whd")
			expectOKOr(t, "webhookEndpoints.Deliveries.Retrieve (absent)", err, nombaone.ErrCodeClientResourceNotFound)
			_, err = client.WebhookEndpoints.Deliveries.Replay(ctx, ep.ID, "nbo000000000000whd")
			expectOKOr(t, "webhookEndpoints.Deliveries.Replay (absent)", err, nombaone.ErrCodeClientResourceNotFound)
		}

		_, err = client.WebhookEndpoints.Delete(ctx, ep.ID)
		expectOKOr(t, "webhookEndpoints.Delete", err)
	})

	t.Run("events", func(t *testing.T) {
		page, err := client.Events.List(ctx, nombaone.EventListParams{Limit: nombaone.Int(5)})
		mustOK(t, "events.List", err)
		if len(page.Data) > 0 {
			wantDomain(t, "events.List item", page.Data[0].Domain, "event")
			ev, err := client.Events.Retrieve(ctx, page.Data[0].ID)
			if expectOKOr(t, "events.Retrieve", err) {
				wantDomain(t, "events.Retrieve", ev.Domain, "event")
			}
		}
		catalog, err := client.Events.Catalog(ctx)
		if expectOKOr(t, "events.Catalog", err) {
			t.Logf("  catalog has %d event types", len(catalog))
		}
	})

	t.Run("organization_and_metrics", func(t *testing.T) {
		_, err := client.Organization.Retrieve(ctx)
		expectOKOr(t, "organization.Retrieve", err)
		_, err = client.Organization.Update(ctx, nombaone.TenantSettingsUpdateParams{Branding: &nombaone.OrgBranding{DisplayName: "Full Surface Co"}})
		expectOKOr(t, "organization.Update", err)
		_, err = client.Organization.Billing.Retrieve(ctx)
		expectOKOr(t, "organization.Billing.Retrieve", err)
		_, err = client.Organization.Billing.Update(ctx, nombaone.BillingSettingsUpdateParams{CommsEnabled: nombaone.Bool(true)})
		expectOKOr(t, "organization.Billing.Update", err)
		_, err = client.Metrics.Billing(ctx, nombaone.BillingMetricsParams{})
		expectOKOr(t, "metrics.Billing", err)
	})

	t.Run("sandbox", func(t *testing.T) {
		sub := newActiveSub(t)
		_, err := client.Sandbox.AdvanceCycle(ctx, sub.ID)
		expectOKOr(t, "sandbox.AdvanceCycle", err)
		_, err = client.Sandbox.SimulateWebhook(ctx, nombaone.SandboxSimulateWebhookParams{Type: "invoice.payment_failed"})
		expectOKOr(t, "sandbox.SimulateWebhook", err)
		// CreatePaymentMethod already exercised in setup.
		t.Log("✓ sandbox.CreatePaymentMethod (exercised in setup)")
	})
}
