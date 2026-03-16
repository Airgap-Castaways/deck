package filemode

import (
	"fmt"
	"os"
	"path/filepath"
)

type StorageClass int

const (
	PrivateState StorageClass = iota
	SharedRuntime
	PublishedArtifact
)

const (
	PrivateDirMode   = 0o700
	PrivateFileMode  = 0o600
	SharedDirMode    = 0o750
	SharedFileMode   = 0o640
	ArtifactDirMode  = 0o755
	ArtifactFileMode = 0o644
)

func DirMode(class StorageClass) os.FileMode {
	switch class {
	case PrivateState:
		return PrivateDirMode
	case SharedRuntime:
		return SharedDirMode
	case PublishedArtifact:
		return ArtifactDirMode
	default:
		return PrivateDirMode
	}
}

func FileMode(class StorageClass) os.FileMode {
	switch class {
	case PrivateState:
		return PrivateFileMode
	case SharedRuntime:
		return SharedFileMode
	case PublishedArtifact:
		return ArtifactFileMode
	default:
		return PrivateFileMode
	}
}

func EnsureDir(path string, class StorageClass) error {
	if err := os.MkdirAll(path, DirMode(class)); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}
	return nil
}

func EnsureParentDir(path string, class StorageClass) error {
	return EnsureDir(filepath.Dir(path), class)
}

func WriteFile(path string, data []byte, class StorageClass) error {
	if err := EnsureParentDir(path, class); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, FileMode(class)); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func OpenFile(path string, flag int, class StorageClass) (*os.File, error) {
	if err := EnsureParentDir(path, class); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, flag, FileMode(class))
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

func EnsurePrivateDir(path string) error {
	return EnsureDir(path, PrivateState)
}

func EnsureParentPrivateDir(path string) error {
	return EnsurePrivateDir(filepath.Dir(path))
}

func WritePrivateFile(path string, data []byte) error {
	return WriteFile(path, data, PrivateState)
}

func EnsureArtifactDir(path string) error {
	return EnsureDir(path, PublishedArtifact)
}

func EnsureParentArtifactDir(path string) error {
	return EnsureArtifactDir(filepath.Dir(path))
}

func WriteArtifactFile(path string, data []byte) error {
	return WriteFile(path, data, PublishedArtifact)
}
