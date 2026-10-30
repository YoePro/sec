# Package 15 acceptance matrix — updated 2026-10-30

This is the completed P15-69 mapping of all 56 section-155 criteria, not a claim
that all criteria pass. Each criterion has a focused test or exact implementation
location. Blocked criteria remain blocked regardless of aggregate test results.

Current execution details, failures and package boundaries are in
[the implementation report](sec-mlir-dialect_package15-implementation-report.md).

| # | Section-155 criterion | Focused evidence / remaining boundary |
|---|---|---|
| 1 | baseline documents repo 152c772 + local P13/P14 or newer merged equivalent | Latest frozen source HEAD `aa59b4c18c91ed04eef69c7dd0028e41630067f5` plus the 3989-file overlay manifest recorded in the report; original descendant-of-152c772 baseline evidence is retained. Distribution binaries are excluded. |
| 2 | previous package regressions remain green | 2026-10-30: P12/P13/P14 integration passes all 12 source/target cases; adjacent schema-9/10/11 selection passes 28/28. Full isolated Go passes 43 packages; the two tool-gated Boolean execution tests pass in a separate explicitly configured run. Historical baseline failures remain in the report as superseded evidence. |
| 3 | reference/runtime-check synchronization applied | `rules/memory/borrowing.md`; MD-061, MD-076, MD-084/085 applied; no rule edits in this change. |
| 4 | Semantic IR place/reference amendment applied | `rules/compiler/semantic_ir.md`; `internal/ir/semantic/package15_test.go`. |
| 5 | schema-v11 dialect rulebook installed | `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`; consolidated dialect amendments. |
| 6 | lowering-v11 rulebook installed | `rules/mlir/lowering-versions/sec_mlir_lowering_v11.md`; consolidated lowering amendments. |
| 7 | stable PlaceRootID implemented | `internal/sema/place_root_identity_test.go`: `TestPlaceRootIdentityFollowsDeclarations` and summary/generic identity test. |
| 8 | canonical StorageIdentityID implemented/reused | `internal/sema/place_identities_test.go`: `TestRootPlacesCarryStorageIdentity`. |
| 9 | canonical InvalidationDomainID implemented/reused | `internal/sema/reference_plans_test.go`: `TestReferencePlansPublishCompleteFacts`. |
| 10 | canonical EpochDependencyID implemented/reused | `internal/sema/reference_plans_test.go`: `TestReferencePlansPublishCompleteFacts`. |
| 11 | arbitrary-precision constant place indexes implemented | `TestPackage15PlaceReferenceOperationsVerifyAndFormat`: index 18446744073709551619; native `sec.place.array_element` verifier. |
| 12 | canonical PlaceRelationship implemented | `internal/sema/place_relationship_test.go`: canonical paths and joined origins. |
| 13 | legacy PlacesOverlap delegates to canonical relationship | `TestLegacyPlacesOverlapDelegatesToRelationship`; `TestLegacyPlacesOverlapHasNoCorrectnessCallers`. |
| 14 | finite alternative origins preserved | `TestAlternativeOriginSetIsBounded`; joined-origin relationship tests. |
| 15 | ResolvedPlaceOf is read-only | `internal/sema/reference_plans.go: ResolvedPlaceOf`; `TestReferencePlanQueriesAreReadOnly`. |
| 16 | ResolvedReferencePlanOf is read-only | `internal/sema/reference_plans.go: ResolvedReferencePlanOf`; `TestReferencePlanQueriesAreReadOnly`. |
| 17 | ReferenceFacts implemented | `internal/sema/reference_plans.go`; `mlir/lib/Analysis/ReferenceFacts.h`; complete-facts test. |
| 18 | validity policy proven/dynamic implemented | `TestPackage15DynamicReferenceUseValidatesOnce`; `TestPackage15LowersProvenReferenceUses`; direct-reference CFG lit matrices. |
| 19 | 64-bit logical epoch default represented in CompilationPlan facts | `TestReferencePolicyEpochWidth`: 32/64 target plans retain default logical epoch width 64. |
| 20 | shorter widths require compile-time proof | `TestReferencePolicyEpochWidth`: short-width proof required; invalid widths rejected. |
| 21 | address stability and pinning represented (formerly "relocation class"; MD-084) | `reference-metadata-consistency.mlir`: four stability values; pinning remains separate. |
| 22 | address space represented | `TestPackage15EmitRejectsUnrepresentedReferenceFacts`; Place `ordinary`/`mmio` contract and reference address-space validation. |
| 23 | BorrowID/LifetimeID represented | `package15_test.go`; `invalid-reference-lifetime-cfg.mlir`; nonzero typed metadata checks. |
| 24 | shared ref copy classification preserved | `TestPackage15SourceMatrix`; shared copy vs holder lifetime lit tests. |
| 25 | ref mut move-only preserved | Malformed Semantic IR reference-copy test; `invalid-reference-lifetime-cfg.mlir` and `reference-lifetime-cfg.mlir`. |
| 26 | Semantic IR place operations implemented | Local/value/storage, field, element, payload and dereference Places: `internal/ir/semantic/reference_lowering.go`, `reference_use.go`, `payload_reference.go`, and source pipeline tests. MD-116 scalar static roots are covered by `TestPackage15LowersStaticPlaceRoots` and `TestPackage15EmitsStaticPlaceRoots`; addressed/hardware roots still require MD-111. Aggregate/non-literal static producers and quantity carriers remain separately gated. |
| 27 | Semantic IR ref borrow/reborrow operations implemented | `TestPackage15LowersExplicitReborrows`; `TestPackage15LowersReferenceParametersAndPassing`; schema-11 reference ops. |
| 28 | Semantic IR reference validity op implemented | `TestPackage15DynamicReferenceUseValidatesOnce`; schema-11 `sec.ref.is_valid`. |
| 29 | Semantic IR invalid-generation failure endpoint implemented | `TestPackage15EmitsDynamicReferenceGuards`; `sec.fail.reference_generation` reason verifier. |
| 30 | Semantic IR reference compare implemented | `TestPackage15LowersProvenReferenceUses`; reference comparison CFG lit matrix. |
| 31 | Semantic IR borrow-end marker implemented | `TestPackage15PayloadBorrowsEndOnEveryArmExit`; lifetime/identity CFG lit matrices. |
| 32 | !sec.place implemented | `mlir/include/sec/Dialect/Sec/SecTypes.td`; schema-11 Place/reference type lit tests. |
| 33 | !sec.ref implemented | `mlir/include/sec/Dialect/Sec/SecTypes.td`; schema-11 Place/reference type lit tests. |
| 34 | !sec.ref_mut implemented | `mlir/include/sec/Dialect/Sec/SecTypes.td`; schema-11 Place/reference type lit tests. |
| 35 | schema-v11 place/ref operations implemented | `mlir/include/sec/Dialect/Sec/SecOps.td`, `SecTypes.td`; schema-11 Place/reference fixtures and the 191-test native suite (2026-10-30). MD-114 detached register value transports are implemented; hardware roots and operations retain MD-111 and the MD-077 transport boundary. Type support alone does not complete those operations. |
| 36 | sec.fail.reference_generation implemented | `schema11-place-reference-ops.mlir`; reason enum validation; no Result failure substitution. |
| 37 | --sec-verify-places registered | `TestPackage15SourceToSchema11Pipeline` invokes `--sec-verify-places` at both target widths. |
| 38 | --sec-verify-reference-guards registered | Same pipeline invokes `--sec-verify-reference-guards`; guard CFG lit regression tests. |
| 39 | --sec-verify-borrow-semantics registered | Same pipeline invokes `--sec-verify-borrow-semantics`; transfer/lifetime CFG lit regression tests. |
| 40 | P5 address-taken storage guard enforced | `TestCheckedIntegerLoweringCoexistsWithReferences`; schema-11 address-taken storage guard fixtures. |
| 41 | P6 preserves ref/place wrappers | `mlir/test/Conversion/SecToCore/schema11-reference-scalar-layout.mlir`, P32/P64 checks with all reference verifiers. |
| 42 | P8 preserves ref/place wrappers | Same fixture P8 check: integer-core lowering retains reference wrappers. |
| 43 | P13 field borrowing works | `TestPackage15LowersFieldBorrowsWithoutAggregateCopies`; `TestPackage15FieldAndElementBorrowIntegration`. |
| 44 | P14 fixed-array element borrowing works | `TestPackage15LowersFixedArrayElementBorrows`; original evaluated bounds/index plans preserved. |
| 45 | P12 ref/ref-mut payload match binding works for supported payloads | `payload_reference_test.go`: field subjects, receiver fields, Result edges and arm-exit borrow ends. |
| 46 | returned-reference origin summaries preserved | `internal/ir/semantic/return_origin.go`, `mlir/lib/Analysis/VerifyPlaceReferences.cpp`; returned input/subfield/element/call tests, including `TestPackage15SharedReferenceCallReborrow`. MD-116 static-origin MLIR admission is implemented; generic receivers and unrepresented root-dependent returns remain P15-47 work. |
| 47 | ordinary stale direct ref is not lowered to Result | `TestPackage15EmitsDynamicReferenceGuards`; high-level reference-generation panic endpoint, no Result construction. |
| 48 | panic.invalid-reference-generation used for reachable dynamic failure | Exact reason in Semantic IR and `schema11-place-reference-ops.mlir`; effect/noPanic tests mapped separately. |
| 49 | no stable/weak handle source semantics added | `TestPackage15RejectsOutOfScopeReferencesWithoutPartialModule`; audit changed paths add no stable/weak source feature. |
| 50 | no slices added | Same test rejects slices at MaxPackage 15. Existing P16/schema 12 remains a distinct installed extension, not a P15 feature. |
| 51 | no RawPtr collapse | `internal/sema/raw_pointer_references_test.go`; P15 preserves distinct reference types and does not introduce raw-pointer lowering. |
| 52 | no physical reference layout selected | `TestPackage15ModulesHaveNoPhysicalVocabulary`; native lowering support gate rejects unresolved reference/register representation. |
| 53 | no mandatory generation manager/runtime | Same no-physical-vocabulary fixture; no runtime manager or hidden allocation added here. |
| 54 | check-sec-mlir passes | 2026-10-30: configured `check-sec-mlir -j2` and frozen-tree lit both pass 191/191. Separate schema-9/10/11 selection passes 28/28, 163 excluded. LLVM 24.0.0git (optimized/assertions), lit 24.0.0dev. |
| 55 | go test ./... passes | Satisfied 2026-10-30: full isolated `go test ./... -count=1 -json` passes 43 packages, with four without tests and zero failures; full vet passes. The two tool-gated skips pass in an explicit native-execution rerun. See the dated P15-75 revalidation in the report; historical section 43 is retained. |
| 56 | legacy paths remain operational | MD-101 supersedes the historical multiple-backend wording. The one native track and predecessor inspection/unsupported boundaries are preserved; no legacy backend is restored. |

