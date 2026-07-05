package nombaone

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultSandboxBaseURL = "https://sandbox.api.nombaone.xyz"
	defaultLiveBaseURL    = "https://api.nombaone.xyz"
	defaultTimeout        = 30 * time.Second
	defaultMaxRetries     = 2
	apiKeyEnvVar          = "NOMBAONE_API_KEY"
)

// Client is the NombaOne API client. Construct it with [New] and reach the API
// through its resource services (Customers, Subscriptions, …). It is safe for
// concurrent use by multiple goroutines.
type Client struct {
	apiKey        string
	baseURL       string
	httpClient    HTTPClient
	timeout       time.Duration
	maxRetries    int
	defaultHeader http.Header
	mode          Mode

	// Customers is the customers namespace — the people and businesses you
	// bill, plus their credit and discounts.
	Customers *CustomersService
	// Plans is your catalog. Prices nest under Plans.Prices.
	Plans *PlansService
	// Prices reads and deactivates prices (create/list under Plans.Prices).
	Prices *PricesService
	// Subscriptions is the core billing object, with Schedule and Dunning
	// sub-namespaces.
	Subscriptions *SubscriptionsService
	// Invoices reads what billing produced, and voids the uncollectible.
	Invoices *InvoicesService
	// Coupons are reusable discount rules.
	Coupons *CouponsService
}

// New constructs a client. The API key is taken from [WithAPIKey] or, when
// absent, the NOMBAONE_API_KEY environment variable. The host is derived from
// the key prefix (nbo_sandbox_… → sandbox, nbo_live_… → live) unless
// overridden with [WithBaseURL]. It returns an error — never panics — when the
// key is missing or its prefix is unrecognized and no base URL was given.
//
//	client, err := nombaone.New() // reads NOMBAONE_API_KEY
//	client, err := nombaone.New(nombaone.WithAPIKey("nbo_sandbox_…"))
func New(opts ...Option) (*Client, error) {
	cfg := &clientConfig{
		timeout:    defaultTimeout,
		maxRetries: defaultMaxRetries,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	apiKey := cfg.apiKey
	if !cfg.apiKeySet {
		apiKey = os.Getenv(apiKeyEnvVar)
	}
	if apiKey == "" {
		return nil, fmt.Errorf(
			"nombaone: missing API key — set %s or pass nombaone.WithAPIKey(\"nbo_sandbox_…\"). Create keys in the dashboard under API keys",
			apiKeyEnvVar,
		)
	}

	mode := deriveMode(apiKey)
	if mode == "" && cfg.baseURL == "" {
		return nil, fmt.Errorf(
			"nombaone: unrecognized API key format — expected a key starting with \"nbo_sandbox_\" or \"nbo_live_\". Copy the key exactly as shown in the dashboard, or pass nombaone.WithBaseURL(...) to target a custom host",
		)
	}
	effectiveMode := mode
	if effectiveMode == "" {
		effectiveMode = ModeSandbox
	}

	baseURL := cfg.baseURL
	if baseURL == "" {
		baseURL = defaultBaseURL(effectiveMode)
	}
	baseURL = strings.TrimRight(baseURL, "/")

	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = &http.Client{}
	}

	c := &Client{
		apiKey:        apiKey,
		baseURL:       baseURL,
		httpClient:    httpClient,
		timeout:       cfg.timeout,
		maxRetries:    cfg.maxRetries,
		defaultHeader: cfg.defaultHeader,
		mode:          effectiveMode,
	}
	c.initResources()
	return c, nil
}

// initResources wires every resource service to the client. Extended as each
// resource namespace is added.
func (c *Client) initResources() {
	c.Customers = &CustomersService{client: c}

	c.Plans = &PlansService{client: c}
	c.Plans.Prices = &PlanPricesService{client: c}
	c.Prices = &PricesService{client: c}

	c.Subscriptions = &SubscriptionsService{client: c}
	c.Subscriptions.Schedule = &SubscriptionScheduleService{client: c}
	c.Subscriptions.Dunning = &SubscriptionDunningService{client: c}

	c.Invoices = &InvoicesService{client: c}
	c.Coupons = &CouponsService{client: c}
}

// Mode reports the environment this client talks to, derived from the key
// prefix. It is read-only.
func (c *Client) Mode() Mode { return c.mode }

// BaseURL reports the API origin in use (no /v1). It is read-only.
func (c *Client) BaseURL() string { return c.baseURL }

func deriveMode(apiKey string) Mode {
	switch {
	case strings.HasPrefix(apiKey, "nbo_sandbox_"):
		return ModeSandbox
	case strings.HasPrefix(apiKey, "nbo_live_"):
		return ModeLive
	default:
		return ""
	}
}

func defaultBaseURL(mode Mode) string {
	if mode == ModeLive {
		return defaultLiveBaseURL
	}
	return defaultSandboxBaseURL
}
