# Data Race Analysis

- **Status:** Normative
- **Created:** 2026-07-29
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/data_races.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`

---

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

## § 1 Purpose

**§ 1(1)** Data race analysis answers questions such as:

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

**§ 1(2)** Data race analysis is not a replacement for ownership, borrowing, escape,
lifetime, Place/provenance, call-graph, closure, FFI, effect, or concurrency
memory-model analysis.

**§ 1(3)** It consumes canonical facts from those analyses and correlates them.

---

## § 2 Normative ownership

**§ 2(1)** The canonical concurrency memory model owns:

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

**§ 2(2)** Data race analysis owns:

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

**§ 2(3)** The analysis must not silently redefine a rule owned by the concurrency memory
model merely to make analysis easier.

---

## § 3 Core race condition

**§ 3(1)** A data race requires all of the following semantic conditions:

```text
1. two relevant accesses may target overlapping storage;
2. the accesses conflict according to the concurrency memory model;
3. their execution contexts may overlap in a permitted execution;
4. the ordering or exclusion required by the concurrency memory model is absent.
```

**§ 3(2)** Conceptually:

```text
RaceCandidate(A, B)
    if
        MayOverlap(A.Place, B.Place)
        and ConflictingAccess(A, B)
        and MayExecuteConcurrently(A, B)
        and not OrderedOrExcluded(A, B)
```

**§ 3(3)** This expression is explanatory. The implementation is not required to use this
exact representation.

**§ 3(4)** A syntactic pattern such as "the same variable is used in two threads" is not a
sufficient race model.

---

## § 4 Memory access events

**§ 4(1)** The analysis operates on semantic memory access events.

**§ 4(2)** A conceptual event contains at least:

```text
MemoryAccessEvent {
    Place
    AccessKind
    ExecutionContext
    Atomicity
    Source
}
```

**§ 4(3)** The exact internal form is implementation-defined.

### § 4.1 Access kinds

**§ 4.1(1)** The analysis must distinguish access behavior sufficiently to identify
conflicts. At minimum this includes the semantic equivalent of:

```text
Read
Write
ReadWrite
```

**§ 4.1(2)** Whether two access kinds conflict is interpreted according to the canonical
concurrency memory model.

**§ 4.1(3)** Ordinary concurrent reads do not form a data race merely because they occur at
the same time unless the canonical rules for the relevant storage class say
otherwise.

---

## § 5 Canonical Place and provenance

**§ 5(1)** Data race analysis uses canonical Place/provenance as the identity of accessed
storage.

**§ 5(2)** For example:

```sec
state.Left.Count += 1
```

**§ 5(3)** must remain structurally distinguishable from:

```sec
state.Right.Count += 1
```

**§ 5(4)** when Place analysis proves the corresponding storage disjoint.

**§ 5(5)** The analysis must preserve available precision for:

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

**§ 5(6)** It must not unnecessarily collapse a precise Place to its root aggregate.

---

## § 6 Storage overlap

**§ 6(1)** Before two access events can form a race candidate, their Places must be capable
of overlapping.

**§ 6(2)** The implementation must represent at least the semantic distinctions:

```text
Disjoint
Overlap
MayOverlap
```

**§ 6(3)** An implementation may retain additional internal precision.

**§ 6(4)** `Disjoint` eliminates the candidate.

**§ 6(5)** `Overlap` means the relevant storage relationship is established.

**§ 6(6)** `MayOverlap` means the analysis cannot prove disjointness. It is not by itself
proof that the accesses actually target the same storage in a reachable unsafe
execution.

**§ 6(7)** Loss of alias precision must not be silently converted into `ProvenRace`.

---

## § 7 Execution contexts

**§ 7(1)** Conflicting accesses are relevant to race analysis only when their execution
contexts may overlap.

**§ 7(2)** A conceptual execution-context model may include:

```text
Thread
Task
ISR
ForeignCallback
PlatformExecution
Unknown
```

**§ 7(3)** The exact representation is implementation-defined.

**§ 7(4)** The analysis must reason from canonical execution semantics rather than from
function identity.

**§ 7(5)** Two calls to different functions may execute sequentially.

**§ 7(6)** Two runtime instances of the same function may execute concurrently.

**§ 7(7)** Ordinary synchronous calls normally inherit the execution context of their
caller and do not create concurrency merely by crossing a function boundary.

---

## § 8 Execution-context abstraction

**§ 8(1)** Execution contexts must use a finite static abstraction.

**§ 8(2)** A task or thread creation site that executes repeatedly must not require an
unbounded number of static context nodes.

**§ 8(3)** A suitable abstraction may represent a context class by creation site and retain
whether multiple runtime instances may overlap.

**§ 8(4)** Conceptually:

```text
TaskContext(creation_site)
MayHaveMultipleConcurrentInstances = true
```

**§ 8(5)** This matters because one task body may race with another runtime instance of the
same task body.

**§ 8(6)** Function equality does not imply execution-instance equality.

---

## § 9 Concurrency relationships

**§ 9(1)** The analysis must be able to distinguish relationships equivalent to:

```text
MayOverlap
CannotOverlap
UnknownOverlap
```

**§ 9(2)** It may also retain start/completion or other more precise relations.

**§ 9(3)** These relations are semantic and need not correspond to physical timestamps.

**§ 9(4)** A possible legal overlap is enough for race analysis when the other race
conditions can be established. The accesses do not need to overlap on every
possible schedule.

---

## § 10 Concurrency intervals

**§ 10(1)** The analysis should retain lifecycle relationships for spawned or otherwise
concurrent execution contexts when canonical semantics provide them.

**§ 10(2)** For example:

```text
spawn Worker
    |
    | possible overlap
    |
