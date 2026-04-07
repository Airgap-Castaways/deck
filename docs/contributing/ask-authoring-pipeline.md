# Ask Authoring Pipeline

This document describes the current `deck ask` authoring architecture for contributors.

## Goal

- Keep route classification ahead of generation, with LLM-assisted classification for normal requests.
- Block ambiguous authoring through clarification instead of weak generation.
- Let the model select among constrained options.
- Let code assemble, transform, validate, and repair workflow documents.
- Keep source-of-truth in `stepmeta`, generated schema, and validation layers rather than in ask-local rule maps.
- Keep the pipeline small enough that contributors can explain each stage in terms of facts, judgment, execution, or validation.

## Execution rules for contributors

- Run a before/after `ask` quality suite for meaningful pipeline changes. See [Ask Quality Evaluation](ask-quality-evaluation.md).
- Prefer deleting replaced paths over keeping long-lived legacy fallbacks.
- If a stage mixes authoritative facts with recommendation behavior, split or rename it before adding more logic.
- If a simplification improves maintainability but weakens workflow quality, it is not done yet.

## High-level flow

The current runtime pipeline is still more detailed than the simplified target architecture.

For `draft` and `refine`, the current flow is:

1. classify the request
2. build an external evidence plan
3. gather route-specific facts (`workspace`, `local-facts`, `examples`, `external evidence`, local state)
4. build an execution plan
5. run `plan critic`
6. stop for clarification or plan-review gating when the plan is not ready to execute
7. run a second retrieval pass after clarification answers when needed
8. collect constrained draft builder or refine transform candidates
9. let the model choose among those candidates
10. compile or transform workflow documents in code
11. validate the result
12. apply automatic repair when structured issues permit it
13. optionally run `judge`
14. optionally run targeted `postprocess` review/edit for blocking operational issues
15. write files only after validation succeeds

For `question`, `explain`, and `review`, the current flow is:

1. classify the request
2. build an external evidence plan
3. gather route-specific facts
4. answer with explicit evidence boundaries

Non-authoring routes do not enter the compile path.

The target simplification direction is still to reduce the number of overlapping review and prompt-reinterpretation stages, but the list above reflects the current code path more accurately than the simplified end-state.

## Core rule: source-of-truth stays outside ask

The ask pipeline may project canonical workflow facts into an authoring-friendly catalog, but it must not become a second schema system.

- Step validity belongs to `internal/stepmeta`, generated schema, and `internal/validate`.
- Canonical workspace path rules belong to `internal/workspacepaths`.
- Ask-side packages may project those facts for prompting, planning, compile-time filling, and repair.
- Ask-side packages must not re-declare step enums, defaults, field ownership, or path truth unless the logic is a narrow migration shim.

This rule is the main defense against drift between authoring behavior and actual workflow validation.

## Main packages and responsibilities

- `internal/askcli`: top-level route orchestration, LLM calls, planning flow, generation loop, and final write behavior.
- `internal/askaugment/mcp`: built-in external-docs provider registry, capability adapters, health checks, and evidence normalization.
- `internal/askpolicy`: plan normalization, blocking decisions, and `AuthoringProgram` derivation.
- `internal/askevidenceplan`: external evidence planning and upstream-doc need detection.
- `internal/askcontract`: typed contracts shared across planning, generation, compile, refine, and repair.
- `internal/askcontext`: build prompt-facing workspace and authoring context.
- `internal/askretrieve`: merge workspace facts, local facts, examples, local state, and external evidence into route-specific chunks.
- `internal/askcatalog`: project source-of-truth step and field metadata into an ask-facing catalog.
- `internal/askdraft`: expose draft builder candidates and compile selected builders into workflow documents.
- `internal/askrefine`: compute refine transform candidates from parsed workflow structure.
- `internal/askrepair`: map structured diagnostics to automatic repair operations.
- `internal/askir`: materialize generated documents into files and apply structured edits.

