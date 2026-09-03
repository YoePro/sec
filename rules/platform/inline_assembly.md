# Sec Inline Assembly

- **Status:** Normative
- **Created:** 2026-09-03
- **Last updated:** 2026-09-03
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `998d8d1`
- **Canonical path:** `rules/platform/inline_assembly.md`
- **Related rulebooks:** `rules/memory/unsafe.md`, `rules/platform/platform_model.md`, `rules/platform/target_profiles.md`, `rules/platform/volatile.md`, `rules/platform/hardware-register-access.md`, `rules/platform/interrupts.md`, `rules/platform/ffi.md`, `rules/memory/memory_model.md`, `rules/memory/raw_pointers.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/copy_move.md`, `rules/memory/destruction.md`, `rules/analysis/effect_analysis.md`, `rules/analysis/isr_analysis.md`, `rules/analysis/stack_analysis.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/linking.md`, `rules/mlir/sec_mlir.md`

## 1. Purpose

This rulebook defines source-level inline assembly semantics in Sec 0.1.

Inline assembly is an explicit unsafe target escape hatch for machine operations
that cannot be expressed adequately through ordinary Sec operations or a
compiler/platform-known primitive.

This rulebook owns:

```text
the Sec inline-assembly boundary;
assembly templates;
typed machine operands;
fixed-register and target operand constraints;
outputs and clobbers;
assembly observability;
memory/external/machine-state effects at the assembly boundary;
control-flow and stack boundary rules;
symbol dependencies and relocation integration;
target validation;
privilege and execution-environment constraints;
system-call/trap/environment-transition integration;
Semantic IR / Sec MLIR preservation;
backend materialization requirements;
diagnostics, tooling, testing, and completion criteria.
```

This rulebook does not redefine:

```text
target applicability syntax such as file/declaration target selection;
the canonical unsafe-context syntax;
Sec type validity generally;
ownership or borrowing generally;
the canonical concurrency memory model;
volatile physical-storage semantics;
hardware-register transaction planning;
interrupt lifecycle or ISR semantics;
foreign ABI semantics;
stack-analysis algorithms;
link reachability or binary symbol identity;
platform registry syntax;
Sec MLIR or LLVM IR as source-language syntax.
```

Those concerns remain owned by their canonical rulebooks.

---

## 2. Core principle

Inline assembly escapes Sec's ordinary instruction vocabulary.

It does not escape Sec's compiler model.

The programmer may directly express target machine instructions while the
compiler continues to own the surrounding:

```text
type semantics;
unsafe obligations;
ownership;
borrowing;
effects;
control flow;
cleanup;
stack model;
target selection;
execution context;
concurrency;
interrupt constraints;
link dependencies;
diagnostics.
```

Where the compiler lacks a declared semantic contract for arbitrary assembly,
it remains conservative rather than inventing correctness-critical knowledge
from the assembly text.

---

# Part I — Source boundary and target model

## 3. Inline assembly is target machine assembly

The template inside an `asm` operation contains real assembly for the selected
target.

Example:

```sec
unsafe {
    asm {
        "syscall"

        inputs:
            rax(number)

        outputs:
            rax(result)

        clobbers:
            rcx
            r11
            memory
    }
}
```

`syscall`, `rax`, `rcx`, and `r11` are target assembly/register concepts.

They are not Sec variables and are not LLVM IR syntax.

---

## 4. Inline assembly is not Sec MLIR

Sec does not define ordinary source syntax for injecting Sec MLIR.

Conceptually:

```text
Sec source inline assembly
    !=
direct Sec MLIR injection
```

Compiler tests and development tooling may accept direct Sec MLIR fixtures
without making MLIR a stable source-language interface.

---

## 5. Inline assembly is not LLVM IR

Sec does not define ordinary source syntax for direct LLVM IR injection.

Conceptually:

```text
Sec source inline assembly
    !=
LLVM IR
```

LLVM IR remains a compiler/backend representation.

Compiler tests may use LLVM IR fixtures directly.

---

## 6. Backend use of LLVM inline assembly

An LLVM-based backend may lower Sec inline assembly through LLVM inline-assembly
facilities.

Conceptually:

```text
Sec source asm
        ↓
resolved Sec inline-assembly operation
        ↓
Semantic IR
        ↓
Sec MLIR / target lowering
        ↓
LLVM inline asm
        ↓
target assembler / machine encoding
```

LLVM constraint strings, LLVM dialect flags, and LLVM symbol spelling are
backend implementation details.

They are not the normative Sec source contract.

---

## 7. `sec/core` and `sec/platform`

Compiler-shipped `sec/core` and `sec/platform` code may legitimately use inline
assembly to implement:

```text
system-call primitives;
special CPU operations;
startup glue;
interrupt/controller glue;
target ABI adapters;
runtime-free platform operations;
hardware operations;
compiler/platform-known primitives.
```

Their special role does not disable ordinary Sec semantics.

They remain subject to applicable:

```text
type checks;
unsafe rules;
ownership;
borrowing;
effect analysis;
ISR rules;
target validation;
declared clobbers;
stack rules;
linking rules.
```

Their privilege is in purpose and trusted platform contracts, not exemption from
the language model.

---

## 8. Target applicability syntax is owned elsewhere

This rulebook does not redefine `#target`, `@target`, or other target-selection
scope syntax.

Inline assembly is always validated against the already resolved
`CompilationPlan`.

An asm block does not select its own target.

---

## 9. Cross-compilation uses the target, never the host

Inline assembly is interpreted for the selected build target.

For example:

```text
compiler host:
    linux/amd64

build target:
    baremetal/cortex-m4
```

means the assembly is ARM/Cortex-M assembly according to the resolved target
contract.

The host assembler, host register set, host ABI, or host architecture must not
silently determine Sec inline-assembly semantics.

---

## 10. Canonical assembly dialect

Sec 0.1 uses the canonical assembler dialect resolved by the selected target.

An ordinary asm block does not independently select another assembler dialect.

Dialect-switch directives inside ordinary inline assembly must not be used to
circumvent the target's canonical dialect.

---

## 11. Real assembly is allowed

Inline assembly may consist of one or multiple real target instructions.

It may contain internal target-assembly control flow and local labels where the
target assembler supports them.

The exact string-literal/multiline-literal grammar is owned by the canonical
grammar rulebook.

The structured block form used by this rulebook is canonical for describing the
assembly contract.

Any additional shorthand accepted by the grammar must lower to the same
semantics.

---

# Part II — Unsafe boundary

## 12. Inline assembly is an unsafe operation

Inline assembly is compiler-classified as unsafe.

It requires the explicit unsafe context defined by `rules/memory/unsafe.md`.

Being inside an `unsafe fn` does not by itself create an unsafe context.

Example:

```sec
unsafe fn RawOperation(value: uint64) uint64 {
    unsafe {
        asm {
            "..."

            inputs:
                rax(value)

            outputs:
                rax(result)
        }
    }

    return result
}
```

The exact unsafe surface syntax remains owned by the unsafe rulebook.

---

## 13. Unsafe does not disable analysis

An unsafe asm operation does not disable:

```text
type checking;
ownership checking;
borrow checking;
effect analysis;
stack analysis;
ISR analysis;
target validation;
link dependency tracking;
control-flow validation;
cleanup requirements.
```

`unsafe` accepts the specific proof obligations of the assembly boundary.

It does not mean "trust everything around this operation".

---

## 14. Unsafe does not grant capability or privilege

Unsafe does not grant:

```text
kernel privilege;
supervisor privilege;
hardware authority;
mapped memory;
interrupt authority;
CPU features;
target ISA extensions;
address-space access.
```

Known-invalid target operations remain invalid inside unsafe code.

---

# Part III — Assembly contract

## 15. Assembly text and compiler-visible contract

An asm operation consists conceptually of:

```text
target assembly template
+
compiler-visible Sec contract
```

The compiler-visible contract may describe:

```text
inputs;
outputs;
physical operand constraints;
clobbers;
memory effects;
external effects;
machine-state effects;
control-flow behavior;
stack effects;
symbol dependencies;
target requirements;
execution-environment requirements;
stronger compiler/platform-known semantics.
```

The assembler text is not itself the complete compiler contract.

---

## 16. Correctness-critical semantics are not inferred from mnemonics

The compiler must not generally infer facts such as:

```text
pure;
atomic;
Acquire;
Release;
interrupt masking;
hardware completion;
no blocking;
no fault;
```

merely because the assembly template contains a recognized instruction name.

Compiler/core/platform primitives may carry stronger explicit contracts.

Raw user assembly without such a contract remains opaque for those properties.

---

# Part IV — Inputs and outputs

## 17. Inputs are Sec expressions

An input operand contains an ordinary Sec expression.

Example:

```sec
inputs:
    rdi(argument)
```

The input expression:

1. is type-checked under ordinary Sec rules;
2. is evaluated exactly once;
3. is evaluated before the assembly operation;
4. is materialized according to the declared target operand constraint.

Backend register allocation must not duplicate evaluation of a Sec expression.

---

## 18. Fixed registers are physical constraints

In:

```sec
inputs:
    rax(number)
```

`rax` identifies a canonical target register constraint.

It means the input representation must occupy that physical register at the
assembly boundary.

It does not make `rax` a Sec variable.

---

## 19. Register classes and target operand classes

Targets may provide operand constraints equivalent to:

```text
FixedRegister;
RegisterClass;
Immediate;
MemoryOperand;
AddressOperand;
TargetDefined.
```

The exact source spelling is target/compiler API design.

The Semantic IR representation must not reduce these concepts prematurely to
opaque LLVM constraint strings.

---

## 20. Constraint names are target-owned

Registers and operand classes resolve against the selected target.

For example, an x86-64 register such as:

```text
rax
```

is invalid for an AArch64 CompilationPlan.

The target model, not LLVM source spelling, is the canonical authority.

---

## 21. Immediate operands

A target may require a compile-time immediate or restrict an immediate to a
specific width/range.

The compiler should diagnose known immediate-contract violations before final
machine-code emission where practical.

---

# Part V — Direct operand types

## 22. Machine-compatible Sec 0.1 operand types

Sec 0.1 intentionally keeps the direct register/value asm boundary small.

Direct machine operands are limited to:

```text
int8 / uint8
int16 / uint16
int32 / uint32
int64 / uint64
int128 / uint128 when the target operand class supports that width

float32 / float64 when the target operand class supports that representation

RawPtr[T] when the target operand supports the corresponding raw address width
and address-space representation
```

A target may define additional primitive machine operand categories in a future
revision.

---

## 23. Exact logical operand width

A fixed-width assembly operand requires a Sec integer operand of the same logical
bit width.

Inline assembly performs no implicit:

```text
integer widening;
integer narrowing;
sign extension;
zero extension.
```

Example:

```text
32-bit register operand
    requires 32-bit Sec integer

64-bit register operand
    requires 64-bit Sec integer
```

If conversion is intended, the programmer performs an explicit Sec conversion
before crossing the asm boundary.

---

## 24. Register container width does not change logical width

A target may define overlapping or partially written register forms.

The target owns the relationship among those machine registers.

The Sec operand still matches the logical operand width selected by the assembly
constraint.

Target rules may additionally define whether a physical write:

```text
preserves upper bits;
zeros upper bits;
leaves upper bits unspecified;
defines a complete physical register.
```

Those machine-state rules do not create implicit Sec type conversions.

---

## 25. Target-sized `int` and `uint`

Target-sized `int` and `uint` are not direct fixed-width asm register operands
in Sec 0.1 merely because their current resolved width happens to match a
register.

Low-level asm boundaries use the explicit fixed-width integer type required by
the machine/ABI contract.

This keeps the assembly boundary explicit across target changes.

---

## 26. User-defined and high-level types are not direct register operands

Sec 0.1 does not directly pass the following through ordinary register asm
operands:

```text
string;
bool;
char;
rune;
user-defined nominal types;
constrained types;
enum;
struct;
union;
array;
list;
map;
set;
vector;
matrix;
tensor;
tensor_view;
ref T;
ref mut T;
owned resources;
interfaces;
closures.
```

Being a Sec built-in type does not by itself make a type a machine operand.

---

## 27. Explicit decomposition at the boundary

A high-level value crosses an asm boundary through the low-level representation
the operation actually requires.

Conceptually:

```text
high-level Sec value
        ↓
explicit extraction / conversion / decomposition
        ↓
fixed-width primitive or RawPtr
        ↓
asm
```

A string-like value, for example, would ordinarily expose the pointer/length or
other ABI components required by the operation rather than pass an opaque Sec
`string` object directly.

---

## 28. Shaped values and GPU assembly

Sec 0.1 does not pass shaped Sec values directly as inline-assembly operands.

GPU or SIMD work may later require target-defined machine vector/register
operand categories.

That future capability does not imply that an entire Sec tensor, matrix, or
other shaped object becomes one asm operand.

Where a GPU/kernel boundary needs:

```text
data address;
shape;
strides;
rank;
element metadata;
memory-space information;
```

those machine/ABI-relevant components are passed explicitly according to the
owning platform contract.

---

## 29. `RawPtr[T]`

`RawPtr[T]` is the canonical raw-address operand.

It:

```text
does not create a safe reference;
does not establish ownership;
does not extend lifetime;
does not prove memory validity;
does not grant hardware authority.
```

Raw pointer semantics remain owned by `rules/memory/raw_pointers.md` and
`rules/memory/unsafe.md`.

