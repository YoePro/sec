# Stack Analysis

## Status

Normative compiler-analysis rulebook for Sec 0.1.

This rulebook defines the semantic and machine stack-analysis model, stack-bound
classification, per-function and per-root composition, recursion and SCC
handling, callable and foreign-call integration, frame construction, live-range
and stack-slot reuse requirements, stack-budget validation, tooling integration,
summary and incremental-analysis requirements, diagnostics, tests, and completion
criteria for Sec stack analysis.

Mutable implementation status does not belong in this rulebook. It is governed
by the repository-level `implementation-status.yaml` ledger.

The canonical ledger integration identifier is:

```text
sema.stack-analysis
```

---

# Purpose

Stack analysis determines how much stack resource an execution may require and
why that requirement arises.

The analysis answers questions such as:

```text
How much automatic storage is simultaneously live in this function?
What is the maximum stack contribution of this call path?
Which recursive SCC determines the maximum stack depth?
Is a finite upper bound proven, unknown, or unbounded?
Which callable or foreign boundary prevents a finite proof?
How much stack is required from each execution root?
Does the verified requirement satisfy the active stack budget?
```

Stack analysis is a resource analysis. It does not redefine storage placement,
object lifetime, borrowing, escape, ownership, layout, call reachability, task
semantics, ISR semantics, or backend ABI rules.

Those semantics are consumed as canonical input facts.

---

# Normative role

This rulebook is normative for:

- the distinction between semantic and machine stack requirements;
- stack-bound classification;
- per-function own-frame and transitive stack summaries;
- per-path stack composition;
- recursion and mutually recursive SCC handling;
- recursion-depth bounds;
- closed and open indirect-call target handling;
- callable, foreign, runtime, and platform stack contracts;
- execution roots and physical stack domains;
- frame construction from materialized automatic storage;
- live-range constrained stack-slot reuse;
- stack accounting for arrays, aggregates, descriptors, views, temporaries,
  return storage, cleanup, defer, and destruction;
- stack-budget validation;
- summary versioning and incremental invalidation;
- Interactive, Standard, and Deep stack-analysis behavior;
- LSP and `sec analyse` integration requirements;
- required test classes;
- completion criteria for Sec 0.1 stack analysis.

This rulebook does not redefine:

- `StorageOrigin`, `BackingRelation`, or `ReclamationAuthority`;
- object or storage lifetime;
- ownership, copy, move, or borrow legality;
- escape validity;
- type or aggregate layout;
- function-value target discovery;
- call graph construction;
- source-level recursion policy;
- task runtime implementation;
- ISR restrictions;
- FFI declaration syntax or foreign metadata import;
- ABI calling convention rules;
- register allocation;
- backend optimization legality;
- project configuration syntax.

Those concerns remain owned by their respective normative rulebooks and target
or tooling specifications.

---

# Core principle

Stack analysis determines stack resource requirements without changing storage
semantics.

In particular:

```text
Stack pressure
    does not authorize
implicit heap promotion
implicit arena promotion
implicit allocator-backed promotion
```

If an automatic object is too large for an active stack budget, the compiler
must not silently repair the program by changing its storage origin.

Any alternative storage strategy remains an explicit programmer or platform
choice governed by the storage and allocation rules.

---

# Stack analysis levels

Sec distinguishes two stack-analysis levels.

## Semantic stack requirement

The semantic stack requirement describes materialized stack storage required by
Sec semantics and canonical semantic lowering.

It may include:

```text
automatic local storage
materialized semantic temporaries
semantically required return storage
cleanup or destruction state required by canonical lowering
alignment and padding introduced at the semantic frame level
```

It does not attempt to predict backend register spills or other final
code-generation accidents.

## Machine stack requirement

The machine stack requirement describes the final target-specific stack cost of
the compiled code.

It may additionally include:

```text
ABI argument or return areas
saved registers
register spills
outgoing call area
target frame alignment
prologue or epilogue state
backend/runtime bookkeeping
```

The relationship is conceptually:

```text
Semantic stack requirement
    + target/ABI/backend frame effects
    -> Machine stack requirement
```

The two results are distinct compiler facts and must not overwrite one another.

For example:

```text
Semantic upper bound: 8192 B
Machine exact maximum: 7344 B
```

is a valid pair of results.

---

# Stack-bound classification

A stack requirement must carry the quality of the result rather than only a
byte count.

Conceptually:

```text
StackBound:
    Exact(bytes)
    UpperBound(bytes)
    Unknown
    Unbounded
```

## Exact

`Exact(bytes)` means the analysis level knows the exact required amount for the
reported scope and CompilationPlan.

## UpperBound

`UpperBound(bytes)` means the compiler has proven that the requirement cannot
exceed the reported value, although the real requirement may be smaller.

A verified upper bound is sufficient for a stack safety or resource proof.

## Unknown

`Unknown` means that a finite requirement may exist, but the compiler cannot
prove a finite upper bound from the currently available semantic facts,
contracts, summaries, or analysis precision.

`Unknown` is not zero and is not evidence that the program will overflow the
stack.

## Unbounded

`Unbounded` means the compiler has proven that the reachable execution structure
admits arbitrarily large stack depth or stack demand under the relevant model.

`Unbounded` is stronger information than `Unknown` and must remain distinct.

---

# Partial information

Loss of an overall upper bound must not erase independently known information.

For example:

```text
known stack contribution before unknown call: 1536 B
overall stack maximum: Unknown
cause: callable target contract has no verified stack bound
```

Deep analysis may additionally expose known minimum contributions.

A known minimum contribution must never be used as an upper bound for a safety
or budget proof.

---

# CompilationPlan dependence

