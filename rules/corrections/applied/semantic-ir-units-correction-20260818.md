# Semantic IR correction — complete unit quantity semantics

- **Status:** Applied 2026-08-18
- **Created:** 2026-08-18
- **Last updated:** 2026-08-18
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `20b3606`
- **Target rulebook:** `rules/compiler/semantic_ir.txt`

---

## Correction

Semantic IR must preserve the complete resolved quantity semantics defined by
`rules/types/units.md` revision 2.0 until every unit-sensitive operation has been
made explicit.

### Required semantic data

Where relevant, a unit-bearing Semantic IR value/type must be able to preserve:

```text
numeric carrier
named unit identity or structural unit identity
source named factors
normalized dimension
category provenance
known or unknown Kind
System
Transform
Scale
Offset
Origin
LogBase
LogFactor
Reference
point/difference role
canonical structural form
conversion plan
conversion exactness
```

The existing dimension/category/scale representation is insufficient on its own.

### Named versus structural identity

Semantic IR must distinguish:

```text
<mps>
```

from:

```text
<m/s>
```

when the first is a named unit and the second is a structural unit expression,
even if they normalize to compatible dimension and scale.

### Kind

Semantic IR must not derive Kind solely from normalized dimension.

Known Kind differences must remain visible through operator validation and
conversion lowering.

### Point and difference

Point values and difference values must remain distinguishable through Semantic
IR.

A `point - point` operation must explicitly produce difference semantics before
backend lowering.

Affine offsets must not survive incorrectly into difference arithmetic.

### Transform-aware conversion

Linear, affine, and logarithmic conversions must be represented distinctly
enough that verification can reject an invalid lowering.

A scale-only lowering must not be used for affine or logarithmic transforms.

### Factor-provided conversions

A source form such as:

```sec
EUR(sek, factor)
```

must lower as an explicit conversion operation that records:

- target unit;
- source unit;
- factor expression;
- normalized factor unit relationship;
- numeric carrier conversion if any;
- exactness/overflow requirements.

It must not become an implicit assignment conversion.

### Erasure boundary

Unit metadata may be erased only after all semantic unit behavior has become
explicit low-level behavior.

MLIR/LLVM must not be expected to infer source units from raw numeric types.

### Diagnostics and tooling facts

Semantic IR or the immediately preceding resolved semantic model must expose the
facts needed by diagnostics/LSP without requiring tooling to re-run unit algebra.

## Cross-reference

Canonical unit semantics are defined by:

```text
rules/types/units.md
```
