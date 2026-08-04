# Sec Language Rulebook Status

## Purpose

This document is the canonical inventory of the rulebooks expected for the Sec
language, compiler, target model, core library, and standard library.

It replaces the former temporary `temp2.txt` checklist.

This document tracks documentation status only.

It does not claim that a written rulebook has been implemented by the compiler.

Implementation progress belongs in:

```text
rules/rules_implementations.txt
```

and in the `Implementation status` section of rulebooks that require one.

---

# Status definitions

| Status | Meaning |
|---|---|
| **Written** | A rulebook exists and is part of the current rule set. |
| **Written — sync required** | A rulebook exists, but newer decisions must be merged into it or into related rulebooks. |
| **Written — repository sync pending** | The rulebook has been written after the latest repository version reviewed by this document and must be added to `rules/`. |
| **Living** | The document exists and is intentionally updated as implementation progresses. |
| **Planned** | The rulebook is expected but has not yet been written. |
| **Deferred** | The topic is intentionally outside the immediate Sec 0.1 closure work. |
| **Covered** | The topic is covered by another canonical rulebook and should not have a duplicate document. |
| **Candidate** | The need or final filename has not yet been locked. |

A rulebook may be written while the corresponding feature remains entirely
unimplemented.

---

# Repository baseline

The current repository already contains the broad language, ownership,
concurrency, compiler, MLIR, diagnostics, and project rule sets.

The following newer rulebooks have been added to the canonical inventory:

```text
collections-shaped-types.md
thread_local.md
discard.md
ownership.md
lsp.md
formatter.md
copy_move.md
memory_model.md
operators.md
default_values.md
contracts.md
grammar.md
attributes.md
panic.md
runtime_checks.md
effect_analysis.md
unsafe.md
```

These were written after the older temporary checklist was last synchronized.

---

# 1. Language foundations and lexical rules

| Rulebook | Status | Notes |
|---|---|---|
| `language_philosophy.txt` | **Written** | Core language direction and design principles. |
| `lexical_structure.md` | **Written** | Includes `default` keyword contexts and canonical `0c`/`0r` literals. |
| `types.txt` | **Written — sync required** | Defaultability and mutable initialization are synchronized; collections, panic/runtime checks, and layout still require work. |
| `contracts.md` | **Written** | Canonical named-type contracts; replaces the obsolete variable-contract model. |
| `default_values.md` | **Written** | Canonical primitive, constrained, aggregate, list and explicit-default semantics. |
| `units.txt` | **Written — sync required** | Must be synchronized with shaped arithmetic and matrix multiplication. |
| `grammar.md` | **Written** | Canonical consolidated grammar for Sec 0.1; implementation differences are tracked in the document. |
| `operators.md` | **Written** | Canonical operator inventory, precedence, associativity, contextual `x`, indexing, conversion syntax and constant defaults. |
| `names_scopes_visibility.md` | **Written — sync required** | Top-level module declaration namespace conflicts are partially implemented; remaining scope, visibility, reserved-name and naming-rule audit still needed. |
| `attributes.md` | **Written** | Canonical closed Sec 0.1 attribute set, syntax, attachment, selection, target binding, `@noCopy`, verified guarantees, conflicts, formatter/LSP behavior, and explicit implementation status. |
| `unsafe.md` | **Written** | Canonical unsafe contexts, operations, functions and extern declarations, caller obligations, raw pointers, trust boundaries and provenance; compiler support remains partial. |

## Contextual words and operators

Sec may use a spelling contextually without making it a globally reserved word.

The following must be handled contextually:

```sec
let x := 10
let product := left x right
```

Here `x` is:

- an ordinary identifier in declaration and expression-name position;
- a matrix-multiplication operator between compatible shaped expressions.

The built-in collection type `set` and the property accessor spelling `set`
must follow the same principle.

```sec
let values: set[int]
```

In a type position followed by generic arguments, `set` is the built-in type
constructor.

Inside the property-accessor grammar, `set` introduces or identifies the setter.

Outside those contexts, `set` may remain available as an ordinary identifier if
the grammar can resolve it unambiguously.

`set` must therefore not become a globally reserved keyword solely because of
the collection type.

The parser must resolve it from:

- the current grammar context;
- token lookahead;
- type position versus property accessor position.

This decision requires synchronization of:

```text
lexical_structure.md
grammar.md
types.txt
properties.txt
collections-shaped-types.md
formatter.md
VS Code grammar
LSP token classification
```

---

# 2. Declarations, functions, and control flow

