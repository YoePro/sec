# SEC MLIR Package 16 implementation report

Audit date: 2026-10-21. Reporting contract: P16 §145, all 43 topics in order.

This report completes P16-57 only. **P16 remains partial.** It describes the
current working tree, including uncommitted predecessor and P16 changes.
Fresh checks below are working-tree evidence; final isolated acceptance is
P16-55/P16-58. Source, Semantic IR, emitted schema 12 and physical execution
are distinguished throughout. The acceptance map remains in
`package16-todo.md`; the sole owner is `lowering.sec-mlir-package16` in
`governance/lowering.yaml`.

## 1. Repository HEAD implemented against

Audited HEAD: `080ec6415f98620de347b495ec2c3efbbe1236f7`, plus the dirty
working tree inventoried in sections 6–7. HEAD alone does not reproduce this
implementation. No commit, clean-tree assertion or final isolated snapshot is
created by this report. The older 2026-10-19 archive and its digest in the TODO
are historical evidence, not a snapshot of today's code.

## 2. Local/merged P13/P14/P15 status

The canonical ledger marks P13 and P14 implemented and P15 partial. P13/P14
types, identities and value operations are maintained predecessor code.
P15 schema 11, Place/reference operations and verifiers exist; relevant P16
extensions also include current uncommitted files. P15 acceptance must not be
inferred from successful P16 tests. See `package15-todo.md` and its owner.

## 3. Previous package status

The maintained path reads schemas 1–12 and defaults to emitting schema 11.
P15 still has source/acceptance work, including addressed storage and some
parameter/return paths. MD-084 covers concrete reference metadata spelling;
MD-085 covers MLIR returned-origin checking depth. P16-07 retains these
dependencies. There is one compilation track under compiler_pipeline §45(6).

## 4. Slice syntax/range/error synchronization

P16-03/04/05 integrated explicit `ref`/`ref mut`, `try ref`, ordered operand
evaluation, half-open normalization, empty provenance, exact/conservative
ranges, authority and canonical error identities. The three applied
`p16-*-20261019.md` corrections named by governance record the authority audit
and consolidated source/IR/schema changes. They preserve subsequent source
decisions rather than restoring historical package text.

## 5. Unsigned len(slice)/.Len agreement and explicit signed-conversion boundary

Both spellings resolve to target-sized `uint` and one unsigned length
operation. There is no `SliceLengthInt` or `sec.slice.len_int`. An implicit
assignment of either length form to `int` is rejected, including through the
public CLI. Explicit signed conversion has its own checked conversion
contract; P16 does not silently narrow a count. Tests:
`slice_source_length_test.go`, `slice_unsupported_test.go` and
`cmd/compiler/slice_boundary_test.go`.

## 6. Files added

The P16-related untracked files at this audit are enumerated below. This is a
working-tree inventory relative to HEAD, not a claim that all historical P16
files are newly added today. Already committed implementation anchors are
identified in their corresponding sections. This report itself is a new file.

- `cmd/compiler/slice_boundary_test.go`
- `internal/ir/semantic/slice_aggregate_test.go`
- `internal/ir/semantic/slice_capabilities.go`
- `internal/ir/semantic/slice_value_facts.go`
- `internal/ir/semantic/slice_value_facts_test.go`
- `internal/ir/semantic/struct_verification.go`
- `internal/lowering/secmlir/operation_attributes.go`
- `internal/lowering/secmlir/slice.go`
- `internal/lowering/secmlir/slice_boundaries.go`
- `internal/lowering/secmlir/slice_emit_test.go`
- `internal/lowering/secmlir/slice_unsupported_test.go`
- `internal/lowering/secmlir/slice_verification_matrix_test.go`
- `internal/sema/slice_aggregate_test.go`
- `mlir/lib/Analysis/ReferenceFlow.h`
- `mlir/lib/Analysis/VerifySliceReferences.cpp`
- `mlir/lib/Dialect/Sec/StructOps.cpp`
- `mlir/test/Analysis/invalid-schema12-slice-borrows.mlir`
- `mlir/test/Analysis/schema12-slice-borrows.mlir`
- `mlir/test/Dialect/Sec/invalid-schema12-slice-aggregates.mlir`
- `mlir/test/Dialect/Sec/schema12-slice-aggregates.mlir`
- `testdata/sema/slice_aggregate_copy_invalid.sec`
- `testdata/sema/slice_aggregate_escape_invalid.sec`
- `testdata/sema/slice_aggregate_traits.sec`
- `testdata/semantic_ir/package16_boundaries/element_move_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/entry.sec`
- `testdata/semantic_ir/package16_boundaries/equality_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/ffi_parameter_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/ffi_return_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/iteration.sec`
- `testdata/semantic_ir/package16_boundaries/membership.sec`
- `testdata/semantic_ir/package16_boundaries/move_only_read_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/ordering_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/pointer.sec`
- `testdata/semantic_ir/package16_boundaries/raw_conversion_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/reverse.sec`
- `testdata/semantic_ir/package16_boundaries/signed_global_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/signed_property_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/stable_source_invalid.sec`
- `testdata/semantic_ir/package16_boundaries/weak_source_invalid.sec`
- `testdata/semantic_ir/slice_aggregate_arrays.sec`
- `testdata/semantic_ir/slice_disjoint_mutable.sec`
- `testdata/semantic_ir/slice_emit_nominal.sec`
- `testdata/semantic_ir/slice_empty_mutable.sec`
- `internal/ir/semantic/slice_reference_flow.go`
- `internal/ir/semantic/slice_reference_flow_test.go`
- `internal/lowering/secmlir/slice_reference_flow_test.go`
- `internal/ir/semantic/cfg_value_origins.go`
- `internal/ir/semantic/slice_cfg_proofs_test.go`
- `internal/lowering/secmlir/slice_cfg_proofs_test.go`
- `internal/lowering/secmlir/branch_targets.go`

