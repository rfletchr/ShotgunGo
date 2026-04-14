package shotgun

// QueryOption configures a Query when passed to Client.Find.
type QueryOption interface {
	applyTo(*queryConfig)
}

// queryConfig accumulates options before a Query is built.
type queryConfig struct {
	fields    []string
	condition Condition
	pageSize  int
}

// fieldsOption sets the fields to return.
type fieldsOption struct{ fields []string }

func (f fieldsOption) applyTo(c *queryConfig) { c.fields = f.fields }

// Fields returns a QueryOption that specifies which entity fields to return.
func Fields(fields ...string) QueryOption {
	return fieldsOption{fields: fields}
}

// PageSize returns a QueryOption that overrides the default page size (500).
func PageSize(n int) QueryOption {
	return pageSizeOption(n)
}

type pageSizeOption int

func (p pageSizeOption) applyTo(c *queryConfig) { c.pageSize = int(p) }

// Condition is a filter expression. It also implements QueryOption so it can
// be passed directly to Client.Find.
type Condition interface {
	QueryOption
	marshalCondition() any
}

// leafCondition is a single filter triple: [field, relation, value].
type leafCondition struct {
	field    string
	relation string
	value    any
}

func (c leafCondition) applyTo(cfg *queryConfig)  { cfg.condition = c }
func (c leafCondition) marshalCondition() any {
	return []any{c.field, c.relation, c.value}
}

// logicalCondition groups conditions with "and" or "or".
type logicalCondition struct {
	operator   string
	conditions []Condition
}

func (c logicalCondition) applyTo(cfg *queryConfig) { cfg.condition = c }
func (c logicalCondition) marshalCondition() any {
	conditions := make([]any, len(c.conditions))
	for i, cond := range c.conditions {
		conditions[i] = cond.marshalCondition()
	}
	return map[string]any{
		"logical_operator": c.operator,
		"conditions":       conditions,
	}
}

// Filter returns a leaf Condition: [field, relation, value].
func Filter(field, relation string, value any) Condition {
	return leafCondition{field: field, relation: relation, value: value}
}

// And returns a Condition that requires all sub-conditions to be true.
func And(conditions ...Condition) Condition {
	return logicalCondition{operator: "and", conditions: conditions}
}

// Or returns a Condition that requires at least one sub-condition to be true.
func Or(conditions ...Condition) Condition {
	return logicalCondition{operator: "or", conditions: conditions}
}

// marshalFilters converts a Condition into the filters body value and
// the required content-type for the _search request.
// A nil condition produces an empty array (array format).
// Any condition is normalised to the hash format.
func marshalFilters(c Condition) (any, string) {
	if c == nil {
		return []any{}, "application/vnd+shotgun.api3_array+json"
	}
	// Hash format requires a logical operator at the top level.
	if _, ok := c.(logicalCondition); !ok {
		c = And(c)
	}
	return c.marshalCondition(), "application/vnd+shotgun.api3_hash+json"
}
