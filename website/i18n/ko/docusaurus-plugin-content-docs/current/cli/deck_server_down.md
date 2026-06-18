---
source: docs/cli/deck_server_down.md
source_hash: 77c65216cd19d3507e2f0a0bb6545cb2de05bca5
---
## deck server down

로컬 서버 데몬을 중지합니다.

```
deck server down [flags]
```

### Options

```
  -h, --help          help for down
      --unit string   daemon unit/name to stop (default "deck-server")
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck server](deck_server.md)	 - 로컬 콘텐츠 서버를 실행하고 원격 조회 기본값을 관리합니다
