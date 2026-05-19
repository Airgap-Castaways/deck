package maputil

import "strings"

func SetDottedPath(root map[string]any, path string, value any) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 {
		return
	}
	current := root
	for _, part := range parts[:len(parts)-1] {
		if part == "" {
			return
		}
		next, _ := current[part].(map[string]any)
		if next == nil {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	if last := parts[len(parts)-1]; last != "" {
		current[last] = value
	}
}
