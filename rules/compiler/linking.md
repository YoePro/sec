# Sec Linking

- **Status:** Normative
- **Created:** 2026-08-23
- **Last updated:** 2026-08-23
- **Document revision:** 1
- **Sec language version:** 0.1
- **Repository baseline reviewed:** `c515862`
- **Canonical path:** `rules/compiler/linking.md`

## 1. Purpose

This rulebook defines the Sec 0.1 linking model.

It specifies how already-resolved program, ABI, FFI, initialization, runtime,
platform, and project/build requirements are assembled into one concrete target
artifact.

This rulebook owns:

- canonical `LinkPlan` construction;
- resolved link environments and linker/toolchain requirements;
- binary symbol identity;
- native and foreign symbol resolution;
- typed link inputs and dependency provenance;
- static and dynamic native dependencies;
- archive-member selection;
- duplicate and coalescible symbol handling;
- link roots and binary reachability;
- dead stripping constraints;
- LTO/link-time optimization boundaries;
- concrete linker invocation materialization;
- search paths and linker defaults;
- reproducible link planning;
- final artifact verification;
- linking diagnostics and provenance.

This rulebook does not define:

- Sec module identity;
- source declaration identity;
- source visibility;
- ABI parameter/return classification;
- FFI source-position legality;
- foreign resource ownership;
- program initialization or shutdown semantics;
- storage layout;
- target-profile capabilities;
- package/version dependency resolution beyond the resolved dependency inputs supplied to linking;
- exact MLIR operation syntax;
- exact object-format binary encoding;
- operating-system loader semantics beyond the resolved `LinkEnvironment`.

Those concerns remain owned by their canonical rulebooks.

---

## 2. Core principle

Linking answers:

> How are already-resolved binary units, required runtime/startup facilities,
> native dependencies, and symbol contracts assembled into the concrete artifact
> required by this `CompilationPlan`?

Linking consumes earlier semantic decisions. It does not independently redefine
`ModuleIdentity`, `SemanticDeclarationIdentity`, `ABISignature`, FFI legality,
ownership, initialization order, or target selection.

---

## 3. LinkPlan

Every concrete `CompilationPlan` that requires binary linkage has one canonical
`LinkPlan`.

Conceptually:

```text
LinkPlan {
    CompilationPlan
    LinkEnvironment
    Output
    Inputs
    Symbols
    Roots
    NativeDependencies
    InitializationRequirements
    LinkOptions
    ToolchainRequirements
}
```

The exact implementation representation is non-normative.

The `LinkPlan` is the complete canonical physical link contract. Concrete
linker/toolchain invocation is materialized from the verified `LinkPlan`.

---

## 4. LinkEnvironment authority

The active `CompilationPlan.LinkEnvironment` is authoritative for target linkage
facts such as:

```text
object format
symbol model
entry convention
relocation model
platform linkage requirements
supported linker capabilities
startup/object conventions
dynamic-linking model
```

Downstream linking stages must not independently infer those facts from the
compiler host, host operating system or architecture, default linker behavior,
the first linker found in `PATH`, or target triples alone when the resolved plan
contains a more precise model.

---

## 5. Typed link inputs

Canonical link inputs are structured entities rather than an untyped list of
command-line strings. An implementation may use categories equivalent to:

```text
SecObject
GeneratedStartupObject
GeneratedRuntimeObject
NativeObject
StaticLibrary
SharedLibrary
ImportLibrary
Framework
PlatformLibrary
LTOUnit
```

Concrete command-line spelling is produced only when the selected toolchain is
materialized.

---

## 6. Logical dependencies and physical link inputs

Logical native dependencies are resolved through project, package, platform, or
build metadata before concrete linker invocation to the extent supported by the
target dependency model.

```text
LogicalNativeDependency
        ↓
CompilationPlan resolution
        ↓
ResolvedNativeDependency
        ↓
Concrete LinkInput
```

