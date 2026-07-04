// Package nombaone is the official Go SDK for the NombaOne subscription-billing
// API — recurring billing for Nigeria over card, direct debit, bank transfer,
// and more, with dunning that recovers and a ledger that never loses a kobo.
//
// # Quickstart
//
// Grab a sandbox key (nbo_sandbox_…), set it as NOMBAONE_API_KEY, and you are
// three objects away from an active subscription:
//
//	client, err := nombaone.New() // reads NOMBAONE_API_KEY
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	plan, _ := client.Plans.Create(ctx, nombaone.PlanCreateParams{Name: "Pro"})
//	price, _ := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{
//		UnitAmountInKobo: 250_000, // ₦2,500.00 / month
//		Interval:         "month",
//	})
//	customer, _ := client.Customers.Create(ctx, nombaone.CustomerCreateParams{
//		Email: "ada@example.com",
//		Name:  "Ada Lovelace",
//	})
//
//	method, _ := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{
//		CustomerID: customer.ID,
//	})
//	sub, _ := client.Subscriptions.Create(ctx, nombaone.SubscriptionCreateParams{
//		CustomerID:      customer.ID,
//		PriceID:         price.ID,
//		PaymentMethodID: nombaone.String(method.ID),
//	})
//	fmt.Println(sub.Status) // "active"
//
// # Money is integer kobo
//
// Every amount in the API is an integer number of kobo: ₦1.00 = 100. 250_000
// is ₦2,500 — not ₦250,000. No floats, no decimal strings; currency is always
// "NGN". Every money field is suffixed InKobo.
//
// # Errors are a feature
//
// Failed calls return a typed error carrying the stable Code to branch on, a
// Hint telling you exactly what to do next, a DocURL into the error reference,
// per-field details on validation failures, and the RequestID to quote to
// support. Match with errors.As against [APIError] or a status-specific type
// such as [NotFoundError] or [RateLimitError].
//
// The client is server-side only — there is no publishable key. Verify incoming
// webhooks with the standalone [github.com/nomba/nomba-go/webhook] package,
// which needs only the endpoint's signing secret, never an API key.
package nombaone
