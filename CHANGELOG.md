# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.1.0] - 2026-07-05

Initial release of the official Go SDK for the NombaOne subscription-billing API.

### Added

- **Client** — `nombaone.New(...)` with functional options (`WithAPIKey`,
  `WithBaseURL`, `WithHTTPClient`, `WithTimeout`, `WithMaxRetries`,
  `WithDefaultHeader`). Host is derived from the key prefix; read-only `Mode()`
  and `BaseURL()`.
- **Transport** — `/v1` applied once; the `Idempotency-Key` for a POST is
  computed once before the retry loop so automatic retries replay the same
  operation; retries cover transport failures, timeouts, 408/429/5xx and 409
  only when `IDEMPOTENCY_IN_PROGRESS`; caller cancellation (`context`) is never
  retried; full-jitter backoff honors `Retry-After`; any 2xx is success. Zero
  third-party dependencies.
- **Errors** — open `ErrorCode` with the ~72 public codes; an `APIError` base
  carrying `Code`/`Hint`/`DocURL`/`Fields`/`RequestID` (hint folded into
  `Error()`), status-typed errors reachable via `errors.As`, plus
  `ConnectionError`/`TimeoutError` (unwraps to `context.DeadlineExceeded`).
- **Pagination** — `*Page[T]` with `HasNextPage`/`NextPage` and a Go 1.23
  `All(ctx)` range-over-func auto-pager that threads cursors and preserves
  filters.
- **Resources** — the full 15-namespace surface (83 operations): customers
  (+credit, discount), plans (+nested prices), prices, subscriptions
  (+schedule, +dunning, upcoming invoice, events), invoices, coupons, payment
  methods, mandates, settlements, webhook endpoints (+deliveries), events,
  organization (+billing), metrics, and the sandbox toolkit (which fails
  locally with `ErrSandboxRequiresSandboxKey` when given a live key).
- **Webhooks** — standalone `github.com/nombaone/nombaone-go/webhook` package
  (no API key required): `ConstructEvent`, `VerifySignature`,
  `GenerateTestHeader`, the open ~32-type event catalog, and a generic
  `DecodeData[T]` helper. Passes the golden signature vector byte-for-byte.
- **Money** — integer kobo everywhere (`int64`, aliased `Kobo`); `currency`
  always `"NGN"`.
- **Tests** — unit + wire tests for every method, the same-idempotency-key-
  across-retries invariant, the webhook rejection matrix, a bidirectional
  OpenAPI conformance suite, and an env-gated live integration suite.

[Unreleased]: https://github.com/nombaone/nombaone-go/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/nombaone/nombaone-go/releases/tag/v0.1.0
