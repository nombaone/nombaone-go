# Build summary — nombaone-go v0.1.0

Official Go SDK for the NombaOne subscription-billing API. Module
`github.com/nombaone/nombaone-go`, Go 1.23+, **zero third-party dependencies**.

## What shipped

- Full **83-operation** surface across **15 namespaces** plus a standalone
  `github.com/nombaone/nombaone-go/webhook` package (usable without an API key).
- Transport with the money-safety invariants: `/v1` applied once, idempotency
  key computed once **before** the retry loop, retries on transport/timeout/
  408/429/5xx and 409-only-when-`IDEMPOTENCY_IN_PROGRESS`, caller cancellation
  never retried, full-jitter backoff honoring `Retry-After`.
- Typed error hierarchy (`errors.As`), open `ErrorCode` (72 codes), cursor
  pagination with a Go 1.23 `iter.Seq2` auto-pager, integer-kobo money,
  pointer/`Optional[T]` optionals, sandbox live-key local guard.
- Webhook helper implementing the **documented** `t=<unix>,v1=<hex>` scheme;
  golden vector passes **byte-for-byte**.

## Verification performed

- **~296 unit/wire tests**, `-race` clean; `gofmt`, `go vet`, and `staticcheck`
  all clean on the module and the `webhook` package.
- **Same-idempotency-key-across-retries** unit test passes.
- **Webhook golden vector + full rejection matrix** pass.
- **Bidirectional OpenAPI conformance** (all 78 non-excluded spec ops covered,
  no SDK call outside the spec); the **deliberate-break drill** was performed
  (breaking one path turns the suite red naming the route, then reverted).
- **Live integration against the deployed sandbox** (`https://sandbox.api.nombaone.xyz`):
  the core lifecycle suite (7 checks) **and** a full-surface suite that exercises
  every method across all 15 namespaces and **asserts the `domain` discriminator
  on returned objects** (so a silent wire/model mismatch is a defect, not a pass).
  Owner-legible verdict: **`89 method checks across 15 namespaces | ok 83 |
  expected-errors 6 | DEFECTS 0`**. Each method passes only on success or a
  *specific expected* typed API error.
- **Pre-release bug caught by the gate:** `subscriptions.UpdatePaymentMethod`
  returns a **PaymentMethod** on the wire (domain `payment_method`, id `…pmt`),
  not the Subscription the spec claims — verified live and corrected before
  release (the same mismatch the Ruby/Rust/.NET SDKs hit). The strengthened
  domain assertions now lock it.
- Module **consumed from a scratch external module** (via local `replace`) and
  built + vetted clean.

## New backend quirks discovered (deployed sandbox, 2026-07-05) — report to the operator

1. **`POST /v1/mandates` returns HTTP 504.** The NIBSS mandate upstream is
   unavailable in the deployed sandbox; a single attempt 504s in ~700ms. The
   SDK behaves correctly — a 504 is a retryable server error, so with default
   retries it retries (honoring any `Retry-After`) and then surfaces a typed
   `*ServerError` with code `SYSTEM_UPSTREAM_ERROR`. Mandate creation could not
   be exercised to `consent_pending` on this environment; the request shape and
   error surfacing are confirmed correct. (Retried 504s can take minutes end to
   end — for a known-dead endpoint, pass `WithRequestMaxRetries(0)` to fail
   fast, as the integration suite does.)
2. **Settlement subaccount not configured on the sandbox org.** Escrow,
   settlement-list, and payout reads return the typed
   `SETTLEMENT_SUBACCOUNT_NOT_FOUND` ("Create and verify the subaccount before
   recording splits or payouts"). A legitimate business state, surfaced
   correctly by the SDK — not a client error.

## Brief quirks (§10) confirmed to hold on the wire

- Creates return **201**; any 2xx is treated as success (verified end to end).
- `mode` is **`sandbox`** on the wire (spec says `test`) — every created
  resource reported `mode: sandbox`.
- Filter-name inconsistencies are wire law — `customerRef` (payment-methods)
  vs `customerId` (subscriptions/invoices) vs `planRef` (prices) all accepted.
- Invoice `void` on a paid invoice returns `INVOICE_NOT_VOIDABLE`.
- Every response carries `X-Request-Id`; error envelopes carry
  `code`/`hint`/`docUrl`/`requestId` (verified via typed-error assertions).
- Event catalog returned **34** event types.

## Not done (out of SDK scope / needs operator)

- Publish to a Git remote + tag `v0.1.0` (needs GitHub org access; release
  workflow is tag-driven and ready).
- Webhook **round-trip** against a live delivery is skipped for a remote sandbox
  (a deployed API cannot reach a `127.0.0.1` listener); the golden vector proves
  the SDK's documented-scheme implementation. Run it against a local API to
  exercise live delivery.
