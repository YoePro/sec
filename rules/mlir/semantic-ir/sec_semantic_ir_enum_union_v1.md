# Semantic IR Amendment - Enum and Union Values

## Status

Normative amendment for:

```text
rules/compiler/semantic_ir.md
```

Package:

```text
SEC-MLIR-P11
```

Repository baseline:

```text
152c772
```

This amendment specifies the minimum canonical Semantic IR representation for
enum and union values.

---

# 1. Canonical enum type

Semantic IR enum types retain:

```text
stable TypeID
declaration SymbolID
qualified source identity
resolved underlying integer type
representation kind
bit-backed width when applicable
declaration-order cases
exact arbitrary-precision numeric value for every declared case
source locations
```

Enum identity must not be reduced to the underlying integer type.

---

# 2. Enum case identity

Each declared case has a stable declaration identity distinct from its numeric
value.

Multiple cases may share the same numeric value.

The case identity is provenance/declaration identity.

The numeric value is runtime enum semantics.

---

# 3. Enum SSA values

An enum SSA value always has the enum TypeID.

A value may originate from:

```text
declared enum constant
explicit integer-to-enum conversion
other validated semantic operation
```

An ordinary enum SSA value belongs to one declared numeric value class, although
aliases mean that it need not have one unique case-name identity. An open
bit-backed enum SSA value need not have a declared case identity because every
in-width pattern is valid.

---

# 4. Enum operations

Semantic IR must support explicit operations for:

```text
declared enum constant
integer-to-enum conversion
enum-to-integer conversion
same-enum equality
same-enum inequality
```

No implicit enum/integer conversion exists.

No enum ordering/arithmetic operation is introduced.

---

# 5. Enum numeric precision

Enum numeric constants use arbitrary precision.

Do not truncate to host widths.

This includes enum underlyings based on:

```text
int128
int256
uint128
uint256
bit[N] through 256
```

---

# 6. Default enum underlying type

Default underlying `int` remains target-sized semantically.

Semantic IR retains target-sized identity until CompilationPlan-aware lowering
resolves its physical width.

Do not hard-code 32-bit default enum representation.

---

# 7. Bit-backed enums

Semantic IR retains:

```text
bit-backed classification
exact width
unsigned behavior
```

The width is semantic and may be 1 through 256.

---

# 8. Canonical union type

Semantic IR union types retain:

```text
stable TypeID
declaration SymbolID
qualified source identity
concrete generic type arguments
closed declaration-order variant list
stable zero-based variant index
variant name
variant payload kind
single payload type or struct-like payload fields
copy classification
trivial-destruction classification
optional resolved LayoutRef
source locations
```

Union type identity must not be reduced to its tag representation.

---

# 9. Union variant kinds

Canonical kinds:

```text
empty
single
fields
```

Rules:

```text
empty: no payload
single: exactly one payload type
fields: one or more named payload fields
```

---

# 10. Union variant index

Variant index is:

```text
stable
zero-based
declaration-order
compiler metadata
```

It is not a source integer.

No Semantic IR conversion may expose it as a language integer value.

---

# 11. Concrete generic union values

Runtime union values use concrete type arguments.

Unresolved generic payload placeholders must not appear in runtime union
construction/projection operations.

---

# 12. Union construction

Semantic IR must represent union variant construction explicitly.

Construction records:

```text
union TypeID
variant index
payload operands
payload field identity/order when fields are present
payload transfer action
source location
```

The operation must not hide ownership transfer.

Package 11 compiler output permits only:

```text
copy-trivial
```

payload action.

---

# 13. Union active-variant test

Semantic IR must provide an explicit semantic active-variant test.

It returns bool.

It tests variant identity, not a physical tag integer.

---

# 14. Union payload projection

Semantic IR must provide explicit projection for:

```text
single payload
named struct-like payload field
```

Projection is valid only on a control-flow path that proves the matching active
variant.

The verifier treats unguarded or mismatched projection as invalid compiler IR.

---

# 15. Layout separation

Semantic union operations remain semantic even when a physical layout has been
resolved.

Semantic IR may reference resolved layout metadata.

It must not replace:

```text
construct variant
test variant
extract active payload
```

with raw byte/tag operations before the appropriate representation lowering
stage.

---

# 16. Copy/destruction boundary

Semantic IR type metadata retains complete copy/destruction classification.

Package 11 value construction/projection is limited to the current
copy-trivial payload subset.

A non-trivial union type remains a valid semantic type.

Its value operations are deferred until explicit ownership/copy/move/destruction
operations exist.

---

# 17. Result relationship

`Result[T,E]` may retain a specialized high-level Semantic IR abstraction for
error-flow optimization.

That abstraction must eventually materialize according to the canonical Result
union semantics.

It must not create a conflicting independent physical representation.

---

# 18. Option relationship

Concrete `Option[T]` may use the ordinary canonical union representation once
its generic argument is resolved.

---

# 19. ArithmeticError migration

`ArithmeticError` is an enum.

Once enum Semantic IR exists, it must use the same canonical enum
representation as other enum declarations.

A temporary core-error-only semantic type must not remain the canonical new
representation for enum-shaped core errors.

---

# 20. Verifier requirements

Semantic IR verifier must validate:

```text
enum case identity/order
enum value representability
enum alias legality
enum conversion operand/result types
same-enum compare types

union non-empty variant set
union index sequence
variant payload-shape invariants
concrete generic payloads
construction payload count/types
payload action validity
guarded projection
copy/destruction implementation boundary
```

---

# 21. Printer requirements

Deterministic printer output must preserve declaration order.

Do not use map iteration order for enum cases or union variants.

Arbitrary-precision enum values print canonical base-10.
