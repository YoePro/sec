# Atomics

- **Status:** Normative
- **Created:** 2026-09-06
- **Last updated:** 2026-09-06
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/concurrency/atomics.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `0f5027d`
- **Related rulebooks:** `rules/concurrency/concurrency.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/mutex.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/transferability.md`, `rules/types/types.md`, `rules/declarations/impl.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/compiler_analysis.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/platform/interrupts.md`, `rules/tooling/lsp.md`

---

## § 1. Purpose and authority

**Governance tags:** `concurrency.atomics-v2`

§ 1(1) This rulebook defines the Sec 0.1 atomic type family, its public source surface, the legal atomic value types, the legal operation families, memory-order selection, target capability validation, tooling requirements, and the semantic facts that must survive into Semantic IR and lowering.

§ 1(2) Atomic memory-order semantics such as acquire, release, sequential consistency, modification order, release sequences, fences, and happens-before are owned by `concurrency_memory_model.md`.

§ 1(3) This rulebook owns the source-visible atomic API and the semantic classification of atomic operations.

§ 1(4) Mutable implementation status does not belong in this normative rulebook.

§ 1(5) Implementation status is maintained through `implementation-status.yaml`.

---

## § 2. Core design

**Governance tags:** `concurrency.atomics-v2`, `compiler.platform-model`

§ 2(1) `Atomic[T]` provides atomic access to one small scalar value of type `T`.

§ 2(2) Atomics are intended for flags, counters, state markers, generation values, reference counters, sequence numbers, pointer coordination, and lock-free or low-lock coordination primitives.

§ 2(3) Atomics are not a general replacement for `Mutex[T]`.

§ 2(4) Atomicity applies to one atomic storage identity at a time.

§ 2(5) Atomic operations do not make a group of unrelated fields transactional.

§ 2(6) Atomic eligibility is a language-level semantic rule.

§ 2(7) Target capability is a separate `CompilationPlan` question.

§ 2(8) Sec must not hard-code a machine bit width as the language definition of atomic eligibility.

§ 2(9) A semantically valid `Atomic[T]` may therefore exist even when a selected target cannot implement one or more operations for that concrete `T`.

§ 2(10) An unsupported concrete operation is rejected through target capability validation; the compiler must not silently replace it with non-atomic access.

---

## § 3. Public naming

**Governance tags:** `concurrency.atomics-v2`, `tooling.atomics-v2`

§ 3(1) All public atomic functions, methods, and properties use the canonical Sec public-member naming convention.

§ 3(2) Public methods defined by this rulebook therefore use names such as:

```text
Load
Store
Swap
CompareExchange
FetchAdd
FetchSub
FetchAnd
FetchOr
FetchXor
```

§ 3(3) The public fence function is:

```text
atomic.Fence
```

§ 3(4) Lowercase spellings such as `load`, `store`, `swap`, `compareExchange`, `fetchAdd`, or `fence` are not canonical public API spellings.

§ 3(5) Compiler diagnostics, LSP completion, hover, documentation generation, formatter fixtures, and tests must use the canonical public spellings.

---

## § 4. Compiler-known and source-visible core types

**Governance tags:** `concurrency.atomics-v2`, `tooling.atomics-v2`

§ 4(1) `Atomic[T]`, `MemoryOrder`, and `CompareExchangeResult[T]` are compiler-known core concepts.

§ 4(2) Compiler-known status means that the compiler recognizes their canonical identity and special semantics.

§ 4(3) Compiler-known status does not permit the compiler to implement only a hidden internal name while omitting the source-visible type surface.

§ 4(4) The source-visible declarations that can be represented directly in Sec source must exist in module `core` in Sec 0.1.

§ 4(5) File placement inside `sec/core/` is organizational and may remain pragmatic in Sec 0.1.

§ 4(6) The compiler, parser/Sema, LSP, formatter, documentation tooling, Semantic IR, and backend must agree on the same canonical declarations and member surface.

§ 4(7) A core declaration must not be replaced by a second incompatible compiler-private declaration.

---

## § 5. Exact `MemoryOrder` declaration

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`, `tooling.atomics-v2`

§ 5(1) The exact Sec 0.1 source declaration is:

```sec
enum MemoryOrder {
    Relaxed
    // Atomicity for the atomic operation itself.
    // Does not add acquire or release ordering for surrounding ordinary memory.

    Acquire
    // Acquire ordering.
    // Used by operations that read from the atomic object.

    Release
    // Release ordering.
    // Used by operations that publish through a write to the atomic object.

    AcqRel
    // Combined acquire and release ordering.
    // Used by read-modify-write operations.

    SeqCst
    // Sequentially consistent ordering.
    // Also participates in the global SeqCst order.
}
```

§ 5(2) `enum` declares a nominal Sec enum type.

§ 5(3) `MemoryOrder` is the canonical type name.

§ 5(4) `Relaxed`, `Acquire`, `Release`, `AcqRel`, and `SeqCst` are the complete Sec 0.1 variant set.

§ 5(5) These variants have no payload.

§ 5(6) The enum is predeclared/core-visible and requires no ordinary user import.

§ 5(7) User code must not shadow or replace the compiler-known core identity in a way that changes atomic semantics.

---

## § 6. Exact `CompareExchangeResult[T]` declaration

**Governance tags:** `concurrency.atomics-v2`, `tooling.atomics-v2`

§ 6(1) The exact Sec 0.1 source declaration is:

```sec
enum CompareExchangeResult[T] {
    Exchanged
    // The comparison matched `expected` and `desired` was stored.
    // No payload is required because the observed value is known
    // to have been equal to `expected`.

