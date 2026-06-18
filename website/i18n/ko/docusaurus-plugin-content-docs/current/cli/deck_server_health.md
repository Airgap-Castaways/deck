---
source: docs/cli/deck_server_health.md
source_hash: 7ad72554de49117c62b9a15cab0647db3c0ac464
---
```
## deck server health

명시적으로 지정한 서버 또는 저장된 원격 서버 URL의 상태를 점검합니다

```
deck server health [flags]
```

### Options

```
  -h, --help            help for health
  -o, --output string   output format (text|json) (default "text")
      --server string   server base URL (defaults to the saved remote server URL)
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### 함께 보기

* [deck server](deck_server.md)	 - 로컬 콘텐츠 서버를 실행하고 원격 조회 기본값을 관리합니다
```
