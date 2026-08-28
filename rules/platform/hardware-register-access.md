# Hardware Register Access

- **Status:** Normative
- **Created:** 2026-08-28
- **Last updated:** 2026-08-28
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `6a63b37`
- **Canonical path:** `rules/platform/hardware-register-access.md`
- **Related rulebooks:** `rules/declarations/registers.md`, `rules/platform/volatile.md`, `rules/platform/fixed-address-bindings.md`, `rules/platform/target_profiles.md`, `rules/memory/storage.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/destruction.md`, `rules/memory/raw_pointers.md`

## 1. Purpose

### 1.1 Scope

This rulebook defines hardware-register access semantics in Sec 0.1.

It owns the semantic boundary between a logical register-field operation and the physical hardware operation or operation sequence used to realize it.

It defines:

```text
hardware register bindings
field read and write behavior against hardware
safe implicit field observation
explicit register Read() and Write() operations
shadow state
physical endpoints
hardware resource identity
endpoint aliases
physical transaction capabilities
access width and alignment
transaction footprints
coherent multi-step accesses
ordering and completion requirements
access context and authority
runtime-resolved hardware mappings
hardware-access fault behavior
register-operation planning and lowering requirements
```

### 1.2 Non-goals

This rulebook does not redefine:

```text
register[N] bit layout
bit numbering
lsb-first / msb-first allocation
register byte-order declaration syntax
bit-backed enum semantics
nested register layout
ordinary volatile semantics
ordinary ownership and borrowing
interrupt lifecycle semantics
DMA ownership protocols
atomic shared-memory semantics
device-specific command protocols
SPI/I2C/I2S/UART protocol APIs
project or knowledge-pack file formats
```

Those concerns remain owned by their canonical rulebooks or library modules.

### 1.3 Core principle

A register field describes a logical value and its field-local semantics.

A physical endpoint describes how hardware may actually be accessed.

The compiler connects the two only through a semantics-preserving register-operation plan.

The compiler must never infer physical hardware behavior merely from ordinary integer operations, field width, register width, address proximity, field names, or source spelling.

---

## 2. Relationship to register types

### 2.1 Register types remain general value types

A `register[N]` declaration remains a nominal fixed-width bit-layout type.

It does not by itself imply:

```text
MMIO
fixed address
port I/O
CSR access
volatility
runtime mapping
hardware resource ownership
physical transaction width
```

The same register type may be used as an ordinary detached value or through a hardware binding.

```sec
type Status register[8] {
    Ready: bit read-only,
    Error: bit,
    _: bit[6],
}

let snapshot: Status
```

A hardware binding adds hardware-access semantics without changing the register type's declared bit layout.

### 2.2 Hardware access does not add register storage

Hardware binding metadata, endpoint metadata, shadow metadata, operation plans, or compiler-generated helper state must not change:

```text
register width
field offsets
sizeof(register type)
alignment of the detached register value
nominal type identity
```

An `impl` block on a register type likewise does not add hardware-register storage.

---

## 3. Field access permissions

### 3.1 Default permission

A register field is `read-write` by default.

```sec
Enabled: bit
```

is equivalent in access permission to:

```sec
Enabled: bit read-write
```

### 3.2 Read-only fields

```sec
Ready: bit read-only
```

A `read-only` field may be observed when the owning binding and physical endpoint permit a legal read.

Writing a `read-only` field is a compile-time error.

### 3.3 Write-only fields

```sec
Command: bit write-only
```

A `write-only` field may be written when the owning binding and physical endpoint permit a legal write.

Reading a `write-only` field is a compile-time error.

### 3.4 Permission and behavior are separate axes

Access permission and specialized read/write behavior are semantically separate.

A modifier whose behavior requires writing cannot be combined with an effective read-only permission.

A modifier whose behavior requires reading cannot be combined with an effective write-only permission when that combination would make the declared operation impossible or contradictory.

The compiler rejects contradictory combinations rather than choosing one modifier silently.

---

## 4. Read and write field semantics

### 4.1 Ordinary read/write

An ordinary readable field observation returns the logical field value represented by the selected hardware observation.

An ordinary writable field assignment requests a logical state update.

The compiler selects a legal physical realization; source-level assignment does not imply read-modify-write.

### 4.2 Read-clear

Canonical destructive-read spelling is:

```sec
Event: bit read-clear
```

A read that observes `Event` also performs the hardware-defined clear side effect.

The compiler must treat the observation as effectful.

It must not remove, duplicate, speculate, merge, or reorder the observation in a way that changes the defined effect.

### 4.3 Read-set

Where declared by the register/device contract:

```sec
Flag: bit read-set
```

observing the field performs the hardware-defined set side effect.

`read-set` follows the same effect-preservation rules as `read-clear`.

### 4.4 Write-one-clear

```sec
Pending: bit write-one-clear
```

Writing logical one requests the field's clear operation according to the hardware contract.

The compiler must not reinterpret this as an ordinary stored-one update.

### 4.5 Write-one-set

```sec
Enabled: bit write-one-set
```

Writing logical one requests the field's set operation according to the hardware contract.

### 4.6 Write-one-toggle

```sec
Output: bit write-one-toggle
```

Writing logical one requests the field's toggle operation according to the hardware contract.

### 4.7 Write-zero-clear

```sec
Error: bit write-zero-clear
```

Writing logical zero requests the field's clear operation according to the hardware contract.

### 4.8 Write-zero-set

```sec
Enabled: bit write-zero-set
```

Writing logical zero requests the field's set operation according to the hardware contract.

### 4.9 Write-zero-toggle

```sec
Output: bit write-zero-toggle
```

Writing logical zero requests the field's toggle operation according to the hardware contract.

### 4.10 Stronger device effects

A device contract may describe stronger effects that do not reduce to the modifier family above, including:

```text
FIFO advancement
latching
command acceptance
external signaling
state-machine transitions
read projection differences
write projection differences
```

Such effects remain compiler-known hardware facts when they describe one logical register operation.

They do not authorize the compiler to invent a multi-register device protocol.

---

## 5. Hardware operation versus device protocol

### 5.1 Compiler-owned hardware realization

The compiler may own a multi-step sequence when the sequence is the hardware-defined physical realization of one logical register operation.

Example:

```text
read low word
hardware latches high word
read latched high word
combine into one coherent value
```

Source may remain:

```sec
let snapshot := counter.Read()
```

### 5.2 Device protocol remains ordinary code

A sequence such as:

```text
write unlock key
write configuration within four cycles
poll ready
read result
acknowledge completion
```

is device protocol.

It belongs in ordinary Sec driver/library code or generated device support.

It must not be hidden inside a field modifier merely because the device datasheet describes it near a register.

### 5.3 Bus-level device addressing is protocol

