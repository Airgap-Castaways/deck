package deckignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

type Matcher struct {
	compiled *ignore.GitIgnore
}

func Load(root string) (Matcher, error) {
	path := filepath.Join(root, ".deckignore")
	compiled, err := ignore.CompileIgnoreFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Matcher{}, nil
		}
		return Matcher{}, fmt.Errorf("read .deckignore: %w", err)
	}
	return Matcher{compiled: compiled}, nil
}

func (m Matcher) Matches(rel string, isDir bool) bool {
	if m.compiled == nil {
		return false
	}
	rel = filepath.ToSlash(strings.TrimPrefix(strings.TrimSpace(rel), "./"))
	if rel == "" || rel == "." {
		return false
	}
	if isDir && m.compiled.MatchesPath(rel+"/") {
		return true
	}
	return m.compiled.MatchesPath(rel)
}
