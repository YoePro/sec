# MD-019 — Explicit struct layout modifiers (approved correction)

- **Decision ID:** MD-019 — `explicit-layout-syntax`
- **Document kind:** Approved normative correction; canonical integration handoff
- **Status:** **APPROVED — normatively integrated 2026-11-05** (grammar.md struct and foreign struct productions; struct.md § 15A; layout.md §§ 1(12), 4.3(4), 27(2), 27(6), 28(5)–(6), 29(5), 30(6); ffi.md § 23; attributes.md; compile_time_evaluation.md § 4(6); allocation.md § 9(7a); semantic_ir.md § 104(5); parameter_usage_analysis.md). MD-019 removed from missing-decisions.yaml. **Compiler implementation pending** (governance `frontend.structs`, `frontend.layout`, `frontend.ffi-v2`, allocation and analysis owners).
- **Created / owner approval recorded:** 2026-11-05 (decisions established across 2026-11-04–05)
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Repository:** `YoePro/sec`
- **Reviewed `main` HEAD:** `0cdacb04fb3757a26569d51c21a5ae445f0facaf` (2026-11-05)
- **Normative owners:** `rules/foundations/grammar.md`, `rules/declarations/struct.md`, `rules/memory/layout.md`, `rules/platform/ffi.md`
- **Supporting owners:** `rules/foundations/attributes.md`, `rules/compiler/compile_time_evaluation.md`, `rules/memory/arena.md`, `rules/memory/allocation.md`, `rules/compiler/semantic_ir.md`, `rules/analysis/parameter_usage_analysis.md`
- **Governance:** existing fragments indexed by `governance/index.yaml`; notably `governance/memory.yaml` (`frontend.layout`, `semantic-ir.layout`, `lowering.layout`, `platform.layout`, `tooling.layout`), `governance/declarations.yaml` (`frontend.structs`), `governance/ffi.yaml` (`frontend.ffi-v2`), `governance/allocation.yaml`, `governance/analysis.yaml`, and the existing parser/formatter/diagnostics owners
- **Decision register:** `missing-decisions.yaml`, entry `MD-019`

> **Integration contract.** This file records approved Sec language design, **not** completed implementation. Integrate into the existing canonical rulebooks **one numbered section / one vertical step at a time**, test the step, and update the **existing** governance IDs. Preserve previously published rulebook paragraph numbers; amend in place or append a uniquely numbered subsection rather than renumbering references. Do not archive this correction as *applied* or close `MD-019` merely because this file exists. Close it only after all relevant canonical language rules have been synchronized; compiler implementation requires separate, verified evidence.
>
> **Scope.** Prioritize hosted systems (Linux, Windows and other supported hosted targets) and actual C/system ABI/FFI use. This correction does not design freestanding/bare-metal allocation profiles (MD-034) or the standard-library string API (MD-090).

---

## § 1. Purpose, scope and inherited authority

**§ 1(1).** MD-019 supplies the missing **source spelling** for existing explicit layout contracts. The canonical physical layout model, target `CompilationPlan`, storage and alignment safety rules remain authoritative. MD-019 must not invent another physical layout service or ABI classifier.

**§ 1(2).** The relevant source entities are ordinary named Sec `struct` types, complete `extern "C"` named `struct` types and their **stored fields**. `register[N]` and C bitfields continue to follow their own canonical declarations and layout rules. No general layout modification of enums, unions, local variables, properties, methods, expressions or incomplete foreign types is introduced here.

**§ 1(3).** `extern "C" type Name struct` **already** selects the active target's C ABI representation (`rules/platform/ffi.md` §§ 15, 23; `rules/declarations/struct.md` § 15). This is an inherited decision, **not** a new MD-019 decision. There is no redundant `@repr(C)` and ordinary Sec structs do not become FFI-compatible by layout coincidence.

**§ 1(4).** The general `@` attribute set remains **closed** (`rules/foundations/attributes.md`). The new layout notation is a family of **compiler-known contextual declaration modifiers** within `struct`/stored-field grammar, following the declaration-modifier pattern already used by `register[N]`. It is **not** an extension of the `@` attribute attachment model.

