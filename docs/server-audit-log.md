# Server Audit Log

`deck server up` writes audit records to a JSONL log file under the bundle root.

## Location

Default file location:

```text
<root>/.deck/logs/server-audit.log
```

## Current emitted record shape

`deck server up` currently emits audit schema version `2` records with structured top-level fields.

Common fields:

- `ts`: RFC3339Nano timestamp in UTC
- `schema_version`: currently `2`
- `component`: log producer, currently `server`
- `event`: normalized event name such as `request`
- `level`: `info`, `warn`, or `error`
- `message`: short human-readable description

Request records also include top-level request attributes such as:

- `method`
- `path`
- `proto`
- `status`
- `bytes`
- `remote_addr`
- `duration_ms`

## Typical examples

- Records are written for routed server responses including site API, registry, static file, and health checks
- The current writer keeps request attributes at the top level rather than under a nested `extra` object

Example request record:

```json
{"ts":"2026-04-03T05:00:00Z","schema_version":2,"component":"server","event":"request","level":"info","message":"http request handled","method":"GET","path":"/healthz","proto":"HTTP/1.1","status":200,"bytes":0,"remote_addr":"127.0.0.1:53422","duration_ms":1}
```

## Compatibility note

- current `deck server up` writes the structured version-2 shape above
- `deck server logs` can still normalize older legacy records that used fields such as `source`, `event_type`, or nested `extra`
- downstream consumers that read the raw audit file should expect the version-2 top-level structure going forward

## Rotation

- `deck server up` rotates the audit log when it exceeds the configured size limit
- defaults: `50` MB max size and `10` retained files
- related flags: `--audit-max-size-mb`, `--audit-max-files`

## Viewing logs

```bash
deck server logs --source file --path <root>/.deck/logs/server-audit.log
```
