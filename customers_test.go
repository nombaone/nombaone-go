package nombaone

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
)

const customerJSON = `{"domain":"customer","id":"nbo123456789012cus","email":"ada@example.com","name":"Ada Lovelace","phone":null,"metadata":{"crmId":"crm_812"},"mode":"sandbox","createdAt":"2026-07-04T10:00:00.000Z","updatedAt":"2026-07-04T10:00:00.000Z"}`

func decodeBody(t *testing.T, raw string) map[string]any {
	t.Helper()
	if raw == "" {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("decode body %q: %v", raw, err)
	}
	return m
}

// lastCall returns the single recorded call, failing if there was not exactly one.
func lastCall(t *testing.T, m *mockTransport) recordedCall {
	t.Helper()
	if len(m.calls) != 1 {
		t.Fatalf("expected exactly 1 call, got %d", len(m.calls))
	}
	return m.calls[0]
}

func wantMethodPath(t *testing.T, call recordedCall, method, path string) {
	t.Helper()
	if call.Method != method {
		t.Errorf("method = %s, want %s", call.Method, method)
	}
	if call.Path != path {
		t.Errorf("path = %s, want %s", call.Path, path)
	}
}

func TestCustomers_Create(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusCreated, body: okEnvelope(customerJSON)})
	c := testClient(t, m)

	cust, err := c.Customers.Create(context.Background(), CustomerCreateParams{
		Email:    "ada@example.com",
		Name:     "Ada Lovelace",
		Metadata: Metadata{"crmId": "crm_812"},
	})
	if err != nil {
		t.Fatal(err)
	}
	call := lastCall(t, m)
	wantMethodPath(t, call, http.MethodPost, "/v1/customers")

	body := decodeBody(t, call.Body)
	if body["email"] != "ada@example.com" || body["name"] != "Ada Lovelace" {
		t.Errorf("body = %v", body)
	}
	if _, ok := body["metadata"].(map[string]any); !ok {
		t.Errorf("metadata missing from body: %v", body)
	}
	if !uuidRE.MatchString(call.Header.Get("Idempotency-Key")) {
		t.Errorf("Create must send an auto Idempotency-Key")
	}
	if cust.ID != "nbo123456789012cus" || cust.Mode != ModeSandbox {
		t.Errorf("unmarshaled customer = %+v", cust)
	}
	if cust.Phone != nil {
		t.Errorf("Phone = %v, want nil for JSON null", cust.Phone)
	}
}

func TestCustomers_Retrieve(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(customerJSON)})
	c := testClient(t, m)

	_, err := c.Customers.Retrieve(context.Background(), "nbo123456789012cus")
	if err != nil {
		t.Fatal(err)
	}
	call := lastCall(t, m)
	wantMethodPath(t, call, http.MethodGet, "/v1/customers/nbo123456789012cus")
	if call.Header.Get("Idempotency-Key") != "" {
		t.Error("GET must not send an Idempotency-Key")
	}
}

func TestCustomers_UpdateClearsPhoneWithNull(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(customerJSON)})
	c := testClient(t, m)

	_, err := c.Customers.Update(context.Background(), "nbo123456789012cus", CustomerUpdateParams{
		Phone: Null[string](),
	})
	if err != nil {
		t.Fatal(err)
	}
	call := lastCall(t, m)
	wantMethodPath(t, call, http.MethodPatch, "/v1/customers/nbo123456789012cus")
	if call.Body != `{"phone":null}` {
		t.Errorf("body = %s, want {\"phone\":null}", call.Body)
	}
}

func TestCustomers_UpdateSetsPhoneAndName(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(customerJSON)})
	c := testClient(t, m)

	_, err := c.Customers.Update(context.Background(), "nbo1", CustomerUpdateParams{
		Name:  String("Ada L."),
		Phone: Set("+2348012345678"),
	})
	if err != nil {
		t.Fatal(err)
	}
	body := decodeBody(t, lastCall(t, m).Body)
	if body["name"] != "Ada L." || body["phone"] != "+2348012345678" {
		t.Errorf("body = %v", body)
	}
}

func TestCustomers_ListWithFilters(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: listEnvelope("[]", false, "")})
	c := testClient(t, m)

	_, err := c.Customers.List(context.Background(), CustomerListParams{
		Email: String("ada@example.com"),
		Limit: Int(50),
	})
	if err != nil {
		t.Fatal(err)
	}
	call := lastCall(t, m)
	wantMethodPath(t, call, http.MethodGet, "/v1/customers")
	q, _ := url.ParseQuery(call.Query)
	if q.Get("email") != "ada@example.com" || q.Get("limit") != "50" {
		t.Errorf("query = %v", q)
	}
}

