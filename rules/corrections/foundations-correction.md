# Correction: raw numeric `@address` spelling does not bypass region validation

- **Status:** Normative correction
- **Created:** 2026-08-26
- **Last updated:** 2026-08-26
- **Document revision:** 1
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `fc8d632`
- **Canonical path:** `rules/foundations/correction.md`
- **Corrects:** `rules/foundations/attributes.md`

## Correction

Raw numeric `@address` syntax remains valid Sec syntax.

For example:

```sec
@address(0x40021000)
let mut device: DeviceRegisters
```

may be appropriate for:

- custom boards;
- new target ports;
- application-specific hardware;
- targets whose canonical device knowledge is still expressed through numeric
  addresses.

However, the raw numeric spelling does not itself establish that the binding is
valid.

The resulting `@address` binding must still resolve against an applicable
canonical target-owned address-region/storage contract and satisfy its complete
extent, address-space, permission, alignment, physical-representation, and
access-width requirements.

Conceptually:

```text
raw numeric spelling
        ↓
target/device region resolution
        ↓
validated @address binding
```

not:

```text
raw numeric spelling
        ↓
implicitly trusted storage
```

Target/device symbolic constants and future knowledge mechanisms may make this
resolution more descriptive, but they are not mandatory syntax for custom
hardware.

This correction intentionally preserves low ceremony for new/custom targets:
programmers may still write raw numeric addresses while the target definition
provides the region facts required to validate them.

## Superseding rule

Any wording in `attributes.md` that can be read as making a raw numeric address
independently sufficient for a valid `@address` binding is superseded by this
correction.
