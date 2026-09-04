# Arena

- Status: Normative
- Created: 2026-09-02
- Last updated: 2026-09-02
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/arena.md`
- Replaces: previous revision of `rules/memory/arena.md`
- Repository baseline reviewed: `814a584` (latest publicly verifiable `main`; current `main` contents reviewed 2026-09-02)

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the programmer-visible `Arena` type and Arena-specific allocation-domain semantics.

§ 1(2) It defines:

```text
Arena ownership;
ArenaDomain identity;
owned, borrowed, static, and target-provided backing;
fixed and growable Arenas;
capacity and alignment;
typed safe allocation;
allocation failure;
initialization;
Reset;
Release;
Arena destruction;
validity epochs;
nested Arenas;
task/thread dependencies;
allocation-context interaction;
ordered Arena effects;
capacity-demand analysis;
Semantic IR requirements;
Sec MLIR requirements;
lowering and optimization constraints;
target-profile restrictions;
diagnostics;
tooling;
required tests.
```

§ 1(3) `rules/memory/allocation.md` owns general allocation semantics, allocation effects, no-hidden-allocation rules, and allocation-context policy.

§ 1(4) This Arena rulebook owns the Arena-specific realization of those general allocation semantics.

§ 1(5) `storage.md` owns canonical storage origin/domain/backing/address-stability/memory-space semantics.

§ 1(6) `reference_model.md` owns safe-reference validity, storage identity, invalidation domains, epochs/generations, stable handles, weak handles, and profile-selected representation.

§ 1(7) `references.md`, `borrowing.md`, and `lifetime_analysis.md` own source reference, borrow, and lifetime rules.

§ 1(8) `ownership.md`, `copy_move.md`, and `destruction.md` own ordinary ownership, moves, cleanup, and destruction.

§ 1(9) `layout.md` owns `SizeOf`, `AlignOf`, alignment, stride, representation validity, and plan-resolved layout.

§ 1(10) Effect/call-graph rulebooks own transitive effect inference and call relationships.

§ 1(11) Concurrency rulebooks own task/thread lifecycle, synchronization, cancellation, and structured concurrency.

§ 1(12) FFI/platform/interrupt rulebooks own foreign retention, providers, MMIO, target memory spaces, and ISR restrictions.

§ 1(13) This rulebook introduces no mandatory general runtime.

### § 1.1 Canonical cross-reference index

§ 1.1(1) Arena semantics must remain synchronized with the following canonical
rulebooks. These references identify authority; they do not move normative
Arena behavior out of this document.

| Concern | Canonical rulebooks |
|---|---|
| Allocation, storage, initialization, and layout | `rules/memory/allocation.md`, `rules/memory/storage.md`, `rules/memory/layout.md`, `rules/types/default_values.md` |
| Ownership, transfer, cleanup, and unsafe boundaries | `rules/memory/ownership.md`, `rules/memory/copy_move.md`, `rules/memory/destruction.md`, `rules/memory/unsafe.md` |
| References, borrowing, provenance, and lifetime | `rules/memory/references.md`, `rules/memory/reference_model.md`, `rules/memory/borrowing.md`, `rules/memory/lifetime_analysis.md` |
| Compiler-known surface and compiler pipeline | `rules/compiler/compiler_known_members.md`, `rules/compiler/compiler_analysis.md`, `rules/compiler/compiler_pipeline.md`, `rules/compiler/semantic_ir.md`, `rules/foundations/attributes.md` |
| Effects, reachability, escape, panic, and runtime checks | `rules/analysis/effect_analysis.md`, `rules/analysis/call_graph.md`, `rules/analysis/escape_analysis.md`, `rules/errors/panic.md`, `rules/errors/runtime_checks.md` |
| Tasks, threads, cancellation, and concurrency memory semantics | `rules/concurrency/concurrency.md`, `rules/concurrency/concurrency_memory_model.md`, `rules/concurrency/tasks.md`, `rules/concurrency/spawn.md`, `rules/concurrency/threads.md`, `rules/concurrency/cancellation.md`, `rules/concurrency/structured_concurrency.md` |
| FFI, targets, and interrupt restrictions | `rules/platform/ffi.md`, `rules/platform/target_profiles.md`, `rules/analysis/isr_analysis.md` |
| Diagnostics and language tooling | `rules/tooling/diagnostics.txt`, `rules/tooling/lsp.md` |

§ 1.1(2) If a referenced rulebook is revised, it must consume ArenaDomain,
backing, epoch, ownership, dependency, allocation-context, and lifecycle facts
from this rulebook rather than redefining them.

---

## § 2 Core rule

§ 2(1) An `Arena` is a move-only programmer-visible owner/controller of one allocation domain.

§ 2(2) The compiler-visible identity of that domain is an `ArenaDomain`.

§ 2(3) An `ArenaDomain` is independent of the physical address of its backing storage.

§ 2(4) Allocations produced from an Arena belong to:

```text
one ArenaDomain;
the Arena's current validity epoch;
one concrete bounded allocation extent;
one storage/memory-space contract;
one allocation site.
```

§ 2(5) Ordinary Arena allocation does not advance the Arena validity epoch.

§ 2(6) `Reset()` ends the current allocation epoch and starts a new epoch while preserving the ArenaDomain.

§ 2(7) `Release()` terminates the ArenaDomain.

§ 2(8) Arena destruction performs terminal Release semantics when the Arena is still owned and live.

§ 2(9) Arena allocation is bulk-reclamation storage management, not per-allocation deallocation.

---

## § 3 Non-goals

§ 3(1) Sec 0.1 Arena does not define:

```text
garbage collection;
reference counting;
individual Arena allocation Free;
automatic heap promotion;
relocating compaction;
automatic trimming on Reset;
general destructor registry;
general placement construction;
general safe uninitialized typed storage;
public raw access to the complete Arena backing;
concurrent Arena mutation;
lock-free Arena mutation;
self-referential Arena owner/result wrappers;
runtime reflection over Arena allocations;
universal allocator ABI;
universal Arena ABI.
```

§ 3(2) Future specialized APIs may add separate semantics without changing this base Arena contract.

---

## § 4 Terminology

### § 4.1 Arena

§ 4.1(1) `Arena` is the programmer-visible move-only semantic builtin controlling one ArenaDomain.

### § 4.2 ArenaDomain

§ 4.2(1) `ArenaDomain` is the compiler-visible identity of one live Arena allocation domain.

§ 4.2(2) It is not source-level region syntax.

§ 4.2(3) It remains stable across Arena ownership moves.

§ 4.2(4) It remains stable across ordinary allocation and Reset.

§ 4.2(5) It ends at Release/destruction.

### § 4.3 Backing storage

§ 4.3(1) Backing storage is the physical storage from which Arena allocations are produced.

### § 4.4 Arena state version

§ 4.4(1) Arena state version is the compiler SSA state after a mutating Arena operation.

§ 4.4(2) State version is distinct from validity epoch.

### § 4.5 Validity epoch

§ 4.5(1) The Arena validity epoch identifies one live incarnation of allocations in the ArenaDomain.

§ 4.5(2) Ordinary allocation changes state version but not epoch.

§ 4.5(3) Reset changes state version and epoch.

§ 4.5(4) Release ends the domain rather than merely advancing the epoch.

### § 4.6 Allocation context

§ 4.6(1) Allocation context is compiler-visible semantic state through which implicit allocation-capable operations obtain storage.

§ 4.6(2) It is not an automatically visible source variable.

### § 4.7 Arena dependency

§ 4.7(1) An Arena dependency is a compiler-visible requirement that an ArenaDomain and, where applicable, one validity epoch remain live.

---

## § 5 `Arena` is a semantic builtin

§ 5(1) `Arena` is a compiler-known semantic builtin type.

§ 5(2) Its existence does not require an ordinary source declaration.

§ 5(3) The compiler owns:

```text
member existence;
member signatures;
ownership behavior;
ArenaDomain identity;
effect classification;
dependency semantics;
Semantic IR meaning;
target/profile requirements.
```

§ 5(4) Core/target libraries may provide helper implementations.

§ 5(5) Helper implementation details must not redefine Arena semantics.

---

## § 6 Lowercase `arena`

§ 6(1) Lowercase `arena` is not a Sec 0.1 keyword through this rulebook.

§ 6(2) It is an ordinary identifier.

Valid:

```sec
let mut arena := try Arena.WithCapacity(4096)
```

§ 6(3) A lexer/parser implementation must not reserve lowercase `arena` unless another canonical rulebook later assigns syntax to it.

§ 6(4) This rulebook introduces no scoped `arena { ... }` syntax.

---

## § 7 Arena ownership

§ 7(1) `Arena` is move-only and non-copyable.

§ 7(2) Copying an Arena would create multiple apparent owners/controllers of one allocation domain and is invalid.

Invalid:

```sec
let second := first
```

when `first: Arena`.

§ 7(3) Ownership move is valid using the canonical move syntax.

Conceptually:

```sec
let second :<- first
```

§ 7(4) After the move:

```text
second owns the same ArenaDomain;
the same validity epoch remains current;
the same backing remains controlled;
the same allocation state continues;
first becomes unavailable.
```

§ 7(5) Moving an Arena does not invalidate Arena-backed references.

§ 7(6) Moving an Arena does not create a new ArenaDomain.

§ 7(7) Moving an Arena need not perform a runtime operation.

---

## § 8 Arena composition

§ 8(1) A type containing an Arena is move-only by composition unless another canonical representation abstracts Arena ownership away.

Example:

```sec
type WorkerStorage struct {
    arena: Arena
}
```

§ 8(2) Moving the containing value transfers Arena ownership.

§ 8(3) Destruction of the containing value destroys the Arena field if still owned.

§ 8(4) Field destruction order follows `destruction.md`.

---

## § 9 Allocation-domain ownership versus backing ownership

§ 9(1) An Arena always owns/controls its ArenaDomain.

§ 9(2) It does not necessarily own the physical backing bytes.

§ 9(3) Arena backing relation is represented through the canonical storage model.

§ 9(4) Arena-specific backing categories include conceptually:

```text
Owned
Borrowed
Static
TargetProvided
```

§ 9(5) These categories describe how the Arena obtains/releases backing and map onto canonical storage/backing/reclamation facts.

§ 9(6) They are not replacement `StorageOrigin` values.

§ 9(7) A growable Arena may control multiple stable backing segments.

---

## § 10 Borrowed fixed Arena

§ 10(1) Canonical source form:

```sec
let mut arena := Arena.FromBuffer(ref mut buffer)
```

§ 10(2) Conceptual declaration:

```sec
fn FromBuffer(buffer: ref mut byte[]) Arena
```

§ 10(3) `FromBuffer` creates a new ArenaDomain using the supplied mutable contiguous storage as fixed backing.

§ 10(4) It is allocation-free with respect to acquiring new backing.

§ 10(5) It is infallible when the supplied safe view already satisfies all static requirements.

§ 10(6) It takes exclusive authority over allocation use of that backing for the Arena lifetime.

§ 10(7) It does not deallocate/destroy the backing owner when the Arena ends.

§ 10(8) The supplied storage must be:

```text
mutable;
contiguous;
addressable;
live for the complete Arena lifetime;
representably bounded;
compatible with required address space;
capable of satisfying requested allocation alignments.
```

§ 10(9) Direct conflicting use of the borrowed backing is invalid while the Arena controls it.

---

## § 11 Empty borrowed Arena

§ 11(1) A zero-length mutable backing view may create a valid borrowed Arena.

§ 11(2) Such an Arena:

```text
is live;
has zero byte capacity;
may satisfy zero-element allocations;
cannot satisfy positive-storage allocation;
may Reset;
may Release.
```

§ 11(3) A statically obvious zero-capacity Arena may produce configurable informational diagnostics.

§ 11(4) Zero capacity is not itself a semantic error.

---

## § 12 Owned fixed Arena

§ 12(1) Canonical source form:

```sec
let mut arena := try Arena.WithCapacity(4096)
```

§ 12(2) Conceptual declaration:

```sec
fn WithCapacity(capacity: uint) Result[Arena, AllocationError]
```

§ 12(3) `WithCapacity` creates a fresh ArenaDomain.

§ 12(4) It requests owned backing through the selected target/profile provider.

§ 12(5) It creates fixed capacity and does not grow.

§ 12(6) It may fail.

§ 12(7) The constructor is available only when the active `CompilationPlan` provides a compatible backing provider.

§ 12(8) Provider effects and trust provenance remain visible to analysis/call graph.

§ 12(9) Arena destruction/Release returns owned backing through the matching provider contract.

---

## § 13 Growable Arena

§ 13(1) Sec 0.1 defines growable Arena semantics.

Canonical source form where exposed:

```sec
let mut arena := try Arena.Growable(4096)
```

Conceptual declaration:

```sec
fn Growable(initialCapacity: uint) Result[Arena, AllocationError]
```

§ 13(2) A growable Arena may acquire additional backing only through a strategy preserving every existing live allocation address and validity guarantee.

§ 13(3) Permitted strategies include:

```text
additional stable segments;
reserved virtual address space;
target-provided non-relocating extension;
another CompilationPlan-defined stable strategy.
```

§ 13(4) The following strategy is forbidden while prior allocations remain live:

```text
allocate a larger buffer;
copy old Arena allocations;
change their addresses;
continue using old references as if nothing changed.
```

§ 13(5) Growth preserves:

```text
ArenaDomain identity;
current validity epoch;
prior allocation addresses;
prior allocation bounds;
prior safe-reference validity;
prior ownership/dependency facts.
```

§ 13(6) Growth is not Arena Reset.

---

## § 14 Growth policy

§ 14(1) Growth policy is selected by constructor, target profile, `CompilationPlan`, or compiler-managed allocation-context policy.

§ 14(2) It is fixed for the live ArenaDomain.

§ 14(3) Policy may include:

```text
initial capacity;
next-segment policy;
maximum capacity;
provider;
alignment constraints;
memory-space/address-space constraints.
```

§ 14(4) Sec 0.1 does not require public source syntax for every policy field.

§ 14(5) A bounded growable Arena becomes effectively fixed after its maximum capacity is acquired.

---

## § 15 Capacity

§ 15(1) Arena capacity is measured in bytes.

§ 15(2) Compiler-visible quantitative facts may include:

```text
reserved capacity;
used capacity;
current-segment capacity;
total acquired growable capacity;
maximum capacity;
alignment padding;
peak logical demand.
```

§ 15(3) This rulebook does not require public properties for all such facts.

§ 15(4) Public compiler-known member inventory is owned by `compiler_known_members.md`.

---

## § 16 Typed allocation count

§ 16(1) `Arena.Alloc[T](count)` interprets `count` as element count, not byte count.

Example:

```sec
let values := try arena.Alloc[int32](100)
```

§ 16(2) Required payload bytes are conceptually:

```text
CheckedMultiply(count, SizeOf(T))
```

§ 16(3) Allocation start is aligned according to `AlignOf(T)`.

§ 16(4) Alignment padding consumes Arena capacity.

§ 16(5) A fixed-arena request is conceptually valid when:

```text
alignedOffset = AlignUp(currentOffset, AlignOf(T))
payloadSize   = CheckedMultiply(count, SizeOf(T))
endOffset     = CheckedAdd(alignedOffset, payloadSize)

