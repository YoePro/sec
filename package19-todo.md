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

## Current execution checkpoint — 2026-11-16

This checkpoint supersedes the historical inventory where they differ:
HEAD `7520210ab62ff36024a796f3076c1df4250460be`, shared dirty working tree
with Claude's P17 work and Codex's P20/P19 changes; Go 1.26.8 linux/amd64.
Canonical lowering governance records P13–P16 implemented and P17/P18 partial.
The allocation readiness gate remains closed for Arena IR. No schema-15 or
native Arena execution is claimed. P19-01/02 remain open: this checkpoint is
not the complete predecessor/synchronization acceptance audit.

`internal/sema/arena_operation_plans.go` now publishes detached constructor,
allocation, Reset and Release snapshots, reusing the existing generated Arena
identity and P15 domain/epoch allocator. Missing layout/default/provider,
state-version, epoch-exhaustion and destruction facts remain explicitly named;
the snapshots are not a lowering-readiness certificate. MD-136 blocks only
unresolved `Arena {}`/implicit-default semantics; defined constructors can
progress independently. Do not infer a backing/provider from the old internal
empty-struct representation.

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

- [x] P19-01 — Record the exact execution snapshot and predecessor matrix
  (P13–P18 actual state) (§§ 2, 164).

  Done 2026-11-18 (Claude): HEAD `c907b00c` plus the shared working tree;
  Go 1.26.8 linux/amd64; LLVM/MLIR 24.0.0git at llvm-project `252f31a6893b`.
  Predecessors: P13–P16 closed, P17 closed 2026-11-17, P18 44/47 (P18-17
  MD-135, P18-18 MD-072 and P18-22 blocked; schema 14 with dynamic arrays and
  storage transitions implemented, report
  `sec-mlir-dialect_package18-implementation-report.md`). Emission schemas
  11–14, C++ accepts 1–14; schema 15 does not exist. Lit 238/238. The
  allocation readiness gate preserves only P18 Append growth, so Arena
  New/Alloc/WithCapacity/Growable from source still stop at "allocation
  lowering is unsupported".
- [x] P19-02 — Audit `sec_arena_allocation_sync_v1.md` paragraph by
  paragraph against `arena.md`, `allocation.md`, `storage.md`,
  `reference_model.md` and effect analysis; merge missing amendments without
  renumbering; record conflicts as MD entries (§ 1).

  Done 2026-11-18 (Claude): sync §§ 1–17 are present in `arena.md` (single
  builtin type, § 7 move-only, FromBuffer/WithCapacity/Growable, the safe
  typed restriction, Result contracts, § 22 zero-element allocation, atomicity,
  epoch preservation, Reset/Release, implicit destruction, nesting); §§ 18–25
  in `arena.md` §§ 67–76 (MayAllocate vs RequiresAllocationContext, § 71
  selection order, § 70 no lexical candidates, hidden context, § 75 foreign
  ABI guard, spawn boundary) and `allocation.md` §§ 5–6, 22; § 26 is
  `arena.md` § 76; §§ 27–29 (demand summary, panic boundary, storage reuse via
  `reference_model.md`/`storage.md`) are present; `AllocationError` is
  `core-library.md`. § 30 lists implementation work tracked by P19 items.
  No missing amendment and no conflict; no new MD. Editorial only: the sync
  status names `rules/memory/allocation.txt` for `allocation.md`.
- [x] P19-03 — Confirm against current Sema: Arena is a semantic builtin,
  lowercase `arena` is not a keyword, move-only ownership, ArenaDomain
  identity and the owner-state versus validity-epoch distinction (§§ 4–8).

  Completed 2026-11-16: source-backed tests use builtin Arena without a
  declaration and ordinary lowercase arena bindings, confirm MoveOnly and
  nontrivial destruction, preserve constructor provenance through owner moves,
  and keep repeated allocation in one epoch while Reset advances and Release
  ends it. Canonical P15 identities match source dynamic-epoch reference uses.
  The matrix runs at three analysis depths and no/32/64-bit scalar plans.
  This frontend conformance does not complete Arena IR/destruction lowering.

## B. Publish immutable Arena and allocation-context plans in Sema

- [ ] P19-04 — Backing kinds mapped to canonical storage facts: borrowed
  fixed (`FromBuffer`, borrowed-backing lifetime, empty borrowed Arena),
  owned fixed (`WithCapacity`), growable (no relocation of live
  allocations); capacity units and layout source (§§ 9–20).

  Partial 2026-11-16: `ArenaCapacityFacts` retains initial byte-capacity sources
  independently of allocation element counts. Borrowed fixed arrays and static
  slices reuse canonical exact extents; dynamic byte backing and unknown Arena
  parameter backing stay distinct. Owned/growable requests preserve exact or
  runtime requested bytes without guessing provider-delivered extent. Moves
  and Reset retain the creation source and backing/growth policy. Queries detach
  arbitrary-precision counts. Provider extent, current remaining capacity,
  layout/alignment and checked allocation-fit plans remain open.
