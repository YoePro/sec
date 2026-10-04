# Projects, Manifest, Targets, Versions, Init and Builds

- **Status:** Normative
- **Created:** Legacy rulebook; exact original creation date not established
- **Last updated:** 2026-10-04
- **Document revision:** 2.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/projects/projects.md`
- **Replaces:** `rules/projects/projects.txt`
- **Repository baseline reviewed:** `main-reviewed-2026-10-04`
- **Implementation governance:** `governance/tooling_projects.yaml`
- **Related rulebooks:** `rules/projects/modules.md`, `rules/platform/platform_model.md`, `rules/platform/target_profiles.md`, `rules/foundations/attributes.md`, `rules/compiler/compiler_pipeline.md`, `rules/compiler/semantic_ir.md`, `rules/compiler/incremental_compilation.md`, `rules/compiler/compiler_testing.md`, `rules/compiler/linking.md`, `rules/declarations/static.md`, `rules/tooling/diagnostics.md`, `rules/tooling/lsp.md`, `rules/tooling/testing.md`, `rules/analysis/closure_analysis.md`

---

## § 1. Purpose and authority

§ 1(1) This rulebook defines the canonical Sec 0.1 user-project model.

§ 1(2) It owns:

- the project boundary;
- the single project manifest;
- project human-readable identity;
- project UUID identity;
- project versions and target version overrides;
- build targets;
- reusable build variants;
- project build defaults;
- compile-time project parameters;
- configuration precedence;
- build output layout;
- build-number storage and automatic increment behavior;
- project discovery;
- `sec init`;
- project-file validation and safe mutation.

§ 1(3) Canonical source-module identity, source-directory membership, imports, `ModuleName`, `ModuleIdentity`, `ModuleInstance`, `ModuleSurface`, import bindings, module graphs, import cycles and `internal` import access are owned by `rules/projects/modules.md`.

§ 1(4) This rulebook supplies the project root, project UUID, Targets, Variants, project configuration and concrete `CompilationPlan` inputs consumed by that module model.

§ 1(5) Platform capability resolution and target support are owned by the platform rulebooks.

§ 1(6) Compiler pipeline, linking, binary artifact composition, Semantic IR and target lowering are owned by their compiler rulebooks.

§ 1(7) Diagnostic identity, severity, localization and rendering are owned by `rules/tooling/diagnostics.md`.

§ 1(8) Mutable implementation status is owned by `governance/tooling_projects.yaml` and must not be maintained inside this normative rulebook.

---

## § 2. Design goals

§ 2(1) The project model is designed for:

- one clear project boundary;
- projects containing one or many independently buildable products;
- logical source organization without requiring manifest boilerplate for every module;
- deterministic project and module identity;
- reusable cross-platform build definitions;
- visible and manually editable version information;
- reproducible build selection;
- safe additive project initialization;
- transactional project-file changes;
- no hidden file replacement;
- no dependency on one prescribed application architecture.

§ 2(2) A project may be a small single-command program, a library, firmware, or a larger suite containing several commands, libraries, tools and other buildable products.

§ 2(3) Sec does not equate a project with one executable.

---

## § 3. Core terminology

### § 3.1 Project

§ 3.1(1) A **Project** is the complete source tree governed by one manifest:

```text
.sec/sec.toml
```

§ 3.1(2) A Project may contain several independently buildable products.

§ 3.1(3) Example:

```text
SEC Compiler Suite
    sec
    lsp
    support libraries
    compiler modules
    shared modules
```

§ 3.1(4) The Project is the ownership boundary for its manifest, project UUID, ordinary project-local import root, default version, Targets, Variants and project-wide configuration.

### § 3.2 ProjectName

§ 3.2(1) `ProjectName` is the human-readable name stored in `[project].name`.

§ 3.2(2) It is descriptive metadata.

§ 3.2(3) It is not:

- a Sec identifier;
- a source module name;
- a canonical import path;
- a Target name;
- the stable project identity.

§ 3.2(4) A name such as the following is valid:

```text
SEC Compiler Suite
```

§ 3.2(5) Renaming a Project does not create a new project identity.

### § 3.3 ProjectUUID

§ 3.3(1) `ProjectUUID` is the stable machine identity stored in `[project].uuid`.

§ 3.3(2) It is a UUID conforming to RFC 9562.

§ 3.3(3) Sec does not attach project semantics to a particular RFC 9562 UUID version beyond the requirement that the UUID is valid and non-nil.

§ 3.3(4) `sec init` generates a random UUIDv4.

§ 3.3(5) The manifest stores the canonical lowercase hyphenated textual representation:

```text
xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

§ 3.3(6) The UUID is generated once for a new Project and is then version-controlled as part of the manifest.

§ 3.3(7) Moving the Project directory, checking it out on another machine, building it in CI, or creating another checkout does not change the UUID.

§ 3.3(8) Deliberately replacing the UUID creates a new project identity for compiler identity, cache and module-import-root purposes.

### § 3.4 Target

§ 3.4(1) A **Target** is one independently buildable logical product within a Project.

§ 3.4(2) Examples include:

```text
sec
lsp
shop
worker
controller
protocol
```

§ 3.4(3) A Target is the project-model concept corresponding to a buildable subproduct or subproject.

§ 3.4(4) Sec 0.1 does not introduce a second nested Project manifest merely because a Project contains several Targets.

§ 3.4(5) A Target is not an operating-system or architecture selection.

### § 3.5 Variant

§ 3.5(1) A **Variant** is one reusable concrete platform/build configuration.

§ 3.5(2) Examples include:

```text
linux-amd64
macos-arm64
windows-amd64
cortex-m4
```

§ 3.5(3) A Target may be compiled for several Variants.

### § 3.6 Module

§ 3.6(1) A **Module** is a compiler-visible logical source grouping defined by the module rulebook.

§ 3.6(2) Ordinary directories may organize code into logical paths such as:

```text
compiler/lexer
compiler/parser
compiler/sema
lsp/protocol
lsp/workspace
inventory/orders
inventory/storage
```

§ 3.6(3) Modules are not required to be Targets.

§ 3.6(4) A Module is not a Project.

§ 3.6(5) A Module does not receive a manifest entry merely because it exists.

### § 3.7 Artifact

§ 3.7(1) An **Artifact** is an output produced for one concrete `CompilationPlan`.

§ 3.7(2) Examples include:

- executable;
- static library;
- shared library;
- firmware image;
- object file;
- MLIR output;
- LLVM IR output;
- debug-information output.

### § 3.8 ResolvedVersion

§ 3.8(1) `ResolvedVersion` is the version data used for one Target build after project defaults, target overrides and an explicit or automatic build number have been resolved.

