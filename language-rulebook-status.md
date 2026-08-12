# Sec Language Rulebook Status

## Purpose

This document is the canonical inventory of the rulebooks expected for the Sec
language, compiler, target model, core library, and standard library.

It replaces the former temporary `temp2.txt` checklist.

This document tracks documentation status only.

The physical rulebook layout and directory responsibilities are indexed by
`rules/README.md`. Paths in this inventory are relative to `rules/` unless a
repository-root path is written explicitly.

It does not claim that a written rulebook has been implemented by the compiler.

Implementation progress is governed by:

```text
implementation-status.yaml
```

Rulebooks contain normative requirements and must not duplicate the current
repository implementation state. Existing status sections are migrated one
rulebook at a time into the implementation ledger.

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
collections/collections.md
collections/shaped-types.md
concurrency/thread_local.md
control-flow/discard.md
memory/ownership.md
tooling/lsp.md
tooling/formatter.md
memory/copy_move.md
memory/memory_model.md
foundations/operators.md
types/default_values.md
types/contracts.md
foundations/grammar.md
foundations/attributes.md
errors/panic.md
errors/runtime_checks.md
analysis/effect_analysis.md
memory/unsafe.md
memory/reference_model.md
analysis/call_graph.md
memory/arena.md
memory/storage.md
memory/layout.md
compiler/compiler_known_members.md
analysis/escape_analysis.md
analysis/closure_analysis.md
analysis/parameter_usage_analysis.md
analysis/pitfall_analysis.md
analysis/stack_analysis.md
analysis/data_races.md
analysis/deadlock_analysis.md
analysis/isr_analysis.md
```

These were written after the older temporary checklist was last synchronized.

---

# 1. Language foundations and lexical rules

| Rulebook | Status | Notes |
|---|---|---|
| `foundations/language_philosophy.md` | **Written** | Core language direction and design principles. |
| `foundations/lexical_structure.md` | **Written** | Canonical lexical rules; implementation status is tracked by `frontend.lexical-structure` in `implementation-status.yaml`. |
| `types/types.md` | **Written** | Canonical replacement for the retired `types.txt`; implementation is tracked by `frontend.types-core`, `frontend.literal-family-suffix-v2`, `frontend.temporal-builtin-types`, and `frontend.wide-numeric-language-types`. |
| `types/contracts.md` | **Written** | Canonical named-type contracts; replaces the obsolete variable-contract model. |
| `types/default_values.md` | **Written** | Canonical primitive, constrained, aggregate, list and explicit-default semantics. |
| `types/units.txt` | **Written — sync required** | Direct conversion dimension validation is implemented; shaped arithmetic, scale paths, and matrix multiplication still require synchronization. |
| `foundations/grammar.md` | **Written** | Canonical consolidated grammar for Sec 0.1. |
| `foundations/operators.md` | **Written** | Canonical operator semantics; compiler progress belongs in `implementation-status.yaml`. |
| `foundations/names_scopes_visibility.md` | **Written — sync required** | Top-level module declaration namespace conflicts are partially implemented; remaining scope, visibility, reserved-name and naming-rule audit still needed. |
| `foundations/attributes.md` | **Written** | Canonical closed Sec 0.1 attribute set, syntax, attachment, selection, target binding, `@noCopy`, verified guarantees, conflicts, formatter/LSP behavior, and explicit implementation status. |
| `memory/unsafe.md` | **Written** | Canonical unsafe contexts, operations, functions and extern declarations, caller obligations, raw pointers, trust boundaries and provenance; compiler support remains partial. |

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
foundations/lexical_structure.md
foundations/grammar.md
types/types.md
declarations/properties.txt
collections/collections.md
collections/shaped-types.md
tooling/formatter.md
VS Code grammar
LSP token classification
```

---

# 2. Declarations, functions, and control flow

