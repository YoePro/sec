# Registers

**Status:** Normative  
**Created:** 2026-08-13  
**Last updated:** 2026-10-22 (MD-077 hardware-bound access, field carriers)
**Replaces:** register-type portions of `rules/platform/registers.txt`  
**Document revision:** 4
**Sec language version:** 0.1

## 1. Purpose

A `register[N]` declaration defines a nominal, fixed-width bit-layout type whose total logical width is exactly `N` bits.

Register types are general language types. They are not inherently MMIO types and are not restricted to hardware registers. They may be used for hardware register layouts, protocol headers, packed control words, file-format fields, wire-format values, and other fixed-width bit layouts.

Physical address binding, MMIO, volatile storage, and target-specific access are specified separately by platform rules.

Example:

```sec
 type Control register[16] {
    Enabled: bit,
    Mode: bit[3],
    _: bit[12],
}
```

## 2. Nominal type identity

Each declared register type is nominal.

Two register types are distinct even when they have the same width and identical field layouts.

```sec
 type A register[8] {
    Value: bit[8],
}

 type B register[8] {
    Value: bit[8],
}
```

`A` and `B` are different types.

## 3. Width

The width expression in `register[N]` is a
`SemanticCompileTimeRequiredContext` that must produce a positive integer.
The register rule remains the owner of width/range/domain validation; a
successfully evaluated but invalid value is a register semantic error rather
than evaluator failure. Semantic CTE never reads register/MMIO state.

The sum of the widths of all declared fields, including reserved fields, must equal `N` exactly.

A register declaration that underfills or overfills its declared width is invalid.

```sec
 type Control register[8] {
    Enabled: bit,
    Mode: bit[3],
    _: bit[4],
}
```

## 4. Register field types

A register field may be:

- `bit`, with width 1;
- `bit[N]`, with width `N`;
- a bit-backed enum whose underlying representation is `bit` or `bit[N]`;
- another register type with a compile-time-known width.

`bit` and `bit[N]` are register-field representations. They are not ordinary standalone value types.

Therefore this is not a general Sec variable declaration:

```sec
// Invalid as an ordinary standalone type.
let raw: bit[32]
```

Ordinary raw values use existing types such as `uint8`, `uint16`, `uint32`, `uint64`, or byte arrays as appropriate.

The same spelling has one separate expression use: the compiler-known low-bit extraction `bit[N](value)` (`types.md` "Explicit low-bit extraction"; MD-046/MD-056, 2026-10-12, `rules/corrections/applied/md046-md056-bit-extraction-correction-20261012.md`). It yields an ordinary unsigned integer value, never a standalone `bit[N]` value, and does not make `bit[N]` a variable type.

## 5. Reserved fields

The identifier `_` declares reserved or intentionally unnamed bits.

Reserved fields occupy layout width but do not introduce a named field that can be accessed by source code.

```sec
 type Status register[8] {
    Ready: bit,
    _: bit[7],
}
```

## 6. Bit numbering

**Bit 0 is always the least-significant bit (LSB).**

For an `N`-bit register:

- bit `0` is the LSB;
- bit `N - 1` is the MSB.

Normative layout rules are defined in bit-number terms. Left/right visual descriptions are illustrative only and must not override bit numbering.

## 7. Field allocation order

### 7.1 Default: `lsb-first`

Register field allocation is `lsb-first` by default.

The first declared field occupies the least-significant available bit or bits. Each following field is allocated toward the MSB.

```sec
 type A register[2] {
    A: bit,
    B: bit,
}
```

This means:

```text
A = bit 0
B = bit 1
```

For a wider example:

```sec
 type Control register[16] {
    Enabled: bit,
    Mode: bit[3],
    _: bit[12],
}
```

The layout is:

```text
Enabled = bit 0
Mode    = bits 3..1
_       = bits 15..4
```

### 7.2 Explicit `msb-first`

A register type may explicitly select `msb-first` field allocation.

```sec
 type Header register[8] msb-first {
    Version: bit[4],
    IHL: bit[4],
}
```

The layout is:

```text
Version = bits 7..4
IHL     = bits 3..0
```

