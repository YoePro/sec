# MD-111 — Hardware fact declarations, typed schemas, project overrides, and verified endpoint bindings

- **Decision ID:** MD-111 — `hardware-endpoint-fact-source`
- **Document kind:** Cross-rulebook normative correction and implementation contract
- **Status:** **APPROVED — NORMATIVE DECISION**; canonical rulebook integration and implementation pending
- **Created:** 2026-10-27 (original MD-111 proposal)
- **Consolidated:** 2026-10-30
- **Approved by:** Sec language owner, 2026-10-30
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reviewed repository:** `YoePro/sec`, `main` at `aa59b4c18c91ed04eef69c7dd0028e41630067f5` (HEAD checked 2026-10-30)
- **Supersedes as design handoff:** `MD-111-source-and-project-hardware-facts-proposal.md` (revision 0.1) and `MD-111-hardware-declaration-member-contract-review-20261030.md` (revision 0.1)
- **Primary authorities to amend:** `rules/platform/platform_model.md`; `rules/platform/fixed-address-bindings.md`; `rules/platform/hardware-register-access.md`; `rules/projects/projects.md`; `rules/compiler/semantic_ir.md`
- **Additional authorities:** `rules/declarations/registers.md`; `rules/foundations/grammar.md`; `rules/foundations/lexical_structure.md`; `rules/foundations/attributes.md`; `rules/projects/modules.md`; `rules/compiler/compiler.md`; `rules/compiler/incremental_compilation.md`; `rules/tooling/lsp.md`; Sec MLIR and lowering contracts where applicable
- **Governance:** reconcile with existing `governance/platform.yaml` hardware-register integration and existing frontend/project/lowering/P15 owners; **do not create parallel duplicate owners**
- **Related decided contracts:** MD-077 (hardware instances versus detached register values), MD-110 and MD-114 (register value/MLIR), MD-112 (specialized writes), MD-113 (shadow knowledge)

> **Authority and use.** The Sec language owner approved the complete consolidated MD-111 contract on 2026-10-30, including both the previously approved foundations and the exact schemas, spellings and integration details in this document. **This decision is normative but not yet integrated or implemented.** Neither this document nor its illustrative declarations claims that the parser or platform model already implements the feature. Integrate **without renumbering existing numbered paragraphs**, test each discrete change before proceeding, and track implementation separately from the language decision.

---

## § 1. Scope and approved foundations

**§ 1(1). Approved foundation.** Sec 0.1 admits two authorable, typed, compile-time sources of hardware facts: (A) `hardware region`, `hardware resource`, and `hardware endpoint` declarations in selected Sec source modules, and (B) project hardware entries inside the existing `.sec/sec.toml`. Both normalize to **one** target/plan-scoped canonical hardware fact model. A future (C) knowledge-pack provider is reserved but is not part of the 0.1 acceptance criteria.

**§ 1(2). Approved foundation.** The project has precedence over source **for an explicitly identified source declaration**; an effective project property that differs from its overridden source value produces a compiler warning. Partial override is permitted; absent properties inherit from the selected source definition. A project may not weaken target/Device constraints, forge hardware capabilities, or change `register[N]` semantics.

**§ 1(3). Approved foundation.** Different names do not imply different hardware. Overlapping address regions and duplicate or overlapping register bindings produce appropriate alias warnings. The explicit spelling to acknowledge a verified binding alias is `@address(Endpoint, alias)`. Such an acknowledgement silences only the relevant acknowledged-alias warning; it does **not** create a hardware alias, grant ownership, permit unsafe interference, or make an illegal physical access legal.

**§ 1(4). Approved foundation.** Sec 0.1 hardware bindings must use a **named endpoint**, including on small MCUs and bare metal. A naked numeric `@address(0x...)` is a compile-time error **even when an applicable region is known**. Numeric hardware addresses remain legal **inside typed hardware metadata declarations**.

**§ 1(5). Approved foundation.** `Region`, `Resource`, and `Endpoint` remain distinct concepts. Regions classify address ranges; resources identify shared logical hardware state; endpoints describe supported physical access mechanisms and operations. The complete distinction must be reflected in `hardware-register-access.md` §§11–13 and explained, without duplication, in `registers.md`.

**§ 1(6). Approved foundation.** A `domain` refers to an address domain **already supplied by the selected target/platform**. Declaring a region in that domain does not authorize invention of a new address domain. Physical facts are contractual input, not empirical proof that the connected device matches its description.

**§ 1(7). Approved foundation.** Domain address values must not be restricted to a CPU's ordinary target-sized `uint`. In particular, `base` and `offset` use checked, compile-time address-domain-sized unsigned integer semantics (policy B). The exact metadata types and validation rules are specified below; these are not implicit conversions to runtime pointers.

**§ 1(8).** This decision introduces no runtime initialization by declaration, no automatic hardware observations, no hidden hardware writes, no untyped MMIO, no global shadow manager, no implicit raw-pointer access, no general device protocol execution, and no `fmt`/stdlib dependency. The ordinary Sec memory/ownership model, hardware register access permissions, MD-112 non-trigger effects, and MD-113 shadow rules remain authoritative.

