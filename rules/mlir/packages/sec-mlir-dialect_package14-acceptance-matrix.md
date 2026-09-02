# SEC MLIR Package 14 acceptance matrix

This matrix maps every acceptance criterion in
`sec-mlir-dialect_package14.md` section 107 to focused executable evidence or
an exact implementation/rule location. It was audited on 2026-09-02 against
repository HEAD `475add8` plus the pending P14-48 and P14-70 through P14-77
working-tree diff.

`Covered` means that the individual criterion has identifiable evidence. P14-48
and P14-72 through P14-74 completed the remaining focused and aggregate gates;
P14-75 therefore marks the implementation `implemented`. P14-76 and P14-77
subsequently completed the mandatory report and final clean validation.

## Baseline and synchronized rules

| # | Section-107 criterion | Focused evidence or exact location | Status |
|---:|---|---|---|
| 1 | Implementation baseline documents repo `152c772` plus local P13 or newer merged equivalent | `package14-todo.md`, “Inventory snapshot”, records `069c111b...` with completed P13; `implementation-status.yaml`, `lowering.sec-mlir-package14.repository_head` | Covered |
| 2 | Previous package regressions remain green | `implementation-status.yaml`, Package 14 verification entries for the isolated P14-72 Go/vet run, full P14-73 MLIR run, and complete P14-77 final rerun | Covered |
| 3 | Wide builtin invariant remains | `internal/ir/semantic/semantic_test.go::TestBuiltinTypeCoversWideScalars`; `TestPackage3BuildsWideScalarConstantsStorageAndCalls` | Covered |
| 4 | Stale no-array-spread wording synchronized | `rules/collections/collections.md`, fixed-array literal spread rules; `rules/declarations/spread.md`, array spread rules | Covered |
| 5 | `IndexError.OutOfBounds` is the canonical fixed-index error | `rules/errors/runtime_checks.md`; `rules/library/core-library.md`; `internal/ir/semantic/package14_test.go::TestPackage14FallibleBoundsBuildsIndexError` | Covered |
| 6 | Semantic IR fixed-array amendment applied | `rules/mlir/semantic-ir/sec_semantic_ir_fixed_array_v1.md`; executable model in `internal/ir/semantic/model.go` and verifier in `internal/ir/semantic/verifier.go` | Covered |
| 7 | Schema-v10 dialect rulebook installed | `rules/mlir/dialect-versions/sec_mlir_dialect_v10.md` | Covered |
| 8 | Lowering-v10 rulebook installed | `rules/mlir/lowering-versions/sec_mlir_lowering_v10.md` | Covered |

## Canonical shape and compact Sema plans

| # | Section-107 criterion | Focused evidence or exact location | Status |
|---:|---|---|---|
| 9 | Canonical array shape no longer relies on `-1` sentinel | `internal/sema/types.go::ArrayShapeKind`, `NewFixedArrayType`, `NewDynamicArrayType`; `internal/sema/array_shape_test.go::TestCanonicalArrayShapeAndTargetLengthMatrix` | Covered |
| 10 | Canonical fixed length uses arbitrary precision | `internal/sema/types.go::exactFixedArrayLength`; `TestCanonicalArrayLengthRejectsOneAboveUint64WithoutHostConversion` | Covered |
| 11 | Fixed-length type identity uses the exact value | `internal/sema/types.go::sameArrayShape`; `TestCanonicalArrayIdentityUsesExactDecimalLength` | Covered |
| 12 | Target-`uint` validation is plan-specific | `internal/sema/types.go::ValidateArrayTypeForScalarPlan`; `TestCanonicalArrayShapeAndTargetLengthMatrix` | Covered |
| 13 | A semantic length above `int64` survives on a valid 64-bit plan | `internal/sema/array_shape_test.go::TestCanonicalArrayShapeAndTargetLengthMatrix`; `internal/lowering/secmlir/emitter_test.go::TestEmitPackage14ArrayLengthUsesTargetUintBoundary` | Covered |
| 14 | `ResolvedArrayLiteralPlan` implemented | `internal/sema/semantic_facts.go::ResolvedArrayLiteralPlan`; `TestResolvedArrayLiteralPlanFactCopiesCompactExactEntries` | Covered |
| 15 | Literal plan does not expand spread O(N) | `internal/sema/semantic_facts.go::ResolvedArrayLiteralPlanOf`; `TestResolvedArrayLiteralPlanQueryIsReadOnlyAndCompact` | Covered |
| 16 | Array-literal source order preserved | `internal/sema/analyzer.go::resolveArrayLiteralPlan`; `TestArrayLiteralAnalysisPopulatesAuthoritativePlans` | Covered |
| 17 | Multiple fixed-array spreads preserved | `internal/sema/semantic_facts_test.go::TestArrayLiteralPlansPreserveMultipleAndHugeSpreads` | Covered |

## Semantic IR value and bounds layer