§ 3.8(2) Conceptually:

```text
ResolvedVersion {
    Major: VersionNumber
    Minor: VersionNumber
    Revision: VersionNumber
    Build: VersionNumber
    UserDefined: optional string
    Format: string
    Rendered: string
}
```

§ 3.8(3) This is a logical tooling/compiler contract. It does not introduce a public Sec source-language struct by itself.

---

## § 4. Project boundary and single-manifest rule

§ 4(1) A Sec Project contains exactly one manifest:

```text
project/
    .sec/
        sec.toml
```

§ 4(2) The manifest governs the complete project source tree below the Project root.

§ 4(3) Nested Sec Projects are not supported in Sec 0.1.

§ 4(4) A second `.sec/sec.toml` below an already discovered Project root is an error.

§ 4(5) Applications, tools, libraries, firmware products and other independently buildable parts are represented as Targets in the root manifest.

§ 4(6) They do not receive nested manifests.

§ 4(7) Invalid:

```text
project/
    .sec/sec.toml

    compiler/
        .sec/sec.toml

    lsp/
        .sec/sec.toml
```

§ 4(8) The correct model is one Project with Targets such as `sec` and `lsp`.

§ 4(9) The single-manifest model provides one project-wide view for:

- import-root identity;
- dependency resolution when defined;
- Target and Variant selection;
- formatting;
- diagnostics;
- program analysis;
- build caching;
- platform selection;
- versioning;
- project tooling.

---

## § 5. Manifest location and format

§ 5(1) The canonical project manifest is:

```text
.sec/sec.toml
```

§ 5(2) It uses TOML syntax.

§ 5(3) Unknown top-level keys or tables are errors unless a separately specified extension mechanism owns them.

§ 5(4) Tools that modify a valid manifest must preserve comments, unrelated sections and user formatting when practical.

§ 5(5) A project tool must not deserialize and rewrite the complete manifest merely for convenience when doing so would destroy comments, ordering or unrelated user formatting.

§ 5(6) Syntax-aware edits or precise source patches are required for ordinary automatic manifest mutation.

---

## § 6. Canonical logical manifest model

§ 6(1) The project subsystem exposes the following logical model:

```text
ProjectManifest {
    Project: ProjectSection
    Version: ProjectVersion
    Build: optional BuildDefaults
    Analysis: optional AnalysisSettings
    Profiles: map<ProjectKey, ProfileDefinition>
    Targets: map<ProjectKey, TargetDefinition>
    Variants: map<ProjectKey, VariantDefinition>
    Parameters: map<string, CompileTimeParameterValue>
}
```

§ 6(2) Additional sections owned by another rulebook may extend the manifest.

§ 6(3) The logical model does not require one particular host-language struct layout.

§ 6(4) `ProjectKey` is used for machine-facing manifest names such as Target, Variant and Profile names.

§ 6(5) A `ProjectKey` matches:

```text
[a-z][a-z0-9_-]*
```

§ 6(6) `ProjectKey` comparison is case-sensitive.

§ 6(7) Human-facing `ProjectName` does not use the `ProjectKey` grammar.

---

## § 7. `[project]`

§ 7(1) Every manifest contains exactly one `[project]` table.

§ 7(2) Its canonical schema is:

```text
ProjectSection {
    name: required ProjectName
    uuid: required ProjectUUID
    module: optional ModuleName
}
```

§ 7(3) Example:

```toml
[project]
name = "SEC Compiler Suite"
uuid = "550e8400-e29b-41d4-a716-446655440000"
```

§ 7(4) `name` is a non-empty TOML string.

§ 7(5) Leading or trailing whitespace is part of the human-readable name and should normally be diagnosed as suspicious rather than silently trimmed.

§ 7(6) `uuid` must parse as a valid non-nil RFC 9562 UUID.

§ 7(7) The optional `module` field is only the expected `ModuleName` override for a source module formed directly by `.sec` files in the Project root.

§ 7(8) `[project].module` is not:

- the Project identity;
- a Target name;
- a module search path;
- a list of Modules;
- an import prefix.

§ 7(9) Ordinary nested Modules are discovered from source directories and imports and are not declared through `[project].module`.

---

## § 8. Project version model

§ 8(1) Every Sec 0.1 Project manifest contains exactly one `[version]` table.

§ 8(2) Its canonical schema is:

```text
ProjectVersion {
    major: required VersionNumber
    minor: required VersionNumber
    revision: required VersionNumber
    build: required VersionNumber
    userdefined: optional UserDefinedVersion
    format: required VersionFormat
}
```

§ 8(3) `VersionNumber` is a non-negative TOML integer in the range:

```text
0 .. 9223372036854775807
```

§ 8(4) `major`, `minor`, `revision` and `build` are independent stored values.

§ 8(5) `build` is visible, explicit project data. It is not authoritative hidden compiler state.

§ 8(6) Changing `major`, `minor` or `revision` does not automatically reset, increment or otherwise modify `build`.

§ 8(7) The programmer may manually set `build` to a new valid value when a release series, synchronization process or other project workflow requires it.

§ 8(8) Sec does not require semantic-versioning semantics for these fields.

§ 8(9) The names `major`, `minor`, `revision` and `build` describe Sec's structured project version components.

§ 8(10) Example:

```toml
[version]
major = 1
minor = 0
revision = 3
build = 123
userdefined = "alpha"
format = "[major].[minor].[revision]-[userdefined].[build]"
```

§ 8(11) The example resolves to:

```text
1.0.3-alpha.123
```

---

## § 9. User-defined version component

§ 9(1) `userdefined` is an optional project-defined version component.

§ 9(2) Its purpose is to carry project-specific version classification such as:

```text
alpha
beta
rc1
windows
nightly
```

§ 9(3) A non-empty `userdefined` value must match:

```text
[A-Za-z0-9][A-Za-z0-9._-]*
```

§ 9(4) The value is inserted into the rendered version exactly as stored.

§ 9(5) The version formatter does not automatically add, remove or normalize punctuation around `userdefined`.

§ 9(6) Example:

```toml
userdefined = "windows"
format = "[major].[minor].[revision]-[userdefined].[build]"
```

may render:

```text
1.0.3-windows.123
```

§ 9(7) Another Project may choose:

```toml
userdefined = "alpha"
format = "[major].[minor].[revision]-[userdefined]-[build]"
```

and render:

```text
1.0.3-alpha-123
```

---

## § 10. Version format

§ 10(1) `version.format` defines the rendered version string.

§ 10(2) Sec 0.1 defines exactly these placeholders:

```text
[major]
[minor]
[revision]
[build]
[userdefined]
```

§ 10(3) Numeric placeholders render as ordinary unsigned base-10 text without leading zero padding, except that zero renders as `0`.

