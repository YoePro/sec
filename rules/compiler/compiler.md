# Compiler

- Status: Normative
- Created: 2026-09-03
- Last updated: 2026-09-03
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/compiler/compiler.md`
- Replaces: `rules/compiler/compiler.txt`
- Repository baseline reviewed: `998d8d1`

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the top-level responsibilities and contracts of the Sec compiler.

§ 1(2) It defines the compiler-facing composition of:

```text
project/build requests;
Target and Variant selection;
platform resolution;
CompilationPlan construction;
source selection;
compile-time configuration;
compiler-known language/platform surfaces;
frontend/analysis orchestration;
Semantic IR;
Sec MLIR and backend orchestration;
artifact production;
cross compilation;
diagnostic/tooling consistency;
implementation-capability reporting.
```

§ 1(3) Detailed pipeline boundaries are owned by `rules/compiler/compiler_pipeline.md`.

§ 1(4) Detailed compiler-analysis coordination is owned by `rules/compiler/compiler_analysis.md`.

§ 1(5) Canonical post-Sema representation is owned by `rules/compiler/semantic_ir.md`.

§ 1(6) Target/platform resolution is owned by `rules/platform/platform_model.md`.

§ 1(7) Target-profile semantics are owned by `rules/platform/target_profiles.md`.

§ 1(8) Project, Target, Variant, build configuration, and compile-time parameter semantics are owned by the project rulebooks.

§ 1(9) Source attributes such as `@target` and `@when` are owned by `rules/foundations/attributes.md`.

§ 1(10) MLIR and lowering semantics are owned by the MLIR rulebooks.

§ 1(11) Link semantics are owned by `rules/compiler/linking.md`.

§ 1(12) Mutable implementation status does not belong in this rulebook.

§ 1(13) Repository implementation state is tracked by `implementation-status.yaml`.

---

## § 2 Compiler philosophy

§ 2(1) Sec is compiled without textual preprocessing.

§ 2(2) The compiler uses typed declarations, typed configuration, target-aware source selection, semantic analysis, and verified lowering.

§ 2(3) Target-dependent behavior is driven by a canonical `CompilationPlan`, not by ad-hoc downstream host queries.

§ 2(4) The compiler must preserve one coherent language meaning from source through lowering.

§ 2(5) A backend does not define Sec language semantics.

§ 2(6) The compiler must distinguish:

```text
source-language invalidity;
target/profile incompatibility;
current compiler implementation limitation;
external toolchain limitation;
internal compiler defect.
```

§ 2(7) These categories must not be collapsed into one generic “cannot compile” error.

---

## § 3 No textual preprocessor

§ 3(1) Sec 0.1 does not provide C-style:

```text
#define
#ifdef
#ifndef
text substitution
token concatenation
conditional lexical regions
-DNAME-style macro definitions
```

§ 3(2) All source remains lexically and syntactically parseable.

§ 3(3) Conditional source inclusion is declaration-aware and typed.

§ 3(4) Target selection uses `@target`.

§ 3(5) Program/build configuration uses `@when` and typed `config.<name>` values.

§ 3(6) Compiler optimization/debug settings are not source-selection macros.

---

## § 4 Canonical target terminology

§ 4(1) The compiler uses the platform model's canonical terminology.

§ 4(2) A `Target` is one logical buildable product.

Examples:

```text
server
worker
controller
firmware
```

§ 4(3) A Target is not an operating-system/architecture pair.

§ 4(4) A `Variant` is one concrete requested build configuration for a Target.

§ 4(5) A Variant may select or constrain facts such as:

```text
operating system;
architecture;
ABI;
CPU;
CPU features;
TuneCPU;
Device;
TargetProfile;
toolchain intent;
BuildProfile;
compile-time parameter overrides.
```

§ 4(6) One Target may have multiple Variants.

§ 4(7) Each selected Target + Variant produces one concrete verified `CompilationPlan`.

---

## § 5 Platform identity

§ 5(1) A `Platform` is the compiler-known system/execution environment resolved for one Variant.

§ 5(2) Platform identity is distinct from source-directory layout.

§ 5(3) Directory names and file presence may implement a platform but do not define platform semantics.

§ 5(4) Platform capabilities are resolved canonically before platform source routing depends on them.

§ 5(5) Downstream consumers must not infer the platform independently from path names.

---

## § 6 BuildProfile versus TargetProfile

§ 6(1) `BuildProfile` controls build behavior such as:

```text
optimization;
debug information;
stripping;
LTO;
similar compiler/output settings.
```

§ 6(2) `TargetProfile` controls semantic platform capabilities and constraints.

§ 6(3) TargetProfile may constrain:

```text
threads;
interrupts;
TLS;
allocation;
panic policy;
runtime availability;
MMIO;
resource bounds;
memory/reference policies;
other platform capabilities.
```

§ 6(4) BuildProfile and TargetProfile are distinct even where project syntax uses a shorter contextual name such as `profile`.

§ 6(5) Changing BuildProfile must not silently change source-language meaning or platform capability truth.

---

## § 7 CompilationPlan

§ 7(1) `CompilationPlan` is the authoritative immutable resolved compiler contract for one Target and one Variant.

Conceptually:

```text
Target + Variant + effective configuration
                    |
                    v
              PlatformResolver
                    |
                    v
             CompilationPlan
