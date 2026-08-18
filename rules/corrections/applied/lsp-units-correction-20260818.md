# LSP correction — unit quantity facts and warnings

- **Status:** Applied 2026-08-18
- **Created:** 2026-08-18
- **Last updated:** 2026-08-18
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `20b3606`
- **Target rulebook:** `rules/tooling/lsp.md`

---

## Correction

The LSP must expose compiler-resolved unit facts from `rules/types/units.md`
revision 2.0.

The LSP must not implement an independent unit algebra or a separate conversion
policy.

### Hover information

When known and relevant, unit-bearing hover information should include:

```text
unit name
symbol
numeric carrier
default numeric representation
category
dimension
kind
system
transform
scale
offset
origin
logarithmic base/factor/reference
named or structural identity
canonical structural form
point or difference role
conversion exactness
```

Unknown Kind should be shown as unknown rather than guessed from dimension.

### Named versus structural display

Tooling must preserve the distinction between the source's named and structural
forms.

For example, hover may show that `<m/s>` is structurally compatible with a named
unit without rewriting the source to `<mps>`.

### Diagnostics

The LSP must surface compiler diagnostics for:

- incompatible dimensions;
- incompatible known Kinds;
- point/difference misuse;
- invalid Origin compatibility;
- invalid affine/logarithmic arithmetic;
- lossy implicit conversion;
- deprecated/obsolete units;
- derived currency warnings.

Currency expressions such as `<EUR/s>` and `<SEK/EUR>` remain valid even when a
warning is shown.

### Conversion assistance

Where the compiler has a valid explicit conversion path, code actions may offer:

- an explicit fixed unit conversion;
- a declared conversion function;
- a factor-provided conversion such as `EUR(sek, factor)` when a suitable factor
  is in scope;
- a numeric carrier change when that is required for exactness.

A code action must never invent a runtime exchange rate or rounding policy.

### Derivation information

Deep analysis/hover may show how a structural result was derived, for example:

```text
<m> / <s> -> <m/s>
```

and why no named Kind was inferred.

This information is compiler-derived and must remain consistent with Sema.

## Cross-reference

Canonical unit semantics are defined by:

```text
rules/types/units.md
```
