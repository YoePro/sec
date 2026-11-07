# Call Graph

- **Status:** Normative
- **Created:** 2026-08-08
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 2
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/call_graph.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`

---

This document is the canonical call-graph rulebook for Sec 0.1.

It defines:

- the semantic meaning of the Sec call graph;
- callable node identity;
- call-site identity;
- dispatch kinds;
- execution relationships;
- direct, indirect, interface, closure, foreign, intrinsic, generated, deferred,
  destruction, task, thread, process, interrupt, and callback relationships;
- roots and reachability;
- per-`CompilationPlan` graph construction;
- open-world library analysis;
- recursion and strongly connected components;
- task-spawn and thread-spawn cycles;
- `spawn`, `await`, and `join` integration;
- conservative callable contracts;
- effect, stack, allocation, ISR, panic, unsafe, reference, escape, and
  ownership analysis integration;
- diagnostics and LSP behavior;
- separate compilation and incremental invalidation;
- compile-time-only implementation without a mandatory runtime.

The call graph is a compiler analysis structure.

It does not introduce source-level reflection, runtime call-graph metadata,
dynamic registration, or a mandatory runtime.

---

## § 1 Purpose

**§ 1(1)** The call graph answers questions such as:

```text
Which callable units may execute?
Which call sites may reach a function?
Which concrete targets may an indirect call invoke?
Which open callable contract applies when the concrete target is unknown?
Which operations run on the current stack?
Which operations begin a new task, thread, or process?
Where does task execution synchronize with ordinary control flow?
Which functions form a recursion cycle?
Which spawn relationships may create unbounded concurrent work?
Which interrupt roots can reach a blocking or allocating operation?
Which public functions depend on unsafe or foreign trust?
Which functions can allocate after initialization?
Which path introduces a panic, effect, stack contribution, or reference escape?
Which graph facts changed after an incremental edit?
```

**§ 1(2)** The graph is shared infrastructure for multiple analyses.

**§ 1(3)** It must not become a collection of unrelated, partially inconsistent graphs.

---

## § 2 Core principle

```text
The Sec call graph is a sound compile-time representation of every reachable
callable execution relationship for one concrete CompilationPlan.
```

**§ 2(1)** Sound means:

```text
a possible callable relationship must not be omitted;
a target set may conservatively contain targets that do not execute at runtime;
an unknown concrete target may be represented by a valid callable contract;
an unknown callable contract may not be silently accepted.
```

**§ 2(2)** The graph is primarily a may-execute graph.

**§ 2(3)** It records execution that may occur on at least one semantically reachable path.

---

## § 3 One canonical graph, multiple analysis views

**§ 3(1)** Sec builds one canonical semantic graph per concrete `CompilationPlan`.

**§ 3(2)** Analyses derive specialized views from that graph.

**§ 3(3)** Examples:

```text
complete execution reachability;
same-stack call view;
task-spawn view;
thread-start view;
process-launch view;
interrupt call closure;
effect-propagation view;
allocation-reachability view;
panic-reachability view;
unsafe-trust view;
reference-retention view;
stack-analysis view;
recursion view.
```

**§ 3(4)** The compiler must not build independent graphs that disagree about:

```text
which call sites exist;
which targets are possible;
which target variant is active;
whether an edge crosses an execution-context boundary;
whether the relationship is implicit or explicit;
whether the relationship is trusted or proven.
```

---

## § 4 Non-goals

**§ 4(1)** This rulebook does not define:

```text
function declaration syntax;
lambda syntax;
interface syntax;
generic syntax;
spawn syntax;
await syntax;
join syntax;
task outcome syntax;
thread result syntax;
process result syntax;
panic payload syntax;
effect-constraint syntax;
callback-retention source syntax;
stack-bound source syntax;
foreign effect-declaration syntax.
```

**§ 4(2)** Those are defined by their owning rulebooks.

**§ 4(3)** This rulebook consumes their semantic results.

**§ 4(4)** The call graph does not replace:

```text
control-flow graphs;
data-flow analysis;
ownership analysis;
borrow checking;
reference validity analysis;
escape analysis;
effect analysis;
stack analysis;
allocation analysis;
recursion policy;
ISR analysis;
task lifecycle analysis;
thread analysis;
process analysis;
linking.
```

---

## § 5 Terminology

### § 5.1 Callable unit

**§ 5.1(1)** A callable unit is a semantic execution body that may be entered through a call,
spawn, callback, interrupt, foreign entry, generated invocation, deferred
execution, destruction, or another compiler-known relationship.

### § 5.2 Callable declaration

**§ 5.2(1)** A source or compiler declaration that describes callable behavior.

**§ 5.2(2)** A callable declaration is not always a concrete graph node.

**§ 5.2(3)** A generic template is a declaration but not a concrete executable
specialization.

### § 5.3 Callable node

**§ 5.3(1)** A concrete semantic graph node representing one callable execution unit for one
active compilation plan and specialization.

### § 5.4 Call site

**§ 5.4(1)** A semantic source or generated location that invokes or schedules a callable.

**§ 5.4(2)** A call site is distinct from the target callable node.

### § 5.5 Target set

**§ 5.5(1)** The sound set of concrete callable nodes that a call site may invoke.

### § 5.6 Callable contract

**§ 5.6(1)** The semantic contract used when the complete concrete target set is not known.

### § 5.7 Dispatch kind

**§ 5.7(1)** How the callable target is selected.

### § 5.8 Execution relation

**§ 5.8(1)** How execution of the target relates to the caller's current execution context.

### § 5.9 Root

**§ 5.9(1)** A graph entry from which reachability analysis begins.

### § 5.10 Open-world root

**§ 5.10(1)** An externally callable library declaration that may be invoked by code not
present in the current compilation unit.

### § 5.11 Same-stack relationship

**§ 5.11(1)** A relationship where the target executes as part of the current synchronous
stack path.

### § 5.12 Execution-context boundary

**§ 5.12(1)** A relationship that starts or enters another task, thread, process, interrupt,
or continuation context.

---

## § 6 Graph layers

**§ 6(1)** The canonical graph contains cooperating relationship layers.

### § 6.1 Callable reachability layer

**§ 6.1(1)** Represents which callable bodies may execute.

### § 6.2 Call-site layer

**§ 6.2(1)** Preserves source location, dispatch, target set, contracts, and analysis
provenance.

### § 6.3 Execution-context layer

**§ 6.3(1)** Distinguishes:

```text
same-stack execution;
new task execution;
new thread execution;
new process execution;
interrupt execution;
resumed continuation execution.
```

### § 6.4 Synchronization layer

**§ 6.4(1)** Represents operations such as:

```text
await;
join;
task completion observation;
thread join;
process wait where defined.
```

**§ 6.4(2)** A synchronization relation is not necessarily a call edge to the original
callable target.

### § 6.5 Root layer

**§ 6.5(1)** Represents executable, interrupt, foreign, test, generated, and open-world
entrypoints.

**§ 6.5(2)** These layers may use one implementation structure or cooperating structures.

**§ 6.5(3)** They must share stable node and site identities.

---

## § 7 Callable node kinds

**§ 7(1)** The initial callable node kinds are:

```text
SecFunction
SecMethod
GenericSpecialization
ClosureBody
LambdaBody
DeferBody
Destructor
ForeignFunction
ForeignCallbackEntry
CompilerIntrinsic
CompilerGeneratedHelper
TaskEntry
ThreadEntry
ProcessEntryContract
InterruptEntry
TargetStartupEntry
TestEntry
UnknownCallableContract
PanicHandler
```

**§ 7(2)** An implementation may use a smaller internal enum when several kinds share one
representation.

**§ 7(3)** The semantic distinctions must remain available.

---

## § 8 Ordinary functions

**§ 8(1)** A normal function declaration produces a callable node when it is active and
concrete for the current compilation plan.

```sec
fn Parse(input: ref rune[]) Result[Node, ParseError] {
    // ...
}
```

**§ 8(2)** The callable node records at least:

```text
semantic declaration identity;
active target/configuration variant;
parameter and return contract;
unsafe status;
declared guarantees;
direct Semantic IR body;
source location.
```

---

## § 9 Methods

**§ 9(1)** A method is a callable node.

```sec
impl Parser {
    fn Next() Token {
        // ...
    }
}
```

**§ 9(2)** The implicit `self` receiver is represented through callable and call-site
metadata.

**§ 9(3)** The node identity must distinguish:

```text
receiver type;
implementation identity;
generic receiver substitutions;
active target variant.
```

---

## § 10 Generic declarations

**§ 10(1)** A generic declaration is a callable template.

```sec
fn Convert[T](value: T) string {
    // ...
}
```

**§ 10(2)** The unspecialized template is not a concrete executable call-graph node.

**§ 10(3)** Concrete semantic specializations are nodes:

```text
Convert[int]
Convert[Customer]
```

**§ 10(4)** A compiler may retain a template-summary object for analysis.

**§ 10(5)** That template-summary object is not treated as an executable node unless the
runtime representation truly supports unspecialized generic dispatch.

---

## § 11 Generic specialization identity

**§ 11(1)** A specialization node identity includes:

```text
generic declaration identity;
type substitutions;
compile-time value substitutions;
receiver substitutions;
active implementation variant;
target/configuration identity.
```

**§ 11(2)** Backend code sharing does not merge semantic graph identity.

**§ 11(3)** Two specializations may lower to one machine implementation while retaining
different:

```text
effect facts;
copy/move behavior;
destruction behavior;
stack facts;
reference behavior;
contract proofs;
diagnostics.
```

---

## § 12 Closures and lambdas

**§ 12(1)** Every closure or lambda body receives a stable compiler identity.

```sec
let callback := fn(value: int) int {
    return value + 1
}
```

**§ 12(2)** The body is a callable node.

**§ 12(3)** The capture environment is not a callable node.

**§ 12(4)** Capture creation, allocation, moves, borrows, retention, destruction, and
escape are represented through Semantic IR and cooperating analyses.

---

## § 13 Local callable identity

**§ 13(1)** A local callable identity must remain stable across unrelated edits where
possible.

**§ 13(2)** A recommended identity includes:

```text
containing declaration identity;
source syntax identity;
semantic ordinal within the containing declaration;
capture signature;
active specialization.
```

**§ 13(3)** A raw parser object address or unstable build-order counter is not a stable
identity.

---

## § 14 `defer` bodies

**§ 14(1)** Every `defer` body receives a synthetic callable node.

**§ 14(2)** This applies even when the body contains no explicit call.

```sec
defer {
    state.completed = true
}
```

**§ 14(3)** Conceptually:

```text
FunctionNode
    -> DeferredExecution
        -> DeferBodyNode
