# Debug Information Cross-Rulebook Correction
- **Status:** Applied
- **Created:** 2026-09-23
- **Last updated:** 2026-09-23
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical correction path:** `rules/corrections/applied/debug-information-cross-rulebook-correction-20260923.md`
- **Primary new rulebook:** `rules/compiler/debug_information.md`
- **Implementation governance:** `governance/compiler.yaml` (`compiler.debug-information-v1`)
- **Repository baseline reviewed:** `main-reviewed-2026-09-23`

---
## § 1. Purpose

§ 1(1) This correction package synchronizes existing canonical rulebooks and inventory/governance references with `rules/compiler/debug_information.md`.

§ 1(2) It does not redefine the debug-information model. The new rulebook is the canonical owner of debug levels, source stepping, value/type debug state, generic debug identity, logical execution frames, debug-artifact correlation, target debug realization, and debug conformance obligations.

§ 1(3) Existing owning rulebooks remain authoritative for their own semantics.

§ 1(4) Corrections in this package should be applied atomically with addition of `rules/compiler/debug_information.md` and the `compiler.debug-information-v1` governance integration where practical.

---
## § 2. `language-rulebook-status.md`

§ 2(1) In the compiler/tooling inventory, replace:

```text
| `compiler/debug_information.md` | **Planned** | Source mapping, variables, optimized code, generics, async/task frames, and targets. |
```

with a Written entry equivalent to:

```text
| `compiler/debug_information.md` | **Written** | Canonical Sec 0.1 debug-information model: None/LineTables/Full, source step points, truthful optimized values, ownership/lifetime availability, concrete generic identities, logical task/thread/process debug views, split artifacts/source correlation, target-neutral lowering, and compiler conformance. Implementation is tracked by `compiler.debug-information-v1` in `governance/compiler.yaml`. |
```

§ 2(2) In the canonical written-rulebook list, add:

```text
compiler/debug_information.md
```

§ 2(3) In the Planned closure list, remove:

```text
compiler/debug_information.md
```

so that the immediate Sec 0.1 design-closure list becomes:

```text
compiler_testing.md
incremental_compilation.md
```

§ 2(4) `library/stdlib.md` remains a separate candidate and is not pulled into the compiler-rulebook closure by this correction.

---
## § 3. `rules/compiler/semantic_ir.md`

§ 3(1) `semantic_ir.md` remains authoritative for canonical source provenance, semantic declaration identity, Place identity, availability state, type identity, synthesized origins, and source-location tables.

§ 3(2) Add an explicit debug-consumer obligation to the source-provenance section equivalent to:

> Source provenance, semantic identities, Place/projection identities, availability transitions, concrete specialization identity, and synthesized-origin relations required by `rules/compiler/debug_information.md` must remain available until all requested debug-information consumers are complete. A lowering or optimization pass must not discard or coarsen these facts merely because they are no longer required by source semantic analysis.

§ 3(3) Add an explicit statement that Semantic IR does not define a second debug-specific availability lattice. `compiler/debug_information.md` consumes the canonical existing availability facts.

§ 3(4) Where synthesized operations are discussed, cross-reference `compiler/debug_information.md` for source stepping and synthetic-origin visibility.

§ 3(5) Where concrete generic/specialization identity is carried in Semantic IR, cross-reference `compiler/debug_information.md` for debugger presentation and machine-address independence.

§ 3(6) No source-visible type or syntax change is required in `semantic_ir.md`.

---
## § 4. `rules/compiler/compiler_pipeline.md`

§ 4(1) Add a normative pipeline invariant equivalent to:

> Every compiler transformation that affects code with surviving source/debug provenance must preserve the provenance accurately, transform it accurately, or explicitly invalidate it. Stale debug metadata is never a legal compiler output.

§ 4(2) State explicitly that the invariant applies through:

```text
semantic transformations
monomorphization
optimization
inlining/outlining
code motion
aggregate splitting
state-machine/task lowering
ABI lowering
Sec MLIR / LLVM lowering
LTO
link-time coalescing
final artifact emission
```

§ 4(3) State explicitly that requested debug-information level is orthogonal to optimization legality.

§ 4(4) `Full` must not be documented as implicitly disabling optimization, tail calls, frame omission, body sharing, dead-code elimination, or other otherwise legal transformations.

§ 4(5) When optimization makes exact source state unrecoverable, the compiler must reduce debug precision rather than retain a stale source value or source frame.

§ 4(6) Add `rules/compiler/debug_information.md` to the related/cross-reference list for source provenance, optimization, backend, and final artifact sections.

