# CLI reference

> Full per-command flag and argument reference is auto-generated under [the CLI reference](cli/deck.md). This page covers the overview and shared conventions only. For task-oriented guidance, see the [guides](guides/authoring-workflows.md).

The `deck` CLI is intentionally small. It supports a simple operator flow: author the workflow, lint it, prepare bundle contents, build the bundle, and run locally.

## Overview

### Default local flow

- `init`: create starter workflow files under `workflows/`
- `lint`: validate a workflow file or workspace against the workflow and step schemas (`-o text|json`)
- `prepare`: gather artifacts into `outputs/`, write a local `deck` launcher, and write `.deck/manifest.json`
- `bundle build`: package the current workspace into a transportable archive
- `apply`: execute the apply scenario locally

### Additional helpers

- `plan`: inspect which apply steps would run or skip before execution (`-o text|json`)
- `list`: list available scenarios from the local workspace or the saved remote server
- `cache list` / `cache clean`: inspect or prune cached artifact entries
- `server remote set/show/unset`: manage the default remote server URL
- `server up` / `server down`: expose a prepared bundle root over HTTP, or stop a daemonized server
- `server health`: check `/healthz` on an explicit server or the saved remote server URL (`-o text|json`)
- `server logs`: read local server audit logs from file or journal
- `state show/list/clear`: inspect and remove saved apply state
- `version`: show the current `deck` build version and metadata (`-o text|json`)
- `completion`: generate shell completion for bash, zsh, fish, and PowerShell

### Authoring helper

- `ask`: experimental helper to question, explain, review, draft, or refine workflows from the current workspace using an LLM-backed authoring assistant

`ask` is experimental and ships as part of the standard `deck` binary.

For a task-oriented guide to configuring and using `deck ask`, see [Using deck ask](ask.md).

`ask` routes requests before generation. Explicit authoring and review flags such as `--create`, `--edit`, and `--review` act as hard overrides. Other requests go through LLM-assisted route classification, and ambiguous requests can stop for clarification instead of drifting into generation.

When model access is unavailable, `ask` degrades explicitly instead of silently pretending to answer with full reasoning. `explain` falls back to a local structural summary of the target file, `review` falls back to local findings, and generation routes fail fast because local validation cannot replace model output.

OpenAI-compatible provider support currently targets: `openai`, `openrouter`, `gemini`.

For per-command `ask` flags and subcommands, see [deck ask](cli/deck_ask.md).

## Output formats

`deck lint -o json` returns a structured report with the validated workflow list, summary counts, supported workflow contracts, and warning-level `findings` such as opaque `Command` steps or remote artifacts without integrity checks.

`deck plan -o json` returns the resolved workflow path, state path, runtime var keys, per-step actions, and a summary section.

`deck plan vars -o json` returns the execution input snapshot: effective `vars`, resolved `context`, initial `runtime` values known before execution, and planned `runtime` keys that later steps may register.

`deck server health -o json` returns the resolved server URL, `/healthz` URL, and HTTP status.

`deck bundle verify -o json` returns the verified bundle path and final status.

`deck cache list -o json` and `deck server logs -o json` keep machine-readable output on stdout while `--v=<n>` sends path and count diagnostics to stderr.

## Verbosity (`--v`)

Global `--v=<n>` writes diagnostics to stderr without changing stdout result contracts. Supported levels are 0–3:

- `--v=0`: result-focused output; long-running `apply` and `prepare` runs still show phase and step progress on stderr
- `--v=1`: workflow/source/path decisions, progress events, and high-level execution context
- `--v=2`: apply execution plan/state/step metadata, ask debug logs, plan evaluation details, and deeper bundle/prepare/health inspection counts
- `--v=3`: key-level traces for apply/prepare/bundle/list/server/cache, ask trace artifacts, contract notes, lint finding hints, and the most detailed plan/lint traces

In practice:

- `deck plan --v=3` adds workflow/runtime var traces and per-step evaluation details
- `deck apply --v=2` adds execution plan, state snapshot, phase/batch plan, and per-step metadata
- `deck apply --v=3` adds workflow hash/state key, context keys, workflow var keys, runtime state keys, and step contract keys without logging spec values
- `deck prepare --v=2` adds artifact group and cache reuse/fetch diagnostics
- `deck prepare --v=3` adds workflow, phase, step, and runtime-binary key traces without logging spec values
- `deck bundle build --v=3` and `deck bundle verify --v=3` add per-manifest-entry path/category/size/hash-prefix traces
- `deck list --v=3`, `deck cache ... --v=3`, and `deck server ... --v=3` add per-entry or request/response/window traces where applicable

For `deck ask` specifically:

- `--v=0`: no ask diagnostics
- `--v=1`: route, provider, and progress summary on stderr
- `--v=2`: route/provider summary plus the user command and MCP events
- `--v=3`: debug logs plus classifier/route system prompts and user prompts; also writes prompt and response payload artifacts under `.deck/ask/runs/<run-id>/`

## Log format (`--log-format`)

Global `--log-format=text|json` controls how migrated diagnostic logs are rendered on stderr.

- `--log-format=text`: one-line structured text logs intended for humans
- `--log-format=json`: JSON Lines on stderr with the same event schema for machine processing

Current migrated command families include `ask`, `prepare`, `apply`, `server`, `list`, and `cache`.

Text diagnostics prioritize high-signal fields such as `phase`, `step`, `status`, `reason`, `kind`, `duration_ms`, `batch`, `invocation_id`, and path/location fields before lower-priority attributes. JSON diagnostics retain the full event fields for machine processing.

For `apply` and `prepare` progress logs, text output expands fields by verbosity: `--v=0` shows core progress fields, `--v=1` adds kind/duration/failure details, and `--v>=2` includes batch, parallelism, attempt, and invocation correlation fields. JSON diagnostics always retain full event fields.

Event naming conventions for new diagnostics:

- `*_requested`: command intent received
- `*_selected` or `*_resolved`: input path, source, or config resolution
- `*_planned`: work plan computed
- `*_started`, `*_succeeded`, `*_failed`: unit-level execution progress
- `*_completed`: command-level completion summary with `status` and `duration_ms`
- `*_summary`: aggregate counts
- `*_trace`: key-level details intended for `--v=3`

New fields use `duration_ms` for durations, `*_bytes` for byte sizes, `*_count` for new count fields, and `has_*` booleans for presence checks. URL diagnostics redact userinfo, query values, and fragments.

Example text diagnostics:

```text
ts=2026-04-02T09:20:00Z level=info component=prepare event=batch_started phase=prepare batch=prepare:downloads parallel_group=downloads batch_size=2 max_parallelism=2 status=started
ts=2026-04-02T09:20:00Z level=info component=prepare event=step_started phase=prepare batch=prepare:downloads step=download-runc kind=DownloadFile attempt=1 status=started
```

Example JSON diagnostics:

```json
{"ts":"2026-04-02T09:20:00Z","level":"info","component":"prepare","event":"batch_started","phase":"prepare","batch":"prepare:downloads","parallel_group":"downloads","batch_size":2,"max_parallelism":2,"status":"started"}
{"ts":"2026-04-02T09:20:00Z","level":"info","component":"prepare","event":"step_started","phase":"prepare","batch":"prepare:downloads","step":"download-runc","kind":"DownloadFile","attempt":1,"status":"started"}
```

## Workflow source locators

For `plan`, `apply`, and `state`, prefer selecting the workflow source and the entrypoint separately:

```bash
deck apply --root ./demo --scenario apply
deck plan --server https://server --scenario apply
deck state show --server https://server --scenario apply
```

- `--root <path>` selects a local workflow tree or bundle root containing `workflows/`.
- `--server <url>` selects a remote workflow server and resolves scenarios under `<url>/workflows/scenarios/`.
- `--scenario <name>` selects `workflows/scenarios/<name>.yaml` under the selected source.
- `--workflow <path-or-url>` remains available as an explicit workflow-file escape hatch.
- `--root` and `--server` are mutually exclusive.
- `--workflow` and `--scenario` are mutually exclusive.
- Existing `--source server --scenario <name>` still works with `DECK_SERVER` or `deck server remote set`, but `--server <url> --scenario <name>` is the clearer inline form.