```

**§ 14(4)** Reasons:

```text
stable diagnostics;
cleanup-path effect analysis;
destruction analysis;
panic analysis;
incremental identity;
LSP navigation;
consistent Semantic IR;
future body changes without node-kind changes.
```

**§ 14(5)** The `defer` body remains subject to all canonical defer and cleanup rules.

---

## § 15 Destructors

**§ 15(1)** A destructor is a callable node.

**§ 15(2)** Implicit destructor invocation is represented through a destruction
relationship.

**§ 15(3)** Destructor relationships must remain visible to:

```text
effect analysis;
panic analysis;
stack analysis;
recursion analysis;
allocation analysis;
unsafe provenance;
diagnostics.
```

**§ 15(4)** The graph must not omit a destructor because the invocation is compiler
generated.

---

## § 16 Foreign functions

**§ 16(1)** A foreign function declaration is a callable node or callable contract node.

```sec
unsafe extern "system" fn rawSysCall(...) int64
```

**§ 16(2)** The node records:

```text
linkage;
ABI;
signature;
unsafe status;
declared or conservative effects;
ownership contract;
callback contract;
trust provenance;
source location.
```

**§ 16(3)** The implementation body is not proven from Sec source.

---

## § 17 Compiler intrinsics

**§ 17(1)** A compiler intrinsic may be represented as:

```text
a callable node with a compiler-known summary;
or
an explicit Semantic IR operation without a callable node.
```

**§ 17(2)** An intrinsic may be omitted as a graph node only when it is semantically atomic
for every analysis that consumes the call graph.

**§ 17(3)** Its summary must still expose all relevant:

```text
effects;
stack behavior;
allocation behavior;
panic behavior;
reference behavior;
volatile behavior;
control-flow behavior;
trust provenance.
```

---

## § 18 Compiler-generated helpers

**§ 18(1)** A compiler-generated helper is a visible callable node when its internal
behavior matters to any analysis.

**§ 18(2)** Examples:

```text
closure adapter;
foreign ABI adapter;
task entry adapter;
thread entry adapter;
destruction helper;
collection-growth helper;
copy or move helper;
generic dispatch helper.
```

**§ 18(3)** A helper may use an intrinsic summary only when treating it as atomic preserves
all language semantics and diagnostics.

**§ 18(4)** The compiler must not hide a meaningful call chain merely because the helper is
generated.

---

## § 19 Callable node identity

**§ 19(1)** A node identity must not depend on:

```text
memory address of an AST node;
iteration order of a map;
temporary parser index;
backend symbol numbering;
link order.
```

**§ 19(2)** A conceptual node key includes:

```text
declaration identity;
node kind;
active CompilationPlan;
target/configuration variant;
generic substitutions;
receiver implementation;
closure/defer/destructor identity;
foreign linkage identity where relevant.
```

**§ 19(3)** Exact compiler representation is implementation-defined.

---

## § 20 Call-site identity

**§ 20(1)** Every callable invocation or scheduling site receives a stable identity.

**§ 20(2)** A call-site identity includes:

```text
containing callable node;
source or generated syntax identity;
source location;
semantic operation kind;
active specialization;
active CompilationPlan.
```

**§ 20(3)** The graph stores one semantic call-site record.

**§ 20(4)** It does not replace one indirect call site with unrelated anonymous edges and
discard the site identity.

---

## § 21 Call-site record

**§ 21(1)** A call-site record should contain at least:

```text
caller node;
source location;
dispatch kind;
execution relation;
target set;
open callable contract if any;
unsafe invocation requirement;
trust provenance;
argument and receiver Semantic IR references;
spawn or synchronization metadata where relevant;
diagnostic causes.
```

**§ 21(2)** Conceptual structure:

```go
type CallSite struct {
    ID                CallSiteID
    Caller            NodeID
    Dispatch          DispatchKind
    Execution         ExecutionRelation
    Targets           []NodeID
    OpenContract      *CallableContractID
    Source            source.Location
    Provenance        TrustProvenance
}
```

**§ 21(3)** This is illustrative.

**§ 21(4)** All implementation code is written in English.

---

## § 22 Dispatch kinds

**§ 22(1)** The initial dispatch kinds are:

```text
Direct
StaticMethod
Closure
FunctionValue
InterfaceDispatch
ForeignDirect
ForeignFunctionPointer
Intrinsic
CompilerGenerated
UnknownContract
```

**§ 22(2)** Dispatch kind answers:

```text
how is the callable selected?
```

**§ 22(3)** It does not answer:

```text
where or when does it execute?
```

---

## § 23 Execution relations

**§ 23(1)** The initial execution relations are:

```text
Synchronous
Deferred
Destruction
SpawnTask
SpawnThread
SpawnProcess
InterruptEntry
ForeignCallbackEntry
ContinuationResume
ImmediateCallback
RetainedCallback
AsynchronousCallback
```

**§ 23(2)** An implementation may use flags or a normalized relation structure.

**§ 23(3)** The distinctions must remain available to analysis.

---

## § 24 Dispatch and execution are independent

**§ 24(1)** A function value may be invoked synchronously:

```text
Dispatch:
    FunctionValue

Execution:
    Synchronous
```

**§ 24(2)** An interface method may be spawned:

```text
Dispatch:
    InterfaceDispatch

Execution:
    SpawnTask
```

**§ 24(3)** A foreign callback may enter from outside the program:

```text
Dispatch:
    ForeignFunctionPointer

Execution:
    ForeignCallbackEntry