- [ ] P19-05 — Safe typed allocation restrictions: trivially destructible
  `T` only, `New[T]` and `Alloc[T](count)` semantics, temporary mutable
  Arena borrow, repeated allocation not invalidating prior allocations,
  zero-element and zero-sized `T`, full default initialization (§§ 21–31,
  143–145).
- [x] P19-06 — `AllocationError` mapping, stable `Result` contract and
  allocation atomicity (§§ 32–35).

  Completed 2026-11-16 at the Sema-plan boundary: `ArenaFailurePlan` records
  all four canonical failure-condition mappings to the existing AllocationError,
  the unchanged source Result type and either no established domain (failed
  creation) or unchanged owner/allocations (failed allocation). Unknown failure
  conditions have no implicit fallback mapping. The existing ErrorCauses view
  now derives from this one contract. FromBuffer/Reset/Release remain without
  a fallible plan; zero capacity/count and dynamic counts retain Result.
  Public queries detach mapping and nested result/error-type data. Source
  tests cover all constructors and New/Alloc at three analysis depths and
  no/32/64-bit scalar plans. Full Sema/IR/emitter suites pass. Provider execution,
  initialization, source IR producers and native lowering remain separate open
  points; this contract does not open allocation readiness.

- [ ] P19-07 — Reset: dependency rule with non-lexical liveness, transition,
  no zeroing, atomicity, epoch exhaustion (§§ 36–42); reuse the existing
  generation/invalidation facts. Epoch exhaustion decided by MD-140
  (2026-11-25): one panic policy (panic.arena-epoch-exhaustion before
  commit); stages tracked in frontend.panic, analysis.arena and
  lowering.sec-mlir-package19.

  MD-140 stage 1b completed 2026-11-25: the canonical reference policy
  supplies the full logical width (64 bits by default even on 32-bit targets)
  and permits only panic on Arena exhaustion. Shorter widths require explicit
  checked-advance safety and producer justification; target projection and
  scalar-plan validation enforce them. This plan selection is no proof of
  nonexhaustion or metadata elimination. Reset effects, Semantic IR and
  executable checked advancement remain open in their owning stage items.
- [ ] P19-08 — Release: dependency rule, transition, per-backing-kind
  behavior, implicit destruction, panic cleanup boundary (§§ 43–48).
- [x] P19-09 — Nested Arena facts (§§ 49–50).

  Completed 2026-11-16: `arena_parent_plans.go` publishes the already checked
  parent origins, canonical invalidation domains and retained epochs in child
  Arena operation plans. Borrowed backing and the parent's reclamation authority
  remain distinct: child Release ends its loan and domain, without individually
  reclaiming parent allocations. Child operations inherit creation dependencies
  by stable child domain, preserving parent facts across owner moves and child
  Reset. Joined backing preserves every possible parent in deterministic order.
  Public queries detach the parent snapshots and reanalysis clears them.
  Source tests cover named/temporary backing, moved owners, grandchildren,
  branches/loops/scope cleanup, zero-length backing, parent Reset epochs and
  joined origins, including three analysis depths and no/32/64-bit scalar plans.
  Existing nested dependency diagnostics remain the source liveness authority.
  Full Sema, Semantic IR and emitter tests pass. Source IR production still
  requires the other incomplete P19 plans; allocation readiness is unchanged.

- [~] P19-10 — Allocation context: `MayAllocate` vs
  `RequiresAllocationContext`, selection order, lexical Arenas are not
  ambient candidates, compiler-managed local context, synchronous
  propagation, internal context parameter and direct-call propagation,
  explicit Arena context, target-provided context, no-allocation profile,
  `@noAlloc` vs missing ambient context, spawn/task/thread and foreign
  entrypoint boundaries (§§ 51–64).
  Update 2026-11-23 (Claude): requirement summary, hidden context,
  direct-call forwarding, entry-root target context, spawn and foreign-ABI
  boundaries. Open: compiler-local and explicit-Arena contexts, exported
  wrappers, indirect calls.
- [x] P19-11 — Ordered effects and per-operation effect summaries; Arena
  resource identity; allocation not CSE-safe and not movable across
  Reset/Release (§§ 65–69).

  Completed 2026-11-16 at the Sema-plan boundary: each successful operation
  retains its existing ordered call-graph event, callable/direct-effect index,
  canonical domain resource and storage transition. Ordering barriers and
  CSE-unsafety are explicit; equal-looking allocations keep distinct events.
  Provider acquire/grow/reclaim effects remain explicitly unknown until the
  provider is resolved. Borrowed Release ends the loan without reclaiming
  backing; zero-count allocation requires no growth while keeping source
  MayAllocate and Result. Source tests cover moved owners, separate resources,
  Reset ordering and parameter backing across three depths and no/32/64-bit
  plans. This is the Sema effect contract; IR production, schema and physical
  optimization/provider implementation remain separate open points.
