package shotgun

// OrderDirection specifies ascending or descending sort order.
type OrderDirection string

const (
	Asc  OrderDirection = "asc"
	Desc OrderDirection = "desc"
)

// OrderField specifies a field to sort by and its direction.
type OrderField struct {
	Field     string
	Direction OrderDirection
}

type orderOption []OrderField

func (o orderOption) applyTo(cfg *queryConfig) { cfg.order = []OrderField(o) }

// Order returns a QueryOption that sorts results by the given fields in sequence.
func Order(fields ...OrderField) QueryOption {
	return orderOption(fields)
}