---

## § 2. Canonical resolved identities and facts

**§ 2(1).** The compiler records separate canonical identities for:

```text
AddressDomainIdentity
MemoryRegionIdentity
HardwareResourceIdentity
HardwareEndpointIdentity
PhysicalOperationIdentity
HardwareBindingIdentity
VerifiedOperationPlanIdentity
```

Each is a distinct semantic identity category, not a source-level integer or Sec runtime type. An address, register type, binding name, or source spelling must never substitute for `HardwareResourceIdentity`.

**§ 2(2).** An A declaration is identified by **resolved canonical `ModuleIdentity` + declaration category + declaration name**. An ordinary B declaration is identified by **project identity + declaration category + manifest key**. A B override is an explicitly identified *revision of the resolved A declaration* and retains the A declaration's effective semantic identity, not a second independent resource or endpoint. The project export name is an additional lookup binding to that same effective declaration when explicitly declared.

**§ 2(3).** The normalized `HardwareFacts` model contains address domains, regions, resources, endpoints, physical operations, per-property provenance, effective applicability, alias relations and validity evidence. Its in-memory Go representation is implementation-defined; the semantic contracts and serializable identities are not.

**§ 2(4).** Alias and physical operation footprint facts are explicit. Numeric address equality, similar names, adjacent addresses and equal register layout do **not** prove that two endpoints have the same resource. Two different endpoints may intentionally name the same resource (e.g. GPIO output, set and clear aliases). Conversely, identical numeric addresses in different address domains need not mean physical aliasing.

**§ 2(5).** Exactly equivalent resolved hardware facts have the same semantic meaning independently of whether their source is A or B. A property override with an unchanged value is silent; a materially changed value generates a warning but does not itself constitute a compile-time validity failure.

---

## § 3. Metadata type model, units and value syntax

**§ 3(1).** Hardware members have **closed, compiler-known metadata types**. These are compile-time schema categories, not implicit dynamically boxed values, reflection types, hardware objects or automatically public runtime enums. The symbolic types in §§4–7 are fully documented within this metadata language. Their enum member spellings are deliberately modeled on Sec's ordinary closed string-backed enum syntax.

**§ 3(2).** The metadata categories are:

| Metadata type | Exact accepted domain | Evaluation rule |
|---|---|---|
| `string` | Sec string literal | Compile-time literal; no runtime expression |
| `bool` | `true`, `false` | Compile-time boolean literal |
| `uint` | Nonnegative compile-time integer fitting selected target `uint` | No wrap/truncation |
| `HardwareDomainAddress` | Nonnegative exact compile-time integer fitting selected address domain | **Not** limited by target `uint`; checked against domain address width |
| `HardwareDomainExtent` | Positive exact compile-time integer extent fitting the selected domain's addressable-unit range | **Not** limited by target `uint`; see §4 |
| `HardwareRegionRef` | Statically resolved declared hardware region symbol | Resolves to canonical region identity; no evaluation |
| `HardwareResourceRef` | Statically resolved declared hardware resource symbol | Resolves to canonical resource identity; no evaluation |
| `HardwareRegionKind`, etc. | Exactly one qualified member of the closed enum declared below | No unqualified bare enum values in newly written source |
| `list<uint>` | Bracketed sequence of exact `uint` elements | Ordered source spelling; normalized as a set where specified |

`HardwareDomainAddress` and `HardwareDomainExtent` are *descriptive compiler metadata types*, not new ordinary Sec runtime numeric types and not intended as manually-instantiable source declarations. They are resolved **after** the address domain is selected. Endpoints inherit the domain of their region. For a linear byte-addressed domain, address and extent units are bytes; another domain may define different addressable units.

**§ 3(3).** Every metadata integer is parsed as an exact, arbitrary-precision nonnegative literal (decimal or canonical hexadecimal), validated without intermediate overflow, and only then narrowed against its declared semantic domain. An integer expression relying on runtime values, arbitrary functions, hardware reads or implicit CTE is invalid. The single-value syntax for A is therefore deliberately not unrestricted Sec expression syntax.

**§ 3(4).** `HardwareDomainExtent` can denote a complete domain's cardinality where the chosen base permits it; validation uses the mathematical half-open range `[base, base + size)` without wrapping. It is rejected when the resulting range exceeds the domain limit. Endpoint `offset` uses `HardwareDomainAddress` measured in the region's addressable units; endpoint `size` uses `HardwareDomainExtent`; `alignment` uses `uint` measured in those units. Read/write transfer widths and resource width are in **bits**.

**§ 3(5).** The metadata parser accepts only: integer literals, string literals, `true`/`false`, qualified metadata enum members (`EnumName.Member`), resolvable hardware symbols (`Name` or a legal imported-module qualifier), and homogeneous bracketed lists of those values where the member specifies a list. A reference to a hardware symbol is a symbolic selector, not a read of the device or a load of an ordinary source binding. Other syntax is rejected with a diagnostic naming the expected member type.

