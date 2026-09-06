# Compile-Time Evaluation

- Status: Normative
- Created: 2026-09-06
- Last updated: 2026-09-06
- Document revision: 1.0
- Sec language version: 0.1
- Canonical path: `rules/compiler/compile_time_evaluation.md`
- Replaces: No canonical rulebook. This rulebook closes the previously planned compile-time-evaluation area.
- Repository baseline reviewed: `0f5027d`
- Related rulebooks: `rules/foundations/attributes.md`, `rules/declarations/static.md`, `rules/declarations/generics.md`, `rules/compiler/generics_lowering.md`, `rules/compiler/monomorphization.md`, `rules/compiler/compiler_known_members.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/compiler_pipeline.md`, `rules/projects/modules.md`, `rules/memory/layout.md`, `rules/memory/ownership.md`, `rules/memory/borrowing.md`, `rules/memory/copy_move.md`, `rules/memory/destruction.md`, `rules/memory/raw_pointers.md`, `rules/analysis/effect_analysis.md`, `rules/errors/panic.md`, `rules/errors/runtime_checks.md`, `rules/mlir/sec_mlir.md`, `rules/mlir/sec_mlir_dialect.md`, `rules/mlir/sec_mlir_lowering.md`
- Required corrections: `rules/corrections/compile-time-evaluation-cross-rulebook-correction.md`, `rules/corrections/compiler-known-fundamentals-cross-rulebook-correction.md`
- Governance note: this rulebook defines evaluation of Sec expressions into typed compile-time values. It does not redefine source visibility, generic specialization discovery, layout, ABI, source generation, reflection, optimizer constant folding, or compiler-internal data structures.

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the canonical Sec 0.1 semantics for compile-time evaluation of Sec expressions.

§ 1(2) It defines:

```text
plan-time versus semantic compile-time evaluation;
compile-time-required contexts;
ordinary user-function execution during semantic CTE;
allowed and forbidden effects;
transient compile-time storage;
ownership, borrowing, and destruction during evaluation;
compile-time values and static materialization;
dependency graphs and dependency cycles;
cross-module compile-time execution;
resource limits;
diagnostics and compiler outcomes;
caching and determinism;
the typed Semantic IR execution contract;
the relation between CTE and the MLIR -> LLVM -> binary pipeline.
```

§ 1(3) This rulebook does not define every language construct that requires a compile-time value.

§ 1(4) The rulebook owning a source construct determines whether that construct requires a plan-time or semantic compile-time value and determines the required result type, domain, and materialization rules.

§ 1(5) This rulebook defines how such an expression is evaluated after the owning rule has established that compile-time evaluation is required.

---

## § 2 Compile-time evaluation is ordinary Sec execution

§ 2(1) Semantic compile-time evaluation uses ordinary Sec language semantics in a restricted compile-time execution environment.

§ 2(2) The following semantics remain the same as ordinary Sec unless this rulebook explicitly restricts an effect or resource:

```text
arithmetic;
comparison;
boolean operations;
control flow;
function calls;
return;
Result and Option;
try;
assert;
panic;
ownership;
move;
copy;
borrowing;
destruction;
defer;
generic concrete callable semantics;
property and method resolution;
nominal type identity.
```

§ 2(3) CTE must not introduce a second set of integer, floating-point, ownership, lifetime, or error semantics.

§ 2(4) The compiler implementation must evaluate Sec values according to Sec type semantics rather than the arithmetic, pointer width, endianness, ABI, or object representation of the compiler host.

---

## § 3 CTE is not a separate source language

§ 3(1) Sec 0.1 does not introduce:

```text
const fn
comptime fn
comptime expression
comptime block
compile-time namespace
IsCompileTime()
```

as source-level language mechanisms.

§ 3(2) An ordinary Sec function may execute either at runtime or during semantic CTE depending on the context in which it is used.

§ 3(3) An ordinary function must not be able to observe merely that the compiler is currently evaluating it at compile time.

§ 3(4) The same function and concrete arguments must therefore have the same Sec semantics whether execution occurs during semantic CTE or at runtime, subject only to the fact that CTE forbids some runtime effects.

---

## § 4 Two compile-time evaluator classes

§ 4(1) Sec distinguishes:

```text
PlanTimeRequiredContext
SemanticCompileTimeRequiredContext
```

as compiler-semantic context classes.

§ 4(2) These names are normative compiler terms and are not Sec declarations.

§ 4(3) `PlanTimeRequiredContext` is a compile-time-required source position that must be resolved before the active semantic source graph is fully established.

§ 4(4) `SemanticCompileTimeRequiredContext` is a typed source position whose value must be established after sufficient semantic information exists for ordinary typed Sec execution.

§ 4(5) Plan-time evaluation and semantic CTE may share implementation infrastructure, but their available semantic universes are different.

---

## § 5 Plan-time evaluation

§ 5(1) Plan-time evaluation is intentionally restricted.

§ 5(2) It may use only values and operations explicitly permitted by the owning plan-time rule, including where applicable:

```text
literals;
typed configuration parameters;
canonical target or platform facts available at plan time;
restricted operators;
compiler-known identities explicitly available in the plan-time grammar.
```

§ 5(3) Plan-time evaluation must not require ordinary user-function calls whose name resolution or body semantics depend on the active source graph being selected by that same evaluation.

§ 5(4) Plan-time evaluation must not introduce a phase cycle.

§ 5(5) `@when` and the initial expression processing of attributes that determine the active source graph remain governed by `foundations/attributes.md`.

---

## § 6 Semantic compile-time evaluation

§ 6(1) Semantic CTE executes canonical typed Sec semantics.

§ 6(2) Semantic CTE may execute ordinary user-defined functions when:

```text
the callable is semantically valid;
all required call arguments are compile-time values;
the concrete executed path uses only CTE-permitted semantics;
generic identities are concrete where required;
the evaluation terminates within compiler resource limits.
```

§ 6(3) No `const fn` or similar modifier is required.

§ 6(4) A semantic CTE use is a property of the evaluation request and context, not a permanent classification of the source function.

---

## § 7 Required CTE versus optional early evaluation

§ 7(1) Required CTE and optimizer folding are semantically distinct.

