package nombaone

import "encoding/json"

// Pointer constructors for optional request fields. Optional scalars are typed
// as pointers so an unset field (nil) is omitted from the request body
// entirely, while a set field is sent. Use these to take the address of a
// literal inline:
//
//	client.Subscriptions.Create(ctx, nombaone.SubscriptionCreateParams{
//		CustomerID: customer.ID,
//		PriceID:    price.ID,
//		TrialDays:  nombaone.Int(14), // optional
//	})

// String returns a pointer to v, for an optional string field.
func String(v string) *string { return &v }

// Int returns a pointer to v, for an optional int field.
func Int(v int) *int { return &v }

// Int64 returns a pointer to v, for an optional int64 field — including
// optional money fields (integer kobo, ₦1.00 = 100).
func Int64(v int64) *int64 { return &v }

// Bool returns a pointer to v, for an optional bool field.
func Bool(v bool) *bool { return &v }

// Optional is a three-state value for the handful of request fields the API
// lets you clear by sending an explicit JSON null (for example a customer's
// phone). The three states are:
//
//   - the zero value / a nil *Optional — the field is omitted (unchanged);
//   - Set(v) — the field is sent with value v;
//   - Null[T]() — the field is sent as JSON null (cleared).
//
// Fields of this kind are typed *Optional[T] with json:",omitempty", so a nil
// pointer omits the field and a non-nil pointer marshals to the value or null.
type Optional[T any] struct {
	value T
	null  bool
}

// Set returns an *Optional[T] that marshals to v — for a nullable field you
// want to assign a value.
func Set[T any](v T) *Optional[T] { return &Optional[T]{value: v} }

// Null returns an *Optional[T] that marshals to JSON null — for a nullable
// field you want to clear.
func Null[T any]() *Optional[T] { return &Optional[T]{null: true} }

// MarshalJSON emits null when the field was cleared, otherwise the value.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if o.null {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}