endOffset <= capacity
```

§ 16(6) Every arithmetic operation affecting capacity/offset is checked.

---

## § 17 Alignment

§ 17(1) Every Arena allocation satisfies the canonical layout/alignment of `T` under the active `CompilationPlan`.

§ 17(2) Arena implementation must not assume:

```text
SizeOf(T) == AlignOf(T);
every target alignment is a power of two;
cursor is already aligned;
all backing bases satisfy every possible T alignment.
```

§ 17(3) `FromBuffer` can allocate `T` only where backing base/range permits required alignment.

§ 17(4) Dynamic inability to satisfy alignment returns `AllocationError`.

§ 17(5) Statically impossible alignment is a compile-time error.

---

## § 18 Safe typed allocation

§ 18(1) Canonical safe typed operations are:

```sec
let value := try arena.New[Value]()
let values := try arena.Alloc[Value](count)
```

Conceptual declarations:

```sec
fn New[T]() Result[ref mut T, AllocationError]

fn Alloc[T](count: uint) Result[ref mut T[], AllocationError]
```

§ 18(2) These are compiler-known instance operations.

§ 18(3) They temporarily require mutable authority over Arena control state.

§ 18(4) The temporary mutable borrow of Arena control state ends when the allocation operation returns.

§ 18(5) The returned reference/slice retains a storage dependency on the ArenaDomain/current epoch.

§ 18(6) It does not retain an exclusive borrow of the entire Arena control value.

§ 18(7) Repeated Arena allocations are therefore permitted when capacity remains and no conflicting operation exists.

---

## § 19 Type requirements

§ 19(1) Safe `New[T]`/`Alloc[T]` require:

```text
complete layout under active CompilationPlan;
sized T;
known supported alignment;
valid compiler-defined default;
safe initialization;
trivial destruction classification.
```

§ 19(2) Exact layout/default/destruction rules remain owned by their canonical rulebooks.

§ 19(3) These safe parameterless/default allocation forms do not accept types lacking a valid infallible default.

§ 19(4) These safe forms do not directly allocate non-trivially-destructible `T` in Sec 0.1.

---

## § 20 `Arena.New[T]`

§ 20(1) `New[T]` allocates storage for exactly one `T`.

§ 20(2) It:

```text
validates T/layout/alignment;
checks capacity or performs valid growth;
fully initializes one T;
returns ref mut T;
never exposes uninitialized T;
returns AllocationError on allocation failure.
```

§ 20(3) The result is:

```text
non-null;
bounded to one T;
associated with the current ArenaDomain;
associated with the current epoch;
mutable;
non-owning with respect to backing storage;
not individually deallocatable.
```

---

## § 21 `Arena.Alloc[T]`

§ 21(1) `Alloc[T](count)` allocates exactly `count` values.

§ 21(2) It:

```text
checks count arithmetic;
checks layout/alignment;
checks capacity or performs valid non-relocating growth;
fully initializes every element;
returns ref mut T[];
never returns a shorter slice;
never exposes partial initialization;
returns AllocationError on allocation failure.
```

§ 21(3) The returned slice:

```text
has exactly count elements;
has bounds matching the complete allocation;
belongs to the current ArenaDomain/epoch;
has mutable authority;
does not own/reclaim backing.
```

---

## § 22 Zero-element allocation

§ 22(1) `Alloc[T](0)` is valid.

Example:

```sec
let values := try arena.Alloc[int](0)
```

§ 22(2) It:

```text
succeeds;
returns a valid empty mutable slice;
consumes no capacity;
requires no growth;
creates no dereferenceable element;
does not advance epoch.
```

§ 22(3) Literal zero count is not inherently suspicious.

§ 22(4) General useless-operation diagnostics may still apply when appropriate.

---

## § 23 Zero-sized types

§ 23(1) Layout rules own whether a concrete zero-sized `T` is valid.

§ 23(2) When `SizeOf(T) == 0` is valid:

```text
Alloc[T](count) may consume no payload bytes;
the result retains semantic count;
element identity/bounds semantics must remain valid;
no dereferenceable byte location is implied merely by count.
```

§ 23(3) Arena does not independently create zero-sized type semantics.

---

## § 24 Full initialization

§ 24(1) Safe Arena typed allocation exposes only fully initialized semantic values.

§ 24(2) Full initialization means:

```text
every result value satisfies canonical default semantics;
every readable field/element is valid;
required type invariants are established.
```

§ 24(3) It does not require:

```text
zeroing every physical byte;
defined padding contents;
bulk memset;
all-zero representation.
```

§ 24(4) Compiler implementation may use field stores, loops, vectorized stores, target intrinsics, valid bulk zeroing, or proof-driven store elimination.

---

## § 25 Default initialization

§ 25(1) The default used by `New[T]`/`Alloc[T]` must be infallible.

§ 25(2) Allocation may fail independently.

§ 25(3) A missing/invalid default rejects these safe allocation forms.

§ 25(4) A future explicit initializer/placement-construction operation requires separate canonical semantics.

---

## § 26 Trivial-destruction restriction

§ 26(1) Safe `New[T]`/`Alloc[T]` require `T` to be trivially destructible in Sec 0.1.

§ 26(2) The reason is that Arena performs bulk reclamation and is not an implicit registry of arbitrary destructors.

§ 26(3) The Arena does not automatically store:

```text
destructor callables;
per-allocation type metadata;
element counts for destruction;
per-allocation initialization masks;
destruction ordering records.
```

§ 26(4) A type owning files, locks, sockets, owning collections, custom-free resources, or other nontrivial lifecycle state is rejected by the direct safe forms unless its canonical classification is trivial.

---

## § 27 Owning containers using Arena backing

§ 27(1) The trivial-destruction restriction does not forbid an owning collection/container from using Arena-backed element/raw storage when its own rulebook defines the lifecycle protocol.

§ 27(2) Responsibilities remain distinct:

```text
Arena:
    controls storage domain/backing

