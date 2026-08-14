# Escape Analysis

## Status

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the semantic purpose, analysis domain, dataflow model,
interprocedural summaries, conservative behavior, consumers, diagnostics
requirements, and completion criteria of Sec escape analysis.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.

---

# Purpose

Escape analysis determines whether and how a value, reference, view, address,
callable environment, handle, or storage dependency becomes usable outside the
context for which its current semantic dependencies were established.

Escape analysis is a semantic dependency-flow analysis.

It is not a storage-placement algorithm.

It must not silently choose heap allocation, arena allocation, copying,
retention, or another ownership/storage strategy in order to make an otherwise
invalid program compile.

Escape analysis exists to provide reusable facts for:

- lifetime validation;
- reference validation;
- ownership validation;
- storage validation;
- closure analysis;
- parameter-usage analysis;
- stack analysis;
- call-graph refinement;
- effect analysis;
- concurrency and ISR analysis;
- diagnostics and LSP tooling;
- optimization;
- later compiler lowering where an escape proof is useful.

---

# Normative role

This rulebook is normative for:

- the definition of semantic escape;
- the distinction between value escape, borrow escape, address escape,
  retention, capture, concurrency transfer, and foreign retention;
- the canonical classes of escape subjects and destinations;
- the facts produced by escape analysis;
- the required interaction with canonical Place provenance;
- control-flow joins and bounded path-disjunctive provenance;
- loop and recursion fixed points;
- interprocedural escape summaries;
- conservative treatment of unresolved calls and opaque retention;
- aggregate, container, view, closure, handle, and backing-storage dependency
  propagation;
- separate-compilation summary requirements;
- diagnostic information required from escape facts;
- analysis consumers;
- required test classes;
- completion criteria for Sec 0.1 escape analysis.

This rulebook does not redefine:

- ownership;
- copy or move legality;
- object lifetime;
- storage lifetime;
- storage origin;
- backing relation;
- reclamation authority;
- reference validity;
- borrow legality;
- closure capture syntax;
- FFI retention syntax;
- task/thread ownership rules;
- physical calling convention;
- physical stack placement.

Those semantics remain owned by their respective normative rulebooks.

---

# Relationship to other analyses

Escape analysis is intentionally separate from lifetime analysis.

Conceptually:

```text
Escape analysis:
    Where and how does a value, capability, or dependency travel?

Lifetime analysis:
    Does the source remain valid for the destination's required lifetime?
```

Escape analysis is also separate from ownership analysis:

```text
Ownership analysis:
    Who owns the value, and is copy/move/consume legal?

Escape analysis:
    Does that value or one of its dependencies become usable elsewhere?
```

Escape analysis is separate from storage analysis:

```text
Storage facts:
    Where does storage come from, how is it invalidated, and who may reclaim it?

Escape facts:
    Where does a dependency on that storage travel?
```

These analyses cooperate through explicit facts and shared canonical Place
provenance rather than duplicating one another's semantic models.

---

# Core rule

Escape is a semantic dependency-flow property, not a physical storage-placement
decision.

The following are distinct:

```text
value escape
reference escape
view escape
address escape
callable-environment escape
handle escape
backing-storage dependency
physical storage escape
```

Returning or transferring an owned value does not imply that the source
`Automatic` storage itself escapes.

For example:

```sec
fn MakeValue() LargeStruct {
    let value := LargeStruct { }
    return value
}
```

The value may be transferred to the caller while the callee's `Automatic`
storage does not survive the function invocation.

The compiler may implement this through result storage, semantic move, SSA
elision, or another semantics-preserving lowering.

Escape analysis must not reinterpret such a transfer as implicit storage
promotion.

---

# No implicit escape promotion

Sec does not repair escape by changing storage behind the programmer's back.

An analysis result such as:

```text
reference to Automatic local
    +
return to caller
```

must never trigger an implicit transformation such as:

```text
promote local to heap
```

or:

```text
deep-copy backing storage
```

or another source-invisible ownership/storage change.

When the required lifetime or retention cannot be provided by the program's
explicit semantics, safe code is invalid.

This rule applies even when the active target provides a heap allocator.

---

# Escape is not a boolean

The primary result of escape analysis must not be only:

```text
Escapes = true
```

The analysis must retain enough information to determine:

- what escapes;
- how it escapes;
- where it escapes;
- what semantic origins/dependencies are carried;
- whether the behavior is known or conservative unknown.

The canonical conceptual dimensions are:

```text
EscapeSubject
EscapeMode
EscapeDestination
```

The exact implementation representation is not prescribed.

---

# Escape subjects

Canonical escape subjects include:

```text
OwnedValue
SafeReference
View
RawPointer
CallableEnvironment
Handle
StorageDependency
Unknown
```

## OwnedValue

Ownership of the value itself is transferred or made available in another
context.

Examples include:

```sec
return value
```

and ownership transfer into another owner.

Owned-value escape may be ordinary and valid.

It does not imply that source storage must escape.

## SafeReference

A safe reference becomes usable outside the context in which its current
validity dependencies were established.

This is commonly the subject relevant to returned-borrow and retained-borrow
validation.

## View

A non-owning or view-like descriptor becomes usable elsewhere while continuing
to depend on backing storage.

Examples include:

- slices;
- borrowed collection views;
- subranges;
- other descriptor values that do not own the storage they expose.

## RawPointer

A raw pointer/address capability may escape even when safe-reference guarantees
no longer apply.

Unsafe code does not make pointer flow invisible to escape analysis.

Raw-pointer escape facts remain relevant to:

- FFI analysis;
- closure analysis;
- task/thread transfer analysis;
- ISR analysis;
- target/hardware analysis;
- diagnostics and compiler auditing.

## CallableEnvironment

A capturing callable may carry an environment whose dependencies escape with the
callable value.

