package prepare

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
)

func TestRun_PrepareParallelGroupRunsContainerDownloadsConcurrently(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bundle := t.TempDir()
	r := &concurrencyRunner{delegate: &fakeRunner{}}
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:           "prepare",
			MaxParallelism: 2,
			Steps: []config.Step{
				{ID: "ubuntu", Kind: "DownloadPackage", ParallelGroup: "distros", Spec: map[string]any{"packages": []any{"containerd"}, "distro": map[string]any{"family": "debian", "release": "ubuntu2204"}, "repo": map[string]any{"type": "deb-flat"}, "backend": map[string]any{"mode": "container", "runtime": "docker", "image": "ubuntu:22.04"}}},
				{ID: "rhel", Kind: "DownloadPackage", ParallelGroup: "distros", Spec: map[string]any{"packages": []any{"containerd"}, "distro": map[string]any{"family": "rhel", "release": "rhel9"}, "repo": map[string]any{"type": "rpm"}, "backend": map[string]any{"mode": "container", "runtime": "docker", "image": "rockylinux:9"}}},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: r}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if r.maxActive < 2 {
		t.Fatalf("expected concurrent container downloads, maxActive=%d", r.maxActive)
	}
}

func TestRun_ContainerBackendsWithFakeRunner(t *testing.T) {
	imageOps := stubDownloadImageOps()

	bundle := t.TempDir()
	r := &fakeRunner{}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{
				{
					ID:   "pkg",
					Kind: "DownloadPackage",
					Spec: map[string]any{
						"packages": []any{"containerd"},
						"backend": map[string]any{
							"mode":    "container",
							"runtime": "docker",
							"image":   "ubuntu:22.04",
						},
					},
				},
				{
					ID:   "img",
					Kind: "DownloadImage",
					Spec: map[string]any{
						"images": []any{"registry.k8s.io/kube-apiserver:v1.30.1"},
						"backend": map[string]any{
							"engine": "go-containerregistry",
						},
					},
				},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: r, imageDownloadOps: imageOps}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(bundle, "packages", "mock-package.deb")); err != nil {
		t.Fatalf("expected mock package artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "images", "registry.k8s.io_kube-apiserver_v1.30.1.tar")); err != nil {
		t.Fatalf("expected mock image artifact: %v", err)
	}
}

func TestRun_DownloadImageUsesOutputDir(t *testing.T) {
	bundle := t.TempDir()
	imageOps := stubDownloadImageOps()
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "img",
				Kind: "DownloadImage",
				Spec: map[string]any{
					"images":    []any{"registry.k8s.io/kube-apiserver:v1.30.1"},
					"outputDir": "images/control-plane",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, imageDownloadOps: imageOps}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "images", "control-plane", "registry.k8s.io_kube-apiserver_v1.30.1.tar")); err != nil {
		t.Fatalf("expected image artifact in custom dir: %v", err)
	}
}

func TestRun_DownloadImageRejectsNonCanonicalOutputDir(t *testing.T) {
	bundle := t.TempDir()
	imageOps := stubDownloadImageOps()
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "img",
				Kind: "DownloadImage",
				Spec: map[string]any{
					"images":    []any{"registry.k8s.io/kube-apiserver:v1.30.1"},
					"outputDir": "artifacts/images",
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, imageDownloadOps: imageOps})
	if err == nil || !strings.Contains(err.Error(), "DownloadImage outputDir must stay under images/") {
		t.Fatalf("expected canonical outputDir error, got %v", err)
	}
}

func stubDownloadImageOps() imageDownloadOps {
	return imageDownloadOps{
		parseReference: func(v string) (name.Reference, error) {
			return name.ParseReference(v, name.WeakValidation)
		},
		fetchImage: func(_ name.Reference, _ ...remote.Option) (v1.Image, error) {
			return empty.Image, nil
		},
		writeArchive: func(path string, _ name.Reference, _ v1.Image, _ ...tarball.WriteOption) error {
			return os.WriteFile(path, []byte("image"), 0o644)
		},
	}
}

func TestRun_PackagesContainerBackend(t *testing.T) {
	bundle := t.TempDir()
	r := &fakeRunner{}

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{
				{
					ID:   "k8s-pkgs",
					Kind: "DownloadPackage",
					Spec: map[string]any{
						"packages": []any{"kubelet", "kubeadm"},
						"backend": map[string]any{
							"mode":    "container",
							"runtime": "docker",
							"image":   "ubuntu:22.04",
						},
					},
				},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: r}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(bundle, "packages", "mock-package.deb")); err != nil {
		t.Fatalf("expected mock package artifact: %v", err)
	}
}