```

**§ 24(4)** The compiler must not encode all combinations as unrelated special-case enums
that lose the shared semantics.

---

## § 25 Same-stack execution

**§ 25(1)** The following normally execute on the current stack:

```text
Synchronous;
ImmediateCallback;
Deferred;
Destruction.
```

**§ 25(2)** They participate in same-stack:

```text
reachability;
effect propagation;
stack accumulation;
recursion SCCs;
panic chains;
allocation chains.
```

**§ 25(3)** A target-specific lowering may inline the call.

**§ 25(4)** Inlining does not remove the semantic relationship from analysis.

---

## § 26 New task execution

**§ 26(1)** `spawn expression` and `spawn task expression` start new task execution.

**§ 26(2)** They are equivalent at the language level.

```sec
let worker := try spawn Work()
let explicitWorker := try spawn task Work()
```

**§ 26(3)** The spawned function remains an ordinary synchronous callable declaration.

**§ 26(4)** `spawn` creates the new execution entity.

**§ 26(5)** The same callable may be:

```text
called synchronously;
spawned as a task;
spawned through another supported execution kind.
```

---

## § 27 Task spawn relationship

**§ 27(1)** A task spawn call site records:

```text
callable target or callable contract;
argument evaluation;
receiver evaluation;
copied values;
moved values;
retained shared borrows;
retained mutable borrows;
captures;
parent task context;
resulting Task[T] type;
source location;
target task profile.
```

**§ 27(2)** The spawn target becomes the entry node of a new task execution context.

**§ 27(3)** The target remains reachable from the spawning root.

**§ 27(4)** It does not run on top of the spawner's current stack.

---

## § 28 Eager task semantics

**§ 28(1)** Task spawning is eager.

**§ 28(2)** After the spawn expression completes, the task is:

```text
scheduled;
running;
or already completed.
```

**§ 28(3)** The graph must not model the task body as a lazy callable that becomes reachable
only at `await`.

**§ 28(4)** Reachability begins at the spawn site.

**§ 28(5)** `await` synchronizes with an already-created task.

---

## § 29 Task parenthood

**§ 29(1)** A task spawn relationship records the current task as the parent task context.

**§ 29(2)** Parenthood does not replace ownership of the `Task[T]` handle.

**§ 29(3)** The call graph records execution parenthood.

**§ 29(4)** Task lifecycle analysis records handle ownership.

---

## § 30 Spawn effects

**§ 30(1)** The spawner receives effects from task creation and scheduling.

**§ 30(2)** Examples may include:

```text
MaySpawn;
MayAllocate;
task-creation failure;
scheduler interaction.
```

**§ 30(3)** The spawned body's effects belong to the new task execution context.

**§ 30(4)** They remain part of whole-program reachability.

**§ 30(5)** They do not automatically become synchronous effects of the spawner.

---

## § 31 New thread execution

```sec
let worker := try spawn thread Work()
```

**§ 31(1)** creates a `Thread[T]` execution context according to the canonical thread rules.

**§ 31(2)** A `SpawnThread` relationship:

```text
starts a separate thread execution context;
does not extend the caller's current stack;
retains ownership, transfer, and reference metadata;
requires target support;
must not silently lower to an ordinary task.
```

**§ 31(3)** The thread entry target remains reachable from the spawning root.

---

## § 32 New process execution

```sec
let child := try spawn process Program()
```

**§ 32(1)** creates or launches a process according to the process rulebook.

**§ 32(2)** A process launch crosses an address-space and process boundary.

**§ 32(3)** The call graph records:

```text
launching callable;
process-launch site;
known process entry contract or program identity;
argument and ownership transfer contract;
process result-handle contract;
trust and target provenance.
```

**§ 32(4)** The child process may have a separate executable call graph.

**§ 32(5)** The parent graph must not treat child process functions as same-address-space
callees.

---

## § 33 `await`

**§ 33(1)** `await` is task-specific in Sec 0.1.

```sec
let outcome := await task
```

**§ 33(2)** `await`:

```text
requires Task[T];
waits for completion;
consumes the owning Task[T] handle;
resolves the task outcome;
returns execution to ordinary synchronous control flow;
may establish the end of task-retained borrows;
may suspend or block according to the selected backend while preserving the
language semantics.
```

---

## § 34 Await is not a call edge to the task body

**§ 34(1)** The task body became reachable at `spawn`.

**§ 34(2)** `await task` does not call the task body again.

**§ 34(3)** The graph represents `await` as a synchronization relation connected to:

```text
the awaiting callable;
the Task[T] value;
its origin spawn site or transferred task origin where known;
the continuation after completion;
the `Completed(T)`, `Cancelled`, `Panicked(PanicInfo)`, and
`Failed(TaskError)` outcome paths;
the borrow-release boundary.
```

---

## § 35 Await continuation

**§ 35(1)** Semantic IR records the continuation after task completion.

**§ 35(2)** The call graph or cooperating execution graph records:

```text
AwaitTask synchronization;
ContinuationResume relation;
continuation ownership state;
live borrows across suspension;
cleanup and cancellation paths.
```

**§ 35(3)** A continuation may remain compiler-internal.

**§ 35(4)** It receives a stable identity when represented as a separate callable node.

---

## § 36 Direct await

```sec
let outcome := await (try spawn Calculate())
```

**§ 36(1)** has the same semantic execution relationships as:

```sec
let task := try spawn Calculate()
let outcome := await task
```

**§ 36(2)** The compiler may optimize the physical implementation.

**§ 36(3)** The graph preserves:

```text
task creation;
task execution context;
await synchronization;
completion continuation.
```

---

## § 37 `join`

**§ 37(1)** `join task` waits for completion while preserving the owning task handle.

**§ 37(2)** The graph represents `join` as a synchronization relation.

**§ 37(3)** It does not call the task body again.

**§ 37(4)** The relationship records:

```text
task origin where known;
completion observation;
continuation after join;
memory synchronization;
result availability;
preserved handle ownership.
```

**§ 37(5)** `await` and `join` remain distinct.

---

## § 38 Await and same-stack recursion

**§ 38(1)** `await` is not a same-stack call edge to the spawned callable.

**§ 38(2)** A cycle involving:

```text
A spawns B;
B spawns A;
```

**§ 38(3)** is a spawn cycle.

**§ 38(4)** It is not ordinary same-stack recursion.

**§ 38(5)** A continuation may later call into an existing recursion component, but that
relationship is represented by its actual synchronous call sites.

---

## § 39 Spawn cycles

**§ 39(1)** Spawn cycles are analyzed separately from stack recursion.

**§ 39(2)** Examples:

```text
self-spawn;
mutual task spawn;
recursive detached spawn;
exponential child-task creation.
```

**§ 39(3)** A spawn cycle may be legal only when:

```text
the selected profile permits it;
task creation is bounded;
ownership of every task handle is resolved;
resource use is bounded or accepted by profile policy.
```

**§ 39(4)** Statically evident unbounded creation is diagnosed.

---

## § 40 Thread-start cycles

**§ 40(1)** Thread-start cycles are analyzed separately from task-spawn cycles and
same-stack recursion.

**§ 40(2)** They may indicate:

```text
unbounded thread creation;
resource exhaustion;
profile violation;
shutdown or supervision problems.
```

---

## § 41 Process-launch cycles

**§ 41(1)** Process-launch cycles are not same-stack recursion.

**§ 41(2)** They may indicate:

```text
unbounded process creation;
fork-like explosion;
resource exhaustion;
supervision cycles.
```

**§ 41(3)** Their legality is governed by process and profile rules.

---

## § 42 Direct calls

**§ 42(1)** A statically resolved call has one concrete target.

```sec
let node := Parse(input)
```

**§ 42(2)** Conceptually:

```text
Dispatch:
    Direct

Execution:
    Synchronous

Targets:
    Parse
```

**§ 42(3)** Static method calls use the resolved implementation target.

---

## § 43 Function values

**§ 43(1)** A function-value call may have:

```text
one exact target;
a closed finite target set;
an open callable contract;
known targets plus an open contract.
```

**§ 43(2)** The compiler uses value-flow analysis to derive a sound target set.

**§ 43(3)** Example:

```sec
let callback := SelectCallback()
let value := callback(input)
```

**§ 43(4)** Possible targets might be:

```text
Normalize;
Validate;
Reject.
```

**§ 43(5)** The target set must be a safe over-approximation.

---

## § 44 Interfaces

**§ 44(1)** Interface dispatch uses the same target-set model.

**§ 44(2)** For a closed executable build, the compiler may derive a finite set of concrete
implementations.

**§ 44(3)** For an open library boundary, future implementations may exist.

**§ 44(4)** The call site then retains:

```text
known concrete targets;
the open interface callable contract.
```

**§ 44(5)** Analyses may use the declared interface guarantees.

**§ 44(6)** They must not use accidental stronger properties of only the currently known
implementations as public guarantees.

---

## § 45 Unknown concrete target

**§ 45(1)** An unknown concrete target is permitted when a valid callable contract exists.

**§ 45(2)** Example:

```text
target identity:
    unknown

callable contract:
    known
```

**§ 45(3)** The call remains type-correct and analyzable according to the contract.

**§ 45(4)** The compiler must not require every dynamic target to be statically enumerated
in ordinary hosted code.

---

## § 46 Unknown callable contract

**§ 46(1)** A call without a valid callable contract is invalid.

**§ 46(2)** Examples include attempting to invoke a raw address without established:

```text
signature;
ABI;
return representation;
argument ownership;
safe or unsafe status;
effect contract.
```

**§ 46(3)** Canonical rule:

```text
An unresolved target identity is permitted.

An unresolved callable contract is a compile error.
```

---

## § 47 Callable contract

**§ 47(1)** A callable contract includes enough information to validate invocation and
perform conservative analysis.

**§ 47(2)** It may contain:

```text
parameter types;
return type;
receiver contract;
calling convention;
safe or unsafe invocation;
ownership and borrow modes;
effect guarantees or upper bounds;
callback retention behavior;
reentry behavior;
stack bound where declared;
allocation behavior where declared;
trust provenance.
```

**§ 47(3)** Exact source syntax for every optional contract property may be defined by other
rulebooks.

**§ 47(4)** The semantic model is fixed here.

---

## § 48 Closed target set

**§ 48(1)** A closed target set contains every possible concrete target.

**§ 48(2)** Analyses combine facts conservatively.

**§ 48(3)** Examples:

```text
effects:
    union of target effects

same-stack maximum:
    maximum target contribution

recursion:
    include every possible same-stack edge

unsafe provenance:
    union of trusted dependencies
```

**§ 48(4)** A closed set does not require a normal build diagnostic.

**§ 48(5)** LSP may display the target set.

---

## § 49 Open callable contract

**§ 49(1)** An open callable contract permits targets not present in the current build.

**§ 49(2)** The graph records:

```text
known concrete targets;
open contract identity;
contract guarantees;
open-world provenance.
```

**§ 49(3)** Analysis uses the contract as the public upper bound.

---

## § 50 Conservative unknown facts

**§ 50(1)** When a callable contract omits an optional guarantee, the corresponding
property is conservative.

**§ 50(2)** Examples:

```text
no @noPanic guarantee:
    panic cannot be excluded

no @noAlloc guarantee:
    allocation cannot be excluded

no @noBlock guarantee:
    blocking cannot be excluded

no stack bound:
    static maximum cannot be established from the contract

no no-reentry guarantee:
    reentry cannot be excluded
