# Layout

- Status: Normative
- Created: 2026-09-02
- Last updated: 2026-09-02
- Document revision: 2.0
- Sec language version: 0.1
- Canonical path: `rules/memory/layout.md`
- Replaces: previous revision of `rules/memory/layout.md`
- Repository baseline reviewed: `814a584` (latest publicly verifiable `main`; current `main` contents reviewed 2026-09-02)

---

## § 1 Purpose and authority

§ 1(1) This rulebook defines the physical representation of materialized Sec values within storage.

§ 1(2) Layout answers size, alignment, field placement, stride, padding, union tag/payload placement, register storage width, descriptor representation, and representation-validity questions.

§ 1(3) Layout does not determine ownership, storage origin, storage lifetime, allocation, callable ABI classification, serialization, reference validity, or transferability.

§ 1(4) `storage.md` owns storage origin/lifetime, backing storage, relocation, memory spaces, and reclamation.

§ 1(5) Type/declaration rulebooks own semantic type identity and declaration semantics.

§ 1(6) `reference_model.md` and `references.md` own safe-reference semantics and profile-selected reference representation requirements.

§ 1(7) `allocation.md` owns allocation operations/failure.

§ 1(8) `destruction.md` owns cleanup/destruction.

§ 1(9) FFI/ABI rulebooks own foreign representation compatibility and callable ABI classification.

§ 1(10) Target-profile rules own target pointer width, target data layout, endianness, supported alignments, address spaces, and target-specific representation constraints.

§ 1(11) Semantic IR may remain target-independent until a concrete `CompilationPlan` is resolved; layout-sensitive lowering must consume Sec-resolved layout rather than inventing it.

§ 1(12) This rulebook does not by itself introduce final syntax for packing, alignment, offsets, endianness, explicit representation, or general reinterpretation.

---

## § 2 Core principles

§ 2(1) Layout is resolved for one concrete `CompilationPlan`.

§ 2(2) Sec distinguishes semantic layout, native layout, and explicit layout.

§ 2(3) Materialized struct fields retain declaration order unless an explicit future layout contract canonically says otherwise.

§ 2(4) Native padding is not semantic data.

§ 2(5) Physical layout similarity does not imply type compatibility.

§ 2(6) Storage layout and callable ABI are separate concerns.

§ 2(7) A value must have complete sized layout before it may be stored by value.

§ 2(8) Size and alignment do not imply that every bit pattern is a valid value.

§ 2(9) Initialization is separate from layout.

§ 2(10) Direct recursive by-value layout is invalid.

§ 2(11) Generic layout is resolved per concrete instantiation.

§ 2(12) Backends consume Sec-resolved layout and must not redefine it independently.

---

## § 3 CompilationPlan

§ 3(1) Every concrete native layout is resolved under one `CompilationPlan`.

§ 3(2) Layout-relevant plan facts include at least:

```text
target architecture
target pointer width
target data layout
target endianness
selected ABI
target profile
supported scalar alignments
supported aggregate alignments
supported unaligned accesses
compiler layout rules
explicit layout contracts
concrete generic arguments
selected reference representation
selected descriptor representation
```

§ 3(3) Two compilation plans may produce different native layouts for the same semantic type.

§ 3(4) A layout query that depends on the target must not be evaluated from compiler-host layout.

§ 3(5) Layout caches must include every plan fact that can alter the result.

---

## § 4 Semantic, native, and explicit layout

### § 4.1 Semantic layout

§ 4.1(1) Semantic layout records representation requirements fixed by the language/type independent of one target.

§ 4.1(2) Examples include field declaration order, fixed-array element count, enum underlying type when explicit, register semantic bit width, and union variant set/order.

### § 4.2 Native layout

§ 4.2(1) Native layout is the target-resolved physical representation selected by Sec under a concrete `CompilationPlan`.

§ 4.2(2) Native layout may include target-dependent padding, alignment, pointer width, descriptor representation, and tag placement.

