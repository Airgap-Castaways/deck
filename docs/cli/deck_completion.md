## deck completion

Generate shell completion scripts

### Synopsis

Generate shell completion scripts.

Immediate sourcing:
  bash: source <(deck completion bash)
  zsh:  source <(deck completion zsh)
  fish: deck completion fish | source

Persistent setup:
  bash: add 'source <(deck completion bash)' to ~/.bashrc
  zsh:  add 'source <(deck completion zsh)' to ~/.zshrc
  fish: deck completion fish > ~/.config/fish/completions/deck.fish

```
deck completion <bash|zsh|fish|powershell>
```

### Options

```
  -h, --help   help for completion
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck](deck.md)	 - deck

