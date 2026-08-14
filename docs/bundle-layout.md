# Bundle layout

`deck prepare` writes a self-contained workspace under the current directory. `deck bundle build` archives that workspace into a single tarball you carry into the site.

The bundle is the unit of offline handoff. Everything the workflow needs to run on the target machine should be inside it, no implicit fetch at execution time, no reach-back to external services.

## Canonical bundle inputs

`deck bundle build` archives the following workspace paths:

- `deck`: a launcher script written to the workspace root during `prepare`
- `workflows/`: scenario, component, and variable files used at the site
- `outputs/bin/`: platform-specific runtime binaries selected during `prepare`
- `outputs/packages/`: OS or Kubernetes packages fetched during `prepare`
- `outputs/images/`: container image archives fetched during `prepare`
- `outputs/files/`: supporting files copied or downloaded during `prepare`
- `.deck/manifest.json`: integrity manifest used by `bundle verify`

`bundle build` does not archive arbitrary extra root-level paths by default. If a workflow needs additional content at the site, place it under `workflows/` or produce it under `outputs/` during `prepare` so it travels with the canonical bundle.

## Example bundle contents

A typical Kubernetes control-plane bundle might contain:

```
deck
.deck/manifest.json
workflows/scenarios/apply.yaml
workflows/prepare.yaml
workflows/vars.yaml
outputs/bin/linux/amd64/deck
outputs/packages/kubernetes-1.29.tar.gz
outputs/images/pause-3.9.tar
outputs/images/coredns-1.11.tar
outputs/files/kubeadm.conf
```

The operator unpacks this on the target node, then runs `./deck apply`. The launcher selects the matching runtime binary from `outputs/bin/<os>/<arch>/deck` when that platform is included in the bundle.

## Prepare Reuse Integrity

`deck prepare` may reuse previously fetched artifacts between runs, but the reuse rules differ by artifact family:

- `DownloadFile` re-checks local SHA256 state and, for URL sources, can also consult remote validators such as `ETag` and `Last-Modified`.
- `DownloadPackage` now records SHA256 metadata for published package outputs and exported package cache payloads, then revalidates those checksums before reuse.
- `DownloadImage` now records SHA256 metadata for saved image archives and revalidates those checksums before reuse.

`DownloadImage` reuse requires the saved tar files and the `outputs/images/.deck-cache-images.json` metadata written by a previous successful `prepare` run. The metadata can track multiple image/platform sets in the same output directory. A matching tar file without that metadata is treated as a cache miss. `deck prepare --refresh` bypasses image reuse and downloads the archives again.

Remaining drift gap:

- package reuse still does not detect upstream repository drift on its own; closing that gap likely requires repository snapshot metadata such as repodata/release fingerprints or explicit mirror version contracts.
- image reuse now preserves fetched source digests in metadata, but mutable tag drift is not yet probed on reuse; a follow-up can compare saved digests against current registry manifests when remote access is allowed.

## Apply-time manifest verification {#apply-time-manifest-verification}

Whenever `deck apply` resolves a bundle root, it verifies the bundle manifest before executing any workflow phase. A verification failure aborts the run immediately, no phase begins.

### Invocations that trigger verification

Verification runs automatically in three cases:

- **Plain `deck apply` in a workspace**: when no explicit path is given, deck uses the current directory as the bundle root if it contains a `workflows/` tree; verification runs against it.
- **`deck apply --root <dir>`**: the explicit root is used as the bundle root; verification runs.
- **`deck apply <bundle-path>`**: a positional directory or `.tar` archive is used as the bundle root; verification runs. When a `.tar` archive is given, deck extracts it to a keyed cache directory first, then verifies the extracted contents.

Verification is skipped only when no bundle root is resolved, for example, when `--workflow <path>` is supplied without a positional bundle, or when `--scenario <name> --source server` is supplied without a positional bundle.

To verify a bundle explicitly before transfer or apply, run:

```bash
deck bundle verify --file ./bundle.tar
```

### What is verified

Verification reads `.deck/manifest.json` and checks every entry against the corresponding artifact in `outputs/{files,packages,images,bin}` (the `outputs/` prefix is optional, legacy bundles using bare `files/`, `packages/`, `images/`, `bin/` paths are also tracked). For each entry it confirms:

- the artifact exists on disk (or inside the tar archive),
- the SHA-256 digest matches the recorded value,
- the file size matches when a non-zero size is recorded.

After per-entry checks, the function also cross-checks that every package-repository index file (`Release`, `Packages.gz`, `repomd.xml`) and every image `.tar` present in the bundle is covered by a manifest entry.

See [workspace-layout.md](workspace-layout.md) for the full list of paths tracked by the manifest.

### Failure modes

| Error code | Condition |
|---|---|
| `E_MANIFEST_MISSING` | `.deck/manifest.json` is absent from the bundle directory or tar archive |
| `E_MANIFEST_EMPTY` | The manifest file exists but its `entries` array is empty |
| `E_BUNDLE_INTEGRITY` | An artifact is missing, or its size or SHA-256 digest does not match the manifest, or a manifest entry path is structurally invalid, or a required offline artifact is present in the bundle but absent from the manifest |

See [diagnostics/error-codes.md](diagnostics/error-codes.md) for the full error-code catalog and [troubleshooting.md](troubleshooting.md) for resolution steps.

## Core rule

If the site needs it to run the workflow, place it in the canonical bundle inputs rather than assume it already exists on the target machine.
