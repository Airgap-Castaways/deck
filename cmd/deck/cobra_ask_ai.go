//go:build ai

package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Airgap-Castaways/deck/internal/askauth"
	"github.com/Airgap-Castaways/deck/internal/askcli"
	"github.com/Airgap-Castaways/deck/internal/askconfig"
	"github.com/Airgap-Castaways/deck/internal/askcontext"
	"github.com/Airgap-Castaways/deck/internal/askprovider"
	openaiprovider "github.com/Airgap-Castaways/deck/internal/askprovider/openai"
)

var newAskBackend = func() askprovider.Client {
	return openaiprovider.New()
}

func newAskCommand() *cobra.Command {
	var fromPath string
	var write bool
	var review bool
	var planName string
	var planDir string
	var maxIterations int
	var provider string
	var model string
	var endpoint string
	meta := askcontext.AskCommandMeta()

	cmd := &cobra.Command{
		Use:   "ask [request]",
		Short: meta.Short,
		Example: strings.Join([]string{
			`  deck ask "explain what workflows/scenarios/apply.yaml does"`,
			`  deck ask --write "create an air-gapped rhel9 kubeadm cluster workflow"`,
			`  deck ask plan "create an air-gapped rhel9 kubeadm cluster workflow"`,
		}, "\n"),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			request := strings.TrimSpace(strings.Join(args, " "))
			return askcli.Execute(cmd.Context(), askcli.Options{
				Root:          ".",
				Prompt:        request,
				FromPath:      fromPath,
				PlanName:      planName,
				PlanDir:       planDir,
				Write:         write,
				Review:        review,
				MaxIterations: maxIterations,
				Provider:      provider,
				Model:         model,
				Endpoint:      endpoint,
				Stdout:        cmd.OutOrStdout(),
				Stderr:        cmd.ErrOrStderr(),
			}, newAskBackend())
		},
	}
	cmd.Flags().StringVar(&fromPath, "from", "", "load additional request details from a text or markdown file")
	cmd.Flags().BoolVar(&write, "write", false, "write generated workflow changes into the current workspace")
	cmd.Flags().BoolVar(&review, "review", false, "review the current workspace without writing files")
	cmd.Flags().IntVar(&maxIterations, "max-iterations", 0, "max repair attempts for draft/refine routes (0 uses route default)")
	cmd.Flags().StringVar(&provider, "provider", "", "override the configured ask provider for this run")
	cmd.Flags().StringVar(&model, "model", "", "override the configured ask model for this run")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "override the configured ask provider endpoint for this run")
	cmd.Flags().StringVar(&planName, "plan-name", "", "optional plan artifact name used by ask plan")
	cmd.Flags().StringVar(&planDir, "plan-dir", ".deck/plan", "directory for ask plan artifacts")

	cmd.AddCommand(newAskPlanCommand())
	cmd.AddCommand(newAskConfigCommand())
	cmd.AddCommand(newAskLoginCommand(), newAskLogoutCommand(), newAskStatusCommand())
	return cmd
}

func newAskPlanCommand() *cobra.Command {
	var fromPath string
	var planName string
	var planDir string
	var provider string
	var model string
	var endpoint string
	meta := askcontext.AskCommandMeta()
	cmd := &cobra.Command{
		Use:   "plan [request]",
		Short: meta.Plan.Short,
		Long:  meta.Plan.Long,
		Example: strings.Join([]string{
			`  deck ask plan "create an air-gapped rhel9 kubeadm cluster workflow"`,
			`  deck ask plan --plan-name kubeadm-ha "create a 3-node kubeadm workflow"`,
		}, "\n"),
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			request := strings.TrimSpace(strings.Join(args, " "))
			return askcli.Execute(cmd.Context(), askcli.Options{
				Root:     ".",
				Prompt:   request,
				FromPath: fromPath,
				PlanOnly: true,
				PlanName: planName,
				PlanDir:  planDir,
				Provider: provider,
				Model:    model,
				Endpoint: endpoint,
				Stdout:   cmd.OutOrStdout(),
				Stderr:   cmd.ErrOrStderr(),
			}, newAskBackend())
		},
	}
	cmd.Flags().StringVar(&fromPath, "from", "", "load additional request details from a text or markdown file")
	cmd.Flags().StringVar(&planName, "plan-name", "", "optional plan artifact name")
	cmd.Flags().StringVar(&planDir, "plan-dir", ".deck/plan", "directory for ask plan artifacts")
	cmd.Flags().StringVar(&provider, "provider", "", "override the configured ask provider for this run")
	cmd.Flags().StringVar(&model, "model", "", "override the configured ask model for this run")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "override the configured ask provider endpoint for this run")
	return cmd
}

func newAskConfigCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: askcontext.AskCommandMeta().Config.Short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAskConfigSetCommand(), newAskConfigShowCommand(), newAskConfigUnsetCommand())
	return cmd
}