**§ 1(5).** This final spelling replaces the earlier illustrative `@minAlign`, `@pack` and `@byteOffset` examples. The semantic decisions remain as approved; only their source spelling and parser attachment point are consolidated. Old illustrative forms do not become aliases.

---

## § 2. D1 — Compiler-known declaration modifiers (approved; final syntax consolidation)

**§ 2(1).** A layout modifier is written **inline with its declaration**, not as an `@` attribute, an independent type contract or a special library call:

```sec
extern "C" type Header struct pack(1) minAlign(8) {
    kind: c::uchar,
    value: c::uint,
}
```

**§ 2(2).** A stored field may carry its own inline layout modifiers between its type and any optional existing raw-string field tag:

```sec
type Message struct pack(1) {
    kind: byte,
    value: uint32 minAlign(8) `json:"value"`,
    checksum: uint16 byteOffset(16),
}
```

**§ 2(3).** The established punctuation convention is retained: `[]` expresses type arguments, widths, extents and dimensions (for example, `register[32]`, `bit[4]`, `byte[16]`, `list[T]`); `()` supplies **modifier arguments**, whether integer or boolean in future modifier families. A valueless modifier, if defined by its owning grammar, has no parentheses (for example, existing register `shadow`). These rules do **not** change the already approved register syntax.

**§ 2(4).** The spellings `pack`, `minAlign` and `byteOffset` are contextual in their permitted declaration positions. They are not general-purpose global keywords, functions or executable expressions. The parser, formatter and LSP must retain the modifier identity, source span and compile-time argument expression.

**§ 2(5).** A modifier kind may occur at most once at the same declaration level. The order of **different** kinds on the same declaration does not alter their semantics. Repetition, conflicts, unknown modifiers in a layout-modifier position, malformed arguments and invalid attachment targets are diagnosed; they are not silently ignored.

**§ 2(6).** Existing `implements` placement and the current struct-tag contract remain unchanged. The grammar extension is localized to positions owned by `StructTypeDeclaration`, `ForeignTypeDeclaration` and `StructField`; it must not reinterpret constructor calls, function arguments, generics, raw tags or C bitfield width declarators.

---

## § 3. D2 — `minAlign(N)` (approved)

**§ 3(1).** `minAlign(N)` denotes a **minimum required alignment**, measured in **bytes**. The physical alignment may be larger. A compiler may not treat it as an exact alignment request or silently lower it.

**§ 3(2).** It is permitted on a named complete `struct` declaration and on a stored struct field, for both ordinary Sec structs and complete `extern "C"` structs.

**§ 3(3).** `N` must evaluate at compile time to a **positive power-of-two integer** (1, 2, 4, 8, 16, 32, ...), and must be representable and supported by the selected target/ABI/storage context. Zero, negative, non-power-of-two, non-integer or runtime-dependent values are invalid. This source requirement does **not** assert that every internal target layout quantity is a power of two.

**§ 3(4).** Type-level `minAlign(N)` constrains every materialized object of that type, including its occurrences as a nested stored field. Field-level `minAlign(N)` constrains that particular field's placement and may increase the containing aggregate's effective alignment and size.

**§ 3(5).** An over-aligned type still requires compatible allocation and storage when materialized; a syntactically valid `minAlign(N)` is not proof that every allocator, Arena or foreign provider can satisfy it (`rules/memory/layout.md` § 28; `rules/memory/arena.md`; `rules/memory/allocation.md`).

---

## § 4. D3 — `pack(N)` (approved)

**§ 4(1).** `pack(N)` is a **type-level** modifier for complete struct declarations, including `extern "C"` structs. It caps the **ordinary/natural field alignment used for placement** at `N` bytes. It does not automatically erase explicitly declared minimum alignment requirements.

**§ 4(2).** `N` must be a positive power-of-two integer known at compile time, measured in bytes, and valid for the selected target. This is the same value-domain policy as § 3(3).

**§ 4(3).** Packing affects field placement, inter-field padding and potentially aggregate alignment/size; it does not reorder fields, change field types or nominal identity, implicitly change the active C ABI, or grant access to otherwise invalid memory representations.