§ 7(2) A `SemanticCompileTimeRequiredContext` requires successful semantic CTE according to this rulebook.

§ 7(3) Failure of required CTE is a compilation outcome governed by §§ 50–55.

§ 7(4) An ordinary local expression remains ordinary runtime semantics even if the compiler can evaluate it early.

§ 7(5) The compiler may optionally evaluate ordinary runtime expressions for optimization.

§ 7(6) Failure of optional early evaluation means only that the compiler does not use that optional fold, except that a true compiler invariant failure remains a compiler bug.

§ 7(7) Disabling optional constant folding must not make a valid ordinary runtime program invalid.

---

## § 8 Required contexts are owned by their language rules

§ 8(1) This rulebook does not make every statically known expression a required CTE expression.

§ 8(2) A rulebook introducing a compile-time-required source position must specify:

```text
the exact source construct;
whether it is PlanTimeRequiredContext or SemanticCompileTimeRequiredContext;
the required result type or result category;
additional legal-domain restrictions;
whether static materialization is required;
the compiler-known inputs permitted by that context.
```

§ 8(3) Vague phrases such as:

```text
must be constant
constant expression
known at compile time
```

must not substitute for an explicit required-context contract where evaluation semantics matter.

§ 8(4) Static analysis proving a value does not by itself turn the containing expression into a `SemanticCompileTimeRequiredContext`.

---

## § 9 Static initializers

§ 9(1) When `declarations/static.md` requires a module/static initializer to be established during compilation, that initializer is a `SemanticCompileTimeRequiredContext`.

§ 9(2) Example:

```sec
fn CalculateBufferSize() uint {
    return 4096
}

let BufferSize := CalculateBufferSize()
```

§ 9(3) The successful CTE result establishes the initial semantic value.

§ 9(4) Required CTE failure must not be replaced by a hidden runtime initializer call.

§ 9(5) A mutable static may have a CTE-required initializer:

```sec
let mut RetryCount: int := CalculateInitialRetryCount()
```

§ 9(6) After initialization, the resulting mutable static is ordinary runtime mutable state and is not CTE state.

---

## § 10 Local expressions

§ 10(1) Ordinary local immutable bindings do not become CTE-required merely because their initializer is statically evaluable.

§ 10(2) Example:

```sec
fn Work() void {
    let value := 10 * 20
}
```

has ordinary runtime expression semantics.

§ 10(3) The compiler may fold the expression to `200`.

§ 10(4) A nested source position inside a runtime function may nevertheless be a CTE-required context when its owning rule requires a compile-time value, for example a fixed type extent.

---

## § 11 Compile-time values are typed Sec values

§ 11(1) Successful semantic CTE produces a canonical typed Sec value.

§ 11(2) Conceptually:

```text
CompileTimeValue

fields:
    TypeIdentity
    SemanticValue
```

§ 11(3) `CompileTimeValue` in § 11(2) is a compiler-semantic structure, not a Sec declaration.

§ 11(4) `TypeIdentity` is the canonical Sec semantic type identity.

§ 11(5) `SemanticValue` is the canonical abstract value of that Sec type.

§ 11(6) Examples include:

```text
TypeIdentity = uint32
SemanticValue = 42

TypeIdentity = SortOrder[User]
SemanticValue = Ascending

TypeIdentity = Point
SemanticValue = { X = 10, Y = 20 }
```

§ 11(7) A CTE value must not be defined by host-language bytes, host pointers, host integer width, or compiler-object layout.

---

## § 12 Types are not first-class CTE values

§ 12(1) Sec 0.1 does not make Sec types themselves ordinary first-class values.

§ 12(2) A generic type parameter is a semantic type parameter, not a runtime or CTE value.

§ 12(3) CTE does not introduce a `Type` or `TypeInfo` metatype.

§ 12(4) This rule does not prohibit a future deliberately designed type-identity or type-information facility.

§ 12(5) If a future `TypeOf`-like facility is introduced, its semantic boundary must distinguish Sec semantic type identity from target representation.

§ 12(6) `compile_time_evaluation.md` does not introduce such a facility in Sec 0.1.

---

## § 13 Need-driven introspection boundary

§ 13(1) CTE does not automatically expose compiler knowledge as source-level reflection.

§ 13(2) Sec 0.1 does not gain a general structural reflection facility merely because semantic CTE can execute user code.

§ 13(3) CTE must not automatically expose iterable compiler data for:

```text
fields;
methods;
properties;
attributes;
interfaces;
enum declarations;
union declarations;
modules;
symbols;
source locations;
compiler IDs;
Semantic IR;
MLIR.
```

§ 13(4) Specific compiler-known semantic facts may be exposed by their owning rulebooks when a concrete language or library use case justifies them.

§ 13(5) Compiler knowledge is not automatically a reflection API.

§ 13(6) A future reflection or derivation system is left open for separate design.

---

## § 14 No declaration generation

§ 14(1) Semantic CTE produces Sec values, not source declarations.

§ 14(2) CTE must not create, insert, delete, or rewrite:

```text
functions;
types;
fields;
methods;
properties;
enum members;
union variants;
interfaces;
imports;
modules;
generic declarations.
```

§ 14(3) A string produced by CTE is a string value.

§ 14(4) A CTE-generated string must not be reparsed as Sec source merely because its content resembles Sec syntax.

§ 14(5) CTE must not mutate the compiler symbol table as a source-generation side effect.

---

## § 15 Compiler-known semantic facts

§ 15(1) CTE may consume a compiler-known semantic fact only when that fact is available in the current compiler phase and is explicitly exposed by its owning rule.

§ 15(2) If a compiler-known fact is source-visible as a Sec property, function, type, enum, error, or other declaration, the owning rulebook must contain the complete canonical Sec declaration or complete canonical source member shape.

§ 15(3) `compile_time_evaluation.md` does not invent source syntax for layout, type, target, or reflection queries.

§ 15(4) Compiler-internal semantic queries that are not Sec declarations must be documented as compiler services rather than represented by invented pseudo-Sec declarations.

---

## § 16 `SizeOf`

§ 16(1) `SizeOf` is owned by `compiler/compiler_known_members.md` and `memory/layout.md`.

