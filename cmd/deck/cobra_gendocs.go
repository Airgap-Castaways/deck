package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newGenDocsCommand returns a hidden subcommand that writes the full cobra
// command tree to Markdown files under a given directory. Intended for use
// by "make generate" only; not exposed to end users.
func newGenDocsCommand(env *cliEnv) *cobra.Command {
	return &cobra.Command{
		Use:    "__gendocs <output-dir>",
		Short:  "Generate CLI reference markdown (internal)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenDocs(cmd.Root(), args[0])
		},
	}
}

func runGenDocs(root *cobra.Command, dir string) error {
	disableAutoGenTag(root)
	if err := doc.GenMarkdownTree(root, dir); err != nil {
		return fmt.Errorf("generate cli docs: %w", err)
	}
	return nil
}

func disableAutoGenTag(cmd *cobra.Command) {
	cmd.DisableAutoGenTag = true
	for _, c := range cmd.Commands() {
		disableAutoGenTag(c)
	}
}