- [ ] P19-12 — Plans and read-only queries: Arena state plans, operation
  plans (create, allocation, reset, release) (§§ 70–76).
  Partial 2026-11-16 (P19-04/05/07/08/12; P19-06 completed separately): constructor/allocation/reset/release
  read-only queries publish represented decisions at successful final-body
  analysis. They retain borrowed/owned and fixed/growable facts, byte capacity
  versus element counts, stable Result/AllocationError contracts, failure
  atomicity, explicit Arena context, canonical allocation/domain/epoch IDs,
  receiver/backing Places and element defaultability/trivial destruction.
  Mutation tests cover nested type/Place facts, identities and reanalysis;
  invalid allocation/dependency forms get no operation plan. Unresolved facts
  stay named instead of defaulted. Remaining: full layout/default and checked
  extent/capacity/growth plans, provider/reclamation and retained-borrow
  lifetime contracts, state versions, epoch exhaustion/rekey and implicit
  destruction. P19-04/05/07/08 and P19-12 therefore remain unchecked.

  Partial update 2026-11-16 (P19-10/12/28): allocation-context analysis facts
  now consume stored successful Arena operation contexts. Explicit selected
  domains stay distinct from constructor-created domains and ambient backing
  context, including in diagnostics presentation. Known semantic availability
  does not certify a physical provider. Missing/conflicting plans stay unknown;
  read-only queries perform no inference or identity allocation. Context
  propagation/callable summaries and separate-compilation metadata remain open.

- [ ] P19-13 — Allocation-context callable summary, context origin, basic
  Arena demand summary, demand semantics, interprocedural demand boundary,
  strict profile validation, statically impossible allocation (§§ 77–83).
  Partial 2026-11-23 (Claude): basic Arena demand summary and demand
  semantics (§§ 79–80) for domains a callable creates. Open: the
  interprocedural boundary for Arenas passed as parameters (§ 81), strict
  profile validation (§ 82, blocked by the open MD-152). Statically impossible
  allocation (§ 83) done 2026-11-24 for single allocations.

## C. Add the Semantic IR representation

- [x] P19-14 — Semantic IR Arena type and allocation-context type (§§ 84–85).

  Completed 2026-11-16: `internal/ir/semantic/arena_types.go` interns one
  canonical Arena owner type independent of backing/profile/target width,
  and a distinct internal allocation-context type. The source type mapper
  admits Arena from package 19 and rejects it by name in older packages;
  references retain the same Arena identity and authority. Ownership checks
  retain MoveOnly and nontrivial destruction. The verifier rejects physical
  or nominal specialization of either semantic type and context storage in
  arrays, Results, named wrappers, references/Places, aggregate payloads,
  local/static storage or ordinary results. No source AllocationContext type
  is registered; foreign context parameters are rejected. Tests cover source
  parameter modes and both scalar plans plus planless analysis, canonical
  interning, malformed type/storage forms and the unsupported emitter gate.
  This is the type milestone only: operation producers, implicit Arena
  destruction, context propagation, schema 15 and physical lowering remain
  owned by their unchecked points.
- [x] P19-15 — Arena operations with Place-based mutation; creation returns
  an owned value: `ArenaCreateBorrowedOp`, `ArenaCreateOwnedFixedOp`,
  `ArenaCreateGrowableOp`, `ArenaNewOp`, `ArenaAllocOp`, `ArenaResetOp`,
  `ArenaReleaseOp`, `ArenaDestroyOp` (§§ 86–96).

  Completed 2026-11-16: the eight operations have explicit Semantic IR
  contracts in `arena_operations.go`, with canonical storage transitions,
  separate state versions/validity epochs, borrowed/owned/growth facts and
  resolved provider/capacity/layout/default/dependency plan references.
  Creation produces an owned Arena; New/Alloc/Reset require writable Places;
  Release/Destroy consume an owner and terminate its domain. Destroy retains
  its P17 Arena resource destruction plan and cause. Borrowed creation retains
  the transferred byte-slice borrow; owned creation/allocation use the checked
  candidate/failed/AllocationError shape. Verification rejects failed-candidate
  publication, reused creation domains, malformed transitions and use after
  transfer across blocks. Deterministic printing preserves these semantic
  facts. Tests exercise all operations with no/32/64-bit scalar plans and
  integrate a New candidate with the P15 dynamic-validity CFG. This closes
  the operation representation only: source plan completion/production,
  context propagation, schema 15 emission and native lowering remain separate
  open points (P19-12/17/18/20/22–29). No allocation-readiness gate is opened.

- [x] P19-16 — Nested Arena IR (§ 97).

  Completed 2026-11-16: `ArenaParentDependency` on borrowed creation retains
  canonical parent storage identity, existing P15 reference/domain/epoch facts
  and P16 static or symbolic borrow bounds. Verification requires this record
  for an Arena-derived backing slice, checks it against the transferred borrow
  and any represented source bounds, and rejects parent/child domain identity
  conflation or individual reclamation of parent bytes. A CFG fixed-point
  analysis blocks parent Reset/Release/Destroy while a child may remain live;
  child terminal release removes that dependency, while child Reset retains
  it, including zero-length backing. Deterministic printing includes parent
  storage/reference/range facts. Tests cover scalar plans, malformed metadata,
  terminal causes, branch joins, loops and retained bounds. This is nested
  Semantic IR support: source plan production/builders, context propagation,
  schema 15 and physical lowering remain separate open milestones.

