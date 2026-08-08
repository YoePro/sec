# Parameter Usage Analysis

## Status

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the semantic purpose, demand model, transfer rules,
interprocedural propagation, FFI contract boundaries, narrowing-candidate model,
recommendation policy, tooling integration, summary requirements, diagnostics,
and completion criteria for Sec parameter usage analysis.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.

---

# Purpose

Parameter usage analysis determines the minimum semantic capabilities that a
function implementation requires from each parameter and receiver.

The analysis answers questions such as:

```text
Does the callee require ownership?
Does it only require a borrow?
Does it mutate elements or collection structure?
Does it require an exact fixed-array extent?
Does it only require sequence access?
Does it require a stable address?
Does it retain the argument or a dependency on it?
Does a foreign call impose stronger storage or representation requirements?
```

The result may reveal that the declared source-level parameter contract is
stronger than the implementation requires.

Examples include:

- a large by-value struct whose implementation only performs call-local reads;
- a fixed array whose implementation only requires a read-only sequence view;
- an owning collection whose implementation only requires element access;
- a mutable parameter whose implementation never performs mutation;
- an FFI wrapper that requires an owning fixed array even though the foreign
  operation only requires a call-local pointer and length;
- a chain of functions that all forward an unnecessarily strong parameter
  contract.

Parameter usage analysis is primarily an API-design and efficiency analysis.
It does not redefine the validity of a stronger source-level parameter contract.

---

# Normative role

This rulebook is normative for:

- the concept of `ParameterDemand`;
- the distinction between declared capability and required capability;
- the demand dimensions that the compiler must be able to represent;
- the transfer rules by which parameter uses raise demand;
- interprocedural propagation of parameter demand;
- conservative control-flow joins and recursive fixed points;
- the treatment of function-value calls and callable contracts;
- FFI as an explicit contract boundary;
- dimension-specific unknown demand;
- candidate parameter narrowing;
- separation between semantic demand and recommendation policy;
- Interactive, Standard, and Deep analysis behavior as it applies to parameter
  usage analysis;
- separate-compilation summary requirements;
- diagnostic/advisory requirements;
- required test classes;
- completion criteria for Sec 0.1 parameter usage analysis.

This rulebook does not redefine:

- source-level parameter syntax;
- reference syntax;
- array or slice syntax;
- ownership;
- copy or move legality;
- borrow legality;
- object lifetime;
- storage lifetime;
- escape semantics;
- callable semantics;
- FFI declaration syntax;
- physical ABI argument passing;
- physical stack placement;
- physical closure representation;
- source-level overload resolution.

Those semantics remain owned by their respective normative rulebooks.

---

# Core rule

Parameter usage analysis determines the weakest semantic parameter contract that
is sufficient for every reachable use in the analyzed function.

The analysis does not change the source declaration.

For example:

```sec
fn Sum(values: int[10000]) int {
    let mut total := 0

    for i in 0..<values.len {
        total += values[i]
    }

    return total
}
```

The declared parameter requires a fixed array with exact extent `10000`.

The implementation may only require:

```text
read access
call-local lifetime
random-access sequence semantics
no ownership
no structural mutation
no exact extent
```

That difference creates a candidate API narrowing.

It does not authorize the compiler to silently change the function type.

---

# Source semantics and ABI are distinct

Source-level parameter semantics are not the same as physical argument passing.

A source declaration such as:

```sec
fn Inspect(value: LargeStruct) void {
    Use(value.Field)
}
```

remains a by-value source parameter even if ABI lowering passes the physical
value indirectly through a hidden address.

Conversely, parameter usage analysis may conclude that a shared reference would
be a sufficient and potentially better source-level API even when the current
ABI already avoids a physical copy.

Therefore:

```text
Source parameter semantics
    !=
Physical ABI argument passing
```

Parameter usage analysis owns the first question.

ABI lowering owns the second.

---

# Declared capability and required capability

Each analyzed parameter has two conceptually separate capability sets:

```text
DeclaredCapability
RequiredCapability
```

`DeclaredCapability` is derived from the source-level parameter type and its
normative contracts.

`RequiredCapability` is derived from the reachable implementation and applicable
callee/foreign contracts.

A narrowing opportunity exists when:

```text
DeclaredCapability
    is strictly stronger than
RequiredCapability
```

The existence of such a relation does not prove that the programmer intended a
weaker public API.

The implementation demand and API intent remain distinct concepts.

---

# ParameterDemand

Parameter demand is multidimensional.

The primary result must not be reduced to a single classification such as:

```text
ShouldBeRef = true
```

Conceptually, the analysis must be able to represent:

```text
ParameterDemand {
    Access
    Mutation
    Ownership
    Lifetime
    Identity
    Shape
    Storage
    Representation
    Precision
}
```

The exact compiler data structure is implementation-defined.

The semantic dimensions are normative.

---

# Access demand

Canonical access demand includes at least:

```text
Unused
Read
Write
Unknown
```

## Unused

The implementation has no reachable semantic use of the parameter.

Unused-parameter diagnostics may be owned elsewhere, but the fact remains a
valid parameter-demand result.

## Read

The implementation observes the parameter or one of its components without
requiring mutation.

Examples include:

```sec
let name := customer.Name
let value := items[index]
if buffer.len == 0 { ... }
```

Read demand does not imply ownership.

## Write

The implementation requires mutation of the parameter or one of its components.

Write demand does not by itself imply ownership.

A mutable borrow may be sufficient.

---

# Mutation demand

Collection and aggregate mutation must not be collapsed into one boolean.

Canonical mutation facts include at least:

```text
NoMutation
ElementOrFieldMutation
StructuralMutation
UnknownMutation
```

## Element or field mutation

Examples:

```sec
customer.Name = newName
values[index] = 0
```

Such uses may be satisfiable through a mutable reference or mutable sequence
view without transferring ownership.

