package askcli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontract"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askretrieve"
	"github.com/Airgap-Castaways/deck/internal/askreview"
	"github.com/Airgap-Castaways/deck/internal/filemode"
	"github.com/Airgap-Castaways/deck/internal/logs"
)

type Options struct {
	Root          string
	Prompt        string
	FromPath      string
	Answers       []string
	PlanOnly      bool
	PlanName      string
	PlanDir       string
	Create        bool
	Edit          bool
	Review        bool
	MaxIterations int
	Provider      string
	Model         string
	Endpoint      string
	Stdin         io.Reader
	Stdout        io.Writer
	Stderr        io.Writer
}

type runResult struct {
	Route              askintent.Route
	Target             askintent.Target
	Confidence         float64
	Reason             string
	Summary            string
	Answer             string
	ReviewLines        []string
	LintSummary        string
	LocalFindings      []askreview.Finding
	Files              []askcontract.GeneratedFile
	WroteFiles         bool
	RetriesUsed        int
	LLMUsed            bool
	ClassifierLLM      bool
	Termination        string
	Chunks             []askretrieve.Chunk
	DroppedChunks      []string
	AugmentEvents      []string
	UserCommand        string
	PromptTraces       []promptTrace
	ConfigSource       askconfig.EffectiveSettings
	Plan               *askcontract.PlanResponse
	PlanMarkdown       string
	PlanJSON           string
	FallbackNote       string
	Critic             *askcontract.CriticResponse
	Judge              *askcontract.JudgeResponse
	PlanCritic         *askcontract.PlanCriticResponse
	ApprovedPaths      []string
	ToolCalls          []string
	ToolTranscriptPath string
	CandidateFiles     []string
}

type promptTrace struct {
	Label        string
	SystemPrompt string
	UserPrompt   string
}

type askLogger struct {
	writer io.Writer
	level  string
	state  *askLoggerState
}

type askLoggerState struct {
	mu         sync.Mutex
	root       string
	runDir     string
	payloadSeq map[string]int
}

type flushWriter interface {
	Flush() error
}

type syncWriter interface {
	Sync() error
}

func newAskLogger(writer io.Writer, level string, roots ...string) askLogger {
	if writer == nil {
		writer = io.Discard
	}
	root := ""
	if len(roots) > 0 {
		root = strings.TrimSpace(roots[0])
	}
	return askLogger{writer: writer, level: askconfigLogLevel(level), state: &askLoggerState{root: root, payloadSeq: map[string]int{}}}
}

func (l askLogger) enabled(required string) bool {
	return shouldLogAsk(l.level, required)
}

func (l askLogger) info(event string, attrs ...any) {
	l.emit("basic", "info", event, attrs...)
}

func (l askLogger) debug(event string, attrs ...any) {
	l.emit("debug", "debug", event, attrs...)
}

func (l askLogger) trace(event string, attrs ...any) {
	l.emit("trace", "trace", event, attrs...)
}

func (l askLogger) emit(required string, level string, event string, attrs ...any) {
	if !l.enabled(required) {
		return
	}
	_ = logs.WriteCLIEvent(l.writer, logs.CLIEvent{TS: time.Now().UTC(), Level: level, Component: "ask", Event: strings.TrimSpace(event), Attrs: askLogFields(attrs...)})
	l.flush()
}

func (l askLogger) prompt(label string, systemPrompt string, userPrompt string) {
	if !l.enabled("trace") {
		return
	}
	l.emitPayload("prompt", strings.TrimSpace(label), "system", strings.TrimSpace(systemPrompt))
	l.emitPayload("prompt", strings.TrimSpace(label), "user", strings.TrimSpace(userPrompt))
}

func (l askLogger) response(label string, content string) {
	if !l.enabled("trace") {
		return
	}
	l.emitPayload("response", strings.TrimSpace(label), "", strings.TrimSpace(content))
}

func (l askLogger) emitPayload(event string, label string, stream string, content string) {
	attrs := []any{"label", askValueOrDash(label), "content", content}
	if strings.TrimSpace(stream) != "" {
		attrs = append(attrs, "stream", strings.TrimSpace(stream))
	}
	if path := l.writePayloadArtifact(event, label, stream, content); path != "" {
		attrs = append(attrs, "path", path)
	}
	l.trace(event, attrs...)
}

func (l askLogger) writePayloadArtifact(event string, label string, stream string, content string) string {
	if l.state == nil || strings.TrimSpace(l.state.root) == "" {
		return ""
	}
	l.state.mu.Lock()
	defer l.state.mu.Unlock()
	if l.state.runDir == "" {
		runName := fmt.Sprintf("%s-%d", time.Now().UTC().Format("2006-01-02-150405.000000000"), os.Getpid())
		runDir := filepath.Join(l.state.root, ".deck", "ask", "runs", runName)
		if err := filemode.EnsureDir(runDir, filemode.PrivateState); err != nil {
			return ""
		}
		l.state.runDir = runDir
	}
	kindDir := "responses"
	fileName := payloadFileName(label, event, stream, l.nextPayloadSeq(event, label, stream))
	if event == "prompt" {
		kindDir = "prompts"
	}
	kindPath := filepath.Join(l.state.runDir, kindDir)
	if err := filemode.EnsureDir(kindPath, filemode.PrivateState); err != nil {
		return ""
	}
	absPath := filepath.Join(kindPath, fileName)
	if err := filemode.WriteFile(absPath, []byte(content+"\n"), filemode.PrivateState); err != nil {
		return ""
	}
	relPath, err := filepath.Rel(l.state.root, absPath)
	if err != nil {
		return filepath.ToSlash(absPath)
	}
	return filepath.ToSlash(relPath)
}

func (l askLogger) nextPayloadSeq(event string, label string, stream string) int {
	if l.state == nil {
		return 1
	}
	key := strings.Join([]string{strings.TrimSpace(event), strings.TrimSpace(label), strings.TrimSpace(stream)}, ":")
	l.state.payloadSeq[key]++
	return l.state.payloadSeq[key]
}

func payloadFileName(label string, event string, stream string, seq int) string {
	sanitizedLabel := planSlug(label)
	if sanitizedLabel == "" {
		sanitizedLabel = event
	}
	if event == "response" {
		return fmt.Sprintf("%s.%d.json", sanitizedLabel, seq)
	}
	stream = planSlug(stream)
	if stream == "" {
		stream = "payload"
	}
	return fmt.Sprintf("%s.%d.%s.txt", sanitizedLabel, seq, stream)
}

func askLogFields(attrs ...any) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	fields := make(map[string]any, len(attrs)/2)
	for i := 0; i+1 < len(attrs); i += 2 {
		key := strings.TrimSpace(fmt.Sprint(attrs[i]))
		if key == "" {
			continue
		}
		fields[key] = attrs[i+1]
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func askValueOrDash(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "-"
	}
	return trimmed
}

func (l askLogger) flush() {
	flushOutput(l.writer)
}