With `msb-first`, the first declared field occupies the most-significant available bit or bits and subsequent fields are allocated toward the LSB.

## 8. Byte order

Field allocation order and byte order are independent concepts.

- `lsb-first` / `msb-first` determine how declared fields are assigned logical bit positions.
- byte order determines how a multi-byte register value is encoded to or decoded from ordered bytes in storage, files, protocols, or wire formats.

A register type may explicitly specify:

```sec
 type DeviceWord register[32] little-endian {
    ...
}
```

or:

```sec
 type PacketHeader register[32] big-endian {
    ...
}
```

When no byte order is explicitly specified, the applicable contextual/native byte-order rule is used.

An endianness declaration never changes the register's logical bit numbers or field allocation.

For example:

```sec
 type Word register[16] lsb-first big-endian {
    Code: bit[4],
    Value: bit[12],
}
```

`Code` remains bits `3..0`. `big-endian` controls the byte representation of the complete 16-bit value.

For a one-byte register, explicit endianness is semantically redundant but may be accepted for uniform generated or declarative code.

## 9. Bit-backed enum fields

A bit-backed enum may be used directly as a register field.

```sec
 enum ClockSource bit[2] {
    Internal = 0b00,
    External = 0b01,
    Bypass = 0b10,
}

 type ClockConfig register[8] {
    Source: ClockSource,
    Enabled: bit,
    _: bit[5],
}
```

`Source` occupies exactly two bits and retains the nominal type `ClockSource`.

Bit-backed enums are open over their full representable bit domain. Therefore an undeclared encoding such as `0b11` remains a valid `ClockSource` value and may be read from a register without failure.

Hardware-facing code should normally handle undeclared or reserved encodings explicitly:

```sec
match config.Source {
    ClockSource.Internal => { ... }
    ClockSource.External => { ... }
    ClockSource.Bypass => { ... }
    _ => {
        // Handle reserved or unknown encoding.
    }
}
```

## 10. Nested register fields

A register type may be used as a field type inside another register.

The nested register contributes exactly its declared width and occupies one contiguous slice of the containing register.

```sec
 type Flags register[4] {
    Carry: bit,
    Zero: bit,
    Negative: bit,
    Overflow: bit,
}

 type StatusWord register[16] {
    Flags: Flags,
    Mode: bit[4],
    _: bit[8],
}
```

With the default outer `lsb-first` allocation:

```text
Flags = bits 3..0
Mode  = bits 7..4
_     = bits 15..8
```

### 10.1 Inner layout

The outer register's allocation rule determines where the nested field's contiguous bit slice is placed.

Inside that slice, the nested register's own field-allocation rule applies.

The nested register's bit 0 maps to the least-significant bit of its assigned slice.

Therefore a nested `msb-first` register may internally allocate its fields from the MSB of its slice even when the containing register is `lsb-first`.

### 10.2 Nested endianness

Nested register composition is logical bit composition, not byte serialization.

The outer register's byte-order rule controls encoding or decoding of the complete outer value.

A nested register's own byte-order rule applies when that nested value is serialized or deserialized independently.

No implicit byte swap occurs merely because a nested register declares a different byte order from its containing register.

### 10.3 Recursive layouts

Recursive or otherwise infinitely sized register layouts are invalid.

Every composed register must have a finite, exact compile-time-known width.

## 11. Register field access semantics

Register fields may carry compiler-known access and side-effect semantics.

### 11.1 Ordinary read/write

Ordinary fields are `read-write` by default.

The modifier may be written explicitly when useful:

```sec
Control: bit read-write,
```

### 11.2 Read-only and write-only

```sec
Ready: bit read-only,
Command: bit write-only,
```

Reading a `write-only` field is a compile-time error.

Writing a `read-only` field is a compile-time error.

### 11.3 Specialized write semantics

```sec
Pending: bit write-one-clear,
Enable: bit write-one-set,
Invert: bit write-one-toggle,
Error: bit write-zero-clear,
Disable: bit write-zero-set,
Restore: bit write-zero-toggle,
```

The complete compiler-known write-behavior family is:

```text
write-one-clear
write-one-set
write-one-toggle
write-zero-clear
write-zero-set
write-zero-toggle
```

Writing the named trigger value performs the corresponding clear, set, or
toggle operation. These modifiers describe one logical field write; they do not
describe a multi-register device protocol.

The compiler must not lower such an operation as an ordinary read-modify-write when doing so could change the defined semantics of the physical register or other fields.

### 11.4 Specialized read semantics

```sec
Event: bit read-clear,
Flag: bit read-set,
```

`read-clear` is the canonical spelling for a read whose hardware-defined side
effect clears the field. `read-set` is the symmetric declared behavior whose
hardware-defined side effect sets the field.

The compiler and analyses must treat both reads as effectful and must not duplicate, remove, reorder, or speculate them as ordinary pure reads.

### 11.5 Shadow policy modifiers

The compiler-known `shadow` and `noshadow` modifiers select hardware-access
shadow policy without changing field permission or logical field layout.

The global default is `noshadow`. A register declaration may establish a local
default:

```sec
type Configuration register[16] shadow {
    Mode: bit[3],
    CurrentInput: bit read-only noshadow,
    _: bit[12],
}
```

A field-level `shadow` or `noshadow` overrides the register-level default.
Contradictory policy modifiers on the same declaration are invalid.

This rulebook owns modifier syntax, inheritance, and validation.
`rules/platform/hardware-register-access.md` owns the resulting hardware
binding, shadow-state, explicit `Read()`/`Write()`, and transaction semantics.

### 11.6 Composition of access semantics

Access semantics apply through nested registers.

```sec
 type InterruptFlags register[4] {
    Pending: bit write-one-clear,
    Enabled: bit,
    Ready: bit read-only,
    _: bit,
}

 type DeviceStatus register[16] {
    IRQ: InterruptFlags,
    _: bit[12],
}
```

`DeviceStatus.IRQ.Pending` retains `write-one-clear` semantics and `DeviceStatus.IRQ.Ready` remains read-only.

An operation on an entire nested field is valid only when the compiler can lower it without violating any contained access or side-effect semantics.

### 11.7 Hardware-bound access through the owning `impl` (MD-077)

Amendment MD-077 (2026-10-22, `rules/corrections/applied/md077-register-semantic-ir-final-decision-20261022.md` §§ 1–2, 7):

- **Two kinds of register.** A `register[N]` is a nominal bit-layout type. As a *detached value* (a protocol header, packet field, file format or snapshot) it carries no hardware effect. As a *hardware-bound instance* (`@address`), its physical register is an external resource, not a memory Place.
- **Type-owned `impl`.** A hardware-bound register type requires a type-owned primary `impl`. Normal code reaches the hardware only through the properties and methods that `impl` exposes. Physical fields are available for resolution inside the owning `impl` and are not public members because they appear in the declaration.
- **Field syntax inside the `impl`** resolves to the declared logical register operation. A `write-one-clear` field written with its trigger value is a clear action, not a memory store.
- **Direct access outside the `impl`.** Direct hardware field access outside the owning `impl` is rejected with a diagnostic naming the permitted public member.
- **Permissions.** Permissions and specialized behaviors (§§ 11.1–11.6) are unchanged. No property or method can rename its way around them.

Clarification (owner, 2026-10-23):

- **The receiver decides, not the `impl`.** One `impl T` serves both kinds of register. On a hardware register (`HardwareInstance[T]`), `self.Field` is the field's hardware access with its declared access semantics. On a software register (a detached value in a memory Place, such as an IP or file header), `self.Field` is an ordinary field, much like a struct field. Each member is lowered statically for the receiver kind it is used with. Nothing is dispatched at run time.
- **Specialized semantics on software registers.** On a detached value, a field with specialized read or write semantics (for example `write-one-clear` or `read-clear`) is just bits. Writing it updates the bits, and reading it is pure. The specialized behavior exists only as a hardware operation.
- **`Read` and `Write` are reserved.** In an `impl` of a register type, `Read` and `Write` cannot be declared as members. Inside the `impl`, `self.Read()`, `self.Write()` and `self.Write(value)` always mean the compiler-known hardware operations (`rules/platform/hardware-register-access.md` §§ 7–8). They are not reachable outside the owning `impl`, and they require a hardware receiver. A member that uses them can therefore only be used on a hardware instance. A public operation is published under another name, for example `fn Snapshot() T { return self.Read() }`.
- **Borrowed fields.** A member that borrows `self.Field` (`ref`/`ref mut`) needs a software receiver, because physical register bits are not memory (§ 11.7, MD-077 § 4(1)). Using such a member on a hardware instance is a compile-time error. This also applies when the member is reached through another member that calls it on its own receiver.

