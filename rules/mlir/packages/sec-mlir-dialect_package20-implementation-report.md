# SEC-MLIR-P20 Implementation Report — Binary Floating Point

> See the [live implementation ledger](../../../governance/lowering.yaml)
> (`lowering.sec-mlir-package20`) and the work list `package20-todo.md`.

- Status: report for package 20 (§ 94), written 2026-11-23 (P20-38)
- Package specification: `rules/mlir/packages/sec-mlir-dialect_package20.md`
- Work list and acceptance evidence: `package20-todo.md`
- Governance owner: `lowering.sec-mlir-package20` in `governance/lowering.yaml`

Package 20 is **not complete**. The binary-float core is implemented end to
end on the one native track. What is still open:

- the rulebook reconciliation (P20-03, P20-37);
- the complete repository suite (P20-35), which the owner runs;
- decimal and unit static leaves (P21 and units).

The acceptance matrix is in § 45 at the end of this report.

## 1. Repository HEAD

The work is built on `5b2e5b29` plus the uncommitted working tree:

- 222 changed entries;
- 111 of them untracked.

The 2026-10-18 inventory was taken at `831f6e1`. The historical `e0af215`
baseline is context only, as stated in `package20-todo.md`.

## 2. Local P18/P19 predecessor status

- P13–P17 are closed.
- P18 has a report (`sec-mlir-dialect_package18-implementation-report.md`)
  and is partial.
- P19 is partial; its schema 15 is admitted.
- All predecessors are local in this working tree.

Schema 16 is admitted after schema 15, so the order in package 20 § 73 holds.

## 3. Previous package status

These regressions stay green on 2026-11-23:

- **Lit:** 261/261 for schemas 1–16.
- **Go suites in full:** `internal/sema/...`, `internal/ir/semantic`,
  `internal/lowering/...`, `internal/traceability`.
- **Compiler subset:** `cmd/compiler -run
  'Float|Static|Conversion|Contract|Default|CTE|CompileTime'` passes.
- **LSP subset:** `cmd/lsp -run 'Float|CTE|CompileTime|Conversion'` passes.
- **`go vet`:** clean on the touched packages.

No pre-existing failures were observed in these runs.

## 4. Float/numeric normative synchronization

The authority for the current implementation is the reconciliation section of
`package20-todo.md`:

- **MD-014:** plain `float` follows the selected plan.
- **MD-012 and the package-22 compatibility note:** `ConversionError`
  replaces `NumericConversionError`.
- **MD-101:** there is one compilation track.

Still open (P20-03):

- `sec_float_numeric_sync_v1.md` and the Semantic IR float amendment
  `sec_semantic_ir_float_v1.md` are not yet merged into the canonical
  rulebooks (`types.md`, `semantic_ir.md`).
- The canonical dialect and lowering rulebooks still declare v11.

## 5. Files added

Sema:

- `float_core.go`, `float_operation_facts.go`, `float_defaults.go`,
  `float_literal_range.go`, `float_intervals.go`
- `float_target_capabilities.go`
- `numeric_conversion_plan.go`, `numeric_conversion_validation.go`
- `contract_cte_operators.go` (float part), `contract_cte_conversions.go`,
  `contract_cte_float_core.go`

Semantic IR:

- `float_ops.go`, `float_semantics.go`, `numeric_conversion.go`

Layout:

- `float_capabilities.go`

MLIR:

- `Analysis/VerifyFloatSemantics.cpp`, `Analysis/VerifyNumericConversions.cpp`
- `Conversion/SecToLLVM/Floats.cpp`, `FloatRemainder.cpp`, `FloatLowering.h`

Native fixtures, all in `testdata/build/native/`:

- `floats`, `float_literals`, `float_operator_matrix`
- `float_core`, `float_core_matrix`
- `float_conversion_exact`, `float_conversion_checked`
- `float_statics`
- `float_cte_parity`, `float_cte_nonfinite`, `float_cte_conversions`,
  `float_cte_core`

Lit tests:

- **Arithmetic and core lowering:** `floats`, `float-core`, `float-clamp`,
  `float-clamp-execution`, `float-remainder`, `float-standard-output`.
- **Conversions:** `float-conversions`, `convert-checked`,
  `numeric-conversions`.
- **Statics, schema and policy:** `float-statics`,
  `schema16-float-constants`, `float-policy-facts`, `verify-float-semantics`,
  `check-float-lowering-support`.
- **Negative tests:** `invalid-float-ops`, `invalid-float-semantics`,
  `invalid-float-weakenings`, `invalid-float-conversions`,
  `invalid-numeric-conversions`, `invalid-conversion-proof`,
  `invalid-float-static`, `invalid-unresolved-float-lowering`.

