package nombaone

import (
	"context"
	"errors"
	"iter"
	"net/url"
)

// Page is one page of a list response, plus everything needed to keep going.
// Every List method returns a *Page. Read this page's items from Data, walk
// pages by hand with HasNextPage and NextPage, or iterate every item across
// every page with All — cursors are threaded for you and the original filters
// are preserved.
//
//	page, err := client.Invoices.List(ctx, nombaone.InvoiceListParams{Status: nombaone.String("open")})
//	// this page only:
//	for _, inv := range page.Data { /* … */ }
//	// or every invoice across every page:
//	for inv, err := range page.All(ctx) {
//		if err != nil { /* handle */ break }
//		// …
//	}
type Page[T any] struct {
	// Data holds the items on this page.
	Data []T
	// Pagination is the cursor block: Limit, HasMore, NextCursor.
	Pagination Pagination
	// RequestID is the request id for this page's fetch.
	RequestID string

	client *Client
	spec   requestSpec
}

func newPage[T any](c *Client, spec requestSpec, res *result[[]T]) *Page[T] {
	pagination := Pagination{Limit: len(res.Data)}
	if res.Pagination != nil {
		pagination = *res.Pagination
	}
	return &Page[T]{
		Data:       res.Data,
		Pagination: pagination,
		RequestID:  res.RequestID,
		client:     c,
		spec:       spec,
	}
}

// executePage runs a list request and wraps the result as a *Page.
func executePage[T any](ctx context.Context, c *Client, spec requestSpec) (*Page[T], error) {
	res, err := execute[[]T](ctx, c, spec)
	if err != nil {
		return nil, err
	}
	return newPage[T](c, spec, res), nil
}

// HasNextPage reports whether another page exists after this one.
func (p *Page[T]) HasNextPage() bool {
	return p.Pagination.HasMore && p.Pagination.NextCursor != nil
}

// NextPage fetches the next page, threading NextCursor while preserving the
// original filters. It returns an error if there is no next page — guard with
// HasNextPage first.
func (p *Page[T]) NextPage(ctx context.Context) (*Page[T], error) {
	if !p.HasNextPage() {
		return nil, errors.New("nombaone: no next page — check HasNextPage() before calling NextPage()")
	}
	nextSpec := p.spec
	q := cloneQuery(p.spec.query)
	q.Set("cursor", *p.Pagination.NextCursor)
	nextSpec.query = q
	return executePage[T](ctx, p.client, nextSpec)
}

// All returns an iterator over every item across this page and all subsequent
// pages. Use it with range (Go 1.23+):
//
//	for customer, err := range client.Customers.List(ctx).All(ctx) {
//		if err != nil {
//			return err
//		}
//		// …
//	}
//
// The pair form surfaces a mid-iteration fetch error as the second value;
// stop iterating when it is non-nil.
func (p *Page[T]) All(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		page := p
		for {
			for _, item := range page.Data {
				if !yield(item, nil) {
					return
				}
			}
			if !page.HasNextPage() {
				return
			}
			next, err := page.NextPage(ctx)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			page = next
		}
	}
}

// cloneQuery returns an independent copy of q so threading a cursor never
// mutates a caller's or an earlier page's query.
func cloneQuery(q url.Values) url.Values {
	out := make(url.Values, len(q))
	for key, values := range q {
		out[key] = append([]string(nil), values...)
	}
	return out
}