Ordinary Sec source does not define host-specific filesystem paths as semantic
linkage identity. A foreign `@link_name` does not create a native-library
dependency.

---

## 7. Link-input provenance

Every resolved link input retains enough provenance to explain why it
participates in the plan. Typical origins include project target metadata,
foreign binding packages, `ProgramInitializationPlan`, `ProgramShutdownPlan`,
`RuntimeEnvironment`, `TargetProfile`, platform requirements, and explicit build
configuration.

Diagnostics preserve this provenance through symbol resolution and concrete tool
invocation.

---

## 8. Symbol identity layers

Sec distinguishes:

```text
SemanticDeclarationIdentity
ABISignature / ABIFingerprint
BinarySymbolIdentity
EncodedLinkSymbol
```

These are distinct concepts.

`SemanticDeclarationIdentity` belongs to the language/module model.
`ABISignature` and `ABIFingerprint` belong to the ABI model.
`BinarySymbolIdentity` is the canonical compiler-level identity used by linking.
`EncodedLinkSymbol` is the object-format/toolchain representation of that
identity.

---

## 9. Native Sec BinarySymbolIdentity

A native Sec declaration that requires binary linkage receives a deterministic,
collision-resistant `BinarySymbolIdentity` derived from canonical facts
sufficient to distinguish the binary entity, including as applicable:

```text
ModuleIdentity
DeclarationIdentity
specialization identity
required linkage identity
ABI-relevant binary identity
```

It is not derived from import alias, source-file traversal order, filesystem
enumeration order, linker input order, compiler worker scheduling, or compiler
host properties.

The exact textual mangling format is not defined here.

---

## 10. Mangling and object-format encoding

Sec-native symbol mangling and target object-format symbol encoding are separate
stages:

```text
BinarySymbolIdentity
        ↓
canonical Sec-native symbol spelling/mangling
        ↓
LinkEnvironment.SymbolModel
        ↓
EncodedLinkSymbol
```

Target decoration does not redefine semantic or canonical binary identity.

---

## 11. Foreign linkage identity

A foreign declaration may select a foreign linkage name:

```sec
@link_name("foreign_symbol")
extern "C" fn SecName(value: C::int) C::int
```

The annotation changes only the foreign/linkage symbol identity. It does not
alter Sec declaration identity, ABI family, ownership, effects, FFI legality, or
native dependency selection.

---

## 12. No general C export in Sec 0.1

General user-selected export of Sec functions as public named C ABI symbols is
outside Sec 0.1.

Compiler-generated callback thunks may have C ABI entry points where required by
FFI, but they remain compiler-generated implementation symbols unless another
canonical rule explicitly defines a public export mechanism.

---

## 13. Source visibility and binary linkage visibility

Sec source visibility and binary linkage visibility are distinct.

A declaration that is public in the Sec module model does not automatically
become an externally exported binary symbol. A public declaration may be
internalized, inlined, specialized, dead-stripped, or not materialized as an
externally visible symbol when every required contract remains satisfied.

---

## 14. Canonical linkage categories

Linking preserves enough structured metadata to distinguish the semantic
equivalents of:

```text
Private
NativeInternal
NativeLinkable
ForeignImport
GeneratedEntry
GeneratedRuntime
GeneratedCallback
PlatformRequired
```

The exact names are implementation-specific.

---

## 15. Object-unit identity

Object-file boundaries are build artifacts rather than Sec language semantics.
A compiler may emit one object per module, multiple objects per module, a
whole-program object, or LTO units without changing `ModuleIdentity` or source
declaration identity.

```text
ModuleIdentity != ObjectUnitIdentity
```

---

## 16. Output artifact kind

The concrete `CompilationPlan` resolves an explicit `OutputArtifactKind`, such
as `Executable`, `StaticLibrary`, `SharedLibrary`, or `ObjectFile` where
supported by project/build rules.