§ 10(4) `[userdefined]` renders the resolved user-defined component verbatim.

§ 10(5) All other characters in the format string are literal.

§ 10(6) A bracketed ASCII identifier that is not one of the placeholders in § 10(2) is a manifest error.

§ 10(7) A format referencing `[userdefined]` is invalid when the resolved version has no user-defined component.

§ 10(8) Sec 0.1 defines no conditional punctuation, optional groups or formatter directives inside a version format.

§ 10(9) Therefore the formatter must not silently remove separators when an optional component is absent.

§ 10(10) A resolved version format must produce a non-empty string.

§ 10(11) The rendered version is metadata and is not automatically used as a filesystem path or artifact filename.

---

## § 11. Target version overrides

§ 11(1) A Target may inherit the complete Project version or override selected components.

§ 11(2) Target overrides are declared under:

```toml
[target.<name>.version]
```

§ 11(3) The logical schema is:

```text
TargetVersionOverride {
    major: optional VersionNumber
    minor: optional VersionNumber
    revision: optional VersionNumber
    build: optional VersionNumber
    userdefined: optional string
    format: optional VersionFormat
}
```

§ 11(4) Resolution begins with `[version]`.

§ 11(5) Every field explicitly present in `[target.<name>.version]` replaces the corresponding Project field.

§ 11(6) Every omitted field is inherited.

§ 11(7) A target may explicitly clear an inherited `userdefined` component using:

```toml
userdefined = ""
```

§ 11(8) An empty target `userdefined` means "no resolved user-defined component". It is not a rendered empty component.

§ 11(9) If clearing `userdefined` causes the inherited format to reference an unavailable `[userdefined]`, the Target configuration is invalid until the Target also supplies a compatible format.

§ 11(10) Example using the Project release numbers with an independent Target build series:

```toml
[target.sec.version]
build = 421
userdefined = "alpha"
```

§ 11(11) Example of a fully independent Target version:

```toml
[target.lsp.version]
major = 0
minor = 8
revision = 2
build = 97
userdefined = "beta"
format = "[major].[minor].[revision]-[userdefined].[build]"
```

§ 11(12) A Target's resolved version is a Target property and does not vary implicitly between its Variants.

§ 11(13) Operating system, architecture and Variant identity are separate build metadata.

§ 11(14) Sec never inserts the Variant or OS into a version string unless the programmer explicitly places equivalent text in `userdefined` or otherwise chooses a version format containing that text literally.

---

## § 12. Project version example for a suite

§ 12(1) A Project containing several independently buildable tools may use:

```toml
[project]
name = "SEC Compiler Suite"
uuid = "550e8400-e29b-41d4-a716-446655440000"

[version]
major = 1
minor = 0
revision = 3
build = 123
userdefined = "alpha"
format = "[major].[minor].[revision]-[userdefined].[build]"

[target.sec]
kind = "command"
source = "cmd/sec"
artifact = "sec"
variants = ["linux-amd64", "windows-amd64"]

[target.sec.version]
build = 421

[target.lsp]
kind = "command"
source = "cmd/lsp"
artifact = "sec-lsp"
variants = ["linux-amd64", "windows-amd64"]

[target.lsp.version]
major = 0
minor = 8
revision = 2
build = 97
userdefined = "beta"
```

§ 12(2) In this example:

```text
Project human name
    SEC Compiler Suite

Project UUID
    one stable identity for the complete source tree

Target sec
    independently buildable product

Target lsp
    independently buildable product

compiler/parser
lsp/protocol
etc.
    ordinary logical source Modules
```

---

## § 13. Source directories and logical Modules

§ 13(1) Module semantics are owned by `rules/projects/modules.md`.

§ 13(2) For project layout purposes, one source directory containing applicable `.sec` files forms one source Module.

§ 13(3) The complete logical location is the canonical import path.

§ 13(4) Intermediate organizational directories do not require special manifest declarations.

§ 13(5) Example:

```text
compiler/
    lexer/
        lexer.sec
    parser/
        parser.sec

lsp/
    protocol/
        protocol.sec
    workspace/
        workspace.sec
```

§ 13(6) The leaf source files may declare:

```sec
module lexer
module parser
module protocol
module workspace
```

§ 13(7) Their canonical project import paths remain:

```text
compiler/lexer
compiler/parser
lsp/protocol
lsp/workspace
```

§ 13(8) The compiler must never identify these Modules only by their short `ModuleName`.

§ 13(9) Project-local `ModuleIdentity` incorporates the Project import-root identity derived from the stable Project UUID.

§ 13(10) A Project may use arbitrary ordinary organizational paths such as:

```text
compiler
lsp
inventory
domain
application
services
features
components
adapters
infrastructure
```

§ 13(11) Such directory names receive no language semantics unless another normative rule explicitly reserves one.

---

## § 14. Project-root source files

§ 14(1) `.sec` files may exist directly in the Project root.

§ 14(2) Those files form one root source Module.

§ 14(3) For a root library Module, the expected `ModuleName` is:

1. `[project].module` when present;
2. otherwise `[project].name` only when the complete ProjectName is a valid Sec `ModuleName`.

§ 14(4) Example:

```toml
[project]
name = "codec"
uuid = "..."
module = "codec"
```

§ 14(5) Root source files then declare:

```sec
module codec
```

§ 14(6) A human-readable ProjectName such as:

```text
SEC Compiler Suite
```

is not required to be a valid ModuleName.

§ 14(7) When root source files form a library Module and the ProjectName is not a valid ModuleName, `[project].module` is required.

§ 14(8) A root minimal command Target may instead use:

```sec
module main
```

with Target source:

```toml
source = "."
```

§ 14(9) One root source directory cannot simultaneously be `module main` and a differently named root library Module.

---

## § 15. Reserved and recognized directories

### § 15.1 Reserved directories

§ 15.1(1) The following Project-root directory names are reserved:

```text
cmd
bin
.sec
```

### § 15.2 `cmd`

§ 15.2(1) `cmd` is the canonical source root for executable and firmware entry Targets.

§ 15.2(2) Example:

```text
cmd/
    sec/
        main.sec
    lsp/
        main.sec
```

§ 15.2(3) `.sec` files may not exist directly in `cmd/`.

§ 15.2(4) Each child entry directory is an ordinary source Module and normally declares:

```sec
module main
```

§ 15.2(5) Reusable implementation code should normally live in ordinary non-entry Modules.

### § 15.3 `bin`

§ 15.3(1) `bin` is reserved for generated build output.

§ 15.3(2) `bin` is never a project source Module.

§ 15.3(3) Imports from `bin/...` are invalid.

