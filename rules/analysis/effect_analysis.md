# Effect Analysis

- **Status:** Normative
- **Created:** 2026-08-04
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering)
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/analysis/effect_analysis.md`
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`

---

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

## § 1 Purpose

**§ 1(1)** Effect analysis answers questions such as:

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

**§ 1(2)** The analysis is compiler-visible and compile-time.

**§ 1(3)** It does not require dynamic effect tags, effect handlers, exception tables,
runtime dispatch, a scheduler, or a mandatory runtime library.

---

## § 2 Definition of an effect

**§ 2(1)** An effect is compiler-tracked information about how evaluating an expression or
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

**§ 2(2)** Not every effect uses the same representation.

**§ 2(3)** Sec effect analysis contains several cooperating analysis domains.

---

## § 3 Non-goals

**§ 3(1)** Effect analysis does not replace:

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

**§ 3(2)** It consumes facts from those analyses and contributes facts back to them.

**§ 3(3)** Sec does not expose a general algebraic-effect language in version 0.1.

**§ 3(4)** Sec does not require programmers to write explicit region parameters, lifetime
variables, linear region tokens, or substructural effect types.

---

## § 4 Analysis domains

**§ 4(1)** Sec 0.1 uses four cooperating effect-related domains:

```text
summary may-effects;
ordered arena-lifetime effects;
quantitative resource facts;
trust provenance.
```

**§ 4(2)** These domains must not be collapsed into one undifferentiated flag set.

---

## § 5 Summary may-effects

**§ 5(1)** Summary may-effects describe operations that may occur on at least one reachable
execution path.

**§ 5(2)** The initial internal set is:

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

**§ 5(3)** These effects are normally combined as an idempotent set.

**§ 5(4)** Example:

```text
{MayAllocate, MayIO}
∪
{MayIO, MayBlock}
=
{MayAllocate, MayIO, MayBlock}
```

**§ 5(5)** The same effect appearing multiple times remains one summary capability, while
the compiler may retain multiple causes for diagnostics.

---

## § 6 Ordered arena-lifetime effects

**§ 6(1)** Arena-lifetime effects describe ordered changes to programmer-visible arenas and
arena-backed storage.

**§ 6(2)** The initial conceptual events are:

```text
ArenaCreate(A, capacity)
ArenaAllocate(A, size)
ArenaReset(A)
ArenaRelease(A)
```

**§ 6(3)** Order matters.

**§ 6(4)** This sequence may be valid:

```text
ArenaCreate(A, 4096)
ArenaAllocate(A, 64)
ArenaReset(A)
ArenaAllocate(A, 128)
ArenaRelease(A)
```

**§ 6(5)** This sequence is invalid:

```text
ArenaCreate(A, 4096)
ArenaRelease(A)
ArenaAllocate(A, 64)
```

**§ 6(6)** An arena is the programmer-visible storage mechanism.

**§ 6(7)** The compiler may use an internal storage-lifetime identity to analyze the arena,
but Sec 0.1 does not expose that identity as source-level region syntax.

---

## § 7 Quantitative resource facts

**§ 7(1)** Quantitative analysis records facts such as:

```text
maximum live arena allocation;
remaining arena capacity;
unknown allocation bound;
maximum stack contribution;
maximum simultaneous task creation where provable;
path-specific resource peaks.
```

**§ 7(2)** These are not boolean effects.

**§ 7(3)** They are associated with control-flow paths, calls, loops, recursion, arenas,
and concrete compilation plans.

---

## § 8 Trust provenance

**§ 8(1)** Trust provenance records why the compiler accepts an effect claim or semantic
fact.

**§ 8(2)** Initial provenance categories include:

```text
proven from Sec source;
derived from a compiler intrinsic;
provided by target knowledge;
trusted through an explicit unsafe FFI declaration;
trusted through an inline assembly declaration;
unknown.
```

**§ 8(3)** Trust provenance is not a runtime effect.

**§ 8(4)** `unsafe` is not represented as `MayUnsafe`.

**§ 8(5)** Instead, the compiler records that a proof depends on an unsafe or trusted
boundary.

---

## § 9 Effects versus ordinary return values

**§ 9(1)** An explicit error value is not itself an effect.

**§ 9(2)** Example:

```sec
fn Open() Result[File, OpenError]
```

**§ 9(3)** The function may still have:

```text
MayAllocate
MayBlock
MayIO
```

**§ 9(4)** Returning `Err` does not imply panic, but it also does not remove allocation,
blocking, I/O, suspension, volatile access, or external mutation.

---

## § 10 Inferred summaries

**§ 10(1)** Every function, method, closure, destructor, and deferred body receives an
inferred effect summary.

**§ 10(2)** This applies even when no effect attribute is written.

**§ 10(3)** Example:

```sec
fn AddOne(value: int) Result[int, ArithmeticError] {
    let result := try value + 1
    return Ok(result)
}
```

**§ 10(4)** The compiler may infer:

```text
MayPanic: absent
MayAllocate: absent
MayBlock: absent
MaySuspend: absent
```

**§ 10(5)** The inferred summary describes the current implementation.

**§ 10(6)** It may be used for:

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

## § 11 Declared guarantees

**§ 11(1)** Attributes such as:

```text
@noPanic
@noAlloc
@noBlock
@interruptSafe
@isr
@interrupt
```

**§ 11(2)** declare stable guarantees.

**§ 11(3)** A declared guarantee is verified against the inferred summary and all other
required analyses.

**§ 11(4)** Example:

```sec
@noPanic
fn AddOne(value: int) Result[int, ArithmeticError] {
    let result := try value + 1
    return Ok(result)
}
```

**§ 11(5)** A future implementation change that introduces a reachable panic is a compiler
error.

---

## § 12 Public contract boundary

**§ 12(1)** Inferred absence of an effect is not automatically a stable public API promise.

**§ 12(2)** Public callers may rely on:

```text
declared guarantees;
guarantees required by an interface;
guarantees required by a function type or generic constraint;
compiler-known intrinsic guarantees;
target-knowledge guarantees where the target contract exposes them.
```

**§ 12(3)** Public callers must not rely on an undeclared implementation accident across a
separate compilation boundary.

**§ 12(4)** Module metadata may store both:

```text
inferred implementation summary;
declared public guarantee.
```

**§ 12(5)** They remain semantically distinct.

---

## § 13 Positive internal representation

**§ 13(1)** The compiler stores possible effects positively.

**§ 13(2)** Example:

```text
MayPanic
MayAllocate
MayBlock
```

**§ 13(3)** A negative source guarantee means the corresponding positive effect must be
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

## § 14 Effect ordering

**§ 14(1)** A function with fewer possible effects satisfies a stronger guarantee.

**§ 14(2)** Conceptually:

```text
{}
    stronger than

{MayAllocate}
    stronger than

{MayAllocate, MayBlock}
    stronger than

{MayPanic, MayAllocate, MayBlock}
```

**§ 14(3)** Effect compatibility uses subset inclusion.

**§ 14(4)** A function with effect set `E1` may be used where `E2` is permitted when:

```text
E1 is a subset of E2
```

---

## § 15 Direct effects

**§ 15(1)** A direct effect is introduced by the function body itself.

**§ 15(2)** Examples:

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

## § 16 Transitive effects

**§ 16(1)** A transitive effect is introduced by reachable behavior.

**§ 16(2)** The summary includes effects from:

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

**§ 16(3)** Conceptually:

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

## § 17 Call graph requirement

**§ 17(1)** Effect analysis operates over the active call graph for one concrete
compilation plan.

**§ 17(2)** The call graph must include:

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

**§ 17(3)** Unknown call targets are handled conservatively.

---

## § 18 Fixed-point analysis

**§ 18(1)** Recursive and mutually recursive functions require fixed-point analysis.

**§ 18(2)** Example:

```text
A calls B
B calls C
C calls A
```

**§ 18(3)** If `C` may allocate, the recursive strongly connected component is
transitively allocation-capable unless proof eliminates the path.

**§ 18(4)** The compiler computes a stable summary for the complete component.

---

## § 19 Path sensitivity

**§ 19(1)** An effect is included only when it remains reachable after semantic proof and
control-flow refinement.

**§ 19(2)** Example:

```sec
if false {
    PanicCapableOperation()
}
```

**§ 19(3)** The dead branch contributes no reachable effect.

**§ 19(4)** Example:

```sec
if divisor != 0 {
    let result := total / divisor
}
```

**§ 19(5)** The division-by-zero panic effect may be absent when Sema proves the divisor is
nonzero on the path.

**§ 19(6)** A runtime condition that may be either true or false contributes effects from
both reachable branches.

---

## § 20 Branch joins

**§ 20(1)** Summary may-effects join by union.

**§ 20(2)** Ordered arena state joins by state compatibility.

**§ 20(3)** Example:

```sec
if condition {
    arena.Release()
}

Use(arena)
```

**§ 20(4)** At the join point the arena is:

```text
Live on one path;
Released on another path.
```

**§ 20(5)** The later use is invalid.

**§ 20(6)** Example:

```sec
if condition {
    arena.Release()
    return
}

Use(arena)
```

**§ 20(7)** Only the path where the arena remains live reaches `Use`.

**§ 20(8)** The use may be valid.

---

## § 21 Loops

**§ 21(1)** Loop analysis computes a fixed point over:

```text
summary effects;
arena state;
borrow liveness;
allocation bounds;
resource peaks;
control-flow exits.
```

**§ 21(2)** Effects inside a reachable loop contribute to the containing function.

**§ 21(3)** Arena allocation may be bounded per iteration when reset or release is proven
before the next iteration.

**§ 21(4)** Example:

```sec
for item in items {
    let arena := Arena.FromBuffer(ref buffer)
    let value := arena.New[Value]()
    Process(item, ref value)
    arena.Reset()
}
```

**§ 21(5)** The analysis may prove one-iteration peak use rather than multiplying capacity
by iteration count.

---

## § 22 Optional optimization must not establish guarantees

**§ 22(1)** A declared effect guarantee must not depend only on an optional optimizer.

**§ 22(2)** A guarantee may be established through:

```text
semantic constant evaluation;
required dead-path proof;
mandatory escape analysis;
mandatory call-graph resolution;
type and contract proof;
required lowering semantics;
compiler-known intrinsic semantics.
```

**§ 22(3)** A guarantee must not be valid only because:

```text
release optimization removed an allocation;
debug mode happened to retain one;
inlining happened to remove a helper;
optional loop optimization changed the result.
```

**§ 22(4)** Debug and release builds must agree on semantic effect guarantees for the same
compilation plan and source semantics.

---

## § 23 Runtime checks

**§ 23(1)** Language-defined runtime checks contribute effects according to their selected
control flow.

**§ 23(2)** An ordinary checked operation may contribute `MayPanic`.

**§ 23(3)** Example:

```sec
let total := left + right
```

**§ 23(4)** When overflow is possible and not handled explicitly, the operation may panic.

**§ 23(5)** A proven-safe check contributes no panic effect.

---

## § 24 `try`

**§ 24(1)** `try` converts supported language-defined failure from panic-capable control
flow to explicit fallible control flow.

**§ 24(2)** Example:

```sec
let total := try left + right
```

**§ 24(3)** The arithmetic overflow check no longer contributes `MayPanic`.

**§ 24(4)** The explicit error flow remains part of the ordinary function result and
control flow.

---

## § 25 `try` does not remove unrelated effects

**§ 25(1)** Example:

```sec
let value := try allocator.New[Value]()
```

**§ 25(2)** Allocation failure may be explicit and panic-free, but:

```text
MayAllocate remains.
```

**§ 25(3)** Example:

```sec
let data := try network.Read()
```

**§ 25(4)** The explicit error path does not remove:

```text
MayIO
MayBlock
```

**§ 25(5)** where the operation has those effects.

**§ 25(6)** `try` does not catch or erase panic from arbitrary called functions.

---

## § 26 `@noPanic`

**§ 26(1)** `@noPanic` requires absence of `MayPanic`.

**§ 26(2)** The complete guarantee includes:

```text
no reachable explicit panic;
no reachable unchecked language-defined check failure;
no reachable panic-capable call;
no unproven assert;
no reachable checked unreachable;
no panic-capable cleanup;
no unknown foreign abort or unwind accepted as safe.
```

**§ 26(3)** The detailed panic semantics belong to:

```text
panic.md
runtime_checks.md
```

---

## § 27 `@noAlloc`

**§ 27(1)** `@noAlloc` requires absence of `MayAllocate`.

**§ 27(2)** Dynamic allocation includes:

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

## § 28 Operations that are not dynamic allocation

**§ 28(1)** The following are not dynamic allocation merely by existing:

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

**§ 28(2)** A specific operation may still allocate according to its semantic definition.

---

## § 29 Arena allocation and `@noAlloc`

**§ 29(1)** Creating an arena view over existing storage may be no-allocation.

**§ 29(2)** Example:

```sec
let arena := Arena.FromBuffer(ref buffer)
```

**§ 29(3)** Allocating an object from the arena consumes arena capacity.

**§ 29(4)** Example:

```sec
let value := arena.New[Value]()
```

**§ 29(5)** That operation contributes `MayAllocate`.

**§ 29(6)** Therefore, bounded arena allocation is still allocation.

**§ 29(7)** A separate profile may permit bounded arena allocation while forbidding heap
allocation, but that profile is not the meaning of `@noAlloc`.

---

## § 30 `@noBlock`

**§ 30(1)** `@noBlock` requires absence of synchronous blocking.

**§ 30(2)** Blocking includes:

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

## § 31 Asynchronous suspension

**§ 31(1)** Asynchronous suspension is represented separately as:

```text
MaySuspend
```

**§ 31(2)** `@noBlock` does not by itself forbid `MaySuspend`.

**§ 31(3)** An asynchronous task may suspend and allow other work to execute without
synchronously blocking an execution thread.

**§ 31(4)** Example:

```sec
@noBlock
async fn ReadAsync() Result[Data, ReadError] {
    return await device.Read()
}
```

**§ 31(5)** The function may be:

```text
MaySuspend
MayIO
```

**§ 31(6)** while remaining free from `MayBlock`.

**§ 31(7)** Exact `async` and `await` source semantics remain defined by their dedicated
rulebook.

---

## § 32 ISR suspension rule

**§ 32(1)** `@isr` and `@interruptSafe` forbid both:

```text
MayBlock
MaySuspend
```

**§ 32(2)** Interrupt execution may not block, park, yield, or resume later as an ordinary
task continuation.

---

## § 33 `MaySpawn`

**§ 33(1)** Creating concurrent work contributes `MaySpawn`.

**§ 33(2)** Examples:

```text
task creation;
thread creation;
detached work creation;
scheduler submission where it creates a new execution instance.
```

**§ 33(3)** The same operation may also contribute:

```text
MayAllocate;
MaySuspend;
MayMutateExternalState.
```

**§ 33(4)** `MaySpawn` remains separate because ISR, lifetime, supervision, embedded
profiles, and structured-concurrency analysis need to identify it directly.

---

## § 34 `MayIO`

**§ 34(1)** `MayIO` describes interaction with external input or output through an I/O
abstraction.

**§ 34(2)** Examples:

```text
file operations;
socket operations;
device APIs;
terminal operations;
foreign I/O;
platform services.
```

**§ 34(3)** I/O may be:

```text
blocking;
nonblocking;
suspending;
polling.
```

**§ 34(4)** `MayIO` is therefore not equivalent to `MayBlock`.

---

## § 35 `MayAccessVolatile`

**§ 35(1)** Volatile access contributes:

```text
MayAccessVolatile
```

**§ 35(2)** Examples:

```text
read from addressed storage;
write to addressed storage;
volatile register access;
compiler-known hardware register operations.
```

**§ 35(3)** Volatile operations may not be:

```text
removed;
duplicated;
merged unsafely;
reordered contrary to the volatile rules;
treated as ordinary constant memory.
```

**§ 35(4)** Volatile access is not automatically classified as ordinary file or socket I/O.

---

## § 36 `MayMutateExternalState`

**§ 36(1)** This effect describes mutation observable outside local computation.

**§ 36(2)** Examples:

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

**§ 36(3)** Local temporary mutation does not by itself contribute this effect.

**§ 36(4)** Example:

```sec
fn Normalize(value: int) int {
    let mut local := value
    // Local computation only.
    return local
}
```

---

## § 37 `MayUseNondeterministicInput`

**§ 37(1)** This effect describes dependence on input that may vary independently of the
ordinary explicit parameters.

**§ 37(2)** Examples:

```text
clock;
random source;
environment state;
process identity;
thread identity;
uncontrolled external device state;
foreign nondeterministic input.
```

**§ 37(3)** A volatile read may also be nondeterministic, but the effects remain separately
recorded.

---

## § 38 Composite interrupt contracts

**§ 38(1)** `@isr` is a composite verified contract.

**§ 38(2)** It requires at least:

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

**§ 38(3)** Stack and ABI facts come from other analyses.

**§ 38(4)** Effect analysis coordinates them.

---

## § 39 `@interrupt`

**§ 39(1)** `@interrupt(...)` implies `@isr`.

**§ 39(2)** Therefore:

```text
@interrupt(...)
    implies all @isr effect restrictions.
```

**§ 39(3)** Vector binding and target lowering remain defined by `attributes.md` and target
knowledge.

---

## § 40 `@interruptSafe`

**§ 40(1)** `@interruptSafe` verifies that a function or method may be called from ISR code.

