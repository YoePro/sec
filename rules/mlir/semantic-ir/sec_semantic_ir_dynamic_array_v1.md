# Semantic IR Amendment - Owning Dynamic Arrays

## Status

Normative amendment for:

```text
rules/compiler/semantic_ir.md
```

Package:

```text
SEC-MLIR-P18
```

Repository baseline:

```text
152c772
```

Local predecessors:

```text
SEC-MLIR-P13
SEC-MLIR-P14
SEC-MLIR-P15
SEC-MLIR-P16
SEC-MLIR-P17
```

This amendment defines canonical target-independent Semantic IR for owning
runtime-sized arrays and the storage-domain transitions they require.

---

# 1. Dynamic-array type

Semantic IR adds:

```text
DynamicArrayType(Element TypeID)
```

Source spelling:

```text
T[]
```

Length/capacity/backing facts are value state, not type identity.

---

# 2. Ownership

Every `T[]` is MoveOnly.

No ordinary copy operation is valid.

Whole-value transfer uses P17 move semantics.

---

# 3. Default

Semantic IR provides a non-allocating empty owning value:

```text
length 0
capacity 0
backing none
```

No element default is required.

---

# 4. Value invariant

Every readable dynamic array satisfies:

```text
0 <= length <= capacity
live initialized elements = [0,length)
capacity-only slots are not live T objects
```

---

# 5. Backing facts

A backed array retains/reference facts equivalent to:

```text
storage identity
allocation identity/context
backing relation
reclamation authority
reclamation plan
invalidation domain
epoch
address stability
relocation class
memory space
```

---

# 6. Allocation

Allocation is explicit Semantic IR.

Potentially failing allocation produces:

```text
failed
AllocationError
```

and establishes backing only on success.

---

# 7. Partial construction

A partially constructed array remains safe by exposing only the initialized
prefix as logical length.

Cleanup destroys only that prefix.

---

# 8. Internal append

A prepared element may be appended only when capacity is available.

Ownership transfers into the array after successful element initialization.

The logical length increments last.

---

# 9. Growth

Backing growth is explicit and transactional.

Failure preserves the original owner.

Success identifies whether backing invalidation is:

```text
none
epoch advance
domain replacement
```

---

# 10. Storage transitions

Semantic IR explicitly represents:

```text
EstablishDomain
AdvanceEpoch
EndDomain
ReclaimStorage
```

Destruction and reclamation remain separate.

---

# 11. Relocation

Physical backing relocation is not source-level value move.

Element ownership remains with the owning dynamic array.

Relocation invalidates direct dependencies according to the storage domain.

---

# 12. Destruction

Dynamic-array destruction:

```text
destroys initialized elements in reverse logical index order
ends backing domain
reclaims backing only through resolved reclamation authority/plan
```

---

# 13. Arena-backed owner

Arena-backed dynamic arrays still own element object lifetimes.

Array destruction destroys elements.

Arena policy owns raw backing reclamation.

---

# 14. Indexing

Dynamic-array indexing produces a P15 Place.

Bounds use runtime length.

P17/P15 then handle read, replacement and borrowing.

---

# 15. Indexed move boundary

Runtime-index move-out of move-only elements remains unsupported.

No per-index partial ownership mask is introduced.

---

# 16. Slice borrowing

P16 slice borrow may consume a dynamic-array owner Place as contiguous source.

The resulting slice depends on the current backing incarnation.

---

# 17. Length

Semantic IR distinguishes:

```text
DynamicArrayLength -> uint
DynamicArrayLengthInt -> int
```

No implicit narrowing between them.

---

# 18. Internal capacity

Capacity exists as compiler/internal semantic state.

It is not source member semantics.

---

# 19. Effects

Dynamic-array operations may explicitly contribute:

```text
AllocateStorage
RelocateStorage
ReclaimStorage
EstablishDomain
AdvanceEpoch
EndDomain
```

---

# 20. Verifier

Semantic IR verification checks:

```text
MoveOnly classification
length/capacity invariant
initialized-prefix invariant
allocation success/failure use
allocation/reclamation pairing
growth transactional behavior
storage transitions
index guard
element Place backing dependency
slice origin
borrow exclusion for invalidating growth
destruction/reclamation order
```

---

# 21. Physical separation

Semantic IR does not define descriptor field order, pointer shape, allocator ABI,
or physical capacity policy.

---

# 22. Deterministic printer

Print high-level dynamic-array state/provenance deterministically:

```text
element type
logical length identity/value
capacity identity/value where internal
storage/invalidation IDs
allocation/reclamation plan IDs
transition kinds
```

Do not print assumed physical pointer layout.
