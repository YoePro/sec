# Sec Interrupts

- **Status:** Normative
- **Created:** 2026-08-29
- **Last updated:** 2026-08-29
- **Document revision:** 1
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `fafd8cb`
- **Canonical path:** `rules/platform/interrupts.md`

## 1. Purpose

This rulebook defines interrupt semantics for Sec 0.1.

It owns interrupt identities and handler binding; interrupt execution roots and contexts; priority, preemption, nesting, masking, and reentrancy; interrupt classes; source/route/entry, pending/active, claim/dispatch/completion semantics; interrupt configuration capabilities; ISR-safe execution; interrupt/shared-state integration; stack/runtime/bounded-execution requirements; platform/startup/link integration; and interrupt diagnostics/tooling/completion criteria.

It does not redefine hardware-register transaction semantics, hardware-register access legality/ordering/completion/fault behavior, the Sec concurrency memory model, atomic memory ordering, race/deadlock/stack algorithms, cleanup ordering, general initialization/linking semantics, or platform-registry file syntax.

Canonical dependencies include:

```text
rules/foundations/attributes.md
rules/platform/platform_model.md
rules/platform/target_profiles.md
rules/platform/hardware-register-access.md
rules/memory/memory_model.md
rules/memory/destruction.md
rules/control-flow/defer.md
rules/compiler/initialization.md
rules/compiler/linking.md
rules/analysis/isr_analysis.md
rules/analysis/stack_analysis.md
```

Dependency direction is normative: interrupts may consume canonical hardware-access, concurrency, stack, race, deadlock, FFI, runtime, initialization, and linking facts, but must not redefine them.

# Part I — Interrupt identity and handler binding

## 2. Bound handlers

The ordinary bound form is:

```sec
@interrupt(vector: Interrupt.Timer0)
fn Timer0Handler() void {
}
```

`@interrupt(vector: ...)` binds the function to one resolved interrupt identity under the active `CompilationPlan` and implies `@isr`.

`@isr` remains valid without `@interrupt` for externally bound entries, platform/startup integration, FFI entry arrangements, and explicit ISR testing infrastructure.

## 3. ISR entries are execution roots

An ISR entry is not an ordinary caller-driven function.

```text
hardware / interrupt controller
        ↓
interrupt entry
        ↓
ISR execution root
        ↓
reachable call graph
```

The ISR execution context propagates through ordinary synchronous calls until a canonical context transition moves later work to another execution context.

## 4. ISR entries are not ordinary-callable

A bound ISR entry, and an unbound `@isr` entry, must not be called as an ordinary Sec function.

Reusable logic belongs in ordinary helpers:

```sec
@interrupt(vector: Interrupt.Timer0)
fn Timer0Handler() void {
    HandleTimer()
}

@interruptSafe
fn HandleTimer() void {
}
```

Testing does not redefine an ISR entry as an ordinary callable.

## 5. Platform-owned interrupt identity

Preferred bindings use named platform identities such as:

```sec
@interrupt(vector: Interrupt.UART1)
fn UART1Handler() void {
}
```

Raw numeric binding remains available for new/custom targets:

```sec
@interrupt(vector: 15)
fn CustomHandler() void {
}
```

Raw numeric syntax still resolves through canonical platform interrupt metadata and does not bypass validation.

A numeric vector is not universally the complete interrupt identity. Canonical identity may include an interrupt domain, logical source, route, and physical entry.

## 6. Binding and ABI validation

Within one resolved `CompilationPlan`, an exclusive interrupt binding has one active owner unless the platform explicitly defines a shared generated-dispatch arrangement.

Reserved, runtime-owned, OS-owned, or otherwise non-bindable interrupts reject user binding. `unsafe` does not waive bindability.

The target/platform owns the machine ISR ABI and legal source-visible handler shape. A common source shape is `fn Handler() void`, but target-specific context may be exposed where the platform contract allows it.

Normal source completion means: perform required Sec cleanup, satisfy required interrupt-controller lifecycle, and perform the target interrupt return. The programmer does not write the machine return sequence in ordinary Sec ISR source.

# Part II — Priority, preemption, nesting, masking, and reentrancy

