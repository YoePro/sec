# Sec — MD-021: Target C Binding Environment — TOML, Compiler CLI, and Resolution Rules

- **Status:** Complete design proposal; pending user acceptance and normative integration
- **Created:** 2026-11-19
- **Last updated:** 2026-11-20
- **Document revision:** 1.1
- **Sec language version:** 0.1
- **Decision:** MD-021 (`missing-decisions.yaml`)
- **Repository baseline:** `d680ce820748224e3527a63699aefa4fd34250e0` (`main`, checked 2026-11-20)
- **Intended integration:** `rules/platform/ffi.md`, `rules/platform/abi.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/foundations/attributes.md`, `rules/foundations/grammar.md`, `rules/projects/projects.md`, `rules/projects/modules.md`, `rules/tooling/cli.md`, related governance fragments
- **Scope:** Metadata schema, source binding split, machine-safe compiler CLI, deterministic resolution for each CompilationPlan, cross-compilation and validation. This is proposed normative text, not a claim of implementation.

---

## § 1. Purpose and authoritative split

**§ 1(1)** A C binding *package* can describe multiple operating systems, architectures, ABIs, C runtime families and supported runtime-version ranges. Each immutable `CompilationPlan` resolves exactly one compatible concrete C binding *environment*. Cross-compilation never obtains missing ABI facts from the compiler host.

**§ 1(2)** TOML expresses identity, compatibility and module membership. **Sec source** expresses foreign declarations and transparent C aliases. TOML must not become an alternative type-declaration language.

**§ 1(3)** Compiler-owned fundamental C ABI types and foreign constructs retain their existing meaning. A selected and verified platform binding environment supplies additional canonical `c::...` types without a source-level import. Third-party C libraries remain ordinary imported Sec modules; no ordinary project may redefine fundamental or pre-existing canonical platform `c::` definitions.

**§ 1(4)** A compiler installation may ship supported binding packages. Project-local additions must pass normal trust, ABI compatibility and duplicate-definition checks; they must not override fundamental compiler-known ABI facts. Discovery, uniqueness and explicit selection are governed by § 8. There is no implicit project-local override or last-wins precedence.

## § 2. Location and metadata contract, version 1

**§ 2(1)** The default portable metadata file for this binding package is `.sec/platform/c-bindings.toml`. Its corresponding Sec declarations normally live under `sec/platform/`. The compiler can also accept an explicit file path through the CLI.

**§ 2(2)** Proposed schema, with illustrative runtime version constraints:

```toml
schema_version = 1

[bindings]
id = "sec.hosted-c"
version = "1.0.0"
source_root = "sec/platform"

[modules]
common = [
    "c/common"
]

[environments.linux-amd64-glibc]
os = "linux"
arch = "amd64"
abi = "gnu"
c_runtime = "glibc"
c_runtime_versions = ">=2.31, <3.0"
modules = [
    "c/linux/glibc/common",
    "c/linux/glibc/amd64"
]

[environments.linux-arm64-glibc]
os = "linux"
arch = "arm64"
abi = "gnu"
c_runtime = "glibc"
c_runtime_versions = ">=2.31, <3.0"
modules = [
    "c/linux/glibc/common",
    "c/linux/glibc/arm64"
]

[environments.linux-amd64-musl]
os = "linux"
arch = "amd64"
abi = "gnu"
c_runtime = "musl"
c_runtime_versions = ">=1.2, <2.0"
modules = [
    "c/linux/musl/common",
    "c/linux/musl/amd64"
]

[environments.windows-amd64-ucrt]
os = "windows"
arch = "amd64"
abi = "msvc"
c_runtime = "ucrt"
modules = [
    "c/windows/ucrt/common",
    "c/windows/ucrt/amd64"
]
```

**§ 2(3)** Schema keys:

