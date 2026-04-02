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
	for _, item := range clarifications {
		switch strings.TrimSpace(item.ID) {
		case "topology.kind":
			for _, topic := range []string{"topology", "single-node", "multi-node", "ha"} {
				topics[topic] = true
			}
		case "topology.nodeCount":
			for _, topic := range []string{"node count", "total node count", "how many nodes"} {
				topics[topic] = true
			}
		case "topology.roleModel":
			for _, topic := range []string{"role model", "control-plane", "worker", "role layout"} {
				topics[topic] = true
			}
		case "cluster.implementation":
			for _, topic := range []string{"implementation", "kubeadm"} {
				topics[topic] = true
			}
		}
	}
	return topics
}

func clarificationNoteMatchesTopic(note string, topics map[string]bool) bool {
	if len(topics) == 0 {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(note))
	for topic := range topics {
		if strings.Contains(lower, topic) {
			return true
		}
	}
	return false
}
