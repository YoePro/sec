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

Private machine paths have been replaced by environment-variable references.
Set `P15_BASELINE` to the local isolated worktree, `SEC_MLIR_BIN` to the absolute
Sec MLIR tool directory, and `LLVM_LIT` to the local llvm-lit executable before
reproducing the commands. Their expanded locations must not be published.

P15 work has a clean detached baseline at:

```text
${P15_BASELINE}
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
SEC_MLIR_BIN:     ${SEC_MLIR_BIN}
llvm-lit path:    ${LLVM_LIT}
```

Reproduction commands:

```sh
git worktree add --detach ${P15_BASELINE} 2c67853ee31358a9a8a486aa11fba57c6b6a7270
git -C ${P15_BASELINE} status --short --branch
go version
${SEC_MLIR_BIN}/sec-mlir-opt --version
${LLVM_LIT} --version
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
- [x] P15-02 — Run and record predecessor gates before changing Places:
  `go test ./...`, `go vet ./...`, P13/P14 focused source-to-schema tests, and
  `cmake --build build/sec-mlir --target check-sec-mlir -j2`.

  Completed 2026-10-11 against primary HEAD 55cc937 (dirty worktree; recorded as a pre-change gate, not final acceptance evidence): `go test ./...` passes except the unrelated stale generated `sec/stdlib/unicode/unicode.sec` check, `go vet ./...` is clean, `go test ./internal/lowering/... ./internal/ir/...` (P13/P14 lowering and Semantic IR) passes, and `cmake --build build/sec-mlir --target check-sec-mlir -j2` passes 91/91.
- [x] P15-03 — Read the P15 normative authority chain and applicable
  corrections: reference model, references, borrowing, lifetime, storage,
  ownership, copy/move, runtime checks, panic, RawPtr, collections, structs,
  unions, Semantic IR, and Sec MLIR rules. Add exact section links to the
  implementation plan and new semantic functions.

  Completed 2026-10-11. Authority chain used for P15 work: `rules/memory/reference_model.md` §§ 33-34 (epoch width), § 45, §§ 73-76 (stale panic); `rules/memory/references.md` §§ 3-5, 8-14; `rules/memory/borrowing.md` § 15; `rules/memory/lifetime_analysis.md`; `rules/memory/storage.md` § 5; `rules/memory/ownership.md`; `rules/memory/copy_move.md`; `rules/errors/runtime_checks.md` (reference-generation outcomes); `rules/errors/panic.md` § 13; `rules/memory/raw_pointers.md` (out of scope boundary); `rules/collections/collections.md` (fixed arrays); `rules/compiler/semantic_ir.md` §§ 10, 45, 46, 48, 96, 108; `rules/mlir/sec_mlir_dialect.md` schema v11 part; `rules/mlir/sec_mlir_lowering.md` version-11 part. New semantic functions cite these sections.
- [x] P15-04 — Apply the direct-reference synchronization to canonical
  `rules/memory/references.md`, `rules/memory/reference_model.md`,
  `rules/errors/runtime_checks.md`, and `rules/errors/panic.md`: validity can
  be proven or dynamically validated; ordinary stale direct references use
  `panic.invalid-reference-generation`, not `Result`/`try`; no runtime is
  implied merely by an epoch dependency.

  Completed 2026-10-11: `runtime_checks.md` (three reference-generation outcomes, no `try` recovery) and `reference_model.md` § 76(2) (registered `InvalidReferenceGeneration` reason) synchronized; `references.md` § 14(9) and `panic.md` § 13 already matched.
- [x] P15-05 — Apply the Place/direct-reference amendment to
  `rules/compiler/semantic_ir.md` and install schema-v11 and lowering-v11
  material into the canonical Sec MLIR rulebooks. Preserve version history and
  keep schema 9/10 compatibility promises explicit.

  Completed 2026-10-11: the Semantic IR amendment is integrated into `semantic_ir.md` v2 (§§ 10(5)-(11), 45(5)-(13), 46(6)-(9), 48(6)-(7), 96(5), 108(5)); schema v10/v11 and lowering v10/v11 are installed as versioned parts of `sec_mlir_dialect.md` and `sec_mlir_lowering.md`, which keep the 1-10 compatibility promise and state that the compiler emits schema 10 until P15-50.
- [x] P15-06 — Check `rules/corrections/` for every affected rulebook, move any
  applied correction through the prescribed workflow, and update
  `language-rulebook-status.md` for every changed/superseded rulebook.

  Completed 2026-10-11: no pending correction targets these rulebooks; the synchronization is archived as `rules/corrections/applied/p15-place-reference-synchronization-20261011.md`, and `language-rulebook-status.md` rows for reference_model, runtime_checks, semantic_ir, sec_mlir_dialect and sec_mlir_lowering are updated.
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

- [x] P15-09 — Audit every producer and consumer of `internal/sema.Place`,
  `PlacesOverlap`, reference origins, field/index projections, borrowing,
  escaping, lifetime, destruction, and prospective race analysis. Select the
  existing model as the single migration target; do not create a parallel P15
  place representation.

  Completed 2026-10-11. One Place model exists: `internal/sema/place.go` (`Place`, `PlaceProjection`, `PlaceRootID`, `PlaceRootKind`, `Relationship`). Producers: `resolvePlace`/`rootPlace`/`referencePlaceOrigin` (place.go, analyzer.go), the parameter-usage builder (`parameter_usage_place.go`, now carrying Sema root identities) and closure captures. Consumers: borrowing/reservations and move-into-itself (analyzer_lifetime.go, analyzer.go), defer (analyzer_defer.go), discard/availability (analyzer_discard.go, availability_diagnostics.go), custom free, Arena flow, escape analysis and its call-boundary/storage-domain facts, iterator dependencies/readiness, pitfall and foreign-extent analyses, synchronization resources (race groundwork), Result projections, semantic facts (`ResolvedAvailabilityTest`) and LSP hover. No parallel Place representation exists; `PlacesOverlap` survives only as the P15-16 adapter. This model is the single migration target.
- [x] P15-10 — Introduce stable compiler-owned `PlaceRootID` and root kind
  facts for local, parameter, static, addressed, allocation, and foreign roots
  as applicable. Root identity must be independent of source spelling and
  remain stable through shadowing, renaming, generic instantiation, and
  diagnostics presentation.

Partially completed 2026-10-04: `Place` carries `RootID PlaceRootID` and
`RootKind PlaceRootKind` beside the display name `Root`. Identities follow the
declaration token, are allocated deterministically in first-use order per
analysis, and are never addresses or spellings; `Relationship` compares them
when both Places have one, so equal names of different declarations are
disjoint. Root kinds follow the revised § 10 binding roles (local, parameter,
receiver including implicit member aliases, static including thread-local,
deref for an unknown referent reached through a reference). Remaining:
identity stability across generic instantiation, and identities on the Places
that interprocedural parameter-usage summaries build from names.

Completed 2026-10-11: parameter-usage summary Places carry the Sema root
identity of the parameter declaration (`parameterRootPlace`) with parameter or
receiver kind, and a generic body keeps one root identity per declaration
across instantiations (`TestPlaceRootIdentityCoversSummariesAndGenericInstantiation`).
Allocation and foreign roots are storage domains, carried by StorageOrigin and
the P15-11 storage identity rather than by additional binding-role kinds.
- [x] P15-11 — Add canonical `StorageIdentityID`, `InvalidationDomainID`,
  `EpochDependencyID`, `AddressSpaceID`, `BorrowID`, and `LifetimeID` facts.
  Define ownership, allocation, cloning, deterministic printing, and invalid/
  absent state; none may be treated as numeric addresses or source names.

  Completed 2026-10-11: `internal/sema/place_identities.go` defines `StorageIdentityID`, `InvalidationDomainID`, `EpochDependencyID`, `AddressSpaceID` (Ordinary/MMIO fixed, target-defined allocated after them), `BorrowID` and `LifetimeID`; zero is the absent identity of every kind; identities are allocated per analysis in first-use order from stable semantic keys (declaration locations, Arena domain identities, creation sites), print as `storage#N`, `domain#N`, `epoch-dep#N`, `space:...`, `borrow#N`, `lifetime#N`, and the allocator clones detached tables. Root Places carry `StorageIdentity`, `StorageOrigin` and `AddressSpace` (`TestPlaceIdentityAllocationAndPrinting`, `TestRootPlacesCarryStorageIdentity`).
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
- [x] P15-13 — Add dynamic-index identity based on a resolved expression/value
  identity, not source text. It must distinguish evaluated indexes while
  retaining P14 source type, signedness, exact fixed-array length, and
  bounds-proof facts.

  Completed 2026-10-11: a dynamic index projection carries `DynamicIndexID`, allocated from the evaluated index expression location (never its text), so different index expressions differ and re-resolving one keeps its identity; the projection retains the P14 index type, signedness, exact fixed-array length and bounds-proof kind from `ResolvedArrayIndexPlan`; two dynamic indexes remain potentially overlapping (`TestDynamicIndexIdentityAndCanonicalProjections`).