| Rulebook | Status | Notes |
|---|---|---|
| `struct.txt` | **Written** | Includes recursive semantic initialization of omitted fields. |
| `enums.txt` | **Written — sync required** | Must remain aligned with `bit[N]`, aliases, and `iota`. |
| `unions.txt` | **Written** | Tagged union rules. |
| `registers.txt` | **Written** | Register declarations, fields, widths, and reserved `_` bits. |
| `functions.txt` | **Written — sync required** | Must be synchronized with discard of non-void results and effects. |
| `functions_lambda.txt` | **Written — sync required** | Canonical lambda and closure rulebook; closure scope for 0.1 must be finalized. |
| `lambdas.md` | **Covered** | Covered by `functions_lambda.txt`; no duplicate rulebook expected. |
| `closures.md` | **Covered** | Covered by `functions_lambda.txt`; no duplicate rulebook expected. |
| `generics.txt` | **Written — sync required** | Must be synchronized with compile-time values, collections, register widths, and lowering. |
| `interfaces.txt` | **Written — sync required** | Must be synchronized with effects, hashing/equality, and stdlib contracts. |
| `impl.txt` | **Written — sync required** | Must include privileged core/stdlib impl access for built-in lowercase types. |
| `properties.txt` | **Written — sync required** | Must include contextual `set` resolution. |
| `defer.txt` | **Written — sync required** | Must be synchronized with discard, panic, and cancellation cleanup. |
| `spread.txt` | **Written — sync required** | Must be synchronized with collection and shaped literals. |
| `flowcontrol_if.txt` | **Written** | |
| `flowcontrol_for.txt` | **Written — sync required** | Current Sema includes collection, map/set and rank-one `vector[T, N]` iteration; tensor and axis iteration still need synchronization. |
| `flowcontrol_for_1.txt` | **Covered** | Merged into `flowcontrol_for.txt`; no separate rulebook remains in `rules/`. |
| `flowcontrol_while.txt` | **Written** | |
| `flowcontrol_switch.txt` | **Written** | |
| `flowcontrol_match.txt` | **Written — sync required** | Current Sema covers Result, enum, union/Option matching and rejects pattern-binding shadowing; still needs panic/outcome and future collection-pattern synchronization. |

---

# 3. Collections, arrays, and shaped values

| Rulebook | Status | Notes |
|---|---|---|
| `arrays-slices.txt` | **Written — sync required** | Fixed-array defaults and slice non-defaultability are synchronized; common indexing and collection integration remain. |
| `collections-shaped-types.md` | **Written — sync required** | Empty list defaults/literals are canonical; constructors, APIs, shaped values and lowering remain incomplete. |
| `spread.txt` | **Written — sync required** | Collection expansion and literal integration. |
| `flowcontrol_for.txt` | **Written — sync required** | Iteration over collections and shaped values; rank-one `vector[T, N]` now participates in Sema iterable inference. |

The first-class language types are expected to include:

```sec
list[T]
list[T, Capacity]

map[K, V]
map[K, V, Capacity]

set[T]
set[T, Capacity]

vector[T, N]
matrix[T, Rows, Columns]
tensor[T, Dimensions...]
tensor_view[T, Rank]
```

The related nominal types include:

```sec
Shape[Rank]
Strides[Rank]
TensorLayout[Rank]
MemorySpace
```

The collection rulebook is not fully implemented until the required public APIs
and algorithms also exist in stdlib.

Expected stdlib data structures include at least:

```sec
Stack[T]
Queue[T]
Deque[T]
LinkedList[T]
RingBuffer[T, Capacity]

BinaryHeap[T]
PriorityQueue[T, Priority]

OrderedMap[K, V]
OrderedSet[T]
MultiMap[K, V]
MultiSet[T]
FlatMap[K, V]
FlatSet[T]

BitSet[N]
BloomFilter[T, Bits]
Trie[K, V]
RadixTree[K, V]

Tree[T]
BinaryTree[T]
Graph[Node, Edge]
DirectedAcyclicGraph[Node, Edge]

Complex[T]
Quaternion[T]
Polynomial[T]

Grid[T, Rows, Columns]
Image[T, Width, Height]
Volume[T, X, Y, Z]
```

## Remaining collection closure questions

The main type families are decided.

The remaining details to close are:

- map and set literal syntax;
- final equality and hashing interface names;
- exact capacity and allocation error taxonomy;
- dynamic owned tensor extents;
- source syntax for layout and memory-space policies;
- precise public stdlib API names;
- sparse layout policy API.