```

**§ 50(3)** Conservative does not mean the behavior definitely occurs.

**§ 50(4)** It means the behavior has not been excluded.

---

## § 51 Mentor diagnostic policy

**§ 51(1)** Indirect-call uncertainty is diagnosed according to consequence.

### § 51.1 No diagnostic

**§ 51.1(1)** No diagnostic is required when:

```text
the concrete target is unknown;
the callable contract is valid;
the contract is sufficient for all active requirements.
```

**§ 51.1(2)** Ordinary callback use must not produce constant noise.

### § 51.2 Information

**§ 51.2(1)** Information is appropriate when:

```text
conservative analysis was used;
the program remains valid;
no declared guarantee or strict profile requirement is violated;
the information is useful for understanding analysis.
```

**§ 51.2(2)** Information is normally shown through:

```text
LSP hover;
analysis reports;
verbose compiler mode;
configurable informational diagnostics.
```

### § 51.3 Warning

**§ 51.3(1)** Warning is appropriate when:

```text
the program remains accepted;
an important bound or proof is unavailable;
the uncertainty may surprise the programmer;
the selected profile permits the uncertainty.
```

**§ 51.3(2)** Examples:

```text
unknown maximum stack contribution in a permissive hosted profile;
open callback contract permits allocation although current targets do not;
unknown reentry behavior where recursion is allowed but should be reviewed.
```

### § 51.4 Error

**§ 51.4(1)** Error is required when:

```text
no valid callable contract exists;
a declared guarantee cannot be proven;
a strict target/profile requirement cannot be proven;
ISR safety cannot be proven;
required stack bounds cannot be established;
recursion freedom is required but reentry cannot be excluded.
```

---

## § 52 Configurable diagnostic severity

**§ 52(1)** Informational and warning diagnostics use stable diagnostic IDs.

**§ 52(2)** They may be:

```text
promoted;
demoted;
disabled;
reported in analysis-only mode.
```

**§ 52(3)** A semantic error required for soundness cannot be disabled.

---

## § 53 Mentor diagnostic content

**§ 53(1)** A call-graph diagnostic must state:

```text
what is unknown;
why it matters;
which analysis is affected;
which conservative assumption was used;
which call site introduced the uncertainty;
which concrete targets are known;
which callable contract applies;
how the programmer can strengthen the proof.
```

---

## § 54 Indirect-call information example

```text
information: indirect call target is not statically closed

call site:
    handler(packet)

known targets:
    DecodeFast
    DecodeStrict

analysis also uses open callable contract:
    Handler

conservative properties:
    may panic
    may allocate
```

---

## § 55 Stack warning example

```text
warning: maximum stack use cannot be determined across indirect call

Process
  -> invokes callback `handler`
  -> callable contract has no maximum stack contribution

the selected hosted profile permits an unknown dynamic stack contribution

help:
    declare a stack bound for the callback contract
    restrict the callback to a closed target set
    or select a profile that permits dynamic stack growth
```

---

## § 56 Guarantee error example

```text
error: `Process` does not satisfy @noPanic

Process
  -> invokes callback `handler`
  -> callable contract does not guarantee @noPanic

help:
    require @noPanic in the callable contract
    or remove @noPanic from Process
```

---

## § 57 ISR error example

```text
error: interrupt safety cannot be proven across indirect call

TimerHandler
  -> invokes callback `handler`

required:
    @interruptSafe

callable contract:
    no interrupt-safety guarantee
```

---

## § 58 Roots

**§ 58(1)** Reachability begins from graph roots.

**§ 58(2)** The initial root kinds are:

```text
ProgramEntryRoot
TargetStartupRoot
InitializationRoot
TestRoot
BenchmarkRoot
InterruptRoot
ForeignExportRoot
GeneratedRuntimeRoot
OpenWorldRoot
```

---

## § 59 Program entry roots

**§ 59(1)** A normal executable includes its configured program entrypoint.

**§ 59(2)** Target startup code may introduce a separate startup root that eventually
reaches the language entrypoint.

**§ 59(3)** The exact entrypoint syntax and ABI belong to project, target, and ABI
rulebooks.

---

## § 60 Initialization roots

**§ 60(1)** Runtime initialization may use explicit initialization roots.

**§ 60(2)** Compile-time evaluation is not represented as ordinary runtime reachability.

**§ 60(3)** Hidden static initialization must not silently introduce task spawning or
awaiting.

---

## § 61 Test roots

**§ 61(1)** Each selected test entrypoint is a root for its test compilation plan.

**§ 61(2)** A test graph may include:

```text
test harness generated root;
test setup;
test body;
test cleanup;
selected runtime support.
```

---

## § 62 Interrupt roots

**§ 62(1)** An `@interrupt(...)` function is an interrupt execution root.

**§ 62(2)** An `@isr` function may be an externally or manually bound interrupt root
according to the interrupt rulebook.

**§ 62(3)** Interrupt reachability is analyzed separately from ordinary application entry
reachability.

**§ 62(4)** The interrupt root does not become an ordinary caller of `main`.

---

## § 63 Foreign export roots

**§ 63(1)** A Sec callable exported for foreign invocation is a foreign-entry root.

**§ 63(2)** The node records:

```text
ABI;
linkage;
foreign trust boundary;
panic restrictions;
thread attachment assumptions;
effect and ownership contract.
```

---

## § 64 Open-world library roots

**§ 64(1)** During a library build, every externally callable declaration is represented as
an `OpenWorldRoot`.

**§ 64(2)** This does not assume `pub` syntax.

**§ 64(3)** Sec visibility and external-callability follow the canonical `_` and `__`
scope/visibility rules and related module/linkage rules.

**§ 64(4)** An open-world root means:

```text
code outside the analyzed library may invoke this callable according to its
public callable contract.
```

**§ 64(5)** It is not an ordinary executable startup root.

---

## § 65 Root reachability classes

**§ 65(1)** A node may be reachable from multiple root classes.

**§ 65(2)** Examples:

```text
reachable from main;
reachable from interrupt;
reachable from foreign export;
reachable from open-world library root;
reachable from test only.
```

**§ 65(3)** The graph retains this information for diagnostics and binary stripping.

---

## § 66 Task entry roots

**§ 66(1)** A spawned task target becomes a derived task-entry root for its new execution
context.

**§ 66(2)** It remains connected to the spawning call site.

**§ 66(3)** It is not a global root when no reachable spawn site creates it.

---

## § 67 Thread entry roots

**§ 67(1)** A spawned thread target becomes a derived thread-entry root.

**§ 67(2)** Its stack and execution policies are analyzed separately from the spawning
stack.

---

## § 68 Process entry contracts

**§ 68(1)** A spawned process may refer to:

```text
another known Sec executable graph;
an external program contract;
a target-specific process entry.
```

**§ 68(2)** The parent graph records the launch relationship.

**§ 68(3)** A separate child graph is used when the child program is part of the build.

---

## § 69 Target and configuration selection

**§ 69(1)** The concrete call graph is built after:

```text
@target selection;
@when selection;
target profile selection;
device/board knowledge selection;
active implementation-variant selection;
generic specialization selection where required.
```

**§ 69(2)** Only active declarations and call sites contribute to the concrete graph.

---

## § 70 One graph per `CompilationPlan`

**§ 70(1)** A logical build target may produce multiple concrete outputs.

**§ 70(2)** Example:

```text
linux/amd64;
linux/arm64;
linux/arm32.
```

**§ 70(3)** Each output has its own:

```text
active declarations;
ABI;
target knowledge;
intrinsics;
foreign implementations;
call graph;
analysis summaries.
```

**§ 70(4)** Canonical rule:

```text
One concrete CompilationPlan produces one concrete semantic call graph.
```

---

## § 71 Cross-plan comparison

**§ 71(1)** The compiler may maintain an aggregate index across compilation plans.

**§ 71(2)** It is used for questions such as:

```text
does a declared guarantee hold on every configured target?
does one target introduce recursion?
does one target require allocation?
are public callable contracts compatible?
```

**§ 71(3)** The aggregate index is not a merged runtime graph.

---

## § 72 Inactive declarations

**§ 72(1)** An inactive declaration is not reachable in the current concrete graph.

**§ 72(2)** It may still be checked in another compilation plan.

**§ 72(3)** Example:

```sec
@target(os: "linux")
fn PlatformWrite(...) void {
}

