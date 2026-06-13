---
source: docs/cli/deck_state_list.md
source_hash: dd61c5342e9ffc2bca4db8b4148ab51af5e2798e
---
## deck state list

적용 상태 파일 목록을 표시합니다

```
deck state list [flags]
```

### Options

```
  -h, --help               help for list
  -o, --output string      output format (text|json) (default "text")
      --state-dir string   directory for apply state files (defaults to .deck/state/apply)
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck state](deck_state.md)	 - 저장된 적용 상태를 검사하고 지웁니다