The exact capture model is refined by `closure_analysis.md`.

Escape analysis must nevertheless be able to represent environment dependencies
and their movement.

## Handle

A handle may intentionally be designed to survive address relocation or storage
reuse through a generation/domain protocol.

Handles must not be silently treated as ordinary safe references.

## StorageDependency

Some values expose no direct borrowed reference while still requiring backing
storage to remain valid.

Examples include view descriptors and wrappers whose validity depends on a
storage domain.

The dependency must follow the value when the value escapes.

## Unknown

When the subject or dependency behavior cannot be resolved soundly, the analysis
must represent conservative unknown rather than assume non-escape.

---

# Escape modes

Canonical escape modes include:

```text
ValueTransfer
BorrowEscape
AddressEscape
Retention
Capture
ConcurrencyTransfer
ForeignRetention
Unknown
```

Multiple modes may apply to one source across different flows.

## ValueTransfer

Ownership of a value transfers to a different context.

This is distinct from retaining a borrow.

## BorrowEscape

A safe reference or view becomes usable beyond its current borrow/lifetime
context.

Example:

```sec
fn Broken() ref int {
    int value := 10
    return ref value
}
```

Escape analysis determines that the returned reference originates from the
local value.

Lifetime/storage validation determines that the origin cannot satisfy caller
use.

## AddressEscape

An address capability such as a raw pointer leaves its original context.

Address escape remains visible even in unsafe code.

## Retention

A callee, aggregate, registry, callback mechanism, or other destination may keep
a value/dependency after the immediate operation returns.

Retention is not implied merely by crossing a function-call boundary.

## Capture

A dependency becomes part of a callable environment.

Capture does not itself imply that the environment escapes the function.

The later flow of the callable determines the destination/lifetime demand.

## ConcurrencyTransfer

A value, capability, callable, handle, or dependency becomes available in
another execution context such as a task or thread.

Ownership, lifetime, Send-like restrictions, and synchronization remain separate
questions.

## ForeignRetention

Foreign code may retain a value/address/callback beyond the immediate call.

The allowed retention is determined by the FFI contract.

## Unknown

When retention or transfer behavior cannot be resolved, the result must remain
conservative.

---

# Escape destinations

Canonical destination classes include:

```text
Caller
OuterPlace
Aggregate
RetainingCall
ClosureEnvironment
Task
Thread
StaticStorage
ThreadLocalStorage
ForeignCode
ReturnedValue
Unknown
```

The implementation may attach additional structured information.

## Caller

A returned reference/view/handle remains available to the caller.

This can be valid when the returned dependency originates from caller-provided
storage.

## OuterPlace

A value or dependency is stored in a place whose required validity extends
beyond the current inner context.

## Aggregate

A dependency is placed into an aggregate/container that may later escape.

Storing into an aggregate does not by itself prove long-lived escape; the
carrier's later flow determines that.

## RetainingCall

A callee contract permits retention after call return.

## ClosureEnvironment

The dependency becomes part of a callable environment.

## Task / Thread

The dependency crosses execution-context boundaries.

## StaticStorage / ThreadLocalStorage

The dependency is stored in storage whose required lifetime is governed by the
static or thread-local domain.

## ForeignCode

The dependency enters foreign code under a defined or conservative retention
contract.

## ReturnedValue

A carrier aggregate, callable, view, handle, or other value is returned.

## Unknown

The destination/lifetime demand cannot be resolved soundly.

---

# Function and lexical boundaries

A function boundary does not by itself imply long-lived escape.

A lexical scope boundary does not by itself define all escape behavior.

For example:

```sec
Inspect(ref value)
```

with a proven non-retaining call contract represents call-duration use, not
retention beyond the call.

Conversely:

```sec
Register(ref value)
```

with a retaining contract may create an escape even though the source and call
occur in the same lexical scope.

Escape is determined by future usability and retention requirements, not merely
by syntax nesting.

---

# Call retention contracts

Call behavior must distinguish at least:

```text
NoRetention
MayRetain
TransfersOwnership
UnknownRetention
```

These are conceptual categories. The exact function/FFI metadata representation
belongs to the corresponding function/FFI/compiler metadata rules.

## NoRetention

The callee may use the argument only for the call's permitted dynamic extent and
may not retain the dependency after return.

## MayRetain

The callee may keep the argument or a dependency derived from it after return.

## TransfersOwnership

The callee consumes/transfers ownership according to the function's ownership
contract.

This is not equivalent to retaining a borrow.

## UnknownRetention

Retention behavior is unresolved or unavailable.

Unknown retention is conservative for reference-like/address-like/callable
arguments unless another explicit contract supplies stronger guarantees.

---

# Canonical sources

Escape summaries and diagnostics need symbolic origin categories.

Canonical conceptual sources include:

```text
Parameter
Receiver
Local
Temporary
Static
ThreadLocal
ArenaStorage
AllocatorBackedStorage
ForeignStorage
CapturedValue
ReturnedCallResult
Unknown
```

Source classification describes origin.

It does not itself decide validity, lifetime, ownership, or physical placement.

---

# Canonical sinks

Canonical conceptual escape sinks include:

```text
Return
OuterAssignment
AggregateStore
StaticStore
ThreadLocalStore
RetainingCall
ClosureCapture
TaskTransfer
ThreadTransfer
ForeignCall
UnknownCall
HandleConstruction
```

These sinks represent operations or destinations that may extend dependency
usability beyond the current context.

---

# Place provenance

Escape analysis must consume canonical Place provenance.

It must not replace the compiler's Place model with a separate symbol-only
origin system.

Relevant projections include, where supported by canonical Place analysis:

```text
field
property where semantically addressable
constant index
dynamic index
slice/range
reference dereference
union variant payload
nested aggregate path
```

