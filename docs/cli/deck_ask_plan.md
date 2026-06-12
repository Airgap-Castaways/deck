## deck ask plan

Generate an ask plan artifact without writing workflow files

### Synopsis

Generate a reusable planning artifact under .deck/plan without writing workflow files. This mode is intended for draft/refine style authoring requests.

```
deck ask plan [request] [flags]
```

### Examples

```
  deck ask plan "create an air-gapped rhel9 single-node kubeadm workflow"
  deck ask plan --plan-name kubeadm-ha "create a 3-node kubeadm workflow"
```

### Options

```
      --answer stringArray   apply plan clarification answers as key=value when resuming from a saved plan artifact
      --endpoint string      override the configured ask provider endpoint for this run
      --from string          load additional request details from a text or markdown file
  -h, --help                 help for plan
      --model string         override the configured ask model for this run
      --plan-dir string      directory for ask plan artifacts (default ".deck/plan")
      --plan-name string     optional plan artifact name
      --provider string      override the configured ask provider for this run
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck ask](deck_ask.md)	 - (Experimental) AI helper for drafting and reviewing workflows

