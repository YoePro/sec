# Sec Volatile Access

- **Status:** Normative
- **Created:** 2026-08-26
- **Last updated:** 2026-08-28
- **Document revision:** 2
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `fc8d632`
- **Canonical path:** `rules/platform/volatile.md`

## 1. Purpose

This rulebook defines volatile physical-storage access in Sec 0.1.

Volatile access is used when the act of reading or writing storage is externally
observable or when the storage may change independently of ordinary Sec writes.

This rulebook owns:

- the meaning of volatile reads and writes;
- explicit raw volatile access;
- the relationship between `@address` and volatile access;
- physical access contracts for volatile/MMIO storage;
- access-width, alignment, address-space, byte-order, and ordering constraints;
- volatile versus atomic/synchronization semantics;
- optimizer restrictions for volatile effects;
- physical representability requirements;
- lowering requirements;
- diagnostics and tooling expectations.

This rulebook does not define:

- logical register-field transaction semantics;
- destructive-read field grouping;
- physical register read groups or selective field observation;
- read-clear, FIFO-advancing, latch-on-read, or similar register-specific
  transaction models beyond requiring their stronger effects to be preserved;
- interrupt semantics;
- DMA ownership protocols;
- cache-maintenance APIs;
- atomic memory ordering;
- a target/device knowledge-pack format.

Logical hardware-register observation, transaction footprints, destructive-read
grouping, aliases, shadow state, and register-operation planning are owned
canonically by `rules/platform/hardware-register-access.md`.

---

## 2. Core semantic model

Volatile is a storage/access semantic.

It is not a Sec type qualifier.

Sec 0.1 does not define C-style forms such as:

```text
volatile uint32
RawPtr[volatile uint32]
```

and does not define a general `volatile` keyword or general `@volatile`
attribute.

Volatility describes how a physical access must be treated.

It does not change the semantic identity of the value type being accessed.

---

## 3. Volatile storage and ordinary values

A volatile read produces an ordinary detached Sec value.

Conceptually:

```text
volatile storage
        ↓
volatile read
        ↓
ordinary Sec value
```

Volatility does not propagate into the returned value.

After:

```sec
let status := deviceStatus
```

where `deviceStatus` is read from volatile storage, subsequent ordinary use of
`status` does not repeat the physical read.

A new hardware/storage observation requires a new source-level access.

---

## 4. Register types do not imply volatility

A `register[N]` type is a nominal fixed-width value type.

The register type itself does not imply:

- MMIO;
- fixed address;
- external storage;
- volatility;
- a particular physical bus operation.

An ordinary local value of a register type has ordinary value semantics.

A register type participates in volatile physical access only where the owning
register-access rules and resolved storage contract define a legal physical
operation.

Register transaction semantics stronger than ordinary volatility are not
defined by this rulebook.

---

## 5. `@address` and volatility

The Sec 0.1 hardware-facing `@address` binding form implies volatile access for
the addressed external storage that it binds.

For example:

```sec
@address(Peripheral.GPIOA)
let mut gpio: GPIORegisters
```

does not require an additional `@volatile` annotation.

Abstract fixed-address storage does not universally imply volatility.

A linker-placed ordinary RAM object may have a stable address without being
volatile.

The volatility of a concrete storage region comes from the applicable
storage/device contract and the owning source construct.

---

## 6. `@address` region validation

A compile-time-known numeric address is necessary for raw numeric Sec 0.1
`@address` syntax but is not sufficient to make a binding valid.

Every `@address` binding must resolve to an applicable canonical address-region
or storage contract supplied by the active `CompilationPlan`,
`MemoryEnvironment`, `DeviceModel`, or another canonical target-owned source.

The complete bound extent must be valid for that region.

Validation includes, as applicable:

```text
AddressDomain
AddressSpace / MemorySpace
region extent
availability
read/write permissions
physical representation
alignment
allowed access widths
storage kind
device compatibility
```

