package nombaone

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// ErrorCode is a stable, machine-readable error identifier. Branch on it (it
// never changes for a given failure) rather than on the human-readable
// message. The set below is vendored from the platform's PUBLIC_ERROR_CODES,
// but the type is an open string: a code the API adds tomorrow still parses
// into an [APIError] today — it simply will not match one of these constants.
type ErrorCode string

// The public error codes the API can emit. Kept in sync with
// packages/errors/src/codes.ts (PUBLIC_ERROR_CODES).
const (
	// Generic request errors.
	ErrCodeClientInvalidRequest   ErrorCode = "CLIENT_INVALID_REQUEST"
	ErrCodeClientValidationFailed ErrorCode = "CLIENT_VALIDATION_FAILED"
	ErrCodeClientForbidden        ErrorCode = "CLIENT_FORBIDDEN"
	ErrCodeClientRouteNotFound    ErrorCode = "CLIENT_ROUTE_NOT_FOUND"
	ErrCodeClientResourceNotFound ErrorCode = "CLIENT_RESOURCE_NOT_FOUND"
	ErrCodeClientConflict         ErrorCode = "CLIENT_CONFLICT"
	ErrCodeInvalidCursor          ErrorCode = "INVALID_CURSOR"

	// API-key authentication.
	ErrCodeAPIKeyMissing             ErrorCode = "API_KEY_MISSING"
	ErrCodeAPIKeyInvalid             ErrorCode = "API_KEY_INVALID"
	ErrCodeAPIKeyScopeForbidden      ErrorCode = "API_KEY_SCOPE_FORBIDDEN"
	ErrCodeAPIKeyEnvironmentMismatch ErrorCode = "API_KEY_ENVIRONMENT_MISMATCH"

	// Idempotency.
	ErrCodeIdempotencyKeyMissing ErrorCode = "IDEMPOTENCY_KEY_MISSING"
	ErrCodeIdempotencyKeyReused  ErrorCode = "IDEMPOTENCY_KEY_REUSED"
	ErrCodeIdempotencyInProgress ErrorCode = "IDEMPOTENCY_IN_PROGRESS"

	// Rate limiting and platform.
	ErrCodeRateLimitExceeded   ErrorCode = "RATE_LIMIT_EXCEEDED"
	ErrCodeQuotaExceeded       ErrorCode = "QUOTA_EXCEEDED"
	ErrCodePlatformMaintenance ErrorCode = "PLATFORM_MAINTENANCE"

	// Webhooks.
	ErrCodeWebhookSignatureInvalid ErrorCode = "WEBHOOK_SIGNATURE_INVALID"

	// Customers.
	ErrCodeCustomerNotFound   ErrorCode = "CUSTOMER_NOT_FOUND"
	ErrCodeCustomerEmailTaken ErrorCode = "CUSTOMER_EMAIL_TAKEN"

	// Plans and prices.
	ErrCodePlanNotFound             ErrorCode = "PLAN_NOT_FOUND"
	ErrCodePlanNameTaken            ErrorCode = "PLAN_NAME_TAKEN"
	ErrCodePlanAlreadyArchived      ErrorCode = "PLAN_ALREADY_ARCHIVED"
	ErrCodePlanHasActiveSubscribers ErrorCode = "PLAN_HAS_ACTIVE_SUBSCRIBERS"
	ErrCodePriceNotFound            ErrorCode = "PRICE_NOT_FOUND"
	ErrCodePricePlanMismatch        ErrorCode = "PRICE_PLAN_MISMATCH"
	ErrCodePriceAlreadyInactive     ErrorCode = "PRICE_ALREADY_INACTIVE"
	ErrCodePriceTieredNotSupported  ErrorCode = "PRICE_TIERED_NOT_SUPPORTED"

	// Payment methods and mandates.
	ErrCodePaymentMethodNotFound     ErrorCode = "PAYMENT_METHOD_NOT_FOUND"
	ErrCodePaymentMethodNotActive    ErrorCode = "PAYMENT_METHOD_NOT_ACTIVE"
	ErrCodePaymentMethodKindMismatch ErrorCode = "PAYMENT_METHOD_KIND_MISMATCH"
	ErrCodeMandateNotActive          ErrorCode = "MANDATE_NOT_ACTIVE"
	ErrCodeMandateMaxAmountExceeded  ErrorCode = "MANDATE_MAX_AMOUNT_EXCEEDED"
	ErrCodeMandateConsentPending     ErrorCode = "MANDATE_CONSENT_PENDING"

	// Subscriptions and invoices.
	ErrCodeSubscriptionNotFound              ErrorCode = "SUBSCRIPTION_NOT_FOUND"
	ErrCodeSubscriptionIllegalTransition     ErrorCode = "SUBSCRIPTION_ILLEGAL_TRANSITION"
	ErrCodeSubscriptionVersionConflict       ErrorCode = "SUBSCRIPTION_VERSION_CONFLICT"
	ErrCodeSubscriptionNotTerminal           ErrorCode = "SUBSCRIPTION_NOT_TERMINAL"
	ErrCodeSubscriptionPaymentMethodRequired ErrorCode = "SUBSCRIPTION_PAYMENT_METHOD_REQUIRED"
	ErrCodeInvoiceNotFound                   ErrorCode = "INVOICE_NOT_FOUND"
	ErrCodeInvoiceAlreadyFinalized           ErrorCode = "INVOICE_ALREADY_FINALIZED"
	ErrCodeInvoiceAlreadyPaid                ErrorCode = "INVOICE_ALREADY_PAID"
	ErrCodeInvoiceNotVoidable                ErrorCode = "INVOICE_NOT_VOIDABLE"

	// Schedules and proration.
	ErrCodeSubscriptionScheduleNotFound           ErrorCode = "SUBSCRIPTION_SCHEDULE_NOT_FOUND"
	ErrCodeSubscriptionScheduleConflict           ErrorCode = "SUBSCRIPTION_SCHEDULE_CONFLICT"
	ErrCodeSubscriptionScheduleInvalidEffectiveAt ErrorCode = "SUBSCRIPTION_SCHEDULE_INVALID_EFFECTIVE_AT"
	ErrCodeProrationNotApplicable                 ErrorCode = "PRORATION_NOT_APPLICABLE"
	ErrCodeProrationIntervalSwitchUnsupported     ErrorCode = "PRORATION_INTERVAL_SWITCH_UNSUPPORTED"

	// Coupons, discounts, and credits.
	ErrCodeCouponNotFound              ErrorCode = "COUPON_NOT_FOUND"
	ErrCodeCouponExpired               ErrorCode = "COUPON_EXPIRED"
	ErrCodeCouponMaxRedemptionsReached ErrorCode = "COUPON_MAX_REDEMPTIONS_REACHED"
	ErrCodeCouponInvalidDefinition     ErrorCode = "COUPON_INVALID_DEFINITION"
	ErrCodeCouponAlreadyApplied        ErrorCode = "COUPON_ALREADY_APPLIED"
	ErrCodeDiscountNotFound            ErrorCode = "DISCOUNT_NOT_FOUND"
	ErrCodeCreditGrantNotFound         ErrorCode = "CREDIT_GRANT_NOT_FOUND"
	ErrCodeCreditGrantAlreadyVoided    ErrorCode = "CREDIT_GRANT_ALREADY_VOIDED"
	ErrCodeCreditInsufficientBalance   ErrorCode = "CREDIT_INSUFFICIENT_BALANCE"
	ErrCodeCreditInvalidAmount         ErrorCode = "CREDIT_INVALID_AMOUNT"

	// Dunning.
	ErrCodeDunningNoOpenInvoice      ErrorCode = "DUNNING_NO_OPEN_INVOICE"
	ErrCodeDunningAttemptNotFound    ErrorCode = "DUNNING_ATTEMPT_NOT_FOUND"
	ErrCodeDunningCardUpdateRequired ErrorCode = "DUNNING_CARD_UPDATE_REQUIRED"
	ErrCodeDunningAlreadyTerminal    ErrorCode = "DUNNING_ALREADY_TERMINAL"

	// Settlement, refunds, and payouts.
	ErrCodeSettlementNotFound           ErrorCode = "SETTLEMENT_NOT_FOUND"
	ErrCodeSettlementSubaccountNotFound ErrorCode = "SETTLEMENT_SUBACCOUNT_NOT_FOUND"
	ErrCodeRefundAlreadyRefunded        ErrorCode = "REFUND_ALREADY_REFUNDED"
	ErrCodeRefundAmountExceedsNet       ErrorCode = "REFUND_AMOUNT_EXCEEDS_NET"
	ErrCodeEscrowLocked                 ErrorCode = "ESCROW_LOCKED"
	ErrCodePayoutExceedsAvailable       ErrorCode = "PAYOUT_EXCEEDS_AVAILABLE"

	// Example scaffold.
	ErrCodeExampleNotFound ErrorCode = "EXAMPLE_NOT_FOUND"

	// System fallbacks.
	ErrCodeSystemInternalError ErrorCode = "SYSTEM_INTERNAL_ERROR"
	ErrCodeSystemUpstreamError ErrorCode = "SYSTEM_UPSTREAM_ERROR"
)

