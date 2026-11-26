# SEC-MLIR-P19 Implementation Report — Allocation Context and Arena Semantic Lowering

> Current canonical responsibilities: [semantic_ir.md §§ 50–51](../../compiler/semantic_ir.md),
> [arena.md](../../memory/arena.md), [sec_mlir_dialect_v15.md](../dialect-versions/sec_mlir_dialect_v15.md).
> Live implementation ledger: [governance/lowering.yaml](../../../governance/lowering.yaml).

- Status: report for package 19 (§ 164), written 2026-11-26 (P19-44)
- Package specification: `rules/mlir/packages/sec-mlir-dialect_package19.md`
- Work list and acceptance evidence: `package19-todo.md` (P19-42 matrix)
- Governance owner: `lowering.sec-mlir-package19` in `governance/lowering.yaml`

Package 19 is **not complete**. The acceptance matrix in `package19-todo.md`
has 46 criteria met, 6 partial, 1 open and 1 not run. Sections A, C and E–H
are closed; B and D keep plan and integration remainders whose owners are
named below.

## 1. Repository HEAD

The work is built on `1a693eec` plus the uncommitted working tree. The P19
sections A–B were started on 2026-11-16 (P19-01).

## 2. Local/merged P13–P18 status

- P13–P17 are closed.
- P18 is partial (40 met, 4 partial, 1 open); its schema 14 is admitted.
- All predecessors are local in this working tree.

## 3. Previous package status

Regressions stay green on 2026-11-26:

- **Go:** `internal/ir/semantic` and `internal/lowering/...` pass in full
  (with `SEC_MLIR_BIN`); `internal/sema` passes the Arena, allocation,
  atomic, panic and effect subsets.
- **Lit:** 269/269, covering schema 1–15 inputs.
- **Emission:** schemas 11–14 still emit; schema 15 is admitted after 14.

## 4. Arena/allocation normative synchronization

- **Sync document:** `sec_arena_allocation_sync_v1.md` was audited in P19-02.
- **Semantic IR:** `semantic_ir.md` §§ 50–51 and
  `rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md`; § 51(6)–(8) added
  for MD-140.
- **Corrections integrated during P19:** MD-136 (default construction),
  MD-140 (epoch exhaustion), MD-151 (`sec.system-heap.v1`), MD-095 (atomic
  storage layout touches the shared resolver).
- **Open decisions:** MD-152 (strict bounded-memory policy), MD-135
  (non-trivial `T[]` Append ownership), MD-053 (constraint-effect contracts),
  MD-153 (Reset headroom query).

## 5. Files added

- **C++ dialect:** `SecArenaOps.td`, `ArenaOps.cpp`.
- **C++ analysis passes:** `VerifyArenas.cpp`, `VerifyAllocationContexts.cpp`.
- **Go Semantic IR:** `allocation_context.go`, `allocation_context_lowering.go`,
  `arena_allocation.go`, `arena_demand.go`, `arena_lowering.go`,
  `arena_nested.go`, `arena_operations.go`, `arena_types.go`,
  `arena_verification.go`.
- **Go emitter:** `arena_emit.go`, `arena_demand_emit.go`.
- **Go Sema:** `allocation_context_requirements.go`, `arena_allocation_layout.go`,
  `arena_capacity_plans.go`, `arena_default.go`, `arena_default_plans.go`,
  `arena_demand.go`, `arena_effect_plans.go`, `arena_failure_plans.go`,
  `arena_operation_plans.go`, `arena_parent_plans.go`, `defer_arena_effects.go`.
- **Go tests:** the `package19_*_test.go` matrices in `internal/lowering/secmlir`
  (allocation, atomicity, context, integration, physical, release, reset,
  source), `arena_schema15_test.go`, `arena_types_test.go`, the Sema
  `arena_*_test.go` plan tests, `package19_surface_test.go`,
  `arena_epoch_exhaustion_test.go`, `loop_arena_generation_test.go`,
  `defer_arena_effects_test.go`, and the IR `arena_*_test.go` and
  `allocation_context_test.go`.
- **Lit:** `schema15-arena{,-default,-effects,-reset-epoch,-roundtrip}.mlir`,
  `invalid-schema15-arena{,-default,-verifier}.mlir`,
  `invalid-arena-{demand,publication,reset-epoch}.mlir`.
