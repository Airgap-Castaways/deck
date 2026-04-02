package askretrieve

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askintent"
	"github.com/Airgap-Castaways/deck/internal/askknowledge"
	"github.com/Airgap-Castaways/deck/internal/askstate"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

type factGroup struct {
	chunks []Chunk
	limit  int
}

type WorkspaceSummary struct {
	Root            string
	HasWorkflowTree bool
	HasPrepare      bool
	HasApply        bool
	Files           []WorkspaceFile
}

type WorkspaceFile struct {
	Path    string
	Content string
}

type Chunk struct {
	ID       string
	Source   string
	Label    string
	Topic    askcontext.Topic
	Content  string
	Score    int
	Evidence *EvidenceSummary
}

type EvidenceSummary struct {
	Provider       string   `json:"provider,omitempty"`
	SourceURL      string   `json:"sourceUrl,omitempty"`
	SourceID       string   `json:"sourceId,omitempty"`
	Domain         string   `json:"domain,omitempty"`
	DomainCategory string   `json:"domainCategory,omitempty"`
	Title          string   `json:"title,omitempty"`
	Excerpt        string   `json:"excerpt,omitempty"`
	Freshness      string   `json:"freshness,omitempty"`
	Official       bool     `json:"official,omitempty"`
	TrustLevel     string   `json:"trustLevel,omitempty"`
	VersionSupport string   `json:"versionSupport,omitempty"`
	ArtifactKinds  []string `json:"artifactKinds,omitempty"`
	InstallHints   []string `json:"installHints,omitempty"`
	OfflineHints   []string `json:"offlineHints,omitempty"`
}

type RetrievalResult struct {
	Chunks   []Chunk
	Dropped  []string
	MaxBytes int
}

func InspectWorkspace(root string) (WorkspaceSummary, error) {
	resolvedRoot, err := filepath.Abs(strings.TrimSpace(root))
	if err != nil {
		return WorkspaceSummary{}, fmt.Errorf("resolve workspace root: %w", err)
	}
	workflowRoot := filepath.Join(resolvedRoot, workspacepaths.WorkflowRootDir)
	preparePath := workspacepaths.CanonicalPrepareWorkflowPath(resolvedRoot)
	applyPath := workspacepaths.CanonicalApplyWorkflowPath(resolvedRoot)
	varsPath := workspacepaths.CanonicalVarsPath(resolvedRoot)
	out := WorkspaceSummary{Root: resolvedRoot}
	if info, err := os.Stat(workflowRoot); err == nil && info.IsDir() {
		out.HasWorkflowTree = true
	}
	out.HasPrepare = isFile(preparePath)
	out.HasApply = isFile(applyPath)

	for _, path := range []string{varsPath, preparePath, applyPath} {
		if !isFile(path) {
			continue
		}
		content, err := os.ReadFile(path) //nolint:gosec // Workspace-derived files only.
		if err != nil {
			return WorkspaceSummary{}, fmt.Errorf("read workspace file %s: %w", path, err)
		}
		out.Files = append(out.Files, WorkspaceFile{Path: toRel(resolvedRoot, path), Content: string(content)})
	}
	for _, dir := range []string{
		filepath.Join(workflowRoot, workspacepaths.WorkflowScenariosDir),
		filepath.Join(workflowRoot, workspacepaths.WorkflowComponentsDir),
	} {
		if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			lower := strings.ToLower(d.Name())
			if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
				return nil
			}
			content, readErr := os.ReadFile(path) //nolint:gosec // Workspace-derived files only.
			if readErr != nil {
				return fmt.Errorf("read workspace file %s: %w", path, readErr)
			}
			rel := toRel(resolvedRoot, path)
			if !containsWorkspacePath(out.Files, rel) {
				out.Files = append(out.Files, WorkspaceFile{Path: rel, Content: string(content)})
			}
			return nil
		}); err != nil && !os.IsNotExist(err) {
			return WorkspaceSummary{}, fmt.Errorf("walk workflow directory %s: %w", dir, err)
		}
	}
	sort.Slice(out.Files, func(i, j int) bool {
		return out.Files[i].Path < out.Files[j].Path
	})
	return out, nil
}

