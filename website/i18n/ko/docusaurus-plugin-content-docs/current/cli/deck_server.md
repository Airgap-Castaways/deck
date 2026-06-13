---
source: docs/cli/deck_server.md
source_hash: 24e329a1571360e2846b587957e66558c6b0fc3f
---
## deck server

로컬 콘텐츠 서버를 실행하고 원격 조회 기본값을 관리합니다

```
deck server [flags]
```

### Options

```
  -h, --help   help for server
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck](deck.md)	 - deck
* [deck server down](deck_server_down.md)	 - 로컬 서버 데몬을 중지합니다
* [deck server health](deck_server_health.md)	 - 명시한 서버 또는 저장된 원격 서버 URL을 점검합니다
* [deck server logs](deck_server_logs.md)	 - 파일 또는 저널에서 로컬 서버 감사 로그를 읽습니다
* [deck server remote](deck_server_remote.md)	 - 시나리오 조회에 사용할 저장된 원격 서버 URL을 관리합니다
* [deck server up](deck_server_up.md)	 - 로컬 번들 서버를 시작합니다
