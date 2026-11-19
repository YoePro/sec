# SEC-MLIR-P18 Implementation Report — Owning Dynamic Arrays

> Navigation added 2026-11-19; historical evidence below is unchanged.
> Current canonical Semantic IR responsibilities: [§ 127 owning arrays](../../compiler/semantic_ir.md#127-owning-dynamic-arrays-sec-mlir-p18).
> See the [amendment crosswalk and maintenance workflow](../../../governance/semantic-ir-amendments.md)
> and [live implementation ledger](../../../governance/lowering.yaml).

- Status: report for package 18 (§ 136), written 2026-11-18 (P18-47)
- Package specification: `rules/mlir/packages/sec-mlir-dialect_package18.md`
- Work list and acceptance evidence: `package18-todo.md` (P18-45 matrix)
- Governance owner: `lowering.sec-mlir-package18` in `governance/lowering.yaml`

Package 18 is **not complete**. The acceptance matrix in `package18-todo.md`
has 40 criteria met, 4 partial, 1 open and 1 not run. Sections A and C are
partly open, D is mostly open, E–G are done and H is closed by this report.

## 1. Repository HEAD

The work is built on `c907b00c` plus the uncommitted working tree. The
earlier P18 sections A–C were built on `28780c01` (P18-01).

## 2. Local/merged P13–P17 status

- P13–P16 are closed.
- P17 was closed on 2026-11-17 with 44 items met and 1 superseded.
- All predecessors are local in this working tree.

## 3. Previous package status

Regressions stay green:

- **Go:** `internal/sema`, `internal/ir/semantic` and
  `internal/lowering/secmlir` pass in full.
- **Lit:** 238/238, which covers schema 1–13 regression inputs.
- **Emission:** schemas 11–13 still emit.

## 4. Dynamic-array normative synchronization

- **Sync document:** `sec_dynamic_array_sync_v1.md` was audited paragraph by
  paragraph in P18-02, and `destruction.md` § 17.3(3) was added.
- **Semantic IR:** the dynamic-array amendment was installed as `semantic_ir.md`
  § 127 (sync § 22) on 2026-11-18.
- **Status index:** `language-rulebook-status.md` is updated.
- **Decisions:** MD-074 (`len`) is unchanged. MD-072 (core producers) and
  MD-135 (Append ownership on failure) remain open.

## 5. Files added

- **C++ dialect:** `mlir/include/sec/Dialect/Sec/SecDynamicArrayOps.td` and
  `mlir/lib/Dialect/Sec/DynamicArrayOps.cpp`.
- **C++ analysis passes:** `mlir/lib/Analysis/VerifyDynamicArrays.cpp` and
  `mlir/lib/Analysis/VerifyStorageTransitions.cpp`.
- **Go emitter:** `internal/lowering/secmlir/dynamic_array_emit.go`.
- **Go tests:**
  - `internal/lowering/secmlir/dynamic_array_schema14_test.go`
  - `internal/lowering/secmlir/dynamic_array_ownership_test.go`
  - `internal/lowering/secmlir/package18_matrix_test.go`
  - `internal/lowering/secmlir/package18_source_test.go`
  - `internal/sema/package18_matrix_test.go`
  - `cmd/compiler/dynamic_array_emit_test.go`
- **Fixtures:**
  - `testdata/semantic_ir/dynamic_array_schema14.sec`
  - `testdata/semantic_ir/dynamic_array_ownership.sec`
  - `testdata/semantic_ir/dynamic_array_source.sec`
  - `testdata/semantic_ir/package18_matrix.sec`
- **Lit tests:**
  - `mlir/test/Dialect/Sec/schema14-dynamic-array.mlir`
  - `mlir/test/Dialect/Sec/schema14-dynamic-array-types.mlir`
  - `mlir/test/Dialect/Sec/invalid-schema14-dynamic-array.mlir`
  - `mlir/test/Dialect/Sec/invalid-schema14-dynamic-array-protocol.mlir`
  - `mlir/test/Dialect/Sec/invalid-schema14-storage-transitions.mlir`
  - `mlir/test/Dialect/Sec/invalid-schema13-dynamic-array-type.mlir`

Sections A–C added their own Semantic IR and Sema files on 2026-11-15:

- `internal/ir/semantic/dynamic_array_*.go`
- `internal/sema/dynamic_array_*.go`

## 6. Files modified

**C++ dialect:**

- `SecTypes.td` and `SecTypes.cpp`: the type.
- `SecOps.td`: the include.
- `SecDialect.cpp`: the schema-14 gate.
- `OwnershipOps.cpp`: the move-only rejections.
- `ReferenceBoundaries.cpp`: the `dynamic-array-index` bounds endpoint.
- `SliceOps.cpp`: the dynamic-array slice source.

**C++ analysis:**

- `DestructionFacts.h` and `VerifyDestructionPlans.cpp`: the dynamic-array
  plan kind.
- `VerifyPlaceReferences.cpp`: Place consumers and the element guard.
- `VerifySliceGuards.cpp`: the runtime-length extent.
- `Passes.td` and `Passes.h`: the two new passes.
- `CMakeLists.txt`.

**Go:**

- `emitter.go`, `emitter_options.go` and `ownership.go`: schema 14, type text
  and plan backing.
- `internal/mlir/toolchain.go`: the CLI verifier list.
- `cmd/compiler/native_pipeline.go`: inspection at package 18 and schema 14.
- `internal/ir/semantic/dynamic_array_ops.go` and `builder.go`: field-path
  owners, IsEmpty and the named boundaries.
- `internal/sema/allocation_facts.go`: allocation-free cleanup of a T[].
- `internal/sema/compiler_known_members.go`: the substituted `Option` payload.

**Tests whose expectations changed:**

- `reference_emit_boundary_test.go`: schema 14 output, and the
  beyond-package case is now an Arena Alloc.
- `slice_verification_matrix_test.go`, `emitter_options_test.go` and
  `ownership_capabilities_test.go`: 15 is the unknown schema.
- `invalid-schema12-slice-ops.mlir`: the new source message.

## 7. T[] type-model changes

- `!sec.dynamic_array<T>` is parameterized only by its element type.
- Semantic IR `TypeDynamicArray` is unchanged from P18-09.

## 8. MoveOnly classification

- Sema classifies `T[]` as MoveOnly for every `T` (S1007).
- `sec.own.copy` and `sec.own.semantic_copy` reject the type.

## 9. Empty-default implementation

- `let mut v: T[]` lowers to `dynamic-array.empty`, which does not allocate.
- An immutable binding without an initializer is S1047.
- Dynamic-array cleanup now counts as allocation-free when the element's
  cleanup is, so the empty default is valid under `noalloc`.

## 10. Dynamic-array destruction plan

- The plan has `kind = "dynamic-array"`, one element plan and a `backing` list:
  `[destroy-elements-reverse?, end-domain, reclaim-per-allocation-plan?]`.
- `--sec-verify-destruction-plans` requires `destroy-elements-reverse` exactly
  when the element needs destruction.

## 11. DynamicArrayFacts representation

- `ResolvedDynamicArrayPlan` (P18-05) carries the facts.
- `BackingMetadata` on operations carries domain, transition, allocation,
  allocation context, origin, relation, authority, reclamation, stability and
  space.

## 12. Allocation-context integration

- Every `Append` publishes a `ResolvedAllocationOperationPlan` in the active
  context.
- Hosted and embedded-arena contexts are Arena contexts.
- A reachable allocation under `noalloc` is a compile-time diagnostic.

## 13. AllocationError CFG

- `grow_backing` yields `failed` and `core::AllocationError` and is tested
  immediately.
- The failure edge maps to `CollectionError.AllocationFailed` without a panic
  and without a storage transition.

## 14. Empty allocation-free state

`dynamic-array.empty` has length 0, capacity 0 and no backing.

## 15. Allocated empty-capacity state

`dynamic-array.allocate` and `sec.dynamic_array.allocate` exist and are
verified. No source construct produces them yet (P18-18, MD-072).

## 16. Initialized-prefix tracking

- `append_prepared` initializes the element at the old length, then increments.
- It is reachable only after `has_capacity` or a completed transition
  (`--sec-verify-dynamic-arrays`).
- Element Places are guarded by the length.

## 17. DynamicArrayAllocateOp

The op is implemented with the following properties:

- **Contract:** candidate, `failed` and `AllocationError`; count, allocation
  identity, context and the establish-domain facts.
- **Protocol:** the candidate is never used on its failure edge.

## 18. Internal append-prepared implementation

- Single `Append` of a trivially copyable element lowers.
- Range `Append` and non-trivial elements (MD-135) remain open (P18-17).

## 19. Internal grow-backing implementation

- An owner without backing grows with `establish-domain`.
- An owner without spare capacity grows with `advance-epoch`.
- In-place growth and `replace-domain` are not selected; growth is
  conservative.

## 20. Transactional growth

- A failed growth leaves the owner unchanged.
- `--sec-verify-storage-transitions` rejects a transition on the failure edge.

## 21. Storage-domain establishment

`storage.establish_domain` must open the success edge of the matching
`grow_backing`. It requires owned relation, a known authority, stability,
space and the reclamation pairing.

## 22. Epoch advancement/replacement

- `storage.advance_epoch` runs only after a true `has_backing`.
- Live element and slice borrows across growth are rejected in Sema (S1162)
  and in MLIR.

## 23. Reclamation-plan implementation

`storage.reclaim` consumes an owning-value plan and must directly follow the
`end_domain` or `advance_epoch` of the same owner and domain.

## 24. Arena versus owning-value reclamation

- Arena authority never reclaims individually.
- Owning-value authority pairs a reclamation plan. It is verified in MLIR
  fixtures, but no source context selects it (MD-034).

## 25. Dynamic-array length operations

- `.Len` and `len(...)` lower to `dynamic-array.length` (uint, MD-074).
- `.IsEmpty` lowers to `length == 0`.
- `len_int` and `capacity_internal` exist in schema 14 without a source
  producer.

## 26. Index/place implementation

- `dynamic-array.index-in-bounds` guards `dynamic-array.element-place`.
- Element Places carry the owner's authority and backing domain.
- Reads, writes, compound writes and try indexing (IndexError) are supported.

## 27. P15 element borrow integration

- `ref values[i]` and `ref mut values[i]` borrow the guarded element Place.
- Sema reference facts naming the backing domain as their epoch remain open
  (governance).

## 28. P16 slice-borrow integration

- `sec.slice.borrow_shared` and `borrow_mut` accept a dynamic-array owner
  Place from schema 14.
- The extent is `dynamic_array.len` of the same owner
  (`--sec-verify-slice-guards`).

## 29. P17 move/replacement/cleanup integration

These operations now work for `T[]` (P18-30):

- tracked owned locals;
- moves and consuming arguments that cancel source cleanup;
- Result- and Option-carried returns;
- union payloads;
- transactional `own.replace_place` under the dynamic-array plan.

## 30. Schema-v14 types/ops

- **Type:** `!sec.dynamic_array<T>`.
- **Operations:** `sec.dynamic_array.{empty, allocate, len, len_int,
  capacity_internal, has_backing, has_capacity, index_in_bounds,
  element_place, append_prepared, grow_backing}`.
- **Gating:** every operation and the type require schema 14.

## 31. Storage transition ops

- `sec.storage.{establish_domain, advance_epoch, end_domain, reclaim}`.
- Their backing facts are typed properties.

## 32. Dynamic-array verifier

`--sec-verify-dynamic-arrays` checks the growth protocol: the immediate
`failed` test, the candidate never on its failure edge, transitions on the
success edge after the failed state test, and append only after ensured
capacity.

## 33. Storage-transition verifier

`--sec-verify-storage-transitions` checks:

- one domain per owner;
- no dependence after `end_domain`;
- paired reclaim;
- advance only on a live domain;
- borrow exclusion across growth;
- no transition on failed growth;
- agreeing establishment facts.

## 34. Existing verifier extensions

| Verifier | Extension |
|---|---|
| `--sec-verify-places` | Place consumers and the element-guard dominance |
| `--sec-verify-slice-guards` | runtime-length extent |
| `--sec-verify-destruction-plans` | dynamic-array plans |
| Local verifiers | own.copy/semantic_copy rejection; `fail.bounds dynamic-array-index` |

## 35. noalloc profile tests

`TestPackage18AllocationPlanMatrix` and `TestPackage18SemaTypeMatrix` cover
two cases:

- a reachable `Append` is rejected;
- the empty default is accepted.

## 36. Wide-element tests

`schema14-dynamic-array-types.mlir` round-trips three element types:

- `si128`;
- a struct element;
- a nested dynamic-array element.

## 37. Non-trivial element destruction tests

- `Handle[]` plans destroy elements in reverse first.
- `TestPackage18Matrices` §125 and the destruction-plan verifier cover it.
- Appending non-trivial elements waits for MD-135.

## 38. Relocation/borrow tests

`TestPackage18Matrices` §§124/126, `TestPackage18BorrowAndElementMatrix` and
`invalid-schema14-storage-transitions.mlir` cover three properties:

- the relocation transition is recorded;
- S1162 blocks growth under a live borrow;
- borrow exclusion is enforced in MLIR.

## 39. Unsupported public-growth API tests

`TestPackage18UnsupportedSurface` rejects:

- Push, Reserve, Capacity(), capacity and Clone();
- element literals;
- implicit fixed-array-to-owning and slice-to-owning conversions.

## 40. CMake commands

Build with `cmake --build build/sec-mlir -j 4`.

## 41. Exact LLVM/MLIR version

LLVM/MLIR `24.0.0git` at llvm-project `252f31a6893b`.

## 42. check-sec-mlir result

`llvm-lit -j 4 build/sec-mlir/test`: 238/238 passed.

## 43. go test ./... result

The full suite was **not run** for this report, because it needs the owner's
authorization. These ran instead, all green:

- `internal/sema`, `internal/ir/semantic` and `internal/lowering/secmlir`;
- the `cmd/compiler` subsets for emission, inspection and allocation;
- the `cmd/lsp` subsets for Option, collection and hover.

## 44. End-to-end source → schema-v14 results

Four fixtures emit and pass every verifier on 32- and 64-bit plans:

- `dynamic_array_source.sec`;
- `dynamic_array_schema14.sec`;
- `dynamic_array_ownership.sec`;
- `package18_matrix.sec`.

`emit-sec-mlir` publishes schema 14 for `linux-amd64` and `linux-armv7`.

## 45. Deviations

1. **Extra operations.** `has_backing` and `has_capacity` were added for the
   Semantic IR growth tests. `capacity_internal` exists, but the growth
   protocol does not use it.
2. **Transition spelling.** Transitions are validated string properties, not
   a custom enum attribute.
3. **Owner operands.** Owners are Place operands, matching Semantic IR, rather
   than dynamic-array values.
4. **Clear and RemoveAt.** No schema-14 operation removes elements or shrinks
   the length, so both stop at named boundaries.
5. **SizeOf and Ptr.** SizeOf waits for P24's element stride; Ptr waits for
   the P18-25 raw-pointer boundary.
6. **Growth.** Growth is conservative: always `advance-epoch`, never in-place,
   never `replace-domain`.
7. **Allocator context.** No allocator-backed context is selected (MD-034).

## 46. Recommendations for Package 19

- Follow § 137: allocation contexts, explicit `ref mut Arena`,
  `Arena.Alloc[T]`, the Arena storage domain and epoch, Reset, Release and
  destruction, and high-level `!sec.arena` operations.
- Unblock Arena allocation from source in Semantic IR first. It currently
  stops at "allocation lowering is unsupported", which also blocks P17's
  deferred P19-21 source test.
- Resolve MD-034's configuration gap so an allocator-backed context can be
  selected.
- Resolve MD-135 before growth of non-trivial elements.
- Add a length-reducing operation family, required by Clear and RemoveAt.
