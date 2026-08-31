# Allocation

- Status: Normative
- Created: 2026-08-30
- Last updated: 2026-08-30
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/allocation.md`
- Replaces: `rules/memory/allocation.txt`
- Repository baseline reviewed: `57cd774`

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the general dynamic-allocation model of Sec.

§ 1(2) Ordinary programmers should not need to select allocators, manually propagate allocator handles, or manage individual raw allocations.

§ 1(3) Manual allocation control remains available for systems programming, embedded targets, allocator implementations, FFI wrappers, deterministic-memory designs, and performance-sensitive code.

§ 1(4) This rulebook defines common allocation semantics. Public APIs and operation-specific semantics of collections, strings, shaped types, closures, FFI wrappers, and platform mappings remain owned by their canonical rulebooks.

§ 1(5) `rules/memory/arena.md` owns the programmer-visible `Arena` type and Arena-specific allocation, backing storage, reset, release, generation, capacity, and lifetime semantics.

§ 1(6) `rules/memory/storage.md` owns canonical storage-origin and storage-property classification.

§ 1(7) `rules/memory/ownership.md`, `borrowing.md`, `lifetime_analysis.md`, `destruction.md`, `references.md`, and `reference_model.md` own their corresponding memory-model semantics.

§ 1(8) Platform rulebooks may restrict or provide allocation capabilities, but they must not redefine source-level ownership, lifetime, reference, or destruction semantics.

---

## § 2 Core principles

§ 2(1) Dynamic allocation is explicit through the operation being performed.

§ 2(2) Selection and propagation of the allocation context are normally implicit.

§ 2(3) The compiler may choose or propagate an allocation context only for an operation whose canonical contract permits allocation.

§ 2(4) The compiler must not introduce dynamic allocation for an operation that is not defined as allocation-capable.

§ 2(5) Copy, move, borrow, parameter passing, return, reference creation, escape analysis, and lifetime repair must not by themselves introduce dynamic allocation.

§ 2(6) Allocation is distinct from ownership transfer, initialization, destruction, storage reclamation, and reference validity.

§ 2(7) Allocation is distinct from physical-address binding, MMIO, register access, and volatile storage access.

§ 2(8) Allocation origin and allocation semantics must be decided before backend lowering. LLVM, MLIR lowering, and target code generation must not invent source-level allocation semantics.

---

## § 3 Allocation-capable operations

§ 3(1) An operation is allocation-capable only when its canonical language, core, standard-library, collection, shaped-type, or platform contract states that it may allocate.

§ 3(2) Calling an allocation-capable operation does not require source syntax naming an allocator or Arena when an implicit allocation context is permitted.

```sec
let values := try list[int].WithCapacity(100)
let text := try string.From(source)
```

§ 3(3) The absence of explicit allocation-context syntax does not make an allocating operation allocation-free.

§ 3(4) Tooling and analysis must be able to identify allocation-capable operations even when the allocation context is implicit.

§ 3(5) An operation defined as allocation-free must remain allocation-free in conforming lowering.

§ 3(6) A backend must not transform an allocation-free source operation into a dynamic program allocation merely for convenience.

---

## § 4 Operations that must not silently allocate

§ 4(1) Ordinary copy must not silently allocate.

```sec
let second := first
```

§ 4(2) Ordinary assignment must not silently allocate merely because a value is replaced.

```sec
target = source
```

§ 4(3) Move initialization and move assignment must not silently allocate.

```sec
let second :<- first
destination <- source
```

§ 4(4) Parameter passing must not silently allocate merely because a value crosses a call boundary.

```sec
Inspect(value)
Consume(<-resource)
```

§ 4(5) Returning a value must not silently allocate merely because ownership or a value crosses a return boundary.

```sec
return value
```

§ 4(6) Creating `ref`, `ref mut`, slices, or other safe views must not silently allocate merely to make the reference legal.

§ 4(7) Closure capture must not silently allocate merely to repair an otherwise invalid lifetime or escape.

§ 4(8) `match`, `try`, `switch`, `if`, loops, control-flow joins, `discard`, and ordinary destruction must not introduce allocation merely because analysis becomes complex.

---

## § 5 Default allocation model

§ 5(1) Arena allocation is the default dynamic-allocation model for Sec 0.1.

§ 5(2) The compiler maintains a canonical active allocation context when dynamic allocation is available for the selected target and build profile.

§ 5(3) An active allocation context may be a compiler-managed local Arena, a caller-propagated Arena, an explicitly selected Arena, a target-provided Arena, another canonically defined allocation domain, or unavailable in a profile that forbids dynamic allocation.

§ 5(4) The active allocation context is a compiler semantic fact, not a source-level global variable.

§ 5(5) Ordinary application code must not be forced to thread an Arena argument through every function merely because nested operations may allocate.

§ 5(6) Explicit allocation control may require an explicit Arena or other allocation-domain argument.

§ 5(7) Compiler-selected and explicitly selected allocation contexts must preserve the same source-level ownership, error, lifetime, reference, and destruction semantics.

§ 5(8) The compiler must not silently switch to a semantically different allocation domain when that changes lifetime, address stability, capacity guarantees, destruction, or error behavior.

---

## § 6 Allocation-context propagation

§ 6(1) Allocation context is propagated only through operations whose semantic effect permits allocation.

§ 6(2) A callable that may allocate has a compiler-visible allocation effect.

§ 6(3) Allocation-effect inference must be transitive through the represented call graph.

§ 6(4) Imported, indirect, interface, generic, recursive, and bodyless callables must carry sufficient allocation-effect metadata or be handled conservatively.

§ 6(5) A caller may provide an allocation context to a callee without exposing that context in ordinary source syntax when the callable contract permits implicit propagation.

§ 6(6) Explicit context selection overrides the implicit context only for the operation/API contract that accepts it.

§ 6(7) Context propagation must not extend the lifetime of borrowed or locally owned values merely to satisfy an escape.

§ 6(8) `noalloc` or an equivalent target/project policy may require proof that the complete reachable call graph from a root is allocation-free.

---

## § 7 No hidden escape promotion

§ 7(1) The compiler must never allocate merely because a value or reference escapes its original scope.

```sec
fn Invalid() ref byte[] {
    let data: byte[16] := [...]
    return ref data[..]
}
```

§ 7(2) The compiler must reject the invalid escape. It must not repair the program by moving `data` into an Arena, heap, hidden box, closure object, reference-counted cell, or other longer-lived storage.

§ 7(3) An owning operation that intentionally creates escaping storage must allocate directly in a suitable allocation context.

§ 7(4) Escape analysis may recommend a different API or storage strategy but must not silently alter storage lifetime.

§ 7(5) This prohibition applies equally to returns, closures, callbacks, tasks, threads, deferred work, ISR handoff, FFI retention, and stored references.

---

## § 8 Explicit Arena selection

§ 8(1) Allocation-capable APIs may accept an explicit mutable Arena reference.

```sec
let values := try Type.Create(ref mut arena, arguments)
```

§ 8(2) An equivalent normal form may omit the Arena when implicit allocation-context selection is permitted.

```sec
let values := try Type.Create(arguments)
```

§ 8(3) Passing an explicit Arena is ordinary borrowing of the Arena value according to `borrowing.md`.

§ 8(4) Mutation of Arena allocation state requires mutable authority.

§ 8(5) Explicit Arena selection does not transfer ownership of the Arena unless the API separately declares a consuming contract.

§ 8(6) Sec 0.1 introduces no general allocation keyword and no new scoped Arena syntax through this rulebook.

---

## § 9 Manual typed Arena allocation

§ 9(1) Manual safe allocation is performed through the canonical `Arena` API.

```sec
let storage := try arena.Alloc[byte](4096)
```

§ 9(2) The exact public signature and Arena-specific restrictions are owned by `arena.md`.

§ 9(3) For the safe initialized allocation form, `T` must be sized and satisfy the initialization/destruction constraints required by `arena.md`.

§ 9(4) Every element exposed as readable safe `T` storage must be initialized before it becomes readable.

§ 9(5) The element count is a count of `T`, not an untyped byte count.

§ 9(6) Required byte-size computation must be checked for arithmetic overflow.

§ 9(7) Required alignment is derived from `T` and target/layout rules.

§ 9(8) Returned storage lifetime is bounded by the Arena allocation domain.

§ 9(9) The returned reference/slice/view does not individually own or deallocate the Arena's raw storage.

§ 9(10) General safe uninitialized allocation must not expose uninitialized bytes as ordinary readable `ref mut T[]`.

§ 9(11) A future uninitialized-memory facility must use a distinct explicit uninitialized-memory type or an unsafe operation.

---

## § 10 Allocation failure

§ 10(1) A dynamic allocation that may fail uses the ordinary Sec fallibility model.

§ 10(2) The standard allocation failure type is `AllocationError` where the canonical API specifies it.

§ 10(3) A failed safe allocation must not silently return null as a safe reference, return an invalid safe reference, expose partially initialized readable storage, continue with a shorter allocation unless explicitly specified, terminate the process, or panic merely because ordinary allocation failed.

§ 10(4) Allocation failure is handled or propagated using ordinary `Result`, `try`, and error rules.

§ 10(5) An allocation may be treated as infallible only when the compiler or platform contract proves the required storage, capacity, alignment, and other preconditions.

---

## § 11 Storage ownership versus value ownership

§ 11(1) Raw storage ownership and value ownership are distinct semantic concepts.

§ 11(2) An Arena owns or controls reclamation of its backing raw storage according to `arena.md`.

§ 11(3) A value stored in Arena-backed memory still follows ordinary Sec ownership, move, availability, and destruction rules.

§ 11(4) A reference or slice into Arena-backed storage is non-owning.

§ 11(5) Destroying a value stored in Arena memory does not necessarily reclaim the Arena bytes immediately.

§ 11(6) Reclaiming Arena storage does not replace the obligation to destroy live non-trivial values before storage becomes invalid.

§ 11(7) Allocation analysis must never infer value ownership merely from storage origin.

§ 11(8) Value ownership must never be inferred to imply raw-storage reclamation authority.

```text
allocation domain
    controls raw storage