- **Rulebooks:** `sec_mlir_dialect_v15.md`, `sec_mlir_lowering_v15.md`,
  this report.

## 6. Files modified

- **Sema:** `analyzer.go`, `analyzer_arena.go`, `call_graph.go` (Arena
  effects), `semantic_facts.go`, `layout_requirements.go`.
- **Semantic IR:** `builder.go`, `verifier.go`, `model.go`, `printer.go`,
  `place_reference.go`, `ownership_ops.go`, `ownership_state.go`,
  `slice_source_try.go` and `reference_use.go` (handled-try Arena results).
- **Layout/platform:** `reference_policy.go` and `scalar_plan.go` (epoch
  policy), `target.go`.
- **Emitter:** `emitter.go`, `emitter_options.go`.
- **Rulebooks:** `arena.md`, `allocation.md`, `semantic_ir.md`,
  `reference_model.md`, `target_profiles.md`, `panic.md`.

## 7. Arena builtin normalization

`Arena` is one compiler-known builtin with canonical identity (P19-03,
TestArenaFrontendConformance).

## 8. Lowercase arena keyword status

`arena` is an ordinary identifier; no reserved keyword remains (P19-03).

## 9. MoveOnly integration

Arena is MoveOnly with a non-trivial destruction plan through P17
(TestOwningDynamicArrayAndArenaTraits, TestPackage19ReleaseMatrix).

## 10. ArenaDomain implementation

Domains are canonical identities independent of backing address and survive
owner moves (TestArenaDomainIdentitySurvivesOwnerMove,
TestArenaCreationDomainFreshness).

## 11. Arena state-version implementation

State versions are numbered by the Semantic IR builder in mutation order and
checked per block by both verifiers (v15 § 9, TestArenaEffectSiteOrdering).

## 12. Backing/growth/provider facts

Borrowed, owned and none backing with fixed and growable-nonrelocating growth
are planned and verified. **Partial:** provider-backed facts need the
`sec.system-heap.v1` implementation (MD-151; P19-04).

## 13. FromBuffer implementation

Source to schema 15 end to end (TestPackage19ArenaSourceLowering).

## 14. WithCapacity implementation

**Partial:** Sema plan, IR op and `sec.arena.create_owned_fixed` exist;
source lowering is a named boundary until `sec.system-heap.v1` (P19-20).

## 15. Growable semantic model

**Partial:** growth policy is planned and verified; growable allocation
lowering waits for the provider (P19-33 remainder).

## 16. New[T] validation/lowering

Canonical layout, default and capacity plans; lowered with
dynamic-epoch reference facts (TestPackage19ArenaAllocationLowering).

## 17. Alloc[T] validation/lowering

As § 16, with checked count and slice facts; locally handled `try` lowers
since P19-33.

## 18. AllocationError mapping

ArenaFailurePlan maps to the core AllocationError (P19-06).

## 19. Zero-element allocation

Zero-storage capacity plan; no backing or epoch change.

## 20. Allocation atomicity

Branch on the failed flag before publication, enforced in IR and
`--sec-verify-arenas` (TestPackage19AllocationAtomicityMatrix,
invalid-arena-publication.mlir).

## 21. Reset dependency/epoch implementation

AdvanceEpoch with a state step, retained backing and cursor reset
(TestPackage19ResetMatrix). MD-140: checked epoch advance with width and
`panic.arena-epoch-exhaustion` through Sema, IR and schema 15 (stages 2–4a).
**Partial:** defer interaction (P19-21), elimination proofs and physical
exhaustion lowering (MD-140 stage 2 remainder, 4b).

## 22. Release/backing-kind implementation

Borrowed and default Release end the domain without reclamation
(TestPackage19ArenaReleaseLowering). **Partial:** owned Release needs the
provider.

## 23. Implicit destruction integration

P17 cleanup performs Release at scope exit; explicit Release cancels it
(TestPackage19ReleaseMatrix).

## 24. Nested Arena dependency

Parent dependencies through Sema, IR and verifiers (P19-09, P19-16).

## 25. Allocation-context summary

RequiresAllocationContext distinct from MayAllocate
(TestAllocationContextRequirements).

## 26. Context selection/propagation