func TestRun_PackagesContainerRuntimeMissing(t *testing.T) {
	bundle := t.TempDir()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{
				{
					ID:   "pkg",
					Kind: "DownloadPackage",
					Spec: map[string]any{
						"packages": []any{"containerd"},
						"backend": map[string]any{
							"mode":    "container",
							"runtime": "auto",
							"image":   "ubuntu:22.04",
						},
					},
				},
			},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &noRuntimeRunner{}})
	if err == nil {
		t.Fatalf("expected runtime detection error")
	}
	if !strings.Contains(err.Error(), "E_PREPARE_RUNTIME_NOT_FOUND") {
		t.Fatalf("expected runtime error code, got: %v", err)
	}
}

func TestRun_PackagesContainerNoArtifact(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bundle := t.TempDir()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{
				{
					ID:   "pkg",
					Kind: "DownloadPackage",
					Spec: map[string]any{
						"packages": []any{"containerd"},
						"backend": map[string]any{
							"mode":    "container",
							"runtime": "docker",
							"image":   "ubuntu:22.04",
						},
					},
				},
			},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &noArtifactRunner{}})
	if err == nil {
		t.Fatalf("expected no artifacts error")
	}
	if !strings.Contains(err.Error(), "E_PREPARE_NO_ARTIFACTS") {
		t.Fatalf("expected no-artifacts error code, got: %v", err)
	}
}

func TestRun_DownloadPackageUsesOutputDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bundle := t.TempDir()
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "pkg",
				Kind: "DownloadPackage",
				Spec: map[string]any{
					"packages":  []any{"containerd"},
					"outputDir": "packages/custom",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "packages", "custom", "containerd.txt")); err != nil {
		t.Fatalf("expected package artifact in custom dir: %v", err)
	}
}

func TestRun_DownloadPackageRejectsNonCanonicalOutputDir(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bundle := t.TempDir()
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "pkg",
				Kind: "DownloadPackage",
				Spec: map[string]any{
					"packages":  []any{"containerd"},
					"outputDir": "artifacts/packages",
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle})
	if err == nil || !strings.Contains(err.Error(), "DownloadPackage outputDir must stay under packages/") {
		t.Fatalf("expected canonical outputDir error, got %v", err)
	}
}

func TestRun_ExposesTypedRuntimeErrorCode(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	bundle := t.TempDir()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "pkg",
				Kind: "DownloadPackage",
				Spec: map[string]any{
					"packages": []any{"containerd"},
					"backend": map[string]any{
						"mode":    "container",
						"runtime": "auto",
						"image":   "ubuntu:22.04",
					},
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &noRuntimeRunner{}})
	if !errcode.Is(err, errCodePrepareRuntimeMissing) {
		t.Fatalf("expected typed code %s, got %v", errCodePrepareRuntimeMissing, err)
	}
}

func TestRun_DownloadPackageReusesExportedArtifactCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bundle := t.TempDir()
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "pkg-cache",
				Kind: "DownloadPackage",
				Spec: map[string]any{
					"packages": []any{"containerd"},
					"backend":  map[string]any{"mode": "container", "runtime": "docker", "image": "ubuntu:22.04"},
				},
			}},
		}},
	}
	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &fakeRunner{}}); err != nil {
		t.Fatalf("initial run failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(bundle, "packages")); err != nil {
		t.Fatalf("remove bundle packages: %v", err)
	}
	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &noArtifactRunner{}}); err != nil {
		t.Fatalf("expected exported cache reuse, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "packages", "mock-package.deb")); err != nil {
		t.Fatalf("expected package restored from exported cache: %v", err)
	}
	cacheRoot := filepath.Join(home, ".cache", "deck", "artifacts", "package")
	if _, err := os.Stat(cacheRoot); err != nil {
		t.Fatalf("expected exported package cache root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".cache", "deck", "packages")); !os.IsNotExist(err) {
		t.Fatalf("expected old package-manager cache path to stay unused, got %v", err)
	}
}

func TestRun_DownloadPackageCorruptedBundleArtifactFallsBackToExportedCache(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bundle := t.TempDir()
	step := config.Step{
		ID:   "pkg-cache",
		Kind: "DownloadPackage",
		Spec: map[string]any{
			"packages": []any{"containerd"},
			"backend":  map[string]any{"mode": "container", "runtime": "docker", "image": "ubuntu:22.04"},
		},
	}
	wf := &config.Workflow{Version: "v1", Phases: []config.Phase{{Name: "prepare", Steps: []config.Step{step}}}}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &fakeRunner{}}); err != nil {
		t.Fatalf("initial run failed: %v", err)
	}
	bundlePkg := filepath.Join(bundle, "packages", "mock-package.deb")
	if err := os.WriteFile(bundlePkg, []byte("corrupt"), 0o644); err != nil {
		t.Fatalf("corrupt bundle package: %v", err)
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &noArtifactRunner{}}); err != nil {
		t.Fatalf("expected exported cache fallback after corruption, got %v", err)
	}
	raw, err := os.ReadFile(bundlePkg)
	if err != nil {
		t.Fatalf("read restored package: %v", err)
	}
	if string(raw) != "pkg" {
		t.Fatalf("expected restored package payload, got %q", string(raw))
	}
}