**§ 40(2)** It requires at least:

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

**§ 40(3)** It does not bind a vector and does not make the function an ISR entrypoint.

---

## § 41 Target-aware ISR effects

**§ 41(1)** Not all I/O or external mutation is forbidden in an ISR.

**§ 41(2)** Target knowledge may classify operations such as:

```text
acknowledging an interrupt;
reading a status register;
writing a device command register;
clearing an interrupt flag;
performing a bounded lock-free queue operation.
```

**§ 41(3)** The compiler must use target and operation-specific knowledge rather than a
blanket rule that every volatile access is invalid.

**§ 41(4)** Unknown behavior remains conservative.

---

## § 42 Arena model

**§ 42(1)** Sec takes a limited arena-lifetime idea from region-effect research without
adopting a general region language.

**§ 42(2)** The programmer sees:

```text
arena;
arena allocation;
arena reset;
arena release;
references to arena-backed storage.
```

**§ 42(3)** The compiler may assign internal lifetime identities.

**§ 42(4)** Those identities are not normally visible in source.

---

## § 43 Arena state

**§ 43(1)** The initial arena state model is:

```text
Live
Released
```

**§ 43(2)** `ArenaReset` is an event that:

```text
keeps the arena Live;
restores capacity according to the arena definition;
invalidates all previous allocations from that arena;
invalidates references depending on those allocations.
```

**§ 43(3)** `ArenaRelease`:

```text
ends ownership of the arena storage;
changes the arena to Released;
invalidates all dependent allocations and references;
forbids later allocation, reset, or ordinary use.
```

---

## § 44 Arena-backed references

**§ 44(1)** The compiler records when a value or reference depends on arena-backed storage.

**§ 44(2)** Conceptual facts:

```text
StorageDependsOn(value, A)
ReferenceDependsOn(reference, A)
```

**§ 44(3)** Example invalid program:

```sec
let arena := Arena.FromBuffer(ref buffer)
let value := arena.New[Value]()
let view := ref value

arena.Reset()

Use(view)
```

**§ 44(4)** `view` depends on an allocation invalidated by `ArenaReset`.

---

## § 45 Non-lexical borrow live ranges

**§ 45(1)** A borrow may end at its last possible use rather than at the end of the lexical
block.

**§ 45(2)** Example:

```sec
let value := arena.New[Value]()
let view := ref value

Consume(view)

// The borrow is no longer live.
arena.Reset()
```

**§ 45(3)** The reset may be valid when the compiler proves that `view` is not used later.

**§ 45(4)** The source variable may remain in lexical scope while its borrow live range has
ended.

---

## § 46 Arena lifetime is not a source-code block

**§ 46(1)** The term storage lifetime does not mean a lexical source block.

**§ 46(2)** An arena may:

```text
be created after block entry;
remain live across nested blocks;
be reset several times;
be released before block exit;
be passed through calls;
have references whose live ranges end before the arena.
```

**§ 46(3)** Sec source does not require explicit region parameters for this analysis.

---

## § 47 Arena analysis does not replace ownership

**§ 47(1)** Ownership analysis determines:

```text
who owns the arena;
whether the arena may move;
whether an owner remains valid;
whether release occurs exactly according to ownership rules.
```

**§ 47(2)** Borrow analysis determines:

```text
shared and mutable borrow conflicts;
borrow live ranges;
field separation;
alias validity.
```

**§ 47(3)** Escape analysis determines:

```text
whether references outlive arena-backed storage;
whether closures retain arena references;
whether returned references are valid.
```

**§ 47(4)** Effect analysis tracks ordered arena events and coordinates these facts.

---

## § 48 Arena capacity

**§ 48(1)** Arena allocation may carry a compile-time or symbolic size.

**§ 48(2)** Conceptual:

```text
ArenaAllocate(A, size)
```

**§ 48(3)** The compiler may compute:

```text
remaining capacity;
maximum live allocation;
per-path capacity use;
unknown bound.
```

**§ 48(4)** Allocation fails or becomes fallible according to the arena API and allocation
rulebook.

**§ 48(5)** `try` may make capacity failure explicit, but `MayAllocate` remains.

---

## § 49 Arena reset in loops

**§ 49(1)** A reset before the next iteration may allow bounded peak analysis.

**§ 49(2)** Example:

```sec
for item in items {
    let value := arena.New[Value]()
    Process(item, ref value)
    arena.Reset()
}
```

**§ 49(3)** When no arena-backed value escapes and all borrows end before reset, the
compiler may report one-iteration peak capacity rather than accumulated use.

---

## § 50 Arena release across branches

**§ 50(1)** Example:

```sec
if condition {
    arena.Release()
}

Use(arena)
```

**§ 50(2)** The use is rejected because at least one reaching path has released the arena.

**§ 50(3)** The compiler must report the release path.

---

## § 51 Arena splitting

**§ 51(1)** Sub-arenas and explicit arena splitting are not part of this rulebook unless
their source and ownership semantics are separately decided.

**§ 51(2)** The analysis architecture must permit a future event such as:

```text
ArenaSplit(parent, child, capacity)
```

**§ 51(3)** but Sec 0.1 does not gain source syntax merely because the internal model can
represent it.

---

## § 52 Function summaries for arenas

**§ 52(1)** A function summary may include facts such as:

```text
requires arena A live;
allocates at most N bytes from A;
may reset A;
releases A;
returns storage depending on A;
does not retain a reference to A;
does not change external arena liveness.
```

**§ 52(2)** These facts may be inferred and stored in module metadata.

**§ 52(3)** They are not necessarily written as explicit region syntax.

---

## § 53 Local arena activity

**§ 53(1)** A function may create, use, and release a private arena without exposing the
arena lifetime to callers.

**§ 53(2)** The caller summary may still include:

```text
MayAllocate;
maximum temporary allocation.
```

**§ 53(3)** It need not expose an external arena-state transition when no arena-backed value
escapes.

---

## § 54 Ownership transfer effects

