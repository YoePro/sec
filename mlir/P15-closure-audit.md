# P15 closure audit — 2026-10-23

**Disposition: P15 cannot close yet.** This is an evidence matrix and blocker report, not an implementation-complete claim. All primary-worktree edits in this pass stay under `mlir/`; Claude owns Go/governance/rulebook updates.

## Tested snapshot

- Base HEAD: `9c97b6bedd042212360b5e94840a8d34280c6065` (descends from `152c772`).
- Source manifest: `d32e6a83ffa038ea3ab3359758e0aa964c8a00e55879eb7204f70db8de69c40e`; 3447 files, 70 changed/untracked paths overlaid onto an isolated detached worktree.
- Modified distribution binaries under `bin/` and `vscode/bin/` were excluded from the overlay. Test tooling was copied separately: `sec-mlir-opt` and `mlir-translate`.
- Snapshot sources do not change while Claude continues working in the primary checkout. Results apply to this snapshot, not to subsequent edits.
- Native `sec-mlir-opt` SHA-256: `931b2ece85856e0644f723381a82f605e9f1614f8254a833546d65891d9c210d`.
- Toolchain: Go 1.26.8 linux/amd64; LLVM 24.0.0git optimized/assertions; lit 24.0.0dev.

## Closure blockers

1. **MD-111 / P15-42 and P15-42a:** canonical endpoint/DeviceModel fact source is undecided. Hardware binding/access cannot obtain a verified physical plan. The register table, field path and hardware-plan MLIR text contract is also undecided; Claude confirmed this in `MD077-handoff.md`. MD-110 conversion remains a separate open decision.
2. **Static reference chain is incomplete:** `internal/lowering/secmlir/place_reference.go` explicitly rejects `semantic.OpPlaceStaticRoot` as `static Place roots`; native `verifyReturnOrigins` explicitly rejects `static@N` because no schema-11 static Place root exists. Static scalar load/store support is not reference-root support.
3. **P15-47 scope reconciliation:** returned parameter/subfield/call references and MD-076 temporaries have focused tests, but static/addressed returns and the documented consuming/generic receiver residuals prevent treating the entire unchecked item as complete without explicit package ownership decisions.
4. **Governance validation fails:** `governance/index.yaml` lists `concurrency_events.yaml` and `concurrency_spawn.yaml`, neither of which exists in the snapshot or primary checkout. Do not fabricate fragments or silently remove registry entries in this MLIR-only task.
5. **Final acceptance not yet complete:** full Go results and the final package/governance/report reconciliation must be recorded; an aggregate pass cannot override the preceding implementation blockers.

## Commands and results

| Check | Result |
| --- | --- |
| Focused P15 + temporary/static Semantic IR/emitter tests with absolute `SEC_MLIR_BIN` | Passed in primary worktree before snapshot; includes source → schema 11 at 32/64 and all three verifiers. |
| `cmake --build build/sec-mlir --target check-sec-mlir -j4` | 156/156 passed. |
| `llvm-lit -sv --filter='schema(9|10|11)' build/sec-mlir/test` | 26/26 selected tests passed; 130 excluded. |
| `SEC_MLIR_BIN="$SNAPSHOT/build/sec-mlir/bin" GOCACHE=/tmp/sec-go-cache go test ./...` | Already failed in `sec/cmd/compiler`; remaining packages did not complete within this audit budget. See final update. |
| `GOCACHE=/tmp/sec-go-cache go vet ./...` in snapshot | Passed, exit 0. |
| `GOCACHE=/tmp/sec-go-cache go run ./cmd/yamlstatus check` in snapshot | Failed: 2 missing registered fragments. |
| `git diff --check` in snapshot | Passed. |

The focused test command was:

```sh
SEC_MLIR_BIN="$PWD/build/sec-mlir/bin" GOCACHE=/tmp/sec-go-cache go test ./internal/ir/semantic ./internal/lowering/secmlir -run 'TestPackage15|TestTemporaryBorrows|TestStaticStorage|TestVerifierChecksStatic' -count=1 -v
```

## Section 155 criterion-by-criterion matrix

“Test mapped” identifies exact checked-in tests; it is not a standalone claim that the entire language surface is complete. “Focused/native/pipeline pass” refers to the commands above. The entire criterion remains open wherever a blocker is identified.