**§ 3(6).** Member names are case-sensitive. The formatter preserves canonical member spelling, emits qualified enum cases and retains source structure. Unknown, duplicate or misspelled members are compile errors. Values do not gain target capabilities by type coercion.

---

## § 4. `hardware region`: exact v0.1 schema

| Member | Metadata type | Required | Contract |
|---|---|---|---|
| `domain` | `string` | Yes | Resolves to one target/platform-declared address domain identity; unknown domain is an error. |
| `kind` | `HardwareRegionKind` | Yes | Region storage/access classification. |
| `base` | `HardwareDomainAddress` | Yes for a linear region | First addressable unit in selected domain. |
| `size` | `HardwareDomainExtent` | Yes for a linear region | Positive extent in domain-addressable units. |
| `readable` | `bool` | Yes | Region-level read permission, not an endpoint safety proof. |
| `writable` | `bool` | Yes | Region-level write permission, not an endpoint safety proof. |

**§ 4(1).** The 0.1 source/manifest region schema represents **linear numeric regions only**. Other address domains may exist in the canonical platform model, but a user-defined `hardware region` requiring an unsupported non-linear location schema is rejected as unsupported, not silently interpreted as linear memory. This does not collapse PortMapped/InstructionEncoded endpoint identities into MMIO.

**§ 4(2).** `base`/`size` must describe a nonempty range entirely within a selected domain. Region `kind` must be compatible with the target/Device's established memory map. A project declaration cannot turn a target-forbidden region into usable MMIO. `kind: Reserved` forbids binding.

**§ 4(3).** `RAM` and `ROM` identify *candidate* ordinary addressed storage only. Ordinary `Place`/`ref` eligibility additionally needs an independently proven lifetime, storage, authority, reference compatibility, and mutability contract. In the absence of those facts the compiler fails closed; region `kind` alone does not manufacture safe references.

**§ 4(4).** The region's source identifier is its name. There is no duplicate `name:` member in A. A B manifest table key names the stored declaration; `name` is reserved only as a Sec-facing endpoint export in B (§9).

### § 4A. Exact `HardwareRegionKind` enum

```sec
enum HardwareRegionKind string {
    MMIO = "mmio",
    RAM = "ram",
    ROM = "rom",
    Reserved = "reserved",
}
```

| Case | Precise interpretation |
|---|---|
| `HardwareRegionKind.MMIO` | Device-owned state; accessible only by validated endpoint operations, not as ordinary `Place` memory. |
| `HardwareRegionKind.RAM` | Potentially reference-compatible writable storage subject to separately established storage/authority facts. |
| `HardwareRegionKind.ROM` | Potentially reference-compatible read-only storage subject to separately established storage/authority facts. |
| `HardwareRegionKind.Reserved` | Range unavailable for `@address` bindings. |

---

## § 5. `hardware resource`: exact v0.1 schema

| Member | Metadata type | Required | Contract |
|---|---|---|---|
| `width` | `uint` | Yes | Positive width **in bits** of one modeled register-state resource; check against nominal `register[N]` and endpoint operation footprints. |

**§ 5(1).** A resource represents logical state, not its address. Do **not** put `base`, `offset`, read/write direction, or endpoint `kind` on the resource. Separate endpoints may refer to the same resource and modify it with distinct effects.

**§ 5(2).** This 0.1 declarative schema covers bit-addressable register resources only. Future non-register hardware resources require explicit typed schema additions; do not invent a fictitious bit width for them.

**§ 5(3).** A resource declaration does not by itself allocate, initialize, snapshot, read, write or reset hardware.

---

## § 6. `hardware endpoint`: exact v0.1 schema

| Member | Metadata type | Required | Contract |
|---|---|---|---|
| `kind` | `HardwareEndpointKind` | Yes | Access mechanism; first executable declared kind is `MemoryMapped`. |
| `region` | `HardwareRegionRef` | Yes for `MemoryMapped` | Must resolve to a valid, applicable and compatible region. |
| `resource` | `HardwareResourceRef` | Yes | Declares the logical resource affected/observed by this endpoint. |
| `offset` | `HardwareDomainAddress` | Yes for `MemoryMapped` | Offset from effective region base, in domain units. |
| `size` | `HardwareDomainExtent` | Yes for `MemoryMapped` | Positive mapped extent in domain units; range must be contained in region. |
| `alignment` | `uint` | Yes for `MemoryMapped` | Positive required alignment in domain units; compatible with target transfer requirements. |
| `read_widths` | `list<uint>` | Yes | Permitted physical read widths in **bits**; empty list disables reads. |
| `write_widths` | `list<uint>` | Yes | Permitted physical write widths in **bits**; empty list disables writes. |
| `read_footprint` | `HardwareFootprintKind` | If reads enabled | Logical resource bits observed/effected by a read. First declarative contract: entire resource. |
| `write_footprint` | `HardwareFootprintKind` | If writes enabled | Logical resource bits potentially affected by a write. First contract: entire resource. |
| `read_effect` | `HardwareReadEffect` | If reads enabled | Physical read side effect; not interchangeable with register field read behavior. |
| `write_operation` | `HardwareWriteOperation` | If writes enabled | Available physical write capability. |
| `ordering` | `HardwareOrdering` | If any operation enabled | Required physical ordering/volatile occurrence contract. |
| `completion` | `HardwareCompletion` | If writes enabled | Declared physical write completion model. |
| `atomicity` | `HardwareAtomicity` | If any operation enabled | Declared guarantee; `Unspecified` proves no atomicity. |
| `context` | `HardwareAccessContext` | Yes | Access-context dependency; does not grant privilege. |
| `fault` | `HardwareFaultContract` | Yes | Applicable physical fault contract; not implicit `Result`. |

