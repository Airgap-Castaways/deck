# Ask Agent Runtime

This document is the contributor-facing source of truth for the current `deck ask` architecture.

## Goal

- Keep `deck ask` useful for both read-only analysis and workflow authoring.
- Let authoring work against real workflow files through a bounded tool loop.
- Keep scope, validation, clarification, and final writes code-owned.
- Keep `deck ask plan` read-only.

## Runtime split

`deck ask` now has two execution shapes.

- `analyze mode`
  - used for `question`, `explain`, `review`, and `plan`
  - stays read-only
  - may gather external evidence up front when policy requires it
- `author mode`
  - used when the request resolves to `draft` or `refine`, including explicit `--create` and `--edit`
  - runs the bounded agent runtime over real workspace files
  - exposes external lookup as an optional tool instead of prefetching it for every run

## Authoring loop

Author mode follows this loop:

1. classify the request and inspect the workspace
2. run code-owned preflight for scope, target inference, and true blockers
3. create a session with approved paths, candidate file state, and retry budgets
4. ask the model for the next tool action, clarification, or finish signal
5. execute tool calls in order and append structured results to the session transcript
6. continue until `deck_lint` succeeds and the model finishes, a clarification is required, or the turn budget is exhausted
7. write accepted candidate files to disk and persist the session transcript under `.deck/ask`

The important boundary is that the model operates on file tools, not on a deck-specific builder DSL. The runtime keeps the model close to the real artifact while still enforcing deck rules in code.

## Available authoring tools

- `file_search`
  - searches approved workflow files and local example files
- `file_read`
  - reads approved workflow files and read-only examples
- `file_write`
  - replaces an approved target file in session-owned candidate state
- `deck_init`
  - prepares scaffold metadata for an empty workspace
- `deck_lint`
  - validates the current candidate state and returns structured diagnostics
- `mcp_web_search`
  - optional external evidence lookup when config and evidence policy allow it

## Guardrails

- write scope is derived during preflight and enforced before candidate state mutates
- refine stays inside the anchor and any code-approved companion paths
- `file_write` updates candidate state first; disk writes happen only after successful finish
- the internal `init` tool is the exception: in an empty workflow workspace it may create the minimal scaffold directories, ignore files, and output `.keep` files needed before final workflow writes
- `finish` is rejected until `deck_lint` succeeds in the current session
- tool calls and verifier output are persisted in `.deck/ask/last-agent-session.json`
- final disk writes still go through normal scaffold and validation helpers

## Source-of-truth boundary

Ask may project deck facts into prompts and runtime tools, but it must not become a second schema system.

- step and field validity belong to `internal/stepmeta`, generated schema, and `internal/validate`
- shared ask authoring defaults belong to `internal/askdefaults`, not duplicated in prompt, policy, or repair code
- canonical workflow path rules belong to workspace/path helpers, not ask prompts
- workspace scaffold primitives belong to `internal/initcli`; ask must not maintain a parallel copy of `deck init` layout logic
- evidence planning decides when upstream docs are needed, but external docs do not override deck-owned workflow truth
- clarification decisions, path scope, and finish gating stay code-driven

If an ask change requires hardcoding workflow truth in prompts or contracts that already exists elsewhere in deck, it is usually the wrong abstraction.

## Main packages

- `internal/askcli`: route orchestration, runtime loop, model calls, evidence planning, prompt construction, and final write behavior
- `internal/askcontract`: author/runtime response contracts, parsing, and provider type definitions
- `internal/askpolicy`: preflight, scope rules, clarification policy, authoring decisions, and authoring fact inference
- `internal/askcli/evidence_plan.go`: heuristic external evidence planning (moved from former `askevidenceplan` package)
- `internal/askcontext` and `internal/askretrieve`: prompt-facing workspace and retrieved context assembly
- `internal/askaugment/mcp`: built-in external evidence providers and adapters
- `internal/askstate`: persisted ask state and agent session transcripts
- `internal/askir`: workflow document parsing (ParseDocument, Summaries)
- `internal/askrepair`: auto-repair for schema and role violations after validation
- `internal/askprovider`: LLM provider abstraction (re-exports types from `askcontract/provider.go`)

## Runtime tuning

- **Turn budget**: max 30 turns per authoring session (`agentRuntimeMaxTurns`)
- **Verification budget**: 5 validation failures before the session stops (`generationAttempts`)
- **Schema caching**: repeated schema tool calls for the same topic return cached results with a hint to proceed
- **Read-only loop breaker**: after 3 consecutive read-only turns (read/grep/glob/schema), the runtime restricts tools to action-only (file_write, file_edit, validate, finish)
- **Auto-validate**: after 3 consecutive write turns without a validate call, the runtime auto-triggers validation
- **Step kind reference**: the runtime pre-loads typed step schemas (summary, key fields, examples, when/template syntax) into the prompt based on plan requirements, eliminating the need for the model to discover schemas via the schema tool
- **Prepare auto-remove**: if prepare.yaml only contains disallowed step kinds for the prepare role, it is automatically removed from candidates during repair

## Verification

Meaningful ask changes need both automated checks and a small live suite.

### Required automated checks

Run:

- targeted package tests for the changed subsystem
- regression tests for the motivating failure mode
- `make test && make lint`

### Required live quality checks

Build the current branch first:

```bash
make build
```

For each prompt, record:

- expected route
- whether external evidence was used or intentionally avoided
- whether clarification was correctly triggered or avoided
- for authoring routes, whether the final output matched the requested shape and passed `deck lint`

Run this baseline suite.

#### Suite A: repo-root informational checks

```bash
./bin/deck ask "Explain the typed step builders defined in internal/stepspec for DownloadPackage, InstallPackage, InitKubeadm, and JoinKubeadm."
./bin/deck ask "Explain how to install kubeadm 1.35.1."
```

#### Suite B: empty-workspace authoring checks

```bash
tmpdir=$(mktemp -d)
cd "$tmpdir"
/home/opencode/workspace/deck/bin/deck ask --create "Create a minimal single-node apply-only offline kubeadm workflow for Kubernetes 1.35.1 using only init-kubeadm and check-kubernetes-cluster builders"
/home/opencode/workspace/deck/bin/deck ask --create "Create a single-node apply-only workflow that installs Docker and enables the docker service on Ubuntu 24.04 using typed steps where possible."
/home/opencode/workspace/deck/bin/deck ask --create "Create a 3-node offline kubeadm workflow with prepare and apply phases."
```

#### Suite C: seeded-workspace refine checks

```bash
tmpdir=$(mktemp -d)
mkdir -p "$tmpdir/workflows"
cp -R test/workflows/. "$tmpdir/workflows/"
cd "$tmpdir"
/home/opencode/workspace/deck/bin/deck ask --edit "Refactor workflows/scenarios/control-plane-bootstrap.yaml to use workflows/vars.yaml for repeated values"
/home/opencode/workspace/deck/bin/deck ask --edit "Keep workflows/scenarios/control-plane-bootstrap.yaml stable while extracting only clearly repeated structure into companion files when justified."
```

If maintainability improves but this suite regresses, the change is not done yet.
