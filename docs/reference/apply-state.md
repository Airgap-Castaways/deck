# Apply State

`deck apply` keeps workflow progress in a state file so later runs can skip already completed steps.

## What identifies a saved run

The saved state path is derived from the workflow `StateKey`.

`StateKey` is computed from:

- the resolved workflow bytes after phase imports are expanded
- the effective vars used for that run, including `workflows/vars.yaml`, scenario `vars:`, and CLI `--var` overrides

This means saved apply state is isolated by the resolved workflow fingerprint plus effective vars.

## What is stored

The apply state records:

- completed step IDs
- skipped step IDs
- runtime vars registered by completed steps
- the current phase
- the last failed step and error, when a run stops on failure

## How completed-step skipping works

During apply, `deck` checks the saved `completedSteps` list before it evaluates the current step.

- if the current step ID is already marked completed, the step is skipped with reason `completed`
- otherwise `when:` is evaluated and the step either runs or is skipped with reason `when`

This is why a repeated apply run usually resumes instead of re-running the whole scenario.

## What happens when workflow content changes

If step content changes, the resolved workflow bytes change.

That changes the workflow state key, which points apply at a different state file.

In practice:

- changing a step body, `when`, `register`, `timeout`, phase import, or other workflow content usually produces a new state file
- changing effective vars also produces a new state file

The old state is not reused across different workflow fingerprints.

## What happens when a step ID changes

Changing a step ID also changes the resolved workflow bytes, so the workflow usually gets a new state key.

If an operator somehow forces reuse of an older state file, completed-step matching still uses step ID only. In that case the renamed step is treated as not completed and can run again.

## Shared component steps across scenarios

Component fragments under `workflows/components/` are expanded into each scenario before the workflow state key is computed.

That means shared component steps are not tracked globally by component path.

Instead they are tracked as part of each resolved scenario workflow.

In practice:

- different scenarios usually get different state files because their resolved workflows differ
- two scenarios can only share state if their final resolved workflow bytes and effective vars are identical

## `--fresh`

Use `--fresh` to ignore any saved workflow state for the current command.

- `deck apply --fresh` starts from an empty in-memory state, then writes new progress back to the normal state path as steps complete
- `deck plan --fresh` shows the plan without completed-step skips or saved runtime vars from prior runs

`--fresh` does not delete the state file before execution. It simply ignores the saved contents for that invocation.

## Step selection

`deck apply` and `deck plan` support step-level selection:

- `--step <id>` selects one step
- `--from-step <id>` selects from that step to the end
- `--to-step <id>` selects from the beginning through that step
- `--from-step <id> --to-step <id>` selects an inclusive range

Selectors filter the execution workflow. They do not automatically include prerequisite steps that register runtime values for later steps.

If a selected step depends on earlier registered runtime vars, use a wider selector or re-run the full scenario.
