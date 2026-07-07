package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// WebhookEndpoint is a URL you registered to receive signed event deliveries.
type WebhookEndpoint struct {
	Domain string `json:"domain"` // "webhook"
	ID     string `json:"id"`     // nbo…whk
	URL    string `json:"url"`
	// EnabledEvents are the event types fanned out to this endpoint; ["*"]
	// means everything.
	EnabledEvents []string `json:"enabledEvents"`
	// SigningSecretPrefix is the display prefix of the signing secret (the
	// full secret is shown once).
	SigningSecretPrefix string  `json:"signingSecretPrefix"`
	DisabledAt          *string `json:"disabledAt"`
	CreatedAt           string  `json:"createdAt"`
}

// WebhookEndpointWithSecret is returned by Create only — the one time the full
// secret is visible.
type WebhookEndpointWithSecret struct {
	WebhookEndpoint
	// SigningSecret is the full signing secret. Shown exactly once — store it
	// now; it is not recoverable later (only rotatable).
	SigningSecret string `json:"signingSecret"`
}

// RotatedWebhookSecret is returned by RotateSecret — again, the only time this
// secret is visible.
type RotatedWebhookSecret struct {
	Domain string `json:"domain"` // "webhook_secret"
	ID     string `json:"id"`
	// SigningSecret is the new full signing secret — shown exactly once.
	SigningSecret       string `json:"signingSecret"`
	SigningSecretPrefix string `json:"signingSecretPrefix"`
}

// WebhookDeliveryStatus is the state of a delivery attempt.
type WebhookDeliveryStatus string

const (
	WebhookDeliveryStatusPending   WebhookDeliveryStatus = "pending"
	WebhookDeliveryStatusSucceeded WebhookDeliveryStatus = "succeeded"
	WebhookDeliveryStatusFailed    WebhookDeliveryStatus = "failed"
	WebhookDeliveryStatusDead      WebhookDeliveryStatus = "dead"
)

// WebhookDelivery is one attempt-tracked delivery of an event to one endpoint.
type WebhookDelivery struct {
	Domain     string `json:"domain"` // "webhook_delivery"
	ID         string `json:"id"`     // nbo…whd
	EventType  string `json:"eventType"`
	EndpointID string `json:"endpointId"`
	// EventID is the domain event this delivery carries (nbo…evt) — the dedupe
	// key.
	EventID        string                `json:"eventId"`
	Status         WebhookDeliveryStatus `json:"status"`
	Attempts       int                   `json:"attempts"`
	NextAttemptAt  *string               `json:"nextAttemptAt"`
	LastAttemptAt  *string               `json:"lastAttemptAt"`
	ResponseStatus *int                  `json:"responseStatus"`
	ReplayedAt     *string               `json:"replayedAt"`
	ReplayCount    int                   `json:"replayCount"`
	CreatedAt      string                `json:"createdAt"`
}

// WebhookEndpointCreateParams are the inputs to WebhookEndpointsService.Create.
type WebhookEndpointCreateParams struct {
	URL string `json:"url"`
	// EnabledEvents defaults to ["*"] (all events) server-side.
	EnabledEvents []string `json:"enabledEvents,omitempty"`
}

// WebhookEndpointUpdateParams are the inputs to WebhookEndpointsService.Update.
// At least one field must be set.
type WebhookEndpointUpdateParams struct {
	URL           *string  `json:"url,omitempty"`
	EnabledEvents []string `json:"enabledEvents,omitempty"`
	// Disabled true pauses deliveries; false re-enables.
	Disabled *bool `json:"disabled,omitempty"`
}

// WebhookDeliveryListParams are the filters for
// WebhookEndpointDeliveriesService.List.
type WebhookDeliveryListParams struct {
	Status    WebhookDeliveryStatus
	EventType *string
	Endpoint  *string
	Limit     *int
	Cursor    *string
}

func (p WebhookDeliveryListParams) toQuery() url.Values {
	q := url.Values{}
	addQueryEnum(q, "status", p.Status)
	addQueryStr(q, "eventType", p.EventType)
	addQueryStr(q, "endpoint", p.Endpoint)
	addQueryInt(q, "limit", p.Limit)
	addQueryStr(q, "cursor", p.Cursor)
	return q
}

