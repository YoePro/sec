# SEC-MLIR Package 18 — implementation TODO

Package: `SEC-MLIR-P18` — Owning Dynamic Array Semantic Value Representation

Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package18.md`

Package contract: `rules/mlir/packages/sec-mlir-dialect_package18.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_dynamic_array_sync_v1.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_dynamic_array_v1.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v14.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v14.md`
- `rules/collections/collections.md`, `rules/memory/allocation.md`,
  `rules/memory/storage.md`, `rules/memory/destruction.md`,
  `rules/compiler/semantic_ir.md`

This is an implementation plan, not a completion report or a language
correction. It continues `package15-todo.md`, `package16-todo.md` and
`package17-todo.md`; it does not declare those predecessors complete. The
package's `152c772` baseline is historical context, not today's execution
snapshot.

## Inventory snapshot — 2026-10-16

Audited HEAD:

```text
9903247 (with uncommitted P15 and Codex work in the checkout)
```

| Area | Observed implementation | P18 consequence |
|---|---|---|
| Source/Sema type | `T[]` is a Sema `DynamicArrayType` with compiler-known members (`Append`, `Clear`, `RemoveAt`, `.Len`; `Reserve`/`WithCapacity` belong to list/map/set — corrected 2026-11-07), collection capacity/allocation facts and the MD-004 allocation-context machinery. | Evolve these into the immutable P18 plans; do not add a second collection model. |
| Allocation facts | `allocation_facts.go`, `allocation_context_facts.go`, `collection_allocation.go`, `collection_capacity.go`, `allocation_target_policy.go` and the Codex `allocation_lowering_readiness.go` gate exist. | `ResolvedAllocationOperationPlan` must consume them; the readiness gate stays the rejection point until each operation lowers. |
| Semantic IR | `T[]` is rejected as a type at every package (`type int[]`); P15 rejects dynamic-array element and slice borrows (P15-67 test). No dynamic-array, allocation or storage-transition operations exist. | Add the P18 type, operations and verifiers (§§ 52–80). |
| P15 | Places, storage/domain/epoch identities, dynamic-epoch references and static storage exist; P15 is partial. | Element Places and epoch-dependent references build on P15 (§§ 74–77, 100). |
| P16 | `package16-todo.md` exists; canonical slices are predecessor work. | Slice borrowing from `T[]` (§§ 46–48, 101) waits for P16. |
| P17 | `package17-todo.md` exists; no `ResolvedDestructionPlan`/`ResolvedTransferPlan` or `sec.own.*`/`sec.cleanup.*` operations exist. | Destruction/cleanup/replacement (§§ 54, 97–99) wait for P17. |
| Schema | Go emission was schema 10 at this snapshot; since P15-50/51 (2026-10-16) it is schema 11 and C++ accepts schemas 1–11 with the Place/reference types. | Schema 14 needs the P15–P17 schemas (11–13) first (§ 81). |
| Legacy backends | Removed 2026-10-17 (MD-101): the single track rejects dynamic arrays until P18 lowers them; the earlier legacy dynamic-array support is gone. | Keep them operational or reject explicitly; never treat them as P18 evidence. |

Primary implementation anchors:

```text
internal/sema/types.go, compiler_known_members.go, type_references.go
internal/sema/allocation_*.go, collection_allocation.go, collection_capacity.go
internal/ir/semantic/ (model, builder, verifier, printer, place_reference.go)
internal/lowering/secmlir/emitter.go
mlir/include, mlir/lib/Dialect/Sec (C++ dialect, schema gates)
```

## Current authority, decisions and predecessor gates

Rulebooks remain authoritative; the package text is implementation detail
where no conflict exists. Open decisions that constrain P18 (do not invent
their semantics):

| MD | Subject | P18 handling |
|---|---|---|
| MD-074 | Decided 2026-10-17: `len(...)` returns `uint`, the same count as `.Len` (`rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md`). | `.Len` and `len(T[])` lower to the same unsigned length; never narrow, wrap, saturate or panic implicitly. |
| MD-034 | Allocation profile of freestanding targets. | Freestanding targets keep no positive allocation context; allocating operations are rejected there. |
| MD-072 | `FromByteArray`/`FromRuneArray`/`ToByteArray`/`ToRuneArray` signatures and error family (P23-D2). | § 73 core materialization integration waits for it. |
| MD-050 | Invalid UTF-8 policy. | Affects § 73/§ 133 string-producing paths only. |
| MD-065 | Borrow-end marker placement. | Applies to element references (§ 77) once markers are emitted for ordinary borrows. |

Predecessor gates: P15 (Places, epochs, references), P16 (slices), P17
(transfer, destruction, cleanup). A stage needing a missing predecessor is
**blocked** with the exact missing ID, never marked done.

## Completion boundary

P18 is complete only when owning `T[]` values, their empty/allocated states,
explicit allocation through producing operations, internal append/growth,
indexing, element Places/references, length, destruction and backing
transitions are represented in Semantic IR and schema-14 Sec MLIR, verified
by the dynamic-array and storage-transition verifiers, and lowered without
hidden allocation, mandatory runtime, physical descriptor or allocator choice
(§§ 115–118, 134–135).

## A. Establish the execution baseline and synchronized authority

- [x] P18-01 — Record the exact execution snapshot and predecessor matrix
  (P13–P17 actual state) before any P18 change (§§ 2, 136).
- [x] P18-02 — Audit `sec_dynamic_array_sync_v1.md` paragraph by paragraph
  against `collections.md`, `allocation.md`, `storage.md`, `destruction.md`
  and `semantic_ir.md`; merge missing amendments without renumbering, and
  record conflicts as MD entries (§ 1).

  Done 2026-11-07 (Claude), sections A as a whole:
  - **P18-01 snapshot.** HEAD `28780c01` plus the uncommitted P17 working
    tree. P13 53/53, P14 77/77, P15 76/76, P16 58/58 done; P17 62/64 (P17-41
    defer captures, P17-49 FFI metadata open). Go emits schemas 11–13, C++
    accepts 1–13; schema 14 does not exist. Semantic IR stops `T[]` as the
    structured unsupported "dynamic array type int[]".
  - **P18-02 audit.** Sync §§ 1–12 and 15–20 agree with `collections.md`
    §§ 6, 12, `copy_move.md` § 4.6, `default_values.md` and `storage.md`
    §§ 20–23. Sync § 16 `AllocationError` is the allocation layer; the public
    failure is `CollectionError` (collections.md §§ 6.4, 6.7; package § 15), so
    no conflict. `destruction.md` § 17.3 gained (3) for reverse logical order and
    the reclamation-authority split (sync §§ 13–14). Sync § 21 waits on MD-072;
    § 22 Semantic IR docs is a governance remaining item. No new MD.
  - **P18-03 Sema.** `T[]` is MoveOnly and non-trivially destructible for every
    `T` (S1007 on implicit copy, `:<-` moves), `let mut v: T[]` is the empty
    owner, `let v: T[]` is S1047. Members: Len, IsEmpty, Ptr, SizeOf, ToString,
    Append, Clear, RemoveAt; `Reserve`/`WithCapacity`/`Capacity` exist only for
    list/map/set.
  - **P18-04.** MD-074 unchanged (`len` and `.Len` are the same `uint`); no
    public capacity, growth not observable. Recorded in
    `governance/lowering.yaml` `lowering.sec-mlir-package18`.
- [x] P18-03 — Confirm the canonical source type `T[]`, its ownership
  classification (MoveOnly owner, no implicit copy), explicit duplication
  boundary and the wide builtin invariant against current Sema (§§ 3–6).
- [x] P18-04 — Record MD-074 handling for `len(T[])`, the absent public
  capacity and the non-observable growth policy in governance (§§ 12, 14, 51,
  111).

## B. Publish immutable dynamic-array and allocation plans in Sema

- [x] P18-05 — `ResolvedDynamicArrayPlan`: element type, ownership and copy
  classes, empty-default and mutable-default facts, destruction requirement
  and backing-domain identity; read-only query API (§§ 7–11, 55).

  Done 2026-11-07 (Claude): `ResolveDynamicArrayPlan` (pure, type level) and
  `ResolvedDynamicArrayPlanOf(expr)` in `internal/sema/dynamic_array_plans.go`.
  MoveOnly and destruction required for every `T`; element copy class and
  element destruction plan; empty default that allocates nothing and constructs
  no element; mutable default declaration / immutable needs an initializer.
  An owner Place carries its descriptor storage and a separate backing
  `InvalidationDomainID`, allocated when Sema publishes the Place (never by
  the query); fields of one struct share storage but not backing; a
  runtime-indexed owner has no static backing identity. Capacity, context,
  backing relation and reclamation stay with the producing operation
  (P18-06/07). Evidence: `TestDynamicArrayPlans`.
- [x] P18-06 — `ResolvedAllocationOperationPlan` for every producing
  operation (`Append`, non-empty literal and conversion producers; `T[]` has
  no `WithCapacity`/`Reserve`, collections.md § 6.3, corrected 2026-11-07): allocation context from the MD-004 machinery, failure channel
  (`AllocationError`/`CollectionError` as the rules declare), no-allocation
  profile rejection and default context (§§ 16–19, 56–57).

  Done 2026-11-07 (Claude): `ResolvedAllocationOperationPlan` and
  `ResolvedAllocationPlanOf(expr)` (§ 57 name) in
  `internal/sema/dynamic_array_allocation_plans.go`. `Append` is the only
  Sec 0.1 producer that gives `T[]` backing (nonempty literals are S1155,
  no WithCapacity/Reserve, byte[]/rune[] conversions are MD-072's). Each
  resolved `Append` records one plan: single or compact range append with
  its exact count, `MayAllocate` false for an empty range, the site's active
  allocation context and origin (the Arena in hosted), public
  `CollectionError` over allocation-layer `AllocationError`, and the
  receiver's backing domain. A missing context is rejected by the existing
  reachable-graph policy (S1108 under noalloc, verified); freestanding has no
  context until MD-034. Evidence: `TestDynamicArrayAllocationPlans`
  (hosted and noalloc).
- [x] P18-07 — Allocation and backing identities: value ownership versus
  backing reclamation, empty backing relation, backing allocation relation,
  collection object versus backing domain (§§ 20–24).

  Done 2026-11-07 (Claude): `internal/sema/storage_relations.go`
  (BackingRelation, ReclamationAuthority, AllocationIdentityID,
  AllocationContextID, ReclamationPlanID). Each possibly allocating Append has
  its own allocation identity, Owned backing, the context's authority (Arena in
  hosted, Unknown without context) and a ReclamationPlanID only for
  OwningValue; the empty owner is None/None. Evidence:
  `TestDynamicArrayAllocationPlans`, `TestDynamicArrayPlans`.
- [x] P18-08 — Backing transitions as resolved facts: establishment,
  in-place mutation, in-place growth, relocation preserving logical identity,
  logical replacement, destruction transition, reclamation pairing (§§ 25–31).

  Done 2026-11-07 (Claude): `ResolvedBackingTransitionPlan` per Append
  (establish when absent, none within capacity, advance-epoch on growth, none
  on failure; § 29 choice: the owner's BackingDomain is the logical domain, so
  no replace-domain) and `DestructionTransitions` on the array plan
  (destroy-elements-reverse, end-domain, reclaim-per-allocation-plan). Element
  access records BackingTransition none.
- [x] P18-09 — `ResolvedDynamicArrayIndexPlan`: evaluated root and index,
  bounds proof or runtime check, failure endpoint, consumed by P15 element
  Places (§§ 40–42).

  Done 2026-11-07 (Claude): `internal/sema/dynamic_array_index_plans.go`;
  runtime check, panic operation dynamic-array-index or IndexError under try,
  owner backing domain. `.Len`-refinement proofs remain a governance item.
  Evidence: `TestDynamicArrayIndexPlans`.
- [x] P18-10 — Element move-out restriction, element replacement and element
  borrowing plans; slice-borrow source facts recorded for P16 (§§ 43–48).

  Done 2026-11-07 (Claude): the index plan's use/action follow write, ref and
  ref mut contexts with epoch dependency for borrows; runtime-indexed move-out
  stays rejected and S1007 now points to borrowing. Slice borrows of T[] use
  the existing P16 slice plans.
- [x] P18-11 — Direct references during structural mutation: epoch
  dependency of element references and conflicts with growth/replacement;
  stable handles stay out of scope (§§ 38–39, 77).

  Done 2026-11-07 (Claude, verified): Append across a live element or slice
  borrow is S1162. Naming the backing domain in P15/P16 reference epoch
  facts is left to P18-15/19 (governance).
- [x] P18-12 — Effects: allocation effects per operation, `@noPanic` for
  runtime index failure and allocation failure paths, no hidden allocation
  (§§ 71, 113–115).

  Done 2026-11-07 (Claude, verified): see
  `TestDynamicArrayIndexRestrictionsAndEffects`.

## C. Add the Semantic IR representation

- [x] P18-13 — Semantic IR dynamic-array type distinct from fixed arrays and
  slices; printer, verifier, type interning (§ 52). Done 2026-11-15:
  `dynamic-array<!T>` (dynamic_array_types.go), package-18 gate; package 17
  still reports "dynamic array type". Sec-MLIR stops at "semantic type
  dynamic-array" until schema 14 (P18-27/28).
- [x] P18-14 — Move and destruction-plan representation of `T[]` values,
  connected to P17 plans when available (§§ 53–54). Done 2026-11-15:
  move-only copy class, non-trivial `dynamic-array element=…` destruction
  plan (Sema DestructionDynamicArray), cleanup.track-owned and by-value
  moves through the P17 ownership ops.
- [x] P18-15 — Storage transition operations: `StorageEstablishDomainOp`,
  `StorageAdvanceEpochOp`, `StorageEndDomainOp`, `StorageReclaimOp`
  (§§ 58–62). Done 2026-11-15: `storage.establish-domain`,
  `storage.advance-epoch`, `storage.end-domain`, `storage.reclaim` with
  BackingMetadata (domain, origin, relation, authority, reclamation plan,
  stability, space) and a CFG verifier; Append emits establish/advance (and
  reclaim under OwningValue); the T[] destruction plan carries
  `backing=[…end-domain,reclaim-per-allocation-plan]`. Explicit end/reclaim
  emission when destruction is expanded belongs to section F.
- [x] P18-16 — Allocation backing operation and the checked allocation
  convention (candidate, failure, reason) (§§ 63–64). Done 2026-11-15:
  `dynamic-array.allocate` (owned candidate, failed, AllocationError; the
  candidate never reaches its failure edge) and the convention on
  `dynamic-array.grow-backing` (failed -> AllocationError path ->
  CollectionError.AllocationFailed, no panic). Producers are P18-18.
- [ ] P18-17 — Internal append and growth operations, growth transition
  application, element ownership during growth, non-relocatable elements,
  address stability (§§ 35–37, 65–70). Update 2026-11-15 (Claude): single
  `Append` of a trivial element lowers (has-backing / has-capacity /
  grow-backing / storage transition / append-prepared, Movable backing, no
  element moves on growth). Open: non-trivial elements (MD-135), range
  Append, non-relocatable elements (§ 69). Update 2026-11-23 (Claude): range
  Append lowers (one growth by the segment count, compact
  `dynamic-array.append-range-prepared`, count-matched capacity verifier).
  Open: non-trivial elements (MD-135), non-relocatable elements (§ 69).
- [ ] P18-18 — Construction from existing compiler/core producers; keep
  `ToByteArray`/`ToRuneArray` at the MD-072 boundary (§§ 72–73).
  Update 2026-11-23 (Claude): `ToRuneArray` of a string Place lowers from
  `ResolvedRuneArrayMaterializationPlan` (view + decode loop + per-rune growth,
  Ok/Err with prefix destruction). `ToByteArray`/`ToCharArray` and temporary
  string receivers stay named P23 boundaries.
- [x] P18-19 — Dynamic-array Place, `DynamicArrayIndexInBoundsOp`,
  `DynamicArrayElementPlaceOp`, element reference epoch dependency (§§ 74–77).
  Done 2026-11-15: `dynamic-array.index-in-bounds` and the guarded
  `dynamic-array.element-place` (owner authority, `backing=#domain`) for
  reads, plain/compound writes, try indexing (IndexError) and direct shared/
  mutable element borrows. Sema reference facts naming the backing domain as
  epoch and paths through an element remain governance items.