---

# Part VI — Output values and ownership

## 30. Outputs produce new low-level Sec values

An output such as:

```sec
outputs:
    rax(result)
```

means the assembly operation produces a machine representation from the
declared physical output location and constructs the corresponding permitted
low-level Sec output value.

The output becomes available only after normal completion of the asm operation.

---

## 31. Output type is not inferred from register width

The width of a physical register does not by itself define the Sec output type.

The output type must be resolved explicitly or unambiguously from the Sec
contract.

Backend representation must not silently choose a Sec type based only on a
register name.

---

## 32. Outputs remain low-level

Sec 0.1 asm outputs do not directly fabricate:

```text
safe references;
owned resources;
closed enums;
constrained user types;
high-level aggregates.
```

The asm operation produces the permitted low-level representation first.

The program then performs the canonical Sec construction/conversion/validation
needed to produce a higher-level type.

---

## 33. Physical copies do not create semantic owners

Moving bits into or out of target registers does not itself:

```text
copy ownership;
move ownership;
create ownership;
extend lifetime;
duplicate a resource.
```

Physical register movement and Sec ownership are separate semantics.

---

## 34. No implicit consuming operands

Sec 0.1 ordinary inline assembly does not implicitly consume ownership.

If a future low-level operation needs ownership transfer, it requires an
explicit independently defined semantic operation.

---

## 35. Same physical register as input and output

A physical location may be both input and output.

Example:

```sec
inputs:
    rax(number)

outputs:
    rax(result)
```

means:

```text
initial RAX representation comes from number;
assembly executes;
final RAX representation produces result.
```

`number` and `result` remain distinct Sec values.

The shared physical register does not imply in-place mutation of the Sec input
variable.

---

## 36. Future in/out mutation

If Sec later supports true in/out mutation of a Sec place through inline
assembly, that requires an explicit operand form with ordinary Sec place and
mutation semantics.

It is not inferred merely because input and output constraints name the same
physical register.

---

# Part VII — Clobbers

## 37. Clobber semantics

A clobber states that an assembly operation may destroy a target machine
resource whose post-operation value is not represented as a declared Sec
output.

Example:

```sec
clobbers:
    rcx
    r11
```

The backend must not treat previous live machine values in those resources as
preserved across the asm operation.

---

## 38. Every destroyed undeclared resource is a contract violation

Every machine resource destroyed by an asm operation and not represented by an
output must be declared as a clobber where that resource participates in the
target/compiler register model.

Where target instruction semantics are known, the compiler may diagnose missing
clobbers.

Where the compiler cannot verify the instruction behavior, correctness is part
of the unsafe assembly contract.

---

## 39. Outputs need not also be clobbers

A register already declared as an output is known to be written.

It need not also be listed as a clobber merely because the output modifies that
register.

---

## 40. No hidden target-specific default clobbers

Ordinary inline assembly has no implicit architecture-specific clobber set such
as:

```text
rcx;
r11;
flags;
```

unless a stronger compiler/platform-known operation explicitly defines those
effects.

Raw asm authors declare the clobbers required by the actual machine operation.

A `nop` need not invent artificial clobbers.

---

## 41. Target resources beyond general registers

Target clobbers may include resources such as:

```text
condition/status flags;
floating-point status;
vector state;
special processor state;
target-defined implicit resources.
```

Where the effect changes higher-level Sec execution semantics, a plain machine
clobber may additionally require a stronger semantic effect contract.

---

# Part VIII — Observability and optimizer contract

## 42. Ordinary asm is observable by default

An ordinary `asm` operation is observable.

If execution reaches the operation on a live semantic path, the compiler must
preserve its occurrence.

The compiler must not remove an ordinary asm operation merely because its
outputs are unused.

---

## 43. Observability is not link reachability

Asm observability does not make the containing declaration a `LinkRoot`.

An unreachable private declaration containing observable asm may still be
removed according to canonical Sec link reachability.

Conceptually:

```text
live function
    ↓
observable asm
        must remain

unreachable function
    ↓
observable asm
        whole unreachable declaration may be removed
```

Interrupt, startup, export, platform, callback, and other retention roots remain
owned by their canonical rulebooks and `LinkPlan`.

---

## 44. Observable does not mean global optimizer barrier

Observable asm is not automatically a barrier to every unrelated optimization.

The compiler may transform independent surrounding code where:

```text
data dependencies;
memory effects;
external effects;
machine-state effects;
ordering contracts;
control-flow contracts
```

permit the transformation.

---

## 45. Stronger pure contracts

A compiler/core/platform-known asm implementation may be classified as pure only
when an explicit owning contract establishes that all relevant semantics permit
ordinary pure-operation optimization.

Ordinary raw `asm` is not pure by default.

Sec 0.1 does not define C-style `asm volatile`.

---

## 46. No `volatile` asm syntax

Sec does not define:

```text
asm volatile
volatile asm
@volatile asm
```

or an equivalent general inline-assembly keyword form.

`volatile` in Sec is physical storage/access semantics owned by
`rules/platform/volatile.md`.

Asm observability is a separate semantic property.

---

# Part IX — Memory and external effects

## 47. `memory` is a special compiler-visible effect

The special clobber:

```sec
clobbers:
    memory
```

states that the assembly may perform memory accesses whose complete footprint is
not otherwise described by precise operands/effects.

It forces conservative compiler memory reasoning around the operation.

---

## 48. `memory` is not a hardware fence

`memory` does not itself provide:

```text
Acquire;
Release;
SeqCst;
CPU fence;
cache flush;
cache invalidation;
DMA ownership transfer;
device completion;
hardware-register completion.
```

It is a compiler-visible unknown-memory effect.

Actual hardware ordering/completion requires the applicable target operation and
canonical contract.

---

## 49. Precise memory footprints

When an asm operation has a known memory footprint, the compiler/platform
contract should preserve that information rather than collapse it needlessly
into unknown memory.

Conceptually:

```text
ReadMemory(address, size)
WriteMemory(address, size)
ReadWriteMemory(address, size)
UnknownMemoryRead
UnknownMemoryWrite
UnknownMemoryReadWrite
```

or equivalent semantic facts may be used internally.

Exact source syntax is not defined here.

---

## 50. Borrowing is not disabled by `memory`

Declaring unknown memory effects does not globally disable Sec borrow rules.

Known typed memory operands may still establish shared or mutable borrow
relationships according to their owning rules.

Opaque unknown memory effects make analysis conservative; they do not grant
permission to violate ownership or alias contracts.

---

## 51. External/system effects

Memory effects are distinct from externally observable non-memory effects.

Asm may interact with:

```text
operating system;
device I/O;
processor state;
privileged state;
environmental/nondeterministic input;
other execution environments.
```

