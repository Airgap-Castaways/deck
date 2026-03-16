package config

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/taedi90/deck/internal/fsutil"
)

type workflowOrigin struct {
	localPath string
	remoteURL *url.URL
}

func loadWorkflowSource(ctx context.Context, source string) ([]byte, workflowOrigin, error) {
	if u, ok := parseHTTPURL(source); ok {
		b, err := getRequiredHTTP(ctx, u.String())
		if err != nil {
			return nil, workflowOrigin{}, err
		}
		return b, workflowOrigin{remoteURL: u}, nil
	}

	abs, err := filepath.Abs(source)
	if err != nil {
		return nil, workflowOrigin{}, fmt.Errorf("resolve path: %w", err)
	}
	b, err := fsutil.ReadFile(abs)
	if err != nil {
		return nil, workflowOrigin{}, fmt.Errorf("read workflow file: %w", err)
	}
	return b, workflowOrigin{localPath: abs}, nil
}

func normalizeComponentImportRef(ref string) (string, error) {
	ref = strings.TrimSpace(strings.ReplaceAll(ref, "\\", "/"))
	if ref == "" {
		return "", fmt.Errorf("workflow import path is empty")
	}
	if strings.HasPrefix(ref, "/") {
		return "", fmt.Errorf("workflow import path must be components-relative: %s", ref)
	}
	if strings.Contains(ref, "://") {
		return "", fmt.Errorf("workflow import path must not be a URL: %s", ref)
	}
	cleaned := path.Clean(ref)
	if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
		return "", fmt.Errorf("workflow import path must stay under components root: %s", ref)
	}
	return cleaned, nil
}

func localComponentsRoot(localPath string) (string, error) {
	workflowRoot, err := WorkflowRootForPath(localPath)
	if err != nil {
		return "", err
	}
	return filepath.Join(workflowRoot, "components"), nil
}

func WorkflowRootForPath(localPath string) (string, error) {
	current := filepath.Dir(localPath)
	for {
		if filepath.Base(current) == "workflows" {
			return current, nil
		}
		next := filepath.Dir(current)
		if next == current {
			return "", fmt.Errorf("workflow import requires file under workflows/: %s", localPath)
		}
		current = next
	}
}

func remoteComponentsRoot(u *url.URL) (*url.URL, error) {
	workflowRoot, err := remoteWorkflowRoot(u)
	if err != nil {
		return nil, err
	}
	v := *workflowRoot
	v.Path = path.Join(v.Path, "components")
	v.RawQuery = ""
	v.Fragment = ""
	return &v, nil
}

func remoteWorkflowRoot(u *url.URL) (*url.URL, error) {
	cleanPath := path.Clean(u.Path)
	marker := "/workflows/"
	idx := strings.LastIndex(cleanPath, marker)
	if idx < 0 {
		return nil, fmt.Errorf("workflow import requires URL under /workflows/: %s", u.String())
	}
	rootPath := cleanPath[:idx+len("/workflows")]
	v := *u
	v.Path = rootPath
	v.RawQuery = ""
	v.Fragment = ""
	return &v, nil
}

func parseHTTPURL(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false
	}
	if strings.TrimSpace(u.Host) == "" {
		return nil, false
	}
	return u, true
}

func siblingURL(u *url.URL, fileName string) *url.URL {
	v := *u
	v.Path = path.Join(path.Dir(u.Path), fileName)
	v.RawQuery = ""
	v.Fragment = ""
	return &v
}