§ 16(2) Its canonical Sec 0.1 source forms are properties:

```sec
value.SizeOf
TypeName.SizeOf
```

§ 16(3) The result type is:

```sec
uint
```

§ 16(4) `SizeOf(TypeName)` is not a canonical Sec 0.1 source form.

§ 16(5) A `SizeOf` result is determined for the active concrete `CompilationPlan`.

§ 16(6) `SizeOf` describes the owning rule's representation/layout fact; it does not change the semantic Sec type of the receiver.

§ 16(7) A type-sized value such as `int` remains semantically `int` even when `int.SizeOf` differs across targets.

---

## § 17 No implicit generic specialization by introspection

§ 17(1) Generic template validity remains governed by declared generic constraints.

§ 17(2) CTE does not grant generic code an implicit capability to inspect arbitrary concrete type identity or undeclared type capabilities in order to choose template semantics.

§ 17(3) Sec 0.1 does not introduce generic source mechanisms equivalent to:

```text
if T == int
IsCopyable[T]()
Implements[T, Interface]()
HasMethod[T](...)
```

through this rulebook.

§ 17(4) A specifically defined compiler-known property in another rulebook does not by itself establish a general generic-specialization mechanism.

---

## § 18 Const/value generics are separate

§ 18(1) The existence of CTE does not make arbitrary compile-time values legal generic arguments.

§ 18(2) Sec 0.1 type generics remain governed by `declarations/generics.md`.

§ 18(3) Value/const generic parameters require separate normative semantics and are not introduced by this rulebook.

---

## § 19 Path-sensitive CTE legality

§ 19(1) CTE legality is determined by the operations actually executed for the concrete CTE invocation.

§ 19(2) A function may contain a runtime-only operation on an unexecuted branch without making every CTE invocation of that function invalid.

§ 19(3) Example:

```sec
fn SelectValue(useDefault: bool) int {
    if useDefault {
        return 10
    }

    return ReadFromRuntimeSource()
}
```

§ 19(4) `SelectValue(true)` may be CTE-legal when `ReadFromRuntimeSource()` is not executed.

§ 19(5) `SelectValue(false)` is not CTE-legal if the executed call requires a forbidden CTE effect.

§ 19(6) CTE legality is therefore an evaluation property, not merely a declaration-wide `const` classification.

---

## § 20 CTE-permitted local computation

§ 20(1) Subject to ordinary Sec legality and resource limits, semantic CTE may execute:

```text
arithmetic;
comparisons;
boolean operations;
local immutable state;
local mutable state;
if;
switch;
match;
for;
while;
ordinary function calls;
concrete generic function calls;
aggregate construction;
Result;
Option;
try;
assert;
panic handling according to § 43;
transient allocation;
move;
copy;
borrowing;
destruction;
defer.
```

§ 20(2) Local mutation during CTE is permitted.

§ 20(3) Local mutation does not create persistent compiler-global or runtime state.

---

## § 21 Persistent mutable runtime state

§ 21(1) Semantic CTE must not read or mutate ordinary mutable runtime static state.

§ 21(2) A mutable static's initial value being known at compile time does not make later reads of that storage CTE-legal.

§ 21(3) Immutable compile-time-established values may be read when ordinary visibility and dependency rules permit it.

§ 21(4) This distinction preserves evaluation-order independence and deterministic caching.

---

## § 22 Forbidden ambient and external effects

§ 22(1) Semantic CTE must not execute ambient operations whose result or effect depends on uncontrolled compiler-host or runtime state.

§ 22(2) Forbidden executed effects include:

```text
filesystem I/O;
network I/O;
ambient environment-variable access;
ambient current time;
ambient entropy/randomness;
target memory access;
MMIO;
volatile access;
atomic operations;
memory barriers;
task/thread/channel/scheduler operations;
foreign calls;
system calls unless separately modeled as a canonical compiler-known pure semantic operation.
```

§ 22(3) An explicit deterministic pseudo-random algorithm operating entirely on compile-time values is ordinary computation and is not ambient randomness.

§ 22(4) Explicit canonical `CompilationPlan` facts are not ambient host state.

---

## § 23 `unsafe`

§ 23(1) Entering an `unsafe` source context does not itself make CTE invalid.

§ 23(2) `unsafe` also does not grant additional CTE capabilities.

§ 23(3) A CTE-executed unsafe operation must have defined CTE semantics.

§ 23(4) `unsafe` must not authorize CTE to dereference arbitrary host memory, target physical memory, MMIO, or call arbitrary foreign code.

---

## § 24 Transient allocation

§ 24(1) Semantic CTE may allocate transient evaluator-local storage.

§ 24(2) Transient CTE allocation is not runtime target allocation.

§ 24(3) It must not:

```text
consume target runtime heap capacity;
mutate target allocator state;
require target allocator startup;
create runtime allocator generations;
derive runtime storage addresses from compiler-host addresses.
```

§ 24(4) The compiler may implement transient CTE storage using an arena, object graph, interpreter heap, host allocations, or another internal strategy.

§ 24(5) The implementation strategy is not Sec semantics.

---

## § 25 Runtime allocator resources

§ 25(1) Transient evaluator allocation does not imply that CTE may use an explicit runtime allocator resource.

§ 25(2) An operation that semantically interacts with a runtime allocator or runtime allocation context is CTE-forbidden unless another canonical rule explicitly defines a compile-time semantic equivalent.

§ 25(3) The compiler must not silently substitute its own evaluator allocator for an explicit runtime allocator object.

---

## § 26 Ownership during CTE

§ 26(1) Ordinary ownership rules apply during semantic CTE.

§ 26(2) A CTE-created owning value may be:

```text
moved;
borrowed;
destroyed;
used by defer;
conditionally available;
partially moved where ordinary Sec rules permit it.
```

§ 26(3) CTE does not imply that a value is copyable.

§ 26(4) Copyability and move-only behavior follow the concrete Sec type.

§ 26(5) The evaluator must preserve ordinary exact-once destruction obligations for ordinary executed control flow.

---

## § 27 Destruction during CTE

§ 27(1) A CTE-local value reaches destruction according to ordinary Sec destruction semantics.

§ 27(2) A destructor executed during CTE must itself use CTE-permitted semantics.

