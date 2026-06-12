# Release Notes

This directory holds per-release highlight files that are injected into GitHub Release notes.

## How it works

For each release a file named `docs/releases/v<MAJOR>.<MINOR>.<PATCH>.md` (for example, `docs/releases/v0.3.0.md`) may be added to the repository. The file contains 3–6 Korean "주요 변경 사항" (key changes) bullets written from a user perspective.

When the release workflow runs:

1. If `docs/releases/<tag>.md` exists, its contents are loaded into the `DECK_RELEASE_HIGHLIGHTS` environment variable.
2. GoReleaser reads that variable and renders it above the auto-generated English feat/fix changelog in the GitHub Release header.
3. The resulting release notes contain the Korean highlights block, a horizontal rule, and then the default release artifact text followed by the generated changelog.

If no file exists for the tag, the header falls back to the default text ("Release artifacts for `deck` are attached below...") and the release proceeds without a highlights block. Providing a highlights file is optional.

## File format

Each file must start with a `## 주요 변경 사항` heading followed by bullet items. Use `docs/releases/TEMPLATE.md` as a starting point.

## Offline changelog

Highlight files are committed alongside the code, so they travel with the repository and serve as a human-readable offline changelog for air-gapped sites that cannot access GitHub Release pages.