A device register index behind SPI, I2C, I2S, UART, or another external bus is not a CPU hardware address merely because documentation calls it a register address.

Such addressing remains bus/device protocol handled by the applicable library or driver infrastructure.

Register types may still be reused to describe the transferred register representation.

---

## 6. Logical field access and safe implicit observation

### 6.1 Field access requests a logical observation

For a hardware-bound register:

```sec
if status.Ready {
}
```

requests observation of `Ready`.

It does not request a one-bit physical bus read.

Field width never implies physical access width.

### 6.2 Safe implicit read rule

The compiler may implicitly perform the required physical register observation only when it can prove that the chosen physical operation plan introduces no unintended collateral semantic effect.

This is the canonical Sec 0.1 rule for implicit hardware field reads.

### 6.3 Collateral destructive effect

Consider:

```sec
type Status register[8] {
    Ready: bit read-only,
    Count: bit[3] read-clear,
    _: bit[4],
}
```

If the only hardware operation capable of observing `Ready` is one eight-bit read that also observes and clears `Count`, this source is invalid:

```sec
if status.Ready {
}
```

The compiler must diagnose that observing `Ready` implicitly would also perform the destructive `Count` read effect.

### 6.4 Selective hardware capability

If the resolved target/device contract supplies a semantics-preserving operation that observes only the requested field or an equivalent safe projection, implicit access may use it.

Examples include target-defined selective bit operations, safe read aliases, instruction-encoded selected operations, or another explicitly modeled mechanism.

Sec does not assume that such an operation exists.

### 6.5 No field-level explicit physical `Read()`

Sec does not define a general field-level operation such as:

```sec
status.Ready.Read()
```

Explicit hardware reads operate at register/endpoint granularity.

This avoids implying a physical field-read capability that the hardware may not possess.

---

## 7. Explicit register `Read()`

### 7.1 Meaning

For a hardware-bound register, compiler-known:

```sec
let snapshot := status.Read()
```

requests one logical register observation.

The compiler selects a legal physical operation or operation sequence that satisfies the complete endpoint contract.

### 7.2 Detached snapshot

`Read()` returns a detached value of the register type when the register has a representable readable projection.

Subsequent field access on the detached value has ordinary value semantics and performs no additional hardware access.

```sec
let snapshot := status.Read()

if snapshot.Ready {
}

let mode := snapshot.Mode
```

### 7.3 Field restrictions remain in force

Returning a detached register value does not make `write-only` fields source-readable.

The nominal register field restrictions remain part of the type semantics.

### 7.4 Destructive explicit read

An explicit `Read()` authorizes the read effects that belong to the selected logical register observation.

The compiler must still reject a physical plan that introduces effects outside the operation contract.

Explicitness does not authorize arbitrary unrelated device behavior.

### 7.5 Read result coherence

`Read()` may return a snapshot only when the selected plan establishes a valid logical observation of the returned projection.

A sequence that can combine values from incompatible moments without a defined coherence contract must not be presented as one coherent snapshot.

---

## 8. Explicit register `Write()`

### 8.1 Shadow commit form

For a hardware binding with writable shadow state:

```sec
control.Write()
```

commits pending shadowed logical write intents through a legal register-operation plan.

It does not dump a cached raw register image.

### 8.2 No pending write

Calling `Write()` when the applicable shadow state contains no pending write intent performs no physical write.

The compiler may diagnose the operation as redundant according to normal diagnostic policy, but the semantic result is a no-op.

### 8.3 Whole-value write form

Where the endpoint and register semantics permit a whole-register write:

```sec
control.Write(value)
```

requests an explicit whole-register write of the detached register value.

This form is legal only when every physically affected field and reserved position has a well-defined semantics-preserving write plan.

### 8.4 Mixed-semantics rejection

A whole-value write may be rejected for registers containing semantics that cannot safely be represented by one whole-register value write, including incompatible combinations of:

```text
write-one-clear fields
write-zero-clear fields
write-only command fields
read-only fields requiring preservation
reserved-bit preservation
trigger-like device effects
hardware-owned fields
```

The compiler must reject the operation rather than silently reinterpret it as a different sequence.

---

## 9. Shadow semantics

### 9.1 Purpose

`shadow` requests compiler-managed software state associated with hardware register fields.

Shadow state allows software to retain an observed or intended logical value without requiring every ordinary field access to perform hardware I/O.

Shadow does not make the hardware itself non-volatile.

### 9.2 Default

The global default is `noshadow`.

Without a shadow declaration, ordinary hardware-bound field access follows live hardware-access semantics.

### 9.3 Field-level shadow

A field may be shadowed explicitly:

```sec
type Configuration register[8] {
    Mode: DeviceMode shadow,
    Ready: bit read-only,
    _: bit[5],
}
```

`Mode` uses shadow semantics.

`Ready` remains live.

### 9.4 Register-level shadow default

A register may establish `shadow` as the local default:

```sec
type Configuration register[16] shadow {
    Enabled: bit,
    Mode: DeviceMode,
    CurrentInput: bit read-only noshadow,
    _: bit[12],
}
```

The inheritance rule is:

```text
field shadow/noshadow
    explicit field override

otherwise register shadow/noshadow
    register-local default

otherwise
    noshadow
```

### 9.5 Shadow does not change access permission

These concerns are orthogonal.

A `write-only shadow` field remains unreadable through ordinary field access.

A `read-only shadow` field remains non-writable.

Shadow is not a backdoor around field permissions.

### 9.6 Shadow does not assert current hardware truth

A shadowed value represents software-held knowledge or intended state.

It does not assert that external hardware still has the same state at the current instant.

Hardware may change independently after an observation.

### 9.7 Shadow state model

The semantic shadow model separates observed knowledge from pending write intent.

Conceptually:

```text
ObservedState:
    Unknown
    Known(value)

PendingWrite:
    none
    one or more logical write intents
```

A previous `Clean/Dirty` terminology must not be interpreted as equality with current hardware.

### 9.8 No fabricated initialization

Shadow state must not fabricate an initial zero or default register value.

A shadow field may be read as a shadowed value only when its required observed/intended state is known on the executing path.

The compiler performs definite-shadow-state analysis where possible.

When path-dependent state must exist at runtime, the compiler may materialize bounded ordinary shadow metadata such as a known mask. This is compiler-managed storage, not a runtime service.

### 9.9 Read updates shadow

An explicit or safe implicit physical observation updates every shadowed field whose value is established by that coherent observation.

A single physical transaction may therefore update multiple shadowed fields.

### 9.10 `noshadow` fields remain live

If a physical read performed for a shadowed field also returns bits belonging to a `noshadow` field, the `noshadow` field does not become persistently shadowed merely because its bits were physically available.

A later live access to that field performs a new hardware observation when required and when safe.

