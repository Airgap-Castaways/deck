package main

import (
	"strings"

	"github.com/spf13/cobra"
)

func newSourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source",
		Short: "Manage the saved remote content source",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newSourceSetCommand(),
		newSourceShowCommand(),
		newSourceUnsetCommand(),
	)

	return cmd
}

func newSourceSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <url>",
		Short: "Save the default server-backed scenario source URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return executeSourceSet(args[0])
		},
	}
	return cmd
}

func newSourceShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the effective default server-backed scenario source URL",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return executeSourceShow()
		},
	}
	return cmd
}

func newSourceUnsetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Clear the saved default server-backed scenario source URL",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return executeSourceUnset()
		},
	}
	return cmd
}

func executeSourceSet(rawURL string) error {
	resolved := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if err := validateSourceURL(resolved); err != nil {
		return err
	}
	if err := saveSourceDefaults(sourceDefaults{URL: resolved}); err != nil {
		return err
	}
	return stdoutPrintf("source default set: %s\n", resolved)
}

func executeSourceShow() error {
	resolved, source, err := resolveSourceURL("")
	if err != nil {
		return err
	}
	if resolved == "" {
		if err := stdoutPrintln("source="); err != nil {
			return err
		}
		return stdoutPrintln("origin=none")
	}
	if err := stdoutPrintf("source=%s\n", resolved); err != nil {
		return err
	}
	return stdoutPrintf("origin=%s\n", source)
}

func executeSourceUnset() error {
	if err := clearSourceDefaults(); err != nil {
		return err
	}
	return stdoutPrintln("source default cleared")
}