Stack results that depend on size, alignment, ABI, runtime behavior, target
calling convention, or backend frame construction are resolved per active
`CompilationPlan`.

The same Sec function may therefore have different stack requirements on
platforms such as:

```text
linux/amd64
bare-metal Cortex-M
riscv32
```

without changing its source-language meaning.

Machine stack summaries are necessarily CompilationPlan-specific.

Semantic summaries may remain target-independent or symbolic only where their
inputs are target-independent.

---

# ResolvedLayout is authoritative

Stack analysis must consume canonical resolved layout facts.

It must not implement a parallel struct, array, union, pointer, descriptor, or
alignment model.

Relevant inputs include:

```text
Size
Alignment
Stride where applicable
resolved descriptor representation
resolved aggregate representation
```

When layout is not yet resolved, target-dependent byte results may remain
symbolic or unknown according to the analysis stage.

---

# Per-function stack summaries

A function stack summary must distinguish at least:

```text
OwnFrame
TransitiveMaximum
BoundKind
MaximumCause
```

The exact internal representation is implementation-defined.

## Own frame

`OwnFrame` is the maximum stack storage attributable to the function's own
materialized frame at the relevant analysis level.

## Transitive maximum

`TransitiveMaximum` includes the function's own frame together with the maximum
simultaneously live callee contribution on any reachable execution path.

## Maximum cause

The summary should preserve enough information to explain the call path, SCC,
unknown boundary, or frame contributor responsible for the result.

---

# Call-path composition

Sequential calls do not automatically sum their complete transitive maxima.

For example:

```text
A own frame = 100 B
B maximum   = 500 B
C maximum   = 800 B
```

for:

```sec
fn A() void {
    B()
    C()
}
```

produces conceptually:

```text
100 + max(500, 800) = 900 B
```

when the two calls cannot be simultaneously active.

Nested calls do sum simultaneously live frames.

For:

```text
A -> B -> C
```

the active call chain may require:

```text
Own(A) + Own(B) + Own(C)
```

plus applicable machine-level overhead at the machine analysis level.

---

# Control-flow composition

Stack analysis operates on reachable execution paths.

For mutually exclusive branches such as:

```sec
if condition {
    LargePathA()
} else {
    LargePathB()
}
```

the call contribution is the maximum of the two reachable paths, not their sum.

The same principle applies to:

```text
if
match
switch
select
error paths
cleanup paths
panic paths where applicable
```

Proven unreachable paths do not contribute to a stack bound.

---

# Loops and stack depth

Ordinary loop iteration does not multiply frame depth.

For example:

```sec
for item in values {
    Process(item)
}
```

does not imply:

```text
iteration count * Stack(Process)
```

because each iteration normally reuses the same active call structure.

Loop analysis may still affect frame live ranges and cleanup overlap.

Recursion is different because each recursive call may create another
simultaneously live frame.

---

# Execution roots

Whole-program stack analysis is measured from execution roots rather than only
from the program's ordinary main entry.

Conceptual execution roots include:

```text
ProgramEntry
ThreadEntry
TaskExecutionContext
ISR
PlatformEntry
UnknownEntry
```

The exact root model is determined by canonical call graph, runtime, platform,
and CompilationPlan semantics.

Reachability and maximum stack requirement are root-specific.

A function reachable from the ordinary program entry but not from an ISR root
does not contribute to that ISR root's stack requirement.

---

# Physical stack domains

Logical execution roots and physical stacks are separate concepts.

Two threads with separate stacks are analyzed as separate stack domains.

Their per-stack maxima do not become one artificial combined depth.

Whole-program resource reporting may separately report aggregate reserved stack
memory across several threads or tasks.

If several logical roots may execute on the same physical stack, the active
runtime and CompilationPlan must define possible nesting or reentry, and stack
analysis must account for those relations.

Task stack behavior follows the actual Sec runtime/platform model. Stack
analysis must not assume that every task has a dedicated native stack.

Possible implementations may include dedicated stacks, shared execution stacks,
stackless state machines, or platform-defined execution contexts.

This rulebook does not choose among them.

---

# Interrupt nesting

Stack analysis must be capable of composing nested execution contributions when
the platform model permits them.

Conceptually:

```text
thread frame chain
+ interrupt frame
+ nested interrupt frame
```

may share one physical stack domain.

The ISR rulebook determines which ISR call paths and nesting relationships are
valid. Stack analysis consumes those relationships and computes their resource
contribution.

---

# Call graph ownership

Stack analysis consumes the canonical call graph defined by `call_graph.md`.

It must not create a parallel call-reachability model.

Relevant call-graph facts include:

```text
known targets
closed or open target sets
callable contracts
execution roots
SCCs
reentry relationships where represented
```

---

# Acyclic call graphs

For an acyclic reachable call graph, stack maxima can be composed bottom-up.

Conceptually:

```text
Maximum(F) = OwnFrame(F) + maximum reachable callee contribution at each call path
```

A leaf function has no callee contribution, but it may still have a non-zero
own frame.

A machine-level leaf function may have an exact zero frame when no stack storage,
ABI area, saved state, spill, or alignment requirement exists.

Zero own-frame size does not imply zero transitive stack usage.

---

# Recursion and SCCs

Recursion is identified from call-graph strongly connected components.

A multi-node SCC may represent mutual recursion.

A singleton SCC is recursive only when it contains a self-edge.

Recursion can arise through:

```text
direct calls
mutual direct calls
function-value calls
callbacks
foreign reentry
runtime hooks
```

Therefore stack analysis must operate on the resolved semantic call graph rather
than source syntax alone.

---

# Recursion-depth classification

Recursion depth is a separate fact from byte stack requirement.

Conceptually:

```text
RecursionDepthBound:
    ExactDepth(n)
    UpperBoundDepth(n)
    UnknownDepth
    UnboundedDepth
```

A recursive call does not automatically imply `UnboundedDepth`.

---

# Finite recursion and finite global bounds

A recursive function may terminate for every concrete invocation without having
a finite compile-time maximum over all permitted root inputs.

For example:

```sec
fn Countdown(value: uint) void {
    if value == 0 {
        return
    }

    Countdown(value - 1)
}
```

has finite depth for each concrete `value`.

However, a global finite upper bound requires a finite proven upper bound on the
initial recursive measure.

Therefore:

```text
decreasing measure
    + finite proven initial upper bound
    -> possible finite recursion-depth proof
```

A decreasing argument alone does not prove a finite whole-program maximum.

---

# Sources of finite recursion bounds

Stack analysis may consume canonical facts such as:

```text
constant call arguments
range-constrained parameters
constrained types
verified collection extents
verified finite state transitions
root contracts
other canonical range or control-flow facts
```

to establish a finite recursion-depth bound.

Stack analysis should reuse these facts rather than build an unrelated range or
termination analysis.

The required Sec 0.1 baseline need not implement a general-purpose termination
prover.

Simple bounded recursive measures are sufficient for the baseline.

Deep analysis may later add more advanced techniques such as size-change
reasoning, multi-variable ranking functions, or path-sensitive recursive measure
tracking.

---

# Unknown recursion depth

A recursive structure whose concrete depth may be finite but whose maximum
cannot be proven is `UnknownDepth`.

For example, recursively walking a linked structure with no statically bounded
maximum chain length is normally unknown rather than proven unbounded.

The distinction is normative.

---

# Proven unbounded recursion

`UnboundedDepth` requires a proof that arbitrarily large recursive depth is
reachable under the relevant execution model.

Simple examples include reachable unconditional recursive cycles with no finite
limiting state transition.

For example:

```sec
fn Again() void {
    Again()
}
```

may yield a proven unbounded stack requirement when the call is reachable and
no guaranteed constant-stack recursion rule applies.

Mutual recursion may be proven unbounded by the same principle.

---

# Recursive SCC stack contribution

A recursive SCC must account for every simultaneously live frame along a
recursive cycle.

For example:

```text
A -> B -> C -> A
```

with own frames:

```text
A = 100 B
B = 200 B
C = 300 B
```

has a cycle contribution involving all three frames, not merely the largest
single frame.

Different cycles within one SCC may have different stack contributions.

A sound bound must account for the maximum relevant recursive cycle or use an
alternative sound abstraction that proves an equal or larger upper bound.

---

# Recursive progress must cover relevant transitions

A recursion-depth proof must apply to all recursive transitions relevant to the
claimed bound.

If one cycle is known to decrease a bounded measure but another possible cycle
in the same SCC has no corresponding progress proof, the bounded measure from
the first cycle must not be unsoundly applied to the second.

The result is `UnknownDepth` or `UnboundedDepth` according to what can actually
be proven.

---

# Tail recursion

Source-level tail position does not by itself prove constant stack usage.

Optional tail-call elimination is a backend optimization and must not be used as
a source-level resource proof.

If Sec or a specific CompilationPlan later guarantees constant-stack tail calls
for a class of calls, stack analysis may consume that guarantee.

Final machine stack summaries may naturally reflect tail calls that were
actually and validly applied by the backend.

---

# Inlining

Inlining may alter final machine frame requirements by changing call overhead,
materialization, slot reuse, register pressure, and spills.

Semantic stack proofs must not depend on optional inlining.

Final machine stack analysis reflects the actual compiled frame structure.

---

# Closed indirect-call target sets

For a reachable indirect call whose canonical callable analysis provides a
closed target set:

```text
KnownTargets = {A, B, C}
IsClosed = true
```

the stack contribution of the call is the maximum requirement among all
possible targets, accounting for the caller's own frame and path context.

Closed target sets therefore permit finite stack composition when all targets
have finite compatible summaries.

---

# Open callable contracts

Known targets do not close an otherwise open target set.

For example:

```text
KnownTargets = {A, B}
OpenContract = X
```

requires the open contract to contribute a sound bound for every additional
target permitted by that contract.

An open callable target set can provide a finite stack bound only when its
callable contract supplies a verified stack bound sufficient for all permitted
targets.

The exact source or metadata syntax of such a contract is outside this
rulebook.

---

# Open calls without stack contracts

A reachable open indirect call whose contract provides no usable stack bound
contributes `Unknown` to the transitive maximum.

Known target contributions should still be preserved for explanation where
useful.

The compiler must not assume that an unknown callback consumes zero stack.

---

# Reentry through callable and foreign boundaries

Possible reentry participates in recursion analysis.

For example:

```text
A
 -> ForeignCall
 -> Callback
 -> A
```

is stack-recursive/reentrant when the foreign or callable contracts permit this
path.

A verified `NoReentry`-equivalent contract may exclude such a recursive edge.

Unknown reentry behavior may force the affected stack result to `Unknown`.

This rulebook defines consumption of such facts, not their source syntax.

---

# Foreign, runtime, and platform calls

Foreign or runtime code executing on the same physical stack may contribute to
the current stack requirement.

A verified foreign/runtime/platform stack contract may provide a finite
CompilationPlan-specific contribution.

If no such contract or body-level summary is available, the reachable foreign
contribution is `Unknown` rather than zero.

Compiler-known platform functions may provide canonical stack summaries when
that knowledge is explicit and trustworthy.

FFI tooling and metadata import are outside this rulebook.

---

# Separate compilation

Separate compilation must be able to preserve compatible stack summaries so
callers can compose stack requirements without the callee body.

