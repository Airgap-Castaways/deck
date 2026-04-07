package prepare

import (
	"fmt"
	"path/filepath"
	"strings"
)

func computeArtifactChecksums(base string, relFiles []string) (map[string]string, error) {
	checksums := make(map[string]string, len(relFiles))
	for _, rel := range normalizeStrings(relFiles) {
		abs := filepath.Join(base, filepath.FromSlash(rel))
		sum, err := fileSHA256(abs)
		if err != nil {
			return nil, fmt.Errorf("compute artifact checksum for %s: %w", abs, err)
		}
		checksums[rel] = sum
	}
	return checksums, nil
}

func normalizeChecksumMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for key, value := range in {
		trimmedKey := filepath.ToSlash(strings.TrimSpace(key))
		trimmedValue := strings.ToLower(strings.TrimSpace(value))
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		out[trimmedKey] = trimmedValue
	}
	return out
}

func verifyArtifactChecksums(base string, relFiles []string, checksums map[string]string) (bool, error) {
	normalizedChecksums := normalizeChecksumMap(checksums)
	for _, rel := range normalizeStrings(relFiles) {
		want, ok := normalizedChecksums[rel]
		if !ok {
			return false, nil
		}
		abs := filepath.Join(base, filepath.FromSlash(rel))
		got, err := fileSHA256(abs)
		if err != nil {
			return false, err
		}
		if !strings.EqualFold(got, want) {
			return false, nil
		}
	}
	return true, nil
}
