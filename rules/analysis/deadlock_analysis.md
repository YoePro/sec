# Deadlock Analysis

- **Status:** Normative
- **Created:** 2026-07-29
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/deadlock_analysis.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`

---

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the compiler analysis used to detect proven deadlocks,
identify meaningful potential deadlocks, preserve uncertainty when dependency
information is insufficient, summarize synchronization behavior
interprocedurally, consume canonical execution-context and resource semantics,
integrate incrementally with the LSP, and provide deterministic diagnostics and
tooling results.

The canonical Sec concurrency, synchronization, task/thread, ownership,
lifetime, FFI, ISR, and platform rulebooks own the semantics of the operations
that may participate in blocking and progress dependencies. This rulebook
defines how the compiler reasons about those semantics to detect cyclic loss of
progress.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.

---

## § 1 Purpose

**§ 1(1)** Deadlock analysis answers questions such as:

```text
Which synchronization resources are held at this program point?
Which resource or execution completion can this operation wait for?
Can these resource identities alias?
Can the participating execution contexts overlap, preempt, or reenter?
Can a reachable wait-for cycle be established?
Can every edge in that cycle be simultaneously realized?
Does any participant have a guaranteed progress mechanism that breaks the cycle?
Is the result ProvenDeadlock, PotentialDeadlock, or Unknown?
Which source locations and interprocedural paths explain the cycle?
```

**§ 1(2)** Deadlock analysis is not a replacement for data-race, effect, ownership,
lifetime, Place/provenance, call-graph, closure, task/thread, FFI, ISR, or
concurrency-memory-model analysis.

**§ 1(3)** It consumes canonical facts from those analyses and correlates them.

---

## § 2 Normative ownership

**§ 2(1)** Canonical concurrency and synchronization rules own:

```text
resource acquisition and release semantics
resource reentrancy
resource capacity
blocking and nonblocking acquisition behavior
condition/event semantics
task and thread completion semantics
join and await semantics
callback and reentry semantics
ISR preemption and masking semantics
foreign synchronization contracts
guaranteed timeout or cancellation semantics
```

**§ 2(2)** Deadlock analysis owns:

```text
held-resource state
resource-order relationships
wait dependencies
interprocedural deadlock summaries
LockOrderGraph construction
WaitForGraph construction
cycle detection
cycle realizability analysis
ProvenDeadlock / PotentialDeadlock / Unknown classification
representative cycle witnesses
deadlock diagnostics
incremental invalidation and LSP integration
```

**§ 2(3)** The analysis must not redefine synchronization or scheduler semantics merely to
make deadlock reasoning easier.

---

## § 3 Scope

**§ 3(1)** Deadlock is a cyclic blocking/progress dependency.

**§ 3(2)** This rulebook does not redefine:

```text
data race
livelock
starvation
performance contention
long but finite blocking
```

**§ 3(3)** Conceptually:

```text
Data race:
    conflicting memory accesses without required ordering

Deadlock:
    cyclic blocking dependencies prevent required progress

Livelock:
    execution continues but useful progress does not occur

Starvation:
    a participant may indefinitely fail to obtain progress
```

**§ 3(4)** Only deadlock is normative here.

---

## § 4 General dependency model

**§ 4(1)** Deadlock analysis is not limited to mutexes.

**§ 4(2)** An execution context may wait on canonical progress from:

```text
Lock
TaskCompletion
ThreadCompletion
ConditionOrEvent
ForeignCompletion
PlatformSynchronization
ResourceAvailability
Unknown
```

**§ 4(3)** The implementation may use different internal categories, but the semantic
model must be able to represent mixed cycles involving both execution contexts
and synchronization resources.

**§ 4(4)** Conceptually, the analysis reasons about:

```text
ExecutionContext
SynchronizationResource
HeldResource
WaitDependency
ProgressDependency
```

**§ 4(5)** and relationships such as:

```text
Holds(Context, Resource)
WaitsFor(Context, ResourceOrProgress)
ProgressProvidedBy(ResourceOrProgress, Context)
```

---

## § 5 Canonical synchronization-resource identity

**§ 5(1)** Synchronization resources are identified through canonical semantic identity,
including Place/provenance where applicable.

**§ 5(2)** For example:

```sec
state.Lock
```

**§ 5(3)** may be represented symbolically as:

```text
Parameter(0).Lock
```

**§ 5(4)** inside an exported function summary.

**§ 5(5)** Two resource expressions must remain distinguishable as:

```text
Same
Disjoint
MayAlias
Unknown
```

**§ 5(6)** or equivalent semantic states supported by canonical Place/provenance analysis.

**§ 5(7)** Names such as `mutex`, `stateLock`, or `cacheGuard` are never proof of resource
identity.

---

## § 6 Lock-order relationships

**§ 6(1)** A lock-order relationship:

```text
L1 -> L2
```

**§ 6(2)** means that there exists a reachable execution in which `L2` may be acquired
while `L1` remains held.

**§ 6(3)** For example:

```sec
first.Lock()
second.Lock()
```

**§ 6(4)** may establish:

```text
first -> second
```

**§ 6(5)** provided `first` is still held when acquisition of `second` occurs.

**§ 6(6)** A single order edge is not a deadlock.

**§ 6(7)** A path such as:

```text
L1 -> L2 -> L3
```

**§ 6(8)** is not a deadlock unless relevant dependencies form a realizable cycle.

---

## § 7 LockOrderGraph and WaitForGraph

**§ 7(1)** The analysis may maintain both a lock-order projection and a generalized
wait-for representation.

**§ 7(2)** The `LockOrderGraph` is useful for efficient analysis of nested resource
acquisitions.

**§ 7(3)** The generalized `WaitForGraph` can represent relationships such as:

```text
Execution -> Resource
Resource  -> OwningExecution
Execution -> TaskCompletion
Execution -> ThreadCompletion
Execution -> ForeignCompletion
Execution -> EventProgress
```

**§ 7(4)** The exact graph representation is implementation-defined.

**§ 7(5)** The generalized wait-for model is normative because deadlocks may involve
mixed dependencies that a pure lock-order graph cannot express.

---

## § 8 A cycle is initially a candidate

**§ 8(1)** A cycle in a lock-order or wait-for graph does not by itself prove deadlock.

**§ 8(2)** A complete deadlock proof additionally requires enough information to show that
its dependencies can coexist in a reachable execution.

**§ 8(3)** Conceptually:

```text
CandidateCycle
    + reachable participants
    + compatible execution paths
    + compatible resource identities
    + compatible execution overlap, preemption, or reentry
    + blocking waits
    + no guaranteed progress mechanism that breaks the cycle
    = ProvenDeadlock
