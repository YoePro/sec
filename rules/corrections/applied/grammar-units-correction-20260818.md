# Grammar correction — unit declarations, metadata, and structural unit expressions

- **Status:** Applied 2026-08-18
- **Created:** 2026-08-18
- **Last updated:** 2026-08-18
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `20b3606`
- **Target rulebook:** `rules/foundations/grammar.md`

---

## Correction

The canonical grammar must be synchronized with `rules/types/units.md` revision
2.0.

### Unit declaration

The unit declaration remains conceptually:

```text
UnitDeclaration
    ::= "unit" Identifier [ UnitDefaultNumericType ] [ UnitCategory ]
```

`UnitDefaultNumericType` is not an arbitrary `TypeReference`.
It must resolve to a supported plain numeric scalar carrier and must not itself
be unit-bearing.

If omitted, the default numeric representation is `decimal`.

If `UnitCategory` is omitted, the category is `other`.

### Unit categories

Replace the current three-category production with:

```text
UnitCategory
    ::= "physical"
      | "currency"
      | "information"
      | "ratio"
      | "other"
```

These are compiler-known category names in Sec 0.1.

### Unit names

A parsed unit identifier is still subject to semantic name validation.
A unit declaration must not use a compiler-known type/type-constructor/keyword
name in the relevant namespace.

In particular, `bit` and `byte` are not valid user unit declarations.

### Unit metadata

The grammar/parser metadata surface must recognize the canonical fields:

```text
LongName
Symbol
BaseUnit
Status
Dimension
Kind
Scale
System
Transform
Offset
Origin
LogBase
LogFactor
Reference
```

Canonical source spelling is PascalCase.
Existing case/underscore-tolerant parser compatibility may remain during Sec 0.1
migration, but it is not the preferred spelling.

### Unit annotation grammar

Replace the opaque canonical description:

```text
UnitAnnotation ::= "<" UnitTokens ">"
```

with a structural grammar that preserves the source expression while making the
allowed Sec 0.1 forms explicit:

```text
UnitAnnotation
    ::= "<" UnitExpression ">"

UnitExpression
    ::= UnitProduct

UnitProduct
    ::= UnitFactor { ( "*" | "/" ) UnitFactor }

UnitFactor
    ::= UnitAtom [ "^" SignedIntegerConstant ]

UnitAtom
    ::= QualifiedIdentifier
      | "1"
      | "(" UnitExpression ")"
```

Semantic normalization, not parsing, determines dimensional equivalence.

The exponent is a compile-time signed integer and is validated by Sema.

### Unit-only type

Keep:

```text
UnitOnlyType
    ::= UnitAnnotation
```

and make its general Sec 0.1 meaning explicit:

- `<NamedUnit>` uses the named unit's default numeric representation;
- if the unit has no explicit default representation, the carrier is `decimal`;
- a compound structural unit expression such as `<m/s>` defaults to `decimal`;
- an explicit carrier remains available as `float64<m/s>`, `decimal<m/s>`, etc.

### Named versus structural unit reference

The grammar must not imply that:

```text
<mps>
```

and:

```text
<m/s>
```

are syntactic aliases.

The first is a single named unit reference when `mps` resolves as a unit.
The second is a structural unit expression.

### Constructor-shaped conversions

The parser must preserve ordinary call/type-conversion structure so Sema can
resolve forms such as:

```sec
decimal<m>(value)
EUR(sek, factor)
```

`EUR(sek, factor)` does not require a special parser-only currency node.
Sema resolves it as the compiler-known explicit factor-provided unit conversion
when the target and arguments satisfy the unit conversion rules.

### Dimension metadata syntax

Canonical metadata dimension entries use:

```text
Axis "^" SignedIntegerConstant
```

for example:

```text
[length^1, time^-1]
```

The old `length * 1` spelling must not appear as canonical grammar.

## Cross-reference

Canonical semantics are defined by:

```text
rules/types/units.md
```
