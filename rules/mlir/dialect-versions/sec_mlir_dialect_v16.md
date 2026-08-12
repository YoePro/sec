# Sec MLIR Dialect - Schema Version 16

Created: 2026-08-11
Last updated: 2026-08-11
Revision: 1
Sec language version: 0.1
## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `16`

---

# 1. Module version

```mlir
sec.dialect_version = 16 : i32
```

---

# 2. Existing float types

Reuse:

```text
!sec.float
f32
f64
```

P6 resolves:

```text
!sec.float -> f64
```

Reuse `sec.scalar_kind` for `float`, `float32`, and `float64`.

---

# 3. Numeric conversion reason

Add:

```text
!sec.numeric_conversion_reason
```

Values:

```text
none
out-of-range
precision-loss
not-finite
```

---

# 4. Refined `sec.const.float`

Required resolved data:

```text
lexeme
scalar kind
binary format
exact bit pattern
```

Verifier checks direct nearest-even literal shaping.

---

# 5. New floating operations

```text
sec.float.unary_plus
sec.float.neg
sec.float.binary
sec.float.cmp
sec.float.abs
sec.float.minimum
sec.float.maximum
sec.float.clamp_checked
sec.float.classify
sec.float.round
sec.float.property
sec.float.to_bool
```

---

# 6. `sec.float.binary`

Kinds:

```text
add
sub
mul
div
rem
```

Attrs:

```text
rounding = nearest-even
fast_math = none
```

`rem` additionally records truncation remainder semantics.

---

# 7. `sec.float.cmp`

Predicates:

```text
eq
ne
lt
le
gt
ge
```

Result `i1`.

---

# 8. Classification and rounding

Classification kinds:

```text
finite
infinite
nan
normal
subnormal
```

Rounding kinds:

```text
floor
ceiling
truncate
nearest-even
```

---

# 9. Properties

```text
min
max
epsilon
infinity
negative-infinity
nan
```

---

# 10. Checked clamp

`sec.float.clamp_checked` results:

```text
T
i1 failed
```

Failure means `RangeError.InvalidRange`.

---

# 11. Numeric conversion operations

```text
sec.numeric.convert_exact
sec.numeric.convert_checked
sec.numeric_conversion_error.from_reason
```

Checked result shape:

```text
Target
i1
!sec.numeric_conversion_reason
```

---

# 12. Standard lowering map

```text
add -> arith.addf to_nearest_even
sub -> arith.subf to_nearest_even
mul -> arith.mulf to_nearest_even
div -> arith.divf to_nearest_even
rem -> arith.remf

eq -> arith.cmpf oeq
ne -> arith.cmpf une
lt -> arith.cmpf olt
le -> arith.cmpf ole
gt -> arith.cmpf ogt
ge -> arith.cmpf oge

Abs      -> math.absf
Floor    -> math.floor
Ceiling  -> math.ceil
Truncate -> math.trunc
Round    -> math.roundeven

Min -> arith.minimumf
Max -> arith.maximumf
```

No fast-math.

---

# 13. Classification lowering

Use:

```text
arith.bitcast
arith.andi
arith.cmpi
```

with format-specific exponent/fraction masks.

No runtime library is required.

---

# 14. Checked conversion safety

`arith.fptosi`/`arith.fptoui` may appear only after finite, range, and integral
guards.

Integer-to-float final casts appear only after exactness/range proof.

---

# 15. Verifiers

Register:

```text
--sec-verify-float-semantics
--sec-verify-numeric-conversions
```

---

# 16. No fast-math/denormal weakening

Ordinary compiler-generated P20 output rejects:

```text
non-empty fast-math
arith.flush_denormals
```

until a future explicit semantic rule permits them.

---

# 17. No LLVM requirement

Schema v16 lowers to standard:

```text
Arith
Math
CF
Func
```

LLVM is outside P20.