```

§ 7(2) A valid CompilationPlan contains or references concrete resolved target facts.

§ 7(3) Later compiler stages consume the frozen plan rather than re-resolving platform configuration independently.

§ 7(4) A conceptual plan may include:

```text
Project
Target
Variant
BuildProfile
PlatformIdentity
TargetProfile
ArchitectureModel
DeviceModel?
MemoryEnvironment
RuntimeEnvironment
ExecutionEnvironment
ABIModel
LinkEnvironment
SourceSelection
CompilerTargetSupport
CompilerOptions
CompileTimeParameters
```

§ 7(5) Exact internal struct layout is implementation-defined.

§ 7(6) Every required fact must be resolved before the plan is frozen.

---

## § 8 CompilationPlan verification

§ 8(1) A plan is valid only after canonical verification.

§ 8(2) Verification includes where relevant:

```text
canonical identity resolution;
Target/Variant compatibility;
architecture/CPU compatibility;
Device constraints;
ABI compatibility;
TargetProfile consistency;
memory/runtime/execution capability consistency;
source-selection validity;
link-environment consistency;
compiler target support;
toolchain requirements;
compile-time parameter validation.
```

§ 8(3) Unknown required platform facts must not be guessed.

§ 8(4) A malformed compiler-builtin registry is an internal compiler defect.

§ 8(5) An invalid project/platform configuration is a user-visible configuration error.

---

## § 9 Immutable plan consumption

§ 9(1) After plan freeze, Sema, analysis, layout, lowering, linking, and tooling consume plan facts read-only.

§ 9(2) A downstream stage must not mutate effective platform semantics.

§ 9(3) If an upstream configuration change requires a different result, a new plan snapshot is resolved.

§ 9(4) Plan-dependent cached results are invalidated when any relevant plan fact changes.

§ 9(5) Target-independent facts may be reused only when their dependency set proves independence from changed plan facts.

---

## § 10 Multi-Variant compilation

§ 10(1) One Target may be compiled for several Variants in one build request.

Example:

```text
Target: server

Variants:
    linux-amd64
    linux-arm64
    linux-armv7
    windows-amd64
```

§ 10(2) This produces four independent concrete CompilationPlans.

§ 10(3) A single CompilationPlan must not represent several operating systems or architectures simultaneously.

§ 10(4) Plan-dependent diagnostics retain Target and Variant identity.

§ 10(5) Failure of one Variant does not make another Variant's semantic result invalid.

§ 10(6) Build orchestration may report the whole multi-Variant command as failed if one requested Variant fails.

---

## § 11 Host and target separation

§ 11(1) Compiler host and program target are strictly separate environments.

§ 11(2) Target facts such as:

```text
pointer width;
endianness;
ABI;
object format;
CPU capabilities;
memory topology;
platform sources;
interrupt model;
address spaces;
runtime availability;
```

must come from the active CompilationPlan.

§ 11(3) They must never be silently taken from the host.

§ 11(4) Cross compilation does not require executing the target program on the host.

§ 11(5) Host-dependent discovery is permitted only as an explicit resolver input.

---

## § 12 Explicit native discovery

§ 12(1) No host CPU is implicitly used as the target CPU.

§ 12(2) If a supported command/project setting explicitly requests a native CPU, host discovery occurs as one resolver step.

§ 12(3) The discovered CPU/features are normalized to canonical target facts.

§ 12(4) The result is frozen into the CompilationPlan.

§ 12(5) Later stages do not query host CPU state again.

§ 12(6) `TuneCPU` may influence optimization but must not broaden the permitted CPU capability baseline.

---

## § 13 Source selection overview

§ 13(1) Source selection is part of CompilationPlan resolution.

§ 13(2) It selects the source declarations/files applicable to one plan.

§ 13(3) Source selection may depend on:

```text
project Target;
Variant;
resolved platform identity;
@target;
@when;
project compile-time parameters;
module/project rules;
test compilation mode.
```

§ 13(4) Source selection does not use textual macro substitution.

§ 13(5) Excluded source remains syntactically parseable and tool-visible according to attribute rules.

---

## § 14 `@target`

§ 14(1) `@target` is the canonical Sec 0.1 source-selection attribute for target identity.

§ 14(2) It may apply to a complete file or the next top-level statement according to attribute-placement rules.

Example:

```sec
@target(os: "linux", arch: "amd64")
module platform.linux.amd64
```

§ 14(3) Statement-level example:

```sec
@target(device: "controller-a")
fn InitializeDevice() void {
}
```

§ 14(4) Initial canonical selector names are:

```text
os
arch
cpu
device
board
```

§ 14(5) Omitted selectors impose no restriction.

§ 14(6) All supplied selectors must match.

§ 14(7) Exact selector semantics and future selector extensions belong to `attributes.md`.

---

## § 15 Legacy `#target`

§ 15(1) `#target(...)` is not the canonical Sec 0.1 target-selection syntax.

§ 15(2) The canonical form is `@target(...)`.

§ 15(3) A compiler may temporarily accept the old `#target` compatibility form during migration.

§ 15(4) Compatibility acceptance is implementation governance, not a second normative language model.

§ 15(5) New rulebooks, examples, formatter output, and generated source should use `@target`.

---

## § 16 `@target` and module identity

§ 16(1) Target selection must not silently create unrelated public APIs under one logical declaration identity.

§ 16(2) Mutually exclusive target variants of one declaration may coexist when canonical public-shape compatibility rules are satisfied.

§ 16(3) If multiple variants with the same logical identity are simultaneously active for one plan, compilation fails.

§ 16(4) A module's logical identity remains stable across Variants.

§ 16(5) `@target` does not define a new module merely because implementation source differs.

---

## § 17 Platform source routing

§ 17(1) Platform source routing uses canonical plan/platform identities.

§ 17(2) The compiler may map a resolved platform to implementation source packages/directories.

§ 17(3) The mapping is deterministic.

§ 17(4) Source directory depth/presence must not itself establish target semantics.

§ 17(5) Portable standard-library code should depend on canonical platform surfaces rather than hardcoded architecture-specific implementation files where such surfaces exist.

§ 17(6) Temporary explicit platform-path imports may exist as bootstrap implementation status.

---

## § 18 Virtual platform modules

§ 18(1) The compiler/platform model may provide canonical virtual platform import surfaces.

Conceptually:

```sec
import "platform"
```

§ 18(2) The import resolves to the selected platform implementation for the active plan.

§ 18(3) Virtual platform imports keep portable library code independent from architecture-specific package paths.

§ 18(4) Exact package/module resolution semantics belong to the project/module/platform rulebooks.

§ 18(5) A bootstrap resolver is not the normative permanent platform model.

---

## § 19 `@when`

§ 19(1) `@when` conditionally includes the next top-level statement according to typed compile-time program configuration.

Example:

```sec
@when(config.telemetry)
fn SendTelemetry() Result[void, TelemetryError] {
    // ...
}
```

§ 19(2) `@when` does not create runtime control flow.