---
## § 5. `rules/compiler/monomorphization.md`

§ 5(1) `monomorphization.md` remains authoritative for generic template, concrete instantiation, plan realization, plan entry, implementation body, and canonical specialization identity.

§ 5(2) Add an explicit cross-reference that `compiler/debug_information.md` consumes the concrete specialization identity.

§ 5(3) Preserve or add the invariant:

```text
Concrete semantic specialization identity
    != PlanEntry identity
    != ImplementationBody identity
    != machine address
    != backend mangled name
```

§ 5(4) State that distinct concrete semantic specializations retain distinct debug identities even when legal monomorphization shares machine code.

§ 5(5) State that debug tooling, breakpoints, or cached debug data must not create generic specialization demand that is absent from canonical program reachability/demand.

§ 5(6) State that compiler/backends may use distinct entries or physical cloning for tooling when necessary, but no unique machine body per specialization is required when correct debug identity can be preserved without cloning.

§ 5(7) No runtime generic-reflection mechanism is introduced by this correction.

---
## § 6. `rules/compiler/linking.md`

§ 6(1) `linking.md` remains authoritative for LinkPlan, final image construction, LTO/linking, binary symbol identity, reproducibility, and final artifact verification.

§ 6(2) Add debug output to the final-artifact planning/verification boundary when the resolved CompilationPlan requests debug information.

§ 6(3) The LinkPlan/final artifact model must support:

```text
requested DebugInformationLevel
requested debug placement
final image/debug companion correlation identity
post-LTO/post-link source mapping finalization
strip policy that distinguishes debug-only metadata from runtime-required metadata
```

§ 6(4) Separate debug artifacts must correlate with the exact final generated image, not merely with a source project or filename.

§ 6(5) Late transformations such as LTO, ICF/coalescing, outlining, dead stripping, and symbol/address rewriting must update, merge, relocate, or invalidate debug mappings before final artifact verification.

§ 6(6) Existing reproducibility rules must include relevant debug-artifact inputs and must exclude incidental host paths/timestamps/random values where the resolved reproducibility policy requires deterministic output.

§ 6(7) Stripping must never remove metadata required by runtime, ABI, loader, panic, or another non-debug contract merely because that metadata is also useful to a debugger.

---
## § 7. `rules/compiler/compiler.md`

§ 7(1) Add `rules/compiler/debug_information.md` to the canonical compiler-rulebook architecture.

§ 7(2) The compiler architecture should identify debug lowering as a consumer of canonical Semantic IR/CompilationPlan facts rather than an independent semantics source.

§ 7(3) The compiler must not infer complete Sec debug support from the existence of a native DWARF/CodeView emitter alone.

§ 7(4) Unsupported requested debug contracts are build/compiler diagnostics; inconsistent metadata after support was promised is a compiler defect.

---
## § 8. `rules/mlir/sec_mlir.md` and maintained lowering documentation

§ 8(1) Sec MLIR and later lowering layers must preserve the source/debug facts needed by the selected debug level until those facts are materialized into final target debug metadata.

§ 8(2) Exact operation syntax is owned by the MLIR package/rulebooks and is not prescribed here.

§ 8(3) Maintained lowering paths must not use backend line numbers as a replacement for canonical Sec source ranges, semantic identities, availability, or concrete generic identity before debug lowering is complete.

§ 8(4) Legacy lowering paths that cannot preserve the requested contract must reject that requested debug contract or remain explicitly incomplete; they must not silently emit misleading `Full` metadata.

---
## § 9. `rules/platform/target_profiles.md` and CompilationPlan target facts

§ 9(1) Target/debug support must be represented using semantic capability facts rather than one `DebugSupported` boolean.

§ 9(2) Resolved target information must be able to distinguish at least:

```text
supported debug levels
supported debug placements
source mapping support
Full source value/type/availability support
logical task-debug support when required by active program features
artifact/source correlation support
physical stack-walk support where requested
```

§ 9(3) The exact compiler-internal target-fact representation remains owned by target/CompilationPlan rules.

§ 9(4) A native debug format such as DWARF or CodeView is a mechanism, not proof that every Sec `Full` semantic capability exists.

§ 9(5) Missing statically known capability must be diagnosed during CompilationPlan/build validation with no silent downgrade.

---
## § 10. `rules/platform/abi.md`

§ 10(1) `abi.md` remains authoritative for physical parameter/return classification and calling convention.