**§ 6(1).** `read_widths` and `write_widths` are positive, unique widths. At least one direction must be enabled. If read or write is disabled (empty list), its direction-specific properties **must be absent**, not filled with fake effects. The other direction remains fully validated. A direction declared at endpoint level must also be allowed by the region and the target.

**§ 6(2).** An endpoint's physical footprint, width, byte order, location and permissions must be checked against the nominal register type, actual physical resource, architectural Device facts, and each requested field operation. `width` does not imply whole-register read-modify-write safety. No inferred set/clear alias, atomicity, barrier or non-destructive observation.

**§ 6(3).** `read_footprint: Whole` and `write_footprint: Whole` refer to the complete declared logical resource width. Noncontiguous footprints, masked side-effects or multi-step protocols require separately specified typed operation data. The initial schema does **not** treat an omitted or invented bit mask as equivalent to a `Whole` footprint.

**§ 6(4).** `HardwareReadEffect.ReadClear` / `.ReadSet` are only expressible here when the effect scope is demonstrably the full resource and exact target semantics are supplied; this version's first executable backend may reject them as unsupported. The effects of `read-clear` or `read-set` at individual logical register fields cannot be deduced merely from the endpoint enum. All physical effects must be verified against the requested logical field operation.

**§ 6(5).** `HardwareWriteOperation.SetBits` / `.ClearBits` / `.ToggleBits` define physical endpoint capabilities, not new source operators. Their non-trigger and collateral guarantees remain subject to MD-112 and the target's physical contract; no generic RMW substitution is allowed.

**§ 6(6).** `HardwareEndpointKind.PortMapped`, `.InstructionEncoded`, `.RuntimeMapped` and `.TargetDefined` are reserved semantic cases so the canonical model does not assume MMIO. The 0.1 user-authored schema here provides no invented scalar `offset` semantics for them: construction is rejected with an explicit *unsupported endpoint-kind schema* diagnostic until that kind's location and operation contracts have been specified. Target-owned platform knowledge may support those kinds under independently defined contracts; they do not become arbitrary `@address(integer)` bind targets.

**§ 6(7).** `Fault: Platform` does not transform a target trap into recoverable `Result`. `context: Current` requires verification under the selected execution/privilege context; it does not claim that all contexts are valid. `completion: Issued` does not guarantee that a posted device write has become externally visible.

---

## § 7. Complete compiler-known metadata enums

All cases and their **exact** TOML serialized values are closed and case-sensitive. The forms below define compile-time **hardware metadata schema enums**, not implicitly public Sec runtime types. Each enum has exactly the cases listed for this v0.1 schema; no implicit `Unknown`, no numeric wildcard, and no undocumented alternative spellings.

```sec
enum HardwareEndpointKind string {
    MemoryMapped = "memory-mapped",
    PortMapped = "port-mapped",
    InstructionEncoded = "instruction-encoded",
    RuntimeMapped = "runtime-mapped",
    TargetDefined = "target-defined",
}

enum HardwareFootprintKind string {
    Whole = "whole",
}

enum HardwareReadEffect string {
    NonDestructive = "non_destructive",
    ReadClear = "read_clear",
    ReadSet = "read_set",
}

enum HardwareWriteOperation string {
    Replace = "replace",
    SetBits = "set_bits",
    ClearBits = "clear_bits",
    ToggleBits = "toggle_bits",
}

enum HardwareOrdering string {
    Volatile = "volatile",
}

enum HardwareCompletion string {
    Issued = "issued",
}

enum HardwareAtomicity string {
    Unspecified = "unspecified",
}

enum HardwareAccessContext string {
    Current = "current",
}

enum HardwareFaultContract string {
    Platform = "platform",
}
```

**§ 7(1).** `HardwareFootprintKind.Whole` is a full-resource operation footprint; the enum cannot describe arbitrary subfields, so the compiler must reject operations requiring a more specific footprint contract. `HardwareReadEffect.ReadClear` and `.ReadSet` are whole-effect *categories* and still require supported target semantics.

**§ 7(2).** `HardwareOrdering.Volatile` means exact required occurrence and ordering constraints as established by existing register hardware-access rules; it **does not** by itself define a CPU/device memory fence, acquire/release atomic order, or completion barrier.

**§ 7(3).** `HardwareCompletion.Issued` means successful issue of the physical operation, not confirmed external device completion. `HardwareAtomicity.Unspecified` carries no positive atomicity claim. Both are conservative rather than optimistically assumed target capabilities.