Escape precision should preserve these projections whenever doing so is sound
and within the analysis precision budget.

For example:

```sec
return ref pair.left
```

should preserve:

```text
Parameter(0).Field(left)
```

when `pair` is a parameter reference and the Place model can represent that
projection.

It must not unnecessarily collapse the origin to the whole parameter.

---

# Disjoint places

When canonical Place analysis proves places disjoint, escape analysis should
preserve that precision.

Examples include, where proven:

```text
obj.left    vs obj.right
array[3]    vs array[7]
slice[0..4] vs slice[8..12]
```

Escape of one disjoint projection must not automatically classify unrelated
projections as escaping.

---

# Dynamic indices and projections

When an exact index or projection cannot be known, escape analysis may widen
locally.

For example:

```text
Parameter(0).Index(3)
```

may widen to:

```text
Parameter(0).AnyIndex
```

without discarding the known root `Parameter(0)`.

Conservative uncertainty should remain as local and structured as practical.

---

# Views and slices

A view carries a dependency on its backing storage.

Example:

```sec
fn Tail(values: ref int[]) ref int[] {
    return values[1..]
}
```

The returned view must preserve that its backing dependency derives from
`values`.

Static range information should be retained where known.

Dynamic ranges may use an unknown/dynamic range projection while preserving the
known backing root.

Escape of a slice/view does not imply ownership escape of the complete backing
aggregate.

It does imply that the required backing-storage dependency travels with the
view.

---

# Aggregates as dependency carriers

Aggregates may contain:

- owned values;
- safe references;
- views;
- handles;
- callable environments;
- other dependency-carrying values.

Escape analysis must track relevant contained dependencies structurally.

Example:

```sec
type Pair struct {
    item: ref Item
    count: int
}
```

Returning the complete `Pair` carries the dependency in `item`.

Reading or copying only `count` does not cause the `item` dependency to escape.

---

# Containers

Placing a dependency in a local container does not automatically imply escape
outside the function.

Example:

```sec
localCollection.Add(ref local)
```

establishes a contained dependency.

If the collection remains local and is destroyed before the dependency becomes
invalid, the operation may be valid.

If the collection later escapes, the contained dependency escapes transitively.

For dynamic collections where exact element identity cannot be represented
cheaply, the analysis may use conservative summarized element provenance.

Sec 0.1 does not require a fully precise shape analysis for every collection
implementation.

Compiler-known collection contracts may supply summary behavior.

---

# Fixed arrays

Fixed arrays may be tracked per element when static index precision is practical.

For large or dynamically indexed arrays, analysis may widen to summarized
projections such as:

```text
AnyIndex
```

The compiler must not require one analysis node per physical element of a large
array solely to remain sound.

---

# Tagged unions and Result/Option payloads

When the active variant is known, escape dependencies should be associated with
the active payload projection.

Inactive payload storage must not be treated as a live dependency.

When control flow merges several possible active variants, dependencies from all
possible live variants are joined conservatively.

The same principle applies to `Result` and `Option` payloads where represented
as tagged semantic alternatives.

---

# Current provenance versus historical escape

Escape analysis must distinguish:

```text
current provenance / current carrier dependencies
```

from:

```text
historical escape facts
```

Current provenance may change through mutation, rebinding, move, destruction,
or replacement.

Historical escape facts are monotone.

For example:

```sec
holder.ref = ref a
holder.ref = ref b
```

after the second assignment, `holder.ref` no longer currently depends on `a`
when no alias retains the old value.

However, if the earlier value was already passed to a retaining call, that
historical escape remains true.

---

# Assignment

Dependency propagation through assignment must follow Sec's semantic
classification of the operation.

Escape analysis must not infer copy versus move merely from assignment syntax.

It consumes ownership/copy-move classification.

## Copy

Copy creates a new value according to the type's copy semantics.

Contained reference/view/handle dependencies are copied according to their own
semantic rules.

Escape analysis does not decide whether a type is copyable.

## Move

Move transfers the relevant dependencies to the destination and updates source
availability/ownership according to ownership analysis.

The analysis must not model semantic move as if source and destination both
remain independent owners.

## Borrow/view rebinding

Rebinding replaces the current provenance of the destination reference/view.

It does not erase historical escape facts produced by the previous binding.

---

# Partial moves and structural updates

Partial move, replacement, and reinitialization must operate at canonical Place
projection granularity.

A move from:

```text
object.left
```

must not automatically invalidate unrelated `object.right` escape provenance
when ownership/Place semantics keep the sibling valid.

Escape analysis must consume the compiler's authoritative partial-availability
facts rather than implement its own competing partial-move legality model.

---

# Destruction and object lifetime

The following events remain distinct:

```text
destruction
object lifetime end
storage reclamation
storage epoch transition
```

Destroying a carrier ends its future ability to carry dependencies after its
object lifetime ends.

Destruction does not retroactively erase escape that already occurred through
another value/capability.

Escape analysis must not redefine lifetime validity.

Lifetime analysis owns whether later use is legal.

---

# Storage domains, epochs, and generations

Escape analysis preserves dependencies needed by the storage/reference model.

A dependency may include relation to:

```text
storage domain
epoch/generation requirement
backing storage
reclamation/protection contract
```

An epoch transition may make a previous dependency stale.

It does not erase the historical fact that the dependency escaped.

Handles that validate generation at runtime may remain valid semantic values
across address reuse according to their own contracts.

Escape analysis must not collapse handles into ordinary direct references.

---

# Returned owned values

A returned owned value must be distinguished from a returned borrow into local
storage.

Example:

```sec
fn Make() Buffer {
    let buffer := Buffer(...)
    return buffer
}
```

The return summary should represent an owned result such as:

```text
FreshOwnedValue
```

