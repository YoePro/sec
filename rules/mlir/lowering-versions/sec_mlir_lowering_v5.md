# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `5`

This document is subordinate to:

```text
rules/errors/runtime_checks.md
rules/errors/errorhandling.txt
rules/foundations/operators.md
rules/errors/panic.md
rules/library/core-library.md
rules/compiler/semantic_ir.txt
rules/mlir/sec_mlir.md
rules/mlir/sec_mlir_dialect.md
```

Version 5 extends checked integer lowering with typed failure reasons and the
first fallible arithmetic Result path.

---

# 1. Failure reason model

Every compiler-generated checked integer operation now semantically produces:

```text
result
failed
reason
```

Reason values:

```text
none
overflow
division_by_zero
invalid_shift
```

Invariant:

```text
failed == (reason != none)
```

---

# 2. Reason mapping

Canonical:

```text
neg/add/sub/mul overflow
    -> overflow

division or remainder zero divisor
    -> division_by_zero

signed min/-1 division/remainder
    -> overflow

negative or too-large shift count
    -> invalid_shift

signed-left representability failure
    -> overflow
```

---

# 3. Reason priority

Division/remainder:

```text
if rhs == 0:
    division_by_zero
else if signed min/-1:
    overflow
else:
    none
```

Signed left shift:

```text
if count invalid:
    invalid_shift
else if result unrepresentable:
    overflow
else:
    none
```

This priority is semantic.

Safe substitute computations do not change it.

---

# 4. Package 8 reason generation

Package 8 total Arith lowering computes both:

```text
failed
reason
```

using total comparisons/selects.

Examples:

```text
add:
    reason = select failed, overflow, none

unsigned divide:
    reason = select divByZero, division_by_zero, none

signed divide:
    reason =
        select divByZero,
            division_by_zero,
            select signedOverflow, overflow, none
```

Equivalent total SSA is allowed.

---

# 5. Reason representation remains high-level

Package 8/P9 may leave:

```text
!sec.arithmetic_failure_reason
```

and reason constants/selects in mixed MLIR.

No physical `i2`/`i8` representation is selected by lowering version 5.

---

# 6. Ordinary arithmetic failure

Ordinary checked arithmetic routes:

```text
failed true
    -> sec.fail.arithmetic(reason)
```

The op remains high-level.

It is the semantic panic-capable endpoint.

---

# 7. Fallible arithmetic failure

Naked arithmetic `try` routes:

```text
failed true
    -> arithmetic_error.from_reason
    -> result.err
    -> func.return
```

No arithmetic panic endpoint occurs on that failure path.

---

# 8. ArithmeticError mapping

```text
overflow
    -> ArithmeticError.Overflow

division_by_zero
    -> ArithmeticError.DivisionByZero

invalid_shift
    -> ArithmeticError.InvalidShift
```

No other mapping is permitted in lowering v5.

---

# 9. Exact propagation requirement

Naked Package 9 arithmetic propagation requires the enclosing function's error
type to be exactly:

```text
ArithmeticError
```

No implicit union mapping.

No automatic wrapper selection.

---

# 10. Result remains high-level

Package 9 does not lower:

```text
!sec.result
sec.result.ok
sec.result.err
!sec.core_error
sec.arithmetic_error.from_reason
```

to physical representation.

They remain legal after integer Arith lowering.

---

# 11. Effect distinction

Ordinary unchecked-by-proof arithmetic:

```text
may panic
```

Fallible arithmetic under `try`:

```text
explicit ArithmeticError flow
no arithmetic panic effect
```

Operand effects remain unchanged.

---

# 12. Panic reason mapping

Future panic lowering maps ordinary arithmetic reason to:

```text
panic.arithmetic-overflow
panic.division-by-zero
panic.invalid-shift
```

No root panic ABI is selected by lowering v5.

---

# 13. No mandatory runtime

Fallible arithmetic lowers through:

```text
SSA
comparisons
branches
static error values
Result construction
```

No exception runtime is introduced.

---

# 14. Checked guard verifier v5

The verifier checks:

```text
reason is passed on the failure edge
reason matches the checked op's result
none cannot reach failure endpoint
ordinary and fallible endpoints are structurally valid
checked result is success-only
```

---

# 15. Local handler boundary

Lowering version 5 does not implement:

```text
try expression { handlers }
```

That requires handler selection, exhaustiveness and explicit mapping CFG.

---

# 16. General Result-call boundary

Lowering version 5 does not yet unwrap arbitrary:

```text
call returning Result[T,E]
```

through naked `try`.

Package 9 is specifically the arithmetic-check path.

---

# 17. Completion

Lowering version 5 is complete when:

```text
P8 computes exact reason
ordinary failure preserves reason
fallible arithmetic constructs exact ArithmeticError
Result Err returns correctly
no arithmetic panic endpoint exists on fallible failure path
Result/error physical representation remains deferred
no runtime dependency is introduced
```