## 7. Priority is semantic, not universally numeric

Sec defines no universal rule that larger or smaller numbers mean higher priority.

The compiler consumes resolved semantic relations such as:

```text
MayPreempt(A, B)
CannotPreempt(A, B)
MayNest(A, B)
CannotNest(A, B)
MayReenter(A)
```

Priority need not form one total order. Targets may have priority classes, domains, non-maskable classes, equal-priority groups, routing contexts, or target-specific arbitration.

Preemption is directional. Nesting and self-reentrancy are distinct.

## 8. Repeated occurrence is not reentrancy

A new occurrence while an ISR is active may become pending, count, coalesce, be ignored, execute later, or reenter immediately according to the controller contract.

Sec does not equate repeated occurrence with simultaneous handler invocation.

## 9. Masking

Masking is not one universal boolean. Targets may expose global masks, thresholds, source masks, class masks, controller groups, and unmaskable classes.

Masking changes effective execution relations and may contribute facts to preemption, nesting, race, deadlock, and stack analysis.

Masking does not create unrelated capabilities: it does not make allocation, blocking, panic, incompatible runtime, illegal FFI, or illegal hardware access valid.

Targets may change mask/priority state on interrupt entry. Normal interrupt return restores the target-defined execution state.

# Part III — Interrupt classes, faults, pending, active, and software trigger

## 10. Interrupt classes

Platforms may classify roots using concepts equivalent to:

```text
ExternalIRQ
Timer
SoftwareTriggered
Fault
Exception
NonMaskable
TargetDefined
```

Named identities such as `Interrupt.Timer0`, `Interrupt.NMI`, or `Interrupt.HardFault` carry resolved class/ABI/binding/execution metadata rather than being mere integer aliases.

## 11. Faults and exceptions

Processor faults, traps, and exceptions do not automatically become ordinary Sec `Result` errors.

A fault may be handler-bindable, runtime-owned, OS-owned, recoverable, fatal, reset-producing, or process-terminating according to the selected target/execution environment.

Non-maskable execution remains subject to ISR stack/runtime/allocation/blocking/memory/synchronization/FFI constraints.

## 12. Pending and active

`Pending` and `Active` are distinct controller/execution states.

Pending does not imply a queued event count. A controller may preserve one pending bit, a count, a queue, or another device-defined representation.

Reading or writing hardware state that represents pending/active is hardware-register access. This rulebook owns the lifecycle meaning; `hardware-register-access.md` owns the physical/logical access operation.

## 13. Software-triggered interrupts

Targets may expose set-pending, software-interrupt instructions, inter-processor interrupts, trap instructions, or other software trigger mechanisms.

Triggering is not an ordinary function call to the handler. Priority, masking, routing, pending, and context transition semantics still apply.

The platform may distinguish synchronous trap/exception entry from asynchronous interrupt delivery and may expose target-specific handler context where appropriate.

# Part IV — Source, route, claim, dispatch, and completion

## 14. Source, route, and entry are distinct

The canonical model distinguishes:

```text
InterruptSource
InterruptRoute
InterruptEntry
```

A simple target may map them one-to-one. A multiplexed controller may route many logical sources through one CPU entry.

A named binding describes the logical interrupt identity and may resolve to controller source IDs, routes, physical entries, and generated dispatch requirements.

## 15. Delivery planes

Where applicable, distinguish:

```text
peripheral interrupt-generation enable
controller source/route enable
CPU/execution-context masking
```

Disabling one does not automatically disable or clear the others. Disable does not imply clear-pending.

Peripheral condition/assertion, controller pending, claim/accept, active/in-service, peripheral source service, and controller completion are separate lifecycle concepts even when one target collapses some stages.

## 16. Trigger mode and signal polarity

The platform may provide edge/level trigger metadata where relevant.

Signal polarity is not compiler semantics. Sec does not infer or reinterpret active-high/active-low, asserted/deasserted, enabled/disabled, or equivalent domain meaning from a name or electrical polarity. Driver/application code owns such interpretation.

## 17. No universal acknowledge operation

Sec does not define one universal `acknowledge` that combines all lifecycle behavior.

Where applicable, distinguish:

```text
Claim / Accept
Peripheral Source Service
Complete / EndOfInterrupt
```

Claim may select a pending source and mutate controller state. Peripheral source servicing remains device semantics. Controller completion remains controller lifecycle.

## 18. Completion before return

A platform may require:

```text
Complete(controller_operation)
    before
InterruptReturn
```

or a stronger service/complete ordering.

`interrupts.md` owns the lifecycle obligation. `hardware-register-access.md` owns how the relevant hardware operation becomes ordered and complete, including any target-required barrier, read-back, stronger access, or intrinsic.

## 19. Generated controller lifecycle

Mandatory controller lifecycle fully defined by the resolved platform contract may be compiler/platform-generated around the logical source handler.

This may include claim, source dispatch, controller completion/EOI, and ABI adaptation.

Peripheral-specific source servicing is not automatically compiler-owned merely because an interrupt source exists.

A spurious/no-source claim must not fabricate a logical handler invocation.

# Part V — Programmer-controlled interrupt configuration

## 20. Handler binding does not activate delivery

Declaring:

```sec
@interrupt(vector: Interrupt.Timer0)
fn Timer0Handler() void {
}
```

does not by itself enable the peripheral source, enable the controller route, unmask interrupts, set priority, clear pending state, start the peripheral, or configure routing.

Platform/project startup may perform such configuration only when it is explicitly part of the resolved configuration.

## 21. Control capabilities

A resolved interrupt identity may expose capabilities equivalent to:

```text
CanEnableDelivery
CanDisableDelivery
CanSetPriority
CanSetPending
CanClearPending
CanSoftwareTrigger
CanRoute
CanSetAffinity
CanMask
```

Not every interrupt/class supports every capability.

`Interrupt.Timer0` is first a canonical platform identity; it need not be implemented as ordinary mutable runtime storage.

## 22. Enable, pending, priority, trigger, and routing

Enable/disable affects exactly the delivery plane defined by the operation contract and does not silently clear pending, start the peripheral, or change unrelated configuration.

ClearPending is distinct from disable, peripheral servicing, and controller completion.

SetPending and SoftwareTrigger may be distinct target capabilities.

Interrupt configuration may be fixed, configured-before-execution, or runtime-mutable. Analyses may treat a configuration as immutable only when the `CompilationPlan` and reachable code prove that it cannot change.

Runtime priority or routing changes widen/invalidate dependent execution proofs.

## 23. Structured masking

The safe common masking abstraction preserves and restores previous relevant state:

```text
saved = SaveAndApplyMask(...)
execute body
Restore(saved)
```

It is not equivalent to blindly disabling and later enabling everything.

Restoration must occur on ordinary Sec exits including fallthrough, return, Result/error return, and canonical cleanup. Nested scopes must not re-enable an interrupt excluded by an outer scope.

A saved mask-state capability represents real previous execution state and must not be arbitrarily fabricable if that would permit invalid restoration.

Low-level target-specific enable/disable/set-mask primitives may still exist.

# Part VI — ISR-safe execution

## 24. Full reachable behavior is checked

ISR validity includes direct calls, indirect calls, generic specializations, callbacks, FFI callbacks, runtime calls, `defer`, destruction, cleanup, error paths, compiler-generated helpers, and generated target/platform wrappers.

## 25. `@interruptSafe`

`@interruptSafe` is a compiler-verified ordinary-callable contract, not a programmer assertion.

Ordinary helpers without the attribute may still be used from ISR context when the compiler proves an equivalent contract for the relevant `CompilationPlan`.

Indirect calls require every reachable target to be safe or a sufficient verified callable contract. Generic code is checked per relevant specialization.

## 26. Sec 0.1 ISR baseline

Sec 0.1 locks:

```text
@isr
    => noPanic
    => noAlloc
    => noBlock

@interruptSafe
    => noPanic
    => noAlloc
    => noBlock
```

Targets/profiles do not weaken these three baseline guarantees.

Explicit `Result`/`try` error handling remains available when all reachable handling is ISR-safe and handled before normal interrupt return.

## 27. Allocation, blocking, and progress

No-allocation does not prohibit stack/static/caller-provided/fixed-capacity/already-owned storage when the actual operation does not allocate.

