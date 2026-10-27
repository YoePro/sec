# MD-111 — Source and project hardware facts, endpoint identities, and first verified MMIO plans

- **Status:** Design proposal — **not owner-approved; not normative; not integrated**
- **Created:** 2026-10-27
- **Last updated:** 2026-10-27
- **Document revision:** 0.1 (proposal for review)
- **Sec language version:** 0.1
- **Repository baseline:** `YoePro/sec` `main` at `5426af6cf8cce6fe85ff897b97050dfd330ef4fe` (2026-10-26)
- **Decision being addressed:** `missing-decisions.yaml` MD-111
- **Intended destination after approval:** `rules/corrections/`, then `applied/` only after integration
- **Primary owners:** `rules/platform/platform_model.md`, `rules/platform/fixed-address-bindings.md`, `rules/platform/hardware-register-access.md`, `rules/projects/projects.md`, `rules/compiler/semantic_ir.md`
- **Additional owners:** `rules/projects/modules.md`, `rules/compiler/compiler.md`, `rules/foundations/grammar.md`, `rules/foundations/lexical_structure.md`, `rules/foundations/attributes.md`, `rules/compiler/incremental_compilation.md`, `rules/tooling/lsp.md`, relevant platform and lowering governance fragments
- **Depends on:** MD-077 (hardware versus detached register operations), MD-114 (register-definition MLIR encoding), MD-110 (detached conversion); P15-42/42a
- **Separately open:** MD-112 (non-trigger specialized writes), MD-113 (shadow knowledge across calls), runtime-mapping API details, physical platform support beyond first MMIO plan
- **Future extension:** Knowledge packs (not a prerequisite for Sec 0.1; version/milestone not established by this proposal)

> **Review boundary.** The statements below are proposed MD-111 decisions, **not** claims that the repository already implements this syntax or schema. The source form `hardware ...` and all `[hardware.*]` TOML tables are **new proposed spellings**. Existing normative contracts and the design amendments proposed here are distinguished explicitly. No repository file has been modified.

---

## § 1. Scope and non-goals

**§ 1(1).** Sec 0.1 admits two authorable declarative sources of hardware facts: **A**, compile-time hardware declarations in selected Sec source modules, and **B**, project-owned hardware tables in the single `.sec/sec.toml` manifest. Both normalize into the same canonical hardware-fact registry and ultimately the selected immutable `CompilationPlan`.

**§ 1(2).** **C**, future knowledge packs, is reserved as a third *input provider* of the same canonical model. No knowledge-pack parser, filename, schema, installation workflow, trust mechanism, or source syntax is required by MD-111 for Sec 0.1. Future C may not redefine the meaning of A/B facts.

**§ 1(3).** MD-111 describes hardware location, resource identity, endpoint capability, plan prerequisites and address validation. It does **not** change the Sec `register[N]` type, bits/endianness, field read/write semantics, ownership/borrow/lifetime rules, or the existing `@address` grammar. It does not add ordinary source access to physical bits, new runtime construction, untyped MMIO, implicit driver protocols, or an LLVM pointer representation.

**§ 1(4).** The normative separation in MD-077 survives: a hardware-bound register produces `HardwareInstance[T]` with `register.hw.*` operations; a detached `T` is an ordinary software value. Memory certified as ordinary reference-compatible addressed storage may use the P15 Place path, subject to independently established reference/lifetime authority.

**§ 1(5).** A static source definition must not execute a hardware operation, probe target memory, allocate a runtime object, invoke arbitrary CTE code, run a project hook, or affect the runtime simply because the definition exists.

## § 2. One canonical resolved model

**§ 2(1).** The compiler has one normalized, typed, immutable model, regardless of the origin of each accepted declaration:

```text
HardwareFacts {
    AddressDomains[]
    MemoryRegions[]
    HardwareResources[]
    HardwareEndpoints[]
    PhysicalOperations[]
    Provenance[]
}
```

`HardwareFacts` is a conceptual interface, not a mandated Go struct.

**§ 2(2).** Definitions pass through the same stages: parse → source attribution → canonical identity registration → structural validation → cross-domain constraint validation → selected Target/Variant applicability → immutable plan integration → operation-plan resolution. A and B must not implement different validation or planning semantics.

