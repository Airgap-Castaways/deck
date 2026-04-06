package workflowrefs

import "testing"

func TestTemplateReferencesAcceptAliasForms(t *testing.T) {
	refs, err := TemplateReferences("{{ vars.kubernetesVersion }} ${{ vars.joinFile }} {{ .vars.criSocket }}")
	if err != nil {
		t.Fatalf("TemplateReferences returned error: %v", err)
	}
	assertReferencePath(t, refs, NamespaceVars, "kubernetesVersion")
	assertReferencePath(t, refs, NamespaceVars, "joinFile")
	assertReferencePath(t, refs, NamespaceVars, "criSocket")
}

func TestTemplateReferencesIncludeEmbeddedAndIndexPaths(t *testing.T) {
	refs, err := TemplateReferences(`files/{{ .vars.kubernetesVersion }}.bin {{ index .vars.downloads 0 "outputPath" }} {{ .runtime.downloaded }}`)
	if err != nil {
		t.Fatalf("TemplateReferences returned error: %v", err)
	}
	assertReferencePath(t, refs, NamespaceVars, "kubernetesVersion")
	assertReferencePath(t, refs, NamespaceVars, "downloads")
	assertReferencePath(t, refs, NamespaceRuntime, "downloaded")
}

func TestTemplateReferencesNormalizeAliasesInsideFunctionCalls(t *testing.T) {
	refs, err := TemplateReferences(`{{ index vars.nodes 0 "ip" }} {{ eq runtime.host.os.family "rhel" }}`)
	if err != nil {
		t.Fatalf("TemplateReferences returned error: %v", err)
	}
	assertReferencePath(t, refs, NamespaceVars, "nodes")
	assertReferencePath(t, refs, NamespaceRuntime, "host.os.family")
}

func TestTemplateReferencesReturnErrorForMalformedTemplates(t *testing.T) {
	if _, err := TemplateReferences(`{{ if vars.enabled }}`); err == nil {
		t.Fatalf("expected malformed template to return error")
	}
}

func TestWhenReferencesCollectNestedSelectors(t *testing.T) {
	refs, err := WhenReferences(`runtime.host.os.family == "rhel" && vars.role == "control-plane"`)
	if err != nil {
		t.Fatalf("WhenReferences returned error: %v", err)
	}
	assertReferencePath(t, refs, NamespaceRuntime, "host.os.family")
	assertReferencePath(t, refs, NamespaceRuntime, "host")
	assertReferencePath(t, refs, NamespaceVars, "role")
}

func assertReferencePath(t *testing.T, refs []Reference, namespace, path string) {
	t.Helper()
	for _, ref := range refs {
		if ref.Namespace == namespace && ref.Path == path {
			return
		}
	}
	t.Fatalf("expected %s.%s in %#v", namespace, path, refs)
}
