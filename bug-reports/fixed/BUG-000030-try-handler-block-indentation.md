# BUG-000030 — Try-handler lists drift after inline thread configuration

## Status

Fixed on 2026-11-19.

## Report

The formatter does not classify a local `try` handler list as a block. Its
inner arm body expands, but the handler list remains attached to that body on
one line. After multiline inline thread configuration, the next format pass
adds indentation to subsequent statements and top-level declarations.

## Reproduction

```sh
go test ./internal/formatter -run '^TestFormatInlineThreadConfiguration$' -count=1
```

Input: `testdata/sema/thread_config_valid.sec`. The original test fails its
second-pass idempotence assertion. The same failure occurs with the parser and
AST restored to their pre-initializer-signature versions through a temporary
Go overlay. No new language decision is needed.

## Expected and actual behavior

Handler lists use ordinary block and arm formatting. The configuration's
closing brace remains beside the child callable; handler arms belong on
subsequent lines, and `join worker` and following functions retain their
structural indentation. Repeated formatting is byte-identical.

Previously only inner arm blocks were expanded; the containing handler list
had no parser-owned formatter role, and delimiter counting accumulated an
incorrect nesting level during subsequent passes.

## Scope

- Area: shared formatter and parser-owned CST roles.
- Rules: `rules/tooling/formatter.md` §§ 5–6, 8, 19(2–6), 29;
  `rules/concurrency/threads.md` § 11.
- Governance integration: `tooling.formatter-v2`.
- Affected phase: formatting, including CLI and LSP consumers.

## Resolution

Parser-owned `HandlersOpen` metadata now supplies the exact handler-list brace
for expression and assignment forms. `internal/cst` gives its matching pair
ordinary executable-block roles, so the existing shared block formatter expands
the containing handler list and nested arm blocks consistently. No line-text
special case for thread configuration was added. Bodyless try, operand value
literals, inline configuration and field-pattern braces receive no handler-list
role.

Validation:

- Complete CST and formatter race suites passed (1.042s / 15.135s).
- Parser/Sema/Semantic IR try-focused suites passed (0.007s / 0.263s / 0.273s).
- Focused CLI/LSP formatting suites passed (0.244s / 0.117s), including golden
  output, project configuration, check mode and repeated requests.
- `go vet` passed for AST, parser, CST, formatter, compiler and LSP packages.
- `testdata/formatter/try_handler_blocks.sec` and its explicit expected output
  cover thread configuration, expression and statement try, fallible assignment,
  bodyless propagation, field patterns and following statements/declarations.
- The golden suite verifies both vertical styles, indentation widths 2/4/6,
  preferred widths 80/120/200, parseability, unchanged tokens and idempotence.

The control-condition correction test retains its full parenthesized try
condition while accepting the canonical multiline handler block. This is a
published-contract bug fix; no format version or language decision changed.
Broad formatter/CST migration remains open under `tooling.formatter-v2`.