| Key | Requirement | Meaning |
| --- | --- | --- |
| `schema_version` | Required | Metadata grammar version; initial supported value `1`. |
| `bindings.id` | Required | Stable binding-package identity; not a target identity. |
| `bindings.version` | Required | Version of the binding package, distinct from the C runtime. |
| `bindings.source_root` | Required | Path to the root of the Sec declaration modules, relative to the binding package root. |
| `modules.common` | Optional | Module directories shared by all environments. |
| `environments.<name>` | Zero or more during initial authoring; one or more for a resolvable package | Stable environment key, unique within the package. |
| `.os`, `.arch`, `.abi`, `.c_runtime` | Required per environment | Canonical compatibility selectors. |
| `.c_runtime_versions` | Optional per environment | Compatibility constraint on the C runtime, **not** the binding-package version. |
| `.modules` | Required per environment | Module-directory paths relative to `bindings.source_root`; an empty array is permitted if all definitions are common. |

**§ 2(4)** Module path values identify Sec source module directories according to the existing modules rulebook, not source `module` declarations and not filesystem paths in the source grammar. The `module` statement contains exactly one identifier; dotted module declarations are forbidden.

**§ 2(5)** The binding package root is the root in which `.sec/platform/c-bindings.toml` and `sec/platform/` are siblings in the example layout, regardless of whether that root is the project or compiler-supplied installation/package. Explicit references must be portable and normalized; no machine-specific SDK, compiler-installation or local C header paths belong in this contract. Path escape outside the allowed root is rejected.

**§ 2(6)** There is no `extends` mechanism and no inheritance between named environments. Common modules are shared, while environment-specific modules specialize the binding surface. Environment module-list order and file enumeration order have no semantic precedence.

**§ 2(7)** Each environment is identified by binding-package ID, binding-package version and environment key. A resolved plan additionally records exact selected C runtime/ABI facts, the resolved module set and a canonical semantic fingerprint. A file-byte checksum used for safe editing is distinct from this semantic fingerprint.

**§ 2(8)** A newly initialized package with zero environments is a valid **editing draft**, but cannot be selected by any `CompilationPlan` until it contains at least one resolvable environment. Schema version `1` rejects unknown semantic keys unless an owning extension explicitly specifies them. Unknown schema versions are rejected. The version-constraint grammar, runtime contract facts and ambiguity handling are governed by § 8.

## § 3. Transparent C aliases reside in Sec source

**§ 3(1)** `@cAlias` is a compiler-known **standalone module-scope declaration**. It does not attach to a following declaration and does not alter ordinary Sec `type` (nominal) semantics. No `alias` keyword or general transparent type-alias syntax is introduced.

**§ 3(2)** Example in a properly named Sec source module:

```sec
module amd64

@cAlias(
    name: "c::stddef::size_t",
    target: c::ulong
)

@cAlias(
    name: "c::example::pointer",
    target: RawPtr[c::char]
)
```

**§ 3(3)** The `name` is a compile-time qualified `c::...` spelling. The `target` is a **TypeReference**, not an arbitrary general expression or quoted type string. This is a specific grammar exception for `@cAlias` to the existing general attribute-argument model. Syntax, formatter, parser, Sema and LSP must all recognize it.

**§ 3(4)** Aliases may chain through other transparent C aliases and may resolve to any legal FFI-representable type, including pointers, arrays, structs, unions and callback/function-pointer representations. The chain must be acyclic and resolve to an existing compatible type under the selected `CompilationPlan`. An alias never bypasses foreign ABI legality checks.

**§ 3(5)** Common and environment-specific declarations are composed for each selected plan. An environment-specific definition of the same C alias specializes a common definition; multiple definitions at the same precedence level are errors regardless of file/import order. Alias resolution occurs **after** environment-specific specialization.

**§ 3(6)** `@target` continues to use Sec's canonical syntax, e.g. `@target(os: "linux")`. It is not required to encode C runtime choices into `@target`; module membership is chosen by the binding environment. Attribute declarations are only effective in compiler-authorized C binding modules.

## § 4. Compiler CLI must own creation, mutation and inspection

