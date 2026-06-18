---
source: docs/cli/deck_prepare.md
source_hash: e08107b0da18edcfdff3b0fb71c060cf1a28501e
---
## deck prepare

outputs/ 디렉터리에 번들 콘텐츠를 준비합니다.

```
deck prepare [flags]
```

### 옵션

```
      --bundle-binary strings           runtime binary target tuple (os/arch), repeatable
      --bundle-binary-dir string        directory containing local runtime binaries for --bundle-binary-source=local
      --bundle-binary-exclude strings   runtime binary target tuple (os/arch) to exclude, repeatable
      --bundle-binary-source string     runtime binary source (auto|local|release) (default "auto")
      --bundle-binary-version string    release version override for --bundle-binary-source=release
      --clean                           remove the prepared directory before writing
      --dry-run                         print prepare plan without writing files
  -h, --help                            help for prepare
      --refresh                         re-download artifacts instead of reusing prepared files
      --root string                     prepared bundle output directory (default "outputs")
      --var stringToString              set variable override (key=value), repeatable
  -f, --vars-file strings               vars file overlay relative to workflows/ (repeatable)
```

### 상위 명령에서 상속된 옵션

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### 관련 항목

* [deck](deck.md)	 - deck
