# Correction: interface conformance placement in impl v2

- **Status:** Applied 2026-08-13
- **Created:** 2026-08-13
- **Last updated:** 2026-08-13
- **Document revision:** 1
- **Sec language version:** 0.1

Synchronize `rules/declarations/impl.md` with the canonical interface model.

## Primary impl

The primary implementation may declare explicit interface conformance:

```sec
impl FileReader implements Reader, Closeable {
    ...
}
```

The `implements` list is part of the primary implementation entry point.

## Extended fragments

`impl extends Type` fragments contribute members to the same ordinary implementation but must not redeclare interface conformance.

```sec
impl extends FileReader {
    fn Helper() void {
        ...
    }
}
```

## Ownership

Ordinary impl and interface conformance remain owned by the implemented type's defining module.

Interface-specific conformance semantics are owned by `rules/declarations/interfaces.md`.
