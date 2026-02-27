package collections

import (
	"sort"
	"strings"
)

// ResolveContext returns the concatenated context for a given path.
// Algorithm: for every prefix key in PathContext, if path has that prefix,
// collect it. Sort matched prefixes shortest-to-longest.
// Concatenate their values with "\n\n".
// Returns empty string if no matches.
func (c *Collection) ResolveContext(path string) string {
	if c == nil || len(c.PathContext) == 0 {
		return ""
	}

	type match struct {
		prefix string
		value  string
	}

	var matches []match
	for prefix, value := range c.PathContext {
		if strings.HasPrefix(path, prefix) {
			matches = append(matches, match{prefix: prefix, value: value})
		}
	}

	if len(matches) == 0 {
		return ""
	}

	// Sort by prefix length (shortest first)
	sort.Slice(matches, func(i, j int) bool {
		return len(matches[i].prefix) < len(matches[j].prefix)
	})

	values := make([]string, len(matches))
	for i, m := range matches {
		values[i] = m.value
	}
	return strings.Join(values, "\n\n")
}
