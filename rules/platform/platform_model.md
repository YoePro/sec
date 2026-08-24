# Platform Model

- **Status:** Normative
- **Created:** 2026-08-13
- **Last updated:** 2026-08-24
- **Document revision:** 1
- **Sec language version:** 0.1
- **Canonical path:** `rules/platform/platform_model.md`

---

## 1. Purpose

This rulebook defines the canonical compiler model that connects target-independent
Sec semantics to the concrete platform selected for one compilation.

The platform model answers one central question:

```text
Which target-dependent facts are authoritative for this compilation, and how are
those facts resolved, validated, exposed, cached, and consumed by the compiler?
```

The platform model exists so that parsing, semantic analysis, static analysis,
layout, lowering, FFI, linking, LSP tooling, and target-specific source selection do
not each invent their own interpretation of operating system, architecture, CPU,
ABI, runtime, memory, or execution environment.

All target-dependent compiler behavior must be justified by a fact in the active
`CompilationPlan` or by a canonical resolved model referenced by that plan.

The platform model does not redefine target-independent Sec language semantics.
Platform differences may restrict feature availability, representation, lowering,
or runtime support, but they must not silently change the meaning of otherwise
portable Sec source code.

Mutable implementation progress does not belong in this rulebook. Repository
implementation state is tracked by the repository-level `implementation-status.yaml`.

---

## 2. Normative ownership

This rulebook owns:

```text
Target / Variant platform terminology
Platform identity and platform resolution
TargetProfile versus BuildProfile distinction
CompilationPlan construction and invariants
canonical platform registry and identities
CPU, TuneCPU, CPU feature, and Device selection
resolved platform capability availability and cross-domain compatibility
resolved Architecture, Memory, Runtime, Execution, ABI, and Link submodels
platform source selection
compiler-target-support separation
platform diagnostics and resolution provenance
platform-related LSP reload and invalidation requirements
platform registry validation, trust boundaries, and determinism
```

This rulebook does not own the detailed semantics of:

```text
project manifest syntax             -> projects rulebook
target-profile activation, policy,
resource, and resolution semantics  -> rules/platform/target_profiles.md
ABI calling conventions             -> abi rulebook
interrupt semantics                 -> interrupts rulebook
volatile access                     -> volatile rulebook
inline assembly                     -> inline_assembly rulebook
storage ownership and lifetime      -> storage / ownership rulebooks
concurrency semantics               -> concurrency rulebooks
link-time symbol/output semantics   -> linking rulebook
FFI source semantics                -> FFI rulebook
compiler analysis algorithms        -> owning analysis rulebooks
```

The platform model composes facts from those domains. It does not duplicate them.

---

## 3. Canonical terminology

### 3.1 Target

A `Target` is one logical buildable product.

Examples include:

```text
server
worker
controller
firmware
```

A Target is not an operating-system/architecture pair.

The same Target may be built for multiple platform variants.

### 3.2 Variant

A `Variant` is one concrete requested build configuration for a Target.

A Variant may select or override platform-relevant facts such as:

```text
operating system
architecture
ABI
CPU
CPU features
TuneCPU
device
target profile
toolchain intent
build-profile settings
compile-time parameter overrides
```

A Variant is configuration input. It is not the final resolved platform model.

### 3.3 Platform

A `Platform` is the compiler-known system and execution environment selected for a
Variant.

Platform identity is distinct from platform source files. A source directory such
as a Linux- or bare-metal-specific implementation may implement operations required
by the selected platform, but file presence does not define platform semantics.

Platform capabilities are resolved first. Source routing then selects compatible
implementations.

### 3.4 BuildProfile

A `BuildProfile` controls build behavior such as optimization, debug information,
stripping, LTO, and related compiler options.

BuildProfile does not define whether the platform intrinsically supports threads,
interrupts, TLS, MMIO, or other semantic platform capabilities.

### 3.5 TargetProfile

A `TargetProfile` is a compiler-known semantic capability and constraint profile
used when resolving one platform configuration.

