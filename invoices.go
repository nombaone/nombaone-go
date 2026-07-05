package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// InvoiceStatus is the lifecycle state of an invoice. Note that partially_paid
// can appear on an invoice object but is not accepted by the list status
// filter.
type InvoiceStatus string

const (
	InvoiceStatusDraft         InvoiceStatus = "draft"
	InvoiceStatusOpen          InvoiceStatus = "open"
	InvoiceStatusPartiallyPaid InvoiceStatus = "partially_paid"
	InvoiceStatusPaid          InvoiceStatus = "paid"
	InvoiceStatusVoid          InvoiceStatus = "void"
	InvoiceStatusUncollectible InvoiceStatus = "uncollectible"
)

// Invoice is what a billing cycle produced. You never create invoices —
// subscription cycles do; amounts are locked at finalization. All amounts are
// integer kobo.
type Invoice struct {
	Domain                string            `json:"domain"` // "invoice"
	ID                    string            `json:"id"`     // nbo…inv
	CustomerID            string            `json:"customerId"`
	SubscriptionID        *string           `json:"subscriptionId"`
	Status                InvoiceStatus     `json:"status"`
	BillingReason         BillingReason     `json:"billingReason"`
	SubtotalInKobo        Kobo              `json:"subtotalInKobo"`
	DiscountTotalInKobo   Kobo              `json:"discountTotalInKobo"`
	CreditTotalInKobo     Kobo              `json:"creditTotalInKobo"`
	TotalInKobo           Kobo              `json:"totalInKobo"`
	AmountDueInKobo       Kobo              `json:"amountDueInKobo"`
	AmountPaidInKobo      Kobo              `json:"amountPaidInKobo"`
	AmountRemainingInKobo Kobo              `json:"amountRemainingInKobo"`
	Currency              string            `json:"currency"` // "NGN"
	PeriodStart           *string           `json:"periodStart"`
	PeriodEnd             *string           `json:"periodEnd"`
	DueDate               *string           `json:"dueDate"`
	LineItems             []InvoiceLineItem `json:"lineItems"`
	FinalizedAt           *string           `json:"finalizedAt"`
	PaidAt                *string           `json:"paidAt"`
	VoidedAt              *string           `json:"voidedAt"`
	Mode                  Mode              `json:"mode"`
	CreatedAt             string            `json:"createdAt"`
}

// InvoiceListParams are the filters for InvoicesService.List. The status
// filter accepts draft, open, paid, void, and uncollectible — not
// partially_paid.
type InvoiceListParams struct {
	CustomerID     *string
	SubscriptionID *string
	Status         InvoiceStatus
	Limit          *int
	Cursor         *string
}

func (p InvoiceListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryStr(q, "customerId", p.CustomerID)
	addQueryStr(q, "subscriptionId", p.SubscriptionID)
	addQueryEnum(q, "status", p.Status)
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// InvoiceVoidParams are the inputs to InvoicesService.Void.
type InvoiceVoidParams struct {
	Comment *string `json:"comment,omitempty"`
}

// InvoicesService reads what the billing engine produced and voids what should
// never be collected.
type InvoicesService struct {
	client *Client
}

// Retrieve returns an invoice by id.
//
// Common errors: 404 INVOICE_NOT_FOUND.
func (s *InvoicesService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*Invoice, error) {
	res, err := execute[Invoice](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/invoices/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns invoices, newest first.
//
//	for invoice, err := range client.Invoices.List(ctx, nombaone.InvoiceListParams{
//		Status: nombaone.InvoiceStatusOpen,
//	}).All(ctx) {
//		if err != nil { return err }
//		fmt.Println(invoice.ID, invoice.AmountDueInKobo)
//	}
func (s *InvoicesService) List(ctx context.Context, params InvoiceListParams, opts ...RequestOption) (*Page[Invoice], error) {
	return executePage[Invoice](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/invoices", query: params.toQuery(), opts: opts,
	})
}

// Void voids an open, unpaid invoice. Paid invoices cannot be voided — refund
// the settlement instead.
//
// Common errors: 409 INVOICE_NOT_VOIDABLE; 409 INVOICE_ALREADY_PAID.
func (s *InvoicesService) Void(ctx context.Context, id string, params InvoiceVoidParams, opts ...RequestOption) (*Invoice, error) {
	res, err := execute[Invoice](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/invoices/" + seg(id) + "/void", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
