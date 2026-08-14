# .deckignore

`.deckignore` controls which files are excluded when building a bundle and when serving content with `deck server up`. It lives at the root of the workspace (or bundle root) and uses gitignore-style pattern syntax.

## Location

Place `.deckignore` at the workspace root: the same directory that contains `workflows/`, `outputs/`, and `.deck/`. `deck init` creates a starter file there automatically.

## Syntax

Patterns are compiled to Go `regexp` by the `github.com/sabhiram/go-gitignore` library, which implements a subset of gitignore rules. Because translation goes through Go regexp (not a full gitignore engine), a few characters behave differently from standard gitignore. The supported features are:

| Feature | Supported | Notes |
|---------|-----------|-------|
| Blank lines | Yes | Ignored (separator) |
| `#` comments | Yes | Lines beginning with `#` are ignored |
| Trailing spaces | Yes | Stripped unless escaped with `\` |
| Negation `!` | Yes | `!pattern` un-ignores a previously matched path |
| Trailing `/` (directory-only) | Yes | `foo/` matches only directories named `foo` |
| No-slash pattern (unanchored) | Yes | `*.log` matches anywhere in the tree |
| Leading `/` (anchored) | Yes | `/foo` matches only at the root |
| Single `*` wildcard | Yes | Does not cross directory boundaries |
| `**` double-star | Yes | Matches across directories (`a/**/b` matches `a/b`, `a/x/b`, etc.) |
| `?` wildcard | No | `?` is treated as a **literal** `?` character (the library escapes it to `\?` in the regexp); it does **not** match any single character |
| Character classes `[…]` | Partial | Bracket classes **do** work as Go regexp character classes (e.g. `[abc]`, `[0-9]`). However, gitignore's negated form `[!abc]` does **not** negate, Go regexp requires `[^abc]`; a `[!…]` pattern will not behave as expected |
| Backslash escaping `\#`, `\!` | Yes | Treats `#` or `!` as a literal character |

> **Note:** `.deckignore` uses `go-gitignore`, which compiles patterns to Go `regexp` rather than implementing the full gitignore spec. Two notable differences: `?` is treated as a literal character (not a single-character wildcard), and character class negation uses Go regexp syntax: `[!abc]` does **not** negate (use `[^abc]` if you need negation, but that is a raw regexp construct, not standard gitignore syntax).

### Rules applied by `Matches`

`deck` calls `Matcher.Matches(rel, isDir)` with the forward-slash path relative to the workspace root. Before pattern evaluation:

- Leading `./` is stripped.
- Paths are converted to forward slashes on all platforms.
- The root path itself (`.` or empty string) is never matched.

For directories, `deck` tests both `rel/` (trailing-slash form) and `rel`, so a trailing-slash pattern such as `outputs/` correctly prunes directory traversal.

## Behavior when the file is absent

If `.deckignore` does not exist, `Load` returns an empty matcher and `Matches` always returns `false`: nothing is excluded. This is a no-op, not an error.

## Where it applies

`.deckignore` is enforced in four places:

### Bundle building (`deck bundle build`)
`internal/bundle/collect.go` calls `deckignore.Load` once from the bundle root before walking the archive tree. Matched files are skipped; matched directories cause the entire subtree to be pruned (`filepath.SkipDir`).

### Static file and workflow serving (`deck server up`)
`internal/server/http_static.go` loads `.deckignore` inside `resolveCategoryPath` and `buildWorkflowIndex`. Requests for a path that matches an ignore rule receive HTTP 404.

### Browse UI (`deck server up`)
`internal/server/http_browse.go` (`listBrowseEntries`) loads `.deckignore` and omits matched entries from directory listings shown in the browser.

### OCI registry catalog (`deck server up`)
`internal/server/http_registry.go` (`scanRegistryCatalog`) loads `.deckignore` before scanning `outputs/images/` and `images/` for `.tar` files. Any `.tar` whose path relative to the bundle root matches is excluded from the registry catalog; it will not appear in `/v2/_catalog` and cannot be pulled.

## Default content

`deck init` writes the following starter `.deckignore`:

```
.git/
.gitignore
.deckignore
/*.tar
```

This hides version-control internals, the ignore files themselves, and any loose `.tar` archives at the workspace root.

## Example

```gitignore
# Exclude version-control internals
.git/

# Exclude the ignore files themselves from the bundle
.gitignore
.deckignore

# Exclude all loose tar archives directly under the workspace root
/*.tar

# Exclude a specific large image tarball from the server registry
outputs/images/dev-tools.tar

# Exclude an entire scenario subdirectory
workflows/scenarios/internal/

# Re-include one file inside a broader excluded directory
!outputs/files/README.txt
```

## Related

- [Workspace Layout](workspace-layout.md): workspace directory structure
- [Bundle Layout](bundle-layout.md): what goes into a bundle archive
- [Server Registry](server/registry.md): OCI registry served by `deck server up`