### 9.11 Destructive read with shadow

A field may combine destructive read semantics and shadow:

```sec
Events: bit[4] read-clear shadow
```

A legal explicit observation may return the pre-clear value and preserve that observed value in shadow even though hardware has already changed as a consequence of the read.

`Known(value)` therefore means the observation is known, not that the current hardware state equals `value`.

### 9.12 Write-only shadow

A write-only field may be shadowed so software can retain write intent or last-established software state needed to construct later legal operations.

Ordinary source read of the write-only field remains invalid.

### 9.13 Pending writes are operations, not cached bits

For specialized write semantics, shadow state records logical write intent rather than blindly storing the bit pattern later to be re-emitted.

For example, a pending `write-one-clear` intent represents a clear operation. It must not later be treated as an instruction to store logical one into ordinary register state.

### 9.14 Shadow update after write

After a hardware write, the compiler updates or invalidates observed shadow knowledge according to the operation's guaranteed postcondition.

If the operation proves the resulting logical value, the shadow may become `Known(result)`.

If the resulting hardware value cannot be established, the corresponding observed state becomes `Unknown` while unrelated known state may remain known.

### 9.15 No automatic refresh

The compiler must not spontaneously refresh shadow state merely because it suspects the value may be stale.

Every hardware observation must arise from source semantics or a compiler-known library/platform operation whose contract requests that observation.

### 9.16 No automatic write-back

Pending shadow state must not be written automatically at scope exit, destruction, arbitrary optimization points, or background runtime intervals.

Hardware write-back occurs only through a defined semantic operation such as `Write()` or another explicit compiler-known hardware API.

### 9.17 No runtime manager

Shadow support must not require:

```text
garbage collection
a global shadow registry
a background refresh service
a background write-back service
runtime ownership tracking
runtime borrow tracking
```

The compiler may use ordinary stack, static, owned-object, or other explicitly valid storage when persistent shadow state is actually required.

---

## 10. Shadow granularity

### 10.1 Declaration granularity

Shadow may be declared at field granularity or as a register-level default.

### 10.2 Semantic granularity

Observed knowledge and pending write intent are tracked at the finest semantic granularity required to preserve field behavior.

### 10.3 Storage granularity

The compiler may combine multiple shadowed fields into one physical software storage object when doing so preserves semantics.

It may use raw register-shaped storage plus masks, separate typed values, SSA state, or another equivalent representation.

### 10.4 Shadow groups

The compiler may form internal shadow groups that follow coherent hardware observation or write relationships.

Shadow groups are not source syntax.

A group may contain:

```text
fields
software storage
known-state information
pending logical write intents
read plan
write plan
```

### 10.5 Coherent group update

Multiple fields may become known together only when one coherent physical observation plan actually establishes them together.

The compiler must not fabricate a coherent snapshot by combining unrelated reads performed at different times unless the endpoint contract defines that sequence as one coherent logical observation.

---

## 11. Physical endpoints

### 11.1 Endpoint concept

A hardware register binding resolves to one or more physical endpoints.

An endpoint is an access mechanism, not necessarily ordinary memory storage.

### 11.2 Endpoint kinds

The canonical model must be able to represent at least:

```text
MemoryMapped
PortMapped
InstructionEncoded
RuntimeMapped
TargetDefined
```

These are semantic categories, not required source keywords.

### 11.3 Memory-mapped endpoint

A memory-mapped endpoint is accessed through the applicable target memory/device address domain.

It may be bound statically with `@address(...)` when all fixed-address requirements are satisfied.

### 11.4 Port-mapped endpoint

A port-mapped endpoint uses a distinct target I/O address space or target operation family.

The compiler must not reinterpret a port number as an MMIO address merely because both are represented numerically.

### 11.5 Instruction-encoded endpoint

An instruction-encoded endpoint is selected through target instructions or instruction operands rather than ordinary memory addressing.

Examples include architecture-defined control/system-register spaces.

Such an endpoint need not use `@address` source syntax.

### 11.6 Runtime-mapped endpoint

A runtime-mapped endpoint has a statically known semantic contract while its concrete CPU-visible location is acquired at runtime.

Runtime mapping does not reduce the endpoint to an untyped integer address.

### 11.7 Endpoint operations

An endpoint exposes an explicit set of physical operations.

Conceptually:

```text
PhysicalEndpoint {
    Identity
    Location
    ContextRequirements
    Operations[]
}
```

One endpoint may support multiple operations, such as read, replacement write, set-bits, clear-bits, toggle-bits, or another target-defined operation.

---

## 12. Hardware resource identity and aliases

### 12.1 Resource identity

Hardware state has a canonical `HardwareResourceIdentity` independent of any one endpoint location.

A peripheral may contain multiple distinct resources, for example:

```text
GPIO.OutputState
GPIO.DirectionState
GPIO.InterruptState
```

### 12.2 Endpoint identity

An endpoint has its own identity even when it operates on the same hardware resource as another endpoint.

### 12.3 Operation identity

Distinct physical operations may target the same endpoint or different endpoints while affecting the same hardware resource.

The compiler keeps hardware resource identity, endpoint identity, and operation identity separate.

### 12.4 Alias through shared hardware state

Alias semantics are defined by shared hardware resource effects, not by address equality.

Example:

```text
OUT       -> replace/read GPIO.OutputState
OUT_SET   -> set selected bits in GPIO.OutputState
OUT_CLEAR -> clear selected bits in GPIO.OutputState
OUT_XOR   -> toggle selected bits in GPIO.OutputState
```

The alias endpoints do not represent four independent stored values.

### 12.5 No address-based alias inference

The compiler must not infer semantic alias relationships merely because addresses are equal, adjacent, similarly named, or use a common base.

Alias facts come from canonical target/device/project knowledge.

### 12.6 Different address may mean same state

Two different endpoint locations may intentionally affect the same resource.

### 12.7 Same numeric address may mean different resources

Equal numeric values in different address domains, address spaces, privilege/security views, execution units, or target contexts do not imply one resource identity.

### 12.8 Read aliases

Two read endpoints may observe the same logical hardware state with different effects.

For example:

```text
STATUS      -> observe + clear events
STATUS_PEEK -> observe without clearing
```

The planner may select the safe alias for an implicit field observation when that alias satisfies the requested semantics.

### 12.9 Write aliases

Set/clear/toggle or similar aliases may provide a semantics-preserving field update that avoids an unsafe read-modify-write.

The planner should prefer a legal stronger operation when required to preserve side effects, reserved bits, or concurrency guarantees.

---

## 13. Requested operations and planning

### 13.1 Logical operation

Source register access is first represented as a logical requested operation.

Examples include:

```text
ObserveField(Ready)
ReadRegister(Status)
SetField(Enabled, true)
ClearField(Pending)
CommitShadow(Control)
WriteRegister(Control, value)
```