## 12. Register values and conversions

A register type is a fixed-width nominal value type.

Explicit conversion from compatible integer values is permitted when the source value fits the register width.

```sec
 type Packet register[32] {
    Value: bit[32],
}

let raw: uint32 := ReadWord()
let packet := Packet(raw)
```

Because every `uint32` value fits in `register[32]`, no width check is required.

When the integer source may contain values outside the register width, the conversion is checked.

```sec
 type Packet12 register[12] {
    Value: bit[12],
}

let raw: uint16 := ReadWord()
let packet := try Packet12(raw)
```

If an out-of-range constant is known at compile time, compilation fails.

```sec
let packet := Packet12(0xABC)   // valid
let invalid := Packet12(0x1ABC) // compile-time error
```

This conversion interprets an already-formed numeric value or bit pattern. It is not byte decoding and does not perform implicit endian conversion.

### 12.1 Plain field writes and explicit bit extraction

A plain `bit[N]` field without a unit annotation keeps its physical width. A write to it is valid only when the field conversion rules prove that the value fits the field's representable unsigned interval; an unproven wider value is never silently truncated, and field assignment itself is not fallible (MD-046, 2026-10-12).

An explicit low-bit extraction no wider than the field proves that fit:

```sec
type Sensor register[16] {
    Reading: bit[12],
    _: bit[4],
}

let source: uint16 := ReadValue()
sensor.Reading = bit[12](source)
```

The extraction's result is its ordinary unsigned carrier (`uint16` here); no second conversion and no `uint12` type are required. Access permissions, special write semantics, shadow contracts and platform operation plans still apply, and extraction never bypasses them.

### 12.2 Field observations and unit-annotated fields

Decided by MD-046 (2026-10-13, `rules/corrections/applied/md040-md046-md047-md056-units-register-bits-rounding-correction-20261013.md` § 4). Field width and unit identity are independent: `bit[N]<U>` occupies `N` physical bits and carries the unit contract `<U>`; `N` stays within the enclosing register and its specification.

Reading a plain multi-bit `bit[N]` field gives the smallest unsigned carrier of `N` bits; reading `bit[N]<U>` gives the same carrier with the unit preserved. The compiler retains the field range `0..2^N-1` in both cases. A one-bit plain field reads as `bool` (§ 4).

| Field | Read value type |
|---|---|
| `bit[4]` | `uint8` |
| `bit[12]` | `uint16` |
| `bit[24]` | `uint32` |
| `bit[12]<rpm>` | `uint16<rpm>` |
| `bit[24]<Hz>` | `uint32<Hz>` |