| Rulebook | Status | Notes |
|---|---|---|
| `declarations/struct.txt` | **Written** | Includes recursive semantic initialization of omitted fields. |
| `declarations/enums.txt` | **Written — sync required** | Must remain aligned with `bit[N]`, aliases, and `iota`. |
| `declarations/unions.txt` | **Written** | Tagged union rules; recursive payload-derived copyability is implemented. Same-type equality currently uses general nominal compatibility, but payload-derived comparability remains pending. |
| `platform/registers.txt` | **Written** | Register declarations, fields, widths, and reserved `_` bits. |
| `declarations/functions.txt` | **Written — sync required** | Must be synchronized with discard of non-void results and effects. |
| `declarations/functions_lambda.txt` | **Written — sync required** | Canonical lambda and closure rulebook; closure scope for 0.1 must be finalized. |
| `lambdas.md` | **Covered** | Covered by `declarations/functions_lambda.txt`; no duplicate rulebook expected. |
| `closures.md` | **Covered** | Covered by `declarations/functions_lambda.txt`; no duplicate rulebook expected. |
| `declarations/generics.txt` | **Written — sync required** | Must be synchronized with compile-time values, collections, register widths, and lowering. |
| `declarations/interfaces.txt` | **Written — sync required** | Interface inheritance-cycle detection is implemented; effects, hashing/equality, and stdlib contracts still require synchronization. |
| `declarations/impl.txt` | **Written — sync required** | Must include privileged core/stdlib impl access for built-in lowercase types. |
| `declarations/properties.txt` | **Written — sync required** | Must include contextual `set` resolution. |
| `control-flow/defer.txt` | **Written — sync required** | Must be synchronized with discard, panic, and cancellation cleanup. |
| `declarations/spread.txt` | **Written — sync required** | Must be synchronized with collection and shaped literals. |
| `control-flow/flowcontrol_if.txt` | **Written** | |
| `control-flow/flowcontrol_for.txt` | **Written — sync required** | Current Sema includes collection, map/set and rank-one `vector[T, N]` iteration; tensor and axis iteration still need synchronization. |
| `flowcontrol_for_1.txt` | **Covered** | Merged into `control-flow/flowcontrol_for.txt`; no separate rulebook remains in `rules/`. |
| `control-flow/flowcontrol_while.txt` | **Written** | |
| `control-flow/flowcontrol_switch.txt` | **Written** | String-literal duplicate cases and non-exhaustive enum coverage warnings are implemented in Sema. |
| `control-flow/flowcontrol_match.txt` | **Written — sync required** | Current Sema covers Result, enum, union/Option matching and rejects pattern-binding shadowing; still needs panic/outcome and future collection-pattern synchronization. |

---

# 3. Collections, arrays, and shaped values

| Rulebook | Status | Notes |
|---|---|---|
| `collections/collections.md` | **Written** | Canonical fixed-array, owning dynamic-array, slice, list, map, and set semantics. |
| `collections/shaped-types.md` | **Written** | Canonical shaped values, views, layout, and memory-space semantics. |
| `declarations/spread.txt` | **Written — sync required** | Collection expansion and literal integration. |
| `control-flow/flowcontrol_for.txt` | **Written — sync required** | Iteration over collections and shaped values; rank-one `vector[T, N]` now participates in Sema iterable inference. |

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
| `memory/allocation.txt` | **Written — sync required** | Must be synchronized with collections, threads, explicit backing storage, and shaped buffers. |
| `memory/arena.md` | **Written** | Canonical Arena ownership, backing, allocation, reset/release, validity epoch, effects, analysis and lowering model. Recognized operations now produce direct graph events, synchronous `MayAllocate` summaries, cause paths, and LSP hover; context, demand, dependency, and lowering work remains partial. |
| `memory/ownership.md` | **Living** | Defines explicit move syntax; remaining collection and lifecycle integration is tracked in the rulebook. |
| `memory/borrowing.txt` | **Written — sync required** | Must include views, thread-local references, and discard interactions. |
| `memory/references.txt` | **Written — sync required** | Must include shaped views and thread-bound references. |
| `memory/reference_model.md` | **Written** | Canonical safe-reference guarantees, validity epochs, stable and weak handles, relocation, profile representations, and `RawPtr` boundaries. |
| `generational_references.md` | **Covered** | Generational validity is canonical in `memory/reference_model.md`; no separate rulebook is required. |
| `memory/raw_pointers.txt` | **Written — sync required** | Must be synchronized with memory spaces, ABI, and unsafe rules. |
| `memory/copy_move.md` | **Living** | Canonical copy/move semantics. Implementation status is tracked by `frontend.copy-move` in `implementation-status.yaml`. |
| `memory/lifetime_analysis.txt` | **Written — sync required** | Must include detached work, thread-local values, views, and explicit storage. |
| `memory/destruction.txt` | **Written — sync required** | Must include discard, panic, cancellation, collection elements, and TLS destruction. |
| `memory/memory_model.md` | **Written** | Canonical source and compiler memory model, including default lifetime and origin rules. |
| `declarations/static.md` | **Written — sync required** | Must include thread-local keys, collection backing storage, and initialization order. |
| `control-flow/discard.md` | **Written — sync required** | General explicit consumption and early deterministic destruction. |
| `memory/storage.md` | **Written** | Canonical storage origin, backing relation, reclamation authority, address stability, regions, invalidation domains, validity epochs, memory spaces, and placement guarantees; implementation remains partial. |
| `memory/layout.md` | **Written** | Canonical semantic/native layout, size, alignment, stride, padding, aggregate representation, explicit contracts, plan-specific queries, and compatibility; the shared layout phase remains partial. |

