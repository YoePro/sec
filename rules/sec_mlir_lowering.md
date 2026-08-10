# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `6`

This version adds local try handler CFG and high-level Result discrimination.

Physical Result/error lowering remains deferred.

---

# 1. Local handler lowering principle

A resolved local `try` handler list lowers to explicit CFG.

The lowering consumes Sema-resolved facts.

It does not resolve patterns or handler coverage.

---

# 2. Two fallible inputs

Package 10 supports two fallible input forms.

## Result value

```text
Result[T, ArithmeticError]
```

Use:

```text
result.is_err
conditional branch
unwrap_err / unwrap_ok
```

## Checked arithmetic

Use existing:

```text
result
failed
reason
```

On failure:

```text
reason -> ArithmeticError
```

Then enter the same Err handler dispatch model.

---

# 3. No temporary Result required for arithmetic

Do not construct:

```text
ResultOk
ResultErr
```

merely to implement a local arithmetic handler.

Arithmetic already has direct success/error control data.

Construct Result only when the handler source explicitly returns/propagates one.

---

# 4. Implicit success path

Without explicit Ok handler:

```text
success T -> try merge
```

for value context.

No additional semantic operation.

---

# 5. Explicit Ok path

With explicit:

```text
Ok(value)
Ok(_)
```

route success to that handler.

A value-producing Ok handler supplies T to merge.

A returning/terminating Ok handler has no merge edge.

---

# 6. Ordered error dispatch

Specific error variants are tested in source order.

For Package 10:

```text
ArithmeticError.Overflow
ArithmeticError.DivisionByZero
ArithmeticError.InvalidShift
```

using:

```text
sec.core_error.is_variant
cf.cond_br
```

Catch-all receives the exact error directly.

---

# 7. Exhaustive specific handlers

When no catch-all exists, Sema must have proven the closed variant set exhaustive.

Lowering may route the final unmatched case directly to the only remaining
variant handler.

Do not add automatic propagation.

---

# 8. Fallback merge

A fallback expression produces T.

Branch:

```text
cf.br merge(fallback)
```

Implicit/explicit success path also branches with T.

Merge block argument:

```text
T
```

is the local try expression value.

---

# 9. Return handler

A return handler uses ordinary Semantic IR/MLIR return construction.

Examples:

```text
return plain value
return Result Ok
return Result Err
```

No edge to merge.

---

# 10. Error propagation handler

For:

```text
return Err(error)
```

construct the exact enclosing Result Err using existing Package 9 operation.

No implicit conversion of E.

---

# 11. Result naked propagation

For:

```text
try Result[T, ArithmeticError]
```

without local handlers:

```text
Err:
    unwrap error
    construct enclosing ResultErr
    return

Ok:
    unwrap success
    continue
```

This generalizes Package 9 arithmetic-only naked propagation.

---

# 12. Result guard remains high-level

Do not lower:

```text
sec.result.is_err
sec.result.unwrap_ok
sec.result.unwrap_err
```

to memory/tag operations in Package 10.

The physical representation is unresolved.

---

# 13. Core error variant test remains high-level

Do not lower:

```text
sec.core_error.is_variant
```

to integer comparison in Package 10.

The physical ArithmeticError encoding is unresolved.

---

# 14. Verification before further lowering

Compiler output must pass:

```text
normal MLIR verification
sec-verify-result-guards
sec-verify-try-handlers
```

Checked arithmetic stages additionally use the checked-integer guard verifier
where applicable.

---

# 15. Package 8 compatibility

Integer arithmetic may already be lowered to standard Arith.

The local handler flow depends only on:

```text
failed
reason
ArithmeticError conversion
```

and therefore remains valid after Package 8.

---

# 16. Ownership boundary

Result projections do not define copy/move ownership semantics for non-trivial
payloads.

Package 10 end-to-end compiler support is limited to payloads accepted by the
current Semantic IR ownership subset.

Do not silently copy resource-owning Result payloads.

---

# 17. User error enum/union boundary

Do not create special lowering representations for unsupported user error types.

The generic Result operations may accept future E types once those types exist
canonically.

Variant tests for user enums/unions are deferred to the enum/union
representation package.

---

# 18. Optimization independence

Correctness does not depend on:

```text
canonicalize
CSE
SCCP
DCE
```

Handler ordering must remain semantically visible before any proof-based
simplification.

---

# 19. Completion

Lowering specification version 6 is implemented when:

```text
ordinary Result branching works without physical layout
naked Result propagation works
local ArithmeticError handlers work
handler order is preserved
fallback merges work
return/terminate paths do not merge
implicit Ok semantics work
explicit Ok semantics work
no unmatched error auto-propagation is introduced
Result/error physical representation remains deferred
```
