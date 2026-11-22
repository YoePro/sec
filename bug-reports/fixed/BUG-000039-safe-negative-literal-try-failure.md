# BUG-000039 — Safe negative literal creates an impossible try failure

- Status: fixed (2026-11-22)
- Owner: `frontend.errorhandling-v2`
- Phase: Sema operator/failure facts and canonical IR consumption
- Rules: `rules/errors/runtime_checks.md` §§5.1,7;
  `rules/errors/errorhandling.md` §§9,11–13;
  `rules/foundations/operators.md` §§18.1–18.2;
  `rules/compiler/semantic_ir.md` §58.

## Reproduction

In `sec/stdlib/temporal/business/calculate.sec`, a protected call of the form
`try _findBusinessDay(value, rules, -1)` reported S1093 even though the
callee and enclosing function both use `Result[date, BusinessError]`.

The minimal source fixture is `testdata/sema/try_negative_literal_valid.sec`.
It reproduces the same failure using an integer success value. Before the fix,
`TestTryNegativeLiteral` reports ArithmeticError from `(-1)` at the first
32-bit Interactive case. The return type is not the problem: try includes
checks inside argument expressions, and unary negation was unconditionally
marked as capable of overflow, including this safe constant.

## Fix

Sema proves only builtin signed integer literal negations with an exact result
inside the selected canonical representation's bounds. The literal expression
may have an untyped integer shell without bounds, so the proof consults the
selected canonical builtin rather than guessing a width from that shell.

The immutable operator fact carries the exact constant spelling, has no runtime
check and contributes neither a panic effect nor a try arithmetic failure.
Canonical IR materializes the proven result as an integer constant. It does not
emit a single-result checked operation or reconstruct a dead panic edge.
The operator lowering function was extracted from the oversized builder into
`internal/ir/semantic/operator_lowering.go`, with explicit rulebook traceability;
its old exemption was removed with the traceability updater.

Dynamic negation still can overflow at the signed minimum. Named contracts,
decimal operators and other arithmetic proofs were not weakened.

## Verification

- Full Sema: 31.353s; full canonical IR: 8.478s; traceability: 0.249s.
- Focused CLI/LSP/IR regressions: 3.038s / 2.918s / 0.018s.
- Both widths and all three Sema depths accept the safe literal argument.
- Dynamic negative arguments retain exactly one S1093 in Sema/CLI/LSP.
- Verified IR preserves signed minima and zero as constants and keeps the
  dynamic checked negation.
- The actual calculate.sec CLI probe no longer reports ArithmeticError from
  `(-1)`. It still reports three unrelated errors in
  `sec/stdlib/temporal/calendar/partial_validation.sec`; a clean full module
  compilation is not claimed.

The minimal reproduction reports one error before the fix, so a separate
editor publication/display duplication in the original report is not established.
No blanket diagnostic deduplication or error-channel widening was added.
