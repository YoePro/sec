# Deadlock Analysis

## Status

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

# Purpose

Deadlock analysis answers questions such as:

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

Deadlock analysis is not a replacement for data-race, effect, ownership,
lifetime, Place/provenance, call-graph, closure, task/thread, FFI, ISR, or
concurrency-memory-model analysis.

It consumes canonical facts from those analyses and correlates them.

---

# Normative ownership

Canonical concurrency and synchronization rules own:

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

Deadlock analysis owns:

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

The analysis must not redefine synchronization or scheduler semantics merely to
make deadlock reasoning easier.

---

# Scope

Deadlock is a cyclic blocking/progress dependency.

This rulebook does not redefine:

```text
data race
livelock
starvation
performance contention
long but finite blocking
```

Conceptually:

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

Only deadlock is normative here.

---

# General dependency model

Deadlock analysis is not limited to mutexes.

An execution context may wait on canonical progress from:

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

The implementation may use different internal categories, but the semantic
model must be able to represent mixed cycles involving both execution contexts
and synchronization resources.

Conceptually, the analysis reasons about:

```text
ExecutionContext
SynchronizationResource
HeldResource
WaitDependency
ProgressDependency
```

and relationships such as:

```text
Holds(Context, Resource)
WaitsFor(Context, ResourceOrProgress)
ProgressProvidedBy(ResourceOrProgress, Context)
```

---

# Canonical synchronization-resource identity

Synchronization resources are identified through canonical semantic identity,
including Place/provenance where applicable.

For example:

```sec
state.Lock
```

may be represented symbolically as:

```text
Parameter(0).Lock
```

inside an exported function summary.

Two resource expressions must remain distinguishable as:

```text
Same
Disjoint
MayAlias
Unknown
```

or equivalent semantic states supported by canonical Place/provenance analysis.

Names such as `mutex`, `stateLock`, or `cacheGuard` are never proof of resource
identity.

---

# Lock-order relationships

A lock-order relationship:

```text
L1 -> L2
```

means that there exists a reachable execution in which `L2` may be acquired
while `L1` remains held.

For example:

```sec
first.Lock()
second.Lock()
```

may establish:

```text
first -> second
```

provided `first` is still held when acquisition of `second` occurs.

A single order edge is not a deadlock.

A path such as:

```text
L1 -> L2 -> L3
```

is not a deadlock unless relevant dependencies form a realizable cycle.

---

# LockOrderGraph and WaitForGraph

The analysis may maintain both a lock-order projection and a generalized
wait-for representation.

The `LockOrderGraph` is useful for efficient analysis of nested resource
acquisitions.

The generalized `WaitForGraph` can represent relationships such as:

```text
Execution -> Resource
Resource  -> OwningExecution
Execution -> TaskCompletion
Execution -> ThreadCompletion
Execution -> ForeignCompletion
Execution -> EventProgress
```

The exact graph representation is implementation-defined.

The generalized wait-for model is normative because deadlocks may involve
mixed dependencies that a pure lock-order graph cannot express.

---

# A cycle is initially a candidate

A cycle in a lock-order or wait-for graph does not by itself prove deadlock.

A complete deadlock proof additionally requires enough information to show that
its dependencies can coexist in a reachable execution.

Conceptually:

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

This expression is explanatory. The implementation need not use this exact
form.

---

# Core result classes

The analysis distinguishes:

```text
ProvenDeadlock
PotentialDeadlock
Unknown
```

## ProvenDeadlock

`ProvenDeadlock` means the compiler has a sound reachable witness in which all
participants can simultaneously realize a cyclic wait dependency and no
participant has a guaranteed mechanism that allows the required progress to
break the cycle.

## PotentialDeadlock

`PotentialDeadlock` means the compiler has established a meaningful reachable
cyclic dependency, but one or more compatibility, identity, scheduling,
capacity, or path facts are not strong enough to establish a complete
simultaneous blocked-state witness.

A potential deadlock is not equivalent to arbitrary uncertainty.

## Unknown

`Unknown` means deadlock-relevant information is insufficient to establish even
a sufficiently concrete cyclic dependency.

Examples include:

```text
foreign call may block but its wait target is unspecified
resource provenance is lost
custom synchronization semantics are unknown
platform preemption relationship is unknown
open callback behavior lacks a usable contract
```

Unknown must not be silently converted into `PotentialDeadlock`.

---

# Failure to prove required deadlock freedom

An active language, target, runtime, ISR, project, or CompilationPlan policy may
require a positive deadlock-freedom proof for a particular context.

If the analysis result is `Unknown`, compilation may fail because the required
proof could not be established.

Such a failure is distinct from both:

```text
ProvenDeadlock
PotentialDeadlock
```

The diagnostic must state failure to prove the required property rather than
claiming that a deadlock has been proven.

---

# Self-deadlock

