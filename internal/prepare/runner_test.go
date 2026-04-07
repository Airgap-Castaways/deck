package prepare

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Airgap-Castaways/deck/internal/config"
	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/workflowcontract"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

func nilContextForPrepareTest() context.Context { return nil }

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

func TestRun_FileFallbackLocalThenBundle(t *testing.T) {
	bundleOut := t.TempDir()
	localCache := t.TempDir()
	bundleCache := t.TempDir()

	relSource := filepath.ToSlash(filepath.Join("files", "artifact.bin"))
	bundleOnlyPath := filepath.Join(bundleCache, filepath.FromSlash(relSource))
	if err := os.MkdirAll(filepath.Dir(bundleOnlyPath), 0o755); err != nil {
		t.Fatalf("mkdir bundle cache path: %v", err)
	}
	if err := os.WriteFile(bundleOnlyPath, []byte("from-bundle-source"), 0o644); err != nil {
		t.Fatalf("write bundle cache source: %v", err)
	}
	sum := sha256.Sum256([]byte("from-bundle-source"))

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source": map[string]any{
						"path":   relSource,
						"sha256": hex.EncodeToString(sum[:]),
					},
					"fetch": map[string]any{
						"strategy": "fallback",
						"sources": []any{
							map[string]any{"type": "local", "path": localCache},
							map[string]any{"type": "bundle", "path": bundleCache},
						},
					},
					"outputPath": "files/fetched.bin",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundleOut}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(bundleOut, "files", "fetched.bin"))
	if err != nil {
		t.Fatalf("read fetched output: %v", err)
	}
	if string(raw) != "from-bundle-source" {
		t.Fatalf("unexpected fetched content: %q", string(raw))
	}
}

func TestRun_FileFallbackSourceMissing(t *testing.T) {
	bundleOut := t.TempDir()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source": map[string]any{
						"path": "files/missing.bin",
					},
					"fetch": map[string]any{
						"strategy": "fallback",
						"sources":  []any{map[string]any{"type": "local", "path": t.TempDir()}},
					},
					"outputPath": "files/out.bin",
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundleOut})
	if err == nil {
		t.Fatalf("expected source not found error")
	}
	if !errcode.Is(err, errCodeArtifactSourceNotFound) {
		t.Fatalf("expected typed code %s, got %v", errCodeArtifactSourceNotFound, err)
	}
	if !strings.Contains(err.Error(), "E_PREPARE_SOURCE_NOT_FOUND") {
		t.Fatalf("expected E_PREPARE_SOURCE_NOT_FOUND, got %v", err)
	}
}

func TestRun_DownloadFileRejectsNonCanonicalOutputPath(t *testing.T) {
	bundleOut := t.TempDir()
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source":     map[string]any{"url": "https://example.invalid/artifact.bin"},
					"outputPath": "artifacts/out.bin",
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundleOut})
	if err == nil || !strings.Contains(err.Error(), "DownloadFile outputPath must stay under files/") {
		t.Fatalf("expected canonical outputPath error, got %v", err)
	}
}

func TestResolveSourceBytes_PreservesContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
		_, _ = w.Write([]byte("should-not-complete"))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := resolveSourceBytes(ctx, map[string]any{
		"fetch": map[string]any{
			"sources": []any{map[string]any{"type": "online", "url": server.URL}},
		},
	}, "files/remote.bin")
	if err == nil {
		t.Fatalf("expected context cancellation error")
	}
	if strings.Contains(err.Error(), "E_PREPARE_SOURCE_NOT_FOUND") {
		t.Fatalf("expected cancellation to not be mapped to source-not-found, got %v", err)
	}
	if !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("expected canceled context in error, got %v", err)
	}
}

func TestRun_FileFallbackRepoThenOnline(t *testing.T) {
	bundleOut := t.TempDir()

	repo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer repo.Close()

	online := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/files/remote.bin" {
			_, _ = w.Write([]byte("from-online-source"))
			return
		}
		http.NotFound(w, r)
	}))
	defer online.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source": map[string]any{
						"path": "files/remote.bin",
					},
					"fetch": map[string]any{
						"strategy": "fallback",
						"sources": []any{
							map[string]any{"type": "repo", "url": repo.URL},
							map[string]any{"type": "online", "url": online.URL},
						},
					},
					"outputPath": "files/fetched-online.bin",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundleOut}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(bundleOut, "files", "fetched-online.bin"))
	if err != nil {
		t.Fatalf("read fetched output: %v", err)
	}
	if string(raw) != "from-online-source" {
		t.Fatalf("unexpected fetched content: %q", string(raw))
	}
}