An address that is numerically well formed but is not covered by an applicable
canonical region is invalid for `@address`.

---

## 7. Hosted targets and `@address`

Hosted execution does not make arbitrary process virtual addresses valid
`@address` bindings.

A Hosted target may nevertheless permit `@address` where the concrete target
environment explicitly defines a stable compatible mapped region.

This is important for legitimate hosted low-level use such as:

- drivers;
- mapped peripherals;
- GPIO on systems such as Raspberry Pi;
- PCIe/FPGA mappings;
- other target-defined device mappings.

Therefore:

```text
Hosted != @address forbidden
```

and:

```text
Hosted != arbitrary process address allowed
```

The concrete target memory/device model decides which regions are valid.

---

## 8. `unsafe` does not waive `@address` validation

`unsafe` does not make an unverified `@address` declaration legal.

The following principle is normative:

> `@address` is a compiler-validated fixed-storage binding.

If the compiler cannot resolve and validate the binding against a canonical
region contract, the declaration is invalid even inside `unsafe`.

Dynamic or otherwise unverifiable address access uses an applicable `RawPtr` or
checked low-level mechanism instead.

`unsafe` also grants no operating-system permission and cannot make unmapped,
inaccessible, protected, or foreign-process memory valid.

---

## 9. Explicit raw volatile access

Sec 0.1 provides compiler-known raw volatile operations equivalent to:

```text
RawPtr[T].VolatileRead() T
RawPtr[T].VolatileWrite(value: T) void
```

Example:

```sec
unsafe {
    let value := address.VolatileRead()
    address.VolatileWrite(value)
}
```

These are explicit observable physical-storage operations.

They are method-shaped operations because performing them is effectful.

They need not be implemented as ordinary core-library source.

---

## 10. Ordinary RawPtr access remains distinct

Ordinary raw-pointer read/write and volatile raw-pointer access remain
semantically distinct.

Conceptually:

```text
Read / Write
    ordinary raw memory access

VolatileRead / VolatileWrite
    explicitly observable raw physical-storage access
```

The compiler does not inspect the runtime numeric address of an ordinary
`RawPtr` read and silently change it into a volatile operation.

Volatile behavior must come from the source/storage contract.

---

## 11. Raw volatile access requires `unsafe`

Ordinary `RawPtr` volatile access requires an `unsafe` context.

The programmer accepts obligations the compiler cannot prove for the dynamic
address, including as applicable:

```text
address validity
region/storage identity
lifetime
alignment
permissions
correct address space
correct access width
representation compatibility
```

`unsafe` does not waive restrictions that the compiler already knows are
forbidden by the selected target or storage contract.

If the compiler knows an access width, alignment, address space, or physical
operation is illegal for the target, the access is rejected even inside
`unsafe`.

---

## 12. Physical access contract

Every statically known volatile/MMIO storage region has a canonical physical
access contract derived from the active target plan.

Conceptually, such a contract may contain facts equivalent to:

```text
VolatileAccessContract {
    AddressDomain
    MemorySpace
    ReadPermission
    WritePermission
    AccessWidths
    Alignment
    ByteOrder
    AccessGranularity
    OrderingRequirements
    MemoryAttributes
    SideEffectSemantics
}
```

The exact compiler representation is implementation-specific.

The contract is semantic input to physical lowering.

It is not reconstructed from compiler-host behavior.

---

## 13. Address spaces and memory spaces

A numeric address alone is not necessarily a globally unique storage identity.

Targets may distinguish address domains/spaces such as:

```text
Program
Data
MMIO
Device
SpecialAddressSpace
```

or target-specific equivalents.

In a Harvard or modified-Harvard target model, equal numeric addresses in
different address spaces may denote different storage.

Conceptually:

```text
AddressDomain + AddressSpace + Address
```

identifies the applicable storage location more precisely than the numeric
address alone.

