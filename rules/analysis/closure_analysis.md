# Closure Analysis

## Status

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the semantic model, analysis domain, callable-value flow,
closure-environment dependencies, control-flow analysis baseline, analysis
budgets, interprocedural summaries, diagnostics requirements, tooling
integration, and completion criteria for Sec closure analysis.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.

---

# Purpose

Closure analysis determines:

- what a closure captures;
- how captured values enter the closure environment;
- what semantic dependencies the closure environment carries;
- which callable bodies a function-valued value may invoke;
- how callable values flow through control flow, Places, aggregates, calls, and
  returns;
- what symbolic callable relationships must survive interprocedural and
  separate-compilation boundaries;
- what callable and environment facts other analyses may safely consume.

Closure analysis is broader than lambda syntax.

It covers three closely related analysis responsibilities:

```text
CaptureAnalysis
ClosureLifetimeAnalysis
CallableFlowAnalysis
```

The three responsibilities share the same callable-value and Place/provenance
infrastructure and therefore belong to one normative analysis model.

Closure analysis does not choose physical closure representation, storage
placement, heap allocation, ABI representation, or backend calling convention.

---

# Normative role

This rulebook is normative for:

- the distinction between callable body, function value, closure instance, and
  closure environment;
- capture analysis for Sec 0.1 value captures;
- closure-environment dependency propagation;
- callable target-set analysis;
- the required monovariant higher-order control-flow baseline;
- callable flow through assignments, branches, loops, Places, aggregates,
  parameters, returns, and calls;
- interprocedural callable and closure summaries;
- recursion and fixed-point behavior;
- bounded precision and conservative widening;
- Interactive, Standard, and Deep analysis budgets as they apply to closure and
  callable-flow analysis;
- separate-compilation summary requirements;
- diagnostic facts and cause paths;
- compiler-analysis consumers;
- LSP and `sec analyse` integration requirements;
- required test classes;
- completion criteria for Sec 0.1 closure analysis.

This rulebook does not redefine:

- lambda syntax;
- function-type syntax or type identity;
- overload resolution;
- ownership legality;
- copy or move legality;
- borrow legality;
- object lifetime;
- storage lifetime;
- storage origin or reclamation authority;
- escape semantics;
- callable effect semantics;
- task/thread ownership or synchronization rules;
- FFI callback syntax or foreign retention syntax;
- physical closure layout;
- source-level ABI.

Those semantics remain owned by their respective normative rulebooks.

---

# Relationship to existing language rules

`rules/declarations/lambda-functions.md` defines the source-language
function-value, callable-capability, and lambda semantics consumed by this
analysis.

In Sec 0.1:

- lambdas are anonymous `fn` values;
- named functions may be used as function values when overload resolution is
  unambiguous;
- function values may be passed, stored, returned, and called;
- a function value is never null;
- optional absence is represented explicitly with `Option[fn(...)]`;
- enclosing locals are never captured implicitly;
- `capture(...)` explicitly identifies captured values;
- capture modes are explicit value, forced-consuming value, shared reference,
  and mutable reference;
- owned capture bindings are mutable inside the closure, independently of the
  mutability of the outer binding;
- callable environment capability is inferred as `fn`, `mut fn`, or `-> fn`;
- capture expressions are evaluated exactly once;
- capturing and non-capturing callables with the same parameter and return
  types have the same source-level function type;
- no hidden cloning, ownership transfer, or heap allocation is permitted.

Closure analysis consumes those rules and must not invent additional source
syntax or capture semantics.

---

# Relationship to other analyses

Closure analysis is intentionally separated from escape, lifetime, ownership,
storage, call-graph, and effect analysis.

Conceptually:

```text
Closure analysis:
    What does this callable capture?
    What environment does it depend on?
    What callable bodies may this function value invoke?

Escape analysis:
    Where does the callable or environment dependency travel?

Lifetime analysis:
    Do the environment and captured dependencies remain valid long enough?

Ownership analysis:
    Was capture transfer, callable copy, or callable move legal?

Storage analysis:
    What storage and invalidation domains do dependencies require?

Call graph:
    What may execute from each call site?

Effect analysis:
    What effects may occur through the possible callable targets/contracts?
```

The analyses cooperate through explicit facts and shared canonical identities.
They must not reimplement one another's semantic models.

---

# Core semantic entities

Closure analysis distinguishes four concepts:

```text
CallableBody
FunctionValue
ClosureInstance
ClosureEnvironment
```

They are not interchangeable.

## Callable body

A `CallableBody` is the executable semantic body selected by a callable value.

Examples include:

```text
NamedFunction
NonCapturingLambdaBody
CapturingLambdaBody
ForeignCallable
CompilerKnownCallable
```

A lambda expression has one stable callable-body identity for a concrete
CompilationPlan and specialization, regardless of how many times the lambda
expression executes at runtime.

Example:

```sec
fn Make(value: int) fn() int {
    return capture(value) fn() int {
        return value
    }
}
```

The lambda body has one callable-body identity.

Two calls to `Make` may create different runtime closure instances.

---

## Function value

A `FunctionValue` is a source-level value whose type is a Sec function type.

Example:

```sec
fn(int) int
```

A function value may refer to:

- a named function;
- a non-capturing lambda;
- a capturing closure.

Function type identity does not include:

```text
capture shape
closure-environment type
closure creation site
runtime closure instance identity
concrete target identity
```

Therefore:

```text
function type != capture shape
function type != environment type
function type != concrete target identity
```

---

## Closure instance

A `ClosureInstance` is a runtime callable instance produced by evaluating a
capturing lambda expression.

Example:

```sec
fn Make(value: int) fn() int {
    return capture(value) fn() int {
        return value
    }
}

let first := Make(10)
let second := Make(20)
```

Conceptually:

```text
CallableBody:
    same

ClosureInstance:
    distinct

ClosureEnvironment:
    distinct
```

Analysis abstraction must not imply that distinct runtime closure instances are
the same runtime object.

---

## Closure environment

A `ClosureEnvironment` is the semantic carrier of the values captured by one
closure instance.

Conceptually:

```text
ClosureEnvironment {
    capture[0]
    capture[1]
    ...
}
```

The environment is not a source-visible struct.

Closure analysis defines semantic environment dependencies.

Backend representation, field layout, hidden lowering parameters, and storage
placement are separate lowering concerns.

---

# Capturing and non-capturing callables

## Named functions

A named function used as a value has:

```text
Target = exact resolved callable body
Environment = None
```

Overload resolution is completed before closure analysis consumes the callable
identity.

---

## Non-capturing lambdas

A non-capturing lambda has:

```text
Target = exact lambda body
Environment = None
```

No closure-environment dependency is required merely because the source syntax
uses an anonymous function.

A compiler may still materialize a callable runtime representation when later
lowering or ABI rules require one.