Conceptually, imported information may include:

```text
semantic own-frame information
semantic transitive bound
machine own-frame information for a CompilationPlan
machine transitive bound for a CompilationPlan
bound kind
unknown/unbounded cause metadata
```

Machine summaries must not be reused for an incompatible CompilationPlan, ABI,
summary schema, or compiler semantic model.

Missing compatible summary information produces localized `Unknown` where the
information is required.

---

# Generic and symbolic summaries

Stack analysis need not duplicate a complete analysis for every generic
instantiation when a sound symbolic semantic summary can be preserved.

Conceptual examples include:

```text
OwnFrame(T) <= SizeOf(T) + K
Maximum(F<N>) <= N * K + C
```

Such representations are internal analysis facts, not source syntax.

When types, constants, specialization choices, and CompilationPlan are resolved,
the symbolic facts may be instantiated.

Machine summaries may still differ per specialization when lowering produces
different final machine behavior.

---

# Frame construction

A function's semantic own frame is based on maximum simultaneously live
materialized automatic storage rather than the sum of all declarations that
appear anywhere in the function.

Conceptually:

```text
SemanticOwnFrame =
    maximum simultaneously live materialized automatic storage
    + semantic frame metadata
    + semantic alignment/padding
```

at the semantic analysis level.

---

# Lexical scope and live range

Source lexical scope is not itself the authoritative stack-storage live range.

A storage object may cease to require materialized stack storage before the end
of a broad lexical block when all canonical lifetime, destruction, cleanup,
address, and escape dependencies are finished.

Conversely, `defer`, destruction, borrowing, retained addresses, or cleanup paths
may keep storage live beyond its last ordinary source read.

Stack analysis consumes canonical lifetime and storage facts to establish these
ranges.

---

# Stack-slot reuse

Two materialized stack objects may share physical stack space only when the
compiler proves that their required live storage cannot overlap.

Conceptually:

```text
CanReuse(A, B)
    only if
NoSimultaneousLiveStorage(A, B)
    and
size/alignment constraints are satisfied
```

When overlap is unknown and an upper-bound proof is required, the conservative
result is no reuse.

---

# Mutually exclusive branches

Storage required exclusively by mutually exclusive control-flow branches may
share stack space when the compiler proves the branch live ranges cannot
overlap.

For example:

```sec
if condition {
    let first: LargeA
    UseA(first)
} else {
    let second: LargeB
    UseB(second)
}
```

may require approximately the larger branch-local storage rather than the sum of
both branch-local objects, subject to all surrounding live storage, alignment,
and cleanup constraints.

---

# Sequential local reuse

Sequential locals within one branch may reuse storage when the earlier object's
materialized lifetime has definitely ended before the later object's storage is
needed.

Reuse must respect:

```text
borrows
references
raw or safe address validity
escape facts
destruction
cleanup
defer
partial move state
pinning
stable-address requirements
```

No stack-specific shortcut may invalidate these semantics.

---

# Alignment and padding

Required alignment and padding are real stack costs.

Stack analysis consumes resolved object alignment and applies frame-level
alignment requirements at the level where they are introduced.

Semantic layout padding belongs to semantic frame accounting when required by
canonical semantic frame layout.

ABI/backend padding belongs to machine frame accounting.

---

# Parameters

Source parameters do not automatically consume local semantic stack storage.

Their physical placement may be in registers, incoming stack areas, or indirect
ABI locations.

If Sec semantics require a parameter value to have addressable storage, semantic
analysis may record that requirement.

The backend may then materialize an actual machine stack slot when required by
the target ABI and lowering.

Source-level by-value semantics remain by-value even when the ABI passes the
physical representation indirectly.

ABI indirection does not turn a source value parameter into a borrow.

---

# Address-taken values

Taking the address of a local or parameter does not by itself require heap or
arena storage.

A stack object remains valid when lifetime and escape analysis prove that every
reference or pointer dependency is valid for the stack object's storage
lifetime.

Addressability may constrain register-only lowering and stack-slot reuse.

---

# Return storage

Return-value semantics and physical return storage are distinct.

Depending on semantic lowering and ABI, a result may use:

```text
register return
caller-provided return storage
hidden return pointer
callee temporary
direct construction into final destination
```

Stack accounting must count the storage where it is actually materialized and
must not double-count one caller-owned result slot as both caller and callee
storage when no second buffer exists.

---

# Temporaries

Only temporaries that require materialized stack storage contribute to stack
usage.

Pure SSA values or values kept entirely in registers do not consume semantic
stack bytes merely because a conceptual temporary exists.

Materialized aggregate temporaries may contribute substantially even when there
is no named local variable.

Semantically guaranteed direct construction or copy elision may remove a
semantic temporary.

Optional backend optimization may improve only the machine-level result unless
its behavior is part of canonical semantic lowering.

---

# Destruction and cleanup

The last ordinary read of a value does not necessarily end its stack-storage
requirement.

When destruction requires the materialized object representation, storage must
remain available until the relevant destruction action is complete.

Stack analysis therefore distinguishes:

```text
last ordinary use
object lifetime end
destruction
storage reuse/reclamation point
```

according to the canonical lifetime, destruction, and storage rules.

---

# Defer

A deferred action may extend the stack-storage dependency of values it uses.

For example:

```sec
fn Work() void {
    let resource := Open()

    defer {
        Close(resource)
    }

    HeavyWork()
}
```

may require `resource` to remain materially available through cleanup.

Stack analysis consumes canonical defer-use and capture/dependency facts rather
than implementing an independent defer-capture model.

---

# Error and panic paths

Reachable error, cleanup, destruction, and panic paths contribute to maximum
stack demand when their semantics require additional active frames or
simultaneously live storage.