**§ 4(4).** A nested aggregate retains its own **internal** field layout. Applying `pack(N)` to the outer struct does not recursively rewrite the nested type's field offsets or representation.

**§ 4(5).** Packing does not permit an unsafe or invalid aligned safe reference to a misaligned field. Any permitted by-value field access that requires an unaligned load/store must use the target-supported lowering and retain the canonical borrowing/reference restrictions (`rules/memory/layout.md` §§ 8, 27; `rules/platform/ffi.md` § 23).

---

## § 5. D4 — Combining type-level `pack(N)` and `minAlign(M)` (approved)

**§ 5(1).** Both modifiers may occur on one complete struct type:

```sec
extern "C" type Header struct pack(1) minAlign(8) {
    kind: c::uchar,
    value: c::uint,
}
```

**§ 5(2).** They constrain **different things**: `pack(1)` caps normal field-placement alignment, while `minAlign(8)` requires at least 8-byte alignment of the containing object. The latter does not restore the fields' unmodified natural alignment.

**§ 5(3).** The aggregate's total size/stride and tail padding must satisfy the resolved final alignment and the selected target/ABI layout contract. Any impossible or inconsistent combination is a compile-time error. No backend may silently choose a different placement.

---

## § 6. D5 — Explicit field alignment and nested-type guarantees (approved / derived)

**§ 6(1).** A stored field's explicit `minAlign(N)` overrides the **normal field-alignment cap** imposed by the enclosing `pack(M)`:

```sec
extern "C" type Packet struct pack(1) {
    tag: c::uchar,
    payload: c::uint minAlign(8),
}
```

**§ 6(2).** That override may add padding before the field and raise the enclosing struct's required alignment. It never changes the field type's nominal identity or internal representation.

**§ 6(3).** An explicitly declared **type-level** `minAlign` on the nested field's type is also an alignment guarantee that must be preserved. The enclosing `pack` cannot invalidate it. This is a consequence of §§ 3(4), 4(1), and 6(1), **not** an additional open MD decision:

```sec
type Block struct minAlign(16) {
    value: uint64,
}

type Container struct pack(1) {
    tag: byte,
    block: Block,
}
```

`Container.block` must be suitably aligned for `Block`. The compiler may add required padding and raise `Container`'s alignment accordingly; it may not quietly reinterpret `Block` as less aligned.

**§ 6(4).** The rules do **not** claim that every packed C compiler/layout behaves this way. If the actual selected foreign ABI contract places a nested field at an offset that violates Sec's explicit `minAlign` guarantee, the **FFI layout contract is incompatible** and must be rejected. The compiler must not claim ABI equivalence merely from `extern "C"` syntax or from a superficial match of field names and sizes.

**§ 6(5).** Natural field alignment, explicit type minima, explicit field minima, packing, explicit offsets and the selected ABI's aggregate requirements are distinct **resolved layout facts**. A lowering pass must not reconstruct or override their precedence independently.

---

## § 7. D6 — `byteOffset(N)` on stored fields (approved)

**§ 7(1).** `byteOffset(N)` requests that a stored struct field begin **exactly N bytes from the start of the containing struct object**. `N` is a compile-time-known **nonnegative integer**, in bytes. Unlike an alignment, the offset need not be a power of two.

**§ 7(2).** It is permitted on stored fields of complete ordinary Sec and `extern "C"` structs. It is not a property accessor, ordinary computed member, bit-position directive, register field or independent pointer arithmetic operation.

**§ 7(3).** An explicit offset can require padding before a field. It cannot reorder source fields, overlap ordinary stored fields, violate a field's effective alignment requirements, silently truncate any field, or create a representation outside supported target constraints. An overlap requiring union/overlay semantics must use a separately valid union/overlay contract, not `byteOffset` on an ordinary struct.

**§ 7(4).** A target whose ABI or physical layout cannot satisfy the requested exact placement must reject it, with both requested and required/actual placement information where available. An explicit byte offset never relaxes borrow, pointer validity, initialization or storage-lifetime rules.

