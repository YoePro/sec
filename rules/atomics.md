# Atomics

## Purpose

Atomics provide synchronized access to small values without using a mutex.

Atomic operations are indivisible with respect to other atomic operations on the
same storage location.

Atomics are intended for values such as:

- flags
- counters
- state markers
- generation values
- reference counters
- sequence numbers
- lock-free coordination primitives

Atomics are not a general replacement for `Mutex[T]`.

---

## Type classification

Atomic types are compiler-known types.

The initial generic form is:

```sec
Atomic[T]
```

Examples:

```sec
Atomic[bool]
Atomic[int]
Atomic[uint64]
Atomic[RawPtr[Node]]
```

The compiler, language server and backend must understand atomic semantics
directly.

`Atomic[T]` is handled similarly to compiler-known generic types such as:

```sec
Result[T, E]
Option[T]
Task[T]
Mutex[T]
```

but has distinct storage, operation and memory-order rules.

---

## Supported value types

Version 0.1 should support atomic storage only for types that the selected target
can operate on atomically.

The initial portable set should include:

```sec
bool
byte
int8
int16
int32
int64
uint8
uint16
uint32
uint64
RawPtr[T]
```

Support for the following depends on target capability:

```sec
int
uint
i128
u128
enum integer representations
register-sized named types
```

Floating-point and decimal types are not atomic in version 0.1.

Structured values are not atomic merely because their total size matches a
machine word.

Invalid examples:

```sec
Atomic[ApplicationState]
Atomic[string]
Atomic[decimal]
Atomic[float64]
Atomic[int[]]
```

Expected diagnostic:

```text
type ApplicationState is not supported by Atomic
```

---

## Target support

Atomic support depends on:

- value size
- alignment
- target architecture
- target profile
- required operation
- required memory order

A target may support atomic load and store for a type without supporting all
read-modify-write operations.

The compiler must not silently lower an unsupported atomic operation to
unsynchronized ordinary access.

A target profile may:

- use native atomic instructions
- use a compiler-provided atomic intrinsic
- use a verified platform primitive
- reject the operation

A hidden global mutex fallback is not permitted unless the target profile
explicitly declares that implementation and preserves interrupt and blocking
semantics.

---

## Construction

An atomic value is initialized with one ordinary value of type `T`.

Example:

```sec
let ready: Atomic[bool] := Atomic(false)
```

Static atomic storage is valid:

```sec
static let Requests: Atomic[uint64] := Atomic(0)
```

The atomic binding is normally immutable.

The contained value changes through atomic operations.

Preferred:

```sec
static let Counter: Atomic[uint64] := Atomic(0)
```

Usually unnecessary:

```sec
static let mut Counter: Atomic[uint64] := Atomic(0)
```

Replacing the atomic object is different from atomically changing its contained
value.

---

## Ownership

`Atomic[T]` owns exactly one atomic storage location containing a value of type
`T`.

The protected value is not directly accessible through ordinary loads or stores.

Invalid conceptual access:

```sec
Counter.value
```

All access must use atomic operations.

The ownership of the `Atomic[T]` object follows ordinary Sec ownership rules.

Concurrent access may occur through valid shared references to the same atomic
storage.

---

## Canonical operations

Version 0.1 should provide at least:

```sec
load()
store(value)
swap(value)
compareExchange(expected, desired)
```

Integer atomics should additionally support:

```sec
fetchAdd(value)
fetchSub(value)
fetchAnd(value)
fetchOr(value)
fetchXor(value)
```

Boolean atomics may support:

```sec
fetchAnd(value)
fetchOr(value)
fetchXor(value)
```

Pointer atomics should initially support:

```sec
load()
store(value)
swap(value)
compareExchange(expected, desired)
```

---

## Load

Atomic load reads the current value.

```sec
let value := Counter.load()
```

The result type is `T`.

Example:

```sec
let ready: bool := Ready.load()
```

A load does not modify the atomic value.

---

## Store

Atomic store replaces the contained value.

```sec
Ready.store(true)
```

The argument must be compatible with `T`.

A store does not return the previous value.

