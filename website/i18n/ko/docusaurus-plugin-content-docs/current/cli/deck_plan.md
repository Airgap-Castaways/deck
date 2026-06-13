---
source: docs/cli/deck_plan.md
source_hash: d2fc1197171c81757ba0f182a9f4eda5e1f21f23
---
## deck plan

계획된 적용 스텝 실행을 표시합니다

```
deck plan [flags]
```

### Options

```
  -h, --help                 help for plan
  -o, --output string        output format (text|json) (default "text")
      --phase string         phase name to plan (defaults to all phases)
      --root string          local workflow root containing workflows/
      --scenario string      scenario name to plan
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
* [deck plan vars](deck_plan_vars.md)	 - 유효 vars, 컨텍스트, 초기 런타임 값을 표시합니다