§ 10(2) Add a cross-reference requiring debug lowering to preserve the canonical source signature independently from:

```text
hidden sret/result pointers
split register arguments
physical tag/payload transport
closure/environment parameters
compiler-generated ABI adapters
```

§ 10(3) Hidden ABI parameters do not become Sec source parameters in the canonical debugger view.

§ 10(4) Debugger stack-walk metadata and physical frame-pointer policy are target implementation details and do not alter the Sec ABI contract unless the target ABI itself requires them.

---
## § 11. `rules/concurrency/tasks.md`

§ 11(1) `tasks.md` remains authoritative for `Task[T]`, `TaskID`, lifecycle, migration, cancellation, observers, and result semantics.

§ 11(2) Add a cross-reference that debug tooling treats Task identity as logical execution identity independent of the worker thread that currently runs it.

§ 11(3) Task suspension/resumption and migration do not create a new Task debug identity.

§ 11(4) Debug observation grants no task lifecycle or result authority.

§ 11(5) Debug-only task enumeration/frame metadata is not a Sec source reflection API.

---
## § 12. `rules/concurrency/await.md`

§ 12(1) `await.md` remains authoritative for task suspension, completion, cancellation, and result semantics.

§ 12(2) Add `rules/compiler/debug_information.md` as the owner of debugger-specific suspension/stepping representation.

§ 12(3) `await`'s source location and live semantic state must remain available to debug lowering when `LineTables` or `Full` requires logical suspended-task inspection.

§ 12(4) The compiler-generated resume/state-machine path remains an implementation detail; debug information must not redefine `await` as a second call or source frame.

---
## § 13. `rules/concurrency/threads.md`

§ 13(1) `threads.md` remains authoritative for physical Sec Thread identity and lifecycle.

§ 13(2) Add a cross-reference that canonical Sec `ThreadID` is distinct from a platform/native thread identifier and from logical `TaskID`.

§ 13(3) A debugger may correlate an executing Task with its current physical worker Thread without merging their identities.

§ 13(4) Debug observation does not grant thread join, detach, cancellation, termination, or result authority.

---
## § 14. `rules/concurrency/processes.md`

§ 14(1) `processes.md` remains authoritative for process isolation, `ProcessID`, lifecycle, spawn, join, detach, termination, and Command behavior.

§ 14(2) Add a cross-reference that each process forms a separate debug address-space domain.

§ 14(3) `ProcessID` may correlate the Sec process in debugger metadata but is not an OS PID and grants no debugger attach authority.

§ 14(4) Parent/child spawn and join relations are causal/wait relations rather than call-stack edges.

§ 14(5) Automatic follow/attach of spawned child processes remains debugger/target/security policy.

---
## § 15. `rules/errors/panic.md`

§ 15(1) `panic.md` remains authoritative for panic policy, containment, cleanup, and target termination semantics.

§ 15(2) Add an explicit cross-reference stating:

```text
debugger physical stack walking
    != Sec panic/runtime unwinding
```

§ 15(3) Presence of CFI/frame metadata or a debugger's ability to reconstruct source frames must not be described as creating a general Sec unwind runtime.

§ 15(4) Panic source/provenance metadata may consume debug/source facts without making `LineTables` or `Full` mandatory for canonical panic semantics unless the panic rulebook separately requires specific metadata.

---
## § 16. Memory/ownership rulebooks

§ 16(1) `memory/ownership.md`, `memory/borrowing.md`, `memory/destruction.md`, `memory/memory_model.md`, `memory/storage.md`, and `memory/layout.md` remain authoritative for availability, validity, destruction, storage identity, and representation.

§ 16(2) Add cross-references only where useful; do not introduce debug-specific ownership semantics.

§ 16(3) `compiler/debug_information.md` consumes the canonical availability state so that moved/destroyed/uninitialized/partially available values are not falsely displayed as live.

§ 16(4) Debug observation does not create a borrow, lifetime extension, pin, owner, or resource capability.

§ 16(5) Physical representation/layout metadata used by a debugger does not turn private/internal layout into a stable public source ABI unless the owning layout rulebook already makes that guarantee.

---
## § 17. Hardware/MMIO rulebooks

§ 17(1) `platform/hardware-register-access.md`, volatile/register rules, and storage rules remain authoritative for potentially side-effecting device reads.

§ 17(2) Add a cross-reference that `Full` debug does not permit automatic source-value evaluation by performing a hardware read that the target cannot classify as safe observational access.

§ 17(3) A debugger may expose type/address/identity while the value remains not safely readable in ordinary source inspection.

