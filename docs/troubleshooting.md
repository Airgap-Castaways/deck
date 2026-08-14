# Troubleshooting

This guide is a symptom-oriented companion to the error-code catalog. It walks through the most common failure modes, explains what each one means, and shows how to fix it. For the full list of stable error codes see [diagnostics/error-codes.md](diagnostics/error-codes.md).

## How to read a deck error

deck renders errors in the form `CODE: message`, for example:

```
E_BUNDLE_INTEGRITY: artifact outputs/files/kubeadm.conf: sha256 mismatch (expected a3f1…, got 9b2d…)
```

The `CODE` part is a stable, machine-readable identifier. The `message` part contains context for the specific failure.

### Getting more detail

Use `--v=<n>` to increase diagnostic verbosity on stderr without changing what goes to stdout:

| Level | What you get |
|-------|-------------|
| `--v=0` | Result-focused output; `apply` and `prepare` still show phase and step progress |
| `--v=1` | Workflow/source/path decisions, progress events, high-level execution context |
| `--v=2` | Execution plan, state snapshot, phase/batch plan, per-step metadata |
| `--v=3` | Key-level traces for workflow hash, state key, context keys, step contract keys |

Use `--log-format=json` to emit diagnostics as JSON Lines on stderr for machine processing:

```bash
deck apply --root . --scenario apply --v=2 --log-format=json 2>apply.log
```

This is useful for grepping or piping to `jq`. The JSON event schema matches the text log events field-for-field.

For `apply` and `prepare` progress, `--v=1` adds kind, duration, and failure details; `--v=2` adds batch, parallelism, attempt, and invocation correlation fields.