A TargetProfile may select, enable, disable, require, or constrain facilities
provided by the platform.

TargetProfile and BuildProfile are distinct concepts even where project syntax uses
the shorter word `profile` in an unambiguous context.

### 3.6 CompilationPlan

A `CompilationPlan` is the authoritative immutable resolved compiler contract for
one Target and one Variant.

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

A valid CompilationPlan contains concrete target facts. Later compiler stages must
consume those facts rather than re-resolving platform configuration independently.

---

## 4. Multi-Variant compilation

One logical Target may produce several Variants in one build operation.

For example:

```text
Target: server

Variants:
    linux-amd64
    linux-arm64
    linux-armv7
    windows-amd64
```

This produces four separate concrete CompilationPlans:

```text
Plan(server, linux-amd64)
Plan(server, linux-arm64)
Plan(server, linux-armv7)
Plan(server, windows-amd64)
```

A multi-Variant build must never represent several operating systems or
architectures inside one polymorphic CompilationPlan.

Target-independent compiler results may be shared across plans where their declared
dependencies remain target-independent. Plan-dependent results remain scoped to the
facts of the plan that produced them.

Diagnostics produced by plan-dependent work must retain Target and Variant identity.
Failure of one Variant does not make the semantic result of another Variant invalid.
Build orchestration may still report the overall build command as failed when any
requested Variant fails.

---

## 5. Resolved platform dimensions

A CompilationPlan exposes resolved immutable submodels rather than unresolved
manifest configuration.

A conceptual plan may contain:

```text
CompilationPlan {
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
}
```

This structure is explanatory. Implementations may use different types or split the
models differently, provided their semantic ownership remains equivalent.

---

## 6. Architecture, CPU, TuneCPU, and features

### 6.1 Architecture

Architecture identifies the machine ISA family, such as a canonical Sec identity
for AMD64, ARM64, ARM32, or RISC-V32.

Architecture owns machine representation fundamentals that are independent of a
particular operating system, including facts such as native word properties,
pointer representation constraints, endianness, legal machine operation classes,
and architectural capability bounds.

### 6.2 CPU

`CPU` selects the processor or microarchitecture capability baseline that generated
code may require.

Examples conceptually include:

```text
Architecture = amd64
CPU = raptorlake
```

and:

```text
Architecture = arm32
CPU = cortex-m4
```

Exact compiler-recognized CPU names are registry data, not language keywords.

An explicitly selected CPU may broaden the permitted machine instruction capability
set relative to a generic architecture baseline, but only through canonical CPU and
feature resolution.

### 6.3 TuneCPU

`TuneCPU` is an optional optimization preference distinct from the CPU capability
baseline.

Conceptually:

```text
CPU = generic-amd64
TuneCPU = raptorlake
```

may permit optimization and scheduling decisions tuned for Raptor Lake while still
requiring generated code to remain legal for the `generic-amd64` capability target.

TuneCPU must never broaden the instruction capabilities permitted by CPU and the
resolved CPU feature set.

### 6.4 CPU features

CPU features are resolved canonical machine capabilities.

Resolution may combine:

```text
CPU defaults
explicitly enabled features
explicitly disabled features
features required by the selected CPU or Device
```

Feature dependencies and conflicts must be validated before the CompilationPlan is
frozen.

Downstream consumers observe the resolved feature set. They do not repeat feature
merge logic.

### 6.5 Explicit CLI selection

Project configuration provides reproducible defaults. CLI configuration may
explicitly override CPU, TuneCPU, Device, or related target selections where the
platform model permits the override.

Such overrides participate in ordinary CompilationPlan resolution and compatibility
validation.

No host CPU is used implicitly as a target CPU.

If an explicit `native`-style CPU request is supported, host discovery occurs only
as an explicit resolver step. The detected host CPU and features are immediately
converted to canonical target facts. Later compiler stages do not query host CPU
state again.

---

## 7. Concrete Device model

`Device` is an optional concrete hardware identity distinct from CPU.