A write to a multi-bit field must prove both the unit identity — exactly `<U>` for `bit[N]<U>`, no unit for a plain field — and that the value lies in `0..2^N-1`, from a constant or a proven value range (an untyped integer literal takes the field's unit). It is never truncated and never becomes fallible; an unknown or wider value is narrowed explicitly first, and a compound assignment (`+=`, ...) to such a field is rejected because its result is not proven to fit. Reading and writing never bypass the declared access permissions or special write semantics.

```sec
unit rpm uint physical

type Motor register[16] {
    Speed: bit[12]<rpm>,
    _: bit[4],
}

let raw: uint16 := ReadRawSpeed()
let speed: uint16<rpm> := ReadSpeed()

motor.Speed = bit[12](raw)<rpm>     // labels raw bits with <rpm>
motor.Speed = bit[12](speed)<rpm>   // keeps the source's own unit
// motor.Speed = bit[12](raw)       // error: missing unit identity
// motor.Speed = speed              // error: not proven to fit 0..4095
```

`bit[N](value)` never inherits a unit; the suffix `bit[N](value)<U>` labels a unitless value or keeps the source's own unit, and never changes a unit (`types.md` "Explicit low-bit extraction"; `units.md` "Interaction with registers and FFI").

Byte-oriented encoding and decoding are separate operations and must obey the register type's byte-order contract.

### 12.x Field carriers and lossy extraction (MD-077)

Amendment MD-077 (2026-10-22, `rules/corrections/applied/md077-register-semantic-ir-final-decision-20261022.md` §§ 5–6):

- **Detached values.** Field extraction and update on a detached register value are pure value operations. They imply no hardware transaction, and a snapshot returned by a hardware read stays detached.
- **`bit` fields.** A `bit` field reads as `bool`.
- **`bit[N]` fields with `N >= 2`.** They read as the smallest standard unsigned carrier: `uint8` up to 8 bits, then `uint16`, `uint32`, `uint64`, `uint128` and `uint256`. Sema also records the proven interval `0..2^N-1`. That interval is a refinement fact, not a named `range` type, and `bit[N]` is never a standalone integer type.
- **Explicit extraction.** `bit[N](value)`, including `N = 1`, still returns the unsigned carrier of `types.md`, never `bool`. It is the only low-bit extraction and is never inserted implicitly by a field write (§ 12.1).
- **Bit-backed enums.** A bit-backed enum field keeps its nominal enum type and open bit domain.

## 13. Field access

Named register fields are accessed using ordinary member syntax:

```sec
let enabled := control.Enabled
control.Mode = value
```

A reserved `_` field cannot be named or accessed.

Field access retains the nominal type of the declared field, including bit-backed enum and nested register types.

## 14. Implementations

Register types are nominal Sec types and may have implementations.

```sec
 type Status register[8] {
    Ready: bit read-only,
    Error: bit,
    _: bit[6],
}

 impl Status {
    fn IsHealthy() bool {
        return self.Ready && !self.Error
    }
}
```

Register types may participate in methods, properties, and interface implementations where the corresponding general declaration rules permit them.

The detailed semantics of `impl`, properties, and interface implementation are defined by their respective rulebooks.

## 15. Separation from fixed-address and MMIO semantics

A register declaration does not by itself imply:

- a physical address;
- MMIO;
- volatile access;
- hardware storage identity;
- target address validity;
- address aliasing;
- device-specific read/write effects beyond field semantics explicitly declared by the register type.

These concerns are specified separately by fixed-address and platform rules.

The same register type may therefore be used as an ordinary value, decoded protocol value, file-format value, or fixed-address hardware register according to context.

## 16. Diagnostics

The compiler must diagnose at least:

- non-constant or non-positive register width;
- field widths whose sum does not equal the declared register width;
- invalid field types;
- recursive or infinitely sized nested register layouts;
- invalid or contradictory field access semantics;
- invalid or contradictory `shadow`/`noshadow` policy, including invalid register-level defaults;
- reads from `write-only` fields;
- writes to `read-only` fields;
- operations that cannot be lowered without violating specialized read/write, shadow, or nested access semantics;
- integer-to-register conversions whose out-of-range value is statically known;
- runtime integer-to-register conversions lacking required checked handling when fit cannot be proven.

## 17. Required semantic invariants

A conforming implementation must preserve the following invariants:

1. bit 0 is always the LSB;
2. `lsb-first` is the default field-allocation rule;
3. `msb-first` is an explicit override;
4. field allocation order is independent of byte order;
5. register width is exact and compile-time known;
6. reserved fields occupy bits but are not source-accessible;
7. bit-backed enum fields retain nominal enum typing and accept every representable underlying bit pattern;
8. nested registers occupy a contiguous compile-time-known bit slice and preserve inner semantics;
9. special field access semantics are compiler-known and cannot be erased by lowering;
10. register declarations do not themselves imply fixed-address or MMIO storage;
11. register types may have implementations under the normal `impl` rules.
12. `read-clear` is the canonical destructive-read spelling;
13. shadow policy does not add fields to or otherwise change a register's logical bit layout.
