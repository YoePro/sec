# SEC MLIR Package 17 implementation report

Report date: 2026-11-03. Reporting contract: P17 § 137, all 45 topics in order.

**P17 is partial.** Thirty of the 45 § 136 criteria are met, one is superseded
by MD-025, thirteen are partial and one is not met; the row-by-row evidence is
"P17-60 acceptance evidence" in `package17-todo.md`. Source, Semantic IR,
emitted schema 13 and physical execution are distinguished throughout: P17
reaches verified schema 13, and no destructor or cleanup runs natively. The sole
owner is `lowering.sec-mlir-package17` in `governance/lowering.yaml`, whose
remaining items list every open boundary named below.

## 1. Repository HEAD implemented against

HEAD `cfff1b4bca0002818dc07af53ea85160a3caffbe` (2026-11-02) plus 146
uncommitted or untracked paths, from this package and from concurrent work on
other areas. HEAD alone does not reproduce the tree; no commit or clean-tree
snapshot is made by this report.

## 2. Local/merged P13–P16 status

The ledger marks P13, P14, P15 and P16 implemented. P17 builds on their
types, Places, references, slices and verifiers and keeps their evidence in
their own plans.

## 3. Previous package status

`emit-ir` and `emit-sec-mlir` publish package 17 / schema 13 after all P17
verifiers; native routes (`emit-mlir`, `emit-llvm`, `build`) stay at package 14
and stop at a named boundary for programs that need ownership cleanup.
TestPackage17KeepsPackage16Programs builds every valid fixture that builds at
package 16 at package 17 as well; it found two P17 regressions, both fixed
(see 44).

## 4. Ownership/destruction/defer/discard synchronization

The applied normative sync and Semantic IR amendment are implemented (P17-04).
A recorded paragraph-by-paragraph comparison (P17-03) and the v11/v12 → v13
schema/lowering reconciliation (P17-05) remain open. Two owner decisions of
2026-11-03 were added to `destruction.md` § 8: a fresh match subject an arm
leaves lives through the whole match (§ 8(8)), and an if/while condition is a
full expression of its own (§ 8(9)).

## 5. Files added

Sema: `cleanup_*.go` (assignments, cancellations, exits, identities, match
bindings, parameters, plans, registrations), `copy_classification.go`,
`defer_plans.go`, `destruction_plans.go`, `transfer_destinations.go`,
`transfer_plans.go`. Semantic IR: `ownership_ops.go`, `ownership_state.go`,
`ownership_capabilities.go`, `cleanup_ops.go`, `cleanup_producers.go`,
`defer_producer.go`, `discard.go`, `match_owned_payload.go`, `temporaries.go`,
`result_projection.go`. Emission: `internal/lowering/secmlir/ownership.go`,
`array_ownership_boundary.go`. MLIR: `SecOwnershipOps.td`, `OwnershipOps.cpp`,
`Analysis/DestructionFacts.h`, `VerifyOwnership.cpp`, `VerifyCleanups.cpp`,
`VerifyDestructionPlans.cpp`. Tests and fixtures: the `*_test.go` siblings
above, `package17_predecessor_test.go`, nine schema-13 lit files under
`mlir/test/Analysis` and `mlir/test/Dialect/Sec`, `testdata/semantic_ir/
ownership_*.sec`, `package17_boundary_*.sec` and `package17_variants/`.

## 6. Files modified

Shared builders and verifiers: `internal/ir/semantic/builder.go`, `model.go`,
`verifier.go`, `place_reference.go`, `match_error_variants.go`, `loops.go`,
`reference_lowering.go`, `option_try.go`, `failure_set_try.go`; Sema
`analyzer.go`, `analyzer_inference.go`, `analyzer_discard.go`,
`analyzer_try_handlers.go`, `analyzer_result_projection.go`,
`match_arm_bindings.go`, `match_field_patterns.go`, `interface_views.go`,
`type_pack_spreads.go`; emission `emitter.go`, `emitter_options.go`,
`operation_attributes.go`, `place_reference.go`; MLIR `SecOps.cpp`,
`StructOps.cpp`, `SecDialect.cpp`, `VerifyPlaceReferences.cpp`, `Passes.td`,
`Passes.h`; `internal/mlir/toolchain.go`; `cmd/compiler/native_pipeline.go`;
`rules/compiler/compiler_pipeline.md` §§ 59, 61; `rules/memory/destruction.md`
§ 8.

## 7. Copy classification implementation

`CopyClassificationOf` resolves trivial, semantic, move-only and non-copyable
classes for concrete types; a still-conditional class is never treated as a
trivial copy. Transfer actions follow from it (`ResolveTransferAction`).