This distinction is especially important for MCUs. Two devices may use the same CPU
core while differing in:

```text
RAM and flash topology
memory map
MMIO regions
interrupt controller and vectors
peripherals
DMA capabilities
device-specific registers
platform startup requirements
link placement constraints
```

A Device definition may provide omitted defaults and also impose compatibility
constraints.

For example, a concrete Device may require:

```text
Architecture = arm32
CPU = cortex-m4
RequiredFeatures = {...}
```

If configuration explicitly selects an incompatible CPU or architecture, the result
is an invalid platform configuration. Override precedence never defeats Device
compatibility constraints.

Changing Device invalidates every resolved fact derived from or constrained by the
previous Device definition.

A Device identity is not required for ordinary desktop or server compilation when
Architecture, CPU, and platform configuration already provide all relevant facts.

The model does not require a database of every commercial CPU or MCU. Sec 0.1 needs
only canonical baselines and concrete devices that the compiler/platform packages
actually support.

---

## 8. Accelerator reservation

The platform model reserves the possibility of separately resolved accelerator or
compute devices.

An accelerator is not a CPU feature.

Conceptually, a future platform may expose:

```text
PrimaryProcessor
Accelerators[]
```

with an accelerator model containing independent architecture, features, memory, and
execution facts.

Sec 0.1 does not require GPU code generation, heterogeneous scheduling, GPU memory
semantics, CUDA-like source semantics, or support for any specific GPU product.

This reservation exists only to prevent the platform architecture from assuming that
all future code must execute on exactly one processor domain.

---

## 9. ArchitectureModel contract

`ArchitectureModel` is the canonical resolved machine capability model consumed by
layout and lowering.

It must be able to provide, directly or through canonical owned submodels, all
architecture-dependent facts required by compiler consumers, including as applicable:

```text
canonical architecture identity
canonical CPU identity
resolved CPU features
optional TuneCPU
native word properties
pointer-width constraints
endianness
integer operation capabilities
floating-point hardware capabilities
atomic hardware capabilities
alignment capabilities
register and instruction capabilities
```

Architecture capability is distinct from language type support.

For example, lack of a native machine instruction for a Sec wide integer or decimal
operation does not by itself remove that language type. Lowering or runtime support
may provide the operation. The compiler must distinguish:

```text
language semantic support
native hardware support
compiler lowering support
runtime fallback availability
```

Atomic hardware capabilities likewise describe implementation capability and do not
redefine the Sec concurrency memory model.

---

## 10. MemoryEnvironment

`MemoryEnvironment` describes canonical target memory topology and target memory
capabilities.

It may expose:

```text
address spaces
memory-space identities
addressability
pointer-relevant machine properties
alignment requirements
MMIO capabilities
static or device memory regions
CPU / DMA / accelerator accessibility
cacheability and coherence properties where defined
```

MemoryEnvironment does not redefine ownership, borrowing, storage origin, object
lifetime, or reclamation authority. Those remain owned by the memory/storage
rulebooks.

Canonical memory spaces require stable identities so that storage, shaped values,
FFI, raw pointers, volatile access, device registers, and later lowering can refer to
the same semantic memory domain.

MMIO must not be inferred independently from arbitrary numeric address tests inside
compiler passes. A Device or MemoryEnvironment supplies the canonical MMIO or device
memory facts consumed by the volatile/register/platform rules.

Device memory descriptions must be structurally validated for malformed ranges,
invalid address widths, illegal overlap where overlap is forbidden, conflicting
properties, and duplicate identities. Legitimate aliases or overlapping views must
be declared through canonical metadata rather than inferred accidentally.

---

## 11. RuntimeEnvironment

`RuntimeEnvironment` describes runtime services and contracts available to generated
Sec code under the active CompilationPlan.

Possible service domains include:

```text
startup
panic handling
allocation
generation tracking
thread support
task scheduling
synchronization helpers
thread-local storage
numeric helpers
compiler-required lowering helpers
```

No complete runtime is mandatory.

