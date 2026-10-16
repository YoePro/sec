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
| Source/Sema type | `T[]` is a Sema `DynamicArrayType` with compiler-known members (`Append`, `Reserve`, `WithCapacity`, `.Len`), collection capacity/allocation facts and the MD-004 allocation-context machinery. | Evolve these into the immutable P18 plans; do not add a second collection model. |
| Allocation facts | `allocation_facts.go`, `allocation_context_facts.go`, `collection_allocation.go`, `collection_capacity.go`, `allocation_target_policy.go` and the Codex `allocation_lowering_readiness.go` gate exist. | `ResolvedAllocationOperationPlan` must consume them; the readiness gate stays the rejection point until each operation lowers. |
| Semantic IR | `T[]` is rejected as a type at every package (`type int[]`); P15 rejects dynamic-array element and slice borrows (P15-67 test). No dynamic-array, allocation or storage-transition operations exist. | Add the P18 type, operations and verifiers (§§ 52–80). |
| P15 | Places, storage/domain/epoch identities, dynamic-epoch references and static storage exist; P15 is partial. | Element Places and epoch-dependent references build on P15 (§§ 74–77, 100). |
| P16 | `package16-todo.md` exists; canonical slices are predecessor work. | Slice borrowing from `T[]` (§§ 46–48, 101) waits for P16. |
| P17 | `package17-todo.md` exists; no `ResolvedDestructionPlan`/`ResolvedTransferPlan` or `sec.own.*`/`sec.cleanup.*` operations exist. | Destruction/cleanup/replacement (§§ 54, 97–99) wait for P17. |
| Schema | Go emission was schema 10 at this snapshot; since P15-50/51 (2026-10-16) it is schema 11 and C++ accepts schemas 1–11 with the Place/reference types. | Schema 14 needs the P15–P17 schemas (11–13) first (§ 81). |
| Legacy backends | Direct LLVM/legacy MLIR paths handle some dynamic-array forms outside Semantic IR. | Keep them operational or reject explicitly; never treat them as P18 evidence. |

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
| MD-074 | `len(...)` returns `int` while `.Len` returns `uint`; behavior when a length does not fit `int` is undefined (P18 § 14). | Full `.Len`; emit `len(T[]) -> int` only when representability is proven; never narrow, wrap, saturate or panic implicitly. |
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

- [ ] P18-01 — Record the exact execution snapshot and predecessor matrix
  (P13–P17 actual state) before any P18 change (§§ 2, 136).
- [ ] P18-02 — Audit `sec_dynamic_array_sync_v1.md` paragraph by paragraph
  against `collections.md`, `allocation.md`, `storage.md`, `destruction.md`
  and `semantic_ir.md`; merge missing amendments without renumbering, and
  record conflicts as MD entries (§ 1).
- [ ] P18-03 — Confirm the canonical source type `T[]`, its ownership
  classification (MoveOnly owner, no implicit copy), explicit duplication
  boundary and the wide builtin invariant against current Sema (§§ 3–6).
- [ ] P18-04 — Record MD-074 handling for `len(T[])`, the absent public
  capacity and the non-observable growth policy in governance (§§ 12, 14, 51,
  111).

## B. Publish immutable dynamic-array and allocation plans in Sema

- [ ] P18-05 — `ResolvedDynamicArrayPlan`: element type, ownership and copy
  classes, empty-default and mutable-default facts, destruction requirement
  and backing-domain identity; read-only query API (§§ 7–11, 55).
- [ ] P18-06 — `ResolvedAllocationOperationPlan` for every producing
  operation (`WithCapacity`, `Append`, `Reserve`, literal/conversion
  producers): allocation context from the MD-004 machinery, failure channel
  (`AllocationError`/`CollectionError` as the rules declare), no-allocation
  profile rejection and default context (§§ 16–19, 56–57).
- [ ] P18-07 — Allocation and backing identities: value ownership versus
  backing reclamation, empty backing relation, backing allocation relation,
  collection object versus backing domain (§§ 20–24).
- [ ] P18-08 — Backing transitions as resolved facts: establishment,
  in-place mutation, in-place growth, relocation preserving logical identity,
  logical replacement, destruction transition, reclamation pairing (§§ 25–31).
- [ ] P18-09 — `ResolvedDynamicArrayIndexPlan`: evaluated root and index,
  bounds proof or runtime check, failure endpoint, consumed by P15 element
  Places (§§ 40–42).
- [ ] P18-10 — Element move-out restriction, element replacement and element
  borrowing plans; slice-borrow source facts recorded for P16 (§§ 43–48).
- [ ] P18-11 — Direct references during structural mutation: epoch
  dependency of element references and conflicts with growth/replacement;
  stable handles stay out of scope (§§ 38–39, 77).
- [ ] P18-12 — Effects: allocation effects per operation, `@noPanic` for
  runtime index failure and allocation failure paths, no hidden allocation
  (§§ 71, 113–115).

## C. Add the Semantic IR representation

- [ ] P18-13 — Semantic IR dynamic-array type distinct from fixed arrays and
  slices; printer, verifier, type interning (§ 52).
