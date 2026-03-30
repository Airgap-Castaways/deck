package stepspec

type ImageAuthBasic struct {
	// Registry username used for basic authentication.
	// @deck.example robot
	Username string `json:"username"`
	// Registry password or access token paired with `username`.
	// @deck.example ${REGISTRY_PASSWORD}
	Password string `json:"password"`
}

type ImageAuth struct {
	// Registry host matched against each image reference.
	// @deck.example registry.example.com
	Registry string `json:"registry"`
	// Explicit basic-auth credentials used for the matched registry.
	// @deck.example {username:robot,password:${REGISTRY_PASSWORD}}
	Basic ImageAuthBasic `json:"basic"`
}

type ImageBackend struct {
	// Image download engine implementation.
	// @deck.example go-containerregistry
	Engine string `json:"engine"`
}

// Download container images into prepared bundle storage.
// @deck.when Use this during prepare to collect required images for offline use.
// @deck.note Omit `outputDir` unless you need a dedicated image subdirectory; deck writes to `images/` by default.
// @deck.note `spec.auth` is optional and only applies to `DownloadImage`.
// @deck.example
// kind: DownloadImage
// spec:
//
//	images:
//	  - registry.k8s.io/kube-apiserver:v1.30.1
//	  - registry.example.com/platform/pause:3.9
//	auth:
//	  - registry: registry.example.com
//	    basic:
//	      username: "{{ .vars.registryUser }}"
//	      password: "{{ .vars.registryPassword }}"
type DownloadImage struct {
	// Fully qualified image references to download.
	// @deck.example [registry.k8s.io/pause:3.9]
	Images []string `json:"images"`
	// Optional registry authentication entries used during download.
	// @deck.example [{registry:registry.example.com,basic:{username:robot,password:${REGISTRY_PASSWORD}}}]
	Auth []ImageAuth `json:"auth"`
	// Backend-specific download settings.
	// @deck.example {engine:go-containerregistry}
	Backend ImageBackend `json:"backend"`
	// Optional bundle-relative directory for per-image tar archives.
	// @deck.example images/control-plane
	OutputDir string `json:"outputDir"`
	// Optional total timeout for the download step.
	// @deck.example 10m
	Timeout string `json:"timeout"`
}

// Load prepared image archives into the local container runtime.
// @deck.when Use this during apply before verifying or using images from an offline bundle.
// @deck.note `command` may include `{archive}` placeholders that deck substitutes per image archive.
// @deck.example
// kind: LoadImage
// spec:
//
//	sourceDir: images/control-plane
//	runtime: ctr
//	images:
//	  - registry.k8s.io/kube-apiserver:v1.30.1
type LoadImage struct {
	// Image references to load from the prepared archives.
	// @deck.example [registry.k8s.io/kube-apiserver:v1.30.1]
	Images []string `json:"images"`
	// Directory containing prepared image archives.
	// @deck.example images/control-plane
	SourceDir string `json:"sourceDir"`
	// Runtime loader to use for imports.
	// @deck.example ctr
	Runtime string `json:"runtime"`
	// Optional runtime command override.
	// @deck.example [ctr,-n,k8s.io,images,import,{archive}]
	Command []string `json:"command"`
	// Optional total timeout for the load step.
	// @deck.example 10m
	Timeout string `json:"timeout"`
}

// Verify that required container images already exist on the node.
// @deck.when Use this during apply when images should already be present and only need verification.
// @deck.note Use this instead of `LoadImage` when the runtime is expected to be pre-populated.
// @deck.example
// kind: VerifyImage
// spec:
//
//	command: [ctr, -n, k8s.io, images, list, -q]
//	images:
//	  - registry.k8s.io/kube-apiserver:v1.30.1
type VerifyImage struct {
	// Image references that must already exist in the runtime.
	// @deck.example [registry.k8s.io/kube-apiserver:v1.30.1]
	Images []string `json:"images"`
	// Optional image-listing command override.
	// @deck.example [ctr,-n,k8s.io,images,list,-q]
	Command []string `json:"command"`
	// Optional total timeout for the verification step.
	// @deck.example 5m
	Timeout string `json:"timeout"`
}
