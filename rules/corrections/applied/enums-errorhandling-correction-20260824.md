# Enum correction — error enums

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/declarations/enums.md`

---

## Correction

Ordinary closed enums may be marked as Sec error types.

Canonical forms:

```sec
enum ParseError error {
    EmptyInput
    InvalidToken
}
```

and, with an explicit ordinary integer underlying type:

```sec
enum ProtocolError uint16 error {
    InvalidFrame = 1
    UnsupportedVersion = 2
}
```

The `error` marker:

- follows ordinary enum representation information;
- does not replace or become the underlying integer type;
- does not alter enum member initialization, `iota`, alias, default, equality,
  conversion, or closed-domain semantics;
- declares that values of the enum are assignable to compiler-known `error`.

Error enums remain nominal and closed.
Different concrete error enums do not become implicitly assignable to one
another.

When widened to `error`, the concrete enum type and member identity must remain
recoverable by the error-specific matching rules.

Hardware `bit` / `bit[N]` enums are not automatically error types merely because
they are enums. They require the explicit error marker if a future specialized
rule permits that combination.

## Cross-reference

```text
rules/errors/errorhandling.md
rules/types/types.md
```
