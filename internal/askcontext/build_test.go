package askcontext

import "testing"

func TestBuildStepKindsUsesStepmetaAskMetadata(t *testing.T) {
	manifest := Current()
	var command StepKindContext
	var downloadImage StepKindContext
	for _, kind := range manifest.StepKinds {
		switch kind.Kind {
		case "Command":
			command = kind
		case "DownloadImage":
			downloadImage = kind
		}
	}
	if len(command.MatchSignals) == 0 || command.MatchSignals[0] != "shell" {
		t.Fatalf("expected command match signals from stepmeta, got %+v", command.MatchSignals)
	}
	if len(command.QualityRules) == 0 || command.QualityRules[0].Trigger != "typed-preferred" {
		t.Fatalf("expected command quality rules from stepmeta, got %+v", command.QualityRules)
	}
	if len(command.AntiSignals) == 0 || command.AntiSignals[0] != "typed" {
		t.Fatalf("expected command anti-signals from stepmeta, got %+v", command.AntiSignals)
	}
	if len(downloadImage.ConstrainedLiteralFields) == 0 || downloadImage.ConstrainedLiteralFields[0].Path != "spec.backend.engine" {
		t.Fatalf("expected download image constrained field from stepmeta, got %+v", downloadImage.ConstrainedLiteralFields)
	}
}