§ 27(3) Compiler-internal reclamation of evaluator memory is an implementation detail and does not replace observable Sec destruction semantics.

§ 27(4) A compiler resource-limit abort is an external compiler abort of the evaluation and is not required to continue arbitrary source-level cleanup after the abort point.

§ 27(5) Compiler-owned evaluator resources must nevertheless be reclaimed safely.

---

## § 28 Borrowing and references during CTE

§ 28(1) Safe references and borrows may exist inside semantic CTE.

§ 28(2) Ordinary Sec lifetime, aliasing, mutable-borrow, and reborrow rules apply.

§ 28(3) The evaluator may represent CTE-local reference identity abstractly.

§ 28(4) Semantic correctness must not depend on the numeric host address of an evaluator object.

§ 28(5) A CTE-local reference identity must not escape into a runtime artifact unless an owning storage/materialization rule explicitly creates a distinct valid runtime reference identity.

---

## § 29 Raw pointers during CTE

§ 29(1) A raw pointer to evaluator-local storage may exist only where ordinary raw-pointer and unsafe rules permit it and where the evaluator defines deterministic CTE semantics.

§ 29(2) Numeric host pointer addresses must not become escaping CTE values.

§ 29(3) Converting a transient evaluator address to an integer must not allow that host-dependent address identity to escape materialization.

§ 29(4) Target/platform address constants are a separate semantic category from evaluator-local addresses.

§ 29(5) CTE may consume an explicit target address value as metadata when the owning platform rule permits it.

§ 29(6) CTE must not dereference target hardware memory.

---

## § 30 Compile-time evaluability and materializability are distinct

§ 30(1) A value may be successfully computed by semantic CTE without being statically materializable in the resulting program.

§ 30(2) The compiler must distinguish:

```text
CompileTimeEvaluable
CompileTimeMaterializable
```

as semantic properties/concepts.

§ 30(3) These names are compiler terms, not Sec declarations.

§ 30(4) Evaluation precedes context validation and materialization.

§ 30(5) A materialization failure must not be misreported as an evaluation failure.

---

## § 31 Materializable scalar values

§ 31(1) Subject to the owning type and storage rules, ordinary scalar values may be materialized when their concrete type has a defined static representation.

§ 31(2) This includes where valid:

```text
bool;
integer types;
floating-point types;
rune;
enum values;
named scalar values;
unit values.
```

§ 31(3) Nominal type identity must be preserved.

§ 31(4) Physical representation equality does not erase Sec type identity.

---

## § 32 Fixed aggregates

§ 32(1) Fixed aggregates may be statically materialized recursively when their concrete type and every stored component have defined static materialization.

§ 32(2) This may include:

```text
fixed arrays;
structs;
unions;
Option;
Result.
```

§ 32(3) Active union/enum variant identity and payload type identity must be preserved.

§ 32(4) Physical layout remains owned by the layout/lowering pipeline.

---

## § 33 Strings

§ 33(1) An immutable string value produced during CTE may be statically materialized where the canonical string/storage rules permit it.

§ 33(2) A CTE-generated string is semantically equivalent to the same ordinary immutable string value produced by another valid source form.

§ 33(3) Materialization must create the target/runtime static representation required by the string rules.

§ 33(4) The evaluator's transient string storage does not become runtime storage by address preservation.

---

## § 34 Dynamic owning collections

§ 34(1) Dynamic owning collections may be used transiently during semantic CTE when their executed operations are CTE-permitted.

§ 34(2) Sec 0.1 does not implicitly define static materialization for every heap-backed dynamic collection merely because the evaluator can construct it.

§ 34(3) A value such as a `list[T]`, `map[K, V]`, or another dynamic owner may therefore be:

```text
CTE-evaluable;
not statically materializable.
```

§ 34(4) A later owning collection/storage rule may define static materialization for a specific type without changing CTE evaluation semantics.

---

## § 35 Views, slices, and transient backing storage

§ 35(1) A view or slice may be used during CTE over evaluator-local backing storage.

§ 35(2) Such a view is not statically materializable unless the backing storage receives a valid runtime/static identity under another canonical rule.

§ 35(3) CTE must not preserve an evaluator-local pointer as the runtime backing address.

---

## § 36 Static materialization graph

§ 36(1) Materialization validation is recursive over the complete reachable value graph.

§ 36(2) The compiler must verify:

```text
the concrete type has defined static materialization;
all stored subvalues are materializable;
no transient evaluator address escapes;
no transient evaluator generation/liveness identity escapes;
no unsupported evaluator-local cyclic object graph escapes;
required target relocations and static references are legal.
```

§ 36(3) A cyclic value graph has no implicit CTE static-heap semantics in Sec 0.1.

§ 36(4) Another owning rule may define such a representation in the future.

---

## § 37 Static storage receives a new program identity

§ 37(1) A successfully materialized result becomes ordinary program storage/value according to the owning storage rule.

§ 37(2) The runtime/static object is not the evaluator object that computed it.

§ 37(3) Evaluator-local object identity, generation, lifetime, and host address do not survive merely because the semantic value survives.

---

## § 38 Compile-time dependency graph

§ 38(1) Required compile-time values form semantic dependency graphs.

§ 38(2) Evaluation order is determined by semantic dependencies rather than textual declaration order, subject to ordinary source visibility and name-resolution rules.

§ 38(3) CTE does not introduce an additional rule that a referenced declaration must textually appear earlier when ordinary Sec resolution already makes it visible.

§ 38(4) Independent required evaluations may be reordered or run in parallel by the compiler.

§ 38(5) Such scheduling must not be observable in Sec semantics.

---

## § 39 Compile-time value dependency cycles

§ 39(1) A required compile-time value must not directly or transitively require itself in order to establish that same value.

§ 39(2) Example:

```sec
let A := B + 1
let B := A + 1
```

forms an invalid compile-time value dependency cycle.

§ 39(3) The compiler must diagnose the semantic cycle rather than recurse indefinitely.

§ 39(4) The diagnostic should show the relevant source declarations/facts in the cycle.

---

## § 40 Function recursion is not a dependency-cycle error

§ 40(1) Ordinary recursive and mutually recursive function execution may occur during semantic CTE.