func Retrieve(route askintent.Route, prompt string, target askintent.Target, workspace WorkspaceSummary, state askstate.Context, external []Chunk) RetrievalResult {
	budget, maxChunks := routeBudget(route, prompt)
	lowerPrompt := strings.ToLower(strings.TrimSpace(prompt))
	groups := routeFactGroups(route, lowerPrompt, target, workspace, state, external)
	selected, dropped := selectFactGroups(lowerPrompt, groups, budget, maxChunks)
	return RetrievalResult{Chunks: selected, Dropped: dropped, MaxBytes: budget}
}

func routeFactGroups(route askintent.Route, lowerPrompt string, target askintent.Target, workspace WorkspaceSummary, state askstate.Context, external []Chunk) []factGroup {
	groups := make([]factGroup, 0, 6)
	info := factGroup{chunks: informationalFactChunks(route, lowerPrompt)}
	workspaceGroup := factGroup{chunks: workspaceFactChunks(route, lowerPrompt, target, workspace)}
	localFacts := factGroup{chunks: localFactsForRoute(route, lowerPrompt, target)}
	examples := factGroup{chunks: exampleReferenceChunks(route, lowerPrompt), limit: 2}
	stateGroup := factGroup{chunks: stateFactChunks(route, state), limit: 1}
	externalGroup := factGroup{chunks: externalFactChunks(external)}
	switch route {
	case askintent.RouteDraft, askintent.RouteRefine:
		groups = appendFactGroup(groups, workspaceGroup, localFacts, examples, externalGroup)
	default:
		groups = appendFactGroup(groups, info, externalGroup, workspaceGroup, localFacts, stateGroup)
	}
	return groups
}

func localFactsForRoute(route askintent.Route, lowerPrompt string, target askintent.Target) []Chunk {
	if !shouldIncludeLocalFacts(route, lowerPrompt, target) {
		return nil
	}
	return localFactChunks(route, lowerPrompt)
}

func appendFactGroup(groups []factGroup, items ...factGroup) []factGroup {
	for _, item := range items {
		if len(item.chunks) == 0 {
			continue
		}
		groups = append(groups, item)
	}
	return groups
}

func informationalFactChunks(route askintent.Route, prompt string) []Chunk {
	bundle := askknowledge.Current()
	switch route {
	case askintent.RouteQuestion:
		return []Chunk{{ID: "workflow-meta", Source: "askcontext", Label: "workflow-summary", Topic: askcontext.TopicWorkflowInvariants, Content: bundle.WorkflowPromptBlock(), Score: 50}}
	case askintent.RouteExplain:
		return []Chunk{
			{ID: "workflow-meta", Source: "askcontext", Label: "workflow-summary", Topic: askcontext.TopicWorkflowInvariants, Content: bundle.WorkflowPromptBlock(), Score: 50},
			{ID: "topology", Source: "askcontext", Label: "workspace-topology", Topic: askcontext.TopicWorkspaceTopology, Content: askcontext.WorkspaceTopologyBlock(), Score: 52},
		}
	case askintent.RouteReview:
		return []Chunk{
			{ID: "workflow-meta", Source: "askcontext", Label: "workflow-summary", Topic: askcontext.TopicWorkflowInvariants, Content: bundle.WorkflowPromptBlock(), Score: 50},
			{ID: "topology", Source: "askcontext", Label: "workspace-topology", Topic: askcontext.TopicWorkspaceTopology, Content: askcontext.WorkspaceTopologyBlock(), Score: 52},
			{ID: "role-guidance", Source: "askcontext", Label: "prepare-apply-guidance", Topic: askcontext.TopicPrepareApplyGuidance, Content: askcontext.RoleGuidanceBlock(), Score: roleGuidanceScore(prompt)},
		}
	default:
		return nil
	}
}