**§ 7(4).** Adding richer atomicity, footprint, ordering, completion or context cases requires a **versioned normative amendment** with full semantic meaning and a demonstrated target contract, not a free-form string in source/TOML. The first P15-backed executable slice may support only one fixed-width whole-register, single-transaction MMIO read or replacement write.

---

## § 8. Exact source declaration grammar and example

**§ 8(1).** Introduce contextual `hardware` at top-level declaration boundaries (not a globally reserved identifier without a lexical compatibility check). The canonical schematic grammar is:

```text
HardwareDeclaration
    ::= "hardware" ("region" | "resource" | "endpoint") Identifier "{" HardwareMemberList "}"

HardwareMemberList
    ::= HardwareMember { "," HardwareMember } [ "," ]

HardwareMember
    ::= Identifier ":" HardwareMetadataValue

HardwareMetadataValue
    ::= IntegerLiteral | StringLiteral | BooleanLiteral
      | QualifiedHardwareEnumCase | HardwareSymbolPath
      | "[" [ IntegerLiteral { "," IntegerLiteral } [ "," ] ] "]"

QualifiedHardwareEnumCase
    ::= HardwareEnumTypeName "." EnumCaseName

HardwareSymbolPath
    ::= Identifier | ImportedModuleQualifier "." Identifier
```

This is declaration-specific **restricted metadata syntax**, not an unrestricted struct initializer or arbitrary expression. Member separators and trailing commas are fully specified. The ordinary top-level declaration grammar must include this alternative. User member source spellings, enum names and enum cases must not accidentally resolve to ordinary runtime bindings.

**§ 8(2).** Normative example for the initial `MemoryMapped` contract:

```sec
hardware region PeripheralSpace {
    domain: "cpu-physical",
    kind: HardwareRegionKind.MMIO,
    base: 0x40000000,
    size: 0x10000,
    readable: true,
    writable: true,
}

hardware resource GpioOutputState {
    width: 32,
}

hardware endpoint GpioOutput {
    kind: HardwareEndpointKind.MemoryMapped,
    region: PeripheralSpace,
    resource: GpioOutputState,
    offset: 0x200,
    size: 4,
    alignment: 4,
    read_widths: [32],
    write_widths: [32],
    read_footprint: HardwareFootprintKind.Whole,
    write_footprint: HardwareFootprintKind.Whole,
    read_effect: HardwareReadEffect.NonDestructive,
    write_operation: HardwareWriteOperation.Replace,
    ordering: HardwareOrdering.Volatile,
    completion: HardwareCompletion.Issued,
    atomicity: HardwareAtomicity.Unspecified,
    context: HardwareAccessContext.Current,
    fault: HardwareFaultContract.Platform,
}

@address(GpioOutput)
let mut gpio: GpioOutputRegister
```

The register type and its primary type-owned `impl` must be separately valid. A declaration of `hardware endpoint GpioOutput` never creates a `GpioOutputRegister` type or the `gpio` binding.

**§ 8(3).** An enum value in source is qualified, e.g. `HardwareRegionKind.MMIO`; in TOML it is the exact string encoding, e.g. `"mmio"`. A hardware reference in source names a selected, imported-visible symbol or the local declaration. Source paths follow existing module import qualification and visibility; there is no fabricated `Platform.*` module alias.

**§ 8(4).** Hardware declarations only provide metadata. Parsing or selecting them must never perform hardware I/O or cause general runtime allocation or initialization.

---

## § 9. `.sec/sec.toml` tables, exact override selector and exports

**§ 9(1).** B introduces exactly three manifest table families, preserving all existing single-manifest and strict unknown-key rules:

```toml
[hardware.region.<project-key>]
[hardware.resource.<project-key>]
[hardware.endpoint.<project-key>]
```

`<project-key>` is the existing case-sensitive `ProjectKey`, `[a-z][a-z0-9_-]*`. Every table may additionally contain `variants = ["<project-key>", ...]` to scope applicability. The list must be nonempty when present; an omitted list means all compatible selected variants. A referenced region/resource must be applicable wherever an endpoint using it is applicable.

**§ 9(2).** Exactly one of these table modes applies:

1. **Independent definition:** no `override` key; all properties required for that category must be present.
2. **Project override:** `override` selects exactly one A declaration of the **same category**; the table may provide only changed properties. Missing properties inherit unchanged. The selected source declaration must be in the eligible module graph and visible/applicable as required by project authority.

**§ 9(3).** **Canonical TOML override selector** (an exact schema definition, not a Sec source expression):

```toml
[hardware.endpoint.gpio-output]
name = "GpioOutput"
override = { module = "drivers/gpio", declaration = "GpioOutput" }
offset = 768
```

- `override` has TOML type **inline table** containing **exactly** `module: string` and `declaration: string`.
- `module` is a canonical import-path spelling resolved by the project's existing module/import-root resolver to **one and only one** selected canonical `ModuleIdentity`. It is **not** the module's short `ModuleName`, a file path, a user-defined string alias, or a guessed namespace.
- `declaration` is an exact case-sensitive Sec identifier resolved in that module's hardware declarations of the **same table category**.
- If the canonical import path is ambiguous between import roots, missing, inaccessible, or not selected, the compiler reports an error and asks for an unambiguous supported project module selection; it must never resolve by display name or enumeration order. No alternate ad hoc `root` property is invented here.
- An override selector is invalid if it names a project declaration, names an incorrect category, or attempts to replace more than one declaration.

