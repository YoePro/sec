# Effect Analysis

## Status

This document is the canonical effect-analysis rulebook for Sec 0.1.

It defines how the compiler:

- infers effects from expressions, statements, functions, methods, closures,
  destructors, deferred actions, foreign calls, compiler-generated operations,
  and target-specific lowering;
- distinguishes inferred implementation facts from declared API guarantees;
- propagates effects through the active call graph;
- verifies `@noPanic`, `@noAlloc`, `@noBlock`, `@interruptSafe`, `@isr`, and
  `@interrupt`;
- represents synchronous blocking separately from asynchronous suspension;
- tracks externally observable operations such as I/O, volatile access, shared
  mutation, task creation, and nondeterministic input;
- tracks ordered arena-lifetime effects without exposing a general region type
  system in Sec source;
- integrates effect analysis with ownership, borrowing, escape analysis,
  destruction, stack analysis, target knowledge, and FFI;
- produces stable, cause-aware diagnostics;
- performs all analysis without requiring a Sec runtime.

This rulebook does not introduce new source syntax except where another
canonical rulebook already defines it.

---

# Purpose

Effect analysis answers questions such as:

```text
Can this function panic?
Can this function allocate?
Can this function block?
Can this function suspend?
Can this function perform I/O?
Can this function access volatile storage?
Can this function mutate externally visible state?
Can this function create concurrent work?
Can this function depend on nondeterministic input?
Can this function be called from an interrupt service routine?
Can this function be used where a stronger effect guarantee is required?
Is an arena still live at this program point?
Does an arena reset invalidate a later reference?
What is the maximum live arena allocation on this path?
Why does a transitive guarantee fail?
```

The analysis is compiler-visible and compile-time.

It does not require dynamic effect tags, effect handlers, exception tables,
runtime dispatch, a scheduler, or a mandatory runtime library.

---

# Definition of an effect

An effect is compiler-tracked information about how evaluating an expression or
function may:

```text
change externally observable behavior;
constrain where the operation may safely execute;
consume or release resources;
change storage validity;
change the validity of references or owned values;
interact with external state;
transfer control in a way not represented only by the ordinary return value.
```

Not every effect uses the same representation.

Sec effect analysis contains several cooperating analysis domains.

---

# Non-goals

Effect analysis does not replace:

```text
the type system;
ownership analysis;
borrow checking;
move and invalidation analysis;
escape analysis;
copyability classification;
stack analysis;
recursion analysis;
destruction rules;
target selection;
ABI validation;
unsafe validation.
```

It consumes facts from those analyses and contributes facts back to them.

Sec does not expose a general algebraic-effect language in version 0.1.

Sec does not require programmers to write explicit region parameters, lifetime
variables, linear region tokens, or substructural effect types.

---

# Analysis domains

Sec 0.1 uses four cooperating effect-related domains:

```text
summary may-effects;
ordered arena-lifetime effects;
quantitative resource facts;
trust provenance.
```

These domains must not be collapsed into one undifferentiated flag set.

---

# Summary may-effects

Summary may-effects describe operations that may occur on at least one reachable
execution path.

The initial internal set is:

```text
MayPanic
MayAllocate
MayBlock
MaySuspend
MaySpawn
MayIO
MayAccessVolatile
MayMutateExternalState
MayUseNondeterministicInput
```

These effects are normally combined as an idempotent set.

Example:

```text
{MayAllocate, MayIO}
∪
{MayIO, MayBlock}
=
{MayAllocate, MayIO, MayBlock}
```

The same effect appearing multiple times remains one summary capability, while
the compiler may retain multiple causes for diagnostics.

---

# Ordered arena-lifetime effects

Arena-lifetime effects describe ordered changes to programmer-visible arenas and
arena-backed storage.

The initial conceptual events are:

```text
ArenaCreate(A, capacity)
ArenaAllocate(A, size)
ArenaReset(A)
ArenaRelease(A)
```

Order matters.

This sequence may be valid:

```text
ArenaCreate(A, 4096)
ArenaAllocate(A, 64)
ArenaReset(A)
ArenaAllocate(A, 128)
ArenaRelease(A)
```

This sequence is invalid:

```text
ArenaCreate(A, 4096)
ArenaRelease(A)
ArenaAllocate(A, 64)
```

An arena is the programmer-visible storage mechanism.

The compiler may use an internal storage-lifetime identity to analyze the arena,
but Sec 0.1 does not expose that identity as source-level region syntax.

---

# Quantitative resource facts

Quantitative analysis records facts such as:

```text
maximum live arena allocation;
remaining arena capacity;
unknown allocation bound;
maximum stack contribution;
maximum simultaneous task creation where provable;
path-specific resource peaks.
```

These are not boolean effects.

They are associated with control-flow paths, calls, loops, recursion, arenas,
and concrete compilation plans.

---

# Trust provenance

Trust provenance records why the compiler accepts an effect claim or semantic
fact.

Initial provenance categories include:

```text
proven from Sec source;
derived from a compiler intrinsic;
provided by target knowledge;
trusted through an explicit unsafe FFI declaration;
trusted through an inline assembly declaration;
unknown.
```

Trust provenance is not a runtime effect.

`unsafe` is not represented as `MayUnsafe`.

Instead, the compiler records that a proof depends on an unsafe or trusted
boundary.

---

# Effects versus ordinary return values

An explicit error value is not itself an effect.

Example:

```sec
fn Open() Result[File, OpenError]
```

The function may still have:

```text
MayAllocate
MayBlock
MayIO
```

Returning `Err` does not imply panic, but it also does not remove allocation,
blocking, I/O, suspension, volatile access, or external mutation.

---

# Inferred summaries

Every function, method, closure, destructor, and deferred body receives an
inferred effect summary.

This applies even when no effect attribute is written.

Example:

```sec
fn AddOne(value: int) Result[int, ArithmeticError] {
    let result := try value + 1
    return Ok(result)
}
```

The compiler may infer:

```text
MayPanic: absent
MayAllocate: absent
MayBlock: absent
MaySuspend: absent
```

The inferred summary describes the current implementation.

It may be used for:

```text
local verification;
whole-program verification;
optimization;
call-graph analysis;
ISR analysis;
diagnostics;
LSP information;
specialization;
dead-support elimination.
```

---

# Declared guarantees

Attributes such as:

```text
@noPanic
@noAlloc
@noBlock
@interruptSafe
@isr
@interrupt
```

