# Correction: interface property requirements

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/interfaces.md`

## State contracts use properties

Interfaces must not require stored instance fields.

When an interface requires readable or writable state, declare a property requirement.

```sec
interface Positioned {
    property Position: Point {
        get
        set position
    }
}
```

A fallible setter requirement is written:

```sec
interface Configurable {
    property Mode: Mode {
        get
        try set mode
    }
}
```

The setter parameter is explicit even though the interface accessor has no body.

Conformance checks:

- property type,
- getter presence,
- setter presence,
- infallible versus fallible setter,
- static versus instance category,
- receiver capability.

A setter requirement implies mutable receiver access for the setter operation.
