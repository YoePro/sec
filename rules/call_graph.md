# Call Graph

## Status

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

# Purpose

The call graph answers questions such as:

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

The graph is shared infrastructure for multiple analyses.

It must not become a collection of unrelated, partially inconsistent graphs.

---

# Core principle

```text
The Sec call graph is a sound compile-time representation of every reachable
callable execution relationship for one concrete CompilationPlan.
```

Sound means:

```text
a possible callable relationship must not be omitted;
a target set may conservatively contain targets that do not execute at runtime;
an unknown concrete target may be represented by a valid callable contract;
an unknown callable contract may not be silently accepted.
```

The graph is primarily a may-execute graph.

It records execution that may occur on at least one semantically reachable path.

---

# One canonical graph, multiple analysis views

Sec builds one canonical semantic graph per concrete `CompilationPlan`.

Analyses derive specialized views from that graph.

Examples:

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

The compiler must not build independent graphs that disagree about:

```text
which call sites exist;
which targets are possible;
which target variant is active;
whether an edge crosses an execution-context boundary;
whether the relationship is implicit or explicit;
whether the relationship is trusted or proven.
```

---

# Non-goals

This rulebook does not define:

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

Those are defined by their owning rulebooks.

This rulebook consumes their semantic results.

The call graph does not replace:

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

# Terminology

## Callable unit

A callable unit is a semantic execution body that may be entered through a call,
spawn, callback, interrupt, foreign entry, generated invocation, deferred
execution, destruction, or another compiler-known relationship.

## Callable declaration

A source or compiler declaration that describes callable behavior.

A callable declaration is not always a concrete graph node.

A generic template is a declaration but not a concrete executable
specialization.

## Callable node

A concrete semantic graph node representing one callable execution unit for one
active compilation plan and specialization.

## Call site

A semantic source or generated location that invokes or schedules a callable.

A call site is distinct from the target callable node.

## Target set

The sound set of concrete callable nodes that a call site may invoke.

## Callable contract

The semantic contract used when the complete concrete target set is not known.

## Dispatch kind

How the callable target is selected.

## Execution relation

How execution of the target relates to the caller's current execution context.

## Root

A graph entry from which reachability analysis begins.

## Open-world root

An externally callable library declaration that may be invoked by code not
present in the current compilation unit.

## Same-stack relationship

A relationship where the target executes as part of the current synchronous
stack path.

## Execution-context boundary

A relationship that starts or enters another task, thread, process, interrupt,
or continuation context.

---

# Graph layers

The canonical graph contains cooperating relationship layers.

## Callable reachability layer

Represents which callable bodies may execute.

## Call-site layer

Preserves source location, dispatch, target set, contracts, and analysis
provenance.

## Execution-context layer

Distinguishes:

```text
same-stack execution;
new task execution;
new thread execution;
new process execution;
interrupt execution;
resumed continuation execution.
```

## Synchronization layer

Represents operations such as:

```text
await;
join;
task completion observation;
thread join;
process wait where defined.
```

A synchronization relation is not necessarily a call edge to the original
callable target.

## Root layer

Represents executable, interrupt, foreign, test, generated, and open-world
entrypoints.

These layers may use one implementation structure or cooperating structures.

They must share stable node and site identities.

---

# Callable node kinds

The initial callable node kinds are:

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

An implementation may use a smaller internal enum when several kinds share one
representation.

The semantic distinctions must remain available.

---

# Ordinary functions

A normal function declaration produces a callable node when it is active and
concrete for the current compilation plan.

```sec
fn Parse(input: ref rune[]) Result[Node, ParseError] {
    // ...
}
```

The callable node records at least:

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

# Methods

A method is a callable node.

```sec
impl Parser {
    fn Next() Token {
        // ...
    }
}
```

The implicit `self` receiver is represented through callable and call-site
metadata.

The node identity must distinguish:

```text
receiver type;
implementation identity;
generic receiver substitutions;
active target variant.
```

---

# Generic declarations

A generic declaration is a callable template.

```sec
fn Convert[T](value: T) string {
    // ...
}
```

The unspecialized template is not a concrete executable call-graph node.

Concrete semantic specializations are nodes:

```text
Convert[int]
Convert[Customer]
```

A compiler may retain a template-summary object for analysis.

That template-summary object is not treated as an executable node unless the
runtime representation truly supports unspecialized generic dispatch.

---

# Generic specialization identity

A specialization node identity includes:

```text
generic declaration identity;
type substitutions;
compile-time value substitutions;
receiver substitutions;
active implementation variant;
target/configuration identity.
```

Backend code sharing does not merge semantic graph identity.

Two specializations may lower to one machine implementation while retaining
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

# Closures and lambdas

Every closure or lambda body receives a stable compiler identity.

```sec
let callback := fn(value: int) int {
    return value + 1
}
```

The body is a callable node.

The capture environment is not a callable node.

Capture creation, allocation, moves, borrows, retention, destruction, and
escape are represented through Semantic IR and cooperating analyses.

---

# Local callable identity

A local callable identity must remain stable across unrelated edits where
possible.

A recommended identity includes:

```text
containing declaration identity;
source syntax identity;
semantic ordinal within the containing declaration;
capture signature;
active specialization.
```