declare stable guarantees.

A declared guarantee is verified against the inferred summary and all other
required analyses.

Example:

```sec
@noPanic
fn AddOne(value: int) Result[int, ArithmeticError] {
    let result := try value + 1
    return Ok(result)
}
```

A future implementation change that introduces a reachable panic is a compiler
error.

---

# Public contract boundary

Inferred absence of an effect is not automatically a stable public API promise.

Public callers may rely on:

```text
declared guarantees;
guarantees required by an interface;
guarantees required by a function type or generic constraint;
compiler-known intrinsic guarantees;
target-knowledge guarantees where the target contract exposes them.
```

Public callers must not rely on an undeclared implementation accident across a
separate compilation boundary.

Module metadata may store both:

```text
inferred implementation summary;
declared public guarantee.
```

They remain semantically distinct.

---

# Positive internal representation

The compiler stores possible effects positively.

Example:

```text
MayPanic
MayAllocate
MayBlock
```

A negative source guarantee means the corresponding positive effect must be
absent.

```text
@noPanic
    requires absence of MayPanic

@noAlloc
    requires absence of MayAllocate

@noBlock
    requires absence of MayBlock
```

---

# Effect ordering

A function with fewer possible effects satisfies a stronger guarantee.

Conceptually:

```text
{}
    stronger than

{MayAllocate}
    stronger than

{MayAllocate, MayBlock}
    stronger than

{MayPanic, MayAllocate, MayBlock}
```

Effect compatibility uses subset inclusion.

A function with effect set `E1` may be used where `E2` is permitted when:

```text
E1 is a subset of E2
```

---

# Direct effects

A direct effect is introduced by the function body itself.

Examples:

```text
checked arithmetic that may panic;
bounds check that may panic;
explicit panic;
allocator invocation;
arena allocation;
blocking syscall;
suspension;
task or thread creation;
file or socket I/O;
volatile read or write;
external state mutation;
clock or random input;
arena reset;
arena release.
```

---

# Transitive effects

A transitive effect is introduced by reachable behavior.

The summary includes effects from:

```text
direct function calls;
method calls;
generic specializations;
closures;
indirect calls;
interface dispatch;
destructors;
defer bodies;
implicit conversions;
compiler-generated helpers;
foreign functions;
inline assembly declarations;
target-specific intrinsics;
task and thread operations.
```

Conceptually:

```text
EffectiveEffects(F)
    =
DirectEffects(F)
∪
Effects(ReachableCallees(F))
∪
Effects(ImplicitOperations(F))
∪
Effects(Cleanup(F))
```

---

# Call graph requirement

Effect analysis operates over the active call graph for one concrete
compilation plan.

The call graph must include:

```text
direct calls;
resolved method calls;
generic specializations;
closure invocation;
function values;
interface dispatch targets;
destructors;
defer bodies;
compiler-generated operations;
foreign declarations;
task entry functions;
thread entry functions;
supervisor callbacks;
panic sinks where relevant;
target handlers.
```

Unknown call targets are handled conservatively.

---

# Fixed-point analysis

Recursive and mutually recursive functions require fixed-point analysis.

Example:

```text
A calls B
B calls C
C calls A
```

If `C` may allocate, the recursive strongly connected component is
transitively allocation-capable unless proof eliminates the path.

The compiler computes a stable summary for the complete component.

---

# Path sensitivity

An effect is included only when it remains reachable after semantic proof and
control-flow refinement.

Example:

```sec
if false {
    PanicCapableOperation()
}
```

The dead branch contributes no reachable effect.

Example:

```sec
if divisor != 0 {
    let result := total / divisor
}
```

The division-by-zero panic effect may be absent when Sema proves the divisor is
nonzero on the path.

A runtime condition that may be either true or false contributes effects from
both reachable branches.

---

# Branch joins

Summary may-effects join by union.

Ordered arena state joins by state compatibility.

Example:

```sec
if condition {
    arena.Release()
}

Use(arena)
```

At the join point the arena is:

```text
Live on one path;
Released on another path.
```

The later use is invalid.

Example:

```sec
if condition {
    arena.Release()
    return
}

Use(arena)
```

Only the path where the arena remains live reaches `Use`.

The use may be valid.

---

# Loops

Loop analysis computes a fixed point over:

```text
summary effects;
arena state;
borrow liveness;
allocation bounds;
resource peaks;
control-flow exits.
```

Effects inside a reachable loop contribute to the containing function.

Arena allocation may be bounded per iteration when reset or release is proven
before the next iteration.

Example:

```sec
for item in items {
    let arena := Arena.FromBuffer(ref buffer)
    let value := arena.New[Value]()
    Process(item, ref value)
    arena.Reset()
}
```

The analysis may prove one-iteration peak use rather than multiplying capacity
by iteration count.

---

# Optional optimization must not establish guarantees

A declared effect guarantee must not depend only on an optional optimizer.

A guarantee may be established through:

```text
semantic constant evaluation;
required dead-path proof;
mandatory escape analysis;
mandatory call-graph resolution;
type and contract proof;
required lowering semantics;
compiler-known intrinsic semantics.
```

A guarantee must not be valid only because:

```text
release optimization removed an allocation;
debug mode happened to retain one;
inlining happened to remove a helper;
optional loop optimization changed the result.
```

Debug and release builds must agree on semantic effect guarantees for the same
compilation plan and source semantics.

---

# Runtime checks

Language-defined runtime checks contribute effects according to their selected
control flow.

An ordinary checked operation may contribute `MayPanic`.

Example:

```sec
let total := left + right
```

When overflow is possible and not handled explicitly, the operation may panic.

A proven-safe check contributes no panic effect.

---

# `try`

`try` converts supported language-defined failure from panic-capable control
flow to explicit fallible control flow.

Example:

```sec
let total := try left + right
```

The arithmetic overflow check no longer contributes `MayPanic`.

The explicit error flow remains part of the ordinary function result and
control flow.

---

# `try` does not remove unrelated effects

Example:

```sec
let value := try allocator.New[Value]()
```

Allocation failure may be explicit and panic-free, but:

```text
MayAllocate remains.
```

Example:

```sec
let data := try network.Read()
```

The explicit error path does not remove:

```text
MayIO
MayBlock
```

where the operation has those effects.

`try` does not catch or erase panic from arbitrary called functions.

---

# `@noPanic`

