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
The current partial implementation report is
`rules/mlir/packages/sec-mlir-dialect_package16-implementation-report.md`.
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
   Concrete metadata serialization MD-084 remained open at the time; no new
   spelling was approved by this synchronization. (MD-084 decided and applied
   2026-10-22; schema 12 inherits the schema-11 spellings unchanged.)
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
  Default emission stays schema 11; MD-084 was open then (applied 2026-10-22). This closes the
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

- [x] P16-08 — Audit slice projections, aliases, reference origins, nested range
  composition, borrowing, escape/return summaries, analysis and tooling consumers.
  Use the existing canonical Place model as the single migration target (§§24–30).

  Completed 2026-10-20. The inventory below follows current consumers of
  `SliceType`, `PlaceSlice`, range endpoints/identities and their shared
  Place/reference APIs, including consumers without a direct slice-kind switch.
  File names in Sema rows are relative to `internal/sema/`.

  | Consumer | Current authority and migration result |
  |---|---|
  | Source types, identity and traits | `type_references.go`, `type_compatibility.go`, `types.go`, `analyzer_let_bindings.go` and declaration checks in `analyzer.go` distinguish owning arrays from bounded references, reject bare slice declarations and keep element type/authority in identity. Shared-reference copying does not copy elements; mutable references remain move-only. Slices do not destroy elements. |
  | Projections, aliases and overlap | `place.go` owns `PlaceSlice`, printing, origin alternatives and relationships. `slice_normalization.go` and `slice_places.go` own exact half-open bounds, omitted ends and static composition. `place_identities.go` owns equality and evaluated dynamic identities. `storage_overlap.go` delegates structural relationships conservatively; unknown bounds and provenance never prove physical disjointness. No endpoint consumer narrows `SliceStart`/`SliceEnd` to host integers. |
  | Borrowing, availability and transfers | `reference_inference.go`, `analyzer_borrow_access.go`, `analyzer_lifetime.go`, `reference_holder_origins.go`, `explicit_reborrow.go` and `reference_plans.go` reuse canonical Places and the existing P15 borrow/lifetime/storage facts. `index_assignment.go` checks projected writes. Empty ranges retain origin/lifetime while borrowing no elements. P16-17/P16-19 cover copying, moving, ending holders and reborrowing transferred ranges. |
  | Return origins and escape summaries | `reference_origin_join.go`, `reference_return_origins.go`, `reference_contained_origins.go`, `slice_return_origins.go`, `slice_origin_dependencies.go` and `reference_summary_round.go` preserve finite origins, field paths, open ends and rebased dynamic identities. `escape_analysis.go`, `escape_call_boundary.go`, `escape_storage_domain.go` and `return_reference_summary.go` consume those origins rather than deriving a second alias model; local-storage escapes remain invalid. Imported returned-origin verification is still the P15/MD-085 gate. |
  | Arena and temporal dependencies | `analyzer_arena.go` supplies borrowed allocation results; transferred/reference-carried slices retain storage, epoch and address-space facts through the same origin producers. `slice_effects.go` and `reference_plans.go` keep generation failure separate from range failure. P16-17/P16-19 exercise Arena return chains and reset rejection. |
  | Range creation, indexing and length | `slice_creation_plans.go`, `slice_index_plans.go`, `slice_range_classification.go`, `index_bounds_diagnostics.go`, `compiler_known_functions.go` and `analyzer_inference.go` use canonical exact extent/range facts. `.Len` and `len` return `uint`; observing a length retains temporal-use checks. Detached snapshots live in `slice_plan_snapshots.go`. |
  | Try and effects | `try_failure_set.go` consumes resolved creation/index plans over the protected subtree. `slice_effects.go` records ordinary bounds effects and removes only handled spatial failures; generation failure remains independent. No slice allocation/blocking effect is invented. |
  | Iteration | `analyzer_for.go` recognizes slice sequences separately from user iterator protocols. `iterator_dependencies.go` consumes canonical source/contained origins and live borrow records, preserving unknown backing. `iterator_tooling.go` exposes resolved protocol targets; it does not construct another slice/iterator model. |
  | FFI and buffer provenance | `ffi_legality.go` rejects native slices at C ABI boundaries. `foreign_extent_provenance.go` and `foreign_extent_quantity.go` use resolved compiler-known members and canonical storage origins for pointer/extent advice, retaining unknown provenance when no referent is established. No native slice descriptor ABI is inferred. |
  | Compiler-known members and strings | `compiler_known_members.go`, `compiler_known_presentation.go`, `explicit_ref_required.go` and `string_from_utf8_intrinsic.go` distinguish borrowed byte/rune sequences from owning arrays, preserve explicit borrowing and present the existing member surface. This recognition does not supply source-to-IR slice lowering. |
  | Contracts and callable identity | `contracts.go`, `unique_contracts.go` and `call_graph_identity.go` classify sequence elements and recursively preserve type identity. They consume established length/value facts; a slice-kind match alone is not a range or uniqueness proof. |
  | Parameter analysis and persistence | `parameter_usage_place.go` now consumes detached, analysis-time source-relative projections retained by `sequence_projections.go`, then uses the same slice/index composition helpers as Places. Its separate endpoint arithmetic is removed. This also prevents reevaluation in another callable's constant environment and new identity allocation during analysis consumption. `parameter_usage_persistence.go` remains schema 2: exact bounds persist, analysis-local identities/open-end relationships do not become imported proofs. `parameter_usage_operations.go`, `parameter_usage_contracts.go`, `parameter_usage_budget.go` and `parameter_usage_recommendations.go` consume sequence/reference capabilities conservatively. The size estimate in `parameter_usage.go` is an advisory cost heuristic, not a slice ABI/layout contract. |
  | LSP and diagnostic presentation | `cmd/lsp/sema_diagnostic.go`, `diagnostic_provenance.go`, `iterator_hover.go`, `signature_help.go` and compiler-known presentation consume Sema diagnostics/resolutions. Borrow diagnostics use canonical Place printing, including unknown starts; LSP does not normalize ranges or infer aliases. `internal/lsp/features/defaults` consumes Sema type/default facts without creating slices. |
  | Semantic IR | `internal/ir/semantic/slice_types.go`, `slice_ranges.go`, `slice_borrows.go`, `slice_length.go`, `slice_transfer.go` and `slice_index.go` implement the completed C-section contracts using P15 metadata and exact operand types. `slice_lowering.go` snapshots resolved plans only for explicit P16 builds and rejects unimplemented source producers by name. P16-26..28/P16-30..37 retain their separate verification and source-production work. |
  | MLIR and emission | `internal/lowering/secmlir/emitter_options.go` explicitly selects schema 12; default emission remains 11. `mlir/lib/Dialect/Sec/SecTypes.cpp` and `ReferenceBoundaries.cpp` admit the completed type/local-reference boundaries. `CanonicalProjectionSlice` remains the P15 compatibility category `deferred-slice`; it does not mean Semantic IR has no slice model. P16-40 registers the local slice operation contracts in `SecSliceOps.td`/`SliceOps.cpp`; P16-42..45 own whole-function verification and emission; P24 owns physical ABI representation. |

  Regression evidence: `TestParameterSlicePlacesUseCanonicalRanges` reads
  `testdata/sema/parameter_slice_places_valid.sec` for 32/64-bit target plans.
  It checks fixed/open/nested/CTE ranges, composed indices, dynamic parent
  identity, detached results and zero identity allocation even after the
  analyzer's constant environment changes. Existing slice, borrow, escape,
  iterator, foreign-buffer, persistence, IR and LSP tests cover the other rows.
  All section B audit/modernization points are complete; runtime acceptance
  remains in the explicitly open sections C–G.
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
- [x] P16-22 — Add fixed-array shared/mutable borrows and shared/mutable slice
  reborrows. Enforce writable source, no authority/range/lifetime expansion, and
  preservation of storage/epoch facts without copying elements (§§42–46).

  Completed 2026-10-20 (Semantic IR operations). `slice.borrow-shared`,
  `slice.borrow-mut`, `slice.reborrow-shared` and `slice.reborrow-mut` consume
  the source Place/slice and one normalized uint start/length tuple. Verification
  requires matching element/holder authority and a writable source for mutable
  results. Both normalized operands must come from the same range operation;
  its source extent is the exact fixed-array length or a length observation of
  the exact source slice SSA. Checked tuples stay on their successful edge.
  Construction preserves canonical storage/address-space and inherited
  validity/epoch/stability/pin facts, including fixed arrays reached through
  reference-derived sub-Places. Reborrows have a distinct borrow tied to the
  parent; dynamic sources require the existing P15 source validity guard.
  Borrow-end verification follows derived slices and their copied/moved
  holders so the child cannot outlive its parent, while independent shared
  copies and fresh loop holders retain their existing behavior.
  Deterministic printing carries all operands, guard and reference metadata.
  Tests cover 32/64-bit plans, both authorities and validity policies,
  weakening authority, zero/host-wide extents, referenced arrays, checked
  success/failure edges and malformed SSA, extent, epoch, pin, provenance,
  ownership and parent-lifetime contracts. No array read/copy/allocation is
  introduced. Fixed-array source production was added in P16-30; slice-source
  reborrow source production was added in P16-33. Complete CFG fact joins,
  conflict/return verification and MLIR operations remain their separate points.
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
  Source-to-operation production was added in P16-34; schema 12 and physical
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

  Progress 2026-10-21: canonical slice reference facts now propagate through
  block arguments when every incoming root agrees, including anchored loop
  forwarding. Rootless cycles and any unknown/conflicting predecessor remain
  unknown, independently of block order. Published facts are detached; length,
  copy/move/end and dynamic-guard consumers retain their existing obligations.
  P15 references borrowed from slice element/subfield Places must preserve
  the granting slice's validity/storage/epoch/pin/address facts and use a
  distinct borrow identity. This closes forged metadata paths that local
  operation-shape verification alone accepted. Source-derived corruption
  stops emission without bytes on both plans. Separate white-box forwarding
  of a source-built module verifies, emits schema 12 and round-trips through
  all four real slice/P15 passes. Broader joins and the remaining verifier
  integration keep this checkbox open;
  P16-27/P16-36 returned origins and source value production stay separate.

  Further progress 2026-10-21: range tuples, source slices and original indexes
  now retain exact proof identity through unambiguous CFG forwarding. A shared
  fixed-point provenance utility feeds both reference-fact propagation and a
  detached slice-verification view; generic SSA/dominance and holder lifetimes
  still inspect original IR. Different roots are not equated, even with equal
  constants/metadata, and the absent guard marker remains absent. P15 direct
  references/Places retain their own source-fact boundary rather than acquiring
  authority through slice canonicalization. Typed source-built IR transformed
  into forwarding cases emits and passes all four real verifiers before/after
  P6 on both plans. Swapped tuples stop emission without bytes. Those tests also
  exposed and fixed generic multi-argument successor syntax: cf operands now
  precede their shared type list. The cohesive printer lives in branch_targets.go.
  This closes the unambiguous forwarding work, not the whole P16-26 checklist.