### 13.2 Candidate plans

The register-operation planner enumerates physical endpoint operations capable of realizing the requested operation.

### 13.3 Validation

Each candidate is validated against the complete resolved hardware contract before lowering.

At minimum the planner considers:

```text
resource identity
endpoint identity
field permission
field read/write semantics
physical access direction
width
alignment
read footprint
write footprint
collateral effects
reserved-bit requirements
coherence
atomicity
failure atomicity
ordering
completion
access context
fault model
```

### 13.4 Deterministic selection

When more than one legal plan exists, target policy selects one deterministically according to canonical lowering rules.

The selection must not change source semantics.

### 13.5 No legal plan

If no semantics-preserving physical operation plan exists, compilation fails.

The compiler must not emit an approximate operation merely because ordinary integer manipulation could produce the requested final bits in an idealized memory model.

---

## 14. Access width

### 14.1 Separate widths

The compiler keeps separate:

```text
FieldWidth
RegisterWidth
PhysicalAccessWidth
```

None implies either of the others.

### 14.2 Allowed read and write widths

A physical endpoint contract declares the access widths legal for reads and writes.

Read and write width sets may differ.

Conceptually:

```text
ReadWidths  = { 32 }
WriteWidths = { 8, 16, 32 }
```

### 14.3 CPU capability is insufficient

A CPU instruction capable of reading or writing a particular width does not prove that the target peripheral endpoint accepts that width.

Both target instruction capability and endpoint access permission must be satisfied.

### 14.4 No arbitrary narrowing or widening

The compiler must not narrow or widen a hardware access merely because the selected field occupies fewer or more bits.

### 14.5 No arbitrary splitting or merging

The compiler must not split one required access into smaller accesses or merge multiple accesses into one larger access unless the endpoint contract explicitly permits the transformation as semantics-preserving.

---

## 15. Alignment

### 15.1 Endpoint alignment

Required alignment is part of the endpoint access contract.

It is a correctness requirement where the hardware contract defines it as such, not merely an optimization hint.

### 15.2 Static validation

Statically known misalignment is a compile-time error.

### 15.3 Runtime-resolved validation

When alignment depends on a runtime mapping or runtime offset, the checked mapping/view operation validates the requirement before producing a safe register view.

### 15.4 Unsafe does not waive known alignment rules

`unsafe` does not make a hardware endpoint accept a transfer that its resolved contract forbids.

---

## 16. Transaction footprints

### 16.1 Read footprint

A read operation has a physical read footprint describing which logical hardware state is physically observed or affected by the operation.

The result projection may be narrower than that footprint.

### 16.2 Write footprint

A write operation has a physical write footprint describing which hardware state positions may be affected by the operation.

The requested logical field update may be narrower than that footprint.

### 16.3 Collateral-effect analysis

Collateral read and write effects are evaluated over the physical transaction footprint, not only over the requested field.

### 16.4 Byte strobes and masks

Byte strobes, write masks, selected-bit instructions, and equivalent hardware capabilities affect the physical write footprint only when explicitly supplied by the endpoint contract.

The compiler must not invent selective write capability from source field width.

---

## 17. Multi-step access and coherence

### 17.1 Logical operation versus physical steps

One logical Sec register operation may require multiple target instructions or hardware transactions.

The number of machine instructions or bus beats does not define the source semantic operation count.

### 17.2 Defined multi-step plan

A multi-step plan is legal only when the target/device contract defines the steps as the realization of the requested logical operation.

### 17.3 Coherent observation

A multi-step read may produce one logical snapshot only when its coherence contract guarantees that the combined value represents one valid logical observation.

### 17.4 Latch-based reads

A hardware-defined latch sequence is a canonical example of a compiler-owned coherent multi-step read.

### 17.5 Retry algorithms

A target-defined bounded retry algorithm may be represented only when the target contract defines it as the canonical observation mechanism and its effects, termination conditions, and failure behavior are fully specified.

Arbitrary driver polling or protocol control flow remains ordinary Sec code.

---

## 18. Atomicity

### 18.1 Width does not imply atomicity

Physical access width does not by itself define atomicity relative to CPUs, cores, DMA, peripherals, or other bus masters.

### 18.2 Operation atomicity

Atomicity properties belong to the resolved physical operation contract.

### 18.3 Alias operations

A set/clear/toggle alias may provide stronger update atomicity than ordinary read-modify-write.

The planner must preserve such semantics when required by the requested operation or resource contract.

### 18.4 Shared-memory atomics remain separate

Hardware-register atomicity does not automatically create Sec shared-memory atomic or synchronization semantics.

The concurrency/atomic rulebooks remain canonical for ordinary shared memory.

---

## 19. Ordering

### 19.1 Source semantic order

Hardware operations execute in the observable order defined by Sec program execution and the applicable hardware contracts.

The compiler must not reorder operations when doing so changes that defined order.

### 19.2 Volatile is not a hardware barrier

Volatile preserves required physical access occurrence and compiler-visible ordering.

Volatile does not by itself guarantee visibility ordering across CPUs, interconnects, devices, DMA engines, or other observers.

### 19.3 Ordering relation

A Sec hardware ordering requirement represents a relationship between effects, not a specific ISA instruction.

Conceptually:

```text
Order(A, B)
```

means that the target must realize the required visibility/order relationship between operation A and operation B.

### 19.4 Effect classes

Ordering analysis distinguishes at least:

```text
ordinary memory read
ordinary memory write
device read
device write
hardware barrier/order effect
```

Additional target-defined classes may exist.

### 19.5 Scope

An ordering requirement may also have a target-resolved scope, such as one execution unit, one shareability domain, one device/resource domain, or system-wide.

Exact target scope naming is not source language syntax.

### 19.6 Compiler materialization

The compiler emits the minimum target mechanism that satisfies an already-established semantic ordering requirement.

The mechanism may be a barrier instruction, stronger access operation, read-back, target intrinsic, or no extra instruction where the target contract already guarantees the relation.

### 19.7 No invented protocol ordering

The compiler must not infer that two unrelated source operations form a device protocol and then invent ordering or polling merely because their names, addresses, or proximity appear related.

---

## 20. Completion

### 20.1 Ordering and completion are distinct

Ordering constrains relative visibility/order.

Completion requires an operation to have reached the completion point defined by its hardware contract before a later semantic event proceeds.

Conceptually:

```text
Order(A, B)
Complete(A) before B
```

are distinct requirements.

### 20.2 Posted writes

A physical write may be posted or buffered according to the target/device contract.

A locally retired store instruction does not necessarily imply device completion.

### 20.3 Completion mechanism

The target may satisfy completion through a synchronization instruction, a defined read-back, a stronger endpoint operation, or another contract-defined mechanism.