## 7. Files modified

The following dirty tracked files participate in current P16 work. Shared
files can also contain unrelated changes; this inventory does not attribute
their entire diff to P16. Existing pitfall changes are outside this report.

- `governance/lowering.yaml`
- `internal/ir/semantic/model.go`
- `internal/ir/semantic/printer.go`
- `internal/ir/semantic/slice_lowering.go`
- `internal/ir/semantic/slice_source_borrows.go`
- `internal/ir/semantic/verifier.go`
- `internal/lowering/secmlir/emitter.go`
- `internal/lowering/secmlir/emitter_options.go`
- `internal/lowering/secmlir/emitter_options_test.go`
- `language-rulebook-status.md`
- `missing-decisions.yaml`
- `mlir/lib/Analysis/CMakeLists.txt`
- `mlir/lib/Analysis/ReferenceFacts.h`
- `mlir/lib/Analysis/VerifyPlaceReferences.cpp`
- `mlir/lib/Analysis/VerifySliceGuards.cpp`
- `mlir/lib/Dialect/Sec/CMakeLists.txt`
- `mlir/lib/Dialect/Sec/OperationContracts.h`
- `mlir/lib/Dialect/Sec/SecOps.cpp`
- `mlir/lib/Dialect/Sec/SliceOps.cpp`
- `mlir/test/Analysis/invalid-schema12-slice-guards.mlir`
- `mlir/test/Dialect/Sec/invalid-schema12-slice-authority.mlir`
- `mlir/test/Dialect/Sec/invalid-schema12-slice-ops.mlir`
- `mlir/test/Dialect/Sec/schema12-slice-types.mlir`
- `package16-todo.md`
- `internal/ir/semantic/place_reference.go`
- `internal/ir/semantic/reference_lifetimes.go`

## 8. PlaceSlice arbitrary-precision migration

`internal/sema/place.go` stores detached `*big.Int` half-open slice bounds;
nil means an unknown endpoint. `slice_range_exact_test.go` covers bounds past
int64, exact endpoint presentation and nested composition. This is independent
of host integer width and the target's separately checked array-size limit.

## 9. SliceRangeID implementation

`internal/sema/slice_creation_plans.go` and the IR `slice_types.go` retain
symbolic identities for evaluated dynamic ranges. Unknown bounds remain
unknown; the ID is neither an address nor a runtime counter. Detached query
and fresh-build determinism tests preserve it across independent analyses.

## 10. Static nested range normalization

`slice_normalization.go` and `slice_return_origins.go` retain exact bounds
and conservatively join alternative origins. Omitted endpoints, inclusive
increment, empty ranges and constant diagnostic precedence have dedicated
Sema tests. Distinct static mutable ranges remain disjoint; unresolved
dynamic overlap is not granted an invented runtime proof.

## 11. ResolvedSliceCreationPlan API

`ResolvedSliceCreationPlanOf` publishes source kind/Place, element/authority,
original endpoint types, evaluation order, normalization/check classification,
exact or symbolic range and canonical P15 reference facts. Queries clone
nested data without inference or state mutation. Evidence:
`TestResolvedSliceCreationPlans`, `TestSlicePlanQueriesAreDetachedAndReadOnly`.

## 12. SliceFacts representation