Those effects must be represented when relevant to Sec analyses.

---

# Part X — Hardware, volatile, ordering, atomics, and concurrency

## 52. No second memory model

Inline assembly does not create a separate Sec memory or concurrency model.

Assembly may physically perform low-level memory/hardware operations, while the
canonical Sec memory, atomic, hardware-register, volatile, and concurrency
semantics continue to define the proofs available to the compiler.

---

## 53. Raw MMIO assembly

Raw user asm may directly access MMIO or another hardware endpoint where the
unsafe/target obligations permit it.

If the compiler lacks a canonical hardware contract for the exact operation, it
must not invent one from the instruction text.

The operation may therefore be physically valid while remaining opaque to
higher-level hardware proofs.

---

## 54. Compiler/platform-known hardware primitives

Where `sec/core` or `sec/platform` implements a canonical hardware operation with
inline assembly, the owning primitive carries the stronger semantics required by
hardware-register analysis.

Conceptually:

```text
canonical hardware operation
        ↓
resolved semantic contract
        ↓
inline-assembly lowering
```

not:

```text
opaque assembly
        ↓
later passes guess hardware semantics
```

---

## 55. Compiler ordering versus hardware ordering

Compiler ordering and hardware ordering are separate.

An asm memory effect may constrain compiler movement without issuing a hardware
fence.

A target fence instruction may provide physical ordering only when the owning
canonical contract says what ordering relation it establishes.

---

## 56. Ordering versus completion

Hardware ordering and hardware completion remain separate.

An assembly sequence may implement:

```text
fence;
read-back;
special completion instruction;
controller-specific completion sequence.
```

When Sec relies on that operation as a positive completion proof, the
compiler/platform contract explicitly states the completion semantics.

Mnemonic recognition alone is not the proof.

---

## 57. Machine atomicity versus Sec atomic semantics

An assembly instruction may be physically atomic on the target.

That alone does not make surrounding shared-memory access a proven Sec atomic or
race-free operation.

Raw assembly that performs an atomic instruction without a canonical atomic
contract may remain `Unproven` for analyses requiring positive synchronization
proof.

---

## 58. Atomics implemented by platform/core

Canonical Sec atomic operations may lower through inline assembly.

In that case the owning atomic operation defines:

```text
location;
width;
operation;
indivisibility;
memory ordering;
target support;
blocking/progress properties.
```

Inline assembly materializes those semantics.

It does not define them by mnemonic inference.

---

## 59. CPU atomic operations against MMIO

A CPU atomic instruction is not automatically legal or atomic against a device
or MMIO endpoint.

The hardware resource/endpoint contract owns:

```text
legal operation;
width;
bus semantics;
atomicity;
side effects.
```

CPU capability alone does not establish device capability.

---

## 60. DMA and cache coherence

`memory` and asm observability do not establish:

```text
DMA ownership transfer;
cache coherence;
cache maintenance;
descriptor publication ordering;
device visibility.
```

Applicable memory/device contracts remain required.

---

# Part XI — Interrupt integration

## 61. Asm inside ISR remains ISR execution

An asm operation inside an ISR executes under the resolved ISR context.

Unsafe assembly cannot bypass:

```text
noPanic;
noAlloc;
noBlock;
runtime constraints;
stack constraints;
hardware-access context;
race/deadlock requirements.
```

The canonical interrupt/ISR rulebooks own those requirements.

---

## 62. Raw interrupt-mask instructions do not create analysis proofs

Raw assembly may physically change interrupt state.

The compiler does not generally infer critical-section or `MayPreempt`
relationships by parsing the instruction mnemonic.

If Sec analysis must rely on the change, a compiler/platform-known masking
primitive carries the canonical interrupt-mask effect explicitly.

---

## 63. Interrupt return instructions

Ordinary inline asm must not replace canonical ISR/exception return.

Instructions such as target interrupt/exception return operations belong in
compiler/platform entry/return lowering or another explicitly stronger machine
entry contract.

They are not ordinary inline-asm substitutes for source `return`.

---

# Part XII — Control flow

## 64. One Sec entry

An ordinary inline-assembly operation has one Sec control-flow entry.

Sec does not branch into an internal assembly label from arbitrary source
control flow.

---

## 65. Normal fallthrough is the default

Ordinary inline asm normally:

```text
enters;
executes;
returns to the next Sec statement.
```

A non-fallthrough operation requires an explicit compiler-visible control-flow
contract.

---

## 66. Internal local control flow

Assembly-local labels, loops, and branches are permitted where supported by the
target assembler.

Their control flow remains internal to one Sec asm operation.

---

## 67. No arbitrary jumps into or out of Sec CFG

Ordinary inline asm does not provide a general `asm goto` mechanism.

It must not jump:

```text
from asm into an arbitrary Sec basic block;
from Sec into an internal asm label.
```

Such behavior would bypass canonical:

```text
ownership;
borrowing;
definite initialization;
defer;
destruction;
cleanup;
stack;
debug/control-flow assumptions.
```

---

## 68. Non-returning asm

An operation that does not return to the next Sec statement requires an explicit
`NoReturn` or equivalent compiler-visible contract.

The exact source spelling is not defined here.

The Sec CFG must become terminal at that operation.

---

## 69. Machine `ret`

Ordinary inline asm must not return directly from the enclosing Sec function
using a machine return instruction.

Function return remains owned by Sec control flow and target ABI lowering.

This prevents bypass of:

```text
defer;
destruction;
cleanup;
compiler epilogue;
stack restoration;
ABI obligations.
```

---

## 70. Tail transfer

A permanent machine jump/tail transfer to another function or execution context
is not ordinary fallthrough inline asm.

If Sec later supports such a primitive, it requires an explicit stronger
control-flow/ABI contract.

---

# Part XIII — Stack and ABI

## 71. Enclosing function ABI remains compiler-owned

Ordinary inline asm executes inside the surrounding Sec function's resolved ABI.

The compiler continues to own:

```text
function prologue;
function epilogue;
stack alignment;
callee-saved preservation;
return convention;
frame layout.
```

Inline asm does not implicitly become a naked function body.

---

## 72. Stack pointer preservation

On every normal fallthrough exit from ordinary inline asm, the surrounding stack
pointer/state must be restored according to the target ABI.

Conceptually:

```text
stack state before asm
    ==
required enclosing stack state after normal asm completion
```

---

## 73. Stack pointer is not an ordinary clobber

Listing the stack pointer as a machine clobber is not sufficient to permit an
arbitrary persistent stack change.

Stack changes interact with:

```text
local storage;
spills;
return address;
cleanup;
stack analysis;
debug information;
ISR stack bounds;
ABI.
```

They require a dedicated stack-effect contract.

---

## 74. Temporary stack use

Inline asm may temporarily use stack storage when it restores the required
enclosing stack state before normal exit.