Deadlock does not require two execution contexts.

For a non-reentrant exclusive resource:

```sec
lock.Lock()
lock.Lock()
```

may establish a self-deadlock when the first acquisition remains active and the
second acquisition blocks waiting for the same resource.

Self-deadlock analysis must use canonical resource identity and reentrancy
semantics.

A self-order edge:

```text
L -> L
```

is not automatically invalid for a resource whose canonical semantics permit
reentrant acquisition by the same execution context.

---

# Held-resource state

Deadlock analysis is flow-sensitive with respect to held resources.

At each relevant program point the compiler must retain enough information to
distinguish the semantic equivalents of:

```text
DefinitelyHeld
PossiblyHeld
```

A coarse set of all resources used by the function is insufficient.

For example:

```sec
first.Lock()
first.Unlock()
second.Lock()
```

must not establish:

```text
first -> second
```

when canonical semantics prove `first` has been released before acquisition of
`second`.

---

# Acquisition

When a blocking resource is acquired while other resources remain held, the
analysis may establish order/dependency relationships from the held resources
to the acquired resource.

For example, if:

```text
DefinitelyHeld = {A, B}
```

and `C` is acquired, the analysis may establish:

```text
A -> C
B -> C
```

A resource that is only possibly held contributes weaker evidence than a
resource definitely held on the relevant path.

Proof strength must be preserved.

---

# Wait versus successful acquisition

For blocking resource operations, the analysis distinguishes the wait event
from successful ownership.

Conceptually:

```text
WaitFor(L)
AcquireSucceeded(L)
```

During the wait, the current execution depends on progress associated with
`L`.

After successful acquisition, `L` becomes held according to the resource's
canonical ownership semantics.

---

# Release

Release removes the relevant resource from held state when canonical identity
and ownership semantics establish that the released resource is the held
resource.

If alias or ownership information is incomplete, uncertainty remains localized
to the affected held-state relationship.

---

# Nonblocking and fallible acquisition

A synchronization operation that can fail without blocking must not be modeled
as an unconditional blocking wait.

For a fallible or nonblocking acquisition, held state is updated only on paths
where acquisition succeeds according to canonical semantics.

A nonblocking failure path cannot by itself participate in a permanent wait
cycle.

---

# Scoped guards and resource tokens

When synchronization ownership is represented by a guard or token value, the
analysis may consume canonical ownership, lifetime, move, destruction, and
release facts.

Conceptually:

```text
Guard live and owned by context
    -> resource held by context

Guard released or destroyed
    -> resource no longer held

Guard moved to another owner
    -> ownership follows the canonical move semantics
```

Guard-based analysis is a precision source, not a requirement that all Sec
synchronization APIs use guards.

---

# `defer`, cleanup, and destruction

Deferred release participates in the actual held-resource interval.

For example:

```sec
lock.Lock()
defer lock.Unlock()

Call()
```

means the resource remains held across `Call()` on every path where the deferred
release has not yet executed.

Canonical cleanup, destruction, error, early-return, and panic paths must be
respected.

A dependency edge from proven unreachable code must not participate in the
analysis.

---

# Conditional release

If one control-flow path releases a resource and another does not, the resource
may become only `PossiblyHeld` after the join.

For example:

```sec
lock.Lock()

if condition {
    lock.Unlock()
}

other.Lock()
```

may establish a possible, but not definite, order relationship from `lock` to
`other`.

This distinction contributes to deadlock proof strength.

---

# Path compatibility

Cycle edges must correspond to paths that can participate in a common reachable
execution.

Two opposite lock orders occurring only on mutually exclusive paths in one
execution instance do not automatically form a proven deadlock.

However, the same source-level mutually exclusive branches may be selected by
different concurrent runtime instances.

Path compatibility is therefore evaluated together with execution-context
identity and instance multiplicity.

---

# Execution contexts

Deadlock analysis reuses the canonical execution-context model used by other
concurrency analyses.

Conceptual context classes may include:

```text
Thread
Task
ISR
ForeignCallback
PlatformExecution
Unknown
```

The analysis consumes canonical relations such as:

```text
MayOverlap
CannotOverlap
MayPreempt
MayReenter
CompletionDependsOn
```

It must not construct an incompatible parallel model of thread or task
semantics.

---

# Multiple runtime instances

A finite execution-context abstraction may represent multiple runtime
instances created from one semantic creation site.

The analysis must preserve whether:

```text
ContextClass(site X)
MayHaveConcurrentInstances = true
```

or its semantic equivalent.

Two runtime instances of the same function or task body may participate in one
deadlock when canonical execution semantics permit them to overlap.

---

# Synchronous calls

An ordinary synchronous function call normally remains in the caller execution
context.

It does not create an independent wait dependency merely because the caller
cannot continue until the callee returns.

