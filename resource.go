package nombaone

import (
	"net/url"
	"strconv"
)

// seg URL-encodes one path segment. Resource ids come from user input and API
// responses, so every id interpolated into a path is escaped — never trusted
// raw.
func seg(s string) string { return url.PathEscape(s) }

// Query helpers used by List methods to build the query string, dropping any
// unset (nil / empty) filter rather than sending a blank value.

func addQueryStr(q url.Values, key string, val *string) {
	if val != nil {
		q.Set(key, *val)
	}
}

func addQueryInt(q url.Values, key string, val *int) {
	if val != nil {
		q.Set(key, strconv.Itoa(*val))
	}
}

func addQueryBool(q url.Values, key string, val *bool) {
	if val != nil {
		q.Set(key, strconv.FormatBool(*val))
	}
}

// addQueryEnum adds a typed string-enum filter unless it is the zero value.
func addQueryEnum[T ~string](q url.Values, key string, val T) {
	if val != "" {
		q.Set(key, string(val))
	}
}