§ 19(3) The declaration participates in the active compilation plan or it does not.

§ 19(4) The condition language is the restricted compile-time language defined by `attributes.md`.

§ 19(5) Target identity belongs in `@target`, not `@when`, in Sec 0.1.

---

## § 20 Compile-time parameters

§ 20(1) Compile-time parameters originate from project build configuration.

§ 20(2) They are:

```text
typed;
validated;
part of CompilationPlan identity;
part of build-cache identity;
not text macros;
not ordinary compiler options;
not runtime configuration.
```

§ 20(3) Canonical boolean access is:

```sec
config.telemetry
```

§ 20(4) Parameter overrides retain the declared/configured type.

§ 20(5) Invalid/unknown parameter names or values are compile-time configuration errors.

---

## § 21 Legacy `#if`

§ 21(1) The old compiler-specific `#if` conditional form is not the canonical Sec 0.1 source-selection model.

§ 21(2) Canonical declaration selection uses:

```text
@target for target identity;
@when for typed program configuration.
```

§ 21(3) A temporary parser/bootstrap compatibility form may be implementation-specific during migration.

§ 21(4) New normative source examples must not use `#if` for target/configuration selection.

§ 21(5) Compile-time ordinary language evaluation, where separately defined, is distinct from declaration selection.

---

## § 22 Legacy compiler target constants

§ 22(1) `compiler.target_os` and `compiler.target_arch` are not the canonical Sec 0.1 source-selection API.

§ 22(2) Target source selection uses `@target`.

§ 22(3) Program configuration uses `config.<name>` through `@when`.

§ 22(4) Target facts remain available internally through CompilationPlan and compiler tooling.

§ 22(5) A future explicit read-only target-info source API requires its own canonical design and is not implied by legacy bootstrap constants.

---

## § 23 Compiler options

§ 23(1) Compiler options configure compilation behavior.

§ 23(2) Examples may include:

```text
optimization;
debug information;
warnings;
LTO;
stripping;
inspection output;
toolchain location;
build orchestration.
```

§ 23(3) Compiler options do not define program source shape through `@when`.

§ 23(4) Build features that intentionally affect source inclusion use typed compile-time parameters.

§ 23(5) Compiler options belong to BuildProfile/compilation request rather than ordinary runtime program state.

---

## § 24 Configuration precedence

§ 24(1) Effective configuration is resolved before plan freeze.

§ 24(2) Precedence is owned by project/platform rulebooks.

§ 24(3) A more specific override must preserve the type and legality of the value being overridden.

§ 24(4) CLI overrides affect only the current invocation unless a project command explicitly persists them.

§ 24(5) Downstream compiler phases see only the resolved effective configuration, not unresolved precedence layers.

---

## § 25 Compiler-known entities

§ 25(1) The compiler may provide compiler-known language/platform entities.

§ 25(2) Compiler-known entities may include:

```text
builtin types;
compiler-known members;
compiler-known interfaces such as Iterator[T];
intrinsics;
platform identities;
target resources;
special lifecycle operations;
semantic helper identities.
```

§ 25(3) Compiler-known privilege is closed and compiler-versioned.

§ 25(4) User code cannot create arbitrary compiler-known entities.

§ 25(5) Core/library implementations may realize compiler-known semantics only where the canonical rulebooks permit it.

---

## § 26 `Iterator[T]`

§ 26(1) `Iterator[T]` is a compiler-known generic interface.

§ 26(2) It participates in the ordinary canonical interface/conformance model.

§ 26(3) `for` iteration resolves concrete conformance and `Next() Option[T]` statically when the concrete iterator is known.

§ 26(4) The compiler must not use a method-name convention as the language definition of iteration.

§ 26(5) The compiler must not use a closed nominal whitelist as the user-visible definition of iterable types.

§ 26(6) Compiler-known collections/ranges/strings may receive specialized lowering after equivalent iterator semantics are established.

---

## § 27 Compiler architecture overview

§ 27(1) The canonical current compiler architecture is:

```text
Sec source
    -> lex/parse
    -> name/type/interface/generic resolution
    -> semantic validation and coordinated analysis
    -> verified Semantic IR
    -> verified Sec MLIR
    -> lower MLIR dialects
    -> LLVM dialect / LLVM IR
    -> target object
    -> link/final artifact
```

§ 27(2) `compiler_pipeline.md` owns exact phase-boundary semantics.

§ 27(3) Compile-time evaluation is a semantic service used by relevant frontend stages rather than one mandatory late pass.

§ 27(4) Platform/target validation is progressive and plan-driven.

---

## § 28 Semantic analysis

§ 28(1) Semantic analysis establishes language meaning and validity before lowering.

§ 28(2) It consumes the canonical analysis architecture.

§ 28(3) Relevant fact domains include:

```text
types;
interfaces/generics;
control flow;
ownership/copy/move;
borrowing;
lifetime/reference;
escape;
destruction;
effects;
allocation/Arena;
storage;
transferability;
concurrency;
FFI/unsafe;
target/platform;
hardware/interrupts.
```

§ 28(4) No backend phase may invent a missing source semantic decision.

---

## § 29 Semantic IR

§ 29(1) Semantic IR is the canonical typed representation of validated Sec meaning before lowering.

§ 29(2) It is independent of source spelling and LLVM representation.

§ 29(3) The compiler emits Semantic IR only after all required source semantic facts are valid/resolved.

§ 29(4) Semantic IR is verified before Sec MLIR lowering.

§ 29(5) Unsupported current implementation coverage is reported explicitly rather than represented as malformed IR.

---

## § 30 Sec MLIR

§ 30(1) Sec MLIR is the canonical high-level lowering representation for the current compiler architecture.

§ 30(2) It preserves Sec-specific semantic distinctions until they can be discharged safely.

§ 30(3) Versioned Sec MLIR schemas may evolve with implementation packages.

§ 30(4) Sec MLIR is verified by the maintained MLIR toolchain/verifier.

§ 30(5) MLIR verification complements but does not replace Sec semantic verification.

---

## § 31 SEC-MLIR implementation packages

§ 31(1) P1–P14 are vertical implementation milestones.

§ 31(2) They record implementation progress across:

```text
frontend/Sema;
Semantic IR;
Sec MLIR schema;
verification;
conversion/lowering;
tests.
```

§ 31(3) They are not alternative language specifications.

§ 31(4) They are not compiler pipeline stages.

§ 31(5) Later canonical rulebook changes may add synchronization work beyond a historically completed package.

§ 31(6) Governance records current completion relative to today's canonical Sec.

---

## § 32 Legacy direct LLVM

§ 32(1) A direct AST/Sema-to-LLVM path may temporarily remain for bootstrap/regression compatibility.

§ 32(2) It is a legacy implementation path.

§ 32(3) It is not the canonical architecture for new Sec semantics.

§ 32(4) It must not redefine target-sized types, ownership, error handling, layout, or any other language semantics independently.

§ 32(5) A maintained legacy path must consume canonical frontend/plan facts or explicitly reject unsupported lowering.

§ 32(6) Governance tracks remaining divergence and retirement.

---

## § 33 CompilerTargetSupport

§ 33(1) Platform semantic capability and compiler implementation support are separate facts.

§ 33(2) `CompilerTargetSupport` or equivalent records what the current compiler/toolchain can do for one plan.

§ 33(3) Capability dimensions may include:

```text
parse/check;
Semantic IR support;
Sec MLIR support;
lowering;
object emission;
linking;
running;
debugging;
specific target operations/features.
```

§ 33(4) Exact internal capability structure is implementation-governed.

§ 33(5) Missing compiler support must not be reported as missing hardware/platform capability.

---

## § 34 Target capability states

§ 34(1) Platform capability availability and activation remain separate.

§ 34(2) Canonical availability may conceptually be:

```text
Supported
Unsupported
Unknown
```

§ 34(3) Activation may conceptually be:

```text
Enabled
Disabled
NotApplicable
```

§ 34(4) `Unknown` required capability must not be guessed from another target or host.

§ 34(5) TargetProfile determines semantic policy; CompilerTargetSupport determines implementation availability.

---

## § 35 Language validity versus implementation support

§ 35(1) A Sec program can be semantically valid while not yet lowerable by the current compiler.

§ 35(2) The compiler reports a structured implementation-capability limitation.

§ 35(3) It must not manufacture a language error to hide missing lowering.

§ 35(4) Similarly, a semantically valid program may be invalid for a selected target/profile when the target lacks a required semantic capability.

§ 35(5) Target invalidity and implementation incompleteness remain distinguishable.

---

## § 36 Compile-time evaluation

§ 36(1) Compile-time evaluation uses Sec semantics.

§ 36(2) It is invoked where canonical rules require compile-time facts.

§ 36(3) Consumers include:

```text
attributes;
configuration;
generic values;
enum constants;
array sizes;
layout constraints;
register/hardware facts;
target selection values;
capacity/resource proofs.
```

§ 36(4) Target-dependent evaluation uses the active CompilationPlan.

§ 36(5) Host arithmetic/pointer width must not silently determine target compile-time results.

---

## § 37 Source exclusion semantics

§ 37(1) A file or statement excluded by `@target` or `@when`:

```text
is lexed and parsed;
is formatter-visible;
is LSP-visible as excluded;
does not enter the active symbol table;
does not require inactive-only imports/types for the plan;
does not undergo ordinary active Sema;
does not reach Semantic IR;
does not reach lowering.
```

§ 37(2) Syntax errors remain syntax errors in excluded source.

§ 37(3) Target-dependent semantic errors are evaluated when the source is active.

§ 37(4) Exclusion cannot hide permanently malformed syntax.

---

## § 38 Source-shape compatibility across Variants

§ 38(1) Target/configuration variants of one logical public declaration must preserve required public-shape compatibility.

§ 38(2) Private implementation details may differ.

§ 38(3) Target-specific bodies, addresses, vectors, and lowering details may differ.

§ 38(4) A selector must not silently turn one public symbol into unrelated APIs across Variants.

§ 38(5) Compatibility rules are owned by the attribute/module/project rulebooks.

---

## § 39 Platform contract

§ 39(1) Every CompilationPlan defines required platform capabilities and implementation surfaces.

§ 39(2) Platform requirements are resolved from typed capability/profile models rather than one hardcoded list of required function names.

§ 39(3) Missing required semantic platform capability is diagnosed before incompatible lowering.

§ 39(4) Missing platform implementation for an otherwise supported capability is a distinct platform/compiler integration error.

§ 39(5) Missing external toolchain support is distinct again.

---

## § 40 Platform functions and compiler-known surfaces

§ 40(1) Portable library code may call canonical platform abstractions.

§ 40(2) The selected platform implementation supplies the concrete operation.

§ 40(3) The compiler validates that the active plan has a valid provider/implementation for every reachable required platform operation.

§ 40(4) Provider identity and effects are preserved through analysis/lowering.

§ 40(5) Unused platform operations need not be linked.

---

## § 41 Architecture model

§ 41(1) Architecture is a canonical typed identity.

§ 41(2) It provides machine representation fundamentals such as:

```text
pointer/word constraints;
endianness;
legal machine operation classes;
architecture capability bounds.
```

§ 41(3) Exact compiler-recognized names are registry data rather than language keywords.

§ 41(4) `amd64`, `arm64`, `arm32`, and `riscv32` are examples of canonical architecture identities where registered.

§ 41(5) Unsupported/unknown architecture identities are diagnosed during plan resolution.

---

## § 42 CPU model

§ 42(1) CPU selects the machine capability baseline.

§ 42(2) CPU may permit instruction capabilities beyond a generic architecture baseline only through canonical CPU/feature resolution.

§ 42(3) `TuneCPU` is an optimization preference and cannot broaden CPU capability.

§ 42(4) CPU features are validated for dependencies/conflicts.

§ 42(5) Device may constrain/default CPU and features.

---

## § 43 Device model

§ 43(1) Device is optional concrete hardware identity distinct from CPU.

§ 43(2) Device is especially relevant to MCU/SoC targets.

§ 43(3) Device may define/constrain:

```text
memory map;
RAM/flash topology;
MMIO;
interrupts;
peripherals;
DMA;
registers;
startup requirements;
link placement;
CPU/architecture compatibility.
```