Local workflow apply state stays under `./.deck/state/apply/`; remote workflow apply state uses the user-local XDG state root under `deck/state/apply/`. Workspace-local metadata stays under `./.deck/`, while user-global config, remote workflow state, cache, and run history use standard XDG locations.

## Variable overrides

`lint`, `prepare`, `plan`, and `apply` support repeatable `-f, --vars-file` YAML overlays. `prepare`, `plan`, and `apply` also support repeatable `--var key=value` overrides for one invocation.

- Vars files are merged on top of `workflows/vars.yaml` in the order provided.
- Node-scoped `all:` and `hosts:` values are selected after vars-file overlays are merged.
- Workflow `vars:` are applied after node-scoped selection.
- `--var` overrides are applied last and have the highest precedence.

Vars file paths are relative to the selected workflow root. For example, `--root ./demo -f vars/site.yaml` reads `./demo/workflows/vars/site.yaml`.

## Runtime binary selection

When preparing, `--bundle-binary-source=auto` (the default) resolves to `release`; on dev builds it fetches the latest GitHub Release unless `--bundle-binary-dir` selects local binaries. `--bundle-binary-source=local` without `--bundle-binary-dir` uses the current executable for the current host tuple. The selected binary is published atomically into `outputs/bin/`.

## Shell completion

`deck completion` is the only completion entrypoint. Supported shells: `bash`, `zsh`, `fish`, `powershell`.

To enable completion for your current shell session:

```bash
source <(deck completion bash)
source <(deck completion zsh)
deck completion fish | source
```

For persistent registration, add the sourcing command to your shell's initialization file (e.g., `~/.bashrc`, `~/.zshrc`). For PowerShell, add `deck completion powershell | Out-String | Invoke-Expression` to your `$PROFILE`.

## Common examples

```bash
deck init --out ./demo
deck version
deck version -o json
deck list --source local
deck lint --workflow ./demo/workflows/scenarios/apply.yaml
deck lint --workflow ./demo/workflows/scenarios/apply.yaml -o json

cd ./demo
deck prepare
deck prepare -f vars/site.yaml --var registryHost=mirror.local --var kubernetesVersion=v1.30.1
deck bundle build --out ./bundle.tar
deck plan --root . --scenario apply
deck plan --root . --scenario apply -o json
deck apply --root . --scenario apply
deck apply --root . --scenario apply -f vars/cp1.yaml --var role=control-plane --var nodeIP=10.0.0.10
deck cache clean --older-than 30d --dry-run
```

Optional site-local server example:

```bash
deck server remote set http://127.0.0.1:8080
deck server up --root ./bundle --addr :8080
deck server up --root ./bundle --addr :8443 --tls-self-signed
deck server health --server http://127.0.0.1:8080 -o json
deck plan --server http://127.0.0.1:8080 --scenario apply
deck bundle verify --file ./bundle -o json
```

Optional `ask` example:

```bash
deck ask config set --provider openai --model gpt-5.4 --api-key "$DECK_ASK_API_KEY"
deck ask "explain what workflows/scenarios/apply.yaml does"
deck ask --create "create an air-gapped rhel9 single-node kubeadm workflow"
deck ask --review
```

<!-- BEGIN GENERATED:ASK_CLI_CONTEXT -->
## Ask CLI context

- `deck ask` writes workflow files directly for authoring routes; use `--create` or `--edit` to make authoring intent explicit.
- `deck ask plan` saves reusable plan artifacts under `./.deck/plan/`.
<!-- END GENERATED:ASK_CLI_CONTEXT -->

<!-- BEGIN GENERATED:ASK_AUTHORING_CONTEXT -->
## Ask authoring context

- Top-level workflow authoring reference for deck workflows.
- Imports are only valid under phases[].imports and resolve from workflows/components/ using component-relative paths.
- Prefer workflows/vars.yaml for configurable values that would otherwise be repeated inline across steps or files.
- Start with typed step groups first. Prefer typed steps over `Command` when a typed step exists.
<!-- END GENERATED:ASK_AUTHORING_CONTEXT -->
