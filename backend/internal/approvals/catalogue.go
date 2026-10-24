package approvals

import (
	"fmt"
	"slices"
)

// Catalogue is the set of approvable actions, in registration order. It is
// filled once at start-up by the wiring and read-only afterwards.
type Catalogue struct {
	order []string
	byKey map[string]Action
}

// NewCatalogue builds an empty catalogue.
func NewCatalogue() *Catalogue { return &Catalogue{byKey: map[string]Action{}} }

// Register adds an action; a duplicate key or a missing executor is a wiring bug.
func (c *Catalogue) Register(a Action) {
	if a.Key == "" || a.Execute == nil {
		panic(fmt.Sprintf("approvals: action %q needs a key and an executor", a.Key))
	}
	if _, dup := c.byKey[a.Key]; dup {
		panic(fmt.Sprintf("approvals: action %q registered twice", a.Key))
	}
	if a.Condition != "" && a.Condition != ConditionAmount {
		panic(fmt.Sprintf("approvals: action %q has unknown condition field %q", a.Key, a.Condition))
	}
	c.order = append(c.order, a.Key)
	c.byKey[a.Key] = a
}

// Get returns one action.
func (c *Catalogue) Get(key string) (Action, bool) {
	a, ok := c.byKey[key]
	return a, ok
}

// List returns every action in registration order.
func (c *Catalogue) List() []Action {
	out := make([]Action, 0, len(c.order))
	for _, k := range c.order {
		out = append(out, c.byKey[k])
	}
	return out
}

// Keys returns the registered keys.
func (c *Catalogue) Keys() []string { return slices.Clone(c.order) }
