package askauthoring

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

type Facts struct {
	Connectivity       string
	RequestedMode      string
	NeedsPrepare       bool
	ArtifactKinds      []string
	Topology           string
	NodeCount          int
	ControlPlaneCount  int
	WorkerCount        int
	Capabilities       []string
	Intents            []string
	Ambiguities        []string
	Clarifications     []askcontract.PlanClarification
	MultiRoleRequested bool
}

func InferFacts(prompt string, artifactKinds []string, connectivity string) Facts {
	lower := strings.ToLower(strings.TrimSpace(prompt))
	facts := Facts{
		Connectivity:  strings.TrimSpace(connectivity),
		RequestedMode: requestedMode(lower),
		NeedsPrepare:  len(artifactKinds) > 0 || strings.Contains(lower, "prepare"),
		ArtifactKinds: append([]string(nil), artifactKinds...),
	}
	nodeCount, nodeCountKnown := detectNodeCount(lower)
	cpCount, workerCount, explicitRoleModel := detectRoleCounts(lower, nodeCount)
	hasHA := hasAny(lower, "high availability", "high-availability") || containsWholeToken(lower, "ha")
	hasSingle := hasAny(lower, "single-node", "single node")
	hasMulti := hasAny(lower, "multi-node", "multi node") || nodeCount > 1
	hasCluster := mentionsClusterWorkflow(lower)
	hasJoinWorkflow := mentionsJoinWorkflow(lower)
	verificationOnly := isVerificationOnlyClusterRequest(lower)

	if hasSingle {
		facts.Topology = "single-node"
		if nodeCount == 0 {
			nodeCount = 1
		}
	}
	if hasHA {
		facts.Topology = "ha"
	}
	if facts.Topology == "" && hasMulti {
		facts.Topology = "multi-node"
	}
	if facts.Topology == "" {
		facts.Topology = "unspecified"
	}

	if strings.Contains(lower, "kubeadm") {
		facts.Intents = append(facts.Intents, "kubeadm")
		facts.Capabilities = append(facts.Capabilities, "kubeadm-bootstrap", "cluster-verification")
	}
	if mentionsClusterVerification(lower) {
		facts.Intents = append(facts.Intents, "cluster-verification")
		facts.Capabilities = append(facts.Capabilities, "cluster-verification")
	}
	if facts.NeedsPrepare {
		facts.Capabilities = append(facts.Capabilities, "prepare-artifacts")
	}
	for _, kind := range artifactKinds {
		switch strings.TrimSpace(strings.ToLower(kind)) {
		case "package":
			facts.Capabilities = append(facts.Capabilities, "package-staging")
		case "image":
			facts.Capabilities = append(facts.Capabilities, "image-staging")
		case "repository-mirror":
			facts.Capabilities = append(facts.Capabilities, "repository-setup")
		}
	}
	if explicitRoleModel || hasAny(lower, "worker", "workers") || hasJoinWorkflow || facts.Topology == "multi-node" || facts.Topology == "ha" {
		facts.Intents = append(facts.Intents, "join")
		facts.Capabilities = append(facts.Capabilities, "kubeadm-join")
		facts.MultiRoleRequested = true
	}
	if facts.Topology == "single-node" {
		facts.Intents = append(facts.Intents, "single-node")
	}
	if facts.Topology == "multi-node" {
		facts.Intents = append(facts.Intents, "multi-node")
	}
	if facts.Topology == "ha" {
		facts.Intents = append(facts.Intents, "ha")
	}

	if nodeCount > 0 {
		facts.NodeCount = nodeCount
		facts.Intents = append(facts.Intents, fmt.Sprintf("node-count:%d", nodeCount))
	}
	if cpCount > 0 {
		facts.ControlPlaneCount = cpCount
	}
	if workerCount > 0 {
		facts.WorkerCount = workerCount
	}

	if facts.Topology == "unspecified" && hasCluster {
		facts.Ambiguities = append(facts.Ambiguities, "cluster-topology")
	}
	if hasCluster && !contains(facts.Intents, "kubeadm") && !verificationOnly && clusterImplementationLikelyRelevant(lower, facts, hasJoinWorkflow) {
		facts.Ambiguities = append(facts.Ambiguities, "cluster-implementation")
	}
	if (facts.Topology == "multi-node" || facts.Topology == "ha" || facts.NodeCount > 1) && !explicitRoleModel {
		facts.Ambiguities = append(facts.Ambiguities, "role-model")
	}
	if (facts.Topology == "multi-node" || facts.Topology == "ha") && !nodeCountKnown && facts.NodeCount == 0 {
		facts.Ambiguities = append(facts.Ambiguities, "node-count")
	}
	if facts.NodeCount == 0 && cpCount > 0 && workerCount > 0 {
		facts.NodeCount = cpCount + workerCount
	}
	if facts.NodeCount == 0 && facts.Topology == "single-node" {
		facts.NodeCount = 1
	}
	if facts.NodeCount > 0 && facts.Topology == "unspecified" && facts.NodeCount > 1 {
		facts.Topology = "multi-node"
	}
	if facts.ControlPlaneCount == 0 && facts.Topology == "single-node" && strings.Contains(lower, "kubeadm") {
		facts.ControlPlaneCount = 1
	}
	if facts.ControlPlaneCount == 0 && facts.Topology == "ha" && facts.NodeCount >= 3 {
		facts.ControlPlaneCount = facts.NodeCount
	}
	facts.Intents = dedupe(facts.Intents)
	facts.Capabilities = dedupe(facts.Capabilities)
	facts.Ambiguities = dedupe(facts.Ambiguities)
	facts.Clarifications = buildClarifications(facts)
	return facts
}

