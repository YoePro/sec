# Package 19 Normative Synchronization - Allocation Context and Arena

## Status

Normative synchronization for:

```text
rules/memory/arena.md
rules/memory/allocation.txt
rules/memory/storage.md
rules/memory/reference_model.md
rules/library/core-library.md
```

Package:

```text
SEC-MLIR-P19
```

Repository baseline:

```text
152c772
```

Local predecessors:

```text
SEC-MLIR-P13 through SEC-MLIR-P18
```

`rules/memory/arena.md` is the canonical Sec 0.1 Arena rulebook.

This document locks the compiler choices needed by the new Semantic IR pipeline
without changing the existing source semantics.

---

# 1. Arena is one source builtin type

All of these values have source type:

```text
Arena
```

regardless of whether backing is:

```text
owned
borrowed
static
target-provided
fixed
growable
```

Backing/growth policy is semantic value/CompilationPlan state, not a distinct
source nominal type.

---

# 2. Arena is MoveOnly

Canonical:

```text
Arena -> MoveOnly
```

Move preserves ArenaDomain, epoch and backing state.

Copy is invalid.

---

# 3. Lowercase `arena` remains ordinary identifier

`Arena` is the builtin type.

A lowercase variable named `arena` is ordinary source syntax.

Remove any leftover keyword reservation unless another canonical rulebook later
assigns a language meaning.

---

# 4. ArenaDomain versus state version

P19 distinguishes:

```text
ArenaDomain identity
Arena state version
validity epoch
```

Ordinary allocation changes only state version.

Reset changes state version and epoch.

Release consumes the state and ends the ArenaDomain.

---

# 5. Borrowed Arena

`Arena.FromBuffer(ref mut byte[])`:

```text
fresh ArenaDomain
fixed capacity
infallible
no backing allocation
retains exclusive backing borrow
no backing deallocation on Release
```

Zero-length backing is valid.

---

# 6. Owned fixed Arena

`Arena.WithCapacity(uint)`:

```text
Result[Arena, AllocationError]
fresh ArenaDomain on success
owned provider backing
fixed capacity
provider/reclamation plan
```

No partial Arena is published on failure.

---

# 7. Growable Arena

The semantic model supports growable Arena.

Growth may acquire additional stable backing but may never relocate any existing
live Arena allocation.

The public `Growable` constructor may remain implementation/profile gated while
the semantic model exists.

---

# 8. Safe typed allocation restriction remains

Even after P17 general destruction support:

```text
Arena.New[T]
Arena.Alloc[T]
```

initial safe forms still require T to be:

```text
sized
complete layout
known alignment
canonically defaultable
safely initializable
trivially destructible
```

Do not widen this merely because general cleanup IR now exists.

---

# 9. Arena.New and Arena.Alloc remain Result-returning

Source contracts remain stable:

```text
New[T]   -> Result[ref mut T, AllocationError]
Alloc[T] -> Result[ref mut T[], AllocationError]
```

A proof may remove a runtime failure branch.

It must not change source callable type.

---

# 10. AllocationError identity

Use the canonical compiler-known/core enum:

```sec
enum AllocationError {
    OutOfMemory
    Unsupported
    InvalidSize
    InvalidAlignment
}
```

No Arena-specific replacement.

---

# 11. Zero-element allocation

`Arena.Alloc[T](0)`:

```text
succeeds
returns valid empty mutable slice
consumes no capacity
requires no growth
does not advance epoch
```

Its source type remains Result.

---

# 12. Allocation atomicity

Failed Arena create/alloc/growth must not publish:

```text
partial Arena
partial segment
partial typed object
partial slice
changed cursor
changed epoch
changed prior allocation validity
```

---

# 13. Ordinary allocation preserves epoch

Repeated allocation within one Arena epoch does not invalidate earlier
allocations.

This remains true when a growable Arena acquires a stable additional segment.

---