| # | Normative criterion | Evidence/disposition |
| --- | --- | --- |
| 1 | baseline documents repo 152c772 + local P13/P14 or newer merged equivalent | **Recorded.** HEAD below; `git merge-base --is-ancestor 152c772 HEAD` succeeds; P13/P14 predecessor records in `governance/lowering.yaml`. |
| 2 | previous package regressions remain green | **Partial evidence.** 156/156 native regressions; full Go snapshot result below. |
| 3 | reference/runtime-check synchronization applied | **Located.** `rules/memory/borrowing.md`; MD-061, MD-076, MD-084/085 applied; no rule edits in this change. |
| 4 | Semantic IR place/reference amendment applied | **Located.** `rules/compiler/semantic_ir.md`; `internal/ir/semantic/package15_test.go`. |
| 5 | schema-v11 dialect rulebook installed | **Located.** `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`; consolidated dialect amendments. |
| 6 | lowering-v11 rulebook installed | **Located.** `rules/mlir/lowering-versions/sec_mlir_lowering_v11.md`; consolidated lowering amendments. |
| 7 | stable PlaceRootID implemented | **Test mapped.** `internal/sema/place_root_identity_test.go`: `TestPlaceRootIdentityFollowsDeclarations` and summary/generic identity test. |
| 8 | canonical StorageIdentityID implemented/reused | **Test mapped.** `internal/sema/place_identities_test.go`: `TestRootPlacesCarryStorageIdentity`. |
| 9 | canonical InvalidationDomainID implemented/reused | **Test mapped.** `internal/sema/reference_plans_test.go`: `TestReferencePlansPublishCompleteFacts`. |
| 10 | canonical EpochDependencyID implemented/reused | **Test mapped.** `internal/sema/reference_plans_test.go`: `TestReferencePlansPublishCompleteFacts`. |
| 11 | arbitrary-precision constant place indexes implemented | **Focused pass.** `TestPackage15PlaceReferenceOperationsVerifyAndFormat`: index 18446744073709551619; native `sec.place.array_element` verifier. |
| 12 | canonical PlaceRelationship implemented | **Test mapped.** `internal/sema/place_relationship_test.go`: canonical paths and joined origins. |
| 13 | legacy PlacesOverlap delegates to canonical relationship | **Test mapped.** `TestLegacyPlacesOverlapDelegatesToRelationship`; `TestLegacyPlacesOverlapHasNoCorrectnessCallers`. |
| 14 | finite alternative origins preserved | **Test mapped.** `TestAlternativeOriginSetIsBounded`; joined-origin relationship tests. |
| 15 | ResolvedPlaceOf is read-only | **Test mapped.** `internal/sema/reference_plans.go: ResolvedPlaceOf`; `TestReferencePlanQueriesAreReadOnly`. |
| 16 | ResolvedReferencePlanOf is read-only | **Test mapped.** `internal/sema/reference_plans.go: ResolvedReferencePlanOf`; `TestReferencePlanQueriesAreReadOnly`. |
| 17 | ReferenceFacts implemented | **Located/test mapped.** `internal/sema/reference_plans.go`; `mlir/lib/Analysis/ReferenceFacts.h`; complete-facts test. |
| 18 | validity policy proven/dynamic implemented | **Focused/native pass.** `TestPackage15DynamicReferenceUseValidatesOnce`; `TestPackage15LowersProvenReferenceUses`; direct-reference CFG lit matrices. |
| 19 | 64-bit logical epoch default represented in CompilationPlan facts | **Test mapped.** `TestReferencePolicyEpochWidth`: 32/64 target plans retain default logical epoch width 64. |
| 20 | shorter widths require compile-time proof | **Test mapped.** `TestReferencePolicyEpochWidth`: short-width proof required; invalid widths rejected. |
| 21 | address stability and pinning represented (formerly "relocation class"; MD-084) | **Native pass.** `reference-metadata-consistency.mlir`: four stability values; pinning remains separate. |
| 22 | address space represented | **Focused/native pass.** `TestPackage15EmitRejectsUnrepresentedReferenceFacts`; Place `ordinary`/`mmio` contract and reference address-space validation. |
| 23 | BorrowID/LifetimeID represented | **Focused/native pass.** `package15_test.go`; `invalid-reference-lifetime-cfg.mlir`; nonzero typed metadata checks. |
| 24 | shared ref copy classification preserved | **Focused/native pass.** `TestPackage15SourceMatrix`; shared copy vs holder lifetime lit tests. |
| 25 | ref mut move-only preserved | **Focused/native pass.** Malformed Semantic IR reference-copy test; `invalid-reference-lifetime-cfg.mlir` and `reference-lifetime-cfg.mlir`. |
| 26 | Semantic IR place operations implemented | **Blocked for closure.** Local/field/element/payload/deref supported; addressed roots blocked by MD-111; static Place emission still explicitly unsupported. |
| 27 | Semantic IR ref borrow/reborrow operations implemented | **Focused/native pass.** `TestPackage15LowersExplicitReborrows`; `TestPackage15LowersReferenceParametersAndPassing`; schema-11 reference ops. |
| 28 | Semantic IR reference validity op implemented | **Focused/native pass.** `TestPackage15DynamicReferenceUseValidatesOnce`; schema-11 `sec.ref.is_valid`. |
| 29 | Semantic IR invalid-generation failure endpoint implemented | **Focused/native pass.** `TestPackage15EmitsDynamicReferenceGuards`; `sec.fail.reference_generation` reason verifier. |
| 30 | Semantic IR reference compare implemented | **Focused/native pass.** `TestPackage15LowersProvenReferenceUses`; reference comparison CFG lit matrix. |
| 31 | Semantic IR borrow-end marker implemented | **Focused/native pass.** `TestPackage15PayloadBorrowsEndOnEveryArmExit`; lifetime/identity CFG lit matrices. |
| 32 | !sec.place implemented | **Native pass.** `mlir/include/sec/Dialect/Sec/SecTypes.td`; schema-11 Place/reference type lit tests. |
| 33 | !sec.ref implemented | **Native pass.** `mlir/include/sec/Dialect/Sec/SecTypes.td`; schema-11 Place/reference type lit tests. |
| 34 | !sec.ref_mut implemented | **Native pass.** `mlir/include/sec/Dialect/Sec/SecTypes.td`; schema-11 Place/reference type lit tests. |
| 35 | schema-v11 place/ref operations implemented | **Partial for closure.** Existing schema-11 local/projection/reference operations round-trip; MD-077 table/value/hardware operation text is undecided. |
| 36 | sec.fail.reference_generation implemented | **Native pass.** `schema11-place-reference-ops.mlir`; reason enum validation; no Result failure substitution. |
| 37 | --sec-verify-places registered | **Pipeline pass.** `TestPackage15SourceToSchema11Pipeline` invokes `--sec-verify-places` at both target widths. |
| 38 | --sec-verify-reference-guards registered | **Pipeline pass.** Same pipeline invokes `--sec-verify-reference-guards`; guard CFG lit regression tests. |
| 39 | --sec-verify-borrow-semantics registered | **Pipeline pass.** Same pipeline invokes `--sec-verify-borrow-semantics`; transfer/lifetime CFG lit regression tests. |
| 40 | P5 address-taken storage guard enforced | **Focused/native pass.** `TestCheckedIntegerLoweringCoexistsWithReferences`; schema-11 address-taken storage guard fixtures. |
| 41 | P6 preserves ref/place wrappers | **Native pass.** `mlir/test/Conversion/SecToCore/schema11-reference-scalar-layout.mlir`, P32/P64 checks with all reference verifiers. |
| 42 | P8 preserves ref/place wrappers | **Native pass.** Same fixture P8 check: integer-core lowering retains reference wrappers. |
| 43 | P13 field borrowing works | **Focused pass.** `TestPackage15LowersFieldBorrowsWithoutAggregateCopies`; `TestPackage15FieldAndElementBorrowIntegration`. |
| 44 | P14 fixed-array element borrowing works | **Focused pass.** `TestPackage15LowersFixedArrayElementBorrows`; original evaluated bounds/index plans preserved. |
| 45 | P12 ref/ref-mut payload match binding works for supported payloads | **Focused pass.** `payload_reference_test.go`: field subjects, receiver fields, Result edges and arm-exit borrow ends. |
| 46 | returned-reference origin summaries preserved | **Partial for closure.** Parameter/subfield/element/call summaries tested; native `verifyReturnOrigins` rejects static roots and emitter rejects static Places. |
| 47 | ordinary stale direct ref is not lowered to Result | **Focused/native pass.** `TestPackage15EmitsDynamicReferenceGuards`; high-level reference-generation panic endpoint, no Result construction. |
| 48 | panic.invalid-reference-generation used for reachable dynamic failure | **Focused/native pass.** Exact reason in Semantic IR and `schema11-place-reference-ops.mlir`; effect/noPanic tests mapped separately. |
| 49 | no stable/weak handle source semantics added | **Boundary evidence.** `TestPackage15RejectsOutOfScopeReferencesWithoutPartialModule`; audit changed paths add no stable/weak source feature. |
| 50 | no slices added | **Boundary evidence.** Same test rejects slices at MaxPackage 15. Existing P16/schema 12 remains a distinct installed extension, not a P15 feature. |
| 51 | no RawPtr collapse | **Boundary evidence.** `internal/sema/raw_pointer_references_test.go`; P15 preserves distinct reference types and does not introduce raw-pointer lowering. |
| 52 | no physical reference layout selected | **Focused pass.** `TestPackage15ModulesHaveNoPhysicalVocabulary`; native lowering support gate rejects unresolved reference/register representation. |
| 53 | no mandatory generation manager/runtime | **Focused pass.** Same no-physical-vocabulary fixture; no runtime manager or hidden allocation added here. |
| 54 | check-sec-mlir passes | **Passed.** 156/156; separate schema 9/10/11 selection 26/26; exact versions below. |
| 55 | go test ./... passes | **Failed/incomplete.** Snapshot has two failing compiler IO tests; full-suite completion is not claimed. |
| 56 | legacy paths remain operational | **Superseded wording.** MD-101/codex.md mandates one compilation track. Preserve adjacent regressions; do not restore separate legacy backends. |