owning container:
    owns initialized logical elements
    tracks element count
    performs required element destruction
    ends dependencies before Reset/Release
```

§ 27(3) Arena never silently takes over logical element ownership/destruction.

---

## § 28 Uninitialized storage

§ 28(1) General safe uninitialized typed Arena allocation is not part of Sec 0.1.

§ 28(2) An Arena operation must not expose uninitialized storage directly as ordinary:

```text
ref mut T
ref mut T[]
```

§ 28(3) A future API must use an explicit uninitialized-storage abstraction or unsafe operation.

§ 28(4) Safe `ref`/slice may be produced only after initialization validity is established.

§ 28(5) Internal/compiler raw byte allocation is not ordinary safe typed Arena allocation.

---

## § 29 Allocation failure

§ 29(1) Arena creation/allocation failure uses `AllocationError`.

§ 29(2) Exact variants belong to canonical error/core rules.

§ 29(3) Arena creation/allocation must not silently:

```text
return null;
return invalid reference;
return shorter slice;
expose partial initialization;
fall back to unrelated allocator/heap;
select another Arena;
panic merely because capacity is exhausted;
terminate process merely because capacity is exhausted.
```

§ 29(4) Caller handles/propagates failure using normal `Result`/`try`.

---

## § 30 Stable source result type

§ 30(1) `New[T]`/`Alloc[T]` retain `Result` in their source signature even if compiler proof removes the dynamic failure path.

§ 30(2) Proof may eliminate runtime checks/error construction in Semantic IR/lowering.

§ 30(3) Proof does not rewrite the public callable type.

§ 30(4) This stability applies across generics, interfaces, function values, module metadata, callable contracts, and separate compilation.

---

## § 31 Allocation atomicity

§ 31(1) Arena allocation is atomic from Sec program semantics.

§ 31(2) Conceptual sequence:

```text
validate Arena state;
validate T;
compute checked payload size;
compute alignment padding;
ensure complete fixed/growth capacity;
reserve complete range;
fully initialize all result values;
publish reference/slice.
```

§ 31(3) If failure occurs before publication:

```text
Arena remains live;
ArenaDomain unchanged;
epoch unchanged;
allocation cursor unchanged;
prior allocations remain valid;
no partial result is published;
no partially initialized typed value becomes observable.
```

§ 31(4) Allocation failure therefore does not partially consume the requested capacity.

---

## § 32 Growable allocation failure

§ 32(1) If growth requires a new backing segment and provider acquisition fails:

```text
allocation returns Err;
prior Arena physical state remains equivalent to input;
existing segments remain linked/valid;
existing allocations remain valid;
no partial segment becomes visible;
epoch remains unchanged.
```

§ 32(2) A new segment joins the Arena only after successful complete acquisition.

§ 32(3) Provider effects/failure remain observable according to effect/error rules.

---

## § 33 Repeated allocation

§ 33(1) Successful allocation does not invalidate earlier allocations in the current epoch.

Example:

```sec
let first := try arena.Alloc[byte](100)
let second := try arena.Alloc[byte](200)
```

§ 33(2) Positive-sized allocation ranges are non-overlapping.

§ 33(3) They belong to the same ArenaDomain and normally current epoch.

§ 33(4) Compiler alias analysis may consume these proven non-overlap facts.

§ 33(5) Zero-sized/zero-element cases do not imply a dereferenceable disjoint byte range.

---

## § 34 No individual reclamation

§ 34(1) Ending a reference live range does not reclaim Arena bytes.

§ 34(2) Storage consumed by successful allocation remains consumed in the current epoch until Reset/Release or another future explicitly defined bulk reclamation operation.

§ 34(3) Sec 0.1 defines no `Arena.Free(allocation)`.

---

## § 35 `Reset()`

§ 35(1) Canonical source form:

```sec
arena.Reset()
```

Conceptual declaration:

```sec
fn Reset() void
```

§ 35(2) `Reset()`:

```text
keeps Arena owner live;
keeps same ArenaDomain;
ends current validity epoch;
invalidates prior allocations;
resets allocation cursors;
retains reusable backing;
starts a new validity epoch;
allows later allocation.
```

§ 35(3) Reset is not Release.

§ 35(4) Reset is an ordinary mutable Arena operation, not a consuming whole-self operation.

---

## § 36 Reset requirements

§ 36(1) Reset is valid only when no validity-preserving dependency crosses the Reset point.

§ 36(2) Relevant dependencies include:

```text
ordinary shared/mutable references;
slices/views;
nested Arenas;
containers using Arena backing;
closure captures;
task captures;
thread arguments;
deferred operations;
foreign-retained dependencies;
strong validity-preserving handles;
returned/result values still depending on Arena storage.
```

§ 36(3) NLL/final-use analysis determines whether a dependency still crosses Reset.

Valid:

```sec
let values := try arena.Alloc[int](100)
Process(values)

