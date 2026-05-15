package prepare

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/Airgap-Castaways/deck/internal/errcode"
	"github.com/Airgap-Castaways/deck/internal/filemode"
	"github.com/Airgap-Castaways/deck/internal/stepspec"
	"github.com/Airgap-Castaways/deck/internal/workflowexec"
)

type imageArtifactMeta struct {
	Images        []string          `json:"images"`
	Platforms     []string          `json:"platforms,omitempty"`
	Files         []string          `json:"files"`
	Checksums     map[string]string `json:"checksums,omitempty"`
	SourceDigests map[string]string `json:"sourceDigests,omitempty"`
}

type imageDownloadOps struct {
	parseReference func(string) (name.Reference, error)
	fetchImage     func(name.Reference, *v1.Platform, ...remote.Option) (v1.Image, error)
	writeArchive   func(string, name.Reference, v1.Image, ...tarball.WriteOption) error
}

func defaultDownloadImageOps() imageDownloadOps {
	return imageDownloadOps{
		parseReference: parseWeakImageReference,
		fetchImage:     fetchRemoteImage,
		writeArchive:   tarball.WriteToFile,
	}
}

func fetchRemoteImage(ref name.Reference, platform *v1.Platform, opts ...remote.Option) (v1.Image, error) {
	if platform != nil {
		opts = append(opts, remote.WithPlatform(*platform))
	}
	return remote.Image(ref, opts...)
}

func parseWeakImageReference(v string) (name.Reference, error) {
	return name.ParseReference(v, name.WeakValidation)
}

func resolveDownloadImageOps(opts RunOptions) imageDownloadOps {
	if opts.imageDownloadOps.parseReference == nil || opts.imageDownloadOps.fetchImage == nil || opts.imageDownloadOps.writeArchive == nil {
		return defaultDownloadImageOps()
	}
	return opts.imageDownloadOps
}

func runDownloadImage(ctx context.Context, runner CommandRunner, bundleRoot string, spec map[string]any, opts RunOptions) ([]string, error) {
	_ = runner
	decoded, err := workflowexec.DecodeSpec[stepspec.DownloadImage](spec)
	if err != nil {
		return nil, fmt.Errorf("decode DownloadImage spec: %w", err)
	}
	dir := strings.TrimSpace(decoded.OutputDir)
	if dir == "" {
		dir = "images"
	}
	validatedDir, err := ensurePreparedPathUnderRoot("DownloadImage", "outputDir", dir, "images")
	if err != nil {
		return nil, err
	}
	dir = validatedDir

	images := decoded.Images
	if len(images) == 0 {
		return nil, fmt.Errorf("image action download requires images")
	}

	engine := strings.TrimSpace(decoded.Backend.Engine)
	if engine == "" {
		engine = "go-containerregistry"
	}

	if engine != "go-containerregistry" {
		return nil, errcode.Newf(errCodePrepareEngineUnsupported, "unsupported image engine: %s", engine)
	}

	auth, err := parseImageRegistryAuth(decoded)
	if err != nil {
		return nil, err
	}

	platforms, err := parseImagePlatforms(decoded.Platforms)
	if err != nil {
		return nil, err
	}

	return runGoContainerRegistryDownloads(ctx, bundleRoot, dir, images, platforms, auth, opts)
}

type imagePlatformSelection struct {
	Raw      string
	Platform v1.Platform
}

func runGoContainerRegistryDownloads(ctx context.Context, bundleRoot, dir string, images []string, platforms []imagePlatformSelection, auth imageRegistryAuthMap, opts RunOptions) ([]string, error) {
	deps := resolveDownloadImageOps(opts)
	platformKeys := imagePlatformKeys(platforms)
	if files, reused, err := tryReuseImageArtifact(bundleRoot, dir, images, platformKeys, opts); err != nil {
		return nil, err
	} else if reused {
		return files, nil
	}
	fetchTargets := imageFetchTargets(platforms)
	files := make([]string, 0, len(images)*len(fetchTargets))
	sourceDigests := map[string]string{}
	for _, img := range images {
		ref, err := deps.parseReference(img)
		if err != nil {
			return nil, fmt.Errorf("parse image reference %s: %w", img, err)
		}
		registry := ref.Context().RegistryStr()

		for _, fetchTarget := range fetchTargets {
			rel := imageArchiveRel(dir, img, fetchTarget.raw)
			target := filepath.Join(bundleRoot, filepath.FromSlash(rel))
			if err := filemode.EnsureParentDir(target, filemode.PublishedArtifact); err != nil {
				return nil, err
			}
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return nil, err
			}

			imageObj, err := deps.fetchImage(
				ref,
				fetchTarget.platform,
				remote.WithAuthFromKeychain(auth.keychain()),
				remote.WithContext(ctx),
			)
			if err != nil {
				if auth.hasRegistry(registry) {
					return nil, fmt.Errorf("pull image %s%s from registry %s with configured auth: %w", img, imagePlatformErrorSuffix(fetchTarget.raw), registry, err)
				}
				return nil, fmt.Errorf("pull image %s%s: %w", img, imagePlatformErrorSuffix(fetchTarget.raw), err)
			}

			if err := deps.writeArchive(target, ref, imageObj); err != nil {
				return nil, fmt.Errorf("write image archive %s%s: %w", img, imagePlatformErrorSuffix(fetchTarget.raw), err)
			}
			if digest, digestErr := imageObj.Digest(); digestErr == nil {
				sourceDigests[imageDigestKey(img, fetchTarget.raw)] = digest.String()
			}

			if info, err := os.Stat(target); err != nil {
				return nil, err
			} else if info.Size() == 0 {
				return nil, fmt.Errorf("write image archive %s%s: empty archive", img, imagePlatformErrorSuffix(fetchTarget.raw))
			}

			files = append(files, rel)
		}
	}
	if err := writeImageArtifactMeta(bundleRoot, dir, images, platformKeys, files, sourceDigests); err != nil {
		return nil, err
	}
	return files, nil
}