// WebhookEndpointDeliveriesService is the deliveries sub-namespace: inspect and
// replay deliveries under an endpoint.
type WebhookEndpointDeliveriesService struct {
	client *Client
}

// List returns an endpoint's deliveries, newest first.
func (s *WebhookEndpointDeliveriesService) List(ctx context.Context, endpointID string, params WebhookDeliveryListParams, opts ...RequestOption) (*Page[WebhookDelivery], error) {
	return executePage[WebhookDelivery](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/webhooks/" + seg(endpointID) + "/deliveries", query: params.toQuery(), opts: opts,
	})
}

// Retrieve returns one delivery.
func (s *WebhookEndpointDeliveriesService) Retrieve(ctx context.Context, endpointID, deliveryID string, opts ...RequestOption) (*WebhookDelivery, error) {
	res, err := execute[WebhookDelivery](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/webhooks/" + seg(endpointID) + "/deliveries/" + seg(deliveryID), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Replay redelivers a past delivery. The original event id is kept, so a
// receiver that dedupes on event.id will correctly treat it as already-seen.
func (s *WebhookEndpointDeliveriesService) Replay(ctx context.Context, endpointID, deliveryID string, opts ...RequestOption) (*WebhookDelivery, error) {
	res, err := execute[WebhookDelivery](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/webhooks/" + seg(endpointID) + "/deliveries/" + seg(deliveryID) + "/replay", body: struct{}{}, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// WebhookEndpointsService registers and manages the URLs that receive signed
// events. To verify incoming deliveries in your handler, use the standalone
// github.com/nombaone/nombaone-go/webhook package — the crypto helper, not this REST
// resource.
type WebhookEndpointsService struct {
	client *Client
	// Deliveries is the deliveries-under-an-endpoint sub-namespace.
	Deliveries *WebhookEndpointDeliveriesService
}

// Create registers an endpoint. The response includes the full SigningSecret
// exactly once — store it in your secret manager immediately.
//
//	endpoint, err := client.WebhookEndpoints.Create(ctx, nombaone.WebhookEndpointCreateParams{
//		URL:           "https://example.com/nombaone/webhooks",
//		EnabledEvents: []string{"invoice.paid", "invoice.payment_failed"},
//	})
//	// store endpoint.SigningSecret now — it is not shown again
func (s *WebhookEndpointsService) Create(ctx context.Context, params WebhookEndpointCreateParams, opts ...RequestOption) (*WebhookEndpointWithSecret, error) {
	res, err := execute[WebhookEndpointWithSecret](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/webhooks", body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Retrieve returns an endpoint by id.
func (s *WebhookEndpointsService) Retrieve(ctx context.Context, id string, opts ...RequestOption) (*WebhookEndpoint, error) {
	res, err := execute[WebhookEndpoint](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/webhooks/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// Update changes the url, event subscription, or enabled state.
func (s *WebhookEndpointsService) Update(ctx context.Context, id string, params WebhookEndpointUpdateParams, opts ...RequestOption) (*WebhookEndpoint, error) {
	res, err := execute[WebhookEndpoint](ctx, s.client, requestSpec{
		method: http.MethodPatch, path: "/webhooks/" + seg(id), body: params, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// List returns your endpoints.
func (s *WebhookEndpointsService) List(ctx context.Context, opts ...RequestOption) (*Page[WebhookEndpoint], error) {
	return executePage[WebhookEndpoint](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/webhooks", opts: opts,
	})
}

// Delete deletes an endpoint. Pending deliveries to it are retired.
func (s *WebhookEndpointsService) Delete(ctx context.Context, id string, opts ...RequestOption) (*WebhookEndpoint, error) {
	res, err := execute[WebhookEndpoint](ctx, s.client, requestSpec{
		method: http.MethodDelete, path: "/webhooks/" + seg(id), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}

// RotateSecret rotates the signing secret. The new secret is returned exactly
// once; the old one is briefly honored so you can roll without dropping
// in-flight deliveries.
func (s *WebhookEndpointsService) RotateSecret(ctx context.Context, id string, opts ...RequestOption) (*RotatedWebhookSecret, error) {
	res, err := execute[RotatedWebhookSecret](ctx, s.client, requestSpec{
		method: http.MethodPost, path: "/webhooks/" + seg(id) + "/rotate-secret", body: struct{}{}, opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
