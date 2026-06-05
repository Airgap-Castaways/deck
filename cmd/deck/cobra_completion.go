package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion <bash|zsh|fish|powershell>",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts.

Immediate sourcing:
  bash: source <(deck completion bash)
  zsh:  source <(deck completion zsh)
  fish: deck completion fish | source

Persistent setup:
  bash: add 'source <(deck completion bash)' to ~/.bashrc
  zsh:  add 'source <(deck completion zsh)' to ~/.zshrc
  fish: deck completion fish > ~/.config/fish/completions/deck.fish`,
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}

	cmd.ValidArgs = []string{"bash", "zsh", "fish", "powershell"}

	return cmd
}
