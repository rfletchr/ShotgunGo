package shotgun

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const defaultPageSize = 500

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// Client manages authentication and HTTP communication with the Shotgun REST API.
type Client struct {
	baseURL      string
	scriptName   string
	scriptKey    string
	accessToken  string
	refreshToken string
	expiresAt    time.Time
	http         *http.Client
	mu           sync.Mutex
}

// NewClient creates a Client using script-based credentials.
// baseURL should be the root of the Shotgun site, e.g. "https://studio.shotgunstudio.com".
func NewClient(baseURL, scriptName, scriptKey string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		scriptName: scriptName,
		scriptKey:  scriptKey,
		http:       &http.Client{},
	}
}

func (c *Client) requestToken(ctx context.Context, body url.Values) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1.1/auth/access_token",
		strings.NewReader(body.Encode()),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth failed with status %d", resp.StatusCode)
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return fmt.Errorf("failed to decode auth response: %w", err)
	}

	c.accessToken = token.AccessToken
	c.refreshToken = token.RefreshToken
	c.expiresAt = time.Now().Add(time.Duration(token.ExpiresIn) * time.Second)
	return nil
}

func (c *Client) authenticate(ctx context.Context) error {
	body := url.Values{}
	body.Set("client_id", c.scriptName)
	body.Set("client_secret", c.scriptKey)
	body.Set("grant_type", "client_credentials")
	return c.requestToken(ctx, body)
}

func (c *Client) refresh(ctx context.Context) error {
	body := url.Values{}
	body.Set("refresh_token", c.refreshToken)
	body.Set("grant_type", "refresh_token")
	if err := c.requestToken(ctx, body); err != nil {
		// Refresh token expired; fall back to full re-authentication.
		return c.authenticate(ctx)
	}
	return nil
}

// ensureAuthenticated guarantees a valid access token before each request.
// It is safe for concurrent use.
func (c *Client) ensureAuthenticated(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.accessToken == "" {
		return c.authenticate(ctx)
	}
	if time.Now().After(c.expiresAt.Add(-30 * time.Second)) {
		return c.refresh(ctx)
	}
	return nil
}

// post issues an authenticated POST to rawURL. Relative paths are resolved
// against the client's baseURL.
func (c *Client) post(ctx context.Context, rawURL, contentType string, body any) (*http.Response, error) {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	resolvedURL := rawURL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		resolvedURL = c.baseURL + rawURL
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resolvedURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")

	return c.http.Do(req)
}

// get issues an authenticated GET to rawURL. Relative paths are resolved
// against the client's baseURL.
func (c *Client) get(ctx context.Context, rawURL string) (*http.Response, error) {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	resolvedURL := rawURL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		resolvedURL = c.baseURL + rawURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, resolvedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Accept", "application/json")

	return c.http.Do(req)
}

// put issues an authenticated PUT to rawURL with a JSON body.
func (c *Client) put(ctx context.Context, rawURL string, body any) (*http.Response, error) {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	resolvedURL := rawURL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		resolvedURL = c.baseURL + rawURL
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, resolvedURL, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return c.http.Do(req)
}

// delete issues an authenticated DELETE to rawURL with no body.
func (c *Client) delete(ctx context.Context, rawURL string) (*http.Response, error) {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	resolvedURL := rawURL
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		resolvedURL = c.baseURL + rawURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, resolvedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Accept", "application/json")

	return c.http.Do(req)
}

// Find builds an immutable Query for entityType.
// Pass Fields(...) and a Condition to describe the search.
func (c *Client) Find(entityType string, options ...QueryOption) *Query {
	cfg := &queryConfig{pageSize: defaultPageSize}
	for _, opt := range options {
		opt.applyTo(cfg)
	}
	return &Query{
		client:     c,
		entityType: entityType,
		fields:     cfg.fields,
		condition:  cfg.condition,
		pageSize:   cfg.pageSize,
	}
}
