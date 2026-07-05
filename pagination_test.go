package nombaone

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
)

func TestPagination_SinglePageExposesDataAndCursor(t *testing.T) {
	m := newMock(scriptedResponse{
		status: http.StatusOK,
		body:   listEnvelope(`[{"id":"a"},{"id":"b"}]`, true, "cur1"),
	})
	c := testClient(t, m)

	page, err := executePage[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/customers"})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Data) != 2 || page.Data[0].ID != "a" {
		t.Errorf("Data = %+v", page.Data)
	}
	if !page.Pagination.HasMore {
		t.Error("HasMore = false, want true")
	}
	if page.Pagination.NextCursor == nil || *page.Pagination.NextCursor != "cur1" {
		t.Errorf("NextCursor = %v, want cur1", page.Pagination.NextCursor)
	}
	if !page.HasNextPage() {
		t.Error("HasNextPage() = false, want true")
	}
}

func TestPagination_NextPageThreadsCursorAndPreservesFilters(t *testing.T) {
	m := newMock(
		scriptedResponse{status: http.StatusOK, body: listEnvelope(`[{"id":"a"}]`, true, "cur1")},
		scriptedResponse{status: http.StatusOK, body: listEnvelope(`[{"id":"b"}]`, false, "")},
	)
	c := testClient(t, m)

	page, err := executePage[testData](context.Background(), c, requestSpec{
		method: http.MethodGet,
		path:   "/subscriptions",
		query:  url.Values{"status": {"active"}, "customerId": {"nbo1"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	next, err := page.NextPage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if next.Data[0].ID != "b" {
		t.Errorf("next page Data = %+v", next.Data)
	}

	// The second request must carry the cursor AND the original filters.
	q, _ := url.ParseQuery(m.calls[1].Query)
	if q.Get("cursor") != "cur1" {
		t.Errorf("cursor = %q, want cur1", q.Get("cursor"))
	}
	if q.Get("status") != "active" || q.Get("customerId") != "nbo1" {
		t.Errorf("filters not preserved on next page: %v", q)
	}
}

func TestPagination_AllIteratesEveryItemAcrossPages(t *testing.T) {
	m := newMock(
		scriptedResponse{status: http.StatusOK, body: listEnvelope(`[{"id":"a"}]`, true, "cur1")},
		scriptedResponse{status: http.StatusOK, body: listEnvelope(`[{"id":"b"}]`, true, "cur2")},
		scriptedResponse{status: http.StatusOK, body: listEnvelope(`[{"id":"c"}]`, false, "")},
	)
	c := testClient(t, m)

	page, err := executePage[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/customers"})
	if err != nil {
		t.Fatal(err)
	}

	var ids []string
	for item, err := range page.All(context.Background()) {
		if err != nil {
			t.Fatalf("iteration error: %v", err)
		}
		ids = append(ids, item.ID)
	}
	if len(ids) != 3 || ids[0] != "a" || ids[1] != "b" || ids[2] != "c" {
		t.Errorf("ids = %v, want [a b c]", ids)
	}
	if m.callCount() != 3 {
		t.Errorf("callCount = %d, want 3 pages fetched", m.callCount())
	}
}

func TestPagination_AllStopsEarlyWithoutFetchingMore(t *testing.T) {
	m := newMock(
		scriptedResponse{status: http.StatusOK, body: listEnvelope(`[{"id":"a"},{"id":"b"}]`, true, "cur1")},
		scriptedResponse{status: http.StatusOK, body: listEnvelope(`[{"id":"c"}]`, false, "")},
	)
	c := testClient(t, m)

	page, err := executePage[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/customers"})
	if err != nil {
		t.Fatal(err)
	}

	var got []string
	for item, err := range page.All(context.Background()) {
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, item.ID)
		if item.ID == "a" {
			break // stop before consuming the rest of page 1 or fetching page 2
		}
	}
	if len(got) != 1 || got[0] != "a" {
		t.Errorf("got = %v, want [a]", got)
	}
	if m.callCount() != 1 {
		t.Errorf("callCount = %d, want 1 (no extra fetch after early stop)", m.callCount())
	}
}

func TestPagination_AllSurfacesMidIterationError(t *testing.T) {
	noBackoff(t)
	m := newMock(
		scriptedResponse{status: http.StatusOK, body: listEnvelope(`[{"id":"a"}]`, true, "cur1")},
		scriptedResponse{status: http.StatusInternalServerError, body: errEnvelope("SYSTEM_INTERNAL_ERROR")},
	)
	c := testClient(t, m, WithMaxRetries(0))

	page, err := executePage[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/customers"})
	if err != nil {
		t.Fatal(err)
	}

	var gotErr error
	var count int
	for item, err := range page.All(context.Background()) {
		if err != nil {
			gotErr = err
			break
		}
		_ = item
		count++
	}
	if count != 1 {
		t.Errorf("yielded %d good items before the error, want 1", count)
	}
	var se *ServerError
	if !errors.As(gotErr, &se) {
		t.Fatalf("want a ServerError surfaced mid-iteration, got %T (%v)", gotErr, gotErr)
	}
}

func TestPagination_NextPageWithoutNextErrors(t *testing.T) {
	m := newMock(scriptedResponse{status: http.StatusOK, body: listEnvelope(`[{"id":"a"}]`, false, "")})
	c := testClient(t, m)

	page, err := executePage[testData](context.Background(), c, requestSpec{method: http.MethodGet, path: "/customers"})
	if err != nil {
		t.Fatal(err)
	}
	if page.HasNextPage() {
		t.Fatal("HasNextPage() = true, want false")
	}
	if _, err := page.NextPage(context.Background()); err == nil {
		t.Error("NextPage() on a last page should error")
	}
}
