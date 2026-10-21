# SEC-MLIR Package 17 — implementation TODO

Package: `SEC-MLIR-P17` — Ownership Transfer and Destruction Semantic Core

Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package17.md`

Package contract: `rules/mlir/packages/sec-mlir-dialect_package17.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_ownership_destruction_sync_v1.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_ownership_destruction_v1.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v13.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v13.md`

This is an implementation plan, not a completion report or a language
correction. It continues `package15-todo.md` and `package16-todo.md`; it does
not declare those predecessors complete. The package's `152c772` baseline is
historical context, not today's execution snapshot.

## Inventory snapshot — 2026-10-14

Audited HEAD:

```text
08d8b8fbf5ca87fe3157cec9d36feed2cc2d7cbd
```

The checkout contains unrelated tracked and untracked changes, including P15
work and newer semantic/lowering fixes. Record an exact execution snapshot
containing the intended changes before package acceptance; HEAD alone does not
reproduce this inventory. Do not resume from the older isolated P15 snapshot
without first reconciling its differences from the current checkout.

| Area | Observed implementation | P17 consequence |
|---|---|---|
| P13/P14 | Canonical struct/fixed-array values, identities, compact array plans and schema-9/10 emission exist. | Extend their actions and remove trivial-only gates only when transfer, destruction and cleanup are represented together. |
| P15 | Canonical Places, storage/domain/borrow/lifetime identities, detached reference plans and selected reference IR paths exist. The package is partial. | Reuse them; P17 must not introduce another Place model or borrow checker. |
| P16 | `package16-todo.md` exists; the canonical slice representation and schema-12 bridge remain predecessor work. | Slice integration is a dependency, not already implemented P17 infrastructure. |
| Sema classification | `CopyClassificationOf`, `TriviallyDestructible`, move/partial-Place state, call consumption, discard and defer dependencies exist. | Evolve the existing decisions into complete immutable use/destruction/cleanup plans. |
| Sema lifecycle | Custom `free` is parsed/analyzed; `CustomFree` makes a type non-trivially destructible. MD-025 records unresolved interactions. | Reconcile the older P17 source gate before changing acceptance or relying on copy classification. |
| Maintained Semantic IR | Ownership classes, aggregate classifications and selected transfer actions/reference operations exist; aggregate reads/writes still contain explicit non-trivial deferrals. | Metadata is useful infrastructure, not the P17 terminal-action/cleanup model. |
| P17 plans and operations | No `ResolvedTransferPlan`, `ResolvedDestructionPlan`, `ResolvedCleanupPlan`, `DestructionPlanID` or `CleanupActionID` definitions were found in the audited Sema/IR. No P17 `sec.own.*` / `sec.cleanup.*` operations or verifier implementations were found in C++. | Add the canonical representation and verification; do not mark renamed existing metadata as package completion. |
| Schema boundary | Go emission was schema 10 at this snapshot; since P15-50/51 (2026-10-16) it is schema 11 and C++ accepts schemas 1–11 with the Place/reference types. Consolidated dialect/lowering documents name version 11. | Finish the P15/P16 bridges before schema-13 acceptance. Installed versioned documents are not implemented schemas. |
| Rule synchronization | Current destruction/defer rules already describe unified registration order and binding capture. P17's detailed input documents exist. | Audit synchronization paragraph by paragraph and merge missing amendments; do not overwrite newer canonical rules with historical copies. |

Primary implementation anchors:

```text
internal/sema/types.go
internal/sema/place.go
internal/sema/place_identities.go
internal/sema/reference_plans.go
internal/sema/call_ownership.go
internal/sema/analyzer_match_ownership.go
internal/sema/analyzer_discard.go
internal/sema/analyzer_defer.go
internal/sema/semantic_facts.go
internal/ir/semantic/model.go
internal/ir/semantic/place_reference.go
internal/ir/semantic/reference_use.go
internal/ir/semantic/builder.go
internal/ir/semantic/verifier.go
internal/lowering/secmlir/emitter.go
mlir/include/sec/Dialect/Sec/SecOps.td
mlir/include/sec/Analysis/Passes.td
mlir/lib/Dialect/Sec/SecDialect.cpp
mlir/lib/Dialect/Sec/SecOps.cpp
mlir/lib/Analysis
mlir/lib/Conversion/SecToCore
```

### Evidence collected for this plan

The following focused command passed in the current dirty checkout. It checks
existing frontend/predecessor behavior; it is not P17 acceptance evidence:

```sh
export SEC_MLIR_BIN="$(pwd)/build/sec-mlir/bin"
go test ./internal/sema ./internal/ir/semantic ./internal/lowering/secmlir \
  -run 'Test(CopyClassification|Destruction|Defer|Discard|Package15)|TestEmitPackage13StructsEndToEnd|TestEmitPackage14ArraysEndToEnd' \
  -count=1
