package install

import (
	"fmt"
	"os"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
)

// resolveFallbackStateReadPath migrates existing state into the selected
// default target path. It never deletes or reads from old paths after a
// successful migration.
func resolveFallbackStateReadPath(wf *config.Workflow, preferredPath string) (string, bool, error) {
	if wf == nil || strings.TrimSpace(wf.StateKey) == "" {
		return strings.TrimSpace(preferredPath), false, nil
	}
	for _, resolver := range []func(*config.Workflow) (string, error){XDGStatePath, LegacyStatePath} {
		fallbackPath, err := resolver(wf)
		if err != nil {
			return "", false, err
		}
		if fallbackPath == strings.TrimSpace(preferredPath) {
			continue
		}
		if _, err := os.Stat(fallbackPath); err == nil {
			if err := migrateStateFile(fallbackPath, strings.TrimSpace(preferredPath)); err != nil {
				return "", false, err
			}
			return fallbackPath, true, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", false, fmt.Errorf("stat fallback state file: %w", err)
		}
	}
	return strings.TrimSpace(preferredPath), false, nil
}

func migrateStateFile(sourcePath string, targetPath string) error {
	source := strings.TrimSpace(sourcePath)
	target := strings.TrimSpace(targetPath)
	if source == "" || target == "" || source == target {
		return nil
	}
	if _, err := os.Stat(target); err == nil {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("stat target state file: %w", err)
	}
	raw, err := fsutil.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read fallback state file %s: %w", source, err)
	}
	if err := SaveRawStateFile(target, raw); err != nil {
		return fmt.Errorf("migrate state file from %s to %s: %w", source, target, err)
	}
	return nil
}
