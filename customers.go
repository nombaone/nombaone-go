package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// Customer is a subscriber — the person or business you bill.
type Customer struct {
	Domain string `json:"domain"` // "customer"
	// ID is the public reference, e.g. "nbo123456789012cus".
	ID string `json:"id"`
	// Email is unique within your organization and environment.
	Email     string   `json:"email"`
	Name      string   `json:"name"`
	Phone     *string  `json:"phone"`
	Metadata  Metadata `json:"metadata"`
	Mode      Mode     `json:"mode"`
	CreatedAt string   `json:"createdAt"`
	UpdatedAt string   `json:"updatedAt"`
}

// CreditSource is where a credit grant came from. Only Manual and Goodwill are
// valid at grant time; the others appear on system-created grants.
type CreditSource string

const (
	CreditSourceManual             CreditSource = "manual"
	CreditSourceGoodwill           CreditSource = "goodwill"
	CreditSourceDowngradeProration CreditSource = "downgrade_proration"
	CreditSourceCoupon             CreditSource = "coupon"
)

// CreditGrant is a grant of credit that future invoices draw down — oldest
// grant first — before charging any payment rail.
type CreditGrant struct {
	Domain     string `json:"domain"` // "credit_grant"
	ID         string `json:"id"`     // nbo…crg
	CustomerID string `json:"customerId"`
	// AmountInKobo is the original granted amount, integer kobo (₦1.00 = 100).
	AmountInKobo Kobo `json:"amountInKobo"`
	// RemainingInKobo is what is left to consume, integer kobo.
	RemainingInKobo Kobo         `json:"remainingInKobo"`
	Source          CreditSource `json:"source"`
	SourceReference *string      `json:"sourceReference"`
	Mode            Mode         `json:"mode"`
	VoidedAt        *string      `json:"voidedAt"`
	CreatedAt       string       `json:"createdAt"`
}

// CreditBalance is a customer's live credit position: the total balance plus
// the grants behind it.
type CreditBalance struct {
	Domain     string `json:"domain"` // "credit_balance"
	CustomerID string `json:"customerId"`
	// BalanceInKobo is the sum of remaining credit across active grants,
	// integer kobo.
	BalanceInKobo Kobo          `json:"balanceInKobo"`
	Grants        []CreditGrant `json:"grants"`
}

// CustomerCreateParams are the inputs to CustomersService.Create.
type CustomerCreateParams struct {
	// Email must be unique per organization + environment
	// (CUSTOMER_EMAIL_TAKEN on reuse).
	Email    string   `json:"email"`
	Name     string   `json:"name"`
	Phone    *string  `json:"phone,omitempty"`
	Metadata Metadata `json:"metadata,omitempty"`
}

// CustomerUpdateParams are the inputs to CustomersService.Update. At least one
// field must be set.
type CustomerUpdateParams struct {
	Name *string `json:"name,omitempty"`
	// Phone is nullable: pass nombaone.Set("+234…") to change it, or
	// nombaone.Null[string]() to clear it. Leave nil to keep it unchanged.
	Phone    *Optional[string] `json:"phone,omitempty"`
	Metadata Metadata          `json:"metadata,omitempty"`
}

// CustomerListParams are the filters for CustomersService.List.
type CustomerListParams struct {
	// Email is an exact-match filter.
	Email *string
	// Limit is the page size, 1–100 (API default 20).
	Limit *int
	// Cursor is an opaque cursor from a previous page's NextCursor.
	Cursor *string
}

