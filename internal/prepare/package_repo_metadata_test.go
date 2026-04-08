package prepare

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestRun_PackagesKubernetesSetRepoModeDebFlatGeneratesMetadata(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bundle := t.TempDir()
	r := &fakeRunner{}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "pkgs",
				Kind: "DownloadPackage",
				Spec: map[string]any{
					"packages": []any{"containerd"},
					"distro": map[string]any{
						"family":  "debian",
						"release": "ubuntu2204",
					},
					"repo": map[string]any{
						"type": "deb-flat",
					},
					"backend": map[string]any{
						"mode":    "container",
						"runtime": "docker",
						"image":   "ubuntu:22.04",
					},
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: r}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(bundle, "packages", "deb", "ubuntu2204", "pkgs", "mock-package.deb")); err != nil {
		t.Fatalf("expected mock deb artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "packages", "deb", "ubuntu2204", "Packages.gz")); err != nil {
		t.Fatalf("expected Packages.gz: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "packages", "deb", "ubuntu2204", "Release")); err != nil {
		t.Fatalf("expected Release: %v", err)
	}
}

func TestRun_PackagesKubernetesSetRepoModeRpmGeneratesRepodata(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bundle := t.TempDir()
	r := &fakeRunner{}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "pkgs",
				Kind: "DownloadPackage",
				Spec: map[string]any{
					"packages": []any{"containerd"},
					"distro": map[string]any{
						"family":  "rhel",
						"release": "rhel9",
					},
					"repo": map[string]any{
						"type": "rpm",
					},
					"backend": map[string]any{
						"mode":    "container",
						"runtime": "docker",
						"image":   "rockylinux:9",
					},
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: r}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(bundle, "packages", "rpm", "rhel9", "repodata", "repomd.xml")); err != nil {
		t.Fatalf("expected repodata/repomd.xml: %v", err)
	}
}