- [x] P16-27 — Extend returned-origin/range verification using the represented P15
  return contracts. Add deterministic printing and defensive clones of slice
  metadata, with no physical descriptor vocabulary (amendment §§22,25–26).

  Completed 2026-10-27 after MD-115 (dialect v12.34, lowering v12.32;
  Semantic IR §§45(12a),126.22(1)). The existing ReturnOriginStep algebra now
  carries an exact arbitrary-precision half-open `[A..<B]` range or a genuinely
  indeterminate `[?]` range. Producer/transport/composition detach big integers;
  the existing Semantic IR and schema-12 printers use the canonical spelling.
  Source-relative exact nested ranges compose by addition; copies, moves,
  reborrows and direct calls preserve narrowing. Full-open reborrows preserve
  the current origin, including a prior range. The native MD-085 tracer proves
  endpoints from represented normalize/check SSA and preserves source/guard
  identity, rather than trusting return metadata. Its existing block-argument,
  finite-alternative and anchored-loop tracing remains fail-closed where no
  external origin or supported structural proof exists. Typed paths reject
  non-sequences, mismatched result elements and authority escalation. Schemas
  1–11 reject new range projections; default emission remains schema 11.

  Coverage: `return_origin_ranges_test.go` in Semantic IR and the emitter;
  `testdata/semantic_ir/slice_range_returns.sec`; native grammar, typed-negative
  and exact/unknown copy/move/call/loop fixtures. Exact, empty, nested, inclusive,
  omitted/dynamic endpoints, alternatives, huge endpoints and defensive-copy
  tests pass, as do source-to-schema-12 round trips on 32/64 plans through all
  four slice/reference verifiers before/after P6. Forged metadata is rejected
  without output. Other P16-26/P16-36 CFG/holder limitations and physical slice
  lowering retain their own owners; no P16 final acceptance is claimed.

- [x] P16-28 — Add positive/negative white-box IR tests, including mismatched SSA,
  misplaced guards, stale validation caught as a spatial error, missing proofs,
  widened reborrows, mutable copies and uses after borrow end (§§129,136).

  Completed 2026-10-20 (white-box Semantic IR scope). The composed matrix in
  `internal/ir/semantic/slice_guard_matrix_test.go` connects parent validity,
  range normalization, reborrow, separate child validity, runtime bounds,
  element Place/read and child/parent borrow ends. Fresh builds print
  deterministically for shared/mutable slices on 32/64-bit plans. Negative
  mutations first verify their baseline, then require the owning diagnostic
  reason: parent/child guard substitution, wrong source SSA, missing parent
  guard/proof, rewritten epoch, stale-to-spatial failure substitution,
  mismatched normalized tuple and derived Place use after parent end.

  | Required white-box coverage | Tests in `internal/ir/semantic/` |
  |---|---|
  | Shared/mutable types, exact/detached facts | `slice_types_test.go` |
  | Fixed-array borrows, all reborrow authorities, narrowing/source extent, empty/wide ranges | `slice_borrows_test.go` |
  | Shared copy, mutable copy rejection/move, borrow-end and forwarded-holder lifetime | `slice_transfer_test.go` |
  | Normalize/check, original wide endpoint SSA, canonical failure tuples and success/failure dominance | `slice_ranges_test.go` |
  | Unsigned length, wrong/missing/misplaced temporal guards and stale failure endpoints | `slice_length_test.go` |
  | Bounds predicate/element Place, source/index SSA, missing proofs, guard bypass, stale versus bounds failure and ended holders | `slice_index_test.go` |
  | Composed parent/child and temporal/spatial identities across the operation families | `slice_guard_matrix_test.go` |

  The corrected unsigned length contract supersedes the historical signed
  length test in §129. Epoch dependencies here are symbolic P15 identities;
  these tests do not choose a runtime epoch representation or claim physical
  64-bit epoch execution. Full verifier CFG/returned-origin work remains
  P16-26/P16-27, source tests P16-37 and executable MLIR integration P16-50.

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
  now remain P16-36 after P16-30..P16-35;
  schema 12 and physical lowering remain separate, and
  no CLI/default package capability is promoted.
- [x] P16-30 — Lower explicit fixed-array borrows from the source Place plus
  normalized range. Evaluate root/start/end once, in order, without array load,
  copy or allocation; support valid empty sources/ranges (§§42,78–80).

  Completed 2026-10-20. `slice_source_borrows.go` consumes the detached Sema
  creation plan and original operand schedule. P15 Place production is shared
  through `lowerBorrowPlace`, including stored-field/index projections and
  fixed arrays reached through direct reference holders. The source Place
  precedes present endpoints, evaluated once with their original integer
  types/signedness. Omitted bounds have no synthetic source expression.
  Proven ranges produce normalization without a redundant range check;
  shared/mutable construction uses resolved authority and retains source
  storage/reference facts. Empty fixed arrays and zero-length subranges keep
  valid origins. Fresh local slice holders can be explicitly discarded with
  `ref.end-borrow`, without releasing backing storage. General holder
  copy/move/signature/return and automatic end production remains P16-36.
  Source tests in `slice_source_borrows_test.go` cover 32/64-bit plans, all
  range forms, mutable/reference sources, fields and nested array Places,
  source-index/start/end call order and absence of aggregate loads/extracts.
  Default/P14/P15 routes remain unchanged. Slice-source reborrow was added in
  P16-33; fallible range, element use and MLIR emission remain capability gates.
- [x] P16-31 — Lower ordinary runtime range CFG to bounds panic with slice-range
  provenance, forming the slice only after success (§§20,80).

  Completed 2026-10-20. An unproven fixed-array range emits the existing total
  `slice.range-check` with original endpoint SSA and canonical core RangeError.
  Failed=true terminates through `fail.bounds operation="slice-range"`;
  failed=false alone consumes normalized start/length to construct the slice.
  The verifier admits this bounds provenance and rejects index provenance on
  an ordinary range-failure edge. Source tests retain int128/uint256 endpoints,
  inclusive/exclusive and omitted bounds, check successful verified IR, and
  reject reversed guard edges and incorrect panic provenance. P16-32
  subsequently adds fallible ranges without substituting panic; unsupported
  recovery-holder joins and heterogeneous failure sets still return no module
  after earlier supported functions. Schema-12 emission and binary
  acceptance remain separate.
- [x] P16-32 — Lower `try ref` / `try ref mut` range CFG to canonical RangeError,
  including local handlers and naked propagation. Preserve failure precedence
  and existing try metadata boundaries; no new handler schema (§§21–23,131).

  Completed 2026-10-20 for the existing source/metadata boundaries. The shared
  range producer routes its check's exact P11 RangeError result through the
  existing failure-set router instead of emitting bounds panic. Local
  catch-all, exhaustive variant and guarded/partial handlers use the ordinary
  try dispatcher; naked failures propagate through the exact enclosing
  Result error channel. Terminating/propagating handlers retain the original
  successful slice SSA and its borrow facts without a new holder or phi.
  The local handler dispatcher moved from builder.go to try_local_handlers.go.
  Tests cover shared/mutable construction, fixed-array sources and reborrows,
  wide signed/unsigned endpoints and evaluation order on 32/64-bit plans.
  Epoch-injected producer tests confirm temporal failure remains a separate
  reference-generation panic before fallible spatial checking; this is not
  Arena source or physical runtime evidence. Wrong range edges are rejected.
  Recovery-value holder joins remain explicitly gated by P16-26/P16-36,
  slice-valued Result signatures/returns remain P16-36, and heterogeneous
  error widening retains its existing failure-set boundary. These cases
  return no partial module. Schema-12 try metadata extensions (MD-031), slice
  emission and binary acceptance retain their separate owners.
- [x] P16-33 — Lower slice reborrow with source validity before spatial checks.
  Shared source yields shared only; mutable source can yield shared or mutable,
  preserving normalized range/origin facts (§§43–46,60–62).

  Completed 2026-10-20. The common `lowerSliceCreation` source producer now
  consumes slice creation plans for fixed arrays and slice reborrows. A named
  slice source retains its existing SSA holder and reference facts without a
  copy or move. Endpoints are evaluated once before any dynamic validity
  guard; guarded `slice.length` then supplies the exact source extent to the
  shared normalization/check producer. `slice.reborrow-shared` or
  `slice.reborrow-mut` consumes the successful tuple, retains the source borrow
  and canonical storage/epoch/address facts, and cannot expand authority.
  Proven ranges avoid redundant spatial checks. Source tests cover shared and
  mutable nested chains, mutable-to-shared weakening, empty slices, static and
  wide runtime ranges and endpoint call order on 32/64-bit plans.
  A Sema defect exposed by mutable -> mutable -> shared chains is fixed:
  access/creation follows unanimous registered granting-borrow ancestry within
  a canonical root. Unknown/cyclic/loop-carried links do not supply transitive
  proof, and sibling/parent conflicts remain errors (dedicated regressions).
  A separate producer-level white-box test injects dynamic epoch facts into a
  parameter, verifies calls -> validity -> length/range -> reborrow order and
  rejects replacing stale failure with bounds panic. This is not Arena source
  allocation or signature-production evidence. General projected/returned
  holders and signatures remain P16-36; P16-32 subsequently adds `try` source
  CFG within the existing metadata boundaries, and MLIR emission
  remains separate. Fresh child holders use the existing explicit end marker.
- [x] P16-34 — Lower `.Len` and proven `len(slice)` from resolved core operations,
  with dynamic validity and no element read. Both forms return `uint`; reject
  missing/incompatible resolved core facts with no partial IR (§§47–49,
  MD-074/MD-088).

  Completed 2026-10-20. Sema records detached `ResolvedSliceLengthPlan` facts
  at validated core `Len`/`len` resolution, including core identity and source/
  result types. Querying performs no lookup, inference or identity allocation,
  and analyzer reuse clears old decisions. P16 preflight snapshots these
  plans; `slice_source_length.go` consumes the actual source value once,
  validates it when required and emits the existing unsigned `slice.length`.
  No source holder copy/move or element read is introduced. An inline explicit
  borrow's temporary holder ends after the observation through the existing
  marker. Proven length observations remain @noPanic-compatible.
  Source tests cover both core forms, shared/mutable, nested/empty and inline
  operands on 32/64-bit plans, plus missing resolution returning no module
  after preceding supported functions. Separate producer-level white-box
  tests install epoch-qualified parameters for all four spelling/authority
  combinations and verify source guards; this does not implement P16-36
  signatures or Arena source allocation. The historical signed-length clause
  is superseded by the accepted unsigned length correction (2026-10-17);
  there is no signed length operation or representability gate. General
  projected/returned holders and signatures remain P16-36, and MLIR emission
  and physical lowering retain their separate ownership.