arena.Reset()
```

when `Process(values)` is the final use.

Invalid:

```sec
let values := try arena.Alloc[int](100)
arena.Reset()
Process(values)
```

§ 36(4) Lexical scope alone does not keep a dead reference dependency alive.

---

## § 37 Reset and stale-capable handles

§ 37(1) Weak or explicitly stale-capable handles may survive Reset when their contract permits fallible resolution.

§ 37(2) Such handle identities do not block Reset solely because the handle value remains live.

§ 37(3) A strong validity-preserving dependency blocks Reset.

§ 37(4) After Reset, stale-capable handle resolution follows `reference_model.md`.

---

## § 38 Reset epoch

§ 38(1) Reset advances/replaces the logical Arena validity epoch.

§ 38(2) The epoch belongs to the ArenaDomain.

§ 38(3) Ordinary references from the old epoch must be dead before Reset.

§ 38(4) Stale-capable handles may retain the old expected epoch and fail resolution after Reset.

§ 38(5) Epoch increments/rekeys must never revive stale references.

§ 38(6) Default logical epoch policy follows `reference_model.md`.

§ 38(7) Runtime epoch metadata may be eliminated when proof makes it unnecessary.

---

## § 39 Arena epoch exhaustion

§ 39(1) Arena epoch exhaustion must use one canonical safe strategy:

```text
fresh distinguishable ArenaDomain identity/rekey;
explicit fallible behavior if Arena API/profile defines it;
deterministic panic/target trap when recovery is unavailable.
```

§ 39(2) Finite epoch representation must never silently wrap.

§ 39(3) Reuse must never make old references/handles match the new live incarnation.

---

## § 40 Reset capacity behavior

§ 40(1) Fixed Arena Reset sets used capacity/cursor to zero, retains total capacity/backing/domain, and advances epoch.

§ 40(2) Borrowed Arena Reset retains the exclusive borrow of backing.

§ 40(3) Borrowed backing is not returned to the owner by Reset.

§ 40(4) Growable Arena Reset retains acquired stable segments and resets their allocation cursors.

§ 40(5) Reset does not automatically trim/release extra growable segments.

§ 40(6) Future `Trim` semantics require a separate canonical operation.

---

## § 41 Reset does not zero backing

§ 41(1) Reset does not require clearing every backing byte.

§ 41(2) After Reset, prior typed object lifetimes in the Arena have ended.

§ 41(3) Later safe typed allocations must fully initialize new objects before exposure.

§ 41(4) Compiler/profile may zero backing for security/debug/performance reasons when semantically valid.

§ 41(5) Such zeroing is implementation/profile policy, not base Reset semantics.

---

## § 42 Reset atomicity

§ 42(1) Reset is atomic from program semantics.

§ 42(2) Conceptually:

```text
verify Reset is permitted;
prepare a distinguishable next epoch;
end old epoch;
reset all allocation cursors;
publish next epoch.
```

§ 42(3) No program observation may see a mixture of old/new Arena allocation state.

§ 42(4) Ordinary Arena is not concurrently mutable, so concurrent observation is independently restricted.

---

## § 43 `Release()` lifecycle operation

§ 43(1) Canonical source form:

```sec
arena.Release()
```

§ 43(2) `Release()` is a compiler-known Arena lifecycle termination operation.

§ 43(3) It uses method-form source syntax but is not an ordinary user-defined whole-self-consuming method.

§ 43(4) Therefore `Release()` does not weaken the general Sec 0.1 rule that ordinary user-defined methods do not consume whole `self`.

§ 43(5) `Release()`:

```text
consumes Arena ownership;
terminates ArenaDomain;
invalidates validity-preserving dependencies;
returns owned backing to provider where required;
ends borrowed-backing control;
permits no later use of that Arena value.
```

§ 43(6) After successful Release, the source Arena Place is unavailable.

§ 43(7) `Release()` returns `void`.

§ 43(8) Release is terminal and cannot fail in the base Arena contract; provider contracts used by safe Arena must provide a non-fallible terminal release path or expose a different resource abstraction.

---

## § 44 Release requirements

§ 44(1) No ordinary validity-preserving dependency may cross Release.

Invalid:

```sec
let values := try arena.Alloc[int](100)
arena.Release()
Process(values)
```

§ 44(2) Stale-capable weak handle identities may remain, but cannot resolve into the ended ArenaDomain.

§ 44(3) A later Arena using the same physical address receives a distinct ArenaDomain identity.

§ 44(4) Old handles/references never revive through address reuse.

---

## § 45 Release by backing relation

§ 45(1) Owned backing is returned to the provider.

§ 45(2) Borrowed backing is not deallocated; Release ends the Arena's exclusive control/borrow.

§ 45(3) Static backing is not deallocated; Release ends ArenaDomain semantics over it.

§ 45(4) Target-provided backing follows the provider/platform contract.

§ 45(5) ArenaDomain ends even if target/platform retains physical bytes.

---

## § 46 Implicit Arena destruction

§ 46(1) The programmer need not explicitly call `Release()` on every normal path.

§ 46(2) Destruction of a still-owned Arena performs terminal Release semantics.

§ 46(3) This may occur through normal scope exit, early return/error propagation cleanup, field destruction, task/thread cleanup, or another canonical ownership boundary.

§ 46(4) If explicit `Release()` already consumed the Arena, no implicit second Release occurs.

§ 46(5) Double Release is invalid.

§ 46(6) Arena's compiler-known lifecycle cleanup composes with the ordinary destruction model.

---

## § 47 Arena storage versus logical value destruction

§ 47(1) Arena controls storage; Arena-backed logical values may have separate ownership/lifecycle rules.

§ 47(2) Destroying an ordinary value does not necessarily reclaim its Arena bytes.

§ 47(3) Reset/Release do not replace required nontrivial logical destruction.

§ 47(4) The direct safe `New`/`Alloc` trivial-destruction restriction exists so bulk reclamation never silently skips required object destruction.

§ 47(5) Specialized owning containers may perform element destruction before their Arena dependency ends.

---

## § 48 Early return and escape

§ 48(1) Return expressions/results are established before local destruction according to ordinary destruction rules.

§ 48(2) A returned reference/value must not depend on a local Arena destroyed during function exit.

Invalid:

```sec
fn Invalid() Result[ref int, AllocationError] {
    let mut arena := try Arena.WithCapacity(1024)
    let value := try arena.New[int]()
    return Ok(value)
}
```

§ 48(3) Compiler must not repair this through hidden Arena/heap promotion.

§ 48(4) Arena-backed reference escape is valid only when its ArenaDomain already outlives the result through an external owner/context.

---

## § 49 Returning Arena ownership

§ 49(1) Returning the Arena owner itself is valid.

Example:

```sec
fn CreateArena() Result[Arena, AllocationError] {
    let arena := try Arena.WithCapacity(4096)
    return Ok(<- arena)
}
```

§ 49(2) Ownership moves to result.

§ 49(3) The same ArenaDomain/current epoch continues.

§ 49(4) Return of Arena ownership does not by itself invalidate Arena-backed storage.

---

## § 50 Self-referential Arena results

§ 50(1) Sec 0.1 Arena does not automatically support ordinary values bundling an Arena owner and a direct reference into that same Arena.

Example shape:

```sec
type ArenaResult struct {
    arena: Arena
    value: ref int
}
```

§ 50(2) Such a shape requires explicit self-reference/relocation ownership semantics not introduced by this rulebook.

§ 50(3) Compiler must not infer self-reference correctness from field order.

---

## § 51 Nested Arenas

§ 51(1) Nested Arenas require no special source syntax.

Example:

```sec
let childBuffer := try parent.Alloc[byte](4096)
let mut child := Arena.FromBuffer(childBuffer)
```

§ 51(2) Child has its own ArenaDomain and validity epoch.

§ 51(3) Child borrows backing from one parent allocation.

§ 51(4) Child therefore depends on parent ArenaDomain/current epoch.

§ 51(5) Parent cannot Reset/Release while child remains live.

§ 51(6) Releasing child ends child domain/borrow but does not individually reclaim `childBuffer` from parent.

§ 51(7) Later parent Reset may reclaim the parent allocation after child dependency ends.

---

## § 52 `defer`

§ 52(1) Arena operations in defer follow canonical common LIFO cleanup ordering.

§ 52(2) A deferred use of Arena-backed storage must execute before a deferred/implicit Arena release invalidates it.

Example concept:

```sec
let mut arena := try Arena.WithCapacity(4096)

defer {
    arena.Release()
}

let values := try arena.Alloc[int](100)

defer {
    FinalUse(values)
}
```

§ 52(3) LIFO order executes `FinalUse(values)` before `arena.Release()`.

§ 52(4) A cleanup plan releasing Arena before a required dependent cleanup use is invalid.

§ 52(5) Panic cleanup occurs only where `panic.md`/`destruction.md` guarantee cleanup.

---

## § 53 Cancellation

§ 53(1) Cancellation does not erase Arena dependencies until the canonical cancellation/completion boundary proves execution can no longer access the storage.

§ 53(2) Task cancellation request alone is not completion proof.

§ 53(3) Cleanup/destruction performed during cancellation must complete before a dependency is considered ended where concurrency rules require it.

§ 53(4) Arena reset/release after cancellation therefore requires canonical completion/cleanup proof.

---

## § 54 Arena and tasks

§ 54(1) Ordinary Arena is not implicitly concurrent.

§ 54(2) Spawned task does not automatically inherit the parent's mutable Arena allocation context.

§ 54(3) Hidden inheritance would create implicit concurrent mutation and is forbidden.

§ 54(4) A task obtains allocation capability through one of:

```text
task-specific target/compiler context;
Arena explicitly moved into task;
explicit task allocation contract;
no allocation capability.
```

---

## § 55 Borrowed Arena storage captured by task

§ 55(1) A task capturing an Arena-backed reference retains dependency on:

```text
ArenaDomain;
allocation epoch;
allocation bounds;
capture/borrow authority.
```

§ 55(2) Parent cannot Reset/Release while task may still access the storage.

§ 55(3) Dependency is tracked through task lifecycle and result transfer.

---

## § 56 Task completion proof

§ 56(1) Static validity uses semantic completion boundaries, not physical timing guesses.

§ 56(2) Valid boundaries may include:

```text
await;
join;
structured-concurrency scope completion;
another compiler-proven completion event.
```

§ 56(3) Runtime observation that a task likely/actually finished without the canonical synchronization/lifecycle boundary is insufficient for static dependency release.

---

## § 57 `await` and Arena dependency

§ 57(1) `await` proves canonical task completion according to task rules.

§ 57(2) Execution-local captures/dependencies end after task completion cleanup.

§ 57(3) Result dependencies transfer to the continuation.

§ 57(4) Reset may become valid after `await` only when no Arena dependency remains in the returned result or elsewhere.

Example:

```sec
let task := try spawn Process(values)
discard await task

