package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// PaymentMethodKind is the rail a payment method uses. Card and mandate are
// pull rails (the engine initiates the debit); virtual_account is the push
// rail (the customer sends a transfer and the engine matches it).
type PaymentMethodKind string

const (
	PaymentMethodKindCard           PaymentMethodKind = "card"
	PaymentMethodKindMandate        PaymentMethodKind = "mandate"
	PaymentMethodKindVirtualAccount PaymentMethodKind = "virtual_account"
)

// PaymentMethodStatus is the lifecycle state of a payment method.
type PaymentMethodStatus string

const (
	PaymentMethodStatusSetupPending   PaymentMethodStatus = "setup_pending"
	PaymentMethodStatusConsentPending PaymentMethodStatus = "consent_pending"
	PaymentMethodStatusActive         PaymentMethodStatus = "active"
	PaymentMethodStatusRemoved        PaymentMethodStatus = "removed"
	PaymentMethodStatusExpired        PaymentMethodStatus = "expired"
)

// PaymentMethod is how a customer pays. It never contains a PAN or token.
type PaymentMethod struct {
	Domain     string              `json:"domain"` // "payment_method"
	ID         string              `json:"id"`     // nbo…pmt
	CustomerID string              `json:"customerId"`
	Kind       PaymentMethodKind   `json:"kind"`
	Status     PaymentMethodStatus `json:"status"`
	IsDefault  bool                `json:"isDefault"`
	Brand      *string             `json:"brand"`
	Last4      *string             `json:"last4"`
	ExpMonth   *int                `json:"expMonth"`
	ExpYear    *int                `json:"expYear"`
	Mode       Mode                `json:"mode"`
	CreatedAt  string              `json:"createdAt"`
	UpdatedAt  string              `json:"updatedAt"`
}

// CheckoutSetup is a hosted-checkout handoff: send the customer to CheckoutLink.
type CheckoutSetup struct {
	Domain    string `json:"domain"` // "checkout_setup"
	Reference string `json:"reference"`
	// CheckoutLink is the PCI-scoped hosted page where the customer enters
	// their card.
	CheckoutLink string `json:"checkoutLink"`
}

// VirtualAccount is a dedicated NUBAN the customer pushes transfers to.
type VirtualAccount struct {
	Domain        string `json:"domain"` // "virtual_account"
	Reference     string `json:"reference"`
	BankName      string `json:"bankName"`
	AccountNumber string `json:"accountNumber"`
	AccountName   string `json:"accountName"`
	AccountRef    string `json:"accountRef"`
}

// PaymentMethodSetupParams are the inputs to PaymentMethodsService.Setup.
type PaymentMethodSetupParams struct {
	// CustomerRef is the customer this card will belong to (nbo…cus).
	CustomerRef string `json:"customerRef"`
	// AmountInKobo is the validation charge, integer kobo (₦1.00 = 100).
	AmountInKobo Kobo `json:"amountInKobo"`
	// CallbackURL is where the hosted checkout returns the customer afterwards.
	CallbackURL string `json:"callbackUrl"`
}

// PaymentMethodVirtualAccountParams are the inputs to
// PaymentMethodsService.CreateVirtualAccount.
type PaymentMethodVirtualAccountParams struct {
	// CustomerRef is the customer to issue the account for (nbo…cus).
	CustomerRef string `json:"customerRef"`
	// ExpectedAmount is an optional expected-amount hint, integer kobo.
	ExpectedAmount *Kobo `json:"expectedAmount,omitempty"`
	// ExpiryDate is an optional ISO date the account should expire.
	ExpiryDate *string `json:"expiryDate,omitempty"`
}

// PaymentMethodListParams are the filters for PaymentMethodsService.List. Note
// the wire filter name is customerRef (not customerId).
type PaymentMethodListParams struct {
	// CustomerRef filters to one customer (nbo…cus).
	CustomerRef *string
	Limit       *int
	Cursor      *string
}

func (p PaymentMethodListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryStr(q, "customerRef", p.CustomerRef)
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// PaymentMethodsService is the payment-methods namespace — cards (via hosted
// checkout), direct-debit mandates (see client.Mandates), and virtual accounts
// for the transfer rail.
type PaymentMethodsService struct {
	client *Client
}

// Setup starts a hosted-checkout card capture. Card entry happens on the PCI
// hosted page — no card data ever touches your servers. The method appears as
// setup_pending until the customer completes checkout.
//
//	setup, err := client.PaymentMethods.Setup(ctx, nombaone.PaymentMethodSetupParams{
//		CustomerRef:  customer.ID,
//		AmountInKobo: 5_000, // ₦50 validation charge
//		CallbackURL:  "https://example.com/billing/return",
//	})
//	// redirect the customer to setup.CheckoutLink
func (s *PaymentMethodsService) Setup(ctx context.Context, params PaymentMethodSetupParams, opts ...RequestOption) (*CheckoutSetup, error) {
	res, err := execute[CheckoutSetup](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/payment-methods/setup", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// CreateVirtualAccount issues a dedicated virtual account (NUBAN) so the
// customer can pay by bank transfer. The engine matches inbound transfers to
// invoices by reference and exact integer-kobo amount.
func (s *PaymentMethodsService) CreateVirtualAccount(ctx context.Context, params PaymentMethodVirtualAccountParams, opts ...RequestOption) (*VirtualAccount, error) {
	res, err := execute[VirtualAccount](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/payment-methods/virtual-account", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Retrieve returns a payment method by id.
//
// Common errors: 404 PAYMENT_METHOD_NOT_FOUND.
func (s *PaymentMethodsService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*PaymentMethod, error) {
	res, err := execute[PaymentMethod](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/payment-methods/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns payment methods, newest first.
func (s *PaymentMethodsService) List(ctx context.Context, params PaymentMethodListParams, opts ...RequestOption) (*Page[PaymentMethod], error) {
	return executePage[PaymentMethod](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/payment-methods", query: params.toQuery(), opts: opts,
	})
}

// SetDefault makes this the customer's default payment method.
func (s *PaymentMethodsService) SetDefault(ctx context.Context, id string, opts ...RequestOption) (*PaymentMethod, error) {
	res, err := execute[PaymentMethod](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/payment-methods/" + seg(id) + "/default", body: struct{}{}, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Remove detaches a payment method. Subscriptions still billing against it will
// need a replacement (SUBSCRIPTION_PAYMENT_METHOD_REQUIRED at next charge
// otherwise).
func (s *PaymentMethodsService) Remove(ctx context.Context, id string, opts ...RequestOption) (*PaymentMethod, error) {
	res, err := execute[PaymentMethod](ctx, s.client, requestSpec{
		method: http.MethodDelete, path: "/payment-methods/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