- [x] P16-35 — Lower ordinary/proven/fallible indexes to element Places with
  IndexError.OutOfBounds or slice-index bounds panic as appropriate. Evaluate
  source/index once, temporal validity first, and Place before write RHS (§§50–58).

  Completed 2026-10-20 within P15's existing trivial-value and source-holder
  boundaries. `slice_source_index.go` consumes detached index/use plans to
  form `slice.element-place`, then uses P15 reads, writes and explicit shared/
  mutable element borrows. Runtime predicates consume the original signed or
  unsigned index, including int128/uint256, and the successful Place reuses
  exactly those source/index SSA values. Proven indexes need no bounds branch.
  Ordinary failure retains slice-index bounds-panic provenance; fallible
  reads route IndexError.OutOfBounds through the existing failure-set router
  and handler/Result machinery, including recovery values. A write establishes
  its guarded destination Place before evaluating the RHS and commits once.
  Slice-derived element references and their direct-reference copies/moves
  cannot survive the parent holder's end; existing independent shared-copy
  boundaries remain intact. Source tests cover both authorities, trivial
  reads/writes and element borrows, empty slices, reborrowed sources, specific
  and catch-all handlers, naked propagation and @noPanic proven reads on
  32/64-bit plans. Wrong bounds edges and early parent ends are rejected.
  Separate epoch-injected parameter tests exercise the real producer's
  index -> validity -> bounds -> Place -> RHS order and retain stale-reference
  failure outside try. This does not implement source signatures, Arena
  allocation or physical runtime execution. Nontrivial transfers/replacement,
  reference-valued element reads outside P15's value surface, general slice
  holders/signatures, automatic ends and remaining try/metadata dependencies
  keep their named P17/P16-36/P15 boundaries; unsupported builds return no
  partial module. Slice emission and binary acceptance remain separate.
  Further completed 2026-10-26: trivial integer slice element compound
  assignments now establish the guarded mutable element Place once, read it
  before evaluating the RHS, apply the Sema-resolved checked operator, then
  write that same Place only on success. All ten `op=` forms and `++`/`--`
  aliases use existing P15/checked arithmetic operations. The shared operator
  value producer and local assignment builder now live in the focused
  `compound_assignment.go`, reducing builder.go without changing local behavior.
  `slice_compound_assignment.sec` covers named and returned sources, runtime
  indexes, arithmetic/bitwise/shifts and uint256 elements. IR tests check exact
  single-evaluation order and failure paths without writes; emitter tests pass
  native schema-12 round trips through all four verifiers before/after P6 on
  32/64 plans. Invalid source rejects shared authority, wrong operand types
  and live shared child borrows without partial IR. Nontrivial replacement,
  fallible contract assignment and physical lowering remain separate.

- [ ] P16-36 — Lower copy/move, parameters, permitted returned slices, projected
  origins and borrow ends from Sema plans. Retain exact static subranges and
  conservative runtime origins; reject local escape (§§39–41,63–67).

  Progress 2026-10-23:
  - **Parameters.** `ref T[]` / `ref mut T[]` parameters are P16 slice types with Sema's parameter reference facts, so length, index, element Place and write work on them (`slice_parameters.go`).
  - **Arguments.** At a call:
    - a fresh `ref data[..]` argument is built in place;
    - an existing shared holder is passed with `slice.copy-shared` under its own borrow;
    - an existing mutable holder is passed with an implicit full-range `slice.reborrow-mut` under the pass's new borrow, so the caller keeps it.

    Call argument actions are `shared-reference` / `mutable-reference`, matching the Semantic IR verifier and the dialect's `verifyArgumentActions` (v12.5; lit `invalid-schema12-slice-call-action.mlir`).
  - **Holder transfers.** `let copy := shared` is `slice.copy-shared`, and `let moved :<- mutable` is `slice.move-mut` from Sema's transfer plan. `ref holder` / `ref mut holder` reborrow the complete extent (`slice_holder_transfers.go`).
  - **Returns.** A slice derived from a slice parameter can be returned as the holder, a copy, a full-extent reborrow or a call result. The summary is `returns-from=[param#0]`, the verifier traces through slice copy/move/whole-range reborrow, and callers get the instantiated facts. Sub-slice returns fail closed on MD-115, which has no return-origin slice projection spelling.
  - **Tests and fixtures.** The new fixtures `slice_source_parameters.sec`, `slice_source_transfers.sec` and `slice_source_returns.sec` emit schema 12 and pass all four real verifiers on 32/64-bit plans (`TestPackage16EmitSourceSlices`). The obsolete fixtures `package16_unsupported/{shared_copy,mutable_move,holder_reborrow}.sec` were removed.
  - **Automatic local ends (verified 2026-10-23).** The existing Sema schedule and shared borrow-end producer also handle local slices in their own statement list. `slice_source_automatic_ends.sec` covers shared holders/copies, mutable moves, child reborrows and unused holders before subsequent backing-array writes. `TestPackage16AutomaticSliceBorrowEnds` checks the resolved endpoints, including ending the surviving moved holder; `TestPackage16EmitSourceSlices` checks the complete source-to-schema-12 chain on both 32/64 plans before/after P6. These tests contain no explicit `discard` and inject no IR facts.
  - **Mutable-to-shared calls (2026-10-25).** A named `ref mut T[]` holder may grant a whole-range shared reborrow to a selected `ref T[]` parameter (`borrowing.md` §15.3(2)). Sema preserves the source type for overload ranking, checks all call reservations and publishes shared pass facts; `slice_parameters.go` emits `slice.reborrow-shared` with a distinct resolved borrow/lifetime, retaining the mutable parent. The parent may be used mutably after a non-retaining call; a returned shared dependency keeps its normal lifetime. `slice_shared_call_reborrow.sec` and focused Sema/IR/emitter tests cover both widths, return composition and exact mutable overloads, then pass all four native verifiers before/after P6. Reusing the parent's borrow identity returns no emitted bytes. Negative source tests retain overlap, returned-dependency, shared-to-mutable, element-type and storage-initialization restrictions. Generic candidate selection is covered in Sema only; generic IR production retains its explicit capability boundary. Fresh/non-holder mutable-to-shared slice arguments are covered by the next completed step.
  - **Expression call arguments (2026-10-25).** `Read(ref mut data[start..<end])` and `Read(IdentityMutable(view))` now grant a separate shared pass using canonical Sema referent origins and distinct call-pass borrow/lifetime identities; the original creation/return facts remain intact. Endpoints and nested calls evaluate once and in order. Exact mutable overloads retain priority; returned shared dependencies still prevent parent mutation and local escape. `slice_expression_call_reborrow.sec`, the five-case `slice_expression_call_invalid.sec`, Sema pass-identity checks, `TestPackage16SliceExpressionCallReborrow` and the expanded `TestPackage16EmitSharedSliceCallReborrow` cover 32/64 plans, deterministic reanalysis, native round trips through all four verifiers before/after P6, and malformed child identities producing no bytes. This closes non-holder mutable-to-shared slice argument adaptation; projected holders, general CFG borrow ends, generic IR and MD-115 remain separate boundaries.
  - **Returned expression sources (2026-10-26).** `ref Same(view)[start..<end]`, `ref mut Writable(view)[..]`, `Same(view).Len`, `len(Same(view))` and `Same(view)[index]` now consume the call result once with its instantiated Sema origin/borrow facts. Shared/mutable and authority-reducing reborrows retain the call result as source; endpoint/index expressions run once after it. Writes through the named mutable child are supported. `slice_returned_expression_sources.sec`, `TestPackage16ReturnedSliceSources` and `TestPackage16EmitReturnedSliceSources` verify exact source SSA/borrow links, operation order, repeat-build determinism and real schema-12 round trips through all four verifiers before/after P6 on 32/64 plans. Corrupt child/source identity emits no bytes. `TestPackage16ReturnedSourceRestrictions` preserves authority, retained-parent, descending-range and local-escape errors and the MD-115 gate for a narrowed returned call slice. Direct call-result index writes were completed by the next step below. The full-open returned-call identity projection was eliminated by the subsequent full-open normalization step below; actual narrowed-range serialization stays separate. No generalized projected holder, return-summary grammar, CFG endpoint or physical lowering capability is implied.
  - **Direct returned-slice stores (2026-10-26).** `Writable(view)[index] = rhs` now projects the returned mutable slice's canonical referent Place. Call origins preserve the actual argument's granting holder through nested calls, so local mutable holders keep their authority without bypassing child/sibling conflicts. Owning array temporaries and shared returned slices remain unassignable. `slice_call_index_assignment.sec` and Sema/IR/emitter tests cover parameter/local/nested calls, disjoint ranges, exact single-evaluation source/index/Place/RHS/write ordering, deterministic IR and native schema-12 round trips through all four verifiers before/after P6 on 32/64 plans. Separate Sema-only finite-origin tests retain both granting holders and composed exact indices; they do not imply broader join lowering. Invalid fixtures preserve overlapping shared/child borrow rejections.
  - **Full-open returned origins (2026-10-26).** `return ref Same(view)[..]` and its mutable, authority-reducing, named-source and nested-call forms now preserve the source's complete canonical origin without an extra identity range projection. `slice_full_open_returns.sec` and focused IR/emitter tests check exact parameter-root summaries, single evaluation and stable 32/64 native schema-12 round trips before/after P6. Sema tests retain exact/empty/dynamic source intervals and every alternative; they do not claim broader join lowering. A full-open reborrow over an actually narrowed source still requires MD-115 for return serialization, and shared-to-mutable escalation/local escape remain errors. This changes no dialect spelling and requires no new decision.
  - **Nested automatic ends (2026-10-26).** Sema-published slice-holder endpoints inside nested statement lists now produce their own `ref.end-borrow` markers. A focused post-build CFG pass tracks only which published ends have executed, resets at a fresh SSA definition and removes redundant join candidates. At a mixed join before any real instruction it splits uniformly live incoming edges and preserves their arguments; more complex unresolved layouts fail clearly at P16-36/MD-065. `slice_branch_ends.sec` covers shared/mutable branches, mixed joins, nested and terminal branches and iteration-local redefinitions. IR tests traverse each reachable path to require exactly one end before owner writes and deterministic builds on 32/64 plans; native schema-12 round trips pass all four verifiers before/after P6. Required future uses still prevent release, and a forged endpoint lifetime emits no bytes. The pre-fix fixture had no end at either branch's owner write; the mixed form ended only after those writes at the merge.
  - **Structural early-return liveness (2026-10-26).** Sema now stops the existing suffix-use scan at a structurally proven function exit after checking its operands and dependencies, so an early-return branch does not inherit unreachable future uses from the outer continuation. The same shared structural termination predicate is used without relying on unresolved future flow facts; break/continue do not incorrectly terminate the search. `borrow_return_liveness_test.go` verifies separate nested and continuing endpoints on both target plans. Extended `slice_branch_ends.sec` and its IR/native matrices cover shared/mutable, nested and branch-unused early-return cases, exactly one resolved end per path before owner writes, deterministic builds and native schema-12 round trips through all four verifiers before/after P6. Returning the holder itself, deferred uses and loopback dependencies retain conflicts (`slice_early_return_liveness_invalid.sec`). No new IR endpoint inference or dialect text is involved.
  - **Transparent nested mixed joins (2026-10-26).** A mixed incoming path may now be traced backward through parameter-free blocks containing only an unconditional branch. `slice_borrow_end_edges.go` first plans the complete set of uniformly live incoming edges without mutation; `slice_borrow_ends.go` then places the existing Sema-published marker on those edges, retaining original branch arguments. It crosses no real instruction or parameter definition, and cycles remain a named unsupported boundary. `Transparent`/`DeepTransparent` in `slice_branch_ends.sec` previously failed the P16-36/MD-065 edge-schedule gate; they now pass exact once-per-path end/owner-write checks, deterministic builds and 32/64 native schema-12 round trips through all four verifiers before/after P6. Focused negative planner tests preserve instruction/parameter/cycle boundaries and verify no CFG change on rejected planning.
  - **MD-115 completed (2026-10-27).** Source-returned fixed-array/slice
    subranges now carry canonical exact `[A..<B]` or genuinely dynamic `[?]`
    origins through Semantic IR and explicit schema 12, including empty ranges,
    nested composition, copies/moves, call results and outer full-open reborrows.
    P16-27 is checked above; broader holder/CFG and physical lowering work stays
    open. This supersedes the historical MD-115 gates in earlier progress notes.
  - **Remaining.** Projected holders (a slice in a struct field, which needs P17 aggregates with references), non-structural termination/loop-control schedules and mixed CFG paths crossing real instructions, parameter definitions or cycles. Direct P15 reference endpoints keep their own implementation boundary.
