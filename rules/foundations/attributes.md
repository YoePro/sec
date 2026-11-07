# Attributes

- **Status:** Normative
- **Created:** 2026-08-04
- **Last updated:** 2026-11-07 (stable chapter, subsection, and paragraph numbering); 2026-09-29
- **Document revision:** 1.1
- **Sec language version:** 0.1
- **Canonical path:** `rules/foundations/attributes.md`
- **Replaces:** Earlier unversioned revision at the same canonical path
- **Repository baseline reviewed:** `28780c01d63c3fc9eec2321ef67e3dd69962f043`
- **Implementation governance:** `governance/attributes.yaml`
- **Related rulebooks:** `rules/foundations/grammar.md`, `rules/foundations/lexical_structure.md`, `rules/compiler/parser_recovery.md`, `rules/compiler/compile_time_evaluation.md`, `rules/declarations/static.md`, `rules/memory/copy_move.md`, `rules/errors/runtime_checks.md`, `rules/errors/panic.md`, `rules/platform/fixed-address-bindings.md`, `rules/platform/interrupts.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/platform/ffi.md`, `rules/compiler/semantic_ir.md`, `rules/tooling/lsp.md`

---

## § 1 Purpose and authority

**§ 1(1)** This document is the canonical attribute rulebook for Sec 0.1.

**§ 1(2)** It defines:

- the purpose of attributes;
- attribute syntax;
- attachment and scope;
- compile-time argument requirements;
- target selection;
- conditional declaration inclusion;
- absolute-address storage;
- interrupt binding and ISR verification;
- copy policy;
- verified no-allocation, no-panic, and no-block guarantees;
- interrupt-safe functions;
- duplicate and conflict rules;
- formatter and LSP behavior;

**§ 1(3)** Sec 0.1 uses a closed set of compiler-known attributes.

**§ 1(4)** The struct layout modifiers `pack(N)`, `minAlign(N)` and `byteOffset(N)` are compiler-known contextual declaration modifiers in struct and stored-field grammar (MD-019), not entries of this attribute set; `@pack`, `@minAlign` and `@byteOffset` do not exist, and attribute attachment is not extended to struct fields.

**§ 1(5)** Unknown attributes are compile errors.

**§ 1(6)** User-defined passive metadata attributes are not part of Sec 0.1.

---

## § 2 Design principle

**§ 2(1)** Attributes are a first-class but deliberately limited part of the language.

**§ 2(2)** They are used when a property:

```text
belongs to a source declaration
changes compilation, verification, placement, target selection, or lowering
is orthogonal to the declaration's ordinary grammar
does not justify a new declaration keyword
must be understood and validated by the compiler
```

**§ 2(3)** Attributes are not arbitrary comments.

**§ 2(4)** They are not a textual preprocessor.

**§ 2(5)** They are not macros.

**§ 2(6)** They are not runtime reflection metadata unless a future rule explicitly says
otherwise.

**§ 2(7)** `static` is a storage-duration and type-association modifier, not a physical
placement mechanism. It does not by itself select a linker section, absolute
address, MMIO binding, memory space, or target-specific storage class. Physical
placement uses only mechanisms defined by this closed attribute set or by the
canonical storage, ABI, and platform rules. In particular, source must not
assume a conceptual `@section(...)` attribute unless it is separately added to
the compiler-known set with complete semantics.

---

## § 3 Initial compiler-known attribute set

**§ 3(1)** Sec 0.1 initially defines:

```text
@target(...)
@when(...)
@address(...)
@interrupt(...)
@isr
@interruptSafe
@noCopy
@noAlloc
@noPanic
@noBlock
@link_name("foreign-symbol")
```

**§ 3(2)** The set may grow in later language versions.

**§ 3(3)** Adding a new compiler-known attribute requires:

```text
a stable name
defined allowed targets
defined arguments
defined duplicate behavior
defined conflicts
defined semantic effect
defined diagnostics
defined formatter behavior
defined conformance tests
```

---

## § 4 Closed attribute set

**§ 4(1)** Only attributes known to the selected compiler version and language profile are
accepted.

**§ 4(2)** Invalid:

```sec
@audit(category: "finance")
fn PostInvoice() void {
}
```

**§ 4(3)** unless a future rule has registered `@audit` as a compiler-known attribute.

**§ 4(4)** Expected diagnostic:

```text
unknown attribute @audit
```

**§ 4(5)** Sec 0.1 does not preserve unknown attributes as passive metadata.

**§ 4(6)** This avoids introducing syntax without a defined consumer.

**§ 4(7)** Possible future consumers such as:

```text
compiler plugins
code generators
documentation generators
linters
framework metadata
reflection
binary metadata
```

**§ 4(8)** must be designed before arbitrary user-defined attributes are introduced.

---

## § 5 Attribute categories

**§ 5(1)** The initial attributes fall into four semantic categories.

### § 5.1 Selection attributes

```text
@target
@when
```

**§ 5.1(1)** These decide whether a file or top-level statement participates in a concrete
compilation plan.

### § 5.2 Target-binding attributes

```text
@address
@interrupt
```

**§ 5.2(1)** These bind a declaration to target-specific storage or interrupt machinery.

**§ 5.2(2)** **Amendment MD-111 (2026-10-30, `rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md` § 11).** `@address` takes exactly
one statically resolved endpoint path and an optional second contextual marker
`alias`: `@address(Endpoint)` or `@address(Endpoint, alias)`. A numeric,
computed or runtime argument, a third argument, an unknown second marker and a
duplicated `alias` are invalid. `alias` is not a separate `@alias` attribute
(`rules/platform/fixed-address-bindings.md` §§ 2, 10).

### § 5.3 Verified guarantee attributes

```text
@isr
@interruptSafe
@noAlloc
@noPanic
@noBlock
```

**§ 5.3(1)** The compiler must verify the declared guarantee.

**§ 5.3(2)** They are not unchecked programmer promises for ordinary Sec code.

### § 5.4 Nominal policy attributes

```text
@noCopy
```

**§ 5.4(1)** These change a nominal type's semantic classification.

### § 5.5 Foreign-binding attributes

```text
@link_name("foreign-symbol")
```

**§ 5.5(1)** `@link_name` is valid only on an `extern` declaration. It requires exactly one
non-empty string-literal argument and may appear at most once on a declaration.
It changes only the foreign/link symbol: the Sec name, calling convention,
safety, ownership, effects, dependency metadata, and ABI compatibility are
unchanged. Conflicting explicit symbols in one link domain are invalid.

---

## § 6 General syntax

**§ 6(1)** An attribute begins with `@` followed by its compiler-known name.

```ebnf
Attribute
    ::= "@" Identifier AttributeArguments?

AttributeArguments
    ::= "(" AttributeArgumentList? ")"

AttributeArgumentList
    ::= AttributeArgument ("," AttributeArgument)* ","?

AttributeArgument
    ::= Expression
      | Identifier ":" Expression
```

**§ 6(2)** Examples:

```sec
@isr
```

```sec
@address(GpioOutput)
```