type imageFetchTarget struct {
	raw      string
	platform *v1.Platform
}

func imageFetchTargets(platforms []imagePlatformSelection) []imageFetchTarget {
	if len(platforms) == 0 {
		return []imageFetchTarget{{}}
	}
	targets := make([]imageFetchTarget, 0, len(platforms))
	for _, platform := range platforms {
		p := platform.Platform
		targets = append(targets, imageFetchTarget{raw: platform.Raw, platform: &p})
	}
	return targets
}

func imagePlatformErrorSuffix(platform string) string {
	if strings.TrimSpace(platform) == "" {
		return ""
	}
	return " for platform " + platform
}

func imageDigestKey(img, platform string) string {
	if strings.TrimSpace(platform) == "" {
		return img
	}
	return img + "@" + platform
}

func imageArchiveRel(dir, img, platform string) string {
	name := sanitizeImageName(img)
	if strings.TrimSpace(platform) != "" {
		name += "_" + sanitizeImagePlatformName(platform)
	}
	return filepath.ToSlash(filepath.Join(dir, name+".tar"))
}

func imageMetaFileAbs(bundleRoot, dir string) string {
	return filepath.Join(bundleRoot, filepath.FromSlash(dir), ".deck-cache-images.json")
}

func readImageArtifactMetaFile(path string) ([]byte, bool) {
	// #nosec G304 -- path is derived from the internal bundle artifact layout under bundleRoot.
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return raw, true
}

func parseImageArtifactMeta(raw []byte) (imageArtifactMeta, bool) {
	var meta imageArtifactMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return imageArtifactMeta{}, false
	}
	meta.Images = normalizeStrings(meta.Images)
	meta.Platforms = normalizeStrings(meta.Platforms)
	meta.Files = normalizeStrings(meta.Files)
	meta.Checksums = normalizeChecksumMap(meta.Checksums)
	meta.SourceDigests = normalizeChecksumMap(meta.SourceDigests)
	if len(meta.Images) == 0 || len(meta.Files) == 0 {
		return imageArtifactMeta{}, false
	}
	return meta, true
}

func loadImageArtifactMeta(bundleRoot, dir string) (imageArtifactMeta, bool, error) {
	raw, ok := readImageArtifactMetaFile(imageMetaFileAbs(bundleRoot, dir))
	if !ok {
		return imageArtifactMeta{}, false, nil
	}
	meta, ok := parseImageArtifactMeta(raw)
	if !ok {
		return imageArtifactMeta{}, false, nil
	}
	return meta, true, nil
}