owned value
    controls value lifetime and owned subresources

reference or slice
    borrows live initialized storage
```

---

## § 12 Initialization

§ 12(1) Allocation produces storage; it does not by itself prove that a valid readable `T` exists in that storage.

§ 12(2) Safe APIs returning readable `T` or `T[]` must complete required initialization before exposure.

§ 12(3) Partial initialization follows ordinary partial-construction and destruction rules.

§ 12(4) If initialization fails after allocation, every already-initialized non-trivial subvalue must be cleaned up according to `destruction.md`.

§ 12(5) Reclaiming raw bytes is not a substitute for required value cleanup.

---

## § 13 Destruction and reclamation

§ 13(1) Arena allocation must not delay ordinary value destruction until Arena reset or Arena destruction.

§ 13(2) Owned values are destroyed at their normal ownership boundaries even when their storage came from an Arena.

§ 13(3) An Arena must not become an implicit destructor registry for arbitrary values.

§ 13(4) The compiler must distinguish ending a value lifetime, running custom `free`, releasing an external resource, invalidating a reference, and reclaiming allocation-domain storage.

§ 13(5) Storage reclamation must not occur while a live value or reference still requires the reclaimed storage.

---

## § 14 Arena reset and release

§ 14(1) Arena reset/release semantics are owned by `arena.md`; this section defines only their general allocation relationship.

§ 14(2) Reset or release invalidates allocations whose storage lifetime is tied to the affected Arena generation/domain.

§ 14(3) Reset/release must not proceed while a live dependency makes reclamation invalid.

§ 14(4) Dependencies include live owned values, safe references/views, captures, deferred operations, returned dependencies, and other storage users defined by lifetime analysis.

§ 14(5) The compiler must reject statically provable invalid reset/release.

§ 14(6) A generation/epoch dependency may preserve reference validity when static proof alone is insufficient under the selected profile.

§ 14(7) Generation metadata is not a mandatory runtime header, garbage collector handle, or reference-counting scheme.

§ 14(8) Repeated ordinary allocation from one Arena does not by itself invalidate earlier allocations.

§ 14(9) A temporary mutable borrow needed to update Arena metadata may end when the allocation call returns while returned storage retains an Arena-domain lifetime dependency.

---

## § 15 Storage origin and properties

§ 15(1) Allocation analysis must preserve canonical storage metadata from `storage.md`.

§ 15(2) Arena-allocated storage has the canonical Arena storage origin defined by `storage.md`.

§ 15(3) Storage origin is separate from external/foreign provenance, fixed-address status, address stability, reclamation authority, mutability, volatility, device liveness, and interrupt accessibility.

§ 15(4) Storage origin is not nominal type identity.

§ 15(5) Two references or slices with the same Sec type may refer to different storage origins and lifetimes.

§ 15(6) Physical MMIO/register storage is not Arena allocation merely because the compiler has a typed view of that address.

---

## § 16 Collections and dynamic owning values

§ 16(1) Collection, shaped-type, and string rulebooks define which of their operations allocate.

§ 16(2) Their allocation-capable operations must consume the canonical allocation-context and allocation-effect model.

§ 16(3) An empty owning collection may be allocation-free when its canonical rule defines an allocation-free empty representation.

§ 16(4) Growing a collection may allocate only when that operation is canonically allocation-capable.

§ 16(5) Views, slices, reshapes, transposes, borrows, and other non-owning transformations must remain allocation-free when their canonical rule defines them as allocation-free.

§ 16(6) Materialization operations that intentionally create new owning storage are allocation-capable and must obey this rulebook.

---

## § 17 Strings

§ 17(1) String operations that allocate must be classified as allocation-capable by the canonical string/core rule.

§ 17(2) Copy, move, borrow, comparison, indexing, iteration, and other string operations must not allocate unless explicitly permitted by their canonical rule.

§ 17(3) Allocation analysis must preserve actual string storage/lifetime facts rather than assume every string owns heap storage.

§ 17(4) Runtime concatenation/materialization may allocate only through the canonical allocation context and failure model.

§ 17(5) Compile-time folded string materialization is not a runtime dynamic allocation merely because constant data is stored in the output image.

---

## § 18 Closures and escaping execution

§ 18(1) Closure creation must not allocate merely to repair an illegal captured-reference lifetime.

§ 18(2) If a closure representation is canonically allocation-capable, that allocation must be explicit in its semantic effect and must not change capture ownership rules.

§ 18(3) Allocation may intentionally create storage whose lifetime covers later execution only through an operation whose contract explicitly creates such owned storage.

§ 18(4) The compiler must not infer automatic heap promotion for escaping captures.

---

## § 19 FFI and foreign allocation

§ 19(1) Foreign allocation does not become Arena allocation merely because Sec owns or wraps the returned resource.

§ 19(2) FFI contracts must identify ownership, lifetime, deallocation responsibility, and retention independently from Sec allocation-context propagation.

§ 19(3) A wrapper may normalize foreign allocation failure into `Result`.

§ 19(4) A foreign pointer returned by an allocator does not automatically become a safe reference.

§ 19(5) Sec must not silently copy foreign-allocated storage into an Arena merely to obtain safe provenance unless the source operation explicitly requests such materialization.

---

## § 20 Fixed-address and hardware storage

§ 20(1) Fixed-address storage, MMIO, and hardware register mappings are not dynamic allocation merely because they provide storage.

§ 20(2) Binding a physical address must not be modeled as Arena allocation.

§ 20(3) Mapping an OS/device region may be a fallible resource operation, but its physical backing semantics are owned by the platform mapping rulebook.

§ 20(4) Typed register views borrow mapping/platform validity; they do not own Arena storage.

§ 20(5) Volatile reads/writes are storage accesses, not allocations.

---

## § 21 Interrupt and restricted execution contexts

§ 21(1) Allocation legality in ISR/interrupt execution is owned by `interrupts.md` and target policy.

§ 21(2) An allocation-capable operation reachable from an ISR root must be rejected when the ISR/target policy forbids dynamic allocation.

§ 21(3) Preallocated Arenas or fixed-capacity domains may be used only when interrupt and target rules permit their operations.

§ 21(4) The compiler must not hide allocation inside an apparently allocation-free helper reachable from an ISR.

§ 21(5) Allocation effects must therefore be visible to call-graph and ISR analysis.

---

## § 22 Target and build profiles

§ 22(1) A target or build profile may restrict or forbid dynamic allocation.

§ 22(2) Typical semantic configurations include:

```text
hosted
    compiler-managed allocation context available