Instead, callee acquisitions, releases, waits, and resource state are propagated
into the caller context through interprocedural summaries.

---

# Task and thread completion

Task and thread joins or equivalent completion waits are first-class deadlock
dependencies.

For example:

```text
Task A waits for Task B
Task B waits for Task A
```

may form a deadlock without a mutex.

Mixed cycles are also relevant:

```text
A holds L
A waits for B
B waits for L
```

The analysis consumes canonical join, completion, scheduling, and execution
semantics.

---

# Await and blocking operations

A blocking or suspending operation is not automatically a deadlock.

An effect such as:

```text
MayBlock
MaySuspend
```

may help identify candidate wait sites, but deadlock analysis requires the
actual dependency behind the wait where that dependency is known.

Effect analysis is an input and filtering mechanism; it does not replace
progress-dependency analysis.

---

# Progress providers

For condition, event, completion, and similar waits, the analysis should retain
which execution context or resource can provide the required progress when that
information is canonical and available.

For example:

```text
Task A waits for Event E
Task B can provide E
Task B waits for Lock L
Task A holds L
```

may form a mixed cyclic dependency.

If the progress provider cannot be identified, the result remains localized as
an unknown progress dependency rather than being connected to arbitrary
resources.

---

# Condition waits

Condition-like synchronization must be interpreted according to its canonical
contract.

If a condition wait atomically:

```text
releases a mutex
waits
reacquires the mutex
```

held-resource analysis must model that behavior rather than assuming the mutex
remains held throughout the wait.

This requirement is essential for avoiding false deadlock reports.

---

# Resource capacity

Not every synchronization resource is an exclusive mutex.

Canonical resource semantics may define:

```text
capacity
acquisition count
ownership model
release behavior
progress provider
reentrancy
blocking behavior
```

A counting semaphore with capacity greater than one must not be silently
modeled as a capacity-one mutex.

Sec 0.1 does not require arbitrary resource-allocation or Petri-net analysis,
but loss of capacity precision must weaken the result to
`PotentialDeadlock` or `Unknown` as appropriate rather than fabricate a proven
cycle.

---

# Reentrant resources

Resource reentrancy is interpreted according to canonical synchronization
semantics.

A resource that permits recursive acquisition by its current owning context does
not produce self-deadlock solely because the same resource is acquired again.

Reentrancy for one owner does not imply that the resource is nonexclusive
between different execution contexts.

---

# Guaranteed progress and escape

A deadlock proof is broken only by a canonical guarantee sufficient to ensure
that a participant can make the progress required to break the cycle.

Examples may include, where canonical semantics guarantee them:

```text
guaranteed bounded timeout
guaranteed cancellation
guaranteed external completion
```

A mechanism that merely may occur is not a positive deadlock-freedom proof.

Conceptually:

```text
may escape
```

is weaker than:

```text
must escape
```

Only the latter can invalidate an otherwise complete permanent deadlock
witness.

---

# Lock-order consistency and partial orders

Deadlock analysis may prove that a set of nested acquisitions follows a
consistent partial order.

For example:

```text
DatabaseLock < CacheLock < LogLock
```

may support a positive no-lock-order-cycle proof for the analyzed scope.

Sec does not require a single global total order over every synchronization
resource.

Resources that are never nested together need not have a relative order.

---

# Dynamic resource collections

Resource identities may arise from indexed or ranged collections.

For example:

```sec
locks[i].Lock()
locks[j].Lock()
```

uses canonical index, range, Place, alias, and ordering facts.

If `i == j` is proven and the resource is non-reentrant, self-deadlock may be
possible.

If `i` and `j` are proven disjoint, the self-alias candidate is eliminated.

If their relation is unknown, proof strength is weakened rather than assuming
identity or disjointness.

A proven strict acquisition ordering over dynamic resource identities may be
consumed as evidence against order cycles.

---

# Ownership and resource transfer

Deadlock analysis consumes canonical ownership and move semantics for resource
guards, synchronization tokens, and other values that represent held-resource
authority.

If ownership of a guard moves to another execution context or value, the old
owner must not continue to be treated as holding the resource unless canonical
semantics say otherwise.

The analysis does not redefine ownership transfer.

---

# ISR and preemption

ISR execution can participate in deadlock analysis when canonical platform
semantics allow preemption or nesting.

A high-value deadlock pattern is:

```text
Thread holds L
ISR preempts Thread
ISR waits for L
```

If the thread cannot resume to release `L` until the ISR returns, and the ISR
cannot return until acquiring `L`, the cycle may form a `ProvenDeadlock`.

Canonical interrupt masking, priority, or exclusion facts may eliminate the
cycle when they guarantee the ISR cannot execute during the relevant held
interval.

ISR-specific legality and policy remain owned by ISR analysis.

---

# Foreign code and FFI

Foreign code may participate in deadlock analysis through canonical FFI
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