**§ 9(4).** `name` is an optional `string` containing a **valid Sec identifier** on an endpoint table. If present, it explicitly exports this effective endpoint as `Project.<name>`; duplicate exported names in the same effective project scope are errors. `name` does **not** rename the overridden source identity. Region/resource names remain source symbols or manifest keys and have no automatic `Project.*` endpoint export.

**§ 9(5).** A project override is **per-property**. Each supplied property is type-checked against exactly the same schema used in Sec source; inherited values are resolved by canonical source identity, never by copying raw source text or implicitly applying last-wins precedence. A changed value generates one warning identifying the property, source value/location, new project value/location and effective Target/Variant. Identical values are silent. An invalid override produces an error with no fallback to the source value.

**§ 9(6).** TOML int is signed 64-bit in TOML 1.0. To preserve arbitrary-precision domain addresses in TOML, the canonical accepted B spelling for `HardwareDomainAddress` and `HardwareDomainExtent` is either a nonnegative TOML integer **or** a **string containing exactly one unsigned decimal or `0x` hexadecimal integer literal**, with no whitespace, sign or exponent. Both normalize to the same exact mathematical value. Ordinary `uint` members (`width`, `alignment`, `read_widths`, `write_widths`) remain TOML integers and are checked against target `uint`; no implicit string-to-uint conversion is admitted for them.

**§ 9(7).** B enum values are **exact strings** matching §4A and §7, not bare integers or variant names. Symbolic `region`/`resource` members use exact canonical manifest keys or an explicitly qualified, resolvable A declaration reference. If B refers to an A declaration, the resolver must preserve the canonical module/declaration identity; no bare name search across modules. For this v0.1 schema, use TOML inline selectors `{ module = "drivers/gpio", declaration = "PeripheralSpace" }` when a source symbol is referenced from B. Bare TOML string references resolve only to B table keys of the expected category. Category mismatches and ambiguities are errors.

**§ 9(8).** Illustrative *independent* B definition (all required members supplied):

```toml
[hardware.region.peripheral]
variants = ["board-m4"]
domain = "cpu-physical"
kind = "mmio"
base = "0x40000000"
size = 65536
readable = true
writable = true

[hardware.resource.gpio-output-state]
variants = ["board-m4"]
width = 32

[hardware.endpoint.gpio-output]
name = "GpioOutput"
variants = ["board-m4"]
kind = "memory-mapped"
region = "peripheral"
resource = "gpio-output-state"
offset = 512
size = 4
alignment = 4
read_widths = [32]
write_widths = [32]
read_footprint = "whole"
write_footprint = "whole"
read_effect = "non_destructive"
write_operation = "replace"
ordering = "volatile"
completion = "issued"
atomicity = "unspecified"
context = "current"
fault = "platform"
```

**§ 9(9).** Unknown `hardware.*` tables/keys, duplicate table identities, invalid override selectors, mismatched JSON-like/TOML types, duplicate endpoint export names, mismatched Variant applicability and conflicting platform constraints are errors. The project editor preserves comments and unrelated tables; no second hardware manifest is created.

---

## § 10. Address bindings, alias warnings and explicit acknowledgement

**§ 10(1).** The **only** Sec 0.1 static `@address` forms for hardware register bindings under this decision are:

```sec
@address(GpioOutput)
let mut gpio: GpioRegister

@address(Project.GpioOutput)
let mut projectGpio: GpioRegister

@address(Project.GpioOutputAlias, alias)
let mut acknowledgedAlias: GpioRegister
```

The first argument must resolve statically to a **named canonical endpoint descriptor**. A numeric constant, arithmetic address expression, raw pointer, variable holding an address or runtime-computed endpoint is invalid. The `alias` argument is a compiler-known **contextual marker inside `@address` only**, not an ordinary expression, runtime boolean, declaration-wide annotation or reserved global source identifier. `@address` accepts exactly one endpoint argument plus this optional second marker; a third argument, unknown second marker or duplicated `alias` is invalid.

**§ 10(2).** `alias` acknowledges the compiler's proven relationship between *this binding's endpoint* and **at least one other statically known binding** to the **same canonical resource** in the selected compilation. The explicit acknowledgement is invalid if no such related binding exists in the relevant statically analyzable binding set; it must not fabricate physical resource identity. If another resource is merely *suspected* due to an overlapping address but has a different unproven logical identity, the programmer must correct the hardware facts rather than suppress that diagnostic with `alias`.