// APIError is a non-2xx response from the API. It carries everything the error
// envelope said: the stable [ErrorCode] to branch on, the human Message, an
// actionable Hint telling you exactly what to do next, a DocURL deep-linking
// into the error reference, per-field validation errors on 422s, and the
// RequestID to quote to support.
//
// Every failed API call returns a status-specific type that embeds *APIError
// ([BadRequestError], [AuthenticationError], [NotFoundError], [RateLimitError],
// …). Match either the specific type or the base with errors.As:
//
//	if _, err := client.Customers.Retrieve(ctx, id); err != nil {
//		var apiErr *nombaone.APIError
//		if errors.As(err, &apiErr) {
//			log.Printf("%s: %s (%s)", apiErr.Code, apiErr.Hint, apiErr.RequestID)
//		}
//		var notFound *nombaone.NotFoundError
//		if errors.As(err, &notFound) {
//			// handle a missing resource specifically
//		}
//	}
type APIError struct {
	// StatusCode is the HTTP status of the response.
	StatusCode int
	// Code is the stable, machine-readable error code — branch on this.
	Code ErrorCode
	// Message is the human-readable summary from the API.
	Message string
	// Hint is actionable guidance on exactly what to do next.
	Hint string
	// DocURL deep-links to this code's entry in the public error reference.
	DocURL string
	// Fields holds per-field validation errors (field path → messages),
	// present on 422 responses.
	Fields map[string][]string
	// RequestID identifies this request — quote it when contacting support.
	RequestID string
}

