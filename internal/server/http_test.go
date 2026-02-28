package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNewHandler(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "files"), 0o755); err != nil {
		t.Fatalf("mkdir files: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "packages"), 0o755); err != nil {
		t.Fatalf("mkdir packages: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "files", "a.txt"), []byte("file-data"), 0o644); err != nil {
		t.Fatalf("write files entry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "packages", "pkg.txt"), []byte("pkg-data"), 0o644); err != nil {
		t.Fatalf("write packages entry: %v", err)
	}

	h := NewHandler(root)

	t.Run("serves files", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/files/a.txt", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if rr.Body.String() != "file-data" {
			t.Fatalf("unexpected body: %q", rr.Body.String())
		}
	})

	t.Run("serves packages", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/packages/pkg.txt", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rr.Code)
		}
		if rr.Body.String() != "pkg-data" {
			t.Fatalf("unexpected body: %q", rr.Body.String())
		}
	})

	t.Run("returns 404 for unsupported routes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rr.Code)
		}
	})
}