A raw parser object address or unstable build-order counter is not a stable
identity.

---

# `defer` bodies

Every `defer` body receives a synthetic callable node.

This applies even when the body contains no explicit call.

```sec
defer {
    state.completed = true
}
```

Conceptually:

```text
FunctionNode
    -> DeferredExecution
        -> DeferBodyNode
```

Reasons:

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

The `defer` body remains subject to all canonical defer and cleanup rules.

---

# Destructors

A destructor is a callable node.

Implicit destructor invocation is represented through a destruction
relationship.

Destructor relationships must remain visible to:

```text
effect analysis;
panic analysis;
stack analysis;
recursion analysis;
allocation analysis;
unsafe provenance;
diagnostics.
```

The graph must not omit a destructor because the invocation is compiler
generated.

---

# Foreign functions

A foreign function declaration is a callable node or callable contract node.

```sec
unsafe extern "system" fn rawSysCall(...) int64
```

The node records:

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

The implementation body is not proven from Sec source.

---

# Compiler intrinsics

A compiler intrinsic may be represented as:

```text
a callable node with a compiler-known summary;
or
an explicit Semantic IR operation without a callable node.
```

An intrinsic may be omitted as a graph node only when it is semantically atomic
for every analysis that consumes the call graph.

Its summary must still expose all relevant:

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

# Compiler-generated helpers

A compiler-generated helper is a visible callable node when its internal
behavior matters to any analysis.

Examples:

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

A helper may use an intrinsic summary only when treating it as atomic preserves
all language semantics and diagnostics.

The compiler must not hide a meaningful call chain merely because the helper is
generated.

---

# Callable node identity

A node identity must not depend on:

```text
memory address of an AST node;
iteration order of a map;
temporary parser index;
backend symbol numbering;
link order.
```

A conceptual node key includes:

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

Exact compiler representation is implementation-defined.

---

# Call-site identity

Every callable invocation or scheduling site receives a stable identity.

A call-site identity includes:

```text
containing callable node;
source or generated syntax identity;
source location;
semantic operation kind;
active specialization;
active CompilationPlan.
```

The graph stores one semantic call-site record.

It does not replace one indirect call site with unrelated anonymous edges and
discard the site identity.

---

# Call-site record

A call-site record should contain at least:

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

Conceptual structure:

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

This is illustrative.

All implementation code is written in English.

---

# Dispatch kinds

The initial dispatch kinds are:

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

Dispatch kind answers:

```text
how is the callable selected?
```

It does not answer:

```text
where or when does it execute?
```

---

# Execution relations

The initial execution relations are:

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

An implementation may use flags or a normalized relation structure.

The distinctions must remain available to analysis.

---

# Dispatch and execution are independent

A function value may be invoked synchronously:

```text
Dispatch:
    FunctionValue

Execution:
    Synchronous
```

An interface method may be spawned:

```text
Dispatch:
    InterfaceDispatch

Execution:
    SpawnTask
```

A foreign callback may enter from outside the program:

```text
Dispatch:
    ForeignFunctionPointer

Execution:
    ForeignCallbackEntry
```

The compiler must not encode all combinations as unrelated special-case enums
that lose the shared semantics.

---

# Same-stack execution

The following normally execute on the current stack:

```text
Synchronous;
ImmediateCallback;
Deferred;
Destruction.
```

They participate in same-stack:

```text
reachability;
effect propagation;
stack accumulation;
recursion SCCs;
panic chains;
allocation chains.
```

A target-specific lowering may inline the call.

Inlining does not remove the semantic relationship from analysis.

---

# New task execution

`spawn expression` and `spawn task expression` start new task execution.

They are equivalent at the language level.

```sec
let worker := spawn Work()
let explicitWorker := spawn task Work()
```

The spawned function remains an ordinary synchronous callable declaration.

`spawn` creates the new execution entity.

The same callable may be:

```text
called synchronously;
spawned as a task;
spawned through another supported execution kind.
```

---

# Task spawn relationship

A task spawn call site records:

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

The spawn target becomes the entry node of a new task execution context.

The target remains reachable from the spawning root.

It does not run on top of the spawner's current stack.

---

# Eager task semantics

Task spawning is eager.

After the spawn expression completes, the task is:

```text
scheduled;
running;
or already completed.
```

The graph must not model the task body as a lazy callable that becomes reachable
only at `await`.

Reachability begins at the spawn site.

`await` synchronizes with an already-created task.

---

# Task parenthood

A task spawn relationship records the current task as the parent task context.

Parenthood does not replace ownership of the `Task[T]` handle.

The call graph records execution parenthood.

Task lifecycle analysis records handle ownership.

---

# Spawn effects

The spawner receives effects from task creation and scheduling.

Examples may include:

```text
MaySpawn;
MayAllocate;
task-creation failure;
scheduler interaction.
```

The spawned body's effects belong to the new task execution context.

They remain part of whole-program reachability.

They do not automatically become synchronous effects of the spawner.

---

# New thread execution

```sec
let worker := spawn thread Work()
```

creates a `Thread[T]` execution context according to the canonical thread rules.

A `SpawnThread` relationship:

```text
starts a separate thread execution context;
does not extend the caller's current stack;
retains ownership, transfer, and reference metadata;
requires target support;
must not silently lower to an ordinary task.
```

The thread entry target remains reachable from the spawning root.

---

# New process execution

```sec
let child := spawn process Program()
```

creates or launches a process according to the process rulebook.

A process launch crosses an address-space and process boundary.

The call graph records:

```text
launching callable;
process-launch site;
known process entry contract or program identity;
argument and ownership transfer contract;
process result-handle contract;
trust and target provenance.
```

The child process may have a separate executable call graph.

The parent graph must not treat child process functions as same-address-space
callees.

---

# `await`

`await` is task-specific in Sec 0.1.

```sec
let value := await task
```

`await`:

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

# Await is not a call edge to the task body

The task body became reachable at `spawn`.

`await task` does not call the task body again.

The graph represents `await` as a synchronization relation connected to:

```text
the awaiting callable;
the Task[T] value;
its origin spawn site or transferred task origin where known;
the continuation after completion;
the result and cancellation/failure paths;
the borrow-release boundary.
```

---

# Await continuation

Semantic IR records the continuation after task completion.

The call graph or cooperating execution graph records:

```text
AwaitTask synchronization;
ContinuationResume relation;
continuation ownership state;
live borrows across suspension;
cleanup and cancellation paths.
```

A continuation may remain compiler-internal.

It receives a stable identity when represented as a separate callable node.

---

# Direct await

```sec
let value := await spawn Calculate()
```

has the same semantic execution relationships as:

```sec
let task := spawn Calculate()
let value := await task
```

The compiler may optimize the physical implementation.

The graph preserves:

```text
task creation;
task execution context;
await synchronization;
completion continuation.
```

---

# `join`

`join task` waits for completion while preserving the owning task handle.

The graph represents `join` as a synchronization relation.

It does not call the task body again.

The relationship records:

```text
task origin where known;
completion observation;
continuation after join;
memory synchronization;
result availability;
preserved handle ownership.
```

`await` and `join` remain distinct.

---

# Await and same-stack recursion

`await` is not a same-stack call edge to the spawned callable.

A cycle involving:

```text
A spawns B;
B spawns A;
```

is a spawn cycle.

It is not ordinary same-stack recursion.

A continuation may later call into an existing recursion component, but that
relationship is represented by its actual synchronous call sites.

---

# Spawn cycles

Spawn cycles are analyzed separately from stack recursion.

Examples:

```text
self-spawn;
mutual task spawn;
recursive detached spawn;
exponential child-task creation.
```

A spawn cycle may be legal only when:

```text
the selected profile permits it;
task creation is bounded;
ownership of every task handle is resolved;
resource use is bounded or accepted by profile policy.
```

Statically evident unbounded creation is diagnosed.

---

# Thread-start cycles

Thread-start cycles are analyzed separately from task-spawn cycles and
same-stack recursion.

They may indicate:

```text
unbounded thread creation;
resource exhaustion;
profile violation;
shutdown or supervision problems.
```

---

# Process-launch cycles

Process-launch cycles are not same-stack recursion.

They may indicate:

```text
unbounded process creation;
fork-like explosion;
resource exhaustion;
supervision cycles.
```

Their legality is governed by process and profile rules.

---

# Direct calls

A statically resolved call has one concrete target.

```sec
let node := Parse(input)
```

Conceptually:

```text
Dispatch:
    Direct

Execution:
    Synchronous

Targets:
    Parse
```

Static method calls use the resolved implementation target.

---

# Function values

A function-value call may have:

```text
one exact target;
a closed finite target set;
an open callable contract;
known targets plus an open contract.
```

The compiler uses value-flow analysis to derive a sound target set.

Example:

```sec
let callback := SelectCallback()
let value := callback(input)
```

Possible targets might be:

```text
Normalize;
Validate;
Reject.
```

The target set must be a safe over-approximation.

---

# Interfaces

Interface dispatch uses the same target-set model.

For a closed executable build, the compiler may derive a finite set of concrete
implementations.

For an open library boundary, future implementations may exist.

The call site then retains:

```text
known concrete targets;
the open interface callable contract.
```

Analyses may use the declared interface guarantees.

They must not use accidental stronger properties of only the currently known
implementations as public guarantees.

---

# Unknown concrete target

An unknown concrete target is permitted when a valid callable contract exists.

Example:

```text
target identity:
    unknown

callable contract:
    known
```

The call remains type-correct and analyzable according to the contract.

The compiler must not require every dynamic target to be statically enumerated
in ordinary hosted code.

---

# Unknown callable contract

A call without a valid callable contract is invalid.

Examples include attempting to invoke a raw address without established:

```text
signature;
ABI;
return representation;
argument ownership;
safe or unsafe status;
effect contract.
```

Canonical rule:

```text
An unresolved target identity is permitted.

An unresolved callable contract is a compile error.
```

---

# Callable contract

A callable contract includes enough information to validate invocation and
perform conservative analysis.

It may contain:

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

Exact source syntax for every optional contract property may be defined by other
rulebooks.

The semantic model is fixed here.

---

# Closed target set