    NotExchanged(T)
    // The exchange did not occur.
    // The payload is the value observed atomically by CompareExchange.
}
```

§ 6(2) `[T]` declares one generic type parameter.

§ 6(3) `Exchanged` is a payload-less enum variant.

§ 6(4) `NotExchanged(T)` is an associated-value enum variant carrying exactly one payload of the same `T` used by the atomic object.

§ 6(5) `CompareExchangeResult[T]` is not an error type.

§ 6(6) `NotExchanged(T)` represents normal compare-exchange control flow, not a Sec error.

§ 6(7) The representation intentionally does not use `Result[void, E]`.

§ 6(8) The representation intentionally does not use `Option[T]`, because Sec names both semantic outcomes directly rather than making absence mean success.

§ 6(9) The result representation also remains suitable for a future weak compare-exchange operation because `NotExchanged(expected)` can represent a spurious failure if weak compare-exchange is later standardized.

§ 6(10) Weak compare-exchange itself is not defined by this revision.

---

## § 7. `Atomic[T]` generic type use

**Governance tags:** `concurrency.atomics-v2`, `frontend.atomics-v2`

§ 7(1) The exact generic type-use syntax is:

```sec
Atomic[T]
```

§ 7(2) `Atomic` is the compiler-known nominal generic type family.

§ 7(3) `[T]` supplies exactly one contained value type.

§ 7(4) `Atomic[T]` owns one atomic storage identity containing one value of type `T`.

§ 7(5) The physical representation and any stronger alignment required by the target are compiler/platform concerns and are not exposed as ordinary source fields.

§ 7(6) User code must not access an internal storage field of `Atomic[T]`.

§ 7(7) The compiler must not model the source-facing type as an ordinary public struct containing a directly accessible `T`.

§ 7(8) The LSP hover surface for `Atomic[T]` must identify the generic type parameter and the public operations applicable to the resolved concrete `T`.

---

## § 8. Atomic-compatible types in Sec 0.1

**Governance tags:** `concurrency.atomics-v2`, `frontend.atomics-v2`, `compiler.platform-model`

§ 8(1) The Sec 0.1 atomic-compatible type set is intentionally conservative.

§ 8(2) The following categories are semantically eligible for `Atomic[T]`:

```text
bool
primitive signed integer types
primitive unsigned integer types
RawPtr[T]
integer-backed enums
named types whose underlying type is atomic-compatible under this section
```

§ 8(3) The primitive signed integer family includes the signed integer types defined by `types.md`.

§ 8(4) The primitive unsigned integer family includes the unsigned integer types defined by `types.md`.

§ 8(5) `int` and `uint` are semantically eligible even though their concrete width is selected by the target.

§ 8(6) Explicit-width integer eligibility is not limited by a hard-coded maximum bit width in this language rule.

§ 8(7) The selected `CompilationPlan` determines whether a concrete atomic operation is implementable for the resolved width, alignment, execution context, and memory order.

§ 8(8) Integer-backed enums are eligible only for the operation families allowed for enums by this rulebook.

§ 8(9) Named types are eligible only when their underlying type is atomic-compatible and the requested operation preserves the named type's semantic contracts.

---

## § 9. Types excluded from `Atomic[T]` in Sec 0.1

**Governance tags:** `concurrency.atomics-v2`, `frontend.atomics-v2`

§ 9(1) The following type categories are not atomic-compatible in Sec 0.1:

```text
float
float32
float64
decimal
decimal128
string
ordinary structs
ordinary unions
arrays
owning arrays
slices
collections
Result[T, E]
Option[T]
Task[T]
Thread[T]
arbitrary representation-compatible composite types
```

§ 9(2) A type is not made atomic-compatible merely because its representation could theoretically fit in one or more atomic machine words.

§ 9(3) A struct is not made atomic-compatible merely because its total size matches a target atomic width.

§ 9(4) Floating-point types are omitted because Sec 0.1 has no demonstrated requirement strong enough to justify additional atomic equality, representation, and arithmetic semantics.

§ 9(5) Exact decimal types are omitted because they are semantically composite numeric values in Sec and do not belong to the intentionally small initial atomic surface.

§ 9(6) This exclusion is a deliberate Sec 0.1 language design, not a claim that all targets are incapable of atomic operations over those representations.

---

## § 10. Extensibility note

**Governance tags:** `concurrency.atomics-v2`

§ 10(1) The initial atomic-compatible type set is deliberately smaller than every representation the compiler could theoretically lower atomically.

§ 10(2) Future language revisions may add additional scalar or representation-stable categories when concrete use cases demonstrate a clear user benefit.

§ 10(3) User and implementation feedback is explicitly relevant to such expansion.

§ 10(4) Future expansion must be specified as a language change; a backend must not independently make a currently excluded `Atomic[T]` legal merely because the target has a matching instruction width.

---

## § 11. Construction

**Governance tags:** `concurrency.atomics-v2`, `frontend.atomics-v2`

§ 11(1) An atomic value is initialized from one ordinary value of `T`.

§ 11(2) Canonical construction syntax is:

```sec
let ready: Atomic[bool] := Atomic(false)
```

§ 11(3) `Atomic(false)` constructs one `Atomic[bool]` value from the contextual `bool` value.

§ 11(4) Static atomic storage is valid:

```sec
static let Requests: Atomic[uint64] := Atomic(0)
```

§ 11(5) The binding holding an atomic object normally remains immutable.

§ 11(6) Mutation of the contained value occurs through atomic methods.

§ 11(7) Replacing the entire `Atomic[T]` object is distinct from atomically modifying the contained `T`.

§ 11(8) The compiler must reject construction when the contextual `T` is not atomic-compatible.

---

## § 12. Common public operation surface

**Governance tags:** `concurrency.atomics-v2`, `frontend.atomics-v2`, `tooling.atomics-v2`

§ 12(1) Every atomic-compatible `Atomic[T]` provides the following common public signatures:

```sec
impl Atomic[T] {
    fn Load() T
    // Reads the current atomic value using MemoryOrder.SeqCst.

