# Platform registers extraction and rename correction

**Status:** Applied  
**Applied:** 2026-08-13  
**Created:** 2026-08-13  
**Last updated:** 2026-08-13  
**Document revision:** 1  
**Sec language version:** 0.1  
**Primary target:** `rules/platform/registers.txt`  
**Rename target:** `rules/platform/fixed-address-bindings.md`  
**Register type source of truth:** `rules/declarations/registers.md`  
**Storage source of truth:** `rules/memory/storage.md`

## 1. Required file operation

Rename:

```text
rules/platform/registers.txt
```

to:

```text
rules/platform/fixed-address-bindings.md
```

Use a repository move/rename so file history is retained where practical.

Do not retain a second normative copy at the old path.

After the rename, update repository references that still name
`rules/platform/registers.txt` or refer to `registers.txt` as the owner of
fixed-address/MMIO semantics.

The general `register[N]` type is now specified by:

```text
rules/declarations/registers.md
```

The renamed platform rulebook must consume that rule and must not duplicate it.

## 2. New scope of `fixed-address-bindings.md`

The renamed rulebook specifies source-level fixed-address hardware bindings and
their platform/storage semantics.

It owns at least:

```text
@address(...)
fixed-address source bindings
MMIO-facing volatility
binding mutability
observable reads and writes
target address validation
alignment and access-width validation
fixed-address extent and layout compatibility
address aliasing and overlap checks
initialization restrictions
safe versus unsafe address binding
lowering requirements for volatile addressed access
interaction with register field access semantics
```

It does not own:

```text
register[N] declaration syntax
bit / bit[N] field representation
register width calculation
reserved fields
bit numbering
lsb-first / msb-first field allocation
register byte-order declarations
bit-backed enum field rules
nested register composition
register nominal typing
integer-to-register conversion
register impl eligibility
```

Those rules belong to `rules/declarations/registers.md`.

## 3. Required opening model

The renamed rulebook should begin from this separation:

> A fixed-address binding associates a Sec value with storage whose address is
> fixed by a platform, device, linker contract, or other target-defined address
> domain. A fixed-address binding is a storage/access property. It is not a
> type category and does not make `register[N]` itself an MMIO type.

A register type may exist and be used without any fixed-address binding:

```sec
let snapshot: DeviceStatus
```

An addressed instance binds a value to platform storage:

```sec
@address(0x40021000)
let status: DeviceStatus
```

The same register type may therefore be used both as an ordinary value and as
an addressed hardware value.

## 4. Relationship to the canonical storage model

The renamed rulebook must consume `rules/memory/storage.md` rather than define a
competing storage classification.

Normative requirements:

- fixed-address placement uses `AddressStability.Fixed`;
- fixed-address placement is not a storage origin;
- storage origin remains independently classified;
- creating a binding does not allocate the platform storage;
- creating a binding does not initialize the platform storage;
- creating a binding does not transfer ownership of the platform storage;
- creating a binding does not by itself prove permanent or unlimited lifetime;
- the address is part of the binding/storage contract and the bound storage may
  not relocate while that contract is valid.

The fixed-address contract consumed from `storage.md` includes at least:

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

Do not describe `FixedAddress` as an independent storage origin.

## 5. `@address` and volatility

The abstract fixed-address storage contract does not by itself imply volatility.

For Sec 0.1, an `@address` hardware binding is an explicitly hardware-facing
fixed-address binding and therefore has volatile access semantics.

Example:

```sec
@address(0x40021000)
let status: DeviceStatus
```

Each source-level read that must observe the addressed storage is an observable
access.

The compiler must not assume that repeated reads return the same value:

```sec
let first := status.Ready
let second := status.Ready
```

The second read may not be replaced with `first` merely because no Sec write is
visible between the accesses.

Required volatile semantics include:

- required reads are not removed as dead pure loads;
- required writes are not removed merely because their values are unused;
- volatile accesses are not merged, duplicated, speculated, or reordered when
  doing so changes observable device/platform behavior;
- a read with additional register semantics such as `clear-on-read` remains an
  effectful read and receives the stronger ordering required by that semantic.

Sec does not require a separate source-level `volatile` keyword for
`@address` hardware bindings.

## 6. Binding mutability

Binding mutability and volatility are independent.

```sec
@address(0x40021000)
let status: DeviceStatus
```

means:

```text
volatile
readable from Sec when the type/field permits reading
not writable from Sec through this binding
```

External hardware may still change the value.

```sec
@address(0x40021000)
let mut control: DeviceControl
```

means:

```text
volatile
writable from Sec when the type/field semantics permit writing
```

`mut` grants write authority to the binding. It does not override more
restrictive register-field semantics.

Therefore a `read-only` register field remains non-writable even through an
`@address` binding declared with `mut`.

Likewise, `write-only`, `write-one-clear`, `write-zero-clear`, and
`clear-on-read` semantics defined by `rules/declarations/registers.md` remain in
force when the register is addressed.

## 7. Field access lowering

The platform rulebook owns the storage-access consequences of addressed field
operations; it does not redefine register field meaning.

For an ordinary `read-write` field, the compiler may use volatile
read-modify-write when that transformation is semantically safe.

A read-modify-write sequence is not automatically valid merely because the
selected field is writable.

Before using read-modify-write, the compiler must account for the semantics of
the complete physical register access, including:

```text
reserved bits
read-only fields
write-only fields
write-one-clear fields
write-zero-clear fields
clear-on-read fields
nested register fields containing any such semantics
target-specific write masks or access restrictions
```

If ordinary read-modify-write would cause an unintended read or write side
effect, the compiler must instead use a target/device operation that preserves
the declared semantics.

If no valid lowering is available under the target contract, compilation must
fail rather than silently emit an unsafe read-modify-write sequence.

The programmer should not be required to manually reproduce masks and shifts
for a correctly declared register field.

## 8. Bit allocation and byte order at an addressed boundary

Logical register layout remains defined exclusively by
`rules/declarations/registers.md`.

In particular:

- bit 0 remains the LSB;
- `lsb-first` / `msb-first` control field allocation;
- field allocation is independent of byte order.

When an addressed multi-byte register type has an explicit `little-endian` or
`big-endian` declaration, fixed-address loads and stores must preserve that byte
order at the storage boundary.

When no explicit byte order is declared, the applicable target/native contract
controls the addressed representation.

No platform rule may reinterpret `lsb-first` or `msb-first` as byte endianness.

## 9. Address validation

For the current `@address` source form, the address is compile-time known.

The compiler and selected target profile must validate all statically knowable
requirements, including as applicable:

```text
address-domain validity
required alignment
bound extent
access width
layout compatibility
memory-space compatibility
read/write permission
forbidden or unavailable address ranges
```

A target may define a linear integer address domain or another target-defined
address domain through compiler-known platform metadata.

The source-level `@address` syntax must not be treated as permission to bypass
known target restrictions.

Dynamic or otherwise unverifiable fixed-address access is not made safe by this
rule. Such access requires a separately specified checked compiler-known API or
an explicit `unsafe` boundary.

Ordinary statically validated `@address` use does not require `RawPtr`.

## 10. Initialization

An addressed binding refers to already existing external/platform storage.

Declaring the binding must not perform an implicit hardware write.

Therefore an ordinary initializer is invalid:

```sec
@address(0x40021000)
let mut control: DeviceControl := initialValue
```

Required initialization writes must be explicit after the binding:

```sec
@address(0x40021000)
let mut control: DeviceControl

control.Enabled = true
```

This preserves source visibility of hardware side effects.

## 11. Address identity, overlap, and aliasing

Statically known overlapping fixed-address bindings must be checked against the
canonical fixed-address storage contract.

The compiler must consider at least:

```text
address range
extent
layout
alignment
mutability/access permission
access width
memory-space contract
aliasing policy
```

Two declarations that are statically known to overlap incompatibly are a
compile-time error.

Intentional overlap is valid only when a compiler-known target/platform contract
explicitly declares the overlapping views compatible under its aliasing policy.

This correction does not introduce a new source-level alias keyword or syntax.

Do not retain older wording that merely says overlapping declarations "may be
rejected" without a semantic criterion.

## 12. Register implementations on addressed storage

The ability of a register type to have an `impl` belongs to the general register
and implementation rulebooks.

The platform consequence must nevertheless be stated:

When an impl method or property operates on an `@address`-bound register value,
field access through `self` preserves the binding's platform semantics.

Therefore:

```text
reads remain volatile
writes remain volatile
binding mutability is enforced
register field access restrictions are enforced
special read/write semantics are preserved
```

