# Apply run logs

Every `deck apply` invocation writes a run log for diagnostics and audit purposes. Run logs are written outside the workspace under the XDG state root and are never included in bundles.

## Location

```text
$XDG_STATE_HOME/deck/runs/<run-id>/
~/.local/state/deck/runs/<run-id>/    (default when XDG_STATE_HOME is unset)
```

Each apply invocation gets its own directory. The run ID is a UTC timestamp formatted as `20060102T150405.000000000Z` (Go reference time, nanosecond precision), so directory names sort chronologically.

Two files are written inside each run directory:

```text
<run-id>/
├── record.json     # structured summary of the run
└── events.jsonl    # per-step event stream (one JSON object per line)
```

Both files are created with private permissions (mode `0600`/`0700`). They are written incrementally during the run so partial records exist even for interrupted applies.

## `record.json`

A single JSON object summarising the entire apply invocation. The file is rewritten after each step event, so it always reflects the latest known state.

Fields (all `omitempty` except `id`, `command`, and `workflow_ref`):

| Field | Type | Description |
|---|---|---|
| `id` | string | Run ID; matches the directory name. |
| `command` | string | Always `"apply"`. |
| `workflow_ref` | string | Workflow path or URL passed to `deck apply`. |
| `workflow_source` | string | Inferred source: `"local"` or `"server"`. |
| `scenario` | string | `--scenario` value, if supplied. |
| `bundle_root` | string | `--root` value, if supplied. |
| `selected_phase` | string | `--phase` value, if supplied. |
| `hostname` | string | Machine hostname at apply start. |
| `started_at` | string | RFC 3339 nanosecond timestamp when the run began. |
| `ended_at` | string | RFC 3339 nanosecond timestamp when the run finished. |
| `status` | string | Final run status, e.g. `"running"`, `"success"`, `"failed"`. |
| `error` | string | Top-level error message if the run failed. |
| `steps` | array | Per-step summary records (see below). |

Each entry in `steps` is an object with these fields:

| Field | Type | Description |
|---|---|---|
| `step_id` | string | Step identifier from the workflow. |
| `kind` | string | Step kind, e.g. `"WriteFile"`. |
| `phase` | string | Phase the step belongs to. |
| `status` | string | Last observed step status. |
| `reason` | string | Short reason string when status is `"skipped"` or similar. |
| `attempt` | int | Retry attempt number. |
| `started_at` | string | RFC 3339 nanosecond timestamp. |
| `ended_at` | string | RFC 3339 nanosecond timestamp. |
| `error` | string | Error message if the step failed. |

Short example:

```json
{
  "id": "20240315T120000.000000000Z",
  "command": "apply",
  "workflow_ref": "workflows/scenarios/apply.yaml",
  "workflow_source": "local",
  "hostname": "node-01",
  "started_at": "2024-03-15T12:00:00.000000000Z",
  "ended_at": "2024-03-15T12:00:05.123456789Z",
  "status": "success",
  "steps": [
    {
      "step_id": "write-config",
      "kind": "WriteFile",
      "phase": "configure",
      "status": "done",
      "started_at": "2024-03-15T12:00:01.000000000Z",
      "ended_at": "2024-03-15T12:00:01.500000000Z"
    }
  ]
}
```

## `events.jsonl`

A newline-delimited JSON stream. Each line is one event emitted by the apply engine as steps start, finish, or are skipped. Unlike `record.json`, events are never rewritten — each line is appended and fsynced immediately, making `events.jsonl` the most reliable source of timing and ordering data.

Fields on each event line:

| Field | Type | Description |
|---|---|---|
| `ts` | string | RFC 3339 nanosecond timestamp when the event was recorded. |
| `event` | string | Event type name. |
| `step_id` | string | Step identifier. |
| `kind` | string | Step kind. |
| `phase` | string | Phase the step belongs to. |
| `status` | string | Step status at the time of this event. |
| `reason` | string | Short reason string. |
| `attempt` | int | Retry attempt number. |
| `started_at` | string | Step start time (RFC 3339 nanosecond). |
| `ended_at` | string | Step end time (RFC 3339 nanosecond). |
| `error` | string | Error message if the step failed. |
| `batch_id` | string | Batch identifier when the step is part of a parallel batch. |
| `parallel_group` | string | `parallelGroup` label from the workflow. |
| `parallel` | bool | `true` when the step ran in parallel. |
| `batch_size` | int | Number of steps in the batch. |
| `max_parallelism` | int | Concurrency limit in effect for the batch. |
| `failed_step` | string | ID of the first failed step when a batch aborts. |

Example lines (two events from a parallel batch):

```jsonl
{"ts":"2024-03-15T12:00:01.000000000Z","event":"step_started","step_id":"pull-image","kind":"LoadImage","phase":"load","status":"running","batch_id":"b1","parallel_group":"images","parallel":true,"batch_size":3,"max_parallelism":3}
{"ts":"2024-03-15T12:00:03.500000000Z","event":"step_done","step_id":"pull-image","kind":"LoadImage","phase":"load","status":"done","started_at":"2024-03-15T12:00:01.000000000Z","ended_at":"2024-03-15T12:00:03.500000000Z","batch_id":"b1","parallel_group":"images","parallel":true}
```

## Relationship to apply state

Run logs and [apply state](apply-state.md) serve different purposes:

- **Apply state** (`<workspace>/.deck/state/apply/<key>.json` or `$XDG_STATE_HOME/deck/state/apply/<key>.json`) is the resumable checkpoint that `deck apply` consults when deciding which phases to skip. It is keyed by workflow fingerprint and is updated in place.
- **Run logs** are an append-only diagnostic history. Each apply invocation produces exactly one run directory. Run logs are never consulted by `deck apply` itself and are not cleaned up automatically.

Use run logs to answer questions such as: which steps ran, in what order, with what timing, and what error messages were produced. Use apply state to answer whether a phase has been completed and to resume interrupted applies.
