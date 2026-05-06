package main

import "github.com/spf13/cobra"

const (
	commandGroupCore       = "core"
	commandGroupAdditional = "additional"
)

func newRootCommand(env *cliEnv) *cobra.Command {
	if env == nil {
		env = newCLIEnv(nil, nil)
	}
	cobra.EnableCommandSorting = false
	env.setVerbosity(0)
	env.setLogFormat("text")

	cmd := &cobra.Command{
		Use:                "deck",
		Short:              "deck",
		Long:               "Run deck workflows for offline preparation and local execution.",
		SilenceErrors:      true,
		SilenceUsage:       true,
		DisableSuggestions: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			resolved, err := resolveCLILogFormat(env.logFormat)
			if err != nil {
				return err
			}
			env.setLogFormat(resolved)
			env.setVerbosity(env.verbosity)
			return nil
		},
	}
	cmd.PersistentFlags().IntVar(&env.verbosity, "v", 0, "diagnostic verbosity level")
	cmd.PersistentFlags().StringVar(&env.logFormat, "log-format", "text", "diagnostic log format (text|json)")

	cmd.CompletionOptions.DisableDefaultCmd = true
	cmd.SetHelpCommandGroupID(commandGroupAdditional)
	cmd.AddGroup(
		&cobra.Group{ID: commandGroupCore, Title: "Core Commands:"},
		&cobra.Group{ID: commandGroupAdditional, Title: "Additional Commands:"},
	)

	for _, child := range []*cobra.Command{
		withGroup(newInitCommand(env), commandGroupCore),
		withGroup(newLintCommand(env), commandGroupCore),
		withGroup(newPrepareCommand(env), commandGroupCore),
		withGroup(newBundleCommand(env), commandGroupCore),
		withGroup(newPlanCommand(env), commandGroupCore),
		withGroup(newApplyCommand(env), commandGroupCore),
		withGroup(newListCommand(env), commandGroupAdditional),
		withGroup(newServerCommand(env), commandGroupAdditional),
		withGroup(newAskCommand(env), commandGroupAdditional),
		withGroup(newVersionCommand(env), commandGroupAdditional),
		withGroup(newCompletionCommand(), commandGroupAdditional),
		withGroup(newCacheCommand(env), commandGroupAdditional),
	} {
		if child != nil {
			cmd.AddCommand(child)
		}
	}

	return cmd
}

func withGroup(cmd *cobra.Command, groupID string) *cobra.Command {
	if cmd == nil {
		return nil
	}
	cmd.GroupID = groupID
	return cmd
}
