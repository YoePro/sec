# Package 20 Normative Synchronization - Binary Floating Point and Numeric Conversion

Created: 2026-08-11  
Last updated: 2026-08-11  
Revision: 1  
Sec language version: 0.1  
## Status

Normative synchronization for:

```text
rules/types.md
rules/operators.md
rules/core-library.md
rules/runtime_checks.md
rules/compiler_known_members.md
```

Package: `SEC-MLIR-P20`  
Repository baseline: `e0af215`

---

# 1. Binary formats

```text
float32 = binary32
float64 = binary64
float   = binary64 semantic numeric format in Sec 0.1
```

Source identities remain distinct.

---

# 2. Ordinary rounding mode

Ordinary binary floating arithmetic uses round-to-nearest, ties-to-even.

No source-visible dynamic floating rounding environment exists in Sec 0.1.

---

# 3. No contraction or fast-math

Ordinary source operations are not silently fused or reassociated.

No profile may silently enable semantics-changing fast-math.

Subnormal values are preserved.

---

# 4. Literal direct rounding

Binary float literal shaping rounds once from the exact source numeric value
directly to the resolved binary format.

For `float32`, a binary64 intermediate is invalid.

A finite literal outside the finite target range is a compile-time error.

---

# 5. Literal shaping versus conversion

Literal shaping may perform nearest-even format rounding.

Typed runtime conversion is checked and rejects precision loss.

These are separate semantic operations.

---

# 6. Core float constants

Define:

```text
min              most negative finite value
max              largest positive finite value
epsilon          next representable value above 1 minus 1
infinity         positive infinity
negativeInfinity negative infinity
nan              canonical quiet NaN
```

---

# 7. Min/Max

Core Min/Max are NaN-propagating.

Signed zero selection:

```text
Min(-0,+0) -> -0
Max(-0,+0) -> +0
```

---

# 8. Clamp

`Clamp` fails with `RangeError.InvalidRange` when:

```text
minimum is NaN
maximum is NaN
minimum > maximum
```

A NaN value with valid bounds returns `Ok(NaN)`.

---

# 9. Classification

Define total:

```text
IsFinite
IsInfinite
IsNaN
IsNormal
IsSubnormal
```

from the selected binary format.

---

# 10. Rounding methods

```text
Floor    toward negative infinity
Ceiling  toward positive infinity
Truncate toward zero
Round    nearest integral value, halfway cases to even
```

---

# 11. Numeric conversion error

Add:

```sec
enum NumericConversionError {
    OutOfRange
    PrecisionLoss
    NotFinite
}
```

It is a standard core error.

---

# 12. Error priority

```text
NotFinite
OutOfRange
PrecisionLoss
```

---

# 13. Float narrowing

Finite wider-to-narrower conversion:

```text
outside finite target range -> OutOfRange
not exactly representable   -> PrecisionLoss
otherwise                   -> success
```

NaN and infinity map successfully by semantic category.

---

# 14. Integer to float

Typed integer-to-float conversion succeeds only if the integer is exactly
representable.

```text
outside finite float range -> OutOfRange
rounding required          -> PrecisionLoss
```

---

# 15. Float to integer

Requires finite, in-range, integral source.

```text
NaN/infinity -> NotFinite
out of range -> OutOfRange
fractional   -> PrecisionLoss
```

---

# 16. Float to bool

Explicit numeric-to-bool:

```text
±0 -> false
every other floating value, including NaN -> true
```

---

# 17. Comparison mapping

```text
== -> ordered equal
!= -> unordered or not equal
<  -> ordered less
<= -> ordered less or equal
>  -> ordered greater
>= -> ordered greater or equal
```

This preserves the canonical NaN rules.

---

# 18. SquareRoot boundary

`SquareRoot()` remains a core method but its `MathError` semantics and lowering
are deferred to a dedicated core-math package.

---

# 19. Required synchronization

Update:

```text
core standard errors
compiler-known float intrinsic IDs
runtime conversion mapping
Sema numeric conversion plans
Semantic IR float ops
Sec MLIR float ops
legacy float != predicate
float literal direct-width parser
constant folding
derived tests/manuals
```