`@noPanic` requires absence of `MayPanic`.

The complete guarantee includes:

```text
no reachable explicit panic;
no reachable unchecked language-defined check failure;
no reachable panic-capable call;
no unproven assert;
no reachable checked unreachable;
no panic-capable cleanup;
no unknown foreign abort or unwind accepted as safe.
```

The detailed panic semantics belong to:

```text
panic.md
runtime_checks.md
```

---

# `@noAlloc`

`@noAlloc` requires absence of `MayAllocate`.

Dynamic allocation includes:

```text
heap allocation;
arena allocation that consumes arena capacity;
collection growth;
implicit boxing;
closure environment allocation;
task or thread control storage allocation;
foreign allocation;
compiler helper allocation;
hidden storage acquisition.
```

---

# Operations that are not dynamic allocation

The following are not dynamic allocation merely by existing:

```text
fixed stack locals;
fixed arrays;
static storage;
addressed storage;
register values;
caller-provided storage;
an arena view created over caller-provided storage;
reuse of existing collection capacity without growth;
compiler-elided temporary representation.
```

A specific operation may still allocate according to its semantic definition.

---

# Arena allocation and `@noAlloc`

Creating an arena view over existing storage may be no-allocation.

Example:

```sec
let arena := Arena.FromBuffer(ref buffer)
```

Allocating an object from the arena consumes arena capacity.

Example:

```sec
let value := arena.New[Value]()
```

That operation contributes `MayAllocate`.

Therefore, bounded arena allocation is still allocation.

A separate profile may permit bounded arena allocation while forbidding heap
allocation, but that profile is not the meaning of `@noAlloc`.

---

# `@noBlock`

`@noBlock` requires absence of synchronous blocking.

Blocking includes:

```text
blocking I/O;
sleep;
waiting for a mutex or semaphore;
condition-variable wait;
thread join;
blocking task wait;
scheduler parking that occupies or blocks the execution domain;
foreign calls declared as blocking;
waiting for an unbounded external event.
```

---

# Asynchronous suspension

Asynchronous suspension is represented separately as:

```text
MaySuspend
```

`@noBlock` does not by itself forbid `MaySuspend`.

An asynchronous task may suspend and allow other work to execute without
synchronously blocking an execution thread.

Example:

```sec
@noBlock
async fn ReadAsync() Result[Data, ReadError] {
    return await device.Read()
}
```

The function may be:

```text
MaySuspend
MayIO
```

while remaining free from `MayBlock`.

Exact `async` and `await` source semantics remain defined by their dedicated
rulebook.

---

# ISR suspension rule

`@isr` and `@interruptSafe` forbid both:

```text
MayBlock
MaySuspend
```

Interrupt execution may not block, park, yield, or resume later as an ordinary
task continuation.

---

# `MaySpawn`

Creating concurrent work contributes `MaySpawn`.

Examples:

```text
task creation;
thread creation;
detached work creation;
scheduler submission where it creates a new execution instance.
```

The same operation may also contribute:

```text
MayAllocate;
MaySuspend;
MayMutateExternalState.
```

`MaySpawn` remains separate because ISR, lifetime, supervision, embedded
profiles, and structured-concurrency analysis need to identify it directly.

---

# `MayIO`

`MayIO` describes interaction with external input or output through an I/O
abstraction.

Examples:

```text
file operations;
socket operations;
device APIs;
terminal operations;
foreign I/O;
platform services.
```

I/O may be:

```text
blocking;
nonblocking;
suspending;
polling.
```

`MayIO` is therefore not equivalent to `MayBlock`.

---

# `MayAccessVolatile`

Volatile access contributes:

```text
MayAccessVolatile
```

Examples:

```text
read from addressed storage;
write to addressed storage;
volatile register access;
compiler-known hardware register operations.
```

Volatile operations may not be:

```text
removed;
duplicated;
merged unsafely;
reordered contrary to the volatile rules;
treated as ordinary constant memory.
```

Volatile access is not automatically classified as ordinary file or socket I/O.

---

# `MayMutateExternalState`

This effect describes mutation observable outside local computation.

Examples:

```text
global mutable state;
shared mutable state;
foreign state;
device state;
externally visible storage;
persistent state;
state reachable through an externally supplied mutable reference where the
function contract exposes the mutation.
```

Local temporary mutation does not by itself contribute this effect.

Example:

```sec
fn Normalize(value: int) int {
    let mut local := value
    // Local computation only.
    return local
}
```

---

# `MayUseNondeterministicInput`

This effect describes dependence on input that may vary independently of the
ordinary explicit parameters.

Examples:

```text
clock;
random source;
environment state;
process identity;
thread identity;
uncontrolled external device state;
foreign nondeterministic input.
```

A volatile read may also be nondeterministic, but the effects remain separately
recorded.

---

# Composite interrupt contracts

`@isr` is a composite verified contract.

It requires at least:

```text
absence of MayPanic;
absence of MayAllocate;
absence of MayBlock;
absence of MaySuspend;
restricted MaySpawn;
interrupt-safe I/O and volatile access;
safe external-state mutation;
bounded stack under the target ISR policy;
target-compatible signature;
target-compatible entry and return behavior;
interrupt-safe reachable call graph;
no forbidden synchronization;
no unknown effect source.
```

Stack and ABI facts come from other analyses.

Effect analysis coordinates them.

---

# `@interrupt`

`@interrupt(...)` implies `@isr`.

Therefore:

```text
@interrupt(...)
    implies all @isr effect restrictions.
```

Vector binding and target lowering remain defined by `attributes.md` and target
knowledge.

---

# `@interruptSafe`

`@interruptSafe` verifies that a function or method may be called from ISR code.

It requires at least:

```text
absence of MayPanic;
absence of MayAllocate;
absence of MayBlock;
absence of MaySuspend;
no forbidden spawning;
safe volatile and external-state behavior;
compatible reachable callees;
acceptable stack behavior;
no unknown effect source.
```

It does not bind a vector and does not make the function an ISR entrypoint.

---

# Target-aware ISR effects

Not all I/O or external mutation is forbidden in an ISR.

Target knowledge may classify operations such as:

```text
acknowledging an interrupt;
reading a status register;
writing a device command register;
clearing an interrupt flag;
performing a bounded lock-free queue operation.
```

The compiler must use target and operation-specific knowledge rather than a
blanket rule that every volatile access is invalid.

Unknown behavior remains conservative.

---

