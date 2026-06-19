---
source: docs/cli/deck_lint.md
source_hash: 2b79f1d3e655f1bda1b0b42a2f62250471b97fec
---
## deck lint

워크플로 트리 또는 단일 워크플로 파일을 lint합니다

```
deck lint [scenario] [flags]
```

### Options

```
  -h, --help                help for lint
  -o, --output string       output format (text|json) (default "text")
      --root string         workspace root containing workflows/ (default ".")
  -f, --vars-file strings   vars file overlay relative to workflows/ (repeatable)
      --workflow string     path or URL to workflow file
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck](deck.md)	 - deck
