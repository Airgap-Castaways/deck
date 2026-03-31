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
	return askProgress{writer: writer, enabled: writer != nil && writer != io.Discard}
}

func (p askProgress) status(format string, args ...any) {
	if !p.enabled || p.writer == nil {
		return
	}
	_, _ = fmt.Fprintf(p.writer, "ask status: "+format+"\n", args...)
	flushOutput(p.writer)
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
	if fdWriter, ok := writer.(fileDescriptor); ok {
		file := os.NewFile(fdWriter.Fd(), "writer")
		if file != nil {
			_ = file.Sync()
			return
		}
	}
	if file, ok := writer.(*os.File); ok {
		_ = file.Sync()
	}
}

func phaseLabel(route string) string {
	return strings.TrimSpace(route)
}
