package askpolicy

import (
	"strings"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func PlanNeedsClarification(plan askcontract.PlanResponse) bool {
	for _, item := range plan.Clarifications {
		if item.BlocksGeneration && strings.TrimSpace(item.Answer) == "" {
			return true
		}
	}
	return false
}

func NormalizePlanNotes(existingBlockers []string, existingQuestions []string, clarifications []askcontract.PlanClarification) ([]string, []string) {
	clarificationText := map[string]bool{}
	clarificationTopics := clarificationTopicHints(clarifications)
	for _, item := range clarifications {
		if text := strings.TrimSpace(item.Question); text != "" {
			clarificationText[text] = true
		}
	}
	blockers := []string{}
	for _, item := range existingBlockers {
		text := strings.TrimSpace(item)
		if text == "" || clarificationText[text] || clarificationNoteMatchesTopic(text, clarificationTopics) {
			continue
		}
		blockers = append(blockers, text)
	}
	questions := []string{}
	for _, item := range existingQuestions {
		text := strings.TrimSpace(item)
		if text == "" || clarificationText[text] || clarificationNoteMatchesTopic(text, clarificationTopics) {
			continue
		}
		questions = append(questions, text)
	}
	return dedupeStrings(blockers), dedupeStrings(questions)
}

func clarificationTopicHints(clarifications []askcontract.PlanClarification) map[string]bool {
	topics := map[string]bool{}
	addTopics := func(values ...string) {
		for _, topic := range values {
			topic = strings.TrimSpace(strings.ToLower(topic))
			if topic != "" {
				topics[topic] = true
			}
		}
	}
	for _, item := range clarifications {
		if strings.TrimSpace(item.Answer) == "" {
			continue
		}
		switch strings.TrimSpace(item.ID) {
		case "topology.kind":
			addTopics("topology", "single-node", "multi-node", "ha")
		case "topology.nodeCount":
			addTopics("node count", "total node count", "how many nodes")
		case "topology.roleModel":
			addTopics("role model", "control-plane", "worker", "role layout")
		case "cluster.implementation":
			addTopics("implementation", "kubeadm")
		case "runtime.platformFamily":
			addTopics("platform family", "distro", "debian", "rhel", "os family")
		}
		question := strings.ToLower(strings.TrimSpace(item.Question))
		switch {
		case strings.Contains(question, "platform family") || strings.Contains(question, "os family") || strings.Contains(question, "distro"):
			addTopics("platform family", "distro", "debian", "rhel", "os family")
		case strings.Contains(question, "repository") || strings.Contains(question, "repo"):
			addTopics("repository", "repo", "local repo", "mirror")
		case strings.Contains(question, "kubernetes version") || strings.Contains(question, "version should") || strings.Contains(question, "version should package staging"):
			addTopics("kubernetes version", "package version", "version pinning")
		}
	}
	return topics
}

func clarificationNoteMatchesTopic(note string, topics map[string]bool) bool {
	if len(topics) == 0 {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(note))
	if !clarificationNoteLooksLikeMissingInput(lower) {
		return false
	}
	for topic := range topics {
		if strings.Contains(lower, topic) {
			return true
		}
	}
	return false
}

func clarificationNoteLooksLikeMissingInput(note string) bool {
	for _, marker := range []string{
		"not specified",
		"not explicit",
		"unclear",
		"which ",
		"what ",
		"missing ",
		"depends on",
	} {
		if strings.Contains(note, marker) {
			return true
		}
	}
	return false
}
