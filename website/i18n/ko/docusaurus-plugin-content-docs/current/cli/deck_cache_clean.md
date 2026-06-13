---
source: docs/cli/deck_cache_clean.md
source_hash: 9e3fdc521cf5867eb4407c498e7d88eb234a75f8
---
```markdown
## deck cache clean

캐시된 항목을 삭제하며, 선택적으로 보관 기간 기준으로 삭제

```
deck cache clean [flags]
```

### Options

```
      --dry-run             print deletion plan without deleting
  -h, --help                help for clean
      --older-than string   delete entries not modified within this duration (e.g. 30d, 24h)
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck cache](deck_cache.md)	 - deck 캐시 데이터를 조회하거나 정리
```
