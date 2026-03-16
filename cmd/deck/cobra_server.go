package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/taedi90/deck/internal/executil"
)

func newServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run and inspect the local content server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newServerUpCommand(),
		newServerDownCommand(),
		newServerHealthCommand(),
		newServerLogsCommand(),
	)

	return cmd
}

func newServerUpCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start the local bundle server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			addr, err := cmdFlagValue(cmd, "addr")
			if err != nil {
				return err
			}
			auditMaxSize, err := cmdFlagIntValue(cmd, "audit-max-size-mb")
			if err != nil {
				return err
			}
			auditMaxFiles, err := cmdFlagIntValue(cmd, "audit-max-files")
			if err != nil {
				return err
			}
			tlsCert, err := cmdFlagValue(cmd, "tls-cert")
			if err != nil {
				return err
			}
			tlsKey, err := cmdFlagValue(cmd, "tls-key")
			if err != nil {
				return err
			}
			tlsSelfSigned, err := cmdFlagBoolValue(cmd, "tls-self-signed")
			if err != nil {
				return err
			}
			daemon, err := cmdFlagBoolValue(cmd, "daemon")
			if err != nil {
				return err
			}
			unit, err := cmdFlagValue(cmd, "unit")
			if err != nil {
				return err
			}
			return executeServerUp(serverUpOptions{
				root:          root,
				addr:          addr,
				auditMaxSize:  auditMaxSize,
				auditMaxFiles: auditMaxFiles,
				tlsCert:       tlsCert,
				tlsKey:        tlsKey,
				tlsSelfSigned: tlsSelfSigned,
				daemon:        daemon,
				unit:          unit,
			})
		},
	}
	cmd.Flags().String("root", ".", "server content root")
	cmd.Flags().String("addr", ":8080", "server listen address")
	cmd.Flags().Int("audit-max-size-mb", 50, "max audit log size in MB before rotation")
	cmd.Flags().Int("audit-max-files", 10, "max retained rotated audit files")
	cmd.Flags().String("tls-cert", "", "TLS certificate path")
	cmd.Flags().String("tls-key", "", "TLS private key path")
	cmd.Flags().Bool("tls-self-signed", false, "auto-generate and use self-signed TLS cert")
	cmd.Flags().BoolP("daemon", "d", false, "run as a transient systemd service")
	cmd.Flags().String("unit", "deck-server", "systemd unit name for daemon mode")
	return cmd
}

func newServerDownCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop the local server daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			unit, err := cmdFlagValue(cmd, "unit")
			if err != nil {
				return err
			}
			return executeServerDown(unit)
		},
	}
	cmd.Flags().String("unit", "deck-server", "systemd unit name to stop")
	return cmd
}

func newServerHealthCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Probe the configured or explicit server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := cmdFlagValue(cmd, "server")
			if err != nil {
				return err
			}
			return executeHealth(server)
		},
	}
	cmd.Flags().String("server", "", "server base URL (defaults to the saved source URL)")
	return cmd
}

func newServerLogsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Read local server audit logs from file or journal",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root, err := cmdFlagValue(cmd, "root")
			if err != nil {
				return err
			}
			source, err := cmdFlagValue(cmd, "source")
			if err != nil {
				return err
			}
			path, err := cmdFlagValue(cmd, "path")
			if err != nil {
				return err
			}
			unit, err := cmdFlagValue(cmd, "unit")
			if err != nil {
				return err
			}
			output, err := cmdFlagValue(cmd, "output")
			if err != nil {
				return err
			}
			return executeLogs(root, source, path, unit, output)
		},
	}
	cmd.Flags().String("root", ".", "serve root directory")
	cmd.Flags().String("source", "file", "log source (file|journal|both)")
	cmd.Flags().String("path", "", "explicit audit log file path")
	cmd.Flags().String("unit", "deck-server.service", "systemd unit for journal logs")
	cmd.Flags().StringP("output", "o", "text", "output format (text|json)")
	return cmd
}

type serverUpOptions struct {
	root          string
	addr          string
	auditMaxSize  int
	auditMaxFiles int
	tlsCert       string
	tlsKey        string
	tlsSelfSigned bool
	daemon        bool
	unit          string
}

func executeServerUp(opts serverUpOptions) error {
	if err := validateServerUpDaemonMode(opts); err != nil {
		return err
	}
	if !opts.daemon {
		return executeServe(opts.root, opts.addr, opts.auditMaxSize, opts.auditMaxFiles, opts.tlsCert, opts.tlsKey, opts.tlsSelfSigned)
	}
	return runServerDaemon(opts)
}

func executeServerDown(unit string) error {
	resolvedUnit := normalizeServerUnitName(unit)
	raw, err := executil.CombinedOutputSystemctl(context.Background(), "stop", resolvedUnit)
	if err != nil {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("server down: %s", msg)
	}
	return stdoutPrintf("server down: ok (%s)\n", resolvedUnit)
}

func runServerDaemon(opts serverUpOptions) error {
	resolvedUnit := normalizeServerUnitBaseName(opts.unit)
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("server up: resolve executable: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("server up: resolve working directory: %w", err)
	}
	args := []string{
		"--unit", resolvedUnit,
		"--property", "WorkingDirectory=" + cwd,
		"--service-type=simple",
		execPath,
		"server", "up",
		"--root", opts.root,
		"--addr", opts.addr,
		"--audit-max-size-mb", fmt.Sprintf("%d", opts.auditMaxSize),
		"--audit-max-files", fmt.Sprintf("%d", opts.auditMaxFiles),
	}
	if strings.TrimSpace(opts.tlsCert) != "" {
		args = append(args, "--tls-cert", opts.tlsCert)
	}
	if strings.TrimSpace(opts.tlsKey) != "" {
		args = append(args, "--tls-key", opts.tlsKey)
	}
	if opts.tlsSelfSigned {
		args = append(args, "--tls-self-signed")
	}
	raw, err := executil.CombinedOutputSystemdRun(context.Background(), args...)
	if err != nil {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("server up: %s", msg)
	}
	if err := stdoutPrintf("server up: ok (%s.service)\n", resolvedUnit); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	return stdoutPrintf("%s\n", trimmed)
}

func normalizeServerUnitBaseName(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "deck-server"
	}
	return strings.TrimSuffix(trimmed, ".service")
}

func normalizeServerUnitName(raw string) string {
	base := normalizeServerUnitBaseName(raw)
	if strings.HasSuffix(base, ".service") {
		return base
	}
	return base + ".service"
}

func validateServerUpDaemonMode(opts serverUpOptions) error {
	if !opts.daemon {
		return nil
	}
	if _, err := executil.LookPathSystemdRun(); err != nil {
		return errors.New("server up: systemd-run not found")
	}
	if _, err := executil.LookPathSystemctl(); err != nil {
		return errors.New("server up: systemctl not found")
	}
	if strings.TrimSpace(opts.unit) == "" {
		return errors.New("server up: --unit must not be empty")
	}
	if strings.ContainsRune(opts.unit, filepath.Separator) {
		return errors.New("server up: --unit must be a unit name, not a path")
	}
	return nil
}