- [x] P16-37 — Add source-to-IR tests for every supported path and every deferred
  dependency. Prove operand order, SSA reuse, no redundant proven checks, no
  aggregate copies, no partial modules and unchanged predecessor paths (§§129–137).

  Completed 2026-10-20 for the current source capability boundary. The
  source matrix in `slice_source_matrix_test.go` analyzes six fixture groups
  with actual 32/64-bit scalar plans and builds verified IR directly, without
  hand-editing IR or injecting epoch facts. Its operation inventory covers
  shared/mutable construction and reborrow, proven/runtime/fallible ranges,
  both uint length forms, proven/runtime/fallible indexes, P15 element reads,
  writes/borrows and explicit holder ends. Counts are checked against detached
  Sema plans so omitted or redundant source checks/observations cannot silently
  reduce coverage. Repeat builds retain printed IR and semantic identities;
  target widths and slice source locations survive lowering, and backing
  element/aggregate copies and implicit holder transfers remain absent.
  Existing focused source tests prove operand/RHS order, exact SSA reuse and
  wrong-edge/borrow-end rejection; separate epoch-injected producer tests
  remain identified as white-box evidence, not source allocation evidence.
  The 32/64 dependency matrix includes a supported preceding function and
  checks nil modules with named capability errors for dynamic owners,
  signatures, returns, projections, holder copy/move/reborrow, recovery joins,
  heterogeneous errors, unsupported element values and custom-free lifecycle
  replacement. Deferred holder expressions now stop at a named P16-36 gate
  instead of falling into direct-reference P15 lowering. Existing missing-plan,
  module-selection, Sema invalid-source and predecessor/default tests remain
  green. P16-36 source producers, complete P16-26/P16-27 verification and
  schema-12 source-to-tool/binary acceptance retain their separate items;
  those capabilities are tested as unsupported here, not marked implemented.

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
  P16-39/P16-40/P16-41 subsequently add slice types, operations and local
  reference/bounds contracts; whole-function guard/borrow verifiers and
  emission remain P16-42..P16-45; schema registration
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
- [x] P16-40 — Add range_normalize/range_check, borrow_shared/borrow_mut,
  reborrow_shared/reborrow_mut, copy_shared/move_mut, len,
  index_in_bounds/element_place with synchronized P15 metadata (§§91–103).

  Completed 2026-10-20. All eleven schema-12 slice operations have registered
  ODS forms and local verifiers for endpoint arity/signedness, target-sized
  unsigned counts, proof kinds, canonical core RangeError, element identity,
  shared/mutable authority, borrow identities and canonical exact range facts.
  P15 reference/bounds/integer helpers are shared through OperationContracts.h;
  SliceOps.cpp owns the slice contracts. Scalar layout resolves count/result
  carriers on 32/64-bit plans while retaining original wide endpoint/index
  carriers. Round-trip and negative tests cover every operation and preserve
  explicit unsupported-lowering rejection with no output artifact. The full
  MLIR suite passes 129/129. Len remains unsigned under MD-074/MD-088; no
  signed len_int operation is introduced. Whole-function guards/borrow checks
  remain P16-42/P16-43, source emission P16-44/P16-45, and physical
  representation P24. Default emission remains schema 11.
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
- [x] P16-42 — Register `--sec-verify-slice-guards`. Verify same-source guard
  dominance, range normalization/error precedence, index bounds and element
  Place formation; require dynamic validity before spatial guards (§§105–110).

  Completed 2026-10-20. The registered function pass verifies canonical
  dedicated-edge range CFGs, confines normalized start/length to success and
  RangeError to failure (including forwarded block arguments), and checks
  canonical bounds-failure provenance. Construction must consume one exact
  normalization tuple bound to its fixed-array extent or the same source
  slice's length; reborrows cannot substitute another source's bounds.
  Runtime element Places require a matching slice/index bounds-true edge;
  index failure paths must reach slice-index panic or canonical IndexError
  flow. Proven forms retain the local operation verifier's proof vocabulary.
  Dynamic length, range, index and reborrow observations require same-source
  validity dominance with an independent stale-reference endpoint.
  Unambiguous SSA forwarding is retained; distinct unknown phi origins cannot
  establish equality. P15 reference facts and edge-dominance helpers are
  shared via ReferenceFacts.h, extracted from VerifyPlaceReferences.cpp.
  Tests cover shared/mutable operations, ordinary/fallible paths, forwarded
  values, indirect failure paths, temporal ordering, malformed joins and
  rejection without an output artifact on 32/64-bit plans, including P6.
  Canonical RangeError classes/order remain owned by the operation verifier;
  this pass consumes represented proof rather than repeating Sema analysis.
  Full MLIR suite passes 132/132 and focused Go IR/lowering/compiler tests
  pass. P16-43's complete P15 verifier integration, P16-44 emission and
  P16-45's complete integrated matrix remain separate.
- [x] P16-43 — Extend P15's place, reference-guard and borrow verifiers for slice
  operands, narrowing, shared copy/mutable transfer and returned-range facts.
  Do not duplicate source borrow analysis (§§83–86,116).

  Completed 2026-10-21 for represented schema-12 contracts. All three P15
  verifier entry points now consume the shared slice guard contracts; Place
  consumers accept fixed-array slice borrows without allowing Place escape.
  The borrow verifier checks complete source reference facts for shared copy,
  mutable move, reborrow, element Places and element references, including
  borrow/lifetime, storage, validity/proof, epoch/domain, stability, pin and
  address space. Owned fixed-array roots retain represented storage/address
  facts; transfers cannot contradict represented range facts. An ordinary P15
  element borrow may derive implicitly through its Place operand, preserving
  that existing encoding instead of requiring new metadata syntax.
  A focused VerifySliceReferences.cpp follows CFG holder aliases and derived
  Places/references through end/move boundaries, preserves independent shared
  holders, and distinguishes loop redefinition from reuse of an ended holder.
  ReferenceFlow.h shares P42's conservative SSA forwarding; unknown facts do
  not grant authority. Returned slice/element origins follow existing P15
  parameter-root summaries through construction, transfer and narrowing;
  local roots and unlisted parameters are rejected. The provisional metadata
  spelling and root-level return verification depth remain MD-084/MD-085,
  owned by P16-07; no new path/callee-summary policy is chosen here.
  Positive/negative MLIR tests cover identities, epochs, pin/storage/address
  contradictions, CFG aliases/loops, parent/child/element ends, mutable moves,
  independent copies and returned local/nonlocal/narrowed origins. Existing
  invalid guard tests also run through both P15 Place/reference guard passes.
  Actual source matrices now pass all slice/P15 verifiers (P16-50); malformed
  emitted transfers fail without leaving output. MLIR 136/136; complete
  Sema/IR/lowering, targeted compiler and vet checks pass. Source holder/return
  production, complete IR verification and physical lowering retain their
  P16-36, P16-26/P16-27 and P24 owners.

- [x] P16-44 — Emit schema-12 slice operations only from verified Semantic IR,
  preserving TypeIDs, source locations, exact ranges, dynamic identities, CFG,
  borrow/lifetime/storage/epoch facts and target plans; unsupported input emits
  no artifact (lowering-v12 §§1–31).

  Completed 2026-10-21. Explicit schema-12 emission maps all eleven represented
  slice operations and shared/mutable types through the existing emitter,
  preserving original operand/result types, CFG, locations and P15 reference
  facts. Source construction now attaches detached SliceFacts to IR operations:
  exact source-relative ranges, dynamic range IDs, source Place and normalized
  length SSA survive verification/printing and emit through the existing P16
  provenance attributes. Contradictory value facts are rejected before output.
  Cohesive emission and boundary checks live in slice.go/slice_boundaries.go;
  common proof/try/match attribute emission was extracted into
  operation_attributes.go so slice-specific metadata shares its dictionary;
  foreign/physical slice carriers and stale scalar plans remain rejected.
  Real source→Sema→verified IR→schema-12 tests cover 32/64 plans, empty/nested
  shared/mutable borrows, ranges, .Len/len, index read/write/element references,
  ordinary/fallible errors, wide and nominal elements/endpoints/indexes.
  Outputs round-trip through the real tool and pass P42 before/after P6.
  Separate white-box tests cover shared copy/mutable move and epoch-qualified
  length observation without claiming source Arena/signature production.
  The bridge exposed two local verifier gaps: a fixed-array Place rooted in a
  reference must retain its granting borrow, and slice endpoints/indexes may
  retain nominal integer wrappers. These contracts are corrected without
  admitting nominal operands to unrelated checked-integer operations.
  Guarded/residual try metadata remains MD-031; invalid IR, schema 11, foreign
  slice ABI and target mismatch return no bytes. Physical lowering rejects the
  emitted slices by name without leaving an artifact. Default schema 11 and
  source builder capability gates remain unchanged. Full Sema/IR/lowering and
  targeted compiler/vet checks pass; MLIR passes 132/132. Complete P15 verifier
  integration and final integrated acceptance remain P16-43/P16-45/P16-50.
