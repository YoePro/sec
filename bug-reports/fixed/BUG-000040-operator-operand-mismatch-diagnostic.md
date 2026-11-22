# BUG-000040 — Unregistered binary operand mismatch diagnostic

- Status: fixed (2026-11-22)
- Owner: `errors.diagnostics`
- Rules: `rules/tooling/diagnostics.md` §§5–9,12,15;
  `rules/foundations/operators.md` §§16–17;
  `rules/types/types.md` (No implicit conversion).

## Finding

The reported `cannot apply operator - to uint and int` message was emitted
without registry metadata. Four identical reporting sites in Sema covered
plain arithmetic, bitwise type mismatches and decimal-family rejection.
This violated diagnostics.md §12(2), independently of whether the language
should accept the operand pair. The numeric-carrier-specific message is a
separate reporting path and is not covered by this fix.

## Resolution

Registered **S1259**, `operator.incompatible-operand-types`, family `operators`,
mandatory Error. A focused `operator_diagnostics.go` helper retains the exact
message and operator token span, adding guidance to select compatible types
and explicitly handle any fallible conversion. No automatic cast or source fix
is offered and no numeric acceptance rule changed.

The existing plain-arithmetic inference function was moved out of the oversized
analyzer into `arithmetic_inference.go`, retaining behavior and adding rulebook
traceability. Its existing exemption was removed by the prescribed updater.

## Verification

Fixtures cover uint/int subtraction, mismatched integer widths, bitwise widths,
and float/decimal operands. Same-type arithmetic and contextual integer literals
remain accepted. Sema tests check all three depths on 32/64-bit plans, ID,
mandatory severity, help and exact one-character operator spans. CLI JSON marks
all four errors registered as S1259; `sec diagnostics S1259` resolves the entry.
LSP diagnostics share the ID, span and help.

Focused Sema/CLI/LSP tests pass (0.036s/1.973s/3.713s). The broader Sema
operator/arithmetic/numeric/bitwise/unit-conversion selection passes (0.894s),
as do diagnostic registry tests (0.002s), full traceability (0.223s) and the
CLI diagnostic catalog/operator selection (1.271s). This does not claim a full
Sema suite run during concurrent compilation-mode work.
