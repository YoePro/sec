# Debug Information
- **Status:** Normative
- **Created:** 2026-09-23
- **Last updated:** 2026-09-23
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/compiler/debug_information.md`
- **Replaces:** Planned debug-information entry; no earlier canonical rulebook
- **Repository baseline reviewed:** `main-reviewed-2026-09-23`
- **Implementation governance:** `governance/compiler.yaml` (`compiler.debug-information-v1`)
- **Related rulebooks:** `rules/compiler/semantic_ir.md`, `rules/compiler/compiler_pipeline.md`, `rules/compiler/monomorphization.md`, `rules/compiler/compiler.md`, `rules/compiler/linking.md`, `rules/mlir/sec_mlir.md`, `rules/platform/target_profiles.md`, `rules/platform/abi.md`, `rules/concurrency/tasks.md`, `rules/concurrency/await.md`, `rules/concurrency/threads.md`, `rules/concurrency/processes.md`, `rules/errors/panic.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/destruction.md`, `rules/memory/memory_model.md`, `rules/memory/storage.md`, `rules/memory/layout.md`, `rules/tooling/lsp.md`, `rules/compiler_testing.md`, `rules/incremental_compilation.md`

---
## § 1. Purpose and authority

**Governance tags:** `compiler.debug-information-v1`

§ 1(1) This rulebook defines the canonical Sec 0.1 debug-information model.

§ 1(2) It owns the compiler and artifact contracts for:
- debug-information levels;
- source mapping and source stepping;
- lexical scopes and source-variable identity;
- source-level value availability under optimization and ownership transitions;
- semantic type presentation;
- generic-specialization debug identity;
- logical task, thread, and process debug identity;
- embedded and separate debug-artifact correlation;
- target/debug-format realization;
- debug provenance through optimization, lowering, linking, and LTO;
- debug-information conformance obligations.

§ 1(3) This rulebook does not redefine Sec ownership, borrowing, lifetime, concurrency, panic, generic, ABI, linking, target, or source-language semantics. Their owning rulebooks remain authoritative.

§ 1(4) Mutable implementation status belongs in `governance/compiler.yaml`, not in this normative rulebook.

§ 1(5) This rulebook introduces no source-visible `DebugInfo`, `DebugFrame`, `DebugVariable`, reflection API, or debugger-control API in Sec 0.1.

---
## § 2. Fundamental debug-information principle

**Governance tags:** `compiler.debug-information-v1`, `compiler.pipeline`

§ 2(1) Debug information is metadata describing the relationship between generated execution and canonical Sec source semantics.

§ 2(2) Enabling, disabling, embedding, separating, or stripping debug information does not change the meaning of a Sec program.

§ 2(3) Debug information is orthogonal to:
- optimization level;
- runtime-check policy;
- panic strategy;
- ABI selection;
- type and storage layout;
- ownership and borrowing semantics;
- scheduling and synchronization semantics;
- target program semantics.

§ 2(4) `Full` debug information does not imply an unoptimized build.

§ 2(5) Debug hardening, compiler assertions, sanitizers, runtime checks, or similar instrumentation are separate build/profile concerns and are not implied by any debug-information level.

§ 2(6) A compiler may choose a different physical realization when required to satisfy the selected debug contract, provided the resulting program preserves the same Sec semantics and external ABI contract.

---
## § 3. Canonical debug-information levels

**Governance tags:** `compiler.debug-information-v1`

§ 3(1) Sec 0.1 defines exactly three canonical debug-information levels:

```text
None
LineTables
Full
```

§ 3(2) These are compiler/project configuration values. They are not source-visible `core` declarations.

§ 3(3) Exact command-line and project-manifest syntax is owned by the project/build configuration rules.

§ 3(4) Sec 0.1 defines no `Limited`, `LineDirectivesOnly`, or target-specific fourth language-level debug-information tier.

§ 3(5) A backend may internally use target-specific line directives or reduced physical encodings when realizing one of the canonical levels, provided the externally claimed Sec contract is satisfied.

---
## § 4. `None`

§ 4(1) `None` requires no source-level debug-information payload in the produced target artifact.

§ 4(2) `None` does not permit the frontend, semantic analyzer, Semantic IR, or compiler analyses to discard source provenance that is still required for diagnostics, verification, tooling, panic metadata, or another canonical compiler consumer.

§ 4(3) `None` does not mean that the binary is stripped.

§ 4(4) `None` does not require removal of:
- dynamic-linker symbols;
- exported symbols;
- unwind metadata required by runtime or ABI semantics;
- platform loader metadata;
- panic metadata required by another rulebook;
- any other non-debug runtime artifact requirement.

---
## § 5. `LineTables`

§ 5(1) `LineTables` is the lightweight source-debugging level.

§ 5(2) Where the selected target and debugger integration support the corresponding operation, `LineTables` must preserve enough metadata for:
- generated code to canonical source-file mapping;
- source line and source range mapping;
- function/subprogram identity;
- concrete generic-specialization callable identity;
- inline call-chain reconstruction;
- source breakpoint binding;
- source stepping;
- source symbolization of recovered stack frames.

§ 5(3) `LineTables` does not guarantee full parameter, local-variable, type-graph, or value-location inspection.

§ 5(4) `LineTables` and `Full` use the same canonical source-stepping and logical-frame model. Their principal difference is the amount of variable, type, availability, and value-location metadata emitted.

---
## § 6. `Full`

§ 6(1) `Full` includes the complete `LineTables` contract.

§ 6(2) `Full` additionally requires, for reachable/debug-relevant Sec code and where the concrete semantic value is representable under this rulebook:
- lexical scopes;
- source parameters;
- source locals;
- source-visible static/global bindings;
- canonical Sec type identity;
- generic-specialization and substitution metadata;
- source-variable availability state;
- source-variable location or exact value reconstruction metadata;
- relevant capture metadata;
- logical task-frame metadata required by this rulebook for active program features.

§ 6(3) `Full` does not mean that every source value must remain recoverable after arbitrary optimization.

§ 6(4) When an exact source value cannot be represented correctly, the canonical answer is unavailable or optimized-out, not a fabricated or stale value.

§ 6(5) `Full` is not a serialized compiler database and does not require emission of every parsed, dead, unreachable, or non-instantiated declaration.

---
## § 7. Truthfulness and source-semantic state

**Governance tags:** `compiler.debug-information-v1`, `semantic-ir.debug-provenance`

§ 7(1) Debug information must never knowingly describe stale machine state as a current Sec source value.

§ 7(2) A displayed collection of source variables and values must be compatible with a source-semantic state that could exist at the reported execution point.

§ 7(3) If optimization prevents exact reconstruction, information must be reduced rather than fabricated.

§ 7(4) The following rule is normative:

```text
Unavailable is correct.
Fabricated or stale is incorrect.
```

§ 7(5) Raw machine-memory inspection by a debugger is distinct from Sec source-level variable metadata and may expose bytes that no longer correspond to a live Sec source value.

---
## § 8. Ownership and lifetime availability

**Governance tags:** `compiler.debug-information-v1`, `semantic-ir.availability`

§ 8(1) Source-level debugger availability follows canonical Sec ownership and lifetime state.

§ 8(2) A committed move, discard, destruction, detach transition that consumes the binding, lifetime termination, or other canonical availability transition may make a binding unavailable even if old bits remain in registers or memory.

§ 8(3) Debug metadata must not resurrect an unavailable value merely because its former representation remains readable.

§ 8(4) Lexical visibility and value availability are distinct facts.

§ 8(5) A binding may remain lexically visible while its current value is unavailable.

§ 8(6) The debugger integration may present more specific availability reasons such as moved, destroyed, uninitialized, or inactive variant when those facts are preserved, but exact UI wording is not language semantics.

---
## § 9. Canonical source provenance

**Governance tags:** `semantic-ir.debug-provenance`

§ 9(1) Canonical debug source provenance is range-aware and origin-aware.

§ 9(2) The compiler-internal source-provenance model must preserve at least:
- canonical source-file identity;
- start line and column;
- end line and column where known;
- semantic source operation identity where relevant;
- lexical scope identity where relevant;
- user/synthesized/inlined origin classification;
- origin relation for synthesized code.

§ 9(3) Physical target encodings may be less expressive than the canonical model, but the compiler must not reduce its own source model to a bare line number before all semantic/debug consumers are finished.

§ 9(4) Synthesized code must point to the source construct that caused it where a meaningful source origin exists.

---
## § 10. Debug step points

**Governance tags:** `compiler.debug-information-v1`

§ 10(1) The canonical unit of source stepping is a compiler-internal debug step point, not a source line.

§ 10(2) A debug step point identifies a source-semantic operation that is meaningful for source-level observation.

§ 10(3) A debug step point conceptually includes:

```text
SourceRange
SemanticOperationIdentity
LexicalScopeIdentity
SourceOrigin
optional InlineChain
optional SyntheticOrigin
```

§ 10(4) This conceptual structure is compiler metadata and is not a Sec source declaration.

§ 10(5) Multiple debug step points may occur on one source line.

§ 10(6) Source columns, ranges, and target-format discriminators or equivalent mechanisms must be used where needed to distinguish multiple step points on the same line.

---
## § 11. Which operations are source step points

§ 11(1) Typical user-authored source operations that may form debug step points include:
- binding initialization;
- assignment;
- ordinary function and method calls;
- property operations;
- explicit return;
- semantically meaningful branch/loop conditions;
- explicit assertions;
- explicit process, task, thread, IPC, synchronization, or other source operations;
- `defer` registration;
- user statements inside a deferred body.

§ 11(2) Backend and compiler machinery is not automatically a user source step point.

§ 11(3) The following are ordinarily synthetic rather than independent user step points:
- register spills and reloads;
- SSA merge operations;
- ABI adaptation;
- hidden tag tests;
- ownership bookkeeping;
- cleanup-list manipulation;
- stack adjustment;
- compiler-generated bounds-check branches;
- scheduler/continuation machinery;
- generated process or IPC transport helpers.

---
## § 12. Breakpoints

§ 12(1) A source breakpoint binds to one or more executable generated code ranges that correspond to canonical debug step points.

§ 12(2) A step point may map to zero, one, or multiple generated code ranges after optimization.

§ 12(3) The compiler must not fabricate executable mappings for blank lines, comments, declarations with no runtime action, or completely eliminated source operations.

§ 12(4) A breakpoint for a completely optimized-away operation may remain unresolved.

§ 12(5) Automatic relocation of an unresolved breakpoint to another source line is debugger/UI policy, not Sec language semantics.

§ 12(6) If a debugger relocates a breakpoint, it must be able to report the actual resolved source location rather than pretending the original source point remained executable.

---
## § 13. Lexical scopes, initialization, and shadowing

§ 13(1) `Full` preserves canonical lexical-scope identity for source variables.

§ 13(2) Two source bindings with the same spelling but different semantic identities remain distinct debug variables.

§ 13(3) A local binding becomes source-level available only after successful semantic initialization commit.

§ 13(4) A fallible initializer that fails does not create an initialized source value for the destination binding.

§ 13(5) The same semantic binding may have multiple distinct availability/location intervals across its lexical lifetime due to move, reinitialization, optimization, or storage changes.

§ 13(6) Reinitialization does not require the debugger to invent a second source declaration for the same binding.

---
## § 14. Default source stepping

§ 14(1) Default source stepping prioritizes user-authored source operations and logical source frames.

§ 14(2) `Step Into` proceeds to the next relevant user-authored step point and may enter a user-authored callable.

§ 14(3) `Step Over` completes the current logical source operation and proceeds to the next relevant source step point in the same logical caller.

§ 14(4) `Step Out` proceeds until the current logical source invocation exits and its logical caller reaches the next relevant source step point.

§ 14(5) Explicit breakpoints, panic/debugger events, and other debugger stop conditions may interrupt normal stepping.

§ 14(6) Compiler-generated helpers are not made user-authored merely because machine execution passes through them.

---
## § 15. `defer`, automatic destruction, and `try`

§ 15(1) Executing a `defer` declaration performs source-visible defer registration and may be represented as a source step point.

§ 15(2) Statements in the deferred body are user-authored source operations when that body later executes.

§ 15(3) Compiler-generated defer scheduling and dispatch are synthetic.

§ 15(4) Automatic invocation of cleanup/destruction machinery is synthetic at the call site.

§ 15(5) A user-authored `free` body remains breakpoint-addressable and debuggable even when entered through implicit destruction.

§ 15(6) `try` evaluation is user-authored source semantics.

§ 15(7) The compiler-generated propagation branch produced by `try` is synthetic and attributed to the source `try` expression without becoming an extra fake source statement or frame.

§ 15(8) User-authored deferred or destruction code executed during error propagation remains ordinary debuggable user code.

---
## § 16. Properties and implicit user-authored callables

§ 16(1) A property operation at the call site is a source step point where appropriate.

§ 16(2) A user-authored property getter or setter retains canonical Sec callable identity.

§ 16(3) `Step Into` may enter a user-authored accessor.

§ 16(4) `Step Over` treats the property access as one source-level operation.

§ 16(5) Compiler-generated accessor thunks or dispatch adapters are synthetic.

---
## § 17. Branches and loops

§ 17(1) Backend branch instructions do not become source step points merely because they exist.

§ 17(2) Source conditions and user-authored bodies retain their source step points.

§ 17(3) Generated variant dispatch, switch lowering, loop backedges, and equivalent control machinery are synthetic unless they correspond to a distinct user-authored source operation.

§ 17(4) A new loop iteration returns to the appropriate source step point rather than exposing a generated backedge as a fake statement.

---
## § 18. Inlining and logical call frames

§ 18(1) Inlining must not erase canonical logical call provenance when the selected debug contract requires it and the information remains representable.

§ 18(2) Inlined user code retains callee source locations and inline call-site chains.

§ 18(3) `LineTables` and `Full` may expose logical inline frames even when no physical stack frame exists.

§ 18(4) `Step Into` and `Step Out` may operate on logical inline frames where the target/debugger integration can represent them correctly.

§ 18(5) Debug information must never require creation of a physical frame solely because a logical inline frame exists.

§ 18(6) Tail-call and frame-elimination optimizations remain legal. Logical provenance may be reduced where exact reconstruction is impossible.

---
## § 19. Many-to-many source mapping

§ 19(1) The canonical source-to-generated-code relation is not one-to-one.

§ 19(2) One source operation may map to zero, one, or multiple generated code ranges.

§ 19(3) Multiple source operations may share generated code after legal optimization.

§ 19(4) Compiler transformations that split, duplicate, sink, hoist, outline, merge, or remove code must preserve enough semantic provenance to avoid assigning a generated range to an incorrect source operation.

§ 19(5) When exact source ordering can no longer be represented, precision must be reduced instead of manufacturing misleading line transitions.

---
## § 20. Full debug variable identity

**Governance tags:** `compiler.debug-information-v1`, `semantic-ir.debug-values`

§ 20(1) `Full` describes a source binding using canonical semantic binding identity, lexical scope, source declaration provenance, and canonical Sec type identity.

§ 20(2) Relevant variable categories include:
- source parameters;
- receiver/self bindings where applicable;
- locals;
- source-visible static/global bindings;
- captures.

§ 20(3) Compiler-generated ABI parameters, hidden environment pointers, hidden result storage, task-runtime pointers, tag storage, and equivalent implementation temporaries are not automatically source variables.

§ 20(4) A raw/debugger-specific machine view may expose such implementation details separately.

---
## § 21. Semantic type identity

§ 21(1) Full debug presents canonical Sec semantic types rather than merely their lowered carrier or ABI types.

§ 21(2) This requirement applies to named/distinct types, units, structs, enums, unions, `Option`, `Result`, references, strings, arrays, slices/views, concrete generic instances, and compiler-known source-visible types.

§ 21(3) A named Sec type that lowers to the same machine representation as another type remains a distinct source/debug type.

§ 21(4) Debug metadata must not use carrier equivalence to collapse distinct nominal types.

---
## § 22. Parameter ownership and call transfer

§ 22(1) Parameter debug state follows canonical call and ownership semantics.

§ 22(2) If a named caller binding is consumed by a committed call, the caller binding becomes unavailable according to the ownership rules even if the calling convention leaves duplicate bits in caller storage.

§ 22(3) The callee parameter becomes available according to the callable's parameter semantics.

§ 22(4) Debug metadata must not imply two simultaneous source owners merely because an ABI temporarily materializes two physical copies.

---
## § 23. Partial availability

§ 23(1) `Full` must preserve Place/projection-level availability where the canonical compiler model and selected debug integration can represent it.

§ 23(2) A partially moved aggregate may retain inspectable available fields while moved, destroyed, uninitialized, or inactive fields are marked unavailable.

§ 23(3) A debugger may therefore present a value conceptually as:

```text
pair: Pair
    First  = <moved>
    Second = { ... }