### 20.4 Protocol boundary

A simple read-back may be compiler-owned when it is the target-defined completion mechanism for the write.

Polling a device until a status transition occurs remains device protocol unless the endpoint contract explicitly defines that polling as part of one canonical hardware operation.

### 20.5 ISR relevance

The interrupt rulebook may require completion of particular device operations before handler return or interrupt re-enablement.

This rulebook defines what completion means and how the target may realize it; the interrupt rulebook defines when that relationship is required.

---

## 21. External events

### 21.1 Memory ordering is not event ordering

A memory/device ordering guarantee does not automatically define when an external event such as an interrupt assertion, DMA start, or device-side state-machine transition becomes observable.

### 21.2 External event effects

Physical operation contracts may therefore describe external event effects separately from memory and device-state effects.

### 21.3 No polarity inference

Signal polarity and domain meaning are outside compiler register semantics.

The compiler must not infer active-high/active-low, asserted/deasserted, enabled/disabled, reset polarity, chip-select polarity, interrupt polarity, or similar logical meaning from field or signal names.

Driver/application code owns that interpretation.

---

## 22. Static `@address` bindings

### 22.1 Role

`@address(...)` is the static hardware binding mechanism for a location that can be fully resolved by the active `CompilationPlan` before execution.

### 22.2 One argument

`@address` takes exactly one address/endpoint expression.

Width, alignment, access class, permissions, ordering, aliases, and other hardware facts are not supplied as extra positional arguments.

They come from canonical endpoint/storage contracts.

### 22.3 Named endpoint preferred

A compiler-known named endpoint is the preferred form:

```sec
@address(Platform.GPIOA)
let mut gpio: GpioRegisters
```

A project-defined canonical endpoint may be used similarly:

```sec
@address(Project.FpgaControl)
let mut control: FpgaControlRegisters
```

### 22.4 Named endpoint is not an integer constant

A named endpoint resolves to a canonical endpoint descriptor containing the semantic facts required by the planner.

It must not be reduced to an ordinary integer constant before those facts are consumed.

### 22.5 Numeric address form

A compile-time numeric address may be used only in the target's canonical linear hardware address domain and only when the complete binding resolves unambiguously to a canonical valid region/storage contract.

```sec
@address(0x40021000)
let mut control: ControlRegisters
```

### 22.6 Ambiguous or non-linear domains

A numeric literal does not identify an arbitrary port-I/O, CSR, system-register, banked, or other non-canonical address domain.

Such endpoints require compiler-known named endpoint mechanisms or another target-defined binding API.

### 22.7 Validation

Every `@address` binding is validated for its complete extent and applicable contract, including:

```text
address domain
address space
region extent
availability
permissions
alignment
layout/representation
legal access widths
storage/device compatibility
```

### 22.8 Unsafe does not waive validation

`unsafe` cannot turn an otherwise unverifiable or invalid `@address` declaration into a valid binding.

Dynamic or unverified address access must use a checked runtime hardware mapping mechanism or the applicable RawPtr/unsafe path.

---

## 23. Runtime-resolved hardware mappings

### 23.1 Purpose

Some hardware resources have statically known identity and access semantics while their concrete CPU-visible location is resolved only at runtime.

PCI/PCIe BAR mappings and operating-system mediated device mappings are canonical examples.

### 23.2 Compiler-known resource type

A runtime-resolved mapping is represented by a compiler-known move-only resource abstraction, conceptually:

```text
MappedHardwareRegion[Contract]
```

The exact platform constructor may vary, but the semantic type must preserve the mapping contract and ownership.

### 23.3 Not a raw integer

A runtime mapping is not represented to safe Sec merely as `uint` or arbitrary `RawPtr`.

Its runtime representation may contain values such as:

```text
mapped base
mapped extent
platform handle/cookie
```

while endpoint identities, aliases, register layouts, and transaction contracts remain compile-time metadata.

### 23.4 Ownership

The mapping resource owns the mapping lifetime when the selected platform API creates an owning mapping.

It is move-only unless a stronger canonical resource contract explicitly defines safe duplication.

### 23.5 Destruction

Destroying an owning mapping performs the platform-defined release/unmap action where required.

No hardware register write, reset, or shadow flush is implied by ordinary mapping destruction.

### 23.6 No hidden runtime manager

Runtime mapping support must not require a Sec global mapping registry, garbage collector, runtime ownership table, or background service.

### 23.7 Register views

Typed register views are derived from a valid mapping, conceptually:

```sec
let control := try mapping.Registers[ControlRegisters](0x100)
```

The public platform API may provide more specific generated properties or methods.

### 23.8 Register-view lifetime

A register view does not own the mapping.

Its lifetime is bounded by the mapping or other canonical owner that establishes the endpoint validity.

The compiler rejects a view that escapes beyond the mapping lifetime.

### 23.9 Checked construction

When bounds, extent, alignment, or authority depend on runtime values, creation of a safe register view is checked and may return `Result`.

Once a checked view has been established, ordinary hardware access through that view does not repeat those structural checks unless the contract requires them.

### 23.10 Runtime base remains late-bound

Register operation planning may retain symbolic endpoint offsets and resource identity until lowering/execution.

The concrete physical location may be formed late as:

```text
runtime mapping base + statically known endpoint offset
```

without losing the typed hardware contract.

---

## 24. Binding identity and lifetime

### 24.1 Mapping identity and resource identity differ

The compiler distinguishes:

```text
MappingIdentity
HardwareResourceIdentity
EndpointIdentity
```

These identities need not each exist as runtime integers.

### 24.2 One owning claim

A platform API may define a hardware resource as exclusively claimable.

Sec ownership can then make an owning mapping or device handle move-only and prevent duplicate safe owning claims.

### 24.3 Multiple borrowed views

One mapping may expose multiple borrowed typed views when their aliasing, mutability, and hardware contracts permit them.

### 24.4 Hardware mutation is not a Sec ownership conflict

External hardware may modify volatile state while a Sec value exclusively owns the software mapping capability.

Ownership controls Sec-side lifetime and authority; it does not imply that hardware state is immutable or exclusively modified by the CPU.

### 24.5 Remapping

A remap that changes the concrete physical realization must end or invalidate the old mapping before new views are established unless the platform contract proves existing views remain valid.

The compiler must not silently mutate the base location behind live views when doing so would invalidate their semantic identity or address stability.

---

## 25. Access context

### 25.1 Context-sensitive legality

A physical endpoint operation is legal only when its access requirements are satisfied by the current execution context.

Existence of the endpoint does not imply current authority to use it.

### 25.2 Context dimensions

The canonical model can represent, as applicable:

```text
privilege
security domain
execution unit/core/hart
execution mode
virtualization context
acquired authority/capability
```