- [x] P19-17 — Explicit allocation-context propagation and selection in
  Semantic IR (§§ 98–99).

  Completed 2026-11-16: functions retain RequiresAllocationContext separately
  from other effects and receive one marked compiler-hidden context input,
  deterministically first. Internal calls explicitly forward that SSA
  capability with its own argument action; ordinary calls receive none.
  AllocationContextFacts preserves the selected propagated, explicit-Arena,
  target-provided or compiler-local origin and resolved origin-plan reference.
  Context-producing operations retain resolved authority/provider/proof
  selection; owned Arena creation requires an explicit selected capability.
  Verification checks requirement/signature agreement, SSA dominance, origin
  consistency, writable explicit Arena authority and ordinary foreign ABI/data
  escape boundaries. Printing exposes hidden arguments and context selection.
  Tests cover all origins, invalid signatures/forwarding/origins/ABI/escapes,
  missing owned-creation context and rejection at the pre-schema-15 emitter.
  This closes the Semantic IR representation and verification milestone;
  source summary/origin-proof production and builder integration remain
  P19-13/20, with schema/physical work still open. Source readiness is unchanged.

- [ ] P19-18 — P15 integration: Arena references built from source carry
  dynamic-epoch facts and reach the existing validity CFG; P19 retires the
  white-box-only status of `TestPackage15DynamicReferenceUseValidatesOnce`
  (§ 102).
- [ ] P19-19 — P16 slice, P17 ownership and P18 dynamic-array/owning
  container integration (§§ 100–101, 103–104; blocked on P16–P18).

## D. Connect source lowering through resolved plans only

- [~] P19-20 — Builder lowers Arena creation, `New`, `Alloc`, Reset,
  Release, implicit destruction and context selection solely from the P19
  plans; anything unplanned is a package-19 unsupported error with no
  partial module; the allocation readiness gate is narrowed per operation as
  it lowers.

  Update 2026-11-18 (Claude): dependencies found. `New`/`Alloc` need the
  CompilationPlan resolved layout of `T` (P19 § 20 forbids inventing it), and
  no layout service exists before P24-10..16, not even for scalars
  (`ResolvedScalarPlan` has no size/alignment facts). `WithCapacity`/
  `Growable` need a provider/reclamation contract, the MD-034 configuration
  gap. Lowerable without them: `FromBuffer` creation, Reset (once Sema
  publishes the state-version and epoch-exhaustion plans) and Release/implicit
  destruction of borrowed Arenas.
  Update 2026-11-23 (Claude): FromBuffer creation, Reset and implicit
  destruction of borrowed Arenas lower end to end to schema 15
  (TestPackage19ArenaSourceLowering). Open: owned creation (MD-034),
  New/Alloc (P24 layout), explicit Release (P17-29/33), nested children,
  parameter Arenas, per-operation readiness narrowing.
  Correction 2026-11-23 (Claude): the 2026-11-18 note is outdated. MD-141/
  MD-142 have been implemented in `internal/layout` since 2026-11-20 (scalar
  sizes/alignments per verified triple, field placement, arrays, decimal
  carriers), so New/Alloc on a borrowed Arena is not blocked by any decision;
  it only needs to consume that layout service. Owned creation
  (WithCapacity/Growable) needs the sec.system-heap.v1 implementation (MD-151 decided 2026-11-24), not
  MD-034's former configuration gap, which MD-139 closed.
  Done 2026-11-23 (Claude): New/Alloc on a FromBuffer Arena created in the
  same function lower to checked `sec.arena.new` and `sec.arena.alloc` with
  the resolved layout, default and capacity plans, and builder-numbered state
  versions (sec_mlir_dialect_v15.md § 9; TestPackage19ArenaAllocationLowering).
  Open: owned creation (needs the sec.system-heap.v1 implementation (MD-151 decided 2026-11-24)), Arena parameters, nested children.
  Explicit Release of a FromBuffer or default Arena done 2026-11-24 (Claude;
  TestPackage19ArenaReleaseLowering).
  MD-136 decided 2026-11-23 (arena.md § 131, P19 § 166): `Arena {}` and
  an omitted Arena initialization are the infallible, allocation-free default
  creation (fresh domain, zero capacity, no backing, frozen growth policy).
  Its Sema plan, Semantic IR/schema-15 representation, readiness diagnostics
  and tests T01-T11 are governance remaining items of P19.
- [ ] P19-21 — P17 cleanup and defer interaction; task/thread boundary
  scope; FFI retention boundary (§§ 139–142).

## E. Schema 15 and the verifiers

