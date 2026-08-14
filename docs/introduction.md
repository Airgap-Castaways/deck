---
slug: /
---

# Introduction

## What is deck

`deck` is a structured workflow tool for air-gapped, disconnected, and operationally constrained environments. It replaces the growing shell scripts that accumulate around offline Kubernetes bootstraps, package installations, and host configuration with typed, validated steps packed into a self-contained bundle.

The tool is designed for operators who cannot assume internet access or a reachable control plane when they actually run a procedure. Regulated data centers, industrial edge sites, government networks, and any environment where "SSH into the target and pull from the internet" is not an option, these are the environments deck was built for.

A single static `deck` binary handles everything: authoring, validation, artifact gathering, bundling, and local execution on the target. The target machine needs no pre-installed runtime and no network reach-back. Everything it needs travels with the bundle.

## The core idea

Deck splits an offline operation into three distinct stages:

- **Prepare**: while you still have internet access, `deck prepare` downloads every artifact the workflow needs: container images, OS packages, binary files. These land in a local `outputs/` tree.
- **Bundle**: `deck bundle build` packages the workflow definitions, the prepared artifacts, and the `deck` binary itself into a single `bundle.tar`. This is the archive you transfer across the air gap on a USB drive, a bastion hop, or whatever your site allows.
- **Apply**: on the disconnected target, `deck apply` executes the workflow locally. No SSH, no controller, no external reach-back. The machine runs the steps directly from the bundle it received.

The prepare → bundle → apply lifecycle is the central idea. Each stage is explicit and can be reviewed before you carry the bundle into the site.

## Why not just shell scripts

Shell scripts are easy to start and can run almost anywhere. They become a problem as procedures grow:

- Typed steps make intent visible. An `InstallPackage` step tells the reviewer what will happen before they read the spec; a block of `rpm` or `apt` commands does not.
- Pre-flight validation catches mistakes early. `deck lint` checks the workflow structure and every step schema before you leave the connected environment, because discovering a typo or a missing field inside the air gap is expensive.
- The bundle is self-contained and verifiable. Workflow definitions, all required artifacts, and the `deck` binary ship together in one archive, so there are no surprise missing dependencies when you unpack on site.
- Apply is resumable. If execution stops partway through, `deck apply` resumes from the last completed phase rather than starting from scratch. Shell scripts offer no equivalent without custom bookkeeping.

## What's in the box

Deck ships as a single binary with a small, focused surface area:

- **Typed step kinds**: covering file writes, package installs, service management, sysctl, kernel modules, container image loading, kubeadm lifecycle, and more. Use [`deck ask`](ask.md) as an authoring assistant when you are drafting or reviewing workflows.
- **CEL `when` conditions**: skip or guard any step with a typed expression evaluated at runtime.
- **Phases and parallelism**: named phases make a procedure readable at a glance; `parallelGroup` runs independent steps concurrently within a phase.
- **Variables and templating**: a shared `vars.yaml` plus per-invocation `--var` overrides keep site-specific values out of the step definitions.
- **Resumable apply state**: phase-level state tracks what has already run so a stopped apply can continue safely.
- **Built-in content server**: `deck server up` exposes a prepared bundle root over HTTP inside the air gap, useful when multiple nodes need to pull from a shared local source.
- **AI authoring assistant**: `deck ask` is an experimental, opt-in helper that drafts, explains, and reviews workflows using an LLM-backed assistant. It is part of the standard `deck` binary and degrades gracefully when model access is unavailable.

## Where to go next

Follow this path to get productive quickly:

1. [Installation](installation.md): install the binary, verify it, and set up shell completion.
2. [Quick Start](quick-start.md): create a workspace, lint it, build a bundle, and apply it locally in a few commands.
3. [Core concepts](core-concepts/why-deck.md): understand why the prepare/bundle/apply split matters and what principles drive the design.
4. [Guides](guides/authoring-workflows.md): task-focused how-tos for authoring workflows, variables and templating, conditions, phases and parallelism, capturing output, and running the server.
5. [Workflow model](workflow-model.md): the reference for the YAML structure, variables, phases, and step envelope fields.
6. [Step kinds](step-kinds.md): browse the full catalog of typed step kinds organized by phase and task.