Blocking, sleeping, scheduler parking, blocking I/O, or equivalent operations are forbidden.

Non-sleeping is not sufficient: a spin loop can still be unbounded or deadlock because progress depends on a preempted context.

Deferred work handoff is allowed when the handoff itself is ISR-safe. Later work executes in its own context.

`spawn` is allowed only if the spawn operation itself is noAlloc/noBlock/runtime-compatible. An `await` that suspends/parks violates the Sec 0.1 ISR contract.

## 28. Runtime, TLS, hardware access, unsafe, and FFI

NoAlloc/noBlock are necessary but not sufficient. ISR code may still be invalid because it requires thread-only runtime state, incompatible TLS, non-reentrant runtime state, unavailable memory, or incompatible platform helpers.

TLS semantics are target/runtime-defined. The compiler does not fabricate thread context.

Hardware-register access from ISR is valid only when the canonical hardware-access contract permits it in the current ISR context.

`unsafe` does not waive noPanic/noAlloc/noBlock, runtime restrictions, hardware-access context restrictions, or supporting proof requirements.

Foreign calls require sufficient canonical ISR/FFI contracts. Missing mandatory facts are `Unproven`. Synchronous callbacks from foreign code remain in the current ISR context.

## 29. Cleanup

`defer` and automatic destruction are allowed when all reachable cleanup is ISR-safe.

Reachable error/cleanup/destruction paths participate in the ISR proof. Only proven unreachable paths may be excluded.

Compiler-generated helpers are part of ISR legality; a source operation is invalid if its required lowering necessarily invokes ISR-incompatible behavior.

# Part VII — Shared state, atomics, and synchronization

## 30. Canonical concurrency model

Interrupt execution participates in Sec's ordinary concurrency memory model.

Interrupt entry/return does not itself establish universal acquire/release/publication semantics.

Volatile physical access is not synchronization.

Ordinary shared mutable state between ISR and other execution is legal only where conflicting accesses are proven unable to overlap or are correctly synchronized.

## 31. Atomics

Atomic operations are usable in ISR only when the complete target lowering satisfies the ISR contract.

A source-level atomic is not automatically ISR-safe if it requires blocking runtime, a hidden mutex, incompatible helper, or unsupported-width emulation.

Interrupts use the ordinary Sec atomic memory-order model. Payload-plus-flag publication must use ordering that actually establishes the required visibility relation.

## 32. Masking as exclusion

Interrupt masking may prove race-free ordinary shared access only for the execution contexts that the resolved platform proves are excluded.

Local interrupt masking does not automatically exclude another core/hart, a non-maskable interrupt, DMA, a device, or other external actors.

Blocking synchronization is forbidden by noBlock. Nonblocking synchronization may be legal when lowering, race, deadlock, ownership, and progress proofs succeed.

A spin lock is not ISR-safe merely because it does not sleep when its owner cannot run while the ISR is active.

Preallocated bounded queues/ring buffers are a canonical useful ISR handoff pattern when their contracts satisfy the ISR requirements.

Race and deadlock analyses remain the owners of the final proofs.

# Part VIII — Stack, runtime resources, bounded execution, failure, and cleanup

## 33. Interrupt stack domains

Interrupt execution uses a resolved physical stack domain. Targets may use shared stacks, dedicated interrupt stacks, per-core/per-hart stacks, banked stacks, or target-defined stack switching.

The canonical stack analysis computes stack requirements using interrupt roots, nesting, physical stack domains, ABI, and lowering facts.

## 34. Stack accounting

Nested ISRs sharing one physical stack may contribute simultaneously. Distinct physical stack domains are not blindly summed.

Machine stack proof includes applicable hardware exception frames, saved registers, alignment, generated wrappers/dispatchers, claim/completion wrappers, ABI adaptation, and stack switching.

Semantic and machine stack requirements remain distinct where the compiler uses both.

For finite budgets:

```text
Exact(N) <= budget        valid
UpperBound(N) <= budget   valid
Exact(N) > budget         invalid
UpperBound(N) > budget    unproven unless overflow is otherwise proven
Unknown                   unproven
Unbounded                 invalid where finite bound is required
```

