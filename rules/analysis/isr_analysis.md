# ISR Analysis

**Created:** 2026-08-11  
**Last updated:** 2026-08-29
**Document revision:** 3
**Language version:** Sec 0.1  
**Status:** Draft for Sec 0.1

---

## 1. Purpose

ISR analysis verifies that every operation which executes in an interrupt context
satisfies the constraints of the active interrupt profile and `CompilationPlan`.

ISR analysis is primarily a cross-analysis constraint verifier. It does not
redefine interrupt scheduling, memory ordering, storage, stack composition,
ownership, data-race semantics, deadlock semantics, FFI behavior, or runtime
behavior. It consumes canonical facts from the rulebooks and analyses that own
those domains.

The core question is:

```text
Are all operations reachable in this interrupt execution context permitted by
its resolved ISR profile, and have all safety properties required by that
profile been proven?
```

The analysis must distinguish:

```text
proven forbidden behavior
proven safety
completed but insufficient proof
analysis that is not yet current or complete
```

Lack of evidence for unsafety is never proof of ISR safety.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.

---

## 2. Normative ownership and dependencies

`rules/platform/interrupts.md` owns interrupt execution semantics. The active
platform and frozen `CompilationPlan` supply the resolved facts consumed here,
including, where applicable:

```text
interrupt roots
interrupt classes
priority relationships
preemption relationships
nesting relationships
masking relationships
physical stack domains
execution-context transitions
runtime restrictions
available synchronization primitives
memory-space restrictions
```

Other canonical analyses own their respective reasoning:

```text
call graph                 -> call_graph.md
callable/closure targets   -> closure_analysis.md and callable-flow rules
semantic effects           -> effect_analysis.md
stack bounds               -> stack_analysis.md
data races                 -> data_races.md
deadlocks                  -> deadlock_analysis.md
storage/lifetime           -> storage and lifetime rules
ownership/borrowing        -> ownership and borrowing rules
foreign behavior           -> FFI contracts
runtime behavior           -> runtime/compiler-known contracts
hardware register access   -> platform/hardware-register-access.md
```

The hardware-register rulebook owns register access legality, access context,
hardware ordering and completion, hardware access faults, register side
effects, and target-specific transaction plans. ISR analysis consumes those
facts and checks them against the resolved ISR profile; it does not redefine
register transactions, barrier instructions, or completion mechanisms.

ISR analysis owns:

```text
resolution of the ISR profile for each interrupt root
construction of reusable interrupt-requirement summaries
propagation of interrupt-context requirements across calls
recognition of canonical context transitions
composition of supporting analysis results
constraint evaluation against the resolved ISR profile
Valid / Invalid / Unproven root classification
proof-dependency tracking for ISR validity
ISR-specific diagnostics and LSP integration
incremental invalidation of ISR summaries and root results
```

ISR analysis must not duplicate an owning analysis merely to obtain a more
convenient result.

---

## 3. Interrupt roots and ISR profiles

Every interrupt handler analyzed as an interrupt execution root is associated
with one resolved ISR profile under the active `CompilationPlan`.

Conceptually:

```text
ISRRoot {
    Handler
    InterruptClass
    PriorityClass
    CompilationPlan
}

ISRProfile {
    ExecutionClass
    Capabilities
    RequiredProofs
    StackBudget
    PlatformConstraints
}
```

These structures are explanatory; the implementation may use different internal
representations.

Profile resolution must be deterministic for a given source program,
`CompilationPlan`, compiler semantic version, and project configuration.

If a required interrupt execution profile cannot be resolved, the compiler must
not substitute a permissive profile. The result is a configuration or proof
failure appropriate to the owning phase.

### 3.1 Capabilities and required proofs are distinct

A profile may permit an operation while still requiring a positive safety proof.
For example, a profile may permit shared-memory access but require race freedom,
or permit synchronization but require deadlock freedom.

Conceptually:

```text
Capabilities:
    MayBlock
    MaySuspend
    MayAllocate
    MayUseRuntime
    MayCallForeign
    MayUseThreadLocal
    MayAcquireSynchronization
    ...

RequiredProofs:
    RequireBoundedStack
    RequireRaceFreedom
    RequireDeadlockFreedom
    RequireForeignInterruptSafety
    ...
```

The exact set is determined by canonical Sec semantics and the active platform.
This rulebook does not create source-level syntax for profiles.

### 3.2 Capability-first modeling

ISR restrictions should be expressed through semantic capabilities and
requirements rather than function-name blacklists.

For example, the analysis reasons about:

```text
MayBlock
MaySuspend
AllocatorBackedStorage
RequiresThreadContext
RequiresNonReentrantRuntime
BlockingAcquire
```

rather than assuming that a function is safe or unsafe because of its name.

