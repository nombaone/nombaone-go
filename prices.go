package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// PriceInterval is the billing cadence of a price.
type PriceInterval string

const (
	PriceIntervalDay   PriceInterval = "day"
	PriceIntervalWeek  PriceInterval = "week"
	PriceIntervalMonth PriceInterval = "month"
	PriceIntervalYear  PriceInterval = "year"
)

// PriceUsageType is how usage is counted for a price.
type PriceUsageType string

const (
	PriceUsageTypeLicensed PriceUsageType = "licensed"
	PriceUsageTypeMetered  PriceUsageType = "metered"
)

// PriceBillingScheme is how a price computes its amount.
type PriceBillingScheme string

const (
	PriceBillingSchemePerUnit PriceBillingScheme = "per_unit"
	PriceBillingSchemeTiered  PriceBillingScheme = "tiered"
)

// Price is how much a plan costs per billing interval. Prices are immutable
// once created — to change pricing, create a new price and deactivate the old
// one. Existing subscriptions keep the price they were sold at.
type Price struct {
	Domain string `json:"domain"` // "price"
	ID     string `json:"id"`     // nbo…prc
	PlanID string `json:"planId"`
	// UnitAmountInKobo is the amount per unit per interval, integer kobo
	// (₦1.00 = 100).
	UnitAmountInKobo Kobo               `json:"unitAmountInKobo"`
	Currency         string             `json:"currency"` // "NGN"
	Interval         PriceInterval      `json:"interval"`
	IntervalCount    int                `json:"intervalCount"`
	UsageType        PriceUsageType     `json:"usageType"`
	BillingScheme    PriceBillingScheme `json:"billingScheme"`
	TrialPeriodDays  int                `json:"trialPeriodDays"`
	Active           bool               `json:"active"`
	Metadata         Metadata           `json:"metadata"`
	Mode             Mode               `json:"mode"`
	CreatedAt        string             `json:"createdAt"`
}

// PriceCreateParams are the inputs to PlansService.Prices.Create.
type PriceCreateParams struct {
	// UnitAmountInKobo is the amount per unit per interval, integer kobo.
	// 250_000 is ₦2,500.00 — not ₦250,000. Multiply naira by 100 exactly once.
	UnitAmountInKobo Kobo          `json:"unitAmountInKobo"`
	Interval         PriceInterval `json:"interval"`
	// IntervalCount bills every N intervals. Defaults to 1 server-side.
	IntervalCount *int `json:"intervalCount,omitempty"`
	// UsageType defaults to licensed server-side.
	UsageType PriceUsageType `json:"usageType,omitempty"`
	// BillingScheme defaults to per_unit server-side (tiered is not yet
	// chargeable).
	BillingScheme PriceBillingScheme `json:"billingScheme,omitempty"`
	// TrialPeriodDays is free-trial days granted at subscribe time. Defaults
	// to 0 server-side.
	TrialPeriodDays *int     `json:"trialPeriodDays,omitempty"`
	Metadata        Metadata `json:"metadata,omitempty"`
}

// PriceListParams are the filters for PricesService.List. Note the wire filter
// name is planRef (not planId).
type PriceListParams struct {
	// PlanRef filters to one plan's prices (nbo…pln).
	PlanRef *string
	// Active filters by the active flag.
	Active *bool
	// Limit is the page size, 1–100 (API default 20).
	Limit *int
	// Cursor is an opaque cursor from a previous page's NextCursor.
	Cursor *string
}

func (p PriceListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryStr(q, "planRef", p.PlanRef)
	addQueryBool(q, "active", p.Active)
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// PricesService reads and deactivates prices. Create and list them under a
// plan via client.Plans.Prices.
type PricesService struct {
	client *Client
}

// Retrieve returns a price by id.
//
// Common errors: 404 PRICE_NOT_FOUND.
func (s *PricesService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*Price, error) {
	res, err := execute[Price](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/prices/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns prices across all plans, newest first.
//
//	page, err := client.Prices.List(ctx, nombaone.PriceListParams{
//		PlanRef: nombaone.String(plan.ID),
//		Active:  nombaone.Bool(true),
//	})
func (s *PricesService) List(ctx context.Context, params PriceListParams, opts ...RequestOption) (*Page[Price], error) {
	return executePage[Price](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/prices", query: params.toQuery(), opts: opts,
	})
}

// Deactivate deactivates a price so no new subscriptions can be created
// against it. Existing subscriptions are unaffected — prices are immutable
// history.
//
// Common errors: 409 PRICE_ALREADY_INACTIVE.
func (s *PricesService) Deactivate(ctx context.Context, id string, opts ...RequestOption) (*Price, error) {
	res, err := execute[Price](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/prices/" + seg(id) + "/deactivate", body: struct{}{}, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