See [CLI Reference](cli.md#verbosity---v) for the full verbosity table.

---

## `deck lint` failures

`deck lint` validates workflow YAML structure and the schema for each step before the workflow is packaged or run. Catching mistakes here is cheaper than discovering them inside the air gap.

### Schema errors (`E_SCHEMA_INVALID`)

**Symptom:** `E_SCHEMA_INVALID: step "write-config": field "spec.path" is required`

**Cause:** A step field has an invalid or deprecated value, or a required field is missing.

**Fix:** Check the step kind's schema. Run `deck lint -o json` to get a structured report with a `findings` list; each entry names the step, field, and constraint that failed. Refer to [step-kinds.md](step-kinds.md) for the allowed fields for each kind.

### Role/mode mismatches (`E_KIND_ROLE_MISMATCH`)

**Symptom:** `E_KIND_ROLE_MISMATCH: step "fetch-kubeadm": kind DownloadFile is not valid in apply role`

**Cause:** A step kind that belongs to the prepare phase (such as `DownloadFile`, `DownloadImage`, or `DownloadPackage`) appeared in an apply scenario, or vice versa. The workflow role is determined by command context and file location, not by an in-file field.

**Fix:** Move the step to the correct workflow file. Prepare-only kinds go in `workflows/prepare.yaml`. Apply kinds go in `workflows/scenarios/<name>.yaml` or in component fragments imported from there. See the phase/group index in [step-kinds.md](step-kinds.md).

### Duplicate step IDs (`E_DUPLICATE_STEP_ID`)

**Symptom:** `E_DUPLICATE_STEP_ID: step id "install-packages" is used more than once`

**Cause:** Two or more steps share the same `id` value, either in the same file or after imports are expanded.

**Fix:** Give each step a unique `id`. When component fragments are imported into multiple phases, each fragment must use IDs that are unique across the combined workflow. Use a prefix convention such as `<component>-<action>` to avoid collisions.

### Duplicate phase names (`E_DUPLICATE_PHASE_NAME`)

**Symptom:** `E_DUPLICATE_PHASE_NAME: phase name "install" is used more than once`

**Cause:** Two phases in the same workflow share the same `name` (or both have an empty name).

**Fix:** Give each phase a unique, non-empty name.

### Unknown step kind

**Symptom:** `E_SCHEMA_INVALID: step "my-step": unknown kind "InstallKubeconfig"`

**Cause:** The `kind` value does not match any registered step kind. Common causes are typos and version drift between the workflow and the deck binary.

**Fix:** Check the exact kind name in [step-kinds.md](step-kinds.md). Kind names are case-sensitive.

### Discontiguous `parallelGroup` (`E_PARALLEL_GROUP_DISCONTIGUOUS`)

**Symptom:** `E_PARALLEL_GROUP_DISCONTIGUOUS: parallelGroup "images" is not contiguous in phase "load"`

**Cause:** Steps that share a `parallelGroup` label are not adjacent within their phase. Once a parallel batch closes, the same `parallelGroup` label cannot reappear later in the phase.

**Fix:** Move all steps that share a `parallelGroup` value so they are consecutive within the phase. If the second group needs to run after some intervening steps, use a different `parallelGroup` label.

### `register` on a kind with no outputs (`E_REGISTER_OUTPUT_NOT_FOUND`)

**Symptom:** `E_REGISTER_OUTPUT_NOT_FOUND: step "write-config": kind WriteFile declares no outputs; register key "result" is not valid`

**Cause:** A `register` block names an output key that the step kind does not produce. Only step kinds that explicitly declare outputs support `register`.

**Fix:** Remove the `register` block, or switch to a step kind that produces the output you need (for example, `InitKubeadm` produces `joinFile`). The step kind's entry in [step-kinds.md](step-kinds.md) lists its declared outputs.

### Reserved runtime variable names (`E_RUNTIME_VAR_RESERVED`, `E_REGISTER_VAR_INVALID`)

**Symptom:** `E_RUNTIME_VAR_RESERVED: register name "host" conflicts with built-in runtime.host`

**Cause:** A `register` key conflicts with a built-in runtime namespace (`host`) or does not match the required naming pattern.

**Fix:** Choose a register key that does not shadow built-in names. Built-in fields live under `runtime.host.*` and `context.*`. User-defined register outputs become `runtime.<your-key>`.

---

## `deck prepare` failures

`deck prepare` fetches artifacts from the network or local sources and writes them under `outputs/`. Failures here mean the bundle cannot be built yet.

### Network and download errors (`E_PREPARE_SOURCE_NOT_FOUND`, `E_PREPARE_OFFLINE_POLICY_BLOCK`)

**Symptom:** `E_PREPARE_SOURCE_NOT_FOUND: step "fetch-kubeadm": source.path "bin/kubeadm" not resolved to any configured fetch source`

**Cause:** The source path does not match any configured fetch source, or a URL download was blocked by the offline policy.

**Fix:** Check that the `source.path` or `source.url` matches a reachable source. If the machine running `prepare` is offline and the step requires a URL, either configure a local mirror or run `prepare` from a machine with network access. Use `deck prepare --dry-run` to see what would be fetched before running.

### Checksum mismatches (`E_PREPARE_CHECKSUM_MISMATCH`)

**Symptom:** `E_PREPARE_CHECKSUM_MISMATCH: step "fetch-containerd": sha256 mismatch (expected …, got …)`

**Cause:** The downloaded artifact does not match the expected SHA-256 digest. Possible causes include a corrupted download, an upstream content change, or a stale local cache.

**Fix:** Run `deck prepare --refresh` to bypass reuse and re-download all artifacts. If the mismatch persists, verify the expected digest in the workflow matches the upstream source.

### Resolver metadata mismatches

**Cause:** Reuse metadata written by a previous `prepare` run (`outputs/images/.deck-cache-images.json` for images, package index metadata for packages) no longer matches what is on disk.

**Fix:** Run `deck prepare --refresh` to ignore cached metadata and fetch fresh copies. Use `deck prepare --clean` to remove the prepared outputs directory before writing, this is the most thorough reset when the outputs tree is in an inconsistent state.

### Missing container runtime (`E_PREPARE_RUNTIME_NOT_FOUND`, `E_PREPARE_RUNTIME_UNSUPPORTED`)

**Symptom:** `E_PREPARE_RUNTIME_NOT_FOUND: no supported container runtime (docker/podman) found`

**Cause:** A `DownloadPackage` step with `backend.mode: container` requires `docker` or `podman`, and neither was found. This step builds OS packages inside a container, so a working container runtime must be available on the machine running `prepare`.

**Fix:** Install Docker or Podman on the prepare machine. Alternatively, if the workflow allows it, switch `backend.mode` to a non-container mode, but check the [step-kinds.md](step-kinds.md) entry for `DownloadPackage` to confirm which modes are available for your target distro.

---

## Bundle integrity failures at apply start

Before executing any workflow phase, `deck apply` verifies the bundle manifest. A verification failure aborts the run immediately.

### `E_MANIFEST_MISSING`

**Symptom:** `E_MANIFEST_MISSING: .deck/manifest.json not found in bundle`

**Cause:** The bundle was transferred without the `.deck/manifest.json` file, or the bundle was assembled without running `deck prepare` first (which writes the manifest).

**Fix:**
1. If you transferred the bundle as a tarball, verify the archive contains `.deck/manifest.json`: `tar -tf bundle.tar | grep manifest.json`.
2. If the manifest is missing, rebuild the bundle: run `deck prepare` followed by `deck bundle build --out ./bundle.tar`.
3. Re-transfer the complete archive to the target.

### `E_MANIFEST_EMPTY`

**Symptom:** `E_MANIFEST_MISSING: manifest exists but contains no tracked entries` (or `E_MANIFEST_EMPTY`)

**Cause:** The manifest file is present but its `entries` array is empty. This typically means `deck prepare` ran but produced no artifacts (an empty `outputs/` tree), or the manifest was truncated during transfer.

**Fix:** Re-run `deck prepare` to populate `outputs/` with the artifacts declared by the prepare workflow. Then rebuild and re-transfer the bundle.

### `E_BUNDLE_INTEGRITY`

**Symptom:** `E_BUNDLE_INTEGRITY: artifact outputs/files/kubeadm.conf: sha256 mismatch`

**Cause:** An artifact is missing, or its SHA-256 digest or file size does not match the manifest. Common causes: partial transfer, file corruption, or manual edits to `outputs/` after `prepare`.

**Fix:**
1. Run `deck bundle verify --file ./bundle.tar` (or `deck bundle verify --file ./bundle-dir`) to get a full report of all mismatched entries.
2. If the bundle directory was transferred file-by-file, re-transfer using a checksum-preserving method such as `rsync -c` or a verified archive.
3. If the bundle itself is correct but the archive was extracted incorrectly, re-extract and verify again.
4. If artifacts were intentionally changed, rebuild the bundle from scratch: `deck prepare` then `deck bundle build`.

See [bundle-layout.md](bundle-layout.md#apply-time-manifest-verification) for the full list of paths that the manifest covers.

---

## Apply failures and resuming

### How resume works

`deck apply` stores progress as a state file keyed by a fingerprint of the workflow, vars, and execution context. Completed phases are recorded. On the next non-fresh run, completed phases are skipped and execution resumes at the first incomplete phase.

A failed phase is rerun from its first step on the next run. Partial progress inside a failed phase is not reused, the whole phase reruns.

```bash
# Check what has completed and what would run next
deck state show --root . --scenario apply

# Then resume
deck apply --root . --scenario apply
```

See [apply-state.md](apply-state.md) for the full state model and file locations.

### When to use `--fresh`

Use `deck apply --fresh` when you want to rerun all phases regardless of saved state:

```bash
deck apply --root . --scenario apply --fresh
```

`--fresh` clears only the selected state key. Other state files in the same directory are preserved. Note that `--fresh` and `--dry-run` cannot be combined.

### Why state may reset (new state key)

If the workflow file content, the effective vars, or the execution context changes, deck computes a different state key. The old state is not deleted, it is simply no longer matched by the new key. This means a resumed run after editing the workflow will start from the beginning.

Use `deck plan --root . --scenario apply` before applying to see the resolved state key and which steps would run or skip. Use `deck state list` to see all state files on disk.

### Inspecting and clearing state

```bash
# Show current state for a workflow
deck state show --root . --scenario apply

# List all saved state files
deck state list

# Clear state for a specific workflow (requires --yes)
deck state clear --root . --scenario apply --yes

# Clear all saved state
deck state clear --all --yes
```

---

## Template and CEL mistakes

### Single-brace syntax (`E_TEMPLATE_SINGLE_BRACE`)

**Symptom:** `E_TEMPLATE_SINGLE_BRACE: step "write-config": template uses {vars.name}; use {{.vars.name}}`

**Cause:** A template expression uses single-brace syntax (`{var}`) instead of the required double-brace syntax. deck uses Go templates for string interpolation.

**Fix:** Replace `{vars.name}` with `{{ .vars.name }}`. Note the leading dot: vars are accessed as `.vars.NAME`, runtime values as `.runtime.NAME`, and context values as `.context.NAME` inside templates.

```yaml
# Wrong
content: "cluster: {vars.clusterName}"

# Correct
content: "cluster: {{ .vars.clusterName }}"
```

### CEL type errors and `E_CONDITION_EVAL`

**Symptom:** `E_CONDITION_EVAL: step "install-rhel-packages": when: type error: expected bool, got string`

**Cause:** The `when` field takes a CEL expression, not a Go template. CEL uses a different syntax: no braces, no leading dot, and strict types.

**Fix:** Check the CEL expression. Variable references in `when` use `vars.NAME`, `runtime.NAME`, and `context.NAME` (no braces, no leading dot):

```yaml
# Wrong, Go template syntax in a CEL field
when: "{{ .vars.role == \"control-plane\" }}"

# Correct, CEL expression
when: vars.role == "control-plane"
```

For runtime host facts, use `runtime.host.os.family`, `runtime.host.arch`, etc. (see [workflow-model.md](workflow-model.md#built-in-runtime-fields)).

### Register key conflicts (`E_RUNTIME_VAR_REDEFINED`, `E_PARALLEL_OUTPUT_CONFLICT`)

**Symptom:** `E_RUNTIME_VAR_REDEFINED: register key "joinFile" is defined more than once`

**Cause:** Two steps register an output under the same key, creating an ambiguous runtime value.

**Fix:** Use distinct register key names for each step. If two steps intentionally produce the same logical value (for example, in a branching workflow), use `when:` conditions to ensure only one runs.

### Consuming a registered value within the same parallel batch (`E_PARALLEL_RUNTIME_DEPENDENCY`)

**Symptom:** `E_PARALLEL_RUNTIME_DEPENDENCY: step "join-node" reads runtime.joinFile produced by step "get-join-cmd" in the same parallelGroup`

**Cause:** A step inside a `parallelGroup` batch tries to consume a `runtime.*` value registered by another step in the same batch. Registered values from a parallel batch only become visible after the full batch succeeds.

**Fix:** Move the producing step (the one with `register:`) to an earlier phase or an earlier sequential step, outside the parallel batch. The consuming step can remain in the parallel batch or run after it.

See [workflow-model.md](workflow-model.md#parallel-batches) for the full set of parallel batch constraints.

---

## `deck ask` issues

### Provider not configured

**Symptom:** `deck ask` replies with an error about a missing provider or model, or returns a degraded local-only response.

**Cause:** No provider has been configured, or the saved config is incomplete.

**Fix:** Configure a provider:

```bash
deck ask config set \
  --provider openai \
  --model gpt-5.4 \
  --api-key "$DECK_ASK_API_KEY"
```

Supported providers: `openai`, `openrouter`, `gemini`. Inspect the current config:

```bash
deck ask config show
```

### OAuth or transport failures

**Symptom:** `deck ask` reports a transport start failure, MCP initialize failure, or tool-list mismatch.

**Fix:** Run the health check to distinguish transport issues from capability gaps:

```bash
deck ask config health
```

This is the quickest way to see whether the provider endpoint is reachable, whether MCP servers are starting, and whether required capabilities are present.

For OpenAI OAuth sessions specifically:

```bash
deck ask status --provider openai   # check saved session
deck ask login --provider openai    # start browser login
```

### Authoring routes fail fast

**Cause:** `deck ask` authoring routes (`--create`, `--edit`) depend on model access. They do not fall back to local generation.

**Fix:** Confirm provider connectivity with `deck ask config health`. If the model is unreachable (for example, inside an air gap with no external evidence provider), use `deck ask --review` or `deck ask` question/explain routes, which have limited local fallbacks, or pre-generate workflow files on a machine with connectivity and carry them in as static files.

### Diagnostics for `deck ask`

Increase verbosity to trace route selection and provider events:

```bash
deck ask --v=1 "explain workflows/scenarios/apply.yaml"   # route + provider summary
deck ask --v=2 "explain workflows/scenarios/apply.yaml"   # adds user command + MCP events
deck ask --v=3 "review this workspace"                    # full debug + prompt artifacts
```

At `--v=3`, prompt and response artifacts are written under `.deck/ask/runs/<run-id>/`. Session state is also under `.deck/ask/last-agent-session.json`.

See [ask.md](ask.md#diagnostics-and-troubleshooting) for the full ask diagnostics reference.

---

## Getting diagnostics

### Verbosity levels

```bash
deck apply --root . --scenario apply --v=1   # phase/step progress with kind and duration
deck apply --root . --scenario apply --v=2   # + execution plan and state snapshot
deck apply --root . --scenario apply --v=3   # + state key, context keys, step contract keys
```

Verbosity levels apply across all commands. `deck prepare --v=2` adds artifact group and cache reuse/fetch diagnostics; `deck bundle verify --v=3` adds per-entry path, category, size, and hash traces.

### Structured JSON logs

```bash
deck apply --root . --scenario apply --v=2 --log-format=json 2>apply.log
jq 'select(.event == "step_failed")' apply.log
```

JSON Lines on stderr retain all event fields and are stable for scripting.

### Apply run logs

Every `deck apply` invocation writes a per-invocation run log under the XDG state root:

```text
$XDG_STATE_HOME/deck/runs/<run-id>/
~/.local/state/deck/runs/<run-id>/    (default)
```

Each run directory contains `record.json` (structured summary, updated after each step) and `events.jsonl` (append-only per-step event stream). These are the most reliable source of step timing, ordering, and error details for a completed or interrupted apply.

See [apply-runlogs.md](apply-runlogs.md) for the run log schema and field reference.