func workspaceFactChunks(route askintent.Route, lowerPrompt string, target askintent.Target, workspace WorkspaceSummary) []Chunk {
	if !shouldIncludeWorkspaceFacts(route, lowerPrompt, target) {
		return nil
	}
	related := relatedWorkspaceTargets(workspace, target)
	chunks := make([]Chunk, 0, len(workspace.Files))
	for _, file := range workspace.Files {
		if !workspaceFileAllowed(file.Path) {
			continue
		}
		score := 30
		if target.Path != "" && filepath.ToSlash(file.Path) == filepath.ToSlash(target.Path) {
			score += 50
		}
		if related[filepath.ToSlash(file.Path)] {
			score += 35
		}
		if target.Name != "" && strings.Contains(strings.ToLower(filepath.Base(file.Path)), strings.ToLower(target.Name)) {
			score += 25
		}
		if strings.Contains(lowerPrompt, strings.ToLower(filepath.Base(file.Path))) {
			score += 30
		}
		if strings.Contains(file.Path, "workflows/scenarios/") {
			score += 10
		}
		chunks = append(chunks, Chunk{
			ID:      "workspace-" + strings.ReplaceAll(file.Path, "/", "_"),
			Source:  "workspace",
			Label:   file.Path,
			Topic:   askcontext.Topic("workspace:" + filepath.ToSlash(file.Path)),
			Content: compressChunkContent(lowerPrompt, file.Path, file.Content, 3200),
			Score:   score,
		})
	}
	sort.SliceStable(chunks, func(i, j int) bool {
		if chunks[i].Score == chunks[j].Score {
			return chunks[i].ID < chunks[j].ID
		}
		return chunks[i].Score > chunks[j].Score
	})
	return chunks
}

func shouldIncludeLocalFacts(route askintent.Route, lowerPrompt string, target askintent.Target) bool {
	switch route {
	case askintent.RouteDraft, askintent.RouteRefine, askintent.RouteReview:
		return true
	case askintent.RouteExplain:
		if strings.HasPrefix(filepath.ToSlash(strings.TrimSpace(target.Path)), "internal/") {
			return true
		}
		for _, token := range []string{"internal/", "stepspec", "stepmeta", "askdraft", "askpolicy", "askrepair", "typed step", "builder selection", "source-of-truth"} {
			if strings.Contains(lowerPrompt, token) {
				return true
			}
		}
	}
	return false
}

func shouldIncludeWorkspaceFacts(route askintent.Route, lowerPrompt string, target askintent.Target) bool {
	if route == askintent.RouteDraft || route == askintent.RouteRefine {
		return true
	}
	cleanTarget := filepath.ToSlash(strings.TrimSpace(target.Path))
	if strings.HasPrefix(cleanTarget, "workflows/") {
		return true
	}
	for _, token := range []string{"workflows/", "scenario", "component", "vars.yaml", "prepare.yaml", "apply.yaml"} {
		if strings.Contains(lowerPrompt, token) {
			return true
		}
	}
	return false
}

func stateFactChunks(route askintent.Route, state askstate.Context) []Chunk {
	if route == askintent.RouteDraft || route == askintent.RouteRefine || strings.TrimSpace(state.LastLint) == "" {
		return nil
	}
	return []Chunk{{
		ID:      "state-last-lint",
		Source:  "state",
		Label:   "last-lint",
		Topic:   askcontext.Topic("state:last-lint"),
		Content: state.LastLint,
		Score:   20,
	}}
}

func externalFactChunks(external []Chunk) []Chunk {
	chunks := make([]Chunk, 0, len(external))
	for _, chunk := range external {
		if strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		chunks = append(chunks, chunk)
	}
	return chunks
}

func selectFactGroups(lowerPrompt string, groups []factGroup, budget int, maxChunks int) ([]Chunk, []string) {
	selected := make([]Chunk, 0, maxChunks)
	dropped := make([]string, 0)
	remaining := budget
	seenTopics := map[askcontext.Topic]bool{}
	for _, group := range groups {
		keptInGroup := 0
		for _, chunk := range group.chunks {
			if len(selected) >= maxChunks {
				dropped = append(dropped, chunk.ID)
				continue
			}
			topic := chunk.Topic
			if topic == "" {
				topic = askcontext.Topic("id:" + chunk.ID)
				chunk.Topic = topic
			}
			if seenTopics[topic] {
				continue
			}
			if group.limit > 0 && keptInGroup >= group.limit {
				dropped = append(dropped, chunk.ID)
				continue
			}
			content := chunk.Content
			size := len(content)
			if size > remaining && shouldCompressChunk(chunk.Label, chunk.Content) {
				content = compressChunkContent(lowerPrompt, chunk.Label, chunk.Content, remaining)
				size = len(content)
			}
			if size > remaining {
				dropped = append(dropped, chunk.ID)
				continue
			}
			chunk.Content = content
			selected = append(selected, chunk)
			seenTopics[topic] = true
			remaining -= size
			keptInGroup++
		}
	}
	return selected, dropped
}