§ 15.3(4) Project `.sec` source files below `bin` are invalid project source.

§ 15.3(5) `sec clean` may remove compiler-owned outputs whose ownership is known.

§ 15.3(6) The compiler must not delete unknown user files merely because they are below `bin`.

### § 15.4 `.sec`

§ 15.4(1) `.sec` stores the manifest and compiler/tooling project state.

§ 15.4(2) Source Modules must not be declared below `.sec`.

### § 15.5 `internal`

§ 15.5(1) `internal` is recognized as an access-control path marker.

§ 15.5(2) It is not a general code-organization mechanism.

§ 15.5(3) It is optional.

§ 15.5(4) Project authors may organize logical code directly under paths such as `compiler/`, `lsp/`, `inventory/` or other ordinary directories instead of placing everything below `internal/`.

§ 15.5(5) Whenever `internal` occurs, its import-access semantics are mandatory and are defined by `rules/projects/modules.md`.

§ 15.5(6) `.sec` files may not exist directly in an `internal` marker directory.

§ 15.5(7) The path segment remains part of the canonical import path but does not become the short source `ModuleName`.

---

## § 16. Standard library and platform roots

§ 16(1) Standard-library and compiler platform resolution is not ordinary project-local shadowing.

§ 16(2) Standard-library Modules use their canonical logical paths directly, for example:

```sec
import "io"
import "net/http"
```

§ 16(3) The compiler-reserved platform import root is:

```text
platform
```

§ 16(4) Example:

```sec
import "platform/linux/amd64"
```

§ 16(5) A project-local Module must not silently replace a canonical standard-library Module or compiler-reserved platform Module with the same import path.

§ 16(6) Detailed import-root precedence and `ModuleIdentity` semantics are owned by `rules/projects/modules.md`.

---

## § 17. Targets

§ 17(1) A Target is declared under:

```toml
[target.<name>]
```

§ 17(2) `<name>` is a `ProjectKey`.

§ 17(3) Initial Target kinds are:

```text
command
library
firmware
test
```

§ 17(4) The logical Target schema is:

```text
TargetDefinition {
    kind: required TargetKind
    source: required ProjectRelativeModulePath
    artifact: optional string
    variants: required non-empty list<ProjectKey>
    version: optional TargetVersionOverride
    parameters: optional map<string, CompileTimeParameterValue>
    build_counter: optional BuildCounterDefinition
    variant_overrides: optional map<ProjectKey, TargetVariantOverride>
}
```

§ 17(5) `source` identifies the Target's entry Module directory relative to the Project root.

§ 17(6) The complete source graph is derived by resolving imports.

§ 17(7) The manifest must not contain a manually maintained list of every Module used by a Target.

§ 17(8) `artifact` is the artifact base name.

§ 17(9) When omitted, the Target name is the default artifact base name.

§ 17(10) A Target owns logical product identity, entry Module, Target kind, artifact base name, selected Variants, optional version overrides, Target-specific settings and build-number behavior.

§ 17(11) The Target source and Target kind do not vary per Variant.

§ 17(12) If two outputs require different entry Modules or are semantically different products, they are different Targets.

---

## § 18. Test Target boundary

§ 18(1) Ordinary language test declarations and `sec test` do not require a manifest Target of kind `test`.

§ 18(2) `sec test` creates a `TestCompilationPlan` according to the testing rulebooks.

§ 18(3) The manifest `test` Target kind denotes an explicitly buildable test-oriented product or artifact.

§ 18(4) A test Target does not grant friend visibility or change ordinary Module access rules by itself.

---

## § 19. Variants

§ 19(1) Reusable Variants are declared under:

```toml
[variant.<name>]
```

§ 19(2) `<name>` is a `ProjectKey`.

§ 19(3) Example:

```toml
[variant.linux-amd64]
os = "linux"
arch = "amd64"
abi = "gnu"

[variant.macos-arm64]
os = "macos"
arch = "arm64"

[variant.windows-amd64]
os = "windows"
arch = "amd64"
abi = "msvc"
```

§ 19(4) Canonical operating-system names include:

```text
linux
windows
macos
freebsd
netbsd
baremetal
freertos
```

§ 19(5) The source-level operating-system name is `macos`, not `darwin`.

§ 19(6) Canonical architecture names initially include:

```text
amd64
arm64
arm32
riscv32
```

§ 19(7) A Variant may additionally declare fields owned by the target/profile rules, including:

```text
cpu
features
abi
toolchain
profile
```

§ 19(8) Project manifests express toolchain intent.

§ 19(9) Machine-specific SDK paths, compiler installation paths and local toolchain paths belong in user or CI configuration rather than the portable Project manifest.

§ 19(10) Variant-specific project settings do not create a different ProjectUUID or Target identity.

---

## § 20. Build command behavior

§ 20(1) The canonical Target build command is:

```text
sec build <target>
```

§ 20(2) It builds every Variant listed by the Target unless command-line filtering narrows the set.

§ 20(3) Example:

```text
sec build sec
```

§ 20(4) Filters may include:

```text
sec build sec --variant linux-amd64
sec build sec --os linux
sec build sec --arch amd64
sec build sec --profile debug
```

§ 20(5) Filters select configured Variants.

§ 20(6) They do not silently create undeclared Variants.

§ 20(7) By default, the tool attempts every selected Variant and reports an aggregate result.

§ 20(8) `--fail-fast` may stop after the first failed Variant.

§ 20(9) The command exits unsuccessfully if any requested Variant fails.

---

## § 21. CompilationPlan integration

§ 21(1) Every selected Target-and-Variant pair produces one concrete `CompilationPlan`.

§ 21(2) Conceptually, Project resolution contributes at least:

```text
CompilationPlan {
    ProjectUUID
    ProjectName
    Target
    Variant
    ResolvedVersion
    SourceGraph
    OS
    Architecture
    ABI
    CPU
    CPUFeatures
    Profile
    CompilerOptions
    CompileTimeParameters
    OutputDirectory
    LinkConfiguration
}
```

§ 21(3) This list defines project-model inputs and does not replace the complete `CompilationPlan` contract owned by the compiler/platform rulebooks.

§ 21(4) The compiler may reuse target-independent work only when reuse is sound for the complete plan-sensitive dependency model.

§ 21(5) Variant-specific source selection, layout and lowering occur before architecture-specific assumptions may be committed.

§ 21(6) A multi-Variant build is not one polymorphic CompilationPlan.

§ 21(7) Diagnostics for a plan-specific failure should identify the affected Target and Variant.

---

## § 22. Profiles and compiler options

§ 22(1) Compiler options configure compilation behavior.

§ 22(2) They are distinct from program compile-time parameters.

