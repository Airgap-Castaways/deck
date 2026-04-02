# Versioning

`deck` uses semantic versioning for released binaries.

## Source of truth

- Release versions are Git tags in the form `vX.Y.Z`.
- `main` is the branch for the next release in development.
- Maintenance branches are created only when needed.

The first planned release is `v0.1.0`.

## Pre-1.0 compatibility policy

Until `v1.0.0`, `deck` should be treated as a fast-moving pre-stable tool.

- Minor and patch releases in the v0.x series may still change workflow schemas, CLI contracts, bundle structure, audit records, and other published contracts.
- The project does not promise broad legacy compatibility across pre-1.0 releases.
- When a simpler canonical shape is identified, prefer converging on it instead of carrying long-lived compatibility shims for abandoned pre-1.0 designs.
- Compatibility code that does exist in the `v0.x.y` line should stay narrow and intentional, typically only where it protects a real on-disk migration path or a clearly documented upgrade flow.

In practice, users and downstream automation should pin exact `v0.x.y` releases and expect that upgrading between pre-1.0 versions may require workflow, script, or integration updates.

## Build metadata

`deck version` reports build metadata embedded at build time through Go linker flags.

Current fields:

- `version`: release semver such as `v0.1.0`, or `dev` for local non-release builds
- `commit`: short Git commit SHA
- `date`: UTC build timestamp
- `dirty`: whether the working tree had local modifications at build time

This keeps local support output useful even before formal releases begin.

## Build paths

Use `Makefile` targets as the canonical build entrypoints:

```bash
make build
```

By default, local builds report `dev` until a real release process starts supplying a semver.

## CLI contract

The CLI exposes:

```bash
deck version
deck version -o json
```

Examples:

```text
deck dev
repo https://github.com/Airgap-Castaways/deck
```

```json
{
  "name": "deck",
  "version": "dev",
  "commit": "abc1234",
  "date": "2026-03-17T10:00:00Z",
  "dirty": true,
  "repository": "https://github.com/Airgap-Castaways/deck"
}
```

Release builds should report the tagged semver instead of `dev`.

The version command reports build identity, but it does not imply pre-v1.0.0 compatibility guarantees for CLI contracts, workflow schemas, bundle structure, or other published contracts.

## Release notes and automation

Tagged releases are published through GoReleaser.

- Tag format: `vX.Y.Z`
- Release target: GitHub Releases in `Airgap-Castaways/deck`
- Published artifacts: release tarballs, `deb`, `rpm`, checksums, and a Homebrew tap formula update

See [Release Process](release-process.md) for the operational checklist, required secrets, and validation flow.
