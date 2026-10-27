# MD-114 — Canonical Sec MLIR register-definition, field-path, and field-write-proof contract

- **Decision:** MD-114 — `sec-mlir-register-value-text-contract`
- **Status:** **Owner-approved — normative correction pending repository integration and implementation**
- **Created:** 2026-10-27
- **Last updated:** 2026-10-27
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Reference repository:** `YoePro/sec`, `main` at `5426af6cf8cce6fe85ff897b97050dfd330ef4fe` (2026-10-26)
- **Pending correction location:** `rules/corrections/MD-114-canonical-register-value-mlir-contract-correction.md`
- **Archive after complete normative integration:** `rules/corrections/applied/MD-114-canonical-register-value-mlir-contract-correction-YYYYMMDD.md`
- **Primary normative owners:** `rules/mlir/sec_mlir_dialect.md`, especially v11.41.1; `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md` (append-only amendment)
- **Related normative owners:** `rules/mlir/sec_mlir_lowering.md`; `rules/mlir/lowering-versions/sec_mlir_lowering_v11.md`; `rules/compiler/semantic_ir.md` § 21; `rules/declarations/registers.md` §§ 1–12; `rules/mlir/packages/sec-mlir-dialect_package15.md`
- **Implementation ownership:** existing `governance/lowering.yaml` integration `lowering.sec-mlir-package15` and existing Semantic IR/register integration owners; `package15-todo.md` P15-42/P15-42a and acceptance
- **Decision tracking:** `missing-decisions.yaml` MD-114
- **Existing decisions preserved:** MD-077 (§§ 5–11, 19), MD-110 (§§ 3–9); hardware endpoint/plan representation remains MD-111
- **Decision authority:** Approved by the Sec language owner, 2026-10-27, following review of the existing Semantic IR model and schema-11 early admission.

> **Integration contract:** This correction resolves the *textual and verifiable representation* of detached register definitions, field paths, and field-update proofs. It does not declare P15 complete, grant hardware-access support, or replace existing ownership/governance entries. Synchronize the normative target books before moving this file to `applied/`. Preserve all existing paragraph numbers; add an explicit amendment to v11.41.1 and an append-only entry in the historical v11 snapshot. A repository integrator must test the actual baseline before reporting implementation as complete.

---

## § 1. Purpose, authority, and non-goals

**§ 1(1).** This correction provides the previously missing concrete Sec MLIR encoding required by MD-077 § 19(2), (4), and (9): a module-wide register-definition table, an operation-specific canonical register-field path, and an update-proof attribute. It does not re-decide any source-language register semantics or rename MD-077's operations.

**§ 1(2).** Sema is the sole source-language authority for register identity, access eligibility, nested field selection, byte/field ordering, units, enum type, write-range evidence, and hardware operation planning. The canonical Semantic IR carries the resolved facts. Sec MLIR serializes and verifies their structural preservation; it must not reconstruct source-language permissions from names or invent new range proofs.

**§ 1(3).** The covered operations are the **detached, pure** value operations `sec.register.extract` and `sec.register.update`, together with register-definition lookup needed by existing MD-110 detached conversion operations. `!sec.register<"module::TypeName">` remains a nominal detached value type. A detached register is not a physical device, MMIO endpoint, raw pointer, or `!sec.hw_instance<T>`.

**§ 1(4).** Hardware-instance/resource records, endpoint/DeviceModel facts, physical access plans, `sec.hw.*` verification and physical lowering are *not* defined by MD-114; they remain MD-111 and existing MD-077 contracts. No hardware operation may become accepted merely because a detached register table is present.

**§ 1(5).** This correction does not select the physical storage form of a register value, introduce source syntax, change source bit allocation, create an integer alias for a nominal register, choose an ABI, infer host byte order, weaken field-write conversion rules, or add implicit hardware observation. Type identity and logical bit position are preserved independently from physical representation.