Compiler-known, core, standard-library, platform, runtime, and foreign
operations require canonical summaries or contracts when their behavior is
relevant to ISR safety.

---

## 4. ISR safety is contextual

ISR safety is not an unconditional property of a function.

A function can be valid from one interrupt root and invalid from another because
profiles, stack budgets, priority relationships, runtime services, memory spaces,
or target lowering differ.

A positive result is therefore scoped to at least:

```text
interrupt root
resolved ISR profile
CompilationPlan
relevant specialization
supporting proof assumptions
```

Tooling must not present a contextual result as a universal statement such as
"this function is ISR-safe" unless the stronger universal claim has actually
been proven.

---

## 5. Transitive reachability

ISR restrictions apply to all behavior that executes in the interrupt context,
not only to statements written directly in the handler.

For example:

```sec
fn TimerHandler() {
    UpdateState()
}

fn UpdateState() {
    WaitForReady()
}
```

If `WaitForReady` may block and blocking is forbidden by the resolved profile,
the handler is invalid even though the blocking operation is not written in the
handler body.

Reachability includes, where semantically reachable:

```text
direct calls
indirect callable targets
recursive calls
compiler-required helpers
destruction
cleanup
defer
error paths
panic paths
synchronous callbacks
foreign synchronous reentry
```

Proven unreachable semantic paths do not contribute requirements.

---

## 6. Two-stage implementation architecture

A conforming implementation should separate ISR analysis into two logical
stages:

```text
Stage 1:
    build reusable transitive function-requirement summaries

Stage 2:
    evaluate each ISR root against its resolved ISR profile
```

Conceptually:

```text
Semantic function bodies
        |
        v
LocalRequirementSummary
        |
        v
TransitiveFunctionRequirementSummary
        |
        +----------------------+
                               |
ISRRoot + ResolvedISRProfile --+
                               |
                               v
                         ISRRootResult
```

This architecture is normative at the behavioral level because it ensures:

```text
summary reuse
separate compilation
bounded interprocedural analysis
profile-specific validation without body reanalysis
clean incremental invalidation
clear ownership of supporting analyses
```

An implementation may fuse physical passes as an optimization if the observable
semantics remain equivalent.

---

## 7. Local function-requirement summaries

Each function first receives a local interrupt-requirement summary derived from
its own semantic behavior.

Conceptually:

```text
LocalISRRequirements {
    Effects
    RuntimeRequirements
    StorageRequirements
    SynchronizationRequirements
    ForeignRequirements
    ExecutionTransitions
    SupportingDependencies
    CauseSites
}
```

The local summary excludes transitive requirements that arise only inside
callees. Those are added by interprocedural propagation.

### 7.1 Semantic IR as the preferred input

ISR analysis should consume canonical semantic operations rather than surface
syntax whenever the compiler pipeline provides such an IR.

Relevant semantic operations may include the equivalent of:

```text
Call
IndirectCall
Spawn
Await
BlockingWait
Acquire
Release
Allocation
StorageAccess
HardwareRegisterOperation
ForeignCall
Panic
Cleanup
Destruction
RuntimeHelperRequirement
ContextTransition
```

The exact IR operation names are implementation-defined.

The requirement is that ISR analysis observes semantic behavior rather than
source spelling.

### 7.2 Compiler-inserted behavior

Required behavior introduced by lowering must be visible to ISR safety proof.
A source operation accepted as ISR-safe must not later be lowered into behavior
that violates the active ISR profile under the assumptions used by the proof.

A compiler may satisfy this rule by either:

```text
1. representing required helper/runtime behavior explicitly in Semantic IR; or
2. making ISR analysis query canonical lowering/runtime contracts for the
   semantic operation.
```

It is invalid for lowering to silently introduce an ISR-forbidden operation
that was absent from the proof model.

---

## 8. Requirement domains

Requirements must be tracked per semantic dimension rather than collapsed into
one global unknown state.

A conceptual per-dimension state is:

```text
ProvenAbsent
Present
Unknown
```

where `Present` means that the behavior may occur on at least one reachable
semantic path.

An implementation may use a richer lattice where required.

Example:

```text
Blocking          = ProvenAbsent
Allocation        = Present
ForeignReentrancy = Unknown
```

The unknown foreign dimension must not erase the known blocking or allocation
facts.

### 8.1 May-analysis for forbidden behavior

For a profile that forbids an operation, a reachable may-effect is sufficient to
violate the capability constraint.

For example:

```text
RequirementSummary.Blocking = Present
ISRProfile.MayBlock = false
```

is a proven violation because a reachable execution can perform forbidden
behavior.

The operation need not occur on every path.

### 8.2 Control-flow joins

May-requirements are joined conservatively across reachable control-flow paths.
Proven unreachable paths are excluded using canonical reachability information.

The implementation must not perform its own contradictory reachability model
when a canonical CFG/Sema reachability result already exists.

