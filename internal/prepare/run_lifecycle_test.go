package prepare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
)

func TestRun_PrepareArtifactAndManifest(t *testing.T) {
	imageOps := stubDownloadImageOps()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello-download-file"))
	}))
	defer server.Close()

	bundle := t.TempDir()

	wf := &config.Workflow{
		Version: "v1",
		Vars: map[string]any{
			"kubernetesVersion": "v1.30.1",
		},
		Phases: []config.Phase{
			{
				Name: "prepare",
				Steps: []config.Step{
					{
						ID:   "download-file",
						Kind: "DownloadFile",
						Spec: map[string]any{
							"source":     map[string]any{"url": server.URL + "/artifact"},
							"outputPath": "files/artifact.bin",
						},
					},
					{
						ID:   "download-os-packages",
						Kind: "DownloadPackage",
						Spec: map[string]any{
							"packages": []any{"containerd", "iptables"},
						},
					},
					{
						ID:   "download-k8s-packages",
						Kind: "DownloadPackage",
						Spec: map[string]any{
							"packages": []any{"kubelet"},
						},
					},
					{
						ID:   "download-images",
						Kind: "DownloadImage",
						Spec: map[string]any{
							"images": []any{"registry.k8s.io/kube-apiserver:{{ .vars.kubernetesVersion }}"},
						},
					},
				},
			},
		},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, imageDownloadOps: imageOps}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expectFiles := []string{
		"files/artifact.bin",
		"packages/containerd.txt",
		"packages/iptables.txt",
		"packages/kubelet.txt",
		"images/registry.k8s.io_kube-apiserver_v1.30.1.tar",
		".deck/manifest.json",
	}

	for _, rel := range expectFiles {
		abs := filepath.Join(bundle, rel)
		if _, err := os.Stat(abs); err != nil {
			t.Fatalf("expected artifact %s: %v", rel, err)
		}
	}

	manifestRaw, err := os.ReadFile(filepath.Join(bundle, ".deck", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var mf struct {
		Entries []struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
			Size   int64  `json:"size"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(manifestRaw, &mf); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}

	if len(mf.Entries) < 5 {
		t.Fatalf("expected >= 5 entries, got %d", len(mf.Entries))
	}
	for _, e := range mf.Entries {
		if e.Path == "" || e.SHA256 == "" || e.Size <= 0 {
			t.Fatalf("invalid manifest entry: %+v", e)
		}
		if strings.HasPrefix(e.Path, "workflows/") || e.Path == "deck" {
			t.Fatalf("manifest must exclude workflow and root deck entries: %+v", e)
		}
		if !strings.HasPrefix(e.Path, "packages/") && !strings.HasPrefix(e.Path, "images/") && !strings.HasPrefix(e.Path, "files/") {
			t.Fatalf("manifest entry outside allowed prefixes: %+v", e)
		}
	}
}

func TestRun_NoPrepareSteps(t *testing.T) {
	wf := &config.Workflow{Version: "v1", Phases: []config.Phase{{Name: "install"}}}
	if err := Run(context.Background(), wf, RunOptions{BundleRoot: t.TempDir()}); err == nil {
		t.Fatalf("expected error when prepare workflow has no steps")
	}
}
