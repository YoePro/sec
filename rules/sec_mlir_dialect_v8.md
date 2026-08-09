# Sec MLIR Dialect

## Status

Normative detailed representation specification.

Current Sec MLIR dialect schema version: `8`

Schema version 8 adds a general synthesized unreachable terminator used by
verified match CFG and reserves match provenance attribute names.

It does not add a monolithic match operation.

---

# 1. Version history

```text
v1  dialect foundation
v2  Semantic IR bridge
v3  scalar/target coverage
v4  checked integer operations
v5  typed arithmetic failure and Result construction
v6  Result branching/local try handlers
v7  enum and union semantic values
v8  verified match CFG support and synthesized unreachable
```

Compiler-generated v8 modules carry:

```mlir
sec.dialect_version = 8 : i32
```

Schema versions 1 through 7 remain regression inputs.

---

# 2. No `sec.match` operation

Schema v8 intentionally does not define:

```text
sec.match
sec.match_arm
sec.match_merge
```

Resolved match semantics use standard explicit MLIR CFG.

---

# 3. `sec.unreachable`

New operation:

```text
sec.unreachable
```

No operands.

No results.

No successors.

Terminator.

Required attributes:

```text
sec.synthesized: BoolAttr = true
reason: StringAttr
```

Optional match provenance:

```text
sec.match_id
```

---

# 4. Unreachable meaning

`sec.unreachable` states that higher-level compiler semantics proved the path
impossible.

It is not:

```text
panic
trap policy
Result Err
language exception
permission for invalid source behavior
```

Compiler-generated match residual uses:

```text
reason = "exhaustive-match-fallthrough"
```

---

# 5. Verification

Operation verifier requires:

```text
sec.synthesized == true
reason non-empty
no successors
terminator placement
```

A dedicated match verifier validates the proof context for
`exhaustive-match-fallthrough`.

---

# 6. Match provenance attributes

Reserved compiler-generated attributes:

```text
sec.match_id
sec.match_arm_index
sec.match_stage
sec.match_pattern_kind
```

They may be attached as discardable dialect-scoped attributes to standard CFG
operations created for match lowering.

---

# 7. `sec.match_id`

Type:

```text
positive i32-compatible integer
```

Function-local match identity.

Not a source identifier.

---

# 8. `sec.match_arm_index`

Type:

```text
non-negative i32-compatible integer
```

Source-order arm index.

---

# 9. `sec.match_stage`

Allowed values:

```text
pattern
guard
body-exit
merge
residual
```

---

# 10. `sec.match_pattern_kind`

Allowed values:

```text
enum-value
union-variant
result-ok
result-err
option-some
option-none
catch-all
```

---

# 11. Provenance is not authority

The attributes assist:

```text
verification
debugging
IR inspection
```

They do not determine pattern semantics.

The operation/type structure must already be semantically valid.

---

# 12. Enum match primitives

Schema v8 reuses schema-v7:

```text
sec.enum.constant
sec.enum.cmp
```

No new enum pattern operation.

---

# 13. Union match primitives

Schema v8 reuses:

```text
sec.union.is_variant
sec.union.unwrap_payload
sec.union.unwrap_field
```

P12 complete match support uses `unwrap_payload` only for copy-trivial
single-payload variants.

---

# 14. Result match primitives

Schema v8 reuses:

```text
sec.result.is_err
sec.result.unwrap_ok
sec.result.unwrap_err
```

The Result guard verifier is extended for match CFG shapes.

---

# 15. Option match primitives

Concrete Option uses ordinary union operations.

No special Option match op.

---

# 16. Standard CFG

Match lowering uses:

```text
cf.cond_br
cf.br
block arguments
func.return
```

No implicit fallthrough.

---

# 17. Match CFG verifier

Register:

```text
--sec-verify-match-cfg
```

It verifies compiler-generated match control flow and match provenance.

It does not re-run source pattern analysis.

---

# 18. Result verifier extension

`--sec-verify-result-guards` must accept both:

```text
canonical try guard
canonical source-order match guard
```

while preserving projection-path safety.

---

# 19. Union verifier extension

`--sec-verify-union-guards` must accept P12 matching-path projections and guards
dominated by the true variant edge.

---

# 20. Schema-v8 completion

Schema v8 is complete when:

```text
sec.unreachable parses/prints/verifies
match provenance attributes are recognized by verifier tooling
enum/union/Result/Option match CFG verifies
invalid projection/control-flow shapes fail
schema-v7 regressions remain valid
no monolithic match op exists
```