func requestedMode(lower string) string {
	mentionsPrepare := strings.Contains(lower, "prepare workflow") || strings.Contains(lower, "prepare-only") || strings.Contains(lower, "prepare only")
	mentionsApply := strings.Contains(lower, "apply workflow") || strings.Contains(lower, "apply-only") || strings.Contains(lower, "apply only") || strings.Contains(lower, "scenario workflow")
	switch {
	case mentionsPrepare && !mentionsApply:
		return "prepare-only"
	case mentionsApply && !mentionsPrepare:
		return "apply-only"
	default:
		return "workspace"
	}
}

func detectNodeCount(lower string) (int, bool) {
	replacements := map[string]string{
		"one-node":   "1-node",
		"two-node":   "2-node",
		"three-node": "3-node",
		"four-node":  "4-node",
		"five-node":  "5-node",
		"one node":   "1 node",
		"two node":   "2 node",
		"three node": "3 node",
		"four node":  "4 node",
		"five node":  "5 node",
	}
	for old, newValue := range replacements {
		lower = strings.ReplaceAll(lower, old, newValue)
	}
	for _, token := range []string{"-node", " node", " nodes"} {
		for _, field := range strings.FieldsFunc(lower, func(r rune) bool { return r < '0' || r > '9' }) {
			if field == "" {
				continue
			}
			if n, err := strconv.Atoi(field); err == nil && n > 0 && strings.Contains(lower, field+token) {
				return n, true
			}
		}
	}
	for _, field := range strings.FieldsFunc(lower, func(r rune) bool { return r < '0' || r > '9' }) {
		if field == "" {
			continue
		}
		if n, err := strconv.Atoi(field); err == nil && n > 0 && strings.Contains(lower, field+" 노드") {
			return n, true
		}
	}
	return 0, false
}

func detectRoleCounts(lower string, nodeCount int) (int, int, bool) {
	cp := extractCountNear(lower, []string{"control-plane", "control plane", "control-planes", "control planes", "컨트롤플레인"})
	workers := extractCountNear(lower, []string{"worker", "workers", "워커"})
	if cp == 0 && workers == 0 {
		return 0, 0, false
	}
	if nodeCount > 0 && cp == 0 && workers > 0 {
		cp = nodeCount - workers
	}
	if nodeCount > 0 && workers == 0 && cp > 0 && cp < nodeCount {
		workers = nodeCount - cp
	}
	return cp, workers, true
}

