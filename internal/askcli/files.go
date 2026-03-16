package askcli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/taedi90/deck/internal/askcontract"
	"github.com/taedi90/deck/internal/askretrieve"
	"github.com/taedi90/deck/internal/askreview"
	"github.com/taedi90/deck/internal/fsutil"
	"github.com/taedi90/deck/internal/validate"
	"github.com/taedi90/deck/internal/workspacepaths"
)

func validateGeneratedPath(path string) error { /* split helper below */
	clean := filepath.ToSlash(strings.TrimSpace(path))
	if clean == "" {
		return fmt.Errorf("generated file path is empty")
	}
	allowed := strings.HasPrefix(clean, "workflows/scenarios/") || strings.HasPrefix(clean, "workflows/components/") || clean == "workflows/vars.yaml"
	if !allowed {
		return fmt.Errorf("generated file path is not allowed: %s", clean)
	}
	if strings.Contains(clean, "..") {
		return fmt.Errorf("generated file path escapes workspace: %s", clean)
	}
	return nil
}

func validateGeneratedFile(root string, file askcontract.GeneratedFile) error {
	if err := validateGeneratedPath(file.Path); err != nil {
		return err
	}
	target, err := fsutil.ResolveUnder(root, strings.Split(filepath.ToSlash(file.Path), "/")...)
	if err != nil {
		return err
	}
	if strings.HasSuffix(file.Path, ".yaml") || strings.HasSuffix(file.Path, ".yml") {
		if isVarsPath(file.Path) {
			var vars map[string]any
			if err := yaml.Unmarshal([]byte(file.Content), &vars); err != nil {
				return fmt.Errorf("%s: parse vars yaml: %w", file.Path, err)
			}
			return nil
		}
		if err := validate.Bytes(target, []byte(file.Content)); err != nil {
			return err
		}
	}
	return nil
}

func writeFiles(root string, files []askcontract.GeneratedFile) error {
	if err := ensureScaffold(root); err != nil {
		return err
	}
	for _, file := range files {
		if err := validateGeneratedPath(file.Path); err != nil {
			return err
		}
		target, err := fsutil.ResolveUnder(root, strings.Split(filepath.ToSlash(file.Path), "/")...)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return fmt.Errorf("create ask target directory: %w", err)
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o600); err != nil {
			return fmt.Errorf("write %s: %w", file.Path, err)
		}
	}
	return nil
}

func stageWorkspace(root string, files []askcontract.GeneratedFile) (string, error) {
	tempRoot, err := os.MkdirTemp("", "deck-ask-workspace-")
	if err != nil {
		return "", fmt.Errorf("create ask staging workspace: %w", err)
	}
	workflowRoot := filepath.Join(root, workspacepaths.WorkflowRootDir)
	if info, err := os.Stat(workflowRoot); err == nil && info.IsDir() {
		if err := copyTree(workflowRoot, filepath.Join(tempRoot, workspacepaths.WorkflowRootDir)); err != nil {
			return "", err
		}
	}
	for _, file := range files {
		if err := validateGeneratedPath(file.Path); err != nil {
			return "", err
		}
		target, err := fsutil.ResolveUnder(tempRoot, strings.Split(filepath.ToSlash(file.Path), "/")...)
		if err != nil {
			return "", err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return "", fmt.Errorf("create ask staging directory: %w", err)
		}
		if err := os.WriteFile(target, []byte(file.Content), 0o600); err != nil {
			return "", fmt.Errorf("write ask staging file: %w", err)
		}
	}
	return tempRoot, nil
}

func scenarioPaths(root string, candidatePaths []string) []string {
	paths := make([]string, 0)
	seen := map[string]bool{}
	for _, rel := range candidatePaths {
		clean := filepath.ToSlash(strings.TrimSpace(rel))
		if !strings.HasPrefix(clean, "workflows/scenarios/") {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(clean))
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	if len(paths) > 0 {
		sort.Strings(paths)
		return paths
	}
	scenarioDir := filepath.Join(root, workspacepaths.WorkflowRootDir, workspacepaths.WorkflowScenariosDir)
	entries, err := os.ReadDir(scenarioDir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			paths = append(paths, filepath.Join(scenarioDir, entry.Name()))
		}
	}
	sort.Strings(paths)
	return paths
}