func newAskConfigSetCommand() *cobra.Command {
	var apiKey string
	var oauthToken string
	var provider string
	var model string
	var endpoint string
	var logLevel string
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Save ask config defaults and api key",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			settings, err := askconfig.LoadStored()
			if err != nil {
				return err
			}
			updated := settings
			if value := strings.TrimSpace(apiKey); value != "" {
				updated.APIKey = value
			}
			if value := strings.TrimSpace(oauthToken); value != "" {
				updated.OAuthToken = value
			}
			if value := strings.TrimSpace(provider); value != "" {
				updated.Provider = value
			}
			if value := strings.TrimSpace(model); value != "" {
				updated.Model = value
			}
			if value := strings.TrimSpace(endpoint); value != "" {
				updated.Endpoint = value
			}
			if value := strings.TrimSpace(logLevel); value != "" {
				updated.LogLevel = value
			}
			changed := settings.Provider != updated.Provider ||
				settings.Model != updated.Model ||
				settings.APIKey != updated.APIKey ||
				settings.OAuthToken != updated.OAuthToken ||
				settings.Endpoint != updated.Endpoint ||
				settings.LogLevel != updated.LogLevel
			if !changed {
				return fmt.Errorf("ask config set requires at least one of --api-key, --oauth-token, --provider, --model, --endpoint, or --log-level")
			}
			if err := askconfig.SaveStored(updated); err != nil {
				return err
			}
			return stdoutPrintln("ask config saved")
		},
	}
	cmd.Flags().StringVar(&apiKey, "api-key", "", "save the ask api key in XDG config")
	cmd.Flags().StringVar(&oauthToken, "oauth-token", "", "save the ask oauth bearer token in XDG config")
	cmd.Flags().StringVar(&provider, "provider", "", "save the default ask provider")
	cmd.Flags().StringVar(&model, "model", "", "save the default ask model")
	cmd.Flags().StringVar(&endpoint, "endpoint", "", "save the default ask provider endpoint")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "save the ask terminal log level (basic, debug, trace)")
	return cmd
}

func newAskConfigShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the effective ask provider, model, and masked key source",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			effective, err := askconfig.ResolveEffective(askconfig.Settings{})
			if err != nil {
				return err
			}
			if err := stdoutPrintf("provider=%s\n", effective.Provider); err != nil {
				return err
			}
			if err := stdoutPrintf("providerSource=%s\n", effective.ProviderSource); err != nil {
				return err
			}
			if err := stdoutPrintf("model=%s\n", effective.Model); err != nil {
				return err
			}
			if err := stdoutPrintf("modelSource=%s\n", effective.ModelSource); err != nil {
				return err
			}
			if err := stdoutPrintf("endpoint=%s\n", effective.Endpoint); err != nil {
				return err
			}
			if err := stdoutPrintf("endpointSource=%s\n", effective.EndpointSource); err != nil {
				return err
			}
			if err := stdoutPrintf("logLevel=%s\n", effective.LogLevel); err != nil {
				return err
			}
			if err := stdoutPrintf("mcpEnabled=%t\n", effective.MCP.Enabled); err != nil {
				return err
			}
			if err := stdoutPrintf("lspEnabled=%t\n", effective.LSP.Enabled); err != nil {
				return err
			}
			if err := stdoutPrintf("apiKey=%s\n", askconfig.MaskAPIKey(effective.APIKey)); err != nil {
				return err
			}
			if err := stdoutPrintf("apiKeySource=%s\n", effective.APIKeySource); err != nil {
				return err
			}
			if err := stdoutPrintf("oauthToken=%s\n", askconfig.MaskAPIKey(effective.OAuthToken)); err != nil {
				return err
			}
			if err := stdoutPrintf("oauthTokenSource=%s\n", effective.OAuthTokenSource); err != nil {
				return err
			}
			return stdoutPrintf("authStatus=%s\n", effective.AuthStatus)
		},
	}
	return cmd
}

func newAskConfigUnsetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unset",
		Short: "Clear saved ask config settings from XDG config",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := askconfig.ClearStored(); err != nil {
				return err
			}
			return stdoutPrintln("ask config cleared")
		},
	}
	return cmd
}

