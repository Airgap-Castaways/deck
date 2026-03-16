package fsutil

import (
	"io/fs"
	"os"
	"path/filepath"
)

func (r Root) WalkDir(fn fs.WalkDirFunc, segments ...string) error {
	rootPath, err := r.Resolve(segments...)
	if err != nil {
		return err
	}
	return filepath.WalkDir(rootPath, fn)
}

func (r Root) WalkFiles(fn func(path string, d os.DirEntry) error, segments ...string) error {
	return r.WalkDir(func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return fn(path, d)
	}, segments...)
}