`internal/ir/semantic/slice_types.go` keeps element/authority, P15 reference
metadata, origin Places, runtime length SSA, optional exact range and dynamic
range identity. `Clone` detaches nested range/storage data. Source construction
publishes these facts; `slice_value_facts.go` rejects contradictory attached
identity, authority, length and provenance. No physical descriptor is selected.

## 13. Shared/mutable slice type implementation

`TypeSharedSlice`/`TypeMutableSlice` and `TypeTable.InternSlice` distinguish
bounded shared and exclusive references from arrays and direct element refs.
Element identity and authority define the type; extent and target width do
not. Shared holders are copyable; mutable holders move. Both are non-owning
and trivially destructible independently of element copyability.

## 14. Fixed-array borrow implementation

`slice_source_borrows.go` consumes the selected source Place and normalized
tuple; `slice_borrows.go` verifies element/authority, exact array extent and
source storage/address facts. Construction neither loads nor copies backing
elements. Supported local and projected fixed-array source tests run on both
32- and 64-bit plans.

## 15. Slice reborrow implementation

Reborrow consumes the existing bounded source, preserves its validity/storage
contract, creates the resolved child borrow and narrows through the matching
normalized tuple. Source production is tested by `slice_source_reborrow_test.go`;
dynamic epochs also have explicitly white-box tests. Unknown joins and general
holder production remain part of P16-26/P16-36.

## 16. Shared copy/mutable move

IR and schema 12 implement `slice.copy-shared`/`slice.move-mut` and canonical
holder-end markers. Transfers preserve bounded identity without observing
backing storage. Tests reject changed metadata, mutable copying and use after
move/end, including derived values. General source holder copies/moves,
signature values and automatic end production remain P16-36.

## 17. Range normalize/check operations

`slice_ranges.go` implements proven normalization and runtime checking using
the original signed/unsigned endpoint SSA types and a target-sized unsigned
source extent. Original wide inputs are not first narrowed. The total checked
tuple includes normalized start/count, failure flag and canonical RangeError.
Checked values are confined to their appropriate CFG edges.

## 18. RangeError mapping/CFG

Sema classification and runtime plans retain InvalidRange, StartAfterEnd and
OutOfBounds with canonical precedence. `slice_source_try.go` composes ordinary
slice-range bounds panic and fallible Result flow with existing handlers.
Guarded/residual/Option handler emission still retains MD-031; recovery-value
holder joins and slice-valued signatures are not claimed complete.

## 19. Slice length operations

`slice_length.go` verifies shared/mutable input and exact unsigned result.
`slice_source_length.go` consumes resolved `.Len`/`len` facts, preserves SSA
identity and performs no element read or holder transfer. Dynamic validity
precedes observation. The same behavior is emitted as `sec.slice.len`.

## 20. ResolvedSliceIndexPlan API

`internal/sema/slice_index_plans.go` publishes source/element/reference facts,
original index type, check/proof and selected use action. Query tests cover
empty/negative/wide indices, exact static extent, authority, borrow and
transfer intent. Snapshot tests prove detachment without rerunning Sema.

## 21. Slice index bounds implementation

`slice.index-in-bounds` retains the exact slice/index SSA pair and original
signedness. Runtime element access requires its true edge; proven accesses
retain resolved proof provenance. Tests reject wrong predicates, bypasses,
wrong SSA and temporal/spatial confusion without inventing layout arithmetic.

## 22. Element place implementation

`slice.element-place` produces the P15 Place of the same element and source
authority. Shared sources cannot produce writable Places. Canonical storage,
borrow/lifetime and validity facts survive. `slice_index_test.go` exercises
local operation and function-level guard contracts on both target plans.

## 23. P15 read/write/ref integration

`slice_source_index.go` reuses P15 trivial reads/replacements and element
reference borrows. Ending a parent invalidates derived access paths. No
backing-array extraction/copy is substituted. Nontrivial reads/replacement and
unrepresented reference-valued element carriers retain named P15/P17 limits.

## 24. IndexError flow

Ordinary failed slice indexing uses slice-index bounds panic; protected
indexing uses canonical IndexError.OutOfBounds through existing Result flow.
`slice_source_index_test.go` and schema-12 guard tests distinguish these from
RangeError and stale-reference failure. There is no parallel BoundsError type.

## 25. Reference-validity ordering

Canonical P15 guard identity and its success edge must dominate dynamic
length/range/index/element observation. Stale-reference failure remains
InvalidReferenceGeneration and is not caught as a spatial error. Source
injected epoch tests and composed white-box tests are labelled separately
from real Arena source allocation, which is not claimed here.

