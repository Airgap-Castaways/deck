package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ResolveUnder(root string, segments ...string) (string, error) {
	resolvedRoot, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return "", fmt.Errorf("resolve root path: %w", err)
	}
	parts := []string{resolvedRoot}
	parts = append(parts, segments...)
	target := filepath.Join(parts...)
	resolvedTarget, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolve target path: %w", err)
	}
	rootPrefix := resolvedRoot + string(os.PathSeparator)
	if resolvedTarget != resolvedRoot && !strings.HasPrefix(resolvedTarget, rootPrefix) {
		return "", fmt.Errorf("path escapes root: %s", resolvedTarget)
	}
	return resolvedTarget, nil
}

func ReadFileUnder(root string, segments ...string) ([]byte, string, error) {
	path, err := ResolveUnder(root, segments...)
	if err != nil {
		return nil, "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, path, err
	}
	return raw, path, nil
}

func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func Open(path string) (*os.File, error) {
	return os.Open(path)
}

func OpenFile(path string, flag int, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(path, flag, mode)
}

func Create(path string) (*os.File, error) {
	return os.Create(path)
}

func CreateWithMode(path string, mode os.FileMode) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
}

func StatUnder(root string, segments ...string) (os.FileInfo, string, error) {
	path, err := ResolveUnder(root, segments...)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, path, err
	}
	return info, path, nil
}

func ReadDirUnder(root string, segments ...string) ([]os.DirEntry, string, error) {
	path, err := ResolveUnder(root, segments...)
	if err != nil {
		return nil, "", err
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, path, err
	}
	return entries, path, nil
}
