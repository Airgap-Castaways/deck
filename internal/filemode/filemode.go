package filemode

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	PrivateDirMode   = 0o700
	PrivateFileMode  = 0o600
	ArtifactDirMode  = 0o755
	ArtifactFileMode = 0o644
)

func EnsurePrivateDir(path string) error {
	if err := os.MkdirAll(path, PrivateDirMode); err != nil {
		return fmt.Errorf("create private directory: %w", err)
	}
	return nil
}

func EnsureParentPrivateDir(path string) error {
	return EnsurePrivateDir(filepath.Dir(path))
}

func WritePrivateFile(path string, data []byte) error {
	if err := EnsureParentPrivateDir(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, PrivateFileMode); err != nil {
		return fmt.Errorf("write private file: %w", err)
	}
	return nil
}

func EnsureArtifactDir(path string) error {
	if err := os.MkdirAll(path, ArtifactDirMode); err != nil {
		return fmt.Errorf("create artifact directory: %w", err)
	}
	return nil
}

func EnsureParentArtifactDir(path string) error {
	return EnsureArtifactDir(filepath.Dir(path))
}

func WriteArtifactFile(path string, data []byte) error {
	if err := EnsureParentArtifactDir(path); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, ArtifactFileMode); err != nil {
		return fmt.Errorf("write artifact file: %w", err)
	}
	return nil
}
