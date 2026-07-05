package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// CouponDuration is how long a coupon's discount lasts once applied.
type CouponDuration string

const (
	CouponDurationOnce      CouponDuration = "once"
	CouponDurationRepeating CouponDuration = "repeating"
	CouponDurationForever   CouponDuration = "forever"
)

// Coupon is a reusable discount rule. Applying a coupon to a customer or
// subscription creates a Discount — the coupon is the rule, the discount is
// one application of it.
type Coupon struct {
	Domain string `json:"domain"` // "coupon"
	ID     string `json:"id"`     // nbo…cpn
	// Code is the tenant-facing redemption code, e.g. "LAUNCH20".
	Code     string         `json:"code"`
	Duration CouponDuration `json:"duration"`
	// AmountOffInKobo and PercentOff are mutually exclusive: exactly one is
	// non-nil.
	AmountOffInKobo  *Kobo   `json:"amountOffInKobo"`
	PercentOff       *int    `json:"percentOff"`
	DurationInCycles *int    `json:"durationInCycles"`
	RedeemBy         *string `json:"redeemBy"`
	MaxRedemptions   *int    `json:"maxRedemptions"`
	TimesRedeemed    int     `json:"timesRedeemed"`
	Mode             Mode    `json:"mode"`
	CreatedAt        string  `json:"createdAt"`
}

// CouponCreateParams are the inputs to CouponsService.Create. Set exactly one
// of AmountOffInKobo or PercentOff.
type CouponCreateParams struct {
	Code     string         `json:"code"`
	Duration CouponDuration `json:"duration"`
	// AmountOffInKobo is a fixed discount, integer kobo (₦1.00 = 100).
	AmountOffInKobo *Kobo `json:"amountOffInKobo,omitempty"`
	// PercentOff is a percentage discount, 1–100.
	PercentOff *int `json:"percentOff,omitempty"`
	// DurationInCycles is required when Duration is repeating.
	DurationInCycles *int `json:"durationInCycles,omitempty"`
	// RedeemBy is an ISO-8601 date-time after which the coupon can no longer
	// be applied.
	RedeemBy       *string  `json:"redeemBy,omitempty"`
	MaxRedemptions *int     `json:"maxRedemptions,omitempty"`
	Metadata       Metadata `json:"metadata,omitempty"`
}

// CouponUpdateParams are the inputs to CouponsService.Update. At least one
// field must be set.
type CouponUpdateParams struct {
	RedeemBy       *string  `json:"redeemBy,omitempty"`
	MaxRedemptions *int     `json:"maxRedemptions,omitempty"`
	Metadata       Metadata `json:"metadata,omitempty"`
}

// CouponListParams are the filters for CouponsService.List.
type CouponListParams struct {
	Limit  *int
	Cursor *string
}

func (p CouponListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// CouponsService is the coupons namespace — discount rules you apply via
// client.Customers.ApplyDiscount / client.Subscriptions.ApplyDiscount.
type CouponsService struct {
	client *Client
}

// Create creates a coupon.
//
// Common errors: 422 COUPON_INVALID_DEFINITION — set exactly one of
// AmountOffInKobo / PercentOff.
//
//	coupon, err := client.Coupons.Create(ctx, nombaone.CouponCreateParams{
//		Code:             "LAUNCH20",
//		PercentOff:       nombaone.Int(20),
//		Duration:         nombaone.CouponDurationRepeating,
//		DurationInCycles: nombaone.Int(3),
//	})
func (s *CouponsService) Create(ctx context.Context, params CouponCreateParams, opts ...RequestOption) (*Coupon, error) {
	res, err := execute[Coupon](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/coupons", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Retrieve returns a coupon by id.
//
// Common errors: 404 COUPON_NOT_FOUND.
func (s *CouponsService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*Coupon, error) {
	res, err := execute[Coupon](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/coupons/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Update changes a coupon's redeem-by, max redemptions, or metadata.
func (s *CouponsService) Update(ctx context.Context, id string, params CouponUpdateParams, opts ...RequestOption) (*Coupon, error) {
	res, err := execute[Coupon](ctx, s.client, requestSpec{
		method: http.MethodPatch, path: "/coupons/" + seg(id), body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns coupons, newest first.
func (s *CouponsService) List(ctx context.Context, params CouponListParams, opts ...RequestOption) (*Page[Coupon], error) {
	return executePage[Coupon](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/coupons", query: params.toQuery(), opts: opts,
	})
}