§ 4.2(3) Native layout is not a portable serialization format.

### § 4.3 Explicit layout

§ 4.3(1) Explicit layout is a user/platform/FFI contract constraining physical representation beyond ordinary native layout.

§ 4.3(2) Explicit layout contracts must be validated against target capabilities and semantic type validity.

§ 4.3(3) An impossible explicit layout contract is a compile-time error.

§ 4.3(4) This rulebook defines the semantics such contracts must preserve without locking all source syntax.

---

## § 5 Complete and incomplete layout

§ 5(1) A layout is complete when size, alignment, and all required representation facts are resolved for the intended use.

§ 5(2) A layout may remain incomplete while semantic analysis has insufficient target or generic-instantiation information.

§ 5(3) Incomplete layout is permitted only where no operation requires complete physical layout yet.

§ 5(4) By-value storage, field-offset queries, ABI materialization, raw memory access, and layout-sensitive lowering require complete layout.

§ 5(5) An incomplete type/layout must not be assigned guessed size/alignment.

---

## § 6 Sized values

§ 6(1) A by-value materialized type must have finite target-representable size.

§ 6(2) Zero-sized types are sized.

§ 6(3) Runtime-sized backing storage does not make a descriptor value unsized.

§ 6(4) A bare owning dynamic array `T[]` is a sized descriptor value; its runtime-sized element backing is separate storage.

§ 6(5) A safe slice `ref T[]` or `ref mut T[]` is a sized non-owning descriptor/reference representation selected by the reference/layout model.

§ 6(6) Older wording describing bare `T[]` itself as unsized is non-canonical.

---

## § 7 Size

§ 7(1) `SizeOf(T)`/`T.SizeOf` returns the byte size of the complete materialized representation of `T` for the active plan where the query is canonical.

§ 7(2) Size excludes separate backing storage unless that storage is embedded in the value representation.

§ 7(3) Size includes required padding within the representation.

§ 7(4) Size computation must use checked arithmetic.

§ 7(5) A size that cannot be represented by the target/compiler layout model is a compile-time error.

§ 7(6) Size of a descriptor is the descriptor size, not the total storage reachable through it.

---

## § 8 Alignment

§ 8(1) Alignment is the placement constraint required for a materialized value of `T`.

§ 8(2) Native alignment is target/plan dependent unless a semantic or explicit contract constrains it.

§ 8(3) Aggregate alignment is derived from Sec layout rules and target capabilities.

§ 8(4) Alignment is not automatically equal to size.

§ 8(5) Alignment requests stricter than target support are rejected unless a target-defined mechanism provides them.

§ 8(6) Misaligned safe reference formation is invalid.

§ 8(7) Unsafe unaligned access requires explicit target-supported semantics; `unsafe` does not manufacture target support.

---

## § 9 Stride

§ 9(1) Stride is the byte distance between adjacent elements of an array-like materialized sequence.

§ 9(2) Fixed-array stride must satisfy element size/alignment and target layout rules.

§ 9(3) Stride may exceed element semantic data size because of padding/alignment.

§ 9(4) Raw-pointer `Offset(elements)` uses canonical `sizeof/stride` semantics required by the raw-pointer rule.

§ 9(5) A layout query for stride requires complete layout.

---

## § 10 Padding

§ 10(1) Padding is physical representation space not belonging to semantic stored fields/payload data.

§ 10(2) Padding bytes are not semantic values.

§ 10(3) Ordinary equality/hash/serialization must not treat unspecified padding contents as semantic data unless a canonical representation contract explicitly defines otherwise.

§ 10(4) Copy lowering may copy padding where representation-safe, but doing so must not make uninitialized padding observable through safe language semantics.

§ 10(5) FFI/export of padding-sensitive representations must prevent uninitialized-data leakage where required by the ABI/security contract.

§ 10(6) Explicit layout may constrain padding ranges.

---

## § 11 Field placement