```

**§ 8(4)** This expression is explanatory. The implementation need not use this exact
form.

---

## § 9 Core result classes

**§ 9(1)** The analysis distinguishes:

```text
ProvenDeadlock
PotentialDeadlock
Unknown
```

### § 9.1 ProvenDeadlock

**§ 9.1(1)** `ProvenDeadlock` means the compiler has a sound reachable witness in which all
participants can simultaneously realize a cyclic wait dependency and no
participant has a guaranteed mechanism that allows the required progress to
break the cycle.

### § 9.2 PotentialDeadlock

**§ 9.2(1)** `PotentialDeadlock` means the compiler has established a meaningful reachable
cyclic dependency, but one or more compatibility, identity, scheduling,
capacity, or path facts are not strong enough to establish a complete
simultaneous blocked-state witness.

**§ 9.2(2)** A potential deadlock is not equivalent to arbitrary uncertainty.

### § 9.3 Unknown

**§ 9.3(1)** `Unknown` means deadlock-relevant information is insufficient to establish even
a sufficiently concrete cyclic dependency.

**§ 9.3(2)** Examples include:

```text
foreign call may block but its wait target is unspecified
resource provenance is lost
custom synchronization semantics are unknown
platform preemption relationship is unknown
open callback behavior lacks a usable contract
```

**§ 9.3(3)** Unknown must not be silently converted into `PotentialDeadlock`.

---

## § 10 Failure to prove required deadlock freedom

**§ 10(1)** An active language, target, runtime, ISR, project, or CompilationPlan policy may
require a positive deadlock-freedom proof for a particular context.

**§ 10(2)** If the analysis result is `Unknown`, compilation may fail because the required
proof could not be established.

**§ 10(3)** Such a failure is distinct from both:

```text
ProvenDeadlock
PotentialDeadlock
```

**§ 10(4)** The diagnostic must state failure to prove the required property rather than
claiming that a deadlock has been proven.

---

## § 11 Self-deadlock

**§ 11(1)** Deadlock does not require two execution contexts.

**§ 11(2)** For a non-reentrant exclusive resource:

```sec
lock.Lock()
lock.Lock()
```

**§ 11(3)** may establish a self-deadlock when the first acquisition remains active and the
second acquisition blocks waiting for the same resource.

**§ 11(4)** Self-deadlock analysis must use canonical resource identity and reentrancy
semantics.

**§ 11(5)** A self-order edge:

```text
L -> L
```

**§ 11(6)** is not automatically invalid for a resource whose canonical semantics permit
reentrant acquisition by the same execution context.

---

## § 12 Held-resource state

**§ 12(1)** Deadlock analysis is flow-sensitive with respect to held resources.

**§ 12(2)** At each relevant program point the compiler must retain enough information to
distinguish the semantic equivalents of:

```text
DefinitelyHeld
PossiblyHeld
```

**§ 12(3)** A coarse set of all resources used by the function is insufficient.

**§ 12(4)** For example:

```sec
first.Lock()
first.Unlock()
second.Lock()
```

**§ 12(5)** must not establish:

```text
first -> second
```

**§ 12(6)** when canonical semantics prove `first` has been released before acquisition of
`second`.

---

## § 13 Acquisition

**§ 13(1)** When a blocking resource is acquired while other resources remain held, the
analysis may establish order/dependency relationships from the held resources
to the acquired resource.

**§ 13(2)** For example, if:

```text
DefinitelyHeld = {A, B}
```

**§ 13(3)** and `C` is acquired, the analysis may establish:

```text
A -> C
B -> C
```

**§ 13(4)** A resource that is only possibly held contributes weaker evidence than a
resource definitely held on the relevant path.

**§ 13(5)** Proof strength must be preserved.

---

## § 14 Wait versus successful acquisition

**§ 14(1)** For blocking resource operations, the analysis distinguishes the wait event
from successful ownership.

**§ 14(2)** Conceptually:

```text
WaitFor(L)
AcquireSucceeded(L)
```

**§ 14(3)** During the wait, the current execution depends on progress associated with
`L`.

**§ 14(4)** After successful acquisition, `L` becomes held according to the resource's
canonical ownership semantics.

---

## § 15 Release

**§ 15(1)** Release removes the relevant resource from held state when canonical identity
and ownership semantics establish that the released resource is the held
resource.

**§ 15(2)** If alias or ownership information is incomplete, uncertainty remains localized
to the affected held-state relationship.

---

## § 16 Nonblocking and fallible acquisition

**§ 16(1)** A synchronization operation that can fail without blocking must not be modeled
as an unconditional blocking wait.

**§ 16(2)** For a fallible or nonblocking acquisition, held state is updated only on paths
where acquisition succeeds according to canonical semantics.

**§ 16(3)** A nonblocking failure path cannot by itself participate in a permanent wait
cycle.

---

## § 17 Scoped guards and resource tokens

**§ 17(1)** When synchronization ownership is represented by a guard or token value, the
analysis may consume canonical ownership, lifetime, move, destruction, and
release facts.

**§ 17(2)** Conceptually:

```text
Guard live and owned by context
    -> resource held by context

Guard released or destroyed
    -> resource no longer held

Guard moved to another owner
    -> ownership follows the canonical move semantics
```

**§ 17(3)** Guard-based analysis is a precision source, not a requirement that all Sec
synchronization APIs use guards.

---

## § 18 `defer`, cleanup, and destruction

**§ 18(1)** Deferred release participates in the actual held-resource interval.

**§ 18(2)** For example:

```sec
lock.Lock()
defer lock.Unlock()

Call()
```

**§ 18(3)** means the resource remains held across `Call()` on every path where the deferred
release has not yet executed.

**§ 18(4)** Canonical cleanup, destruction, error, early-return, and panic paths must be
respected.

**§ 18(5)** A dependency edge from proven unreachable code must not participate in the
analysis.

---

## § 19 Conditional release

**§ 19(1)** If one control-flow path releases a resource and another does not, the resource
may become only `PossiblyHeld` after the join.

**§ 19(2)** For example:

```sec
lock.Lock()

if condition {
    lock.Unlock()
}