# Arena model

Sec takes a limited arena-lifetime idea from region-effect research without
adopting a general region language.

The programmer sees:

```text
arena;
arena allocation;
arena reset;
arena release;
references to arena-backed storage.
```

The compiler may assign internal lifetime identities.

Those identities are not normally visible in source.

---

# Arena state

The initial arena state model is:

```text
Live
Released
```

`ArenaReset` is an event that:

```text
keeps the arena Live;
restores capacity according to the arena definition;
invalidates all previous allocations from that arena;
invalidates references depending on those allocations.
```

`ArenaRelease`:

```text
ends ownership of the arena storage;
changes the arena to Released;
invalidates all dependent allocations and references;
forbids later allocation, reset, or ordinary use.
```

---

# Arena-backed references

The compiler records when a value or reference depends on arena-backed storage.

Conceptual facts:

```text
StorageDependsOn(value, A)
ReferenceDependsOn(reference, A)
```

Example invalid program:

```sec
let arena := Arena.FromBuffer(ref buffer)
let value := arena.New[Value]()
let view := ref value

arena.Reset()

Use(view)
```

`view` depends on an allocation invalidated by `ArenaReset`.

---

# Non-lexical borrow live ranges

A borrow may end at its last possible use rather than at the end of the lexical
block.

Example:

```sec
let value := arena.New[Value]()
let view := ref value

Consume(view)

// The borrow is no longer live.
arena.Reset()
```

The reset may be valid when the compiler proves that `view` is not used later.

The source variable may remain in lexical scope while its borrow live range has
ended.

---

# Arena lifetime is not a source-code block

The term storage lifetime does not mean a lexical source block.

An arena may:

```text
be created after block entry;
remain live across nested blocks;
be reset several times;
be released before block exit;
be passed through calls;
have references whose live ranges end before the arena.
```

Sec source does not require explicit region parameters for this analysis.

---

# Arena analysis does not replace ownership

Ownership analysis determines:

```text
who owns the arena;
whether the arena may move;
whether an owner remains valid;
whether release occurs exactly according to ownership rules.
```

Borrow analysis determines:

```text
shared and mutable borrow conflicts;
borrow live ranges;
field separation;
alias validity.
```

Escape analysis determines:

```text
whether references outlive arena-backed storage;
whether closures retain arena references;
whether returned references are valid.
```

Effect analysis tracks ordered arena events and coordinates these facts.

---

# Arena capacity

Arena allocation may carry a compile-time or symbolic size.

Conceptual:

```text
ArenaAllocate(A, size)
```

The compiler may compute:

```text
remaining capacity;
maximum live allocation;
per-path capacity use;
unknown bound.
```

Allocation fails or becomes fallible according to the arena API and allocation
rulebook.

`try` may make capacity failure explicit, but `MayAllocate` remains.

---

# Arena reset in loops

A reset before the next iteration may allow bounded peak analysis.

Example:

```sec
for item in items {
    let value := arena.New[Value]()
    Process(item, ref value)
    arena.Reset()
}
```

When no arena-backed value escapes and all borrows end before reset, the
compiler may report one-iteration peak capacity rather than accumulated use.

---

# Arena release across branches

Example:

```sec
if condition {
    arena.Release()
}

Use(arena)
```

The use is rejected because at least one reaching path has released the arena.

The compiler must report the release path.

---

# Arena splitting

Sub-arenas and explicit arena splitting are not part of this rulebook unless
their source and ownership semantics are separately decided.

The analysis architecture must permit a future event such as:

```text
ArenaSplit(parent, child, capacity)
```

but Sec 0.1 does not gain source syntax merely because the internal model can
represent it.

---

# Function summaries for arenas

A function summary may include facts such as:

```text
requires arena A live;
allocates at most N bytes from A;
may reset A;
releases A;
returns storage depending on A;
does not retain a reference to A;
does not change external arena liveness.
```

These facts may be inferred and stored in module metadata.

They are not necessarily written as explicit region syntax.

---

# Local arena activity

A function may create, use, and release a private arena without exposing the
arena lifetime to callers.

The caller summary may still include:

```text
MayAllocate;
maximum temporary allocation.
```

It need not expose an external arena-state transition when no arena-backed value
escapes.

---

# Ownership transfer effects

Move and invalidation remain governed by `copy_move.md` and `ownership.md`.

Effect analysis may consume ordered facts such as:

```text
owner moved;
source invalidated;
destination initialized.
```

These facts are not part of the public summary may-effect set.

They are control-flow and value-validity facts used by cooperating analyses.

---

# Destructors

The effective summary of a function includes destructors that may run on
reachable exits.

Destructors are already required to satisfy the panic rules defined elsewhere.

A destructor may still contribute other effects unless prohibited.

Possible destructor effects include:

```text
MayAllocate;
MayBlock;
MayIO;
MayAccessVolatile;
MayMutateExternalState.
```

A containing guarantee must include or forbid them accordingly.

---

# `defer`

A deferred body contributes effects to every exit path where it runs.

Example:

```sec
@noBlock
fn Work() void {
    defer {
        BlockingClose()
    }
}
```

The function violates `@noBlock`.

The violation occurs even when the explicit statements before scope exit do not
block.

---

# Cleanup paths

Effect analysis includes cleanup on:

```text
normal return;
early return;
explicit error propagation;
scope exit;
task cancellation where cleanup is defined;
contained panic cleanup where the panic rulebook requires it.
```

A cleanup path may have stricter panic rules than an ordinary path.

---

# Closures

A closure summary includes:

```text
body effects;
capture creation effects;
capture destruction effects;
environment allocation;
effects of values retained by the closure.
```

A closure that requires heap storage contributes `MayAllocate`.

A closure whose environment is stack-proven may avoid that effect.

The proof must not depend only on optional optimization.

---

# Function values and indirect calls

Effect guarantees are semantically part of function compatibility.

Exact source syntax for effect-constrained function types may be defined later.

The semantic rule is fixed:

```text
a function with fewer effects may be used where more effects are permitted;
a function with more effects may not be used where fewer are required.
```

An indirect call with unknown effect guarantees is conservative.

---

# Callback example

Conceptual:

```sec
@noPanic
fn Execute(callback: fn() void) void {
    callback()
}
```

The function cannot be verified as `@noPanic` unless the callback contract or
resolved target proves absence of `MayPanic`.

