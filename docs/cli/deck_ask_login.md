## deck ask login

Authenticate ask with OpenAI Codex OAuth

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

* [deck ask](deck_ask.md)	 - (Experimental) AI helper for drafting and reviewing workflows