or an equivalent symbolic result.

It must not claim that the caller receives a borrow into the callee's local
storage merely because the source variable was local.

---

# Returned references and views

Returned borrowed values must preserve symbolic origin paths.

Examples include:

```text
Parameter(0)
Receiver
Parameter(1).Field(data)
Parameter(0).Index(3)
Parameter(0).Slice(2..8)
Captured(0)
Static(GlobalX)
Unknown
```

Multiple possible origins must remain representable:

```text
Parameter(0) OR Parameter(1)
```

Returned views must additionally preserve relevant backing dependencies.

---

# Calls

Call transfer uses a callee escape summary or an explicit contract.

Conceptually, for:

```sec
let result := F(arg0, arg1)
```

analysis performs:

1. obtain the applicable summary/contract for `F`;
2. substitute symbolic `Parameter(0)` origins with the provenance of `arg0`;
3. substitute symbolic `Parameter(1)` origins with the provenance of `arg1`;
4. substitute the receiver where applicable;
5. apply symbolic projections;
6. propagate returned origins/dependencies to `result`;
7. apply callee retention/transfer/capture/concurrency/foreign effects to the
   arguments;
8. preserve source evaluation order for operand-side effects.

The implementation may optimize this process but must preserve the same facts.

---

# Function summaries

Every analyzable function must be able to produce a composable symbolic escape
summary.

A conceptual summary includes:

```text
EscapeSummary {
    Parameters
    Receiver
    Return
    Retention
    Captures
    ConcurrencyTransfers
    ForeignTransfers
    UnknownEffects
}
```

The exact in-memory and persisted representation is implementation-defined.

The semantic information is normative.

---

# Parameter summary facts

For each parameter, the summary must be able to represent combinations of facts
equivalent to:

```text
NoEscape
Returned
Retained
StoredInEscapingCarrier
OwnershipTransferred
Captured
TransferredToTask
TransferredToThread
PassedToForeign
UnknownRetention
```

These are not necessarily mutually exclusive.

A parameter may, for example, be both `Returned` and `Retained`.

---

# Receiver summary facts

Method receivers must be represented explicitly in summaries.

Example:

```sec
fn GetData() ref Buffer {
    return ref self.data
}
```

must be able to produce a symbolic result equivalent to:

```text
Receiver.Field(data)
```

Receiver-origin flows must not be collapsed to `Unknown` solely because the
receiver is implicit in source syntax.

---

# Return summaries

A return summary must be able to represent:

```text
FreshOwnedValue
Parameter(index)
Receiver
Static(symbol)
ThreadLocal(symbol)
Captured(index)
Unknown
```

and applicable projections such as:

```text
Field(name)
Index(constant)
AnyIndex
Slice(range)
UnknownProjection
```

and disjunctions of possible sources.

The summary must also carry relevant storage/backing dependencies for view- and
handle-like returned values.

---

# NoEscape

`NoEscape` is a positive proof.

It may be produced only when analysis proves that the relevant dependency:

- is not returned;
- is not retained;
- is not stored into an escaping carrier;
- is not captured by an escaping callable;
- is not transferred to a task/thread;
- is not exposed to foreign retention;
- does not pass through unresolved behavior that may retain it.

Absence of a discovered escape is not automatically `NoEscape`.

---

# Unknown escape

The analysis must distinguish:

```text
ProvenNoEscape
KnownEscape
UnknownEscape
```

`UnknownEscape` is a first-class conservative result.

Typical causes include:

- unresolved function-value target;
- missing persisted summary;
- opaque imported implementation;
- unknown foreign retention;
- precision exhaustion;
- unsupported call form;
- unknown compiler-known contract.

Unknown must never be used as proof of non-escape.

---

# Function-value calls

Function-value calls must consume callable-target information when available.

For example, if a separate callable-flow analysis proves:

```text
Targets(f) = { A, B }
```

then the escape result of calling `f` must conservatively combine the applicable
summaries of `A` and `B`.

When the target set is unknown, the compiler uses the function/call contract if
one exists and otherwise falls back conservatively.

The full callable-target/capture model is refined by `closure_analysis.md`.

Escape analysis must remain sound before that additional precision exists.

---

# Closures

A capturing callable establishes explicit dependencies from its environment to
captured entities.

Escape analysis distinguishes at least non-capturing callables, non-escaping
owned closures, escaping owned closures, shared-borrow closures,
mutable-borrow closures, and consuming closures.

For example, conceptually:

```text
ClosureEnvironment
    depends on Local(state)
```

If the closure remains local, the dependency may remain local.

If the closure is returned, retained, stored into an escaping carrier, or
transferred to another execution context, its contained dependencies escape
transitively.

`closure_analysis.md` owns detailed capture mode and callable-flow precision.

Escape analysis owns the movement/retention facts once those dependencies are
known.

Owned closures may escape when their environment receives sufficient lifetime.
Borrowed closures may escape only while every captured borrow remains valid.
Environment storage is compiler-selected and may be eliminated, lexical,
regional, static, or dynamically owned; escaping does not itself promise heap
allocation. Any selected allocation or resource effect remains visible to the
relevant analyses.

---

# Task and thread transfer

Task/thread transfer must remain explicit escape information.

The analysis must distinguish semantic ownership transfer from borrowed
cross-context access.

Conceptually useful distinctions include:

```text
MoveToTask
BorrowToTask
MoveToThread
BorrowToThread
```

or equivalent structured facts.

Escape analysis classifies the transfer.

Other rules/analyses decide:

- whether ownership transfer is legal;
- whether the dependency lifetime is sufficient;
- whether thread/task sharing is permitted;
- whether synchronization is required.

---

# FFI and foreign retention

Foreign calls must use explicit retention contracts defined by the FFI model.

Escape analysis must be able to consume retention classes equivalent to:

```text
CallOnly
MayRetainUntilReturnOfHandle
MayRetainUntilExplicitRelease
MayRetainIndefinitely
TransfersOwnership
Unknown
```

The exact source syntax and FFI declaration format are defined by `ffi.txt`.

Unknown foreign retention is conservative.

Safe borrowed values must not rely on an unknown foreign lifetime.

Raw-pointer escape in unsafe code may remain permitted while still being
recorded as escape.

---

# Static and thread-local destinations

Storing a borrowed dependency into `Static` or `ThreadLocal` storage must be
represented as an escape to the corresponding storage domain.

Escape analysis reports the destination and origin.

Lifetime/storage validation determines whether the dependency can satisfy the
required lifetime.

Owned value transfer remains separate from borrowed dependency retention.

---

# Interprocedural composition

Escape summaries must be composable.

For a call chain:

```text
F -> G -> H
```

`F` should be able to obtain a correct symbolic summary without every caller
re-analyzing all of `H` from source each time.

Summaries therefore use symbolic parameter/receiver/capture/static paths rather
than caller-local Places.

---

# Recursion and SCCs

Direct recursion, mutual recursion, recursive method cycles, and recursive
callable-summary cycles require fixed-point analysis.

Call graph SCC information should be reused where applicable.

The result must be independent of source declaration order.

The implementation may use bottom-up monotone discovery, conservative initial
states, or another sound fixed-point algorithm.

The normative requirements are:

- soundness;
- monotonicity;
- termination;
- declaration-order independence;
- conservative handling of unresolved behavior.

---

# Forward dataflow model

Escape analysis is a monotone forward dataflow analysis over canonical Semantic
IR control flow and Place provenance.

Conceptually, a relevant analysis state includes:

```text
Origins
Dependencies
EscapeFacts
CarrierFacts
Precision
```

The exact representation is implementation-defined.

---

# Current state and historical state

The analysis must model at least two kinds of information:

```text
current value/carrier provenance
historical escape facts
```

Current provenance may be replaced or invalidated.

Historical escape facts only accumulate.

This distinction is required for sound mutation/rebinding analysis.

---

# Control-flow join

At a join point, all possible continuing-path origins and escape facts must be
preserved.

Conceptually:

```text
Origins      = union of possible origins
Dependencies = conservative union/structural join
EscapeFacts  = union
CarrierFacts = structural join
```

The analysis must not arbitrarily choose one predecessor origin.

---

# Path-disjunctive provenance

The analysis should preserve multiple alternative origins when practical.

For example:

```sec
ref Item r

if condition {
    r = ref a
} else {
    r = ref b
}

return r
```

must preserve the possibility:

```text
Origin(r) = a OR b
```

until a valid conservative widening is required.

---

# Bounded precision

Path-disjunctive precision may be bounded.

The bound is an implementation tuning parameter and may change between compiler
versions.

When the precision budget is exhausted, analysis must widen conservatively.

It must never:

- discard a possible escape;
- choose one arbitrary source;
- convert uncertainty into `NoEscape`.

---

# Structured widening

Widening should preserve as much independently known structure as practical.

A preferred conceptual progression is:

```text
Parameter(0).Field(a).Index(3)
        ↓
Parameter(0).Field(a).AnyIndex
        ↓
Parameter(0).AnyProjection
        ↓
Parameter(0)
        ↓
Unknown
```

The implementation need not use these exact classes.

The normative rule is that loss of one dimension of precision should not force
unnecessary loss of unrelated known facts.

---

# Loops

Loops require fixed-point dataflow.

The loop state must conservatively combine:

```text
entry state
normal body fallthrough
continue edges
other continuing back-edges
break exit states for post-loop analysis
```

Zero-iteration behavior must be preserved where the language permits it.

Iteration-local bindings must not be incorrectly retained across iterations.

---

# Loop widening

Loop-carried symbolic paths may otherwise grow without bound.

The analysis must widen projections/origin sets where necessary to guarantee
termination.

Widening should preserve known roots and known storage/dependency classes where
possible.

---

# Summary termination

Interprocedural summary construction must also guarantee termination.

Recursive symbolic path growth such as conceptually:

```text
x.field.field.field...
```

must be bounded through projection widening or equivalent finite abstraction.

---

# Error, panic, and cleanup paths

Escape analysis must operate over the canonical control-flow representation.

Success, explicit error, panic, and cleanup paths participate according to their
actual semantics.

Dependencies that exist only on one path must not be invented on unrelated
paths.

When path precision is merged, the result must remain conservative.

---

# Defer and delayed uses

A deferred operation creates a future use at its defined execution point.

A deferred use may extend the required internal validity of a dependency.

It does not automatically imply escape outside the function.

Escape analysis must consume canonical defer/cleanup control flow or equivalent
delayed-use facts.

It must not assume that a panic means no deferred cleanup/use occurs when the
language requires cleanup to run.

---

# Unreachable paths

Proven unreachable runtime paths do not contribute escape facts.

Compile-time-eliminated paths do not contribute runtime escape facts once the
language/compiler semantics have definitively removed them.

Optimizer speculation must not be used to weaken source-level safety.

---

# Separate compilation

Escape analysis must remain sound when a callee body is unavailable.

The compiler must be able to consume a persisted escape summary or explicit
function/FFI contract.

The minimum persisted semantic information includes, where applicable:

```text
parameter escape/retention facts
receiver escape/retention facts
return symbolic origins
ownership transfer relevant to escape
capture/retention behavior
concurrency transfer
foreign retention
unknown/conservative marker
summary schema/version
```

Persisted summaries are compiler metadata.

They are not user-written lifetime annotations.

---

# Persisted summary validation

An imported summary must be validated against the relevant identity information,
including as applicable:

```text
function identity
function type/signature
generic specialization identity
relevant semantic/compiler version
escape-summary schema version
CompilationPlan-dependent identity
```

A stale or incompatible summary must never be used for a safety proof.

If the function body is available, the summary may be recomputed.

If the body is unavailable and no compatible contract exists, analysis must fall
back conservatively.

---

# Summary versioning

Persisted escape summaries require an internal schema/semantic version.

The escape-summary version is independent of the Sec language version.

When the meaning or schema changes incompatibly, old summaries must be rejected,
invalidated, or conservatively ignored.

The compiler must never silently reinterpret old summary data with new meaning.

---

# Generic functions

A generic function may share a generic escape summary when escape behavior is
independent of concrete specialization.

A specialization-specific summary is required when escape behavior depends on
facts such as:

- ownership/copy traits;
- concrete field structure;
- callable behavior;
- target-selected implementation;
- specialization-selected code;
- other type-dependent semantics.

The implementation may reuse equivalent summaries when equivalence is proven.

---

# CompilationPlan dependence

Escape semantics should remain target-independent when the analyzed semantic
body is target-independent.

When target/profile/configuration selection changes the semantic body or
contracts, the applicable `CompilationPlan` identity must participate in cache
selection/invalidation.

A compiler must not reuse a summary across plans when the relevant escape
behavior differs.

---

# Summary invalidation

Cached escape summaries must be invalidated when relevant dependencies change.

These include, as applicable:

```text
function body
callee summary
resolved call target
function contract
foreign retention contract
callable target set
generic specialization
Place/provenance semantics
relevant type semantics
CompilationPlan-selected path
analysis algorithm/schema version
```

Incremental compilation may use dependency hashes or equivalent mechanisms.

The normative requirement is correctness, not one cache implementation.

---

# Public compiler-facing result

Escape analysis must expose reusable compiler-facing facts.

A conceptual result is:

```text
EscapeAnalysisResult {
    FunctionSummaries
    PlaceFacts
    ValueFacts
    UnknownFacts
}
```

This is not a required Go type.

The normative requirement is that consumers can obtain the needed facts without
re-running an incompatible private escape interpretation.

---

# Cause paths

Escape facts that lead to diagnostics must be able to produce a meaningful cause
path.

Example:

```text
local `buffer`
    ↓ borrowed here
view `slice`
    ↓ stored in `request.body`
`request`
    ↓ passed to `Queue`
parameter may be retained
    ↓
dependency escapes local storage lifetime
```

Cause paths may be reconstructed lazily.

They need not be stored permanently for every successful analysis.

---

# Diagnostic architecture

Escape analysis primarily produces facts.

User diagnostics are derived by combining escape facts with the normative facts
from lifetime, ownership, storage, reference, FFI, concurrency, and other
relevant analyses/rules.

Conceptually:

```text
EscapeResult
    +
Lifetime facts
    +
Ownership facts
    +
Storage/reference contracts
        ↓
Validation
        ↓
Diagnostic
```

This separation keeps the escape result reusable for non-diagnostic consumers.

---

# Diagnostic categories

The compiler must provide stable diagnostics for invalid escape classes.

Categories should cover at least behavior equivalent to:

```text
escape.reference-outlives-origin
escape.view-outlives-backing
escape.retained-borrow
escape.static-store-of-short-lived-reference
escape.thread-local-store-of-short-lived-reference
escape.foreign-retention
escape.task-transfer
escape.thread-transfer
escape.closure-capture
escape.unknown-retention
escape.analysis-precision-exhausted
```

Exact stable diagnostic IDs follow the central diagnostics governance.

An existing ID must not be reused for a different semantic meaning.

Precision-exhaustion reporting is normally informational/debug-oriented rather
than a default user warning.

---

# Diagnostic quality

An escape diagnostic should identify, where known:

- the escaping value/capability;
- the semantic origin;
- the relevant Place projection;
- the escape sink;
- the retention/transfer mode;
- the source storage/lifetime constraint;
- the destination requirement;
- the cause path;
- a source-level remedy when one can be recommended without changing semantics
  implicitly.

A diagnostic should not merely say:

```text
reference escapes
```

when the compiler knows the dependency chain.

---

# Unknown-retention diagnostics

When safety cannot be established because retention behavior is unknown, the
compiler should explain the missing contract.

For example, conceptually:

```text
error: retention behavior of function value is unknown

this argument contains a reference to local `buffer`
the callee may retain the argument after this call
```

or:

```text
error: foreign retention contract is required

the foreign function receives a pointer derived from `data`
```

Unknown behavior should not be reported as a misleading concrete lifetime fact.

---

# Precision exhaustion

Reaching an internal precision cap is not normally a source warning by itself.

The analysis widens conservatively.

Compiler analysis dumps, mentor mode, or development diagnostics may expose that
precision was widened.

If widening causes safe compilation to fail because the compiler cannot prove a
required guarantee, the final diagnostic should explain that the required
safety relation could not be proven and should include the most specific known
cause.

---

# LSP consumers

Escape results should be suitable for LSP/IDE features such as:

```text
Returned borrow from parameter `buffer`
This parameter may be retained by the callee
This closure captures `state` and escapes through the return value
```

These user-facing features are not required to be fully implemented with the
first escape-analysis implementation.

The analysis contract must not prevent them.

---

# Parameter-usage analysis consumer

`parameter_usage_analysis.md` consumes escape facts.

For example, facts such as:

```text
ReadOnly
NoEscape
NoRetention
NoIdentityRequirement
```

may allow parameter-usage analysis to recommend a borrowed reference or view in
place of an unnecessarily large by-value parameter.

Escape analysis must not itself rewrite function signatures or issue those
higher-level ergonomic recommendations.

---

# Closure-analysis consumer

