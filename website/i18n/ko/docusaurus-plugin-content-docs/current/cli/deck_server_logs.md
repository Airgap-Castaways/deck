---
source: docs/cli/deck_server_logs.md
source_hash: 0c41248e4d73a6fa120ca2c82b20e47194209321
---
로컬 서버의 감사 로그를 파일이나 journal에서 읽습니다.

```
deck server logs [flags]
```

### Options

```
  -h, --help            help for logs
  -o, --output string   output format (text|json) (default "text")
      --path string     explicit audit log file path
      --root string     serve root directory (default ".")
      --source string   log source (file|journal|both) (default "file")
      --unit string     systemd unit for journal logs (default "deck-server.service")
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck server](deck_server.md)	 - 로컬 콘텐츠 서버를 실행하고 원격 조회 기본값을 관리합니다
