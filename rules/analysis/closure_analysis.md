# Closure Analysis

- **Status:** Normative
- **Created:** 2026-08-09
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 2
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/closure_analysis.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`

---

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the semantic model, analysis domain, callable-value flow,
closure-environment dependencies, control-flow analysis baseline, analysis
budgets, interprocedural summaries, diagnostics requirements, tooling
integration, and completion criteria for Sec closure analysis.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.

---

## § 1 Purpose

**§ 1(1)** Closure analysis determines:

- what a closure captures;
- how captured values enter the closure environment;
- what semantic dependencies the closure environment carries;
- which callable bodies a function-valued value may invoke;
- how callable values flow through control flow, Places, aggregates, calls, and
  returns;
- what symbolic callable relationships must survive interprocedural and
  separate-compilation boundaries;
- what callable and environment facts other analyses may safely consume.

**§ 1(2)** Closure analysis is broader than lambda syntax.

**§ 1(3)** It covers three closely related analysis responsibilities:

```text
CaptureAnalysis
ClosureLifetimeAnalysis
CallableFlowAnalysis
```

**§ 1(4)** The three responsibilities share the same callable-value and Place/provenance
infrastructure and therefore belong to one normative analysis model.

**§ 1(5)** Closure analysis does not choose physical closure representation, storage
placement, heap allocation, ABI representation, or backend calling convention.

---

## § 2 Normative role

**§ 2(1)** This rulebook is normative for:

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

**§ 2(2)** This rulebook does not redefine:

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

**§ 2(3)** Those semantics remain owned by their respective normative rulebooks.

---

## § 3 Relationship to existing language rules

**§ 3(1)** `rules/declarations/lambda-functions.md` defines the source-language
function-value, callable-capability, and lambda semantics consumed by this
analysis.

**§ 3(2)** In Sec 0.1:

- lambdas are anonymous `fn` values;
- named functions may be used as function values when overload resolution is
  unambiguous;
- function values may be passed, stored, returned, and called;
- a function value is never null;
- optional absence is represented explicitly with `Option[fn(...)]`;
- enclosing locals are never captured implicitly;
- `capture(...)` explicitly identifies captured values;
- capture modes are explicit copy, move, shared reference,
  and mutable reference;
- owned capture bindings are mutable inside the closure, independently of the
  mutability of the outer binding;
- callable environment capability is inferred as `fn`, `mut fn`, or `-> fn`;
- capture expressions are evaluated exactly once;
- capturing and non-capturing callables with the same parameter and return
  types have the same source-level function type;
- no hidden cloning, ownership transfer, or heap allocation is permitted.

**§ 3(3)** Closure analysis consumes those rules and must not invent additional source
syntax or capture semantics.

---

## § 4 Relationship to other analyses

**§ 4(1)** Closure analysis is intentionally separated from escape, lifetime, ownership,
storage, call-graph, and effect analysis.

**§ 4(2)** Conceptually:

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

**§ 4(3)** The analyses cooperate through explicit facts and shared canonical identities.
They must not reimplement one another's semantic models.

---

## § 5 Core semantic entities

**§ 5(1)** Closure analysis distinguishes four concepts:

```text
CallableBody
FunctionValue
ClosureInstance
ClosureEnvironment
```

**§ 5(2)** They are not interchangeable.

### § 5.1 Callable body

**§ 5.1(1)** A `CallableBody` is the executable semantic body selected by a callable value.

**§ 5.1(2)** Examples include:

```text
NamedFunction
NonCapturingLambdaBody
CapturingLambdaBody
ForeignCallable
CompilerKnownCallable
```

**§ 5.1(3)** A lambda expression has one stable callable-body identity for a concrete
CompilationPlan and specialization, regardless of how many times the lambda
expression executes at runtime.

**§ 5.1(4)** Example:

```sec
fn Make(value: int) fn() int {
    return capture(value) fn() int {
        return value
    }
}
```

**§ 5.1(5)** The lambda body has one callable-body identity.

**§ 5.1(6)** Two calls to `Make` may create different runtime closure instances.

---

### § 5.2 Function value

**§ 5.2(1)** A `FunctionValue` is a source-level value whose type is a Sec function type.

**§ 5.2(2)** Example:

```sec
fn(int) int
```

**§ 5.2(3)** A function value may refer to:

- a named function;
- a non-capturing lambda;
- a capturing closure.

**§ 5.2(4)** Function type identity does not include:

```text
capture shape
closure-environment type
closure creation site
runtime closure instance identity
concrete target identity
```

**§ 5.2(5)** Therefore:

```text
function type != capture shape
function type != environment type
function type != concrete target identity
```

---

### § 5.3 Closure instance

**§ 5.3(1)** A `ClosureInstance` is a runtime callable instance produced by evaluating a
capturing lambda expression.

**§ 5.3(2)** Example:

```sec
fn Make(value: int) fn() int {
    return capture(value) fn() int {
        return value
    }
}

let first := Make(10)
let second := Make(20)
```

**§ 5.3(3)** Conceptually:

```text
CallableBody:
    same

ClosureInstance:
    distinct

ClosureEnvironment:
    distinct
```

**§ 5.3(4)** Analysis abstraction must not imply that distinct runtime closure instances are
the same runtime object.

---

### § 5.4 Closure environment

**§ 5.4(1)** A `ClosureEnvironment` is the semantic carrier of the values captured by one
closure instance.

**§ 5.4(2)** Conceptually:

```text
ClosureEnvironment {
    capture[0]
    capture[1]
    ...
}
```

**§ 5.4(3)** The environment is not a source-visible struct.

**§ 5.4(4)** Closure analysis defines semantic environment dependencies.

**§ 5.4(5)** Backend representation, field layout, hidden lowering parameters, and storage
placement are separate lowering concerns.

---

## § 6 Capturing and non-capturing callables

### § 6.1 Named functions

**§ 6.1(1)** A named function used as a value has:

```text
Target = exact resolved callable body
Environment = None
```

**§ 6.1(2)** Overload resolution is completed before closure analysis consumes the callable
identity.

---

### § 6.2 Non-capturing lambdas

**§ 6.2(1)** A non-capturing lambda has:

```text
Target = exact lambda body
Environment = None
```

**§ 6.2(2)** No closure-environment dependency is required merely because the source syntax
uses an anonymous function.

**§ 6.2(3)** A compiler may still materialize a callable runtime representation when later
lowering or ABI rules require one.

---

### § 6.3 Capturing lambdas

**§ 6.3(1)** A capturing lambda has:

```text
Target = exact lambda body
Environment = known closure environment
```

**§ 6.3(2)** Invocation of the callable semantically requires the corresponding environment
to remain valid.

**§ 6.3(3)** The source-level function type remains unchanged.

**§ 6.3(4)** The environment is not a source parameter and must not affect source parameter
count, overload resolution, or function-type identity.

---

## § 7 Capture model

### § 7.1 Captures are explicit

**§ 7.1(1)** Closure analysis must never infer an undeclared capture merely because the
lambda body refers to an outer local.

**§ 7.1(2)** A valid capture is introduced by the source-language capture clause.

**§ 7.1(3)** Example:

```sec
let factor := 2

