# Sec MLIR Dialect

## Status

Normative detailed representation specification for the Sec MLIR dialect.

Current dialect schema version: `4`

This document is subordinate to:

```text
rules/foundations/operators.md
rules/errors/runtime_checks.md
rules/types/types.md
rules/memory/layout.md
rules/compiler/semantic_ir.md
rules/mlir/sec_mlir.md
```

It defines representation and verifier obligations.

It does not redefine Sec source-language operator semantics.

---

# 1. Version history

## Schema version 1

Dialect foundation.

## Schema version 2

First Semantic IR bridge, semantic scalar/storage/call operations.

## Schema version 3

Wide scalar completion, decimal128, target/DLTI metadata.

## Schema version 4

Adds explicit builtin-integer semantic operations and ordinary arithmetic
failure termination:

```text
sec.int.unary_plus
sec.int.neg_checked
sec.int.binary_checked
sec.int.bit_not
sec.int.bitwise
sec.int.shift_checked
sec.int.cmp
sec.fail.arithmetic
```

---

# 2. Wide builtin invariant

The following are active Sec builtins:

```text
int128
int256
uint128
uint256
decimal128
```

Schema version 4 integer operations support the active 128-bit and 256-bit
integer families exactly like smaller builtin integers.

No dialect documentation may classify them as future or planned types.

---

# 3. Accepted builtin integer semantic types

Schema-v4 integer operations accept:

```text
!sec.int
!sec.uint

si8
si16
si32
si64
si128
si256

ui8
ui16
ui32
ui64
ui128
ui256
```

Excluded:

```text
i1
!sec.char
!sec.rune
!sec.float
!sec.decimal
!sec.decimal128
f32
f64
!sec.named
!sec.distinct
```

Named/distinct integer operator representation is not defined by schema v4.

---

# 4. Signed integer category

Signed integer semantic types:

```text
!sec.int
si8
si16
si32
si64
si128
si256
```

---

# 5. Unsigned integer category

Unsigned integer semantic types:

```text
!sec.uint
ui8
ui16
ui32
ui64
ui128
ui256
```

`byte` appears as `ui8` at this layer.

---

# 6. Checked operation contract

A checked integer semantic operation produces:

```text
result
failed
```

where:

```text
failed: i1
```

The high-level operation is total.

The operation must define its `(result, failed)` pair for every input bit
pattern accepted by its operand types.

It must not model Sec failure as:

```text
undefined behavior
poison
implicit target trap
masked shift count
silent checked-arithmetic wrap
```

When `failed=true`, the result is semantically unavailable to source execution.

Compiler-generated MLIR must immediately guard the flag before evaluating later
source operations.

---

# 7. `sec.int.unary_plus`

Operands:

```text
value: T
```

Results:

```text
result: T
```

T must be a schema-v4 integer semantic type.

No failure.

No memory effect.

---

# 8. `sec.int.neg_checked`

Operand:

```text
value: signed integer T
```

Results:

```text
result: T
failed: i1
```

`failed=true` exactly for the minimum representable signed value.

No unsigned operand is valid.

---

# 9. `sec.int.binary_checked`

Required enum attribute:

```text
kind
```

Allowed cases:

```text
add
subtract
multiply
divide
remainder
```

Operands:

```text
left: T
right: T
```

Results:

```text
result: T
failed: i1
```

Verifier:

```text
T is schema-v4 integer semantic type
left/right types equal
result type equals T
failed result type is i1
```

Semantics:

```text
add:
    failed when mathematical sum not representable

subtract:
    failed when mathematical difference not representable

multiply:
    failed when mathematical product not representable

divide:
    truncation toward zero
    unsigned: failed on zero divisor
    signed: failed on zero divisor or minimum / -1

remainder:
    uses truncation-toward-zero quotient
    unsigned: failed on zero divisor
    signed: failed on zero divisor or minimum / -1
```

---

# 10. `sec.int.bit_not`

Operand/result:

```text
T -> T
```

T is schema-v4 integer semantic type.

Flips every value bit.

No failure.

---

# 11. `sec.int.bitwise`

Required enum:

```text
and
or
xor
```

Operands:

```text
T, T
```

Result:

```text
T
```

No failure.

---

# 12. `sec.int.shift_checked`

Required enum:

```text
left_unsigned
left_signed
right_unsigned
right_signed
```

Operands:

```text
value: T
count: C
```

Results:

```text
result: T
failed: i1
```

T and C must be schema-v4 integer semantic types.

Count type may differ from value type.

Verifier:

```text
left_unsigned/right_unsigned require unsigned T
left_signed/right_signed require signed T
result type equals T
failed result is i1
```

Semantics:

```text
all modes:
    failed on count < 0 when count type is signed
    failed on count >= value bit width

left_unsigned:
    valid count performs fixed-width left shift
    high bits are discarded
    no arithmetic overflow failure

left_signed:
    valid count still fails if mathematical shifted result is not representable

right_unsigned:
    logical right shift

right_signed:
    arithmetic right shift
```

No target-specific count masking is permitted.

---

# 13. `sec.int.cmp`

Required enum:

```text
eq
ne
lt
le
gt
ge
```

Operands:

```text
left: T
right: T
```

Result:

```text
i1
```

Verifier:

```text
T is schema-v4 integer semantic type
operand types equal
result is i1
```

Semantics:

```text
eq/ne:
    integer equality

lt/le/gt/ge:
    signed ordering for signed T
    unsigned ordering for unsigned T
```

---

# 14. Integer operation effects

All `sec.int.*` operations defined in schema v4:

```text
have no memory effects
do not themselves perform the arithmetic failure action
```

Checked operations calculate a semantic failure flag.

They may be treated as total computational operations.

The dialect implementation must not claim target poison/UB semantics.

---

# 15. Arithmetic failure categories

Define enum:

```text
overflow
division
remainder
shift
```

---

# 16. `sec.fail.arithmetic`

No operands.

No results.

No successors.

Terminator.

Required attributes:

```text
category: Sec arithmetic failure category
sec.operator: StringAttr
```

Meaning:

```text
ordinary deterministic non-returning arithmetic failure
```

It does not define the lower implementation mechanism.

It must not be:

```text
speculatable
canonicalized away
treated as a pure no-op
```

---

# 17. Canonical checked guard

Compiler-generated checked integer operation:

```mlir
%result, %failed = "sec.int.binary_checked"(...)
    {kind = ...} : (...) -> (T, i1)

cf.cond_br %failed, ^failure, ^success
```

Canonical meaning:

```text
true -> arithmetic failure
false -> success continuation
```

Dedicated failure block:

```mlir
^failure:
    "sec.fail.arithmetic"() {
        category = ...,
        sec.operator = "..."
    } : () -> ()
```

---

# 18. Failure-category mapping

Canonical mapping:

```text
neg_checked
    overflow

binary add
    overflow

binary subtract
    overflow

binary multiply
    overflow

binary divide
    division

binary remainder
    remainder

any shift_checked
    shift
```

---

# 19. Source operator spelling metadata

`sec.operator` on `sec.fail.arithmetic` uses the resolved source operator
spelling where available.

Examples:

```text
negation: "-"
add: "+"
subtract: "-"
multiply: "*"
divide: "/"
remainder: "%"
left shift: "<<"
right shift: ">>"
```

This is provenance/diagnostic metadata.

It does not determine operation semantics.

---

# 20. Checked result availability

The result of a checked operation is semantically usable only on the
success continuation.

The dialect operation verifier checks local type structure.

Cross-block result availability and guard shape are validated by the dedicated
checked-integer guard verifier pass.

---

# 21. Scalar resolution compatibility

Schema-v4 operations may initially contain:

```text
!sec.int
!sec.uint
```

The scalar-layout pass may convert those operand/result types to:

```text
si32/si64
ui32/ui64
```

without changing operation kind.

The checked failure result remains `i1`.

---

# 22. No signless normalization

Schema-v4 semantic integer operations retain signed/unsigned type identity.

They are not standard Arith operations.

No schema-v4 rule converts:

```text
siN
uiN
```

to signless `iN`.

---

# 23. No named integer operators yet

Schema version 4 does not define integer operators whose result/operands are:

```text
!sec.named
!sec.distinct
```

The source language may support such operators through higher-authority rules.

Their Sec MLIR representation requires a later package preserving nominal/unit
semantics.

Do not unwrap nominal identity to reuse builtin-integer schema-v4 operations.

---

# 24. No float/decimal operator representation in schema v4

Existing scalar types remain valid:

```text
!sec.float
!sec.decimal
!sec.decimal128
```

but schema v4 does not add their arithmetic operator operations.

---

# 25. Verification boundary

Operation verifiers enforce:

```text
allowed integer type category
signedness requirements
operand type equality
result type equality
failure i1 type
enum attribute validity
failure terminator attribute validity
```

They do not perform:

```text
source operator resolution
constant overflow diagnostics
range proof
try/error propagation
runtime implementation selection
```

---

# 26. Schema-v4 completion

Schema v4 is complete when:

```text
all integer operation kinds parse/print/verify
128/256-bit integer operands verify
invalid signedness combinations reject
checked results use T + i1
arithmetic failure terminator verifies
schema-v1/v2/v3 regression tests remain green
compiler-generated checked operator CFG passes the dedicated guard verifier
```