## 26. Returned-slice origin summary

Sema retains external origins and exact/conservative ranges, including
alternative roots; local escape is rejected. Represented MLIR return checking
follows slices through construction/reborrow/transfer/element Places to allowed
parameter roots. Complete Semantic IR returned-range/origin verification is
P16-27 and general source signatures/returns are P16-36. MD-084/MD-085 remain
open. Sema-only origin evidence is not full returned-slice lowering.

## 27. Schema-v12 types/ops

`SecTypes.td`, `SecSliceOps.td`, `SliceOps.cpp` and the Go `slice.go` emitter
implement the two abstract slice types and all eleven represented operations.
Schema 12 is explicit; normal output remains 11. Source locations, original
SSA and represented reference/range facts survive round-trip and scalar
resolution. Multi-argument CFG successors now use the correct grouped operand
and type lists; the real-tool forwarding tests exposed the old per-operand
annotation bug. Unknown versions and unsupported downgrades return no artifact.

## 28. Slice guard verifier

`--sec-verify-slice-guards` checks range outcomes, normalized tuple use,
source identity, bounds-true element Places and dynamic-validity ordering.
Positive and 145 split negative MLIR cases exercise owning diagnostics and
absence of output. Go has operation/guard checks; complete CFG/fact/borrow
integration in its Semantic IR verifier remains P16-26. The same-date
P16-26 update now propagates unanimous reference facts through CFG block
arguments and anchored loops, withholding unknown/conflicting joins; it also
validates the granting slice facts of P15 element/subfield borrows. The subsequent same-date update verifies exact
source/index/normalized-tuple SSA identity through unambiguous CFG forwarding
using a detached proof view. Generic SSA/dominance and lifetime checks still
consume original IR, and P15 direct-reference/Place facts retain their separate
boundary. Equal constants or metadata do not equate distinct roots. Broader
joins and remaining verifier integration keep P16-26 open.

## 29. Place/reference/borrow verifier extensions

P16-43 extends the three P15 passes with shared/mutable slice holders,
derived Places/references, metadata preservation, move/end flow and represented
return roots. `VerifySliceReferences.cpp` and shared `ReferenceFlow.h` contain
the slice integration. This consumes established facts rather than performing
a second source borrow analysis or choosing the unresolved metadata policy.

## 30. P6/P8 compatibility

P6 resolves target-sized scalars inside abstract slice types without selecting
a descriptor. P8 rejected unresolved Place/reference representation before
checked-integer lowering and left no output file; since MD-087 (2026-10-22)
it lowers the representable integer computation selectively and keeps every
slice/Place/reference operation high-level, while the physical gates are
`--sec-check-lowering-support` and `--sec-lower-to-llvm`. Both 32/64 tests run before
and after P6. The composed LLVM pipeline likewise rejects unsupported slice
operations; schema-12 success does not imply slice binary support.

## 31. P13/P14/P15 integration

Source Sema tests cover nested aggregate slice origins and traits. Actual
source-to-schema tests borrow nested fixed-array fields, preserve exact
extents/granting borrows and avoid backing aggregate copies. Handwritten MLIR
covers shared aggregate operations and rejects exclusive-holder duplication.
P16-48 still owns remaining value/return integration; P16-36 source holder
fields/signatures and P16-26/27 verifier work remain separate.

## 32. Wide-type slice tests

`slice_source_wide_test.go` covers int128/int256/uint128/uint256/decimal128
elements, original wide endpoint/index inputs and target-aware unsigned
lengths on both plans. Source emission and real dialect tests retain the
carrier identity. These tests do not implement deferred decimal arithmetic
or physical element layout.

## 33. Empty-slice tests

Sema/IR/source/schema tests cover zero-length arrays, equal endpoints,
boundary-empty ranges and mutable empty reborrow. Empty references retain
origin/lifetime even while borrowing no elements. Indexing an empty view is
invalid. Dedicated empty mutable source emission passes all four verifiers.

## 34. Disjoint-range tests

Exact nested mutable subranges retain disjointness; dynamic ranges remain
conservatively overlapping unless proven otherwise. Empty ranges do not
reserve elements. `slice_disjoint_mutable.sec`, Sema overlap tests and actual
32/64 source-to-schema verification establish represented disjoint-range
behavior without runtime borrow counters.

## 35. Try/noPanic tests

Sema effects separate temporal panic from handled spatial range/index checks.
Proven accesses add no redundant spatial check; valid @noPanic paths retain
their guarantee. `slice_effects_test.go`, source try/index matrices and
composed guard tests cover the represented forms. MD-031 handler schemas and
recovery-value holder joins retain explicit rejection boundaries.

