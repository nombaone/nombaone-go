// Command pagination shows the three ways to page a list: one page, manual
// cursor walking, and range-over-func auto-iteration.
//
//	NOMBAONE_API_KEY=nbo_sandbox_… go run ./examples/pagination
package main

import (
	"context"
	"fmt"
	"log"

	nombaone "github.com/nombaone/nombaone-go"
)

func main() {
	client, err := nombaone.New()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	// 1. One page.
	page, err := client.Customers.List(ctx, nombaone.CustomerListParams{Limit: nombaone.Int(5)})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("page: %d customers, hasMore=%v\n", len(page.Data), page.Pagination.HasMore)

	// 2. Manual paging.
	if page.HasNextPage() {
		next, err := page.NextPage(ctx)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("next page: %d customers\n", len(next.Data))
	}

	// 3. Auto-iteration — cursors handled for you. Stop early after 12.
	count := 0
	for customer, err := range page.All(ctx) {
		if err != nil {
			log.Fatal(err)
		}
		count++
		fmt.Printf("  %2d. %s  %s\n", count, customer.ID, customer.Email)
		if count >= 12 {
			break
		}
	}
	fmt.Printf("iterated %d customers across pages\n", count)
}
