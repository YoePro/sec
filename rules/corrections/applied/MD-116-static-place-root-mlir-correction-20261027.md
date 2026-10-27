# MD-116 — Canonical Sec MLIR static Place roots and returned-static identity

- **Decision:** MD-116 — `sec-mlir-static-place-root-text-contract`
- **Status:** **Applied — normative integration and implementation verified 2026-10-27**
- **Created:** 2026-10-27
- **Last updated:** 2026-10-27
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reference repository:** `YoePro/sec`, `main` at `5426af6cf8cce6fe85ff897b97050dfd330ef4fe` (2026-10-26)
- **Applied correction location:** `rules/corrections/applied/MD-116-static-place-root-mlir-correction-20261027.md`
- **Archive after full normative integration:** `rules/corrections/applied/MD-116-static-place-root-mlir-correction-YYYYMMDD.md`
- **Primary normative owners:** `rules/mlir/sec_mlir_dialect.md` (schema-11 amendment), `rules/mlir/sec_mlir_lowering.md`
- **Related normative owners:** `rules/compiler/semantic_ir.md` §§ 10, 28, 45; `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md` (append-only amendment); `rules/mlir/lowering-versions/sec_mlir_lowering_v11.md` (append-only amendment); `rules/mlir/packages/sec-mlir-dialect_package15.md`; `rules/declarations/static.md`
- **Tracking:** `missing-decisions.yaml` MD-116; `package15-todo.md` P15-42, P15-42a, P15-47, P15-75; existing `governance/lowering.yaml` integration `lowering.sec-mlir-package15`
- **Related decisions:** MD-084 (canonical reference/Place metadata), MD-085 (returned-origin verification), MD-087 (P5/P6/P8 boundaries), MD-081 (source legality of static references), MD-111 (addressed hardware/region facts), MD-114 (register textual contract)
- **Decision authority:** Approved by the Sec language owner on 2026-10-27.

> **Integration contract:** This document records the applied normative correction and its verified high-level schema-11/12 implementation. Physical reference lowering remains outside this correction. It must be synchronized into the named canonical rulebooks without renumbering existing paragraphs. Move the correction to `applied/` only after normative integration. Keep P15 and its governance entry `partial` until implementation and package acceptance evidence satisfy their separate criteria.

---

## § 1. Purpose, authority, and non-goals

**§ 1(1).** MD-116 specifies the missing *high-level* Sec MLIR representation of the existing Semantic IR `place.static-root` operation. The canonical MLIR spelling is **`sec.place.static`**. It denotes a Place rooted in an already declared static storage object and never constructs storage by itself.

**§ 1(2).** Sema is authoritative for static-binding legality, borrow/lifetime validity, mutation permission, synchronization, and reference escape. Semantic IR preserves those resolved facts. MLIR emission and its verifiers establish structural fidelity, not an independent source-language analysis.

**§ 1(3).** This correction introduces no source syntax, initialization policy, thread-local storage rule, physical pointer layout, static ABI, new storage class, hidden startup operation, or permission to borrow hardware register bits. It does not resolve MD-081, MD-111, MD-114, generic/static initialization beyond the existing represented subset, or the full P15 completion gate.

**§ 1(4).** Schema **11** admits this operation as a recorded P15 amendment; schema **12** inherits the same spelling and contract unchanged. Schemas 1–10 do not admit it. Schema version 11's historical numbered paragraphs must remain stable; append a new numbered amendment (for example `v11.42`) rather than inserting or renumbering earlier paragraphs.

## § 2. Three identities with different responsibilities

**§ 2(1).** A static source binding resolved in Semantic IR has a **module-local `StaticID`**. That identity selects one `Static` in `Module.Statics`, whether source-declared at module scope or as a function-local `static let`. It is independent of a source name, the generated MLIR symbol, a Place root, or physical address.

**§ 2(2).** The Place referencing that static retains its independently resolved **`PlaceRootID`** and **`StorageIdentityID`**. These must not be replaced, equated numerically with `StaticID`, or re-created from text. Existing root properties `place_root_id`, `storage_identity`, and `address_space` remain mandatory according to the P15 Place-root contract.

**§ 2(3).** MLIR has one **`sec.static.global` symbol** for each represented static. The existing emitter may spell this `@sec_static_1`, but this text is only a symbol name: no verifier may parse the suffix `1`, parse `sec.source_name`, or infer a `StaticID` from it.

**§ 2(4).** Each newly emitted `sec.static.global` in schema 11 or later carries the exact discardable integer attribute **`sec.static_id = N : i32`**, where `N` is the originating nonzero Semantic IR `StaticID` interpreted as an unsigned 32-bit identity. A module must not contain two statics with the same `sec.static_id`, or two static declarations with the same symbol. The symbol and ID must be stable and deterministic for one verified Semantic IR module. Existing symbol generation may remain unchanged.

