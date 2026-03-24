# Versioning

`deck` uses semantic versioning for released binaries.

## Source of truth

- Release versions are Git tags in the form `vX.Y.Z`.
- `main` is the branch for the next release in development.
- Maintenance branches are created only when needed.

The first planned release is `v0.1.0`.

## Build metadata

`deck version` reports build metadata embedded at build time through Go linker flags.

Current fields:

- `version`: release semver such as `v0.1.0`, or `dev` for local non-release builds
- `commit`: short Git commit SHA
- `date`: UTC build timestamp
- `variant`: `core` or `ai`
- `dirty`: whether the working tree had local modifications at build time

This keeps local support output useful even before formal releases begin.

## Build paths

Use `Makefile` targets as the canonical build entrypoints:

```bash
make build
make build-ai
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
```

```json
{
  "name": "deck",
  "version": "dev",
  "commit": "abc1234",
  "date": "2026-03-17T10:00:00Z",
  "variant": "core",
  "dirty": true
}
```

Release builds should report the tagged semver instead of `dev`.

## Release notes and automation

Tagged releases are published through GoReleaser.

- Tag format: `vX.Y.Z`
- Release target: GitHub Releases in `Airgap-Castaways/deck`
- Published artifacts: release tarballs, `deb`, `rpm`, checksums, and a Homebrew tap formula update

See [Release Process](release-process.md) for the operational checklist, required secrets, and validation flow.