**§ 54(1)** Move and invalidation remain governed by `copy_move.md` and `ownership.md`.

**§ 54(2)** Effect analysis may consume ordered facts such as:

```text
owner moved;
source invalidated;
destination initialized.
```

**§ 54(3)** These facts are not part of the public summary may-effect set.

**§ 54(4)** They are control-flow and value-validity facts used by cooperating analyses.

---

## § 55 Destructors

**§ 55(1)** The effective summary of a function includes destructors that may run on
reachable exits.

**§ 55(2)** Destructors are already required to satisfy the panic rules defined elsewhere.

**§ 55(3)** A destructor may still contribute other effects unless prohibited.

**§ 55(4)** Possible destructor effects include:

```text
MayAllocate;
MayBlock;
MayIO;
MayAccessVolatile;
MayMutateExternalState.
```

**§ 55(5)** A containing guarantee must include or forbid them accordingly.

---

## § 56 `defer`

**§ 56(1)** A deferred body contributes effects to every exit path where it runs.

**§ 56(2)** Example:

```sec
@noBlock
fn Work() void {
    defer {
        BlockingClose()
    }
}
```

**§ 56(3)** The function violates `@noBlock`.

**§ 56(4)** The violation occurs even when the explicit statements before scope exit do not
block.

---

## § 57 Cleanup paths

**§ 57(1)** Effect analysis includes cleanup on:

```text
normal return;
early return;
explicit error propagation;
scope exit;
task cancellation where cleanup is defined;
contained panic cleanup where the panic rulebook requires it.
```

**§ 57(2)** A cleanup path may have stricter panic rules than an ordinary path.

---

## § 58 Closures

**§ 58(1)** A closure summary includes:

```text
body effects;
capture creation effects;
capture destruction effects;
environment allocation;
effects of values retained by the closure.
```

**§ 58(2)** A closure that requires heap storage contributes `MayAllocate`.

**§ 58(3)** A closure whose environment is stack-proven may avoid that effect.

**§ 58(4)** The proof must not depend only on optional optimization.

---

## § 59 Function values and indirect calls

**§ 59(1)** Effect guarantees are semantically part of function compatibility.

**§ 59(2)** Exact source syntax for effect-constrained function types may be defined later.

**§ 59(3)** The semantic rule is fixed:

```text
a function with fewer effects may be used where more effects are permitted;
a function with more effects may not be used where fewer are required.
```

**§ 59(4)** An indirect call with unknown effect guarantees is conservative.

---

## § 60 Callback example

**§ 60(1)** Conceptual:

```sec
@noPanic
fn Execute(callback: fn() void) void {
    callback()
}
```

**§ 60(2)** The function cannot be verified as `@noPanic` unless the callback contract or
resolved target proves absence of `MayPanic`.

**§ 60(3)** The compiler must not assume a callback is panic-free from its current observed
uses when the public type permits future unknown values.

---

## § 61 Interfaces

**§ 61(1)** An interface method may declare effect guarantees.

**§ 61(2)** Every implementation must satisfy them.

**§ 61(3)** An implementation may provide a stronger guarantee.

**§ 61(4)** Example:

```text
interface method permits allocation;
implementation is noAlloc;
valid.
```

**§ 61(5)** Invalid:

```text
interface method is noAlloc;
implementation may allocate.
```

**§ 61(6)** Effect compatibility uses subset inclusion.

---

## § 62 Open and closed worlds

**§ 62(1)** For a complete executable build, the compiler may know every concrete dispatch
target.

**§ 62(2)** For a public library or extensible interface boundary, future implementations
may be unknown.

**§ 62(3)** The compiler must use declared public effect contracts across open boundaries.

**§ 62(4)** It must not convert an accidental property of currently known implementations
into an undeclared stable guarantee.

---

## § 63 Generics

**§ 63(1)** Generic effect summaries may depend on:

```text
operations available on T;
copy, move, and destruction behavior of T;
callback effect constraints;
selected generic implementations;
specialization;
target-specific implementations.
```

**§ 63(2)** A summary may remain symbolic until specialization.

**§ 63(3)** Conceptual:

```text
Effects(Apply[T, F]) includes Effects(F)
```

**§ 63(4)** Exact effect-constraint syntax for generics is deferred.

**§ 63(5)** Unknown generic effects are conservative.

---

## § 64 Specialization

**§ 64(1)** A concrete specialization may prove fewer effects than the generic maximum.

**§ 64(2)** Example:

```text
generic checked arithmetic may panic;
a constrained specialization proves overflow impossible.
```

**§ 64(3)** The compiler may use the stronger inferred specialization summary internally.

**§ 64(4)** Public guarantees remain governed by the generic declaration and applicable
constraints.

---

## § 65 Foreign functions

**§ 65(1)** Unknown foreign code is conservative.

**§ 65(2)** Unless explicitly and safely described by the FFI model, it may contribute:

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

**§ 65(3)** A normal Sec effect attribute on an extern declaration is not silently treated
as proof.

**§ 65(4)** Foreign effect declarations require an explicit unsafe trust model.

---

## § 66 Trusted foreign summaries

**§ 66(1)** A future FFI rule may permit an unsafe declaration of foreign effects.

**§ 66(2)** The compiler must retain provenance:

```text
effect absence trusted from foreign declaration
```

**§ 66(3)** Diagnostics may say:

```text
ISR safety depends on trusted foreign declaration `ReadStatusRegister`
```

**§ 66(4)** The compiler must not report the fact as proven from Sec source.

---

## § 67 Inline assembly

**§ 67(1)** Inline assembly requires explicit effect classification.

**§ 67(2)** The compiler cannot infer arbitrary machine-code behavior from text alone.

**§ 67(3)** An inline assembly declaration may need to describe:

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

**§ 67(4)** Accepted claims carry inline-assembly trust provenance.

---

## § 68 Compiler intrinsics

**§ 68(1)** Compiler intrinsics have compiler-known summaries.