arena.Reset()
```

§ 57(5) `await` does not call task body again.

---

## § 58 `join` and Arena dependency

§ 58(1) `join` may prove task/thread completion while retaining an observer/handle according to concurrency rules.

§ 58(2) Execution-local Arena dependency may end at join.

§ 58(3) A retained handle does not keep Arena storage live unless it stores/owns a result/dependency or its contract explicitly does so.

---

## § 59 Task result dependencies

§ 59(1) Task completion ends execution-local captures.

§ 59(2) Arena dependencies transferred through a task result remain live.

Example concept:

```sec
let task := try spawn Find(values)
let outcome := await task

Use(outcome)

arena.Reset()
```

§ 59(3) Reset is valid only after final use of `outcome` if its active
`Completed` payload references the Arena. The `TaskOutcome[T]` wrapper follows
ordinary union/payload ownership and does not erase that dependency.

§ 59(4) Completion alone does not erase result-borne dependency.

---

## § 60 Moving Arena into a task

§ 60(1) Arena may be transferred exclusively to a task according to transferability/concurrency rules.

Example:

```sec
let task := try spawn Worker(<-arena)
```

§ 60(2) After committed spawn transfer, parent no longer owns Arena.

§ 60(3) Task owns Arena.

§ 60(4) Task destroys Arena at completion unless ownership is transferred into result.

§ 60(5) ArenaDomain remains the same across the ownership transfer.

---

## § 61 Task completion point

§ 61(1) Arena execution-local dependencies end only after the canonical task completion point.

§ 61(2) That point occurs after:

```text
task body ends;
result is moved into outcome storage;
required defers run where guaranteed;
required local destruction runs;
task-owned Arenas are released unless transferred;
task can no longer access captured storage.
```

§ 61(3) Await/join/structured concurrency observe this semantic completion point.

---

## § 62 Arena and threads

§ 62(1) Thread dependency rules mirror task rules plus physical-thread transfer/synchronization requirements.

§ 62(2) Borrowing Arena-backed storage into another physical thread requires proof of:

```text
storage lifetime beyond thread execution;
valid access authority;
exclusive mutation where needed;
no conflicting Arena mutation;
no Reset/Release during use;
transferability;
memory synchronization;
address-space compatibility;
target thread support.
```

---

## § 63 Thread completion

§ 63(1) Canonical thread completion/join proves execution ended and required synchronization occurred.

§ 63(2) It may end execution-local Arena dependencies.

§ 63(3) Reset/Release becomes valid only if no result/other dependency remains.

§ 63(4) Physical thread termination without a canonical observed completion boundary is insufficient where synchronization/dependency proof is required.

---

## § 64 Moving Arena into a thread

§ 64(1) Arena may be transferred to a physical thread only when:

```text
backing is transferable;
provider permits destination-thread use/release;
target/profile permits transfer;
no parent dependency conflicts;
Arena does not rely on immovable thread-local state.
```

§ 64(2) Destination thread owns/destructs Arena unless ownership is transferred onward/result.

---

## § 65 Structured concurrency

§ 65(1) A structured-concurrency scope may discharge child Arena dependencies when it proves:

```text
all relevant children completed;
required cleanup completed;
no detached child retained dependency;
no result transferred dependency outside scope.
```

§ 65(2) Detached child prevents the scope itself from proving completion for that child's retained Arena dependency.

---

## § 66 Concurrent Arena access

§ 66(1) Ordinary `Arena` does not support concurrent mutation.

§ 66(2) Forbidden without a distinct synchronization abstraction include:

```text
concurrent New/Alloc;
concurrent Reset;
concurrent Release;
allocation concurrent with Reset/Release;
Reset/Release concurrent with access requiring old epoch;
parent/child Arena mutation that conflicts through shared backing.
```

§ 66(3) Ownership/borrow/concurrency analyses reject these cases.

§ 66(4) Future synchronized/concurrent Arena must be a distinct explicit abstraction/contract.

---

## § 67 Allocation context

§ 67(1) Each callable invocation has zero or one active allocation context according to `allocation.md`.

§ 67(2) Allocation context is compiler-visible semantic state.

§ 67(3) It is not:

```text
source global;
automatically visible Arena variable;
universal thread-local allocator;
implicit heap;
mandatory runtime object.
```

§ 67(4) Arena is a primary concrete allocation-domain realization but allocation context and explicit Arena values are distinct concepts.

---

## § 68 `MayAllocate` versus `RequiresAllocationContext`

§ 68(1) `MayAllocate` and `RequiresAllocationContext` are separate facts.

§ 68(2) `MayAllocate` means reachable execution may perform allocation.

§ 68(3) `RequiresAllocationContext` means a callable reaches implicit allocation that needs an ambient context.

Example:

```sec
fn Fill(arena: ref mut Arena) Result[ref mut byte[], AllocationError] {
    return arena.Alloc[byte](1024)
}
```

§ 68(4) `Fill` may allocate using explicit Arena without requiring ambient allocation context.

§ 68(5) An allocating string/collection operation without explicit Arena may require ambient allocation context.

---

## § 69 Synchronous allocation-context propagation

§ 69(1) A synchronous call propagates the active allocation context when the callee requires it.

Conceptually:

```text
A(context X)
    -> B(context X)
        -> C(context X)
```

§ 69(2) A callable that does not require context needs no mandatory runtime context argument.

§ 69(3) Propagation may be compile-time-only when no runtime parameter is necessary.

---

## § 70 Explicit Arenas are not ambient candidates

§ 70(1) Compiler must not guess among ordinary Arena values in lexical scope.

Example:

```sec
let temporaryArena := ...
let outputArena := ...

