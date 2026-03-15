package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	workflowRootDir             = "workflows"
	workflowScenariosDir        = "scenarios"
	workflowComponentsDir       = "components"
	canonicalPrepareWorkflowRel = "scenarios/prepare.yaml"
	canonicalApplyWorkflowRel   = "scenarios/apply.yaml"
	workflowVarsRel             = "vars.yaml"
	deckWorkDirName             = ".deck"
	preparedDirRel              = "outputs"
)

func workflowPath(root string, rel string) string {
	parts := append([]string{root, workflowRootDir}, strings.Split(filepath.ToSlash(rel), "/")...)
	return filepath.Join(parts...)
}

func canonicalPrepareWorkflowPath(root string) string {
	return workflowPath(root, canonicalPrepareWorkflowRel)
}

func canonicalApplyWorkflowPath(root string) string {
	return workflowPath(root, canonicalApplyWorkflowRel)
}

func canonicalVarsPath(root string) string {
	return workflowPath(root, workflowVarsRel)
}

func defaultPreparedRoot(root string) string {
	return filepath.Join(root, preparedDirRel)
}

func locateWorkflowTreeRoot(workflowPath string) (string, error) {
	resolved, err := filepath.Abs(strings.TrimSpace(workflowPath))
	if err != nil {
		return "", fmt.Errorf("resolve workflow path: %w", err)
	}
	dir := filepath.Dir(resolved)
	for {
		if filepath.Base(dir) == workflowRootDir {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("workflow path is not under %s/: %s", workflowRootDir, resolved)
}