    fn Load(order: MemoryOrder) T
    // Reads the current atomic value using the explicit valid load order.

    fn Store(value: T) void
    // Replaces the current value using MemoryOrder.SeqCst.

    fn Store(value: T, order: MemoryOrder) void
    // Replaces the current value using the explicit valid store order.

    fn Swap(value: T) T
    // Atomically replaces the value using MemoryOrder.SeqCst
    // and returns the previous value.

    fn Swap(value: T, order: MemoryOrder) T
    // Atomically replaces the value using an explicit valid RMW order
    // and returns the previous value.

    fn CompareExchange(
        expected: T,
        desired: T
    ) CompareExchangeResult[T]
    // Strong compare-exchange using SeqCst for both success and failure.

    fn CompareExchange(
        expected: T,
        desired: T,
        successOrder: MemoryOrder,
        failureOrder: MemoryOrder
    ) CompareExchangeResult[T]
    // Strong compare-exchange using separate success and failure orders.
}
```

§ 12(2) The signature-only presentation above is normative API notation.

§ 12(3) Core/compiler implementation may use privileged intrinsic bodies rather than ordinary Sec bodies, but the public signatures must match exactly.

§ 12(4) Ordinary instance methods have implicit `self` according to `impl.md`; no explicit `self` parameter is written.

§ 12(5) The compiler derives the special shared-receiver atomic semantics from the compiler-known method identity rather than exposing an ordinary mutable borrow to the stored `T`.

---

## § 13. Integer public operation surface

**Governance tags:** `concurrency.atomics-v2`, `frontend.atomics-v2`, `tooling.atomics-v2`

§ 13(1) Atomic signed and unsigned integer types additionally provide:

```sec
impl Atomic[T] {
    fn FetchAdd(value: T) T
    // Atomically adds value using SeqCst and returns the previous value.

    fn FetchAdd(value: T, order: MemoryOrder) T
    // Atomically adds value using an explicit valid RMW order
    // and returns the previous value.

    fn FetchSub(value: T) T
    // Atomically subtracts value using SeqCst and returns the previous value.

    fn FetchSub(value: T, order: MemoryOrder) T
    // Atomically subtracts value using an explicit valid RMW order
    // and returns the previous value.

    fn FetchAnd(value: T) T
    // Atomically applies bitwise AND using SeqCst and returns the previous value.

    fn FetchAnd(value: T, order: MemoryOrder) T
    // Atomically applies bitwise AND using an explicit valid RMW order
    // and returns the previous value.

    fn FetchOr(value: T) T
    // Atomically applies bitwise OR using SeqCst and returns the previous value.

    fn FetchOr(value: T, order: MemoryOrder) T
    // Atomically applies bitwise OR using an explicit valid RMW order
    // and returns the previous value.

    fn FetchXor(value: T) T
    // Atomically applies bitwise XOR using SeqCst and returns the previous value.

