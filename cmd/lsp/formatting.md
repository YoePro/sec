# Formatter presentation context

`textDocument/formatting`, `textDocument/rangeFormatting` and
`textDocument/onTypeFormatting` use the project's `[format]` settings through
`internal/formatconfig`, also used by `sec fmt`. Canonical style, indentation,
format version and alignment limits remain project-owned. Standard client
`tabSize`/`insertSpaces` preferences do not override those settings.

The request's `options` may additionally contain `secPresentationWidth`, an
observed viewport width in columns encoded as an unsigned integer. It is
presentation context, not a request to change project style:

- In project `width_mode = "fixed"`, the context is ignored; `line_width` wins.
- In `adaptive` mode, a positive observation is clamped to the project's
  inclusive `adaptive_min_width..adaptive_max_width` range.
- Missing or zero observations use `adaptive_fallback_width`, also clamped to
  that range. Clients without a reliable observation, including the bundled
  client today, omit the property. CLI file/stdin formatting uses the fallback.
- An invalid property type or negative value returns JSON-RPC `InvalidParams`
  without changing the source. Formatting never writes the project manifest.

The resolved preferred width is passed to the shared formatter. Its existing
parameter-list layout consumes it for single-line and mixed callable headers;
deliberately multiline lists, comments and multiline parameter values retain
their layout. Structural indentation is resolved first so nested headers use
canonical columns. Width is a preference rather than a source validity limit;
this change does not claim complete wrapping of every language construct.

Rules: `rules/tooling/formatter.md` §4(9–23), §10 and §16(1–2). Complete language
layout remains separate governance work.

Range formatting expands to complete parser/CST top-level declarations; a
nested member selects its containing impl or type so indentation and alignment
retain their context. Imports select their sortable group. The formatter uses
full-document context and returns one minimal edit inside the selected units.
Unselected bytes, including blank lines and line endings, remain unchanged.
A full-document range yields the same bytes as document formatting.

On-type formatting triggers after `}` or newline and selects the same complete
syntax unit. Literal/comment contents cannot act as triggers. Malformed or
incomplete source receives no speculative edit. Both local modes use the latest
snapshot, UTF-16 positions and formatting priority behind preceding document
synchronization. They perform no semantic analysis or source-file writes.

Rules: `rules/tooling/formatter.md` §§5,9,12,13,25,28,29;
`rules/tooling/lsp.md` Appendix A.8.