func TestRun_FileOfflinePolicyBlocksOnlineFallback(t *testing.T) {
	bundleOut := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("should-not-be-downloaded"))
	}))
	defer server.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source": map[string]any{
						"path": "files/not-found.bin",
						"url":  server.URL + "/files/not-found.bin",
					},
					"fetch": map[string]any{
						"offlineOnly": true,
						"strategy":    "fallback",
						"sources":     []any{map[string]any{"type": "online", "url": server.URL}},
					},
					"outputPath": "files/out.bin",
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundleOut})
	if err == nil {
		t.Fatalf("expected offline policy block error")
	}
	if !errcode.Is(err, errCodePrepareOfflinePolicyBlock) {
		t.Fatalf("expected typed code %s, got %v", errCodePrepareOfflinePolicyBlock, err)
	}
	if !strings.Contains(err.Error(), "E_PREPARE_OFFLINE_POLICY_BLOCK") {
		t.Fatalf("expected E_PREPARE_OFFLINE_POLICY_BLOCK, got %v", err)
	}
}

func TestRun_FileOfflinePolicyBlocksDirectURL(t *testing.T) {
	bundleOut := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("should-not-be-downloaded"))
	}))
	defer server.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source":     map[string]any{"url": server.URL + "/files/a.bin"},
					"fetch":      map[string]any{"offlineOnly": true},
					"outputPath": "files/out.bin",
				},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundleOut})
	if err == nil {
		t.Fatalf("expected offline policy block error")
	}
	if !errcode.Is(err, errCodePrepareOfflinePolicyBlock) {
		t.Fatalf("expected typed code %s, got %v", errCodePrepareOfflinePolicyBlock, err)
	}
	if !strings.Contains(err.Error(), "E_PREPARE_OFFLINE_POLICY_BLOCK") {
		t.Fatalf("expected E_PREPARE_OFFLINE_POLICY_BLOCK, got %v", err)
	}
}