The compiler-visible stack contract must be sufficient to describe at least:

```text
final stack delta;
maximum additional stack consumption;
required alignment preservation;
```

where such facts are required by canonical stack/resource analysis.

Without a sufficient stack contract, arbitrary stack-pointer modification is
forbidden.

---

## 75. Unknown or unbounded stack use

Asm with unknown/unbounded stack consumption cannot satisfy a context that
requires a finite positive stack proof.

`unsafe` does not convert unknown stack use into a valid finite bound.

---

## 76. Permanent stack switching

Permanent stack switching, manual context-frame ownership, and naked machine
entry/exit semantics are not ordinary inline-assembly behavior.

They require a stronger compiler/platform contract because they change:

```text
execution context;
stack domain;
lifetime/cleanup assumptions;
control flow;
runtime state;
ABI.
```

---

# Part XIV — Calls, symbols, and relocations

## 77. Ordinary Sec calls remain preferred

Inline assembly is not an alternative ordinary syntax for calling a Sec
function.

Where an ordinary Sec call expresses the required operation correctly, ordinary
Sec call semantics should be used.

---

## 78. Machine calls inside asm

Machine calls may exist inside legitimate assembly sequences.

When an asm call depends on a Sec, foreign, or platform symbol, that dependency
must be compiler-visible.

Opaque assembly text must not be the canonical source of whole-program link
reachability.

---

## 79. Symbol operands/dependencies

The compiler must have a semantic representation equivalent to:

```text
assembly placeholder
    -> canonical Sec/foreign/platform declaration identity
    -> BinarySymbolIdentity
    -> target symbol spelling / relocation
```

The exact source syntax is not defined here.

The programmer should not need to know Sec's final mangled/linker symbol spelling
to reference a canonical Sec symbol from asm.

---

## 80. Call ABI and effects

Where an asm operation calls a compiler-visible callable, the resolved call
contract must preserve relevant:

```text
calling convention;
ABI;
clobbers;
stack requirements;
memory effects;
MayAllocate;
MayBlock;
MayPanic;
ISR compatibility;
other declared effects.
```

A machine call does not bypass FFI, ISR, stack, or effect rules.

---

## 81. Indirect machine calls

An asm call through an unknown runtime address is an indirect call boundary.

Where a positive effect/ABI/ISR proof is required and the compiler lacks a
sufficient callable contract, the result is conservative/`Unproven`.

Unsafe does not make unknown callable effects disappear.

---

## 82. Local symbols

Assembly-local labels and local assembler symbols are assembler-owned.

They do not require Sec binary identities when they cannot escape the asm
operation.

---

## 83. Global symbol definitions are not ordinary asm

Ordinary inline asm must not define a parallel externally visible symbol model.

Directives/constructs equivalent to:

```text
.global;
.weak;
global symbol labels;
COMDAT/link-once identity;
symbol visibility;
object symbol type/size.
```

are not ordinary inline-assembly facilities.

---

## 84. Section/object-model directives

Ordinary inline asm must not independently manipulate the object/link layout
using constructs equivalent to:

```text
.section;
.pushsection;
.popsection;
.org;
arbitrary object placement;
compiler-owned unwind/debug metadata.
```

Section placement and object layout remain owned by `LinkPlan`, platform, and
backend lowering.

---

## 85. External include directives

Ordinary inline asm must not introduce hidden build dependencies through
assembler directives equivalent to:

```text
.include;
.incbin.
```

File/binary dependencies must enter the canonical Sec build graph explicitly.

---

## 86. Relocations

Target relocations are permitted when produced from compiler-visible symbol or
address dependencies.

The target may select:

```text
PC-relative relocation;
GOT/PLT relocation;
high/low relocation pair;
target-defined relocation.
```

The canonical dependency remains known to Sec before final linking.

---

# Part XV — Target assembler directives and validation

## 87. Directive classification

The target model may classify assembler directives conceptually as:

```text
LocalAllowed;
ObjectModelAffecting;
ForbiddenOrUnsupported.
```

Ordinary inline asm may use target-approved local directives that do not escape
the operation or create a second object/link model.

---

## 88. Local assembler directives

Target-approved local directives may include, where valid:

```text
local constants;
local label helpers;
local encoding directives;
block-local alignment;
raw instruction encodings;
target-local assembler state restored before block exit.
```

Exact directive availability is target-defined.

---

## 89. Raw instruction encodings

A target may permit assembly forms equivalent to:

```text
.byte;
.word;
.inst;
raw target encoding.
```

Using a raw encoding may reduce the compiler's ability to validate the actual
instruction semantics.

The programmer therefore bears a stronger unsafe obligation for the emitted
bits.

The surrounding Sec contract remains mandatory.

---

## 90. Raw bytes do not waive effects

Encoding an instruction as bytes does not remove the requirement to describe
correct:

```text
inputs;
outputs;
clobbers;
memory effects;
control-flow effects;
stack effects;
external/machine-state effects.
```

---

## 91. ISA membership is insufficient

An instruction being known to the assembler does not prove it is legal under
the selected `CompilationPlan`.

Validation may additionally require:

```text
CPU feature;
ISA extension;
execution mode;
privilege;
platform authority;
security state;
target availability.
```

---

## 92. Assembly cannot enable absent target features

Inline asm and assembler directives must not silently enlarge the
`CompilationPlan` feature set.

If an instruction requires a feature absent from the plan, compilation fails.

---

## 93. Privilege validation

A privileged instruction remains invalid in an execution environment that does
not provide the required authority, even if the target assembler can encode it.

For example:

```text
ISA-valid
    !=
userspace-valid
```

and:

```text
unsafe
    !=
privileged.
```

---

# Part XVI — Traps, faults, syscalls, and environment transitions

## 94. Environment transitions

Inline assembly may implement target-defined execution-environment transitions
such as:

```text
system call;
supervisor call;
hypercall;
firmware call;
secure monitor call;
debug trap;
RTOS kernel entry;
other target-defined service transition.
```

The canonical semantic category is broader than "syscall".

---

## 95. System call is not an ordinary Sec call

A system call is an architectural transition.

Conceptually:

```text
Sec userspace
    ↓
architectural transition
    ↓
kernel / service environment
    ↓
architectural return
    ↓
Sec execution
```

Its ABI is platform-specific.

---

## 96. Platform owns transition ABI

A system/environment transition contract may define:

```text
input locations;
output locations;
clobbers;
memory effects;
external effects;
privilege transition;
fault behavior;
blocking behavior;
return behavior;
raw error convention.
```

These facts come from platform/ABI knowledge rather than generic ISA semantics.

---

## 97. Raw results remain raw

An asm output transports the machine-level primitive result.

A platform-specific error convention such as:

```text
negative integer means errno
```

