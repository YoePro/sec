# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `14`

Schema version 14 adds owning dynamic-array semantic values and the general
storage-domain transitions required by allocator-backed/collection storage.

It does not define physical dynamic-array descriptor layout.

---

# 1. Version history

```text
v1   dialect foundation
v2   Semantic IR bridge
v3   scalar/target coverage
v4   checked integer operations
v5   typed arithmetic failure and Result construction
v6   Result branching/local try handlers
v7   enum and union values
v8   verified match CFG
v9   struct semantic values
v10  fixed-array semantic values
v11  places and direct references
v12  slice semantic values
v13  ownership transfer and deterministic destruction
v14  owning dynamic arrays and storage transitions
```

Compiler-generated v14 modules carry:

```mlir
sec.dialect_version = 14 : i32
```

---

# 2. `!sec.dynamic_array<T>`

High-level source type:

```text
T[]
```

Semantic properties:

```text
owned
move-only
runtime length
compiler-internal capacity
possibly separate backing storage
non-trivial destruction
```

No length/capacity/allocator field is part of type identity.

---

# 3. No copy op for dynamic arrays

`sec.own.copy` and `sec.own.semantic_copy` are invalid for:

```text
!sec.dynamic_array<T>
```

unless a later explicit named duplication operation produces a separate owner
through explicit allocation semantics.

---

# 4. `sec.dynamic_array.empty`

No operands.

Result:

```text
!sec.dynamic_array<T>
```

Canonical facts:

```text
length = 0
capacity = 0
backing = none
```

No allocation effect.

---

# 5. `sec.dynamic_array.allocate`

Operands include:

```text
minimum capacity
```

and compiler-selected allocation-plan/context metadata.

Results:

```text
!sec.dynamic_array<T>
i1 failed
AllocationError
```

High-level total checked allocation.

---

# 6. Allocation success rule

On success:

```text
length = 0
capacity >= requested minimum
backing domain is established
```

The array result is consumed only on success.

---

# 7. Allocation failure rule

On failure:

```text
no backing domain is established
error is consumed on failure path
array result is not consumed as a valid owner
```

---

# 8. `sec.dynamic_array.len`

```text
!sec.dynamic_array<T> -> !sec.uint
```

before P6 target scalar resolution.

---

# 9. `sec.dynamic_array.len_int`

```text
!sec.dynamic_array<T> -> !sec.int
```

Compiler generation requires proven representability until the core length rule
is synchronized.

---

# 10. `sec.dynamic_array.capacity_internal`

```text
!sec.dynamic_array<T> -> !sec.uint
```

Compiler/core-internal only.

Ordinary source member resolution must not target this operation.

---

# 11. `sec.dynamic_array.index_in_bounds`

Operands:

```text
dynamic array value/place
integer index
```

Result:

```text
i1
```

Meaning:

```text
0 <= index < runtime length
```

with ordinary signed/unsigned rules.

---

# 12. `sec.dynamic_array.element_place`

Operands:

```text
!sec.place<!sec.dynamic_array<T>, A>
index
```

Result:

```text
!sec.place<T, A>
```

Required:

```text
bounds_kind
bounds_proof
backing dependency
```

Runtime form requires matching bounds guard.

---

# 13. `sec.dynamic_array.append_prepared`

Operands:

```text
writable dynamic-array Place
owned T
```

No ordinary result.

Requires:

```text
length < capacity
no conflicting direct borrow
resolved P17 ownership transfer
```

No allocation.

---

# 14. `sec.dynamic_array.grow_backing`

Operands:

```text
writable dynamic-array Place
minimum capacity
```

Results:

```text
i1 failed
AllocationError
backing transition
```

Compiler/core-internal.

Failure preserves original owner.

---

# 15. Backing transition enum

Canonical values:

```text
none
advance-epoch
replace-domain
```

Use a typed enum/custom attribute or type.

---

# 16. `sec.storage.establish_domain`

High-level general storage transition.

Represents:

```text
Absent -> Live(initial epoch)
```

Carries/reference canonical storage facts.

---

# 17. `sec.storage.advance_epoch`

Represents:

```text
Live(old) -> Live(new)
```

Logical domain identity preserved.

Old dependencies become stale.

---

# 18. `sec.storage.end_domain`

Represents:

```text
Live(epoch) -> Ended
```

No physical reclamation implied.

---

# 19. `sec.storage.reclaim`

High-level physical reclamation semantic operation.

Requires a resolved reclamation plan.

It is distinct from:

```text
value destruction
domain end
```

---

# 20. Dynamic-array storage metadata

Relevant ops may carry:

```text
sec.storage_identity
sec.allocation_identity
sec.allocation_context
sec.reclamation_authority
sec.reclamation_plan
sec.invalidation_domain
sec.epoch_dependency
sec.address_stability
sec.relocation_class
sec.memory_space
```

These remain semantic metadata.

---

# 21. P17 destruction plan

Schema v14 accepts:

```text
DestroyDynamicArray
```

as a destruction-plan kind.

It may remain one compact operation/plan over runtime length.

---

# 22. P17 ownership

Whole dynamic-array transfer uses:

```text
sec.own.move
sec.own.move_from_place
sec.own.initialize_place
sec.own.replace_place
sec.own.destroy_value
sec.own.destroy_place
```

No dynamic-array-specific copy/move semantics.

---

# 23. P16 slice borrow extension

`sec.slice.borrow_shared` and `sec.slice.borrow_mut` accept a compatible:

```text
!sec.place<!sec.dynamic_array<T>, A>
```

as contiguous source.

The range must be valid against runtime dynamic-array length.

---

# 24. Direct backing dependency

Slices/references derived from a dynamic array carry the backing invalidation
dependency, not merely the outer owner variable identity.

---

# 25. Storage transition verifier

Register:

```text
--sec-verify-storage-transitions
```

---

# 26. Dynamic-array verifier

Register:

```text
--sec-verify-dynamic-arrays
```

---

# 27. Dynamic-array verifier rules

Check:

```text
MoveOnly use
length <= capacity
allocated capacity/result flow
append capacity guard/fact
index guard
element-place type/authority
growth transactional failure
borrow exclusion metadata
slice origin
destruction plan
no ordinary source use of capacity_internal
```

---

# 28. Storage transition verifier rules

Check:

```text
establish only for absent/new domain incarnation
advance only for live domain
end only for live domain
no dependent access after end
reclaim plan matches provider/allocation facts
reclaim after element destruction
Arena authority does not use owner individual reclaim
```

---

# 29. No physical descriptor

Schema v14 does not define:

```text
pointer field
length field offset
capacity field offset
allocator pointer
epoch field
inline small-buffer layout
LLVM struct body
```

---

# 30. No allocator ABI

Schema v14 does not choose:

```text
malloc
free
new
delete
mmap
specific Arena implementation
```

---

# 31. Schema-v14 completion

Schema v14 is complete when:

```text
dynamic-array type parses/prints/verifies
empty/allocate/len/index/place ops verify
internal append/grow verify
storage transitions verify
P17 ownership/destruction integration verifies
P16 slice borrowing verifies
schema-v13 regressions remain valid
no public capacity/growth API is added
no physical descriptor/allocator ABI is selected
```
