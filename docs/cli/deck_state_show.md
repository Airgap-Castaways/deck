## deck state show

Show apply state selected by workflow inputs

```
deck state show [flags]
```

### Options

```
  -h, --help                 help for show
  -o, --output string        output format (text|json) (default "text")
      --root string          local workflow root containing workflows/
      --scenario string      scenario name
      --server string        remote workflow server URL
      --source string        scenario source (local|server) (default "local")
      --state-dir string     directory for apply state files (overrides local .deck/state/apply or remote XDG state)
      --var stringToString   set variable override (key=value), repeatable
  -f, --vars-file strings    vars overlay path relative to the selected workflow root (workflows/), repeatable
      --workflow string      path or URL to workflow file
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck state](deck_state.md)	 - Inspect and clear saved apply state