§ 43(4) The compiler consumes Device facts through the frozen plan.

§ 43(5) Device data does not become arbitrary executable compiler plugins by default.

---

## § 44 Address spaces and memory environment

§ 44(1) Target memory/address-space facts come from the plan's resolved memory environment.

§ 44(2) They drive:

```text
safe references;
RawPtr;
MMIO;
DMA;
fixed addresses;
storage allocation;
layout;
atomics;
lowering.
```

§ 44(3) Downstream passes must not infer memory-space meaning from raw integer address ranges alone when the plan provides canonical facts.

---

## § 45 ABI environment

§ 45(1) ABI is a resolved plan fact.

§ 45(2) ABI affects:

```text
call lowering;
foreign calls;
aggregate classification;
symbol/link rules;
varargs;
callbacks;
target runtime integration.
```

§ 45(3) ABI selection is distinct from source-language function/type semantics.

§ 45(4) Backend defaults do not override an explicit/resolved ABI.

---

## § 46 Link environment

§ 46(1) Link semantics consume the plan's resolved `LinkEnvironment`.

§ 46(2) It may contain:

```text
object format;
symbol model;
entry convention;
startup objects;
platform libraries;
relocation model;
linker capabilities.
```

§ 46(3) Host linker defaults must not silently define target semantics.

§ 46(4) Toolchain discovery may locate compatible tools but cannot change resolved target/link meaning.

---

## § 47 Runtime environment

§ 47(1) Runtime support is plan/profile dependent.

§ 47(2) The compiler derives runtime requirements from reachable program semantics.

§ 47(3) Possible runtime support includes:

```text
panic endpoint;
allocation provider;
task/thread runtime;
reference metadata helpers;
test harness;
platform helpers.
```

§ 47(4) No universal Sec runtime is mandatory.

§ 47(5) Runtime-free/freestanding programs remain supported where their required semantics permit it.

---

## § 48 Execution environment

§ 48(1) Execution facts are plan-dependent and may include:

```text
hosted/freestanding status;
threads/tasks;
TLS;
interrupt model;
scheduler/runtime availability;
blocking constraints;
startup/shutdown model.
```

§ 48(2) These facts participate in effects, transferability, stack, ISR, allocation, and concurrency analysis.

§ 48(3) The compiler must not infer them from OS naming alone when the plan provides a more precise TargetProfile/ExecutionEnvironment.

---

## § 49 TargetProfile constraints

§ 49(1) TargetProfile may strengthen requirements beyond general Sec validity.

Examples:

```text
no allocation;
no panic;
no blocking;
bounded stack;
bounded Arena/storage demand;
runtime-free references;
ISR-safe effects;
restricted dynamic ownership bookkeeping.
```

§ 49(2) Required `Unproven` facts may reject compilation under such a profile.

§ 49(3) Diagnostics identify the profile requirement and missing/violated proof.

---

## § 50 Compiler-known target resources

§ 50(1) Platform/device models may expose canonical compiler-known resources.

Examples:

```text
Interrupt.*
Peripheral.*
MemoryRegion.*
Device.*
Architecture.*
CPU.*
```

§ 50(2) Exact exposed resource namespaces are defined by their owning rulebooks/registries.

§ 50(3) Compiler-known identities are preferred over raw numbers where canonical metadata exists.

§ 50(4) Raw numeric escape hatches remain governed by unsafe/platform rules.

---

## § 51 Interrupt integration

§ 51(1) The compiler resolves named interrupt/vector identities from the active plan/platform metadata.

§ 51(2) ISR bindings do not require user code to hardcode raw numeric vector values where canonical identities exist.

§ 51(3) Interrupt source semantics are validated before wrapper/vector generation.

§ 51(4) Generated interrupt artifacts are plan-scoped and deterministic.

§ 51(5) Interrupt binding does not itself enable/unmask/configure the hardware source.

---

## § 52 Hardware register integration

§ 52(1) Compiler-known register/MMIO declarations consume canonical device/platform/layout/storage facts.

§ 52(2) The compiler preserves exact access width, volatile semantics, ordering/completion, and target availability.

§ 52(3) Hardware signal polarity is not compiler target semantics.

§ 52(4) Platform metadata does not authorize arbitrary source-visible hardware access outside canonical contracts.

---

## § 53 Allocation providers

§ 53(1) Allocation/backing providers are plan-resolved capabilities.

§ 53(2) The compiler must not invent/fallback to a host or unrelated allocator when the selected plan lacks one.

§ 53(3) Allocation contexts and Arena planning consume canonical provider facts.

§ 53(4) Provider failure/effects remain visible.

§ 53(5) Runtime-free/no-allocation profiles may have no ordinary dynamic provider.

---

## § 54 Reference representation policy

§ 54(1) Safe-reference semantics are target-independent language guarantees.

§ 54(2) The plan may select runtime representation/hardening policy.

§ 54(3) Possible implementations include:

```text
address-only proven references;
address plus epoch;
capability/tagged pointers;
slot/handle metadata;
side-table validation.
```

§ 54(4) A profile may remove checks/metadata after proof.

§ 54(5) It may not weaken `ref` into raw-pointer semantics.

---

## § 55 Compiler front end

§ 55(1) The frontend owns:

```text
lexing;
parsing;
recovery;
declaration collection;
name resolution;
type/interface/generic resolution;
compile-time semantic queries;
Sema;
coordinated analyses;
source selection;
target-dependent semantic validation.
```

§ 55(2) The frontend produces verified semantic facts/Semantic IR, not target machine instructions.

§ 55(3) Frontend components share canonical workspace/project/plan state.

---

## § 56 Analysis integration

§ 56(1) The compiler analysis layer follows `compiler_analysis.md`.

§ 56(2) Analyses may be local, interprocedural, graph-based, fixed-point, demand-driven, incremental, or parallel.

§ 56(3) Each semantic fact has one canonical owner.

§ 56(4) Compiler, LSP, and `sec analyse` consume the same fact model.

§ 56(5) Analyses must distinguish `Valid`, `Invalid`, and `Unproven` where canonical proof requirements use that model.

---

