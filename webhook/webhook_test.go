package webhook

import (
	"errors"
	"math"
	"strconv"
	"testing"
	"time"
)

// The golden vector every NombaOne SDK must pass byte-for-byte.
const (
	goldenSecret  = "nbo_whsec_golden_0123456789abcdef0123456789abcdef"
	goldenT       = int64(1751600000)
	goldenPayload = `{"id":"nbo000000000001whd","type":"invoice.paid","event":{"id":"nbo000000000001evt","type":"invoice.paid","createdAt":"2026-07-04T10:00:00.000Z"},"data":{"reference":"nbo000000000001inv"}}`
	goldenHeader  = "t=1751600000,v1=ba56a072beccddbc014a3f72ef1b4a30e2008b61dcbcca4ae2f16c7e4427b374"
)

// hugeTolerance verifies against a fixed past timestamp without a stale-t failure.
func hugeTolerance() Option { return WithTolerance(time.Duration(math.MaxInt64)) }

func TestGoldenVector(t *testing.T) {
	// The computed signature must equal the documented header exactly.
	got := GenerateTestHeader([]byte(goldenPayload), goldenSecret, time.Unix(goldenT, 0))
	if got != goldenHeader {
		t.Fatalf("GenerateTestHeader mismatch:\n got  %s\n want %s", got, goldenHeader)
	}

	// And verification of the documented header must pass.
	if err := VerifySignature([]byte(goldenPayload), goldenHeader, goldenSecret, hugeTolerance()); err != nil {
		t.Fatalf("golden header failed verification: %v", err)
	}

	// ConstructEvent returns the typed, dedupe-able event.
	event, err := ConstructEvent([]byte(goldenPayload), goldenHeader, goldenSecret, hugeTolerance())
	if err != nil {
		t.Fatalf("ConstructEvent: %v", err)
	}
	if event.Type != EventTypeInvoicePaid {
		t.Errorf("Type = %q", event.Type)
	}
	if event.Event.ID != "nbo000000000001evt" {
		t.Errorf("Event.ID = %q, want the dedupe id", event.Event.ID)
	}
	data, err := DecodeData[RefData](event)
	if err != nil || data.Reference != "nbo000000000001inv" {
		t.Errorf("data = %+v, err = %v", data, err)
	}
}

func signNow(t *testing.T, payload, secret string) string {
	t.Helper()
	return GenerateTestHeader([]byte(payload), secret)
}

func TestRejectionMatrix(t *testing.T) {
	payload := goldenPayload

	t.Run("tampered payload is rejected", func(t *testing.T) {
		header := signNow(t, payload, goldenSecret)
		tampered := payload[:len(payload)-2] + `X}` // corrupt the last bytes
		if err := VerifySignature([]byte(tampered), header, goldenSecret); err == nil {
			t.Fatal("tampered payload passed verification")
		}
	})

	t.Run("wrong secret is rejected", func(t *testing.T) {
		header := signNow(t, payload, goldenSecret)
		if err := VerifySignature([]byte(payload), header, "nbo_whsec_wrong"); err == nil {
			t.Fatal("wrong secret passed verification")
		}
	})

	t.Run("stale timestamp beyond default tolerance is rejected", func(t *testing.T) {
		stale := time.Now().Add(-301 * time.Second)
		header := GenerateTestHeader([]byte(payload), goldenSecret, stale)
		err := VerifySignature([]byte(payload), header, goldenSecret)
		if err == nil {
			t.Fatal("stale timestamp passed")
		}
	})

	t.Run("just-inside tolerance is accepted", func(t *testing.T) {
		fresh := time.Now().Add(-290 * time.Second)
		header := GenerateTestHeader([]byte(payload), goldenSecret, fresh)
		if err := VerifySignature([]byte(payload), header, goldenSecret); err != nil {
			t.Fatalf("fresh timestamp rejected: %v", err)
		}
	})

	t.Run("future timestamp beyond tolerance is rejected (symmetric)", func(t *testing.T) {
		future := time.Now().Add(400 * time.Second)
		header := GenerateTestHeader([]byte(payload), goldenSecret, future)
		if err := VerifySignature([]byte(payload), header, goldenSecret); err == nil {
			t.Fatal("future timestamp passed")
		}
	})

	t.Run("multiple v1 where only the second matches is accepted (rotation)", func(t *testing.T) {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		good := computeSignature(goldenSecret, ts, []byte(payload))
		stale := computeSignature("nbo_whsec_old", ts, []byte(payload))
		header := "t=" + ts + ",v1=" + stale + ",v1=" + good // stale first, good second
		if err := VerifySignature([]byte(payload), header, goldenSecret); err != nil {
			t.Fatalf("rotation header rejected: %v", err)
		}
	})

	t.Run("missing header, malformed header, and missing secret give distinct errors", func(t *testing.T) {
		header := signNow(t, payload, goldenSecret)

		var ve *VerificationError
		if err := VerifySignature([]byte(payload), "", goldenSecret); !errors.As(err, &ve) {
			t.Fatalf("missing header: want VerificationError, got %v", err)
		}
		missingHeaderMsg := ve.Message

		if err := VerifySignature([]byte(payload), "garbage", goldenSecret); !errors.As(err, &ve) {
			t.Fatalf("malformed header: want VerificationError, got %v", err)
		}
		malformedMsg := ve.Message

		if err := VerifySignature([]byte(payload), header, ""); !errors.As(err, &ve) {
			t.Fatalf("missing secret: want VerificationError, got %v", err)
		}
		missingSecretMsg := ve.Message

		if missingHeaderMsg == malformedMsg || malformedMsg == missingSecretMsg || missingHeaderMsg == missingSecretMsg {
			t.Errorf("failure messages should be distinct: %q / %q / %q", missingHeaderMsg, malformedMsg, missingSecretMsg)
		}
	})

	t.Run("non-JSON body is rejected after signature check", func(t *testing.T) {
		body := []byte("not json at all")
		header := GenerateTestHeader(body, goldenSecret)
		if _, err := ConstructEvent(body, header, goldenSecret); err == nil {
			t.Fatal("non-JSON body passed ConstructEvent")
		}
	})
}