**§ 2(5).** A legacy schema-11 scalar-only `sec.static.global` lacking `sec.static_id` may be accepted solely by an explicitly documented compatibility path that does **not** allow `sec.place.static` or `static@N` to refer to it. New emission always writes the ID. Absence, zero, duplication, truncation, or an ID differing from the producer's canonical identity is not a recoverable default for a referenced static.

**§ 2(6).** Module identity scopes `StaticID`. Combining separately compiled modules must preserve disambiguation; matching numeric `StaticID`s in different modules never establishes equal storage. If an imported/cross-module static origin cannot be linked to its owning module's validated declaration, emission or verification fails closed. MD-116 does not introduce a new cross-module ABI.

## § 3. `sec.place.static` — exact operation contract

**§ 3(1).** Canonical operation name: `sec.place.static`.

**§ 3(2).** Its input SSA operand list is **empty**. It is a symbol-referencing root operation, not a `sec.storage.declare` consumer, not a pointer conversion, and not a load of the static value.

**§ 3(3).** Its one result is `!sec.place<T, "ro">` or `!sec.place<T, "rw">`. `T` is **exactly** the element type carried by the selected `sec.static.global` at the same representation stage. Numeric scalar-layout resolution may transform both corresponding types together while preserving that identity and authority.

**§ 3(4).** The operation has these required properties, in the established generic operation/property format:

```mlir
%place = "sec.place.static"() <{
  global = @sec_static_1,
  place_root_id = 12 : i32,
  storage_identity = 7 : i32,
  address_space = "ordinary"
}> : () -> !sec.place<si32, "ro">
```

The example is **illustrative schema-11 syntax**; the operation is not executable until the dialect has implemented this correction. `global` is a `FlatSymbolRefAttr` resolving to `sec.static.global`, `place_root_id` and `storage_identity` use the canonical P15 identity attribute representation, and `address_space` uses the existing P15 address-space vocabulary. The values 12 and 7 are independent of static ID 1.

**§ 3(5).** The operation must not introduce a second initialization, backing store, temporary local, runtime pointer, hidden allocation, or indirection. It denotes the same existing static storage that `sec.static.load/store` reference by symbol.

**§ 3(6).** The existing P15 root identity, storage identity, provenance, and address-space rules apply without inventing a `sec.static_storage` type or copying `!sec.storage` local-storage semantics into statics.

## § 4. Declaration linkage and address-taken preservation

**§ 4(1).** A `sec.place.static` `global` reference resolves to **exactly one** `sec.static.global` in the appropriate module/symbol scope. A missing symbol, a symbol of the wrong operation kind, an ambiguous reference, or an unresolved imported identity is invalid.

**§ 4(2).** The selected `sec.static.global` retains its existing `sym_name`, `type`, `initial`, and `is_mutable` declaration contract. It additionally carries `sec.static_id` (§ 2) and, when reference-addressable, **`sec.address_taken = true`**. This is the same canonical boolean marker used by MD-084 for reference-dependent local storage; it is not a new alternative marker or an assertion of a numeric machine address.

**§ 4(3).** Every `sec.static.global` targeted by any `sec.place.static` **must** have `sec.address_taken = true`. A static whose address is never taken need not have that marker; an explicitly emitted `false` is not a synonym for the required `true`. New emission mirrors the existing Semantic IR `Static.AddressTaken` fact.

**§ 4(4).** No pass may replace an address-taken global with an unrelated scalar, duplicate its backing storage, or erase its symbol while Place/reference dependents remain high-level. Extending the reference-dependency check to statics must preserve the single authoritative P5/P6/P8 boundary of MD-087; it must not introduce a competing root-identity or allocation classifier.

**§ 4(5).** `sec.static.load`, `sec.static.store`, and `sec.place.static` must resolve their static identity through the **same** symbol declaration and its `sec.static_id`. Static loads and stores do not automatically construct a Place; use of a Place does not implicitly load the value.

**§ 4(6).** The resolved static-root Place metadata must be consistent with the existing Semantic IR facts for that static. Multiple Place roots selecting the same static preserve the same underlying storage identity and referent. Distinct `StaticID`s do not become aliases because their initial values or source names match. Verification must reject conflicting root/storage identity claims rather than choose one arbitrarily.

## § 5. Type, mutability, storage validity, and effects

**§ 5(1).** `!sec.place<T, "rw">` requires a mutable `sec.static.global` (`is_mutable = true`). An immutable static admits only `"ro"`. A mutable static may admit an `"ro"` view when Sema has granted a valid shared borrow; a `"ro"` Place does not itself establish thread safety.

**§ 5(2).** `sec.place.static` is allowed for both module-level and function-local statics after they have been lowered to one module-level static declaration. A function-local `static let` never becomes automatic stack storage and is never reinitialized for each invocation.

