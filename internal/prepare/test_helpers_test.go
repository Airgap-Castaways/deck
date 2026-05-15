package prepare

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func nilContextForPrepareTest() context.Context { return nil }

func stubDownloadImageOps() imageDownloadOps {
	return imageDownloadOps{
		parseReference: func(v string) (name.Reference, error) {
			return name.ParseReference(v, name.WeakValidation)
		},
		fetchImage: func(_ name.Reference, _ *v1.Platform, _ ...remote.Option) (v1.Image, error) {
			return empty.Image, nil
		},
		writeArchive: func(path string, _ name.Reference, _ v1.Image, _ ...tarball.WriteOption) error {
			return os.WriteFile(path, []byte("image"), 0o644)
		},
	}
}

type fakeRunner struct {
	mu                sync.Mutex
	containerPayloads map[string]map[string][]byte
	createArgs        [][]string
	failExport        bool
}

type concurrencyRunner struct {
	delegate  CommandRunner
	mu        sync.Mutex
	active    int
	maxActive int
}

type failOnceRunner struct {
	attempts int
	delegate *fakeRunner
}

type noRuntimeRunner struct{}

func (f *failOnceRunner) LookPath(file string) (string, error) {
	if file == "docker" || file == "podman" {
		return "/usr/bin/" + file, nil
	}
	return "", fmt.Errorf("not found")
}

func (f *failOnceRunner) Run(ctx context.Context, name string, args ...string) error {
	fr := f.delegate
	if fr == nil {
		fr = &fakeRunner{}
		f.delegate = fr
	}
	return fr.Run(ctx, name, args...)
}

func (f *failOnceRunner) RunWithIO(ctx context.Context, stdout io.Writer, stderr io.Writer, name string, args ...string) error {
	if (name == "docker" || name == "podman") && len(args) > 0 && args[0] == "start" {
		f.attempts++
		if f.attempts == 1 {
			return fmt.Errorf("intentional first failure")
		}
	}
	fr := f.delegate
	if fr == nil {
		fr = &fakeRunner{}
		f.delegate = fr
	}
	return fr.RunWithIO(ctx, stdout, stderr, name, args...)
}

func (n *noRuntimeRunner) LookPath(_ string) (string, error) {
	return "", fmt.Errorf("not found")
}

func (n *noRuntimeRunner) Run(_ context.Context, _ string, _ ...string) error {
	return nil
}

func (n *noRuntimeRunner) RunWithIO(_ context.Context, _ io.Writer, _ io.Writer, _ string, _ ...string) error {
	return nil
}

type noArtifactRunner struct{}

func (n *noArtifactRunner) LookPath(file string) (string, error) {
	if file == "docker" || file == "podman" {
		return "/usr/bin/" + file, nil
	}
	return "", fmt.Errorf("not found")
}

func (n *noArtifactRunner) Run(_ context.Context, _ string, _ ...string) error {
	return nil
}

func (n *noArtifactRunner) RunWithIO(_ context.Context, stdout io.Writer, _ io.Writer, name string, args ...string) error {
	if (name == "docker" || name == "podman") && len(args) > 0 && args[0] == "create" {
		_, _ = io.WriteString(stdout, "empty-container\n")
	}
	return nil
}

func (f *fakeRunner) LookPath(file string) (string, error) {
	if file == "docker" || file == "podman" {
		return "/usr/bin/" + file, nil
	}
	return "", fmt.Errorf("not found")
}

func (c *concurrencyRunner) LookPath(file string) (string, error) {
	return c.delegate.LookPath(file)
}

func (c *concurrencyRunner) Run(ctx context.Context, name string, args ...string) error {
	return c.delegate.Run(ctx, name, args...)
}

func (c *concurrencyRunner) RunWithIO(ctx context.Context, stdout io.Writer, stderr io.Writer, name string, args ...string) error {
	if name == "docker" || name == "podman" {
		c.mu.Lock()
		c.active++
		if c.active > c.maxActive {
			c.maxActive = c.active
		}
		c.mu.Unlock()
		defer func() {
			c.mu.Lock()
			c.active--
			c.mu.Unlock()
		}()
		time.Sleep(100 * time.Millisecond)
	}
	return c.delegate.RunWithIO(ctx, stdout, stderr, name, args...)
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) error {
	return f.RunWithIO(context.Background(), nil, nil, name, args...)
}

