# BUG-000046 — Conformance explains the wrong overload

## Status

Fixed, 2026-11-23.

## Reproduction and expected behavior

`testdata/sema/interface_overload_matrix/contracts_invalid.sec` implements
`Convert(int) int` and `Convert(bool) int` against requirements that include
`Convert(bool) bool`. The old diagnostic selected the unrelated int overload,
reporting a parameter mismatch instead of the actual incorrect result type.

The diagnostic must identify the failed requirement and its corresponding
implementation signature. Rules: `rules/declarations/interfaces.md` §§6,12;
`rules/tooling/diagnostics.md`. Owner: `frontend.interfaces`.

## Resolution

Prefer matching static/instance category and parameter-type/variadic identity
before the existing arity fallback when choosing a diagnostic witness.
Conformance acceptance and source-level overload ranking are unchanged.
The Sema/CLI/LSP regression matrix also covers generic and diamond inheritance,
static and instance overloads, view and constrained calls, duplicate requirements,
conflicting returns, requirement links and concrete signature help.
