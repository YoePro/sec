# Diagnostics correction — unit semantic errors and currency warnings

- **Status:** Applied 2026-08-18
- **Created:** 2026-08-18
- **Last updated:** 2026-08-18
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `20b3606`
- **Target rulebook:** `rules/tooling/diagnostics.txt`

---

## Correction

Unit diagnostics must distinguish hard semantic errors from valid-but-suspicious
unit expressions according to `rules/types/units.md` revision 2.0.

### Error-class unit diagnostics

Compiler errors include at least:

- unknown unit;
- malformed/invalid structural unit expression;
- incompatible dimension;
- incompatible known Kind where implicit compatibility is required;
- invalid point/difference arithmetic;
- incompatible Origin;
- invalid affine conversion/arithmetic;
- invalid logarithmic conversion/arithmetic;
- invalid transform metadata;
- invalid Dimension metadata;
- invalid unit default numeric representation;
- unit identifier collision with compiler-known names;
- impossible or lossy implicit unit conversion;
- implicit conversion that would require a runtime/configured relationship;
- invalid factor unit in a factor-provided conversion.

Diagnostics should identify the semantic reason rather than report only a raw
numeric type mismatch.

Example shape:

```text
cannot implicitly convert frequency Hz to rotational_frequency rpm
help: use an explicit unit conversion
```

### Currency-derived warnings

Anonymous structural expressions that derive from currency through multiplication
or division are valid Sec code but should produce warning-class diagnostics by
default.

Examples:

```text
<EUR/s>
<SEK/EUR>
<EUR/SEK>
```

The diagnostic must not call these expressions invalid merely because currency
is present.

A currency-ratio warning should explain that the compiler does not know or invent
a current exchange rate.

Example shape:

```text
warning: derived currency unit SEK/EUR has no compiler-known runtime exchange rate
help: use an explicit factor or conversion operation when converting currencies
```

`EUR/s` may use a lower-severity advisory wording because it commonly represents
a legitimate rate, but it remains governed by project warning policy.

### Precision diagnostics

When a fixed unit relationship exists but is not exactly representable in the
selected target numeric carrier, diagnostics must explain the representation
problem separately from unit compatibility.

Suggested help may include using `decimal`, another wider/exact carrier, or an
explicit rounding/conversion operation.

### Warning policy

Warnings remain warnings unless project diagnostic configuration promotes them.
The language definition must not silently convert currency advisories into hard
errors.

## Cross-reference

Canonical unit semantics are defined by:

```text
rules/types/units.md
```