A cleanup path may therefore determine the maximum even when the ordinary
success path is cheaper.

Panic strategy may be CompilationPlan-dependent where runtime behavior differs.

Stack analysis must not assume that panic cleanup is free.

---

# Fixed arrays

A fixed array embedded in automatic storage contributes through its resolved
inline size and alignment.

For example:

```sec
let buffer: byte[4096]
```

is not a dynamic descriptor and must not be silently modeled as separate
allocator-backed storage.

---

# Embedded aggregates

Embedded fields contribute through the resolved size of the containing object.

For example, if an automatic struct contains a fixed array, stack analysis
counts the containing struct's resolved size rather than double-counting the
field as a separate independent stack object.

Tagged unions contribute according to their resolved union representation, not
the sum of all variants.

---

# Dynamic owning collections and views

For a dynamic owning collection whose backing storage is separate from its
stack-resident value representation, stack analysis counts the materialized
owning descriptor or handle but not the separately allocated backing storage.

Likewise, a borrowed view contributes its stack-resident reference/descriptor
representation while its backing storage is accounted for at the storage origin
where that backing actually resides.

Therefore:

```text
runtime collection length
    !=
runtime stack frame size
```

when the backing storage is separate.

---

# Arena and allocator-backed storage

Arena-backed and allocator-backed payload bytes are not stack bytes merely
because a handle, descriptor, or reference to them is stored in an automatic
local.

Stack analysis counts only the stack-resident representation.

Arena and allocator resource accounting remains owned by the corresponding
storage/allocation analysis and runtime model.

---

# Runtime-sized automatic storage

Sec 0.1 stack analysis must not invent runtime-sized automatic stack allocation
where no such source/storage feature exists.

If a future Sec feature explicitly permits runtime-sized automatic storage, its
storage semantics must represent that choice explicitly.

A finite stack proof for such storage requires a finite proven upper bound on
its runtime size expression.

---

# Partial moves

A partial move does not automatically make the containing stack storage reusable.

Remaining live fields, destruction responsibility, cleanup state, and ownership
metadata may still require the containing representation or parts of it to
remain live.

Stack analysis consumes the canonical ownership and destruction state rather
than inventing a stack-specific partial-move model.

---

# Pinning and stable addresses

Pinning temporarily constrains storage movement and reuse.

Pinning does not change storage origin or automatically extend storage to the
end of the function.

When the pin and all other dependencies end, reuse may again become possible.

Likewise, a stable-address requirement constrains slot reuse while the address
may still be semantically observed.

Address stability, reference lifetime, and generation/invalidation facts are
canonical inputs to stack reuse decisions where applicable.

---

# Generational-pointer representation

Any stack bytes required by the eventual canonical generational-pointer
representation are determined by resolved layout and lowering.

Stack analysis counts that representation where it is materialized but does not
define the representation itself.

---

# Compiler bookkeeping

Compiler bookkeeping may contribute to a frame.

Conceptual examples include:

```text
cleanup flags
destruction state
panic/unwind state
generation or validation tokens
```

Bookkeeping mandated by canonical semantic lowering belongs to semantic frame
analysis.

Bookkeeping introduced only by a particular backend, ABI, or runtime belongs to
machine frame analysis.

---

# Semantic and machine frame authority

Semantic stack analysis provides the authoritative semantic frame proof.

After backend lowering and frame layout are known, machine stack analysis
provides the authoritative machine-level requirement.

For example:

```text
SemanticUpperBound = 4608 B
MachineExactFrame  = 4384 B
```

is not a contradiction.

The values answer different questions.

---

# Stack budgets

Required stack and available stack are separate concepts.

Stack analysis first determines a requirement.

An independent target, project, thread, task, ISR, platform, or build contract
may provide an available stack budget.

Validation then compares the appropriate verified requirement with that budget.

Conceptually:

```text
StackBudget {
    Domain
    MeasurementLevel
    MaximumBytes
}
```

where the measurement level distinguishes at least semantic and machine stack
budgets.

This representation is conceptual and does not define configuration syntax.

---

# Budget validation

A finite verified bound satisfies a finite stack budget only when the compiler
can prove:

```text
RequiredStack <= AvailableStack
```

at the budget's specified measurement level and physical stack domain.

If the budget requires a finite proof and the stack result is `Unknown`, the
build fails because the required proof could not be established.

This is distinct from proving that runtime overflow necessarily occurs.

If the result is `Unbounded` and the budget is finite, the budget cannot be
satisfied.

---

# Machine-level revalidation

A hard machine-stack budget must be revalidated after final backend frame
construction.

A semantic estimate below the machine budget is not sufficient when later ABI,
spill, or frame-layout effects increase the final machine requirement above the
budget.

Conversely, if the applicable contract explicitly concerns final machine stack,
a conservative semantic upper bound above the budget does not by itself reject
the build when the final verified machine requirement satisfies the budget.

The budget must identify which measurement level is authoritative.

---

# No-budget behavior

When no active contract requires a finite stack proof, `Unknown` may remain a
valid analysis result rather than automatically invalidating the program.

It remains useful for:

```text
sec analyse
LSP optional insight
resource reports
ISR analysis
platform tooling
```

A target or project may impose stronger requirements.

Stack analysis reports facts; resource policy determines whether those facts
are acceptable.

---

# Large-frame advisories

A large automatic frame may be worth reporting even when no hard budget is
violated.

Such a diagnostic is advisory or optional insight, not a language validity rule.

No normative byte threshold is defined here.

Recommendation policy may consider the active CompilationPlan and configured
resource guidance.

The compiler should explain the major automatic-storage contributor rather than
prescribe hidden heap or arena promotion.

