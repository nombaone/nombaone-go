package nombaone

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// uuidRE matches an RFC 4122 version-4 UUID.
var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// recordedCall captures one request the mock transport saw.
type recordedCall struct {
	Method string
	URL    string
	Path   string
	Query  string
	Header http.Header
	Body   string
}

// scriptedResponse is one queued reply. Exactly one of {err, delay+status} is
// meaningful: err makes Do fail (a network error); delay makes Do wait
// (respecting the request context) before replying, to provoke a timeout.
type scriptedResponse struct {
	status int
	body   string
	header http.Header
	err    error
	delay  time.Duration
}

// mockTransport is an [HTTPClient] that records every call and replays a queued
// script of responses. Once the script is exhausted, the last response repeats.
type mockTransport struct {
	mu        sync.Mutex
	responses []scriptedResponse
	calls     []recordedCall
	idx       int
}

func newMock(responses ...scriptedResponse) *mockTransport {
	return &mockTransport{responses: responses}
}

func (m *mockTransport) Do(req *http.Request) (*http.Response, error) {
	var body string
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = string(b)
	}

	m.mu.Lock()
	m.calls = append(m.calls, recordedCall{
		Method: req.Method,
		URL:    req.URL.String(),
		Path:   req.URL.EscapedPath(), // the path as it goes on the wire
		Query:  req.URL.RawQuery,
		Header: req.Header.Clone(),
		Body:   body,
	})
	var r scriptedResponse
	switch {
	case m.idx < len(m.responses):
		r = m.responses[m.idx]
		m.idx++
	case len(m.responses) > 0:
		r = m.responses[len(m.responses)-1]
	default:
		r = scriptedResponse{status: http.StatusOK, body: okEnvelope("{}")}
	}
	m.mu.Unlock()

	// Honor cancellation like a real transport would.
	if ctxErr := req.Context().Err(); ctxErr != nil {
		return nil, ctxErr
	}

	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}
	if r.err != nil {
		return nil, r.err
	}

	header := r.header
	if header == nil {
		header = http.Header{}
	}
	if header.Get("Content-Type") == "" {
		header.Set("Content-Type", "application/json")
	}
	return &http.Response{
		StatusCode: r.status,
		Status:     http.StatusText(r.status),
		Body:       io.NopCloser(strings.NewReader(r.body)),
		Header:     header,
	}, nil
}

// callCount returns how many requests the transport has seen.
func (m *mockTransport) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// okEnvelope wraps data JSON in a success envelope.
func okEnvelope(dataJSON string) string {
	if dataJSON == "" {
		dataJSON = "{}"
	}
	return `{"success":true,"statusCode":200,"data":` + dataJSON + `,"meta":{"requestId":"req_mock"}}`
}

// listEnvelope wraps items JSON in a paginated success envelope.
func listEnvelope(itemsJSON string, hasMore bool, nextCursor string) string {
	cursor := "null"
	if nextCursor != "" {
		cursor = `"` + nextCursor + `"`
	}
	more := "false"
	if hasMore {
		more = "true"
	}
	return `{"success":true,"statusCode":200,"data":` + itemsJSON +
		`,"pagination":{"limit":20,"hasMore":` + more + `,"nextCursor":` + cursor +
		`},"meta":{"requestId":"req_mock"}}`
}

// errEnvelope builds an error envelope for a code.
func errEnvelope(code string) string {
	return `{"success":false,"statusCode":0,"error":{"code":"` + code +
		`","message":"something went wrong","hint":"try this instead",` +
		`"docUrl":"https://docs.nombaone.xyz/errors#` + code +
		`"},"meta":{"requestId":"req_err"}}`
}

// noBackoff replaces the retry delay with zero for the duration of a test.
func noBackoff(t *testing.T) {
	t.Helper()
	prev := backoffFunc
	backoffFunc = func(int) time.Duration { return 0 }
	t.Cleanup(func() { backoffFunc = prev })
}

// testClient builds a client wired to the mock transport.
func testClient(t *testing.T, m *mockTransport, opts ...Option) *Client {
	t.Helper()
	base := []Option{
		WithAPIKey("nbo_sandbox_test_key"),
		WithBaseURL("http://api.test"),
		WithHTTPClient(m),
	}
	c, err := New(append(base, opts...)...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}