No dependency is inferred from foreign function or parameter names.

A foreign call whose contract says only:

```text
MayBlock
```

but does not identify its dependency remains a localized unknown wait.

It must not be connected speculatively to every resource in the program.

---

# Foreign callback reentry

A foreign call can create self-deadlock through synchronous reentry.

Conceptually:

```text
Sec holds L
    -> calls Foreign
        -> Foreign synchronously invokes callback
            -> callback waits for L
```

If the callback target, reentry relation, resource identity, and non-reentrant
blocking semantics are all established, this may produce a complete
self-deadlock witness.

---

# Foreign completion cycles

Foreign completion dependencies may participate in mixed cycles.

For example:

```text
SecContext holds L
SecContext waits for ForeignCompletion
ForeignCompletion depends on SecCallback
SecCallback waits for L
```

may form a cycle when supported by canonical FFI contracts.

---

# Function summaries

Deadlock analysis exposes composable function summaries.

Conceptually:

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

The exact internal representation is implementation-defined.

The summary must preserve enough symbolic information for callers to instantiate
resource and wait dependencies without requiring the callee body.

---

# Symbolic resource roots

Summaries may refer to symbolic roots such as:

```text
Parameter(n)
Receiver
Static(symbol)
Captured(origin)
Foreign(origin)
AllocationSite(site)
Unknown
```

Projection and Place precision must be retained where available.

---

# Symbolic acquisition order

For example:

```sec
fn AcquireBoth(first: ref Mutex, second: ref Mutex) {
    first.Lock()
    second.Lock()
}
```

may export the semantic relationship:

```text
Parameter(0) -> Parameter(1)
```

or an equivalent structured summary.

At a call site, alias analysis may instantiate both parameters to the same
resource and thereby expose a self-deadlock candidate.

---

# Caller-held propagation

A callee may acquire a resource that was not known to be held inside the callee
summary itself.

For example:

```sec
external.Lock()
CallLibrary()
```

if `CallLibrary` may acquire `Static(CacheLock)`, the caller must be able to
establish:

```text
external -> Static(CacheLock)
```

when `external` remains held across the call.

Therefore summaries must preserve acquisition behavior separately from only
fully internal lock-order edges.

---

# Callee release of inherited resources

If canonical ownership or guard semantics allow a callee to release or transfer
a caller-held resource, inherited held state must be updated before later
acquisitions are correlated.

The analysis must not assume all caller-held resources remain held through every
call when canonical semantics prove otherwise.

---

# Call-site summary instantiation

Symbolic resource, wait, completion, callback, and execution facts are
instantiated at call sites using canonical Place/provenance and identity
relations.

Instantiation must preserve:

```text
Same
Disjoint
MayAlias
Unknown
```

or equivalent precision rather than eagerly collapsing resources to unknown.

---

# Recursive call graphs

Interprocedural deadlock summaries are solved over canonical call-graph SCCs.

Ordinary recursion is not itself concurrency or deadlock.

However recursion may expose deadlock-relevant resource behavior.

For example, a recursive traversal that holds `node.Lock` while recursing to a
`Next` node may self-deadlock if canonical alias analysis proves the next node
can be the same resource and the lock is non-reentrant.

Summary fixed points must be deterministic and declaration-order independent.

---

# Call-graph SCCs versus deadlock SCCs

Call-graph recursion SCCs and deadlock dependency SCCs are distinct.

```text
CallGraph SCC:
    recursive or mutually recursive function/callable structure

WaitForGraph SCC:
    cyclic progress dependency
```

The first is used to solve interprocedural summaries.

The second is used to detect deadlock candidates.

They must not be conflated.

---

# Callable and closure integration

Closed callable target sets are consumed from canonical closure/callable-flow
analysis.

If a callback may target multiple functions, deadlock-relevant summaries from
all permitted targets participate conservatively.

A callback invoked while holding `L` can establish:

```text
L -> M
```

when a possible callback target may acquire `M`.

Open callable contracts may expose relevant behavior such as:

```text
MayBlock
MayAcquire(...)
MayWaitForCompletion
MayReenter
```

Missing dimensions remain localized as Unknown.

---

# Finite resource abstraction

The analysis must not require one static graph node per runtime resource
instance.

Finite symbolic identities may include:

```text
ResourcePlace
ParameterOrigin
StaticResource
AllocationSite
AnyIndex(resource collection)
```

or equivalent abstractions.

Widening should preserve the most specific known root/projection rather than
collapsing immediately to all resources.

---

# Edge aggregation

Equivalent abstract dependency edges may be aggregated.

For example, many source instances corresponding to:

```text
L1 -> L2
```

need not create an unbounded number of graph edges.

Aggregated edges retain deterministic representative causes and may retain
occurrence information for detailed analysis views.

---

# Dependency-edge evidence

A dependency edge retains enough evidence to explain its origin.