- [x] P18-20 — Empty owner slicing and indexing; no null owning-array value
  (§§ 78–80). Done 2026-11-15: `ref values[..]`/`ref mut values[a..b]` borrow
  the owner Place with `dynamic-array.length` as extent (no allocation); an
  empty owner's index fails through the ordinary bounds path; the empty owner
  is the owned `dynamic-array.empty`, never a null value.
- [x] P18-21 — Length operations: `.Len` and `len(...)` are both `uint` and the
  same count (MD-074, decided 2026-10-17) (§§ 13–14, 49–50). Done 2026-11-15:
  both lower to `dynamic-array.length` over the owner's Place (no move).
- [ ] P18-22 — Partial construction, safe empty-with-capacity state and
  element destruction order (§§ 32–34).
  Update 2026-11-23 (Claude): ToRuneArray is the first partial-construction
  producer: a failed growth destroys the initialized prefix through the
  owner's destruction plan before propagating the failure (§ 33). Open:
  non-trivial elements (no producer yet).

- 2026-11-23 (Claude): T[] index bounds proofs (§ 41) — dominating
  `i < values.Len` branch/assertion proofs for directly held owners, with
  length-event invalidation; proven element Places carry no guard.

## D. Connect source lowering through resolved plans only

- [x] P18-23 — Builder lowers `T[]` declarations, defaults, moves,
  producers, `.Len`, indexing, element writes and borrows solely from the P18
  plans; anything unplanned is a package-18 unsupported error with no partial
  module. Done 2026-11-18: every T[] form lowers from its P18 plan (dynamic-array, index, allocation, owner Places incl. stored field paths) and every unplanned form is a package-18 unsupported error with no partial module: owners through references and call results, element paths, SizeOf, Ptr, Clear, RemoveAt, ToString, core producers, for and membership (TestPackage18SourceEndToEnd, TestPackage18NestingAndBoundaries). Range and non-trivial Append stay with P18-17/MD-135.
