package pisyn

// Construct is the base tree node. Every pisyn type (App, Pipeline, Stage, Job) embeds it.
type Construct struct {
	id       string
	scope    *Construct
	children []*Construct
	node     any
}

func newConstruct(scope *Construct, id string, node any) Construct {
	c := Construct{id: id, scope: scope, node: node}
	if scope != nil {
		scope.children = append(scope.children, &c)
	}
	return c
}

// ID returns the construct's identifier.
func (c *Construct) ID() string { return c.id }

// Children returns the construct's child nodes.
func (c *Construct) Children() []*Construct { return c.children }

// Node returns the typed object this construct wraps.
func (c *Construct) Node() any { return c.node }

// childrenOfType returns typed children by filtering the tree.
func childrenOfType[T any](c *Construct) []*T {
	var result []*T
	for _, child := range c.children {
		if typed, ok := child.node.(*T); ok {
			result = append(result, typed)
		}
	}
	return result
}