---

## Capturing lambdas

A capturing lambda has:

```text
Target = exact lambda body
Environment = known closure environment
```

Invocation of the callable semantically requires the corresponding environment
to remain valid.

The source-level function type remains unchanged.

The environment is not a source parameter and must not affect source parameter
count, overload resolution, or function-type identity.

---

# Capture model

## Captures are explicit

Closure analysis must never infer an undeclared capture merely because the
lambda body refers to an outer local.

A valid capture is introduced by the source-language capture clause.

Example:

```sec
let factor := 2

let multiply := capture(factor) fn(value: int) int {
    return value * factor
}
```

Use of an enclosing local without explicit capture is invalid according to the
lambda rulebook.

Closure analysis validates and classifies captures. It does not invent them.

---

## Capture analysis uses canonical Places

Capture analysis operates on canonical Place/provenance information where the
capture source has an addressable Place identity.

Relevant capture sources may include, where source syntax and ownership rules
permit them:

```text
local binding
parameter
receiver-derived value
field projection
array element
slice/view value
union payload value
function-valued Place
reference-typed value
handle-typed value
```

Closure analysis must reuse canonical Place identity and projection precision.
It must not construct a separate symbol-only alias model.

---

# Capture record

Each capture must be representable conceptually as:

```text
CaptureRecord {
    SourcePlace
    CapturedType
    CaptureMode
    Transfer
    Dependencies
}
```

Exact compiler data structures are implementation-defined.

## SourcePlace

`SourcePlace` identifies the canonical source Place where such identity exists.

For captured expressions without a stable addressable Place, the compiler must
retain equivalent source/provenance identity sufficient for ownership,
lifetime, diagnostics, and dependency analysis.

## CapturedType

`CapturedType` is the resolved semantic type of the captured value.

## CaptureMode

Sec 0.1 defines:

```text
Value
ForcedConsume
SharedReference
MutableReference
```

These correspond to `capture(value)`, `capture(-> value)`,
`capture(ref value)`, and `capture(ref mut value)`. Capture mode is explicit;
closure analysis must not infer reference capture from the captured value's
type or outer-binding mutability.

## Transfer

The mechanism by which the value enters the environment is distinct from
capture mode.

Conceptually:

```text
Copy
Move
Unknown
```

The transfer is determined by normal Sec ownership and copy/move semantics.

Closure analysis records the resulting fact but does not independently decide
whether the copy or move is legal.

## Dependencies

A captured value may carry dependencies such as:

```text
referent lifetime
backing storage
storage domain
generation
owned resource
nested closure environment
callable contract
```

Value capture does not erase those dependencies.

---

# Value capture semantics

## Snapshot at closure creation

A value capture observes and transfers the value at closure creation.

Example:

```sec
let mut factor := 2

let operation := capture(factor) fn(value: int) int {
    return value * factor
}

factor = 10
```

The closure continues to observe the captured value `2`.

Conceptually:

```text
Value capture
    = value transfer at closure creation
    != alias to the outer binding
```

---

## Captured binding is distinct

After capture, the outer binding and the environment binding are distinct
semantic bindings.

Conceptually:

```text
outer factor
environment.capture[0]
```

If transfer was `Copy`, both values remain available according to normal
ownership rules.

If transfer was `Move`, the outer source is moved according to normal ownership
rules.

The name used inside the lambda denotes the captured environment binding, not a
continued access to the outer binding.

---

## Owned capture bindings are mutable

An owned value capture is a mutable closure-local binding regardless of whether
the outer binding was mutable. Mutating it requires at least `mut fn` callable
capability.

`capture(-> value)` differs only in forcing the creation-time transfer to be a
move even when the captured type is copyable. It does not introduce a distinct
binding-mutability class.

Closure analysis records environment mutation and consumption so callable
capability inference can distinguish reusable shared, reusable mutable, and
consuming callables.

---

# Reference-typed values versus reference capture

Capturing a value whose type is itself reference-like is still a Sec 0.1 value
capture.

Example:

```sec
let itemRef: ref Item := ref item

let operation := capture(itemRef) fn() int {
    return itemRef.value
}
```

Conceptually:

```text
CaptureMode = Value
```

The reference value is transferred into the environment.

Its referent dependency remains attached to the captured value.

Therefore the closure environment may depend on:

```text
referent object lifetime
referent storage lifetime
storage domain/generation
borrow validity
```

as applicable.

This is distinct from explicit reference-capture syntax such as:

```sec
capture(ref item) fn() int {
    return item.value
}
```

which establishes a borrow directly from another Place. Shared and mutable
reference captures retain the normal borrow, lifetime, storage, and provenance
dependencies of that Place. A mutable-reference capture is move-only and
requires at least `mut fn` callable capability.

---

# Views and slices

A captured slice or view is a value capture of its descriptor/value semantics.

Its backing-storage dependency remains attached.

Example:

```sec
let view := data[10..20]

let operation := capture(view) fn() int {
    return view[0]
}
```

Closure analysis must preserve the relationship:

```text
ClosureEnvironment
    -> captured view
    -> backing storage dependency
```

Value capture does not make the backing storage independently owned.

---

# Captured function values

Function values may themselves be captured.

Example:

```sec
fn Wrap(operation: fn(int) int) fn(int) int {
    return capture(operation) fn(value: int) int {
        return operation(value)
    }
}
```

The outer closure environment owns the captured function value according to
normal copy/move semantics.

The captured function value may itself carry:

```text
callable target set
callable contract
nested closure-environment dependency
```

Closure analysis must preserve these transitively.

Nested closure environments are therefore valid semantic dependency graphs.

---

# Closure environment as dependency carrier

A closure environment is a structural dependency carrier.

Conceptually:

```text
Environment(S) {
    capture[0] -> dependencies A
    capture[1] -> dependencies B
}
```

If the callable value that depends on the environment escapes, the relevant
environment dependencies are visible to escape analysis.

Capture creation alone does not imply escape.

---

# Capture and escape are distinct

A closure may be created and used entirely within its original context.

Example:

```sec
let operation := capture(factor) fn(value: int) int {
    return value * factor
}

return operation(10)
```

The environment exists, but the callable value need not escape the function.

Conceptually:

```text
capture != escape
```

Closure analysis records environment dependencies.

Escape analysis determines where the callable/environment dependency travels.

Lifetime analysis determines whether those dependencies remain valid for the
required destination lifetime.

---

# Closure escape and storage

Closure analysis may derive facts such as:

```text
EnvironmentRequiresEscape
RequiredEnvironmentLifetime
EnvironmentDependencies
```

It must not derive a physical allocation decision merely from escape.

In particular:

```text
closure environment escapes
```

does not imply:

```text
allocate environment on heap
```

No implicit heap promotion is permitted.