**§ 5(3).** Address space, address stability, epoch/provenance, pinning, and lifetime facts remain those resolved by Sema and the existing P15 reference model. In particular, `static` duration does not waive the existing prohibition on unsafe mutable static aliases or synchronization requirements (`rules/declarations/static.md` § 5, §§ 17–19). No implicit reference to TLS *per-thread values* follows from a static `ThreadLocal[T]` key.

**§ 5(4).** Any nonordinary or target-specific address-space form is accepted only if the existing CompilationPlan and current MLIR schema can represent and verify it. Unrepresentable `target:` spaces fail explicitly, with no mapping to `ordinary`. Addressed MMIO/hardware endpoints belong to MD-111/MD-114 rather than `sec.place.static` by mere numerical address equality.

**§ 5(5).** As a high-level Place-root derivation, this operation represents an existing location and does not observe its value, allocate, initialize, destroy, or itself perform hardware I/O. The usual borrow, reference-guard, Place consumer, and effects contracts apply to later uses.

## § 6. Returned static references and `static@N`

**§ 6(1).** The existing MD-084 `sec.return_origins` root grammar already admits `static@N`. MD-116 does **not** invent a second return-origin root syntax or a new return attribute.

**§ 6(2).** For a function whose Sema-approved reference result may derive from module static with `StaticID = N`, the corresponding `sec.return_origins` entry may contain `static@N` and any existing type-valid projections. The referenced static declaration must have `sec.static_id = N` in the correct module and must be address-taken when returned through a Place.

**§ 6(3).** Native return-origin tracing recognizes `sec.place.static` as a root, resolves its `global`, reads the declaration's **explicit** `sec.static_id`, and records `static@N`. Subsequent `sec.ref.borrow_*`, reborrow, field/element Place projection, copy/move, branch argument, and permitted call path preserve the same root plus the resolved projection path according to MD-085. A forged string `static@N` without a traceable static origin is rejected.

**§ 6(4).** The same call-result and finite-origin-set rules of MD-085 apply. A call returning a static-origin reference may use that origin only if the callee's verified summary and the referenced storage identity can be instantiated unambiguously in the importing/calling context. Distinct modules' `static@1` are **not** interchangeable; unrepresented cross-module linkage remains unsupported, not guessed.

**§ 6(5).** A static-origin summary is provenance, **not** evidence that an otherwise invalid mutable borrow, static initializer reference, unsynchronized shared access, or invalidated lifetime is permitted. Sema must already have proved such permission.

**§ 6(6).** A `!sec.place` still must not escape as a returned value. Only a permitted reference produced through the Place may be returned, with its established reference facts and origin summary.

## § 7. Verifiers and lowering preservation

**§ 7(1).** The operation's own verifier checks operand count, one result, result Place type, exact global symbol resolution, declaration type equality, mutable authority, root metadata value types, valid address space, and the mandatory address-taken marker.

**§ 7(2).** The module/symbol verifier checks `sec.static_id` uniqueness and its required presence for the identity-bearing path. A static return summary that names an absent ID or wrong module fails before any origin is accepted.

**§ 7(3).** `--sec-verify-places` treats `sec.place.static` as a legal Place root and applies the same Place nonescape and metadata consistency rules as other P15 roots. It must not reject an otherwise well-formed static Place merely because it has zero SSA operands.

**§ 7(4).** `--sec-verify-borrow-semantics` applies MD-085 path-aware static return-origin tracing and ordinary borrow/authority/lifetime checks. `--sec-verify-reference-guards` continues to require the resolved dynamic-validity guards where applicable. Static lifetime must not be synthesized as blanket `proven` validity.

**§ 7(5).** Passes that preserve high-level Places/references preserve the static symbol, explicit identity, address-taken marker, root metadata, and reference facts. P6 resolves scalar types consistently; P8 does not erase Place/reference wrappers; P5 does not discharge address-taken static storage prematurely.

**§ 7(6).** Existing physical `sec-lower-static-storage` may lower scalar-only statics as before. If any reference-dependent static cannot yet be represented through the physical/static ABI pass, that pass must reject the specific unsupported path rather than erase the semantic Place, produce a raw pointer automatically, or silently alter the semantics.

**§ 7(7).** Failed module verification or unsupported emission must yield **no partially published MLIR bytes**. The actual package implementation must preserve the compiler's established all-or-nothing emission boundary.

## § 8. Example of the identity relationship

**§ 8(1).** One high-level module may contain the equivalent of:

```mlir
"sec.static.global"() <{
  sym_name = "sec_static_1",
  type = si32,
  initial = "10",
  is_mutable = false
}> {sec.static_id = 1 : i32, sec.address_taken = true,
   sec.source_name = "example::Count"} : () -> ()

%root = "sec.place.static"() <{
  global = @sec_static_1,
  place_root_id = 12 : i32,
  storage_identity = 7 : i32,
  address_space = "ordinary"
}> : () -> !sec.place<si32, "ro">
```