`storage_allocation.txt` from the old checklist is replaced by the clearer
canonical rulebook:

```text
memory/storage.md
```

Allocation policy remains in:

```text
memory/allocation.txt
```

Physical storage categories and placement belong in:

```text
memory/storage.md
```

---

# 5. Errors, panic, and runtime checks

| Rulebook | Status | Notes |
|---|---|---|
| `errors/errorhandling.txt` | **Written — sync required** | Explicit `Result`, `Ok`, `Err`, `try`, and matching. |
| `errors/panic.md` | **Written** | Canonical panic domains, containment, cleanup, assertions, checked unreachable, task/thread outcomes, panic information, no-panic verification, and runtime-free support model; exact explicit-panic and build-manifest syntax remain open. |
| `errors/runtime_checks.md` | **Written** | Canonical checked-operation model, proof elimination, fallible `try` paths, panic-capable ordinary paths, typed propagation, no-panic integration, and runtime-free lowering requirements. |
| `library/core-library.md` | **Written — sync required** | Must include the compiler/core access model and all language-level core errors. |
| `core/errors.sec` | **Implementation artifact** | Every language-level runtime error type must be declared here. |

A runtime check does not imply a general managed runtime.

A check may lower to:

- inline comparisons and branches;
- a direct panic path;
- a target-specific trap;
- a profile-selected handler;
- a statically eliminated operation.

The canonical behavior is defined by `errors/panic.md` and `errors/runtime_checks.md`; their
implementation remains tracked separately.

---

# 6. Concurrency and asynchronous execution