let result := try Build()
```

§ 70(2) Compiler must not choose either variable as ambient context merely because it is visible.

§ 70(3) Passing an explicit Arena as an ordinary argument does not automatically rebind ambient allocation context.

---

## § 71 Allocation-context selection

§ 71(1) General allocation-context selection order is owned by `allocation.md`.

§ 71(2) Arena-specific realization recognizes conceptually:

```text
explicit Arena selected by operation;
propagated ambient context;
compiler-managed local Arena with proven backing/non-escape;
target-provided context;
no valid context -> compile-time error.
```

§ 71(3) Selection occurs before physical lowering.

§ 71(4) Backend cannot change allocation origin/failure semantics.

---

## § 72 Compiler-managed local Arena

§ 72(1) Compiler-managed local Arena is permitted only when:

```text
concrete backing strategy exists;
required lifetime is proven;
references do not illegally escape;
failure semantics remain correct;
selected profile permits it.
```

§ 72(2) Possible backing includes:

```text
bounded automatic storage;
static storage;
caller-context-backed stable storage;
target-provided local storage.
```

§ 72(3) Compiler must not silently move escaping local storage into longer-lived Arena/heap storage to repair lifetime.

---

## § 73 Escaping allocation-context results

§ 73(1) A result depending on ambient/local Arena storage may escape only when the context/domain lifetime relation guarantees the result.

§ 73(2) Compiler-managed local Arena cannot be selected for an allocation whose non-owning reference escapes beyond that local domain.

§ 73(3) Hidden lifetime extension/promotion is forbidden.

---

## § 74 Spawned allocation contexts

§ 74(1) Spawned task/thread is a new execution context.

§ 74(2) It does not automatically receive parent's mutable Arena context.

§ 74(3) Its context comes from task/thread profile, target-provided context, explicitly transferred Arena, explicit execution contract, or no allocation capability.

§ 74(4) Process receives separate address-space allocation context.

§ 74(5) ISR has no ordinary allocation context unless interrupts/target rules explicitly provide and permit one.

---

## § 75 Foreign entrypoints

§ 75(1) Foreign code does not automatically supply a Sec allocation context.

§ 75(2) Exported Sec callable requiring one needs an explicit Sec-aware ABI/wrapper/target-provided context or export is rejected.

§ 75(3) Hidden Sec allocation-context parameters must not be added to ordinary foreign ABI without explicit contract.

---

## § 76 Arena ordered effects

§ 76(1) Arena operations contribute ordered semantic effects/events.

Conceptual identities:

```text
ArenaCreate(A, policy)
ArenaAllocate(A, size)
ArenaReset(A)
ArenaRelease(A)
```

§ 76(2) Order is semantically significant.

§ 76(3) Effects are associated with ArenaDomain identity.

§ 76(4) They are not reducible to one unordered boolean.

---

## § 77 Operation effect summaries

§ 77(1) `Arena.FromBuffer` contributes Arena creation but not backing `MayAllocate`.

§ 77(2) `Arena.WithCapacity` contributes Arena creation and `MayAllocate`, plus provider effects.

§ 77(3) `Arena.Growable` contributes Arena creation and `MayAllocate`; later growth contributes provider effects.

§ 77(4) `Arena.New`/`Arena.Alloc` contribute Arena allocation and `MayAllocate`, even when backing capacity already exists.

§ 77(5) `Arena.Reset` contributes ArenaReset.

§ 77(6) `Arena.Release` contributes ArenaRelease and any required provider-release effects.

---

## § 78 `@noAlloc`

§ 78(1) Arena typed allocation is allocation for canonical effect semantics.

§ 78(2) `@noAlloc` forbids reachable `Arena.New`/`Arena.Alloc` under current Sec 0.1 semantics.

§ 78(3) `FromBuffer` does not itself add backing allocation.

§ 78(4) This rulebook introduces no separate `@noBackingAlloc`.

§ 78(5) Profile/static lowering of Arena operations does not retroactively change source-level allocation effect unless canonical effect rules explicitly define proof-based guarantee semantics.

---

## § 79 Ordered-effect validity

§ 79(1) Compiler must reject invalid ordered Arena sequences.

Invalid:

```text
ArenaRelease(A)
ArenaAllocate(A, size)
```

Valid concept:

```text
ArenaCreate(A)
ArenaAllocate(A)
ArenaReset(A)
ArenaAllocate(A)
ArenaRelease(A)
```

§ 79(2) Branches, loops, defer, destruction, tasks, threads, implicit allocation-context operations, and provider calls participate in ordered-effect analysis.

---

## § 80 Call-graph integration

§ 80(1) Call graph retains where relevant:

```text
Arena effects;
RequiresAllocationContext;
Arena-demand summary;
explicit Arena argument/domain identity where known;
task/thread dependency transfer;
destruction/defer operations;
provider calls;
open callable contracts.
```

§ 80(2) Synchronous callees propagate Arena effects normally.

§ 80(3) Spawned body effects belong to new execution context; spawn creation effects belong to spawner.

---

## § 81 Arena-demand analysis

§ 81(1) Compiler analyzes capacity demand per ArenaDomain and epoch.

§ 81(2) Demand classification is:

```text
Exact
Bounded
Unknown
Unbounded
```

§ 81(3) Analysis records where relevant:

```text
known capacity;
minimum required capacity;
maximum proven use;
alignment overhead;
worst-case path;
allocation sites;
unknown loop/call/recursion causes.
```

---

## § 82 Sequential demand

§ 82(1) Successful allocations in one epoch accumulate.

§ 82(2) End of reference live range does not subtract Arena bytes.

§ 82(3) Sequential demand includes alignment padding.

---

## § 83 Branch demand

§ 83(1) Mutually exclusive branches starting from the same Arena state combine using maximum continuing demand.

§ 83(2) Later continuing allocations add to the joined maximum state.

§ 83(3) Path analysis must preserve differing Arena state versions.

---

## § 84 Loop demand

§ 84(1) Bounded loop without Reset accumulates per-iteration Arena demand.

§ 84(2) Compiler may use constant/range/compile-time/proven upper bounds.

§ 84(3) Valid Reset per iteration separates epochs; peak demand becomes per-epoch maximum rather than cumulative across reset boundaries.

§ 84(4) Unknown loop with accumulating allocation is `Unknown` or `Unbounded`.

---

## § 85 Recursion demand

§ 85(1) Same-stack recursive Arena demand uses call-graph recursion analysis.

§ 85(2) Proven recursion bound may produce bounded demand.

§ 85(3) Unbounded/unknown recursive allocating depth yields `Unknown`/`Unbounded`.

§ 85(4) Spawn recursion is analyzed as execution/resource creation, not same-stack Arena cursor accumulation.

---

## § 86 Callable demand summaries

§ 86(1) Callable summaries may describe demand on explicit Arena parameters and ambient allocation context.

§ 86(2) Summary algebra may include:

```text
constant;
sum;
maximum;
constant multiplication;
range upper bound;
unknown;
unbounded.
```

§ 86(3) Sec 0.1 does not require a general theorem prover.

§ 86(4) Separate-compilation summaries must be validated/version-compatible.

---

## § 87 Indirect calls

§ 87(1) Closed target set uses maximum possible target demand.

§ 87(2) Open callable contract uses declared Arena bound when available.

§ 87(3) Otherwise contribution is `Unknown`.

§ 87(4) Permissive hosted profile may accept Unknown with diagnostics.

§ 87(5) Strict bounded-memory profile may reject when full bound proof is required.

---

## § 88 Growable demand

§ 88(1) Growable Arena still receives logical capacity-demand analysis.

§ 88(2) Report may distinguish:

```text
initial capacity;
maximum proven logical demand;
minimum growth;
maximum configured capacity;
provider failure possibility.
```

§ 88(3) Growability does not eliminate bounded-memory analysis.

---

## § 89 Statically impossible allocation

§ 89(1) If compiler proves an allocation can never succeed, the success continuation is unreachable.

§ 89(2) Source/control-flow diagnostics follow canonical dead-code/unreachable rules.

§ 89(3) In a `try` form whose success path is provably impossible, compiler reports the impossible allocation with required/available bytes, element count/type, alignment, and active `CompilationPlan`.

§ 89(4) Explicit direct handling of the failure `Result` remains valid where the programmer intentionally tests exhaustion and no unreachable source path is formed.

---

## § 90 Proven sufficient capacity

§ 90(1) When capacity/alignment/arithmetic proof guarantees success, compiler may eliminate:

```text
runtime capacity branch;
overflow branch;
AllocationError construction;
dynamic offset computation.
```

§ 90(2) Source method type remains `Result`.

§ 90(3) Proof is represented in Semantic IR/lowering.

---

## § 91 Semantic IR requirements

§ 91(1) Arena semantics remain explicit in Semantic IR until ownership, dependencies, Reset/Release ordering, task/thread completion, effects, capacity planning, and target strategy are established.

§ 91(2) Lower MLIR/backend stages must not invent Arena semantics.

---

## § 92 Semantic IR Arena concepts

§ 92(1) Semantic IR distinguishes at least:

```text
Arena owner state;
ArenaDomain identity;
Arena state version;
validity epoch;
allocation context;
backing relation/policy;
growth policy;
typed allocation;
Arena dependency;
ordered Arena effect;
failure path;
provider operation.
```

§ 92(2) Arena state version and validity epoch are distinct.

§ 92(3) Allocation advances state version but not epoch.

§ 92(4) Reset advances state version and epoch.

§ 92(5) Release consumes owner state and ends ArenaDomain.

---

## § 93 Required Semantic IR operations

§ 93(1) Semantic IR must represent distinctions equivalent to:

```text
ArenaCreateBorrowed
ArenaCreateOwnedFixed
ArenaCreateGrowable
ArenaNew
ArenaAlloc
ArenaReset
ArenaRelease
ArenaDestroy
```

§ 93(2) It must additionally represent/relate:

```text
allocation-context propagation;
task/thread ownership transfer;
task/thread Arena dependency;
await/join completion;
result dependency transfer;
provider invocation;
try success/failure flow;
capacity/demand facts;
epoch invalidation.
```

§ 93(3) Exact implementation operation names are non-normative.

---

## § 94 SSA Arena state

§ 94(1) Mutating Arena operations use SSA-style state versions in Semantic IR/MLIR.

Conceptually:

```text
arena1 = create
arena2, result1 = allocate(arena1)
arena3, result2 = allocate(arena2)
arena4 = reset(arena3)
release(arena4)
```

§ 94(2) One owner-state input is consumed by each mutating Arena operation.

§ 94(3) Exactly one continuing Arena state exists after each non-terminal operation.

§ 94(4) Release produces no continuing Arena owner state.

§ 94(5) SSA state consumption is compiler representation of unique Arena control and does not imply source value return syntax.

---

## § 95 Allocation failure in SSA

§ 95(1) Allocation produces one continuing Arena state plus one allocation `Result`.

§ 95(2) On success, continuing state contains advanced cursor/new stable segment.

§ 95(3) On failure, continuing state is physically equivalent to input state.

§ 95(4) Input SSA control state is nevertheless consumed and replaced by one continuing state to avoid duplicate owners.

§ 95(5) Error propagation cleanup uses continuing Arena state.

---

## § 96 Arena reference provenance in IR

§ 96(1) Successful Arena allocation records at least:

```text
ArenaDomain;
validity epoch dependency;
T;
bounds;
mutability authority;
storage origin/domain;
allocation identity/site.
```

§ 96(2) Allocation does not change epoch.

§ 96(3) Reset creates new epoch.

§ 96(4) Release ends domain.

§ 96(5) Metadata may be represented in SSA types, operation attributes, side tables, canonical identities, or combinations.

§ 96(6) Metadata need not all survive to runtime.

---

## § 97 Sec MLIR path

§ 97(1) Intended semantic lowering path is:

```text
Sec source
    -> Sec Semantic IR
    -> high-level Sec MLIR
    -> Arena planning/specialization
    -> standard MLIR dialects
    -> LLVM dialect/target code
