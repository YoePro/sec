# Union correction — payload-bearing error unions

- **Status:** Applied normative correction
- **Applied:** 2026-08-24
- **Created:** 2026-08-24
- **Last updated:** 2026-08-24
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/declarations/unions.md`

---

## Correction

A tagged Sec union may be marked as an error type by placing `error` after the
union kind:

```sec
type IOError union error {
    OpenError {
        Path: string
        Code: int
    }

    ReadError {
        Path: string
        Offset: uint64
    }
}
```

This preserves the canonical declaration order:

```text
type Name union error
```

The marker:

- does not introduce general inheritance;
- does not make variants independent runtime types;
- does not change ordinary tagged-union layout semantics;
- makes the complete union type assignable to compiler-known `error`;
- permits variants to carry arbitrary legal Sec payload data;
- preserves ordinary ownership, move, borrow, destruction, default, and match
  semantics.

When widened to `error`, the concrete union identity, active variant, and payload
must remain semantically available for error-specific narrowing and destruction.

An error union remains closed even though the root type `error` is open over all
concrete Sec error types.

## Cross-reference

```text
rules/errors/errorhandling.md
rules/types/types.md
rules/control-flow/flowcontrol_match.md
```
