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
5. **Try/Option metadata:** MD-031 constrained guarded/residual/Option handler
   emission until 2026-11-04 (decided as dialect v11.41.6). Preserve the existing supported Result path for initial cleanup
   tests. Complete the required P11/P12/Option integration only after the
   affected representation is resolved; do not hide unsupported handlers under
   generic cleanup success.
6. **Implementation boundaries:** Owning dynamic `T[]` representation and
   backing-storage release belong to P18. P17 must not infer an array/string/
   interface resource representation from today's legacy carrier or a generic
   template's classification. A concrete unsupported representation stays an
   explicit lowering limitation, not a declaration that destruction is trivial.

## P17-06 migration map — audited 2026-10-22

This map supersedes the historical inventory above for ownership implementation
locations. It records the current working checkout, including uncommitted P17
changes; it is not the reproducible acceptance snapshot required by P17-01.
The canonical owner remains `lowering.sec-mlir-package17`. This audit changes
neither language rules nor the supported source-to-binary surface.

Authority: P17 §§9–18 (identities and transfers), §§19–27 (destruction),
§§31–44 (replacement and cleanup), §§79–80 (cleanup queries), §135
(architecture); `rules/memory/copy_move.md` §§3,5,8–12;
`rules/memory/destruction.md` §6.1 and §§9–18;
`rules/control-flow/defer.md` §§9–14. P15 Places/reference facts and P16 slice
facts remain their respective authorities. The current corrections and
capability gates take precedence over historical package assumptions.

### Existing producers and their migration owners

Paths in the first table are relative to `internal/sema/`.

| Responsibility | Existing producer and published facts | Remaining work and owning task |
|---|---|---|
| Type classification | `types.go`: `CopyClassificationOf`, `TriviallyDestructible`; `destruction_plans.go`: `ResolveTransferAction`, `ResolveDestructionPlans`, `ResolvedDestructionPlanOf`. Destruction graphs retain unsupported concrete boundaries. | P17-09 must resolve an infallible semantic-copy operation before enabling that action. `CopySemantic` currently propagates through aggregate classification but has no leaf producer selecting an operation. A named duplication method is not such a producer. |
| Availability and partial moves | `place.go`: `unavailablePlace`, `markPlaceUnavailable`, `markPlaceAvailable`; `ownership_moves.go`: `markMoveSource`, `markResourceTransfer`. `analyzer.go` retains `assigned`, `moved`, `moveReasons`, `closedResources` and branch/loop snapshots and merges. | P17-15/16 must publish path/subobject state and failed-construction cleanup from these existing decisions. String-keyed availability and resource bookkeeping are current producer internals, not a complete cleanup plan to reconstruct in the builder. |
| Initialization and assignment | `ownership_transfer_context.go` wraps existing ownership validation and binds accepted destination Places; `analyzer_let_bindings.go` and `analyzer.go` commit the existing declaration/assignment state. `transfer_plans.go` publishes detached `ResolvedTransferPlanOf` and `ResolvedTransferPlans`. | P17-13 completes use-site coverage; P17-15/16 completes partial-state facts; P17-28/35/36/39 consumes them for initialization, replacement and projected stores. Preserve RHS preparation before old-value destruction. |
| Calls and constructions | `call_ownership.go` resolves ordinary argument copies and explicit moves against the selected parameter. `analyzer_inference.go` validates explicit construction moves. `semantic_facts.go` exposes `ResolvedConstructionOf`, `ResolvedUnionConstructionOf`, `ResolvedStructLiteralPlanOf` and `ResolvedArrayLiteralPlanOf`. | P17-13 completes construction sources and fresh-value/observation distinctions; P17-31/32/35 extends the existing aggregate/call plans. Preserve selected overloads, field identities, variant identities and exact array plans. |
| Returns, captures and branches | `return_ownership.go`: terminal return construction; `analyzer_lifetime.go`: lambda capture validation; `match_arm_bindings.go`: branch-local payload borrow/move and arm-result validation. `ResolvedMatchPlanOf` retains selected binding actions in `semantic_facts.go`. | P17-13 still needs copy-return/copy-capture and complete match-binding/branch-use publication. P17-23/32/37/42 then relates protected outgoing values to cleanup and exits. Do not recheck arms after losing their branch-local state. |
| Borrowing and observation | `analyzer_borrow_access.go`, `analyzer_lifetime.go` and `borrow_liveness.go` enforce existing borrow/defer obligations. `reference_plans.go` publishes `ResolvedPlaceOf`, `ResolvedReferencePlanOf`, `ResolvedReferenceUseOf` and lifetime ends; `slice_creation_plans.go`, `slice_index_plans.go`, `slice_length_plans.go` own slice facts. | P17-13 distinguishes these uses from owned transfers by consuming the existing P15/P16 facts. Ending a holder must never destroy its referent or slice elements. No second borrow checker belongs in P17. |
| Explicit discard and ownership ends | `analyzer_discard.go` validates terminal use, redundant no-op and conditional convergence. Existing accepted move, resource, receiver-consumption and arena-release producers publish through `cleanup_cancellations.go`: `ResolvedOwnedCleanupCancellationsAt`. | P17-19 must match binding/Place cancellation targets to the current registration on each executing path; partial cancellation must preserve siblings. P17-29/40 owns explicit terminal IR and temporary lifetime ends. |
| Owned registration and replacement | `cleanup_registrations.go`: `ResolvedOwnedCleanupInitializationOf`; `cleanup_assignments.go`: `ResolvedOwnedCleanupAssignmentOf`. Accepted non-trivial initialized locals get success-only obligations; whole tracked local assignments retain the declaration scope and a fresh action with distinct previous-availability states. | P17-19 still needs initially empty locals, parameters, projected producers and path-exact incarnation matching. P17-21..25 adds dynamic registrations, exits and the consolidated cleanup plan. Current facts do not implement those consumers. |
| Defer capture and lifetime | `analyzer_defer.go` checks bodies in isolated flow state and retains dependencies through existing `borrowRecord`s. `defer_plans.go`: `ResolvedDeferPlanOf`, including binding/Place uses, separate retained reference holders and invocation/body identities. | P17-21/24/25/33/41/42 must preserve one registration per execution, interleave owned cleanup and defer in one order, and select eligible scope/function exits. A capture plan does not copy a value or evaluate a deferred index early. |

### Identity decisions

Existing identities are reused at their current granularity. Deterministic
per-analysis allocation does not promise persistent IDs across different
programs, compiler versions or edited analysis snapshots. Query methods must
not allocate identities or infer again.

| Identity | Canonical owner / meaning | Migration constraint |
|---|---|---|
| `BindingID`, `CallableID` | `semantic_facts.go` and `call_graph_identity.go` / `call_graph.go`: declaration and selected callable identities. | Use declaration/callable identity rather than spelling; same-named locals in different functions remain distinct. |
| `PlaceRootID`, `Place`, storage/domain/epoch/borrow/lifetime/index/range IDs | `place.go`, `place_identities.go` and P15/P16 producers: canonical storage paths and validity dependencies. | Preserve projections, alternative origins, address stability and separate pin dependencies. A holder, referent and backing storage are distinct responsibilities. |
| `TransferDestinationID` | `transfer_destinations.go`: use-node, site kind and callable context, refined by destination Place or selected callable/argument position. | Same-coordinate synthesized use nodes stay distinct; the ID is a semantic continuation, not a physical address or loop execution. |
| `DestructionPlanID` | `destruction_plans.go`: canonical type identity and its derived destruction graph. | Type-level plan identity does not identify a particular initialized value or its remaining subobjects. |
| `CleanupScopeID` | `cleanup_identities.go`: parent scope plus lexical block site. | Lexical scope identity does not enumerate dynamic invocations. Preserve declaration lifetime when assignment occurs inside an inner block. |
| `CleanupActionID`, `DeferID` | `cleanup_identities.go`: binding plus initialization/assignment site, or lexical defer site; owned and defer actions share one action namespace. | Distinct sites have distinct IDs. Reexecuting a site still needs distinct dynamic registrations under P17-21; an ID alone is not the active incarnation or registration order. |
| `ValueID`, `StorageID`, `BlockID` | `internal/ir/semantic/model.go`: existing SSA, storage and CFG identities. | Retain explicit mappings from Sema binding/Place facts. No additional `OwnershipID` is required now: P17 §9 makes it optional. Do not add a source or runtime ownership-token type (§87). |

The cancellation producer deliberately records binding/Place targets rather
than guessing one earlier `CleanupActionID`. P17-19/24 must resolve the current
incarnation along CFG paths, including loop repetitions and joins. The current
initialized-to-moved transfer facts and four replacement availability labels
must not be presented as the complete per-path P17-15 state model.

### IR, schema and focused file ownership

| Layer | Current boundary | Chosen integration location |
|---|---|---|
| Semantic IR model | `internal/ir/semantic/model.go` has value ownership classes, aggregate action metadata and trivial `OpDiscardValue`; these do not represent all P17 terminal obligations. | P17-26..34 extends the existing model, printer and verifier. Keep ownership operation building/verification in focused sibling `ownership_values.go` and cleanup building/verification in sibling `cleanup.go` when implemented; these two files are planned, not present implementation. |
| Builder | `internal/ir/semantic/builder.go` still has predecessor trivial-only aggregate/call/branch gates. `discard.go` lowers trivial discard and slice-holder borrow end. `ownership_capabilities.go` consumes Sema facts to reject unavailable P17 operations before constructing IR. | P17-35..44 migrates producers incrementally to the canonical transfer/destruction/cleanup queries. Extract touched cohesive code from the builder into the focused siblings; remove an unsupported gate only when its complete operation and verifier exist. |
| MLIR emission | `internal/lowering/secmlir/emitter_options.go` selects schema 11/12 and reports schema 13 as structured unsupported; `errors.go` preserves the requested boundary. | P17-45..50 extends existing schema selection and emission, with cohesive ownership/cleanup emission in a sibling file. Preserve no-output-on-failure and do not downgrade schema 13. |
| MLIR dialect and analysis | `mlir/include/sec/Dialect/Sec/SecOps.td`, `mlir/lib/Dialect/Sec/SecOps.cpp` and `mlir/include/sec/Analysis/Passes.td` have no P17 `sec.own.*`, `sec.cleanup.*` or three P17 verifiers yet. | P17-46..54 extends the existing dialect and pass registry with focused ownership, destruction-plan and cleanup analysis implementations under `mlir/lib/Analysis/`. |
| Physical/runtime handoff | P17 targets verified high-level schema 13; it does not implement backing storage, destructor ABI or physical cleanup execution. | P18 reuses these plans and P15/P16 validity identities. The sole source-to-binary pipeline remains unchanged; no alternate backend or cleanup engine is introduced. |

Implementation sequence: finish producer coverage and partial-state facts
(P17-09/13/15/16), resolve registration incarnations and one unified cleanup
plan (P17-19/21..25), then add explicit IR and migrate producers
(P17-26..44), and finally schema operations/verifiers (P17-45..54).
Independent IR model work may proceed earlier, but it must keep unsupported
capability gates until the corresponding producers and consumers agree.