func roleGuidanceScore(prompt string) int {
	score := 40
	if strings.Contains(strings.ToLower(prompt), "prepare") || strings.Contains(strings.ToLower(prompt), "apply") {
		score += 20
	}
	return score
}

func containsWorkspacePath(files []WorkspaceFile, path string) bool {
	for _, file := range files {
		if filepath.ToSlash(file.Path) == filepath.ToSlash(path) {
			return true
		}
	}
	return false
}

func relatedWorkspaceTargets(workspace WorkspaceSummary, target askintent.Target) map[string]bool {
	if target.Path == "" {
		return nil
	}
	targetPath := filepath.ToSlash(target.Path)
	fileByPath := make(map[string]WorkspaceFile, len(workspace.Files))
	for _, file := range workspace.Files {
		fileByPath[filepath.ToSlash(file.Path)] = file
	}
	current, ok := fileByPath[targetPath]
	if !ok {
		return nil
	}
	related := map[string]bool{targetPath: true}
	if !strings.HasPrefix(targetPath, "workflows/scenarios/") {
		return related
	}
	for _, importPath := range importPaths(current.Content) {
		resolved := filepath.ToSlash(filepath.Join("workflows/components", importPath))
		if _, exists := fileByPath[resolved]; exists {
			related[resolved] = true
		}
	}
	return related
}

func importPaths(content string) []string {
	paths := make([]string, 0)
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- path:") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- path:"))
		value = strings.Trim(value, `"'`)
		if value != "" {
			paths = append(paths, filepath.ToSlash(value))
		}
	}
	return paths
}

func BuildChunkText(retrieval RetrievalResult) string {
	return BuildChunkTextWithoutTopics(retrieval)
}

func BuildChunkTextWithoutTopics(retrieval RetrievalResult, excluded ...askcontext.Topic) string {
	excludedSet := map[askcontext.Topic]bool{}
	for _, topic := range excluded {
		excludedSet[topic] = true
	}
	b := &strings.Builder{}
	appendChunkGroup := func(title string, chunks []Chunk) {
		if len(chunks) == 0 {
			return
		}
		b.WriteString(title)
		b.WriteString("\n")
		for _, chunk := range chunks {
			b.WriteString("[chunk:")
			b.WriteString(chunk.ID)
			b.WriteString(",source:")
			b.WriteString(chunk.Source)
			b.WriteString(",label:")
			b.WriteString(chunk.Label)
			b.WriteString("]\n")
			b.WriteString(chunk.Content)
			if !strings.HasSuffix(chunk.Content, "\n") {
				b.WriteString("\n")
			}
		}
	}
	localFactChunks := make([]Chunk, 0)
	externalChunks := make([]Chunk, 0)
	otherChunks := make([]Chunk, 0)
	for _, chunk := range retrieval.Chunks {
		if excludedSet[chunk.Topic] {
			continue
		}
		switch chunk.Source {
		case "local-facts":
			localFactChunks = append(localFactChunks, chunk)
		case "mcp", "external-evidence":
			externalChunks = append(externalChunks, chunk)
		default:
			otherChunks = append(otherChunks, chunk)
		}
	}
	appendChunkGroup("Local facts:", localFactChunks)
	appendChunkGroup("External evidence:", externalChunks)
	appendChunkGroup("Retrieved context:", otherChunks)
	return b.String()
}

func routeBudget(route askintent.Route, prompt string) (maxBytes int, maxChunks int) {
	complex := isComplexAuthoringPrompt(route, prompt)
	switch route {
	case askintent.RouteDraft, askintent.RouteRefine:
		if complex {
			return 20000, 14
		}
		return 12000, 10
	case askintent.RouteReview, askintent.RouteExplain:
		return 8000, 8
	default:
		return 4000, 6
	}
}

func isComplexAuthoringPrompt(route askintent.Route, prompt string) bool {
	if route != askintent.RouteDraft && route != askintent.RouteRefine {
		return false
	}
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	tokens := []string{"prepare and apply", "prepare", "apply", "air-gapped", "airgapped", "kubeadm", "cluster", "multi-node", "single-node", "worker", "workers", "join", "control-plane", "control plane"}
	hits := 0
	for _, token := range tokens {
		if strings.Contains(prompt, token) {
			hits++
		}
	}
	return hits >= 3
}

