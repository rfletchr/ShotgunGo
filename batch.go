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
// Use NewCreateRequest to construct one.
type CreateRequest struct {
	entityType string
	data       map[string]any
}

// NewCreateRequest returns a BatchRequest that creates a new entity record.
func NewCreateRequest(entityType string, data map[string]any) CreateRequest {
	return CreateRequest{entityType: entityType, data: data}
}

func (r CreateRequest) marshalBatchRequest() map[string]any {
	return map[string]any{
		"request_type": "create",
		"entity":       r.entityType,
		"data":         r.data,
	}
}

// UpdateRequest updates an existing entity record within a batch.
// Use NewUpdateRequest to construct one.
type UpdateRequest struct {
	entityType string
	id         int
	data       map[string]any
}

// NewUpdateRequest returns a BatchRequest that updates an existing entity record.
func NewUpdateRequest(entityType string, id int, data map[string]any) UpdateRequest {
	return UpdateRequest{entityType: entityType, id: id, data: data}
}

func (r UpdateRequest) marshalBatchRequest() map[string]any {
	return map[string]any{
		"request_type": "update",
		"entity":       r.entityType,
		"record_id":    r.id,
		"data":         r.data,
	}
}

// DeleteRequest deletes an entity record within a batch.
// Use NewDeleteRequest to construct one.
type DeleteRequest struct {
	entityType string
	id         int
}

// NewDeleteRequest returns a BatchRequest that deletes an entity record.
func NewDeleteRequest(entityType string, id int) DeleteRequest {
	return DeleteRequest{entityType: entityType, id: id}
}

func (r DeleteRequest) marshalBatchRequest() map[string]any {
	return map[string]any{
		"request_type": "delete",
		"entity":       r.entityType,
		"record_id":    r.id,
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