`UnknownLimit` is not `NoDeclaredLimit`, and neither means infinite physical resources.

Mandatory stack safety is not weakened into UB because a target lacks guard pages/runtime checks.

Recursion is not syntactically forbidden but must satisfy the resource proof. Generic specialization may change stack legality.

## 35. Runtime and bounded execution resources

Runtime availability is distinct from stack.

`noBlock` does not imply bounded execution. A resolved ISR profile may require a bounded-execution proof independently.

Quantitative realtime constraints, where present, use typed units/resources. Interrupt latency, own execution time, masking duration, and response time including interference are separate quantities.

This rulebook does not define a universal WCET algorithm. The owning timing/resource analysis supplies a proof where required.

Generated wrappers, controller lifecycle, compiler helpers, cleanup, and target ABI behavior contribute to applicable resource proofs.

## 36. Failure and cleanup

Ordinary Sec cleanup remains in force on normal ISR exits and must complete before target interrupt return according to the canonical ordering constraints.

Scope exit does not implicitly commit hardware register shadow state or arbitrary MMIO state.

Panic is forbidden, so fallible helpers require explicit ISR-safe handling.

Machine faults/exceptions are not ordinary Result/error exits and do not automatically guarantee ordinary cleanup unless the owning target fault contract explicitly establishes stronger behavior.

Hardware-access fault behavior remains owned by `hardware-register-access.md`; interrupt rules determine whether that fault behavior is legal in the current ISR profile.

# Part IX — Platform integration, startup, linking, and generated artifacts

## 37. Platform/device interrupt knowledge

Platform and Device definitions provide canonical interrupt facts consumed by `CompilationPlan`.

A resolved interrupt definition may include facts equivalent to:

```text
Identity
InterruptDomain
SourceIdentity
Route
Entry
Class
BindingPolicy
EntryABI
PriorityModel
MaskingModel
PendingModel
ActiveModel
ClaimModel
CompletionModel
TriggerModel
StackDomain
ISRProfile
PlatformAvailability
Provenance
```

Exact registry/file syntax is not defined by this rulebook.

## 38. Resolution and validation

Named and raw bindings resolve through the frozen `CompilationPlan` before ISR semantic verification and lowering.

Platform/device interrupt registry data is validated before it becomes authoritative. Invalid examples include duplicate identities, conflicting vectors, unknown controllers, invalid routes, invalid priority metadata, unknown stack domains, invalid ABI, invalid lifecycle relationships, or reserved sources marked user-bindable.

Each build Variant has its own concrete interrupt catalog and plan.

## 39. Generated vector entries, wrappers, and dispatchers

A logical interrupt source need not correspond one-to-one with a physical vector slot.

The compiler/platform may generate direct vector entries, minimal wrappers, shared dispatchers, claim/source resolution, controller completion, and ABI adapters.

Generated wrapper/dispatcher identity is distinct from the logical source-handler identity and is deterministic from canonical plan facts.

On direct-vectored targets the compiler uses the simplest legal direct form; it does not generate unnecessary dispatcher layers.

## 40. Vector tables and linking

A target vector table is a platform binary artifact, not necessarily an ordinary Sec array.

The target/link environment owns layout, encoding, alignment, placement, relocation, sections, startup objects, and retention mechanisms.

Vector tables, generated wrappers/dispatchers, required platform data, and source handlers are explicit canonical link roots/reachable artifacts. A handler does not become dead merely because no ordinary Sec function calls it.

Source visibility remains separate from binary reachability. Sec 0.1 does not require a general source-level weak-handler feature.

Unbound interrupt policy is platform-defined and deterministic.

## 41. Startup

Interrupt infrastructure may contribute explicit requirements to `ProgramInitializationPlan`, including vector-table installation/copying, vector-base setup, exception-entry installation, interrupt-stack establishment, or dispatcher-state initialization.

Infrastructure setup does not imply enablement. Handler binding does not activate delivery.

The platform defines whether interrupts may execute during early startup, after runtime establishment, after explicit configuration, or only later. ISR analysis uses the runtime/context facts actually available in the execution phase where the interrupt may occur.

## 42. Determinism, variants, cross-compilation, and LTO