    fn FetchXor(value: T, order: MemoryOrder) T
    // Atomically applies bitwise XOR using an explicit valid RMW order
    // and returns the previous value.
}
```

§ 13(2) These signatures apply only when `T` is an atomic-compatible signed or unsigned integer type or an eligible named type whose contracts permit the operation.

§ 13(3) Enum eligibility does not imply integer fetch arithmetic or bitwise fetch operations.

§ 13(4) `RawPtr[T]` eligibility does not imply integer fetch arithmetic or bitwise fetch operations.

§ 13(5) `bool` eligibility does not imply the integer fetch operation surface.

---

## § 14. `Load`

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 14(1) `Load` atomically reads one complete `T`.

§ 14(2) The return type is exactly `T`.

§ 14(3) `Load()` is equivalent to `Load(MemoryOrder.SeqCst)`.

§ 14(4) Valid explicit load orders are:

```text
MemoryOrder.Relaxed
MemoryOrder.Acquire
MemoryOrder.SeqCst
```

§ 14(5) `MemoryOrder.Release` is invalid for `Load`.

§ 14(6) `MemoryOrder.AcqRel` is invalid for `Load`.

§ 14(7) `Load` does not modify the atomic object and therefore does not add a modification to its modification order.

Example:

```sec
let ready := Ready.Load(MemoryOrder.Acquire)
```

---

## § 15. `Store`

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 15(1) `Store` atomically replaces the contained value.

§ 15(2) `Store` returns `void`.

§ 15(3) `Store(value)` is equivalent to `Store(value, MemoryOrder.SeqCst)`.

§ 15(4) Valid explicit store orders are:

```text
MemoryOrder.Relaxed
MemoryOrder.Release
MemoryOrder.SeqCst
```

§ 15(5) `MemoryOrder.Acquire` is invalid for `Store`.

§ 15(6) `MemoryOrder.AcqRel` is invalid for `Store`.

§ 15(7) Every successful `Store` is a modification of the atomic object and participates in that object's modification order.

Example:

```sec
Ready.Store(true, MemoryOrder.Release)
```

---

## § 16. `Swap`

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 16(1) `Swap` atomically reads the old `T`, stores the supplied `T`, and returns the old `T`.

§ 16(2) `Swap` is one read-modify-write operation.

§ 16(3) `Swap(value)` is equivalent to `Swap(value, MemoryOrder.SeqCst)`.

§ 16(4) Valid explicit `Swap` orders are all Sec 0.1 `MemoryOrder` values.

§ 16(5) A `Swap` modification participates in modification order.

§ 16(6) A `Swap` can extend a release sequence when the memory-model rules permit it.

---

## § 17. `CompareExchange`

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 17(1) `CompareExchange` is the strong compare-exchange operation in Sec 0.1.

§ 17(2) It atomically observes the current `T`.

§ 17(3) If the observed value equals `expected` under the atomic compare-exchange semantics of the eligible `T`, `desired` is stored and the result is:

```sec
CompareExchangeResult[T].Exchanged
```

§ 17(4) If the exchange does not occur, the result is:

```sec
CompareExchangeResult[T].NotExchanged(observed)
```

§ 17(5) `observed` is the value read by that compare-exchange operation.

§ 17(6) The failure path must not perform a separate later `Load` merely to obtain the observed value.

§ 17(7) A successful `CompareExchange` is one read-modify-write modification.

§ 17(8) A failed `CompareExchange` is an atomic read only.

§ 17(9) A successful `CompareExchange` participates in modification order and may extend a release sequence.

§ 17(10) A failed `CompareExchange` does not add a modification and does not extend a release sequence.

---

## § 18. Compare-exchange default ordering

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 18(1) The simple overload:

```sec
State.CompareExchange(expected, desired)
```

uses:

```text
successOrder = MemoryOrder.SeqCst
failureOrder = MemoryOrder.SeqCst
```

§ 18(2) The explicit overload accepts separate success and failure orders.

§ 18(3) The success order applies only when the exchange occurs.

§ 18(4) The failure order applies only when the exchange does not occur.

---

## § 19. Compare-exchange success order

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 19(1) Valid success orders are:

```text
MemoryOrder.Relaxed
MemoryOrder.Acquire
MemoryOrder.Release
MemoryOrder.AcqRel
MemoryOrder.SeqCst
```

§ 19(2) The successful operation is read-modify-write, so all five Sec 0.1 orderings are meaningful.

---

## § 20. Compare-exchange failure order

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 20(1) Valid failure orders are:

```text
MemoryOrder.Relaxed
MemoryOrder.Acquire
MemoryOrder.SeqCst
```

§ 20(2) `MemoryOrder.Release` is invalid as a failure order.

§ 20(3) `MemoryOrder.AcqRel` is invalid as a failure order.

§ 20(4) The reason is semantic: the failure path performs no write and therefore cannot perform release publication.

§ 20(5) Sec does not impose an additional rule that the failure order must be weaker than the success order.

§ 20(6) Each path's order is validated according to the semantic work performed on that path.

§ 20(7) Therefore combinations such as:

```sec
State.CompareExchange(
    expected,
    desired,
    MemoryOrder.Release,
    MemoryOrder.Acquire
)
```

are semantically valid if the selected target can implement the requested operation.

---

## § 21. Fetch arithmetic

**Governance tags:** `concurrency.atomics-v2`

§ 21(1) `FetchAdd` and `FetchSub` are read-modify-write operations.

§ 21(2) They return the value stored immediately before the operation's modification.

§ 21(3) Their arithmetic semantics must be the same semantics defined for the corresponding operation on `T`.

§ 21(4) Atomic arithmetic must not introduce C-style undefined signed overflow.

§ 21(5) If the ordinary numeric contract for a named type would be violated by the requested fetch operation, the compiler must reject that operation unless the ordinary type rule provides a defined atomic-preservable result.

§ 21(6) The compiler must not silently substitute wrapping arithmetic merely because a target atomic instruction wraps.

---

## § 22. Fetch bitwise operations

**Governance tags:** `concurrency.atomics-v2`

§ 22(1) `FetchAnd`, `FetchOr`, and `FetchXor` are integer read-modify-write operations.

§ 22(2) They return the value stored immediately before the operation's modification.

§ 22(3) Their bitwise semantics are exactly the ordinary bitwise semantics of the concrete integer `T`.

§ 22(4) Enum atomic eligibility does not expose these operations merely because the enum has an integer representation.

§ 22(5) Pointer atomic eligibility does not expose these operations merely because a pointer has an integer-like machine representation.

---

## § 23. RMW ordering

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 23(1) Read-modify-write operations may use any Sec 0.1 `MemoryOrder`.

§ 23(2) The RMW operation set includes:

```text
Swap
FetchAdd
FetchSub
FetchAnd
FetchOr
FetchXor
successful CompareExchange
```

§ 23(3) A failed `CompareExchange` is not RMW.

§ 23(4) RMW operations participate in modification order.

§ 23(5) RMW operations may extend a release sequence even when the RMW operation itself uses `MemoryOrder.Relaxed`.

---

## § 24. Memory-order validation table

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`, `tooling.atomics-v2`

