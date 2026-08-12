# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `8`

Schema version 7 adds canonical high-level enum and tagged-union semantic value
representation.

It also migrates enum-shaped core errors, including ArithmeticError, to the
ordinary enum representation.

Schema version 8 additionally defines verified explicit match CFG and the
synthesized `sec.unreachable` terminator. Physical union layout remains deferred.

---

# 1. Version history

```text
v1  dialect foundation
v2  Semantic IR bridge
v3  scalar/target coverage
v4  checked integer operations
v5  typed arithmetic failure and high-level Result construction
v6  Result branching and local try handlers
v7  enum and tagged-union semantic value representation
v8  verified match CFG support and synthesized unreachable
```

Compiler-generated v8 modules carry:

```mlir
sec.dialect_version = 8 : i32
```

Schema versions 1 through 7 remain valid regression inputs.

---

# 2. Enum case attribute

Canonical conceptual syntax:

```text
#sec.enum_case<ordinal, "name", "value">
```

Fields:

```text
ordinal
name
arbitrary-precision base-10 numeric value
```

Rules:

```text
ordinal >= 0
name non-empty
value parses exactly as arbitrary-precision integer
```

Duplicate numeric values are allowed.

---

# 3. Enum representation kind

Canonical values:

```text
integer
bit-backed
```

---

# 4. `!sec.enum`

Canonical conceptual syntax:

```text
!sec.enum<
    "type-id",
    underlying-type,
    representation-kind,
    bit-width,
    [cases]
>
```

Example:

```text
!sec.enum<
    "main::Color",
    si32,
    integer,
    0,
    [
        #sec.enum_case<0, "Red", "0">,
        #sec.enum_case<1, "Green", "1">
    ]
>
```

Rules:

```text
type-id non-empty
underlying type integer-compatible
integer representation bit-width = 0
bit-backed width in 1..256
cases in contiguous ordinal order
case names unique
case numeric values representable
duplicate numeric values legal
```

---

# 5. Enum identity

Two `!sec.enum` values with different type IDs are different Sec types even when
all representation data is otherwise identical.

No implicit conversion exists.

---

# 6. Enum aliases

Cases may share numeric values.

Case ordinal/name remain declaration provenance.

Runtime equality is based on the semantic enum numeric value, not case ordinal.

---

# 7. `sec.enum.constant`

No operands.

Result:

```text
!sec.enum
```

Required attribute:

```text
case ordinal
```

Verifier confirms the ordinal exists in the result enum type.

---

# 8. `sec.enum.from_integer`

Operand:

```text
integer semantic value
```

Result:

```text
!sec.enum
```

It represents an explicit conversion already validated by Sema.

The value need not correspond to a declared case.

No truncation is implied.

---

# 9. `sec.enum.to_integer`

Operand:

```text
!sec.enum
```

Result:

```text
resolved integer conversion target
```

It is an explicit enum conversion.

It is not an implicit underlying-type extraction.

---

# 10. `sec.enum.cmp`

Operands:

```text
same !sec.enum type
```

Predicate:

```text
eq
ne
```

Result:

```text
i1
```

No ordering predicate.

---

# 11. Union field attribute

Canonical conceptual syntax:

```text
#sec.union_field<"name", type>
```

Field name is non-empty.

---

# 12. Union variant attribute

Canonical conceptual forms:

```text
#sec.union_variant<index, "Name", empty>

#sec.union_variant<index, "Name", single<type>>

#sec.union_variant<
    index,
    "Name",
    fields<[
        #sec.union_field<"x", type>,
        #sec.union_field<"y", type>
    ]>
>
```

Rules:

```text
index non-negative
name non-empty
empty has no payload
single has exactly one payload type
fields has one or more uniquely named fields
```

---

# 13. `!sec.union`

Canonical conceptual syntax:

```text
!sec.union<
    "type-id",
    [type-arguments],
    [variants]
>
```

Example:

```text
!sec.union<
    "main::Option",
    [i32],
    [
        #sec.union_variant<0, "Some", single<i32>>,
        #sec.union_variant<1, "None", empty>
    ]
>
```

Rules:

```text
type-id non-empty
runtime type arguments concrete
at least one variant
indices contiguous from zero
variant names unique
payload types valid
```

---

# 14. Union identity

Type identity includes:

```text
declaration identity
concrete type arguments
```

It is not the physical tag.

---

# 15. `sec.union.construct`

Operands:

```text
zero or more payload values
```

Result:

```text
one !sec.union
```

Required:

```text
variant index
payload actions
```

Struct-like variant additionally carries:

```text
field names
```

P11 compiler-generated payload action:

```text
copy-trivial
```

only.

---

# 16. Construction verifier

For the selected variant:

```text
empty:
    zero operands

single:
    one operand
    operand type equals payload type

fields:
    operand count equals field count
    field names exactly declaration order
    operand types equal field types
```

Payload action count matches operand count.

---

# 17. `sec.union.is_variant`

Operand:

```text
!sec.union
```

Required:

```text
variant index
```

Result:

```text
i1
```

Total semantic variant test.

---

# 18. `sec.union.unwrap_payload`

Operand:

```text
!sec.union
```

Required:

```text
single-payload variant index
payload action
```

Result:

```text
declared payload type
```

Control-flow validity is checked by the union guard verifier.

---

# 19. `sec.union.unwrap_field`

Operand:

```text
!sec.union
```

Required:

```text
struct-like variant index
field name
payload action
```

Result:

```text
declared field type
```

---

# 20. Union guard verifier

Register:

```bash
--sec-verify-union-guards
```

It verifies canonical guarded union projections.

Required:

```text
same union SSA for test and projection
projection is on true/matching path
variant identity matches
projection kind matches variant shape
field exists
result type matches
dominance is valid
```

---

# 21. Enum target-sized underlying

Package 6 scalar resolution may recurse one level into `!sec.enum` solely to
resolve:

```text
!sec.int
!sec.uint
!sec.char
!sec.rune
```

where such types are valid underlying representations according to source rules.

The enum wrapper and case metadata remain.

For ordinary default enum:

```text
!sec.int -> si32 or si64
```

by active CompilationPlan.

---

# 22. P8 signless boundary

Package 8 must not normalize the underlying signedness inside `!sec.enum`.

It must not recursively normalize `!sec.union`.

Dedicated representation lowering owns that transition later.

---

# 23. ArithmeticError

Schema-v7 compiler output represents ArithmeticError as:

```text
ordinary !sec.enum
```

with cases:

```text
Overflow
DivisionByZero
InvalidShift
```

The default underlying source type follows the core enum declaration.

If no explicit underlying is declared, it is `int`.

---

# 24. `!sec.core_error` compatibility

Schema-v6 `!sec.core_error<...>` remains parseable for regression fixtures.

Schema-v7 compiler-generated output must not use it for an enum-shaped core
error.

It is not the canonical enum representation.

---

# 25. Arithmetic error conversion

`sec.arithmetic_error.from_reason` in schema v7 returns the canonical
ArithmeticError `!sec.enum`.

No physical enum integer lowering is implied.

---

# 26. P10 enum handlers

Schema-v7 compiler-generated local enum-error dispatch uses:

```text
sec.enum.constant
sec.enum.cmp eq
```

rather than:

```text
sec.core_error.is_variant
```

Catch-all binds the complete enum value.

Every schema-v7 branch that reaches a local enum-error handler merge carries:

```text
sec.try_handler_exhaustive = true
```

This boolean records Sema's exhaustiveness proof. The MLIR verifier consumes
that proof instead of inferring a closed error domain from hard-coded enum case
names. Schema-v6 compatibility input retains its legacy ArithmeticError check.

---

# 27. Result with enum error

Valid:

```text
!sec.result<T, !sec.enum<...>>
```

Result guard/unwrapping operations from schema v6 remain unchanged.

---

# 28. Result remains specialized high-level form

`!sec.result<T,E>` is retained because high-level error-flow operations benefit
from the semantic abstraction.

Its future physical representation must obey the canonical Result tagged-union
semantics.

P11 does not lower Result to `!sec.union`.

---

# 29. Option

Concrete Option values may use `!sec.union` with:

```text
Some(T)
None
```

No dedicated new Option type is required.

---

# 30. Layout boundary

`!sec.union` does not encode:

```text
tag physical type
tag offset
payload offset
payload size
payload alignment
total size
total alignment
```

Those properties belong to resolved layout metadata and representation lowering.

---

# 31. No tag exposure

`UnionVariantAttr.index` is not a source numeric value.

No schema-v7 operation converts it to an integer.

---

# 32. Effects

Enum and union semantic construction/test/projection operations introduced here
have no memory effect by themselves.

This does not mean payload ownership effects are absent.

P11 compiler output permits only trivial-copy payload actions.

Future ownership-aware forms may require stronger effects/traits.

---

# 33. No speculatable invalid projection

`sec.union.unwrap_payload` and `sec.union.unwrap_field` must not be marked
speculatable across variant guards.

They rely on a proven active-variant path.

---

# 34. Schema-v7 completion

Schema v7 is complete when:

```text
enum attrs/type/ops parse, print and verify
aliases remain valid
wide/bit-backed enums verify
union attrs/type/ops parse, print and verify
generic concrete unions verify
guarded union projection verifier works
ArithmeticError uses ordinary enum output
P10 enum handlers use ordinary enum operations
schema-v6 regression fixtures remain valid
no physical union layout is selected
```

---

# 35. Schema-v8 match CFG

Schema v8 intentionally adds no monolithic match operation. Match lowering uses
`cf.cond_br`, `cf.br`, block arguments and ordinary function returns.

`sec.unreachable` is a no-operand, no-result, no-successor terminator requiring
`sec.synthesized = true` and a non-empty `reason`. Exhaustive match residuals
use reason `exhaustive-match-fallthrough`.

Compiler-generated CFG may carry positive `sec.match_id`, non-negative
`sec.match_arm_index`, `sec.match_stage` (`pattern`, `guard`, `body-exit`,
`merge`, `residual`) and `sec.match_pattern_kind` provenance. Provenance aids
verification but does not define semantics. `--sec-verify-match-cfg` validates
source order, false/true edges, guarded projections, merge types and exhaustive
residual termination.
