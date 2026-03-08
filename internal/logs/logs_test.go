package logs

import (
	"testing"
)

func TestControlLogsNormalizeOldAudit(t *testing.T) {
	lines := []string{
		`{"timestamp":"2025-03-05T11:00:00Z","event_type":"agent_connected","job_id":"job-1","decision":"allow","hostname":"cp-1"}`,
		`{"timestamp":"2025-03-05T11:01:00Z","method":"GET","path":"/healthz","status":200,"remote_addr":"127.0.0.1","duration_ms":12}`,
	}
	records := make([]LogRecord, 0, len(lines))
	for _, line := range lines {
		record, parseErr := NormalizeJSONLine([]byte(line))
		if parseErr != nil {
			t.Fatalf("normalize old audit line: %v", parseErr)
		}
		records = append(records, record)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 normalized records, got %d", len(records))
	}
	if records[0].EventType != "agent_connected" || records[0].Message != "agent connected" {
		t.Fatalf("unexpected lifecycle normalized record: %+v", records[0])
	}
	if records[0].Status != "" || records[0].ExtraValue("hostname") != "cp-1" {
		t.Fatalf("unexpected lifecycle fields: %+v", records[0])
	}
	if records[1].EventType != "http_request" || records[1].Message != "http request handled" {
		t.Fatalf("unexpected request normalized record: %+v", records[1])
	}
	if records[1].Status != "200" || records[1].ExtraValue("path") != "/healthz" {
		t.Fatalf("unexpected request fields: %+v", records[1])
	}
}

func TestControlLogsNormalizeJournalRecord(t *testing.T) {
	line := map[string]any{
		"PRIORITY":             "4",
		"MESSAGE":              "kubelet warning",
		"__REALTIME_TIMESTAMP": "2025-03-05T11:00:00Z",
		"_SYSTEMD_UNIT":        "deck-server.service",
	}
	record := NormalizeJournalRecord(line)
	if record.TS != "2025-03-05T11:00:00Z" {
		t.Fatalf("unexpected ts: %q", record.TS)
	}
	if record.Level != "warn" {
		t.Fatalf("unexpected level: %q", record.Level)
	}
	if record.ExtraValue("unit") != "deck-server.service" {
		t.Fatalf("unexpected unit extra: %v", record.ExtraValue("unit"))
	}
}