**§ 4(1)** The Sec compiler must provide a first-party CLI to **create, read, validate and edit** the TOML metadata without requiring a human or AI harness to manipulate TOML text directly. This is required functionality of MD-021, not an optional future editor integration.

**§ 4(2)** Proposed canonical command family: `sec platform c-bindings`. It must be registered in the existing authoritative CLI command registry so command parsing, `sec help`, `sec help platform`, contextual `--help` and diagnostics remain synchronized (`rules/tooling/cli.md` §§ 2–4).

**§ 4(3)** The minimum command surface is:

```text
sec platform c-bindings init --id ID --version VERSION --source-root DIR [--file FILE] [--json]
sec platform c-bindings show [--file FILE] [--json]
sec platform c-bindings package set [--id ID] [--version VERSION] [--source-root DIR] [--file FILE] [--if-match SHA256] [--json]
sec platform c-bindings validate [--file FILE] [--variant NAME] [--json]
sec platform c-bindings resolve --variant NAME [--file FILE] [--json]
sec platform c-bindings environment add NAME --os OS --arch ARCH --abi ABI --c-runtime RUNTIME [--c-runtime-versions RANGE] [--file FILE] [--if-match SHA256] [--json]
sec platform c-bindings environment set NAME [--os OS] [--arch ARCH] [--abi ABI] [--c-runtime RUNTIME] [--c-runtime-versions RANGE | --clear-c-runtime-versions] [--file FILE] [--if-match SHA256] [--json]
sec platform c-bindings environment remove NAME [--file FILE] [--if-match SHA256] [--json]
sec platform c-bindings module add PATH (--common | --environment NAME) [--file FILE] [--if-match SHA256] [--json]
sec platform c-bindings module remove PATH (--common | --environment NAME) [--file FILE] [--if-match SHA256] [--json]
```

**§ 4(4)** `--file` defaults to `.sec/platform/c-bindings.toml` in the current project/package root. `init` creates the directory/file if absent and rejects incompatible existing files instead of overwriting them. `package set` explicitly edits package identity/version/source-root fields; changes of package identity remain traceable and may invalidate consumers. `show`, `validate` and `resolve` are read-only. Mutating commands update **only** the requested logical element; they must not silently change another environment, module, package identity or version.

**§ 4(5)** Commands support both normal human-readable results and `--json`. Diagnostics-producing commands must also honor the global `--diagnostic-format <human|json>` contract. Exact accepted spellings are documented in `sec help`; no undocumented aliases are normative.

## § 5. AI-harness-safe editing requirements

**§ 5(1)** `show --json` produces a stable, versioned, parseable representation of the complete metadata model plus `file_sha256` (SHA-256 of the raw current file bytes). This lets an AI harness understand the existing configuration without scraping TOML or human output.

**§ 5(2)** `--if-match SHA256` is an optimistic concurrency guard. Every mutation made with this option must re-read and compare the file checksum immediately before the atomic replacement. If the checksum differs, the command rejects the edit without changing the file. A missing file is never implicitly treated as a match. This protects concurrent developer and agent edits.

**§ 5(3)** Mutations must be atomic: parse, check preconditions, perform a syntax-aware minimal edit, validate the resulting TOML structure and schema, write a temporary file and replace only after successful validation. Failures leave the previous file unmodified. Comment and unrelated formatting preservation is mandatory where possible, following `rules/projects/projects.md` § 5(4–6). Re-serialization of the whole file that needlessly destroys comments is not acceptable.

**§ 5(4)** Repeated `module add` of an already-present identical module is a successful no-op (`changed: false`), not a duplicate. `environment add` with an identical existing environment is likewise a no-op; differing existing definitions are errors and require explicit `environment set`. `module remove` and `environment remove` operate on exact identifiers, never wildcard patterns. Removing a non-existent target is reported as `changed: false` with an explanatory machine-readable status. These behaviors support repeatable AI-generated plans without masking conflicting definitions.

**§ 5(5)** A mutating `--json` command emits exactly one UTF-8 JSON document on stdout; human progress and diagnostics do not corrupt it. Minimum result contract:

```json
{
  "format": "sec.c-bindings-cli",
  "version": 1,
  "operation": "environment.add",
  "status": "complete",
  "changed": true,
  "file": ".sec/platform/c-bindings.toml",
  "before_sha256": "<hex-or-null>",
  "after_sha256": "<hex-or-null>",
  "diagnostics": []
}
```

**§ 5(6)** Failed commands report `status: "rejected"`, `changed: false`, and structured diagnostics with stable IDs and actionable messages conforming to `rules/tooling/diagnostics.md`; rejected edits do not write. JSON schema/format versions may only change through explicit versioning. Exit success is `0`; invalid command usage and semantic/validation failures must follow the established CLI error-status conventions instead of introducing arbitrary new exit codes.

**§ 5(7)** `validate` must check TOML syntax, supported schema version, mandatory fields, canonical target spelling, duplicate/contradictory declarations, module-path safety and runtime version constraints under the specified constraint grammar. An editing draft with zero environments may pass structural validation but must be reported as not resolvable. With `--variant NAME`, `validate` also executes the selected variant's full binding-environment resolution and checks the selected Sec declarations and ABI contract (§ 8). Without `--variant`, it performs structural/schema validation only and must clearly report that scope. `resolve --variant NAME` provides the selected environment, exact resolved facts and provenance without modifying files.

**§ 5(8)** The CLI manages metadata. It must **not** invent ABI-dependent alias targets, infer target C declarations from the build host, run an implicit C preprocessor, or generate unsafe foreign signatures. Source declarations remain separately authored Sec code or explicit outputs of an optional binding-generation tool.

**§ 5(9)** The compiler's standard help must document the workflow, and conformance tests must cover initialization, every mutation, no-op idempotence, stale-hash rejection, failed-edit rollback, comment preservation, stable JSON, cross-platform environment coexistence, and malformed/incompatible inputs.

## § 6. Example authoring workflow

```sh
sec platform c-bindings init \
  --id sec.hosted-c --version 1.0.0 --source-root sec/platform --json

sec platform c-bindings module add c/common --common --json

sec platform c-bindings environment add linux-amd64-glibc \
  --os linux --arch amd64 --abi gnu --c-runtime glibc \
  --c-runtime-versions '>=2.31, <3.0' --json

sec platform c-bindings module add c/linux/glibc/amd64 \
  --environment linux-amd64-glibc --json

sec platform c-bindings validate --json
sec platform c-bindings show --json
```

The commands above are a **proposed Sec CLI contract** and are not claims that the current compiler already implements them. For concurrent agent editing, the agent first reads `show --json`, then passes the returned `file_sha256` as `--if-match` to each mutating command and refreshes it after every successful change.

## § 7. Implementation and governance obligations

**§ 7(1)** The compiler command registry, argument validation, TOML read/write layer, syntax-aware edits, JSON output, diagnostics and CLI tests are part of the MD-021 implementation scope. A spec that only describes how to hand-edit TOML does **not** satisfy MD-021.

**§ 7(2)** Parser/Sema support for standalone `@cAlias`, source-file/statement target selection, authorized `c::` namespace registration, alias composition/cycle checks, target ABI verification and immutable plan fingerprinting are required to realize the language-facing part.

**§ 7(3)** Add traceable governance entries for TOML schema, CLI creation/editing/validation, alias source semantics, environment resolution and cross-Variant conformance. Mark MD-021 closed only after the user accepts this complete contract and the normative rulebooks are synchronized (implementation may remain partial and tracked separately).

## § 8. Deterministic C binding environment resolution

### § 8.1. Resolution unit and inputs

**§ 8.1(1)** Resolve C bindings independently for **each** selected Target Variant. A multi-variant build produces one immutable `CompilationPlan` per Variant and must not merge C environments from different plans. Compiler-host OS, native libc, SDK and C headers are never implicit substitutes for target facts.