- [x] P19-22 — Schema 15 version gates (after 11–14), registration,
  parser/printer; why `!sec.arena` has no backing-kind parameter (§§ 105–106).
  Done 2026-11-23 (Claude): schema 15 gate and types (sec_mlir_dialect_v15.md).
- [x] P19-23 — `!sec.alloc_context`, the hidden allocation-context function
  argument and the foreign ABI restriction (§§ 107–109).
  Done 2026-11-23 (Claude): hidden context marking/position/forwarding.
- [x] P19-24 — Operations `sec.arena.create_borrowed`, `.create_owned_fixed`,
  `.create_growable`, `.new`, `.alloc`, `.reset`, `.release`, `.destroy`,
  `sec.alloc_context.from_arena`, `.target`, `.compiler_local` (§§ 110–120).
  Done 2026-11-23 (Claude): all eleven operations with local verifiers.
- [x] P19-25 — Arena state-version and ArenaDomain metadata; Arena MLIR
  effects (§§ 121–123).
  Done 2026-11-23 (Claude): state/epoch/domain properties and the <SecArena>
  MemoryEffect resource (schema15-arena-effects.mlir).
- [~] P19-26 — Verifiers: Arena, allocation-context, dependency extension,
  Arena effect, demand-summary representation, strict profile (§§ 124–131).
  Update 2026-11-23 (Claude): Arena and allocation-context verifiers, Arena
  effects, sec.may_allocate / sec.requires_allocation_context. Byte demand
  summary done 2026-11-23 (Claude): `sec.arena_demand_summary` from Sema's
  ArenaDemandSummariesOf, verified by --sec-verify-arenas
  (sec_mlir_dialect_v15.md § 10). Open: strict-profile verifier (blocked by
  the open MD-152, strict bounded-memory policy selection).
- [ ] P19-27 — Profiles: hosted, embedded fixed, bare-metal bounded (with
  MD-034), no-allocation (§§ 132–135). Hosted needs the sec.system-heap.v1 implementation (MD-151 decided 2026-11-24);
  embedded fixed and bare-metal use caller-supplied backing (MD-151 declares no
  provider for them); their strict bounded requirements wait for MD-152;
  no-allocation and freestanding-disabled are not blocked.
- [ ] P19-28 — Effect-analysis, call-graph and separate-compilation metadata
  integration (§§ 136–138).
- [x] P19-29 — Go emitter from verified Semantic IR to schema 15.
  Done 2026-11-23 (Claude): Go emitter to schema 15 (TestSchema15*).

## F. Lowering without physical choices

- [x] P19-30 — No individual Arena allocation free, no uninitialized safe
  allocation, no universal physical Arena representation or ABI, no
  mandatory runtime (§§ 143–148). Done 2026-11-26 (Claude):
  TestPackage19NoArenaSurface (Free, AllocUninit, Capacity, Used, Trim,
  Arena and ref mut Arena over the C ABI, AllocationContext as a source
  type all rejected) and TestPackage19NoPhysicalLowering (no physical
  representation, allocator, runtime or LLVM form in schema 15; the LLVM
  track rejects sec.arena.* as unsupported-lowering). § 145 is
  TestArenaTypeRestrictionMatrix.

## G. Tests (package-required matrices)

- [x] P19-31 — Source/frontend tests (§ 149).

  Completed 2026-11-16: dedicated valid/invalid source fixtures cover the full
  frontend matrix using defined constructors, including owner move/return,
  rejected copying and local-reference escape, zero count/backing, Reset after
  final use versus retained reference/slice/nested child, and consuming Release
  with use-after-release/double-release rejection. Tests run at three analysis
  depths and no/32/64-bit scalar plans, require correctly located diagnostics
  and no operation plans for rejected invalidations. Valid cases must cover
  all five operation kinds and three constructors. Full Sema/Semantic IR/emitter
  suites and Sema vet pass. This is frontend conformance, not source IR/native
  Arena acceptance; MD-136 empty/default construction remains separate.
- [x] P19-32 — Type-restriction tests (§ 150).
  Done 2026-11-24 (Claude): TestArenaTypeRestrictionMatrix plus the existing
  TestArenaTypedAllocationRequirementsUseDistinctDiagnostics; § 83 statically
  impossible allocation is now a compile-time error. The alignment-invalid
  provider row needs the system-heap qualified alignment, the sec.system-heap.v1 implementation (MD-151 decided 2026-11-24)
  (governance remaining item).
- [x] P19-33 — Allocation atomicity tests (§ 151).
  Done 2026-11-24 (Claude): TestPackage19AllocationAtomicityMatrix (fixed
  success, fixed capacity failure with a local handler that continues,
  failure edge carries only AllocationError and mutates nothing, epoch and
  state step only on success, old slice valid after the failure, candidate
  across the failure edge rejected). A locally handled try of New/Alloc now
  lowers; the Semantic IR verifier and --sec-verify-arenas enforce success-
  edge-only publication (lit invalid-arena-publication.mlir). Growable rows
  need the sec.system-heap.v1 implementation (MD-151 decided 2026-11-24)
  (governance remaining item).
