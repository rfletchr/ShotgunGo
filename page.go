package shotgun

import "context"

// pageLinks mirrors the "links" object returned at the top level of a
// paginated Shotgun response.
type pageLinks struct {
	Self string `json:"self"`
	Next string `json:"next"`
	Prev string `json:"prev"`
}

// pageFetcher is implemented by any query type that can re-fetch a page by URL.
// The rawURL is the next/prev link returned by the API.
type pageFetcher interface {
	fetchPage(ctx context.Context, rawURL string) (*Page, error)
}

// Page holds a single page of Entity results along with enough state to
// navigate to adjacent pages.
type Page struct {
	Entities []Entity
	links    pageLinks
	fetcher  pageFetcher
}

// HasNext reports whether there is a subsequent page of results.
func (p *Page) HasNext() bool {
	return p.links.Next != ""
}

// HasPrev reports whether there is a preceding page of results.
func (p *Page) HasPrev() bool {
	return p.links.Prev != ""
}

// Next fetches the next page. It returns nil, nil when there are no more pages.
func (p *Page) Next(ctx context.Context) (*Page, error) {
	if !p.HasNext() {
		return nil, nil
	}
	return p.fetcher.fetchPage(ctx, p.links.Next)
}

// Prev fetches the previous page. It returns nil, nil when already on the first page.
func (p *Page) Prev(ctx context.Context) (*Page, error) {
	if !p.HasPrev() {
		return nil, nil
	}
	return p.fetcher.fetchPage(ctx, p.links.Prev)
}
