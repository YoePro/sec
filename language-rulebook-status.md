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
declarations/generics.md
compiler/generics_lowering.md
compiler/monomorphization.md
declarations/static.md
memory/ownership.md
memory/borrowing.md
tooling/lsp.md
tooling/formatter.md
memory/copy_move.md
memory/destruction.md
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
platform/volatile.md
platform/hardware-register-access.md
platform/interrupts.md
platform/inline_assembly.md
compiler/compiler.md
compiler/compiler_analysis.md
compiler/compiler_pipeline.md
compiler/semantic_ir.md
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
| `types/default_values.md` | **Written** | Canonical primitive, constrained, aggregate, list and explicit-default semantics, including declared-member defaults for integer-, string-, and bit-backed enums. |
| `types/units.md` | **Written** | Canonical revision 2.0 carrier-independent unit model; implementation progress is tracked by `frontend.units-v2` and `stdlib.units-catalog` in `implementation-status.yaml`. |
| `foundations/grammar.md` | **Written** | Canonical consolidated grammar for Sec 0.1, including string-backed enum declarations and canonical immutable associated `let` members. |
| `foundations/operators.md` | **Written** | Canonical operator semantics; compiler progress belongs in `implementation-status.yaml`. |
| `foundations/names_scopes_visibility.md` | **Written — sync required** | Defines one member identity for canonical immutable impl `let` and compatibility `static let`; top-level conflicts are partially implemented and the remaining scope, visibility, reserved-name and naming-rule audit is still needed. |
| `foundations/attributes.md` | **Written** | Canonical closed Sec 0.1 attribute set, syntax, attachment, selection, target binding, `@noCopy`, verified guarantees, conflicts, formatter/LSP behavior, and explicit implementation status. |
| `memory/unsafe.md` | **Written** | Canonical revision 2.0 unsafe contexts, operation-level obligations, unsafe functions/extern calls, trusted declarations, safe wrappers, trust provenance, target/FFI/assembly boundaries, Semantic IR, lowering, diagnostics, and tooling. Implementation progress is tracked by the six `*.unsafe` entries in `implementation-status.yaml`. |

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
declarations/properties.md
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
| `declarations/struct.md` | **Written** | Canonical named aggregate declarations, field tags, complete/partial/spread construction, recursive defaults, field Places, copy/move, equality, destruction, and layout boundaries. Implementation is tracked by `frontend.structs`. |
| `declarations/enums.md` | **Written** | Revision 2.1 adds compile-time generic nominal enum templates and concrete owner identity while retaining the closed integer/string and open bit-backed enum model. Implementation is tracked by `frontend.enums` and `frontend.generics-v2`. |
| `declarations/unions.md` | **Written** | Canonical closed nominal unions, payload construction, explicit defaults, empty initialization state, matching, ownership, equality, generics, recursion, and representation requirements. Implementation is tracked by `frontend.unions`. |
| `declarations/registers.md` | **Written** | Canonical nominal `register[N]` types, logical bit layout, nested registers, field access semantics, conversions, and impl eligibility. Implementation is tracked by `frontend.registers`. |
| `declarations/functions.md` | **Written** | Canonical revision 2.0 functions, owned/borrowed/consuming parameters, call-transfer commit, overloads, and native typed variadics. Frontend and lowering progress is tracked by `frontend.functions-v2`. |
| `declarations/lambda-functions.md` | **Written** | Canonical revision 2.0 lambda, capture, callable-capability, and closure rulebook. Frontend and lowering progress is tracked by `frontend.lambda-functions-v2`. |
| `lambdas.md` | **Covered** | Covered by `declarations/lambda-functions.md`; no duplicate rulebook expected. |
| `closures.md` | **Covered** | Covered by `declarations/lambda-functions.md`; no duplicate rulebook expected. |
| `declarations/generics.md` | **Written** | Canonical revision 2.1 compile-time type generics, now including generic enums with phantom nominal identity parameters, concrete member owners, ordinary generic impl scope, and no runtime generic machinery. Implementation progress is tracked by `frontend.generics-v2`. |
| `declarations/interfaces.md` | **Written** | Revision 2.1 includes compiler-known generic interfaces such as statically dispatched `Iterator[T]`; ordinary receiver capabilities, inheritance, and primary-impl conformance remain canonical. Frontend progress is tracked by `frontend.interfaces` and `frontend.for-loops-v2`. |
| `declarations/impl.md` | **Written** | Revision 2.1 additionally makes immutable impl `let` the canonical associated-value spelling and immutable impl `static let` its equivalent compatibility form; lifecycle status is tracked by `frontend.impl-lifecycle-construction` and associated values by `frontend.static-declarations-members`. |
| `declarations/properties.md` | **Written** | Canonical property declarations, explicit setter parameters, fallible setters, impl fragments, static properties, and interface requirements. Frontend progress is tracked by `frontend.properties`. |
| `control-flow/defer.md` | **Written** | Canonical revision 2.0 invocation-scoped deferred cleanup, unified LIFO ordering with automatic destruction, lifetime extension, forbidden control transfer, and callable-context boundaries. Implementation progress is tracked by `frontend.defer-v2`. |
| `declarations/spread.md` | **Written** | Canonical postfix spread for fixed-array calls/literals and same-type struct construction. Frontend progress is tracked by `frontend.spread`. |
| `control-flow/flowcontrol_if.md` | **Written** | Canonical revision 2.0 `if`/`else if`/`else` semantics, boolean-only conditions, short-circuiting, state-test boundaries, branch flow, and diagnostics. Implementation progress is tracked by `frontend.if-statements-v2`. |
| `control-flow/flowcontrol_for.md` | **Written** | Revision 2.1 adds explicit compiler-known `Iterator[T]` conformance and static `Next() Option[T]` iteration without runtime dispatch; infinite, range, collection, ownership, flow, and cleanup rules remain canonical. Implementation progress is tracked by `frontend.for-loops-v2`. |
| `flowcontrol_for_1.txt` | **Covered** | Merged into `control-flow/flowcontrol_for.md`; no separate rulebook remains in `rules/`. |
| `control-flow/flowcontrol_while.md` | **Written** | Canonical revision 2.0 condition-controlled loops, boolean conditions, loop-control targets, non-continuing loops, flow merging, and explicit Sec 0.1 exclusions. Implementation progress is tracked by `frontend.while-statements-v2`. |
| `control-flow/flowcontrol_switch.md` | **Written** | Canonical revision 2.0 subject and subjectless switches, ordered value/range/relational cases, explicit fallthrough, case flow, and statement-only boundaries. Implementation progress is tracked by `frontend.switch-statements-v2`. |
| `control-flow/flowcontrol_match.md` | **Written** | Canonical revision 2.0 structural and variant matching, exhaustiveness, guarded ownership commit, contextual arm-block values, union empty state, and match/LSP facts. Implementation progress is tracked by `frontend.match-v2`. |