Conceptually:

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

The exact representation is implementation-defined.

Evidence must be sufficient to construct stable cycle diagnostics and to avoid
strengthening a cycle beyond the strength of its constituent facts.

---

# Proof strength

Individual dependency edges may differ in proof strength.

An implementation may use categories equivalent to:

```text
ProvenPossible
ConservativePossible
Unknown
```

The exact enum names are not normative.

A cycle based on precise canonical relationships can support stronger
classification than a cycle that depends on unresolved aliasing, path
conditions, resource capacity, or foreign behavior.

---

# Cycle realizability

`ProvenDeadlock` requires more than graph cyclicity.

The analysis must establish enough of the following to support a complete
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

A missing fact weakens classification rather than being silently assumed.

---

# Concurrent realizability

For multi-context cycles, participating execution contexts must be able to
coexist according to canonical execution semantics.

If two contexts are proven unable to overlap, a lock-order inversion between
them cannot form a simultaneous multi-context deadlock.

This does not eliminate self-deadlock within one context.

---

# Reentry realizability

Reentrant callback or platform execution may create deadlock without ordinary
thread overlap.

The analysis must use canonical `MayReenter` or equivalent relationships rather
than requiring two independent threads for every cycle.

---

# Resource identity and proof classification

Possible resource aliasing can support a meaningful `PotentialDeadlock` when a
concrete dependency structure exists, but possible aliasing alone must not
fabricate `ProvenDeadlock`.

A proven deadlock witness requires resource identity facts strong enough to
support every cycle edge that depends on aliasing.

---

# Blocking semantics and cycle proof

Only dependencies whose canonical semantics can block contribute blocking wait
edges.

For example:

```text
BlockingAcquire
CompletionWait
ConditionWait
```

may participate when their contracts establish the relevant dependency.

A nonblocking acquisition that simply returns failure is not treated as a
permanent wait edge.

---

# Resource capacity and proof classification

Resource capacity may determine whether all participants can be blocked
simultaneously.

When capacity information is insufficient, the result is weakened to
`PotentialDeadlock` or `Unknown` according to the amount of dependency structure
that remains known.

Insufficient capacity analysis must never be replaced by an assumption that the
resource behaves as an exclusive mutex.

---

# Precision widening

Deadlock analysis may use bounded abstractions to avoid state or path explosion.

Widening may reduce precision through states such as:

```text
precise resource identity
    -> bounded resource class
    -> localized unknown resource dimension
```

or:

```text
precise path relationship
    -> summarized compatible path class
    -> unknown path compatibility
```

Loss of precision may weaken:

```text
ProvenDeadlock -> PotentialDeadlock -> Unknown
```

but must never strengthen an unsupported result.

---

# Deep precision can strengthen or eliminate a potential finding

Increasing precision is not the same as widening.

Deep analysis may legitimately refine:

```text
PotentialDeadlock -> ProvenDeadlock
```

when it establishes the missing witness.

It may also refine:

```text
PotentialDeadlock -> no finding
```

when it proves path incompatibility, resource disjointness, guaranteed progress,
or non-overlapping execution.

The underlying deadlock semantics do not change.

---

# Avoiding path explosion

The normative analysis does not require exhaustive enumeration of every runtime
path.

Sound implementations may use:

```text
path summaries
symbolic conditions
SCC condensation
bounded context sensitivity
resource abstractions
worklist fixed points
widening
```

provided unsupported precision is represented as `PotentialDeadlock` or
`Unknown` rather than as a fabricated proof.

---

# Representative cycle witness

A deadlock finding retains a representative cycle.

The selected witness should favor, in order of semantic usefulness:

```text
stronger proof
smaller understandable dependency cycle
precise resource and context identity
clear source locations
less uncertainty
```

The implementation may refine this ordering, but selection must be
deterministic.

A graph-theoretically shortest cycle is not required if another stable cycle
provides materially clearer source evidence.

---

# Interprocedural cause paths

When an edge is established through nested calls, the finding should retain the
interprocedural cause path.

For example:

```text
Handler
  -> Update
    -> Cache.Store
      -> acquire CacheLock
```

may explain the edge:

```text
DatabaseLock -> CacheLock
```

Normal diagnostics may present the compact edge while detailed tooling exposes
the full cause path.

---

# Diagnostics

Deadlock diagnostics preserve the distinction between proven, potential, and
unknown results.

## Proven deadlock

A proven finding states that the deadlock can occur.

Conceptually:

```text
error: deadlock can occur

Thread A:
    holds `database.Lock`
    waits for `cache.Lock`

Thread B:
    holds `cache.Lock`
    waits for `database.Lock`
```

Exact wording and severity are governed by the canonical diagnostic rules.

## Potential deadlock

A potential finding states that a meaningful dependency cycle exists but that
the complete blocked-state witness is not proven.

