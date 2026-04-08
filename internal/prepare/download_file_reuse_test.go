package prepare

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/config"
)

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