other.Lock()
```

**§ 19(3)** may establish a possible, but not definite, order relationship from `lock` to
`other`.

**§ 19(4)** This distinction contributes to deadlock proof strength.

---

## § 20 Path compatibility

**§ 20(1)** Cycle edges must correspond to paths that can participate in a common reachable
execution.

**§ 20(2)** Two opposite lock orders occurring only on mutually exclusive paths in one
execution instance do not automatically form a proven deadlock.

**§ 20(3)** However, the same source-level mutually exclusive branches may be selected by
different concurrent runtime instances.

**§ 20(4)** Path compatibility is therefore evaluated together with execution-context
identity and instance multiplicity.

---

## § 21 Execution contexts

**§ 21(1)** Deadlock analysis reuses the canonical execution-context model used by other
concurrency analyses.

**§ 21(2)** Conceptual context classes may include:

```text
Thread
Task
ISR
ForeignCallback
PlatformExecution
Unknown
```

**§ 21(3)** The analysis consumes canonical relations such as:

```text
MayOverlap
CannotOverlap
MayPreempt
MayReenter
CompletionDependsOn
```

**§ 21(4)** It must not construct an incompatible parallel model of thread or task
semantics.

---

## § 22 Multiple runtime instances

**§ 22(1)** A finite execution-context abstraction may represent multiple runtime
instances created from one semantic creation site.

**§ 22(2)** The analysis must preserve whether:

```text
ContextClass(site X)
MayHaveConcurrentInstances = true
```

**§ 22(3)** or its semantic equivalent.

**§ 22(4)** Two runtime instances of the same function or task body may participate in one
deadlock when canonical execution semantics permit them to overlap.

---

## § 23 Synchronous calls

**§ 23(1)** An ordinary synchronous function call normally remains in the caller execution
context.

**§ 23(2)** It does not create an independent wait dependency merely because the caller
cannot continue until the callee returns.

**§ 23(3)** Instead, callee acquisitions, releases, waits, and resource state are propagated
into the caller context through interprocedural summaries.

---

## § 24 Task and thread completion

**§ 24(1)** Task and thread joins or equivalent completion waits are first-class deadlock
dependencies.

**§ 24(2)** For example:

```text
Task A waits for Task B
Task B waits for Task A
```

**§ 24(3)** may form a deadlock without a mutex.

**§ 24(4)** Mixed cycles are also relevant:

```text
A holds L
A waits for B
B waits for L
```

**§ 24(5)** The analysis consumes canonical join, completion, scheduling, and execution
semantics.

---

## § 25 Await and blocking operations

**§ 25(1)** A blocking or suspending operation is not automatically a deadlock.

**§ 25(2)** An effect such as:

```text
MayBlock
MaySuspend
```

**§ 25(3)** may help identify candidate wait sites, but deadlock analysis requires the
actual dependency behind the wait where that dependency is known.

**§ 25(4)** Effect analysis is an input and filtering mechanism; it does not replace
progress-dependency analysis.

---

## § 26 Progress providers

**§ 26(1)** For condition, event, completion, and similar waits, the analysis should retain
which execution context or resource can provide the required progress when that
information is canonical and available.

**§ 26(2)** For example:

```text
Task A waits for Event E
Task B can provide E
Task B waits for Lock L
Task A holds L
```

**§ 26(3)** may form a mixed cyclic dependency.

**§ 26(4)** If the progress provider cannot be identified, the result remains localized as
an unknown progress dependency rather than being connected to arbitrary
resources.

---

## § 27 Condition waits

**§ 27(1)** Condition-like synchronization must be interpreted according to its canonical
contract.

**§ 27(2)** If a condition wait atomically:

```text
releases a mutex
waits
reacquires the mutex
```

**§ 27(3)** held-resource analysis must model that behavior rather than assuming the mutex
remains held throughout the wait.

**§ 27(4)** This requirement is essential for avoiding false deadlock reports.

---

## § 28 Resource capacity

**§ 28(1)** Not every synchronization resource is an exclusive mutex.

**§ 28(2)** Canonical resource semantics may define:

```text
capacity
acquisition count
ownership model
release behavior
progress provider
reentrancy
blocking behavior
```

**§ 28(3)** A counting semaphore with capacity greater than one must not be silently
modeled as a capacity-one mutex.

**§ 28(4)** Sec 0.1 does not require arbitrary resource-allocation or Petri-net analysis,
but loss of capacity precision must weaken the result to
`PotentialDeadlock` or `Unknown` as appropriate rather than fabricate a proven
cycle.

---

## § 29 Reentrant resources

**§ 29(1)** Resource reentrancy is interpreted according to canonical synchronization
semantics.

**§ 29(2)** A resource that permits recursive acquisition by its current owning context does
not produce self-deadlock solely because the same resource is acquired again.

**§ 29(3)** Reentrancy for one owner does not imply that the resource is nonexclusive
between different execution contexts.

---

## § 30 Guaranteed progress and escape

**§ 30(1)** A deadlock proof is broken only by a canonical guarantee sufficient to ensure
that a participant can make the progress required to break the cycle.

**§ 30(2)** Examples may include, where canonical semantics guarantee them:

```text
guaranteed bounded timeout
guaranteed cancellation
guaranteed external completion
```

**§ 30(3)** A mechanism that merely may occur is not a positive deadlock-freedom proof.

**§ 30(4)** Conceptually:

```text
may escape
```

**§ 30(5)** is weaker than:

```text
must escape
```

**§ 30(6)** Only the latter can invalidate an otherwise complete permanent deadlock
witness.

---

## § 31 Lock-order consistency and partial orders

**§ 31(1)** Deadlock analysis may prove that a set of nested acquisitions follows a
consistent partial order.

**§ 31(2)** For example:

```text
DatabaseLock < CacheLock < LogLock
```

**§ 31(3)** may support a positive no-lock-order-cycle proof for the analyzed scope.

**§ 31(4)** Sec does not require a single global total order over every synchronization
resource.

**§ 31(5)** Resources that are never nested together need not have a relative order.

---

## § 32 Dynamic resource collections

**§ 32(1)** Resource identities may arise from indexed or ranged collections.

**§ 32(2)** For example:

```sec
locks[i].Lock()
locks[j].Lock()
```

**§ 32(3)** uses canonical index, range, Place, alias, and ordering facts.

**§ 32(4)** If `i == j` is proven and the resource is non-reentrant, self-deadlock may be
possible.

**§ 32(5)** If `i` and `j` are proven disjoint, the self-alias candidate is eliminated.

**§ 32(6)** If their relation is unknown, proof strength is weakened rather than assuming
identity or disjointness.

**§ 32(7)** A proven strict acquisition ordering over dynamic resource identities may be
consumed as evidence against order cycles.

---

## § 33 Ownership and resource transfer

**§ 33(1)** Deadlock analysis consumes canonical ownership and move semantics for resource
guards, synchronization tokens, and other values that represent held-resource
authority.

**§ 33(2)** If ownership of a guard moves to another execution context or value, the old
owner must not continue to be treated as holding the resource unless canonical
semantics say otherwise.

**§ 33(3)** The analysis does not redefine ownership transfer.

---

## § 34 ISR and preemption

**§ 34(1)** ISR execution can participate in deadlock analysis when canonical platform
semantics allow preemption or nesting.

**§ 34(2)** A high-value deadlock pattern is:

```text
Thread holds L
ISR preempts Thread
ISR waits for L
```

**§ 34(3)** If the thread cannot resume to release `L` until the ISR returns, and the ISR
cannot return until acquiring `L`, the cycle may form a `ProvenDeadlock`.

**§ 34(4)** Canonical interrupt masking, priority, or exclusion facts may eliminate the
cycle when they guarantee the ISR cannot execute during the relevant held
interval.

**§ 34(5)** ISR-specific legality and policy remain owned by ISR analysis.

---

## § 35 Foreign code and FFI

**§ 35(1)** Foreign code may participate in deadlock analysis through canonical FFI
contracts describing relevant behavior such as:

```text
MayBlock
MayAcquire(resource)
MayWaitForCompletion
MayInvokeCallback
MayReenter
CallbackExecutionContext
CompletionDependency
ForeignSynchronization
```

**§ 35(2)** No dependency is inferred from foreign function or parameter names.

**§ 35(3)** A foreign call whose contract says only:

```text
MayBlock
```

**§ 35(4)** but does not identify its dependency remains a localized unknown wait.

**§ 35(5)** It must not be connected speculatively to every resource in the program.

---

## § 36 Foreign callback reentry

**§ 36(1)** A foreign call can create self-deadlock through synchronous reentry.

**§ 36(2)** Conceptually:

```text
Sec holds L
    -> calls Foreign
        -> Foreign synchronously invokes callback
            -> callback waits for L
