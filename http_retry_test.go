package nombaone

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestRetry_RecoversAfter5xx(t *testing.T) {
	noBackoff(t)
	m := newMock(
		scriptedResponse{status: http.StatusBadGateway, body: errEnvelope("SYSTEM_UPSTREAM_ERROR")},
		scriptedResponse{status: http.StatusOK, body: okEnvelope(`{"id":"nbo1"}`)},
	)
	c := testClient(t, m)

	res, err := execute[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/customers/x"})
	if err != nil {
		t.Fatalf("want success after retry, got %v", err)
	}
	if res.Data.ID != "nbo1" {
		t.Errorf("Data.ID = %q", res.Data.ID)
	}
	if m.callCount() != 2 {
		t.Errorf("callCount = %d, want 2", m.callCount())
	}
}

func TestRetry_RecoversAfter429(t *testing.T) {
	noBackoff(t)
	m := newMock(
		scriptedResponse{status: http.StatusTooManyRequests, body: errEnvelope("RATE_LIMIT_EXCEEDED")},
		scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")},
	)
	c := testClient(t, m)
	if _, err := execute[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/x"}); err != nil {
		t.Fatalf("want success, got %v", err)
	}
	if m.callCount() != 2 {
		t.Errorf("callCount = %d, want 2", m.callCount())
	}
}

func TestRetry_RecoversAfterNetworkError(t *testing.T) {
	noBackoff(t)
	m := newMock(
		scriptedResponse{err: errors.New("connection reset by peer")},
		scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")},
	)
	c := testClient(t, m)
	if _, err := execute[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/x"}); err != nil {
		t.Fatalf("want success after transport retry, got %v", err)
	}
	if m.callCount() != 2 {
		t.Errorf("callCount = %d, want 2", m.callCount())
	}
}

func TestRetry_RecoversAfterTimeout(t *testing.T) {
	noBackoff(t)
	m := newMock(
		scriptedResponse{status: http.StatusOK, body: okEnvelope("{}"), delay: 50 * time.Millisecond},
		scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")},
	)
	c := testClient(t, m)
	_, err := execute[testData](context.Background(), c, requestSpec{
		method: http.MethodGet, path: "/x",
		opts: []RequestOption{WithRequestTimeout(10 * time.Millisecond)},
	})
	if err != nil {
		t.Fatalf("want success after a timed-out attempt retried, got %v", err)
	}
	if m.callCount() != 2 {
		t.Errorf("callCount = %d, want 2", m.callCount())
	}
}

func TestRetry_TimeoutSurfacesAsTimeoutError(t *testing.T) {
	noBackoff(t)
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope("{}"), delay: 50 * time.Millisecond})
	c := testClient(t, m)
	_, err := execute[testData](context.Background(), c, requestSpec{
		method: http.MethodGet, path: "/x",
		opts: []RequestOption{WithRequestTimeout(10 * time.Millisecond), WithRequestMaxRetries(0)},
	})
	var te *TimeoutError
	if !errors.As(err, &te) {
		t.Fatalf("want *TimeoutError, got %T (%v)", err, err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("TimeoutError should unwrap to context.DeadlineExceeded")
	}
}

func TestRetry_NoRetryOnClient4xx(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusUnprocessableEntity, body: errEnvelope("CLIENT_VALIDATION_FAILED")})
	c := testClient(t, m)
	_, err := execute[testData](context.Background(), c, requestSpec{method: http.MethodPost, path: "/customers", body: map[string]any{}})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if m.callCount() != 1 {
		t.Errorf("callCount = %d, want 1 (4xx is never retried)", m.callCount())
	}
}