Calling the same method on an ordinary non-addressed register value does not by
itself make the access volatile.

No impl member adds storage to the physical register layout.

## 13. Type eligibility boundary

This correction does not silently generalize `@address` to every Sec type.

The existing Sec 0.1 addressed-register form remains valid:

```sec
@address(0x40021000)
let status: RegisterType
```

Any extension allowing ordinary structs, arrays, or other types to be bound by
`@address` requires that those types have an explicit physical-layout contract
sufficient to satisfy the fixed-address storage requirements. That extension
must be specified deliberately rather than inferred from ordinary Sec struct
layout.

The renamed filename reflects that the platform semantics are fundamentally
about fixed-address bindings, while current source eligibility may remain more
restricted.

## 14. Compiler validation required by this rulebook

The renamed rulebook should require diagnostics for at least:

- invalid `@address` argument form;
- non-constant address where the current syntax requires a constant;
- target-invalid address;
- misaligned binding;
- extent or access-width incompatibility;
- invalid target memory-space use;
- write through a non-`mut` addressed binding;
- write to a register field whose field semantics forbid it;
- read from a register field whose field semantics forbid it;
- addressed field lowering that would violate W1C, W0C, clear-on-read, or
  nested special semantics;
- statically known incompatible overlapping fixed-address bindings;
- initializer on an addressed binding;
- unsafe/unverifiable dynamic fixed-address access outside the required checked
  or `unsafe` mechanism.

Layout diagnostics for the `register[N]` type itself belong to
`rules/declarations/registers.md` and must not be duplicated here.

## 15. Required tests

The renamed rulebook should require tests covering at least:

```text
read-only @address binding
read-write @address binding
external mutation between repeated reads
volatile read preservation
volatile write preservation
initializer rejection
target-invalid address
alignment failure
overlapping incompatible bindings
addressed ordinary register field update
addressed read-only field rejection
addressed write-only field read rejection
addressed W1C lowering constraint
addressed W0C lowering constraint
addressed clear-on-read effect preservation
nested register special semantics through @address
explicit register endianness at addressed storage boundary
impl method on addressed receiver preserves volatile semantics
same impl method on ordinary value is non-volatile
```

## 16. Sections to remove from the old platform rulebook

The following subjects must be removed from the renamed platform document rather
than duplicated:

```text
Register Type Declaration
Bit Fields
Enum-Backed Bit Fields
Reserved Bits
Units on Register Fields
Register Values and Ordinary Values, except the addressed/non-addressed
    distinction needed by this platform rule
Impl Blocks on Register Types, except the addressed-receiver consequence
register width/layout validation
bit-backed enum width/type rules
```

Their canonical replacement is `rules/declarations/registers.md` and the
adjacent declaration/type rulebooks it references.

## 17. Sections to retain and rewrite from the old platform rulebook

The semantic material from these old sections remains relevant but must be
rewritten under the new fixed-address model:

```text
Addressed Register Instances
Volatile Semantics
Read-Only Addressed Instances
Read-Write Addressed Instances
Field Updates
Initialization
Address Uniqueness and Aliasing
Safety and Unsafe Code
platform-specific parts of Compiler Validation
addressed-access parts of Example
```

Do not preserve old statements that conflict with the canonical storage model or
the new register declaration rulebook.

## 18. Normative-document cleanup

The renamed normative rulebook must use Markdown and include normal document
metadata.

Remove the embedded `Current Implementation Status` section from the normative
rulebook.

Preserve implementation knowledge in a separate delta-only status artifact,
for example:

```text
implementation-status-fixed-address-bindings.yaml
```

Do not replace or overwrite any broader locally maintained implementation-status
file.

Remove the old `Future Extensions` dump. Any unresolved item belongs in the
appropriate roadmap/status artifact or in a dedicated normative rule when its
semantics are defined.

## 19. Required cross-reference updates

Repository references must be updated so that:

```text
rules/declarations/registers.md
    owns register type semantics

rules/platform/fixed-address-bindings.md
    owns @address/MMIO fixed-address binding semantics

rules/memory/storage.md
    owns the abstract fixed-address storage contract
```

In particular, references in `storage.md` that currently say volatility or
fixed-address specialization comes from `registers.txt` should point to
`rules/platform/fixed-address-bindings.md`.

No document should continue using `rules/platform/registers.txt` as the
canonical register declaration rulebook.
