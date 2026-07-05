// Command quickstart takes a sandbox key from NOMBAONE_API_KEY and reaches a
// live, active subscription in a handful of calls: plan → price → customer →
// sandbox card → subscription.
//
//	NOMBAONE_API_KEY=nbo_sandbox_… go run ./examples/quickstart
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	nombaone "github.com/nomba/nomba-go"
)

func main() {
	client, err := nombaone.New() // reads NOMBAONE_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	fmt.Printf("talking to %s (%s)\n", client.BaseURL(), client.Mode())

	plan, err := client.Plans.Create(ctx, nombaone.PlanCreateParams{Name: "Pro"})
	if err != nil {
		log.Fatalf("create plan: %v", err)
	}

	price, err := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{
		UnitAmountInKobo: 250_000, // ₦2,500.00 per month
		Interval:         nombaone.PriceIntervalMonth,
	})
	if err != nil {
		log.Fatalf("create price: %v", err)
	}

	customer, err := client.Customers.Create(ctx, nombaone.CustomerCreateParams{
		Email: fmt.Sprintf("ada+%d@example.com", time.Now().UnixNano()),
		Name:  "Ada Lovelace",
	})
	if err != nil {
		log.Fatalf("create customer: %v", err)
	}

	// Sandbox: mint a deterministic test card, then subscribe.
	method, err := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{
		CustomerID: customer.ID,
	})
	if err != nil {
		log.Fatalf("sandbox payment method: %v", err)
	}

	sub, err := client.Subscriptions.Create(ctx, nombaone.SubscriptionCreateParams{
		CustomerID:      customer.ID,
		PriceID:         price.ID,
		PaymentMethodID: nombaone.String(method.ID),
	})
	if err != nil {
		log.Fatalf("create subscription: %v", err)
	}

	fmt.Printf("plan     %s\n", plan.ID)
	fmt.Printf("price    %s  (₦%d/%s)\n", price.ID, price.UnitAmountInKobo/100, price.Interval)
	fmt.Printf("customer %s  (%s)\n", customer.ID, customer.Email)
	fmt.Printf("method   %s  (%s)\n", method.ID, method.Kind)
	fmt.Printf("subscription %s is %s\n", sub.ID, sub.Status)
}