- [x] P15-14 — Normalize projections into field, constant-index, dynamic-index,
  union-payload, and dereference forms. Retain slice/property metadata only as
  deferred compatibility data; ordinary properties must not become addressable
  field Places.

  Completed 2026-10-11: `PlaceProjection.Canonical()` normalizes projections into field, constant-index, dynamic-index, union-payload and dereference; slice and property survive only as deferred compatibility kinds, and a property projection keeps its Place non-addressable.
- [x] P15-15 — Implement the one canonical `PlaceRelationship` query:
  `same`, `disjoint`, `contains`, `contained-by`, `potentially-overlapping`,
  and `unknown`. Make borrowing, move/escape/lifetime logic, and future
  consumers call it rather than reimplement alias decisions.
- [x] P15-16 — Retain `PlacesOverlap` only as a compatibility adapter over the
  relationship result. Migrate new correctness decisions first and add tests
  that prevent its legacy boolean behavior from silently classifying unknown
  or potentially-overlapping Places as disjoint.

Completed 2026-10-04: `Relationship` and the canonical relationship kinds were
already in place (P15-15). The borrow, defer, lifetime, reservation, and
move-into-itself checks now decide through `borrowRecordMayOverlap` /
`placesMayOverlap`, which consult `Relationship`; only Places without a
resolved root (no tracked storage, § 16 revised 2026-10-03) answer
"no overlap". `TestLegacyPlacesOverlapDelegatesToRelationship` pins the
adapter to the relationship result and
`TestLegacyPlacesOverlapHasNoCorrectnessCallers` keeps correctness code off
the adapter.
- [x] P15-17 — Implement relationship rules for same/nested fields, distinct
  stored struct fields, equal/distinct arbitrary-precision constant array
  indexes, dynamic indexes, union payload variants under active-variant proof,
  dereference projections, different storage identities, and containment.

Status 2026-10-04: every case except different storage identities is
implemented and covered by `TestPlaceRelationshipClassifiesCanonicalPaths`;
storage identities wait for `StorageIdentityID` (P15-11). Different root
declarations are disjoint through `PlaceRootID`.

Completed 2026-10-11: two Places with different known storage identities are
disjoint, while an unknown identity never proves disjointness
(`TestRootPlacesCarryStorageIdentity`).
- [x] P15-18 — Preserve finite alternative origin sets across control-flow
  joins, deterministically bounded at the documented implementation limit;
  degrade overflowed/unknown origin knowledge conservatively without adding a
  runtime origin tag or inventing a single origin.

Status 2026-10-04: alternative origins are joined pairwise by `Relationship`
and ambiguous provenance degrades to `unknown` (governance
`lowering.sec-mlir-package15`). The documented bound on the origin set is not
yet enforced.

Completed 2026-10-11: every origin join goes through `localOriginWithPlaces`,
which keeps up to 16 alternatives (`maxReferenceOriginAlternatives`) in
deterministic order and degrades a larger set to unknown provenance without
inventing a single origin; `samePlaceIdentity` now compares root identities, so
equally named roots of different declarations stay distinct alternatives
(`TestAlternativeOriginSetIsBounded`).
- [x] P15-19 — Add focused Sema tests for stable root identity independent of
  display name, relationship symmetry/containment, wide and above-`int64`
  indexes, field/index disjointness, dynamic overlap, union proof scope,
  dereference origin, alternative joins, unknown degradation, and legacy
  `PlacesOverlap` compatibility.

  Completed 2026-10-11: `TestPlaceRootIdentityFollowsDeclarations`, `TestPlaceRootIdentityCoversSummariesAndGenericInstantiation`, `TestPlaceRelationshipClassifiesCanonicalPaths` (now also checking symmetry up to containment direction), above-int64 indexes in that test and `testdata/sema/place_wide_constant_indexes.sec`, `TestPlaceRelationshipJoinsAlternativeOrigins`, `TestAlternativeOriginSetIsBounded`, `TestPlaceRelationshipDegradesAmbiguousProvenanceToUnknown`, `TestDynamicIndexIdentityAndCanonicalProjections`, `TestRootPlacesCarryStorageIdentity`, union proof scope in `TestUnionPayloadPlaceVariantsAreDisjoint` and the match-payload tests, and the legacy adapter tests `TestLegacyPlacesOverlapDelegatesToRelationship` / `TestLegacyPlacesOverlapHasNoCorrectnessCallers`.

## C. Publish compiler-resolved reference facts and plans

- [x] P15-20 — Define `ReferenceKind` (`shared-direct`, `mutable-direct`),
  `ReferenceValidityPolicy` (`proven`, `dynamic-epoch`), relocation class,
  semantic provenance kind, and a complete immutable `ReferenceFacts` model.
  Facts must include referent TypeID, possible origin Places/storage identities,
  address space, BorrowID/LifetimeID, authority, validity, epoch dependency,
  relocation, and provenance.

  Completed 2026-10-11: `internal/sema/reference_plans.go` defines `ReferenceKind` (shared-direct, mutable-direct), `ReferenceValidityPolicy` (proven, dynamic-epoch; empty = uncheckable), `ReferenceProvenanceKind`, and the immutable `ReferenceFacts` (referent type, origin Places, storage identities, address space, BorrowID/LifetimeID, mutable authority, validity with proof provenance, invalidation domain and epoch dependency, AddressStability plus a separate pin flag in place of the withdrawn RelocationClass per § 28 revised 2026-10-03, provenance).
- [x] P15-21 — Preserve the source-level classification: `ref T` is
  `CopyTrivial`; `ref mut T` is `MoveOnly`. A shared copy duplicates only the
  non-owning reference holder, while a mutable move transfers the holder and
  borrow obligation, never the referent.

  Completed 2026-10-11: plans carry `CopyClass` CopyTrivial for `ref T` and MoveOnly for `ref mut T`; existing Sema already copies shared holders and moves mutable holders without touching the referent.
- [x] P15-22 — Add a read-only `ResolvedPlaceOf(ast.Expression)` (or exact
  equivalent) keyed by the analyzed expression. It must expose existing facts
  only: no Place re-resolution, borrow creation, origin inference, diagnostics,
  AST mutation, or analyzer-state mutation.

  Completed 2026-10-11: `ResolvedPlaceOf(expr)` returns a clone of the Place recorded when Sema resolved that expression (recorded in `resolvePlace`); it performs no resolution, borrow creation, inference or diagnostics.
- [x] P15-23 — Add immutable, read-only `ResolvedReferencePlanOf` facts for
  every source reference creation and reborrow. The plan must contain the
  Place, reference facts, copy class, and proof/check policy established by
  successful Sema; defensive copies protect slices and big integers.

  Completed 2026-10-11: `ResolvedReferencePlanOf(node)` covers `ref`/`ref mut` expressions (reborrows flagged when the Place is reached through a reference) and implicit call-site borrows of owned arguments (`Implicit`), with defensive copies of Places, origin slices and big integers.
- [x] P15-24 — Add a read-only reference-use plan for read, write, reborrow,
  equality, parameter passing, return, and end-of-lifetime uses. The Semantic
  IR builder must consume this plan and must not rerun a borrow checker or
  infer reference behavior from source syntax.

  Completed 2026-10-11: `ResolvedReferenceUseOf(expr)` records read, write, reborrow, equality, pass and return uses with their reference facts, copy-trivial-read and trivial-replacement requirements; `ReferenceLifetimeEnds()` records resolved borrow ends (holder rebinding/release) with source locations.
- [x] P15-25 — Classify validity independently from physical representation:
  proven references retain closed proof provenance; dynamic references retain
  storage/domain/epoch, relocation, and address-space dependencies. Reject a
  safe reference that is neither proven nor dynamically checkable.

  Completed 2026-10-11: validity is classified independently of representation: known non-Arena origins are proven with closed proof provenance (lexical-borrow, caller-owned-borrow, static-storage, reborrow-of-valid-reference, fixed-address-storage), Arena-backed origins are dynamic-epoch with invalidation domain and epoch dependency, and unknown/ambiguous provenance is neither (`Checkable()` false) so it is rejected at the Semantic IR boundary.
