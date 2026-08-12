# Data Race Analysis

## Status

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the compiler analysis used to prove data races, prove scoped
race freedom, preserve uncertainty when proof is unavailable, summarize memory
and concurrency behavior interprocedurally, integrate synchronization and
happens-before facts, support incremental LSP analysis, and provide deterministic
diagnostics and tooling results.

The concurrency memory model owns the definition of legal concurrent behavior.
This rulebook defines how the compiler reasons about programs under that model.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.

---

# Purpose

Data race analysis answers questions such as:

```text
Can these two accesses refer to overlapping storage?
Can the corresponding execution contexts overlap?
Does at least one access write?
Does the concurrency memory model order or otherwise protect the accesses?
Is a race proven, is race freedom proven, or is the result unknown?
Which source accesses and execution paths explain the result?
Which missing alias, synchronization, callback, FFI, ISR, or callable fact
prevents a stronger proof?
```

Data race analysis is not a replacement for ownership, borrowing, escape,
lifetime, Place/provenance, call-graph, closure, FFI, effect, or concurrency
memory-model analysis.

It consumes canonical facts from those analyses and correlates them.

---

# Normative ownership

The canonical concurrency memory model owns:

```text
happens-before semantics
program-order semantics
publication semantics
mutex semantics
atomic access legality
atomic ordering semantics
join/completion ordering
thread/task synchronization semantics
ISR/preemption synchronization semantics
```

Data race analysis owns:

```text
memory access event construction
symbolic access summaries
execution-context correlation
Place overlap correlation
race candidate generation
race pairing
use of canonical happens-before and exclusion facts
ProvenRace / ProvenRaceFree / Unknown classification
scoped positive race-free proofs
race diagnostic construction
incremental invalidation and LSP integration
```

The analysis must not silently redefine a rule owned by the concurrency memory
model merely to make analysis easier.

---

# Core race condition

A data race requires all of the following semantic conditions:

```text
1. two relevant accesses may target overlapping storage;
2. the accesses conflict according to the concurrency memory model;
3. their execution contexts may overlap in a permitted execution;
4. the ordering or exclusion required by the concurrency memory model is absent.
```

Conceptually:

```text
RaceCandidate(A, B)
    if
        MayOverlap(A.Place, B.Place)
        and ConflictingAccess(A, B)
        and MayExecuteConcurrently(A, B)
        and not OrderedOrExcluded(A, B)
```

This expression is explanatory. The implementation is not required to use this
exact representation.

A syntactic pattern such as "the same variable is used in two threads" is not a
sufficient race model.

---

# Memory access events

The analysis operates on semantic memory access events.

A conceptual event contains at least:

```text
MemoryAccessEvent {
    Place
    AccessKind
    ExecutionContext
    Atomicity
    Source
}
```

The exact internal form is implementation-defined.

## Access kinds

The analysis must distinguish access behavior sufficiently to identify
conflicts. At minimum this includes the semantic equivalent of:

```text
Read
Write
ReadWrite
```

Whether two access kinds conflict is interpreted according to the canonical
concurrency memory model.

Ordinary concurrent reads do not form a data race merely because they occur at
the same time unless the canonical rules for the relevant storage class say
otherwise.

---

# Canonical Place and provenance

Data race analysis uses canonical Place/provenance as the identity of accessed
storage.

For example:

```sec
state.Left.Count += 1
```

must remain structurally distinguishable from:

```sec
state.Right.Count += 1
```

when Place analysis proves the corresponding storage disjoint.

The analysis must preserve available precision for:

```text
fields
constant indices
dynamic indices
slices and ranges
aggregate projections
captured values
static storage
thread-local storage
foreign-derived storage
```

It must not unnecessarily collapse a precise Place to its root aggregate.

---

# Storage overlap

Before two access events can form a race candidate, their Places must be capable
of overlapping.

The implementation must represent at least the semantic distinctions:

```text
Disjoint
Overlap
MayOverlap
```

An implementation may retain additional internal precision.

`Disjoint` eliminates the candidate.

`Overlap` means the relevant storage relationship is established.

`MayOverlap` means the analysis cannot prove disjointness. It is not by itself
proof that the accesses actually target the same storage in a reachable unsafe
execution.

Loss of alias precision must not be silently converted into `ProvenRace`.

---

# Execution contexts

Conflicting accesses are relevant to race analysis only when their execution
contexts may overlap.

A conceptual execution-context model may include:

```text
Thread
Task
ISR
ForeignCallback
PlatformExecution
Unknown
```

The exact representation is implementation-defined.

The analysis must reason from canonical execution semantics rather than from
function identity.

Two calls to different functions may execute sequentially.

Two runtime instances of the same function may execute concurrently.

Ordinary synchronous calls normally inherit the execution context of their
caller and do not create concurrency merely by crossing a function boundary.

---

# Execution-context abstraction

Execution contexts must use a finite static abstraction.

A task or thread creation site that executes repeatedly must not require an
unbounded number of static context nodes.

A suitable abstraction may represent a context class by creation site and retain
whether multiple runtime instances may overlap.

Conceptually:

```text
TaskContext(creation_site)
MayHaveMultipleConcurrentInstances = true
```

This matters because one task body may race with another runtime instance of the
same task body.

Function equality does not imply execution-instance equality.

---

# Concurrency relationships

The analysis must be able to distinguish relationships equivalent to:

```text
MayOverlap
CannotOverlap
UnknownOverlap
```

It may also retain start/completion or other more precise relations.

These relations are semantic and need not correspond to physical timestamps.

