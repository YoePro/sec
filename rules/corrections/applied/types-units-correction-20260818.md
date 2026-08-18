# Types correction — carrier-independent unit identity

- **Status:** Applied 2026-08-18
- **Created:** 2026-08-18
- **Last updated:** 2026-08-18
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `20b3606`
- **Target rulebook:** `rules/types/types.md`

---

## Correction

The central type model must describe units according to `rules/types/units.md`
revision 2.0.

### Unit identity is not a numeric representation

Any text that describes a unit itself as a numeric type must be read as obsolete.

The canonical model is:

```text
unit identity
    semantic quantity identity

unit-bearing numeric type
    numeric carrier + unit semantic descriptor
```

A unit is therefore not inherently `decimal`, `float64`, `uint`, or another
numeric carrier.

Examples:

```sec
unit m physical

let exact: decimal<m> := 10
let measured: float64<m> := 10
let integral: int64<m> := 10
```

All three values share the unit identity `m`.

### Default numeric representation

A unit declaration may name a default/preferred numeric representation:

```sec
unit Item uint other
```

The default is shorthand behavior only.
It is not part of the unit's semantic identity.

If omitted, the unit default is `decimal`.

### Unit-only shorthand

The type:

```text
<m>
```

uses the unit's default numeric representation.

A compound structural unit expression such as:

```text
<m/s>
```

defaults to `decimal` unless a carrier is written explicitly.

### Named types remain independently nominal

A named type that uses a unit-bearing numeric type remains a distinct named type:

```sec
type Distance decimal<m>
type Altitude decimal<m>
```

`Distance` and `Altitude` are distinct even though they share carrier and unit.

Unit compatibility must not erase named-type identity.

### Compiler-known names

`bit` and `byte` retain their compiler-known language meanings and are not
available as user unit declaration names.

The unit rulebook owns the detailed unit namespace and unit-bearing type rules.

## Cross-reference

Canonical unit semantics are defined by:

```text
rules/types/units.md
```