- [x] P15-26 — Extend immutable `CompilationPlan` reference policy with a
  default 64-bit logical epoch width on both 32- and 64-bit targets. Permit a
  shorter width only from compile-time proof/exhaustion facts; prohibit runtime
  selection and do not encode epoch width in source type identity.

  Completed 2026-10-11: `layout.ResolvedReferencePolicy` is part of `ResolvedScalarPlan`: zero value = 64-bit logical epoch on every target, shorter widths only with a named compile-time proof, validated with the plan; Sema exposes it as `ReferencePolicy()`; the width never enters source reference types.
- [x] P15-27 — Ensure reference/subreference derivation only preserves or
  narrows lifetime, spatial extent, authority, compatible origin/domain/epoch,
  and address space. In particular, never derive `ref mut` from `ref`.

  Completed 2026-10-11: Sema rejects deriving `ref mut` from a shared reference or immutable Place (`TestReferenceDerivationNeverWidensAuthority`); reborrow facts preserve origin, domain/epoch and address space from the borrowed Place.
- [x] P15-28 — Preserve existing reference-return origin summaries through
  calls, fields, elements, and finite joins. Reject local/match-scoped/captured
  escaping origins while permitting approved parameter, external-owner, static,
  and addressed origins.

  Completed 2026-10-11: existing reference-return origin summaries are preserved (return uses carry origin facts) and local-origin returns stay rejected (S1128, `TestReferenceDerivationNeverWidensAuthority`, lifetime tests).
- [x] P15-29 — Add plan/query tests proving read-only behavior, defensive
  copies, identity stability, shared-copy/mutable-move classification,
  shared/mutable reborrows, source-origin joins, prohibited authority widening,
  local-return rejection, and no hidden borrow-state transition when a query is
  repeated.

  Completed 2026-10-11: `TestReferencePlansPublishCompleteFacts`, `TestReferenceUsePlansCoverAccessKinds`, `TestReferencePlanQueriesAreReadOnly` (repeat-query equality, defensive copies, no borrow/plan/identity mutation), `TestReferenceDerivationNeverWidensAuthority`.
- [x] P15-30 — Add the epoch/reference-plan matrix: proven local and addressed
  references, arena dependency fact without physical arena metadata, dynamic
  epoch fact, 64-bit default under both plans, shorter-width proof gate, no
  runtime-selected width, relocation/address-space preservation, and rejection
  of unknown uncheckable safe references.

  Completed 2026-10-11: proven local and static references, Arena element reborrow as a dynamic-epoch fact with domain/epoch identities and no physical metadata, 64-bit default on 32- and 64-bit plans, shorter-width proof gate, address-space/stability preservation and unknown-provenance rejection (`TestReferencePolicyEpochWidth`, `TestUncheckableReferenceFacts`).

## D. Build the Semantic IR Place and direct-reference core

- [x] P15-31 — Add canonical Semantic IR Place IDs/root facts, projection
  records, reference types/facts, deterministic printer support, cloning, and
  verifier ownership. Place, value, storage, safe reference, and RawPtr must
  remain distinct categories.

  Completed 2026-10-11: `internal/ir/semantic/place_reference.go` adds the `place<T, ro|rw>` and `ref`/`ref_mut` type kinds (authority in `Type.Mutable`, part of type identity), `PlaceMetadata` (stable root identity/kind, storage identity, address space, dynamic index identity, variant proof) and `ReferenceMetadata` (borrow/lifetime/source borrow, validity with proof or domain/epoch, address space, stability, pin, provenance, origin storages), deterministic printing, metadata cloning and verifier ownership; Place, value, storage, safe reference and RawPtr stay distinct.
- [x] P15-32 — Add `PlaceValueRootOp` and `PlaceStorageRootOp` with stable root,
  storage, address-space, and authority facts. Immutable SSA roots are `ro`;
  storage-derived authority may not exceed storage mutability.

  Completed 2026-10-11: `place.value-root` (immutable SSA root, always ro) and `place.storage-root` (authority never exceeds storage mutability; storage declaration/initialization must dominate it) with complete root metadata.
- [x] P15-33 — Add `PlaceFieldOp`, `PlaceArrayIndexInBoundsOp`,
  `PlaceArrayElementOp`, `PlaceUnionPayloadOp`, and `ReferenceDerefPlaceOp`.
  Reuse P13 field identity, P14 exact index/bounds plans, and P11/P12 active
  variant facts; no operation may calculate offsets, stride, GEPs, or copy an
  enclosing aggregate merely to form a subplace.

  Completed 2026-10-11: `place.field` (P13 field identity), `place.array-index-in-bounds` and `place.array-element` (P14 exact length, bounds kind/proof/guard, exact constant index checked against the length), `place.union-payload` (requires the active-variant proof) and `ref.deref-place`; all preserve authority and compute no offsets, strides or GEPs.
- [x] P15-34 — Add `PlaceReadOp` for `copy-trivial` whole-value reads only and
  `PlaceWriteOp` for writable, copy-trivial, trivially destructible replacement
  only. Explicitly reject move-only/semantic-copy reads, non-trivial write
  cleanup, unresolved contracts, and authority mismatch without partial IR.

  Completed 2026-10-11: `place.read` accepts copy-trivial referents only and `place.write` read/write Places with trivially replaceable referents only; move-only/semantic-copy reads, non-trivial writes and authority mismatches are rejected.
- [x] P15-35 — Add `ReferenceBorrowSharedOp` and `ReferenceBorrowMutableOp`.
  Require compatible Place authority plus complete reference metadata; creation
  is non-owning and has no implicit allocation or physical address operation.

  Completed 2026-10-11: `ref.borrow-shared` and `ref.borrow-mut` require compatible Place authority (mutable borrow needs rw) and complete reference metadata; creation has no source borrow, allocation or address operation.
- [x] P15-36 — Add `ReferenceCopySharedOp`, `ReferenceMoveOp`,
  `ReferenceReborrowSharedOp`, and `ReferenceReborrowMutableOp`. Verify new
  BorrowID/LifetimeID derivation, non-increasing authority, compatible
  provenance/epoch/address space, and no `ref` → `ref mut` path.

  Completed 2026-10-11: `ref.copy-shared` (shared only, keeps its borrow), `ref.move` (mutable only, keeps its borrow), `ref.reborrow-shared` and `ref.reborrow-mut` (new borrow derived from the source borrow; no `ref` -> `ref mut`).
