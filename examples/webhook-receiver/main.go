// Command webhook-receiver is a minimal HTTP server that verifies NombaOne
// webhook deliveries with the standalone webhook package (no API key needed)
// and dedupes on the event id. Point a webhook endpoint at http://<host>/nombaone/webhooks.
//
//	NOMBAONE_WEBHOOK_SECRET=nbo_whsec_… PORT=8080 go run ./examples/webhook-receiver
package main

import (
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/nombaone/nombaone-go/webhook"
)

// seen dedupes deliveries — delivery is at-least-once, never exactly-once.
var (
	mu   sync.Mutex
	seen = map[string]bool{}
)

func alreadyProcessed(eventID string) bool {
	mu.Lock()
	defer mu.Unlock()
	if seen[eventID] {
		return true
	}
	seen[eventID] = true
	return false
}

func main() {
	secret := os.Getenv("NOMBAONE_WEBHOOK_SECRET")
	if secret == "" {
		log.Fatal("set NOMBAONE_WEBHOOK_SECRET (shown once when the endpoint was created)")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/nombaone/webhooks", func(w http.ResponseWriter, r *http.Request) {
		// Read the RAW body — never re-serialize; it would break the signature.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		event, err := webhook.ConstructEvent(body, r.Header.Get("X-Nombaone-Signature"), secret)
		if err != nil {
			log.Printf("rejected delivery: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		// Dedupe on the underlying event id, then respond 2xx fast.
		if alreadyProcessed(event.Event.ID) {
			w.WriteHeader(http.StatusOK)
			return
		}

		switch event.Type {
		case webhook.EventTypeInvoicePaid:
			data, _ := webhook.DecodeData[webhook.RefData](event)
			log.Printf("invoice paid: %s", data.Reference)
		case webhook.EventTypeInvoicePaymentFailed:
			data, _ := webhook.DecodeData[webhook.InvoicePaymentFailedData](event)
			log.Printf("payment failed for %s: %s", data.Reference, data.Reason)
		case webhook.EventTypeInvoiceActionRequired:
			data, _ := webhook.DecodeData[webhook.InvoiceActionRequiredData](event)
			log.Printf("action required for %s → %s", data.Reference, data.CheckoutLink)
		default:
			log.Printf("event %s (%s)", event.Type, event.Event.ID)
		}

		w.WriteHeader(http.StatusOK) // do heavy work async
	})

	log.Printf("listening on :%s/nombaone/webhooks", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