let multiply := capture(factor) fn(value: int) int {
    return value * factor
}
```

**§ 7.1(4)** Use of an enclosing local without explicit capture is invalid according to the
lambda rulebook.

**§ 7.1(5)** Closure analysis validates and classifies captures. It does not invent them.

---

### § 7.2 Capture analysis uses canonical Places

**§ 7.2(1)** Capture analysis operates on canonical Place/provenance information where the
capture source has an addressable Place identity.

**§ 7.2(2)** Relevant capture sources may include, where source syntax and ownership rules
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

**§ 7.2(3)** Closure analysis must reuse canonical Place identity and projection precision.
It must not construct a separate symbol-only alias model.

---

## § 8 Capture record

**§ 8(1)** Each capture must be representable conceptually as:

```text
CaptureRecord {
    SourcePlace
    CapturedType
    CaptureMode
    Transfer
    Dependencies
}
```

**§ 8(2)** Exact compiler data structures are implementation-defined.

### § 8.1 SourcePlace

**§ 8.1(1)** `SourcePlace` identifies the canonical source Place where such identity exists.

**§ 8.1(2)** For captured expressions without a stable addressable Place, the compiler must
retain equivalent source/provenance identity sufficient for ownership,
lifetime, diagnostics, and dependency analysis.

### § 8.2 CapturedType

**§ 8.2(1)** `CapturedType` is the resolved semantic type of the captured value.

### § 8.3 CaptureMode

**§ 8.3(1)** Sec 0.1 defines:

```text
CopyCapture
MoveCapture
SharedReference
MutableReference
```

**§ 8.3(2)** These correspond to `capture(value)`, `capture(<-value)`,
`capture(ref value)`, and `capture(ref mut value)`. Capture mode is explicit;
closure analysis must not infer reference capture from the captured value's
type or outer-binding mutability.

### § 8.4 Transfer

**§ 8.4(1)** The mechanism by which the value enters the environment is distinct from
capture mode.

**§ 8.4(2)** Conceptually:

```text
Copy
Move
Unknown
```

**§ 8.4(3)** `CopyCapture` requires `Copy`; it must not be reclassified as `Move` because the
source type is non-copyable. `MoveCapture` requires `Move`, including for a
copyable source.

**§ 8.4(4)** Closure analysis records the resulting fact but does not independently decide
whether the copy or move is legal.

### § 8.5 Dependencies

**§ 8.5(1)** A captured value may carry dependencies such as:

```text
referent lifetime
backing storage
storage domain
generation
owned resource
nested closure environment
callable contract
```

**§ 8.5(2)** Value capture does not erase those dependencies.

---

## § 9 Value capture semantics

### § 9.1 Snapshot at closure creation

**§ 9.1(1)** A copy capture observes and copies the value at closure creation.

**§ 9.1(2)** Example:

```sec
let mut factor := 2

let operation := capture(factor) fn(value: int) int {
    return value * factor
}

factor = 10
```

**§ 9.1(3)** The closure continues to observe the captured value `2`.

**§ 9.1(4)** Conceptually:

```text
Copy capture
    = value copy at closure creation
    != alias to the outer binding
```

---

### § 9.2 Captured binding is distinct

**§ 9.2(1)** After capture, the outer binding and the environment binding are distinct
semantic bindings.

**§ 9.2(2)** Conceptually:

```text
outer factor
environment.capture[0]
```

**§ 9.2(3)** If transfer was `Copy`, both values remain available according to normal
ownership rules.

**§ 9.2(4)** An explicit `MoveCapture` makes the outer source unavailable only when closure
environment construction successfully commits.

**§ 9.2(5)** The name used inside the lambda denotes the captured environment binding, not a
continued access to the outer binding.

---

### § 9.3 Owned capture bindings are mutable

**§ 9.3(1)** An owned value capture is a mutable closure-local binding regardless of whether
the outer binding was mutable. Mutating it requires at least `mut fn` callable
capability.

**§ 9.3(2)** `capture(<-value)` explicitly requests the creation-time transfer and is a
move even when the captured type is copyable. It does not introduce a distinct
binding-mutability class.

**§ 9.3(3)** Closure analysis records environment mutation and consumption so callable
capability inference can distinguish reusable shared, reusable mutable, and
consuming callables.

---

## § 10 Reference-typed values versus reference capture

**§ 10(1)** Capturing a value whose type is itself reference-like is still a Sec 0.1 value
capture.

**§ 10(2)** Example:

```sec
let itemRef: ref Item := ref item

let operation := capture(itemRef) fn() int {
    return itemRef.value
}
```

**§ 10(3)** Conceptually:

```text
CaptureMode = Value
```

**§ 10(4)** The reference value is transferred into the environment.

**§ 10(5)** Its referent dependency remains attached to the captured value.

**§ 10(6)** Therefore the closure environment may depend on:

```text
referent object lifetime
referent storage lifetime
storage domain/generation
borrow validity
```

**§ 10(7)** as applicable.

**§ 10(8)** This is distinct from explicit reference-capture syntax such as:

```sec
capture(ref item) fn() int {
    return item.value
}
```

**§ 10(9)** which establishes a borrow directly from another Place. Shared and mutable
reference captures retain the normal borrow, lifetime, storage, and provenance
dependencies of that Place. A mutable-reference capture is move-only and
requires at least `mut fn` callable capability.

---

## § 11 Views and slices

**§ 11(1)** A captured slice or view is a value capture of its descriptor/value semantics.

**§ 11(2)** Its backing-storage dependency remains attached.

**§ 11(3)** Example:

```sec
let view := data[10..20]

let operation := capture(view) fn() int {
    return view[0]
}
```

**§ 11(4)** Closure analysis must preserve the relationship:

```text
ClosureEnvironment
    -> captured view
    -> backing storage dependency