func exampleReferenceChunks(route askintent.Route, lowerPrompt string) []Chunk {
	if route != askintent.RouteDraft && route != askintent.RouteRefine {
		return nil
	}
	root := repoRootFallback()
	if root == "" {
		return nil
	}
	candidates := []string{
		filepath.Join(root, "docs", "user-guide", "examples"),
		filepath.Join(root, "test", "workflows"),
	}
	out := make([]Chunk, 0, 8)
	for _, dir := range candidates {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			lowerName := strings.ToLower(d.Name())
			if !strings.HasSuffix(lowerName, ".yaml") && !strings.HasSuffix(lowerName, ".yml") {
				return nil
			}
			content, readErr := os.ReadFile(path) //nolint:gosec // repository-owned examples only.
			if readErr != nil {
				return readErr
			}
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				rel = path
			}
			cleanRel := filepath.ToSlash(rel)
			if !exampleChunkAllowed(cleanRel, string(content)) {
				return nil
			}
			score := exampleChunkScore(lowerPrompt, cleanRel, string(content))
			if score < 55 {
				return nil
			}
			out = append(out, Chunk{
				ID:      "example-" + strings.ReplaceAll(cleanRel, "/", "_"),
				Source:  "example",
				Label:   cleanRel,
				Topic:   askcontext.Topic("example:" + cleanRel),
				Content: exampleChunkContent(cleanRel, compressChunkContent(lowerPrompt, cleanRel, string(content), 3600)),
				Score:   score,
			})
			return nil
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].ID < out[j].ID
		}
		return out[i].Score > out[j].Score
	})
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func exampleChunkScore(prompt string, path string, content string) int {
	score := 20
	text := strings.ToLower(path + "\n" + content)
	for _, token := range []string{"air-gapped", "airgapped", "offline", "kubeadm", "cluster", "worker", "join", "control-plane", "control plane", "containerd", "repo", "package", "image", "prepare", "apply", "artifact", "handoff", "publish", "kubeconfig"} {
		if strings.Contains(prompt, token) && strings.Contains(text, token) {
			score += 12
		}
	}
	if strings.Contains(path, "docs/user-guide/examples/") {
		score += 4
	}
	if strings.Contains(path, "test/workflows/") {
		score += 18
	}
	if strings.Contains(path, "scenarios/") {
		score += 8
	}
	if strings.Contains(path, "upgrade") && !strings.Contains(prompt, "upgrade") {
		score -= 20
	}
	if strings.Contains(path, "worker-join") && (strings.Contains(prompt, "worker") || strings.Contains(prompt, "join")) {
		score += 12
	}
	if strings.Contains(path, "bootstrap") && (strings.Contains(prompt, "control-plane") || strings.Contains(prompt, "bootstrap") || strings.Contains(prompt, "kubeadm")) {
		score += 10
	}
	if strings.Contains(path, "kubeadm") && strings.Contains(prompt, "kubeadm") {
		score += 10
	}
	if strings.Contains(text, "apiVersion: deck/") {
		score -= 18
	}
	if strings.Contains(text, "kind: swap") || strings.Contains(text, "kind: sysctl") {
		score -= 6
	}
	if strings.Contains(text, "version: v1alpha1") {
		score += 6
	}
	if strings.Contains(text, "kind: initkubeadm") || strings.Contains(text, "kind: joinkubeadm") {
		score += 8
	}
	if strings.Contains(text, "outputjoinfile") || strings.Contains(text, "joinfile") {
		score += 8
	}
	return score
}

func exampleChunkAllowed(path string, content string) bool {
	text := strings.ToLower(path + "\n" + content)
	if strings.Contains(path, "docs/user-guide/examples/") && strings.Contains(text, "apiversion: deck/") {
		return false
	}
	return true
}