A closed target set contains every possible concrete target.

Analyses combine facts conservatively.

Examples:

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

A closed set does not require a normal build diagnostic.

LSP may display the target set.

---

# Open callable contract

An open callable contract permits targets not present in the current build.

The graph records:

```text
known concrete targets;
open contract identity;
contract guarantees;
open-world provenance.
```

Analysis uses the contract as the public upper bound.

---

# Conservative unknown facts

When a callable contract omits an optional guarantee, the corresponding
property is conservative.

Examples:

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

Conservative does not mean the behavior definitely occurs.

It means the behavior has not been excluded.

---

# Mentor diagnostic policy

Indirect-call uncertainty is diagnosed according to consequence.

## No diagnostic

No diagnostic is required when:

```text
the concrete target is unknown;
the callable contract is valid;
the contract is sufficient for all active requirements.
```

Ordinary callback use must not produce constant noise.

## Information

Information is appropriate when:

```text
conservative analysis was used;
the program remains valid;
no declared guarantee or strict profile requirement is violated;
the information is useful for understanding analysis.
```

Information is normally shown through:

```text
LSP hover;
analysis reports;
verbose compiler mode;
configurable informational diagnostics.
```

## Warning

Warning is appropriate when:

```text
the program remains accepted;
an important bound or proof is unavailable;
the uncertainty may surprise the programmer;
the selected profile permits the uncertainty.
```

Examples:

```text
unknown maximum stack contribution in a permissive hosted profile;
open callback contract permits allocation although current targets do not;
unknown reentry behavior where recursion is allowed but should be reviewed.
```

## Error

Error is required when:

```text
no valid callable contract exists;
a declared guarantee cannot be proven;
a strict target/profile requirement cannot be proven;
ISR safety cannot be proven;
required stack bounds cannot be established;
recursion freedom is required but reentry cannot be excluded.
```

---

# Configurable diagnostic severity

Informational and warning diagnostics use stable diagnostic IDs.

They may be:

```text
promoted;
demoted;
disabled;
reported in analysis-only mode.
```

A semantic error required for soundness cannot be disabled.

---

# Mentor diagnostic content

A call-graph diagnostic must state:

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

# Indirect-call information example

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

# Stack warning example

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

# Guarantee error example

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

# ISR error example

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

# Roots

Reachability begins from graph roots.

The initial root kinds are:

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

# Program entry roots

A normal executable includes its configured program entrypoint.

Target startup code may introduce a separate startup root that eventually
reaches the language entrypoint.

The exact entrypoint syntax and ABI belong to project, target, and ABI
rulebooks.

---

# Initialization roots

Runtime initialization may use explicit initialization roots.

Compile-time evaluation is not represented as ordinary runtime reachability.

Hidden static initialization must not silently introduce task spawning or
awaiting.

---

# Test roots

Each selected test entrypoint is a root for its test compilation plan.

A test graph may include:

```text
test harness generated root;
test setup;
test body;
test cleanup;
selected runtime support.
```

---

# Interrupt roots

An `@interrupt(...)` function is an interrupt execution root.

An `@isr` function may be an externally or manually bound interrupt root
according to the interrupt rulebook.

Interrupt reachability is analyzed separately from ordinary application entry
reachability.

The interrupt root does not become an ordinary caller of `main`.

---

# Foreign export roots

A Sec callable exported for foreign invocation is a foreign-entry root.

The node records:

```text
ABI;
linkage;
foreign trust boundary;
panic restrictions;
thread attachment assumptions;
effect and ownership contract.
```

---

# Open-world library roots

During a library build, every externally callable declaration is represented as
an `OpenWorldRoot`.

This does not assume `pub` syntax.

Sec visibility and external-callability follow the canonical `_` and `__`
scope/visibility rules and related module/linkage rules.

An open-world root means:

```text
code outside the analyzed library may invoke this callable according to its
public callable contract.
```

It is not an ordinary executable startup root.

---

# Root reachability classes

A node may be reachable from multiple root classes.

Examples:

```text
reachable from main;
reachable from interrupt;
reachable from foreign export;
reachable from open-world library root;
reachable from test only.
```

The graph retains this information for diagnostics and binary stripping.

---

# Task entry roots

A spawned task target becomes a derived task-entry root for its new execution
context.

It remains connected to the spawning call site.

It is not a global root when no reachable spawn site creates it.

---

# Thread entry roots

A spawned thread target becomes a derived thread-entry root.

Its stack and execution policies are analyzed separately from the spawning
stack.

---

# Process entry contracts

A spawned process may refer to:

```text
another known Sec executable graph;
an external program contract;
a target-specific process entry.
```

The parent graph records the launch relationship.

A separate child graph is used when the child program is part of the build.

---

# Target and configuration selection

The concrete call graph is built after:

```text
@target selection;
@when selection;
target profile selection;
device/board knowledge selection;
active implementation-variant selection;
generic specialization selection where required.
```

Only active declarations and call sites contribute to the concrete graph.

---

# One graph per `CompilationPlan`

A logical build target may produce multiple concrete outputs.

Example:

```text
linux/amd64;
linux/arm64;
linux/arm32.
```

Each output has its own:

```text
active declarations;
ABI;
target knowledge;
intrinsics;
foreign implementations;
call graph;
analysis summaries.
```

Canonical rule:

```text
One concrete CompilationPlan produces one concrete semantic call graph.
```

---

# Cross-plan comparison

The compiler may maintain an aggregate index across compilation plans.

It is used for questions such as:

```text
does a declared guarantee hold on every configured target?
does one target introduce recursion?
does one target require allocation?
are public callable contracts compatible?
```

The aggregate index is not a merged runtime graph.

---

# Inactive declarations

An inactive declaration is not reachable in the current concrete graph.

It may still be checked in another compilation plan.

Example:

```sec
@target(os: "linux")
fn PlatformWrite(...) void {
}

@target(os: "windows")
fn PlatformWrite(...) void {
}
```

The Linux graph contains only the Linux variant.

---

# Path sensitivity

A call site contributes reachability only when its containing path remains
semantically reachable.

```sec
if false {
    DangerousOperation()
}
```

does not add a reachable call relationship.

A branch proven impossible through:

```text
constant evaluation;
type refinement;
contract refinement;
target selection;
configuration selection;
exhaustive match analysis;
```

does not contribute reachable calls.

---

# Runtime conditions

A runtime condition that may evaluate either way contributes call sites from all
reachable branches.

The graph remains a may-execute graph.

Must-execute counts and path frequency belong to control-flow and quantitative
analysis.

---

# Unreachable code

Proven unreachable code is already a compiler error under Sec rules.

The call graph must not use erroneous unreachable code to introduce false
reachable edges into later analysis.

Diagnostics may still use the local code to explain the original unreachable
error.

---

# Recursion

Same-stack recursion is detected through strongly connected components over
same-stack execution relations.

Included relations normally include:

```text
Synchronous;
ImmediateCallback;
Deferred;
Destruction.
```

Excluded from same-stack recursion include:

```text
SpawnTask;
SpawnThread;
SpawnProcess;
InterruptEntry;
AwaitTask synchronization;
Join synchronization.
```

---

# Direct recursion

```sec
fn Walk(node: ref Node) void {
    Walk(node)
}
```

produces a self-cycle in the same-stack view.

---

# Indirect recursion

```text
A -> B -> C -> A
```

produces one same-stack strongly connected component.

The diagnostic reports the clearest cycle path.

---

# Recursion through function values

A function-value target set may create a recursion edge.

When a callable contract is open and reentry is not excluded, recursion freedom
cannot be proven solely from known targets.

---

# Reentry contracts

A callable contract may eventually express or imply:

```text
MayReenter;
NoReentry;
ClosedTargetSet.
```

Exact source syntax is defined elsewhere.

The call graph stores the semantic fact.

When recursion is disabled, unknown reentry is an error.

When recursion is permitted, unknown reentry may produce information or warning
according to profile policy.

---

# Recursive `defer`

A deferred body may create ordinary recursion.

```sec
fn Work() void {
    defer {
        Work()
    }
}
```

The synthetic defer node participates in the same-stack SCC.

---

# Recursive destruction

Destructor call chains participate in same-stack recursion analysis.

Implicit destruction must not hide a recursive cycle.

---

# Complete execution SCC

The compiler may compute an SCC over all execution relationships for analysis
and visualization.

This complete SCC is not used as ordinary stack recursion proof.

Spawn, thread, process, and interrupt cycles retain their execution-boundary
classification.

---

# Effect-analysis integration

Each callable node has:

```text
direct effect summary;
transitive effect summary;
declared guarantees;
trust provenance.
```

Effect propagation uses the call graph and execution relation.

---

# Same-stack effect transfer

For:

```text
Synchronous;
ImmediateCallback;
Deferred;
Destruction;
```

callee effects propagate into the caller's transitive summary.

---

# Spawn effect transfer

For task, thread, and process spawn:

```text
creation and scheduling effects propagate to the spawner;
body effects belong to the new execution context;
body effects remain reachable in the whole program;
body effects do not automatically become synchronous caller effects.
```

---

# Await effect transfer

`await` contributes its own semantic effects.

These may include:

```text
MaySuspend;
MayBlock according to backend/profile;
task outcome handling;
cancellation point behavior.
```

The spawned body's effects were already assigned to the task execution context.

---

# Interrupt effect closure

ISR analysis traverses the interrupt root through all reachable same-context
callable relationships.

It verifies:

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

# Stack-analysis integration

The graph provides:

```text
same-stack call relationships;
execution-context boundaries;
recursion SCCs;
indirect target sets;
open stack contracts;
root-to-node paths.
```

Node-local frame size comes from stack analysis and final target lowering.

---

# Same-stack stack accumulation

For a same-stack call path, maximum stack conceptually combines:

```text
caller frame;
callee path;
cleanup path;
defer and destructor paths;
target ABI contribution.
```

Indirect calls use the maximum known target or declared callable bound.

---

# Task and thread stacks

`SpawnTask` and `SpawnThread` begin separate stack analyses.

The caller's active stack is not naively added to the complete spawned stack.

The graph still retains parent and creation relationships for diagnostics and
resource analysis.

---

# Unknown stack contribution