join Worker
```

**§ 10(3)** A canonical join/completion relation may prove that the worker no longer
overlaps the continuation after the join.

**§ 10(4)** The analysis should exploit such intervals where available rather than assuming
that a spawned execution remains concurrent forever.

**§ 10(5)** Scheduler coincidence is not a safety proof. Non-overlap may be used only when
it is guaranteed by language, runtime, platform, or explicit contract semantics.

---

## § 11 Core result classes

**§ 11(1)** Data race analysis distinguishes at least:

```text
ProvenRace
ProvenRaceFree
Unknown
```

### § 11.1 ProvenRace

**§ 11.1(1)** `ProvenRace` means the compiler has a sound witness that there exists a reachable
permitted execution in which conflicting accesses can operate on overlapping
storage without the ordering or exclusion required by the canonical concurrency
memory model.

### § 11.2 ProvenRaceFree

**§ 11.2(1)** `ProvenRaceFree` means the compiler has a positive proof for the stated scope
that the relevant conflict cannot occur.

**§ 11.2(2)** Possible proof reasons include:

```text
DisjointStorage
ExclusiveOwnership
NoConcurrentExecution
ReadOnly
MutualExclusion
HappensBefore
AtomicRaceSafe
```

**§ 11.2(3)** The exact internal classification is implementation-defined.

### § 11.3 Unknown

**§ 11.3(1)** `Unknown` means the analysis cannot prove either a race or race freedom for the
relevant relationship.

**§ 11.3(2)** Failure to prove a race is not proof of race freedom.

**§ 11.3(3)** Failure to prove race freedom is not proof of a race.

---

## § 12 Required proof versus proven violation

**§ 12(1)** A language, target, runtime, or CompilationPlan contract may require positive
race-freedom proof for a particular operation or execution model.

**§ 12(2)** In such a context:

```text
Unknown
```

**§ 12(3)** may make compilation fail because the required proof is unavailable.

**§ 12(4)** This must remain diagnostically distinct from:

```text
ProvenRace
```

**§ 12(5)** The compiler must distinguish:

```text
proof of a violation
```

**§ 12(6)** from:

```text
failure to prove a required safety property
```

---

## § 13 Localized Unknown

**§ 13(1)** Loss of precision in one relationship must not erase unrelated known facts.

**§ 13(2)** If most shared accesses are proven race-free and one foreign callback has
unknown concurrency behavior, the analysis should preserve the proven results
and localize uncertainty to the affected accesses and contexts.

**§ 13(3)** Useful structural unknown reasons include:

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

**§ 13(4)** The exact enum is implementation-defined, but the cause of lost proof precision
must remain representable.

---

## § 14 Borrowing as race-freedom evidence

**§ 14(1)** Data race analysis consumes canonical borrowing results.

**§ 14(2)** If the borrow checker establishes that a mutable Place is exclusively accessible
for the relevant interval, competing ordinary access cannot coexist when the
borrowing semantics guarantee that exclusivity.

**§ 14(3)** Conceptually:

```text
ExclusiveBorrow
    -> competing ordinary access cannot coexist
```

**§ 14(4)** Data race analysis does not reimplement borrow checking.

**§ 14(5)** Borrow errors remain owned by borrowing/lifetime/ownership analysis.

---

## § 15 Ownership transfer

**§ 15(1)** Ownership transfer can eliminate shared access entirely.

**§ 15(2)** For example:

```sec
spawn Worker(move value)
```

**§ 15(3)** may establish that the old execution context no longer has legal access to the
moved value.

**§ 15(4)** When ownership and escape analysis prove exclusive transfer and no competing
alias remains, the storage is not concurrently shared merely because two
execution contexts exist.

**§ 15(5)** Data race analysis must consume this proof rather than first treating every
spawn argument as shared memory.

---

## § 16 Concurrent borrowed storage

**§ 16(1)** If Sec permits a borrow or view to cross an execution-context boundary, its
lifetime and ownership legality must first be established by the owning
analyses.

**§ 16(2)** Data race analysis then reasons about memory access through the valid shared
dependency.

**§ 16(3)** It must not use race analysis to legalize an otherwise invalid lifetime or
ownership transfer.

---

## § 17 Publication

**§ 17(1)** Publication and race freedom are distinct properties.

**§ 17(2)** Correct publication may establish ordering between initialization and another
context observing the published object.

**§ 17(3)** It does not automatically make all later concurrent mutation race-free.

**§ 17(4)** Conceptually:

```text
initialize object
publish object
other context observes publication
read object
```

**§ 17(5)** may be ordered according to the canonical memory model.

**§ 17(6)** Later mutation still requires the ordering, exclusion, ownership, or atomic
semantics required by that model.

---

## § 18 Publication summaries

**§ 18(1)** Interprocedural analysis must be able to represent publication effects
symbolically.

**§ 18(2)** A conceptual publication fact may include:

```text
PublishedOrigin
DestinationContextClass
OrderingEstablished
```

**§ 18(3)** The exact representation is implementation-defined.

**§ 18(4)** Publication may occur indirectly through retained references, registries,
callbacks, runtime facilities, or foreign code when canonical summaries or
contracts establish that behavior.

---

## § 19 Synchronization and happens-before

**§ 19(1)** Data race analysis consumes canonical synchronization semantics.

**§ 19(2)** It must be able to query or derive a semantic relationship equivalent to:

```text
HB(A, B)
```

**§ 19(3)** where the edge semantics come from the concurrency memory model.

**§ 19(4)** Possible canonical sources include:

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

**§ 19(5)** This list does not redefine those operations. It identifies the kinds of
canonical facts that may be consumed.

---

## § 20 Happens-before transitivity

**§ 20(1)** Happens-before is transitive.

**§ 20(2)** If:

```text
A happens-before B
B happens-before C
```

**§ 20(3)** then the analysis may use:

```text
A happens-before C
```

**§ 20(4)** The implementation is not required to materialize a complete global transitive
closure. Graph reachability, summaries, incremental indexes, or equivalent
sound mechanisms may be used.

---

## § 21 Program order

**§ 21(1)** Sequential operations in one execution instance are ordered according to the
canonical program-order semantics.

**§ 21(2)** For example:

```sec
state.Value = 1
let value := state.Value
```

**§ 21(3)** does not create a race merely because both events access the same Place.

**§ 21(4)** Program order in one runtime instance must not be confused with ordering
between separate concurrent runtime instances of the same function body.

---

## § 22 Path compatibility

**§ 22(1)** Access pairing must account for control-flow compatibility.

**§ 22(2)** Two accesses in mutually exclusive branches of one execution instance do not
execute concurrently merely because both are present in the CFG.

**§ 22(3)** For example:

```sec
if condition {
    state.Value = 1
} else {
    state.Value = 2
}
```

**§ 22(4)** does not form a race within that execution instance.

**§ 22(5)** However, two concurrent runtime instances may each execute one of the branches.
Path compatibility must therefore be interpreted relative to execution
instances, not only source-level CFG nodes.

---

## § 23 Locks and canonical synchronization identity

**§ 23(1)** A lock protects accesses only when canonical synchronization semantics establish
the relevant relationship.

**§ 23(2)** Naming conventions are never sufficient.

**§ 23(3)** The analysis must not infer that a lock named `stateLock` protects `state`, or
that two locks with similar names are identical.

**§ 23(4)** Lock identity is derived from canonical Place/provenance, synchronization object
identity, guard/token semantics, or explicit contract information.

---

## § 24 Must-held lock state

**§ 24(1)** A positive lock-based race-freedom proof requires the relevant lock to be
definitely held during the memory access.

**§ 24(2)** It is insufficient that the lock may be held on some path.

**§ 24(3)** For example:

```sec
if condition {
    lock.Lock()
}

