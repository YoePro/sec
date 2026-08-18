# Formatter correction — canonical unit source spelling

- **Status:** Applied 2026-08-18
- **Created:** 2026-08-18
- **Last updated:** 2026-08-18
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `20b3606`
- **Target rulebook:** `rules/tooling/formatter.md`

---

## Correction

Formatter rules for units must be synchronized with `rules/types/units.md`
revision 2.0.

### Unit declarations

Preserve the declared unit name, optional default numeric representation, and
category.

Do not add a redundant explicit `decimal` carrier merely because it is the unit
default.

Canonical examples:

```sec
unit m physical
unit Item uint other
```

### Metadata spelling

When the formatter normalizes compiler-known unit metadata spelling, use:

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

PascalCase is canonical.

### Dimension vectors

Canonical dimension metadata uses compact exponent notation:

```text
[length^1, time^-1]
```

Use normal list comma spacing.

Do not emit the obsolete `length * 1` form.

### Structural unit expressions

Canonical source spacing inside unit annotations is compact:

```text
<m/s>
<kg*m/s^2>
<EUR/SEK>
```

Do not add spaces around `*`, `/`, or `^` inside the angle brackets.

Parentheses required by source grouping are preserved unless the formatter can
remove them without changing parsing or meaning.

### No semantic rewriting

The formatter must not:

- replace `<m/s>` with `<mps>`;
- replace `<mps>` with `<m/s>`;
- reorder factors to match internal canonical order;
- replace one named unit with another compatible named unit;
- change numeric carrier to make a conversion exact;
- insert or remove conversion operations;
- rewrite currency-derived expressions merely to suppress warnings.

Semantic canonicalization is compiler-internal and is not source rewriting.

## Cross-reference

Canonical unit semantics are defined by:

```text
rules/types/units.md
```