---
## § 18. `rules/tooling/lsp.md`

§ 18(1) `lsp.md` remains authoritative for editor/LSP source-coordinate translation and language-server behavior.

§ 18(2) Debug information and LSP must consume a compatible canonical source-file/range identity model rather than inventing incompatible line/column coordinate systems.

§ 18(3) Protocol UTF-16 translation remains an LSP transport concern; debug information uses canonical compiler source coordinates and lowers them to the selected debugger format as required.

§ 18(4) A future debugger plugin may reuse compiler/LSP services for Sec naming, type display, or expression support, but this is not a requirement that `Full` embed an LSP database.

---
## § 19. `rules/compiler_testing.md` handoff

§ 19(1) When `compiler_testing.md` is written, it must own the test harness and execution infrastructure for compiler conformance.

§ 19(2) It must include a mandatory debug-information category capable of driving:

```text
canonical debug-model structural tests
DWARF/CodeView/target encoding tests
external/native debugger integration tests
Sec-aware debugger integration tests
multiple optimization levels
target capability matrices
split-artifact tests
path/source correlation tests
negative capability diagnostics
```

§ 19(3) `compiler/debug_information.md` owns what behavior must be tested; `compiler_testing.md` owns how those tests are organized, executed, isolated, compared, and reported.

---
## § 20. `rules/incremental_compilation.md` handoff

§ 20(1) When `incremental_compilation.md` is written, it must treat debug configuration as artifact/backend-generation state rather than canonical program-semantic identity.

§ 20(2) Relevant cache/invalidation inputs include:

```text
DebugInformationLevel
debug placement
physical debug format/version where output-affecting
path/source correlation policy
debug-only runtime hook requirements
final image/debug companion identity inputs
strip/debug artifact output policy
```

§ 20(3) Changing `None` to `Full` must not by itself redefine canonical types, ownership, generic identity, or source semantics.

§ 20(4) Semantic analysis may be reused when its cached artifacts preserve all canonical provenance/value/type facts required by the newly requested debug output.

§ 20(5) Exact cache-key and invalidation policy remains owned by `incremental_compilation.md`.

---
## § 21. Governance synchronization

§ 21(1) Add one integration entry:

```text
compiler.debug-information-v1
```

to `governance/compiler.yaml`.

§ 21(2) Do not create a second semantic-owner fragment for debug information.

§ 21(3) `governance/compiler.yaml` already owns compiler pipeline, linking, initialization, and artifacts and is therefore the canonical owner.

§ 21(4) `governance/index.yaml` requires no new fragment registration for this work.

§ 21(5) Cross-links from compiler backend/linking, target capabilities, and future compiler-testing/incremental-compilation governance may refer to `compiler.debug-information-v1`, but they must not duplicate the same implementation status as independent semantic owners.

---
## § 22. Correction application order

§ 22(1) The recommended repository application order is:

```text
1. add rules/compiler/debug_information.md
2. add compiler.debug-information-v1 to governance/compiler.yaml
3. apply language-rulebook-status.md synchronization
4. apply compiler/Semantic-IR/monomorphization/linking cross-references
5. apply target/ABI cross-references and debug capability obligations
6. apply concurrency/panic/memory/tooling cross-references
7. register this correction under rules/corrections/ until every canonical owner change is merged
8. move the correction to rules/corrections/applied/ according to the repository correction workflow
```

§ 22(2) Application must preserve paragraph/section references in existing rulebooks. Existing numbered paragraphs must not be renumbered merely to insert a cross-reference; append or use the repository's correction integration convention instead.

§ 22(3) No historical transition wording is required in user-facing/manual documentation after synchronization. Canonical rulebooks should describe the current model directly.

---
## § 23. Completion criteria

§ 23(1) This correction is fully applied when:
- `rules/compiler/debug_information.md` exists at the canonical path;
- `language-rulebook-status.md` marks it Written and removes it from Planned;
- `governance/compiler.yaml` contains `compiler.debug-information-v1`;
- compiler pipeline/Semantic IR/monomorphization/linking contain the required provenance/debug cross-references;
- target/ABI rules expose the required debug capability/lowering boundary;
- task/await/thread/process/panic rules contain no contradictory debug assumptions;
- no rulebook states or implies that Full disables optimization or that debugger stack walking creates panic unwinding;
- no existing semantic owner has been duplicated by debug-specific semantics.

§ 23(2) Until these updates are merged, this correction package is the normative harmonization record for the new debug-information rulebook.
