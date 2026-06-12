# Release Process

`deck` releases are published with GoReleaser from signed or annotated Git tags in the form `vX.Y.Z`.

## What a release publishes

- GitHub Release assets for `deck`
- Cross-platform tarballs (`.tar.gz`) for macOS and Linux — each archive includes the full `docs/` tree and `README.ko.md` for offline use
- Linux packages (`.deb` and `.rpm`) — each package installs docs under `/usr/share/doc/deck/docs/` and `README.ko.md` alongside the binary
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
make release-check
make release-snapshot
```

`make release-snapshot` verifies the generated tarballs, packages, checksums, and Homebrew formula output without publishing a release.

## Publishing a release

1. Ensure `main` is ready and CI is green.
2. Confirm that PR titles and squash-commit subjects follow conventional-commit prefixes (`feat:`, `fix:`, `refactor:`, `chore:`, `docs:`, `test:`) so GoReleaser groups the auto-generated changelog correctly.
3. **Optionally** add a Korean highlights file before tagging:

   ```bash
   cp docs/releases/TEMPLATE.md docs/releases/v0.2.0.md
   # edit the file, then:
   git add docs/releases/v0.2.0.md
   git commit -m "docs(releases): add highlights for v0.2.0"
   git push origin main
   ```

4. Create and push a release tag:

   ```bash
   git tag v0.2.0
   git push origin v0.2.0
   ```

5. Wait for `.github/workflows/release.yml` to complete (smoke-test job, then release/publish job).
6. Verify the GitHub Release assets, the generated changelog, and the updated Homebrew formula.

## Release notes

### Auto-generated English changelog

GoReleaser generates the changelog automatically from commits between the previous and new tag using `changelog.use: github`. Commits are grouped into:

| Group | Matched prefixes |
|-------|-----------------|
| Features | `feat:` / `feat(…):` |
| Bug Fixes | `fix:` / `fix(…):` |
| Maintenance | `refactor:`, `chore:`, `docs:`, `test:` (with optional scope) |
| Other | everything else |

Sync-merge commits (`Merge pull request`, `Merge branch`, `Merge remote-tracking branch`) are filtered out automatically.

`Other` is a temporary catch-all for commit subjects that do not follow conventional-commit prefixes. Prefer `feat:`, `fix:`, `refactor:`, `chore:`, `docs:`, and `test:` in PR titles and squash commit subjects so the changelog stays well grouped.

### Optional release highlights

For releases where a human-written summary adds value, a maintainer can create `docs/releases/v<MAJOR>.<MINOR>.<PATCH>.md` before pushing the tag (copy `docs/releases/TEMPLATE.md`). The file must start with a `## Highlights` heading followed by 3–6 user-facing bullet points.

When `.github/workflows/release.yml` runs, it checks for that file and — if present — loads it into the `DECK_RELEASE_HIGHLIGHTS` environment variable. GoReleaser's `release.header` renders the highlights above the auto-generated English changelog in the GitHub Release. If the file is absent the release proceeds with only the default header text.

Providing a highlights file is optional. See [docs/releases/README.md](../releases/README.md) for the format and further details.

### What maintainers do NOT do

- Hand-write the full GitHub Release changelog (GoReleaser generates it).
- Pass `--release-notes` to GoReleaser; the header injection is handled entirely by the workflow via `DECK_RELEASE_HIGHLIGHTS`.

## Installation targets

- GitHub Releases page: direct asset downloads
- Homebrew tap:

```bash
brew tap Airgap-Castaways/tap
brew install deck
```
