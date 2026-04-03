package main

import (
	"fmt"
	"os"
	"strings"
)

const (
	workflowSchemaBlockBegin    = "<!-- BEGIN GENERATED:WORKFLOW_SCHEMA_CONTRACT -->"
	workflowSchemaBlockEnd      = "<!-- END GENERATED:WORKFLOW_SCHEMA_CONTRACT -->"
	componentFragmentBlockBegin = "<!-- BEGIN GENERATED:COMPONENT_FRAGMENT_CONTRACT -->"
	componentFragmentBlockEnd   = "<!-- END GENERATED:COMPONENT_FRAGMENT_CONTRACT -->"
	toolDefinitionBlockBegin    = "<!-- BEGIN GENERATED:TOOL_DEFINITION_SCHEMA -->"
	toolDefinitionBlockEnd      = "<!-- END GENERATED:TOOL_DEFINITION_SCHEMA -->"
)

func syncGeneratedBlock(targetPath string, begin string, end string, block []byte) error {
	//nolint:gosec // Paths come from generator-owned repo locations.
	target, err := os.ReadFile(targetPath)
	if err != nil {
		return err
	}
	updated, err := replaceManagedBlock(string(target), begin, end, string(block))
	if err != nil {
		return fmt.Errorf("sync %s managed block: %w", targetPath, err)
	}
	return writeFile(targetPath, []byte(updated))
}

func replaceManagedBlock(content string, begin string, end string, block string) (string, error) {
	start := strings.Index(content, begin)
	finish := strings.Index(content, end)
	if start == -1 || finish == -1 || finish < start {
		return "", fmt.Errorf("missing managed block markers %q and %q", begin, end)
	}
	finish += len(end)
	replacement := begin + "\n" + strings.TrimRight(block, "\n") + "\n" + end
	return content[:start] + replacement + content[finish:], nil
}
