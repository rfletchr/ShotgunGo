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