---

## 9. Requirement evidence and cause paths

Each requirement fact should retain representative evidence sufficient to
construct diagnostics.

Conceptually:

```text
RequirementFact {
    State
    Cause
}

Cause {
    Function
    SemanticOperation
    SourceLocation
}
```

When a fact becomes transitive through calls, the analysis retains or can
reconstruct a semantic cause path such as:

```text
TimerHandler
    -> UpdateDevice
        -> WaitForReady
            -> blocking operation
```

Representative cause selection must be deterministic.

---

## 10. Transitive function summaries

The transitive function summary joins local requirements with requirements from
all semantically reachable callees that execute in the same relevant context.

Conceptually:

```text
Summary(F) =
    LocalSummary(F)
    join Summary(Callee1)
    join Summary(Callee2)
    ...
```

Call-site instantiation must preserve symbolic requirements where arguments,
resource identities, storage, generic parameters, or execution transitions
matter.

### 10.1 Ordinary synchronous calls

An ordinary synchronous call preserves the caller execution context unless
canonical semantics establish a context transition.

Therefore requirements of a synchronous callee propagate into the caller's ISR
requirements.

### 10.2 Execution-context transitions

Operations may explicitly transition execution out of the current interrupt
context.

Conceptually:

```text
SameContextCall
ContextTransition
DeferredExecution
ConcurrentExecution
```

The exact internal representation is implementation-defined.

If an ISR calls an operation that queues work for a later worker context, the
queueing operation itself must satisfy the ISR profile, but the later worker
body is checked under its own canonical execution context rather than being
blindly attributed to the ISR.

For example:

```text
ISR
    -> Enqueue()              executes under ISR constraints

WorkerContext
    -> HeavyWork()            executes under worker constraints
```

Context transitions must come from canonical effects/contracts. They must not be
inferred from names.

### 10.3 Avoid per-root body reanalysis

Implementations should avoid analyzing the same helper body independently for
every ISR root when a reusable symbolic summary suffices.

Context-dependent behavior should preferably be represented as symbolic
requirements such as:

```text
RequiresThreadLocalContext
RequiresInterruptSafeRuntime
RequiresMemorySpace(X)
```

and evaluated against the resolved profile later.

---

## 11. Interprocedural fixed point

Transitive requirement summaries must be computed over the canonical call
graph.

Recursive and mutually recursive call-graph SCCs require fixed-point analysis.

A suitable algorithm is:

```text
1. obtain the canonical call graph;
2. condense the graph into strongly connected components;
3. process the SCC DAG bottom-up where possible;
4. initialize every function in a recursive SCC with its local requirements;
5. repeatedly join requirements from callees and other members of the SCC;
6. continue until no exported summary in the SCC changes;
7. publish/cache the stable summaries.
```

The summary domain must support monotone iteration.

Within one analysis generation, worklist order, declaration order, and parallel
compiler scheduling must not change the final result.

Precision refinement caused by a new external fact or a deeper analysis mode is
a new analysis generation; it is not oscillation inside one fixed point.

### 11.1 Widening

Where finite abstraction requires widening, precision loss must be localized to
the affected dimensions.

Widening must never create a positive ISR-safety proof that was not supported by
the more precise facts.

---

## 12. Generic and specialized functions

Generic requirement summaries may remain symbolic.

Examples of symbolic requirements include:

```text
StackRequirement = SizeOf(T) + K
RequiresDestructor(T)
RequiresOperation(T, Op)
```

A concrete specialization instantiates these facts.

The same generic function may be valid for one specialization and invalid for
another, for example because layout, destruction, stack use, or runtime helper
requirements differ.

Cache identity must distinguish specializations whenever their ISR-relevant
semantics differ.

Compilation-plan identity should be part of the cache key only where target or
runtime behavior can change the requirement summary.

---

## 13. Indirect calls and callable contracts

ISR analysis consumes canonical callable-flow results.

For a closed target set:

```text
Targets = {A, B, C}
```

all target requirements are conservatively joined for behavior that may execute
in the interrupt context.

If any reachable target contains a forbidden may-behavior, the corresponding
requirement is `Present`.

Open callable behavior relies on canonical callable contracts. Missing required
contract dimensions become localized `Unknown`.

A callback parameter is not an escape hatch from ISR restrictions.

---

## 14. Stack integration

ISR analysis does not compute stack size.

It consumes `SemanticStackRequirement` or `MachineStackRequirement` from
`stack_analysis.md`, according to the constraint being checked.

Nested interrupt contributions and physical stack-domain composition must be
computed by stack analysis under the active platform execution model.

ISR analysis must not add nested ISR frame sizes a second time.

### 14.1 Stack result mapping

For a finite stack budget `B`:

```text
Exact(N), N <= B       -> Satisfied
UpperBound(N), N <= B  -> Satisfied
Exact(N), N > B        -> Violated
UpperBound(N), N > B   -> Unproven unless overflow is independently proven
Unknown                -> Unproven
Unbounded              -> Violated when a finite bound is required
```

The distinction for `UpperBound(N) > B` is normative.

An upper bound larger than the budget proves only that the current upper bound
is insufficient to establish safety. It does not prove that actual stack use
exceeds the budget.

`Unbounded` and `Unknown` remain distinct.

---

## 15. Data-race integration

ISR execution contexts participate in canonical data-race analysis.

ISR analysis consumes relevant race results rather than performing its own
Place, alias, access-pairing, or happens-before analysis.

When a profile requires race freedom, the conceptual mapping is:

```text
ProvenRace      -> Violated
ProvenRaceFree  -> Satisfied
Unknown         -> Unproven
Pending         -> Pending
```

Only race results relevant to the ISR root and its reachable execution
relationships may affect that root.

ISR analysis must not strengthen `Unknown` into a proven race.

---

## 16. Deadlock integration

ISR analysis consumes deadlock results from `deadlock_analysis.md`.

When a profile requires positive deadlock-freedom proof, the conceptual mapping
is:

```text
ProvenDeadlock          -> Violated
PotentialDeadlock       -> Unproven
Unknown                 -> Unproven
ProvenNoRelevantCycle   -> Satisfied
Pending                 -> Pending
```

`PotentialDeadlock` remains potential. ISR analysis must not strengthen it into
`ProvenDeadlock`.

A separate ISR capability may forbid blocking synchronization even when no
cycle exists. Such a capability violation is independent of the deadlock
classification.

---

## 17. Preemption, nesting, masking, and priority

ISR analysis consumes resolved execution relationships such as the semantic
equivalent of:

```text
MayPreempt(A, B)
CannotPreempt(A, B)
MayNest(A, B)
CannotNest(A, B)
MayReenter(A)
```

Raw numeric priorities alone are not sufficient unless the canonical platform
semantics define their complete meaning.

Masking and critical-section semantics may alter preemption, race, deadlock, and
stack facts. Masking does not automatically permit unrelated forbidden
behavior such as blocking, allocation, unsafe runtime use, or invalid foreign
calls.

### 17.1 ISR deadlock pattern

A preemption relationship may participate in a proven deadlock, for example:

```text
Thread holds L
ISR preempts Thread
ISR waits for L
Thread cannot resume until ISR returns
```

Deadlock analysis owns the cycle proof. ISR analysis consumes the result and/or
independently checks a profile rule forbidding the blocking acquisition.

---

## 18. Blocking and synchronization requirements

Blocking, nonblocking, spinning, and bounded-wait primitives must be interpreted
according to their canonical synchronization contracts.

The analysis must not assume that every synchronization operation blocks or
that every non-sleeping operation is interrupt-safe.

A spin loop may be invalid when its progress depends on the preempted context.

A profile may, for example, allow a nonblocking acquisition while forbidding a
blocking acquisition. This distinction must survive through function summaries.

---

## 19. Storage and allocation requirements

ISR profiles may constrain storage behavior at the semantic storage-model level.

The analysis must preserve distinctions such as:

```text
Automatic
Static
ThreadLocal
Arena
AllocatorBacked
```

It must not equate preallocated arena use with allocator-backed heap behavior.

A profile may forbid allocator-backed dynamic allocation while permitting
preallocated arena or static storage.

Storage requirements are consumed from canonical storage/allocation semantics.

---

## 20. Thread-local storage

Thread-local availability and meaning in interrupt context are platform and
runtime properties.

An interrupt context may:

```text
share interrupted-thread TLS
have dedicated interrupt-local state
have no valid TLS context
```

ISR analysis consumes the resolved rule and checks requirements such as
`RequiresThreadLocalContext`.

TLS availability and runtime reentrancy are separate dimensions.

---

## 21. Runtime reentrancy and interrupt safety

Runtime reentrancy and interrupt safety are distinct semantic properties.

A runtime operation may be nonblocking and nonallocating yet still be invalid in
an ISR because it uses non-reentrant shared runtime state or assumes normal
thread context.

Conversely, a specialized interrupt operation may be valid in a specific ISR
profile without making a broader claim of universal reentrancy.

Canonical runtime/compiler-known contracts must expose the properties required
by active ISR profiles.

The exact contract syntax is owned elsewhere.

---

## 22. Destruction, defer, cleanup, error, and panic paths

ISR analysis applies to the complete reachable semantic execution, including:

```text
destructors
cleanup
scope-exit behavior
defer
error handling
panic paths
compiler-required cleanup helpers
```

For example, a handler whose normal path is interrupt-safe is still invalid if a
reachable destructor performs a forbidden blocking or foreign operation.