- [x] P16-45 — Add MLIR round-trip and invalid suites for types/ops, metadata,
  range/index/reference guards, authority, move/copy/end use, empty slices,
  unknown versions and schema-11 regression (§§138–139).

  Completed 2026-10-21. `slice_verification_matrix_test.go` composes all
  slice/P15 verifiers across the nine canonical negative lit suites: types,
  local operations, authority, aggregate copies, reference boundaries, type
  and operation version gates, spatial/temporal guards and borrow metadata/
  lifetime/return flow. Each of 145 split inputs must fail with its original
  owning diagnostic, no stdout IR and no output artifact, in pipelines with
  and without P6 scalar resolution. Parse/local-verifier errors stop at their
  proper boundary; CFG/borrow negatives also run after P6. Lit retains exact
  diagnostic-location checks; the matrix reuses those inputs rather than
  maintaining duplicate malformed contracts. The authority fixture now also
  uses a precise expected diagnostic.
  Five whole-function-valid positive fixtures cover slice types/metadata,
  shared aggregate fields, guard/borrow flows and unchanged schema-11 P15
  metadata. Each passes all four verifiers and deterministic repeated
  round-trips in both pipelines. Nested struct slice elements extend the
  existing type fixture. Unknown versions 0/13/-1 and a schema-11 downgrade
  fail without artifacts, while the positive P15 fixture retains schema 11.
  Actual source tests now also cover an empty mutable fixed-array borrow and
  empty mutable reborrow on both 32/64 plans, preserving their provenance and
  explicit ends through verified schema-12 emission. Existing all-operation,
  wide/nominal, range/index/failure and copy/move tests remain in force.
  Full Sema/IR/lowering and targeted vet pass; MLIR 136/136. This completes
  integrated MLIR verification coverage, not general source holder production,
  physical slice lowering or the final isolated P16-55 acceptance gate.


## F. Prove predecessor integration and architecture boundaries

- [x] P16-46 — P6/P8: test 32/64 target scalar resolution of slice elements and
  length results while preserving wrappers; integer normalization must not
  recursively rewrite slice elements or lower slice semantics (§§112–113).

  Completed 2026-10-20. P6 now resolves target-sized scalar leaves through
  shared/mutable slice wrappers, retaining nested nominal/array identities
  and arbitrary-precision array extents. Recursive semantic container
  conversions moved from SecToCore.cpp into ScalarContainers.cpp; callbacks
  capture the context by value and retain the owning converter by reference.
  Real-tool 32/64 lit tests prove P6 preserves wrappers, authority and existing
  reference metadata/end markers, and P8 retains signed/unsigned semantic
  element types rather than recursively converting them to signless types.
  The canonical uint count carrier resolves to ui32/ui64 under P6; its
  standalone scalar path may normalize under P8 while slice elements remain
  wrapped. Full MLIR tests pass 126/126 and focused Go compiler/lowering
  predecessor tests pass. P16-40 subsequently registers the local length
  operation contract; actual slice emission remains P16-44; this is type/metadata and scalar-carrier evidence, not physical
  slice lowering or source-to-binary acceptance.
- [x] P16-47 — P13/P14: test shared slice fields/contained origins and mutable
  slice aggregate move-only classification; fixed-array Place sources, exact
  lengths and no array/aggregate copies (§§114–115,140).

  Completed 2026-10-21. P13/P14 compatibility tests now cover recursive
  shared/mutable slice fields, nested structs/arrays and shared slices whose
  referents are move-only. Source Sema tests retain contained origins, exact
  subranges, storage/address/epoch dependencies across field extraction,
  holder copies and returned fields; local-field escape and implicit nested
  exclusive-holder copy remain rejected. Hand-built IR exposed a verifier
  gap: forged trivial struct classifications and copy-trivial operations could
  duplicate mutable slice holders. Recursive owned-field checks now reject
  those classifications and construct/spread/extract/replacement paths in
  Semantic IR and MLIR; shared/reference referents stop the recursion.
  Move-aware aggregate construction retains its P17 capability boundary.
  Cohesive struct verification moved from verifier.go/SecOps.cpp into focused
  sibling files, with common MLIR field checks shared by existing users.
  Actual 32/64 source→verified IR→schema-12 round-trip tests cover nested
  fixed-array fields, mutable referenced fields and empty fields: exact array
  extents and granting borrows survive without array/aggregate copies.
  Handwritten MLIR round-trips all four shared struct operations and rejects
  35 direct/nested/nominal/array/Result exclusive-holder cases. Corrupted IR
  emits no bytes. General source slice-holder fields/transfers/returns remain
  P16-36, full ownership integration P16-43 and physical lowering P24.
  Full Sema/IR/lowering and targeted compiler/vet checks pass; MLIR 134/134.

- [x] P16-48 — P15/P12: test parameter/return origins, nested element refs,
  matching lifetimes/epochs, dynamic guard ordering and ordinary slice values
  in existing value paths without adding slice patterns (§§116–117,140).

  Completed 2026-10-25 (Claude) for the current source capability boundary:
  - **Parameter and return origins.** Element references through slice parameters are returned with `returns-from=[param#N[*]]`. This is always the conservative `[*]`, because a slice element is bounds-checked even for a constant index (§ 67). A field of a slice element extends it to `param#0[*].field#1`. The IR verifier follows `slice.element-place`, and the native tracer agrees (`TestPackage16SliceElementReferenceOrigins`).
  - **Nested element refs.** `ref s[i].f` / `ref mut s[i].f` borrow a field/array sub-Place below the slice element Place. The source, index validity and bounds guard are lowered once (`slice_element_subplaces.go`). Such a borrow can be passed to a `ref` parameter.
  - **Matching lifetimes/epochs and dynamic guard order.** Arguments and reborrows keep the holder's storage, epoch and domain facts under a new borrow derived from it. In an epoch-injected white-box build of a source function, each implicit argument reborrow of a dynamic-epoch slice parameter is preceded by `ref.is-valid`, and its length/range observation uses that guard. Removing the guard is rejected (`TestPackage16SliceArgumentDynamicGuardOrder`). Source Arena slices remain P18.
  - **Slice values in value paths.** Besides let copy/move, arguments, returns and `try` ranges, union payloads and struct fields hold slices: construction copies the shared holder, and the aggregate is returned and copied as an ordinary value (`slice_source_aggregates.sec`, `TestPackage16SliceValuesInAggregates`). No slice patterns are added.
  - **Emission.** `slice_source_element_origins.sec` and `slice_source_aggregates.sec` emit schema 12 through all four real verifiers on 32/64-bit plans (`TestPackage16EmitSourceSlices`).
  - **Boundary.** Reading a contained slice or reference back out of an aggregate (a pattern binding or field read) needs P15 § 101 contained-reference facts per aggregate path, which have no Semantic IR or MLIR representation yet. It fails closed without a module. The same dependency blocks P16-36 projected holders.
- [x] P16-49 — Exercise int128/int256/uint128/uint256/decimal128 elements and
  wide endpoints/indexes where source rules allow, with target-aware length
  proofs, empty sources and conservative dynamic ranges (§§3,126–128,140).

  Completed 2026-10-20. Ten source functions exercise all five element
  carriers on actual 32/64-bit scalar plans: proven element reads, mutable
  writes, fallible wide ranges/indexes, recovery values, shared reborrow,
  boundary-empty ranges and zero-length backing arrays. All four wide integer
  families occur as original endpoints and indexes; their SSA values are
  consumed directly without a target-width cast. Dynamic ranges keep symbolic
  identities and unknown extents rather than inventing exact provenance.
  Length remains target-sized uint independently of the element carrier,
  proven accesses add no redundant spatial check, and the source guarantees
  remain @noPanic. Real P6/P8 lit tests retain every wide integer and decimal128
  element under shared/mutable wrappers on both plans (MLIR suite 127/127).
  The composed source tests exposed and fixed a Sema origin loss after
  `try ref`: only a resolved success-only continuation may forward its
  protected slice Place. Recovery-value and unresolved plans do not establish
  that proof; local-origin escape remains rejected. The cohesive origin
  producer moved out of analyzer.go into reference_place_origins.go. Dedicated
  Sema and IR regressions cover the fix. Actual slice-operation emission,
  physical representation, decimal arithmetic and binary acceptance retain
  their existing owners; this does not claim those deferred capabilities.
- [x] P16-50 — Add source → target-aware Sema → verified Semantic IR → schema-12
  → real sec-mlir-opt tests on both 32/64 plans using absolute tooling and all
  slice/P15 verifier passes. Keep dynamic-epoch white-box evidence distinct
  from source evidence while Arena allocation IR remains deferred (§141).

  Completed 2026-10-21 alongside P16-43. Eight actual source fixtures build
  through target-aware Sema and verified Semantic IR to explicit schema 12 on
  both 32/64 plans. Absolute configured tooling runs slice guards, Places,
  reference guards and borrow semantics before/after P6 scalar resolution,
  with repeated verification and canonical round-trips. Coverage includes
  empty/static/dynamic/nested ranges, shared/mutable borrows and reborrows,
  lengths, ordinary/fallible index/range paths, element reads/writes/references,
  wide/nominal carriers, nested fixed-array fields and disjoint mutable ranges.
  Separate white-box tests exercise epoch/pin/storage-bearing holder transfers
  without backing observation and guarded dynamic length; they do not claim
  source Arena allocation or general slice signature/return production.
  Corrupted emitted lifetime/epoch facts fail the real borrow verifier without
  an artifact on both plans. Physical slice lowering still fails by name,
  default schema remains 11, and MD-031 unsupported handler plans stay gated.

