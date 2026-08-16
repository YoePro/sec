# Attributes

## Status

This document is the canonical attribute rulebook for Sec 0.1.

It defines:

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
- current implementation status;
- implementation requirements and tests.

Sec 0.1 uses a closed set of compiler-known attributes.

Unknown attributes are compile errors.

User-defined passive metadata attributes are not part of Sec 0.1.

---

# Design principle

Attributes are a first-class but deliberately limited part of the language.

They are used when a property:

```text
belongs to a source declaration
changes compilation, verification, placement, target selection, or lowering
is orthogonal to the declaration's ordinary grammar
does not justify a new declaration keyword
must be understood and validated by the compiler
```

Attributes are not arbitrary comments.

They are not a textual preprocessor.

They are not macros.

They are not runtime reflection metadata unless a future rule explicitly says
otherwise.

`static` is a storage-duration and type-association modifier, not a physical
placement mechanism. It does not by itself select a linker section, absolute
address, MMIO binding, memory space, or target-specific storage class. Physical
placement uses only mechanisms defined by this closed attribute set or by the
canonical storage, ABI, and platform rules. In particular, source must not
assume a conceptual `@section(...)` attribute unless it is separately added to
the compiler-known set with complete semantics.

---

# Initial compiler-known attribute set

Sec 0.1 initially defines:

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

The set may grow in later language versions.

Adding a new compiler-known attribute requires:

```text
a stable name
defined allowed targets
defined arguments
defined duplicate behavior
defined conflicts
defined semantic effect
defined diagnostics
defined formatter behavior
defined implementation status
```

---

# Closed attribute set

Only attributes known to the selected compiler version and language profile are
accepted.

Invalid:

```sec
@audit(category: "finance")
fn PostInvoice() void {
}
```

unless a future rule has registered `@audit` as a compiler-known attribute.

Expected diagnostic:

```text
unknown attribute @audit
```

Sec 0.1 does not preserve unknown attributes as passive metadata.

This avoids introducing syntax without a defined consumer.

Possible future consumers such as:

```text
compiler plugins
code generators
documentation generators
linters
framework metadata
reflection
binary metadata
```

must be designed before arbitrary user-defined attributes are introduced.

---

# Attribute categories

The initial attributes fall into four semantic categories.

## Selection attributes

```text
@target
@when
```

These decide whether a file or top-level statement participates in a concrete
compilation plan.

## Target-binding attributes

```text
@address
@interrupt
```

These bind a declaration to target-specific storage or interrupt machinery.

## Verified guarantee attributes

```text
@isr
@interruptSafe
@noAlloc
@noPanic
@noBlock
```

The compiler must verify the declared guarantee.

They are not unchecked programmer promises for ordinary Sec code.

## Nominal policy attributes

```text
@noCopy
```

These change a nominal type's semantic classification.

## Foreign-binding attributes

```text
@link_name("foreign-symbol")
```

`@link_name` is valid only on an `extern` declaration. It requires exactly one
non-empty string-literal argument and may appear at most once on a declaration.
It changes only the foreign/link symbol: the Sec name, calling convention,
safety, ownership, effects, dependency metadata, and ABI compatibility are
unchanged. Conflicting explicit symbols in one link domain are invalid.

---

# General syntax

An attribute begins with `@` followed by its compiler-known name.

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

Examples:

```sec
@isr
```

```sec
@address(0x40021000)
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

# Positional and named arguments

An attribute defines whether its arguments are:

```text
positional
named
either
```

The initial forms are:

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

Using the wrong argument form is a compile error.

---

# Compile-time arguments

Every initial attribute argument must be a compile-time expression.

Attribute evaluation may use only values available while constructing or
validating the compilation plan.

No initial attribute argument may require:

```text
runtime execution
allocation
I/O
mutable state
ordinary function calls
dynamic dispatch
foreign calls
```

Compiler-known target constants and configuration values are allowed where the
attribute defines them.

---

# Attribute attachment

An attribute normally applies to the next top-level statement.

Example:

```sec
@noPanic
fn Calculate() int {
    return 10
}
```

Multiple attributes may attach to the same statement:

```sec
@noAlloc
@noPanic
@noBlock
fn PollDevice() Result[State, DeviceError] {
    // ...
}
```

The attachment sequence ends at the attached statement.

An attribute does not modify the following attribute independently.

All consecutive attributes form one attribute set for the same statement.

---

# Comments and whitespace

Whitespace and comments between an attribute and its attached statement do not
change attachment.

Example:

```sec
@noPanic