**§ 10(3).** A compiler warning must identify each **detectable, unacknowledged** pair of bindings that share the same resource, or whose proven or possible physical footprints overlap in an applicable common address domain/context. Diagnostics should distinguish (a) proved same-resource alias; (b) address overlap without established shared-resource semantics; and (c) incompatible physical overlap. Acknowledgement silences **only (a)** for the marked binding pair(s); (b) remains a warning (and can be an error when safe planning requires a missing fact); (c) remains a compile-time error. A physically plausible alias that cannot be established by available compile-time information is never silently treated as independent.

**§ 10(4).** A project/canonical hardware description may explicitly declare different endpoints to the same resource, including set/clear aliases. That is a **hardware fact**, not a source `@address` acknowledgement. It is legal for two distinct endpoints to describe the same resource when their operations/footprints are valid. No warning is required merely because two endpoint *declarations* exist; the warning concerns simultaneous potentially conflicting *bindings* and their uses.

**§ 10(5).** No `alias` or project override relaxes borrow exclusivity, lifetime, mutability, allowed read/write operations, shadow correctness, register field permissions, side-effect safety or ordering. It grants no general permission for two mutable software views of otherwise exclusive state. Where a shared-resource access plan cannot prove its obligations, compilation fails closed even when the programmer acknowledges the alias.

**§ 10(6).** Region overlap warnings are separate from binding alias acknowledgements: two regions with different names may overlap and elicit a warning without asserting identical resource identity. `@address(..., alias)` cannot waive malformed region contracts or unsafe overlapping physical binding plans.

**§ 10(7).** The existing source `@address` numeric examples and associated numeric-bind acceptance rules in `fixed-address-bindings.md`, `hardware-register-access.md`, `attributes.md`, grammar, tests and help text are superseded for **static hardware bindings** in v0.1. Maintain the existing distinct checked runtime-mapping and explicit unsafe RawPtr models; they do not become an alternative way to forge a compiler-known hardware endpoint contract.

---

## § 11. Operation planning and target capabilities

**§ 11(1).** A normalized endpoint contract does not itself select a physical instruction. A verified `PhysicalOperationPlan` is derived for each requested logical operation from selected Target/Device/platform facts and the endpoint/resource/region capabilities, binding/access context, field semantics and exact effects.

**§ 11(2).** The first executable path may cover fixed, byte-addressable `MemoryMapped` endpoints with one legal whole-register transfer width, target-compatible alignment and layout, and **non-destructive full-register observations** or **replacement writes** whose requested logical meaning is provably preserved. These operations must be exact physical steps, not inferred unsafe RMW.

**§ 11(3).** No guarantee is fabricated for missing atomicity, ordering barriers, completion, fault recovery, targeted field effects, read-clear behavior, set/clear aliases or multi-step device protocols. A source or manifest declaration inconsistent with known platform facts is invalid; a semantically valid but as-yet unsupported endpoint plan yields an explicit unsupported-capability diagnostic. `unsafe` does not override a known invalid target fact.

**§ 11(4).** Source and B hardware facts may refine the selected model but cannot alter the CPU instruction set, ISA address width, ABI, architectural byte order, privilege/security model or required Device restrictions. Distinct operation plans are versioned/fingerprinted against a single immutable `CompilationPlan`.

---

## § 12. Plan assembly, module scope and deterministic selection

**§ 12(1).** Compile-time A facts and B tables feed a **restricted two-stage plan**:

```text
1. Resolve target/variant, platform architecture and baseline selection facts.
2. Select source modules without depending on A hardware declarations.
3. Collect selected A hardware declarations in a declaration-only metadata pass.
4. Parse B, apply exact source-identity overrides, normalize and validate all facts.
5. Freeze one immutable CompilationPlan and canonical hardware fingerprint.
6. Run ordinary Sema; validate register types, bind endpoints and prove requested operations.
7. Produce Semantic IR and verified Sec MLIR/lowering operations from that plan.
```

**§ 12(2).** Hardware metadata may not influence its own source selection, imports, active Variant selection or `@when` inclusion. Arbitrary compile-time execution, runtime observation, platform probing and side-effecting project hooks are disallowed during hardware fact normalization.

**§ 12(3).** Project partial-override resolution is deterministic and does not depend on TOML table order, import source enumeration order, hash-table iteration, process scheduling or diagnostic ordering. Effective property provenance and source-visible export names are retained across separate compilation as required by the existing module model.

---

## § 13. Semantic IR, diagnostics, LSP and conformance

**§ 13(1).** Canonical Semantic IR preserves separate IDs for resource, endpoint, binding, operation and verified physical plan, target/plan compatibility facts, capability/effect/footprint facts, alias evidence and explicit hardware-side effects. Detached `register[N]` values remain pure value operations; physical register bits are not ordinary `Place`s. MD-110/MD-114 register-definition encoding remains canonical and must not be replaced with a second hardware-metadata register table.

**§ 13(2).** Diagnostics must say whether a declaration fails because of a wrong member type, an unknown enum case, an unknown domain, an invalid address extent, an inaccessible/ambiguous source override selector, a missing required endpoint capability, an incompatible region overlap, or an unacknowledged/invalid alias. Warned property changes report old and new values with provenance. Warning severity is not elevated merely because a property differs; failed validity checks are independent errors.