- [x] P19-34 — Reset tests (§ 152).
  Done 2026-11-24 (Claude): TestPackage19ResetMatrix (fixed/borrowed and
  growable Reset, epoch/state advance, retained capacity and loan, cursor
  reset as demand window peak, no zeroing write, no runtime epoch value);
  diagnostic rows by TestArenaInvalidationDiagnostics,
  TestArenaResetInvalidatesAllocatedSlice, TestNestedArenaDependencies and
  TestDeferredArenaInvalidationPlans. Physical epoch-metadata elimination
  (arena.md § 105) is a governance remaining item for physical lowering.
- [x] P19-35 — Release/destruction tests (§ 153).
  Done 2026-11-24 (Claude): TestPackage19ReleaseMatrix (borrowed local and
  static backing, implicit destruction, explicit Release cancels cleanup,
  moved source not released, returned Arena transfers cleanup). Owned and
  target-provided Release rows need the sec.system-heap.v1 implementation (MD-151 decided 2026-11-24)
  (governance remaining item).
- [x] P19-36 — Allocation-context tests (§ 154).
  Done 2026-11-24 (Claude): TestPackage19AllocationContextMatrix (explicit
  Arena, A->B->C, none for pure, lexical Arena not guessed, target-provided
  context, missing context under noalloc/unresolved profile). Spawn boundary
  in TestAllocationContextRequirements, foreign-ABI rejection in the IR
  verifier matrix. Explicit selection (§ 59, shaped init), compiler-local
  context (§ 55), source exports (P25) and unresolved-profile classification
  (MD-151 migration, target_profiles.md § 41A(10)) are governance remaining items.
- [x] P19-37 — Demand-summary tests (§ 155).
  Done 2026-11-24 (Claude): TestArenaDemandSummaries plus
  TestArenaDemandInterproceduralBoundary; implemented the § 81 caller
  boundary (closed non-allocating callee 0, allocating recursive callee
  unbounded, other allocating or open callee unknown; new kind "unbounded"
  through Sema, IR, MLIR verifier and v15 § 10). Strict-profile rows wait for
  MD-152 (governance remaining item).
- [x] P19-38 — Semantic IR tests (§ 156).
  Done 2026-11-24 (Claude): existing IR operation/context/nested tests map
  every row; added the per-block Arena effect-site chain check to the
  Semantic IR verifier (mirrors --sec-verify-arenas, v15 § 9) with
  TestArenaEffectSiteOrdering.
- [x] P19-39 — Sec MLIR dialect tests (§ 157). Done 2026-11-23 (Claude).
- [~] P19-40 — Arena verifier and allocation-context verifier negative
  tests (§§ 158–159).
- [~] P19-41 — Integration tests (§ 160).
  2026-11-24 (Claude): TestPackage19IntegrationMatrix covers the P15, P16 and
  P18 context/storage rows; P17 move/destruction/Release rows are
  TestPackage19ReleaseMatrix. Open: defer ordering with Arena operations or
  Arena-backed holders (P19-21) and a T[] backed by an explicit Arena
  (P19-10 §§ 55, 59; MD-135 for non-trivial elements).

## H. Final acceptance, governance and handoff

- [x] P19-42 — Record the explicitly deferred items (§ 161); architecture
  rules check (§ 162) and acceptance criteria matrix (§ 163). Done
  2026-11-26 (Claude): see "P19-42 deferred, architecture and acceptance
  evidence" below: 14 deferred items confirmed absent; 31 rules hold or hold
  in the model, 1 partial; 46 criteria met, 6 partial, 1 open (MD-152), 1 not
  run; Package 19 stays partial.
- [x] P19-43 — Governance: reuse the existing Arena, allocation and lowering
  entries; record implemented/partial/remaining and known holes; keep MD
  links (MD-034, MD-053, MD-065, MD-074). Done 2026-11-26 (Claude): the
  lowering.sec-mlir-package19 summary states the partial state and every
  known hole with its owner; MD links kept; P19 test files listed.
- [x] P19-44 — Implementation report (§ 164) and the Package 20 boundary
  (§ 165). Done 2026-11-26 (Claude):
  rules/mlir/packages/sec-mlir-dialect_package19-implementation-report.md,
  all 51 topics in order; P19 stays partial (46/54 met); Package 20 is
  already in progress with its own report.

## P19-42 deferred, architecture and acceptance evidence — 2026-11-26

Snapshot: HEAD `1a693eec` plus the uncommitted working tree. Evidence names a
test, verifier, lit file or rulebook; `Partial` and `Open` name the owner of
what remains. Nothing here declares Package 19 complete.

### § 161 explicitly deferred (confirmed not implemented)

