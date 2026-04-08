package prepare

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
)

func TestRun_CheckHostStep(t *testing.T) {
	t.Run("runtime host available without checkhost", func(t *testing.T) {
		bundle := t.TempDir()
		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "prepare",
				Steps: []config.Step{{
					ID:   "runtime-branch",
					Kind: "DownloadPackage",
					When: "runtime.host.os.family == \"debian\" && runtime.host.arch == \"arm64\"",
					Spec: map[string]any{
						"packages": []any{"containerd"},
						"backend": map[string]any{
							"mode":    "container",
							"runtime": "docker",
							"image":   "ubuntu:22.04",
						},
					},
				}},
			}},
		}

		checkRuntime := checksRuntime{
			readHostFile: func(path string) ([]byte, error) {
				switch path {
				case "/etc/os-release":
					return []byte("ID=ubuntu\nID_LIKE=debian\nVERSION=\"24.04 LTS\"\nVERSION_ID=\"24.04\"\n"), nil
				case "/proc/sys/kernel/osrelease":
					return []byte("6.8.0-test\n"), nil
				default:
					return os.ReadFile(path)
				}
			},
			currentGOOS:   func() string { return "linux" },
			currentGOARCH: func() string { return "arm64" },
		}

		if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &fakeRunner{}, checksRuntime: checkRuntime}); err != nil {
			t.Fatalf("expected runtime.host without CheckHost, got %v", err)
		}
	})

	t.Run("pass and register", func(t *testing.T) {
		bundle := t.TempDir()
		wf := &config.Workflow{
			Version: "v1",
			Vars:    map[string]any{"want": "ok"},
			Phases: []config.Phase{{
				Name: "prepare",
				Steps: []config.Step{
					{
						ID:       "host-check",
						Kind:     "CheckHost",
						Register: map[string]string{"hostPassed": "passed"},
						Spec: map[string]any{
							"checks":   []any{"os", "arch", "binaries"},
							"binaries": []any{"docker"},
						},
					},
					{
						ID:   "runtime-branch",
						Kind: "DownloadPackage",
						When: "runtime.hostPassed == true && vars.want == \"ok\" && runtime.host.os.family == \"debian\" && runtime.host.arch == \"arm64\"",
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

		checkRuntime := checksRuntime{
			readHostFile: func(path string) ([]byte, error) {
				switch path {
				case "/etc/os-release":
					return []byte("ID=ubuntu\nID_LIKE=debian\nVERSION=\"24.04 LTS\"\nVERSION_ID=\"24.04\"\n"), nil
				case "/proc/sys/kernel/osrelease":
					return []byte("6.8.0-test\n"), nil
				default:
					return os.ReadFile(path)
				}
			},
			currentGOOS:   func() string { return "linux" },
			currentGOARCH: func() string { return "arm64" },
		}

		if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &fakeRunner{}, checksRuntime: checkRuntime}); err != nil {
			t.Fatalf("expected checkhost pass, got %v", err)
		}
	})

	t.Run("failfast false aggregates errors", func(t *testing.T) {
		bundle := t.TempDir()
		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "prepare",
				Steps: []config.Step{{
					ID:   "host-check",
					Kind: "CheckHost",
					Spec: map[string]any{
						"checks":   []any{"os", "arch", "binaries", "swap", "kernelModules"},
						"binaries": []any{"missing-bin"},
						"failFast": false,
					},
				}},
			}},
		}

		checkRuntime := checksRuntime{
			readHostFile: func(path string) ([]byte, error) {
				switch path {
				case "/proc/swaps":
					return []byte("Filename\tType\tSize\tUsed\tPriority\n/dev/sda file 1 0 -2\n"), nil
				case "/proc/modules":
					return []byte("overlay 1 0 - Live 0x0\n"), nil
				default:
					return os.ReadFile(path)
				}
			},
			currentGOOS:   func() string { return "darwin" },
			currentGOARCH: func() string { return "386" },
		}

		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &noRuntimeRunner{}, checksRuntime: checkRuntime})
		if err == nil {
			t.Fatalf("expected checkhost failure")
		}
		if !errcode.Is(err, errCodePrepareCheckHostFailed) {
			t.Fatalf("expected typed code %s, got %v", errCodePrepareCheckHostFailed, err)
		}
		if !strings.Contains(err.Error(), "E_PREPARE_CHECKHOST_FAILED") {
			t.Fatalf("expected E_PREPARE_CHECKHOST_FAILED, got %v", err)
		}
		if !strings.Contains(err.Error(), "os:") || !strings.Contains(err.Error(), "arch:") || !strings.Contains(err.Error(), "binaries:") {
			t.Fatalf("expected aggregated failures, got %v", err)
		}
	})
}

func TestRun_ExposesTypedPrepareValidationCodes(t *testing.T) {
	t.Run("unsupported image engine", func(t *testing.T) {
		bundle := t.TempDir()
		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "prepare",
				Steps: []config.Step{{
					ID:   "img",
					Kind: "DownloadImage",
					Spec: map[string]any{"images": []any{"registry.k8s.io/pause:3.10.1"}, "backend": map[string]any{"engine": "other"}},
				}},
			}},
		}
		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle})
		if !errcode.Is(err, errCodePrepareEngineUnsupported) {
			t.Fatalf("expected typed code %s, got %v", errCodePrepareEngineUnsupported, err)
		}
	})

	t.Run("no package artifacts", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		bundle := t.TempDir()
		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "prepare",
				Steps: []config.Step{{
					ID:   "pkg",
					Kind: "DownloadPackage",
					Spec: map[string]any{"packages": []any{"containerd"}, "backend": map[string]any{"mode": "container", "runtime": "docker", "image": "ubuntu:22.04"}},
				}},
			}},
		}
		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: &noArtifactRunner{}})
		if !errcode.Is(err, errCodePrepareArtifactEmpty) {
			t.Fatalf("expected typed code %s, got %v", errCodePrepareArtifactEmpty, err)
		}
	})
}