§ 22(3) Project defaults may be declared under `[build]`.

Example:

```toml
[build]
backend = "mlir"
profile = "release"
warnings = "default"
bounds_checks = true
overflow_checks = true
```

§ 22(4) Profiles are reusable named compiler-option groups.

Example:

```toml
[profile.release]
optimization = "speed"
debug = "line"
strip = true

[profile.debug]
optimization = "none"
debug = "full"
strip = false
```

§ 22(5) Profile names are `ProjectKey` values.

§ 22(6) The exact supported compiler-option set is compiler-versioned and owned by the corresponding compiler rulebooks.

§ 22(7) Unknown compiler options are errors unless an explicit extension mechanism defines them.

---

## § 23. Project analysis and tooling settings

§ 23(1) Project-wide analysis and tooling resource settings may be declared in their owning manifest sections.

§ 23(2) The currently defined closure/LSP budget setting remains:

```toml
[analysis]
lsp_depth = "interactive"
```

§ 23(3) Supported values and reload behavior are owned by `rules/analysis/closure_analysis.md` and `rules/tooling/lsp.md`.

§ 23(4) Resource and precision settings must not disable required language safety analysis.

---

## § 24. Compile-time parameters

§ 24(1) Compile-time parameters are typed program configuration values supplied by project/build configuration.

§ 24(2) They are not compiler options.

§ 24(3) They are not textual preprocessor macros.

§ 24(4) The Project declaration is:

```toml
[parameters]
telemetry = false
maximum_connections = 1000
product_name = "Shop"
```

§ 24(5) Sec 0.1 project parameters may use these TOML scalar kinds:

```text
bool
integer
float
string
```

§ 24(6) Arrays, inline tables, date/time values and nested parameter tables are not Sec 0.1 compile-time parameter values unless a later rule extends the model.

§ 24(7) The first declaration of a parameter under project `[parameters]` establishes its configuration type.

§ 24(8) Target-specific overrides use:

```toml
[target.shop.parameters]
maximum_connections = 2000
```

§ 24(9) Variant-specific overrides use:

```toml
[variant.windows-amd64.parameters]
native_service = true
```

§ 24(10) Target-and-Variant-specific overrides use:

```toml
[target.shop.variant.windows-amd64.parameters]
service_name = "ShopService"
```

§ 24(11) Every override must preserve the original parameter type.

§ 24(12) An override must not introduce a parameter that has no project-level declaration unless a later rule explicitly permits scoped parameter declarations.

§ 24(13) Compile-time parameters must not contain secrets.

§ 24(14) They may appear in generated machine code, IR, object files, debug information or caches.

§ 24(15) Passwords, private keys and authentication tokens belong in runtime or protected external configuration.

§ 24(16) Exact Sec source syntax for reading compile-time parameters is owned separately.

---

## § 25. Configuration precedence

§ 25(1) Settings are merged from least specific to most specific.

§ 25(2) The canonical ordinary build-setting precedence is:

```text
1. compiler defaults
2. project [build] settings
3. selected profile
4. reusable Variant settings
5. Target settings
6. Target-specific Variant settings
7. command-line overrides
```

§ 25(3) A more specific value replaces a less specific value only when that option's type and owning semantics permit replacement.

§ 25(4) Command-line overrides affect only the current invocation unless a command explicitly performs a manifest-editing operation.

§ 25(5) Ordinary `sec build` overrides do not rewrite the manifest.

§ 25(6) Version resolution uses the separate rules in §§ 8–11 and build-number rules in §§ 27–30 rather than generic compiler-option precedence.

---

## § 26. Build output

§ 26(1) Default output layout is:

```text
bin/<target>/<variant>/<profile>/<artifact>
```

§ 26(2) Example:

```text
bin/
    sec/
        linux-amd64/
            release/
                sec
        windows-amd64/
            release/
                sec.exe
```

§ 26(3) Platform-appropriate filename extensions are applied by target tooling.

§ 26(4) Outputs remain separated by Target, Variant and Profile.

§ 26(5) One Variant must not overwrite another Variant's output.

§ 26(6) Artifact naming must not use ProjectName as an implicit filesystem path component.

---

## § 27. Build component and manual control

§ 27(1) `build` is the fourth canonical numeric version component.

§ 27(2) The authoritative stored Project build value is:

```toml
[version]
build = <number>
```

§ 27(3) A Target that uses an independent Target build sequence stores it explicitly as:

```toml
[target.<name>.version]
build = <number>
```

§ 27(4) There is no authoritative hidden build counter in `.sec/state/build-counters.toml`.

§ 27(5) A programmer may edit an authoritative build value manually.

§ 27(6) Manual edits are not considered corruption merely because the value moves backward, jumps forward or is reset.

§ 27(7) Such edits deliberately redefine the next automatic-counter baseline.

§ 27(8) Tooling must therefore read the current manifest value before allocating an automatic candidate.

§ 27(9) The compiler must not infer a reset merely from a changed `major`, `minor` or `revision`.

---

## § 28. Automatic build counter

§ 28(1) A Target may enable automatic build-number advancement.

§ 28(2) Existing Sec 0.1 syntax is retained:

```toml
[target.shop.build_counter]
enabled = true
scope = "target"
```

§ 28(3) Supported scopes are exactly:

```text
target
project
```

§ 28(4) `target` is the default when `scope` is omitted.

### § 28.1 Target scope

§ 28.1(1) `scope = "target"` uses the Target's explicit local version build field as the authoritative stored counter.

§ 28.1(2) Therefore this configuration requires:

```toml
[target.shop.version]
build = 142
```

§ 28.1(3) Inheriting `[version].build` is not sufficient for a Target-scoped automatic counter because the manifest would otherwise contain no visible Target-local counter to adjust manually.

### § 28.2 Project scope

§ 28.2(1) `scope = "project"` uses `[version].build` as the authoritative shared counter.

§ 28.2(2) A Target configured with project-scoped automatic build numbering must not also declare a conflicting local `target.<name>.version.build`.

§ 28.2(3) A project-scoped build candidate becomes the resolved Target build component for that invocation.

§ 28.2(4) Several Targets may deliberately share one project build sequence.

---

## § 29. Build-number allocation and commit

§ 29(1) For an automatic counter, the candidate number is:

```text
stored build + 1
```

§ 29(2) Overflow beyond the maximum `VersionNumber` is a build-configuration error.

§ 29(3) One logical Target build allocates at most one candidate number.

§ 29(4) If one command builds several Variants of the same Target, every selected Variant uses the same candidate.

§ 29(5) A filtered build is still a logical build and may consume a build number on success.

§ 29(6) The authoritative manifest build value is updated only after every requested Variant succeeds.

§ 29(7) If any requested Variant fails:

- the candidate is not committed;
- the manifest remains unchanged;
- the next build attempt may reuse the same candidate.

§ 29(8) Automatic build-number mutation must be serialized for the affected authoritative counter.

§ 29(9) Compiler-owned lock files or temporary transactional metadata may exist under `.sec/state/`, but they are not authoritative counter storage.

§ 29(10) The implementation must:

1. determine the authoritative manifest build field;
2. acquire the corresponding project/Target build-counter lock;
3. parse the current manifest and read the committed value;
4. reserve `value + 1` for the invocation;
5. resolve the complete Target version using that candidate;
6. build every requested Variant;
7. verify that the relevant manifest version field has not been externally changed since reservation;
8. on complete success, update only the authoritative build field using a syntax-aware atomic manifest edit;
9. on failure or concurrent manifest change, leave the manifest value unchanged;
10. release the lock.

§ 29(11) Automatic build-number editing must preserve manifest comments and unrelated formatting.

§ 29(12) If the manifest commit cannot be completed safely after successful compilation, the build invocation fails and must not present the candidate as a committed build number.

---

## § 30. Explicit build numbers

§ 30(1) CI or release tooling may provide an explicit build component:

```text
sec build shop --build-number 2841
```

§ 30(2) An explicit build number:

- must satisfy `VersionNumber`;
- replaces the resolved `Build` component for the current invocation;
- is used by every selected Variant of the Target;
- does not advance or rewrite the Project or Target manifest build value;
- participates in rendered version resolution;
- participates in artifact/cache identity wherever the resolved version affects output.

§ 30(3) An explicit build number takes precedence over automatic counter allocation for that invocation.

§ 30(4) The command must not reserve or commit an automatic build number when `--build-number` is supplied.

§ 30(5) An explicit number may therefore reproduce an earlier build identity without mutating the manifest.

---

## § 31. Rendered version during a build

§ 31(1) Version formatting occurs after the final build component has been selected.

§ 31(2) Therefore an automatic candidate or explicit CLI build number is visible through `[build]` substitution in the rendered version for that invocation.

§ 31(3) Example manifest:

```toml
[version]
major = 1
minor = 0
revision = 3
build = 122
userdefined = "alpha"
format = "[major].[minor].[revision]-[userdefined].[build]"
```

§ 31(4) An automatic successful candidate `123` renders:

```text
1.0.3-alpha.123
```

§ 31(5) The stored manifest is then updated to:

```toml
build = 123
```

§ 31(6) A failed build renders its candidate internally for compilation but does not commit that candidate to project state.

---

## § 32. Build metadata

§ 32(1) The compiler/toolchain should expose typed compile-time build metadata through the separately defined compiler-known/core build-metadata API.

§ 32(2) Required conceptual facts include:

```text
ProjectUUID
ProjectName
TargetName
VariantName
VersionMajor
VersionMinor
VersionRevision
VersionBuild
VersionUserDefined when present
VersionRendered
OperatingSystem
Architecture
Profile
SourceRevision when available
DirtySourceStatus when available
```

§ 32(3) These facts are structured values, not textual macro substitution.

§ 32(4) The exact Sec source declarations are owned by the rulebook that defines the build-metadata API.

§ 32(5) Project version and build are no longer separate unrelated stores: `build` is an explicit version component, while still remaining separately addressable as structured metadata.

§ 32(6) Current time is not embedded by default.

§ 32(7) Build timestamps require explicit separately defined behavior because ambient time reduces reproducibility.

---

## § 33. Reproducible builds

§ 33(1) A reproducible build must resolve the same relevant `CompilationPlan` inputs.

§ 33(2) A command requesting reproducibility must not silently allocate a new automatic build number.

§ 33(3) `--build-number` may be used to provide the original build component without rewriting the manifest.

§ 33(4) Source revision and dirty-source status are metadata only when available and requested/defined by the owning build-metadata contract.

§ 33(5) No compiler-host timestamp is an implicit reproducibility input.

---

## § 34. Build and cache identity

§ 34(1) Build caches must include every input that may affect semantic or generated output.

§ 34(2) Relevant project inputs include at least:

- compiler/toolchain compatibility identity;
- ProjectUUID;
- Target identity;
- Variant identity;
- resolved platform/ABI/CPU features;
- selected Profile;
- compiler options;
- compile-time parameters;
- resolved version fields when observable in generated output or build metadata;
- source hashes;
- dependency hashes;
- selected platform-module hashes;
- target directives;
- relevant `CompilationPlan` capability state.

§ 34(3) ProjectName is not a substitute for ProjectUUID in semantic project identity.

§ 34(4) A Project rename must not by itself turn every Module into a different logical Module.

§ 34(5) Changing ProjectUUID intentionally invalidates project-identity-dependent cache/module identities.

§ 34(6) Cache reuse remains governed by the incremental-compilation rulebook. A cache is never canonical truth.

---

## § 35. `sec init` purpose

§ 35(1) `sec init` creates or extends a Sec Project safely.

§ 35(2) It must be:

- additive;
- idempotent;
- conflict-aware;
- transactional;
- non-destructive.

§ 35(3) It must never overwrite an existing conflicting user file.

§ 35(4) It must never replace an existing valid manifest wholesale.

§ 35(5) It must never create a nested `.sec/sec.toml` inside an existing Project.

---

## § 36. Project discovery

§ 36(1) Project discovery searches the current directory and then parent directories for:

```text
.sec/sec.toml
```

§ 36(2) The nearest enclosing manifest defines the active Project.

§ 36(3) Discovery stops at filesystem/project boundaries defined by the tooling implementation only after no valid enclosing Project can exist.

§ 36(4) If no manifest exists, `sec init` may create a Project in the selected directory.

§ 36(5) If a manifest exists, `sec init` extends or validates that Project.

§ 36(6) It does not create another Project below it.

---

## § 37. New-project `sec init`

§ 37(1) When creating a new Project, `sec init` must create a complete valid `[project]` and `[version]` model.

§ 37(2) Unless the command receives explicit creation options, initial values are:

```toml
[project]
name = "<final project-directory name>"
uuid = "<new RFC 9562 UUIDv4>"

[version]
major = 0
minor = 1
revision = 0
build = 0
format = "[major].[minor].[revision].[build]"
```

§ 37(3) `userdefined` is omitted by default.

§ 37(4) A user may subsequently edit the human Project name without changing UUID.

§ 37(5) A Project creation interface may accept an explicit human-readable name, but it must still generate a UUID unless an explicitly supported import/migration operation provides one.

§ 37(6) The default application profile may create:

```text
.sec/sec.toml
cmd/<target>/main.sec
bin/
internal/
```