- [x] P18-24 — Integration with P13 nesting, P14, union/Result/Option
  nesting, function parameters and results (§§ 102–105). Done 2026-11-18: struct fields (the struct plan steps to the dynamic-array plan), union/Result/Option payloads and by-value parameters and results through P17 moves; T[N] and T[] stay distinct (TestPackage18NestingAndBoundaries, TestPackage18SourceEndToEnd, TestPackage18SemaTypeMatrix).
- [x] P18-25 — Boundaries: FFI, raw pointers, iterators, equality and
  membership stay rejected or explicit (§§ 106–110). Done 2026-11-18: FFI parameters (S1079) and equality (S1019) are rejected in Sema; raw-pointer Ptr, for iteration and membership stop at named boundaries with no lowering introduced (TestPackage18BoundarySurface, TestPackage18NestingAndBoundaries).
- [x] P18-26 — Zero-sized element handling (§ 112). Done 2026-11-18: zero-sized elements keep append, length and destruction semantics through schema 14 and every verifier (TestPackage18NestingAndBoundaries).

## E. Schema 14 and the verifiers

- [x] P18-27 — Schema 14 version gates (after schemas 11–13), ODS/C++
  registration, parser/printer (§ 81). Done 2026-11-18: schema 14 is
  accepted, every P18 operation and `!sec.dynamic_array<T>` (also nested in
  type attributes, signatures and block parameters) require it, and
  `sec.fail.bounds {operation = "dynamic-array-index"}` is schema-14 only;
  ODS in SecDynamicArrayOps.td, verifiers in DynamicArrayOps.cpp.