The compiler must not assume a callback is panic-free from its current observed
uses when the public type permits future unknown values.

---

# Interfaces

An interface method may declare effect guarantees.

Every implementation must satisfy them.

An implementation may provide a stronger guarantee.

Example:

```text
interface method permits allocation;
implementation is noAlloc;
valid.
```

Invalid:

```text
interface method is noAlloc;
implementation may allocate.
```

Effect compatibility uses subset inclusion.

---

# Open and closed worlds

For a complete executable build, the compiler may know every concrete dispatch
target.

For a public library or extensible interface boundary, future implementations
may be unknown.

The compiler must use declared public effect contracts across open boundaries.

It must not convert an accidental property of currently known implementations
into an undeclared stable guarantee.

---

# Generics

Generic effect summaries may depend on:

```text
operations available on T;
copy, move, and destruction behavior of T;
callback effect constraints;
selected generic implementations;
specialization;
target-specific implementations.
```

A summary may remain symbolic until specialization.

Conceptual:

```text
Effects(Apply[T, F]) includes Effects(F)
```

Exact effect-constraint syntax for generics is deferred.

Unknown generic effects are conservative.

---

# Specialization

A concrete specialization may prove fewer effects than the generic maximum.

Example:

```text
generic checked arithmetic may panic;
a constrained specialization proves overflow impossible.
```

The compiler may use the stronger inferred specialization summary internally.

Public guarantees remain governed by the generic declaration and applicable
constraints.

---

# Foreign functions

Unknown foreign code is conservative.

Unless explicitly and safely described by the FFI model, it may contribute:

```text
MayPanic or foreign abort/unwind;
MayAllocate;
MayBlock;
MaySuspend where applicable;
MaySpawn;
MayIO;
MayAccessVolatile;
MayMutateExternalState;
MayUseNondeterministicInput.
```

A normal Sec effect attribute on an extern declaration is not silently treated
as proof.

Foreign effect declarations require an explicit unsafe trust model.

---

# Trusted foreign summaries

A future FFI rule may permit an unsafe declaration of foreign effects.

The compiler must retain provenance:

```text
effect absence trusted from foreign declaration
```

Diagnostics may say:

```text
ISR safety depends on trusted foreign declaration `ReadStatusRegister`
```

The compiler must not report the fact as proven from Sec source.

---

# Inline assembly

Inline assembly requires explicit effect classification.

The compiler cannot infer arbitrary machine-code behavior from text alone.

An inline assembly declaration may need to describe:

```text
memory reads;
memory writes;
volatile access;
external mutation;
control-flow behavior;
stack behavior;
register clobbers;
possible blocking;
possible trap or abort.
```

Accepted claims carry inline-assembly trust provenance.

---

# Compiler intrinsics

Compiler intrinsics have compiler-known summaries.

Examples may include:

```text
volatile load;
volatile store;
target trap;
nonblocking register operation;
allocation primitive;
scheduler primitive.
```

The intrinsic definition is the source of truth.

---

# Control-flow facts

Some compiler-tracked control-flow properties are not initial public effects.

Internal facts may include:

```text
MayTerminateProcess
MayTrap
MayNotReturn
```

These are distinct from `MayPanic`.

A function may be `@noPanic` and still:

```text
exit the process;
trap by explicit target operation;
run forever;
never return by design.
```

The relevant entrypoint, panic, and control-flow rulebooks govern legality.

---

# Target and configuration selection

Effect analysis runs after:

```text
file-level @target selection;
statement-level @target selection;
@when configuration selection;
target knowledge selection;
implementation-variant selection.
```

Only active source contributes to the active call graph and effect summary.

Excluded source remains parsed but does not contribute effects for the current
compilation plan.

---

# Per-compilation-plan analysis

Every concrete compilation plan receives its own effect analysis.

A logical function may have different implementations per:

```text
OS;
architecture;
CPU;
device;
board;
build configuration.
```

A declared public guarantee must hold in every active variant where the
declaration promises that guarantee.

Cross-variant validation must report inconsistent guarantees.

---

# Knowledge packs

Target knowledge packs may provide effect and execution facts for:

```text
interrupt vectors;
peripheral operations;
volatile registers;
intrinsics;
syscalls;
ABI helpers;
scheduler operations;
device-specific I/O;
target traps;
startup and shutdown behavior.
```

Knowledge-pack facts carry target-knowledge provenance.

Unknown target behavior remains conservative.

---

# Semantic IR requirements

Semantic IR must represent effect-relevant operations explicitly.

It must not hide important behavior inside opaque generic nodes.

At minimum, the IR must make visible:

```text
panic-capable checks;
fallible checks handled by try;
allocation;
arena allocation;
arena reset;
arena release;
blocking operation;
suspension;
spawn;
I/O;
volatile load;
volatile store;
external mutation;
nondeterministic input;
destructor invocation;
defer invocation;
foreign call;
inline assembly;
implicit compiler helper;
indirect call;
interface dispatch.
```

---

# Function summary representation

Conceptual compiler structure:

```go
type EffectSummary struct {
    MayPanic                    bool
    MayAllocate                 bool
    MayBlock                    bool
    MaySuspend                  bool
    MaySpawn                    bool
    MayIO                       bool
    MayAccessVolatile           bool
    MayMutateExternalState      bool
    MayUseNondeterministicInput bool

    Causes map[EffectKind][]EffectCause
}
```

This is illustrative, not mandatory implementation syntax.

Arena state and quantitative facts should use separate structures.

---

# Effect causes

An effect summary must retain causes.

Conceptual:

```go
type EffectCause struct {
    Kind       EffectKind
    Source     source.Location
    Operation  string
    Callee     *FunctionID
    Parent     *EffectCause
    Provenance TrustProvenance
}
```

A boolean summary alone is insufficient for diagnostics.

---

# Arena analysis representation

Conceptual:

```go
type ArenaState int

const (
    ArenaLive ArenaState = iota
    ArenaReleased
)

type ArenaFacts struct {
    State                 ArenaState
    Generation            uint64
    Capacity              SizeValue
    MaximumLiveAllocation SizeValue
    CurrentLiveAllocation SizeValue
}
```

`Generation` may model invalidation across resets internally.

This is an implementation option, not source semantics.

---

# Reset generation model

One practical implementation may increment an arena generation at reset.

Arena-backed values and references retain the generation in which they were
created as compiler metadata.

After reset:

```text
arena generation changes;
old dependent values are invalid;
new allocations use the new generation.
```