| Deferred item | State in the tree |
|---|---|
| physical Arena descriptor | none; `!sec.arena` is opaque, no LLVM conversion accepts Arena ops |
| physical provider ABI | none; owned creation is a named boundary until `sec.system-heap.v1` (MD-151) |
| malloc/free selection | none in P19; MD-151 decides the catalog, implementation is platform.target-profiles |
| general uninitialized Arena allocation | none; only canonical-default New/Alloc (TestArenaTypeRestrictionMatrix) |
| individual Arena allocation Free | none; no source member, no reclaim op (P19-30 open as a test point) |
| full task/thread spawn/await/join lowering | none; spawn IR is a named boundary (concurrency packages) |
| concurrent Arena | none |
| stable/weak handles | none |
| full FFI retention lowering | none; foreign ABI never receives hidden context (IR verifier) |
| full physical panic cleanup/unwinder | none; MD-140 stage 4b keeps the Reset panic edge physical |
| public capacity properties | none |
| public arbitrary Arena trimming | none |
| general collection types | none beyond P18 `T[]` |
| LLVM Arena lowering | none; schema 15 stops at Sec MLIR |

### § 162 architecture rules

| Rule | Status | Evidence |
|---|---|---|
| Arena is one move-only builtin source type | Holds | TestArenaFrontendConformance, TestOwningDynamicArrayAndArenaTraits |
| ArenaDomain identity independent of backing address | Holds | TestArenaDomainIdentitySurvivesOwnerMove, TestArenaCreationDomainFreshness |
| State version is not the validity epoch | Holds | TestArenaEffectSiteOrdering; v15 § 9 |
| Ordinary allocation changes state, not epoch | Holds | IR verifier (New/Alloc preserve epoch); TestPackage19AllocationAtomicityMatrix |
| Reset preserves ArenaDomain and advances epoch | Holds | TestPackage19ResetMatrix |
| Release consumes Arena and ends ArenaDomain | Holds | TestPackage19ReleaseMatrix, TestPackage19ArenaReleaseLowering |
| FromBuffer is infallible borrowed fixed construction | Holds | TestPackage19ArenaSourceLowering |
| WithCapacity is fallible owned fixed construction | Holds (Sema/IR) | TestArenaOperationPlans, TestArenaOperationContracts; lowering waits for MD-151 |
| Growable adds stable backing, never relocates | Holds (model) | growth kind growable-nonrelocating in plans/verifiers; no lowering yet (MD-151) |
| New/Alloc publish only fully initialized T | Holds | default_plan canonical-default; TestArenaElementDefaultPlans |
| New/Alloc require trivially destructible T | Holds | TestArenaTypeRestrictionMatrix |
| Allocation failure uses AllocationError and is atomic | Holds | TestPackage19AllocationAtomicityMatrix; lit invalid-arena-publication.mlir |
| Zero-element Alloc succeeds without change | Holds | zero-storage capacity plan; TestArenaDefaultConstruction |
| Repeated allocation keeps prior allocations | Holds | TestPackage19AllocationAtomicityMatrix (old slice after handled failure) |
| Arena allocations not individually freed | Holds | no Free surface; reclaim requires OwningValue |
| Arena object vs backing ownership distinct | Holds | backing relation/authority facts; TestBorrowedArenaRetainsBackingBorrowUntilRelease |
| Nested Arena has own domain, depends on parent | Holds | TestNestedArenaDependencies, TestNestedArenaIR, TestNestedArenaBlocksParentInvalidation |
| Allocation context compiler-visible, not global | Holds | TestArenaContextIsNotSourceVisible |
| MayAllocate and RequiresAllocationContext distinct | Holds | TestAllocationContextRequirements |
| Explicit Arena selection beats ambient context | Partial | explicit selection from source is the P19-10 remainder (§§ 55, 59) |
| Lexical Arena never guessed as ambient | Holds | TestPackage19AllocationContextMatrix |
| Synchronous internal calls propagate only when required | Holds | TestPackage19AllocationContextPropagation, TestSchema15HiddenContextForwarding |
| Foreign ABI never receives hidden context | Holds | IR verifier matrix (TestAllocationContextContracts) |
| No-allocation profiles never fall back to heap | Holds | TestPackage19AllocationContextMatrix (noalloc, freestanding) |
| Arena effects ordered and domain-aware | Holds | TestCallGraphRecordsOrderedDirectArenaEffects, TestArenaEffectSitesCarryStableDomains |
| CSE/reordering may not erase Arena semantics | Holds | MemoryEffects on Arena ops (schema15-arena-effects.mlir) |
| P17 cleanup owns implicit Arena destruction | Holds | TestPackage19ReleaseMatrix |
| P18 storage transitions reused, no second epoch model | Holds | TestPackage19IntegrationMatrix |
| No mandatory runtime | Holds | no runtime dependency in schema 15 |
| No physical representation or allocator ABI | Holds | § 161 table |
| No LLVM dialect generated by P19 | Holds | no conversion accepts Arena ops |

### § 163 acceptance criteria