§ 24(1) The compiler and LSP must enforce this exact operation/order matrix:

| Operation category | Relaxed | Acquire | Release | AcqRel | SeqCst |
|---|---:|---:|---:|---:|---:|
| `Load` | yes | yes | no | no | yes |
| `Store` | yes | no | yes | no | yes |
| RMW success | yes | yes | yes | yes | yes |
| `CompareExchange` failure | yes | yes | no | no | yes |
| `atomic.Fence` | no | yes | yes | yes | yes |

§ 24(2) Invalid combinations are compile-time semantic errors.

§ 24(3) Target capability validation occurs after or alongside semantic order validation and must not make an otherwise semantically invalid order valid.

---

## § 25. Default memory order

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 25(1) Omission of an order on ordinary atomic object operations means:

```sec
MemoryOrder.SeqCst
```

§ 25(2) `SeqCst` is the default because it provides the simplest portable reasoning model.

§ 25(3) A programmer requests a weaker ordering explicitly.

§ 25(4) The formatter must not insert explicit `MemoryOrder.SeqCst` arguments where source omitted them.

§ 25(5) The fence operation is the deliberate exception: `atomic.Fence` requires an explicit order and has no zero-argument form.

---

## § 26. Exact fence surface

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`, `tooling.atomics-v2`

§ 26(1) Sec 0.1 provides the public function:

```sec
atomic.Fence(order: MemoryOrder) void
// Establishes an explicit memory-ordering fence for the current
// execution context. It does not itself Load or Store an Atomic[T].
```

§ 26(2) `atomic` is the owning core namespace/module surface for the function.

§ 26(3) `Fence` is the public function name and follows the public CamelCase naming rule.

§ 26(4) The `order` argument is mandatory.

§ 26(5) Valid fence orders are:

```text
MemoryOrder.Acquire
MemoryOrder.Release
MemoryOrder.AcqRel
MemoryOrder.SeqCst
```

§ 26(6) `MemoryOrder.Relaxed` is invalid for a fence.

§ 26(7) Exact fence synchronization semantics are defined by `concurrency_memory_model.md`.

---

## § 27. Named integer types

**Governance tags:** `concurrency.atomics-v2`, `frontend.atomics-v2`

§ 27(1) A named type whose underlying type is atomic-compatible may itself be atomic-compatible.

Example:

```sec
type RequestCount uint64

let count: Atomic[RequestCount] := Atomic(RequestCount(0))
```

§ 27(2) Atomic operations preserve the named type.

§ 27(3) `Load` returns the named type rather than its underlying primitive type.

§ 27(4) `Swap` and `CompareExchange` accept and return the named type.

§ 27(5) Fetch operations are available only when the named type's contracts remain enforceable.

§ 27(6) A ranged or constrained type must not be updated through an atomic arithmetic operation that can produce an invalid named value without the ordinary type semantics providing a valid atomic failure/result model.

---

## § 28. Integer-backed enums

**Governance tags:** `concurrency.atomics-v2`, `frontend.atomics-v2`

§ 28(1) An enum with an atomic-compatible integer underlying representation may be used in `Atomic[T]`.

Example:

```sec
enum WorkerState uint8 {
    Idle
    Running
    Stopping
}