```

§ 97(2) Arena must not be flattened to ordinary generic allocation before Arena-specific proof/planning is complete.

---

## § 98 Sec MLIR types and operations

§ 98(1) High-level Sec MLIR may define types conceptually equivalent to:

```text
!sec.arena<backing, growth, profile>
!sec.arena_domain
!sec.alloc_context
!sec.ref<T>
!sec.ref_mut<T>
!sec.slice<T>
!sec.slice_mut<T>
!sec.allocation_error
```

§ 98(2) It may define operations equivalent to Arena create/new/alloc/reset/release.

§ 98(3) Exact textual MLIR syntax is implementation-defined.

§ 98(4) Required semantic distinctions are normative.

---

## § 99 Multi-result IR

§ 99(1) Sec source functions returning one source value do not restrict internal IR/MLIR from using multiple SSA results.

§ 99(2) Arena allocation naturally produces:

```text
next Arena state;
allocation Result.
```

§ 99(3) This does not change source-language return semantics.

---

## § 100 MLIR effects

§ 100(1) High-level Arena operations integrate with MLIR effects where appropriate.

§ 100(2) Standard generic `Allocate`/`Free` effects alone are insufficient.

§ 100(3) Reset is not ordinary free because backing/domain remain live/reusable.

§ 100(4) MLIR representation must preserve ArenaDomain-aware resource/effect identity and owner-state consumption.

---

## § 101 Alias analysis

§ 101(1) Distinct successful positive-sized Arena allocations in one epoch are non-overlapping.

§ 101(2) Compiler may expose this to alias analysis.

§ 101(3) Same backing base does not mean allocations overlap.

§ 101(4) Bounds, allocation identity/order, domain, and epoch remain relevant.

§ 101(5) Zero-element allocations do not imply dereferenceable address ranges.

---

## § 102 `memref`

§ 102(1) MLIR `memref` may represent a lowered typed bounded Arena view after Arena semantics are verified.

§ 102(2) `memref.alloc` is not the canonical high-level meaning of `Arena.Alloc`.

§ 102(3) Arena represents one domain with shared capacity, many allocations, bulk Reset/Release, backing relation, epoch, ownership, and execution dependencies.

§ 102(4) Lowering every source Arena allocation immediately to independent `memref.alloc` is invalid when it loses these semantics.

---

## § 103 Physical Arena planning

§ 103(1) After semantic verification, compiler selects a concrete physical strategy per `CompilationPlan`.

Possible strategies:

```text
automatic/stack-backed fixed Arena;
static-backed fixed Arena;
borrowed descriptor;
owned fixed descriptor;
segmented growable descriptor;
reserved-address-space Arena;
fully scalarized/eliminated Arena.
```

§ 103(2) Strategy is implementation/profile choice preserving source semantics.

---

## § 104 Physical descriptors

§ 104(1) A fixed Arena may lower conceptually to:

```text
base;
capacity;
cursor;
optional epoch;
provider/domain metadata where required.
```

§ 104(2) A growable Arena may lower conceptually to:

```text
current segment;
stable segment list/equivalent;
cursor;
capacity;
optional epoch;
provider state.
```

§ 104(3) No universal runtime ABI/layout is required.

§ 104(4) Fields may be removed/compressed/replaced after proof.

---

## § 105 Epoch lowering

§ 105(1) Logical epoch semantics remain even when runtime epoch storage is eliminated.

§ 105(2) Runtime representation may be inline metadata, side table, domain token, capability, compact generation, or absent after proof.

§ 105(3) Epoch metadata may be eliminated only when no runtime stale-reference/handle mechanism requires it.

---

## § 106 Fixed borrowed lowering

§ 106(1) Borrowed fixed Arena may lower conceptually by storing base/capacity/cursor plus fresh logical domain/epoch.

§ 106(2) Typed allocation computes checked size/alignment/end offset, verifies capacity, initializes values, advances cursor, and returns a typed bounded safe view.

§ 106(3) Proof may remove redundant arithmetic/capacity checks.

§ 106(4) Lowering must preserve borrowed-backing lifetime/exclusive-control semantics.

---

## § 107 Growable lowering

§ 107(1) When current segment cannot satisfy complete request:

```text
compute complete request;
request stable segment;
if provider fails:
    preserve prior state and return Err;
