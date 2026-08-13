# Fixed-address bindings

**Status:** Normative  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Replaces:** `rules/platform/registers.txt`  
**Document revision:** 1  
**Sec language version:** 0.1

## 1. Purpose

A fixed-address binding associates a Sec value with storage whose address is
fixed by a platform, device, linker contract, or other target-defined address
domain. A fixed-address binding is a storage/access property. It is not a type
category and does not make `register[N]` itself an MMIO type.

The abstract fixed-address storage contract is defined by
`rules/memory/storage.md`. Register type and bit-layout semantics are defined by
`rules/declarations/registers.md`.

## 2. Addressed bindings

The current Sec 0.1 source form binds an explicitly typed register value to a
compile-time-known address:

```sec
@address(0x40021000)
let status: DeviceStatus
```

The same register type may be used as an ordinary value without a binding:

```sec
let snapshot: DeviceStatus
```

Creating an addressed binding:

- uses `AddressStability.Fixed`;
- does not introduce a new storage origin;
- does not allocate or initialize the platform storage;
- does not transfer ownership of the platform storage;
- does not by itself prove permanent or unlimited lifetime;
- prevents relocation of the bound storage while the contract is valid.

Its storage origin remains independently classified. A linker-defined region
may be `Static`, while platform-owned hardware storage may be `Unknown` unless
a stronger target contract exists.

## 3. Fixed-address contract

A fixed-address binding consumes the canonical storage contract, including:

```text
address identity
target address domain
extent
alignment
layout
memory space
access permissions
aliasing policy
reclamation authority
lifetime or availability contract
```

Fixed address placement is not itself an origin and does not by itself imply
volatility, mutability, atomicity, thread safety, or unlimited lifetime.

## 4. `@address` volatility

The abstract fixed-address contract does not imply volatility. The Sec 0.1
`@address` form is explicitly hardware-facing and therefore does.

Every source-level read that must observe the addressed storage is observable.
Repeated reads may observe different values:

```sec
let first := status.Ready
let second := status.Ready
```

The compiler must not replace `second` with `first` merely because no Sec write
is visible between them.

Required volatile semantics include:

- required reads are not removed as dead pure loads;
- required writes are not removed merely because their results are unused;
- accesses are not merged, duplicated, speculated, or reordered when that
  changes observable platform behavior;
- a read with additional semantics such as `clear-on-read` remains effectful
  and receives the stronger ordering required by that semantic.

Sec does not require a separate source-level `volatile` keyword for an
`@address` hardware binding.

## 5. Binding mutability

Binding mutability and volatility are independent.

```sec
@address(0x40021000)
let status: DeviceStatus
```

This binding is volatile and readable when its type and fields permit reading,
but Sec code cannot write through it. Hardware may still change its value.

```sec
@address(0x40021000)
let mut control: DeviceControl
```

`mut` grants Sec write authority when the type and field semantics permit it.
It does not override `read-only`, `write-only`, `write-one-clear`,
`write-zero-clear`, or `clear-on-read` register field semantics.

## 6. Addressed field operations

Register field meaning is defined by `rules/declarations/registers.md`. This
rulebook owns the storage-access consequences when those operations target an
addressed binding.

For an ordinary `read-write` field, a volatile read-modify-write is permitted
only when it is semantically safe for the complete physical register access.
The compiler must account for:

```text
reserved bits
read-only fields
write-only fields
write-one-clear fields
write-zero-clear fields
clear-on-read fields
nested register fields containing special semantics
target-specific write masks and access restrictions
```

If ordinary read-modify-write would produce an unintended read or write side
effect, the compiler must use a semantics-preserving target operation. If none
is available, compilation must fail rather than emit an unsafe sequence.

The programmer is not required to reproduce masks and shifts for a correctly
declared register field.

## 7. Bit allocation and byte order

Logical register layout remains defined exclusively by
`rules/declarations/registers.md`:

- bit 0 is the LSB;
- `lsb-first` and `msb-first` control logical field allocation;
- field allocation is independent of byte order.

At an addressed storage boundary, an explicit `little-endian` or `big-endian`
register declaration controls multi-byte loads and stores. Without an explicit
declaration, the applicable target/native contract controls the representation.

No platform rule may reinterpret field allocation order as byte endianness.

## 8. Address validation

The current `@address` form requires a compile-time-known integer address. The
compiler and selected target profile must validate every statically knowable
requirement, including as applicable:

```text
address-domain validity
required alignment
bound extent
access width
layout compatibility
memory-space compatibility
read and write permission
forbidden or unavailable address ranges
```

A target may define a linear integer address domain or another compiler-known
target address domain. `@address` does not grant permission to bypass known
target restrictions.

Dynamic or otherwise unverifiable access requires a separately specified
checked compiler-known API or an explicit `unsafe` boundary. Ordinary
statically validated `@address` use does not require `RawPtr`.

## 9. Initialization

An addressed binding refers to existing external or platform storage. Its
declaration must not perform an implicit write, so an initializer is invalid:

```sec
@address(0x40021000)
let mut control: DeviceControl := initialValue
```

Required initialization writes are explicit after the binding:

```sec
@address(0x40021000)
let mut control: DeviceControl

control.Enabled = true
```

## 10. Address identity, overlap, and aliasing

Statically known overlapping bindings are checked against the canonical
fixed-address contract. The compiler must consider at least address range,
extent, layout, alignment, permissions, access width, memory space, and aliasing
policy.

Bindings known to overlap incompatibly are a compile-time error. Intentional
overlap is valid only when a compiler-known platform contract declares the
views compatible. This rule introduces no source-level alias keyword.

## 11. Register implementations on addressed storage

Register `impl` eligibility belongs to the register and implementation
rulebooks. When a method or property operates on an `@address`-bound receiver,
field access through `self` preserves the binding semantics:

```text
reads remain volatile
writes remain volatile
binding mutability is enforced
register field restrictions are enforced
special read and write semantics are preserved
```

Calling the same member on an ordinary non-addressed value does not make the
access volatile. No impl member adds storage to the physical register layout.

## 12. Type eligibility

Sec 0.1 accepts the addressed-register form:

```sec
@address(0x40021000)
let status: RegisterType
```

Extending `@address` to structs, arrays, or other types requires an explicit
physical-layout contract sufficient for the fixed-address requirements. Such an
extension must be specified deliberately and is not inferred from ordinary Sec
struct layout.

## 13. Diagnostics

The compiler must diagnose at least:

- an invalid `@address` argument form;
- a non-constant address where the current syntax requires a constant;
- a target-invalid or misaligned address;
- an incompatible extent, layout, access width, or memory space;
- a write through a non-`mut` addressed binding;
- a read or write forbidden by register field semantics;
- field lowering that would violate W1C, W0C, clear-on-read, or nested special
  semantics;
- statically known incompatible overlapping bindings;
- an initializer on an addressed binding;
- unverifiable dynamic fixed-address access outside the required checked or
  `unsafe` mechanism.

Register declaration and layout diagnostics belong to
`rules/declarations/registers.md`.

## 14. Required tests

Coverage must include:

```text
read-only and read-write @address bindings
volatile read and write preservation
external mutation between repeated reads
initializer rejection
target address and alignment failures
incompatible overlap
ordinary and restricted addressed field operations
W1C, W0C, and clear-on-read lowering constraints
nested register special semantics
explicit register byte order at the addressed boundary
impl access through addressed and ordinary receivers
```
