package store

import "fmt"

func (s *Store) resolve(segments ...string) (string, error) {
	path, err := s.root.Resolve(segments...)
	if err != nil {
		return "", fmt.Errorf("resolve site path: %w", err)
	}
	return path, nil
}

func (s *Store) siteDir() (string, error) {
	return s.resolve(".deck", "site")
}

func (s *Store) releasesDir() (string, error) {
	return s.resolve(".deck", "site", "releases")
}

func (s *Store) releaseDir(releaseID string) (string, error) {
	return s.resolve(".deck", "site", "releases", releaseID)
}

func (s *Store) sessionDir(sessionID string) (string, error) {
	return s.resolve(".deck", "site", "sessions", sessionID)
}
