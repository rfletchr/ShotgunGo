package shotgun

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

// EntityType describes a ShotGrid entity type.
type EntityType struct {
	Name    string
	Label   string
	Visible bool
}

// SchemaField describes a single field on a ShotGrid entity.
type SchemaField struct {
	Name        string
	Label       string
	Description string
	DataType    string
	Editable    bool
	Mandatory   bool
	Visible     bool
	ValidValues []string // populated for list and status_list fields
	ValidTypes  []string // populated for entity and multi_entity fields
}

type schemaFieldRaw struct {
	Name        struct{ Value string `json:"value"` } `json:"name"`
	Description struct{ Value string `json:"value"` } `json:"description"`
	DataType    struct{ Value string `json:"value"` } `json:"data_type"`
	Editable    struct{ Value bool   `json:"value"` } `json:"editable"`
	Mandatory   struct{ Value bool   `json:"value"` } `json:"mandatory"`
	Visible     struct{ Value bool   `json:"value"` } `json:"visible"`
	Properties  map[string]struct {
		Value json.RawMessage `json:"value"`
	} `json:"properties"`
}

// toSnakeCase converts PascalCase entity type names to the snake_case form
// required by schema URL paths (e.g. "HumanUser" -> "human_user").
func toSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// EntityTypes returns all entity types visible in this ShotGrid instance.
// Pass an optional project ID to get the schema in the context of that project.
func (c *Client) EntityTypes(ctx context.Context, projectID ...int) (map[string]EntityType, error) {
	path := "/api/v1.1/schema"
	if len(projectID) > 0 {
		path += fmt.Sprintf("?project_id=%d", projectID[0])
	}
	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("schema request failed with status %d", resp.StatusCode)
	}

	var envelope struct {
		Data map[string]struct {
			Name    struct{ Value string `json:"value"` } `json:"name"`
			Visible struct{ Value bool   `json:"value"` } `json:"visible"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode entity types: %w", err)
	}

	result := make(map[string]EntityType, len(envelope.Data))
	for typeName, raw := range envelope.Data {
		result[typeName] = EntityType{
			Name:    typeName,
			Label:   raw.Name.Value,
			Visible: raw.Visible.Value,
		}
	}
	return result, nil
}

// Fields returns the field schema for the given entity type.
// entityType should be in PascalCase (e.g. "Shot", "HumanUser").
// Pass an optional project ID to get field configuration in the context of that project,
// which is required to get valid status values and other project-specific field properties.
func (c *Client) Fields(ctx context.Context, entityType string, projectID ...int) (map[string]SchemaField, error) {
	path := fmt.Sprintf("/api/v1.1/schema/%s/fields", toSnakeCase(entityType))
	if len(projectID) > 0 {
		path += fmt.Sprintf("?project_id=%d", projectID[0])
	}

	resp, err := c.get(ctx, path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("schema/%s/fields returned status %d", entityType, resp.StatusCode)
	}

	var envelope struct {
		Data map[string]schemaFieldRaw `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("failed to decode fields for %s: %w", entityType, err)
	}

	result := make(map[string]SchemaField, len(envelope.Data))
	for fieldName, raw := range envelope.Data {
		field := SchemaField{
			Name:        fieldName,
			Label:       raw.Name.Value,
			Description: raw.Description.Value,
			DataType:    raw.DataType.Value,
			Editable:    raw.Editable.Value,
			Mandatory:   raw.Mandatory.Value,
			Visible:     raw.Visible.Value,
		}
		if vv, ok := raw.Properties["valid_values"]; ok {
			var values []string
			if err := json.Unmarshal(vv.Value, &values); err == nil {
				field.ValidValues = values
			}
		}
		if vt, ok := raw.Properties["valid_types"]; ok {
			var types []string
			if err := json.Unmarshal(vt.Value, &types); err == nil {
				field.ValidTypes = types
			}
		}
		result[fieldName] = field
	}
	return result, nil
}
