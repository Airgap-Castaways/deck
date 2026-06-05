package stepmeta_test

import (
	"strings"
	"testing"

	"github.com/Airgap-Castaways/deck/internal/stepmeta"
	_ "github.com/Airgap-Castaways/deck/internal/stepspec"
	_ "github.com/Airgap-Castaways/deck/internal/workflowschema"
)

func TestLookupCommand(t *testing.T) {
	entry, ok, err := stepmeta.Lookup("Command")
	if err != nil {
		t.Fatalf("Lookup(Command): %v", err)
	}
	if !ok {
		t.Fatal("expected Command registration")
	}
	if entry.TypeName != "Command" {
		t.Fatalf("unexpected type name: %q", entry.TypeName)
	}
	if entry.Schema.Patch == nil || entry.Schema.SpecType == nil {
		t.Fatalf("expected command schema projection, got %+v", entry.Schema)
	}
	if entry.Docs.Summary == "" || entry.Docs.WhenToUse == "" || entry.Docs.Example == "" {
		t.Fatalf("expected command docs to be populated: %+v", entry.Docs)
	}
	if len(entry.Docs.Fields) != 4 {
		t.Fatalf("expected 4 command fields, got %d", len(entry.Docs.Fields))
	}
}

func TestLookupCheckHostAskMetadataIncludesBinariesContract(t *testing.T) {
	entry, ok, err := stepmeta.Lookup("CheckHost")
	if err != nil {
		t.Fatalf("Lookup(CheckHost): %v", err)
	}
	if !ok {
		t.Fatal("expected CheckHost registration")
	}
	if !hasAskKeyField(entry.Definition.Ask.KeyFields, "spec.binaries") || !hasValidationHint(entry.Definition.Ask.ValidationHints, "binaries") {
		t.Fatalf("expected CheckHost ask metadata to include binaries guidance: %+v", entry.Definition.Ask)
	}
}

func TestLookupWriteFile(t *testing.T) {
	entry, ok, err := stepmeta.Lookup("WriteFile")
	if err != nil {
		t.Fatalf("Lookup(WriteFile): %v", err)
	}
	if !ok {
		t.Fatal("expected WriteFile registration")
	}
	if entry.Docs.Example == "" {
		t.Fatal("expected writefile example")
	}
	for _, field := range entry.Docs.Fields {
		if field.Description == "" {
			t.Fatalf("missing description for %s", field.Path)
		}
		if field.Example == "" {
			t.Fatalf("missing example for %s", field.Path)
		}
	}
}

func TestLookupDownloadFileIncludesNestedFieldDocs(t *testing.T) {
	entry, ok, err := stepmeta.Lookup("DownloadFile")
	if err != nil {
		t.Fatalf("Lookup(DownloadFile): %v", err)
	}
	if !ok {
		t.Fatal("expected DownloadFile registration")
	}
	wanted := []string{
		"spec.source.url",
		"spec.source.bundle.root",
		"spec.fetch.sources[].url",
		"spec.items[].source.path",
	}
	for _, want := range wanted {
		if !hasFieldPath(entry.Docs.Fields, want) {
			t.Fatalf("expected nested field doc %s", want)
		}
	}
}

func TestLookupDownloadImageIncludesNestedFieldDocs(t *testing.T) {
	entry, ok, err := stepmeta.Lookup("DownloadImage")
	if err != nil {
		t.Fatalf("Lookup(DownloadImage): %v", err)
	}
	if !ok {
		t.Fatal("expected DownloadImage registration")
	}
	wanted := []string{
		"spec.auth[].registry",
		"spec.auth[].basic.username",
		"spec.backend.engine",
		"spec.outputDir",
	}
	for _, want := range wanted {
		if !hasFieldPath(entry.Docs.Fields, want) {
			t.Fatalf("expected nested field doc %s", want)
		}
	}
}

