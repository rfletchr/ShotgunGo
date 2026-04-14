package shotgun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// BatchRequest is a sealed interface implemented by CreateRequest, UpdateRequest,
// and DeleteRequest. Pass any combination to Client.Batch.
type BatchRequest interface {
	marshalBatchRequest() map[string]any
}

// CreateRequest creates a new entity record within a batch.
type CreateRequest struct {
	EntityType string
	Data       map[string]any
}

func (r CreateRequest) marshalBatchRequest() map[string]any {
	return map[string]any{
		"request_type": "create",
		"entity":       r.EntityType,
		"data":         r.Data,
	}
}

// UpdateRequest updates an existing entity record within a batch.
type UpdateRequest struct {
	EntityType string
	ID         int
	Data       map[string]any
}

func (r UpdateRequest) marshalBatchRequest() map[string]any {
	return map[string]any{
		"request_type": "update",
		"entity":       r.EntityType,
		"record_id":    r.ID,
		"data":         r.Data,
	}
}

// DeleteRequest deletes an entity record within a batch.
type DeleteRequest struct {
	EntityType string
	ID         int
}

func (r DeleteRequest) marshalBatchRequest() map[string]any {
	return map[string]any{
		"request_type": "delete",
		"entity":       r.EntityType,
		"record_id":    r.ID,
	}
}

// Batch executes multiple create, update, and delete operations in a single
// request. Results are returned in the same order as the requests.
// Delete operations produce a zero-valued Entity in the result slice.
func (c *Client) Batch(ctx context.Context, requests ...BatchRequest) ([]Entity, error) {
	raw := make([]map[string]any, len(requests))
	for i, r := range requests {
		raw[i] = r.marshalBatchRequest()
	}

	body := map[string]any{"requests": raw}

	resp, err := c.post(ctx, "/api/v1.1/entity/_batch", "application/json", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("batch request failed with status %d", resp.StatusCode)
	}

	var envelope struct {
		Data []json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode batch response: %w", err)
	}

	results := make([]Entity, len(envelope.Data))
	for i, d := range envelope.Data {
		// Delete operations return null in the data array.
		if string(d) == "null" || len(d) == 0 {
			continue
		}
		var e Entity
		if err := json.Unmarshal(d, &e); err != nil {
			return nil, fmt.Errorf("failed to decode entity at index %d: %w", i, err)
		}
		e.raw = d
		results[i] = e
	}

	return results, nil
}