A possible legal overlap is enough for race analysis when the other race
conditions can be established. The accesses do not need to overlap on every
possible schedule.

---

# Concurrency intervals

The analysis should retain lifecycle relationships for spawned or otherwise
concurrent execution contexts when canonical semantics provide them.

For example:

```text
spawn Worker
    |
    | possible overlap
    |
join Worker
```

A canonical join/completion relation may prove that the worker no longer
overlaps the continuation after the join.

The analysis should exploit such intervals where available rather than assuming
that a spawned execution remains concurrent forever.

Scheduler coincidence is not a safety proof. Non-overlap may be used only when
it is guaranteed by language, runtime, platform, or explicit contract semantics.

---

# Core result classes

Data race analysis distinguishes at least:

```text
ProvenRace
ProvenRaceFree
Unknown
```

## ProvenRace

`ProvenRace` means the compiler has a sound witness that there exists a reachable
permitted execution in which conflicting accesses can operate on overlapping
storage without the ordering or exclusion required by the canonical concurrency
memory model.

## ProvenRaceFree

`ProvenRaceFree` means the compiler has a positive proof for the stated scope
that the relevant conflict cannot occur.

Possible proof reasons include:

```text
DisjointStorage
ExclusiveOwnership
NoConcurrentExecution
ReadOnly
MutualExclusion
HappensBefore
AtomicRaceSafe
```

The exact internal classification is implementation-defined.

## Unknown

`Unknown` means the analysis cannot prove either a race or race freedom for the
relevant relationship.

Failure to prove a race is not proof of race freedom.

Failure to prove race freedom is not proof of a race.

---

# Required proof versus proven violation

A language, target, runtime, or CompilationPlan contract may require positive
race-freedom proof for a particular operation or execution model.

In such a context:

```text
Unknown
```

may make compilation fail because the required proof is unavailable.

This must remain diagnostically distinct from:

```text
ProvenRace
```

The compiler must distinguish:

```text
proof of a violation
```

from:

```text
failure to prove a required safety property
```

---

# Localized Unknown

Loss of precision in one relationship must not erase unrelated known facts.

If most shared accesses are proven race-free and one foreign callback has
unknown concurrency behavior, the analysis should preserve the proven results
and localize uncertainty to the affected accesses and contexts.

Useful structural unknown reasons include:

```text
UnknownAlias
UnknownConcurrency
UnknownSynchronization
UnknownForeignBehavior
UnknownCallableTarget
UnknownAtomicContract
UnknownPlatformPreemption
PrecisionWidened
```

The exact enum is implementation-defined, but the cause of lost proof precision
must remain representable.

---

# Borrowing as race-freedom evidence

Data race analysis consumes canonical borrowing results.

If the borrow checker establishes that a mutable Place is exclusively accessible
for the relevant interval, competing ordinary access cannot coexist when the
borrowing semantics guarantee that exclusivity.

Conceptually:

```text
ExclusiveBorrow
    -> competing ordinary access cannot coexist
```

Data race analysis does not reimplement borrow checking.

Borrow errors remain owned by borrowing/lifetime/ownership analysis.

---

# Ownership transfer

Ownership transfer can eliminate shared access entirely.

For example:

```sec
spawn Worker(move value)
```

may establish that the old execution context no longer has legal access to the
moved value.

When ownership and escape analysis prove exclusive transfer and no competing
alias remains, the storage is not concurrently shared merely because two
execution contexts exist.

Data race analysis must consume this proof rather than first treating every
spawn argument as shared memory.

---

# Concurrent borrowed storage

If Sec permits a borrow or view to cross an execution-context boundary, its
lifetime and ownership legality must first be established by the owning
analyses.

Data race analysis then reasons about memory access through the valid shared
dependency.

It must not use race analysis to legalize an otherwise invalid lifetime or
ownership transfer.

---

# Publication

Publication and race freedom are distinct properties.

Correct publication may establish ordering between initialization and another
context observing the published object.

It does not automatically make all later concurrent mutation race-free.

Conceptually:

```text
initialize object
publish object
other context observes publication
read object
```

may be ordered according to the canonical memory model.

Later mutation still requires the ordering, exclusion, ownership, or atomic
semantics required by that model.

---

# Publication summaries

Interprocedural analysis must be able to represent publication effects
symbolically.

A conceptual publication fact may include:

```text
PublishedOrigin
DestinationContextClass
OrderingEstablished
```

The exact representation is implementation-defined.

Publication may occur indirectly through retained references, registries,
callbacks, runtime facilities, or foreign code when canonical summaries or
contracts establish that behavior.

---

# Synchronization and happens-before

Data race analysis consumes canonical synchronization semantics.

It must be able to query or derive a semantic relationship equivalent to:

```text
HB(A, B)
```

where the edge semantics come from the concurrency memory model.

Possible canonical sources include:

```text
program order
publication
lock release/acquire
atomic synchronization
thread/task start
join/completion
platform or ISR ordering
foreign synchronization contract
```

This list does not redefine those operations. It identifies the kinds of
canonical facts that may be consumed.

---

# Happens-before transitivity

Happens-before is transitive.

If:

```text
A happens-before B
B happens-before C
```

then the analysis may use:

```text
A happens-before C
```

The implementation is not required to materialize a complete global transitive
closure. Graph reachability, summaries, incremental indexes, or equivalent
sound mechanisms may be used.

---

# Program order

Sequential operations in one execution instance are ordered according to the
canonical program-order semantics.

For example:

```sec
state.Value = 1
let value := state.Value
```

does not create a race merely because both events access the same Place.

Program order in one runtime instance must not be confused with ordering
between separate concurrent runtime instances of the same function body.

