# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `14`

Version 14 lowers owning dynamic-array Semantic IR to high-level schema-v14 Sec
MLIR and preserves storage/allocation semantics explicitly.

It stops before physical descriptor and allocator lowering.

---

# 1. Type mapping

Semantic IR:

```text
DynamicArrayType(T)
```

maps to:

```text
!sec.dynamic_array<T>
```

Length and capacity remain value state.

---

# 2. Empty default

Lower the canonical no-allocation default to:

```text
sec.dynamic_array.empty
```

Do not allocate zero bytes merely to fabricate an empty owner.

---

# 3. Whole ownership transfer

Use P17:

```text
sec.own.move
```

for by-value transfer.

Do not copy the descriptor.

Do not allocate.

---

# 4. Allocation

Lower a resolved allocation-capable dynamic-array producer to:

```text
sec.dynamic_array.allocate
```

with selected allocation-context metadata.

Preserve `AllocationError`.

Do not choose a physical allocator.

---

# 5. Allocation CFG

Canonical:

```text
array, failed, error = dynamic_array.allocate

failed:
    consume AllocationError through Result/handler flow

success:
    establish/consume established backing facts
    register P17 cleanup responsibility for the owner
```

---

# 6. Domain establishment

Successful new backing allocation emits/retains:

```text
sec.storage.establish_domain
```

according to the resolved storage plan.

No domain transition on allocation failure.

---

# 7. Internal append

Lower compiler/core prepared element insertion to:

```text
sec.dynamic_array.append_prepared
```

after capacity availability is established.

Element ownership transfers into the array.

---

# 8. Internal growth

Lower approved capacity growth to:

```text
sec.dynamic_array.grow_backing
```

Failure leaves owner unchanged.

Success applies the emitted transition kind.

---

# 9. No-transition growth

For:

```text
transition = none
```

no storage invalidation op is emitted.

---

# 10. Epoch-advance growth

For:

```text
transition = advance-epoch
```

emit:

```text
sec.storage.advance_epoch
```

after successful backing change and before new dependent references are created.

---

# 11. Domain-replacement growth

For:

```text
transition = replace-domain
```

emit canonical:

```text
end old domain
establish new domain
reclaim old backing when authorized
```

according to the resolved plan.

---

# 12. Direct-reference legality

Do not lower invalidating growth if Sema reports a conflicting live direct
reference/slice/pin dependency.

No runtime borrow-check fallback is introduced.

---

# 13. Length

`.len`:

```text
sec.dynamic_array.len
```

`len(owner)`:

```text
sec.dynamic_array.len_int
```

with current int-representability guard.

---

# 14. Indexing

Preserve:

```text
owner expression/place once
index once
bounds proof/check
element Place
```

Use:

```text
sec.dynamic_array.index_in_bounds
sec.dynamic_array.element_place
```

---

# 15. Element read

Use P17/P15:

```text
copy-from-place
semantic-copy-from-place
or ordinary Place read path
```

according to resolved copy semantics.

Runtime-index move-out of move-only element remains unsupported.

---

# 16. Element replacement

Use:

```text
element Place
sec.own.replace_place
```

for non-trivial ownership-safe replacement.

No backing transition.

---

# 17. Element borrow

Use P15:

```text
sec.ref.borrow_shared
sec.ref.borrow_mut
```

from the element Place.

Attach current backing-domain dependency.

---

# 18. Slice borrow

Use P16:

```text
range normalize/check
sec.slice.borrow_shared
sec.slice.borrow_mut
```

with dynamic-array Place source.

No allocation.

---

# 19. Dynamic-array destruction

P17 destruction lowers semantically as:

```text
destroy initialized elements in reverse runtime order
end backing domain
reclaim backing when OwningValue authority applies
```

Keep compact/high-level until physical cleanup lowering.

---

# 20. Arena-backed destruction

For Arena reclamation authority:

```text
destroy elements
end collection/backing logical domain as defined
do not individually reclaim Arena bytes
```

Arena reset/release remains P19/allocation lowering.

---

# 21. Individually reclaimable destruction

For owning-value reclamation authority:

```text
destroy elements
end domain
sec.storage.reclaim
```

using the exact matching plan.

---

# 22. Replacement

Replacing an owning dynamic array uses P17 transactional replacement:

```text
prepare new owner
on success destroy/reclaim old owner
install new owner
```

No early reclamation.

---

# 23. Function calls/returns

Keep:

```text
!sec.dynamic_array<T>
```

in high-level signatures.

Ownership metadata says move.

No descriptor ABI classification.

---

# 24. Struct/union integration

Nested dynamic-array fields/payloads remain high-level values.

P17 aggregate destruction invokes their dynamic-array destruction plan.

---

# 25. P6 interaction

Target scalar resolution may recurse through element type and resolve length/capacity
observation result widths while preserving the dynamic-array wrapper.

---

# 26. P8 interaction

Do not recursively signless-normalize semantic element types inside the
dynamic-array wrapper.

---

# 27. Effects

Preserve storage effects across lowering:

```text
AllocateStorage
RelocateStorage
ReclaimStorage
EstablishDomain
AdvanceEpoch
EndDomain
```

Do not reorder them across dependent references, cleanup, defer, or
synchronization.

---

# 28. No public capacity lowering

`sec.dynamic_array.capacity_internal` may appear only in compiler/core generated
IR.

Do not create source member metadata for it.

---

# 29. No public growth lowering

Internal append/grow operations are lowering primitives.

They are not evidence that `T[]` has public `Push`/`Reserve` methods.

---

# 30. No physical descriptor lowering

Do not lower to:

```text
LLVM struct
MemRef
pointer+length+capacity tuple
C++ vector representation
```

in lowering version 14.

---

# 31. No physical allocation lowering

Do not select:

```text
malloc/free
Arena object layout
system allocator calls
```

in version 14.

---

# 32. Verification pipeline

Run as applicable:

```text
normal MLIR verifier
previous P13-P17 verifiers
sec-verify-dynamic-arrays
sec-verify-storage-transitions
```

Storage-transition verification must occur before any pass erases domain facts.

---

# 33. Optimization independence

Correctness does not depend on:

```text
allocation elision
capacity folding
SROA
mem2reg
copy elision
move elision
bounds-check elimination
generation-check elimination
```

Those are later legal optimizations after proof.

---

# 34. Completion

Lowering version 14 is implemented when:

```text
T[] stays move-only
empty default allocates nothing
allocation/failure remains explicit
length/capacity invariants survive
element places use runtime length
slice borrowing uses backing dependencies
growth transitions are explicit
destruction and reclamation remain separate
Arena/owner reclamation authority is respected
no public growth API is invented
no physical descriptor or allocator ABI is selected
```
