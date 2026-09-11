# SEC-MLIR Package 15 — implementation TODO

Package: `SEC-MLIR-P15` — Safe Place and Direct Reference Semantic Core
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package15.md`
Package contract: `rules/mlir/packages/sec-mlir-dialect_package15.yaml`
Normative inputs: `rules/mlir/normative-sync/sec_reference_sync_v1.md`,
`rules/mlir/semantic-ir/sec_semantic_ir_place_reference_v1.md`,
`rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`, and
`rules/mlir/lowering-versions/sec_mlir_lowering_v11.md`

## Inventory snapshot — 2026-09-09

Planning was prepared against repository HEAD:

```text
fa1e9ed5b6dfe527711e23a3787e8a32f2e6cc1c
```

The primary worktree has unrelated tracked and untracked changes. It is not a
clean Package 15 baseline. P15-01 must create and record an isolated worktree
or commit-based snapshot before any completion command is presented as P15
evidence.

### Isolated execution baseline — 2026-09-10

P15 work has a clean detached baseline at:

```text
/home/jonas/small-projects/sec-p15-baseline-2c67853ee313
```

Recorded environment:

```text
HEAD:             2c67853ee31358a9a8a486aa11fba57c6b6a7270
tree:             225922f80c0537e8c2b21e3388d6a068cbb032ee
tree state:       clean, detached HEAD
primary worktree: dirty and excluded from final P15 acceptance evidence
Go:               go1.26.0 linux/amd64
Git:              2.53.0
sec-mlir-opt:     LLVM 24.0.0git, optimized build with assertions
llvm-lit:         lit 24.0.0dev
SEC_MLIR_BIN:     /home/jonas/small-projects/sec/build/sec-mlir/bin
llvm-lit path:    /home/jonas/mlir/llvm-project/build/bin/llvm-lit
```

Reproduction commands:

```sh
git worktree add --detach /home/jonas/small-projects/sec-p15-baseline-2c67853ee313 2c67853ee31358a9a8a486aa11fba57c6b6a7270
git -C /home/jonas/small-projects/sec-p15-baseline-2c67853ee313 status --short --branch
go version
/home/jonas/small-projects/sec/build/sec-mlir/bin/sec-mlir-opt --version
/home/jonas/mlir/llvm-project/build/bin/llvm-lit --version
```

Verified by source inventory, not by a package acceptance run:

- P13 and P14 are recorded as implemented in the legacy
  `implementation-status.yaml`; P14 owns the current schema-10 fixed-array
  representation and rejects element references at its Semantic IR boundary.
- The frontend has an existing `internal/sema/place.go` and source-level
  reference/borrow/origin analysis. It still needs the P15 canonical identity,
  arbitrary-precision, plan-query, and IR contracts; it must be evolved rather
  than shadowed with a second Place model.
- The P14 Semantic IR builder explicitly rejects fixed-array element borrows
  and array-to-slice construction. Element borrowing is P15; slice construction
  remains P16.
- The supplied P15 schema-v11, lowering-v11, reference synchronization, and
  Semantic IR amendment documents exist, but the canonical consolidated
  rulebooks do not yet contain the P15 place/reference operations.
- No implementation of `PlaceRootID`, `StorageIdentityID`,
  `InvalidationDomainID`, `EpochDependencyID`, `ResolvedPlaceOf`,
  `ResolvedReferencePlanOf`, or the P15 Semantic IR operations was found.

This package creates a target-independent, high-level representation for
addressable Places and direct safe `ref T` / `ref mut T`. It does **not** choose
a pointer layout, an epoch runtime layout, a reference ABI, or LLVM lowering.

## Completion boundary

P15 owns:

- one canonical Place model with stable root/storage identities, structured
  projections, arbitrary-precision constant indexes, and relationship queries;
- compiler-resolved, read-only Place/reference/use plans and complete direct
  reference facts;
- shared and mutable direct borrows, shared copy, mutable move, reborrowing,
  subplaces, bounded origin joins, and borrow-end metadata;
- proven versus dynamic-epoch validity, semantic invalidation dependencies, and
  the ordinary stale-reference panic path;
- copy-trivial reference reads and trivially replaceable mutable-reference
  writes; reference equality as semantic live-location equality;
- high-level Semantic IR and schema-11 Sec MLIR place/reference operations,
  plus the place, reference-guard, and borrow-structure verifiers;
- P5, P6, P8, P11/P12, P13, and P14 compatibility, with source → Sema →
  verified Semantic IR → schema-11 evidence on 32- and 64-bit plans.

Do **not** pull these into P15:

- slice values, mutable slice representation, subslices, or fixed-array-to-
  slice borrowing (P16);
- owning dynamic arrays and their backing-storage transitions (P18);
- stable/weak handles, slot tables, or fallible handle resolution;
- RawPtr-to-ref or ref-to-RawPtr conversion, FFI reference ABI, physical
  pinning, physical epoch storage, allocation/arena/collection epoch producers,
  or concurrent invalidation protocols;
- move-only/semantic-copy whole-value reads, non-trivial replacement or
  destruction (P17);
- physical offsets, array stride/GEP calculation, MemRef/LLVM pointer
  representation, capability/side-table selection, or a mandatory runtime.

Every language-semantic function added for P15 must contain a short
responsibility comment and the governing rulebook section, as required by
`codex.md`. New governance belongs in `governance/lowering.yaml`; do not add a
new entry to the legacy `implementation-status.yaml`.

## A. Establish a reproducible, rule-synchronized baseline

- [x] P15-01 — Create an isolated P15 baseline from the recorded HEAD (or a
  newer clean merged equivalent). Record HEAD, tree state, Go version,
  `sec-mlir-opt`/`llvm-lit` versions, and the absolute `SEC_MLIR_BIN` path.
  Never use the dirty primary worktree as final acceptance evidence.
- [ ] P15-02 — Run and record predecessor gates before changing Places:
  `go test ./...`, `go vet ./...`, P13/P14 focused source-to-schema tests, and
  `cmake --build build/sec-mlir --target check-sec-mlir -j2`.
- [ ] P15-03 — Read the P15 normative authority chain and applicable
  corrections: reference model, references, borrowing, lifetime, storage,
  ownership, copy/move, runtime checks, panic, RawPtr, collections, structs,
  unions, Semantic IR, and Sec MLIR rules. Add exact section links to the
  implementation plan and new semantic functions.
- [ ] P15-04 — Apply the direct-reference synchronization to canonical
  `rules/memory/references.md`, `rules/memory/reference_model.md`,
  `rules/errors/runtime_checks.md`, and `rules/errors/panic.md`: validity can
  be proven or dynamically validated; ordinary stale direct references use
  `panic.invalid-reference-generation`, not `Result`/`try`; no runtime is
  implied merely by an epoch dependency.
- [ ] P15-05 — Apply the Place/direct-reference amendment to
  `rules/compiler/semantic_ir.md` and install schema-v11 and lowering-v11
  material into the canonical Sec MLIR rulebooks. Preserve version history and
  keep schema 9/10 compatibility promises explicit.
- [ ] P15-06 — Check `rules/corrections/` for every affected rulebook, move any
  applied correction through the prescribed workflow, and update
  `language-rulebook-status.md` for every changed/superseded rulebook.
- [x] P15-07 — Add one `lowering.sec-mlir-package15` integration to
  `governance/lowering.yaml` with status `partial`, baseline, rules, known
  predecessor state, deferred boundaries, code/test locations, tool versions,
  and the exact reproducible commands. Validate unique governance ownership.
- [x] P15-08 — Extend the shared absolute-path `SEC_MLIR_BIN` test helper for
  P15 schema-11 source tests. It must reject relative tool directories and be
  usable from every Go package working directory.

Completed 2026-09-10: `internal/testsupport` now owns the shared
`SEC_MLIR_BIN` resolver. It rejects configured relative directories, exposes
optional and required-tool forms to every repository-internal Go package, and
returns one absolute `sec-mlir-opt` path independent of the package test working
directory. Existing Package 10/11 compiler tests and Package 13/14 lowering
tests consume the shared resolver. Focused tests cover relative rejection and a
nested working-directory change.

## B. Make the existing Place model canonical

- [ ] P15-09 — Audit every producer and consumer of `internal/sema.Place`,
  `PlacesOverlap`, reference origins, field/index projections, borrowing,
  escaping, lifetime, destruction, and prospective race analysis. Select the
  existing model as the single migration target; do not create a parallel P15
  place representation.
- [ ] P15-10 — Introduce stable compiler-owned `PlaceRootID` and root kind
  facts for local, parameter, static, addressed, allocation, and foreign roots
  as applicable. Root identity must be independent of source spelling and
  remain stable through shadowing, renaming, generic instantiation, and
  diagnostics presentation.
- [ ] P15-11 — Add canonical `StorageIdentityID`, `InvalidationDomainID`,
  `EpochDependencyID`, `AddressSpaceID`, `BorrowID`, and `LifetimeID` facts.
  Define ownership, allocation, cloning, deterministic printing, and invalid/
  absent state; none may be treated as numeric addresses or source names.
- [x] P15-12 — Replace `int64` constant place indexes with immutable,
  defensive-copy arbitrary-precision values. Keep any legacy `int64` display
  or backend adapter behind one checked representability helper only.

Completed 2026-09-11: canonical frontend `PlaceProjection.ConstantIndex` now
stores exact `big.Int` values. Place resolution, interprocedural parameter
projection, path printing, overlap/identity comparison, static-slice index
composition, and cloning preserve full precision without an `int64` adapter.
All Place-copy boundaries clone the integer storage. A source fixture proves
that distinct mutable element borrows above `MaxInt64` remain disjoint, while
focused unit assertions cover exact presentation and defensive cloning.
- [ ] P15-13 — Add dynamic-index identity based on a resolved expression/value
  identity, not source text. It must distinguish evaluated indexes while
  retaining P14 source type, signedness, exact fixed-array length, and
  bounds-proof facts.
- [ ] P15-14 — Normalize projections into field, constant-index, dynamic-index,
  union-payload, and dereference forms. Retain slice/property metadata only as
  deferred compatibility data; ordinary properties must not become addressable
  field Places.
- [ ] P15-15 — Implement the one canonical `PlaceRelationship` query:
  `same`, `disjoint`, `contains`, `contained-by`, `potentially-overlapping`,
  and `unknown`. Make borrowing, move/escape/lifetime logic, and future
  consumers call it rather than reimplement alias decisions.
- [ ] P15-16 — Retain `PlacesOverlap` only as a compatibility adapter over the
  relationship result. Migrate new correctness decisions first and add tests
  that prevent its legacy boolean behavior from silently classifying unknown
  or potentially-overlapping Places as disjoint.
- [ ] P15-17 — Implement relationship rules for same/nested fields, distinct
  stored struct fields, equal/distinct arbitrary-precision constant array
  indexes, dynamic indexes, union payload variants under active-variant proof,
  dereference projections, different storage identities, and containment.
- [ ] P15-18 — Preserve finite alternative origin sets across control-flow
  joins, deterministically bounded at the documented implementation limit;
  degrade overflowed/unknown origin knowledge conservatively without adding a
  runtime origin tag or inventing a single origin.
- [ ] P15-19 — Add focused Sema tests for stable root identity independent of
  display name, relationship symmetry/containment, wide and above-`int64`
  indexes, field/index disjointness, dynamic overlap, union proof scope,
  dereference origin, alternative joins, unknown degradation, and legacy
  `PlacesOverlap` compatibility.

## C. Publish compiler-resolved reference facts and plans

- [ ] P15-20 — Define `ReferenceKind` (`shared-direct`, `mutable-direct`),
  `ReferenceValidityPolicy` (`proven`, `dynamic-epoch`), relocation class,
  semantic provenance kind, and a complete immutable `ReferenceFacts` model.
  Facts must include referent TypeID, possible origin Places/storage identities,
  address space, BorrowID/LifetimeID, authority, validity, epoch dependency,
  relocation, and provenance.
- [ ] P15-21 — Preserve the source-level classification: `ref T` is
  `CopyTrivial`; `ref mut T` is `MoveOnly`. A shared copy duplicates only the
  non-owning reference holder, while a mutable move transfers the holder and
  borrow obligation, never the referent.
- [ ] P15-22 — Add a read-only `ResolvedPlaceOf(ast.Expression)` (or exact
  equivalent) keyed by the analyzed expression. It must expose existing facts
  only: no Place re-resolution, borrow creation, origin inference, diagnostics,
  AST mutation, or analyzer-state mutation.
- [ ] P15-23 — Add immutable, read-only `ResolvedReferencePlanOf` facts for
  every source reference creation and reborrow. The plan must contain the
  Place, reference facts, copy class, and proof/check policy established by
  successful Sema; defensive copies protect slices and big integers.
- [ ] P15-24 — Add a read-only reference-use plan for read, write, reborrow,
  equality, parameter passing, return, and end-of-lifetime uses. The Semantic
  IR builder must consume this plan and must not rerun a borrow checker or
  infer reference behavior from source syntax.
- [ ] P15-25 — Classify validity independently from physical representation:
  proven references retain closed proof provenance; dynamic references retain
  storage/domain/epoch, relocation, and address-space dependencies. Reject a
  safe reference that is neither proven nor dynamically checkable.
- [ ] P15-26 — Extend immutable `CompilationPlan` reference policy with a
  default 64-bit logical epoch width on both 32- and 64-bit targets. Permit a
  shorter width only from compile-time proof/exhaustion facts; prohibit runtime
  selection and do not encode epoch width in source type identity.
- [ ] P15-27 — Ensure reference/subreference derivation only preserves or
  narrows lifetime, spatial extent, authority, compatible origin/domain/epoch,
  and address space. In particular, never derive `ref mut` from `ref`.
- [ ] P15-28 — Preserve existing reference-return origin summaries through
  calls, fields, elements, and finite joins. Reject local/match-scoped/captured
  escaping origins while permitting approved parameter, external-owner, static,
  and addressed origins.
- [ ] P15-29 — Add plan/query tests proving read-only behavior, defensive
  copies, identity stability, shared-copy/mutable-move classification,
  shared/mutable reborrows, source-origin joins, prohibited authority widening,
  local-return rejection, and no hidden borrow-state transition when a query is
  repeated.
- [ ] P15-30 — Add the epoch/reference-plan matrix: proven local and addressed
  references, arena dependency fact without physical arena metadata, dynamic
  epoch fact, 64-bit default under both plans, shorter-width proof gate, no
  runtime-selected width, relocation/address-space preservation, and rejection
  of unknown uncheckable safe references.

## D. Build the Semantic IR Place and direct-reference core

- [ ] P15-31 — Add canonical Semantic IR Place IDs/root facts, projection
  records, reference types/facts, deterministic printer support, cloning, and
  verifier ownership. Place, value, storage, safe reference, and RawPtr must
  remain distinct categories.
- [ ] P15-32 — Add `PlaceValueRootOp` and `PlaceStorageRootOp` with stable root,
  storage, address-space, and authority facts. Immutable SSA roots are `ro`;
  storage-derived authority may not exceed storage mutability.
- [ ] P15-33 — Add `PlaceFieldOp`, `PlaceArrayIndexInBoundsOp`,
  `PlaceArrayElementOp`, `PlaceUnionPayloadOp`, and `ReferenceDerefPlaceOp`.
  Reuse P13 field identity, P14 exact index/bounds plans, and P11/P12 active
  variant facts; no operation may calculate offsets, stride, GEPs, or copy an
  enclosing aggregate merely to form a subplace.
- [ ] P15-34 — Add `PlaceReadOp` for `copy-trivial` whole-value reads only and
  `PlaceWriteOp` for writable, copy-trivial, trivially destructible replacement
  only. Explicitly reject move-only/semantic-copy reads, non-trivial write
  cleanup, unresolved contracts, and authority mismatch without partial IR.
- [ ] P15-35 — Add `ReferenceBorrowSharedOp` and `ReferenceBorrowMutableOp`.
  Require compatible Place authority plus complete reference metadata; creation
  is non-owning and has no implicit allocation or physical address operation.
- [ ] P15-36 — Add `ReferenceCopySharedOp`, `ReferenceMoveOp`,
  `ReferenceReborrowSharedOp`, and `ReferenceReborrowMutableOp`. Verify new
  BorrowID/LifetimeID derivation, non-increasing authority, compatible
  provenance/epoch/address space, and no `ref` → `ref mut` path.
- [ ] P15-37 — Add `ReferenceIsValidOp`, `ReferenceGenerationFailureOp`, and
  `ReferenceEndBorrowOp`. The dynamic false edge must terminate with
  `panic.invalid-reference-generation`; borrow end has no runtime semantics but
  makes subsequent reachable use invalid.
- [ ] P15-38 — Add `ReferenceCompareOp` for `eq`/`ne` over compatible live
  direct references. It represents storage identity plus location equality,
  never raw address/integer equality, and it must participate in dynamic
  validity guarding.
- [ ] P15-39 — Implement Semantic IR verification for Place type/projection
  identity, authority narrowing, exact big-int index facts, root/storage/domain
  consistency, reference facts, borrow/lifetime transitions, dynamic CFG guards,
  failure endpoints, end-borrow reachability, and no Place escape as a source
  value, call argument, stored field, array element, union payload, or return.
- [ ] P15-40 — Add deterministic Semantic IR tests covering all root/projection
  operations, wide referents, constant/dynamic fixed-array elements, union
  payloads, dereference Places, reads/writes, borrows/copy/move/reborrows,
  validity, compare, end-borrow, printer stability, malformed facts, and every
  P15 unsupported ownership boundary.

## E. Lower resolved source behavior without re-analysis

- [ ] P15-41 — Refactor the Semantic IR builder so all reference and Place
  paths consume P15 resolved plans. Unknown/missing facts are package-tagged
  unsupported errors; failed builds return no partially assembled module or
  placeholder operation.
- [ ] P15-42 — Lower local/static/addressed roots and reference-dependent
  mutable storage through high-level storage plus Place roots. Mark/address
  track address-taken storage so P5 trivial-core lowering cannot erase it before
  the P15 representation boundary is discharged.
- [ ] P15-43 — Lower direct field/subfield borrows via Place projections;
  demonstrate that P13 struct values are neither loaded nor copied solely to
  create a reference. Keep properties out of field-Place lowering.
- [ ] P15-44 — Lower fixed-array element borrows through a root array Place and
  P14's original evaluated index/bounds plan. A dynamic ordinary borrow must
  evaluate root then index once, emit one place-bounds predicate, branch before
  forming the element/reference, and use existing ordinary bounds failure on
  the false edge; a proven borrow emits no redundant branch.
- [ ] P15-45 — Lower union/Result payload reference bindings only from the
  compiler-resolved matching-variant Place. Keep branch-local BorrowID/lifetime
  scope, retain guard dominance, and reject match-result, outer-assignment,
  return, and lambda capture escapes when Sema says the payload origin is
  scoped.
- [ ] P15-46 — Lower dereference/read/write/reborrow/equality through one
  validity policy. Proven uses retain proof metadata and no runtime check;
  dynamic uses validate the same reference SSA, branch once, terminate the
  false edge in reference-generation failure, and allow the guarded operation
  only on the true edge.
- [ ] P15-47 — Lower reference parameters, implicit owned-place borrows,
  existing shared-reference passing, mutable-reference move/reborrow passing,
  allowed returned input/subfield references, and preserved return-origin
  summaries. Do not choose a physical ABI or silently copy `ref mut`.
- [ ] P15-48 — Integrate effects and `@noPanic`: proven use adds no invalid-
  reference panic; a reachable dynamic failure adds the exact panic reason;
  validation itself adds neither allocation nor blocking; borrow creation/end
  remains runtime-effect-free. Reject `@noPanic` where a dynamic failure remains
  reachable.
- [ ] P15-49 — Add source-to-Semantic-IR tests for local and parameter refs,
  field and array subreferences, shared copy, mutable move, all legal/
  prohibited reborrows, dynamic validity CFG, equality, writes, borrow ends,
  effects, and `@noPanic`. Assert evaluation order, exact SSA reuse, no
  aggregate copy, no partial IR, no `undef`/poison, no hidden allocation, and
  no physical reference/layout vocabulary.

## F. Implement Sec MLIR schema 11 and its three verifiers

- [ ] P15-50 — Bump compiler-generated modules to schema 11 only after schema
  9 and schema 10 remain accepted compatibility inputs and schema 12 remains
  rejected. Update ODS/C++ registration, parser/printer, version gates, and
  generated-module tests.
- [ ] P15-51 — Implement `!sec.place<T,"ro">`, `!sec.place<T,"rw">`,
  `!sec.ref<T>`, and `!sec.ref_mut<T>` with exact type parsing, printing, type
  constraints, authority rules, and no physical address/epoch fields.
- [ ] P15-52 — Implement schema-11 root/projection/read/write operations:
  `sec.place.value`, `sec.place.storage`, `sec.place.field`,
  `sec.place.array_index_in_bounds`, `sec.place.array_element`,
  `sec.place.union_payload`, `sec.place.deref`, `sec.place.read`, and
  `sec.place.write`. Require deterministic root/storage/address-space,
  field/variant, P14-compatible bounds, authority, action, and proof attrs.
- [ ] P15-53 — Implement schema-11 reference operations:
  `sec.ref.borrow_shared`, `sec.ref.borrow_mut`, `sec.ref.copy_shared`,
  `sec.ref.move`, `sec.ref.reborrow_shared`, `sec.ref.reborrow_mut`,
  `sec.ref.is_valid`, `sec.ref.compare`, `sec.ref.end_borrow`, and terminating
  `sec.fail.reference_generation` with its canonical panic ID.
- [ ] P15-54 — Register `--sec-verify-places`. Verify roots, wrapper and
  projection types, authority non-escalation, field identity/type, P14 array
  proof or guard, union active variant, deref authority, property exclusion,
  and every prohibited Place escape.
- [ ] P15-55 — Register `--sec-verify-reference-guards`. For dynamic facts,
  require validation of the same reference SSA, true-edge dominance of every
  dereference/read/write/reborrow/compare, and a false path ending in
  `sec.fail.reference_generation`. For proven facts, require closed
  compiler-owned proof provenance and no bogus runtime guard requirement.
- [ ] P15-56 — Register `--sec-verify-borrow-semantics`. Verify immutable
  metadata consistency, shared-copy authority, mutable no-copy/move behavior,
  legal reborrow origins, BorrowID/LifetimeID transitions, no reachable
  post-end use, allowed return origins, and place/reference type agreement.
  It must not duplicate Sema's source-language borrow checker.
- [ ] P15-57 — Extend the Go emitter from verified Semantic IR to schema 11.
  Preserve TypeIDs, root/storage/domain/epoch/borrow/lifetime facts, exact
  big-int index/proof metadata, source locations, CFG ownership, and high-level
  storage. Reject unrepresented physical ABI/layout requests explicitly.
- [ ] P15-58 — Add schema-11 MLIR round-trip and invalid suites for all types
  and operations; schema 9/10 regression; malformed root/authority/field/index/
  variant/reference metadata; place escape; guard errors; borrow errors; end
  use; deterministic printing; and canonical panic failure.

## G. Prove predecessor integration and architecture boundaries

- [ ] P15-59 — P5: prove reference-dependent/address-taken storage remains
  high-level through `--sec-lower-trivial-core`; reject MemRef conversion while
  a Place/reference dependency exists, without changing ordinary P5 storage.
- [ ] P15-60 — P6/P8: prove target scalar resolution recursively converts only
  referents under `!sec.place`, `!sec.ref`, and `!sec.ref_mut` on 32- and
  64-bit plans, while checked-integer signless normalization does not recurse
  through those wrappers or lower P15 operations.
- [ ] P15-61 — P13: test shared and mutable struct-field references, two
  disjoint mutable field borrows, whole-struct versus mutable-field conflict,
  nested subfield reborrow, property rejection, and `int128`/`uint256` field
  referents without an aggregate copy.
- [ ] P15-62 — P14: test shared/mutable constant element references, distinct
  versus equal index conflict, conservative dynamic-index overlap, exactly-once
  dynamic bounds validation, proven no-branch element borrowing, `int128` /
  `uint256` index proof metadata, and zero-length rejection.
- [ ] P15-63 — P11/P12: test union single-payload and Result `Ok`/`Err`
  ref/ref-mut bindings where source rules permit, active-variant guards,
  borrowed guards, branch-end lifetime markers, and all scoped escape
  rejections. Do not weaken union guard verification or introduce a copy path.
- [ ] P15-64 — Prove calls and returns: ref/ref-mut parameters, implicit borrow
  of owned Places, shared direct-call copy, mutable move/reborrow according to
  resolved mode, returned input/subfield references, local return rejection,
  and deterministic instantiated origin summaries.
- [ ] P15-65 — Add end-to-end source → target-aware Sema → verified Semantic IR
  → schema-11 → `sec-mlir-opt` tests on 32- and 64-bit CompilationPlans using
  an absolute tool path. Include P13/P14/P12 integration, proven and dynamic
  validity, failure CFG, wrapper preservation, and all three verifier passes.
- [ ] P15-66 — Assert successful P15 modules contain no physical offset/stride,
  GEP, pointer arithmetic, MemRef/LLVM reference form, raw-pointer collapse,
  address-plus-epoch aggregate, side-table/capability choice, hidden ABI args,
  runtime borrow counter, global generation manager, hidden allocation,
  `undef`, or poison.
- [ ] P15-67 — Add explicit no-partial-module rejection tests for stable/weak
  handle syntax and resolution, slice values and array-to-slice borrow,
  RawPtr conversions, physical pinning/epoch/collection invalidation lowering,
  concurrent invalidation, move-only/semantic-copy reference reads, non-trivial
  write/destruction, physical reference ABI/layout, and LLVM lowering.
- [ ] P15-68 — Add determinism tests rebuilding Place/reference facts and
  source modules with fresh maps and permuted source metadata. Assert stable
  root/borrow/lifetime IDs under the documented deterministic allocator,
  origin ordering, big-int printing, diagnostics, and independent 32/64 output
  without mutating shared Semantic IR.

## H. Final acceptance, governance, and report

- [ ] P15-69 — Create a section-155 acceptance matrix mapping every criterion
  to a focused test and/or exact implementation location. Aggregate test success
  is evidence only for the explicitly repository-wide criteria.
- [ ] P15-70 — From a fresh isolated worktree containing the exact P15 diff,
  run `go test ./...`, `go vet ./...`, `git diff --check`, and unique-key YAML
  validation. Record commands, HEAD/tree, environment, and results.
- [ ] P15-71 — Run the complete `check-sec-mlir` target and explicit adjacent
  schema-9/10/11 selection. Record exact LLVM/MLIR/lit versions and all test
  counts; split failures into new checklist items rather than weakening tests.
- [ ] P15-72 — Run the package-specific 32/64 source-to-schema-11 pipeline
  with an absolute `SEC_MLIR_BIN`, `--sec-verify-places`,
  `--sec-verify-reference-guards`, and `--sec-verify-borrow-semantics`.
- [ ] P15-73 — Update the sole `lowering.sec-mlir-package15` governance entry
  in `governance/lowering.yaml` with exact implemented surface, intentional
  P16/P17/P18/FFI deferrals, baseline/head, code/tests, toolchains, commands,
  results, and `status: implemented` only after every section-155 criterion is
  satisfied. Do not duplicate it in `implementation-status.yaml`.
- [ ] P15-74 — Write the section-156 implementation report in the P15 package
  directory, covering all 46 required topics in order, deviations, exact
  commands/results, and Package 16 recommendations.
- [ ] P15-75 — Re-run the acceptance matrix, governance unique-ownership check,
  rulebook-status check, complete Go/vet suites with absolute MLIR tooling, and
  full Sec MLIR suite before handoff. Confirm every P15 TODO item is either
  complete or explicitly moved to its owning later package.

## Recommended execution order

1. P15-01–08 — isolate baseline, synchronize normative rules, and create
   accurate governance.
2. P15-09–19 — migrate the canonical Place identity/projection/relationship
   model before exposing it to new lowering.
3. P15-20–30 — publish immutable Sema Place/reference/use facts and policy.
4. P15-31–40 — add the target-independent Semantic IR core and verifier.
5. P15-41–49 — connect resolved source behavior, CFG, effects, and storage.
6. P15-50–58 — implement schema 11, emitter, and the three MLIR verifiers.
7. P15-59–68 — prove P5/P6/P8/P11/P12/P13/P14 integration, safety boundaries,
   and determinism.
8. P15-69–75 — complete evidence, final governance, and the mandatory report.