- [x] P15-37 — Add `ReferenceIsValidOp`, `ReferenceGenerationFailureOp`, and
  `ReferenceEndBorrowOp`. The dynamic false edge must terminate with
  `panic.invalid-reference-generation`; borrow end has no runtime semantics but
  makes subsequent reachable use invalid.

  Completed 2026-10-11: `ref.is-valid` (dynamic-epoch references only), `fail.reference-generation` (terminator with the registered `InvalidReferenceGeneration` reason #8, counted as a panic endpoint by @noPanic verification) and `ref.end-borrow`.
- [x] P15-38 — Add `ReferenceCompareOp` for `eq`/`ne` over compatible live
  direct references. It represents storage identity plus location equality,
  never raw address/integer equality, and it must participate in dynamic
  validity guarding.

  Completed 2026-10-11: `ref.compare eq|ne` over compatible direct references; a dynamic-epoch operand requires its validity guard like a dereference.
- [x] P15-39 — Implement Semantic IR verification for Place type/projection
  identity, authority narrowing, exact big-int index facts, root/storage/domain
  consistency, reference facts, borrow/lifetime transitions, dynamic CFG guards,
  failure endpoints, end-borrow reachability, and no Place escape as a source
  value, call argument, stored field, array element, union payload, or return.

  Completed 2026-10-11: per-operation verification (types/projection identity, authority, exact indexes, root/storage metadata, reference facts, borrow derivation) plus function-level checks: no Place escapes through return, calls, storage, aggregates, union payloads or block arguments; every dynamic-epoch dereference/compare is guarded by its `ref.is-valid` whose false edge terminates in `fail.reference-generation`; no reference use is reachable after its `ref.end-borrow`.
- [x] P15-40 — Add deterministic Semantic IR tests covering all root/projection
  operations, wide referents, constant/dynamic fixed-array elements, union
  payloads, dereference Places, reads/writes, borrows/copy/move/reborrows,
  validity, compare, end-borrow, printer stability, malformed facts, and every
  P15 unsupported ownership boundary.

  Completed 2026-10-11: `internal/ir/semantic/package15_test.go` verifies and deterministically prints a module covering storage/value roots, field, constant, wide (above uint64) and guarded dynamic array elements, a variant-proven union payload, read/write through ref mut, borrow/copy/move/reborrow, compare, end-borrow and a guarded dynamic-epoch reference, and rejects 19 malformed variants (authority widening, ref -> ref mut, ref mut copy, borrow derivation, uncheckable/unproven facts, read-only write, out-of-range index, missing guards and proofs, move-only read, missing failure endpoint or wrong reason, use after end-borrow, Place returned or stored).

## E. Lower resolved source behavior without re-analysis

- [x] P15-41 — Refactor the Semantic IR builder so all reference and Place
  paths consume P15 resolved plans. Unknown/missing facts are package-tagged
  unsupported errors; failed builds return no partially assembled module or
  placeholder operation.

  Completed 2026-10-11: at package 15 (`BuildOptions.MaxPackage = 15`; the default stays 14) `internal/ir/semantic/reference_lowering.go` routes every expression with a resolved reference plan, a resolved reference use or a reference type, every assignment target and every reference parameter through the P15 plans: missing plans, uncheckable references and not-yet-lowered plan kinds are package-15 `UnsupportedFeatureError`s naming the planned facts (for example "borrow of reference shared-direct of local storage"), the pre-P15 reference-parameter approximation is never used, `ref T` / `ref mut T` intern as P15 reference types, and `Build` returns no module on any failure. Reference-free programs build identically at packages 14 and 15, and earlier packages keep their boundaries (`TestPackage15BuilderConsumesReferencePlans`, `TestPackage15BoundaryLeavesEarlierPackagesUnchanged`).
- [ ] P15-42 — Lower local/static/addressed roots and reference-dependent
  mutable storage through high-level storage plus Place roots. Mark/address
  track address-taken storage so P5 trivial-core lowering cannot erase it before
  the P15 representation boundary is discharged.

  Status 2026-10-11 (partial): borrows of whole local bindings lower through Place roots: an immutable binding becomes a read-only `place.value-root` over its SSA value, a mutable binding a `place.storage-root` over its P5 storage, which is marked `AddressTaken` (printed `address-taken`; the verifier rejects a storage root over unmarked storage, so P5 trivial-core lowering cannot erase it) and is neither loaded nor copied; the borrow carries the resolved reference facts unchanged (`TestPackage15LowersLocalRootBorrows`). Static roots (2026-10-16, with P15-42a): `ref` / `ref mut` of a module static or function-local static lowers to `place.static-root @N` with the static's authority, marks the static address-taken and carries Sema's static-storage facts (`TestPackage15LowersStaticPlaceRoots`). Remaining: addressed (fixed-address/MMIO) roots, blocked by MD-077 (no Semantic IR register types or operations are specified; Semantic IR rejects every register access).
- [ ] P15-42a — Added 2026-10-11 (dependency discovered by P15-42): represent
  static (module-level), thread-local and addressed (fixed-address binding,
  register/MMIO) storage in Semantic IR, which today has only local-automatic
  storage, then lower static and addressed Place roots through it with their
  initialization, mutability, address-stability and memory-space facts.
  Governance: `semantic-ir.core-v2` and `lowering.sec-mlir-package15`.

  Partial 2026-10-16: static storage is represented (semantic_ir.md § 28). Sema publishes `ResolvedStaticOf` and `ResolvedStaticDeclarations` for module-level bindings and function-local `static let`.

  The Semantic IR module has `statics { @N "module::name" … init=… }`:
  - each static has its own type, mutability, enclosing function and address-taken flag;
  - each has an exact compile-time initializer, with no startup function (§ 28(5));
  - operations are `static.load`, `static.store` (mutable statics only) and `place.static-root`.

  A returned reference to a module static is recorded as a `static@N` origin in the return summary and verified by tracing.

  This also fixes a miscompilation that predates P15 and affected every package level: a function-local `static let mut` was lowered as automatic local storage and re-initialized on every call. The legacy direct `emit-llvm` and `emit-mlir` routes had the same defect (an `alloca`); they now reject function-local statics. The Sec MLIR emitter rejects statics, since no schema represents them yet (`TestStaticStorageLowersToModuleStatics`, `TestPackage15LowersStaticPlaceRoots`, `TestVerifierChecksStaticStorage`, `TestResolvedStaticOfDistinguishesStaticStorage`, `TestStaticStorageEmitRoutes`, `TestLegacyRoutesRejectFunctionLocalStatics`).

  Current limits, each a package boundary:
  - statics of integer-family or bool type with a literal (or default) initializer only;
  - no compound assignment to statics;
  - no statics of another module.

  Remaining:
  - addressed storage (fixed-address register/MMIO bindings) and its Place roots;
  - thread-local storage: Sec 0.1 has no thread-local static syntax, only `ThreadLocal[T]` owning a TLS key (static.md § 4);
  - aggregate and compile-time-evaluated (non-literal) static initializers.
- [x] P15-43 — Lower direct field/subfield borrows via Place projections;
  demonstrate that P13 struct values are neither loaded nor copied solely to
  create a reference. Keep properties out of field-Place lowering.

  Completed 2026-10-11: a borrow whose resolved Place reaches its local root through stored struct fields lowers as the root Place followed by one `place.field` per field, each carrying the P13 field identity from `ResolvedStructMemberOf` and checked against the resolved Place projection; authority follows the root (immutable SSA root ro, mutable storage root rw and address-taken) and a mutable field borrow requires a read/write root. The struct value is neither loaded (`storage.load`) nor copied (`struct.extract-field`) to form the subplace. Properties never become field Places: Sema rejects borrowing them ("non-addressable place") and the builder additionally rejects any non-stored member (`TestPackage15LowersFieldBorrowsWithoutAggregateCopies`).
- [x] P15-44 — Lower fixed-array element borrows through a root array Place and
  P14's original evaluated index/bounds plan. A dynamic ordinary borrow must
  evaluate root then index once, emit one place-bounds predicate, branch before
  forming the element/reference, and use existing ordinary bounds failure on
  the false edge; a proven borrow emits no redundant branch.

  Completed 2026-10-11: borrow paths may now mix stored fields and fixed-array elements. Each element step consumes the `ResolvedArrayIndexPlanOf` plan of its index expression (checked against the resolved Place projection and required to be a fixed array). The root Place is formed first, then the index is evaluated exactly once. A proven-safe plan emits `place.array-element` directly with `bounds=proven-safe`, its P14 proof kind and the constant index, without a predicate or branch. A runtime-checked ordinary plan emits one `place.array-index-in-bounds`, branches on it, reaches `fail.bounds operation="fixed-array-index"` on the false edge, and only on the true edge forms `place.array-element` (`runtime-check`, `guarded`, `guard=` the predicate) and the reference. The element Place carries the projection's DynamicIndexID, and authority follows the root. Neither the array nor the element is loaded or copied (`array.extract`/`storage.load` absent). By-value parameters are now accepted as immutable SSA roots (`TestPackage15LowersFixedArrayElementBorrows`).
- [x] P15-45 — Lower union/Result payload reference bindings only from the
  compiler-resolved matching-variant Place. Keep branch-local BorrowID/lifetime
  scope, retain guard dominance, and reject match-result, outer-assignment,
  return, and lambda capture escapes when Sema says the payload origin is
  scoped.

  Partial 2026-10-13: Sema publishes a `ResolvedReferencePlan` for every `ref`/`ref mut` union, Option and Result payload binding, keyed by the bound identifier (the stable pattern node; `MatchPattern.Expression()` builds fresh wrappers). Semantic IR lowers union and Option single-payload borrows whose subject is a whole local binding or by-value parameter: the subject's own root Place (value root, or address-taken storage root for a mutable binding), one `place.union-payload` with `guard=` naming the arm's `union.is-variant` predicate, then `ref.borrow-shared`/`ref.borrow-mut` with the plan's facts unchanged. No `union.unwrap-payload` or copy is emitted; a `ref mut` payload writes through `place.write`. The binding is arm-scoped and Sema's existing match-scoped origin rules reject the escapes. The verifier checks the proof instead of trusting `VariantProven`: the guard must test the same variant of the Place root's own union value, and the payload Place must be confined to the guard's true edge (paths through a loop back to the test are not counted) (`TestPackage15LowersUnionPayloadReferenceBindings`, `TestPackage15PayloadReferenceBoundaries`, `TestPackage15VerifierRejectsUnprovenPayloadPlaces`). Subjects reached through stored fields (2026-10-16): a subject such as `match holder.figure` or `match outer.holder.figure` borrows its payload from the root Place projected through the same stored fields (`place.field`), and inside a method `match self.figure` or the implicit `match figure` projects from `ref.deref-place` of the receiver. The verifier now compares canonical Place chains — the root (SSA value, storage, or dereference of one reference value) plus the stored-field path — between the payload Place and the tested union value, following `struct.extract-field`, `storage.load` and `place.read` on the value side (`TestPackage15LowersFieldSubjectPayloadBorrows`, `TestPackage15LowersReceiverFieldPayloadBorrows`, `TestPackage15VerifierRejectsPayloadPlaceOfOtherField`). Completed 2026-10-16: Result `Ok`/`Err` payloads borrow through `place.union-payload` over the Result Place with `result.is-err` as the guard (Ok proven on its false edge, Err on its true edge); struct-like payloads borrow as their canonical Package 13 synthetic payload struct; a subject reached through an array element or a non-receiver reference holder forms its Place once, is read from it for the match (`place.read`) and the payload is projected from that same Place, so the index or dereference is evaluated exactly once (`TestPackage15LowersRemainingPayloadBorrowShapes`, `TestPackage15VerifierChecksResultPayloadEdges`). Sema now infers a mutable receiver from member writes inside match arms, `ref mut` borrows of receiver members and `ref mut` payload bindings of receiver-rooted subjects (`TestReceiverMutabilityCountsMatchArmsAndMutableBorrows`). Deferred to P17 by the package boundary: a borrowed payload of a non-copy-trivial subject reached through an element or reference, whose subject read would be a non-trivial read.
- [x] P15-46 — Lower dereference/read/write/reborrow/equality through one
  validity policy. Proven uses retain proof metadata and no runtime check;
  dynamic uses validate the same reference SSA, branch once, terminate the
  false edge in reference-generation failure, and allow the guarded operation
  only on the true edge.

  Completed 2026-10-15 (partial 2026-10-12, explicit reborrow after MD-061): reads, writes and equality are lowered through one validity policy taken from the facts of the exact reference SSA they consume (the builder records the metadata of every reference it creates). A read through a holder is `ref.deref-place` (Place authority from the reference), the resolved field/fixed-array element steps via the shared `projectPlace`, and a copy-trivial `place.read`. A write forms the destination Place first, then evaluates the source and ends in `place.write` (trivially replaceable referents through `ref mut` only). Equality is `ref.compare eq|ne`. Proven references keep their proof metadata and emit no check. A dynamic-epoch reference is validated once with `ref.is-valid` on the same SSA value: the false edge ends in `fail.reference-generation` and the guarded operation, carrying `ReferenceGuard`, exists only on the true edge. Tests: `TestPackage15LowersProvenReferenceUses`, plus the white-box `TestPackage15DynamicReferenceUseValidatesOnce`, because Arena references cannot be built from source yet. Explicit reborrow (2026-10-15, MD-061): `ref r` / `ref mut r` over a reference holder is typed `ref T` / `ref mut T` in Sema and lowers to `ref.reborrow-shared` / `ref.reborrow-mut` of the holder's reference with a derived BorrowID and the holder's storage origins (`TestPackage15LowersExplicitReborrows`). Compound assignment and non-trivial reads/writes through references stay unsupported (P17).
- [ ] P15-47 — Lower reference parameters, implicit owned-place borrows,
  existing shared-reference passing, mutable-reference move/reborrow passing,
  allowed returned input/subfield references, and preserved return-origin
  summaries. Do not choose a physical ABI or silently copy `ref mut`.

  Partial 2026-10-12: Sema records callee-side facts for every ref/ref mut parameter (`ReferenceParameterFactsOf`; caller-owned-borrow proof, or dynamic-epoch for Arena-backed referents). Semantic IR parameters carry them (`Parameter.Reference`, printed and verifier-required), and the verifier now tracks dynamic-epoch parameters so their dereferences need guards (the P15-46 note). Reads and writes through a reference parameter use the P15-46 policy. At a call site an owned Place passed to a ref or ref mut parameter lowers Sema's implicit borrow as a Place borrow, an explicit `ref` argument its borrow, an existing shared reference is passed with `ref.copy-shared` (same borrow), and an existing ref mut is passed with `ref.reborrow-mut`, never copied. The new argument actions `shared-reference` and `mutable-reference` are verifier-checked against the parameter type (`TestPackage15LowersReferenceParametersAndPassing`, `TestPackage15DynamicReferenceUseValidatesOnce`). Instance-method receivers (2026-10-12): Sema publishes `ReceiverReferenceFactsOf` per instance method, `ResolvedImplicitReceiverOf` for `self` and implicit stored-field identifiers (reads and assignment targets), and an implicit borrow or pass use for the receiver expression of each instance-method call. From package 15 Semantic IR builds non-generic instance methods with a borrowed receiver, with an explicit first parameter `self` of type ref/ref mut Target carrying those facts. Field reads and writes through `self.f` or an implicit `f` dereference it and project `place.field`. A call passes the receiver: an implicit borrow of an owned Place, a pass of a reference holder, or the caller's own `self` (copied when shared), including bare method calls inside a method (`TestPackage15LowersInstanceMethodReceivers`, `TestPackage15ReceiverBoundaries`, `TestPackage15ImplicitFieldWriteUsesMutableReceiver`). Returned references (2026-10-16): Sema publishes `ReturnReferenceSummaryOf` (positional origins `$param:N`/`$receiver` with canonical projections) and `ResolvedCallReferenceOf` (caller-side facts instantiated from the callee summary over the actual arguments). A function returning a direct reference carries `ReturnOrigins` metadata (printed `returns-from=[param#0.field#0]`, `receiver.field#0`, `[1]`/`[*]` for elements). A returned reference may be the parameter's own reference, a sub-Place reborrow of it (`return ref pair.left`; `ref self.left` in a method reborrows the receiver), a local holder of one, or a call result. The verifier traces every returned reference through copies, moves, reborrows, dereferences, field and element Places and calls (via the callee's summary), and requires each traced origin to be listed. A call returning a reference carries the instantiated facts on its result (`TestPackage15LowersReturnedReferences`, `TestPackage15VerifierChecksReturnedReferenceOrigins`). Static and addressed origins stay unsupported until P15-42a. Receiver reborrows (2026-10-16): Sema publishes `ResolvedReceiverReborrowOf(call)` for a call on the enclosing method's own mutable receiver. `Bump()` and `self.Bump()` inside a mutating method lower to `ref.reborrow-mut` of the receiver with a fresh BorrowID/LifetimeID per call, or `ref.reborrow-shared` when the callee only reads. The receiver is never copied. `self` or a receiver field passed to a ref/ref mut parameter (`Read(self)`, `ReadInner(inner)`, `Write(self)`), and explicit `ref self` / `ref mut self`, derive the borrow from the dereferenced receiver with Sema's implicit-borrow facts (`TestPackage15ReborrowsReceiversForNestedCallsAndArguments`). The temporary-borrow rejection ("cannot borrow temporary expression") now carries S1168. Remaining: consuming receivers; generic implementations; and receivers that are temporaries without a Place (for example `Start().Next()`), which keep try-chain-intermediate-lowering blocked and await MD-076 (Sema accepts call-lifetime temporaries without an explicit rule).
