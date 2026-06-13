---
source: docs/cli/deck_server_remote.md
source_hash: cae8f21db40e4dfbf8be34adb5ef95432b0ab9d3
---
## deck server remote

시나리오 조회를 위해 저장된 원격 서버 URL을 관리합니다

```
deck server remote [flags]
```

### Options

```
  -h, --help   help for remote
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck server](deck_server.md)	 - 로컬 콘텐츠 서버를 실행하고 원격 조회 기본값을 관리합니다
* [deck server remote set](deck_server_remote_set.md)	 - 시나리오 조회를 위한 기본 원격 서버 URL을 저장합니다
* [deck server remote show](deck_server_remote_show.md)	 - 적용 중인 저장된 원격 서버 URL을 표시합니다
* [deck server remote unset](deck_server_remote_unset.md)	 - 저장된 원격 서버 URL을 지웁니다
