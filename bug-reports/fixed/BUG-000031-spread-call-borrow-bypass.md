# BUG-000031 — Prepared spreads bypass call borrow checks

## Status

Fixed 2026-11-20. Unsupported provenance cases retain explicit capability
diagnostics rather than publishing unproven call facts.

## Reproduction

`testdata/sema/spread_borrow_audit_invalid.sec` contains:

```sec
fn Exclusive(owner: ref mut int[2], first: int, second: int) void {}
fn Overlap(values: ref mut int[2]) void {
    Exclusive(values, values...)
}
```

The pre-fix Sema probe accepted this call and published resolved call facts.
It also published a call for `ReturnFirst(values...)` whose expanded inputs
were borrowed parameters; static inspection showed returned-origin
instantiation used the whole array expression rather than per-element facts.

## Expected and actual behavior

Borrowing §15.4 prohibits the overlapping read while an earlier exclusive
argument reservation is live. Skipping prepared slots bypassed that rule.
Reference/retention consumers also lacked per-slot provenance despite the new
source/index facts. An array-wide origin cannot certify such reference uses.

## Scope

- Area: frontend spread, borrow and return-reference analysis.
- Rules: `rules/declarations/spread.md` §§3–4;
  `rules/memory/borrowing.md` §15.4;
  `rules/memory/lifetime_analysis.md` §28;
  `rules/compiler/compiler_pipeline.md` §32.
- Governance: `frontend.spread-validity-and-ownership`.
- Phase: Sema, with CLI/LSP diagnostic projection.

## Resolution

Prepared source reads now use canonical source/slot and referent-Place facts
against earlier reservations, producing `S1161` with the original borrow
location. Missing per-slot reference/capture provenance produces the existing
`S1239` implementation boundary before publishing resolved call/return facts.
Expected-type generic calls now use prepared-value flags consistently across
ownership, borrow, escape, reference-plan and source-move bookkeeping.
Computed spread sources evaluated after borrowed arguments also stop at
`S1239` until their access effects can be checked against the reservations;
materializing the source before the call is accepted.
Empty spreads retain the source check at their original position even though
they supply no parameter slots.

Tests: `internal/sema/spread_borrow_audit_test.go`,
`cmd/compiler/spread_borrow_audit_test.go`,
`cmd/lsp/spread_borrow_audit_test.go`, and paired valid/invalid source fixtures.
The audit and retained implementation boundaries are documented in
`governance/spread-borrow-audit.md`.

Verification on 2026-11-20:

- Race-enabled `TestSpreadBorrowAudit` suites passed in Sema, CLI and LSP,
  including eight rejection cases and valid controls. Sema runs both pointer
  widths and all three analysis depths.
- Focused spread, call-argument, borrow, reference, return-origin, type-pack
  and expected-generic tests passed in Sema and Semantic IR.
- `go vet` passed for Sema, CLI and LSP; implementation traceability passed.
