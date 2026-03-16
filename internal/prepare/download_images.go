package prepare

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"

	"github.com/taedi90/deck/internal/filemode"
)

type imageDownloadOps struct {
	parseReference func(string) (name.Reference, error)
	fetchImage     func(name.Reference, ...remote.Option) (v1.Image, error)
	writeArchive   func(string, name.Reference, v1.Image, ...tarball.WriteOption) error
}

func defaultImageDownloadOps() imageDownloadOps {
	return imageDownloadOps{
		parseReference: parseWeakImageReference,
		fetchImage:     remote.Image,
		writeArchive:   tarball.WriteToFile,
	}
}

func parseWeakImageReference(v string) (name.Reference, error) {
	return name.ParseReference(v, name.WeakValidation)
}

func resolveImageDownloadOps(opts RunOptions) imageDownloadOps {
	if opts.imageDownloadOps.parseReference == nil || opts.imageDownloadOps.fetchImage == nil || opts.imageDownloadOps.writeArchive == nil {
		return defaultImageDownloadOps()
	}
	return opts.imageDownloadOps
}

func runImageDownload(ctx context.Context, runner CommandRunner, bundleRoot string, spec map[string]any, opts RunOptions) ([]string, error) {
	_ = runner
	output := mapValue(spec, "output")
	dir := stringValue(output, "dir")
	if dir == "" {
		dir = "images"
	}

	images := stringSlice(spec["images"])
	if len(images) == 0 {
		return nil, fmt.Errorf("image action download requires images")
	}

	backend := mapValue(spec, "backend")
	engine := stringValue(backend, "engine")
	if engine == "" {
		engine = "go-containerregistry"
	}

	if engine != "go-containerregistry" {
		return nil, fmt.Errorf("%s: unsupported image engine: %s", errCodePrepareEngineUnsupported, engine)
	}

	return runGoContainerRegistryDownloads(ctx, bundleRoot, dir, images, opts)
}

func runGoContainerRegistryDownloads(ctx context.Context, bundleRoot, dir string, images []string, opts RunOptions) ([]string, error) {
	deps := resolveImageDownloadOps(opts)
	files := make([]string, 0, len(images))
	for _, img := range images {
		rel := filepath.ToSlash(filepath.Join(dir, sanitizeImageName(img)+".tar"))
		target := filepath.Join(bundleRoot, filepath.FromSlash(rel))
		if err := filemode.EnsureParentDir(target, filemode.PublishedArtifact); err != nil {
			return nil, err
		}
		if !opts.ForceRedownload {
			if info, err := os.Stat(target); err == nil {
				if info.Size() > 0 {
					files = append(files, rel)
					continue
				}
			} else if !os.IsNotExist(err) {
				return nil, err
			}
		} else if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		ref, err := deps.parseReference(img)
		if err != nil {
			return nil, fmt.Errorf("parse image reference %s: %w", img, err)
		}

		imageObj, err := deps.fetchImage(
			ref,
			remote.WithAuthFromKeychain(authn.DefaultKeychain),
			remote.WithContext(ctx),
		)
		if err != nil {
			return nil, fmt.Errorf("pull image %s: %w", img, err)
		}

		if err := deps.writeArchive(target, ref, imageObj); err != nil {
			return nil, fmt.Errorf("write image archive %s: %w", img, err)
		}

		if info, err := os.Stat(target); err != nil {
			return nil, err
		} else if info.Size() == 0 {
			return nil, fmt.Errorf("write image archive %s: empty archive", img)
		}

		files = append(files, rel)
	}
	return files, nil
}

func sanitizeImageName(v string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		":", "_",
		"@", "_",
	)
	return replacer.Replace(v)
}