§ 11(1) Materialized struct fields retain source declaration order under native Sec layout.

§ 11(2) Native layout may insert padding between or after fields.

§ 11(3) The compiler must resolve canonical field offsets before layout-sensitive lowering.

§ 11(4) A property is not stored instance layout unless its owning declaration rule explicitly defines backing storage.

§ 11(5) `impl` members do not add instance fields.

§ 11(6) Type-associated static storage is not part of instance layout.

---

## § 12 Fixed arrays

§ 12(1) `T[N]` is a fixed-size aggregate containing exactly `N` element slots.

§ 12(2) Fixed-array length is compile-time known.

§ 12(3) Zero-length fixed arrays are valid when the element type/layout rules permit the type.

§ 12(4) Fixed arrays contain no hidden pointer, length, capacity, or allocator field.

§ 12(5) Total size is computed from canonical element stride and count with checked arithmetic.

§ 12(6) Embedded fixed arrays follow enclosing storage placement and relocation.

---

## § 13 Struct layout

§ 13(1) Struct layout contains stored fields in declaration order.

§ 13(2) Field semantic identity is independent of physical offset.

§ 13(3) Native struct layout may add padding but must not reorder fields.

§ 13(4) Struct total alignment/size are resolved canonically for the plan.

§ 13(5) Nested structs use their resolved nested layouts.

§ 13(6) Direct recursive by-value struct cycles are invalid.

§ 13(7) Recursive indirection through reference/raw pointer/descriptor/owning dynamic storage does not itself create a by-value layout cycle.

---

## § 14 Enum layout

§ 14(1) A fieldless enum with a selected underlying scalar representation occupies that representation unless an explicit canonical rule requires more.

§ 14(2) Explicit enum values must be representable in the selected underlying type.

§ 14(3) Bit-backed enums use their canonical bit width and representation rules.

§ 14(4) String-underlying enums are semantic enum values and do not imply that the enum is represented as a C integer.

§ 14(5) Layout of string-underlying or otherwise non-integer enum representations must follow the canonical enum/type representation contract.

§ 14(6) The backend must not silently force default enums to `i32` when Sec semantics/CompilationPlan require another representation.

---

## § 15 Tagged-union layout

§ 15(1) A tagged union requires representation for an active-variant discriminator and sufficient storage for the selected active payload.

§ 15(2) Variant semantic identity/order is declaration-defined.

§ 15(3) Native tag width/placement may be target/layout dependent.

§ 15(4) Payload storage size/alignment must accommodate every possible payload.

§ 15(5) Padding and inactive payload bytes are not semantic data.

§ 15(6) The compiler must not read inactive payload storage as another variant.

§ 15(7) Direct recursive by-value union layout is invalid.

§ 15(8) Niche/tag-elision optimization is permitted only when the language/reference/type contract proves every semantic invariant and representation-validity rule remains preserved.

---

## § 16 Result and Option

§ 16(1) `Result[T,E]` and `Option[T]` are tagged/variant values whose physical representation may use canonical layout optimizations.

§ 16(2) Their semantic alternatives remain explicit even if physical tag representation is optimized.

§ 16(3) Layout optimization must not change matching, ownership, destruction, reference, or FFI semantics.

§ 16(4) A backend may not invent a niche optimization independently of Sec-resolved layout.

---

## § 17 Register layout

§ 17(1) Register declarations have exact semantic bit widths defined by register rules.

§ 17(2) Register bit fields are semantic hardware bit positions, not ordinary struct field offsets.

§ 17(3) Target-specific storage unit, alignment, access width, and endianness must be validated before physical hardware access.

§ 17(4) Active-high/active-low interpretation is not layout semantics.

§ 17(5) Addressed register storage uses platform hardware-access contracts in addition to layout.

---

## § 18 Scalar layout

§ 18(1) Fixed-width integer types have their semantic widths.

§ 18(2) `int` and `uint` use the canonical target/profile pointer-sized integer rule.