§ 40(2) A CTE call-stack cycle is not automatically an invalid compile-time dependency cycle.

§ 40(3) Recursive execution is legal when ordinary Sec semantics are valid and the concrete invocation terminates within resource limits.

§ 40(4) Non-terminating or excessively expensive recursion is governed by § 48.

---

## § 41 Type/layout/value dependency cycles

§ 41(1) Compile-time dependency cycles may involve semantic facts other than values.

§ 41(2) Example:

```text
layout of Packet
 -> PacketSize
 -> evaluation of CalculatePacketSize()
 -> layout of Packet
```

§ 41(3) The compiler must detect such cycles.

§ 41(4) CTE must not invent placeholder layouts or speculative fixed-point semantics unless the owning type/layout rule explicitly defines such behavior.

---

## § 42 Cross-module semantic CTE

§ 42(1) A source-accessible public function may be executed by semantic CTE across a module boundary.

§ 42(2) Module boundaries must not change whether otherwise identical ordinary Sec semantics are CTE-evaluable.

§ 42(3) Cross-module CTE requires compatible canonical semantic execution information for the called declaration.

§ 42(4) A binary ABI signature alone is not sufficient when compile-time execution requires the function body.

§ 42(5) Separate-compilation artifacts may therefore carry canonical CTE-executable semantic bodies or equivalent semantic execution representations.

§ 42(6) The exact serialized representation is implementation-defined.

---

## § 43 Cross-module visibility

§ 43(1) Compiler availability of semantic bodies does not change Sec source visibility.

§ 43(2) `_name` and `__name` retain their ordinary source-language visibility meanings.

§ 43(3) An importing source module may directly call a declaration during CTE only when ordinary source visibility permits that call.

§ 43(4) A public function's resolved semantic body may call private or sourcefile-only helpers from its origin scope when that call was legal in the origin source.

§ 43(5) Compiler-facing semantic artifacts may carry such helper semantics solely to execute the already-legal origin body.

§ 43(6) This does not make the helper source-visible or importable.

---

## § 44 Cross-module semantic identity

§ 44(1) Imported CTE execution uses canonical declaration identities from the origin module.

§ 44(2) The evaluator must not redo name lookup inside an imported function using the importing module's lexical scope.

§ 44(3) Imported generic concrete callables use the same canonical specialization identity as ordinary runtime uses.

§ 44(4) CTE execution context does not create a second generic specialization identity.

---

## § 45 Generic CTE

§ 45(1) Generic code executes during semantic CTE only as sufficiently concrete semantic instances.

§ 45(2) `generics_lowering.md` owns generic concretization requirements.

§ 45(3) `monomorphization.md` owns specialization demand, canonical instantiation identity, worklists, deduplication, specialization ownership, and specialization caching.

§ 45(4) CTE must not execute unresolved runtime-like generic parameters.

§ 45(5) The implementation may use a template plus a verified concrete substitution view internally when every executed operation observes fully concrete Sec semantics.

---

## § 46 `Result`, `Err`, and language-level errors

§ 46(1) A Sec `Result.Err(...)` value is ordinary program data.

§ 46(2) A CTE execution that successfully returns:

```sec
Err(ParseError.InvalidDigit)
```

has successfully evaluated a `Result` value.

§ 46(3) It is not a compiler CTE failure merely because the program-level result is `Err`.

§ 46(4) Ordinary `try` behavior remains ordinary Sec behavior during CTE.

§ 46(5) Compiler CTE failures are outside the source function's declared `Result`/error channel.

---

## § 47 `assert`, panic, and semantic operation failure

§ 47(1) `assert` executes during semantic CTE.

§ 47(2) A reached failed assertion causes compile-time evaluation failure for a required evaluation.

§ 47(3) A reached panic/fatal Sec semantic operation causes compile-time evaluation failure according to the ordinary panic semantics applicable to that path.

§ 47(4) A language-defined semantic operation failure, including where applicable division by zero, invalid shift, checked numeric failure, bounds violation, invalid conversion, or contract violation, is classified as:

```text
SemanticOperationFailed
```

in the compiler-semantic CTE failure taxonomy.

§ 47(5) `SemanticOperationFailed` is not a Sec error type.

§ 47(6) The owning language rule remains authoritative for the exact user diagnostic and semantic reason.

---

## § 48 Loops, recursion, and resource limits

§ 48(1) Ordinary loops, recursion, and mutual recursion are permitted during semantic CTE.

§ 48(2) Sec 0.1 does not require a general static proof of termination before CTE execution.

§ 48(3) The compiler must impose finite resource limits sufficient to protect the compiler process.

§ 48(4) Resource categories include at least:

```text
ExecutionSteps
CallDepth
TransientMemory
AggregateValueSize
```

§ 48(5) These are compiler-semantic resource categories, not Sec enum members.

§ 48(6) Wall-clock time is not a canonical language-level CTE budget.

§ 48(7) An implementation may use an emergency wall-clock watchdog to protect the compiler process, but that watchdog must not be exposed as deterministic Sec evaluation semantics.

---

## § 49 Resource exhaustion

§ 49(1) Resource-limit exhaustion is not proof that the Sec expression is semantically non-terminating.

§ 49(2) A smaller evaluation budget may fail where a larger permitted budget succeeds.

§ 49(3) Required-evaluation diagnostics must identify the resource category reached when practical.

§ 49(4) A failed resource-limited result must not be cached as permanent semantic impossibility under a later larger budget.

§ 49(5) CTE transient-memory exhaustion is a compiler resource outcome, not target-runtime out-of-memory behavior.

---

## § 50 Compiler-semantic request contract

§ 50(1) A semantic CTE request is conceptually described by:

```text
CompileTimeEvaluationRequest

fields:
    Expression
    ContextClass
    CompilationPlan
    ExpectedResultConstraint
    EvaluationMode
    ProvenanceRoot
```

§ 50(2) `CompileTimeEvaluationRequest` is not a Sec declaration.

§ 50(3) `Expression` is the canonical typed semantic root to execute.

§ 50(4) `ContextClass` identifies the required-context category relevant to the request.

§ 50(5) `CompilationPlan` identifies the compatible concrete target/profile/configuration facts.