## Structural mutation

Structural mutation changes collection structure or backing relation.

Examples conceptually include:

```text
Append
Grow
Resize
Remove
RebindBackingStorage
```

A mutable element view is not automatically sufficient for structural mutation.

The required candidate contract must expose the structural capability actually
used.

---

# Ownership demand

Canonical ownership demand includes at least:

```text
BorrowSufficient
OwnershipRequired
ConsumptionRequired
UnknownOwnership
```

## BorrowSufficient

The function does not require ownership of the value.

The required borrow may be shared or mutable depending on mutation demand.

## OwnershipRequired

The function requires an owned value to satisfy reachable semantics, for
example because the value must become owned state somewhere else.

## ConsumptionRequired

The function must be able to consume or move the parameter or a relevant owned
subobject.

Example:

```sec
fn Enqueue(job: Job) void {
    queue.Add(move job)
}
```

A borrowed parameter is not semantically equivalent.

---

# Current copy is not proof of required copy

A parameter may currently be copied because its declaration is by-value.

That does not prove that an independent copy is required by the implementation.

Example:

```sec
fn Inspect(value: LargeCopyableStruct) int {
    return value.Count
}
```

The implementation may require only a shared borrow even if ordinary source
parameter passing currently creates value semantics.

The analysis must distinguish:

```text
what the current declaration causes
```

from:

```text
what the implementation requires
```

A semantic copy requirement exists only when a distinct copied value is itself
required by reachable semantics.

---

# Lifetime and retention demand

Parameter usage analysis consumes escape and lifetime facts.

It does not implement a second escape analysis.

Canonical lifetime/retention demand includes at least:

```text
CallOnly
Returned
Retained
CrossTask
CrossThread
ForeignRetention
UnknownLifetime
```

## CallOnly

No dependency on the parameter remains usable outside the dynamic invocation
except through ordinary value results that no longer depend on the parameter.

Call-only parameters are strong candidates for borrowed APIs when other demand
dimensions permit it.

## Returned

A returned result depends on the parameter.

Examples include returned references or views into caller-provided storage.

A borrowed parameter may still be appropriate, but its return-lifetime relation
must remain valid.

## Retained

The function or a callee may retain a dependency after the call returns.

## CrossTask and CrossThread

A dependency crosses into another execution context.

Ownership, lifetime, storage, and synchronization requirements remain separate
and are consumed from their respective analyses.

## ForeignRetention

Foreign code may retain the value, address, pointer, callback, view, or related
dependency according to an explicit FFI contract.

---

# Identity demand

Parameter usage analysis distinguishes value observation from address/identity
requirements.

Canonical identity demand includes at least:

```text
ValueOnly
AddressRequired
StableIdentityRequired
UnknownIdentity
```

## ValueOnly

The function only needs semantic value access.

## AddressRequired

The implementation forms or forwards a safe reference, raw pointer, address, or
other address-bearing capability.

Address requirement alone does not imply long-lived stable identity.

## StableIdentityRequired

The same object/storage identity must remain valid for a longer semantic
interval, for example because a reference is retained, a foreign API requires a
stable address, or another contract explicitly depends on identity.

Stable identity is derived from reference, escape, storage, FFI, and callable
contracts rather than from syntax alone.

---

# Shape demand

Collection and fixed-array parameters require independent shape facts.

The analysis must be able to represent at least:

```text
WholeValue
Sequence
ContiguousSequence
RandomAccessSequence
ExactExtent
MinimumExtent
KnownRange
UnknownShape
```

These facts may be combined.

They are not required to form one exclusive enum.

---

# Sequence demand

Iteration may require only sequence semantics.

Example:

```sec
for value in values {
    Process(value)
}
```

This does not automatically require random access or exact extent.

---

# Random-access demand

Indexing requires random access when the relevant collection/view abstraction
uses random access semantics.

Example:

```sec
let value := values[index]
```

This does not automatically require ownership or exact fixed-array extent.

---

# Contiguous-sequence demand

Some operations require contiguous storage.

Examples may include:

- foreign pointer-plus-length calls;
- compiler-known byte operations;
- target-specific operations requiring contiguous memory.

Contiguity is a stronger shape/storage capability than generic sequence access.

---

# Exact extent and length observation are distinct

Observing `len` does not by itself mean that the declared exact fixed-array
extent is required.

Example:

```sec
if values.len == 16 {
    ...
}
```

This observes length.

A genuine exact-extent requirement exists when the semantic contract or
reachable implementation requires exactly that extent.

The analysis must not infer:

```text
Declared T[N]
    =>
ExactExtent(N) required
```

without a semantic reason.

---

# Fixed-index access establishes minimum extent

A fixed index may establish a minimum extent without requiring the full declared
extent.

Example:

```sec
return values[7]
```

conceptually implies:

```text
MinimumExtent >= 8
```

not:

```text
ExactExtent = declared N
```

The compiler may use this precision in Deep analysis even if no source-level
minimum-extent contract exists in Sec 0.1.

---

# Partial-range demand

The analysis may retain known range usage.

Example:

```sec
fn Header(packet: byte[4096]) Header {
    return ParseHeader(packet[0..<64])
}
```

may yield:

```text
KnownRange = [0, 64)
```

This fact may support API-design advisories or call-site slicing analysis.

It does not automatically prove that `byte[64]` is the intended public API.

---

# Storage demand

Parameter demand may include storage requirements obtained from canonical
storage facts and callee contracts.

The analysis must be able to preserve at least:

```text
NoSpecialStorage
Contiguous
StableAddress
Aligned
Pinned
SpecificMemorySpace
UnknownStorage
```

These may be combined.

A generic borrowed view must not be recommended when a required storage
property would be lost.

---

# Representation demand

Some operations require a particular physical representation or explicit
layout contract.

Examples include:

```text
FFI struct layout
explicit packing
explicit endian representation
hardware-defined representation
compiler-known representation contract
```

