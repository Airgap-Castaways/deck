# Internals

This document describes the internal Go package layout and safety model for contributors. Operator-level readers should see [Architecture](../core-concepts/architecture.md) instead.

## How the codebase maps to the architecture

The code layout roughly follows the system boundaries described in the architecture doc:

- `cmd/`: CLI entrypoints
- `internal/config` and `internal/workflowexec`: workflow contracts, decoding, and execution rules
- `internal/ask*`: routed AI-assisted question, review, planning, draft, refine, compile, and repair flow
- `internal/prepare` and `internal/preparecli`: connected-side preparation logic
- `internal/install`: target-side host mutation and apply behavior
- `internal/bundle`: bundle collection, import, merge, and verify logic
- `internal/server`: optional site-local HTTP server
- `internal/fsutil`, `internal/filemode`, `internal/hostfs`, `internal/executil`: safety-oriented helper boundaries

The exact package layout may continue to evolve, but the architectural direction stays the same: thin CLI layer, typed workflow boundary, explicit trust boundaries, and minimal hidden behavior.

## Ask authoring internals

`deck ask` follows the same source-of-truth direction as the rest of the system. It is not meant to be a parallel workflow-definition system.

For authoring routes, the intended shape is route classification first, clarify when blocking ambiguity remains, then bounded tool loop. In practice, that means:

- code classifies whether the request is question, explain, review, draft, refine, or clarify
- code runs preflight for scope, target inference, and blocking clarifications
- the model operates on real workspace files through a bounded tool loop (read, write, edit, validate, schema)
- code enforces write scope, validation gates, and candidate state management
- code auto-repairs schema and role violations after each validation
- code writes accepted candidate files to disk only after successful finish

The important architectural boundary is that ask may project canonical facts for prompting and assembly, but it should not own a second copy of step validity, field enums, or workspace path rules.

At the code level, `internal/askcontext` is the main home for those canonical ask facts: command metadata, prompt/source-of-truth bundles, and schema-derived authoring context live there, while orchestration stays in `internal/askcli`. That split keeps the metadata surface easier to discover without changing ask behavior.

For the runtime shape, authoring loop detail, and guardrails, see [Ask Agent Runtime](ask-agent-runtime.md).

## Safety and trust boundaries

The principle is that feature code should describe intent while sensitive operations stay localized in small helper layers that are easier to audit.

Important trust boundaries include:

- **filesystem path resolution**: rooted path helpers constrain how paths are resolved within bundle, site, and state roots
- **host path mutation**: host-oriented writes stay explicit rather than being mixed into generic path handling
- **file mode policy**: common permission patterns are centralized instead of open-coded everywhere
- **command execution**: execution helpers separate workflow-driven commands from broader system-level capabilities
- **HTTP response and template rendering**: server output is localized in small response and template helpers

This reduces the need for broad security suppressions in feature code and keeps risky behavior easier to reason about.

## Extending the system

New capabilities should follow the same shape.

- prefer adding a typed step over expanding `Command` usage
- prefer extending an existing noun family before introducing a new top-level step kind
- keep runtime side effects in focused helper boundaries
- keep prepare-side network work out of apply-side host mutation paths
- avoid expanding override and composition rules unless the added flexibility is clearly worth the extra operator complexity
- document workflow and schema changes together
- keep the default path local-first even when optional server features expand

## Compatibility and legacy removal

`deck` is still unreleased, so the project currently prefers converging on a smaller canonical model over carrying legacy compatibility layers for every abandoned design.

That means transitional wrappers, duplicate command surfaces, legacy step kinds, and temporary compatibility shims are expected to be removed once the preferred shape becomes clear. The goal is to keep the architecture understandable before release rather than accumulating historical layers that would have to be supported indefinitely.

For the same reason, pre-v1.0.0 releases are allowed to make breaking changes to workflow schemas, CLI contracts, bundle structure, and other published contracts when that is the clearest way to converge on the intended model.

After release, the compatibility posture is expected to change. At that stage, published contracts should be stabilized through explicit version boundaries such as `apiVersion`, with compatibility managed at those documented edges rather than through implicit support for every previous internal structure.

In other words, compatibility should live at clear boundaries such as workflow schemas, bundle contracts, and published APIs. Internal package layout and transitional implementation details are not the main stability target.

## Related references

- [Architecture](../core-concepts/architecture.md)
- [Ask Agent Runtime](ask-agent-runtime.md)
- [Legacy Compatibility](legacy-compatibility.md)
