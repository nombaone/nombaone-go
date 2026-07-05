package nombaone

import (
	"context"
	"net/http"
	"testing"
)

// testData is a throwaway target type for transport-level tests.
type testData struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func TestIdempotency_AutoGeneratesUUIDOnPost(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(`{"id":"nbo1"}`)})
	c := testClient(t, m)

	_, err := execute[testData](context.Background(), c, requestSpec{
		method: http.MethodPost, path: "/customers", body: map[string]any{"name": "Ada"},
	})
	if err != nil {
		t.Fatal(err)
	}
	key := m.calls[0].Header.Get("Idempotency-Key")
	if !uuidRE.MatchString(key) {
		t.Fatalf("Idempotency-Key = %q, want a v4 UUID", key)
	}
}

func TestIdempotency_ReusesSameKeyAcrossRetries(t *testing.T) {
	noBackoff(t)
	m := newMock(
		scriptedResponse{status: http.StatusInternalServerError, body: errEnvelope("SYSTEM_INTERNAL_ERROR")},
		scriptedResponse{status: http.StatusServiceUnavailable, body: errEnvelope("SYSTEM_UPSTREAM_ERROR")},
		scriptedResponse{status: http.StatusOK, body: okEnvelope(`{"id":"nbo1"}`)},
	)
	c := testClient(t, m) // default maxRetries = 2 → 3 attempts

	_, err := execute[testData](context.Background(), c, requestSpec{
		method: http.MethodPost, path: "/subscriptions", body: map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.callCount() != 3 {
		t.Fatalf("callCount = %d, want 3", m.callCount())
	}
	seen := map[string]struct{}{}
	for _, call := range m.calls {
		seen[call.Header.Get("Idempotency-Key")] = struct{}{}
	}
	if len(seen) != 1 {
		t.Fatalf("distinct idempotency keys across retries = %d, want 1 (the money-safety invariant)", len(seen))
	}
	for k := range seen {
		if !uuidRE.MatchString(k) {
			t.Fatalf("Idempotency-Key = %q, want a v4 UUID", k)
		}
	}
}

func TestIdempotency_FreshKeyPerLogicalCall(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")})
	c := testClient(t, m)

	ctx := context.Background()
	_, _ = execute[testData](ctx, c, requestSpec{method: http.MethodPost, path: "/customers", body: map[string]any{}})
	_, _ = execute[testData](ctx, c, requestSpec{method: http.MethodPost, path: "/customers", body: map[string]any{}})

	if m.calls[0].Header.Get("Idempotency-Key") == m.calls[1].Header.Get("Idempotency-Key") {
		t.Fatal("separate logical calls must use different idempotency keys")
	}
}

func TestIdempotency_HonorsExplicitKey(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")})
	c := testClient(t, m)

	_, err := execute[testData](context.Background(), c, requestSpec{
		method: http.MethodPost, path: "/settlements/payout",
		body: map[string]any{"amountInKobo": 100000},
		opts: []RequestOption{WithIdempotencyKey("payout-2026-07-04-001")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := m.calls[0].Header.Get("Idempotency-Key"); got != "payout-2026-07-04-001" {
		t.Fatalf("Idempotency-Key = %q, want the explicit override", got)
	}
}

func TestIdempotency_NotSentOnNonPost(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")})
	c := testClient(t, m)

	ctx := context.Background()
	_, _ = execute[testData](ctx, c, requestSpec{method: http.MethodGet, path: "/customers"})
	_, _ = execute[testData](ctx, c, requestSpec{method: http.MethodPatch, path: "/customers/x", body: map[string]any{}})
	_, _ = execute[testData](ctx, c, requestSpec{method: http.MethodDelete, path: "/customers/x/discount"})

	for i, call := range m.calls {
		if got := call.Header.Get("Idempotency-Key"); got != "" {
			t.Errorf("call %d (%s): unexpected Idempotency-Key %q", i, call.Method, got)
		}
	}
}
