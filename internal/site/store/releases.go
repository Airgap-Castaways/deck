package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/taedi90/deck/internal/filemode"
)

func (s *Store) ImportRelease(release Release, importedBundlePath string) error {
	return s.importReleaseWithCopyDir(release, importedBundlePath, copyDir)
}

func (s *Store) importReleaseWithCopyDir(release Release, importedBundlePath string, copyDirFn func(string, string) error) error {
	if err := validateRecordID(release.ID, "release id"); err != nil {
		return err
	}
	if strings.TrimSpace(importedBundlePath) == "" {
		return fmt.Errorf("imported bundle path is empty")
	}
	stat, err := os.Stat(importedBundlePath)
	if err != nil {
		return fmt.Errorf("stat imported bundle path: %w", err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("imported bundle path must be a directory")
	}

	releaseDir, err := s.releaseDir(release.ID)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(releaseDir, "manifest.json")
	bundlePath := filepath.Join(releaseDir, "bundle")

	if _, err := os.Stat(manifestPath); err == nil {
		return alreadyExistsError("release %q already imported", release.ID)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check release manifest: %w", err)
	}
	if _, err := os.Stat(bundlePath); err == nil {
		return alreadyExistsError("release %q bundle already imported", release.ID)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check release bundle path: %w", err)
	}

	releasesDir, err := s.releasesDir()
	if err != nil {
		return err
	}
	if err := filemode.EnsureDir(releasesDir, filemode.PrivateState); err != nil {
		return fmt.Errorf("create releases directory: %w", err)
	}
	tmpDir, err := os.MkdirTemp(releasesDir, release.ID+".tmp-")
	if err != nil {
		return fmt.Errorf("create temporary release directory: %w", err)
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	tmpBundlePath := filepath.Join(tmpDir, "bundle")
	tmpManifestPath := filepath.Join(tmpDir, "manifest.json")
	if err := copyDirFn(importedBundlePath, tmpBundlePath); err != nil {
		return fmt.Errorf("copy release bundle: %w", err)
	}
	if err := writeAtomicJSON(tmpManifestPath, release); err != nil {
		return fmt.Errorf("write release manifest: %w", err)
	}
	if err := os.Rename(tmpDir, releaseDir); err != nil {
		return fmt.Errorf("finalize release import: %w", err)
	}
	cleanupTmp = false
	return nil
}

func (s *Store) GetRelease(releaseID string) (Release, bool, error) {
	if err := validateRecordID(releaseID, "release id"); err != nil {
		return Release{}, false, err
	}
	releaseDir, err := s.releaseDir(releaseID)
	if err != nil {
		return Release{}, false, err
	}
	return readJSON[Release](filepath.Join(releaseDir, "manifest.json"))
}

func (s *Store) ListReleases() ([]Release, error) {
	releasesDir, err := s.releasesDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Release{}, nil
		}
		return nil, fmt.Errorf("read releases directory: %w", err)
	}

	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)

	out := make([]Release, 0, len(ids))
	for _, id := range ids {
		release, found, err := s.GetRelease(id)
		if err != nil {
			return nil, err
		}
		if found {
			out = append(out, release)
		}
	}
	return out, nil
}