- [x] P15-48 — Integrate effects and `@noPanic`: proven use adds no invalid-
  reference panic; a reachable dynamic failure adds the exact panic reason;
  validation itself adds neither allocation nor blocking; borrow creation/end
  remains runtime-effect-free. Reject `@noPanic` where a dynamic failure remains
  reachable.

  Completed 2026-10-12: Sema adds the panic effect `may-panic-invalid-reference` with the exact reason `InvalidReferenceGeneration` for each read, write, equality and reborrow use of a dynamic-epoch reference, which are exactly the uses Semantic IR validates. A later, more specific use of the same node replaces the effect. Proven uses, borrow creation and end, and reference passing add no effect, and validation adds no allocation or blocking effect. `@noPanic` (S1092) and ISR panic checks reject a reachable dynamic use with mentor help. In Semantic IR, the verifier rejects a `@noPanic` function that reaches `fail.reference-generation` and accepts proven uses (`TestDynamicReferenceUsesCarryInvalidReferencePanicEffect`, `TestNoPanicRejectsReachableInvalidReferenceGeneration`).
- [ ] P15-49 — Add source-to-Semantic-IR tests for local and parameter refs,
  field and array subreferences, shared copy, mutable move, all legal/
  prohibited reborrows, dynamic validity CFG, equality, writes, borrow ends,
  effects, and `@noPanic`. Assert evaluation order, exact SSA reuse, no
  aggregate copy, no partial IR, no `undef`/poison, no hidden allocation, and
  no physical reference/layout vocabulary.

## F. Implement Sec MLIR schema 11 and its three verifiers