- [x] P16-51 — Test unsupported dynamic owners, raw pointers, physical/FFI
  descriptors, handles, equality, iteration/membership/Reverse, non-trivial
  element operations and implicit signed-length narrowing, each with no partial artifact
  (§§70–75,121–124,142). Do not claim P17/P18 lowering from legacy support.

  Completed 2026-10-21. The 32/64-bit unsupported-source matrix distinguishes
  owning-array and slice value capability gates from mandatory source errors:
  pointer extraction, membership, iteration, mutable Reverse, equality/ordering,
  raw-pointer slicing, foreign slice parameters/returns, move-only reads/moves,
  nontrivial replacement, unsupported text elements and both implicit signed
  length forms. Ordinary user-declared StableHandle/WeakHandle structs gain no
  slice API from their names; no public handle syntax or representation is
  invented. Every IR-stage failure retains its source location and returns no
  partial module after an earlier supported function. Compiler-known pointer,
  Reverse and resolved membership operations now have specific P16 capability
  diagnostics instead of generic missing-member/call errors.
  Separate verified-IR cases cover shared/mutable foreign parameters/returns
  and nested struct/union carriers, plus explicit physical layout requests;
  each returns the owning error and no schema bytes. Actual emitted source
  passes all four slice/P15 verifiers before the real support gate, composed
  LLVM pipeline and P8 boundary reject physical representation without output
  files on either target plan. (MD-087, 2026-10-22: P8 now lowers such
  modules selectively instead; the support gate and composed LLVM pipeline
  remain the physical boundaries.) CLI tests add a real supported entry and check
  build/emit-mlir/emit-llvm error classes and absence of stdout/artifacts for
  sixteen source boundaries. Default schema remains 11; these tests do not
  promote P17/P18/P24 or source holder/signature support.
- [x] P16-52 — Assert successful P16 modules contain no physical pointer/length
  aggregate, GEP/stride arithmetic, hidden ABI arguments, runtime borrow counters,
  new generation manager, hidden allocation, undef or poison (§143).

  Completed 2026-10-20. `slice_architecture_test.go` audits verified Semantic
  IR from six source fixtures on actual 32/64-bit Sema plans: fixed-array
  borrows, empty ranges, nested shared/mutable reborrows, lengths, ordinary and
  fallible range/index access, element references and wide numeric elements.
  Closed semantic type/operation inventories reject physical/runtime additions;
  source declarations independently constrain function/parameter counts,
  aggregate fields and direct-call targets/counts. Only semantic array storage
  is admitted in these fixtures, so borrow/generation counters cannot acquire
  storage. Separate white-box dynamic-epoch modules cover guarded access,
  length and holder transfer for both authorities without a runtime object,
  helper call or hidden argument. These are not Arena source-allocation tests.
  `schema12-slice-architecture.mlir` audits all eleven handwritten local slice
  operation contracts after P6 on 32/64 plans, preserving exact signatures
  while forbidding physical descriptors, LLVM/memref operations, address
  arithmetic, allocation/helper calls, atomics, undef and poison throughout
  the output. P8 rejected unresolved reference representation without an
  output artifact; MD-087 (2026-10-22) replaces that with selective lowering
  that keeps the slice operations high-level. Full MLIR suite passes 130/130. Source-to-schema emission,
  complete guard/borrow verification and physical execution remain separate.
- [x] P16-53 — Rebuild plans/modules with fresh analyzers and reordered metadata.
  Verify deterministic identities, origin ordering, exact range printing, source
  diagnostics, and independent 32/64 outputs without mutating shared input.

  Completed 2026-10-20. `slice_determinism_test.go` builds six supported source
  fixtures with fresh analyzers on interleaved 64/32/64/32 plans, comparing
  complete IR and detached creation/index/length facts. Reversed syntax-order
  queries and metadata-map insertion retain the same verified IR and ordered
  facts. Independent reparsing also reproduces the output, excluding syntax
  addresses as semantic identities. JSON snapshots check that these fixtures'
  shared ASTs are unchanged; prior modules retain their text and target facts
  after later analyses/builds. Abstract printed IR may legitimately match
  across plans; the modules retain their independent PointerWidthBits.
  A separate Sema fixture checks two possible backing roots in stable order
  with exact composed ranges left[2..<4] and right[3..<5]; slice-valued
  signatures still retain their P16-36 source-to-IR gate. Four invalid fixtures
  reproduce complete diagnostics, locations, help and related-cause ordering,
  including negative/empty indices and an exact upper bound above 128 bits.
  Explicit initializers keep this shared-input check separate from existing
  default-initializer AST synthesis. Full Semantic IR/lowering tests and
  targeted vet pass. This completes determinism coverage for the represented
  source/IR scope, not schema emission or final package acceptance.

## G. Final acceptance and handoff

- [x] P16-54 — Expand the matrix below to one row per P16 §144 criterion, with
  exact implementation and test references. Keep unsupported cases and missing
  normative decisions visible; aggregate success proves only the stated scope.

  Completed 2026-10-20. The matrix below has one numbered row for each of all
  55 current §144 criteria, retaining the exact criterion text and linking
  concrete implementation/specification and test evidence. Stage-specific
  status distinguishes Sema, Semantic IR, handwritten MLIR, source emission
  and recorded gates. Open implementation, MD-031 (MD-084/MD-085 applied 2026-10-22), P17/P18/P24
  exclusions, final snapshot/full-Go gates and the retired legacy-route wording
  remain explicit. Completing the matrix does not complete those criteria or
  change P16's partial governance status. Row count/order, referenced paths
  and named Go tests are checked against the current repository.
- [ ] P16-55 — On the isolated execution snapshot, run full Go/vet, configured
  P13/P14/P15/P16 source pipelines, check-sec-mlir, diff checks and governance
  checks. Record exact commands, results, environment, commit/tree and failures.
- [ ] P16-56 — Update the single P16 governance entry with actual implemented
  scope, P15 dependencies and P17/P18 deferrals; mark implemented only after
  every applicable §144 acceptance criterion has evidence.
- [x] P16-57 — Write the §145 report covering all 43 requested topics in order,
  including current predecessor state, signed-len limitation, deviations and
  recommendations for P17. Record its concrete path in governance.

  Completed 2026-10-21 in
  `rules/mlir/packages/sec-mlir-dialect_package16-implementation-report.md`.
  All 43 current §145 topics appear in order, including unsigned length and
  explicit signed conversion (the historical signed-len limitation is resolved).
  The report identifies current HEAD and P16-related dirty files, predecessor
  status, exact tool versions/source revision, implemented stage boundaries,
  unsupported paths, MD-084/MD-085 and recommendations for independent P17 work.
  Fresh focused P16 IR/lowering tests pass with real tooling; check-sec-mlir
  passes 136/136. Older full-Go results are dated and distinguished from current
  evidence. No final isolated snapshot, full-Go pass or package completion is
  claimed. Report completion leaves eight numbered P16 points open.
- [ ] P16-58 — Recheck acceptance, rulebook status, unique governance ownership,
  predecessor regressions and unsupported boundaries before handoff.

## Acceptance coverage map

P16-54 audit, 2026-10-20; updated for P16-44 on 2026-10-21: each numbered row corresponds, in order and without
omissions, to one criterion in the current P16 §144 list. This is a coverage
matrix, not a declaration that all criteria pass. Normative authority remains
`rules/mlir/packages/sec-mlir-dialect_package16.md`, its current-authority table,
and the installed consolidated rulebooks.

**Recorded** means a specification, snapshot or dated gate has evidence.
**Implemented: scope** names the stage whose contract is covered.
**Partial** and **Pending** retain the unimplemented work listed in the last
column. A source→IR test does not prove source→schema→binary support; a
handwritten MLIR test does not prove Go emitter support. Completion of this
matrix closes P16-54 only. P16-56 stays open, and the sole governance owner
`lowering.sec-mlir-package16` remains partial.

Evidence references use repository-relative paths. `file::TestName` identifies
an existing Go test in that exact file. Gate results below are dated records
from prior implementation steps, not new executions performed for this audit.
MD-084 and MD-085 were open at the audit and were applied on 2026-10-22: canonical metadata grammar and path-aware return-origin verification. MD-031
separately limits guarded/residual try metadata. P17/P18 and P24 retain their
non-trivial-element, owning-array and physical-representation responsibilities.