## 6. Files modified

- **Sema:** `analyzer_inference.go`, `semantic_facts.go`,
  `checked_numeric_conversions.go`, `contract_cte.go`, `static_initializers.go`.
- **Semantic IR:** `builder.go`, `verifier.go`, `static_storage.go`.
- **Emitter:** `emitter.go`, `emitter_options.go`.
- **Platform:** `internal/platform/target/target.go`.
- **Layout:** `internal/layout/scalar_plan.go`.
- **Diagnostics:** `internal/diagnostics/diagnostics.go` (S1260).
- **MLIR:** `SecOps.td`, `SecOps.cpp`, `SecDialect.cpp`, `SecToLLVM.cpp`.

## 7. NumericConversionError implementation

There is no `NumericConversionError`. The canonical core `ConversionError`
(MD-012) carries `NonFinite`, `OutOfRange` and `PrecisionLoss`. Sema never
creates a second public error family. `TestFloatConversionErrorChannel`
checks that a mismatched channel is S1093.

## 8. Float format representation

- `float32` is binary32 and `float64` is binary64.
- Plain `float` takes its width from the selected scalar plan. Without a plan
  its width is explicitly unknown, and the source TypeID stays distinct.
- `FloatSemantics` facts record the following on every operation family:
  - format;
  - nearest-even;
  - no fast-math or contraction;
  - preserved subnormals;
  - the comparison predicate;
  - truncation remainder.

## 9. Direct literal parser and rounder

- The parser keeps the exact source spelling.
- `constant.FloatLiteralBits` rounds the exact rational once, directly to the
  resolved format. CTE and Semantic IR share it (P20-10).

## 10. No-double-rounding tests

The 63-case literal matrix (P20-29) uses independent rational oracles on both
plans. It includes decimal and integer-form double-rounding witnesses, and
native `float_literals` executes them.

## 11. `sec.const.float` lowering

- The constant carries canonical `bits`, and lowering produces `arith.constant`
  from those bits without reparsing the lexeme.
- From schema 16 the `bits` are required, and the emitter refuses a
  lexeme-only constant.

## 12. Float arithmetic lowering

`add`, `sub`, `mul` and `div` lower to `arith.addf`, `subf`, `mulf` and `divf`
with no fast-math flags. `--sec-verify-float-semantics` gates the lowering.

## 13. Remainder lowering

The truncation remainder uses exact integer significand reduction in CF
(`FloatRemainder.cpp`) instead of `arith.remf`. This removes the undeclared
`fmod` dependency. The test is 584 differential cases (P20-30).

## 14. Compare predicates

- `==` lowers to `oeq` and `!=` to `une`.
- `<`, `<=`, `>` and `>=` lower to the ordered predicates.
- Membership equality uses the same predicates.

## 15. Legacy UNE correction

Source `!=` is unordered-not-equal everywhere: Sema facts, IR, MLIR and CTE.
`NaN != NaN` is true.

## 16. No-fast-math enforcement

`--sec-verify-float-semantics` rejects:

- non-empty fast-math;
- contraction;
- denormal flushing;
- weakened inherited policies.

The negative tests are in `invalid-float-weakenings.mlir`.

## 17. Rounding-mode enforcement

Only nearest-even is admitted. Explicit `RoundingMode` conversions (MD-047)
are a named unsupported-lowering boundary until P21.

## 18. Property constants

`Min`, `Max`, `Epsilon`, `Infinity`, `NegativeInfinity` and `NaN` lower to
exact APFloat constants. `NaN` is the canonical quiet NaN. CTE folds the same
values.

## 19. Classification implementation

`IsFinite`, `IsInfinite`, `IsNaN`, `IsNormal` and `IsSubnormal` use
`arith.bitcast` and format-specific field masks. No runtime library is
needed.

## 20. Rounding methods

`Floor`, `Ceiling`, `Truncate` and `Round` (ties to even) use a
`|x| < 2^p`-guarded `fptosi`/`sitofp`. Signed zero, NaN and infinities are
preserved.

## 21. Min/Max/Clamp

- `Min` and `Max` lower to `arith.minimumf` and `maximumf`, which propagate
  NaN.
- `Clamp` lowers to `sec.float.clamp_checked`, which yields
  `RangeError.InvalidRange` on failure.
- The value is consumed only on the success edge.
- Native tests: 45 Clamp cases, and 6,868 core differential checks.

## 22. ResolvedNumericConversionPlan

- The plan is a detached, read-only snapshot. It covers all 96 primitive pairs
  through 256-bit integers.