```

**§ 36(3)** If the callback target, reentry relation, resource identity, and non-reentrant
blocking semantics are all established, this may produce a complete
self-deadlock witness.

---

## § 37 Foreign completion cycles

**§ 37(1)** Foreign completion dependencies may participate in mixed cycles.

**§ 37(2)** For example:

```text
SecContext holds L
SecContext waits for ForeignCompletion
ForeignCompletion depends on SecCallback
SecCallback waits for L
```

**§ 37(3)** may form a cycle when supported by canonical FFI contracts.

---

## § 38 Function summaries

**§ 38(1)** Deadlock analysis exposes composable function summaries.

**§ 38(2)** Conceptually:

```text
DeadlockSummary {
    Acquisitions
    Releases
    Waits
    HeldOnReturn
    RequiredHeldOnEntry
    ExecutionEffects
    Precision
}
```

**§ 38(3)** The exact internal representation is implementation-defined.

**§ 38(4)** The summary must preserve enough symbolic information for callers to instantiate
resource and wait dependencies without requiring the callee body.

---

## § 39 Symbolic resource roots

**§ 39(1)** Summaries may refer to symbolic roots such as:

```text
Parameter(n)
Receiver
Static(symbol)
Captured(origin)
Foreign(origin)
AllocationSite(site)
Unknown
```

**§ 39(2)** Projection and Place precision must be retained where available.

---

## § 40 Symbolic acquisition order

**§ 40(1)** For example:

```sec
fn AcquireBoth(first: ref Mutex, second: ref Mutex) {
    first.Lock()
    second.Lock()
}
```

**§ 40(2)** may export the semantic relationship:

```text
Parameter(0) -> Parameter(1)
```

**§ 40(3)** or an equivalent structured summary.

**§ 40(4)** At a call site, alias analysis may instantiate both parameters to the same
resource and thereby expose a self-deadlock candidate.

---

## § 41 Caller-held propagation

**§ 41(1)** A callee may acquire a resource that was not known to be held inside the callee
summary itself.

**§ 41(2)** For example:

```sec
external.Lock()
CallLibrary()
```

**§ 41(3)** if `CallLibrary` may acquire `Static(CacheLock)`, the caller must be able to
establish:

```text
external -> Static(CacheLock)
```

**§ 41(4)** when `external` remains held across the call.

**§ 41(5)** Therefore summaries must preserve acquisition behavior separately from only
fully internal lock-order edges.

---

## § 42 Callee release of inherited resources

**§ 42(1)** If canonical ownership or guard semantics allow a callee to release or transfer
a caller-held resource, inherited held state must be updated before later
acquisitions are correlated.

**§ 42(2)** The analysis must not assume all caller-held resources remain held through every
call when canonical semantics prove otherwise.

---

## § 43 Call-site summary instantiation

**§ 43(1)** Symbolic resource, wait, completion, callback, and execution facts are
instantiated at call sites using canonical Place/provenance and identity
relations.

**§ 43(2)** Instantiation must preserve:

```text
Same
Disjoint
MayAlias
Unknown
```

**§ 43(3)** or equivalent precision rather than eagerly collapsing resources to unknown.

---

## § 44 Recursive call graphs

**§ 44(1)** Interprocedural deadlock summaries are solved over canonical call-graph SCCs.

**§ 44(2)** Ordinary recursion is not itself concurrency or deadlock.

**§ 44(3)** However recursion may expose deadlock-relevant resource behavior.

**§ 44(4)** For example, a recursive traversal that holds `node.Lock` while recursing to a
`Next` node may self-deadlock if canonical alias analysis proves the next node
can be the same resource and the lock is non-reentrant.

**§ 44(5)** Summary fixed points must be deterministic and declaration-order independent.

---

## § 45 Call-graph SCCs versus deadlock SCCs

**§ 45(1)** Call-graph recursion SCCs and deadlock dependency SCCs are distinct.

```text
CallGraph SCC:
    recursive or mutually recursive function/callable structure

WaitForGraph SCC:
    cyclic progress dependency
```

**§ 45(2)** The first is used to solve interprocedural summaries.

**§ 45(3)** The second is used to detect deadlock candidates.

**§ 45(4)** They must not be conflated.

---

## § 46 Callable and closure integration

**§ 46(1)** Closed callable target sets are consumed from canonical closure/callable-flow
analysis.

**§ 46(2)** If a callback may target multiple functions, deadlock-relevant summaries from
all permitted targets participate conservatively.

**§ 46(3)** A callback invoked while holding `L` can establish:

```text
L -> M
```

**§ 46(4)** when a possible callback target may acquire `M`.

**§ 46(5)** Open callable contracts may expose relevant behavior such as:

```text
MayBlock
MayAcquire(...)
MayWaitForCompletion
MayReenter
```

**§ 46(6)** Missing dimensions remain localized as Unknown.

---

## § 47 Finite resource abstraction

**§ 47(1)** The analysis must not require one static graph node per runtime resource
instance.

**§ 47(2)** Finite symbolic identities may include:

```text
ResourcePlace
ParameterOrigin
StaticResource
AllocationSite
AnyIndex(resource collection)
```

**§ 47(3)** or equivalent abstractions.

**§ 47(4)** Widening should preserve the most specific known root/projection rather than
collapsing immediately to all resources.

---

## § 48 Edge aggregation

**§ 48(1)** Equivalent abstract dependency edges may be aggregated.

**§ 48(2)** For example, many source instances corresponding to:

```text
L1 -> L2
```

**§ 48(3)** need not create an unbounded number of graph edges.

**§ 48(4)** Aggregated edges retain deterministic representative causes and may retain
occurrence information for detailed analysis views.

---

## § 49 Dependency-edge evidence

**§ 49(1)** A dependency edge retains enough evidence to explain its origin.

**§ 49(2)** Conceptually:

```text
DependencyEdge {
    From
    To
    Context
    SourceLocation
    CausePath
    PathConditionSummary
    ProofStrength
}
```

**§ 49(3)** The exact representation is implementation-defined.

**§ 49(4)** Evidence must be sufficient to construct stable cycle diagnostics and to avoid
strengthening a cycle beyond the strength of its constituent facts.

---

## § 50 Proof strength

**§ 50(1)** Individual dependency edges may differ in proof strength.

**§ 50(2)** An implementation may use categories equivalent to:

```text
ProvenPossible
ConservativePossible
Unknown
```

**§ 50(3)** The exact enum names are not normative.

**§ 50(4)** A cycle based on precise canonical relationships can support stronger
classification than a cycle that depends on unresolved aliasing, path
conditions, resource capacity, or foreign behavior.

---

## § 51 Cycle realizability

**§ 51(1)** `ProvenDeadlock` requires more than graph cyclicity.

**§ 51(2)** The analysis must establish enough of the following to support a complete
witness:

```text
reachability of every participant
resource identity compatibility
path compatibility
execution-context overlap, preemption, or reentry where required
blocking semantics for every wait edge
resource ownership/progress dependencies
resource capacity where relevant
absence of a guaranteed cycle-breaking progress mechanism
```

**§ 51(3)** A missing fact weakens classification rather than being silently assumed.

---

## § 52 Concurrent realizability

**§ 52(1)** For multi-context cycles, participating execution contexts must be able to
coexist according to canonical execution semantics.

**§ 52(2)** If two contexts are proven unable to overlap, a lock-order inversion between
them cannot form a simultaneous multi-context deadlock.

**§ 52(3)** This does not eliminate self-deadlock within one context.

---

## § 53 Reentry realizability

**§ 53(1)** Reentrant callback or platform execution may create deadlock without ordinary
thread overlap.

**§ 53(2)** The analysis must use canonical `MayReenter` or equivalent relationships rather
than requiring two independent threads for every cycle.

---

## § 54 Resource identity and proof classification

**§ 54(1)** Possible resource aliasing can support a meaningful `PotentialDeadlock` when a
concrete dependency structure exists, but possible aliasing alone must not
fabricate `ProvenDeadlock`.

**§ 54(2)** A proven deadlock witness requires resource identity facts strong enough to
support every cycle edge that depends on aliasing.

---

## § 55 Blocking semantics and cycle proof

**§ 55(1)** Only dependencies whose canonical semantics can block contribute blocking wait
edges.

**§ 55(2)** For example:

```text
BlockingAcquire
CompletionWait
ConditionWait
```

**§ 55(3)** may participate when their contracts establish the relevant dependency.

**§ 55(4)** A nonblocking acquisition that simply returns failure is not treated as a
permanent wait edge.

---

## § 56 Resource capacity and proof classification

**§ 56(1)** Resource capacity may determine whether all participants can be blocked
simultaneously.

**§ 56(2)** When capacity information is insufficient, the result is weakened to
`PotentialDeadlock` or `Unknown` according to the amount of dependency structure
that remains known.

**§ 56(3)** Insufficient capacity analysis must never be replaced by an assumption that the
resource behaves as an exclusive mutex.

---

## § 57 Precision widening

**§ 57(1)** Deadlock analysis may use bounded abstractions to avoid state or path explosion.

**§ 57(2)** Widening may reduce precision through states such as:

```text
precise resource identity
    -> bounded resource class
    -> localized unknown resource dimension