## 36. Unsupported dynamic-owner/FFI/ownership tests

P16-51 covers 36 target-aware source cases, 20 verified foreign/layout IR
cases, six real-tool physical no-artifact checks and 48 public CLI cases.
The matrix distinguishes source errors from capability limits. Pointer
extraction, membership and Reverse use canonical named P16 gates. Ordinary
user StableHandle/WeakHandle structs do not acquire an undefined slice API.
P17/P18/P24 ownership, owning storage and physical representation remain
deferred; failures return no partial module, schema bytes or output file.

## 37. CMake commands

Fresh command on 2026-10-21:

```sh
cmake --build build/sec-mlir --target check-sec-mlir -j2
```

It completed successfully against the configured build tree. This report
does not claim a new clean configure/build with invented LLVM installation
paths. Tool directories are configured locally through `LLVM_DIR`, `MLIR_DIR`
and absolute `SEC_MLIR_BIN`; their machine-specific expansions are omitted.

## 38. Exact LLVM/MLIR version

The configured `sec-mlir-opt --version` reports LLVM **24.0.0git**, optimized
with assertions. The shared LLVM/MLIR source checkout HEAD is
`252f31a6893b6a3dcb8b2a404e13933e376bf36b`; this identifies the inspected
source, not an assertion that its whole checkout is clean. Lit reports
**24.0.0dev**, CMake **4.2.3**, and Go **go1.26.8 linux/amd64**.

## 39. check-sec-mlir result

Fresh 2026-10-21 result: **136 discovered, 136 passed**. This is the complete
configured MLIR test target. It is separate from final isolated package
acceptance and the Go integration tests below.

## 40. go test ./... result

No fresh repository-wide Go result is claimed by this reporting task. The
last complete recorded run, 2026-10-19, passed 39 packages and failed four
packages/seven tests. The exact tests and owners are in the TODO baseline:
IO migration, stale Unicode generation, three dynamic-array contract fixtures
and a legacy mutex fixture. The later 2026-10-21 selected-package run passed
Sema/IR/lowering but exposed IO errors in compiler tests; its full LSP run was
stopped, so it is not a full-Go result. P16-55 must rerun the final snapshot.

## 41. End-to-end source -> schema-v12 results

Fresh command, with the existing tool configured explicitly:

```sh
SEC_MLIR_BIN="$PWD/build/sec-mlir/bin" go test ./internal/ir/semantic ./internal/lowering/secmlir -run 'TestPackage16|TestSlice' -count=1
```

Both packages passed. Actual source is analyzed on 32/64 scalar plans, built
to verified IR, emitted as explicit schema 12, checked by the real slice and
three P15 verifiers before/after P6 and round-tripped deterministically.
Coverage includes ranges, lengths, index Places, shared/mutable reborrows,
errors, wide/nominal elements, projected arrays and empty/disjoint ranges.
Injected-epoch and hand-built transfer tests are separately white-box evidence.
The P16-26 update additionally transforms a source-built module to forward a
slice holder through a block argument; its verified emission and real-tool
round-trip are explicitly white-box CFG evidence, not new source phi production.
Further cases forward complete source/index pairs and normalized reborrow
tuples through typed block arguments before/after P6 on both plans. Forged
tuples return no emitted bytes.

## 42. Deviations

The implementation follows later unsigned-length and address-stability/pin
decisions; old signed-len and legacy-backend wording is historical. Default
schema 11 and all named capability gates remain. Metadata spelling and MLIR
return-depth decisions are not invented. P16 is partial: P16-07, P16-26/27,
P16-36, P16-48 and final acceptance/governance gates remain. This report adds
no source or compiler behavior and makes no repository-wide green claim.
Governance validation still reports the two pre-existing absent fragments
`concurrency_events.yaml` and `concurrency_spawn.yaml`.

## 43. Recommendations for Package 17

Continue independent Sema transfer/destruction planning through the existing
canonical Place and ownership model. Refresh P17's older inventory: destruction
plans and transfer queries exist, and ordinary argument copies now publish
transfer facts. Finish destination and before/after state, match/branch uses,
partial construction and cleanup registration without a parallel move table.
Reuse P16 holder types/ranges; ending a holder must not destroy its elements.
Keep custom-free decisions current and P18 owning backing storage separate.
Do not promote schema 13 or source slice value paths before their predecessor
contracts and verifiers exist. P16's remaining verifier/source work can proceed
alongside independent P17 plans; resolve MD-084/MD-085 before final acceptance.