state.Value = 10
```

**§ 24(4)** must not produce a `MustHold(lock)` proof at the write after the control-flow
join.

**§ 24(5)** At joins, positive must-held facts survive only when they hold on every relevant
incoming path.

---

## § 25 Multiple locks

**§ 25(1)** Two protected accesses need not have identical held-lock sets.

**§ 25(2)** If the canonical synchronization semantics prove mutual exclusion through a
shared lock identity or another valid relationship, that proof may be used.

**§ 25(3)** Conversely:

```text
A holds L1
B holds L2
```

**§ 25(4)** does not by itself prove race freedom for overlapping storage.

**§ 25(5)** Lock ordering belongs primarily to deadlock analysis. Data race analysis needs
only the synchronization/exclusion facts necessary to classify memory access.

---

## § 26 Synchronization wrappers

**§ 26(1)** Interprocedural analysis must support wrapper functions that establish
synchronization.

**§ 26(2)** For example:

```sec
fn WithLock(lock: ref Mutex, action: fn()) {
    lock.Lock()
    defer lock.Unlock()
    action()
}
```

**§ 26(3)** must remain analyzable when callable contracts and synchronization summaries are
sufficient.

**§ 26(4)** Programmers must not be forced to place lock acquisition syntactically in the
same function as every protected memory access merely to obtain race analysis.

---

## § 27 Locks held across function boundaries

**§ 27(1)** If synchronization APIs permit a function to return while a lock or guard is
still semantically held, summaries must represent this through canonical state,
ownership, or guard-token facts.

**§ 27(2)** The analysis must not assume that every function releases every lock before
return unless the language or API semantics guarantee that property.

---

## § 28 Atomic accesses

**§ 28(1)** Atomic access legality and ordering are interpreted exclusively according to the
canonical concurrency memory model.

**§ 28(2)** Data race analysis must distinguish:

```text
race legality
```

**§ 28(3)** from:

```text
strong ordering or visibility guarantees
```

**§ 28(4)** Two accesses may be race-legal under atomic semantics without providing the
stronger ordering needed for an unrelated correctness property.

**§ 28(5)** The latter is outside the scope of this rulebook unless another analysis
consumes the ordering facts.

---

## § 29 Atomic and ordinary access

**§ 29(1)** The analysis must not classify a mixed atomic/ordinary access pair from general
intuition.

**§ 29(2)** The canonical concurrency memory model determines whether and under what
conditions the combination is legal.

**§ 29(3)** Data race analysis consumes that rule.

---

## § 30 Raw pointers and unsafe code

**§ 30(1)** Raw-pointer use may reduce alias precision but must not make the analysis
intentionally blind.

**§ 30(2)** Where canonical provenance is known, it should be retained.

**§ 30(3)** For example, a raw pointer known to derive from `state.Buffer` should remain
associated with that origin as far as the provenance model permits.

**§ 30(4)** When provenance is genuinely lost, the affected alias relationship becomes
`Unknown` or `MayOverlap` as appropriate.

**§ 30(5)** Loss of provenance must not be interpreted as proof of safety.

---

## § 31 FFI boundary

**§ 31(1)** FFI is an explicit contract boundary for data race analysis.

**§ 31(2)** Foreign behavior may include:

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

**§ 31(3)** Such behavior is consumed only from canonical FFI contracts, trusted imported
metadata, generated bindings, or compiler-known platform/library contracts.

**§ 31(4)** Data race analysis must not infer concurrency behavior from foreign function or
parameter names.

**§ 31(5)** The import mechanism itself is outside this rulebook.

---

## § 32 Foreign callbacks

**§ 32(1)** A foreign callback may introduce a new execution context.

**§ 32(2)** If an FFI contract states that a callback may be invoked from foreign worker
threads, the corresponding execution relationship participates in race
analysis.

**§ 32(3)** If the contract establishes same-thread synchronous callback behavior, the
analysis may use that fact.

**§ 32(4)** If callback execution context is unspecified, only the affected concurrency
dimension becomes `Unknown`.

---

## § 33 Foreign ordering

**§ 33(1)** Sufficiently precise foreign contracts may define relationships such as:

```text
foreign worker begins after registration
foreign worker may access buffer
foreign worker completes before JoinForeign returns
```

**§ 33(2)** When canonical, these facts may be treated similarly to Sec task/thread
ordering.

**§ 33(3)** Missing foreign information is not replaced by guessed behavior.

---

## § 34 ISR interaction

**§ 34(1)** ISR and ordinary execution can access the same storage.

**§ 34(2)** Data race analysis must be able to consume canonical platform facts such as:

```text
ISR may preempt context
interrupts disabled for a critical interval
ISR priority relation
ISR nesting permitted
ISR nesting forbidden
```

**§ 34(3)** The platform or CompilationPlan owns these semantics.

**§ 34(4)** Data race analysis uses them to determine whether conflicting memory accesses
may overlap and whether canonical exclusion exists.

**§ 34(5)** `isr_analysis.md` may later impose additional ISR restrictions; it does not need
a separate race engine.

---

## § 35 Asymmetric preemption

**§ 35(1)** ISR concurrency need not be modeled as symmetric scheduling.

**§ 35(2)** If an ISR may preempt a thread, conflicting accesses can be concurrently
relevant even though the thread cannot preempt the ISR in the reverse
direction.

**§ 35(3)** The race analysis requires a sound `ConcurrentConflictPossible` relationship,
not a symmetric scheduler model.

---

## § 36 Data-race and deadlock analysis

**§ 36(1)** Data race and deadlock analysis are sibling analyses.

**§ 36(2)** They may consume shared canonical synchronization identities and lock-state
facts, but their responsibilities differ:

```text
DataRaceAnalysis:
    is shared memory access sufficiently ordered or excluded?

