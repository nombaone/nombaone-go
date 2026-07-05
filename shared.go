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
