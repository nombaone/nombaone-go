package nombaone

import (
	"context"
	"errors"
	"net/http"
)

// ErrSandboxRequiresSandboxKey is returned by every client.Sandbox method when
// the client was constructed with a live key. The /v1/sandbox endpoints do not
// exist on the live API, so the SDK fails locally, before any network call.
var ErrSandboxRequiresSandboxKey = errors.New(
	"nombaone: Sandbox.* only works with a sandbox key (nbo_sandbox_…) — the /v1/sandbox endpoints do not exist on the live API. Use your sandbox key to rehearse, then go live without the sandbox calls",
)

// SandboxPaymentMethodBehavior is the deterministic outcome a sandbox payment
// method produces when charged — the "magic values" of the sandbox.
type SandboxPaymentMethodBehavior string

const (
	SandboxBehaviorSuccess                  SandboxPaymentMethodBehavior = "success"
	SandboxBehaviorDeclineInsufficientFunds SandboxPaymentMethodBehavior = "decline_insufficient_funds"
	SandboxBehaviorDeclineExpiredCard       SandboxPaymentMethodBehavior = "decline_expired_card"
	SandboxBehaviorDeclineDoNotHonor        SandboxPaymentMethodBehavior = "decline_do_not_honor"
	SandboxBehaviorRequiresOTP              SandboxPaymentMethodBehavior = "requires_otp"
)

// SandboxPaymentMethodParams are the inputs to SandboxService.CreatePaymentMethod.
type SandboxPaymentMethodParams struct {
	// CustomerID is the customer to attach the test method to (nbo…cus).
	CustomerID string `json:"customerId"`
	// Behavior defaults to success server-side.
	Behavior SandboxPaymentMethodBehavior `json:"behavior,omitempty"`
	// Kind defaults to card server-side; mandate simulates silent direct debit.
	Kind PaymentMethodKind `json:"kind,omitempty"`
}

// AdvanceCycleResult is what forcing one billing cycle produced.
type AdvanceCycleResult struct {
	Domain         string `json:"domain"` // "advance_cycle_result"
	SubscriptionID string `json:"subscriptionId"`
	// Outcome is the cycle's billing outcome: paid | past_due | pending | open.
	Outcome string `json:"outcome"`
	// Invoice is the invoice the cycle produced (or the existing one if
	// already billed).
	Invoice Invoice `json:"invoice"`
}

// SandboxSimulateWebhookParams are the inputs to SandboxService.SimulateWebhook.
type SandboxSimulateWebhookParams struct {
	// Type is any catalog event type, e.g. "invoice.payment_failed".
	Type string `json:"type"`
	// Payload shapes the delivery's data object.
	Payload map[string]any `json:"payload,omitempty"`
}

// WebhookSimulation is the minted event and how many endpoint deliveries fired.
type WebhookSimulation struct {
	Domain string `json:"domain"` // "webhook_simulation"
	// Event is the emitted event's reference (nbo…evt).
	Event          string `json:"event"`
	Type           string `json:"type"`
	DeliveredCount int    `json:"deliveredCount"`
}

// SandboxService is the sandbox namespace — simulation instruments that make
// billing outcomes happen on demand (no cron waits, no real cards). These
// endpoints exist only on the sandbox deployment; every method returns
// ErrSandboxRequiresSandboxKey (locally, before any network call) when the
// client holds a live key.
type SandboxService struct {
	client *Client
}

func (s *SandboxService) assertSandbox() error {
	if s.client.mode == ModeLive {
		return ErrSandboxRequiresSandboxKey
	}
	return nil
}

// CreatePaymentMethod mints a ready, chargeable test payment method whose
// Behavior decides every future charge outcome deterministically.
//
// Sandbox only.
//
//	method, err := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{
//		CustomerID: customer.ID,
//		Behavior:   nombaone.SandboxBehaviorDeclineInsufficientFunds, // rehearse thin-balance dunning
//	})
func (s *SandboxService) CreatePaymentMethod(ctx context.Context, params SandboxPaymentMethodParams, opts ...RequestOption) (*PaymentMethod, error) {
	if err := s.assertSandbox(); err != nil {
		return nil, err
	}
	res, err := execute[PaymentMethod](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/sandbox/payment-methods", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// AdvanceCycle is the test clock: run the subscription's next billing cycle
// right now, through the real engine — invoice, charge, ledger, webhooks and
// all.
//
// Sandbox only.
//
//	result, err := client.Sandbox.AdvanceCycle(ctx, subscription.ID)
//	// result.Outcome == "paid"; result.Invoice is the real invoice it produced
func (s *SandboxService) AdvanceCycle(ctx context.Context, subscriptionID string, opts ...RequestOption) (*AdvanceCycleResult, error) {
	if err := s.assertSandbox(); err != nil {
		return nil, err
	}
	res, err := execute[AdvanceCycleResult](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/sandbox/subscriptions/" + seg(subscriptionID) + "/advance-cycle", body: struct{}{}, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// SimulateWebhook emits a real, signed catalog event to your registered
// endpoints — the genuine pipeline (real secret, real signature, real
// retries), not a mock. The sandbox sends no organic webhooks; this is how you
// rehearse your handler.
//
// Sandbox only.
//
//	_, err := client.Sandbox.SimulateWebhook(ctx, nombaone.SandboxSimulateWebhookParams{
//		Type:    "invoice.payment_failed",
//		Payload: map[string]any{"reference": invoice.ID, "reason": "insufficient_funds"},
//	})
func (s *SandboxService) SimulateWebhook(ctx context.Context, params SandboxSimulateWebhookParams, opts ...RequestOption) (*WebhookSimulation, error) {
	if err := s.assertSandbox(); err != nil {
		return nil, err
	}
	res, err := execute[WebhookSimulation](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/sandbox/webhooks/simulate", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