**§ 2(3).** `ArchitectureModel`, `MemoryEnvironment`, `DeviceModel` and existing platform facts remain distinct owning domains. A user-supplied region or endpoint refines *available facts*; it may not change the CPU instruction set, native address width, ABI, architecture endianness, security domain or required Device constraints.

**§ 2(4).** An accepted project/source description is a declared hardware **contract**, not an empirical proof that real-world hardware matches the description. The compiler validates the contract and verifies accesses *against it*. A malformed, incomplete or conflicting contract fails closed; it cannot silently override a known platform prohibition.

**§ 2(5).** A declaration's source/provenance is retained for diagnostics and explanation. Runtime semantics are determined by resolved canonical facts, not by whether the facts came from A, B, built-ins or eventually C.

## § 3. Canonical identities, aliasing, and deterministic composition

**§ 3(1).** These are distinct identity domains:

```text
AddressDomainIdentity
MemoryRegionIdentity
HardwareResourceIdentity
HardwareEndpointIdentity
PhysicalOperationIdentity
HardwareBindingIdentity
VerifiedOperationPlanIdentity
```

A memory address, text name, register nominal type, implementation instance or SSA value never substitutes for any of these identities.

**§ 3(2).** Each authored definition has a canonical symbolic key comprising its declaring namespace, kind and local key. Project entries belong to the project namespace, which is associated with the canonical ProjectUUID. Source entries belong to the canonical ModuleIdentity namespace. The active Target/Variant scopes *applicability and resolved instances*, not source identity by incidental file enumeration order.

**§ 3(3).** Stable symbolic identities are maintained independently of compiler-local numeric IDs. Numeric `HardwareResourceID`, `HardwareEndpointID`, etc. may be assigned deterministically per resolved snapshot, but are not stable external names and must never be derived from hash-table iteration, raw memory addresses or process scheduling.

**§ 3(4).** Two endpoints may reference the **same** resource if their operations affect the same logical hardware state. Alias relations must be explicit through shared canonical `HardwareResourceIdentity` and operation footprints. Equal or nearby addresses never create inferred aliases; different addresses may intentionally update the same resource.

**§ 3(5).** Duplicate effective definitions of the same canonical key are rejected unless a separately defined, explicit compatible extension mechanism applies. Sec 0.1 MD-111 introduces **no implicit override mechanism**. Source declarations do not shadow project declarations; project declarations do not override built-in restrictions. A missing field is not filled with the last declaration found.

**§ 3(6).** Region containment and compatibility are verified in the selected address domain and context. Distinct numeric address domains remain distinct even when their numerical locations match. Ambiguous overlapping bindings are rejected unless a canonical explicit alias/view relationship permits them.

## § 4. Address domains and region kinds

**§ 4(1).** An address domain supplies canonical identity, applicable width and addressability, whether it has a selected **linear numeric hardware address form** usable by `@address(integer)`, and required platform/address-space constraints. A port, CSR or instruction selector does not become MMIO because it is representable as an integer.

**§ 4(2).** A region has at least identity, domain, base and exclusive end/extent, memory kind, read/write permissions, alignment constraints where applicable, applicability, and source provenance. Ranges are validated with arbitrary-precision arithmetic before conversion to the target's address width. Zero extent, wraparound and out-of-domain addresses are rejected.

**§ 4(3).** Initial region-kind vocabulary proposed for 0.1:

```text
mmio        // Hardware state: not ordinary reference-compatible memory.
ram         // Candidate ordinary addressed storage; separate lifetime/authority proof required.
rom         // Candidate read-only addressed storage; separate lifetime/authority proof required.
reserved    // Not bindable through @address.
```

An implementation may retain finer architectural kinds internally. These spellings are proposed input schema values, not changes to source `register[N]` kinds.

**§ 4(4).** Declaring `ram`/`rom` does not by itself prove safe reference lifetime, exclusive borrow authority, mapping lifetime, writable mutability, or synchronization. To enter the ordinary addressed P15 Place branch, the selected contract must **also** explicitly establish reference-compatible storage, availability/lifetime and applicable access authority. Unknown facts do not authorize the branch.

**§ 4(5).** An MMIO region is a containment and constraint contract, **not** a default assertion that arbitrary loads/stores within it are safe or semantically complete. A specific endpoint or separately declared, fully specified region-default transfer contract must authorize each physical access.