func TestRetry_409OnlyWhenIdempotencyInProgress(t *testing.T) {
	noBackoff(t)

	// IDEMPOTENCY_IN_PROGRESS is our own earlier attempt — retryable.
	inProgress := newMock(
		scriptedResponse{status: http.StatusConflict, body: errEnvelope("IDEMPOTENCY_IN_PROGRESS")},
		scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")},
	)
	c := testClient(t, inProgress)
	if _, err := execute[testData](context.Background(), c, requestSpec{method: http.MethodPost, path: "/subscriptions", body: map[string]any{}}); err != nil {
		t.Fatalf("IDEMPOTENCY_IN_PROGRESS should be retried: %v", err)
	}
	if inProgress.callCount() != 2 {
		t.Errorf("callCount = %d, want 2", inProgress.callCount())
	}

	// Any other 409 is a real conflict — not retried.
	conflict := newMock(scriptedResponse{status: http.StatusConflict, body: errEnvelope("CUSTOMER_EMAIL_TAKEN")})
	c2 := testClient(t, conflict)
	_, err := execute[testData](context.Background(), c2, requestSpec{method: http.MethodPost, path: "/customers", body: map[string]any{}})
	var ce *ConflictError
	if !errors.As(err, &ce) {
		t.Fatalf("want *ConflictError, got %T", err)
	}
	if conflict.callCount() != 1 {
		t.Errorf("callCount = %d, want 1 (a real 409 is not retried)", conflict.callCount())
	}
}

func TestRetry_UserCancellationIsNeverRetried(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")})
	c := testClient(t, m)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before the call

	_, err := execute[testData](ctx, c, requestSpec{method: http.MethodPost, path: "/subscriptions", body: map[string]any{}})
	var conn *ConnectionError
	if !errors.As(err, &conn) {
		t.Fatalf("want *ConnectionError for a canceled request, got %T (%v)", err, err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Error("ConnectionError should unwrap to context.Canceled")
	}
	if m.callCount() > 1 {
		t.Errorf("callCount = %d, want ≤1 (a cancellation is never retried)", m.callCount())
	}
}

func TestRetry_ExhaustsBudgetThenReturnsLastError(t *testing.T) {
	noBackoff(t)
	m := newMock(scriptedResponse{status: http.StatusInternalServerError, body: errEnvelope("SYSTEM_INTERNAL_ERROR")})
	c := testClient(t, m, WithMaxRetries(2))
	_, err := execute[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/x"})
	var se *ServerError
	if !errors.As(err, &se) {
		t.Fatalf("want *ServerError after exhausting retries, got %T", err)
	}
	if m.callCount() != 3 { // 1 + 2 retries
		t.Errorf("callCount = %d, want 3", m.callCount())
	}
}

func TestSuccess_Any2xxUnwrapsData(t *testing.T) {
	// Creates return 201; the SDK treats any 2xx as success.
	m := newMock(scriptedResponse{status: http.StatusCreated, body: okEnvelope(`{"id":"nbo123456789012cus","email":"ada@example.com"}`)})
	c := testClient(t, m)
	res, err := execute[testData](context.Background(), c, requestSpec{method: http.MethodPost, path: "/customers", body: map[string]any{}})
	if err != nil {
		t.Fatalf("201 should be success: %v", err)
	}
	if res.Data.ID != "nbo123456789012cus" || res.Data.Email != "ada@example.com" {
		t.Errorf("unwrapped data = %+v", res.Data)
	}
	if res.RequestID != "req_mock" {
		t.Errorf("RequestID = %q, want req_mock", res.RequestID)
	}
}

func TestSuccess_RawResponseCapture(t *testing.T) {
	h := http.Header{}
	h.Set("X-Request-Id", "req_raw")
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope("{}"), header: h})
	c := testClient(t, m)

	var raw *http.Response
	_, err := execute[testData](context.Background(), c, requestSpec{
		method: http.MethodGet, path: "/x",
		opts: []RequestOption{WithRawResponse(&raw)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if raw == nil {
		t.Fatal("raw response was not captured")
	}
	if raw.Header.Get("X-Request-Id") != "req_raw" {
		t.Errorf("captured header X-Request-Id = %q", raw.Header.Get("X-Request-Id"))
	}
}

func TestRequest_AppliesV1PrefixOnce(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")})
	c := testClient(t, m)
	_, _ = execute[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/customers/nbo1"})
	if got := m.calls[0].Path; got != "/v1/customers/nbo1" {
		t.Errorf("path = %q, want /v1/customers/nbo1", got)
	}
}