**§ 8.1(2)** The authoritative inputs are: normalized Project Target/Variant and relevant CLI selection; the target/ABI/runtime facts resolved by `rules/platform/platform_model.md` and `rules/platform/abi.md`; the discovered, validated C-binding package set; and any explicit C-binding selector on the Variant. A package's compatibility selectors may **filter** candidates; they must not mutate the target ABI or choose a different C runtime to make themselves fit.

**§ 8.1(3)** Resolution is read-only relative to project/package metadata. `sec build`, `sec check`, `sec analyse` and `sec platform c-bindings resolve` must not rewrite a TOML file, upgrade a package or fetch unrequested dependencies merely to find a compatible environment.

### § 8.2. Discovery and package identity

**§ 8.2(1)** The initial candidate registry consists of (a) compiler-distributed, trusted C-binding packages and (b) the current project's optional `.sec/platform/c-bindings.toml`, if present. Additional external package roots may become candidates only through an **explicit** configured package/dependency mechanism. Merely finding a file somewhere on the host filesystem does not authorize or select it.

**§ 8.2(2)** Every package first passes supported-schema, origin/trust, path-containment and identity validation. A candidate that is incompatible with the target may be ignored for that target; a malformed or untrusted package **explicitly requested** by a Variant is an error, never a fallback to another package. Compiler-provided facts and fundamental `c::` types cannot be overwritten by package data.

**§ 8.2(3)** Package identity is `(bindings.id, bindings.version)`. Two discovered packages with the same identity but different canonical content are a conflict and cannot be reconciled by filesystem order, package locality, timestamp or discovery precedence. The same canonical content may be deduplicated after independent trust validation. Project-local packages do **not** silently override compiler-shipped packages.

**§ 8.2(4)** For `validate --variant` and `resolve`, an explicit `--file FILE` adds the named TOML file as an explicitly requested binding-package candidate (and uses its own package root to interpret `source_root`). It does not suppress identity conflicts or authorize overriding a compiler-owned C ABI fact. In the absence of `--file`, only the defined registry sources are considered. Editing commands use `--file` only to identify the file to read or modify and do not change build resolution.

**§ 8.2(5)** A normal third-party library remains a separately imported Sec module and is not automatically a platform C-binding package. To add canonical `c::` names, the user must select an authorized binding package; third-party imports cannot implicitly extend the selected platform environment. This avoids an unspecified global overlay mechanism in Sec 0.1.

### § 8.3. Optional, portable Variant selectors

**§ 8.3(1)** The following additional fields are proposed under `rules/projects/projects.md` § 19. They are **Variant** fields, not build-host settings:

```toml
[variant.linux-amd64]
os = "linux"
arch = "amd64"
abi = "gnu"
c_runtime = "glibc"
c_runtime_version = "2.39"
c_bindings = { id = "sec.hosted-c", version = "=1.0.0", environment = "linux-amd64-glibc" }

[variant.windows-amd64]
os = "windows"
arch = "amd64"
abi = "msvc"
c_runtime = "ucrt"
c_bindings = { id = "sec.hosted-c", version = "=1.0.0", environment = "windows-amd64-ucrt" }
```

**§ 8.3(2)** `c_runtime` selects the **target** C-runtime family. If omitted, a unique, verifiable default may be supplied by the canonical target/toolchain configuration. `c_runtime_version` is the target's declared and verified C-runtime **API/ABI compatibility baseline**, not the currently installed runtime version on the compiler host. If neither the Variant nor trusted target metadata supplies a version, its value is **unknown**, not inferred from the host.

**§ 8.3(3)** `c_bindings` is optional. If present, `id` is mandatory and identifies the binding package; `version` constrains binding-package versions; `environment` selects a particular environment key within the selected package. Omitting the optional version or environment never grants a right to choose arbitrarily among competing candidates. An explicit environment name still must pass every compatibility check. Unknown fields and invalid constraints are configuration errors.

