package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestRun_WriteContainerdConfigDefaultGenerationRespectsParentContext(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	statePath := filepath.Join(dir, "state", "state.json")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	artifact := filepath.Join(bundle, "files", "a.txt")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("ok")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fakeWriteContainerdConfig := filepath.Join(binDir, "containerd")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"${1:-}\" == \"config\" && \"${2:-}\" == \"default\" ]]; then\n  sleep 1\n  echo 'version = 2'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(fakeWriteContainerdConfig, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake containerd: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	target := filepath.Join(dir, "containerd", "config.toml")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "install",
			Steps: []config.Step{{ID: "containerd-config", Kind: "WriteContainerdConfig", Spec: map[string]any{"path": target, "timeout": "5s"}}},
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := Run(ctx, wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected canceled containerd config generation")
	}
	if !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
		t.Fatalf("expected deadline exceeded in error, got %v", err)
	}
}

func TestRun_WriteContainerdConfigDefaultGenerationTimeoutUsesTimeoutClassification(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	statePath := filepath.Join(dir, "state", "state.json")
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		t.Fatalf("mkdir bundle: %v", err)
	}
	artifact := filepath.Join(bundle, "files", "a.txt")
	if err := os.MkdirAll(filepath.Dir(artifact), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.WriteFile(artifact, []byte("ok"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	if err := writeManifestForTest(bundle, "files/a.txt", []byte("ok")); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	fakeWriteContainerdConfig := filepath.Join(binDir, "containerd")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nif [[ \"${1:-}\" == \"config\" && \"${2:-}\" == \"default\" ]]; then\n  sleep 1\n  echo 'version = 2'\n  exit 0\nfi\nexit 1\n"
	if err := os.WriteFile(fakeWriteContainerdConfig, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake containerd: %v", err)
	}
	t.Setenv("PATH", fmt.Sprintf("%s:%s", binDir, os.Getenv("PATH")))

	target := filepath.Join(dir, "containerd", "config.toml")
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name:  "install",
			Steps: []config.Step{{ID: "containerd-config", Kind: "WriteContainerdConfig", Spec: map[string]any{"path": target, "timeout": "20ms"}}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, StatePath: statePath})
	if err == nil {
		t.Fatalf("expected containerd config timeout")
	}
	if !strings.Contains(err.Error(), "containerd config default generation timed out") {
		t.Fatalf("expected timeout classification, got %v", err)
	}
}

func TestWriteContainerdConfigStep(t *testing.T) {
	t.Run("updates existing config.toml fields", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "config.toml")
		initial := "version = 2\n[plugins.\"io.containerd.grpc.v1.cri\".registry]\n  config_path = \"\"\n[plugins.\"io.containerd.grpc.v1.cri\".containerd.runtimes.runc.options]\n  SystemdCgroup = false\n"
		if err := os.WriteFile(target, []byte(initial), 0o644); err != nil {
			t.Fatalf("write initial config: %v", err)
		}

		spec := map[string]any{"path": target, "rawSettings": []any{
			map[string]any{"op": "set", "key": "registry.configPath", "value": "/etc/containerd/certs.d"},
			map[string]any{"op": "set", "key": "runtime.runtimes.runc.options.SystemdCgroup", "value": true},
		}}
		if err := runWriteContainerdConfig(context.Background(), spec); err != nil {
			t.Fatalf("runWriteContainerdConfig failed: %v", err)
		}

		raw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config: %v", err)
		}
		got := string(raw)
		if !strings.Contains(got, "/etc/containerd/certs.d") || !strings.Contains(got, "SystemdCgroup = true") {
			t.Fatalf("unexpected config content: %q", got)
		}
	})

	t.Run("writes single registry hosts file under explicit configPath", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "config.toml")
		configPath := filepath.Join(dir, "certs.d")
		if err := os.WriteFile(target, []byte("version = 2\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		spec := map[string]any{
			"path": configPath,
			"registryHosts": []any{
				map[string]any{
					"registry":     "registry.k8s.io",
					"server":       "https://registry.k8s.io",
					"host":         "http://127.0.0.1:5000",
					"capabilities": []any{"pull", "resolve"},
					"skipVerify":   true,
				},
			},
		}

		if err := runWriteContainerdRegistryHosts(spec); err != nil {
			t.Fatalf("runWriteContainerdRegistryHosts failed: %v", err)
		}

		hostsPath := filepath.Join(configPath, "registry.k8s.io", "hosts.toml")
		hostsRaw, err := os.ReadFile(hostsPath)
		if err != nil {
			t.Fatalf("read hosts.toml: %v", err)
		}
		expected := "server = \"https://registry.k8s.io\"\n\n[host.\"http://127.0.0.1:5000\"]\n  capabilities = [\"pull\", \"resolve\"]\n  skip_verify = true\n"
		if string(hostsRaw) != expected {
			t.Fatalf("unexpected hosts.toml content: %q", string(hostsRaw))
		}

		if err := runWriteContainerdRegistryHosts(spec); err != nil {
			t.Fatalf("runWriteContainerdRegistryHosts second pass failed: %v", err)
		}
		hostsRawAgain, err := os.ReadFile(hostsPath)
		if err != nil {
			t.Fatalf("read hosts.toml second pass: %v", err)
		}
		if string(hostsRawAgain) != expected {
			t.Fatalf("hosts.toml changed unexpectedly on second pass: %q", string(hostsRawAgain))
		}
	})

	t.Run("writes multiple registry hosts files with default configPath", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "containerd", "config.toml")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("mkdir config parent: %v", err)
		}
		if err := os.WriteFile(target, []byte("version = 2\n"), 0o644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		defaultConfigPath := filepath.Join(filepath.Dir(target), "certs.d")
		firstHostsPath := filepath.Join(defaultConfigPath, "registry.k8s.io", "hosts.toml")
		secondHostsPath := filepath.Join(defaultConfigPath, "docker.io", "hosts.toml")

		spec := map[string]any{
			"path": target,
			"registryHosts": []any{
				map[string]any{
					"registry":     "registry.k8s.io",
					"server":       "https://registry.k8s.io",
					"host":         "http://127.0.0.1:5000",
					"capabilities": []any{"pull", "resolve"},
					"skipVerify":   true,
				},
				map[string]any{
					"registry":     "docker.io",
					"server":       "https://registry-1.docker.io",
					"host":         "http://127.0.0.1:5001",
					"capabilities": []any{"pull"},
					"skipVerify":   false,
				},
			},
		}

		if err := runWriteContainerdRegistryHosts(map[string]any{"path": defaultConfigPath, "registryHosts": spec["registryHosts"]}); err != nil {
			t.Fatalf("runWriteContainerdRegistryHosts failed: %v", err)
		}

		firstRaw, err := os.ReadFile(firstHostsPath)
		if err != nil {
			t.Fatalf("read first hosts.toml: %v", err)
		}
		if !strings.Contains(string(firstRaw), "server = \"https://registry.k8s.io\"") {
			t.Fatalf("unexpected first hosts.toml content: %q", string(firstRaw))
		}

		secondRaw, err := os.ReadFile(secondHostsPath)
		if err != nil {
			t.Fatalf("read second hosts.toml: %v", err)
		}
		if !strings.Contains(string(secondRaw), "server = \"https://registry-1.docker.io\"") || !strings.Contains(string(secondRaw), "skip_verify = false") {
			t.Fatalf("unexpected second hosts.toml content: %q", string(secondRaw))
		}

		configRaw, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("read config.toml: %v", err)
		}
		if strings.Contains(string(configRaw), "config_path") {
			t.Fatalf("did not expect config_path to be injected when configPath is omitted: %q", string(configRaw))
		}
	})

	t.Run("applies structured edits for toml yaml and json", func(t *testing.T) {
		dir := t.TempDir()

		tomlPath := filepath.Join(dir, "config.toml")
		if err := os.WriteFile(tomlPath, []byte("[plugins.\"io.containerd.grpc.v1.cri\".registry]\nconfig_path = \"\"\n"), 0o644); err != nil {
			t.Fatalf("write toml: %v", err)
		}
		if err := runEditTOML(map[string]any{"path": tomlPath, "edits": []any{map[string]any{"op": "set", "rawPath": `plugins."io.containerd.grpc.v1.cri".registry.config_path`, "value": "/etc/containerd/certs.d"}}}); err != nil {
			t.Fatalf("runEditTOML failed: %v", err)
		}
		tomlRaw, err := os.ReadFile(tomlPath)
		if err != nil {
			t.Fatalf("read toml: %v", err)
		}
		if !strings.Contains(string(tomlRaw), "/etc/containerd/certs.d") {
			t.Fatalf("unexpected toml content: %q", string(tomlRaw))
		}

		yamlPath := filepath.Join(dir, "kubeadm.yaml")
		if err := runEditYAML(map[string]any{"path": yamlPath, "createIfMissing": true, "edits": []any{map[string]any{"op": "set", "rawPath": "ClusterConfiguration.imageRepository", "value": "registry.local/k8s"}}}); err != nil {
			t.Fatalf("runEditYAML failed: %v", err)
		}
		yamlRaw, err := os.ReadFile(yamlPath)
		if err != nil {
			t.Fatalf("read yaml: %v", err)
		}
		if !strings.Contains(string(yamlRaw), "imageRepository: registry.local/k8s") {
			t.Fatalf("unexpected yaml content: %q", string(yamlRaw))
		}

		jsonPath := filepath.Join(dir, "cni.json")
		if err := os.WriteFile(jsonPath, []byte(`{"plugins":[{"type":"loopback"}]}`), 0o644); err != nil {
			t.Fatalf("write json: %v", err)
		}
		if err := runEditJSON(map[string]any{"path": jsonPath, "edits": []any{map[string]any{"op": "set", "rawPath": "plugins.0.type", "value": "bridge"}}}); err != nil {
			t.Fatalf("runEditJSON failed: %v", err)
		}
		jsonRaw, err := os.ReadFile(jsonPath)
		if err != nil {
			t.Fatalf("read json: %v", err)
		}
		if !strings.Contains(string(jsonRaw), `"type": "bridge"`) {
			t.Fatalf("unexpected json content: %q", string(jsonRaw))
		}
	})
}
