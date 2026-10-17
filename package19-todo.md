# SEC-MLIR Package 19 — implementation TODO

Package: `SEC-MLIR-P19` — Allocation Context and Arena Semantic Lowering

Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package19.md`

Package contract: `rules/mlir/packages/sec-mlir-dialect_package19.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_arena_allocation_sync_v1.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v15.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v15.md`
- `rules/memory/arena.md`, `rules/memory/allocation.md`,
  `rules/memory/storage.md`, `rules/memory/reference_model.md`,
  `rules/analysis/effect_analysis.md`, `rules/compiler/semantic_ir.md`

This is an implementation plan, not a completion report or a language
correction. It continues `package15-todo.md` through `package18-todo.md`; it
does not declare those predecessors complete. The package's `152c772`
baseline is historical context, not today's execution snapshot.

## Inventory snapshot — 2026-10-16

Audited HEAD:

```text
9903247 (with uncommitted P15 and Codex work in the checkout)
```

| Area | Observed implementation | P19 consequence |
|---|---|---|
| Arena frontend | Substantial Sema support: `analyzer_arena.go`, `analyzer_arena_flow.go`, `Arena.FromBuffer`, owned/growable constructors, ArenaDomain identities (`ArenaDomainID`), per-domain generations through branches/loops/matches, Reset/Release invalidation and stale-use diagnostics, callee Arena-reset summaries (`ArenaParameterResets`), generic Arena requirements. | Evolve these facts into the immutable P19 state and operation plans (§§ 70–76); do not add a second Arena model. |
| Allocation context | `allocation_context_facts.go`, `allocation_facts.go`, `allocation_cause_paths.go`, `allocation_target_policy.go`, `analyzer_noalloc.go`, `interrupt_allocation.go`, MD-004 context selection, `MayAllocate` effect facts and Codex's `allocation_lowering_readiness.go` gate. | Map them onto `MayAllocate` vs `RequiresAllocationContext`, the selection order and the callable demand summary (§§ 51–64, 77–83); the readiness gate stays the rejection point until each operation lowers. |
| P15 references | Arena-backed references are dynamic-epoch with `InvalidationDomain`/`EpochDependency`; Semantic IR validates them (`ref.is-valid`, `fail.reference-generation`), but only white-box tests reach it because Arena references cannot be built from source in Semantic IR yet. | P19 makes them reachable from source (§ 102). |
| Semantic IR | No Arena type, allocation-context type, Arena operations or plans (`ResolvedArena*`, `alloc_context` absent). | Add §§ 84–99. |
| Predecessors | P15 partial; P16, P17 and P18 have TODOs only (`package16/17/18-todo.md`). | Integration items (§§ 100–104, 139) are blocked until those exist. |
| Schema | Go emission is schema 11 and C++ accepts 1–11 since P15-50/51 (2026-10-16). | Schema 15 needs schemas 11–14 first (§ 105). |

Primary implementation anchors:

```text
internal/sema/analyzer_arena*.go, arena_*.go, allocation_*.go, analyzer_noalloc.go
internal/sema/reference_plans.go (dynamic-epoch facts)
internal/ir/semantic/ (model, builder, verifier, printer, reference_use.go)
internal/lowering/secmlir/emitter.go
mlir/include, mlir/lib/Dialect/Sec
```

## Current authority, decisions and predecessor gates

Rulebooks remain authoritative. Open decisions that constrain P19 (do not
invent their semantics):

| MD | Subject | P19 handling |
|---|---|---|
| MD-034 | Allocation profile of freestanding targets. | Freestanding targets provide no positive allocation context; the bare-metal bounded profile (§ 134) and strict profile validation (§ 82) must not assume one. |
| MD-053 | Constraint/effect contracts. | Effect summaries (§§ 65–66, 136) follow current effect rules; no new contract surface. |
| MD-065 | Borrow-end marker placement. | Applies to Arena references once ordinary borrow ends are emitted. |
| MD-074 | Decided 2026-10-17: `len(...)` is `uint` like `.Len`. | Arena `Alloc[T](count)` element counts and slices follow the same no-narrowing rule. |

Predecessor gates: P15 (dynamic-epoch references), P16 (slices for
`Arena.Alloc`), P17 (ownership, destruction, cleanup), P18 (owning
dynamic-array backing). A stage needing a missing predecessor is **blocked**
with the exact missing ID, never marked done.

## Completion boundary

P19 is complete only when Arena values (borrowed, owned fixed, growable),
`New`/`Alloc`, Reset, Release, implicit destruction and nested Arenas, and the
allocation context (selection, propagation, demand summaries, profiles) are
represented in Semantic IR and schema-15 Sec MLIR, checked by the Arena,
allocation-context, dependency, effect and strict-profile verifiers, and
lowered with no universal physical Arena representation, ABI or mandatory
runtime (§§ 143–148, 162–163).

## A. Establish the execution baseline and synchronized authority

- [ ] P19-01 — Record the exact execution snapshot and predecessor matrix
  (P13–P18 actual state) (§§ 2, 164).
- [ ] P19-02 — Audit `sec_arena_allocation_sync_v1.md` paragraph by
  paragraph against `arena.md`, `allocation.md`, `storage.md`,
  `reference_model.md` and effect analysis; merge missing amendments without
  renumbering; record conflicts as MD entries (§ 1).
- [ ] P19-03 — Confirm against current Sema: Arena is a semantic builtin,
  lowercase `arena` is not a keyword, move-only ownership, ArenaDomain
  identity and the owner-state versus validity-epoch distinction (§§ 4–8).

## B. Publish immutable Arena and allocation-context plans in Sema

- [ ] P19-04 — Backing kinds mapped to canonical storage facts: borrowed
  fixed (`FromBuffer`, borrowed-backing lifetime, empty borrowed Arena),
  owned fixed (`WithCapacity`), growable (no relocation of live
  allocations); capacity units and layout source (§§ 9–20).
- [ ] P19-05 — Safe typed allocation restrictions: trivially destructible
  `T` only, `New[T]` and `Alloc[T](count)` semantics, temporary mutable
  Arena borrow, repeated allocation not invalidating prior allocations,
  zero-element and zero-sized `T`, full default initialization (§§ 21–31,
  143–145).
- [ ] P19-06 — `AllocationError` mapping, stable `Result` contract and
  allocation atomicity (§§ 32–35).
- [ ] P19-07 — Reset: dependency rule with non-lexical liveness, transition,
  no zeroing, atomicity, epoch exhaustion (§§ 36–42); reuse the existing
  generation/invalidation facts.
- [ ] P19-08 — Release: dependency rule, transition, per-backing-kind
  behavior, implicit destruction, panic cleanup boundary (§§ 43–48).
- [ ] P19-09 — Nested Arena facts (§§ 49–50).
- [ ] P19-10 — Allocation context: `MayAllocate` vs
  `RequiresAllocationContext`, selection order, lexical Arenas are not
  ambient candidates, compiler-managed local context, synchronous
  propagation, internal context parameter and direct-call propagation,
  explicit Arena context, target-provided context, no-allocation profile,
  `@noAlloc` vs missing ambient context, spawn/task/thread and foreign
  entrypoint boundaries (§§ 51–64).
- [ ] P19-11 — Ordered effects and per-operation effect summaries; Arena
  resource identity; allocation not CSE-safe and not movable across
  Reset/Release (§§ 65–69).
- [ ] P19-12 — Plans and read-only queries: Arena state plans, operation
  plans (create, allocation, reset, release) (§§ 70–76).
- [ ] P19-13 — Allocation-context callable summary, context origin, basic
  Arena demand summary, demand semantics, interprocedural demand boundary,
  strict profile validation, statically impossible allocation (§§ 77–83).

## C. Add the Semantic IR representation

- [ ] P19-14 — Semantic IR Arena type and allocation-context type (§§ 84–85).
- [ ] P19-15 — Arena operations with Place-based mutation; creation returns
  an owned value: `ArenaCreateBorrowedOp`, `ArenaCreateOwnedFixedOp`,
  `ArenaCreateGrowableOp`, `ArenaNewOp`, `ArenaAllocOp`, `ArenaResetOp`,
  `ArenaReleaseOp`, `ArenaDestroyOp` (§§ 86–96).
- [ ] P19-16 — Nested Arena IR (§ 97).
- [ ] P19-17 — Explicit allocation-context propagation and selection in
  Semantic IR (§§ 98–99).
- [ ] P19-18 — P15 integration: Arena references built from source carry
  dynamic-epoch facts and reach the existing validity CFG; P19 retires the
  white-box-only status of `TestPackage15DynamicReferenceUseValidatesOnce`
  (§ 102).
- [ ] P19-19 — P16 slice, P17 ownership and P18 dynamic-array/owning
  container integration (§§ 100–101, 103–104; blocked on P16–P18).

## D. Connect source lowering through resolved plans only

- [ ] P19-20 — Builder lowers Arena creation, `New`, `Alloc`, Reset,
  Release, implicit destruction and context selection solely from the P19
  plans; anything unplanned is a package-19 unsupported error with no
  partial module; the allocation readiness gate is narrowed per operation as
  it lowers.
- [ ] P19-21 — P17 cleanup and defer interaction; task/thread boundary
  scope; FFI retention boundary (§§ 139–142).

## E. Schema 15 and the verifiers

- [ ] P19-22 — Schema 15 version gates (after 11–14), registration,
  parser/printer; why `!sec.arena` has no backing-kind parameter (§§ 105–106).
- [ ] P19-23 — `!sec.alloc_context`, the hidden allocation-context function
  argument and the foreign ABI restriction (§§ 107–109).
- [ ] P19-24 — Operations `sec.arena.create_borrowed`, `.create_owned_fixed`,
  `.create_growable`, `.new`, `.alloc`, `.reset`, `.release`, `.destroy`,
  `sec.alloc_context.from_arena`, `.target`, `.compiler_local` (§§ 110–120).
- [ ] P19-25 — Arena state-version and ArenaDomain metadata; Arena MLIR
  effects (§§ 121–123).
- [ ] P19-26 — Verifiers: Arena, allocation-context, dependency extension,
  Arena effect, demand-summary representation, strict profile (§§ 124–131).
- [ ] P19-27 — Profiles: hosted, embedded fixed, bare-metal bounded (with
  MD-034), no-allocation (§§ 132–135).
- [ ] P19-28 — Effect-analysis, call-graph and separate-compilation metadata
  integration (§§ 136–138).
- [ ] P19-29 — Go emitter from verified Semantic IR to schema 15.

## F. Lowering without physical choices

- [ ] P19-30 — No individual Arena allocation free, no uninitialized safe
  allocation, no universal physical Arena representation or ABI, no
  mandatory runtime (§§ 143–148).

## G. Tests (package-required matrices)

- [ ] P19-31 — Source/frontend tests (§ 149).
- [ ] P19-32 — Type-restriction tests (§ 150).
- [ ] P19-33 — Allocation atomicity tests (§ 151).
- [ ] P19-34 — Reset tests (§ 152).
- [ ] P19-35 — Release/destruction tests (§ 153).
- [ ] P19-36 — Allocation-context tests (§ 154).
- [ ] P19-37 — Demand-summary tests (§ 155).
- [ ] P19-38 — Semantic IR tests (§ 156).
- [ ] P19-39 — Sec MLIR dialect tests (§ 157).
- [ ] P19-40 — Arena verifier and allocation-context verifier negative
  tests (§§ 158–159).
- [ ] P19-41 — Integration tests (§ 160).

## H. Final acceptance, governance and handoff

- [ ] P19-42 — Record the explicitly deferred items (§ 161); architecture
  rules check (§ 162) and acceptance criteria matrix (§ 163).
- [ ] P19-43 — Governance: reuse the existing Arena, allocation and lowering
  entries; record implemented/partial/remaining and known holes; keep MD
  links (MD-034, MD-053, MD-065, MD-074).
- [ ] P19-44 — Implementation report (§ 164) and the Package 20 boundary
  (§ 165).

## Recommended execution order

1. A (baseline, sync audit) — independent of predecessors.
2. B Sema plans (P19-04..13) — the existing Arena/allocation frontend makes
   this the most implementable part today; publish plans over current facts.
3. C/D for Arena values, `New`, Reset/Release and P15 dynamic-epoch
   references (P19-14..18, 20) — needs P15 (available) and P17 for
   destruction; `Alloc[T]` slices need P16.
4. E/F schema 15 — after schemas 11–14 exist.
5. G/H throughout, with blocked items marked as such.
