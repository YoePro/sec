# Operators correction — unit Kind, structural algebra, and transforms

- **Status:** Applied 2026-08-18
- **Created:** 2026-08-18
- **Last updated:** 2026-08-18
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `20b3606`
- **Target rulebook:** `rules/foundations/operators.md`

---

## Correction

Unit-bearing operator semantics must use the full quantity model from
`rules/types/units.md` revision 2.0.

Dimension equality alone is no longer sufficient to establish semantic
compatibility.

### Addition and subtraction

For unit-bearing operands, `+` and `-` must consider:

- ordinary numeric operator validity;
- dimension;
- named unit identity or a permitted conversion relationship;
- known `Kind`;
- transform mode;
- point/difference role and Origin;
- exact representability in the numeric carrier.

Different known Kinds must not be implicitly combined merely because their
dimensions match.

Examples include energy versus torque and frequency versus rotational frequency.

### Point arithmetic

For compatible point `P` and difference `D` quantities:

```text
P - P -> D
P + D -> P
P - D -> P
D + D -> D
D - D -> D
```

`P + P` is invalid.

Multiplication and division of point quantities are invalid unless a specialized
rule explicitly defines the operation.

### Multiplication and division

Ordinary linear quantities combine structurally:

```text
<m> / <s>       -> <m/s>
<KiB> / <s>     -> <KiB/s>
<Packet> / <s>  -> <Packet/s>
```

Structural algebra canonicalizes dimensions internally but does not invent a
named result or semantic Kind when the result is ambiguous.

Target context may validate a structural result against a named target.

Multiplication or division by a semantically valid dimensionless ratio may
preserve the other operand's Kind.

### Logarithmic quantities

A `Transform: logarithmic` quantity must not silently use ordinary linear
arithmetic.

Operations require a specialized logarithmic rule or explicit conversion to a
compatible linear quantity.

### Remainder

`%` requires compatible unit-bearing operands and preserves the left operand's
unit semantics when the underlying numeric remainder operation is valid.

### Comparison

Comparison requires compatible dimension, Kind, transform, point/difference
role, Origin where applicable, and a valid conversion plan.

No comparison may invent a runtime/configured currency exchange rate.

### Exact conversion requirement

Any implicit fixed unit conversion used while resolving an operator must be
exact for the selected numeric carrier.

The compiler must never introduce hidden rounding, truncation, overflow, or
precision loss to make unit-bearing operands compatible.

### Currency structural algebra

Structural expressions containing currency, including:

```text
<EUR/s>
<SEK/EUR>
```

are valid operator results when the underlying operation is otherwise valid.
They may produce warning diagnostics but are not rejected merely for containing
currency.

## Cross-reference

Canonical unit semantics are defined by:

```text
rules/types/units.md
```
