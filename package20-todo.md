# SEC-MLIR Package 20 — implementation TODO

Package: `SEC-MLIR-P20` — Floating-Point Semantic Operations and Arith Lowering  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package20.md`  
Package contract: `rules/mlir/packages/sec-mlir-dialect_package20.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_float_numeric_sync_v1.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_float_v1.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v16.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v16.md`
- `rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md`
- `rules/mlir/normative-sync/sec_p20_p21_conversion_compatibility_package22.md`

This is an implementation plan, not a completion report or a new language
decision. It continues P15–P19, whose incomplete items remain their owners.
The historical `e0af215` snapshot is context only; execution starts from the
current checkout and must record a reproducible snapshot before acceptance.

## Inventory snapshot — 2026-10-18

Audited HEAD:

```text
831f6e1
```

The working tree has 134 changed entries. This inventory includes the current
working tree, not only HEAD. Capture the intended predecessor changes and
untracked files before treating any completion claim as reproducible.

| Area | Current implementation | P20 consequence |
|---|---|---|
| Float type facts | Sema and Semantic IR resolve `float32`/`float64` and target-sized plain `float`; MD-014 makes plain `float` binary32 on 32-bit plans and binary64 on 64-bit plans. | Retain selected-plan provenance; do not restore P20's obsolete fixed-binary64 plain-float assumption. |
| Float frontend/IR subset | `internal/sema` resolves float arithmetic/comparisons and `internal/ir/semantic/float_ops.go` represents format-precise constants, negation, binary operations and comparisons. | Treat this as an early, partial P20 implementation; extend rather than duplicate it. |
| Literal payload | `FloatConstant` carries direct-width bits and the emitter writes `sec.const.float` with canonical bits and lexeme. | Complete all source-to-MLIR verification and constant-fold parity; never reparse the lexeme in lowering. |
| Current Sec dialect | ODS/C++ contain `sec.const.float`, `sec.float.neg`, `.binary` and `.cmp`; the unified native track has a `sec-lower-floats` conversion path. | This is not schema-16 acceptance: the full P20 operation set, v16 gate and dedicated verifiers remain absent. |
| Conversions | `ConversionError` is the canonical public failure family, and exact integer conversion proofs already use `numeric.convert-exact`. | Reconcile every P20 reference to the obsolete public `NumericConversionError`; no second source-visible error enum may be created. |
| Core surface | Existing float methods/properties are not yet represented as the complete P20 intrinsic surface or lowered as P20 operations. | Implement only the rulebook-defined subset; keep `SquareRoot`, decimal, formatting and parsing deferred. |
| Compilation track | Sec has one native track via Sec MLIR and standard MLIR to LLVM. | P20 must widen that track; it must not reintroduce the retired legacy backends mentioned by the August package. |

Primary implementation anchors:

```text
internal/sema/analyzer_inference.go
internal/sema/semantic_facts.go
internal/sema/checked_conversions.go
internal/sema/constant/float.go
internal/ir/semantic/float_ops.go
internal/ir/semantic/builder.go
internal/ir/semantic/verifier.go
internal/lowering/secmlir/emitter.go
mlir/include/sec/Dialect/Sec/SecOps.td
mlir/lib/Dialect/Sec/SecOps.cpp
mlir/lib/Conversion/SecToCore/SecToCore.cpp
mlir/lib/Conversion/SecToLLVM/Floats.cpp
```

## Current authority and non-negotiable reconciliation

1. **Plain float width:** MD-014 supersedes P20 §§4–5: plain `float` follows
   the selected CompilationPlan (binary32 on 32-bit, binary64 on 64-bit). The
   source identity remains distinct from `float32` and `float64`.
2. **Conversion errors:** MD-012 and the P20/P21 compatibility note supersede
   P20 §§39–41, 43, 56–58 where they prescribe public
   `NumericConversionError`. Checked P20 conversions use the canonical core
   `ConversionError` and its established variants/mapping. An internal reason
   enum may exist only if it maps deterministically to that public contract.
3. **One compilation track:** P20's legacy-parity wording is historical. All
   accepted lowering runs on the one Sec MLIR → standard MLIR → LLVM track.
   Remove or migrate stale references rather than retaining a second backend.
4. **No semantic invention:** P20 does not settle decimal arithmetic,
   decimal/float conversion, `SquareRoot`, float parsing/formatting, total
   ordering, lossy APIs, fast-math source modes, NaN-payload APIs, or a dynamic
   rounding environment. Record any newly encountered ambiguity in
   `missing-decisions.yaml`.
5. **Package order:** Schema 16 is after P19's schema 15. Do not claim a v16
   module or default schema until predecessor schema work has been reconciled
   in the current unified track.

## Completion boundary

P20 is complete only when the target-plan-aware binary floating core is
represented end-to-end: direct one-rounding literals; unary/binary arithmetic
and IEEE comparison behavior; the specified core properties/methods; checked
numeric conversions using `ConversionError`; constant-folding parity; schema
16 representation and verifiers; and lowering through standard MLIR without
fast-math, contraction, denormal flushing, hidden allocation, or a second
backend.

P20 does not own decimal/decimal128 operations (P21), dynamic/owning array
work (P18), Arena/context work (P19), `SquareRoot`, advanced math, string
materialization/formatting/parsing, total order, lossy APIs, NaN payload APIs,
or a new runtime/ABI model.

## A. Establish the execution baseline and synchronize authority

- [ ] P20-01 — Record the exact execution snapshot, untracked-file inventory,
  relevant P15–P19 predecessor matrix, Go/MLIR/Clang versions, and local
  `SEC_MLIR_BIN` setting. Acceptance must be reproducible from that snapshot.
- [ ] P20-02 — Run and record predecessor Go, vet, MLIR verification and
  native-track controls. Separate pre-existing failures from P20 failures.
- [ ] P20-03 — Reconcile the P20 package, both compatibility notes, the float
  sync, current `types.md`, operators, core-library, runtime-checks, layout,
  Semantic IR, dialect and lowering rulebooks. Preserve version history and
  update `language-rulebook-status.md` and owning governance entries.
- [ ] P20-04 — Update P20's stale source/error/track assumptions: selected-plan
  plain-float width, `ConversionError`, and one compilation track. Record any
  remaining rule conflict through the correction workflow rather than encoding
  it in Sema.
- [ ] P20-05 — Create or update the sole `lowering.sec-mlir-package20`
  integration in `governance/lowering.yaml` with predecessors, partial state,
  evidence, deferrals and the current baseline; validate with `yamlstatus`.

## B. Credit the implemented early subset

- [x] P20-06 — Resolve the binary-float family in Sema and Semantic IR:
  `float32` is 32-bit, `float64` is 64-bit, and plain `float` gets its width
  from the selected scalar plan while preserving source identity.

  Current anchors: `internal/sema/types.go`,
  `internal/platform/target/target.go`, and
  `internal/ir/semantic/builder.go`.

- [x] P20-07 — Represent direct-width float constants in Semantic IR with
  canonical binary bits, resolved 32/64 format, source lexeme and negative-zero
  handling; reject finite literal overflow and avoid a binary64 intermediate
  for binary32 constants.

  Current anchors: `internal/ir/semantic/float_ops.go` and
  `internal/ir/semantic/float_ops_test.go`. This credit does not yet establish
  complete parser/CTE/lowering parity.

- [x] P20-08 — Resolve and build the early unary-negation, add/sub/mul/div/rem
  and comparison subset as Semantic IR `float.neg`, `float.binary` and
  `float.compare`, with verifier coverage and SSA identity for unary plus.

  Current anchors: `internal/sema/semantic_facts.go`,
  `internal/ir/semantic/builder.go`, `internal/ir/semantic/float_ops.go`, and
  `internal/ir/semantic/verifier.go`.

- [x] P20-09 — Emit the early subset as Sec MLIR and provide its existing
  native-track lowering entry point, including canonical `sec.const.float`
  payload emission.

  Current anchors: `internal/lowering/secmlir/emitter.go`,
  `mlir/include/sec/Dialect/Sec/SecOps.td`,
  `mlir/lib/Dialect/Sec/SecOps.cpp`, and
  `mlir/lib/Conversion/SecToLLVM/Floats.cpp`. This is not yet proof that the
  complete P20 schema or semantics are accepted.

## C. Finish frontend and immutable Sema facts

Progress 2026-10-18 for P20-10: untyped literals are now shaped to the float
type beside a float operand (`a + 2.0`), in a float context (let, return,
argument) and through a sign prefix (`-1.5`); compound assignment on floats
resolves; `decimal % decimal` and decimal mixed with another numeric family no
longer type silently as invalid. CTE parity is not audited yet.

- [x] P20-10 — Audit lexer/parser and context shaping for `g` literals,
  unsuffixed float contexts, integer/base-prefixed forms, source spelling,
  direct-width rounding, overflow/underflow, and signed zero. Ensure CTE uses
  exactly the same rules (§§11–19, 82).
  Completed 2026-10-26 (Codex): parser retains exact source spelling even
  when its convenience float64 value underflows/rounds. Sema now shapes g
  literals in explicit-width contexts (including signs and chosen call
  parameters), refreshes stale sign-operator facts, admits unsuffixed integer
  forms in float contexts and preserves explicit non-float family boundaries.
  CTE and Semantic IR share direct-width source rounding; CTE retains negative
  zero through sign application and rechecking. Source overflow rejects rather
  than materializing infinity. Context handling extracted from analyzer.go into
  numeric_literal_context.go. Evidence: P20-29 matrix and native float_literals.

- [x] P20-11 — Publish immutable resolved float-operation facts: common type,
  nearest-even policy, no contraction/fast-math, truncation remainder and the
  exact source comparison predicate (§§20–27).
  Completed 2026-10-26 (Codex): `ResolvedFloatOperation` is a value-only
  snapshot published alongside successful operator/compound-assignment facts.
  It retains common source type, selected binary width (explicitly unknown for
  planless platform float), source operator, canonical comparison predicate,
  nearest-even/no-fast-math/no-contraction/subnormal-preserving policy and
  truncation remainder. Read-only expression/assignment APIs detach all facts;
  source-backed tests cover all 13 kinds and five compound kinds for all three
  primitive float types, all analysis depths and no-plan/32/64-bit plans.

- [ ] P20-12 — Add compiler-known property and method facts for `min`, `max`,
  `epsilon`, infinities, NaN, `Abs`, `Min`, `Max`, `Clamp`, classification and
  `Floor`/`Ceiling`/`Truncate`/`Round`; keep `SquareRoot` explicitly outside
  the lowering surface (§§28–38).

  Partial 2026-10-26 (Claude): owner decided PascalCase spellings
  (`core-library.md` §§ 7.1/8.1/9.1, note in P20 § 28). Sema resolves
  `T.Min/Max/Epsilon/Infinity/NegativeInfinity/NaN` and the methods `Abs`,
  `Min`, `Max`, `Clamp` (Result[T, RangeError]), `IsFinite/IsInfinite/IsNaN/
  IsNormal/IsSubnormal`, `Floor/Ceiling/Truncate/Round` on float, float32 and
  float64 (`internal/sema/float_core.go`, registry IDs `CKM-FLOAT-*`), with
  same-type argument and arity checks, a PascalCase misspelling diagnostic,
  and static-property receivers (`float64.NegativeInfinity.Abs()`).
  Remaining: `SquareRoot` (no `MathError` yet, § 37) and named/unit floats.
- [ ] P20-13 — Implement a detached, read-only resolved numeric-conversion
  plan for all P20 binary-float pairs, integer widths through 256 bits, and
  float-to-bool. Carry selected-plan width, exactness proof, runtime-check
  classification and canonical `ConversionError` mapping (§§42–61).
- [ ] P20-14 — Enforce typed conversion semantics: exact widening, checked
  narrowing with finite range and round-trip proof, integer-to-float exactness,
  safe finite/range/integral float-to-integer guards, and explicit float-to-bool
  behavior (§§45–55). Constant known failure is a diagnostic, not a guaranteed
  runtime branch.
- [ ] P20-15 — Integrate plans with existing `try`, local handlers and failure
  sets using `ConversionError`; retain success-only values and do not create a
  hidden union or a second public conversion-error family (§§56–59).

## D. Complete Semantic IR


  Partial 2026-10-26 (Claude): checked built-in integer conversions under
  `try` lower through the existing failure-set router: Sema's
  `TryFailureConversion` point gets `numeric.convert-checked` (candidate,
  failed; static `OutOfRange`), the failure edge builds the payload-less
  core `ConversionError.OutOfRange` and propagates or reaches local handlers,
  and the IR verifier requires every candidate use to sit behind the false
  edge of its own failed flag (`internal/ir/semantic/integer_conversion.go`,
  `checked_conversion_test.go`). Remaining: rune `InvalidScalar`, float
  conversions and their reason mapping (§ 72).
- [ ] P20-16 — Extend the early float subset with explicit operations and
  verifier rules for Abs, NaN-propagating Min/Max, checked Clamp,
  classification, rounding, properties, exact conversion, checked conversion,
  reason mapping and float-to-bool (§§63–72).

  Partial 2026-10-26 (Claude): `float.abs`, `float.minimum`, `float.maximum`,
  `float.classify`, `float.round` and `float.property` with kinds and a local
  verifier (`internal/ir/semantic/float_ops.go`); integer `Min/Max/Bits`
  build exact `const.int`. Clamp (2026-10-26) is `float.clamp-checked`
  (value, failed) building Err(RangeError.InvalidRange) or Ok; the value is
  consumed only on success. Remaining: conversions, float-to-bool.
- [ ] P20-17 — Make float-operation metadata explicit and verified: resolved
  binary format, nearest-even policy, no fast-math, no contraction, preserved
  subnormals, comparison predicate, truncation remainder and source scalar
  provenance (§§18, 20–27, 63–69).
- [ ] P20-18 — Model checked conversion candidate/failed/reason results with
  `ConversionError` mapping and prove candidate consumption is success-only.
  Preserve guard order: non-finite, range, then precision/integrality (§§41,
  48–54, 70–72).
- [ ] P20-19 — Make compile-time folding use the same exact literal rounding,
  arithmetic, comparison, core-method and conversion semantics as runtime;
  add NaN, signed-zero, subnormal, infinity and wide-integer parity tests
  (§§13–18, 82).

## E. Schema 16, emission and verification

- [ ] P20-20 — Install schema-16 version gating only after predecessor
  reconciliation. Add/refine the numeric-conversion reason representation and
  all P20 Sec operations in ODS/C++, parser/printer and registration (§§73–75).
- [ ] P20-21 — Extend the Go emitter to emit every verified P20 operation from
  Semantic IR facts only. It must write canonical float bits and never reparse
  literal text (§§74–76).

  Partial 2026-10-26 (Claude): emitter writes `sec.float.abs/minimum/maximum/
  classify/round/property`; ODS ops and verifiers in SecOps.td/SecOps.cpp,
  admitted early in schema 11 like the arithmetic subset; Clamp emits
  `sec.float.clamp_checked`. Remaining: conversions, schema-16 gating.
- [x] P20-22 — Add `--sec-verify-float-semantics` to reject invalid formats,
  payloads, non-nearest-even policy, fast-math, contraction/denormal weakening,
  invalid predicates/remainder/method properties and malformed Clamp (§81).
  Completed 2026-10-26 (Codex): standalone module verifier and the same gate
  before `sec-lower-floats`; registered operation validation plus strict inherited
  policies, canonical bits and success-confined Clamp candidates. Analysis lit
  cases cover positive/negative operation, policy and CFG cases. Legacy dialect
  round-tripping remains available without claiming schema-16 acceptance.

- [ ] P20-23 — Add `--sec-verify-numeric-conversions` to validate supported
  domains, exact-versus-checked classification, canonical error mapping,
  reason invariants, candidate dominance, guard ordering and try compatibility
  (§§79, 81).

## F. Lower through the single native track

- [x] P20-24 — Lower constants from canonical bits to standard MLIR constants;
  lower arithmetic to `arith` with nearest-even/no-fast-math semantics and no
  FMA contraction; lower remainder and comparisons with the required
  truncation/ordered-or-unordered predicates (§§75–77).

  Done 2026-10-18: `sec-lower-floats` (`mlir/lib/Conversion/SecToLLVM/Floats.cpp`)
  turns `sec.const.float {bits}` into `arith.constant` without reparsing the
  lexeme, `float.binary` into `addf/subf/mulf/divf` (no fastmath flags)
  and exact integer/CF significand reduction for truncation remainder and
  `float.cmp` into `cmpf oeq/une/olt/ole/ogt/oge`; scalar layout gives plain
  float f32/f64 by plan (MD-014). Evidence: `floats.mlir`,
  `invalid-float-ops.mlir`, native fixture `floats.sec`. The LLVM IR is now
  compiled by the same-build `llc` and only linked by clang (system clang 21
  rejected the newer textual float constants).
- [x] P20-25 — Lower properties, Abs, Min/Max, Clamp, classification and
  rounding to standard `arith`/`math`/`cf`, retaining NaN, signed-zero and
  subnormal semantics without a mandatory runtime library (§§78, 80).

  Completed 2026-10-26 (Claude implementation, Codex audit/execution):
  `sec-lower-floats` lowers the core members
  with Arith only (no runtime library or libm call): abs/classify by bitcast
  and field masks (§ 78), minimum/maximum to `arith.minimumf/maximumf` (§ 36),
  rounding through a |x| < 2^p guarded fptosi/sitofp that keeps signed zero,
  NaN and infinities, and properties to exact APFloat constants. Evidence:
  lit `float-core.mlir`, `invalid-float-ops.mlir`, native
  `testdata/build/native/float_core.sec` (float64/float32/plain float).
  Clamp lowers via cmpf uno/ogt and minimumf/maximumf (lit `float-clamp.mlir`).
  Verified by native `float_core_matrix` for all three primitive types and
  45 dialect-native Clamp cases (including NaN bounds/input, invalid/equal
  bounds, signed zeros and infinities). The native harness links no math
  library; public Result/union materialization is not claimed. Target
  capability checks belong to P20-27 and remain open there.
- [ ] P20-26 — Lower checked numeric conversion plans to total guarded CFG.
  No `fptosi`, `fptoui`, narrowing or integer-to-float cast may execute before
  its required proof/guards. Map failure deterministically to `ConversionError`
  and existing try control flow (§§48–54, 79).

  Partial 2026-10-26 (Claude): `sec.numeric.convert_checked` (ODS + verifier)
  lowers in `sec-lower-checked-integers`: the source is extended one bit beyond
  both widths by its signedness and compared with the target domain; the
  candidate is the exact width change, used only on success. Evidence: lit
  `convert-checked.mlir`, verifier negatives in `invalid-conversion-proof.mlir`,
  `cmd/compiler` emit-sec-mlir test over the core `ConversionError`. Native
  execution waits for union/Result lowering (ConversionError is a union).
- [ ] P20-27 — Add target capability checks for binary32/binary64,
  nearest-even, preserved subnormals, required comparison/remainder semantics
  and signed zero. Unsupported targets reject the operation/profile rather
  than weakening Sec semantics (§§6, 9, 83).
- [x] P20-28 — Register P20 support with `sec-check-lowering-support`, retain
  clear unsupported-lowering boundaries for incomplete surfaces, and verify
  there is no alternate or legacy backend route (§§79–84).

  Completed 2026-10-26 (Codex): all eleven currently represented Sec float
  operations (constants, arithmetic/comparison and core members including
  Clamp) are registered through one focused support classifier. Canonical bits
  are required at native-track entry; legacy lexeme-only constants receive
  `[unsupported-lowering]` before rewriting. Unsupported Sec operations receive
  the same explicit category. Lit covers full float registration, both the
  standalone gate and the unified native pipeline, decimal/decimal128 rejection
  and rejection of unimplemented binary-float conversions/float-to-bool by the
  current schema verifier. Checked integer conversions remain independently
  admitted; full float conversions, schema 16 and target capabilities remain
  their separate package points. Driver audit confirms only the one native
  `sec-lower-to-llvm` route (MD-101), with no alternate/legacy backend route.

## G. Required test matrix

- [x] P20-29 — Add parser/Sema/IR tests for all literal forms, direct f32/f64
  rounding, no-double-rounding witnesses, subnormal/underflow/overflow and
  negative zero (§85).
  Completed 2026-10-26 (Codex): shared source fixture has 63 literal cases
  across float32/float64/plain float, plus local/call/operand contexts. Sema CTE
  and Semantic IR tests run on both selected 32/64-bit plans using independent
  rational oracles. Cases include g/unsuffixed fractional/exponent/integer/base
  forms, separators, signed zero, signed underflow, subnormals, maximum finite
  binary32 and decimal/integer-form no-double-rounding witnesses. Dedicated
  overflow and explicit-family fixtures actually reject. The native fixture
  checks direct rounding and negative zero through executable generation.

- [x] P20-30 — Add source, Semantic IR, Sec MLIR and native tests for every
  arithmetic and comparison kind across float32/float64/plain float, including
  NaN, infinity, signed zero, remainder and `!=` unordered behavior (§86).

  Completed 2026-10-26 (Codex): `float_operator_matrix.sec` exercises all
  five arithmetic and six comparison kinds, unary signs, both NaN operand
  orders, signed zeros, infinities, overflow and gradual underflow for each
  primitive source type. Functions take runtime operands; no constant-only
  remainder acceptance. Source/IR/emitter tests verify the same matrix on
  both selected 32/64-bit plans; native execution covers f32/f64/plain float
  on the hosted 64-bit target (no 32-bit native execution claim).

  The runtime matrix exposed an undeclared fmod/fmodf linker dependency behind
  arith.remf. Fixed with bounded exact integer significand reduction and CF in
  FloatRemainder.cpp, preserving truncation remainder, subnormals and x's zero
  sign. All 584 independent edge/random differential cases execute through
  the native track without a math library. NaN payload identity is not assumed.
- [ ] P20-31 — Test properties, Abs/Min/Max/Clamp, all classification cases
  and rounding ties/signs/non-finite values (§87).
- [ ] P20-32 — Test conversions at 2^24/2^53 boundaries; signed/unsigned
  bounds; 128/256-bit values; widening/narrowing; NaN/infinity/subnormals;
  bool conversion; constant failures; and success-only IR values (§88).
- [ ] P20-33 — Test naked propagation, local/catch-all handlers, wrong error
  compatibility, canonical `ConversionError` identity and no hidden error
  union (§89).
- [ ] P20-34 — Add schema/verifier negative tests and standard-MLIR/native
  output tests for the required Arith/Math/CF operations (§§90, 93).
- [ ] P20-35 — Run focused Go tests, `go vet`, Sec MLIR verification, relevant
  MLIR lit tests, target-plan 32/64 checks and the complete repository suite;
  record exact commands and results in governance (§§85–90, 93).

## H. Acceptance, governance and handoff

- [ ] P20-36 — Reconcile all P20 acceptance criteria with the October
  corrections and the one-track architecture; record deviations rather than
  checking stale requirements literally.
- [ ] P20-37 — Update the owning lowering/types/core/compiler governance
  entries, `language-rulebook-status.md`, package status and missing-decision
  links with implemented, partial, deferred and unsupported boundaries.
- [ ] P20-38 — Produce the required implementation report with snapshot,
  predecessor status, normative changes, format/conversion representation,
  verifier/lowering evidence, tests, deviations and P21 recommendations (§94).
- [ ] P20-39 — Keep the P21 boundary explicit: decimal/decimal128 arithmetic,
  precision/scale semantics and decimal conversions start only under its
  independent authority (§95).

## Recommended execution order

1. A: establish the execution snapshot and reconcile the superseded P20 text.
2. C: complete Sema facts and conversion plans over the existing float subset.
3. D: complete Semantic IR operations and folding parity.
4. E/F: install schema-16 verification and widen the single native track.
5. G/H: run the full matrix throughout and publish only evidence-backed status.
