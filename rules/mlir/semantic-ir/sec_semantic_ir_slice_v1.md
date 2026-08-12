# Semantic IR Amendment - Slice Values

## Status

Normative amendment for:

```text
rules/compiler/semantic_ir.txt
```

Package:

```text
SEC-MLIR-P16
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
```

This amendment defines canonical target-independent Semantic IR for safe borrowed
slices.

---

# 1. Slice semantic category

A slice is a bounded direct safe reference over a contiguous sequence.

Canonical source categories:

```text
shared slice:  ref T[]
mutable slice: ref mut T[]
```

They are distinct from owning dynamic `T[]`.

---

# 2. Slice value facts

A slice value retains:

```text
element TypeID
shared/mutable authority
runtime length
normalized spatial range
origin Places
storage identities
BorrowID
LifetimeID
validity policy
epoch dependency
relocation class
address space
```

These facts may be attached to the value or stored in an analysis side table.

---

# 3. Runtime length

Slice runtime length is semantic value data.

It is not part of type identity.

Two slices of the same element type and authority but different lengths have
the same source type.

---

# 4. Normalized range

Every valid slice range uses canonical half-open form:

```text
[start, endExclusive)
```

Static endpoints use arbitrary precision.

Dynamic endpoints use stable compiler analysis identity.

---

# 5. Empty slices

Length-zero slices are valid.

They retain origin/borrow facts even though no element can be accessed.

No element Place may be produced without a successful bounds proof/check.

---

# 6. Explicit construction

Semantic IR produces a source slice only from an explicit source borrow or
reborrow plan.

A bare source SliceExpression is not enough.

---

# 7. Fixed-array slice borrow

Slice borrow from a fixed-array Place records:

```text
source Place
normalized start
normalized length
BorrowID/LifetimeID
shared/mutable authority
reference validity facts
```

It does not copy the array.

---

# 8. Slice reborrow

Reborrow from another slice:

```text
preserves storage identity
narrows/equal spatial extent
narrows/equal lifetime
preserves or weakens authority
preserves compatible epoch/relocation/address-space facts
```

Shared cannot become mutable.

---

# 9. Shared slice copy

Shared slice copy is explicit semantic reference-value copy.

It does not copy elements.

---

# 10. Mutable slice move

Mutable slice transfer is move-only.

It transfers holder/exclusive-reference value semantics.

It does not move elements.

---

# 11. Range normalization

Semantic IR distinguishes:

```text
proven range normalization
runtime range check
```

A proven range produces normalized start/length with proof provenance.

A runtime check additionally produces:

```text
failed
RangeError
```

---

# 12. Range error identity

Fallible slice range uses the canonical core `RangeError`.

No slice-specific error representation is introduced.

---

# 13. Ordinary range failure

Ordinary unhandled range failure reaches the canonical bounds panic path with
slice-range provenance.

---

# 14. Slice length

Semantic IR distinguishes:

```text
SliceLength -> uint
SliceLengthInt -> int
```

because source `.len` and compiler-known `len(...)` are distinct resolved core
operations.

No implicit narrowing is hidden between them.

---

# 15. Element indexing

Slice indexing produces a semantic Place.

It does not inherently copy/move/borrow the element.

Element use context consumes the Place through the P15 place/reference model.

---

# 16. Index bounds

Slice index validity:

```text
0 <= index < runtime slice length
```

Semantic IR distinguishes proven-safe and runtime-checked access.

Runtime-checked element Place formation is control-flow guarded.

---

# 17. Index error identity

Fallible slice index uses:

```text
IndexError.OutOfBounds
```

Ordinary failure uses:

```text
panic.bounds
```

No parallel BoundsError type.

---

# 18. Temporal before spatial validity

When a source slice requires dynamic reference validation, that validation
dominates the spatial range/index operation.

`try` converts only the spatial failure.

---

# 19. Element read/write/borrow

Semantic IR does not add separate ownership operations for slice elements.

Use existing P15:

```text
PlaceRead
PlaceWrite
ReferenceBorrowShared
ReferenceBorrowMutable
```

on the slice element Place.

---

# 20. Ownership boundary

P16 complete whole-value element reads/replacements support the existing trivial
subset only.

Moving an element out of a slice is not introduced.

---

# 21. Borrow end

Slice lifetime end reuses the canonical direct-reference borrow-end marker.

Slice destruction does not destroy elements or backing storage.

---

# 22. Returned slices

Function return metadata extends direct-reference origin summary with:

```text
slice element type
authority
origin storage class
known normalized range where available
conservative unknown range where not
epoch/relocation dependencies
```

Returning local-origin slices remains invalid.

---

# 23. Dynamic owning sequence boundary

Owning `T[]` is not represented as a slice.

P16 new IR may reject a dynamic-owner-to-slice source until owning dynamic-array
Semantic IR exists.

---

# 24. Physical layout separation

Semantic IR does not define:

```text
base pointer field
length field storage type
epoch field
fat-pointer struct
FFI layout
```

Profile lowering decides the descriptor representation later.

---

# 25. Verifier

Semantic IR verifier must validate:

```text
shared/mutable slice kind
range normalization/proof
range failure/error CFG
range narrowing on reborrow
shared/mutable authority
shared copy / mutable move
runtime length type
slice index guard
element Place type/authority
reference validity ordering
BorrowID/LifetimeID
returned-origin legality
no owning dynamic array conflation
```

---

# 26. Deterministic printer

Print:

```text
slice kind
element type
BorrowID/LifetimeID
validity policy
storage identity
known normalized range
dynamic SliceRangeID
```

Do not print physical address assumptions.