| §144 row | Criterion (verbatim) | Current scope/status | Implementation/specification anchor | Concrete evidence | Remaining work / limit |
|---|---|---|---|---|---|
| 1 | baseline documents repo 152c772 + local P13/P14/P15 or newer equivalent | Recorded | `package16-todo.md` — Execution baseline refresh; local snapshot tree and archive digest. | P16-01/P16-02 isolated extraction and recorded focused/full gates. | Recapture the final execution tree under P16-55; the 2026-10-19 snapshot predates later completions. |
| 2 | previous package regressions remain green | Partial | `internal/ir/semantic/builder.go`; `internal/lowering/secmlir/emitter_options.go`. | `internal/ir/semantic/slice_lowering_test.go::TestPackage16BuilderPreservesPredecessorRoutes`; focused P42 Go gate recorded in `governance/lowering.yaml`. | P16-07/P16-55/P16-58: full predecessor acceptance is not established by focused regressions. |
| 3 | slice normative synchronization applied | Recorded | `rules/corrections/applied/p16-slice-source-rules-synchronization-20261019.md`; `rules/mlir/normative-sync/sec_slice_sync_v1.md`. | P16-04 completion note and the source matrix in `internal/sema/slice_matrix_test.go`. | Source-rule installation is complete; representation and executable acceptance are separate. |
| 4 | Semantic IR slice amendment applied | Recorded | `rules/compiler/semantic_ir.md` §126; `rules/corrections/applied/p16-slice-ir-schema12-synchronization-20261019.md`. | P16-05 installation record; `internal/ir/semantic/slice_types_test.go`. | The executable IR version remains 1; P16-26/P16-27 remain open. |
| 5 | schema-v12 dialect rulebook installed | Recorded | `rules/mlir/sec_mlir_dialect.md` v12.1–33; `mlir/lib/Dialect/Sec/SecDialect.cpp`. | `internal/lowering/secmlir/emitter_options_test.go::TestEmitSchema12RoundTripWithRealTool`; `mlir/test/Dialect/Sec/schema12-slice-types.mlir`. | Installed schema/registration is not source slice emission; default output remains schema 11. |
| 6 | lowering-v12 rulebook installed | Recorded | `rules/mlir/sec_mlir_lowering.md` v12.1–31; P16-05 correction. | P16-05 completion record; `internal/lowering/secmlir/emitter_options_test.go`. | Specification installation and represented slice emission are complete; P16-43/P16-45/P16-50 retain complete verification/integration. |
| 7 | explicit fallible ref/ref-mut slice syntax synchronized | Implemented: source rules | `rules/collections/collections.md` §7.5; `internal/ir/semantic/slice_source_try.go`. | `internal/ir/semantic/slice_source_try_test.go::TestPackage16SliceSourceTry`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 8 | RangeError mapping implemented | Partial: source→IR→schema12 | `internal/sema/slice_range_classification.go`; `internal/ir/semantic/slice_ranges.go`; `mlir/lib/Dialect/Sec/SliceOps.cpp`. | `internal/sema/slice_creation_plans_test.go::TestSliceRangeErrorCoreIdentity`; `mlir/test/Dialect/Sec/invalid-schema12-slice-ops.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 9 | static range bounds use arbitrary precision | Implemented: exact facts | `internal/sema/slice_places.go`; `internal/ir/semantic/slice_types.go`. | `internal/sema/slice_range_exact_test.go::TestSliceProjectionBoundsAreExactBeyondInt64`; `internal/ir/semantic/slice_borrows_test.go::TestSliceBorrowExactFixedExtents`. | No host-width cap is chosen; physical extent representation remains P24. |
| 10 | dynamic SliceRangeID implemented | Implemented: Sema identity | `internal/sema/place_identities.go`; `internal/sema/slice_creation_plans.go`. | `internal/sema/slice_range_id_test.go::TestSliceRangeIDsAreStableAndQueriesDetached`; `TestSliceRangeIdentityCompositionAndAllocation` in that file. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. P16-26/P16-27 retain complete fact/returned-range verification gaps. |
| 11 | PlaceSlice migrated from canonical int64 bounds | Implemented: migration | `internal/sema/place.go` — PlaceProjection exact bounds; `internal/sema/slice_places.go`; `internal/sema/sequence_projections.go`. | `internal/sema/slice_range_exact_test.go`; `internal/sema/parameter_usage_slice_test.go::TestParameterSlicePlacesUseCanonicalRanges`. | Persistence uses the revised exact-bound schema; this does not select runtime layout. |
| 12 | static nested slice normalization implemented | Implemented: Sema/source IR | `internal/sema/slice_places.go`; `internal/ir/semantic/slice_source_borrows.go`. | `internal/sema/slice_static_subranges_test.go::TestStaticSliceSubrangesAndEmptyProvenance`; `internal/ir/semantic/slice_source_reborrow_test.go`. | Complete cross-block/returned-slice facts remain P16-26/P16-27/P16-36. |
| 13 | disjoint static range reasoning preserved | Partial: Sema reasoning | `internal/sema/place.go` — Relationship; `internal/sema/slice_places.go`. | `internal/sema/slice_static_subranges_test.go`; `internal/sema/slice_range_exact_test.go`. | P16-43/P16-50 also verify source disjoint mutable ranges through schema 12; P16-26 retains complete IR verification. |
| 14 | ResolvedSliceCreationPlan implemented | Implemented: public Sema plan | `internal/sema/slice_creation_plans.go` — ResolvedSliceCreationPlanOf. | `internal/sema/slice_creation_plans_test.go::TestResolvedSliceCreationPlans`. | No query-time inference or new borrow allocation; remaining consumers retain their own gates. |
| 15 | plan query is read-only | Implemented: query contract | `internal/sema/slice_plan_snapshots.go`; `internal/sema/slice_creation_plans.go`. | `internal/sema/slice_plan_snapshots_test.go::TestSlicePlanQueriesAreDetachedAndReadOnly`; `internal/ir/semantic/slice_determinism_test.go`. | Defensive/query evidence does not complete all source value-flow producers. |
| 16 | shared/mutable slice Semantic IR types implemented | Implemented: abstract types | `internal/ir/semantic/slice_types.go`; `mlir/include/sec/Dialect/Sec/SecTypes.td`. | `internal/ir/semantic/slice_types_test.go::TestSliceTypesPreserveElementAndAuthority`; `mlir/test/Dialect/Sec/schema12-slice-types.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 17 | SliceFacts reuse P15 reference facts | Partial: facts model | `internal/ir/semantic/slice_types.go` — SliceFacts; `mlir/lib/Analysis/ReferenceFacts.h`. | `internal/ir/semantic/slice_types_test.go::TestSliceFactsClonePreservesReferenceContract`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. P16-26/P16-27 retain complete fact/returned-range verification gaps. MD-084 remains open. |
| 18 | fixed-array shared slice borrow implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_source_borrows.go`; `mlir/include/sec/Dialect/Sec/SecSliceOps.td`. | `internal/ir/semantic/slice_source_borrows_test.go::TestPackage16FixedSliceSource`; `mlir/test/Dialect/Sec/schema12-slice-ops.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 19 | fixed-array mutable slice borrow implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_borrows.go`; `internal/ir/semantic/slice_source_borrows.go`; `mlir/lib/Dialect/Sec/SliceOps.cpp`. | `internal/ir/semantic/slice_source_borrows_test.go`; `mlir/test/Dialect/Sec/invalid-schema12-slice-authority.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 emits mutable fixed-array borrows; P16-43/P16-50 cover represented MLIR ownership; P16-26 retains complete IR verification. |
| 20 | shared slice reborrow implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_source_borrows.go`; `mlir/lib/Analysis/VerifySliceGuards.cpp`. | `internal/ir/semantic/slice_source_reborrow_test.go::TestPackage16SliceSourceReborrow`; `mlir/test/Analysis/schema12-slice-guards.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 21 | mutable slice reborrow implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_borrows.go`; `mlir/lib/Dialect/Sec/SliceOps.cpp`. | `internal/ir/semantic/slice_source_reborrow_test.go`; `mlir/test/Analysis/schema12-slice-guards.mlir` — mutable_reborrow. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 22 | shared slice copy implemented | Partial: white-box IR→schema12 | `internal/ir/semantic/slice_transfer.go`; `mlir/include/sec/Dialect/Sec/SecSliceOps.td`. | `internal/ir/semantic/slice_transfer_test.go::TestSharedSliceCopyHasIndependentHolderEnd`; `mlir/test/Dialect/Sec/schema12-slice-ops.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. General source holder copy/move remains P16-36. |
| 23 | mutable slice move implemented | Partial: white-box IR→schema12 | `internal/ir/semantic/slice_transfer.go`; `internal/ir/semantic/reference_lifetimes.go`. | `internal/ir/semantic/slice_transfer_test.go::TestMutableSliceMoveRejectsForwardedOldHolder`; `mlir/test/Dialect/Sec/schema12-slice-ops.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. General source holder copy/move remains P16-36. |
| 24 | empty slices implemented | Partial: source→IR→schema12 | `internal/sema/slice_places.go`; `internal/ir/semantic/slice_source_borrows.go`. | `internal/sema/slice_static_subranges_test.go`; `internal/ir/semantic/slice_source_wide_test.go::TestPackage16WideSliceSource`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 25 | SliceRangeNormalizeOp implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_ranges.go`; `mlir/lib/Dialect/Sec/SliceOps.cpp`. | `internal/ir/semantic/slice_ranges_test.go::TestSliceRangeOperations`; `mlir/test/Dialect/Sec/schema12-slice-ops.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 26 | SliceRangeCheckOp implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_ranges.go`; `mlir/lib/Analysis/VerifySliceGuards.cpp`. | `internal/ir/semantic/slice_ranges_test.go::TestSliceRangeCheckRetainsExactExtremeEndpoints`; `mlir/test/Analysis/invalid-schema12-slice-guards.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 27 | ordinary range panic CFG implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_source_borrows.go`; `internal/ir/semantic/sequence_bounds.go`. | `internal/ir/semantic/slice_source_borrows_test.go::TestPackage16FixedSliceSource`; `mlir/test/Analysis/schema12-slice-guards.mlir` — range. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 28 | fallible RangeError CFG implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_source_try.go`; `internal/ir/semantic/failure_set_try.go`. | `internal/ir/semantic/slice_source_try_test.go::TestPackage16SliceSourceTry`; `mlir/test/Analysis/schema12-slice-guards.mlir` — fallible. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 29 | local/naked try range integration works | Partial: represented try forms | `internal/ir/semantic/slice_source_try.go`; `internal/ir/semantic/try_local_handlers.go`. | `internal/ir/semantic/slice_source_try_test.go::TestPackage16SliceTryDependencyBoundaries`; `internal/ir/semantic/slice_source_matrix_test.go::TestPackage16SourceDependencyMatrix`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 emits represented ordinary/local/naked try forms. P16-26/P16-36 retain holder recovery joins; MD-031 retains guarded/residual MLIR metadata. |
| 30 | SliceLengthOp implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_length.go`; `internal/ir/semantic/slice_source_length.go`; `mlir/lib/Dialect/Sec/SliceOps.cpp`. | `internal/ir/semantic/slice_length_test.go::TestSliceLengthOperation`; `mlir/test/Analysis/schema12-slice-guards.mlir` — length. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 31 | len(slice) and .Len share unsigned length lowering without narrowing | Implemented: unsigned source/IR/MLIR contract | `internal/sema/slice_length_plans.go`; `internal/ir/semantic/slice_source_length.go`. | `internal/ir/semantic/slice_source_length_test.go::TestPackage16SliceSourceLength`; `internal/ir/semantic/slice_source_wide_test.go`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 32 | ResolvedSliceIndexPlan implemented | Implemented: public Sema plan | `internal/sema/slice_index_plans.go` — ResolvedSliceIndexPlanOf. | `internal/sema/slice_index_plans_test.go::TestResolvedSliceIndexPlans`; `internal/sema/slice_plan_snapshots_test.go`. | P16-26 retains complete IR verification; P16-43 covers represented MLIR verification; plan availability alone is not acceptance. |
| 33 | slice index bounds proof/runtime classification works | Implemented: Sema classification | `internal/sema/index_plans.go`; `internal/sema/slice_index_plans.go`. | `internal/sema/slice_index_plans_test.go::TestSliceIndexConstantBounds`; `internal/ir/semantic/slice_source_matrix_test.go::TestPackage16SourceMatrix`. | P16-26 retains complete IR verification; P16-43 covers represented MLIR verification. |
| 34 | SliceIndexInBoundsOp implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_index.go`; `internal/ir/semantic/slice_source_index.go`. | `internal/ir/semantic/slice_index_test.go::TestSliceIndexOriginalIntegerCarriers`; `mlir/test/Analysis/schema12-slice-guards.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 35 | SliceElementPlaceOp implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_index.go`; `mlir/lib/Analysis/VerifySliceGuards.cpp`. | `internal/ir/semantic/slice_index_test.go::TestSliceIndexRejectsInvalidContracts`; `mlir/test/Analysis/invalid-schema12-slice-guards.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 36 | slice reads/writes reuse P15 places | Partial: source→IR→schema12 | `internal/ir/semantic/slice_source_index.go`; `internal/ir/semantic/place_reference.go`. | `internal/ir/semantic/slice_source_index_test.go::TestPackage16SliceSourceIndex`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. P17 non-trivial elements and P15 value-carrier limitations remain separate. |
| 37 | slice element ref/ref-mut borrowing works | Partial: source→IR→schema12 | `internal/ir/semantic/slice_source_index.go`; `internal/ir/semantic/reference_use.go`. | `internal/ir/semantic/slice_source_matrix_test.go::TestPackage16SourceMatrix`; `internal/ir/semantic/slice_index_temporal_test.go`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. P17 non-trivial elements and P15 value-carrier limitations remain separate. |
| 38 | ordinary slice index panic CFG implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_source_index.go`; `internal/ir/semantic/sequence_bounds.go`. | `internal/ir/semantic/slice_source_index_test.go`; `mlir/test/Analysis/schema12-slice-guards.mlir` — index. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 39 | fallible IndexError CFG implemented | Partial: source→IR→schema12 | `internal/ir/semantic/slice_source_index.go`; `internal/ir/semantic/failure_set_try.go`. | `internal/ir/semantic/slice_index_test.go::TestSliceIndexFallibleFailure`; `mlir/test/Analysis/schema12-slice-guards.mlir` — fallible_index. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 40 | dynamic slice validity ordering implemented | Partial: source/white-box IR/local MLIR | `internal/sema/slice_effects.go`; `internal/ir/semantic/slice_source_index.go`; `mlir/lib/Analysis/VerifySliceGuards.cpp`. | `internal/ir/semantic/slice_reborrow_temporal_test.go`; `internal/ir/semantic/slice_index_temporal_test.go`; `mlir/test/Analysis/invalid-schema12-slice-guards.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | Epoch-injected producer tests are white-box evidence, not Arena source allocation. P16-43/P16-50 verify represented MLIR; P16-26 and source Arena production remain separate. |
| 41 | sec.ref.is_valid accepts direct slices | Implemented: local MLIR extension | `mlir/lib/Dialect/Sec/ReferenceBoundaries.cpp`. | `mlir/test/Dialect/Sec/schema12-slice-reference-boundaries.mlir`; `mlir/test/Dialect/Sec/invalid-schema12-slice-reference-boundaries.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-43 extends P15 reference guards; P16-45 completes the integrated matrix; P16-55 retains final isolated acceptance. |
| 42 | sec.ref.end_borrow handles slice borrows | Partial: IR/local MLIR marker | `internal/ir/semantic/reference_lifetimes.go`; `mlir/lib/Dialect/Sec/ReferenceBoundaries.cpp`. | `internal/ir/semantic/slice_transfer_test.go`; `mlir/test/Dialect/Sec/schema12-slice-reference-boundaries.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-43 verifies represented MLIR holder/derived/CFG end and move flow; P16-36 automatic/source holder ends remain. |
| 43 | sec.fail.bounds accepts slice-index/slice-range | Implemented: local MLIR provenance | `mlir/lib/Dialect/Sec/ReferenceBoundaries.cpp`. | `mlir/test/Dialect/Sec/invalid-schema12-slice-reference-boundaries.mlir`; `mlir/test/Analysis/schema12-slice-guards.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P16-44 now emits represented facts/operations (see its completion note). P16-43/P16-50 compose all slice/P15 verifiers over represented modules; P16-45 completes the integrated MLIR matrix; P16-55 retains final isolated acceptance. |
| 44 | --sec-verify-slice-guards registered | Implemented: registered guard pass | `mlir/include/sec/Analysis/Passes.td`; `mlir/lib/Analysis/VerifySliceGuards.cpp`. | `mlir/test/Analysis/schema12-slice-guards.mlir`; `mlir/test/Analysis/invalid-schema12-slice-guards.mlir` (P42: full lit 132/132). | This is the represented CFG/SSA guard boundary; P16-43 now composes represented P15 borrow verification; MD-084/MD-085 remain open in P16-07. |
| 45 | place/reference/borrow verifiers extended | Implemented: represented MLIR contracts | `mlir/lib/Analysis/VerifyPlaceReferences.cpp`; `mlir/lib/Analysis/VerifySliceReferences.cpp`; `mlir/lib/Analysis/ReferenceFlow.h`. | `mlir/test/Analysis/schema12-slice-borrows.mlir`; its invalid companion; `internal/lowering/secmlir/slice_emit_test.go::TestPackage16EmitSourceSlices`; existing direct-reference baseline. | P16-43 is complete within the existing metadata/return-proof model. MD-084/MD-085 remain P16-07; source value production and complete IR verification remain P16-36/P16-26/P16-27. |
| 46 | returned-slice origin summaries implemented | Partial: Sema and represented MLIR roots | `internal/sema/slice_return_origins.go`; `internal/sema/reference_return_origins.go`. | `internal/sema/slice_origins_test.go::TestSliceOriginTransferMatrix`; `internal/ir/semantic/slice_determinism_test.go::TestPackage16OriginOrderDeterminism` (Sema-only); P16-43 MLIR return-root tests cover direct/narrowed/forwarded slices and reject local/unlisted origins. | P16-07/P16-27/P16-36/P16-48; MD-084/MD-085 keep metadata/return-proof obligations visible. |
| 47 | P13/P14/P15 integration passes | Partial: P13/P14 compatibility complete | `internal/ir/semantic/struct_verification.go`; `mlir/lib/Dialect/Sec/StructOps.cpp`; `internal/ir/semantic/slice_source_index.go`. | P16-47: `internal/sema/slice_aggregate_test.go`; `internal/ir/semantic/slice_aggregate_test.go`; `internal/lowering/secmlir/slice_emit_test.go`; `mlir/test/Dialect/Sec/schema12-slice-aggregates.mlir`; its invalid companion. Source nested-array fields pass 32/64 schema-12 round-trips; contained slice origins are Sema evidence and shared aggregate copy operations have handwritten MLIR evidence. | P16-43/P16-50 complete represented P15 verifier composition. P16-36/P16-48/P16-55 retain general source holder/return/value integration and final acceptance. |
| 48 | owning dynamic T[] remains distinct/unsupported in new IR | Implemented exclusion and unsupported matrix | `internal/ir/semantic/slice_lowering.go`; `internal/ir/semantic/slice_types.go`. | `internal/lowering/secmlir/slice_unsupported_test.go::TestPackage16UnsupportedSourceMatrix`; `cmd/compiler/slice_boundary_test.go::TestSliceUnsupportedCLIBoundaries`. | P16-51 covers target-aware rejection and no partial modules/artifacts; P18 still owns dynamic backing/capacity/ownership and P16-55 final acceptance. |
| 49 | no slice equality lowering | Implemented exclusion and unsupported matrix | `mlir/include/sec/Dialect/Sec/SecSliceOps.td` — no slice comparison operation; `internal/sema/slice_matrix_test.go`. | `internal/lowering/secmlir/slice_unsupported_test.go::TestPackage16UnsupportedSourceMatrix`; `cmd/compiler/slice_boundary_test.go::TestSliceUnsupportedCLIBoundaries`. | P16-51 verifies equality/ordering source errors on both plans and CLI no-artifact boundaries; P16-55 retains final acceptance. No equality semantics are invented. |
| 50 | no FFI slice ABI | Implemented exclusion and dedicated rejection coverage | `rules/mlir/sec_mlir_lowering.md` v12.26; `internal/sema/ffi_legality.go`; `internal/lowering/secmlir/slice_boundaries.go`. | `internal/lowering/secmlir/slice_unsupported_test.go::TestPackage16UnsupportedRepresentationMatrix`; `cmd/compiler/slice_boundary_test.go::TestSliceUnsupportedCLIBoundaries`. | P16-51 covers source foreign parameters/returns, verified direct and nested struct/union IR carriers, nil output and CLI no-artifact evidence. P16-55 retains final acceptance; no FFI descriptor is selected. |
| 51 | no physical slice descriptor selected | Implemented abstraction invariant | `internal/ir/semantic/slice_types.go`; `mlir/include/sec/Dialect/Sec/SecTypes.td`. | `internal/ir/semantic/slice_architecture_test.go`; `mlir/test/Conversion/SecToCore/schema12-slice-architecture.mlir`. P16-44: `internal/lowering/secmlir/slice_emit_test.go`. | P24 owns physical descriptors; current P6 succeeds abstractly and P8 rejects unresolved reference representation without output. |
| 52 | no mandatory runtime | Implemented abstraction invariant | `internal/ir/semantic/slice_transfer.go`; `mlir/lib/Analysis/ReferenceFacts.h`. | `internal/ir/semantic/slice_architecture_test.go::TestPackage16EpochArchitecture`; `mlir/test/Conversion/SecToCore/schema12-slice-architecture.mlir`. | Symbolic epoch facts do not imply an allocated generation manager or runtime borrow counter; binary support is separate. |
| 53 | check-sec-mlir passes | Recorded focused/current tool gate | `mlir/lib/Analysis/CMakeLists.txt`; `mlir/test/lit.cfg.py`. | P16-45 recorded `cmake --build build/sec-mlir --target check-sec-mlir -j2`: 136/136; real emitted-source tests separately exercise the rebuilt tool. | P16-45 completes the current integrated matrix. P16-55 must repeat the complete gate against the final isolated snapshot. |
| 54 | go test ./... passes | Pending full gate | `package16-todo.md` — Execution baseline refresh and seven individually owned failures; `governance/lowering.yaml` verification records. | P16-45 complete Sema/IR/lowering suites and focused compiler/vet checks passed; the last recorded full Go run had seven failures. | P16-55 must run a fresh full Go gate; historical failures are not asserted to remain current, and focused success is not full-Go success. |
| 55 | legacy paths remain operational | Superseded: single track | `rules/compiler/compiler_pipeline.md` §45(6); `cmd/compiler/native_pipeline.go`. | P42 recorded native/enum/emit/reference regression command; `internal/ir/semantic/slice_lowering_test.go::TestPackage16BuilderPreservesPredecessorRoutes`. | The historical criterion does not authorize restoring legacy backends. P16-55/P16-58 validate build/emit-mlir/emit-llvm boundaries of the single track. |

The signed-length wording in older inventories is superseded by MD-074/MD-088:
`.Len` and `len(slice)` share unsigned count semantics (row 31). Row 55 retains
the criterion's historical wording verbatim for traceability, but its current
interpretation is the single track in compiler_pipeline.md §45(6), not an
additional compiler route.

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