func (p CustomerListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryStr(q, "email", p.Email)
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// CustomerGrantCreditParams are the inputs to CustomersService.GrantCredit.
type CustomerGrantCreditParams struct {
	// AmountInKobo is the amount to grant, integer kobo (₦1.00 = 100).
	// 250_000 is ₦2,500 — not ₦250,000. Multiply naira by 100 exactly once,
	// at the edge of your system.
	AmountInKobo Kobo `json:"amountInKobo"`
	// Source defaults to Manual server-side; only Manual and Goodwill are
	// valid here.
	Source CreditSource `json:"source,omitempty"`
	// SourceReference is your own reference for this grant (support ticket,
	// promo id, …).
	SourceReference *string  `json:"sourceReference,omitempty"`
	Metadata        Metadata `json:"metadata,omitempty"`
}

// CustomerApplyDiscountParams are the inputs to CustomersService.ApplyDiscount.
type CustomerApplyDiscountParams struct {
	// Coupon is a coupon id (nbo…cpn) or its code (e.g. "LAUNCH20").
	Coupon string `json:"coupon"`
}

// CustomersService is the customers namespace — the people and businesses you
// bill, plus their credit and discounts.
type CustomersService struct {
	client *Client
}

// Create creates a customer.
//
// Common errors: 422 CLIENT_VALIDATION_FAILED (see APIError.Fields); 409
// CUSTOMER_EMAIL_TAKEN — reuse the existing customer instead.
//
//	customer, err := client.Customers.Create(ctx, nombaone.CustomerCreateParams{
//		Email:    "ada@example.com",
//		Name:     "Ada Lovelace",
//		Metadata: nombaone.Metadata{"crmId": "crm_812"},
//	})
//	// customer.ID == "nbo…cus"
func (s *CustomersService) Create(ctx context.Context, params CustomerCreateParams, opts ...RequestOption) (*Customer, error) {
	res, err := execute[Customer](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/customers", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Retrieve returns a customer by id.
//
// Common errors: 404 CUSTOMER_NOT_FOUND — check the id and that your key
// matches the environment the customer was created in.
//
//	customer, err := client.Customers.Retrieve(ctx, "nbo123456789012cus")
func (s *CustomersService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*Customer, error) {
	res, err := execute[Customer](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/customers/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Update changes a customer's mutable fields. At least one field is required.
//
//	_, err := client.Customers.Update(ctx, customer.ID, nombaone.CustomerUpdateParams{
//		Phone: nombaone.Set("+2348012345678"),
//	})
//	// clear it instead:
//	_, err = client.Customers.Update(ctx, customer.ID, nombaone.CustomerUpdateParams{
//		Phone: nombaone.Null[string](),
//	})
func (s *CustomersService) Update(ctx context.Context, id string, params CustomerUpdateParams, opts ...RequestOption) (*Customer, error) {
	res, err := execute[Customer](ctx, s.client, requestSpec{
		method: http.MethodPatch, path: "/customers/" + seg(id), body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns customers, newest first, as one page. Iterate every customer
// across all pages with the returned page's All method.
//
//	for customer, err := range client.Customers.List(ctx, nombaone.CustomerListParams{}).All(ctx) {
//		if err != nil { return err }
//		fmt.Println(customer.Email)
//	}
func (s *CustomersService) List(ctx context.Context, params CustomerListParams, opts ...RequestOption) (*Page[Customer], error) {
	return executePage[Customer](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/customers", query: params.toQuery(), opts: opts,
	})
}

// ApplyDiscount applies a coupon to a customer. The resulting discount shapes
// every future invoice for the customer until it ends or is removed.
//
// Common errors: 404 COUPON_NOT_FOUND; 409 COUPON_ALREADY_APPLIED.
//
//	discount, err := client.Customers.ApplyDiscount(ctx, customer.ID, nombaone.CustomerApplyDiscountParams{
//		Coupon: "LAUNCH20",
//	})
func (s *CustomersService) ApplyDiscount(ctx context.Context, id string, params CustomerApplyDiscountParams, opts ...RequestOption) (*Discount, error) {
	res, err := execute[Discount](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/customers/" + seg(id) + "/discount", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// RemoveDiscount removes the customer's active discount and returns the ended
// discount.
func (s *CustomersService) RemoveDiscount(ctx context.Context, id string, opts ...RequestOption) (*Discount, error) {
	res, err := execute[Discount](ctx, s.client, requestSpec{
		method: http.MethodDelete, path: "/customers/" + seg(id) + "/discount", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// GrantCredit grants credit to a customer. Credit is drawn down oldest-grant
// first by future invoices, before any payment rail is charged.
//
// This moves money-shaped state, so the API requires an Idempotency-Key; the
// SDK sends one automatically. Pass nombaone.WithIdempotencyKey to control it
// across process restarts.
//
//	_, err := client.Customers.GrantCredit(ctx, customer.ID, nombaone.CustomerGrantCreditParams{
//		AmountInKobo: 250_000, // ₦2,500.00
//		Source:       nombaone.CreditSourceGoodwill,
//	})
func (s *CustomersService) GrantCredit(ctx context.Context, id string, params CustomerGrantCreditParams, opts ...RequestOption) (*CreditGrant, error) {
	res, err := execute[CreditGrant](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/customers/" + seg(id) + "/credit", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// RetrieveCreditBalance returns the customer's credit balance and the grants
// behind it.
func (s *CustomersService) RetrieveCreditBalance(ctx context.Context, id string, opts ...RequestOption) (*CreditBalance, error) {
	res, err := execute[CreditBalance](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/customers/" + seg(id) + "/credit", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// VoidCredit voids a credit grant — its remaining balance becomes unusable.
// Already-consumed credit is untouched.
//
// Common errors: 409 CREDIT_GRANT_ALREADY_VOIDED.
func (s *CustomersService) VoidCredit(ctx context.Context, id, grantID string, opts ...RequestOption) (*CreditGrant, error) {
	res, err := execute[CreditGrant](ctx, s.client, requestSpec{
		method: http.MethodDelete, path: "/customers/" + seg(id) + "/credit/" + seg(grantID), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
