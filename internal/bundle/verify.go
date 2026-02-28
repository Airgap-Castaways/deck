package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ManifestFile struct {
	Entries []ManifestEntry `json:"entries"`
}

type ManifestEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func VerifyManifest(bundleRoot string) error {
	manifestPath := filepath.Join(bundleRoot, "manifest.json")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("E_MANIFEST_MISSING: %s", manifestPath)
		}
		return fmt.Errorf("read manifest: %w", err)
	}

	var mf ManifestFile
	if err := json.Unmarshal(raw, &mf); err != nil {
		return fmt.Errorf("parse manifest: %w", err)
	}
	if len(mf.Entries) == 0 {
		return fmt.Errorf("E_MANIFEST_EMPTY: %s", manifestPath)
	}

	for _, e := range mf.Entries {
		if strings.TrimSpace(e.Path) == "" {
			return fmt.Errorf("E_BUNDLE_INTEGRITY: empty path entry")
		}
		abs := filepath.Join(bundleRoot, filepath.FromSlash(e.Path))
		content, err := os.ReadFile(abs)
		if err != nil {
			return fmt.Errorf("E_BUNDLE_INTEGRITY: missing artifact %s: %w", e.Path, err)
		}
		fi, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("E_BUNDLE_INTEGRITY: stat artifact %s: %w", e.Path, err)
		}
		if e.Size > 0 && fi.Size() != e.Size {
			return fmt.Errorf("E_BUNDLE_INTEGRITY: size mismatch for %s", e.Path)
		}

		h := sha256.Sum256(content)
		actual := hex.EncodeToString(h[:])
		if !strings.EqualFold(actual, e.SHA256) {
			return fmt.Errorf("E_BUNDLE_INTEGRITY: sha256 mismatch for %s", e.Path)
		}
	}

	return nil
}