func (f *fakeRunner) RunWithIO(_ context.Context, stdout io.Writer, _ io.Writer, name string, args ...string) error {
	if name != "docker" && name != "podman" {
		return nil
	}
	if f.containerPayloads == nil {
		f.mu.Lock()
		if f.containerPayloads == nil {
			f.containerPayloads = map[string]map[string][]byte{}
		}
		f.mu.Unlock()
	}
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "create":
		f.mu.Lock()
		f.createArgs = append(f.createArgs, append([]string(nil), args...))
		containerID := fmt.Sprintf("container-%d", len(f.containerPayloads)+1)
		f.containerPayloads[containerID] = fakePackageContainerPayload(args)
		f.mu.Unlock()
		if stdout != nil {
			_, _ = io.WriteString(stdout, containerID+"\n")
		}
		return nil
	case "start":
		return nil
	case "cp":
		if f.failExport {
			return fmt.Errorf("intentional export failure")
		}
		if len(args) < 3 {
			return fmt.Errorf("cp requires src and dst")
		}
		containerID := strings.SplitN(args[1], ":", 2)[0]
		f.mu.Lock()
		payload := f.containerPayloads[containerID]
		f.mu.Unlock()
		if stdout == nil {
			return nil
		}
		return writeFakeTar(stdout, payload)
	case "rm":
		if len(args) > 1 {
			f.mu.Lock()
			delete(f.containerPayloads, args[len(args)-1])
			f.mu.Unlock()
		}
		return nil
	}

	for i := 0; i < len(args); i++ {
		if args[i] == "-v" && i+1 < len(args) {
			mount := args[i+1]
			parts := strings.SplitN(mount, ":", 2)
			if len(parts) != 2 {
				continue
			}
			host := parts[0]
			container := parts[1]
			if container == "/out" {
				if err := os.MkdirAll(host, 0o755); err != nil {
					return err
				}
				// repo-mode simulation: create minimal artifacts + metadata
				if strings.Contains(host, string(filepath.Separator)+"packages"+string(filepath.Separator)+"deb"+string(filepath.Separator)) ||
					strings.Contains(host, string(filepath.Separator)+"packages"+string(filepath.Separator)+"deb-k8s"+string(filepath.Separator)) {
					pkgs := filepath.Join(host, "pkgs")
					if err := os.MkdirAll(pkgs, 0o755); err != nil {
						return err
					}
					if strings.Contains(host, string(filepath.Separator)+"packages"+string(filepath.Separator)+"deb-k8s"+string(filepath.Separator)) {
						for _, name := range []string{"kubelet.deb", "kubeadm.deb", "kubectl.deb"} {
							if err := os.WriteFile(filepath.Join(pkgs, name), []byte("pkg"), 0o644); err != nil {
								return err
							}
						}
					} else {
						if err := os.WriteFile(filepath.Join(pkgs, "mock-package.deb"), []byte("pkg"), 0o644); err != nil {
							return err
						}
					}
					if err := os.WriteFile(filepath.Join(host, "Packages"), []byte("Packages"), 0o644); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(host, "Packages.gz"), []byte("Packages.gz"), 0o644); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(host, "Release"), []byte("Release"), 0o644); err != nil {
						return err
					}
					continue
				}
				if strings.Contains(host, string(filepath.Separator)+"packages"+string(filepath.Separator)+"rpm"+string(filepath.Separator)) ||
					strings.Contains(host, string(filepath.Separator)+"packages"+string(filepath.Separator)+"rpm-k8s"+string(filepath.Separator)) {
					repodata := filepath.Join(host, "repodata")
					if err := os.MkdirAll(repodata, 0o755); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(host, "mock-package.rpm"), []byte("pkg"), 0o644); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(repodata, "repomd.xml"), []byte("repomd"), 0o644); err != nil {
						return err
					}
					continue
				}
				if err := os.WriteFile(filepath.Join(host, "mock-package.deb"), []byte("pkg"), 0o644); err != nil {
					return err
				}
			}
		}
	}

	for _, a := range args {
		if strings.Contains(a, "docker-archive:/bundle/") {
			prefix := "docker-archive:/bundle/"
			s := strings.TrimPrefix(a, prefix)
			rel := strings.SplitN(s, ":", 2)[0]
			for i := 0; i < len(args); i++ {
				if args[i] == "-v" && i+1 < len(args) {
					parts := strings.SplitN(args[i+1], ":", 2)
					if len(parts) == 2 && parts[1] == "/bundle" {
						abs := filepath.Join(parts[0], filepath.FromSlash(rel))
						if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
							return err
						}
						return os.WriteFile(abs, []byte("image"), 0o644)
					}
				}
			}
		}
	}

	return nil
}

func fakePackageContainerPayload(args []string) map[string][]byte {
	payload := map[string][]byte{}
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "apt-ftparchive") {
		payload["pkgs/mock-package.deb"] = []byte("pkg")
		payload["Packages"] = []byte("Packages")
		payload["Packages.gz"] = []byte("Packages.gz")
		payload["Release"] = []byte("Release")
		if strings.Contains(joined, "apt-k8s") {
			payload["pkgs/kubelet.deb"] = []byte("pkg")
			payload["pkgs/kubeadm.deb"] = []byte("pkg")
			payload["pkgs/kubectl.deb"] = []byte("pkg")
			delete(payload, "pkgs/mock-package.deb")
		}
		return payload
	}
	if strings.Contains(joined, "createrepo_c") {
		payload["mock-package.rpm"] = []byte("pkg")
		payload["repodata/repomd.xml"] = []byte("repomd")
		return payload
	}
	payload["mock-package.deb"] = []byte("pkg")
	return payload
}

func writeFakeTar(w io.Writer, payload map[string][]byte) error {
	tw := tar.NewWriter(w)
	defer func() { _ = tw.Close() }()
	keys := make([]string, 0, len(payload))
	for key := range payload {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	seenDirs := map[string]bool{}
	for _, key := range keys {
		dir := filepath.Dir(key)
		for dir != "." && dir != "" {
			if seenDirs[dir] {
				break
			}
			seenDirs[dir] = true
			if err := tw.WriteHeader(&tar.Header{Name: filepath.ToSlash(dir), Typeflag: tar.TypeDir, Mode: 0o755}); err != nil {
				return err
			}
			dir = filepath.Dir(dir)
		}
		content := payload[key]
		if err := tw.WriteHeader(&tar.Header{Name: filepath.ToSlash(key), Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(content))}); err != nil {
			return err
		}
		if _, err := tw.Write(content); err != nil {
			return err
		}
	}
	return nil
}