## § 57 Lowering-readiness

§ 57(1) A validated function/module reaches lowering only when required semantic and target facts are complete.

§ 57(2) Lowering-readiness includes:

```text
resolved types/names;
interface/generic/Iterator resolution;
ownership/borrow/lifetime validity;
cleanup plan;
effects/guarantees;
target/profile validity;
layout facts where required;
ABI/platform operation support;
current compiler representation capability.
```

§ 57(3) Advisory warnings do not block lowering.

§ 57(4) Required missing proof or unsupported current implementation does.

---

## § 58 Backend orchestration

§ 58(1) The compiler backend orchestrates verified lowering rather than reinterpreting source semantics.

§ 58(2) Canonical current path:

```text
Semantic IR
    -> Sec MLIR
    -> lower MLIR
    -> LLVM dialect / LLVM IR
    -> object
    -> linker/artifact
```

§ 58(3) Alternate verified backends may be added later.

§ 58(4) Backend choice must preserve identical language semantics.

---

## § 59 MLIR toolchain

§ 59(1) The compiler may use in-process or out-of-process MLIR tools.

§ 59(2) Tool location is host/toolchain configuration, not source-language semantics.

§ 59(3) The maintained real verifier/toolchain is authoritative for MLIR structural validity.

§ 59(4) Tool discovery must not redefine target semantics.

§ 59(5) Missing toolchain support is a toolchain/compiler capability error.

---

## § 60 Object and link generation

§ 60(1) Object generation and linking follow the active plan.

§ 60(2) The compiler passes canonical target triple/object format/ABI/link facts to the backend/toolchain.

§ 60(3) Host startup objects/libraries/link defaults must not leak into a cross-target artifact.

§ 60(4) Only required reachable runtime/platform/library components are linked.

§ 60(5) Multi-Variant artifacts remain separated.

---

## § 61 Build outputs

§ 61(1) Project output structure is owned by project/build rules.

§ 61(2) Each Target/Variant/BuildProfile artifact has a distinct output identity/path.

§ 61(3) One Variant must not overwrite another Variant's artifact.

§ 61(4) Platform-appropriate extensions may be applied according to the LinkEnvironment/artifact kind.

---

## § 62 Build command semantics

§ 62(1) A project build selects a Target and one or more configured Variants.

§ 62(2) Filters may narrow configured Variants.

§ 62(3) Filters do not silently create undeclared Variants unless another canonical project rule explicitly permits such creation.

§ 62(4) Each selected Variant produces an independent plan and result.

§ 62(5) Overall command success is determined by project/build orchestration policy.

§ 62(6) Exact CLI spelling belongs to the CLI/project tooling contract, not this core compiler rulebook.

---

## § 63 Single-source/developer commands

§ 63(1) Compiler developer/inspection commands may compile source outside a full project when their command contract defines a valid default compilation request.

§ 63(2) Such commands must still construct a coherent plan for every target-dependent operation they perform.

§ 63(3) They must not use host target facts implicitly beyond explicitly defined default/host-resolution behavior.

§ 63(4) A developer command may stop before object/link stages.

---

## § 64 Compiler inspection surfaces

§ 64(1) The compiler may expose inspection views for:

```text
tokens;
AST;
Sema/analysis;
diagnostics registry;
Semantic IR;
Sec MLIR;
lowered MLIR;
LLVM IR;
target/CompilationPlan;
link plan;
test plan.
```

§ 64(2) Exact subcommand names/options are mutable tooling surface unless separately stabilized.

§ 64(3) Inspection output identifies the relevant target/Variant/plan/schema where required.

§ 64(4) An inspection command must not use a separate semantic implementation.

---

## § 65 `sec check`

§ 65(1) A semantic check command validates source for the selected compilation context without requiring final artifact creation.

§ 65(2) It runs every mandatory frontend/analysis/target validation required by the selected profile.

§ 65(3) It should not require backend tools merely to establish source validity where no backend-verifiable target fact is semantically required.

§ 65(4) It may optionally report implementation-capability gaps for requested later boundaries.

---

## § 66 `sec build`

§ 66(1) `sec build` produces the requested project/artifact according to project/pipeline rules.

§ 66(2) It may build several Variants.

§ 66(3) Build semantics use the canonical pipeline and frozen plans.

§ 66(4) The compiler does not choose direct LLVM as a semantic default architecture merely because a legacy implementation path exists.

§ 66(5) Exact backend-selection escape hatches, where temporarily exposed, are implementation/tooling governance.

---

## § 67 `sec run`

§ 67(1) `sec run` builds a runnable hosted artifact and executes it when the host/tooling configuration can run the selected Variant.

§ 67(2) Runtime program arguments are not compile-time parameters.

§ 67(3) Cross-target inability to execute locally is a tooling/run capability issue, not a Sec semantic error.

§ 67(4) Exact CLI status/availability belongs to implementation governance.

---

## § 68 `sec test`

§ 68(1) `sec test` uses the canonical testing compilation model.

§ 68(2) Compiler workspace owns test discovery and stable test identities.

§ 68(3) Test selection feeds a canonical `TestCompilationPlan`.

§ 68(4) Test execution uses the same semantic/lowering pipeline as ordinary code.

§ 68(5) Editors/LSP do not implement separate Sec test discovery/runners.

---

## § 69 LSP compiler reuse

§ 69(1) LSP uses compiler workspace/project/plan/Sema/analysis services.

§ 69(2) It must not independently implement:

```text
target resolution;
module imports;
interface/Iterator semantics;
test discovery;
ownership/lifetime;
effect analysis;
platform source selection.
```

§ 69(3) LSP may use reduced analysis budgets where safe.

§ 69(4) Complete LSP analysis for one snapshot/plan agrees with compiler check/build semantics.

---

## § 70 Source and compiler option separation

§ 70(1) Source program configuration and compiler build configuration are separate.

§ 70(2) Source-visible build configuration uses typed project parameters.

§ 70(3) Optimization/debug/tool-path choices are compiler/build options.

§ 70(4) Compiler options do not become arbitrary source identifiers.

§ 70(5) Source APIs must not depend on optimizer choice.

---

## § 71 Determinism

