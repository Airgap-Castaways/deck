package workflowrefs

import "testing"

func TestTemplateReferencesAcceptAliasForms(t *testing.T) {
	refs := TemplateReferences("{{ vars.kubernetesVersion }} ${{ vars.joinFile }} {{ .vars.criSocket }}")
	assertReferencePath(t, refs, NamespaceVars, "kubernetesVersion")
	assertReferencePath(t, refs, NamespaceVars, "joinFile")
	assertReferencePath(t, refs, NamespaceVars, "criSocket")
}

func TestTemplateReferencesIncludeEmbeddedAndIndexPaths(t *testing.T) {
	refs := TemplateReferences(`files/{{ .vars.kubernetesVersion }}.bin {{ index .vars.downloads 0 "outputPath" }} {{ .runtime.downloaded }}`)
	assertReferencePath(t, refs, NamespaceVars, "kubernetesVersion")
	assertReferencePath(t, refs, NamespaceVars, "downloads")
	assertReferencePath(t, refs, NamespaceRuntime, "downloaded")
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