A callable contract may declare a stack upper bound.

When no concrete targets or bound exist:

```text
permissive hosted profile:
    information or warning may be emitted

strict embedded profile:
    compile error when a complete stack proof is required
```

---

# Allocation-analysis integration

The graph supports:

```text
root-to-allocator paths;
allocation during initialization;
allocation after initialization;
allocation in ISR;
allocation in cleanup;
allocation in tasks or threads;
unknown allocation through open callable contracts.
```

Spawn-body allocation remains reachable program behavior.

It is not described as synchronous allocation by the spawner unless task
creation itself allocates.

---

# Panic-analysis integration

Panic-capable Semantic IR operations contribute direct panic effects.

A fictional panic node is not required.

When the compilation plan uses a real configured panic handler callable, the
graph may contain:

```text
panic operation -> panic handler
```

A target trap does not require a callable handler node.

---

# Unsafe-analysis integration

Unsafe status is part of the callable contract.

An unsafe callable target cannot silently convert to a safe callable contract.

The graph preserves:

```text
unsafe invocation requirement;
trusted foreign declaration provenance;
inline assembly provenance;
compiler intrinsic provenance;
safe wrapper boundary.
```

A safe wrapper is a safe callable node.

Its internal unsafe edge remains visible for auditing and diagnostics.

---

# Reference-model integration

The call graph does not duplicate `reference_model.md`.

Call-site metadata links to Semantic IR facts for:

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

Reference and escape analyses determine validity.

The call graph determines which callable bodies may execute and in which
execution relationship.

---

# Spawned borrows

A spawn call site records borrows retained by the spawned execution.

The borrow remains active until task/thread completion or another proven
lifecycle boundary.

`await` may establish the completion point where a task-retained borrow ends.

The call graph exposes the relationship.

Borrow analysis enforces validity.

---

# Callback retention

Callable contracts may distinguish:

```text
invoked only during the current call;
may be retained;
may be invoked asynchronously;
may be invoked concurrently.
```

The graph records the semantic callback relation.

Reference, closure, ownership, and escape analyses consume the retention facts.

A missing retention guarantee must not be interpreted as immediate invocation
when the API permits retention.

---

# Immediate callbacks

An immediate callback invocation is same-stack execution.

Its effects and stack contribution propagate normally.

---

# Retained callbacks

A retained callback does not necessarily execute during the retaining call.

The graph records:

```text
retention relation;
possible later callback entry;
execution contract;
lifetime dependency;
known or open target set.
```

The actual callback execution may become a derived root of an event, task,
thread, foreign, or target execution context.

---

# Foreign callbacks

A callback invoked by foreign code is a foreign-entry relationship.

It records:

```text
ABI;
foreign thread/task attachment assumptions;
callable contract;
unsafe provenance;
panic boundary;
ownership and retention contract.
```

---

# Interfaces and effect compatibility

Interface callable contracts retain declared effect guarantees.

Concrete implementations must satisfy those guarantees.

Known implementations may be stronger.

The open call site uses the interface contract.

---

# Separate compilation

Compiled module metadata contains enough callable information for callers to
analyze public relationships without reading all implementation source.

It should include:

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

Private implementation details may be included separately for LTO.

---

# Link-time refinement

Link-time or whole-program analysis may refine:

```text
open target sets;
function-value target sets;
interface implementations;
foreign adapter targets;
dead callable nodes.
```

Refinement may strengthen internal inferred facts.

It must not weaken declared public guarantees.

---

# Dead stripping

A callable not reachable from any active root may be removed from the binary
when linking and reflection rules permit it.

The compile-time graph may retain the node for diagnostics or incremental reuse.

No runtime graph is required.

---

# Incremental compilation

Stable node and call-site identities allow targeted invalidation.

When a node changes, the compiler invalidates at least:

```text
its direct call-site set;
its direct summaries;
its target-resolution facts;
its containing SCC;
transitive callers affected by changed facts;
derived effect, stack, allocation, ISR, panic, unsafe, and reachability facts.
```

A change to an open callable contract may invalidate external dependents.

---

# Graph update phases

A recommended incremental flow is:

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

# Semantic IR requirements

Semantic IR must preserve call-graph-relevant operations explicitly.

At minimum:

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

Exact IR names are implementation-defined.

The semantic distinctions are required.

---

# Spawn IR integration

Spawn IR already records or must record:

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

The call graph consumes these facts.

The backend must not rediscover them from low-level operations.

---

# Await IR integration

Await IR records:

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

The graph preserves synchronization and continuation relationships.

---

# Graph data model

A recommended model contains:

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

Exact implementation structure is not normative.

---

# Node record

Conceptual:

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

# Target set record

Conceptual:

```go
type TargetSet struct {
    KnownTargets []NodeID
    IsClosed     bool
    OpenContract *CallableContractID
}
```

A target set with `IsClosed == false` must have a valid open contract.

---

# Root record

Conceptual:

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

# Analysis views

The graph service should expose views such as:

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

Exact API naming is implementation-defined.

---

# Diagnostics

Diagnostics must retain complete cause paths.

A call-graph path includes:

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

# Direct caller diagnostic

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

# Interface diagnostic

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