**§ 7(5).** Existing raw-string struct-field tags retain their syntax and meaning; modifiers appear **before** a tag:

```sec
type Record struct {
    id: uint32 byteOffset(8) `db:"id"`,
}
```

---

## § 8. D7 — Endianness and existing foreign representation (approved scope boundary)

**§ 8(1).** For ordinary structs and C-compatible foreign structs, byte order follows the resolved target and ABI. MD-019 introduces **no general `endian(...)` or `@endian(...)` struct/field modifier**.

**§ 8(2).** The existing `register[N]` language facility has its own independent bit-allocation and byte-order semantics, including `lsb-first`, `msb-first`, `little-endian` and `big-endian`. This correction neither rewrites that syntax nor treats register bit layout as C struct packing. Protocol/file serialization uses the mechanisms already owned by the relevant representation/serialization rules.

**§ 8(3).** `extern "C"` remains the existing explicit C ABI declaration mechanism. No `repr(C)` or `@repr(C)` alias is added. Applying `pack`, `minAlign` or `byteOffset` to an ordinary Sec struct does **not** grant FFI-legal representation.

---

## § 9. Canonical grammar synchronization

**§ 9(1).** Extend the existing `grammar.md` **"Struct type declaration"** and **foreign struct** productions without replacing ordinary struct, register, field-tag, interface or foreign declaration grammar. The following is the intended production *delta* (existing non-layout productions remain unchanged):

```ebnf
StructTypeDeclaration
    ::= "type" Identifier [ GenericParameterList ]
        "struct" { StructLayoutModifier }
        [ ImplementsClause ] StructBody

ForeignStructTypeDeclaration
    ::= "extern" ForeignABI "type" Identifier
        "struct" { StructLayoutModifier }
        [ ForeignStructBody ]

StructField
    ::= Identifier ":" TypeReference
        { StructFieldLayoutModifier } [ StructTag ]

StructLayoutModifier
    ::= "pack" "(" Expression ")"
      | "minAlign" "(" Expression ")"

StructFieldLayoutModifier
    ::= "minAlign" "(" Expression ")"
      | "byteOffset" "(" Expression ")"
```

**§ 9(2).** `Expression` in these modifier positions is a `SemanticCompileTimeRequiredContext` with integer result, under `rules/compiler/compile_time_evaluation.md` §§ 4–8. This permits valid constant expressions, not merely decimal integer literals, without permitting runtime values. Where a generic-dependent layout is not yet concrete, evaluate and validate it when the concrete specialization and `CompilationPlan` are available; never guess the result from the compiler host.

**§ 9(3).** This grammar delta preserves existing `implements` placement, comma-separated fields, optional trailing commas, raw `StructTag`, and the bodyless incomplete foreign struct form. A **bodyless** incomplete foreign struct cannot provide an actual stored-layout contract, so layout modifiers requiring a complete layout are invalid there rather than fabricating size or offsets.

**§ 9(4).** This production is a **modifier-specific contextual grammar**, not function invocation and not an expression- or statement-level attribute. There is no leading `@`. The formatter writes no space between modifier name and `(`, places one space between successive modifiers and before `{`, and preserves the existing field/tag formatting rules.

**§ 9(5).** Attribute-system changes are limited to clarifying that these forms are **not** entries in the closed `@` registry; no new general nested-attribute support is authorized. Existing register declaration modifiers and C bitfield grammar retain their own meanings.

---

## § 10. Layout resolution, ABI safety, and diagnostics

**§ 10(1).** Sema must retain each written modifier as a typed, source-located, validated requirement. The canonical target/plan-keyed `ResolvedLayout` service must calculate final field alignments, exact offsets, aggregate alignment, size, padding, stride and support/validity facts. Parser recognition alone does not establish a valid layout.

**§ 10(2).** **Alignment precedence:** a field's normal/natural alignment can be capped by an enclosing `pack`, but any explicitly required minimum alignment of the field or its nominal type remains in force. Type-level `minAlign` independently sets a lower bound for the enclosing struct. Explicit `byteOffset` must satisfy the resulting effective requirements. Existing ABI and target constraints remain additional requirements, not an implicit override of Sec's explicit minima.

