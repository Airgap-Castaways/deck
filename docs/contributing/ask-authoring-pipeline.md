# Ask Authoring Pipeline

This document describes the current `deck ask` authoring architecture for contributors.

## Goal

- Keep route classification ahead of generation, with LLM-assisted classification for normal requests.
- Block ambiguous authoring through clarification instead of weak generation.
- Let the model select among constrained options.
- Let code assemble, transform, validate, and repair workflow documents.
- Keep source-of-truth in `stepmeta`, generated schema, and validation layers rather than in ask-local rule maps.

## High-level flow

For `draft` and `refine`, the pipeline is:

1. classify the request
2. gather workspace and schema-derived context
3. build an execution plan
4. normalize an `AuthoringProgram`
5. stop for clarification when the plan still has blocking gaps
6. collect constrained draft builder or refine transform candidates
7. let the model choose among those candidates
8. compile or transform workflow documents in code
9. validate the result
10. apply automatic repair when structured issues permit it
11. write files only after validation succeeds

Non-authoring routes do not enter the compile path.

## Core rule: source-of-truth stays outside ask

The ask pipeline may project canonical workflow facts into an authoring-friendly catalog, but it must not become a second schema system.

- Step validity belongs to `internal/stepmeta`, generated schema, and `internal/validate`.
- Canonical workspace path rules belong to `internal/workspacepaths`.
- Ask-side packages may project those facts for prompting, planning, compile-time filling, and repair.
- Ask-side packages must not re-declare step enums, defaults, field ownership, or path truth unless the logic is a narrow migration shim.

This rule is the main defense against drift between authoring behavior and actual workflow validation.

## Main packages and responsibilities

- `internal/askcli`: top-level route orchestration, LLM calls, planning flow, generation loop, and final write behavior.
- `internal/askpolicy`: plan normalization, blocking decisions, and `AuthoringProgram` derivation.
- `internal/askcontract`: typed contracts shared across planning, generation, compile, refine, and repair.
- `internal/askcontext`: build prompt-facing workspace and authoring context.
- `internal/askcatalog`: project source-of-truth step and field metadata into an ask-facing catalog.
- `internal/askdraft`: expose draft builder candidates and compile selected builders into workflow documents.
- `internal/askrefine`: compute refine transform candidates from parsed workflow structure.
- `internal/askrepair`: map structured diagnostics to automatic repair operations.
- `internal/askir`: materialize generated documents into files and apply structured edits.

## Execution contract

The execution contract for authoring is carried by `internal/askcontract.PlanResponse`.

Important fields include:

- `AuthoringBrief`: route intent, scope, topology, coverage requirements, and refine boundary information.
- `AuthoringProgram`: normalized platform, artifact, cluster, and verification facts that code can consume directly.
- `ExecutionModel`: execution and verification shape derived during planning.
- `Clarifications`: code-generated questions that can block generation.
- `Files`: intended output files and actions.

Treat this plan as executable input, not as documentation text for the model to reinterpret later.

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

If a builder cannot produce a valid required binding, compilation fails with a structured error instead of letting a malformed step leak into output.

## Refine path

Refine is selection-based as well.

- Code parses the existing workflow structure.
- Code computes candidate transforms that are in scope for the requested anchor files.
- The model selects transform candidates by id.
- Code applies the selected transforms and re-renders documents.

Refine should keep anchor files stable and only expand into allowed companion files declared by plan policy.

## Repair path

Repair is automatic first.

- Validation emits structured diagnostics.
- `internal/askrepair` maps diagnostics to repair operations.
- Common repairs run in code using projected metadata and `AuthoringProgram` values.
- Model selection should only be needed when multiple valid repair operations remain.

Avoid broad document replacement when a narrow structured repair exists.

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
- run `make test && make lint`

## Related docs

- [Using deck ask](../guides/ask.md)
- [Comment-Driven Step Metadata](comment-driven-step-metadata.md)
- [Architecture](../core-concepts/architecture.md)