@target(os: "windows")
fn PlatformWrite(...) void {
}
```

**§ 72(4)** The Linux graph contains only the Linux variant.

---

## § 73 Path sensitivity

**§ 73(1)** A call site contributes reachability only when its containing path remains
semantically reachable.

```sec
if false {
    DangerousOperation()
}
```

**§ 73(2)** does not add a reachable call relationship.

**§ 73(3)** A branch proven impossible through:

```text
constant evaluation;
type refinement;
contract refinement;
target selection;
configuration selection;
exhaustive match analysis;
```

**§ 73(4)** does not contribute reachable calls.

---

## § 74 Runtime conditions

**§ 74(1)** A runtime condition that may evaluate either way contributes call sites from all
reachable branches.

**§ 74(2)** The graph remains a may-execute graph.

**§ 74(3)** Must-execute counts and path frequency belong to control-flow and quantitative
analysis.

---

## § 75 Unreachable code

**§ 75(1)** Proven unreachable code is already a compiler error under Sec rules.

**§ 75(2)** The call graph must not use erroneous unreachable code to introduce false
reachable edges into later analysis.

**§ 75(3)** Diagnostics may still use the local code to explain the original unreachable
error.

---

## § 76 Recursion

**§ 76(1)** Same-stack recursion is detected through strongly connected components over
same-stack execution relations.

**§ 76(2)** Included relations normally include:

```text
Synchronous;
ImmediateCallback;
Deferred;
Destruction.
```

**§ 76(3)** Excluded from same-stack recursion include:

```text
SpawnTask;
SpawnThread;
SpawnProcess;
InterruptEntry;
AwaitTask synchronization;
Join synchronization.
```

---

## § 77 Direct recursion

```sec
fn Walk(node: ref Node) void {
    Walk(node)
}
```

**§ 77(1)** produces a self-cycle in the same-stack view.

---

## § 78 Indirect recursion

```text
A -> B -> C -> A
```

**§ 78(1)** produces one same-stack strongly connected component.

**§ 78(2)** The diagnostic reports the clearest cycle path.

---

## § 79 Recursion through function values

**§ 79(1)** A function-value target set may create a recursion edge.

**§ 79(2)** When a callable contract is open and reentry is not excluded, recursion freedom
cannot be proven solely from known targets.

---

## § 80 Reentry contracts

**§ 80(1)** A callable contract may eventually express or imply:

```text
MayReenter;
NoReentry;
ClosedTargetSet.
```

**§ 80(2)** Exact source syntax is defined elsewhere.

**§ 80(3)** The call graph stores the semantic fact.

**§ 80(4)** When recursion is disabled, unknown reentry is an error.

**§ 80(5)** When recursion is permitted, unknown reentry may produce information or warning
according to profile policy.

---

## § 81 Recursive `defer`

**§ 81(1)** A deferred body may create ordinary recursion.

```sec
fn Work() void {
    defer {
        Work()
    }
}
```

**§ 81(2)** The synthetic defer node participates in the same-stack SCC.

---

## § 82 Recursive destruction

**§ 82(1)** Destructor call chains participate in same-stack recursion analysis.

**§ 82(2)** Implicit destruction must not hide a recursive cycle.

---

## § 83 Complete execution SCC

**§ 83(1)** The compiler may compute an SCC over all execution relationships for analysis
and visualization.

**§ 83(2)** This complete SCC is not used as ordinary stack recursion proof.

**§ 83(3)** Spawn, thread, process, and interrupt cycles retain their execution-boundary
classification.

---

## § 84 Effect-analysis integration

**§ 84(1)** Each callable node has:

```text
direct effect summary;
transitive effect summary;
declared guarantees;
trust provenance.
```

**§ 84(2)** Effect propagation uses the call graph and execution relation.

---

## § 85 Same-stack effect transfer

**§ 85(1)** For:

```text
Synchronous;
ImmediateCallback;
Deferred;
Destruction;
```

**§ 85(2)** callee effects propagate into the caller's transitive summary.

---

## § 86 Spawn effect transfer

**§ 86(1)** For task, thread, and process spawn:

```text
creation and scheduling effects propagate to the spawner;
body effects belong to the new execution context;
body effects remain reachable in the whole program;
body effects do not automatically become synchronous caller effects.
```

---

## § 87 Await effect transfer

**§ 87(1)** `await` contributes its own semantic effects.

**§ 87(2)** These may include:

```text
MaySuspend;
MayBlock according to backend/profile;
task outcome handling;
cancellation point behavior.
```

**§ 87(3)** The spawned body's effects were already assigned to the task execution context.

---

## § 88 Interrupt effect closure

**§ 88(1)** ISR analysis traverses the interrupt root through all reachable same-context
callable relationships.

**§ 88(2)** It verifies:

```text
panic;
allocation;
blocking;
suspension;
spawn restrictions;
volatile access;
shared state;
stack;
unsafe trust;
unknown callable contracts.
```

---

## § 89 Stack-analysis integration

**§ 89(1)** The graph provides:

```text
same-stack call relationships;
execution-context boundaries;
recursion SCCs;
indirect target sets;
open stack contracts;
root-to-node paths.
```

**§ 89(2)** Node-local frame size comes from stack analysis and final target lowering.

---

## § 90 Same-stack stack accumulation

**§ 90(1)** For a same-stack call path, maximum stack conceptually combines:

```text
caller frame;
callee path;
cleanup path;
defer and destructor paths;
target ABI contribution.
```

**§ 90(2)** Indirect calls use the maximum known target or declared callable bound.

---

## § 91 Task and thread stacks

**§ 91(1)** `SpawnTask` and `SpawnThread` begin separate stack analyses.

**§ 91(2)** The caller's active stack is not naively added to the complete spawned stack.

**§ 91(3)** The graph still retains parent and creation relationships for diagnostics and
resource analysis.

---

## § 92 Unknown stack contribution

**§ 92(1)** A callable contract may declare a stack upper bound.

**§ 92(2)** When no concrete targets or bound exist:

```text
permissive hosted profile:
    information or warning may be emitted

strict embedded profile:
    compile error when a complete stack proof is required
```

---

## § 93 Allocation-analysis integration

**§ 93(1)** The graph supports:

```text
root-to-allocator paths;
allocation during initialization;
allocation after initialization;
allocation in ISR;
allocation in cleanup;
allocation in tasks or threads;
unknown allocation through open callable contracts.
```

**§ 93(2)** Spawn-body allocation remains reachable program behavior.

**§ 93(3)** It is not described as synchronous allocation by the spawner unless task
creation itself allocates.

---

## § 94 Panic-analysis integration

**§ 94(1)** Panic-capable Semantic IR operations contribute direct panic effects.

**§ 94(2)** A fictional panic node is not required.

**§ 94(3)** When the compilation plan uses a real configured panic handler callable, the
graph may contain:

```text
panic operation -> panic handler
```

**§ 94(4)** A target trap does not require a callable handler node.

---

## § 95 Unsafe-analysis integration

**§ 95(1)** Unsafe status is part of the callable contract.

**§ 95(2)** An unsafe callable target cannot silently convert to a safe callable contract.

**§ 95(3)** The graph preserves:

```text
unsafe invocation requirement;
trusted foreign declaration provenance;
inline assembly provenance;
compiler intrinsic provenance;
safe wrapper boundary.
```

**§ 95(4)** A safe wrapper is a safe callable node.

**§ 95(5)** Its internal unsafe edge remains visible for auditing and diagnostics.

---

## § 96 Reference-model integration

**§ 96(1)** The call graph does not duplicate `reference_model.md`.

**§ 96(2)** Call-site metadata links to Semantic IR facts for:

```text
borrowed arguments;
mutable borrows;
moves;
copies;
captures;
returned references;
task-retained borrows;
callback retention;
foreign pointer retention;
execution timing.
```

**§ 96(3)** Reference and escape analyses determine validity.

**§ 96(4)** The call graph determines which callable bodies may execute and in which
execution relationship.

---

## § 97 Spawned borrows

**§ 97(1)** A spawn call site records borrows retained by the spawned execution.

**§ 97(2)** The borrow remains active until task/thread completion or another proven
lifecycle boundary.

**§ 97(3)** `await` may establish the completion point where a task-retained borrow ends.

**§ 97(4)** The call graph exposes the relationship.

**§ 97(5)** Borrow analysis enforces validity.

---

## § 98 Callback retention

**§ 98(1)** Callable contracts may distinguish:

```text
invoked only during the current call;
may be retained;
may be invoked asynchronously;
may be invoked concurrently.
```

**§ 98(2)** The graph records the semantic callback relation.

**§ 98(3)** Reference, closure, ownership, and escape analyses consume the retention facts.

**§ 98(4)** A missing retention guarantee must not be interpreted as immediate invocation
when the API permits retention.

---

## § 99 Immediate callbacks

**§ 99(1)** An immediate callback invocation is same-stack execution.

**§ 99(2)** Its effects and stack contribution propagate normally.

---

## § 100 Retained callbacks

**§ 100(1)** A retained callback does not necessarily execute during the retaining call.

**§ 100(2)** The graph records:

```text
retention relation;
possible later callback entry;
execution contract;
lifetime dependency;
known or open target set.
```

**§ 100(3)** The actual callback execution may become a derived root of an event, task,
thread, foreign, or target execution context.

---

## § 101 Foreign callbacks

**§ 101(1)** A callback invoked by foreign code is a foreign-entry relationship.

**§ 101(2)** It records:

```text
ABI;
foreign thread/task attachment assumptions;
callable contract;
unsafe provenance;
panic boundary;
ownership and retention contract.
```

---

## § 102 Interfaces and effect compatibility

**§ 102(1)** Interface callable contracts retain declared effect guarantees.

**§ 102(2)** Concrete implementations must satisfy those guarantees.

**§ 102(3)** Known implementations may be stronger.

**§ 102(4)** The open call site uses the interface contract.

---

## § 103 Separate compilation

**§ 103(1)** Compiled module metadata contains enough callable information for callers to
analyze public relationships without reading all implementation source.

**§ 103(2)** It should include:

```text
public callable identity;
signature;
unsafe status;
declared effect guarantees;
open interface contracts;
callback retention contracts;
generic call dependencies;
foreign provenance;
stack summary where required;
allocation summary where required;
reentry contract where required.
```

**§ 103(3)** Private implementation details may be included separately for LTO.

---

## § 104 Link-time refinement

**§ 104(1)** Link-time or whole-program analysis may refine:

```text
open target sets;
function-value target sets;
interface implementations;
foreign adapter targets;
dead callable nodes.
```

**§ 104(2)** Refinement may strengthen internal inferred facts.

**§ 104(3)** It must not weaken declared public guarantees.

---

## § 105 Dead stripping

**§ 105(1)** A callable not reachable from any active root may be removed from the binary
when linking and reflection rules permit it.

**§ 105(2)** The compile-time graph may retain the node for diagnostics or incremental reuse.

**§ 105(3)** No runtime graph is required.

---

## § 106 Incremental compilation

**§ 106(1)** Stable node and call-site identities allow targeted invalidation.

**§ 106(2)** When a node changes, the compiler invalidates at least:

```text
its direct call-site set;
its direct summaries;
its target-resolution facts;
its containing SCC;
transitive callers affected by changed facts;
derived effect, stack, allocation, ISR, panic, unsafe, and reachability facts.
```

**§ 106(3)** A change to an open callable contract may invalidate external dependents.

---

## § 107 Graph update phases

**§ 107(1)** A recommended incremental flow is:

```text
resolve changed declarations;
rebuild changed Semantic IR;
update direct call sites;
update target sets;
update roots;
update reachability;
update affected SCCs;
recompute dependent summaries;
emit changed diagnostics.
```

---

## § 108 Semantic IR requirements

**§ 108(1)** Semantic IR must preserve call-graph-relevant operations explicitly.

**§ 108(2)** At minimum:

```text
DirectCall;
MethodCall;
FunctionValueCall;
InterfaceCall;
ForeignCall;
IntrinsicCall;
GeneratedCall;
SpawnTask;
SpawnThread;
SpawnProcess;
TaskAwait;
TaskJoin;
CallbackRetain;
CallbackInvoke;
DeferRegister;
DeferExecute;
DestructorCall;
InterruptEntry;
ContinuationResume.
```

**§ 108(3)** Exact IR names are implementation-defined.

**§ 108(4)** The semantic distinctions are required.

---

## § 109 Spawn IR integration

**§ 109(1)** Spawn IR already records or must record:

```text
callable;
arguments;
receiver;
result type;
execution-handle type;
copied values;
moved values;
retained borrows;
captures;
parent task;
target execution profile;
source location.
```

**§ 109(2)** The call graph consumes these facts.

**§ 109(3)** The backend must not rediscover them from low-level operations.

---

## § 110 Await IR integration

**§ 110(1)** Await IR records:

```text
concrete Task[T] type;
owner being consumed;
task origin where known;
result ownership transfer;
cancellation path;
task failure path;
cleanup before suspension;
borrows live across suspension;
continuation after completion;
source location.
```

**§ 110(2)** The graph preserves synchronization and continuation relationships.

---

## § 111 Graph data model

**§ 111(1)** A recommended model contains:

```text
Graph;
Node;
CallSite;
CallableContract;
TargetSet;
Root;
ExecutionContext;
SynchronizationSite;
SummaryCache;
DiagnosticCause.
```

**§ 111(2)** Exact implementation structure is not normative.

---

## § 112 Node record

**§ 112(1)** Conceptual:

```go
type Node struct {
    ID              NodeID
    Kind            NodeKind
    Declaration     DeclarationID
    Plan            CompilationPlanID
    Specialization  SpecializationKey
    Source          source.Location
    Contract        CallableContractID
}
```

---

## § 113 Target set record

**§ 113(1)** Conceptual:

```go
type TargetSet struct {
    KnownTargets []NodeID
    IsClosed     bool
    OpenContract *CallableContractID
}
```

**§ 113(2)** A target set with `IsClosed == false` must have a valid open contract.

---

## § 114 Root record

**§ 114(1)** Conceptual:

```go
type Root struct {
    Kind       RootKind
    Node       NodeID
    Plan       CompilationPlanID
    Source     source.Location
    Provenance TrustProvenance
}
```

---

## § 115 Analysis views

**§ 115(1)** The graph service should expose views such as:

```text
AllReachable();
SameStackEdges();
TaskSpawnEdges();
ThreadStartEdges();
ProcessLaunchEdges();
InterruptClosure(root);
Callers(node);
Callees(node);
PossibleTargets(site);
RootsReaching(node);
SameStackSCC(node);
TrustPath(node);
EffectPath(node, effect);
AllocationPath(node);
```

**§ 115(2)** Exact API naming is implementation-defined.

---

## § 116 Diagnostics

**§ 116(1)** Diagnostics must retain complete cause paths.

**§ 116(2)** A call-graph path includes:

```text
root;
caller;
call site;
dispatch kind;
execution relation;
target or callable contract;
introducing analysis fact;
active CompilationPlan.
```

---

## § 117 Direct caller diagnostic

```text
error: `HandlePacket` reaches an allocating operation