A valid platform may provide a minimal runtime and leave allocation, threads, tasks,
or other facilities unsupported or disabled.

Runtime service descriptions must expose semantic contracts whenever compiler
analysis or lowering depends on those properties. Such contracts may include, as
applicable:

```text
MayBlock
MayAllocate
MayPanic
Reentrant
InterruptSafe
RequiredExecutionContext
```

or references to equivalent canonical effect/runtime contracts.

RuntimeEnvironment answers what service or contract exists if lowering requires it.
It does not imply that every program uses every available runtime service.

Compiler-inserted runtime behavior that can affect correctness analysis must be
represented by canonical contracts before a positive analysis proof relies on its
absence.

---

## 12. ExecutionEnvironment

`ExecutionEnvironment` describes the resolved execution-context model applicable to
one CompilationPlan.

It may reference canonical models for:

```text
process execution
threads
tasks
scheduler behavior
interrupts
preemption
nesting
blocking
reentry
execution roots
context transitions
physical stack domains
```

ExecutionEnvironment selects and exposes these models. Detailed concurrency and
interrupt semantics remain owned by their respective rulebooks.

Static analyses must consume canonical execution relationships rather than derive
platform behavior from operating-system names or raw priority numbers.

Where relevant, consumers receive resolved relations equivalent to:

```text
MayPreempt(A, B)
CannotPreempt(A, B)
MayNest(A, B)
CannotNest(A, B)
MayReenter(A)
```

Thread and task models are distinct. A Task is not implicitly equivalent to an OS
thread.

Blocking capability and blocking policy are also distinct from whether a particular
operation has a `MayBlock` effect.

---

## 13. ABIModel and LinkEnvironment

### 13.1 ABIModel

`ABIModel` is a resolved plan component used consistently by function lowering, FFI,
machine stack reasoning, linking, and debug information.

`rules/platform/abi.md` owns the detailed semantics of calling conventions, parameter and
return placement, stack conventions, aggregate passing, symbol ABI, and foreign
interface compatibility.

Platform resolution selects one canonical ABIModel. Later compiler stages must not
guess the ABI independently from operating-system or architecture names.

### 13.2 LinkEnvironment

`LinkEnvironment` is the resolved binary and link context for the plan.

It may contain or reference:

```text
object format
symbol model
entry convention
required startup objects
platform libraries
relocation model
linker capabilities
```

Detailed linking semantics remain owned by the linking rulebook.

Object format and target linkage are selected from the CompilationPlan, never from
the compiler host by implicit fallback.

Toolchain availability is distinct from semantic platform identity. A semantic plan
may be valid even when the current host lacks the required linker or SDK.

---

## 14. Capability availability and activation

The platform model distinguishes capability availability from capability activation.

Availability has at least these semantic states:

```text
Supported
Unsupported
Unknown
```

Activation has the conceptual states:

```text
Enabled
Disabled
NotApplicable
```

`Supported` means the selected platform can provide the capability.

`Unsupported` means the resolved platform cannot provide it or explicitly forbids
it.

`Unknown` means the compiler lacks sufficient canonical information.

`Enabled` means the active CompilationPlan permits or selects use of the capability.

`Disabled` means the platform can provide the capability, but the active profile or
configuration does not enable it.

`Disabled` and `Unsupported` are not interchangeable and should produce distinct
diagnostics.

A program requirement must not implicitly enable a disabled capability.

Missing required platform facts may not be guessed from the compiler host or from
another platform.

An implementation may use an internal `Unresolved` state while constructing a plan,
but required facts must not remain unresolved in a valid frozen CompilationPlan.

---

## 15. Compiler support versus platform semantics

The platform's semantic capabilities and the current compiler's implementation
support are separate facts.

For example:

```text
Platform:
    Interrupts = Supported

CompilerTargetSupport:
    InterruptLowering = Unsupported
```

means that the platform supports interrupts but the current compiler implementation
cannot yet lower them for that target.

Compiler implementation support may be tracked per pipeline capability, for example:

```text
semantic validation
analysis
Sec MLIR lowering
object emission
linking
execution
```

A missing compiler stage must not be reported as if the user's program were
semantically invalid or as if the hardware lacked the capability.

The same semantic CompilationPlan may therefore be usable by commands such as check
or analyse even when object emission or linking is not yet implemented.

---

## 16. Canonical platform registry

Platform resolution is driven by a canonical compiler platform registry.

The registry may contain typed definitions for:

```text
operating systems
architectures
CPUs
CPU features
Devices
ABIs
TargetProfiles
runtime models
execution models
memory models
link models
platform source mappings
compiler target support
```

The physical storage of those definitions is implementation-defined.

Input spellings and aliases are normalized at registry entry. Downstream compiler
stages work with canonical identities rather than free-form strings.

Examples such as:

```text
amd64
x86_64
x64
```

may be accepted as aliases if Sec defines them, but they must normalize to one
canonical architecture identity before semantic resolution continues.

Registry entries have stable identity distinct from display name and documentation
text.

---

## 17. Registry composition and ownership

Registry components compose by canonical semantic domain rather than arbitrary
inheritance.

Examples of domain ownership include:

```text
Architecture    -> machine representation fundamentals
ABI             -> binary interface conventions
Memory          -> memory spaces and machine memory capabilities
Runtime         -> runtime services
Execution       -> execution-context behavior
Link            -> object and link environment
Device          -> concrete hardware refinements and constraints
TargetProfile   -> profile selection, activation, requirements, and policy
```

A component may validate a fact owned by another domain, but it must not independently
produce a competing value for the same canonical fact unless an explicit cross-domain
resolution rule defines that ownership.

Defaults are deterministic selection rules, not late overwrite rules.

If an ABI or CPU is omitted, the resolver may select one canonical default. Once
selected, the resulting ABI or CPU is explicit in the frozen CompilationPlan.

Ambiguous defaults are resolution errors.

---

## 18. External and built-in platform definitions

Sec 0.1 may implement platform support initially through compiler-built-in typed
registry definitions.

The architecture must also permit declarative externally supplied definitions where
supported, particularly for concrete Devices and platform packages.

External platform definitions are data, not arbitrary executable compiler plugins.

Declarative definitions may provide canonical information such as:

```text
device identity
CPU and required features
memory regions
interrupt descriptions
ABI selection
platform source mapping
runtime availability
```

An executable compiler-plugin model, if ever introduced, is a separate mechanism and
is not implied by this rulebook.

Every definition retains origin and version/fingerprint provenance sufficient for
diagnostics and cache compatibility.

Duplicate canonical definitions are errors unless a defined extension mechanism
explicitly allows one definition to refine or override another.

Load order, directory enumeration, or "last definition wins" behavior must not decide
platform semantics.

Some canonical facts may be non-overridable. External Device metadata may refine a
concrete memory map or bind a CPU, but it cannot redefine the instruction semantics
of the underlying CPU architecture.

---

## 19. Trust and validation boundaries

External registry data must be schema-validated, normalized, and structurally
verified before it enters a registry snapshot.

Validation includes, where applicable:

```text
unknown referenced identities
duplicate identities
ambiguous defaults
invalid override relationships
feature dependency or conflict errors
invalid numeric ranges
malformed memory regions
invalid interrupt metadata
invalid address widths
cross-domain compatibility errors
```

Platform metadata must not bypass Sec project/package source-location rules or cause
arbitrary host filesystem source inclusion.

Environment variables, host discovery, SDK discovery, and toolchain probing that can
affect target selection or build output are explicit resolver inputs. Downstream
compiler stages must not query such host state independently.

Malformed compiler-built-in registry data is a compiler defect.

Malformed externally supplied platform data is attributed to the defining platform
package or configuration, not to unrelated Sec source code.

---

## 20. Platform resolution algorithm

Platform resolution is an explicit compiler phase.

A conforming implementation may use different internal functions, but the semantic
staging is equivalent to:

```text
1. normalize Target, Variant, project configuration, and CLI overrides;
2. resolve canonical platform, architecture, profile, and explicit identities;
3. load Device constraints and defaults when a Device is selected;
4. select deterministic omitted defaults such as ABI or CPU;
5. resolve CPU features and validate feature dependencies/conflicts;
6. construct Architecture, Memory, Runtime, Execution, ABI, and Link submodels;
7. derive canonical target properties owned by those models;
8. validate cross-domain compatibility and Device constraints;
9. resolve capability availability and activation;
10. resolve platform/runtime source selection;
11. verify every required CompilationPlan invariant;
12. freeze the immutable CompilationPlan;
13. compute canonical semantic fingerprints;
14. validate compiler/toolchain support required by the requested pipeline.
```

Fact derivation in Sec 0.1 has an acyclic resolution order.

Cross-domain requirements should normally be checked after participating domains are
resolved rather than introducing recursive platform-resolution fixed points.

Precedence selects among legal configuration alternatives. Compatibility constraints
determine legality. A higher-precedence CLI value does not make an incompatible
CPU/Device or ABI/architecture combination valid.

---

## 21. CompilationPlan invariants

A canonical CompilationPlan is valid only after successful verification and freeze.

The verifier must ensure at least:

```text
required canonical identities exist
required facts are resolved
no required internal Unresolved state remains
all enabled CPU features are legal
all Device constraints are satisfied
all cross-domain compatibility rules are satisfied
runtime requirements are coherent with execution/memory capabilities
source selection is coherent
required semantic submodels are present
semantic fingerprints can be computed deterministically
```

Arbitrary partially initialized CompilationPlans are not valid compiler inputs.

Compiler implementations should enforce construction through an internal resolver or
builder followed by verification and freeze rather than allowing unrelated passes to
construct ad-hoc plans.

After freeze, the plan is immutable for one compilation or analysis generation.
Configuration changes create new plans rather than mutating facts beneath existing
analysis state.

---

## 22. Source selection

Platform source selection is derived from canonical resolved plan identities.

The compiler must not infer platform semantic capabilities from directory names or
file existence.

A frozen `SourceSelection` may include:

```text
project sources
platform-common sources
architecture-specific sources
Device-specific sources
runtime sources
generated sources where defined
```

Source selection should retain provenance explaining why a source participated in the
plan.

Selection must be deterministic when several platform implementations exist. The
platform registry or source-routing rules define matching and specificity; arbitrary
filesystem depth or enumeration order does not.

The compiler should detect missing required platform implementations before a later
link failure obscures the real cause.

Which platform operations are required depends on the active plan; not every platform
must provide every operation known to Sec.

Source-validation directives such as target constraints validate compatibility with
the selected CompilationPlan. They do not choose or mutate the platform plan.

---

## 23. Fingerprints and dependency domains

Resolved plans expose stable semantic identities suitable for cache and incremental
compilation.

Implementations should provide domain-specific fingerprints such as the semantic
equivalent of:

```text
ArchitectureFingerprint
LayoutFingerprint
MemoryFingerprint
RuntimeFingerprint
ExecutionFingerprint
ABIFingerprint
LinkFingerprint
OptimizationFingerprint
```

and a composite plan fingerprint where useful.

A semantic fingerprint is computed from canonical resolved meaning, not file
modification time, registry insertion order, display text, or configuration
provenance that resolves to the same semantic value.

A change to display name or comments in Device metadata does not require semantic
invalidation when the canonical resolved semantics remain identical.

TuneCPU affects only optimization-related fingerprints when it does not change
permitted capabilities or source-visible target facts.

CPU capability selection, Device changes, ABI changes, memory changes, or other facts
participate in every fingerprint whose result can depend on them.

Compiler passes and analyses should declare which plan domains they depend on so that
incremental invalidation can be selective.

Conservative over-invalidation is permitted. Reuse of a stale plan-dependent result is
not.

---

## 24. Diagnostics