does not automatically become a Sec `Result`.

A higher-level Sec/platform wrapper interprets the raw ABI and constructs
`Result`, `Option`, or domain-specific values using their canonical rules.

---

## 98. Trap and fault are distinct from ordinary Result

Machine faults, processor exceptions, and architectural traps do not
automatically become ordinary Sec `Err(...)` propagation.

The target/runtime/interrupt model defines what happens after:

```text
page fault;
bus fault;
illegal instruction;
alignment fault;
privilege fault;
explicit trap.
```

---

## 99. Explicit transition versus possible fault

The compiler may distinguish semantic facts equivalent to:

```text
ExecutionEnvironmentTransition;
ExplicitTrap;
MayFault.
```

These are not necessarily the same event.

---

## 100. One instruction may have strong effects

The semantic effect of an asm operation is not measured by instruction count.

A one-instruction system transition may be:

```text
MayBlock;
NoReturn;
MayIO;
UnknownMemoryReadWrite;
MayMutateExternalState;
```

according to the owning platform contract.

---

## 101. Kernel-internal allocation is not Sec allocation

A system service may allocate internally without making the Sec operation
`MayAllocate` in the Sec heap/resource sense.

Sec effects describe the Sec-visible execution contract.

Kernel/internal implementation details do not automatically become Sec
allocation effects.

---

## 102. Raw process termination

A raw exit/service transition may be `NoReturn`.

When a raw no-return transition occurs, there is no ordinary continuation after
the operation.

Higher-level APIs may perform required cleanup before invoking the raw terminal
primitive.

---

# Part XVII — Effects and analysis integration

## 103. Effect analysis consumes asm contracts

Inline assembly contributes effects to canonical effect analysis.

Relevant facts may include:

```text
MayPanic;
MayAllocate;
MayBlock;
MaySuspend;
MayIO;
MayAccessVolatile;
MayMutateExternalState;
MayUseNondeterministicInput;
machine-state effects;
target-specific trusted effects.
```

The exact owning effect set remains defined by `rules/analysis/effect_analysis.md`.

---

## 104. Trust provenance

Where an effect or semantic guarantee depends on an inline-assembly declaration,
the compiler preserves trust provenance equivalent to:

```text
trusted through an inline assembly declaration
```

as defined by the canonical effect/unsafe model.

---

## 105. Raw asm does not provide positive facts it did not declare

Arbitrary raw asm may execute physically while lacking sufficient compiler
knowledge for:

```text
atomicity;
race freedom;
hardware completion;
interrupt exclusion;
no blocking;
bounded stack;
no fault;
specific alias footprint.
```

Where such positive proof is mandatory, absence of a sufficient contract yields
conservative analysis rather than optimistic inference.

---

# Part XVIII — Semantic IR, MLIR, and backend lowering

## 106. Inline asm is first-class Sec semantics

The parser does not lower directly from source asm to an LLVM constraint string.

The Sec pipeline conceptually resolves:

```text
source asm
    ↓
typed/target-resolved asm contract
    ↓
Semantic IR asm operation
    ↓
Sec MLIR / target lowering
    ↓
backend inline-assembly mechanism
    ↓
machine encoding
```

---

## 107. Semantic IR requirements

Semantic IR must preserve all correctness-relevant facts needed by later
compiler stages, including as applicable:

```text
assembly template;
input/output Sec types;
physical constraints;
clobbers;
memory effects;
external effects;
machine-state effects;
atomic/order/completion contracts;
control-flow behavior;
stack effects;
symbol dependencies;
execution-environment contracts;
target requirements;
observability;
trust provenance;
source ranges.
```

The exact internal record layout is implementation-specific.

---

## 108. Assembly text is not operation identity

Two asm operations containing identical target text may carry different
high-level contracts.

For example, two platform wrappers may both execute the same system-transition
instruction but differ in:

```text
MayBlock;
NoReturn;
memory footprint;
error interpretation;
external effect.
```

Therefore assembly text alone is not the semantic identity.

---

## 109. Sec MLIR preservation

Sec MLIR must preserve the Sec asm contract until all dependent analyses have
consumed the information or the information has been explicitly represented in
the next lowering level.

A dedicated Sec MLIR operation or an equivalent semantics-preserving form may be
used.

The exact MLIR operation spelling is compiler implementation detail.

---

## 110. Lowering to LLVM

The LLVM backend translates Sec physical operand constraints into LLVM's
required inline-asm representation.

The user does not write LLVM constraint strings.

The backend may materialize:

```text
fixed-register constraints;
register-class constraints;
clobbers;
memory constraints/effects;
side-effect flags;
symbol operands;
target dialect;
other backend-required details.
```

from the already resolved Sec contract.

---

## 111. Backend cannot weaken semantics

If the selected backend cannot exactly materialize required semantics, compilation
fails.

It must not silently:

```text
drop clobbers;
change operand widths;
add implicit conversions;
remove observability;
lose symbol dependencies;
change control flow;
discard required ordering/completion.
```

---

## 112. Backend independence

Sec `asm` is not defined as "LLVM inline asm".

A future backend may implement the same Sec semantic contract through another
target-assembly mechanism.

---

# Part XIX — Optimization and LTO

## 113. High-level optimization uses the Sec contract

Optimizers may use:

```text
data dependencies;
typed inputs/outputs;
precise memory footprints;
observability;
control-flow facts;
explicit pure/platform contracts.
```

They do not need to understand target mnemonics to preserve correctness.

---

## 114. Opaque asm does not freeze the entire function

Unrelated surrounding code may still be optimized where the complete asm
contract permits it.

Asm is not automatically a whole-function optimization barrier.

---

## 115. LTO may inline functions containing asm

LTO may inline a Sec function containing inline assembly when:

```text
target requirements remain satisfied;
execution context remains legal;
observability remains preserved;
stack/control-flow contracts remain valid;
effects remain equivalent.
```

---

## 116. LTO preserves live observable asm

Observable asm on a reachable execution path must not be:

```text
removed;
duplicated;
merged;
reordered
```

in a way that changes its declared semantics.

---

## 117. LTO may remove unreachable containers

An unreachable private declaration containing observable asm may still be
removed according to canonical link reachability.

Asm does not create a hidden global root.

---

# Part XX — Separate compilation and caching

## 118. Separate compilation

Libraries may contain inline assembly.

Separate-compilation summaries preserve the facts consumers require without
requiring consumers to reparse the original assembly text.

Relevant summaries may include:

```text
declared/inferred effects;
stack requirements;
ISR-relevant guarantees;
symbol dependencies;
public guarantees;
target requirements;
trust provenance.
```

---

## 119. Implementation verification versus consumer summary

When assembly source is available during compilation of the defining module, the
compiler verifies the implementation to the extent required by this rulebook.