```

§ 23(4) The exact presentation string is not normative.

§ 23(5) The compiler must not read old storage for an unavailable field and present it as live merely to produce a complete aggregate display.

---
## § 24. Conditional availability

§ 24(1) Compile-time control-flow analysis may classify a Place as conditionally available at a merge.

§ 24(2) At runtime, a concrete execution path has occurred.

§ 24(3) Debug metadata may expose the value only on runtime paths where that value is semantically available.

§ 24(4) If optimization removes the information needed to distinguish the runtime path correctly, the value is unavailable rather than guessed.

---
## § 25. Exact optimized value reconstruction

§ 25(1) A source value does not require a dedicated stack slot to be debuggable.

§ 25(2) Full debug may reconstruct an exact value from:
- a register;
- ordinary valid memory;
- a compiler-known constant;
- multiple register/memory pieces;
- a pure debug expression over exact machine state.

§ 25(3) Such reconstruction is valid only when it exactly represents the current Sec source value.

§ 25(4) Canonical debug-value reconstruction must not require the debugger to:
- call user code;
- evaluate a property getter;
- allocate Sec memory;
- acquire a mutex or semaphore;
- perform IPC;
- advance an iterator;
- execute destruction or cleanup;
- otherwise change program-visible state.

§ 25(5) Effectful evaluation belongs to a separate debugger expression-evaluation facility, not ordinary source-variable inspection.

---
## § 26. Structs and properties

§ 26(1) Source-level struct display describes Sec stored fields.

§ 26(2) Padding, hidden tags, ABI helper fields, runtime bookkeeping fields, and equivalent implementation representation are not source fields.

§ 26(3) Properties are not automatically displayed as stored fields.

§ 26(4) Ordinary source-variable inspection must not execute a property getter to fabricate a field-like value.

---
## § 27. Enums, unions, `Option`, and `Result`

§ 27(1) Enum values should be represented by semantic enum type and member identity where available, not only by the backing integer/string representation.

§ 27(2) A tagged union displays only its active variant and valid active payload as a live source value.

§ 27(3) Inactive variant payload storage is not a live source value.

§ 27(4) `Option[T]` is displayed according to semantic `Some(T)`/`None` state, independent of any niche lowering.

§ 27(5) `Result[T, E]` is displayed according to semantic `Ok(T)`/`Err(E)` state, independent of physical tag or error-code lowering.

---
## § 28. References and raw pointers

§ 28(1) Full debug preserves the distinction between safe shared references, safe mutable references, and raw pointers.

§ 28(2) Debug metadata preserves the canonical referenced/pointed-to type and relevant reference category/mutability authority.

§ 28(3) Safe source-level reference dereference is valid only while canonical reference/storage/lifetime semantics say the reference is valid.

§ 28(4) Old address bits do not justify showing a safe live reference after its validity ends.

§ 28(5) Debug inspection does not create a Sec borrow, pin, lifetime extension, or ownership claim.

§ 28(6) `RawPtr[T]` may expose address and pointed-to type information, but it never gains safe-reference guarantees merely because a debugger can inspect the address.

---
## § 29. Strings, arrays, slices, and opaque runtime types

§ 29(1) Full debug should provide enough language-aware metadata/integration to present a valid `string` as a semantic string when its representation is available.

§ 29(2) This requirement does not freeze the internal string representation as public Sec ABI.

§ 29(3) Fixed arrays preserve semantic element type and fixed extent.

§ 29(4) Slices/views preserve semantic element type, logical length/shape where applicable, non-owning relationship, and reference/mutability category.

§ 29(5) Opaque/compiler-known/runtime resource types such as collections, files, processes, tasks, or synchronization capabilities are not required to expose private representation as source members.

§ 29(6) Language-aware visualizers may provide logical views without turning implementation-private representation into a public ABI contract.

---
## § 30. Static/global and compile-time-known values

§ 30(1) Full debug describes source-visible static/global declarations using canonical declaration identity, source name, source provenance, and canonical Sec type.

§ 30(2) Runtime storage location is included where applicable and representable.

§ 30(3) A source value that was constant-folded may be shown as an exact constant without allocating runtime storage solely for debugging.

§ 30(4) Source-language visibility is not a confidentiality rule for debug artifacts.

---
## § 31. Device, MMIO, and potentially side-effecting storage

§ 31(1) Full debug must not assume every readable address can be observed without side effects.

§ 31(2) Device/register/MMIO storage may have read-to-clear, volatile, timing-sensitive, or otherwise target-defined side effects.

§ 31(3) Debug metadata may describe type, source declaration, storage identity, and target address while declining automatic value reading.

§ 31(4) When safe observational reading cannot be guaranteed by the target/debugger integration, the source-level value is unavailable or not safely readable.

§ 31(5) Explicit raw/device inspection is a debugger/platform operation outside safe Sec source-value guarantees.

§ 31(6) Inspectability never implies general safe editability.

---
## § 32. Generic runtime debug identity

**Governance tags:** `compiler.debug-information-v1`, `compiler.monomorphization`

§ 32(1) A generic template declaration is a source/debug declaration, not an executable runtime frame.

§ 32(2) Runtime generic debug identity is the canonical concrete specialization identity.

§ 32(3) Examples include:

```text
Identity[int32]
Stack[int32].Map[string]
```

§ 32(4) Concrete generic argument order, enclosing specialization substitutions, and phantom generic arguments that contribute to canonical specialization identity also contribute to debug identity.

§ 32(5) Explicit versus inferred generic argument spelling does not create a different specialization identity when canonical inference resolves the same concrete instance.

---
## § 33. Generic type metadata

§ 33(1) Full debug describes concrete instantiated generic types, members, variants, locals, parameters, and captures.

§ 33(2) Runtime-relevant debug metadata must not retain an unresolved type parameter where monomorphization requires a concrete runtime type.

§ 33(3) Full debug may record substitution facts such as:

```text
T = Packet
U = string
```

§ 33(4) Such metadata is tooling-only and does not create runtime generic dictionaries or type reflection.

---
## § 34. Generic identity in `LineTables`

§ 34(1) Concrete specialization callable identity is required by both `LineTables` and `Full`.

§ 34(2) A source backtrace must be able to distinguish concrete specializations where the selected debugger integration supports source frame naming.

§ 34(3) Backend mangled names and internal shared-body names do not replace the canonical Sec specialization name in the primary source view.

§ 34(4) `Full` adds complete substitution/type/variable metadata beyond this callable identity.

---
## § 35. Physical code sharing and debug identity

§ 35(1) Distinct concrete semantic specializations retain distinct debug identities even when legal monomorphization shares a physical implementation body or machine address.

§ 35(2) Machine address is not canonical Sec callable identity.

§ 35(3) A backend may realize correct specialization debugging through distinct entries, debug context metadata, adapters, physical cloning, or another correct target mechanism.

§ 35(4) A selected debug level may influence physical sharing where necessary to meet the requested debug contract, but it must not alter canonical Sec semantics, ABI contracts, or generic identity.

§ 35(5) Physical cloning is not required when the target/debug format can represent distinct semantic identities sharing one body correctly.

---
## § 36. Generic source breakpoints

§ 36(1) An ordinary source breakpoint in a generic template body binds by default to every executable concrete specialization that maps to the selected source debug step point.

§ 36(2) This includes fully inlined concrete instances where executable inline ranges survive.

§ 36(3) Template-only, unreachable, unused, or otherwise non-demanded concrete instances create no executable breakpoint target.

§ 36(4) Debug tooling and cache state must not create monomorphization demand that does not otherwise exist in the program.

§ 36(5) When a generic breakpoint is hit, the canonical debugger view must identify the actual concrete specialization where representable.

§ 36(6) Specialization-specific breakpoint filtering is debugger UX, not core source semantics.

---
## § 37. Nested generic callables and per-instantiation state

§ 37(1) Generic methods include both relevant enclosing type specialization and method-level substitutions in their debug identity.

§ 37(2) Lambdas/closures nested inside a generic specialization retain the relevant concrete enclosing specialization context.

§ 37(3) Captures in Full debug use concrete substituted types.

§ 37(4) Per-instantiation static storage remains distinct in debug metadata even when executable implementation code is shared.

§ 37(5) Separate-compilation duplicate physical definitions that are canonically coalesced do not create multiple source-level debug identities for one canonical specialization.

---
## § 38. Debug generic metadata is not reflection

§ 38(1) Debug metadata describing concrete generic types and substitutions is tooling metadata only.

§ 38(2) It introduces no Sec runtime reflection, generic dictionary, type descriptor, generic dispatch mechanism, or source API.

§ 38(3) Debug metadata may be split or stripped without changing generic execution semantics.

---
## § 39. Execution identity domains

**Governance tags:** `compiler.debug-information-v1`, `concurrency.tasks-v2`, `concurrency.threads`, `concurrency.process-v2`

§ 39(1) Debugger metadata/integration distinguishes:

```text
Process execution identity
Physical Thread execution identity
Logical Task execution identity
```

§ 39(2) These identities are related but never interchangeable.

§ 39(3) A task running on a physical worker thread does not become that thread.

§ 39(4) Task migration between worker threads does not create a new logical task identity.

---
## § 40. Logical task frames

§ 40(1) A logical task frame is not defined by the current physical worker stack.

§ 40(2) Task suspension and resumption do not destroy and recreate the source invocation merely because its physical realization changes.

§ 40(3) Logical source invocation identity, source location, inline provenance, source bindings, and semantic availability continue across suspension when the source invocation remains live.

§ 40(4) Compiler-generated task resume functions, state-machine operations, scheduler loops, and continuation helpers are synthetic in the default Sec source view.

§ 40(5) A target may realize a task using stackful fibers, heap task frames, state machines, native threads, RTOS tasks, or another valid mechanism without changing canonical debug semantics.

---
## § 41. Suspended tasks

§ 41(1) Where the selected debug integration supports logical task inspection, a suspended live task retains reconstructable logical source frames at the requested debug level.

§ 41(2) `LineTables` may provide logical frame names, source locations, inline chains, and suspension location without variable values.

§ 41(3) `Full` additionally provides source locals, parameters, captures, concrete types, and ownership availability where representable.

§ 41(4) A value moved from a native stack to a task-frame slot across suspension remains the same semantic source binding.

§ 41(5) Suspension is not itself an ownership transition.

---
## § 42. `await` debug semantics

§ 42(1) `await` is a canonical source suspension/step point.

§ 42(2) A debugger may distinguish:

```text
before waiting commit
suspended waiting
resumed after completion
```

without fabricating additional user-authored source statements.

§ 42(3) `Step Over` on `await` may suspend the task and later resume stepping at the next logical source step point in that same task.

§ 42(4) Ordinary scheduler, executor, and continuation machinery is skipped by default source stepping.

§ 42(5) Resumption does not create a second Sec source call frame.

---
## § 43. Physical thread and logical task views

§ 43(1) An explicit Sec thread has its own physical execution identity and physical source stack.

§ 43(2) Canonical Sec thread identity is distinct from a platform/native thread identifier.

§ 43(3) A debugger may present both:

```text
Task #9 running on Thread #7
Thread #7 currently executing Task #9
```

when the runtime/debugger integration knows that relation.

§ 43(4) Runtime executor frames do not become ordinary Sec source frames merely because they are present on the physical stack.

---
## § 44. Causal execution relations

§ 44(1) Spawn, await, and join relationships are causal/wait relationships, not ordinary synchronous caller/callee frames.

§ 44(2) Debug metadata may preserve relations such as:

```text
spawned by
awaiting
joining
```

§ 44(3) A debugger may present a causal or async chain separately from the normal call stack.

§ 44(4) A child task, thread, or process stack must not be fabricated as an ordinary nested source frame under the creator solely because the creator spawned or waits for it.

---
## § 45. Lifecycle ownership versus debug observability

§ 45(1) Debug observability is independent from Sec lifecycle ownership authority.

§ 45(2) Detaching or moving an owning task/thread/process handle does not by itself erase the logical debug identity of an execution that continues to run.

§ 45(3) Debug observation grants no source-level join, result, detach, termination, cancellation, or resource authority.

§ 45(4) Completed executions are not required to preserve complete historical stacks indefinitely.

§ 45(5) Post-mortem trace retention beyond ordinary live/suspended debug information is a separate crash/runtime/debugger feature.

---
## § 46. Process debug domains

§ 46(1) Each Sec process execution is a distinct debug address-space domain.

§ 46(2) Child-process frames never become physical continuation frames of the parent process.

§ 46(3) Spawn provenance and parent/child join relations may be represented as causal relations.

§ 46(4) `ProcessID` may identify a Sec process execution but is not an OS PID and grants no debugger attach authority.

§ 46(5) Automatic child-process follow/attach is debugger, target, and security policy rather than portable Sec semantics.

---
## § 47. Panic and cancellation provenance

§ 47(1) Panic reporting should preserve logical Sec source execution provenance as far as the selected panic and debug policies permit.

§ 47(2) Cooperative cancellation and cleanup use the ordinary source-stepping and ownership rules of this rulebook.

§ 47(3) Synthetic runtime unwind/cancellation machinery is hidden by default.

§ 47(4) User-authored deferred/destruction code remains source-debuggable while executed during cancellation or panic cleanup.

§ 47(5) Debugger stack walking never changes panic containment or runtime unwind semantics.

---
## § 48. Debugger-facing runtime hooks

§ 48(1) A target/runtime may provide debugger-facing hooks or metadata for task enumeration, suspended logical-frame reconstruction, task-to-worker relations, and causal provenance.

§ 48(2) Such mechanisms are not Sec source reflection APIs.

§ 48(3) Debug-only runtime state may increase artifact size or observational overhead.

§ 48(4) Debug-only hooks must not change scheduling order, synchronization semantics, ownership, lifecycle outcomes, or other Sec program semantics.

§ 48(5) `None` need not retain runtime state that exists solely to satisfy `LineTables` or `Full` debugger integration.

---
## § 49. Debug-information placement

**Governance tags:** `compiler.debug-information-v1`, `compiler.linking`

§ 49(1) Debug-information level and debug-information placement are independent configuration axes.

§ 49(2) Sec 0.1 recognizes the target-neutral placement concepts:

```text
Embedded
Separate
```

§ 49(3) These are compiler/build concepts and are not source-visible `core` declarations.

§ 49(4) The selected target may support one or both placement modes for a given debug level.

§ 49(5) `Separate` means that the primary source-level debug payload is externalized to a companion artifact. It does not require the executable to contain literally no debug-related correlation metadata.

---
## § 50. Debug-artifact correlation

§ 50(1) A separate debug artifact must be unambiguously correlated with the exact executable, library, firmware image, or other generated image it describes.

§ 50(2) Filename or path equality is not sufficient correlation.

§ 50(3) The compiler/linker artifact model must provide an image/debug artifact identity sufficient to reject stale or incompatible companions.

§ 50(4) A mismatched debug artifact must not be silently accepted as if its source mappings were correct.

§ 50(5) Each independently generated or loadable image may own a separate debug-artifact identity and companion artifact.

---
## § 51. Relocation and image-aware mapping

§ 51(1) Absolute runtime machine address is not canonical source identity.

§ 51(2) Debug mapping must tolerate relocation, ASLR, shared libraries, dynamically loaded images, and target-specific image placement.

§ 51(3) Canonical mapping conceptually combines loaded-image identity with generated location and debug provenance.

§ 51(4) Relocation does not change source function, source variable, type, or generic-specialization identity.

---
## § 52. Stripping

§ 52(1) Debug generation, debug externalization, and artifact stripping are separate operations.

§ 52(2) A build may generate `Full`, externalize it to a separate companion, and distribute a stripped runtime artifact while retaining the matching `Full` companion elsewhere.

§ 52(3) Stripping debug information may remove only state that is debug-only under the selected runtime/ABI/artifact contracts.

§ 52(4) Metadata required for runtime execution, dynamic linking, platform loading, panic policy, required stack semantics, or another non-debug contract must not be removed merely because a debugger also consumes it.

---
## § 53. Canonical source-file identity

**Governance tags:** `compiler.debug-information-v1`, `tooling.source-position`

§ 53(1) Canonical source-file identity is independent of build-host absolute filesystem location.

§ 53(2) The compiler must distinguish:

```text
SourceFileIdentity
SourceLookupPath
```

§ 53(3) Project/package/module logical identity and a stable source-relative identity participate in canonical source identity as defined by their owning rules.

§ 53(4) The debug rulebook does not invent a separate package/module identity syntax.

§ 53(5) Two different packages/modules containing the same relative file path must remain distinguishable.

---
## § 54. Source path remapping

§ 54(1) Build/debug configuration may remap source roots for emitted debug lookup/presentation paths.

§ 54(2) Path remapping must not change:
- module identity;
- source-file semantic identity;
- source ranges;
- type identity;
- generic identity;
- incremental semantic dependencies.

§ 54(3) When a stable logical source root is known, default debug emission should avoid embedding build-machine-specific absolute paths unless build policy explicitly requests them.

§ 54(4) This rule supports reproducibility, remote debugging, container/CI builds, cross-compilation, and reduced incidental path disclosure.

---
## § 55. Source-content identity

§ 55(1) The canonical debug model must associate source provenance with a content identity sufficient to detect when debugger-presented source text differs from the source content compiled into the artifact.

§ 55(2) The exact digest algorithm or physical encoding is toolchain/artifact policy rather than Sec source semantics.

§ 55(3) Source mismatch must be detectable.

§ 55(4) Whether a debugger warns, refuses source breakpoints, or proceeds after a warning is debugger policy.

§ 55(5) `Full` does not require complete source text to be embedded in the artifact.

§ 55(6) Embedded source is an optional build/tooling feature, not a fourth debug-information level.

---
## § 56. Debug information and confidentiality

§ 56(1) Debug metadata is not a confidentiality boundary.

§ 56(2) Sec source visibility such as private declarations does not require the compiler to hide corresponding debug metadata.

§ 56(3) `LineTables` may expose file paths, function identities, and concrete generic specialization names.

§ 56(4) `Full` may additionally expose local names, type structure, private member names, static bindings, and other development information.

§ 56(5) Deployment tooling must therefore treat debug artifacts as potentially sensitive development artifacts according to deployment policy.

§ 56(6) Conversely, `Full` does not require the compiler to serialize every compiler-internal fact or dead declaration.

---
## § 57. Linking, LTO, and final artifact authority

§ 57(1) Debug metadata in an intermediate object is provisional with respect to later transformations.

§ 57(2) Linking, LTO, dead stripping, outlining, inlining, coalescing, generic implementation sharing, and equivalent late transformations must update, merge, relocate, or invalidate debug metadata correctly.

§ 57(3) Final debug information must describe the actual final generated artifact.

§ 57(4) Dead code need not be retained solely to keep a breakpoint executable.

§ 57(5) Unreachable/non-instantiated declarations may be omitted when they have no generated/debug relevance.

---
## § 58. Debug artifacts versus compiler semantic databases

§ 58(1) A debug artifact is not a complete compiler semantic database.

§ 58(2) Compiler/LSP indexes, incremental caches, semantic databases, and similar artifacts may contain substantially more information than `Full` debug information.

§ 58(3) Their format stability and content are owned by their corresponding compiler/tooling rules, not by this debug rulebook.

---
## § 59. Reproducible debug generation

§ 59(1) Debug generation participates in the compiler/build reproducibility contract.

§ 59(2) Under a reproducible build configuration, incidental host state should not perturb debug artifacts when it can be normalized or excluded.

§ 59(3) Incidental state includes, where avoidable:
- temporary-directory paths;
- build-machine absolute paths;
- current timestamps;
- random identifiers;
- process IDs;
- nondeterministic traversal order.

§ 59(4) Debug-artifact identities must nevertheless distinguish incompatible generated images.

§ 59(5) Different incompatible executable realizations must not accidentally produce a matching companion identity.

---
## § 60. Target-neutral debug model

**Governance tags:** `compiler.debug-information-v1`, `platform.debug-information`

§ 60(1) Sec debug semantics are format-independent.

§ 60(2) The canonical chain is conceptually:

```text
Sec source semantics
    -> Semantic IR debug facts
    -> target-neutral Sec debug model
    -> target/backend debug encoding
    -> debugger integration
