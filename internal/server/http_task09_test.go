package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInFlight(t *testing.T) {
	TestLeaseTTLRequeue(t)
}

func TestLeaseTTLRequeue(t *testing.T) {
	root := t.TempDir()
	h, err := NewHandler(root, HandlerOptions{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	server := httptest.NewServer(h)
	defer server.Close()

	enqueueStatus, _ := postJSON(t, server.URL+"/api/agent/job", `{"id":"ttl-job","type":"noop","max_attempts":3}`)
	if enqueueStatus != http.StatusOK {
		t.Fatalf("expected enqueue 200, got %d", enqueueStatus)
	}

	leaseStatus, leaseBody := postJSON(t, server.URL+"/api/agent/lease", `{"hostname":"alpha-node"}`)
	if leaseStatus != http.StatusOK {
		t.Fatalf("expected lease 200, got %d", leaseStatus)
	}

	var leasePayload struct {
		Status string   `json:"status"`
		Job    alphaJob `json:"job"`
	}
	if err := json.Unmarshal(leaseBody, &leasePayload); err != nil {
		t.Fatalf("parse lease payload: %v", err)
	}
	if leasePayload.Job.ID != "ttl-job" || leasePayload.Job.Attempt != 1 {
		t.Fatalf("unexpected leased job payload: %+v", leasePayload.Job)
	}

	stateAfterLease, err := loadAlphaServerState(root)
	if err != nil {
		t.Fatalf("loadAlphaServerState: %v", err)
	}
	if len(stateAfterLease.InFlight) != 1 {
		t.Fatalf("expected 1 in-flight lease, got %d", len(stateAfterLease.InFlight))
	}
	if stateAfterLease.InFlight[0].Job.ID != "ttl-job" {
		t.Fatalf("unexpected in-flight job id: %+v", stateAfterLease.InFlight[0])
	}
	if stateAfterLease.InFlight[0].LeaseTTLSec != defaultLeaseTTLSec {
		t.Fatalf("unexpected lease ttl: %d", stateAfterLease.InFlight[0].LeaseTTLSec)
	}
	if stateAfterLease.InFlight[0].LeasedBy != "alpha-node" {
		t.Fatalf("unexpected leased_by: %q", stateAfterLease.InFlight[0].LeasedBy)
	}
	leasedAt, err := time.Parse(time.RFC3339, stateAfterLease.InFlight[0].LeasedAt)
	if err != nil {
		t.Fatalf("parse leased_at: %v", err)
	}

	hRestart, err := NewHandler(root, HandlerOptions{})
	if err != nil {
		t.Fatalf("NewHandler restart: %v", err)
	}
	restartedServer := httptest.NewServer(hRestart)
	defer restartedServer.Close()

	restartedHandler, ok := hRestart.(*serverHandler)
	if !ok {
		t.Fatalf("unexpected handler type %T", hRestart)
	}
	restartedHandler.sweepLeasesOnce(leasedAt.Add(time.Duration(defaultLeaseTTLSec+1) * time.Second))

	stateAfterSweep, err := loadAlphaServerState(root)
	if err != nil {
		t.Fatalf("loadAlphaServerState after sweep: %v", err)
	}
	if len(stateAfterSweep.InFlight) != 0 {
		t.Fatalf("expected no in-flight leases after sweep, got %d", len(stateAfterSweep.InFlight))
	}

	releaseStatus, releaseBody := postJSON(t, restartedServer.URL+"/api/agent/lease", `{"hostname":"alpha-node"}`)
	if releaseStatus != http.StatusOK {
		t.Fatalf("expected re-lease 200, got %d", releaseStatus)
	}

	var reLeasePayload struct {
		Status string   `json:"status"`
		Job    alphaJob `json:"job"`
	}
	if err := json.Unmarshal(releaseBody, &reLeasePayload); err != nil {
		t.Fatalf("parse re-lease payload: %v", err)
	}
	if reLeasePayload.Job.ID != "ttl-job" || reLeasePayload.Job.Attempt != 2 {
		t.Fatalf("expected requeued job to be leasable with incremented lease attempt, got %+v", reLeasePayload.Job)
	}

	entries := readAuditEntries(t, root)
	foundExpired := false
	for _, entry := range entries {
		eventType, _ := entry["event_type"].(string)
		if eventType != auditEventJobLeaseExpired {
			continue
		}
		jobID, _ := entry["job_id"].(string)
		extra, _ := entry["extra"].(map[string]any)
		decision, _ := extra["decision"].(string)
		if jobID == "ttl-job" && decision == "requeued" {
			foundExpired = true
			break
		}
	}
	if !foundExpired {
		t.Fatalf("expected %s audit entry for ttl-job", auditEventJobLeaseExpired)
	}
}

func TestLateReportAudit(t *testing.T) {
	root := t.TempDir()
	h, err := NewHandler(root, HandlerOptions{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	server := httptest.NewServer(h)
	defer server.Close()

	reportStatus, reportBody := postJSON(t, server.URL+"/api/agent/report", `{"job_id":"late-job","status":"failed","detail":"late"}`)
	if reportStatus != http.StatusOK {
		t.Fatalf("expected report 200, got %d", reportStatus)
	}
	if !strings.Contains(string(reportBody), `"status":"accepted"`) {
		t.Fatalf("unexpected report response: %s", string(reportBody))
	}

	reportsResp, err := http.Get(server.URL + "/api/agent/reports?job_id=late-job")
	if err != nil {
		t.Fatalf("get reports: %v", err)
	}
	defer reportsResp.Body.Close()
	reportsRaw, err := io.ReadAll(reportsResp.Body)
	if err != nil {
		t.Fatalf("read reports response: %v", err)
	}
	if reportsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected reports 200, got %d", reportsResp.StatusCode)
	}
	if !strings.Contains(string(reportsRaw), `"job_id":"late-job"`) {
		t.Fatalf("expected late report to be persisted: %s", string(reportsRaw))
	}

	jobsResp, err := http.Get(server.URL + "/api/agent/jobs")
	if err != nil {
		t.Fatalf("get jobs: %v", err)
	}
	defer jobsResp.Body.Close()
	jobsRaw, err := io.ReadAll(jobsResp.Body)
	if err != nil {
		t.Fatalf("read jobs response: %v", err)
	}
	if !strings.Contains(string(jobsRaw), `"jobs":[]`) {
		t.Fatalf("expected queue to remain unchanged for late report: %s", string(jobsRaw))
	}

	entries := readAuditEntries(t, root)
	foundLate := false
	foundAccepted := false
	for _, entry := range entries {
		eventType, _ := entry["event_type"].(string)
		switch eventType {
		case auditEventReportLate:
			jobID, _ := entry["job_id"].(string)
			if jobID == "late-job" {
				foundLate = true
			}
		case auditEventReportAccepted:
			jobID, _ := entry["job_id"].(string)
			status, _ := entry["status"].(string)
			if jobID == "late-job" && status == "failed" {
				foundAccepted = true
			}
		}
	}
	if !foundLate {
		t.Fatalf("expected %s audit entry", auditEventReportLate)
	}
	if !foundAccepted {
		t.Fatalf("expected %s audit entry", auditEventReportAccepted)
	}
}

func postJSON(t *testing.T, url, body string) (int, []byte) {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read POST response %s: %v", url, err)
	}
	return resp.StatusCode, raw
}

func readAuditEntries(t *testing.T, root string) []map[string]any {
	t.Helper()
	auditPath := filepath.Join(root, ".deck", "logs", "server-audit.log")
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	entries := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		entry := map[string]any{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("parse audit entry: %v", err)
		}
		entries = append(entries, entry)
	}
	return entries
}
