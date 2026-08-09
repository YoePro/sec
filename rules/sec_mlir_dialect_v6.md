# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `6`

Schema version 6 adds semantic Result discrimination/unwrapping and core error
variant testing for local `try` handler control flow.

It does not define physical Result or enum layout.

---

# 1. Version history

```text
v1  dialect foundation
v2  Semantic IR bridge
v3  scalar/target coverage
v4  checked integer operations
v5  typed arithmetic failure and high-level Result construction
v6  Result guard/unwrapping and local try handler support
```

Compiler-generated v6 modules carry:

```mlir
sec.dialect_version = 6 : i32
```

---

# 2. Existing Result type

Unchanged:

```mlir
!sec.result<T,E>
```

No physical representation is implied.

---

# 3. Existing core error type

Unchanged:

```mlir
!sec.core_error<"identity">
```

Package 10 required end-to-end identity:

```text
core::ArithmeticError
```

---

# 4. `sec.result.is_err`

Operand:

```text
result: !sec.result<T,E>
```

Result:

```text
i1
```

Meaning:

```text
false -> Ok
true  -> Err
```

Total operation.

No memory effect.

No physical discriminant representation implied.

---

# 5. `sec.result.unwrap_ok`

Operand:

```text
result: !sec.result<T,E>
```

Result:

```text
T
```

Valid semantic use requires the current control-flow path to be proven the Ok
path of the same Result SSA value.

The operation verifier checks types.

`--sec-verify-result-guards` checks path validity.

For `T=void`, compiler-generated code does not emit this value-producing op.

---

# 6. `sec.result.unwrap_err`

Operand:

```text
result: !sec.result<T,E>
```

Result:

```text
E
```

Valid semantic use requires the Err path of the same Result SSA value.

---

# 7. Canonical Result guard

```mlir
%is_err = "sec.result.is_err"(%result)
    : (!sec.result<T,E>) -> i1

cf.cond_br %is_err, ^err, ^ok

^err:
    %error = "sec.result.unwrap_err"(%result)
        : (!sec.result<T,E>) -> E
    ...

^ok:
    %value = "sec.result.unwrap_ok"(%result)
        : (!sec.result<T,E>) -> T
    ...
```

The true branch is always Err.

The false branch is always Ok.

---

# 8. Result projection is internal

`unwrap_ok` and `unwrap_err` are compiler-internal semantic projections.

They are not source operations.

They do not introduce a new panic/check.

An invalid projection path is malformed compiler IR, not a source runtime
failure.

---

# 9. `sec.core_error.is_variant`

Operand:

```text
!sec.core_error<"core::ArithmeticError">
```

Required attribute:

```text
variant
```

Allowed schema-v6 compiler-generated variants:

```text
Overflow
DivisionByZero
InvalidShift
```

Result:

```text
i1
```

Total.

No memory effect.

No physical enum representation implied.

---

# 10. Variant identity

The variant attribute is semantic identity.

It is not:

```text
an integer discriminant
an ABI code
a panic reason ID
```

Physical encoding is deferred.

---

# 11. Handler CFG representation

Schema v6 uses standard:

```text
cf.cond_br
cf.br
MLIR block arguments
func.return
```

for handler CFG.

No dedicated:

```text
sec.try
sec.try_handler
sec.try_merge
```

operation is required.

Sec-specific semantics remain in:

```text
Result projections
core-error variant tests
handler verifier metadata
```

---

# 12. Handler provenance metadata

Reserved optional block-associated implementation metadata:

```text
sec.try_handler_kind
sec.try_handler_index
sec.try_handler_variant
```

Canonical kinds:

```text
ok
err-variant
err-catch-all
merge
```

This metadata assists verification/debugging.

It is not source semantic authority.

Sema's resolved handler plan remains authoritative before MLIR construction.

---

# 13. Handler ordering

Compiler-generated variant-test CFG preserves source handler order.

The dialect does not permit variant-test reordering that changes first-match
behavior.

Later optimization may simplify only when semantic equivalence is proven.

---

# 14. Catch-all

A catch-all handler consumes the complete error value.

No variant test is needed at the catch-all entry.

No later live Err handler is valid.

---

# 15. Exhaustive variant-only handlers

For closed `ArithmeticError`, exhaustive specific variants are:

```text
Overflow
DivisionByZero
InvalidShift
```

The compiler may omit the final comparison and route the remaining case to the
last uncovered handler after the preceding comparisons.

This is valid only because Sema already proved exhaustive closed coverage and
the handler verifier confirms the metadata.

---

# 16. Fallback merge

A value-producing local try has a standard merge block.

Every continuing value-producing path supplies exactly:

```text
one value of success type T
```

The merge block has one block argument of T.

Returning/terminating handlers have no merge edge.

---

# 17. Explicit Ok handler

When present, an explicit Ok handler replaces the implicit Ok continuation.

Only one live explicit Ok handler is valid.

`Ok(value)` receives T.

`Ok(_)` receives no binding.

---

# 18. Implicit Ok handler

When no explicit Ok handler exists, compiler-generated handler CFG contains the
normal success continuation.

For value context it supplies the unwrapped T to the merge.

---

# 19. Result guard verifier

`--sec-verify-result-guards` validates canonical Result discrimination.

It verifies at least:

```text
same Result SSA for test and projection
true branch is Err
false branch is Ok
Err projection occurs only on Err path
Ok projection occurs only on Ok path
Result dominates projections
```

---

# 20. Try handler verifier

`--sec-verify-try-handlers` validates compiler-generated local handler CFG for
the schema-v6 supported domain.

It checks:

```text
source-order metadata
catch-all finality
specific variant uniqueness
ArithmeticError exhaustive coverage when no catch-all
fallback merge type
return/terminate path separation
explicit-versus-implicit Ok exclusivity
```

---

# 21. Error-type boundary

`sec.result.*` operations are generic in E.

`sec.core_error.is_variant` schema-v6 implementation is specifically defined for:

```text
core::ArithmeticError
```

General user enum/union variant testing requires canonical enum/union Sec MLIR
representation.

Do not encode user errors as `core_error`.

---

# 22. No physical layout

Schema v6 does not define:

```text
Result tag
Result payload union
core error integer value
enum integer value
LLVM representation
ABI representation
```

---

# 23. No runtime implication

Schema-v6 Result/handler operations imply only static SSA semantics.

No exception runtime, allocation, or unwinder is required.

---

# 24. Schema-v6 completion

Schema v6 is complete when:

```text
Result guard operations parse/print/verify
core ArithmeticError variant test verifies
canonical Result guard passes dedicated verifier
invalid projection paths fail dedicated verifier
handler CFG passes dedicated verifier
local ArithmeticError handlers round-trip
schema-v5 regressions remain valid
```