§ 37(7) `internal/` is only a recommended starting structure and is not required for ordinary Project organization.

§ 37(8) A minimal profile may create:

```text
.sec/sec.toml
main.sec
```

with a root command Target.

---

## § 38. `sec init module`

§ 38(1) A normal Module may be added with:

```text
sec init module inventory/orders
```

§ 38(2) This creates an appropriate source directory and starter source file without adding a manifest Target.

§ 38(3) Example output:

```text
inventory/orders/orders.sec
```

with:

```sec
module orders
```

§ 38(4) Intermediate directories are ordinary logical organization.

§ 38(5) An internal Module may be created through a path containing the recognized marker:

```text
sec init module inventory/internal/storage
```

§ 38(6) `sec init module` must apply module/path validation from `rules/projects/modules.md`.

---

## § 39. `sec init` Target creation

§ 39(1) A command Target may be added with:

```text
sec init command worker
```

§ 39(2) It creates an entry Module such as:

```text
cmd/worker/main.sec
```

§ 39(3) The starter file declares:

```sec
module main

fn main() int {
    return 0
}
```

§ 39(4) The manifest is extended additively with a compatible `[target.worker]` entry.

§ 39(5) Initial Target creation forms may include:

```text
sec init command <name>
sec init library <name>
sec init firmware <name>
sec init test <name>
```

§ 39(6) A creation command must not invent a second Project identity.

§ 39(7) A new Target inherits the Project version until a Target version override is explicitly introduced.

§ 39(8) Target-specific automatic build numbering is not enabled implicitly.

---

## § 40. Idempotence

§ 40(1) Repeating the same compatible `sec init` operation succeeds as a no-op.

§ 40(2) It must not duplicate:

- manifest tables;
- Target declarations;
- files;
- directories;
- version fields;
- UUIDs.

§ 40(3) Re-running `sec init` on an existing Project never regenerates its UUID.

---

## § 41. Existing manifest preservation

§ 41(1) When `.sec/sec.toml` already exists, `sec init` must:

- parse and validate it;
- preserve ProjectUUID;
- preserve ProjectName unless an explicit rename operation was requested;
- preserve version values unless an explicit version/build operation requested a change;
- preserve comments;
- preserve unknown but permitted extension sections;
- preserve user ordering and formatting when practical;
- add only requested missing sections or keys;
- reject incompatible existing declarations.

§ 41(2) It must not deserialize and rewrite the complete manifest in a way that destroys comments or formatting.

§ 41(3) Automatic build-number commits are also subject to the same syntax-aware preservation requirement.

---

## § 42. Existing files and conflicts

§ 42(1) `sec init` applies these rules:

```text
missing file
    create it

identical generated file
    no-op

compatible existing user file
    preserve it

conflicting existing file
    error and change nothing
```

§ 42(2) A conflict in any planned operation aborts the complete init transaction.

---

## § 43. Transactional init behavior

§ 43(1) Before applying an init operation, tooling must:

1. discover the Project;
2. read and validate the manifest;
3. inspect relevant existing files;
4. calculate the complete change plan;
5. detect every known conflict.

§ 43(2) If any planned operation conflicts, no operation is applied.

§ 43(3) The implementation must not create early files or directories and later discover an avoidable conflict that leaves a partial init operation.

§ 43(4) Where the host filesystem cannot provide one atomic transaction across all paths, the tool must use staging/rollback or another strategy that preserves the all-or-nothing programmer-visible contract as far as the host permits.

---

## § 44. Dry run

§ 44(1) `sec init` supports:

```text
sec init --dry-run
```

and corresponding subcommand dry-run behavior.

§ 44(2) Dry run performs discovery, parsing, validation and change planning but does not modify files.

§ 44(3) Dry-run output should identify planned creates, manifest edits and conflicts.

---

## § 45. Project-root invocation

§ 45(1) `sec init` may be invoked from any directory inside a Project.

§ 45(2) It operates on the single manifest found by upward Project discovery.

§ 45(3) It never creates:

```text
subdirectory/.sec/sec.toml
```

inside an existing Project.

---

## § 46. Dependency and linking boundary

§ 46(1) Native libraries, objects, import libraries, frameworks and search paths are project/package/build metadata resolved into the active `CompilationPlan`.

§ 46(2) FFI declarations carry symbol and ABI semantics and must not duplicate linker dependency metadata through ad-hoc per-function project configuration.

§ 46(3) Dependency acquisition/version-solving semantics that are not yet normatively defined are outside this revision.

§ 46(4) This rulebook must not be interpreted as permission for an implementation to invent package-manager semantics.

---

## § 47. Architecture recommendations

§ 47(1) Sec does not require one application architecture.

§ 47(2) The compiler must not infer business meaning merely from ordinary directory names.

§ 47(3) Organize Modules by responsibility and reason for change.

§ 47(4) A Project may choose structures such as:

```text
compiler/
    lexer/
    parser/
    sema/

lsp/
    protocol/
    workspace/
```

or:

```text
inventory/
    orders/
    stock/
    storage/
```

§ 47(5) `internal` should be introduced where the import-access boundary is desired, not as a mandatory container for all implementation code.

§ 47(6) Directory names such as `utils`, `helpers`, `common`, `misc` or `shared` may receive mentor advice when they accumulate unrelated responsibilities.

§ 47(7) Such organization advice is not a language error.

---

## § 48. Project validation

§ 48(1) Project/compiler tooling validates at least:

- exactly one Project manifest governs the Project;
- no nested Project manifest exists;
- `[project]` exists exactly once;
- ProjectName is non-empty;
- ProjectUUID is a valid non-nil RFC 9562 UUID;
- `[version]` exists exactly once;
- all required numeric version fields are valid `VersionNumber`;
- version placeholders are known;
- `[userdefined]` references resolve;
- Target version inheritance resolves completely;
- automatic Target counters have explicit Target-local build storage;
- project-scoped counters do not conflict with Target-local build overrides;
- every source directory obeys module naming/membership rules;
- root-module declarations match the root project-module rules;
- reserved directories are used correctly;
- Target names are unique;
- Variant names are unique;
- Profile names are unique;
- Target source paths exist and form valid entry Modules;
- each Target has at least one configured Variant;
- every referenced Variant exists;
- Target and Variant settings are compatible;
- compiler options are known and valid;
- compile-time parameter override types match;
- output paths do not collide;
- automatic build-number mutation can identify exactly one authoritative build field;
- init operations are conflict-free before application.

§ 48(2) Project diagnostics should identify:

- the violated project rule;
- the relevant manifest key or path;
- the invalid value;
- the expected form;
- a safe correction when one is known.

§ 48(3) Stable diagnostic IDs are allocated and transported according to `rules/tooling/diagnostics.md`.

