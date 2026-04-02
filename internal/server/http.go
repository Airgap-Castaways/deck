package server

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"
)

type HandlerOptions struct {
	AuditMaxSizeMB int
	AuditMaxFiles  int
	AccessLog      io.Writer
}

type serverHandler struct {
	rootAbs string
	logger  *auditLogger
	base    http.Handler
}

func NewHandler(root string, opts HandlerOptions) (http.Handler, error) {
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve server root: %w", err)
	}

	auditMaxSizeMB := opts.AuditMaxSizeMB
	if auditMaxSizeMB <= 0 {
		auditMaxSizeMB = defaultAuditMaxSizeMB
	}
	auditMaxFiles := opts.AuditMaxFiles
	if auditMaxFiles <= 0 {
		auditMaxFiles = defaultAuditMaxFiles
	}

	logger, err := newAuditLogger(resolvedRoot, auditLoggerOptions{maxSizeBytes: int64(auditMaxSizeMB) * 1024 * 1024, maxFiles: auditMaxFiles})
	if err != nil {
		return nil, fmt.Errorf("init audit logger: %w", err)
	}
	h := &serverHandler{rootAbs: resolvedRoot, logger: logger}
	h.base = http.HandlerFunc(h.routeRequest)
	accessLog := opts.AccessLog

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now().UTC()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.base.ServeHTTP(rw, r)
		level := "info"
		if rw.status >= http.StatusInternalServerError {
			level = "error"
		} else if rw.status >= http.StatusBadRequest {
			level = "warn"
		}
		entry := buildServerAuditRecord(start, auditEventRequest, level, "http request handled")
		addExtra(entry, map[string]any{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      rw.status,
			"remote_addr": r.RemoteAddr,
			"duration_ms": time.Since(start).Milliseconds(),
		})
		logger.Write(entry)
		writeAccessLog(accessLog, start, r, rw)
	}), nil
}

func (h *serverHandler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func writeAccessLog(w io.Writer, start time.Time, r *http.Request, rw *statusRecorder) {
	if w == nil {
		return
	}
	_, _ = fmt.Fprintf(
		w,
		"%s - [%s] \"%s %s %s\" %d %d %dms\n",
		r.RemoteAddr,
		start.UTC().Format(time.RFC3339),
		r.Method,
		r.URL.RequestURI(),
		r.Proto,
		rw.status,
		rw.bytes,
		time.Since(start).Milliseconds(),
	)
}