func TestLookupWaitVariantsUseDefinitionOverrides(t *testing.T) {
	entry, ok, err := stepmeta.Lookup("WaitForService")
	if err != nil {
		t.Fatalf("Lookup(WaitForService): %v", err)
	}
	if !ok {
		t.Fatal("expected WaitForService registration")
	}
	if entry.Docs.Summary != "Wait until a systemd service reports active." {
		t.Fatalf("unexpected summary: %q", entry.Docs.Summary)
	}
	if entry.Docs.WhenToUse == "" || entry.Docs.Example == "" {
		t.Fatalf("expected override docs for wait variant: %+v", entry.Docs)
	}
	for _, want := range []string{"spec.name", "spec.interval", "spec.timeout"} {
		if !hasFieldPath(entry.Docs.Fields, want) {
			t.Fatalf("expected wait field doc %s", want)
		}
	}

	closed, ok, err := stepmeta.Lookup("WaitForMissingTCPPort")
	if err != nil {
		t.Fatalf("Lookup(WaitForMissingTCPPort): %v", err)
	}
	if !ok {
		t.Fatal("expected WaitForMissingTCPPort registration")
	}
	if closed.Docs.Summary != "Wait until a TCP port closes." {
		t.Fatalf("unexpected TCP closed summary: %q", closed.Docs.Summary)
	}
}

func TestLookupInitKubeadmIncludesRepresentativeFields(t *testing.T) {
	entry, ok, err := stepmeta.Lookup("InitKubeadm")
	if err != nil {
		t.Fatalf("Lookup(InitKubeadm): %v", err)
	}
	if !ok {
		t.Fatal("expected InitKubeadm registration")
	}
	if entry.Docs.Summary != "Run kubeadm init and write the join command to a file." {
		t.Fatalf("unexpected kubeadm init summary: %q", entry.Docs.Summary)
	}
	for _, want := range []string{"spec.outputJoinFile", "spec.kubernetesVersion", "spec.timeout"} {
		if !hasFieldPath(entry.Docs.Fields, want) {
			t.Fatalf("expected kubeadm init field doc %s", want)
		}
	}
	if !hasAskKeyField(entry.Definition.Ask.KeyFields, "spec.configTemplate") || !hasValidationHint(entry.Definition.Ask.ValidationHints, "configFile") {
		t.Fatalf("expected kubeadm init ask metadata to describe configTemplate/configFile coupling: %+v", entry.Definition.Ask)
	}

	reset, ok, err := stepmeta.Lookup("ResetKubeadm")
	if err != nil {
		t.Fatalf("Lookup(ResetKubeadm): %v", err)
	}
	if !ok {
		t.Fatal("expected ResetKubeadm registration")
	}
	for _, want := range []string{"spec.removePaths", "spec.restartRuntimeService", "spec.reportFile"} {
		if !hasFieldPath(reset.Docs.Fields, want) {
			t.Fatalf("expected kubeadm reset field doc %s", want)
		}
	}
}

func TestLookupServiceAndPackageKinds(t *testing.T) {
	service, ok, err := stepmeta.Lookup("ManageService")
	if err != nil {
		t.Fatalf("Lookup(ManageService): %v", err)
	}
	if !ok {
		t.Fatal("expected ManageService registration")
	}
	for _, want := range []string{"spec.name", "spec.state", "spec.timeout"} {
		if !hasFieldPath(service.Docs.Fields, want) {
			t.Fatalf("expected service field doc %s", want)
		}
	}

	pkg, ok, err := stepmeta.Lookup("DownloadPackage")
	if err != nil {
		t.Fatalf("Lookup(DownloadPackage): %v", err)
	}
	if !ok {
		t.Fatal("expected DownloadPackage registration")
	}
	for _, want := range []string{"spec.distro.family", "spec.repo.modules[].name", "spec.backend.image"} {
		if !hasFieldPath(pkg.Docs.Fields, want) {
			t.Fatalf("expected package field doc %s", want)
		}
	}

	repo, ok, err := stepmeta.Lookup("ConfigureRepository")
	if err != nil {
		t.Fatalf("Lookup(ConfigureRepository): %v", err)
	}
	if !ok {
		t.Fatal("expected ConfigureRepository registration")
	}
	for _, want := range []string{"spec.repositories[].id", "spec.repositories[].baseurl"} {
		if !hasAskKeyField(repo.Definition.Ask.KeyFields, want) {
			t.Fatalf("expected repository ask key field %s", want)
		}
	}
	if !hasValidationHint(repo.Definition.Ask.ValidationHints, "spec.repositories") {
		t.Fatalf("expected repository ask metadata to include repository field guidance: %+v", repo.Definition.Ask)
	}
}

