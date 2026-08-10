# Package 16 Normative Synchronization - Slices

## Status

Normative synchronization for:

```text
rules/arrays-slices.txt
rules/runtime_checks.md
rules/core-library.md
rules/reference_model.md
```

Package:

```text
SEC-MLIR-P16
```

Repository baseline:

```text
152c772
```

Local predecessor reference model:

```text
SEC-MLIR-P15
```

This synchronization resolves source/lowering ambiguities required for canonical
slice Semantic IR.

---

# 1. Slice creation remains explicit

Canonical slice values are created with:

```sec
ref source[range]
ref mut source[range]
```

There is no implicit array-to-slice conversion.

A bare range expression does not by itself create a source-level slice value.

---

# 2. Fallible slice syntax uses explicit reference syntax

Replace illustrative:

```sec
let part := try values[start..<end]
```

with canonical:

```sec
let part := try ref values[start..<end]
```

or:

```sec
let part := try (ref values[start..<end])
```

Mutable:

```sec
let part := try ref mut values[start..<end]
```

This keeps `try` semantics consistent with explicit slice construction.

---

# 3. Canonical normalized range

Every accepted slice range normalizes to:

```text
[start, endExclusive)
```

Open bounds:

```text
missing start -> 0
missing end   -> source length
```

Inclusive end:

```text
endExclusive = end + 1
```

after validation.

---

# 4. Empty slices

Exclusive equal ranges are valid:

```sec
ref source[2..<2]
```

A complete zero-length source produces a valid empty slice.

Empty slice semantics do not imply nullable safe references.

---

# 5. Runtime RangeError mapping

Direct slice-range checks use the core enum:

```sec
RangeError.InvalidRange
RangeError.StartAfterEnd
RangeError.OutOfBounds
```

Minimum mapping:

```text
negative runtime endpoint
    InvalidRange

explicit start > explicit represented end
    StartAfterEnd

start outside source
exclusive end > source length
inclusive end >= source length
    OutOfBounds

other unrepresentable range normalization
    InvalidRange
```

Compile-time-known invalid cases remain compile-time diagnostics.

---

# 6. Failure precedence

When several invalid conditions overlap, choose deterministically:

```text
1. invalid/negative endpoint
2. start-after-end
3. out-of-bounds
```

Optimization preserves the selected error identity.

---

# 7. Ordinary range failure

Ordinary unproven slice construction is panic-capable.

Stable panic reason:

```text
panic.bounds
```

Operation provenance:

```text
slice-range
```

---

# 8. Fallible range failure

`try` converts the range validation failure to:

```text
RangeError
```

It does not convert stale direct-reference failure.

---

# 9. Slice indexing error

Slice element indexing uses:

```text
IndexError.OutOfBounds
```

on the fallible path.

Ordinary unproven indexing uses:

```text
panic.bounds
```

with operation provenance:

```text
slice-index
```

No `BoundsError` type is introduced.

---

# 10. Reference validity precedes spatial failure

When a source slice requires dynamic direct-reference validity validation:

```text
evaluate source/endpoints/index
validate reference generation/provenance
then perform range/index validation
```

Therefore:

```sec
try staleSlice[index]
```

does not turn stale-reference failure into IndexError.

Stale direct slice semantics remain:

```text
panic.invalid-reference-generation
```

---

# 11. Static range precision

Static slice bounds are compile-time integers and must not be limited to
compiler-host `int64`.

Use arbitrary precision.

Nested static slice ranges compose through arbitrary-precision half-open
arithmetic.

---

# 12. Dynamic range precision

Dynamic/symbolic slice bounds remain conservatively overlapping unless proof
establishes disjointness.

Do not invent runtime borrow checks.

---

# 13. Slice classification

Canonical:

```text
ref T[]
    copyable shared bounded reference

ref mut T[]
    move-only exclusive bounded reference
```

Both are trivially destructible.

Neither owns elements.

---

# 14. Slice equality remains unsupported

Do not expose reference equality for slice syntax.

Slice `==`/`!=` remain invalid in Sec 0.1.

Content equality, descriptor equality and range identity are distinct possible
future operations.

---

# 15. `.len` remains `uint`

Intrinsic:

```sec
slice.len
```

returns:

```text
uint
```

and is the runtime element count.

---

# 16. Compiler-known `len(slice)` remains `int`

The existing core rule remains:

```sec
len(slice) -> int
```

P16 does not silently change that decision.

---

# 17. Unresolved representability rule

The current normative rules do not specify what happens if a runtime slice
length:

```text
fits uint
does not fit int
```

This synchronization intentionally does not invent semantics.

Until a later normative decision is made:

```text
new Semantic IR may emit len(slice) -> int only when representability is proven
```

Otherwise report an explicit implementation gap.

Forbidden behavior:

```text
wrap
truncate
saturate
silently narrow
change return type
silently reduce maximum slice length
```

This issue should be resolved in the compiler-known length/core synchronization
work.

---

# 18. Dynamic owning arrays remain separate

Bare:

```sec
T[]
```

is an owning dynamic sequence descriptor.

It is not a slice.

P16 does not use the slice representation for it.

---

# 19. Required synchronization

Update at least:

```text
arrays-slices fallible examples
runtime-check slice example
Sema range constant representation
Place slice projection representation
core error lowering references
Semantic IR documentation
tests/manual examples derived from these rules
```
