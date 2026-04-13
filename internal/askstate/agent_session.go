package askstate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

type AgentToolEvent struct {
	Turn      int       `json:"turn"`
	Name      string    `json:"name"`
	Path      string    `json:"path,omitempty"`
	Paths     []string  `json:"paths,omitempty"`
	Query     string    `json:"query,omitempty"`
	Include   []string  `json:"include,omitempty"`
	Intent    string    `json:"intent,omitempty"`
	OK        bool      `json:"ok"`
	Summary   string    `json:"summary,omitempty"`
	Result    string    `json:"result,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

type AgentSession struct {
	Prompt               string                      `json:"prompt,omitempty"`
	Route                string                      `json:"route,omitempty"`
	ApprovedPaths        []string                    `json:"approvedPaths,omitempty"`
	ToolEvents           []AgentToolEvent            `json:"toolEvents,omitempty"`
	CandidateFiles       []askcontract.GeneratedFile `json:"candidateFiles,omitempty"`
	VerifierSummary      string                      `json:"verifierSummary,omitempty"`
	VerificationFailures int                         `json:"verificationFailures,omitempty"`
	Turns                int                         `json:"turns,omitempty"`
	TerminationReason    string                      `json:"terminationReason,omitempty"`
	UpdatedAt            time.Time                   `json:"updatedAt,omitempty"`
}

func SaveAgentSession(root string, session AgentSession) (string, error) {
	dir, err := Dir(root)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create ask state directory: %w", err)
	}
	session.UpdatedAt = time.Now().UTC()
	raw, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal agent session: %w", err)
	}
	raw = append(raw, '\n')
	path := filepath.Join(dir, "last-agent-session.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", fmt.Errorf("write agent session: %w", err)
	}
	return filepath.ToSlash(filepath.Join(dirName, "last-agent-session.json")), nil
}

func LoadAgentSession(root string) (AgentSession, error) {
	dir, err := Dir(root)
	if err != nil {
		return AgentSession{}, err
	}
	//nolint:gosec // Path stays under the current workspace .deck/ask directory.
	raw, err := os.ReadFile(filepath.Join(dir, "last-agent-session.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return AgentSession{}, nil
		}
		return AgentSession{}, fmt.Errorf("read agent session: %w", err)
	}
	var state AgentSession
	if err := json.Unmarshal(raw, &state); err != nil {
		return AgentSession{}, fmt.Errorf("parse agent session: %w", err)
	}
	return state, nil
}