Representation demand is distinct from ordinary value, ownership, and shape
requirements.

The analysis consumes `ResolvedLayout` and relevant FFI/hardware contracts.

It must not infer representation requirements merely from type size.

---

# Operation-to-demand transfer

Every reachable parameter use contributes the minimum semantic requirements of
that use.

The total `ParameterDemand` is the conservative join of all reachable uses.

The analysis must derive demand from semantic operations rather than from
superficial syntax patterns alone.

---

# Field reads

Example:

```sec
let name := customer.Name
```

normally contributes:

```text
Access = Read
```

It does not automatically contribute:

```text
OwnershipRequired
AddressRequired
ExactExtent
```

---

# Field mutation

Example:

```sec
customer.Name = newName
```

contributes:

```text
Access = Write
Mutation = ElementOrFieldMutation
```

An ordinary aggregate may therefore be compatible with a mutable-reference
candidate if all other demands permit it.

---

# Collection element read

Example:

```sec
let x := values[i]
```

normally contributes:

```text
Access = Read
Shape = RandomAccessSequence
```

It does not by itself require exact extent, ownership, or structural mutation.

---

# Collection element mutation

Example:

```sec
values[i] = 0
```

normally contributes:

```text
Access = Write
Mutation = ElementOrFieldMutation
Shape = RandomAccessSequence
```

It does not by itself require structural mutation or ownership.

---

# Structural collection operations

A structural collection operation raises structural mutation demand.

A candidate borrowed view is valid only if the candidate abstraction exposes the
required structural behavior.

The analysis must not replace an owning/growable parameter with a fixed view
when the callee performs operations such as grow, resize, append, or backing
rebind.

---

# Taking a safe reference

Example:

```sec
let r := ref value
```

contributes at least:

```text
Identity = AddressRequired
```

and the applicable lifetime/borrow dependency.

The lifetime requirement depends on how the reference is subsequently used.

---

# Raw pointer/address formation

Creating or forwarding a raw pointer similarly establishes address demand and
applicable unsafe/storage/FFI requirements.

The analysis must not later recommend a candidate parameter type from which the
required pointer/address semantics cannot be formed safely and legally.

---

# Returning a parameter by value

Example:

```sec
return value
```

may require ownership, copy, or move capability according to ordinary Sec value
semantics.

A read-only function body does not automatically imply that a borrow is
sufficient when the parameter value itself is returned or transferred.

---

# Returning a borrow/view

Example:

```sec
return ref value.Field
```

may yield:

```text
Lifetime = Returned
Identity = AddressRequired
```

A borrowed source parameter may remain sufficient if lifetime rules prove the
returned dependency valid.

Escape and lifetime analysis own that proof.

---

# Retaining/storing a parameter

Storing a value into longer-lived state may raise ownership demand.

Storing a borrow, view, address, callback, or other dependency may raise
retention demand.

The specific result depends on the destination and applicable ownership/escape
contracts.

The parameter usage analysis consumes those facts rather than inferring
retention from container syntax alone.

---

# Move use

A reachable move/consume use contributes:

```text
Ownership = ConsumptionRequired
```

for the relevant Place or subobject.

Example:

```sec
Consume(move value)
```

A borrow-only candidate cannot satisfy this demand.

Partial moves should retain canonical Place precision where Sec ownership rules
permit partial use.

---

# Copy use

A source operation that currently causes a copy does not automatically establish
`CopyRequired`.

The analysis must determine whether semantic independence of the copy is itself
required.

If the copied local is only used for read-only observation that could have been
performed through a borrow, the implementation may still have
`BorrowSufficient` demand.

---

# Whole-value operations

Some operations observe or transfer the complete semantic value.

Examples may include:

- value equality;
- hashing;
- serialization;
- copy/move construction;
- destruction-sensitive transfer;
- compiler-known whole-value operations.

`WholeValue` observation remains distinct from ownership.

A function may require the whole value but still only require borrowed read
access.

---

# Calls propagate demand

Parameter usage analysis is interprocedural.

For a direct call:

```sec
Inner(value)
```

caller demand includes the applicable callee demand for the corresponding
parameter.

Conceptually:

```text
Demand(value) =
    Join(
        local uses,
        Instantiate(Demand(Inner.parameter0), value)
    )
```

This propagation must preserve relevant Place/projection information where the
callee summary is projection-sensitive.

---

# Forwarding

A parameter that is only forwarded still inherits callee requirements.

Example:

```sec
fn Outer(values: int[1000]) int {
    return Inner(values)
}
```

If `Inner` requires only a call-local read-only sequence, `Outer` may have the
same minimum demand.

If `Inner` consumes ownership, `Outer` must retain the stronger demand.

---

# Control-flow joins

Demands from all semantically reachable paths are conservatively joined.

Example:

```sec
if store {
    Save(move value)
} else {
    Inspect(value)
}
```

requires consumption capability because one reachable path consumes the value.

The total demand is the minimum combined contract sufficient for every reachable
use, which means joining toward the strongest capability required by any path.

---

# Unreachable paths

Proven unreachable semantic paths do not contribute demand.

This includes paths eliminated by canonical compile-time reasoning before the
parameter demand result is finalized.

Ordinary optimizer speculation must not change source validity or semantic
demand.

---

# Loops

A use inside a loop contributes the same semantic capability requirement as a
use outside the loop.

Loop frequency is not part of semantic demand.

Frequency may later affect recommendation ranking.

Loop-carried parameter and forwarding facts must participate in the analysis
fixed point where required.

---

# Recursive functions

Recursive and mutually recursive parameter-demand summaries are solved to a
sound fixed point.

The compiler should reuse canonical call-graph SCC information or an equivalent
declaration-order-independent mechanism.

Results must not depend on source declaration order.

---

# Function-value calls

For:

```sec
operation(value)
```