## 8. Destruction classification implementation

`TriviallyDestructible` and `ResolveDestructionPlans` classify structs (with
custom free), fixed arrays, unions, Option, Result and compiler-known
resources; generic templates, `any` and dynamic arrays are explicit
unsupported plan kinds.

## 9. ResolvedTransferPlan API

Read-only, detached plans per transfer site (initialization, assignment,
argument, return, construction, capture) with source Place, action, partial
flag and ownership states. Branch-use sites are not published separately
(P17-13 partial).

## 10. ResolvedDestructionPlan API

`ResolveDestructionPlans` returns a closed, pure plan graph keyed by canonical
type identity: struct fields in reverse declaration order, array element and
exact count, active-variant steps (single payloads and named-field payloads),
resources.

## 11. ResolvedCleanupPlan API

`ResolvedCleanupPlanOf` publishes scopes, registrations (owned locals,
parameters, replacements, reinitializations, defers, match and try-handler
bindings) and exits with exited scopes and protected values, without
mutating analysis state.

## 12. Whole-value copy implementation

`own.copy` and `own.copy-from-place` for copy-trivial values; MLIR rejects a
copy of a value that needs destruction and of a mutable reference.

## 13. Semantic-copy implementation

`own.semantic-copy` exists with plan verification at both levels; no source
producer emits it yet (P17-09).

## 14. Whole-value move implementation

`own.move` plus `cleanup.cancel` for tracked locals, parameters, returns,
arguments and constructions; holders move on their P15 path.

## 15. Place copy/move implementation

`own.move-from-place` for owned storage, stored fields, active payloads and
error-variant payloads, with root/path state in both verifiers.

## 16. Initialization/reinitialization

`own.initialize-place` for `let mut`, reinitialization after a move, field
reinitialization and owners held in storage for partial state.

## 17. Replacement algorithm

The RHS is built first; an available old value is replaced with
`own.replace-place` (new registration, old cancelled); a value owned on some
paths uses `if_owned`/`if_active`; after field moves the old value is
destroyed with skipped paths before initialization. A fallible RHS (P17-39)
and replacing an owner a registered defer reads are named boundaries.

## 18. Partial struct move/reinit

Field moves leave the owner registered; scope cleanup destroys the remaining
fields. Field replacement uses `own.replace-place` without new identities.

## 19. Union/Result/Option payload move

`match`, `if x is Some(y)`, `try`, `Ok()`/`Err()`, guarded arms through a
non-owning candidate, named-field payloads, error-variant payloads on fresh
Results and try handlers. Guarded whole named-field payloads and guarded
handlers on owned errors remain boundaries.

## 20. Discard implementation

`own.discard` for explicit, implicit call-result and conditional discards;
discarding an unavailable binding is a no-op. `discard try ...` and discarding
a projection that needs destruction remain boundaries.

## 21. Destruction plan lowering

`sec.destruction_plans` module table; every destroy, discard, replace and
track names a plan of exactly its type.

## 22. Struct/array/variant destruction order

Reverse declaration order for fields, reverse index for arrays, active payload
only for variants; custom free runs before the remaining fields
(destruction.md §§ 10.3, 15).

## 23. Cleanup track/cancel

`cleanup.track-owned` after successful initialization, `cleanup.cancel` at
every ownership end; path-dependent incarnations cancel with `if_active`.

## 24. Defer registration/capture representation

`cleanup.defer-register` with an isolated region whose entry arguments are
capture Places; owned captures are read through them. Reference-holder
captures, owned locals and nested defers in a body remain boundaries.

## 25. Scope cleanup

`cleanup.run-scope` on fallthrough, break, continue and arm exits, split
around match subjects and handler errors in creation order.

## 26. Function cleanup

`cleanup.run-function` immediately before every return.

## 27. Error propagation cleanup

Propagation returns run function cleanup after the error leaves; pending
temporaries and arm temporaries are destroyed on that edge first.

## 28. Branch/loop cleanup

Loop exits run the exited scopes; reassignment and moves in loops cancel each
incarnation where active.

## 29. Schema-v13 ops/attrs

Ten `sec.own.*` and six `sec.cleanup.*` operations, `if_owned`/`if_active`,
`payloadAction = "move"`, `construct-direct`, observing and candidate
`place.read`, the destruction-plan table and module metadata.

## 30. Constructor/call/branch/return ownership metadata

`sec.argument_actions`, `sec.result_action`, `sec.return_action` and
`sec.successor_actions` (move/copy/non-owning); struct fields admit `move`
and `copy-semantic-infallible` from schema 13. Foreign consumption needs FFI
ownership metadata that does not exist yet.