Conceptually:

```text
warning: potential deadlock

one reachable path acquires:
    `database.Lock` -> `cache.Lock`

another concurrent path acquires:
    `cache.Lock` -> `database.Lock`

simultaneous blocked-state reachability could not be fully proven
```

## Unknown

Unknown is not presented as a potential deadlock unless a meaningful dependency
cycle has been established.

Detailed analysis may instead report:

```text
deadlock status unknown

reason:
    foreign call may block, but its wait dependency is unspecified
```

---

# Failure-to-prove diagnostics

When an active policy requires positive deadlock-freedom proof and the analysis
cannot establish it, the diagnostic states that the proof is unavailable.

It must not claim either a proven or potential deadlock unless the corresponding
classification is independently established.

---

# Diagnostic navigation

Deadlock findings are relational and may span several locations.

LSP diagnostics should expose a primary location and related locations for the
resource acquisitions, waits, completion dependencies, callbacks, and other
cycle edges.

The user should be able to navigate through the representative cycle.

---

# Diagnostic presentation

Normal diagnostics present a minimal understandable witness rather than dumping
the complete `WaitForGraph`.

Detailed tooling may expose:

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

# Automatic fixes

Deadlock analysis must not offer a generic transformation such as:

```text
reverse these locks
```

unless a canonical semantic contract proves that the transformation preserves
program behavior and satisfies the intended synchronization order.

Lock ordering is often an architectural choice.

Baseline tooling prioritizes:

```text
explanation
navigation
representative cycle
resource order
cause paths
```

over automatic rewriting.

---

# Positive deadlock-free proofs

The analysis may retain scoped positive facts such as:

```text
NoLockOrderCycle
NoDeadlockCycle
```

when proven for a particular scope, resource set, context relation, and set of
assumptions.

A positive proof is scoped; it must not be generalized to an entire program
when only a subset of resources or execution contexts was analyzed.

Such proofs are primarily reusable analysis facts and OptionalInsight tooling
information.

---

# Analysis budgets

Deadlock analysis uses the common Sec analysis-budget model:

```text
Interactive
Standard
Deep
```

No additional global `Intermediate` analysis mode is introduced.

---

# Interactive analysis

Interactive analysis prioritizes fast sound results from local facts and cached
summaries.

High-value Interactive checks include:

```text
direct self-lock
direct lock-order inversion
cached interprocedural inversion
join while holding a known required resource
known callback reentry
known ISR/preemption self-deadlock
```

Interactive analysis may remain incomplete while refinement is pending.

---

# Progressive LSP refinement

LSP execution is progressively refined within the existing analysis modes.

Conceptually:

```text
Interactive fast result
    -> incremental interprocedural refinement
    -> Standard-equivalent result
    -> optional Deep refinement
```

The intermediate refinement step is scheduling and precision work within the
existing model, not a fourth language-analysis mode.

---

# Pending tooling state

LSP analysis may use a first-class `Pending` state while invalidated summaries,
resource identities, callback targets, or cycle proofs are being recomputed.

`Pending` is distinct from:

```text
ProvenDeadlock
PotentialDeadlock
Unknown
```

A stale proven or potential finding must not continue to be treated as a current
result after one of its proof dependencies has been invalidated.

---

# Standard analysis

Standard analysis is the stable analysis required for normal compilation.

Any deadlock proof required by active language, target, runtime, ISR, project,
or CompilationPlan semantics is mandatory in Standard analysis.

Required proof cannot be deferred to Deep merely because it is expensive.

---

# Deep analysis

Deep analysis may increase precision through more expensive reasoning such as:

```text
path-sensitive held-resource state
context-sensitive resource identities
larger WaitForGraph correlation
dynamic collection ordering
foreign callback/reentry refinement
resource-capacity refinement
richer cycle realizability proofs
```

Deep does not change deadlock semantics.

---

# LSP presentation classes

Deadlock analysis uses the common LSP distinction between information the
programmer needs to see and information that is optional insight.

## NeedToKnow

Normally includes:

```text
ProvenDeadlock
high-confidence PotentialDeadlock
required deadlock-freedom proof failure
```

Exact severity remains governed by canonical diagnostic policy.

## OptionalInsight

May include:

```text
positive no-cycle explanations
canonical lock-order explanation
held-resource information
execution-context relationships
full WaitForGraph details
precision-loss explanations when no required proof fails
```

Optional insight must remain configurable.

---

# LSP graph views

A lock-order view is a natural OptionalInsight representation.

For example:

```text
DatabaseLock
    -> CacheLock
    -> LogLock
```

A detailed Deep view may expose mixed dependencies such as:

```text
ThreadA -> TaskB -> LockC -> ThreadD -> EventE -> ThreadA
```

These views are tooling presentations, not language syntax.

---

# LSP configuration reload