DeadlockAnalysis:
    can synchronization dependencies prevent progress?
```

**§ 36(3)** A lock may simultaneously remove a data race and participate in a deadlock
cycle.

**§ 36(4)** Data race analysis must not duplicate the deadlock lock-order graph.

---

## § 37 Function summaries

**§ 37(1)** Each analyzable function must be capable of producing a composable summary of
its race-relevant behavior.

**§ 37(2)** Conceptually:

```text
DataRaceSummary {
    MemoryEffects
    SynchronizationEffects
    ExecutionEffects
    PublicationEffects
    Precision
}
```

**§ 37(3)** The exact data structure is implementation-defined.

**§ 37(4)** Summaries exist so callers and separate compilation can reason without
re-analyzing every function body in every context.

---

## § 38 Symbolic memory roots

**§ 38(1)** Function summaries preserve access roots symbolically.

**§ 38(2)** Relevant roots include the semantic equivalent of:

```text
Parameter(n)
Receiver
Static(symbol)
ThreadLocal(symbol)
Captured(origin)
Foreign(origin)
Unknown
```

**§ 38(3)** For example:

```sec
fn Update(state: ref mut State) {
    state.Counter += 1
}
```

**§ 38(4)** should retain an effect equivalent to:

```text
Parameter(0).Counter:
    MayRead
    MayWrite
```

**§ 38(5)** rather than collapsing to `UnknownMemory`.

---

## § 39 Symbolic projections

**§ 39(1)** Summary projections must retain the precision supported by canonical Place
analysis.

**§ 39(2)** For example:

```text
Parameter(0).Left.Count
```

**§ 39(3)** must remain distinguishable from:

```text
Parameter(0).Right.Count
```

**§ 39(4)** when the language and Place semantics prove those fields disjoint.

**§ 39(5)** Likewise, index and slice abstractions should retain disjoint range information
where proven.

---

## § 40 May-access summaries

**§ 40(1)** Interprocedural memory effects are primarily may-effects.

**§ 40(2)** At minimum the summary distinguishes effects equivalent to:

```text
MayRead(Place)
MayWrite(Place)
```

**§ 40(3)** A may-write fact means a reachable path may perform the write. It does not mean
that every invocation writes.

**§ 40(4)** At control-flow joins, may-effects are conservatively joined by union.

---

## § 41 Must-state summaries

**§ 41(1)** Positive protection facts are must-properties.

**§ 41(2)** For example, an access protected by a mutex requires proof that the relevant
mutex is held on every path reaching that access under the summarized
condition.

**§ 41(3)** At control-flow joins, must-properties survive only when valid for all relevant
incoming paths.

**§ 41(4)** The analysis must keep may- and must-domains conceptually separate.

---

## § 42 Call-site summary instantiation

**§ 42(1)** A caller instantiates callee symbolic roots using actual call-site provenance.

**§ 42(2)** For example:

```text
Callee summary:
    Parameter(0).Value -> MayWrite

Call site:
    Set(sharedState)

Instantiated effect:
    sharedState.Value -> MayWrite