---

# 4. Ownership, borrowing, lifetime, and storage

| Rulebook | Status | Notes |
|---|---|---|
| `allocation.txt` | **Written — sync required** | Must be synchronized with collections, threads, explicit backing storage, and shaped buffers. |
| `ownership.md` | **Living** | Defines explicit move syntax; remaining collection and lifecycle integration is tracked in the rulebook. |
| `borrowing.txt` | **Written — sync required** | Must include views, thread-local references, and discard interactions. |
| `references.txt` | **Written — sync required** | Must include shaped views and thread-bound references. |
| `raw_pointers.txt` | **Written — sync required** | Must be synchronized with memory spaces, ABI, and unsafe rules. |
| `copy_move.md` | **Living** | Explicit move frontend is implemented; place-sensitive analysis and ownership IR remain incomplete. |
| `lifetime_analysis.txt` | **Written — sync required** | Must include detached work, thread-local values, views, and explicit storage. |
| `destruction.txt` | **Written — sync required** | Must include discard, panic, cancellation, collection elements, and TLS destruction. |
| `memory_model.md` | **Written — sync required** | Default lifetime/origin rules are synchronized; concurrency and volatile integration remain. |
| `static.md` | **Written — sync required** | Must include thread-local keys, collection backing storage, and initialization order. |
| `discard.md` | **Written — sync required** | General explicit consumption and early deterministic destruction. |
| `storage.md` | **Planned** | Storage classes, inline storage, stack, static, arena, allocator-backed storage, and explicit backing storage. |
| `layout.md` | **Planned** | Alignment, padding, packing, field order, shaped layouts, observable layout, and layout guarantees. |

`storage_allocation.txt` from the old checklist is replaced by the clearer
planned name:

```text
storage.md
```

Allocation policy remains in:

```text
allocation.txt
```

Physical storage categories and placement belong in:

```text
storage.md
```

---

# 5. Errors, panic, and runtime checks

| Rulebook | Status | Notes |
|---|---|---|
| `errorhandling.txt` | **Written — sync required** | Explicit `Result`, `Ok`, `Err`, `try`, and matching. |
| `panic.md` | **Written** | Canonical panic domains, containment, cleanup, assertions, checked unreachable, task/thread outcomes, panic information, no-panic verification, and runtime-free support model; exact explicit-panic and build-manifest syntax remain open. |
| `runtime_checks.md` | **Written** | Canonical checked-operation model, proof elimination, fallible `try` paths, panic-capable ordinary paths, typed propagation, no-panic integration, and runtime-free lowering requirements. |
| `core-library.md` | **Written — sync required** | Must include the compiler/core access model and all language-level core errors. |
| `core/errors.sec` | **Implementation artifact** | Every language-level runtime error type must be declared here. |

A runtime check does not imply a general managed runtime.

A check may lower to:

- inline comparisons and branches;
- a direct panic path;
- a target-specific trap;
- a profile-selected handler;
- a statically eliminated operation.

The canonical behavior is defined by `panic.md` and `runtime_checks.md`; their
implementation remains tracked separately.

---

# 6. Concurrency and asynchronous execution

| Rulebook | Status | Notes |
|---|---|---|
| `concurrency.md` | **Written — sync required** | Overview must be synchronized with the complete concurrency set. |
| `concurrency_memory_model.txt` | **Written — sync required** | Must remain aligned with tasks, threads, channels, atomics, and events. |
| `concurrency_runtime_model.md` | **Written — sync required** | Must include core errors and no-required-runtime profiles. |
| `tasks.txt` | **Written — sync required** | Must be synchronized with fallible spawn and discard. |
| `spawn.md` | **Written — sync required** | All spawn forms are fallible; process spawn is deferred. |
| `await.md` | **Written — sync required** | Must be synchronized with task outcome and cancellation. |
| `threads.md` | **Written — sync required** | Thread design is closed for Sec 0.1; implementation remains. |
| `thread_local.md` | **Written — sync required** | Physical thread-local storage and task-migration restrictions. |
| `scheduling.md` | **Written — sync required** | |
| `blocking.md` | **Written — sync required** | |
| `cancellation.md` | **Written — sync required** | |
| `structured_concurrency.md` | **Written — sync required** | |
| `transferability.md` | **Written — sync required** | |
| `data_races.md` | **Written — sync required** | |
| `deadlock_analysis.md` | **Written — sync required** | |
| `channels.md` | **Written — sync required** | |
| `events.md` | **Written — sync required** | C#-style publish/subscribe event model; distinct from readiness/completion. |
| `select.md` | **Written — sync required** | |
| `mutex.md` | **Written — sync required** | |
| `atomics.md` | **Written — sync required** | |
| `processes.txt` | **Written — deferred feature** | Rulebook exists, but `spawn process` design/implementation is intentionally postponed. |
| `ipc.md` | **Deferred** | Process communication is postponed with process spawning. |