The generated program does not require runtime generation checks when static
analysis proves validity.

A target profile may separately choose generational runtime references, but that
is not required by this effect analysis.

---

# No mandatory runtime

Effect analysis is compile-time.

It must not require:

```text
runtime effect objects;
effect stack;
effect dispatcher;
dynamic effect checks;
exception unwinder;
mandatory scheduler;
mandatory arena metadata;
mandatory generational references;
mandatory allocation tracker.
```

Compiler metadata may be discarded after compilation unless needed for separate
compilation, debugging, LSP, or diagnostics.

---

# Dead support elimination

When effect analysis proves support is unused, the binary may omit it.

Examples:

```text
panic-free binary omits panic endpoint;
no managed tasks means no scheduler support;
no allocation means no allocator support;
no volatile access means no volatile helper;
no arena use means no arena helper.
```

Direct target instructions or inline lowering may be used without introducing a
general runtime.

---

# Diagnostics

Diagnostics must identify:

```text
the failed declared guarantee;
the introducing operation;
the complete relevant call chain;
the source location;
whether the fact was proven, trusted, or unknown;
the active compilation plan;
the relevant arena state transition;
the invalidated reference or value;
possible explicit alternatives where language rules define them.
```

---

# Panic diagnostic example

```text
error: `ProcessInvoice` does not satisfy @noPanic

ProcessInvoice
  -> calls CalculateTotals
  -> performs checked multiplication
  -> overflow is not proven impossible

effect introduced at:
    invoice.sec:142
```

---

# Allocation diagnostic example

```text
error: `HandlePacket` does not satisfy @noAlloc

HandlePacket
  -> calls Decode
  -> grows collection `fields`
  -> collection growth may allocate

effect introduced at:
    decoder.sec:87
```

---

# Blocking diagnostic example

```text
error: `PollDevice` does not satisfy @noBlock

PollDevice
  -> calls ReadDevice
  -> calls OSRead
  -> operation may wait for external input

effect introduced at:
    platform/linux/device.sec:51
```

---

# Suspension diagnostic example

```text
error: ISR function `TimerHandler` may suspend

TimerHandler
  -> calls WaitForTask
  -> await may suspend the current task

@isr forbids both blocking and suspension
```

---

# Arena invalidation diagnostic example

```text
error: reference `view` depends on storage invalidated by arena reset

reference created:
    parser.sec:40

arena reset:
    parser.sec:46

invalid use:
    parser.sec:49
```

---

# Trust diagnostic example

```text
error: @interruptSafe cannot be proven

ReadRegister
  -> calls foreign function `device_read`
  -> foreign blocking behavior is unknown

help:
    provide a valid unsafe foreign effect declaration,
    use a compiler-known target intrinsic,
    or remove @interruptSafe
```

---

# Multiple causes

When several causes exist, diagnostics may summarize:

```text
MayPanic
    3 causes

MayAllocate
    2 causes

MayBlock
    1 cause
```

The LSP should permit navigation to each cause.

---

# LSP behavior

The LSP should display:

```text
inferred effects;
declared guarantees;
effective implied guarantees;
direct versus transitive effects;
effect causes;
call chains;
trust provenance;
active compilation plan;
arena lifetime and invalidation information;
quantitative allocation facts where available.
```

Example hover:

```text
Effects:
    MayIO
    MaySuspend

Guarantees:
    noBlock

Provenance:
    Sec source
```

---

# Function compatibility diagnostics

When a function value or implementation has too many effects, report the
specific difference.

Example:

```text
callback requires:
    noPanic
    noAlloc

provided function may:
    allocate

allocation introduced at:
    callback.sec:31
```

---

# Implementation architecture

A recommended pipeline is:

```text
parse source;
resolve target and configuration selection;
construct active declarations;
perform name and type resolution;
build Semantic IR;
identify direct effects;
build active call graph;
infer transitive summary effects;
run arena-lifetime dataflow;
run quantitative resource analysis;
combine ownership, borrow, escape, stack, and target facts;
verify declared guarantees;
emit diagnostics;
lower verified Semantic IR.
```

Some stages may iterate to a fixed point.

---

# Effect registry

The compiler should define a central registry for effect kinds.

Each effect entry should identify:

```text
stable internal identity;
display name;
summary domain;
source operations that introduce it;
guarantees that forbid it;
composite contracts that restrict it;
diagnostic category;
module metadata encoding.
```

Do not distribute effect meaning across unrelated ad hoc checks.

---

# Separate compilation

Compiled module metadata should contain enough information to verify callers
without re-reading the source implementation.

At minimum:

```text
declared public guarantees;
required callback or interface guarantees;
conservative public effect summary where needed;
generic effect dependencies;
trusted provenance at public boundaries;
arena escape and lifetime requirements that cross the API.
```

Private inferred details need not become public contract.

---

# Versioning

Changing a declared public effect guarantee may be an API compatibility change.

Examples:

```text
removing @noPanic weakens the API;
removing @noAlloc weakens the API;
adding MayBlock to a previously @noBlock function violates the contract;
strengthening an implementation while keeping the same declaration is safe.
```

The project and ABI rulebooks determine exact compatibility policy.

---

# Effect inference and diagnostics mode

The compiler may report inferred effects without requiring attributes.

Examples:

```text
information: function is inferred noPanic, noAlloc, and noBlock
```

Such reports must not silently modify source.

An explicit semantic tooling command may propose attributes when the guarantee
is proven across the intended build variants.

Normal formatting must not add effect attributes.

---

# Current language-design status

The following semantics are decided:

```text
all functions receive inferred effect summaries;
declared effect attributes are verified guarantees;
public callers rely on declared guarantees;
summary may-effects and ordered arena effects are separate domains;
MayBlock and MaySuspend are distinct;
@noBlock forbids synchronous blocking but not asynchronous suspension;
@isr and @interruptSafe forbid both blocking and suspension;
@noAlloc includes arena capacity consumption;
try removes only the supported panic path, not unrelated effects;
effect compatibility uses subset inclusion;
unknown indirect and foreign effects are conservative;
arena reset invalidates earlier arena-backed values and references;
arena release ends arena use;
borrow live ranges may be non-lexical;
effect analysis is per compilation plan;
effect analysis requires no runtime.
```

---

# Source syntax deliberately deferred

This rulebook does not decide:

```text
effect-constrained function-type syntax;
effect constraints on generic parameters;
unsafe foreign effect declaration syntax;
inline assembly effect-declaration syntax;
future @noSuspend attribute;
future pure or deterministic attribute;
future arena split syntax;
future visible region syntax.
```

The semantics are prepared for those features without inventing source forms.

---

# Initial implementation scope

The initial implementation should prioritize:

```text
MayPanic;
MayAllocate;
MayBlock;
MaySuspend;
direct and transitive function summaries;
@noPanic verification;
@noAlloc verification;
@noBlock verification;
@isr and @interruptSafe composite verification;
try effect transformation;
defer and destructor inclusion;
foreign unknown-effect handling;
basic arena live/reset/release analysis;
arena-backed reference invalidation;
full cause-aware diagnostics.
```

Additional summary effects may be implemented incrementally.

---

# Required tests

Create or update:

```text
effects_valid.sec
effects_invalid.sec
no_panic_effects_valid.sec
no_panic_effects_invalid.sec
no_alloc_effects_valid.sec
no_alloc_effects_invalid.sec
no_block_effects_valid.sec
no_block_effects_invalid.sec
suspension_effects_valid.sec
suspension_effects_invalid.sec
isr_effects_valid.sec
isr_effects_invalid.sec
interrupt_safe_effects_valid.sec
interrupt_safe_effects_invalid.sec
arena_effects_valid.sec
arena_effects_invalid.sec
effect_callbacks_valid.sec
effect_callbacks_invalid.sec
effect_interfaces_valid.sec
effect_interfaces_invalid.sec
effect_generics_valid.sec
effect_generics_invalid.sec
effect_ffi_valid.sec
effect_ffi_invalid.sec
```

---

# Summary-effect tests

Test:

```text
direct effect;
transitive effect;
multiple call levels;
mutual recursion;
dead branch elimination;
runtime branch union;
generic specialization;
indirect call;
interface dispatch;
closure body;
closure allocation;
destructor effect;
defer effect;
compiler-generated helper;
target-specific implementation.
```

---

# `try` tests

Test that:

```text
try arithmetic removes MayPanic from arithmetic failure;
try bounds handling removes the handled panic path;
try allocation retains MayAllocate;
try blocking I/O retains MayBlock and MayIO;
try suspending operation retains MaySuspend;
try does not catch panic from arbitrary called functions.
```

---

# `@noAlloc` tests

Test:

```text
heap allocation rejected;
arena allocation rejected;
collection growth rejected;
existing capacity reuse accepted;
fixed stack array accepted;
caller-provided storage accepted;
closure heap environment rejected;
stack-proven closure accepted;
task control allocation rejected;
transitive allocation rejected.
```

---

# `@noBlock` tests

Test:

```text
blocking I/O rejected;
mutex wait rejected;
sleep rejected;
thread join rejected;
async suspension accepted when no synchronous blocking occurs;
synchronous task wait rejected;
transitive blocking rejected;
nonblocking polling accepted.
```

---

# ISR tests

Test:

```text
MayPanic rejected;
MayAllocate rejected;
MayBlock rejected;
MaySuspend rejected;
forbidden spawn rejected;
approved volatile register access accepted;
unknown volatile helper rejected;
unsafe shared mutation rejected;
bounded interrupt-safe helper accepted;
trusted foreign dependency reported.
```

---

# Arena tests

Test:

```text
allocate while live;
reset and allocate again;
release and later allocate rejected;
release and later reset rejected;
reference used after reset rejected;
reference used after release rejected;
borrow ending before reset accepted;
branch releases on one path then use rejected;
release-and-return branch then use accepted;
loop reset produces bounded peak;
arena-backed reference escape rejected;
private arena does not expose external lifetime state.
```

---

# Compatibility tests

Test:

```text
fewer-effect implementation satisfies broader contract;
more-effect implementation rejected for narrower contract;
interface guarantee enforced;
unknown callback conservative;
resolved noPanic callback accepted;
public undeclared inferred absence not treated as stable contract.
```

---

# Binary tests

Verify:

```text
effect analysis introduces no runtime support;
panic-free build may omit panic endpoint;
allocation-free build may omit allocator support;
no task use may omit scheduler support;
arena analysis does not require runtime generation metadata;
target-specific inactive effects do not reach object code.
```

---

# Diagnostics tests

Diagnostics must test:

```text
stable ID;
source location;
direct cause;
complete call chain;
active compilation plan;
explicit versus implied guarantee;
trusted versus proven provenance;
arena reset or release source;
invalid reference source;
help text where a defined alternative exists.
```

---

# Required synchronization

This rulebook must remain synchronized with:

```text
attributes.md
runtime_checks.md
panic.md
allocation rulebook
ownership.md
copy_move.md
memory_model.md
defer rulebook
destruction rulebook
functions rulebook
interfaces rulebook
generics rulebook
closures and lambdas rulebook
async and task rulebook
thread and concurrency rulebook
arena rulebook
FFI rulebook
inline assembly rulebook
interrupt and ISR rulebook
stack analysis rulebook
call graph rulebook
compiler pipeline rulebook
Semantic IR rulebook
projects.txt
formatter.md
lsp.md
diagnostics rulebook
language-rulebook-status.md
rules_implementations.txt
```

---

# Appendix A — Codex implementation plan

## A.1 Add the rulebook

Add:

```text
rules/analysis/effect_analysis.md
```

Update:

```text
language-rulebook-status.md
rules/compiler/rules_implementations.txt
```

Mark the rulebook Written.

Do not mark all effect analysis implemented.

---

## A.2 Inventory existing effect logic

Locate current logic for:

```text
panic-capable operations;
allocation detection;
blocking detection;
ISR restrictions;
interrupt-safe restrictions;
call graph;
defer;
destructors;
foreign calls;
inline assembly;
arena validity;
borrow liveness;
escape analysis;
target knowledge.
```

Do not create parallel analyses when existing compiler facts can be unified.

---

## A.3 Add central effect kinds

Create stable internal identities for:

```text
MayPanic
MayAllocate
MayBlock
MaySuspend
MaySpawn
MayIO
MayAccessVolatile
MayMutateExternalState
MayUseNondeterministicInput
```

Keep ordered arena events separate.

---

## A.4 Add cause tracking

Every introduced effect must retain:

```text
effect kind;
source location;
operation;
callee;
parent cause;
provenance;
active compilation plan.
```

Do not keep only booleans.

