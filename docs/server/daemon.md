# Server daemon mode

`deck server up --daemon` starts the content server in the background. The
mechanism differs by platform: Linux uses a transient systemd service; macOS
and Windows use a detached background process tracked by a pid file.

## Linux — transient systemd service

On Linux, `deck server up --daemon` invokes `systemd-run` to launch a
transient service unit. Both `systemd-run` and `systemctl` must be present on
`PATH` or the command fails before starting anything.

The unit name is set with `--unit` (default `deck-server`). The value is
normalized: a trailing `.service` suffix is stripped before use, then
re-appended as needed. The working directory is forwarded to the unit via a
`WorkingDirectory=` property.

To stop the daemon:

```
deck server down [--unit deck-server]
```

`deck server down` calls `systemctl stop <unit>.service`. Logs are available
through the journal:

```
journalctl -u deck-server.service
```

or via `deck server logs --source journal --unit deck-server.service`.

## macOS and Windows — detached background process

On macOS and Windows, `deck server up --daemon` spawns a detached child
process:

- **macOS:** the child process starts a new session (`Setsid: true`).
- **Windows:** the child process uses `CREATE_NEW_PROCESS_GROUP` and the
  `DETACHED_PROCESS` creation flag.

Both stdout and stderr of the child process are redirected to a log file.

### State file locations

The pid file and log file are written under the XDG state root
(`$XDG_STATE_HOME/deck` if set, otherwise `~/.local/state/deck`):

```
~/.local/state/deck/server/<unit>.pid
~/.local/state/deck/server/<unit>.log
```

where `<unit>` is the normalized `--unit` value (default `deck-server`). On
startup, `deck server up` prints the log file path to stdout:

```
server up: ok (deck-server, pid 12345)
server log: /home/user/.local/state/deck/server/deck-server.log
```

### Stopping the daemon

```
deck server down [--unit deck-server]
```

`deck server down` reads the pid file, sends `SIGTERM` (macOS) or calls
`Process.Kill` (Windows), then removes the pid file. If the pid file is not
found, the command fails with:

```
server down: pid file not found: <path>
```

If the process is already gone (stale pid), the stop still succeeds and the
pid file is cleaned up.

## Checking server health

After starting the daemon on any platform, use `deck server health` to confirm
the server is responding:

```
deck server health --server http://127.0.0.1:8080
```

This sends a GET request to `<server>/healthz` and reports `health: ok` on
HTTP 200.

## Related

- [deck server up](../cli/deck_server_up.md)
- [Server Registry](registry.md)