## 31. Ownership verifier

`--sec-verify-ownership`: path-sensitive owner/subobject state; rejects use
after move, copies of owners, double transfers, moves out of borrowed or
custom-free values and array elements, missing terminal actions and missing
or wrong action metadata.

## 32. Cleanup verifier

`--sec-verify-cleanups`: unique identities, registration order, active-state
cancellation, scope/function exits, returned-value protection, defer captures
and registrations that would outlive a reading defer.

## 33. Destruction-plan verifier

`--sec-verify-destruction-plans`: plan table completeness and order, custom
free signature and body, variant coverage including named-field payload
structs, and plan/type agreement at every use.

## 34. P11–P16 integration changes

Unions, Option, Result, enums and error unions; struct fields; fixed arrays;
Places, references and slices beside owners; payload Place chains in
`--sec-verify-places`.

## 35. Wide-type regression tests

`WideKinds` and `WideBeside` hold int128, int256, uint128, uint256 and
decimal128 beside owners through all verifiers on 32/64-bit plans.

## 36. Non-trivial aggregate tests

`ownership_variants.sec`, `ownership_source_core.sec`, `owned_arrays_test.go`
and the lit files cover struct, array, union, Option, Result and named-field
plans, partial moves and replacement.

## 37. Defer/order tests

Defer ordering, captures, loops, returns and the defer/owner ordering rule;
lit `@order`, `@defer_order`, `@defer_capture`, `@defer_reads_newer_owner`.

## 38. Unsupported partial-array/custom-free tests

Array-element move-out is rejected in both verifiers; partial moves out of
custom-free values are rejected by Sema and MLIR, including a payload moved out
of a custom-free union by a match arm (2026-11-04). A custom-free union's plan
carries its free before the active payload.

## 39. CMake commands

```sh
cmake --build build/sec-mlir
cmake --build build/sec-mlir --target check-sec-mlir
```

Tool directories are configured locally through `LLVM_DIR`, `MLIR_DIR` and an
absolute `SEC_MLIR_BIN`; their machine paths are omitted.

## 40. Exact LLVM/MLIR version

LLVM/MLIR 24.0.0git from llvm-project `252f31a6893b`; lit 24.0.0dev; Go 1.26.

## 41. check-sec-mlir result

2026-11-03: 208 discovered, 208 passed.

## 42. go test ./... result

2026-11-03 at HEAD `cfff1b4b` plus the dirty tree: `go vet ./internal/...
./cmd/...` clean; `go test ./... -count=1` passed 43 packages and failed one,
`sec/cmd/lsp`, with three tests. `TestAnalyzeLoadsFmtPackageAndTransitiveImports`
and `TestAnalyzeGroupedPackageImportsAndShortNames` fail on the owner's draft
`sec/stdlib` fmt package; `TestAtomicOrderMatrixLSP` expects enum hover text
that concurrent LSP enum-hover work changed. None is a P17 path, but the
criterion "go test ./... passes" is not met. The full `cmd/lsp` suite takes
about 12 minutes and needs `-timeout` above Go's default 10 minutes.

## 43. End-to-end source -> schema-v13 results

TestSchema13EmitSourceOwnership and TestSchema13EmitsOwnedVariants emit real
source deterministically on 32/64-bit plans and pass all seven verifiers;
TestOwnershipProgramsReachOnlyTheInspectionBoundaries keeps native routes at a
named boundary.

## 44. Deviations

- An immutable local match subject moves its payload at value level
  (`unwrap_payload` action `move`) rather than through a Place, since its SSA
  owner has no writable Place; mutable and stored subjects use the Place form
  of § 63.
- The guard of a moving arm reads a non-owning `candidate`; a fresh subject
  whose arms test an error variant or take a named-field payload is held in
  compiler storage under a Sema temporary Place.
- `sec.successor_actions` is a discardable attribute that MLIR canonicalization
  drops; the verifier then rejects the result (governance item).
- Two regressions found by TestPackage17KeepsPackage16Programs were P17's own
  and are fixed: a moved reference holder lost its reference facts, and a
  forwarded trivial pack hit the cancellation gate.

## 45. Recommendations for Package 18

Owning dynamic arrays need the plan kind `owning-dynamic-array` with backing
release and allocator pairing in `--sec-verify-destruction-plans` (§ 116);
partial element move needs element masks; physical lowering must choose
representations for `if_owned`, `if_active` and arm temporaries
(destruction.md § 11.2) and carry branch actions as inherent properties.
