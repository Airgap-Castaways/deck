package main

import (
	"github.com/spf13/cobra"
)

func newSiteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "site",
		Short: "Manage site releases, sessions, and assignments",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	siteReleaseCommand := &cobra.Command{
		Use:   "release",
		Short: "Import or list site releases",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	siteReleaseCommand.AddCommand(
		newSiteReleaseImportCommand(),
		newSiteReleaseListCommand(),
	)

	siteSessionCommand := &cobra.Command{
		Use:   "session",
		Short: "Create or close sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	siteSessionCommand.AddCommand(
		newSiteSessionCreateCommand(),
		newSiteSessionCloseCommand(),
	)

	siteAssignCommand := &cobra.Command{
		Use:   "assign",
		Short: "Assign workflows by role or node",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	siteAssignCommand.AddCommand(
		newSiteAssignRoleCommand(),
		newSiteAssignNodeCommand(),
	)

	cmd.AddCommand(
		siteReleaseCommand,
		siteSessionCommand,
		siteAssignCommand,
		newSiteStatusCommand(),
	)
	return cmd
}

func newSiteReleaseImportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a bundle archive as a release",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			id, err := cmdFlagValue(cmd, "id")
			if err != nil {
				return err
			}
			bundle, err := cmdFlagValue(cmd, "bundle")
			if err != nil {
				return err
			}
			createdAt, err := cmdFlagValue(cmd, "created-at")
			if err != nil {
				return err
			}
			return executeSiteReleaseImport(root, id, bundle, createdAt)
		},
	}
	cmd.Flags().String("root", ".", "site server root")
	cmd.Flags().String("id", "", "release id")
	cmd.Flags().String("bundle", "", "local bundle archive path")
	cmd.Flags().String("created-at", "", "release timestamp (RFC3339, optional)")
	return cmd
}

func newSiteReleaseListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List stored releases",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			output, err := cmdFlagValue(cmd, "output")
			if err != nil {
				return err
			}
			return executeSiteReleaseList(root, output)
		},
	}
	cmd.Flags().String("root", ".", "site server root")
	cmd.Flags().StringP("output", "o", "text", "output format (text|json)")
	return cmd
}

func newSiteSessionCreateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new session for a release",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			id, err := cmdFlagValue(cmd, "id")
			if err != nil {
				return err
			}
			release, err := cmdFlagValue(cmd, "release")
			if err != nil {
				return err
			}
			startedAt, err := cmdFlagValue(cmd, "started-at")
			if err != nil {
				return err
			}
			return executeSiteSessionCreate(root, id, release, startedAt)
		},
	}
	cmd.Flags().String("root", ".", "site server root")
	cmd.Flags().String("id", "", "session id")
	cmd.Flags().String("release", "", "release id")
	cmd.Flags().String("started-at", "", "session start timestamp (RFC3339, optional)")
	return cmd
}

func newSiteSessionCloseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close",
		Short: "Close an existing session",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			id, err := cmdFlagValue(cmd, "id")
			if err != nil {
				return err
			}
			closedAt, err := cmdFlagValue(cmd, "closed-at")
			if err != nil {
				return err
			}
			return executeSiteSessionClose(root, id, closedAt)
		},
	}
	cmd.Flags().String("root", ".", "site server root")
	cmd.Flags().String("id", "", "session id")
	cmd.Flags().String("closed-at", "", "session close timestamp (RFC3339, optional)")
	return cmd
}

func newSiteAssignRoleCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "role",
		Short: "Assign a workflow to a role for a session",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			session, err := cmdFlagValue(cmd, "session")
			if err != nil {
				return err
			}
			assignment, err := cmdFlagValue(cmd, "assignment")
			if err != nil {
				return err
			}
			role, err := cmdFlagValue(cmd, "role")
			if err != nil {
				return err
			}
			workflow, err := cmdFlagValue(cmd, "workflow")
			if err != nil {
				return err
			}
			return executeSiteAssignRole(root, session, assignment, role, workflow)
		},
	}
	cmd.Flags().String("root", ".", "site server root")
	cmd.Flags().String("session", "", "session id")
	cmd.Flags().String("assignment", "", "assignment id")
	cmd.Flags().String("role", "", "role")
	cmd.Flags().String("workflow", "", "workflow path inside release bundle")
	return cmd
}

func newSiteAssignNodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Override assignment for a specific node",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			session, err := cmdFlagValue(cmd, "session")
			if err != nil {
				return err
			}
			assignment, err := cmdFlagValue(cmd, "assignment")
			if err != nil {
				return err
			}
			node, err := cmdFlagValue(cmd, "node")
			if err != nil {
				return err
			}
			role, err := cmdFlagValue(cmd, "role")
			if err != nil {
				return err
			}
			workflow, err := cmdFlagValue(cmd, "workflow")
			if err != nil {
				return err
			}
			return executeSiteAssignNode(root, session, assignment, node, role, workflow)
		},
	}
	cmd.Flags().String("root", ".", "site server root")
	cmd.Flags().String("session", "", "session id")
	cmd.Flags().String("assignment", "", "assignment id")
	cmd.Flags().String("node", "", "node id")
	cmd.Flags().String("role", "", "role (optional)")
	cmd.Flags().String("workflow", "", "workflow path inside release bundle")
	return cmd
}

func newSiteStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show release and session status summaries",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			output, err := cmdFlagValue(cmd, "output")
			if err != nil {
				return err
			}
			return executeSiteStatus(root, output)
		},
	}
	cmd.Flags().String("root", ".", "site server root")
	cmd.Flags().StringP("output", "o", "text", "output format (text|json)")
	return cmd
}