§ 71(1) Equivalent compiler semantic version, registry semantics, project configuration, Target, Variant, and explicit overrides produce deterministic resolved semantic results.

§ 71(2) Determinism includes:

```text
CompilationPlan;
source selection;
canonical identities;
interface/Iterator resolution;
analysis facts;
Semantic IR;
generated helpers;
link roots;
diagnostic ordering/provenance.
```

§ 71(3) Results must not depend on:

```text
map iteration order;
filesystem enumeration order;
parallel compiler scheduling;
process-local pointer values;
implicit host architecture/CPU.
```

---

## § 72 Incremental invalidation

§ 72(1) Compiler workspace/incremental systems invalidate by semantic dependency.

§ 72(2) Changes to:

```text
Target/Variant;
registry/platform facts;
project parameters;
@target/@when;
source/module graph;
types/conformances;
Iterator target;
FFI/platform contracts;
BuildProfile where output-relevant;
TargetProfile;
toolchain capability
```

invalidate affected results.

§ 72(3) Prior positive plan-dependent results must not remain active after the plan becomes invalid.

§ 72(4) Target-independent editor functionality may continue where sound.

---

## § 73 Diagnostics

§ 73(1) Compiler diagnostics follow mentor-style rules.

§ 73(2) Plan-dependent diagnostics identify Target/Variant where useful.

§ 73(3) Diagnostic categories distinguish:

```text
unknown Target/Variant/config;
invalid platform configuration;
source-selection conflict;
language semantic error;
profile requirement violation;
missing platform capability;
missing platform implementation;
missing compiler support;
missing toolchain support;
link failure;
internal compiler error.
```

§ 73(4) A missing lowering implementation must not masquerade as a type error.

---

## § 74 Resolution provenance

§ 74(1) Effective plan values preserve resolution provenance where useful.

§ 74(2) Provenance may indicate:

```text
compiler default;
TargetProfile;
project Variant;
Target override;
Device;
command-line override;
explicit native discovery;
toolchain/user configuration.
```

§ 74(3) Diagnostics/tooling may explain how an effective value was selected.

§ 74(4) Provenance participates in incremental invalidation.

---

## § 75 Registry model

§ 75(1) Platform/compiler registries contain canonical definitions and typed references.

§ 75(2) Registry data is normalized and structurally validated before use.

§ 75(3) Unknown references, duplicate identities, illegal overrides, malformed ranges, invalid feature dependencies, malformed memory/interrupt metadata, and incompatible definitions are rejected.

§ 75(4) Registry insertion order must not affect resolution.

§ 75(5) The registry is not queried as mutable global semantic state by arbitrary downstream passes.

---

## § 76 Resolver model

§ 76(1) The platform resolver consumes normalized project/Target/Variant/override inputs plus a registry snapshot.

§ 76(2) It produces one verified frozen CompilationPlan.

§ 76(3) Resolution is deterministic.

§ 76(4) Ambiguous defaults are rejected.

§ 76(5) Required unknown facts are rejected/deferred according to their canonical policy rather than guessed.

---

## § 77 Resolved models

§ 77(1) Downstream compiler stages consume immutable resolved models referenced by the plan.

§ 77(2) They do not query unresolved platform definitions directly.

§ 77(3) Examples include:

```text
ArchitectureModel;
DeviceModel;
MemoryEnvironment;
RuntimeEnvironment;
ExecutionEnvironment;
ABIModel;
LinkEnvironment;
ResolvedTargetProfile.
```

§ 77(4) Exact internal decomposition may differ while preserving semantic ownership.

---

## § 78 Platform metadata trust

§ 78(1) Compiler-builtin platform metadata is trusted compiler input but must be schema/semantic validated.

§ 78(2) External declarative metadata may be supported after validation.

§ 78(3) Arbitrary executable compiler plugins are not required for Sec 0.1 platform definitions.

§ 78(4) Platform metadata cannot bypass project/package/source-location rules.

§ 78(5) Trusted metadata provenance remains available to diagnostics/auditing where relevant.

---

## § 79 Compiler/toolchain separation

§ 79(1) A semantically valid CompilationPlan may exist even when the compiler host lacks the required linker, SDK, assembler, emulator, or device programmer.

§ 79(2) Toolchain availability is separate from platform semantics.

§ 79(3) Host configuration may locate tools.

§ 79(4) PATH/default tool behavior must not silently define target semantics.

§ 79(5) Toolchain capability is validated before dependent artifact stages.

---

## § 80 Error classes

§ 80(1) The compiler distinguishes at least:

```text
ConfigurationError
SourceError
TargetError
CompilerCapabilityError
ToolchainError
LinkError
InternalCompilerError
```

as conceptual classes.

§ 80(2) Exact public error type names are tooling implementation details unless stabilized elsewhere.

§ 80(3) Diagnostic category and exit status must preserve the semantic distinction.

---

## § 81 Internal compiler errors

§ 81(1) An internal compiler error indicates a violated compiler invariant.

Examples include:

```text
invalid verified Semantic IR generated from accepted source;
Sec MLIR verifier failure from supported valid emitter output;
contradictory frozen CompilationPlan;
impossible ownership state after validation;
invalid target metadata embedded by the compiler;
miscompiled supported construct.
```

§ 81(2) ICE reporting includes enough snapshot/plan/source context for reproduction where possible.

§ 81(3) User source must not be blamed for an internal invariant failure.

---

## § 82 Current implementation versus canonical compiler

§ 82(1) This rulebook defines the canonical compiler model, not mutable repository progress.

§ 82(2) Current implementation may temporarily contain:

```text
legacy target strings;
bootstrap source imports;
legacy #target parsing;
legacy direct LLVM;
partial Sec MLIR coverage;
partial CLI commands;
partial target registry;
hardcoded compatibility paths.
```

§ 82(3) Those are tracked only in governance.

§ 82(4) Their existence does not create alternative normative semantics.

---

## § 83 Governance synchronization

§ 83(1) `implementation-status.yaml` is the canonical current implementation ledger.

§ 83(2) Implementation-status entries use `compiler.md` as the canonical compiler rulebook path.

§ 83(3) Bootstrap implementation notes removed from this book remain preserved in governance where still relevant.