Use `swap()` when the previous value is required.

---

## Swap

Atomic swap replaces the value and returns the previous value.

```sec
let previous := State.swap(newState)
```

The return type is `T`.

The complete exchange is one atomic read-modify-write operation.

---

## Fetch operations

Fetch operations atomically update the stored value and return the previous
value.

Example:

```sec
let previous := Requests.fetchAdd(1)
```

If `Requests` contained `10`, the operation:

- stores `11`
- returns `10`

The operation must use the arithmetic and bit-width semantics of `T`.

Atomic arithmetic must not silently use different overflow rules from ordinary
Sec arithmetic.

The exact overflow behavior must follow the numeric operation selected for the
atomic API.

---

## Compare and exchange

Compare-and-exchange conditionally replaces the value.

Conceptual form:

```sec
let result := State.compareExchange(expected, desired)
```

The operation:

1. atomically reads the current value
2. compares it with `expected`
3. stores `desired` when they are equal
4. reports whether the exchange occurred
5. exposes the observed value

A suitable compiler-known result type is:

```sec
CompareExchangeResult[T]
```

Conceptually:

```sec
type CompareExchangeResult[T] struct {
    exchanged: bool
    observed: T
}
```

Example:

```sec
let result := State.compareExchange(State.Idle, State.Running)

if result.exchanged {
    StartWork()
}
```

When the exchange fails, `result.observed` contains the value that prevented the
exchange.

This avoids requiring an in-out `expected` parameter.

---

## Weak compare and exchange

Version 0.1 should provide only strong compare-and-exchange.

A future operation may expose spurious-failure semantics:

```sec
compareExchangeWeak(expected, desired)
```

This is not required for version 0.1.

---

## Memory order

Every atomic operation has a memory order.

The default operation should use the safest general ordering:

```text
sequentially consistent
```

Examples:

```sec
Ready.load()
Ready.store(true)
Counter.fetchAdd(1)
```

Explicit memory order may be provided through overloads:

```sec
Ready.load(MemoryOrder.Acquire)
Ready.store(true, MemoryOrder.Release)
Counter.fetchAdd(1, MemoryOrder.Relaxed)
```

The exact ordering semantics are defined in
`concurrency_memory_model.txt`.

---

## MemoryOrder

`MemoryOrder` is a compiler-known enum or equivalent language-defined type.

Version 0.1 should define:

```sec
MemoryOrder.Relaxed
MemoryOrder.Acquire
MemoryOrder.Release
MemoryOrder.AcqRel
MemoryOrder.SeqCst
```

Not every order is valid for every operation.

Examples:

```text
load
    Relaxed
    Acquire
    SeqCst

store
    Relaxed
    Release
    SeqCst

read-modify-write
    Relaxed
    Acquire
    Release
    AcqRel
    SeqCst
```

Invalid order combinations are semantic errors.

Example:

```sec
Ready.load(MemoryOrder.Release)
```

Expected diagnostic:

```text
MemoryOrder.Release is not valid for atomic load
```

---

## Default ordering

Omitting the memory order is equivalent to:

```sec
MemoryOrder.SeqCst
```

This default prioritizes correctness and clarity.

Programmers may request weaker ordering only when they intentionally need it.

The formatter must not insert explicit `SeqCst` arguments when they were omitted.

---

## Compare-exchange ordering

Compare-and-exchange may require:

- one order for success
- one order for failure

The simple overload should use sequential consistency:

```sec
State.compareExchange(expected, desired)
```

An explicit overload may use:

```sec
State.compareExchange(
    expected,
    desired,
    MemoryOrder.AcqRel,
    MemoryOrder.Acquire
)
```

The failure order must not include release semantics.

Invalid:

```sec
State.compareExchange(
    expected,
    desired,
    MemoryOrder.AcqRel,
    MemoryOrder.Release
)
```

Expected diagnostic:

```text
compare-exchange failure order cannot use release semantics
```

---

## Atomic flags

A boolean atomic may be used as a flag.

Example:

```sec
static let ShutdownRequested: Atomic[bool] := Atomic(false)

fn RequestShutdown() void {
    ShutdownRequested.store(true)
}

fn IsShutdownRequested() bool {
    return ShutdownRequested.load()
}
```

For task cancellation, the language-level task API remains preferred.

An atomic flag does not automatically integrate with:

- task cancellation
- task shutdown
- blocking waits
- scheduler wakeups

---

## Atomic counters

Atomic counters are valid when each update is independent.

Example:

```sec
static let Requests: Atomic[uint64] := Atomic(0)

fn RecordRequest() void {
    Requests.fetchAdd(1)
}
```

An atomic counter is not sufficient when other values must change as one
invariant.

---

## Multi-field invariants

Atomics protect individual operations on individual atomic locations.

They do not make a group of values transactional.

Invalid design:

```sec
type AccountState struct {
    balance: Atomic[int]
    reserved: Atomic[int]
}
```

when correctness requires:

```text
balance + reserved == total
```

across every observation.

Use:

```sec
Mutex[AccountState]
```

when multiple fields must be read or modified consistently.

---

## Mixing atomic and ordinary access

The same storage location must not be accessed both atomically and
non-atomically while concurrent access is possible.

Invalid conceptual behavior:

```sec
Counter.store(10)
let value := raw ordinary load of Counter storage
```

The compiler must not expose an ordinary reference to the contained value.

Unsafe raw-pointer access to atomic storage is explicitly unsafe and may create
an invalid data race.

---

## References

A shared reference to an atomic value may be passed between tasks when its
lifetime is valid.

Example:

```sec
fn Increment(counter: ref Atomic[uint64]) void {
    counter.fetchAdd(1)
}
```

An exclusive mutable reference is normally unnecessary for atomic operations.

Atomic mutation occurs through a shared reference to the atomic object.

This does not violate ordinary aliasing because the contained access is governed
by atomic semantics.

---

## Methods and receiver model

Atomic methods that modify the contained value should use a shared receiver.

Conceptually:

```sec
fn load(ref self) T
fn store(ref self, value: T) void
fn swap(ref self, value: T) T
fn fetchAdd(ref self, value: T) T
```

The atomic object itself is not mutably borrowed by each operation.

The contained storage is synchronized internally by the atomic semantics.

---

## Moving atomics

An atomic value may be moved before it is published to another task.

After publication, the atomic storage location may need a stable address.

The compiler should reject moves that may invalidate references or target atomic
identity.

Examples include moving:

- a static atomic
- an atomic referenced by another task
- a struct containing a published atomic
- an atomic used by an interrupt handler

A target using address-independent atomic representation may relax physical
movement while preserving semantic identity.

---

## Atomics in structs

A struct may own atomic fields.

Example:

```sec
type Statistics struct {
    requests: Atomic[uint64]
    failures: Atomic[uint64]
}
```

Each struct instance owns distinct atomic storage.

The fields remain subject to:

- initialization
- movement
- publication
- target alignment
- destruction
- memory-order rules

Atomic fields do not automatically make the entire struct concurrency-safe.

Non-atomic fields still require ordinary synchronization.

---

## Atomics and static storage

Static atomics are suitable for globally shared counters and flags.

Example:

```sec
impl Metrics {
    static let Requests: Atomic[uint64] := Atomic(0)

    static fn RecordRequest() void {
        Metrics.Requests.fetchAdd(1)
    }
}
```

The static binding is immutable.

The atomic contained value remains mutable through atomic operations.

---

## Atomics and properties

A property may wrap atomic operations.

Example:

```sec
impl Metrics {
    static let _requests: Atomic[uint64] := Atomic(0)

    static property Requests: uint64 {
        get {
            return Metrics._requests.load()
        }
    }
}
```

A property must not imply that a sequence of multiple atomic operations is one
transaction.

---

## Waiting

Version 0.1 should not require general atomic wait and notify operations.

Possible future APIs include:

```sec
value.wait(expected)
value.notifyOne()
value.notifyAll()
```

Busy-wait loops should not be the default waiting mechanism for tasks.

Use task-aware synchronization where blocking or suspension is required.

---

## Spin loops