```

**§ 11(5)** Value capture does not make the backing storage independently owned.

---

## § 12 Captured function values

**§ 12(1)** Function values may themselves be captured.

**§ 12(2)** Example:

```sec
fn Wrap(operation: fn(int) int) fn(int) int {
    return capture(operation) fn(value: int) int {
        return operation(value)
    }
}
```

**§ 12(3)** The outer closure environment owns the captured function value according to
normal copy/move semantics.

**§ 12(4)** The captured function value may itself carry:

```text
callable target set
callable contract
nested closure-environment dependency
```

**§ 12(5)** Closure analysis must preserve these transitively.

**§ 12(6)** Nested closure environments are therefore valid semantic dependency graphs.

---

## § 13 Closure environment as dependency carrier

**§ 13(1)** A closure environment is a structural dependency carrier.

**§ 13(2)** Conceptually:

```text
Environment(S) {
    capture[0] -> dependencies A
    capture[1] -> dependencies B
}
```

**§ 13(3)** If the callable value that depends on the environment escapes, the relevant
environment dependencies are visible to escape analysis.

**§ 13(4)** Capture creation alone does not imply escape.

---

## § 14 Capture and escape are distinct

**§ 14(1)** A closure may be created and used entirely within its original context.

**§ 14(2)** Example:

```sec
let operation := capture(factor) fn(value: int) int {
    return value * factor
}

return operation(10)
```

**§ 14(3)** The environment exists, but the callable value need not escape the function.

**§ 14(4)** Conceptually:

```text
capture != escape
```

**§ 14(5)** Closure analysis records environment dependencies.

**§ 14(6)** Escape analysis determines where the callable/environment dependency travels.

**§ 14(7)** Lifetime analysis determines whether those dependencies remain valid for the
required destination lifetime.

---

## § 15 Closure escape and storage

**§ 15(1)** Closure analysis may derive facts such as:

```text
EnvironmentRequiresEscape
RequiredEnvironmentLifetime
EnvironmentDependencies
```

**§ 15(2)** It must not derive a physical allocation decision merely from escape.

**§ 15(3)** In particular:

```text
closure environment escapes
```

**§ 15(4)** does not imply:

```text
allocate environment on heap
```

**§ 15(5)** No implicit heap promotion is permitted.

**§ 15(6)** No hidden allocation may be introduced merely to repair an otherwise invalid
closure lifetime.

**§ 15(7)** If no valid source-semantic ownership/storage strategy exists, the program is
invalid rather than silently promoted.

**§ 15(8)** Optimization may eliminate or transform an environment only when all source
semantics and required dependencies remain preserved.

---

## § 16 Callable-flow analysis

**§ 16(1)** Callable-flow analysis determines a sound may-target set for function-valued
values at relevant program points.

**§ 16(2)** It is the callable-specific higher-order control-flow analysis used by Sec.

**§ 16(3)** The analysis tracks callable values through:

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

**§ 16(4)** It reuses canonical Place/provenance and aggregate-flow infrastructure.

---

## § 17 Control-flow-analysis baseline

**§ 17(1)** Sec callable-flow analysis belongs to the established control-flow-analysis
family for higher-order programs.

**§ 17(2)** The required baseline is a sound monovariant analysis equivalent in purpose to
0-CFA, adapted to Sec's:

- static function types;
- explicit captures;
- canonical Place/provenance model;
- ownership information;
- symbolic interprocedural summaries;
- callable contracts;
- concrete CompilationPlan.

**§ 17(3)** The rulebook does not require classical k-CFA for `k > 0`.

**§ 17(4)** Implementations may provide stronger sound refinements, including:

```text
context-sensitive CFA
selective polyvariance
call-string sensitivity
closure/environment sensitivity
pushdown refinement
other sound target refinements
```

**§ 17(5)** Such refinements improve precision. They do not define different source
semantics.

**§ 17(6)** The required baseline is monovariant, not intentionally imprecise.

**§ 17(7)** Cheap Sec-specific symbolic relations may preserve more precision than a naive
classical 0-CFA implementation without violating the baseline model.

---

## § 18 Callable target sets

**§ 18(1)** Closure analysis uses the canonical call-graph target-set model.

**§ 18(2)** Conceptually:

```text
TargetSet {
    KnownTargets
    IsClosed
    OpenContract
}
```

**§ 18(3)** The following semantic forms are useful descriptions of that model:

```text
Exact
ClosedSet
OpenContract
KnownSetPlusOpenContract
```

**§ 18(4)** They do not require a second competing representation.

### § 18.1 Exact

**§ 18.1(1)** An exact target is a closed target set containing one callable body.

```text
KnownTargets = { A }
IsClosed = true
OpenContract = none
```

### § 18.2 ClosedSet

**§ 18.2(1)** A closed finite set contains all possible concrete targets.

```text
KnownTargets = { A, B, C }
IsClosed = true
OpenContract = none
```

### § 18.3 OpenContract

**§ 18.3(1)** Concrete targets are not fully known, but all legal unknown targets must satisfy
an explicit callable contract.

```text
KnownTargets = {}
IsClosed = false
OpenContract = C
```

### § 18.4 KnownSetPlusOpenContract

**§ 18.4(1)** Some concrete targets are known and additional targets remain permitted by the
open contract.

```text
KnownTargets = { A, B }
IsClosed = false
OpenContract = C
```

**§ 18.4(2)** A target set with `IsClosed == false` must have a valid open callable contract
where semantic consumers require guarantees about the unknown targets.

---

## § 19 Soundness of target sets

**§ 19(1)** Callable target sets are may-target over-approximations.

**§ 19(2)** Every callable body that may actually execute at runtime must be represented by
one of:

- a known concrete target;
- a sound open callable contract covering that target.

**§ 19(3)** It is permitted for a bounded analysis to temporarily include concrete targets
that cannot execute on a particular runtime path.

**§ 19(4)** It is not permitted to omit a possible runtime target in order to gain
precision or optimization.

---

## § 20 Function type versus callable contract

**§ 20(1)** Function type compatibility does not imply identical callable behavior.

**§ 20(2)** A function type such as:

```sec
fn(Request) Response
```

**§ 20(3)** specifies source-level parameter and return types.

**§ 20(4)** It does not by itself establish guarantees such as:

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

**§ 20(5)** Callable contracts are separate compiler-visible semantic contracts.

**§ 20(6)** Closure analysis propagates those contracts where concrete targets are open or
unknown.

**§ 20(7)** It does not redefine the effect, retention, stack, or reentry semantics stored
inside the contract.

---

## § 21 Callable creation

### § 21.1 Named function value

**§ 21.1(1)** After overload resolution:

```sec
let operation: fn(int) int := Transform
```

**§ 21.1(2)** produces conceptually:

```text
TargetSet = Exact(Transform)
Environment = NoEnvironment
```

---

### § 21.2 Non-capturing lambda

```sec
let operation := fn(value: int) int {
    return value * 2
}
```

**§ 21.2(1)** produces conceptually:

```text
TargetSet = Exact(lambda-body-id)
Environment = NoEnvironment
```

---

### § 21.3 Capturing lambda

```sec
let operation := capture(factor) fn(value: int) int {
    return value * factor
}
```

**§ 21.3(1)** produces conceptually:

```text
TargetSet = Exact(lambda-body-id)
Environment = Environment(creation-site)
```

**§ 21.3(2)** The environment carries the capture records defined by this rulebook.

---

## § 22 Abstract closure identity

**§ 22(1)** The baseline analysis may abstract all runtime closure instances created at one
closure creation site into one finite abstract environment identity.

**§ 22(2)** Conceptually:

```text
all runtime environments created at site S
    -> AbstractEnvironment(S)