func TestFlatLegacyBodySynthesizesEventRef(t *testing.T) {
	flat := `{"id":"evt_flat_1","type":"invoice.paid","createdAt":"2026-07-04T10:00:00.000Z","data":{"reference":"nbo000000000001inv"}}`
	header := GenerateTestHeader([]byte(flat), goldenSecret)
	event, err := ConstructEvent([]byte(flat), header, goldenSecret)
	if err != nil {
		t.Fatal(err)
	}
	if event.Event.ID != "evt_flat_1" || event.Event.Type != "invoice.paid" {
		t.Errorf("flat body did not synthesize Event ref: %+v", event.Event)
	}
}

func TestTypedPayloadDecoding(t *testing.T) {
	body := `{"id":"nbo1whd","type":"invoice.action_required","event":{"id":"nbo1evt","type":"invoice.action_required","createdAt":"2026-07-04T10:00:00.000Z"},"data":{"reference":"nbo1inv","reason":"authentication_required","checkoutLink":"https://pay.example/x"}}`
	header := GenerateTestHeader([]byte(body), goldenSecret)
	event, err := ConstructEvent([]byte(body), header, goldenSecret)
	if err != nil {
		t.Fatal(err)
	}
	data, err := DecodeData[InvoiceActionRequiredData](event)
	if err != nil {
		t.Fatal(err)
	}
	if data.CheckoutLink != "https://pay.example/x" || data.Reason != "authentication_required" {
		t.Errorf("decoded = %+v", data)
	}
}

func TestUnknownEventTypeStillParses(t *testing.T) {
	body := `{"id":"nbo1whd","type":"future.event_type","event":{"id":"nbo1evt","type":"future.event_type","createdAt":"2026-07-04T10:00:00.000Z"},"data":{"reference":"nbo1xxx","novel":true}}`
	header := GenerateTestHeader([]byte(body), goldenSecret)
	event, err := ConstructEvent([]byte(body), header, goldenSecret)
	if err != nil {
		t.Fatalf("unknown event type must still parse: %v", err)
	}
	if event.Type != "future.event_type" {
		t.Errorf("Type = %q", event.Type)
	}
	ref, _ := DecodeData[RefData](event)
	if ref.Reference != "nbo1xxx" {
		t.Errorf("Reference = %q", ref.Reference)
	}
}

// clock injection is exercised to keep withNow covered and deterministic.
func TestClockInjection(t *testing.T) {
	fixed := time.Unix(goldenT, 0)
	header := GenerateTestHeader([]byte(goldenPayload), goldenSecret, fixed)
	err := VerifySignature([]byte(goldenPayload), header, goldenSecret, withNow(func() time.Time { return fixed }))
	if err != nil {
		t.Fatalf("clock-injected verification failed: %v", err)
	}
}
