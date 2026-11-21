# BUG-000036 — Unnamed union payload loses carried reference origin

## Status

Fixed 2026-11-21.

## Reproduction

```sec
type Choice union { Single(ref int) Named { value: ref int, } }
fn Bad() Choice {
    let local := 1
    return Choice.Single(ref local)
}
```

The pre-fix paired regression accepted `Single(ref local)` and rejected only
the equivalent named-field payload. This was directly observed in Sema; no
native execution of a dangling reference was attempted.

## Expected and actual behavior

The returned union carries a reference to function-local storage and must be
rejected. Constructor syntax cannot remove that lifetime dependency. The
call-shaped constructor lacked an expression/contained origin, unlike the
struct-shaped path, so local holder and returned-origin analysis could lose it.

## Scope

- Phase: Sema reference/escape analysis and diagnostic projection.
- Governance: `frontend.union-constructor-value-semantics`.
- Rules: `rules/declarations/unions.md` §§5,7.8;
  `rules/memory/lifetime_analysis.md` §§14,28;
  `rules/memory/borrowing.md` §§12–14.

## Resolution

Both forms record direct and contained origins from accepted typed operand
plans. Variant/field paths preserve dependencies for bound/nested values and
canonical function-return/call substitution. Recorded contents, not AST lists,
feed aggregate propagation. Existing S1128/S1130 escape diagnostics retain
referent links; unknown provenance is never promoted to proof.
Paired shared/mutable, bound, nested, forwarded and branch-scoped fixtures and
valid caller-origin controls exercise Sema, CLI and unsaved LSP projection.
The complete audit is `governance/union-reference-origin-audit.md`.


## Verification

The original two-case probe observed one error, demonstrating acceptance of
its unnamed local-reference return. After the fix, twelve paired escapes are
rejected across both scalar plans and all three analysis depths. Valid shared
and mutable caller origins, bound/nested return paths and nonescaping controls
pass. Complete Sema, Semantic IR and Sec MLIR emitter suites pass. CLI check/IR
and unsaved LSP races cover the same diagnostics and related referent links.
No native reference execution or new lifetime precision is claimed.
