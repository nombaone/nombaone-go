package nombaone

import (
	"strings"
	"testing"
)

func TestUserAgent(t *testing.T) {
	if !strings.HasPrefix(userAgent, "nombaone-go/") {
		t.Fatalf("userAgent = %q, want prefix nombaone-go/", userAgent)
	}
	if !strings.HasSuffix(userAgent, Version) {
		t.Fatalf("userAgent = %q, want suffix %q", userAgent, Version)
	}
}