func localFindings(files []askcontract.GeneratedFile) []askreview.Finding {
	content := make(map[string]string, len(files))
	for _, file := range files {
		content[file.Path] = file.Content
	}
	return askreview.Candidate(content)
}

func ensureScaffold(root string) error {
	for _, dir := range []string{
		filepath.Join(root, ".deck"),
		filepath.Join(root, workspacepaths.WorkflowRootDir, workspacepaths.WorkflowScenariosDir),
		filepath.Join(root, workspacepaths.WorkflowRootDir, workspacepaths.WorkflowComponentsDir),
		filepath.Join(root, workspacepaths.PreparedDirRel, "files"),
		filepath.Join(root, workspacepaths.PreparedDirRel, "images"),
		filepath.Join(root, workspacepaths.PreparedDirRel, "packages"),
	} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("create workspace scaffold: %w", err)
		}
	}
	defaults := map[string]string{
		filepath.Join(root, ".gitignore"):                                       strings.Join([]string{"/.deck/", "/deck", "/outputs/", "*.tar", ""}, "\n"),
		filepath.Join(root, ".deckignore"):                                      strings.Join([]string{".git/", ".gitignore", ".deckignore", "/*.tar", ""}, "\n"),
		filepath.Join(root, workspacepaths.PreparedDirRel, "files", ".keep"):    "",
		filepath.Join(root, workspacepaths.PreparedDirRel, "images", ".keep"):   "",
		filepath.Join(root, workspacepaths.PreparedDirRel, "packages", ".keep"): "",
	}
	for path, content := range defaults {
		if _, err := os.Stat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("stat scaffold file %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return fmt.Errorf("write scaffold file %s: %w", path, err)
		}
	}
	return nil
}

func copyTree(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		raw, err := os.ReadFile(path) //nolint:gosec
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		return os.WriteFile(target, raw, 0o600) //nolint:gosec
	})
}

func loadRequestText(root string, prompt string, fromPath string) (string, error) {
	prompt = strings.TrimSpace(prompt)
	fromPath = strings.TrimSpace(fromPath)
	if fromPath == "" {
		return prompt, nil
	}
	candidate := fromPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", fmt.Errorf("resolve ask request file: %w", err)
	}
	resolved, err := fsutil.ResolveUnder(root, strings.Split(filepath.ToSlash(rel), "/")...)
	if err != nil {
		return "", fmt.Errorf("resolve ask request file: %w", err)
	}
	raw, err := os.ReadFile(resolved) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("read ask request file: %w", err)
	}
	fromText := strings.TrimSpace(string(raw))
	if prompt == "" {
		return fromText, nil
	}
	return prompt + "\n\nAttached request details:\n" + fromText, nil
}

func renderUserCommand(opts Options) string {
	parts := []string{"deck", "ask"}
	if opts.Write {
		parts = append(parts, "--write")
	}
	if opts.Review {
		parts = append(parts, "--review")
	}
	if opts.MaxIterations > 0 {
		parts = append(parts, "--max-iterations", fmt.Sprintf("%d", opts.MaxIterations))
	}
	if strings.TrimSpace(opts.FromPath) != "" {
		parts = append(parts, "--from", strings.TrimSpace(opts.FromPath))
	}
	if strings.TrimSpace(opts.Provider) != "" {
		parts = append(parts, "--provider", strings.TrimSpace(opts.Provider))
	}
	if strings.TrimSpace(opts.Model) != "" {
		parts = append(parts, "--model", strings.TrimSpace(opts.Model))
	}
	if strings.TrimSpace(opts.Endpoint) != "" {
		parts = append(parts, "--endpoint", strings.TrimSpace(opts.Endpoint))
	}
	if strings.TrimSpace(opts.Prompt) != "" {
		parts = append(parts, strconv.Quote(strings.TrimSpace(opts.Prompt)))
	}
	return strings.Join(parts, " ")
}

func isVarsPath(path string) bool {
	return filepath.ToSlash(strings.TrimSpace(path)) == "workflows/vars.yaml"
}

func filePaths(files []askcontract.GeneratedFile) []string {
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.Path)
	}
	sort.Strings(paths)
	return paths
}

func chunkIDs(chunks []askretrieve.Chunk) []string {
	ids := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		ids = append(ids, chunk.ID)
	}
	return ids
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
