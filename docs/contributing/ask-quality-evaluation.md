# Ask Quality Evaluation

This document defines the minimum quality checks for `deck ask` pipeline changes.

The goal is not prompt prettiness or lower token counts. The goal is that `ask` keeps producing the right route, the right evidence boundary, and the right workflow output after simplification work.

## Rules

- Every meaningful `ask` pipeline change must run a before/after quality suite.
- The same prompts must be run before the change and after the change.
- If maintainability improves but the quality suite regresses, the change is not done yet.
- When a new path replaces an old path, remove the old path in the same issue unless a concrete blocker prevents it.
- Do not keep legacy fallback paths, compatibility labels, or dormant code branches just to make a refactor feel safer.

## Required automated checks

Every issue must run:

- targeted package tests for the changed subsystem
- `make test && make lint`
- regression tests for the failure mode that motivated the change

## Required manual quality suite

Use the built `deck` binary from the current branch.

```bash
make build
```

Record the following for each prompt:

- expected route
- whether external evidence was used or intentionally avoided
- whether local facts were present when they should have been
- whether clarification was correctly triggered or avoided
- for authoring routes, whether the final output was valid and matched the requested workflow shape

## Suite A: Repo-root informational checks

Run these from the repository root.

### Prompt A1: local typed-step explanation

```bash
./bin/deck ask "Explain the typed step builders defined in internal/stepspec for DownloadPackage, InstallPackage, InitKubeadm, and JoinKubeadm."
```

Expected outcome:

- route: `explain`
- external evidence: not required
- local facts: should rely on repo-owned typed-step facts rather than external docs
- answer shape: should mention the requested step kinds or clearly state if some are unsupported

### Prompt A2: upstream install evidence boundary

```bash
./bin/deck ask "Explain how to install kubeadm 1.35.1."
```

Expected outcome:

- route: `explain`
- external evidence: required or clearly used
- local facts: should not override upstream install/version guidance
- answer shape: should cite or reflect upstream installation guidance rather than deck-local workflow rules

## Suite B: Empty-workspace authoring checks

Create a temporary empty workspace and run `deck ask` there.

```bash
tmpdir=$(mktemp -d)
cd "$tmpdir"
```

### Prompt B1: supported draft generation

```bash
/home/opencode/workspace/deck/bin/deck ask --create "Create a minimal single-node apply-only offline kubeadm workflow for Kubernetes 1.35.1 using only init-kubeadm and check-cluster builders"
```

Expected outcome:

- route: `draft`
- clarification: should only stop if there is a real execution gap
- external evidence: may be used for version-sensitive kubeadm facts if required by the evidence plan
- output shape: should produce a minimal apply-only workflow, not an unrelated prepare/apply expansion
- validity: generated files should pass `deck lint`

## Suite C: Seeded-workspace refine checks

Create a temporary workspace seeded from `test/workflows`.

```bash
tmpdir=$(mktemp -d)
mkdir -p "$tmpdir/workflows"
cp -R test/workflows/. "$tmpdir/workflows/"
cd "$tmpdir"
```

### Prompt C1: vars hoist refine

```bash
/home/opencode/workspace/deck/bin/deck ask --edit "Refactor workflows/scenarios/control-plane-bootstrap.yaml to use workflows/vars.yaml for repeated values"
```

Expected outcome:

- route: `refine`
- clarification: should not drift into unrelated scenario files
- local facts: should rely on deck-owned refine candidates and path rules
- output shape: should keep the requested anchor file stable while updating `workflows/vars.yaml` only if needed
- validity: resulting workspace should pass `deck lint`

## How to compare before and after

For each issue, capture a short note set:

- what changed
- which prompts were run
- which outputs improved
- whether any route, evidence, clarification, or validity behavior regressed

Short notes are enough. The important part is that contributors can compare the same prompts directly.

## When the suite may be narrowed

Small changes can use a smaller subset only when the untouched areas are obviously unaffected.

Examples:

- a local-facts-only refactor may skip the refine fixture if no authoring selection path changed
- an external evidence adapter fix may focus on A2 plus one authoring prompt that depends on evidence gating

If there is doubt, run the full suite.

## When a change fails the suite

- Do not hide the regression behind a compatibility shim.
- Either fix the new path, narrow the scope, or split the work into a follow-up issue.
- If a temporary shim is unavoidable, document the exact removal issue before merging.