- It records:
  - the selected widths;
  - the exactness basis;
  - the ordered failures;
  - the checks;
  - the integer bounds.
- Queries never infer.

## 23. Exact conversion lowering

`sec.numeric.convert_exact`, which carries its proof, lowers to a single
`extf`, `truncf`, `sitofp`, `uitofp`, `fptosi` or `fptoui`.

## 24. Checked conversion lowering

- `sec.numeric.convert_checked` returns the candidate, a `failed` flag and one
  reason flag per listed failure.
- The flags are tested in priority order and lower to guarded CF in
  `sec-lower-checked-integers`.

## 25. Int-to-float exactness algorithm

`sitofp`/`uitofp` produces the candidate. The conversion is checked for
infinity and for an exact round trip through an integer one bit wider. Sema
proves wide singleton constants exactly.

## 26. Float-to-int guard algorithm

The guards run in this order:

1. unordered or infinite;
2. exact power-of-two bounds (vacuous beyond the format);
3. integrality through i64 for `|x| < 2^p`.

`fptosi`/`fptoui` sees only a value selected after all three guards.

## 27. Float-narrow exactness algorithm

- The candidate is `truncf`.
- Overflow to infinity fails.
- The `extf` round trip must be exact.
- NaN and infinities succeed.

## 28. Float-to-bool

`sec.float.to_bool` lowers to `cmpf une` with +0.0, so NaN is true. CTE folds
the same value.

## 29. Result/try integration

The failure edge dispatches the reason flags to static `ConversionError`
variants through the existing failure-set router. This covers naked
propagation, local handlers and catch-all handlers. No hidden union is
created.

## 30. Schema-v16 types and ops

- Schema 16 is admitted.
- The P20 operations have been admitted since schema 11, which is a deviation
  (§ 43).
- The reason type is represented as one-hot `i1` flags.

## 31. Float verifier

`--sec-verify-float-semantics` checks:

- formats and payloads;
- policies and predicates;
- remainder and method properties;
- that Clamp candidates are used only on success.

It runs standalone and before `sec-lower-floats`.

## 32. Numeric conversion verifier

`--sec-verify-numeric-conversions` checks:

- the classification;
- the immediate `failed` branch and success-only use of the candidate;
- the reason priority order;
- the canonical variant mapping.

It runs on every emit and native route.

## 33. Target capability checks

`layout.FloatCapabilities` is part of the plan:

- Every target with an LLVM triple is IEEE-preserving.
- A target without a triple is Unknown.
- Sema reports S1260 `float.target-capability` when a known contract lacks a
  required semantic.

## 34. Constant folding parity

CTE folds float `+ - * / %` with IEEE per-operation rounding, signed zero and
fmod. It also folds:

- IEEE comparisons;
- non-finite values;
- type properties;
- proven-exact conversions;
- the total core methods.

Each area has a native fixture comparing the folded value with the runtime
value: `float_cte_parity`, `float_cte_nonfinite`, `float_cte_conversions` and
`float_cte_core`.

## 35. Wide-integer conversion tests

Native and Sema tests cover:

- `int128` max → `float32`;
- `2^126` (`int128`) and `2^200` (`int256`) exact;
- `uint256` max → `float32` (`OutOfRange`);
- `±2^127` → `int128`.

## 36. NaN, signed-zero and subnormal tests

The operator matrix, the core matrix and the conversion fixtures cover:

- both NaN operand orders;
- ±0;
- the smallest and largest subnormals;
- the smallest normal;
- infinities.

NaN payload identity is not assumed.

## 37. Unsupported SquareRoot/decimal/string tests

- **SquareRoot** is outside the surface (no `MathError`, § 37).
- **decimal/decimal128** get `[unsupported-lowering]`, tested in
  `check-float-lowering-support.mlir`.
- **Float formatting** stays with P23.

## 38. CMake commands

`cmake --build build/sec-mlir -j 4` (ninja `-j 4`).

## 39. Exact LLVM/MLIR version

- LLVM/MLIR 24.0.0git at llvm-project `252f31a6893b` (2026-07-22).
- Go 1.26.8.
- System clang 21.1.8. It only links; `llc` comes from the same LLVM build.
- `SEC_MLIR_BIN=build/sec-mlir/bin`.

## 40. check-sec-mlir result

`llvm-lit build/sec-mlir/test` passes 261/261.

## 41. go test ./... result

The full suite was not run in this report. The focused Go suites and subsets
of § 3 pass. The complete repository run is P20-35 and belongs to the owner.

## 42. End-to-end source → schema-v16 → Arith/Math results