**§ 10(3).** The type's own internal layout remains distinct from its placement as a field. This applies recursively. A generic specialization uses the concrete type and target `CompilationPlan`; caches must not substitute an incompatible ABI, profile or layout context.

**§ 10(4).** `extern "C"` retains C ABI legality checks. Explicit modifiers must be checked against **the foreign contract on the chosen target**; contradictory or unsupported layouts fail closed. The compiler must not silently translate a nonconforming Sec representation into a foreign struct by assuming compiler-specific packing extensions exist or by reordering fields.

**§ 10(5).** A packed/misaligned field must not be used to form an invalid `ref T` or `ref mut T`. Target-supported unaligned access may be lowered only where the owning field/reference rules permit it. C bitfield access and layout stay ABI-owned; no ordinary byte-offset modifier becomes a bitfield layout mechanism.

**§ 10(6).** A diagnostic should name the exact declaration or field, the written modifier and argument, the selected target/ABI where relevant, and why the constraint cannot be met. Cases include: invalid compile-time argument, non-power-of-two alignment, unsupported alignment, duplicate/conflicting modifier, offset overlap, offset versus alignment, unsatisfiable nested-layout guarantee, incompatible FFI layout, illegal placement, and misaligned safe reference. Stable diagnostic codes belong to the existing diagnostics registry; **do not invent numeric IDs** in this correction.

**§ 10(7).** The formatter, LSP and `sec analyse` must consume the same parser/Sema/layout facts. `rules/analysis/parameter_usage_analysis.md` may use resolved source-declared layout constraints to block unsafe narrowing advice, without claiming a prospective view or reference satisfies a foreign alignment/representation contract unless proven.

---

## § 11. Required conformance matrix (design-to-implementation handoff)

**§ 11(1).** Every case below needs parser, Sema, layout/IR, compiler and LSP coverage as appropriate. The examples illustrate the approved contract; they are **not** a claim that the current compiler accepts them.

| Case | Expected outcome |
| --- | --- |
| Ordinary struct without modifiers | Existing native behavior unchanged. |
| Complete `extern "C"` struct without modifiers | Existing target C ABI layout unchanged. |
| `struct minAlign(16)` | Type/object alignment at least 16 bytes if target supports it. |
| Field `minAlign(32)` | Correctly aligned field and containing aggregate. |
| `struct pack(1)` | Ordinary field alignments capped at 1 byte; retained internal nested-type layout. |
| `struct pack(1) minAlign(8)` | Normal fields packed; aggregate alignment at least 8. |
| `struct pack(1)` with field `minAlign(8)` | Field explicit minimum wins; compatible aggregate layout. |
| Nested `Block struct minAlign(16)` inside outer `pack(1)` | Nested type guarantee preserved; padding/alignment resolved; C ABI mismatch rejected when applicable. |
| Field `byteOffset(0)` | Legal only if declaration order, non-overlap and alignment permit it. |
| Field `byteOffset(8)` | Exact byte displacement, target-valid layout. |
| Field `minAlign(4) byteOffset(6)` | Invalid: exact offset contradicts required field alignment. |
| Fields with overlapping explicit offsets | Invalid for ordinary structs. |
| `minAlign(0)`, `minAlign(3)`, `pack(0)`, `pack(6)` | Invalid compile-time value. |
| `byteOffset(-1)` or non-constant argument | Invalid compile-time value. |
| Duplicate same-kind layout modifiers | Invalid with useful source locations. |
| Modifier on bodyless `extern "C" type Opaque struct` | Invalid: no concrete stored layout. |
| Raw field tag after `minAlign` or `byteOffset` | Valid; raw tag identity/data unchanged. |
| `@minAlign(8)`, `@pack(1)` or `@byteOffset(8)` in place of inline modifiers | Invalid: not part of the closed `@` attribute set. |
| `register[16] msb-first big-endian` | Existing register semantics unchanged. |
| Foreign struct whose actual ABI cannot realize explicit constraints | Compile-time rejection, no backend guess. |
| Misaligned `ref` / `ref mut` to field | Rejected unless required alignment proven. |
| A concrete generic struct specialization with compile-time modifier expression | Correct plan-keyed layout; no cross-target cache reuse. |