## TODO handoff

| TODO | Disposition |
| --- | --- |
| P15-42 / 42a | Blocked/partial for reasons 1–2 above. |
| P15-47 | Partial; reconcile the actual remaining receiver/static/addressed surface with package owners. MD-076 temporary support is already tested. |
| P15-69 | All 56 section-155 criteria mapped above; ready for owner review/integration. No TODO checkbox changed here. |
| P15-70 | Isolated source snapshot, vet and whitespace evidence recorded; YAML fails and full tests remain to be recorded. |
| P15-71 | Full native and adjacent-schema evidence recorded, including exact tool versions. |
| P15-72 | Focused source-to-schema-11 test covers both widths and all three verifiers; snapshot-only rerun result to be recorded below. |
| P15-73 | Claude owns governance; status must remain partial until blockers close. |
| P15-74 | Section-156 report index follows; unresolved boundaries are explicitly reported rather than marked implemented. |
| P15-75 | Cannot be complete while blockers, YAML errors or incomplete final test runs remain. |

## Section 156 implementation-report index

This index preserves all 46 report topics in order. It is a review artifact for the coordinating owner, not a claim that all package acceptance criteria are satisfied.

| # | Required topic | Evidence / boundary |
| --- | --- | --- |
| 1 | repository HEAD implemented against | Snapshot HEAD and manifest above. |
| 2 | local/merged P13 and P14 status | Current predecessor records in governance/lowering.yaml; adjacent schema 9/10 native selection passes. |
| 3 | previous package status | Native regressions pass; full Go snapshot result below. |
| 4 | reference/runtime-check rule synchronization | Installed canonical rulebooks and MD-061/076/084/085 corrections; matrix rows 3–6. |
| 5 | files added | See MD077-handoff.md fixture/helper list; this audit is new under mlir/. |
| 6 | files modified | Type/operation/dialect/reference analysis files described in MD077-handoff.md; primary Go edits belong to Claude. |
| 7 | PlaceRootID migration | Matrix row 7. |
| 8 | arbitrary-precision place-index migration | Matrix row 11; exact index exceeds uint64. |
| 9 | PlaceRelationship implementation | Matrix row 12. |
| 10 | legacy PlacesOverlap compatibility | Matrix row 13. |
| 11 | StorageIdentity/InvalidationDomain/EpochDependency representation | Matrix rows 8–10. |
| 12 | ResolvedPlaceOf API | Matrix row 15. |
| 13 | ResolvedReferencePlanOf API | Matrix row 16. |
| 14 | ReferenceFacts representation | Matrix row 17. |
| 15 | validity-policy implementation | Matrix row 18; transfer/guard CFG lit matrices. |
| 16 | CompilationPlan epoch-width facts | Matrix rows 19–20. |
| 17 | relocation/address-space facts | Matrix rows 21–22; MD-084 replaces historical relocation_class vocabulary. |
| 18 | BorrowID/LifetimeID implementation | Matrix row 23. |
| 19 | Semantic IR place operations | Matrix row 26; static/addressed root blockers remain. |
| 20 | Semantic IR reference operations | Matrix rows 27–31. |
| 21 | shared-copy/mutable-move implementation | Matrix rows 24–25; authority-copy guards and holder lifetime tests. |
| 22 | reborrow implementation | Explicit and receiver reborrow tests; source_borrow identities verified through joins. |
| 23 | validity guard/failure CFG | Same-SSA guard/failure checks and incoming dynamic facts through CFG joins/loops. |
| 24 | reference equality | Reference comparison requires the applicable dynamic guard. |
| 25 | end-borrow representation | End identity and forwarded-holder liveness checks; ambiguous alias liveness is not full path-sensitive analysis. |
| 26 | schema-v11 types/ops | Matrix rows 32–36; MD-077 types only, no register/hardware operation admission. |
| 27 | place verifier | Registered and exercised in 32/64 source pipeline; Place escape/authority/bounds/variant checks. |
| 28 | reference guard verifier | Registered and exercised; dynamic uses through CFG joins/loops require same-SSA validation. |
| 29 | borrow verifier | Registered and exercised; transfers, derivation, lifetimes, returned origins; unknown facts remain an explicit proof boundary. |
| 30 | P5 address-taken storage changes | Address-taken marker true and P5 guard; no early scalarization of reference-dependent storage. |
| 31 | P6/P8 compatibility | schema11-reference-scalar-layout.mlir; P32/P64/P8 checks. |
| 32 | P13 field-borrow integration | Matrix row 43. |
| 33 | P14 array-element-borrow integration | Matrix row 44. |
| 34 | P12 match-borrow integration | Matrix row 45. |
| 35 | returned-reference integration | Matrix row 46; static origins explicitly unsupported. |
| 36 | proven-reference tests | Source/emitter proven-reference fixtures and no-Panic tests mapped. |
| 37 | dynamic-epoch tests | Dynamic-use source/white-box fixtures and native transfer/guard matrices. |
| 38 | 32/64-bit profile tests | TestPackage15SourceToSchema11Pipeline and TestPackage15PlaceReferenceFactsAreDeterministic. |
| 39 | unsupported handle/slice/raw-conversion tests | TestPackage15RejectsOutOfScopeReferencesWithoutPartialModule; RawPtr remains a separate source feature. |
| 40 | CMake commands | Commands above. |
| 41 | exact LLVM/MLIR version | LLVM 24.0.0git, optimized with assertions; lit 24.0.0dev. |
| 42 | check-sec-mlir result | 156/156 native; 26/26 adjacent selected. |
| 43 | go test ./... result | Snapshot reports two compiler IO test failures; see baseline comparison below. |
| 44 | end-to-end source -> schema-v11 results | Focused test passes both target widths; snapshot-only focused command result below. |
| 45 | deviations | No invented hardware serialization; MD-111/MD-110 and static-root gaps recorded; no Go/governance edits; MD-101 supersedes legacy-backend wording. |
| 46 | recommendations for Package 16 | P16 must retain P15 reference facts/verifiers. P17 owns nontrivial lifecycle/ownership work. Hardware admission waits for the shared normative contract. |