**§ 68(2)** Examples may include:

```text
volatile load;
volatile store;
target trap;
nonblocking register operation;
allocation primitive;
scheduler primitive.
```

**§ 68(3)** The intrinsic definition is the source of truth.

---

## § 69 Control-flow facts

**§ 69(1)** Some compiler-tracked control-flow properties are not initial public effects.

**§ 69(2)** Internal facts may include:

```text
MayTerminateProcess
MayTrap
MayNotReturn
```

**§ 69(3)** These are distinct from `MayPanic`.

**§ 69(4)** A function may be `@noPanic` and still:

```text
exit the process;
trap by explicit target operation;
run forever;
never return by design.
```

**§ 69(5)** The relevant entrypoint, panic, and control-flow rulebooks govern legality.

---

## § 70 Target and configuration selection

**§ 70(1)** Effect analysis runs after:

```text
file-level @target selection;
statement-level @target selection;
@when configuration selection;
target knowledge selection;
implementation-variant selection.
```

**§ 70(2)** Only active source contributes to the active call graph and effect summary.

**§ 70(3)** Excluded source remains parsed but does not contribute effects for the current
compilation plan.

---

## § 71 Per-compilation-plan analysis

**§ 71(1)** Every concrete compilation plan receives its own effect analysis.

**§ 71(2)** A logical function may have different implementations per:

```text
OS;
architecture;
CPU;
device;
board;
build configuration.
```

**§ 71(3)** A declared public guarantee must hold in every active variant where the
declaration promises that guarantee.

**§ 71(4)** Cross-variant validation must report inconsistent guarantees.

---

## § 72 Knowledge packs

**§ 72(1)** Target knowledge packs may provide effect and execution facts for:

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

**§ 72(2)** Knowledge-pack facts carry target-knowledge provenance.

**§ 72(3)** Unknown target behavior remains conservative.

---

## § 73 Semantic IR requirements

**§ 73(1)** Semantic IR must represent effect-relevant operations explicitly.

**§ 73(2)** It must not hide important behavior inside opaque generic nodes.

**§ 73(3)** At minimum, the IR must make visible:

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

## § 74 Function summary representation

**§ 74(1)** Conceptual compiler structure:

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

**§ 74(2)** This is illustrative, not mandatory implementation syntax.

**§ 74(3)** Arena state and quantitative facts should use separate structures.

---

## § 75 Effect causes

**§ 75(1)** An effect summary must retain causes.

**§ 75(2)** Conceptual:

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

**§ 75(3)** A boolean summary alone is insufficient for diagnostics.

---

## § 76 Arena analysis representation

**§ 76(1)** Conceptual:

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

**§ 76(2)** `Generation` may model invalidation across resets internally.

**§ 76(3)** This is an implementation option, not source semantics.

---

## § 77 Reset generation model

**§ 77(1)** One practical implementation may increment an arena generation at reset.

**§ 77(2)** Arena-backed values and references retain the generation in which they were
created as compiler metadata.

**§ 77(3)** After reset:

```text
arena generation changes;
old dependent values are invalid;
new allocations use the new generation.
```

**§ 77(4)** The generated program does not require runtime generation checks when static
analysis proves validity.

**§ 77(5)** A target profile may separately choose generational runtime references, but that
is not required by this effect analysis.

---

## § 78 No mandatory runtime

**§ 78(1)** Effect analysis is compile-time.

**§ 78(2)** It must not require:

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

**§ 78(3)** Compiler metadata may be discarded after compilation unless needed for separate
compilation, debugging, LSP, or diagnostics.

---

## § 79 Dead support elimination

**§ 79(1)** When effect analysis proves support is unused, the binary may omit it.

**§ 79(2)** Examples:

```text
panic-free binary omits panic endpoint;
no managed tasks means no scheduler support;
no allocation means no allocator support;
no volatile access means no volatile helper;
no arena use means no arena helper.
```

**§ 79(3)** Direct target instructions or inline lowering may be used without introducing a
general runtime.

---

## § 80 Diagnostics

**§ 80(1)** Diagnostics must identify:

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

## § 81 Panic diagnostic example

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

## § 82 Allocation diagnostic example

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

## § 83 Blocking diagnostic example

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

## § 84 Suspension diagnostic example

```text
error: ISR function `TimerHandler` may suspend

TimerHandler
  -> calls WaitForTask
  -> await may suspend the current task

@isr forbids both blocking and suspension
```

---

## § 85 Arena invalidation diagnostic example

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

## § 86 Trust diagnostic example

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

## § 87 Multiple causes

**§ 87(1)** When several causes exist, diagnostics may summarize:

```text
MayPanic
    3 causes

MayAllocate
    2 causes

MayBlock
    1 cause
```

**§ 87(2)** The LSP should permit navigation to each cause.

---

## § 88 LSP behavior

**§ 88(1)** The LSP should display:

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

**§ 88(2)** Example hover:

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

## § 89 Function compatibility diagnostics

**§ 89(1)** When a function value or implementation has too many effects, report the
specific difference.

**§ 89(2)** Example:

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

## § 90 Implementation architecture

**§ 90(1)** A recommended pipeline is:

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

**§ 90(2)** Some stages may iterate to a fixed point.

---

## § 91 Effect registry

**§ 91(1)** The compiler should define a central registry for effect kinds.

**§ 91(2)** Each effect entry should identify:

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

**§ 91(3)** Do not distribute effect meaning across unrelated ad hoc checks.

---

## § 92 Separate compilation

**§ 92(1)** Compiled module metadata should contain enough information to verify callers
without re-reading the source implementation.

**§ 92(2)** At minimum:

```text
declared public guarantees;
required callback or interface guarantees;
conservative public effect summary where needed;
generic effect dependencies;
trusted provenance at public boundaries;
arena escape and lifetime requirements that cross the API.
```

**§ 92(3)** Private inferred details need not become public contract.

---

## § 93 Versioning

