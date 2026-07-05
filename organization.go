package nombaone

import (
	"context"
	"net/http"
)

// SettlementMode is how a tenant's collected funds are settled.
type SettlementMode string

const (
	SettlementModeSplitAtCollection SettlementMode = "split_at_collection"
	SettlementModeCollectThenPayout SettlementMode = "collect_then_payout"
)

// ProrationCreditPolicy is how downgrade proration credit is handled.
type ProrationCreditPolicy string

const (
	ProrationCreditPolicyCreditNextCycle ProrationCreditPolicy = "credit_next_cycle"
	ProrationCreditPolicyNone            ProrationCreditPolicy = "none"
)

// OrgBranding is a tenant's display branding.
type OrgBranding struct {
	DisplayName     string `json:"displayName,omitempty"`
	SupportEmail    string `json:"supportEmail,omitempty"`
	LogoURL         string `json:"logoUrl,omitempty"`
	PrimaryColorHex string `json:"primaryColorHex,omitempty"`
}

// OrgPlatformFee is a tenant's platform-fee configuration.
type OrgPlatformFee struct {
	Bps       *int  `json:"bps"`
	MinInKobo *Kobo `json:"minInKobo"`
	MaxInKobo *Kobo `json:"maxInKobo"`
}

// OrgGrace is a tenant's grace/dunning summary.
type OrgGrace struct {
	GracePeriodHours   int `json:"gracePeriodHours"`
	DunningMaxAttempts int `json:"dunningMaxAttempts"`
}

// OrgBilling is the billing block of tenant settings.
type OrgBilling struct {
	RateLimitPerMinute  *int           `json:"rateLimitPerMinute"`
	MonthlyRequestQuota *int           `json:"monthlyRequestQuota"`
	SettlementMode      SettlementMode `json:"settlementMode"`
	PlatformFee         OrgPlatformFee `json:"platformFee"`
	Grace               OrgGrace       `json:"grace"`
	Branding            OrgBranding    `json:"branding"`
}

// OrgWebhook is the webhook block of tenant settings.
type OrgWebhook struct {
	URL                 *string `json:"url"`
	SigningSecretPrefix *string `json:"signingSecretPrefix"`
	Configured          bool    `json:"configured"`
}

// OrgNombaAccount is the underlying Nomba account block of tenant settings.
type OrgNombaAccount struct {
	AccountRef *string `json:"accountRef"`
	Status     *string `json:"status"`
}

// TenantSettings is org-level configuration: limits, settlement mode,
// branding, webhook, and Nomba account status.
type TenantSettings struct {
	Domain       string          `json:"domain"` // "organization"
	Billing      OrgBilling      `json:"billing"`
	Webhook      OrgWebhook      `json:"webhook"`
	NombaAccount OrgNombaAccount `json:"nombaAccount"`
}

// TenantSettingsUpdateParams are the inputs to OrganizationService.Update. At
// least one field must be set. Rate limits are operator-set (not here).
type TenantSettingsUpdateParams struct {
	MonthlyRequestQuota *int           `json:"monthlyRequestQuota,omitempty"`
	SettlementMode      SettlementMode `json:"settlementMode,omitempty"`
	Branding            *OrgBranding   `json:"branding,omitempty"`
}

// BillingSettings is the org-wide billing + dunning policy — how hard and when
// the engine retries, payday bias, grace windows, and collection defaults.
type BillingSettings struct {
	Domain                   string                `json:"domain"` // "billing_settings"
	PartialCollectionEnabled bool                  `json:"partialCollectionEnabled"`
	ProrationCreditPolicy    ProrationCreditPolicy `json:"prorationCreditPolicy"`
	DunningMaxAttempts       int                   `json:"dunningMaxAttempts"`
	DunningIntervalsHours    []int                 `json:"dunningIntervalsHours"`
	DunningMaxWindowHours    int                   `json:"dunningMaxWindowHours"`
	GracePeriodHours         int                   `json:"gracePeriodHours"`
	// PaydayDays are days of the month retries bias toward.
	PaydayDays              []int            `json:"paydayDays"`
	PaydayPullForwardDays   int              `json:"paydayPullForwardDays"`
	PaydayBiasEnabled       bool             `json:"paydayBiasEnabled"`
	DefaultCollectionMethod CollectionMethod `json:"defaultCollectionMethod"`
	CommsEnabled            bool             `json:"commsEnabled"`
}

// BillingSettingsUpdateParams are the inputs to OrganizationBillingService.Update.
// PUT semantics, but only the supplied keys change.
type BillingSettingsUpdateParams struct {
	PartialCollectionEnabled *bool                 `json:"partialCollectionEnabled,omitempty"`
	ProrationCreditPolicy    ProrationCreditPolicy `json:"prorationCreditPolicy,omitempty"`
	// DunningMaxAttempts is 1–10.
	DunningMaxAttempts    *int  `json:"dunningMaxAttempts,omitempty"`
	DunningIntervalsHours []int `json:"dunningIntervalsHours,omitempty"`
	// DunningMaxWindowHours must be ≥ the largest configured dunning interval.
	DunningMaxWindowHours *int `json:"dunningMaxWindowHours,omitempty"`
	GracePeriodHours      *int `json:"gracePeriodHours,omitempty"`
	// PaydayDays are days of month, 1–31.
	PaydayDays []int `json:"paydayDays,omitempty"`
	// PaydayPullForwardDays is 0–28.
	PaydayPullForwardDays   *int             `json:"paydayPullForwardDays,omitempty"`
	PaydayBiasEnabled       *bool            `json:"paydayBiasEnabled,omitempty"`
	DefaultCollectionMethod CollectionMethod `json:"defaultCollectionMethod,omitempty"`
	CommsEnabled            *bool            `json:"commsEnabled,omitempty"`
}

// OrganizationBillingService is the billing + dunning policy sub-namespace
// under /organization/billing.
type OrganizationBillingService struct {
	client *Client
}

// Retrieve reads the org's billing + dunning policy.
func (s *OrganizationBillingService) Retrieve(ctx context.Context, opts ...RequestOption) (*BillingSettings, error) {
	res, err := execute[BillingSettings](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/organization/billing", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Update updates the billing policy. PUT semantics, but only supplied keys
// change.
//
//	_, err := client.Organization.Billing.Update(ctx, nombaone.BillingSettingsUpdateParams{
//		PaydayBiasEnabled: nombaone.Bool(true),
//		PaydayDays:        []int{25, 28, 30},
//	})
func (s *OrganizationBillingService) Update(ctx context.Context, params BillingSettingsUpdateParams, opts ...RequestOption) (*BillingSettings, error) {
	res, err := execute[BillingSettings](ctx, s.client, requestSpec{
		method: http.MethodPut, path: "/organization/billing", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// OrganizationService is the organization namespace — tenant configuration, not
// a billing object. Billing + dunning policy lives under Billing.
type OrganizationService struct {
	client *Client
	// Billing is the billing + dunning policy sub-namespace.
	Billing *OrganizationBillingService
}

// Retrieve reads org-level settings (limits, settlement mode, branding,
// statuses).
func (s *OrganizationService) Retrieve(ctx context.Context, opts ...RequestOption) (*TenantSettings, error) {
	res, err := execute[TenantSettings](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/organization", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Update updates tenant-editable settings. At least one field is required.
func (s *OrganizationService) Update(ctx context.Context, params TenantSettingsUpdateParams, opts ...RequestOption) (*TenantSettings, error) {
	res, err := execute[TenantSettings](ctx, s.client, requestSpec{
		method: http.MethodPut, path: "/organization", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