func TestCustomers_ApplyAndRemoveDiscount(t *testing.T) {
	discountJSON := `{"domain":"discount","id":"nbo1dsc","couponId":"nbo1cpn","customerId":"nbo1cus","subscriptionId":null,"status":"active","cyclesRemaining":null,"startAt":"2026-07-04T10:00:00.000Z","endAt":null,"mode":"sandbox","createdAt":"2026-07-04T10:00:00.000Z"}`

	m := newMock(scriptedResponse{status: http.StatusCreated, body: okEnvelope(discountJSON)})
	c := testClient(t, m)
	d, err := c.Customers.ApplyDiscount(context.Background(), "nbo1cus", CustomerApplyDiscountParams{Coupon: "LAUNCH20"})
	if err != nil {
		t.Fatal(err)
	}
	call := lastCall(t, m)
	wantMethodPath(t, call, http.MethodPost, "/v1/customers/nbo1cus/discount")
	if decodeBody(t, call.Body)["coupon"] != "LAUNCH20" {
		t.Errorf("body = %s", call.Body)
	}
	if d.Status != DiscountStatusActive {
		t.Errorf("status = %q", d.Status)
	}

	m2 := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(discountJSON)})
	c2 := testClient(t, m2)
	if _, err := c2.Customers.RemoveDiscount(context.Background(), "nbo1cus"); err != nil {
		t.Fatal(err)
	}
	wantMethodPath(t, lastCall(t, m2), http.MethodDelete, "/v1/customers/nbo1cus/discount")
}

func TestCustomers_GrantCredit(t *testing.T) {
	grantJSON := `{"domain":"credit_grant","id":"nbo1crg","customerId":"nbo1cus","amountInKobo":250000,"remainingInKobo":250000,"source":"goodwill","sourceReference":null,"mode":"sandbox","voidedAt":null,"createdAt":"2026-07-04T10:00:00.000Z"}`
	m := newMock(scriptedResponse{status: http.StatusCreated, body: okEnvelope(grantJSON)})
	c := testClient(t, m)

	grant, err := c.Customers.GrantCredit(context.Background(), "nbo1cus", CustomerGrantCreditParams{
		AmountInKobo: 250_000,
		Source:       CreditSourceGoodwill,
	})
	if err != nil {
		t.Fatal(err)
	}
	call := lastCall(t, m)
	wantMethodPath(t, call, http.MethodPost, "/v1/customers/nbo1cus/credit")
	body := decodeBody(t, call.Body)
	if body["amountInKobo"] != float64(250000) || body["source"] != "goodwill" {
		t.Errorf("body = %v", body)
	}
	if !uuidRE.MatchString(call.Header.Get("Idempotency-Key")) {
		t.Error("GrantCredit must send an auto Idempotency-Key")
	}
	if grant.AmountInKobo != 250_000 || grant.Source != CreditSourceGoodwill {
		t.Errorf("grant = %+v", grant)
	}
}

func TestCustomers_RetrieveCreditBalanceAndVoid(t *testing.T) {
	balanceJSON := `{"domain":"credit_balance","customerId":"nbo1cus","balanceInKobo":250000,"grants":[]}`
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(balanceJSON)})
	c := testClient(t, m)
	bal, err := c.Customers.RetrieveCreditBalance(context.Background(), "nbo1cus")
	if err != nil {
		t.Fatal(err)
	}
	wantMethodPath(t, lastCall(t, m), http.MethodGet, "/v1/customers/nbo1cus/credit")
	if bal.BalanceInKobo != 250_000 {
		t.Errorf("balance = %d", bal.BalanceInKobo)
	}

	grantJSON := `{"domain":"credit_grant","id":"nbo1crg","customerId":"nbo1cus","amountInKobo":250000,"remainingInKobo":0,"source":"goodwill","sourceReference":null,"mode":"sandbox","voidedAt":"2026-07-05T00:00:00.000Z","createdAt":"2026-07-04T10:00:00.000Z"}`
	m2 := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(grantJSON)})
	c2 := testClient(t, m2)
	if _, err := c2.Customers.VoidCredit(context.Background(), "nbo1cus", "nbo1crg"); err != nil {
		t.Fatal(err)
	}
	wantMethodPath(t, lastCall(t, m2), http.MethodDelete, "/v1/customers/nbo1cus/credit/nbo1crg")
}

func TestCustomers_PathSegmentsAreEncoded(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: okEnvelope(customerJSON)})
	c := testClient(t, m)
	// A slash in an id must not break out of the path segment.
	_, _ = c.Customers.Retrieve(context.Background(), "nbo/evil id")
	if got := lastCall(t, m).Path; got != "/v1/customers/nbo%2Fevil%20id" {
		t.Errorf("path = %q, want the id percent-encoded", got)
	}
}