| Rulebook | Status | Notes |
|---|---|---|
| `concurrency/concurrency.md` | **Written — sync required** | Overview must be synchronized with the complete concurrency set. |
| `concurrency/concurrency_memory_model.md` | **Written — sync required** | Must remain aligned with tasks, threads, channels, atomics, and events. |
| `concurrency/concurrency_runtime_model.md` | **Written — sync required** | Must include core errors and no-required-runtime profiles. |
| `concurrency/tasks.txt` | **Written — sync required** | Must be synchronized with fallible spawn and discard. |
| `concurrency/spawn.md` | **Written — sync required** | All spawn forms are fallible; process spawn is deferred. |
| `concurrency/await.md` | **Written — sync required** | Must be synchronized with task outcome and cancellation. |
| `concurrency/threads.md` | **Written — sync required** | Thread design is closed for Sec 0.1; implementation remains. |
| `concurrency/thread_local.md` | **Written — sync required** | Physical thread-local storage and task-migration restrictions. |
| `concurrency/scheduling.md` | **Written — sync required** | |
| `concurrency/blocking.md` | **Written — sync required** | |
| `concurrency/cancellation.md` | **Written — sync required** | |
| `concurrency/structured_concurrency.md` | **Written — sync required** | |
| `memory/transferability.md` | **Written — sync required** | |
| `analysis/data_races.md` | **Written** | Canonical data-race analysis rules; implementation status is tracked by `sema.data-race-analysis` in `implementation-status.yaml`. |
| `analysis/deadlock_analysis.md` | **Written** | Canonical deadlock-analysis rules; implementation status is tracked by `sema.deadlock-analysis` in `implementation-status.yaml`. |
| `concurrency/channels.md` | **Written — sync required** | |
| `concurrency/events.md` | **Written — sync required** | C#-style publish/subscribe event model; distinct from readiness/completion. |
| `concurrency/select.md` | **Written — sync required** | |
| `concurrency/mutex.md` | **Written — sync required** | |
| `concurrency/atomics.md` | **Written — sync required** | |
| `concurrency/processes.txt` | **Written — planned feature** | Rulebook exists, but `spawn process` design/implementation is intentionally postponed. |
| `ipc.md` | **Planned** | Process communication is postponed with process spawning. |

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
| `types/types.md` | **Written** | Canonical fundamental, scalar, temporal, named, collection-shaped, and declaration type contract; implementation status is maintained in `implementation-status.yaml`. |
| `declarations/generics.txt` | **Written — sync required** | Type and compile-time value arguments. |
| `declarations/interfaces.txt` | **Written — sync required** | |
| `declarations/impl.txt` | **Written — sync required** | |
| `declarations/properties.txt` | **Written — sync required** | |
| `library/core-library.md` | **Written — sync required** | Compiler-known core declarations and privileged impl access. |
| `compiler/compiler_known_members.md` | **Written** | Canonical typed registry, stable member identities, lookup, builtin member surface, core boundary and tooling behavior. Initial Sema/LSP registry integration is implemented. |
| `library/stdlib.md` | **Written — partially implemented** | Standard-library boundaries and target contracts plus Linux/amd64 streaming file IO, exact and complete caller-buffer reads, writes/copy, seek/flush/truncate/close, directory iteration, non-recursive path operations, path bridging, and explicit resource lifecycle diagnostics. Allocating complete-file and directory-list APIs remain pending. |

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
| `compiler/compiler.txt` | **Written — sync required** | |
| `compiler/compiler_analysis.txt` | **Written — sync required** | High-level compiler analysis model. |
| `compiler/compiler_pipeline.txt` | **Written — sync required** | |
| `compiler/semantic_ir.txt` | **Written — sync required** | Must include discard, threads, collections, shaped values, panic, and effects. |
| `compiler/rules_implementations.txt` | **Living** | Legacy implementation notes being migrated into `implementation-status.yaml`. |
| `analysis/call_graph.md` | **Written** | Canonical callable reachability and execution relationships. Implementation status is tracked by `analysis.call-graph` in `implementation-status.yaml`. |
| `analysis/stack_analysis.md` | **Written** | Canonical semantic and machine stack-resource analysis; implementation status is tracked by `sema.stack-analysis` in `implementation-status.yaml`. |
| `analysis/escape_analysis.md` | **Written** | Canonical escape subjects, destinations, retention, summaries, diagnostics, and no-silent-promotion contract. Implementation status is tracked by `sema.escape-analysis`. |
| `analysis/closure_analysis.md` | **Written** | Canonical capture, callable-flow, target-set, closure-summary, and analysis-budget model. Implementation status is tracked by `sema.closure-analysis`. |
| `analysis/parameter_usage_analysis.md` | **Written** | Canonical multidimensional parameter-demand and advisory-narrowing model. Implementation status is tracked by `sema.parameter-usage-analysis`. |
| `analysis/pitfall_analysis.md` | **Written** | Canonical semantic pitfall-finding, evidence, suppression, confidence, corrective-action, budget, incremental, LSP, and FFI-contract model. Implementation status is tracked by `sema.pitfall-analysis`. |
| `analysis/effect_analysis.md` | **Written** | Canonical compile-time effect domains and propagation model. Initial Arena event sites, synchronous `MayAllocate` propagation, cause paths, and LSP hover are implemented; the complete effect set, guarantees, contexts, indirect targets, and per-plan analysis remain. |
| `analysis/isr_analysis.md` | **Written** | Canonical cross-analysis ISR constraint verifier: resolved profiles, reusable requirement summaries, execution-context propagation, stack/race/deadlock/FFI composition, Valid/Invalid/Unproven proof states, incremental dependencies, and progressive LSP refinement. Implementation status is tracked by `sema.isr-analysis`. |
| `compiler/parser_recovery.md` | **Written** | Canonical deterministic recovery model; structured implementation remains partial. |
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

# 9. Sec MLIR dialect and lowering

The Sec MLIR material has its own category because governance, current
specifications, historical schema snapshots, Semantic IR amendments, normative
synchronization notes, and bounded implementation packages have different
lifecycles.

| Rulebook group | Status | Notes |
|---|---|---|
| `mlir/sec_mlir.md` | **Written** | Governance and canonical high-level Sec MLIR boundary. |
| `mlir/sec_mlir_dialect.md` | **Written** | Current canonical dialect specification. |
| `mlir/sec_mlir_lowering.md` | **Written** | Current canonical lowering specification. |
| `mlir/mlir.txt` | **Written — sync required** | General MLIR architecture notes. |
| `mlir/mlir-optimize.txt` | **Living** | Updated as optimization support grows. |
| `mlir/packages/` | **Living** | Numbered implementation packages 1–19 with separate package YAML files. |
| `mlir/dialect-versions/` | **Written** | Historical dialect snapshots retained for schema history and regression work; not the current canonical specification. |
| `mlir/lowering-versions/` | **Written** | Historical lowering snapshots retained for compatibility and regression work. |
| `mlir/semantic-ir/` | **Written** | Package-scoped amendments to the canonical Semantic IR rulebook. |
| `mlir/normative-sync/` | **Written** | Package-scoped synchronization amendments for non-MLIR rulebooks. |