For identical source, platform registry state, and `CompilationPlan`, interrupt resolution and generated artifacts are deterministic and independent of file order, parallel scheduling, link-input order, or cache iteration order.

Variant-specific interrupt artifacts do not leak between builds.

Cross-compilation uses target interrupt ABI/controller/vector/stack/link facts and never host defaults.

LTO may inline/specialize/remove redundant wrapper layers while preserving logical source identity, entry ABI, lifecycle, completion, roots, and required vector entries.

Relevant platform changes invalidate dependent ISR proofs, generated artifacts, initialization requirements, and link work.

# Part X — Diagnostics, tooling, tests, and completion

## 43. Diagnostics

Diagnostics lead with logical interrupt identity and source handler rather than generated symbol names.

The primary location should be the actual semantic cause where possible, with the ISR root/profile as related information.

Diagnostics are context-specific and distinguish causes such as:

```text
unknown/unavailable interrupt
reserved/non-bindable interrupt
duplicate binding
ABI mismatch
invalid route
priority/masking conflict
reentrancy/nesting violation
may panic
may allocate
may block
runtime incompatibility
illegal hardware access
insufficient FFI contract
stack budget exceeded
stack proof missing
race/deadlock proof missing
bounded-execution proof missing
completion obligation unsatisfied
target cannot materialize required interrupt ABI/lifecycle
```

The canonical result distinction is:

```text
Invalid
    violation proven

Unproven
    required positive proof missing/insufficient

Pending
    tooling result not yet current/complete
```

`Pending` is not a final semantic result and must not be shown as completed `Valid`.

Platform/device provenance is retained.

## 44. LSP and `sec analyse`

LSP uses the same resolved interrupt `CompilationPlan` semantics as a real build.

Interactive/Standard/Deep analysis levels may vary cost, but mandatory proven violations and failures to prove required ISR safety remain need-to-know results.

Useful hover/navigation may expose logical identity, handler, class, route/entry, priority/preemption facts, stack proof/budget, ISR validity, generated dispatcher status, platform provenance, and representative unsafe call paths.

`sec analyse` includes ISR analysis in canonical all-analysis behavior.

Separate-compilation summaries may carry ISR-relevant facts without knowing future application roots. Stale/incompatible summaries may not serve as positive safety proofs.

Incremental invalidation follows semantic dependencies.

## 45. Required test families

A conforming implementation maintains regression coverage for at least:

### Binding and identity
- named interrupt resolution;
- raw numeric resolution through canonical metadata;
- unknown/reserved/non-bindable rejection;
- duplicate binding rejection;
- mutually exclusive target bindings;
- ordinary-call rejection for ISR entries.

### Priority and execution
- directional preemption;
- nesting versus self-reentry;
- pending while active;
- non-maskable execution;
- exact mask-state restoration;
- runtime priority/route invalidation.

### Classes and faults
- external IRQ;
- software-triggered interrupt;
- synchronous trap/exception;
- non-maskable class;
- system-owned exception rejection;
- target-specific handler context;
- machine fault not treated as ordinary Result.

### Routing and lifecycle
- direct vector;
- shared entry;
- generated dispatcher;
- claim/source dispatch;
- spurious/no-source claim;
- peripheral service distinct from controller completion;
- completion before return.

### Configuration
- handler declaration does not enable delivery;
- enable/disable do not imply clear-pending;
- fixed-priority mutation rejection;
- SetPending versus SoftwareTrigger distinction;
- structured mask restoration.

### ISR-safe execution
- panic/allocation/blocking rejection;
- preallocated operation acceptance;
- transitive `@interruptSafe` proof;
- compiler-proven ordinary helper;
- unsafe does not waive ISR requirements;
- FFI callback context propagation;
- safe defer/destruction;
- unsafe cleanup rejection;
- generated helper legality.

### Shared state
- volatile is not synchronization;
- target-supported atomic acceptance;
- incompatible atomic emulation rejection;
- correct publication ordering;
- single-core masking proof;
- local masking insufficient for another core/DMA;
- blocking mutex rejection;
- nonblocking synchronization proof;
- spin-on-preempted-owner rejection;
- preallocated ISR queue/ring-buffer handoff.