# 14. Reset

Reset:

```text
requires no live validity-preserving dependency
preserves ArenaDomain
advances epoch
resets allocation state/cursors
retains reusable backing
does not require zeroing bytes
```

Use `AdvanceEpoch`.

---

# 15. Release

Release:

```text
consumes Arena
requires no live validity-preserving dependency
ends ArenaDomain
handles backing according to ownership/provider
permits no later Arena use
```

Use `EndDomain`.

---

# 16. Implicit Arena destruction

Normal destruction of a still-owned Arena has terminal Release semantics.

Explicit Release consumes the Arena and prevents a second implicit release.

---

# 17. Nested Arena

A child Arena created from parent Arena allocation:

```text
has fresh child ArenaDomain
has its own epoch
depends on parent ArenaDomain + epoch
blocks parent Reset/Release while live
```

Child Release ends the child dependency without individually reclaiming parent
allocation bytes.

---

# 18. Allocation context count

Every callable invocation has:

```text
zero or one active allocation context
```

This is compiler semantic state.

---

# 19. `MayAllocate` and `RequiresAllocationContext`

Keep separate:

```text
MayAllocate:
    reachable allocation effect exists

RequiresAllocationContext:
    implicit allocation needs ambient context
```

Explicit Arena allocation may be MayAllocate without requiring ambient context.

---

# 20. Context selection order

Canonical:

```text
explicit Arena selected by operation
propagated ambient context
compiler-managed local proven context
target-provided context
otherwise compile-time error
```

No backend selection.

---

# 21. No lexical Arena guessing

Ordinary Arena values in scope are never ambient candidates merely because they
are visible.

---

# 22. High-level hidden context

P19 represents ambient context explicitly in internal Semantic IR/Sec MLIR.

This does not add a source function parameter.

A function requiring ambient context receives a compiler-hidden internal
context value.

---

# 23. Foreign ABI guard

Ordinary foreign/export ABI may not receive the hidden context directly.

Use:

```text
wrapper
explicit ABI contract
or reject export
```

---

# 24. Spawn/thread boundary

Spawned task/thread contexts do not automatically inherit parent mutable Arena
context.

P19 preserves this as semantic summary metadata.

Full concurrency lowering is deferred.

---

# 25. No-allocation profile

A no-allocation profile:

```text
does not gain hidden heap fallback
may retain Arena.FromBuffer
has no ambient provider-backed allocation context unless explicitly declared
```

Profile/effect rules may reject Arena.New/Alloc even against preexisting backing.

---

# 26. Arena allocation is an ordered effect

Arena create/allocate/reset/release events retain ArenaDomain identity and
source order.

Compiler optimization may not treat them as pure calls or generic unordered
memory writes.

---

# 27. Capacity-demand summary

P19 introduces a basic compiler summary with:

```text
constant
sum
maximum
constant multiplication
range upper bound
unknown
unbounded
```

This is sufficient for initial bounded-profile enforcement without creating a
general theorem prover.

---

# 28. Panic cleanup boundary

Arena supports the canonical profile distinction:

```text
cleanup-capable panic
immediate trap/abort
```

P19 does not add a universal unwinder.

Normal P17 cleanup remains required.

---

# 29. Storage model reuse

Arena-specific semantics must reuse:

```text
StorageIdentity
BackingRelation
ReclamationAuthority
InvalidationDomain
ValidityEpoch
EstablishDomain
AdvanceEpoch
EndDomain
```

Do not build an Arena-only competing storage/epoch model.

---

# 30. Required synchronization

Update:

```text
Arena builtin/type metadata
lowercase arena lexer keyword status if needed
copy/move classification
allocation-context summaries
Semantic IR Arena operations
Sec MLIR Arena/context types and ops
effect/resource metadata
storage-domain transitions
P17 destruction plan
P15/P16 dependency facts
P18 allocation-context consumer
tests and derived manuals
```
