# Correction: spread into native variadic functions

- **Status:** Applied 2026-08-14
- **Created:** 2026-08-14
- **Last updated:** 2026-08-14
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** spread rulebook

Native typed variadic functions are explicit runtime-arity destinations.

```sec
fn Write(values: ...byte) void {
    ...
}
```

Therefore runtime-length spread may be used when calling them.

```sec
let bytes: byte[] := ...

Write(
    0x01,
    bytes...,
    0xff,
)
```

Rules:

- ordinary and spread arguments may mix;
- multiple spread sources may occur;
- arguments/spread sources evaluate left-to-right;
- each spread source evaluates exactly once;
- every expanded element must satisfy the variadic element type.

Spread remains non-consuming.

A spread from move-only collection elements is invalid when satisfying the variadic call would require moving individual elements out of the source collection.

Native variadic support does not add consuming or partial-move spread semantics.
