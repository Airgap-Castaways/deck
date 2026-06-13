---
source: docs/cli/deck_state_clear.md
source_hash: 5a1b2dc663add82076f2f59243b5c05d377ad4f4
---
```markdown
## deck state clear

적용 상태를 삭제합니다

```
deck state clear [flags]
```

### Options

```
      --all                  delete all apply state files in the selected state directory
  -h, --help                 help for clear
  -o, --output string        output format (text|json) (default "text")
      --root string          local workflow root containing workflows/
      --scenario string      scenario name
      --server string        remote workflow server URL
      --source string        scenario source (local|server) (default "local")
      --state-dir string     directory for apply state files (overrides local .deck/state/apply or remote XDG state)
      --var stringToString   set variable override (key=value), repeatable
  -f, --vars-file strings    vars overlay path relative to the selected workflow root (workflows/), repeatable
      --workflow string      path or URL to workflow file
      --yes                  confirm state deletion without prompting
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck state](deck_state.md)	 - 저장된 적용 상태를 검사하고 삭제합니다
```