---

# Path compatibility

Access pairing must account for control-flow compatibility.

Two accesses in mutually exclusive branches of one execution instance do not
execute concurrently merely because both are present in the CFG.

For example:

```sec
if condition {
    state.Value = 1
} else {
    state.Value = 2
}
```

does not form a race within that execution instance.

However, two concurrent runtime instances may each execute one of the branches.
Path compatibility must therefore be interpreted relative to execution
instances, not only source-level CFG nodes.

---

# Locks and canonical synchronization identity

A lock protects accesses only when canonical synchronization semantics establish
the relevant relationship.

Naming conventions are never sufficient.

The analysis must not infer that a lock named `stateLock` protects `state`, or
that two locks with similar names are identical.

Lock identity is derived from canonical Place/provenance, synchronization object
identity, guard/token semantics, or explicit contract information.

---

# Must-held lock state

A positive lock-based race-freedom proof requires the relevant lock to be
definitely held during the memory access.

It is insufficient that the lock may be held on some path.

For example:

```sec
if condition {
    lock.Lock()
}

state.Value = 10
```

must not produce a `MustHold(lock)` proof at the write after the control-flow
join.

At joins, positive must-held facts survive only when they hold on every relevant
incoming path.

---

# Multiple locks

Two protected accesses need not have identical held-lock sets.

If the canonical synchronization semantics prove mutual exclusion through a
shared lock identity or another valid relationship, that proof may be used.

Conversely:

```text
A holds L1
B holds L2
```

does not by itself prove race freedom for overlapping storage.

Lock ordering belongs primarily to deadlock analysis. Data race analysis needs
only the synchronization/exclusion facts necessary to classify memory access.

---

# Synchronization wrappers

Interprocedural analysis must support wrapper functions that establish
synchronization.

For example:

```sec
fn WithLock(lock: ref Mutex, action: fn()) {
    lock.Lock()
    defer lock.Unlock()
    action()
}
```

must remain analyzable when callable contracts and synchronization summaries are
sufficient.

Programmers must not be forced to place lock acquisition syntactically in the
same function as every protected memory access merely to obtain race analysis.

---

# Locks held across function boundaries

If synchronization APIs permit a function to return while a lock or guard is
still semantically held, summaries must represent this through canonical state,
ownership, or guard-token facts.

The analysis must not assume that every function releases every lock before
return unless the language or API semantics guarantee that property.

---

# Atomic accesses

Atomic access legality and ordering are interpreted exclusively according to the
canonical concurrency memory model.

Data race analysis must distinguish:

```text
race legality
```

from:

```text
strong ordering or visibility guarantees
```

Two accesses may be race-legal under atomic semantics without providing the
stronger ordering needed for an unrelated correctness property.

The latter is outside the scope of this rulebook unless another analysis
consumes the ordering facts.

---

# Atomic and ordinary access

The analysis must not classify a mixed atomic/ordinary access pair from general
intuition.

The canonical concurrency memory model determines whether and under what
conditions the combination is legal.

Data race analysis consumes that rule.

---

# Raw pointers and unsafe code

Raw-pointer use may reduce alias precision but must not make the analysis
intentionally blind.

Where canonical provenance is known, it should be retained.

For example, a raw pointer known to derive from `state.Buffer` should remain
associated with that origin as far as the provenance model permits.

When provenance is genuinely lost, the affected alias relationship becomes
`Unknown` or `MayOverlap` as appropriate.

Loss of provenance must not be interpreted as proof of safety.

---

# FFI boundary

FFI is an explicit contract boundary for data race analysis.

Foreign behavior may include:

```text
reading supplied storage
writing supplied storage
retaining storage
publishing storage
accessing storage from a foreign thread
calling back synchronously
calling back asynchronously
establishing synchronization or completion ordering
```

Such behavior is consumed only from canonical FFI contracts, trusted imported
metadata, generated bindings, or compiler-known platform/library contracts.

Data race analysis must not infer concurrency behavior from foreign function or
parameter names.

The import mechanism itself is outside this rulebook.

---

# Foreign callbacks

A foreign callback may introduce a new execution context.

If an FFI contract states that a callback may be invoked from foreign worker
threads, the corresponding execution relationship participates in race
analysis.

If the contract establishes same-thread synchronous callback behavior, the
analysis may use that fact.

If callback execution context is unspecified, only the affected concurrency
dimension becomes `Unknown`.

---

# Foreign ordering

Sufficiently precise foreign contracts may define relationships such as:

```text
foreign worker begins after registration
foreign worker may access buffer
foreign worker completes before JoinForeign returns
```

When canonical, these facts may be treated similarly to Sec task/thread
ordering.

Missing foreign information is not replaced by guessed behavior.

---

# ISR interaction

ISR and ordinary execution can access the same storage.

Data race analysis must be able to consume canonical platform facts such as:

```text
ISR may preempt context
interrupts disabled for a critical interval
ISR priority relation
ISR nesting permitted
ISR nesting forbidden
```

The platform or CompilationPlan owns these semantics.

Data race analysis uses them to determine whether conflicting memory accesses
may overlap and whether canonical exclusion exists.

`isr_analysis.md` may later impose additional ISR restrictions; it does not need
a separate race engine.

---

# Asymmetric preemption

ISR concurrency need not be modeled as symmetric scheduling.

If an ISR may preempt a thread, conflicting accesses can be concurrently
relevant even though the thread cannot preempt the ISR in the reverse
direction.

The race analysis requires a sound `ConcurrentConflictPossible` relationship,
not a symmetric scheduler model.

