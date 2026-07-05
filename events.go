package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// EventCatalogEntry is one entry in the event catalog: when the event fires and
// which data keys it carries.
type EventCatalogEntry struct {
	When    string   `json:"when"`
	Payload []string `json:"payload"`
}

// EventListParams are the filters for EventsService.List.
type EventListParams struct {
	// Type filters to one catalog type, e.g. "invoice.paid".
	Type   *string
	Limit  *int
	Cursor *string
}

func (p EventListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryStr(q, "type", p.Type)
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// EventsService is the events namespace — the append-only log behind every
// webhook. Webhook delivery is at-least-once; this log is your reconciliation
// backstop when a delivery was missed or you need to backfill.
type EventsService struct {
	client *Client
}

// List returns events, newest first.
//
//	for event, err := range client.Events.List(ctx, nombaone.EventListParams{
//		Type: nombaone.String("invoice.paid"),
//	}).All(ctx) {
//		if err != nil { return err }
//		fmt.Println(event.ID, event.Payload)
//	}
func (s *EventsService) List(ctx context.Context, params EventListParams, opts ...RequestOption) (*Page[DomainEvent], error) {
	return executePage[DomainEvent](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/events", query: params.toQuery(), opts: opts,
	})
}

// Retrieve returns one event by id (nbo…evt).
func (s *EventsService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*DomainEvent, error) {
	res, err := execute[DomainEvent](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/events/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Catalog returns the machine-readable event catalog — every event type the
// platform can emit, keyed by type, with a description and its data keys.
func (s *EventsService) Catalog(ctx context.Context, opts ...RequestOption) (map[string]EventCatalogEntry, error) {
	res, err := execute[map[string]EventCatalogEntry](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/events/catalog", opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}
