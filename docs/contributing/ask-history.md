# Ask History

This document records the short architectural history behind the current `deck ask` runtime. For the current implementation, use [Ask Agent Runtime](ask-agent-runtime.md).

## Why this exists

- `deck ask` has already gone through multiple authoring designs.
- The current runtime makes more sense when contributors know what failed before.
- This document is for background only, not for current behavior.

## Stage 1: direct workflow generation

The earliest authoring path let the model generate workflow files directly.

- the model returned workflow paths and YAML content
- ask validated the generated files and wrote them on success
- this kept the model close to the real artifact, but schema and path errors were common

Main lesson: direct authoring targeted the right artifact, but left too much workflow correctness to the model.

## Stage 2: planner/compiler-heavy authoring

The next design moved more correctness into code.

- ask added planning, plan review, clarification, and repair stages
- draft moved to builder selection instead of raw YAML generation
- refine moved to transform candidates instead of free-form edits
- code compiled the selected intermediate representation back into workflow files

This improved structural control, but it introduced a new problem: the model often failed the ask-specific contract before it ever reached a good workflow result.

Main lesson: pushing correctness into code was right, but the model became too far removed from the real file-editing task.

## Stage 3: bounded agent runtime

The current runtime keeps the model closer to real workflow files again, but with stronger code-owned guardrails.

- authoring uses a bounded tool loop over real files
- candidate state, path scope, and final writes stay code-owned
- `deck_lint` gates finish before disk writes happen
- analyze routes remain read-only, and `deck ask plan` stays read-only as well

Main lesson: the default path works best when the model can inspect and edit real files, while deck keeps validation, boundaries, and policy in code.

## Design principles that survived the transitions

- keep source-of-truth outside ask
- keep path scope and validation in code
- block true ambiguity with clarification instead of guessing
- measure quality by final workflow outcomes, not by intermediate contract elegance
