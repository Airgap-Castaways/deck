package validate

import (
	"fmt"
	"strings"

	"github.com/taedi90/deck/internal/config"
)

// File validates that the given path exists and is parseable YAML.
func File(path string) error {
	if path == "" {
		return fmt.Errorf("file path is empty")
	}

	wf, err := config.Load(path)
	if err != nil {
		return err
	}

	if wf.Version == "" {
		return fmt.Errorf("version is required")
	}
	if strings.TrimSpace(wf.Version) != "v1" {
		return fmt.Errorf("unsupported version: %s", wf.Version)
	}

	return nil
}