- [x] P15-50 — Bump compiler-generated modules to schema 11 only after schema
  9 and schema 10 remain accepted compatibility inputs and schema 12 remains
  rejected. Update ODS/C++ registration, parser/printer, version gates, and
  generated-module tests.

  Completed 2026-10-16. At that completion checkpoint, `sec-mlir-opt` accepted schema 11 and rejected 12 (`invalid-schema12-module-metadata.mlir`, formerly the schema-11 rejection test); schemas 1–10 remain accepted regression inputs. A module that declares a schema below 11 but uses a Place or reference type is rejected. The Go emitter declares schema 11 (`dialectSchemaVersion`), and the generated-module tests in `emitter_test.go` and the `cmd/compiler` emit tests expect 11. The full lit suite (93 tests) and the emitter-dependent Go suites pass with `SEC_MLIR_BIN` set.
  Subsequent P16-38 integration (2026-10-19) accepts schema 12 and rejects
  unknown schema 13. The existing schema-12 negative fixture now checks missing
  required metadata. Default emission remains schema 11; explicit P16 schema
  selection does not yet emit slice types/operations.

- [x] P15-51 — Implement `!sec.place<T,"ro">`, `!sec.place<T,"rw">`,
  `!sec.ref<T>`, and `!sec.ref_mut<T>` with exact type parsing, printing, type
  constraints, authority rules, and no physical address/epoch fields.

  Completed 2026-10-16: the ODS types `Sec_PlaceType` (element type plus authority `"ro"`/`"rw"`), `Sec_RefType` and `Sec_RefMutType` (element type only) carry no physical address, generation or epoch parameter. Their verifiers require a non-void ordinary referent that is not storage, a Place or a reference, and an exact authority string (`schema11-place-reference-types.mlir` round-trip, `invalid-schema11-place-reference-types.mlir`).
- [x] P15-52 — Implement schema-11 root/projection/read/write operations:
  `sec.place.value`, `sec.place.storage`, `sec.place.field`,
  `sec.place.array_index_in_bounds`, `sec.place.array_element`,
  `sec.place.union_payload`, `sec.place.deref`, `sec.place.read`, and
  `sec.place.write`. Require deterministic root/storage/address-space,
  field/variant, P14-compatible bounds, authority, action, and proof attrs.
- [x] P15-53 — Implement schema-11 reference operations:
  `sec.ref.borrow_shared`, `sec.ref.borrow_mut`, `sec.ref.copy_shared`,
  `sec.ref.move`, `sec.ref.reborrow_shared`, `sec.ref.reborrow_mut`,
  `sec.ref.is_valid`, `sec.ref.compare`, `sec.ref.end_borrow`, and terminating
  `sec.fail.reference_generation` with its canonical panic ID.

  Completed 2026-10-16 (P15-52 and P15-53): ODS operations `sec.place.value`, `.storage`, `.field`, `.array_index_in_bounds`, `.array_element`, `.union_payload`, `.deref`, `.read`, `.write`, `sec.ref.borrow_shared`, `.borrow_mut`, `.copy_shared`, `.move`, `.reborrow_shared`, `.reborrow_mut`, `.is_valid`, `.compare`, `.end_borrow` and the terminator `sec.fail.reference_generation`.
  - Root metadata: `place_root_id`, `storage_identity`, `address_space` (ordinary/mmio).
  - Reference metadata: `borrow_id`, `lifetime_id`, `validity_policy` (proven/dynamic-epoch), `validity_proof`, `invalidation_domain`, `epoch_dependency`, `address_stability`, `provenance`, and `source_borrow` for derivations. Address stability replaces the v11 snapshot's `relocation_class`, following package15.md § 28 as revised 2026-10-03.
  - Per-op verifiers check exact element types, authority (no widening, `ro`/`rw` per deref, rw for writes and mutable borrows, no shared-to-mutable reborrow, `ref_mut` never copied), array bounds metadata, union/Result payload types including the synthetic payload struct of a fields variant, copy-trivial reads, policy-consistent reference facts, derived borrow identities, eq/ne comparison and the canonical failure reason.
  - Schema-10 modules cannot use any of them.
  - Guard dominance, escapes and borrow semantics are left to P15-54..P15-56.
  - lit tests: `schema11-place-reference-ops.mlir`, `schema11-place-projections.mlir`, `invalid-schema11-place-reference-ops.mlir` (16 cases); the full suite (96 tests) passes.

- [x] P15-54 — Register `--sec-verify-places`. Verify roots, wrapper and
  projection types, authority non-escalation, field identity/type, P14 array
  proof or guard, union active variant, deref authority, property exclusion,
  and every prohibited Place escape.
- [x] P15-55 — Register `--sec-verify-reference-guards`. For dynamic facts,
  require validation of the same reference SSA, true-edge dominance of every
  dereference/read/write/reborrow/compare, and a false path ending in
  `sec.fail.reference_generation`. For proven facts, require closed
  compiler-owned proof provenance and no bogus runtime guard requirement.
- [x] P15-56 — Register `--sec-verify-borrow-semantics`. Verify immutable
  metadata consistency, shared-copy authority, mutable no-copy/move behavior,
  legal reborrow origins, BorrowID/LifetimeID transitions, no reachable
  post-end use, allowed return origins, and place/reference type agreement.
  It must not duplicate Sema's source-language borrow checker.

  Completed 2026-10-16 (P15-54, P15-55; P15-56 partial): `mlir/lib/Analysis/VerifyPlaceReferences.cpp` registers `--sec-verify-places`, `--sec-verify-reference-guards` and `--sec-verify-borrow-semantics`. Root/projection types, authority, field identity, deref authority and copy/move authority are enforced by the per-op verifiers (P15-52/53); properties never become Places.
  - `--sec-verify-places` rejects:
    - Places in function signatures, block arguments, returns, calls, branches and any consumer other than Place and borrow ops;
    - a runtime-check element Place not dominated by the exclusive true edge of `sec.place.array_index_in_bounds` on the same base and index;
    - a payload Place not dominated by a same-variant `sec.union.is_variant` (or `sec.result.is_err`, Ok on the false edge) on a value denoting the same Place. A canonical chain (value/storage/deref root plus field path) is compared through `place.read`, `struct.extract` and `storage.load`.
  - `--sec-verify-reference-guards` requires every deref, compare and direct reborrow of a dynamic-epoch reference to be dominated by the true edge of `sec.ref.is_valid` on the same SSA value, whose false edge ends in `sec.fail.reference_generation`. Proven references need no guard; their proof provenance is required by the per-op verifier.
  - `--sec-verify-borrow-semantics` requires:
    - reborrows to name the source reference's `borrow_id` (directly or through a dereferenced sub-Place);
    - `end_borrow` to name the ended reference's own identities;
    - no use to be reachable after `end_borrow` or after `ref.move`.
  - lit: `Analysis/verify-place-references.mlir` (4 well-formed CFGs) and `Analysis/invalid-verify-place-references.mlir` (12 violations); the full suite (98 tests) passes.

  P15-56 completed 2026-10-17 together with P15-57:
  - A reference parameter's facts come from its `sec.reference` argument attribute, and a call result's from the call's `sec.reference` attribute. Uses of dynamic-epoch parameters are therefore guard-checked, and `end_borrow`/derivations of them are checked against their identities.
  - Returned origins: a function with a reference result must carry a non-empty `sec.return_origins` whose parameter entries name reference parameters. Every returned reference must derive only from listed parameters, traced through copies, moves, reborrows, dereferences, field/element/payload projections, block arguments and calls (through their reference arguments). Local storage and value roots are rejected. Projection paths are Sema facts and are not re-derived (MD-085).
  - Fix: a reborrow from an element or payload sub-Place of a dereference was rejected as underived; derivation now follows field, element and payload projections down to `place.deref`.
