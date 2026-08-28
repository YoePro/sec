# Hardware register access cross-rulebook corrections

- **Status:** Applied 2026-08-28
- **Created:** 2026-08-28
- **Last updated:** 2026-08-28
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `6a63b37`
- **Primary new rulebook:** `rules/platform/hardware-register-access.md`

## 1. `rules/declarations/registers.md`

### 1.1 Canonical destructive-read spelling

Replace the current canonical modifier spelling:

```text
clear-on-read
```

with:

```text
read-clear
```

Canonical source example:

```sec
Event: bit read-clear
```

Update prose, diagnostics, formatter expectations, tests, Semantic IR naming, and implementation-status references accordingly.

### 1.2 Read-set

Add the symmetric field semantic where explicitly declared by hardware metadata/source rules:

```sec
Flag: bit read-set
```

Reading the field performs the hardware-defined set side effect.

### 1.3 Write modifier family

Extend the compiler-known write behavior family to include:

```text
write-one-clear
write-one-set
write-one-toggle
write-zero-clear
write-zero-set
write-zero-toggle
```

These modifiers describe the logical effect of one field write. They do not describe multi-register device protocols.

### 1.4 Shadow modifiers

Add compiler-known:

```text
shadow
noshadow
```

as hardware-access policy modifiers usable on register fields.

A register declaration may also specify `shadow` or `noshadow` as its local default.

The global default is `noshadow`.

Field-level `shadow`/`noshadow` overrides the register-level default.

The declarations rulebook owns the modifier syntax and validation.

`rules/platform/hardware-register-access.md` owns hardware binding, shadow state, explicit `Read()`/`Write()`, and transaction semantics.

### 1.5 Separation of ownership

Keep `register[N]` as a general nominal bit-layout type.

Do not make register types themselves volatile, MMIO-bound, runtime-mapped, or hardware-owned.

## 2. `rules/platform/fixed-address-bindings.md`

### 2.1 Named endpoint argument

Extend the Sec 0.1 `@address` form from only a compile-time integer to exactly one statically resolvable address/endpoint expression.

Canonical preferred form:

```sec
@address(Platform.GPIOA)
let mut gpio: GpioRegisters
```

Project-defined canonical endpoints are also valid:

```sec
@address(Project.FpgaControl)
let mut control: FpgaControlRegisters
```

A named endpoint is a compiler-known descriptor, not merely an integer constant.

It retains canonical hardware facts such as address domain/space, location, extent, access contract, aliases, resource identity, ordering requirements, and access-context requirements.

### 2.2 Numeric form

Retain numeric `@address(...)` for the target's canonical linear hardware address domain only.

A numeric address remains invalid unless the complete binding resolves to a canonical valid region/storage contract.

### 2.3 Runtime addresses

Do not extend `@address` to runtime-discovered numeric addresses.

Runtime-resolved hardware mappings use the compiler-known checked mapping/resource model defined by `rules/platform/hardware-register-access.md`.

### 2.4 Field lowering ownership

Replace detailed hardware-register transaction planning rules in this book with a cross-reference to:

```text
rules/platform/hardware-register-access.md
```

Keep fixed-address binding, region validation, mutability, initialization restriction, and volatility consequences in `fixed-address-bindings.md`.

## 3. `rules/platform/volatile.md`

### 3.1 Cross-reference

Retain the existing volatile boundary:

```text
volatile owns physical access occurrence and compiler-visible ordering;
hardware-register-access owns logical field observation, transaction footprints,
destructive-read grouping, aliases, shadow, and register-operation planning.
```

Add `rules/platform/hardware-register-access.md` as the canonical owner of those stronger register semantics.

No other normative volatile change is required.

## 4. Unsafe rulebook

### 4.1 Remove raw numeric `@address` as an unchecked trust assertion

Any unsafe-rule wording that describes raw numeric `@address` as an accepted unvalidated or merely trusted target assertion is superseded.

Every `@address` binding remains compiler-validated against a canonical target/platform/device/project contract.

`unsafe` does not waive that validation.

### 4.2 Dynamic low-level access

Dynamic or otherwise unverifiable addresses use:

```text
checked runtime hardware mapping/resource APIs
or
RawPtr/unsafe operations
```

according to the applicable contract.

Possessing a RawPtr does not grant hardware privilege, mapping authority, security-domain authority, or resource ownership.

## 5. Memory ownership/destruction rulebooks

### 5.1 Runtime mappings

Where cross-references are needed, recognize compiler-known runtime hardware mappings as ordinary move-only resource values whose lifetime is governed by Sec ownership and deterministic destruction.

### 5.2 Register views

Typed register views borrow the mapping/resource lifetime and must not outlive the mapping owner.

### 5.3 No implicit hardware cleanup

Retain the existing rule that destroying a register view or volatile accessor performs no implicit hardware read/write, reset, clear, or shadow flush.

An owning runtime mapping may perform only its explicit platform release/unmap cleanup.

## 6. Semantic IR and MLIR rulebooks

### 6.1 Required preservation

Add explicit hardware-register operations or equivalent verified plans so Semantic IR/MLIR preserve at least:

```text
requested logical register operation
hardware resource identity
endpoint identity
physical operation plan
read/write projection
transaction footprint
special field effects
shadow effects
ordering/completion edges
access-context requirements
fault model
```

LLVM lowering must not re-derive those semantics from generic integer loads/stores.

## 7. ISR rulebook dependency

### 7.1 Dependency direction

`isr.md` should consume `rules/platform/hardware-register-access.md` for:

```text
register access legality
access context
hardware ordering
completion
hardware access faults
register side effects
```

The ISR rulebook should not redefine target-specific register transaction semantics or barrier instructions.
