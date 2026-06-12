# Apply State

`deck apply` stores progress in a state file derived from the workflow `StateKey`.

## Where state is stored

Local workflow state is stored in the workspace:

```text
<workspace>/.deck/state/apply/<state-key>.json
```

Remote workflow state is stored in the user-local XDG state root:

```text
$XDG_STATE_HOME/deck/state/apply/<state-key>.json
~/.local/state/deck/state/apply/<state-key>.json
```

Use `--state-dir` to choose an explicit directory:

```bash
deck apply --state-dir /var/lib/deck/state/apply --server https://example.invalid --scenario apply
deck plan --state-dir /var/lib/deck/state/apply --server https://example.invalid --scenario apply
```

When `--state-dir` is supplied, deck uses only `<dir>/<state-key>.json`. It does not infer workspace-local state, read old default paths, or run automatic migration.

State under `.deck/state/` is runtime-local metadata. It should not be committed and is not included in bundles. Fresh clones and freshly extracted bundles start without saved apply state unless a state directory is copied or supplied explicitly.

Remote workflows are not anchored to the current directory, bundle root, or fixed temporary paths such as `/tmp/deck`. Those locations can be surprising, shared, or volatile.

## Privilege and user scope

Default state is user-scoped.

- `deck apply` and `sudo deck apply` use different default XDG roots for remote workflows.
- `sudo -E deck apply` can preserve user XDG variables and create root-owned files in a user-owned state directory.
- Repeated privileged remote workflow runs should use an explicit system state directory such as `/var/lib/deck/state/apply`.
- If privileged and unprivileged commands both need to inspect state, choose explicit directory permissions deliberately.

## What identifies saved state

- resolved workflow bytes after imports expand
- effective vars for the run
- the apply execution context fingerprint

This keeps state isolated by the final workflow fingerprint and input vars.

Workflow, vars, or context changes produce a different state key. Old state is not treated as corrupt or invalid; it is simply no longer selected by the new key.

## Migration

Default-path runs migrate existing state forward when the new target file does not already exist.

Migration sources:

```text
$XDG_STATE_HOME/deck/state/<state-key>.json
~/.local/state/deck/state/<state-key>.json
~/.deck/state/<state-key>.json
```

Migration targets:

```text
local workflow:  <workspace>/.deck/state/apply/<state-key>.json
remote workflow: $XDG_STATE_HOME/deck/state/apply/<state-key>.json
remote workflow: ~/.local/state/deck/state/apply/<state-key>.json
```

Migration copies state instead of deleting old files. If both old XDG state and legacy `~/.deck/state/` exist for the same key, old XDG state wins. Existing target files are never overwritten.

At `--v>=1`, deck reports migration as `event=state_migrated source=<old> target=<new>`.

## Phase-based resume

Apply now resumes at phase boundaries, not step boundaries.

- completed phases are skipped on later non-fresh runs
- a failed phase is rerun from its first step on the next non-fresh run
- partial progress from inside a failed phase is not reused

## What is stored

- format version and kind
- state key
- workflow path, source, and hash when available
- status and current phase
- completed phase names
- failed phase error, when the run stops
- runtime vars exported by fully completed phases
- runtime secret metadata, without secret values

New state files use a versioned v2 JSON shape. Older v1 files remain readable and are normalized internally.

## Parallel batches inside a phase

When a phase uses explicit `parallelGroup` batches:

- steps in the same batch start from the same `runtime` snapshot
- `register` outputs from that batch become visible only after the full batch succeeds
- if any step in the batch fails, the whole phase remains incomplete

Authoring and validation rules still apply before the batch runs:

- `parallelGroup` values must stay contiguous inside the phase
- apply-time parallel batches only support a limited safe kind allowlist
- same-batch steps cannot target the same literal path, share the same prepared output root, or consume each other's `runtime.*` outputs

Use [Workflow Model](workflow-model.md#parallel-batches) for the current batching rules and constraints.

## `--fresh`

Use `deck apply --fresh` to clear the selected saved apply state before execution.

- `deck apply --fresh` reruns all phases and writes fresh state back to the normal path.
- Only the selected state key is cleared. Other state files in the same directory are preserved.
- `deck apply --dry-run --fresh` is rejected because `--fresh` clears state.

`deck plan` is read-only and does not support `--fresh`. Use `deck state clear` for explicit state management without running apply.

## State management

Use `deck state` to inspect and delete apply state:

```bash
deck state show --root . --scenario apply
deck state show --server https://example.invalid --scenario apply
deck state list
deck state clear --root . --scenario apply --yes
deck state clear --all --yes
```

`deck state show` uses the same workflow, vars, and state-key logic as `deck plan` and `deck apply`. Use `--state-dir` with `deck state` when apply/plan used an explicit state directory.