```

**§ 42(3)** This same principle applies to synchronization identities, publication effects,
captured origins, and other symbolic relationships.

---

## § 43 Synchronous calls

**§ 43(1)** An ordinary synchronous call executes in the caller's execution context unless
canonical language, runtime, or foreign semantics state otherwise.

**§ 43(2)** Function boundaries alone do not introduce concurrency.

---

## § 44 Spawn and thread creation

**§ 44(1)** Spawn, explicit thread creation, runtime task creation, foreign asynchronous
callbacks, ISR entry, and similar operations establish new execution contexts
according to their canonical semantics.

**§ 44(2)** The summary model must retain the execution effects necessary to determine
which contexts may overlap and what storage is shared between them.

---

## § 45 Self-concurrent creation sites

**§ 45(1)** A single creation site may create multiple overlapping runtime instances.

**§ 45(2)** For example:

```sec
for item in items {
    spawn Process(item)
}
```

**§ 45(3)** must allow the analysis to compare one `Process` instance with another when the
runtime semantics permit overlap.

**§ 45(4)** This is required even though both instances share the same semantic body.

---

## § 46 Join and completion

**§ 46(1)** Canonical join/completion semantics may create happens-before relations.

**§ 46(2)** For example:

```text
worker writes X
worker completes
main joins worker
main reads X
```

**§ 46(3)** may be proven ordered when the concurrency memory model defines the appropriate
completion/join relation.

**§ 46(4)** The data race analysis consumes this relation; it does not define it.

---

## § 47 Callable-value calls

**§ 47(1)** Callable-flow and closure analysis provide possible target information for
function-value calls.

**§ 47(2)** For a closed target set, data-race summaries are conservatively joined over all
possible targets.

**§ 47(3)** Conceptually:

```text
Summary(call) = join(summary(target) for each possible target)
```

**§ 47(4)** Known target identities may also be used to recover more precise source cause
paths.

---

## § 48 Open callable contracts

**§ 48(1)** An open callable target set requires a callable contract sufficient to describe
the race-relevant behavior permitted by unknown targets.

**§ 48(2)** Relevant dimensions may include the semantic equivalent of:

```text
MayRead
MayWrite
MayRetain
MayPublish
MaySpawn
ConcurrencyBehavior
SynchronizationBehavior
```

**§ 48(3)** This rulebook does not define source syntax for those contracts.

**§ 48(4)** Missing information widens only the affected dimensions.

**§ 48(5)** An unknown callable is not automatically modeled as "writes all memory" when
more structured information is available.

---

## § 49 Recursive call graphs

**§ 49(1)** Data-race summaries are solved over the canonical call graph.

**§ 49(2)** Recursive and mutually recursive SCCs use deterministic monotone fixed-point
analysis.

**§ 49(3)** Data race analysis must not construct a parallel call graph.

**§ 49(4)** Ordinary recursion does not itself imply concurrency. Multiple recursive stack
frames in one thread remain sequential execution unless a spawn, callback,
thread, ISR, or other concurrency-producing semantic edge is involved.

---

## § 50 Recursive spawning

**§ 50(1)** Recursive creation of concurrent work may create arbitrarily many runtime
instances, but data race analysis does not need an exact runtime instance count
to determine whether two abstract instances may overlap.

**§ 50(2)** A finite context abstraction with a sound self-overlap property is sufficient
for baseline race reasoning.

---

## § 51 Index and slice abstractions

**§ 51(1)** Large arrays and dynamic index sets must use bounded Place abstractions.

**§ 51(2)** The analysis must not create a distinct static Place node for every possible
runtime index in a large collection.

**§ 51(3)** Where range analysis proves indices or backing ranges disjoint, the race
analysis should preserve that proof.

**§ 51(4)** Otherwise the relationship may widen to `MayOverlap`.

---

## § 52 Parallel disjoint partitions

**§ 52(1)** Sec race analysis must support race-free proofs for parallel access to disjoint
parts of one backing collection.

**§ 52(2)** For example:

```sec
left := values[0..<mid]
right := values[mid..<values.len]

spawn Process(ref mut left)
Process(ref mut right)
```

**§ 52(3)** may be proven race-free when canonical Place/range/borrowing facts establish
that the backing ranges are disjoint and the cross-context transfer is valid.

**§ 52(4)** A shared backing allocation does not imply overlapping storage.

---

## § 53 Disjoint aggregate fields

**§ 53(1)** Likewise, different fields of an aggregate may be accessed concurrently without
synchronization when canonical layout/Place/borrowing semantics prove the
storage disjoint and the execution-context transfer is valid.

**§ 53(2)** The analysis must not unnecessarily coarse-grain every field access to the
aggregate root.

---

## § 54 Union payloads

**§ 54(1)** Different union payload projections are not automatically disjoint.

**§ 54(2)** Union payload storage may overlap physically.

**§ 54(3)** Data race analysis must consume canonical Place/layout/active-variant facts and
must not infer field-like disjointness merely from different payload names.

---

## § 55 MMIO and volatile storage

**§ 55(1)** Memory-mapped I/O, volatile access, and target-defined memory spaces may have
special synchronization and access semantics.

**§ 55(2)** Data race analysis must consume the canonical memory-space and target rules.

**§ 55(3)** It must not blindly apply ordinary mutex/atomic assumptions to MMIO.

---

## § 56 Race candidate generation

**§ 56(1)** The analysis must not require naïve comparison of every memory event with every
other memory event in a large program.

**§ 56(2)** Candidate generation should first exploit canonical facts such as:

```text
Place root or alias class
projection or range class
access kind
execution-context class
synchronization class
```

**§ 56(3)** Accesses that are provably disjoint, non-conflicting, or unable to overlap in
execution should be eliminated before detailed race pairing.

**§ 56(4)** The exact index data structures are implementation-defined.

---

## § 57 Summary-level candidate pairing

**§ 57(1)** Whole-program analysis may compare symbolic summaries before expanding to
individual event pairs.

**§ 57(2)** For example:

```text
Context A may write Parameter(0).Data
Context B may read Parameter(0).Data
```

**§ 57(3)** can first establish a potentially relevant relationship.

**§ 57(4)** Detailed source events need be recovered only when required for proof,
precision, diagnostics, or cause reporting.

**§ 57(5)** This permits scalable analysis without changing race semantics.

---

## § 58 Proven race witness

**§ 58(1)** `ProvenRace` requires a sound reachable witness.

**§ 58(2)** The compiler must establish enough canonical facts to show that a legal
execution exists where:

```text
A and B conflict;
A and B may operate on overlapping storage;
A and B may overlap in execution;
no required ordering or exclusion prevents the conflict.
```

**§ 58(3)** The witness need not be a concrete fully scheduled runtime trace, but it must be
stronger than mere loss of precision.

---

## § 59 May-overlap and proof strength

**§ 59(1)** `MayOverlap` storage together with `MayExecuteConcurrently` is not automatically
`ProvenRace` if the alias/concurrency facts are too weak to establish an unsafe
reachable execution under the active rules.

**§ 59(2)** When proof strength is insufficient, the result remains `Unknown` unless the
program is rejected because an explicit positive race-freedom proof is required.

**§ 59(3)** This is particularly important around raw pointers, open callable contracts,
FFI, and platform-specific preemption.

---

## § 60 Widening

**§ 60(1)** Analysis state must remain finite.

**§ 60(2)** If Place origins, target sets, execution contexts, paths, or other dimensions
become too large, the implementation may widen:

```text
precise finite set
    -> structured abstraction
    -> Unknown for the affected dimension