Platform resolution produces structured diagnostics.

The diagnostic model must distinguish at least these causes conceptually:

```text
UnknownDefinition
InvalidConfiguration
UnsupportedCombination
DisabledCapability
MissingRequiredCapability
MissingPlatformImplementation
MissingCompilerSupport
MissingToolchainSupport
RegistryDefinitionError
```

An unknown identity is not the same as a known but unsupported combination.

A disabled capability is not the same as an unsupported capability.

Missing compiler support is not the same as invalid user source.

Diagnostics should retain resolution provenance sufficient to explain effective
configuration where relevant.

For example:

```text
device `ExampleDevice` requires CPU `cortex-m4`,
but CPU `cortex-m7` was selected by command-line override
```

is preferable to a generic `invalid target` message.

The resolver should report independent configuration errors together when doing so
does not require fabricated assumptions. Dependency-blocked secondary errors should
be suppressed rather than cascading from missing prerequisite facts.

Diagnostic ordering and representative causes are deterministic.

---

## 25. LSP and configuration reload

The LSP uses the same canonical project configuration, platform registry, and
CompilationPlan resolver as command-line compilation.

At workspace/project load, the LSP resolves active Target/Variant plans before
publishing plan-dependent semantic results.

When project, platform, Device, profile, ABI, CPU, or other relevant configuration
changes, the LSP:

```text
loads a new configuration/registry snapshot where required
resolves affected CompilationPlans
compares semantic sub-fingerprints
invalidates affected plan-dependent state
refreshes diagnostics and analysis without restart
```

A temporarily invalid configuration must not leave positive plan-dependent results
from the previous valid plan presented as current.

Target-independent editor functionality may continue where sound, including parsing
and other facts whose dependencies do not require the unresolved plan.

An earlier valid plan may be retained as historical/tooling state but must not serve
as the current proof basis for a changed invalid configuration.

Registry changes invalidate only dependent definitions, plans, and derived compiler
state where dependency information permits that precision.

---

## 26. Cross compilation and host separation

The compiler host and program target are strictly separate environments.

Cross compilation obtains target facts from the active CompilationPlan.

Facts such as:

```text
pointer width
endianness
ABI
object format
CPU features
memory topology
platform source selection
interrupt model
```

must never be silently taken from the compiler host.

The target binary need not execute on the host for compilation to succeed.

Host-dependent discovery is permitted only as an explicit resolver input, such as a
user-requested native CPU selection or toolchain discovery, and its resolved result is
then frozen into the target/build configuration.

---

## 27. Determinism

For the same compiler semantic version, canonical registry semantics, project
configuration, Target, Variant, and explicit overrides, platform resolution must
produce the same:

```text
canonical identities
resolved submodels
capability states
source selection
semantic fingerprints
resolution diagnostics and ordering
```

Results must not depend on:

```text
map iteration order
registry insertion order
filesystem enumeration order
parallel compiler scheduling
compiler host architecture except through explicit requested discovery
```

The platform registry itself must be testable independently for deterministic
normalization, references, defaults, constraints, and structural validity.

---

## 28. Required implementation strategy

A conforming implementation should separate three logical layers:

```text
Registry
Resolver
Resolved Models
```

`Registry` contains canonical definitions and references.

`Resolver` consumes normalized configuration and produces a verified plan.

`Resolved Models` are immutable compiler inputs consumed by later stages.

Downstream Sema, analysis, layout, lowering, and linking stages must not query the
unresolved platform registry directly.

A practical Sec 0.1 implementation order is:

```text
1. implement canonical built-in registry identities and typed definitions;
2. implement immutable resolved submodels;
3. implement registry validation and snapshots;
4. implement normalization and deterministic defaults;
5. implement CPU, feature, and Device resolution;
6. implement domain-model resolution and cross-validation;
7. implement capability availability/activation;
8. implement source selection;
9. implement plan verification and freeze;
10. implement semantic fingerprints and dependency-domain queries;
11. integrate structured diagnostics and LSP reload;
12. add declarative external Device/platform definitions after the built-in path is stable.
```