```

**§ 57(3)** or:

```text
precise path relationship
    -> summarized compatible path class
    -> unknown path compatibility
```

**§ 57(4)** Loss of precision may weaken:

```text
ProvenDeadlock -> PotentialDeadlock -> Unknown
```

**§ 57(5)** but must never strengthen an unsupported result.

---

## § 58 Deep precision can strengthen or eliminate a potential finding

**§ 58(1)** Increasing precision is not the same as widening.

**§ 58(2)** Deep analysis may legitimately refine:

```text
PotentialDeadlock -> ProvenDeadlock
```

**§ 58(3)** when it establishes the missing witness.

**§ 58(4)** It may also refine:

```text
PotentialDeadlock -> no finding
```

**§ 58(5)** when it proves path incompatibility, resource disjointness, guaranteed progress,
or non-overlapping execution.

**§ 58(6)** The underlying deadlock semantics do not change.

---

## § 59 Avoiding path explosion

**§ 59(1)** The normative analysis does not require exhaustive enumeration of every runtime
path.

**§ 59(2)** Sound implementations may use:

```text
path summaries
symbolic conditions
SCC condensation
bounded context sensitivity
resource abstractions
worklist fixed points
widening
```

**§ 59(3)** provided unsupported precision is represented as `PotentialDeadlock` or
`Unknown` rather than as a fabricated proof.

---

## § 60 Representative cycle witness

**§ 60(1)** A deadlock finding retains a representative cycle.

**§ 60(2)** The selected witness should favor, in order of semantic usefulness:

```text
stronger proof
smaller understandable dependency cycle
precise resource and context identity
clear source locations
less uncertainty
```

**§ 60(3)** The implementation may refine this ordering, but selection must be
deterministic.

**§ 60(4)** A graph-theoretically shortest cycle is not required if another stable cycle
provides materially clearer source evidence.

---

## § 61 Interprocedural cause paths

**§ 61(1)** When an edge is established through nested calls, the finding should retain the
interprocedural cause path.

**§ 61(2)** For example:

```text
Handler
  -> Update
    -> Cache.Store
      -> acquire CacheLock
```

**§ 61(3)** may explain the edge:

```text
DatabaseLock -> CacheLock
```

**§ 61(4)** Normal diagnostics may present the compact edge while detailed tooling exposes
the full cause path.

---

## § 62 Diagnostics

**§ 62(1)** Deadlock diagnostics preserve the distinction between proven, potential, and
unknown results.

### § 62.1 Proven deadlock

**§ 62.1(1)** A proven finding states that the deadlock can occur.

**§ 62.1(2)** Conceptually:

```text
error: deadlock can occur

Thread A:
    holds `database.Lock`
    waits for `cache.Lock`

Thread B:
    holds `cache.Lock`
    waits for `database.Lock`
```

**§ 62.1(3)** Exact wording and severity are governed by the canonical diagnostic rules.

### § 62.2 Potential deadlock

**§ 62.2(1)** A potential finding states that a meaningful dependency cycle exists but that
the complete blocked-state witness is not proven.

**§ 62.2(2)** Conceptually:

```text
warning: potential deadlock

one reachable path acquires:
    `database.Lock` -> `cache.Lock`

another concurrent path acquires:
    `cache.Lock` -> `database.Lock`

simultaneous blocked-state reachability could not be fully proven
```

### § 62.3 Unknown

**§ 62.3(1)** Unknown is not presented as a potential deadlock unless a meaningful dependency
cycle has been established.

**§ 62.3(2)** Detailed analysis may instead report:

```text
deadlock status unknown

reason:
    foreign call may block, but its wait dependency is unspecified
```

---

## § 63 Failure-to-prove diagnostics

**§ 63(1)** When an active policy requires positive deadlock-freedom proof and the analysis
cannot establish it, the diagnostic states that the proof is unavailable.

**§ 63(2)** It must not claim either a proven or potential deadlock unless the corresponding
classification is independently established.

---

## § 64 Diagnostic navigation

**§ 64(1)** Deadlock findings are relational and may span several locations.

**§ 64(2)** LSP diagnostics should expose a primary location and related locations for the
resource acquisitions, waits, completion dependencies, callbacks, and other
cycle edges.

**§ 64(3)** The user should be able to navigate through the representative cycle.

---

## § 65 Diagnostic presentation

**§ 65(1)** Normal diagnostics present a minimal understandable witness rather than dumping
the complete `WaitForGraph`.

**§ 65(2)** Detailed tooling may expose:

```text
full cause paths
all relevant cycle edges
resource alias facts
held-resource states
execution relations
unknown dimensions
additional equivalent occurrences
```

---

## § 66 Automatic fixes

**§ 66(1)** Deadlock analysis must not offer a generic transformation such as:

```text
reverse these locks
```

**§ 66(2)** unless a canonical semantic contract proves that the transformation preserves
program behavior and satisfies the intended synchronization order.

**§ 66(3)** Lock ordering is often an architectural choice.

**§ 66(4)** Baseline tooling prioritizes:

```text
explanation
navigation
representative cycle
resource order
cause paths
```

**§ 66(5)** over automatic rewriting.

---

## § 67 Positive deadlock-free proofs

**§ 67(1)** The analysis may retain scoped positive facts such as:

```text
NoLockOrderCycle
NoDeadlockCycle
```

**§ 67(2)** when proven for a particular scope, resource set, context relation, and set of
assumptions.

**§ 67(3)** A positive proof is scoped; it must not be generalized to an entire program
when only a subset of resources or execution contexts was analyzed.

**§ 67(4)** Such proofs are primarily reusable analysis facts and OptionalInsight tooling
information.

---

## § 68 Analysis budgets

**§ 68(1)** Deadlock analysis uses the common Sec analysis-budget model:

```text
Interactive
Standard
Deep
```

**§ 68(2)** No additional global `Intermediate` analysis mode is introduced.

---

## § 69 Interactive analysis

**§ 69(1)** Interactive analysis prioritizes fast sound results from local facts and cached
summaries.

**§ 69(2)** High-value Interactive checks include:

```text
direct self-lock
direct lock-order inversion
cached interprocedural inversion
join while holding a known required resource
known callback reentry
known ISR/preemption self-deadlock
```

**§ 69(3)** Interactive analysis may remain incomplete while refinement is pending.

---

## § 70 Progressive LSP refinement

**§ 70(1)** LSP execution is progressively refined within the existing analysis modes.

**§ 70(2)** Conceptually:

```text
Interactive fast result
    -> incremental interprocedural refinement
    -> Standard-equivalent result
    -> optional Deep refinement