A later consumer may use compatible exported summaries/contracts without
reopening the source asm.

---

## 120. Stale summaries cannot prove safety

A stale or incompatible inline-assembly summary must not be used as a positive
proof.

Relevant invalidation includes changes to:

```text
assembly text;
operand contract;
clobbers;
effects;
stack contract;
symbol dependencies;
target requirements;
compiler semantic version.
```

---

## 121. Cache identity

Target-validation/materialization caches include all inputs that may affect the
result, including as applicable:

```text
assembly text;
Sec asm contract;
Architecture;
CPU/profile;
ISA feature set;
assembler dialect;
execution environment;
ABI;
backend assembler/lowering semantics;
toolchain version/fingerprint.
```

---

## 122. Semantic summaries and backend caches are distinct

A backend/toolchain upgrade may invalidate encoded assembly/materialization
without changing the public Sec semantic contract.

The compiler should not make high-level API effect summaries
LLVM-version-dependent when no Sec semantic fact changed.

---

# Part XXI — Diagnostics and LSP

## 123. Diagnostic classes

Inline-assembly diagnostics distinguish:

```text
Sec contract error;
target assembler error;
backend/materialization error.
```

They must not all be reported as a generic "invalid asm" error.

---

## 124. Sec contract diagnostics

Sec should diagnose known contract errors before target assembly where
practical.

Examples include:

```text
operand width mismatch;
forbidden direct Sec operand type;
unknown target register;
conflicting constraints;
missing unsafe context;
invalid output contract;
undeclared compiler-visible symbol dependency;
forbidden object-model directive;
missing stack contract;
illegal control-flow form.
```

---

## 125. Target assembler diagnostics

The target assembler owns instruction grammar/encoding diagnostics such as:

```text
unknown mnemonic;
invalid instruction form;
invalid addressing mode;
invalid immediate;
invalid local-label expression.
```

Sec does not need a duplicate full parser/assembler for every supported ISA.

---

## 126. Backend failures

If a valid Sec contract cannot be represented correctly by the selected backend,
the diagnostic identifies a compiler/backend limitation rather than pretending
that otherwise valid source assembly was syntactically invalid.

---

## 127. Source mapping

Multiline assembly diagnostics map back to the original Sec source line/column
inside the asm template.

Operand, output, clobber, symbol, and contract entries retain individual source
ranges where practical.

---

## 128. Hide backend constraint syntax

Ordinary diagnostics speak in Sec assembly terms:

```text
register;
operand type;
width;
clobber;
target;
symbol;
effect.
```

LLVM constraint strings are not the primary user model.

Raw backend details may be shown as secondary debugging information where useful.

---

## 129. LSP target parity

LSP uses the same resolved `CompilationPlan` and target register/feature/dialect
facts as real compilation.

It never validates asm against the editor host merely because the editor runs on
that architecture.

---

## 130. Progressive validation

Tooling may distinguish:

```text
Sec contract: Valid
target assembly: Pending
overall: Pending
```

during incremental recomputation.

It must not present incomplete validation as completed `Valid`.

---

## 131. Hover

Tooling may expose resolved asm information such as:

```text
target;
assembler dialect;
observable status;
typed inputs;
typed outputs;
clobbers;
memory effects;
control-flow contract;
stack effect;
target validation status;
stronger platform/compiler contract provenance.
```

---

## 132. Symbol navigation

Compiler-visible asm symbol dependencies participate in:

```text
Go to Definition;
Find References;
Rename
```

where ordinary symbol tooling supports those operations.

Local assembler labels remain assembler-local and are not required to become Sec
declarations.

---

# Part XXII — Required regression coverage

## 133. Parsing and source forms

Tests cover:

```text
single instruction;
multi-instruction template;
structured asm block;
inputs;
outputs;
clobbers;
memory clobber;
local labels;
multiple asm operations in one function;
pure instruction-only asm with no artificial operand requirement.
```

Additional shorthand accepted by grammar must lower to identical canonical
semantics.

---

## 134. Unsafe tests

Tests verify:

```text
asm rejected outside explicit unsafe context;
asm accepted inside valid unsafe context;
unsafe fn body does not implicitly satisfy the unsafe context;
unsafe does not waive type/target/ISR/effect rules.
```

---

## 135. Operand width tests

Tests accept exact width matches and reject implicit conversion.

Examples include:

```text
32-bit machine operand <- uint32/int32;
64-bit machine operand <- uint64/int64;
32-bit machine operand <- uint64 rejected;
64-bit machine operand <- uint32 rejected.
```

---

## 136. Operand type tests

Tests cover supported:

```text
fixed-width integers;
supported fixed-width floating values;
RawPtr[T].
```

Tests reject direct:

```text
string;
bool;
rune;
enum;
struct;
array;
tensor;
user-defined nominal type;
ref;
owned resource.
```

---

## 137. Output tests

Tests verify:

```text
output unavailable before asm;
output available after normal completion;
same physical input/output register is legal;
output type not inferred solely from register width;
no implicit high-level type construction.
```

---

## 138. Clobber tests

Tests cover:

```text
known clobber accepted;
unknown register rejected;
output does not need duplicate clobber;
conflicting constraints rejected;
live machine values preserved around declared clobber;
no hidden architecture-specific default clobbers.
```

---

## 139. Observability tests

Tests verify:

```text
ordinary live asm survives unused output;
observable asm is not duplicated;
observable asm is not incorrectly merged;
unreachable function containing asm may be removed;
interrupt/startup/link roots retain containing code through their own canonical roots.
```

---

## 140. `memory` tests

Tests verify:

```text
memory forces conservative compiler memory reasoning;
memory does not emit or imply a hardware fence;
precise memory footprints remain more precise than unknown memory where provided.
```

---

## 141. Control-flow tests

Tests cover:

```text
internal local branch accepted;
arbitrary jump into/out of Sec CFG rejected;
normal fallthrough;
explicit no-return behavior;
ordinary machine return rejected as a Sec function exit;
interrupt/exception return rejected as ordinary inline-asm source exit.
```

---

## 142. Stack tests

Tests verify:

```text
normal asm preserves required stack state;
known stack-pointer modification without stack contract is rejected;
bounded temporary stack use contributes to stack analysis;
unknown/unbounded stack use cannot satisfy finite required proof;
permanent stack switching is not ordinary asm.
```

---

## 143. Directive tests

Target-specific tests cover allowed local directives/raw encodings.

Negative tests cover object/build escape forms equivalent to:

```text
.section;
.global;
.weak;
.org;
.include;
.incbin;
compiler-owned unwind/debug directives.
```

---

## 144. Target feature tests

Tests verify:

```text
instruction valid for selected ISA;
assembler-known instruction rejected when CPU feature absent;
assembler-known privileged instruction rejected where execution environment lacks authority;
asm cannot enable absent ISA extension through directive;
cross-compilation uses target rather than host.
```

