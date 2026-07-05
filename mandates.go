package nombaone

import (
	"context"
	"net/http"
)

// MandateFrequency is the debit cadence a mandate authorizes.
type MandateFrequency string

const (
	MandateFrequencyVariable          MandateFrequency = "variable"
	MandateFrequencyWeekly            MandateFrequency = "weekly"
	MandateFrequencyEveryTwoWeeks     MandateFrequency = "every_two_weeks"
	MandateFrequencyMonthly           MandateFrequency = "monthly"
	MandateFrequencyEveryTwoMonths    MandateFrequency = "every_two_months"
	MandateFrequencyEveryThreeMonths  MandateFrequency = "every_three_months"
	MandateFrequencyEveryFourMonths   MandateFrequency = "every_four_months"
	MandateFrequencyEverySixMonths    MandateFrequency = "every_six_months"
	MandateFrequencyEveryTwelveMonths MandateFrequency = "every_twelve_months"
)

// MandateSetup is what mandate creation hands back — consent is still pending
// at this point.
type MandateSetup struct {
	Domain string `json:"domain"` // "mandate_setup"
	// Reference is the payment-method reference this mandate will live under
	// (nbo…pmt).
	Reference string `json:"reference"`
	// MandateRef is the provider-side mandate reference.
	MandateRef string `json:"mandateRef"`
	// Status is consent_pending until the customer's bank confirms.
	Status string `json:"status"`
	// ConsentInstruction is human instructions to relay to the customer to
	// authorize the debit.
	ConsentInstruction string `json:"consentInstruction"`
}

// MandateCreateParams are the inputs to MandatesService.Create.
type MandateCreateParams struct {
	// CustomerRef is the customer this mandate belongs to (nbo…cus).
	CustomerRef           string `json:"customerRef"`
	CustomerAccountNumber string `json:"customerAccountNumber"`
	// BankCode is the CBN 3-digit bank code (058 GTB · 044 Access · 033 UBA · …).
	BankCode            string `json:"bankCode"`
	CustomerName        string `json:"customerName"`
	CustomerAccountName string `json:"customerAccountName"`
	CustomerPhoneNumber string `json:"customerPhoneNumber"`
	CustomerAddress     string `json:"customerAddress"`
	// Narration is shown on the customer's statement.
	Narration string `json:"narration"`
	// MaxAmountInKobo is the hard per-debit ceiling, integer kobo (₦1.00 = 100).
	// Charges above it fail with MANDATE_MAX_AMOUNT_EXCEEDED.
	MaxAmountInKobo Kobo `json:"maxAmountInKobo"`
	// Frequency defaults to monthly server-side.
	Frequency MandateFrequency `json:"frequency,omitempty"`
	// StartDate is a local date-time (no zone). Defaults to tomorrow server-side.
	StartDate *string `json:"startDate,omitempty"`
	// EndDate is a local date-time (no zone). Defaults to one year out server-side.
	EndDate *string `json:"endDate,omitempty"`
}

// MandatesService is the mandates namespace — direct-debit mandates (NIBSS).
// Creation is asynchronous: the mandate starts consent_pending and activates
// only after the customer authorizes it with their bank. Listen for
// payment_method.attached / payment_method.updated; don't poll, and don't
// charge before it is active (MANDATE_NOT_ACTIVE / MANDATE_CONSENT_PENDING).
type MandatesService struct {
	client *Client
}

// Create creates a mandate. Requires an Idempotency-Key (sent automatically).
//
//	mandate, err := client.Mandates.Create(ctx, nombaone.MandateCreateParams{
//		CustomerRef:           customer.ID,
//		CustomerAccountNumber: "0123456789",
//		BankCode:              "058",
//		CustomerName:          "Ada Lovelace",
//		CustomerAccountName:   "Ada Lovelace",
//		CustomerPhoneNumber:   "+2348012345678",
//		CustomerAddress:       "1 Marina, Lagos",
//		Narration:             "Acme Pro subscription",
//		MaxAmountInKobo:       500_000, // ₦5,000 ceiling per debit
//	})
//	// relay mandate.ConsentInstruction to the customer, then wait for the webhook
func (s *MandatesService) Create(ctx context.Context, params MandateCreateParams, opts ...RequestOption) (*MandateSetup, error) {
	res, err := execute[MandateSetup](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/mandates", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Retrieve checks a mandate's current standing. It returns the underlying
// PaymentMethod (its Status moves consent_pending → active) — not a mandate
// object.
func (s *MandatesService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*PaymentMethod, error) {
	res, err := execute[PaymentMethod](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/mandates/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