Linking does not infer a unique binary library form merely from the fact that a
source target is a library unless the owning project rules define that mapping.

Each selected Target Variant has its own `CompilationPlan`, `LinkPlan`, and
output artifact.

---

## 17. Initialization and shutdown handoff

`ProgramInitializationPlan` and `ProgramShutdownPlan` provide linking-facing
requirements such as startup objects, entry adapters, runtime components,
shutdown components, termination adapters, and required section relationships.

Initialization/shutdown rules define what facilities must exist and what
dependencies they must satisfy. Linking defines how those requirements are
realized through concrete objects, symbols, sections, libraries, and entry
integration.

---

## 18. Symbol definitions and references

After lowering, linking operates on canonical `SymbolDefinition` and
`SymbolReference` entities.

Native Sec definitions/references are associated with `BinarySymbolIdentity`.
Foreign references carry foreign linkage identity and the resolved provider or
dependency requirements supplied by FFI/build metadata.

Linking does not restart symbol resolution from source declaration spelling.

---

## 19. Native Sec symbol resolution

A native Sec reference resolves to the canonical native definition representing
the required `BinarySymbolIdentity`.

Textual encoded symbol equality does not independently define native Sec
semantic equality. If two incompatible compiler entities accidentally encode to
the same textual object symbol, the build is invalid.

---

## 20. Resolution modes

Every symbol requirement has an explicit resolution mode appropriate to the
selected `LinkEnvironment`, for example:

```text
StaticDefinition
DynamicImport
PlatformProvided
```

Additional target-specific modes may exist.

An unresolved reference in the current object set is not universally an error.
Validity depends on `OutputArtifactKind`, `LinkEnvironment`, and the resolved
symbol policy.

---

## 21. Final executable resolution

For a final executable, every mandatory symbol requirement must either resolve
to an included definition or be a valid declared dynamic/platform-provided
import permitted by the selected `LinkEnvironment`.

A permissive external-linker option does not make an otherwise invalid Sec
artifact contract valid.

---

## 22. Object-file and static-library outputs

Object-file and static-library outputs may preserve unresolved external
references intended to be satisfied by a later consuming link.

Reusable artifacts should preserve enough metadata to expose, as applicable:

```text
provided symbols
required symbols
native dependency metadata
ABI metadata
binary surface metadata
```

The serialization format is implementation-specific.

---

## 23. Duplicate native definitions

Two non-coalescible native definitions for the same `BinarySymbolIdentity` are
invalid. Link order does not select a winning native Sec definition.

Such a conflict normally indicates a compiler identity defect, duplicate input,
stale object, incompatible artifact, or invalid build state.

---

## 24. Coalescible definitions

Some compiler-generated or specialization-related definitions may legitimately
be emitted by more than one object unit. Such definitions must be explicitly
marked coalescible or equivalent and retain a canonical definition identity and
ABI compatibility requirements.

Target weak/link-once support does not automatically make arbitrary duplicate
Sec definitions legal.

---

## 25. No general source-level weak linkage

Sec 0.1 defines no general source-level weak-linkage declaration.

Weak, link-once, optional, coalescing, or equivalent linkage mechanics required
by platforms, foreign metadata, runtime objects, compiler-generated helpers, or
LTO remain linking/build metadata.

---

## 26. Static archives

A static archive is a container of link units rather than an unconditionally
expanded dependency.

```text
StaticLibrary
    contains LinkUnits
```

Archive members are selected according to unresolved requirements reachable
from the current link roots and already-selected inputs.

---

## 27. Canonical archive extraction

Archive-member selection follows fixed-point dependency resolution:

```text
requiredSymbols = current unresolved reachable requirements

repeat:
    select archive members that provide requiredSymbols
    add references introduced by selected members
until no additional member is required
```

The semantics are not defined as a single left-to-right command-line scan.
Concrete linkers may use rescans, archive groups, whole-archive mechanisms, or
other capabilities to materialize the same canonical result.

---

