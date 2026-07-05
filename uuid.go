package nombaone

import (
	"crypto/rand"
	"encoding/hex"
)

// newIdempotencyKey returns a random RFC 4122 version-4 UUID string. It uses
// crypto/rand so the SDK carries zero third-party dependencies. This is
// computed once per logical call, before the retry loop, so every automatic
// retry of a POST replays the same key.
func newIdempotencyKey() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read never fails on supported platforms; if it somehow
		// did, a non-conforming but unique-enough fallback is far better than
		// panicking inside a payment call.
		return "nbo-idem-" + hex.EncodeToString(b[:])
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10

	var buf [36]byte
	hex.Encode(buf[0:8], b[0:4])
	buf[8] = '-'
	hex.Encode(buf[9:13], b[4:6])
	buf[13] = '-'
	hex.Encode(buf[14:18], b[6:8])
	buf[18] = '-'
	hex.Encode(buf[19:23], b[8:10])
	buf[23] = '-'
	hex.Encode(buf[24:36], b[10:16])
	return string(buf[:])
}
