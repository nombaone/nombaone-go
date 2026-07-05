// Command sandbox-dunning rehearses involuntary churn without waiting on a real
// cron: it subscribes with a trial and a card that declines like a thin
// balance, then uses the test clock (advance-cycle) to run the first real
// charge, and reads the dunning state the failure produced.
//
//	NOMBAONE_API_KEY=nbo_sandbox_… go run ./examples/sandbox-dunning
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	nombaone "github.com/nomba/nomba-go"
)

func main() {
	client, err := nombaone.New()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	plan, err := client.Plans.Create(ctx, nombaone.PlanCreateParams{Name: fmt.Sprintf("Dunning %d", time.Now().UnixNano())})
	if err != nil {
		log.Fatal(err)
	}
	price, err := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{
		UnitAmountInKobo: 250_000,
		Interval:         nombaone.PriceIntervalMonth,
	})
	if err != nil {
		log.Fatal(err)
	}
	customer, err := client.Customers.Create(ctx, nombaone.CustomerCreateParams{
		Email: fmt.Sprintf("dunning+%d@example.com", time.Now().UnixNano()),
		Name:  "Katherine Johnson",
	})
	if err != nil {
		log.Fatal(err)
	}

	// A card that declines like a thin balance — "not yet", not "no".
	method, err := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{
		CustomerID: customer.ID,
		Behavior:   nombaone.SandboxBehaviorDeclineInsufficientFunds,
	})
	if err != nil {
		log.Fatal(err)
	}

	// Subscribe with a trial so creation succeeds; the first real charge is
	// deferred to trial end (rehearse the decline via the test clock, not at
	// create — a first-charge decline currently 422s).
	sub, err := client.Subscriptions.Create(ctx, nombaone.SubscriptionCreateParams{
		CustomerID:      customer.ID,
		PriceID:         price.ID,
		PaymentMethodID: nombaone.String(method.ID),
		TrialDays:       nombaone.Int(14),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("subscribed with trial: %s → %s\n", sub.ID, sub.Status)

	// The test clock: run the next cycle now, through the real engine.
	result, err := client.Sandbox.AdvanceCycle(ctx, sub.ID)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("advanced cycle: outcome=%s invoice=%s (₦%d)\n",
		result.Outcome, result.Invoice.ID, result.Invoice.TotalInKobo/100)

	// Read where the subscription stands in recovery. past_due is not canceled —
	// honor graceAccessUntil before cutting anyone off.
	dunning, err := client.Subscriptions.Dunning.Retrieve(ctx, sub.ID)
	if err != nil {
		log.Fatal(err)
	}
	grace := "n/a"
	if dunning.GraceAccessUntil != nil {
		grace = *dunning.GraceAccessUntil
	}
	fmt.Printf("dunning: status=%s attempts=%d/%d graceAccessUntil=%s\n",
		dunning.Status, dunning.AttemptsUsed, dunning.MaxAttempts, grace)

	attempts, err := client.Subscriptions.Dunning.ListAttempts(ctx, sub.ID, nombaone.DunningListAttemptsParams{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("recovery attempts so far: %d\n", len(attempts.Data))
}