**§ 8.3(4)** For the initial hosted model, `c_runtime`, `c_runtime_version` and `c_bindings` are permitted at Variant scope as above. Any CLI overrides of these selectors must be explicitly registered in the canonical CLI contract and must be normalized before selection; this proposal defines no undocumented flags.

**§ 8.3(5)** A bare-metal or other target that does not use C bindings must not be forced to select a hosted C environment. Whether bindings are required is determined by the selected target/profile and actual C-binding/FFI requirements; a C-binding-required plan without a resolvable environment fails closed.

### § 8.4. Constraint syntax and facts

**§ 8.4(1)** `bindings.version` uses `MAJOR.MINOR.PATCH` decimal notation; `c_bindings.version` supports an exact version (`=1.0.0`) or a comma-separated conjunction of `>`, `>=`, `<`, `<=`, `=` comparisons. No implicit `latest`, wildcard, prerelease, OR operator, caret or tilde expansion is defined in schema v1. Numeric comparison is componentwise. A different feature/version grammar requires an explicit schema revision.

**§ 8.4(2)** C-runtime versions in `c_runtime_version` and `c_runtime_versions` use `MAJOR.MINOR` or `MAJOR.MINOR.PATCH`, normalizing omitted patch to zero for comparison. `c_runtime_versions` uses the same comparator and comma-conjunction syntax as § 8.4(1); constraints are runtime-family-specific and do not imply that every real runtime release is ABI-compatible. The package owner is responsible for declaring only actually verified compatible ranges.

**§ 8.4(3)** An environment without `c_runtime_versions` places no **additional version restriction** but still must pass runtime-family and target ABI validation. When an environment declares `c_runtime_versions`, a verified `c_runtime_version` is required; unknown versions cannot be assumed to satisfy a range. An absent target/runtime version is not a match against a bounded interval. The target toolchain or target registry may provide verified facts, but the compiler-host runtime is not authoritative for cross-compilation.

**§ 8.4(4)** The binding-package version and the selected C-runtime version are separate dimensions. Matching a binding-package version never proves the C runtime is compatible; matching a C-runtime range never selects a binding-package version.

### § 8.5. Selection algorithm

**§ 8.5(1)** The resolver performs the following logical stages in a deterministic order. Implementations may reorganize internal code but must return equivalent results:

```text
1. Normalize the selected Target Variant and explicit C binding constraints.
2. Resolve the target architecture, ABI, runtime family and verified runtime version,
   without consulting candidate binding packages to invent target facts.
3. Discover and validate eligible binding packages (§ 8.2).
4. Filter packages by explicit package ID and binding-package version constraint,
   if supplied.
5. Enumerate each package's environments and filter by exact canonical os,
   arch, abi and c_runtime values and by applicable runtime-version constraints.
6. Apply any explicit environment-key selector and reject incompatible keys.
7. Require exactly one surviving (package ID, exact package version, environment key)
   tuple, or reject with missing/ambiguous-environment diagnostics.
8. Select common modules plus the chosen environment's modules, after safety
   and module-identity validation; no module ordering confers precedence.
9. Collect authorized C declarations in a declaration-only binding pass;
   apply alias specialization, resolve alias chains and check FFI/ABI legality.
10. Bind exact environment identity, target runtime/ABI facts, module set,
    verified alias surface, provenance and fingerprints into the CompilationPlan.
11. Verify and freeze the immutable CompilationPlan before ordinary Sema and lowering.
```

**§ 8.5(2)** A known, unique canonical default is permitted **only** when the normalized target facts and available registry yield exactly one compatible candidate after all applicable constraints. If zero match, report missing/incompatible bindings. If more than one matches, report ambiguity with the candidate identities and suggest an explicit `c_bindings` selection. Never choose "highest version", "first registered", "local before built-in" or "closest match" implicitly.

**§ 8.5(3)** Version ranges are selection constraints, **not a lockfile**. A plan records the exact resolved version and fingerprint; rerunning resolution after the available package set changes may require a new explicit selection. Projects requiring repeatable resolution should pin `c_bindings.version` to an exact version. A range must never silently make an **already frozen** plan change its resolved binding identity.

