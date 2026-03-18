# deck

<img align="right" src="assets/logo.png" width="120" alt="logo">

[Korean README](./README.ko.md) | [Documentation](./docs/README.md)

**A tool that converts air-gapped network operation procedures running on Bash into verifiable bundle-based workflows.**

<br clear="right" />

## What is deck?

Operational procedures for disconnected sites—such as Kubernetes bootstraps, package installations, and host configuration—often start as shell scripts. Over time, these scripts grow until they become too large and complex to review confidently. 

`deck` provides a cleaner, structured alternative. It replaces fragile Bash scripts with typed steps, validates your workflows before execution, and packs everything needed into a self-contained bundle that can be securely transported and run locally on the target machine.

## How to use it? (Quick Start)

Create a workflow, validate it, bundle it, and run it on your target machine.

```bash
# Initialize a new demo project
deck init --out ./demo

cd ./demo

# Validate the generated workflows
deck lint

# Prepare artifacts defined in the workflows
deck prepare

# Build a self-contained bundle
deck bundle build --out ./bundle.tar

# Run the workflow locally
deck apply
```

For a detailed walkthrough, start with the [Quick Start Guide](docs/getting-started/quick-start.md).

## Core Features

- **Typed Workflow Steps:** Replace arbitrary shell commands with explicit, typed steps for common host changes, making your intent visible and reviewable.
- **Pre-flight Validation:** Catch errors before you step into the datacenter. `deck` lints and validates workflow structure before transport or execution.
- **Self-contained Bundles:** Build a single archive containing your workflows, required artifacts, and the `deck` binary itself. No missing dependencies on site.
- **Air-gap Native:** Designed specifically for environments with no SSH-driven orchestration, no internet access, and a local human in the loop.

## Installation

Requirements:

- Go 1.25+ (Any OS for build and prepare)
- Linux target environment (RHEL, Ubuntu) for the `apply` step

```bash
# Install the binary
go install github.com/taedi90/deck/cmd/deck@latest

# Verify installation
deck version
```

### Shell Completion

To enable shell completion in your current session, run:

```bash
source <(deck completion bash) # for bash
source <(deck completion zsh)  # for zsh
deck completion fish | source  # for fish
```

To make it persistent, add the above command to your `~/.bashrc` or `~/.zshrc`.

## Documentation

- [Getting Started](docs/getting-started/README.md)
- [Core Concepts](docs/core-concepts/README.md)
- [User Guide](docs/user-guide/README.md)
- [Reference](docs/reference/README.md)
- [Contributing](docs/contributing/README.md)

## License

Apache-2.0. See `LICENSE`.
