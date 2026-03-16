package fsutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type Root struct {
	abs string
}

type BundleRoot struct{ root Root }

type PreparedRoot struct{ root Root }

type StateRoot struct{ root Root }

type SiteRoot struct{ root Root }

func NewRoot(path string) (Root, error) {
	abs, err := filepath.Abs(strings.TrimSpace(path))
	if err != nil {
		return Root{}, fmt.Errorf("resolve root path: %w", err)
	}
	return Root{abs: abs}, nil
}

func NewBundleRoot(path string) (BundleRoot, error) {
	root, err := NewRoot(path)
	if err != nil {
		return BundleRoot{}, err
	}
	return BundleRoot{root: root}, nil
}

func NewPreparedRoot(path string) (PreparedRoot, error) {
	root, err := NewRoot(path)
	if err != nil {
		return PreparedRoot{}, err
	}
	return PreparedRoot{root: root}, nil
}

func NewStateRoot(path string) (StateRoot, error) {
	root, err := NewRoot(path)
	if err != nil {
		return StateRoot{}, err
	}
	return StateRoot{root: root}, nil
}

func NewSiteRoot(path string) (SiteRoot, error) {
	root, err := NewRoot(path)
	if err != nil {
		return SiteRoot{}, err
	}
	return SiteRoot{root: root}, nil
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

func (r BundleRoot) Abs() string { return r.root.Abs() }

func (r BundleRoot) Resolve(segments ...string) (string, error) { return r.root.Resolve(segments...) }

func (r BundleRoot) ReadFile(segments ...string) ([]byte, string, error) {
	return r.root.ReadFile(segments...)
}

func (r BundleRoot) Stat(segments ...string) (os.FileInfo, string, error) {
	return r.root.Stat(segments...)
}

func (r BundleRoot) ReadDir(segments ...string) ([]os.DirEntry, string, error) {
	return r.root.ReadDir(segments...)
}

func (r BundleRoot) Open(segments ...string) (*os.File, string, error) {
	return r.root.Open(segments...)
}

func (r BundleRoot) Create(segments ...string) (*os.File, string, error) {
	return r.root.Create(segments...)
}

func (r BundleRoot) OpenFile(flag int, mode os.FileMode, segments ...string) (*os.File, string, error) {
	return r.root.OpenFile(flag, mode, segments...)
}

func (r BundleRoot) WalkDir(fn fs.WalkDirFunc, segments ...string) error {
	return r.root.WalkDir(fn, segments...)
}

func (r BundleRoot) WalkFiles(fn func(path string, d os.DirEntry) error, segments ...string) error {
	return r.root.WalkFiles(fn, segments...)
}

func (r PreparedRoot) Abs() string { return r.root.Abs() }

func (r PreparedRoot) Resolve(segments ...string) (string, error) { return r.root.Resolve(segments...) }

func (r PreparedRoot) ReadFile(segments ...string) ([]byte, string, error) {
	return r.root.ReadFile(segments...)
}

func (r PreparedRoot) Stat(segments ...string) (os.FileInfo, string, error) {
	return r.root.Stat(segments...)
}

func (r PreparedRoot) ReadDir(segments ...string) ([]os.DirEntry, string, error) {
	return r.root.ReadDir(segments...)
}

func (r PreparedRoot) Open(segments ...string) (*os.File, string, error) {
	return r.root.Open(segments...)
}

func (r PreparedRoot) Create(segments ...string) (*os.File, string, error) {
	return r.root.Create(segments...)
}

func (r PreparedRoot) OpenFile(flag int, mode os.FileMode, segments ...string) (*os.File, string, error) {
	return r.root.OpenFile(flag, mode, segments...)
}

func (r PreparedRoot) WalkDir(fn fs.WalkDirFunc, segments ...string) error {
	return r.root.WalkDir(fn, segments...)
}

func (r PreparedRoot) WalkFiles(fn func(path string, d os.DirEntry) error, segments ...string) error {
	return r.root.WalkFiles(fn, segments...)
}

func (r StateRoot) Abs() string { return r.root.Abs() }

func (r StateRoot) Resolve(segments ...string) (string, error) { return r.root.Resolve(segments...) }

func (r StateRoot) ReadFile(segments ...string) ([]byte, string, error) {
	return r.root.ReadFile(segments...)
}

func (r StateRoot) Stat(segments ...string) (os.FileInfo, string, error) {
	return r.root.Stat(segments...)
}

func (r StateRoot) ReadDir(segments ...string) ([]os.DirEntry, string, error) {
	return r.root.ReadDir(segments...)
}

func (r StateRoot) Open(segments ...string) (*os.File, string, error) {
	return r.root.Open(segments...)
}

func (r StateRoot) Create(segments ...string) (*os.File, string, error) {
	return r.root.Create(segments...)
}

func (r StateRoot) OpenFile(flag int, mode os.FileMode, segments ...string) (*os.File, string, error) {
	return r.root.OpenFile(flag, mode, segments...)
}

func (r StateRoot) WalkDir(fn fs.WalkDirFunc, segments ...string) error {
	return r.root.WalkDir(fn, segments...)
}

func (r StateRoot) WalkFiles(fn func(path string, d os.DirEntry) error, segments ...string) error {
	return r.root.WalkFiles(fn, segments...)
}

func (r SiteRoot) Abs() string { return r.root.Abs() }

func (r SiteRoot) Resolve(segments ...string) (string, error) { return r.root.Resolve(segments...) }

func (r SiteRoot) ReadFile(segments ...string) ([]byte, string, error) {
	return r.root.ReadFile(segments...)
}

func (r SiteRoot) Stat(segments ...string) (os.FileInfo, string, error) {
	return r.root.Stat(segments...)
}

func (r SiteRoot) ReadDir(segments ...string) ([]os.DirEntry, string, error) {
	return r.root.ReadDir(segments...)
}

func (r SiteRoot) Open(segments ...string) (*os.File, string, error) { return r.root.Open(segments...) }

func (r SiteRoot) Create(segments ...string) (*os.File, string, error) {
	return r.root.Create(segments...)
}

func (r SiteRoot) OpenFile(flag int, mode os.FileMode, segments ...string) (*os.File, string, error) {
	return r.root.OpenFile(flag, mode, segments...)
}

func (r SiteRoot) WalkDir(fn fs.WalkDirFunc, segments ...string) error {
	return r.root.WalkDir(fn, segments...)
}

func (r SiteRoot) WalkFiles(fn func(path string, d os.DirEntry) error, segments ...string) error {
	return r.root.WalkFiles(fn, segments...)
}