func extractCountNear(lower string, labels []string) int {
	replaced := lower
	words := map[string]string{"one": "1", "two": "2", "three": "3", "four": "4", "five": "5"}
	for old, newValue := range words {
		replaced = strings.ReplaceAll(replaced, old, newValue)
	}
	for _, label := range labels {
		for _, form := range []string{"+ " + label, " " + label, label + " +", label + ","} {
			_ = form
		}
		idx := strings.Index(replaced, label)
		if idx < 0 {
			continue
		}
		prefix := strings.TrimSpace(replaced[:idx])
		fields := strings.Fields(prefix)
		if len(fields) == 0 {
			continue
		}
		last := strings.Trim(fields[len(fields)-1], "+,()")
		if n, err := strconv.Atoi(last); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func buildClarifications(f Facts) []askcontract.PlanClarification {
	items := []askcontract.PlanClarification{}
	if contains(f.Ambiguities, "cluster-topology") {
		items = append(items, askcontract.PlanClarification{
			ID:                 "topology.kind",
			Question:           "The request implies a cluster workflow, but the topology is not explicit. Should this be single-node, multi-node, or HA?",
			Kind:               "enum",
			Reason:             "Cluster topology changes file structure, execution contracts, and verification expectations.",
			Decision:           "topology",
			Options:            []string{"single-node", "multi-node", "ha"},
			RecommendedDefault: "single-node",
			BlocksGeneration:   true,
			Affects:            []string{"authoringBrief.topology", "executionModel.roleExecution", "executionModel.verification"},
		})
	}
	if contains(f.Ambiguities, "role-model") {
		defaultRole := "1cp-2workers"
		if f.Topology == "ha" {
			defaultRole = "3cp-ha"
		}
		items = append(items, askcontract.PlanClarification{
			ID:                 "topology.roleModel",
			Question:           "The request implies multiple nodes, but the role model is unclear. Which node role layout should the plan use?",
			Kind:               "enum",
			Reason:             "Multi-node plans need an explicit control-plane and worker layout before execution contracts can be assembled safely.",
			Decision:           "topology",
			Options:            []string{"1cp-2workers", "3cp-ha", "custom"},
			RecommendedDefault: defaultRole,
			BlocksGeneration:   true,
			Affects:            []string{"executionModel.roleExecution", "executionModel.sharedStateContracts", "executionModel.verification"},
		})
	}
	if contains(f.Ambiguities, "node-count") {
		items = append(items, askcontract.PlanClarification{
			ID:                 "topology.nodeCount",
			Question:           "The request needs a multi-node topology, but the total node count is not explicit. What node count should the plan use?",
			Kind:               "number",
			Reason:             "Node count affects role cardinality, final verification, and shared-state execution planning.",
			Decision:           "topology",
			RecommendedDefault: "3",
			BlocksGeneration:   true,
			Affects:            []string{"authoringBrief.nodeCount", "executionModel.verification.expectedNodeCount"},
		})
	}
	if contains(f.Ambiguities, "cluster-implementation") {
		items = append(items, askcontract.PlanClarification{
			ID:                 "cluster.implementation",
			Question:           "The request implies a Kubernetes cluster workflow, but the bootstrap implementation is not explicit. Which implementation should the plan use?",
			Kind:               "enum",
			Reason:             "Bootstrap implementation determines which typed capabilities and execution contracts are available.",
			Decision:           "coverage",
			Options:            []string{"kubeadm", "custom"},
			RecommendedDefault: "kubeadm",
			BlocksGeneration:   true,
			Affects:            []string{"authoringBrief.requiredCapabilities", "executionModel"},
		})
	}
	return items
}

func hasAny(lower string, items ...string) bool {
	for _, item := range items {
		if strings.Contains(lower, item) {
			return true
		}
	}
	return false
}

func mentionsClusterWorkflow(lower string) bool {
	return hasAny(lower,
		"cluster",
		"kubeadm",
		"control-plane",
		"control plane",
		"worker",
		"workers",
		"checkcluster",
		"check-cluster",
		"check cluster",
		"verify cluster",
		"cluster-verification",
		"cluster verification",
		"클러스터",
		"쿠버네티스",
		"클러스터링",
		"노드",
		"워커",
	)
}

func mentionsClusterVerification(lower string) bool {
	return hasAny(lower,
		"checkcluster",
		"check-cluster",
		"check cluster",
		"verify cluster",
		"cluster-verification",
		"cluster verification",
		"cluster health",
		"verifies the cluster",
		"validates the cluster",
	)
}

func mentionsJoinWorkflow(lower string) bool {
	if hasAny(lower, "kubeadm join", "join worker", "worker join", "worker-join", "조인") {
		return true
	}
	if strings.Contains(lower, "join") && hasAny(lower, "kubeadm", "cluster", "worker", "control-plane", "control plane", "node", "nodes") {
		return true
	}
	return false
}

func isVerificationOnlyClusterRequest(lower string) bool {
	if !mentionsClusterVerification(lower) {
		return false
	}
	if strings.Contains(lower, "kubeadm") || mentionsJoinWorkflow(lower) {
		return false
	}
	if hasAny(lower, "bootstrap", "initkubeadm", "joinkubeadm", "install package", "downloadpackage", "downloadimage", "prepare workflow") {
		return false
	}
	return true
}

func clusterImplementationLikelyRelevant(lower string, facts Facts, hasJoinWorkflow bool) bool {
	if facts.NeedsPrepare || hasJoinWorkflow || facts.MultiRoleRequested {
		return true
	}
	if facts.Topology == "multi-node" || facts.Topology == "ha" {
		return true
	}
	if hasAny(lower, "bootstrap", "initkubeadm", "joinkubeadm", "set up", "setup", "configure", "compose", "구성", "설치", "만들") {
		return true
	}
	return false
}

func containsWholeToken(lower string, want string) bool {
	fields := strings.FieldsFunc(lower, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == strings.TrimSpace(want) {
			return true
		}
	}
	return false
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