```

§ 60(3) Physical formats such as DWARF or CodeView encode the Sec contract; they do not define Sec semantics.

§ 60(4) The selected target/debugger integration may use another physical format when it satisfies the same canonical contract.

---
## § 61. Standard encodings and Sec extensions

§ 61(1) Standard debug-format constructs must be used when they accurately express the required Sec fact.

§ 61(2) Sec-specific debug extensions, companion records, or debugger integration metadata may be used when a standard encoding cannot faithfully represent a canonical Sec fact.

§ 61(3) Relevant Sec-specific facts may include ownership availability, partial availability, logical task frames, execution-causal relations, or other semantic state not faithfully representable in a generic native debugger format.

§ 61(4) A standard non-Sec-aware debugger may expose a reduced but truthful view.

§ 61(5) A target that claims canonical Sec `Full` support must provide a supported Sec-aware integration path satisfying the complete `Full` contract required by active program features.

---
## § 62. Source language identity

§ 62(1) Sec-generated debug metadata identifies the source language as Sec.

§ 62(2) A backend must not label Sec code as C, C++, Rust, or another language solely to obtain debugger compatibility.

§ 62(3) Where a physical debug format lacks an official Sec language identifier, the backend must use that format's appropriate extension/vendor mechanism or other correct target integration.

§ 62(4) Adoption of a future official Sec language identifier changes physical encoding, not the canonical semantics defined here.

---
## § 63. Debug format and version selection

§ 63(1) Sec 0.1 does not globally mandate DWARF, CodeView, PDB, or a specific format version.

§ 63(2) The CompilationPlan/backend selects a target-supported format/version that can satisfy the requested Sec debug contract.

§ 63(3) Format/version choice is not source-visible.

§ 63(4) Silent reduction of the requested Sec debug contract because a selected physical format is insufficient is forbidden.

---
## § 64. Source signatures versus ABI lowering

**Governance tags:** `compiler.debug-information-v1`, `platform.abi`

§ 64(1) Debugger source signatures use canonical Sec source semantics.

§ 64(2) Hidden result pointers, split register arguments, ABI helper parameters, tag/payload transport, closure-environment pointers, and generated ABI thunks do not become Sec source parameters.

§ 64(3) Full debug must track one source parameter/value across changing physical register, stack, memory, constant, or debug-expression locations where exact representation is possible.

§ 64(4) Canonical source return type is preserved independently of physical return ABI.

§ 64(5) Return-value inspection is guaranteed only when the exact value remains representable; debug information must not force retention of a returned value solely for inspection.

---
## § 65. Physical stack walking

§ 65(1) Physical stack recovery is a distinct target/debugger capability from source debug metadata.

§ 65(2) A target may recover physical frames using:
- frame pointers;
- call-frame information;
- platform unwind tables;
- hardware debugger knowledge;
- runtime-provided frame data;
- another valid target mechanism.

§ 65(3) DebugInformationLevel does not prescribe frame-pointer policy.

§ 65(4) `LineTables` and `Full` symbolize recovered physical frames according to their respective contracts; they do not by themselves create a mandatory stack-unwind mechanism.

---
## § 66. Debugger unwinding versus Sec panic semantics

§ 66(1) Debugger stack walking, call-frame information, and Sec panic/runtime unwinding are separate contracts.

§ 66(2) Presence of debugger stack metadata does not introduce exception semantics, panic catching, cleanup unwinding, or a general runtime unwinder.

§ 66(3) A runtime-free or hard-termination target may still provide debugger stack walking.

§ 66(4) Metadata required by non-debug runtime or ABI semantics is not debug-only and must survive debug stripping when those semantics require it.

§ 66(5) Logical inline frames and suspended-task frames are distinct from physical ABI/call-frame-information frames.

---
## § 67. Mixed-language stacks and FFI

§ 67(1) A physical stack may interleave Sec and foreign-language frames.

§ 67(2) Each frame retains its own source-language/debug semantics.

§ 67(3) Sec `Full` or `LineTables` support does not require foreign dependencies to carry equivalent debug metadata.

§ 67(4) Missing foreign debug information does not invalidate the Sec debug contract for Sec-generated code.

§ 67(5) FFI adapters and ABI thunks remain synthetic.

§ 67(6) A callback from foreign code into Sec regains canonical Sec callable identity, source types, and availability semantics for the Sec frame.

---
## § 68. Optimization and physical-frame elimination

§ 68(1) Inlining, tail calls, frame-pointer omission, frame merging, and other legal optimizations remain permitted with debug information.

§ 68(2) Logical inline/tail/task frames may be represented when accurate metadata supports them.

§ 68(3) `Full` does not require creation or preservation of a physical stack frame solely for debugging.

§ 68(4) When exact logical-frame reconstruction is not possible, information is reduced rather than fabricated.

---
## § 69. Target debug capabilities

**Governance tags:** `compiler.debug-information-v1`, `compiler.compilation-plan`

§ 69(1) Target/debug support must be modeled through semantic capability facts rather than one universal `DebugSupported` boolean.

§ 69(2) Capability resolution must distinguish at least:
- supported debug-information levels;
- supported debug-information placements;
- required source mapping capability;
- required value/type/availability representation for `Full`;
- required logical execution support for active task/debug features;
- artifact/source correlation support;
- physical stack-walk support where requested by the debugging workflow.

§ 69(3) Physical presence of DWARF, CodeView, or another format does not prove support for the complete Sec `Full` contract.

§ 69(4) Composite debug support is resolved for the concrete program and CompilationPlan from the capabilities required by reachable active features.

§ 69(5) A target without managed tasks need not provide task-debug runtime machinery merely to support Full debugging of a program that has no active task requirement.

---
## § 70. No silent debug downgrade

§ 70(1) If the user requests a debug level, placement, or active-program feature contract that the selected target cannot satisfy, compilation/build planning must diagnose the unsupported request.

§ 70(2) The compiler must not silently replace `Full` with `LineTables` or `None`.

§ 70(3) The compiler must not silently replace `Separate` with `Embedded` or the reverse.

§ 70(4) Diagnostics should identify the missing semantic capability and list supported alternatives where useful.

§ 70(5) If the backend has declared the capability but later cannot emit the promised debug contract, compilation must fail. Emitting a successful artifact with knowingly incomplete required metadata is not permitted.

---
## § 71. Debugger compatibility and expression evaluation

§ 71(1) Standard debugger compatibility should be maximized using correct standard metadata.

§ 71(2) Compatibility must never be achieved by falsely describing Sec values as another language with incompatible semantics.

§ 71(3) A Sec-aware debugger/plugin may combine native debugger facilities with Sec-specific metadata and compiler/LSP services.

§ 71(4) Full debug information does not imply a complete Sec expression evaluator inside the debugger.

§ 71(5) Evaluating arbitrary Sec expressions in a stopped program is a separate tooling feature because it may require parser/type-system, availability, borrow, property, unsafe, target-memory, and side-effect semantics.

---
## § 72. Compiler pass provenance obligation

**Governance tags:** `compiler.debug-information-v1`, `compiler.pipeline`

§ 72(1) Every compiler transformation that affects code carrying surviving source provenance must do exactly one of:

```text
preserve it accurately
transform it accurately
invalidate it explicitly
```

§ 72(2) Leaving stale debug provenance is forbidden.

§ 72(3) This obligation applies through at least:
- semantic transformations;
- monomorphization;
- constant folding;
- dead-code elimination;
- aggregate splitting;
- code motion;
- loop transformations;
- inlining and outlining;
- task/state-machine lowering;
- ABI lowering;
- MLIR/LLVM lowering;
- LTO;
- link-time coalescing;
- final artifact emission.

---
## § 73. Compiler debug verification

§ 73(1) Compiler-development/test verification must detect semantic debug contradictions where the compiler has sufficient information.

§ 73(2) Relevant invariants include:
- no live source debug value after semantic move/destruction when the binding is unavailable;
- no active source value for an inactive union variant;
- no unresolved runtime generic parameter where concrete monomorphization is required;
- no merging of distinct semantic specialization identities merely due to shared machine code;
- no synthetic-only operation presented as a user debug step point;
- no variable location outside the semantic availability interval it describes;
- no invalid source-file, source-content, or debug-artifact identity relation.

§ 73(3) Violation of these compiler invariants is a compiler defect, not a source-program error.

---
## § 74. Conformance matrix

§ 74(1) Compiler conformance testing must cover representative combinations of:

```text
None
LineTables
Full
```

and representative optimization levels and supported target families.

§ 74(2) Debug-enabled and debug-disabled builds of the same semantic program must preserve Sec-observable program semantics.

§ 74(3) `LineTables` and `Full` require independent tests. Passing `Full` tests does not prove that the `LineTables` emission path is correct.

§ 74(4) Conformance must test optimized debug information rather than only unoptimized stack-slot-heavy builds.

---
## § 75. `LineTables` conformance categories

§ 75(1) `LineTables` conformance must cover at least:
- file/line/range mapping;
- multiple same-line debug step points;
- source function identity;
- concrete generic specialization identity;
- inline call chains;
- breakpoint binding;
- Step Into/Over/Out behavior;
- synthetic code suppression;
- fully optimized-away source locations;
- relocation/ASLR-aware symbolization where applicable;
- separate debug-artifact correlation where supported;
- source-content mismatch detection.

---
## § 76. `Full` conformance categories

§ 76(1) `Full` conformance includes all `LineTables` categories.

§ 76(2) It additionally covers at least:
- parameters;
- locals;
- shadowed bindings;
- reinitialization;
- move-to-unavailable transitions;
- partial moves;
- destruction transitions;
- inactive union variants;
- exact constants without storage;
- piecewise aggregate locations;
- optimized-out variables;
- named/distinct types and units;
- enum members;
- `Option` and `Result` semantic variants;
- `ref`, `ref mut`, and `RawPtr` distinction;
- strings;
- arrays and slices/views;
- generic substitutions;
- per-instantiation static state;
- task locals across suspension;
- MMIO/device not-safely-readable behavior.

---
## § 77. Compiler, backend, and debugger test layers

§ 77(1) Debug-information conformance requires more than checking for the presence of physical debug sections.

§ 77(2) The complete strategy includes:

```text
compiler structural debug-model tests
physical encoding/backend tests
supported debugger/integration tests
```

§ 77(3) Structural tests verify canonical compiler facts before physical encoding.

§ 77(4) Encoding tests verify target-format realization.

§ 77(5) Debugger/integration tests verify externally observed source behavior such as frame identity, current source location, value visibility, specialization identity, logical task identity, and artifact/source correlation.

---
## § 78. Standard debugger versus canonical Sec debugger tests

§ 78(1) Standard native-debugger compatibility and canonical Sec-aware debug conformance are distinct test targets.

§ 78(2) A standard debugger may provide a truthful reduced view without implementing all Sec-specific availability or logical-task semantics.

§ 78(3) A target claiming canonical Sec `Full` support must have a supported Sec-aware integration path that satisfies the complete required contract for active program features.

§ 78(4) Capability declarations and tests must not conflate these two support levels.

---
## § 79. Negative debug capability tests

§ 79(1) The compiler-test suite must include unsupported debug-level, placement, and active-feature combinations.

§ 79(2) Unsupported combinations are compile/build-time diagnostics.

§ 79(3) A compiler must not downgrade silently.

§ 79(4) Diagnostics should identify the missing capability and the closest supported alternatives where that information is known.

---
## § 80. Artifact and source-correlation tests

§ 80(1) Reproducibility/path tests must build equivalent source from different host workspaces under the same canonical path-remapping policy and verify that incidental build-root paths do not alter the reproducible debug result.

§ 80(2) Companion mismatch tests must verify that a debug artifact from one incompatible build is rejected for another executable image.

§ 80(3) Source mismatch tests must verify that tooling can detect when local source content differs from the source used to build the debugged artifact.

---
## § 81. Relationship to `semantic_ir.md`

§ 81(1) `semantic_ir.md` remains authoritative for source provenance, semantic declaration identity, Place identity, availability state, and synthesized-origin facts.

§ 81(2) This rulebook consumes those facts for debugging.

§ 81(3) The compiler must preserve the relevant facts until all debug consumers that require them are complete.

§ 81(4) This rulebook does not create a second availability lattice or second declaration-identity system.

---
## § 82. Relationship to monomorphization

§ 82(1) `compiler/monomorphization.md` remains authoritative for generic template, concrete instantiation, plan realization, plan entry, and implementation-body identity.

§ 82(2) This rulebook consumes canonical concrete specialization identity.

§ 82(3) Debug identity never replaces monomorphization identity and machine-code sharing never defines it.

---
## § 83. Relationship to concurrency rulebooks

§ 83(1) `tasks.md`, `await.md`, `threads.md`, and `processes.md` remain authoritative for execution identities, lifecycle, suspension, migration, spawn, await, join, detach, and cancellation semantics.

§ 83(2) This rulebook consumes their canonical identities and causal relations for debugger presentation.

§ 83(3) Debug metadata does not create new concurrency lifecycle authority or change scheduling/synchronization semantics.

---
## § 84. Relationship to panic

§ 84(1) `errors/panic.md` remains authoritative for panic semantics and target panic policy.

§ 84(2) Debugger stack walking is observational and does not imply a general Sec panic unwinder.

§ 84(3) A target may provide useful source stack debugging while using a panic strategy that halts, resets, aborts, or otherwise does not unwind source frames.

---
## § 85. Relationship to compiler pipeline and linking

§ 85(1) `compiler_pipeline.md` remains authoritative for compiler phase ordering and optimization legality.

§ 85(2) Pipeline passes that transform code with surviving source provenance must obey § 72.

§ 85(3) `compiler/linking.md` remains authoritative for image construction, LTO/linking, symbol/link identities, and final artifact verification.

§ 85(4) Linking must include debug metadata/artifact correlation among final-artifact obligations when the selected CompilationPlan requests debug information.

---
## § 86. Relationship to compiler testing

§ 86(1) This rulebook defines what debug-information behavior and conformance categories must be tested.

§ 86(2) `rules/compiler_testing.md` owns the compiler-test harness, test discovery, execution, golden-file policy, debugger-driving infrastructure, target matrices, regression organization, and test-result workflow.

§ 86(3) Debug-information conformance is a mandatory compiler-testing category.

---
## § 87. Relationship to incremental compilation

§ 87(1) Debug configuration is artifact-generation state rather than Sec program-semantic identity.

§ 87(2) Debug level, placement, path/source correlation policy, debug-only runtime hooks, physical debug format, and related artifact options participate in relevant backend/artifact/link cache identities.

§ 87(3) Changing `None` to `Full` does not by itself change canonical type, generic, ownership, or source-program semantics.

§ 87(4) Detailed invalidation and reuse policy belongs to `rules/incremental_compilation.md`.

---
## § 88. Diagnostics

§ 88(1) Debug capability diagnostics must be precise and source/build actionable.

§ 88(2) A diagnostic should distinguish at least:
- unsupported requested level;
- unsupported placement;
- missing active-program logical execution support;
- unsupported physical debug-format realization;
- artifact/debug companion mismatch;
- source-content mismatch where surfaced by build/debug tooling.

§ 88(3) Unsupported target capability is a user/build configuration diagnostic.

§ 88(4) A backend that claims support but emits inconsistent or stale required metadata has a compiler defect.

§ 88(5) Diagnostics must not suggest that enabling or disabling debug information changes source ownership, panic, concurrency, or type semantics.

---
## § 89. Sec 0.1 exclusions

§ 89(1) Sec 0.1 does not standardize a source-level debugger control API.

§ 89(2) Sec 0.1 does not standardize a debugger expression evaluator.

§ 89(3) Sec 0.1 does not standardize a remote debugger protocol, symbol server, source server, or debuginfod-like service.

§ 89(4) Sec 0.1 does not require embedded source text.

§ 89(5) Sec 0.1 does not require a universal debug physical format or format version.

§ 89(6) Sec 0.1 does not require frame pointers or a general runtime unwinder solely for debugging.

§ 89(7) Sec 0.1 does not expose runtime generic reflection merely because debug artifacts contain generic metadata.

§ 89(8) Sec 0.1 does not guarantee arbitrary debugger mutation of Sec values while preserving language invariants.

---
## § 90. Implementation completeness gate

§ 90(1) The debug-information implementation is not complete merely because the backend emits DWARF, CodeView, or another physical debug section.

§ 90(2) A target/debug level is complete only when the target-specific path satisfies the canonical Sec contract claimed for that level and active program feature set.

§ 90(3) At minimum, implementation completion requires:
- canonical source provenance through final emission;
- correct source step points;
- correct concrete generic identities;
- correct variable/type/availability metadata for `Full`;
- correct logical task-frame integration where required;
- exact companion/image correlation for separate artifacts;
- source-content correlation;
- correct final-artifact mapping after optimization/LTO/linking;
- negative capability diagnostics;
- conformance tests defined by §§ 74–80.

§ 90(4) Implementation status and evidence belong in `governance/compiler.yaml` under `compiler.debug-information-v1`.

---
## § 91. Normative summary

§ 91(1) Sec 0.1 has exactly three debug-information levels: `None`, `LineTables`, and `Full`.

§ 91(2) Debug information is observational metadata and never changes Sec program semantics.

§ 91(3) Source mappings are range-, origin-, and semantic-operation-aware rather than line-number-only.

§ 91(4) `Full` follows canonical Sec type identity and ownership/lifetime availability; stale machine bits never resurrect a source value.

§ 91(5) Concrete generic specializations retain distinct debug identity independent of physical implementation sharing.

§ 91(6) Tasks, physical threads, and processes remain distinct debug execution identities; suspended tasks retain logical source frames independent of worker stacks.

§ 91(7) Debug level, placement, and stripping are separate axes.

§ 91(8) Separate debug artifacts require exact image correlation and source mappings require source-content correlation sufficient to detect mismatch.

§ 91(9) The canonical debug model is target-neutral; physical encodings such as DWARF or CodeView are backend realizations.

§ 91(10) Debugger physical stack walking does not imply Sec runtime/panic unwinding.

§ 91(11) Compiler transformations must preserve, transform, or explicitly invalidate debug provenance and must never knowingly emit stale metadata.

§ 91(12) No unsupported requested debug contract may be silently downgraded.