func exampleChunkContent(path string, content string) string {
	b := &strings.Builder{}
	b.WriteString("Reference example:\n")
	b.WriteString("- path: ")
	b.WriteString(path)
	b.WriteString("\n")
	b.WriteString(content)
	if !strings.HasSuffix(content, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func compressChunkContent(prompt string, label string, content string, maxBytes int) string {
	content = strings.TrimSpace(content)
	if content == "" || maxBytes <= 0 {
		return content
	}
	if !shouldCompressChunk(label, content) {
		return content
	}
	if len(content) <= maxBytes {
		return content
	}
	lines := strings.Split(content, "\n")
	keywords := requestKeywords(prompt, label)
	selected := selectRelevantLineWindows(lines, keywords, 2)
	compressed := renderSelectedLines(lines, selected)
	compressed = strings.TrimSpace(compressed)
	if compressed == "" {
		compressed = strings.Join(lines[:min(80, len(lines))], "\n")
	}
	if len(compressed) <= maxBytes {
		return compressed
	}
	if maxBytes > 64 {
		trimmed := compressed[:maxBytes-16]
		trimmed = strings.TrimRightFunc(trimmed, unicode.IsSpace)
		return trimmed + "\n...\n"
	}
	return compressed[:maxBytes]
}

func shouldCompressChunk(label string, content string) bool {
	lowerLabel := strings.ToLower(strings.TrimSpace(label))
	if strings.HasSuffix(lowerLabel, ".yaml") || strings.HasSuffix(lowerLabel, ".yml") {
		return false
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return true
	}
	if looksLikeStructuredYAML(trimmed) {
		return false
	}
	return true
}

func looksLikeStructuredYAML(content string) bool {
	if strings.Contains(content, "version: v1alpha1") {
		return true
	}
	for _, token := range []string{"\nphases:\n", "\nsteps:\n", "\nvars:\n", "\nimports:\n", "\n  - name:", "\n  - id:", "\nkind: ", "\nspec:\n"} {
		if strings.Contains(content, token) {
			return true
		}
	}
	return false
}

func requestKeywords(prompt string, label string) []string {
	parts := strings.Fields(strings.ToLower(prompt + " " + label))
	keywords := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "`\"'.,:;()[]{}")
		if len(part) < 4 {
			continue
		}
		switch part {
		case "with", "from", "that", "this", "workflow", "create", "using", "where", "possible", "offline", "cluster":
			continue
		}
		keywords = append(keywords, part)
	}
	for _, item := range []string{"kubeadm", "join", "worker", "control-plane", "prepare", "apply", "artifact", "image", "package", "handoff", "checkcluster", "initkubeadm", "joinkubeadm"} {
		if strings.Contains(strings.ToLower(prompt), item) || strings.Contains(strings.ToLower(label), item) {
			keywords = append(keywords, item)
		}
	}
	return dedupeStrings(keywords)
}

func selectRelevantLineWindows(lines []string, keywords []string, radius int) []int {
	selected := make([]int, 0, len(lines))
	seen := map[int]bool{}
	matchCount := 0
	for i, line := range lines {
		lower := strings.ToLower(line)
		matched := false
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		matchCount++
		start := max(0, i-radius)
		end := min(len(lines)-1, i+radius)
		for idx := start; idx <= end; idx++ {
			if !seen[idx] {
				seen[idx] = true
				selected = append(selected, idx)
			}
		}
		if matchCount >= 24 {
			break
		}
	}
	if len(selected) == 0 {
		limit := min(80, len(lines))
		for i := 0; i < limit; i++ {
			selected = append(selected, i)
		}
	}
	sort.Ints(selected)
	return selected
}

func renderSelectedLines(lines []string, selected []int) string {
	if len(selected) == 0 {
		return ""
	}
	b := &strings.Builder{}
	prev := -2
	for _, idx := range selected {
		if idx < 0 || idx >= len(lines) {
			continue
		}
		if prev >= 0 && idx > prev+1 {
			b.WriteString("...\n")
		}
		b.WriteString(lines[idx])
		b.WriteString("\n")
		prev = idx
	}
	return b.String()
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func dedupeStrings(items []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func repoRootFallback() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if info, err := os.Stat(filepath.Join(root, "go.mod")); err == nil && !info.IsDir() {
		return root
	}
	return ""
}

func workspaceFileAllowed(path string) bool {
	clean := filepath.ToSlash(strings.ToLower(strings.TrimSpace(path)))
	if clean == "" {
		return false
	}
	if strings.Contains(clean, "..") {
		return false
	}
	if strings.HasSuffix(clean, ".env") || strings.Contains(clean, "/.env") {
		return false
	}
	if strings.HasPrefix(clean, "outputs/") || strings.HasPrefix(clean, ".git/") || strings.HasPrefix(clean, "bin/") || strings.HasPrefix(clean, "test/artifacts/") {
		return false
	}
	if askcontext.AllowedGeneratedPath(clean) {
		return true
	}
	return false
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func toRel(root string, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