```sec
@interrupt(vector: Interrupt.Timer0)
```

```sec
@target(os: "linux", arch: "amd64")
```

```sec
@when(config.telemetry)
```

---

## § 7 Positional and named arguments

**§ 7(1)** An attribute defines whether its arguments are:

```text
positional
named
either
```

**§ 7(2)** The initial forms are:

```text
@address(value)
    one positional compile-time address expression

@interrupt(vector: value)
    named `vector` argument

@target(os: ..., arch: ..., cpu: ..., device: ..., board: ...)
    named selector arguments

@when(condition)
    one positional compile-time boolean expression

@isr
@interruptSafe
@noCopy
@noAlloc
@noPanic
@noBlock
    no arguments
```

**§ 7(3)** Using the wrong argument form is a compile error.

---

## § 8 Compile-time arguments

**§ 8(1)** Every initial attribute argument is a `PlanTimeRequiredContext` as defined by
`rules/compiler/compile_time_evaluation.md`.

**§ 8(2)** Attribute evaluation may use only values available while constructing or
validating the compilation plan.

**§ 8(3)** This preserves the restricted attribute-expression contract. Ordinary
user-function semantic CTE is not permitted in initial `@when` selection,
because plan-time selection establishes the active semantic graph before
ordinary typed Sema:

```text
CompilationRequest
    -> plan-time attribute/selection evaluation
    -> active source graph
    -> ordinary typed Sema
```

**§ 8(4)** No initial attribute argument may require:

```text
runtime execution
allocation
I/O
mutable state
ordinary function calls
dynamic dispatch
foreign calls
```

**§ 8(5)** Compiler-known target constants and configuration values are allowed where the
attribute defines them.

---

## § 9 Attribute attachment

**§ 9(1)** An attribute normally applies to the next top-level statement.

**§ 9(2)** Example:

```sec
@noPanic
fn Calculate() int {
    return 10
}
```

**§ 9(3)** Multiple attributes may attach to the same statement:

```sec
@noAlloc
@noPanic
@noBlock
fn PollDevice() Result[State, DeviceError] {
    // ...
}
```

**§ 9(4)** The attachment sequence ends at the attached statement.

**§ 9(5)** An attribute does not modify the following attribute independently.

**§ 9(6)** All consecutive attributes form one attribute set for the same statement.

---

## § 10 Comments and whitespace

**§ 10(1)** Whitespace and comments between an attribute and its attached statement do not
change attachment.

**§ 10(2)** Example:

```sec
@noPanic

// Public arithmetic entrypoint.
fn Add(left: int, right: int) Result[int, ArithmeticError] {
    let result := try left + right
    return Ok(result)
}
```

**§ 10(3)** A new top-level statement ends the attribute set.

**§ 10(4)** An attribute left without an attached statement before EOF is a compile error.

---

## § 11 Formatter layout

**§ 11(1)** The canonical formatter writes one attribute per line.

```sec
@target(os: "linux")
@when(config.telemetry)
@noPanic
fn SendTelemetry() Result[void, TelemetryError] {
    // ...
}
```

**§ 11(2)** The formatter preserves source order unless a future rule defines a canonical
semantic ordering.

**§ 11(3)** The formatter must not remove redundant attributes automatically merely because
another attribute implies them.

**§ 11(4)** Example:

```sec
@interrupt(vector: Interrupt.Timer0)
@noPanic
fn Timer0Handler() void {
}
```

**§ 11(5)** `@interrupt` already implies `@noPanic` through `@isr`, but the explicit
attribute may remain as documentation.

---

## § 12 Allowed attachment level

**§ 12(1)** The initial general attribute system applies at compilation-unit level.

**§ 12(2)** An attribute may attach to a top-level statement allowed by that attribute.

**§ 12(3)** Initial attributes do not attach to arbitrary local statements or expressions.
The one exception is `@noPanic` before a `let` that directly initializes a
lambda ("Callable-body attachment (MD-030)").

**§ 12(4)** Invalid:

```sec
fn Work() void {
    @target(os: "linux")
    PerformLinuxOperation()
}
```

**§ 12(5)** Local conditional compilation would affect:

```text
control flow
definite assignment
ownership
borrow analysis
unreachable-code analysis
formatting
debug information
```

**§ 12(6)** It is deferred until explicitly designed.

---

## § 13 Attribute validation order

**§ 13(1)** For a concrete compilation plan, the compiler processes attributes in this
order:

```text
1. parse every file and attribute
2. determine file-level @target inclusion
3. evaluate top-level @target and @when selection
4. construct the active source and symbol set
5. validate duplicate and overlapping variants
6. perform ordinary Sema on active declarations
7. verify @noCopy and effect attributes
8. validate target bindings
9. lower active declarations
```

**§ 13(2)** Excluded source remains syntactically parsed.

**§ 13(3)** It does not enter active Sema or lowering for the current plan.

---

## § 14 `@target`

### § 14.1 Purpose

**§ 14.1(1)** `@target` selects source according to target identity.

**§ 14.1(2)** It can apply to:

```text
a complete source file
or the next top-level statement
```

**§ 14.1(3)** The position determines which.

---

## § 15 File-level `@target`

**§ 15(1)** When `@target` is the first source-bearing line in a file, it applies to the
complete file.

**§ 15(2)** Comments and a byte-order marker, where supported, may precede it.

**§ 15(3)** Example:

```sec
@target(os: "linux", arch: "amd64")

module platform.linux.amd64
```

**§ 15(4)** The file participates only in matching compilation plans.

**§ 15(5)** A file-level `@target` must appear before:

```text
module declaration
import
type declaration
function
variable
or any other non-comment source statement
```

**§ 15(6)** A later `@target` applies only to the next top-level statement.

---

## § 16 Statement-level `@target`

**§ 16(1)** When `@target` is not the first source-bearing line, it applies only to the next
top-level statement.

**§ 16(2)** Examples:

```sec
@target(os: "linux")
import "platform/linux"
```

```sec
@target(device: "controller-a")
@address(DeviceEndpoint)
let mut Device: DeviceRegisters
```

```sec
@target(os: "linux", arch: "amd64")
fn RawWrite(...) Result[int, WriteError] {
    // ...
}
```

**§ 16(3)** Allowed statement targets include:

```text
import
type declaration
enum declaration
union declaration
register declaration
unit declaration
interface declaration
impl block
function
method-bearing top-level construct
extern declaration
module-scope variable
static declaration
other module-level declarations defined by grammar.md
```

**§ 16(4)** `@target` does not attach to a module declaration in Sec 0.1.

**§ 16(5)** The module identity must remain stable across variants of one logical module.

---

## § 17 Target selectors

**§ 17(1)** The initial selector names are:

```text
os
arch
cpu
device
board
```

**§ 17(2)** Examples:

```sec
@target(os: "linux")
```

```sec
@target(arch: "amd64")
```

```sec
@target(cpu: "cortex-m4")
```

```sec
@target(device: "stm32f407vg")
```

```sec
@target(board: "controller-rev-b")
```

