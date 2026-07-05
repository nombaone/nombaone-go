package nombaone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// apiPrefix is the version segment, applied at exactly one place in the
// transport and never in per-resource paths.
const apiPrefix = "/v1"

// requestSpec is the internal description of one HTTP call. Resource methods
// produce these and hand them to [execute].
type requestSpec struct {
	method string
	// path is the route below /v1, with any id segments already URL-encoded.
	path  string
	query url.Values
	// body is marshaled to JSON, or nil for no body.
	body any
	opts []RequestOption
}

// result is what the transport hands back to a resource method.
type result[T any] struct {
	Data       T
	Pagination *Pagination
	RequestID  string
	Response   *http.Response
}

// execute runs one logical API call: it computes the idempotency key once,
// runs the retry loop, parses the success envelope, and unwraps Data into T.
//
// The money-safety invariants live here and nowhere else:
//   - the Idempotency-Key for a POST is computed once, before the retry loop,
//     so every automatic retry replays the same logical operation;
//   - a caller-initiated cancellation is never retried — only network
//     failures, timeouts, 408/429/5xx, and our own in-flight idempotency
//     conflict are.
func execute[T any](ctx context.Context, c *Client, spec requestSpec) (*result[T], error) {
	rc := &requestConfig{}
	for _, opt := range spec.opts {
		opt(rc)
	}

	method := strings.ToUpper(spec.method)

	// Compute the idempotency key ONCE, before the retry loop (POST only).
	var idempotencyKey string
	if method == http.MethodPost {
		if rc.idempotencyKey != "" {
			idempotencyKey = rc.idempotencyKey
		} else {
			idempotencyKey = newIdempotencyKey()
		}
	}

	var bodyBytes []byte
	if spec.body != nil {
		encoded, err := json.Marshal(spec.body)
		if err != nil {
			return nil, &ConnectionError{Message: "failed to encode request body", Err: err}
		}
		bodyBytes = encoded
	}

	fullURL := c.baseURL + apiPrefix + spec.path
	if len(spec.query) > 0 {
		fullURL += "?" + spec.query.Encode()
	}

	timeout := c.timeout
	if rc.timeout != nil {
		timeout = *rc.timeout
	}
	maxRetries := c.maxRetries
	if rc.maxRetries != nil {
		maxRetries = *rc.maxRetries
	}
	if maxRetries < 0 {
		maxRetries = 0
	}

	header := c.buildHeader(method, idempotencyKey, spec.body != nil, rc.header)

	resp, respBody, err := c.roundtripWithRetry(ctx, method, fullURL, bodyBytes, header, timeout, maxRetries)
	if rc.rawResponse != nil {
		*rc.rawResponse = resp
	}
	if err != nil {
		return nil, err
	}

	var env successEnvelope
	if e := json.Unmarshal(respBody, &env); e != nil {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Code:       ErrCodeSystemInternalError,
			Message:    "response was not a valid NombaOne envelope",
			RequestID:  resp.Header.Get("X-Request-Id"),
		}
	}

	var data T
	if len(env.Data) > 0 && string(env.Data) != "null" {
		if e := json.Unmarshal(env.Data, &data); e != nil {
			return nil, &APIError{
				StatusCode: resp.StatusCode,
				Code:       ErrCodeSystemInternalError,
				Message:    "failed to decode response data: " + e.Error(),
				RequestID:  requestIDOf(env, resp.Header),
			}
		}
	}

	return &result[T]{
		Data:       data,
		Pagination: env.Pagination,
		RequestID:  requestIDOf(env, resp.Header),
		Response:   resp,
	}, nil
}

func requestIDOf(env successEnvelope, header http.Header) string {
	if env.Meta.RequestID != "" {
		return env.Meta.RequestID
	}
	return header.Get("X-Request-Id")
}