// Public arithmetic entrypoint.
fn Add(left: int, right: int) Result[int, ArithmeticError] {
    let result := try left + right
    return Ok(result)
}
```

A new top-level statement ends the attribute set.

An attribute left without an attached statement before EOF is a compile error.

---

# Formatter layout

The canonical formatter writes one attribute per line.

```sec
@target(os: "linux")
@when(config.telemetry)
@noPanic
fn SendTelemetry() Result[void, TelemetryError] {
    // ...
}
```

The formatter preserves source order unless a future rule defines a canonical
semantic ordering.

The formatter must not remove redundant attributes automatically merely because
another attribute implies them.

Example:

```sec
@interrupt(vector: Interrupt.Timer0)
@noPanic
fn Timer0Handler() void {
}
```

`@interrupt` already implies `@noPanic` through `@isr`, but the explicit
attribute may remain as documentation.

---

# Allowed attachment level

The initial general attribute system applies at compilation-unit level.

An attribute may attach to a top-level statement allowed by that attribute.

Initial attributes do not attach to arbitrary local statements or expressions.

Invalid:

```sec
fn Work() void {
    @target(os: "linux")
    PerformLinuxOperation()
}
```

Local conditional compilation would affect:

```text
control flow
definite assignment
ownership
borrow analysis
unreachable-code analysis
formatting
debug information
```

It is deferred until explicitly designed.

---

# Attribute validation order

For a concrete compilation plan, the compiler processes attributes in this
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

Excluded source remains syntactically parsed.

It does not enter active Sema or lowering for the current plan.

---

# `@target`

## Purpose

`@target` selects source according to target identity.

It can apply to:

```text
a complete source file
or the next top-level statement
```

The position determines which.

---

# File-level `@target`

When `@target` is the first source-bearing line in a file, it applies to the
complete file.

Comments and a byte-order marker, where supported, may precede it.

Example:

```sec
@target(os: "linux", arch: "amd64")

module platform.linux.amd64
```

The file participates only in matching compilation plans.

A file-level `@target` must appear before:

```text
module declaration
import
type declaration
function
variable
or any other non-comment source statement
```

A later `@target` applies only to the next top-level statement.

---

# Statement-level `@target`

When `@target` is not the first source-bearing line, it applies only to the next
top-level statement.

Examples:

```sec
@target(os: "linux")
import "platform/linux"
```

```sec
@target(device: "controller-a")
@address(0x40010000)
let mut Device: DeviceRegisters
```

```sec
@target(os: "linux", arch: "amd64")
fn RawWrite(...) Result[int, WriteError] {
    // ...
}
```

Allowed statement targets include:

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

`@target` does not attach to a module declaration in Sec 0.1.

The module identity must remain stable across variants of one logical module.

---

# Target selectors

The initial selector names are:

```text
os
arch
cpu
device
board
```

Examples:

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

At least one selector is required.

Omitted selectors impose no restriction.

All supplied selectors must match.

The initial selector combination therefore uses logical AND.

---

# Target selector extensibility

Additional selector names may be added in later language or compiler versions.

Possible future selectors include:

```text
abi
cpuFeature
vendor
environment
objectFormat
```

They are not accepted merely because they appear in this list.

Unknown selector names are errors for the current compiler version.

The target-selector set is compiler-versioned and documented.

---

# Target terminology

The canonical distinctions are:

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

Examples:

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

Two devices using the same CPU core may have different:

```text
peripheral maps
interrupt vectors
memory maps
clock controllers
DMA channels
pin multiplexing
```

Device-specific addresses should therefore normally select by `device` or
`board`, not only by `cpu`.

---

# Selector values

String literals are accepted for target selectors.

A future target-identity constant type may also be accepted when explicitly
defined.

Selector comparison is exact after canonical target-name normalization.

The compiler must diagnose:

```text
unknown target value
value incompatible with selected architecture
device incompatible with board
CPU incompatible with device
contradictory selectors
```

Target aliases, if supported, belong to target knowledge packs.

---

# Multiple declaration variants

Declarations with the same logical symbol name may coexist when their selection
conditions are mutually exclusive.

Example:

```sec
@target(device: "controller-a")
@address(0x40010000)
let mut GPIO: GPIORegisters

@target(device: "controller-b")
@address(0x50020000)
let mut GPIO: GPIORegisters
```

For `controller-a`, only the first declaration is active.

For `controller-b`, only the second declaration is active.

---

# Overlapping variants

If two declarations with the same logical identity are active in one
compilation plan, compilation fails.

Example:

```sec
@target(os: "linux")
fn Open() void {
}