---

# 3. Collections, arrays, and shaped values

| Rulebook | Status | Notes |
|---|---|---|
| `collections/collections.md` | **Written** | Canonical fixed-array, owning dynamic-array, slice, list, map, and set semantics. |
| `collections/shaped-types.md` | **Written** | Canonical runtime/static shaped values, affine views, layout, storage requests and transfer, broadcasting, vector/matrix algebra, and contraction semantics. Implementation progress is tracked by `frontend.shaped-types` in `implementation-status.yaml`. |
| `declarations/spread.md` | **Written** | Fixed-array expansion and struct construction integration. Frontend progress is tracked by `frontend.spread`. |
| `control-flow/flowcontrol_for.md` | **Written** | Canonical compiler-known collection, shaped-value, and explicit `Iterator[T]` iteration; implementation progress is tracked by `frontend.for-loops-v2`. |

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
| `memory/allocation.md` | **Written** | Canonical revision 2.0 allocation contexts, effects, Arena integration, failure, capacity, target policy, Semantic IR, lowering, diagnostics, and LSP. Implementation progress is tracked by the six `*.allocation` entries in `implementation-status.yaml`. |
| `memory/arena.md` | **Written** | Canonical revision 2.0 Arena ownership, ArenaDomain identity, backing/providers, allocation, Reset/Release lifecycle, validity epochs, execution dependencies, effects, capacity analysis, Semantic IR, lowering, target profiles, diagnostics, and tooling. The restored §1.1 cross-reference index names all adjacent authorities. Implementation progress is tracked by the six `*.arena` entries in `implementation-status.yaml`. |
| `memory/ownership.md` | **Written** | Canonical revision 2.0 ownership, including Correction 30 exact-Place availability rules and the 2026-09-03 recursive terminal-return forwarding correction for aggregate, union, Option, Result, and value-producing control-flow construction. Implementation progress is tracked by `frontend.ownership-v2` in `implementation-status.yaml`. |
| `memory/borrowing.md` | **Written** | Canonical revision 2.0 borrowing, including shared/mutable borrow authority, reborrowing, Place overlap, control-flow merging, match/defer interactions, and reference-origin obligations. Implementation progress is tracked by `frontend.borrowing`, `semantic-ir.borrowing`, `lowering.borrowing`, and `tooling.borrowing` in `implementation-status.yaml`. |
| `memory/references.md` | **Written** | Canonical revision 2.0 safe-reference semantics, including provenance, views, returned references, generations, relocation, hardware/ISR boundaries, Semantic IR, lowering, diagnostics, and LSP. Implementation progress is tracked by the six `*.references` entries in `implementation-status.yaml`. |
| `memory/reference_model.md` | **Written** | Canonical revision 2.0 validity and representation model for safe references, stable and weak handles, storage identity, provenance, spatial/temporal/type validity, authority, relocation, address spaces, epochs, target profiles, FFI/ISR boundaries, Semantic IR, lowering, and diagnostics. Implementation progress is tracked by the six `*.reference-model` entries in `implementation-status.yaml`. |
| `generational_references.md` | **Covered** | Generational validity is canonical in `memory/reference_model.md`; no separate rulebook is required. |
| `memory/raw_pointers.md` | **Written** | Canonical revision 2.0 unchecked raw-address semantics, operations, unsafe obligations, target/address-space rules, ownership/lifetime boundaries, Semantic IR, lowering, diagnostics, and tooling. Implementation progress is tracked by the six `*.raw-pointers` entries in `implementation-status.yaml`. |
| `memory/copy_move.md` | **Living** | Canonical copy/move semantics, including recursive terminal-return ownership forwarding without redundant inner `<-` markers while non-terminal construction retains explicit-move requirements. Implementation status is tracked by `frontend.copy-move` in `implementation-status.yaml`. |
| `memory/lifetime_analysis.md` | **Written** | Canonical revision 2.0 lifetime analysis, including value/storage/reference lifetimes, Place-sensitive invalidation, non-lexical borrows, control-flow and loop joins, returned-reference summaries, defer/capture dependencies, arena epochs, fixed-address and runtime-mapping boundaries, Semantic IR obligations, and diagnostics. Implementation progress is tracked by `frontend.lifetime-analysis`, `interprocedural.lifetime-analysis`, `semantic-ir.lifetime-analysis`, `lowering.lifetime-analysis`, `platform.lifetime-analysis`, and `tooling.lifetime-analysis` in `implementation-status.yaml`. |
| `memory/destruction.md` | **Written** | Canonical revision 2.0 deterministic destruction, exact-once cleanup responsibility, partial and conditional aggregate cleanup, custom `free`, construction-failure cleanup, unified defer/destruction ordering, and target-policy boundaries. Implementation progress is tracked by `frontend.destruction`, `semantic-ir.destruction`, `lowering.destruction`, `target-policy.destruction`, and `tooling.destruction` in `implementation-status.yaml`. |
| `memory/memory_model.md` | **Written** | Canonical revision 2.0 abstract memory machine separating values, objects, bindings, Places, storage, representations, ownership, borrows, provenance, validity, concurrency, hardware effects, and lowering obligations. Implementation progress is tracked by the six `*.memory-model` entries in `implementation-status.yaml`. |
| `declarations/static.md` | **Written** | Revision 2.1 defines module and function-local static storage plus type-associated members; immutable impl `let` is canonical and immutable impl `static let` is equivalent compatibility syntax, while mutable associated storage remains explicit. Implementation progress is tracked by `frontend.static-declarations-members`. |
| `control-flow/discard.md` | **Written** | Canonical revision 2.0 explicit and implicit discard, must-use/discardability, reinitialization, lifecycle-handle, and deterministic destruction semantics. The implemented frontend slice and remaining lowering, Place, diagnostics, and path-sensitive work are tracked by `frontend.discard-v2`. |
| `memory/storage.md` | **Written** | Canonical revision 2.0 storage origin, backing relation, reclamation authority, address stability, regions, invalidation domains, epochs, pin/protection state, memory spaces, placement, concurrency, Semantic IR, lowering, and diagnostics. Implementation progress is tracked by the six `*.storage` entries in `implementation-status.yaml`. |
| `memory/layout.md` | **Written** | Canonical revision 2.0 semantic/native/explicit layout, size, alignment, stride, padding, aggregate and union representation, completeness, validity, target plans, ABI/register integration, Semantic IR, lowering, and tooling. Implementation progress is tracked by the six `*.layout` entries in `implementation-status.yaml`. |