```

Source searches establish the P17 API/operation absences above. Full Go/vet,
`check-sec-mlir` and P17 source pipelines were not run as package acceptance.

## Current authority, decisions and predecessor gates

1. **Custom free — MD-025:** P17 §22, synchronization §17 and the package's
   acceptance wording assume a source rejection gate. Current
   `rules/declarations/impl.md` §19, destruction rules and frontend behavior
   accept custom `free`; the applied lifecycle correction is
   `rules/corrections/applied/destruction-init-lifecycle-correction-20260813.md`.
   MD-025 still leaves copyability, field discard, legal targets and other
   restrictions undecided. Do not restore blanket rejection from the older
   package text. Reconcile its acceptance criteria with current authority and
   resolve the affected decisions before claiming custom-free ownership support.
   Ordinary transfers/derived destruction can proceed independently; a custom
   hook alone does not settle this branch of package acceptance.
2. **Discard convergence:** Current `analyzer_discard.go` explicitly accepts
   discard of already unavailable Places when outstanding obligations permit it.
   Preserve current discard/ownership rules: redundant or conditional discard
   must be planned as an appropriate no-op/conditional terminal action, not
   turned into a second destruction or a new use-after-move error. Reconcile
   conditional availability with P17 §§70,114–115 before choosing merge rules.
3. **P15/P16 bridge:** Schema-11 Place/reference and schema-12 slice emission,
   dialect types, verifiers and integration evidence are required predecessors
   of schema 13. Sema planning and independent ownership IR work can begin
   first. Do not promote the default package/schema while these paths are absent.
4. **Reference model:** Reuse `AddressStability`, pin dependencies and canonical
   validity identities. Older relocation-class wording must not restore the
   withdrawn model. Explicit reference-holder reborrow is MD-061; P17 does not
   decide whether `ref r` refers to a holder or reborrows its referent.
5. **Try/Option metadata:** MD-031 constrains guarded/residual/Option handler
   emission. Preserve the existing supported Result path for initial cleanup
   tests. Complete the required P11/P12/Option integration only after the
   affected representation is resolved; do not hide unsupported handlers under
   generic cleanup success.
6. **Implementation boundaries:** Owning dynamic `T[]` representation and
   backing-storage release belong to P18. P17 must not infer an array/string/
   interface resource representation from today's legacy carrier or a generic
   template's classification. A concrete unsupported representation stays an
   explicit lowering limitation, not a declaration that destruction is trivial.

## Completion boundary

P17 represents the already-resolved ownership action and terminal responsibility
for each supported concrete value. It owns copy, infallible semantic copy, move,
Place transfers, initialization/reinitialization, transactional replacement,
direct struct/active-payload partial state, discard, destruction plans, cleanup
registration/cancellation, defer regions and normal-exit cleanup.

Its endpoint is verified high-level **schema 13**, with Semantic IR version 1.
It must not introduce source `move`/`copy`/`replace`/`take`/`swap` syntax, fallible
implicit copy, general array-element partial move, owning dynamic arrays,
physical allocator/destructor/cleanup ABI, mandatory runtime ownership tables,
panic unwinding or LLVM ownership/destruction lowering.

Reuse P11/P12 variant identities and proofs, P13 stored-field identities, P14
exact lengths and index plans, P15 Places/borrows and P16 slice ranges. Shared
references/slices are CopyTrivial; mutable references/slices are MoveOnly.
Destroying/discarding a reference holder never destroys its referent/elements.

All new semantic functions need responsibility and exact rulebook references
per `codex.md`. Extract cohesive ownership/destruction/cleanup work into focused
files instead of enlarging `analyzer.go`, `builder.go` or `emitter.go`. Keep
fixtures under `testdata/`; no unrelated core/stdlib/manual rewrite belongs here.

## A. Establish the execution baseline and synchronized authority

- [ ] P17-01 — Record the actual HEAD, tree/snapshot, dirty changes included,
  tool versions and predecessor evidence. Establish a reproducible execution
  baseline without discarding unrelated work or relying on P15's older snapshot.
- [ ] P17-02 — Run and record P13/P14/P15/P16 regressions against that baseline;
  distinguish completed predecessor scope, missing bridges and skipped tools.
  Keep P15/P16 implementation/evidence owned by their respective plans.
- [ ] P17-03 — Compare the ownership/destruction sync and IR amendment with
  current ownership, copy/move, destruction, defer, discard and Semantic IR
  rules. Merge only missing/conflicting paragraphs through the normal correction
  workflow; preserve newer rules and update `language-rulebook-status.md`.
- [x] P17-04 — Reconcile MD-025 and the stale custom-free acceptance gate;
  document the exact resolved scope and remaining restrictions. Update the
  existing decision entry rather than creating a competing decision or policy.

  Done 2026-10-20: MD-025 was applied 2026-10-15 (custom `free` accepted, MoveOnly, runs before
  remaining field destruction, no field move-out of `self`); P17 § 22's rejection gate is marked
  superseded in the package book. Remaining MD-025 work belongs to its own owners.

- [ ] P17-05 — Synchronize schema/lowering v13 after the v11/v12 predecessor
  contracts, resolving historical path/attribute spelling and MD-031/MD-061
  intersections. A supplied document's existence is not synchronization evidence.
- [ ] P17-06 — Audit existing ownership/type/Place/call/discard/defer state and
  publish one migration map. Decide focused file ownership and stable identities;
  do not add a parallel move table, borrow checker or cleanup engine.
- [ ] P17-07 — Establish a package-17 capability boundary and one canonical
  package governance entry when implementation starts. Link existing
  `semantic-ir.copy-move`, `lowering.copy-move`, `semantic-ir.destruction`,
  `lowering.destruction`, discard/defer and P15/P16 owners; avoid duplicate claims.

## B. Publish complete immutable transfer and destruction plans

- [x] P17-08 — Audit concrete copy classification and map existing CopyTrivial,
  CopySemantic, MoveOnly, Conditional and NonCopyable facts into valid transfer
  actions. Do not treat conditional/unresolved classification as trivial copy.

  Done 2026-10-20: `ResolveTransferAction` (`internal/sema/destruction_plans.go`) maps the five
  classifications to copy-trivial / copy-semantic / move / rejected; MoveOnly and @noCopy need an
  explicit move, conditional is never a trivial copy (`TestTransferActionsFollowCopyClassification`).

- [ ] P17-09 — Permit implicit semantic copy only with a resolved infallible
  copy plan. Fallible duplication remains an ordinary explicit Result-returning
  call; pure move adds no copy/allocator failure channel (P17 §§5–7).
- [x] P17-10 — Resolve destruction plans for every supported non-trivial
  concrete type. Cover stored struct fields, compact fixed arrays, active union/
  Result/Option payloads, compiler-known resources and the resolved custom-free
  hook. Keep generic-template and unsupported representation boundaries explicit.

  Done 2026-10-20 for the supported kinds: `ResolveDestructionPlans` covers derived structs
  (custom `free` first, then non-trivial fields in reverse declaration order), compact fixed
  arrays, active union/Option/Result payloads (`VariantFamily`) and compiler-known resources;
  owning dynamic arrays (P18), generic templates and `any` are named unsupported boundaries.

- [x] P17-11 — Allocate deterministic `DestructionPlanID` identities, including
  recursive plan references where legal. Preserve concrete type identity,
  release authority and source provenance without field offsets or physical ABI.

  Done 2026-10-20: `DestructionPlanID` = `destroy:` + canonical type identity; nested and
  recursive references share IDs in a self-contained `DestructionPlanSet`; no offsets or ABI.

- [ ] P17-12 — Implement `ResolvedTransferPlan` or an equivalent consolidated
  model containing action, source value/Place, destination, partial path and
  before/after ownership state. Retain the existing canonical Place identities.

  Partial 2026-10-20: `ResolvedTransferPlan` (`internal/sema/transfer_plans.go`) records site,
  action, cloned source Place and partial flag for every transfer accepted by
  `validateNamedOwnershipSource`. Missing: destination Place and before/after ownership state.

- [ ] P17-13 — Publish transfer plans for initialization, assignment, argument,
  return, constructor field/element/payload, match binding and branch use sites.
  Distinguish non-consuming observation and borrowing from ownership transfer.

  Partial 2026-10-20: initialization, assignment, explicit-move argument, return, construction
  (`<-` in aggregate/payload) and lambda move captures publish plans. Missing: implicit argument
  copies, match bindings and branch uses.

- [x] P17-14 — Provide detached read-only transfer/destruction queries. Prove
  repeated queries neither infer types nor mutate borrow/move state; clone nested
  lists, masks, exact lengths and Places so consumers cannot corrupt facts.

  Done 2026-10-20: `ResolveDestructionPlans` is pure and returns detached sets; transfer plans
  are cloned on every query (`TestDestructionPlanQueriesAreDetached`,
  `TestTransferPlansRecordSiteActionAndSource`).

- [ ] P17-15 — Preserve uninitialized/initialized/moved/destroyed/partial state
  per path and per supported field/active payload. Reinitialization restores only
  the assigned state; whole-value use requires valid state. Reject deferred array
  element move-out and unresolved custom-free partial actions explicitly.
- [ ] P17-16 — Resolve destruction order and partial construction facts:
  complete structs destroy remaining stored fields in reverse declaration order;
  fixed arrays use reverse index order; active variants destroy only initialized,
  unmoved payloads. Failed construction cleans successful obligations without
  invoking completed-value custom free or manufacturing a completed outer value.

  Partial 2026-10-20: reverse field and reverse index order and active-payload-only
  destruction are in the plans; failed/partial construction cleanup is not.

- [ ] P17-17 — Add Sema tests for classifications, detached queries, consuming
  calls/returns, whole and partial move/reinit, copyable/move-only references and
  slices, destruction completeness, custom-free decisions and forbidden actions.


  Partial 2026-10-20: `destruction_plans_test.go` and `transfer_plans_test.go` cover
  classifications, detached queries, struct/array/variant/resource plans, the P18 boundary and
  whole/partial moves; consuming-call returns, reinit and forbidden actions remain.

## C. Resolve one cleanup registration and exit model in Sema

- [x] P17-18 — Add deterministic `CleanupActionID`, `CleanupScopeID` and
  `DeferID`, tied to semantic binding incarnations rather than source names.

  Done 2026-10-21: `internal/sema/cleanup_identities.go` interns typed identities
  per analysis, reserving zero for absence. Owned-action keys contain the canonical
  BindingID and initialization site; reinitialization gets a different action.
  Defer actions share that action namespace and retain their lexical scope.
  Source block/defer analysis publishes read-only scope and validated defer-site
  queries. `cleanup_identities_test.go` covers incarnation/site separation,
  deterministic reanalysis, unknown queries and rejected defer bodies, using
  `testdata/sema/cleanup_identities.sec` and its invalid companion.
  These are semantic site identities: repeated executions in a loop remain distinct
  dynamic registrations. Owned-action registration/cancellation, exit plans
  and runtime instance multiplicity remain P17-19/P17-21..24; capture plans are
  completed under P17-20 below. No cleanup IR or execution
  support is claimed here.
- [ ] P17-19 — Record destruction registration only after successful non-trivial
  initialization. Transfer/discard/early destruction cancels the old obligation;
  reinitialization and replacement create a new registration at their own point.

  Partial 2026-10-21: `internal/sema/cleanup_registrations.go` publishes detached
  `ResolvedOwnedCleanupInitializationOf` facts after accepted local declaration
  initialization, with canonical binding/Place identity, scope, invocation,
  cleanup action and the complete destruction plan graph. Activation is explicitly
  after successful initialization, including the success continuation of `try`;
  failed type/ownership checks publish no completed-value obligation. Trivial and
  borrowed locals are excluded, and unsupported destruction (including nested P18
  boundaries) remains represented explicitly in the plan graph. Tests cover
  inferred/typed declarations, custom free, fixed arrays, reverse-field aggregate
  destruction, fallible initialization, fresh destinations of explicit moves,
  rejected initializers, detached queries and analysis reset. These are source-site
  facts, not a runtime cleanup log or an active-owner map. Cancellation, partial
  transfer/discard, reinitialization/replacement and path-sensitive continuation
  through exit plans remain open; no cleanup IR or native execution is claimed.
- [x] P17-20 — Resolve defer capture Places/binding identities and required
  lifetime extension. Registration does not copy a value; execution observes the
  current value of the captured binding instance (P17 §§42–43).

  Done 2026-10-21: `internal/sema/defer_plans.go` publishes detached
  `ResolvedDeferPlanOf` facts from the existing canonical dependency producer:
  invocation/body identities, captured BindingIDs and precise Places, separate
  retained-reference-holder dependencies, and the cleanup action through whose
  execution each capture must remain valid. No value/index/range is snapshotted
  at registration. Nested types, exact indices and origin alternatives are
  detached at publication and query boundaries. Tests in `defer_plans_test.go`
  cover late assignment, field/index/parameter captures, same-spelled bindings
  in different invocations, local-only bodies, retained holder authority,
  overlapping invalidation versus disjoint sibling moves, rejected bodies,
  reproducible reanalysis and query purity. P17-19/P17-21..24 still own cleanup
  registration, dynamic multiplicity, exit plans and the consolidated cleanup API;
  this completion does not claim source-to-IR cleanup or native defer execution.
- [ ] P17-21 — Preserve conditional registration and one registration per
  executed loop defer, including distinct iteration bindings. A site ID or
  registration ordinal is not a replacement for dynamic instance multiplicity.
- [ ] P17-22 — Resolve normal scope/function exit edges and eligible automatic
  obligations. Block fallthrough, break, continue and branch exit run exited-scope
  automatic cleanup; function-scoped defer remains pending until function exit.
- [ ] P17-23 — Resolve protected return/error transfers before function cleanup.
  Preserve failure-before-replacement, construction temporaries and argument
  consumption; distinguish normal error propagation from non-unwinding panic.
- [ ] P17-24 — Publish detached `ResolvedCleanupPlan` queries containing scopes,
  registrations, defer bodies/captures, normal exits and protected values. Builders
  must not reconstruct cleanup by walking variable names or counting SSA uses.
- [ ] P17-25 — Test interleaved local/defer registration, replacement/reinit order,
  branches, loops, scope exits, captures observing later assignments, return/Err
  protection, discard convergence and existing defer-body restrictions.

## D. Add explicit ownership and cleanup Semantic IR

- [ ] P17-26 — Add CopyValue, SemanticCopyValue and MoveValue operations with
  resolved actions/plans, value identity and provenance. Copy keeps the source;
  move creates the ownership continuation and removes source responsibility.
- [ ] P17-27 — Add CopyFromPlace/MoveFromPlace through canonical P15 Places.
  Preserve holder/referent distinctions, partial-path state and borrow obligations.
- [ ] P17-28 — Add InitializePlace and ReplacePlace. Initialization/reinit has
  no old destruction; replacement consumes a fully prepared new value and owns
  old destruction plus old/new cleanup incarnation facts.
- [ ] P17-29 — Add DiscardValue/DestroyValue/DestroyPlace with plan identity and
  explicit/implicit/compiler provenance. Represent conditional/no-op discard
  without double destruction. Borrow-holder termination never frees referents.
- [ ] P17-30 — Represent derived destruction and exact remaining subobject state
  compactly. Large arrays must not expand one IR record per element. Keep custom
  free, remaining-field cleanup and semantic release authority distinct.
- [ ] P17-31 — Extend struct/array/union/Result/Option constructor actions to
  resolved move and infallible semantic copy. Spread remains copying; this package
  does not silently make a move-only spread source legal.
- [ ] P17-32 — Make argument, return and branch ownership actions explicit.
  Mutually exclusive successors may transfer the same current owner; merges
  select one continuation. Verify compatible partial/active-variant state and
  destruction plans without rejecting legal source convergence by approximation.
- [ ] P17-33 — Add cleanup track/cancel/defer-register/run-scope/run-function
  operations and explicit defer regions with capture-Place parameters. Preserve
  dynamic registrations without selecting a physical stack or global runtime.
- [ ] P17-34 — Extend the model, deterministic printer, effect/provenance facts
  and verifier. Print transfer, Place, destruction/cleanup/scope/defer identities;
  verify terminal actions and cleanup order without physical representation.

## E. Connect source lowering solely through resolved plans

- [ ] P17-35 — Migrate local, call, constructor, return and branch producers to
  consume immutable plans. Missing required facts produce structured unsupported
  lowering; do not fall back to fresh ownership inference in the builder.
- [ ] P17-36 — Lower direct stored-field partial move, field reinit and non-trivial
  field replacement through Places. Preserve the owner of untouched fields; do
  not replace one owned field by copying and rebuilding its whole struct.
- [ ] P17-37 — Lower move-only active union/Result/Option payload bindings only
  on the predecessor's proven variant path, updating container partial state.
  Keep borrowed bindings on P15 and respect MD-031 handler emission limits.
- [ ] P17-38 — Lower non-trivial whole fixed-array ownership and destruction.
  Extend resolved ordinary element actions; preserve spread copy rules and the
  unsupported partial element-move boundary, including runtime indexes.
- [ ] P17-39 — Prepare fallible RHS ownership before replacement. Failure leaves
  the destination and old cleanup intact; success terminates the old incarnation
  once, installs the new value and registers its obligation at that point.
- [ ] P17-40 — Lower explicit/implicit discard and temporary lifetime end through
  the same destruction plans as normal cleanup; honor must-use legality and
  non-consuming expression classification instead of dropping an SSA result.
- [ ] P17-41 — Lower executed defer sites into regions with binding/Place captures.
  Preserve function scope and iteration-instance identity; never copy captures or
  collapse dynamic registrations into one active boolean per source site.
- [ ] P17-42 — Insert planned scope cleanup before exited-scope edges, and function
  cleanup after return/Err transfer but before return. Keep panic/raw process-exit/
  proven-unreachable endpoints outside normal cleanup; no hidden unwinding.
- [ ] P17-43 — Remove predecessor trivial-only gates only for operations whose
  complete transfer, destruction and cleanup path now exists. Keep default
  package selection conservative until the emitter/verifier bridge is operational.
- [ ] P17-44 — Add source-to-IR tests for every new use/exit form, including
  old-owner cancellation, partial masks, transactional failures and repeated defer.
  Assert semantic operations/order, not merely parser/Sema success.

## F. Implement schema 13 emission and three independent verifiers

- [ ] P17-45 — Add schema-13 acceptance/version metadata after schema 11/12 works.
  Keep schema 1–12 regression inputs valid and Semantic IR version 1 unchanged.
- [ ] P17-46 — Define ownership actions, destruction-plan references, cleanup
  IDs/ordinals, discard origins and exit-kind metadata. Map Semantic IR
  `construct` to dialect `construct-direct` explicitly; no physical offsets or
  runtime/source ownership-token type belongs in these attributes.
- [ ] P17-47 — Add all ten `sec.own.*` operations from dialect-v13 §§4–13 with
  parse/print/type/plan/Place verification and appropriate semantic effects.
- [ ] P17-48 — Add `sec.cleanup.track_owned`, `cancel`, `defer_register`,
  `defer_yield`, `run_scope` and `run_function`. Validate region captures,
  terminators, permitted exit kinds and scope/function eligibility.
- [ ] P17-49 — Extend constructor/call/branch/return ownership attributes and
  predecessor action enums. Foreign consumption requires existing declared FFI
  ownership metadata; do not invent release or ownership contracts.
- [ ] P17-50 — Emit the exact resolved ownership/cleanup plans, regions and source
  provenance. Earlier scalar/trivial lowering must preserve address-taken and
  non-trivial storage and must not erase identities/actions before verification.
- [ ] P17-51 — Register the three analysis passes in TableGen, pass registration,
  CMake and the configured compiler verification path. Preserve previous guard/
  borrow/slice checks; verify ownership before destructive cleanup normalization.
- [ ] P17-52 — Implement `--sec-verify-ownership`: availability, legal copy/move,
  no mutable-reference copy, partial state, mutually exclusive transfers, compatible
  merges and one terminal responsibility for represented non-trivial ownership.
- [ ] P17-53 — Implement `--sec-verify-destruction-plans`: concrete completeness,
  field/array/active-payload order, resolved custom-free restrictions and applicable
  semantic allocator/release pairing. Reuse P15/P16 validity and borrow facts.
- [ ] P17-54 — Implement `--sec-verify-cleanups`: successful registration,
  cancellation, remaining-state cleanup, reverse registration order, defer
  multiplicity/capture lifetime, exit eligibility and protected return/Err values.

## G. Prove integration and capability boundaries

- [ ] P17-55 — Add positive and malformed-IR tests for every ownership/cleanup
  operation, attribute and defer region. Cover the negative matrices in P17
  §§130–131, including missing cleanup, wrong order, double terminal actions,
  moved/uninitialized destruction, invalid merges and defer on block exit.
- [ ] P17-56 — Prove P11–P16 integration: active variants and by-value match
  payloads, non-trivial structs/fields, whole fixed arrays, reference/slice transfer
  and discard without referent destruction. Keep each unresolved predecessor gap
  visible and exclude no required integration from the package completion claim.
- [ ] P17-57 — Test correctness without copy/move elision, DCE, RVO, SROA or
  mem2reg. Reject optimization that duplicates owners, loses cleanup, changes
  registration/destruction order or violates borrow/defer lifetime dependencies.
- [ ] P17-58 — Regress int128/int256/uint128/uint256/decimal128 and 32/64-bit
  scalar plans without changing builtin status or narrowing exact array lengths.
  Keep existing legacy paths operational; their defer lowering is not P17 proof.
- [ ] P17-59 — Run real source → Semantic IR → emitted schema-13 → all applicable
  Sec verifiers, for valid and invalid cases. Confirm dynamic arrays, partial
  array move, unresolved custom-free cases, interface representation and physical
  ownership lowering remain explicit boundaries. P17 does not promise native
  resource/destructor execution before a physical lowering package exists.

## H. Final acceptance, governance and handoff

- [ ] P17-60 — Expand the coverage map below into one evidence row per §136
  acceptance criterion, with implementation/test references and any unresolved
  authority conflict. A completed subtask is not completion of its entire family.
- [ ] P17-61 — On the recorded execution snapshot, run full Go/vet, predecessor
  source pipelines, all three ownership/cleanup verifiers, `check-sec-mlir`,
  diff/private-path checks and canonical governance validation. Configured missing
  tools or skipped required integration tests cannot count as acceptance success.
- [ ] P17-62 — Synchronize the package entry and linked memory/control-flow/
  predecessor governance according to actual scope. Mark P17 implemented only
  after every applicable criterion has evidence and authority conflicts are
  resolved; retain physical/P18 deferrals rather than closing unrelated items.
- [ ] P17-63 — Write the §137 implementation report addressing all 45 requested
  topics in order: exact snapshot/environment, files, plans/operations, algorithms,
  verifiers, integration, test commands/results, deviations and P18 handoff.
- [ ] P17-64 — Recheck report, unique ownership, rulebook status, negative cases,
  operational legacy paths and predecessor evidence before final handoff.

## Acceptance coverage map

This covers the families in P17 §136; P17-60 must expand it into individual
criterion/evidence rows. Nothing in this table is a completed acceptance claim.

| §136 criterion family | TODO ownership |
|---|---|
| Newer baseline equivalent to P13–P16; predecessor regressions | P17-01–02, 61, 64 |
| Ownership/defer/discard synchronization, IR amendment, schema/lowering v13 | P17-03–05, 45, 60 |
| Concrete copy/destruction classification and read-only plans | P17-08–17, 24 |
| Explicit trivial/semantic copy, move and Place transfers | P17-26–27, 35 |
| Initialization, replacement, partial struct move and field reinit | P17-15–16, 28, 36, 39 |
| Active payload moves and deferred array-element partial move | P17-37–38, 56, 59 |
| Explicit/implicit discard and non-trivial aggregate destruction | P17-10–11, 16, 29–30, 40 |
| Cleanup track/cancel and unified registration order | P17-18–19, 24–25, 33–34, 54 |
| Defer regions, binding capture and dynamic registration | P17-20–21, 33, 41, 48, 54 |
| Scope/function cleanup and protected return/error transfer | P17-22–23, 32, 42, 54 |
| No cleanup-on-panic or custom-free acceptance invention | P17-04, 23, 42, 55, 59 |
| Schema-v13 ops/attrs and all three registered verifiers | P17-45–54 |
| Constructor/call/branch/return ownership metadata | P17-13, 31–32, 49–50 |
| P11–P16 non-trivial integrations and wide builtins | P17-36–38, 56, 58 |
| No mandatory runtime, physical cleanup/destructor ABI or LLVM decisions | P17-30, 33, 46, 50, 57, 59 |
| check-sec-mlir, full Go gates and operational legacy paths | P17-02, 55, 58, 61, 64 |

## Recommended execution order and reproducible gates

1. P17-01–07: snapshot, current authority, decision reconciliation and predecessors.
2. P17-08–25: Sema transfer/destruction/cleanup plans and query tests.
3. P17-26–34: explicit Semantic IR and verification without a physical runtime.
4. P17-35–44: source lowering and predecessor ownership integration.
5. P17-45–54: schema-13 bridge and the three verifiers after schema 11/12 exists.
6. P17-55–64: integration, full acceptance evidence, report and P18 handoff.

Sema/IR steps may proceed while P15/P16 bridges are being completed, but their
absence must not be concealed by skipping slice/reference acceptance tests.
Resolve MD-025's relevant semantics before activating custom-free transfers;
resolve MD-031/MD-061 where required by the selected integration cases.

Set `SEC_MLIR_BIN` to the absolute Sec MLIR tool directory for the recorded
execution snapshot. These are final gates to run during implementation, not
commands whose results this planning document claims:

```sh
go test ./...
go vet ./...
go test ./internal/sema ./internal/ir/... ./internal/lowering/... ./cmd/compiler -count=1
cmake --build build/sec-mlir --target check-sec-mlir -j2
go run ./cmd/yamlstatus check
go run ./cmd/yamlstatus fmt -check governance/lowering.yaml governance/memory.yaml governance/control_flow.yaml
git diff --check
```

For a compiler-emitted P17 module after the predecessor bridge exists:

```sh
"${SEC_MLIR_BIN}/sec-mlir-opt" package17.mlir \
  --sec-verify-ownership --sec-verify-destruction-plans --sec-verify-cleanups \
  --verify-each -o /dev/null
```

Add predecessor borrow/slice/variant/checked-failure passes as applicable; the
three P17 verifiers do not replace them. Record exact tool versions and actual
test names once implemented. Package 18 must reuse the resulting transfer/
destruction plans, cleanup registration and P15/P16 invalidation identities for
owning dynamic arrays rather than introducing collection-specific ownership.