**§ 1(6).** This is an **explicit schema-11 early-admission amendment** to v11.41.1. Schema 12 inherits it unchanged. Schemas 1–10 reject its register types, attributes when used as part of these register constructs, and operations. Neither the default emitted schema nor package acceptance status is changed.

## § 2. Canonical module attribute `sec.registers`

**§ 2(1).** A module representing one or more nominal register types referenced by its types or operations has exactly one module-level attribute **`sec.registers`**. Its value is an `ArrayAttr` of the custom typed attribute **`#sec.register_def<...>`**, with no substitute stringly-typed table or independently inferred table. The table belongs to the current MLIR module.

**§ 2(2).** The canonical register-definition record has precisely these fields:

| Field | MLIR value | Semantics |
| --- | --- | --- |
| `identity` | Nonempty `StringAttr` | Complete canonical nominal identity, equal to the identity in `!sec.register<"...">`. |
| `width` | Positive integer attribute | Exact **logical** bit width, independent of target machine word size. |
| `order` | `StringAttr` | Exactly `"lsb-first"` or `"msb-first"`; the resolved default is emitted explicitly. |
| `byte_order` | Optional `StringAttr` | Exactly `"little-endian"` or `"big-endian"` when explicitly resolved as such; absent otherwise. |
| `fields` | `ArrayAttr` of `#sec.register_field<...>` | Every declared field in source declaration order, **including** reserved fields. |

**§ 2(3).** Canonical record spelling is `#sec.register_def<identity = ..., width = ..., order = ..., byte_order = ... [when present], fields = [...]>`. The compiler must define a typed ODS/AttrDef representation and a parse/print contract for the exact names above. The optional `byte_order` field is omitted, rather than replaced with a fabricated `native`/host-endian guess, if the semantic type leaves byte order contextual. Resolved contextual byte order is supplied only by an explicit applicable serialization/target plan, not by a detached field-access operation.

**§ 2(4).** Every register identity occurs **once** in `sec.registers`, even if used in several functions or nested in multiple outer registers. Identities are case-sensitive, module-qualified, and nominal; identical width/layout does not imply identity equality. Duplicate identities, conflicting definitions, references without definitions, missing mandatory fields, and unknown mandatory field names fail closed.

**§ 2(5).** Table order is the deterministic **canonical first-use/registration order** of the verified Semantic IR register-definition table, not hash-map iteration or an emitter-selected alphabetical sort. Nested types may require forward references during parsing: verification resolves all definitions as a complete module before validating nested paths. Determinism is required for an identical Semantic IR module and CompilationPlan; no new global ordering or renumbering of Semantic IR types is invented.

**§ 2(6).** An empty table is represented by `sec.registers = []` only when needed for a canonical explicitly present empty registry. A module that uses no register types may omit `sec.registers`. A module that **uses** register types cannot omit it or treat omission as an implicit registry assembled from operation names.

## § 3. Typed register-field definitions

**§ 3(1).** Each element of `fields` is the typed custom attribute **`#sec.register_field<...>`**. It contains precisely:

| Field | MLIR value | Meaning |
| --- | --- | --- |
| `ordinal` | Nonnegative `i32` integer attribute | Declaration-order `RegisterFieldID`, local to its containing register; includes reserved fields. |
| `name` | `StringAttr` | Resolved declared name; `"_"` for a reserved field. |
| `offset` | Nonnegative integer attribute | The lowest logical bit index in the field's slice, with **bit 0 = LSB**. |
| `width` | Positive integer attribute | Exact contiguous bit width. |
| `type` | MLIR `Type` | Resolved value carrier/nominal enum/nested register type, not an invented standalone `bit[N]` type. |
| `access` | `StringAttr` | Resolved effective access behavior from `registers.md` § 11. |
| `shadow` | `BoolAttr` | Effective shadow policy (`true` = shadow, `false` = noshadow). |
| `unit` | Optional `StringAttr` | Canonically resolved unit identity when applicable; absent if unitless. |
| `reserved` | `BoolAttr` | `true` precisely for a reserved field, otherwise `false`. |