@target(os: "linux", arch: "amd64")
fn Open() void {
}
```

Both are active for Linux amd64.

Expected diagnostic shape:

```text
multiple active target variants define `Open`

active declarations:
    platform.sec:10
    platform.sec:15

compilation plan:
    os = linux
    arch = amd64
```

The compiler must not select the "more specific" declaration implicitly.

---

# Variant shape compatibility

Mutually exclusive variants of one public symbol must have compatible public
shape.

For functions:

```text
same name
same visibility
same generic parameter shape
same parameter types
same return type
compatible declared effects
compatible ABI contract
```

For variables:

```text
same name
same visibility
same declared type
same mutability
compatible ownership policy
```

For types:

```text
same public identity rules as defined by the type and module rulebooks
```

Target-specific bodies, addresses, vectors, calling-convention lowering, and
private implementation details may differ.

A target selector must not silently turn one public API into an unrelated API.

---

# No active variant

When no declaration variant is active, the symbol is unavailable for that
compilation plan.

A use should produce a target-aware diagnostic.

Example:

```text
`GPIO` is unavailable for device "controller-c"

available variants:
    device = "controller-a"
    device = "controller-b"
```

---

# Current `#target` compatibility form

The current compiler implementation recognizes:

```sec
#target(os: "linux", arch: "amd64")
```

as a file-level compiler directive.

The implemented form currently:

```text
must appear before code or declarations
accepts `os` and `arch`
requires both
applies to the file
```

The canonical unified source form defined by this rulebook is:

```sec
@target(...)
```

The implementation may temporarily accept `#target(...)` as compatibility
syntax during migration.

Compatibility behavior:

```text
#target remains file-level only
it does not become a statement-level selector
the formatter may offer an explicit migration fix
new selector support belongs to @target
ordinary formatting must not silently rewrite semantics
```

The compatibility form should be removed only through the normal language
deprecation process.

---

# `@when`

## Purpose

`@when` conditionally includes the next top-level statement according to typed
compile-time program configuration.

Example:

```sec
@when(config.telemetry)
fn SendTelemetry() Result[void, TelemetryError] {
    // ...
}
```

`@when` does not create runtime branching.

The declaration either participates in the compilation plan or does not.

---

# Compile-time parameters

Compile-time parameters originate from the project build configuration.

They are:

```text
typed
validated
part of compilation-plan identity
part of build-cache identity
not compiler options
not text macros
not runtime configuration
```

Canonical source access for boolean configuration is:

```sec
config.telemetry
```

Example manifest concept:

```toml
[parameters]
telemetry = false
audit = true
```

Example source:

```sec
@when(config.audit)
fn RecordAuditEntry(...) Result[void, AuditError] {
    // ...
}
```

---

# `@when` condition language

Sec 0.1 permits a restricted compile-time boolean expression.

Allowed forms:

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

Examples:

```sec
@when(config.audit)
```

```sec
@when(!config.telemetry)
```

```sec
@when(config.audit && config.databaseLogging)
```

Target identity belongs in `@target`, not in `@when`, for Sec 0.1.

Preferred:

```sec
@target(os: "linux")
@when(config.audit)
fn InstallAuditIntegration() void {
}
```

Not canonical in Sec 0.1:

```sec
@when(config.audit && target.os == "linux")
```

This separation keeps target selection and program configuration distinct.

---

# Compiler options are not `@when` inputs

Compiler options do not control declaration existence.

Invalid conceptual inputs include:

```text
optimization level
debug-information level
warning level
LTO
strip setting
bounds-check lowering policy
overflow-check lowering policy
```

Changing debug or optimization configuration must not silently change the
program's public source graph.

Build features use typed compile-time parameters instead.

---

# No textual preprocessor

Sec does not provide C-style:

```text
#define
#ifdef
#ifndef
-DNAME
text substitution
token concatenation
conditional lexical regions
```

All source remains parseable.

Conditional inclusion is declaration-aware and typed.

---

# Excluded source

A file or statement excluded by `@target` or `@when`:

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

Syntax errors remain syntax errors even in excluded source.

This prevents invalid source from being hidden indefinitely behind a selector.

Target-dependent semantic errors are evaluated only when the source is active.

---

# `@address`

## Purpose

`@address` binds one module-scope storage declaration to a compile-time address.

Canonical:

```sec
@address(0x40021000)
let mut GPIO: GPIORegisters
```

The current parser already recognizes `@address(...)` before a single `let`
declaration.

---

# Address semantics

`@address(value)` means:

```text
the storage exists at the supplied address
the declaration does not allocate ordinary storage
reads have volatile semantics
mut controls whether writes are permitted
the address is part of target binding
the declaration has address-stability restrictions
ordinary relocation does not apply to the storage
```

`@address` implies volatile access.

A separate `@volatile` is not required for addressed storage.

---

# Address argument

The argument must be a compile-time address expression.

Allowed initial forms:

```text
integer address literal
compiler-known typed peripheral address constant
target knowledge-pack address constant
other explicitly defined compile-time address constant
```

Examples:

```sec
@address(0x40021000)
let mut GPIO: GPIORegisters
```

```sec
@address(Peripheral.GPIOA)
let mut GPIOA: GPIORegisters
```

The compiler validates:

```text
representability in target pointer width
alignment for declared type
target address-space legality
required privilege
known peripheral availability where named
overlap with other absolute declarations where detectable
```

---

# Address declaration target

`@address` applies to exactly one module-scope `let` declaration.

It does not apply to:

```text
grouped declarations
local variables
parameters
fields
temporary expressions
functions
types
```

Invalid:

```sec
@address(0x40021000)
let mut first: Register, second: Register
```

The declaration must name exactly one storage object.

---

# Address and initialization

Addressed storage is not initialized like ordinary allocated storage.

The declaration must not generate ordinary zero initialization or copy an
initializer into the hardware address unless a specialized rule explicitly
permits initialization.

The type's default value does not imply a startup write.

Read and write behavior follows:

```text
declared type
mutability
volatile semantics
register rules
target knowledge
```

---

# Target-dependent addresses

Use mutually exclusive declaration variants.

```sec
@target(device: "controller-a")
@address(0x40010000)
let mut GPIO: GPIORegisters

@target(device: "controller-b")
@address(0x50020000)
let mut GPIO: GPIORegisters
```

All attributes in each block attach to that declaration.

An attribute does not scope another attribute.

---

# Knowledge-pack addresses

A target knowledge pack may define typed names:

```sec
@address(Peripheral.GPIOA)
let mut GPIOA: GPIORegisters
```

The knowledge pack may provide:

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

Raw numeric addresses remain allowed for:

```text
custom targets
new ports
application-specific hardware
external memory-mapped devices
early platform development
```

---

# `@interrupt`

## Purpose

`@interrupt` binds a function to an interrupt vector and implies `@isr`.

Canonical:

```sec
@interrupt(vector: Interrupt.Timer0)
fn Timer0Handler() void {
}
```

The implication is normative:

```text
@interrupt(...)
    implies @isr
```

The reverse does not apply.

---

# Interrupt vector argument

`@interrupt` requires one named argument:

```text
vector
```

Examples:

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

The value must be a compile-time vector identity accepted by the selected
target.

---

# Named interrupt vectors

Target knowledge packs may expose typed vector constants.

Examples:

```text
Interrupt.Reset
Interrupt.NMI
Interrupt.HardFault
Interrupt.Timer0
Interrupt.UART1
```

The knowledge pack owns target facts such as:

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

Named vectors are preferred when available.

Raw numeric vectors remain allowed for custom targets and new ports.

---

# Interrupt validation

The compiler validates:

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

Duplicate active vector binding is a compile error.

---

# Target-dependent interrupt binding

Use mutually exclusive function variants when binding differs.

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

The public function shape must remain compatible.

---

# `@isr`

## Purpose

`@isr` declares and verifies an interrupt service routine without automatically
binding it to a vector.

```sec
@isr
fn SharedInterruptHandler() void {
}
```

Use cases include:

```text
external startup code performs vector binding
manual vector tables
shared interrupt entry functions
handlers exported through FFI
testing ISR-compatible code
targets with nonstandard binding
```

---

# Relationship between `@interrupt` and `@isr`

The relationship is:

```text
@interrupt is the stronger construct
@interrupt includes vector binding
@interrupt includes all @isr verification
@isr may stand alone
@isr does not choose a vector
```

In set terms:

```text
the guarantees of @isr are a subset of the semantics of @interrupt
```

---

# ISR verification

`@isr` implies at least:

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

The detailed ISR analysis belongs to the call-graph, stack-analysis, allocation,
blocking, ownership, and concurrency rulebooks.

---

# ISR ABI

The compiler or target knowledge pack determines:

```text
entry convention
saved registers
return instruction
stack alignment
privilege transition
interrupt masking assumptions
vector-table representation
```

The source function remains a Sec function declaration with compiler-verified
ISR constraints.

A target may reject `@isr` when it lacks a defined ISR ABI.

---

# `@interruptSafe`

