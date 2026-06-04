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
	order     []OrderField
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

// FilterRelation is the comparison operator used in a Filter condition.
// Using the typed constants prevents typos that would silently send an
// invalid operator to the API.
type FilterRelation string

const (
	Is             FilterRelation = "is"
	IsNot          FilterRelation = "is_not"
	LessThan       FilterRelation = "less_than"
	GreaterThan    FilterRelation = "greater_than"
	Contains       FilterRelation = "contains"
	NotContains    FilterRelation = "not_contains"
	StartsWith     FilterRelation = "starts_with"
	EndsWith       FilterRelation = "ends_with"
	Between        FilterRelation = "between"
	NotBetween     FilterRelation = "not_between"
	InLast         FilterRelation = "in_last"
	NotInLast      FilterRelation = "not_in_last"
	InNext         FilterRelation = "in_next"
	NotInNext      FilterRelation = "not_in_next"
	InCalendarDay  FilterRelation = "in_calendar_day"
	InCalendarWeek FilterRelation = "in_calendar_week"
	InCalendarMonth FilterRelation = "in_calendar_month"
	InCalendarYear FilterRelation = "in_calendar_year"
	In             FilterRelation = "in"
	NotIn          FilterRelation = "not_in"
	TypeIs         FilterRelation = "type_is"
	TypeIsNot      FilterRelation = "type_is_not"
	InGroup        FilterRelation = "in_group"
	NotInGroup     FilterRelation = "not_in_group"
	AddressIs      FilterRelation = "address_is"
	NameContains   FilterRelation = "name_contains"
	NameNotContains FilterRelation = "name_not_contains"
	NameStartsWith FilterRelation = "name_starts_with"
	NameEndsWith   FilterRelation = "name_ends_with"
)

// leafCondition is a single filter triple: [field, relation, value].
type leafCondition struct {
	field    string
	relation FilterRelation
	value    any
}

func (c leafCondition) applyTo(cfg *queryConfig) { cfg.condition = c }
func (c leafCondition) marshalCondition() any {
	return []any{c.field, string(c.relation), c.value}
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
func Filter(field string, relation FilterRelation, value any) Condition {
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
