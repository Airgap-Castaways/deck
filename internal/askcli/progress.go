package askcli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

type askProgress struct {
	writer  io.Writer
	enabled bool
}

func newAskProgress(writer io.Writer) askProgress {
	return askProgress{writer: writer, enabled: isTerminalWriter(writer)}
}

func (p askProgress) status(format string, args ...any) {
	if !p.enabled || p.writer == nil {
		return
	}
	_, _ = fmt.Fprintf(p.writer, "ask status: "+format+"\n", args...)
	flushOutput(p.writer)
}

func isTerminalWriter(writer io.Writer) bool {
	if writer == nil || writer == io.Discard {
		return false
	}
	fdWriter, ok := writer.(fileDescriptor)
	if !ok {
		return false
	}
	return isCharDevice(fdWriter.Fd(), "stdout")
}

func flushOutput(writer io.Writer) {
	if writer == nil || writer == io.Discard {
		return
	}
	if flusher, ok := writer.(flushWriter); ok {
		_ = flusher.Flush()
	}
	if syncer, ok := writer.(syncWriter); ok {
		_ = syncer.Sync()
		return
	}
	if file, ok := writer.(*os.File); ok {
		_ = file.Sync()
	}
}

func phaseLabel(route string) string {
	return strings.TrimSpace(route)
}
