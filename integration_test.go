package nombaone_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	nombaone "github.com/nombaone/nombaone-go"
	"github.com/nombaone/nombaone-go/webhook"
)

// End-to-end suite against a real (deployed or local) NombaOne sandbox API.
// Opt-in:
//
//	NOMBAONE_INTEGRATION=1 \
//	NOMBAONE_API_KEY=nbo_sandbox_… \
//	NOMBAONE_BASE_URL=https://sandbox.api.nombaone.xyz \
//	go test -run TestIntegration -v ./...
//
// It exercises the public API exactly as a consumer would (external test
// package), so it doubles as an API-surface smoke test.

func integrationClient(t *testing.T) *nombaone.Client {
	t.Helper()
	if os.Getenv("NOMBAONE_INTEGRATION") != "1" {
		t.Skip("set NOMBAONE_INTEGRATION=1 (and NOMBAONE_API_KEY) to run the live suite")
	}
	key := os.Getenv("NOMBAONE_API_KEY")
	if key == "" {
		t.Fatal("NOMBAONE_INTEGRATION=1 but NOMBAONE_API_KEY is empty")
	}
	opts := []nombaone.Option{nombaone.WithAPIKey(key)}
	if base := os.Getenv("NOMBAONE_BASE_URL"); base != "" {
		opts = append(opts, nombaone.WithBaseURL(base))
	}
	c, err := nombaone.New(opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.Mode() != nombaone.ModeSandbox {
		t.Fatalf("integration suite must run against sandbox, got mode %q", c.Mode())
	}
	return c
}

func baseURLIsLocal() bool {
	base := os.Getenv("NOMBAONE_BASE_URL")
	return strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1")
}

var (
	custIDRe = regexp.MustCompile(`^nbo\d{12}cus$`)
	subIDRe  = regexp.MustCompile(`^nbo\d{12}sub$`)
)

func TestIntegration(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	unique := fmt.Sprintf("sdk-go-it-%d", time.Now().UnixNano())

	var subscriptionID string

	t.Run("full first-subscription lifecycle", func(t *testing.T) {
		customer, err := client.Customers.Create(ctx, nombaone.CustomerCreateParams{
			Email: unique + "@example.com",
			Name:  "SDK Integration",
		})
		if err != nil {
			t.Fatalf("create customer: %v", err)
		}
		if !custIDRe.MatchString(customer.ID) {
			t.Errorf("customer id %q does not match nbo…cus", customer.ID)
		}
		if customer.Mode != nombaone.ModeSandbox {
			t.Errorf("customer mode = %q, want sandbox", customer.Mode)
		}

		plan, err := client.Plans.Create(ctx, nombaone.PlanCreateParams{Name: "SDK IT " + unique})
		if err != nil {
			t.Fatalf("create plan: %v", err)
		}

		price, err := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{
			UnitAmountInKobo: 250_000, // ₦2,500.00 / month
			Interval:         nombaone.PriceIntervalMonth,
		})
		if err != nil {
			t.Fatalf("create price: %v", err)
		}
		if price.UnitAmountInKobo != 250_000 || price.Currency != "NGN" {
			t.Errorf("price = %+v", price)
		}

		method, err := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{
			CustomerID: customer.ID,
			Behavior:   nombaone.SandboxBehaviorSuccess,
		})
		if err != nil {
			t.Fatalf("sandbox payment method: %v", err)
		}

		sub, err := client.Subscriptions.Create(ctx, nombaone.SubscriptionCreateParams{
			CustomerID:      customer.ID,
			PriceID:         price.ID,
			PaymentMethodID: nombaone.String(method.ID),
		})
		if err != nil {
			t.Fatalf("create subscription: %v", err)
		}
		subscriptionID = sub.ID
		if !subIDRe.MatchString(sub.ID) {
			t.Errorf("subscription id %q does not match nbo…sub", sub.ID)
		}
		switch sub.Status {
		case nombaone.SubscriptionStatusActive, nombaone.SubscriptionStatusIncomplete, nombaone.SubscriptionStatusTrialing:
		default:
			t.Errorf("unexpected subscription status %q", sub.Status)
		}
	})

	t.Run("advance a billing cycle through the real engine", func(t *testing.T) {
		if subscriptionID == "" {
			t.Skip("no subscription from the lifecycle step")
		}
		result, err := client.Sandbox.AdvanceCycle(ctx, subscriptionID)
		if err != nil {
			t.Fatalf("advance cycle: %v", err)
		}
		if result.SubscriptionID != subscriptionID {
			t.Errorf("advance cycle subscription = %q", result.SubscriptionID)
		}
		if result.Invoice.TotalInKobo <= 0 {
			t.Errorf("advance cycle produced no invoice amount: %+v", result.Invoice)
		}
	})

	t.Run("upcoming invoice and dunning reads", func(t *testing.T) {
		if subscriptionID == "" {
			t.Skip("no subscription")
		}
		upcoming, err := client.Subscriptions.RetrieveUpcomingInvoice(ctx, subscriptionID)
		if err != nil {
			t.Fatalf("upcoming invoice: %v", err)
		}
		if upcoming.SubscriptionID != subscriptionID {
			t.Errorf("upcoming subscription = %q", upcoming.SubscriptionID)
		}

		dunning, err := client.Subscriptions.Dunning.Retrieve(ctx, subscriptionID)
		if err != nil {
			t.Fatalf("dunning: %v", err)
		}
		if dunning.SubscriptionRef != subscriptionID {
			t.Errorf("dunning ref = %q", dunning.SubscriptionRef)
		}
	})

	t.Run("pagination with real cursors and auto-iteration", func(t *testing.T) {
		page, err := client.Customers.List(ctx, nombaone.CustomerListParams{Limit: nombaone.Int(1)})
		if err != nil {
			t.Fatalf("list: %v", err)
		}
		if len(page.Data) > 1 || page.Pagination.Limit != 1 {
			t.Errorf("page = %+v", page.Pagination)
		}

		count := 0
		for _, err := range page.All(ctx) {
			if err != nil {
				t.Fatalf("auto-iterate: %v", err)
			}
			count++
			if count >= 3 { // prove cursors thread without walking everything
				break
			}
		}
		if count == 0 {
			t.Error("expected at least one customer across pages")
		}
	})

	t.Run("idempotency replay returns the identical resource", func(t *testing.T) {
		key := "sdk-go-it-idem-" + unique
		params := nombaone.CustomerCreateParams{Email: unique + "-idem@example.com", Name: "Idem Test"}

		first, err := client.Customers.Create(ctx, params, nombaone.WithIdempotencyKey(key))
		if err != nil {
			t.Fatalf("first create: %v", err)
		}
		second, err := client.Customers.Create(ctx, params, nombaone.WithIdempotencyKey(key))
		if err != nil {
			t.Fatalf("replay create: %v", err)
		}
		if first.ID != second.ID {
			t.Errorf("idempotent replay returned a different resource: %q vs %q", first.ID, second.ID)
		}
	})

	t.Run("typed errors carry code, hint, docUrl, requestId", func(t *testing.T) {
		_, err := client.Customers.Retrieve(ctx, "nbo000000000000cus")
		var notFound *nombaone.NotFoundError
		if !errors.As(err, &notFound) {
			t.Fatalf("want NotFoundError, got %T (%v)", err, err)
		}
		if notFound.Code != nombaone.ErrCodeCustomerNotFound {
			t.Errorf("code = %q", notFound.Code)
		}
		if notFound.Hint == "" {
			t.Error("hint is empty")
		}
		if !strings.Contains(notFound.DocURL, "CUSTOMER_NOT_FOUND") {
			t.Errorf("docUrl = %q", notFound.DocURL)
		}
		if !strings.HasPrefix(notFound.RequestID, "req_") {
			t.Errorf("requestId = %q", notFound.RequestID)
		}
	})

	t.Run("clean context cancellation is not retried", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := client.Customers.List(cctx, nombaone.CustomerListParams{})
		if !errors.Is(err, context.Canceled) {
			t.Errorf("want context.Canceled, got %v", err)
		}
	})

	t.Run("webhook round-trip (local targets only)", func(t *testing.T) {
		if !baseURLIsLocal() {
			t.Skip("webhook round-trip needs a local API that can reach a 127.0.0.1 listener; skipped for a remote sandbox")
		}
		runWebhookRoundTrip(t, client)
	})
}