No hidden allocation may be introduced merely to repair an otherwise invalid
closure lifetime.

If no valid source-semantic ownership/storage strategy exists, the program is
invalid rather than silently promoted.

Optimization may eliminate or transform an environment only when all source
semantics and required dependencies remain preserved.

---

# Callable-flow analysis

Callable-flow analysis determines a sound may-target set for function-valued
values at relevant program points.

It is the callable-specific higher-order control-flow analysis used by Sec.

The analysis tracks callable values through:

```text
assignment
copy
move
control-flow merge
loops
parameters
returns
aggregate fields
arrays and collections
unions and options
calls
closure captures
```

It reuses canonical Place/provenance and aggregate-flow infrastructure.

---

# Control-flow-analysis baseline

Sec callable-flow analysis belongs to the established control-flow-analysis
family for higher-order programs.

The required baseline is a sound monovariant analysis equivalent in purpose to
0-CFA, adapted to Sec's:

- static function types;
- explicit captures;
- canonical Place/provenance model;
- ownership information;
- symbolic interprocedural summaries;
- callable contracts;
- concrete CompilationPlan.

The rulebook does not require classical k-CFA for `k > 0`.

Implementations may provide stronger sound refinements, including:

```text
context-sensitive CFA
selective polyvariance
call-string sensitivity
closure/environment sensitivity
pushdown refinement
other sound target refinements
```

Such refinements improve precision. They do not define different source
semantics.

The required baseline is monovariant, not intentionally imprecise.

Cheap Sec-specific symbolic relations may preserve more precision than a naive
classical 0-CFA implementation without violating the baseline model.

---

# Callable target sets

Closure analysis uses the canonical call-graph target-set model.

Conceptually:

```text
TargetSet {
    KnownTargets
    IsClosed
    OpenContract
}
```

The following semantic forms are useful descriptions of that model:

```text
Exact
ClosedSet
OpenContract
KnownSetPlusOpenContract
```

They do not require a second competing representation.

## Exact

An exact target is a closed target set containing one callable body.

```text
KnownTargets = { A }
IsClosed = true
OpenContract = none
```

## ClosedSet

A closed finite set contains all possible concrete targets.

```text
KnownTargets = { A, B, C }
IsClosed = true
OpenContract = none
```

## OpenContract

Concrete targets are not fully known, but all legal unknown targets must satisfy
an explicit callable contract.

```text
KnownTargets = {}
IsClosed = false
OpenContract = C
```

## KnownSetPlusOpenContract

Some concrete targets are known and additional targets remain permitted by the
open contract.

```text
KnownTargets = { A, B }
IsClosed = false
OpenContract = C
```

A target set with `IsClosed == false` must have a valid open callable contract
where semantic consumers require guarantees about the unknown targets.

---

# Soundness of target sets

Callable target sets are may-target over-approximations.

Every callable body that may actually execute at runtime must be represented by
one of:

- a known concrete target;
- a sound open callable contract covering that target.

It is permitted for a bounded analysis to temporarily include concrete targets
that cannot execute on a particular runtime path.

It is not permitted to omit a possible runtime target in order to gain
precision or optimization.

---

# Function type versus callable contract

Function type compatibility does not imply identical callable behavior.

A function type such as:

```sec
fn(Request) Response
```

specifies source-level parameter and return types.

It does not by itself establish guarantees such as:

```text
NoAllocate
NoPanic
NoBlock
NoRetain
NoSpawn
stack bound
reentry behavior
foreign trust
```

Callable contracts are separate compiler-visible semantic contracts.

Closure analysis propagates those contracts where concrete targets are open or
unknown.

It does not redefine the effect, retention, stack, or reentry semantics stored
inside the contract.

---

# Callable creation

## Named function value

After overload resolution:

```sec
let operation: fn(int) int := Transform
```

produces conceptually:

```text
TargetSet = Exact(Transform)
Environment = NoEnvironment
```

---

## Non-capturing lambda

```sec
let operation := fn(value: int) int {
    return value * 2
}
```

produces conceptually:

```text
TargetSet = Exact(lambda-body-id)
Environment = NoEnvironment
```

---

## Capturing lambda

```sec
let operation := capture(factor) fn(value: int) int {
    return value * factor
}
```

produces conceptually:

```text
TargetSet = Exact(lambda-body-id)
Environment = Environment(creation-site)
```

The environment carries the capture records defined by this rulebook.

---

# Abstract closure identity

The baseline analysis may abstract all runtime closure instances created at one
closure creation site into one finite abstract environment identity.

Conceptually:

```text
all runtime environments created at site S
    -> AbstractEnvironment(S)
```

This is an analysis abstraction only.

It must not be interpreted as runtime object identity.

If a closure is created repeatedly in a loop or recursion, different runtime
instances may contain different captured values even when the baseline analysis
joins them into one abstract environment state.

---

# Callable flow through assignment

Assignment updates the current callable-flow state according to normal Sec
value semantics.

Example:

```sec
let mut operation: fn(int) int := A
operation = B
```

After the second assignment:

```text
Targets(operation) = Exact(B)
```

The previous target `A` is not retained merely because it was historically held
by the same variable.

Historical escape or effect facts owned by other analyses remain separate.

---

# Callable flow through Copy and Move

Callable copy and move preserve the semantic relationship between:

```text
TargetSet
CallableContract
EnvironmentDependency
```

according to normal Sec ownership rules.

Closure analysis must not infer callable copyability from backend representation
such as a pair of pointers.

When a callable move is legal, the callable target/environment relation follows
the destination and the source ownership state changes according to ownership
analysis.

When callable copy is legal, the resulting environment ownership/sharing
semantics are whatever the callable ownership rules establish.

Closure analysis must not silently duplicate an owned closure environment.

---

# Control-flow join

Control-flow joins conservatively combine possible callable targets and
contracts.

Example:

```sec
let mut operation: fn(int) int := A

if condition {
    operation = B
}
```

After the branch:

```text
Targets(operation) = ClosedSet { A, B }
```

If one predecessor contains a closed concrete set and another contains an open
contract, the join preserves known concrete targets and the open component.

Conceptually:

```text
Exact(A)
join
OpenContract(C)

=> KnownSetPlusOpenContract { A, C }
```

Known concrete information must not be discarded merely because an open
component also exists.

---

# Places and aggregates

Callable target facts attach to function-valued values and Places.

When Place analysis proves disjoint structural locations, callable-flow analysis
must reuse that precision.

Example:

```sec
type Handlers struct {
    Open: fn(Event) void
    Close: fn(Event) void
}
```

If:

```sec
handlers.Open = OnOpen
handlers.Close = OnClose
```

then a call through `handlers.Open` need not include `OnClose` merely because
both values reside in the same aggregate.

---

# Arrays and collections