The physical Go package layout is implementation-defined.

---

## 29. Required test coverage

The implementation must include positive, negative, incremental, cross-platform, and
determinism coverage for the platform model.

Required categories include:

### 29.1 Identity and defaults

```text
canonical identities and aliases
unknown identities
duplicate identities
single deterministic defaults
ambiguous-default rejection
explicit values overriding legal defaults
```

### 29.2 CPU and Device

```text
generic CPU baseline
explicit CPU selection
CLI CPU override
TuneCPU that does not broaden capabilities
feature dependencies and conflicts
Device-supplied architecture/CPU/features
compatible and incompatible CPU/Device combinations
Device replacement invalidating old derived facts
same CPU with different Device memory maps
explicit native CPU resolution with no implicit native selection
```

### 29.3 Capability states

```text
Supported + Enabled
Supported + Disabled
Unsupported
Unknown optional fact
Unknown required fact
source requiring disabled capability
source requiring unsupported capability
```

### 29.4 Memory, runtime, and execution

```text
default and multiple memory spaces
MMIO and Device regions
minimal runtime
allocator/runtime unavailable
threads and tasks
TLS
interrupt/preemption relations
runtime helper contracts
interrupt-safe and non-reentrant runtime services
```

### 29.5 Multi-Variant and cross compilation

At minimum, a regression must cover one Target built as:

```text
linux-amd64
linux-arm64
linux-armv7
windows-amd64
```

and verify four concrete CompilationPlans, correct platform source routing, correct
architecture/ABI facts, safe target-independent reuse, plan-dependent isolation, and
Variant-scoped diagnostics.

Cross-compilation tests must verify that target pointer width, ABI, object format,
endianness, CPU capabilities, and platform sources do not leak from the host.

### 29.6 Registry and external metadata

```text
valid external Device data
unknown referenced identities
duplicate definitions
illegal overrides
malformed memory regions
invalid interrupt metadata
unsupported schema version
semantic definition changes invalidating dependent plans
display-only changes preserving semantic fingerprints
```

### 29.7 LSP and incremental behavior

```text
CPU change
TuneCPU-only change
Device change
ABI change
profile/runtime change
platform source mapping change
unrelated Variant change
valid -> invalid -> valid configuration edits
old positive plan-dependent results becoming stale immediately
new plans taking effect without LSP restart
```

### 29.8 Determinism and property testing

Resolution must be tested under different registry/map insertion orders, filesystem
enumerations, and parallel plan scheduling.

Property/fuzz testing should verify invariants such as:

```text
FrozenPlan implies required identities are resolved
FrozenPlan implies cross-domain compatibility
Device constraints hold
TuneCPU never broadens CPU capabilities
Disabled is never treated as Unsupported
no consumer-visible required fact remains Unresolved
malformed external metadata cannot crash later compiler phases
```

---

## 30. Completion criteria

PlatformModel is complete for Sec 0.1 when every supported Target and Variant can be
resolved deterministically into one verified immutable CompilationPlan and all
plan-dependent compiler consumers use the same canonical facts.

Completion requires, at minimum:

```text
canonical registry identities and deterministic aliases/defaults
typed capability/property/constraint representation
separate availability and activation state
CPU, TuneCPU, feature, and Device resolution
resolved Architecture, Memory, Runtime, Execution, ABI, and Link models
multi-Variant compilation as independent concrete plans
cross-compilation without host semantic leakage
platform source selection from resolved identities
platform/compiler/toolchain support distinction
structured diagnostics and provenance
immutable snapshots and domain fingerprints
LSP reload with sound invalidation
registry validation and declarative external-definition trust boundaries
required regression, property, and determinism tests
```

Sec 0.1 does not require a complete catalog of processors, MCUs, or accelerators, and
it does not require GPU execution semantics. The model must, however, remain capable
of representing concrete Devices and future separately resolved accelerators without
breaking the primary processor model.