parameter usage analysis consumes callable-flow facts and callable contracts
from closure/callable analysis.

If concrete targets are known, the corresponding parameter demands are joined.

If an open callable contract is available, the analysis consumes the contract.

If a critical demand dimension is absent from the contract, that dimension
becomes conservative `Unknown`.

The analysis must not invent a non-retaining or borrow-only contract for an
unresolved callable target.

---

# Dimension-specific unknown

Unknown information is tracked per demand dimension.

A result such as:

```text
Access = Read
Shape = ContiguousSequence
Ownership = UnknownOwnership
Lifetime = UnknownLifetime
Identity = AddressRequired
```

is valid.

The compiler must not collapse all independently known facts to a single total
`Unknown` merely because one dimension is unresolved.

This rule is especially important across FFI, callable, and separate-compilation
boundaries.

---

# FFI is an explicit contract boundary

Parameter usage analysis may propagate semantic demand across an FFI boundary
only from:

- explicit foreign contracts;
- compiler-known foreign semantics;
- resolved ABI/layout requirements where those requirements are normative for
  the foreign operation.

The analysis must not infer ownership, retention, mutability, or lifetime from
C-like pointer syntax alone.

---

# Foreign call-only access

If a foreign contract explicitly guarantees:

```text
ReadOnly
CallOnly
NoRetention
```

then a foreign call may contribute:

```text
Access = Read
Ownership = BorrowSufficient
Lifetime = CallOnly
```

plus any representation/storage requirements.

---

# Foreign storage requirements

A foreign operation may independently require:

```text
Contiguous
Aligned
StableAddress
Pinned
SpecificMemorySpace
ExactRepresentation
```

Therefore a parameter can be semantically borrow-sufficient while still
requiring strong storage constraints.

Example conceptual demand:

```text
Access = Read
Ownership = BorrowSufficient
Lifetime = CallOnly
Identity = AddressRequired
Storage = Contiguous + Aligned + Pinned
```

A generic sequence abstraction would be insufficient if it cannot guarantee
those storage properties.

---

# Foreign retention

If a foreign contract allows retention after the call, the parameter demand must
reflect it.

Example:

```text
MayRetainUntilExplicitRelease
```

may require:

```text
Lifetime = ForeignRetention
Identity = StableIdentityRequired
```

plus any ownership or storage requirements defined by the FFI contract.

---

# Missing foreign retention information

When retention behavior is not declared and cannot be obtained from a
compiler-known foreign contract:

```text
Lifetime = UnknownLifetime
```

for affected reference/address/view-like dependencies.

The analysis must not treat missing information as `CallOnly`.

This may block a narrowing recommendation even if other dimensions are known.

---

# Foreign ownership transfer

Foreign ownership transfer must be explicit.

When a foreign API consumes ownership, the parameter demand includes the
corresponding `OwnershipRequired` or `ConsumptionRequired` fact according to the
normative FFI ownership model.

Foreign pointers must not be assumed borrowed merely because they are passed as
addresses.

---

# FFI pointer-plus-length wrappers

A foreign pointer-plus-length API is an important narrowing case.

Example:

```sec
fn Write(data: byte[4096]) Result[...] {
    return raw_write(data.Ptr, data.len)
}
```

If the foreign contract proves:

```text
read-only
contiguous
call-only
no retention
```

then parameter usage analysis may derive:

```text
Access = Read
Ownership = BorrowSufficient
Lifetime = CallOnly
Shape = ContiguousSequence
ExactExtent = NotRequired
```

This is a strong candidate for a borrowed contiguous sequence/view API.

The analysis must still preserve any required alignment, pointer, memory-space,
or representation constraints.

---

# FFI callbacks/context

Foreign callback registration may create transitive parameter demand.

Example conceptually:

```text
ForeignRegister(callback, context)
```

may retain both values according to its foreign contract.

Parameter usage analysis consumes:

- closure/callable environment dependencies;
- escape retention summaries;
- FFI callback/context contracts.

It does not reimplement closure or escape analysis.

---

# Compiler-known operations

Compiler-known/core operations expose demand summaries or contracts through the
same analysis interface used for ordinary callees.

A compiler-known operation may require:

- whole-value read;
- address formation;
- contiguous storage;
- exact layout;
- ownership transfer;
- another explicitly defined capability.

Parameter usage analysis must not maintain ad-hoc hard-coded guesses for such
operations when canonical compiler-known contracts are available.

---

# Demand joins

Demand joins are conservative.

Conceptually:

```text
Read + Write
    => Write-capable access

BorrowSufficient + ConsumptionRequired
    => ConsumptionRequired

Sequence + RandomAccessSequence
    => RandomAccessSequence

CallOnly + Retained
    => Retained
```

The lattice may retain multiple independent dimensions rather than forcing all
facts into a single total ordering.

The combined result must be sufficient for every reachable operation.

---

# Monotonicity and termination

Interprocedural demand propagation is monotone.

As additional reachable requirements are discovered, the required capability
may become stronger or more conservative.

A possible required capability must never disappear because another path is
weaker.

Recursive summaries must terminate through finite demand domains, bounded
projection precision, conservative widening, or an equivalent sound method.

---

# Candidate narrowing

Candidate narrowing is derived only after `RequiredDemand` has been established.

The compiler conceptually tests:

```text
Capabilities(candidate)
    >=
RequiredDemand(parameter)
```

A candidate is valid only if every required capability remains available.

The analysis never rewrites the declaration automatically.

---

# Canonical narrowing classes

The analysis should be able to express at least the following conceptual
candidate classes where the Sec type system provides a suitable form:

```text
ByValueToSharedReference
ByValueToMutableReference
FixedArrayToSharedView
FixedArrayToMutableView
OwningSequenceToSharedView
OwningSequenceToMutableView
BroaderSequenceContract
ReducedExtentContract
```

