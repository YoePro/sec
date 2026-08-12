# Sec MLIR Lowering - Version 16

Created: 2026-08-11
Last updated: 2026-08-11
Revision: 1
Sec language version: 0.1
## Status

Normative lowering specification.

Current lowering specification version: `16`

---

# 1. Preconditions

Before lowering:

```text
Sema resolution complete
Semantic IR verified
P6 scalar layout resolution complete
P20 float/numeric verifiers pass
```

Concrete floating values are `f32`/`f64`.

---

# 2. Constants

Lower `sec.const.float` from the canonical stored bit pattern.

Do not reparse the source lexeme.

```text
binary32 -> arith.constant f32
binary64 -> arith.constant f64
```

---

# 3. Arithmetic

```text
add -> arith.addf to_nearest_even
sub -> arith.subf to_nearest_even
mul -> arith.mulf to_nearest_even
div -> arith.divf to_nearest_even
rem -> arith.remf
```

No fast-math.

No hidden FMA.

---

# 4. Comparisons

```text
eq -> oeq
ne -> une
lt -> olt
le -> ole
gt -> ogt
ge -> oge
```

---

# 5. Classification

For f32:

```text
bits = arith.bitcast f32 -> i32
exp  = bits & 0x7f800000
frac = bits & 0x007fffff
```

For f64:

```text
bits = arith.bitcast f64 -> i64
exp  = bits & 0x7ff0000000000000
frac = bits & 0x000fffffffffffff
```

Derive finite/infinite/NaN/normal/subnormal with integer comparisons.

---

# 6. Core methods

```text
Abs      -> math.absf
Min      -> arith.minimumf
Max      -> arith.maximumf
Floor    -> math.floor
Ceiling  -> math.ceil
Truncate -> math.trunc
Round    -> math.roundeven
```

Clamp validates bounds then uses NaN-propagating maximum/minimum.

---

# 7. Exact widening

```text
f32 -> f64
```

uses `arith.extf`.

`float`/`float64` same-format conversion requires no numeric rounding.

---

# 8. Float narrowing

Canonical f64 -> f32 runtime checked flow:

```text
NaN -> success target NaN
infinity -> success target infinity

candidate = arith.truncf to_nearest_even
finite source -> candidate infinity:
    OutOfRange

widened = arith.extf candidate
widened != source:
    PrecisionLoss

otherwise:
    success candidate
```

---

# 9. Integer to float

Check exactness in integer domain before final conversion.

For nonzero magnitude:

```text
bitLength
target precision 24/53
low discarded-bit mask
finite target range
```

Only on success:

```text
arith.sitofp
arith.uitofp
```

---

# 10. Float to integer

Guard order:

```text
1. finite
2. range
3. integral
```

Failures map:

```text
NotFinite
OutOfRange
PrecisionLoss
```

Only after all guards:

```text
arith.fptosi
arith.fptoui
```

---

# 11. Signed range

For N-bit signed target:

```text
-2^(N-1) <= value < 2^(N-1)
```

---

# 12. Unsigned range

For N-bit unsigned target:

```text
0 <= value < 2^N
```

---

# 13. Integrality

```text
truncated = math.trunc value
integral = arith.cmpf oeq truncated, value
```

---

# 14. Float to bool

```text
arith.cmpf une value, +0
```

This yields false for ±0 and true for NaN/nonzero/infinity.

---

# 15. Conversion control flow

`sec.numeric.convert_checked` lowers to explicit CFG.

Failure converts reason to `NumericConversionError` and uses existing P10/P11
handler/propagation flow.

Candidate is success-only.

---

# 16. Constant conversions

Evaluate the same checks at compile time.

Known failure is a compile-time diagnostic.

---

# 17. Passes

Register:

```text
--sec-lower-checked-numeric-conversions
--sec-lower-float-core
```

Recommended order:

```text
sec-resolve-scalar-layout
sec-verify-float-semantics
sec-verify-numeric-conversions
sec-lower-checked-numeric-conversions
sec-lower-float-core
```

---

# 18. Target capability

If later target lowering cannot preserve:

```text
nearest-even
subnormal behavior
NaN comparison
signed zero
remainder
```

use a preserving helper or reject the target.

Do not weaken standard MLIR semantics.

---

# 19. Legacy parity

Patch remaining legacy float semantics:

```text
float != -> UNE
literal direct-width rounding
truncation remainder
no unsafe fast-math
```

---

# 20. Deferred

Do not lower:

```text
SquareRoot
decimal
decimal128
float formatting
advanced math
totalOrder
lossy conversion APIs
```

in P20.

---

# 21. Completion

Lowering v16 is complete when:

```text
float constants lower without reparse/double rounding
arithmetic uses nearest-even
comparison matches Sec NaN rules
remainder is truncation-based
classification is runtime-library-independent
rounding methods are deterministic
checked conversions cannot execute unsafe casts
NumericConversionError priority is deterministic
constant folding matches runtime
no fast-math/denormal weakening is introduced
no LLVM semantic decision is required
```
