---
source: docs/cli/deck_bundle_build.md
source_hash: 1609cd9dc77e7fd97355aa5204596e7e3d42ebe8
---
## deck bundle build

deck, 워크플로, 출력물, 매니페스트를 아카이브로 묶기

```
deck bundle build [flags]
```

### Options

```
  -h, --help          help for build
      --out string    output tar archive path
      --root string   workspace root containing deck, workflows, outputs, and .deck/manifest.json (default ".")
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck bundle](deck_bundle.md)	 - 번들 빌드 또는 검증
