# Package 14 Normative Synchronization - Fixed Arrays

## Status

Normative synchronization for:

```text
rules/collections/collections.md
rules/errors/runtime_checks.md
rules/library/core-library.md
```

Package:

```text
SEC-MLIR-P14
```

Repository baseline:

```text
152c772
```

This document resolves three implementation/documentation inconsistencies before
canonical fixed-array Semantic IR is implemented.

---

# 1. Fixed-array literal spread is current syntax

`rules/collections/collections.md` contains older initial-scope wording stating that array
literals do not include spread.

That statement is superseded by the dedicated newer:

```text
rules/declarations/spread.md
```

which defines and marks implemented:

```text
fixed-size array spread in array literals
multiple spreads
compile-time result length
copyability validation
no hidden allocation
```

The array rulebook must be synchronized accordingly.

Canonical valid example:

```sec
let first: int[2] := [1, 2]
let combined := [first..., 3, 4]
```

Result:

```sec
int[4]
```

---

# 2. Fixed-array spread constraints

A spread source in a fixed-array literal:

```text
must itself have compile-time fixed length
must have compatible element type
must obey ordinary implicit-copy rules
must not be a runtime-length slice
must not allocate backing storage
must not partially move a move-only array
```

Multiple fixed-array spreads are valid.

The total result length is checked compile-time arithmetic.

---

# 3. Fixed-array length is not an `int64` language limit

The source-language array length rule is:

```text
non-negative compile-time integer
representable by target uint
total layout representable by target
```

It is not:

```text
representable by compiler-host int64
```

The canonical compiler representation must therefore preserve exact
arbitrary-precision length until target-plan validation.

A compiler implementation may use a smaller legacy cache only when it is not
semantic authority.

---

# 4. Fixed versus dynamic owning arrays

Canonical semantic distinction:

```text
T[N]
    fixed array
    length is part of type identity

T[]
    dynamic owning array/sequence
    runtime length
    no compile-time length in type identity
```

Do not use a magic semantic length value such as `-1` to make these two shapes
indistinguishable in the canonical type model.

---

# 5. Target uint validation

The fixed-array length must fit the active target's canonical pointer-sized
`uint`.

Therefore:

```text
32-bit plan:
    N <= uint32.Max

64-bit plan:
    N <= uint64.Max
```

Multi-output builds validate each output independently.

---

# 6. Canonical fixed-index fallible error

`rules/errors/runtime_checks.md` currently says bounds access produces:

```text
BoundsError or the canonical equivalent
```

The canonical core error for element indexing is:

```sec
enum IndexError {
    OutOfBounds
}
```

Therefore fixed-array element indexing uses:

```text
IndexError.OutOfBounds
```

for the fallible path.

No additional `BoundsError` type is required.

---

# 7. Range errors remain separate

This synchronization does not redefine slice/range validation.

Range-shaped failures continue to use the canonical range error domain:

```sec
enum RangeError {
    InvalidRange
    StartAfterEnd
    OutOfBounds
}
```

as synchronized by the slice/range rulebooks.

---

# 8. Ordinary indexing panic reason

Ordinary unhandled fixed-array bounds failure remains panic-capable.

Canonical stable panic reason:

```text
panic.bounds
```

No mandatory runtime endpoint is implied.

---

# 9. Zero-length arrays

`T[0]` is valid.

It contains no live elements.

It is defaultable without constructing an element.

It has no valid element index.

This remains true even when constructing a standalone default `T` would be
invalid.

---

# 10. Implementation synchronization

Update:

```text
arrays-slices rule text
runtime-check error wording
Sema fixed-array length representation
Sema literal-spread length logic
Sema constant index checking
core error registration if IndexError is not already registered before user Sema
tests and derived manuals
```

Do not change the source syntax to solve implementation limitations.