embedded-arena
    fixed or program/platform-supplied Arena available

noalloc
    dynamic allocation unavailable
```

§ 22(3) Ownership, borrowing, lifetime, destruction, and reference semantics remain consistent across allocation profiles.

§ 22(4) When no allocation context is available, unresolved use of an operation requiring dynamic allocation is a compile-time error.

§ 22(5) A profile allowing only statically bounded allocation may require capacity proof.

§ 22(6) Runtime-free bare-metal Sec must remain possible.

---

## § 23 Capacity, size, alignment, and overflow

§ 23(1) Allocation size must use target-correct type layout.

§ 23(2) Arithmetic used to compute allocation size must be checked for overflow before allocation.

§ 23(3) Alignment must satisfy the allocated type and target ABI/platform requirements.

§ 23(4) A fixed-capacity domain must reject or fail an allocation that cannot fit.

§ 23(5) The compiler may eliminate failure/checking only when bounded capacity is completely proven.

§ 23(6) Static capacity proof must account for control-flow multiplicity, loops, recursion, call graph, target layout, and all relevant allocation-capable operations.

---

## § 24 Allocation effects

§ 24(1) Allocation is a compiler-visible semantic effect.

§ 24(2) Effect analysis must distinguish operations that may allocate from operations proven allocation-free.

§ 24(3) Allocation effect is independent from panic, I/O, volatile, FFI, mutation, synchronization, blocking, and other effect categories.

§ 24(4) Allocation-effect summaries must be deterministic and reusable by compilation, LSP, ISR analysis, stack analysis, project policy, and lowering.

§ 24(5) Imported/separately compiled allocation-effect summaries must be versioned and validated before use as positive proof.

§ 24(6) Unknown allocation effect is conservative where policy requires proof of allocation freedom.

---

## § 25 Semantic IR

§ 25(1) Every dynamic allocation surviving semantic analysis must be explicit in Semantic IR.

§ 25(2) Semantic IR must preserve enough information to recover allocation kind, selected domain/context, requested type/count/size, alignment, fallibility, resulting storage origin/domain, ownership/borrow mode, lifetime/generation dependency where required, destruction responsibility, and relevant allocation effects.

§ 25(3) Semantic IR must distinguish allocation from initialization and destruction.

§ 25(4) Semantic IR verification must reject contradictory allocation facts.

§ 25(5) Allocation context must be resolved before a lowering stage would otherwise need to guess allocator semantics.

---

## § 26 Lowering

§ 26(1) Lowering must preserve selected allocation domain and source-level failure semantics.

§ 26(2) LLVM/MLIR/backend stages must not select a different allocator merely because it is convenient.

§ 26(3) Lowering must preserve required alignment, overflow checks, capacity checks, lifetime dependencies, and cleanup until equivalent lower-level behavior exists.

§ 26(4) Lowering may eliminate allocation only when every observable Sec semantic requirement is preserved.

§ 26(5) Allocation elimination must not create dangling references, alter ownership/destruction timing, or change failure behavior.

§ 26(6) Stack/static placement may replace a dynamic allocation only when storage lifetime, capacity, address stability, recursion, escape, concurrency, and target constraints are proven.

§ 26(7) Such placement is an optimization/storage-placement decision, never hidden escape repair.

§ 26(8) Ordinary Sec allocation must not require mandatory garbage collection or reference counting.

---

## § 27 Static placement and allocation elimination

§ 27(1) A compiler may choose non-dynamic storage for an allocation-capable source operation only when source-level observable semantics are preserved.

§ 27(2) Storage must remain valid for the complete required lifetime.

§ 27(3) Capacity must be proven sufficient.

§ 27(4) Required address stability and provenance must be preserved.

§ 27(5) Failure behavior may be removed only when allocation success is proven.

§ 27(6) Recursion, reentrancy, concurrency, ISR execution, and multiple simultaneous live instances must be included in the proof.

§ 27(7) Static placement must not accidentally share storage between independent live values.

§ 27(8) A no-allocation target may accept an otherwise allocation-capable source operation when complete proof removes dynamic allocation and preserves semantics.

---

## § 28 Diagnostics

§ 28(1) Allocation diagnostics must follow the mentor-compiler principle.

§ 28(2) Diagnostics should explain which operation may allocate, why allocation is required, which context/profile applies, why no valid context/capacity exists, and a practical alternative when known.

§ 28(3) Hidden-allocation violations should identify both the apparently non-allocating source operation and the behavior that would have required allocation.

§ 28(4) A no-allocation-policy diagnostic should identify the call path/root when the violation is transitive.

```text
error: this call may allocate, but dynamic allocation is disabled for this build