```

**§ 60(3)** Widening should preserve known structure where practical.

**§ 60(4)** For example:

```text
Place = Parameter(0).AnyIndex
ExecutionContext = known worker class
MustHold = known lock L
```

**§ 60(5)** is more useful than global `Unknown`.

**§ 60(6)** Widening must never fabricate a `ProvenRace` or `ProvenRaceFree` result.

---

## § 61 Positive race-free proofs

**§ 61(1)** Positive proofs are first-class compiler facts.

**§ 61(2)** A conceptual proof may contain:

```text
RaceFreeProof {
    Place
    ContextRelation
    Reason
    Assumptions
}
```

**§ 61(3)** A proof is scoped.

**§ 61(4)** For example, proving `state.Left` race-free between two worker contexts does not
prove every field of `state` or every execution context race-free.

**§ 61(5)** Consumers must preserve the proof scope and assumptions.

---

## § 62 Cause paths

**§ 62(1)** A proven race should retain representative source access paths.

**§ 62(2)** For example:

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

**§ 62(3)** Cause paths should use canonical call/execution summaries and be deterministic.

---

## § 63 Diagnostic grouping

**§ 63(1)** A single abstract race may correspond to many runtime instances or repeated loop
iterations.

**§ 63(2)** The compiler should group equivalent race occurrences so one logical problem
does not produce an unbounded or overwhelming number of diagnostics.

**§ 63(3)** Grouping may use a semantic key equivalent to:

```text
Place class
context pair
root cause
representative access pair
```

**§ 63(4)** Distinct root causes must remain distinguishable.

**§ 63(5)** A race on `state.Counter` and an unrelated race on `state.Buffer` should not be
merged merely because they occur in the same task pair.

---

## § 64 Diagnostics

**§ 64(1)** A proven race diagnostic must be distinguishable from failure to prove required
race freedom.

### § 64.1 Proven race

**§ 64.1(1)** A diagnostic may conceptually report:

```text
error: data race on `state.Count`

write:
    WorkerA -> Update -> state.Count

read:
    WorkerB -> Snapshot -> state.Count

these execution contexts may overlap and no required ordering protects the
conflicting accesses
```

### § 64.2 Failure to prove race freedom

**§ 64.2(1)** A different diagnostic may report:

```text
error: race freedom could not be proven for `buffer`

reason:
    foreign callback concurrency is unknown
```

**§ 64.2(2)** The latter must not claim that a race was proven.

---

## § 65 Diagnostic related locations

**§ 65(1)** Race diagnostics are relational and should expose both conflicting source
locations when available.

**§ 65(2)** LSP diagnostics should provide a primary source location and related location so
a programmer can navigate between the accesses.

**§ 65(3)** Interprocedural call or execution cause paths may additionally be shown.

---

## § 66 Diagnostic explanation

**§ 66(1)** Where available, a diagnostic should explain:

```text
why the storage overlaps;
why the execution contexts may overlap;
which access writes;
which required ordering or exclusion is absent;
which foreign, callable, alias, platform, or synchronization fact is unknown.
```

**§ 66(2)** For example:

```text
both accesses refer to `state.Count`
```

**§ 66(3)** or:

```text
both views may overlap the same backing range
```

**§ 66(4)** or:

```text
no happens-before edge orders the worker write before this read
```

---

## § 67 Race diagnostics do not prescribe locking

**§ 67(1)** The compiler must not automatically prescribe a mutex as the universal fix for
a race.

**§ 67(2)** A mutex may be the wrong synchronization strategy, may be forbidden in the
execution context, may harm performance, or may create deadlock.

**§ 67(3)** Diagnostics should explain the missing safety relationship and may suggest that
the programmer establish synchronization, ownership transfer, disjoint storage,
non-overlap, atomic semantics, or another valid design.

**§ 67(4)** Architectural synchronization choices remain explicit programmer decisions.

---

## § 68 Automatic fixes

**§ 68(1)** Automatic race fixes are allowed only when the transformation is canonical and
proven safe under the relevant semantics.

**§ 68(2)** General race repair is not a source-text rewrite problem.

**§ 68(3)** The baseline tooling priority is therefore:

```text
clear explanation
navigation between conflicting accesses
cause path
missing-proof explanation
```

**§ 68(4)** rather than aggressive automatic code modification.

---

## § 69 Analysis budgets

**§ 69(1)** Data race analysis uses the common Sec analysis-budget model:

```text
Interactive
Standard
Deep
```

**§ 69(2)** There is no separate global `Intermediate` analysis mode.

---

## § 70 Interactive analysis

**§ 70(1)** Interactive analysis prioritizes low-latency sound results using available local
facts and cached summaries.

**§ 70(2)** Typical fast inputs include:

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

**§ 70(3)** Interactive analysis may return incomplete tooling state while deeper affected
relationships are being refined.

---

## § 71 Progressive LSP refinement

**§ 71(1)** LSP execution is progressively refined within the existing analysis modes.

**§ 71(2)** Conceptually:

```text
Interactive fast result
    -> incremental Interactive refinement
    -> Standard-equivalent result
    -> optional Deep refinement
