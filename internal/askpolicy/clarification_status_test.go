package askpolicy

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/askcontract"
)

func TestNormalizePlanNotesDropsAnsweredAmbiguityBlockers(t *testing.T) {
	blockers, _ := NormalizePlanNotes([]string{"target platform family is not explicit for repository setup"}, nil, []askcontract.PlanClarification{{
		ID:       "runtime.platformFamily",
		Question: "Which platform family should the plan target?",
		Answer:   "rhel",
	}})
	if len(blockers) != 0 {
		t.Fatalf("expected answered ambiguity blocker to clear, got %#v", blockers)
	}
}

func TestNormalizePlanNotesKeepsConcreteRepoContractBlockers(t *testing.T) {
	blockers, _ := NormalizePlanNotes([]string{"repository contract paths mismatch between prepare and apply consumers"}, nil, []askcontract.PlanClarification{{
		ID:       "repo-delivery",
		Question: "How will apply hosts access the local repository content?",
		Answer:   "filesystem-mounted repo on each node",
	}})
	joined := strings.Join(blockers, "\n")
	if !strings.Contains(joined, "repository contract paths mismatch") {
		t.Fatalf("expected concrete repo contract blocker to remain, got %#v", blockers)
	}
}

func TestNormalizePlanNotesKeepsWorkerJoinSequencingBlocker(t *testing.T) {
	blockers, _ := NormalizePlanNotes([]string{"worker join publication needs explicit sequencing"}, nil, []askcontract.PlanClarification{{
		ID:       "topology.roleModel",
		Question: "Which role layout should the plan use?",
		Answer:   "1cp-2workers",
	}})
	joined := strings.Join(blockers, "\n")
	if !strings.Contains(joined, "worker join publication needs explicit sequencing") {
		t.Fatalf("expected concrete worker-join blocker to remain, got %#v", blockers)
	}
}