func TestRun_DownloadFileReusesURLOnlyArtifact(t *testing.T) {
	bundle := t.TempDir()
	var mu sync.Mutex
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		n := requests
		mu.Unlock()
		_, _ = fmt.Fprintf(w, "payload-%d", n)
	}))
	defer server.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source":     map[string]any{"url": server.URL + "/artifact.bin"},
					"outputPath": "files/out.bin",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("second Run failed: %v", err)
	}

	mu.Lock()
	gotRequests := requests
	mu.Unlock()
	if gotRequests != 1 {
		t.Fatalf("expected URL download to be reused, got %d requests", gotRequests)
	}

	raw, err := os.ReadFile(filepath.Join(bundle, "files", "out.bin"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(raw) != "payload-1" {
		t.Fatalf("expected reused payload, got %q", string(raw))
	}
}

func TestRun_DownloadFileURLOnlyPersistsSidecarMetadata(t *testing.T) {
	bundle := t.TempDir()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"artifact-v1"`)
		w.Header().Set("Last-Modified", "Tue, 07 Apr 2026 07:00:00 GMT")
		_, _ = io.WriteString(w, "payload-1")
	}))
	defer server.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source":     map[string]any{"url": server.URL + "/artifact.bin"},
					"outputPath": "files/out.bin",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	target := filepath.Join(bundle, "files", "out.bin")
	meta, ok := readDownloadFileSidecar(target)
	if !ok {
		t.Fatalf("expected download sidecar metadata for %s", target)
	}
	if meta.URL != server.URL+"/artifact.bin" {
		t.Fatalf("unexpected sidecar url: %#v", meta)
	}
	if meta.ETag != `"artifact-v1"` || meta.LastModified != "Tue, 07 Apr 2026 07:00:00 GMT" {
		t.Fatalf("expected validators in sidecar, got %#v", meta)
	}
	if meta.ContentLength != int64(len("payload-1")) {
		t.Fatalf("expected content length %d, got %#v", len("payload-1"), meta)
	}
	targetSHA, err := fileSHA256(target)
	if err != nil {
		t.Fatalf("fileSHA256: %v", err)
	}
	if meta.LocalSHA256 != targetSHA {
		t.Fatalf("expected sidecar local sha %s, got %#v", targetSHA, meta)
	}
}

func TestRun_DownloadFileForceRedownloadBypassesURLReuse(t *testing.T) {
	bundle := t.TempDir()
	var mu sync.Mutex
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		n := requests
		mu.Unlock()
		_, _ = fmt.Fprintf(w, "payload-%d", n)
	}))
	defer server.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source":     map[string]any{"url": server.URL + "/artifact.bin"},
					"outputPath": "files/out.bin",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, ForceRedownload: true}); err != nil {
		t.Fatalf("forced second Run failed: %v", err)
	}

	mu.Lock()
	gotRequests := requests
	mu.Unlock()
	if gotRequests != 2 {
		t.Fatalf("expected forced redownload to hit URL again, got %d requests", gotRequests)
	}

	raw, err := os.ReadFile(filepath.Join(bundle, "files", "out.bin"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(raw) != "payload-2" {
		t.Fatalf("expected refreshed payload, got %q", string(raw))
	}
}

func TestRun_DownloadFileFailedRedownloadKeepsExistingArtifact(t *testing.T) {
	bundle := t.TempDir()
	var mu sync.Mutex
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		n := requests
		mu.Unlock()
		if n == 1 {
			_, _ = w.Write([]byte("payload-1"))
			return
		}
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source":     map[string]any{"url": server.URL + "/artifact.bin"},
					"outputPath": "files/out.bin",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	target := filepath.Join(bundle, "files", "out.bin")
	metaBefore, ok := readDownloadFileSidecar(target)
	if !ok {
		t.Fatalf("expected sidecar metadata before failed redownload")
	}
	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, ForceRedownload: true})
	if err == nil {
		t.Fatalf("expected forced redownload failure")
	}

	raw, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("read output: %v", readErr)
	}
	if string(raw) != "payload-1" {
		t.Fatalf("expected existing payload to survive failed redownload, got %q", string(raw))
	}
	metaAfter, ok := readDownloadFileSidecar(target)
	if !ok {
		t.Fatalf("expected sidecar metadata after failed redownload")
	}
	if metaAfter != metaBefore {
		t.Fatalf("expected sidecar metadata to stay unchanged, before=%#v after=%#v", metaBefore, metaAfter)
	}

	matches, globErr := filepath.Glob(filepath.Join(bundle, "files", ".*.tmp-*"))
	if globErr != nil {
		t.Fatalf("glob temp files: %v", globErr)
	}
	if len(matches) != 0 {
		t.Fatalf("expected temp files to be cleaned up, got %v", matches)
	}
}

func TestRun_DownloadFileCorruptedURLOnlyArtifactTriggersRedownload(t *testing.T) {
	bundle := t.TempDir()
	var mu sync.Mutex
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		n := requests
		mu.Unlock()
		_, _ = fmt.Fprintf(w, "payload-%d", n)
	}))
	defer server.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source":     map[string]any{"url": server.URL + "/artifact.bin"},
					"outputPath": "files/out.bin",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	target := filepath.Join(bundle, "files", "out.bin")
	if err := os.WriteFile(target, []byte("corrupted"), 0o644); err != nil {
		t.Fatalf("corrupt output: %v", err)
	}
	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("second Run failed: %v", err)
	}

	mu.Lock()
	gotRequests := requests
	mu.Unlock()
	if gotRequests != 2 {
		t.Fatalf("expected corrupted url-only artifact to trigger redownload, got %d requests", gotRequests)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(raw) != "payload-2" {
		t.Fatalf("expected redownloaded payload, got %q", string(raw))
	}
}

func TestRun_DownloadFileRemoteValidatorChangeTriggersRedownload(t *testing.T) {
	bundle := t.TempDir()
	var mu sync.Mutex
	requests := 0
	currentETag := `"artifact-v1"`
	currentLastModified := "Tue, 07 Apr 2026 07:00:00 GMT"
	currentPayload := "payload-1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		requests++
		if match := strings.TrimSpace(r.Header.Get("If-None-Match")); match != "" && match == currentETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", currentETag)
		w.Header().Set("Last-Modified", currentLastModified)
		_, _ = io.WriteString(w, currentPayload)
	}))
	defer server.Close()

	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: map[string]any{
					"source":     map[string]any{"url": server.URL + "/artifact.bin"},
					"outputPath": "files/out.bin",
				},
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	mu.Lock()
	currentETag = `"artifact-v2"`
	currentLastModified = "Tue, 07 Apr 2026 08:00:00 GMT"
	currentPayload = "payload-2"
	mu.Unlock()
	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("second Run failed: %v", err)
	}

	mu.Lock()
	gotRequests := requests
	mu.Unlock()
	if gotRequests != 2 {
		t.Fatalf("expected validator change check plus reused download response to use 2 requests total, got %d", gotRequests)
	}
	raw, err := os.ReadFile(filepath.Join(bundle, "files", "out.bin"))
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if string(raw) != "payload-2" {
		t.Fatalf("expected refreshed payload after validator change, got %q", string(raw))
	}
}

func TestRun_DownloadFileOfflineOnlySkipsRemoteValidatorCheck(t *testing.T) {
	bundle := t.TempDir()
	var mu sync.Mutex
	requests := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		w.Header().Set("ETag", `"artifact-v1"`)
		w.Header().Set("Last-Modified", "Tue, 07 Apr 2026 07:00:00 GMT")
		_, _ = io.WriteString(w, "payload-1")
	}))
	defer server.Close()

	stepSpec := map[string]any{
		"source":     map[string]any{"url": server.URL + "/artifact.bin"},
		"outputPath": "files/out.bin",
	}
	wf := &config.Workflow{
		Version: "v1",
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "download-file",
				Kind: "DownloadFile",
				Spec: stepSpec,
			}},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	stepSpec["fetch"] = map[string]any{"offlineOnly": true}
	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("second Run failed: %v", err)
	}

	mu.Lock()
	gotRequests := requests
	mu.Unlock()
	if gotRequests != 1 {
		t.Fatalf("expected offlineOnly reuse to skip remote validator checks, got %d requests", gotRequests)
	}
}

func TestRun_WhenAndRegisterSemantics(t *testing.T) {
	bundle := t.TempDir()
	localCache := t.TempDir()

	sourceRel := filepath.ToSlash(filepath.Join("files", "a.bin"))
	sourceAbs := filepath.Join(localCache, filepath.FromSlash(sourceRel))
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "control-plane"},
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{
				{
					ID:   "download-a",
					Kind: "DownloadFile",
					Spec: map[string]any{
						"source":     map[string]any{"path": sourceRel},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
						"outputPath": "files/a-out.bin",
					},
					Register: map[string]string{"downloaded": "outputPath"},
				},
				{
					ID:   "download-b",
					Kind: "DownloadFile",
					When: "vars.role == \"control-plane\"",
					Spec: map[string]any{
						"source":     map[string]any{"path": "{{ .runtime.downloaded }}"},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "bundle", "path": bundle}}},
						"outputPath": "files/b-out.bin",
					},
				},
				{
					ID:   "skip-worker-only",
					Kind: "DownloadFile",
					When: "vars.role == \"worker\"",
					Spec: map[string]any{
						"source":     map[string]any{"path": sourceRel},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
						"outputPath": "files/skip.bin",
					},
				},
			},
		}},
	}

	if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(bundle, "files", "a-out.bin")); err != nil {
		t.Fatalf("expected a-out artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "files", "b-out.bin")); err != nil {
		t.Fatalf("expected b-out artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle, "files", "skip.bin")); err == nil {
		t.Fatalf("expected skipped artifact to not exist")
	}
}

func TestRun_EmitsStepEvents(t *testing.T) {
	bundle := t.TempDir()
	localCache := t.TempDir()

	sourceRel := filepath.ToSlash(filepath.Join("files", "a.bin"))
	sourceAbs := filepath.Join(localCache, filepath.FromSlash(sourceRel))
	if err := os.MkdirAll(filepath.Dir(sourceAbs), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	if err := os.WriteFile(sourceAbs, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "control-plane"},
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{
				{
					ID:   "download-a",
					Kind: "DownloadFile",
					Spec: map[string]any{
						"source":     map[string]any{"path": sourceRel},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
						"outputPath": "files/a-out.bin",
					},
				},
				{
					ID:   "skip-worker-only",
					Kind: "DownloadFile",
					When: "vars.role == \"worker\"",
					Spec: map[string]any{
						"source":     map[string]any{"path": sourceRel},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
						"outputPath": "files/skip.bin",
					},
				},
			},
		}},
	}

	var events []StepEvent
	if err := Run(context.Background(), wf, RunOptions{
		BundleRoot: bundle,
		EventSink: func(event StepEvent) {
			events = append(events, event)
		},
	}); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	stepEvents := make([]StepEvent, 0, len(events))
	for _, event := range events {
		if event.StepID != "" {
			stepEvents = append(stepEvents, event)
		}
	}
	if len(stepEvents) != 3 {
		t.Fatalf("expected 3 step events, got %#v", events)
	}
	if stepEvents[0].StepID != "download-a" || stepEvents[0].Status != "started" || stepEvents[0].Phase != "prepare" || stepEvents[0].Attempt != 1 {
		t.Fatalf("unexpected first event: %+v", stepEvents[0])
	}
	if stepEvents[1].StepID != "download-a" || stepEvents[1].Status != "succeeded" || stepEvents[1].Phase != "prepare" || stepEvents[1].Attempt != 1 {
		t.Fatalf("unexpected second event: %+v", stepEvents[1])
	}
	if stepEvents[2].StepID != "skip-worker-only" || stepEvents[2].Status != "skipped" || stepEvents[2].Reason != "when" || stepEvents[2].Phase != "prepare" {
		t.Fatalf("unexpected third event: %+v", stepEvents[2])
	}
}

func TestRunPrepareStep_DownloadFileItems(t *testing.T) {
	bundle := t.TempDir()
	localCache := t.TempDir()

	firstRel := filepath.ToSlash(filepath.Join("files", "first.bin"))
	secondRel := filepath.ToSlash(filepath.Join("files", "second.bin"))
	for rel, content := range map[string]string{firstRel: "first", secondRel: "second"} {
		abs := filepath.Join(localCache, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir source dir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write source: %v", err)
		}
	}

	step := config.Step{Kind: "DownloadFile"}
	key := workflowexec.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: "DownloadFile"}
	rendered := map[string]any{
		"items": []any{
			map[string]any{
				"source":     map[string]any{"path": firstRel},
				"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
				"outputPath": "files/out-first.bin",
			},
			map[string]any{
				"source":     map[string]any{"path": secondRel},
				"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": localCache}}},
				"outputPath": "files/out-second.bin",
			},
		},
	}

	files, outputs, err := runPrepareRenderedStepWithKey(context.Background(), nil, bundle, step, rendered, key, nil, RunOptions{})
	if err != nil {
		t.Fatalf("runPrepareRenderedStepWithKey failed: %v", err)
	}
	if len(files) != 2 || files[0] != "files/out-first.bin" || files[1] != "files/out-second.bin" {
		t.Fatalf("unexpected files: %#v", files)
	}
	if _, ok := outputs["outputPath"]; ok {
		t.Fatalf("did not expect single outputPath for multi-item download: %#v", outputs)
	}
	paths, ok := outputs["outputPaths"].([]string)
	if !ok || len(paths) != 2 {
		t.Fatalf("expected outputPaths list, got %#v", outputs["outputPaths"])
	}
	for _, rel := range files {
		if _, err := os.Stat(filepath.Join(bundle, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("expected artifact %s: %v", rel, err)
		}
	}
}

func TestRun_RetrySemantics(t *testing.T) {
	t.Run("retry succeeds on second attempt", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		bundle := t.TempDir()
		runner := &failOnceRunner{}
		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "prepare",
				Steps: []config.Step{{
					ID:    "retry-packages",
					Kind:  "DownloadPackage",
					Retry: 1,
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

		if err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle, CommandRunner: runner}); err != nil {
			t.Fatalf("expected retry success, got %v", err)
		}
		if runner.attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", runner.attempts)
		}
	})

	t.Run("retry exhausted keeps failure", func(t *testing.T) {
		bundle := t.TempDir()

		wf := &config.Workflow{
			Version: "v1",
			Phases: []config.Phase{{
				Name: "prepare",
				Steps: []config.Step{{
					ID:    "retry-fail",
					Kind:  "DownloadFile",
					Retry: 1,
					Spec: map[string]any{
						"source":     map[string]any{"path": "files/missing.bin"},
						"fetch":      map[string]any{"sources": []any{map[string]any{"type": "local", "path": t.TempDir()}}},
						"outputPath": "files/retry-fail.bin",
					},
				}},
			}},
		}

		err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle})
		if err == nil {
			t.Fatalf("expected failure after retry exhaustion")
		}
		if !strings.Contains(err.Error(), "E_PREPARE_SOURCE_NOT_FOUND") {
			t.Fatalf("expected E_PREPARE_SOURCE_NOT_FOUND, got %v", err)
		}
	})
}

func TestRun_WhenInvalidExpression(t *testing.T) {
	bundle := t.TempDir()
	wf := &config.Workflow{
		Version: "v1",
		Vars:    map[string]any{"role": "worker"},
		Phases: []config.Phase{{
			Name: "prepare",
			Steps: []config.Step{{
				ID:   "bad-when",
				Kind: "DownloadPackage",
				When: "vars.role = \"worker\"",
				Spec: map[string]any{"packages": []any{"containerd"}},
			}},
		}},
	}

	err := Run(context.Background(), wf, RunOptions{BundleRoot: bundle})
	if err == nil {
		t.Fatalf("expected condition eval error")
	}
	if !strings.Contains(err.Error(), "E_CONDITION_EVAL") {
		t.Fatalf("expected E_CONDITION_EVAL, got %v", err)
	}
}

func TestWhen_NamespaceEnforced(t *testing.T) {
	vars := map[string]any{"nodeRole": "worker"}
	runtimeVars := map[string]any{"hostPassed": true}
	ok, err := EvaluateWhen("vars.nodeRole == \"worker\"", vars, runtimeVars)
	if err != nil {
		t.Fatalf("expected vars namespace expression to pass, got %v", err)
	}
	if !ok {
		t.Fatalf("expected vars namespace expression to be true")
	}

	_, err = EvaluateWhen("nodeRole == \"worker\"", vars, runtimeVars)
	if err == nil {
		t.Fatalf("expected bare identifier to fail")
	}
	if !strings.Contains(err.Error(), "unknown identifier \"nodeRole\"; use vars.nodeRole") {
		t.Fatalf("expected bare identifier guidance, got %v", err)
	}

	_, err = EvaluateWhen("context.nodeRole == \"worker\"", vars, runtimeVars)
	if err == nil {
		t.Fatalf("expected context namespace to fail")
	}
	if !strings.Contains(err.Error(), "unknown identifier \"context.nodeRole\"; supported prefixes are vars. and runtime") {
		t.Fatalf("expected namespace restriction message, got %v", err)
	}

	_, err = EvaluateWhen("other.nodeRole == \"worker\"", vars, runtimeVars)
	if err == nil {
		t.Fatalf("expected unknown dotted namespace to fail")
	}
	if !strings.Contains(err.Error(), "unknown identifier \"other.nodeRole\"; supported prefixes are vars. and runtime") {
		t.Fatalf("expected namespace restriction message, got %v", err)
	}
}

func TestRunPrepareStep_DownloadFileDecodeError(t *testing.T) {
	step := config.Step{Kind: "DownloadFile", Spec: map[string]any{"source": 42}}
	key := workflowexec.StepTypeKey{APIVersion: workflowcontract.BuiltInStepAPIVersion, Kind: "DownloadFile"}
	_, _, err := runPrepareRenderedStepWithKey(context.Background(), &fakeRunner{}, t.TempDir(), step, step.Spec, key, nil, RunOptions{})
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode prepare File spec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDownloadURLToFile_RejectsNilContext(t *testing.T) {
	target, err := os.CreateTemp(t.TempDir(), "download-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer func() { _ = target.Close() }()
	if _, err := downloadURLToFile(nilContextForPrepareTest(), target, "https://example.invalid/file", nil); err == nil {
		t.Fatalf("expected nil context error")
	} else if !strings.Contains(err.Error(), "context is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

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

func TestTemplate_RenderVarsAndRuntime(t *testing.T) {
	wf := &config.Workflow{Vars: map[string]any{
		"kubernetesVersion": "v1.30.1",
		"registry":          map[string]any{"host": "registry.k8s.io"},
		"downloads": []any{
			map[string]any{"source": map[string]any{"path": "files/a.bin"}, "outputPath": "files/download-a.bin"},
			map[string]any{"source": map[string]any{"url": "https://example.invalid/b"}, "outputPath": "files/download-b.bin"},
		},
		"downloadSpec": map[string]any{"source": map[string]any{"path": "files/spec.bin"}, "outputPath": "files/spec-out.bin"},
	}}
	runtimeVars := map[string]any{"downloaded": "files/a.bin"}

	rendered, err := renderSpec(map[string]any{
		"source":     map[string]any{"path": "{{ .runtime.downloaded }}"},
		"outputPath": "files/{{ .vars.kubernetesVersion }}.bin",
		"downloads":  "{{ .vars.downloads }}",
		"download":   "{{ .vars.downloadSpec }}",
		"firstPath":  "{{ index .vars.downloads 0 \"outputPath\" }}",
		"images": []any{
			"{{ .vars.registry.host }}/kube-apiserver:{{ .vars.kubernetesVersion }}",
			map[string]any{"tag": "{{ .runtime.downloaded }}"},
			7,
		},
	}, wf, runtimeVars)
	if err != nil {
		t.Fatalf("renderSpec failed: %v", err)
	}

	source, ok := rendered["source"].(map[string]any)
	if !ok || source["path"] != "files/a.bin" {
		t.Fatalf("unexpected rendered source: %#v", rendered["source"])
	}
	outputPath, ok := rendered["outputPath"].(string)
	if !ok || outputPath != "files/v1.30.1.bin" {
		t.Fatalf("unexpected rendered output: %#v", rendered["outputPath"])
	}
	images, ok := rendered["images"].([]any)
	if !ok {
		t.Fatalf("images should be slice, got %#v", rendered["images"])
	}
	if got := images[0]; got != "registry.k8s.io/kube-apiserver:v1.30.1" {
		t.Fatalf("unexpected rendered images[0]: %#v", got)
	}
	imageMap, ok := images[1].(map[string]any)
	if !ok || imageMap["tag"] != "files/a.bin" {
		t.Fatalf("unexpected rendered images[1]: %#v", images[1])
	}
	if got := images[2]; got != 7 {
		t.Fatalf("unexpected rendered images[2]: %#v", got)
	}
	downloads, ok := rendered["downloads"].([]any)
	if !ok || len(downloads) != 2 {
		t.Fatalf("downloads should be structured slice, got %#v", rendered["downloads"])
	}
	firstDownload, ok := downloads[0].(map[string]any)
	if !ok || firstDownload["outputPath"] != "files/download-a.bin" {
		t.Fatalf("unexpected rendered downloads[0]: %#v", downloads[0])
	}
	download, ok := rendered["download"].(map[string]any)
	if !ok || download["outputPath"] != "files/spec-out.bin" {
		t.Fatalf("download should be structured map, got %#v", rendered["download"])
	}
	if got := rendered["firstPath"]; got != "files/download-a.bin" {
		t.Fatalf("unexpected indexed render value: %#v", got)
	}

	_, err = renderSpec(map[string]any{"content": "{{ .vars.missing }}"}, wf, runtimeVars)
	if err == nil {
		t.Fatalf("expected unresolved template reference error")
	}
	if !strings.Contains(err.Error(), "spec.content") {
		t.Fatalf("expected error to include spec path, got %v", err)
	}
}