## 28. Native dependency ordering

Native dependency declaration order does not define Sec symbol-resolution
semantics.

Listing dependency A before dependency B does not mean A wins over B or that B
cannot satisfy A merely because of command-line order.

The `LinkPlan` derives whatever ordering or grouping is required by the selected
toolchain.

---

## 29. Cyclic native dependencies

The Sec `ModuleGraph` is acyclic, but native library dependency graphs may
contain cycles.

Such a cycle is valid when the active `LinkEnvironment` can materialize the
required fixed-point resolution. A concrete `LinkPlan` is rejected when the
required native dependency resolution cannot be implemented by the selected link
environment/toolchain.

---

## 30. Shared libraries

A shared-library dependency represents an external provider relationship rather
than archive-member extraction.

Conceptually:

```text
DynamicDependency {
    LogicalIdentity
    TargetProviderIdentity
    RequiredSymbols
    VersionRequirements
}
```

Loader format and platform provider naming belong to the selected
`LinkEnvironment`.

---

## 31. Import libraries and frameworks

Import libraries, frameworks, and equivalent target mechanisms are physical
realizations of logical native dependencies. They do not define separate Sec
source-level dependency semantics.

```text
LogicalNativeDependency
        ↓
CompilationPlan
        ↓
target-specific realization
```

---

## 32. Provider ambiguity

Native Sec binary symbols normally require one canonical provider. Multiple
incompatible providers are invalid.

Foreign or platform symbols may instead use explicit dynamic-resolution
semantics where the selected `LinkEnvironment` defines loader-time lookup,
interposition, or platform-provided resolution.

Accidental first-provider behavior is not Sec semantics.

---

## 33. Versioned foreign/provider requirements

A `LinkPlan` may carry target-specific provider or foreign-symbol version
requirements supplied by package/build/platform metadata.

Sec 0.1 does not require source syntax for those version constraints.

---

## 34. Link reachability

Linking maintains a canonical reachability model describing which binary
definitions, storage objects, and generated facilities are required by the
concrete output artifact.

Conceptually, `LinkReachabilityGraph` may contain nodes equivalent to:

```text
CallableDefinition
StaticStorage
GeneratedThunk
RuntimeHelper
StartupComponent
ShutdownComponent
PlatformComponent
```

The exact graph representation is implementation-defined.

---

## 35. Link roots

Reachability begins from explicit `LinkRoot` requirements rather than from every
compiled declaration.

The root set depends on `OutputArtifactKind`.

Executable roots include, as applicable:

```text
physical entry requirement
generated entry adapter
source-level target entry
required startup components
required shutdown components
required runtime facilities
platform-retained symbols/data
```

A reusable library has a different root set corresponding to its declared
binary-consumable surface and required support implementation.

---

## 36. Public declarations are not automatic roots

A public Sec declaration is not automatically a binary link root.

It becomes a root when the concrete artifact contract requires that declaration
to remain consumable across a binary boundary. Otherwise it may be inlined,
specialized, internalized, removed, or not emitted as a binary symbol when all
observable contracts remain valid.

---

## 37. Executable entry is a hard root

The physical target entry required by the active `LinkEnvironment` is a hard
root for executable artifacts.

Generated startup code may establish:

```text
physical target entry
        ↓
generated startup adapter
        ↓
source-level Sec main
```

Dead stripping must preserve the complete required chain even when ordinary Sec
code contains no call to `main`.

---

## 38. Initialization/shutdown retention

Every binary facility required by `ProgramInitializationPlan` or
`ProgramShutdownPlan` contributes either a link root or a mandatory dependency
from a root.

Target-specific section retention may implement these constraints, but section
spelling does not define why a facility must remain live.

---

## 39. Static storage reachability

Static storage is retained when required by reachable code/data, deterministic
shutdown/static-destruction requirements, target/platform retention, or a
declared binary artifact surface.