A forbidden behavior on a proven unreachable path does not contribute.

Where expensive path-sensitive reasoning is needed to prove a path unreachable,
insufficient Standard precision yields `Unknown`/`Unproven` rather than an
unsupported positive proof.

---

## 23. FFI integration

Foreign behavior is a contract boundary.

ISR analysis may consume separately known facts such as:

```text
MayBlock
MayAllocate
Reentrant
InterruptSafe
RequiresThreadContext
CallbackExecutionContext
MayReenterSynchronously
MayHandoffAsynchronously
```

These dimensions remain independent.

Missing information is localized to the missing dimension.

No ISR property may be inferred from foreign function or parameter names.

### 23.1 Synchronous callbacks

If a foreign contract specifies synchronous reentry into Sec while the ISR call
is active, the callback executes under the same ISR constraints unless another
canonical execution transition is specified.

### 23.2 Asynchronous handoff

If a foreign contract specifies that work or a callback runs later in a normal
worker context, the later work is checked under that context. The foreign call
that performs the handoff still executes in the ISR and must itself satisfy the
ISR profile.

### 23.3 Missing contracts

If the active profile requires a property and the foreign contract does not
supply enough information, the relevant constraint is `Unproven`; it is not a
proven foreign violation unless forbidden behavior itself is proven.

---

## 24. Compiler-known and target-specific helpers

Compiler-inserted helper behavior is part of ISR analysis whenever the helper is
semantically required by the selected lowering.

Examples may include:

```text
numeric helpers
allocation helpers
panic helpers
copy/destruction helpers
atomic/runtime helpers
platform ABI helpers
```

The same source operation may therefore be valid for one `CompilationPlan` and
invalid for another when one target lowers directly and another requires an
ISR-incompatible helper.

This plan dependence is correct and must be represented explicitly rather than
hidden after analysis.

---

## 25. Constraint evaluation algorithm

After transitive summaries are stable, each ISR root is checked against its
resolved profile.

The implementation should evaluate profile constraints independently and then
aggregate the result.

Conceptually:

```text
for each mandatory profile constraint C:
    R = Evaluate(C, RootRequirementSummary, SupportingProofs)
    record R

if any R == Violated:
    OverallStatus = Invalid
else if any R == Pending:
    OverallStatus = Pending
else if any mandatory R == Unproven:
    OverallStatus = Unproven
else:
    OverallStatus = Valid
```

`Pending` is a tooling/currentness state rather than a completed semantic
classification. An implementation may keep it outside the persisted root-status
enum if the behavior remains equivalent.

### 25.1 Per-constraint results

Conceptually:

```text
ConstraintStatus:
    Satisfied
    Violated
    Unproven
    Pending
    NotApplicable
```

A constraint result should retain:

```text
status
profile requirement
semantic evidence
supporting proof reference
representative cause path
unknown cause, when applicable
```

### 25.2 Invalid dominates completed uncertainty

If one required constraint is proven violated, the root is `Invalid` even when
other dimensions are `Unknown` or `Unproven`.

The unresolved dimensions remain available for diagnostics and tooling.

### 25.3 Valid requires a complete proof set

A root is `Valid` only when every mandatory constraint has a current compatible
positive proof.

The compiler must never infer `Valid` merely because no violation has yet been
found.

This creates an intentional asymmetry:

```text
one proven violation -> Invalid immediately
Valid -> requires every mandatory proof to be current and satisfied
```

---

## 26. Pending versus Unproven

`Pending` and `Unproven` are distinct.

```text
Pending:
    analysis required for the answer is stale, invalidated, or not complete

Unproven:
    required analysis completed at the current precision/contracts but lacks
    sufficient evidence to prove the property
```

A pending dependency may later resolve to `Valid`, `Invalid`, or `Unproven`.

An `Unproven` result is a completed result for the current analysis inputs.

LSP and build integration must not present a stale positive proof as current
while a mandatory dependency is pending.

---

## 27. Supporting proof dependencies

A positive ISR result should depend explicitly on the analyses and contracts
that justify it.

Conceptually:

```text
ISRRootProof
    -> FunctionRequirementSummary(F)
    -> StackResult(Root)
    -> DataRaceResult(R)
    -> DeadlockResult(D)
    -> RuntimeContract(X)
    -> FFIContract(Y)
    -> ResolvedISRProfile(P)
```

ISR analysis should reference supporting results rather than copy and take
ownership of their internal proof models.

A supporting proof dependency should be invalidated when its owning analysis
says the referenced result is no longer current or compatible.

---

## 28. Incremental dependency graph

ISR analysis should participate in the compiler's canonical analysis-dependency
tracking.

Relevant dependencies include:

```text
FunctionSummary(A) -> FunctionSummary(B)
ISRRootResult(R)    -> FunctionSummary(A)
ISRRootResult(R)    -> StackResult(R)
ISRRootResult(R)    -> DataRaceResult(X)
ISRRootResult(R)    -> DeadlockResult(Y)
ISRRootResult(R)    -> FFIContract(Z)
ISRRootResult(R)    -> ResolvedISRProfile(P)
```

If the compiler has a shared dependency-graph facility, ISR analysis should use
it rather than build an incompatible private system.

---

## 29. Incremental recomputation algorithm

When a function changes, a conforming incremental implementation should behave
conceptually as follows:

```text
1. invalidate the function's local ISR-requirement summary;
2. mark dependent transitive summaries stale;
3. invalidate affected ISR-root positive proofs immediately;
4. recompute the changed local summary;
5. push the changed summary into a dependency worklist;
6. recompute direct dependents;
7. propagate further only when an exported summary changes;
8. reevaluate affected ISR roots after required dependencies are current.
```

Conceptually:

```text
queue changed summaries

while queue not empty:
    recompute summary

    if exported summary changed:
        queue direct dependents
```

If a summary is semantically equivalent to its previous exported summary,
transitive propagation may stop along that dependency.

### 29.1 Profile-only changes

If only a profile constraint changes, such as an ISR stack budget, unaffected
function requirement summaries should remain cached.

Only root constraint evaluation and directly dependent supporting analyses need
be rerun unless the changed profile also changes semantic execution behavior.

### 29.2 CompilationPlan changes

A target or plan change may invalidate more information, including:

```text
resolved ISR profile
preemption/nesting relationships
runtime-helper requirements
machine stack summaries
FFI/platform contracts
memory-space requirements
lowering-sensitive summaries
```

Invalidation must follow actual semantic dependencies rather than always
recomputing the entire project.

---

## 30. LSP progressive analysis

ISR analysis uses the common Sec modes:

```text
Interactive
Standard
Deep
```

No separate global `Intermediate` analysis mode is introduced.

The LSP may progressively refine the same result:

```text
fast Interactive result
        -> incremental refinement
        -> Standard-equivalent result
        -> optional Deep refinement
```

### 30.1 Early proven violations

A direct proven violation may be surfaced before unrelated analysis completes.

For example, if the current handler directly calls a known blocking operation
and blocking is forbidden, the LSP need not wait for whole-program stack or
race analysis before showing the violation.

The root may internally be treated as:

```text
Invalid with additional analysis pending
```

### 30.2 Positive results wait for complete proof

The LSP must not present the root as `Valid` until every mandatory dependency is
current and satisfied.

If a previously valid dependency becomes stale, the positive ISR proof ceases
to be current immediately.

### 30.3 Targeted Deep refinement

When Standard ends in `Unproven`, Deep ISR analysis should first identify which
owning analysis prevents the proof.

Examples:

```text
stack bound Unknown
race alias Unknown
PotentialDeadlock
open callable target Unknown
foreign interrupt-safety contract Unknown
```

Deep ISR analysis should request stronger precision from the owning analysis
before adding duplicate stack, alias, race, deadlock, or callable reasoning.

ISR Deep is therefore primarily an orchestrator of supporting precision.

---

## 31. LSP presentation

### 31.1 NeedToKnow

The following are NeedToKnow:

```text
proven ISR constraint violations
failure to prove a mandatory ISR safety property
```

### 31.2 OptionalInsight

Positive proof explanations and detailed requirement/proof relationships are
OptionalInsight.

For example:

```text
ISR-safe under TimerIRQ:
    no reachable blocking operation
    no allocator-backed allocation
    machine stack <= 192 bytes
    relevant shared accesses are race-free
    no relevant deadlock cycle
    all required foreign contracts are satisfied
```

These explanations should not create default hover noise unless configured.

### 31.3 Diagnostic locations

A diagnostic should normally identify the semantic operation responsible for
the violation as the primary location and the ISR root as related information.

Example:

```text
error: blocking operation is not permitted from interrupt root `TimerHandler`

TimerHandler
    -> UpdateDevice
        -> WaitForReady

`WaitForReady` may block under the resolved TimerIRQ profile.
```

Diagnostics must identify the applicable root/profile rather than make an
unsupported universal claim about the helper function.

### 31.4 Grouping

Multiple constraint consequences caused by one semantic operation may be
grouped for presentation, but their semantic proof states remain separate.

Generic architectural quick fixes such as "move this to a thread" or "replace
the mutex" must not be offered automatically unless a canonical transformation
proves the edit correct.

Useful tooling actions include:

```text
navigate to cause
navigate to ISR root
show semantic call path
show violated profile constraint
show supporting analysis result
```

---

## 32. `sec analyse`

ISR analysis participates in the default all-analysis behavior of:

```text
sec analyse
```

Tooling may provide an explicit ISR-only selection equivalent to:

```text
sec analyse isr
```