---

## A.5 Mark direct Semantic IR effects

Annotate or classify Semantic IR nodes for:

```text
checked operations;
try-transformed operations;
allocation;
arena operations;
blocking;
suspension;
spawn;
I/O;
volatile access;
external mutation;
nondeterministic input;
cleanup;
foreign calls;
inline assembly;
implicit helpers.
```

---

## A.6 Build direct summaries

Compute direct summary effects per function, method, closure, destructor, and
defer body.

Include implicit operations.

---

## A.7 Propagate through call graph

Compute transitive summaries.

Use strongly connected components for recursion.

Retain shortest or clearest cause chains and all alternative roots needed for
diagnostics.

---

## A.8 Verify declared guarantees

Integrate with attributes:

```text
@noPanic
@noAlloc
@noBlock
@interruptSafe
@isr
@interrupt
```

Compute explicit and implied guarantees separately.

---

## A.9 Implement suspension distinction

Do not classify every `await` as synchronous blocking.

Add `MaySuspend`.

Verify:

```text
@noBlock permits MaySuspend;
@isr rejects MaySuspend;
@interruptSafe rejects MaySuspend.
```

---

## A.10 Implement arena dataflow

Track:

```text
arena identity;
Live or Released state;
reset generation;
capacity where known;
arena-backed values;
arena-backed references;
last use;
branch joins;
loop fixed points.
```

Reject use after reset or release.

---

## A.11 Integrate non-lexical borrow liveness

Use last-use and control-flow information so a borrow may end before lexical
scope exit.

Do not require explicit source regions.

---

## A.12 Quantitative allocation

Compute where possible:

```text
current arena use;
maximum live arena use;
unknown bounds;
loop peak rather than naive accumulated total.
```

Keep quantitative failure separate from boolean `MayAllocate`.

---

## A.13 Foreign and assembly provenance

Treat unknown effects conservatively.

Prepare representation for future explicit unsafe effect declarations.

Do not invent their syntax.

---

## A.14 Interfaces and function compatibility

Use effect subset rules for:

```text
interface implementation;
function values;
callbacks;
method replacement;
generic constraints when available.
```

Do not infer public callback guarantees from only current call sites.

---

## A.15 Module metadata

Emit declared guarantees and required public effect information.

Keep private inferred implementation facts distinct.

---

## A.16 LSP

Add:

```text
effect hover;
guarantee hover;
call-chain navigation;
cause navigation;
trust provenance;
arena invalidation display;
active compilation-plan display;
code actions for explicit handling where canonical.
```

---

## A.17 Testing

Run:

```text
go test ./...
compiler build
LSP build
formatter tests
fixture validation
effect analysis tests
target matrix tests
binary dependency tests
```

Do not claim complete implementation while any required effect category is only
partially modeled.

---

# Current implementation status

Implemented foundations now include:

```text
compiler-owned direct Arena effect sites for recognized create, allocate,
Reset, and Release operations;
MayAllocate classification for WithCapacity, Growable, New, and Alloc;
synchronous MayAllocate propagation over resolved direct/static-method edges;
shortest callable allocation cause paths;
execution-aware exclusion of spawned-body allocation from the spawner's
synchronous summary;
LSP hover presentation of these Arena allocation facts.
```

This remains partial. The repository does not yet claim the complete canonical
effect set, guarantee verification, ambient allocation-context resolution,
path-aware ordered Arena state merging, indirect/open-target propagation,
per-`CompilationPlan` summaries, or general diagnostic cause paths.

---

# Appendix B — Canonical effect table

| Internal effect | Meaning | Forbidden by |
|---|---|---|
| `MayPanic` | reachable language panic or untrusted abort/unwind | `@noPanic`, `@isr`, `@interruptSafe` |
| `MayAllocate` | dynamic storage allocation, including arena capacity consumption | `@noAlloc`, `@isr`, `@interruptSafe` |
| `MayBlock` | synchronous blocking or execution-domain parking | `@noBlock`, `@isr`, `@interruptSafe` |
| `MaySuspend` | asynchronous suspension with later continuation | `@isr`, `@interruptSafe` |
| `MaySpawn` | creation of concurrent execution | restricted by ISR and profile rules |
| `MayIO` | external I/O operation | restricted by ISR and other profiles |
| `MayAccessVolatile` | volatile memory or register access | restricted by operation and target policy |
| `MayMutateExternalState` | externally observable mutation | restricted by ownership, ISR, and predicate profiles |
| `MayUseNondeterministicInput` | clock, random, environment, or uncontrolled input | restricted by deterministic profiles |

---

# Appendix C — Canonical arena event table

| Arena event | Meaning |
|---|---|
| `ArenaCreate(A, capacity)` | create or establish a live arena over storage |
| `ArenaAllocate(A, size)` | consume capacity and create arena-backed storage |
| `ArenaReset(A)` | invalidate prior allocations while keeping the arena live |
| `ArenaRelease(A)` | end the arena and invalidate all dependent storage |

---

# Final canonical summary

Sec performs effect analysis at compile time.

Every function receives an inferred effect summary.

Declared effect attributes are stable verified guarantees.

Public callers rely on declared guarantees, not accidental implementation facts.

The initial summary effects include:

```text
MayPanic
MayAllocate
MayBlock
MaySuspend
MaySpawn
MayIO
MayAccessVolatile
MayMutateExternalState
MayUseNondeterministicInput
```

`@noPanic`, `@noAlloc`, and `@noBlock` require absence of their corresponding
effects.

`MayBlock` and `MaySuspend` are distinct.

`@noBlock` forbids synchronous blocking but permits asynchronous suspension.

`@isr` and `@interruptSafe` forbid both blocking and suspension.

`try` removes only the supported panic-capable failure path.

It does not remove allocation, blocking, suspension, I/O, or other effects.

Arena allocation contributes `MayAllocate`.

Arena reset invalidates previous arena-backed storage and references.

Arena release ends the arena and forbids later use.

Borrow live ranges may end non-lexically at last use.

Sec uses this limited ordered arena analysis without exposing a general region
type system.

Effect compatibility uses subset inclusion.

Unknown indirect, foreign, assembly, and target behavior is conservative.

Effect analysis includes defer, destructors, implicit operations, generics,
closures, callbacks, interfaces, and target-selected implementations.

Effect analysis is performed per concrete compilation plan.

Effect analysis introduces no mandatory runtime.