| # | Section-107 criterion | Focused evidence or exact location | Status |
|---:|---|---|---|
| 18 | `ArrayConstructOp` implemented | `internal/ir/semantic/model.go::OpArrayConstruct`; `TestPackage14ArrayConstructDefaultAndLengthVerifyAndFormat` | Covered |
| 19 | `ArrayDefaultOp` implemented | `internal/ir/semantic/model.go::OpArrayDefault`; `TestPackage14BuildsCompactFixedArrayDefaultMatrix` | Covered |
| 20 | Zero-length default constructs no element | `internal/sema/defaults_test.go::TestPackage14CompactFixedArrayDefaultMatrix`; `internal/ir/semantic/package14_test.go::TestPackage14BuildsCompactFixedArrayDefaultMatrix` | Covered |
| 21 | No readable partial or `undef` array | `internal/ir/semantic/verifier.go::verifyArrayConstruct`; `internal/lowering/secmlir/emitter_test.go::TestPackage14SuccessfulModuleContainsNoPhysicalArrayShortcut` | Covered |
| 22 | `ArrayLengthOp` implemented | `internal/ir/semantic/model.go::OpArrayLength`; `TestPackage14ArrayConstructDefaultAndLengthVerifyAndFormat` | Covered |
| 23 | `ResolvedArrayIndexPlan` implemented | `internal/sema/semantic_facts.go::ResolvedArrayIndexPlan`; `TestResolvedFixedArrayIndexPlans` | Covered |
| 24 | Constant bounds use arbitrary precision | `internal/sema/semantic_facts_test.go::TestPackage14Section95ConstantIndexMatrix`; `TestFixedArrayIndexPlansRejectExactWideBounds`; `internal/ir/semantic/package14_test.go::TestPackage14RejectsCompileTimeInvalidIndexWithoutPartialIR` | Covered |
| 25 | Proven-safe/runtime-check classification implemented | `internal/sema/analyzer.go::recordFixedArrayIndexPlan`; `TestResolvedFixedArrayIndexRefinementProofs` | Covered |
| 26 | `ArrayIndexInBoundsOp` implemented | `internal/ir/semantic/model.go::OpArrayIndexInBounds`; `TestPackage14ArrayIndexExtractReplaceVerifyAndFormat` | Covered |
| 27 | `ArrayExtractOp` implemented | `internal/ir/semantic/model.go::OpArrayExtract`; `TestPackage14ArrayIndexVerifierMatrix` | Covered |
| 28 | `ArrayReplaceOp` implemented for the trivial subset | `internal/ir/semantic/model.go::OpArrayReplace`; `TestPackage14BuildsTransactionalIndexedLocalReplacement` | Covered |
| 29 | `BoundsFailureOp` implemented | `internal/ir/semantic/model.go::OpBoundsFailure`; `internal/ir/semantic/verifier.go::validArrayBoundsFailureBlock` | Covered |
| 30 | Ordinary bounds path is panic-capable | `internal/ir/semantic/package14_test.go::TestPackage14Section97RuntimeBoundaryMatrix`; `TestPackage14BuildsOrdinaryArrayIndexControlFlowAndOrder`; `internal/sema/effect_analysis_test.go::TestFixedArrayBoundsEffectsDistinguishRuntimeAndProvenIndexes` | Covered |
| 31 | Fallible bounds path produces `IndexError.OutOfBounds` | `internal/ir/semantic/package14_test.go::TestPackage14FallibleBoundsBuildsIndexError` | Covered |
| 32 | Naked/local `try` integration works | `internal/ir/semantic/package14_test.go::TestPackage14BuildsFallibleBoundsPropagationAndLocalHandler`, including `int32`, `int128`, exact and `Err(_)` handlers | Covered |
| 33 | Bounds effect analysis integrated | `internal/sema/analyzer.go::recordArrayIndexEffect`; `TestFallibleFixedArrayBoundsRetainOnlyOperandEffects` | Covered |
| 34 | `@noPanic` proven/fallible cases work | `internal/sema/effect_analysis_test.go::TestNoPanicFixedArrayIndexGuarantees` | Covered |
| 35 | Mutable trivial fixed-array storage works | `internal/ir/semantic/package14_test.go::TestPackage14BuildsTrivialMutableFixedArrayStorage` | Covered |
| 36 | Nested trivial replacement works | `internal/ir/semantic/package14_test.go::TestPackage14BuildsNestedArrayStructReplacementMatrix` | Covered |
| 37 | P5 leaves array storage high-level | `internal/lowering/secmlir/emitter_test.go::TestPackage14SuccessfulModuleContainsNoPhysicalArrayShortcut`; P5 storage eligibility in `internal/ir/semantic/builder.go` | Covered |

## Schema 10 and compatibility