Implementation state remains in `implementation-status.yaml` and in the
package-local YAML files under `mlir/packages/`; it must not be inferred merely
from the presence of a versioned document.

---

# 10. Hardware, platform, interrupts, and ABI

| Rulebook | Status | Notes |
|---|---|---|
| `platform/ffi.txt` | **Written — sync required** | Must be synchronized with ABI, layout, effects, panic, and foreign thread attachment. |
| `platform/registers.txt` | **Written — sync required** | Must remain aligned with volatile/MMIO and target layout. |
| `abi.md` | **Planned** | Calling conventions, value representation, symbol ABI, FFI stability, and target differences. |
| `target_profiles.md` | **Planned** | Hosted, RTOS, bare-metal, allocation, concurrency, checks, and capability profiles. |
| `platform_model.md` | **Planned** | Platform views, target services, system calls, capabilities, and source selection. |
| `inline_assembly.md` | **Planned** | Operands, constraints, clobbers, volatility, memory effects, and target restrictions. |
| `volatile.md` | **Planned** | Volatile access, MMIO, compiler reordering, atomics distinction, and read-modify-write. |
| `interrupts.md` | **Planned** | ISR syntax, vector binding, nesting, priorities, stacks, and deferred work. |
| `analysis/isr_analysis.md` | **Written** | Compiler verification for profile-scoped interrupt safety using canonical analysis results; implementation status is tracked by `sema.isr-analysis`. |

This group is a central remaining language-closure block.

Storage, layout, ABI, FFI, volatile, registers, and interrupts must be designed
as one compatible model.

---

# 11. Modules, projects, initialization, and build

| Rulebook | Status | Notes |
|---|---|---|
| `projects/projects.txt` | **Written — sync required** | Repository manifest, targets, outputs, internal directories, and build structure. |
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

# 12. Diagnostics, formatter, and compiler tooling

| Rulebook | Status | Notes |
|---|---|---|
| `tooling/diagnostics.txt` | **Written — sync required** | Central registry and `sec diagnostics [--json]` expose all registered definitions plus complete definition/parser/sema/token field schemas; LSP exposes parser/sema codes. Full ID migration, localization and machine-readable emitted-diagnostic output remain. |
| `tooling/formatter.md` | **Written** | Canonical formatting behavior; implementation progress belongs in `implementation-status.yaml`. |
| `compiler_diagnostics.md` | **Covered** | Compiler diagnostic policy remains canonical in `tooling/diagnostics.txt`; avoid duplication. |
| `debug_information.md` | **Planned** | Source mapping, variables, optimized code, generics, async/task frames, and targets. |
| `compiler_testing.md` | **Planned** | Compiler unit, integration, invalid, regression, lowering, and backend tests. |
| `incremental_compilation.md` | **Planned** | Dependency invalidation, generic specialization caches, and target-aware rebuilds. |
| `tooling/lsp.md` | **Living** | Canonical language-server architecture and feature rulebook; definition, Sema-backed highlights, direct/task/thread call hierarchy, and compiler-owned root/recursion/spawn/Arena-allocation hover summaries are implemented, while the workspace index, complete per-plan graph views, and broader navigation/reference features remain partial. |

---

# 13. Canonical rulebook set: written

The following rulebooks are currently considered written.