**§ 93(1)** Changing a declared public effect guarantee may be an API compatibility change.

**§ 93(2)** Examples:

```text
removing @noPanic weakens the API;
removing @noAlloc weakens the API;
adding MayBlock to a previously @noBlock function violates the contract;
strengthening an implementation while keeping the same declaration is safe.
```

**§ 93(3)** The project and ABI rulebooks determine exact compatibility policy.

---

## § 94 Effect inference and diagnostics mode

**§ 94(1)** The compiler may report inferred effects without requiring attributes.

**§ 94(2)** Examples:

```text
information: function is inferred noPanic, noAlloc, and noBlock
```

**§ 94(3)** Such reports must not silently modify source.

**§ 94(4)** An explicit semantic tooling command may propose attributes when the guarantee
is proven across the intended build variants.

**§ 94(5)** Normal formatting must not add effect attributes.

---

## § 95 Current language-design status

**§ 95(1)** The following semantics are decided:

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

## § 96 Source syntax deliberately deferred

**§ 96(1)** This rulebook does not decide:

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

**§ 96(2)** The semantics are prepared for those features without inventing source forms.

---

## § 97 Initial implementation scope

**§ 97(1)** The initial implementation should prioritize:

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

**§ 97(2)** Additional summary effects may be implemented incrementally.

---

## § 98 Required tests

**§ 98(1)** Create or update:

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

## § 99 Summary-effect tests

**§ 99(1)** Test:

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

## § 100 `try` tests

**§ 100(1)** Test that:

```text
try arithmetic removes MayPanic from arithmetic failure;
try bounds handling removes the handled panic path;
try allocation retains MayAllocate;
try blocking I/O retains MayBlock and MayIO;
try suspending operation retains MaySuspend;
try does not catch panic from arbitrary called functions.
```

---

## § 101 `@noAlloc` tests

**§ 101(1)** Test:

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

## § 102 `@noBlock` tests

**§ 102(1)** Test:

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

## § 103 ISR tests

**§ 103(1)** Test:

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

## § 104 Arena tests

**§ 104(1)** Test:

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

## § 105 Compatibility tests

**§ 105(1)** Test:

```text
fewer-effect implementation satisfies broader contract;
more-effect implementation rejected for narrower contract;
interface guarantee enforced;
unknown callback conservative;
resolved noPanic callback accepted;
public undeclared inferred absence not treated as stable contract.
```

---

## § 106 Binary tests

**§ 106(1)** Verify:

```text
effect analysis introduces no runtime support;
panic-free build may omit panic endpoint;
allocation-free build may omit allocator support;
no task use may omit scheduler support;
arena analysis does not require runtime generation metadata;
target-specific inactive effects do not reach object code.
```

---

## § 107 Diagnostics tests

**§ 107(1)** Diagnostics must test:

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

## § 108 Required synchronization

