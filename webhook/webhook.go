// Package webhook verifies and parses NombaOne webhook deliveries. It needs
// only the endpoint's signing secret — never an API key — so a webhook
// receiver can import this package alone.
//
// Feed [ConstructEvent] the exact raw request body. JSON re-encoding can
// reorder keys and change bytes, which breaks the signature; capture the body
// before any framework parses it.
//
//	http.HandleFunc("/nombaone/webhooks", func(w http.ResponseWriter, r *http.Request) {
//		body, _ := io.ReadAll(r.Body)
//		event, err := webhook.ConstructEvent(body, r.Header.Get("X-Nombaone-Signature"), secret)
//		if err != nil {
//			w.WriteHeader(http.StatusBadRequest)
//			return
//		}
//		if alreadyProcessed(event.Event.ID) { // at-least-once ⇒ dedupe on event.Event.ID
//			w.WriteHeader(http.StatusOK)
//			return
//		}
//		switch event.Type {
//		case webhook.EventTypeInvoicePaid:
//			// …
//		}
//		w.WriteHeader(http.StatusOK) // respond 2xx fast; do heavy work async
//	})
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"
	"time"
)

// DefaultTolerance is the maximum allowed age of a delivery's timestamp in
// either direction. Raise it only if your clock or queueing genuinely lags.
const DefaultTolerance = 5 * time.Minute

// VerificationError is returned when a delivery cannot be verified — a
// missing/malformed header, a stale timestamp, an invalid signature, a missing
// secret, or a non-JSON body. Each failure mode carries a distinct message.
type VerificationError struct {
	Message string
}

func (e *VerificationError) Error() string { return "nombaone webhook: " + e.Message }

func verr(message string) error { return &VerificationError{Message: message} }

// config holds resolved verification options.
type config struct {
	tolerance time.Duration
	now       func() time.Time
}

// Option configures verification.
type Option func(*config)

// WithTolerance overrides the default 5-minute timestamp tolerance.
func WithTolerance(d time.Duration) Option {
	return func(c *config) { c.tolerance = d }
}

// withNow overrides the clock (used by tests).
func withNow(fn func() time.Time) Option {
	return func(c *config) { c.now = fn }
}

func resolve(opts []Option) config {
	c := config{tolerance: DefaultTolerance, now: time.Now}
	for _, opt := range opts {
		opt(&c)
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c
}

// ConstructEvent verifies a delivery's signature and timestamp, then parses and
// returns the typed [Event]. This is the one call your handler needs.
//
// Delivery is at-least-once — after verification, dedupe on Event.Event.ID
// before acting.
func ConstructEvent(payload []byte, signatureHeader, secret string, opts ...Option) (Event, error) {
	if err := VerifySignature(payload, signatureHeader, secret, opts...); err != nil {
		return Event{}, err
	}

	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return Event{}, verr("payload was not valid JSON")
	}

	// Defensive: guarantee a dedupe-able Event.ID even if a delivery body
	// arrives flat (older shape) — fall back to the top-level fields.
	if event.Event.ID == "" && event.Event.Type == "" {
		event.Event = EventRef{ID: event.ID, Type: event.Type, CreatedAt: event.CreatedAt}
	}
	return event, nil
}

// VerifySignature verifies a delivery's signature and timestamp without
// parsing the body. It returns nil on success or a [*VerificationError].
func VerifySignature(payload []byte, signatureHeader, secret string, opts ...Option) error {
	cfg := resolve(opts)

	if signatureHeader == "" {
		return verr("missing X-Nombaone-Signature header — is this request really from NombaOne?")
	}
	if secret == "" {
		return verr("missing signing secret — pass the secret shown when the endpoint was created")
	}

	timestamp, signatures, err := parseSignatureHeader(signatureHeader)
	if err != nil {
		return err
	}

	ts, convErr := strconv.ParseInt(timestamp, 10, 64)
	if convErr != nil {
		return verr(`malformed X-Nombaone-Signature header — "t" is not a unix timestamp`)
	}
	age := cfg.now().Unix() - ts
	if age < 0 {
		age = -age
	}
	if age > int64(cfg.tolerance.Seconds()) {
		return verr("timestamp is outside the allowed tolerance — possible replay, or severe clock skew")
	}

	expected := computeSignature(secret, timestamp, payload)
	for _, candidate := range signatures {
		if len(candidate) == len(expected) && subtle.ConstantTimeCompare([]byte(candidate), []byte(expected)) == 1 {
			return nil
		}
	}
	return verr("signature verification failed — check the endpoint's current signing secret and the exact raw request body (no re-serialization)")
}

// GenerateTestHeader builds a valid X-Nombaone-Signature header for a payload —
// for testing your own handler without waiting on a real delivery. Pass an
// optional timestamp; it defaults to now.
//
//	header := webhook.GenerateTestHeader(payload, secret)
//	event, err := webhook.ConstructEvent(payload, header, secret)
func GenerateTestHeader(payload []byte, secret string, at ...time.Time) string {
	ts := time.Now()
	if len(at) > 0 {
		ts = at[0]
	}
	timestamp := strconv.FormatInt(ts.Unix(), 10)
	return "t=" + timestamp + ",v1=" + computeSignature(secret, timestamp, payload)
}

// computeSignature is hex(HMAC_SHA256(secret, "{t}.{rawBody}")).
func computeSignature(secret, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// parseSignatureHeader extracts the timestamp and every v1 signature. Multiple
// v1 entries are legal during secret rotation.
func parseSignatureHeader(header string) (timestamp string, signatures []string, err error) {
	for _, pair := range strings.Split(header, ",") {
		eq := strings.IndexByte(pair, '=')
		if eq == -1 {
			continue
		}
		key := strings.TrimSpace(pair[:eq])
		value := strings.TrimSpace(pair[eq+1:])
		switch key {
		case "t":
			timestamp = value
		case "v1":
			if value != "" {
				signatures = append(signatures, value)
			}
		}
	}
	if timestamp == "" || len(signatures) == 0 {
		return "", nil, verr(`malformed X-Nombaone-Signature header — expected "t=<unix>,v1=<hex>"`)
	}
	return timestamp, signatures, nil
}
