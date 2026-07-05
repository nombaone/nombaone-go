package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// PlanStatus is the lifecycle state of a plan.
type PlanStatus string

const (
	PlanStatusActive   PlanStatus = "active"
	PlanStatusArchived PlanStatus = "archived"
)

// Plan is what you sell — "Pro", "Starter". A plan holds the name and
// description; its prices (amount + cadence) live underneath it.
type Plan struct {
	Domain string `json:"domain"` // "plan"
	ID     string `json:"id"`     // nbo…pln
	// Name is unique within your organization (PLAN_NAME_TAKEN on reuse).
	Name        string     `json:"name"`
	Description *string    `json:"description"`
	Status      PlanStatus `json:"status"`
	Metadata    Metadata   `json:"metadata"`
	Mode        Mode       `json:"mode"`
	CreatedAt   string     `json:"createdAt"`
	UpdatedAt   string     `json:"updatedAt"`
}

// PlanCreateParams are the inputs to PlansService.Create.
type PlanCreateParams struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Metadata    Metadata `json:"metadata,omitempty"`
}

// PlanUpdateParams are the inputs to PlansService.Update. At least one field
// must be set.
type PlanUpdateParams struct {
	Name *string `json:"name,omitempty"`
	// Description is nullable: nombaone.Set("…") to change, nombaone.Null[string]()
	// to clear, nil to leave unchanged.
	Description *Optional[string] `json:"description,omitempty"`
	Metadata    Metadata          `json:"metadata,omitempty"`
}

// PlanListParams are the filters for PlansService.List.
type PlanListParams struct {
	Status PlanStatus
	Limit  *int
	Cursor *string
}

func (p PlanListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryEnum(q, "status", p.Status)
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// PlanPriceListParams are the filters for PlanPricesService.List.
type PlanPriceListParams struct {
	Limit  *int
	Cursor *string
}

func (p PlanPriceListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// PlanPricesService is the prices-under-a-plan sub-namespace (create/list);
// read and deactivate prices via client.Prices.
type PlanPricesService struct {
	client *Client
}

// Create creates a price under a plan. Prices are immutable once created.
//
//	price, err := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{
//		UnitAmountInKobo: 250_000, // ₦2,500.00 per month
//		Interval:         nombaone.PriceIntervalMonth,
//	})
func (s *PlanPricesService) Create(ctx context.Context, planID string, params PriceCreateParams, opts ...RequestOption) (*Price, error) {
	res, err := execute[Price](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/plans/" + seg(planID) + "/prices", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns a plan's prices, newest first.
func (s *PlanPricesService) List(ctx context.Context, planID string, params PlanPriceListParams, opts ...RequestOption) (*Page[Price], error) {
	return executePage[Price](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/plans/" + seg(planID) + "/prices", query: params.toQuery(), opts: opts,
	})
}

// PlansService is the plans namespace — your catalog. Prices nest under
// Plans.Prices.
type PlansService struct {
	client *Client
	// Prices is the prices-under-a-plan sub-namespace.
	Prices *PlanPricesService
}

// Create creates a plan.
//
// Common errors: 409 PLAN_NAME_TAKEN.
//
//	plan, err := client.Plans.Create(ctx, nombaone.PlanCreateParams{Name: "Pro"})
func (s *PlansService) Create(ctx context.Context, params PlanCreateParams, opts ...RequestOption) (*Plan, error) {
	res, err := execute[Plan](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/plans", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Retrieve returns a plan by id.
//
// Common errors: 404 PLAN_NOT_FOUND.
func (s *PlansService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*Plan, error) {
	res, err := execute[Plan](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/plans/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Update changes a plan's mutable fields. At least one field is required.
func (s *PlansService) Update(ctx context.Context, id string, params PlanUpdateParams, opts ...RequestOption) (*Plan, error) {
	res, err := execute[Plan](ctx, s.client, requestSpec{
		method: http.MethodPatch, path: "/plans/" + seg(id), body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns plans, newest first.
func (s *PlansService) List(ctx context.Context, params PlanListParams, opts ...RequestOption) (*Page[Plan], error) {
	return executePage[Plan](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/plans", query: params.toQuery(), opts: opts,
	})
}

// Archive archives a plan — it stops being subscribable but its history stays.
//
// Common errors: 409 PLAN_ALREADY_ARCHIVED; 409 PLAN_HAS_ACTIVE_SUBSCRIBERS —
// migrate or cancel those subscriptions first.
func (s *PlansService) Archive(ctx context.Context, id string, opts ...RequestOption) (*Plan, error) {
	res, err := execute[Plan](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/plans/" + seg(id) + "/archive", body: struct{}{}, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
