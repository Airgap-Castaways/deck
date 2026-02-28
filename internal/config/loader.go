package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	ErrImportCycle = errors.New("E_IMPORT_CYCLE")
)

func Load(path string) (*Workflow, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("workflow path is empty")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}

	stack := map[string]bool{}
	wf, err := loadRecursive(abs, stack)
	if err != nil {
		return nil, err
	}

	return wf, nil
}

func loadRecursive(path string, stack map[string]bool) (*Workflow, error) {
	clean := filepath.Clean(path)
	if stack[clean] {
		return nil, fmt.Errorf("%w: %s", ErrImportCycle, clean)
	}

	stack[clean] = true
	defer delete(stack, clean)

	content, err := os.ReadFile(clean)
	if err != nil {
		return nil, fmt.Errorf("read workflow file: %w", err)
	}

	var root Workflow
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}

	result := &Workflow{
		Version: root.Version,
		Vars:    map[string]any{},
		Phases:  []Phase{},
	}

	baseDir := filepath.Dir(clean)
	for _, imp := range root.Imports {
		resolved := imp
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(baseDir, resolved)
		}

		child, err := loadRecursive(resolved, stack)
		if err != nil {
			return nil, err
		}
		if err := mergeWorkflow(result, child); err != nil {
			return nil, err
		}
	}

	if err := mergeWorkflow(result, &root); err != nil {
		return nil, err
	}

	result.Imports = nil
	return result, nil
}

func mergeWorkflow(dst *Workflow, src *Workflow) error {
	if dst.Version == "" {
		dst.Version = src.Version
	} else if src.Version != "" && dst.Version != src.Version {
		return fmt.Errorf("E_MERGE_CONFLICT: version mismatch (%s vs %s)", dst.Version, src.Version)
	}

	if dst.Vars == nil {
		dst.Vars = map[string]any{}
	}
	for k, v := range src.Vars {
		dst.Vars[k] = v
	}

	if err := mergeContext(&dst.Context, &src.Context); err != nil {
		return err
	}

	for _, srcPhase := range src.Phases {
		idx := -1
		for i := range dst.Phases {
			if dst.Phases[i].Name == srcPhase.Name {
				idx = i
				break
			}
		}

		if idx < 0 {
			dst.Phases = append(dst.Phases, Phase{Name: srcPhase.Name, Steps: append([]Step{}, srcPhase.Steps...)})
			continue
		}

		dst.Phases[idx].Steps = append(dst.Phases[idx].Steps, srcPhase.Steps...)
	}

	return nil
}

func mergeContext(dst *Context, src *Context) error {
	if src.BundleRoot != "" {
		if dst.BundleRoot != "" && dst.BundleRoot != src.BundleRoot {
			return fmt.Errorf("E_MERGE_CONFLICT: context.bundleRoot conflict")
		}
		dst.BundleRoot = src.BundleRoot
	}

	if src.StateFile != "" {
		if dst.StateFile != "" && dst.StateFile != src.StateFile {
			return fmt.Errorf("E_MERGE_CONFLICT: context.stateFile conflict")
		}
		dst.StateFile = src.StateFile
	}

	return nil
}
