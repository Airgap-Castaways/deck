package bundle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

	manifestPaths := make(map[string]struct{}, len(mf.Entries))
	for _, e := range mf.Entries {
		rel := filepath.ToSlash(filepath.Clean(filepath.FromSlash(e.Path)))
		manifestPaths[rel] = struct{}{}
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

	if err := verifyOfflineArtifactCoverage(bundleRoot, manifestPaths); err != nil {
		return err
	}

	return nil
}

func verifyOfflineArtifactCoverage(bundleRoot string, manifestPaths map[string]struct{}) error {
	if err := verifyAPTRepoCoverage(bundleRoot, manifestPaths, filepath.Join("packages", "apt")); err != nil {
		return err
	}
	if err := verifyAPTRepoCoverage(bundleRoot, manifestPaths, filepath.Join("packages", "apt-k8s")); err != nil {
		return err
	}
	if err := verifyYUMRepoCoverage(bundleRoot, manifestPaths, filepath.Join("packages", "yum")); err != nil {
		return err
	}
	if err := verifyYUMRepoCoverage(bundleRoot, manifestPaths, filepath.Join("packages", "yum-k8s")); err != nil {
		return err
	}

	return verifyImageTarCoverage(bundleRoot, manifestPaths)
}

func verifyAPTRepoCoverage(bundleRoot string, manifestPaths map[string]struct{}, repoRoot string) error {
	releases, err := listSubdirectories(filepath.Join(bundleRoot, repoRoot))
	if err != nil {
		return fmt.Errorf("E_BUNDLE_INTEGRITY: scan apt repos %s: %w", filepath.ToSlash(repoRoot), err)
	}

	for _, release := range releases {
		releaseRoot := filepath.Join(repoRoot, release)
		if err := requireInBundleAndManifest(bundleRoot, manifestPaths, filepath.Join(releaseRoot, "Release")); err != nil {
			return err
		}
		if err := requireInBundleAndManifest(bundleRoot, manifestPaths, filepath.Join(releaseRoot, "Packages.gz")); err != nil {
			return err
		}
	}

	return nil
}

func verifyYUMRepoCoverage(bundleRoot string, manifestPaths map[string]struct{}, repoRoot string) error {
	repos, err := listSubdirectories(filepath.Join(bundleRoot, repoRoot))
	if err != nil {
		return fmt.Errorf("E_BUNDLE_INTEGRITY: scan yum repos %s: %w", filepath.ToSlash(repoRoot), err)
	}

	for _, repo := range repos {
		repomdRel := filepath.Join(repoRoot, repo, "repodata", "repomd.xml")
		if err := requireInBundleAndManifest(bundleRoot, manifestPaths, repomdRel); err != nil {
			return err
		}
	}

	return nil
}

func verifyImageTarCoverage(bundleRoot string, manifestPaths map[string]struct{}) error {
	imagesDir := filepath.Join(bundleRoot, "images")
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("E_BUNDLE_INTEGRITY: scan images dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".tar" {
			continue
		}
		if err := requireInBundleAndManifest(bundleRoot, manifestPaths, filepath.Join("images", e.Name())); err != nil {
			return err
		}
	}

	return nil
}

func requireInBundleAndManifest(bundleRoot string, manifestPaths map[string]struct{}, relPath string) error {
	relSlash := filepath.ToSlash(relPath)
	abs := filepath.Join(bundleRoot, filepath.FromSlash(relSlash))
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("E_BUNDLE_INTEGRITY: required offline artifact missing from bundle: %s", relSlash)
		}
		return fmt.Errorf("E_BUNDLE_INTEGRITY: stat required offline artifact %s: %w", relSlash, err)
	}
	if _, ok := manifestPaths[relSlash]; !ok {
		return fmt.Errorf("E_BUNDLE_INTEGRITY: required offline artifact missing from manifest: %s", relSlash)
	}

	return nil
}

func listSubdirectories(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	return names, nil
}