## Purpose

`@interruptSafe` declares that a function may be called from ISR code.

```sec
@interruptSafe
fn ReadStatus() Status {
    // ...
}
```

It does not make the function an ISR.

It does not bind a vector.

---

# Interrupt-safe guarantee

The compiler verifies at least:

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

An `@isr` call graph may call:

```text
other @isr-specific compiler operations
@interruptSafe functions
compiler-proven equivalent functions
```

It may not call an ordinary unverified function merely because the current body
appears simple.

---

# `@noCopy`

## Purpose

`@noCopy` is a nominal type policy.

```sec
@noCopy
type SessionID struct {
    value: uint64
}
```

It explicitly forbids compiler-derived implicit copy.

---

# `@noCopy` semantics

For an `@noCopy` type:

```text
ordinary implicit copy is forbidden
explicit ownership transfer may remain valid
named Copy, Clone, Duplicate, Snapshot, or ToOwned methods remain ordinary APIs
copyability is distinct from movability
copyability is distinct from relocatability
copyability is distinct from pinning
```

Example invalid copy:

```sec
let second := first
```

Example possible move:

```sec
let second :<- first
```

The move is valid only when ownership, lifetime, relocation, and pinning rules
also permit it.

---

# `@noCopy` allowed targets

`@noCopy` applies to nominal type declarations.

Initial allowed forms include:

```text
named struct
named enum where policy is meaningful
named union
named register
named underlying type
other nominal user-defined types
```

It does not apply to:

```text
individual variables
fields
parameters
functions
anonymous types
type references
```

A variable cannot override the copy policy of its type.

---

# `@noCopy` and fields

A containing type becomes non-copyable when it stores an `@noCopy` field unless
a separate rule explicitly uses indirection with copyable semantics.

The compiler should diagnose the cause chain.

Example:

```text
Container cannot be copied
  -> field `session` has type SessionID
  -> SessionID explicitly forbids copy through @noCopy
```

---

# No positive copy attribute in the initial subset

Sec 0.1 does not initially define:

```text
@copy
@copyable
```

Derived copyability remains automatic when the compiler proves it.

A future positive attribute may mean:

```text
compilation fails unless this type remains compiler-derivably copyable
```

It must never mean:

```text
invoke arbitrary hidden user copy code
```

---

# `@noAlloc`

## Purpose

`@noAlloc` is a compiler-verified transitive guarantee.

```sec
@noAlloc
fn Process(buffer: ref byte[]) Result[void, ProcessError] {
    // ...
}
```

It means no reachable execution path performs allocation.

---

# Allocation definition

Allocation includes acquiring new dynamic storage through:

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

The allocation rulebook owns the exact classification.

---

# What is not allocation

The following are not allocation merely by existing:

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

A specific operation may still allocate according to its implementation and
declared effects.

---

# `@noAlloc` verification

The compiler verifies:

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

Unknown allocation behavior violates the guarantee.

---

# `@noAlloc` and fallible allocation

Using `try` does not make allocation compatible with `@noAlloc`.

Example:

```sec
let value := try allocator.New[Value]()
```

This is panic-free but still allocates.

`@noAlloc` forbids the operation.

---

# `@noPanic`

## Purpose

`@noPanic` is a compiler-verified transitive guarantee.

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

Its complete semantics are defined by:

```text
panic.md
runtime_checks.md
```

---

# `@noPanic` guarantee

The function has no reachable:

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

The compiler verifies the complete reachable call graph.

---

# `@noPanic` is not exception suppression

`@noPanic` does not catch panic.

It requires the panic effect to be absent.

Expected failures use explicit result values and `try`.

---

# `@noPanic` and proof

Ordinary checked operations may remain in `@noPanic` code when their failure is
proven impossible.

Otherwise the code must:

```text
use try
handle or propagate an explicit error
select an explicit total semantic
change the types or contracts so safety is proven
```

---

# `@noBlock`

## Purpose

`@noBlock` is a compiler-verified transitive guarantee.

```sec
@noBlock
fn PollStatus() Status {
    // ...
}
```

It means no reachable execution path waits for an unbounded external event or
parks the current execution domain.

---

# Blocking operations

Blocking includes operations such as:

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

The concurrency and platform rulebooks own detailed classification.

---

# What `@noBlock` does not automatically forbid

`@noBlock` does not by itself forbid:

```text
ordinary finite computation
statically bounded loops
nonblocking polling
try-lock
bounded lock-free retry where separately proven
reading memory-mapped status
```

Execution-time and worst-case latency guarantees are separate analyses.

