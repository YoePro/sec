# Correction: explicit move capture syntax

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `b3315f6` (semantic parent `45e5cd4`)
- **Target rulebook:** `rules/declarations/lambda-functions.md`

---

## Correction

Synchronize explicit lambda capture syntax and ownership transfer with
`rules/memory/ownership.md` revision 2.0.

### Capture forms

Canonical capture forms are:

```sec
capture(value)
capture(<-value)
capture(ref value)
capture(ref mut value)
```

Their meanings are:

```text
capture(value)       owned copy capture
capture(<-value)     owned consuming capture
capture(ref value)   shared borrow capture
capture(ref mut value) mutable borrow capture
```

The old spelling:

```sec
capture(-> value)
```

is no longer valid Sec syntax.

### No implicit move from plain capture

`capture(value)` must not infer a destructive move from a reusable source. If
`value` cannot be copied, plain copy capture is invalid and the programmer must
choose an explicit consuming or borrowing capture.

Canonical move capture:

```sec
let operation := capture(<-resource) fn() void {
    Use(resource)
}
```

The source becomes unavailable when the closure environment successfully takes
ownership. `capture(<-value)` may also intentionally consume a copyable source.

### Fresh values

The general ownership rule for fresh temporaries remains applicable: syntax
must not require an artificial move marker where no reusable source place is
being consumed.

## Cross-reference

Capture lifetime and environment representation remain owned by the lambda and
closure-analysis rulebooks; ownership transfer and availability are owned by
`rules/memory/ownership.md` revision 2.0.