§ 50(6) `ExpectedResultConstraint` identifies the owning context validation required after successful evaluation; it does not force the evaluator to misclassify a successful value with the wrong context type as evaluator failure.

§ 50(7) `EvaluationMode` is one of:

```text
Required
Optional
```

§ 50(8) `ProvenanceRoot` identifies the source/semantic request site.

---

## § 51 Canonical execution level

§ 51(1) Semantic CTE is normatively defined over canonical typed Sec semantic operations.

§ 51(2) Typed Semantic IR is the canonical compiler-level source of truth for those operations.

§ 51(3) The evaluator must not perform new source-level:

```text
name lookup;
overload resolution;
generic inference;
constraint discovery;
member lookup;
visibility resolution.
```

during execution.

§ 51(4) The implementation may translate canonical typed semantics into:

```text
CTE bytecode;
an interpreter graph;
a dedicated evaluator IR;
a JIT-capable intermediate form;
another semantically equivalent internal form.
```

§ 51(5) Such an internal form must preserve Sec semantics and the active `CompilationPlan`.

---

## § 52 CTE around the MLIR -> LLVM -> binary pipeline

§ 52(1) Sec's ordinary lowering pipeline remains conceptually:

```text
Sec semantic program
    ↓
Sec-MLIR / MLIR
    ↓
LLVM IR
    ↓
object / binary
```

§ 52(2) CTE is a compiler semantic service surrounding and participating in the semantic-to-lowering process.

§ 52(3) CTE may be invoked during semantic analysis whenever a language construct requires a compile-time value.

§ 52(4) CTE may consume canonical generic, type, layout, target, and other semantic facts when those facts are available without creating a phase cycle.

§ 52(5) Successful CTE values then feed ordinary Sec-MLIR/MLIR/LLVM lowering and artifact materialization.

§ 52(6) CTE does not replace the ordinary lowering pipeline.

§ 52(7) The implementation may reuse MLIR infrastructure to implement CTE provided that the executed semantics remain canonical Sec semantics rather than host ABI/machine semantics.

§ 52(8) Cross-compilation must not execute host-native semantics as a substitute for target-dependent Sec semantics unless equivalence has been explicitly proven for the operation.

§ 52(9) MLIR and LLVM constant folding are optional optimization mechanisms and are not the semantic fallback for required CTE.

---

## § 53 Evaluation outcome contract

§ 53(1) A semantic CTE invocation conceptually ends as:

```text
CompileTimeEvaluationOutcome

variants:
    Success
    Failure
    ResourceLimit
    Cancelled
```

§ 53(2) `CompileTimeEvaluationOutcome` is not a Sec declaration.

§ 53(3) `Success` contains:

```text
Value
Dependencies
Provenance
```

§ 53(4) `Failure` contains:

```text
Category
RootCause
Dependencies
Provenance
```

§ 53(5) `ResourceLimit` contains:

```text
LimitKind
ObservedUsageOrLimitWhenAvailable
Dependencies
Provenance
```

§ 53(6) `Cancelled` identifies compiler work cancellation and does not produce a source-language diagnostic by itself.

§ 53(7) A compiler invariant failure is not a normal `CompileTimeEvaluationOutcome` variant.

---

## § 54 Compile-time failure taxonomy

§ 54(1) The compiler-semantic CTE failure categories are:

```text
CompileTimeFailureCategory

values:
    NotEvaluable
    EvaluationFailed
    DependencyCycle
```

§ 54(2) The not-evaluable reasons are:

```text
CompileTimeNotEvaluableReason

values:
    MutableRuntimeState
    RuntimeAllocator
    FileSystem
    Network
    Environment
    Clock
    AmbientRandom
    TargetMemoryAccess
    VolatileAccess
    AtomicOperation
    MemoryBarrier
    Concurrency
    ForeignCall
    NoDefinedCompileTimeSemantics
```

§ 54(3) The evaluation-failure kinds are:

```text
CompileTimeEvaluationFailureKind

values:
    AssertionFailed
    PanicReached
    SemanticOperationFailed
```

§ 54(4) The resource-limit kinds are:

```text
CompileTimeResourceLimitKind

values:
    ExecutionSteps
    CallDepth
    TransientMemory
    AggregateValueSize
```

§ 54(5) The structures and category sets in this section are compiler-semantic contracts, not Sec declarations.

§ 54(6) Sec source code cannot name, construct, import, return, match, or catch these compiler-semantic categories.

---

## § 55 Materialization failure taxonomy

§ 55(1) Materialization occurs after successful value evaluation and owning-context validation.

§ 55(2) The materialization-failure kinds are:

```text
CompileTimeMaterializationFailureKind

values:
    TypeHasNoStaticMaterialization
    TransientAddressEscapes
    TransientGenerationEscapes
    UnsupportedCyclicValueGraph
```

§ 55(3) `CompileTimeMaterializationFailureKind` is not a Sec declaration.

§ 55(4) A materialization failure must not be reported as though the source expression could not be evaluated.

---

## § 56 Compiler implementation gaps

§ 56(1) A valid Sec construct that canonical CTE semantics allow but that the current compiler cannot yet execute is an implementation gap.

§ 56(2) It must not be diagnosed as invalid Sec merely to reflect incomplete compiler coverage.

§ 56(3) Conceptually:

```text
CompileTimeImplementationGap

fields:
    UnsupportedSemanticOperationOrCategory
    RequiredBy
    Provenance
```

§ 56(4) `CompileTimeImplementationGap` is a compiler implementation-status category, not a Sec declaration.

§ 56(5) The compiler must fail closed rather than substitute weaker runtime or host semantics for required CTE.

---

## § 57 Cancellation and compiler invariant failures

§ 57(1) Compiler or LSP cancellation is not a source-language CTE failure.

§ 57(2) An obsolete analysis request may abandon CTE without producing a program diagnostic.

§ 57(3) Cancellation must not be cached as semantic impossibility.

§ 57(4) An impossible evaluator state after successful semantic validation is a compiler invariant failure.

§ 57(5) Examples include:

```text
CompileTimeValue.TypeIdentity disagrees with its semantic payload;
a verified live CTE reference points to an already-destroyed evaluator object;
an unresolved generic parameter reaches a required concrete CTE operation;
the evaluator violates canonical ownership state.
```

§ 57(6) Such failures must not be masked as user CTE errors or implementation gaps.

---

## § 58 Dependency categories

§ 58(1) CTE dependencies must be tracked conservatively enough to prevent stale result reuse.

§ 58(2) Dependency kinds include:

```text
CompileTimeDependencyKind

values:
    DeclarationSemantics
    ImmutableCompileTimeValue
    GenericConcreteInstance
    TypeSemanticFact
    LayoutFact
    CompilationPlanFact
    ImportedSemanticArtifact
```

§ 58(3) `CompileTimeDependencyKind` is not a Sec declaration.

§ 58(4) The implementation may track a stricter or more conservative dependency set.

§ 58(5) Over-invalidation is permitted.

§ 58(6) Under-invalidation that reuses a stale semantic result is invalid compiler behavior.

---

## § 59 Provenance contract

§ 59(1) CTE provenance must be sufficient for diagnostics, tooling, and dependency explanation.

§ 59(2) Conceptually:

```text
CompileTimeProvenance

fields:
    RequiredBy
    RootExpression
    CurrentCallStack
    GenericInstantiationContext
    OriginModule
```

§ 59(3) `CompileTimeProvenance` is not a Sec declaration.

§ 59(4) `RequiredBy` identifies the language construct requiring CTE.

§ 59(5) `RootExpression` identifies the semantic/source expression evaluated.

§ 59(6) `CurrentCallStack` identifies the semantic call chain relevant to a failure.

§ 59(7) `GenericInstantiationContext` identifies concrete generic specialization context where relevant.

§ 59(8) `OriginModule` preserves cross-module origin.

§ 59(9) Diagnostics may compress repeated recursive frames while retaining the root cause and meaningful context.

---

## § 60 CTE caching

§ 60(1) Deterministic CTE results may be memoized.

§ 60(2) A reusable cache identity must account for every semantic input capable of changing the result, directly or transitively.

§ 60(3) Relevant inputs may include:

```text
canonical callable/expression identity;
concrete argument values;
semantic body fingerprints;
transitive helper fingerprints;
immutable compile-time values;
generic concrete instance identity;
type/layout facts;
CompilationPlan facts;
imported semantic artifact compatibility.
```

§ 60(4) Cache representation must not preserve host addresses as semantic values.

§ 60(5) Resource-limit failure under one budget must not become a permanent semantic negative cache under a later larger budget.

§ 60(6) Cancellation must never become a semantic negative cache.

---

## § 61 Determinism

§ 61(1) Under the same compatible `CompilationPlan`, same canonical semantic inputs, and sufficient resource budget, CTE must produce the same semantic value or same semantic failure independent of:

```text
source-file discovery order;
compiler thread scheduling;
hash-map iteration order;
which independent CTE request executes first;
compiler-host memory addresses.
```

§ 61(2) Target-dependent explicit inputs may legitimately produce different values for different `CompilationPlan`s.

§ 61(3) Ambient host filesystem, environment, time, and entropy must not become hidden CTE inputs.

---

## § 62 Context validation after evaluation

§ 62(1) Successful CTE evaluation does not by itself make the owning language construct valid.

§ 62(2) The owning rule validates the returned typed value.

§ 62(3) Examples:

```text
array extent requires an integer in its valid extent domain;
@when requires bool;
enum explicit value requires backing-compatible value;
static initializer requires assignment compatibility;
register width requires a legal width value.
```

§ 62(4) If CTE successfully returns the wrong type or a value outside the owning construct's valid domain, the diagnostic is owned by that language construct rather than classified as evaluator failure.

---

## § 63 Compile-time value verification

§ 63(1) Before a successful CTE value is consumed, the compiler must verify internal consistency.

§ 63(2) Verification includes where relevant:

```text
TypeIdentity matches SemanticValue;
ownership state is valid;
no impossible borrow/lifetime state exists;
concrete generic identity is sufficiently resolved;
semantic value components have valid Sec types;
dependencies and provenance are associated with the evaluation result.
```

§ 63(3) A violation after successful semantic validation is a compiler invariant failure.

---

## § 64 Materialization verification

§ 64(1) When the owning context requires static/artifact materialization, the compiler must separately verify materialization.

§ 64(2) Materialization verification includes:

```text
defined static representation;
recursive subvalue materializability;
no transient address escape;
no transient generation escape;
no unsupported cyclic evaluator graph;
valid static storage/relocation relations.
```

§ 64(3) The materialization verifier need not require that every later MLIR/LLVM physical detail has already been emitted.

§ 64(4) Semantic materializability and final backend encoding remain separate stages.

---

## § 65 No runtime fallback

§ 65(1) Required semantic CTE fails closed.

§ 65(2) If required evaluation cannot produce the required compile-time value, the compiler must not silently:

```text
emit a runtime initializer call;
insert a runtime generic dictionary;
allocate a dynamic result at startup;
replace a forbidden CTE effect with host execution;
reinterpret an evaluator-local pointer as a runtime pointer.
```

§ 65(3) Any future runtime-initialization mechanism must be explicitly defined by its owning rule and must not be invented as a CTE fallback.

---

## § 66 Compiler-known fundamentals

§ 66(1) Compiler-known source members are ordinary Sec semantic surface once resolved, even when the compiler provides their implementation.

§ 66(2) CTE executes the already-resolved semantics of such a member.

§ 66(3) CTE does not perform special spelling-based lookup for `ToString`, `SizeOf`, or another compiler-known member.

§ 66(4) Compiler-known fallback behavior and authoritative semantic properties are governed by `compiler/compiler_known_members.md` and the companion correction.

§ 66(5) A user-defined valid `ToString() string` replacement selected by ordinary member lookup is the callable CTE executes.

§ 66(6) Authoritative semantic properties such as canonical `SizeOf` are not user-overridable merely because they use member syntax.

---

## § 67 Compiler-known declaration governance

§ 67(1) Every compiler-known Sec declaration materially required by this rulebook must be fully specified by its owning rulebook.

§ 67(2) A complete declaration contract includes as applicable:

```text
canonical Sec spelling;
owning module;
generic parameters;
member or variant list;
field types;
parameter types;
return types;
error types;
modifiers;
source visibility;
compile-time/runtime availability;
target dependence.
```

§ 67(3) A normative reference to a compiler-known Sec declaration is incomplete unless this rulebook contains the full declaration or explicitly cross-references the owning rulebook containing it.

§ 67(4) Compiler-semantic structures that are not Sec declarations must instead be explicitly marked as compiler-only and specified structurally.

§ 67(5) `_` and `__` are source-language visibility mechanisms and must not be used merely to mark compiler-facing versus user-facing concepts.

---

## § 68 Effect-analysis boundary

§ 68(1) `effect_analysis.md` owns the canonical effect facts of Sec operations.

§ 68(2) This rulebook owns whether an actually executed effect is permitted in the CTE environment.

§ 68(3) CTE must not invent a competing effect-analysis model.

§ 68(4) An operation may be valid runtime Sec and still be forbidden when actually executed during CTE.

---

## § 69 Layout boundary

§ 69(1) `memory/layout.md` owns concrete layout facts.

§ 69(2) CTE may consume a layout fact only after its dependencies are sufficiently established and without creating a semantic cycle.

§ 69(3) CTE must not derive Sec layout from compiler-host data structures.

§ 69(4) CTE does not own field offsets, padding, union representation, ABI classification, or LLVM layout.

§ 69(5) A CTE-produced semantic value may later require target-specific physical materialization.

---

## § 70 Semantic IR boundary

§ 70(1) Semantic IR must preserve sufficient information for CTE execution:

```text
typed operations;
resolved declaration/member identities;
control flow;
ownership and borrow legality;
concrete generic identity where required;
effect identity;
source provenance.
```

§ 70(2) Imported semantic bodies used by CTE require ordinary artifact compatibility validation.

§ 70(3) CTE must not use a stale or incompatible semantic artifact as proof.

---

## § 71 Sec-MLIR, MLIR, and LLVM boundary

§ 71(1) Required semantic CTE results needed by a dependent semantic construct must be established before that construct's executable lowering requires the value.

§ 71(2) CTE itself need not require a complete executable Sec-MLIR representation for the entire program.

§ 71(3) The implementation may use MLIR infrastructure internally according to § 52.

§ 71(4) Later Sec-MLIR, MLIR, and LLVM constant folding remains optional optimization.

§ 71(5) Backend success must not be used to justify an unresolved required CTE semantic obligation.

---

## § 72 Diagnostics

§ 72(1) A required CTE diagnostic should identify:

```text
why the language context required compile-time evaluation;
the root evaluated expression;
the root semantic failure;
the relevant call chain;
generic concrete context where applicable;
origin module where applicable.
```

§ 72(2) Diagnostics should distinguish:

```text
not CTE-evaluable;
CTE execution failed;
CTE resource limit reached;
CTE dependency cycle;
CTE result not materializable;
CTE implementation gap;
compiler invariant failure.
```

§ 72(3) User-facing diagnostics need not expose the exact compiler-internal category spelling.

§ 72(4) A generic message such as `not a constant expression` is insufficient when the compiler can identify the actual forbidden effect or failure reason.

---

## § 73 LSP and tooling

§ 73(1) The compiler workspace is the source of truth for CTE values, dependencies, provenance, and diagnostics.

§ 73(2) LSP must not implement an independent Sec constant evaluator.

§ 73(3) Tooling may display a known compile-time value when the compiler has established it.

§ 73(4) Cross-module CTE diagnostics and navigation must preserve source visibility while retaining origin provenance.

§ 73(5) Cancellation of obsolete editor analysis must follow § 57.

---

## § 74 Required regression coverage

§ 74(1) Required CTE tests include:

```text
restricted plan-time expressions;
semantic literal/operator evaluation;
ordinary user-function CTE;
nested calls;
path-sensitive forbidden effects;
local mutation;
loops;
recursion;
resource limits;
transient allocation;
temporary dynamic collections;
strings;
ownership and moves;
borrows and references;
destruction and defer;
assert failure;
panic;
SemanticOperationFailed;
Result.Err as successful CTE value;
cross-module public call;
origin-private helper execution;
source visibility not leaked;
concrete generic CTE call;
compile-time value dependency cycle;
layout/value dependency cycle;
SizeOf target dependence;
determinism;
cache invalidation;
optional evaluation failure without source diagnostic;
required evaluation failure with source diagnostic;
materialization failure;
cancellation;
implementation gap;
compiler invariant failure separation.
```

§ 74(2) Tests must include cross-compilation cases proving that host pointer width/layout does not substitute for target Sec semantics.

---

## § 75 Implementation-status contract

§ 75(1) Implementation status must distinguish simple constant-expression folding from complete semantic CTE.

§ 75(2) Existing enum, array, range, or literal constant evaluators do not by themselves establish compliance with this rulebook.

§ 75(3) Implementation status should separately track:

```text
typed Semantic IR evaluator foundation;
ordinary function calls;
control flow;
loops and recursion;
generic concrete calls;
transient allocation;
ownership/borrowing/destruction;
forbidden-effect enforcement;
dependency tracking;
cross-module semantic bodies;
resource budgets;
materialization;
diagnostics;
tooling;
cache/incremental behavior.
```

§ 75(4) Valid Sec CTE semantics must not be reclassified as unsupported language merely because an implementation stage remains incomplete.

---

## § 76 Canonical governing principles

§ 76(1) The first governing principle is:

> Compile-time evaluation executes ordinary typed Sec semantics in a restricted deterministic compiler environment; it is not a second source language.

§ 76(2) The second governing principle is:

> A value being known by the compiler is not sufficient to make its source position a compile-time-required context.

§ 76(3) The third governing principle is:

> Compile-time evaluability and static materializability are distinct.

§ 76(4) The fourth governing principle is:

> Compiler knowledge is not automatically source-visible reflection or metaprogramming.

§ 76(5) The fifth governing principle is:

> Required CTE feeds the ordinary Sec-MLIR -> MLIR -> LLVM -> binary pipeline; it does not replace that pipeline and cannot fall back to runtime semantics when the language requires a compile-time value.
