---
source: docs/cli/deck_ask_config.md
source_hash: 44a4363e1d317d8dfce3b16a68ff504b280e4ac4
---
## deck ask config

전역 ask config 기본값과 API 자격 증명을 관리합니다

```
deck ask config [flags]
```

### Options

```
  -h, --help   help for config
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck ask](deck_ask.md)	 - (실험적) 워크플로 작성 및 검토를 위한 AI 헬퍼
* [deck ask config health](deck_ask_config_health.md)	 - 내장 ask 증강 프로바이더를 점검합니다
* [deck ask config set](deck_ask_config_set.md)	 - ask config 기본값과 api 키를 저장합니다
* [deck ask config show](deck_ask_config_show.md)	 - 적용된 ask 프로바이더, 모델, 마스킹된 키 소스를 표시합니다
* [deck ask config unset](deck_ask_config_unset.md)	 - XDG config에서 저장된 ask config 설정을 지웁니다