A suitable advisory concept is:

```text
`buffer` contributes 1 MiB of automatic storage to this frame.
Consider whether this storage should use an explicitly selected non-stack
storage strategy.
```

The storage strategy remains explicit.

---

# Stack cause paths

Stack analysis should preserve a cause path sufficient to explain a maximum,
unknown result, unbounded result, or budget failure.

For example:

```text
main
  -> Parse
  -> Decode
  -> Inflate
  -> BuildTable
```

with representative frame contributions.

Recursive reports should identify the responsible SCC, relevant depth proof,
and recursive contribution.

Unknown reports should identify the boundary that prevented a finite proof.

Representative paths must be selected deterministically when several equivalent
maximum paths exist.

---

# Reusable stack proofs

A verified per-root stack proof should be reusable by downstream compiler
consumers.

Conceptually:

```text
StackProof {
    Root
    StackDomain
    Bound
    BoundKind
    MeasurementLevel
    CompilationPlan
    Dependencies
}
```

The exact internal structure is implementation-defined.

Consumers must preserve the meaning of `Exact`, `UpperBound`, `Unknown`, and
`Unbounded`.

`Unknown` must never be treated as zero.

---

# Consumers

Canonical stack facts may be consumed by:

```text
build validation
ISR analysis
LSP
sec analyse
resource reports
embedded/platform tooling
optimization and cost diagnostics
```

The ISR analysis must reuse stack-analysis facts rather than implement a second
stack calculator.

---

# Summary model

Stable function summaries should expose semantic stack information independently
from final machine stack information.

Conceptually:

```text
FunctionStackSummary {
    SemanticOwnFrame
    SemanticMaximum
    BoundKind
    RecursiveState
    Dependencies
    MaximumCause
}
```

and, when backend information is available:

```text
MachineStackSummary {
    MachineOwnFrame
    MachineMaximum
    BoundKind
    CompilationPlan
    Dependencies
    MaximumCause
}
```

These names are explanatory rather than mandatory implementation identifiers.

---

# Summary versioning

Persisted stack summaries must be versioned and validated before reuse.

Compatibility may depend on:

```text
function identity
function signature
summary schema version
compiler semantic-analysis version
CompilationPlan
ABI
generic specialization where relevant
```

An incompatible or stale summary must not be used as a verified stack proof.

---

# Summary invalidation

Stack summaries must be invalidated when relevant inputs change.

Relevant dependencies include:

```text
function body
callee stack summary
call graph target set
recursion/SCC structure
ResolvedLayout
lifetime/storage facts affecting live ranges or reuse
defer/destruction behavior
generic specialization
foreign/runtime/platform stack contract
CompilationPlan
ABI/backend frame information
analysis schema or semantic version
```

Semantic and machine cache dependencies may differ and may therefore be
invalidated independently where safe.

---

# Incremental propagation

Incremental analysis should propagate changed stack summaries through dependent
callers and execution roots.

If a changed function recomputes to an equivalent stack summary, further
propagation may stop.

An unrelated function edit should not force complete project-wide stack
reanalysis when dependency information proves that cached results remain valid.

Changes to callable target sets or reentry contracts may invalidate stack
results even when the source text of the call site did not change.

---

# Analysis state

Interactive tooling must distinguish incomplete analysis from a proven stack
result.

Conceptually, tooling state may distinguish:

```text
Pending
Partial
Complete
```

or an equivalent model.

A pending recursive, indirect, or foreign contribution must not be rendered as
zero or as a verified finite maximum.

---

# Analysis budgets

Stack analysis follows the common Sec analysis-budget model.

## Interactive

Interactive analysis prioritizes low-latency, incremental results such as:

```text
local frame facts
cached direct-call summaries
cheap closed-target composition
available recursion summaries
```

Interactive analysis may report `Pending` while stronger proof is still being
computed.

It must remain sound.

## Standard

Standard analysis performs every stack proof required for source validity or the
active `CompilationPlan` and resource contracts.

If a build requires a finite root stack bound, Standard analysis must perform
the necessary SCC, indirect-target, imported-summary, and budget proof.

Required proof must not be deferred to Deep.

Standard results are deterministic.

## Deep

Deep analysis may add greater precision and richer explanation, including:

```text
all execution-root reports
alternative maximum paths
frame-contributor breakdowns
stack-slot reuse explanations
more expensive recursion reasoning
symbolic or context-sensitive refinement
cross-module resource maps
suppressed or blocked precision explanations
```

Deep analysis improves precision and information without changing source
semantics.

If a specific build contract requires a refinement normally associated with
Deep, that refinement becomes required for that build rather than remaining
optional.

---

# LSP presentation

LSP stack presentation distinguishes information the programmer needs from
optional resource insight.

## NeedToKnow

Examples include:

```text
required finite stack proof failed
hard stack budget exceeded
proven unbounded stack requirement violates active policy
```

## OptionalInsight

Examples include:

```text
local frame size
transitive maximum
largest contributor
maximum call path
recursion depth source
stack-slot reuse explanation
```

Optional stack insight is configurable.

A normal hover should remain compact.

A possible presentation is:

```text
Stack: <= 624 B local, <= 4.8 KiB transitive
```

when such insight is enabled and proven.

Detailed or Deep views may expose the full cause path and contributor breakdown.

---

# LSP configuration changes

Persistent LSP analysis must respond to relevant project configuration changes
without requiring an LSP restart.

When stack-analysis depth, optional insight, stack budgets, or related analysis
settings change, the LSP must:

```text
reload the affected configuration
invalidate affected stack summaries or presentation caches
recompute required stack analysis
refresh dependent diagnostics, hovers, code lenses, or analysis views
```