Hidden context forwarding and target-provided roots lower
(TestPackage19AllocationContextPropagation). **Partial:** explicit selection
and compiler-local context (P19-10 remainder).

## 27. Hidden Sec MLIR context parameter

`!sec.alloc_context` with `sec.hidden_allocation_context`
(TestSchema15HiddenContextForwarding).

## 28. Foreign ABI context guard

The IR verifier rejects hidden contexts on foreign signatures; Arena never
crosses the C ABI (TestPackage19NoArenaSurface).

## 29. noalloc behavior

Missing context is a compile-time error under noalloc and freestanding
(TestPackage19AllocationContextMatrix).

## 30. Ordered Arena effect implementation

Source-ordered direct Arena effects with stable domains (P19-11).

## 31. ArenaResource interface

MemoryEffects on the Arena resource (schema15-arena-effects.mlir).

## 32. Capacity-demand summary

`sec.arena_demand_summary` with constant, unknown and unbounded demand and
the § 81 call boundary (P19-37).

## 33. Strict profile validation

**Open:** waits for MD-152.

## 34. Schema-v15 types/ops

`!sec.arena`, `!sec.alloc_context` and all Arena/context ops (P19-22..25).

## 35. Arena verifier

`--sec-verify-arenas`: domain freshness, state/epoch chains, owner
consumption, stale use, demand summaries and failure-atomic publication.

## 36. Allocation-context verifier

`--sec-verify-allocation-contexts`.

## 37. P15/P16 verifier extensions

Dynamic-epoch reference and slice facts of Arena results verified
(TestPackage19IntegrationMatrix).

## 38. P17 cleanup integration

See § 23.

## 39. P18 allocation-context integration

`T[]` grows through the resolved context with Arena reclamation authority
(TestPackage19IntegrationMatrix).

## 40. Wide-type Arena tests

Layouts come from the canonical resolver on 32- and 64-bit plans
(TestPackage19ArenaAllocationLowering on amd64 and armv7).

## 41. Borrowed/owned/growable tests

Borrowed and default rows run end to end; owned and growable rows are the
provider remainders.

## 42. Reset/Release tests

TestPackage19ResetMatrix, TestPackage19ReleaseMatrix,
TestArenaResetEpochAdvance*.

## 43. Demand/profile tests

TestArenaDemandSummaries, TestArenaDemandInterproceduralBoundary; strict rows
wait for MD-152.

## 44. Unsupported concurrency/FFI/uninitialized tests

TestPackage19NoArenaSurface, TestPackage19NoPhysicalLowering; spawn
boundaries in TestAllocationContextRequirements.

## 45. CMake commands

```sh
cmake -G Ninja -S mlir -B build/sec-mlir \
  -DMLIR_DIR=$LLVM_BUILD/lib/cmake/mlir -DLLVM_DIR=$LLVM_BUILD/lib/cmake/llvm
ninja -C build/sec-mlir -j 4 sec-mlir-opt
ninja -C build/sec-mlir -j 4 check-sec-mlir
```

## 46. Exact LLVM/MLIR version

LLVM/MLIR 24.0.0git, llvm-project `252f31a6893b`, assertions enabled; Go
1.26.8.

## 47. check-sec-mlir result

269/269 passed (2026-11-26).

## 48. go test ./... result

Not run: the full suite needs owner authorization. Package suites are green.

## 49. End-to-end source -> schema-v15 results

FromBuffer creation, New/Alloc (propagated and locally handled `try`),
Reset, explicit Release, implicit destruction, the MD-136 default and hidden
context forwarding emit verified schema-15 Sec MLIR on linux amd64 and armv7.

## 50. Deviations

- Owned and growable rows stop at the provider boundary instead of
  inventing a provider ABI.
- Strict bounded-memory validation is not implemented while MD-152 is open.
- The Reset exhaustion check is represented, but its physical test and panic
  edge wait for physical Arena lowering (MD-140 stage 4b).

## 51. Recommendations for Package 20

Package 20 (floating-point semantics, § 165) is already in progress with its
own report. For Package 19, the next steps are the `sec.system-heap.v1`
provider (unblocking §§ 12, 14, 15, 22), the P19-10 context-selection
remainder, P19-21 defer interaction and an owner decision on MD-152.
