package nombaone

// Objects that appear across several resource namespaces. Everything here
// mirrors the API's response DTOs field for field.

// DiscountStatus is the lifecycle state of an applied discount.
type DiscountStatus string

const (
	DiscountStatusActive DiscountStatus = "active"
	DiscountStatusEnded  DiscountStatus = "ended"
)

// Discount is a coupon applied to a customer or a subscription — the
// application, not the coupon definition.
type Discount struct {
	Domain   string `json:"domain"` // "discount"
	ID       string `json:"id"`     // nbo…dsc
	CouponID string `json:"couponId"`
	// CustomerID is set when the discount is applied to a customer.
	CustomerID *string `json:"customerId"`
	// SubscriptionID is set when the discount is applied to a subscription.
	SubscriptionID *string        `json:"subscriptionId"`
	Status         DiscountStatus `json:"status"`
	// CyclesRemaining is the cycles left for a repeating coupon; nil for
	// once/forever.
	CyclesRemaining *int    `json:"cyclesRemaining"`
	StartAt         string  `json:"startAt"`
	EndAt           *string `json:"endAt"`
	Mode            Mode    `json:"mode"`
	CreatedAt       string  `json:"createdAt"`
}

// BillingReason is why an invoice (or upcoming invoice) exists.
type BillingReason string

const (
	BillingReasonSubscriptionCreate BillingReason = "subscription_create"
	BillingReasonSubscriptionCycle  BillingReason = "subscription_cycle"
	BillingReasonSubscriptionUpdate BillingReason = "subscription_update"
	BillingReasonManual             BillingReason = "manual"
)

// LineItemKind is the kind of a single invoice line. Discount and credit lines
// carry negative amounts.
type LineItemKind string

const (
	LineItemKindSubscription LineItemKind = "subscription"
	LineItemKindProration    LineItemKind = "proration"
	LineItemKindDiscount     LineItemKind = "discount"
	LineItemKindCredit       LineItemKind = "credit"
	LineItemKindAdjustment   LineItemKind = "adjustment"
)

// InvoiceLineItem is one line on an invoice. Amounts are integer kobo;
// discount and credit lines are negative.
type InvoiceLineItem struct {
	ID          string       `json:"id"`
	Kind        LineItemKind `json:"kind"`
	Description string       `json:"description"`
	// AmountInKobo is integer kobo (₦1.00 = 100). Negative for discount/credit.
	AmountInKobo Kobo `json:"amountInKobo"`
	Quantity     int  `json:"quantity"`
}

// DomainEvent is an entry in the append-only domain-event log — the audit
// trail behind every webhook. Payload carries the same data your endpoints
// receive.
type DomainEvent struct {
	Domain string `json:"domain"` // "event"
	// ID is the event reference (nbo…evt) — the id webhook receivers dedupe on.
	ID string `json:"id"`
	// Type is the catalog event type, e.g. "invoice.paid".
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	CreatedAt string         `json:"createdAt"`
}