Existing evidence anchors are `internal/sema/transfer_destinations_test.go`,
`package17_ownership_matrix_test.go`, `cleanup_identities_test.go`,
`cleanup_transitions_test.go` and `defer_plans_test.go` (the latter four also
under `internal/sema/`), plus `ownership_capabilities_test.go` in both
`internal/ir/semantic/` and `internal/lowering/secmlir/`. These test detached
queries, identities, existing source decisions and named boundaries; they do
not establish dynamic cleanup execution or schema-13 acceptance.

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

- [x] P17-01 — Record the actual HEAD, tree/snapshot, dirty changes included,
  tool versions and predecessor evidence. Establish a reproducible execution
  baseline without discarding unrelated work or relying on P15's older snapshot.

  Done 2026-11-03: recorded under "P17-01 execution baseline" below.
- [x] P17-02 — Run and record P13/P14/P15/P16 regressions against that baseline;
  distinguish completed predecessor scope, missing bridges and skipped tools.
  Keep P15/P16 implementation/evidence owned by their respective plans.

  Done 2026-11-06 (Claude), on HEAD `e37b17ee` plus 140 uncommitted paths (this
  package and concurrent Codex work), Go 1.26.8, the same LLVM/MLIR as P17-01:
  - **Per-package focused runs** (`internal/ir/semantic` + `internal/lowering/secmlir`):
    - P13 (`Package13|Struct`): 18 pass;
    - P14 (`Package14|Array|FixedArray`): 43 pass;
    - P15 (`Package15|Reference|Place`): 67 pass;
    - P16 (`Slice|Package16`): 111 pass;
    - none failed or skipped.
  - **Full suites.** `go test ./internal/...` passes. Lit passes 218/218,
    including the schema 11/12 regression inputs.
  - **Missing bridges.** These remain with their owners and are not regressions:
    - P15/P16 contained-origin reads and holder rebinding (named boundaries in
      `lowering.sec-mlir-package17` remaining);
    - native lowering of P15/P16/P17 forms (package-14 native track).
  - **Not caused by predecessor work.** The `cmd/compiler` failures seen on this
    snapshot come from the `sec/core` drafts (`number_parse.sec`) and from
    concurrent diagnostic-explanation work (`TestPitfall*`, `TestPanicMentor*`),
    not from P13–P16. No tool was skipped: `SEC_MLIR_BIN` was set for every run.
- [x] P17-03 — Compare the ownership/destruction sync and IR amendment with
  current ownership, copy/move, destruction, defer, discard and Semantic IR
  rules. Merge only missing/conflicting paragraphs through the normal correction
  workflow; preserve newer rules and update `language-rulebook-status.md`.

  Audited 2026-11-03 in `rules/corrections/applied/p17-ownership-destruction-rules-synchronization-20261105.md`:
  every sync and amendment section mapped to its owner; B/C/E merged (defer.md § 13,
  semantic_ir.md §§ 35(6), 44(2), 44(2a)); A/D (replacement registration point and
  a defer reading the replaced owner) are MD-125. The audit found defer.md § 8 was
  violated (inner-scope owner destroyed before the defer reading it); that and
  loop storage read by a defer now fail closed.

  Done 2026-11-05: the owner decided MD-125 as option B (a replacement keeps the
  Place's existing cleanup registration and position; reinitialization registers
  anew). Applied in destruction.md §§ 6.1(4)–(5), 13.2(5)–(8), 13.3(5),
  semantic_ir.md § 40(7)–(10) and a defer.md § 9 clarification; the P17 sync § 1/§ 3
  and the Semantic IR amendment § 11 are marked superseded in part, and proposals
  A/D of the audit are superseded. **Source rules synchronized; implementation still
  partial:** inherited registration in Sema facts, producers and --sec-verify-cleanups
  is tracked in governance (`frontend.destruction`, `lowering.sec-mlir-package17`).
- [x] P17-04 — Reconcile MD-025 and the stale custom-free acceptance gate;
  document the exact resolved scope and remaining restrictions. Update the
  existing decision entry rather than creating a competing decision or policy.

  Done 2026-10-20: MD-025 was applied 2026-10-15 (custom `free` accepted, MoveOnly, runs before
  remaining field destruction, no field move-out of `self`); P17 § 22's rejection gate is marked
  superseded in the package book. Remaining MD-025 work belongs to its own owners.

- [x] P17-05 — Synchronize schema/lowering v13 after the v11/v12 predecessor
  contracts, resolving historical path/attribute spelling and MD-031/MD-061
  intersections. A supplied document's existence is not synchronization evidence.

  Done 2026-11-06 (Claude): the synchronization record. Each point is backed by
  emitted-and-verified code, not by the document alone.
  - **Spelling.** `rules/mlir/dialect-versions/sec_mlir_dialect_v13.md` §§ 10, 15
    and the new § 36A, `sec_mlir_lowering_v13.md` §§ 5–6 and the delegated-spelling
    table of `sec_mlir_dialect.md` now record what the emitter and verifiers
    implement:
    - MD-125 equal-identity replacement and `unless_active`;
    - deferred argument consumption and failed-construction cleanup;
    - the MD-126 string-view operations.

    Evidence: `TestSchema13InheritsReplacementRegistration`,
    `TestSchema13CleansFailedConstruction`, `TestSchema13EmitsStringViews`, lit
    `schema13-inherited-registration.mlir`, `schema13-string-views.mlir` and the
    invalid companions.
  - **MD-031.** The try-handler kinds `none`/`residual` and the
    `sec.try_handler_guarded`/`residual` flags are unchanged in schema 13.
    Handler bodies run the same scope/function cleanup as other blocks
    (`TestValueProducingHandlerBlockRunsScopeCleanup`, `HandlerReturn`). The
    only intersection still open is a guarded handler on an error that needs
    destruction, a named P17-37 boundary.
  - **MD-061.** Slice and string-view reborrows stay P16 operations. Their holders
    end with `sec.ref.end_borrow`, and a discarded holder is `sec.own.discard`
    without a plan (v13 § 31), so ownership never frees a referent.
  - **Paths and attributes.** Ownership paths are the compact field/payload
    strings of `skipped` (v13 § 13). Call/return/branch actions use
    `sec.argument_actions`, `sec.result_action`, `sec.return_action` and
    `sec.successor_actions` (v13 §§ 24–27), as emitted by
    `internal/lowering/secmlir/ownership.go`.
- [x] P17-06 — Audit existing ownership/type/Place/call/discard/defer state and
  publish one migration map. Decide focused file ownership and stable identities;
  do not add a parallel move table, borrow checker or cleanup engine.
  Completed 2026-10-22: see the migration map above for current producer/query
  owners, identity granularity, named gaps, focused future file ownership and
  the staged Sema → IR → schema migration. No runtime support is claimed.
- [x] P17-07 — Establish a package-17 capability boundary and one canonical
  package governance entry when implementation starts. Link existing
  `semantic-ir.copy-move`, `lowering.copy-move`, `semantic-ir.destruction`,
  `lowering.destruction`, discard/defer and P15/P16 owners; avoid duplicate claims.

  Done 2026-10-22: `internal/ir/semantic/ownership_capabilities.go` preflights
  explicit P17-or-later requests against canonical Sema facts of the requested
  module before constructing IR. Missing owned-value initialization/transfer,
  replacement, termination and defer cleanup operations have named capability
  errors, with source locations and no partial module. Existing scalar/reference/
  slice routes remain available; package selection alone does not enable absent
  ownership/cleanup operations. Explicit schema 13 is a structured unsupported
  emission boundary naming schema 13 and returning no bytes. Default schema 11,
  explicit schema 12 and earlier package errors retain their existing behavior.
  The two `ownership_capabilities_test.go` files cover predecessor equality,
  repeated read-only preflight, module isolation, source diagnostics, and 32/64-bit
  schema rejection without downgrade or mutation. Native build/run also passes.
  The canonical owner remains `lowering.sec-mlir-package17` in
  `governance/lowering.yaml`, alongside predecessor owners
  `lowering.sec-mlir-package15` / `lowering.sec-mlir-package16`. Shared source/IR
  responsibilities remain with `semantic-ir.copy-move`, `lowering.copy-move`,
  `semantic-ir.destruction`, `lowering.destruction` in `governance/memory.yaml`
  and `frontend.discard-v2`, `frontend.defer-v2`, `sema.defer-delayed-uses` in
  `governance/control_flow.yaml`. This closes the capability/ownership boundary
  task, not the unimplemented schema-13 operations or cleanup execution.

## B. Publish complete immutable transfer and destruction plans

- [x] P17-08 — Audit concrete copy classification and map existing CopyTrivial,
  CopySemantic, MoveOnly, Conditional and NonCopyable facts into valid transfer
  actions. Do not treat conditional/unresolved classification as trivial copy.

  Done 2026-10-20: `ResolveTransferAction` (`internal/sema/destruction_plans.go`) maps the five
  classifications to copy-trivial / copy-semantic / move / rejected; MoveOnly and @noCopy need an
  explicit move, conditional is never a trivial copy (`TestTransferActionsFollowCopyClassification`).

- [x] P17-09 — Permit implicit semantic copy only with a resolved infallible
  copy plan. Fallible duplication remains an ordinary explicit Result-returning
  call; pure move adds no copy/allocator failure channel (P17 §§5–7).

  Done 2026-11-06 (Claude). The requirement holds by construction:
  - **No implicit semantic copy.** No Sec 0.1 type is semantically copyable.
    copy_move.md § 3.3(2) recognizes no user `Copy`/`Clone` method as an implicit
    copy, and `copyClassificationOf` gives no base type `CopySemantic`; it only
    propagates from parts that are.
  - **Gates.** Semantic IR accepts `own.semantic-copy` /
    `own.copy-from-place mode=semantic-infallible` only with a named infallible
    plan (P17-26). A Sema `copy-semantic-infallible` transfer plan reaching
    lowering is rejected by the P17 capability preflight
    (`ownership_capabilities.go`).
  - **Explicit duplication and moves.** Fallible duplication stays an ordinary
    Result-returning call. A move never adds a copy or allocator failure channel:
    `own.move`/`own.move-from-place` have no failure edge.
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

- [x] P17-12 — Implement `ResolvedTransferPlan` or an equivalent consolidated
  model containing action, source value/Place, destination, partial path and
  before/after ownership state. Retain the existing canonical Place identities.

  Done 2026-10-22: `ResolvedTransferPlan` retains its selected action, detached
  source Place and partial path, plus a typed owning destination continuation and
  the exact source's before/after state at transfer commit. A move transitions
  initialized to moved; copy preserves initialized availability. Destination IDs
  are allocated per semantic use, separating even synthetic nodes with identical
  token coordinates. Local initialization/assignment retain canonical target
  Places; argument destinations retain the selected callable, parameter token and
  argument position; return, construction and capture continuations retain their
  kind, identity and enclosing callable. No physical storage or dynamic loop
  instance is inferred from these IDs. `ownership_transfer_context.go` extracts
  declaration/assignment validation from `analyzer.go`; `transfer_destinations.go`
  owns destination production/cloning. Both query APIs deeply detach destination
  and source data, with deterministic ordering for equal source coordinates.
  `transfer_destinations_test.go` and `testdata/sema/transfer_destinations.sec`
  cover every currently produced site kind, exact field targets, state transitions,
  selected callees, mutation isolation and repeatable analysis. These are resolved
  commit plans, not premature commitment of a partly prepared call. Complete
  producer coverage remains P17-13, aggregate availability/masks P17-15 and
  Semantic IR ownership operations P17-26 onward.

