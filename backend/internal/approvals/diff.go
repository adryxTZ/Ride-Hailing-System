package approvals

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// emptyValue is how a missing field reads in a diff.
const emptyValue = "—"

// Diff compares two values field by field through their JSON forms and lists
// what changed, with dotted paths ("terms.booking_fee"). Describers use it to
// show approvers exactly what the change does.
func Diff(before, after any) []Change {
	b, a := flatten(before), flatten(after)
	keys := map[string]bool{}
	for k := range b {
		keys[k] = true
	}
	for k := range a {
		keys[k] = true
	}
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)
	out := []Change{}
	for _, k := range sorted {
		if b[k] != a[k] {
			out = append(out, Change{Field: k, Before: orEmpty(b[k]), After: orEmpty(a[k])})
		}
	}
	return out
}

func orEmpty(s string) string {
	if s == "" {
		return emptyValue
	}
	return s
}

// flatten turns a value into path → text; lists of scalars read as "a, b".
func flatten(v any) map[string]string {
	out := map[string]string{}
	if v == nil {
		return out
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return out
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return out
	}
	walk("", generic, out)
	return out
}

func walk(path string, v any, out map[string]string) {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			walk(join(path, k), x, out)
		}
	case []any:
		if scalars(t) {
			parts := make([]string, len(t))
			for i, x := range t {
				parts[i] = fmt.Sprint(x)
			}
			out[path] = strings.Join(parts, ", ")
			return
		}
		for i, x := range t {
			walk(join(path, fmt.Sprint(i+1)), x, out)
		}
	case nil:
		// absent and null read the same
	default:
		out[path] = fmt.Sprint(t)
	}
}

func scalars(xs []any) bool {
	for _, x := range xs {
		switch x.(type) {
		case map[string]any, []any:
			return false
		}
	}
	return true
}

func join(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}
