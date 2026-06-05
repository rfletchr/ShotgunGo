package shotgun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// SummaryField names a field and the aggregation to apply.
// Valid types: record_count, count, sum, maximum, minimum, average,
// earliest, latest, percentage, status_percentage, status_list, checked, unchecked.
type SummaryField struct {
	Field string `json:"field"`
	Type  string `json:"type"`
}

// GroupingField controls how results are bucketed.
// Valid types depend on the field's data type (e.g. exact, day, week, month,
// firstletter, entitytype). Direction is "asc" or "desc".
type GroupingField struct {
	Field     string `json:"field"`
	Type      string `json:"type"`
	Direction string `json:"direction,omitempty"`
}

// SummaryGroup is one bucket returned when grouping is requested.
type SummaryGroup struct {
	GroupName  string         `json:"group_name"`
	GroupValue any            `json:"group_value"`
	Summaries  map[string]any `json:"summaries"`
}

// SummarizeResult is the response from a _summarize request.
// Summaries contains the totals across all matched records.
// Groups is populated only when grouping fields are specified.
type SummarizeResult struct {
	Summaries map[string]any `json:"summaries"`
	Groups    []SummaryGroup `json:"groups"`
}

// Summarize runs an aggregation query against entityType.
// condition filters which records are included (pass nil to include all).
// summaryFields must contain at least one entry.
// grouping may be nil or empty for an ungrouped total.
func (c *Client) Summarize(
	ctx context.Context,
	entityType string,
	condition Condition,
	summaryFields []SummaryField,
	grouping []GroupingField,
) (*SummarizeResult, error) {
	filters, contentType := marshalFilters(condition)

	body := map[string]any{
		"filters":        filters,
		"summary_fields": summaryFields,
	}
	if len(grouping) > 0 {
		body["grouping"] = grouping
	}

	path := fmt.Sprintf("/api/v1.1/entity/%s/_summarize", entityType)
	resp, err := c.post(ctx, path, contentType, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s summarize returned status %d", entityType, resp.StatusCode)
	}

	var envelope struct {
		Data SummarizeResult `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode summarize response: %w", err)
	}

	return &envelope.Data, nil
}
