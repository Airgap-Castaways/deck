package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAuditSchema(t *testing.T) {
	root := t.TempDir()
	h, err := NewHandler(root, HandlerOptions{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	healthRR := httptest.NewRecorder()
	h.ServeHTTP(healthRR, healthReq)
	if healthRR.Code != http.StatusOK {
		t.Fatalf("expected health 200, got %d", healthRR.Code)
	}

	enqueueReq := httptest.NewRequest(http.MethodPost, "/api/agent/job", strings.NewReader(`{"id":"schema-job","type":"noop"}`))
	enqueueRR := httptest.NewRecorder()
	h.ServeHTTP(enqueueRR, enqueueReq)
	if enqueueRR.Code != http.StatusOK {
		t.Fatalf("expected enqueue 200, got %d", enqueueRR.Code)
	}

	entries := readAuditEntries(t, root)
	if len(entries) == 0 {
		t.Fatalf("expected audit entries")
	}

	for _, entry := range entries {
		requireAuditSchemaFields(t, entry)
		if _, exists := entry["timestamp"]; exists {
			t.Fatalf("legacy timestamp field should not exist: %+v", entry)
		}
		if eventType, _ := entry["event_type"].(string); eventType == auditEventRequest {
			if _, exists := entry["method"]; exists {
				t.Fatalf("request metadata should be under extra: %+v", entry)
			}
			extra, ok := entry["extra"].(map[string]any)
			if !ok {
				t.Fatalf("expected request extra object: %+v", entry)
			}
			for _, k := range []string{"method", "path", "status", "remote_addr", "duration_ms"} {
				if _, ok := extra[k]; !ok {
					t.Fatalf("missing request extra field %s in %+v", k, entry)
				}
			}
		}
	}
}

func TestReportAcceptedAudit(t *testing.T) {
	root := t.TempDir()
	h, err := NewHandler(root, HandlerOptions{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/agent/report", strings.NewReader(`{"job_id":"report-job","status":"failed"}`))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected report 200, got %d", rr.Code)
	}

	entries := readAuditEntries(t, root)
	foundAccepted := false
	for _, entry := range entries {
		requireAuditSchemaFields(t, entry)
		eventType, _ := entry["event_type"].(string)
		if eventType != auditEventReportAccepted {
			continue
		}
		jobID, _ := entry["job_id"].(string)
		status, _ := entry["status"].(string)
		if jobID == "report-job" && status == "failed" {
			foundAccepted = true
		}
	}
	if !foundAccepted {
		t.Fatalf("expected %s audit event for report-job", auditEventReportAccepted)
	}
}

func requireAuditSchemaFields(t *testing.T, entry map[string]any) {
	t.Helper()
	for _, key := range []string{"ts", "schema_version", "source", "event_type", "level", "message"} {
		if _, ok := entry[key]; !ok {
			t.Fatalf("missing required audit key %s in %+v", key, entry)
		}
	}

	ts, _ := entry["ts"].(string)
	if _, err := time.Parse(time.RFC3339Nano, ts); err != nil {
		t.Fatalf("invalid ts format %q: %v", ts, err)
	}

	schemaVersion, ok := entry["schema_version"].(float64)
	if !ok || int(schemaVersion) != auditSchemaVersion {
		t.Fatalf("expected schema_version=%d, got %+v", auditSchemaVersion, entry["schema_version"])
	}

	source, _ := entry["source"].(string)
	if source != auditSourceServer {
		t.Fatalf("expected source=%q, got %q", auditSourceServer, source)
	}
}
