# Ask Agent Runtime

This document describes the default `deck ask` authoring runtime.

## Goal

- Make `deck ask --create` and `deck ask --edit` run as a bounded tool loop over real workflow files.
- Keep path scope, validation, persistence, and clarification policy in code.
- Keep `deck ask plan` read-only.

## Runtime split

- `analyze mode`
  - entrypoints: `deck ask`, `deck ask plan`, `deck ask --review`
  - remains read-only
  - may still gather MCP/web evidence up front when policy requires it
- `author mode`
  - entrypoints: `deck ask --create`, `deck ask --edit`
  - uses the agent runtime by default
  - keeps MCP/web lookup as an optional in-loop tool instead of prefetching it for every run

## Authoring loop

1. classify the route and inspect the workspace
2. run code-owned preflight for scope, target inference, and true blockers
3. build a session with approved paths, candidate file state, and retry budgets
4. ask the model for the next tool action, clarification, or finish signal
5. execute tool calls in order and append structured tool results to the session transcript
6. keep iterating until `deck_lint` succeeds and the model finishes, a clarification is required, or the turn budget is exhausted
7. write accepted candidate files to disk and persist the session transcript under `.deck/ask`

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

- write scope is derived from preflight and enforced before candidate state mutates
- refine requests stay inside anchor and approved companion paths
- `finish` is rejected until `deck_lint` succeeds in the current session
- tool calls and verifier output are persisted for replay/debugging in `.deck/ask/last-agent-session.json`
- final disk writes still go through the normal scaffold and file validation helpers

## Migration note

- The older v3 action loop has been removed from the default authoring path.
- Default author mode now routes through the bounded agent runtime tool loop.