```

**§ 70(3)** The intermediate refinement step is scheduling and precision work within the
existing model, not a fourth language-analysis mode.

---

## § 71 Pending tooling state

**§ 71(1)** LSP analysis may use a first-class `Pending` state while invalidated summaries,
resource identities, callback targets, or cycle proofs are being recomputed.

**§ 71(2)** `Pending` is distinct from:

```text
ProvenDeadlock
PotentialDeadlock
Unknown
```

**§ 71(3)** A stale proven or potential finding must not continue to be treated as a current
result after one of its proof dependencies has been invalidated.

---

## § 72 Standard analysis

**§ 72(1)** Standard analysis is the stable analysis required for normal compilation.

**§ 72(2)** Any deadlock proof required by active language, target, runtime, ISR, project,
or CompilationPlan semantics is mandatory in Standard analysis.

**§ 72(3)** Required proof cannot be deferred to Deep merely because it is expensive.

---

## § 73 Deep analysis

**§ 73(1)** Deep analysis may increase precision through more expensive reasoning such as:

```text
path-sensitive held-resource state
context-sensitive resource identities
larger WaitForGraph correlation
dynamic collection ordering
foreign callback/reentry refinement
resource-capacity refinement
richer cycle realizability proofs
```

**§ 73(2)** Deep does not change deadlock semantics.

---

## § 74 LSP presentation classes

**§ 74(1)** Deadlock analysis uses the common LSP distinction between information the
programmer needs to see and information that is optional insight.

### § 74.1 NeedToKnow

**§ 74.1(1)** Normally includes:

```text
ProvenDeadlock
high-confidence PotentialDeadlock
required deadlock-freedom proof failure
```

**§ 74.1(2)** Exact severity remains governed by canonical diagnostic policy.

### § 74.2 OptionalInsight

**§ 74.2(1)** May include:

```text
positive no-cycle explanations
canonical lock-order explanation
held-resource information
execution-context relationships
full WaitForGraph details
precision-loss explanations when no required proof fails
```

**§ 74.2(2)** Optional insight must remain configurable.

---

## § 75 LSP graph views

**§ 75(1)** A lock-order view is a natural OptionalInsight representation.

**§ 75(2)** For example:

```text
DatabaseLock
    -> CacheLock
    -> LogLock
