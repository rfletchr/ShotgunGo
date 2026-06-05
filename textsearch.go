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

// TextSearchQuery describes a text search across one or more entity types.
// Build one with Client.TextSearch, then call Page, One, All, or Iter.
type TextSearchQuery struct {
	client        *Client
	text          string
	entityFilters map[string]json.RawMessage // raw array-format filter JSON per entity type
	pageSize      int
	order         []OrderField
}

// TextSearch creates a query that searches for text across entity types.
// Call FilterEntity or FilterEntityJSON to restrict results to specific entity
// types or add per-entity filters. Without any entity filters every entity
// type is searched.
func (c *Client) TextSearch(text string) *TextSearchQuery {
	return &TextSearchQuery{
		client:        c,
		text:          text,
		entityFilters: make(map[string]json.RawMessage),
		pageSize:      defaultPageSize,
	}
}

// FilterEntityJSON restricts the search to records of entityType that match
// the provided raw JSON filter array (same triplet format as sg_find filters).
// Can be called multiple times with different entity types.
func (q *TextSearchQuery) FilterEntityJSON(entityType string, raw json.RawMessage) *TextSearchQuery {
	q.entityFilters[entityType] = raw
	return q
}

// FilterEntity restricts the search to records of entityType that satisfy cond.
// The condition is serialised to array format. Pass nil to include the entity
// type with no additional filter.
func (q *TextSearchQuery) FilterEntity(entityType string, cond Condition) *TextSearchQuery {
	raw, _ := json.Marshal(conditionToTriples(cond))
	q.entityFilters[entityType] = raw
	return q
}

// conditionToTriples converts a Condition to the array-of-triplets format used
// by the text search API. OR conditions are not representable in array format;
// they fall back to a single-element array wrapping the hash representation.
func conditionToTriples(c Condition) []any {
	if c == nil {
		return []any{}
	}
	if lc, ok := c.(logicalCondition); ok && lc.operator == "and" {
		result := make([]any, len(lc.conditions))
		for i, sub := range lc.conditions {
			result[i] = sub.marshalCondition()
		}
		return result
	}
	return []any{c.marshalCondition()}
}

// WithPageSize overrides the default page size (500).
func (q *TextSearchQuery) WithPageSize(n int) *TextSearchQuery {
	q.pageSize = n
	return q
}

// WithOrder sorts results by the given fields.
func (q *TextSearchQuery) WithOrder(fields ...OrderField) *TextSearchQuery {
	q.order = fields
	return q
}

func (q *TextSearchQuery) buildBody(pageNumber, pageSize int) map[string]any {
	body := map[string]any{
		"text": q.text,
		"page": map[string]any{"size": pageSize, "number": pageNumber},
	}
	if len(q.entityFilters) > 0 {
		body["entity_types"] = q.entityFilters
	}
	if len(q.order) > 0 {
		parts := make([]string, len(q.order))
		for i, o := range q.order {
			if o.Direction == Desc {
				parts[i] = "-" + o.Field
			} else {
				parts[i] = o.Field
			}
		}
		body["sort"] = strings.Join(parts, ",")
	}
	return body
}

// fetchPage implements pageFetcher. It extracts the page number from the
// next/prev link URL and rebuilds the full POST body.
func (q *TextSearchQuery) fetchPage(ctx context.Context, rawURL string) (*Page, error) {
	pageNum := 1
	if u, err := url.Parse(rawURL); err == nil {
		if n, err := strconv.Atoi(u.Query().Get("page[number]")); err == nil && n > 0 {
			pageNum = n
		}
	}
	return q.doPost(ctx, rawURL, pageNum, q.pageSize)
}

func (q *TextSearchQuery) doPost(ctx context.Context, rawURL string, pageNumber, pageSize int) (*Page, error) {
	body := q.buildBody(pageNumber, pageSize)
	resp, err := q.client.post(ctx, rawURL, "application/vnd+shotgun.api3_array+json", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("text search returned status %d", resp.StatusCode)
	}

	var raw pageResponse
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode text search response: %w", err)
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

// Page fetches a specific page of text search results (1-indexed).
func (q *TextSearchQuery) Page(ctx context.Context, number int) (*Page, error) {
	return q.doPost(ctx, "/api/v1.1/entity/_text_search", number, q.pageSize)
}

// One returns the first matching entity, or nil if there are no results.
func (q *TextSearchQuery) One(ctx context.Context) (*Entity, error) {
	page, err := q.doPost(ctx, "/api/v1.1/entity/_text_search", 1, 1)
	if err != nil {
		return nil, err
	}
	if len(page.Entities) == 0 {
		return nil, nil
	}
	return &page.Entities[0], nil
}

// All fetches every matching entity by walking pages sequentially.
func (q *TextSearchQuery) All(ctx context.Context) ([]Entity, error) {
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
func (q *TextSearchQuery) Iter(ctx context.Context) iter.Seq2[Entity, error] {
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
