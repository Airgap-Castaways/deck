---
source: docs/cli/deck_completion.md
source_hash: 003f6140f6088c4d2a2f5a87580dca71f2f636cb
---
## deck completion

셸 자동완성 스크립트 생성

### 개요

셸 자동완성 스크립트를 생성합니다.

즉시 소싱:
  bash: source <(deck completion bash)
  zsh:  source <(deck completion zsh)
  fish: deck completion fish | source

영구 설정:
  bash: '~/.bashrc'에 'source <(deck completion bash)' 추가
  zsh:  '~/.zshrc'에 'source <(deck completion zsh)' 추가
  fish: deck completion fish > ~/.config/fish/completions/deck.fish

```
deck completion <bash|zsh|fish|powershell>
```

### Options

```
  -h, --help   help for completion
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck](deck.md)	 - deck