```

**§ 22(3)** This is an analysis abstraction only.

**§ 22(4)** It must not be interpreted as runtime object identity.

**§ 22(5)** If a closure is created repeatedly in a loop or recursion, different runtime
instances may contain different captured values even when the baseline analysis
joins them into one abstract environment state.

---

## § 23 Callable flow through assignment

**§ 23(1)** Assignment updates the current callable-flow state according to normal Sec
value semantics.

**§ 23(2)** Example:

```sec
let mut operation: fn(int) int := A
operation = B
```

**§ 23(3)** After the second assignment:

```text
Targets(operation) = Exact(B)
```

**§ 23(4)** The previous target `A` is not retained merely because it was historically held
by the same variable.

**§ 23(5)** Historical escape or effect facts owned by other analyses remain separate.

---

## § 24 Callable flow through Copy and Move

**§ 24(1)** Callable copy and move preserve the semantic relationship between:

```text
TargetSet
CallableContract
EnvironmentDependency
```

**§ 24(2)** according to normal Sec ownership rules.

**§ 24(3)** Closure analysis must not infer callable copyability from backend representation
such as a pair of pointers.

**§ 24(4)** When a callable move is legal, the callable target/environment relation follows
the destination and the source ownership state changes according to ownership
analysis.

**§ 24(5)** When callable copy is legal, the resulting environment ownership/sharing
semantics are whatever the callable ownership rules establish.

**§ 24(6)** Closure analysis must not silently duplicate an owned closure environment.

---

## § 25 Control-flow join

**§ 25(1)** Control-flow joins conservatively combine possible callable targets and
contracts.

**§ 25(2)** Example:

```sec
let mut operation: fn(int) int := A

if condition {
    operation = B
}
```

**§ 25(3)** After the branch:

```text
Targets(operation) = ClosedSet { A, B }
```

**§ 25(4)** If one predecessor contains a closed concrete set and another contains an open
contract, the join preserves known concrete targets and the open component.

**§ 25(5)** Conceptually:

```text
Exact(A)
join
OpenContract(C)

=> KnownSetPlusOpenContract { A, C }
```

**§ 25(6)** Known concrete information must not be discarded merely because an open
component also exists.

---

## § 26 Places and aggregates

**§ 26(1)** Callable target facts attach to function-valued values and Places.

**§ 26(2)** When Place analysis proves disjoint structural locations, callable-flow analysis
must reuse that precision.

**§ 26(3)** Example:

```sec
type Handlers struct {
    Open: fn(Event) void
    Close: fn(Event) void
}
```

**§ 26(4)** If:

```sec
handlers.Open = OnOpen
handlers.Close = OnClose
```

**§ 26(5)** then a call through `handlers.Open` need not include `OnClose` merely because
both values reside in the same aggregate.

---

## § 27 Arrays and collections

**§ 27(1)** Function values stored in fixed arrays or collections carry target sets like
other values.

**§ 27(2)** Constant-index precision should be preserved where canonical Place analysis
already provides it.

**§ 27(3)** Example:

```sec
handlers[0] = A
handlers[1] = B
```

**§ 27(4)** A call through `handlers[0]` may remain exact.

**§ 27(5)** A call through a dynamic index may join possible element target sets.

**§ 27(6)** Large or dynamically shaped containers may use bounded summarized element facts
rather than one analysis node per possible runtime element.

**§ 27(7)** Compiler-known collection summaries may describe callable propagation without
requiring full shape analysis for every library collection.

---

## § 28 Options and unions

**§ 28(1)** Optional callable absence is represented by `Option`, not by a null callable
target.

**§ 28(2)** For:

```sec
Option[fn(Event) void]
```

**§ 28(3)** callable target facts belong to the `Some` payload.

**§ 28(4)** `None` is not a pseudo callable target.

**§ 28(5)** The same active-payload principle applies to unions and other tagged aggregate
forms.

**§ 28(6)** When active variant information is lost, possible callable payloads are joined
conservatively.

---

## § 29 Callable parameters

**§ 29(1)** Function-valued parameters are represented symbolically in interprocedural
summaries.

**§ 29(2)** Example:

```sec
fn Apply(operation: fn(int) int, value: int) int {
    return operation(value)
}
```

**§ 29(3)** The analysis may represent:

```text
CallableParameter(0): Invoked
```

**§ 29(4)** without knowing the concrete target of `operation` while analyzing `Apply`.

**§ 29(5)** If an explicit callable contract exists, that contract is associated with the
symbolic callable parameter.

---

## § 30 Returned callable relationships

**§ 30(1)** Callable return summaries preserve symbolic relationships.

**§ 30(2)** Example:

```sec
fn Identity(operation: fn(int) int) fn(int) int {
    return operation
}
```

**§ 30(3)** Summary:

```text
ReturnedCallable = CallableParameter(0)
```

**§ 30(4)** A caller passing exact target `A` may therefore instantiate:

```text
Targets(result) = Exact(A)
```

**§ 30(5)** without context-sensitive reanalysis of `Identity`.

---

## § 31 Disjunctive callable returns

**§ 31(1)** Example:

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

**§ 31(2)** The summary may preserve:

```text
ReturnedCallable = CallableParameter(0) OR CallableParameter(1)
```

**§ 31(3)** Call-site instantiation may then produce an exact finite target set.

**§ 31(4)** This symbolic relation is permitted in the required monovariant baseline.

---

## § 32 Returning newly created closures

**§ 32(1)** Example:

```sec
fn Make(offset: int) fn(int) int {
    return capture(offset) fn(value: int) int {
        return value + offset
    }
}
```

**§ 32(2)** The callable summary must be able to express conceptually:

```text
ReturnedCallable.Target = Exact(lambda-body-id)
ReturnedCallable.Environment = Environment(lambda-creation-site)
Environment.capture[0] derives from Parameter(0)
```

**§ 32(3)** Escape analysis separately records that the returned callable/environment leaves
the function.

**§ 32(4)** No physical allocation choice follows from the callable summary itself.

---

## § 33 Callable forwarding

**§ 33(1)** Interprocedural callable flow must preserve forwarding relationships.

**§ 33(2)** Example:

```sec
fn Outer(operation: fn(int) int, value: int) int {
    return Inner(operation, value)
}
```

**§ 33(3)** If `Inner` invokes its first callable parameter, that fact must propagate through
`Outer` according to the callee summary.

**§ 33(4)** Returned forwarding behaves similarly.

**§ 33(5)** Example:

```sec
fn Outer(operation: fn(int) int) fn(int) int {
    return Inner(operation)
}
```

**§ 33(6)** If:

```text
Inner.ReturnedCallable = CallableParameter(0)
```

**§ 33(7)** then `Outer` may derive the same symbolic relationship.

---

## § 34 Capture forwarding

**§ 34(1)** A callable parameter may itself be captured by a newly created closure.

**§ 34(2)** Example:

```sec
fn Wrap(operation: fn(int) int) fn(int) int {
    return capture(operation) fn(value: int) int {
        return operation(value)
    }
}
```

**§ 34(3)** Closure analysis must preserve two different relationships:

```text
returned callable body
    = Wrap's lambda body