| Criterion | Status | Evidence / owner |
|---|---|---|
| baseline documented | Met | P19-01; this snapshot |
| previous package regressions green | Met | sema subsets, ir/semantic, lowering/... full; lit 269/269 (2026-11-26) |
| Arena/allocation synchronization applied | Met | P19-02 |
| Semantic IR Arena amendment applied | Met | semantic_ir.md §§ 50–51; rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md |
| schema-v15 dialect rulebook installed | Met | rules/mlir/dialect-versions/sec_mlir_dialect_v15.md |
| lowering-v15 rulebook installed | Met | rules/mlir/lowering-versions/sec_mlir_lowering_v15.md |
| Arena builtin normalized | Met | P19-03 |
| lowercase arena keyword removed | Met | P19-03 (ordinary lowercase identifiers) |
| MoveOnly classification integrated with P17 | Met | P19-03, TestOwningDynamicArrayAndArenaTraits |
| ArenaDomainID implemented | Met | P19-03, P19-14 |
| ArenaStateVersionID implemented | Met | builder-numbered state versions (v15 § 9) |
| backing/growth facts implemented | Partial | P19-04 open: owned/provider rows wait for MD-151 implementation |
| FromBuffer implemented | Met | TestPackage19ArenaSourceLowering |
| WithCapacity implemented | Partial | Sema/IR/MLIR op; source lowering waits for `sec.system-heap.v1` (P19-20 remainder) |
| growable semantic model implemented | Partial | plans and verifiers; growable allocation lowering waits for MD-151 (P19-33 remainder) |
| safe New[T] implemented | Met | TestPackage19ArenaAllocationLowering |
| safe Alloc[T] implemented | Met | TestPackage19ArenaAllocationLowering |
| safe type restrictions enforced | Met | P19-32 |
| AllocationError mapping implemented | Met | P19-06 |
| zero-element allocation implemented | Met | zero-storage plan |
| atomic failure semantics implemented | Met | P19-33 |
| ordinary allocation preserves epoch | Met | IR and MLIR verifiers |
| Reset implements AdvanceEpoch | Met | P19-34; MD-140 stages 2–4a (checked advance) |
| Reset dependency validation integrated | Partial | P19-07 open: defer/Arena interaction (P19-21) and MD-140 stage 2 proof remainder |
| Release implements EndDomain | Met | P19-35 |
| release-by-backing-kind implemented | Partial | borrowed and default rows; owned/target-provided need MD-151 implementation |
| implicit destruction integrated with P17 | Met | TestPackage19ReleaseMatrix |
| nested Arena dependency implemented | Met | P19-09, P19-16 |
| allocation-context requirement summary | Met | P19-10 implemented part, TestAllocationContextRequirements |
| MayAllocate kept distinct | Met | P19-10 |
| context selection order implemented | Partial | explicit selection and compiler-local context are the P19-10 remainder |
| hidden !sec.alloc_context propagation | Met | P19-23, TestSchema15HiddenContextForwarding |
| lexical Arena never auto-selected | Met | TestPackage19AllocationContextMatrix |
| foreign ABI hidden-context guard | Met | IR verifier |
| noalloc profile behavior implemented | Met | P19-36 |
| ordered Arena effects represented | Met | P19-11 |
| basic Arena demand summary implemented | Met | P19-26 demand part, P19-37 |
| strict profile capacity checks implemented | Open | MD-152 (strict policy selection) |
| ArenaResource/effect interface implemented | Met | schema15-arena-effects.mlir |
| !sec.arena implemented | Met | P19-22..25 |
| !sec.alloc_context implemented | Met | P19-23 |
| schema-v15 Arena/context ops implemented | Met | P19-24; lit schema15-arena*.mlir |
| --sec-verify-arenas registered | Met | Passes.td SecVerifyArenas; lit invalid-arena-*.mlir |
| --sec-verify-allocation-contexts registered | Met | Passes.td SecVerifyAllocationContexts |
| P15/P16 dependency verifiers extended | Met | P19-26 verifier part; TestPackage19IntegrationMatrix |
| P18 allocation-context integration | Met | TestPackage19IntegrationMatrix (T[] growth through the context) |
| no public uninitialized allocation | Met | § 161 |
| no arbitrary non-trivial Arena.Alloc | Met | TestArenaTypeRestrictionMatrix |
| no physical Arena descriptor | Met | § 161 |
| no allocator ABI | Met | § 161 |
| no mandatory runtime | Met | § 162 |
| check-sec-mlir passes | Met | lit 269/269 |
| go test ./... passes | Not run | full suite needs owner authorization; package suites green |
| legacy paths remain operational | Met | schema 11–14 emission and lit regressions |

Result: 46 Met, 6 Partial, 1 Open, 1 Not run. Package 19 stays partial.

## Recommended execution order

1. A (baseline, sync audit) — independent of predecessors.
2. B Sema plans (P19-04..13) — the existing Arena/allocation frontend makes
   this the most implementable part today; publish plans over current facts.
3. C/D for Arena values, `New`, Reset/Release and P15 dynamic-epoch
   references (P19-14..18, 20) — needs P15 (available) and P17 for
   destruction; `Alloc[T]` slices need P16.
4. E/F schema 15 — after schemas 11–14 exist.
5. G/H throughout, with blocked items marked as such.
