// Command lifecycle walks a subscription through its states: create → change
// (prorated upgrade) → pause → resume → cancel, printing the status at each
// step and demonstrating typed error handling.
//
//	NOMBAONE_API_KEY=nbo_sandbox_… go run ./examples/lifecycle
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	nombaone "github.com/nombaone/nombaone-go"
)

func main() {
	client, err := nombaone.New()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	plan, err := client.Plans.Create(ctx, nombaone.PlanCreateParams{Name: fmt.Sprintf("Lifecycle %d", time.Now().UnixNano())})
	if err != nil {
		log.Fatal(err)
	}
	starter, err := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{UnitAmountInKobo: 100_000, Interval: nombaone.PriceIntervalMonth})
	if err != nil {
		log.Fatal(err)
	}
	pro, err := client.Plans.Prices.Create(ctx, plan.ID, nombaone.PriceCreateParams{UnitAmountInKobo: 300_000, Interval: nombaone.PriceIntervalMonth})
	if err != nil {
		log.Fatal(err)
	}
	customer, err := client.Customers.Create(ctx, nombaone.CustomerCreateParams{
		Email: fmt.Sprintf("lifecycle+%d@example.com", time.Now().UnixNano()),
		Name:  "Grace Hopper",
	})
	if err != nil {
		log.Fatal(err)
	}
	method, err := client.Sandbox.CreatePaymentMethod(ctx, nombaone.SandboxPaymentMethodParams{CustomerID: customer.ID})
	if err != nil {
		log.Fatal(err)
	}

	sub, err := client.Subscriptions.Create(ctx, nombaone.SubscriptionCreateParams{
		CustomerID:      customer.ID,
		PriceID:         starter.ID,
		PaymentMethodID: nombaone.String(method.ID),
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("created:  %s → %s\n", sub.ID, sub.Status)

	upgraded, err := client.Subscriptions.Change(ctx, sub.ID, nombaone.SubscriptionChangeParams{PriceID: nombaone.String(pro.ID)})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("upgraded: now on price %s (prorated)\n", upgraded.PriceID)

	paused, err := client.Subscriptions.Pause(ctx, sub.ID, nombaone.SubscriptionPauseParams{})
	if err != nil {
		// Pausing an incomplete subscription is an illegal transition — show the typed error.
		var conflict *nombaone.ConflictError
		if errors.As(err, &conflict) {
			fmt.Printf("pause refused: %s (%s)\n", conflict.Code, conflict.Hint)
		} else {
			log.Fatal(err)
		}
	} else {
		fmt.Printf("paused:   %s\n", paused.Status)
		resumed, err := client.Subscriptions.Resume(ctx, sub.ID)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("resumed:  %s\n", resumed.Status)
	}

	canceled, err := client.Subscriptions.Cancel(ctx, sub.ID, nombaone.SubscriptionCancelParams{Mode: nombaone.CancelModeNow})
	if err != nil {
		log.Fatal(err)
	}
	reason := "n/a"
	if canceled.CancellationReason != nil {
		reason = string(*canceled.CancellationReason)
	}
	fmt.Printf("canceled: %s (%s)\n", canceled.Status, reason)
}