The exact CLI spelling is owned by CLI/tooling rules.

Detailed analysis output should be capable of reporting per interrupt root:

```text
resolved profile
overall status
forbidden-behavior results
mandatory proof results
stack status
race status
deadlock status
runtime requirements
FFI requirements
Unknown causes
representative call/cause paths
```

Deep output may additionally expose requirement-summary and proof-dependency
chains.

---

## 33. Separate compilation

Libraries may export compatible function-requirement summaries without knowing
which interrupt roots will later call them.

Imported summaries may contain independently known dimensions such as:

```text
Blocking       = ProvenAbsent
Allocation     = ProvenAbsent
RuntimeSafety  = Known
StackSymbolic  = Known
ForeignSafety  = Unknown
```

Known information remains usable even when another dimension is unknown.

If a compatible summary is unavailable, only the affected required dimensions
become `Unknown`. Missing body information must never imply safety.

Summary compatibility must account for all semantic versions that can affect
meaning, including as relevant:

```text
summary schema
Sec language/compiler semantic version
effect model
runtime contracts
storage semantics
stack-summary schema
callable contracts
FFI contracts
CompilationPlan-sensitive behavior
```

An incompatible or stale imported summary cannot serve as positive ISR-safety
evidence.

---

## 34. Determinism

For identical:

```text
source
CompilationPlan
contracts
project analysis configuration
compiler semantic version
```

ISR analysis must produce equivalent:

```text
function requirement summaries
root classifications
constraint results
representative causes
diagnostic ordering
```

independently of declaration order, worklist order, cache iteration order, or
parallel compiler scheduling.

Representative cause selection should prefer, in a deterministic order,
stronger proof, shorter semantic paths, stable source order, and stable symbol
identity or equivalent criteria.

---

## 35. Analysis budgets

### 35.1 Interactive

Interactive analysis prioritizes cheap, current facts such as:

```text
direct forbidden effects
cached transitive summaries
known blocking operations
known allocator-backed storage
known unsafe runtime/FFI operations
cached stack violations
cached race/deadlock results
```

Interactive may be `Pending` while affected interprocedural or supporting
analysis is recomputed.

### 35.2 Standard

Standard performs every proof required for source validity and for the active
ISR profile under the selected `CompilationPlan`.

If a stronger analysis is required to establish a mandatory property, that
precision becomes required Standard work for that program/profile rather than
being deferred to optional Deep analysis.

### 35.3 Deep

Deep may reduce `Unproven` results through more expensive supporting analysis,
including:

```text
callable-target refinement
stack-bound refinement
alias/race refinement
path-sensitive effect refinement
foreign-contract correlation
context-sensitive execution reasoning
```

Deep does not change ISR profile semantics.

---

## 36. Required implementation properties

A conforming Sec 0.1 implementation of ISR analysis must provide the behavioral
equivalent of:

```text
deterministic ISR-root discovery
resolved ISR profiles
local requirement summaries
transitive interprocedural summaries
call-graph SCC fixed point
same-context call propagation
canonical execution-context transition handling
effect composition
runtime/compiler-helper accounting
storage/allocation requirement composition
stack-analysis integration
data-race integration
deadlock integration
FFI interrupt-contract integration
callable target integration
generic specialization handling
Valid / Invalid / Unproven root evaluation
Pending/currentness tracking
supporting proof dependencies
incremental dependency-driven invalidation
LSP progressive refinement
separate-compilation summary support
```

---

## 37. Test requirements

The implementation must maintain regression coverage for the following classes.

### 37.1 Profile resolution

```text
handler resolves the correct profile
same helper is valid under one profile and invalid under another
missing required profile fails safely
permitted capability
forbidden capability
mandatory proof satisfied
mandatory proof unavailable
```

### 37.2 Effects and transitive behavior

```text
direct blocking call
transitive blocking call
blocking only on a proven unreachable path
conditional reachable blocking
forbidden suspension/await
allowed nonblocking operation
forbidden allocator-backed allocation
allowed preallocated arena-backed operation
```

### 37.3 Execution-context propagation

```text
ordinary helper inherits ISR context
nested synchronous callback inherits ISR context
queued worker does not inherit ISR context
enqueue operation itself remains ISR-constrained
spawn/handoff follows canonical context semantics
foreign synchronous callback remains ISR-contextual
foreign asynchronous callback runs in its declared worker context
```

### 37.4 Stack integration

```text
Exact below budget -> satisfied
Exact above budget -> violated
UpperBound below budget -> satisfied
UpperBound above budget -> unproven unless overflow independently proven
Unknown -> unproven
Unbounded -> violated when finite stack is required
nested ISR stack result is consumed without double-counting
different profiles may have different stack budgets
```

### 37.5 Data-race integration

