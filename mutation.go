package shotgun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// entityResponse is the envelope returned by Create and Update.
type entityResponse struct {
	Data json.RawMessage `json:"data"`
}

func decodeEntityResponse(resp *http.Response) (Entity, error) {
	defer resp.Body.Close()
	var envelope entityResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return Entity{}, fmt.Errorf("failed to decode response: %w", err)
	}
	var e Entity
	if err := json.Unmarshal(envelope.Data, &e); err != nil {
		return Entity{}, fmt.Errorf("failed to decode entity: %w", err)
	}
	e.raw = envelope.Data
	return e, nil
}

// Create creates a new record of entityType. fields is a flat map of field
// names to values — the same names used in Find and Filter.
//
// Relationship fields are set by passing an object with id and type:
//
//	client.Create(ctx, "tasks", map[string]any{
//	    "content": "Animation",
//	    "project": map[string]any{"id": 123, "type": "Project"},
//	})
func (c *Client) Create(ctx context.Context, entityType string, fields map[string]any) (Entity, error) {
	path := fmt.Sprintf("/api/v1.1/entity/%s", entityType)

	resp, err := c.post(ctx, path, "application/json", fields)
	if err != nil {
		return Entity{}, err
	}
	if resp.StatusCode != http.StatusCreated {
		resp.Body.Close()
		return Entity{}, fmt.Errorf("create %s failed with status %d", entityType, resp.StatusCode)
	}
	return decodeEntityResponse(resp)
}

// Update updates an existing record. fields is a flat map containing only the
// fields to change — unspecified fields are left untouched.
func (c *Client) Update(ctx context.Context, entityType string, id int, fields map[string]any) (Entity, error) {
	path := fmt.Sprintf("/api/v1.1/entity/%s/%d", entityType, id)

	resp, err := c.put(ctx, path, fields)
	if err != nil {
		return Entity{}, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return Entity{}, fmt.Errorf("update %s/%d failed with status %d", entityType, id, resp.StatusCode)
	}
	return decodeEntityResponse(resp)
}

// Delete deletes a record by id. Returns nil on success.
func (c *Client) Delete(ctx context.Context, entityType string, id int) error {
	path := fmt.Sprintf("/api/v1.1/entity/%s/%d", entityType, id)

	resp, err := c.delete(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete %s/%d failed with status %d", entityType, id, resp.StatusCode)
	}
	return nil
}