---

# Data-race and deadlock analysis

Data race and deadlock analysis are sibling analyses.

They may consume shared canonical synchronization identities and lock-state
facts, but their responsibilities differ:

```text
DataRaceAnalysis:
    is shared memory access sufficiently ordered or excluded?

DeadlockAnalysis:
    can synchronization dependencies prevent progress?
```

A lock may simultaneously remove a data race and participate in a deadlock
cycle.

Data race analysis must not duplicate the deadlock lock-order graph.

---

# Function summaries

Each analyzable function must be capable of producing a composable summary of
its race-relevant behavior.

Conceptually:

```text
DataRaceSummary {
    MemoryEffects
    SynchronizationEffects
    ExecutionEffects
    PublicationEffects
    Precision
}
```

The exact data structure is implementation-defined.

Summaries exist so callers and separate compilation can reason without
re-analyzing every function body in every context.

---

# Symbolic memory roots

Function summaries preserve access roots symbolically.

Relevant roots include the semantic equivalent of:

```text
Parameter(n)
Receiver
Static(symbol)
ThreadLocal(symbol)
Captured(origin)
Foreign(origin)
Unknown
```

For example:

```sec
fn Update(state: ref mut State) {
    state.Counter += 1
}
```

should retain an effect equivalent to:

```text
Parameter(0).Counter:
    MayRead
    MayWrite
```

rather than collapsing to `UnknownMemory`.

---

# Symbolic projections

Summary projections must retain the precision supported by canonical Place
analysis.

For example:

```text
Parameter(0).Left.Count
```

must remain distinguishable from:

```text
Parameter(0).Right.Count
```

when the language and Place semantics prove those fields disjoint.

Likewise, index and slice abstractions should retain disjoint range information
where proven.

---

# May-access summaries

Interprocedural memory effects are primarily may-effects.

At minimum the summary distinguishes effects equivalent to:

```text
MayRead(Place)
MayWrite(Place)
```

A may-write fact means a reachable path may perform the write. It does not mean
that every invocation writes.

At control-flow joins, may-effects are conservatively joined by union.

---

# Must-state summaries

Positive protection facts are must-properties.

For example, an access protected by a mutex requires proof that the relevant
mutex is held on every path reaching that access under the summarized
condition.

At control-flow joins, must-properties survive only when valid for all relevant
incoming paths.

The analysis must keep may- and must-domains conceptually separate.

---

# Call-site summary instantiation

A caller instantiates callee symbolic roots using actual call-site provenance.

For example:

```text
Callee summary:
    Parameter(0).Value -> MayWrite

Call site:
    Set(sharedState)

Instantiated effect:
    sharedState.Value -> MayWrite
```

This same principle applies to synchronization identities, publication effects,
captured origins, and other symbolic relationships.

---

# Synchronous calls

An ordinary synchronous call executes in the caller's execution context unless
canonical language, runtime, or foreign semantics state otherwise.

Function boundaries alone do not introduce concurrency.

---

# Spawn and thread creation

Spawn, explicit thread creation, runtime task creation, foreign asynchronous
callbacks, ISR entry, and similar operations establish new execution contexts
according to their canonical semantics.

The summary model must retain the execution effects necessary to determine
which contexts may overlap and what storage is shared between them.

---

# Self-concurrent creation sites

A single creation site may create multiple overlapping runtime instances.

For example:

```sec
for item in items {
    spawn Process(item)
}
```

must allow the analysis to compare one `Process` instance with another when the
runtime semantics permit overlap.

This is required even though both instances share the same semantic body.

---

# Join and completion

Canonical join/completion semantics may create happens-before relations.

For example:

```text
worker writes X
worker completes
main joins worker
main reads X
```

may be proven ordered when the concurrency memory model defines the appropriate
completion/join relation.

The data race analysis consumes this relation; it does not define it.

---

# Callable-value calls

Callable-flow and closure analysis provide possible target information for
function-value calls.

For a closed target set, data-race summaries are conservatively joined over all
possible targets.

Conceptually:

```text
Summary(call) = join(summary(target) for each possible target)
```

Known target identities may also be used to recover more precise source cause
paths.

---

# Open callable contracts

An open callable target set requires a callable contract sufficient to describe
the race-relevant behavior permitted by unknown targets.

Relevant dimensions may include the semantic equivalent of:

```text
MayRead
MayWrite
MayRetain
MayPublish
MaySpawn
ConcurrencyBehavior
SynchronizationBehavior
```

This rulebook does not define source syntax for those contracts.

Missing information widens only the affected dimensions.

An unknown callable is not automatically modeled as "writes all memory" when
more structured information is available.

---

# Recursive call graphs

Data-race summaries are solved over the canonical call graph.

Recursive and mutually recursive SCCs use deterministic monotone fixed-point
analysis.

Data race analysis must not construct a parallel call graph.

Ordinary recursion does not itself imply concurrency. Multiple recursive stack
frames in one thread remain sequential execution unless a spawn, callback,
thread, ISR, or other concurrency-producing semantic edge is involved.

---

# Recursive spawning

Recursive creation of concurrent work may create arbitrarily many runtime
instances, but data race analysis does not need an exact runtime instance count
to determine whether two abstract instances may overlap.

A finite context abstraction with a sound self-overlap property is sufficient
for baseline race reasoning.

---

# Index and slice abstractions

Large arrays and dynamic index sets must use bounded Place abstractions.

The analysis must not create a distinct static Place node for every possible
runtime index in a large collection.

Where range analysis proves indices or backing ranges disjoint, the race
analysis should preserve that proof.