- [x] P18-28 — `!sec.dynamic_array<T>` and the operations
  `sec.dynamic_array.empty`, `.allocate`, `.len`, `.len_int`,
  `.capacity_internal`, `.index_in_bounds`, `.element_place`,
  `.append_prepared`, `.grow_backing` (§§ 82–91). Done 2026-11-18, plus
  `.has_backing` and `.has_capacity` for the Semantic IR growth tests. Owner
  operands are Places (writable for changes); element_place carries
  bounds_kind/bounds_proof and backing_domain; counts are canonical decimals;
  own.copy rejects a dynamic array (move-only).
- [x] P18-29 — Backing transition representation and
  `sec.storage.establish_domain`, `.advance_epoch`, `.end_domain`,
  `.reclaim` (§§ 92–96). Done 2026-11-18: resolved backing facts are typed
  properties (domain, transition, allocation, allocation_context, origin,
  relation, authority, reclamation, stability, space); establishment checks
  owned relation, known authority and the owning-value reclamation pairing;
  reclaim requires an owning-value plan.
- [x] P18-30 — P17 destruction-plan, cleanup and replacement extensions;
  P15 reference and P16 slice integration (§§ 97–101). Done 2026-11-18:
  `kind = "dynamic-array"` plans with the element plan and
  `backing = ["end-domain", "reclaim-per-allocation-plan"]` (verified; a
  dynamic array always needs destruction); owned T[] locals are tracked by
  cleanup, a move (`:<-`, consuming argument) cancels the source cleanup, a
  returned owner travels in Result, and replacement is the transactional
  own.replace_place under the dynamic-array plan; element Places carry the
  backing domain; slice borrows accept a dynamic-array owner Place with the
  runtime-length extent (TestSchema14DynamicArrayOwnership,
  TestSchema14DynamicArrays).