```sec
@target(
    os: "linux",
    arch: "arm64",
    cpu: "cortex-a72",
    board: "raspberry-pi-4",
)
```

**§ 17(3)** At least one selector is required.

**§ 17(4)** Omitted selectors impose no restriction.

**§ 17(5)** All supplied selectors must match.

**§ 17(6)** The initial selector combination therefore uses logical AND.

---

## § 18 Target selector extensibility

**§ 18(1)** Additional selector names may be added in later language or compiler versions.

**§ 18(2)** Possible future selectors include:

```text
abi
cpuFeature
vendor
environment
objectFormat
```

**§ 18(3)** They are not accepted merely because they appear in this list.

**§ 18(4)** Unknown selector names are errors for the current compiler version.

**§ 18(5)** The target-selector set is compiler-versioned and documented.

---

## § 19 Target terminology

**§ 19(1)** The canonical distinctions are:

```text
os
    operating system or freestanding environment family

arch
    instruction-set architecture or architecture family

cpu
    processor core or processor implementation

device
    complete microcontroller, SoC, or other target device

board
    concrete board or hardware product containing the device and external
    components
```

**§ 19(2)** Examples:

```text
arch
    arm32

cpu
    cortex-m4

device
    stm32f407vg

board
    controller-rev-b
```

**§ 19(3)** Two devices using the same CPU core may have different:

```text
peripheral maps
interrupt vectors
memory maps
clock controllers
DMA channels
pin multiplexing
```

**§ 19(4)** Device-specific addresses should therefore normally select by `device` or
`board`, not only by `cpu`.

---

## § 20 Selector values

**§ 20(1)** String literals are accepted for target selectors.

**§ 20(2)** A future target-identity constant type may also be accepted when explicitly
defined.

**§ 20(3)** Selector comparison is exact after canonical target-name normalization.

**§ 20(4)** The compiler must diagnose:

```text
unknown target value
value incompatible with selected architecture
device incompatible with board
CPU incompatible with device
contradictory selectors
```

**§ 20(5)** Target aliases, if supported, belong to target knowledge packs.

---

## § 21 Multiple declaration variants

**§ 21(1)** Declarations with the same logical symbol name may coexist when their selection
conditions are mutually exclusive.

**§ 21(2)** Example:

```sec
@target(device: "controller-a")
@address(GPIOEndpoint)
let mut GPIO: GPIORegisters

@target(device: "controller-b")
@address(GPIOEndpoint)
let mut GPIO: GPIORegisters
```

**§ 21(3)** For `controller-a`, only the first declaration is active.

**§ 21(4)** For `controller-b`, only the second declaration is active.

---

## § 22 Overlapping variants

**§ 22(1)** If two declarations with the same logical identity are active in one
compilation plan, compilation fails.

**§ 22(2)** Example:

```sec
@target(os: "linux")
fn Open() void {
}

@target(os: "linux", arch: "amd64")
fn Open() void {
}
```

**§ 22(3)** Both are active for Linux amd64.

**§ 22(4)** Expected diagnostic shape:

```text
multiple active target variants define `Open`

active declarations:
    platform.sec:10
    platform.sec:15

compilation plan:
    os = linux
    arch = amd64
```

**§ 22(5)** The compiler must not select the "more specific" declaration implicitly.

---

## § 23 Variant shape compatibility

**§ 23(1)** Mutually exclusive variants of one public symbol must have compatible public
shape.

**§ 23(2)** For functions:

```text
same name
same visibility
same generic parameter shape
same parameter types
same return type
compatible declared effects
compatible ABI contract
```

**§ 23(3)** For variables:

```text
same name
same visibility
same declared type
same mutability
compatible ownership policy
```

**§ 23(4)** For types:

```text
same public identity rules as defined by the type and module rulebooks
```

**§ 23(5)** Target-specific bodies, addresses, vectors, calling-convention lowering, and
private implementation details may differ.

**§ 23(6)** A target selector must not silently turn one public API into an unrelated API.

---

## § 24 No active variant

**§ 24(1)** When no declaration variant is active, the symbol is unavailable for that
compilation plan.

**§ 24(2)** A use should produce a target-aware diagnostic.

**§ 24(3)** Example:

```text
`GPIO` is unavailable for device "controller-c"

available variants:
    device = "controller-a"
    device = "controller-b"
```

---

## § 25 `#target` compatibility form

**§ 25(1)** Sec 0.1 permits the temporary file-level compatibility directive:

```sec
#target(os: "linux", arch: "amd64")
```

**§ 25(2)** Its behavior is:

```text
must appear before code or declarations
accepts `os` and `arch`
requires both
applies to the file
```

**§ 25(3)** The canonical unified source form defined by this rulebook is:

```sec
@target(...)
```

**§ 25(4)** Compilers may accept `#target(...)` as compatibility syntax during migration.

**§ 25(5)** Compatibility behavior:

```text
#target remains file-level only
it does not become a statement-level selector
the formatter may offer an explicit migration fix
new selector support belongs to @target
ordinary formatting must not silently rewrite semantics
```

**§ 25(6)** The compatibility form should be removed only through the normal language
deprecation process.

---

## § 26 `@when`

### § 26.1 Purpose

**§ 26.1(1)** `@when` conditionally includes the next top-level statement according to typed
compile-time program configuration.

**§ 26.1(2)** Example:

```sec
@when(config.telemetry)
fn SendTelemetry() Result[void, TelemetryError] {
    // ...
}
```

**§ 26.1(3)** `@when` does not create runtime branching.

**§ 26.1(4)** The declaration either participates in the compilation plan or does not.

---

## § 27 Compile-time parameters

**§ 27(1)** Compile-time parameters originate from the project build configuration.

**§ 27(2)** They are:

```text
typed
validated
part of compilation-plan identity
part of build-cache identity
not compiler options
not text macros
not runtime configuration
```

**§ 27(3)** Canonical source access for boolean configuration is:

```sec
config.telemetry
```

**§ 27(4)** Example manifest concept:

```toml
[parameters]
telemetry = false
audit = true
```

**§ 27(5)** Example source:

```sec
@when(config.audit)
fn RecordAuditEntry(...) Result[void, AuditError] {
    // ...
}
```

---

## § 28 `@when` condition language

**§ 28(1)** Sec 0.1 permits a restricted compile-time boolean expression.

**§ 28(2)** Allowed forms:

```text
boolean compile-time parameter
true
false
!
&&
||
==
!=
parentheses
```

**§ 28(3)** Examples:

```sec
@when(config.audit)
```

```sec
@when(!config.telemetry)
```

```sec
@when(config.audit && config.databaseLogging)
```

**§ 28(4)** Target identity belongs in `@target`, not in `@when`, for Sec 0.1.

**§ 28(5)** Preferred:

```sec
@target(os: "linux")
@when(config.audit)
fn InstallAuditIntegration() void {
}
```

**§ 28(6)** Not canonical in Sec 0.1:

```sec
@when(config.audit && target.os == "linux")
```

**§ 28(7)** This separation keeps target selection and program configuration distinct.

---

## § 29 Compiler options are not `@when` inputs