```

**§ 75(3)** A detailed Deep view may expose mixed dependencies such as:

```text
ThreadA -> TaskB -> LockC -> ThreadD -> EventE -> ThreadA
```

**§ 75(4)** These views are tooling presentations, not language syntax.

---

## § 76 LSP configuration reload

**§ 76(1)** Changes to project analysis depth or optional insight settings must be observed
without requiring an LSP restart.

**§ 76(2)** Affected summaries, cycle classifications, diagnostics, hovers, and analysis
views are refreshed according to the common LSP analysis model.

---

## § 77 Incremental invalidation

**§ 77(1)** Deadlock summaries and findings are invalidated when relevant dependencies
change.

**§ 77(2)** Examples include:

```text
function body
callee deadlock summary
callable target set
resource identity
Place/provenance facts
acquire/release semantics
guard lifetime or destruction
ownership transfer
task/thread relation
join/completion relation
callback/reentry behavior
FFI contract
ISR/preemption model
resource capacity or reentrancy
CompilationPlan where relevant
analysis schema/version
```

**§ 77(3)** Invalidation is dependency-driven rather than project-wide when unrelated facts
can remain valid.

---

## § 78 Stable-summary propagation

**§ 78(1)** If a function is reanalyzed and its exported deadlock summary is semantically
equivalent to the previous summary, dependent callers need not be invalidated
further solely because the body changed.

**§ 78(2)** Likewise, when one dependency edge disappears, findings whose representative or
supporting cycles require that edge are invalidated even if other edges remain.

---

## § 79 Separate compilation

**§ 79(1)** Separate compilation may use versioned symbolic deadlock summaries instead of
callee bodies.

**§ 79(2)** An exported summary may describe facts such as:

```text
resources acquired
resources released
resource-order edges
wait dependencies
resources held on return
required held state on entry
execution effects
callback/reentry behavior
localized unknown dimensions
```

**§ 79(3)** Callers instantiate symbolic origins using canonical Place/provenance and
resource identity.

**§ 79(4)** Missing imported information widens only the affected dimensions.

---

## § 80 Summary versioning and invalidation

**§ 80(1)** Persisted summaries are invalidated when relevant semantics change, including:

```text
body
callee summaries
call graph
closure/callable targets
Place/provenance
resource contracts
ownership or lifetime semantics
defer or cleanup semantics
task/thread runtime model
FFI contracts
ISR/platform semantics
CompilationPlan where applicable
analysis schema/version
```

**§ 80(2)** The cache key or equivalent compatibility mechanism must be sufficient to avoid
using stale summaries under incompatible semantics.

---

## § 81 CompilationPlan dependence

**§ 81(1)** Many source-level lock-order facts are target-independent.

**§ 81(2)** Deadlock results become CompilationPlan-dependent only where behavior actually
differs, including cases such as:

```text
runtime task scheduler semantics
ISR preemption and masking
platform synchronization primitives
foreign implementation contracts
resource-capacity semantics
callback or reentry behavior
```

**§ 81(3)** The analysis should not introduce unnecessary target splitting.

---

## § 82 Determinism

**§ 82(1)** For the same source, compiler version, relevant project configuration, and
CompilationPlan, deadlock analysis must produce deterministic semantic results.

**§ 82(2)** This includes stable:

```text
classification
representative cycle
primary diagnostic location
related diagnostic locations
cause path selection
```

**§ 82(3)** Results must not depend on declaration order, worklist order, cache traversal,
or parallel compiler scheduling.

---

## § 83 `sec analyse`

**§ 83(1)** Deadlock analysis participates in the default all-analysis behavior of:

```text
sec analyse
```

**§ 83(2)** Tooling may expose an explicit deadlock-only selection, conceptually:

```text
sec analyse deadlocks
```

**§ 83(3)** `deadlocks` is the canonical analysis name (`rules/tooling/cli.md` § 5.2,
MD-017).

**§ 83(4)** Deep output may include:

```text
proven deadlock cycles
potential cycles
unknown waits
lock-order graph
relevant WaitForGraph subgraphs
resource aliases
held-resource state
join/completion dependencies
callback and reentry edges
ISR/preemption edges
foreign dependencies
precision-loss causes
positive no-cycle proofs
```

---

## § 84 Required baseline capabilities for Sec 0.1

**§ 84(1)** The Sec 0.1 baseline requires at least:

```text
symbolic synchronization-resource identities
flow-sensitive held-resource tracking
acquire and release semantics
defer/scoped-guard held intervals
interprocedural acquisition summaries
caller-held propagation
resource alias/self-alias detection
lock-order graph construction
generalized wait dependencies
self-deadlock analysis
direct and mutual lock-order cycles
task/thread completion dependencies
join while holding a resource
closed callable callback analysis
reentry contracts
call-graph SCC summary fixed points
bounded resource abstraction
ProvenDeadlock / PotentialDeadlock / Unknown classification
localized Unknown
incremental LSP integration
deterministic representative witnesses
```

**§ 84(2)** Resource-specific condition, event, semaphore, and foreign precision may be
conservative where canonical contracts exist but full proof support is not yet
available. Such limitations must be represented through the result classes
rather than by incorrect mutex assumptions.

---

## § 85 Required test philosophy

**§ 85(1)** Tests must cover both positive findings and false-positive resistance.

**§ 85(2)** A deadlock analyzer that reports every unusual synchronization pattern is not
conforming.

**§ 85(3)** The test suite must verify:

```text
classification strength
resource identity precision
path/execution compatibility
interprocedural propagation
wait dependency construction
incremental invalidation
determinism
bounded graph behavior
```

---

## § 86 Required ProvenDeadlock tests

**§ 86(1)** At minimum:

```text
same non-reentrant lock acquired twice
two-context L1/L2 inversion with realizable overlap
three-resource realizable cycle
task join cycle A -> B -> A
thread join cycle A -> B -> A
join while holding a lock needed by the joined worker
synchronous reentrant callback reacquiring a held non-reentrant lock
ISR waiting for a lock held by the preempted context
mixed task/resource cycle with complete progress dependencies
```

---

## § 87 Required PotentialDeadlock tests

**§ 87(1)** At minimum:

```text
lock-order inversion with unresolved path compatibility
dynamic resource alias may create a cycle
resource-capacity uncertainty prevents complete proof
recursive dynamic lock chain with incomplete identity ordering
concurrent context relation supports the candidate but not the full witness
meaningful foreign/resource cycle with one unresolved compatibility dimension
```

**§ 87(2)** These tests must not be upgraded to `ProvenDeadlock` without the missing proof.

---

## § 88 Required Unknown tests

**§ 88(1)** At minimum:

```text
foreign blocking target unknown
open callback blocking behavior unknown
unknown event progress provider
lost synchronization-resource provenance
unknown platform preemption semantics
unknown custom synchronization contract
```

**§ 88(2)** Unknown behavior must not become `PotentialDeadlock` without a concrete
supported dependency cycle.

---

## § 89 Required safe/non-deadlock tests

**§ 89(1)** At minimum:

```text
consistent L1 -> L2 partial ordering
sequential contexts proven unable to overlap
resource released before second acquisition
reentrant reacquisition where permitted
nonblocking TryLock failure path
guaranteed bounded timeout that breaks permanent wait
interrupt masking that prevents ISR overlap
proven-disjoint resources with no cycle
join performed after releasing the worker-required resource
```

**§ 89(2)** False positives in these cases are regressions.

---

## § 90 Interprocedural tests

**§ 90(1)** At minimum:

```text
caller holds L1 and callee acquires L2
callee acquires and releases before return
callee returns holding a resource
guard passed to callee and released
nested calls create transitive order relationships
recursive call-graph SCC exposes self-lock
summary instantiation aliases two symbolic parameters
summary instantiation proves symbolic parameters disjoint
caller-held resource combined with callee internal static acquisition
```

---

## § 91 Defer and cleanup tests

**§ 91(1)** At minimum:

```text
deferred unlock keeps a resource held across a call
early return releases a scoped guard according to canonical semantics
error path retains a resource until cleanup
panic cleanup releases where canonical semantics guarantee release
conditional release yields PossiblyHeld rather than DefinitelyHeld
```

---

## § 92 Task and thread tests

**§ 92(1)** At minimum:

```text
task waits for task
thread waits for thread
self-join
mixed task-lock cycle
multiple runtime instances from one creation site
scheduler-guaranteed serialization removes a candidate
mere implementation scheduling coincidence does not remove a candidate
```

---

## § 93 Callable and closure tests

**§ 93(1)** At minimum:

```text
closed callback target acquires a resource
multi-target callback with one dangerous target
open callback contract with acquisition facts
unknown open callback behavior
captured synchronization-resource identity
reentrant closure callback
callback invoked while caller-held resource remains active
```

---

## § 94 FFI tests

**§ 94(1)** At minimum:

```text
synchronous foreign call without reentry
synchronous foreign reentrant callback
foreign worker completion dependency
foreign wait for Sec callback
foreign-held synchronization resource
MayBlock with unknown wait target
precise foreign contract forms a complete cycle
missing contract remains localized Unknown
```

**§ 94(2)** No test may rely on foreign function or parameter names to infer semantics.

---

## § 95 ISR tests

**§ 95(1)** At minimum:

```text
ISR reacquires thread-held non-reentrant mutex
masked ISR cannot preempt the critical section
higher-priority ISR nesting
known exclusion between ISR priority classes
unknown nesting semantics
ISR waiting on task/thread completion where the canonical model permits such a wait
```

**§ 95(2)** ISR policy violations remain owned by ISR analysis even when the same facts also
form a deadlock dependency.

---

## § 96 Resource-semantics tests

**§ 96(1)** At minimum:

```text
non-reentrant mutex
reentrant mutex
binary semaphore
counting semaphore with capacity greater than one
condition wait that releases and reacquires its mutex
event with known progress provider
event with unknown progress provider
nonblocking resource acquisition
bounded wait with guaranteed escape
```

**§ 96(2)** The suite must verify that non-mutex resources retain their own canonical
semantics.

---

## § 97 Dynamic-resource tests

**§ 97(1)** At minimum:

```text
locks[i] then locks[j] with proven strict ordering
locks[i] then locks[j] with unknown ordering
same index proven
different indices proven
sorted acquisition by canonical ordering fact
dynamic aliases across parameters
bounded AnyIndex resource abstraction
```

---

## § 98 LSP incremental tests

**§ 98(1)** At minimum:

```text
introduce reverse lock order and observe finding appear
restore common order and observe finding disappear
change callee acquisition and refresh affected caller cycles
change callback target and refresh affected cycles
change FFI reentry contract and refresh affected cycles
change unrelated function and retain unaffected cached findings
Pending is never presented as a current proven result
project analysis-depth change is applied without LSP restart
Deep refinement upgrades PotentialDeadlock to ProvenDeadlock when a witness is found
Deep refinement removes PotentialDeadlock when incompatibility is proven
```

---

## § 99 Diagnostic stability tests

**§ 99(1)** LSP refinement should preserve a stable finding identity when the same semantic
cycle remains present across Interactive, Standard, and Deep refinement.

**§ 99(2)** Tests must verify that additional proof detail does not unnecessarily create a
new unrelated diagnostic identity.

**§ 99(3)** Stable diagnostic IDs are governed by the canonical diagnostic rules.

---

## § 100 Performance and scalability tests

**§ 100(1)** Large synthetic tests must cover:

```text
many resources
many order edges
large call graphs
large recursive SCCs
many task creation sites
many callbacks
large acyclic lock hierarchies
many equivalent dynamic resource edges
large wait-for graphs with few actual cycles
```

**§ 100(2)** The tests verify:

```text
bounded symbolic graph construction
SCC/cycle scalability
edge aggregation
incremental invalidation
deterministic representative cycle selection
```

**§ 100(3)** No normative wall-clock threshold is defined by this rulebook.

---

## § 101 False-positive regression corpus

**§ 101(1)** The compiler should maintain a permanent regression corpus for safe patterns
that coarse deadlock analyses commonly misclassify.

**§ 101(2)** At minimum it should include:

```text
common global partial lock order
scoped locks
temporary unlock before nested operation
condition waits that release a mutex
nonblocking TryLock
guaranteed timeout
serialized actor/task execution
ISR masking
ordered dynamic lock arrays
resource ownership transfer
reentrant resource use where permitted
join after required resource release
```

**§ 101(3)** Every legitimate false positive discovered during implementation or external
review should become a regression test.

---

## § 102 Interaction with other analyses

**§ 102(1)** Deadlock analysis consumes canonical facts from analyses and rulebooks including:

```text
CallGraph
ClosureAnalysis
EffectAnalysis
Place/Provenance
Ownership
Lifetime
Destruction and Defer
ConcurrencyMemoryModel
DataRace-related shared execution/synchronization facts
FFI contracts
ISR/platform execution semantics
CompilationPlan
```

**§ 102(2)** It owns only cyclic blocking/progress dependency analysis.

**§ 102(3)** It must not absorb or duplicate the normative semantics of those sources.

---

## § 103 Completion criteria for Sec 0.1

**§ 103(1)** Deadlock analysis is complete for Sec 0.1 when the compiler can soundly:

```text
track canonical synchronization-resource identities
track held-resource intervals
propagate acquire and release interprocedurally
build lock-order relationships
build generalized wait dependencies
reason about self-deadlock
reason about task/thread completion cycles
consume callback/reentry contracts
consume FFI synchronization and completion contracts
consume ISR/preemption facts
distinguish resource reentrancy and capacity semantics
find dependency cycles with bounded symbolic analysis
classify ProvenDeadlock, PotentialDeadlock, and Unknown
preserve failure-to-prove separately from deadlock findings
produce deterministic representative cycle witnesses
support separate-compilation summaries
integrate incrementally with the LSP
provide required Standard proofs and optional Deep refinement
pass the required safety, false-positive, incremental, scalability, and determinism tests
```

---

## § 104 Implementation governance

**§ 104(1)** Normative deadlock semantics belong in:

```text
rules/analysis/deadlock_analysis.md
```

**§ 104(2)** Mutable implementation state, code locations, remaining implementation work,
and verification commands belong in:

```text
implementation-status.yaml
```

**§ 104(3)** The canonical ledger integration identifier is:

```text
sema.deadlock-analysis
```

**§ 104(4)** A suitable implementation capability breakdown includes:

```text
resource_identity
held_resource_state
acquire_release
lock_order
wait_for_graph
self_deadlock
interprocedural_summaries
join_dependencies
callback_reentry
isr_preemption
resource_capacity
cycle_proof
incremental_lsp
deep_reporting
```

**§ 104(5)** Capability status must describe observable repository implementation rather than
aspirational behavior.

---

## § 105 Normative summary

**§ 105(1)** Deadlock analysis determines whether reachable execution contexts and
synchronization resources can form cyclic blocking or progress dependencies that
prevent required progress.

**§ 105(2)** The analysis is distinct from data-race analysis even though both consume
canonical synchronization, execution-context, ownership, and Place facts.

**§ 105(3)** Deadlock analysis is not limited to mutexes. Locks, task/thread completion,
conditions, events, foreign completion, platform synchronization, and other
canonical blocking resources may participate in mixed dependency cycles.

**§ 105(4)** Synchronization-resource identity is derived from canonical semantic identity
and Place/provenance rather than naming conventions.

**§ 105(5)** Held-resource state is flow-sensitive and distinguishes definite from possible
ownership. Acquisition, release, guard lifetime, move, destruction, defer,
cleanup, and error paths determine the actual held interval.

**§ 105(6)** A lock-order edge records that one resource may be acquired while another
remains held. A lock-order cycle is a deadlock candidate, not automatically a
proven deadlock.

**§ 105(7)** The generalized WaitForGraph represents dependencies between execution
contexts, resources, completion events, and progress providers. The
LockOrderGraph is a useful projection but does not replace the generalized
model.

**§ 105(8)** Task/thread joins, callback reentry, foreign completion, and ISR preemption may
participate in deadlock cycles according to their canonical semantics.

**§ 105(9)** Resource reentrancy, capacity, nonblocking acquisition, condition release and
reacquire behavior, timeout, cancellation, and other progress semantics are
consumed from canonical contracts rather than guessed.

**§ 105(10)** Interprocedural deadlock summaries preserve symbolic resource identities,
acquisitions, releases, waits, held-on-return state, execution effects, and
localized uncertainty. Callers instantiate those summaries using canonical
Place/provenance and resource identity.

**§ 105(11)** Call-graph SCCs are used for interprocedural summary fixed points. Deadlock
cycles are analyzed separately in lock-order or wait-for dependency graphs.

**§ 105(12)** `ProvenDeadlock` requires a sound reachable witness in which all cycle
dependencies can be simultaneously realized and no guaranteed progress
mechanism breaks the cycle.

**§ 105(13)** `PotentialDeadlock` requires a meaningful supported dependency cycle but allows
bounded uncertainty in path, identity, scheduling, capacity, or related
compatibility facts.

**§ 105(14)** `Unknown` is used when dependency information is insufficient to establish such
a meaningful cycle.

**§ 105(15)** Failure to prove a required deadlock-freedom property remains distinct from all
three classifications.

**§ 105(16)** Precision widening may weaken results but never fabricate stronger proof.
Additional Deep precision may strengthen a potential finding to a proven
finding or eliminate it when incompatibility is established.

**§ 105(17)** Deadlock diagnostics provide deterministic representative cycles, participating
resources and contexts, source locations, and interprocedural cause paths.
Normal diagnostics present a minimal understandable witness rather than the
entire dependency graph.

**§ 105(18)** LSP execution uses progressive refinement within Interactive, Standard, and
Deep and introduces no separate Intermediate mode. Pending state is distinct
from current findings, and stale proofs are invalidated when their dependencies
change.

**§ 105(19)** Proven deadlocks and high-confidence potential deadlocks are NeedToKnow
information. Positive no-cycle explanations and detailed graph views are
configurable OptionalInsight.

**§ 105(20)** Separate compilation uses versioned symbolic deadlock summaries. Missing
information widens only affected dimensions.

**§ 105(21)** Results, representative cycles, and diagnostics are deterministic and
independent of compiler scheduling.

**§ 105(22)** Normative behavior belongs in this rulebook. Mutable implementation progress
belongs in `implementation-status.yaml`.
