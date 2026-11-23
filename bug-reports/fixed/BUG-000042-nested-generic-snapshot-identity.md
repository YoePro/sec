# BUG-000042 — Nested generic instances lose their argument identity in snapshots

## Status

Fixed, 2026-11-23.

## Report and reproduction

Hover inspection of a parameter typed `Box[Box[int]]` exposed a detached source
type fact with the argument `Box`, losing the inner `int`. The snapshot cycle
guard used only module and declaration name, so it confused distinct finite
instances of one declaration with recursion. The source-backed regression
`TestGenericInstantiationHover/Box[Box[int]]/Box` reproduced `Z = Box` instead
of `Z = Box[int]` before the fix.

## Expected behavior

Generic type identity includes the ordered concrete arguments. Snapshotting
must preserve distinct instances and detach mutable state without mistaking
ordinary finite nesting for a recursive edge.

## Scope

- Area: immutable Sema type facts and LSP specialization presentation.
- Rulebooks: `rules/declarations/generics.md` §§4,24–26;
  `rules/compiler/compiler_analysis.md` immutable analysis results;
  `rules/tooling/lsp.md` §18(2).
- Governance: `frontend.generics-v2`.
- Phase: post-Sema analysis snapshots.

## Resolution

The snapshot recursion guard uses the existing canonical concrete type identity,
including ordered arguments, instead of just the declaration name. Actual
recursive instances still terminate. Source-backed hover regressions cover both
outer `Box[Box[int]]` and inner `Box[int]`; compiler query tests verify detached
facts and unchanged specialization caches/demand. Full Sema, Semantic IR and
traceability suites pass, together with the focused snapshot regressions.
