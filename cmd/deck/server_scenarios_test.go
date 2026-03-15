package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestServerScenarios(t *testing.T) {
	items := []string{"prepare", "apply"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method: %s", r.Method)
		}
		if r.URL.Path != "/workflows/index.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(items)
	}))
	defer srv.Close()

	t.Run("text", func(t *testing.T) {
		out, err := runWithCapturedStdout([]string{"server", "scenarios", "--server", srv.URL})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		expected := strings.Join(items, "\n") + "\n"
		if out != expected {
			t.Fatalf("unexpected output\nwant: %q\ngot : %q", expected, out)
		}
	})

	t.Run("json", func(t *testing.T) {
		out, err := runWithCapturedStdout([]string{"server", "scenarios", "--server", srv.URL, "-o", "json"})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		var got []string
		if err := json.Unmarshal([]byte(out), &got); err != nil {
			t.Fatalf("decode json output: %v\nraw: %q", err, out)
		}
		if !reflect.DeepEqual(got, items) {
			t.Fatalf("unexpected items\nwant: %#v\ngot : %#v", items, got)
		}
	})

	t.Run("without server returns guidance error", func(t *testing.T) {
		t.Setenv("DECK_SERVER_CONFIG_PATH", filepath.Join(t.TempDir(), "server.json"))
		_, err := runWithCapturedStdout([]string{"server", "scenarios"})
		if err == nil {
			t.Fatalf("expected error, got success")
		}
		if !strings.Contains(err.Error(), "--server is required") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("server 404 index returns empty list", func(t *testing.T) {
		missing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		}))
		defer missing.Close()

		textOut, err := runWithCapturedStdout([]string{"server", "scenarios", "--server", missing.URL})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if textOut != "" {
			t.Fatalf("expected empty text output, got %q", textOut)
		}

		jsonOut, err := runWithCapturedStdout([]string{"server", "scenarios", "--server", missing.URL, "-o", "json"})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		var got []string
		if err := json.Unmarshal([]byte(jsonOut), &got); err != nil {
			t.Fatalf("decode json output: %v\nraw: %q", err, jsonOut)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty json list, got %#v", got)
		}
	})
}