```text
ProvenRace -> violation when race freedom is required
ProvenRaceFree -> satisfied
Unknown -> unproven
irrelevant race result does not affect the ISR root
ISR/thread preemption relation
nested ISR/ISR relation
```

### 37.6 Deadlock integration

```text
ProvenDeadlock -> violation
PotentialDeadlock -> unproven under positive-proof requirement
Unknown -> unproven
proven no relevant cycle -> satisfied
blocking synchronization may independently violate the ISR profile
```

### 37.7 Runtime and reentrancy

```text
nonblocking but non-reentrant runtime helper
reentrant but thread-context-only operation
interrupt-safe compiler helper
target-specific helper changes ISR validity
destructor invokes unsafe runtime
defer invokes unsafe runtime
panic path invokes forbidden runtime
```

### 37.8 FFI

```text
known interrupt-safe foreign call
foreign MayBlock
foreign allocation
foreign reentrancy Unknown
partial foreign contract
synchronous foreign reentry callback
asynchronous foreign handoff
missing foreign contract
same foreign call valid under one profile but unproven under a stricter profile
```

No test or implementation may infer behavior from foreign names.

### 37.9 Callables

```text
closed safe target
closed target set with one forbidden target
open contract with sufficient facts
open contract missing blocking fact
open contract missing interrupt-safety fact
captured callback called synchronously from ISR
```

### 37.10 Generics

```text
generic safe specialization
generic specialization exceeds stack budget
generic destructor introduces forbidden behavior
symbolic stack summary is instantiated
generic callee requirement propagates through an SCC
```

### 37.11 Cleanup and error paths

```text
safe normal path with unsafe destructor
safe normal path with unsafe defer
forbidden panic helper
handled error avoids forbidden path
reachable error invokes unsafe helper
proven unreachable error path is ignored
```

### 37.12 Incremental LSP

```text
add blocking call -> affected ISR diagnostic appears
remove blocking call -> diagnostic disappears
callee summary change -> dependent roots refresh
unchanged exported summary stops propagation
stack result invalidated -> previous Valid root is no longer current
race result Pending -> positive ISR proof is invalidated
FFI contract update -> only dependent roots refresh
profile budget change -> constraints reevaluate without unnecessary body analysis
target change -> plan-sensitive dependencies invalidate
unrelated function edit retains unaffected ISR caches
```

### 37.13 Progressive result states

```text
direct violation appears before unrelated analysis completes
root is never shown Valid while a mandatory dependency is Pending
Pending later resolves to Valid
Pending later resolves to Invalid
completed Unknown produces Unproven, not Pending
Deep can refine Unproven to Valid
Deep can refine Unproven to Invalid
```

### 37.14 Performance and determinism

Large synthetic cases must cover:

```text
many ISR roots sharing helpers
large call graphs
large recursive SCCs
many generic specializations
many identical profiles
many profile variants
deep helper chains
many supporting proof dependencies
```

The implementation should verify:

```text
summary reuse
bounded cache growth
dependency-driven invalidation
SCC convergence
deterministic output
```

No normative wall-clock threshold is defined here.

### 37.15 False-positive regression corpus

The regression corpus should include known-safe patterns such as:

```text
preallocated arena use
nonblocking synchronization
safe MMIO access
safe compiler helper
serialized or masked ISR relationship
worker handoff
synchronous safe callback
profile-specific TLS
bounded stack
race-free disjoint shared state
```

Every confirmed ISR-analysis false positive should be considered for permanent
regression coverage.

---

## 38. Governance

Normative ISR-analysis semantics belong in:

```text
rules/analysis/isr_analysis.md
```

Mutable implementation progress belongs in:

```text
implementation-status.yaml
```

The recommended implementation-ledger identity is:

```text
sema.isr-analysis
```

Implementation status must describe observable repository state and must not be
inferred from this rulebook.

---

## 39. Sec 0.1 completion criterion

ISR analysis is complete for Sec 0.1 when the compiler can soundly:

```text
resolve every required ISR root and profile
construct reusable local requirement summaries
compute transitive summaries through deterministic SCC fixed point
propagate same-context calls and canonical execution transitions
account for compiler-required runtime/lowering behavior
consume stack, race, and deadlock results without duplicating their algorithms
check runtime and FFI interrupt contracts
handle relevant generic specializations
classify root constraints as satisfied, violated, or unproven
produce Valid, Invalid, and Unproven root results correctly
track freshness of positive supporting proofs
invalidate and recompute dependencies incrementally
provide progressive LSP results without a separate Intermediate mode
request Deep refinement from owning supporting analyses where useful
consume compatible separate-compilation summaries
produce deterministic diagnostics and cause paths
pass the required regression suites
```

The central invariant is:

```text
An interrupt root is Valid only when every mandatory ISR constraint is currently
proven satisfied under the resolved ISR profile and CompilationPlan.
```
