---
source: docs/cli/deck_ask_config_set.md
source_hash: 0ae7da39ad176fe56c216de3d4560403659065bb
---
## deck ask config set

ask 설정 기본값과 api key 저장

```
deck ask config set [flags]
```

### Options

```
      --api-key string       save the ask api key in XDG config
      --endpoint string      save the default ask provider endpoint
  -h, --help                 help for set
      --model string         save the default ask model
      --oauth-token string   save the ask oauth bearer token in XDG config
      --provider string      save the default ask provider
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck ask config](deck_ask_config.md)	 - 전역 ask 설정 기본값과 API 자격 증명 관리
