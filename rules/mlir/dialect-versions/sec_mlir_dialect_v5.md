# Sec MLIR Dialect

## Status

Normative detailed representation specification for the Sec MLIR dialect.

Current dialect schema version: `5`

This rulebook is subordinate to:

```text
rules/runtime_checks.md
rules/errorhandling.txt
rules/operators.md
rules/panic.md
rules/core-library.md
rules/semantic_ir.txt
rules/sec_mlir.md
```

Schema version 5 adds typed arithmetic failure flow and the minimum high-level
Result representation needed for naked arithmetic `try`.

---

# 1. Schema version history

```text
v1 foundation
v2 first Semantic IR bridge
v3 scalar/target coverage
v4 checked integer semantic operations
v5 typed arithmetic failure reason + Result construction
```

Compiler-generated v5 modules carry:

```mlir
sec.dialect_version = 5 : i32
```

---

# 2. Core arithmetic error

The canonical core error for fallible builtin arithmetic is:

```sec
enum ArithmeticError {
    Overflow
    DivisionByZero
    InvalidShift
}
```

High-level Sec MLIR does not yet choose the enum's physical integer
representation.

---

# 3. Arithmetic failure reason

New compiler-internal type:

```mlir
!sec.arithmetic_failure_reason
```

Values:

```text
none
overflow
division_by_zero
invalid_shift
```

Invariant for compiler-generated checked operations:

```text
failed == false iff reason == none
failed == true iff reason != none
```

---

# 4. Failure reason constant

Operation:

```text
sec.arithmetic_failure_reason.constant
```

Required enum attribute:

```text
none
overflow
division_by_zero
invalid_shift
```

Result:

```text
!sec.arithmetic_failure_reason
```

No memory effects.

---

# 5. Core error type

New high-level type:

```mlir
!sec.core_error<"identity">
```

Parameter:

```text
identity: StringAttr
```

Rules:

```text
identity non-empty
identity is opaque
type identity includes exact identity string
no universal core-error base is implied
no physical enum layout is implied
```

Package 9 uses:

```mlir
!sec.core_error<"core::ArithmeticError">
```

---

# 6. Result type

New high-level type:

```mlir
!sec.result<T, E>
```

Meaning:

```text
ordinary Sec Result[T,E] semantic value
```

Rules:

```text
T is the success semantic type
E is the exact error semantic type
identity includes both
no tag/payload physical layout is implied
no implicit E conversion
```

A dialect-specific void-success convention may be used for `Result[void,E]`
without introducing a user-visible second return value.

---

# 7. Result Ok construction

Operation:

```text
sec.result.ok
```

Non-void form:

```text
operand: T
result: !sec.result<T,E>
```

Void-success form:

```text
no success operand
result: !sec.result<void,E>
```

No allocation.

No memory effect.

---

# 8. Result Err construction

Operation:

```text
sec.result.err
```

Operand:

```text
E
```

Result:

```text
!sec.result<T,E>
```

Verifier requires exact E equality.

No implicit error mapping.

---

# 9. ArithmeticError conversion

Operation:

```text
sec.arithmetic_error.from_reason
```

Operand:

```text
!sec.arithmetic_failure_reason
```

Result:

```text
!sec.core_error<"core::ArithmeticError">
```

Mapping:

```text
overflow
    -> ArithmeticError.Overflow

division_by_zero
    -> ArithmeticError.DivisionByZero

invalid_shift
    -> ArithmeticError.InvalidShift
```

`none` is not a valid failing-path input.

---

# 10. Checked integer result evolution

Schema-v5 compiler-generated checked integer ops return:

```text
result: T
failed: i1
reason: !sec.arithmetic_failure_reason
```

This applies to the checked v4 operation families when emitted in a schema-v5
module.

Schema-v4 two-result fixtures may remain accepted for regression compatibility.

Implementation may use optional/variadic result plumbing plus a custom
schema-aware verifier if needed to keep one operation name.

Do not create source-visible v5-suffixed operator names.

---

# 11. Failure edge

Canonical:

```mlir
%value, %failed, %reason = "sec.int.binary_checked"(...)
    : (...) -> (T, i1, !sec.arithmetic_failure_reason)

cf.cond_br %failed,
    ^failure(%reason : !sec.arithmetic_failure_reason),
    ^success
```

The true edge is failure.

The false edge is success.

---

# 12. Ordinary arithmetic fail operation

Schema-v5 form:

```text
sec.fail.arithmetic
```

Operand:

```text
reason: !sec.arithmetic_failure_reason
```

No results.

No successors.

Terminator.

The reason operand is authoritative.

Optional:

```text
sec.operator
```

remains diagnostic provenance only.

---

# 13. Ordinary panic reason mapping

Later panic lowering maps:

```text
overflow
    -> panic.arithmetic-overflow

division_by_zero
    -> panic.division-by-zero

invalid_shift
    -> panic.invalid-shift
```

Schema v5 does not choose the physical panic endpoint.

---

# 14. Fallible failure block

Canonical:

```mlir
^failure(%reason: !sec.arithmetic_failure_reason):
    %error = "sec.arithmetic_error.from_reason"(%reason)
        : (!sec.arithmetic_failure_reason)
        -> !sec.core_error<"core::ArithmeticError">

    %err = "sec.result.err"(%error)
        : (!sec.core_error<"core::ArithmeticError">)
        -> !sec.result<T, !sec.core_error<"core::ArithmeticError">>

    func.return %err
```

No panic op exists on this path.

---

# 15. Success result construction

A Result-returning function uses:

```text
sec.result.ok
```

before `func.return`.

The checked arithmetic success value remains an ordinary SSA value until the
function's declared Ok return is constructed.

---

# 16. Error identity

`ArithmeticError` is exact.

Schema v5 does not allow:

```text
automatic wrapping into another union
automatic widening
universal error code conversion
```

That mirrors Sec error-handling rules.

---

# 17. Checked guard verification

Schema-v5 compiler output requires:

```text
failed branch immediately guards checked op
true edge passes exact reason
failure block reason type exact
ordinary endpoint receives reason
fallible endpoint converts reason to ArithmeticError
none cannot be routed to failure endpoint
checked result unavailable on failure path
```

The dedicated checked-integer guard verifier owns cross-operation checks.

---

# 18. Package 8 lowering contract

When lowering schema-v5 checked ops to Arith, Package 8 must replace:

```text
result
failed
reason
```

with total standard MLIR SSA values.

Reason mapping is dynamic where needed.

No failure reason may be reconstructed from source token spelling after the
semantic op is gone.

---

# 19. Result representation boundary

Schema v5 intentionally does not define:

```text
physical discriminant
payload union
memory size
alignment
ABI return classification
LLVM struct
```

Those are later lowering decisions.

---

# 20. Core error representation boundary

Schema v5 intentionally does not define the physical representation of
`ArithmeticError`.

The high-level exact type identity is sufficient for typed control flow.

---

# 21. No runtime requirement

All v5 Result/error operations are high-level static SSA semantics.

They imply no:

```text
heap allocation
exception object
unwinder
global error registry
general runtime
```

---

# 22. Schema-v5 completion

Schema v5 is complete when:

```text
failure reason type and constants verify
core error identity verifies
Result type verifies
Ok/Err construction verifies
ArithmeticError conversion verifies
checked ops expose reason
ordinary fail consumes reason
fallible arithmetic failure constructs Err
schema-v4 regression tests remain accepted
```