link complete segment after successful acquisition;
allocate from new segment.
```

§ 107(2) Provider call remains visible to effects/call graph/trust/stack analysis unless represented by a compiler intrinsic with complete canonical summary.

§ 107(3) Existing allocation addresses must never be changed by growth.

---

## § 108 Permitted optimizations after proof

§ 108(1) Compiler may after proof:

```text
fold SizeOf/AlignOf;
precompute offsets;
combine capacity checks;
eliminate proven capacity/overflow checks;
eliminate runtime epoch metadata;
scalar-replace Arena-backed values;
automatic/stack-lower local Arena;
remove unused zero-element allocation;
remove semantically redundant Reset immediately before terminal Release;
remove Arena descriptor entirely;
inline Arena operations.
```

§ 108(2) Every optimization must preserve effects, failure behavior, dependencies, identity, cleanup, and source-visible semantics.

---

## § 109 Forbidden transformations without proof

§ 109(1) Compiler must not without valid proof:

```text
move allocation across Reset;
move access across Reset/Release;
CSE distinct Arena allocations;
merge distinct epochs;
replace fixed exhaustion with hidden growth;
fall back to another allocator/provider;
relocate live allocations during growth;
release before deferred dependent use;
share ordinary Arena mutation between execution contexts;
drop task/thread completion dependencies;
change allocation failure behavior;
change backing ownership/reclamation semantics.
```

---

## § 110 CSE, DCE, and LICM

§ 110(1) Arena allocation is not pure.

§ 110(2) CSE must not merge distinct allocations.

§ 110(3) Reset/Release have ownership/effect/lifetime semantics even without result value.

§ 110(4) DCE removes them only after proof that every semantic effect is unobservable and ownership remains correct.

§ 110(5) LICM must not change allocation count, failure timing, capacity demand, epoch boundary, resource lifetime, or dependency ordering.

---

## § 111 Verification

§ 111(1) Local Arena operation verification covers:

```text
operand/result types;
backing-policy compatibility;
count type;
complete T layout;
alignment;
trivial destruction;
result shape;
constructor policy.
```

§ 111(2) Global Arena analysis verifies:

```text
no live dependency across Reset;
no validity dependency across Release;
no use after move/Release;
no double Release;
task/thread completion;
cleanup/defer ordering;
allocation-context propagation;
capacity/profile requirements;
provider contracts.
```

§ 111(3) Local IR verifier alone is insufficient for complete Arena correctness.

---

## § 112 Target profiles

§ 112(1) Arena source semantics remain stable across profiles.

§ 112(2) Profiles may differ in:

```text
available constructors;
backing providers;
growth;
capacity-proof strictness;
runtime metadata;
provider failure guarantees;
thread transfer;
ISR use;
memory spaces.
```

§ 112(3) Profile must not weaken ownership, domain/epoch identity, failure atomicity, reference validity, or no-relocation growth semantics.

---

## § 113 Hosted profile

§ 113(1) Hosted profile may support borrowed/owned/growable Arena and compiler-managed ambient allocation context.

§ 113(2) Unknown capacity demand may be accepted with diagnostic in permissive mode.

§ 113(3) Provider failure remains represented unless proven impossible.

---

## § 114 Embedded fixed profile

§ 114(1) Embedded fixed profile may prefer:

```text
borrowed fixed Arena;
static backing;
caller-provided backing;
target-provided fixed pools.
```

§ 114(2) Owned/growable constructor availability is target-dependent.

§ 114(3) Unknown/unbounded demand may be an error under strict bounded-memory policy.

---

## § 115 Bare-metal bounded profile

§ 115(1) Bare-metal bounded profile may require:

```text
statically bounded capacity demand;
no OS backing provider;
no hidden growth;
no mandatory runtime epoch metadata;
no allocation context without explicit/provable backing.
```

§ 115(2) Compiler may eliminate entire Arena descriptor when all offsets/lifetime/identity facts are static.

---

## § 116 No-allocation profile

§ 116(1) A profile may make dynamic allocation unavailable.

§ 116(2) `Arena.FromBuffer` may remain available because it acquires no backing.

§ 116(3) `Arena.New`/`Arena.Alloc` retain canonical allocation effects and may be rejected by `@noAlloc`/profile policy even when physical bytes are pre-reserved.

§ 116(4) This rulebook does not redefine `@noAlloc`.

---

## § 117 ISR use

§ 117(1) Ordinary ISR code must not:

```text
create owned backing;
grow Arena;
allocate from shared ordinary Arena;
Reset storage visible to interrupted code;
Release storage visible to interrupted code.
```

§ 117(2) A target may permit an ISR-exclusive preallocated Arena only when canonical interrupt analysis proves:

```text
bounded demand;
no blocking;
no suspension;
no conflicting access;
safe Reset boundary;
bounded execution;
target support;
noPanic/noAlloc semantics as required by interrupts.md.
```

§ 117(3) Because current Sec 0.1 ISR policy implies `noAlloc`, ordinary `Arena.New`/`Alloc` are not ISR-valid unless the canonical effect/interrupt rules later introduce a narrower explicit exception; preallocated storage use must therefore normally be exposed through an ISR-safe non-allocating abstraction.

§ 117(4) Unsafe does not waive ISR restrictions.

---

## § 118 FFI

§ 118(1) Arena-backed storage passed to foreign code follows FFI retention contracts.

§ 118(2) Call-bounded pointer/view use may be valid when:

```text
foreign code does not retain;
Arena cannot Reset/Release during call;
mutability/aliasing matches contract;
address space/alignment is valid.
```

§ 118(3) Retained foreign pointer creates dependency for declared retention lifetime.

§ 118(4) Unknown retention is conservative.

§ 118(5) `RawPtr[T]` does not keep ArenaDomain alive.

§ 118(6) Retained direct addresses may require pin/stable backing guarantees.

---

## § 119 Diagnostics

§ 119(1) Arena diagnostics follow the mentor-compiler principle.

§ 119(2) Diagnostic provenance should retain:

```text
Arena declaration/domain;
allocation site;
Reset/Release site;
task/thread capture;
await/join/completion state;
defer/destruction path;
required/available bytes;
alignment;
active CompilationPlan/profile;
call-graph/effect path;
provider contract;
help.
```

§ 119(3) User-facing wording should distinguish storage capacity, lifetime dependency, ownership state, and epoch invalidation.

---

## § 120 Required error categories

§ 120(1) Required error families include:

```text
Arena copy;
use after move;
use after Release;
double Release;
Reset with live dependency;
Release with live dependency;
allocation from consumed Arena;
returning reference into local Arena;
parent Reset/Release while nested Arena lives;
task/thread dependency crossing invalidation;
missing allocation context;
unsupported backing provider;
invalid alignment;
incomplete/unsized T;
missing default;
non-trivially-destructible T;
statically impossible allocation;
unbounded demand under strict profile;
epoch exhaustion without safe handling;
invalid foreign retention;
invalid ISR use;
invalid provider/reclamation contract.
```

---

## § 121 Warnings and information

§ 121(1) Configurable warnings may include unknown peak demand, open callable without Arena bound, unknown recursive accumulation, high growable demand, or conservative foreign retention.

§ 121(2) Configurable information may include:

```text
zero-capacity Arena;
eliminated runtime capacity check;
Arena fully automatic/stack-lowered;
runtime epoch metadata eliminated;
redundant Reset before Release;
capacity utilization;
Arena descriptor eliminated.
```

§ 121(3) Analysis-only information should normally appear in LSP/explicit analysis output rather than default build output.

---

## § 122 LSP and tooling

§ 122(1) LSP consumes compiler-owned Arena analysis; it must not implement a separate model.

§ 122(2) Tooling may expose:

```text
backing relation/provider;
capacity;
ArenaDomain/epoch;
profile;
allocation origin;
live dependencies;
Reset/Release blockers;
task/thread captures;
capacity summary;
allocation-context origin;
effect path;
physical-lowering plan;
diagnostic cause chain;
peak demand/utilization.
```

§ 122(3) Tooling facts belong to one active compilation snapshot/plan.

---

## § 123 Separate compilation

§ 123(1) Module metadata preserves Arena-related callable facts needed by callers.

Relevant facts include:

```text
RequiresAllocationContext;
allocation effects;
Arena-demand summary;
explicit Arena parameter summaries;
provider requirements;
task/thread retention contracts;
open callable bounds;
trust provenance.
```

§ 123(2) Public caller must not rely on stronger undeclared implementation facts.

§ 123(3) Changes to public allocation-context requirement/Arena bound/provider contract may require dependent recompilation.

---

## § 124 Incremental compilation

§ 124(1) Arena analysis must invalidate when relevant facts change.

Examples:

```text
capacity;
allocation count;
T layout/alignment/default/destruction;
loop bound/control-flow reachability;
callee demand;
call target set;
effect contract;
target profile;
task/thread capture;
Reset/Release placement;
provider contract;
foreign retention;
storage/reference model.
```

§ 124(2) Stable ArenaDomain/call-site identities should survive unrelated edits where possible.

---

## § 125 Required source tests

§ 125(1) Required positive families include:

```text
FromBuffer;
WithCapacity;
Growable;
New;
Alloc;
zero-element;
zero-capacity;
Reset;
Release;
move;
implicit destruction;
nested Arena;
task borrow;
task move;
thread borrow;
thread move;
await dependency;
join dependency;
result dependency;
defer order;
early return;
failure handling;
bounded loop;
branch capacity;
ambient context;
explicit Arena context.
```

§ 125(2) Required negative families include:

```text
copy;
use after move;
use after Release;
double Release;
Reset live reference;
Release live reference;
return local reference;
nested parent invalidation;
task/thread invalidation;
missing context;
nontrivial T;
incomplete T;
invalid alignment;
impossible try allocation;
unbounded strict-profile demand;
defer cleanup order;
result dependency invalidation;
foreign retention;
ISR use.
```

---

## § 126 Semantic IR and MLIR tests

§ 126(1) Golden Semantic IR tests cover creation forms, New/Alloc, zero-element, failure state, `try`, Reset epoch, Release consumption, destruction, move, nesting, task/thread captures/transfers/completion, result dependencies, and allocation-context propagation.

§ 126(2) Sec MLIR tests cover operation parsing/printing, type verification, effect interfaces, ArenaDomain resource identity, ownership consumption, invalid sequences, source locations, canonicalization, CSE/DCE/LICM, Reset/Release ordering, provider calls, and epoch elimination.

---

## § 127 Capacity-analysis tests

§ 127(1) Tests include:

```text
exact sequential demand;
branch maximum;
branch plus continuation;
bounded loop multiplication;
range-bounded loop;
Reset per iteration;
unknown loop;
unbounded recursion;
closed indirect target maximum;
open callable unknown;
alignment padding;
overflow;
zero-element;
growable initial/max capacity;
statically impossible allocation;
proven sufficient capacity.
```

---

## § 128 Backend/profile tests

§ 128(1) Maintained backend tests cover representative hosted and bare-metal targets.

§ 128(2) They verify target pointer width/layout/alignment, checked size arithmetic, cursor arithmetic, epoch representation/elimination, borrowed backing, provider calls, segment growth, Release, and absence of mandatory general runtime.

§ 128(3) Optimization tests must prove permitted check/descriptor elimination and reject invalid CSE/hoisting/epoch merging/relocation.

---

## § 129 Governance completion criteria

§ 129(1) Frontend Arena support is complete when builtin/member/source forms, move-only ownership, lifecycle state, typed allocation validation, and all source diagnostics are implemented.

§ 129(2) Storage/reference integration is complete when ArenaDomain/epoch/backing identities use canonical storage/reference facts and all Reset/Release dependencies are proven.

§ 129(3) Allocation-context integration is complete when explicit/ambient/provider contexts propagate correctly across synchronous calls and execution boundaries.

§ 129(4) Task/thread integration is complete when captures, ownership transfer, completion, result dependencies, cancellation, and structured concurrency preserve Arena lifetime.

§ 129(5) Capacity analysis is complete when byte-accurate alignment-aware demand is modeled per CompilationPlan across control flow, loops, recursion, indirect calls, and separate compilation.

§ 129(6) Semantic IR/MLIR support is complete when Arena state, domain, epoch, backing, allocation, failure, dependency, effect, and provider semantics remain explicit until safely lowered.

§ 129(7) Lowering is complete when fixed/borrowed/owned/growable/profile strategies preserve all semantics without hidden allocator changes or live relocation.

§ 129(8) Tooling is complete when compiler/LSP/sec analyse consume one canonical Arena model.

§ 129(9) Arena is not fully implemented merely because compiler-known member recognition and partial generation checks exist.

---

## § 130 Core summary

§ 130(1) `Arena` is a move-only programmer-visible owner/controller of one ArenaDomain.

§ 130(2) ArenaDomain identity is separate from physical backing address.

§ 130(3) Backing may be owned, borrowed, static, or target-provided through canonical storage/provider contracts.

§ 130(4) Arena may be fixed or growable; growable Arena may add stable backing but may never relocate live allocations.

§ 130(5) Capacity is measured in bytes; `Alloc[T](count)` uses element count plus checked size/alignment arithmetic.

§ 130(6) Safe `New[T]`/`Alloc[T]` fully initialize results and in Sec 0.1 require sized/defaultable/trivially-destructible `T`.

§ 130(7) Allocation returns `Result`, is atomic, never publishes null/partial/uninitialized result, and never silently switches allocation source.

§ 130(8) Zero-element allocation is valid and consumes no capacity.

§ 130(9) Repeated allocation preserves prior allocation validity.

§ 130(10) Reset retains ArenaDomain/backing, advances validity epoch, and requires no validity-preserving dependency across the boundary.

§ 130(11) `Release()` is a compiler-known Arena lifecycle termination operation that consumes Arena ownership and ends ArenaDomain; it is not a general precedent for user-defined whole-self-consuming methods.

§ 130(12) Normal Arena destruction performs equivalent terminal Release semantics if Arena remains owned.

§ 130(13) Arena dependencies flow through references, nested Arenas, closures, tasks, threads, results, defer/cleanup, and FFI retention.

§ 130(14) Task/thread completion ends execution-local dependencies only at a canonical semantic completion boundary; result dependencies may continue.

§ 130(15) Parent allocation context is not automatically inherited by spawned execution.

§ 130(16) `MayAllocate` and `RequiresAllocationContext` are distinct facts.

§ 130(17) Arena operations remain explicit in Semantic IR/high-level Sec MLIR until ownership, lifetime, effects, capacity, execution dependencies, and target planning are complete.

§ 130(18) Arena state versions use SSA; validity epochs are distinct from state versions.

§ 130(19) MLIR `memref` may represent lowered Arena views but does not replace Arena semantics.

§ 130(20) Compiler may remove checks/metadata/descriptors only after proof.

§ 130(21) Arena supports hosted, embedded, bare-metal, and runtime-free implementations without universal GC, reference counting, allocator runtime, handle table, or Arena registry.