// Error renders the message with the hint appended, so the fix arrives with
// the failure without a docs tab.
func (e *APIError) Error() string {
	if e.Hint != "" {
		return e.Message + " — " + e.Hint
	}
	return e.Message
}

// BadRequestError is a 400 — the request could not be understood.
type BadRequestError struct{ *APIError }

// AuthenticationError is a 401 — missing, invalid, revoked, or
// wrong-environment API key.
type AuthenticationError struct{ *APIError }

// PermissionDeniedError is a 403 — a valid key that is not allowed (missing
// scope, foreign resource).
type PermissionDeniedError struct{ *APIError }

// NotFoundError is a 404 — no resource at that id in this environment.
type NotFoundError struct{ *APIError }

// ConflictError is a 409 — conflicts with current state (including
// idempotency reuse and in-progress).
type ConflictError struct{ *APIError }

// ValidationError is a 422 — one or more fields are invalid; see
// [APIError.Fields].
type ValidationError struct{ *APIError }

// RateLimitError is a 429 — slow down and retry after RetryAfter seconds.
type RateLimitError struct {
	*APIError
	// RetryAfter is the seconds until the current rate-limit window rolls
	// over (from the Retry-After header). Zero if the header was absent.
	RetryAfter int
	// Limit is your per-window request cap (X-RateLimit-Limit). Zero if
	// absent.
	Limit int
	// Remaining is the requests left in the current window
	// (X-RateLimit-Remaining). Zero if absent.
	Remaining int
}

// ServerError is a 5xx — something failed on NombaOne's side. It is safe to
// retry (the SDK already did, up to maxRetries).
type ServerError struct{ *APIError }