```text
memory/allocation.txt
collections/collections.md
memory/arena.md
foundations/attributes.md
concurrency/atomics.md
concurrency/await.md
concurrency/blocking.md
memory/borrowing.txt
concurrency/cancellation.md
analysis/call_graph.md
concurrency/channels.md
compiler/compiler.txt
compiler/compiler_analysis.txt
compiler/compiler_pipeline.txt
compiler/compiler_known_members.md
concurrency/concurrency.md
concurrency/concurrency_memory_model.md
concurrency/concurrency_runtime_model.md
memory/copy_move.md
library/core-library.md
control-flow/defer.txt
types/default_values.md
memory/destruction.txt
tooling/diagnostics.txt
declarations/enums.txt
errors/errorhandling.txt
analysis/effect_analysis.md
concurrency/events.md
platform/ffi.txt
control-flow/flowcontrol_for.txt
control-flow/flowcontrol_if.txt
control-flow/flowcontrol_match.txt
control-flow/flowcontrol_switch.txt
control-flow/flowcontrol_while.txt
tooling/formatter.md
declarations/functions.txt
declarations/functions_lambda.txt
declarations/generics.txt
declarations/impl.txt
declarations/interfaces.txt
analysis/isr_analysis.md
foundations/language_philosophy.md
foundations/lexical_structure.md
memory/layout.md
memory/lifetime_analysis.txt
memory/memory_model.md
mlir/mlir-optimize.txt
mlir/mlir.txt
mlir/sec_mlir.md
mlir/sec_mlir_dialect.md
mlir/sec_mlir_lowering.md
concurrency/mutex.md
foundations/operators.md
memory/ownership.md
errors/panic.md
concurrency/processes.txt
projects/projects.txt
declarations/properties.txt
memory/raw_pointers.txt
memory/reference_model.md
memory/references.txt
platform/registers.txt
compiler/rules_implementations.txt
errors/runtime_checks.md
concurrency/scheduling.md
concurrency/select.md
compiler/semantic_ir.txt
concurrency/spawn.md
declarations/spread.txt
declarations/static.md
memory/storage.md
declarations/struct.txt
concurrency/structured_concurrency.md
concurrency/tasks.txt
concurrency/threads.md
memory/transferability.md
types/types.md
memory/unsafe.md
declarations/unions.txt
types/units.txt
types/contracts.md
tooling/lsp.md
foundations/grammar.md
compiler/parser_recovery.md
```

The following newer rulebooks are also written and present in the canonical
repository state:

```text
collections/collections.md
collections/shaped-types.md
concurrency/thread_local.md
control-flow/discard.md
analysis/escape_analysis.md
analysis/closure_analysis.md
analysis/parameter_usage_analysis.md
analysis/pitfall_analysis.md
analysis/stack_analysis.md
analysis/data_races.md
analysis/deadlock_analysis.md
analysis/isr_analysis.md
```

The package-local MLIR documents under `mlir/packages/`, historical dialect and
lowering snapshots, Semantic IR amendments, and normative synchronization notes
are indexed as document groups in section 9 rather than duplicated file by file
in this canonical-rulebook list.

---

# 14. Canonical rulebook set: planned

The following rulebooks are currently expected before Sec 0.1 can be considered
fully design-closed, unless a later decision explicitly merges one into another.

```text
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
library/stdlib.md
```

---

# 15. Deferred areas

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

# 16. Open design-space register

This section tracks unresolved language surface rather than missing documents.

## Panic and runtime failure

`errors/panic.md` and `errors/runtime_checks.md` now define panic domains, containment,
cleanup, no-panic defer and destruction, foreign-boundary restrictions,
task/thread outcomes, assertions, checked unreachable, checked-operation
outcomes, typed fallible paths, and the no-mandatory-runtime model.

Still to decide or lock in their owning rulebooks:

- exact explicit `panic` source syntax and payload shape;
- exact build-manifest syntax for root panic strategy and required no-panic
  entrypoints;
- exact supervisor source syntax in the concurrency rulebooks.

## Storage, layout, and ABI

`memory/storage.md` now defines storage origin, backing relations, reclamation
authority, address stability, memory spaces, invalidation domains, validity
epochs, and storage-domain transitions. `memory/layout.md` now defines semantic and
native layout, alignment, padding, stride, field order, packing and endianness
semantics, aggregate representations, layout compatibility, and plan-specific
layout queries.

Still to decide or lock in the owning syntax, ABI, FFI, and target rulebooks:

- final source syntax for explicit layout, packing, alignment, field offsets,
  endianness, and memory-space contracts;
- exact FFI-stable representation contracts and their source attachment;
- ABI guarantees by target.

## Attributes and effects

`foundations/attributes.md` now defines the closed Sec 0.1 attribute inventory and excludes
custom user annotations. `analysis/effect_analysis.md` defines inferred effects,
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

`foundations/attributes.md` now defines target-selection and interrupt-binding attribute
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

The canonical rule remains `declarations/functions_lambda.txt`.

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

# 17. Language-design closure criterion

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
