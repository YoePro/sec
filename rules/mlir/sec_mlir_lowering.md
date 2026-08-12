# Sec MLIR Lowering

## Status

Normative lowering specification.

Current lowering specification version: `8`

Version 8 retains the high-level enum/union representation boundary and adds
resolved variant-oriented match lowering as explicit verified CFG.

It does not perform physical tagged-union lowering.

---

# 1. Enum high-level boundary

`!sec.enum` remains a high-level semantic type after Package 11.

Even though the native representation is the underlying integer type, enum
identity remains useful for:

```text
safe explicit conversions
handler dispatch
future match lowering
debugging
verification
```

Do not erase enum identity in lowering v7.

---

# 2. Enum target resolution

The scalar-layout pass may resolve target-sized enum underlying types.

Example:

```text
!sec.enum<..., !sec.int, ...>
    -> !sec.enum<..., si32, ...>
```

on a 32-bit plan.

Likewise:

```text
-> si64
```

on a 64-bit plan.

The wrapper/case table remains unchanged.

---

# 3. No enum signless normalization yet

The checked-integer Arith pass must not recurse into `!sec.enum`.

A dedicated future enum-representation pass will convert the enum to the lower
integer representation when semantic variant identity is no longer required.

---

# 4. Enum constants remain high-level

`sec.enum.constant` remains.

Do not prematurely convert it to `arith.constant` in P11.

This preserves declared-case provenance and enum identity.

---

# 5. Enum conversions remain high-level

Keep:

```text
sec.enum.from_integer
sec.enum.to_integer
```

until integer conversion semantics and enum representation erasure are lowered
together.

No hidden truncation.

---

# 6. Enum comparison remains high-level

Keep:

```text
sec.enum.cmp
```

through P11.

Future enum representation lowering may convert it to integer equality once both
operands use the same erased representation.

---

# 7. Union high-level boundary

`!sec.union` remains a high-level semantic tagged union.

Keep:

```text
sec.union.construct
sec.union.is_variant
sec.union.unwrap_payload
sec.union.unwrap_field
```

No operation becomes a raw tag/load/store in P11.

---

# 8. Optional layout metadata

When Semantic IR carries a valid resolved layout reference, preserve:

```text
sec.layout_ref
```

where the existing metadata convention applies.

When no resolved layout exists, do not invent one.

---

# 9. Union construction is semantic

`sec.union.construct` means:

```text
create a semantic union value with active variant V and initialized payload
```

It does not mean:

```text
allocate stack storage
write integer tag
memcpy payload bytes
```

Those are later representation operations.

---

# 10. Union variant test is semantic

`sec.union.is_variant` tests the semantic active variant.

It does not expose or compare the physical tag integer.

---

# 11. Union projection safety

`sec.union.unwrap_payload` and `sec.union.unwrap_field` require a verified
matching path.

Run:

```text
--sec-verify-union-guards
```

before transformations that may discard high-level control information.

---

# 12. Payload ownership gate

Lowering v7 compiler generation permits:

```text
copy-trivial
```

payload action only.

If a union payload requires:

```text
move
semantic copy
non-copyable handling
conditional copy
non-trivial destruction
```

the P11 pipeline rejects that value operation as unsupported.

Do not lower it as an ordinary SSA copy.

---

# 13. ArithmeticError migration

Schema-v7 new output uses:

```text
!sec.enum
```

for ArithmeticError.

The local try handler path uses:

```text
enum constant
enum equality
```

for specific variants.

Do not emit new `!sec.core_error`/`sec.core_error.is_variant` operations for
ArithmeticError.

---

# 14. Legacy schema compatibility

Schema-v6 regression tests may retain:

```text
!sec.core_error
sec.core_error.is_variant
```

No automatic migration pass is required in Package 11.

The schema-v7 compiler emitter simply uses the new representation.

---

# 15. Result boundary

`!sec.result<T,E>` remains high-level.

It may contain:

```text
E = !sec.enum<...>
```

P10 Result guard/unwrapping remains valid.

Do not lower Result to `!sec.union` in P11.

---

# 16. Option boundary

Concrete Option may use ordinary union representation.

If older code has a dedicated Option representation, migration may be deferred
provided new P11 union semantics remain canonical.

---

# 17. Generic union boundary

Runtime union ops require concrete substituted payload types.

Do not lower unresolved generic union template values.

---

# 18. Struct-like source evaluation order

Lowering from source/Semantic IR to `sec.union.construct` preserves source
expression evaluation order.

Canonical operand ordering inside the final construct op may follow declaration
field order only after all source expressions have been evaluated to SSA values.

---

# 19. No general match lowering

P11 lowering does not create complete match CFG.

The operations introduced by P11 are the primitives consumed by Package 12.

---

# 20. No physical union lowering

Do not lower to:

```text
arith tag constants
memref byte buffer
LLVM struct
LLVM insertvalue/extractvalue
manual alignment padding
```

in lowering v7.

---

# 21. No union equality lowering

Current frontend may classify unions as equality-comparable when every payload
is comparable.

The baseline rulebook explicitly notes union equality lowering is still
pending.

P11 leaves it pending.

Do not synthesize recursive equality here.

---

# 22. No runtime dependency

Enum and union semantic operations are static compiler IR constructs.

No runtime variant table, reflection table, allocation service or type registry
is introduced.

---

# 23. Verification pipeline

Schema-v7 compiler-generated output should pass:

```text
normal MLIR verification
sec-verify-checked-integer-guards when applicable
sec-verify-result-guards when applicable
sec-verify-try-handlers when applicable
sec-verify-union-guards when union projections exist
```

---

# 24. Completion

Lowering specification version 7 is implemented when:

```text
enum identity survives scalar/integer lowering stages
default enum underlying int resolves by target plan
bit-backed width remains exact
ArithmeticError uses ordinary enum representation
Result enum errors remain high-level
union active variant remains semantic
union projections remain guarded
trivial payload action is explicit
non-trivial ownership is rejected rather than hidden
no physical union layout is selected
no LLVM dialect is generated
```

---

# 25. Match CFG lowering

Lowering consumes verified Semantic IR and its Sema-resolved match provenance.
It evaluates one subject SSA value, emits source-order pattern blocks, evaluates
guards only after pattern success, and uses one typed merge argument for match
expressions or a continuation block for match statements.

Enum patterns use `sec.enum.constant` plus `sec.enum.cmp eq`; union and Option
patterns use `sec.union.is_variant` and guarded copy-trivial projection; Result
patterns use `sec.result.is_err` and guarded unwrap operations. An impossible
exhaustive residual emits `sec.unreachable`. The output carries deterministic
function-local match provenance and runs `sec-verify-match-cfg` in addition to
applicable Result/union verifiers. No physical representation or switch-table
optimization is selected in this pass.