This is **normative in attributes and relationships**, but is an illustrative excerpt, not a full executable test module. The returned origin for a reference borrowed from `%root` is `static@1`, not `static@12`, `static@7`, or a value derived by parsing `"sec_static_1"`.

**§ 8(2).** A different symbol spelling with the same verified `sec.static_id` and identical module-local static meaning is acceptable. A different `sec.static_id` denotes a different static and is not silently equivalent.

## § 9. Mandatory conformance and regression tests

**§ 9(1).** **Positive static cases:** immutable module static to `ro` Place; mutable module static to `ro` and to authorized `rw` Place; function-local static retained as single static storage; repeated references to one static retain matching storage identity; field and array-element sub-Places where already representable.

**§ 9(2).** **Positive returned origins:** `static@N`, static field/element projections, an SSA/CFG-forwarded return, a verified direct call with representable static linkage, multiple eligible origins, and deterministic repeated emission. Both 32-bit and 64-bit target plans must be exercised.

**§ 9(3).** **Malformed / forged negatives:** unknown static symbol; reference to non-`sec.static.global` symbol; missing/zero/duplicate/mismatched `sec.static_id`; missing `sec.address_taken = true`; wrong Place type; `rw` on immutable static; inconsistent `place_root_id`/`storage_identity`/address-space facts; missing source-granted return origin; a source-provenance string without a supporting Place chain; mixed module-local static IDs; a local storage masquerading as static.

**§ 9(4).** **Regression boundaries:** scalar-only statics remain supported; a function-local static does not reinitialize; schemas 1–10 reject `sec.place.static`; schema 12 inherits the same semantics; MD-084 metadata and MD-085 path validation still pass; P5/P6/P8 preserve reference-dependent statics; hardware/addressed roots still fail closed pending MD-111/MD-114.

**§ 9(5).** **Pipeline tests:** verified source → Sema → Semantic IR → schema-11 (and inherited schema-12 where applicable) → reparse/print → the three P15 native verifiers. Run representative tests before/after P6/P8 and prove no hidden layout/ABI choice. The suite must assert that invalid inputs do not publish a partial module.

## § 10. Required synchronization and governance

**§ 10(1).** Add the complete operation and `sec.static_id` / static `sec.address_taken` contracts to `rules/mlir/sec_mlir_dialect.md` as a new schema-11 amendment; add the same contract as an **append-only** amendment to `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`. Do not renumber the snapshot's existing §§ 1–41 or override MD-084/MD-085/MD-087.

**§ 10(2).** Amend `rules/mlir/sec_mlir_lowering.md` and append a short note to `rules/mlir/lowering-versions/sec_mlir_lowering_v11.md` describing `OpPlaceStaticRoot` emission and reference-dependent static preservation. Schema 12 inherits the resulting operation and metadata contract; document that inheritance without creating a new schema-12 implementation owner.

**§ 10(3).** Add cross-references in `rules/compiler/semantic_ir.md` §§ 10, 28, 45, the P15 package specification, and `package15-todo.md` P15-42, P15-42a, P15-47; update `language-rulebook-status.md` only when a tracked rulebook's documentation state actually changes. No new source-level `static` permissions arise.

**§ 10(4).** Implement through the existing Go Semantic IR → Sec MLIR emitter, the existing Sec dialect registration/verifiers, and the existing canonical root/reference producers. Do not fork a new emitter, a second static identity model, or a second ownership checker.

**§ 10(5).** **Governance ownership remains unique:** update the existing `lowering.sec-mlir-package15` entry in `governance/lowering.yaml` for actual implementation, tests, and remaining boundaries; coordinate with the existing Semantic IR integration only where producer facts change. Do not invent duplicate `md116.*` integration owners or claim P15 complete on the strength of this document.

**§ 10(6).** After the *normative* rulebook synchronization is complete, remove or explicitly transition MD-116 in `missing-decisions.yaml` according to that register's existing policy. Implementation can continue independently and must remain `partial`/`remaining` in governance until verified. Move this correction to `rules/corrections/applied/` with the actual application date only at normative integration, not at drafting.

## § 11. Final decision and package boundary

**§ 11(1).** **Accepted:** `sec.place.static`, explicit verified module-local `sec.static_id` on `sec.static.global`, canonical `sec.address_taken = true` for referenced statics, unambiguous linkage between static identity, Place root, and existing `static@N` origins, with no physical layout or additional storage construction.

**§ 11(2).** **Not claimed:** implementation of the operation, resolution of MD-081/MD-111/MD-114, all P15-47 receiver cases, complete P15 acceptance, or a green repository-wide test suite. This correction removes one *normative* blocker once integrated; acceptance remains evidence-based.