Changes to project analysis depth or optional insight settings must be observed
without requiring an LSP restart.

Affected summaries, cycle classifications, diagnostics, hovers, and analysis
views are refreshed according to the common LSP analysis model.

---

# Incremental invalidation

Deadlock summaries and findings are invalidated when relevant dependencies
change.

Examples include:

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

Invalidation is dependency-driven rather than project-wide when unrelated facts
can remain valid.

---

# Stable-summary propagation

If a function is reanalyzed and its exported deadlock summary is semantically
equivalent to the previous summary, dependent callers need not be invalidated
further solely because the body changed.

Likewise, when one dependency edge disappears, findings whose representative or
supporting cycles require that edge are invalidated even if other edges remain.

---

# Separate compilation

Separate compilation may use versioned symbolic deadlock summaries instead of
callee bodies.

An exported summary may describe facts such as:

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

Callers instantiate symbolic origins using canonical Place/provenance and
resource identity.

Missing imported information widens only the affected dimensions.

---

# Summary versioning and invalidation

Persisted summaries are invalidated when relevant semantics change, including:

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

The cache key or equivalent compatibility mechanism must be sufficient to avoid
using stale summaries under incompatible semantics.

---

# CompilationPlan dependence

Many source-level lock-order facts are target-independent.

Deadlock results become CompilationPlan-dependent only where behavior actually
differs, including cases such as:

```text
runtime task scheduler semantics
ISR preemption and masking
platform synchronization primitives
foreign implementation contracts
resource-capacity semantics
callback or reentry behavior
```

The analysis should not introduce unnecessary target splitting.

---

# Determinism

For the same source, compiler version, relevant project configuration, and
CompilationPlan, deadlock analysis must produce deterministic semantic results.

This includes stable:

```text
classification
representative cycle
primary diagnostic location
related diagnostic locations
cause path selection
```

Results must not depend on declaration order, worklist order, cache traversal,
or parallel compiler scheduling.

---

# `sec analyse`

Deadlock analysis participates in the default all-analysis behavior of:

```text
sec analyse
```

Tooling may expose an explicit deadlock-only selection, conceptually:

```text
sec analyse deadlocks
```

Exact CLI spelling belongs to the canonical tooling rules.

Deep output may include:

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

# Required baseline capabilities for Sec 0.1

The Sec 0.1 baseline requires at least:

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

Resource-specific condition, event, semaphore, and foreign precision may be
conservative where canonical contracts exist but full proof support is not yet
available. Such limitations must be represented through the result classes
rather than by incorrect mutex assumptions.

---

# Required test philosophy

Tests must cover both positive findings and false-positive resistance.

A deadlock analyzer that reports every unusual synchronization pattern is not
conforming.

The test suite must verify:

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

# Required ProvenDeadlock tests

At minimum:

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

# Required PotentialDeadlock tests

At minimum:

```text
lock-order inversion with unresolved path compatibility
dynamic resource alias may create a cycle
resource-capacity uncertainty prevents complete proof
recursive dynamic lock chain with incomplete identity ordering
concurrent context relation supports the candidate but not the full witness
meaningful foreign/resource cycle with one unresolved compatibility dimension
```

These tests must not be upgraded to `ProvenDeadlock` without the missing proof.

---

# Required Unknown tests

At minimum:

```text
foreign blocking target unknown
open callback blocking behavior unknown
unknown event progress provider
lost synchronization-resource provenance
unknown platform preemption semantics
unknown custom synchronization contract
```

Unknown behavior must not become `PotentialDeadlock` without a concrete
supported dependency cycle.

---

# Required safe/non-deadlock tests

At minimum:

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

False positives in these cases are regressions.

---

# Interprocedural tests

At minimum:

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

# Defer and cleanup tests

At minimum:

```text
deferred unlock keeps a resource held across a call
early return releases a scoped guard according to canonical semantics
error path retains a resource until cleanup
panic cleanup releases where canonical semantics guarantee release
conditional release yields PossiblyHeld rather than DefinitelyHeld
```

---

# Task and thread tests

At minimum:

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

# Callable and closure tests

At minimum:

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

# FFI tests

At minimum:

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

No test may rely on foreign function or parameter names to infer semantics.

---

# ISR tests

At minimum:

```text
ISR reacquires thread-held non-reentrant mutex
masked ISR cannot preempt the critical section
higher-priority ISR nesting
known exclusion between ISR priority classes
unknown nesting semantics
ISR waiting on task/thread completion where the canonical model permits such a wait
```

ISR policy violations remain owned by ISR analysis even when the same facts also
form a deadlock dependency.

---

# Resource-semantics tests

At minimum:

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

The suite must verify that non-mutex resources retain their own canonical
semantics.

---

# Dynamic-resource tests

At minimum:

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

# LSP incremental tests

At minimum:

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

# Diagnostic stability tests