**§ 11(2).** Tests must cover representative Linux and Windows hosted ABI plans, including target-specific C scalar sizes, independent compilation plans, packed aggregates, nested structs and explicit offsets. Tests must verify correct Semantic IR/LLVM layout or an explicit unsupported-lowering diagnostic, not just successful parsing. Include malformed/recovery tests and formatter idempotence.

**§ 11(3).** Regression coverage must retain canonical behavior for struct tags, struct default/copy/move/ownership/destruction, register width/order modifiers, C bitfields, foreign type declarations and the existing generic/`implements` declaration grammar.

---

## § 12. Sync map, scope boundaries and closure conditions

**§ 12(1).** **Canonical rules to amend, with stable paragraph references preserved:**

1. `rules/foundations/grammar.md`: `StructTypeDeclaration`, `StructField`, `ForeignTypeDeclaration`; contextual parsing of layout modifiers, their ordering and tag interaction.
2. `rules/declarations/struct.md`: §§ 3–4, 15–17, 22–23, stored-field modifier placements, normal versus FFI layout, constructor and field-tag preservation.
3. `rules/memory/layout.md`: §§ 1(12), 4.3, 8, 11, 13, 26–30, 33–36, 38 and 44; replace *syntax not yet specified* statements with the decided form, retain separation of native/explicit/FFI layout.
4. `rules/platform/ffi.md`: §§ 15, 20, 23, 45–50; preserve existing `extern "C"` meaning and bitfield ABI boundaries; add modifier validation and incompatibility diagnostics.
5. `rules/foundations/attributes.md`: explicitly distinguish inline struct modifiers from the closed `@` attribute registry, **without** expanding attribute attachment to struct fields.
6. `rules/compiler/compile_time_evaluation.md`, `rules/memory/arena.md`, `rules/memory/allocation.md`, `rules/compiler/semantic_ir.md`, `rules/analysis/parameter_usage_analysis.md`, plus compiler diagnostics/parser-recovery/formatter/LSP rules: refer to the source declarations and resolved layout facts without duplicating ownership.

**§ 12(2).** **Existing governance IDs to update, never duplicate:**

- `governance/memory.yaml`: `frontend.layout`, `semantic-ir.layout`, `lowering.layout`, `platform.layout`, `tooling.layout`.
- `governance/declarations.yaml`: `frontend.structs`.
- `governance/ffi.yaml`: `frontend.ffi-v2`.
- `governance/allocation.yaml`: existing allocation/frontend/tooling owners for alignment checks and diagnostics.
- `governance/analysis.yaml`: existing parameter-usage and layout-analysis entries.
- Existing parser, CST/formatter, Sema, diagnostics and LSP integrations as indexed in `governance/index.yaml`; use their exact existing IDs.

**§ 12(3).** The following **remain explicitly outside** MD-019: a new `repr(C)`/`@repr(C)`, generalized per-field endian overrides, a new serializer, redefining `register[N]`, bare-metal allocation policy, redesign of the general attribute system, new ABI conventions, new implicit unsafe reference conversion, and stdlib design. Existing rulebooks govern these subjects.

**§ 12(4).** `missing-decisions.yaml` retains MD-019 as open until the owning canonical rulebooks express the final syntax and semantics without contradiction. Then resolve the existing MD-019 register entry per the repository's missing-decision workflow; do not create MD-019 duplicates or treat pending implementation as a new missing language decision.

**§ 12(5).** Integration must distinguish **approved specification**, **implemented frontend**, **resolved layout**, **verified ABI compatibility**, **lowering support**, and **tooling/test conformance**. A completed specification is not evidence that all compiler packages are complete.

**§ 12(6).** This correction is normative for **Sec 0.1**. Any future extension to union overlays, explicit per-field endian, freestanding profile contracts or other representation modifiers requires its own independently justified language change; none is inferred from the three modifiers approved here.
