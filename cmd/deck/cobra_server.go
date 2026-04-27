package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Airgap-Castaways/deck/internal/executil"
	ctrllogs "github.com/Airgap-Castaways/deck/internal/logs"
)

func newServerCommand(env *cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run the local content server and manage remote lookup defaults",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newServerUpCommand(env),
		newServerDownCommand(env),
		newServerHealthCommand(env),
		newServerLogsCommand(env),
		newServerRemoteCommand(env),
	)

	return cmd
}

func newServerUpCommand(env *cliEnv) *cobra.Command {
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
			return executeServerUp(env, cmd.Context(), serverUpOptions{
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

func newServerDownCommand(env *cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Stop the local server daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			unit, err := cmdFlagValue(cmd, "unit")
			if err != nil {
				return err
			}
			return executeServerDown(env, cmd.Context(), unit)
		},
	}
	cmd.Flags().String("unit", "deck-server", "systemd unit name to stop")
	return cmd
}

func newServerHealthCommand(env *cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "health",
		Short: "Probe an explicit server or the saved remote server URL",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, err := cmdFlagValue(cmd, "server")
			if err != nil {
				return err
			}
			output, err := cmdFlagValue(cmd, "output")
			if err != nil {
				return err
			}
			return executeHealth(env, cmd.Context(), server, output)
		},
	}
	cmd.Flags().String("server", "", "server base URL (defaults to the saved remote server URL)")
	cmd.Flags().StringP("output", "o", "text", "output format (text|json)")
	return cmd
}

func newServerLogsCommand(env *cliEnv) *cobra.Command {
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
			return executeLogs(env, cmd.Context(), root, source, path, unit, output)
		},
	}
	cmd.Flags().String("root", ".", "serve root directory")
	cmd.Flags().String("source", "file", "log source (file|journal|both)")
	cmd.Flags().String("path", "", "explicit audit log file path")
	cmd.Flags().String("unit", "deck-server.service", "systemd unit for journal logs")
	cmd.Flags().StringP("output", "o", "text", "output format (text|json)")
	return cmd
}

func newServerRemoteCommand(env *cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Manage the saved remote server URL for scenario lookup",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newServerRemoteSetCommand(env),
		newServerRemoteShowCommand(env),
		newServerRemoteUnsetCommand(env),
	)

	return cmd
}

func newServerRemoteSetCommand(env *cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <url>",
		Short: "Save the default remote server URL for scenario lookup",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return executeServerRemoteSet(env, args[0])
		},
	}
	return cmd
}

func newServerRemoteShowCommand(env *cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the effective saved remote server URL",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return executeServerRemoteShow(env)
		},
	}
	return cmd
}

func newServerRemoteUnsetCommand(env *cliEnv) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Clear the saved remote server URL",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return executeServerRemoteUnset(env)
		},
	}
	return cmd
}

func executeServerRemoteSet(env *cliEnv, rawURL string) error {
	resolved := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if err := validateSourceURL(resolved); err != nil {
		return err
	}
	configPath, err := sourceDefaultsPath()
	if err != nil {
		return err
	}
	if err := env.verboseCLIEvent(1, ctrllogs.CLIEvent{Component: "server", Event: "remote_set", Attrs: map[string]any{"url": resolved, "config": configPath}}); err != nil {
		return err
	}
	if err := saveSourceDefaults(sourceDefaults{URL: resolved}); err != nil {
		return err
	}
	return env.stdoutPrintf("server remote set: %s\n", resolved)
}

func executeServerRemoteShow(env *cliEnv) error {
	configPath, err := sourceDefaultsPath()
	if err != nil {
		return err
	}
	resolved, source, err := resolveSourceURL("")
	if err != nil {
		return err
	}
	if err := env.verboseCLIEvent(1, ctrllogs.CLIEvent{Component: "server", Event: "remote_show", Attrs: map[string]any{"config": configPath, "resolved": displayValueOrDash(resolved), "origin": displayValueOrDash(source)}}); err != nil {
		return err
	}
	if resolved == "" {
		if err := env.stdoutPrintln("remote="); err != nil {
			return err
		}
		return env.stdoutPrintln("origin=none")
	}
	if err := env.stdoutPrintf("remote=%s\n", resolved); err != nil {
		return err
	}
	return env.stdoutPrintf("origin=%s\n", source)
}

func executeServerRemoteUnset(env *cliEnv) error {
	configPath, err := sourceDefaultsPath()
	if err != nil {
		return err
	}
	if err := env.verboseCLIEvent(1, ctrllogs.CLIEvent{Component: "server", Event: "remote_unset", Attrs: map[string]any{"config": configPath}}); err != nil {
		return err
	}
	if err := clearSourceDefaults(); err != nil {
		return err
	}
	return env.stdoutPrintln("server remote cleared")
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

func executeServerUp(env *cliEnv, ctx context.Context, opts serverUpOptions) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	if err := validateServerUpDaemonMode(opts); err != nil {
		return err
	}
	if !opts.daemon {
		return executeServe(env, ctx, opts.root, opts.addr, opts.auditMaxSize, opts.auditMaxFiles, opts.tlsCert, opts.tlsKey, opts.tlsSelfSigned)
	}
	return runServerDaemon(env, ctx, opts)
}

func executeServerDown(env *cliEnv, ctx context.Context, unit string) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	resolvedUnit := normalizeServerUnitName(unit)
	raw, err := executil.CombinedOutputSystemctl(ctx, "stop", resolvedUnit)
	if err != nil {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("server down: %s", msg)
	}
	return env.stdoutPrintf("server down: ok (%s)\n", resolvedUnit)
}

func runServerDaemon(env *cliEnv, ctx context.Context, opts serverUpOptions) error {
	if ctx == nil {
		return fmt.Errorf("context is nil")
	}
	resolvedUnit := normalizeServerUnitBaseName(opts.unit)
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("server up: resolve executable: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("server up: resolve working directory: %w", err)
	}
	args := buildServerDaemonArgs(resolvedUnit, execPath, cwd, opts)
	raw, err := executil.CombinedOutputSystemdRun(ctx, args...)
	if err != nil {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("server up: %s", msg)
	}
	if err := env.stdoutPrintf("server up: ok (%s.service)\n", resolvedUnit); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return nil
	}
	return env.stdoutPrintf("%s\n", trimmed)
}

func buildServerDaemonArgs(resolvedUnit string, execPath string, cwd string, opts serverUpOptions) []string {
	args := []string{
		"--unit", resolvedUnit,
		"--property", "WorkingDirectory=" + strings.ReplaceAll(cwd, "%", "%%"),
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
	return args
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