// Unwrap lets errors.As reach the embedded *APIError from any status-specific
// error, so errors.As(err, &apiErr) matches every API failure.
func (e *BadRequestError) Unwrap() error       { return e.APIError }
func (e *AuthenticationError) Unwrap() error   { return e.APIError }
func (e *PermissionDeniedError) Unwrap() error { return e.APIError }
func (e *NotFoundError) Unwrap() error         { return e.APIError }
func (e *ConflictError) Unwrap() error         { return e.APIError }
func (e *ValidationError) Unwrap() error       { return e.APIError }
func (e *RateLimitError) Unwrap() error        { return e.APIError }
func (e *ServerError) Unwrap() error           { return e.APIError }

// ConnectionError means the request never completed — DNS failure, connection
// reset, or a caller-initiated cancellation. A cancellation is never retried.
type ConnectionError struct {
	Message string
	Err     error
}

func (e *ConnectionError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap exposes the underlying transport/context error, so
// errors.Is(err, context.Canceled) works.
func (e *ConnectionError) Unwrap() error { return e.Err }

// TimeoutError means a single attempt exceeded its per-attempt timeout budget.
// It is retried automatically. It unwraps to context.DeadlineExceeded.
type TimeoutError struct {
	Message string
	Err     error
}

func (e *TimeoutError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap exposes context.DeadlineExceeded, so errors.Is works.
func (e *TimeoutError) Unwrap() error { return e.Err }

// errorEnvelope is the wire shape of an error response.
type errorEnvelope struct {
	Error struct {
		Code    string              `json:"code"`
		Message string              `json:"message"`
		Hint    string              `json:"hint"`
		DocURL  string              `json:"docUrl"`
		Fields  map[string][]string `json:"fields"`
	} `json:"error"`
	Meta envelopeMeta `json:"meta"`
}

// apiErrorFromResponse builds the right status-specific error from a non-2xx
// response and its (possibly non-JSON) body. A body that is not a usable error
// envelope degrades gracefully to the default code for the status — it never
// crashes the parser.
func apiErrorFromResponse(status int, body []byte, header http.Header) error {
	var env errorEnvelope
	_ = json.Unmarshal(body, &env) // tolerate non-JSON: fields stay zero

	code := ErrorCode(env.Error.Code)
	if code == "" {
		code = defaultCodeForStatus(status)
	}
	message := env.Error.Message
	if message == "" {
		message = fmt.Sprintf("request failed with status %d", status)
	}
	requestID := env.Meta.RequestID
	if requestID == "" {
		requestID = header.Get("X-Request-Id")
	}

	base := &APIError{
		StatusCode: status,
		Code:       code,
		Message:    message,
		Hint:       env.Error.Hint,
		DocURL:     env.Error.DocURL,
		Fields:     env.Error.Fields,
		RequestID:  requestID,
	}

	switch {
	case status == http.StatusBadRequest:
		return &BadRequestError{base}
	case status == http.StatusUnauthorized:
		return &AuthenticationError{base}
	case status == http.StatusForbidden:
		return &PermissionDeniedError{base}
	case status == http.StatusNotFound:
		return &NotFoundError{base}
	case status == http.StatusConflict:
		return &ConflictError{base}
	case status == http.StatusUnprocessableEntity:
		return &ValidationError{base}
	case status == http.StatusTooManyRequests:
		rl := &RateLimitError{APIError: base}
		if v := header.Get("Retry-After"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				rl.RetryAfter = n
			}
		}
		if v := header.Get("X-RateLimit-Limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				rl.Limit = n
			}
		}
		if v := header.Get("X-RateLimit-Remaining"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				rl.Remaining = n
			}
		}
		return rl
	case status >= 500:
		return &ServerError{base}
	default:
		return base
	}
}

// defaultCodeForStatus is the fallback code when the error body is unusable.
func defaultCodeForStatus(status int) ErrorCode {
	switch status {
	case http.StatusBadRequest:
		return ErrCodeClientInvalidRequest
	case http.StatusUnauthorized:
		return ErrCodeAPIKeyInvalid
	case http.StatusForbidden:
		return ErrCodeClientForbidden
	case http.StatusNotFound:
		return ErrCodeClientResourceNotFound
	case http.StatusConflict:
		return ErrCodeClientConflict
	case http.StatusUnprocessableEntity:
		return ErrCodeClientValidationFailed
	case http.StatusTooManyRequests:
		return ErrCodeRateLimitExceeded
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return ErrCodeSystemUpstreamError
	default:
		return ErrCodeSystemInternalError
	}
}
