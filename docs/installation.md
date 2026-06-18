# Installation

## Where deck runs

`deck` ships as a single static binary with no runtime dependencies.

| Stage | Platforms |
|---|---|
| Author, lint, prepare, bundle | macOS, Linux, Windows |
| Apply (target) | Linux — RHEL / Rocky / CentOS or Debian / Ubuntu |

You do **not** need Go installed on the target machine. The `deck` binary (and everything else the workflow needs) travels inside the bundle. Only the machine where you author and prepare workflows needs a `deck` binary in PATH.

Release artifacts are built for:

- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`

A Windows binary is not yet published in releases; Windows users can build from source (see below).

## Homebrew (recommended)

The Airgap Castaways tap publishes the latest release:

```bash
brew install Airgap-Castaways/tap/deck
```

This taps `Airgap-Castaways/homebrew-tap` and installs the `deck` formula. Upgrades follow the normal `brew upgrade` path.

## GitHub Releases (binary tarball)

Download the archive for your OS and architecture from the [releases page](https://github.com/Airgap-Castaways/deck/releases).

Archive names follow the pattern `deck_<version>_<os>_<arch>.tar.gz`, for example:

- `deck_1.2.3_linux_amd64.tar.gz`
- `deck_1.2.3_darwin_arm64.tar.gz`

Extract and place the binary on your PATH:

```bash
# Example for Linux amd64 — adjust the filename for your version and arch
tar -xzf deck_<version>_linux_amd64.tar.gz
sudo mv deck /usr/local/bin/deck
```

**Verify the checksum** before running the binary. Each release publishes a `checksums.txt` file alongside the archives:

```bash
sha256sum --check --ignore-missing checksums.txt
```

### Debian and RHEL packages

The release page also publishes `.deb` and `.rpm` packages built from the same binary. These install `deck` to `/usr/bin/deck`:

```bash
# Debian / Ubuntu
sudo dpkg -i deck_<version>_linux_amd64.deb

# RHEL / Rocky / CentOS
sudo rpm -i deck_<version>_linux_amd64.rpm
```

## Build from source

Requires Go 1.26 or later.

```bash
go install github.com/Airgap-Castaways/deck/cmd/deck@latest
```

Binaries installed this way do not carry version-stamp metadata (the `deck version` output will show empty commit and date fields). To build with version info, clone the repository and use the Makefile:

```bash
git clone https://github.com/Airgap-Castaways/deck.git
cd deck
make build
```

The `make build` output lands in the repository root as `deck`.

## Verify

Confirm the binary is installed and reachable:

```bash
deck version
```

You should see version, commit, and build date. If the binary was installed via `go install` without ldflags, the version fields will be empty — that is expected for unversioned source builds.

## Shell completion

`deck` generates completion scripts for bash, zsh, fish, and PowerShell via the `deck completion <shell>` command.

### Immediate (current session only)

```bash
source <(deck completion bash)   # bash
source <(deck completion zsh)    # zsh
deck completion fish | source    # fish
```

For PowerShell:

```powershell
deck completion powershell | Out-String | Invoke-Expression
```

### Persistent setup

Add the sourcing command to your shell's initialization file so completion loads automatically in every new session.

**bash** — add to `~/.bashrc`:

```bash
source <(deck completion bash)
```

**zsh** — add to `~/.zshrc`:

```bash
source <(deck completion zsh)
```

**fish** — write to the fish completions directory:

```bash
deck completion fish > ~/.config/fish/completions/deck.fish
```

**PowerShell** — add to your `$PROFILE`:

```powershell
deck completion powershell | Out-String | Invoke-Expression
```

## Next step

Follow the [Quick Start](quick-start.md) to create your first workspace, lint it, build a bundle, and apply it locally.