Explicit spin loops may be valid in low-level or profile-specific code.

Example:

```sec
while !Ready.load(MemoryOrder.Acquire) {
    cpu.relax()
}
```

Such code may require:

- unsafe or low-level context
- target support
- bounded execution
- ISR or bare-metal justification

The compiler should diagnose obvious unbounded spin loops in ordinary hosted
task code when appropriate.

---

## Interrupt safety

Atomics may be used in interrupt-safe code when:

- the target operation is lock-free or interrupt-safe
- the type and alignment are supported
- the memory order is valid
- the operation does not call a blocking fallback

A target profile must expose whether each atomic operation is:

- always lock-free
- sometimes lock-free
- unsupported
- implemented with a blocking fallback

Blocking fallback atomics are invalid in interrupt-safe code.

---

## Lock-free queries

The compiler or standard reflection facilities may expose target capability.

Conceptual forms:

```sec
Atomic[uint64].isAlwaysLockFree
value.isLockFree
```

These are useful for:

- embedded profiles
- ISR validation
- performance-critical code
- portable low-level libraries

The exact API may be finalized later.

---

## ABA problem

Compare-and-exchange on pointers or generation values may be vulnerable to the
ABA problem.

Atomic pointer equality does not prove that an object remained continuously
alive between observations.

Generational pointers or tagged values may be used to detect reuse.

The compiler must not claim that atomic pointer operations alone provide memory
reclamation safety.

---

## Memory reclamation

Atomic pointers do not define how pointed-to objects are destroyed.

Lock-free structures may require an explicit reclamation strategy such as:

- ownership transfer
- generations
- epochs
- hazard references
- deferred reclamation

These strategies are outside the basic atomic rules.

A raw atomic pointer must not outlive its pointee.

---

## Overflow

Atomic integer arithmetic follows the defined Sec arithmetic operation.

Version 0.1 must not silently introduce C-style undefined signed overflow.

The exact API may provide distinct operations matching ordinary numeric rules.

For example, if ordinary checked addition returns an error, the atomic equivalent
must define how failure is reported without losing atomicity.

Until that model is finalized, atomic fetch arithmetic should be limited to
well-defined wrapping-capable unsigned operations or explicitly defined
overflow behavior.

The compiler must diagnose unsupported or ambiguous overflow semantics.

---

## Named integer types

A named type with an atomic-compatible integer representation may be atomic when
its semantic rules remain enforceable.

Example:

```sec
type RequestCount uint64
```

Possible:

```sec
Atomic[RequestCount]
```

only if all atomic operations preserve the named type's contracts.

A ranged or constrained type must not be updated through atomic arithmetic that
can produce an invalid value.

The compiler may restrict such types to:

```sec
load
store
swap
compareExchange
```

unless contract preservation is proven.

---

## Enums

An enum with a supported integer representation may be atomic.

Example:

```sec
enum WorkerState uint8 {
    Idle
    Running
    Stopping
}

let state: Atomic[WorkerState] := Atomic(WorkerState.Idle)
```

Valid operations should initially include:

```sec
load
store
swap
compareExchange
```

Arithmetic fetch operations are not valid for enums.

---

## Atomic destruction

Atomic primitive storage normally has no special destruction behavior.

If future atomic-compatible named types own resources, atomic storage must not
permit replacement that bypasses deterministic destruction.

Version 0.1 should therefore restrict `Atomic[T]` to trivially destructible
values.

Expected rule:

```text
Atomic[T] requires T to be trivially copyable and trivially destructible.
```

Pointer values may be atomic, but the atomic pointer does not own the pointee
unless a separate ownership abstraction explicitly defines that behavior.

---

## FFI

Foreign atomic storage is not automatically equivalent to `Atomic[T]`.

An FFI declaration must specify compatible:

- size
- alignment
- representation
- operation semantics
- memory ordering
- lock-free requirements

C `_Atomic`, compiler intrinsics and platform APIs may require explicit adapters.

The compiler must not treat a foreign `volatile` value as atomic.

---

## Volatile

Atomic and volatile are different concepts.

Atomic provides:

- indivisible operations
- inter-task synchronization
- memory ordering

