package nombaone

import "encoding/json"

// Mode is the environment a key — and everything created with it — lives in.
// It is encoded in the API-key prefix (nbo_sandbox_… or nbo_live_…). Sandbox
// and live are fully isolated: a resource created in one does not exist in the
// other.
type Mode string

const (
	ModeSandbox Mode = "sandbox"
	ModeLive    Mode = "live"
)

// Kobo is an amount of money in kobo, the integer minor unit of the naira
// (₦1.00 = 100). Every money field in the API is an integer number of kobo —
// never a float, never a decimal string. Multiply naira by 100 exactly once,
// at the edge of your system. Kobo is an alias for int64 so it composes with
// ordinary integer arithmetic without conversions.
type Kobo = int64

// Metadata is a free-form set of annotations you can attach to most resources.
// Values are arbitrary JSON, not string-only.
type Metadata = map[string]any

// Pagination is the cursor block returned at the top level of every list
// response.
type Pagination struct {
	// Limit is the page size that was applied (1–100; the API default is 20).
	Limit int `json:"limit"`
	// HasMore reports whether more items exist beyond this page.
	HasMore bool `json:"hasMore"`
	// NextCursor is the opaque cursor for the next page, or nil when HasMore
	// is false.
	NextCursor *string `json:"nextCursor"`
}

// successEnvelope is the wire wrapper around every successful response. The
// SDK unwraps Data as the method's return value.
type successEnvelope struct {
	Success    bool            `json:"success"`
	StatusCode int             `json:"statusCode"`
	Data       json.RawMessage `json:"data"`
	Pagination *Pagination     `json:"pagination"`
	Meta       envelopeMeta    `json:"meta"`
}

type envelopeMeta struct {
	RequestID string `json:"requestId"`
}