`BuildMessage()` may call `string.Concat`, which requires an allocation context.

help: use a preallocated buffer or select a build profile that permits allocation
```

---

## § 29 LSP and analysis tooling

§ 29(1) LSP and `sec analyse` must consume the same canonical allocation-effect/context facts as compilation.

§ 29(2) Tooling may expose whether a callable is allocation-free, may allocate, or has unresolved allocation behavior.

§ 29(3) Tooling may expose the selected/expected allocation domain where useful.

§ 29(4) Tooling should navigate from an allocation warning to the operation or transitive callee introducing the allocation effect.

§ 29(5) Incremental analysis must invalidate allocation summaries when bodies, profiles, imports, or allocating API contracts change.

---

## § 30 Required test families

§ 30(1) Required no-hidden-allocation tests include copy, move, borrow/reference creation, parameter passing, return, control-flow joins, invalid escapes, and closure escapes.

§ 30(2) Required Arena tests include implicit context, explicit selection, repeated allocation, reset/release invalidation, lifetime/generation dependencies, and call-bounded mutable Arena borrow.

§ 30(3) Required failure tests include insufficient capacity, no null safe references, size-overflow handling, derived alignment, and proof-required infallibility.

§ 30(4) Required destruction/storage tests include ordinary value destruction in Arena storage, separate raw-storage reclamation, partial-initialization cleanup, and canonical storage-origin propagation.

§ 30(5) Required target tests include hosted context, fixed/preallocated Arena, noalloc rejection, proven static elimination, ISR allocation restrictions, and runtime-free bare-metal.

§ 30(6) Required IR/lowering tests include explicit allocation IR, preserved context/domain, count/size/alignment/fallibility, no backend-invented allocator, no allocation in proven allocation-free paths, and semantics-preserving allocation elimination.

§ 30(7) Required tooling tests include compiler/LSP parity, allocation-effect inspection, and mentor diagnostics.

---

## § 31 Completion criteria

§ 31(1) Frontend allocation support is complete when every Sec 0.1 allocation-capable operation is classified, validated, and connected to a canonical allocation context/effect without hidden escape promotion.

§ 31(2) Interprocedural allocation analysis is complete when direct, indirect, generic, interface, recursive, imported, and separately compiled call relationships carry validated allocation-effect/context summaries.

§ 31(3) Arena integration is complete when allocation-domain identity, capacity, generation, backing storage, reset/release, lifetime, borrow, ownership, and destruction semantics are represented canonically rather than by lexical-name approximations.

§ 31(4) Semantic IR support is complete when every dynamic allocation surviving Sema has explicit verifiable IR representation with required context and result facts.

§ 31(5) Lowering support is complete when maintained backends preserve allocation domains, failure, layout, lifetime, and cleanup semantics and validly eliminate allocations only after proof.

§ 31(6) Target-policy support is complete when hosted, embedded/fixed-Arena, no-allocation, ISR, and other canonical capability policies use shared allocation facts.

§ 31(7) Tooling support is complete when compiler, LSP, `sec analyse`, governance summaries, and incremental analysis agree on allocation behavior.

§ 31(8) Allocation must not be marked fully implemented merely because `Arena.Alloc` parses or one frontend generation check exists.

---

## § 32 Core summary

§ 32(1) Dynamic allocation is explicit in the semantic operation; Arena/allocation-context selection is normally implicit.

§ 32(2) Sec must not silently allocate during copy, move, borrow, parameter passing, return, reference creation, or escape repair.

§ 32(3) Arena is the default Sec 0.1 dynamic-allocation domain, while general allocation semantics remain distinct from Arena-specific APIs.

§ 32(4) Storage ownership, value ownership, initialization, destruction, reference validity, and reclamation are separate concepts.

§ 32(5) Allocation failure uses ordinary typed error handling.

§ 32(6) Allocation context and effects are compiler-visible and must be resolved before backend lowering.

§ 32(7) Target/profile policy may restrict or eliminate dynamic allocation without changing Sec ownership, lifetime, reference, or destruction semantics.

§ 32(8) Runtime-free and allocation-free Sec targets remain first-class supported designs.