A function can be nonblocking yet take too long for an ISR.

---

# `@noBlock` verification

The compiler verifies:

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

Unknown blocking behavior violates the guarantee.

---

# Allowed targets for verified effects

The initial allowed targets for:

```text
@noAlloc
@noPanic
@noBlock
@interruptSafe
@isr
@interrupt
```

are functions and methods.

`@interrupt` and `@isr` have additional target-specific signature rules.

Attributes on lambdas and function types are deferred until function-effect type
syntax is finalized.

Attributes on complete `impl` blocks are not part of the initial subset.

Each method must declare or derive its own guarantee.

---

# Sec code versus foreign declarations

For an ordinary Sec function with a body, verified attributes are proven by the
compiler.

For foreign declarations, the compiler normally cannot prove the foreign
implementation.

On an extern declaration, an applicable effect-guarantee attribute is a trusted
foreign contract rather than a compiler proof of the foreign body. The compiler
must preserve this trust provenance and treat unspecified foreign effects
conservatively.

```text
unknown foreign effects violate noPanic, noAlloc, noBlock, and interruptSafe
extern guarantees are usable only as explicitly trusted foreign facts
```

---

# Attribute implications

The initial implication graph is:

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

An implied attribute does not require duplicated AST storage as though the user
wrote it.

The compiler records effective guarantees separately from explicitly written
attributes.

---

# Duplicate attributes

An attribute may appear at most once in one attachment set unless its definition
explicitly permits repetition.

Invalid:

```sec
@noCopy
@noCopy
type Identity struct {
    value: uint64
}
```

Invalid:

```sec
@address(0x40000000)
@address(0x50000000)
let mut Device: DeviceRegisters
```

Invalid:

```sec
@target(os: "linux")
@target(arch: "amd64")
fn Open() void {
}
```

Write one target attribute:

```sec
@target(os: "linux", arch: "amd64")
fn Open() void {
}
```

---

# Duplicate arguments

An argument name may appear only once in one attribute.

Invalid:

```sec
@target(os: "linux", os: "windows")
fn Open() void {
}
```

Invalid:

```sec
@interrupt(vector: 10, vector: 20)
fn Handler() void {
}
```

---

# Conflicting attributes

The compiler rejects attributes whose semantics conflict.

Examples include:

```text
two absolute addresses
two interrupt vectors
contradictory target selectors
future positive copy requirement with @noCopy
a verified effect contradicted by the declaration's required behavior
```

The diagnostic should show both source locations and the conflict rule.

---

# Redundant attributes

An explicitly written attribute may be redundant because another implies it.

Example:

```sec
@interrupt(vector: Interrupt.Timer0)
@isr
@noPanic
@noAlloc
@noBlock
fn Timer0Handler() void {
}
```

This is valid.

The compiler may emit an informational diagnostic if configured.

The formatter does not delete the redundant declarations automatically.

---

# Attribute order

Initial attribute semantics are order-independent within one attachment set.

These are semantically equivalent:

```sec
@target(device: "controller-a")
@address(0x40010000)
let mut Device: DeviceRegisters
```

```sec
@address(0x40010000)
@target(device: "controller-a")
let mut Device: DeviceRegisters
```

The first style is recommended because selection precedes binding conceptually.

The formatter may later establish a canonical category order, but ordinary
formatting must not change attachment or semantics.

---

# Parser representation

Attributes should be represented explicitly.

Conceptual AST:

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

A top-level statement should carry:

```go
Attributes []Attribute
```

File-level attributes should be represented on the compilation unit or program.

Do not encode each new attribute solely through an unrelated special-case field.

Specialized semantic fields may be derived after generic attribute parsing.

---

# Semantic representation

Sema should resolve attributes into compiler-known semantic properties.

Conceptual:

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

The original syntax remains available for diagnostics, formatter, and LSP.

---

# Selection before active symbols

`@target` and `@when` selection occurs before active symbol-table construction.

This permits mutually exclusive definitions of one logical symbol.

The compiler must still parse all variants before selection.

---

# Inactive declarations and name conflicts

Inactive declarations do not conflict with active declarations for the current
plan.

However, the compiler should perform cross-variant validation to detect:

```text
overlapping selectors
incompatible public shapes
unreachable variants
unknown target values
invalid configuration references
```

This validation may operate across all declared compilation variants.

---

# LSP behavior

The LSP should expose:

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

Excluded source remains visible.

Example hover:

```text
Excluded from current plan

Requires:
    device = "controller-a"

Current:
    device = "controller-b"
```

---

# LSP completion