---

## § 49. Required implementation model

§ 49(1) The implementation must represent Project identity independently from display name and filesystem path.

§ 49(2) At minimum, the resolved project layer must make these facts available without reparsing ad-hoc strings:

```text
ProjectUUID
ProjectName
ProjectRoot
ProjectVersion
Targets
Variants
Profiles
CompileTimeParameters
```

§ 49(3) Version formatting must operate on parsed structured version fields, not by parsing a precomposed version string back into components.

§ 49(4) Module resolution must consume ProjectUUID/import-root identity rather than ProjectName.

§ 49(5) Build-number mutation must use the parsed manifest syntax tree or an equivalently precise editing representation.

§ 49(6) Tooling must not use a hidden mutable counter as the authoritative value after revision 2.0.

---

## § 50. Required conformance tests

§ 50(1) Project tests must cover at least:

- creation of a new Project with human ProjectName and RFC 9562 UUID;
- UUID stability across repeated `sec init`;
- Project rename without UUID change;
- rejection of nil/invalid UUID;
- rejection of nested manifests;
- multi-Target suite Projects;
- ordinary Modules grouped below arbitrary logical directories;
- `internal` access-control behavior remaining separate from ordinary organization;
- project version parsing;
- Target version inheritance;
- Target numeric override;
- Target `userdefined` override;
- Target `userdefined = ""` clear behavior;
- unknown version placeholder rejection;
- missing `[userdefined]` rejection when format requires it;
- exact rendering of major/minor/revision/build;
- manual build reset/jump acceptance;
- no automatic build reset after major/minor/revision changes;
- Target-scoped counter requiring a Target-local build field;
- project-scoped counter using `[version].build`;
- failed automatic build leaving manifest unchanged;
- successful automatic build committing exactly one increment;
- multi-Variant Target build using one candidate number;
- parallel counter serialization;
- explicit `--build-number` leaving manifest unchanged;
- comment-preserving manifest build update;
- init dry run;
- init conflict rollback;
- no-op idempotent init;
- stable cache/module identity under Project directory moves.

§ 50(2) Tests that validate ModuleIdentity/import behavior should be shared with or reference the module conformance suite rather than duplicate a second independent module model.

---

## § 51. Governance boundary

§ 51(1) The primary implementation-governance owner for this rulebook is:

```text
governance/tooling_projects.yaml
```

§ 51(2) That fragment owns implementation tracking for:

- Project discovery;
- manifest parsing and validation;
- ProjectName and ProjectUUID;
- Project version parsing and rendering;
- Target version overrides;
- Target/Variant project configuration;
- build-number storage and mutation;
- `sec init`;
- project-file editing and preservation.

§ 51(3) Module semantics referenced by this book remain governed by the module/declaration governance owner rather than being duplicated into project tooling status.

§ 51(4) `CompilationPlan`, linking and compiler-pipeline implementation status remain governed by compiler/platform fragments.

§ 51(5) Diagnostic definition/output implementation status remains governed by `governance/errors_diagnostics.yaml`.

§ 51(6) LSP-specific project reload/editor behavior remains governed by `governance/tooling_lsp.yaml`.

§ 51(7) A governance audit for revision 2.0 must therefore synchronize affected integrations rather than copying their implementation status into this rulebook.

---

## § 52. Compatibility and superseded revision-1 rules

§ 52(1) Revision 2.0 preserves the existing:

- single-manifest model;
- Target model;
- Variant model;
- one-directory/one-Module project-layout input;
- `internal` access-control marker;
- `cmd`, `bin` and `.sec` reserved directories;
- `CompilationPlan` per Target-and-Variant model;
- transactional `sec init` model.

§ 52(2) Revision 2.0 changes the project-version/build model.

§ 52(3) The revision-1 hidden authoritative build-counter file:

```text
.sec/state/build-counters.toml
```

is superseded.

§ 52(4) Compiler-owned state below `.sec/state/` may still contain locks, transaction journals or other non-authoritative tooling state.

§ 52(5) Authoritative build values are now the explicit manifest `build` fields defined by this rulebook.

§ 52(6) Revision-1 wording that treats project version as an unspecified independent string is superseded by the structured `[version]` model.

§ 52(7) Revision-1 wording that might suggest `internal` is the preferred general organizational root is narrowed: `internal` is an access-control marker, while ordinary directories provide general logical organization.

---

## § 53. Normative summary

§ 53(1) A Sec Project is one complete source tree governed by exactly one `.sec/sec.toml`.

§ 53(2) A Project may contain many independently buildable Targets.

§ 53(3) Nested Project manifests are not supported in Sec 0.1.

§ 53(4) `[project].name` is human-readable and is not stable semantic identity.

§ 53(5) `[project].uuid` is the stable RFC 9562 project identity.

§ 53(6) `sec init` generates a UUIDv4 once and never regenerates it for an existing Project.

§ 53(7) Ordinary logical source organization uses Modules and canonical import paths; `internal` is only an access-control marker.

§ 53(8) `[project].module` is only the optional root source ModuleName override.

§ 53(9) Every Project has structured `major`, `minor`, `revision` and `build` version components.

§ 53(10) `userdefined` is optional and `format` explicitly defines rendered version spelling.

§ 53(11) Version formatting recognizes only `[major]`, `[minor]`, `[revision]`, `[build]` and `[userdefined]`.

§ 53(12) A Target may override Project version fields individually.

§ 53(13) Target version resolution is independent of platform Variant unless the programmer explicitly chooses version text that says otherwise.

§ 53(14) The authoritative build value is visible in the manifest.

§ 53(15) A programmer may manually change or reset a build value.

§ 53(16) Automatic build numbering increments the selected manifest build field transactionally and only commits after successful requested builds.

§ 53(17) Failed automatic builds do not commit the candidate build number.

§ 53(18) Explicit `--build-number` changes the current invocation only and does not mutate the manifest.

§ 53(19) Changing major, minor or revision never silently resets build.

§ 53(20) Ordinary Modules require no manifest entry unless they are also Target entry Modules.

§ 53(21) A Target is a logical product; a Variant is one concrete platform/build configuration.

§ 53(22) Every Target-and-Variant pair produces a concrete `CompilationPlan`.

§ 53(23) `sec init` is additive, idempotent, conflict-aware, transactional and non-destructive.

§ 53(24) Project tooling implementation status belongs in `governance/tooling_projects.yaml`, while modules, compiler plans, platform behavior, diagnostics and LSP behavior remain in their respective governance owners.

---

## References

- RFC 9562 — Universally Unique IDentifiers (UUIDs): https://www.rfc-editor.org/rfc/rfc9562.html
