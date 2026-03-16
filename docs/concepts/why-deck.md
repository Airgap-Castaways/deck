# Why deck

`deck` grew out of a specific situation: air-gapped Kubernetes operations where SSH-driven tooling was not available, internet access was not assumed, and the shell scripts running maintenance procedures had grown large enough that reviewing them before execution was genuinely hard.

The problem was not just that shell is fragile. It was that a long shell file hides what the procedure actually does. Intent gets buried inside implementation. Reviews become reverse-engineering sessions. Reuse turns into copy-paste.

`deck` gives those procedures a cleaner shape. A typed `Packages` step says more than a block of `apt install` commands. A named phase boundary makes the procedure easier to scan. `deck lint` catches structural mistakes before the bundle leaves the connected environment. The bundle itself travels with the workflow and the artifacts it needs — no implicit dependencies, no reach-back to external services at run time.

## Background

Most infrastructure and configuration tools are built around online environments. They often assume one or more of the following:

- a reachable control plane
- network access during execution
- remote access such as SSH
- a larger language or package runtime already present on the target machine

Those assumptions are reasonable in many environments, but they become friction in air-gapped operations.

Shell scripts are the usual fallback because they run almost anywhere on Linux and are easy to pass around. But that convenience fades as the procedure grows. Once a script becomes large enough, the cost of reviewing, reusing, and safely modifying it rises quickly.

## Constraints that shaped deck

`deck` was shaped by a few recurring constraints from real operations.

- Using tools such as Ansible could introduce Python and package dependency concerns, which made packaging and transfer harder than the procedure itself.
- Some environments were constrained enough that even SSH-based operation was not a safe assumption.
- A large share of the work involved Kubernetes installation and lifecycle tasks, where image mirrors, package repositories, and local serving patterns often had to exist before the main procedure could succeed.
- Supporting multiple environments pushed the workflow toward Docker or Podman almost by default, even when the real need was simply to move prepared content and execute a known local procedure.

None of these problems are unique. The point is that together they made the common online-first tool shape feel heavier than the environments being operated.

## How it fits together

The operating model has two parts: preparation happens in a connected environment where packages, images, and files can be fetched; execution happens locally on the target machine inside the air gap. The `prepare` workflow handles the first part, the `apply` workflow handles the second, and `deck bundle build` packages everything needed to cross the boundary.

This separation is intentional. The operator on the far side of the air gap should be able to run `deck apply` without resolving external dependencies, contacting a control plane, or interpreting a long shell script.

## The practical strategy

`deck` takes a few practical positions to reduce those constraints.

- Use a single-binary execution model wherever possible.
- Keep the default execution path local rather than SSH-driven.
- Treat the bundle as the unit of offline handoff.
- Include optional site-local helper capabilities, such as static serving and pull-oriented registry support, only where they reduce friction inside the air gap.
- Keep the workflow model readable to operators who already think in YAML, stages, and repeatable procedures.

## Design principles

- **Local-first**: the default path is local execution on the machine that needs the change, with no long-lived controller required.
- **Bundle-first**: workflow, artifacts, and the `deck` binary travel together so the offline handoff is explicit and complete.
- **Readable**: typed steps and named phases keep the procedure scannable as it grows.
- **Pragmatic**: `Command` is available for the edges that are not modeled yet, but it should not be the dominant authoring style.

## Core values

- **Not a full IaC platform**: `deck` is not trying to become a universal provisioning system. It is a structured workflow tool that gives shell-like operational tasks a clearer and safer shape.
- **Small and explicit**: the tool should stay easy to understand, package, and operate. New functionality should follow the same direction rather than turning `deck` into a large general controller.
- **Single-binary by default**: the normal site-side experience should not require a larger runtime beyond the `deck` binary and the bundle contents. The connected-side prepare path is the main exception because it is the place where external fetches happen.
- **Friendly to modern infrastructure operators**: the workflow model and CLI should feel comfortable to engineers already used to YAML, artifacts, registries, package repos, and staged rollout procedures.

This is also why `deck` tries to keep both typed steps and CLI commands intentionally simple. If the same operational task can be expressed in too many equivalent ways, review becomes harder, examples become noisier, and operators spend more time choosing between tool shapes than executing the work itself.

## What deck is trying to improve

At a practical level, `deck` tries to reduce a few recurring costs:

- the review cost of long shell procedures
- the packaging cost of dependency-heavy automation runtimes
- the operational cost of assuming remote transport that may not exist
- the ambiguity that appears when artifact gathering and host mutation are mixed together

The tool does not remove all complexity. It tries to move the complexity into a more explicit shape.

## Who it's for

`deck` is for operators and engineers who already think in YAML, stages, and repeatable procedures, but need a smaller tool for disconnected or constrained work — somewhere between a shell script and a full automation platform.