Unchanged semantic summaries may be reused when only presentation policy changes.

---

# sec analyse

`sec analyse` includes stack analysis in the default all-analysis behavior.

The tooling must also permit stack analysis to be selected explicitly.

This rulebook does not define the final command-line spelling.

Deep stack analysis should be able to report:

```text
per-function semantic frames
per-function machine frames when available
per-root maximum stack
physical stack domains
recursive SCCs and depth bounds
unknown and unbounded causes
indirect/callable stack contracts
foreign/runtime boundaries
largest frame contributors
maximum call paths
stack-budget comparisons
semantic versus machine results
```

---

# Diagnostics

Stack diagnostics must distinguish different proof outcomes.

## Proven budget excess

When a verified requirement exceeds the applicable budget, the diagnostic should
state both values and the measurement level.

Conceptually:

```text
error: machine stack budget exceeded for `Worker`

required: 9216 B
available: 8192 B
```

## Failure to prove a required bound

When the active contract requires a finite proof but the result is `Unknown`, the
diagnostic must state that proof failed rather than claim that overflow is
certain.

Conceptually:

```text
error: finite stack bound could not be proven for `Worker`

required by target:
    stack <= 8192 B

cause:
    callback target has no verified stack bound
```

## Proven unbounded stack

A proven unbounded recursive or reentrant structure should be reported as
unbounded rather than merely unknown.

Conceptually:

```text
error: stack requirement is unbounded

recursive cycle:
    Parse -> Visit -> Parse

no finite recursion-depth bound exists for this reachable cycle
```

---

# Diagnostic ownership and severity

Stack analysis owns stack-resource facts and stack-budget proof diagnostics.

A stack finding must not duplicate an existing lifetime, escape, ownership,
layout, or call-graph error when that upstream error already makes the program
invalid.

Optional large-frame or resource-design findings remain advisories unless an
active resource contract promotes them through the normal diagnostic policy.

No separate stack-specific strict mode is introduced by this rulebook.

---

# Determinism

Stack analysis must be deterministic with respect to semantic inputs.

Results must not depend on:

```text
source declaration order
hash-map iteration order
worklist order
parallel scheduling
cache population order
```

When several cause paths produce the same bound, the summary bound remains the
same and any representative diagnostic path is chosen deterministically.

---

# Fixed-point and widening requirements

Recursive and symbolic stack analysis must terminate.

When precision must be reduced, widening must remain sound.

Conceptually, analysis may lose precision in the direction:

```text
Exact -> UpperBound -> Unknown
```

It must never replace unresolved or unknown stack demand with a guessed finite
bound.

`Unbounded` is used only when unboundedness is proven.

---

# Required Sec 0.1 baseline

The Sec 0.1 stack-analysis implementation must support at least:

```text
automatic local frame accounting
fixed arrays and embedded aggregates
descriptors and separate backing storage
alignment and padding
materialized temporaries
return storage
destruction and defer live-range extension
error and cleanup paths
sound stack-slot reuse
direct acyclic call composition
direct recursion detection
mutual recursion via SCCs
simple finite recursion bounds from canonical range/contracts
Unknown versus Unbounded recursion
closed indirect target sets
open callable contracts
reentry uncertainty
foreign/runtime stack summaries
execution roots and physical stack domains
semantic versus machine stack distinction
hard stack-budget validation
localized unknown causes
incremental summaries
Interactive, Standard, and Deep integration
```

Advanced general termination proving, sophisticated symbolic optimization, and
prediction of backend register allocation are not required for Sec 0.1.

---

# Required frame tests

Tests must cover at least:

```text
zero-frame function
single automatic local
multiple simultaneously live locals
sequential non-overlapping locals
mutually exclusive branch locals
fixed arrays
embedded aggregates
tagged-union storage
descriptor versus backing storage
alignment and padding
address-taken local
address-taken parameter
materialized aggregate temporary
return storage
defer-extended storage lifetime
destruction and cleanup overlap
```

---

# Required call-path tests

Tests must cover at least:

```text
leaf function
single direct call
nested direct calls
sequential calls use maximum rather than sum
branch calls use maximum
proven unreachable call is ignored
error-path maximum
cleanup-path maximum
```

---

# Required recursion tests

Tests must cover at least:

```text
direct unconditional recursion -> Unbounded
mutual unconditional recursion -> Unbounded
bounded recursive integer parameter
decreasing parameter without finite root upper bound -> Unknown
bounded mutual recursion
recursive SCC with several cycles
recursive SCC with insufficient progress proof -> Unknown
unreachable recursive SCC does not affect the root
```

The distinction between `Unknown` and `Unbounded` must be explicitly tested.

---

# Required indirect-call tests

Tests must cover at least:

```text
closed single target
closed multiple targets
largest target selected
known targets plus open contract
open contract with finite stack bound
open contract without stack bound -> Unknown
indirect recursion
verified no-reentry contract excludes recursive path
unknown reentry produces conservative Unknown where required
```

---

# Required foreign/runtime tests

Tests must cover at least:

```text
known foreign stack contract
unknown foreign stack contribution
compiler-known runtime stack summary
foreign reentry
callback into the caller SCC
CompilationPlan-specific foreign stack summary
```

Foreign metadata import itself is tested by the FFI/tooling subsystem rather
than this rulebook.

---

# Required storage-accounting tests

Tests must verify the distinction between inline automatic storage and separate
backing storage.

At minimum:

```text
fixed byte[4096] automatic local -> inline stack bytes
dynamic byte[] descriptor with arena backing -> descriptor only
borrowed view -> view representation only
allocator-backed object handle -> handle only
embedded fixed array -> counted through enclosing object
```

---

# Required stack-slot reuse regression tests