Function values stored in fixed arrays or collections carry target sets like
other values.

Constant-index precision should be preserved where canonical Place analysis
already provides it.

Example:

```sec
handlers[0] = A
handlers[1] = B
```

A call through `handlers[0]` may remain exact.

A call through a dynamic index may join possible element target sets.

Large or dynamically shaped containers may use bounded summarized element facts
rather than one analysis node per possible runtime element.

Compiler-known collection summaries may describe callable propagation without
requiring full shape analysis for every library collection.

---

# Options and unions

Optional callable absence is represented by `Option`, not by a null callable
target.

For:

```sec
Option[fn(Event) void]
```

callable target facts belong to the `Some` payload.

`None` is not a pseudo callable target.

The same active-payload principle applies to unions and other tagged aggregate
forms.

When active variant information is lost, possible callable payloads are joined
conservatively.

---

# Callable parameters

Function-valued parameters are represented symbolically in interprocedural
summaries.

Example:

```sec
fn Apply(operation: fn(int) int, value: int) int {
    return operation(value)
}
```

The analysis may represent:

```text
CallableParameter(0): Invoked
```

without knowing the concrete target of `operation` while analyzing `Apply`.

If an explicit callable contract exists, that contract is associated with the
symbolic callable parameter.

---

# Returned callable relationships

Callable return summaries preserve symbolic relationships.

Example:

```sec
fn Identity(operation: fn(int) int) fn(int) int {
    return operation
}
```

Summary:

```text
ReturnedCallable = CallableParameter(0)
```

A caller passing exact target `A` may therefore instantiate:

```text
Targets(result) = Exact(A)
```

without context-sensitive reanalysis of `Identity`.

---

# Disjunctive callable returns

Example:

```sec
fn Select(
    first: fn(int) int,
    second: fn(int) int,
    chooseFirst: bool,
) fn(int) int {
    if chooseFirst {
        return first
    }

    return second
}
```

The summary may preserve:

```text
ReturnedCallable = CallableParameter(0) OR CallableParameter(1)
```

Call-site instantiation may then produce an exact finite target set.

This symbolic relation is permitted in the required monovariant baseline.

---

# Returning newly created closures

Example:

```sec
fn Make(offset: int) fn(int) int {
    return capture(offset) fn(value: int) int {
        return value + offset
    }
}
```

The callable summary must be able to express conceptually:

```text
ReturnedCallable.Target = Exact(lambda-body-id)
ReturnedCallable.Environment = Environment(lambda-creation-site)
Environment.capture[0] derives from Parameter(0)
```

Escape analysis separately records that the returned callable/environment leaves
the function.

No physical allocation choice follows from the callable summary itself.

---

# Callable forwarding

Interprocedural callable flow must preserve forwarding relationships.

Example:

```sec
fn Outer(operation: fn(int) int, value: int) int {
    return Inner(operation, value)
}
```

If `Inner` invokes its first callable parameter, that fact must propagate through
`Outer` according to the callee summary.

Returned forwarding behaves similarly.

Example:

```sec
fn Outer(operation: fn(int) int) fn(int) int {
    return Inner(operation)
}
```

If:

```text
Inner.ReturnedCallable = CallableParameter(0)
```

then `Outer` may derive the same symbolic relationship.

---

# Capture forwarding

A callable parameter may itself be captured by a newly created closure.

Example:

```sec
fn Wrap(operation: fn(int) int) fn(int) int {
    return capture(operation) fn(value: int) int {
        return operation(value)
    }
}
```

Closure analysis must preserve two different relationships:

```text
returned callable body
    = Wrap's lambda body

returned environment
    depends on CallableParameter(0)
```

Inside the lambda body:

```text
invoked callable
    derives from environment capture[0]
```

Callable target identity and closure-environment dependency must not be
collapsed into one fact.

---

# Callable invocation event

Calling a function value is an explicit semantic analysis event.

Conceptually:

```text
CallableInvocation {
    CallSite
    TargetSet
    CallableContract
    EnvironmentDependency
}
```

Exact data structures are implementation-defined.

The event provides canonical input to:

- call graph;
- effect analysis;
- stack analysis;
- ISR analysis;
- diagnostics;
- LSP tooling.

The hidden environment argument used by lowering is not a source-level
parameter and is not required to appear in this semantic record as a fake source
argument.

---

# Callable-flow summaries

Each analyzable callable may expose a composable summary conceptually equivalent
to:

```text
CallableFlowSummary {
    CallableParameters
    ReceiverCallableFacts
    ReturnedCallables
    CallableInvocations
    CallableForwarding
    CreatedClosures
}
```

For created closure sites, the summary may reference:

```text
ClosureSummary {
    BodyIdentity
    CaptureDependencies
    CaptureTransfers
    CallableContract
}
```

Exact compiler structure is not normative.

Summaries must preserve externally relevant symbolic relationships without
exporting irrelevant internal temporaries.

---

# Escape and ownership are not duplicated in callable summaries

Callable-flow summaries must not become duplicate escape or ownership
rulebooks.

Example:

```text
Closure analysis:
    CallableParameter(0)
        -> Environment(S).capture[0]

Escape analysis:
    Environment(S)
        -> escapes through return
```

Those facts compose without closure analysis redefining `Returned`, `Retained`,
or other escape classes.

Similarly:

```text
CaptureTransfer = Move
```

is a closure-analysis fact, while ownership analysis determines whether the move
is legal and how source state changes.

---

# Monotone fixed-point analysis

Callable-flow analysis is a monotone fixed-point analysis.

Control-flow joins add possible targets/contracts or conservatively widen them.

A possible callable target must never disappear merely because a later
analysis iteration sees a narrower path unless the analysis is performing a
separate sound refinement under a stronger context abstraction.

Standard fixed-point results must be independent of source declaration order.

---

# Loops

Loops are analyzed to a sound fixed point.

Example:

```sec
let mut operation: fn(int) int := A

for item in items {
    if item.Enabled {
        operation = B
    }
}
```

A sound post-loop result includes:

```text
Targets(operation) = { A, B }
```

unless control-flow facts prove one alternative unreachable.

Analyzing the loop body exactly once is not sufficient.

---

# Recursive callable summaries

Direct and mutually recursive callable-summary dependencies are solved to a
sound fixed point.

The compiler should reuse canonical call-graph strongly connected components
where available, or use an equivalent declaration-order-independent algorithm.

This includes recursive higher-order forwarding relationships.

Example conceptual cycle:

```text
A returns callable from B
B forwards callable into C
C returns callable relation back to A
```

The analysis must terminate and produce a conservative result.

---

# Closure creation in loops and recursion

A closure creation site executed repeatedly may produce arbitrarily many runtime
closure instances.

The baseline analysis may represent them with one finite abstract environment
identity per creation site.