// roundtripWithRetry runs the retry loop and returns the final response and
// drained body on 2xx, the (non-nil) error response plus an [APIError] on a
// non-retryable non-2xx, or a transport error with a nil response.
func (c *Client) roundtripWithRetry(
	ctx context.Context,
	method, fullURL string,
	body []byte,
	header http.Header,
	timeout time.Duration,
	maxRetries int,
) (*http.Response, []byte, error) {
	for attempt := 0; ; attempt++ {
		resp, respBody, err := c.singleAttempt(ctx, method, fullURL, body, header, timeout)
		if err != nil {
			// A caller cancellation or exhausted parent deadline is a decision,
			// not a fault — never retried.
			if ctx.Err() != nil {
				return nil, nil, &ConnectionError{Message: "request canceled", Err: ctx.Err()}
			}
			var transportErr error
			if errors.Is(err, context.DeadlineExceeded) {
				transportErr = &TimeoutError{
					Message: fmt.Sprintf("request timed out after %s", timeout),
					Err:     err,
				}
			} else {
				transportErr = &ConnectionError{Message: "failed to reach the NombaOne API", Err: err}
			}
			if attempt >= maxRetries {
				return nil, nil, transportErr
			}
			if !sleepCtx(ctx, backoff(attempt)) {
				return nil, nil, &ConnectionError{Message: "request canceled", Err: ctx.Err()}
			}
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, respBody, nil
		}

		apiErr := apiErrorFromResponse(resp.StatusCode, respBody, resp.Header)
		if attempt < maxRetries && isRetryableStatus(resp.StatusCode, apiErr) {
			delay := retryAfter(resp.Header)
			if delay <= 0 {
				delay = backoffFunc(attempt)
			}
			if !sleepCtx(ctx, delay) {
				return resp, respBody, apiErr
			}
			continue
		}
		return resp, respBody, apiErr
	}
}

// singleAttempt performs one HTTP round-trip with its own per-attempt timeout,
// fully draining and closing the response body before returning.
func (c *Client) singleAttempt(
	ctx context.Context,
	method, fullURL string,
	body []byte,
	header http.Header,
	timeout time.Duration,
) (*http.Response, []byte, error) {
	attemptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(attemptCtx, method, fullURL, bodyReader)
	if err != nil {
		return nil, nil, err
	}
	req.Header = header.Clone()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp, respBody, nil
}

// buildHeader assembles request headers: SDK-managed values first, then
// client default headers, then per-call headers (later layers override
// earlier ones by name).
func (c *Client) buildHeader(method, idempotencyKey string, hasBody bool, perCall http.Header) http.Header {
	h := http.Header{}
	h.Set("Authorization", "Bearer "+c.apiKey)
	h.Set("Accept", "application/json")
	h.Set("User-Agent", userAgent)
	if hasBody {
		h.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		h.Set("Idempotency-Key", idempotencyKey)
	}
	for key, values := range c.defaultHeader {
		h.Del(key)
		for _, v := range values {
			h.Add(key, v)
		}
	}
	for key, values := range perCall {
		h.Del(key)
		for _, v := range values {
			h.Add(key, v)
		}
	}
	return h
}

// isRetryableStatus reports whether a non-2xx status warrants a retry. 5xx,
// 408, and 429 always do; 409 does only when it is our own earlier attempt
// still holding the idempotency claim.
func isRetryableStatus(status int, err error) bool {
	switch status {
	case http.StatusRequestTimeout, // 408
		http.StatusTooManyRequests,     // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	}
	if status == http.StatusConflict {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Code == ErrCodeIdempotencyInProgress {
			return true
		}
	}
	return false
}

// backoffFunc computes the delay before a retry. It is a package variable so
// tests can replace it with a zero-delay stub; production always uses backoff.
var backoffFunc = backoff

// backoff returns a full-jitter exponential delay: random(0, min(8s, 500ms·2^attempt)).
func backoff(attempt int) time.Duration {
	const base = 500 * time.Millisecond
	const maxDelay = 8 * time.Second
	ceiling := maxDelay
	if attempt < 5 { // guard the shift from overflowing
		if d := base << attempt; d < maxDelay {
			ceiling = d
		}
	}
	if ceiling <= 0 {
		ceiling = maxDelay
	}
	return time.Duration(rand.Int64N(int64(ceiling) + 1))
}

// retryAfter parses a Retry-After header (delta-seconds or HTTP-date) into a
// delay, or 0 when absent/unparseable/in the past.
func retryAfter(header http.Header) time.Duration {
	v := header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// sleepCtx waits for d or until ctx is done. It returns true if the full delay
// elapsed, false if ctx was canceled first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
