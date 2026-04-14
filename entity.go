package shotgun

import "encoding/json"

// Entity represents a single Shotgun entity record.
// ID and Type are decoded eagerly for identification; all other fields are
// decoded on demand via Decode.
type Entity struct {
	ID   int    `json:"id"`
	Type string `json:"type"`
	raw  json.RawMessage
}

// NewEntityRef returns a relationship reference map suitable for use as a field
// value in Create, Update, and Batch requests.
//
// Note: entityType must be the singular PascalCase name used by the REST API
// for relationship objects (e.g. "Project", "Task", "HumanUser") — NOT the
// plural snake_case name used for endpoint paths (e.g. "projects", "tasks").
func NewEntityRef(entityType string, id int) map[string]any {
	return map[string]any{"type": entityType, "id": id}
}

// Decode unmarshals the full entity JSON into v using standard json struct tags.
// The target struct should mirror the entity envelope:
//
//	type Task struct {
//	    ID         int    `json:"id"`
//	    Attributes struct {
//	        Content string `json:"content"`
//	        Status  string `json:"sg_status_list"`
//	    } `json:"attributes"`
//	}
func (e Entity) Decode(v any) error {
	return json.Unmarshal(e.raw, v)
}