The exact source spelling is owned by the relevant reference/array/slice/type
rulebooks.

A candidate may exist as an analysis fact even when Sec 0.1 has no exact source
type representing the theoretical weakest contract.

---

# Fixed array to view

A fixed-array parameter is a strong narrowing candidate when the implementation
requires sequence access but not exact extent, ownership, or structural
mutation.

Example:

```sec
fn Sum(values: int[4096]) int {
    let mut total := 0

    for i in 0..<values.len {
        total += values[i]
    }

    return total
}
```

may produce:

```text
Access = Read
Mutation = NoMutation
Ownership = BorrowSufficient
Lifetime = CallOnly
Identity = ValueOnly
Shape = RandomAccessSequence
ExactExtent = NotRequired
```

A shared sequence view is then a valid candidate if the source-language view
contract provides all required capabilities.

---

# Mutable fixed array to mutable view

If the implementation mutates elements but does not require ownership,
structural mutation, or exact extent, a mutable sequence view may be a valid
candidate.

The analysis must not recommend a shared view for a parameter whose reachable
uses require mutation.

---

# Owning sequence to borrowed view

An owning collection parameter may be stronger than required when the callee
only reads or mutates elements during the call and never requires structural
ownership.

A borrowed candidate is invalid when any path performs structural mutation,
ownership transfer, retention that requires ownership, or another unsupported
operation.

---

# Large value to reference

`BorrowSufficient` does not imply that a reference is always a useful API.

Example:

```sec
fn Double(value: int) int {
    return value * 2
}
```

A shared reference would normally be worse than a value parameter even though
ownership is unnecessary.

Recommendation policy must therefore distinguish semantic sufficiency from
cost and API quality.

---

# Semantic demand and recommendation policy are separate

The semantic analysis produces the same `RequiredDemand` independently of
whether the user wants advisories.

Recommendation policy may consume additional facts such as:

```text
SizeOf(T)
AlignOf(T)
copy/move cost
construction/destruction cost
call frequency
API visibility
caller conversion cost
CompilationPlan
project analysis configuration
```

These facts influence presentation and ranking.

They do not redefine the semantic demand.

---

# ResolvedLayout as cost input

`ResolvedLayout` may be used to estimate whether by-value transfer is large or
expensive.

Layout size alone must not change facts such as:

```text
BorrowSufficient
ConsumptionRequired
ExactExtent
Retained
```

unless the operation has an explicit layout/representation requirement.

A change in target layout may therefore invalidate recommendation/cost data
without invalidating semantic demand.

---

# Recommendation reasons

A narrowing opportunity may carry one or more reasons such as:

```text
AvoidCopyCost
ReduceOwnershipRequirement
IncreaseAcceptedInputs
RemoveUnusedExactExtent
RemoveUnusedMutability
RemoveUnusedStructuralCapability
ReduceRetentionRequirement
SimplifyFFIWrapperContract
```

Reasons are analysis/tooling facts rather than new source-language semantics.

---

# Explicit contracts and programmer intent

A parameter type may intentionally express a stronger public invariant than the
current body happens to exercise.

Examples include:

- cryptographic fixed blocks;
- protocol-defined fixed headers;
- constrained named types;
- hardware buffers;
- exact FFI representation;
- explicit ownership-signaling APIs;
- target-specific storage contracts.

Parameter usage analysis distinguishes:

```text
UnusedByCurrentImplementation
```

from:

```text
SemanticallyRedundant
```

The latter requires a stronger proof.

An explicit normative contract suppresses incompatible narrowing candidates.

---

# Recommendation confidence

Recommendation output may classify confidence conceptually as:

```text
Certain
Strong
Heuristic
```

`Certain` means the stronger capability is proven unnecessary under the
applicable contracts.

`Strong` means a narrower API is strongly supported by implementation demand,
but API intent remains a design choice.

`Heuristic` means cost or design evidence suggests an opportunity without a
fully compelling API conclusion.

Exact scoring/ranking is compiler policy.

---

# Unknown critical dimensions block narrowing

A narrowing candidate must not be recommended when a required capability is
unknown and the candidate relies on that capability being absent.

Example:

```text
candidate = shared borrow
Lifetime = UnknownLifetime
```

The compiler must not recommend the candidate as proven sufficient.

Deep analysis may instead report a blocked candidate:

```text
A shared borrow may be sufficient, but retention behavior of `X` is unknown.
```

---

# Candidate blockers

Analysis should be able to explain why a plausible narrower contract was
rejected.

Examples:

```text
shared view blocked because ExactExtent(64) is required
shared reference blocked because one path consumes ownership
borrowed buffer blocked because foreign code may retain the address
mutable view blocked because the callee may grow the collection
```

Blocker reporting is especially useful in Deep analysis.

---

# Caller conversion cost

Recommendation policy may consider caller-side conversion cost.

A candidate is not automatically preferable if every caller must perform an
expensive allocation, copy, representation change, or another semantically
significant conversion.

For array-to-view narrowing, conversion is often cheap and therefore likely to
be useful.

Other candidates may require stronger cost analysis.

---

# API visibility

Recommendation policy may rank public/exported API opportunities more strongly
than private helper opportunities.

The semantic `RequiredDemand` remains identical.

Visibility only influences presentation and ranking.

---

# Whole-program call frequency

When available, whole-program call frequency may strengthen an efficiency
advisory.

For example, a large by-value parameter used at many call sites may deserve
higher priority than the same parameter in an infrequently used helper.

Call count is optional analysis information.

It is not required for semantic demand or baseline correctness.

---

# Narrowing groups

Deep analysis may group interprocedural narrowing opportunities.

Example:

```text
A(data: LargeArray)
    -> B(data: LargeArray)
        -> C(data: LargeArray)
```

If all three functions require only call-local read-only sequence access, Deep
analysis may report one `NarrowingOpportunityGroup` rather than three unrelated
advisories.

