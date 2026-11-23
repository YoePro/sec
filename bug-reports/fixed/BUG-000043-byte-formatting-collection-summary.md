# BUG-000043 — Byte radix formatting formats its digit buffer as a collection

## Status

Fixed in the core source, 2026-11-23. Executable string conformance remains
behind the existing P23/static-test-harness boundary.

## Report

All six buffer-producing branches in `sec/core/byte.sec` called
`bytes.ToString()` to create the formatted number. Under MD-091, a byte
collection's ToString formats the collection; it never decodes its contents as
text. Thus the generated ASCII digit buffer was passed to the wrong operation.

## Reproduction

`TestByteFormattingMaterializesDigits` parses and analyzes the actual trusted
core source, then checks its resolved materialization identities. Before the
fix it found zero explicit byte-to-text operations; after the fix it finds six:
three decimal widths, hexadecimal, octal and binary. This is a semantic-routing
regression; no execution of the old or new formatted output is claimed.

## Expected behavior

The byte's numeric value is formatted as decimal or fixed-width uppercase
hexadecimal, octal or binary ASCII text, then materialized as an independent
string through the permitted allocation context.

## Scope

- Area: core byte radix formatting.
- Rulebooks: `rules/library/core-library.md` §8.4;
  `rules/compiler/compiler_known_members.md` §69.1 (MD-091);
  `rules/corrections/applied/md109-core-string-materialization-correction-20261121.md`
  §§1.2–1.7,2.1–2.6.
- Governance: `compiler.compiler-known-surfaces`.
- Phase: core implementation and resolved frontend calls.

## Resolution

The six branches call `string.FromByteArray(bytes)` with borrowed input and
return its existing `Result[string, StringError]`. Collection ToString semantics
are unchanged. The required byte-specific enum, overload, width, padding,
uppercase and materialization rules are synchronized in §8.4 of the core
rulebook. Actual core source Sema, public CLI and LSP hover regressions pass;
`sec test --list sec/core` compiles/discovers the 32 existing tests. Runtime
output/allocation-failure acceptance remains with the existing lowering and
static test harness owners; MD-109 is not declared complete.