**§ 3(2).** Canonical field spelling is `#sec.register_field<ordinal = ..., name = ..., offset = ..., width = ..., type = ..., access = ..., shadow = ..., unit = ... [when present], reserved = ...>`. The key order above is the canonical printer order. Optional values are truly absent when inapplicable; missing required booleans or malformed types are invalid, not reconstructed defaults.

**§ 3(3).** Field ordinals are contiguous **`0, 1, ..., count - 1`** in declaration order. Two nonreserved fields may not have the same resolved name within a register. Multiple reserved entries may use name `"_"` and are distinguished by ordinal, never accessible through that name. The `reserved` flag and reserved-name contract must agree; a reserved field is not a publicly addressable field.

**§ 3(4).** Logical occupied bit intervals are `[offset, offset + width)`. Every interval must be within `[0, register.width)`, all intervals must be pairwise nonoverlapping, and their union must cover exactly the declared register width. Both `lsb-first` and `msb-first` must agree with the verified declaration-order allocation; a field's `offset` is never reinterpreted as an MSB-relative index. Unknown, unsupported or inconsistent layout fails verification.

**§ 3(5).** A plain one-bit `bit` field has the canonical `bool` field value type (schema's boolean scalar, e.g. `i1`). A `bit[N]` field with `N >= 2` has the exact smallest available standard unsigned carrier chosen by MD-077 (8/16/32/64/128/256 bits) and the field's `N`-bit representable interval as its independently verifiable range contract. A bit-backed enum has its distinct nominal enum type and open bit-domain semantics; a nested register has its distinct `!sec.register<...>` type and exactly matching width. Unit-bearing fields preserve their correct unit-qualified value type and unit identity. No serializer may replace one of these types by an arbitrary same-width integer.

**§ 3(6).** The `access` value is the effective canonical register access spelling, including `read-write`, `read-only`, `write-only`, `read-clear`, `read-set`, and `write-one/zero-{clear,set,toggle}`. The `shadow` value is the *resolved effective* policy after register/field defaults, not a guessed copy of source modifiers. Unknown access/policy spellings are invalid. Neither attribute changes the pure behavior of extracting/updating a **detached** value; their behavioral hardware interpretation remains owned by `registers.md`, MD-077, and MD-111.

**§ 3(7).** For a nested-register field, `type` resolves to its own `!sec.register<"...">` and that definition must be found in `sec.registers`. The nested type has its own `order` and `byte_order` contract. Logical composition positions the nested bit vector in the outer interval without any implicit byte swap. Cyclic, infinitely sized, or otherwise invalid nested layouts must be rejected.

**§ 3(8).** `unit` is serialized only as the canonical identity already resolved by Sema, never as a display-only abbreviation when different units could collide. If the selected schema or compiler stage cannot represent the required type/unit identity precisely, emit a focused unsupported-lowering error rather than erase the unit. The same principle applies to bit-backed enum identity and nominal nested-register identity.

## § 4. Register-definition example and exact interpretation

**§ 4(1).** The following example is **illustrative proposed schema-11 generic metadata syntax**; it must not be represented as an already executable lit fixture until the typed attributes and verifier are installed:

```sec
// A detached register type; declaring it does not bind any hardware.
type Control register[8] {
    Enabled: bit,
    _: bit[7],
}
```

```mlir
module attributes {
  sec.dialect_version = 11 : i32,
  sec.registers = [
    #sec.register_def<
      identity = "main::Control",
      width = 8,
      order = "lsb-first",
      fields = [
        #sec.register_field<
          ordinal = 0, name = "Enabled", offset = 0, width = 1,
          type = i1, access = "read-write", shadow = false,
          reserved = false>,
        #sec.register_field<
          ordinal = 1, name = "_", offset = 1, width = 7,
          type = ui8, access = "read-write", shadow = false,
          reserved = true>
      ]>
  ]
} {
  // Register-valued operations appear here when emitted.
}
```

**§ 4(2).** The example's `Enabled` field occupies logical bit 0. The reserved field occupies bits 1 through 7 and remains part of the register's exact logical value. No statement about physical byte layout, MMIO location or hardware effects follows from this table.

**§ 4(3).** For an explicit `msb-first` register, the first declaration receives the highest available contiguous range. A register's `byte_order` affects only an independently specified ordered-byte serialization boundary; field allocation and detached integer bit-pattern conversion do not perform a byte swap. This preserves MD-110 § 6.

## § 5. Canonical `sec.field_path`

**§ 5(1).** A detached `sec.register.extract` or `sec.register.update` has the required operation attribute **`sec.field_path`**, of MLIR `ArrayAttr` type. Every array entry is a canonical nonnegative **`i32`** ordinal. The array is nonempty for these field-level operations and has no duplicate name/offset parallel representation.

**§ 5(2).** The first ordinal selects a field in the nominal register type of the operation's first operand. Each subsequent ordinal selects a field in the **nested register type** selected by the previous step. The last ordinal denotes the leaf field to extract or update. A step into a nonregister carrier, unknown ordinal, reserved field, or missing nested definition is invalid.

**§ 5(3).** A path is a *type-directed series of field identities*, not an array of bit offsets, source member names, or numeric hardware addresses. The verifier computes the effective logical slice offset and width from the canonical register table by composition and confirms it matches the resolved operation/result contract. Two fields cannot become interchangeable because they occupy equal-width slices.

**§ 5(4).** Example of the agreed representation for `Status.Flags.Negative` where `Flags` is ordinal 0 in `Status` and `Negative` is ordinal 2 in `Flags`:

```mlir
{sec.field_path = [0 : i32, 2 : i32]}
```

The example is compiler-only MLIR metadata, not Sec source syntax.

**§ 5(5).** An ordinary detached **whole-register** conversion, move or copy has no field path. It must not counterfeit a field operation with an empty path; whole-value operations retain their own MD-077/MD-110 representations. MD-111 separately owns explicit whole-register hardware projection metadata.

**§ 5(6).** The existing Semantic IR implementation currently emits single `RegisterFieldID` steps and may compose nested operations. Lowering may preserve those individual steps as one-element field paths or combine a proven composition into a longer path if that retains the exact nominal type, intermediate field selections, value-evaluation order, full register bit pattern and update proof. It must not synthesize a path from AST text after Sema.

## § 6. Canonical detached operations and result typing

**§ 6(1).** The names remain exactly **`sec.register.extract`** and **`sec.register.update`**. Both are pure detached-value operations. They do not bind, read, write, observe, refresh or commit external hardware; an unused pure extraction is not a destructive hardware read.

**§ 6(2).** `sec.register.extract` has exactly one SSA operand of a nominal `!sec.register<T>` type, one result of the final field's exact resolved Sec value type, and a required valid `sec.field_path` attribute. It has **no** `sec.write_proof`. It neither consumes a hardware-instance value nor implicitly decodes ordered bytes.

**§ 6(3).** `sec.register.update` has exactly two SSA operands: the original detached `!sec.register<T>` and a replacement of the leaf field's exact resolved type. It returns the **same nominal** `!sec.register<T>`. It has required `sec.field_path` and `sec.write_proof` metadata and updates only the selected logical bit slice. All other bits, including reserved bits, are preserved. It is not a reference mutation, hardware request, or implicit lossy integer cast.

**§ 6(4).** Extraction/update of a nested field uses the nested register's own nominal identity and layout. If a combined nested path is used, the verifier resolves every intermediate nominal register and validates the complete path. An equivalent sequence of typed nested extracts/updates is permitted; it must preserve result bits, nominal identities and ordering.

**§ 6(5).** A pure detached value operation is not forbidden merely because the field's *hardware* access contract is `read-clear`, `read-only`, `write-only` or specialized write-one/zero semantics. On detached values these modifiers are layout metadata, not physical side effects (registers.md § 11.7 clarification of 2026-10-23). The ordinary Sec value/Place mutability, typing and field-write fit rules still apply.

**§ 6(6).** Both operations verify range, nested type, unit, bit-backed enum and reserved-field constraints from the canonical module definition. The operation verifier may not accept a field by name alone, infer a new definition from an unknown type, or use the destination integer width to manufacture a missing field proof.

## § 7. Canonical field-write proof `sec.write_proof`

**§ 7(1).** `sec.register.update` carries one required operation attribute **`sec.write_proof`** whose value is a typed custom attribute **`#sec.register_write_proof<...>`**. This is the canonical serialization of Sema/Semantic IR `RegisterWriteProof`; it is separate from MD-110's **`sec.register_fit_proof`** used for integer-to-register conversion.

**§ 7(2).** The typed record has precisely `basis` (`StringAttr`) and optional `low`, `high` (`StringAttr` each), with these forms:

```mlir
{sec.write_proof = #sec.register_write_proof<basis = "typed">}
{sec.write_proof = #sec.register_write_proof<basis = "constant", low = "5", high = "5">}
{sec.write_proof = #sec.register_write_proof<basis = "interval", low = "0", high = "15">}
```

These are *illustrative pre-integration spellings*. No extra keys or alternate proof basis are accepted.

**§ 7(3).** `typed` is permitted only when the field's replacement already has the exact admitted **bool, bit-backed enum, or nested nominal register** type. Both `low` and `high` must be **absent**. A plain multi-bit unsigned carrier cannot bypass its `N`-bit field range merely because the carrier type matches (`uint8` is not proof that a `bit[3]` field fits).

**§ 7(4).** `constant` requires **both** `low` and `high`, equal to the same nonnegative exact value actually proven for the replacement, and within the selected field's representable bit interval. An arbitrary constant written in an attribute is not evidence by itself: verify its relationship to the represented constant/evaluation facts or the approved canonical proof transport. The verifier must reject a fabricated constant assertion.

**§ 7(5).** `interval` requires **both** `low` and `high`, an ordered inclusive range already proven for the replacement, and `0 <= low <= high <= 2^field_width - 1`. The producer must transport Sema's corresponding certified range fact; the verifier checks consistency with admitted verified evidence and may not treat attacker-controlled or unverified text as an independent source of authority. No new runtime range check or fail edge is created by this attribute.

**§ 7(6).** `low` and `high` are canonical arbitrary-precision, base-10 nonnegative integers with no leading plus sign, whitespace, or redundant leading zeroes (except `"0"`). Parse and compare without truncation to host `int`, `int64` or target `uint`; invalid number text, negative bounds, missing paired bound, unknown basis, unproven value relation or bounds exceeding the declared field interval are errors.

**§ 7(7).** The `sec.write_proof` attribute must refer to the leaf field selected by the **same** `sec.field_path` and the replacement SSA operand. Reusing a proof from a different field, register type, operand, unit domain or intermediate update is invalid. Nested updates require appropriate proof for each actual leaf replacement; an enclosing register replacement may use `typed` only because the nested register's exact nominal value type is known.

**§ 7(8).** These are structural proof-preservation requirements. Sema continues to decide valid source programs and valid range analyses. The C++ verifier checks type, path, intervals, provenance/available proof transport and operation relationships; it must not implement a competing source-language range or borrow analysis. If the installed schema cannot verify sufficient evidence for a required update, emission must fail closed, not quietly downgrade to an unchecked bit mask.

## § 8. Coexistence with MD-110 detached bit-pattern conversions

**§ 8(1).** MD-110 remains authoritative for **`sec.register.fits_bits`**, **`sec.register.from_bits`** and **`sec.register.to_bits`**, with their existing exact integer interpretation/fit rules. This correction supplies their missing common register-definition lookup and does not change their names, permitted scalar types, bit fit rules, target-width resolution, signedness, effects, or error paths.

**§ 8(2).** `sec.register.from_bits` and `sec.register.to_bits` must resolve their nominal `!sec.register<T>` identity and bit width through the **same** `sec.registers` module table defined in § 2. Conversion verification cannot infer width from a guessed machine-integer type or use a second, inconsistent register registry. `fits_bits` uses that same type/width lookup.

**§ 8(3).** The MD-110 attribute **`sec.register_fit_proof = {basis = ...}`** applies only to `sec.register.from_bits`; its three recorded MLIR bases are `"constant"`, `"source-domain"`, and `"guarded"`. It is **not** `sec.write_proof`, and a field-update `"interval"` is not silently a fourth conversion basis. The two proof families must never be interchanged.

**§ 8(4).** The repository baseline has a limited **normative/implementation mismatch**: Semantic IR `RegisterConversionFacts` accepts the additional basis `"bit-extraction"`, whereas MD-110 § 8(4)'s MLIR transport specifies only the three bases in § 8(3). The semantic legitimacy of a correctly narrowed `bit[N](x)` result is already established by MD-110 § 4(2). This correction does **not** introduce a new MLIR proof basis by implication; the MD-110 synchronization must supply a verified lossless encoding or an explicitly approved amendment before Go emission claims support for that IR case. Invented guards, fabricated source-domain coverage and silent unsupported lowering are forbidden.

**§ 8(5).** Byte order remains irrelevant to `from_bits`/`to_bits` logical bit-vector interpretation and to detached field extraction/update. Physical ordered-byte encoding is a separate contract. A nested register does not inherit an implicit byte swap merely because inner and outer declarations use different byte orders.

## § 9. Parser, verifier, preservation, and failure requirements

**§ 9(1).** The compiler and dialect must agree on typed attribute registration, exact field/key spellings, optional-key omission rules, canonical print order, parse/print/parse stability, version gating, and the same module table for extract/update and MD-110 conversion lookup. An arbitrary dictionary with superficially similar strings is not interchangeable with a registered `#sec.register_def` or `#sec.register_field` attribute.

**§ 9(2).** The dialect/module verifier checks: unique nominal identities, presence of all referenced definitions, valid nonzero width and field layout, complete coverage including reserved fields, valid allocation/byte order, correct carrier/enum/nested/unit types, effective policy and access names, and no recursive finite-width contradiction. Unknown required attribute fields must not be ignored.

**§ 9(3).** The operation verifier checks exact operand/result arity and types, legitimate detached category, nonempty field path and correct nested lookup, reserved-field exclusions, correct result type, no forbidden effects, and exact proof shape/value agreement. No source-name matching may replace nominal identity or field ordinal.

**§ 9(4).** Existing schema-11/12 operations that preserve `!sec.register` values must preserve their nominal identity; relevant passes may not strip the module register table while it is referenced. Semantic layout/bit-extraction and write-proof facts may be removed only after their obligations have been discharged by a verified legal lowering.

**§ 9(5).** Regeneration of a table after an optimization is permitted only if it remains derived from verified canonical semantic identities, deterministic, complete for all surviving referenced types, and semantically equivalent. A pass must not silently reorder source fields, change their ordinal identities, shrink the set of reserved bits, or substitute physical endian behavior.

**§ 9(6).** A module with a recognized but currently unsupported register operation fails with a focused diagnostic naming the missing representation/verified capability. Unknown or malformed required metadata, an incompatible schema version, or a failed register verification must fail **before** publication of partial output. Do not fall back to an integer operation without the proven nominal register semantics.

**§ 9(7).** `!sec.hw_instance<T>` and `sec.hw.*` remain subject to MD-077 and MD-111; this detached-value table may establish their **register type identity only**, not physical endpoint, access authorization, effects, shadow-state knowledge or fault plan. No `!sec.place` or reference may be fabricated for external hardware bits.

## § 10. Required implementation synchronization

**§ 10(1).** Install this correction by appending a labelled **MD-114 amendment to `sec_mlir_dialect.md` v11.41.1**, explicitly defining `sec.registers`, both typed attributes, `sec.field_path`, and `sec.write_proof` in the schema-11/12 register family. Keep all existing `v11.41.1` and `v11.41.2` semantics intact; do not renumber historical sections.

**§ 10(2).** Append a corresponding MD-114 synchronization note in `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md` without renumbering the snapshot's numbered paragraphs. Its role is to record that the canonical consolidated dialect contains the exact table/path/proof spelling. Schema 12 inherits the registered schema-11 forms without a separate contradictory definition.

**§ 10(3).** Update the corresponding preserved lowering rules in `rules/mlir/sec_mlir_lowering.md` and append-only historical v11 lowering amendment: map Semantic IR `Module.Registers`, `RegisterDefinition`, `RegisterFieldDefinition`, `RegisterFieldID` paths and `RegisterWriteProof` exactly; perform no name/bit-layout reconstruction after Sema. Keep Semantic IR's existing `register.value.extract/update` identity and fields as the source of truth.

**§ 10(4).** In `mlir/`, implement typed TableGen attributes, registration, parser/printer, module-table verification, detached operation definitions, exact version gate, pure memory/effect interfaces and all negative checks. This may use separate register-specific source files but must remain one canonical dialect registry. Preserve the existing P15/P16 verifier separation.

**§ 10(5).** In Go `internal/lowering/secmlir`, emit the module attribute deterministically from verified `Module.Registers`, emit `sec.field_path` from resolved field steps, and emit the permitted `sec.write_proof` only from established proofs. Do not emit fabricated field definitions from operation-local types or guessed offsets. The emitter must reject categories not representable in the installed schema.

**§ 10(6).** Reconcile the specific MD-110 `bit-extraction` IR/MLIR mismatch separately under MD-110's existing correction and governance owner. Do not convert this decision into a wholesale rewrite of MD-110 or pretend MD-114 alone completed its conversion implementation.

**§ 10(7).** Update `package15-todo.md` and any existing package implementation/acceptance report with the exact implemented/tested surface. Update the **existing** `lowering.sec-mlir-package15` record in `governance/lowering.yaml`, and any existing owning Semantic IR register record, using their established machine-readable IDs and `implemented`/`remaining`/`identified_bugs` conventions. Do **not** introduce a second governance owner for the same work or replace old evidence with a speculative completion claim.

**§ 10(8).** Once the MD-114 normative text is integrated, remove/close its one entry in `missing-decisions.yaml` according to the register's conventions. That means **decision closed**, not automatically **feature implemented**. Keep MD-111 open and P15 partial until their separate contracts and acceptance requirements are met. Move this correction to `rules/corrections/applied/` only after its required normative edits have been merged.

**§ 10(9).** Ensure `language-rulebook-status.md`, if it tracks the relevant affected books, accurately distinguishes a written/integrated rule from completed implementation. Do not edit unrelated package statuses, increment dialect schema arbitrarily or claim a green full-suite result from focused tests.

## § 11. Required conformance and regression matrix

**§ 11(1).** Positive: module with no registers; one register; multiple nominal same-layout registers; nested register types; two register users in different functions; module table ordering deterministic across repeated generation and map-iteration perturbation.

**§ 11(2).** Positive: `lsb-first` and `msb-first` with exact bit offsets; explicit little/big endian, absent byte order and differing nested byte orders without automatic swapping; reserved fields preserved in the table.

**§ 11(3).** Positive: one-bit bool, each smallest unsigned carrier width up to the supported register width, unit-bearing fields, bit-backed open enum fields, nested nominal register field, and exact typed value extraction/update.

**§ 11(4).** Positive: one-level and multi-level `sec.field_path`; composed nested extraction/update equal to the corresponding sequence of pure one-level value operations; preserved unrelated/reserved bits; proof-correct `typed`, `constant` and `interval` updates.

**§ 11(5).** Positive: schema-11 parse/print/verify/parse and schema-12 inheritance, P5/P6/P8 preservation, and unaffected schema-9/10/11 predecessor regressions. Round trips must retain all nominal identities, field names/ordinals, bit offsets/widths, effective access/shadow policy, unit identity, and proof bounds.

**§ 11(6).** Positive: MD-110 `fits_bits`, `from_bits`, and `to_bits` resolve the **same** module register definition and preserve the correct bit width and signed/unsigned interpretation. Coverage of the separately recognized `bit-extraction` transport awaits the documented MD-110 reconciliation, not an invented fourth MLIR basis.

**§ 11(7).** Negative: missing/duplicate module register table or identity, conflicting nominal definitions, malformed/noncanonical attributes, missing referenced type, missing or duplicated field ordinal, overlapping fields, a gap including reserved bits, out-of-register-width slice, invalid field carrier, illegal nested recursion and unsupported policy spelling.

**§ 11(8).** Negative: empty/bad `sec.field_path`, unknown or reserved field, attempt to descend into a scalar field, wrong result type, wrong leaf replacement type, register-update result of another nominal type, and field name/offset substituted for an ordinal.

**§ 11(9).** Negative: `typed` on a plain multi-bit unsigned carrier; missing/extra bounds for any proof basis; reversed, noncanonical, negative, overflowed-for-schema or out-of-field-range endpoints; constant with unequal bounds; unproven proof relationship; missing `sec.write_proof`; `sec.register_fit_proof` substituted for `sec.write_proof` or vice versa.

**§ 11(10).** Negative: schema 1–10 uses of register constructs, implicit hardware read/write in detached value operations, fabricating `HardwareInstance[T]` authority from a detached value, missing endpoint plans treated as valid, and silently accepting unsupported lowering. An unsupported-feature test must assert **no partially emitted successful module**.

**§ 11(11).** Required build/test gates include C++ `check-sec-mlir` on an explicit absolute executable/tool configuration, the relevant Go Semantic IR/emitter and compiler suites, schema version-selective tests, deterministic round-trip tests, P15/P16 relevant reference and register regression fixtures, and governance unique-owner/required-fragment validation. Record exact commands, counts, baseline failures and any deferrals rather than claiming unrun tests passed.

## § 12. Decision and controlled handoff

**§ 12(1).** **Approved A:** `sec.registers` is the single canonical module-level table of typed `#sec.register_def` records with typed `#sec.register_field` entries, including nominal identity and ordered, fully verified logical field layout.

**§ 12(2).** **Approved B:** `sec.field_path` is an ordered nonempty array of `i32` field ordinals for detached field operations, traversed from the operand register through nested nominal registers to the selected leaf. Field identity is not bit offset or source spelling.

**§ 12(3).** **Approved C:** `sec.write_proof` carries typed `#sec.register_write_proof` with **exactly** the `typed`, `constant` and `interval` bases and the conditional arbitrary-precision interval bounds defined in § 7. It is independent of MD-110's integer-to-register `sec.register_fit_proof`.

**§ 12(4).** Existing MD-077 and MD-110 language semantics, nominal register/value distinction, access behaviors, field carriers, range proofs, byte ordering, and versioning remain unchanged. MD-111 retains ownership of hardware endpoints, resource and access-plan contracts. A new physical representation decision is neither introduced nor required.

**§ 12(5).** This correction closes the **MD-114 design decision** only once integrated in canonical authority. Implementation and P15 package closure remain separately tracked and require their own verified evidence. All design outcomes above are owner-approved; the integrator must not solicit new choices for consequences already fixed here, and must stop rather than inventing missing unrelated MD-111/MD-110 semantics.

---

### Integration order (editorial, not new normative sections)

1. Apply § 2–§ 9 to the consolidated dialect v11.41.1 area and record the inherited schema-12 text without renumbering.
2. Append the corresponding versioned v11 dialect and lowering amendments; preserve original snapshot text and its paragraph numbering.
3. Implement typed ODS attributes, module and operation verification, native round trips, and schema gates; test each stage before moving to the next.
4. Implement verified Semantic IR → MLIR Go emission; test detached types/extract/update and MD-110 shared-table lookup without claiming MD-111 hardware support.
5. Synchronize existing P15/governance entries and the one MD-114 register item; archive this correction only after normative integration, preserving implementation status as actually verified.
