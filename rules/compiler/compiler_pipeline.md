# Compiler Pipeline

- Status: Normative
- Created: 2026-09-03
- Last updated: 2026-09-03
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/compiler/compiler_pipeline.md`
- Replaces: `rules/compiler/compiler_pipeline.txt`
- Repository baseline reviewed: `998d8d1`

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the canonical compilation pipeline of the Sec compiler.

§ 1(2) It defines:

```text
pipeline boundaries;
required inputs and outputs;
CompilationPlan ownership;
workspace/project preparation;
source/module discovery;
frontend validation;
analysis coordination;
compile-time evaluation integration;
target/platform validation;
Semantic IR generation;
Sec MLIR lowering;
lower-dialect optimization;
LLVM lowering;
object generation;
linking;
test compilation mode;
inspection/emit commands;
error recovery;
implementation-capability boundaries;
stage invariants;
cache/incremental boundaries;
diagnostic provenance;
pipeline determinism.
```

§ 1(3) `compiler_analysis.md` owns the detailed architecture and coordination of compiler analyses.

§ 1(4) `semantic_ir.md` owns the canonical validated semantic representation before lowering.

§ 1(5) `compiler.md`, project/target/platform rulebooks, and `CompilationPlan` governance own target/project selection semantics.

§ 1(6) MLIR rulebooks own the Sec dialect, lowering, conversion, and MLIR optimization semantics.

§ 1(7) Linking rulebooks own symbol/linker behavior.

§ 1(8) Testing rulebooks own test discovery/selection/execution semantics.

§ 1(9) This pipeline coordinates those rulebooks; it does not redefine their language semantics.

---

## § 2 Core principles

§ 2(1) The pipeline consists of explicit semantic boundaries.

§ 2(2) Every boundary has defined input invariants and output invariants.

§ 2(3) A later phase must not repair missing semantic validation by inventing language meaning.

§ 2(4) A later phase may perform proof-preserving refinement or lowering when earlier facts are already canonical.

§ 2(5) Compilation must not enter representation lowering while required language-validity facts remain unresolved.

§ 2(6) Backend success never proves source-language validity.

§ 2(7) Frontend success does not imply that the current compiler implementation has lowering support for every otherwise valid Sec feature.

§ 2(8) The pipeline must distinguish:

```text
invalid Sec program;
valid Sec program unsupported by selected target/profile;
valid Sec program not yet supported by current compiler implementation;
internal compiler error.
```

§ 2(9) These categories must not be conflated in diagnostics or governance.

---

## § 3 Pipeline versus pass order

§ 3(1) Canonical pipeline boundaries do not require one rigid globally linear implementation pass list.

§ 3(2) Within a pipeline boundary, the compiler may use:

```text
demand-driven queries;
analysis graphs;
worklists;
fixed points;
parallel analyses;
incremental caches;
staged semantic plans;
specialization work queues.
```

§ 3(3) `compiler_analysis.md` is authoritative for analysis dependency coordination.

§ 3(4) A physical implementation may interleave operations whose semantic prerequisites are satisfied.

§ 3(5) A physical implementation must not cross a canonical boundary before that boundary's required invariants hold.

---

## § 4 Canonical top-level pipeline

§ 4(1) A full build conceptually passes through:

```text
1. Invocation and workspace preparation
2. CompilationPlan construction
3. Source-set and module-graph construction
4. Lexing and parsing
5. Declaration/symbol/name/type resolution
6. Semantic validation and coordinated analysis
7. Lowering-readiness validation
8. Semantic IR construction and verification
9. Sec MLIR emission and verification
10. Sec/standard MLIR lowering and optimization
11. LLVM dialect / LLVM IR lowering
12. Target code and object generation
13. Linking / artifact assembly
```

§ 4(2) Test compilation adds test discovery/selection and static harness planning before lowering.

§ 4(3) Inspection commands may stop after an earlier boundary.

§ 4(4) Multiple `CompilationPlan` instances may share target-independent validated work when dependency equivalence is proven.

---

## § 5 Invocation and workspace preparation

§ 5(1) A compiler invocation establishes a workspace/project snapshot before source compilation begins.

§ 5(2) The snapshot may include:

```text
project root;
manifest/project configuration;
requested logical target;
requested variants;
requested profile;
requested artifact kind;
compiler options;
feature/build options;
test selection;
source overrides;
optimization/debug settings;
toolchain settings.
```

§ 5(3) Project initialization such as `sec init` is not itself a compilation phase.

§ 5(4) `sec init` produces project configuration consumed by later invocations.

§ 5(5) Current CLI implementation details belong to governance and must not be embedded as normative pipeline behavior unless separately specified by a CLI rulebook.

---

## § 6 Compilation request

§ 6(1) Every build/check/emit/test operation creates a canonical compilation request.

§ 6(2) A request identifies at least:

```text
workspace/project snapshot;
compilation mode;
requested artifact/inspection boundary;
logical target;
profile;
variant set;
feature/build options;
diagnostic configuration;
source/test selection.
```

§ 6(3) A request may expand into multiple concrete `CompilationPlan` instances.

§ 6(4) Each plan is independently target/profile validatable.

---

## § 7 CompilationPlan

§ 7(1) `CompilationPlan` is established early enough to guide source selection and target-dependent semantics.

§ 7(2) A plan includes where relevant:

```text
logical target identity;
OS/platform;
architecture;
CPU/MCU/GPU selection;
target triple/backend identity;
profile;
variant;
pointer/int widths;
data layout;
endianness;
ABI;
address spaces;
memory spaces;
reference representation policy;
allocation/backing providers;
panic/runtime policy;
interrupt profile;
atomic capabilities;
platform package selection;
hardware knowledge;
optimization/debug policy.
```

§ 7(3) The exact internal structure is implementation-defined.

§ 7(4) The plan is the canonical source of target/profile facts.

§ 7(5) Compiler-host properties must not silently substitute for plan facts.

---

## § 8 Plan construction timing

§ 8(1) Plan construction occurs before target-dependent source selection.

§ 8(2) Some plan facts may be refined after project/platform discovery.

§ 8(3) Refinement must be monotonic with respect to the selected request and must not silently change the requested target/profile.

§ 8(4) A missing required plan fact must be diagnosed before a dependent phase proceeds.

§ 8(5) A plan change invalidates all dependent cached analysis/lowering artifacts.

---

## § 9 Multi-target builds

§ 9(1) One logical build request may produce multiple concrete plans.

§ 9(2) Each plan has independent:

```text
source filtering;
platform resolution;
layout;
ABI;
target validation;
Semantic IR target-sensitive facts;
Sec MLIR lowering;
object generation;
linking.
```

§ 9(3) Target-independent parsing/type/Sema work may be reused only when semantically equivalent across plans.

§ 9(4) Reuse must never leak one target's layout, pointer width, platform, or effect fact into another plan.

---

## § 10 Compilation mode

§ 10(1) Compilation mode is explicit.

§ 10(2) At minimum the model distinguishes:

```text
Production
Test
```

§ 10(3) Future modes may exist without changing the meaning of these modes.

§ 10(4) Mode is part of the plan/snapshot and is not a late boolean source-inclusion hack.

---

## § 11 TestCompilationPlan

§ 11(1) Test mode uses a `TestCompilationPlan` or equivalent specialization of the compilation-plan model.

§ 11(2) It retains:

```text
selected test declarations;
stable TestIdentity values;
source ranges;
test-only dependency direction;
testing context;
selected project test scope;
static harness roots;
target/profile;
execution metadata.
```

§ 11(3) Test selection occurs early enough to affect reachability and artifact composition.

§ 11(4) Later stages must not reconstruct test identity from generated function/linker names.

§ 11(5) `sec test`, compiler test views, and LSP/editor test views consume the same compiler-owned test metadata.

---

## § 12 Source-set construction

§ 12(1) The compiler constructs the source set for each plan.

§ 12(2) Source-set construction includes:

```text
project target sources;
module/package files;
imports;
standard library;
core;
selected platform sources;
target-specific sources;
generated compiler/platform sources where canonical;
test sources in Test mode.
```

§ 12(3) Source inclusion/exclusion is decided from canonical project/target rules.

§ 12(4) Lowering must never discover additional source files opportunistically.

---

## § 13 Target source filtering

§ 13(1) Target/platform source filtering occurs before semantic analysis of excluded source.

§ 13(2) A source excluded by the selected plan does not participate in declarations, conformance, effects, call graph, or lowering for that plan.

§ 13(3) Target validation directives may still be parsed/validated as required by their owning rules.

§ 13(4) Platform routing is plan-driven, not host-driven.

---

## § 14 Module graph

§ 14(1) Source loading constructs a deterministic module/import graph.

§ 14(2) It resolves:

```text
module identities;
package/source membership;
imports;
aliases;
visibility surfaces;
target/platform virtual imports;
standard-library/core imports;
test-only dependency edges.
```

§ 14(3) Import cycles are handled according to module rules.

§ 14(4) Semantic stages consume canonical module identities rather than raw path strings.

---

## § 15 File loading and source identity

§ 15(1) Every participating file receives a canonical source identity.

§ 15(2) The compiler preserves:

```text
canonical file path/URI;
byte offsets;
line/column mapping;
encoding;
module identity;
source provenance;
generated-source origin where applicable.
```

§ 15(3) All later diagnostics/debug information can map operations back to source.

§ 15(4) Generated sources remain distinguishable from user-authored sources.

---

## § 16 Lexical analysis

§ 16(1) Lexing converts source text to tokens.

§ 16(2) The lexer owns lexical validity.

§ 16(3) It handles:

```text
identifiers;
keywords;
literals;
operators;
punctuation;
comments;
annotations/directives;
source positions.
```

§ 16(4) The lexer does not perform ownership, type, target-layout, interface, or lowering decisions.

§ 16(5) Literal text retains sufficient information for canonical later interpretation.

---

## § 17 Parsing

§ 17(1) Parsing converts tokens into validated/recoverable syntax trees.

§ 17(2) Parsing handles:

```text
declarations;
statements;
expressions;
types;
attributes/directives;
interfaces/generics;
control flow;
unsafe syntax;
tests;
inline assembly syntax when defined by its rulebook.
```

§ 17(3) Parser output retains source ranges.

§ 17(4) The parser does not make semantic ownership/borrow/layout/ABI decisions.

§ 17(5) Parser recovery is governed by `parser_recovery.md`.

---

## § 18 Syntax recovery

§ 18(1) Frontend recovery may continue after syntax errors to report additional independent diagnostics.

§ 18(2) Recovered/invalid nodes are marked non-authoritative.

§ 18(3) Recovered invalid syntax must not become lowering input.

§ 18(4) Semantic facts derived from recovery cannot be used as positive code-generation proof.

---

## § 19 Structural syntax validation

§ 19(1) Structural validation may run during parsing or as a separate frontend query.

§ 19(2) It validates rules such as:

```text
modifier combinations;
declaration placement;
required components;
statement placement;
unsafe/extern syntax;
test declaration placement;
inline assembly structural syntax.
```

§ 19(3) Target-independent structural validation must not depend on machine layout.

---

## § 20 Declaration collection

§ 20(1) Declaration collection registers source declarations into canonical semantic identities.

§ 20(2) It includes where relevant:

```text
modules;
types;
functions;
methods;
properties;
interfaces;
impls;
statics;
enum members;
union variants;
register fields;
generic parameters;
tests;
extern declarations.
```

§ 20(3) Collection supports forward references where Sec permits them.

§ 20(4) Duplicate/conflict checks occur according to owning declaration/scope rules.

---

## § 21 Name resolution

§ 21(1) Name resolution binds source identifiers to canonical declarations.

§ 21(2) It resolves:

```text
local scopes;
module scopes;
imports;
aliases;
types;
functions;
members;
fields;
enum/union identities;
generic parameters;
compiler-known members;
platform symbols.
```

§ 21(3) Ambiguous/unresolved names do not reach lowering-ready validation.

§ 21(4) Visibility is enforced here or by a tightly coupled semantic query.

---

## § 22 Type resolution

§ 22(1) Type resolution produces canonical semantic types.

§ 22(2) It covers all Sec canonical type categories.

§ 22(3) Named/distinct types remain semantically distinct from carriers.

§ 22(4) Type resolution retains:

```text
ownership/copy classification;
reference form;
callable safety/ownership modes;
unit information;
enum/union semantics;
layout requirements;
interface constraints;
target-sized status;
FFI/ABI metadata;
compiler-known type identity.
```

§ 22(5) Concrete target-dependent layout need not be finalized merely to establish semantic type identity.

---

## § 23 Interface and generic resolution

§ 23(1) Interface conformance and generic constraints are resolved within frontend semantic validation.

§ 23(2) Compiler-known interfaces use the same canonical conformance engine.

§ 23(3) `Iterator[T]` conformance is therefore resolved before Semantic IR generation.

§ 23(4) Semantic IR/lowering must not discover iteration by probing source method names.

§ 23(5) Generic specialization may occur incrementally as concrete uses are discovered.

§ 23(6) Exact monomorphization scheduling is owned by the monomorphization/generics-lowering rulebooks when finalized.

---

## § 24 Compile-time evaluation is a service

§ 24(1) Compile-time evaluation is not one mandatory late linear stage.

§ 24(2) It is a semantic service used whenever a canonical frontend rule requires a compile-time value.

§ 24(3) Consumers include:

```text
constants;
enum values;
array lengths;
generic value arguments;
attributes/directives;
register widths;
unit/type contracts;
build conditions;
target conditions;
layout constraints;
capacity proofs;
hardware addresses;
inline assembly constraints where applicable.
```

§ 24(4) Target-dependent evaluation consumes `CompilationPlan`.

§ 24(5) Compile-time evaluation uses Sec semantics and must not depend on host overflow/pointer width/locale.

---

## § 25 Compile-time evaluation ordering

§ 25(1) Compile-time queries may occur during declaration, type, generic, semantic, layout, or target validation.

§ 25(2) A query executes only after its dependencies are resolved.

§ 25(3) Cyclic compile-time dependencies are rejected or handled according to compile-time evaluation rules.

§ 25(4) Lowering may consume compile-time results but does not rerun the general evaluator.

---

## § 26 Semantic validation boundary

§ 26(1) Frontend semantic validation proves all source-language rules required before Semantic IR.

§ 26(2) It includes coordinated facts from:

```text
typing;
conversion;
operators;
calls;
interfaces/generics;
control flow;
ownership;
copy/move;
borrowing;
lifetime/reference;
escape;
destruction;
initialization;
errors/try/match;
effects;
allocation/Arena;
storage/reference model;
transferability;
concurrency;
FFI/unsafe;
hardware/registers;
target/platform restrictions;
ISR constraints;
inline assembly contract validation where applicable.
```

§ 26(3) The implementation may compute these facts through the analysis graph rather than a fixed order.

---

## § 27 Analysis graph

§ 27(1) Semantic validation consumes the analysis architecture from `compiler_analysis.md`.

§ 27(2) Ownership, borrow, lifetime, call graph, effects, escape, closure, stack, race, deadlock, ISR, and other analyses may participate in fixed-point/demand-driven scheduling.

§ 27(3) The pipeline only requires that all facts needed by the next boundary are valid and current.

§ 27(4) No later lowering phase may substitute a missing required analysis.

---

## § 28 Control-flow validation

§ 28(1) Control-flow facts are established as part of frontend semantic validation.

§ 28(2) They include:

```text
reachability;
all-path return;
break/continue targets;
loop state;
match/switch exhaustiveness;
try/error edges;
panic/termination;
defer/cleanup edges;
ownership joins;
definite initialization.
```

§ 28(3) Control-flow analysis may interoperate iteratively with ownership/lifetime/effects.

§ 28(4) It is not required to be one isolated pass after ownership.

---

## § 29 Target/platform validation is progressive

§ 29(1) Target/platform validation occurs wherever plan-dependent semantics are needed.

§ 29(2) It is not restricted to one late stage.

§ 29(3) Early validation may include:

```text
source/platform selection;
target availability;
compiler-known platform symbols;
address-space support;
target-specific builtins;
feature selection.
```

§ 29(4) Later validation may include:

```text
layout;
ABI;
calling convention;
register/MMIO access;
atomics;
interrupts;
allocation providers;
inline assembly constraints;
object format;
linker capabilities.
```

§ 29(5) The selected target does not change general Sec semantics.

---

## § 30 Pre-lowering target closure

§ 30(1) Before representation lowering begins, all target-dependent semantics required by the represented program must be resolved.

§ 30(2) The compiler must know or have canonical deferred representation for:

```text
selected plan;
required layouts;
ABI contracts;
memory/address spaces;
platform resources;
target operations;
reference policy;
runtime/panic policy;
allocation providers;
inline assembly target contract;
interrupt resources.
```

§ 30(3) Unsupported selected-target operations are diagnosed before incompatible lowering.

---

## § 31 Language validity versus target validity

§ 31(1) A source program may be valid Sec yet invalid for a selected target/profile.

§ 31(2) Examples include use of unavailable:

```text
OS syscall/platform facility;
atomic width;
address space;
interrupt feature;
inline assembly constraint/instruction;
allocation provider;
ABI;
hardware register/resource.
```

§ 31(3) The diagnostic identifies the selected target/profile constraint.

§ 31(4) Such rejection must not be described as a general Sec syntax/type rule violation.

---

## § 32 Implementation capability validation

§ 32(1) The current compiler implementation may support only a subset of semantically valid Sec through a particular lowering path.

§ 32(2) This is an implementation-capability boundary, not language invalidity.

§ 32(3) When a valid source construct is not yet representable/lowerable by the selected implementation path:

```text
no invalid partial Semantic IR/MLIR may be published;
the compiler reports a compiler capability/unsupported-lowering diagnostic;
the language-validity result remains distinct;
governance records the missing implementation.
```

§ 32(4) An implementation limitation must not be disguised as a source semantic error.

§ 32(5) P1–P14 contain intentional dependency-bounded lowering boundaries and therefore require this distinction.

---

## § 33 Lowering-readiness gate

§ 33(1) A function/module is lowering-ready only when:

```text
source syntax is valid;
names/types are resolved;
required compile-time values are resolved;
required analyses are Valid;
ownership/cleanup plans are complete;
required target facts are resolved;
required generic/interface/Iterator plans are resolved;
required target operations are supported;
the selected compiler path can represent the construct through the requested output boundary.
```

§ 33(2) Advisory analysis warnings do not by themselves block lowering.

§ 33(3) Required `Unproven` facts block lowering when the active rule/profile requires positive proof.

---

## § 34 Semantic IR generation

§ 34(1) Lowering-ready validated semantics are converted into canonical Semantic IR.

§ 34(2) Semantic IR consumes immutable resolved semantic plans/facts.

§ 34(3) Semantic IR generation must not:

```text
perform name lookup;
discover interface conformance;
discover Iterator Next by method name;
invent ownership actions;
invent cleanup;
invent allocation;
invent target behavior.
```

§ 34(4) Semantic IR may normalize source syntax into explicit semantic operations/CFG.

§ 34(5) Semantic IR source mapping is preserved.

---

## § 35 Semantic IR verification

§ 35(1) Semantic IR is verified before it becomes lowering input.

§ 35(2) Verification checks the canonical invariants from `semantic_ir.md`.

§ 35(3) A verifier failure after successful frontend validation is an internal compiler defect.

§ 35(4) Unsupported-but-valid source must be rejected at the explicit implementation-capability boundary rather than encoded as malformed Semantic IR.

---

## § 36 Semantic IR transformations

§ 36(1) Semantic IR may undergo semantic canonicalization/specialization before MLIR emission.

§ 36(2) Valid transformations include those defined by `semantic_ir.md`.

§ 36(3) Examples include:

```text
cleanup normalization;
match/try CFG normalization;
iterator-loop normalization;
generic specialization;
constant folding using Sec semantics;
proven check elimination;
ownership-state normalization;
Arena planning;
reference-check elimination after proof.
```

§ 36(4) Transformations are re-verified according to compiler mode/policy.

---

## § 37 Generic specialization boundary

§ 37(1) Concrete representation-dependent lowering requires sufficient generic specialization.

§ 37(2) The pipeline does not require all generic templates to disappear before initial Semantic IR if canonical Semantic IR supports retaining them.

§ 37(3) Before a backend stage requiring concrete layout/operations, all reachable representation-dependent generic uses must have concrete specialization.

§ 37(4) Unresolved generic parameters must not leak into a backend that cannot represent them.

§ 37(5) Detailed specialization/deduplication scheduling belongs to dedicated generic-lowering rulebooks.

---

## § 38 Iterator lowering boundary

§ 38(1) `for` over `Iterator[T]` reaches Semantic IR with conformance and concrete `Next()` target already resolved.

§ 38(2) Semantic normalization may represent repeated `Next() Option[T]` calls explicitly.

§ 38(3) Static specialization/inlining may occur before or during MLIR lowering.

§ 38(4) No runtime dynamic dispatch is required solely because `Iterator[T]` is an interface.

§ 38(5) Lowering must not replace the language model with a closed nominal iterable whitelist.

---

## § 39 Sec MLIR emission

§ 39(1) Verified Semantic IR is emitted to the Sec MLIR dialect or equivalent current high-level Sec MLIR representation.

§ 39(2) Sec MLIR retains semantic distinctions that are not yet safely discharged.

§ 39(3) Emission consumes canonical Semantic IR only.

§ 39(4) It must not inspect AST/source syntax to reconstruct meaning.

§ 39(5) Emitted Sec MLIR is verified before publication/use by later passes.

---

## § 40 Sec MLIR schema/version

§ 40(1) The Sec dialect has a schema/version governed by its MLIR rulebooks and implementation packages.

§ 40(2) An emitter must produce one supported schema.

§ 40(3) A consumer/verifier rejects incompatible unsupported schemas.

§ 40(4) Package schema increments are implementation-version milestones, not new pipeline stages.

---

## § 41 SEC-MLIR packages P1–P14

§ 41(1) SEC-MLIR Packages P1–P14 are vertical implementation milestones.

§ 41(2) They record which semantic constructs have been carried through:

```text
frontend/Sema;
Semantic IR;
Sec MLIR schema;
verification;
conversion/lowering;
tests.
```

§ 41(3) They are not a normative sequence of compiler pipeline stages.

§ 41(4) Later canonical rulebook updates may require implementation synchronization beyond P14 without rewriting the historical package scope.

§ 41(5) Governance must identify when a new canonical rule makes an older implemented vertical incomplete relative to current Sec.

---

## § 42 Sec dialect lowering

§ 42(1) Sec MLIR lowering progressively converts discharged Sec operations/types into standard/core MLIR dialects.

§ 42(2) A Sec operation remains high-level while unresolved semantic/representation obligations remain.

§ 42(3) Conversion may use:

```text
arith;
cf/scf;
func;
memref;
LLVM dialect;
other suitable MLIR dialects.
```

§ 42(4) Selection of lower dialect is an implementation choice constrained by Sec semantics.

---

## § 43 MLIR verification

§ 43(1) MLIR verification is mandatory at defined dialect/schema boundaries.

§ 43(2) The real supported MLIR toolchain/verifier is authoritative for MLIR structural validity.

§ 43(3) Successful MLIR verification does not replace Sec semantic verification.

§ 43(4) An MLIR verifier failure from supposedly valid emitted Semantic IR is a compiler defect.

---

## § 44 MLIR optimization

§ 44(1) MLIR optimization may operate after required semantic facts have been materialized.

§ 44(2) Optimizations may include:

```text
canonicalization;
constant propagation;
CSE where semantically valid;
loop optimization;
vectorization;
shape-aware optimization;
bounds/check elimination after proof;
dead code elimination;
inlining;
devirtualization;
scalar replacement;
dialect conversion.
```

§ 44(3) Optimization must preserve Sec observable behavior.

§ 44(4) Optimization must not invent hidden allocation, ownership transfer, cleanup reorder, changed panic/error behavior, or stronger reference assumptions.

---

## § 45 MLIR as primary lowering architecture

§ 45(1) The canonical Sec backend architecture uses MLIR as the primary lowering/optimization framework between Semantic IR and LLVM/target code.

§ 45(2) A maintained legacy direct-LLVM path may exist temporarily for bootstrap/regression purposes.

§ 45(3) A legacy path is an implementation path, not an alternative source-language semantic authority.

§ 45(4) Every maintained backend path must preserve the same validated Semantic IR semantics.

§ 45(5) Governance tracks divergence and retirement of legacy backend paths.

---

## § 46 Legacy direct LLVM path

§ 46(1) A legacy direct LLVM emitter may consume validated frontend/Semantic facts only for explicitly maintained feature subsets.

§ 46(2) It must not define semantics not shared by the canonical frontend.

§ 46(3) Feature support in a legacy path may lag or differ in implementation completeness.

§ 46(4) The compiler must clearly identify which output/backend path lacks support.

§ 46(5) Direct LLVM support does not permit bypassing Semantic IR semantics for newly canonical compiler architecture where the maintained path requires Semantic IR.

---

## § 47 LLVM dialect lowering

§ 47(1) MLIR lowering may reach LLVM dialect after Sec-specific and higher-level semantics are discharged.

§ 47(2) LLVM dialect selection includes:

```text
LLVM-compatible types;
calling convention representation;
address spaces;
control flow;
intrinsics;
inline assembly;
object-level symbols;
target attributes.
```

§ 47(3) Language validation is complete before this boundary.

§ 47(4) LLVM dialect must not decide whether a source operation was legal.

---

## § 48 LLVM IR translation

§ 48(1) LLVM dialect may be translated to LLVM IR using the supported MLIR/LLVM toolchain.

§ 48(2) Translation preserves selected target triple/data layout and verified ABI.

§ 48(3) Sec-only semantic types/operations must not remain unless represented through a deliberate runtime/metadata contract.

§ 48(4) Translation failure from valid verified input is a compiler/toolchain defect or unsupported toolchain configuration.

---

## § 49 LLVM IR validity

§ 49(1) LLVM IR produced by Sec must be valid for the selected LLVM target/toolchain.

§ 49(2) Backend attributes such as:

```text
nonnull;
dereferenceable;
noalias;
alignment;
noundef;
lifetime;
calling-convention attributes.
```

must not be stronger than Sec proof.

§ 49(3) Sec-defined checked behavior must not become LLVM UB merely for optimization convenience.

---

## § 50 Target code generation

§ 50(1) Target code generation converts LLVM IR or another canonical target backend representation to target machine code.

§ 50(2) It owns machine-level tasks such as:

```text
instruction selection;
register allocation;
machine scheduling;
target instruction optimization;
relocations;
object sections;
CPU feature encoding.
```

§ 50(3) It uses the selected plan rather than compiler-host defaults.

§ 50(4) Cross-compilation must not require executing target binaries.

---

## § 51 Object generation

§ 51(1) Object generation produces target-specific object artifacts.

§ 51(2) It preserves:

```text
defined/exported symbols;
undefined/imported symbols;
sections;
relocations;
target ABI;
debug metadata;
unwind/runtime metadata only when required.
```

§ 51(3) Object format follows the selected target/toolchain.

§ 51(4) Object generation may be a requested terminal compiler artifact.

---

## § 52 Linking

§ 52(1) Linking is a distinct artifact-assembly boundary after object generation.

§ 52(2) Linking resolves:

```text
project objects;
Sec/core/stdlib objects;
required platform objects;
explicit native libraries;
runtime components actually required;
entrypoint;
linker script/target layout where applicable.
```

§ 52(3) Unused libraries/runtime components must not be linked merely because the compiler knows about them.

§ 52(4) Runtime-free targets must remain runtime-free when the program does not require runtime support.

§ 52(5) Linking follows the selected plan and artifact kind.

---

## § 53 Link reachability

§ 53(1) Reachability information may reduce emitted/linked code.

§ 53(2) Test mode may restrict roots to selected tests and dependencies.

§ 53(3) Compiler-known platform/runtime helpers are included only when reachable/required.

§ 53(4) Dead stripping does not change externally required export/linkage behavior.

---

## § 54 Artifact kinds

§ 54(1) The pipeline may produce:

```text
Semantic IR;
Sec MLIR;
lowered MLIR;
LLVM IR;
assembly;
object file;
executable;
static/shared library;
target-specific firmware/image;
test executable/harness;
analysis/debug artifacts.
```

§ 54(2) Exact supported artifact kinds are target/toolchain governed.

---

## § 55 Inspection boundaries

§ 55(1) Developer commands may stop at explicit pipeline boundaries.

§ 55(2) Conceptual inspection surfaces include:

```text
tokens;
AST;
resolved semantic facts;
analysis reports;
Semantic IR;
Sec MLIR;
lowered MLIR;
LLVM IR;
objects.
```

§ 55(3) Exact CLI command names/options are implementation/tooling contract and may evolve independently of the language rulebook.

§ 55(4) Inspection output must clearly state which boundary/schema/target plan it represents where ambiguity is possible.

---

## § 56 `sec check`

§ 56(1) A full check operation performs every frontend/analysis/target validation required to determine source validity for the selected plan without requiring target-code generation.

§ 56(2) It may additionally detect current compiler implementation-capability gaps when requested/configured.

§ 56(3) A check command must not require LLVM/backend tools merely to establish language validity unless the selected rule explicitly depends on backend-verifiable target facts unavailable otherwise.

---

## § 57 `sec sema`

§ 57(1) A semantic-inspection command may expose resolved Sema/analysis facts.

§ 57(2) It does not redefine the canonical check boundary.

§ 57(3) Exact coverage/output of `sec sema` belongs to CLI/tooling governance.

---

## § 58 Semantic IR emission command

§ 58(1) Emitting Semantic IR requires successful lowering-readiness for the represented subset.

§ 58(2) The emitted form is verified.

§ 58(3) Unsupported valid source beyond current Semantic IR coverage produces an implementation-capability diagnostic, not malformed output.

---

## § 59 Sec MLIR emission command

§ 59(1) A Sec-MLIR emission command emits the canonical verified Sec dialect representation for the selected schema.

§ 59(2) It must not publish temporary output before successful verification.

§ 59(3) Output records/identifies schema and selected plan where required.

---

## § 60 Generic MLIR emission command

§ 60(1) A generic `emit-mlir` surface may emit a documented MLIR boundary.

§ 60(2) The command must make clear whether the output is:

```text
Sec dialect;
partially lowered MLIR;
LLVM dialect MLIR;
another documented boundary.
```

§ 60(3) The pipeline rulebook does not freeze one ambiguous meaning for `emit-mlir`.

---

## § 61 LLVM emission command

§ 61(1) LLVM IR emission requires all relevant semantics to be lowered.

§ 61(2) It consumes selected target/layout/ABI facts.

§ 61(3) Unsupported source must be diagnosed before publishing partial invalid LLVM IR.

---

## § 62 Build

§ 62(1) `build` executes the required pipeline through the selected final artifact.

§ 62(2) Build uses the project/request `CompilationPlan`.

§ 62(3) Build does not default to compiler-host target when an explicit project target/variant has been selected.

§ 62(4) Default host selection is permitted only where project/CLI rules define it for an otherwise unspecified request.

---

## § 63 Run

§ 63(1) `run` performs the same compilation semantics as build for a runnable hosted target.

§ 63(2) Execution occurs only after successful artifact creation.

§ 63(3) Program arguments are runtime data and must not alter compilation semantics except where the CLI explicitly classifies a value as a compile-time option.

§ 63(4) Cross-target artifacts that cannot execute on the host are rejected by run/tooling capability, not by Sec language validity.

---

## § 64 Test

§ 64(1) `sec test` uses the canonical TestCompilationPlan.

§ 64(2) Test declarations are discovered by the compiler workspace.

§ 64(3) Test selection is resolved before reachability/lowering.

§ 64(4) An editor-specific runner must not implement separate Sec test discovery/execution semantics.

§ 64(5) Selected tests use the same Semantic IR/MLIR/backend pipeline as ordinary compilation.

---

## § 65 Analysis-only mode

§ 65(1) `sec analyse` or equivalent tooling may request deeper analysis without code generation.

§ 65(2) Analysis-only mode uses the same source/plan/Sema facts as build/check.

§ 65(3) Reduced-budget advisory analysis may be permitted, but no required positive proof may be fabricated.

---

## § 66 Diagnostic collection

§ 66(1) All phases emit through the shared diagnostic system.

§ 66(2) Diagnostic occurrences preserve:

```text
stable diagnostic identity where defined;
severity;
primary source range;
related ranges;
cause/provenance;
selected target/profile where relevant;
phase/boundary;
compiler capability versus program-invalid category;
notes/help.
```

§ 66(3) Later phases should suppress cascaded diagnostics whose root cause is an earlier invalid construct.

---

## § 67 Diagnostic phase ownership

§ 67(1) The earliest canonical phase owning an error reports the primary diagnostic.

§ 67(2) Examples:

```text
lexical error -> lexer;
grammar error -> parser;
name/type error -> frontend Sema;
ownership/borrow error -> owning analysis;
target capability error -> target validation;
unsupported current lowering -> implementation-capability boundary;
malformed emitted IR -> internal compiler error.
```

§ 67(3) Backend diagnostics must not misreport a frontend semantic rule.

---

## § 68 Internal compiler errors

§ 68(1) Internal compiler errors are distinct from user program errors.

§ 68(2) They include:

```text
Semantic IR verifier failure after validated construction;
Sec MLIR verifier failure from supposedly valid emitter output;
impossible ownership state after verified analysis;
invalid backend IR generated from supported verified input;
compiler invariant violation.
```

§ 68(3) An ICE report should preserve:

```text
pipeline phase;
current declaration/function;
source location;
compiler version;
CompilationPlan;
schema/backend version;
reproducible context.
```

---

## § 69 Toolchain failure

§ 69(1) External toolchain failure is distinct from source-language invalidity.

§ 69(2) Examples include unavailable/incompatible:

```text
sec-mlir-opt;
mlir-opt;
mlir-translate;
LLVM tools;
linker;
clang/target driver;
assembler;
platform image tool.
```

§ 69(3) The diagnostic identifies the tool/configuration failure.

§ 69(4) The compiler must not reinterpret tool absence as a Sec semantic error.

---

## § 70 Error recovery boundary

§ 70(1) Frontend syntax/Sema may recover for diagnostics.

§ 70(2) Recovery ends before lowering-ready Semantic IR construction.

§ 70(3) No recovered-invalid node may enter executable Semantic IR.

§ 70(4) Inspection tooling may display recovered semantic state only when clearly marked non-authoritative.

---

## § 71 Partial implementation boundary

§ 71(1) A compiler under development may deliberately stop supported source at a documented boundary.

§ 71(2) It must return a structured unsupported-implementation result/diagnostic rather than:

```text
panic accidentally;
emit malformed Semantic IR;
emit malformed MLIR;
silently miscompile;
pretend the source feature is semantically invalid.
```

§ 71(3) Package 14's intentionally deferred array/operator/ABI/ownership cases are examples of this architecture.

§ 71(4) Governance records these boundaries.

---

## § 72 No backend semantic repair

§ 72(1) MLIR/LLVM/codegen must not compensate for missing frontend semantics.

§ 72(2) Forbidden backend repair includes:

```text
guessing a type conversion;
guessing ownership transfer;
heap-promoting an invalid escaping reference;
choosing an allocator to make code compile;
inventing iterator discovery;
inventing error propagation;
assuming nullability;
changing destruction order;
adding synchronization not specified by semantics;
changing checked arithmetic to UB;
silently selecting another target.
```

---

## § 73 Source evaluation order

§ 73(1) Pipeline transformations preserve source evaluation order where observable.

§ 73(2) Sema/semantic plans make required ordering explicit before lowering.

§ 73(3) Lowering may reorder only after proof of observational equivalence.

§ 73(4) This includes ownership commit, argument evaluation, spread, match guards, cleanup, volatile/hardware, allocation, and error/panic effects.

---

## § 74 Cleanup ordering

§ 74(1) Cleanup/destruction/defer plans are finalized before lowering.

§ 74(2) Semantic IR makes cleanup edges/order explicit or canonically represented.

§ 74(3) MLIR/backend passes must not reorder observable cleanup.

§ 74(4) Panic cleanup behavior follows selected panic policy rather than backend exception defaults.

---

## § 75 Runtime requirements

§ 75(1) Runtime dependencies are derived from actually required semantics.

§ 75(2) The pipeline must not unconditionally link a general Sec runtime.

§ 75(3) Possible runtime/support requirements may include:

```text
panic endpoint;
allocation provider;
task/thread runtime;
reference/handle metadata helpers;
platform syscall wrappers;
test harness;
foreign support;
debug helpers.
```

§ 75(4) Static proof/profile may eliminate them.

§ 75(5) Runtime-free programs/targets remain possible.

---

## § 76 Generated compiler/platform code

§ 76(1) Compiler-generated helpers/wrappers participate in the same semantic pipeline or enter at a verified equivalent boundary.

§ 76(2) Generated code carries synthetic provenance.

§ 76(3) Generated helpers must satisfy effects, ownership, ISR, ABI, and target rules.

§ 76(4) Generated code is not a loophole around language/compiler invariants.

---

## § 77 Platform-generated interrupt wrappers

§ 77(1) Interrupt claim/dispatch/completion wrappers may be generated after interrupt binding resolution.

§ 77(2) They preserve canonical interrupt identities and effects.

§ 77(3) They must be validated before lowering.

§ 77(4) Platform generation must not depend on raw user-supplied numeric vectors where the target provides canonical named identities.

---

## § 78 FFI lowering boundary

§ 78(1) FFI Sema resolves:

```text
extern identity;
ABI;
ownership/retention;
nullability;
callbacks;
effects;
thread affinity;
layout requirements;
target availability.
```

before representation lowering.

§ 78(2) ABI lowering then converts canonical Sec/foreign representations.

§ 78(3) LLVM lowering must not infer foreign ownership/retention from pointer types.

---

## § 79 Inline assembly lowering boundary

§ 79(1) Inline assembly source is fully validated against its canonical rulebook before target lowering.

§ 79(2) The pipeline preserves:

```text
operand roles/types;
constraints;
clobbers;
memory effects;
volatile/ordering;
control-flow effects;
stack effects;
possible trap/abort;
target applicability;
trust provenance.
```

§ 79(3) LLVM/backend assembly emission consumes this resolved contract.

§ 79(4) Backend constraint errors from a supposedly supported verified contract are compiler/target implementation issues.

---

## § 80 Layout resolution

§ 80(1) Layout is plan-dependent and resolved before an operation requiring concrete materialized representation.

§ 80(2) Semantic types may exist before concrete layout is complete.

§ 80(3) `ResolvedLayout` or equivalent is attached/referenced before layout-sensitive Sec MLIR/lowering.

§ 80(4) Compiler-host `sizeof`/alignment must never substitute for target plan.

---

## § 81 Reference representation selection

§ 81(1) Source safe-reference semantics are validated independent of physical representation.

§ 81(2) The selected plan/profile may choose reference representation during lowering planning.

§ 81(3) Runtime generation/epoch metadata may be omitted after proof.

§ 81(4) Representation selection must not weaken source guarantees.

---

## § 82 Allocation-context resolution

§ 82(1) Allocation-context requirements are resolved before lowering allocation-capable operations.

§ 82(2) The compiler determines explicit/ambient/target/provider allocation context according to `allocation.md`.

§ 82(3) Backend lowering must not choose another allocator to repair a missing context.

§ 82(4) Spawned tasks/threads do not silently inherit a mutable parent Arena context.

---

## § 83 Arena planning

§ 83(1) Arena semantic analysis occurs before physical Arena representation planning.

§ 83(2) Physical planning may select:

```text
borrowed descriptor;
automatic/stack-backed fixed Arena;
static-backed Arena;
owned provider-backed Arena;
stable segmented growable Arena;
reserved-address-space Arena;
fully eliminated/scalarized Arena.
```

§ 83(3) Planning must preserve ArenaDomain, state/epoch, failure, address-stability, and dependency semantics.

---

## § 84 Concurrency lowering readiness

§ 84(1) Task/thread/channel/synchronization operations enter lowering only after:

```text
ownership transfer;
transferability;
lifetime;
effects;
thread affinity;
blocking/suspension;
race/synchronization;
cancellation/completion;
target/runtime requirements
```

are resolved.

§ 84(2) Lowering must not add implicit synchronization to make invalid source valid.

---

## § 85 ISR lowering readiness

§ 85(1) ISR roots enter lowering only after cross-analysis proof required by interrupt rules.

§ 85(2) Required guarantees include current canonical:

```text
noPanic;
noAlloc;
noBlock;
bounded work;
stack constraints;
race/deadlock constraints;
FFI constraints;
hardware-access constraints.
```

§ 85(3) Unsafe does not waive the gate.

---

## § 86 Determinism

§ 86(1) Equivalent source/request/CompilationPlan produces deterministic compiler semantic artifacts.

§ 86(2) Determinism applies to:

```text
source/module order;
symbol/type identities;
specialization identities;
iterator resolution;
Semantic IR;
Sec MLIR;
analysis summaries;
generated helper identities;
object/link symbol naming where specified;
diagnostic cause ordering.
```

§ 86(3) Compiler-host map iteration, thread scheduling, temporary filesystem order, or pointer values must not affect semantics.

---

## § 87 Temporary files

§ 87(1) Intermediate files are implementation artifacts.

§ 87(2) The compiler may use in-memory or temporary-file pipelines.

§ 87(3) Temporary representation must not become semantic authority.

§ 87(4) Failure must not publish a partially verified requested output.

§ 87(5) Developer keep/debug options may retain intermediates.

---

## § 88 Verification before publication

§ 88(1) Requested IR artifacts are published only after the relevant verifier succeeds.

§ 88(2) This applies to:

```text
Semantic IR;
Sec MLIR;
other verified MLIR;
LLVM IR where verification is available/required;
objects where target tool validation is required.
```

§ 88(3) Temporary invalid output must not replace an existing successful output artifact.

---

## § 89 Caching

§ 89(1) Pipeline boundaries may be cached.

§ 89(2) Cache keys include every semantic dependency required by the cached artifact.

§ 89(3) Relevant dependencies may include:

```text
source content;
imports/module graph;
compiler/rule/schema version;
CompilationPlan;
feature/build options;
generic specialization;
interface/Iterator conformance;
target/platform knowledge;
FFI contracts;
layout;
analysis summaries;
optimization/debug policy where output-relevant.
```

§ 89(4) An incompatible cache entry is rejected.

---

## § 90 Incremental compilation

§ 90(1) Incremental compilation may reuse validated work across invocations/snapshots.

§ 90(2) Reuse is based on semantic dependency invalidation, not timestamp alone.

§ 90(3) The detailed algorithm is owned by the future/current incremental-compilation rulebook.

§ 90(4) Incremental reuse must never bypass required verification.

---

## § 91 Parallel compilation

§ 91(1) Independent modules/functions/plans may be processed in parallel.

§ 91(2) Parallel execution must not change semantic results or deterministic artifacts.

§ 91(3) Shared caches/registries are thread-safe and deterministic.

§ 91(4) Diagnostics may be collected in parallel but ordered deterministically for stable presentation where required.

---

## § 92 Workspace and LSP reuse

§ 92(1) LSP and compiler commands should share canonical workspace/project/module/Sema/analysis services.

§ 92(2) LSP must not implement a separate source/module/Iterator/test model.

§ 92(3) LSP may stop before lowering and use reduced analysis budgets where safe.

§ 92(4) Complete LSP analysis for one snapshot/plan agrees with compiler check/build facts.

---

## § 93 Pipeline status versus language status

§ 93(1) A language feature may be:

```text
specified;
frontend-valid;
Semantic-IR-representable;
Sec-MLIR-representable;
lowerable;
executable on one or more targets.
```

§ 93(2) These statuses are distinct.

§ 93(3) Governance records implementation progress per vertical feature.

§ 93(4) A parser/Sema implementation alone is not full language implementation.

§ 93(5) A feature is end-to-end supported only when the requested execution/artifact boundary is implemented.

---

## § 94 Governance and P1–P14 synchronization

§ 94(1) P1–P14 implementation records remain valid history.

§ 94(2) When a later canonical rule changes/extends a feature assumed by an earlier package, governance must identify the new remaining work.

§ 94(3) Example: compiler-known `Iterator[T]` updates iteration semantics beyond older closed/builtin iteration assumptions.

§ 94(4) Such synchronization does not erase the fact that an older package was complete for its declared dependency-bounded scope.

§ 94(5) Root `implementation-status.yaml` is the canonical current implementation truth.

---

## § 95 Pipeline invariants

§ 95(1) Every pipeline implementation boundary documents required and produced invariants.

§ 95(2) Important invariants include:

```text
source set fixed before module semantic collection;
names resolved before lowering-ready Sema;
types/conformances resolved before Semantic IR;
Iterator Next target resolved before loop Semantic IR;
required ownership/borrow/lifetime facts Valid before Semantic IR;
cleanup plan complete before lowering;
no source semantic errors before Semantic IR;
Semantic IR verifies before Sec MLIR;
Sec MLIR verifies before conversion;
representation-dependent generics specialized before incompatible lowering;
layout resolved before physical aggregate/ABI lowering;
target operation supported before code generation;
no Sec-only unresolved operation before final LLVM/object boundary.
```

---

## § 96 Boundary assertions

§ 96(1) Debug/assertion compiler builds should verify boundary invariants aggressively.

§ 96(2) Release compilers may reduce repeated verification but retain required verifier gates.

§ 96(3) Serialized/cache-loaded artifacts are revalidated before trust when required.

---

## § 97 Unsupported feature containment

§ 97(1) Unsupported implementation paths must fail at the earliest boundary that can accurately classify the limitation.

§ 97(2) The compiler must avoid building partial downstream state after that failure.

§ 97(3) Other valid independent diagnostics may still be reported if safe.

§ 97(4) The failure includes the feature/category and requested output/backend boundary where practical.

---

## § 98 No silent fallback

§ 98(1) The compiler must not silently change:

```text
backend;
target;
profile;
allocation provider;
panic policy;
reference policy;
ABI;
source set;
optimization semantics;
linker/runtime
```

to make a failed plan succeed.

§ 98(2) Explicit fallback behavior is permitted only when part of the user's/project's selected policy and semantically equivalent.

---

## § 99 Optimization levels

§ 99(1) Optimization level controls optimization strategy, not language semantics.

§ 99(2) `-O0` and optimized builds have the same defined program behavior.

§ 99(3) Optimization may change:

```text
inlining;
check elimination after proof;
layout-preserving representation;
loop/vector optimization;
register/stack placement;
dead code elimination;
code size/speed tradeoffs.
```

§ 99(4) Optimization must not remove required safety behavior.

---

## § 100 Debug modes

§ 100(1) Debug-information settings control emitted diagnostic/debug metadata and permitted debug-specific hardening where profile-defined.

§ 100(2) They do not redefine source semantics.

§ 100(3) Source mappings should survive optimizations according to `debug_information.md` when that rulebook is finalized.

---

## § 101 Build options and compile-time features

§ 101(1) Build-feature options are resolved as part of the compilation request/plan.

§ 101(2) They may participate in compile-time conditions according to their owning rules.

§ 101(3) They must not act as textual preprocessing.

§ 101(4) Inactive semantic branches/source selections are handled according to compile-time selection rules while preserving required parsing/type-validation semantics where specified.

---

## § 102 Platform virtual imports

§ 102(1) Virtual platform imports resolve through the selected plan.

§ 102(2) Standard-library portable code should depend on canonical platform surfaces rather than hardcoding architecture-specific files where the platform model provides a virtual abstraction.

§ 102(3) Resolution occurs before semantic call/member analysis.

§ 102(4) Bootstrap explicit platform-path imports are implementation status, not permanent pipeline semantics.

---

## § 103 Generated specializations

§ 103(1) Generic/interface/Iterator specialization artifacts receive deterministic identities.

§ 103(2) Generated specializations participate in reachability/effects/stack/tooling like ordinary compiler-generated callables.

§ 103(3) They retain origin mapping to generic declaration and concrete arguments.

§ 103(4) Separate plans may require distinct specializations when representation/target facts differ.

---

## § 104 Reachability roots

§ 104(1) Reachability roots are established before final emission/link reachability.

§ 104(2) Roots may include:

```text
program entry;
exports;
selected tests;
task/thread entries;
ISR entries;
foreign callbacks;
compiler-required platform roots.
```

§ 104(3) Generated but unreachable helpers need not be emitted/linked.

---

## § 105 Entry points

§ 105(1) Artifact entrypoint requirements depend on artifact kind and target/project rules.

§ 105(2) Command executables may require canonical `main` semantics.

§ 105(3) Libraries need not have a command entrypoint.

§ 105(4) Test artifacts use compiler-generated static test harness roots rather than rewriting user test identities.

---

## § 106 Binary reproducibility

§ 106(1) Where reproducible-build mode is requested/supported, nondeterministic metadata not semantically required is normalized/excluded.

§ 106(2) Compiler semantic determinism is required even outside reproducible binary mode.

§ 106(3) Target toolchain nondeterminism should be isolated/documented rather than leaking into source semantic artifacts.

---

## § 107 Backend independence

§ 107(1) Frontend language semantics are backend-independent.

§ 107(2) A future non-LLVM backend may consume validated Semantic IR/another canonical lowering boundary.

§ 107(3) Such a backend must preserve the same source semantics/invariants.

§ 107(4) This rulebook's MLIR/LLVM path is the canonical current architecture, not a language requirement that Sec can never gain another verified backend.

---

## § 108 Required tests: plan and source

§ 108(1) Tests include:

```text
host default only when request permits it;
explicit target overrides host;
multi-plan builds isolate target facts;
target source filtering before Sema;
module graph deterministic;
test mode source selection;
CompilationPlan invalidation;
platform virtual import resolution.
```

---

## § 109 Required tests: frontend scheduling

§ 109(1) Tests include:

```text
compile-time evaluation used during type/generic/attribute resolution;
target facts available to target-dependent compile-time evaluation;
analysis graph order does not change result;
Iterator[T] resolved before Semantic IR;
ownership/borrow/lifetime validation before lowering;
recovered invalid AST never reaches IR.
```

---

## § 110 Required tests: implementation boundaries

§ 110(1) Tests include:

```text
valid-but-unsupported lowering gives implementation-capability diagnostic;
unsupported source emits no partial Semantic IR/MLIR;
target-unsupported feature is distinct from implementation-unsupported feature;
frontend-invalid source never reaches backend;
P14 deferred boundaries remain explicit and diagnostic-clean at frontend where appropriate.
```

---

## § 111 Required tests: Semantic IR and MLIR

§ 111(1) Tests include:

```text
Semantic IR verifies before emission;
Sec MLIR emitter consumes only Semantic IR;
Sec MLIR schema verification;
Iterator Next target preserved;
Arena state/domain/epoch preserved;
panic versus Result preserved;
no AST semantic rediscovery;
no hidden allocation/ownership transfer.
```

---

## § 112 Required tests: target lowering

§ 112(1) Tests include:

```text
32-bit and 64-bit plans;
layout/ABI per plan;
address spaces;
inline assembly constraints;
hardware resources;
interrupt roots;
atomics;
reference policy;
runtime-free target;
cross-compilation without target execution.
```

---

## § 113 Required tests: artifacts

§ 113(1) Tests include:

```text
emit Semantic IR;
emit verified Sec MLIR;
emit LLVM IR;
object-only build;
link executable;
library artifact;
test harness;
unused runtime/library omission;
deterministic symbol/order behavior.
```

---

## § 114 Required tests: tooling

§ 114(1) Tests include:

```text
check without backend tool requirement where possible;
LSP/compiler semantic parity;
same TestIdentity in LSP and sec test;
diagnostic phase ownership;
toolchain failure classification;
ICE classification;
inspection schema/plan labeling.
```

---

## § 115 Completion criteria

§ 115(1) Pipeline preparation is complete when project/workspace requests reliably produce canonical per-plan source/module/test snapshots.

§ 115(2) Frontend pipeline is complete when parsing, resolution, compile-time evaluation, coordinated analysis, target validation, and lowering-readiness gates cover every Sec 0.1 construct.

§ 115(3) Semantic IR boundary is complete when every validated Sec construct has canonical verified IR representation or an intentional explicitly governed unsupported boundary.

§ 115(4) Sec MLIR boundary is complete when every required Semantic IR construct reaches a verified Sec dialect representation.

§ 115(5) Lowering is complete when maintained target paths preserve all semantics through LLVM/object/artifact generation.

§ 115(6) Test pipeline is complete when compiler/LSP/test execution share one TestCompilationPlan and static selection model.

§ 115(7) Governance is complete when current implementation status distinguishes frontend validity, Semantic IR support, MLIR support, lowering, and target execution.

§ 115(8) The compiler pipeline must not be marked complete merely because one legacy/direct backend can execute a subset of Sec.

---

## § 116 Core summary

§ 116(1) The Sec pipeline is defined by semantic boundaries and invariants, not one rigid globally linear pass list.

§ 116(2) `CompilationPlan` is constructed early enough to drive source selection and target-dependent semantics.

§ 116(3) Compile-time evaluation is a semantic service used throughout frontend resolution rather than one late standalone stage.

§ 116(4) Target/platform validation is progressive and closes before dependent lowering.

§ 116(5) Coordinated frontend analyses must establish every required language/safety fact before Semantic IR.

§ 116(6) Semantic IR is the canonical validated language meaning.

§ 116(7) The canonical current backend architecture is Semantic IR -> Sec MLIR -> lower MLIR -> LLVM -> target objects/artifacts.

§ 116(8) A legacy direct LLVM path may exist as implementation/bootstrap compatibility but is not a competing language authority.

§ 116(9) P1–P14 are vertical implementation milestones, not pipeline stages.

§ 116(10) Language-invalid, target-invalid, implementation-unsupported, and internal-compiler-error states are distinct.

§ 116(11) `Iterator[T]` is resolved through canonical interface analysis before Semantic IR and is not rediscovered in lowering.

§ 116(12) Test mode uses an early `TestCompilationPlan`, not late test-source injection.

§ 116(13) No later phase may silently repair an invalid earlier semantic state or silently change target/backend/allocation policy to make compilation succeed.
