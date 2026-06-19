## deck prepare

Prepare bundle contents under outputs/

```
deck prepare [flags]
```

### Options

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

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck](deck.md)	 - deck