- [x] P18-31 — Dynamic-array verifier and storage-transition verifier;
  required extensions of existing verifiers (§§ 128–130). Done 2026-11-18:
  --sec-verify-dynamic-arrays (growth protocol: allocate/grow_backing tested
  on failed, candidate never on its failure edge, transitions on the success
  edge after the failed state test, append_prepared after ensured capacity)
  and --sec-verify-storage-transitions (one backing domain per owner, nothing
  after end_domain, reclaim directly after end_domain/advance_epoch of the
  same owner and domain, advance only after a true has_backing, no element
  Place or slice across grow_backing, no transition on a failed growth,
  agreeing establishment facts); --sec-verify-places, slice guards,
  destruction plans and own.copy/semantic_copy are extended; the CLI runs both
  passes. The length/capacity relation has no visible static form (the
  descriptor is never exposed), and capacity_internal has no source producer.
- [x] P18-32 — Go emitter from verified Semantic IR to schema 14. Done
  2026-11-18 (internal/lowering/secmlir/dynamic_array_emit.go): every P18
  Semantic IR operation, the type text, destruction-plan backing steps and
  `EmitOptions{DialectVersion: 14}`; schema 13 rejects dynamic arrays by name.
  emit-ir/emit-sec-mlir inspect at package 18 / schema 14
  (TestSchema14DynamicArrays, TestDynamicArraysReachTheInspectionBoundaries).