---

## 145. Syscall/platform tests

At least one canonical platform regression covers an amd64/Linux raw syscall
using:

```text
fixed registers;
same input/output register;
explicit clobbers;
memory effect;
system transition;
raw primitive result.
```

The test must use fixed-width asm-boundary values according to this rulebook.

---

## 146. Embedded tests

The regression corpus includes non-hosted inline assembly for at least one
embedded target, and should include both ARM and RISC-V families as support
matures.

The corpus must not become x86/hosted-only.

---

## 147. Atomic/concurrency tests

Tests verify:

```text
raw machine atomic instruction does not automatically create Sec atomic proof;
canonical Atomic operation may lower through asm while preserving its owning contract;
raw interrupt-mask asm does not automatically create critical-section proof;
canonical platform masking primitive may provide the canonical interrupt effect;
memory clobber is not synchronization.
```

---

## 148. ISR tests

Tests verify that asm remains subject to ISR:

```text
noPanic;
noAlloc;
noBlock;
stack;
hardware context;
race/deadlock;
runtime;
target authority
```

requirements.

---

## 149. Symbol/link tests

Tests verify:

```text
compiler-visible asm symbol creates canonical link dependency;
source does not require final mangled symbol spelling;
target relocation is generated from canonical dependency;
hidden opaque text is not the canonical reachability source;
global symbol definitions are not ordinary asm.
```

---

## 150. LTO and separate compilation tests

Tests cover:

```text
LTO preserves live observable asm;
LTO may remove unreachable containing code;
inlining preserves target/effect/stack contracts;
separate-compilation summary carries required effects/dependencies;
stale summary cannot provide positive proof.
```

---

## 151. Diagnostics and LSP tests

Tests verify:

```text
Sec contract errors use Sec terms;
assembler errors map to the correct asm source line;
target/feature information appears when relevant;
backend constraint syntax does not dominate ordinary diagnostics;
LSP and compiler use identical target resolution;
incremental edits invalidate only relevant asm validation/cache work;
Pending validation is not displayed as completed Valid.
```

---

## 152. Determinism tests

For identical:

```text
source;
CompilationPlan;
Sec semantic version;
asm contract;
toolchain semantics
```

the compiler produces deterministic:

```text
resolved operands;
clobbers;
effect facts;
symbol dependencies;
validation results;
semantic asm operation.
```

Exact machine-byte tests are required where exact encoding is part of the
contract.

Where several target encodings are semantically equivalent, tests should avoid
unnecessary byte-level brittleness across toolchain upgrades.

---

# Part XXIII — Completion criteria

## 153. Sec 0.1 inline-assembly implementation is complete when

The implementation can deterministically:

1. parse canonical structured inline assembly and real target assembly text;
2. resolve each asm operation against the active `CompilationPlan`;
3. accept instruction-only asm without artificial input/output requirements;
4. require explicit unsafe context according to `unsafe.md`;
5. restrict direct register operands to the machine-compatible Sec 0.1 types
   defined here;
6. enforce exact fixed-width integer operand matching without implicit
   widening/narrowing/sign-extension/zero-extension;
7. support `RawPtr[T]` as the canonical raw address operand where target
   compatible;
8. support target-defined fixed-register and applicable operand-class
   constraints;
9. produce typed low-level outputs only after normal completion;
10. support tied physical input/output locations without conflating Sec values;
11. validate clobbers without hidden architecture-specific defaults;
12. treat ordinary asm as observable on live execution paths;
13. keep asm observability distinct from link-root retention;
14. represent `memory` as conservative compiler-visible memory interaction
   without fabricating hardware ordering;
15. preserve stronger compiler/platform-known atomic, ordering, completion,
   hardware, interrupt, or system-transition contracts when asm implements such
   primitives;
16. support internal target-local labels/control flow while rejecting arbitrary
   jumps across the Sec CFG boundary;
17. model explicit non-returning asm;
18. prevent ordinary inline asm from replacing Sec function/ISR return and
   cleanup semantics;
19. integrate bounded temporary stack use with canonical stack analysis;
20. keep permanent stack switching/naked-entry semantics outside ordinary asm;
21. preserve compiler-visible Sec/foreign/platform symbol dependencies and
   target relocations;
22. reject ordinary asm attempts to create an independent global
   symbol/section/object/build model;
23. validate ISA, CPU features, execution environment, and known privilege
   requirements against the active target plan;
24. represent syscall/trap/environment transitions through platform-visible
   contracts rather than instruction-count heuristics;
25. keep raw ABI result values separate from higher-level `Result` or domain
   construction;
26. propagate asm effects and trust provenance into canonical effect analysis;
27. preserve the complete semantic contract through Semantic IR and Sec MLIR;
28. materialize through LLVM inline asm or another backend without making the
   backend representation the semantic authority;
29. preserve semantics through optimization and LTO;
30. support compatible separate-compilation summaries and correct invalidation;
31. provide source-mapped Sec-contract, target-assembler, and backend diagnostics;
32. use the same target semantics in compiler and LSP;
33. pass the cross-target, syscall, embedded, ISR, concurrency, hardware,
   linking, LTO, incremental, diagnostics, and determinism regression families
   required by this rulebook.

---

## 154. Core invariants

> Inline assembly is an explicit unsafe escape hatch from Sec's ordinary
> instruction vocabulary, not an escape hatch from Sec's compiler model.

> The programmer may directly express target machine instructions while the
> compiler continues to own the surrounding type, ownership, effect,
> control-flow, stack, target, concurrency, interrupt, linking, and diagnostic
> semantics.

> `sec/core` and `sec/platform` may use inline assembly to implement
> compiler-known or platform-known primitives with stronger semantic contracts
> than arbitrary user assembly. Those stronger semantics come from the owning
> Sec contract, not from recognizing particular assembly mnemonics.

> Where the compiler lacks a declared semantic contract for arbitrary assembly,
> it remains conservative rather than inventing knowledge from assembly text.

---

## 155. Non-goals for Sec 0.1

This rulebook does not require:

```text
a Sec-written assembler for every target;
direct Sec MLIR source injection;
direct LLVM IR source injection;
LLVM constraint strings in Sec source;
a general `volatile` keyword or `asm volatile`;
direct string/aggregate/shaped/user-defined asm operands;
implicit integer width conversions at the asm boundary;
implicit ownership transfer through machine registers;
general asm goto;
ordinary inline-asm naked functions;
ordinary inline-asm permanent stack switching;
a hidden global symbol/section/object model inside asm;
automatic inference of atomicity, interrupt masking, hardware ordering,
completion, or blocking semantics from instruction mnemonics;
a specific future GPU/SIMD machine-vector operand model;
a particular `#target`/`@target` scope syntax.
```