HandlePacket
  -> calls Decode
  -> calls fields.Grow
  -> allocation may occur

active compilation plan:
    linux/arm64
```

---

## § 118 Interface diagnostic

```text
error: @noPanic cannot be proven

Process
  -> invokes interface method Handler.Handle
  -> open callable contract permits panic

known implementations:
    FastHandler.Handle
    StrictHandler.Handle
```

---

## § 119 Spawn-cycle diagnostic

```text
error: unbounded recursive task creation detected

Bomb
  -> spawn Bomb

the spawn cycle creates two child tasks before waiting for completion

help:
    bound task creation
    remove recursive spawn
    or select a profile that permits and bounds the execution pattern
```

---

## § 120 Open-world root diagnostic

```text
information: function is analyzed as an open-world library entrypoint

callable:
    Parse

external callers may invoke this callable according to its public contract
```

**§ 120(1)** This is normally an analysis report or LSP fact, not normal build noise.

---

## § 121 Diagnostic IDs

**§ 121(1)** Call-graph diagnostics use stable IDs.

**§ 121(2)** IDs identify the rule or compiler phase, not the current severity.

**§ 121(3)** Suggested categories include:

```text
CGxxxx
```

**§ 121(4)** Exact numbering is assigned by the canonical diagnostics registry.

---

## § 122 LSP behavior

**§ 122(1)** The LSP should provide:

```text
direct callers;
direct callees;
possible indirect targets;
open callable contract;
root reachability;
same-stack recursion component;
task-spawn relationships;
thread-start relationships;
process-launch relationships;
await/join synchronization origin where known;
effect path;
allocation path;
panic path;
unsafe provenance path;
active target variant.
```

---

## § 123 Find callers

**§ 123(1)** “Find callers” distinguishes:

```text
confirmed direct caller;
possible indirect caller;
open-world caller contract;
implicit destructor caller;
deferred caller;
task spawner;
thread starter;
process launcher;
foreign callback entry;
interrupt root.
```

---

## § 124 Hover example

```text
Callable: Decode

Reachable from:
    ProgramEntryRoot main
    OpenWorldRoot ParsePacket

Callers:
    HandlePacket
    callback contract PacketDecoder

Execution:
    synchronous
    task-spawn target

Same-stack recursion:
    none