**§ 4(6).** Source `@address` remains hardware-facing under `fixed-address-bindings.md` §4: it does not silently discard any existing volatile/ordered-access obligations merely because a region is classified as RAM. Ref-compatible RAM additionally needs the ordinary Place/reference validity requirements; the detailed volatile-with-reference transport must be verified during P15 implementation.

## § 5. Minimum MemoryMapped endpoint and physical operation facts

**§ 5(1).** A first Sec 0.1 verifiable MemoryMapped endpoint must resolve at least:

| Area | Required resolved facts |
|---|---|
| Identity | endpoint key/ID, logical resource key/ID, declaring source |
| Location | canonical address domain, region, base/offset and total bound extent |
| Register compatibility | independently checked logical register width, byte order at access boundary and layout compatibility |
| Transfer | explicit legal read widths and/or write widths, required alignment, per-direction availability |
| Effects | operation kind (observe, replace, set, clear, toggle), exact read/write footprints and collateral effects |
| Semantics | declared legal permissions and preserved field access/trigger meaning from `registers.md` |
| Context | allowed privilege/execution/security context, or explicit dependency on applicable platform context |
| Ordering | explicit volatile occurrence and any stronger required order; no implicit hardware barrier |
| Completion | declared completion requirement, including whether mere issue is sufficient for this operation |
| Atomicity/coherence | explicitly proven guarantees or the canonical absence of such guarantees; no width-derived assumption |
| Failure | applicable platform/target fault contract; legal fault-capable access is not an automatic Result |
| Target | applicability and requisite architecture/Device capability, source provenance |

**§ 5(2).** For the simplest verifiable first plan, a *single fixed-width whole-register read or replacement write* may be defined with one endpoint, one physical transaction, one full-register footprint, fixed alignment and explicit non-destructive read semantics (if read is allowed). This does not automatically make **individual field writes** safe: the planner must still prove read-modify-write preserves reserved bits, special field behaviors and collateral effects.

**§ 5(3).** A named endpoint MAY provide more capable operations (e.g., set/clear aliases); these must name actual operations and their effects. The compiler may not synthesize such capabilities from names, addresses or nearby offsets.

**§ 5(4).** An omitted semantic guarantee (atomicity, coherence, device completion, safe partial writes, non-destructive reads, access-context privilege) is treated conservatively as **unproven**, not as true. Where it is necessary to satisfy a requested operation, plan creation fails with a diagnostic naming the missing fact.

**§ 5(5).** An endpoint resource may outlive a single binding and may be accessed through multiple explicit endpoint aliases. Register type width and field access rules are checked against physical operations; the endpoint never redefines the nominal type.

## § 6. A — hardware declarations in Sec source (proposed grammar)

**§ 6(1).** A uses a new **declaration-level, compile-time-only** form. I recommend contextual `hardware` declarations, not user-callable runtime constructors and not arbitrary new attributes. This is intentionally a new grammar addition for review; no existing Sec parser is claimed to accept it.

**§ 6(2).** Illustrative proposed declarations (all code English):

```sec
// Proposed syntax, not yet accepted by the compiler.
hardware region PeripheralSpace {
    domain: "cpu-physical",
    kind: mmio,
    base: 0x40000000,
    size: 0x10000,
    readable: true,
    writable: true,
}

hardware resource GpioOutputState {
    width: 32,
}

hardware endpoint GpioOutput {
    region: PeripheralSpace,
    offset: 0x200,
    size: 4,
    resource: GpioOutputState,
    alignment: 4,
    read_widths: [32],
    write_widths: [32],
    read_footprint: whole,
    write_footprint: whole,
    read_effect: non_destructive,
    write_operation: replace,
    ordering: volatile,
    completion: issued,
    atomicity: unspecified,
    context: current,
    fault: platform,
}

@address(GpioOutput)
let mut gpio: GpioOutputRegister
```

`GpioOutputRegister` must independently be a valid nominal `register[32]` and must meet the ordinary `impl` and hardware-access rules. The endpoint does not declare or implement that register type. `context: current` and `fault: platform` mean a dependency on the verified target/plan's existing contracts, not that the programmer may forge privilege or fault guarantees.

**§ 6(3).** Proposed source grammar outline:

```text
HardwareDeclaration := "hardware" ("region" | "resource" | "endpoint") Identifier HardwareFactBody
HardwareFactBody    := "{" HardwareFactMember* "}"
HardwareFactMember  := Identifier ":" HardwareConstant ","?
HardwareConstant    := IntegerLiteral | StringLiteral | BooleanLiteral
                     | CanonicalName | HardwareConstantList | ApprovedRangeLiteral
```

The parser must define precise delimiters, list and range grammar in the grammar/lexical rulebooks before admitting the feature. Unsupported expressions, executable statements, helper functions and implicit CTFE calls are errors.

**§ 6(4).** The `hardware` token may be contextual at a declaration boundary, following Sec's canonical lexical rules; this proposal does not require globally reserving a new identifier without a lexical compatibility audit. Definitions are module-scoped compile-time metadata. Normal module visibility/imports govern name resolution; only definitions actually in the selected, reachable source graph participate.

**§ 6(5).** A source endpoint resolves as an endpoint descriptor for `@address`, not as an ordinary integer or a runtime value. The compiler binds the source spelling using canonical ModuleIdentity plus the declared name. A local module may use `@address(GpioOutput)`; other modules use the normal resolved module-qualification mechanism, not a fabricated `Platform.*` alias.

**§ 6(6).** Generic hardware declarations, runtime-computed locations, source-dependent `let` values and arbitrary user-defined functions are **not** admitted by this initial syntax. Hardware integer constants use compile-time literal expressions or a narrowly defined pure constant grammar; source values may not depend on hardware reads. Future expressive definitions require a separate explicit language decision.

**§ 6(7).** A source definition that claims a forbidden target domain, conflicts with a selected Device, or describes an impossible transfer is rejected even if the source is well formed. Source code never expands the target's verified hardware execution capability by assertion alone.

## § 7. B — hardware tables in `.sec/sec.toml` (proposed schema)

**§ 7(1).** B extends the project's **existing single** `.sec/sec.toml` manifest. No separate project manifest or implicit hardware-directory scan is introduced. `rules/projects/projects.md` §6(2) already permits separately owned manifest sections.

**§ 7(2).** The proposed table families are `[hardware.region.<key>]`, `[hardware.resource.<key>]` and `[hardware.endpoint.<key>]`, where `<key>` is a canonical `ProjectKey`. Their semantics normalize to the exact same model and constraints as A. The owning project/platform rulebooks must explicitly register these tables so unknown-key rejection stays strict.

**§ 7(3).** Illustrative proposed manifest:

```toml
# Proposed manifest tables; not yet accepted by Sec 0.1.
[hardware.region.peripheral]
variants = ["board-m4"]
domain = "cpu-physical"
kind = "mmio"
base = 1073741824
size = 65536
readable = true
writable = true

[hardware.resource.gpio-output-state]
variants = ["board-m4"]
width = 32

[hardware.endpoint.gpio-output]
name = "GpioOutput"
variants = ["board-m4"]
region = "peripheral"
offset = 512
size = 4
resource = "gpio-output-state"
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

The TOML numbers above are illustrative exact integers. The future canonical parser must explicitly specify how larger-than-TOML-i64 address constants are represented without losing precision; an optional canonical decimal/hex string address encoding may be required if a selected platform supports them.

**§ 7(4).** The existing named-endpoint syntax remains valid:

```sec
@address(Project.GpioOutput)
let mut gpio: GpioOutputRegister
```

**Editorial issue to resolve on integration:** the existing examples use `Project.FpgaControl`, while machine-facing `ProjectKey` permits hyphens and lower-case keys. A normalized **export name** must be explicitly defined; for 0.1 I propose that each `[hardware.endpoint.<key>]` include a required Sec-identifier `name` (for example `name = "FpgaControl"`) when exported through `Project.<name>`. `Project.GpioOutput` above uses that explicit export and remains separate from the TOML key `gpio-output`. Duplicate exported names are errors. This keeps TOML storage keys distinct from Sec identifiers.

**§ 7(5).** Applicability can be declared by a non-empty `variants` list for a project-specific source. A definition is included only for matching resolved Variants; all its referenced resources and regions must also be applicable. Reject orphan references, unknown variants and duplicate effective identities. Omitted `variants` applies to all variants only when facts are valid and compatible in every selected plan that uses them; otherwise explicit scoping is required.

**§ 7(6).** A target-independent project endpoint can be declared once and reused by multiple compatible Variants. A Variant-specific endpoint may have a distinct canonical key or an explicitly separate variant-qualified instance. MD-111 does not grant a general per-key override/merging mechanism based on `[variant]` settings.

**§ 7(7).** Manifest editing must preserve comments and unrelated TOML sections as already required by `projects.md` §5. Strict schema validation reports path/table/key and targeted diagnostic; it does not silently ignore malformed facts.

## § 8. A/B parity, precedence, and future C

**§ 8(1).** A and B both register **declarations**, not ordered override instructions. A particular target-supplied region restriction always constrains any user-defined endpoint. A source definition and a manifest definition with different canonical identities may coexist and refer to the same resource only through explicitly declared identity/alias relations.

**§ 8(2).** When two declarations claim the *same* identity, the resolver either (a) confirms they are the exact same canonically shared imported declaration, or (b) rejects the duplicate. There is no "last wins," "source wins," "manifest wins" or undocumented merge of incomplete contracts.

**§ 8(3).** General project configuration precedence (`projects.md` §25) still controls which Target/Variant/options are selected. It does **not** convert conflicting physical facts into harmless override-able user preferences.

**§ 8(4).** Future knowledge packs may publish descriptions under their own canonical namespaces, optionally bringing in related region, resource and endpoint facts. Exact provider identity, versioning, signatures/trust and import syntax remain future decisions. The A/B normalized model must reserve unambiguous origins and fingerprinting sufficient to admit such a provider later.

## § 9. Two-stage plan assembly for source-declared facts

**§ 9(1).** Source A creates a potential dependency cycle because `CompilationPlan` must be frozen before ordinary Sema and lowering consume it. Resolve that by a constrained two-stage plan process, not by allowing Sema to mutate a frozen plan.

**§ 9(2).** Proposed deterministic stages:

```text
I.   Resolve Target/Variant and platform/architecture baseline facts.
II.  Select the eligible source graph using only facts independent of A hardware declarations.
III. Parse selected module hardware declarations in a limited declaration-only pass.
IV.  Normalize/merge A, B, platform and Device fact registrations; validate references and compatibility.
V.   Freeze the final immutable CompilationPlan with a canonical hardware fingerprint.
VI.  Run ordinary Sema (including register field typing) and verify each @address binding.
VII. Build immutable physical operation plans; emit Semantic IR and Sec MLIR using these facts.
```

**§ 9(3).** A hardware declaration cannot influence Target/Variant selection, its own module inclusion, imports or `@when` source selection. This prohibition breaks the resolution cycle. A future feature requiring hardware-fact-sensitive source routing needs an explicit separate resolver design.

**§ 9(4).** Pre-plan declaration collection does **not** have to resolve nominal Sec register type definitions. Endpoints describe physical widths, resources and transfers. Sema later validates the typed register bound through `@address` against the already-frozen endpoint facts. This prevents a second Sema/CompilationPlan dependency cycle.

**§ 9(5).** Both project and source declarations are plan inputs, never runtime startup calls. An invalid definition fails before publication of a completed plan; failed plan or IR publication must not leave partially trusted caches or half-written MLIR output.

## § 10. `@address` resolution and numerical fallback

**§ 10(1).** A named `@address(Project.X)`, module-qualified A endpoint or supported `Platform.X` resolves directly to the canonical endpoint descriptor; it is **not** replaced by a raw integer before identity and hardware contract verification.

**§ 10(2).** `@address(integer)` remains legal only in the target's explicitly selected canonical **linear** hardware address domain, as existing `fixed-address-bindings.md` §8 and `hardware-register-access.md` §22 require. An arbitrary unmapped numeric address is rejected.

**§ 10(3).** A numeric binding may resolve through a **declared containing region** without a separately named endpoint only when the region supplies an explicit, complete **default transfer contract** for the entire bound extent. If the region supplies only range/kind/permissions, there is not enough information to form a verified hardware physical operation plan: reject rather than assume general-purpose volatile loads/stores are legal.

**§ 10(4).** The resolution of a numeric binding must be unique in the active domain/context. Explicit named endpoint(s) may narrow it to one known operation contract, but a numeric literal is never enough to infer set/clear aliases, W1C behavior, atomicity, implicit safety or read-side effects.

**§ 10(5).** The memory region's kind classifies hardware-bound register instance versus candidate ordinary addressed storage; type eligibility stays as in `fixed-address-bindings.md` §12 (Sec 0.1 `register[N]` only). A `ram`/`rom` classification is insufficient by itself: ordinary Place requires independently verified reference-compatible authority and lifetime. Otherwise fail closed, rather than silently categorizing it as MMIO.

**§ 10(6).** All bindings require alignment, extent, transfer widths, permission, access-context, overlap, declared alias and selected target capability checks. `unsafe` does not waive already-known invalid target facts. Hardware registers never admit `ref` directly to physical bits.

## § 11. Operation-plan identity and admissible first P15 slice

**§ 11(1).** The verified physical operation plan has a canonical identity separate from its resource, endpoint, binding and logical source operation. A plan is a validated instance of a particular requested action under a particular frozen `CompilationPlan` and may select exactly one or several physical steps only when the endpoint contract permits them.

**§ 11(2).** Plan identity incorporates the normalized semantic request, resource/endpoint/operation contract identities, selected Target/Variant/Architecture/Memory facts and the pertinent immutable plan fingerprint. Numeric IDs may be local and deterministic; content hashes/fingerprints must derive from canonical meaning, not filesystem order or timestamps.

**§ 11(3).** First executable P15 scope may implement only: fixed `MemoryMapped` single-transaction reads and replacement writes whose exact type/width/alignment/permissions/footprints/access context/fault/ordering requirements are representable and supported by the target. All other endpoint kinds and specialized effects must fail with precise unsupported-capability diagnostics, rather than being omitted from the canonical model.

**§ 11(4).** P15 can implement named endpoints and numeric bindings covered by explicitly complete linear-region default contracts. Whole-register read and write do not automatically establish safe field RMW, atomicity, completion or special semantics. MD-112 and MD-113 remain independent semantic/analysis work; this decision does not approve unsafe substitutions.

**§ 11(5).** Compile-time `hardware` declarations and TOML project facts do not by themselves assert that P15/P16 acceptance is complete. Implementation, MLIR schema support, full tests, governance and package closure are distinct milestones.

## § 12. Verification, diagnostics, incremental and tooling requirements

**§ 12(1).** Validation must reject missing/duplicate identities, impossible address widths or ranges, invalid/overlapping regions, illegal aliases, cycles in required references, unknown address domains, width/alignment mismatch, conflicting Device constraints, invalid field/enum/nominal compatibility and incomplete required physical operations.

**§ 12(2).** Project or source facts may narrow the set of available operations but may not override canonical target prohibitions or claim unsupported machine instructions. Unsupported target realization is diagnosed separately from semantically invalid declared facts.

**§ 12(3).** Errors identify the exact source declaration or `.sec/sec.toml` table/key, Target/Variant, affected endpoint/resource/binding, and the specific failed capability or validation rule. Unknown target metadata is not silently treated as a safe default.

**§ 12(4).** LSP reads the same model and can report unresolved endpoint names, cross-references, duplicate or conflicting declarations, and hardware-bound access diagnostics. It does not maintain a competing background interpretation of A or B.

**§ 12(5).** A change to selected A declarations or applicable B tables invalidates the relevant plan's hardware/memory fingerprint and dependent Sema/IR/lowering layers. Changes to comments or display labels that preserve normalized semantics need not invalidate target-independent caches. Cold recomputation remains a correctness fallback.

**§ 12(6).** Data must serialize deterministically; imports, concurrency, directory enumeration, TOML table order and hash maps must not change canonical resolution. A change of Target/Variant or selected Device must not reuse a physically incompatible cached operation plan.

## § 13. Conformance and negative test matrix

**§ 13(1).** Positive: one endpoint and resource defined by A; equivalent semantics via B; schema parity; named `@address` resolution; nested A module import; valid platform-owned physical MMIO region; valid whole-register observation; valid replacement write; 32/64-bit cross compilation where target support exists.

**§ 13(2).** Positive: multiple endpoints explicitly addressing the same resource through declared alias operations; same source type used as detached register and in hardware instance; source static declarations with target-agnostic parsing; no runtime initializer/reads for fact definitions.

**§ 13(3).** Negative: duplicate names; unknown region/resource; wrong Variant applicability; source versus project conflict; unselected source declaration; ambiguous numeric address; arbitrary numeric address with no covering region; incomplete default transfer contract; non-linear numeric domain; address overflow; misalignment; forbidden region; context mismatch.

**§ 13(4).** Negative: read-only/write-only mismatch; destructive read used as pure observation; illegal transfer width; unproven RMW to a restricted field; reserved/alias collateral corruption; unproven atomicity/coherence/completion; fictional `HardwareInstance` from an ordinary integer; physical resource treated as a `Place`.

**§ 13(5).** Plan and incremental: A/B equivalent normalized meaning yields equivalent capability semantics; metadata reorder and comments do not change semantic fingerprints; fact changes invalidate only necessary domains; no stale/reused plan is accepted after variant/device switches; cold and incremental results match; parallel resolution is deterministic.

**§ 13(6).** Negative: pre-plan A declaration depending on arbitrary CTE/runtime execution, source selection depending on hardware facts, resource typed as arbitrary target integer, project manifest second/nested manifest, inconsistent physical operation and `CompilationPlan` identity.

## § 14. Controlled integration and implementation order

**§ 14(1).** Do **not** renumber or replace existing normative sections. Insert additive clauses or clearly cited amendments while retaining stable cross-references.

**§ 14(2).** Apply in this order, one independently tested change at a time:

1. Add this decision's normative model and cross-references to `platform_model.md`, `hardware-register-access.md` §32, `fixed-address-bindings.md` §8/§12, `projects.md` §6, and `compiler.md` §7/§43–44.
2. Implement typed canonical `HardwareFacts` plus registry and validation tests, initially fed by explicit in-memory definitions in tests.
3. Implement B manifest tables with source-location-aware diagnostics; preserve `.sec/sec.toml` ownership and comment-preserving editor behavior.
4. Implement A contextual `hardware` grammar, source-only pre-plan collection and deterministic applicability; parser/formatter/LSP negative tests.
5. Implement two-stage plan freeze with canonical identities and domain fingerprints; unchanged code selection semantics.
6. Implement `@address` named and numeric binding resolution and storage-kind distinction; reject unsupported cases.
7. Implement a minimally capable MemoryMapped operation planner and P15 Semantic IR/MLIR bridge; cross-plan conformance and full package regressions.
8. Reconcile single-owner governance fragments and `missing-decisions.yaml` only when the accepted normative text has actually been synchronized.

**§ 14(3).** Hardware metadata records must not be improvised as a second register-definition table. MD-114 remains the nominal register-layout owner; MD-110 conversion proofs remain detached value operations; MD-112/MD-113 retain their own unresolved behavior questions.

**§ 14(4).** Mark MD-111 design-decided on owner approval. Remove its missing-decision record only after normative integration and governance synchronization under the existing `missing-decisions.yaml` policy. Continue tracking unimplemented P15 requirements in governance independently.

---

## Review synopsis (non-normative)

- **A:** contextual `hardware region/resource/endpoint` declarations, compile-time only.
- **B:** `[hardware.region.*]`, `[hardware.resource.*]`, `[hardware.endpoint.*]` in existing `.sec/sec.toml`.
- **C:** reserved future provider of the exact same resolved model.
- **Identity:** namespace + kind + name, distinct resource/endpoint/operation/plan IDs; no address-derived aliases.
- **Composition:** compatible additions and constraints, **no implicit overrides**.
- **Numeric addresses:** accepted only inside an explicitly supported linear domain with a covering, fully specified contract; never unrestricted by default.
- **Early executable implementation:** single-transaction MemoryMapped full-register reads/writes; advanced operations fail closed.
- **Main new compiler mechanism:** restricted source declaration collection before **final** `CompilationPlan` freeze, while the target/source-selection baseline is already known.

**Issues deliberately left for editorial validation, not assumed implemented:** exact parser tokens/delimiters and contextual keyword compatibility; manifest `Project.<SecIdentifier>` export naming; exact TOML encoding of address constants beyond TOML integer width; detailed formal grammar for nested endpoint-operation records; legal representation of optional target-specific context/fault references. These should be resolved in the normative integration without expanding runtime semantics, or explicitly returned for owner review if they alter the accepted contract.