## F. Lowering without physical choices

- [x] P18-33 — No mandatory heap/runtime, no physical descriptor lowering,
  no physical allocator lowering; capability diagnostics on targets without
  an allocation context (MD-034; selection syntax MD-139) (§§ 116–118).

## G. Tests (package-required matrices) Done 2026-11-18: the schema-14 module names no heap, registry, LLVM struct, memref or allocator call; --sec-lower-to-llvm rejects !sec.dynamic_array as [unsupported-lowering]; a reachable allocation under noalloc is a compile-time diagnostic while the empty default stays valid (dynamic-array cleanup is allocation-free when the element cleanup is). The unresolved freestanding profile is the MD-034 configuration gap (platform.target-profiles) (TestPackage18NoPhysicalLowering, TestPackage18AllocationPlanMatrix).
- [x] P18-34 — Sema/type tests (§ 119). Done 2026-11-18: TestPackage18SemaTypeMatrix covers every § 119 row.
- [x] P18-35 — Allocation-plan tests (§ 120). Done 2026-11-18: TestPackage18AllocationPlanMatrix: explicit plan, noalloc error, empty default in noalloc, AllocationError identity with InvalidSize, CollectionError mapping without panic, Arena context and reclamation pairing; no allocator selection is P18-33. Boundaries: no allocator-backed (individually reclaimable) context is selected (MD-034; selection syntax MD-139), and element alignment is a P24 layout fact.
- [x] P18-36 — Dynamic-array invariant tests (§ 121). Done 2026-11-18: TestPackage18Matrices §121 rows; the nonzero-capacity empty state and construction failure are boundaries of P18-18/P18-22 (no source producer).
- [x] P18-37 — Indexing tests (§ 122). Done 2026-11-18: TestPackage18Matrices §122 rows plus TestPackage18BorrowAndElementMatrix (move-only read rejected); guard dominance by --sec-verify-places and the invalid-schema14 lit tests.
- [x] P18-38 — Slice integration tests (§ 123; needs P16). Done 2026-11-18: TestPackage18Matrices §123 rows plus TestPackage18BorrowAndElementMatrix (slice blocks growth, growth after the slice ends, replacement through a mutable slice).
- [x] P18-39 — Storage transition tests (§ 124). Done 2026-11-18: TestPackage18Matrices §124 rows; logical backing replacement (replace-domain) is never selected, so growth advances the epoch conservatively.
- [x] P18-40 — Destruction tests (§ 125; needs P17). Done 2026-11-18: TestPackage18Matrices §125 rows (reverse element destruction exactly for non-trivial elements, Arena owners never reclaim individually, move and replacement from dynamic_array_ownership.sec); the allocator-backed owner is the MD-034 boundary.
- [x] P18-41 — Growth tests (§ 126). Done 2026-11-18: TestPackage18Matrices §126 rows plus the Sema borrow rows; in-place growth is not distinguished from relocation and non-relocatable elements (§ 69) have no source type, both under P18-17.
- [x] P18-42 — MLIR dialect tests (§ 127). Done 2026-11-18:
  schema14-dynamic-array.mlir (every operation, the plan and all verifiers),
  schema14-dynamic-array-types.mlir (round-trip with wide, struct and nested
  dynamic-array elements), invalid-schema14-dynamic-array.mlir and
  -protocol.mlir (local and protocol rejections),
  invalid-schema13-dynamic-array-type.mlir (schema-13 regression).
- [x] P18-43 — End-to-end source tests on 32- and 64-bit plans (§ 131). Done 2026-11-18: testdata/semantic_ir/dynamic_array_source.sec lowers to verified schema 14 on 32- and 64-bit plans: byte[] owner, move, consuming parameter, Result/Option return, union payload, struct field owner (new field-path owner Places), Len, IsEmpty (new: length == 0), len, index read and replacement, ref [..], ref mut [..] and Append; SizeOf (P24 stride), unsafe Ptr (P18-25), Clear and RemoveAt (no length-reducing schema-14 operation) and ToString (P23) stop at named boundaries (TestPackage18SourceEndToEnd).
- [x] P18-44 — Unsupported/new-surface tests and core materialization
  integration tests (§§ 132–133). Done 2026-11-18: § 132 invented surface (Push, Reserve, Capacity(), capacity, Clone(), element literals, implicit fixed/slice-to-owning conversions) is rejected (TestPackage18UnsupportedSurface; Append is normative in collections.md § 6.4); § 133 string.ToByteArray/ToRuneArray stop at the named MD-072/P23 boundary and the substrate is not claimed (TestPackage18CoreMaterializationBoundary). compilerKnownOption now substitutes its payload so Option results such as RemoveAt intern.

