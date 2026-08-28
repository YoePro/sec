# Correction: add standard hardware-bus infrastructure under `stdlib/hw`

- **Status:** Applied 2026-08-28
- **Created:** 2026-08-28
- **Last updated:** 2026-08-28
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `6a63b37`
- **Target rulebook:** `rules/library/stdlib.md`

## 1. Required correction

### 1.1 Add `hw` as a standard-library area

Add `hw` to the expected top-level standard-library areas.

The standard-library hardware area provides reusable, target-aware infrastructure for common hardware buses and controller protocols above compiler/core register and volatile semantics.

The canonical module root is:

```text
stdlib/hw
```

Repository source resides under the standard-library source root as:

```text
sec/stdlib/hw
```

### 1.2 Required initial submodules

The standard-library hardware area must provide or reserve canonical infrastructure for at least:

```text
hw/spi
hw/i2c
hw/i2s
hw/uart
```

Additional common hardware buses may be added under `hw` when their APIs are defined.

## 2. Responsibility boundary

### 2.1 Compiler/core responsibility

Compiler/core and platform hardware rules own:

```text
register[N] semantics
volatile physical access
hardware register transaction planning
fixed and runtime-resolved hardware bindings
target-specific endpoint operations
hardware ordering/completion primitives
```

### 2.2 Stdlib hardware responsibility

`stdlib/hw` owns reusable bus/controller-facing abstractions such as:

```text
bus/controller interfaces
transfer descriptions
byte/word transfer helpers
chip-select/controller coordination where portable
clock/mode/configuration abstractions
ownership and borrowing contracts for controller handles
fallible transfer APIs
portable driver-facing helper types
```

Exact APIs belong to their own module design work.

### 2.3 Device-driver responsibility

Device-specific addressing and multi-step device protocols remain driver/library concerns.

Examples include:

```text
I2C device addresses
SPI device selection conventions
device register indexes behind a bus
unlock-key sequences
command/response workflows
poll-until-ready protocols
device-specific reset or initialization sequences
```

These semantics must not be moved into generic register field modifiers merely because the device documentation calls them register operations.

## 3. Implementation techniques

### 3.1 Compiler-known implementations are permitted

An `hw` module may use ordinary Sec source, compiler-known declarations, target intrinsics, direct Semantic IR/MLIR lowering, FFI, or platform services when required.

The selected implementation must preserve the public module's:

```text
ownership
borrowing
failure
effects
target availability
ordering
completion
blocking/suspension behavior
allocation requirements
```

### 3.2 No mandatory runtime

Importing or using `stdlib/hw` must not intrinsically require a general Sec runtime.

Bare-metal and RTOS targets may provide direct target-specific implementations.

Hosted targets may provide platform/OS-backed implementations where the target profile makes them available.

## 4. Module organization synchronization

### 4.1 Top-level list

Update the expected top-level standard-library areas to include:

```text
hw
```

### 4.2 Example submodule list

Add examples:

```text
hw/spi
hw/i2c
hw/i2s
hw/uart
```

### 4.3 Required synchronization

Add `rules/platform/hardware-register-access.md` to the standard-library rulebook's required synchronization set once that rulebook is integrated.
