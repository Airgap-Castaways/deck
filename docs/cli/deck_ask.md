## deck ask

(Experimental) AI helper for drafting and reviewing workflows

```
deck ask [request] [flags]
```

### Examples

```
  deck ask "explain what workflows/scenarios/apply.yaml does"
  deck ask --create "create an air-gapped rhel9 single-node kubeadm workflow"
  deck ask --edit "refactor workflows/scenarios/apply.yaml to use workflows/vars.yaml"
  deck ask plan "create an air-gapped rhel9 single-node kubeadm workflow"
```

### Options

```
      --answer stringArray   apply plan clarification answers as key=value when resuming from a plan artifact
      --create               treat the request as new workflow authoring
      --edit                 treat the request as workflow refinement
      --endpoint string      override the configured ask provider endpoint for this run
      --from string          load additional request details from a text or markdown file
  -h, --help                 help for ask
      --max-iterations int   max repair attempts for draft/refine routes (0 uses route default)
      --model string         override the configured ask model for this run
      --plan-dir string      directory for ask plan artifacts (default ".deck/plan")
      --plan-name string     optional plan artifact name used by ask plan
      --provider string      override the configured ask provider for this run
      --review               review the current workspace without writing files
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck](deck.md)	 - deck
* [deck ask config](deck_ask_config.md)	 - Manage global ask config defaults and API credentials
* [deck ask login](deck_ask_login.md)	 - Authenticate ask with OpenAI Codex OAuth
* [deck ask logout](deck_ask_logout.md)	 - Delete the saved OAuth session for ask
* [deck ask plan](deck_ask_plan.md)	 - Generate an ask plan artifact without writing workflow files
* [deck ask status](deck_ask_status.md)	 - Show saved ask OAuth session status