A unified target may expose one programmer-visible address space while still
classifying regions as Flash, RAM, MMIO, or other storage kinds.

---

## 14. Access width

Physical volatile access width is determined by the applicable storage/device
contract, not merely by the logical source field width.

If a storage location requires a 32-bit physical read, reading one logical bit
may require:

```text
32-bit volatile read
        ↓
ordinary local bit extraction
```

The compiler must not substitute an 8-bit, 16-bit, 64-bit, split, or merged
access merely because that operation appears convenient.

A contract may require one exact physical access width.

---

## 15. Split, merged, narrowed, and widened accesses

A required volatile operation may be decomposed, combined, narrowed, or widened
only when the applicable target/storage contract explicitly permits a
semantics-preserving transformation.

The compiler must not automatically transform:

```text
one 32-bit read
```

into:

```text
two 16-bit reads
```

or:

```text
four 8-bit reads
```

when those operations may have different device semantics.

The same rule applies to writes.

For aggregates, a volatile access is legal only when the compiler can construct
a semantics-preserving physical access plan for the complete operation.

---

## 16. Alignment

Volatile access alignment must satisfy all applicable constraints, including:

```text
Sec representation alignment
target instruction requirements
address-space requirements
region requirements
device requirements
```

Statically known `@address` bindings are validated at compile time.

Dynamic `RawPtr` volatile accesses place unverifiable address-alignment
obligations on the programmer, but known target-invalid alignments remain
compile-time errors.

---

## 17. Byte order

Volatile/MMIO byte order is resolved from target, device, memory-space, and
storage contracts.

Compiler-host byte order never defines target MMIO semantics.

The compiler distinguishes as needed:

```text
CPU/native byte order
device/register byte order
wire/storage byte order
```

When conversion is required, lowering may perform it only when doing so
preserves the storage/device access contract.

Canonical register bit/field meaning is interpreted over the resolved physical
register value rather than compiler-host endian behavior.

---

## 18. Memory attributes are not implied by volatile

Volatile does not mean:

```text
uncached
cache bypass
DMA coherent
strongly ordered
write-through
write-combining
```

Cacheability, coherence, device-memory classification, write combining, and
related attributes come from the active memory/device model.

A volatile load does not automatically invalidate caches.

A volatile store does not automatically flush caches.

---

## 19. DMA and external memory actors

DMA and similar hardware engines may modify storage without an ordinary Sec
write.

Volatile access alone does not define:

- DMA ownership transfer;
- cache maintenance;
- buffer coherency;
- publication/synchronization;
- descriptor hand-off ordering.

Those requirements come from canonical DMA/device/memory contracts.

A DMA-visible buffer need not be globally volatile.

Ordinary memory plus explicit ownership/coherence transitions may be the correct
model.

---

## 20. Observable read semantics

Every executed semantic volatile read is an observable access.

The compiler must preserve the fact that the read occurs.

Two distinct source-level volatile reads remain two distinct observations unless
a stronger owning storage contract explicitly defines an equivalent combined
operation.

For example:

```sec
let first := status.VolatileRead()
let second := status.VolatileRead()
```

must not become one physical read reused twice merely because Sec code did not
perform an intervening write.

External state may have changed between the observations.

---

## 21. Observable write semantics

Every executed semantic volatile write is an observable access.

The compiler must not remove a volatile write merely because no later Sec code
reads the written storage.

Two equal volatile writes remain two observable operations.

For example:

```sec
command.VolatileWrite(reset)
command.VolatileWrite(reset)
```

must not be deduplicated merely because the values are equal.

Each write may represent a distinct hardware command.

---

## 22. No speculative volatile accesses

The compiler must not introduce a volatile access on a control-flow path where
the Sec execution semantics would not perform that access.

For example:

```sec
if enabled {
    let status := device.VolatileRead()
}
```

must not be transformed into an unconditional volatile read followed by a
conditional use.

This restriction includes speculative execution introduced by compiler
optimization.