Volatile provides target-visible access semantics for cases such as:

- memory-mapped I/O
- hardware registers
- externally modified storage

`volatile` does not make concurrent access atomic.

Atomic access does not automatically provide volatile device semantics.

A future rule document should define volatile behavior separately.

---

## Semantic analysis

The compiler must validate:

- exactly one atomic type argument
- supported atomic value type
- target size and alignment support
- operation availability
- valid method for `T`
- valid memory order
- valid compare-exchange success and failure orders
- no ordinary access to atomic storage
- no invalid movement after publication
- interrupt-safety requirements
- contract preservation for named types
- trivial copy and destruction requirements
- absence of unsynchronized mixed access

---

## Semantic IR

Semantic IR must represent atomic operations explicitly.

At minimum:

```text
AtomicCreate
AtomicLoad
AtomicStore
AtomicSwap
AtomicCompareExchange
AtomicFetchAdd
AtomicFetchSub
AtomicFetchAnd
AtomicFetchOr
AtomicFetchXor
AtomicFence
AtomicPublish
```

IR must record:

- concrete `Atomic[T]` type
- storage identity
- value type
- value width
- alignment
- operation
- success memory order
- failure memory order
- target lock-free capability
- source location

The backend must not infer atomic semantics from ordinary loads, stores or calls.

---

## Diagnostics

Examples:

```text
Atomic requires exactly one type argument
```

```text
type ApplicationState is not supported by Atomic
```

```text
atomic operation fetchAdd is not valid for bool
```

```text
MemoryOrder.Release is not valid for atomic load
```

```text
compare-exchange failure order cannot use release semantics
```

```text
target linux-arm32 does not support atomic uint64 compare-exchange
```

```text
Atomic[BoundedCount] fetchAdd may violate type contract
```

```text
cannot move atomic Counter after it has been published to another task
```

```text
atomic uint64 operation is not lock-free in interrupt-safe context
```

```text
ordinary access to atomic storage is not permitted
```

---

## Restrictions

`Atomic[T]` must not:

- accept arbitrary structured types
- expose ordinary references to contained storage
- silently use non-atomic access
- silently use invalid memory ordering
- silently fall back to a blocking lock in interrupt-safe code
- make multiple fields transactional
- replace `Mutex[T]` for structured invariants
- define memory reclamation for atomic pointers
- treat volatile as atomic
- bypass ownership or lifetime rules
- use C-style undefined signed overflow
- be treated as an ordinary library wrapper by semantic analysis

---

## Future extensions

Possible future additions include:

```sec
Atomic[int128]
Atomic[uint128]
```

when target support exists.

Other possible developments include:

- atomic wait and notify
- weak compare-and-exchange
- explicit fences
- tagged atomic pointers
- atomic reference-count abstractions
- epoch-based reclamation
- hazard references
- platform-specific lock-free requirements

These are not required for version 0.1.

---

## Related rules

Detailed behavior is defined in:

```text
concurrency.txt
concurrency_memory_model.txt
static.txt
mutex.txt
tasks.txt
registers.txt
numeric_types.txt
ffi.txt
```

## Current implementation status

Implemented:

- `Atomic[T]` is a compiler-known generic type.
- supported element types are currently `bool`, `byte`, `int8`, `int16`,
  `int32`, `int64`, `uint8`, `uint16`, `uint32`, `uint64` and `RawPtr[T]`.
- `Atomic(value)` constructs `Atomic[T]` from one initializer value.
- `load()` returns `T`.
- `store(value)` returns `void`.
- `swap(value)` returns `T`.
- `compareExchange(expected, desired)` returns `CompareExchangeResult[T]`.
- integer atomics support `fetchAdd`, `fetchSub`, `fetchAnd`, `fetchOr` and
  `fetchXor`.
- boolean atomics support `fetchAnd`, `fetchOr` and `fetchXor`.

Not implemented yet:

- `MemoryOrder`
- validation of explicit memory-order arguments
- target capability checks
- atomic/non-atomic alias diagnostics
- operation lowering to Semantic IR/MLIR/LLVM