**§ 13(3).** LSP and the formatter use the **same** compiler metadata schema as Sema. Completion/hover expose all allowed members, exact types, requiredness, enum cases and semantics; source-definition navigation resolves `hardware region/resource/endpoint` names and manifest override selectors to canonical identities. LSP also reports target-domain selection and alias/override facts. Neither LSP nor formatter maintains a separate shadow version of the schema.

**§ 13(4). Required positive tests.** A/B equivalent schema normalization, same-source and cross-module references, explicit project override of a single property, override with identical value, wide domain address beyond CPU `uint`, region `base + size` boundary success, explicit endpoint exports, distinct endpoints sharing one resource, `@address(..., alias)` on a proven binding alias, and no runtime hardware I/O caused solely by declarations.

**§ 13(5). Required negative tests.** Unknown/missing/duplicate members, every wrong member type, every unknown enum value, wrong enum namespace, invalid `uint`/domain address, overflow/wraparound, invalid read/write widths and alignment, wrong units, unknown domain/region/resource, target/Device conflict, wrong Variant applicability, unavailable endpoint kind, unsupported operation plan, scope-invisible override, ambiguous import path, duplicate effective override, numeric `@address`, `@address(..., unexpected)`, unproven `alias`, incompatible physical overlap, and unsafe capability fabrication.

**§ 13(6). Required differential tests.** Change project property values and effective A source declarations under incremental compilation; compare to independently rebuilt cold results. Ensure changes invalidate exactly the affected plan/capability layers and that comments or nonsemantic reorderings do not alter canonical fingerprints. Check cross-Target/Variant isolation and parallel determinism.

---

## § 14. Controlled rulebook integration and package handoff

**§ 14(1).** Integrate this decision by *additive numbered amendments* into existing canonical rulebooks. Do not change numbering already cited from compiler diagnostics or governance. Exact implementation source and tests must follow the current repository layout, not the examples' illustrative filenames alone.

**§ 14(2).** Integration owners:

| Authority | Required integration |
|---|---|
| `rules/platform/platform_model.md` | Domain identity and capability authority; typed hardware fact model, validity and A/B normalization. |
| `rules/platform/hardware-register-access.md` | Typed regions/resources/endpoints, physical operation/effect plans, alias identities, no inferred hardware I/O, interaction with MD-112/113. |
| `rules/platform/fixed-address-bindings.md` | Named-endpoint-only `@address`, optional `alias` marker, binding warning/error policy and numeric syntax prohibition. |
| `rules/declarations/registers.md` | Explain independent logical `register[N]` type versus `Region`/`Resource`/`Endpoint`; type-owned `impl` retains hardware access boundary. |
| `rules/projects/projects.md` | Strict `.sec/sec.toml` hardware table grammar, export name, `variants`, override selector, integer encoding and per-property warning. |
| `rules/projects/modules.md` | Symbol visibility and canonical module identity for source hardware declarations and override resolution. |
| `rules/foundations/grammar.md` / `lexical_structure.md` | Contextual `hardware` grammar; closed restricted metadata expressions, symbol selectors, enum case syntax. |
| `rules/foundations/attributes.md` | Expanded `@address(Endpoint[, alias])` arity and marker contract, not a new `@alias` attribute. |
| `rules/compiler/semantic_ir.md` and MLIR | Separate IDs and verified operation facts, no raw-address lowering without an approved plan. |
| `rules/tooling/lsp.md` / formatter / diagnostics | One typed source of member completion, hover, override provenance and warnings. |
| Governance and P15 packages | Reconcile with existing integration IDs; implement and test one slice at a time; keep pending until verified. |

**§ 14(3).** The previously written MD-111 2026-10-27 proposal remains useful background, but its clauses prohibiting project override and accepting numeric `@address` are **superseded** by the approved owner decisions recorded here. Do not merge its obsolete behavior back into the rulebooks. MD-112 and MD-113 are decided separately and must be referenced, not reopened.

**§ 14(4).** MD-111 design is **decided**, approved by the Sec language owner on 2026-10-30. Preserve the `missing-decisions.yaml` entry until its applicable normative integration/governance conditions are met under the existing ledger policy. Do not conflate approval, integration, backend acceptance and full implementation. MD-081 is separate and next in the agreed work sequence.

---

## § 15. Owner approval record

**§ 15(1). Approved in the preceding MD-111 design discussion:** source/manifest authoring, explicit project precedence and partial override, changed-property warnings, named-only `@address`, `@address(Endpoint, alias)`, three hardware declaration categories, known target address domains, domain-sized address metadata (policy B).

**§ 15(2). Approved with the consolidated document on 2026-10-30:** precise closed metadata enums, exact required/optional member tables, domain-sized region/endpoint extents, exact restricted source syntax, exact TOML `override` selector and source-symbol reference syntax, strict endpoint export naming, TOML wide-integer text encoding, conservative first supported operation subset, and complete diagnostics/tests/integration obligations.

**§ 15(3).** No unchosen language-design branches remain inside this approved v0.1 contract. Any later broader footprint/context/operation model must be a separately identified amendment rather than a guessing exercise for the implementation agent.
