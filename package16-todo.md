# SEC-MLIR Package 16 — implementation TODO

Package: `SEC-MLIR-P16` — Slice Semantic Value Representation  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package16.md`  
Package contract: `rules/mlir/packages/sec-mlir-dialect_package16.yaml`  
Normative inputs:

- `rules/mlir/normative-sync/sec_slice_sync_v1.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v12.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v12.md`

This is an implementation plan, not a language correction or completion report.
It follows the structure of `package14-todo.md` and `package15-todo.md`. The
inventory below records the 2026-10-14 checkout; dated completion notes record
subsequent progress. The package's `152c772` / 2026-08-09 snapshot
is historical context, not the execution baseline.

## Inventory snapshot — 2026-10-14

Audited HEAD:

```text
08d8b8fbf5ca87fe3157cec9d36feed2cc2d7cbd
```

The working tree had 132 tracked/untracked change entries before this file was
created. The inventory includes those current changes; HEAD alone cannot
reproduce it. Do not use the old P15 isolated worktree as if it contained today's
implementation. Record a new exact snapshot containing the intended predecessor
changes before final package acceptance.

Source inspection and focused tests establish the following state:

| Area | Current implementation | P16 consequence |
|---|---|---|
| P13/P14 | Struct and fixed-array Semantic IR and schema-9/10 emission exist; exact fixed-array lengths and resolved index plans exist. | Reuse their identities, Places and proof metadata. |
| P15 Sema | Canonical Place/root/storage/domain/borrow/lifetime identities, arbitrary-precision element indexes, detached reference plans, bounded origins and validity policies exist. | Extend this model; do not recreate a slice-specific borrow checker or generation system. |
| P15 Semantic IR | Place/reference types, operations and verification exist. Local/parameter/field/array borrows, reference uses, passing, instance receivers and selected union/Option payload borrows are implemented. | Build slices on these actual interfaces; P15 is still partial. |
| P15 gaps | Static/addressed roots, returned-reference lowering/origin summaries, explicit holder reborrow (MD-061), and some payload/receiver paths remain unsupported. | Track prerequisites separately and retain explicit unsupported boundaries. |
| Go emitter | `internal/lowering/secmlir/emitter.go` emitted schema **10** at this snapshot; since P15-50/51 (2026-10-16) it emits schema **11**. | Schema 11 is now the baseline; schema 12 (P16-38, P16-44) is the next version in the chain 12–15 (P16–P19). |
| C++ dialect | `mlir/lib/Dialect/Sec/SecDialect.cpp` accepted schemas **1–10** at this snapshot; it now accepts **1–12** after P16-38 registration; P15 place/reference types and P16-39 slice types are available, while P16 slice operations remain separate work. Early schema-11 admissions are recorded in `sec_mlir_dialect_v11.md` § 41. | Complete the remaining P15 verifier bridge before P16 end-to-end acceptance. |
| Builder selection | `BuildOptions.MaxPackage` defaults to **14**; P15 is selected explicitly. Slice expressions and array-to-slice creation are rejected. | Add an explicit P16 path before promoting the default. |
| Existing slices | Sema uses `ReferenceType` over `SliceType`; slice origins, overlap checks, nested static composition and local-return rejection already exist. | Migrate producers and consumers rather than replace working source semantics wholesale. |
| Slice precision | `PlaceProjection.SliceStart` / `SliceEnd` and nested slice composition still use `int64`; element indexes already use `big.Int`. | The range migration remains genuine P16 work. |
| P16 APIs/IR | No `SliceRangeID`, `ResolvedSliceCreationPlanOf`, `ResolvedSliceIndexPlanOf`, `SliceFacts`, or P16 slice operations were found in implementation. | These are new work, not completed checklist items. |
| Rule synchronization | At the inventory snapshot the inputs were not installed. P16-05 (2026-10-19) installs the complete slice amendment and schema/lowering v12 in the canonical rulebooks with later accepted corrections. | Source-rule sync (P16-03/P16-04), open metadata decisions and implementation acceptance remain separate gates. |

Primary implementation anchors:

```text
internal/sema/place.go
internal/sema/place_identities.go
internal/sema/reference_plans.go
internal/sema/reference_origin_join.go
internal/sema/analyzer_inference.go
internal/sema/analyzer_reference_summaries.go
internal/ir/semantic/place_reference.go
internal/ir/semantic/reference_lowering.go
internal/ir/semantic/reference_use.go
internal/ir/semantic/reference_parameters_test.go
internal/ir/semantic/payload_reference.go
internal/lowering/secmlir/emitter.go
mlir/include/sec/Dialect/Sec/SecTypes.td
mlir/include/sec/Dialect/Sec/SecOps.td
mlir/include/sec/Analysis/Passes.td
mlir/lib/Dialect/Sec/SecDialect.cpp
```

Existing regression anchors include `TestPackage15*`,
`TestStaticSlicePlaceOverlap`, `TestPlaceBorrowsPermitDisjointStaticSlices`,
`TestSliceAliasSubrangesComposeAndRemainDisjoint`,
`TestDynamicSliceRangesRemainConservativelyOverlapping`,
`TestEmptyStaticSliceDoesNotBorrowElements`, and
`TestReturnSliceIntoLocalArrayIsRejected`.

### Evidence collected while preparing this plan

Machine-specific locations are intentionally omitted. For reproduction, set
`SEC_MLIR_BIN` locally to the absolute tool directory, for example:

```sh
export SEC_MLIR_BIN="$(pwd)/build/sec-mlir/bin"
```

Store environment-variable names in reports rather than their expanded paths.

Environment inspected:

```text
Go:           go1.26.0 linux/amd64
sec-mlir-opt: LLVM 24.0.0git, optimized build with assertions
llvm-lit:     lit 24.0.0dev
tool path:    ${SEC_MLIR_BIN}
```

These exact focused commands passed on 2026-10-14 in the dirty working tree:

```sh
go test ./internal/sema ./internal/ir/semantic -run 'TestPackage15|TestStaticSlicePlaceOverlap|TestWideConstantIndexPlacesRemainDistinct|TestReturnSliceIntoLocalArrayIsRejected|TestCopyClassificationPrimitiveReferenceAndAggregates' -count=1
SEC_MLIR_BIN="${SEC_MLIR_BIN}" go test ./internal/lowering/secmlir -run 'TestEmitPackage13StructsEndToEnd|TestEmitPackage14ArraysEndToEnd|TestEmitPackage14SourceModuleVerifiesOn32And64BitPlans' -count=1
```

This evidence covers existing predecessor behavior and source-level controls.
It does not establish schema-11/12 support, complete P15 acceptance, or a green
repository-wide baseline. Full Go/vet/lit gates must be run against the execution
snapshot; old results in the P15 TODO are not fresh P16 evidence.

## Execution baseline refresh — 2026-10-19 (P16-01/P16-02)

An exact complete file snapshot of the intended current tracked changes and
all non-ignored untracked files is retained locally, independently of the
working tree and normal staging index:

```text
HEAD: db7dba47ac750e759d87098dc6b9847e1a9fe39b
Snapshot tree: cd4d9dfdfe1988306bff5afce7d1ea5222d7ef22
Archive SHA-256: 769d4c6c132acb423068d935ed4be8dd376a25ba5094289367c45f95ea7e9b73
Local artifacts: .git/p16-baseline-20261019/
Archive: source.tar
Inventory/toolchain record: manifest.json
Isolated extraction: temporary directory outside the project, cleaned after verification
Go: go1.26.8 linux/amd64
LLVM: 24.0.0git, optimized with assertions
llvm-lit: 24.0.0dev
Tool directory: ${SEC_MLIR_BIN} (resolved locally, never recorded expanded)
```

The archive contains 38 changed tracked files and 40 untracked files in their
complete tree context, including predecessor and current P16 source changes.
Its digest and representative tracked/untracked contents were checked after
extraction; ignored `codex.md` is excluded and the real staging-index digest
was unchanged. The full inventory stays in the local manifest. These completion
notes and subsequent ledger edits postdate the captured snapshot; they are not
part of its identity. Capture is not final package acceptance.

Fresh current-worktree gates (source code unchanged during the run):

| Command | Result |
|---|---|
| `SEC_MLIR_BIN="$PWD/build/sec-mlir/bin" go test ./... -count=1` | Completed: 39 packages passed, 4 failed with the 7 tests listed below; no green repository-wide claim. LSP passed (390.643s). |
| `go vet ./...` | Passed. |
| `cmake --build build/sec-mlir --target check-sec-mlir -j2` | Passed: 125/125. |
| `SEC_MLIR_BIN="$PWD/build/sec-mlir/bin" go test ./internal/lowering/secmlir ./internal/ir/semantic -run 'TestEmitPackage1[345]\|TestPackage1[345]\|TestSlice\|TestEmitSchema' -count=1` | Passed, including predecessor source-to-schema and explicit 32/64-plan tests. |

The Markdown command above uses `\|` only to escape table delimiters; use
ordinary `|` characters in the test regular expression when invoking it.

Current failures and their owners, recorded before further slice source work:

| Package / failed tests | Observed cause | Owner |
|---|---|---|
| `cmd/compiler`: `TestCompilerLoadsIOReadFileIntoAPI`, `TestCompilerLoadsIOWriteSeekAndDirectoryAPIs` | Imported IO sources propagate RangeError through IOError/IOErrorLegacy return channels and still call removed core string.FromByteArray. | `stdlib.io-root` and core materialization migration (`compiler.compiler-known-surfaces`); not an assertion that compiler acceptance should be weakened. |
| `cmd/unicodegen`: `TestGeneratedUnicodeSourceIsCurrent` | Generated unicode.sec is stale. | `stdlib.unicode-tables`. |
| `internal/sema/collectionshape`: `TestArrayLiteralLengthContractsUseExactCompactLength`, `TestArrayLiteralUniqueContractRejectsProvenDirectDuplicates`, `TestUniqueContractSemanticProofs` | Fixtures use nonempty owning dynamic-array literals, rejected under the current source rule before the expected length/unique diagnostics. | `frontend.type-contracts-v2` with `frontend.dynamic-array-semantic-identity` for fixture/contract alignment. |
| `internal/sema/temporal`: `TestRepresentedMutexCallsDoNotInventUnitOverloads` | Its legacy mutex.lock fixture no longer resolves. | `frontend.temporal-duration` and `concurrency.mutex-v2` API migration. |

The full raw logs remain local temporary artifacts; no machine-specific paths
are copied into this report. These are baseline failures observed before the
next source change, not failures caused by this documentation-only sync.

Isolated snapshot verification passed independently:

```sh
P16_BASELINE="$(mktemp -d)"
tar -xf .git/p16-baseline-20261019/source.tar -C "$P16_BASELINE"
SEC_MLIR_BIN="$PWD/build/sec-mlir/bin" go test -C "$P16_BASELINE" ./internal/ir/semantic ./internal/lowering/secmlir -run 'TestSlice|TestEmitSchema|TestEmitPackage14SourceModuleVerifiesOn32And64BitPlans' -count=1
```

The archive and inventory are local `.git` artifacts. Extract outside the
project tree: a retained nested checkout can trigger nested-project discovery.
The initial in-project extraction was removed and the focused snapshot tests
were rerun successfully outside the project on 2026-10-19.
Final package acceptance must capture and test its actual final tree in an
isolated snapshot; neither this refresh nor earlier dirty-tree evidence claims
that acceptance.

## Current rule reconciliation and dependencies

1. **Property spelling (P16-03/P16-04, synchronized 2026-10-19):** source
   `.Len: uint` agrees across the package, sync and canonical books. Internal
   `sec.slice.len` remains an operation name, not source spelling.
2. **Reference metadata (P16-03/P16-05, synchronized 2026-10-19):** current
   package and consolidated contracts reuse P15 `AddressStability` and a
   separate pin dependency, replacing withdrawn relocation classes. Historical
   inputs retain notices identifying the corrected canonical contracts.
   Concrete metadata serialization MD-084 remains open; no new spelling is
   approved by this synchronization.
3. **Length type (decided 2026-10-17, MD-074/MD-088):** `len(slice)` and
   `slice.Len` are both `uint` and the same count
   (`rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md`).
   No signed length operation and no representability proof exist; signed
   offsets use an explicit checked conversion. No wrapping, cap or invented
   panic.
4. **P15 bridge:** P15-50–58 (schema 11, emitter and three MLIR verifiers) and
   its relevant integration tests are prerequisites for schema-12 acceptance.
   Their implementation/evidence stays owned by `lowering.sec-mlir-package15`.
   P16 Sema and Semantic IR work can proceed using today's P15 core first.
5. **Returns:** P15 returned-reference lowering now exists; its full acceptance
   and remaining MD-085 verification obligations stay owned by P15/P16-07.
   Returned-slice origin/range production remains P16-17/P16-27. Preserve
   existing source return legality rather than accept missing facts.
6. **Reborrow:** MD-061 was decided on 2026-10-15: explicit `ref r` reborrows
   the referent (borrowing.md § 11.4), and direct-reference lowering exists.
   P16 bounded reborrow plans and range narrowing remain their own work;
   they reuse that decision and do not reopen the holder interpretation.
7. **Try metadata:** MD-031 blocks guarded/residual/Option handler metadata in
   the Sec MLIR emitter. Ordinary supported Result handlers can be used for
   range/index tests; unsupported handler forms keep their existing rejection.

## Completion boundary

P16 owns target-independent **borrowed** shared/mutable slice values, exact
half-open ranges, explicit fixed-array borrows and slice reborrows, range/index
checks and error CFG, length observations, shared copy/mutable move, returned
origins/ranges, and schema-12 emission/verification (P16 §§4–69, 76–124).

Reuse P15's reference validity, storage identities, borrow/lifetime facts,
Place reads/writes and borrow-end markers. Empty slices retain their origin and
borrow facts, have no default value, and never allow an element Place without
proof/check. Shared slices are CopyTrivial; mutable slices are MoveOnly; both
are trivially destructible and own no elements.

Keep owning dynamic `T[]` representation/backing transitions in P18 and
non-trivial element reads, replacement, destruction and ownership transfer in
P17. P16 does not select physical descriptors, MemRef/LLVM slice layout, FFI
ABI, raw-pointer conversion, `.Ptr`, stable/weak handles, concurrent generation
protocols, slice equality/ordering/membership/iteration, or Reverse lowering.
Legacy implementations of deferred features do not establish maintained P16 IR.

Every new semantic function needs responsibility and exact rulebook references
per `codex.md`. Extract cohesive slice work from oversized files into focused
siblings. Keep Sec fixtures under `testdata/`; do not rewrite core/stdlib/manual
sources unless an actual synchronized migration requires them.

## A. Establish the current baseline and synchronize authority

- [x] P16-01 — Capture the intended current P13/P14/P15 changes in an exact,
  reproducible snapshot. Record commit/tree or equivalent complete patch and
  untracked-file inventory, toolchain versions and the `SEC_MLIR_BIN` variable
  name (resolve its absolute value locally; never record a private path).
  Final acceptance must use an isolated snapshot containing the actual changes.

  Completed 2026-10-19: exact tree/archive SHA-256, full local change/untracked
  inventory and current tool versions are recorded in the execution-baseline
  refresh above. The archive was extracted and focused slice/schema/32/64 tests
  passed in isolation. Normal staging was unchanged; private ignored files are
  excluded. Final acceptance remains a separate gate against its final tree.
- [x] P16-02 — Run predecessor Go/vet/lit and focused 32/64 gates; record
  failures separately with their current owners before changing slices.

  Completed 2026-10-19 as a current-baseline refresh: full Go tests and vet,
  125 lit tests and focused predecessor/32/64 emission gates completed. Vet,
  lit and focused gates pass. Full Go has 39 passing packages and 7 failing
  tests across 4 packages; the exact failures, causes and owners are listed
  above. This records the baseline before further source work; it neither
  reconstructs a historical pre-P16 run nor claims all repository tests pass.
- [x] P16-03 — Read P16 §§1–2, all four supplied inputs, the current memory,
  collections, errors, core, Semantic IR and MLIR rules, and applied corrections.
  Resolve the spelling/metadata drift above through the prescribed correction
  workflow; update `language-rulebook-status.md` for changed rulebooks.

  Completed 2026-10-19: P16 §§ 1–2 and the four inputs are reconciled against
  the current source/memory/error/core and representation rules. P16-04 and
  P16-05 contain their canonical integration evidence. The package MD/YAML
  now use `.Len`, unsigned `len`, canonical address stability and a separate
  pin dependency; obsolete signed-length operations and unresolved-length
  acceptance gates are removed. All 146 package section numbers are retained.
  The current-authority table names reviewed owners and distinguishes the
  historical input from today's implementation. Applied correction:
  `rules/corrections/applied/p16-package-authority-reconciliation-20261019.md`.
  Concrete serialization MD-084 and return-origin verification MD-085 remain
  open; no missing normative decision is inferred to be settled by this audit.
- [x] P16-04 — Integrate slice-sync §§1–19 into the owning canonical rules:
  explicit fallible borrowing, half-open normalization, deterministic RangeError
  precedence, stale-reference separation, arbitrary precision and copy classes.
  Audit what already agrees before changing text; do not invent signed-length policy.

  Completed 2026-10-19: all 19 slice-sync sections are mapped to canonical
  owners in `rules/corrections/applied/p16-slice-source-rules-synchronization-20261019.md`.
  Collections §§ 7.1, 7.4–7.5, runtime-check slice examples, reference-model
  § 14(8)–(12), copy/move § 4.4(4) and core-library § 13.1 now explicitly agree
  on construction, exact normalization, evaluation order, error precedence,
  temporal/spatial separation, empty provenance and classifications. Existing
  unsigned length rules remain unchanged; stale core summary spellings and
  the sync's `.len` example are corrected. This is specification synchronization;
  remaining source/IR/MLIR production and MD-084 remain separate.
- [x] P16-05 — Integrate the slice Semantic IR amendment and install version-12
  dialect/lowering material in the consolidated rulebooks, preserving version
  history and the implementation/specification distinction. Reconcile the old
  Semantic IR version-1 assumption with today's canonical v2 document and the
  still-version-1 executable IR; P16 itself requires no executable version bump.

  Completed 2026-10-19: all 26 amendment sections are installed in
  `rules/compiler/semantic_ir.md` § 126, linked from § 16(6); § 106(5)
  distinguishes document revision 2.0 from executable IR version 1. All 33
  dialect and 31 lowering sections are installed as v12-prefixed sections in
  the consolidated rulebooks, preserving prior versions and the original
  inputs. Both length forms use unsigned `sec.slice.len`; address stability
  and pin dependency replace historical relocation-class assumptions. Existing
  schema-11 early admissions are retained as v11.41. The applied correction is
  `rules/corrections/applied/p16-slice-ir-schema12-synchronization-20261019.md`.
  Default emission stays schema 11 and MD-084 remains open. This closes the
  representation-rule installation only; P16-03/P16-04 source-rule sync and
  P16-01/P16-02/P16-07 acceptance gates still require their own evidence.
- [x] P16-06 — Create the sole `lowering.sec-mlir-package16` integration in
  `governance/lowering.yaml` when execution begins, with accurate planned/partial
  status, current baseline, dependencies, evidence and deferrals. Validate with
  `yamlstatus`; do not recreate `implementation-status.yaml`.

  Completed 2026-10-17: `lowering.sec-mlir-package16` (status partial) lists the rules, the P15 predecessor state, implemented work, remaining sections and deferrals. P16-01 subsequently captured the current baseline on 2026-10-19. The current global governance check reports two pre-existing missing fragment registrations; the P16 fragment is canonical.
- [ ] P16-07 — Close the relevant P15 schema/emitter/verifier and returned-origin
  dependencies in their P15 owner. Re-run the predecessor checks before treating
  schema 11 as accepted. Keep static/MMIO and unrelated P15 gaps explicit.

## B. Modernize the existing Sema range model

- [ ] P16-08 — Audit slice projections, aliases, reference origins, nested range
  composition, borrowing, escape/return summaries, analysis and tooling consumers.
  Use the existing canonical Place model as the single migration target (§§24–30).

  Partial audit 2026-10-17 (range representation only). P16-17/P16-19
  subsequently cover the represented transfer/return paths and source test
  matrix; the complete cross-consumer inventory remains P16-08.
  - Slice projections live in `internal/sema/place.go`: `PlaceSlice`, `slicePlaceProjection`, `appendSliceProjection`/`appendIndexProjection`, the printer, `placeIsStaticallyEmpty`, `slicesAreStaticallyDisjoint`/`Equal` and `indexOutsideSlice`.
  - Every bound consumer was migrated in P16-09:
    - `storage_overlap.go` (`singleStorageOverlap`);
    - `parameter_usage_place.go` (callee demand places);
    - `parameter_usage_persistence.go` (persisted projections; schema version bumped to 2);
    - `analyzer.go` (`samePlaceIdentity`).
  - `CanonicalProjectionSlice` stays the deferred P15 category.
  - Semantic IR has no slice projection: `builder.go` rejects ref-of-slice at package 14/15.
  - Still to audit: about 68 `SliceType` consumers in Sema, among them escape analysis, reference-origin joins, FFI legality, known members (`.Len`), iterator dependencies and try failure sets. Also LSP/tooling presentation.
- [x] P16-09 — Replace canonical `int64` slice bounds with detached immutable
  arbitrary-precision half-open ranges. Migrate comparisons, printing, cloning,
  empty-range checks and nested slice/index composition; remove host-width
  arithmetic from correctness decisions (§§25,27–29).

  Completed 2026-10-17:
  - `PlaceProjection.SliceStart`/`SliceEnd` are `*big.Int` half-open bounds, with nil for a non-constant endpoint. They replace `int64` plus `SliceStartKnown`/`SliceEndKnown`, and `clonePlaceProjection` copies them.
  - Normalization is exact: a missing start is 0, an inclusive end is `end + 1` (no MaxInt64 give-up), and a missing end of a fixed array is its exact length, even beyond int64.
  - Exact arithmetic is used throughout: nested slice and index composition (inputs are never mutated), emptiness, disjointness, equality (`sliceBoundsEqual`), index containment, storage overlap, callee demand places and identity comparison. `addInt64` is removed.
  - Persisted parameter-demand summaries are now schema version 2.
  - Diagnostic presentation prints an unknown start as `*` (MD-089, decided and integrated 2026-10-18: borrowing.md § 12(9), diagnostics.md § 10(7)).
  - Tests: `TestSliceProjectionBoundsAreExactBeyondInt64` and `TestSlicePlaceProjectionNormalizesEndpointsExactly`; the existing overlap, relationship and disjoint-borrow tests are migrated.
  - MD-088 was subsequently decided on 2026-10-17: `len(slice)` and `.Len` both return `uint`; P16-05 installs the corrected IR contract.
- [x] P16-10 — Add stable `SliceRangeID` for evaluated dynamic ranges using
  compiler semantic identity, not endpoint text, spelling or runtime tags.
  Define absent state, deterministic allocation and detached query behavior (§26).

  Completed 2026-10-18. Dynamic source slice projections carry a compiler-only
  `SliceRangeID`, interned by semantic source site in deterministic first-use
  order; zero means absent and static ranges do not allocate an identity.
  Reanalysis resets the range table; allocator snapshots and `ResolvedPlaceOf`
  queries are detached. Nested dynamic ranges and indexing retain the symbolic
  parent projection. Identities alone never prove runtime equality or
  disjointness, and relative child ranges cannot prove disjointness through
  unknown parents. Analysis-local IDs are excluded from persisted parameter
  summaries. Slice normalization/composition now lives in `slice_places.go`,
  and `samePlaceIdentity` moved from `analyzer.go` to `place_identities.go`.
  Tests: `TestSliceRangeIDsAreStableAndQueriesDetached` and
  `TestSliceRangeIdentityCompositionAndAllocation`, with source fixture
  `testdata/sema/slice_range_id_valid.sec`.
- [x] P16-11 — Normalize missing start/end and inclusive/exclusive endpoints with
  exact arithmetic; preserve source/start/end evaluation order and one evaluation.
  Inclusive-end normalization must not overflow before validation (§§13–19).

  Completed 2026-10-19 (Sema scope). One exact `normalizeSliceRange` producer
  classifies known invalid bounds before inclusive increment, supplies open
  bounds without synthetic expressions, and feeds both creation plans and
  canonical Place projections. Unknown endpoints remain unknown; normalized
  outputs detach arbitrary-precision input values. Creation plans carry a
  detached source/start/end operand schedule, each present operand exactly
  once; Sema uses that order and P16 builder preflight validates the snapshot.
  Known call-returned reference sources now reuse existing P15 origin facts
  without another call/inference. Borrow availability and authority are
  validated after operand analysis, including sources only then resolved.
  Cohesive reference inference moved into reference_inference.go. Tests cover
  all open forms, inclusive/exclusive wide bounds through 2^256, zero length,
  precedence before normalization, detached schedules, ordinary/try call
  operands, source-before-start/start-before-end moves, and no elevation from
  a returned shared reference. Full Sema/IR/emitter suites and focused CLI
  regressions pass. Executable source operand emission and runtime evaluation
  belong to P16-30..P16-33; this does not claim those producers are implemented.
- [x] P16-12 — Preserve static disjointness and empty-element-range behavior;
  compose nested static ranges exactly. Dynamic/unknown overlap stays conservative;
  an empty slice still retains provenance/lifetime dependencies (§§28–30,59–62).

  Completed 2026-10-18. A source slice's established static extent now supplies
  an omitted end when slicing it again. Static inclusive/open subranges compose
  to exact absolute bounds and allocate no dynamic identity. Extent proofs
  require complete static ranges and unanimous lengths across origin alternatives;
  unknown/dynamic origins stay conservative. Source tests keep disjoint mutable
  subranges live together, and verify that nested empty slices and two mutable
  slices of a zero-length array retain root/storage/borrow/lifetime facts while
  borrowing no elements. Returning an empty local-origin slice is still rejected.
  Tests: `TestStaticSliceSubrangesAndEmptyProvenance` and
  `TestStaticSliceExtentRequiresUnanimousOrigins`; existing dynamic-range and
  arbitrary-precision composition tests remain in force. This completes the
  Sema Place scope of P16-12; slice IR and runtime lowering retain their own items.
- [x] P16-13 — Verify constant-invalid source diagnostics and canonical runtime
  classification: InvalidRange, then StartAfterEnd, then OutOfBounds. Preserve
  precedence where multiple conditions fail (§§18–23).

  Completed 2026-10-17 (`internal/sema/slice_range_classification.go`):
  - `ClassifySliceRange` is the one canonical classifier with exact `big.Int` arithmetic. It checks:
    1. a negative start, then a negative end → `InvalidRange`;
    2. an explicit start greater than the explicit end → `StartAfterEnd`;
    3. a start beyond the length, an exclusive end beyond the length, or an inclusive end at or beyond the length → `OutOfBounds`.
  - Unknown facts never fail. The same function defines the runtime `RangeError` for the same values, so P16-14/P16-31 can reuse it and the observable error stays the same under optimization.
  - `checkSliceBounds` now reports exactly one compile-time error per range: the first failure under that precedence, where it used to report every failing condition. It also diagnoses start after end for sources of unknown length (dynamic arrays, slices), not only fixed arrays. The message texts are unchanged.
  - Reporting only the precedence-first failure at compile time is MD-094 (decided and integrated 2026-10-18: collections.md § 7.5, package16.md §§ 18–19).
  - Tests: `TestClassifySliceRangePrecedence` (including values beyond int64) and `TestConstantSliceRangeDiagnosticsFollowPrecedence`.
  - Runtime/fallible lowering remains P16-31/P16-32. The Sema normalization/evaluation contract was subsequently completed by P16-11 (2026-10-19).
- [x] P16-14 — Publish `ResolvedSliceCreationPlanOf(*ast.RefExpression)` with
  source kind, authority, element type, source Place/slice, original endpoint
  types/signedness/constants, range/proof, BorrowID/LifetimeID, P15 reference facts
  and canonical RangeError type (§§33–38). Querying must not register another borrow.

  Completed 2026-10-18. `slice_creation_plans.go` records creation plans only for
  explicit ref-of-range expressions: fixed/dynamic array or slice source,
  original source type/Place, element type and authority, original endpoint
  types/signedness/exact constants, normalized source-relative static range or
  dynamic identity, conservative runtime/proven classification and proof kind,
  and the existing P15 reference plan/BorrowID/LifetimeID. Queries return
  detached endpoint, range, type, origin and error-case data without analysis
  or borrow creation. Endpoints now undergo integer type inference in source
  order, and nested range validation uses an established parent extent. The
  existing core `RangeError` has a matching standalone builtin identity and
  trusted-core declaration support. Slice inference and range diagnostics were
  extracted from the inference/analyzer monoliths into focused slice files.
  Tests: `TestResolvedSliceCreationPlans`, `TestSliceRangeErrorCoreIdentity`,
  `TestSliceCreationRejectsNonIntegerEndpoints`, and
  `TestSliceCreationRejectsRangePastKnownParent`. Source-to-IR slice creation,
  fallible CFG remain separate TODO items; broader query coverage is completed
  by P16-16 below.
- [x] P16-15 — Publish `ResolvedSliceIndexPlanOf(*ast.IndexExpression)` with
  source authority, exact index/type/proof facts, use/transfer action and IndexError
  identity (§§50–58). Extend existing resolved index producers; no second proof engine.

  Completed 2026-10-18. Analysis records source authority/Place, exact known
  slice extent, original integer type/signedness/constant, shared P14 check,
  proof/use/transfer enums and the existing canonical IndexError. Fixed-array
  and slice plans share one exact constant/domain/refinement proof producer;
  unknown extents remain runtime-checked. Proven local subranges, range-typed
  indices and dominating branch guards reuse existing proofs. Known invalid,
  empty-slice and negative indices receive the owning bounds diagnostic.
  Read, write, shared/mutable borrow and accepted try handling update the same
  stored plan. Queries deeply detach all types, origins, exact integers and
  error cases, allocate no identities and reset between analyses. Cohesive
  index plan/proof/use functions moved from analyzer.go into index_plans.go.
  Tests: TestResolvedSliceIndexPlans, TestSliceIndexConstantBounds and existing
  fixed-array/list and slice creation-plan regressions. Slice Semantic IR,
  schema 12 and reference-validity effects remain separate work; the broader
  query matrix is completed by P16-16 below.
- [x] P16-16 — Make all plan queries read-only and defensively detached, including
  big integers, origin alternatives, nested types and ranges. Add mutation and
  repeat-query tests proving no Analyzer changes (§§37,51).

  Completed 2026-10-18. Creation and index queries return detached exact
  integers, endpoint and element types, source Places/ranges and error cases.
  Slice-specific snapshot helpers now preserve nested finite origin trees:
  routing through the general flat P15 Place/reference copier could discard
  deeper alternatives before cloning. Embedded reference facts retain their
  complete Place/origin trees and independently copied storage identities.
  Snapshot helpers live in slice_plan_snapshots.go. The source-backed
  TestSlicePlanQueriesAreDetachedAndReadOnly repeatedly mutates every exposed
  subtree, checks that later queries are unchanged, and verifies identity
  tables, borrow/plan/error counts and empty inference/Place caches stay
  unchanged. Unknown and nil queries remain side-effect-free. Existing exact
  range/index and source creation query regressions remain in force.
- [x] P16-17 — Carry slice origins/ranges through parameters, shared copy, mutable
  move, reborrow, aggregate fields and returns. Preserve caller-origin legality,
  reject local escape and conservatively widen unknown runtime ranges (§§39–43,63–69).

  Completed 2026-10-19 (Sema). Returned-origin binding composes canonical
  static slice/index projections through actual caller Places and discharges
  the parameter dereference already represented by the actual referent. Open
  ends retain their source-length relation until caller substitution; unknown
  runtime bounds remain unknown, with distinct stable call-site range IDs.
  Field-sensitive contents bind value/reference parameters and receivers to
  their existing referents, including aggregate copies, returned aggregates
  and methods. Reference metadata propagation preserves local/match escape
  restrictions and Arena/epoch/address dependencies through calls and further
  slice projections. Moving a slice parameter transfers its holder, not the
  referent's availability; moved/discarded holders remain rejected. No backing
  ownership is transferred. Cohesive return/contained-origin and index-write
  logic was extracted from analyzer.go. Source tests verify exact ranges,
  conservative overlap, caller-owned returns, local-escape rejection, field
  paths, copy/move, mutable authority, epoch identity and deterministic IDs on
  32/64 plans. Source-to-IR returned-slice production remains P16-27/P16-36;
  P15 acceptance decisions MD-084/MD-085 are unchanged.
- [x] P16-18 — Integrate canonical panic/allocation/blocking effects: ordinary
  unproven spatial checks may panic; try handles only spatial failure; dynamic
  stale validation remains invalid-reference-generation. Borrowing/validation adds
  no allocation or blocking (§§118–120). Use current summaries, including ISR/noAlloc.

  Completed 2026-10-19. Resolved unproven slice creation ranges publish
  canonical BoundsFailure effects. Explicit `try ref`/`try ref mut` ranges
  participate in the existing failure set with RangeError; only a valid
  handler/propagation channel removes that range's bounds panic. Endpoint
  arithmetic and operand-call effects remain independently represented.
  Full open `[..]` ranges carry source-open-bound proof even when their extent
  is unknown, without inventing a numeric length or static range. `.Len` and
  compiler-known `len` now publish reference-read facts so dynamic slice
  observations retain InvalidReferenceGeneration; try over range/index does
  not suppress it. Existing panic, allocation, blocking and ISR summaries are
  reused. Tests cover noPanic/noAlloc/noBlock, copy/move, unsafe, mixed checks,
  valid/invalid propagation, operand calls, arena slices and both length forms.
  Global compiler-known function inference was extracted from the inference
  monolith into compiler_known_functions.go. Runtime/source/MLIR slice
  production remains separate from these frontend effect facts.
- [x] P16-19 — Add Sema source tests for exact ranges beyond int64, all open/end
  forms, nested/static disjointness, empty/zero sources, dynamic overlap, negative
  endpoints, conflicting borrows, copy/move and returned origins (§§125–128,134–137).

  Completed 2026-10-19: slice_matrix_{valid,invalid}.sec and
  slice_matrix_wide.sec, plus slice_origins_valid.sec and their Go tests cover
  all six open/inclusive/exclusive source forms, singleton and empty ranges,
  zero-length sources, signed int128/unsigned uint256 runtime bounds, known
  bounds beyond int64 on a 64-bit plan and the owning uint32 extent rejection,
  exact nested disjoint mutable ranges, disjoint indexed writes, static/dynamic
  conflicts, shared and mutable reborrows, shared copy with non-copyable
  elements, mutable copy/move/discard rejection and returned origins through
  functions/aggregates/methods. Negative assertions are tied to the owning
  function and diagnostic, including local escape through a further range on
  a call result and Arena reset while returned dependencies remain live.
  Existing P16 classification, creation/index/query, range-identity, empty
  provenance and effect tests complete the associated matrix. New regressions
  fixed by this work are documented in P16-17 and the P16 ledger; index
  assignment now checks its projected Place without inventing a whole-owner
  read. Full root Sema, IR and lowering suites, focused compiler/native tests,
  reference/borrow LSP tests and vet pass. This is the Sema source matrix;
  executable range/index CFG and runtime slice acceptance retain their own
  later checklist items.

## C. Add high-level Semantic IR and verification

- [x] P16-20 — Intern shared/mutable slice types separately from direct element
  refs and owning dynamic arrays. Element TypeID and authority determine type;
  runtime length does not. Add SliceFacts reusing current P15 facts (§§31–32,76).

  Completed 2026-10-19. The Semantic IR model has distinct shared `slice` and
  move-only `slice-mut` type kinds and `TypeTable.InternSlice`, keyed only by
  the existing element TypeID and authority. Canonical type verification
  rejects absent/non-value elements and attached runtime-length, physical,
  nominal or redundant authority metadata. Printing preserves element identity
  and the shared/mutable distinction. `SliceFacts` reuses P15
  `ReferenceMetadata` for borrow/lifetime, storage, validity, epoch, address
  space and AddressStability; it adds Place SSA origins, runtime length SSA,
  exact half-open static range and symbolic range identity. Cloning detaches
  origin/storage slices and exact bounds without changing identities.
  Tests cover 32/64-bit plans, nominal/wide element distinctions, length-free
  interning, invalid type forms, large/empty range snapshots and the resolved
  source creation plans. This is the type/fact-model step: source slice
  operations, full guard/SSA verification, schema 12 and binary lowering remain
  separate. Source builds at package limits 14/15/16 still reject unsupported
  slice operations without returning a partial module.
- [x] P16-21 — Add range normalize/check operations with original endpoint
  information, normalized start/length, proof provenance and canonical failure/error
  results. Verify types, exactness and failure convention (§§77–80).

  Completed 2026-10-19. Semantic IR now has slice.range-normalize and
  slice.range-check. Ordered operands retain source uint length and each
  present endpoint's original TypeID/signedness; omitted endpoints have no
  synthetic operand. Normalize requires established proof provenance and
  returns uint start/length. The total checked operation returns uint
  start/length, failed bool and the exact P11 core RangeError enum. Its immediate
  failed-true/success-false branch confines normalized results to success and
  error results to failure, including successor arguments and paths that
  attempt to bypass the guard. Printing retains all metadata deterministically.
  Tests cover 32/64-bit plans, every open/inclusive/exclusive shape, extreme
  int128/uint256 endpoints without pre-check narrowing or inclusive increment,
  canonical error domain, malformed tuples/proofs and wrong-edge uses.
  These are semantic operations and verification contracts; source CFG
  production, dynamic-source validity, remaining P16-26 checks, schema 12 and
  physical lowering remain separate.
- [ ] P16-22 — Add fixed-array shared/mutable borrows and shared/mutable slice
  reborrows. Enforce writable source, no authority/range/lifetime expansion, and
  preservation of storage/epoch facts without copying elements (§§42–46).
- [x] P16-23 — Add explicit shared slice copy/mutable slice move, preserving
  holder/borrow facts without element transfer. Extend borrow-end verification;
  neither ending a slice nor discarding it destroys backing storage (§§39–41,63).

  Completed 2026-10-19. Semantic IR has explicit `slice.copy-shared` and
  `slice.move-mut` operations preserving the bounded reference and complete
  P15 borrow/lifetime/storage/epoch facts. Copies retain separate holder ends;
  mutable moves make the old holder unavailable. Canonical `ref.end-borrow`
  now accepts slice values and verifies their source facts; generic SSA slice
  discard must use this marker instead of omitting the holder obligation.
  Copy/move/end perform no element transfer, backing cleanup or epoch
  observation. Lifetime verification was extracted into a focused sibling
  file and covers use after move/end, double end and forwarded old holders.
  Tests cover shared/mutable, proven/dynamic and 32/64-bit plans, independent
  shared-holder ends, malformed facts/authority and fresh loop holders.
  Full slice CFG fact joins remain P16-26; source production remains P16-36,
  and schema 12/physical lowering remain separate.
- [x] P16-24 — Add the unsigned length operation for both `.Len` and `len(...)`
  (one `uint` count since MD-074/MD-088; no signed-length operation).
  Dynamic slice observations require the P15 validity guard (§§47–49,110–111).

  Completed 2026-10-19. `slice.length` accepts shared/mutable slice values
  and produces the exact target-sized `uint` count, with no element read,
  holder transfer, new borrow or signed conversion. Slice parameters reuse
  P15 `ReferenceMetadata`; `ref.is-valid` accepts the distinct slice types.
  The existing reference verifier includes length observations, checks the
  source's complete borrow/epoch identity, and requires the matching guard
  block and successful edge to dominate the observation. Stale-reference
  failure remains `InvalidReferenceGeneration`, including under @noPanic;
  an unrelated guard, wrong epoch or bounds-recovery endpoint cannot replace it.
  Source tests confirm `.Len` and `len(...)` infer `uint` for both authorities.
  IR tests cover proven/dynamic values on 32/64 plans, deterministic printing,
  signed/fixed-width result rejection, false-edge and guard-bypass failures.
  Source-to-operation production remains P16-34; schema 12 and physical
  lowering remain separate. No slice-int-length operation was introduced.
- [x] P16-25 — Add slice index predicate and element Place, then reuse P15
  PlaceRead/PlaceWrite/reference-borrow operations for trivial reads, replacement
  and element refs. Do not add an element ownership-transfer path (§§50–58,81–82).

  Completed 2026-10-19. `slice.index-in-bounds` consumes the bounded slice
  and original integer index; `slice.element-place` produces a P15 Place
  preserving element type, shared/read-only versus mutable/read-write
  authority and complete source reference facts. Proven forms require P14
  proof provenance; runtime forms require the exact slice/index predicate
  and its dominating successful edge. Dynamic source validity precedes both
  spatial observation and Place formation. Ordinary failure uses slice-index
  bounds panic; fallible failure uses canonical IndexError.OutOfBounds.
  Existing P15 reads, trivial replacement and shared/mutable element borrows
  work without array loads/copies or an element ownership-transfer operation.
  Derived Places cannot outlive an ended or moved source holder. The shared
  array/slice failure-endpoint verifier was extracted into sequence_bounds.go.
  Tests cover 32/64-bit plans, proven/runtime bounds, proven/dynamic validity,
  all supported use modes, original wide integer carriers/extreme constants,
  typed fallible propagation, malformed SSA/authority/proof/guard/failure
  contracts and the nontrivial-element boundary. Full slice verification
  remains P16-26; source production is P16-35, with schema 12 and physical
  lowering separate.
- [ ] P16-26 — Extend the Semantic IR verifier for range and index guard dominance,
  exact source SSA, canonical failure CFG, source validity before spatial use,
  shared/mutable authority and reference/borrow lifetimes (§§83–86).
- [ ] P16-27 — Extend returned-origin/range verification using the represented P15
  return contracts. Add deterministic printing and defensive clones of slice
  metadata, with no physical descriptor vocabulary (amendment §§22,25–26).
- [ ] P16-28 — Add positive/negative white-box IR tests, including mismatched SSA,
  misplaced guards, stale validation caught as a spatial error, missing proofs,
  widened reborrows, mutable copies and uses after borrow end (§§129,136).

## D. Connect source lowering exclusively through resolved plans

- [x] P16-29 — Add the explicit MaxPackage-16 builder route. Require resolved
  slice creation/index/use facts; return no module when facts or a required
  dependency are unsupported. Keep lower package behavior unchanged.

  Completed 2026-10-19. The opt-in MaxPackage-16 route snapshots canonical
  Sema creation and index/use plans for the requested module before building
  functions. Missing/incomplete facts fail without re-inference; unavailable
  source producers remain explicit package-16 capability errors. Slice
  expressions dispatch before P15 reference/array handling, with separate
  fixed-array borrow, reborrow, element-use, P18 dynamic-owner and P16-36
  value/signature gates. No failed build returns a partial module, including
  when an earlier function has already been built. Detached-plan, missing-plan,
  module-selection and predecessor tests pass. Slice-free P15 programs build
  identically at P16, and the default P14 boundary remains unchanged.
  This completes the builder route and prerequisites only; source operations
  remain P16-30..P16-36, schema 12 and physical lowering remain separate, and
  no CLI/default package capability is promoted.
- [ ] P16-30 — Lower explicit fixed-array borrows from the source Place plus
  normalized range. Evaluate root/start/end once, in order, without array load,
  copy or allocation; support valid empty sources/ranges (§§42,78–80).
- [ ] P16-31 — Lower ordinary runtime range CFG to bounds panic with slice-range
  provenance, forming the slice only after success (§§20,80).
- [ ] P16-32 — Lower `try ref` / `try ref mut` range CFG to canonical RangeError,
  including local handlers and naked propagation. Preserve failure precedence
  and existing try metadata boundaries; no new handler schema (§§21–23,131).
- [ ] P16-33 — Lower slice reborrow with source validity before spatial checks.
  Shared source yields shared only; mutable source can yield shared or mutable,
  preserving normalized range/origin facts (§§43–46,60–62).
- [ ] P16-34 — Lower `.Len` and proven `len(slice)` from resolved core operations,
  with dynamic validity and no element read. Reject unresolved signed-length
  representability with no partial IR (§§47–49).
- [ ] P16-35 — Lower ordinary/proven/fallible indexes to element Places with
  IndexError.OutOfBounds or slice-index bounds panic as appropriate. Evaluate
  source/index once, temporal validity first, and Place before write RHS (§§50–58).
- [ ] P16-36 — Lower copy/move, parameters, permitted returned slices, projected
  origins and borrow ends from Sema plans. Retain exact static subranges and
  conservative runtime origins; reject local escape (§§39–41,63–67).
- [ ] P16-37 — Add source-to-IR tests for every supported path and every deferred
  dependency. Prove operand order, SSA reuse, no redundant proven checks, no
  aggregate copies, no partial modules and unchanged predecessor paths (§§129–137).

## E. Implement schema 12 and the slice verifier

- [x] P16-38 — After P15 schema 11 works, implement schema-12 registration and
  emitter selection. Preserve accepted schemas 1–11 and reject unknown schema 13;
  update CLI/build defaults only when the intended source path is usable (§87).

  Completed 2026-10-19. C++ dialect registration accepts schema 12 with the
  existing complete module/target metadata contract and preserves schemas
  1–11 as inputs; unknown 13 is rejected. `EmitWithOptions` selects schema 12
  explicitly through the same verified emitter. `Emit` and zero options retain
  schema 11, and CLI/build defaults are unchanged. Schemas 1–10 remain input
  compatibility modes, not output downgrade choices. Unsupported schema
  requests, invalid Semantic IR and unsupported slice types still return no
  output. Emitter setup moved into emitter_options.go as a focused sibling.
  Real-tool tests cover 32/64 plans, deterministic round trips, predecessor
  schema acceptance, unknown-version rejection and the unsupported slice
  boundary. Full MLIR regression suite passed 118/118 at P16-38 completion.
  P16-39/P16-41 subsequently add slice types and reference/bounds local
  contracts; slice operations, guard verifiers and emission remain P16-40
  and P16-42..P16-45; schema registration
  does not claim executable slices or physical representation support.
- [x] P16-39 — Add `!sec.slice<T>` / `!sec.slice_mut<T>` ODS types and exact
  parsing/printing. No length, address, origin or epoch type parameter (§§88–90).

  Completed 2026-10-19. ODS `SliceType` and `SliceMutType` parse/print exactly
  `!sec.slice<T>` and `!sec.slice_mut<T>` with one element-type parameter.
  Element verification rejects void, never, compiler-only Place/storage and
  function signatures, including invalid nominal bases, while retaining wide,
  decimal, nominal, array, reference and nested slice element identity.
  Schemas below 12 reject slice types even inside nominal/array wrappers and
  function type attributes; checks also cover operands/results and region
  block arguments. Shared and mutable types remain incompatible. Round-trip,
  malformed-element, extra-parameter, authority and old-schema tests pass;
  the full MLIR suite passes 122/122. No Go slice emission, slice operations,
  physical representation or default schema promotion is introduced.
- [ ] P16-40 — Add range_normalize/range_check, borrow_shared/borrow_mut,
  reborrow_shared/reborrow_mut, copy_shared/move_mut, len/len_int,
  index_in_bounds/element_place with synchronized P15 metadata (§§91–103).
- [x] P16-41 — Extend sec.ref.is_valid and sec.ref.end_borrow to slices;
  extend sec.fail.bounds with slice-range/slice-index provenance and the
  canonical panic identity (§§104,110–111; schema-v12 §§21–23).

  Completed 2026-10-19. Schema-12 shared/mutable slices use the existing
  `sec.ref.is_valid` predicate and `sec.ref.end_borrow` marker, including
  nonzero borrow/lifetime IDs. The canonical P15 reference dictionary and
  related local metadata accept slice parameters/results with the same
  policy/epoch validation; invalid metadata remains rejected. The bounds
  endpoint accepts fixed-array-index and schema-12-only slice-index/slice-range
  provenance, retaining intrinsic panic.bounds. Stale-reference failure
  remains panic.invalid-reference-generation. Direct element-reference
  dereference, comparison and reborrow rules are not widened to slices.
  Local reference/bounds verifier functions were extracted from SecOps.cpp
  into ReferenceBoundaries.cpp. Round-trip and negative tests cover both
  slice authorities, parameter/call-result facts, missing epoch, invalid IDs,
  array operands, slice comparison/dereference, wrong provenance and missing
  or older schemas. Full MLIR suite passes 125/125, including predecessor
  reference/array tests. Whole-function slice guard/borrow verification remains
  P16-42/P16-43, with dedicated slice operations and emission separate.
- [ ] P16-42 — Register `--sec-verify-slice-guards`. Verify same-source guard
  dominance, range normalization/error precedence, index bounds and element
  Place formation; require dynamic validity before spatial guards (§§105–110).
- [ ] P16-43 — Extend P15's place, reference-guard and borrow verifiers for slice
  operands, narrowing, shared copy/mutable transfer and returned-range facts.
  Do not duplicate source borrow analysis (§§83–86,116).
- [ ] P16-44 — Emit schema-12 slice operations only from verified Semantic IR,
  preserving TypeIDs, source locations, exact ranges, dynamic identities, CFG,
  borrow/lifetime/storage/epoch facts and target plans; unsupported input emits
  no artifact (lowering-v12 §§1–31).
- [ ] P16-45 — Add MLIR round-trip and invalid suites for types/ops, metadata,
  range/index/reference guards, authority, move/copy/end use, empty slices,
  unknown versions and schema-11 regression (§§138–139).

## F. Prove predecessor integration and architecture boundaries

- [ ] P16-46 — P6/P8: test 32/64 target scalar resolution of slice elements and
  length results while preserving wrappers; integer normalization must not
  recursively rewrite slice elements or lower slice semantics (§§112–113).
- [ ] P16-47 — P13/P14: test shared slice fields/contained origins and mutable
  slice aggregate move-only classification; fixed-array Place sources, exact
  lengths and no array/aggregate copies (§§114–115,140).
- [ ] P16-48 — P15/P12: test parameter/return origins, nested element refs,
  matching lifetimes/epochs, dynamic guard ordering and ordinary slice values
  in existing value paths without adding slice patterns (§§116–117,140).
- [ ] P16-49 — Exercise int128/int256/uint128/uint256/decimal128 elements and
  wide endpoints/indexes where source rules allow, with target-aware length
  proofs, empty sources and conservative dynamic ranges (§§3,126–128,140).
- [ ] P16-50 — Add source → target-aware Sema → verified Semantic IR → schema-12
  → real sec-mlir-opt tests on both 32/64 plans using absolute tooling and all
  slice/P15 verifier passes. Keep dynamic-epoch white-box evidence distinct
  from source evidence while Arena allocation IR remains deferred (§141).
- [ ] P16-51 — Test unsupported dynamic owners, raw pointers, physical/FFI
  descriptors, handles, equality, iteration/membership/Reverse, non-trivial
  element operations and unresolved signed len, each with no partial artifact
  (§§70–75,121–124,142). Do not claim P17/P18 lowering from legacy support.
- [ ] P16-52 — Assert successful P16 modules contain no physical pointer/length
  aggregate, GEP/stride arithmetic, hidden ABI arguments, runtime borrow counters,
  new generation manager, hidden allocation, undef or poison (§143).
- [ ] P16-53 — Rebuild plans/modules with fresh analyzers and reordered metadata.
  Verify deterministic identities, origin ordering, exact range printing, source
  diagnostics, and independent 32/64 outputs without mutating shared input.

## G. Final acceptance and handoff

- [ ] P16-54 — Expand the matrix below to one row per P16 §144 criterion, with
  exact implementation and test references. Keep unsupported cases and missing
  normative decisions visible; aggregate success proves only the stated scope.
- [ ] P16-55 — On the isolated execution snapshot, run full Go/vet, configured
  P13/P14/P15/P16 source pipelines, check-sec-mlir, diff checks and governance
  checks. Record exact commands, results, environment, commit/tree and failures.
- [ ] P16-56 — Update the single P16 governance entry with actual implemented
  scope, P15 dependencies and P17/P18 deferrals; mark implemented only after
  every applicable §144 acceptance criterion has evidence.
- [ ] P16-57 — Write the §145 report covering all 43 requested topics in order,
  including current predecessor state, signed-len limitation, deviations and
  recommendations for P17. Record its concrete path in governance.
- [ ] P16-58 — Recheck acceptance, rulebook status, unique governance ownership,
  predecessor regressions and unsupported boundaries before handoff.

## Acceptance coverage map

This maps every family of §144 criteria to planned work; it is not a completed
acceptance matrix. P16-54 must split families into individual evidence rows.

| §144 criterion family | TODO ownership |
|---|---|
| Exact newer baseline; previous package regressions | P16-01–02, 07, 55 |
| Slice sync, Semantic IR amendment, dialect/lowering v12, explicit fallible syntax | P16-03–05, 32, 38 |
| Exact/static/dynamic ranges, PlaceSlice migration, nested normalization/disjointness | P16-08–13, 19 |
| Read-only creation/index plans and index proof/runtime classification | P16-14–16, 19 |
| Shared/mutable types, SliceFacts and P15 reference facts | P16-20, 26–28 |
| Fixed-array borrows, slice reborrows, copy/move and empty slices | P16-22–23, 30, 33, 36–37 |
| Range normalize/check, ordinary panic, RangeError and local/naked try | P16-21, 31–32, 37 |
| Unsigned/signed length with no truncation | P16-24, 34, 49, 51 |
| Index predicate/Place, read/write/element refs, IndexError and ordinary panic | P16-25–26, 35, 37 |
| Dynamic validity ordering; ref.is_valid/end_borrow and fail.bounds extensions | P16-18, 26, 33–35, 41–43 |
| Slice guard verifier and P15 verifier extensions | P16-26, 28, 42–45 |
| Returned-slice origins/ranges | P16-07, 17, 27, 36, 48 |
| P13/P14/P15 integration | P16-46–50 |
| Dynamic owner distinct, no equality/FFI/physical descriptor/mandatory runtime | P16-20, 51–52 |
| Complete check-sec-mlir / Go gates and operational legacy paths | P16-02, 37, 45, 55, 58 |

## Recommended execution order and reproducible gates

1. P16-01–06: current snapshot and synchronized authority.
2. P16-08–19: exact ranges and detached Sema plans; independent of C++ completion.
3. P16-07 alongside P16-20–28: close relevant P15 dependencies and add slice IR.
4. P16-29–37: source lowering, effects and error CFG.
5. P16-38–45: schema-12 emitter/MLIR/verifiers after schema 11 exists.
6. P16-46–58: integration, acceptance evidence and report.

Run from the recorded execution snapshot, not an obsolete predecessor worktree:

```sh
go test ./...
go vet ./...
SEC_MLIR_BIN=/absolute/path/to/sec-mlir/bin go test ./internal/lowering/... ./internal/ir/... ./cmd/compiler -count=1
cmake --build build/sec-mlir --target check-sec-mlir -j2
go run ./cmd/yamlstatus check
go run ./cmd/yamlstatus fmt -check governance/lowering.yaml
git diff --check
```

Add the concrete P16-focused test names and verifier invocations when they
exist. Test tooling must fail on configured relative SEC_MLIR_BIN paths, and
final acceptance must exercise real tools rather than silently skip them.