This is an optional precision/usability capability.

---

# Analysis budgets

Parameter usage analysis follows the compiler-wide analysis-budget model:

```text
Interactive
Standard
Deep
```

These budgets affect cost, precision, and presentation.

They do not define different source-language semantics.

---

# Interactive analysis

Interactive analysis is primarily intended for LSP use.

It may use:

- cached summaries;
- incremental invalidation;
- bounded local/interprocedural propagation;
- conservative demand summaries;
- limited recommendation computation.

Interactive analysis must remain sound for any facts presented as proven.

Incomplete recomputation may be represented as tooling state such as `Pending`.

`Pending` must never be treated as a semantic proof.

---

# Standard analysis

Standard compilation performs all parameter-demand computation required by
other mandatory compiler analyses and by the active `CompilationPlan`.

Parameter design advisories may be suppressed or limited according to project
policy.

Standard compilation must not flood normal build output with low-value API
suggestions.

---

# Deep analysis

Deep analysis is the natural mode for exhaustive parameter-usage review.

It may perform:

- larger interprocedural propagation budgets;
- whole-program call-frequency analysis;
- caller conversion-cost analysis;
- cross-module narrowing groups;
- public API scoring;
- more precise range/shape demand;
- context-sensitive callable refinement;
- blocked-candidate reporting;
- extended cause paths and reasoning.

Deep analysis refines the same semantic demand model.

It does not define a different Sec language.

---

# `sec analyse`

Parameter usage analysis participates in the compiler-wide analysis command.

Conceptually:

```text
sec analyse
```

runs all available analyses by default.

The explicit equivalent is conceptually:

```text
sec analyse --all
```

The user may also request parameter usage analysis specifically.

The exact CLI spelling for selecting an individual analysis is owned by the
compiler/tooling command-line specification.

Deep parameter-usage output may include:

- declared capability;
- required demand;
- narrowing candidates;
- blocked candidates;
- relevant FFI boundaries;
- reasoning/cause paths;
- interprocedural narrowing groups;
- optional size/cost/call-frequency information.

---

# LSP presentation

LSP presentation distinguishes:

```text
NeedToKnow
OptionalInsight
```

Parameter usage information is normally `OptionalInsight` unless needed to
explain another diagnostic or required semantic condition.

Default hover should not dump the complete demand lattice merely because the
compiler knows it.

A concise hover may show facts such as:

```text
Read-only, call-only parameter
```

or:

```text
Fixed extent is not required by this function.
```

Detailed/Deep LSP mode may expose full demand/candidate information.

Call counts, estimated traffic, and similar enrichment are optional and
configurable.

---

# Project-configured LSP depth

Project configuration may control parameter-analysis depth and presentation.

A small project on a high-capacity machine may enable Deep parameter usage
analysis continuously.

A large project or resource-constrained machine may use cheaper Interactive
analysis.

The LSP integration must consume the project configuration through the common
LSP configuration mechanism.

The detailed requirements for reading, watching, reloading, invalidating, and
recomputing analysis state belong to the LSP rulebook and its synchronization
appendix.

---

# Function summaries

Each analyzable function exposes a stable parameter-usage summary.

Conceptually:

```text
ParameterUsageSummary {
    Parameters
    Receiver
    AnalysisPrecision
}
```

Each parameter may expose conceptually:

```text
ParameterSummary {
    DeclaredCapability
    RequiredDemand
    CandidateNarrowings
    BlockedNarrowings
}
```

The exact representation is implementation-defined.

`RequiredDemand` is the persistent semantic core.

Candidate narrowings and recommendation messages are derived information.

---

# Receiver demand

Methods analyze `self`/receiver demand explicitly.

Receiver use must not be hidden merely because it is not written as an ordinary
source parameter.

The same access, mutation, ownership, lifetime, identity, shape, storage, and
representation dimensions apply where meaningful.

---

# Separate compilation

Parameter-demand propagation must continue across compilation-unit boundaries.

When callee bodies are unavailable, the compiler consumes persisted summaries or
explicit normative contracts.

Imported summary information should preserve dimension-specific facts such as:

```text
Read
BorrowSufficient
CallOnly
ContiguousSequence
UnknownLifetime
```

rather than collapsing the entire parameter to `Unknown`.

---

# Inferred and declared demand contracts

Demand may originate from:

```text
InferredDemand
DeclaredDemandContract
```

Inferred demand comes from an analyzable Sec body.

Declared demand contracts are required where body analysis is unavailable or
where another normative boundary owns the behavior, such as FFI or opaque
compiler-known code.

Where both exist and the language permits verification, the compiler should
validate that the declared guarantee is compatible with the implementation.

---

# Persisted summary versioning

Persisted parameter usage summaries require an internal schema/version identity.

A summary must be compatible with at least:

```text
function identity
function signature
relevant generic specialization
relevant CompilationPlan identity
summary schema/version
```

An incompatible stale summary must not be used as current analysis evidence.

If the body is available, the summary may be recomputed.

Otherwise the compiler falls back to an explicit contract or conservative
unknown behavior.

---

# Summary invalidation

A cached semantic-demand summary is invalidated when relevant inputs change,
including:

- function body;
- callee demand summary;
- callable target/contract;
- FFI contract;
- ownership semantics;
- escape/lifetime summary;
- storage requirement;
- type semantics used by the demand;
- generic specialization;
- CompilationPlan-selected implementation;
- analysis schema/version.

Recommendation/cost caches may additionally depend on:

- `ResolvedLayout` size/alignment;
- call frequency;
- API visibility;
- project presentation policy;
- caller conversion-cost facts.

Semantic-demand and recommendation caches may therefore be invalidated
independently.

---

# CompilationPlan

Semantic parameter demand is target-independent when the reachable semantics are
target-independent.

A `CompilationPlan` becomes part of the analysis identity when it changes the
relevant implementation or normative requirements, for example through:

- target-selected code;
- target-specific FFI contracts;
- hardware storage requirements;
- target-known compiler operations;
- target-specific layout/representation contracts.

Cost/recommendation data may be target-specific even when semantic demand is
unchanged.

---

# Generic functions

Generic parameter-demand summaries may be shared when their requirements are
independent of concrete type semantics.

A specialization-specific summary is required when demand depends on, for
example:

- copy/move properties;
- concrete aggregate structure;
- storage requirements;
- callable contracts;
- FFI layout;
- target-selected implementation.

The compiler may reuse summaries when semantic equivalence is proven.

---

# Diagnostics and advisories

Parameter usage diagnostics are primarily advisory/informational.

A stronger source parameter contract remains valid unless another rulebook says
otherwise.

Canonical conceptual diagnostic categories include:

```text
parameter.unused-capability
parameter.unnecessary-by-value
parameter.unnecessary-ownership
parameter.unnecessary-mutable-access
parameter.unnecessary-fixed-extent
parameter.owning-sequence-where-view-suffices
parameter.fixed-array-where-view-suffices
parameter.narrowing-blocked
parameter.ffi-wrapper-overconstrained
```

Exact stable diagnostic IDs are assigned under the compiler-wide diagnostic
governance.

Old IDs must not be reused for a different meaning.

---

# Diagnostic explanations

A useful recommendation explains the proof.

Example:

```text
information: parameter `values` does not require its fixed-array contract

The function:
    reads elements
    performs indexed access
    does not mutate the collection
    does not retain or return a dependency on it
    does not require exactly 4096 elements

A shared sequence view is sufficient for the analyzed implementation.
```

The compiler should prefer evidence-based explanations over terse commands such
as:

```text
use slice instead
```

---

# Large-value advisory

For a large by-value value whose implementation only requires a borrow, an
advisory may explain both semantic and cost evidence.

Example conceptually:

```text
information: parameter `customer` does not require ownership

The function only reads `customer` during the call.
No reachable operation copies for semantic independence, moves, retains,
mutates, or depends on stable identity.

`Customer` is expensive to pass by value on the active CompilationPlan.
Consider a shared reference parameter if borrowed access matches the intended
API.
```

The cost threshold is compiler policy, not a language constant.

---

# FFI wrapper advisory

FFI-related diagnostics should expose the actual foreign requirement.

Example:

```text
information: wrapper parameter is stronger than the foreign operation requires

Foreign contract requires:
    read-only contiguous bytes
    stable address during the call
    no retention

`buffer` additionally requires callers to provide an owned fixed array of
exactly 4096 bytes.
```

This makes foreign requirements visible without requiring the programmer to
manually reconstruct them from ABI details.

---

# Blocked narrowing advisory

Deep analysis may explain a blocked candidate.

Example:

```text
candidate: shared sequence view

not proven because:
    `ForeignRegister` may retain the supplied address
```

or:

```text
candidate: shared reference

blocked because:
    `StoreCustomer` consumes ownership on one reachable path
```

Blocked candidates are optional analysis output and are not normal compiler
errors.

---

# Consumers

Parameter usage analysis produces facts for:

- LSP;
- `sec analyse`;
- API-review tooling;
- documentation tooling;
- recommendation ranking;
- compiler cost reporting;
- optimization planning where source semantics remain unchanged.

Backend lowering may consume semantic demand as additional information.

It must not use a narrowing recommendation to silently change the source-level
function contract.

---

# Inputs from other analyses

Parameter usage analysis consumes canonical facts from:

- Place/provenance analysis;
- ownership/copy/move analysis;
- lifetime analysis;
- escape analysis;
- closure/callable-flow analysis;
- call graph;
- storage analysis;
- layout resolution;
- effect analysis;
- FFI contracts;
- compiler-known operation contracts.

It must not create competing local models for those domains.

---

# Required Standard implementation capabilities

For Sec 0.1, Standard parameter usage analysis must support at least:

- ordinary parameters;
- method receivers;
- field reads and writes;
- fixed-array and slice/view indexing;
- iteration;
- collection element mutation;
- structural collection mutation;
- moves and ownership consumption;
- semantic copy requirements;
- returns;
- retention and escaping dependencies;
- direct call propagation;
- function-value calls through known targets or callable contracts;
- branches;
- loops;
- recursive and mutually recursive call SCCs;
- fixed arrays;
- slices/views;
- ordinary aggregates;
- address requirements;
- storage requirements;
- foreign contract boundaries;
- dimension-specific unknown demand;
- summary production and consumption.

---

# Optional Deep capabilities

Deep parameter usage analysis may additionally provide:

- whole-program call frequency;
- cross-module narrowing groups;
- caller conversion-cost analysis;
- hot-path weighting;
- public API scoring;
- context-sensitive callable refinement;
- more precise known-range/shape demand;
- extended blocker/cause-path output.

These improve precision and usability.

They are not required to define the semantics of `ParameterDemand`.

---

# Required positive/no-advisory tests

The test suite must include cases where narrowing must not be recommended,
including at least:

```text
small cheap value parameter
parameter ownership consumed
parameter retained
fixed extent semantically required
structural collection mutation
stable identity required
pinned storage required
specific memory-space requirement
foreign ownership transfer
foreign retention
observable independent copy requirement
explicit hardware/representation contract
```

These tests protect against over-aggressive recommendations.

---

# Required narrowing tests

The test suite must include at least:

```text
large read-only struct -> shared-reference candidate
mutable large struct -> mutable-reference candidate
fixed array read-only -> shared-view candidate
fixed array element mutation -> mutable-view candidate
owning sequence read-only -> shared-view candidate
owning sequence element mutation -> mutable-view candidate
unused exact extent
unused ownership
unused mutability
```

---

# Required shape tests

The test suite must include at least:

```text
iteration only -> Sequence
indexing -> RandomAccessSequence
fixed index -> MinimumExtent, not ExactExtent
length observation -> not automatically ExactExtent
explicit fixed-size contract -> preserve ExactExtent
known partial range -> retain range demand
contiguous foreign call -> ContiguousSequence
```

---

# Required interprocedural tests

The test suite must include at least:

```text
direct forwarding
transitive forwarding
callee consumes
callee retains
callee only reads
mixed branch demands
loop-carried demand
recursive SCC
mutual recursion
declaration-order independence
function-value call with known targets
function-value call with open contract
unknown critical dimension blocks narrowing
separate-compilation demand summary
stale summary rejection
```

---

# Required FFI tests

The FFI parameter-demand matrix must include at least:

```text
read-only call-only pointer
mutable call-only pointer
pointer + length
contiguous requirement
alignment requirement
stable-address requirement
pinned requirement
specific memory space
foreign retention
foreign ownership transfer
unknown retention
unknown ownership
callback/context retention
exact foreign layout requirement
```

It must also include wrapper opportunities where a foreign contract proves that:

```text
fixed array -> borrowed view
owning buffer -> borrowed contiguous view
```

is a valid candidate.

---

# Required recommendation-policy tests

The same semantic `ParameterDemand` must remain stable when:

```text
advisories are disabled
minimum confidence changes
copy-cost target changes
call-frequency information is unavailable
public/private recommendation policy changes
LSP uses Interactive mode
sec analyse uses Deep mode
```

These tests ensure presentation settings do not leak into semantic analysis.

---

# False-positive regression tests

The compiler must maintain regression cases where strong parameter types are
intentional, including representative cases such as:

```text
cryptographic fixed block
network protocol fixed header
hardware register/buffer contract
FFI ABI struct
constrained domain aggregate
ownership-signaling API
```

The analysis must preserve explicit semantic intent instead of blindly
minimizing every parameter type.

---

# Analysis-tier tests

Interactive tests must verify:

```text
sound bounded result
incremental invalidation compatibility
Pending tooling state does not become a semantic proof
```

Standard tests must verify:

```text
deterministic semantic demand
required interprocedural propagation
required FFI contract handling
```

Deep tests must verify:

```text
same source semantics
same or stronger precision
additional advisory information only
no weaker safety assumptions
```

---

# Determinism

For the same:

- source program;
- active `CompilationPlan`;
- compiler version;
- analysis configuration relevant to Standard semantic precision;

Standard parameter-demand results must be deterministic.

Worklist ordering, thread scheduling, cache hit order, or similar incidental
implementation details must not change the semantic result.

---

# Completion criteria for Sec 0.1

Parameter usage analysis is complete for Sec 0.1 when all of the following hold:

1. multidimensional parameter demand can be represented;
2. ordinary local operations produce the required demand facts;
3. copy, move, borrow, ownership, and consumption are distinguished;
4. element mutation and structural collection mutation are distinguished;
5. sequence use, random access, contiguity, exact extent, and minimum extent are
   distinguished where required;
6. escape/lifetime retention facts are consumed rather than duplicated;
7. address, storage, and representation requirements are preserved;
8. direct-call demand propagates interprocedurally;
9. function-value calls use known target summaries or callable contracts;
10. FFI behaves as an explicit contract boundary;
11. missing information remains dimension-specific and conservative;
12. branch, loop, recursion, and mutual-recursion fixed points are sound;
13. separate compilation can consume versioned parameter-demand summaries;
14. candidate narrowing preserves every required capability;
15. explicit source contracts and API invariants suppress incompatible advice;
16. semantic demand remains separate from cost/recommendation policy;
17. Interactive, Standard, and Deep consumers use the same semantic model;
18. required positive, narrowing, shape, interprocedural, FFI, policy, and
    false-positive regression tests pass.

---

# Governance

This rulebook defines normative analysis behavior and completion requirements.

Current implementation status belongs in:

```text
implementation-status.yaml
```

A suitable ledger integration ID is conceptually:

```text
sema.parameter-usage-analysis
```

The ledger should track granular implementation capabilities rather than a
single binary implemented/not-implemented state.

Examples include:

```text
local demand
control-flow demand
interprocedural direct-call propagation
recursive demand
array/view narrowing
large-value narrowing
FFI contract demand
callable-contract propagation
summary persistence
Deep whole-program analysis
LSP integration
```

The normative rulebook must not contain rapidly aging current-status claims.

---

# Final normative summary

Parameter usage analysis determines the minimum semantic capabilities required
from each parameter and receiver.

The analysis is multidimensional and independently tracks access, mutation,
ownership, lifetime/retention, identity, shape, storage, representation, and
precision.

The analysis distinguishes the parameter contract currently declared by the
programmer from the weaker or equal contract required by the implementation.

It never silently changes the function signature.

Source-level parameter semantics remain distinct from physical ABI argument
passing.

Demand propagates through direct calls, callable contracts, control flow, loops,
recursion, FFI boundaries, compiler-known operations, and separate-compilation
summaries.

FFI behavior is derived only from explicit or compiler-known foreign contracts.
Missing retention, ownership, or other critical foreign information remains
conservative unknown for the affected dimensions.

Candidate narrowing is permitted only when the candidate preserves every
required capability.

A narrowing opportunity is advisory. It does not prove that the weaker contract
better expresses the programmer's intended public API.

Semantic demand and recommendation policy remain separate. Layout, call
frequency, API visibility, caller conversion cost, project settings, and the
active `CompilationPlan` may affect recommendation ranking without redefining
semantic demand.

The LSP presents parameter-analysis information according to configurable
Interactive/Deep policy and distinguishes information the programmer needs from
optional insight.

`sec analyse` runs parameter usage analysis as part of its default all-analysis
behavior and may also run it individually.

Mutable implementation progress is governed by `implementation-status.yaml`.
