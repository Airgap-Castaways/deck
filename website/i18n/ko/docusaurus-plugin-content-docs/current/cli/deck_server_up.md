---
source: docs/cli/deck_server_up.md
source_hash: 8419af28e3efdd01b6ca3f0e3aeecb53fd305800
---
```markdown
## deck server up

로컬 번들 서버를 시작합니다

```
deck server up [flags]
```

### Options

```
      --addr string             server listen address (default ":8080")
      --audit-max-files int     max retained rotated audit files (default 10)
      --audit-max-size-mb int   max audit log size in MB before rotation (default 50)
  -d, --daemon                  run as a background daemon (systemd on Linux; detached process on macOS/Windows; see docs/server/daemon.md)
  -h, --help                    help for up
      --root string             server content root (default ".")
      --tls-cert string         TLS certificate path
      --tls-key string          TLS private key path
      --tls-self-signed         auto-generate and use self-signed TLS cert
      --unit string             daemon unit/name (default "deck-server")
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck server](deck_server.md)	 - 로컬 콘텐츠 서버를 실행하고 원격 조회 기본값을 관리합니다
```
