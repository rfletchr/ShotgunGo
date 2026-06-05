package shotgun

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Query describes a search against a Shotgun entity type.
// All fields are invariant after construction — use Client.Find to create one.
type Query struct {
	client     *Client
	entityType string
	fields     []string
	condition  Condition
	pageSize   int
	order      []OrderField
}

// pageResponse mirrors the top-level JSON envelope for a _search response.
type pageResponse struct {
	Data  []json.RawMessage `json:"data"`
	Links pageLinks         `json:"links"`
}

// searchURL builds the _search path with page and sort query parameters.
func (q *Query) searchURL(pageNumber, pageSize int) string {
	params := url.Values{}
	params.Set("page[number]", strconv.Itoa(pageNumber))
	params.Set("page[size]", strconv.Itoa(pageSize))
	if len(q.order) > 0 {
		parts := make([]string, len(q.order))
		for i, o := range q.order {
			if o.Direction == Desc {
				parts[i] = "-" + o.Field
			} else {
				parts[i] = o.Field
			}
		}
		params.Set("sort", strings.Join(parts, ","))
	}
	return fmt.Sprintf("/api/v1.1/entity/%s/_search?%s", q.entityType, params.Encode())
}

// fetchPage issues the _search POST to rawURL (relative or absolute) and
// returns the decoded Page.
func (q *Query) fetchPage(ctx context.Context, rawURL string) (*Page, error) {
	filters, contentType := marshalFilters(q.condition)
	body := map[string]any{
		"fields":  q.fields,
		"filters": filters,
	}

	resp, err := q.client.post(ctx, rawURL, contentType, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s search returned status %d", q.entityType, resp.StatusCode)
	}

	var raw pageResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode %s response: %w", q.entityType, err)
	}

	entities := make([]Entity, 0, len(raw.Data))
	for _, d := range raw.Data {
		var e Entity
		if err := json.Unmarshal(d, &e); err != nil {
			return nil, fmt.Errorf("failed to decode entity: %w", err)
		}
		e.raw = d
		entities = append(entities, e)
	}

	return &Page{
		Entities: entities,
		links:    raw.Links,
		fetcher:  q,
	}, nil
}

// Page fetches a specific page of results (1-indexed).
func (q *Query) Page(ctx context.Context, number int) (*Page, error) {
	return q.fetchPage(ctx, q.searchURL(number, q.pageSize))
}

// One fetches the first matching entity, or returns nil if there are no results.
func (q *Query) One(ctx context.Context) (*Entity, error) {
	page, err := q.fetchPage(ctx, q.searchURL(1, 1))
	if err != nil {
		return nil, err
	}
	if len(page.Entities) == 0 {
		return nil, nil
	}
	return &page.Entities[0], nil
}

// All fetches every matching entity by walking pages sequentially.
func (q *Query) All(ctx context.Context) ([]Entity, error) {
	var results []Entity
	for e, err := range q.Iter(ctx) {
		if err != nil {
			return nil, err
		}
		results = append(results, e)
	}
	return results, nil
}

// Iter returns a range-over-function iterator that yields entities one at a
// time, fetching subsequent pages on demand.
//
// Usage:
//
//	for entity, err := range q.Iter(ctx) {
//	    if err != nil { ... }
//	    // use entity
//	}
func (q *Query) Iter(ctx context.Context) iter.Seq2[Entity, error] {
	return func(yield func(Entity, error) bool) {
		page, err := q.Page(ctx, 1)
		if err != nil {
			yield(Entity{}, err)
			return
		}
		for {
			for _, e := range page.Entities {
				if !yield(e, nil) {
					return
				}
			}
			if !page.HasNext() || len(page.Entities) == 0 {
				return
			}
			page, err = page.Next(ctx)
			if err != nil {
				yield(Entity{}, err)
				return
			}
		}
	}
}
