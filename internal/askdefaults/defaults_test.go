package askdefaults

import "testing"

func TestPackageOutputDirDefaults(t *testing.T) {
	tests := []struct {
		name    string
		family  string
		release string
		repo    string
		want    string
	}{
		{name: "rpm release", family: "rhel", release: "9", repo: "rpm", want: "packages/rpm/9"},
		{name: "deb release", family: "debian", release: "12", repo: "deb-flat", want: "packages/deb/12"},
		{name: "empty release", family: "rhel", repo: "rpm", want: "packages/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PackageOutputDir(tt.family, tt.release, tt.repo); got != tt.want {
				t.Fatalf("PackageOutputDir()=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestKubeadmImagesDefaultVersion(t *testing.T) {
	images := KubeadmImages("")
	if len(images) != 5 {
		t.Fatalf("expected 5 images, got %d", len(images))
	}
	if images[0] != "registry.k8s.io/kube-apiserver:v"+DefaultKubernetesVersion {
		t.Fatalf("unexpected first image: %q", images[0])
	}
}
