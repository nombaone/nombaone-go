package webhook

import "encoding/json"

// EventType is a webhook event type. The set below is the frozen catalog, but
// the type is an open string: an event type the platform adds tomorrow still
// parses into an [Event] today (its Data is left as raw JSON).
type EventType = string

// The event catalog. Keep in sync with
// packages/core-contracts/src/types/webhook-events.ts.
const (
	EventTypeCustomerCreated string = "customer.created"
	EventTypeCustomerUpdated string = "customer.updated"

	EventTypeCouponCreated string = "coupon.created"

	EventTypeDiscountCreated string = "discount.created"
	EventTypeDiscountRemoved string = "discount.removed"

	EventTypePlanCreated  string = "plan.created"
	EventTypePlanUpdated  string = "plan.updated"
	EventTypePlanArchived string = "plan.archived"

	EventTypePriceCreated     string = "price.created"
	EventTypePriceDeactivated string = "price.deactivated"

	EventTypeSubscriptionCreated      string = "subscription.created"
	EventTypeSubscriptionUpdated      string = "subscription.updated"
	EventTypeSubscriptionTrialWillEnd string = "subscription.trial_will_end"
	EventTypeSubscriptionActivated    string = "subscription.activated"
	EventTypeSubscriptionPaused       string = "subscription.paused"
	EventTypeSubscriptionResumed      string = "subscription.resumed"
	EventTypeSubscriptionCanceled     string = "subscription.canceled"
	EventTypeSubscriptionChurned      string = "subscription.churned"

	EventTypeInvoiceCreated                   string = "invoice.created"
	EventTypeInvoiceFinalized                 string = "invoice.finalized"
	EventTypeInvoicePaid                      string = "invoice.paid"
	EventTypeInvoicePaymentFailed             string = "invoice.payment_failed"
	EventTypeInvoicePaymentPartiallyCollected string = "invoice.payment_partially_collected"
	EventTypeInvoicePaymentRecovered          string = "invoice.payment_recovered"
	EventTypeInvoiceActionRequired            string = "invoice.action_required"
	EventTypeInvoiceVoided                    string = "invoice.voided"

	EventTypePaymentMethodAttached string = "payment_method.attached"
	EventTypePaymentMethodUpdated  string = "payment_method.updated"
	EventTypePaymentMethodExpiring string = "payment_method.expiring"

	EventTypeSettlementCreated       string = "settlement.created"
	EventTypeSettlementRefunded      string = "settlement.refunded"
	EventTypeSettlementPayoutCreated string = "settlement.payout_created"
)

// EventRef is the underlying domain event carried by a delivery. Dedupe on
// EventRef.ID — delivery is at-least-once, and replays keep this id stable.
type EventRef struct {
	ID        string `json:"id"` // nbo…evt
	Type      string `json:"type"`
	CreatedAt string `json:"createdAt"`
}

// Event is a verified, parsed webhook delivery.
//
// Type is the delivery type; Event is the underlying domain event (dedupe on
// Event.ID); Data is the raw event payload — decode it into a typed struct
// with [DecodeData] or [Event.DataInto] after switching on Type.
type Event struct {
	// ID is the delivery reference (nbo…whd) — unique per delivery
	// attempt-target.
	ID   string `json:"id"`
	Type string `json:"type"`
	// Event is the underlying domain event. Dedupe on Event.ID.
	Event EventRef `json:"event"`
	// Data is the raw event payload. Decode it with DecodeData or DataInto.
	Data json.RawMessage `json:"data"`
	// CreatedAt is populated only when a delivery arrives in the flat legacy
	// shape (no nested event object).
	CreatedAt string `json:"createdAt,omitempty"`
}

// DataInto decodes the event payload into v (a pointer to a struct).
func (e Event) DataInto(v any) error { return json.Unmarshal(e.Data, v) }

// DecodeData decodes an event's payload into a T. Use it after switching on
// Event.Type:
//
//	switch event.Type {
//	case webhook.EventTypeInvoicePaymentFailed:
//		data, _ := webhook.DecodeData[webhook.InvoicePaymentFailedData](event)
//		notify(data.Reason)
//	}
func DecodeData[T any](e Event) (T, error) {
	var v T
	err := json.Unmarshal(e.Data, &v)
	return v, err
}

// Typed payloads worth modeling. Every payload also carries Reference (the
// affected resource's public id); decode any event into RefData when you only
// need that.

// RefData is the common shape carried by every event payload.
type RefData struct {
	Reference string `json:"reference"`
}

// InvoicePaymentFailedData is the payload of invoice.payment_failed.
type InvoicePaymentFailedData struct {
	Reference string `json:"reference"`
	Reason    string `json:"reason"`
}

// InvoicePaymentPartiallyCollectedData is the payload of
// invoice.payment_partially_collected. Amounts are integer kobo.
type InvoicePaymentPartiallyCollectedData struct {
	Reference       string `json:"reference"`
	AmountPaid      int64  `json:"amountPaid"`
	AmountRemaining int64  `json:"amountRemaining"`
}

// InvoiceActionRequiredData is the payload of invoice.action_required. Send the
// customer to CheckoutLink.
type InvoiceActionRequiredData struct {
	Reference    string `json:"reference"`
	Reason       string `json:"reason"`
	CheckoutLink string `json:"checkoutLink"`
}

// PaymentMethodAttachedData is the payload of payment_method.attached.
type PaymentMethodAttachedData struct {
	Reference string `json:"reference"`
	Kind      string `json:"kind"`
	Status    string `json:"status"`
}

// PaymentMethodUpdatedData is the payload of payment_method.updated.
type PaymentMethodUpdatedData struct {
	Reference    string `json:"reference"`
	Subscription string `json:"subscription"`
}

// SubscriptionCreatedData is the payload of subscription.created.
type SubscriptionCreatedData struct {
	Reference string `json:"reference"`
	Status    string `json:"status"`
}

// CouponCreatedData is the payload of coupon.created.
type CouponCreatedData struct {
	Reference string `json:"reference"`
	Code      string `json:"code"`
}