Source fixtures execute natively through Sec MLIR → Arith/CF → LLVM.
`float-standard-output.mlir` checks the schema-16 Arith output and the absence
of `arith.remf`, `math.*`, fast-math and `llvm.call`.

## 43. Deviations

The deviations are recorded in `sec_mlir_dialect_v16.md` § 18:

1. The P20 operations are admitted early, from schema 11. Schema 16 adds the
   canonical-bits rule. The native route keeps schema 11 until schema-13+
   constructs have a physical native entry.
2. Reason flags replace `!sec.numeric_conversion_reason` and `from_reason`.
3. Abs, rounding and remainder stay Arith/CF only instead of `math.*` and
   `arith.remf`, so no libm is needed.
4. Conversions lower in `--sec-lower-to-llvm`.
5. LLVM is the end of the one track.

The following are superseded:

- plain `float` as binary64 (MD-014);
- `NumericConversionError` (MD-012);
- legacy backend parity (MD-101).

## 44. Recommendations for Package 21

1. Reuse `ResolvedNumericConversionPlan` and the proven-exact CTE path for
   decimal ↔ float. Do not add a second classifier.
2. Add decimal and unit leaves to `StaticInitializer` and `sec.static.global`.
   This is the only remaining P20 governance item.
3. Carry an explicit `RoundingMode` (MD-047) in the plans instead of
   defaulting to nearest-even.
4. Merge the P20 Semantic IR float amendment and the float sync into the
   canonical rulebooks before installing the P21 amendments on top.

## 45. Acceptance matrix (§ 93, P20-36)

Status values:

- **met:** the criterion holds with the evidence cited.
- **deviation:** the criterion is reconciled with a later authority
  (§ 4, § 43).
- **partial:** part of the criterion holds; the rest is open.
- **not run:** the required run has not been done.

| # | Criterion | Status | Evidence |
|---|---|---|---|
| 1 | baseline e0af215 + P18/P19 chain | deviation | § 1; current checkout is the baseline |
| 2 | previous regressions green | met | § 3 |
| 3 | float/numeric sync applied | partial | § 4; P20-03 |
| 4 | NumericConversionError in core | deviation | § 7; ConversionError (MD-012) |
| 5 | Semantic IR float amendment applied | partial | implemented; canonical merge open (P20-03) |
| 6 | schema-v16 rulebook installed | met | `sec_mlir_dialect_v16.md` |
| 7 | lowering-v16 rulebook installed | met | `sec_mlir_lowering_v16.md` |
| 8 | binary32/binary64 format facts | met | § 8 |
| 9 | nearest-even policy | met | §§ 8, 16 |
| 10 | no fast-math | met | § 16 |
| 11 | no denormal flush | met | § 16 |
| 12 | direct literal rounding | met | § 9 |
| 13 | no f64 intermediate for f32 literal | met | §§ 9–10 |
| 14 | const.float → arith.constant | met | § 11 |
| 15 | add/sub/mul/div/rem | met | §§ 12–13 (rem: deviation 3) |
| 16 | compare predicates | met | § 14 |
| 17 | ONE → UNE fix | met | § 15 |
| 18 | properties | met | § 18 |
| 19 | Abs/Min/Max/Clamp | met | § 21 |
| 20 | classification | met | § 19 |
| 21 | Floor/Ceiling/Truncate/Round | met | § 20 |
| 22 | ResolvedNumericConversionPlan | met | § 22 |
| 23 | exact/checked conversion ops | met | §§ 23–24 |
| 24 | conversion error mapping | deviation | § 29; ConversionError |
| 25 | float narrowing exactness | met | § 27 |
| 26 | int → float 128/256 | met | § 35 |
| 27 | float → int checked | met | § 26 |
| 28 | float → bool | met | § 28 |
| 29 | try/local handlers | met | § 29 |
| 30 | float verifier registered | met | § 31 |
| 31 | conversion verifier registered | met | § 32 |
| 32 | checked conversion lowering | met | § 24 |
| 33 | float core lowering | met | §§ 19–21 |
| 34 | constant-fold parity | met | § 34 |
| 35 | target capability checks | met | § 33 |
| 36 | SquareRoot deferred | met | § 37 |
| 37 | decimal deferred to P21 | met | § 37 |
| 38 | no LLVM dialect required | deviation | § 43(5); Arith output tested |
| 39 | check-sec-mlir | met | § 40 |
| 40 | go test ./... | not run | § 41; P20-35 |
| 41 | legacy paths operational | deviation | MD-101 one track |

Totals over the 41 criteria:

- 33 met;
- 5 deviation;
- 2 partial;
- 1 not run.