```

---

## § 125 Graph visualization

**§ 125(1)** Tooling may visualize:

```text
root class;
same-stack edges;
spawn edges;
thread edges;
process edges;
interrupt edges;
open contracts;
trusted foreign boundaries;
SCCs.
```

**§ 125(2)** Visualization is tooling.

**§ 125(3)** It is not runtime metadata.

---

## § 126 No mandatory runtime

**§ 126(1)** The call graph is compile-time metadata.

**§ 126(2)** It requires no:

```text
runtime call-graph object;
reflection registry;
dynamic dispatch registry;
mandatory RTTI;
scheduler;
stack unwinder;
runtime effect system;
runtime graph traversal.
```

**§ 126(3)** Runtime dispatch structures exist only when required by the actual language
semantics and selected lowering.

---

## § 127 Implementation governance

**§ 127(1)** Implementation status for this rulebook is owned by:

```text
implementation-status.yaml
integration: analysis.call-graph
```

**§ 127(2)** This rulebook contains normative requirements and does not duplicate the
repository's current implementation state.

---

## § 128 Implementation scope

**§ 128(1)** The first implementation should support:

```text
stable callable node identity;
stable call-site identity;
direct calls;
static method calls;
generic specialization nodes;
closure/lambda nodes;
synthetic defer nodes;
destructor nodes;
task-spawn relationships;
thread-spawn relationships;
await synchronization;
join synchronization;
function-value target sets;
interface target sets;
open callable contracts;
roots;
reachability;
same-stack SCCs;
spawn-cycle analysis;
effect-analysis propagation;
cause-aware diagnostics.
```

---

## § 129 Deferred source syntax

**§ 129(1)** This rulebook deliberately does not invent source syntax for:

```text
effect-constrained function types;
stack-bound callable contracts;
no-reentry callable contracts;
callback retention annotations;
foreign callback attachment;
process entry contracts.
```

**§ 129(2)** The graph representation must permit these semantics when their source syntax is
defined.

---

## § 130 Required tests

**§ 130(1)** Create or update:

```text
call_graph_direct_valid.sec
call_graph_direct_invalid.sec
call_graph_methods_valid.sec
call_graph_generics_valid.sec
call_graph_closures_valid.sec
call_graph_defer_valid.sec
call_graph_defer_invalid.sec
call_graph_destruction_valid.sec
call_graph_recursion_valid.sec
call_graph_recursion_invalid.sec
call_graph_function_values_valid.sec
call_graph_function_values_invalid.sec
call_graph_interfaces_valid.sec
call_graph_interfaces_invalid.sec
call_graph_open_world_valid.sec
call_graph_open_world_invalid.sec
call_graph_spawn_valid.sec
call_graph_spawn_invalid.sec
call_graph_spawn_cycles_valid.sec
call_graph_spawn_cycles_invalid.sec
call_graph_await_valid.sec
call_graph_await_invalid.sec
call_graph_threads_valid.sec
call_graph_threads_invalid.sec
call_graph_processes_valid.sec
call_graph_processes_invalid.sec
call_graph_interrupts_valid.sec
call_graph_interrupts_invalid.sec
call_graph_ffi_valid.sec
call_graph_ffi_invalid.sec
call_graph_effects_valid.sec
call_graph_effects_invalid.sec
call_graph_stack_valid.sec
call_graph_stack_invalid.sec
call_graph_incremental_test.go
call_graph_module_metadata_test.go
```

---

## § 131 Node tests

**§ 131(1)** Test:

```text
ordinary function node;
method node;
generic specialization identity;
closure identity;
lambda identity;
always-created defer node;
destructor node;
foreign node;
intrinsic summary;
generated helper node;
task entry node;
thread entry node;
interrupt entry node;
open contract node.
```

---

## § 132 Call-site tests

**§ 132(1)** Test:

```text
stable source identity;
direct target;
closed target set;
open target set;
dispatch kind;
execution relation;
unsafe invocation;
trust provenance;
active CompilationPlan;
source location.
```

---

## § 133 Spawn tests

**§ 133(1)** Test:

```text
spawn defaults to SpawnTask;
spawn task uses SpawnTask;
spawn thread uses SpawnThread;
spawn process uses SpawnProcess;
spawn is eager;
spawn target is reachable without await;
spawned body is not same-stack;
spawn arguments preserve copy/move/borrow facts;
spawned methods preserve receiver behavior;
spawned closures preserve captures;
nested spawn creates child execution context;
spawn cycles are distinct from recursion.
```

---

## § 134 Await and join tests

**§ 134(1)** Test:

```text
await does not create a second call to task body;
await consumes Task[T];
join preserves Task[T];
direct await preserves spawn and synchronization relationships;
continuation is recorded;
task-retained borrow may end at await;
await is not same-stack recursion;
await inside defer remains rejected.
```

---

## § 135 Indirect-call tests

**§ 135(1)** Test:

```text
exact function-value target;
closed finite target set;
open callable contract;
known targets plus open contract;
unknown concrete target with valid contract;
unknown callable contract rejected;
unsafe function value preserved;
foreign function pointer requires trusted contract.
```

---

## § 136 Mentor diagnostic tests

**§ 136(1)** Test:

```text
no noise when contract is sufficient;
information for conservative analysis report;
warning for unknown stack in permissive profile;
warning promotion to error;
@noPanic failure through open callback;
@noAlloc failure through open callback;
ISR failure through open callback;
recursion-disabled failure through unknown reentry;
help text proposes stronger contract.
```

---

## § 137 Root tests

**§ 137(1)** Test:

```text
program entry root;
target startup root;
test root;
interrupt root;
foreign export root;
open-world library root;
derived task entry;
derived thread entry;
process launch contract;
multiple root classes reaching one node.
```

---

## § 138 Reachability tests

**§ 138(1)** Test:

```text
proven-dead call omitted;
runtime branch call retained;
inactive @target variant omitted;
inactive @when variant omitted;
target-specific graph differs correctly;
unreachable node can be dead-stripped;
open-world root preserves externally callable node.
```

---

## § 139 SCC tests

**§ 139(1)** Test:

```text
direct recursion;
mutual recursion;
recursion through function value;
recursion through interface;
recursion through defer;
recursion through destructor;
spawn cycle excluded from same-stack SCC;
thread cycle excluded from same-stack SCC;
unknown reentry blocks recursion-off proof.
```

---

## § 140 Analysis integration tests

**§ 140(1)** Test:

```text
effects through same-stack edge;
spawn creation effects only on spawner;
spawn body effects on new execution context;
await MaySuspend handling;
allocation path;
panic path;
unsafe provenance path;
interrupt call closure;
stack maximum through closed target set;
unknown stack bound behavior by profile.
```

---

## § 141 Incremental tests

**§ 141(1)** Test:

```text
changing callee invalidates affected callers;
changing target set invalidates SCC;
changing callable contract invalidates external dependents;
unrelated edit preserves stable node IDs;
closure identity remains stable across unrelated edits;
defer identity remains stable;
target-plan change rebuilds only relevant graph.
```

---

## § 142 Binary tests

**§ 142(1)** Verify:

```text
call graph requires no runtime table;
unreachable callables may be stripped;
inactive target variants do not enter object code;
analysis-only synthetic nodes do not require runtime objects;
intrinsic summaries do not force helper emission;
open-world metadata is not emitted into executable unless another feature
requires it.
```

---

## § 143 Required synchronization

**§ 143(1)** This rulebook must remain synchronized with:

```text
effect_analysis.md
reference_model.md
unsafe.md
spawn.md
await.md
tasks.md
threads.md
processes.md
concurrency.md
concurrency_memory_model.md
functions.md
declarations/lambda-functions.md
declarations/generics.md
declarations/interfaces.md
defer.md
destruction.md
panic.md
runtime_checks.md
allocation.md
ownership.md
borrowing.md
lifetime_analysis.md
semantic_ir.md
compiler_analysis.md
compiler_pipeline.md
projects.md
attributes.md
diagnostics.md
lsp.md
language-rulebook-status.md
implementation-status.yaml
```

---

## § 144 Rulebook inventory update

**§ 144(1)** After adding this document:

```text
call_graph.md
    status becomes Written