Some current package names are broader than their long-term architectural role. Contributors should treat this document as the intent boundary even when package names still reflect older terminology.

## Execution contract

The execution contract for authoring is carried by `internal/askcontract.PlanResponse`.

Important fields include:

- `AuthoringBrief`: route intent, scope, topology, coverage requirements, and refine boundary information.
- `AuthoringProgram`: normalized platform, artifact, cluster, and verification facts that code can consume directly.
- `ExecutionModel`: execution and verification shape derived during planning.
- `Clarifications`: code-generated questions that can block generation.
- `Files`: intended output files and actions.

Treat this plan as executable input, not as documentation text for the model to reinterpret later.

## Current runtime notes

The contributor-facing intent and the current runtime still differ in a few places.

- `plan critic` can still stop authoring even when the planner returned no explicit blockers.
- a second retrieval pass can happen after clarification answers are applied.
- `judge` and targeted `postprocess` still exist on the default authoring path, even after structural cleanup removal.
- `required external evidence` on authoring routes still stops execution immediately when the evidence planner marks the request as externally dependent.

When updating this document, prefer describing the actual runtime first and the desired simplification second.

## AuthoringProgram

`AuthoringProgram` exists to carry stable authoring facts that should not be repeatedly re-authored by the model.

Examples include:

- platform family and release
- package and image output directories
- Kubernetes join-file path
- role selector and node counts
- verification expectations such as total nodes and control-plane ready count

Draft compilation and repair both consume the same program so they stay aligned.

## Draft path

Draft no longer treats the model as the author of raw `step.spec` payloads.

- `internal/askdraft.Candidates` exposes builder candidates derived from the projected catalog and plan scope.
- The model returns `selection.targets[].builders[]` plus limited override values.
- `internal/askdraft.CompileWithProgram` resolves bindings from:
  - allowed overrides
  - `AuthoringProgram`
  - derived values
  - constants declared in metadata
- Code assembles workflow steps and documents from those bindings.

There is no longer a legacy free-form document fallback on the draft path. Retries must stay inside builder selection rather than switching to open-ended YAML or document authoring.

If a builder cannot produce a valid required binding, compilation fails with a structured error instead of letting a malformed step leak into output.

## Refine path

Refine is selection-based as well.

- Code parses the existing workflow structure.
- Code computes candidate transforms that are in scope for the requested anchor files.
- The model selects transform candidates by id.
- Code applies the selected transforms and re-renders documents.

There is no longer a legacy full-document rewrite fallback on the refine path. Retries must stay inside structured edit documents with code-owned transforms.

Refine should keep anchor files stable and only expand into allowed companion files declared by plan policy.

In the current runtime, refine still has a known sharp edge: the model can return raw transform payloads instead of transform candidate ids, which then fails the primary refine contract. The intended contract is selection-by-candidate-id, but this remains an active quality gap rather than a fully solved property.

## Evidence planning and external docs

External docs are not gathered blindly.

- `internal/askevidenceplan` builds an `EvidencePlan` that marks external evidence as `required`, `optional`, or `unnecessary`.
- Heuristics cover obvious freshness-sensitive requests such as versioned install, compatibility, prerequisite, and troubleshooting asks.
- A small LLM evidence-planning pass may refine ambiguous external entity selection when heuristics cannot identify the upstream technology cleanly.

This keeps ask off a raw tool-calling loop while still letting unfamiliar products route into external evidence lookup.

Known limitation: evidence heuristics are intentionally conservative, but they can still misclassify some prompts. Current work has reduced false positives for local refine/code-explain requests, but evidence planning should still be treated as a place where regressions are possible and should be covered by live quality checks.

## Built-in MCP providers and capability adapters

`ask.mcp.servers[]` still exists in config, but ask now treats it as built-in provider selection plus optional transport override.

Current built-in provider ids are:

- `context7`
- `web-search`

