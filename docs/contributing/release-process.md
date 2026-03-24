# Release Process

`deck` releases are published with GoReleaser from signed or annotated Git tags in the form `vX.Y.Z`.

## What a release publishes

- GitHub Release assets for `deck`
- Cross-platform tarballs for macOS and Linux
- Linux packages in `deb` and `rpm` formats
- `checksums.txt`
- A Homebrew tap formula in `Airgap-Castaways/homebrew-tap`

The Homebrew distribution is a custom tap. It is not intended for `homebrew-core`.

## Repository files

- `.goreleaser.yaml`: release definition
- `.github/workflows/release.yml`: tag-triggered release workflow
- `Makefile`: local `release-check` and `release-snapshot` helpers

## Required secrets

The release workflow uses:

- `GITHUB_TOKEN`: default GitHub Actions token for creating the release and uploading assets in `Airgap-Castaways/deck`
- `HOMEBREW_TAP_GITHUB_TOKEN`: token with contents write access to `Airgap-Castaways/homebrew-tap`

## Local validation

Run these commands before cutting a tag:

```bash
make test
make lint
make test-ai
make release-check
make release-snapshot
```

`make release-snapshot` verifies the generated tarballs, packages, checksums, and Homebrew formula output without publishing a release.

## Publishing a release

1. Ensure `main` is ready and CI is green.
2. Create and push a release tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

3. Wait for `.github/workflows/release.yml` to complete.
4. Verify the GitHub Release assets and the updated Homebrew formula.

## Release notes

Default tagged releases use the fixed GoReleaser header plus the auto-generated changelog.

Use custom release notes only for larger releases such as:

- first stable releases
- breaking changes or migrations
- significant packaging or distribution changes

Example:

```bash
goreleaser release --clean --release-notes docs/release-notes/v0.1.0.md
```

## Installation targets

- GitHub Releases page: direct asset downloads
- Homebrew tap:

```bash
brew tap Airgap-Castaways/tap
brew install deck
```