§ 18(3) Backend paths must not hardcode `int`/`uint` to a host or legacy width.

§ 18(4) `bool`, rune/character types, floats, decimal types, and other scalars use their canonical type representation rules.

§ 18(5) Representation-validity constraints remain distinct from size.

---

## § 19 Decimal representation

§ 19(1) Canonical decimal representations are:

```text
decimal:
    { i64, i32 }

decimal128:
    { i128, i32 }
```

§ 19(2) Legacy direct LLVM representations that differ from the canonical representation are implementation debt, not alternative language semantics.

§ 19(3) Field placement/alignment within these composite scalar representations is resolved by Sec under the plan.

---

## § 20 String layout

§ 20(1) String semantic behavior is owned by string/core rules.

§ 20(2) The materialized string descriptor representation is selected by the canonical string/reference/layout model for the plan.

§ 20(3) A string descriptor does not include separate backing storage in `SizeOf(string)`.

§ 20(4) Static literal storage, borrowed/view storage, and owning materialized storage may have different storage contracts without changing semantic string type identity where allowed.

§ 20(5) Layout must not assume every string owns heap storage.

---

## § 21 Collection descriptors

§ 21(1) Owning dynamic collection values are sized descriptors unless their canonical collection rule defines a different embedded representation.

§ 21(2) Descriptor layout includes only descriptor state physically stored in the value.

§ 21(3) Element backing storage is separate unless explicitly embedded.

§ 21(4) Capacity/length fields and pointer/reference representation are defined by the canonical collection/layout/profile model.

§ 21(5) Safe slice/view descriptors are non-owning and must preserve reference-model validity requirements.

---

## § 22 Reference layout

§ 22(1) Source-level `ref T` and `ref mut T` semantics do not require one universal physical representation.

§ 22(2) A proven reference may lower to a machine address when all required guarantees are statically established.

§ 22(3) Profiles may require generation/epoch/capability/address-space metadata.

§ 22(4) Layout records must preserve the selected representation needed by reference semantics.

§ 22(5) Safe-reference layout is distinct from `RawPtr[T]` layout even where both happen to be one machine word on a target.

---

## § 23 RawPtr layout

§ 23(1) `RawPtr[T]` uses the selected target raw-pointer/address-space representation.

§ 23(2) Raw-pointer representation must preserve target pointer width and address-space semantics.

§ 23(3) Capability/tagged/non-flat targets may require richer raw-pointer representation than an integer.

§ 23(4) Layout must not derive safe-reference metadata from raw-pointer representation.

---

## § 24 Zero-sized types

§ 24(1) Zero-sized types have size zero but remain semantic values/types.

§ 24(2) Zero-sized fields/elements must not create invalid overlapping semantic identity merely because they consume no bytes.

§ 24(3) The implementation may assign canonical addresses/offsets for zero-sized materialization only when alias/reference semantics remain valid.

§ 24(4) Arrays of zero-sized elements retain their semantic element count.

§ 24(5) Pointer/reference arithmetic over zero-sized element types requires a dedicated canonical rule and must not be inferred from division by zero or arbitrary address changes.

---

## § 25 Recursive layout

§ 25(1) Layout resolution uses a by-value dependency graph.

§ 25(2) A cycle containing only by-value containment with no indirection/storage boundary is invalid.

§ 25(3) Diagnostics must show the recursive layout path.

§ 25(4) Recursive types through reference/raw-pointer/owning dynamic indirection may be valid because the recursive payload is not embedded by value.

§ 25(5) Generic instantiation may introduce a cycle even when the generic declaration alone does not.

---

## § 26 Generic layout

§ 26(1) Generic layout is resolved for each concrete instantiation that requires physical layout.

§ 26(2) Layout cache keys must include concrete semantic type identity and relevant `CompilationPlan`.

§ 26(3) A generic type may remain layout-incomplete before instantiation.

§ 26(4) Constraints requiring sized/FFI-compatible/explicit-layout behavior must be checked after or through sufficient layout proof.