### Stack/resources
- dedicated/shared stack domains;
- nested accounting;
- generated ABI/wrapper cost;
- finite budget proof;
- unknown => Unproven;
- bounded versus unbounded recursion;
- noBlock versus bounded execution.

### Startup/linking
- vector-table emission/retention;
- zero-caller ISR retention;
- wrapper/dispatcher retention;
- source-private ISR binary reachability;
- startup infrastructure;
- early ISR profile;
- binding does not enable delivery;
- LTO preservation;
- Variant isolation;
- cross-compilation target ABI isolation.

### Tooling/determinism
- logical identity diagnostics;
- representative transitive cause path;
- Invalid versus Unproven;
- Pending never exposed as Valid;
- LSP/compiler resolution parity;
- dependency-driven invalidation;
- summary compatibility;
- deterministic analysis/artifact identity.

Confirmed safe patterns, including safe MMIO, preallocated queues, nonblocking synchronization, masking-protected single-core state, lock-free atomic handoff, safe callbacks, safe defer/destruction, bounded stack, target-valid runtime helpers, and generated controller dispatchers, should be preserved as false-positive regression cases.

## 46. Completion criteria

The Sec 0.1 interrupt implementation is complete when the compiler can deterministically:

1. resolve named and raw interrupt bindings against `CompilationPlan`;
2. distinguish logical sources, routes, physical entries, classes, pending/active state, and controller lifecycle;
3. validate binding, ABI, bindability, priority/masking, reentrancy, nesting, and execution context;
4. generate/select target-correct vector entries, wrappers, dispatchers, claim/completion paths, and interrupt return;
5. preserve the separation between peripheral source service and controller lifecycle;
6. support target-defined enable/disable, pending, priority, routing, software-trigger, and structured masking without inventing unsupported operations;
7. enforce Sec 0.1 ISR noPanic/noAlloc/noBlock;
8. verify all reachable calls, indirect callables, generics, callbacks, FFI, runtime helpers, compiler helpers, defer, destruction, cleanup, and error paths;
9. consume canonical stack, race, deadlock, concurrency, hardware-register, runtime, and FFI proofs without duplicating their owners;
10. model single-core/multicore exclusion correctly and avoid treating volatile/local masking as universal synchronization;
11. enforce stack, runtime/context, and resolved bounded-execution/resource requirements;
12. integrate with `ProgramInitializationPlan`, `LinkPlan`, binary reachability, LTO, retention, and target placement;
13. keep handler binding separate from activation;
14. use the same semantics in compiler, `sec analyse`, LSP, separate-compilation summaries, and incremental recomputation;
15. produce deterministic provenance-rich root-specific diagnostics distinguishing `Invalid`, `Unproven`, and tooling `Pending`;
16. pass the semantic, analysis, target-integration, startup/linking, cross-compilation, incremental, false-positive, and determinism regression suites.

## 47. Core invariants

> An interrupt handler is valid only when its binding, execution context,
> reachable behavior, target lowering, required supporting proofs, and platform
> lifecycle are all valid under the resolved `CompilationPlan`.

> The compiler must not invent interrupt semantics absent from the resolved
> platform/device contract, and must not expose target mechanisms that can be
> safely and deterministically generated from that contract as mandatory
> source-level boilerplate.

> Platform/device knowledge describes hardware and execution reality. The
> compiler uses that knowledge to make interrupt programming safer and more
> ergonomic without changing the target's actual behavioral requirements.

## 48. Non-goals for Sec 0.1

This rulebook does not require:

- one universal numeric priority convention;
- one universal interrupt-controller model;
- one direct vector slot per logical source;
- one universal acknowledge operation;
- one universal `InterruptContext` source type;
- one universal vector-table representation;
- source-level weak handlers;
- hidden interrupt enablement from handler declaration;
- interrupt masking as universal multicore/DMA synchronization;
- volatile as ISR synchronization;
- panic, dynamic allocation, or blocking inside Sec 0.1 ISR execution;
- a separate interrupt-specific memory model;
- duplicate race/deadlock/stack analysis algorithms;
- a universal WCET algorithm;
- a specific platform registry syntax for interrupt metadata.