Otherwise the relationship may widen to `MayOverlap`.

---

# Parallel disjoint partitions

Sec race analysis must support race-free proofs for parallel access to disjoint
parts of one backing collection.

For example:

```sec
left := values[0..<mid]
right := values[mid..<values.len]

spawn Process(ref mut left)
Process(ref mut right)
```

may be proven race-free when canonical Place/range/borrowing facts establish
that the backing ranges are disjoint and the cross-context transfer is valid.

A shared backing allocation does not imply overlapping storage.

---

# Disjoint aggregate fields

Likewise, different fields of an aggregate may be accessed concurrently without
synchronization when canonical layout/Place/borrowing semantics prove the
storage disjoint and the execution-context transfer is valid.

The analysis must not unnecessarily coarse-grain every field access to the
aggregate root.

---

# Union payloads

Different union payload projections are not automatically disjoint.

Union payload storage may overlap physically.

Data race analysis must consume canonical Place/layout/active-variant facts and
must not infer field-like disjointness merely from different payload names.

---

# MMIO and volatile storage

Memory-mapped I/O, volatile access, and target-defined memory spaces may have
special synchronization and access semantics.

Data race analysis must consume the canonical memory-space and target rules.

It must not blindly apply ordinary mutex/atomic assumptions to MMIO.

---

# Race candidate generation

The analysis must not require naïve comparison of every memory event with every
other memory event in a large program.

Candidate generation should first exploit canonical facts such as:

```text
Place root or alias class
projection or range class
access kind
execution-context class
synchronization class
```

Accesses that are provably disjoint, non-conflicting, or unable to overlap in
execution should be eliminated before detailed race pairing.

The exact index data structures are implementation-defined.

---

# Summary-level candidate pairing

Whole-program analysis may compare symbolic summaries before expanding to
individual event pairs.

For example:

```text
Context A may write Parameter(0).Data
Context B may read Parameter(0).Data
```

can first establish a potentially relevant relationship.

Detailed source events need be recovered only when required for proof,
precision, diagnostics, or cause reporting.

This permits scalable analysis without changing race semantics.

---

# Proven race witness

`ProvenRace` requires a sound reachable witness.

The compiler must establish enough canonical facts to show that a legal
execution exists where:

```text
A and B conflict;
A and B may operate on overlapping storage;
A and B may overlap in execution;
no required ordering or exclusion prevents the conflict.
```

The witness need not be a concrete fully scheduled runtime trace, but it must be
stronger than mere loss of precision.

---

# May-overlap and proof strength

`MayOverlap` storage together with `MayExecuteConcurrently` is not automatically
`ProvenRace` if the alias/concurrency facts are too weak to establish an unsafe
reachable execution under the active rules.

When proof strength is insufficient, the result remains `Unknown` unless the
program is rejected because an explicit positive race-freedom proof is required.

This is particularly important around raw pointers, open callable contracts,
FFI, and platform-specific preemption.

---

# Widening

Analysis state must remain finite.

If Place origins, target sets, execution contexts, paths, or other dimensions
become too large, the implementation may widen:

```text
precise finite set
    -> structured abstraction
    -> Unknown for the affected dimension
```

Widening should preserve known structure where practical.

For example:

```text
Place = Parameter(0).AnyIndex
ExecutionContext = known worker class
MustHold = known lock L
```

is more useful than global `Unknown`.

Widening must never fabricate a `ProvenRace` or `ProvenRaceFree` result.

---

# Positive race-free proofs

Positive proofs are first-class compiler facts.

A conceptual proof may contain:

```text
RaceFreeProof {
    Place
    ContextRelation
    Reason
    Assumptions
}
```

A proof is scoped.

For example, proving `state.Left` race-free between two worker contexts does not
prove every field of `state` or every execution context race-free.

Consumers must preserve the proof scope and assumptions.

---

# Cause paths

A proven race should retain representative source access paths.

For example:

```text
write:
    WorkerA -> Update -> state.Count

read:
    WorkerB -> Snapshot -> state.Count

relationship:
    execution may overlap
    accesses target overlapping storage
    no required happens-before ordering was established
```

Cause paths should use canonical call/execution summaries and be deterministic.

---

# Diagnostic grouping

A single abstract race may correspond to many runtime instances or repeated loop
iterations.

The compiler should group equivalent race occurrences so one logical problem
does not produce an unbounded or overwhelming number of diagnostics.

Grouping may use a semantic key equivalent to:

```text
Place class
context pair
root cause
representative access pair
```

Distinct root causes must remain distinguishable.

A race on `state.Counter` and an unrelated race on `state.Buffer` should not be
merged merely because they occur in the same task pair.

---

# Diagnostics

A proven race diagnostic must be distinguishable from failure to prove required
race freedom.

## Proven race

A diagnostic may conceptually report:

```text
error: data race on `state.Count`

write:
    WorkerA -> Update -> state.Count

read:
    WorkerB -> Snapshot -> state.Count

these execution contexts may overlap and no required ordering protects the
conflicting accesses
```

## Failure to prove race freedom

A different diagnostic may report:

```text
error: race freedom could not be proven for `buffer`

reason:
    foreign callback concurrency is unknown
```

The latter must not claim that a race was proven.

---

# Diagnostic related locations

Race diagnostics are relational and should expose both conflicting source
locations when available.

LSP diagnostics should provide a primary source location and related location so
a programmer can navigate between the accesses.

Interprocedural call or execution cause paths may additionally be shown.

---

# Diagnostic explanation

Where available, a diagnostic should explain:

```text
why the storage overlaps;
why the execution contexts may overlap;
which access writes;
which required ordering or exclusion is absent;
which foreign, callable, alias, platform, or synchronization fact is unknown.
```

For example:

```text
both accesses refer to `state.Count`
```

or:

```text
both views may overlap the same backing range
```

or:

```text
no happens-before edge orders the worker write before this read
```

---

# Race diagnostics do not prescribe locking

The compiler must not automatically prescribe a mutex as the universal fix for
a race.

A mutex may be the wrong synchronization strategy, may be forbidden in the
execution context, may harm performance, or may create deadlock.

Diagnostics should explain the missing safety relationship and may suggest that
the programmer establish synchronization, ownership transfer, disjoint storage,
non-overlap, atomic semantics, or another valid design.

Architectural synchronization choices remain explicit programmer decisions.

---

# Automatic fixes

Automatic race fixes are allowed only when the transformation is canonical and
proven safe under the relevant semantics.

General race repair is not a source-text rewrite problem.

The baseline tooling priority is therefore:

```text
clear explanation
navigation between conflicting accesses
cause path
missing-proof explanation
```

rather than aggressive automatic code modification.

---

# Analysis budgets

Data race analysis uses the common Sec analysis-budget model:

```text
Interactive
Standard
Deep
```

There is no separate global `Intermediate` analysis mode.

---

# Interactive analysis

Interactive analysis prioritizes low-latency sound results using available local
facts and cached summaries.

Typical fast inputs include:

```text
obvious shared accesses
known spawn/thread relationships
known exclusive borrows
known static/shared writes
known lock protection
cheap field/index disjointness
cached callee summaries
cached callable targets
```

Interactive analysis may return incomplete tooling state while deeper affected
relationships are being refined.

---

# Progressive LSP refinement

LSP execution is progressively refined within the existing analysis modes.

Conceptually:

```text
Interactive fast result
    -> incremental Interactive refinement
    -> Standard-equivalent result
    -> optional Deep refinement
```

The intermediate steps are scheduling and precision refinements, not a new
language-analysis mode.

The compiler must not introduce a separate `Intermediate` semantic mode merely
to describe background LSP work.

---

# Pending tooling state

`Pending` is distinct from:

```text
Unknown
ProvenRace
ProvenRaceFree
```

`Pending` means analysis required for the current tooling result has not yet
completed.

The LSP must never treat `Pending` as race freedom.

Likewise, a stale `ProvenRaceFree` result must not remain active after a relevant
dependency has been invalidated.

---

# Standard analysis

Standard is the stable analysis level used for normal compilation.

Every data-race proof required for source validity or the active CompilationPlan
must be completed in Standard.

Required safety analysis must not be deferred to Deep.

Where additional precision is necessary to discharge an active required
race-freedom contract, that precision becomes required for the build.

---

# Deep analysis

Deep may improve precision with more expensive reasoning such as:

```text
path-sensitive concurrency intervals
more precise range partitioning
context-sensitive callable refinement
cross-module access correlation
symbolic lock-identity refinement
more precise foreign callback correlation
richer positive race-free explanation
```

Deep may reduce `Unknown` to a stronger proven result.

It does not change the concurrency memory model or source semantics.

A project may configure the LSP to run Deep analysis continuously where project
size and machine resources make that practical.

---

# LSP presentation classes

LSP presentation distinguishes:

```text
NeedToKnow
OptionalInsight
```

## NeedToKnow

The following are NeedToKnow:

```text
ProvenRace
required race-freedom proof failed
```

They are compiler diagnostics rather than optional explanatory decoration.

## OptionalInsight

Examples include:

```text
race-free because ranges are disjoint
race-free because both accesses are protected by lock X
publication establishes the relevant ordering
these execution contexts cannot overlap
detailed shared-Place access summaries
```

These are configurable and should not flood ordinary editor use.

---

# LSP hover and analysis views

Normal hover should not become a full concurrency-analysis dump.

When detailed or Deep presentation is enabled, tooling may expose information
such as:

```text
Access:
    state.Buffer[i]

Context:
    Worker(site 4)

May overlap:
    Worker(site 4)

Race-free:
    index ranges proven disjoint by range analysis
```

Positive proof explanations are OptionalInsight unless needed to explain an
active diagnostic.

---

# LSP configuration reload

Project analysis settings may control race-analysis depth and optional
presentation.

The LSP must:

```text
read relevant project configuration when the workspace opens;
watch the configuration for changes;
reload changed settings;
invalidate affected summaries, proofs, and findings;
recompute affected analysis;
refresh diagnostics and dependent UI state;
avoid requiring an LSP restart.
```

This rulebook does not define the project configuration syntax.

---

# Incremental invalidation

Race analysis is dependency-driven.

Relevant changes may include:

```text
function body
callee memory summary
callable target set
spawn/thread/task relationship
ownership transfer
escape/publication behavior
Place/provenance
range disjointness
lock identity
synchronization semantics
FFI callback contract
ISR/platform preemption facts
CompilationPlan-dependent concurrency semantics
analysis schema/version
```

Only findings, proofs, summaries, and dependent callers/contexts affected by the
change should be invalidated where the dependency graph can determine that
scope.

---

# Stable-summary propagation

If a changed function re-analyzes to an exported data-race summary semantically
equivalent to the previous summary, unnecessary transitive caller invalidation
should be stoppable.

This is an implementation scalability requirement, not a change in semantics.

---

# Separate compilation

Data race analysis must support separate compilation through versioned symbolic
summaries and explicit contracts.

Imported summaries may describe:

```text
parameter and receiver accesses
static accesses
publication and retention
execution-context effects
must-held synchronization
callback concurrency
```

When a body is unavailable, the compatible summary or explicit contract is the
analysis source of truth for the exported behavior.

Missing information widens only the affected dimensions.

---

# Summary versioning and invalidation

Persisted summaries must be invalidated or rejected when their semantic basis is
incompatible.

Relevant dependencies include:

```text
function body
callee summaries
call graph
closure target sets and callable contracts
Place/provenance model
ownership and escape summaries
concurrency memory model
lock and atomic semantics
task/thread runtime model
FFI contracts
ISR/platform preemption model
CompilationPlan where relevant
analysis schema/version
```

The exact serialization format is implementation-defined.

---

# CompilationPlan dependence

Most data-race reasoning is source-semantic rather than inherently target
specific.

CompilationPlan dependence is introduced only when behavior actually differs,
for example through:

```text
available atomic behavior
ISR/preemption rules
task/runtime execution semantics
foreign/runtime contracts
memory-space semantics
```

The compiler should not duplicate target-independent summaries for every plan
without reason.

---

# Determinism

For the same:

```text
source
CompilationPlan
analysis configuration
compiler version
```

data race analysis must produce deterministic classification and deterministic
diagnostic grouping.

Results must not depend on:

```text
source declaration order
hash-map iteration order
worklist scheduling
parallel compiler scheduling
cache fill order
```

Representative access pairs and cause paths must be selected deterministically.

---

# `sec analyse`

The default:

```text
sec analyse
```

includes data race analysis as part of all-analysis behavior.

Tooling must also permit explicit selection of data race analysis without
requiring every unrelated optional analysis.

The exact CLI spelling is owned by CLI/tooling rules.

Deep data-race output may include:

```text
proven races
required race-freedom proofs that failed
scoped positive race-free proofs
Unknown relationships and their causes
execution-context relationships
publication relationships
representative access pairs
synchronization reasons
suppressed candidate pairs
precision-loss causes
```

---

# Required baseline capabilities for Sec 0.1

The required Sec 0.1 implementation supports at least:

```text
canonical access events
symbolic parameter, receiver, captured, static, and thread-local Places
read/write summaries
field/index/view provenance
Place overlap and disjointness
execution contexts
self-overlap for repeated task/spawn creation sites
ownership transfer
publication
join/completion ordering
must-held lock reasoning
happens-before reasoning
direct call summary instantiation
call-graph SCC fixed point
closed callable target sets
open callable contracts
FFI concurrency contracts
ISR/thread concurrency relationships
bounded candidate pairing
ProvenRace / ProvenRaceFree / Unknown
localized Unknown causes
incremental LSP invalidation
```

Advanced context sensitivity is optional deeper precision unless required to
discharge an active safety contract.

---

# Required test philosophy

Race analysis requires both positive detection tests and strong false-positive
regression coverage.

A compiler that detects obvious races but rejects safe partitioned or
synchronized programs is not conforming to this rulebook.

Tests should verify both analysis facts and diagnostic classification.

---

# Required proven-race tests

The test suite must include at least:

```text
concurrent read/write of the same Place
concurrent write/write of the same Place
two overlapping instances from the same spawn site
static shared mutation
shared field mutation
overlapping slice mutation
foreign callback concurrent access when the contract proves concurrency
ISR/thread conflicting access when platform semantics permit preemption
```

---

# Required race-free tests

The test suite must include at least:

```text
read/read concurrent access
sequential access
worker write followed by join then read
exclusive ownership transfer
the same canonical exclusive lock definitely held for both accesses
disjoint aggregate fields
disjoint constant indices
proven-disjoint dynamic index ranges
proven-disjoint slice ranges
atomic access defined as race-safe by the memory model
interrupt masking that canonically guarantees exclusion
```

---

# Required Unknown tests

The test suite must include at least:

```text
lost raw-pointer provenance
unknown callable target behavior
foreign retention with unspecified concurrency
unknown ISR preemption relationship
unknown alias between shared views
precision widening
```

These cases must not be promoted to `ProvenRace` merely because precision is
missing.

---

# Failure-to-prove tests

The same `Unknown` analysis fact must be testable under both:

```text
a context where positive race-freedom proof is not required;
a context where an explicit rule requires positive race-freedom proof.
```

The first remains an analysis result.

The second may be a compile-time failure-to-prove diagnostic.

Neither case becomes a `ProvenRace` without a race witness.

---

# Lock and synchronization tests

Required cases include:

```text
same lock definitely held
lock only held on one branch
different locks
nested locks protecting an access
synchronization wrapper acquires lock
guard/token passed interprocedurally
lock released before access
```

An access after lock release must not be treated as protected by the old lock
state.

---

# Publication and completion tests

Required cases include:

```text
initialize -> publish -> read
publish -> later unsynchronized mutation/read
spawn -> write -> join -> read
spawn -> write -> read before join
multiple publication relationships
```

The suite must verify that publication does not incorrectly protect unrelated
later mutations.

---

# Place/provenance tests

Required cases include:

```text
different struct fields
same struct field
different constant indices
same constant index
dynamic indices proven unequal
dynamic indices with unknown relation
disjoint slices
overlapping slices
union payload overlap
descriptor storage versus backing storage
```

---

# Ownership and capture tests

Required cases include:

```text
move to worker removes old legal access
illegal post-move access rejected by ownership before race analysis
shared borrow across execution contexts where permitted
exclusive borrow
captured reference in spawned closure
owned closure capture
```

Data race analysis must not duplicate ownership or lifetime diagnostics.

---

# Callable and closure tests

