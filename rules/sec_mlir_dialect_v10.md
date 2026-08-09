# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `10`

Schema version 10 adds canonical high-level fixed-array semantic values,
compact construction/defaults, bounds predicates, guarded element operations
and the ordinary bounds-failure endpoint.

Physical array representation remains deferred.

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
v10  fixed-array semantic values and bounds flow
```

Compiler-generated v10 modules carry:

```mlir
sec.dialect_version = 10 : i32
```

Schema versions 1 through 9 remain regression inputs.

---

# 2. `!sec.array`

Canonical conceptual syntax:

```text
!sec.array<element-type, "length">
```

Examples:

```text
!sec.array<i32, "4">
!sec.array<i128, "2">
!sec.array<!sec.struct<...>, "0">
```

---

# 3. Length syntax

The length parameter is a StringAttr containing canonical base-10
arbitrary-precision unsigned text.

Valid:

```text
"0"
"1"
"4294967295"
"18446744073709551615"
```

Invalid:

```text
"-1"
"+1"
"00"
"01"
" 4 "
```

---

# 4. Why the length is textual

The semantic length is not assigned an arbitrary MLIR integer bit width.

Canonical decimal text:

```text
preserves arbitrary precision
prints deterministically
avoids host-width dependence
remains independent of physical index type
```

---

# 5. Array type identity

Type identity includes:

```text
element type
exact length
```

`!sec.array<i32,"4">` differs from:

```text
!sec.array<i32,"5">
!sec.array<ui32,"4">
```

---

# 6. No dynamic-array use

`!sec.array` in schema v10 means only:

```text
fixed T[N]
```

It never means:

```text
dynamic T[]
slice
```

Those receive separate future representations.

---

# 7. `sec.array.construct`

Variadic operands represent compact literal segments.

Required arrays:

```text
segment_kinds
segment_lengths
segment_actions
```

Each array length equals operand count.

Allowed segment kinds:

```text
element
spread
```

---

# 8. Element segment

For an element segment:

```text
operand type == result element type
segment length == "1"
segment action == "construct-direct"
```

---

# 9. Spread segment

For a spread segment:

```text
operand type == !sec.array<T,"M">
segment length == "M"
segment action == "copy-trivial"
```

P14 compiler output does not emit move/semantic-copy spread.

---

# 10. Construct result

Result:

```text
!sec.array<T,"N">
```

Verifier performs arbitrary-precision checked sum:

```text
sum(segment_lengths) == N
```

No expansion into N operands.

---

# 11. `sec.array.default`

No operands.

Result:

```text
!sec.array<T,"N">
```

Meaning:

```text
fully initialized canonical array default
```

For `N == 0` no element default is constructed.

Builder/Semantic IR verification owns defaultability and ownership checks.

---

# 12. `sec.array.len`

Operand:

```text
!sec.array<T,"N">
```

Result:

```text
!sec.uint
```

before scalar target resolution, or the plan-resolved unsigned scalar after
P6.

The semantic value is exactly N.

Pure and foldable.

---

# 13. `sec.array.index_in_bounds`

Operands:

```text
array: !sec.array<T,"N">
index: integer semantic value
```

Required:

```text
index_signed: BoolAttr
```

Result:

```text
i1
```

Total operation.

No physical address computation.

---

# 14. `sec.array.extract`

Operands:

```text
array
index
```

Result:

```text
T
```

Required:

```text
bounds_kind
bounds_proof
action
```

Allowed P14 values:

```text
bounds_kind:
    proven-safe
    runtime-check

action:
    copy-trivial
```

---

# 15. `sec.array.replace`

Operands:

```text
array
index
new_value
```

Result:

```text
same !sec.array<T,"N">
```

Required:

```text
bounds_kind
bounds_proof
```

The new value type must exactly equal T.

P14 generator separately requires trivial safe replacement semantics.

---

# 16. Proven bounds metadata

Canonical proof strings:

```text
constant
range
branch
contract
analysis
```

For runtime-guarded operations:

```text
bounds_proof = "guarded"
```

These strings are compiler provenance.

They are not source promises.

---

# 17. `sec.fail.bounds`

No operands.

No results.

No successors.

Terminator.

Required:

```text
operation = "fixed-array-index"
```

Semantic panic reason:

```text
panic.bounds
```

It does not select the physical panic endpoint.

---

# 18. Index guard verifier

Register:

```text
--sec-verify-array-index-guards
```

Runtime-check extract/replace requires:

```text
dominating sec.array.index_in_bounds
same array SSA
same index SSA
true-edge dominance
```

Proven-safe operations require valid proof provenance.

---

# 19. Ordinary runtime check shape

Canonical:

```text
index_in_bounds
cf.cond_br

false:
    sec.fail.bounds

true:
    sec.array.extract or sec.array.replace
```

No direct backend trap.

---

# 20. Fallible check shape

Fallible indexing:

```text
index_in_bounds
cf.cond_br

false:
    canonical IndexError.OutOfBounds enum value
    Result/handler flow

true:
    sec.array.extract / replacement path
```

No `sec.fail.bounds` on the handled failure path.

---

# 21. Array storage

High-level:

```text
!sec.storage<!sec.array<T,"N">>
```

may exist for the P14 trivial subset.

Schema v10 does not define a physical MemRef/LLVM aggregate representation.

---

# 22. P5 boundary

`sec-lower-trivial-core` does not lower struct or fixed-array aggregate storage
to MemRef.

Scalar storage behavior remains unchanged.

---

# 23. P6 target resolution

P6 may resolve target-sized scalar types recursively inside the array element
type while preserving:

```text
array wrapper
exact length
nested shape
```

It also resolves `sec.array.len` result representation.

---

# 24. P8 boundary

P8 does not recursively erase integer signedness inside `!sec.array`.

Dedicated aggregate representation lowering owns that transition later.

---

# 25. P13 integration

`!sec.struct` fields may contain `!sec.array`.

`!sec.array` elements may contain `!sec.struct`.

Both wrappers remain high-level.

---

# 26. Zero length

Schema v10 permits:

```text
!sec.array<T,"0">
```

It has no element value.

`sec.array.default` is valid for zero length without requiring one T default.

`sec.array.extract` still requires a proof/guard; no valid safe proof can exist
for any actual index into length zero.

---

# 27. No physical layout

Schema v10 does not define:

```text
element stride
byte size
alignment
element pointer
LLVM array
MemRef
GEP
```

---

# 28. Schema-v10 completion

Schema v10 is complete when:

```text
array type parses/prints/verifies
large exact lengths round-trip
construct segments verify without expansion
default/len operations verify
index predicate verifies
guarded extract/replace verify
ordinary bounds endpoint verifies
schema-v9 regressions remain valid
no dynamic/slice type is represented as !sec.array
no physical array layout is selected
```