- [ ] P18-14 — Move and destruction-plan representation of `T[]` values,
  connected to P17 plans when available (§§ 53–54).
- [ ] P18-15 — Storage transition operations: `StorageEstablishDomainOp`,
  `StorageAdvanceEpochOp`, `StorageEndDomainOp`, `StorageReclaimOp`
  (§§ 58–62).
- [ ] P18-16 — Allocation backing operation and the checked allocation
  convention (candidate, failure, reason) (§§ 63–64).
- [ ] P18-17 — Internal append and growth operations, growth transition
  application, element ownership during growth, non-relocatable elements,
  address stability (§§ 35–37, 65–70).
- [ ] P18-18 — Construction from existing compiler/core producers; keep
  `ToByteArray`/`ToRuneArray` at the MD-072 boundary (§§ 72–73).
- [ ] P18-19 — Dynamic-array Place, `DynamicArrayIndexInBoundsOp`,
  `DynamicArrayElementPlaceOp`, element reference epoch dependency (§§ 74–77).
- [ ] P18-20 — Empty owner slicing and indexing; no null owning-array value
  (§§ 78–80).
- [ ] P18-21 — Length operations: `.Len -> uint` always, `len -> int` only
  with proven representability (MD-074) (§§ 13–14, 49–50).
- [ ] P18-22 — Partial construction, safe empty-with-capacity state and
  element destruction order (§§ 32–34).

## D. Connect source lowering through resolved plans only

- [ ] P18-23 — Builder lowers `T[]` declarations, defaults, moves,
  producers, `.Len`, indexing, element writes and borrows solely from the P18
  plans; anything unplanned is a package-18 unsupported error with no partial
  module.
- [ ] P18-24 — Integration with P13 nesting, P14, union/Result/Option
  nesting, function parameters and results (§§ 102–105).
- [ ] P18-25 — Boundaries: FFI, raw pointers, iterators, equality and
  membership stay rejected or explicit (§§ 106–110).
- [ ] P18-26 — Zero-sized element handling (§ 112).

## E. Schema 14 and the verifiers

- [ ] P18-27 — Schema 14 version gates (after schemas 11–13), ODS/C++
  registration, parser/printer (§ 81).
- [ ] P18-28 — `!sec.dynamic_array<T>` and the operations
  `sec.dynamic_array.empty`, `.allocate`, `.len`, `.len_int`,
  `.capacity_internal`, `.index_in_bounds`, `.element_place`,
  `.append_prepared`, `.grow_backing` (§§ 82–91).
- [ ] P18-29 — Backing transition representation and
  `sec.storage.establish_domain`, `.advance_epoch`, `.end_domain`,
  `.reclaim` (§§ 92–96).
- [ ] P18-30 — P17 destruction-plan, cleanup and replacement extensions;
  P15 reference and P16 slice integration (§§ 97–101).
- [ ] P18-31 — Dynamic-array verifier and storage-transition verifier;
  required extensions of existing verifiers (§§ 128–130).
- [ ] P18-32 — Go emitter from verified Semantic IR to schema 14.

## F. Lowering without physical choices

- [ ] P18-33 — No mandatory heap/runtime, no physical descriptor lowering,
  no physical allocator lowering; capability diagnostics on targets without
  an allocation context (MD-034) (§§ 116–118).

## G. Tests (package-required matrices)

- [ ] P18-34 — Sema/type tests (§ 119).
- [ ] P18-35 — Allocation-plan tests (§ 120).
- [ ] P18-36 — Dynamic-array invariant tests (§ 121).
- [ ] P18-37 — Indexing tests (§ 122).
- [ ] P18-38 — Slice integration tests (§ 123; needs P16).
- [ ] P18-39 — Storage transition tests (§ 124).
- [ ] P18-40 — Destruction tests (§ 125; needs P17).
- [ ] P18-41 — Growth tests (§ 126).
- [ ] P18-42 — MLIR dialect tests (§ 127).
- [ ] P18-43 — End-to-end source tests on 32- and 64-bit plans (§ 131).
- [ ] P18-44 — Unsupported/new-surface tests and core materialization
  integration tests (§§ 132–133).

## H. Final acceptance, governance and handoff

- [ ] P18-45 — Architecture rules check (§ 134) and acceptance criteria
  matrix (§ 135).
- [ ] P18-46 — Governance: reuse existing collection/allocation/lowering
  entries; record implemented/partial/remaining and known holes; keep MD
  links (MD-074, MD-034, MD-072, MD-050).
- [ ] P18-47 — Implementation report (§ 136) and the Package 19 boundary
  (§ 137).

## Recommended execution order

1. A (baseline, sync audit) — independent of predecessors.
2. B Sema plans (P18-05..12) — implementable now on current Sema; this is the
   best next step while P15–P17 are partial.
3. C/D Semantic IR for empty values, length, indexing and element Places —
   needs P15 Places (available) and P17 for destruction.
4. E/F schema 14 — after schemas 11–13 exist.
5. G/H throughout, with blocked items marked as such.