Static storage that is otherwise unreachable, has no required destruction, and
has no explicit platform/artifact retention requirement need not be retained
merely because its declaration exists.

Linker garbage collection must not eliminate storage still required by canonical
shutdown semantics.

---

## 40. Reachability edges

Canonical binary reachability edges include, as applicable:

```text
direct call
direct data reference
callable/function-address use
static-storage reference
generated ABI bridge dependency
startup dependency
shutdown/destruction dependency
runtime-helper dependency
platform-required dependency
```

Future target features may contribute additional compiler-known retention edges.

---

## 41. Foreign-import reachability

An unused foreign declaration does not by itself create a live foreign import
requirement.

A reachable foreign reference creates the corresponding symbol/provider
requirement.

---

## 42. Callback-thunk reachability

Compiler-generated callback thunks become reachable through actual retained
callback use.

Once a callback address participates in a live foreign binary contract, the
thunk must remain retained for that contract's required lifetime.

Potentially generatable callbacks are not global roots merely because the
compiler could synthesize them.

---

## 43. Platform retention requirements

Target/platform facilities that are not naturally reachable through ordinary
Sec calls may contribute explicit `PlatformRetentionRequirement` roots.

Examples may include reset vectors, interrupt vectors, fixed-address objects,
hardware tables, and platform descriptors.

The owning target/platform rulebook defines when retention is required.
`linking.md` defines how the resulting requirement participates in binary
reachability.

---

## 44. Dead stripping

Dead stripping is a permitted optimization. It is not a semantic requirement
that every build remove every unreachable private implementation entity.

Retaining unreachable private code or data is generally correct but potentially
less optimized. Removing a required root or reachable dependency is invalid.

---

## 45. External linker garbage collection

Target linker features such as section garbage collection are implementation
mechanisms.

The compiler materializes roots, retention constraints, section relationships,
and symbol visibility so concrete linker garbage collection remains compatible
with the canonical `LinkReachabilityGraph`.

Sec reachability is not defined as whatever the external linker happens to
retain.

---

## 46. Library reachability

When producing a reusable static or shared library, its declared
binary-consumable surface is part of the root set for that artifact.

The linker must not remove a library entry point merely because no code within
the library itself calls it. The future consuming artifact is an external
caller and is not part of the current internal call graph.

---

## 47. LTO boundary

Link-time optimization may change implementation boundaries.

LTO may inline, internalize, merge optimization units, remove unreachable
private code, propagate constants, and coalesce compatible definitions while
preserving artifact roots, required binary surfaces, ABI contracts, foreign
symbol requirements, initialization/shutdown requirements, platform retention,
and dynamic-link constraints.

LTO does not define a competing program-reachability model.

---

## 48. Object boundaries under LTO

LTO may merge physical object/LTO units without changing semantic identities.
Object boundaries may disappear while the compiler preserves, where still
required:

```text
SemanticDeclarationIdentity
BinarySymbolIdentity
ABISignature
foreign linkage identity
binary surface identity
```

---

## 49. Dynamic linkage and whole-program assumptions

Dynamic linkage may constrain whole-program optimization and dead stripping.
If the selected `LinkEnvironment` permits dynamic symbol resolution,
interposition, external binary consumers, or platform-provided lookup, the
optimizer may make only assumptions valid under that symbol model.

A required dynamically visible binary surface must not be removed merely because
the current artifact contains no internal call edge to it.

---

## 50. Artifact verification

After the concrete linker/toolchain produces a candidate output, the compiler
verifies the artifact against the canonical `LinkPlan`:

```text
Verified LinkPlan
        ↓
toolchain invocation
        ↓
candidate artifact
        ↓
VerifyArtifact
        ↓
accepted artifact
```

A successful external linker exit code is not by itself proof that the Sec
artifact contract is satisfied.

---

## 51. Artifact-verification requirements

Artifact verification checks target-relevant invariants available to the
compiler, including as applicable:

```text
correct OutputArtifactKind
correct architecture/target identity
required entry contract
mandatory symbol resolution
permitted unresolved dynamic imports
required roots and binary surface
startup/shutdown facilities
ABI compatibility
native dependency realization
absence of stale incompatible Sec artifacts
absence of target-incompatible surviving inputs
```

The verifier need not reimplement the entire system linker or fully disassemble
the binary. It must establish that the output conforms to the compiler's own
resolved link contract.

---

## 52. Link toolchain selection

The concrete linker or linker driver is resolved through the active
`LinkEnvironment`.

Conceptually, a `LinkToolchain` describes an implementation capable of
materializing the required `LinkPlan`.

The selected toolchain must provide all capabilities required by the plan, such
as applicable object-format support, relocation support, archive grouping or
rescanning, LTO integration, dynamic-link support, response-file support, and
platform startup/default integration.

---

## 53. Toolchain discovery and host environment

Toolchain discovery may use host configuration to locate a compatible
implementation. The host does not define target semantics.

Sec does not define target linkage as:

```text
find the first linker in PATH and accept its defaults
```

The compiler verifies compatibility between the selected toolchain and the
resolved `LinkEnvironment`.

---

## 54. Search paths

When native-link search paths are required, they are explicit `LinkPlan` inputs
with provenance.

Conceptually:

```text
SearchPath {
    Kind
    Path
    Origin
}
```

Possible origins include project configuration, package dependency, target SDK,
platform toolchain, and explicit user override.

Unspecified working-directory search or accidental host-global paths do not
silently become canonical dependency-resolution semantics.

---

## 55. Ambiguous dependency resolution

If more than one candidate can satisfy a logical dependency and no owning
dependency rule defines a canonical selection policy, resolution is ambiguous
and invalid.

Search-path order does not implicitly define Sec package/provider precedence.
Concrete linker search ordering materializes already-resolved intent rather than
defining semantic dependency selection.

---

## 56. Target default libraries and runtime inputs

A `LinkEnvironment` may define target default libraries, runtime objects,
startup objects, and equivalent platform facilities.

Such defaults remain visible as canonical `LinkPlan` requirements even when the
selected linker driver materializes them implicitly.

The semantic model must not depend on undocumented external-tool defaults.

---

## 57. Raw linker options

Build configuration may support explicit target/toolchain-specific raw linker
options as an escape hatch.

Such options are not Sec language semantics, are explicitly recorded,
participate in link reproducibility/cache identity, and may not silently
invalidate canonical `LinkPlan` invariants.

If a raw option attempts to contradict a required entry, symbol, ABI, output, or
retention contract, the compiler either rejects the conflict or verifies that
the final artifact remains canonically equivalent.

This rulebook defines no source syntax for raw linker options.

---

## 58. Response files

Response files and equivalent mechanisms are transport/serialization details of
a concrete tool invocation.

The same canonical `LinkPlan` may be materialized through direct argv, a
response file, or a toolchain-specific equivalent without changing semantics.
Temporary response-file paths do not become semantic link inputs.

---

## 59. Deterministic invocation materialization

Concrete linker inputs are materialized in a deterministic order derived from
the canonical `LinkPlan`.

The order does not depend on filesystem enumeration, map iteration, parallel
completion order, or source traversal order.

Toolchain-specific grouping, rescanning, or ordering required to implement
fixed-point archive resolution is generated by the linker materializer and does
not become Sec source semantics.

---

## 60. Controlled link execution environment

Link execution uses a controlled or explicitly modeled environment sufficient
to prevent unknown host state from silently changing the intended artifact
contract.

Relevant state may include:

```text
working directory
toolchain path
SDK/sysroot
search paths
environment variables
locale where diagnostic parsing is required
```

Sec does not require a full linker sandbox. It does require output-affecting
environment facts to be explicit, controlled, or proven irrelevant.

---

## 61. Reproducibility

For the same canonical linking inputs, Sec requires deterministic:

```text
LinkPlan
symbol-resolution result
reachability result
toolchain selection
concrete invocation model
artifact contract
diagnostics
```

Toolchains may insert non-semantic metadata such as timestamps or random build
identifiers.

Sec 0.1 does not require byte-identical binaries on every target when the
selected external toolchain cannot provide that guarantee. Where the selected
toolchain declares reproducible-output support, the compiler should use it for
canonical builds unless an explicit build mode requests otherwise.

---

## 62. Link cache identity

Any link cache or incremental reuse identity includes every concrete input that
can affect the artifact contract or physical output where relevant.

This includes, as applicable:

```text
LinkPlan
LinkEnvironment identity
toolchain identity/version
input artifact identities/content
resolved native dependencies
startup/runtime/shutdown inputs
explicit linker options
LTO mode
OutputArtifactKind
reproducibility settings
```

Detailed cache architecture belongs to `incremental_compilation.md`.

---

## 63. Diagnostics

External linker diagnostics are evidence used by the Sec compiler.

Where possible, the compiler maps toolchain errors back to canonical:

```text
BinarySymbolIdentity
ForeignSymbolRequirement
ResolvedNativeDependency
source declaration/use
dependency provenance
LinkRoot
```

Raw linker output may be retained as secondary detail. Users should not normally
need to decode mangled names or raw toolchain flags to understand the primary
Sec diagnostic.

---

## 64. Diagnostic classes

The compiler distinguishes at least:

### 64.1 ToolchainInvocationFailure

Examples include missing linker executable, process-start failure, linker crash,
or response-file creation failure.

### 64.2 LinkResolutionFailure

Examples include unresolved required symbols, duplicate native definitions,
ambiguous providers, unsupported archive-resolution cycles, or missing required
native dependencies.

### 64.3 ArtifactVerificationFailure

Examples include external-linker success with a missing required entry, wrong
target/output format, missing retained symbol, stale ABI artifact, or forbidden
unresolved import.

---

## 65. Inspectable link planning

The compiler should provide build/tooling diagnostics that can expose the
resolved linking model, including selected toolchain, resolved output kind/path,
resolved link inputs, native dependencies, search paths, startup/runtime/shutdown
inputs, effective explicit linker options, and LTO mode.

Concrete command-line spelling remains implementation-specific and is not Sec
language semantics.

---

## 66. Cross-compilation

Cross-compilation uses the target `CompilationPlan` and `LinkEnvironment`.

The compiler host must not silently supply host startup objects, host system
libraries, host object-format defaults, host native dependencies, or host linker
semantics unless those same inputs are explicitly part of the resolved target
`LinkEnvironment`.

A valid cross build proves that every concrete link input and toolchain facility
is compatible with the target plan.

---

## 67. Multi-Variant linking

Each selected Target Variant is linked independently.

For example:

```text
Target server
    Variant linux-amd64
    Variant linux-arm64
    Variant windows-amd64
```

produces three concrete `CompilationPlan`s, `LinkPlan`s, link operations, and
artifacts.

Common frontend or semantic work may be reused where canonical dependency
fingerprints permit it. Physical link state is never shared as one multi-target
link invocation.

---

## 68. Required test families

A conforming Sec 0.1 linking implementation includes regression coverage for at
least the following areas.

### 68.1 Symbol identity

Test deterministic native `BinarySymbolIdentity`, same short declaration names
in different modules, overload/specialization identity, import-alias
independence, object-format encoding independence, and foreign `@link_name`.

### 68.2 Duplicate/coalescible definitions

Test duplicate non-coalescible native rejection, duplicate input artifacts,
valid compiler-generated coalescing, ABI mismatch between coalescible candidates,
and absence of general source-level weak linkage.

### 68.3 Static archives

Test unused members, live member extraction, transitive/fixed-point extraction,
cyclic static-library dependencies when supported, rejection when unsupported,
and independence from source/manifest ordering.

