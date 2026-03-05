package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_MergeImports(t *testing.T) {
	dir := t.TempDir()

	base := filepath.Join(dir, "base.yaml")
	root := filepath.Join(dir, "cluster.yaml")

	baseContent := []byte(`version: v1alpha1
vars:
  role: worker
  imageRepo: registry.local
context:
  bundleRoot: ./bundle
phases:
  - name: prepare
    steps:
      - id: from-base
        apiVersion: deck/v1alpha1
        kind: DownloadFile
        spec: {}
`)
	rootContent := []byte(`version: v1alpha1
imports:
  - ./base.yaml
vars:
  role: control-plane
context:
  stateFile: /var/lib/deck/state.json
phases:
  - name: prepare
    steps:
      - id: from-root
        apiVersion: deck/v1alpha1
        kind: DownloadImages
        spec: {}
`)

	if err := os.WriteFile(base, baseContent, 0o644); err != nil {
		t.Fatalf("write base: %v", err)
	}
	if err := os.WriteFile(root, rootContent, 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}

	wf, err := Load(root)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if got := wf.Vars["role"]; got != "control-plane" {
		t.Fatalf("vars precedence mismatch, got %v", got)
	}
	if got := wf.Vars["imageRepo"]; got != "registry.local" {
		t.Fatalf("missing imported var, got %v", got)
	}

	if wf.Context.BundleRoot != "./bundle" {
		t.Fatalf("bundleRoot not merged: %s", wf.Context.BundleRoot)
	}
	if wf.Context.StateFile != "/var/lib/deck/state.json" {
		t.Fatalf("stateFile not merged: %s", wf.Context.StateFile)
	}

	if len(wf.Phases) != 1 || wf.Phases[0].Name != "prepare" {
		t.Fatalf("unexpected phases: %#v", wf.Phases)
	}
	if len(wf.Phases[0].Steps) != 2 {
		t.Fatalf("expected 2 merged steps, got %d", len(wf.Phases[0].Steps))
	}
}

func TestLoad_ImportCycle(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.yaml")
	b := filepath.Join(dir, "b.yaml")

	if err := os.WriteFile(a, []byte("version: v1\nimports:\n  - ./b.yaml\n"), 0o644); err != nil {
		t.Fatalf("write a: %v", err)
	}
	if err := os.WriteFile(b, []byte("version: v1\nimports:\n  - ./a.yaml\n"), 0o644); err != nil {
		t.Fatalf("write b: %v", err)
	}

	_, err := Load(a)
	if err == nil {
		t.Fatalf("expected cycle error")
	}
	if !errors.Is(err, ErrImportCycle) {
		t.Fatalf("expected ErrImportCycle, got %v", err)
	}
}
