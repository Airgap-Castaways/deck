package main

import (
	"errors"

	"github.com/spf13/cobra"
)

func newBundleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Build, inspect, verify, or extract bundles",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newBundleBuildCommand(),
		newBundleVerifyCommand(),
		newBundleInspectCommand(),
		newBundleExtractCommand(),
	)

	return cmd
}

func newBundleVerifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify [path]",
		Short: "Verify bundle manifest integrity",
		Args:  bundleSinglePathArgs("bundle verify accepts a single <path>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeBundleVerify(cmdFlagValue(cmd, "file"), args)
		},
	}
	cmd.Flags().String("file", "", "bundle path (directory or bundle.tar)")
	return cmd
}

func newBundleInspectCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect [path]",
		Short: "List manifest entries in a bundle",
		Args:  bundleSinglePathArgs("bundle inspect accepts a single <path>"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeBundleInspect(cmdFlagValue(cmd, "file"), cmdFlagValue(cmd, "output"), args)
		},
	}
	cmd.Flags().String("file", "", "bundle path (directory or bundle.tar)")
	cmd.Flags().StringP("output", "o", "text", "output format (text|json)")
	return cmd
}

func newBundleBuildCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Create a bundle archive from a prepared directory",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeBundleBuild(cmdFlagValue(cmd, "root"), cmdFlagValue(cmd, "out"))
		},
	}
	cmd.Flags().String("root", ".", "workspace root to archive")
	cmd.Flags().String("out", "", "output tar archive path")
	return cmd
}

func newBundleExtractCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extract",
		Short: "Extract a bundle archive into a directory",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeBundleExtract(cmdFlagValue(cmd, "file"), cmdFlagValue(cmd, "dest"))
		},
	}
	cmd.Flags().String("file", "", "bundle archive file path")
	cmd.Flags().String("dest", "", "destination directory")
	return cmd
}

func bundleSinglePathArgs(message string) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) > 1 {
			return errors.New(message)
		}
		return nil
	}
}
