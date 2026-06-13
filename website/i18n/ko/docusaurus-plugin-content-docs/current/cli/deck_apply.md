---
source: docs/cli/deck_apply.md
source_hash: 40f741622fe7a9f5d0118114283ff70874b0f939
---
* [deck](deck.md)	 - deck

## deck apply

apply 파일을 번들에 대해 실행합니다

```
deck apply [workflow] [bundle] [flags]
```

### Options

```
      --dry-run              print apply plan without executing steps
      --fresh                clear saved apply state before execution
  -h, --help                 help for apply
      --non-interactive      fail or use defaults for operator interaction steps instead of prompting
      --phase string         phase name to execute (defaults to all phases)
      --root string          local workflow root containing workflows/
      --scenario string      scenario name to execute
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

* [deck](deck.md)	 - deck