Targets that do not use a dimension need not materialize it.

### 25.3 Target-defined privilege

Sec does not define one universal numeric privilege hierarchy.

Target/platform models provide canonical privilege facts and implication relationships.

### 25.4 Security domain

Security domain is independent of privilege.

A privileged operation in one security domain does not automatically satisfy an endpoint restricted to another domain.

### 25.5 Execution unit

An endpoint may be global, current-execution-unit relative, or bound to a specific execution unit according to target semantics.

### 25.6 Operation-specific requirements

Read and write operations on the same endpoint may have different access requirements.

Different aliases or operation forms targeting the same hardware resource may also have different requirements.

### 25.7 Runtime-acquired authority

A checked runtime mapping or platform service may establish authority to use an endpoint without changing the Sec function into a more privileged execution mode.

The resulting capability is tied to the owning mapping/resource contract.

### 25.8 RawPtr does not grant authority

Constructing or possessing a RawPtr does not establish hardware privilege, security-domain access, I/O permission, mapping authority, or resource ownership.

### 25.9 Unsafe does not elevate context

`unsafe` does not:

```text
raise privilege
enter a security domain
grant I/O permission
create a mapping
change execution unit
satisfy unavailable endpoint requirements
```

### 25.10 Compile-time rejection

When context mismatch is statically known, the compiler rejects the hardware operation before lowering.

### 25.11 Explicit authority transitions

System calls, secure gateways, supervisor services, firmware calls, or similar authority transitions are explicit platform/library operations.

The compiler must not invent such a transition merely to make an otherwise illegal register access succeed.

---

## 26. Fault semantics

### 26.1 Illegal and fault-capable are distinct

Sec distinguishes:

```text
statically illegal operation
architecturally permitted but fault-capable operation
ordinary fallible resource acquisition
ordinary device-reported error state
```

### 26.2 Statically known violation

Known violations of field semantics, endpoint width, alignment, context, extent, permissions, or other canonical hardware contracts are compile-time errors.

The compiler must not intentionally emit a known-invalid operation and rely on hardware to fault.

### 26.3 Legal operation may still fault

A semantics-valid hardware access may still be subject to target/environment faults such as bus faults, access faults, page faults, device removal, protection changes, or virtualization traps.

Safe Sec hardware access does not promise that external hardware and platform state can never fault.

### 26.4 Fault is not automatic `Result`

A hardware/architecture fault does not automatically change ordinary `Read()`, `Write()`, or field access into `Result`-returning source operations.

Fault delivery follows the applicable target execution/exception model.

### 26.5 Checked acquisition is ordinary failure

Operations that explicitly acquire or validate a mapping/resource may return `Result` because resource absence, permission denial, invalid extent, or mapping failure are normal recoverable operation outcomes.

### 26.6 Explicit recoverable access API

A platform may expose an explicit fallible hardware access API when it can trap/catch/recover from access failure and return normal Sec control flow.

Such an API is distinct from ordinary hardware access semantics.

### 26.7 Device-reported state

A device status bit or returned status code is ordinary device state until driver/library logic interprets it as a typed Sec error.

The compiler must not globally interpret a particular bit pattern such as all-ones as hardware failure without a specific device/platform contract.

### 26.8 Device liveness and mapping lifetime

A mapping may remain owned while the external device becomes unavailable.

Mapping lifetime and device liveness are separate facts.

Device liveness must not be overloaded onto Sec ownership `is available` semantics.

---

## 27. Failure atomicity

### 27.1 Partial physical effects

A multi-step logical operation may fail after one or more earlier physical steps have already produced observable effects.

### 27.2 No implicit rollback

The compiler must not invent rollback for hardware effects unless the endpoint contract explicitly defines a correct inverse/recovery operation as part of the logical access semantics.

### 27.3 Plan contract

A compiler-owned multi-step access plan must define which effects may have occurred if the plan faults or otherwise fails before completion.

### 27.4 Shadow after failed observation

A failed logical observation does not establish coherent shadow knowledge beyond facts explicitly guaranteed by the operation's failure contract.

The compiler must not mark a complete shadow group known after only a partial observation.

### 27.5 Shadow after partial write

When an explicitly recoverable multi-step commit reports failure after partial completion, pending shadow intent must reflect which operations are known committed and which remain uncommitted according to the failure contract.

For non-recoverable architecture faults, normal Sec control flow does not continue unless the target exception model explicitly provides such recovery.

---

## 28. Volatile integration

### 28.1 Hardware accesses remain volatile where the binding contract says so

`@address` hardware bindings and runtime mappings whose contracts define externally observable hardware access use volatile physical access semantics.

### 28.2 Volatile owns occurrence, this rulebook owns transaction meaning

Volatile preserves required physical access occurrence and ordering constraints.

This rulebook determines which logical register fields and hardware state are observed or affected by one physical access plan.

### 28.3 Stronger operation wins

A register/device operation with semantics stronger than generic volatile read/write remains a distinct operation through Semantic IR and lowering.

It must not be weakened to ordinary volatility.

### 28.4 Read-modify-write

Volatile read-modify-write is legal only when the complete hardware-register operation planner proves that the sequence preserves all field, endpoint, footprint, side-effect, ordering, and access-context requirements.

---

## 29. Raw pointers and low-level escape

### 29.1 RawPtr remains separate

`RawPtr[T]` is an unchecked address value and does not automatically become a typed hardware binding.

### 29.2 Missing facts

A RawPtr does not inherently carry:

```text
HardwareResourceIdentity
EndpointIdentity
alias relationships
legal register transaction plans
access authority
mapping ownership
shadow state
```

### 29.3 Unsafe raw hardware access

Unsafe raw access may be appropriate when the programmer explicitly assumes obligations the compiler cannot prove.

Unsafe does not override hardware facts the compiler already knows to be invalid.

### 29.4 Promotion to checked hardware binding

A platform/compiler-known validation operation may convert or wrap raw/dynamic information into a checked mapping or register view only after establishing the required canonical facts.

---

## 30. Effects and Semantic IR

### 30.1 Explicit semantic operations

Semantic IR must represent hardware operations explicitly before target lowering.

It must not infer hardware semantics from generic integer load/store patterns in LLVM.

### 30.2 Required operation information

Semantic IR or an equivalent verified lowering plan must preserve, where applicable:

```text
requested logical register operation
register/field identity
hardware resource identity
endpoint identity
selected physical operation or unresolved candidate set
read/write projection
transaction footprint
field side effects
external event effects
shadow effects
ordering edges
completion edges
atomicity
failure atomicity
access context requirements
fault model
source location and provenance
```

### 30.3 Ordering edges

Ordering and completion requirements remain explicit edges or equivalent verified relationships until target lowering has selected the mechanism that satisfies them.

