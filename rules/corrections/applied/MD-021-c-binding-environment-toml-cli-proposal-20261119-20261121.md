# Sec — MD-021: Target C Binding Environment — TOML Contract and Compiler Management CLI

- **Status:** Design proposal; pending final resolution rules and normative integration
- **Created:** 2026-11-19
- **Last updated:** 2026-11-19
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Decision:** MD-021 (`missing-decisions.yaml`)
- **Repository baseline:** `cc987fc3bbab8229f45227e3e2be47839d67e282` (`main`, checked 2026-11-19)
- **Intended integration:** `rules/platform/ffi.md`, `rules/platform/abi.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md`, `rules/foundations/attributes.md`, `rules/foundations/grammar.md`, `rules/projects/projects.md`, `rules/projects/modules.md`, `rules/tooling/cli.md`, related governance fragments
- **Scope:** Metadata schema, source binding split, and a machine-safe compiler command surface. The final selection algorithm is a follow-on section and must be completed before MD-021 is closed.

---

## § 1. Purpose and authoritative split

**§ 1(1)** A C binding *package* can describe multiple operating systems, architectures, ABIs, C runtime families and supported runtime-version ranges. Each immutable `CompilationPlan` resolves exactly one compatible concrete C binding *environment*. Cross-compilation never obtains missing ABI facts from the compiler host.

**§ 1(2)** TOML expresses identity, compatibility and module membership. **Sec source** expresses foreign declarations and transparent C aliases. TOML must not become an alternative type-declaration language.

**§ 1(3)** Compiler-owned fundamental C ABI types and foreign constructs retain their existing meaning. A selected and verified platform binding environment supplies additional canonical `c::...` types without a source-level import. Third-party C libraries remain ordinary imported Sec modules; no ordinary project may redefine fundamental or pre-existing canonical platform `c::` definitions.

**§ 1(4)** A compiler installation may ship supported binding packages. Project-local additions must pass normal trust, ABI compatibility and duplicate-definition checks; they must not override fundamental compiler-known ABI facts. The precise discovery and precedence algorithm is governed by § 8 (still to be finalized).

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

**§ 2(8)** A newly initialized package with zero environments is a valid **editing draft**, but cannot be selected by any `CompilationPlan` until it contains at least one resolvable environment. Schema version `1` rejects unknown semantic keys unless an owning extension explicitly specifies them. Unknown schema versions are rejected. C runtime version-constraint grammar, exact package constraints and default/ambiguity resolution are finalized with § 8, not inferred from illustrative strings.

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
sec platform c-bindings validate [--file FILE] [--json]
sec platform c-bindings environment add NAME --os OS --arch ARCH --abi ABI --c-runtime RUNTIME [--c-runtime-versions RANGE] [--file FILE] [--if-match SHA256] [--json]
sec platform c-bindings environment set NAME [--os OS] [--arch ARCH] [--abi ABI] [--c-runtime RUNTIME] [--c-runtime-versions RANGE | --clear-c-runtime-versions] [--file FILE] [--if-match SHA256] [--json]
sec platform c-bindings environment remove NAME [--file FILE] [--if-match SHA256] [--json]
sec platform c-bindings module add PATH (--common | --environment NAME) [--file FILE] [--if-match SHA256] [--json]
sec platform c-bindings module remove PATH (--common | --environment NAME) [--file FILE] [--if-match SHA256] [--json]
```

**§ 4(4)** `--file` defaults to `.sec/platform/c-bindings.toml` in the current project/package root. `init` creates the directory/file if absent and rejects incompatible existing files instead of overwriting them. `package set` explicitly edits package identity/version/source-root fields; changes of package identity remain traceable and may invalidate consumers. `show` and `validate` are read-only. Mutating commands update **only** the requested logical element; they must not silently change another environment, module, package identity or version.

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

**§ 5(7)** `validate` must check TOML syntax, supported schema version, mandatory fields, canonical target spelling, duplicate/contradictory declarations, module-path safety and runtime version constraints under the specified constraint grammar. An editing draft with zero environments may pass structural validation but must be reported as not resolvable. A separate future resolver-backed validation mode may establish whether a given Target/Variant resolves successfully; `validate` does not silently claim that ABI compatibility has been proved when only the metadata was checked.

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

**§ 7(3)** Add traceable governance entries for TOML schema, CLI creation/editing/validation, alias source semantics, environment resolution and cross-Variant conformance. Mark MD-021 closed only when the normative rulebooks include the final agreed contract (implementation may remain partial and tracked separately).

## § 8. Remaining MD-021 item: resolution rules (not yet locked)

**§ 8(1)** The follow-on resolution specification must establish: discovery and precedence of compiler-supplied versus project-local binding packages; explicit C runtime selection and defaults; compatibility and version-range matching; how a `CompilationPlan` binds an **exact** environment/version/source set; deterministic ambiguity/conflict rejection; cross-compilation isolation; and canonical cache fingerprints.

**§ 8(2)** No package/environment selection or override precedence may silently depend on discovery order, host SDK contents or filesystem traversal order. The compiler must fail closed rather than guess.

**§ 8(3)** The TOML contract and management CLI presented here are proposed for acceptance now. § 8's algorithm is the remaining design task before the complete MD-021 correction is ready for normative integration.