**§ 29(1)** Compiler options do not control declaration existence.

**§ 29(2)** Invalid conceptual inputs include:

```text
optimization level
debug-information level
warning level
LTO
strip setting
bounds-check lowering policy
overflow-check lowering policy
```

**§ 29(3)** Changing debug or optimization configuration must not silently change the
program's public source graph.

**§ 29(4)** Build features use typed compile-time parameters instead.

---

## § 30 No textual preprocessor

**§ 30(1)** Sec does not provide C-style:

```text
#define
#ifdef
#ifndef
-DNAME
text substitution
token concatenation
conditional lexical regions
```

**§ 30(2)** All source remains parseable.

**§ 30(3)** Conditional inclusion is declaration-aware and typed.

---

## § 31 Excluded source

**§ 31(1)** A file or statement excluded by `@target` or `@when`:

```text
is lexed and parsed
is available to formatter
is visible to LSP as excluded
does not enter the active symbol table
does not require unavailable imports or types for the current plan
does not undergo ordinary active Sema
does not reach Semantic IR
does not reach lowering
```

**§ 31(2)** Syntax errors remain syntax errors even in excluded source.

**§ 31(3)** This prevents invalid source from being hidden indefinitely behind a selector.

**§ 31(4)** Target-dependent semantic errors are evaluated only when the source is active.

---

## § 32 `@address`

### § 32.1 Purpose

**§ 32.1(1)** `@address` binds one module-scope register declaration to a named hardware
endpoint (MD-111 § 10(1)); the endpoint's metadata supplies the address. A
numeric, arithmetic, call or variable argument is rejected with S1188
`hardware.address-binding`; an unknown, inaccessible or wrong-category name
with S1183 `hardware.symbol-resolution`. `@address(Endpoint, alias)` adds the
contextual alias acknowledgement.

**§ 32.1(2)** Canonical:

```sec
@address(GPIOEndpoint)
let mut GPIO: GPIORegisters
```

---

## § 33 Address semantics

**§ 33(1)** `@address(Endpoint)` means:

```text
the storage exists at the supplied address
the declaration does not allocate ordinary storage
reads have volatile semantics
mut controls whether writes are permitted
the address is part of target binding
the declaration has address-stability restrictions
ordinary relocation does not apply to the storage
```

**§ 33(2)** The spelling supplies an address expression; it does not independently prove a
valid binding. Every `@address` declaration must resolve against a canonical
target-owned address-region or storage contract from the active
`CompilationPlan`, `MemoryEnvironment`, `DeviceModel`, or equivalent source.
The contract must cover the complete bound extent.

**§ 33(3)** `@address` implies volatile access.

**§ 33(4)** A separate `@volatile` is not required for addressed storage.

---

## § 34 Address argument

**§ 34(1)** The argument must be a compile-time address expression.

**§ 34(2)** Allowed initial forms:

```text
integer address literal
compiler-known typed peripheral address constant
target knowledge-pack address constant
other explicitly defined compile-time address constant
```

**§ 34(3)** Examples:

```sec
@address(GPIOEndpoint)
let mut GPIO: GPIORegisters
```

```sec
@address(Peripheral.GPIOA)
let mut GPIOA: GPIORegisters
```

**§ 34(4)** The compiler validates:

```text
representability in target pointer width
canonical region coverage for the complete bound extent
address domain and address-space legality
region availability and read/write permissions
alignment and permitted physical access widths
physical representation and storage/device compatibility
required privilege where applicable
overlap with other absolute declarations where detectable
```

**§ 34(5)** A compile-time numeric value is necessary for raw numeric syntax but is not
sufficient for validity. If no applicable canonical region covers the complete
binding, the declaration is invalid. `unsafe` does not waive this validation.

---

## § 35 Address declaration target

**§ 35(1)** `@address` applies to exactly one module-scope `let` declaration.

**§ 35(2)** It does not apply to:

```text
grouped declarations
local variables
parameters
fields
temporary expressions
functions
types
```

**§ 35(3)** Invalid:

```sec
@address(FirstEndpoint)
let mut first: Register, second: Register
```

**§ 35(4)** The declaration must name exactly one storage object.

---

## § 36 Address and initialization

**§ 36(1)** Addressed storage is not initialized like ordinary allocated storage.

**§ 36(2)** The declaration must not generate ordinary zero initialization or copy an
initializer into the hardware address unless a specialized rule explicitly
permits initialization.

**§ 36(3)** The type's default value does not imply a startup write.

**§ 36(4)** Read and write behavior follows:

```text
declared type
mutability
volatile semantics
register rules
target knowledge
```

---

## § 37 Target-dependent addresses

**§ 37(1)** Use mutually exclusive declaration variants.

```sec
@target(device: "controller-a")
@address(GPIOEndpoint)
let mut GPIO: GPIORegisters

@target(device: "controller-b")
@address(GPIOEndpoint)
let mut GPIO: GPIORegisters
```

**§ 37(2)** All attributes in each block attach to that declaration.

**§ 37(3)** An attribute does not scope another attribute.

---

## § 38 Knowledge-pack addresses

**§ 38(1)** A target knowledge pack may define typed names:

```sec
@address(Peripheral.GPIOA)
let mut GPIOA: GPIORegisters
```

**§ 38(2)** The knowledge pack may provide:

```text
numeric address
address space
alignment
register block compatibility
read/write capabilities
device availability
aliases
reserved status
```

**§ 38(3)** Raw numeric addresses remain allowed for:

```text
custom targets
new ports
application-specific hardware
external memory-mapped devices
early platform development
```

**§ 38(4)** These forms preserve low ceremony, but the selected target definition must
still provide the region facts needed to validate the complete binding. Raw
numeric spelling is never an implicit trust boundary.

---

## § 39 `@interrupt`

### § 39.1 Purpose

**§ 39.1(1)** `@interrupt` binds a function to an interrupt vector and implies `@isr`.

**§ 39.1(2)** Canonical:

```sec
@interrupt(vector: Interrupt.Timer0)
fn Timer0Handler() void {
}
```

**§ 39.1(3)** The implication is normative:

```text
@interrupt(...)
    implies @isr
```

**§ 39.1(4)** The reverse does not apply.

---

## § 40 Interrupt vector argument

**§ 40(1)** `@interrupt` requires one named argument:

```text
vector
```

**§ 40(2)** Examples:

```sec
@interrupt(vector: Interrupt.Timer0)
fn Timer0Handler() void {
}
```

```sec
@interrupt(vector: 15)
fn CustomHandler() void {
}
```

**§ 40(3)** The value must be a compile-time vector identity accepted by the selected
target.

---

## § 41 Named interrupt vectors

**§ 41(1)** The resolved platform/Device interrupt catalog may expose typed vector
constants through the active `CompilationPlan`, as defined by
`rules/platform/interrupts.md`.

**§ 41(2)** Examples:

```text
Interrupt.Reset
Interrupt.NMI
Interrupt.HardFault
Interrupt.Timer0
Interrupt.UART1
```

**§ 41(3)** The resolved interrupt definition supplies target facts such as:

```text
numeric vector
vector-table position
reserved status
exception priority class
required ABI
handler signature
device availability
aliases
```

**§ 41(4)** Named vectors are preferred when available.

**§ 41(5)** Raw numeric vectors remain allowed for custom targets and new ports.

---

## § 42 Interrupt validation

**§ 42(1)** The compiler validates:

```text
vector exists for selected target
vector may be user-bound
only one active handler owns the vector
function signature is valid
calling convention is valid
required priority metadata is present when required
vector-table generation is supported or externally supplied
ISR restrictions are satisfied
```

**§ 42(2)** Duplicate active vector binding is a compile error.

---

## § 43 Target-dependent interrupt binding

**§ 43(1)** Use mutually exclusive function variants when binding differs.

```sec
@target(device: "controller-a")
@interrupt(vector: Interrupt.Timer0)
fn TimerHandler() void {
    // ...
}

@target(device: "controller-b")
@interrupt(vector: Interrupt.Timer2)
fn TimerHandler() void {
    // ...
}
```

**§ 43(2)** The public function shape must remain compatible.

---

## § 44 `@isr`

### § 44.1 Purpose

**§ 44.1(1)** `@isr` declares and verifies an interrupt service routine without automatically
binding it to a vector.

```sec
@isr
fn SharedInterruptHandler() void {
}
```

**§ 44.1(2)** Use cases include:

```text
external startup code performs vector binding
manual vector tables
shared interrupt entry functions
handlers exported through FFI
testing ISR-compatible code
targets with nonstandard binding
```

---

## § 45 Relationship between `@interrupt` and `@isr`

**§ 45(1)** The relationship is:

```text
@interrupt is the stronger construct
@interrupt includes vector binding
@interrupt includes all @isr verification
@isr may stand alone
@isr does not choose a vector
```

**§ 45(2)** In set terms:

```text
the guarantees of @isr are a subset of the semantics of @interrupt
```

---

## § 46 ISR verification

**§ 46(1)** `@isr` implies at least:

```text
noPanic
noAlloc
noBlock
interrupt-safe call graph
target-compatible signature
target-compatible entry and return behavior
bounded stack requirements
no forbidden lock acquisition
no ordinary scheduler suspension
no unsafe shared mutation without an approved synchronization model
no call to code with unknown ISR effects
```

**§ 46(2)** The detailed ISR analysis belongs to the call-graph, stack-analysis, allocation,
blocking, ownership, and concurrency rulebooks.

---

## § 47 ISR ABI

**§ 47(1)** The compiler or target knowledge pack determines:

```text
entry convention
saved registers
return instruction
stack alignment
privilege transition
interrupt masking assumptions
vector-table representation
```

**§ 47(2)** The source function remains a Sec function declaration with compiler-verified
ISR constraints.

**§ 47(3)** A target may reject `@isr` when it lacks a defined ISR ABI.

---

## § 48 `@interruptSafe`

### § 48.1 Purpose

**§ 48.1(1)** `@interruptSafe` declares that a function may be called from ISR code.

```sec
@interruptSafe
fn ReadStatus() Status {
    // ...
}
```

**§ 48.1(2)** It does not make the function an ISR.

**§ 48.1(3)** It does not bind a vector.

---

## § 49 Interrupt-safe guarantee

**§ 49(1)** The compiler verifies at least:

```text
noPanic
noAlloc
noBlock
no forbidden synchronization
no unbounded scheduler interaction
safe shared-state access
compatible callees
stack behavior acceptable under ISR policy
```

**§ 49(2)** An `@isr` call graph may call:

```text
other @isr-specific compiler operations
@interruptSafe functions
compiler-proven equivalent functions
```

**§ 49(3)** It may not call an ordinary unverified function merely because the current body
appears simple.

---

## § 50 `@noCopy`

### § 50.1 Purpose

**§ 50.1(1)** `@noCopy` is a nominal type policy.

```sec
@noCopy
type SessionID struct {
    value: uint64
}
```

**§ 50.1(2)** It explicitly forbids compiler-derived implicit copy.

---

## § 51 `@noCopy` semantics

**§ 51(1)** For an `@noCopy` type:

```text
ordinary implicit copy is forbidden
explicit ownership transfer may remain valid
named Copy, Clone, Duplicate, Snapshot, or ToOwned methods remain ordinary APIs
copyability is distinct from movability
copyability is distinct from relocatability
copyability is distinct from pinning
```

**§ 51(2)** Example invalid copy:

```sec
let second := first
```

**§ 51(3)** Example possible move:

```sec
let second :<- first
```

**§ 51(4)** The move is valid only when ownership, lifetime, relocation, and pinning rules
also permit it.

---

## § 52 `@noCopy` allowed targets

**§ 52(1)** `@noCopy` applies to nominal type declarations.

**§ 52(2)** Initial allowed forms include:

```text
named struct
named enum where policy is meaningful
named union
named register
named underlying type
other nominal user-defined types
```

**§ 52(3)** It does not apply to:

```text
individual variables
fields
parameters
functions
anonymous types
type references
```

**§ 52(4)** A variable cannot override the copy policy of its type.

---

## § 53 `@noCopy` and fields

**§ 53(1)** A containing type becomes non-copyable when it stores an `@noCopy` field unless
a separate rule explicitly uses indirection with copyable semantics.

**§ 53(2)** The compiler should diagnose the cause chain.

**§ 53(3)** Example:

```text
Container cannot be copied
  -> field `session` has type SessionID
  -> SessionID explicitly forbids copy through @noCopy
```

---

## § 54 No positive copy attribute in the initial subset

**§ 54(1)** Sec 0.1 does not initially define:

```text
@copy
@copyable
```

**§ 54(2)** Derived copyability remains automatic when the compiler proves it.

**§ 54(3)** A future positive attribute may mean:

```text
compilation fails unless this type remains compiler-derivably copyable
```

**§ 54(4)** It must never mean:

```text
invoke arbitrary hidden user copy code
```

---

## § 55 `@noAlloc`

### § 55.1 Purpose

**§ 55.1(1)** `@noAlloc` is a compiler-verified transitive guarantee.

```sec
@noAlloc
fn Process(buffer: ref byte[]) Result[void, ProcessError] {
    // ...
}
```

**§ 55.1(2)** It means no reachable execution path performs allocation.

---

## § 56 Allocation definition

**§ 56(1)** Allocation includes acquiring new dynamic storage through:

```text
heap allocator
arena allocator where the operation consumes arena capacity
collection growth
implicit boxing
closure environment allocation
task or thread creation when it allocates control storage
foreign calls declared as allocating
compiler helper that allocates
```

**§ 56(2)** The allocation rulebook owns the exact classification.

---

## § 57 What is not allocation

**§ 57(1)** The following are not allocation merely by existing:

```text
stack locals
fixed arrays
caller-provided storage
addressed storage
static storage
reusing already-owned capacity without growth
register values
compiler-elided temporary storage
```

**§ 57(2)** A specific operation may still allocate according to its implementation and
declared effects.

