package main

import "github.com/spf13/cobra"

const (
	commandGroupCore       = "core"
	commandGroupAdditional = "additional"
)

func newRootCommand() *cobra.Command {
	cobra.EnableCommandSorting = false

	cmd := &cobra.Command{
		Use:                "deck",
		Short:              "deck",
		Long:               "Run deck workflows for offline preparation and local execution.",
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableSuggestions: true,
	}

	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.AddGroup(
		&cobra.Group{ID: commandGroupCore, Title: "Core Commands"},
		&cobra.Group{ID: commandGroupAdditional, Title: "Additional Commands"},
	)

	cmd.AddCommand(
		withGroup(newInitCommand(), commandGroupCore),
		withGroup(newLintCommand(), commandGroupCore),
		withGroup(newPrepareCommand(), commandGroupCore),
		withGroup(newBundleCommand(), commandGroupCore),
		withGroup(newApplyCommand(), commandGroupCore),
		withGroup(newServerCommand(), commandGroupAdditional),
		withGroup(newPlanCommand(), commandGroupAdditional),
		withGroup(newDoctorCommand(), commandGroupAdditional),
		withGroup(newCompletionCommand(), commandGroupAdditional),
		withGroup(newCacheCommand(), commandGroupAdditional),
		withGroup(newNodeCommand(), commandGroupAdditional),
		withGroup(newSiteCommand(), commandGroupAdditional),
	)

	return cmd
}

func withGroup(cmd *cobra.Command, groupID string) *cobra.Command {
	cmd.GroupID = groupID
	return cmd
}