**§ 108(1)** This rulebook must remain synchronized with:

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
projects.md
formatter.md
lsp.md
diagnostics rulebook
language-rulebook-status.md
```

---

## Appendix A — Codex implementation plan

### Appendix A.1 Add the rulebook

**Appendix A.1(1)** Add:

```text
rules/analysis/effect_analysis.md
```

**Appendix A.1(2)** Update:

```text
language-rulebook-status.md
governance/ (owning integration entries)
```

**Appendix A.1(3)** Mark the rulebook Written.

**Appendix A.1(4)** Do not mark all effect analysis implemented.

---

### Appendix A.2 Inventory existing effect logic

**Appendix A.2(1)** Locate current logic for:

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

**Appendix A.2(2)** Do not create parallel analyses when existing compiler facts can be unified.

---

### Appendix A.3 Add central effect kinds

**Appendix A.3(1)** Create stable internal identities for:

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

**Appendix A.3(2)** Keep ordered arena events separate.

---

### Appendix A.4 Add cause tracking

**Appendix A.4(1)** Every introduced effect must retain:

```text
effect kind;
source location;
operation;
callee;
parent cause;
provenance;
active compilation plan.
```

**Appendix A.4(2)** Do not keep only booleans.

---

### Appendix A.5 Mark direct Semantic IR effects

**Appendix A.5(1)** Annotate or classify Semantic IR nodes for:

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

### Appendix A.6 Build direct summaries

**Appendix A.6(1)** Compute direct summary effects per function, method, closure, destructor, and
defer body.

**Appendix A.6(2)** Include implicit operations.

---

### Appendix A.7 Propagate through call graph

**Appendix A.7(1)** Compute transitive summaries.

**Appendix A.7(2)** Use strongly connected components for recursion.

**Appendix A.7(3)** Retain shortest or clearest cause chains and all alternative roots needed for
diagnostics.

---

### Appendix A.8 Verify declared guarantees

**Appendix A.8(1)** Integrate with attributes:

```text
@noPanic
@noAlloc
@noBlock
@interruptSafe
@isr
@interrupt
```

**Appendix A.8(2)** Compute explicit and implied guarantees separately.

---

### Appendix A.9 Implement suspension distinction

**Appendix A.9(1)** Do not classify every `await` as synchronous blocking.

**Appendix A.9(2)** Add `MaySuspend`.

**Appendix A.9(3)** Verify:

```text
@noBlock permits MaySuspend;
@isr rejects MaySuspend;
@interruptSafe rejects MaySuspend.
```

---

### Appendix A.10 Implement arena dataflow

**Appendix A.10(1)** Track:

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

**Appendix A.10(2)** Reject use after reset or release.

---

### Appendix A.11 Integrate non-lexical borrow liveness

**Appendix A.11(1)** Use last-use and control-flow information so a borrow may end before lexical
scope exit.

**Appendix A.11(2)** Do not require explicit source regions.

---

### Appendix A.12 Quantitative allocation

**Appendix A.12(1)** Compute where possible:

```text
current arena use;
maximum live arena use;
unknown bounds;
loop peak rather than naive accumulated total.
```

**Appendix A.12(2)** Keep quantitative failure separate from boolean `MayAllocate`.

---

### Appendix A.13 Foreign and assembly provenance

**Appendix A.13(1)** Treat unknown effects conservatively.

**Appendix A.13(2)** Prepare representation for future explicit unsafe effect declarations.

**Appendix A.13(3)** Do not invent their syntax.

---

### Appendix A.14 Interfaces and function compatibility

**Appendix A.14(1)** Use effect subset rules for:

```text
interface implementation;
function values;
callbacks;
method replacement;
generic constraints when available.
```

**Appendix A.14(2)** Do not infer public callback guarantees from only current call sites.

---

### Appendix A.15 Module metadata

**Appendix A.15(1)** Emit declared guarantees and required public effect information.

**Appendix A.15(2)** Keep private inferred implementation facts distinct.

---

### Appendix A.16 LSP

**Appendix A.16(1)** Add:

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

### Appendix A.17 Testing

**Appendix A.17(1)** Run:

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

**Appendix A.17(2)** Do not claim complete implementation while any required effect category is only
partially modeled.

---

## § 109 Current implementation status

**§ 109(1)** Implemented foundations now include:

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

**§ 109(2)** This remains partial. The repository does not yet claim the complete canonical
effect set, guarantee verification, ambient allocation-context resolution,
path-aware ordered Arena state merging, indirect/open-target propagation,
per-`CompilationPlan` summaries, or general diagnostic cause paths.

---

## § 110 Shaped operation effects

**§ 110(1)** Logical shaped operations do not contribute `MayAllocate` merely because later
materialization could require storage. This applies to elementwise shaped
arithmetic, `x`, `Dot`, `Outer`, `Magnitude`, `Normalize`, `Cross`, `Contract`,
`Reshape`, `ToShape`, `Permute`, `Transpose`, and `BroadcastTo`.

**§ 110(2)** Returning an explicit `ShapeError` after runtime validation is ordinary error
flow and is not itself an effect.

**§ 110(3)** `Create(request)`, `Materialize(request)`, `TransferTo(request)`, and
`Relayout(...)` contribute the actual effects of their selected provider or
helper path. Those concrete effects may include `MayAllocate`, `MayBlock`,
`MaySuspend`, `MayIO`, `MayAccessVolatile`, or `MayMutateExternalState`; being a
tensor or transfer operation does not create a separate public effect.

**§ 110(4)** A synchronous `TransferTo` must not hide outstanding asynchronous work. An
explicit asynchronous transfer contributes the effects and lifetime/publication
obligations of its actual contract.

---

## § 111 Thread-local first-access effects

**§ 111(1)** A potentially first `ThreadLocal[T].Borrow()`, `BorrowMut()`, or `Replace()`
access contributes the selected initializer's effects. The copyable-template
form contributes only ordinary implicit-copy behavior; a factory-backed key
contributes the factory's inferred allocation, blocking, cancellation, panic,
I/O, and other effects.

**§ 111(2)** The attached-thread-context requirement propagates through the same call graph
as other inferred requirements. Analysis may remove factory effects only after
proving that the key is already initialized for the current physical-thread
attachment generation on every reaching path.

---

## Appendix B — Canonical effect table

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

## Appendix C — Canonical arena event table

| Arena event | Meaning |
|---|---|
| `ArenaCreate(A, capacity)` | create or establish a live arena over storage |
| `ArenaAllocate(A, size)` | consume capacity and create arena-backed storage |
| `ArenaReset(A)` | invalidate prior allocations while keeping the arena live |
| `ArenaRelease(A)` | end the arena and invalidate all dependent storage |

---

## § 112 Final canonical summary

**§ 112(1)** Sec performs effect analysis at compile time.

**§ 112(2)** Every function receives an inferred effect summary.

**§ 112(3)** Declared effect attributes are stable verified guarantees.

**§ 112(4)** Public callers rely on declared guarantees, not accidental implementation facts.

**§ 112(5)** The initial summary effects include:

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

**§ 112(6)** `@noPanic`, `@noAlloc`, and `@noBlock` require absence of their corresponding
effects.

**§ 112(7)** `MayBlock` and `MaySuspend` are distinct.

**§ 112(8)** `@noBlock` forbids synchronous blocking but permits asynchronous suspension.

**§ 112(9)** `@isr` and `@interruptSafe` forbid both blocking and suspension.

**§ 112(10)** `try` removes only the supported panic-capable failure path.

**§ 112(11)** It does not remove allocation, blocking, suspension, I/O, or other effects.

**§ 112(12)** Arena allocation contributes `MayAllocate`.

**§ 112(13)** Arena reset invalidates previous arena-backed storage and references.

**§ 112(14)** Arena release ends the arena and forbids later use.

**§ 112(15)** Borrow live ranges may end non-lexically at last use.

**§ 112(16)** Sec uses this limited ordered arena analysis without exposing a general region
type system.

**§ 112(17)** Effect compatibility uses subset inclusion.

**§ 112(18)** Unknown indirect, foreign, assembly, and target behavior is conservative.

**§ 112(19)** Effect analysis includes defer, destructors, implicit operations, generics,
closures, callbacks, interfaces, and target-selected implementations.

**§ 112(20)** Concrete generic specialization may refine type-dependent effect facts but must
not weaken, duplicate, eliminate, or reorder observable effects. The
concretization boundary is owned by `rules/compiler/generics_lowering.md`; this
rulebook remains authoritative for effect meaning and compatibility.

**§ 112(21)** Effect analysis is performed per concrete compilation plan.

**§ 112(22)** Effect analysis introduces no mandatory runtime.

**§ 112(23)** The canonical effect facts defined here are also consumed by semantic CTE.
`rules/compiler/compile_time_evaluation.md` decides whether an effect on the
concretely executed path is permitted; it does not create a competing effect
taxonomy. A forbidden effect in an unexecuted branch does not by itself
invalidate path-sensitive evaluation.
