# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `12`

Schema version 12 adds high-level shared/mutable slice semantic values and
range/index operations.

It does not define physical pointer-plus-length layout.

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
```

Compiler-generated v12 modules carry:

```mlir
sec.dialect_version = 12 : i32
```

---

# 2. `!sec.slice<T>`

Shared safe slice.

High-level semantics:

```text
bounded
runtime-length
non-owning
shared
copyable
trivially destructible
```

No length/origin/epoch type parameter.

---

# 3. `!sec.slice_mut<T>`

Mutable safe slice.

High-level semantics:

```text
bounded
runtime-length
non-owning
exclusive mutable
move-only
trivially destructible
```

No length/origin/epoch type parameter.

---

# 4. No owning sequence conflation

Neither slice type represents:

```text
T[]
```

owning dynamic sequence values.

---

# 5. Reference metadata

Slice-producing operations use the same semantic metadata family as direct
references:

```text
sec.borrow_id
sec.lifetime_id
sec.validity_policy
sec.validity_proof
sec.storage_identity
sec.epoch_dependency
sec.relocation_class
sec.address_space
sec.reference_origin
```

---

# 6. Range provenance metadata

Optional canonical metadata:

```text
sec.slice_range_id
sec.slice_static_start
sec.slice_static_end_exclusive
```

Static values use canonical arbitrary-precision decimal representation.

---

# 7. `sec.slice.range_normalize`

Operands:

```text
source_length
optional start
optional end
```

Results:

```text
start: uint
length: uint
```

Required attrs identify:

```text
start present
end present
endpoint signedness
upper-bound kind
proof kind
```

Valid only for Sema-proven-safe range.

---

# 8. `sec.slice.range_check`

Operands:

```text
source_length
optional start
optional end
```

Results:

```text
normalized_start: uint
normalized_length: uint
failed: i1
error: canonical RangeError enum
```

Total high-level checked range operation.

---

# 9. Range error mapping

Canonical runtime mapping:

```text
InvalidRange
StartAfterEnd
OutOfBounds
```

according to the synchronized P16 range rules.

---

# 10. Range check branch convention

Compiler-generated runtime form:

```text
failed == true  -> failure path
failed == false -> success path
```

Same convention as other checked Sec operations.

---

# 11. `sec.slice.borrow_shared`

Operands:

```text
compatible contiguous source Place
normalized start
normalized length
```

Result:

```text
!sec.slice<T>
```

No allocation.

---

# 12. `sec.slice.borrow_mut`

Requires writable compatible contiguous source Place.

Result:

```text
!sec.slice_mut<T>
```

---

# 13. `sec.slice.reborrow_shared`

Source:

```text
!sec.slice<T>
or
!sec.slice_mut<T>
```

plus normalized subrange.

Result:

```text
!sec.slice<T>
```

No range/lifetime/authority widening.

---

# 14. `sec.slice.reborrow_mut`

Source:

```text
!sec.slice_mut<T>
```

Result:

```text
!sec.slice_mut<T>
```

No mutable reborrow from shared slice.

---

# 15. `sec.slice.copy_shared`

```text
!sec.slice<T> -> !sec.slice<T>
```

Copies descriptor/reference semantics only.

---

# 16. `sec.slice.move_mut`

```text
!sec.slice_mut<T> -> !sec.slice_mut<T>
```

Move-only holder transfer.

---

# 17. `sec.slice.len`

Operand:

```text
shared or mutable slice
```

Result:

```text
!sec.uint
```

before P6 target scalar resolution.

---

# 18. `sec.slice.len_int`

Operand:

```text
shared or mutable slice
```

Result:

```text
!sec.int
```

Represents compiler-known `len(slice)`.

Compiler generation requires a representability proof until the outstanding core
length rule is synchronized.

---

# 19. `sec.slice.index_in_bounds`

Operands:

```text
slice
integer index
```

Result:

```text
i1
```

Required:

```text
index_signed
```

Total spatial bounds predicate.

---

# 20. `sec.slice.element_place`

Operands:

```text
slice
index
```

Result:

```text
!sec.place<T,"ro">
```

for shared source or:

```text
!sec.place<T,"rw">
```

for mutable source.

Required:

```text
bounds_kind
bounds_proof
```

---

# 21. `sec.ref.is_valid` extension

Schema v12 extends direct-reference validity operand support to:

```text
!sec.slice<T>
!sec.slice_mut<T>
```

No new slice generation-check operation is introduced.

---

# 22. `sec.ref.end_borrow` extension

Borrow IDs associated with direct slices may use the same canonical borrow-end
marker.

No slice-specific duplicate lifetime op.

---

# 23. `sec.fail.bounds` extension

Allowed operation provenance includes:

```text
fixed-array-index
slice-index
slice-range
```

Semantic panic reason remains:

```text
panic.bounds
```

---

# 24. Slice guard verifier

Register:

```text
--sec-verify-slice-guards
```

It validates:

```text
range checked CFG
range normalized output use
RangeError failure use
reborrow narrowing
index guard
element-place guard
slice identity
mutable authority
dynamic reference-validity dominance
```

---

# 25. Reference verifier integration

Existing:

```text
--sec-verify-reference-guards
--sec-verify-borrow-semantics
--sec-verify-places
```

are extended for slice direct-reference values and slice element Places.

---

# 26. No slice equality

Schema v12 defines no:

```text
sec.slice.cmp
```

Source slice equality remains unsupported.

---

# 27. No physical descriptor

Schema v12 does not define:

```text
base pointer
physical length field
epoch field
capability bounds
handle representation
FFI layout
```

---

# 28. No default slice op

There is no:

```text
sec.slice.default
```

Safe slices require a valid explicit origin.

---

# 29. Empty slice

An empty slice is represented by the same semantic slice type and runtime length
zero.

No special nullable slice type exists.

---

# 30. P6 compatibility

Target scalar resolution may recurse into element types while preserving slice
wrappers and metadata.

It also resolves:

```text
sec.slice.len -> uint width
sec.slice.len_int -> int width
```

---

# 31. P8 compatibility

Signless normalization does not recurse through slice wrappers.

---

# 32. Place integration

`sec.slice.element_place` creates canonical P15 Place values.

Value read/write/reference borrowing uses P15 operations rather than duplicate
slice memory ops.

---

# 33. Schema-v12 completion

Schema v12 is complete when:

```text
slice types parse/print/verify
range ops parse/print/verify
borrow/reborrow ops verify
copy/move classification is enforced
length ops verify
index/place ops verify
reference guard integration works
empty slices are valid
owning T[] is not represented as slice
schema-v11 regressions remain valid
no physical descriptor layout is selected
```