Required cases include:

```text
closed single-target callable
closed multi-target callable
all known targets race-safe
one possible target races
open callable contract
unknown callable memory behavior
callback capturing shared state
```

---

# FFI tests

When canonical foreign contracts are available, required cases include:

```text
foreign call-only read
foreign synchronous write
retained pointer accessed by foreign worker
foreign callback guaranteed same thread
foreign callback with unknown execution thread
foreign callback on known worker thread
foreign synchronization establishing ordering
missing contract causing localized Unknown
```

The test suite must verify that parameter/function names alone never establish
foreign concurrency facts.

---

# ISR tests

Required cases include:

```text
ISR and thread access the same ordinary storage
ISR atomic access according to memory-model rules
interrupt-masked critical section
non-overlapping ISR priority relationship
nested ISR possibility
unknown platform nesting
```

These tests provide reusable race facts for later ISR analysis.

---

# Incremental LSP tests

Required cases include:

```text
introduce concurrent write -> diagnostic appears
add valid synchronization -> diagnostic disappears
remove join -> race appears
restore join -> race disappears
change callable target set -> affected findings refresh
change FFI contract -> affected findings refresh
change unrelated function -> unaffected cached findings remain valid
Pending is never presented as race freedom
change project analysis depth -> affected analysis refines
enable Deep LSP analysis without restart
```

---

# Candidate-generation and performance tests

Large regression cases must cover:

```text
many memory accesses
large shared aggregates
many task creation sites
many execution-context classes
large call graphs
large recursive SCCs
many disjoint partitions
many equivalent abstract races
```

The implementation must demonstrate:

```text
bounded candidate generation
terminating fixed-point analysis
incremental invalidation
diagnostic grouping
deterministic results
```

No normative wall-clock performance threshold is required.

---

# False-positive regression corpus

The project should retain representative safe concurrent patterns that are easy
for a coarse analysis to misclassify.

Important classes include:

```text
parallel disjoint partitions
per-worker independent fields
exclusive ownership transfer
publication followed by immutable reads
synchronization wrapper functions
guard objects or lock tokens
join-before-read
ISR masking
atomic synchronization
```

When a false positive is found in a conforming safe idiom, it should become a
regression case where the required analysis precision supports the proof.

---

# Completion criteria for Sec 0.1

Data race analysis is complete for Sec 0.1 when all of the following are true:

1. canonical memory access events are represented;
2. Place overlap and required disjointness cases are supported;
3. execution contexts and self-overlap are represented;
4. symbolic interprocedural summaries are composable;
5. direct calls and call-graph SCC fixed points work;
6. ownership transfer and escape/publication facts are consumed;
7. must-held synchronization state is represented soundly;
8. canonical happens-before relationships are consumed transitively;
9. candidate pairing is bounded and does not require naïve global event pairing;
10. `ProvenRace`, `ProvenRaceFree`, and `Unknown` remain distinct;
11. failure to prove required race freedom remains distinct from `ProvenRace`;
12. callable target sets and open callable contracts participate;
13. FFI concurrency contracts participate without name-based inference;
14. ISR/thread preemption relationships can participate;
15. Unknown precision loss remains localized and structurally explained;
16. diagnostics provide relational source locations and cause information;
17. separate-compilation summaries are versioned and compatible;
18. incremental LSP analysis invalidates and refreshes affected facts correctly;
19. Interactive, Standard, and Deep follow the common analysis-budget model;
20. the required detection, race-free, Unknown, false-positive, FFI, ISR,
    incremental, performance, and determinism tests pass.

Advanced whole-program context sensitivity may improve precision after Sec 0.1,
but is not required unless an active safety contract requires that additional
proof power.

---

# Implementation governance

This rulebook contains normative analysis behavior only.

Mutable implementation information such as:

```text
current implementation status
source code locations
remaining implementation work
verification commands
known temporary gaps
```

belongs in the repository-level:

```text
implementation-status.yaml
```

The canonical implementation-status integration identifier for this analysis is:

```text
sema.data-race-analysis
```

The ledger must describe observable repository state rather than aspirational
completion.

---

# Normative summary

Data race analysis verifies memory access against the canonical Sec concurrency
memory model rather than defining a second memory model.

The analysis correlates canonical Place/provenance, memory access kinds,
execution contexts, ownership transfer, publication, synchronization,
happens-before, callable behavior, foreign contracts, and platform preemption.

A proven race requires a sound reachable unsafe witness. Lost precision produces
localized `Unknown`, not a fabricated race.

Positive race-freedom proofs are scoped compiler facts and may be established
through disjoint storage, exclusive ownership, non-overlapping execution,
read-only sharing, mutual exclusion, happens-before ordering, or canonical
atomic semantics.

Interprocedural analysis uses symbolic may-access and must-synchronization
summaries, instantiates them at call sites, and solves recursive call-graph SCCs
by deterministic fixed point.

Candidate pairing is bounded through storage, projection, context, access, and
synchronization indexing rather than naïve global pairwise event comparison.

LSP analysis uses progressive refinement within `Interactive`, `Standard`, and
`Deep`; no additional global `Intermediate` mode exists. `Pending` remains
distinct from `Unknown` and all proven results.

Proven races and required race-freedom failures are NeedToKnow diagnostics.
Positive proof explanations and detailed concurrency relationships are
configurable OptionalInsight.

`sec analyse` includes data race analysis by default as part of all-analysis
behavior and may also run it explicitly.

Separate compilation uses versioned symbolic summaries and contracts.
Implementation progress remains outside this rulebook in
`implementation-status.yaml`.
