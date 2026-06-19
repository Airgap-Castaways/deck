---
source: docs/cli/deck_plan_vars.md
source_hash: 9f1dd6061b69306446080f093842579e9849b7bd
---
## deck plan vars

유효 변수, 컨텍스트, 초기 런타임 값 표시

```
deck plan vars [flags]
```

### Options

```
      --command string       execution command to inspect (apply|prepare) (default "apply")
  -h, --help                 help for vars
  -o, --output string        output format (text|json) (default "text")
      --phase string         phase name to inspect (defaults to all phases)
      --root string          workflow root for --command apply or prepared bundle output directory for --command prepare (default "outputs")
      --scenario string      scenario name to inspect for apply
      --server string        remote workflow server URL for --command apply
      --source string        scenario source for apply (local|server) (default "local")
      --var stringToString   set variable override (key=value), repeatable
  -f, --vars-file strings    vars overlay path relative to the selected workflow root (workflows/), repeatable
      --workflow string      path or URL to apply workflow file
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck plan](deck_plan.md)	 - 계획된 적용 스텝 실행 표시