`storage_allocation.txt` from the old checklist is replaced by the clearer
canonical rulebook:

```text
memory/storage.md
```

Allocation policy remains in:

```text
memory/allocation.md
```

Physical storage categories and placement belong in:

```text
memory/storage.md
```

---

# 5. Errors, panic, and runtime checks

| Rulebook | Status | Notes |
|---|---|---|
| `errors/errorhandling.md` | **Written** | Canonical revision 2.0 compiler-known `error`, typed Result channels, Result projections, general Result/Option/fallible-operation `try`, partial guarded handlers, explicit fallible-setter contracts, ownership, Semantic IR, diagnostics, and LSP requirements. Implementation progress is tracked by `frontend.errorhandling-v2`. |
| `errors/panic.md` | **Written — revision 2.0** | Canonical panic domains, containment, cleanup, checked unreachable, task/thread outcomes, panic information, no-panic verification, and runtime-free support model. Sec 0.1 assertion syntax is locked to `assert condition` or `assert condition, "message"`; exact explicit-panic and build-manifest syntax remain open. |
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
| `concurrency/tasks.md` | **Written** | Canonical revision 2.0 task semantics: fallible task spawn, move-only lifecycle ownership, `TaskOutcome[T]`, cancellation, panic/execution-failure separation, observers, transferability, Semantic IR, and runtime-independent lowering. Implementation is tracked by `concurrency.tasks-v2`. |
| `concurrency/spawn.md` | **Written — sync required** | All spawn forms are fallible; process spawn is deferred. |
| `concurrency/await.md` | **Written — sync required** | Must be synchronized with task outcome and cancellation. |
| `concurrency/threads.md` | **Written — sync required** | Thread design is closed for Sec 0.1; implementation remains. |
| `concurrency/thread_local.md` | **Written — sync required** | Physical thread-local storage and task-migration restrictions. |
| `concurrency/scheduling.md` | **Written — sync required** | |
| `concurrency/blocking.md` | **Written — sync required** | |
| `concurrency/cancellation.md` | **Written — sync required** | |
| `concurrency/structured_concurrency.md` | **Written — sync required** | |
| `memory/transferability.md` | **Written** | Canonical revision 2.0 boundary-specific transferability and shareability across tasks, physical threads, processes, interrupts, and foreign callbacks, including closure/reference/capability dependencies and platform constraints. Implementation progress is tracked by the six `*.transferability` entries in `implementation-status.yaml`. |
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
| `declarations/generics.md` | **Written** | Type-generic declarations, parameters, constraints, inference, generic enums/interfaces/named types/methods, and template-level validity. Canonical concrete specialization is owned by `compiler/monomorphization.md`; compile-time value parameters still require separate normative semantics. |
| `declarations/interfaces.md` | **Written** | Interface declarations, generic constraints, and compiler-known `Iterator[T]`; implementation progress is tracked by `frontend.interfaces` and `frontend.for-loops-v2`. |
| `declarations/impl.md` | **Written** | Frontend lifecycle construction is partially implemented and tracked canonically in `implementation-status.yaml`. |
| `declarations/properties.md` | **Written** | Implementation progress is tracked by `frontend.properties`. |
| `library/core-library.md` | **Written — sync required** | Compiler-known core declarations and privileged impl access. |
| `compiler/compiler_known_members.md` | **Written** | Canonical typed registry, stable member identities, lookup, builtin and shaped member surfaces, core boundary and tooling behavior. Initial Sema/LSP registry integration is implemented. |
| `library/stdlib.md` | **Written — partially implemented** | Standard-library boundaries and target contracts, including the canonical `stdlib/hw` area and reserved `hw/spi`, `hw/i2c`, `hw/i2s`, and `hw/uart` infrastructure, plus Linux/amd64 streaming file IO, exact and complete caller-buffer reads, writes/copy, seek/flush/truncate/close, directory iteration, non-recursive path operations, path bridging, and explicit resource lifecycle diagnostics. Hardware-bus APIs, allocating complete-file APIs, and directory-list APIs remain pending. |

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
| `compiler/compiler.md` | **Written** | Canonical revision 2.0 compiler responsibilities, Target/Variant and CompilationPlan authority, source selection, compiler-known surfaces, frontend orchestration, backend boundaries, diagnostics, and tooling contracts. Implementation progress is tracked by the `compiler-core` integration family in `implementation-status.yaml`. |
| `compiler/compiler_analysis.md` | **Written** | Canonical revision 2.0 analysis-coordination and fact-ownership model. Implementation progress is tracked by the `analysis.compiler-analysis` family in `implementation-status.yaml`. |
| `compiler/compiler_pipeline.md` | **Written** | Canonical revision 2.0 phase-boundary pipeline from CompilationRequest and CompilationPlan through frontend analysis, Semantic IR, Sec MLIR, target artifacts, linking, testing, and publication. Implementation progress is tracked by the `compiler-pipeline` integration family in `implementation-status.yaml`. |
| `compiler/semantic_ir.md` | **Written** | Canonical revision 2.0 typed Semantic IR model, including ownership, allocation, effects, concurrency, collections, shaped values, panic, checks, and resolved iterator plans. Implementation progress is tracked by the `semantic-ir-v2` integration family in `implementation-status.yaml`. |
| `compiler/rules_implementations.txt` | **Living** | Legacy implementation notes being migrated into `implementation-status.yaml`. |
| `analysis/call_graph.md` | **Written** | Canonical callable reachability and execution relationships. Implementation status is tracked by `analysis.call-graph` in `implementation-status.yaml`. |
| `analysis/stack_analysis.md` | **Written** | Canonical semantic and machine stack-resource analysis; implementation status is tracked by `sema.stack-analysis` in `implementation-status.yaml`. |
| `analysis/escape_analysis.md` | **Written** | Canonical escape subjects, destinations, retention, summaries, diagnostics, and no-silent-promotion contract. Implementation status is tracked by `sema.escape-analysis`. |
| `analysis/closure_analysis.md` | **Written** | Canonical capture, callable-flow, target-set, closure-summary, and analysis-budget model. Implementation status is tracked by `sema.closure-analysis`. |
| `analysis/parameter_usage_analysis.md` | **Written** | Canonical multidimensional parameter-demand and advisory-narrowing model. Implementation status is tracked by `sema.parameter-usage-analysis`. |
| `analysis/pitfall_analysis.md` | **Written** | Canonical semantic pitfall-finding, evidence, suppression, confidence, corrective-action, budget, incremental, LSP, and FFI-contract model. Implementation status is tracked by `sema.pitfall-analysis`. |
| `analysis/effect_analysis.md` | **Written** | Canonical compile-time effect domains and propagation model, including the distinction between logical shaped operations and storage-producing shaped operations. Initial Arena event sites, synchronous `MayAllocate` propagation, cause paths, and LSP hover are implemented; the complete effect set, guarantees, contexts, indirect targets, and per-plan analysis remain. |
| `analysis/isr_analysis.md` | **Written** | Canonical cross-analysis ISR constraint verifier: resolved profiles, reusable requirement summaries, execution-context propagation, stack/race/deadlock/FFI composition, Valid/Invalid/Unproven proof states, incremental dependencies, and progressive LSP refinement. Implementation status is tracked by `sema.isr-analysis`. |
| `compiler/parser_recovery.md` | **Written** | Canonical deterministic recovery model; structured implementation remains partial. |
| `compiler/generics_lowering.md` | **Written** | Verified concrete-generic closure and representation-dependent lowering boundary. Canonical specialization identity and demand are owned by `compiler/monomorphization.md`; implementation remains partial under `compiler.generics-lowering`. |
| `compiler/monomorphization.md` | **Written** | Canonical demand-driven specialization model: `InstantiationIdentity`, dependency graph, simultaneous substitution, semantic-versus-physical realization, implementation sharing, cross-module artifacts, ABI/FFI, incremental fingerprints, termination/resource limits, diagnostics, cost policy, and determinism. Implementation is tracked by `compiler.monomorphization`. |
| `compiler/linking.md` | **Written** | Canonical CompilationPlan-specific LinkPlan, binary symbol identity, native/foreign resolution, archives, reachability, dead stripping/LTO, deterministic toolchain materialization, and artifact verification. The existing direct clang-driver build path is a legacy partial slice; canonical work is tracked by `compiler.linking`. |
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
| `mlir/sec_mlir_dialect.md` | **Written** | Current canonical dialect specification, synchronized through schema 9 / SEC-MLIR-P13. |
| `mlir/sec_mlir_lowering.md` | **Written** | Current canonical lowering specification, synchronized through lowering version 9 / SEC-MLIR-P13. |
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
| `platform/ffi.md` | **Written** | Canonical revision 2.0 foreign declarations, C ABI type families, data representations, callbacks, varargs, strings, ownership, effects, symbols, and legality. Implementation progress is tracked by `frontend.ffi-v2`. |
| `platform/fixed-address-bindings.md` | **Written** | Canonical `@address`, MMIO volatility, binding mutability, validation, overlap, and addressed-access semantics. Implementation is tracked by `frontend.fixed-address-bindings`. |
| `platform/abi.md` | **Written** | Canonical Sec, C, and system ABI families; plan-selected classification, call plans, signatures, fingerprints, MLIR staging, and separate-compilation compatibility. Implementation is tracked by `lowering.abi-model`. |
| `platform/target_profiles.md` | **Written** | Canonical Hosted, RTOS, and BareMetal profile families; capability activation, execution and safety policy, typed resource limits, derived profiles, immutable resolved identity, provenance, fingerprints, and compiler-consumer queries. Implementation is tracked by `platform.target-profiles`. |
| `platform/platform_model.md` | **Written** | Canonical Target/Variant terminology, immutable CompilationPlan resolution, typed platform submodels, capabilities, source selection, fingerprints, diagnostics, and LSP invalidation. Implementation is tracked by `compiler.platform-model`. |
| `platform/volatile.md` | **Written** | Canonical volatile physical-access semantics, mandatory `@address` region validation, explicit raw volatile operations, physical access contracts, optimizer invariants, representation eligibility, lowering, diagnostics, and tooling. Compiler-known RawPtr volatile methods, unsafe/non-void frontend validation, effect facts, and shared LSP exposure are implemented; target validation and lowering remain under `platform.volatile`. |
| `platform/hardware-register-access.md` | **Written** | Canonical logical hardware-register access, safe implicit observation, explicit `Read()`/`Write()`, shadow state, resource/endpoint identity, transaction planning, width/alignment/footprints, ordering/completion, access context, runtime mappings, faults, and verified IR/lowering. Implementation is tracked by `platform.hardware-register-access`. |
| `platform/inline_assembly.md` | **Written** | Canonical revision 1.0 inline-assembly operands, constraints, clobbers, effects, control-flow/stack boundaries, symbol dependencies, target restrictions, and lowering contract. Implementation progress is tracked by `platform.inline-assembly-v1`. |
| `platform/interrupts.md` | **Written** | Canonical interrupt identities/binding, ISR roots, priority/preemption/nesting/masking, classes and lifecycle, configuration capabilities, ISR-safe execution, concurrency/stack integration, startup/linking, diagnostics, tooling, and completion. Implementation is tracked by `platform.interrupts-v1`. |
| `analysis/isr_analysis.md` | **Written** | Compiler verification for profile-scoped interrupt safety using canonical analysis results; implementation status is tracked by `sema.isr-analysis`. |

