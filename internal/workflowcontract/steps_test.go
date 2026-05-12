package workflowcontract

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	"github.com/Airgap-Castaways/deck/internal/workspacepaths"
)

func TestStepDefFromMetaMissingRegistrationReturnsError(t *testing.T) {
	_, err := stepDefFromMeta("MissingKind", "missingkind")
	if err == nil {
		t.Fatal("expected error for missing step registration")
	}
	if !strings.Contains(err.Error(), "missing stepmeta registration for MissingKind") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStepDefinitionsUseStepmetaCategoryProjection(t *testing.T) {
	defs, err := StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	for _, def := range defs {
		entry, ok, err := stepmeta.LookupCatalogEntry(def.Kind)
		if err != nil {
			t.Fatalf("LookupCatalogEntry(%s): %v", def.Kind, err)
		}
		if !ok {
			t.Fatalf("expected stepmeta entry for %s", def.Kind)
		}
		if def.Category != stepmeta.CategoryForEntry(entry) {
			t.Fatalf("category mismatch for %s: def=%q stepmeta=%q", def.Kind, def.Category, stepmeta.CategoryForEntry(entry))
		}
	}
}

func TestGeneratorForKindMissingRegistrationReturnsError(t *testing.T) {
	_, err := generatorForKind("MissingKind")
	if err == nil {
		t.Fatal("expected error for missing step registration")
	}
	if !strings.Contains(err.Error(), "missing stepmeta registration for MissingKind") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStepDefinitionsProjectParallelMetadata(t *testing.T) {
	defs, err := StepDefinitions()
	if err != nil {
		t.Fatalf("StepDefinitions: %v", err)
	}
	byKind := make(map[string]StepDefinition, len(defs))
	for _, def := range defs {
		byKind[def.Kind] = def
	}

	safeApplyKinds := map[string]bool{
		"Command":               true,
		"CopyFile":              true,
		"EnsureDirectory":       true,
		"ExtractArchive":        true,
		"WaitForCommand":        true,
		"WaitForFile":           true,
		"WaitForMissingFile":    true,
		"WaitForMissingTCPPort": true,
		"WaitForService":        true,
		"WaitForTCPPort":        true,
		"WriteFile":             true,
	}
	for _, def := range defs {
		if def.Parallel.ApplySafe != safeApplyKinds[def.Kind] {
			t.Fatalf("%s ApplySafe=%v, want %v", def.Kind, def.Parallel.ApplySafe, safeApplyKinds[def.Kind])
		}
	}

	targets := map[string][]string{
		"ConfigureRepository":          {"spec.path"},
		"CopyFile":                     {"spec.path"},
		"CreateSymlink":                {"spec.path"},
		"EditFile":                     {"spec.output.path", "spec.path"},
		"EditJSON":                     {"spec.path"},
		"EditTOML":                     {"spec.path"},
		"EditYAML":                     {"spec.path"},
		"EnsureDirectory":              {"spec.path"},
		"ExtractArchive":               {"spec.output.path", "spec.path"},
		"WriteContainerdConfig":        {"spec.path"},
		"WriteContainerdRegistryHosts": {"spec.path"},
		"WriteFile":                    {"spec.path"},
		"WriteSystemdUnit":             {"spec.output.path", "spec.path"},
	}
	for kind, want := range targets {
		if got := byKind[kind].Parallel.ApplyTargetPaths; !reflect.DeepEqual(got, want) {
			t.Fatalf("%s ApplyTargetPaths=%v, want %v", kind, got, want)
		}
	}
	for _, def := range defs {
		for _, path := range def.Parallel.ApplyTargetPaths {
			if !strings.HasPrefix(path, "spec.") {
				t.Fatalf("%s ApplyTargetPaths contains non-spec path %q", def.Kind, path)
			}
		}
		if path := def.Parallel.PrepareOutput.Path; path != "" && !strings.HasPrefix(path, "spec.") {
			t.Fatalf("%s PrepareOutput.Path contains non-spec path %q", def.Kind, path)
		}
	}

	prepareOutputs := map[string]OutputRootConstraint{
		"DownloadFile":    {Path: "spec.outputPath", Root: workspacepaths.PreparedFilesRoot, Example: "files/flannel.yaml"},
		"DownloadImage":   {Path: "spec.outputDir", Root: workspacepaths.PreparedImagesRoot, Example: "images/control-plane"},
		"DownloadPackage": {Path: "spec.outputDir", Root: workspacepaths.PreparedPackagesRoot, Example: "packages/kubernetes"},
	}
	for kind, want := range prepareOutputs {
		if got := byKind[kind].Parallel.PrepareOutput; got != want {
			t.Fatalf("%s PrepareOutput=%+v, want %+v", kind, got, want)
		}
	}
}