Threads are considered design-complete for Sec 0.1.

The expected implementation order is:

```text
parser
AST
Sema
analysis
Semantic IR
MLIR and target lowering
```

Process spawning and IPC do not block the immediate language closure.

---

# 7. Type system, core, and standard library integration

| Rulebook | Status | Notes |
|---|---|---|
| `types.txt` | **Written — sync required** | Fundamental and named types. |
| `generics.txt` | **Written — sync required** | Type and compile-time value arguments. |
| `interfaces.txt` | **Written — sync required** | |
| `impl.txt` | **Written — sync required** | |
| `properties.txt` | **Written — sync required** | |
| `core-library.md` | **Written — sync required** | Compiler-known core declarations and privileged impl access. |
| `stdlib.md` | **Written — sync required** | Standard-library responsibilities, module boundaries, naming, target capability contracts, and compiler-recognized declaration rules. |

Built-in lowercase types may receive privileged implementations in core.

The standard library may use those types and provide higher-level nominal
types and algorithms, but it may not redefine or globally extend their
first-class member surface.

Examples:

```sec
impl string {
}

impl list[T] {
}

impl matrix[T, Rows, Columns] {
}
```

Ordinary user code may define implementations for its own nominal types, but may
not globally extend built-in lowercase types.

Nominal core, stdlib, and user types begin with uppercase letters:

```sec
Result[T, E]
Option[T]
Thread[T]
Shape[Rank]
Stack[T]
OrderedMap[K, V]
```

---

# 8. Compiler architecture, analysis, and IR

| Rulebook | Status | Notes |
|---|---|---|
| `compiler.txt` | **Written — sync required** | |
| `compiler_analysis.txt` | **Written — sync required** | High-level compiler analysis model. |
| `compiler_pipeline.txt` | **Written — sync required** | |
| `semantic_ir.txt` | **Written — sync required** | Must include discard, threads, collections, shaped values, panic, and effects. |
| `mlir.txt` | **Written — sync required** | |
| `mlir-optimize.txt` | **Living** | Updated as implementation and optimization support grows. |
| `rules_implementations.txt` | **Living** | Canonical implementation progress tracker. |
| `call_graph.md` | **Planned** | Complete direct and indirect call graph semantics and analysis. |
| `stack_analysis.md` | **Planned** | Per-function, per-path, whole-program, and target frame analysis. |
| `escape_analysis.md` | **Planned** | No silent stack-to-heap promotion; closure and reference escape rules. |
| `effect_analysis.md` | **Written** | Canonical compile-time effect domains, inference, call-graph propagation, verified guarantees, blocking/suspension distinction, arena events, trust provenance, diagnostics, and runtime-free model. |
| `isr_analysis.md` | **Planned** | ISR call graph, stack, allocation, blocking, lock, and shared-state restrictions. |
| `parser_recovery.md` | **Written** | Canonical deterministic recovery model; structured implementation remains partial. |
| `generics_lowering.md` | **Planned** | Semantic and MLIR lowering of generic code. |
| `monomorphization.md` | **Planned** | Specialization, symbol identity, code size, and cross-module behavior. |
| `linking.md` | **Planned** | Symbol identity, libraries, executables, dead stripping, and target linkage. |
| `compile_time_evaluation.md` | **Planned** | User-visible compile-time evaluation semantics. |

File extensions for new rulebooks should preferably converge on:

```text
.md
```

Existing `.txt` rulebooks do not need to be renamed immediately unless a
repository-wide migration is chosen.

---

# 9. Hardware, platform, interrupts, and ABI

