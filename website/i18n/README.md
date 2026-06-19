# Localization (i18n)

Korean docs are generated from the English source under `docs/` and live in
`website/i18n/ko/docusaurus-plugin-content-docs/current/`.

## Translate (maintainer, local)

    make docs-translate                       # core narrative docs
    make docs-translate ARGS="docs/cli.md"    # specific files
    make docs-translate ARGS="--force docs/quick-start.md"

Engine: headless `claude -p` (Claude Code) by default. Override with
`DECK_TRANSLATE_CMD` — a shell command that reads English Markdown on stdin and
writes the translation to stdout — for testing or a different provider. Review
the generated Korean before committing. Translation follows
`website/i18n/TERMINOLOGY.md`.

## Drift check (local + CI)

    make docs-check

Each translated file records `source` and `source_hash` (the git blob SHA of the
English source at translation time) in its front matter. `make docs-check` — and
`go test ./...` in CI — fails when a source file changed after its translation.
Re-run `make docs-translate ARGS="--force <path>"` and review.

Untranslated or stale pages fall back to the English source automatically
(`defaultLocale: en`).
