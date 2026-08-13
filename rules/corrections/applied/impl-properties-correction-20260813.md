# Correction: properties in impl v2

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1
- **Language version:** Sec 0.1
- **Target:** `rules/declarations/impl.md`

## Property placement

Properties are behavioral members and may appear in:

```sec
impl Type {
    ...
}
```

and:

```sec
impl extends Type {
    ...
}
```

All such fragments in the defining module form one combined implementation member surface.

Property names and conflicts are therefore checked across the primary implementation and every `impl extends` fragment.

## Representation boundary

A property never adds stored instance representation to the implemented type.

The type declaration owns stored data; the property owns access behavior.

## Receiver

Instance property bodies use implicit `self`, matching ordinary impl methods.

Receiver access requirements are inferred by Sema from the getter/setter bodies.
