# Semantic IR Amendment - Binary Floating Point

Created: 2026-08-11  
Last updated: 2026-08-11  
Revision: 1  
Sec language version: 0.1  
## Status

Normative amendment for `rules/semantic_ir.txt`.

Package: `SEC-MLIR-P20`  
Repository baseline: `e0af215`

---

# 1. Formats and provenance

Semantic formats:

```text
float32 -> binary32
float64 -> binary64
float   -> binary64
```

Retain source scalar identity independently.

---

# 2. Float constants

`ConstFloatOp` retains:

```text
resolved source float kind
resolved binary format
canonical exact bit pattern
source lexeme/provenance
```

The bit pattern is produced by one direct nearest-even rounding from the exact
source numeric value.

---

# 3. Floating operation policy

Record:

```text
rounding = nearest-even
fast-math = none
subnormals = preserve
contraction = forbidden
```

---

# 4. Floating operations

Explicit Semantic IR operations cover:

```text
unary plus
negation
add/sub/mul/div/rem
eq/ne/lt/le/gt/ge
Abs
Min/Max
Clamp
classification
Floor/Ceiling/Truncate/Round
associated properties
```

---

# 5. Checked numeric conversion

Recognize `core::NumericConversionError` with:

```text
OutOfRange
PrecisionLoss
NotFinite
```

Internal reason domain:

```text
None
OutOfRange
PrecisionLoss
NotFinite
```

Invariant:

```text
failed false <=> None
failed true  <=> non-None
```

---

# 6. Exact conversion

`NumericConvertExactOp` records proven/intrinsic exact conversion.

No failure edge.

---

# 7. Checked conversion

`NumericConvertCheckedOp` produces:

```text
candidate
failed
reason
```

Candidate consumption is success-only.

---

# 8. Supported conversion scope

```text
builtin int/uint <-> binary float
binary float <-> binary float
binary float -> bool
```

Decimal remains P21.

---

# 9. Safety requirements

Integer-to-float must validate exact representability.

Float-to-integer must validate:

```text
finite
range
integral
```

Float narrowing must validate finite range and round-trip exactness.

---

# 10. Literal shaping distinction

Literal shaping does not construct `NumericConversionError`.

Typed-value conversion does.

---

# 11. Constant folding

Compile-time evaluation must exactly match runtime P20 semantics.

---

# 12. Effects

P20 scalar operations are allocation-free and memory-effect-free.

They do not require allocation context.

---

# 13. Verification

Verify:

```text
constant bit pattern
format
rounding
fast-math absence
comparison/remainder kind
core method kind
conversion reason invariant
success-only candidate use
```
