# BUG-000051 — Try argument overflow proofs

## Status

Open; local arithmetic proof fixed, returned-field range propagation remains.

## Report and reproduction

`sec/stdlib/temporal/business/combine_arithmetic.sec` uses bodyless try on
Result-returning calls with `origin + 1` and `coverage.Last + 1` arguments.
The former widens an int32 date property to int64; its addition cannot overflow.
Before the fix, `testdata/sema/try_arithmetic_proof/valid.sec` produced S1093
for widening, a narrow property, a constant and a dominating guard.

`rules/errors/errorhandling.md` §§11,13.1 intentionally includes call arguments
in the protected subtree. The owner explicitly retained that rule. The defect
is missing overflow proof consumption, not a different try boundary.
`rules/errors/runtime_checks.md` defines static proof and check elimination.
Governance owner: `frontend.errorhandling-v2`. Affected phase: Sema, shared by
CLI and LSP.

## Implemented fix

Canonical checked integer facts record arbitrary-precision interval proofs for
builtin addition, subtraction and multiplication. Try failure collection omits
only operations proven incapable of overflow, still visits their operands and
retains the Result call's error channel. Checked lowering remains intact.
Unknown int64 parameters, fields and nested possible overflow still give S1093.
Named/contracted and unit arithmetic are outside this proof.

Source-backed Sema, CLI and LSP regressions pass. A native program exercises
both int32 extrema widened to int64 and exits 42. Full Sema, Semantic IR and
traceability suites pass.

## Remaining

`coverage.Last` is an int64 field returned by `_validateCombinedCoverage`.
Although that helper builds it from widened date bounds, the compiler has no
numeric returned-field interval summary through its loop and Result payload.
The caller therefore cannot yet prove `coverage.Last + 1` safe. Keep this report
open until sound cross-function range facts cover that path. No library source
or helper-name special case was introduced.