func TestRun_DownloadPackageCorruptedExportedCacheTriggersFetch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bundle := t.TempDir()
	step := config.Step{
		ID:   "pkg-cache",
		Kind: "DownloadPackage",
		Spec: map[string]any{
			"packages": []any{"containerd"},
			"backend":  map[string]any{"mode": "container", "runtime": "docker", "image": "ubuntu:22.04"},
		},
	}
	wf := &config.Workflow{Version: "v1", Phases: []config.Phase{{Name: "prepare", Steps: []config.Step{step}}}}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &fakeRunner{}}); err != nil {
		t.Fatalf("initial run failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(bundle, "packages")); err != nil {
		t.Fatalf("remove bundle packages: %v", err)
	}
	cachePath, err := exportedPackageCachePath(computeStepCacheKey(step), nil)
	if err != nil {
		t.Fatalf("resolve exported cache path: %v", err)
	}
	cachePkg := filepath.Join(exportedPackageCachePayloadPath(cachePath), "mock-package.deb")
	if err := os.WriteFile(cachePkg, []byte("corrupt"), 0o644); err != nil {
		t.Fatalf("corrupt exported cache package: %v", err)
	}

	err = Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &noArtifactRunner{}})
	if !errcode.Is(err, errCodePrepareArtifactEmpty) {
		t.Fatalf("expected fresh fetch after exported cache corruption, got %v", err)
	}
}

func TestRun_DownloadImageCorruptedArchiveTriggersRefetch(t *testing.T) {
	bundle := t.TempDir()
	fetches := 0
	imageOps := imageDownloadOps{
		parseReference: func(v string) (name.Reference, error) {
			return name.ParseReference(v, name.WeakValidation)
		},
		fetchImage: func(_ name.Reference, _ ...remote.Option) (v1.Image, error) {
			fetches++
			return empty.Image, nil
		},
		writeArchive: func(path string, _ name.Reference, _ v1.Image, _ ...tarball.WriteOption) error {
			return os.WriteFile(path, []byte(fmt.Sprintf("image-%d", fetches)), 0o644)
		},
	}
	wf := &config.Workflow{Version: "v1", Phases: []config.Phase{{Name: "prepare", Steps: []config.Step{{
		ID:   "img",
		Kind: "DownloadImage",
		Spec: map[string]any{"images": []any{"registry.k8s.io/kube-apiserver:v1.30.1"}},
	}}}}}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, imageDownloadOps: imageOps}); err != nil {
		t.Fatalf("initial run failed: %v", err)
	}
	archive := filepath.Join(bundle, "images", "registry.k8s.io_kube-apiserver_v1.30.1.tar")
	if err := os.WriteFile(archive, []byte("corrupt"), 0o644); err != nil {
		t.Fatalf("corrupt image archive: %v", err)
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, imageDownloadOps: imageOps}); err != nil {
		t.Fatalf("expected refetch after corruption, got %v", err)
	}
	if fetches != 2 {
		t.Fatalf("expected image to be fetched twice, got %d", fetches)
	}
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatalf("read image archive: %v", err)
	}
	if string(raw) != "image-2" {
		t.Fatalf("expected refetched image payload, got %q", string(raw))
	}
}

func TestRun_DownloadPackageExportFailureLeavesNoBundleOutput(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bundle := t.TempDir()
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "pkg-export-fail",
				Kind: "DownloadPackage",
				Spec: map[string]any{
					"packages": []any{"containerd"},
					"backend":  map[string]any{"mode": "container", "runtime": "docker", "image": "ubuntu:22.04"},
				},
			}},
		}},
	}
	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &fakeRunner{failExport: true}})
	if err == nil {
		t.Fatalf("expected export failure")
	}
	if _, statErr := os.Stat(filepath.Join(bundle, "packages")); !os.IsNotExist(statErr) {
		t.Fatalf("expected no published bundle output after export failure, got %v", statErr)
	}
	cacheRoot := filepath.Join(home, ".cache", "deck", "artifacts", "package")
	entries, readErr := os.ReadDir(cacheRoot)
	if readErr == nil && len(entries) > 0 {
		t.Fatalf("expected no published cache entries after export failure")
	}
}
