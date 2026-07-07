package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// SubscriptionStatus is the lifecycle state of a subscription. Involuntary
// churn is Canceled with CancellationReason "involuntary" — there is no
// separate "churned" status.
type SubscriptionStatus string

const (
	SubscriptionStatusIncomplete        SubscriptionStatus = "incomplete"
	SubscriptionStatusIncompleteExpired SubscriptionStatus = "incomplete_expired"
	SubscriptionStatusTrialing          SubscriptionStatus = "trialing"
	SubscriptionStatusActive            SubscriptionStatus = "active"
	SubscriptionStatusPastDue           SubscriptionStatus = "past_due"
	SubscriptionStatusPaused            SubscriptionStatus = "paused"
	SubscriptionStatusCanceled          SubscriptionStatus = "canceled"
)

// CollectionMethod is how a subscription's invoices are collected.
type CollectionMethod string

const (
	CollectionMethodChargeAutomatically CollectionMethod = "charge_automatically"
	CollectionMethodSendInvoice         CollectionMethod = "send_invoice"
)

// CancellationReason distinguishes voluntary cancellation from involuntary
// (dunning-exhausted) churn.
type CancellationReason string

const (
	CancellationReasonVoluntary   CancellationReason = "voluntary"
	CancellationReasonInvoluntary CancellationReason = "involuntary"
)

// CancelMode chooses when a cancellation takes effect.
type CancelMode string

const (
	CancelModeNow         CancelMode = "now"
	CancelModeAtPeriodEnd CancelMode = "at_period_end"
)

// ProrationBehavior chooses whether a mid-cycle change prorates.
type ProrationBehavior string

const (
	ProrationBehaviorCreateProrations ProrationBehavior = "create_prorations"
	ProrationBehaviorNone             ProrationBehavior = "none"
)

// SubscriptionItem is a priced line within a subscription.
type SubscriptionItem struct {
	ID       string `json:"id"`
	PriceID  string `json:"priceId"`
	Quantity int    `json:"quantity"`
}

// Subscription is one customer's recurring relationship with one price. The
// engine bills it every cycle, retries failures through dunning, and reports
// every transition as a webhook event.
type Subscription struct {
	Domain             string             `json:"domain"` // "subscription"
	ID                 string             `json:"id"`     // nbo…sub
	CustomerID         string             `json:"customerId"`
	PriceID            string             `json:"priceId"`
	Status             SubscriptionStatus `json:"status"`
	CollectionMethod   CollectionMethod   `json:"collectionMethod"`
	CurrentPeriodIndex int                `json:"currentPeriodIndex"`
	CurrentPeriodStart *string            `json:"currentPeriodStart"`
	CurrentPeriodEnd   *string            `json:"currentPeriodEnd"`
	TrialStart         *string            `json:"trialStart"`
	TrialEnd           *string            `json:"trialEnd"`
	CancelAtPeriodEnd  bool               `json:"cancelAtPeriodEnd"`
	CanceledAt         *string            `json:"canceledAt"`
	EndedAt            *string            `json:"endedAt"`
	// CancellationReason is set once a subscription is canceled: "voluntary"
	// or "involuntary".
	CancellationReason     *CancellationReason `json:"cancellationReason"`
	DefaultPaymentMethodID *string             `json:"defaultPaymentMethodId"`
	Items                  []SubscriptionItem  `json:"items"`
	LatestInvoiceID        *string             `json:"latestInvoiceId"`
	Currency               string              `json:"currency"` // "NGN"
	Mode                   Mode                `json:"mode"`
	CreatedAt              string              `json:"createdAt"`
}

// UpcomingInvoice is a preview of the next cycle's invoice — nothing is
// charged or stored.
type UpcomingInvoice struct {
	Domain          string            `json:"domain"` // "upcoming_invoice"
	SubscriptionID  string            `json:"subscriptionId"`
	PeriodIndex     int               `json:"periodIndex"`
	PeriodStart     string            `json:"periodStart"`
	PeriodEnd       string            `json:"periodEnd"`
	BillingReason   BillingReason     `json:"billingReason"`
	SubtotalInKobo  Kobo              `json:"subtotalInKobo"`
	TotalInKobo     Kobo              `json:"totalInKobo"`
	AmountDueInKobo Kobo              `json:"amountDueInKobo"`
	Currency        string            `json:"currency"` // "NGN"
	LineItems       []InvoiceLineItem `json:"lineItems"`
	Mode            Mode              `json:"mode"`
}

