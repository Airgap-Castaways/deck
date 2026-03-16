package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Root struct {
	abs string
}

func NewRoot(path string) (Root, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return Root{}, fmt.Errorf("resolve root path: %w", err)
	}
	return Root{abs: abs}, nil
}

func (r Root) Abs() string {
	return r.abs
}

func (r Root) Resolve(segments ...string) (string, error) {
	return ResolveUnder(r.abs, segments...)
}

func (r Root) ReadFile(segments ...string) ([]byte, string, error) {
	return ReadFileUnder(r.abs, segments...)
}

func (r Root) Stat(segments ...string) (os.FileInfo, string, error) {
	return StatUnder(r.abs, segments...)
}

func (r Root) ReadDir(segments ...string) ([]os.DirEntry, string, error) {
	return ReadDirUnder(r.abs, segments...)
}

func (r Root) Open(segments ...string) (*os.File, string, error) {
	path, err := r.Resolve(segments...)
	if err != nil {
		return nil, "", err
	}
	f, err := Open(path)
	if err != nil {
		return nil, path, err
	}
	return f, path, nil
}

func (r Root) Create(segments ...string) (*os.File, string, error) {
	path, err := r.Resolve(segments...)
	if err != nil {
		return nil, "", err
	}
	f, err := Create(path)
	if err != nil {
		return nil, path, err
	}
	return f, path, nil
}

func (r Root) OpenFile(flag int, mode os.FileMode, segments ...string) (*os.File, string, error) {
	path, err := r.Resolve(segments...)
	if err != nil {
		return nil, "", err
	}
	f, err := OpenFile(path, flag, mode)
	if err != nil {
		return nil, path, err
	}
	return f, path, nil
}
