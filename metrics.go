package nombaone

import (
	"context"
	"net/http"
	"net/url"
)

// DunningFunnel is the recovery funnel counts inside a metrics window.
type DunningFunnel struct {
	Scheduled          int `json:"scheduled"`
	Attempting         int `json:"attempting"`
	CardUpdateRequired int `json:"cardUpdateRequired"`
	Rescheduled        int `json:"rescheduled"`
	Succeeded          int `json:"succeeded"`
	Exhausted          int `json:"exhausted"`
}

// BillingMetrics are billing KPIs, computed from the ledger on read — never
// stored, never stale.
type BillingMetrics struct {
	Domain string `json:"domain"` // "billing_metrics"
	// MrrInKobo is monthly recurring revenue, integer kobo.
	MrrInKobo           Kobo          `json:"mrrInKobo"`
	ActiveCount         int           `json:"activeCount"`
	VoluntaryChurn      int           `json:"voluntaryChurn"`
	InvoluntaryChurn    int           `json:"involuntaryChurn"`
	FailedChargeRate    float64       `json:"failedChargeRate"`
	DunningRecoveryRate float64       `json:"dunningRecoveryRate"`
	DunningFunnel       DunningFunnel `json:"dunningFunnel"`
	WindowFrom          string        `json:"windowFrom"`
	WindowTo            string        `json:"windowTo"`
}

// BillingMetricsParams are the inputs to MetricsService.Billing.
type BillingMetricsParams struct {
	// From is an ISO-8601 date-time, start of the window.
	From *string
	// To is an ISO-8601 date-time, end of the window.
	To *string
}

func (p BillingMetricsParams) toQuery() url.Values {
	q := url.Values{}
	addQueryStr(q, "from", p.From)
	addQueryStr(q, "to", p.To)
	return q
}

// MetricsService is the metrics namespace — MRR, churn, and the dunning funnel.
type MetricsService struct {
	client *Client
}

// Billing returns billing KPIs over a window (defaults to a recent window
// server-side).
//
//	metrics, err := client.Metrics.Billing(ctx, nombaone.BillingMetricsParams{})
//	// fmt.Printf("MRR ₦%d\n", metrics.MrrInKobo/100)
func (s *MetricsService) Billing(ctx context.Context, params BillingMetricsParams, opts ...RequestOption) (*BillingMetrics, error) {
	res, err := execute[BillingMetrics](ctx, s.client, requestSpec{
		method: http.MethodGet, path: "/metrics/billing", query: params.toQuery(), opts: opts,
	})
	if err != nil {
		return nil, err
	}
	return &res.Data, nil
}