## Scope of completion

P15-69 is complete because every criterion is mapped. P15 package acceptance is
still partial: addressed/hardware facts and operations (MD-111), remaining
P15-42/42a storage producers and P15-47 receivers/return paths are not waived
by aggregate test success. P15 §§151–155 exclude physical reference layout and
LLVM generation: later physical/LLVM consumers own the corresponding MD-061
differential matrix. That matrix is retained under `lowering.reference-model`
in governance/memory.yaml; it is not a prerequisite for closing P15.

P15-stage optimization coverage (2026-10-30):
`TestPackage15HighLevelOptimizationBoundary` verifies emitted source proofs on
32/64 plans before/after canonicalize and CSE, with unlowered, scalar and integer
stages. All three P15 verifiers run at every transformation boundary; neither
physical alias/lifetime promises nor physical reference lowering are admitted.
`TestPackage15OptimizationDependencyCFG` covers valid/malformed epoch guards,
lifetimes and transfers; invalid chunks stop before optimization and emit no
module, while valid controls still optimize and verify. These are high-level
IR checks, not executable LLVM/reference conformance.
MD-116 scalar static roots and MD-114 detached register value transports are
implemented; their former text-contract blockers must not be counted again.

The source-to-schema-11 run includes six fixtures on both target plans (12
subtests): the existing five plus the mutable-to-shared direct-reference call
fixture. It invokes all three P15 verifiers and the scalar/integer pipelines.

Producer delta (2026-10-30): scalar static CTE values now use frozen Sema facts
through the existing global schema; native static_cte and statics pass (2/2).
Copy-trivial whole-self values use the existing receiver Place and one read,
retaining receiver authority and availability. Both slices pass schema 11/12 on
32/64 plans and full Semantic IR/emitter tests. Ordinary whole-self consuming
methods are forbidden by functions.md §22/copy_move.md §18 and are not a pending
receiver mode. These increments narrow P15-42/42a/47 but do not satisfy the
remaining producer or final-acceptance gates; P15 stays partial (3 open entries).