---

## § 58 `@noAlloc` verification

**§ 58(1)** The compiler verifies:

```text
the function body
all reachable callees
generic specializations
closures
defer bodies
destructors reached from the function
foreign declarations
compiler-generated operations
```

**§ 58(2)** Unknown allocation behavior violates the guarantee.

---

## § 59 `@noAlloc` and fallible allocation

**§ 59(1)** Using `try` does not make allocation compatible with `@noAlloc`.

**§ 59(2)** Example:

```sec
let value := try allocator.New[Value]()
```

**§ 59(3)** This is panic-free but still allocates.

**§ 59(4)** `@noAlloc` forbids the operation.

---

## § 60 `@noPanic`

### § 60.1 Purpose

**§ 60.1(1)** `@noPanic` is a compiler-verified transitive guarantee.

```sec
@noPanic
fn Add(
    left: int,
    right: int,
) Result[int, ArithmeticError] {
    let result := try left + right
    return Ok(result)
}
```

**§ 60.1(2)** Its complete semantics are defined by:

```text
panic.md
runtime_checks.md
```

---

## § 61 `@noPanic` guarantee

**§ 61(1)** The function has no reachable:

```text
language-defined panic
panic-capable operation not proven safe
panic-capable call
unproven assert
reachable checked unreachable
explicit panic
unknown foreign abort or unwind
panic-capable cleanup
```

**§ 61(2)** The compiler verifies the complete reachable call graph.

---

## § 62 `@noPanic` is not exception suppression

**§ 62(1)** `@noPanic` does not catch panic.

**§ 62(2)** It requires the panic effect to be absent.

**§ 62(3)** Expected failures use explicit result values and `try`.

---

## § 63 `@noPanic` and proof

**§ 63(1)** Ordinary checked operations may remain in `@noPanic` code when their failure is
proven impossible.

**§ 63(2)** Otherwise the code must:

```text
use try
handle or propagate an explicit error
select an explicit total semantic
change the types or contracts so safety is proven
```

---

## § 64 `@noBlock`

### § 64.1 Purpose

**§ 64.1(1)** `@noBlock` is a compiler-verified transitive guarantee.

```sec
@noBlock
fn PollStatus() Status {
    // ...
}
```

**§ 64.1(2)** It means no reachable execution path waits for an unbounded external event or
parks the current execution domain.

---

## § 65 Blocking operations

**§ 65(1)** Blocking includes operations such as:

```text
blocking I/O
sleep
waiting on a mutex or semaphore
condition-variable wait
thread join
task wait classified as blocking
scheduler parking
waiting for external input without a compile-time bounded policy
foreign calls declared as blocking
```

**§ 65(2)** The concurrency and platform rulebooks own detailed classification.

---

## § 66 What `@noBlock` does not automatically forbid

**§ 66(1)** `@noBlock` does not by itself forbid:

```text
ordinary finite computation
statically bounded loops
nonblocking polling
try-lock
bounded lock-free retry where separately proven
reading memory-mapped status
```

**§ 66(2)** Execution-time and worst-case latency guarantees are separate analyses.

**§ 66(3)** A function can be nonblocking yet take too long for an ISR.

---

## § 67 `@noBlock` verification

**§ 67(1)** The compiler verifies:

```text
the body
all reachable callees
generic specializations
cleanup
foreign declarations
scheduler operations
synchronization operations
I/O operations
```

**§ 67(2)** Unknown blocking behavior violates the guarantee.

---

## § 68 Allowed targets for verified effects

**§ 68(1)** The initial allowed targets for:

```text
@noAlloc
@noPanic
@noBlock
@interruptSafe
@isr
@interrupt
```

**§ 68(2)** are functions and methods. `@noPanic` additionally attaches to the callable
bodies listed in "Callable-body attachment (MD-030)" below.

**§ 68(3)** `@interrupt` and `@isr` have additional target-specific signature rules.

**§ 68(4)** Attributes on lambda expressions and function types are otherwise deferred
until function-effect type syntax is finalized.

**§ 68(5)** Attributes on complete `impl` blocks are not part of the initial subset.

**§ 68(6)** Each method must declare or derive its own guarantee.

---

## § 69 Callable-body attachment (MD-030)

**§ 69(1)** Decided 2026-10-17
(`rules/corrections/applied/md030-no-panic-attribute-placement-correction-20261017.md`).

1. A callable declaration with an independently analyzable body may carry
   `@noPanic` on the line(s) immediately preceding it. The attribute applies to
   that callable body's invocation contract — never to a containing `impl`,
   `property` or binding, and never to a sibling body.
2. Inside an `impl`, `@noPanic` may precede a body-bearing `init` and the
   `free` lifecycle member. `free` stays a lifecycle member, and its generated
   field cleanup is part of the proof; an early `return` never skips cleanup.
3. Inside a `property`, `@noPanic` may precede each accessor — `get`, `set`
   or `try set` — separately. Annotating one accessor never annotates the
   other. A fallible setter keeps its declared error channel; returning `Err`
   does not violate `@noPanic`.
4. `@noPanic` before a `let` whose initializer is directly a lambda
   (`fn(...)` or `capture(...) fn(...)`) annotates the lambda's body, both at
   top level and in local scope. This is the only local statement an attribute
   may precede. A type annotation on the `let` does not change the target. The
   guarantee covers invoking the lambda, not evaluating the initializer or the
   captures.
5. Not introduced: `@noPanic` on `impl` or `property` as a shortcut, on a
   `let` whose initializer is not directly a lambda (a call, conversion or
   conditional), and the expression-prefix form `let f := @noPanic fn(...)`.
   Each is rejected with a diagnostic naming the supported placements.
   Anonymous lambdas passed or returned without a binding may still be
   inferred panic-free but have no explicit attribute placement.
6. Every placement uses the existing transitive proof of
   `rules/errors/panic.md` § 21; unknown effect facts are not proof, and
   FFI/`unsafe` trust boundaries are unchanged. Other verified-effect
   attributes (`@noAlloc`, `@noBlock`) keep their function and method targets.
7. The formatter keeps each attribute on its own line before its declaration
   and never moves it into a lambda expression.

```sec
impl Counter {
    @noPanic
    init(value: int) {
        self._value = value
    }

    property Value: int {
        @noPanic
        get {
            return self._value
        }
    }
}

fn Use() bool {
    @noPanic
    let identity := fn(value: bool) bool {
        return value
    }
    return identity(true)
}
```

---

## § 70 Sec code versus foreign declarations

**§ 70(1)** For an ordinary Sec function with a body, verified attributes are proven by the
compiler.

**§ 70(2)** For foreign declarations, the compiler normally cannot prove the foreign
implementation.

**§ 70(3)** On an extern declaration, an applicable effect-guarantee attribute is a trusted
foreign contract rather than a compiler proof of the foreign body. The compiler
must preserve this trust provenance and treat unspecified foreign effects
conservatively.

```text
unknown foreign effects violate noPanic, noAlloc, noBlock, and interruptSafe
extern guarantees are usable only as explicitly trusted foreign facts
```