returned environment
    depends on CallableParameter(0)
```

**§ 34(4)** Inside the lambda body:

```text
invoked callable
    derives from environment capture[0]
```

**§ 34(5)** Callable target identity and closure-environment dependency must not be
collapsed into one fact.

---

## § 35 Callable invocation event

**§ 35(1)** Calling a function value is an explicit semantic analysis event.

**§ 35(2)** Conceptually:

```text
CallableInvocation {
    CallSite
    TargetSet
    CallableContract
    EnvironmentDependency
}
```

**§ 35(3)** Exact data structures are implementation-defined.

**§ 35(4)** The event provides canonical input to:

- call graph;
- effect analysis;
- stack analysis;
- ISR analysis;
- diagnostics;
- LSP tooling.

**§ 35(5)** The hidden environment argument used by lowering is not a source-level
parameter and is not required to appear in this semantic record as a fake source
argument.

---

## § 36 Callable-flow summaries

**§ 36(1)** Each analyzable callable may expose a composable summary conceptually equivalent
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

**§ 36(2)** For created closure sites, the summary may reference:

```text
ClosureSummary {
    BodyIdentity
    CaptureDependencies
    CaptureTransfers
    CallableContract
}
```

**§ 36(3)** Exact compiler structure is not normative.

**§ 36(4)** Summaries must preserve externally relevant symbolic relationships without
exporting irrelevant internal temporaries.

---

## § 37 Escape and ownership are not duplicated in callable summaries

**§ 37(1)** Callable-flow summaries must not become duplicate escape or ownership
rulebooks.

**§ 37(2)** Example:

```text
Closure analysis:
    CallableParameter(0)
        -> Environment(S).capture[0]

Escape analysis:
    Environment(S)
        -> escapes through return
```

**§ 37(3)** Those facts compose without closure analysis redefining `Returned`, `Retained`,
or other escape classes.

**§ 37(4)** Similarly:

```text
CaptureTransfer = Move
```

**§ 37(5)** is a closure-analysis fact, while ownership analysis determines whether the move
is legal and how source state changes.

---

## § 38 Monotone fixed-point analysis

**§ 38(1)** Callable-flow analysis is a monotone fixed-point analysis.

**§ 38(2)** Control-flow joins add possible targets/contracts or conservatively widen them.

**§ 38(3)** A possible callable target must never disappear merely because a later
analysis iteration sees a narrower path unless the analysis is performing a
separate sound refinement under a stronger context abstraction.

**§ 38(4)** Standard fixed-point results must be independent of source declaration order.

---

## § 39 Loops

**§ 39(1)** Loops are analyzed to a sound fixed point.

**§ 39(2)** Example:

```sec
let mut operation: fn(int) int := A

for item in items {
    if item.Enabled {
        operation = B
    }
}
```

**§ 39(3)** A sound post-loop result includes:

```text
Targets(operation) = { A, B }
```

**§ 39(4)** unless control-flow facts prove one alternative unreachable.

**§ 39(5)** Analyzing the loop body exactly once is not sufficient.

---

## § 40 Recursive callable summaries

**§ 40(1)** Direct and mutually recursive callable-summary dependencies are solved to a
sound fixed point.

**§ 40(2)** The compiler should reuse canonical call-graph strongly connected components
where available, or use an equivalent declaration-order-independent algorithm.

**§ 40(3)** This includes recursive higher-order forwarding relationships.

**§ 40(4)** Example conceptual cycle:

```text
A returns callable from B
B forwards callable into C
C returns callable relation back to A
```

**§ 40(5)** The analysis must terminate and produce a conservative result.

---

## § 41 Closure creation in loops and recursion

**§ 41(1)** A closure creation site executed repeatedly may produce arbitrarily many runtime
closure instances.

**§ 41(2)** The baseline analysis may represent them with one finite abstract environment
identity per creation site.

**§ 41(3)** Captured state associated with that abstract environment is joined
conservatively across possible runtime instances.

**§ 41(4)** The analysis must preserve:

```text
one callable body
potentially many runtime closure instances
potentially different captured values
```

**§ 41(5)** It must not infer shared runtime identity merely because one abstract analysis
node is used.

---

## § 42 Bounded precision

**§ 42(1)** Callable-flow analysis may bound:

- number of known concrete targets;
- number of disjunctive symbolic callable origins;
- projection depth;
- nested environment-dependency depth;
- summary complexity.

**§ 42(2)** The exact budgets are implementation/configuration choices unless another
normative rule requires a minimum.

**§ 42(3)** When a budget is exhausted, analysis must widen conservatively.

---

## § 43 Preferred callable widening

**§ 43(1)** Callable contracts are the preferred semantic widening boundary when concrete
target identity is not required for the current proof.

**§ 43(2)** Conceptually:

```text
large ClosedSet
    -> KnownSetPlusOpenContract
    -> OpenContract