// runWebhookRoundTrip registers a local listener as an endpoint, simulates an
// event, and verifies the signed delivery (when the backend ships the
// documented scheme).
func runWebhookRoundTrip(t *testing.T, client *nombaone.Client) {
	ctx := context.Background()
	type received struct {
		body    string
		headers http.Header
	}
	got := make(chan received, 4)

	srv := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/hooks", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- received{body: string(b), headers: r.Header.Clone()}
		w.WriteHeader(http.StatusOK)
	})
	srv.Handler = mux

	ln, err := listenLoopback()
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	endpoint, err := client.WebhookEndpoints.Create(ctx, nombaone.WebhookEndpointCreateParams{
		URL:           "http://" + ln.Addr().String() + "/hooks",
		EnabledEvents: []string{"*"},
	})
	if err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	if len(endpoint.SigningSecret) < 10 {
		t.Fatalf("signing secret too short: %q", endpoint.SigningSecret)
	}

	if _, err := client.Sandbox.SimulateWebhook(ctx, nombaone.SandboxSimulateWebhookParams{
		Type:    "invoice.paid",
		Payload: map[string]any{"reference": "nbo000000000001inv"},
	}); err != nil {
		t.Fatalf("simulate webhook: %v", err)
	}

	select {
	case delivery := <-got:
		sigHeader := delivery.headers.Get("X-Nombaone-Signature")
		if regexp.MustCompile(`(^|,)\s*t=\d+`).MatchString(sigHeader) && strings.Contains(sigHeader, "v1=") {
			event, err := webhook.ConstructEvent([]byte(delivery.body), sigHeader, endpoint.SigningSecret)
			if err != nil {
				t.Fatalf("verify delivery: %v", err)
			}
			if event.Type != "invoice.paid" {
				t.Errorf("event type = %q", event.Type)
			}
		} else {
			t.Logf("[integration] backend signature header is not in the documented \"t=…,v1=…\" format yet: %q — webhook.ConstructEvent will verify once the backend ships the docs scheme", truncate(sigHeader, 32))
		}
	case <-time.After(15 * time.Second):
		t.Fatal("no delivery arrived within 15s")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func listenLoopback() (net.Listener, error) {
	return net.Listen("tcp", "127.0.0.1:0")
}