# Spawn-cycle diagnostic

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

# Open-world root diagnostic

```text
information: function is analyzed as an open-world library entrypoint

callable:
    Parse

external callers may invoke this callable according to its public contract
```

This is normally an analysis report or LSP fact, not normal build noise.

---

# Diagnostic IDs

Call-graph diagnostics use stable IDs.

IDs identify the rule or compiler phase, not the current severity.

Suggested categories include:

```text
CGxxxx
```

Exact numbering is assigned by the canonical diagnostics registry.

---

# LSP behavior

The LSP should provide:

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

# Find callers

“Find callers” distinguishes:

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

# Hover example

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

# Graph visualization

Tooling may visualize:

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

Visualization is tooling.

It is not runtime metadata.

---

# No mandatory runtime

The call graph is compile-time metadata.

It requires no:

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

Runtime dispatch structures exist only when required by the actual language
semantics and selected lowering.

---

# Current implementation status

The canonical spawn and await rulebooks currently establish:

```text
spawn is an expression;
spawn defaults to task execution;
spawn task, spawn thread, and spawn process preserve requested execution kind;
spawn is eager;
the same callable may be synchronous or spawned;
task handles are move-only lifecycle owners;
await is task-specific in Sec 0.1;
await consumes Task[T];
join preserves the task handle;
await inside defer is invalid;
spawn and await require explicit Semantic IR representation.
```

Current front-end support already includes substantial parser and Sema support
for spawn and await.

The repository does not yet claim the complete canonical call graph defined by
this rulebook.

Codex must inventory existing graph, call-resolution, recursion, effect,
allocation, task, and stack structures before implementing a parallel system.

---

# Implementation scope

The first implementation should support:

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

# Deferred source syntax

This rulebook deliberately does not invent source syntax for:

```text
effect-constrained function types;
stack-bound callable contracts;
no-reentry callable contracts;
callback retention annotations;
foreign callback attachment;
process entry contracts.
```

The graph representation must permit these semantics when their source syntax is
defined.

---

# Required tests

Create or update:

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

# Node tests

Test:

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

# Call-site tests

Test:

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

# Spawn tests

Test:

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

# Await and join tests

Test:

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

# Indirect-call tests

Test:

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

# Mentor diagnostic tests

Test:

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

# Root tests

Test:

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

# Reachability tests

Test:

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

# SCC tests

Test:

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

# Analysis integration tests

Test:

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

# Incremental tests

Test:

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

# Binary tests

Verify:

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

# Required synchronization

This rulebook must remain synchronized with:

```text
effect_analysis.md
reference_model.md
unsafe.md
spawn.md
await.md
tasks.txt
threads.md
processes.txt
structured_concurrency.md
concurrency.md
concurrency_memory_model.txt
functions.txt
functions_lambda.txt
generics.txt
interfaces.txt
defer.txt
destruction.txt
panic.md
runtime_checks.md
allocation.txt
ownership.md
borrowing.txt
lifetime_analysis.txt
semantic_ir.txt
compiler_analysis.txt
compiler_pipeline.txt
projects.txt
attributes.md
diagnostics.txt
lsp.md
language-rulebook-status.md
rules_implementations.txt
```

---

# Rulebook inventory update

After adding this document:

```text
call_graph.md
    status becomes Written
```

Update:

```text
language-rulebook-status.md
rules_implementations.txt
```

`reference_model.md` must also be added to the canonical written inventory when
that repository synchronization has not already occurred.

A separate planned `generational_references.md` must not be added.

Generational validity is covered by:

```text
reference_model.md
```

---

# Appendix A — Codex implementation plan

## A.1 Add the rulebook

Add:

```text
rules/call_graph.md
```

Update:

```text
language-rulebook-status.md
rules/rules_implementations.txt
```

Mark the rulebook Written.

Do not mark the complete implementation finished.

---

## A.2 Inventory existing compiler structures

Locate existing structures for:

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

Reuse or migrate existing facts.

Do not build a disconnected second graph.

---

## A.3 Add stable identities

Define stable IDs for:

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

Avoid map-order and pointer-address identities.

---

## A.4 Add node registry

Create a central node registry keyed by semantic identity.

Support at least:

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

## A.5 Add call-site records

Every Semantic IR call-like operation produces one call-site record.

Store:

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

## A.6 Integrate target selection

Build the graph only after active:

```text
@target;
@when;
target profile;
device/board knowledge;
implementation variant.
```

Use one graph per concrete `CompilationPlan`.

---

## A.7 Direct target resolution

Add exact targets for:

```text
direct function calls;
static method calls;
resolved generated calls;
known foreign functions;
known intrinsics.
```

---

## A.8 Generic specialization nodes

Create semantic nodes for concrete specializations.

Do not merge them merely because backend code may be shared.

---

## A.9 Closure and lambda nodes

Create stable nodes for every callable closure/lambda body.

Connect capture and retention facts through Semantic IR references.

---

## A.10 Always create defer nodes

Create one synthetic callable node for every `defer` body.

Do not condition node creation on whether the body currently contains a call.

---

## A.11 Destructor nodes and implicit edges

Represent implicit destruction through visible destruction relationships.

