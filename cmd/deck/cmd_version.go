package main

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/taedi90/deck/internal/buildinfo"
)

func newVersionCommand() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show deck build version",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if !jsonOutput {
				return stdoutPrintf("%s\n", buildinfo.Summary())
			}

			raw, err := json.MarshalIndent(buildinfo.Current(), "", "  ")
			if err != nil {
				return err
			}
			return stdoutPrintf("%s\n", raw)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print version details as json")
	return cmd
}
