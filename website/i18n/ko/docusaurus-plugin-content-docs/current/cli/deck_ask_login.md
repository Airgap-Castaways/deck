---
source: docs/cli/deck_ask_login.md
source_hash: fe1b646aaf8a4745bcba95b579595c20f4b05b75
---
## deck ask login

OpenAI Codex OAuth로 ask를 인증합니다

```
deck ask login [flags]
```

### Options

```
      --account-email string   optional account email label for status output
      --callback-port int      local callback port for browser-based OAuth login (default 1455)
      --expires-at string      optional RFC3339 access token expiry time
      --headless               use OpenAI Codex device login instead of browser callback login
  -h, --help                   help for login
      --no-browser             print the login URL instead of opening it automatically
      --oauth-token string     oauth bearer token to save
      --provider string        provider to associate with this oauth session
      --refresh-token string   optional refresh token to store for future flows
      --stdin-token            read the oauth bearer token from stdin for headless use
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck ask](deck_ask.md)	 - (실험적 기능) 워크플로 작성 및 검토를 위한 AI 헬퍼