§ 83(4) P1–P14 remain implementation-history records.

§ 83(5) Later canonical decisions such as `Iterator[T]`, `@target`, `@when`, Target/Variant, and CompilationPlan may create additional remaining work against older implementation packages without rewriting their historical completion claims.

---

## § 84 Required tests: Target and Variant

§ 84(1) Required tests include:

```text
one Target with multiple Variants;
Variant-specific OS/arch/ABI/CPU facts;
Target is not parsed as os-arch identity;
per-Variant independent CompilationPlan;
failure of one Variant does not alter another plan;
Variant-scoped diagnostics;
output separation.
```

---

## § 85 Required tests: host separation

§ 85(1) Required cross-compilation tests verify:

```text
target pointer width from plan;
target endianness from plan;
target ABI/object format from plan;
target CPU features from plan;
platform source selection from plan;
no host CPU implicit fallback;
no host linker/startup/library semantic leakage.
```

---

## § 86 Required tests: source selection

§ 86(1) Required tests include:

```text
file-level @target;
statement-level @target;
target selector conjunction;
unknown selector/value diagnostics;
overlapping active declaration variants;
public-shape compatibility;
@when typed config;
unknown config;
excluded source still parses;
excluded source absent from active symbol/Sema/IR;
target identity rejected inside @when where attributes rules forbid it.
```

---

## § 87 Required tests: compatibility migration

§ 87(1) During compatibility migration, tests may verify:

```text
legacy #target accepted only where compatibility policy permits;
legacy syntax maps to canonical target facts;
formatter/new generated code uses @target;
no legacy compiler.target_os/target_arch dependency in new canonical frontend paths;
legacy #if does not remain a second normative selection engine.
```

§ 87(2) Compatibility tests may be removed when migration support is intentionally retired.

---

## § 88 Required tests: profiles

§ 88(1) Tests include:

```text
BuildProfile does not change semantic platform capability;
TargetProfile enforces semantic constraints;
TargetProfile and BuildProfile resolved independently;
strict noAlloc/noPanic/noBlock profile proof;
invalid profile composition rejected;
profile provenance visible.
```

---

## § 89 Required tests: compiler-known interfaces

§ 89(1) Tests include:

```text
Iterator[T] registered as compiler-known interface;
user type explicit conformance;
static Next target;
no method-name iterator discovery;
no closed iterable whitelist;
LSP/compiler parity;
specialized builtin iteration preserves interface semantics.
```

---

## § 90 Required tests: compiler architecture

§ 90(1) Tests include:

```text
source -> Semantic IR -> Sec MLIR canonical path;
Semantic IR verification;
Sec MLIR verification;
legacy direct LLVM consumes canonical target facts or rejects unsupported path;
target-sized int parity across maintained paths;
unsupported valid lowering classified as CompilerCapabilityError;
no backend semantic repair.
```

---

## § 91 Required tests: platform registry

§ 91(1) Tests include:

```text
deterministic normalization;
duplicate identity rejection;
unknown reference rejection;
CPU feature dependency/conflict rejection;
Device constraints;
memory-region validation;
interrupt metadata validation;
immutable plan freeze;
invalid plan cannot feed downstream analysis.
```

---

## § 92 Required tests: incremental/tooling

§ 92(1) Tests include:

```text
Target/Variant edit creates new plan;
@target/@when edit invalidates source selection;
config edit invalidates plan/cache;
Iterator conformance edit invalidates loop/Semantic IR/LSP;
platform registry edit invalidates plan-dependent facts;
LSP project watcher refresh;
deterministic diagnostics after parallel rebuild.
```

---

## § 93 Completion criteria

§ 93(1) Compiler platform integration is complete when every supported Target/Variant resolves deterministically to one verified immutable CompilationPlan and every target-dependent consumer uses it.

§ 93(2) Source selection is complete when `@target`, `@when`, project configuration, platform routing, and excluded-source semantics use one canonical model.

§ 93(3) Frontend integration is complete when compiler, LSP, and analysis share project/module/type/interface/Iterator/target facts.

§ 93(4) Backend integration is complete when the canonical Semantic IR -> Sec MLIR lowering path covers maintained Sec 0.1 semantics and legacy paths either consume canonical facts or reject unsupported constructs explicitly.

§ 93(5) Governance is complete when mutable target capability, CLI implementation, bootstrap import, and backend progress live only in `implementation-status.yaml`.

§ 93(6) Cross compilation is complete when no unrequested host property can affect target semantic facts.

§ 93(7) The compiler is not considered fully supportive of a language feature merely because parsing or Sema accepts it; end-to-end support is measured to the requested artifact/execution boundary.

---

## § 94 Core summary

§ 94(1) A Sec Target is a logical buildable product; a Variant is one concrete build configuration.

§ 94(2) One Target + Variant resolves to one immutable verified CompilationPlan.

§ 94(3) CompilationPlan is the sole downstream source of target/platform facts.

§ 94(4) BuildProfile and TargetProfile are distinct.

§ 94(5) The compiler host and program target are strictly separated.

§ 94(6) `@target` is the canonical target source-selection syntax; legacy `#target` is compatibility-only.

§ 94(7) `@when(config.<name>)` is the canonical typed configuration selection model; legacy `#if`/`compiler.target_os`/`compiler.target_arch` are not canonical Sec 0.1 source-selection APIs.

§ 94(8) Sec has no textual preprocessor.

§ 94(9) Platform capabilities are resolved before platform implementation source routing.

§ 94(10) `Iterator[T]` is a compiler-known generic interface resolved through the ordinary conformance model.

§ 94(11) The canonical compiler architecture is source -> verified Semantic IR -> verified Sec MLIR -> lower MLIR/LLVM -> target artifacts.

§ 94(12) Legacy direct LLVM may exist temporarily but does not define language semantics.

§ 94(13) P1–P14 are implementation milestones, not alternative language or pipeline specifications.

§ 94(14) Source invalidity, target/profile invalidity, missing compiler support, missing toolchain support, and internal compiler errors are distinct.

§ 94(15) Mutable implementation/governance data belongs in `implementation-status.yaml`, not in this normative rulebook.
