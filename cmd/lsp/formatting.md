# Formatter presentation context

`textDocument/formatting` uses the project's `[format]` settings through
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
layout and range/on-type formatting remain separate governance work.