After `@`, completion offers only compiler-known attributes valid at the current
attachment location.

Examples:

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

Attribute argument completion may offer:

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

# Formatter behavior

The formatter:

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

Example:

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

# Diagnostics

Suggested diagnostic families:

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

Final stable IDs belong to the diagnostics registry.

---

# Current implementation status

## Implemented

### `#target` parser directive

The current parser recognizes file-level:

```sec
#target(os: "linux", arch: "amd64")
```

It currently:

```text
must appear before code or declarations
accepts os and arch
requires both
requires string literal values
rejects duplicate arguments
stores OS and architecture in AST
```

### `@address` parser special case

The current parser recognizes:

```sec
@address(expression)
let ...
```

It currently:

```text
requires a following let declaration
rejects grouped let declarations
stores address expression and token on the LetStatement
```

### Compilation-plan concepts

The project rulebook already distinguishes:

```text
OS
architecture
ABI
CPU
CPU features
compiler options
compile-time parameters
variant-specific source selection
```

Compile-time parameters are already specified as typed values rather than text
macros.

### Existing semantic foundations

Other rulebooks already define or require:

```text
addressed storage implies volatile semantics
mut controls writes
noPanic as a transitive verified property
defer and destructors are noPanic
noCopy semantics
ISR call-graph restrictions
allocation and blocking analysis
```

### `@noCopy` frontend path

The current frontend implements:

```text
an Attribute AST node with source tokens
@noCopy parsing on nominal type and enum declarations
argument rejection
duplicate rejection with both source locations
wrong-target rejection
explicit non-copyable type classification
preservation through generic instantiation
derived non-copyability through aggregate fields
cause-aware copy diagnostics
explicit ownership transfer through the existing move rules
formatter placement on its own line
LSP semantic-token modifier classification
```

---

# Partly implemented

```text
target-specific source selection exists conceptually but the canonical @target
attribute is not generally parsed

absolute-address syntax is parsed but not through a general attribute AST

address validation and full lowering may be incomplete

the `@noCopy` vertical is implemented, but consecutive mixed attribute sets and
the general attachment engine are not

call-graph analysis foundations exist, but all noAlloc/noPanic/noBlock
attributes are not fully parsed and verified

ISR rules exist conceptually, but @isr and @interrupt general parsing and target
knowledge integration are incomplete

compile-time parameters exist in project rules, but canonical config.<name>
source access and @when selection are not fully implemented
```

---

# Not implemented

Unless newer repository code proves otherwise, the following remain to be
implemented:

```text
general attribute parser
attribute attachment sets
canonical @target file form
canonical @target statement form
os/arch/cpu/device/board selector support
target-variant overlap analysis
public shape compatibility across variants
@when
config.<name> compile-time parameter access
@interrupt
@isr
@interruptSafe
@noAlloc
@noPanic
@noBlock
attribute implication graph
knowledge-pack peripheral constants
knowledge-pack interrupt vector constants
generic attribute diagnostics
general attribute-aware formatter beyond `@noCopy`
attribute-aware LSP completion
```

---

# Required tests

Create or update:

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

# Target test matrix

Test selectors individually:

```text
os
arch
cpu
device
board
```

Test combinations.

Test:

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

# Conditional compilation tests

Test:

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

# Address tests

Test:

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

# Interrupt tests

Test:

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

# Effect tests

For each verified effect, test:

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

# Binary and lowering tests

Verify:

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

# Required synchronization

This document must remain synchronized with:

```text
grammar.md
lexical_structure.md
parser_recovery.md
projects.txt
copy_move.md
ownership.md
runtime_checks.md
panic.md
allocation.txt
defer.txt
destruction.txt
inline_assembly.md
platform/ffi.md
functions.md
declarations/interfaces.md
impl.md
generics rulebook
registers rulebook
addressed-variable rulebook
interrupt and ISR rulebooks
compiler_pipeline.txt
semantic_ir.txt
formatter.md
lsp.md
diagnostics.txt
language-rulebook-status.md
rules_implementations.txt
```

---

# Appendix A — Codex implementation plan

## A.1 Add the rulebook

Add:

```text
rules/foundations/attributes.md
```

Update:

```text
language-rulebook-status.md
rules/compiler/rules_implementations.txt
```

Mark the rulebook Written.

Do not mark all attributes implemented.

---

## A.2 Add generic attribute AST

Add compiler-neutral syntax representation.

Conceptual:

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

Attach attributes to top-level statements and the compilation unit.

---

## A.3 Parse attribute sets

When parser sees `@`:

1. parse one or more consecutive attributes;
2. preserve comments and source ranges;
3. identify file-level `@target` when first source-bearing line;
4. parse the next top-level statement;
5. attach the complete set;
6. diagnose unattached attributes.

Replace the `@address`-only parser branch with generic parsing.

Preserve existing behavior through semantic validation.

---

## A.4 Migrate `#target`

Continue accepting the current directive temporarily.

Lower it into the same file target-selection representation as `@target`.

Add a compatibility diagnostic and explicit migration fix.

Do not create two independent selection engines.

---

## A.5 Implement target selectors

Support:

```text
os
arch
cpu
device
board
```

Require at least one.

Require compile-time values.

Validate through the compilation plan and target database.

---

## A.6 File and statement selection

Implement:

```text
first source-bearing @target
    file selection

later @target
    next top-level statement selection
```

Selection occurs before active symbol-table construction.

---

## A.7 Variant analysis

Detect:

```text
overlap
no active variant where referenced
public shape mismatch
invalid selector combination
unreachable target variant
```

Do not use implicit specificity ranking.

---

## A.8 Implement compile-time configuration

Expose typed project parameters through:

```sec
config.name
```

Implement restricted boolean constant evaluation for `@when`.

Do not expose compiler options as program configuration.

---

## A.9 Implement `@address`

Resolve one module-scope `let`.

Validate:

```text
compile-time address
alignment
target pointer width
address space
single declaration
no ordinary initialization
volatile semantics
mutability
```

Integrate target knowledge-pack address constants.

---

## A.10 Implement `@noCopy`

Status: implemented for parser/AST, Sema classification and diagnostics,
generic-instance preservation, formatter line layout, and LSP semantic-token
classification. Migration into the future general attachment engine remains.

Apply explicit nominal non-copy policy.

Reuse compiler copyability classification.

Improve cause-aware diagnostics.

Do not add user-defined hidden copy bodies.

---

## A.11 Implement verified effects

Add parser and semantic support for:

```text
@noAlloc
@noPanic
@noBlock
@interruptSafe
@isr
```

Reuse call graph, stack, allocation, panic, blocking, ownership, and ISR analyses.

---

## A.12 Implement `@interrupt`

Resolve the vector through:

```text
raw compile-time number
or target knowledge-pack constant
```

Imply `@isr`.

Validate target ABI and unique active binding.

Generate or contribute to the target vector table.

---

## A.13 Attribute implications

Compute effective properties separately from explicit source attributes.

Example:

```text
explicit @interrupt
effective @isr
effective @noPanic
effective @noAlloc
effective @noBlock
```

Diagnostics should distinguish explicit and implied requirements.

---

## A.14 Formatter

Format one attribute per line.

Format long named argument lists vertically.

Preserve explicit redundant guarantees.

Do not alter selection semantics.

---

## A.15 LSP

Add:

```text
completion
hover
effective guarantee display
excluded-source display
target value completion
configuration completion
vector and address completion
call-chain diagnostics
code actions
```

---

## A.16 Tests and migration

Migrate old `#target` and `@address` tests into the general framework while
retaining compatibility coverage.

Run:

```text
go test ./...
compiler build
LSP build
formatter tests
fixture validation
target matrix tests
```

---

# Appendix B — Canonical initial attribute table

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

# Final design summary

Sec 0.1 uses a closed set of compiler-known attributes.

Unknown attributes are errors.

Attributes are semantic compiler constructs, not comments or macros.

`@target` accepts:

```text
os
arch
cpu
device
board
```

When `@target` is the first source-bearing line, it applies to the complete file.

Otherwise it applies to the next top-level statement.

`@when` selects the next top-level statement using typed compile-time boolean
configuration.

Compiler options are not conditional-compilation flags.

Sec has no textual preprocessor.

`@address` binds one module-scope variable to volatile absolute storage.

Target knowledge packs may provide named peripheral addresses.

`@interrupt` binds a vector and always implies `@isr`.

`@isr` may stand alone without vector binding.

Target knowledge packs may provide named interrupt vectors.

`@noCopy` forbids derived implicit copy on a nominal type.

`@noAlloc`, `@noPanic`, and `@noBlock` are transitive compiler-verified
guarantees.

`@interruptSafe` verifies functions callable from ISR code.

Attributes do not require a Sec runtime.

Selection occurs before active symbol-table construction and lowering.

Mutually exclusive target variants may define one logical symbol.

Overlapping active variants are compile errors.

Public variant shapes must remain compatible.

The attribute set may grow after each new attribute receives complete syntax,
target, semantic, diagnostic, formatter, and implementation rules.