```

**§ 144(2)** Update:

```text
language-rulebook-status.md
implementation-status.yaml
```

**§ 144(3)** `reference_model.md` must also be added to the canonical written inventory when
that repository synchronization has not already occurred.

**§ 144(4)** A separate planned `generational_references.md` must not be added.

**§ 144(5)** Generational validity is covered by:

```text
reference_model.md
```

---

## Appendix A — Codex implementation plan

### Appendix A.1 Add the rulebook

**Appendix A.1(1)** Add:

```text
rules/analysis/call_graph.md
```

**Appendix A.1(2)** Update:

```text
language-rulebook-status.md
implementation-status.yaml
```

**Appendix A.1(3)** Mark the rulebook Written.

**Appendix A.1(4)** Track implementation progress under integration ID `analysis.call-graph` and
do not mark it complete until the complete rulebook contract is verified.

---

### Appendix A.2 Inventory existing compiler structures

**Appendix A.2(1)** Locate existing structures for:

```text
function and method resolution;
generic specialization;
lambda and closure analysis;
function-value flow;
interface implementation lookup;
recursion detection;
effect propagation;
allocation paths;
stack analysis;
spawn and await analysis;
defer and destruction;
foreign calls;
interrupt roots;
module metadata;
incremental compilation.
```

**Appendix A.2(2)** Reuse or migrate existing facts.

**Appendix A.2(3)** Do not build a disconnected second graph.

---

### Appendix A.3 Add stable identities

**Appendix A.3(1)** Define stable IDs for:

```text
callable nodes;
call sites;
callable contracts;
roots;
execution contexts;
synchronization sites;
generic specializations;
closures;
defer bodies;
destructors.
```

**Appendix A.3(2)** Avoid map-order and pointer-address identities.

---

### Appendix A.4 Add node registry

**Appendix A.4(1)** Create a central node registry keyed by semantic identity.

**Appendix A.4(2)** Support at least:

```text
functions;
methods;
specializations;
closures;
defer bodies;
destructors;
foreign callables;
generated helpers;
intrinsics;
task entries;
thread entries;
interrupt entries;
open contracts.
```

---

### Appendix A.5 Add call-site records

**Appendix A.5(1)** Every Semantic IR call-like operation produces one call-site record.

**Appendix A.5(2)** Store:

```text
caller;
source;
dispatch;
execution;
target set;
open contract;
unsafe requirement;
trust provenance.
```

---

### Appendix A.6 Integrate target selection

**Appendix A.6(1)** Build the graph only after active:

```text
@target;
@when;
target profile;
device/board knowledge;
implementation variant.
```

**Appendix A.6(2)** Use one graph per concrete `CompilationPlan`.

---

### Appendix A.7 Direct target resolution

**Appendix A.7(1)** Add exact targets for:

```text
direct function calls;
static method calls;
resolved generated calls;
known foreign functions;
known intrinsics.
```

---

### Appendix A.8 Generic specialization nodes

**Appendix A.8(1)** Create semantic nodes for concrete specializations.

**Appendix A.8(2)** Do not merge them merely because backend code may be shared.

---

### Appendix A.9 Closure and lambda nodes

**Appendix A.9(1)** Create stable nodes for every callable closure/lambda body.

**Appendix A.9(2)** Connect capture and retention facts through Semantic IR references.

---

### Appendix A.10 Always create defer nodes

**Appendix A.10(1)** Create one synthetic callable node for every `defer` body.

**Appendix A.10(2)** Do not condition node creation on whether the body currently contains a call.

---

### Appendix A.11 Destructor nodes and implicit edges

**Appendix A.11(1)** Represent implicit destruction through visible destruction relationships.

**Appendix A.11(2)** Include them in same-stack SCC and effect analysis.

---

### Appendix A.12 Spawn relationships

**Appendix A.12(1)** Consume canonical spawn Semantic IR.

**Appendix A.12(2)** Represent:

```text
SpawnTask;
SpawnThread;
SpawnProcess.
```

**Appendix A.12(3)** Preserve:

```text
eager reachability;
parent execution context;
copy/move/borrow/capture facts;
result handle type;
requested execution kind.
```

---

### Appendix A.13 Await and join synchronization

**Appendix A.13(1)** Consume canonical await/join Semantic IR.

**Appendix A.13(2)** Represent:

```text
TaskAwait;
TaskJoin;
continuation;
task origin where known;
borrow completion boundary;
result ownership.
```

**Appendix A.13(3)** Do not create another call edge to the task body.

---

### Appendix A.14 Function-value target analysis

**Appendix A.14(1)** Perform sound value-flow analysis.

**Appendix A.14(2)** Produce:

```text
exact target;
closed finite set;
open callable contract;
known set plus open contract.
```

**Appendix A.14(3)** Never omit a possible target.

---

### Appendix A.15 Interface target analysis

**Appendix A.15(1)** Resolve known implementation targets.

**Appendix A.15(2)** Preserve open-world interface contract for library boundaries.

---

### Appendix A.16 Callable contracts

**Appendix A.16(1)** Add a normalized callable-contract structure.

**Appendix A.16(2)** At minimum:

```text
signature;
ABI;
unsafe status;
ownership modes;
effect guarantees;
retention behavior;
reentry behavior;
stack bound where known;
trust provenance.
```

**Appendix A.16(3)** Unknown concrete targets may use the contract.

**Appendix A.16(4)** Unknown contracts are errors.

---

### Appendix A.17 Root discovery

**Appendix A.17(1)** Discover:

```text
program roots;
target startup roots;
initialization roots;
test roots;
interrupt roots;
foreign export roots;
generated roots;
open-world roots.
```

**Appendix A.17(2)** Treat externally callable library declarations as `OpenWorldRoot`.

---

### Appendix A.18 Reachability

**Appendix A.18(1)** Compute reachability by root class.

**Appendix A.18(2)** Retain:

```text
which roots reach each node;
which execution relation enters it;
which CompilationPlan applies.
```

---

### Appendix A.19 Path pruning

**Appendix A.19(1)** Use mandatory semantic reachability proof to omit:

```text
constant-false branches;
inactive target variants;
inactive configuration variants;
proven impossible match branches.
```

**Appendix A.19(2)** Do not depend on optional optimizer passes.

---

### Appendix A.20 SCC views

**Appendix A.20(1)** Compute at least:

```text
same-stack SCC;
task-spawn SCC;
thread-start SCC;
process-launch SCC;
complete execution SCC.
```

**Appendix A.20(2)** Use same-stack SCC for recursion policy and stack analysis.

---

### Appendix A.21 Effect integration

**Appendix A.21(1)** Replace ad hoc effect-call propagation with graph-based fixed-point analysis.

**Appendix A.21(2)** Use execution-specific transfer rules.

---

### Appendix A.22 Stack integration

**Appendix A.22(1)** Expose same-stack targets and bounds.

**Appendix A.22(2)** Handle:

```text
closed target maximum;
open declared bound;
permissive unknown contribution;
strict-profile error.
```

---

### Appendix A.23 Allocation and panic paths

**Appendix A.23(1)** Expose complete root-to-cause paths for allocation and panic analysis.

---

### Appendix A.24 Reference and escape integration

**Appendix A.24(1)** Link call sites to Semantic IR argument, receiver, capture, retention, spawn,
await, and foreign-lifetime facts.

**Appendix A.24(2)** Do not duplicate reference-model state inside graph edges.

---

### Appendix A.25 Unsafe provenance

**Appendix A.25(1)** Preserve trusted edges through safe wrappers.

**Appendix A.25(2)** Expose audit paths without marking safe callers as unsafe.

---

### Appendix A.26 Module metadata

**Appendix A.26(1)** Emit public callable contracts and summaries required for separate compilation.

**Appendix A.26(2)** Keep inferred implementation details separate from declared guarantees.

---

### Appendix A.27 Incremental invalidation

**Appendix A.27(1)** Build dependency indexes:

```text
node -> callers;
contract -> call sites;
target set -> SCC;
root -> reachable nodes;
summary -> dependent analyses.
```

**Appendix A.27(2)** Invalidate only affected regions where sound.

---

### Appendix A.28 Diagnostics

**Appendix A.28(1)** Implement consequence-based severity:

```text
none;
information;
warning;
error.
```

**Appendix A.28(2)** Use stable IDs.

**Appendix A.28(3)** Include cause path and actionable help.

---

### Appendix A.29 LSP

**Appendix A.29(1)** Add:

```text
find callers;
find callees;
possible targets;
open contract;
root reachability;
recursion component;
spawn relationships;
await origin;
effect path;
allocation path;
unsafe provenance.
```

---

### Appendix A.30 Tests

**Appendix A.30(1)** Run:

```text
go test ./...
compiler build
LSP build
formatter tests
call-graph fixtures
spawn/await fixtures
effect-analysis fixtures
stack-analysis fixtures
target matrix
incremental tests
binary dependency tests
```

**Appendix A.30(2)** Do not claim complete implementation while indirect calls, roots, spawn/await,
defer, destruction, or SCC views remain partial.

---

## Appendix B — Canonical node table

| Node kind | Meaning |
|---|---|
| `SecFunction` | Concrete ordinary Sec function |
| `SecMethod` | Concrete method implementation |
| `GenericSpecialization` | Concrete semantic generic specialization |
| `ClosureBody` | Closure callable body |
| `LambdaBody` | Lambda callable body |
| `DeferBody` | Always-created synthetic defer body |
| `Destructor` | Destructor callable body |
| `ForeignFunction` | Foreign callable declaration or contract |
| `CompilerIntrinsic` | Compiler-known callable summary |
| `CompilerGeneratedHelper` | Generated callable behavior visible to analysis |
| `TaskEntry` | Entry of spawned task execution |
| `ThreadEntry` | Entry of spawned thread execution |
| `ProcessEntryContract` | Process launch target or program contract |
| `InterruptEntry` | Interrupt execution root |
| `UnknownCallableContract` | Valid open callable contract without exact target |
| `PanicHandler` | Configured callable panic handler where applicable |

---

## Appendix C — Canonical dispatch table

| Dispatch kind | Meaning |
|---|---|
| `Direct` | Exact ordinary function target |
| `StaticMethod` | Exact resolved method target |
| `Closure` | Closure/lambda callable target |
| `FunctionValue` | Function-value dispatch |
| `InterfaceDispatch` | Interface contract dispatch |
| `ForeignDirect` | Direct foreign declaration |
| `ForeignFunctionPointer` | Foreign callable pointer with trusted contract |
| `Intrinsic` | Compiler-known intrinsic |
| `CompilerGenerated` | Generated callable relationship |
| `UnknownContract` | Concrete target unknown, valid callable contract known |

---

## Appendix D — Canonical execution-relation table

| Execution relation | Same stack | New execution context | Notes |
|---|---:|---:|---|
| `Synchronous` | Yes | No | Ordinary call |
| `ImmediateCallback` | Yes | No | Callback invoked during current call |
| `Deferred` | Yes | No | Deferred cleanup execution |
| `Destruction` | Yes | No | Destructor execution |
| `SpawnTask` | No | Yes | New eager task |
| `SpawnThread` | No | Yes | New physical/logical thread |
| `SpawnProcess` | No | Yes | New process/address space |
| `InterruptEntry` | No | Yes | Interrupt root |
| `ForeignCallbackEntry` | Depends on contract | Yes | External entry into Sec |
| `ContinuationResume` | No previous stack continuation | Resumed task context | Resumes after suspension |
| `RetainedCallback` | Not necessarily | Contract-defined | Stored for later invocation |
| `AsynchronousCallback` | No | Yes | Callback executes asynchronously |

---

## Appendix E — Canonical indirect-call outcome table

| Target knowledge | Callable contract | Result |
|---|---|---|
| Exact target | Known | Exact edge |
| Closed finite target set | Known | All possible edges |
| Known targets plus open world | Known | Known edges plus open contract |
| Concrete target unknown | Known | Valid conservative call |
| Concrete target unknown | Missing | Compile error |
| Contract lacks optional guarantee | Otherwise valid | Conservative analysis |
| Missing guarantee blocks declared promise | Otherwise valid | Compile error |
| Missing bound allowed by profile | Otherwise valid | Information or warning |

---

## § 145 Final canonical summary

**§ 145(1)** Sec builds one canonical semantic call graph per concrete `CompilationPlan`.

**§ 145(2)** Callable nodes represent concrete execution units.

**§ 145(3)** Call sites retain source identity, dispatch kind, execution relation, target set,
callable contract, unsafe requirement, and trust provenance.

**§ 145(4)** Dispatch and execution are independent properties.

**§ 145(5)** Same-stack calls, defer, and destruction participate in ordinary recursion and
stack analysis.

**§ 145(6)** Every `defer` body always receives a synthetic callable node.

**§ 145(7)** `spawn` is already defined by the canonical spawn rulebook.

**§ 145(8)** It creates an eager task by default and preserves explicit task, thread, and
process execution kinds.

**§ 145(9)** Spawn targets become reachable when spawned, not when awaited.

**§ 145(10)** `await` is task-specific in Sec 0.1, consumes `Task[T]`, synchronizes with task
completion, and resumes through an explicit continuation.

**§ 145(11)** `await` does not call the task body again.

**§ 145(12)** `join` synchronizes while preserving the handle.

**§ 145(13)** Task, thread, and process cycles are distinct from same-stack recursion.

**§ 145(14)** Unknown concrete targets are permitted when a valid callable contract exists.

**§ 145(15)** An unknown callable contract is a compile error.

**§ 145(16)** Open-world library declarations are `OpenWorldRoot` nodes.

**§ 145(17)** Interface and function-value calls retain known targets plus open contracts.

**§ 145(18)** Conservative uncertainty produces no diagnostic, information, warning, or
error according to its actual consequence.

**§ 145(19)** Declared guarantees and strict profiles require proof.

**§ 145(20)** Call-graph diagnostics explain what is unknown, why it matters, the conservative
assumption, the complete path, and how to strengthen the contract.

**§ 145(21)** Compiler-generated helpers may use intrinsic summaries only when they are
semantically atomic for every relevant analysis.

**§ 145(22)** The call graph coordinates effects, stack, allocation, panic, ISR, unsafe,
reference, ownership, escape, task, thread, and process analysis.

**§ 145(23)** The call graph introduces no mandatory runtime.
