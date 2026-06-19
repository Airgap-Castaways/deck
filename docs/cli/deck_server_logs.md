## deck server logs

Read local server audit logs from file or journal

```
deck server logs [flags]
```

### Options

```
  -h, --help            help for logs
  -o, --output string   output format (text|json) (default "text")
      --path string     explicit audit log file path
      --root string     serve root directory (default ".")
      --source string   log source (file|journal|both) (default "file")
      --unit string     systemd unit for journal logs (default "deck-server.service")
```

### Options inherited from parent commands

```
      --log-format string   diagnostic log format (text|json) (default "text")
      --v int               diagnostic verbosity level (0-3; higher is more detailed)
```

### SEE ALSO

* [deck server](deck_server.md)	 - Run the local content server and manage remote lookup defaults