```

**§ 43(3)** when a valid covering contract exists.

**§ 43(4)** The compiler must not replace an oversized target set with an unsound empty set
or with a contract that provides guarantees not shared by every covered target.

**§ 43(5)** Deep analysis may retain or recover more concrete targets when useful.

---

## § 44 Structured unknown states

**§ 44(1)** Closure analysis distinguishes at least:

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

**§ 44(2)** `NoEnvironment` is a positive proof.

**§ 44(3)** Absence of environment information is not `NoEnvironment`.

**§ 44(4)** Unknown target, unknown contract, or unknown environment must never be treated
as proof of:

```text
NoCapture
NoEscape
NoRetention
NoEffect
NoAllocation
NoBlock
```

**§ 44(5)** Unknown information should remain as local and structured as practical.

---

## § 45 Analysis budgets

**§ 45(1)** Sec distinguishes three analysis budgets:

```text
Interactive
Standard
Deep
```

**§ 45(2)** These budgets control precision, recomputation strategy, diagnostics depth, and
optional analysis work.

**§ 45(3)** They do not define different source-language semantics.

---

## § 46 Interactive analysis

**§ 46(1)** Interactive analysis is intended primarily for LSP/editor use.

**§ 46(2)** It prioritizes low latency through mechanisms such as:

```text
incremental recomputation
cached summaries
bounded target sets
bounded origin/cause-path depth
monovariant callable flow
```

**§ 46(3)** Interactive analysis must remain sound for any result presented as a proof.

**§ 46(4)** While affected analysis is being recomputed, tooling may expose an internal or
UI state equivalent to:

```text
Pending
```

**§ 46(5)** rather than claiming an outdated or incomplete result is proven.

**§ 46(6)** `Pending` is a tooling state and is not a valid compiler proof result.

---

## § 47 Standard analysis

**§ 47(1)** Standard analysis is the analysis budget used by normal compilation for closure
and callable-flow facts required by language correctness and the active
CompilationPlan.

**§ 47(2)** It must perform all closure/callable proofs required for normal Sec validity.

**§ 47(3)** A user must not need to run `sec analyse` to make an otherwise ordinary valid
program semantically safe.

**§ 47(4)** If a specific language feature or CompilationPlan requires a stronger proof,
the necessary analysis becomes required for that build rather than remaining an
optional Deep-only feature.

**§ 47(5)** Standard analysis must be deterministic for the same:

```text
source
CompilationPlan
compiler version
analysis configuration
```

**§ 47(6)** Result validity must not depend on compiler worklist order, worker scheduling,
or nondeterministic cache order.

---

## § 48 Deep analysis

**§ 48(1)** Deep analysis provides optional expensive refinement.

**§ 48(2)** It may use:

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

**§ 48(3)** Deep analysis must preserve the same source-language semantics.

**§ 48(4)** It may refine:

```text
Standard: OpenContract(C)
Deep: ClosedSet { A, B }
```

**§ 48(5)** or:

```text
Standard call-site targets: { A, B, C }
Deep call-site targets: { B }
```

**§ 48(6)** when the refinement is sound.

**§ 48(7)** Deep analysis must never make an unsafe program appear safe by applying a
different language definition.

---

## § 49 Required versus optional analysis

**§ 49(1)** Analysis required for language safety or correctness cannot be disabled when the
relevant language feature is used.

**§ 49(2)** An expensive precision refinement may be optional when the required baseline can
remain sound through conservative facts or callable contracts.

**§ 49(3)** Therefore:

```text
required analysis
    cannot be disabled when needed

optional precision
    may be configured
```

**§ 49(4)** Disabling an optional refinement must never cause a possible unsafe behavior to
be classified as impossible.

---

## § 50 Cross-analysis fixed points

**§ 50(1)** Closure/callable flow may participate in a compiler-wide fixed-point scheduler
with analyses that consume and refine related facts.

**§ 50(2)** Typical dependency relationships include:

```text
callable flow
    -> call graph
    -> callee summaries
    -> escape/effect/stack facts
    -> refined contracts/target information
```

**§ 50(3)** A compiler may iterate these analyses where required.

**§ 50(4)** Each analysis must still expose a separately defined input/output contract.

**§ 50(5)** The implementation must not replace the analysis architecture with one opaque
undocumented global fixed point.

---

## § 51 Termination

**§ 51(1)** All analysis budgets must terminate for a finite program.

**§ 51(2)** Therefore:

- callable target abstractions must be finite or widen;
- abstract environment identities must be finite;
- recursive symbolic expressions must be bounded or widen;
- interprocedural summary lattices must converge;
- Deep analysis may be expensive but must not be unbounded.

**§ 51(3)** Termination is a normative compiler requirement.

---

## § 52 Callable contracts and unknown concrete targets

**§ 52(1)** Unknown concrete target and unknown callable contract are distinct states.

**§ 52(2)** A concrete target may be unknown while a valid open callable contract still
provides the guarantees required by safety analysis.

**§ 52(3)** Conceptually:

```text
UnknownConcreteTarget + KnownContract
```

**§ 52(4)** may be sufficient.

**§ 52(5)** By contrast:

```text
UnknownConcreteTarget + UnknownContract
```

**§ 52(6)** cannot provide safety guarantees that depend on target behavior.

**§ 52(7)** Where a consumer requires guarantees such as non-retention, non-blocking,
non-allocation, non-panic behavior, or stack bounds, a missing contract must be
handled conservatively and may make the program invalid for that context.

---

## § 53 Separate compilation

**§ 53(1)** Closure analysis must remain sound when a callee body is unavailable.

**§ 53(2)** Persisted compiler summaries or explicit callable contracts provide the
necessary information.

**§ 53(3)** At minimum, separate-compilation metadata must be able to preserve externally
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

**§ 53(4)** Implementation-specific local temporaries need not be exported.

---

## § 54 Symbolic relations across module boundaries

**§ 54(1)** Persisted summaries must preserve useful symbolic relations rather than
collapsing them unnecessarily to `Unknown`.

**§ 54(2)** Examples include:

```text
returns CallableParameter(0)
returns CallableParameter(0) OR CallableParameter(1)
invokes CallableParameter(2)
returned closure environment depends on CallableParameter(0)
```

**§ 54(3)** This allows callers to instantiate imported summaries with concrete caller
facts.

---

## § 55 Summary metadata is compiler metadata

**§ 55(1)** Ordinary Sec source does not need to spell compiler-derived relations such as:

```text
ReturnsCallableParameter(0)
```

**§ 55(2)** for analyzable Sec bodies.

**§ 55(3)** Explicit contracts are primarily required where the implementation body cannot
be analyzed, such as:

- foreign code;
- opaque imported code;
- external callbacks;
- compiler-known boundaries;
- other deliberately open-world boundaries.

---

## § 56 Summary versioning

**§ 56(1)** Persisted closure/callable summaries require an internal schema/version identity.

**§ 56(2)** An incompatible summary must never be interpreted using new semantics.

**§ 56(3)** If the body is available, the compiler may invalidate and recompute the
summary.

**§ 56(4)** If the body is unavailable, the compiler must use an explicit compatible
contract or fall back conservatively.

---

## § 57 Summary invalidation

**§ 57(1)** Cached closure/callable summaries must be invalidated when relevant inputs
change.

**§ 57(2)** Relevant dependencies include at least:

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

**§ 57(3)** Incremental compilation may invalidate a smaller subset only when dependency
tracking proves it sound.

---

## § 58 Generic functions

**§ 58(1)** Generic callable summaries may be shared when the symbolic callable relationship
is independent of concrete type arguments.

**§ 58(2)** Example:

```sec
fn Identity[T](operation: fn(T) T) fn(T) T {
    return operation
}
```

**§ 58(3)** may preserve:

```text
ReturnedCallable = CallableParameter(0)
```

**§ 58(4)** across compatible specializations.

**§ 58(5)** When capture behavior, ownership, concrete field structure, callable contract,
or target-selected behavior differs by specialization, the summary must be
specialized accordingly.

---

## § 59 CompilationPlan identity

**§ 59(1)** Closure/callable analysis is target-independent when the analyzed semantic body
and relevant callable contracts are target-independent.

**§ 59(2)** A summary becomes CompilationPlan-dependent when the plan changes relevant
behavior, for example through:

```text
target-selected code
platform-specific implementation
target-specific opaque callable contract
selected specialization
target-known compiler operation
```

**§ 59(3)** CompilationPlan identity must be part of cache/summary identity whenever it can
change the result.

**§ 59(4)** The compiler need not duplicate summaries across plans when equivalence is
proven.

---

## § 60 Diagnostics

**§ 60(1)** Closure analysis primarily produces facts.

**§ 60(2)** Language diagnostics are derived by combining those facts with ownership,
lifetime, escape, storage, callable-contract, effect, concurrency, and other
normative rules.

**§ 60(3)** Diagnostics should be relational and explain the dependency chain.

**§ 60(4)** A useful diagnostic identifies as applicable:

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

## § 61 Capture diagnostic examples

**§ 61(1)** Example shape:

```text
error: captured value cannot be copied

