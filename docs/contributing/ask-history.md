# Ask History

This document records the major architectural phases of `deck ask` authoring and the quality problems that led to the current transition discussion.

## Why this exists

- `deck ask` has already gone through one major authoring redesign.
- We need a short historical record before making the next structural change.
- The goal is not to defend the current pipeline. The goal is to remember what failed, why we changed it, and what new failure mode replaced the old one.

## Stage 1: direct workflow generation

In the first authoring design, the model generated workflow files directly.

- The model returned file paths and YAML content.
- Ask validated the generated files and wrote them on success.
- This matched how general CLI coding agents work: read context, write files, run validation, fix failures.

Representative commits:

- `d1b96f4` `add ai-ready ask authoring command`
- `cac11de` `feat: add document-based ask generation contract`
- `3bf91cd` `feat: route ask generation and refine through document IR`

### Stage 1 strengths

- The model worked on the real output artifact: workflow files.
- The prompt/problem shape was easy to understand.
- Successful generations could look close to what a CLI coding agent would produce.

### Stage 1 failures

- YAML often failed schema or lint validation.
- The model sometimes invented typed steps or fields that did not exist.
- The model could drift from repository path/layout rules.
- Repair loops often had to fix large structural mistakes after generation.

### Stage 1 lesson

Direct authoring gave the model the right output target, but not enough guardrails. The model had too much responsibility for schema-correct workflow construction.

## Stage 2: constrained selection and code-owned assembly

The second design moved authoring responsibility out of the model and into code.

- Ask added planning, plan review, clarification, and repair loops.
- Draft moved to builder selection instead of raw workflow authoring.
- Refine moved to transform candidates instead of free-form edits.
- Code compiled selected builders/transforms into workflow documents.
- Legacy free-form authoring fallbacks were removed so the constrained path became the default runtime.

Representative commits:

- `c253870` `feat: add ask agentic planning and repair loops`
- `3d3850d` `refactor: require planning before ask authoring`
- `72526e7` `refactor: structure ask planning and generation contracts`
- `f5fff83` `refactor: add constrained ask draft and refine transforms`
- `725c40d` `refactor: drive ask draft and refine from candidates`
- `eb75e32` `refactor: add executable ask authoring programs`
- `91539d9` `refactor: compile ask drafts from catalog bindings`
- `f567f8b` `refactor: enforce ask primary authoring contracts`
- `3302d27` `maintenance: remove ask legacy authoring fallback`

### Stage 2 strengths

- Source-of-truth stayed in schema, metadata, validation, and path rules.
- Code could reject unsupported fields, paths, and missing bindings early.
- Draft/refine output became more controlled and safer to validate.
- The runtime became easier to reason about in terms of contracts.

### Stage 2 failures

- The model now had to solve an ask-specific intermediate contract instead of the real workflow authoring problem.
- Failures shifted from invalid workflow YAML to invalid builder ids, unsupported overrides, candidate mismatches, and repair-contract violations.
- Planning, critic, clarification, second-pass retrieval, judge, and postprocess increased the number of stages where authoring could stop before producing files.
- Prompt context still described real workflow structure, but generation asked the model to answer in a narrower ask-local DSL.
- Builder and transform coverage could lag behind legitimate workflow requests.

### Stage 2 lesson

The second design reduced some schema-level failures, but it replaced them with pipeline-contract failures. The model became better constrained, yet farther away from the real output artifact.

## Current diagnosis

The current runtime has a split brain.

- Retrieval and examples still push the model to reason about actual workflow files.
- The primary authoring contract asks the model to return builder selections or transform selections.
- Code owns more correctness, but the model must first satisfy ask-local abstractions that general CLI agents do not have.

This is why `deck ask` can underperform even on deck-specific authoring tasks.

- General CLI agents work on the actual files and fix validator feedback directly.
- `deck ask` often fails one layer earlier on plan or contract shape.

## What should guide the next redesign

- Keep source-of-truth outside ask.
- Keep path and validator guardrails in code.
- Reduce ask-local intermediate contracts on the default authoring path.
- Measure quality against real workflow outcomes, not just contract compliance.
- Prefer a smaller execution loop that writes, validates, and repairs the real artifact.

## Related docs

- [Ask V3 Authoring](ask-v3-authoring.md)
- [Ask Authoring Pipeline](ask-authoring-pipeline.md)
- [Ask Quality Evaluation](ask-quality-evaluation.md)