| Rulebook | Status | Notes |
|---|---|---|
| `ffi.txt` | **Written — sync required** | Must be synchronized with ABI, layout, effects, panic, and foreign thread attachment. |
| `registers.txt` | **Written — sync required** | Must remain aligned with volatile/MMIO and target layout. |
| `abi.md` | **Planned** | Calling conventions, value representation, symbol ABI, FFI stability, and target differences. |
| `target_profiles.md` | **Planned** | Hosted, RTOS, bare-metal, allocation, concurrency, checks, and capability profiles. |
| `platform_model.md` | **Planned** | Platform views, target services, system calls, capabilities, and source selection. |
| `inline_assembly.md` | **Planned** | Operands, constraints, clobbers, volatility, memory effects, and target restrictions. |
| `volatile.md` | **Planned** | Volatile access, MMIO, compiler reordering, atomics distinction, and read-modify-write. |
| `interrupts.md` | **Planned** | ISR syntax, vector binding, nesting, priorities, stacks, and deferred work. |
| `isr_analysis.md` | **Planned** | Compiler verification for interrupt-safe code. |

This group is a central remaining language-closure block.

Storage, layout, ABI, FFI, volatile, registers, and interrupts must be designed
as one compatible model.

---

# 10. Modules, projects, initialization, and build

| Rulebook | Status | Notes |
|---|---|---|
| `projects.txt` | **Written — sync required** | Repository manifest, targets, outputs, internal directories, and build structure. |
| `modules.md` | **Planned** | Module identity, imports, cycles, visibility, and resolution. |
| `initialization.md` | **Planned** | Static/module initialization order, deinitialization, and failure behavior. |
| `linking.md` | **Planned** | Link-time symbol and output semantics. |

The following must eventually be defined coherently:

```text
module identity
import resolution
internal visibility
static initialization order
deinitialization order
entry points
generic symbol identity
FFI linkage
dead stripping
multiple target outputs
```

---

# 11. Diagnostics, formatter, and compiler tooling

| Rulebook | Status | Notes |
|---|---|---|
| `diagnostics.txt` | **Written — sync required** | Central diagnostic registry exists; LSP exposes parser/sema codes; first parser/sema IDs are migrated. Full diagnostic migration, localization and machine-readable CLI output remain. |
| `formatter.md` | **Written — sync required** | Default clauses, membership order and omission are synchronized; broader collection/shaped formatting remains. |
| `compiler_diagnostics.md` | **Covered** | Compiler diagnostic policy remains canonical in `diagnostics.txt`; avoid duplication. |
| `debug_information.md` | **Planned** | Source mapping, variables, optimized code, generics, async/task frames, and targets. |
| `compiler_testing.md` | **Planned** | Compiler unit, integration, invalid, regression, lowering, and backend tests. |
| `incremental_compilation.md` | **Planned** | Dependency invalidation, generic specialization caches, and target-aware rebuilds. |
| `lsp.md` | **Living** | Canonical language-server architecture and feature rulebook. |

---

# 12. Canonical rulebook set: written

The following rulebooks are currently considered written.

```text
allocation.txt
arrays-slices.txt
attributes.md
atomics.md
await.md
blocking.md
borrowing.txt
cancellation.md
channels.md
compiler.txt
compiler_analysis.txt
compiler_pipeline.txt
concurrency.md
concurrency_memory_model.txt
concurrency_runtime_model.md
copy_move.md
core-library.md
data_races.md
deadlock_analysis.md
defer.txt
default_values.md
destruction.txt
diagnostics.txt
enums.txt
errorhandling.txt
effect_analysis.md
events.md
ffi.txt
flowcontrol_for.txt
flowcontrol_if.txt
flowcontrol_match.txt
flowcontrol_switch.txt
flowcontrol_while.txt
formatter.md
functions.txt
functions_lambda.txt
generics.txt
impl.txt
interfaces.txt
language_philosophy.txt
lexical_structure.md
lifetime_analysis.txt
memory_model.md
mlir-optimize.txt
mlir.txt
mutex.md
operators.md
ownership.md
panic.md
processes.txt
projects.txt
properties.txt
raw_pointers.txt
references.txt
registers.txt
rules_implementations.txt
runtime_checks.md
scheduling.md
select.md
semantic_ir.txt
spawn.md
spread.txt
static.md
struct.txt
structured_concurrency.md
tasks.txt
threads.md
transferability.md
types.txt
unsafe.md
unions.txt
units.txt
contracts.md
lsp.md
grammar.md
parser_recovery.md
```

The following newer rulebooks are also written and present in the canonical
repository state:

```text
collections-shaped-types.md
thread_local.md
discard.md
```

---

# 13. Canonical rulebook set: planned

The following rulebooks are currently expected before Sec 0.1 can be considered
fully design-closed, unless a later decision explicitly merges one into another.