## Final validation update

- Snapshot-focused P15/temporary/static/integer-reference tests passed: **44 top-level tests and 64 nested cases**, no skips. The source-to-schema-11 pipeline alone ran **10 cases** (five fixtures at 32/64 bits), invoking all three P15 verifiers.
- Snapshot-focused Sema identity/relationship/read-only-plan/epoch tests passed: **17 top-level tests**.
- Full snapshot `go test ./...` reported failures in `TestCompilerLoadsIOReadFileIntoAPI` and `TestCompilerLoadsIOWriteSeekAndDirectoryAPIs`. The remaining run was interrupted/terminated within the ten-minute audit budget (controller exit 143); broad-suite completion is not claimed.
- A second, clean detached worktree at the same HEAD reproduced **both IO test failures** using `go test ./cmd/compiler -run '^TestCompilerLoadsIO(ReadFileIntoAPI|WriteSeekAndDirectoryAPIs)$' -count=1 -v`. Baseline errors include unmapped RangeError propagation in stdlib IO and removed `string.FromByteArray`; therefore these test failures are not attributable solely to the new MLIR changes. The changed snapshot additionally reports unresolved `linux` references; that difference needs a Go-side investigation.
- `go vet ./...` passed in the changed snapshot. Governance validation failed with the two missing fragment files listed above. Whitespace checks passed.
- P15 remains **partial**. This audit supplies evidence for P15-69/71/72 and the P15-74 report structure, but does not change checklist ownership or claim P15-70/75 passed.