§ 26(5) Separate compilation may serialize validated layout requirements/summaries but must not trust stale incompatible cache data.

---

## § 27 Packing

§ 27(1) Packing is an explicit layout contract that reduces or removes ordinary padding/alignment placement.

§ 27(2) Sec 0.1 layout semantics define packing behavior but this rulebook does not lock a final source spelling unless another canonical declaration/attribute rule does so.

§ 27(3) Packing must not permit formation of an invalid aligned safe reference to a misaligned field.

§ 27(4) Access to packed/misaligned fields may require compiler-generated unaligned operations or explicit unsafe/platform operations where supported.

§ 27(5) Packing does not change semantic field identity/order.

---

## § 28 Explicit alignment

§ 28(1) An explicit alignment contract may raise or otherwise constrain alignment according to target rules.

§ 28(2) Requested alignment must be target-supported.

§ 28(3) Explicit alignment affects placement/size but not semantic type compatibility by itself.

§ 28(4) Over-aligned allocation must use an allocation/storage provider capable of satisfying the resolved alignment.

---

## § 29 Explicit field offsets

§ 29(1) Explicit field offsets constrain field placement.

§ 29(2) Offsets must not violate field size, required alignment unless explicitly permitted, total representation bounds, or overlap rules.

§ 29(3) Overlapping stored fields require a union/overlay representation explicitly defined by a canonical rule; ordinary structs must not accidentally overlap fields.

§ 29(4) Explicit offsets must be validated before FFI/hardware use.

---

## § 30 Endianness

§ 30(1) Target endianness is a `CompilationPlan` fact.

§ 30(2) Native integer/scalar layout follows target endianness where observable through low-level representation operations.

§ 30(3) Endianness is not ordinary semantic numeric value identity.

§ 30(4) Explicit endian representation contracts may constrain byte layout for FFI/protocol/hardware representations.

§ 30(5) Serialization/network order remains owned by serialization/protocol APIs unless an explicit layout contract is used.

---

## § 31 Representation validity

§ 31(1) A sized/aligned bit pattern is not automatically a valid value of `T`.

§ 31(2) Representation validity includes enum discriminants, union active state, bool/character/rune validity, named-type constraints where representation-relevant, safe-reference invariants, and other canonical type invariants.

§ 31(3) Raw/uninitialized storage must not become readable `T` until validity is established.

§ 31(4) Padding bytes are excluded from semantic validity unless an explicit contract says otherwise.

§ 31(5) Unsafe construction may accept proof obligations but cannot make a compiler-proven invalid representation valid.

---

## § 32 Initialization and padding

§ 32(1) Object initialization establishes semantic fields/payloads, not arbitrary semantic values in padding.

§ 32(2) Safe code must not observe uninitialized padding as semantic data.

§ 32(3) FFI/export/copy-to-byte-buffer operations must respect padding-leak rules of their owning contracts.

§ 32(4) Zero-initialization is valid for `T` only when `T`'s representation rules define the all-zero representation as valid.

§ 32(5) Semantic default initialization is not synonymous with zero-filled bytes.

---

## § 33 Layout compatibility

§ 33(1) Two types having equal size/alignment/field offsets does not make them safely interchangeable.

§ 33(2) Layout compatibility is a separate verified relation for a specific purpose such as FFI or explicit representation conversion.

§ 33(3) Compatibility must include representation validity, alignment, field/variant meaning, endianness, and relevant ABI/target rules.

§ 33(4) Native layout coincidence across compiler versions/targets is not a stability guarantee.

---

## § 34 Layout stability

§ 34(1) Layout stability describes the scope over which a representation is guaranteed not to change.

§ 34(2) Useful categories may include:

```text
NativeUnstable
TargetStable
ContractStable
```

§ 34(3) Exact internal names are implementation details.

§ 34(4) Native layout is not automatically stable across target, compiler version, profile, or ABI.

