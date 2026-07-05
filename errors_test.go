package nombaone

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestAPIError_StatusMapsToTypedError(t *testing.T) {
	cases := []struct {
		status int
		assert func(error) bool
	}{
		{400, func(e error) bool { var x *BadRequestError; return errors.As(e, &x) }},
		{401, func(e error) bool { var x *AuthenticationError; return errors.As(e, &x) }},
		{403, func(e error) bool { var x *PermissionDeniedError; return errors.As(e, &x) }},
		{404, func(e error) bool { var x *NotFoundError; return errors.As(e, &x) }},
		{409, func(e error) bool { var x *ConflictError; return errors.As(e, &x) }},
		{422, func(e error) bool { var x *ValidationError; return errors.As(e, &x) }},
		{429, func(e error) bool { var x *RateLimitError; return errors.As(e, &x) }},
		{500, func(e error) bool { var x *ServerError; return errors.As(e, &x) }},
		{503, func(e error) bool { var x *ServerError; return errors.As(e, &x) }},
	}
	for _, tc := range cases {
		err := apiErrorFromResponse(tc.status, []byte(errEnvelope("SOME_CODE")), http.Header{})
		if !tc.assert(err) {
			t.Errorf("status %d: error %T did not match its expected typed error", tc.status, err)
		}
		var api *APIError
		if !errors.As(err, &api) {
			t.Errorf("status %d: errors.As(&APIError) failed for %T", tc.status, err)
		}
	}
}

func TestAPIError_MessageCarriesHint(t *testing.T) {
	err := apiErrorFromResponse(404, []byte(errEnvelope("CUSTOMER_NOT_FOUND")), http.Header{})
	if got := err.Error(); got == "" || !containsAll(got, "something went wrong", "try this instead") {
		t.Errorf("Error() = %q, want message and hint", got)
	}
}

func TestAPIError_FieldsAndCodeAndRequestID(t *testing.T) {
	body := `{"success":false,"statusCode":422,"error":{"code":"CLIENT_VALIDATION_FAILED","message":"invalid","hint":"fix fields","docUrl":"https://docs.nombaone.xyz/errors#CLIENT_VALIDATION_FAILED","fields":{"email":["Invalid email"]}},"meta":{"requestId":"req_123"}}`
	err := apiErrorFromResponse(422, []byte(body), http.Header{})
	var v *ValidationError
	if !errors.As(err, &v) {
		t.Fatalf("want ValidationError, got %T", err)
	}
	if v.Code != ErrCodeClientValidationFailed {
		t.Errorf("Code = %q", v.Code)
	}
	if v.RequestID != "req_123" {
		t.Errorf("RequestID = %q", v.RequestID)
	}
	if got := v.Fields["email"]; len(got) != 1 || got[0] != "Invalid email" {
		t.Errorf("Fields[email] = %v", got)
	}
	if v.DocURL == "" {
		t.Error("DocURL is empty; the SDK must pass the wire value through")
	}
}

func TestAPIError_NonJSONBodyDegradesGracefully(t *testing.T) {
	err := apiErrorFromResponse(502, []byte("<html>502 Bad Gateway</html>"), http.Header{})
	var se *ServerError
	if !errors.As(err, &se) {
		t.Fatalf("want ServerError, got %T", err)
	}
	if se.Code != ErrCodeSystemUpstreamError {
		t.Errorf("Code = %q, want SYSTEM_UPSTREAM_ERROR default", se.Code)
	}
	if se.Message == "" {
		t.Error("Message should have a default even for an unparseable body")
	}
}

func TestAPIError_DefaultCodePerStatus(t *testing.T) {
	// empty body → default code path
	cases := map[int]ErrorCode{
		400: ErrCodeClientInvalidRequest,
		401: ErrCodeAPIKeyInvalid,
		403: ErrCodeClientForbidden,
		404: ErrCodeClientResourceNotFound,
		409: ErrCodeClientConflict,
		422: ErrCodeClientValidationFailed,
		429: ErrCodeRateLimitExceeded,
		503: ErrCodeSystemUpstreamError,
		418: ErrCodeSystemInternalError,
	}
	for status, want := range cases {
		err := apiErrorFromResponse(status, []byte(""), http.Header{})
		var api *APIError
		if !errors.As(err, &api) {
			t.Fatalf("status %d: not an APIError: %T", status, err)
		}
		if api.Code != want {
			t.Errorf("status %d: Code = %q, want %q", status, api.Code, want)
		}
	}
}

func TestRateLimitError_ParsesHeaders(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "30")
	h.Set("X-RateLimit-Limit", "100")
	h.Set("X-RateLimit-Remaining", "5")
	err := apiErrorFromResponse(429, []byte(errEnvelope("RATE_LIMIT_EXCEEDED")), h)
	var rl *RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("want RateLimitError, got %T", err)
	}
	if rl.RetryAfter != 30 || rl.Limit != 100 || rl.Remaining != 5 {
		t.Errorf("RetryAfter=%d Limit=%d Remaining=%d, want 30/100/5", rl.RetryAfter, rl.Limit, rl.Remaining)
	}
}

func TestRequestIDFallsBackToHeader(t *testing.T) {
	h := http.Header{}
	h.Set("X-Request-Id", "req_from_header")
	body := `{"success":false,"statusCode":404,"error":{"code":"CUSTOMER_NOT_FOUND","message":"no"},"meta":{}}`
	err := apiErrorFromResponse(404, []byte(body), h)
	var api *APIError
	errors.As(err, &api)
	if api.RequestID != "req_from_header" {
		t.Errorf("RequestID = %q, want header fallback", api.RequestID)
	}
}

func TestRetryAfter_Parsing(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "12")
	if got := retryAfter(h); got != 12*time.Second {
		t.Errorf("seconds: got %v, want 12s", got)
	}

	future := time.Now().Add(5 * time.Second).UTC().Format(http.TimeFormat)
	h.Set("Retry-After", future)
	if got := retryAfter(h); got <= 0 || got > 6*time.Second {
		t.Errorf("http-date: got %v, want ~5s", got)
	}

	h.Set("Retry-After", "garbage")
	if got := retryAfter(h); got != 0 {
		t.Errorf("garbage: got %v, want 0", got)
	}

	h.Del("Retry-After")
	if got := retryAfter(h); got != 0 {
		t.Errorf("absent: got %v, want 0", got)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