```text
storage.md
layout.md

call_graph.md
stack_analysis.md
escape_analysis.md
isr_analysis.md
generics_lowering.md
monomorphization.md
compile_time_evaluation.md

abi.md
target_profiles.md
platform_model.md
inline_assembly.md
volatile.md
interrupts.md

modules.md
initialization.md
linking.md

debug_information.md
compiler_testing.md
incremental_compilation.md
```

The following remains a candidate until its value as a separate rulebook is
confirmed:

```text
stdlib.md
```

---

# 14. Deferred areas

The following areas are intentionally deferred and do not block immediate Sec
0.1 closure work:

```text
spawn process implementation
IPC
general process supervision
dynamic-rank tensors
arbitrary user-defined operator overloading
```

A deferred area must still be listed so it is not mistaken for an accidental
omission.

---

# 15. Open design-space register

This section tracks unresolved language surface rather than missing documents.

## Panic and runtime failure

`panic.md` and `runtime_checks.md` now define panic domains, containment,
cleanup, no-panic defer and destruction, foreign-boundary restrictions,
task/thread outcomes, assertions, checked unreachable, checked-operation
outcomes, typed fallible paths, and the no-mandatory-runtime model.

Still to decide or lock in their owning rulebooks:

- exact explicit `panic` source syntax and payload shape;
- exact build-manifest syntax for root panic strategy and required no-panic
  entrypoints;
- exact supervisor source syntax in the concurrency rulebooks.

## Storage, layout, and ABI

Still to decide:

- observable versus compiler-private layout;
- alignment and padding;
- packed layout;
- endianness;
- union and enum layout;
- static, stack, arena, inline, and allocator-backed storage;
- shaped memory spaces;
- FFI-stable representation;
- ABI guarantees by target.

## Attributes and effects

`attributes.md` now defines the closed Sec 0.1 attribute inventory and excludes
custom user annotations. `effect_analysis.md` defines inferred effects,
verified guarantees, interface compatibility, conservative indirect effects,
and per-compilation-plan propagation.

Still deliberately deferred:

- effect-constrained function-type syntax;
- generic effect-constraint syntax;
- unsafe foreign effect-declaration syntax;
- inline-assembly effect-declaration syntax;
- future `noSuspend`, purity, determinism, arena-split, and visible-region
  source forms.

## Platform and hardware

`attributes.md` now defines target-selection and interrupt-binding attribute
syntax. Still to decide in the remaining platform rulebooks:

- volatile and MMIO semantics;
- inline assembly;
- native platform views;
- target capability queries.

## Closures

Still to decide for Sec 0.1:

- immutable value capture only versus fuller capture support;
- mutable captures;
- reference captures;
- escaping environment storage;
- callable mutability;
- task/thread transfer.

The canonical rule remains `functions_lambda.txt`.

## Compile-time evaluation and generics

Still to decide:

- user-defined compile-time execution;
- allocation and I/O restrictions;
- compile-time panic;
- recursion and loop limits;
- generic specialization;
- monomorphization guarantees;
- cross-module generic ABI.

## Collections and shaped types

Still to decide:

- literal syntax;
- final equality/hash contract names;
- dynamic owned tensor extents;
- layout policy syntax;
- memory-space syntax;
- final collection error types;
- sparse policy API.

## Modules and initialization

Still to decide:

- module identity and duplicate resolution;
- import cycles;
- static/module initialization order;
- deinitialization;
- initialization failure;
- symbol identity and mangling.

## Explicit Sec 0.1 exclusions

The following must be either included or explicitly marked as excluded:

```text
macros
compile-time reflection
runtime reflection
tuples
multiple return values
variadic functions
default parameters
named arguments
implicit conversions
inheritance
exceptions
garbage collection
dynamic-rank tensors
process spawning
IPC
general user-defined operator overloading
```

---

# 16. Language-design closure criterion

Sec 0.1 may be declared design-complete when:

1. every expected rulebook is written, covered, merged, or explicitly deferred;
2. no canonical rulebook leaves programmer-visible syntax or behavior as an
   unresolved placeholder;
3. no two canonical rulebooks contradict each other;
4. every runtime failure has a typed error, panic path, trap, or compile-time
   diagnostic;
5. every target-dependent feature has explicit capability and fallback
   semantics;
6. core and stdlib responsibilities are defined for every compiler-known type;
7. a canonical grammar and operator precedence table exist;
8. the explicit exclusions for Sec 0.1 are recorded;
9. documentation status remains separate from compiler implementation status.

After design closure, implementation may continue phase by phase without
reopening language semantics unless an implementation finding proves a genuine
design defect.