- [x] P15-57 — Extend the Go emitter from verified Semantic IR to schema 11.
  Preserve TypeIDs, root/storage/domain/epoch/borrow/lifetime facts, exact
  big-int index/proof metadata, source locations, CFG ownership, and high-level
  storage. Reject unrepresented physical ABI/layout requests explicitly.

  Completed 2026-10-17 (`internal/lowering/secmlir/place_reference.go`):
  - Types: `TypePlace` becomes `!sec.place<T, "ro"|"rw">` and `TypeReference` becomes `!sec.ref<T>`/`!sec.ref_mut<T>`, recursively over the referent.
  - Operations: every Semantic IR Place/reference operation lowers to its schema-11 operation.
    - Roots carry `place_root_id`, `storage_identity` and `address_space`.
    - Projections carry the P13 field ordinal, P14 `bounds_kind`/`bounds_proof` and `index_signed`, and the P11/P12 variant (Result Ok 0, Err 1).
    - Reference operations carry `borrow_id`, `lifetime_id`, `validity_policy`, `validity_proof`, `invalidation_domain`/`epoch_dependency` for dynamic-epoch references, `address_stability`, `provenance` and `source_borrow` for reborrows.
    - `fail.reference_generation` carries `panic.invalid-reference-generation`. Source locations and the CFG (guard branches, `fail.bounds` and failure blocks) are kept as is.
  - Facts outside the operation properties, all with spellings awaiting confirmation (MD-084):
    - `sec.reference` dictionary on reference parameters and call results;
    - `sec.return_origins` on reference-returning functions;
    - `sec.address_taken = true` on address-taken storage;
    - `sec.origin_storages`, `sec.pinned` and `sec.reference_address_space` on reference operations.
  - Calls pass `shared-reference`/`mutable-reference` argument actions; the dialect call verifier now requires the action matching each operand type.
  - Rejected as unsupported lowering instead of dropped: target-defined address spaces and static Place roots. Module statics were already rejected.
  - `sec-mlir-opt` validates the new attributes (`verifyRegionArgAttribute` for parameter facts), and the dialect `kSchemaVersion` is now 11.
  - Tests:
    - `TestPackage15EmitsSchema11PlacesAndReferences` covers the returned, place-borrow, payload and parameters fixtures on 32/64-bit plans. It checks deterministic output, an unmutated module and no physical vocabulary, and that all three verifiers pass.
    - `TestPackage15EmitsDynamicReferenceGuards` covers a dynamic parameter guard, move/reborrow/copy/compare and the endpoint.
    - `TestPackage15EmitRejectsUnrepresentedReferenceFacts`.
    - lit `verify-place-references.mlir` adds a dynamic parameter, origins through block arguments, and a call with reference actions.
    - lit `invalid-verify-place-references.mlir` adds 8 violations.
    - The full lit suite (98) passes.
  - Known hole (verified 2026-10-17): the CLI routes `emit-ir`/`emit-sec-mlir` still build at package ceiling 14 and reject reference programs (MD-086). TypeIDs remain structural MLIR types, as in schemas 9–10.
- [x] P15-58 — Add schema-11 MLIR round-trip and invalid suites for all types
  and operations; schema 9/10 regression; malformed root/authority/field/index/
  variant/reference metadata; place escape; guard errors; borrow errors; end
  use; deterministic printing; and canonical panic failure.

  Completed 2026-10-17:
  - Round-trip:
    - `schema11-place-reference-types.mlir`, `schema11-place-reference-ops.mlir` and `schema11-place-projections.mlir` cover every type and operation.
    - The new `schema11-emitter-metadata.mlir` covers the emitter facts outside operation properties: `sec.reference` on parameters and call results, `sec.return_origins`, `sec.address_taken`, `sec.origin_storages`, `sec.pinned`, `sec.reference_address_space`, and reference argument actions. It prints the module twice and `diff`s the two outputs (deterministic printing), then passes the three verifiers.
  - Invalid, per-op contracts (`invalid-schema11-place-reference-ops.mlir`, 16 → 54 cases):
    - root: address space, value/storage type, storage operand, a rw root over immutable storage (new check, package15.md § 64);
    - field: non-struct base, ordinal range, field type;
    - index: non-array base, `index_signed`, non-integer index, element type and authority, `bounds_kind`, runtime and proven proofs;
    - variant: Result variant 2, union variant range, payload authority;
    - deref: operand and referent;
    - read/write: types;
    - borrow: result kinds, zero identities, empty provenance, a proven reference with an epoch, unknown policy;
    - copy/move, reborrow referent, ro-Place mutable reborrow, reborrow source;
    - `is_valid`/`end_borrow` operands, zero `lifetime_id`, compare referents;
    - `fail.reference_generation` not last in its block.
  - Invalid emitter metadata (`invalid-schema11-emitter-metadata.mlir`, 19 cases):
    - each malformed `sec.reference` key, with misplaced or malformed `sec.reference`;
    - empty `sec.return_origins`;
    - `sec.address_taken` value and placement;
    - `sec.origin_storages`, `sec.pinned` and `sec.reference_address_space` value and placement (now restricted to operations defining one reference);
    - mismatched argument actions for `!sec.ref_mut` and for ordinary values.
  - Place escape, guard errors, borrow errors and post-end use: `Analysis/invalid-verify-place-references.mlir` (21 cases). It now also covers a Place root over storage whose declaration lacks `sec.address_taken = true`, which `--sec-verify-places` requires (package15.md § 67, spelling MD-084).
  - Canonical panic failure: wrong reason (existing) and terminator placement.
  - Schema 9/10 regression: the schema-9/10 round-trip and invalid suites are unchanged and pass.
  - The full lit suite (100 tests) and the emitter Go tests with `SEC_MLIR_BIN` pass.

## G. Prove predecessor integration and architecture boundaries

- [x] P15-59 — P5: prove reference-dependent/address-taken storage remains
  high-level through `--sec-lower-trivial-core`; reject MemRef conversion while
  a Place/reference dependency exists, without changing ordinary P5 storage.

  Completed 2026-10-17:
  - `sec::isReferenceDependentStorage` (SecOps.h) is the one authoritative query of lowering v11 § 18: storage counts as reference-dependent when its declaration carries `sec.address_taken = true` or a `sec.place.storage` consumes it.
  - `--sec-lower-trivial-core` keeps declare/init/load/store of such storage as `sec.storage`. `--sec-resolve-scalar-layout` converts only the element type and keeps reference-dependent bool storage as `!sec.storage<i1>` instead of `memref<i8>`.
  - Ordinary storage keeps its P5 `memref<T>` and P6 `memref<i8>` lowering.
  - The marker is valid only on `sec.storage.declare`, so a MemRef can never carry it.
  - A `sec.call.direct` with Place/reference operands or results stays a Sec call with its argument actions and `sec.reference` facts. P6 call lowering keeps every discardable attribute (MD-087).
  - lit: `Conversion/SecToCore/schema11-reference-storage.mlir` (trivial-core and the scalar pipeline, then the three verifiers) and `invalid-schema11-lowered-reference-storage.mlir`.
  - Go: `TestPackage15EmitsSchema11PlacesAndReferences` runs every emitted fixture through `--sec-lower-scalar-core` plus the verifiers on 32/64-bit plans and asserts that no address-taken storage becomes `memref.alloca`.
- [x] P15-60 — P6/P8: prove target scalar resolution recursively converts only
  referents under `!sec.place`, `!sec.ref`, and `!sec.ref_mut` on 32- and
  64-bit plans, while checked-integer signless normalization does not recurse
  through those wrappers or lower P15 operations.

  Completed 2026-10-17:
  - `--sec-resolve-scalar-layout` converts `!sec.place<T,a>`, `!sec.ref<T>` and `!sec.ref_mut<T>` by converting only `T` (recursively through structs, arrays, unions and Results), keeping wrapper and authority.
  - All schema-11 Place/reference operations take part unchanged apart from their types, with properties and discardable facts kept. Function `sec.reference` argument attributes are kept.
  - `--sec-lower-checked-integers` rejects a module with Place/reference types or operations before any rewrite, so it never recurses through the wrappers or lowers P15 operations (MD-087).
  - lit: `Conversion/SecToCore/schema11-reference-scalar-layout.mlir` checks a nested struct/array referent through deref, field, bounds, element, reborrow, end_borrow, read and write. It resolves to `si64`/`ui64` and `si32`/`ui32` with identical metadata, verifies under the three passes, and is rejected by `--sec-lower-integer-core` and by `--sec-lower-checked-integers` after resolution.
  - Go: the emitter fixtures pass through the scalar pipeline on both plans with no `!sec.int`/`!sec.uint` left and are rejected by `--sec-lower-integer-core`.
- [x] P15-61 — P13: test shared and mutable struct-field references, two
  disjoint mutable field borrows, whole-struct versus mutable-field conflict,
  nested subfield reborrow, property rejection, and `int128`/`uint256` field
  referents without an aggregate copy.

  Completed 2026-10-16. The tests found and fixed four defects:
  - Sema rejected two disjoint mutable field borrows (`ref mut inner.count` and then `ref mut inner.limit`) as a read of `inner`, because the base `inner` of the longer path was checked as a whole-value read. Now only the direct base of a Place path skips the owner-read check (`isPlaceBase`); the full path is still checked, and index expressions and arguments inside the base still count as reads.
  - Borrow liveness counted a member name (`inner.count`) or a struct-literal field name as a use of a holder with the same name, which kept such borrows live too long.
  - A nested sub-field reborrow through a reference holder (`ref mut outer.inner.count` with `outer: ref mut Outer`) was unsupported in Semantic IR. It now dereferences the holder once, projects the fields and derives the reborrow from that sub-Place (§ 78: a reborrow input may be a derived subplace). The verifier accepts a Place source only when it comes from `ref.deref-place` of the reference named by `SourceBorrow`, and requires a read/write sub-Place for `ref.reborrow-mut`.
  - Sema gave that sub-Place reborrow `local`/`lexical-borrow` facts without a root identity, because a reference parameter's origin Place lacked its root kind and identity. It now has the same `parameter`/`caller-owned-borrow` facts as the parameter.

  Property borrows now carry a diagnostic ID: S1168 `borrowing.non-addressable-place`, with help text (confirmed by MD-064, 2026-10-16). Whole-value read/write through a reference has no source spelling yet (MD-063), so these tests use struct referents. Tests: `TestPackage15FieldAndElementBorrowIntegration`, `TestPackage15VerifierChecksSubplaceReborrows`, `TestDisjointFieldAndElementBorrowsAreAccepted`, `TestOverlappingFieldAndElementBorrowsAreRejected`.