## H. Final acceptance, governance and handoff

- [x] P18-45 — Architecture rules check (§ 134) and acceptance criteria
  matrix (§ 135). Done 2026-11-18: see "P18-45 architecture and acceptance evidence" below: every § 134 rule and § 135 criterion with evidence; 40 met, 4 partial, 1 open, 1 not run; Package 18 stays partial.
- [x] P18-46 — Governance: reuse existing collection/allocation/lowering
  entries; record implemented/partial/remaining and known holes; keep MD
  links (MD-074, MD-034, MD-072, MD-050). Done 2026-11-18: lowering.sec-mlir-package18 stays partial with every open item and named boundary as a remaining item; Sync § 22 closed by semantic_ir.md § 127; MD-034, MD-072, MD-074 and MD-135 links kept; report linked.
- [x] P18-47 — Implementation report (§ 136) and the Package 19 boundary
  (§ 137). Done 2026-11-18: rules/mlir/packages/sec-mlir-dialect_package18-implementation-report.md, all 46 topics in order, with the Package 19 recommendations of § 137.

## P18-45 architecture and acceptance evidence — 2026-11-18

Snapshot: HEAD `c907b00c` plus the uncommitted working tree (P18 sections C–G
on top of P13–P17 closed). Evidence names a test, verifier or file; `Partial`
and `Open` name the owner of what remains. Nothing here declares Package 18
complete.

### § 134 architecture rules

| Rule | Status | Evidence |
|---|---|---|
| T[] is an owning runtime-sized descriptor | Holds | `!sec.dynamic_array<T>` (no descriptor parameters); TestPackage18SemaTypeMatrix |
| MoveOnly regardless of element | Holds | TestPackage18SemaTypeMatrix; own.copy/semantic_copy reject (invalid-schema14-dynamic-array.mlir) |
| Implicit copy forbidden | Holds | S1007 rows in TestPackage18SemaTypeMatrix |
| Deep duplication may not hide allocation/failure | Holds | no Clone surface (TestPackage18UnsupportedSurface) |
| Empty default allocates nothing | Holds | dynamic-array.empty; noalloc row (TestPackage18AllocationPlanMatrix) |
| Immutable bindings require an initializer | Holds | S1047 row |
| Length is runtime state, not identity | Holds | identity row; type has only the element |
| Capacity internal, no source member | Holds | Capacity/capacity rejected; capacity_internal has no source producer |
| Every live element is initialized | Holds | append_prepared after ensured capacity (--sec-verify-dynamic-arrays) |
| Capacity beyond length holds no live T | Holds | element Places guarded by the length (TestPackage18Matrices §121) |
| Allocation explicit in Semantic IR | Holds | dynamic-array.grow-backing / allocate with allocation identities |
| AllocationError, not panic/null | Holds | grow_backing failed + core::AllocationError → CollectionError |
| No-allocation profiles support empty T[] | Holds | fixed 2026-11-18 (allocationCleanupCovered) |
| Value ownership vs reclamation authority separate | Holds | authority/reclamation facts on backing; storage-transition verifier |
| Arena-backed T[] never reclaims Arena bytes | Holds | Arena rows (§125); reclaim requires OwningValue |
| Allocator-backed T[] records its reclamation plan | Partial | verifiers enforce the pairing; no allocator-backed context is selected (MD-034; selection syntax MD-139) |
| Move does not imply relocation | Holds | own.move/move_from_place carry no storage transition (dynamic_array_ownership.sec) |
| Element replacement does not invalidate backing | Holds | in-place mutation row (§124) |
| Relocating growth emits an explicit transition | Holds | storage.advance_epoch on growth; protocol verifier |
| References/slices never cross invalidating mutation | Holds | S1162 rows; --sec-verify-storage-transitions borrow-exclusion |
| Slice borrowing reuses P15/P16 facts | Holds | slice.borrow_* over the owner Place, runtime-length extent |
| Element indexing produces a P15 Place | Holds | dynamic-array.element-place |
| Move-only indexed move-out deferred; RemoveAt explicit | Partial | move-out rejected (S1007); RemoveAt stops at a named boundary (no schema-14 removal op) |
| Destruction reverse order, then domain end/reclaim | Holds | plan backing steps; destruction-plan verifier |
| No invented Push/Reserve/Resize/Capacity/AsSlice API | Holds | TestPackage18UnsupportedSurface |
| No physical descriptor layout | Holds | TestPackage18NoPhysicalLowering |
| No physical allocator ABI | Holds | TestPackage18NoPhysicalLowering |
| No mandatory heap/GC/refcount/registry | Holds | TestPackage18NoPhysicalLowering |
| No LLVM dialect generated | Holds | --sec-lower-to-llvm rejects dynamic arrays as unsupported |