**§ 8.5(4)** Environment keys are local to their binding package: two packages may use the same environment key, but they remain distinct candidates. Canonical target aliases are normalized before comparing `os`, `arch`, `abi` and runtime identifiers, and normalization never silently changes the selected ABI.

### § 8.6. Modules, specialization and foreign declarations

**§ 8.6(1)** Every selected module directory is rooted under the chosen package's `bindings.source_root`; `..`, absolute paths, symlink escapes and colliding canonical source-module identities are rejected. Source files must obey ordinary Sec module-name rules. Source selection honors existing `@target` rules and is independent of the order of `modules.common`, the environment's `modules` array and filesystem traversal.

**§ 8.6(2)** The compiler processes common declarations and environment declarations as two explicit **semantic layers**, not as an ordered sequence. For `@cAlias` only, one selected environment-level definition of a qualified alias name replaces one common-level definition of that name. Duplicate declarations on the **same** layer, including identical duplicates, are errors. The environment layer cannot replace compiler-owned fundamental C types.

**§ 8.6(3)** This alias-specialization rule is **not** a general declaration or module override rule: conflicting functions, structs, unions or other non-alias declarations follow their existing duplicate/conflict and FFI legality rules. The resolver must not silently override them by module order.

**§ 8.6(4)** After specialization, the compiler resolves the complete alias graph against selected authorized `c::` types and legal FFI-representable types. Unknown names, circular dependencies, illegal representations, incompatible aggregate/callback layouts and unsupported ABI classifications reject the selected environment. A transparent alias does not introduce a nominal type identity.

**§ 8.6(5)** Collection of the selected C-binding declaration surface must be a restricted **declaration-only** pre-freeze phase. It may use already resolved target/ABI facts but must not run arbitrary user code, compile-time side effects or runtime probes. It must not change target identity or source selection in response to its own declarations. Ordinary Sema then consumes the frozen canonical binding surface. This extends the existing staged `CompilationPlan` resolution architecture rather than permitting mutation of a frozen plan.

### § 8.7. Failure behavior and cross-compilation

**§ 8.7(1)** All failures are compile-time/validation diagnostics. The minimum distinguished conditions are: unknown package, unknown environment, unsupported schema, malformed or untrusted package, duplicated package identity with different content, missing modules, unsafe paths, incompatible OS/arch/ABI/runtime, unknown required runtime version, unsatisfied version range, no candidate, multiple candidates, alias collision, alias cycle and unsupported FFI layout/classification.

**§ 8.7(2)** An explicit but incompatible selection is **not** a request to switch the plan's C runtime, target triple or ABI. It is an error. No missing fact is inferred from the host, another Variant, the linker default or the first available binding directory.

**§ 8.7(3)** In a multi-Variant build, a compatible binding environment in Variant A is never evidence of compatibility in Variant B. Plans may share cache entries only when the relevant frozen semantic/ABI/source fingerprints prove reuse is sound. A failed Variant is reported against that Variant and does not contaminate the resolved facts of other Variants.

### § 8.8. Identity, fingerprints and reproducibility

**§ 8.8(1)** The selected environment identity records, at minimum: package ID, **exact** package version, environment key, schema version, canonical target OS/architecture, ABI identity, C-runtime family and verified runtime-version contract when relevant, selected module identities, resolved C alias/type surface and package origin/provenance.

**§ 8.8(2)** The binding semantic fingerprint includes the normalized meaning of all facts that can alter FFI typing, alias resolution, ABI layout/classification or selected module semantics. Relevant source changes invalidate dependent plan/cache domains. It does **not** depend on file modification times, TOML table ordering, comments, irrelevant filesystem paths, or `modules` array ordering. The raw `file_sha256` used by the editing CLI is a separate edit-concurrency guard and is not the compilation semantic fingerprint.