| # | Section-107 criterion | Focused evidence or exact location | Status |
|---:|---|---|---|
| 38 | `!sec.array` implemented | `mlir/include/sec/Dialect/Sec/SecTypes.td::Sec_ArrayType`; `mlir/test/Dialect/Sec/schema10-array-type.mlir` and its invalid companion | Covered |
| 39 | `sec.array.construct` implemented | `mlir/include/sec/Dialect/Sec/SecOps.td::Sec_ArrayConstructOp`; `schema10-array-construct.mlir` and its invalid companion | Covered |
| 40 | `sec.array.default` implemented | `mlir/include/sec/Dialect/Sec/SecOps.td::Sec_ArrayDefaultOp`; `mlir/test/Dialect/Sec/schema10-array-ops.mlir` | Covered |
| 41 | `sec.array.len` implemented | `mlir/include/sec/Dialect/Sec/SecOps.td::Sec_ArrayLenOp`; `schema10-array-ops.mlir` and its invalid companion | Covered |
| 42 | `sec.array.index_in_bounds` implemented | `mlir/include/sec/Dialect/Sec/SecOps.td::Sec_ArrayIndexInBoundsOp`; `schema10-array-ops.mlir` | Covered |
| 43 | `sec.array.extract` implemented | `mlir/include/sec/Dialect/Sec/SecOps.td::Sec_ArrayExtractOp`; `schema10-array-ops.mlir` and `schema10-array-index-guards.mlir` | Covered |
| 44 | `sec.array.replace` implemented | `mlir/include/sec/Dialect/Sec/SecOps.td::Sec_ArrayReplaceOp`; `schema10-array-ops.mlir` and `schema10-array-index-guards.mlir` | Covered |
| 45 | `sec.fail.bounds` implemented | `mlir/include/sec/Dialect/Sec/SecOps.td::Sec_FailBoundsOp`; `mlir/test/Dialect/Sec/schema10-array-ops.mlir` | Covered |
| 46 | `--sec-verify-array-index-guards` registered | `mlir/include/sec/Analysis/Passes.td::SecVerifyArrayIndexGuards`; positive and invalid `schema10-array-index-guards.mlir` suites | Covered |
| 47 | P6 preserves wrapper and resolves nested target scalars | `mlir/test/Conversion/SecToCore/schema10-array-scalar-layout-32.mlir` and `schema10-array-scalar-layout-64.mlir` | Covered |
| 48 | P8 does not recursively normalize the array wrapper | `mlir/test/Conversion/SecIntegerToArith/schema10-array-boundary.mlir` | Covered |
| 49 | P13 struct nesting integration passes | `internal/ir/semantic/package14_test.go::TestPackage14BuildsNestedArrayStructReplacementMatrix`; `TestPackage14BuildsCompactFixedArrayDefaultMatrix` | Covered |

## Boundaries and repository gates

| # | Section-107 criterion | Focused evidence or exact location | Status |
|---:|---|---|---|
| 50 | Dynamic array/slice semantics do not accidentally use `!sec.array` | `internal/ir/semantic/package14_test.go::TestPackage14RejectsDeferredArraySourcePathsWithoutPartialIR` | Covered |
| 51 | Non-trivial ownership paths reject explicitly | `TestPackage14RejectsDeferredArraySpreadActions`; `TestPackage14RejectsNonTrivialFixedArrayStorageWithoutPartialModule` | Covered |
| 52 | No physical array layout selected | `internal/lowering/secmlir/emitter_test.go::TestPackage14RejectsPhysicalFixedArrayLayoutRequest`; `TestPackage14SuccessfulModuleContainsNoPhysicalArrayShortcut` | Covered |
| 53 | No LLVM dialect generated | `internal/lowering/secmlir/emitter_test.go::TestEmitPackage14SourceModuleVerifiesOn32And64BitPlans`, rerun for P14-74 with an absolute tool path; `TestPackage14SuccessfulModuleContainsNoPhysicalArrayShortcut` | Covered |
| 54 | `check-sec-mlir` passes | `implementation-status.yaml`, Package 14 verification: P14-73 passed 91/91 plus 17/17 adjacent schemas; P14-77 passed the final full suite 91/91 | Covered |
| 55 | `go test ./...` passes | `implementation-status.yaml`, Package 14 verification: isolated P14-72 passed, and P14-77 passed with absolute `SEC_MLIR_BIN` | Covered |
| 56 | Legacy paths remain operational | `internal/codegen/mlir/generator_test.go::TestGenerateFixedArrays`, `TestGenerateFixedArrayFillInlineAndEvaluatesValueOnce`, and `TestLegacyGeneratorRejectsAboveInt64ArrayLayoutExplicitly`; schema-9 regressions under `mlir/test/Conversion/SecToCore/` | Covered |

## Audit result

All 56 section-107 criteria have direct, reviewable evidence. Aggregate green
commands are used only for criteria 2, 54, and 55, where the acceptance text
itself asks for aggregate regression status; they are not used as substitutes
for the 53 feature-specific mappings. P14-75 sets Package 14 to `implemented`,
and P14-76/P14-77 complete the mandatory narrative report and clean final
handoff validation.
