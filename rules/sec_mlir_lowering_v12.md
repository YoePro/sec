# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `12`

Version 12 lowers canonical Semantic IR slice values to high-level schema-v12
Sec MLIR.

It does not lower slices to physical descriptors.

---

# 1. Type mapping

Shared slice semantic type:

```text
ref T[]
```

maps to:

```text
!sec.slice<T>
```

Mutable:

```text
ref mut T[]
```

maps to:

```text
!sec.slice_mut<T>
```

Do not map either to owning `T[]`.

---

# 2. Slice creation from fixed array

Lower:

```text
fixed-array Place
normalized start
normalized length
```

to:

```text
sec.slice.borrow_shared
or
sec.slice.borrow_mut
```

No array load/copy.

---

# 3. Slice reborrow

Existing slice source lowers through:

```text
source validity guard when required
range normalization/check
sec.slice.reborrow_shared / reborrow_mut
```

No element copy.

---

# 4. Range operand evaluation

Preserve:

```text
source
start
end
```

evaluation order.

Each expression evaluates once.

---

# 5. Proven range

Use:

```text
sec.slice.range_normalize
```

with proof provenance.

No runtime range branch.

---

# 6. Runtime range

Use:

```text
sec.slice.range_check
cf.cond_br failed
```

True is failure.

False is success.

---

# 7. Ordinary range failure

Failure:

```text
sec.fail.bounds
operation = slice-range
```

No backend trap chosen.

---

# 8. Fallible range failure

Failure uses the produced:

```text
RangeError
```

in existing Result/local-handler lowering.

No slice-specific Result representation.

---

# 9. Dynamic source validity

When the source is a dynamically validated existing slice:

```text
sec.ref.is_valid
cf.cond_br
false -> sec.fail.reference_generation
true  -> slice range/index/len operation
```

This validity branch precedes fallible range/index failure.

---

# 10. Shared slice copy

Lower resolved shared slice copy to:

```text
sec.slice.copy_shared
```

when a distinct semantic copy is represented.

Do not copy elements.

---

# 11. Mutable slice move

Lower resolved mutable slice transfer to:

```text
sec.slice.move_mut
```

when a distinct moved SSA value is represented.

Do not copy descriptor semantics.

---

# 12. Slice length

`.len`:

```text
sec.slice.len
```

`len(slice)`:

```text
sec.slice.len_int
```

The latter is generated only when current representability requirements are
satisfied.

---

# 13. Slice index

Preserve:

```text
slice expression once
index expression once
validity guard when required
bounds guard when required
element Place
```

---

# 14. Proven slice index

Emit:

```text
sec.slice.element_place
bounds_kind = proven-safe
```

with proof provenance.

No runtime bounds branch.

---

# 15. Runtime slice index

Emit:

```text
sec.slice.index_in_bounds
cf.cond_br
```

Failure:

```text
ordinary -> sec.fail.bounds(slice-index)
fallible -> IndexError.OutOfBounds flow
```

Success:

```text
sec.slice.element_place
```

---

# 16. Element read

Use:

```text
sec.place.read
```

on the element Place.

P16 complete value-read subset remains copy-trivial.

---

# 17. Element write

Use:

```text
sec.place.write
```

on a writable mutable-slice element Place.

Do not add array-style whole-aggregate replacement because the slice does not
own the sequence.

---

# 18. Element borrow

Use:

```text
sec.ref.borrow_shared
sec.ref.borrow_mut
```

on the slice element Place.

No whole-sequence copy.

---

# 19. Slice borrow end

Use/generalize:

```text
sec.ref.end_borrow
```

for slice BorrowID/LifetimeID.

No runtime destructor.

---

# 20. Returning slices

Keep high-level slice type in function result.

Preserve returned-reference origin/range metadata.

No ABI descriptor choice.

---

# 21. Function parameters

Keep:

```text
!sec.slice<T>
!sec.slice_mut<T>
```

in high-level signatures.

Explicit fixed-array-to-slice source construction remains visible at the caller.

---

# 22. P13 integration

Struct fields containing slices remain high-level.

No descriptor field expansion.

---

# 23. P14 integration

Fixed-array source stays as high-level Place.

No fixed-array value copy is needed to create a slice.

---

# 24. P15 integration

Reuse:

```text
reference validity
place authority
storage identity
borrow/lifetime
epoch dependency
relocation class
reference generation failure
```

No duplicate logic.

---

# 25. Dynamic owning array boundary

If the source slice plan identifies:

```text
owning dynamic T[]
```

and canonical owning-array Semantic IR is unavailable:

```text
return explicit UnsupportedFeatureError
```

Do not fake the owner as a slice.

---

# 26. Effect semantics

Ordinary runtime range/index failure:

```text
panic.bounds
```

Fallible range:

```text
RangeError
```

Fallible index:

```text
IndexError
```

Dynamic stale slice:

```text
panic.invalid-reference-generation
```

These effects remain distinct.

---

# 27. Unsafe

No P16 lowering path removes bounds or validity checks because the source is
inside `unsafe`.

Unchecked access remains RawPtr-only.

---

# 28. No equality lowering

Do not lower slice `==`/`!=`.

They are source-invalid in Sec 0.1.

---

# 29. No physical lowering

Do not lower to:

```text
LLVM ptr
LLVM struct
MemRef
address+length tuple
address+length+epoch tuple
capability
foreign span
```

in lowering version 12.

---

# 30. Optimization independence

Correctness does not depend on:

```text
range-check elimination
bounds-check elimination
generation-check elimination
CSE
SROA
mem2reg
inlining
```

Proof classification exists before optional optimization.

---

# 31. Completion

Lowering version 12 is complete when:

```text
explicit slice creation remains explicit
ranges normalize correctly
source/endpoints evaluate once
RangeError/panic paths are distinct
shared/mutable authority is preserved
slice reborrow narrows
slice indexing produces Places
P15 read/write/ref operations are reused
dynamic source validity precedes spatial failure
returned origin metadata survives
owning T[] remains distinct
no physical slice descriptor is chosen
```
