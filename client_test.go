package nombaone

import (
	"strings"
	"testing"
)

func TestNew_DerivesModeAndHostFromKeyPrefix(t *testing.T) {
	sandbox, err := New(WithAPIKey("nbo_sandbox_abc"))
	if err != nil {
		t.Fatalf("New sandbox: %v", err)
	}
	if sandbox.Mode() != ModeSandbox {
		t.Errorf("Mode() = %q, want sandbox", sandbox.Mode())
	}
	if sandbox.BaseURL() != defaultSandboxBaseURL {
		t.Errorf("BaseURL() = %q, want %q", sandbox.BaseURL(), defaultSandboxBaseURL)
	}

	live, err := New(WithAPIKey("nbo_live_abc"))
	if err != nil {
		t.Fatalf("New live: %v", err)
	}
	if live.Mode() != ModeLive {
		t.Errorf("Mode() = %q, want live", live.Mode())
	}
	if live.BaseURL() != defaultLiveBaseURL {
		t.Errorf("BaseURL() = %q, want %q", live.BaseURL(), defaultLiveBaseURL)
	}
}

func TestNew_ReadsKeyFromEnv(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "nbo_sandbox_from_env")
	c, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Mode() != ModeSandbox {
		t.Errorf("Mode() = %q, want sandbox", c.Mode())
	}
}

func TestNew_MissingKeyIsActionableError(t *testing.T) {
	t.Setenv(apiKeyEnvVar, "")
	_, err := New()
	if err == nil {
		t.Fatal("expected an error for a missing key")
	}
	if !strings.Contains(err.Error(), "missing API key") {
		t.Errorf("error = %q, want it to mention the missing key", err)
	}
	if !strings.Contains(err.Error(), apiKeyEnvVar) {
		t.Errorf("error = %q, want it to name the env var", err)
	}
}

func TestNew_UnknownPrefixWithoutBaseURLFails(t *testing.T) {
	_, err := New(WithAPIKey("totally_unknown_key"))
	if err == nil {
		t.Fatal("expected an error for an unrecognized key prefix")
	}
	if !strings.Contains(err.Error(), "unrecognized") {
		t.Errorf("error = %q, want it to mention the unrecognized format", err)
	}
}

func TestNew_UnknownPrefixWithBaseURLSucceeds(t *testing.T) {
	c, err := New(WithAPIKey("totally_unknown_key"), WithBaseURL("http://localhost:9000"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Mode() != ModeSandbox { // defaults to sandbox when undetermined
		t.Errorf("Mode() = %q, want sandbox default", c.Mode())
	}
	if c.BaseURL() != "http://localhost:9000" {
		t.Errorf("BaseURL() = %q", c.BaseURL())
	}
}

func TestNew_BaseURLOverrideWinsAndTrimsSlash(t *testing.T) {
	c, err := New(WithAPIKey("nbo_live_abc"), WithBaseURL("http://host.test/"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.BaseURL() != "http://host.test" {
		t.Errorf("BaseURL() = %q, want trailing slash trimmed", c.BaseURL())
	}
	// mode still derives from the key even with a custom host
	if c.Mode() != ModeLive {
		t.Errorf("Mode() = %q, want live", c.Mode())
	}
}