- [x] P17-13 — Publish transfer plans for initialization, assignment, argument,
  return, constructor field/element/payload, match binding and branch use sites.
  Distinguish non-consuming observation and borrowing from ownership transfer.

  Partial, refreshed 2026-10-22: initialization, assignment, ordinary owning
  argument copies, explicit-move arguments, supported return/construction moves
  and lambda move captures publish plans. Their destination/state model is complete
  under P17-12. Remaining return/construction/capture producers, match bindings,
  branch uses and complete observation/borrow distinctions remain open.

  Done 2026-11-07 (Claude):
  - **Copies.** Accepted copies of a reusable value now publish plans at every
    remaining site: return slots, constructed struct fields, union/Option/Result
    payloads, closure environments (`capture(x)`) and match arm results.
  - **Branch uses.** A match arm result in a value context is site `branch`,
    including an explicit `<-h` arm value that was validated as a construction
    operand.
  - **Pattern bindings.** By-value payload, named-field, `if ... is Some(x)` and
    try-handler bindings publish site `match-binding` plans, keyed by the bound
    identifier. The source is the payload or field Place (or a fresh value, marked
    `SourceFresh`, for a try handler's error); the destination is the binding's
    Place. A moved payload is a partial move of its subject.
  - **Observation and borrowing.** `ResolvedOwnershipUseOf` classifies a use as
    transfer, borrow-shared, borrow-mutable, reference or observation from
    published facts only; borrows and observations carry no transfer plan.
  - **Evidence.** `TestTransferPlansCoverEveryUseSite` (`internal/sema`); a
    mutation that drops copy publication fails it.

- [x] P17-14 — Provide detached read-only transfer/destruction queries. Prove
  repeated queries neither infer types nor mutate borrow/move state; clone nested
  lists, masks, exact lengths and Places so consumers cannot corrupt facts.

  Done 2026-10-20: `ResolveDestructionPlans` is pure and returns detached sets; transfer plans
  are cloned on every query (`TestDestructionPlanQueriesAreDetached`,
  `TestTransferPlansRecordSiteActionAndSource`).

- [x] P17-15 — Preserve uninitialized/initialized/moved/destroyed/partial state
  per path and per supported field/active payload. Reinitialization restores only
  the assigned state; whole-value use requires valid state. Reject deferred array
  element move-out and unresolved custom-free partial actions explicitly.

  Done 2026-11-06 (Claude), with evidence gathered for every listed form:
  - **Accepted per-path state.** A field moved on some paths and then reinitialized
    makes the owner whole again. Every field moved and reinitialized works the same
    way. Field moves inside loops, an active payload moved on one path and then
    replaced, and a nested field moved on some paths and read in place after
    reinitialization all lower. They pass `--sec-verify-ownership`/`--sec-verify-cleanups`
    in `TestSchema13PerPathPartialState` (32/64) over
    `testdata/semantic_ir/partial_state.sec`.
  - **Rejections.** `TestPartialStateRejections` over
    `testdata/sema/partial_state_invalid.sec` rejects:
    - array element move-out (explicit indexed extraction);
    - a partial move from a custom-free type (S1085);
    - whole use while a field is moved on some path (S1089);
    - use after a discard on some path (S1089);
    - whole use after reinitializing only one of two moved fields (S1090).
- [x] P17-16 — Resolve destruction order and partial construction facts:
  complete structs destroy remaining stored fields in reverse declaration order;
  fixed arrays use reverse index order; active variants destroy only initialized,
  unmoved payloads. Failed construction cleans successful obligations without
  invoking completed-value custom free or manufacturing a completed outer value.

  Partial 2026-10-20: reverse field and reverse index order and active-payload-only
  destruction are in the plans; failed/partial construction cleanup is not.

  Done 2026-11-06 (Claude), failed/partial construction:
  - **Prepared operands.** Struct fields, named union fields, array elements and
    fresh owned call arguments (calls, struct literals, `try` over a call) are
    recorded as prepared until their construction or call takes them
    (`prepareOperand`/`takePrepared`).
  - **Failure exits.** A `try` propagation, a handler arm's `return` and a handler
    arm's `break`/`continue` destroy the prepared operands they leave with
    `own.destroy-value cause=temporary-end`, in reverse evaluation order and
    before function or scope cleanup. The outer value is never built and no
    completed-value custom `free` runs. A moved owner passed as an argument keeps
    its cancellation at call commit (destruction.md § 9.2(3)).
  - **Arrays.** The former `partial non-trivial array construction cleanup
    (P17-16)` boundary is gone.
  - **Reads in place.** A copy-trivial field below a non-trivial field
    (`pair.left.id`) is read through a `place.field` chain of the binding, never
    by copying the owned subobject (the former `non-trivial struct field read`
    boundary for bindings).
  - **Evidence.** `testdata/semantic_ir/failed_construction.sec` through every
    cleanup/ownership verifier in `TestSchema13CleansFailedConstruction` (32/64),
    and `TestOwnedArrayPartialConstructionCleanup`.

- [x] P17-17 — Add Sema tests for classifications, detached queries, consuming
  calls/returns, whole and partial move/reinit, copyable/move-only references and
  slices, destruction completeness, custom-free decisions and forbidden actions.
  Done 2026-10-22: `package17_ownership_matrix_test.go` and the paired
  `testdata/sema/package17_ownership_matrix*.sec` fixtures complete the Sema gate
  alongside `destruction_plans_test.go`, `transfer_plans_test.go`,
  `copy_move_test.go`, `slice_aggregate_test.go`, custom-free tests and
  `cleanup_transitions_test.go`. The new source matrix verifies consuming
  call/return plans, whole and field move/reinit, shared/exclusive references and
  slices, no accepted plan for rejected copies/extractions, custom-free field
  restrictions and use after consumption. Destruction graph checks cover every
  child ID, active union/Option/Result payloads, non-owning holder destruction
  and compact exact fixed-array counts beyond 64 bits.
  Independent analysis snapshots exposed shared nested type data in transfer
  queries; `transfer_plans.go` now detaches typed Places at publication and in
  both query APIs. Mutation tests cover element/field types and ownership/borrow
  state. These tests establish the supported Sema plans, not the still-open
  semantic-copy producer (P17-09), complete transfer/cleanup models or later
  source-to-IR/schema-13/runtime integration.

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
- [x] P17-19 — Record destruction registration only after successful non-trivial
  initialization. Transfer/discard/early destruction cancels the old obligation;
  reinitialization and replacement create a new registration at their own point.
  Partial 2026-11-03: reassignment in a branch, nested block or loop lowers; a
  binding whose live incarnation depends on the path cancels every incarnation
  where active (`if_active`) at each later end.

  Update 2026-11-06 (Claude, MD-125): the IR producers consume Sema's
  `Assignment.Cleanup`. An available replacement keeps the live registration
  (`own.replace-place` with equal identities, no cancel or new track); a
  replacement after field moves reinitializes under the kept registration;
  reinitialization after a move registers `Cleanup.Register`; a conditional
  replacement registers only where nothing is live (`unless_active`, also over an
  earlier loop instance). Schema 13 and `--sec-verify-cleanups` /
  `--sec-verify-ownership` follow (`TestSchema13InheritsReplacementRegistration`,
  lit `schema13-inherited-registration.mlir`).

  Done 2026-11-06 (Claude):
  - **Projected and parameter producers.** Stored and nested fields are replaced
    under their owner's registration. An owned parameter replaced whole or
    through a field is held in storage from entry (`markPartialRoots` now marks
    assignment roots) and inherits its registration. A declaration without an
    initializer of a defaultable type starts with its default and is replaced in
    place. `TestSchema13RegistrationProducers` (32/64) covers these over
    `testdata/semantic_ir/registration_producers.sec`.
  - **Not in this item.** The only "initially empty" binding Sec has is
    `let mut x: U` of a union without a default. It holds U's implicit empty
    state, which Semantic IR does not represent yet; `match` over a possibly
    empty union is a boundary too. This is tracked as its own governance
    remaining item ("empty union state").

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
  facts, not a runtime cleanup log or an active-owner map. Cancellation,
  reinitialization/replacement and path-sensitive continuation remained open at
  that snapshot; the next increment is recorded below.

  Partial 2026-10-22: `cleanup_cancellations.go` records accepted ownership-end
  sites through the canonical move/discard/consumption producers, including
  precise subobject Places and conditional discard convergence. Repeated general
  and resource tracking of the same end publishes one cancellation. Borrow-holder
  termination creates no referent-destruction cancellation. `cleanup_assignments.go`
  publishes success-only whole-local replacement/reinitialization facts after RHS
  preparation and before destination availability is cleared. Each gets a fresh
  action and retains the declaration's scope/invocation, including inner-block
  assignment. Available, unavailable, conditional and partial previous states
  remain distinct; RHS consumption of the destination is observed before selecting
  reinitialization. `ownership_moves.go` extracts the touched move/resource-transfer
  producers from `analyzer.go` with rule traceability. `cleanup_transitions_test.go`
  covers these source transitions, fallible RHS, partial Places, rejected writes/
  self-moves/borrowed moves, detached queries and deterministic reanalysis.
  Cancellation targets the current binding/Place incarnation on the executing path;
  matching it to registrations through CFG joins, complete partial-state cleanup,
  projected stores and parameters, cleanup IR and execution remain open. P17-19
  therefore stays unchecked; P17-15/16/P17-21..24/P17-36 retain their separate work.
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
- [x] P17-21 — Preserve conditional registration and one registration per
  executed loop defer, including distinct iteration bindings. A site ID or
  registration ordinal is not a replacement for dynamic instance multiplicity.

  Done 2026-10-25 (Claude):
  - **Multiplicity.** Every registration site in `ResolvedCleanupPlan` carries `PerExecution` (inside a loop: one registration per execution, never collapsed into its site ID) and `Conditional` (below the invocation's root scope, so only the paths that execute it register). A loop-body binding is a fresh incarnation per iteration through its owned action.
  - **Evidence.** `testdata/sema/cleanup_plans.sec`, `TestResolvedCleanupPlans`: a loop `defer` and a loop-local owned value are per-execution and conditional; an `if`-local is conditional only.
- [x] P17-22 — Resolve normal scope/function exit edges and eligible automatic
  obligations. Block fallthrough, break, continue and branch exit run exited-scope
  automatic cleanup; function-scoped defer remains pending until function exit.

  Done 2026-10-25 (Claude), `cleanup_exits.go`:
  - **Exit edges.** Analysis records each exit edge with the scopes it leaves, innermost first:
    - a block that can fall through (`fallthrough`; a function exit when it is the invocation's root);
    - `break`/`continue` up to the innermost loop's entry scope;
    - `return`.
  - **Branch exits.** Match/switch arm bodies exit as block fallthrough.
  - **Function-scoped registrations.** Only exits with `FunctionExit` run them, so a defer never runs at its block's end.
  - **Defer bodies.** Exits inside defer bodies are not invocation exits.
  - **Dedup.** Recording is idempotent across loop re-analysis and the summary pass.
- [x] P17-23 — Resolve protected return/error transfers before function cleanup.
  Preserve failure-before-replacement, construction temporaries and argument
  consumption; distinguish normal error propagation from non-unwinding panic.

  Partial 2026-10-25 (Claude):
  - **Protected value.** A `return` protects its returned value (`ProtectedKind = return-value`), and a propagating `try` is a function exit of kind `error-propagation` that protects its Err (`ProtectedKind = error`), read from the resolved try kind.
  - **Panic.** `panic`/`unreachable` are `CleanupExitPanic` with `RunsCleanup = false` (§ 46).
  - **Failure before replacement.** It is kept by the success-only assignment facts (P17-19).
  - **Remaining.** Construction temporaries and argument consumption as protected/consumed values at an exit.
  - **Semantic IR (2026-10-26).** With owned locals, every propagating try exit
    (Result call, checked arithmetic — whose failure return sits at the
    operator and is mapped to the try's exit — bounds and failure sets) emits
    `cleanup.run-function exit=error-propagation` after the Err is built and
    before its return (`TestErrorPropagationRunsFunctionCleanup`).
  Done 2026-11-06 (Claude): the two remaining forms.
  - **Construction temporaries.** Operands already prepared for a construction
    or call are destroyed on the failing exit before function cleanup (P17-16).
  - **Argument consumption.** A moved owned argument (`<-h`, immutable or in
    storage) is moved out only when its call commits, together with its
    cancellation (destruction.md § 9.2(3)). A failing later argument therefore
    leaves the owner unmoved and registered, and function cleanup destroys it once.
  - **Evidence.** `MovedArgument`, `MovedStorageArgument`, `HandlerReturn` and the
    construction cases in `testdata/semantic_ir/failed_construction.sec`
    (`TestSchema13CleansFailedConstruction`, 32/64).
- [x] P17-24 — Publish detached `ResolvedCleanupPlan` queries containing scopes,
  registrations, defer bodies/captures, normal exits and protected values. Builders
  must not reconstruct cleanup by walking variable names or counting SSA uses.

  Done 2026-10-25 (Claude), `cleanup_plans.go`:
  - **Query.** `ResolvedCleanupPlanOf(function)` returns the invocation's scope tree, all registration sites in source order and all exit edges, with protected values. Registrations are owned initialization, reinitialization/replacement with the previous state, and defer with its detached capture plan.
  - **Purity.** It is read-only and detached (`TestResolvedCleanupPlansDetached`), and it consumes the existing registration/assignment/defer facts without new inference.
  - **IR alignment.** IR helpers now classify `string` as copy-trivial and trivially destructible, as Sema does (fixtures use a custom-free struct as the non-trivial type).
- [x] P17-25 — Test interleaved local/defer registration, replacement/reinit order,
  branches, loops, scope exits, captures observing later assignments, return/Err
  protection, discard convergence and existing defer-body restrictions.

  Done 2026-11-06 (Claude): `TestSchema13CleanupInterleaving` (32/64, every
  cleanup/ownership verifier) over `testdata/semantic_ir/cleanup_interleaving.sec`
  covers the following:
  - local/defer/local/defer/local registration order;
  - a replacement keeping its position before a defer while a reinitialization
    after a move registers after it (MD-125);
  - branch scope cleanup, and loop `continue`/`break` cleanup of a loop-body owner;
  - a defer reading a later assignment;
  - return and Err protection;
  - discard on both branches and on one branch.

  `testdata/sema/cleanup_interleaving_invalid.sec` keeps the defer-body
  restrictions (`return`/`break` S1203, bodyless `try` S1093).

## D. Add explicit ownership and cleanup Semantic IR

- [x] P17-26 — Add CopyValue, SemanticCopyValue and MoveValue operations with
  resolved actions/plans, value identity and provenance. Copy keeps the source;
  move creates the ownership continuation and removes source responsibility.

  Done 2026-10-25 (Claude), Semantic IR model scope (`ownership_ops.go`):
  - **Operations.** `own.copy`, `own.semantic-copy` and `own.move` keep the value type.
  - **Copy contracts.** `own.copy` requires a copy-trivial type. `own.semantic-copy` requires a semantic class and a named infallible semantic-copy plan.
  - **Move.** `own.move` consumes its operand.
  - **Verification.** A block-local flow check rejects any later use or second consumption of a consumed value (`TestOwnershipOperationsVerifyAndPrint`, `TestOwnershipOperationsRejectInvalidUse`).
  - **Boundaries.** Producer migration is P17-35, the path-sensitive ownership verifier is P17-52, and schema 13 is P17-45+.
- [x] P17-27 — Add CopyFromPlace/MoveFromPlace through canonical P15 Places.
  Preserve holder/referent distinctions, partial-path state and borrow obligations.

  Partial 2026-10-25 (Claude):
  - **Operations.** `own.copy-from-place` (mode `trivial` / `semantic-infallible`) and `own.move-from-place` take a canonical P15 Place and produce its element type.
  - **Authority.** A move requires a writable Place.
  - **Place consumers.** These operations are admitted by P15's Place-consumer rule.
  - **Partial-path state (2026-10-25, with P17-30).** A move out of a field or active-payload sub-Place is tracked by root Place identity and field/payload path. Moving, copying or replacing an overlapping subobject (or the whole value) is rejected until the subobject is reinitialized, and a move below an array element is rejected (§ 15).
  - **Remaining.** Moves on other CFG paths and borrow obligations of the moved subobject are the path-sensitive verifier's job (P17-52).
  Partial 2026-11-03: stored-field moves out of local and parameter owners lower;
  the owner stays registered and scope cleanup destroys its remaining fields.
  Field replacement and reinitialization lower (P17-28), and so does replacing a
  whole owner after its fields moved.
  Done 2026-11-06 (Claude):
  - **Path-sensitive part.** Moves on other CFG paths and borrow obligations of a
    moved subobject are enforced by `--sec-verify-ownership` (P17-52, done), and
    real source reaches it with per-path field moves (`TestSchema13PerPathPartialState`).
  - **Moves from places.** `own.move-from-place` is produced for whole and
    field moves out of storage and parameters.
  - **Copies from places.** A trivial copy reads its Place with `place.read`;
    `own.copy-from-place` with mode `semantic-infallible` waits for the
    semantic-copy producer of P17-09.
- [x] P17-28 — Add InitializePlace and ReplacePlace. Initialization/reinit has
  no old destruction; replacement consumes a fully prepared new value and owns
  old destruction plus old/new cleanup incarnation facts.

  Done 2026-10-25 (Claude), Semantic IR model scope:
  - **Initialize.** `own.initialize-place` consumes a value into a writable Place of its type. The verifier rejects old destruction-plan/cleanup metadata and accepts a new cleanup identity (`TestOwnershipInitializationNewCleanup`; initialization mutations in `TestOwnershipOperationsRejectInvalidUse`, Codex 2026-10-25).
  - **Replace.** `own.replace-place` consumes the prepared replacement and carries the old value's destruction plan. A non-trivial plan also requires distinct old/new cleanup action identities (printed `cleanup=#old->#new`).
- [x] P17-29 — Add DiscardValue/DestroyValue/DestroyPlace with plan identity and
  explicit/implicit/compiler provenance. Represent conditional/no-op discard
  without double destruction. Borrow-holder termination never frees referents.

  Partial 2026-10-25 (Claude):
  - **Discard.** `own.discard` requires an origin (`explicit` / `implicit-call-result` / `compiler-temporary`). A trivial discard carries no plan and is a no-op, and a non-trivial one requires its plan. A reference/slice discard must not carry a plan, because it never destroys the referent.
  - **Destroy.** `own.destroy-value` and `own.destroy-place` require a plan matching the destroyed type and a cause (scope-exit, function-exit, temporary-end, cleanup, compiler-generated). `own.destroy-value` of a reference/slice is rejected in favour of `ref.end-borrow`.
  - **Plan table.** Module-level `DestructionPlans` records are unique and printed.
  - **Double destruction.** It is rejected within a block.
  - **Remaining.** Conditional discard and double destruction across paths need the path-sensitive verifier (P17-52).
  Done 2026-11-06 (Claude): conditional discard lowers as `own.discard if_owned` /
  `own.destroy-place if_owned`, and double destruction across paths is rejected by
  `--sec-verify-ownership`/`--sec-verify-cleanups` (P17-52/54, done). Source
  coverage: `DiscardConverges`/`DiscardOnOnePath` in `TestSchema13CleanupInterleaving`,
  conditional payload discard in `TestSchema13EmitsOwnedVariants`, and temporary
  ends in `TestSchema13CleansFailedConstruction`.
- [x] P17-30 — Represent derived destruction and exact remaining subobject state
  compactly. Large arrays must not expand one IR record per element. Keep custom
  free, remaining-field cleanup and semantic release authority distinct.

  Done 2026-10-25 (Claude), Semantic IR model scope (`ownership_state.go`):
  - **Destruction plan graph.** The module `DestructionPlans` records form a closed, type-checked graph (`verifyDestructionPlanGraph`):
    - **struct:** a custom-free flag plus remaining-field steps listed once in reverse declaration order. Every non-trivial field must be covered.
    - **fixed-array:** one element plan with the exact count, so a 1000-element array is one record.
    - **active-variant:** per-variant payload plans covering every non-trivial payload.
    - **compiler-known-resource:** a release contract.
    - **trivial:** only for types needing no destruction.

    Steps must name existing plans of the stepped type, and a plan may not carry another kind's steps.
  - **Custom free, field cleanup, release.** Custom free (struct only), remaining-field cleanup (field steps) and resource release (resource contract) stay distinct.
  - **Remaining subobject state.** It is kept compactly as root Place identity plus field/payload path. `own.destroy-place` lists its `Skipped` subobjects (printed `skip=[field#0]`). The verifier requires every subobject moved earlier in the block to be skipped, rejects skipping through custom-free or non-struct/variant plans, rejects partial moves out of custom-free structs (§ 12) and array elements (§ 15), and lets reinitialization (`own.initialize-place` on the field) restore completeness (§ 13).
  - **Tests.** `TestOwnershipDerivedDestructionAndPartialState` and `TestOwnershipDerivedDestructionAndPartialStateRejects` (14 negatives).
  - **Remaining.** Cross-path state is P17-52, and source producers are P17-35.
- [x] P17-31 — Extend struct/array/union/Result/Option constructor actions to
  resolved move and infallible semantic copy. Spread remains copying; this package
  does not silently make a move-only spread source legal.
  Partial 2026-11-02: single union, Option and Result payloads take a fresh or
  moved value (`construct-direct`). Semantic copy and named-field payloads remain.
  Done 2026-11-06 (Claude):
  - **Moved and fresh operands.** Struct fields, named-field union payloads,
    Option/Result payloads and array elements take moved owners and fresh values
    (`TestSchema13ConstructorActions`, 32/64, `testdata/semantic_ir/constructor_actions.sec`).
  - **Failed construction.** Partly evaluated operands are cleaned on a failing
    exit (P17-16).
  - **Semantic copy.** Actions are admitted but have no Sec 0.1 producer (P17-09).
  - **Spread.** Spread stays trivial-copy only; a move-only spread source is
    still rejected.
- [x] P17-32 — Make argument, return and branch ownership actions explicit.
  Mutually exclusive successors may transfer the same current owner; merges
  select one continuation. Verify compatible partial/active-variant state and
  destruction plans without rejecting legal source convergence by approximation.
  Partial 2026-10-26 (Claude): owned by-value parameters are registered by
  Sema at function entry (`cleanup_parameters.go`, kind `owned-parameter`,
  activation `at-function-entry`) and tracked on entry in Semantic IR, so they
  are destroyed at every exit unless moved out. Arguments needing destruction
  use the new action `move`: a moved owned local (`<-h`: own.move, its cancel
  emitted only when the call commits after every argument, destruction.md
  § 9.2(3)) or a forwarded fresh temporary; the verifier requires `move` for
  parameters whose type needs destruction (`TestOwnedParametersAndArgumentMoves`).
  Remaining: branch ownership merges and consuming `->` parameter contracts.
  Done 2026-11-06 (Claude):
  - **Branches.** Mutually exclusive successors transfer the same owner
    (`ExclusiveTransfers`, `DiscardConverges`). A merge receives owned values
    with `sec.successor_actions = [["move"]]` (`MergedOwner`, the arm blocks of
    `match_arm_blocks.sec`).
  - **Consuming parameters.** `->` parameters take moved owners, with the
    `move` argument action and the cancellation at call commit.
  - **Verification.** `--sec-verify-ownership` checks these without
    approximation (`TestSchema13BranchOwnership`, 32/64,
    `testdata/semantic_ir/branch_ownership.sec`).
  - **Elsewhere.** A move into a constructed match subject (`match Some(<-h)`)
    is a separate named boundary in governance.

- [x] P17-33 — Add cleanup track/cancel/defer-register/run-scope/run-function
  operations and explicit defer regions with capture-Place parameters. Preserve
  dynamic registrations without selecting a physical stack or global runtime.

  Done 2026-10-25 (Claude), Semantic IR model scope (`cleanup_ops.go`):
  - **Operations.**
    - `cleanup.track-owned` (value or Place, with action/scope/ordinal identities and a non-trivial destruction plan; borrow holders are rejected);
    - `cleanup.cancel` (registered action, with reason `moved`/`replaced`/`transferred`);
    - `cleanup.defer-register` (action/scope/ordinal/defer identities, capture Places and one body region);
    - `cleanup.defer-yield` (the region terminator);
    - `cleanup.run-scope` (exited scopes plus a scope exit kind);
    - `cleanup.run-function` (function exit kind only, before its block's return).
  - **Defer region.** The region is verified as an isolated body: its entry parameters must equal the capture Place types, it cannot use outer SSA values or return, and every block ends with a yield or an internal branch.
  - **Registrations.** Action and ordinal identities are unique per operation site, which may run repeatedly (dynamic loop registrations stay one site).
  - **Runtime.** No stack, runtime or physical representation is chosen.
  - **Tests.** `TestCleanupOperationsVerifyAndPrint` follows the § 41 order, plus `TestCleanupOperationsRejectInvalidUse` (17 negatives).
  - **Remaining.** Reverse-order and cross-path cleanup execution checks are P17-54, and producers are P17-35. Producers also need Sema `ResolvedCleanupPlan` (P17-19, P17-21–24).
- [x] P17-34 — Extend the model, deterministic printer, effect/provenance facts
  and verifier. Print transfer, Place, destruction/cleanup/scope/defer identities;
  verify terminal actions and cleanup order without physical representation.

  Done 2026-11-06 (Claude), as the union of finished items:
  - **Model and printer.** The ownership/cleanup model and deterministic printer
    come from P17-26..33. They print transfer actions, Places, destruction plans,
    cleanup action/scope/ordinal/defer identities, `if-owned`/`if-active` and
    `unless-active`.
  - **Verification.** Terminal actions and cleanup order are verified without a
    physical representation by the Semantic IR verifier and by
    `--sec-verify-ownership`, `--sec-verify-destruction-plans` and
    `--sec-verify-cleanups` (P17-52..54).
  - **Evidence.** Every schema-13 source test added on 2026-11-06 runs all three.

## E. Connect source lowering solely through resolved plans

- [x] P17-35 — Migrate local, call, constructor, return and branch producers to
  consume immutable plans. Missing required facts produce structured unsupported
  lowering; do not fall back to fresh ownership inference in the builder.

  Partial 2026-10-26 (Claude), first producer slice at MaxPackage 17:
  - **Custom free.** A resolved custom `free` builds as a non-callable lifecycle
    function over `ref mut self` (Sema's `@free:<Type>`); predecessor packages
    still reject custom free outright.
  - **Destruction plans.** `importDestructionPlans` copies Sema's plan graph into
    the module table (struct with custom free and reverse-order field steps,
    fixed arrays, trivial); a custom-free plan names its built free function and
    the verifier checks it is `ref mut T -> void`.
  - **Owned locals.** An immutable root-scope local with
    `ResolvedOwnedCleanupInitializationOf` emits `cleanup.track-owned` (Sema's
    action/scope, source-order ordinal, root plan) after its initialization.
  - **Function exits.** Every return of such a function gets
    `cleanup.run-function` with Sema's resolved exit kind, after the protected
    return value is built.
  - **Boundaries kept.** Nested/loop scopes (P17-42), mutable owned storage,
    error-propagation exits, variant/resource plans and moves out keep explicit
    P17 boundaries (`cleanup_producers.go`, `cleanup_producers_test.go`).
  - **Moves (2026-10-26).** Sema's move of a whole tracked owned local at a
    return or a `let x :<- h` initialization is `own.move` followed by
    `cleanup.cancel` of its registration (`transferred` / `moved`); the new local
    is tracked as the new owner and the returned value leaves before function
    cleanup (`TestOwnedLocalMovesCancelTheirRegistration`). Argument moves need
    owned parameters (P17-32); field and partial moves stay P17-27/P17-36.

  Done 2026-11-07 (Claude):
  - **Remaining producers.** Union construction payloads read
    `ResolvedUnionConstruction.PayloadActions`, try-handler payload bindings
    read `ResolvedTryHandlerBindingTransferOf`, and Option `try` reads
    `ResolvedTry.SuccessTransfer`. Locals, calls, struct literals, returns,
    branches and match bindings already consumed Sema plans (P17-27..44).
  - **No fallback.** A resolved action without a lowering (semantic copy) or a
    missing transfer fact is a structured `UnsupportedFeatureError`. The
    builder's remaining type queries decide destruction, value representation,
    generic specialization or predecessor-package gates, never copy versus move.
  - **Evidence.** `TestBuilderConsumesResolvedTransferPlans`
    (`internal/ir/semantic`); the IR, lowering and schema-13 suites are unchanged.
- [x] P17-36 — Lower direct stored-field partial move, field reinit and non-trivial
  field replacement through Places. Preserve the owner of untouched fields; do
  not replace one owned field by copying and rebuilding its whole struct.
  Partial 2026-10-26 (Claude): mutable owned locals live in address-taken
  storage initialized through `own.initialize-place` and tracked by Place;
  stored fields are read through `place.field`/`place.read` (never a whole-value
  load); whole-local replacement is `own.replace-place` with the old plan and
  `cleanup=#old->#new` plus cancel/re-track, reinitialization after a move is
  initialize + track, and a move out is `own.move-from-place` + cancel
  (`TestMutableOwnedStorage`). Re-registration outside the declaring scope is
  rejected until path-exact incarnation matching (P17-19); field partial moves
  and field replacement remain.

  Done 2026-11-06 (Claude):
  - **Field moves and reinitialization.** Stored-field partial moves and field
    reinitialization lower, including on some paths, in loops and for nested
    fields (`TestSchema13PerPathPartialState`).
  - **Field replacement.** Non-trivial field replacement lowers in place for
    locals, nested fields and owned parameters, under the owner's registration
    (`TestSchema13RegistrationProducers`, `ReplaceAfterFieldMoves` in
    `TestSchema13EmitsOwnedVariants`).
  - **Reads in place.** A field below a non-trivial field is read through
    `place.field` (P17-16).

- [x] P17-37 — Lower move-only active union/Result/Option payload bindings only
  on the predecessor's proven variant path, updating container partial state.
  Keep borrowed bindings on P15 and respect MD-031 handler emission limits.
  Partial 2026-11-02: unguarded whole-payload bindings in `match` and
  `if x is Some(y)` on a fresh subject or a tracked local, on union, Option and
  Result. A mutable local moves through the Place form of package17 § 63; an
  immutable one at value level, since its SSA owner has no writable Place.
  Guarded arms commit the move in the arm body after a non-owning guard candidate.
  Stored-field subjects (`match b.o`) lower since 2026-11-03.
  Remaining: named-field and concrete error-variant payloads.
  Update 2026-11-06 (Claude):
  - **Stored subjects.** Copy-trivial named-field bindings on a stored (mutable)
    owned subject are read through the payload Place on the proven variant path.
  - **Guarded payloads.** A guarded whole named-field payload binding gives the
    guard a candidate read and commits the move in the selected arm.
  - **Evidence.** `TestSchema13PayloadBindings` (32/64,
    `testdata/semantic_ir/payload_bindings.sec`); both fixtures left the boundary
    rows.
  - **Remaining.** A guarded try handler on an error that needs destruction. The
    guard needs a candidate Place for the error payload, and Sema publishes no
    guard Place for try handlers (it does for match arms). Moving or borrowing a
    named field out of a stored subject also remains. Semantic-copy bindings have
    no Sec 0.1 producer (P17-09).
  Update 2026-11-07 (Claude):
  - **Named fields of stored subjects.** `ref` / `ref mut` named fields of a
    stored (mutable) owned subject, or of a fresh subject in compiler storage,
    are borrowed through the subject's payload Place. A named-field move is not
    a lowering gap: flowcontrol_match.md § 15 makes a by-value binding of a
    move-only field a compile-time error.
  - **Guarded try handlers on owned errors.** The handler takes the error, by
    registration or as an arm temporary, only in its guard's true block. A
    false guard passes the error, untouched, to the next handler.
  - **Evidence.** `TestSchema13PayloadBindings` (StoredShared, StoredMutable,
    ImmutableLocal), `TestSchema13FreshSubjectBorrows` (FieldOfFresh),
    `TestSchema13GuardedTryHandlersOnOwnedErrors`
    (`testdata/semantic_ir/guarded_try_handlers.sec`).
  - **Remaining.** A guarded try handler that moves an owned error payload
    (`Err(E.V(h)) where h.id > 0`). The guard needs a candidate read of the
    payload before the move commits, and a try error has no Place: Sema would
    have to publish a guard Place for try-handler payloads, as it does for match
    arms (`ResolvedMatchGuardPlaceOf`), and lowering would hold the error in
    compiler storage. This is the error_handler_payload.sec boundary row.

  Done 2026-11-07 (Claude):
  - **Guarded owned payload moves.** Sema publishes the binding Place of a
    guarded try handler (`ResolvedTryGuardPlaceOf`). The guard reads a candidate
    through `place.value-root` of the error value and its payload Place under
    the handler's variant test, and the payload moves only in the guard's true
    block. A false guard leaves the error whole for the next handler.
  - **Sema fix.** A try handler's binding was declared without `Local`, so its
    Place had a static root. This also made `ref e` of an owned handler binding
    fail in Semantic IR with an internal error. It is now a local, like a match
    payload binding.
  - **Evidence.** `MovedPayload` and `BorrowedBinding` in
    `testdata/semantic_ir/guarded_try_handlers.sec`, and the former boundary
    fixture `package17_variants/error_handler_payload.sec`, all verified by
    `TestSchema13GuardedTryHandlersOnOwnedErrors`. The boundary row is gone.
- [x] P17-38 — Lower non-trivial whole fixed-array ownership and destruction.
  Extend resolved ordinary element actions; preserve spread copy rules and the
  unsupported partial element-move boundary, including runtime indexes.

  Completed 2026-10-27 in this section's source → Semantic IR scope:
  ordinary elements retain resolved direct/move actions, including terminal
  aggregate forwarding; each committed source move precedes the next element.
  Whole local/parameter/return/call transfers, mutable replacement and nested
  arrays use canonical ownership and cleanup plans. Fresh defaults retain compact
  exact counts (including zero and one million elements), with reverse element
  destruction represented by the existing array destruction plan. P15 indexed
  borrows and whole moves of arrays holding mutable references preserve their
  non-owning element destruction. Spread still copies only admitted trivial
  elements; static/runtime indexed partial move-out remains rejected.
  Tests: `owned_arrays_test.go`, `array_literal_ownership_test.go` and
  `array_ownership_boundary_test.go`, with source fixtures in
  `testdata/semantic_ir/owned_fixed_arrays*.sec` and
  `testdata/sema/owned_fixed_arrays_invalid.sec`.
  Failed/partial construction (P17-16), unbound temporary cleanup (P17-40)
  and schema-13 emission (P17-45 onward) retain explicit unsupported boundaries;
  predecessor schemas never silently emit the new ownership construction.
- [x] P17-39 — Prepare fallible RHS ownership before replacement. Failure leaves
  the destination and old cleanup intact; success terminates the old incarnation
  once, installs the new value and registers its obligation at that point.

  Done 2026-11-06 (Claude), with MD-125 superseding "registers its obligation at
  that point" for a replacement (destruction.md §§ 6.1(4)–(5), 13.2(5)–(8)):
  - **Failure.** `h = try TryMake()` builds the right-hand side first; the failure
    edge leaves through `cleanup.run-function exit=error-propagation` (or the
    handler arm's `return`) with the old owner and its registration untouched.
  - **Success.** One `own.replace-place` with equal old/new identities destroys
    the old value and keeps the live registration and its position; a recovery
    value from a local handler replaces the same way.
  - **Handler-arm returns.** `Err(_) => return 0` now records its function cleanup
    exit in Sema (`analyzer_try_handlers.go`), so it lowers with owned locals.
  - **Evidence.** `FallibleReplace`, `RecoverReplace` and `HandlerReturn` in
    `testdata/semantic_ir/inherited_registration.sec`, verified through every
    cleanup/ownership pass by `TestSchema13InheritsReplacementRegistration` (32/64).
- [x] P17-40 — Lower explicit/implicit discard and temporary lifetime end through
  the same destruction plans as normal cleanup; honor must-use legality and
  non-consuming expression classification instead of dropping an SSA result.

  Partial 2026-11-02: `discard Make(1)` and a call statement whose result needs
  destruction lower to `own.discard` with the type's plan and origin. `discard name`
  of a whole owned binding destroys once and cancels its registration, and an
  unavailable binding is a no-op. Remaining: a binding moved on only some paths, a
  discarded projection, `discard try ...`, and a borrowed temporary that needs
  full-expression cleanup.
  Update 2026-11-03: a binding moved on only some paths lowers (`if_owned`), and
  temporaries end with their full expression, including a match subject the arm
  leaves (destruction.md § 8(8)–(9)). Remaining: temporaries created on only some
  paths of their full expression, mutable borrows of owned temporaries, a
  discarded projection and `discard try ...`.
  Update 2026-11-06 (Claude):
  - **Temporaries on some paths.** A short-circuit operand and a match
    expression arm are the scope of their temporaries, which end there before
    the merge (owner decision 2026-11-06; destruction.md § 8(10)).
  - **Mutably borrowed temporaries.** An owned temporary that is mutably borrowed
    lives in compiler storage for the call and ends in place at its full
    expression.
  - **Other forms that now lower.** A fresh array literal observed by `.Len`,
    `discard try ...` and a discarded stored field.
  - **Evidence.** `TestSchema13TemporaryEnds` (32/64) over
    `testdata/semantic_ir/temporary_ends.sec`; two fixtures left the boundary rows.
  - **Remaining.** A borrowed payload of a fresh match subject, and a move into a
    constructed match subject (`match Some(<-h)`).

  Done 2026-11-07 (Claude):
  - **Borrowed payload of a fresh subject.** Allowed by owner decision 2026-11-07
    (flowcontrol_match.md § 13). Sema gives the payload Place under the subject's
    temporary Place and a match-scoped temporary origin; lowering holds the
    subject in compiler storage, borrows through it and ends the borrow before
    the arm destroys the subject, on fallthrough, return, value arms and a
    failing guard (`borrowEndPosition`).
  - **Constructed subject.** `match Some(<-h)` now takes `h` when the subject is
    evaluated. Before this, Sema never committed that move, so a later use of
    `h` was accepted (a soundness bug, now S1088). Lowering emits `own.move` and
    `cleanup.cancel` before `union.construct`.
  - **Evidence.** `TestSchema13FreshSubjectBorrows` (32/64) over
    `testdata/semantic_ir/fresh_subject_borrows.sec`;
    `TestUnionPayloadPatternBorrowsFromTemporarySubject` (replaces the former
    rejection test) and `TestConstructedMatchSubjectMovesItsOperands`.
- [ ] P17-41 — Lower executed defer sites into regions with binding/Place captures.
  Preserve function scope and iteration-instance identity; never copy captures or
  collapse dynamic registrations into one active boolean per source site.

  Partial 2026-11-02: `defer_producer.go` lowers a defer to `cleanup.defer-register`
  with an isolated region over capture Places: a read-only value root for an immutable
  binding and the address-taken storage for a mutable one, so the body sees the value
  when it runs, reads and writes included. Each execution registers one instance, also
  in loops, through schema 13 and the slice and P15 verifiers. Remaining: capturing a
  value that needs destruction or a reference holder, an owned local in the body, a
  nested defer, and forms Sema does not publish as captures, such as `*a`, each a named
  boundary.
  Update 2026-11-03: a defer body reads owned values through their capture Places
  (fields, shared borrows, borrowed receivers). Replacing a captured owner after
  the defer stays a named boundary; --sec-verify-cleanups rejects the ordering.
- [x] P17-42 — Insert planned scope cleanup before exited-scope edges, and function
  cleanup after return/Err transfer but before return. Keep panic/raw process-exit/
  proven-unreachable endpoints outside normal cleanup; no hidden unwinding.

  Partial 2026-10-26 (Claude), at MaxPackage 17 for owned locals:
  - **Scope exits.** Falling out of an if/else, loop, switch-case, unsafe or
    match-arm block, `break` and `continue` emit `cleanup.run-scope` over exactly
    the scopes Sema's exit leaves (innermost first), directly before the edge.
  - **Function exits.** Every return (also from nested scopes and try-handler
    bodies) emits `cleanup.run-function` with Sema's exit kind after the
    returned value is built; panic endpoints run no cleanup.
  - **Only when relevant.** Cleanup is emitted only for exits leaving a scope
    with owned registrations, so programs without owned locals keep their
    predecessor IR (`TestScopeCleanupOnlyWithOwnedLocals`).
  - **Completeness.** Every cleanup-relevant Sema exit must have been emitted;
    one built outside a cleanup-aware block (e.g. a value-producing handler
    block) is rejected instead of dropped.
  - **Value-producing blocks (2026-10-26).** A try-handler block that yields a
    recovery value builds the value first, then runs its scope cleanup, then
    branches to the merge (`TestValueProducingHandlerBlockRunsScopeCleanup`);
    statement handler blocks use the cleanup-aware block path.
  - **Remaining.** Match-expression arm blocks (not yet lowered at all) and
    path-dependent incarnations; error-propagation exits, whole-local moves,
    owned parameters and mutable storage are done (P17-23, P17-32, P17-35/36).
  Done 2026-11-06 (Claude):
  - **Match-expression arm blocks.** These now lower. Sema implements
    flowcontrol_match.md § 21 (the final expression of a continuing arm block
    is the arm result, with ownership validated like an expression arm). The
    builder builds the value, runs the arm scope cleanup and branches to the
    merge (`TestSchema13MatchArmBlocks`, `TestMatchArmBlockValues`).
  - **Path-dependent incarnations.** These are covered by MD-125 inheritance and
    `unless_active` (P17-19).
- [x] P17-43 — Remove predecessor trivial-only gates only for operations whose
  complete transfer, destruction and cleanup path now exists. Keep default
  package selection conservative until the emitter/verifier bridge is operational.

  Done 2026-11-06 (Claude):
  - **Inspection paths.** At `MaxPackage` 17 the trivial-only gates are lifted
    exactly for the operations with a complete transfer, destruction and cleanup
    path. The P17 capability preflight (`ownership_capabilities.go`) and the named
    boundaries keep every other form closed. `emit-ir`/`emit-sec-mlir` run at
    `inspectionMaxPackage = 17` behind all three verifiers
    (`cmd/compiler/native_pipeline.go`).
  - **Predecessor packages.** Packages 14–16 keep their trivial-only gates
    unchanged (`TestPackage17CapabilityModuleAndPredecessorBoundaries`).
  - **Native track.** `emit-mlir`/`emit-llvm`/`build` stay conservative at
    `nativeTrackMaxPackage = 14` until physical ownership cleanup exists.
- [x] P17-44 — Add source-to-IR tests for every new use/exit form, including
  old-owner cancellation, partial masks, transactional failures and repeated defer.
  Assert semantic operations/order, not merely parser/Sema success.

  Done 2026-11-06 (Claude). Each form has a source-to-IR test that asserts
  operations and order, and that then runs the schema-13 verifiers:
  - **Old-owner cancellation and inheritance:** `TestSchema13InheritsReplacementRegistration`
    (equal identities, no `reason=replaced`, single `track-owned`) and
    `TestMutableOwnedStorage`.
  - **Partial masks:** `TestSchema13PerPathPartialState`,
    `ReplaceAfterFieldMoves`/`ReplaceAfterConditionalFieldMove` in
    `TestSchema13EmitsOwnedVariants` (`skip=[...]`, `if_owned`).
  - **Transactional failures:** `TestSchema13CleansFailedConstruction`
    (temporary-end count and reverse order, a move deferred until call commit)
    and the `FallibleReplace` order in `TestSchema13InheritsReplacementRegistration`.
  - **Repeated defer:** `DeferInLoop` in `TestSchema13EmitsOwnedVariants`, and
    the loop registrations and per-execution order in `TestSchema13CleanupInterleaving`.
  - **Exits:** branch/continue/break/return/error-propagation cleanup in
    `TestSchema13CleanupInterleaving`; handler and match-arm block exits in
    `TestSchema13MatchArmBlocks` and `HandlerReturn`/`HandlerBreak`.

## F. Implement schema 13 emission and three independent verifiers

- [x] P17-45 — Add schema-13 acceptance/version metadata after schema 11/12 works.
  Keep schema 1–12 regression inputs valid and Semantic IR version 1 unchanged.

  Done 2026-11-02: `SecDialect.cpp` admits schema 13 with the schema-11 module metadata
  requirements, and P16 operations accept schema 12 or later. Lit stays green with the
  older schemas as regression inputs.
- [x] P17-46 — Define ownership actions, destruction-plan references, cleanup
  IDs/ordinals, discard origins and exit-kind metadata. Map Semantic IR
  `construct` to dialect `construct-direct` explicitly; no physical offsets or
  runtime/source ownership-token type belongs in these attributes.

  Done 2026-11-03: every own.* and cleanup.* operation carries its plan, identities,
  ordinals, causes, origins and exit kinds; calls, returns and branches carry
  `sec.result_action`, `sec.return_action` and `sec.successor_actions`.
- [x] P17-47 — Add all ten `sec.own.*` operations from dialect-v13 §§4–13 with
  parse/print/type/plan/Place verification and appropriate semantic effects.
- [x] P17-48 — Add `sec.cleanup.track_owned`, `cancel`, `defer_register`,
  `defer_yield`, `run_scope` and `run_function`. Validate region captures,
  terminators, permitted exit kinds and scope/function eligibility.

  Done 2026-11-02 for P17-47/48: `SecOwnershipOps.td`, `OwnershipOps.cpp`. The local
  contracts reject a copied `ref mut`, a destroyed or tracked reference, a read-only
  move, equal replacement identities, a defer body returning or reaching outer values,
  and panic as a cleanup exit (`schema13-ownership-ops.mlir` and its invalid companion).
  Path-sensitive state is P17-52 to P17-54.
- [ ] P17-49 — Extend constructor/call/branch/return ownership attributes and
  predecessor action enums. Foreign consumption requires existing declared FFI
  ownership metadata; do not invent release or ownership contracts.

  Partial 2026-11-02: schema 13 admits `move` call arguments and `move` and
  `copy-semantic-infallible` array elements. Struct and union constructor actions,
  branch and return metadata remain, with their source producers.
  Update 2026-11-03: union payloads (single and named-field) construct with
  `construct-direct`, struct fields admit `move` and `copy-semantic-infallible`,
  and call results, returns and branches carry verified ownership actions.
  Remaining: foreign consumption, which needs declared FFI ownership metadata.
- [x] P17-50 — Emit the exact resolved ownership/cleanup plans, regions and source
  provenance. Earlier scalar/trivial lowering must preserve address-taken and
  non-trivial storage and must not erase identities/actions before verification.

  Partial 2026-11-02: `internal/lowering/secmlir/ownership.go` emits every own.* and
  cleanup.* operation, defer regions and the `sec.destruction_plans` module table;
  `TestSchema13EmitSourceOwnership` runs real source through schema 13 and the slice
  and P15 verifiers on 32/64-bit plans.

  Done 2026-11-02: `emit-ir` and `emit-sec-mlir` now run at package 17 and schema 13, after
  all three P17 verifiers (`TestOwnershipProgramsReachOnlyTheInspectionBoundaries`, 32/64-bit,
  deterministic). `emit-mlir`, `emit-llvm` and `build` stay on the package-14 native track and
  name the missing physical ownership cleanup. Source forms that stop at a named boundary
  are listed under `lowering.sec-mlir-package17` in `governance/lowering.yaml`.
- [x] P17-51 — Register the three analysis passes in TableGen, pass registration,
  CMake and the configured compiler verification path. Preserve previous guard/
  borrow/slice checks; verify ownership before destructive cleanup normalization.
- [x] P17-52 — Implement `--sec-verify-ownership`: availability, legal copy/move,
  no mutable-reference copy, partial state, mutually exclusive transfers, compatible
  merges and one terminal responsibility for represented non-trivial ownership.

  Done 2026-11-02: `VerifyOwnership.cpp` follows every owned value and subobject over the
  CFG (`schema13-ownership.mlir` and its invalid companion; real source runs through it in
  `TestSchema13EmitSourceOwnership`). It rejects use after a move on any path, copying a
  value that needs destruction, a move out of a borrowed referent, array element or custom
  free field, an initialization over a possibly live value or a replacement of a possibly
  gone one, a destroy whose skips do not match the moved subobjects, a registered value
  transferred without `sec.own.move`, and a value that needs destruction with no terminal
  action on some path. Non-triviality comes from the plan table (P17-53), so a value whose
  type the table does not list is not held to a terminal action.
  It found a real leak: `Make(1).id` never destroyed its temporary. That form now stops at
  the named boundary "owned struct temporary field read cleanup (P17-40)", because the
  temporary lives to the end of its full expression and that cleanup did not exist yet.
  Since 2026-11-03 the temporary is destroyed at the end of its full expression.
- [x] P17-53 — Implement `--sec-verify-destruction-plans`: concrete completeness,
  field/array/active-payload order, resolved custom-free restrictions and applicable
  semantic allocator/release pairing. Reuse P15/P16 validity and borrow facts.

  Done 2026-11-02: `VerifyDestructionPlans.cpp` checks the `sec.destruction_plans` table
  and every operation naming a plan (`schema13-destruction-plans.mlir` and its invalid
  companion cover the § 123 list; real source runs through it in
  `TestSchema13EmitSourceOwnership`). It requires one plan per identity and per type,
  struct fields once in reverse declaration order with every non-trivial field covered,
  the exact array element plan and count, one payload plan per non-trivial Union, Option
  or Result variant, a free function over a mutable reference to its type, and a body that
  neither ends part of its receiver nor frees it again. Uses must name a plan of exactly
  the destroyed type; a skipped path must exist and go through a plan that may skip.
  Limits: MLIR types carry no custom-free marker, so non-triviality comes from the table
  and a type no operation destroys need not be listed; the Semantic IR verifier covers the
  type-level facts first. Allocator pairing has nothing to check until P18 owning arrays;
  a named-field variant has no payload plan shape yet, so a non-trivial one is rejected.
- [x] P17-54 — Implement `--sec-verify-cleanups`: successful registration,
  cancellation, remaining-state cleanup, reverse registration order, defer
  multiplicity/capture lifetime, exit eligibility and protected return/Err values.

  Done 2026-11-02: `VerifyCleanups.cpp` follows each registration over the CFG with
  a path-exact state (`schema13-cleanups.mlir` and its invalid companion). It rejects a
  missing function exit, a moved value left uncancelled including on one branch, a
  double or unknown cancel, a cancelled defer, a destroyed returned value, registration
  out of source order or still active, cleanup in a defer body and a run_function not
  directly before its return, and it allows no cleanup on a panic path. Real source runs
  through it in `TestSchema13EmitSourceOwnership`. It also rejects consuming what an
  active defer captures, and a moved subobject ends the whole registration, which has no
  remaining-field form yet. `run_function` is one operation, so reverse order is enforced
  on registration order; the execution order is fixed by the later cleanup expansion stage.

## G. Prove integration and capability boundaries

- [x] P17-55 — Add positive and malformed-IR tests for every ownership/cleanup
  operation, attribute and defer region. Cover the negative matrices in P17
  §§130–131, including missing cleanup, wrong order, double terminal actions,
  moved/uninitialized destruction, invalid merges and defer on block exit.

  Done 2026-11-03. Positive: `schema13-ownership-ops.mlir` (every own.* and cleanup.*
  op), `schema13-ownership.mlir`, `schema13-ownership-variants.mlir`,
  `schema13-cleanups.mlir`, `schema13-destruction-plans.mlir`. § 130 evidence
  (Semantic IR `ownership_ops_test.go` for type-level copy rules; lit otherwise):
  copy move-only type — "own.copy requires a copy-trivial type" and "copies a value
  that needs destruction"; semantic copy without plan — "resolved infallible
  semantic-copy plan"; move already moved, use after move, double consume, double
  destroy, destroy moved value — "uses a value that is moved, destroyed or
  uninitialized" (`invalid-schema13-ownership.mlir`); destroy uninitialized field —
  `@destroy_uninitialized_field`; ref-mut copy — "mutable reference is never copied"
  (`invalid-schema13-ownership-ops.mlir`); partial move from custom-free type — "moves
  a field out of a value with custom free"; incompatible ownership merge — "transfers
  one owner twice" and the merged-use cases; return value also cleaned — `@returned`;
  call-consumed value also cleaned — `@uncancelled`, `@branch_uncancelled`
  (`invalid-schema13-cleanups.mlir`). § 131 evidence: missing registration — "leaves
  an owned value with no terminal ownership action"; missing normal-exit cleanup —
  `@missing_exit`; wrong destruction order — `@order`; wrong defer order —
  `@defer_order`; uncancelled moved cleanup — `@uncancelled`; canceled twice —
  `@twice`; defer on lexical block exit and defer omitted on return —
  `@missing_defer_exit` (run-scope never runs a defer); defer after returned control —
  `@not_before_return`; panic claiming normal cleanup — "invalid exit kind".
- [x] P17-56 — Prove P11–P16 integration: active variants and by-value match
  payloads, non-trivial structs/fields, whole fixed arrays, reference/slice transfer
  and discard without referent destruction. Keep each unresolved predecessor gap
  visible and exclude no required integration from the package completion claim.

  Done 2026-11-03: `ownership_variants.sec` through all verifiers on 32/64-bit plans
  (TestSchema13EmitsOwnedVariants) covers variants, payload bindings, fields, fixed
  arrays of owned variants, holder moves, holder discard and slices beside owners.
  TestPackage17KeepsPackage16Programs builds every valid fixture that builds at
  package 16 at package 17 too; it found and fixed two P17 regressions (a moved
  reference holder lost its reference facts; a forwarded trivial pack hit the
  cancellation gate). Remaining predecessor gaps stay in governance/lowering.yaml.
- [x] P17-57 — Test correctness without copy/move elision, DCE, RVO, SROA or
  mem2reg. Reject optimization that duplicates owners, loses cleanup, changes
  registration/destruction order or violates borrow/defer lifetime dependencies.

  Done 2026-11-03: no P17 route runs an optimization; the verifiers run on the
  emitted IR as is. `schema13-ownership-optimizations.mlir` shows canonicalize+CSE
  keep every ownership operation except a pure trivial own.copy and still verify.
  Where canonicalization rewrites a branch it drops the discardable
  `sec.successor_actions`, and --sec-verify-ownership then rejects the result
  rather than accepting a lost move; a later optimization pipeline must carry
  that metadata (governance item).
- [x] P17-58 — Regress int128/int256/uint128/uint256/decimal128 and 32/64-bit
  scalar plans without changing builtin status or narrowing exact array lengths.
  Keep existing legacy paths operational; their defer lowering is not P17 proof.

  Done 2026-11-03: `WideKinds`/`WideBeside` hold all five wide builtins beside an
  owned value through all verifiers on 32/64-bit plans; exact million-element array
  lengths remain covered by `owned_arrays_test.go`; the predecessor test keeps
  every package-16 fixture building.
- [x] P17-59 — Run real source → Semantic IR → emitted schema-13 → all applicable
  Sec verifiers, for valid and invalid cases. Confirm dynamic arrays, partial
  array move, unresolved custom-free cases, interface representation and physical
  ownership lowering remain explicit boundaries. P17 does not promise native
  resource/destructor execution before a physical lowering package exists.

  Done 2026-11-03: TestSchema13EmitSourceOwnership and TestSchema13EmitsOwnedVariants
  run real source through all seven verifiers; TestOwnershipProgramsReachOnlyThe-
  InspectionBoundaries keeps native routes at a named physical boundary. Named
  boundaries: dynamic arrays (P18, TestPackage17CapabilityRejectsMissingOwnershipIR),
  array element partial move ("fixed-array partial move is deferred"), custom-free
  unions, interface views ("type Sink"), and native ownership lowering.

## H. Final acceptance, governance and handoff

- [x] P17-60 — Expand the coverage map below into one evidence row per §136
  acceptance criterion, with implementation/test references and any unresolved
  authority conflict. A completed subtask is not completion of its entire family.

  Done 2026-11-03: "P17-60 acceptance evidence" below, 45 rows; as of
  2026-11-05: 32 met, 1 superseded (MD-025), 11 partial, 1 not met.
- [x] P17-61 — On the recorded execution snapshot, run full Go/vet, predecessor
  source pipelines, all three ownership/cleanup verifiers, `check-sec-mlir`,
  diff/private-path checks and canonical governance validation. Configured missing
  tools or skipped required integration tests cannot count as acceptance success.

  Done 2026-11-03 on the P17-01 baseline: `go vet ./internal/... ./cmd/...` clean;
  `go test ./... -count=1` 43 packages pass, `sec/cmd/lsp` fails three tests outside
  P17 (two on the owner's draft stdlib, `TestAtomicOrderMatrixLSP` on concurrent
  enum-hover text); check-sec-mlir 208/208 with all three P17 verifiers; no private
  paths in changed files; `git diff --check` clean; yamlstatus 0 problems. The
  § 136 row "go test ./... passes" therefore stays Not met.
- [x] P17-62 — Synchronize the package entry and linked memory/control-flow/
  predecessor governance according to actual scope. Mark P17 implemented only
  after every applicable criterion has evidence and authority conflicts are
  resolved; retain physical/P18 deferrals rather than closing unrelated items.

  Done 2026-11-03: `lowering.sec-mlir-package17` stays partial (revised
  2026-11-03, P15 predecessor marked implemented, report linked); its remaining
  items list every open item and named boundary.
- [x] P17-63 — Write the §137 implementation report addressing all 45 requested
  topics in order: exact snapshot/environment, files, plans/operations, algorithms,
  verifiers, integration, test commands/results, deviations and P18 handoff.

  Done 2026-11-03: `rules/mlir/packages/sec-mlir-dialect_package17-implementation-report.md`,
  all 45 topics in order.
- [x] P17-64 — Recheck report, unique ownership, rulebook status, negative cases,
  operational legacy paths and predecessor evidence before final handoff.

  Done 2026-11-03: report claims rechecked against the ledger (P15 is
  implemented, corrected), one governance owner, `language-rulebook-status.md`
  updated for destruction.md § 8(8)–(9), negative cases mapped in P17-55, legacy
  routes and predecessor fixtures covered by TestPackage17KeepsPackage16Programs.

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

## P17-60 acceptance evidence — 2026-11-03

One row per § 136 criterion. "Met" names its evidence; "Partial" and "Not met"
name what remains and its owning item. P17 is not complete while any row is
Partial or Not met (P17-62).

| # | § 136 criterion | Status | Evidence / remaining |
|---|---|---|---|
| 1 | baseline documents repo 152c772 + local P13–P16 or newer equivalent | Met | P17-01: HEAD `cfff1b4b` plus the dirty tree recorded 2026-11-03 (section below); LLVM 24.0.0git, lit 24.0.0dev, Go 1.26. |
| 2 | previous package regressions remain green | Partial | TestPackage17KeepsPackage16Programs (package-16 fixtures still build at 17); full `go test ./...` result in P17-61. The two `cmd/lsp` fmt-package tests fail on the owner's draft `sec/stdlib`, outside P17. |
| 3 | ownership/destruction/defer/discard synchronization applied | Met | P17-03 done 2026-11-05: audit recorded, B/C/E merged, MD-125 (option B) applied to destruction.md, semantic_ir.md and defer.md with the P17 sync texts reconciled. Owner decisions 2026-11-03 in destruction.md § 8(8)–(9). Inherited-registration implementation is tracked separately. |
| 4 | Semantic IR ownership amendment applied | Met | P17-04; `ownership_ops.go`, `cleanup_ops.go`, `ownership_state.go`. |
| 5 | schema-v13 dialect rulebook installed | Partial | `sec_mlir_dialect_v13.md` present and implemented; P17-05 synchronization against v11/v12 not recorded. |
| 6 | lowering-v13 rulebook installed | Partial | `sec_mlir_lowering_v13.md` present; P17-05 open as above. |
| 7 | copy classification is complete for concrete runtime types | Partial | P17-08 done; implicit semantic copy with an infallible plan (P17-09) is not produced by lowering. |
| 8 | destruction classification is complete for concrete runtime types | Partial | P17-10/14; a union with custom free has no accepted plan (named boundary). |
| 9 | ResolvedTransferPlan API implemented/read-only | Partial | P17-12/13: branch-use and some construction sites remain (P17-13 note). |
| 10 | ResolvedDestructionPlan API implemented/read-only | Met | P17-14; `destruction_plans.go`, ResolveDestructionPlans. |
| 11 | ResolvedCleanupPlan API implemented/read-only | Met | P17-21/22/24; `cleanup_plans.go`, ResolvedCleanupPlanOf. |
| 12 | explicit trivial copy IR implemented | Met | own.copy, own.copy-from-place (P17-26). |
| 13 | explicit infallible semantic copy IR implemented | Partial | own.semantic-copy op and verifier exist (P17-26); no source producer (P17-09). |
| 14 | explicit move IR implemented | Met | own.move, cancel (P17-26, P17-35). |
| 15 | copy/move from Place implemented | Met | own.move-from-place for storage, fields, payloads, error payloads; partial-path state (P17-27 note). |
| 16 | initialization/reinitialization implemented | Met | P17-28; whole and field reinitialization. |
| 17 | replacement implemented | Partial | Whole, field, conditional and post-field-move replacement lower; a fallible RHS (P17-39) and replacement of an owner a defer reads are named boundaries. |
| 18 | partial struct field move implemented | Met | P17-36 note 2026-11-03; Field* fixtures. |
| 19 | field reinitialization implemented | Met | assignOwnedField; FieldReplaceAndReinitialize. |
| 20 | active variant payload move implemented | Partial | match, if-binding, try, Ok()/Err(), error variants, handlers, guarded single payloads; guarded whole named-field payloads and guarded handlers on owned errors remain (P17-37). |
| 21 | arbitrary fixed-array element partial move remains rejected | Met | "fixed-array partial move is deferred" (ownership_state.go); invalid lit `moves out of an array element`. |
| 22 | explicit/implicit discard IR implemented | Met | own.discard explicit/implicit/conditional; stored fields through their Place and `discard try ...` since 2026-11-04 (P17-40). |
| 23 | non-trivial destruction IR implemented | Met | own.destroy-value, own.destroy-place with skipped paths and if_owned. |
| 24 | struct/array/union/Result/Option destruction plans implemented | Met | P17-14/53; named-field variants via payload struct plans; custom-free unions carry custom_free on their active-variant plan since 2026-11-04 (CustomFreeUnion). |
| 25 | cleanup tracking/cancellation implemented | Met | cleanup.track-owned/cancel incl. if_active (P17-33). |
| 26 | unified cleanup registration order implemented | Met | Ordinals checked by --sec-verify-cleanups (`@order`, `@defer_order`). |
| 27 | defer registration region implemented | Met | cleanup.defer-register with isolated region (P17-41 note). |
| 28 | defer captures Place/binding identity | Partial | Value and owned captures through Places; reference-holder captures, owned locals or nested defer in a body remain (P17-41). |
| 29 | scope cleanup implemented | Met | cleanup.run-scope on every exit kind incl. arm temporaries (P17-42). |
| 30 | function-exit cleanup implemented | Met | cleanup.run-function before every return (P17-42). |
| 31 | return/error payload excluded from cleanup | Met | Verifier "would destroy the returned value"; propagation returns (P17-23 note). |
| 32 | no cleanup-on-panic assumption introduced | Met | Panic exits run no cleanup; `panic` is an invalid exit kind (lit). |
| 33 | schema-v13 own/cleanup ops implemented | Met | P17-47/48. |
| 34 | ownership metadata added to constructors/calls/branches/returns | Partial | P17-46 done; foreign consumption awaits declared FFI ownership metadata (P17-49). |
| 35 | --sec-verify-ownership registered | Met | P17-52. |
| 36 | --sec-verify-cleanups registered | Met | P17-54. |
| 37 | --sec-verify-destruction-plans registered | Met | P17-53. |
| 38 | P11–P16 non-trivial integrations implemented where specified | Met | P17-56 evidence. |
| 39 | user-defined free syntax remains gated | Superseded | MD-025 (applied 2026-10-15) accepts custom `free`; package17 § 22 sync note records the supersession. |
| 40 | no mandatory runtime | Met | No runtime library or registry is introduced; physical lowering is a named boundary. |
| 41 | no physical cleanup representation selected | Met | if_owned/if_active and run-scope stay abstract (destruction.md § 11.2). |
| 42 | no LLVM ownership/destruction decision deferred | Met | Native routes stop at a named boundary before LLVM (TestOwnershipProgramsReachOnlyTheInspectionBoundaries). |
| 43 | check-sec-mlir passes | Met | 208/208 on 2026-11-03 (P17-61). |
| 44 | go test ./... passes | Not met | See P17-61: the two `cmd/lsp` fmt-package tests fail on the owner's draft stdlib. |
| 45 | legacy paths remain operational | Met | Native routes stay at package 14 and schema-11/12 emission is unchanged; TestPackage17KeepsPackage16Programs. |

### P17-01 execution baseline — 2026-11-03

HEAD `cfff1b4bca0002818dc07af53ea85160a3caffbe` (2026-11-02) plus 146 uncommitted
or untracked paths from this package and concurrent Codex work; HEAD alone does
not reproduce the tree. Toolchain: LLVM/MLIR 24.0.0git (llvm-project
`252f31a6893b`), lit 24.0.0dev, Go 1.26. Tool directories come from the local
`LLVM_DIR`, `MLIR_DIR` and `SEC_MLIR_BIN`; their machine paths are omitted.

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
