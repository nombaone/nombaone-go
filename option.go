package nombaone

import (
	"net/http"
	"time"
)

// HTTPClient is the minimal transport the SDK needs. *http.Client satisfies
// it. Provide your own with [WithHTTPClient] to inject a proxy, custom
// timeouts, or a recording double in tests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// clientConfig accumulates options before [New] validates and freezes them.
type clientConfig struct {
	apiKey        string
	apiKeySet     bool
	baseURL       string
	httpClient    HTTPClient
	timeout       time.Duration
	maxRetries    int
	defaultHeader http.Header
}

// Option configures a [Client] at construction time.
type Option func(*clientConfig)

// WithAPIKey sets the secret API key (nbo_sandbox_… or nbo_live_…). When
// omitted, the client reads NOMBAONE_API_KEY from the environment. Server-side
// only — never ship a key to a browser or mobile bundle.
func WithAPIKey(key string) Option {
	return func(c *clientConfig) {
		c.apiKey = key
		c.apiKeySet = true
	}
}

// WithBaseURL overrides the API origin (no /v1). It defaults to the host that
// matches the key's environment and always wins over the derived host. It is
// required when the key prefix is unrecognized.
func WithBaseURL(baseURL string) Option {
	return func(c *clientConfig) { c.baseURL = baseURL }
}

// WithHTTPClient supplies the transport used for every request. Defaults to a
// new *http.Client. The SDK manages per-attempt timeouts via context, so a
// client-level Timeout is not required (and would cap the whole retried call).
func WithHTTPClient(h HTTPClient) Option {
	return func(c *clientConfig) { c.httpClient = h }
}

// WithTimeout sets the per-attempt timeout. Default 30s. Each retry attempt
// gets its own fresh budget.
func WithTimeout(d time.Duration) Option {
	return func(c *clientConfig) { c.timeout = d }
}

// WithMaxRetries sets the automatic retry budget for network failures,
// timeouts, 408/429/5xx, and in-flight idempotency conflicts. Default 2 (3
// attempts total). Retries of a POST always reuse the same Idempotency-Key.
func WithMaxRetries(n int) Option {
	return func(c *clientConfig) { c.maxRetries = n }
}

// WithDefaultHeader adds a header sent on every request. Repeat to add
// several. These are overridden by per-call headers set with [WithHeader].
func WithDefaultHeader(key, value string) Option {
	return func(c *clientConfig) {
		if c.defaultHeader == nil {
			c.defaultHeader = http.Header{}
		}
		c.defaultHeader.Set(key, value)
	}
}

// requestConfig accumulates per-call options.
type requestConfig struct {
	idempotencyKey string
	header         http.Header
	timeout        *time.Duration
	maxRetries     *int
	rawResponse    **http.Response
}

// RequestOption configures a single API call. It is the trailing variadic
// argument of every method.
type RequestOption func(*requestConfig)

// WithIdempotencyKey overrides the auto-generated Idempotency-Key for a POST.
// The SDK generates a fresh UUID per call and reuses it across automatic
// retries, so a network blip can never double-charge. Pass your own stable key
// when the operation must stay idempotent across process restarts — for
// example a payout keyed by your own transaction reference.
func WithIdempotencyKey(key string) RequestOption {
	return func(r *requestConfig) { r.idempotencyKey = key }
}

// WithHeader sets an extra header for a single request, overriding a default
// header of the same name.
func WithHeader(key, value string) RequestOption {
	return func(r *requestConfig) {
		if r.header == nil {
			r.header = http.Header{}
		}
		r.header.Set(key, value)
	}
}

// WithRequestTimeout overrides the per-attempt timeout for a single call.
func WithRequestTimeout(d time.Duration) RequestOption {
	return func(r *requestConfig) { r.timeout = &d }
}

// WithRequestMaxRetries overrides the retry budget for a single call.
func WithRequestMaxRetries(n int) RequestOption {
	return func(r *requestConfig) { r.maxRetries = &n }
}

// WithRawResponse captures the raw *http.Response of a call into dst — the
// escape hatch for reading response headers (X-Request-Id, rate-limit info)
// and the status line. The body is already drained by the SDK.
//
//	var raw *http.Response
//	customer, err := client.Customers.Retrieve(ctx, id, nombaone.WithRawResponse(&raw))
//	// raw.Header.Get("X-Request-Id")
func WithRawResponse(dst **http.Response) RequestOption {
	return func(r *requestConfig) { r.rawResponse = dst }
}