func newAskLoginCommand() *cobra.Command {
	var provider string
	var oauthToken string
	var refreshToken string
	var accountEmail string
	var expiresAt string
	var headless bool
	var noBrowser bool
	var stdinToken bool
	var callbackPort int
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate ask with OpenAI Codex OAuth",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			providerName, err := resolveAskProvider(provider)
			if err != nil {
				return err
			}
			if err := requireOpenAIProvider(providerName); err != nil {
				return err
			}
			accessToken := strings.TrimSpace(oauthToken)
			if stdinToken {
				raw, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read oauth token from stdin: %w", err)
				}
				accessToken = strings.TrimSpace(string(raw))
			}
			if accessToken == "" {
				var session askauth.Session
				var loginErr error
				if _, err := fmt.Fprintln(os.Stderr, "Starting OpenAI Codex login..."); err != nil {
					return err
				}
				authOpts := askauth.OpenAICodexOptions{CallbackPort: callbackPort, OpenBrowser: !noBrowser, Writer: os.Stderr}
				if headless {
					session, loginErr = askauth.LoginOpenAICodexDevice(cmd.Context(), authOpts)
				} else {
					session, loginErr = askauth.LoginOpenAICodexBrowser(cmd.Context(), authOpts)
				}
				if loginErr != nil {
					return loginErr
				}
				if accountEmail != "" {
					session.AccountEmail = strings.TrimSpace(accountEmail)
				}
				if refreshToken != "" {
					session.RefreshToken = strings.TrimSpace(refreshToken)
				}
				if strings.TrimSpace(expiresAt) != "" {
					parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(expiresAt))
					if err != nil {
						return fmt.Errorf("parse --expires-at: %w", err)
					}
					session.ExpiresAt = parsed.UTC()
				}
				if err := askauth.Save(session); err != nil {
					return err
				}
				return stdoutPrintf("ask login saved provider=%s account=%s expiresAt=%s\n", providerName, fallbackValue(session.AccountEmail, "unknown"), formatExpiry(session.ExpiresAt))
			}
			session := askauth.Session{Provider: providerName, AccessToken: accessToken, RefreshToken: strings.TrimSpace(refreshToken), AccountEmail: strings.TrimSpace(accountEmail)}
			if strings.TrimSpace(expiresAt) != "" {
				parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(expiresAt))
				if err != nil {
					return fmt.Errorf("parse --expires-at: %w", err)
				}
				session.ExpiresAt = parsed.UTC()
			}
			if err := askauth.Save(session); err != nil {
				return err
			}
			return stdoutPrintf("ask login saved provider=%s account=%s expiresAt=%s\n", providerName, fallbackValue(session.AccountEmail, "unknown"), formatExpiry(session.ExpiresAt))
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "provider to associate with this oauth session")
	cmd.Flags().StringVar(&oauthToken, "oauth-token", "", "oauth bearer token to save")
	cmd.Flags().StringVar(&refreshToken, "refresh-token", "", "optional refresh token to store for future flows")
	cmd.Flags().StringVar(&accountEmail, "account-email", "", "optional account email label for status output")
	cmd.Flags().StringVar(&expiresAt, "expires-at", "", "optional RFC3339 access token expiry time")
	cmd.Flags().BoolVar(&stdinToken, "stdin-token", false, "read the oauth bearer token from stdin for headless use")
	cmd.Flags().BoolVar(&headless, "headless", false, "use OpenAI Codex device login instead of browser callback login")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "print the login URL instead of opening it automatically")
	cmd.Flags().IntVar(&callbackPort, "callback-port", 1455, "local callback port for browser-based OAuth login")
	return cmd
}

func newAskLogoutCommand() *cobra.Command {
	var provider string
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Delete the saved OAuth session for ask",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			providerName, err := resolveAskProvider(provider)
			if err != nil {
				return err
			}
			if err := requireOpenAIProvider(providerName); err != nil {
				return err
			}
			if err := askauth.Delete(providerName); err != nil {
				return err
			}
			return stdoutPrintf("ask logout removed provider=%s\n", providerName)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "provider whose saved oauth session should be deleted")
	return cmd
}

func newAskStatusCommand() *cobra.Command {
	var provider string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show saved ask OAuth session status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			providerName, err := resolveAskProvider(provider)
			if err != nil {
				return err
			}
			if err := requireOpenAIProvider(providerName); err != nil {
				return err
			}
			effective, err := askconfig.ResolveEffective(askconfig.Settings{Provider: providerName})
			if err != nil {
				return err
			}
			session, status, ok, err := askauth.SessionStatus(providerName)
			if err != nil {
				return err
			}
			if err := stdoutPrintf("provider=%s\n", providerName); err != nil {
				return err
			}
			if err := stdoutPrintf("authenticated=%t\n", ok); err != nil {
				return err
			}
			if err := stdoutPrintf("oauthTokenSource=%s\n", effective.OAuthTokenSource); err != nil {
				return err
			}
			if !ok {
				if err := stdoutPrintf("status=missing\n"); err != nil {
					return err
				}
				return stdoutPrintf("accountEmail=\n")
			}
			if err := stdoutPrintf("status=%s\n", status); err != nil {
				return err
			}
			if err := stdoutPrintf("accountEmail=%s\n", session.AccountEmail); err != nil {
				return err
			}
			if err := stdoutPrintf("accountID=%s\n", session.AccountID); err != nil {
				return err
			}
			if err := stdoutPrintf("expiresAt=%s\n", formatExpiry(session.ExpiresAt)); err != nil {
				return err
			}
			return stdoutPrintf("hasRefreshToken=%t\n", strings.TrimSpace(session.RefreshToken) != "")
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "provider whose oauth session should be inspected")
	return cmd
}

func formatExpiry(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.UTC().Format(time.RFC3339)
}

func fallbackValue(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func resolveAskProvider(flagValue string) (string, error) {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(os.Getenv("DECK_ASK_PROVIDER")); value != "" {
		return value, nil
	}
	stored, err := askconfig.LoadStored()
	if err != nil {
		return "", err
	}
	if value := strings.TrimSpace(stored.Provider); value != "" {
		return value, nil
	}
	return "openai", nil
}

func requireOpenAIProvider(provider string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" || provider == "openai" {
		return nil
	}
	return fmt.Errorf("ask oauth login currently supports only provider %q", "openai")
}