### 30.4 LLVM boundary

LLVM IR must not decide whether a register field update is W1C, safe RMW, alias write, destructive read, shadow commit, or another Sec hardware operation.

Those decisions must already be resolved or represented by explicit target intrinsics before ordinary LLVM optimization can erase the semantic distinction.

---

## 31. Target lowering

### 31.1 Target authority

Target lowering consumes the frozen `CompilationPlan`, resolved target/device models, endpoint contracts, and register-operation plans.

### 31.2 Possible realizations

A valid operation may lower to:

```text
volatile MMIO load/store
port-I/O instruction
CSR/system-register instruction
set/clear/toggle alias write
selected-bit target instruction
multi-step latch sequence
barrier/synchronization instruction
read-back completion sequence
runtime mapping base + static endpoint offset
target-specific intrinsic
```

### 31.3 No semantic approximation

If the selected target cannot materialize the required operation semantics, compilation fails.

### 31.4 Host independence

Cross-compilation uses target facts, not compiler-host address spaces, endianness, privilege, bus behavior, or alignment defaults.

---

## 32. Knowledge sources

### 32.1 Canonical facts

Target, platform, device, project, and future knowledge-pack mechanisms may provide canonical hardware facts including:

```text
resource identity
endpoint identity
address domain/address space
location or selector
extent
aliases
read/write operations
access widths
alignment
byte order
transaction footprints
field projections
side effects
atomicity
coherence
ordering/completion
access-context requirements
fault behavior
runtime mapping contracts
```

### 32.2 No name-based inference

The compiler must not infer hardware semantics from names such as:

```text
STATUS
RESET_N
IRQ_N
SET
CLEAR
TOGGLE
READY
COMMAND
```

Names aid diagnostics and tooling only unless canonical metadata assigns semantics.

### 32.3 Project-defined endpoints

A project may supply canonical named endpoints for custom boards, FPGA blocks, or other known hardware through project/platform configuration.

The exact configuration syntax belongs to project/platform rulebooks.

### 32.4 Provenance

Resolved hardware facts retain provenance so diagnostics and fingerprints can explain whether a fact came from target, platform, device, project, generated knowledge, or runtime-acquired authority.

---

## 33. Tooling and diagnostics

### 33.1 Diagnostics

Hardware-register diagnostics should identify the logical operation requested and the concrete reason no valid plan exists.

Relevant causes include:

```text
field is read-only/write-only
implicit observation would cause destructive collateral read
no legal physical access width
invalid alignment
write footprint touches incompatible field semantics
no safe reserved-bit preservation plan
no coherent read plan
required alias operation unavailable
access context insufficient
mapping/view lifetime invalid
runtime view out of bounds
required ordering/completion unavailable
operation may only be expressed through explicit register Read()
whole-register Write(value) is not representable
shadow value is not known
shadow commit has no legal write plan
```

### 33.2 Mentor diagnostics

Diagnostics should state both the requested logical operation and the physical conflict when known.

Example shape:

```text
error: Ready cannot be observed implicitly

observing Ready requires an 8-bit register read.
that read also observes and clears Count.

help: perform an explicit Status.Read() when this destructive read is intended
```

### 33.3 LSP

The LSP uses the same resolved `CompilationPlan`, endpoint contracts, access contexts, and planner rules as command-line compilation.

Useful hover information may include:

```text
resource identity
endpoint kind
static/runtime-mapped location
field permission
read/write semantics
shadow policy
legal access widths
alignment
implicit read safety
resolved aliases
access-context requirements
```

The LSP must not guess hardware semantics from numeric address shape or naming conventions.

---

## 34. Interaction with interrupts

### 34.1 Dependency direction

The interrupt rulebook may depend on this rulebook for hardware access legality, ordering, completion, access context, fault behavior, and register side effects.

This rulebook does not define interrupt handler syntax, interrupt routing, masking, nesting, prioritization, vector binding, or handler lifetime.

### 34.2 ISR access context

An ISR or interrupt handler receives a target-resolved access context according to the interrupt/platform rules.

The handler does not automatically receive universal hardware authority merely because it executes as an ISR.

### 34.3 Shared RAM

Communication through ordinary RAM between interrupt and ordinary execution remains governed by concurrency/memory/atomic rules.

Volatile register access does not make ordinary RAM race-safe.

### 34.4 Completion before handler exit

Where an interrupt rule requires an acknowledge/clear operation to complete before exception return, the interrupt rule establishes the completion edge and this rulebook supplies the hardware completion semantics and target lowering mechanism.

---

## 35. Interaction with DMA and external memory actors

### 35.1 DMA protocol remains separate

DMA ownership transfer, cache maintenance, descriptor publication, and buffer coherency are not implied by register volatility.

### 35.2 Hardware ordering edges

A DMA/library contract may establish ordering relationships such as:

```text
publish descriptor memory
before
write DMA start/doorbell
```

or:

```text
observe DMA completion
before
consume DMA-written memory
```

This rulebook defines the device-operation side of those relationships and the target mechanism that satisfies them.

### 35.3 No guessed intent

The compiler must not infer DMA protocol merely because a memory write is followed by a register write.

---

## 36. Standard-library and driver boundary

### 36.1 Generic hardware infrastructure

Reusable bus/controller infrastructure belongs in `stdlib/hw` and platform libraries rather than in register field modifiers.

Expected standard areas include at least:

```text
hw/spi
hw/i2c
hw/i2s
hw/uart
```

### 36.2 Device-specific protocols

Device addresses, register indexes behind buses, unlock sequences, command/response protocols, polling rules, and multi-register workflows remain driver/library concerns.

### 36.3 Compiler-known implementations

Stdlib/platform hardware modules may use compiler-known declarations, target intrinsics, direct Semantic IR/MLIR lowering, FFI, or ordinary Sec source as appropriate.

The public API must preserve the same ownership, failure, effect, ordering, and target contracts regardless of implementation technique.

---

## 37. Safety invariants

### 37.1 No hidden hardware operations

Declaring a hardware binding performs no implicit hardware read or write.

Ordinary destruction performs no implicit register reset, clear, write-back, or shadow flush.

### 37.2 No unsafe RMW invention

The compiler never assumes that ordinary read-modify-write is safe merely because the requested field is read-write.

### 37.3 No false bit-addressability

A `bit` or `bit[N]` field never implies that hardware supports bit-granular physical access.

### 37.4 No protocol invention

The compiler does not invent device protocols, privilege transitions, polling loops, active-level interpretation, or runtime resource discovery.

### 37.5 No runtime dependency by default

Static register access, register planning, shadow semantics, and endpoint selection do not intrinsically require a Sec runtime.

Runtime mappings use ordinary owned Sec resource values and platform operations without requiring a global runtime manager.

---

## 38. Required compiler validation