§ 34(5) FFI/on-disk/network/shared-memory contracts requiring stability must use an explicit canonical stable representation contract.

---

## § 35 Layout queries

§ 35(1) Canonical layout queries may include `SizeOf`, `AlignOf`, `StrideOf`, and `FieldOffset` when defined by compiler-known-member rules.

§ 35(2) A query requiring target-resolved layout must be evaluated under a concrete plan.

§ 35(3) Queries must fail at compile time when the requested layout is incomplete or unavailable.

§ 35(4) Layout queries do not make an otherwise unstable representation stable.

§ 35(5) `SizeOf` is not allocation size of reachable backing storage.

---

## § 36 FFI layout

§ 36(1) FFI-safe representation requires explicit ABI/layout compatibility with the foreign contract.

§ 36(2) Sec native layout must not be assumed C-compatible merely because sizes appear equal.

§ 36(3) Foreign struct/union/enum layout must be validated against the selected ABI.

§ 36(4) Foreign padding/alignment/packing/endianness requirements must be represented explicitly.

§ 36(5) FFI function ABI classification is owned by ABI rules and consumes resolved storage layout.

---

## § 37 Hardware/register layout

§ 37(1) Register declarations use semantic bit layout plus target storage/access rules.

§ 37(2) Fixed-address placement does not change type layout but constrains storage/access.

§ 37(3) MMIO exact access width may be stricter than ordinary layout load/store width choices.

§ 37(4) Register views/mappings must consume canonical resolved register layout and hardware access contracts.

---

## § 38 Semantic IR requirements

§ 38(1) Plan-resolved Semantic IR must retain or reference every resolved layout required for later lowering.

§ 38(2) Relevant facts include:

```text
semantic type identity
resolved physical layout identity
size
alignment
field identities
field offsets
padding ranges where required
array stride
union tag/payload rules
descriptor representation
reference representation
representation validity
explicit layout contracts
source locations for user layout requirements
layout stability class
```

§ 38(3) Target-independent Semantic IR may retain unresolved layout requirements until a concrete plan is available.

§ 38(4) Before layout-sensitive MLIR/backend lowering, every required concrete layout must be attached/referenced through the canonical resolved layout model.

§ 38(5) Semantic IR verification must reject contradictory/incomplete layout facts where complete layout is required.

---

## § 39 ResolvedLayout model

§ 39(1) The compiler must converge on one canonical resolved-layout representation or equivalent shared service.

§ 39(2) A resolved layout must represent at least:

```text
type identity
CompilationPlan identity
size
alignment
layout completeness
field layouts
array stride
union representation
padding where needed
representation validity
stability/contract provenance
```

§ 39(3) A resolved field layout must preserve semantic field identity independently of offset.

§ 39(4) MLIR/LLVM data-layout queries may assist target calculation but are not a second normative layout engine.

---

## § 40 Lowering

§ 40(1) Lowering consumes Sec-resolved layout.

§ 40(2) MLIR/LLVM must not silently reorder fields, change enum width, reinterpret `int`, choose incompatible union tags, or alter explicit contracts.

§ 40(3) Lowering must preserve target-correct size/alignment/stride/offset.

§ 40(4) Required unaligned operations must use target-supported lowering rather than forming invalid aligned references.

§ 40(5) Backend optimizations may change physical placement only when representation-observable and address-sensitive semantics are preserved.

§ 40(6) Layout-specific metadata must not be discarded before every consumer has an equivalent lower-level representation.

---

## § 41 Representation observability

§ 41(1) Ordinary safe Sec code should not depend on unspecified native padding or backend-only representation details.

§ 41(2) Representation becomes observable through explicit layout queries, FFI, raw/unsafe access, serialization adapters, hardware/register access, inline assembly, or another canonical low-level operation.

§ 41(3) Once representation is observable, the compiler must preserve the corresponding canonical contract.

§ 41(4) Optimizations may not rewrite observable representation beyond the contract.

---

## § 42 Diagnostics