func writeImageArtifactMeta(bundleRoot, dir string, images, platforms, files []string, sourceDigests map[string]string) error {
	checksums, err := computeArtifactChecksums(bundleRoot, files)
	if err != nil {
		return err
	}
	meta := imageArtifactMeta{
		Images:        normalizeStrings(images),
		Platforms:     normalizeStrings(platforms),
		Files:         normalizeStrings(files),
		Checksums:     checksums,
		SourceDigests: normalizeChecksumMap(sourceDigests),
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	path := imageMetaFileAbs(bundleRoot, dir)
	if err := filemode.EnsureParentArtifactDir(path); err != nil {
		return err
	}
	return filemode.WriteArtifactFile(path, raw)
}

func tryReuseImageArtifact(bundleRoot, dir string, images, platforms []string, opts RunOptions) ([]string, bool, error) {
	if opts.ForceRedownload {
		return nil, false, nil
	}
	meta, ok, err := loadImageArtifactMeta(bundleRoot, dir)
	if err != nil {
		return nil, false, err
	}
	if !ok {
		return nil, false, nil
	}
	if !equalStrings(normalizeStrings(images), meta.Images) {
		return nil, false, nil
	}
	if !equalStrings(normalizeStrings(platforms), meta.Platforms) {
		return nil, false, nil
	}
	checksumsOK, err := verifyArtifactChecksums(bundleRoot, meta.Files, meta.Checksums)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if !checksumsOK {
		return nil, false, nil
	}
	return meta.Files, true, nil
}

type imageRegistryAuth struct {
	registry string
	username string
	password string
}

type imageRegistryAuthMap map[string]imageRegistryAuth

func (m imageRegistryAuthMap) hasRegistry(registry string) bool {
	_, ok := m[strings.ToLower(strings.TrimSpace(registry))]
	return ok
}

func (m imageRegistryAuthMap) keychain() authn.Keychain {
	if len(m) == 0 {
		return authn.DefaultKeychain
	}
	return imageAuthKeychain{entries: m, fallback: authn.DefaultKeychain}
}

type imageAuthKeychain struct {
	entries  imageRegistryAuthMap
	fallback authn.Keychain
}

func (k imageAuthKeychain) Resolve(resource authn.Resource) (authn.Authenticator, error) {
	registry := strings.ToLower(strings.TrimSpace(resource.RegistryStr()))
	if entry, ok := k.entries[registry]; ok {
		return authn.FromConfig(authn.AuthConfig{Username: entry.username, Password: entry.password}), nil
	}
	if k.fallback == nil {
		return authn.Anonymous, nil
	}
	return k.fallback.Resolve(resource)
}

func parseImageRegistryAuth(spec stepspec.DownloadImage) (imageRegistryAuthMap, error) {
	if len(spec.Auth) == 0 {
		return nil, nil
	}
	entries := make(imageRegistryAuthMap, len(spec.Auth))
	duplicates := make([]string, 0)
	for _, item := range spec.Auth {
		registry := strings.ToLower(strings.TrimSpace(item.Registry))
		username := strings.TrimSpace(item.Basic.Username)
		password := item.Basic.Password
		if registry == "" {
			return nil, fmt.Errorf("image auth entry requires registry")
		}
		if username == "" || password == "" {
			return nil, fmt.Errorf("image auth entry for registry %s requires basic.username and basic.password", registry)
		}
		if _, exists := entries[registry]; exists {
			duplicates = append(duplicates, registry)
			continue
		}
		entries[registry] = imageRegistryAuth{registry: registry, username: username, password: password}
	}
	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		return nil, fmt.Errorf("image auth contains duplicate registry entries: %s", strings.Join(duplicates, ", "))
	}
	return entries, nil
}

func parseImagePlatforms(raw []stepspec.ImagePlatform) ([]imagePlatformSelection, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	platforms := make([]imagePlatformSelection, 0, len(raw))
	seen := map[string]bool{}
	duplicates := make([]string, 0)
	for _, item := range raw {
		platform, err := parseImagePlatform(string(item))
		if err != nil {
			return nil, err
		}
		if seen[platform.Raw] {
			duplicates = append(duplicates, platform.Raw)
			continue
		}
		seen[platform.Raw] = true
		platforms = append(platforms, platform)
	}
	if len(duplicates) > 0 {
		sort.Strings(duplicates)
		return nil, fmt.Errorf("image platforms contains duplicate entries: %s", strings.Join(duplicates, ", "))
	}
	return platforms, nil
}

func parseImagePlatform(raw string) (imagePlatformSelection, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	parts := strings.Split(trimmed, "/")
	if len(parts) != 2 && len(parts) != 3 {
		return imagePlatformSelection{}, fmt.Errorf("image platform %q must use os/arch or os/arch/variant", raw)
	}
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part)
		if parts[i] == "" {
			return imagePlatformSelection{}, fmt.Errorf("image platform %q must not contain empty components", raw)
		}
	}
	platform := v1.Platform{OS: parts[0], Architecture: parts[1]}
	if len(parts) == 3 {
		platform.Variant = parts[2]
	}
	return imagePlatformSelection{Raw: imagePlatformKey(platform), Platform: platform}, nil
}

func imagePlatformKeys(platforms []imagePlatformSelection) []string {
	if len(platforms) == 0 {
		return nil
	}
	keys := make([]string, 0, len(platforms))
	for _, platform := range platforms {
		keys = append(keys, platform.Raw)
	}
	return keys
}

func imagePlatformKey(platform v1.Platform) string {
	key := strings.ToLower(strings.TrimSpace(platform.OS)) + "/" + strings.ToLower(strings.TrimSpace(platform.Architecture))
	if variant := strings.ToLower(strings.TrimSpace(platform.Variant)); variant != "" {
		key += "/" + variant
	}
	return key
}

func sanitizeImageName(v string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		":", "_",
		"@", "_",
	)
	return replacer.Replace(v)
}

func sanitizeImagePlatformName(v string) string {
	return strings.NewReplacer("/", "_").Replace(strings.ToLower(strings.TrimSpace(v)))
}