Captured state associated with that abstract environment is joined
conservatively across possible runtime instances.

The analysis must preserve:

```text
one callable body
potentially many runtime closure instances
potentially different captured values
```

It must not infer shared runtime identity merely because one abstract analysis
node is used.

---

# Bounded precision

Callable-flow analysis may bound:

- number of known concrete targets;
- number of disjunctive symbolic callable origins;
- projection depth;
- nested environment-dependency depth;
- summary complexity.

The exact budgets are implementation/configuration choices unless another
normative rule requires a minimum.

When a budget is exhausted, analysis must widen conservatively.

---

# Preferred callable widening

Callable contracts are the preferred semantic widening boundary when concrete
target identity is not required for the current proof.

Conceptually:

```text
large ClosedSet
    -> KnownSetPlusOpenContract
    -> OpenContract
```

when a valid covering contract exists.

The compiler must not replace an oversized target set with an unsound empty set
or with a contract that provides guarantees not shared by every covered target.

Deep analysis may retain or recover more concrete targets when useful.

---

# Structured unknown states

Closure analysis distinguishes at least:

```text
NoEnvironment
KnownEnvironment
UnknownEnvironment

KnownTargets
OpenContract
UnknownContract

KnownCaptureDependencies
UnknownCaptureDependencies
```

`NoEnvironment` is a positive proof.

Absence of environment information is not `NoEnvironment`.

Unknown target, unknown contract, or unknown environment must never be treated
as proof of:

```text
NoCapture
NoEscape
NoRetention
NoEffect
NoAllocation
NoBlock
```

Unknown information should remain as local and structured as practical.

---

# Analysis budgets

Sec distinguishes three analysis budgets:

```text
Interactive
Standard
Deep
```

These budgets control precision, recomputation strategy, diagnostics depth, and
optional analysis work.

They do not define different source-language semantics.

---

# Interactive analysis

Interactive analysis is intended primarily for LSP/editor use.

It prioritizes low latency through mechanisms such as:

```text
incremental recomputation
cached summaries
bounded target sets
bounded origin/cause-path depth
monovariant callable flow
```

Interactive analysis must remain sound for any result presented as a proof.

While affected analysis is being recomputed, tooling may expose an internal or
UI state equivalent to:

```text
Pending
```

rather than claiming an outdated or incomplete result is proven.

`Pending` is a tooling state and is not a valid compiler proof result.

---

# Standard analysis

Standard analysis is the analysis budget used by normal compilation for closure
and callable-flow facts required by language correctness and the active
CompilationPlan.

It must perform all closure/callable proofs required for normal Sec validity.

A user must not need to run `sec analyse` to make an otherwise ordinary valid
program semantically safe.

If a specific language feature or CompilationPlan requires a stronger proof,
the necessary analysis becomes required for that build rather than remaining an
optional Deep-only feature.

Standard analysis must be deterministic for the same:

```text
source
CompilationPlan
compiler version
analysis configuration
```

Result validity must not depend on compiler worklist order, worker scheduling,
or nondeterministic cache order.

---

# Deep analysis

Deep analysis provides optional expensive refinement.

It may use:

```text
context-sensitive callable flow
selective polyvariance
larger target/origin budgets
deeper environment propagation
more expensive whole-program propagation
extended cause paths
additional advisories
resource and optimization analysis
```

Deep analysis must preserve the same source-language semantics.

It may refine:

```text
Standard: OpenContract(C)
Deep: ClosedSet { A, B }
```

or:

```text
Standard call-site targets: { A, B, C }
Deep call-site targets: { B }
```

when the refinement is sound.

Deep analysis must never make an unsafe program appear safe by applying a
different language definition.

---

# Required versus optional analysis

Analysis required for language safety or correctness cannot be disabled when the
relevant language feature is used.

An expensive precision refinement may be optional when the required baseline can
remain sound through conservative facts or callable contracts.

Therefore:

```text
required analysis
    cannot be disabled when needed

optional precision
    may be configured
```

Disabling an optional refinement must never cause a possible unsafe behavior to
be classified as impossible.

---

# Cross-analysis fixed points

Closure/callable flow may participate in a compiler-wide fixed-point scheduler
with analyses that consume and refine related facts.

Typical dependency relationships include:

```text
callable flow
    -> call graph
    -> callee summaries
    -> escape/effect/stack facts
    -> refined contracts/target information
```

A compiler may iterate these analyses where required.

Each analysis must still expose a separately defined input/output contract.

The implementation must not replace the analysis architecture with one opaque
undocumented global fixed point.

---

# Termination

All analysis budgets must terminate for a finite program.

Therefore:

- callable target abstractions must be finite or widen;
- abstract environment identities must be finite;
- recursive symbolic expressions must be bounded or widen;
- interprocedural summary lattices must converge;
- Deep analysis may be expensive but must not be unbounded.

Termination is a normative compiler requirement.

---

# Callable contracts and unknown concrete targets

Unknown concrete target and unknown callable contract are distinct states.

A concrete target may be unknown while a valid open callable contract still
provides the guarantees required by safety analysis.

Conceptually:

```text
UnknownConcreteTarget + KnownContract
```

may be sufficient.

By contrast:

```text
UnknownConcreteTarget + UnknownContract
```

cannot provide safety guarantees that depend on target behavior.

Where a consumer requires guarantees such as non-retention, non-blocking,
non-allocation, non-panic behavior, or stack bounds, a missing contract must be
handled conservatively and may make the program invalid for that context.

---

# Separate compilation

Closure analysis must remain sound when a callee body is unavailable.

Persisted compiler summaries or explicit callable contracts provide the
necessary information.

At minimum, separate-compilation metadata must be able to preserve externally
relevant facts equivalent to:

```text
function identity
function signature
callable parameter behavior
invoked callable inputs
returned callable relationships
callable forwarding
created closure summaries where externally relevant
open callable contracts
summary version
```

Implementation-specific local temporaries need not be exported.

---

# Symbolic relations across module boundaries

Persisted summaries must preserve useful symbolic relations rather than
collapsing them unnecessarily to `Unknown`.

Examples include:

```text
returns CallableParameter(0)
returns CallableParameter(0) OR CallableParameter(1)
invokes CallableParameter(2)
returned closure environment depends on CallableParameter(0)
```

This allows callers to instantiate imported summaries with concrete caller
facts.

---

# Summary metadata is compiler metadata

Ordinary Sec source does not need to spell compiler-derived relations such as:

```text
ReturnsCallableParameter(0)
```

for analyzable Sec bodies.

Explicit contracts are primarily required where the implementation body cannot
be analyzed, such as:

- foreign code;
- opaque imported code;
- external callbacks;
- compiler-known boundaries;
- other deliberately open-world boundaries.

---

# Summary versioning

Persisted closure/callable summaries require an internal schema/version identity.