`closure_analysis.md` consumes:

- callable-environment dependencies;
- whether the closure escapes;
- where it escapes;
- whether it crosses task/thread/foreign boundaries;
- whether retention outlives capture dependencies.

Closure analysis may refine callable-target and capture precision.

Escape analysis remains the authority for dependency movement/retention once
those dependencies are identified.

---

# Stack-analysis consumer

Stack analysis may consume proven non-escape and known escape facts when
reasoning about valid storage strategies and frame lifetime.

Escape does not command heap allocation.

A value that cannot remain in `Automatic` storage must either already have an
explicitly valid storage strategy or the program is invalid.

---

# Call-graph consumer and dependency

Escape analysis may consume call-graph SCCs and resolved direct call edges.

Call graph may in turn consume escape/callable facts to refine:

- retained callbacks;
- closure reachability;
- task/thread reachability;
- indirect-call targets where known.

The analyses may cooperate through fixed points or staged refinement.

Neither may assume unsound facts from the other.

---

# Effect-analysis relationship

Escape facts and effects are related but not identical.

Escape analysis owns dependency movement and retention.

Effect analysis owns the canonical effect model.

The compiler must not duplicate the complete effect summary inside escape
summaries merely because certain operations such as spawn, foreign interaction,
or reclamation affect both analyses.

---

# ISR and concurrency consumers

Later analyses must be able to ask questions such as:

```text
Does this dependency leave the current execution context?
Does this callable environment depend on thread-local state?
Can this argument be retained asynchronously?
Does this handle cross task/thread boundaries?
```

Task/thread/foreign escape classes are therefore mandatory parts of the escape
analysis contract.

---

# Semantic IR boundary

Normative Sec escape analysis runs on Semantic IR and canonical Place provenance
before Sec MLIR lowering.

Later MLIR/LLVM alias, escape, liveness, or optimization analyses may exist.

They do not replace the normative Sec escape analysis used to enforce language
safety.

The compiler must not lower to machine pointers and then attempt to reconstruct
Sec escape semantics from low-level IR.

Escape proofs required by later lowering may be transferred as explicit facts,
analysis results, or discharged obligations according to the Sec MLIR
architecture.

---

# Governance

Normative behavior remains in this rulebook.

Mutable implementation state belongs in:

```text
implementation-status.yaml
```

The implementation ledger should track an integration entry for escape analysis,
for example an identifier equivalent to:

```text
sema.escape-analysis
```

The ledger may record:

```text
status
integrated date
summary
rules
code
tests
implemented
remaining
verification
```

This rulebook must not contain a quickly aging `Current implementation status`
section.

Stable implementation requirements and completion criteria remain normative
here.

---

# Required implementation capabilities

Sec 0.1 escape analysis is required to support, soundly and according to the
available language features:

```text
direct functions
methods and receivers
parameters
returns
locals
temporaries
canonical Place projections
struct fields
fixed-array elements
slices/views
union payloads
Result/Option payloads
aggregate-contained dependencies
moves
copies
rebinding
replacement
branches
loops
defer/cleanup control flow
recursive functions
mutual recursion
direct-call summaries
separate-compilation summaries
static storage
thread-local storage
retaining calls
foreign retention
task/thread transfers
closure environments
function-value calls conservatively
```

The detailed callable-flow precision of function values may be provided later by
`closure_analysis.md`, but escape behavior must remain conservative and sound
without it.

---

# Positive test requirements

The test suite must include valid examples equivalent to:

```text
return owned local value
return borrow from parameter
return borrow from receiver field
non-retaining call with local borrow
local aggregate containing local borrow that does not escape
disjoint field references
disjoint constant array elements
disjoint static slice ranges
returned slice backed by caller input
move owned value into returned aggregate
safe static reference
valid TLS-contained dependency where contracts permit
valid task ownership transfer
valid thread ownership transfer
valid foreign CallOnly pointer use
recursive summary convergence
mutually recursive summary convergence
```

---

# Negative test requirements

The test suite must include invalid examples equivalent to:

```text
return reference to Automatic local
return view backed by Automatic local
store local borrow into Static storage
store local borrow into longer-lived ThreadLocal storage
pass local borrow to retaining call
foreign code may retain pointer/reference into local storage
return aggregate containing local reference
return callable containing invalid local capture
task retains reference to ending local
thread transfer of invalid borrowed dependency
transitive escape through multiple calls
transitive escape through aggregate field
escape through union payload
escape present on only one branch
escape discovered only after loop fixed point
```

---

# Precision test requirements

The test suite must protect analysis precision as well as soundness.

Required classes include:

```text
field A escape does not mark disjoint field B
constant index 1 does not imply escape of constant index 2
disjoint static slice ranges remain distinct
dynamic index widens only index precision when root remains known
unknown projection preserves known root when possible
branch merge retains all possible origins
projection widening terminates
origin-count cap widens conservatively
aggregate-contained origin precision survives whole-value transfer
```

---

# Interprocedural test requirements

Required classes include:

```text
direct returned parameter
transitive returned parameter
receiver-derived return
field/index/slice-projection-derived return
callee retention propagates to caller
callee ownership transfer remains distinct from borrow retention
recursive SCC
mutually recursive SCC
declaration-order independence
generic specialization behavior
missing summary conservative fallback
stale/incompatible summary rejection
```

---

# Callable and closure test requirements

Even before full closure-analysis precision, the escape suite must include:

```text
non-capturing named function value
known function-value targets
unknown function-value target
capturing closure remains local
capturing closure returned
closure passed to non-retaining call
closure passed to retaining call
closure transferred to task
closure transferred to thread
```

Unknown callable-target cases may use conservative behavior.

---

# FFI test requirements

Required FFI classes include:

```text
foreign CallOnly
foreign explicit retention
foreign ownership transfer
foreign unknown retention
RawPtr escape in unsafe
safe reference/view rejected when foreign lifetime is unknown
callback retention contract
```

---

# Storage and generation test requirements

Escape integration tests must include cases equivalent to:

```text
arena-backed owned result where the language/storage contract permits transfer
borrow escaping across arena reset
handle escaping across generation change
view backed by borrowed storage
allocator-backed owned transfer
storage-domain reuse at the same numeric address
```

Escape analysis may not be the sole analysis responsible for the final
diagnostic in these tests.

The tests verify correct cooperation with storage, lifetime, and reference
analysis.

---

# Concurrency test requirements

Required classes include:

```text
move-owned value to task
invalid borrow to longer-lived task
move-owned value to thread
invalid borrowed dependency across thread boundary
invalid TLS dependency across thread boundary
protected/generational handle transfer where contract permits
foreign asynchronous retention
```

---

# Differential Place-provenance tests

Because escape analysis depends on canonical Place provenance, tests must ensure
that known Place precision is not unnecessarily discarded.

For example:

```text
Place:  Parameter(0).Field(left)
Escape: Parameter(0).Field(left)
```

should remain precise when no widening requirement exists.

New Place/provenance capabilities should normally receive corresponding escape
integration tests.

---

# Completion criteria

Escape analysis is complete for Sec 0.1 when all applicable conditions below are
met:

1. All required escape subjects can be represented.
2. All required escape sinks can be classified.
3. Escape modes and destinations remain distinct.
4. Canonical Place provenance is consumed rather than replaced.
5. Relevant field/index/slice/union/aggregate precision is preserved within a
   bounded model.
6. Precision exhaustion widens conservatively.
7. Branches and loops reach sound fixed points.
8. Direct interprocedural summaries are composable.
9. Recursive and mutually recursive SCC summaries converge soundly.
10. Summary results are independent of declaration order.
11. Separate compilation can consume validated/versioned summaries or explicit
    conservative contracts.
12. Unknown behavior is explicit and conservative.
13. Ownership transfer remains distinct from borrow retention.
14. Aggregates, containers, views, handles, and callable environments carry
    dependencies transitively.
15. Task/thread transfer is represented explicitly.
16. Foreign retention is represented explicitly and unknown foreign behavior is
    conservative.
17. Lifetime/storage/reference validation can consume the produced facts.
18. Parameter-usage, closure, stack, call-graph, effect, concurrency/ISR, LSP,
    and optimization consumers can consume the required facts without inventing
    a competing escape model.
19. Diagnostics can identify origin, sink, mode, and meaningful cause path.
20. Positive, negative, precision, interprocedural, callable, FFI,
    storage/generation, and concurrency test classes pass.
21. No invalid escape is repaired through implicit heap promotion, hidden copy,
    hidden backing replacement, or another source-invisible semantic change.

---

# Summary of mandatory rules

```text
Escape analysis determines whether and how a value, reference, view, address,
callable environment, handle, or storage dependency becomes usable outside the
context for which its current semantic dependencies were established.

Escape is a semantic dependency-flow property, not a physical storage-placement
decision.

Value escape, reference escape, address escape, backing-storage dependency, and
physical storage escape are distinct concepts.

Returning or transferring an owned value does not imply that source Automatic
storage itself escapes.

Escape analysis classifies escape form and destination rather than reducing the
result to a boolean.

A function boundary does not by itself imply retention or long-lived escape.

Owned value escape may be valid and ordinary.

Escape analysis identifies dependency movement. Lifetime analysis determines
whether source dependencies remain valid for the destination's required
lifetime.

Escape analysis must not repair invalid escape through hidden allocation,
implicit heap promotion, hidden deep copy, or another source-invisible ownership
or storage change.

Escape analysis consumes canonical Place provenance and preserves relevant
field, index, slice, union-payload, and aggregate-contained origin precision.

Control-flow joins may produce multiple possible origins.

Bounded precision is permitted, but exhausted precision must widen
conservatively and must never produce unsound non-escape.

Aggregates, arrays, containers, views, handles, and callable environments carry
dependencies structurally.

Current carrier provenance may change through mutation, move, rebinding,
replacement, destruction, or lifetime end. Historical escape facts remain
monotone.

Move follows ownership transfer. Copy follows Sec copy semantics. Escape
analysis does not independently decide move/copy legality.

Object destruction, object lifetime end, storage reclamation, and storage epoch
transition remain distinct events.

Call transfer instantiates symbolic callee summaries with caller provenance.

Function summaries describe parameter, receiver, return, retention, capture,
concurrency, foreign, and unknown escape behavior.

NoEscape is a positive proof and must not be inferred from absence of a known
escape.

Unknown escape and retention are explicit first-class results.

Recursive interprocedural analysis is solved to a sound conservative fixed point
and must be independent of declaration order.

Unknown call targets use explicit contracts where available and otherwise fall
back conservatively.

Function-value calls combine known target summaries and remain conservative when
target sets are unknown.

Closure capture creates dependencies from callable environments to captured
entities; closure analysis may refine capture and target precision.

Task/thread transfers remain explicit escape classes and do not collapse
ownership, lifetime, and synchronization into one property.

FFI retention is governed by explicit foreign contracts and unknown foreign
retention is conservative.

Separate compilation uses versioned, validated summaries or explicit contracts.
Missing or incompatible metadata falls back conservatively.

Lifetime, ownership, storage, reference, and escape analyses cooperate through
shared facts and canonical Place provenance rather than duplicating one
another's semantic models.

Normative Sec escape analysis runs on Semantic IR before Sec MLIR lowering.
Later backend escape/alias analyses do not replace it.

Mutable implementation status is governed by implementation-status.yaml rather
than this normative rulebook.
```