Hardware-level speculative behavior remains governed by the selected target
memory/device model and required barriers.

---

## 23. Repeated control flow

A volatile read executed on each iteration of a loop remains one distinct
observation per executed iteration.

For example:

```sec
while !status.VolatileRead().Ready {
}
```

must not be transformed by hoisting the read outside the loop.

Separate analyses may diagnose inefficient busy waiting where appropriate.

Such diagnostics do not weaken volatile semantics.

---

## 24. Discarding the result

Discarding the result of a volatile read does not remove the physical read.

Example:

```sec
discard status.VolatileRead()
```

still performs the read.

This is required because the access itself may be the observable effect.

The same principle applies to stronger side-effecting storage operations owned
by register/device rules.

---

## 25. Volatile ordering

Volatile accesses execute in the observable order defined by Sec program
execution and the applicable storage/device contracts.

The compiler must not reorder volatile operations when doing so changes their
defined observable order.

This is a compiler semantic ordering constraint.

It does not automatically imply one particular hardware fence instruction.

---

## 26. Compiler ordering versus hardware ordering

Compiler ordering and hardware ordering are separate concepts.

Compiler ordering constrains optimization and lowering transformations.

Hardware ordering constrains what the generated machine-level execution may
allow the CPU, memory system, interconnect, or device to observe.

A target/device/storage contract may require:

```text
compiler ordering only
hardware barrier/order only
both
```

as appropriate.

The compiler emits target barriers or stronger physical operations only when the
resolved contract requires them.

---

## 27. Volatile is not a global compiler barrier

A volatile operation does not freeze all surrounding ordinary memory
optimization.

Ordinary reads and writes remain optimizable according to the Sec memory model
unless an explicit storage/device/DMA contract establishes an ordering
relationship with the volatile operation.

For example, a device doorbell may require prior descriptor writes to become
visible before the MMIO write.

That ordering comes from the device/DMA contract.

It is not an implicit property of every volatile access.

---

## 28. Volatile and atomic semantics

Volatile and atomic semantics are orthogonal.

Volatile means that the physical access itself is externally observable and
must be preserved.

Atomic semantics define indivisibility and/or synchronization according to the
Sec atomic memory model.

Volatile does not by itself provide:

```text
atomicity
acquire
release
sequential consistency
memory fence
happens-before
race safety
thread synchronization
```

Atomicity does not by itself imply volatile device/storage observation.

---

## 29. Shared-memory concurrency

Ordinary shared-memory communication between Sec tasks/threads is not made
correct by volatile access.

Shared-memory concurrency must use the synchronization and atomic mechanisms
defined by the memory-model and concurrency rulebooks.

Sec does not adopt the incorrect C-style pattern of using `volatile` as a
general thread-synchronization mechanism.

---

## 30. Interrupt communication

Hardware-register communication observed by interrupt and ordinary execution may
use volatile/MMIO semantics as appropriate.

Communication through ordinary shared RAM between interrupt and ordinary
execution is governed by interrupt, concurrency, memory-model, and atomic rules.

Ordinary RAM does not become race-safe merely by treating it as volatile.

---

## 31. Stronger register/device effects

Register/device operations may have semantics stronger than ordinary volatile
read/write.

Examples include:

```text
read-clear
write-one-clear
write-zero-clear
FIFO-advancing read
latch-on-read
command write
device-specific read/write transaction
```

This rulebook does not define the logical field/transaction model for those
operations.

It requires only that stronger effects remain explicit and are not weakened into
generic ordinary loads/stores or generic volatility.

Where an owning register/device rule requires one specific physical access,
volatile lowering preserves that access exactly.

---

## 32. Read-modify-write

Volatile does not imply that read-modify-write is safe.

A source operation may be implemented as physical read-modify-write only when
the complete register/storage/device contract permits that sequence.

If an ordinary read-modify-write would incorrectly interact with:

```text
read-only fields
write-only fields
reserved fields
read-clear fields
write-one-clear fields
other device side effects
```

the compiler uses a canonical semantics-preserving target/device operation when
one exists.

Otherwise the operation is rejected.

Detailed hardware-register transaction legality belongs to
`rules/platform/hardware-register-access.md`.

---

## 33. Physical type eligibility

Not every Sec type is eligible for direct volatile `RawPtr` access.

The compiler requires a physical representation for which a semantics-preserving
volatile access plan exists under the active `CompilationPlan`.

Read and write eligibility are separate compiler-known properties.

Conceptually:

```text
VolatileReadable[T, CompilationPlan]
VolatileWritable[T, CompilationPlan]
```

The exact compiler representation is implementation-specific.

These are not ordinary source interfaces that arbitrary user code can implement.

---

## 34. `RawPtr[void]`

`RawPtr[void]` has no value representation to materialize or consume.

Therefore:

```sec
unsafe {
    let value := pointerToVoid.VolatileRead()
}
```

is invalid.

The programmer must first select an explicit physically meaningful pointee
representation.

---

## 35. Representation validity

A direct volatile read may materialize a value of type `T` only when the
physical representation produced by the applicable access contract can
legitimately become a valid `T`.

Direct volatile access must not silently fabricate or bypass:

```text
named-type constraints
closed-enum valid-value domains
ownership
borrow validity
generation validity
destruction obligations
managed/runtime representation invariants
```

For example, if hardware can return an arbitrary integer outside a constrained
named type's valid range, code should normally read an appropriate raw
representation and perform the ordinary checked Sec conversion afterward.

---

## 36. Owning and managed types

Types whose physical materialization would invent Sec ownership, borrowing,
lifetime, generation, or managed-runtime state are not directly volatile
readable merely because their size is known.

Examples that normally require stronger representation rules include:

```text
string
list[T]
ref T
ref mut T
managed descriptors
owning resources
non-trivial lifetime-managed aggregates
```

Raw physical bits do not create ownership or valid references.

---

## 37. Aggregates

Struct, array, and aggregate volatile eligibility requires an explicit physical
layout and access contract.

It is not inferred merely from:

```text
SizeOf(T)
ordinary Sec struct layout
field count
```

A type may be physically representable but still not support one
semantics-preserving volatile physical access.

Register types are likewise subject to the stronger owning register-access rules
where their logical fields have special read/write semantics.

---

## 38. Generics

Generic code may perform direct volatile access only when volatile eligibility
is proven for the relevant type instantiation.

An unconstrained generic `T` does not implicitly satisfy volatile access
requirements.

This rulebook does not require new Sec 0.1 generic constraint syntax solely for
volatile access.

The compiler may prove eligibility after monomorphization or through another
canonical generic contract defined elsewhere.

---

## 39. Binding declarations perform no physical access

Declaring an `@address` binding does not itself:

```text
read the device
write the device
probe the address
initialize the storage
clear the register
```

The declaration binds already-existing external/platform storage.

Physical effects occur only when the program performs an applicable access.

No implicit runtime initialization write is generated merely because the
binding exists.

---

## 40. Compiler effects

Volatile operations carry explicit compiler-visible external-storage effects.

The compiler may model equivalents such as:

```text
ExternalStorageRead
ExternalStorageWrite
VolatileRead
VolatileWrite
```

Exact effect names are implementation-specific.

Volatile access does not automatically imply:

```text
allocation
blocking
panic
atomicity
synchronization
```

Those effects are added only where a stronger operation or target contract
requires them.

---

## 41. Semantic IR and lowering

Volatile semantics are explicit in compiler semantic representations.

The compiler must not lower volatile/MMIO source operations into indistinguishable
ordinary memory operations and hope that a later backend rediscovers their
meaning.

The pipeline preserves, as applicable:

```text
address domain / address space
memory space
physical access width
alignment
read/write direction
observable occurrence
side-effect class
ordering requirements
barrier requirements
byte order
memory attributes
```

Conceptually:

```text
Sema
    ↓
Semantic IR: volatile/external-storage operation + resolved contract
    ↓
Sec MLIR: preserved observable physical-access semantics
    ↓
target lowering
    ↓
target-legal physical operation
```

The exact IR encoding is owned by the IR rulebooks.

---

## 42. Target lowering

Target lowering selects concrete instructions and barriers from the already
resolved `CompilationPlan`, `MemoryEnvironment`, `DeviceModel`, and access
contract.

Lowering must not independently reinterpret source semantics.

When the target cannot materialize the required physical operation without
violating its access contract, compilation fails.

The compiler does not silently approximate a required access with a semantically
different one.

---

## 43. No Sec runtime requirement

Volatile access does not intrinsically require a Sec runtime.

A volatile read/write may lower directly to target machine instructions.

Any runtime dependency arises only from another stronger facility, not from
volatility itself.

This permits runtime-free low-level and embedded code.

---

## 44. Target/device knowledge sources

Canonical target-owned knowledge mechanisms may provide:

```text
address domains
address spaces
memory regions
device identities
MMIO locations
permissions
access widths
alignment
byte order
memory attributes
ordering requirements
availability
```

This includes future target/device knowledge-pack mechanisms.

This rulebook consumes resolved facts from such sources.

It does not define their file format, package format, versioning, distribution,
or user-facing configuration syntax.

---

## 45. LSP behavior

The LSP uses the same `CompilationPlan`, target region, memory-space, and access
contracts as command-line compilation.

It must not guess volatility or address validity from numeric-address shape.

Useful hover/tooling information may include:

```text
storage kind
resolved region
address domain / address space
volatile: yes/no
read/write permission
physical access widths
alignment
provenance
```

Analysis depth may differ from a full build, but target/storage semantics may
not.

---

## 46. Diagnostics

Volatile/MMIO diagnostics distinguish known causes.

Relevant diagnostic classes include:

```text
address not covered by a target region
wrong address space
region unavailable
read forbidden
write forbidden
unsupported access width
invalid alignment
type not volatile-readable
type not volatile-writable
representation cannot form a valid Sec value
operation requires unsafe
target cannot materialize required access
required physical ordering unavailable
illegal transformation requiring unsafe semantics
```

Diagnostics preserve target/region/device provenance where available.

---

## 47. `@address` diagnostics

When `@address` validation fails, diagnostics should explain the failed region
contract rather than only reporting an invalid numeric address.

Example:

```text
error: @address binding is not valid for this target

address:
    0x40021000

required extent:
    16 bytes

selected target:
    linux-arm64 / custom-board

reason:
    no canonical address region covers the complete binding
```

Where a region exists, diagnostics should identify the concrete failed
requirement such as width, alignment, permission, address-space, or
representation mismatch.

---

## 48. Required optimizer invariants

Optimization must preserve every required volatile occurrence and every
required ordering relation.

Subject to stronger owning contracts, optimization must not:

- remove a required volatile read or write;
- duplicate a volatile read or write;
- speculate it onto a path where it would not execute;
- merge distinct observations;
- deduplicate equal writes;
- hoist repeated observations out of loops;
- narrow, widen, split, or merge physical access when not contract-permitted;
- replace a stronger side-effecting register/device operation with weaker
  ordinary volatility;
- introduce unrelated global synchronization merely because an operation is
  volatile.

---

## 49. Required test families

A conforming Sec 0.1 implementation includes regression coverage for at least
the following.

### 49.1 Basic volatile semantics

Test:

- two reads remain two observations;
- two equal writes remain two writes;
- discarded volatile read remains;
- conditional volatile access is not speculated;
- loop volatile read is not hoisted;
- volatile value becomes an ordinary detached value.

### 49.2 `@address`

Test:

- valid target region;
- numeric address with no region rejection;
- incomplete extent rejection;
- permission rejection;
- wrong address-space rejection;
- alignment rejection;
- width rejection;
- Hosted target with no mapped region rejection;
- Hosted target with explicitly mapped GPIO/device region success;
- `unsafe` cannot waive `@address` validation.

### 49.3 RawPtr

Test:

- `VolatileRead` and `VolatileWrite` require unsafe;
- ordinary `Read` does not silently become volatile;
- `RawPtr[void]` direct volatile read/write rejection;
- known target-invalid access remains rejected inside unsafe;
- dynamic unverifiable address obligations remain programmer-owned.

### 49.4 Representation

Test:

- suitable scalar access;
- constrained type invalid representation rejection;
- closed enum invalid representation rejection;
- owning/reference type rejection;
- explicit aggregate physical-layout success where supported;
- aggregate size alone does not imply eligibility;
- generic unconstrained `T` does not imply eligibility.

### 49.5 Physical access

Test:

- exact width preservation;
- forbidden split read/write rejection;
- allowed decomposition where the target contract explicitly permits it;
- target byte-order handling;
- Harvard/address-space distinction;
- same numeric address in distinct spaces does not alias implicitly.

### 49.6 Ordering and concurrency

Test:

- volatile order preserved;
- no unnecessary global hardware fence;
- device-required barrier emitted;
- ordinary memory remains optimizable when unrelated;
- device/DMA ordering constraint respected;
- volatile does not satisfy atomic/thread synchronization.

### 49.7 Stronger register/device semantics

Test:

- stronger side-effecting read/write survives lowering;
- volatile lowering does not weaken read-clear/FIFO/etc. operations;
- unsafe read-modify-write is rejected when the known storage contract forbids
  the required sequence;
- canonical target operation is used when one exists.

### 49.8 Compiler pipeline and tooling

Test:

- Semantic IR preserves volatile/external-storage effects;
- Sec MLIR preserves physical access contract;
- target lowering rejects impossible physical operations;
- LSP and CLI agree on target region validity;
- diagnostics preserve region/device provenance.

---

## 50. Completion criteria

The Sec 0.1 volatile implementation is complete when the compiler can
deterministically:

1. identify volatile physical accesses from source/storage contracts;
2. validate every `@address` binding against a canonical target-owned region;
3. distinguish ordinary RawPtr access from explicit volatile RawPtr access;
4. enforce unsafe obligations for dynamic raw volatile access;
5. resolve physical access width, alignment, address space, byte order, and
   applicable memory attributes;
6. reject physical operations the selected target cannot implement correctly;
7. preserve volatile reads/writes as observable effects;
8. preserve required source/device ordering without inventing unrelated
   synchronization;
9. keep volatile and atomic semantics distinct;
10. preserve stronger register/device operations without redefining their
    transaction semantics;
11. enforce physical type read/write eligibility;
12. prevent physical bits from fabricating invalid Sec ownership, references, or
    constrained values;
13. preserve access semantics through Semantic IR, Sec MLIR, optimization, and
    target lowering;
14. provide consistent compiler/LSP diagnostics with target-region provenance;
15. support both runtime-free embedded use and explicitly modeled Hosted device
    mappings.

---

## 51. Non-goals for Sec 0.1

This rulebook does not require:

- a C-style volatile type qualifier;
- a general `@volatile` attribute;
- volatile as thread synchronization;
- volatile as an implicit memory fence;
- volatile as cache maintenance;
- volatile as DMA ownership/coherence;
- arbitrary Hosted process addresses for `@address`;
- `unsafe` bypass of canonical `@address` validation;
- a universal direct volatile representation for every Sec type;
- a knowledge-pack file format;
- register physical read-group semantics;
- selective logical field observation;
- a register snapshot language model;
- destructive-read grouping rules.

Those register-transaction questions are intentionally left to the owning
register/hardware-access design.