LSP refinement should preserve a stable finding identity when the same semantic
cycle remains present across Interactive, Standard, and Deep refinement.

Tests must verify that additional proof detail does not unnecessarily create a
new unrelated diagnostic identity.

Stable diagnostic IDs are governed by the canonical diagnostic rules.

---

# Performance and scalability tests

Large synthetic tests must cover:

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

The tests verify:

```text
bounded symbolic graph construction
SCC/cycle scalability
edge aggregation
incremental invalidation
deterministic representative cycle selection
```

No normative wall-clock threshold is defined by this rulebook.

---

# False-positive regression corpus

The compiler should maintain a permanent regression corpus for safe patterns
that coarse deadlock analyses commonly misclassify.

At minimum it should include:

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

Every legitimate false positive discovered during implementation or external
review should become a regression test.

---

# Interaction with other analyses

Deadlock analysis consumes canonical facts from analyses and rulebooks including:

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

It owns only cyclic blocking/progress dependency analysis.

It must not absorb or duplicate the normative semantics of those sources.

---

# Completion criteria for Sec 0.1

Deadlock analysis is complete for Sec 0.1 when the compiler can soundly:

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

# Implementation governance

Normative deadlock semantics belong in:

```text
rules/analysis/deadlock_analysis.md
```

Mutable implementation state, code locations, remaining implementation work,
and verification commands belong in:

```text
implementation-status.yaml
```

The canonical ledger integration identifier is:

```text
sema.deadlock-analysis
```

A suitable implementation capability breakdown includes:

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

Capability status must describe observable repository implementation rather than
aspirational behavior.

---

# Normative summary

Deadlock analysis determines whether reachable execution contexts and
synchronization resources can form cyclic blocking or progress dependencies that
prevent required progress.

The analysis is distinct from data-race analysis even though both consume
canonical synchronization, execution-context, ownership, and Place facts.

Deadlock analysis is not limited to mutexes. Locks, task/thread completion,
conditions, events, foreign completion, platform synchronization, and other
canonical blocking resources may participate in mixed dependency cycles.

Synchronization-resource identity is derived from canonical semantic identity
and Place/provenance rather than naming conventions.

Held-resource state is flow-sensitive and distinguishes definite from possible
ownership. Acquisition, release, guard lifetime, move, destruction, defer,
cleanup, and error paths determine the actual held interval.

A lock-order edge records that one resource may be acquired while another
remains held. A lock-order cycle is a deadlock candidate, not automatically a
proven deadlock.

The generalized WaitForGraph represents dependencies between execution
contexts, resources, completion events, and progress providers. The
LockOrderGraph is a useful projection but does not replace the generalized
model.

Task/thread joins, callback reentry, foreign completion, and ISR preemption may
participate in deadlock cycles according to their canonical semantics.

Resource reentrancy, capacity, nonblocking acquisition, condition release and
reacquire behavior, timeout, cancellation, and other progress semantics are
consumed from canonical contracts rather than guessed.

Interprocedural deadlock summaries preserve symbolic resource identities,
acquisitions, releases, waits, held-on-return state, execution effects, and
localized uncertainty. Callers instantiate those summaries using canonical
Place/provenance and resource identity.

Call-graph SCCs are used for interprocedural summary fixed points. Deadlock
cycles are analyzed separately in lock-order or wait-for dependency graphs.

`ProvenDeadlock` requires a sound reachable witness in which all cycle
dependencies can be simultaneously realized and no guaranteed progress
mechanism breaks the cycle.

`PotentialDeadlock` requires a meaningful supported dependency cycle but allows
bounded uncertainty in path, identity, scheduling, capacity, or related
compatibility facts.

`Unknown` is used when dependency information is insufficient to establish such
a meaningful cycle.

Failure to prove a required deadlock-freedom property remains distinct from all
three classifications.

Precision widening may weaken results but never fabricate stronger proof.
Additional Deep precision may strengthen a potential finding to a proven
finding or eliminate it when incompatibility is established.

Deadlock diagnostics provide deterministic representative cycles, participating
resources and contexts, source locations, and interprocedural cause paths.
Normal diagnostics present a minimal understandable witness rather than the
entire dependency graph.

LSP execution uses progressive refinement within Interactive, Standard, and
Deep and introduces no separate Intermediate mode. Pending state is distinct
from current findings, and stale proofs are invalidated when their dependencies
change.

Proven deadlocks and high-confidence potential deadlocks are NeedToKnow
information. Positive no-cycle explanations and detailed graph views are
configurable OptionalInsight.

Separate compilation uses versioned symbolic deadlock summaries. Missing
information widens only affected dimensions.

Results, representative cycles, and diagnostics are deterministic and
independent of compiler scheduling.

Normative behavior belongs in this rulebook. Mutable implementation progress
belongs in `implementation-status.yaml`.
