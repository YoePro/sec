# Registers

**Status:** Normative  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Replaces:** register-type portions of `rules/platform/registers.txt`  
**Document revision:** 2  
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

The width expression in `register[N]` is a compile-time constant positive integer.

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

### 11.3 Write-one-clear

```sec
Pending: bit write-one-clear,
```

Writing one performs the field's defined clear operation.

The compiler must not lower such an operation as an ordinary read-modify-write when doing so could change the defined semantics of the physical register or other fields.

### 11.4 Write-zero-clear

```sec
Error: bit write-zero-clear,
```

Writing zero performs the field's defined clear operation.

The compiler must preserve this semantic operation and must not replace it with an unsafe ordinary read-modify-write sequence.

### 11.5 Clear-on-read

```sec
Event: bit clear-on-read,
```

Reading the field has a semantic side effect: the field is cleared according to the backing storage or device contract.

The compiler and analyses must treat such reads as effectful and must not duplicate, remove, reorder, or speculate them as ordinary pure reads.

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

Byte-oriented encoding and decoding are separate operations and must obey the register type's byte-order contract.

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
- reads from `write-only` fields;
- writes to `read-only` fields;
- operations that cannot be lowered without violating W1C, W0C, clear-on-read, or nested access semantics;
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