// ScheduleStatus is the lifecycle state of a subscription schedule.
type ScheduleStatus string

const (
	ScheduleStatusActive   ScheduleStatus = "active"
	ScheduleStatusReleased ScheduleStatus = "released"
	ScheduleStatusCanceled ScheduleStatus = "canceled"
)

// SchedulePhase is one phase of a subscription schedule.
type SchedulePhase struct {
	StartIndex int     `json:"startIndex"`
	PriceID    string  `json:"priceId"`
	Quantity   int     `json:"quantity"`
	ConsumedAt *string `json:"consumedAt"`
}

// SubscriptionSchedule is a queued change that applies at a period boundary
// instead of mid-cycle.
type SubscriptionSchedule struct {
	Domain         string          `json:"domain"` // "subscription_schedule"
	ID             string          `json:"id"`     // nbo…sch
	SubscriptionID string          `json:"subscriptionId"`
	Status         ScheduleStatus  `json:"status"`
	Phases         []SchedulePhase `json:"phases"`
	Mode           Mode            `json:"mode"`
	CreatedAt      string          `json:"createdAt"`
	UpdatedAt      string          `json:"updatedAt"`
}

// DunningAttemptStatus is the state of a single dunning attempt (or "none" for
// a dunning state with no open recovery).
type DunningAttemptStatus string

const (
	DunningAttemptStatusScheduled          DunningAttemptStatus = "scheduled"
	DunningAttemptStatusAttempting         DunningAttemptStatus = "attempting"
	DunningAttemptStatusSucceeded          DunningAttemptStatus = "succeeded"
	DunningAttemptStatusRescheduled        DunningAttemptStatus = "rescheduled"
	DunningAttemptStatusCardUpdateRequired DunningAttemptStatus = "card_update_required"
	DunningAttemptStatusExhausted          DunningAttemptStatus = "exhausted"
	DunningAttemptStatusNone               DunningAttemptStatus = "none"
)

// DunningBranch is the recovery strategy a dunning attempt took.
type DunningBranch string

const (
	DunningBranchReschedule         DunningBranch = "reschedule"
	DunningBranchCardUpdateRequired DunningBranch = "card_update_required"
	DunningBranchShortPath          DunningBranch = "short_path"
)

// DunningAttempt is one retry in a recovery run.
type DunningAttempt struct {
	Domain         string               `json:"domain"` // "dunning_attempt"
	ID             string               `json:"id"`     // nbo…dun
	AttemptNumber  int                  `json:"attemptNumber"`
	Status         DunningAttemptStatus `json:"status"`
	Branch         DunningBranch        `json:"branch"`
	RailKey        *string              `json:"railKey"`
	FailureReason  *string              `json:"failureReason"`
	GatewayMessage *string              `json:"gatewayMessage"`
	Outcome        *string              `json:"outcome"`
	ScheduledAt    string               `json:"scheduledAt"`
	ExecutedAt     *string              `json:"executedAt"`
	NextAttemptAt  *string              `json:"nextAttemptAt"`
	CreatedAt      string               `json:"createdAt"`
}

// DunningState is where a subscription stands in recovery. past_due is not
// canceled — read GraceAccessUntil before cutting a subscriber off.
type DunningState struct {
	Domain          string               `json:"domain"` // "dunning_state"
	SubscriptionRef string               `json:"subscriptionRef"`
	InvoiceRef      *string              `json:"invoiceRef"`
	Status          DunningAttemptStatus `json:"status"`
	AttemptsUsed    int                  `json:"attemptsUsed"`
	MaxAttempts     int                  `json:"maxAttempts"`
	NextAttemptAt   *string              `json:"nextAttemptAt"`
	// GraceAccessUntil is how long access should be honored while recovery
	// runs. Check it before cutting a past_due subscriber off.
	GraceAccessUntil *string          `json:"graceAccessUntil"`
	Attempts         []DunningAttempt `json:"attempts"`
}

// SubscriptionCreateParams are the inputs to SubscriptionsService.Create.
type SubscriptionCreateParams struct {
	CustomerID string `json:"customerId"`
	// PriceID is the price to subscribe to (nbo…prc) — subscriptions reference
	// a price, not a plan.
	PriceID string `json:"priceId"`
	// PaymentMethodID is required for charge_automatically unless TrialDays > 0
	// (the first charge is deferred to trial end).
	PaymentMethodID *string `json:"paymentMethodId,omitempty"`
	// CollectionMethod defaults to charge_automatically server-side.
	CollectionMethod CollectionMethod `json:"collectionMethod,omitempty"`
	TrialDays        *int             `json:"trialDays,omitempty"`
	// Quantity defaults to 1 server-side.
	Quantity *int     `json:"quantity,omitempty"`
	Metadata Metadata `json:"metadata,omitempty"`
}