### § 135 acceptance criteria

| Criterion | Status | Evidence / owner |
|---|---|---|
| baseline documented | Met | P18-01; this snapshot |
| previous package regressions green | Met | sema, ir/semantic, lowering/secmlir full; lit 238/238 (schemas 1–13 inputs) |
| dynamic-array/default/copy sync applied | Met | P18-02 |
| Semantic IR dynamic-array amendment applied | Met | semantic_ir.md § 127 (2026-11-18) |
| schema-v14 dialect rulebook installed | Met | rules/mlir/dialect-versions/sec_mlir_dialect_v14.md |
| lowering-v14 rulebook installed | Met | rules/mlir/lowering-versions/sec_mlir_lowering_v14.md |
| T[] separate from fixed arrays/slices/lists | Met | identity rows |
| T[] MoveOnly | Met | TestPackage18SemaTypeMatrix |
| non-trivial destruction plan | Met | kind dynamic-array plans |
| empty non-allocating default | Met | dynamic-array.empty; noalloc |
| immutable no-initializer example corrected | Met | P18-03 (S1047) |
| .Len, .IsEmpty, unsafe .Ptr, payload .SizeOf | Partial | Len and IsEmpty lowered; Ptr (P18-25) and SizeOf (P24 stride) are named boundaries |
| len(T[]) no-truncation guard | Met | MD-074; len and .Len are the same uint |
| internal capacity represented | Met | has_capacity / capacity_internal |
| no public capacity member | Met | Capacity rows |
| allocation context plan | Met | TestPackage18AllocationPlanMatrix |
| AllocationError flow | Met | grow_backing failure CFG |
| noalloc rejects only allocating operations | Met | noalloc rows |
| allocation/reclamation pairing | Met | establishment verifiers |
| ownership/reclamation authority separated | Met | backing facts |
| DynamicArrayAllocateOp | Partial | IR + schema-14 op and verifiers; no source producer (P18-18, MD-072) |
| internal append-prepared | Met | Append lowering |
| internal grow-backing | Met | Append lowering |
| initialized-prefix invariant verified | Met | --sec-verify-dynamic-arrays |
| P17 partial construction cleanup | Open | P18-22 (no partial-construction producer) |
| P17 move/replacement/cleanup | Met | P18-30 |
| runtime index/place lowering | Met | element-place rows |
| move-only indexed extraction rejected | Met | S1007 row |
| Append, Clear, RemoveAt, ToString public surface | Partial | Append lowered (trivial elements; MD-135 for others); Clear/RemoveAt (no schema-14 op) and ToString (P23) are named boundaries |
| P15 element reference integration | Met | element Places with backing domain; borrows |
| P16 slice borrowing from T[] | Met | slice rows |
| storage establish/advance/end/reclaim ops | Met | schema-14 ops |
| relocating growth explicit transition | Met | advance_epoch |
| direct borrow blocks invalidating growth | Met | S1162; storage-transition verifier |
| Arena destruction/reclamation distinction | Met | Arena rows |
| individually reclaimable backing works | Partial | verified in MLIR fixtures; never selected from source (MD-034; selection syntax MD-139) |
| --sec-verify-dynamic-arrays registered | Met | pass + lit |
| --sec-verify-storage-transitions registered | Met | pass + lit |
| existing verifiers extended | Met | places, slice guards, destruction plans, copy |
| no invented public growth API | Met | TestPackage18UnsupportedSurface |
| no physical descriptor | Met | TestPackage18NoPhysicalLowering |
| no allocator ABI | Met | TestPackage18NoPhysicalLowering |
| no mandatory runtime | Met | TestPackage18NoPhysicalLowering |
| check-sec-mlir passes | Met | lit 238/238 |
| go test ./... passes | Not run | full suite needs owner authorization; package suites green |
| legacy paths operational | Met | schema 11–13 emission and lit regressions |

Result: 40 Met, 4 Partial, 1 Open, 1 Not run. Package 18 stays partial. (P18-23..26 closed later on 2026-11-18; the open criterion is P18-22 partial construction.)

## Recommended execution order

1. A (baseline, sync audit) — independent of predecessors.
2. B Sema plans (P18-05..12) — implementable now on current Sema; this is the
   best next step while P15–P17 are partial.
3. C/D Semantic IR for empty values, length, indexing and element Places —
   needs P15 Places (available) and P17 for destruction.
4. E/F schema 14 — after schemas 11–13 exist.
5. G/H throughout, with blocked items marked as such.
