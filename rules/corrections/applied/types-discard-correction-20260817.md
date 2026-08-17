# Types correction — must-use and discardability

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-16
- **Last updated:** 2026-08-16
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/types/types.md`

---

## Correction

Add `must-use` and `discardability` to the semantic properties that may be attached to a resolved type or concrete type instance.

They are distinct:

```text
must-use
    an unnamed produced value may not disappear through implicit discard

discardable
    explicit terminal discard is legal
```

A type may therefore be must-use while remaining explicitly discardable.

Canonical examples:

```text
Result[int, IOError]
    must-use
    discardable

Thread[int] while unresolved
    must-use
    non-discardable

Result[Thread[int], SpawnError]
    must-use
    non-discardable while Ok may contain an unresolved Thread[int]

Option[int]
    not automatically must-use
    discardable
```

Discardability is recursive through concrete aggregate and variant payload structure.

The compiler must not derive discardability merely from copy classification or physical representation.

Compiler-known core types may carry these properties before general user-defined must-use syntax is introduced.

The source declaration mechanism for user-defined must-use types remains owned by the attributes/type-design rules and is not introduced by this correction.

## Cross-reference

The complete consuming behavior, implicit-call-result rule, lifecycle restrictions, and diagnostics are defined by:

```text
rules/control-flow/discard.md
```