An incompatible summary must never be interpreted using new semantics.

If the body is available, the compiler may invalidate and recompute the
summary.

If the body is unavailable, the compiler must use an explicit compatible
contract or fall back conservatively.

---

# Summary invalidation

Cached closure/callable summaries must be invalidated when relevant inputs
change.

Relevant dependencies include at least:

```text
function body
lambda body
capture list
resolved overload/callable identity
callable parameter type
callee callable summary
callable contract
Place/provenance result
ownership semantics relevant to capture/callable transfer
generic specialization
CompilationPlan-selected body
analysis algorithm/schema version
```

Incremental compilation may invalidate a smaller subset only when dependency
tracking proves it sound.

---

# Generic functions

Generic callable summaries may be shared when the symbolic callable relationship
is independent of concrete type arguments.

Example:

```sec
fn Identity[T](operation: fn(T) T) fn(T) T {
    return operation
}
```

may preserve:

```text
ReturnedCallable = CallableParameter(0)
```

across compatible specializations.

When capture behavior, ownership, concrete field structure, callable contract,
or target-selected behavior differs by specialization, the summary must be
specialized accordingly.

---

# CompilationPlan identity

Closure/callable analysis is target-independent when the analyzed semantic body
and relevant callable contracts are target-independent.

A summary becomes CompilationPlan-dependent when the plan changes relevant
behavior, for example through:

```text
target-selected code
platform-specific implementation
target-specific opaque callable contract
selected specialization
target-known compiler operation
```

CompilationPlan identity must be part of cache/summary identity whenever it can
change the result.

The compiler need not duplicate summaries across plans when equivalence is
proven.

---

# Diagnostics

Closure analysis primarily produces facts.

Language diagnostics are derived by combining those facts with ownership,
lifetime, escape, storage, callable-contract, effect, concurrency, and other
normative rules.

Diagnostics should be relational and explain the dependency chain.

A useful diagnostic identifies as applicable:

```text
closure creation site
capture source
capture transfer
callable value
environment dependency
escape/retention destination
required guarantee
failing lifetime/storage/ownership fact
```

---

# Capture diagnostic examples

Example shape:

```text
error: captured value cannot be copied

`resource` is not Copy

the closure capture requires value transfer into its environment
```

Example shape:

```text
error: value cannot be moved into closure environment

`resource` is still required by a deferred use
```

Closure analysis provides the capture relation.

Ownership/lifetime analysis provides the reason the transfer is invalid.

---

# Environment lifetime diagnostic example

Example shape:

```text
error: returned closure depends on a value that does not live long enough

closure created here
    captures `view`

`view`
    depends on backing storage `buffer`

`buffer`
    uses Automatic storage ending when `MakeHandler` returns

closure escapes through the return value
```

The diagnostic should expose the shortest useful cause path rather than merely
stating `invalid closure`.

---

# Callable-contract diagnostic example

Example shape:

```text
error: callable contract is insufficient for this operation

the concrete target of `handler` is not known

the current callable contract does not guarantee:
    NoBlock
```

A diagnostic should identify why target precision or contract information is
insufficient for the consuming analysis.

---

# Diagnostic categories

Closure-related diagnostics require stable diagnostic IDs assigned by the
canonical diagnostics registry.

Conceptual categories include:

```text
closure.invalid-capture
closure.capture-move-invalid
closure.capture-copy-invalid
closure.capture-lifetime
closure.capture-storage-dependency
closure.environment-outlives-dependency
closure.returned-invalid-environment
closure.retained-invalid-environment
closure.task-transfer-invalid-environment
closure.thread-transfer-invalid-environment
closure.callable-target-unknown
closure.callable-contract-missing
closure.summary-invalid
closure.summary-version-mismatch
```

The textual category names above are conceptual and do not assign the final
numeric/stable diagnostic IDs.

Diagnostic IDs identify the rule/compiler phase, not the current severity.

---

# Precision exhaustion and diagnostics

Precision-budget exhaustion is not normally a user error by itself.

The compiler widens conservatively.

If widening prevents a required safety proof, the resulting diagnostic should
explain the unresolved guarantee or unknown behavior rather than emitting a
misleading performance-only warning.

Optional analysis tooling may expose precision-widening information for compiler
analysis and debugging.

---

# Consumers

Closure-analysis facts are designed for reuse.

Primary consumers include:

```text
call graph
escape analysis
lifetime analysis
ownership validation
effect analysis
stack analysis
ISR analysis
parameter-usage analysis
LSP tooling
sec analyse
optimization
Semantic IR / Sec MLIR lowering
```

---

# Call-graph integration

Closure analysis owns function-value target flow.

Call graph owns execution relationships.

Conceptually:

```text
Closure analysis:
    call site X target set = { A, B }

Call graph:
    X may execute A
    X may execute B
```

Call graph must not need to build a separate competing function-value dataflow
engine.

Closure analysis may consume call-graph SCCs and other graph structure for
interprocedural fixed points.

---

# Escape/lifetime integration

Conceptually:

```text
Closure analysis:
    Environment(S) depends on Local(x)

Escape analysis:
    Environment(S) escapes through return

Lifetime analysis:
    Local(x) does not survive the destination lifetime
```

The combined result is an invalid returned closure.

Closure analysis must expose environment dependencies precisely enough for this
composition.

---

# Effect-analysis integration

For a function-value invocation, effect analysis consumes the target set or open
callable contract.

For a closed set:

```text
Effects(call) = union Effects(target)
```

subject to the effect-analysis rulebook.

For an open target set, the callable contract supplies the conservative effect
summary.

Closure analysis does not redefine effect propagation.

---

# Stack-analysis integration

Stack analysis consumes possible callable targets for indirect calls.

A closed finite target set allows stack analysis to consider every possible
callee.

An open target set requires sufficient callable stack-bound contract information
where a proven bound is required.

Deep callable-flow refinement may tighten stack estimates without changing
source semantics.

---

# ISR and concurrency integration

ISR and concurrency analyses may query callable-flow and environment facts such
as:

```text
may this callable invoke a blocking target?
may it allocate?
does its closure environment depend on forbidden storage?
does it depend on thread-local state?
may it escape to another execution context?
```

Closure analysis provides callable/environment facts.

The consuming rulebook determines legality.

---

# Parameter-usage integration

Parameter-usage analysis may consume callable facts such as:

```text
Invoked
NotReturned
NotRetained
NotStored
```

when those facts can be proven through closure and escape analysis.

Closure analysis itself does not issue API-shape advisories that belong to
parameter-usage analysis.

---

# Semantic IR requirements

The compiler's canonical semantic representation must preserve enough
information to represent closure/callable facts without reverse-engineering them
from low-level pointer operations.

At minimum, Semantic IR or equivalent compiler state must preserve the semantic
distinctions required for:

```text
function-value creation
named callable identity
lambda body identity
closure creation site
capture values and transfer classification
function-value copy/move
function-value call
callable parameter forwarding
returned callable relationships
closure environment dependency
open callable contract
spawn/task/thread transfer where callable values participate
```

Exact IR operation names are implementation-defined.

Sec MLIR lowering may consume these facts but must not become the normative
source of closure semantics.

---

# No runtime analysis requirement

Closure analysis is compile-time compiler metadata.

This rulebook does not require:

```text
runtime CFA graph
runtime closure-analysis registry
runtime call graph
runtime ownership table
runtime borrow tracker
runtime reflection metadata
```

Runtime closure/environment representation exists only as required by the
actual program semantics and lowering strategy.

---

# LSP integration

The LSP consumes the Interactive analysis budget by default unless project
configuration selects a different supported analysis depth.

Useful closure/callable LSP information may include:

```text
capture list
capture transfer kind
possible callable targets
open callable contract
closure environment dependencies
call hierarchy
escape/retention cause path
analysis precision state
```

LSP output must distinguish information the programmer needs from information
the programmer may merely want.

---

# Required LSP information versus optional insight

Two presentation classes are required conceptually:

```text
NeedToKnow
OptionalInsight
```

`NeedToKnow` includes information necessary to understand:

- an error or warning;
- a safety condition;
- an unresolved callable contract;
- a retention/lifetime requirement;
- another condition the programmer must act on.

`OptionalInsight` includes useful exploratory information that is not required to
understand program validity.

Examples may include:

```text
call count
number of callers
full possible-target list when not needed for a diagnostic
extended cause paths
deep-analysis narrowing information
additional optimization/advisory facts
```

Optional insight must be configurable rather than forced into every hover.

---

# Configurable LSP analysis depth

LSP analysis depth is project-configurable.

A small project on a machine with ample resources may select continuous Deep
analysis.

A large project or resource-constrained machine may select a cheaper Interactive
analysis profile.

The selected profile changes precision and optional information, not language
semantics.

A future LSP synchronization rule/appendix must require the LSP to:

```text
read the project's configuration file when the workspace is opened;
apply analysis-related project settings;
watch the project configuration for changes;
reload changed analysis settings;
invalidate affected analysis caches and summaries;
recompute affected analysis results;
refresh diagnostics, hover, call hierarchy, and other dependent LSP output;
apply the new configuration without requiring an LSP/server restart.
```

This rulebook defines the closure-analysis integration requirement. The LSP
rulebook owns the complete LSP configuration/watch behavior.

---

# Hover behavior

Hover output must not become an unconditional dump of all available compiler
analysis facts.

For example, a function call count may be useful but is normally optional
insight rather than mandatory hover content.

A project/editor configuration may request richer hover information when the
analysis budget and available resources make that appropriate.

Facts necessary to explain a current diagnostic or required safety contract
must remain available regardless of optional hover-detail settings.

---

# `sec analyse`

The compiler provides a dedicated analysis command conceptually named:

```text
sec analyse
```

Default behavior is to run all available analyses:

```text
sec analyse
    == sec analyse --all
```

The user may explicitly select one supported analysis instead of running all of
them.

The exact analysis-name vocabulary and full CLI syntax are owned by the compiler
CLI/tooling rules.

Closure deep analysis may expose information such as:

```text
callable target sets
closure creation sites
capture dependencies
escaping closure environments
open callable contracts
unknown indirect calls
precision widening
context-sensitive target refinement
```

`sec analyse` is not required for normal semantic correctness; it is the default
entry point for full/deep analysis and analysis reporting.

---

# Implementation governance

This rulebook contains normative requirements only.

Mutable implementation progress belongs in:

```text
implementation-status.yaml
```

A closure-analysis ledger entry should track granular capabilities rather than a
single misleading binary implemented/not-implemented flag.

Useful capability classes include:

```text
value capture
callable local flow
aggregate callable flow
interprocedural callable summaries
recursive callable flow
open callable contracts
closure environment dependencies
escape/lifetime integration
separate-compilation summaries
Interactive analysis
Deep analysis
LSP incremental integration
Semantic IR integration
lowering support
```

Current status, source-file paths, verification commands, and remaining work are
governance data and are not normative language semantics.

---

# Required capture tests

The Sec 0.1 test suite must cover at least:

```text
copyable value capture
move-only value capture
invalid move capture
invalid copy capture
mutable owned captured binding and `mut fn` capability inference
forced-consuming copyable capture and `-> fn` capability inference
shared and mutable reference capture lifetime and borrow conflicts
outer mutation after capture does not change captured value
duplicate capture
undefined capture
unassigned capture
implicit outer-variable use is rejected
captured reference value preserves referent dependency
captured slice preserves backing dependency
captured function value preserves callable/environment dependency
nested closure environments
multiple runtime closure instances from one creation site
```

---

# Required callable-flow tests

The test suite must cover at least:

```text
named function exact target
non-capturing lambda exact target
capturing lambda exact body target
local function-value assignment
function-value move
function-value copy where legal
branch target merge
loop target fixed point
callable struct field
constant-index callable array element
dynamic-index target widening
Option callable payload
union callable payload
returned callable parameter
returned finite choice of callable parameters
callable forwarding through multiple functions
capture of a callable parameter
open callable contract propagation
known targets plus open contract
```

---

# Required interprocedural tests

The test suite must cover at least:

```text
direct callable summary
transitive callable forwarding
declaration-order independence
direct recursion
mutual recursion
recursive callable return
recursive closure creation
higher-order SCC convergence
generic callable summary
generic specialization where closure behavior differs
separate-compilation callable summary
stale summary rejection
summary-version mismatch
missing-summary conservative fallback
```

---

# Required escape/lifetime integration tests

The test suite must cover at least:

```text
local non-escaping closure
returned valid closure with self-contained owned captures
returned closure with invalid reference dependency
closure passed to a non-retaining call
closure passed to a retaining call
closure stored in an escaping aggregate
closure transferred to task
closure transferred to thread
nested escaping closure
closure capturing a slice whose backing expires
closure capturing another closure whose environment expires
closure capturing a generation-dependent handle/reference
```

The final diagnostic may be owned by escape, lifetime, storage, ownership, or
concurrency validation, but the closure dependency facts must participate.

---

# Required CFA precision tests

The test suite must verify at least:

```text
disjoint callable fields remain distinct
constant callable indices remain distinct where Place analysis proves them
branch merge produces an exact finite closed set
known targets survive merge with an open contract
unknown target never becomes no-target
precision cap widens conservatively
covering contract survives widening
closure creation-site abstraction terminates
recursive target propagation reaches fixed point
result is independent of traversal/declaration order
```

---

# Analysis-budget tests

## Interactive

Tests must verify:

```text
sound bounded result
incremental invalidation
Pending state allowed while recomputing
no stale incomplete fact presented as Proven
```

## Standard

Tests must verify:

```text
deterministic result
all required correctness proofs
same result independent of compiler worker scheduling/worklist order
```

## Deep

Tests must verify:

```text
same source semantics
precision equal to or stronger than Standard where refinement succeeds
no weaker safety guarantee
context-sensitive refinement remains sound
```

Deep analysis must not make an invalid program valid by changing the language
rules.

---

# LSP integration tests

The complete LSP test matrix belongs to the LSP rulebook/appendix.

Closure integration must nevertheless support tests equivalent to:

```text
open project
load project analysis configuration
compute Interactive closure/callable facts
change project analysis configuration
detect configuration change
invalidate affected closure/callable caches
recompute affected facts
refresh hover/call hierarchy/diagnostics
no LSP restart required
optional hover facts may be toggled independently of required diagnostic facts
```

---

# Performance and termination tests

Compiler regression tests should include:

```text
many closure creation sites
large finite target sets
large recursive SCC
nested higher-order forwarding
closures created inside loops
large aggregate callable state
many captured callable values
open-contract widening
```

No normative wall-clock threshold is defined here.

The tests must protect:

```text
termination
bounded analysis memory according to configured budgets
deterministic widening
sound target coverage
```

---

# Completion criteria for Sec 0.1

Closure analysis is complete for Sec 0.1 when all of the following hold:

1. all Sec 0.1 value, forced-consuming, shared-reference, and
   mutable-reference captures are represented correctly;
2. capture Copy/Move transfer follows ownership semantics;
3. captured reference/view/handle/callable values preserve transitive
   dependencies;
4. named functions, non-capturing lambdas, and capturing closures produce sound
   callable target facts;
5. callable values flow through canonical Places, aggregates, assignments,
   branches, and loops;
6. loop analysis reaches a sound fixed point;
7. interprocedural callable summaries preserve symbolic parameter/return/
   forwarding relationships;
8. recursive and mutually recursive summary dependencies converge soundly;
9. open callable contracts are preserved and usable by consumers;
10. closure-environment dependencies are consumable by escape and lifetime
    analysis;
11. call graph, effect analysis, stack analysis, and ISR analysis can consume
    canonical callable target/contract information;
12. separate compilation can consume compatible versioned summaries or fall
    back conservatively;
13. Interactive, Standard, and Deep analysis budgets satisfy their contracts;
14. precision widening is sound, bounded, terminating, and deterministic;
15. LSP integration can distinguish required information from optional insight
    and can honor project-selected analysis depth;
16. `sec analyse` can run all analyses by default and allow explicit selection of
    a particular analysis;
17. no escaping closure is repaired through implicit heap promotion, hidden
    allocation, hidden copying, or another source-invisible semantic change;
18. the required positive, negative, interprocedural, precision, integration,
    budget, and termination tests pass.

---

# Canonical design summary

The normative closure-analysis model can be summarized as follows:

```text
Closure analysis covers capture analysis, closure-environment dependencies, and
callable-value flow required for sound higher-order function invocation.

Callable bodies, function values, closure instances, and closure environments
are distinct semantic concepts.

A capturing lambda expression has one callable body but each runtime evaluation
may create a distinct closure instance and closure environment.

Capturing and non-capturing callables with the same source signature have the
same source-level function type. Capture shape and environment representation
are not part of function type identity.

Named functions and non-capturing lambdas have no closure-environment
dependency. Capturing callables depend on an explicit environment.

Captures are explicit. Sec 0.1 supports owned value, forced-consuming value,
shared-reference, and mutable-reference capture. Owned environment bindings are
mutable; borrowed captures retain their borrow and lifetime dependencies.

Capture analysis operates on canonical Place/provenance and consumes ownership,
lifetime, borrow, storage, and dependency facts rather than recreating those
analyses.

Capture mode and transfer mechanism are distinct. Plain value capture may use
Copy or Move according to normal Sec ownership semantics, while
forced-consuming capture requires Move even for a copyable value.

Capturing a reference-like, view-like, handle-like, or callable value by value
preserves the value's underlying dependencies.

Capture creation and closure escape are distinct events.

Closure analysis records environment dependencies. Escape analysis determines
where those dependencies travel. Lifetime analysis determines whether they
remain valid. Ownership analysis determines whether transfer is legal.

Callable-flow analysis tracks possible callable bodies through normal value
flow and uses the canonical call-graph TargetSet model.

The required callable-flow baseline is a sound monovariant higher-order
control-flow analysis equivalent in purpose to 0-CFA and adapted to Sec's typed
Place/provenance, summary, and callable-contract model.

The language does not require classical k-CFA for k > 0. Sound stronger
refinements may be provided as optional analysis precision.

Callable target sets are may-target over-approximations. Possible runtime targets
must never be omitted.

Function type compatibility does not imply identical callable contracts.

Callable target provenance reuses canonical Place and aggregate precision.

Callable-body identity is distinct from runtime closure-environment instance
identity.

A bare function value is always valid and non-null. Optional callable absence is
represented explicitly with Option.

Callable copy and move behavior follows semantic ownership and must not be
inferred from backend pointer representation.

Closure analysis reports required environment lifetime and dependency facts but
does not choose physical storage or allocation strategy.

Closure escape must never trigger implicit heap promotion or another hidden
allocation policy.

Unknown concrete target and unknown callable contract are distinct states.
Known open contracts may summarize unknown concrete targets where required
guarantees are present.

Callable flow is monotone and fixed-point based. Loops and recursive
interprocedural dependencies converge through finite abstractions and
conservative widening.

Callable contracts are a preferred widening boundary when concrete target
identity is not required by the current proof.

Sec defines Interactive, Standard, and Deep analysis budgets. Interactive is
optimized for incremental tooling, Standard performs all required normal-build
proofs, and Deep provides optional expensive refinement and analysis reports.

Optional precision never changes source-language semantics. If a language
feature requires a stronger proof, that proof becomes required for the feature.

Standard results are deterministic. All analysis budgets terminate.

Separate compilation uses compatible versioned symbolic callable/closure
summaries or explicit contracts and otherwise falls back conservatively.

Call graph consumes callable targets; escape/lifetime consume environment
dependencies; effect, stack, ISR, parameter-usage, LSP, and optimization consume
appropriate canonical facts.

LSP presentation distinguishes NeedToKnow facts from configurable
OptionalInsight. Project configuration may select analysis depth and must be
reloadable by LSP integration without server restart.

`sec analyse` runs all available analyses by default, equivalent in intent to
`sec analyse --all`, while allowing explicit selection of one supported
analysis.

Mutable implementation status belongs in implementation-status.yaml, not in
this normative rulebook.
```