```

**§ 71(3)** The intermediate steps are scheduling and precision refinements, not a new
language-analysis mode.

**§ 71(4)** The compiler must not introduce a separate `Intermediate` semantic mode merely
to describe background LSP work.

---

## § 72 Pending tooling state

**§ 72(1)** `Pending` is distinct from:

```text
Unknown
ProvenRace
ProvenRaceFree
```

**§ 72(2)** `Pending` means analysis required for the current tooling result has not yet
completed.

**§ 72(3)** The LSP must never treat `Pending` as race freedom.

**§ 72(4)** Likewise, a stale `ProvenRaceFree` result must not remain active after a relevant
dependency has been invalidated.

---

## § 73 Standard analysis

**§ 73(1)** Standard is the stable analysis level used for normal compilation.

**§ 73(2)** Every data-race proof required for source validity or the active CompilationPlan
must be completed in Standard.

**§ 73(3)** Required safety analysis must not be deferred to Deep.

**§ 73(4)** Where additional precision is necessary to discharge an active required
race-freedom contract, that precision becomes required for the build.

---

## § 74 Deep analysis

**§ 74(1)** Deep may improve precision with more expensive reasoning such as:

```text
path-sensitive concurrency intervals
more precise range partitioning
context-sensitive callable refinement
cross-module access correlation
symbolic lock-identity refinement
more precise foreign callback correlation
richer positive race-free explanation
```

**§ 74(2)** Deep may reduce `Unknown` to a stronger proven result.

**§ 74(3)** It does not change the concurrency memory model or source semantics.

**§ 74(4)** A project may configure the LSP to run Deep analysis continuously where project
size and machine resources make that practical.

---

## § 75 LSP presentation classes

**§ 75(1)** LSP presentation distinguishes:

```text
NeedToKnow
OptionalInsight
```

### § 75.1 NeedToKnow

**§ 75.1(1)** The following are NeedToKnow:

```text
ProvenRace
required race-freedom proof failed
```

**§ 75.1(2)** They are compiler diagnostics rather than optional explanatory decoration.

### § 75.2 OptionalInsight

**§ 75.2(1)** Examples include:

```text
race-free because ranges are disjoint
race-free because both accesses are protected by lock X
publication establishes the relevant ordering
these execution contexts cannot overlap
detailed shared-Place access summaries
```

**§ 75.2(2)** These are configurable and should not flood ordinary editor use.

---

## § 76 LSP hover and analysis views

**§ 76(1)** Normal hover should not become a full concurrency-analysis dump.

**§ 76(2)** When detailed or Deep presentation is enabled, tooling may expose information
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

**§ 76(3)** Positive proof explanations are OptionalInsight unless needed to explain an
active diagnostic.

---

## § 77 LSP configuration reload

**§ 77(1)** Project analysis settings may control race-analysis depth and optional
presentation.

**§ 77(2)** The LSP must:

```text
read relevant project configuration when the workspace opens;
watch the configuration for changes;
reload changed settings;
invalidate affected summaries, proofs, and findings;
recompute affected analysis;
refresh diagnostics and dependent UI state;
avoid requiring an LSP restart.
```

**§ 77(3)** This rulebook does not define the project configuration syntax.

---

## § 78 Incremental invalidation

**§ 78(1)** Race analysis is dependency-driven.

**§ 78(2)** Relevant changes may include:

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

**§ 78(3)** Only findings, proofs, summaries, and dependent callers/contexts affected by the
change should be invalidated where the dependency graph can determine that
scope.

---

## § 79 Stable-summary propagation

**§ 79(1)** If a changed function re-analyzes to an exported data-race summary semantically
equivalent to the previous summary, unnecessary transitive caller invalidation
should be stoppable.

**§ 79(2)** This is an implementation scalability requirement, not a change in semantics.

---

## § 80 Separate compilation

**§ 80(1)** Data race analysis must support separate compilation through versioned symbolic
summaries and explicit contracts.

**§ 80(2)** Imported summaries may describe:

```text
parameter and receiver accesses
static accesses
publication and retention
execution-context effects
must-held synchronization
callback concurrency
```

**§ 80(3)** When a body is unavailable, the compatible summary or explicit contract is the
analysis source of truth for the exported behavior.

**§ 80(4)** Missing information widens only the affected dimensions.

---

## § 81 Summary versioning and invalidation

**§ 81(1)** Persisted summaries must be invalidated or rejected when their semantic basis is
incompatible.

**§ 81(2)** Relevant dependencies include:

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

**§ 81(3)** The exact serialization format is implementation-defined.

---

## § 82 CompilationPlan dependence

**§ 82(1)** Most data-race reasoning is source-semantic rather than inherently target
specific.

**§ 82(2)** CompilationPlan dependence is introduced only when behavior actually differs,
for example through:

```text
available atomic behavior
ISR/preemption rules
task/runtime execution semantics
foreign/runtime contracts
memory-space semantics
```

**§ 82(3)** The compiler should not duplicate target-independent summaries for every plan
without reason.

---

## § 83 Determinism

**§ 83(1)** For the same:

```text
source
CompilationPlan
analysis configuration
compiler version
```

**§ 83(2)** data race analysis must produce deterministic classification and deterministic
diagnostic grouping.

**§ 83(3)** Results must not depend on:

```text
source declaration order
hash-map iteration order
worklist scheduling
parallel compiler scheduling
cache fill order
```

**§ 83(4)** Representative access pairs and cause paths must be selected deterministically.

---

## § 84 `sec analyse`

**§ 84(1)** The default:

```text
sec analyse
```

**§ 84(2)** includes data race analysis as part of all-analysis behavior.

**§ 84(3)** Tooling must also permit explicit selection of data race analysis without
requiring every unrelated optional analysis.

**§ 84(4)** The selection is spelled `sec analyse data-races` (`rules/tooling/cli.md`
§ 4.2, MD-017).

**§ 84(5)** Deep data-race output may include:

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

## § 85 Required baseline capabilities for Sec 0.1

**§ 85(1)** The required Sec 0.1 implementation supports at least:

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

**§ 85(2)** Advanced context sensitivity is optional deeper precision unless required to
discharge an active safety contract.

---

## § 86 Required test philosophy

**§ 86(1)** Race analysis requires both positive detection tests and strong false-positive
regression coverage.

**§ 86(2)** A compiler that detects obvious races but rejects safe partitioned or
synchronized programs is not conforming to this rulebook.

**§ 86(3)** Tests should verify both analysis facts and diagnostic classification.

---

## § 87 Required proven-race tests

**§ 87(1)** The test suite must include at least:

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

## § 88 Required race-free tests

**§ 88(1)** The test suite must include at least:

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

## § 89 Required Unknown tests

**§ 89(1)** The test suite must include at least:

```text
lost raw-pointer provenance
unknown callable target behavior
foreign retention with unspecified concurrency
unknown ISR preemption relationship
unknown alias between shared views
precision widening
```

**§ 89(2)** These cases must not be promoted to `ProvenRace` merely because precision is
missing.

---

## § 90 Failure-to-prove tests

**§ 90(1)** The same `Unknown` analysis fact must be testable under both:

```text
a context where positive race-freedom proof is not required;
a context where an explicit rule requires positive race-freedom proof.
```

**§ 90(2)** The first remains an analysis result.

**§ 90(3)** The second may be a compile-time failure-to-prove diagnostic.

**§ 90(4)** Neither case becomes a `ProvenRace` without a race witness.

---

## § 91 Lock and synchronization tests

**§ 91(1)** Required cases include:

```text
same lock definitely held
lock only held on one branch
different locks
nested locks protecting an access
synchronization wrapper acquires lock
guard/token passed interprocedurally
lock released before access
```

**§ 91(2)** An access after lock release must not be treated as protected by the old lock
state.

---

## § 92 Publication and completion tests

**§ 92(1)** Required cases include:

```text
initialize -> publish -> read
publish -> later unsynchronized mutation/read
spawn -> write -> join -> read
spawn -> write -> read before join
multiple publication relationships
```

**§ 92(2)** The suite must verify that publication does not incorrectly protect unrelated
later mutations.

---

## § 93 Place/provenance tests

**§ 93(1)** Required cases include:

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

## § 94 Ownership and capture tests

**§ 94(1)** Required cases include:

```text
move to worker removes old legal access
illegal post-move access rejected by ownership before race analysis
shared borrow across execution contexts where permitted
exclusive borrow
captured reference in spawned closure
owned closure capture
```

**§ 94(2)** Data race analysis must not duplicate ownership or lifetime diagnostics.

---

## § 95 Callable and closure tests

**§ 95(1)** Required cases include:

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

## § 96 FFI tests

**§ 96(1)** When canonical foreign contracts are available, required cases include:

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

**§ 96(2)** The test suite must verify that parameter/function names alone never establish
foreign concurrency facts.

---

## § 97 ISR tests

**§ 97(1)** Required cases include:

```text
ISR and thread access the same ordinary storage
ISR atomic access according to memory-model rules
interrupt-masked critical section
non-overlapping ISR priority relationship
nested ISR possibility
unknown platform nesting
```

**§ 97(2)** These tests provide reusable race facts for later ISR analysis.

---

## § 98 Incremental LSP tests

**§ 98(1)** Required cases include:

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

## § 99 Candidate-generation and performance tests

**§ 99(1)** Large regression cases must cover:

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

**§ 99(2)** The implementation must demonstrate:

```text
bounded candidate generation
terminating fixed-point analysis
incremental invalidation
diagnostic grouping
deterministic results
```

**§ 99(3)** No normative wall-clock performance threshold is required.

---

## § 100 False-positive regression corpus

**§ 100(1)** The project should retain representative safe concurrent patterns that are easy
for a coarse analysis to misclassify.

**§ 100(2)** Important classes include:

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

**§ 100(3)** When a false positive is found in a conforming safe idiom, it should become a
regression case where the required analysis precision supports the proof.

---

## § 101 Completion criteria for Sec 0.1

**§ 101(1)** Data race analysis is complete for Sec 0.1 when all of the following are true:

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

**§ 101(2)** Advanced whole-program context sensitivity may improve precision after Sec 0.1,
but is not required unless an active safety contract requires that additional
proof power.

---

## § 102 Implementation governance

**§ 102(1)** This rulebook contains normative analysis behavior only.

**§ 102(2)** Mutable implementation information such as:

```text
current implementation status
source code locations
remaining implementation work
verification commands
known temporary gaps
```

**§ 102(3)** belongs in the repository-level:

```text
implementation-status.yaml
```

**§ 102(4)** The canonical implementation-status integration identifier for this analysis is:

```text
sema.data-race-analysis
```

**§ 102(5)** The ledger must describe observable repository state rather than aspirational
completion.

---

## § 103 Normative summary

**§ 103(1)** Data race analysis verifies memory access against the canonical Sec concurrency
memory model rather than defining a second memory model.

**§ 103(2)** The analysis correlates canonical Place/provenance, memory access kinds,
execution contexts, ownership transfer, publication, synchronization,
happens-before, callable behavior, foreign contracts, and platform preemption.

**§ 103(3)** A proven race requires a sound reachable unsafe witness. Lost precision produces
localized `Unknown`, not a fabricated race.

**§ 103(4)** Positive race-freedom proofs are scoped compiler facts and may be established
through disjoint storage, exclusive ownership, non-overlapping execution,
read-only sharing, mutual exclusion, happens-before ordering, or canonical
atomic semantics.

**§ 103(5)** Interprocedural analysis uses symbolic may-access and must-synchronization
summaries, instantiates them at call sites, and solves recursive call-graph SCCs
by deterministic fixed point.

**§ 103(6)** Candidate pairing is bounded through storage, projection, context, access, and
synchronization indexing rather than naïve global pairwise event comparison.

**§ 103(7)** LSP analysis uses progressive refinement within `Interactive`, `Standard`, and
`Deep`; no additional global `Intermediate` mode exists. `Pending` remains
distinct from `Unknown` and all proven results.

**§ 103(8)** Proven races and required race-freedom failures are NeedToKnow diagnostics.
Positive proof explanations and detailed concurrency relationships are
configurable OptionalInsight.

**§ 103(9)** `sec analyse` includes data race analysis by default as part of all-analysis
behavior and may also run it explicitly.

**§ 103(10)** Separate compilation uses versioned symbolic summaries and contracts.
Implementation progress remains outside this rulebook in
`implementation-status.yaml`.

**§ 103(11)** The analysis consumes the concurrency memory model's canonical per-atomic
identity, atomic/ordinary classification, program order, synchronizes-with and
happens-before edges, modification/RMW outcome, release sequences, fences,
task/thread completion publication, and FFI/platform synchronization contracts.
It does not define a second `MemoryOrder`. Mutex proofs likewise consume
canonical acquire/release edges, protected Place access, guard lifetime, and
alias legality from their owning rules.