`resource` is not Copy

the closure capture requires value transfer into its environment
```

**§ 61(2)** Example shape:

```text
error: value cannot be moved into closure environment

`resource` is still required by a deferred use
```

**§ 61(3)** Closure analysis provides the capture relation.

**§ 61(4)** Ownership/lifetime analysis provides the reason the transfer is invalid.

---

## § 62 Environment lifetime diagnostic example

**§ 62(1)** Example shape:

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

**§ 62(2)** The diagnostic should expose the shortest useful cause path rather than merely
stating `invalid closure`.

---

## § 63 Callable-contract diagnostic example

**§ 63(1)** Example shape:

```text
error: callable contract is insufficient for this operation

the concrete target of `handler` is not known

the current callable contract does not guarantee:
    NoBlock
```

**§ 63(2)** A diagnostic should identify why target precision or contract information is
insufficient for the consuming analysis.

---

## § 64 Diagnostic categories

**§ 64(1)** Closure-related diagnostics require stable diagnostic IDs assigned by the
canonical diagnostics registry.

**§ 64(2)** Conceptual categories include:

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

**§ 64(3)** The textual category names above are conceptual and do not assign the final
numeric/stable diagnostic IDs.

**§ 64(4)** Diagnostic IDs identify the rule/compiler phase, not the current severity.

---

## § 65 Precision exhaustion and diagnostics

**§ 65(1)** Precision-budget exhaustion is not normally a user error by itself.

**§ 65(2)** The compiler widens conservatively.

**§ 65(3)** If widening prevents a required safety proof, the resulting diagnostic should
explain the unresolved guarantee or unknown behavior rather than emitting a
misleading performance-only warning.

**§ 65(4)** Optional analysis tooling may expose precision-widening information for compiler
analysis and debugging.

---

## § 66 Consumers

**§ 66(1)** Closure-analysis facts are designed for reuse.

**§ 66(2)** Primary consumers include:

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

## § 67 Call-graph integration

**§ 67(1)** Closure analysis owns function-value target flow.

**§ 67(2)** Call graph owns execution relationships.

**§ 67(3)** Conceptually:

```text
Closure analysis:
    call site X target set = { A, B }

Call graph:
    X may execute A
    X may execute B
```

**§ 67(4)** Call graph must not need to build a separate competing function-value dataflow
engine.

**§ 67(5)** Closure analysis may consume call-graph SCCs and other graph structure for
interprocedural fixed points.

---

## § 68 Escape/lifetime integration

**§ 68(1)** Conceptually:

```text
Closure analysis:
    Environment(S) depends on Local(x)

Escape analysis:
    Environment(S) escapes through return

Lifetime analysis:
    Local(x) does not survive the destination lifetime
