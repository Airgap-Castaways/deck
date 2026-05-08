package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Airgap-Castaways/deck/internal/filemode"
	"github.com/Airgap-Castaways/deck/internal/fsutil"
)

type auditLogger struct {
	mu sync.Mutex
	f  *os.File

	path         string
	maxSizeBytes int64
	maxFiles     int
}

type auditLoggerOptions struct {
	maxSizeBytes int64
	maxFiles     int
}

const (
	auditSchemaVersion = 2
	auditSourceServer  = "server"
	auditEventRequest  = "http_request"

	defaultAuditMaxSizeMB = 50
	defaultAuditMaxFiles  = 10
)

var reservedAuditKeys = map[string]struct{}{
	"ts":             {},
	"schema_version": {},
	"component":      {},
	"event":          {},
	"level":          {},
	"message":        {},
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	bytes       int64
}

func newAuditLogger(root string, opts auditLoggerOptions) (*auditLogger, error) {
	logPath := filepath.Join(root, ".deck", "logs", "server-audit.log")
	if err := filemode.EnsureParentPrivateDir(logPath); err != nil {
		return nil, fmt.Errorf("create audit log directory: %w", err)
	}
	if opts.maxSizeBytes <= 0 {
		opts.maxSizeBytes = int64(defaultAuditMaxSizeMB) * 1024 * 1024
	}
	if opts.maxFiles <= 0 {
		opts.maxFiles = defaultAuditMaxFiles
	}
	f, err := fsutil.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, filemode.PrivateFileMode)
	if err != nil {
		return nil, fmt.Errorf("open audit log file: %w", err)
	}
	return &auditLogger{f: f, path: logPath, maxSizeBytes: opts.maxSizeBytes, maxFiles: opts.maxFiles}, nil
}

func (a *auditLogger) Write(entry map[string]any) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	var rotateErr error
	if a.shouldRotateLocked() {
		if err := a.rotateLocked(); err != nil {
			rotateErr = fmt.Errorf("rotate audit log: %w", err)
		}
	}
	raw, err := json.Marshal(entry)
	if err != nil {
		return errors.Join(rotateErr, fmt.Errorf("encode audit log entry: %w", err))
	}
	if _, err := a.f.Write(raw); err != nil {
		return errors.Join(rotateErr, fmt.Errorf("write audit log entry: %w", err))
	}
	if _, err := a.f.WriteString("\n"); err != nil {
		return errors.Join(rotateErr, fmt.Errorf("write audit log newline: %w", err))
	}
	return rotateErr
}

func (a *auditLogger) shouldRotateLocked() bool {
	if a.maxSizeBytes <= 0 {
		return false
	}
	info, err := a.f.Stat()
	if err != nil {
		return false
	}
	return info.Size() > a.maxSizeBytes
}

func (a *auditLogger) rotateLocked() error {
	if err := a.f.Close(); err != nil {
		return err
	}
	var firstErr error

	oldestPath := fmt.Sprintf("%s.%d", a.path, a.maxFiles)
	if err := os.Remove(oldestPath); err != nil && !os.IsNotExist(err) {
		firstErr = err
	}
	if firstErr == nil {
		for i := a.maxFiles - 1; i >= 1; i-- {
			src := fmt.Sprintf("%s.%d", a.path, i)
			dst := fmt.Sprintf("%s.%d", a.path, i+1)
			if err := os.Rename(src, dst); err != nil && !os.IsNotExist(err) {
				firstErr = err
				break
			}
		}
	}
	if firstErr == nil {
		if err := os.Rename(a.path, fmt.Sprintf("%s.1", a.path)); err != nil && !os.IsNotExist(err) {
			firstErr = err
		}
	}
	f, err := fsutil.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, filemode.PrivateFileMode)
	if err != nil {
		if firstErr != nil {
			return fmt.Errorf("%v; reopen audit log: %w", firstErr, err)
		}
		return err
	}
	a.f = f
	return firstErr
}

func buildServerAuditRecord(ts time.Time, eventType, level, message string) map[string]any {
	event := strings.TrimSpace(eventType)
	if event == auditEventRequest {
		event = "request"
	}
	return map[string]any{
		"ts":             ts.UTC().Format(time.RFC3339Nano),
		"schema_version": auditSchemaVersion,
		"component":      auditSourceServer,
		"event":          event,
		"level":          level,
		"message":        message,
	}
}

func addExtra(entry map[string]any, extra map[string]any) {
	if len(extra) == 0 {
		return
	}
	for key, value := range extra {
		if _, reserved := reservedAuditKeys[key]; reserved {
			continue
		}
		entry[key] = value
	}
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(p)
	r.bytes += int64(n)
	return n, err
}