Include them in same-stack SCC and effect analysis.

---

## A.12 Spawn relationships

Consume canonical spawn Semantic IR.

Represent:

```text
SpawnTask;
SpawnThread;
SpawnProcess.
```

Preserve:

```text
eager reachability;
parent execution context;
copy/move/borrow/capture facts;
result handle type;
requested execution kind.
```

---

## A.13 Await and join synchronization

Consume canonical await/join Semantic IR.

Represent:

```text
TaskAwait;
TaskJoin;
continuation;
task origin where known;
borrow completion boundary;
result ownership.
```

Do not create another call edge to the task body.

---

## A.14 Function-value target analysis

Perform sound value-flow analysis.

Produce:

```text
exact target;
closed finite set;
open callable contract;
known set plus open contract.
```

Never omit a possible target.

---

## A.15 Interface target analysis

Resolve known implementation targets.

Preserve open-world interface contract for library boundaries.

---

## A.16 Callable contracts

Add a normalized callable-contract structure.

At minimum:

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

Unknown concrete targets may use the contract.

Unknown contracts are errors.

---

## A.17 Root discovery

Discover:

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

Treat externally callable library declarations as `OpenWorldRoot`.

---

## A.18 Reachability

Compute reachability by root class.

Retain:

```text
which roots reach each node;
which execution relation enters it;
which CompilationPlan applies.
```

---

## A.19 Path pruning

Use mandatory semantic reachability proof to omit:

```text
constant-false branches;
inactive target variants;
inactive configuration variants;
proven impossible match branches.
```

Do not depend on optional optimizer passes.

---

## A.20 SCC views

Compute at least:

```text
same-stack SCC;
task-spawn SCC;
thread-start SCC;
process-launch SCC;
complete execution SCC.
```

Use same-stack SCC for recursion policy and stack analysis.

---

## A.21 Effect integration

Replace ad hoc effect-call propagation with graph-based fixed-point analysis.

Use execution-specific transfer rules.

---

## A.22 Stack integration

Expose same-stack targets and bounds.

Handle:

```text
closed target maximum;
open declared bound;
permissive unknown contribution;
strict-profile error.
```

---

## A.23 Allocation and panic paths

Expose complete root-to-cause paths for allocation and panic analysis.

---

## A.24 Reference and escape integration

Link call sites to Semantic IR argument, receiver, capture, retention, spawn,
await, and foreign-lifetime facts.

Do not duplicate reference-model state inside graph edges.

---

## A.25 Unsafe provenance

Preserve trusted edges through safe wrappers.

Expose audit paths without marking safe callers as unsafe.

---

## A.26 Module metadata

Emit public callable contracts and summaries required for separate compilation.

Keep inferred implementation details separate from declared guarantees.

---

## A.27 Incremental invalidation

Build dependency indexes:

```text
node -> callers;
contract -> call sites;
target set -> SCC;
root -> reachable nodes;
summary -> dependent analyses.
```

Invalidate only affected regions where sound.

---

## A.28 Diagnostics

Implement consequence-based severity:

```text
none;
information;
warning;
error.
```

Use stable IDs.

Include cause path and actionable help.

---

## A.29 LSP

Add:

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

## A.30 Tests

Run:

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

Do not claim complete implementation while indirect calls, roots, spawn/await,
defer, destruction, or SCC views remain partial.

---

# Appendix B — Canonical node table

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

# Appendix C — Canonical dispatch table

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

# Appendix D — Canonical execution-relation table

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

# Appendix E — Canonical indirect-call outcome table

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

# Final canonical summary

Sec builds one canonical semantic call graph per concrete `CompilationPlan`.

Callable nodes represent concrete execution units.

Call sites retain source identity, dispatch kind, execution relation, target set,
callable contract, unsafe requirement, and trust provenance.

Dispatch and execution are independent properties.

Same-stack calls, defer, and destruction participate in ordinary recursion and
stack analysis.

Every `defer` body always receives a synthetic callable node.

`spawn` is already defined by the canonical spawn rulebook.

It creates an eager task by default and preserves explicit task, thread, and
process execution kinds.

Spawn targets become reachable when spawned, not when awaited.

`await` is task-specific in Sec 0.1, consumes `Task[T]`, synchronizes with task
completion, and resumes through an explicit continuation.

`await` does not call the task body again.

`join` synchronizes while preserving the handle.

Task, thread, and process cycles are distinct from same-stack recursion.

Unknown concrete targets are permitted when a valid callable contract exists.

An unknown callable contract is a compile error.

Open-world library declarations are `OpenWorldRoot` nodes.

Interface and function-value calls retain known targets plus open contracts.

Conservative uncertainty produces no diagnostic, information, warning, or
error according to its actual consequence.

Declared guarantees and strict profiles require proof.

Call-graph diagnostics explain what is unknown, why it matters, the conservative
assumption, the complete path, and how to strengthen the contract.

Compiler-generated helpers may use intrinsic summaries only when they are
semantically atomic for every relevant analysis.

The call graph coordinates effects, stack, allocation, panic, ISR, unsafe,
reference, ownership, escape, task, thread, and process analysis.

The call graph introduces no mandatory runtime.
