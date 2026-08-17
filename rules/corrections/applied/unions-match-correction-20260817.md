# Union correction — shallow match destructuring ownership

- **Status:** Applied normative correction
- **Applied:** 2026-08-17
- **Created:** 2026-08-17
- **Last updated:** 2026-08-17
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `56be75d`
- **Target rulebook:** `rules/declarations/unions.md`

---

## Correction

The union rulebook already establishes shallow named-field match destructuring and prohibits hidden partial moves.

The by-value field rule must be made explicit for Sec 0.1:

> A plain by-value field binding in shallow struct-like union destructuring is valid only when the field type is implicitly copyable.

A move-only field must not be moved out through shallow field destructuring.

Invalid when `payload` is move-only:

```sec
match packet {
    Packet { payload } => Consume(payload)
}
```

Use borrowing when field access is required without ownership transfer:

```sec
match packet {
    Packet { payload: ref value } => Inspect(value)
}
```

or, with mutable authority:

```sec
match packet {
    Packet { payload: ref mut value } => Modify(value)
}
```

When ownership of move-only content is required, bind the complete variant payload by value and apply the ordinary whole-payload ownership rules.

The compiler must not:

- silently clone a move-only field;
- silently borrow a plain field binding;
- create a hidden partially moved reusable union subject through shallow destructuring.

This correction narrows the existing wording that leaves by-value field binding to unspecified normal move behavior.

## Cross-reference

Canonical match ownership semantics are defined by:

```text
rules/control-flow/flowcontrol_match.md
rules/memory/ownership.md
```