Tests must ensure reuse is rejected while any relevant dependency remains live.

At minimum:

```text
borrow remains live
defer uses earlier local
destructor requires earlier object
safe or raw address remains valid
stable-address requirement remains active
partial move leaves aggregate state live
cleanup path references earlier local
unknown overlap under required upper-bound proof
```

---

# Required CompilationPlan tests

Tests must verify that different representative target layouts and ABIs may
produce different machine stack summaries without changing source semantics.

Relevant differences include:

```text
pointer width
alignment
calling convention
saved-register requirements
outgoing-call area
```

The exact CI targets are implementation policy rather than normative language
syntax.

---

# Required budget tests

Tests must cover at least:

```text
Exact below budget -> accepted
UpperBound below budget -> accepted
Exact above budget -> rejected
finite verified upper bound proving excess -> rejected
Unknown with mandatory proof -> failure-to-prove rejection
Unbounded with finite budget -> rejected
Unknown with no applicable finite-proof requirement -> retained analysis result
```

Tests must also verify that semantic and machine budgets are compared against the
correct measurement level.

---

# Required incremental and LSP tests

Tests must cover at least:

```text
change local size -> own-frame refresh
change callee frame -> dependent caller maximum refresh
change unrelated function -> unaffected cache reuse
add or remove call -> call-path refresh
change callable target set -> stack result refresh
add recursion -> SCC recomputed
remove recursion -> finite result restored
change stack budget -> diagnostics refreshed
change analysis depth -> optional insight refreshed
project configuration reload requires no LSP restart
```

---

# Required separate-compilation tests

Tests must cover at least:

```text
compatible imported semantic summary
compatible imported machine summary
wrong CompilationPlan rejected
incompatible schema rejected
missing summary produces localized Unknown
body-derived and imported compatible summaries produce equivalent composition
```

---

# Required performance and termination tests

Stack analysis must be tested on representative large inputs including:

```text
large acyclic call graphs
large SCCs
many execution roots
many closed indirect target sets
large functions with many live ranges
incremental leaf edits
```

The tests must establish bounded analysis termination and deterministic results.

This rulebook does not define a normative millisecond threshold.

---

# Completion criteria

Stack analysis is complete for Sec 0.1 when all of the following are true:

1. semantic own frames can be modeled from materialized automatic storage;
2. stack-slot reuse is constrained by canonical lifetime, escape, destruction,
   cleanup, address, pinning, and storage facts;
3. `ResolvedLayout` is consumed as the authoritative size/alignment source;
4. inline fixed storage is distinguished from descriptors and separate backing;
5. direct call-path stack composition is sound;
6. recursive and mutually recursive SCCs are detected;
7. `Unknown` and `Unbounded` recursion are distinct;
8. simple finite recursion-depth bounds can be proven from canonical range or
   contract facts;
9. closed indirect target sets and open callable stack contracts are consumed;
10. foreign/runtime/platform stack contracts are consumed without assuming
    unknown calls cost zero;
11. possible reentry participates in recursion analysis;
12. execution roots and physical stack domains are represented;
13. semantic and machine stack requirements remain distinct;
14. final machine stack can be revalidated against machine-level budgets;
15. hard stack budgets correctly distinguish proven excess from failure to
    prove;
16. stack summaries are versioned and suitable for separate compilation;
17. dependency-driven incremental invalidation is implemented;
18. LSP and `sec analyse` consume the same canonical stack facts;
19. Interactive, Standard, and Deep analysis follow the common budget model;
20. the required frame, call-path, recursion, indirect, FFI/runtime, storage,
    budget, separate-compilation, incremental, and performance tests pass.

---

# Normative summary

Stack analysis determines stack resource requirements without owning storage,
lifetime, escape, layout, or call-reachability semantics.

Stack pressure never authorizes implicit promotion to heap, arena, or other
non-stack storage.

Semantic and machine stack requirements are distinct compiler facts.

Stack bounds distinguish exact, finite upper-bound, unknown, and proven
unbounded results.

Per-function stack composition uses maximum simultaneously live storage and
maximum reachable call-path demand rather than naïvely summing all declarations
or sequential calls.

Recursive and mutually recursive calls are analyzed through canonical call-graph
SCCs. Finite recursion depth requires a sound bound on every relevant recursive
transition and a finite bound on the initial recursive measure.

Closed indirect-call target sets compose the maximum of all permitted targets.
Open target sets require a verified callable stack contract for a finite proof.
Unknown callable, foreign, runtime, or reentry behavior remains explicitly
unknown rather than being treated as zero.

Stack-frame construction consumes resolved layout and canonical live-range,
ownership, lifetime, destruction, defer, escape, pinning, address, and storage
facts. Stack-slot reuse requires proof that materialized storage cannot overlap.

Fixed arrays and embedded aggregates contribute inline storage. Dynamic owning
collections, views, arena-backed objects, and allocator-backed objects contribute
only their stack-resident descriptors or handles when their backing storage is
separate.

Stack requirements are computed per execution root and physical stack domain
according to the active runtime and CompilationPlan.

Required stack and available stack budget are separate concepts. A finite budget
is satisfied only by an appropriate verified finite bound at the budget's
specified semantic or machine measurement level.

A hard budget failure caused by `Unknown` is a failure to prove the required
resource contract, not proof that overflow must occur. `Unbounded` remains a
separate proven result.

Stack summaries are reusable, versioned, incrementally invalidated compiler
facts consumed by build validation, ISR analysis, LSP, `sec analyse`, resource
reporting, and platform tooling.

Interactive analysis may expose pending or partial results but must remain sound.
Standard performs every stack proof required by the active build. Deep may add
more expensive precision and explanation without changing source semantics.