func TestLookupContainerdAndClusterKinds(t *testing.T) {
	containerd, ok, err := stepmeta.Lookup("WriteContainerdConfig")
	if err != nil {
		t.Fatalf("Lookup(WriteContainerdConfig): %v", err)
	}
	if !ok {
		t.Fatal("expected WriteContainerdConfig registration")
	}
	for _, want := range []string{"spec.path", "spec.rawSettings[].rawPath", "spec.rawSettings[].value"} {
		if !hasFieldPath(containerd.Docs.Fields, want) {
			t.Fatalf("expected containerd field doc %s", want)
		}
	}
	if !hasAskKeyField(containerd.Definition.Ask.KeyFields, "spec.rawSettings[].value") || !hasValidationHint(containerd.Definition.Ask.ValidationHints, "rawSettings") {
		t.Fatalf("expected containerd ask metadata to include rawSettings value guidance: %+v", containerd.Definition.Ask)
	}

	cluster, ok, err := stepmeta.Lookup("CheckKubernetesCluster")
	if err != nil {
		t.Fatalf("Lookup(CheckKubernetesCluster): %v", err)
	}
	if !ok {
		t.Fatal("expected CheckKubernetesCluster registration")
	}
	for _, want := range []string{"spec.nodes.total", "spec.versions.server", "spec.fileAssertions[].contains", "spec.reports.nodesPath"} {
		if !hasFieldPath(cluster.Docs.Fields, want) {
			t.Fatalf("expected cluster-check field doc %s", want)
		}
	}
}

func TestCategoryForEntryUsesDefinitionOverrideOrFamilyFallback(t *testing.T) {
	entry, ok, err := stepmeta.Lookup("DownloadFile")
	if err != nil {
		t.Fatalf("Lookup(DownloadFile): %v", err)
	}
	if !ok {
		t.Fatal("expected DownloadFile registration")
	}
	if got := stepmeta.CategoryForEntry(entry); got != "filesystem" {
		t.Fatalf("expected filesystem category for DownloadFile, got %q", got)
	}
	override := entry
	override.Definition.Category = "custom"
	if got := stepmeta.CategoryForEntry(override); got != "custom" {
		t.Fatalf("expected explicit category override to win, got %q", got)
	}
}

func TestLookupStructuredEditKinds(t *testing.T) {
	entry, ok, err := stepmeta.Lookup("EditTOML")
	if err != nil {
		t.Fatalf("Lookup(EditTOML): %v", err)
	}
	if !ok {
		t.Fatal("expected EditTOML registration")
	}
	for _, want := range []string{"spec.edits[].op", "spec.edits[].rawPath", "spec.edits[].value"} {
		if !hasFieldPath(entry.Docs.Fields, want) {
			t.Fatalf("expected structured edit field doc %s", want)
		}
	}
}

func TestAllRegisteredKindsValidate(t *testing.T) {
	for _, kind := range stepmeta.RegisteredKinds() {
		if _, ok, err := stepmeta.Lookup(kind); err != nil {
			t.Fatalf("Lookup(%s): %v", kind, err)
		} else if !ok {
			t.Fatalf("expected %s to be registered", kind)
		}
	}
}

func hasFieldPath(fields []stepmeta.FieldDoc, want string) bool {
	for _, field := range fields {
		if field.Path == want {
			return true
		}
	}
	return false
}

func hasAskKeyField(fields []string, want string) bool {
	for _, field := range fields {
		if field == want {
			return true
		}
	}
	return false
}

func hasValidationHint(hints []stepmeta.ValidationHint, token string) bool {
	for _, hint := range hints {
		if strings.Contains(hint.ErrorContains, token) || strings.Contains(hint.Fix, token) {
			return true
		}
	}
	return false
}