§ 42(1) Layout diagnostics must identify the type, target/plan, unresolved or conflicting requirement, and relevant field/variant/recursive path.

§ 42(2) Diagnostics should distinguish semantic-type incompatibility from physical layout incompatibility.

§ 42(3) Explicit-layout diagnostics should state requested versus supported offset/alignment/size where known.

§ 42(4) Recursive-layout diagnostics should show the by-value cycle.

§ 42(5) Padding/representation diagnostics should use programmer-facing language rather than backend jargon.

---

## § 43 LSP and tooling

§ 43(1) LSP and `sec analyse` must consume the same resolved layout service/facts as compilation.

§ 43(2) Tooling may expose size, alignment, field offsets, stride, layout class/stability, target dependence, and explicit contract provenance.

§ 43(3) Tooling must not calculate independent approximate layout from syntax alone.

§ 43(4) Layout information shown without a resolved target must clearly identify target dependence or unresolved state.

§ 43(5) Incremental invalidation must include target/profile, type declaration, generic instantiation, representation attribute, ABI, and reference/descriptor representation changes.

---

## § 44 Required test families

§ 44(1) Scalar tests include fixed integers, `int`/`uint` target width, floats, decimals, bool/character validity, and pointer/reference representations.

§ 44(2) Aggregate tests include field order, padding, alignment, fixed arrays, zero-length arrays, structs, enums, unions, Result/Option, and descriptors.

§ 44(3) Recursive/generic tests include valid indirection recursion, rejected by-value cycles, instantiated generic layout, and cache separation across plans.

§ 44(4) Explicit-layout tests include packing, alignment, offsets, endianness, impossible contracts, and misaligned field-reference rejection.

§ 44(5) FFI/hardware tests include C-compatible representations, non-compatible native layout rejection, register widths, exact MMIO access requirements, and fixed-address separation from layout.

§ 44(6) Representation tests include invalid discriminants, uninitialized padding leakage, zero-init validity, and raw-to-value validation.

§ 44(7) IR/lowering tests include canonical `ResolvedLayout`, no backend field reordering, plan-sensitive `SizeOf`, target-correct stride, and layout metadata preservation.

§ 44(8) Tooling tests include compiler/LSP parity.

---

## § 45 Completion criteria

§ 45(1) Frontend layout support is complete when every concrete Sec 0.1 materialized type obtains one canonical plan-resolved layout where required.

§ 45(2) Aggregate support is complete when offsets, padding, alignment, array stride, union tags/payloads, descriptor layouts, and representation validity are canonical.

§ 45(3) Explicit-layout support is complete when every supported contract is parsed by its owning syntax rule, validated against target capabilities, and represented in resolved layout.

§ 45(4) FFI/platform support is complete when foreign ABI and hardware/register access consume the same resolved layout.

§ 45(5) Semantic IR/lowering support is complete when layout-sensitive backends consume canonical Sec layout rather than redefining it.

§ 45(6) Tooling support is complete when compiler, LSP, diagnostics, and analysis use one layout service/fact model.

---

## § 46 Core summary

§ 46(1) Layout describes physical representation of a materialized value.

§ 46(2) Layout is resolved under a concrete `CompilationPlan`.

§ 46(3) Sec distinguishes semantic, native, and explicit layout.

§ 46(4) Native struct fields retain declaration order; padding is not semantic data.

§ 46(5) Physical similarity never implies type compatibility.

§ 46(6) A bare `T[]` is a sized owning descriptor; runtime element backing is separate storage.

§ 46(7) Safe-reference and raw-pointer representations are profile/target facts but remain semantically distinct.

§ 46(8) Direct recursive by-value layout is invalid; generic layout is resolved per instantiation.

§ 46(9) Representation validity is separate from size/alignment.

§ 46(10) Sema/plan resolution establishes canonical layout; Semantic IR preserves it; MLIR/LLVM/backends implement it without becoming a competing source of language semantics.
