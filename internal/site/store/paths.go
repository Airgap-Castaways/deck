package store

func (s *Store) mustResolve(segments ...string) string {
	path, err := s.root.Resolve(segments...)
	if err != nil {
		panic(err)
	}
	return path
}

func (s *Store) siteDir() string {
	return s.mustResolve(".deck", "site")
}

func (s *Store) releasesDir() string {
	return s.mustResolve(".deck", "site", "releases")
}

func (s *Store) releaseDir(releaseID string) string {
	return s.mustResolve(".deck", "site", "releases", releaseID)
}

func (s *Store) sessionDir(sessionID string) string {
	return s.mustResolve(".deck", "site", "sessions", sessionID)
}