`web-server` remains a compatibility alias for `web-search`.

Ask core should not depend on raw MCP tool names. Provider adapters own capability routing. The current capability families include:

- `entity-resolve`
- `official-doc-search`
- `doc-fetch`
- `web-search`
- `error-lookup`

Provider-specific adapter code is responsible for translating those capabilities into actual MCP tool calls, argument shapes, and multi-step flows.

## Local facts versus external evidence

Current code uses `local-facts` for local authoritative fact blocks that describe deck-owned source-of-truth.

Retrieval now separates three concerns:

- local facts for deck source-of-truth
- external evidence for upstream product behavior and recency
- normal workspace and example context

Local facts are authoritative for:

- step metadata and builder behavior from `internal/stepmeta`
- typed step metadata from `internal/stepspec/*_meta.go`
- draft compilation behavior from `internal/askdraft`
- planning defaults and authoring rules from `internal/askpolicy`
- repair semantics from `internal/askrepair`

External evidence is only authoritative for upstream facts such as:

- current install steps
- version-sensitive changes
- compatibility and prerequisite notes
- troubleshooting guidance

Prompts must preserve that boundary explicitly. External docs must not override local schema truth, validator rules, workflow path rules, or repair behavior.

If a local-facts block starts behaving like ranked guidance or candidate recommendation, that is a design smell. Facts and guidance should be separate concerns.

## Required evidence failure behavior

When required external evidence cannot be fetched:

- answer routes should surface the limitation explicitly and avoid guessing fresh facts
- authoring routes should stop instead of writing version-sensitive output based on stale or missing upstream context

This failure mode is intentional. It is safer than silently continuing with weak external assumptions.

The important distinction is between a true external-facts dependency and a heuristic false positive. If a local workspace request is incorrectly marked `required`, that is a planner bug and should be fixed rather than normalized as expected behavior.

## Repair path

Repair is automatic first.

- Validation emits structured diagnostics.
- `internal/askrepair` maps diagnostics to repair operations.
- Common repairs run in code using projected metadata and `AuthoringProgram` values.
- Model selection should only be needed when multiple valid repair operations remain.

Avoid broad document replacement when a narrow structured repair exists.

Current runtime note: automatic repair is more reliable on draft schema/field errors than on refine contract violations. Refine failures that come from candidate-id contract mismatch still often require prompt or generation-contract fixes rather than repair-only fixes.

## Known quality gaps

These are current runtime gaps that contributors should treat as known limitations, not intended guarantees:

- local code explain can still have shallow fact coverage when `local-facts` blocks do not surface enough structured detail
- some draft builder selections still produce unsupported overrides or null-required fields and fail after retries
- refine can still stop on contract violations when model output uses raw transforms instead of transform candidate ids
- authoring plan review can still feel over-conservative relative to the user request, especially in multi-node cases where intent is mostly implied but not fully explicit

## Prompting boundary

Prompts should describe:

- available routes
- clarification needs
- candidate builders or transforms
- allowed override keys
- normalized program facts the model can rely on

Prompts should not ask the model to restate canonical low-level workflow truth that code already owns.

## Contributor checklist

When changing ask authoring behavior:

- confirm the change preserves source-of-truth boundaries
- prefer adding metadata or projection fields over ask-local hardcoded rule tables
- keep draft output on builder selection, not raw step authoring
- keep refine output on transform selection, not raw path editing
- keep clarification decisions code-driven
- keep repair automatic-first and structured
- run the before/after `ask` quality suite for meaningful pipeline changes
- remove superseded paths, labels, and aliases instead of leaving dormant compatibility branches
- run `make test && make lint`

## Related docs

- [Using deck ask](../guides/ask.md)
- [Ask Quality Evaluation](ask-quality-evaluation.md)
- [Comment-Driven Step Metadata](comment-driven-step-metadata.md)
- [Architecture](../core-concepts/architecture.md)