// SubscriptionUpdateParams edits metadata or the default payment method only.
// For price/quantity/interval changes (which prorate) use Change. At least one
// field must be set.
type SubscriptionUpdateParams struct {
	DefaultPaymentMethodID *string  `json:"defaultPaymentMethodId,omitempty"`
	Metadata               Metadata `json:"metadata,omitempty"`
}

// SubscriptionListParams are the filters for SubscriptionsService.List.
type SubscriptionListParams struct {
	CustomerID *string
	Status     SubscriptionStatus
	Limit      *int
	Cursor     *string
}

func (p SubscriptionListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryStr(q, "customerId", p.CustomerID)
	addQueryEnum(q, "status", p.Status)
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// SubscriptionListEventsParams are the filters for SubscriptionsService.ListEvents.
type SubscriptionListEventsParams struct {
	Limit  *int
	Cursor *string
}

func (p SubscriptionListEventsParams) toQuery() url.Values {
	q := url.Values{}
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// SubscriptionCancelParams are the inputs to SubscriptionsService.Cancel.
type SubscriptionCancelParams struct {
	// Mode defaults to now server-side; at_period_end keeps access until the
	// cycle closes.
	Mode    CancelMode `json:"mode,omitempty"`
	Comment *string    `json:"comment,omitempty"`
}

// SubscriptionPauseParams are the inputs to SubscriptionsService.Pause.
type SubscriptionPauseParams struct {
	// MaxDays auto-resumes after this many days.
	MaxDays *int `json:"maxDays,omitempty"`
}

// SubscriptionResubscribeParams are the inputs to SubscriptionsService.Resubscribe.
type SubscriptionResubscribeParams struct {
	// PriceID defaults to the previous price.
	PriceID *string `json:"priceId,omitempty"`
	// PaymentMethodID defaults to the previous payment method.
	PaymentMethodID *string `json:"paymentMethodId,omitempty"`
}

// SubscriptionChangeParams are the inputs to SubscriptionsService.Change. At
// least one of PriceID, Quantity, or IntervalSwitch is required.
type SubscriptionChangeParams struct {
	PriceID        *string `json:"priceId,omitempty"`
	Quantity       *int    `json:"quantity,omitempty"`
	IntervalSwitch *bool   `json:"intervalSwitch,omitempty"`
	// ProrationBehavior defaults to create_prorations server-side; none skips
	// proration.
	ProrationBehavior ProrationBehavior `json:"prorationBehavior,omitempty"`
}

// SubscriptionUpdatePaymentMethodParams are the inputs to
// SubscriptionsService.UpdatePaymentMethod. Set exactly one of
// PaymentMethodReference or CheckoutToken.
type SubscriptionUpdatePaymentMethodParams struct {
	// PaymentMethodReference is an already-captured payment method (nbo…pmt).
	PaymentMethodReference *string `json:"paymentMethodReference,omitempty"`
	// CheckoutToken is a fresh hosted-checkout token — attaches and swaps
	// atomically.
	CheckoutToken *string `json:"checkoutToken,omitempty"`
}

// SubscriptionApplyDiscountParams are the inputs to
// SubscriptionsService.ApplyDiscount.
type SubscriptionApplyDiscountParams struct {
	// Coupon is a coupon id (nbo…cpn) or its code.
	Coupon string `json:"coupon"`
}

// SubscriptionScheduleCreateParams are the inputs to
// SubscriptionScheduleService.Create.
type SubscriptionScheduleCreateParams struct {
	// PriceID is the price to switch to at the boundary.
	PriceID  string `json:"priceId"`
	Quantity *int   `json:"quantity,omitempty"`
	// EffectiveAt defaults to next_cycle server-side (the only mode today).
	EffectiveAt string `json:"effectiveAt,omitempty"`
}

// DunningListAttemptsParams are the filters for
// SubscriptionDunningService.ListAttempts.
type DunningListAttemptsParams struct {
	Limit  *int
	Cursor *string
}

func (p DunningListAttemptsParams) toQuery() url.Values {
	q := url.Values{}
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// SubscriptionScheduleService is the schedule sub-namespace: next-cycle
// changes queued against a subscription.
type SubscriptionScheduleService struct {
	client *Client
}

// Create queues a change for the next cycle boundary — the safe way to switch
// billing intervals (mid-cycle interval proration is unsupported).
//
// Common errors: 409 SUBSCRIPTION_SCHEDULE_CONFLICT.
func (s *SubscriptionScheduleService) Create(ctx context.Context, subscriptionID string, params SubscriptionScheduleCreateParams, opts ...RequestOption) (*SubscriptionSchedule, error) {
	res, err := execute[SubscriptionSchedule](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/subscriptions/" + seg(subscriptionID) + "/schedule", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Retrieve returns the subscription's schedule.
//
// Common errors: 404 SUBSCRIPTION_SCHEDULE_NOT_FOUND.
func (s *SubscriptionScheduleService) Retrieve(ctx context.Context, subscriptionID string, opts ...RequestOption) (*SubscriptionSchedule, error) {
	res, err := execute[SubscriptionSchedule](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/subscriptions/" + seg(subscriptionID) + "/schedule", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Release cancels the pending schedule before it applies.
func (s *SubscriptionScheduleService) Release(ctx context.Context, subscriptionID string, opts ...RequestOption) (*SubscriptionSchedule, error) {
	res, err := execute[SubscriptionSchedule](ctx, s.client, requestSpec{
		method: http.MethodDelete, path: "/subscriptions/" + seg(subscriptionID) + "/schedule", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// SubscriptionDunningService is the dunning sub-namespace: a read-only view
// into a subscription's recovery state.
type SubscriptionDunningService struct {
	client *Client
}

// Retrieve returns where the subscription stands in dunning. Check
// GraceAccessUntil before cutting access — past_due usually means "not yet",
// not "no".
func (s *SubscriptionDunningService) Retrieve(ctx context.Context, subscriptionID string, opts ...RequestOption) (*DunningState, error) {
	res, err := execute[DunningState](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/subscriptions/" + seg(subscriptionID) + "/dunning", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// ListAttempts lists every recovery attempt, newest first.
func (s *SubscriptionDunningService) ListAttempts(ctx context.Context, subscriptionID string, params DunningListAttemptsParams, opts ...RequestOption) (*Page[DunningAttempt], error) {
	return executePage[DunningAttempt](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/subscriptions/" + seg(subscriptionID) + "/dunning/attempts", query: params.toQuery(), opts: opts,
	})
}

// SubscriptionsService is the subscriptions namespace — the core billing
// object. Create one against a customer and a price; the engine handles
// cycles, invoices, retries, and recovery. Scheduled changes live under
// Schedule; recovery state under Dunning.
type SubscriptionsService struct {
	client *Client
	// Schedule is the next-cycle scheduled-changes sub-namespace.
	Schedule *SubscriptionScheduleService
	// Dunning is the read-only recovery-state sub-namespace.
	Dunning *SubscriptionDunningService
}

// Create creates a subscription. This can move money (the first charge), so
// the API requires an Idempotency-Key; the SDK sends one automatically and
// reuses it across its own retries.
//
// Common errors: 422 on a missing payment method without a trial; 409
// SUBSCRIPTION_PAYMENT_METHOD_REQUIRED.
//
//	sub, err := client.Subscriptions.Create(ctx, nombaone.SubscriptionCreateParams{
//		CustomerID:      customer.ID,
//		PriceID:         price.ID,
//		PaymentMethodID: nombaone.String(method.ID),
//	})
//	// sub.Status == "active"
func (s *SubscriptionsService) Create(ctx context.Context, params SubscriptionCreateParams, opts ...RequestOption) (*Subscription, error) {
	res, err := execute[Subscription](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/subscriptions", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Retrieve returns a subscription by id.
//
// Common errors: 404 SUBSCRIPTION_NOT_FOUND.
func (s *SubscriptionsService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*Subscription, error) {
	res, err := execute[Subscription](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/subscriptions/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Update edits metadata or the default payment method. For
// price/quantity/interval changes use Change — those prorate.
func (s *SubscriptionsService) Update(ctx context.Context, id string, params SubscriptionUpdateParams, opts ...RequestOption) (*Subscription, error) {
	res, err := execute[Subscription](ctx, s.client, requestSpec{
		method: http.MethodPatch, path: "/subscriptions/" + seg(id), body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns subscriptions, newest first.
func (s *SubscriptionsService) List(ctx context.Context, params SubscriptionListParams, opts ...RequestOption) (*Page[Subscription], error) {
	return executePage[Subscription](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/subscriptions", query: params.toQuery(), opts: opts,
	})
}

// ListEvents returns the subscription's audit trail of domain events, newest
// first.
func (s *SubscriptionsService) ListEvents(ctx context.Context, id string, params SubscriptionListEventsParams, opts ...RequestOption) (*Page[DomainEvent], error) {
	return executePage[DomainEvent](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/subscriptions/" + seg(id) + "/events", query: params.toQuery(), opts: opts,
	})
}

// Pause pauses billing. The subscription keeps its place in the cycle and
// resumes cleanly.
//
// Common errors: 409 SUBSCRIPTION_ILLEGAL_TRANSITION.
func (s *SubscriptionsService) Pause(ctx context.Context, id string, params SubscriptionPauseParams, opts ...RequestOption) (*Subscription, error) {
	res, err := execute[Subscription](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/subscriptions/" + seg(id) + "/pause", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Resume resumes a paused subscription.
func (s *SubscriptionsService) Resume(ctx context.Context, id string, opts ...RequestOption) (*Subscription, error) {
	res, err := execute[Subscription](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/subscriptions/" + seg(id) + "/resume", body: struct{}{}, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Cancel cancels a subscription — immediately (default) or at period end.
//
//	_, err := client.Subscriptions.Cancel(ctx, sub.ID, nombaone.SubscriptionCancelParams{
//		Mode: nombaone.CancelModeAtPeriodEnd,
//	})
func (s *SubscriptionsService) Cancel(ctx context.Context, id string, params SubscriptionCancelParams, opts ...RequestOption) (*Subscription, error) {
	res, err := execute[Subscription](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/subscriptions/" + seg(id) + "/cancel", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Resubscribe starts a fresh subscription for a canceled one's customer,
// reusing the old price/payment method unless overridden. The subscription
// must be in a terminal state.
//
// Common errors: 409 SUBSCRIPTION_NOT_TERMINAL.
func (s *SubscriptionsService) Resubscribe(ctx context.Context, id string, params SubscriptionResubscribeParams, opts ...RequestOption) (*Subscription, error) {
	res, err := execute[Subscription](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/subscriptions/" + seg(id) + "/resubscribe", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Change changes price or quantity mid-cycle, prorating by default. Switching
// the billing interval mid-cycle is unsupported
// (PRORATION_INTERVAL_SWITCH_UNSUPPORTED) — queue it with Schedule.Create
// instead.
//
//	_, err := client.Subscriptions.Change(ctx, sub.ID, nombaone.SubscriptionChangeParams{
//		PriceID: nombaone.String(biggerPrice.ID), // upgrade, prorated
//	})
func (s *SubscriptionsService) Change(ctx context.Context, id string, params SubscriptionChangeParams, opts ...RequestOption) (*Subscription, error) {
	res, err := execute[Subscription](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/subscriptions/" + seg(id) + "/change", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// UpdatePaymentMethod swaps the payment method that bills this subscription —
// the card-update path during dunning. Set exactly one of
// PaymentMethodReference or CheckoutToken.
//
// It returns the attached [PaymentMethod] (not the subscription): the wire
// responds with the payment-method object even though the OpenAPI spec labels
// it a Subscription — verified against the live sandbox.
func (s *SubscriptionsService) UpdatePaymentMethod(ctx context.Context, id string, params SubscriptionUpdatePaymentMethodParams, opts ...RequestOption) (*PaymentMethod, error) {
	res, err := execute[PaymentMethod](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/subscriptions/" + seg(id) + "/payment-method", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// RetrieveUpcomingInvoice previews the next invoice without charging or storing
// anything.
func (s *SubscriptionsService) RetrieveUpcomingInvoice(ctx context.Context, id string, opts ...RequestOption) (*UpcomingInvoice, error) {
	res, err := execute[UpcomingInvoice](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/subscriptions/" + seg(id) + "/upcoming-invoice", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// ApplyDiscount applies a coupon to this subscription only.
func (s *SubscriptionsService) ApplyDiscount(ctx context.Context, id string, params SubscriptionApplyDiscountParams, opts ...RequestOption) (*Discount, error) {
	res, err := execute[Discount](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/subscriptions/" + seg(id) + "/discount", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// RemoveDiscount removes the subscription's active discount and returns the
// ended discount.
func (s *SubscriptionsService) RemoveDiscount(ctx context.Context, id string, opts ...RequestOption) (*Discount, error) {
	res, err := execute[Discount](ctx, s.client, requestSpec{
		method: http.MethodDelete, path: "/subscriptions/" + seg(id) + "/discount", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