---

## § 71 Attribute implications

**§ 71(1)** The initial implication graph is:

```text
@interrupt(...)
    => @isr

@isr
    => @noPanic
    => panic-free requirement

@isr
    => @noAlloc

@isr
    => @noBlock

@isr
    => interrupt-safe call graph

@interruptSafe
    => @noPanic
    => panic-free requirement

@interruptSafe
    => @noAlloc

@interruptSafe
    => @noBlock
```

**§ 71(2)** An implied attribute does not require duplicated AST storage as though the user
wrote it.

**§ 71(3)** The compiler records effective guarantees separately from explicitly written
attributes.

---

## § 72 Duplicate attributes

**§ 72(1)** An attribute may appear at most once in one attachment set unless its definition
explicitly permits repetition.

**§ 72(2)** Invalid:

```sec
@noCopy
@noCopy
type Identity struct {
    value: uint64
}
```

**§ 72(3)** Invalid:

```sec
@address(DeviceEndpoint)
@address(OtherDeviceEndpoint)
let mut Device: DeviceRegisters
```

**§ 72(4)** Invalid:

```sec
@target(os: "linux")
@target(arch: "amd64")
fn Open() void {
}
```

**§ 72(5)** Write one target attribute:

```sec
@target(os: "linux", arch: "amd64")
fn Open() void {
}
```

---

## § 73 Duplicate arguments

**§ 73(1)** An argument name may appear only once in one attribute.

**§ 73(2)** Invalid:

```sec
@target(os: "linux", os: "windows")
fn Open() void {
}
```

**§ 73(3)** Invalid:

```sec
@interrupt(vector: 10, vector: 20)
fn Handler() void {
}
```

---

## § 74 Conflicting attributes

**§ 74(1)** The compiler rejects attributes whose semantics conflict.

**§ 74(2)** Examples include:

```text
two absolute addresses
two interrupt vectors
contradictory target selectors
future positive copy requirement with @noCopy
a verified effect contradicted by the declaration's required behavior
```

**§ 74(3)** The diagnostic should show both source locations and the conflict rule.

---

## § 75 Redundant attributes

**§ 75(1)** An explicitly written attribute may be redundant because another implies it.

**§ 75(2)** Example:

```sec
@interrupt(vector: Interrupt.Timer0)
@isr
@noPanic
@noAlloc
@noBlock
fn Timer0Handler() void {
}
```

**§ 75(3)** This is valid.

**§ 75(4)** The compiler may emit an informational diagnostic if configured.

**§ 75(5)** The formatter does not delete the redundant declarations automatically.

---

## § 76 Attribute order

**§ 76(1)** Initial attribute semantics are order-independent within one attachment set.

**§ 76(2)** These are semantically equivalent:

```sec
@target(device: "controller-a")
@address(DeviceEndpoint)
let mut Device: DeviceRegisters
```

```sec
@address(DeviceEndpoint)
@target(device: "controller-a")
let mut Device: DeviceRegisters
```

**§ 76(3)** The first style is recommended because selection precedes binding conceptually.

**§ 76(4)** The formatter may later establish a canonical category order, but ordinary
formatting must not change attachment or semantics.

---

## § 77 Parser representation

**§ 77(1)** Attributes should be represented explicitly.

**§ 77(2)** Conceptual AST:

```go
type Attribute struct {
    Token     lexer.Token
    Name      string
    Arguments []AttributeArgument
}

type AttributeArgument struct {
    Name  string
    Value ast.Expression
}
```

**§ 77(3)** A top-level statement should carry:

```go
Attributes []Attribute
```

**§ 77(4)** File-level attributes should be represented on the compilation unit or program.

**§ 77(5)** Do not encode each new attribute solely through an unrelated special-case field.

**§ 77(6)** Specialized semantic fields may be derived after generic attribute parsing.

---

## § 78 Semantic representation

**§ 78(1)** Sema should resolve attributes into compiler-known semantic properties.

**§ 78(2)** Conceptual:

```text
TargetSelection
ConditionalSelection
AbsoluteAddressBinding
InterruptBinding
ISRGuarantee
InterruptSafeGuarantee
NoCopyPolicy
NoAllocGuarantee
NoPanicGuarantee
NoBlockGuarantee
```

**§ 78(3)** The original syntax remains available for diagnostics, formatter, and LSP.

---

## § 79 Selection before active symbols

**§ 79(1)** `@target` and `@when` selection occurs before active symbol-table construction.

**§ 79(2)** This permits mutually exclusive definitions of one logical symbol.

**§ 79(3)** The compiler must still parse all variants before selection.

---

## § 80 Inactive declarations and name conflicts

**§ 80(1)** Inactive declarations do not conflict with active declarations for the current
plan.

**§ 80(2)** However, the compiler should perform cross-variant validation to detect:

```text
overlapping selectors
incompatible public shapes
unreachable variants
unknown target values
invalid configuration references
```

**§ 80(3)** This validation may operate across all declared compilation variants.

---

## § 81 LSP behavior

**§ 81(1)** The LSP should expose:

```text
attribute hover documentation
allowed target information
effective implied guarantees
target exclusion reason
configuration exclusion reason
active compilation plan
panic/allocation/blocking call-chain causes
interrupt vector information
address knowledge-pack information
duplicate or conflict diagnostics
```

**§ 81(2)** Excluded source remains visible.

**§ 81(3)** Example hover:

```text
Excluded from current plan

Requires:
    device = "controller-a"

Current:
    device = "controller-b"
```

---

## § 82 LSP completion

**§ 82(1)** After `@`, completion offers only compiler-known attributes valid at the current
attachment location.

**§ 82(2)** Examples:

```text
before a nominal type
    noCopy
    target
    when

before a function
    target
    when
    interrupt
    isr
    interruptSafe
    noAlloc
    noPanic
    noBlock

before a module-scope let
    target
    when
    address
```

**§ 82(3)** Attribute argument completion may offer:

```text
target selector names
known OS values
known architectures
known CPUs
known devices
known boards
known interrupt vectors
known peripheral addresses
compile-time boolean parameters
```

---

## § 83 Formatter behavior

**§ 83(1)** The formatter:

```text
writes one attribute per line
formats named arguments consistently
formats multiline target lists with trailing commas
preserves unknown-invalid syntax for recovery
preserves explicit redundant guarantees
does not change target conditions
does not convert raw addresses to knowledge-pack names automatically
does not merge declarations
does not apply panic-free migration
```

**§ 83(2)** Example:

```sec
@target(
    os: "linux",
    arch: "arm64",
    device: "example-device",
)
@noPanic
fn Open() Result[Handle, OpenError] {
    // ...
}
```

---

## § 84 Diagnostics

**§ 84(1)** Suggested diagnostic families:

```text
attribute.unknown
attribute.missing-target
attribute.invalid-target
attribute.invalid-argument
attribute.duplicate
attribute.duplicate-argument
attribute.conflict
attribute.unattached
attribute.not-allowed-on-target
attribute.argument-not-compile-time
attribute.target-selector-unknown
attribute.target-value-unknown
attribute.target-variants-overlap
attribute.target-shape-mismatch
attribute.config-unknown
attribute.config-not-bool
attribute.address-invalid
attribute.address-misaligned
attribute.address-overlap
attribute.interrupt-vector-unknown
attribute.interrupt-vector-reserved
attribute.interrupt-vector-duplicate
attribute.isr-signature
attribute.isr-effect
attribute.interrupt-safe-effect
attribute.no-copy-violation
attribute.no-alloc-violation
attribute.no-panic-violation
attribute.no-block-violation
attribute.foreign-effect-unproven
```

**§ 84(2)** Final stable IDs belong to the diagnostics registry.

---

## § 85 Required tests

**§ 85(1)** Create or update:

```text
attributes_valid.sec
attributes_invalid.sec
target_file_valid.sec
target_file_invalid.sec
target_statement_valid.sec
target_statement_invalid.sec
target_variants_valid.sec
target_variants_invalid.sec
when_valid.sec
when_invalid.sec
address_valid.sec
address_invalid.sec
interrupt_valid.sec
interrupt_invalid.sec
isr_valid.sec
isr_invalid.sec
interrupt_safe_valid.sec
interrupt_safe_invalid.sec
no_copy_valid.sec
no_copy_invalid.sec
no_alloc_valid.sec
no_alloc_invalid.sec
no_panic_valid.sec
no_panic_invalid.sec
no_block_valid.sec
no_block_invalid.sec
```

---

## § 86 Target test matrix

**§ 86(1)** Test selectors individually:

```text
os
arch
cpu
device
board
```

**§ 86(2)** Test combinations.

**§ 86(3)** Test:

```text
omitted selectors
unknown selectors
unknown values
duplicate selectors
contradictory selectors
file-level selection
statement-level selection
mutually exclusive declarations
overlapping declarations
no active declaration
public shape mismatch
device-specific addresses
board-specific declarations
```

---

## § 87 Conditional compilation tests

**§ 87(1)** Test:

```text
boolean parameter true
boolean parameter false
negation
and
or
equality
inequality
parentheses
unknown parameter
non-boolean parameter
compiler option rejected as config input
excluded source parsed
excluded source absent from active Sema
```

---

## § 88 Address tests

**§ 88(1)** Test:

```text
raw literal address
knowledge-pack address
immutable read-only declaration
mutable declaration
misalignment
pointer-width overflow
grouped declaration rejection
local declaration rejection
ordinary initialization not emitted
volatile reads
volatile writes
target-dependent address variants
overlapping absolute storage
```

---

## § 89 Interrupt tests

**§ 89(1)** Test:

```text
named vector
raw vector
unknown vector
reserved vector
duplicate active binding
mutually exclusive target bindings
signature mismatch
@interrupt implies @isr
@isr without vector
@isr noPanic violation
@isr noAlloc violation
@isr noBlock violation
@isr unsafe shared mutation
@interruptSafe valid helper
@interruptSafe effect violation
```

---

## § 90 Effect tests

**§ 90(1)** For each verified effect, test:

```text
direct violation
transitive violation
generic specialization violation
closure violation where applicable
defer violation
destructor violation
foreign unknown effect
proof removes apparent violation
full call-chain diagnostic
```

---

## § 91 Binary and lowering tests

**§ 91(1)** Verify:

```text
excluded declarations do not reach object files
target-specific address is selected correctly
vector table contains one active handler
@address produces volatile access
no ordinary storage allocation for addressed declarations
no mandatory runtime introduced by attributes
noPanic/noAlloc/noBlock add no runtime support
knowledge-pack constants lower to target values
panic-free binaries can omit panic support
```

---

## § 92 Required synchronization

**§ 92(1)** This document must remain synchronized with:

```text
grammar.md
lexical_structure.md
parser_recovery.md
projects.md
copy_move.md
ownership.md
runtime_checks.md
panic.md
allocation.md
defer.md
destruction.md
inline_assembly.md
platform/ffi.md
functions.md
declarations/interfaces.md
impl.md
generics rulebook
registers rulebook
addressed-variable rulebook
interrupt and ISR rulebooks
compiler_pipeline.md
semantic_ir.md
formatter.md
lsp.md
diagnostics.md
language-rulebook-status.md
```

---

## § 93 Appendix B — Canonical initial attribute table

| Attribute | Category | Initial target | Arguments | Core meaning |
|---|---|---|---|---|
| `@target` | selection | file or top-level statement | `os`, `arch`, `cpu`, `device`, `board` | select source for target |
| `@when` | selection | top-level statement | boolean compile-time condition | select source by typed configuration |
| `@address` | target binding | one module-scope `let` | one address expression | bind volatile storage |
| `@interrupt` | target binding and verified | function | `vector` | bind vector and imply ISR |
| `@isr` | verified | function | none | verify interrupt service routine |
| `@interruptSafe` | verified | function or method | none | permit call from ISR |
| `@noCopy` | nominal policy | nominal type | none | forbid implicit derived copy |
| `@noAlloc` | verified | function or method | none | forbid reachable allocation |
| `@noPanic` | verified | function or method | none | forbid reachable panic |
| `@noBlock` | verified | function or method | none | forbid reachable blocking |

---

## § 94 Final design summary

**§ 94(1)** Sec 0.1 uses a closed set of compiler-known attributes.

**§ 94(2)** Unknown attributes are errors.

**§ 94(3)** Attributes are semantic compiler constructs, not comments or macros.

**§ 94(4)** `@target` accepts:

```text
os
arch
cpu
device
board
```

**§ 94(5)** When `@target` is the first source-bearing line, it applies to the complete file.

**§ 94(6)** Otherwise it applies to the next top-level statement.

**§ 94(7)** `@when` selects the next top-level statement using typed compile-time boolean
configuration.

**§ 94(8)** Compiler options are not conditional-compilation flags.

**§ 94(9)** Sec has no textual preprocessor.

**§ 94(10)** `@address` binds one module-scope variable to volatile absolute storage.

**§ 94(11)** Target knowledge packs may provide named peripheral addresses.

**§ 94(12)** `@interrupt` binds a vector and always implies `@isr`.

**§ 94(13)** `@isr` may stand alone without vector binding.

**§ 94(14)** Target knowledge packs may provide named interrupt vectors.

**§ 94(15)** `@noCopy` forbids derived implicit copy on a nominal type.

**§ 94(16)** `@noAlloc`, `@noPanic`, and `@noBlock` are transitive compiler-verified
guarantees.

**§ 94(17)** `@interruptSafe` verifies functions callable from ISR code.

**§ 94(18)** Attributes do not require a Sec runtime.

**§ 94(19)** Selection occurs before active symbol-table construction and lowering.

**§ 94(20)** Mutually exclusive target variants may define one logical symbol.

**§ 94(21)** Overlapping active variants are compile errors.

**§ 94(22)** Public variant shapes must remain compatible.

**§ 94(23)** The attribute set may grow after each new attribute receives complete syntax,
target, semantic, diagnostic, formatter, and conformance rules.