**§ 8.8(3)** An immutable `CompilationPlan` records the exact selected identity and binding fingerprint. Changes to the package, selected module declarations, environment compatibility, compiler ABI schema, toolchain capabilities or target runtime facts require revalidation and an appropriately new plan/fingerprint. A stale cached result must never substitute for a rejected current environment.

### § 8.9. AI-harness inspection and validation CLI

**§ 8.9(1)** `sec platform c-bindings validate` without `--variant` checks TOML structure, declared identities, schema, path syntax and version-constraint syntax, and reports `scope: "structural"`. It must not claim ABI/FFI validation of unselected sources. A valid editing draft with zero environments remains `resolvable: false`.

**§ 8.9(2)** `sec platform c-bindings validate --variant NAME` performs § 8.5 resolution for the named Variant and validates its selected binding source and ABI legality without mutating the package or project. It reports `scope: "variant"`, the selected candidate if any, structured diagnostics and the resolved binding fingerprint.

**§ 8.9(3)** `sec platform c-bindings resolve --variant NAME --json` is a first-party, read-only explanatory interface for developers and agents. Its output is exactly one versioned JSON document on stdout, for example:

```json
{
  "format": "sec.c-bindings-resolution",
  "version": 1,
  "status": "complete",
  "variant": "linux-amd64",
  "target": {
    "os": "linux", "arch": "amd64", "abi": "gnu",
    "c_runtime": "glibc", "c_runtime_version": "2.39"
  },
  "binding": {
    "id": "sec.hosted-c", "version": "1.0.0",
    "environment": "linux-amd64-glibc",
    "modules": ["c/common", "c/linux/glibc/common", "c/linux/glibc/amd64"],
    "fingerprint": "<canonical-semantic-fingerprint>"
  },
  "diagnostics": []
}
```

**§ 8.9(4)** Failed resolution produces `status: "rejected"`, a `binding` value of `null`, and actionable stable diagnostics. When ambiguity is the cause, the report identifies **all** compatible candidate tuples in canonical sorted order. JSON output must never depend on localized human diagnostic prose, registry iteration or module order. The CLI uses existing error exit-status conventions.

**§ 8.9(5)** To keep agent behavior deterministic, the resolver cannot opportunistically repair or mutate metadata. An agent can inspect `resolve --json`, author targeted changes with `--if-match` from `show --json`, then rerun `validate --variant` and `resolve --json` to verify the intended state.

## § 9. Normative integration and acceptance tests

**§ 9(1)** Incorporate the binding split, C alias semantics and selected environment into `rules/platform/ffi.md`; target selection and failure behavior into `rules/platform/target_profiles.md`, `rules/platform/platform_model.md` and `rules/platform/abi.md`; the new Variant selector keys into `rules/projects/projects.md`; standalone `@cAlias` grammar/attachment into `rules/foundations/grammar.md` and `rules/foundations/attributes.md`; module selection into `rules/projects/modules.md`; and the `sec platform c-bindings` CLI into `rules/tooling/cli.md`. All changes need stable paragraph references, diagnostics and governance tasks.

**§ 9(2)** Conformance tests shall cover at least: Linux glibc versus musl on the same architecture; Windows UCRT/MSVC; x86-64 versus arm64; common-alias fallback and environment override; duplicate aliases on each layer; alias chains/cycles; C aggregate/function-pointer alias targets; malformed or absent modules; target-runtime-version unknown/mismatch; binding-package range/exact selection; multiple matching package versions; competing environments; project-versus-compiler duplicate identities; project-local custom package; cross-compilation without a host C runtime; a cross-Variant build with independent results; stale cache rejection; JSON resolver output and all relevant AI-harness-safe CLI editing tests from § 5(9).

**§ 9(3)** This document is a complete **proposal** for MD-021. Once the user approves its resolver semantics and the normative rulebooks and `missing-decisions.yaml` are synchronized, MD-021 may be marked **closed as a design decision**. Compiler and tooling implementation work remains separately tracked and may be partial; design closure is not an assertion of implemented support.