### 38.1 Frontend and semantic validation

The compiler validates at least:

```text
field permission and modifier compatibility
shadow/noshadow inheritance
legal shadow observation/use state
static @address endpoint/region resolution
binding mutability
register/endpoint compatibility
static access-context mismatches
statically known mapping/view offset and alignment errors
```

### 38.2 Planner validation

Before lowering, the compiler validates at least:

```text
candidate endpoint operations
physical access widths
alignment
read/write footprints
collateral side effects
reserved-bit requirements
read/write projections
alias relationships
coherence
atomicity
failure atomicity
ordering
completion
access context
fault model
shadow effects
```

### 38.3 Verification boundary

No unresolved question of whether an ordinary RMW, alias operation, destructive read, shadow commit, or whole-register access is semantically legal may be delegated to LLVM optimization.

---

## 39. Required test families

### 39.1 Field permissions and semantics

Tests cover:

```text
read-write default
explicit read-write
read-only
write-only
read-clear
read-set
write-one-clear
write-one-set
write-one-toggle
write-zero-clear
write-zero-set
write-zero-toggle
contradictory combinations
nested preservation of special semantics
```

### 39.2 Safe implicit reads

Tests cover:

```text
safe full-register read used for one field
implicit read rejected due to collateral read-clear
safe read alias selected
selective target operation selected when available
no field-level Read() assumption
```

### 39.3 Explicit reads and snapshots

Tests cover:

```text
Read() performs one logical observation
snapshot field access performs no new hardware read
destructive Read() preserves one observation
coherent multi-step snapshot
incoherent candidate rejected
```

### 39.4 Shadow

Tests cover:

```text
noshadow default
field shadow
register shadow default
field noshadow override
Unknown/Known behavior
read-only shadow
write-only shadow
read-clear shadow
pending ordinary write
pending W1C/W0C-style intent
Write() no-op with no pending state
no automatic refresh
no automatic destruction write-back
```

### 39.5 Aliases and planning

Tests cover:

```text
set alias selected instead of RMW
clear alias selected instead of RMW
toggle with known and unknown shadow state
safe read alias
address proximity does not infer alias
same resource with different endpoint identities
same numeric address in different domains remains distinct
```

### 39.6 Width, alignment, and footprints

Tests cover:

```text
field width differs from transfer width
read and write widths differ
illegal narrowing/widening rejected
illegal split/merge rejected
static misalignment rejected
runtime view alignment checked
read footprint collateral effect
write footprint incompatible field effect
```

### 39.7 Ordering and completion

Tests cover:

```text
source order preserved
volatile does not imply unrelated memory barrier
required ordering edge lowered
required completion edge lowered
posted write completion mechanism
no invented device protocol
```

### 39.8 Access context

Tests cover:

```text
privilege mismatch
security-domain mismatch
execution-unit-relative endpoint
runtime-acquired mapping authority
RawPtr does not grant authority
unsafe does not elevate privilege
```

### 39.9 Runtime mappings

Tests cover:

```text
move-only mapping ownership
mapping destruction releases platform mapping when required
register view cannot outlive mapping
runtime extent check
runtime alignment check
runtime base plus static endpoint offset
multiple valid borrowed views
exclusive resource claim where required
```

### 39.10 Fault behavior

Tests cover:

```text
known illegal access rejected before lowering
legal fault-capable access not converted automatically to Result
checked mapping failure uses Result
explicit recoverable access API remains distinct
partial multi-step failure does not fabricate coherent shadow
```

### 39.11 IR and lowering

Tests cover:

```text
Semantic IR preserves requested operation
resource and endpoint identity preserved
special field effects survive lowering
ordering/completion edges survive until target materialization
LLVM does not re-decide hardware semantics
```

---

## 40. Completion criteria

### 40.1 Semantic completion

The Sec 0.1 hardware-register access model is complete when the compiler can represent and validate, without backend guesswork:

1. logical register-field reads and writes;
2. safe versus unsafe implicit field observation;
3. explicit register `Read()` and `Write()` operations;
4. field and register-level shadow policy;
5. shadow observed knowledge and pending write intent;
6. hardware resource, endpoint, and operation identity;
7. intentional endpoint aliases;
8. physical access widths and alignment;
9. read and write transaction footprints;
10. coherent multi-step register operations;
11. atomicity and failure atomicity;
12. hardware ordering and completion edges;
13. context-sensitive access authority;
14. static and runtime-resolved hardware bindings;
15. mapping/view ownership and lifetime;
16. target/environment fault behavior;
17. explicit IR representation through target lowering.

### 40.2 Runtime independence

Completion does not require a general Sec runtime.

Static embedded/bare-metal register access must remain capable of lowering directly to target instructions and storage operations.

### 40.3 Protocol boundary

Completion does not require the compiler to understand device-specific multi-register protocols.

Those remain expressible through ordinary Sec code and reusable `stdlib/hw`, platform, generated device, or driver APIs.

---

## 41. Normative summary

### 41.1 Register fields express logical semantics

Register fields define logical values, permissions, and field-local read/write behavior.

They do not define physical bus transaction width or imply bit-addressable hardware.

### 41.2 Hardware bindings resolve endpoints

A hardware binding resolves canonical hardware resources and endpoint operations from the active compilation plan, target/device/project knowledge, or a checked runtime mapping.

### 41.3 The planner owns physical realization

Every hardware-backed field or register operation is planned before lowering.

The planner selects only a semantics-preserving physical operation or operation sequence.

If none exists, compilation fails.

### 41.4 Implicit reads are safe-only

Ordinary field observation may trigger an implicit hardware read only when the compiler proves that the physical transaction causes no unintended collateral semantic effect.

Explicit register `Read()` is used when the programmer intentionally requests the register observation and its defined effects.

### 41.5 Shadow is software knowledge and intent

Shadow state is compiler-managed software state, not a promise of current hardware equality and not a runtime cache service.

Shadow separates observed knowledge from pending write intent and never writes back automatically.

### 41.6 Aliases are resource relations

SET/CLEAR/TOGGLE/read aliases and target instruction variants are modeled by their operations on canonical hardware resources, not by address resemblance.

### 41.7 Ordering, context, and faults remain explicit

Volatile is not a barrier, ordering is not completion, access availability is not authority, and safe Sec does not promise that external hardware can never fault.

Known violations are rejected statically; target-defined runtime faults follow the applicable execution model.

### 41.8 No compiler invention of domain meaning

The compiler preserves declared physical semantics but does not infer active polarity, device protocol, privilege transition, polling behavior, or domain-specific meaning from names.

### 41.9 Hardware programming remains runtime-free by default

The complete static hardware register model can be used without garbage collection, hidden allocation, background synchronization, runtime ownership tracking, or a general Sec runtime.
