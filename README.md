# nombaone-go

The official Go SDK for the [Nomba One](https://nombaone.xyz) subscription-billing API — recurring billing for Nigeria over card, direct debit, bank transfer, and more, with dunning that recovers and a ledger that never loses a kobo.

```bash
go get github.com/nombaone/nombaone-go
```

Requires Go 1.23+. Zero dependencies (standard library only).

## Quickstart

Grab a sandbox key (`nbo_sandbox_…`) from the [dashboard](https://console.nombaone.xyz), set it as `NOMBAONE_API_KEY`, and you are three objects away from a live subscription:

```go
package main

import (
	"context"
	"fmt"
	"log"

	nombaone "github.com/nombaone/nombaone-go"
)

func main() {
	client, err := nombaone.New() // reads NOMBAONE_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	plan, _ := client.Plans.Create(ctx, nombaone.PlanCreateParams{Name: "Pro"})
	price, _ := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{
		UnitAmountInKobo: 250_000, // ₦2,500.00 per month
		Interval:         nombaone.PriceIntervalMonth,
	})
	customer, _ := client.Customers.Create(ctx, nombaone.CustomerCreateParams{
		Email: "ada@example.com",
		Name:  "Ada Lovelace",
	})

	// Sandbox: mint a deterministic test card, then subscribe.
	method, _ := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{CustomerID: customer.ID})
	sub, _ := client.Subscriptions.Create(ctx, nombaone.SubscriptionCreateParams{
		CustomerID:      customer.ID,
		PriceID:         price.ID,
		PaymentMethodID: nombaone.String(method.ID),
	})

	fmt.Println(sub.Status) // "active"
}
```

The client derives the host from your key prefix — `nbo_sandbox_…` talks to `https://sandbox.api.nombaone.xyz`, `nbo_live_…` to `https://api.nombaone.xyz`. Server-side only; there is no publishable key to leak.

## Sandbox first

The sandbox runs the real billing engine. `client.Sandbox` gives you the levers to make a month happen in a second:

```go
// A card that declines like a thin balance does — "not yet", not "no".
client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{
	CustomerID: customer.ID,
	Behavior:   nombaone.SandboxBehaviorDeclineInsufficientFunds,
	// or SandboxBehaviorSuccess | SandboxBehaviorRequiresOTP | SandboxBehaviorDeclineExpiredCard | SandboxBehaviorDeclineDoNotHonor
})

// The test clock: force the next billing cycle through the real engine.
cycle, _ := client.Sandbox.AdvanceCycle(ctx, sub.ID)
fmt.Println(cycle.Outcome) // "paid" | "past_due" | …

// Fire a real, signed webhook at your registered endpoints.
client.Sandbox.SimulateWebhook(ctx, nombaone.SandboxSimulateWebhookParams{Type: "invoice.payment_failed"})
```

These methods return `nombaone.ErrSandboxRequiresSandboxKey` locally — before any network call — if used with a live key.

## Money is integer kobo

Every amount in the API is an **integer in kobo**: `₦1.00 = 100`. `250_000` is ₦2,500 — not ₦250,000. No floats, no decimal strings; `currency` is always `"NGN"`. Every money field is suffixed `InKobo` and typed `int64` (aliased `nombaone.Kobo`). Multiply naira by 100 exactly once, at the edge of your system.

## Pagination

Every `List` returns a `*Page[T]` and works three ways:

```go
// One page.
page, _ := client.Invoices.List(ctx, nombaone.InvoiceListParams{Status: nombaone.InvoiceStatusOpen, Limit: nombaone.Int(50)})
page.Data
page.Pagination.HasMore
page.Pagination.NextCursor

// Manual paging.
if page.HasNextPage() {
	next, _ := page.NextPage(ctx)
	_ = next
}

// Or let the SDK thread the cursors (Go 1.23 range-over-func).
for invoice, err := range page.All(ctx) {
	if err != nil {
		return err
	}
	fmt.Println(invoice.ID, invoice.AmountDueInKobo)
}
```

## Errors are a feature

Failures return a typed error carrying everything the API said — the stable `Code` to branch on, a `Hint` telling you exactly what to do next, a `DocURL` into the error reference, per-field details on validation failures, and the `RequestID` to quote to support. Match with `errors.As` against the base `*APIError` or a status-specific type:

```go
import "errors"

_, err := client.Subscriptions.Create(ctx, params)

var validation *nombaone.ValidationError
if errors.As(err, &validation) {
	fmt.Println(validation.Fields) // map[string][]string
}
var rateLimit *nombaone.RateLimitError
if errors.As(err, &rateLimit) {
	fmt.Println(rateLimit.RetryAfter) // seconds
}
var apiErr *nombaone.APIError
if errors.As(err, &apiErr) {
	fmt.Println(apiErr.Code, apiErr.Hint, apiErr.DocURL, apiErr.RequestID)
}
```

| Status | Type                       | Notes                                    |
| ------ | -------------------------- | ---------------------------------------- |
| 400    | `*BadRequestError`         | malformed request                        |
| 401    | `*AuthenticationError`     | missing/invalid/wrong-environment key    |
| 403    | `*PermissionDeniedError`   | missing scope, foreign resource          |
| 404    | `*NotFoundError`           | wrong id or wrong environment            |
| 409    | `*ConflictError`           | state conflicts, idempotency reuse       |
| 422    | `*ValidationError`         | `err.Fields` has the per-field messages  |
| 429    | `*RateLimitError`          | `RetryAfter`, `Limit`, `Remaining`       |
| 5xx    | `*ServerError`             | safe to retry (the SDK already did)      |
| —      | `*ConnectionError` / `*TimeoutError` | transport-level (`errors.Is(err, context.DeadlineExceeded)`) |

## Idempotency & retries

The SDK auto-generates an `Idempotency-Key` for every POST and **reuses it across its automatic retries** (network failures, timeouts, 408/429/5xx, and its own in-flight idempotency conflict — 2 retries by default, honoring `Retry-After`), so a blip can never double-charge. Pass your own key when the operation must stay idempotent across _process_ restarts:

```go
payout, err := client.Settlements.CreatePayout(ctx,
	nombaone.PayoutCreateParams{AmountInKobo: 5_000_000, BankCode: "058", AccountNumber: "0123456789"},
	nombaone.WithIdempotencyKey("payout-"+myPayout.ID), // ⚠ doubles as the payout's durable merchantTxRef
)
```

> **Payout warning:** on `Settlements.CreatePayout` the `Idempotency-Key` doubles as the durable `merchantTxRef`. Always pass an explicit, stable key (e.g. your own payout id) — an auto-generated key protects SDK-level retries, but a brand-new process retrying with a fresh key would create a second payout.

Every method also accepts per-call options as trailing arguments — `WithIdempotencyKey`, `WithHeader`, `WithRequestTimeout`, `WithRequestMaxRetries`, and `WithRawResponse(&resp)` to read response headers. Cancellation is via `context.Context`; a caller-cancelled context is never retried.

## Webhooks

Verify before you parse, and dedupe on the event id — delivery is at-least-once, never exactly-once. The `webhook` package needs only the signing secret (no API key), so a receiver can import it alone:

```go
import (
	"io"
	"net/http"

	"github.com/nombaone/nombaone-go/webhook"
)

func handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body) // the RAW body — never re-serialize; it breaks the signature

	event, err := webhook.ConstructEvent(
		body,
		r.Header.Get("X-Nombaone-Signature"),
		secret, // shown once when you created the endpoint
	)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if alreadyProcessed(event.Event.ID) { // at-least-once ⇒ dedupe on event.Event.ID
		w.WriteHeader(http.StatusOK)
		return
	}

	switch event.Type {
	case webhook.EventTypeInvoicePaid:
		data, _ := webhook.DecodeData[webhook.RefData](event)
		unlock(data.Reference)
	case webhook.EventTypeInvoiceActionRequired:
		data, _ := webhook.DecodeData[webhook.InvoiceActionRequiredData](event)
		send(data.CheckoutLink) // typed
	}
	w.WriteHeader(http.StatusOK) // respond 2xx fast, work async
}
```

`ConstructEvent` checks the `X-Nombaone-Signature` (`t=<unix>,v1=<hex>`, HMAC-SHA256 over `` `{t}.{body}` ``) in constant time, rejects stale timestamps (300s tolerance, configurable via `webhook.WithTolerance`), and returns a typed event whose payload you decode with `webhook.DecodeData[T]`. `webhook.GenerateTestHeader` lets you unit-test your handler. Manage endpoints via `client.WebhookEndpoints` (create/rotate return the secret **exactly once**).

> **Raw body per framework:** capture the body before any middleware parses it. Standard `net/http`: `io.ReadAll(r.Body)`. Gin: `c.GetRawData()`. Echo: `io.ReadAll(c.Request().Body)`. Fiber: `c.Body()`. Never feed `ConstructEvent` a re-marshaled struct.

## The full surface

`Customers` (+credit, discount) · `Plans` (+nested `Prices`) · `Prices` · `Subscriptions` (pause/resume/cancel/resubscribe/change, `Schedule`, `Dunning`, upcoming invoice, events) · `Invoices` · `Coupons` · `PaymentMethods` (hosted-checkout cards, virtual accounts) · `Mandates` (NIBSS direct debit) · `Settlements` (escrow, refunds, payouts) · `WebhookEndpoints` (+`Deliveries`, replay) · `Events` (+catalog) · `Organization` (+`Billing` policy) · `Metrics` · `Sandbox` — every operation in the [API reference](https://docs.nombaone.xyz), 1:1.

Worth knowing:

- **Mandates are asynchronous.** They start `consent_pending` and activate when the customer's bank confirms — listen for `payment_method.updated`, don't poll, don't charge early.
- **Bank transfer is a push rail.** `PaymentMethods.CreateVirtualAccount` issues a NUBAN; collection completes when the transfer arrives and reconciles.
- **`past_due` is not canceled.** Read `client.Subscriptions.Dunning.Retrieve()` and honor `GraceAccessUntil` before cutting anyone off. Involuntary churn is `Status: canceled` with `CancellationReason: involuntary` (there is no `churned` status; there is a `subscription.churned` event).
- **Prices are immutable; plans archive, never delete.**
- **`Mandates.Retrieve` returns a `*PaymentMethod`**, not a mandate object.

## Configuration

```go
client, err := nombaone.New(
	nombaone.WithAPIKey(key),           // default: NOMBAONE_API_KEY
	nombaone.WithBaseURL(url),          // override the derived host
	nombaone.WithTimeout(30*time.Second), // per-attempt timeout
	nombaone.WithMaxRetries(2),         // automatic retry budget
	nombaone.WithHTTPClient(myClient),  // bring your own transport (tests, proxies)
	nombaone.WithDefaultHeader("X-App", "acme"),
)
// client.Mode() and client.BaseURL() are read-only.
```

## Examples & development

Runnable programs live in [`examples/`](examples) — quickstart, pagination, the subscription lifecycle, a webhook receiver, and a dunning rehearsal with the test clock:

```bash
NOMBAONE_API_KEY=nbo_sandbox_… go run ./examples/quickstart
```

To develop the SDK: `go test ./...` runs the unit + conformance suites (no key needed). The live suite is opt-in:

```bash
NOMBAONE_INTEGRATION=1 NOMBAONE_API_KEY=nbo_sandbox_… go test -run TestIntegration ./...
```

## Requirements & versioning

Go 1.23+ (built on `net/http` and range-over-func iterators). Semantic versioning; the API itself is versioned at `/v1` and additive changes never break you. MIT licensed.
