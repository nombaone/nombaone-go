package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// SettlementStatus is the lifecycle state of a settlement.
type SettlementStatus string

const (
	SettlementStatusPending    SettlementStatus = "pending"
	SettlementStatusSettled    SettlementStatus = "settled"
	SettlementStatusReconciled SettlementStatus = "reconciled"
	SettlementStatusFailed     SettlementStatus = "failed"
	SettlementStatusRefunded   SettlementStatus = "refunded"
)

// RefundStatus is the lifecycle state of a refund.
type RefundStatus string

const (
	RefundStatusPending    RefundStatus = "pending"
	RefundStatusLedgerOnly RefundStatus = "ledger_only"
	RefundStatusSucceeded  RefundStatus = "succeeded"
	RefundStatusFailed     RefundStatus = "failed"
)

// PayoutStatus is the lifecycle state of a payout.
type PayoutStatus string

const (
	PayoutStatusPending      PayoutStatus = "pending"
	PayoutStatusLedgerPosted PayoutStatus = "ledger_posted"
	PayoutStatusSucceeded    PayoutStatus = "succeeded"
	PayoutStatusFailed       PayoutStatus = "failed"
)

// Settlement is the integer-kobo split of one collection into fee + tenant share.
type Settlement struct {
	Domain           string  `json:"domain"` // "settlement"
	ID               string  `json:"id"`     // nbo…stl
	InvoiceReference *string `json:"invoiceReference"`
	SubAccountRef    string  `json:"subAccountRef"`
	SplitReference   *string `json:"splitReference"`
	MerchantTxRef    string  `json:"merchantTxRef"`
	GrossInKobo      Kobo    `json:"grossInKobo"`
	// PlatformFeeInKobo is non-refundable.
	PlatformFeeInKobo Kobo             `json:"platformFeeInKobo"`
	NetToTenantInKobo Kobo             `json:"netToTenantInKobo"`
	Status            SettlementStatus `json:"status"`
	CreatedAt         string           `json:"createdAt"`
}

// Refund is a refund of a settlement's tenant share (the platform fee stays).
type Refund struct {
	Domain              string       `json:"domain"` // "refund"
	ID                  string       `json:"id"`     // nbo…ref
	SettlementReference string       `json:"settlementReference"`
	SubAccountRef       string       `json:"subAccountRef"`
	AmountInKobo        Kobo         `json:"amountInKobo"`
	Status              RefundStatus `json:"status"`
	ProviderReference   *string      `json:"providerReference"`
	CreatedAt           string       `json:"createdAt"`
}

// Payout is a withdrawal of settled funds to your bank account.
type Payout struct {
	Domain              string       `json:"domain"` // "payout"
	ID                  string       `json:"id"`     // nbo…pay
	SubAccountRef       string       `json:"subAccountRef"`
	AmountInKobo        Kobo         `json:"amountInKobo"`
	BankCode            string       `json:"bankCode"`
	AccountNumber       string       `json:"accountNumber"`
	ResolvedAccountName *string      `json:"resolvedAccountName"`
	Status              PayoutStatus `json:"status"`
	ProviderReference   *string      `json:"providerReference"`
	FailureReason       *string      `json:"failureReason"`
	CreatedAt           string       `json:"createdAt"`
}

// Escrow is your escrow lock and what is actually withdrawable right now.
type Escrow struct {
	Domain                string `json:"domain"` // "escrow"
	LockedInKobo          Kobo   `json:"lockedInKobo"`
	Since                 string `json:"since"`
	BalanceInKobo         Kobo   `json:"balanceInKobo"`
	MinWithdrawableInKobo Kobo   `json:"minWithdrawableInKobo"`
	AvailableInKobo       Kobo   `json:"availableInKobo"`
}

// SettlementListParams are the filters for SettlementsService.List.
type SettlementListParams struct {
	Status SettlementStatus
	Limit  *int
	Cursor *string
}

func (p SettlementListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryEnum(q, "status", p.Status)
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// SettlementRefundParams are the inputs to SettlementsService.Refund.
type SettlementRefundParams struct {
	// AmountInKobo defaults server-side to the full remaining refundable
	// amount when nil.
	AmountInKobo *Kobo `json:"amountInKobo,omitempty"`
}

// PayoutCreateParams are the inputs to SettlementsService.CreatePayout.
type PayoutCreateParams struct {
	// AmountInKobo is integer kobo (₦1.00 = 100).
	AmountInKobo Kobo `json:"amountInKobo"`
	// BankCode is the CBN 3-digit bank code.
	BankCode      string `json:"bankCode"`
	AccountNumber string `json:"accountNumber"`
}

// SettlementsService is the settlements namespace — where collected money
// lands, and how it leaves (refunds, payouts) under the escrow lock.
type SettlementsService struct {
	client *Client
}

// Retrieve returns a settlement by id.
//
// Common errors: 404 SETTLEMENT_NOT_FOUND.
func (s *SettlementsService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*Settlement, error) {
	res, err := execute[Settlement](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/settlements/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns settlements, newest first.
func (s *SettlementsService) List(ctx context.Context, params SettlementListParams, opts ...RequestOption) (*Page[Settlement], error) {
	return executePage[Settlement](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/settlements", query: params.toQuery(), opts: opts,
	})
}

// RetrieveEscrow returns your escrow lock and available-to-withdraw balance.
func (s *SettlementsService) RetrieveEscrow(ctx context.Context, opts ...RequestOption) (*Escrow, error) {
	res, err := execute[Escrow](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/settlements/escrow", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Refund refunds a settlement's tenant share. The platform fee is never
// refunded.
//
// Money moves here. The API requires an Idempotency-Key; the SDK sends one
// automatically, but pass your own stable nombaone.WithIdempotencyKey so a
// retry from a new process cannot refund twice.
//
// Common errors: 409 REFUND_ALREADY_REFUNDED; 422 REFUND_AMOUNT_EXCEEDS_NET.
func (s *SettlementsService) Refund(ctx context.Context, id string, params SettlementRefundParams, opts ...RequestOption) (*Refund, error) {
	res, err := execute[Refund](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/settlements/" + seg(id) + "/refund", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// CreatePayout withdraws settled funds to your bank account.
//
// Money moves here, and the Idempotency-Key doubles as the payout's durable
// merchantTxRef. Always pass an explicit, stable nombaone.WithIdempotencyKey
// (e.g. your own payout id) — an auto-generated key protects SDK-level
// retries, but a brand-new process retrying with a fresh key would create a
// second payout.
//
// Common errors: 409 ESCROW_LOCKED; 422 PAYOUT_EXCEEDS_AVAILABLE.
//
//	payout, err := client.Settlements.CreatePayout(ctx,
//		nombaone.PayoutCreateParams{AmountInKobo: 5_000_000, BankCode: "058", AccountNumber: "0123456789"},
//		nombaone.WithIdempotencyKey("payout-"+myPayoutRow.ID),
//	)
func (s *SettlementsService) CreatePayout(ctx context.Context, params PayoutCreateParams, opts ...RequestOption) (*Payout, error) {
	res, err := execute[Payout](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/settlements/payout", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