```

**§ 68(2)** The combined result is an invalid returned closure.

**§ 68(3)** Closure analysis must expose environment dependencies precisely enough for this
composition.

---

## § 69 Effect-analysis integration

**§ 69(1)** For a function-value invocation, effect analysis consumes the target set or open
callable contract.

**§ 69(2)** For a closed set:

```text
Effects(call) = union Effects(target)
```

**§ 69(3)** subject to the effect-analysis rulebook.

**§ 69(4)** For an open target set, the callable contract supplies the conservative effect
summary.

**§ 69(5)** Closure analysis does not redefine effect propagation.

---

## § 70 Stack-analysis integration

**§ 70(1)** Stack analysis consumes possible callable targets for indirect calls.

**§ 70(2)** A closed finite target set allows stack analysis to consider every possible
callee.

**§ 70(3)** An open target set requires sufficient callable stack-bound contract information
where a proven bound is required.

**§ 70(4)** Deep callable-flow refinement may tighten stack estimates without changing
source semantics.

---

## § 71 ISR and concurrency integration

**§ 71(1)** ISR and concurrency analyses may query callable-flow and environment facts such
as:

```text
may this callable invoke a blocking target?
may it allocate?
does its closure environment depend on forbidden storage?
does it depend on thread-local state?
may it escape to another execution context?
```

**§ 71(2)** Closure analysis provides callable/environment facts.

**§ 71(3)** The consuming rulebook determines legality.

---

## § 72 Parameter-usage integration

**§ 72(1)** Parameter-usage analysis may consume callable facts such as:

```text
Invoked
NotReturned
NotRetained
NotStored
```

**§ 72(2)** when those facts can be proven through closure and escape analysis.

**§ 72(3)** Closure analysis itself does not issue API-shape advisories that belong to
parameter-usage analysis.

---

## § 73 Semantic IR requirements

**§ 73(1)** The compiler's canonical semantic representation must preserve enough
information to represent closure/callable facts without reverse-engineering them
from low-level pointer operations.

**§ 73(2)** At minimum, Semantic IR or equivalent compiler state must preserve the semantic
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

**§ 73(3)** Exact IR operation names are implementation-defined.

**§ 73(4)** Sec MLIR lowering may consume these facts but must not become the normative
source of closure semantics.

---

## § 74 No runtime analysis requirement

**§ 74(1)** Closure analysis is compile-time compiler metadata.

**§ 74(2)** This rulebook does not require:

```text
runtime CFA graph
runtime closure-analysis registry
runtime call graph
runtime ownership table
runtime borrow tracker
runtime reflection metadata
```

**§ 74(3)** Runtime closure/environment representation exists only as required by the
actual program semantics and lowering strategy.

---

## § 75 LSP integration

**§ 75(1)** The LSP consumes the Interactive analysis budget by default unless project
configuration selects a different supported analysis depth.

**§ 75(2)** Useful closure/callable LSP information may include:

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

**§ 75(3)** LSP output must distinguish information the programmer needs from information
the programmer may merely want.

---

## § 76 Required LSP information versus optional insight

**§ 76(1)** Two presentation classes are required conceptually:

```text
NeedToKnow
OptionalInsight
```

**§ 76(2)** `NeedToKnow` includes information necessary to understand:

- an error or warning;
- a safety condition;
- an unresolved callable contract;
- a retention/lifetime requirement;
- another condition the programmer must act on.

**§ 76(3)** `OptionalInsight` includes useful exploratory information that is not required to
understand program validity.

**§ 76(4)** Examples may include:

```text
call count
number of callers
full possible-target list when not needed for a diagnostic
extended cause paths
deep-analysis narrowing information
additional optimization/advisory facts
```

**§ 76(5)** Optional insight must be configurable rather than forced into every hover.

---

## § 77 Configurable LSP analysis depth

**§ 77(1)** LSP analysis depth is project-configurable.

**§ 77(2)** A small project on a machine with ample resources may select continuous Deep
analysis.

**§ 77(3)** A large project or resource-constrained machine may select a cheaper Interactive
analysis profile.

**§ 77(4)** The selected profile changes precision and optional information, not language
semantics.

**§ 77(5)** A future LSP synchronization rule/appendix must require the LSP to:

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

**§ 77(6)** This rulebook defines the closure-analysis integration requirement. The LSP
rulebook owns the complete LSP configuration/watch behavior.

---

## § 78 Hover behavior

**§ 78(1)** Hover output must not become an unconditional dump of all available compiler
analysis facts.

**§ 78(2)** For example, a function call count may be useful but is normally optional
insight rather than mandatory hover content.

**§ 78(3)** A project/editor configuration may request richer hover information when the
analysis budget and available resources make that appropriate.

**§ 78(4)** Facts necessary to explain a current diagnostic or required safety contract
must remain available regardless of optional hover-detail settings.

---

## § 79 `sec analyse`

**§ 79(1)** The compiler provides a dedicated analysis command conceptually named:

```text
sec analyse
```

**§ 79(2)** Default behavior is to run all available analyses:

```text
sec analyse
    == sec analyse --all
```

**§ 79(3)** The user may explicitly select one supported analysis instead of running all of
them.

**§ 79(4)** The analysis-name vocabulary and full CLI syntax are owned by
`rules/tooling/cli.md` (MD-017). Closure analysis has the single registered
name `closures`: `sec analyse closures` selects it individually. It also runs
as part of `sec analyse` and `sec analyse --all` (§ 4.2).

**§ 79(5)** Closure deep analysis may expose information such as:

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

**§ 79(6)** `sec analyse` is not required for normal semantic correctness; it is the default
entry point for full/deep analysis and analysis reporting.

---

## § 80 Implementation governance

**§ 80(1)** This rulebook contains normative requirements only.

**§ 80(2)** Mutable implementation progress belongs in:

```text
implementation-status.yaml
```

**§ 80(3)** A closure-analysis ledger entry should track granular capabilities rather than a
single misleading binary implemented/not-implemented flag.

**§ 80(4)** Useful capability classes include:

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

**§ 80(5)** Current status, source-file paths, verification commands, and remaining work are
governance data and are not normative language semantics.

---

## § 81 Required capture tests

**§ 81(1)** The Sec 0.1 test suite must cover at least:

```text
copyable value capture
move-only value capture
invalid move capture
invalid copy capture
mutable owned captured binding and `mut fn` capability inference
explicit move capture of a copyable value and `-> fn` capability inference
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

## § 82 Required callable-flow tests

**§ 82(1)** The test suite must cover at least:

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

## § 83 Required interprocedural tests

**§ 83(1)** The test suite must cover at least:

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

## § 84 Required escape/lifetime integration tests

**§ 84(1)** The test suite must cover at least:

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

**§ 84(2)** The final diagnostic may be owned by escape, lifetime, storage, ownership, or
concurrency validation, but the closure dependency facts must participate.

---

## § 85 Required CFA precision tests

**§ 85(1)** The test suite must verify at least:

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

## § 86 Analysis-budget tests

### § 86.1 Interactive

**§ 86.1(1)** Tests must verify:

```text
sound bounded result
incremental invalidation
Pending state allowed while recomputing
no stale incomplete fact presented as Proven
```

### § 86.2 Standard

**§ 86.2(1)** Tests must verify:

```text
deterministic result
all required correctness proofs
same result independent of compiler worker scheduling/worklist order
```

### § 86.3 Deep

**§ 86.3(1)** Tests must verify:

```text
same source semantics
precision equal to or stronger than Standard where refinement succeeds
no weaker safety guarantee
context-sensitive refinement remains sound
```

**§ 86.3(2)** Deep analysis must not make an invalid program valid by changing the language
rules.

---

## § 87 LSP integration tests

**§ 87(1)** The complete LSP test matrix belongs to the LSP rulebook/appendix.

**§ 87(2)** Closure integration must nevertheless support tests equivalent to:

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

## § 88 Performance and termination tests

**§ 88(1)** Compiler regression tests should include:

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

**§ 88(2)** No normative wall-clock threshold is defined here.

**§ 88(3)** The tests must protect:

```text
termination
bounded analysis memory according to configured budgets
deterministic widening
sound target coverage
```

---

## § 89 Completion criteria for Sec 0.1

**§ 89(1)** Closure analysis is complete for Sec 0.1 when all of the following hold:

1. all Sec 0.1 copy, move, shared-reference, and
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

## § 90 Canonical design summary

**§ 90(1)** The normative closure-analysis model can be summarized as follows:

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

Captures are explicit. Sec 0.1 supports owned copy, owned move,
shared-reference, and mutable-reference capture. Owned environment bindings are
mutable; borrowed captures retain their borrow and lifetime dependencies.

Capture analysis operates on canonical Place/provenance and consumes ownership,
lifetime, borrow, storage, and dependency facts rather than recreating those
analyses.

Capture mode and transfer mechanism are distinct. Plain value capture may use
Copy or Move according to normal Sec ownership semantics, while
explicit move capture requires Move even for a copyable value. Plain
`capture(value)` is always a copy request and is invalid for a non-copyable
reusable source.

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