let state: Atomic[WorkerState] := Atomic(WorkerState.Idle)
```

§ 28(2) The guaranteed enum atomic operation surface is the common surface:

```text
Load
Store
Swap
CompareExchange
```

§ 28(3) Arithmetic fetch operations are invalid for enums.

§ 28(4) Integer bitwise fetch operations are invalid for enums in Sec 0.1.

§ 28(5) The compiler must preserve enum type identity and valid enum semantics.

---

## § 29. Raw pointer atomics

**Governance tags:** `concurrency.atomics-v2`, `analysis.transferability`, `platform.atomics-v2`

§ 29(1) `RawPtr[T]` is atomic-compatible in Sec 0.1.

§ 29(2) The guaranteed pointer atomic operation surface is:

```text
Load
Store
Swap
CompareExchange
```

§ 29(3) Atomic pointer operations do not create pointee ownership.

§ 29(4) Atomic pointer operations do not extend pointee lifetime.

§ 29(5) Atomic pointer operations do not solve memory reclamation.

§ 29(6) Pointer arithmetic fetch operations are not part of the Sec 0.1 atomic API.

§ 29(7) Bitwise fetch operations are not part of the pointer atomic API.

§ 29(8) ABA safety is not implied by atomic pointer compare-exchange.

---

## § 30. Boolean atomics

**Governance tags:** `concurrency.atomics-v2`

§ 30(1) `Atomic[bool]` is valid.

§ 30(2) The guaranteed Sec 0.1 boolean atomic surface is the common surface:

```text
Load
Store
Swap
CompareExchange
```

§ 30(3) Boolean values do not receive integer arithmetic operations.

§ 30(4) This revision does not standardize additional boolean fetch-bitwise operations as part of the required portable v0.1 surface.

---

## § 31. Atomic object ownership

**Governance tags:** `concurrency.atomics-v2`, `frontend.transferability`

§ 31(1) `Atomic[T]` owns exactly one atomic storage identity.

§ 31(2) The atomic object follows ordinary Sec ownership rules.

§ 31(3) Atomic interior mutation does not require an ordinary `ref mut` to the atomic object.

§ 31(4) Valid shared references may invoke atomic methods.

Example:

```sec
fn Increment(counter: ref Atomic[uint64]) void {
    counter.FetchAdd(1)
}
```

§ 31(5) This exception is specific to compiler-known atomic interior mutation.

§ 31(6) It does not authorize ordinary unsynchronized mutation of surrounding storage.

---

## § 32. No ordinary contained-value reference

**Governance tags:** `concurrency.atomics-v2`, `sema.data-race-analysis`

§ 32(1) `Atomic[T]` must not expose an ordinary `ref T` or `ref mut T` to its contained value.

§ 32(2) All concurrent access to the contained storage must use the atomic API or another explicitly defined compiler/platform operation preserving the same atomic contract.

§ 32(3) Unsafe raw access to the same storage can invalidate race freedom and remains programmer responsibility.

§ 32(4) The compiler must not treat unsafe ordinary access as if it participated in the atomic object's modification order.

---

## § 33. Moving atomics and publication

**Governance tags:** `concurrency.atomics-v2`, `analysis.transferability`

§ 33(1) An unpublished atomic object may be moved according to ordinary ownership rules.

§ 33(2) Once references or execution contexts depend on the atomic storage identity, movement must preserve that semantic identity.

§ 33(3) The compiler must reject a source-level move that can invalidate a live reference, wait structure, platform mapping, interrupt reference, or other identity-dependent use.

§ 33(4) A backend may use internal indirection to preserve identity while relocating representation, provided the source semantics remain unchanged.

---

## § 34. Atomics in structs

**Governance tags:** `concurrency.atomics-v2`

§ 34(1) A struct may own `Atomic[T]` fields.

Example:

```sec
type Statistics struct {
    Requests: Atomic[uint64]
    Failures: Atomic[uint64]
}
```

§ 34(2) Each atomic field has its own atomic storage identity.

§ 34(3) Atomic fields do not make the enclosing struct itself atomic.

§ 34(4) Atomic fields do not make non-atomic fields safe for concurrent mutation.

§ 34(5) Public fields follow the ordinary public naming rules; source-file-private fields may use the ordinary visibility mechanism defined elsewhere.

---

## § 35. Multi-field invariants

**Governance tags:** `concurrency.atomics-v2`

§ 35(1) Separate atomics do not provide transactional observation across multiple storage identities.

§ 35(2) If correctness depends on one invariant covering several values, a mutex or another suitable higher-level synchronization mechanism is normally required.

§ 35(3) Atomic operations must not be documented as protecting unrelated ordinary fields merely because the fields are nearby in memory.

---

## § 36. Mixing atomic and ordinary access

**Governance tags:** `concurrency.atomics-v2`, `sema.data-race-analysis`

§ 36(1) The same shared storage location must not be concurrently accessed through a mixture of Sec atomic operations and ordinary non-atomic operations.

§ 36(2) Such mixing is not repaired by using a strong memory order on the atomic side.

§ 36(3) The compiler must reject statically provable invalid mixing.

§ 36(4) Unsafe code can make static proof unavailable but does not make a data race semantically valid.

---

## § 37. Target capability

**Governance tags:** `concurrency.atomics-v2`, `compiler.platform-model`, `platform.atomics-v2`

§ 37(1) The selected immutable `CompilationPlan` is the sole target/platform truth for concrete atomic capability.

§ 37(2) Capability may depend on:

- the concrete `T`;
- concrete size and alignment;
- operation kind;
- requested memory order;
- execution context;
- interrupt-safety requirements;
- target architecture;
- runtime/profile policy.

§ 37(3) Language eligibility must not be recomputed from compiler-host properties.

§ 37(4) The compiler must not reject `Atomic[T]` merely because the compiler host lacks a native operation that the selected target provides.

§ 37(5) The compiler must not accept an operation merely because the compiler host provides it when the selected target does not.

---

## § 38. Native versus emulated implementation

**Governance tags:** `concurrency.atomics-v2`, `platform.atomics-v2`

§ 38(1) A conforming target may implement a required atomic operation through a native instruction, compiler intrinsic, verified platform primitive, or another target-declared mechanism preserving the full atomic contract.

§ 38(2) Native single-instruction implementation is not required by the language merely for `Atomic[T]` to be semantically valid.

§ 38(3) A hidden blocking global mutex fallback is not automatically permitted.

§ 38(4) Any emulation strategy must be declared compatible with the current execution context and target profile.

§ 38(5) An emulation that blocks is invalid in a context whose contract forbids blocking.

§ 38(6) An emulation that is not interrupt-safe is invalid where interrupt-safe atomicity is required.

§ 38(7) When no conforming implementation exists, compilation for that `CompilationPlan` must fail with a focused capability diagnostic.

---

## § 39. Interrupt contexts

**Governance tags:** `concurrency.atomics-v2`, `platform.atomics-v2`

§ 39(1) Atomic eligibility does not automatically imply ISR suitability.

§ 39(2) An ISR may use an atomic operation only when the selected `CompilationPlan` proves the concrete lowering valid for that interrupt context.

§ 39(3) A blocking emulation must not satisfy an ISR-safe atomic requirement.

§ 39(4) Interrupt masking may be used as a platform implementation technique only where the platform contract proves that it establishes the required exclusion and ordering.

§ 39(5) Local masking must not be assumed to synchronize with another core, DMA engine, or unrelated execution domain.

---

## § 40. Fences

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 40(1) `atomic.Fence` is part of Sec 0.1.

§ 40(2) It is a real source-level operation, not merely permission for a backend to emit a target fence.

§ 40(3) The compiler must model its ordering semantics before lowering.

§ 40(4) A fence does not access an `Atomic[T]` value itself.

§ 40(5) Fence synchronization requires the atomic communication relationships defined by `concurrency_memory_model.md`.

§ 40(6) Two fences do not synchronize merely because they are both fences.

---

## § 41. Modification order participation

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 41(1) Every atomic object has the per-object modification order defined by `concurrency_memory_model.md`.

§ 41(2) Operations that add a modification are:

```text
Store
Swap
FetchAdd
FetchSub
FetchAnd
FetchOr
FetchXor
successful CompareExchange
```

§ 41(3) Operations that do not add a modification are:

```text
Load
failed CompareExchange
atomic.Fence
```

§ 41(4) An RMW operation observes and modifies one coherent state of the same atomic object.

---

## § 42. Release-sequence participation

**Governance tags:** `concurrency.atomics-v2`, `concurrency.memory-model-v2`

§ 42(1) A release sequence starts from a release modification as defined by the memory-model rulebook.

§ 42(2) A contiguous chain of later RMW modifications on the same atomic object may extend that release sequence.

§ 42(3) `Swap`, fetch operations, and successful `CompareExchange` may therefore extend a release sequence.

§ 42(4) A failed `CompareExchange` cannot extend one because it performs no modification.

§ 42(5) A plain `Store` after the head or RMW chain breaks the earlier release sequence even if that store itself is atomic.

---

## § 43. LSP hover requirements

**Governance tags:** `tooling.atomics-v2`, `concurrency.atomics-v2`

§ 43(1) LSP hover for `MemoryOrder` must expose the exact enum declaration and variant documentation.

§ 43(2) LSP hover for `CompareExchangeResult[T]` must expose:

- generic arity;
- `Exchanged`;
- `NotExchanged(T)`;
- the payload meaning;
- the fact that the type is not an error.

§ 43(3) LSP hover for `Atomic[T]` must expose the resolved contained type and the exact public methods applicable to that concrete `T`.

§ 43(4) Hover must use canonical CamelCase member names.

§ 43(5) Hover must not present target-private backend intrinsics as the source API.

§ 43(6) When an operation is semantically valid but unsupported by the selected target, tooling should distinguish "valid Sec operation, unavailable for this CompilationPlan" from "invalid operation for this T".

---

## § 44. Completion and navigation

**Governance tags:** `tooling.atomics-v2`

§ 44(1) Completion after an `Atomic[T]` expression must offer only the public methods semantically applicable to the resolved `T`, subject to the ordinary completion policy.

§ 44(2) Completion for `MemoryOrder.` must offer exactly the five canonical variants.

§ 44(3) Completion for `CompareExchangeResult[T]` pattern contexts must expose the two canonical variants.

§ 44(4) Go-to-definition/navigation for source-visible core declarations must reach the canonical core declaration where the tooling supports source navigation.

§ 44(5) The compiler-known identity and source declaration must be treated as one semantic symbol, not two unrelated types.

---

## § 45. Syntax and type checking

**Governance tags:** `frontend.atomics-v2`, `tooling.atomics-v2`

§ 45(1) The compiler must validate exactly one type argument for `Atomic[T]`.

§ 45(2) The compiler must validate exactly one type argument for `CompareExchangeResult[T]`.

§ 45(3) The compiler must validate the atomic-compatible type set.

§ 45(4) The compiler must validate operation availability by concrete `T`.

§ 45(5) The compiler must validate each explicit `MemoryOrder`.

§ 45(6) The compiler must validate compare-exchange success and failure orders independently.

§ 45(7) The compiler must reject `Release` and `AcqRel` failure orders.

§ 45(8) The compiler must not impose a "failure order must be weaker than success order" rule.

§ 45(9) The compiler must reject `MemoryOrder.Relaxed` for `atomic.Fence`.

§ 45(10) These checks are source-language semantic checks, not backend-verifier substitutions.

---

## § 46. Semantic IR

**Governance tags:** `analysis.semantic-ir-v2`, `semantic-ir.atomics-v2`

§ 46(1) Semantic IR must preserve atomic operations explicitly enough that later lowering does not rediscover them from ordinary loads, stores, calls, or names.

§ 46(2) It must preserve:

- atomic storage identity;
- concrete `Atomic[T]`;
- concrete `T`;
- operation kind;
- success memory order where applicable;
- failure memory order where applicable;
- RMW versus read-only classification;
- source provenance;
- execution-context constraints;
- target capability requirements.

§ 46(3) Successful and failed `CompareExchange` paths must remain semantically distinguishable.

§ 46(4) Semantic IR must preserve that only the successful compare-exchange path performs a modification.

§ 46(5) Fence semantics must remain explicit.

§ 46(6) Concrete Semantic IR operation names are owned by `semantic_ir.md`; this rulebook defines required semantics rather than a competing opcode vocabulary.

---

## § 47. Lowering

**Governance tags:** `lowering.atomics-v2`, `compiler.platform-model`

§ 47(1) Lowering consumes validated atomic Semantic IR plus the selected `CompilationPlan`.

§ 47(2) Lowering must preserve requested ordering or a stronger ordering that is observationally compatible.

§ 47(3) Lowering must never weaken requested ordering.

§ 47(4) Lowering must preserve no-tearing atomicity.

§ 47(5) Lowering must preserve success/failure result semantics of `CompareExchange`.

§ 47(6) Lowering must return the observed value from the actual failed compare-exchange, not from a later unrelated load.

§ 47(7) Lowering must preserve release-sequence and modification-order semantics required by the source memory model.

§ 47(8) Backend instruction names or host atomic APIs are not the source-language definition.

---

## § 48. Diagnostics

**Governance tags:** `tooling.atomics-v2`, `frontend.atomics-v2`

§ 48(1) Diagnostics should distinguish language-type invalidity from target capability failure.

Examples:

```text
type float64 is not atomic-compatible in Sec 0.1
```

```text
FetchAdd is not available for Atomic[WorkerState]
```

```text
MemoryOrder.Release is not valid for Atomic.Load
```

```text
MemoryOrder.AcqRel is not valid as a CompareExchange failure order
```

```text
MemoryOrder.Relaxed is not valid for atomic.Fence
```

```text
selected CompilationPlan cannot implement Atomic[uint64].CompareExchange with the requested ordering
```

§ 48(2) Diagnostics for target capability should identify the selected target/profile fact when known.

§ 48(3) Diagnostics must not suggest converting an invalid ordinary shared access to volatile as a concurrency fix.

---

## § 49. Restrictions

**Governance tags:** `concurrency.atomics-v2`

§ 49(1) `Atomic[T]` must not accept arbitrary composite types in Sec 0.1.

§ 49(2) `Atomic[T]` must not expose ordinary direct access to its contained storage.

§ 49(3) The compiler must not silently use non-atomic access.

§ 49(4) The compiler must not silently use a weaker memory order.

§ 49(5) The compiler must not treat volatile access as atomic.

§ 49(6) Atomics must not bypass ownership or lifetime rules.

§ 49(7) Atomics must not imply memory reclamation safety for pointers.

§ 49(8) Atomics must not make multiple storage identities transactional.

§ 49(9) Atomics must not use compiler-host capability as target truth.

§ 49(10) The compiler must not expose lowercase alternate public method spellings.

---

## § 50. Non-normative future input areas

**Governance tags:** `concurrency.atomics-v2`

The following are intentionally not part of the required Sec 0.1 portable surface in this revision and should be reconsidered only through an explicit later language decision informed by real use:

- floating-point atomic types;
- decimal atomic types;
- arbitrary representation-stable structs;
- weak compare-exchange;
- atomic wait/notify;
- a portable public lock-free capability query API;
- tagged atomic pointers;
- dedicated reclamation abstractions;
- additional boolean or byte fetch operations.

This section is non-normative and does not authorize implementations to expose incompatible source APIs under the same canonical names.

---

## § 51. Governance

**Governance tags:** `concurrency.atomics-v2`, `frontend.atomics-v2`, `tooling.atomics-v2`, `semantic-ir.atomics-v2`, `lowering.atomics-v2`, `platform.atomics-v2`, `concurrency.memory-model-v2`, `compiler.platform-model`, `analysis.semantic-ir-v2`, `sema.data-race-analysis`

§ 51(1) Mutable implementation information for this rulebook must be maintained in `implementation-status.yaml`.

§ 51(2) The primary governance integration is `concurrency.atomics-v2`.

§ 51(3) Core declaration/tooling work is tracked through `tooling.atomics-v2` and frontend atomic governance.

§ 51(4) Semantic IR and lowering must preserve the canonical source semantics even when current implementation coverage is partial.

§ 51(5) Implementation status must not weaken the normative type set, API names, memory-order validation, compare-exchange result type, or fence semantics.

§ 51(6) Cross-rulebook synchronization required by this revision is tracked in the accompanying corrections document.
