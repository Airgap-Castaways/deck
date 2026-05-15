package stepspec

import (
	"fmt"
	"strings"
)

func NormalizePlatform(raw string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 && len(parts) != 3 {
		return "", fmt.Errorf("platform %q must use os/arch or os/arch/variant", raw)
	}
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
		if parts[i] == "" {
			return "", fmt.Errorf("platform %q must not contain empty components", raw)
		}
	}
	return strings.Join(parts, "/"), nil
}