This group is a central remaining language-closure block.

Storage, layout, ABI, FFI, volatile, registers, and interrupts must be designed
as one compatible model.

---

# 11. Modules, projects, initialization, and build

| Rulebook | Status | Notes |
|---|---|---|
| `projects/projects.txt` | **Written** | Repository manifest, targets, outputs, internal directories, and build structure; module semantics are delegated to `projects/modules.md`. |
| `projects/modules.md` | **Written** | Canonical module identity, membership, imports, cycles, visibility, resolution, surfaces, separate compilation, and incremental-tooling model. Implementation progress is tracked by `frontend.modules`. |
| `compiler/initialization.md` | **Written** | Canonical executable entry, runtime-free startup, initialization/shutdown plans, dependency ordering, startup rollback, static-destruction integration, target termination, and lowering/linking boundaries. Implementation is tracked by `compiler.program-initialization`. |
| `compiler/linking.md` | **Written** | Canonical link planning and final artifact semantics; implementation is tracked by `compiler.linking`. |

The following must eventually be defined coherently:

```text
module identity
import resolution
internal visibility
cross-module initialization coordination
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
| `tooling/formatter.md` | **Written** | Canonical formatting behavior, including string-enum initializer preservation, immutable impl `static let` normalization to `let`, and canonical assertion-message comma spacing; implementation progress belongs in `implementation-status.yaml`. |
| `tooling/testing.md` | **Written** | Canonical source-level `test`, `*_test.sec`, `sec test`, compiler-known `testing.*`, subtests, integration tests, `TestCompilationPlan`, execution-provider, structured-result, LSP and editor-integration semantics. Implementation progress is tracked by `tooling.language-testing`. |
| `compiler_diagnostics.md` | **Covered** | Compiler diagnostic policy remains canonical in `tooling/diagnostics.txt`; avoid duplication. |
| `debug_information.md` | **Planned** | Source mapping, variables, optimized code, generics, async/task frames, and targets. |
| `compiler_testing.md` | **Planned** | Compiler unit, integration, invalid, regression, lowering, and backend tests. |
| `incremental_compilation.md` | **Planned** | Dependency invalidation, generic specialization caches, and target-aware rebuilds. |
| `tooling/lsp.md` | **Living** | Canonical language-server architecture and feature rulebook, now including string-backed enum facts and one readonly type-qualified category for both associated-value spellings; implementation remains partial as detailed in governance. |

---

# 13. Canonical rulebook set: written

The following rulebooks are currently considered written.

```text
memory/allocation.md
collections/collections.md
memory/arena.md
foundations/attributes.md
concurrency/atomics.md
concurrency/await.md
concurrency/blocking.md
memory/borrowing.md
concurrency/cancellation.md
analysis/call_graph.md
concurrency/channels.md
compiler/compiler.md
compiler/compiler_analysis.md
compiler/compiler_pipeline.md
compiler/compiler_known_members.md
compiler/linking.md
concurrency/concurrency.md
concurrency/concurrency_memory_model.md
concurrency/concurrency_runtime_model.md
memory/copy_move.md
library/core-library.md
control-flow/defer.md
types/default_values.md
memory/destruction.md
tooling/diagnostics.txt
declarations/enums.md
errors/errorhandling.md
analysis/effect_analysis.md
concurrency/events.md
platform/ffi.md
platform/abi.md
control-flow/flowcontrol_for.md
control-flow/flowcontrol_if.md
control-flow/flowcontrol_match.md
control-flow/flowcontrol_switch.md
control-flow/flowcontrol_while.md
tooling/formatter.md
tooling/testing.md
declarations/functions.md
declarations/lambda-functions.md
declarations/generics.md
compiler/generics_lowering.md
compiler/monomorphization.md
declarations/impl.md
declarations/interfaces.md
analysis/isr_analysis.md
foundations/language_philosophy.md
foundations/lexical_structure.md
memory/layout.md
memory/lifetime_analysis.md
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
projects/modules.md
declarations/properties.md
memory/raw_pointers.md
memory/reference_model.md
memory/references.md
declarations/registers.md
platform/fixed-address-bindings.md
platform/volatile.md
platform/hardware-register-access.md
platform/interrupts.md
platform/inline_assembly.md
compiler/rules_implementations.txt
errors/runtime_checks.md
concurrency/scheduling.md
concurrency/select.md
compiler/semantic_ir.md
concurrency/spawn.md
declarations/spread.md
declarations/static.md
memory/storage.md
declarations/struct.md
concurrency/structured_concurrency.md
concurrency/tasks.md
concurrency/threads.md
memory/transferability.md
types/types.md
memory/unsafe.md
declarations/unions.md
types/units.md
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
platform/interrupts.md
platform/inline_assembly.md
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
compile_time_evaluation.md

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
- concrete `ABIModel` definitions and classification algorithms for each supported target ABI.

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

`foundations/attributes.md` now defines target-selection and interrupt-binding
attribute syntax. `platform/target_profiles.md` defines canonical profile
families, activation, policy, resources, resolution, provenance, and typed
compiler queries. `platform/volatile.md`, `platform/fixed-address-bindings.md`,
and `platform/hardware-register-access.md` now define the volatile, MMIO-binding,
and hardware-register transaction model. `platform/interrupts.md` defines the
canonical interrupt model, while `platform/inline_assembly.md` defines the
target-specific machine-operation boundary. These books consume shared platform
facts without redefining hardware access. Still to decide in the remaining
platform rulebooks:

- native platform views;

## Closures

Still to decide for Sec 0.1:

- immutable value capture only versus fuller capture support;
- mutable captures;
- reference captures;
- escaping environment storage;
- callable mutability;
- task/thread transfer.

The canonical rule is `declarations/lambda-functions.md`.

## Compile-time evaluation and generics

Still to decide:

- user-defined compile-time execution;
- allocation and I/O restrictions;
- compile-time panic;
- recursion and loop limits;

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
- explicit module/program initialization order beyond compile-time static dependencies;
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