- [x] P15-62 — P14: test shared/mutable constant element references, distinct
  versus equal index conflict, conservative dynamic-index overlap, exactly-once
  dynamic bounds validation, proven no-branch element borrowing, `int128` /
  `uint256` index proof metadata, and zero-length rejection.

  Completed 2026-10-16. Two distinct constant mutable element borrows coexist without a bounds branch. An equal constant index and two dynamic indexes conflict (S1159). `int128` and `uint256` dynamic indexes are each checked exactly once, through one `place.array-index-in-bounds` that guards a runtime-check element Place. For zero-length arrays (P14 § 86): a constant index is rejected (S1134), and a dynamic index is never proven safe, so it always gets a runtime bounds check that ends in `fail.bounds`. No borrow copies or loads its array (`TestPackage15FieldAndElementBorrowIntegration`, `TestOverlappingFieldAndElementBorrowsAreRejected`, `TestDisjointFieldAndElementBorrowsAreAccepted`).
- [x] P15-63 — P11/P12: test union single-payload and Result `Ok`/`Err`
  ref/ref-mut bindings where source rules permit, active-variant guards,
  borrowed guards, branch-end lifetime markers, and all scoped escape
  rejections. Do not weaken union guard verification or introduce a copy path.

  Completed 2026-10-16: union, Option and Result `Ok`/`Err` `ref`/`ref mut` bindings, their active-variant guards and the no-copy property are covered by the P15-45 tests. Branch-end lifetime markers are new: Semantic IR now ends every arm-scoped payload borrow with `ref.end-borrow`, which carries the borrow's own BorrowID and LifetimeID (§§ 49, 50, 82). The marker is placed on every edge that leaves the arm: before a return, on the fall-through to the match merge, on a `break`/`continue` out of the arm, and on the false edge of a guard that reads the borrowed payload, which gets its own edge block because the next pattern test is shared. The binding cannot outlive its arm, so the arm exit bounds Sema's resolved liveness; whether that conservative placement conforms is MD-065. The verifier now requires those facts on `ref.end-borrow`, checks them against the ended reference, and stops following a path at the reference's redefinition, so an arm inside a loop verifies (`TestPackage15PayloadBorrowsEndOnEveryArmExit`, `TestPackage15VerifierChecksPayloadBorrowEnds`). Escape rejections: return of `Ok`/`Err` payloads (directly or in an aggregate), storing a `ref mut` payload into an outer Place, a match expression producing the reference (which previously had no diagnostic ID and is now S1130) and a returned capturing lambda are each rejected with one identified diagnostic. A match whose arms all return no longer produces a follow-on "must return" error when one arm reports an error (`TestBranchScopedPayloadReferenceEscapesAreRejected`). Other borrows (`let r := ref x`, call-site borrows) do not emit end markers yet; that belongs with P15-49's borrow-end coverage.
- [x] P15-64 — Prove calls and returns: ref/ref-mut parameters, implicit borrow
  of owned Places, shared direct-call copy, mutable move/reborrow according to
  resolved mode, returned input/subfield references, local return rejection,
  and deterministic instantiated origin summaries.

  Completed 2026-10-16. Parameters, implicit borrows, shared copy and mutable reborrow passing are covered by the P15-47 tests (`TestPackage15LowersReferenceParametersAndPassing`). Returned input and subfield references, receiver-field returns, element paths, call-result returns and caller-side instantiated facts are covered, and two builds print identically (`TestPackage15LowersReturnedReferences`). Local return rejection holds in two places: Sema reports S1128 for local-origin returns (`TestEscapeDiagnostics`), and the verifier rejects a returned reference that does not trace to a listed parameter origin (`TestPackage15VerifierChecksReturnedReferenceOrigins`). The liveness of a holder of a call-returned reference now ends at its last use, because its origin comes from the callee summary (`TestCallReturnedReferenceHolderEndsAtLastUse`; confirmed by MD-069, 2026-10-16).
- [ ] P15-65 — Add end-to-end source → target-aware Sema → verified Semantic IR
  → schema-11 → `sec-mlir-opt` tests on 32- and 64-bit CompilationPlans using
  an absolute tool path. Include P13/P14/P12 integration, proven and dynamic
  validity, failure CFG, wrapper preservation, and all three verifier passes.
- [x] P15-66 — Assert successful P15 modules contain no physical offset/stride,
  GEP, pointer arithmetic, MemRef/LLVM reference form, raw-pointer collapse,
  address-plus-epoch aggregate, side-table/capability choice, hidden ABI args,
  runtime borrow counter, global generation manager, hidden allocation,
  `undef`, or poison.

  Completed 2026-10-16 at the Semantic IR level (the schema-11 MLIR part follows with P15-57/P15-58). All eight successful P15 fixtures (payloads, borrow ends, field/element borrows, parameters, reference uses, returned references) build and verify. Their operations come from the closed semantic set, no reference collapses into a raw pointer type, every reference value is classified shared or mutable reference, and no function has a synthesized parameter. The printed module contains none of the forbidden physical words. A reflective check confirms that `ReferenceMetadata` and `PlaceMetadata` have no offset, stride, pointer, address (other than address space and stability), counter, table, capability or ABI field (`TestPackage15ModulesHaveNoPhysicalVocabulary`).
- [x] P15-67 — Add explicit no-partial-module rejection tests for stable/weak
  handle syntax and resolution, slice values and array-to-slice borrow,
  RawPtr conversions, physical pinning/epoch/collection invalidation lowering,
  concurrent invalidation, move-only/semantic-copy reference reads, non-trivial
  write/destruction, physical reference ABI/layout, and LLVM lowering.

  Completed 2026-10-16. The following are each rejected as a whole package-15 boundary with no module: slice borrows, slice reference parameters, references into dynamic arrays (collection invalidation), foreign reference parameters (physical reference ABI), move-only referents with custom free (non-trivial read, write and destruction) and static roots (`TestPackage15RejectsOutOfScopeReferencesWithoutPartialModule`). Every emit route (`emit-ir`, `emit-sec-mlir`, `emit-llvm`, `emit-mlir`) rejects a valid reference program, prints nothing and leaves an existing output file unchanged (`TestReferenceProgramsAreRejectedByEveryEmitRoute`).

  Not testable from source: stable/weak handles and pinning have no Sec 0.1 source form by decision (reference_model.md §§ 3(3), 10(9), 26(5); storage.md § 24(4)). RawPtr↔reference conversion has no spelling yet (MD-070). Concurrent invalidation has no direct-reference source construct in P15. Sec 0.1 has no semantic-copy type: `CopySemantic` is never produced, and `string` is copy-trivial per copy_move.md § 4.5, so reads of it through references are correctly accepted.

  Known hole (verified 2026-10-16): the legacy `emit-llvm` route rejects reference programs with misleading messages ("emit-llvm only supports members on string for now", "does not support expression *ast.RefExpression yet") instead of naming reference lowering.
- [x] P15-68 — Add determinism tests rebuilding Place/reference facts and
  source modules with fresh maps and permuted source metadata. Assert stable
  root/borrow/lifetime IDs under the documented deterministic allocator,
  origin ordering, big-int printing, diagnostics, and independent 32/64 output
  without mutating shared Semantic IR.

  Completed 2026-10-16. For every P15 fixture:
  - A rebuild from a fresh parse and analyzer prints identically, including root, borrow and lifetime IDs and origin order.
  - Two builds from one analyzer are identical, and the second build does not mutate the first module.
  - Renaming the source file changes only the locations.
  - The 64-bit (x86_64) and 32-bit (armv7) target plans build separately verified modules whose Place/reference operations, identities and facts are the same (`TestPackage15PlaceReferenceFactsAreDeterministic`).

  Big-integer printing (wide constant indexes above uint64) is covered by the deterministic printer test in `package15_test.go`, and diagnostic determinism by `TestEscapeDiagnostics` and the borrow diagnostic tests.

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