### 68.4 Dynamic dependencies

Test shared-library provider resolution, import-library realization, framework
realization where supported, permitted dynamic unresolved imports, invalid
undeclared imports, provider ambiguity, and provider version metadata where
supported.

### 68.5 Reachability/dead stripping

Test executable entry retention, source `main` retention through generated entry
chains, startup/shutdown/runtime roots, removal of unused private helpers, unused
foreign declarations, live callback thunks, static data with required
destruction, unused trivial static data, and platform retention roots.

### 68.6 Library surface

Test retention of reusable library binary surface despite no internal callers,
removal of private implementation, non-equivalence of Sec public visibility and
binary export, and unresolved external references in static/object outputs where
allowed.

### 68.7 LTO

Test object-unit merging, inlining/internalization, preservation of ABI-visible
binary surfaces, foreign symbols, startup/shutdown roots, dynamic/interposition
requirements, and canonical reachability.

### 68.8 Toolchain materialization

Test toolchain selection through `LinkEnvironment`, incompatible toolchain
rejection, deterministic input ordering, response-file/direct-argv equivalence,
archive groups/rescans, search-path provenance, and absence of accidental host
or current-directory dependency resolution.

### 68.9 Defaults and raw options

Test target default libraries as canonical inputs, generated startup/runtime
default visibility, raw option fingerprinting, and rejection/proof of equivalent
conflicting entry/output/retention options.

### 68.10 Artifact verification

Test correct executable/static/shared/object kinds, target architecture/object
format, required entry, required roots, native dependencies, stale ABI artifact
rejection, and invalid unresolved imports after external-linker success where the
target contract requires rejection.

### 68.11 Cross compilation and determinism

Test target libraries/toolchain independently of compiler host, absence of host
startup/object defaults in cross builds, deterministic `LinkPlan` and
resolution, and independence from map/source/filesystem/worker ordering.

---

## 69. Completion criteria

The Sec 0.1 linking implementation is complete when every concrete linkable
`CompilationPlan` can deterministically:

1. resolve one authoritative `LinkEnvironment`;
2. build a canonical typed `LinkPlan`;
3. derive stable native `BinarySymbolIdentity` values;
4. preserve foreign linkage identities and dependency provenance;
5. resolve native and foreign symbol requirements according to the output artifact contract;
6. handle static archives through canonical fixed-point member selection;
7. represent shared/import/framework dependencies through the target dependency model;
8. detect duplicate, incompatible, ambiguous, or stale definitions/providers;
9. build a canonical link-root/reachability model;
10. preserve startup, shutdown, runtime, static-destruction, callback, and platform retention requirements;
11. permit dead stripping/LTO without changing required binary contracts;
12. select a compatible target toolchain;
13. materialize deterministic concrete linker invocation data;
14. control or explicitly model output-affecting search paths and environment;
15. execute the concrete toolchain;
16. map external linker diagnostics to Sec provenance where possible;
17. verify the produced artifact against the `LinkPlan`;
18. reject target-, ABI-, symbol-, dependency-, entry-, or retention-incompatible artifacts even when an external linker would otherwise accept them.

The implementation must achieve this without making correctness depend on
source declaration order, import order, library command-line order, filesystem
enumeration order, map iteration order, compiler worker scheduling, accidental
host linker defaults, accidental host search paths, first-definition-wins native
resolution, or external linker garbage collection as the definition of Sec
reachability.

---

## 70. Non-goals for Sec 0.1

This rulebook does not require:

- general source-level weak symbols;
- general user-selected public C ABI export of Sec functions;
- one object file per module;
- a universal object format;
- a universal linker implementation;
- byte-identical binaries on toolchains without reproducible-output support;
- source-level linker-command syntax;
- source-level linker search-path syntax;
- source-level library ordering semantics;
- eager inclusion of every static archive member;
- whole-program assumptions that violate the selected dynamic-linking model.
