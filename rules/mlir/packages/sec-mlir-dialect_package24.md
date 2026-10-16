# Sec MLIR Program — Package 24: Physical Value Representation and Layout Lowering

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec-mlir-dialect_package24.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P24`

---

**Target:** planned Sec MLIR dialect schema **v20**; planned lowering **v20** (after P23 planned v19).

**Dependency caution:** current checked-in dialect/lowering rulebooks describe v11; planning v20 is not an implementation claim.

**New decisions:** none identified; MD-019 and P23-D1–D3 remain existing gates.

---

## § 1. Mission and exact boundary

P24 builds the **single canonical target-resolved physical layout service** and consumes its results through verified Semantic IR, schema-v20 Sec MLIR, and intra-procedural representation lowering. It must make actual size/alignment/offset/stride/union-representation decisions under one `CompilationPlan`, not create a second frontend/MLIR/LLVM layout engine. Its primary outcome is one reproducible `ResolvedLayout` per `(concrete semantic type, relevant plan identity, representation contract)` plus representation-sensitive operations that lower correctly or fail closed.

P24 is **not** the final foreign/internal call ABI package and is not proof that P15–P23 are executable. Physical value *memory* representation is different from the physical calling convention.

## § 2. Actual predecessor and repository state

User-confirmed baseline: **P1–P14 implemented**, **P15 underway**, **P16/P17 corrected and planned**; actual execution state of P18–P23 must be established in the implementer's checkout. GitHub `main` was reviewed publicly 2026-10-16; exact SHA was not independently established through this tool. Before editing, record `git rev-parse HEAD`, `git status --short`, actual installed schema and tests. Do **not** replace the corrected local P18 or P16/P17 plans with older downloadable versions.

P24 targets **planned schema v20 / lowering v20**, successor to P23 planned v19, *not* the current checked-in schema. Public MLIR dialect/lowering rulebooks currently identify **v11** as the version installed in the repository. Phase-gate operations needing P15–P23 until they are executable.

## § 3. Normative authority

Canonical sources checked in the October tree:

```text
rules/memory/layout.md
rules/memory/storage.md
rules/memory/reference_model.md
rules/memory/references.md
rules/memory/copy_move.md
rules/memory/destruction.md
rules/compiler/compiler_pipeline.md
rules/compiler/semantic_ir.md
rules/compiler/compile_time_evaluation.md
rules/mlir/sec_mlir.md
rules/mlir/sec_mlir_dialect.md
rules/mlir/sec_mlir_lowering.md
rules/platform/abi.md
rules/types/types.md
rules/types/default_values.md
rules/compiler/compiler_known_members.md
governance/memory.yaml
```

Resolve actual paths by `git ls-files` before use. `rules/platform/abi.md` is the **owner** of ABI according to `sec_mlir.md` and `layout.md`; existence/content must be verified in checkout. Do not fabricate any missing rulebook. Apply `sec_layout_sync_package24.md`, `sec_semantic_ir_layout_package24.md`, `sec_mlir_dialect_package24.md`, and `sec_mlir_lowering_package24.md` as additive, non-renumbering amendments.

## § 4. Canonical division of responsibilities

`layout.md` owns native/semantic/explicit physical representations. `storage.md` owns origin, backing, lifetime, relocation and reclamation. `reference_model.md` owns safe reference guarantees and selected representation requirements. Destruction/ownership books own live-value transfers and terminal actions. `CompilationPlan` selects target features. FFI/ABI books own argument/result classification. `Sec MLIR` consumes resolved layout: no re-derivation.

Never infer that equal bytes imply equal Sec types or legal `reinterpret`, or that a pointer-shaped field is automatically a safe reference.

## § 5. Governance: update existing IDs

Synchronize findings into existing `governance/memory.yaml` IDs: `frontend.layout` (partial), `semantic-ir.layout` (planned), `lowering.layout` (partial), `platform.layout` (partial), `tooling.layout` (planned). These statuses were observed in public `main` on 2026-10-16; compare live checkout before editing. Add facts under the existing entries, do not duplicate them in a new namespace. `governance` is an implementation record, **not** a source of new language semantics.

## § 6. Required preflight inventory

Codex must locate the actual type/target/layout services using `rg` instead of assuming a nonexistent `internal/sema/layout.go`. Inspect current `internal/sema/types.go`, `internal/sema/semantic_facts.go`, target-plan types, `internal/ir/semantic/`, the actual `internal/lowering/` and `internal/codegen/` trees, and all MLIR TableGen/pass registration files. Record: existing layout calculators, source of `SizeOf`/`AlignOf`, cache keys, target widths, generic instantiation identity, and field offsets. Locate legacy hardcoded decimal/enum/int/float assumptions by search. **Reuse existing correct code; converge APIs**.

## § 7. Physical layout resolution phases

Use a two-phase service:

```text
semantic type + requirements
    -> canonical plan-independent layout requirements
    -> one concrete CompilationPlan
    -> resolved representation policy
    -> checked ResolvedLayout
    -> verified layout reference in Semantic IR / Sec MLIR
    -> intra-procedural representation lowering
```

A target-independent Semantic IR node may reference unresolved requirements, but no **layout-sensitive** lowering may proceed without a complete resolved layout. A missing requirement is a diagnostic/capability gap, not permission to guess from compiler host.

## § 8. ResolvedLayout identity and immutability

Recommended immutable record (adapt to repository data structures):

```go
type ResolvedLayout struct {
    ID             LayoutID
    SemanticType   TypeID
    PlanKey        CompilationPlanLayoutKey
    ContractID     LayoutContractID
    Completeness   LayoutCompleteness
    SizeBytes      BigUnsigned
    AlignBytes     BigUnsigned
    Kind           LayoutKind
    Fields         []ResolvedFieldLayout
    Array          *ResolvedArrayLayout
    Variant        *ResolvedVariantLayout
    Descriptor     *ResolvedDescriptorLayout
    Validity       RepresentationValidity
    Stability      LayoutStability
    Provenance     LayoutProvenance
}
```

Use canonical arbitrary-precision/checked integers until proving target representability; only then cast to backend APInt/index types. `LayoutID` is stable inside one canonical snapshot and must not be derived from display names or host pointers. Layout records are immutable after publication. `ResolvedFieldLayout` keeps **FieldIdentity** distinct from byte offset.

## § 9. CompilationPlan key and cache safety

Key must include concrete nominal/generic identity, target triple, pointer/int/float width, target data layout and endian, ABI-relevant representation limits (without deciding callable ABI), profile, memory/address space, alignment capability, explicit layout contract, descriptor/reference representation policy, compiler layout revision, and any plan feature that changes the result. Do not key only by source type string or file timestamp.

Add cold-vs-cached conformance: warm with 32-bit plan then query 64-bit, alter reference policy, switch address space, alter alignment, monomorphize different generic T. Stale/incompatible cache is a **miss/recompute**, not authority.

## § 10. Checked layout arithmetic

Implement shared checked operations over nonnegative exact integers:

```text
AlignUp(offset, alignment):
    require alignment > 0 and supported
    padding := (alignment - (offset mod alignment)) mod alignment
    result := CheckedAdd(offset, padding)

StrideOf(T):
    based on resolved element size/alignment and target placement rules

ArraySize(T,N):
    checked stride/count computation consistent with canonical zero-length policy
```

Do not do host-`int` or host-`size_t` arithmetic; bounds and target size addressability must be checked against `CompilationPlan`. Report which subexpression/field/generic instantiation overflowed. A zero-sized element does not erase the logical distinction of N elements.

## § 11. Scalar layouts

Implement semantic-to-representation selection for fixed ints 8/16/32/64/128/256, target-sized `int`/`uint`, `bool`, `byte`, `char`, `rune`, floats and decimals. The *physical* width is not the full type-validity predicate (notably bool/rune/named constraints). Scalar kind provenance must remain until no downstream verifier/debug/ABI consumer requires it.

Plain `float` is **target width dependent** under the October MD-014 correction: binary32 on 32-bit and binary64 on 64-bit profiles. Do not carry forward P20's older fixed-binary64 assumption. `float32` and `float64` retain fixed formats.

## § 12. Decimal representation

Apply the normative canonical forms from `layout.md §19`:

```text
decimal    := { i64 coefficient, i32 scale }
decimal128 := { i128 coefficient, i32 scale }
```

Resolve **field offsets, tail padding, aggregate size/alignment** from the same target plan. The braces above specify field types/order, not an ABI call classification. Distinguish the public decimal `.scale` semantics (P21) from the physical 32-bit scale field. Reject old LLVM-generator shapes that change canonical representation or truncate coefficient. P21-D1–D3 stay decision-gated if unresolved.

## § 13. Named and constrained types

Maintain nominal identity during Semantic IR, P22 contract validation, and debug. Carrier layout may be reused only where the existing named-type/contract rule permits representation equivalence and required representation validity is enforced. Never treat carrier equality as proof that contract validation may be omitted. Layout erasure does not imply semantic type erasure. When embedding a named field, use its resolved carrier layout **plus** exact validity summary and original nominal provenance.

## § 14. Struct layout

For stored fields in **declaration order**, resolve each field layout, compute aligned offset from previous end, record padding gap, compute aggregate alignment and checked final rounded size. Exclude properties/methods/nested types/static associated storage. For explicit valid offsets/packing (only after owning syntax decision), resolve the constraints instead of using native placement. No backend field reordering. Field identity is stable even if offset is zero or equal for zero-sized fields.

## § 15. Fixed arrays

`T[N]` embeds exactly N slots. No hidden pointer, capacity or length. `N` remains exact arbitrary-precision integer until checked against the target plan. Use the canonical element stride; zero-length arrays remain valid subject to element layout completeness. For very large N do not unroll construction or destructors into N operations. Element address computation uses checked `index × stride + base` with index bounds already proven/guarded in P14/P15.

## § 16. Recursive and generic layouts

Build a by-value containment graph. A cycle entirely through by-value edges is invalid and diagnostics show the path. `ref T`, `RawPtr[T]` and owning descriptor/backing indirections may break the by-value cycle. Generic instantiations resolve layout per concrete substituted type, never share a possibly incompatible cached layout for different substitutions. Use worklists/memoization with explicit `in-progress` cycle detection. Do not confuse a valid recursive *type declaration* with an invalid by-value *layout cycle*.

## § 17. Enums and representation validity

Resolve fieldless integer/bit-backed enum underlying representation from canonical enum definitions/target facts. Do not hardcode `i32`. Preserve enum discriminant constraints (including named aliases, unnamed integer values where canonical rules allow them), and do not truncate underlying values. String-underlying enums keep the canonical string representation contract; they are not converted to integers for convenience. Invalid enum representations cannot be synthesized by a zero-filled allocation unless proven valid.

## § 18. Tagged unions, Result and Option

Compute one layout for active discriminator, all possible payload layouts, maximal required payload extent/alignment, total size/padding, and active-state validity. Retain semantic variant IDs independent of numeric tag encoding. `Result`/`Option` may reuse the variant engine. Niche/tag-elision is **not** an automatic optimization: only apply a chosen canonical layout optimization when proof covers all valid/invalid representations and ownership/reference/destruction. Otherwise choose a fully explicit tag+payload resolved layout according to the plan. Inactive payload bytes are not legal values.

## § 19. Reference representations

Consume P15 `ReferenceValidityPolicy` and plan-selected physical reference representation, **not** `RawPtr` layout. An all-proven reference may become address-like when every temporal, spatial, provenance, alignment, address-space, borrow/relocation and concurrency obligation has been discharged. A dynamic-epoch reference needs an actual appropriate validation/storage representation or retaining high-level checks until later lowering; logical epoch default width is 64 even on a 32-bit target unless another proven rule qualifies. Never choose a universal `(ptr,epoch)` representation or erase a needed guard simply because the pointer fits one word.

## § 20. Slice descriptors

P16 `ref T[]` / `ref mut T[]` are **non-owning** slice values with logical extent, borrow authority, owner/backing identity and epoch dependency. The plan may select an address/length/capability/other safe descriptor. Empty slices need no dereferenceable element and must not invoke `&array[0]` when N=0. Pointer width is not a proof for length width or epoch width. The mutable slice remains move-only regardless of physical descriptor triviality.

## § 21. Owning dynamic array descriptors

P18 `T[]` owns elements and possibly individual backing reclamation; its descriptor stores only plan-selected physical fields needed for runtime length/capacity/provider/epoch/relocation. It is always semantically MoveOnly even if the descriptor is a trivially movable bunch of machine words. Distinguish descriptor size from backing size. No public `Capacity` API is invented. Do not conflate owning `T[]` with a non-owning slice.

## § 22. String descriptors

P23 `string` has immutable value semantics and may be static literal, view/borrowed, or materialized with backing. Its actual descriptor layout is plan-selected; **`T.SizeOf` for the string type** means descriptor bytes, while **`stringValue.SizeOf` (MD-058)** counts UTF-8 payload bytes. Physical layouts must not add `.ByteLen`/`.RuneLen` public aliases or resolve P23-D1–D3/MD-050 by assumption. Materialization/concat honors MD-004 one checked allocation. A byte pointer inside descriptor is not a C NUL-terminated string contract.

## § 23. Raw pointers and address spaces

`RawPtr[T]` is target address-space and pointer-representation dependent; neither a source integer nor universally one machine word. A capability target may require richer representation. `unsafe` does not bypass target capability or alignment constraints. Physical typed field addressing must preserve address space through operations. No implicit ref↔RawPtr conversion is introduced.

## § 24. Arena and allocation-context representation

P19 `Arena` and compiler-only `AllocationContext` may have plan-resolved descriptors/capability values, but P24 does not choose a universal arena cursor/segment/provider ABI. Resolve/attach representation requirements and lower only selected plans with actual providers. Keep ArenaDomain, validity epoch and state-version different identities. Descriptor movement must not relocate existing stable Arena allocations. Never insert hidden heap allocation as a representation strategy.

## § 25. Storage and placement

An automatic `Place` does not force stack allocation: retain SSA for non-address-observable, trivially destructible values; materialize storage only where required by borrow, address identity, mutations, cleanup, padding representation or backend target contract. A `memref` or byte buffer is valid **only** if it preserves all remaining semantics of that Place. Avoid early P5 trivial MemRef conversion for address-taken aggregates or P15 references. Static/TLS/Arena/allocator-backed storage retain distinct origins; store position is not source lifetime.

## § 26. Representation validity and initialization

Track validity separately from layout: initialized field/payload mask, bool/rune/char domain, enum discriminants, named contracts, non-null safe reference, `Result` active case, alignment and provenance. `memset(0)` is NOT a generic default constructor. For structs and arrays, initialize semantic fields and leave padding unspecified unless an FFI/security contract requires a canonical value. A physical `load` may not construct a typed Sec value from arbitrary bytes without validity proof/validation.

## § 27. Padding and information disclosure

Padding is not semantic content. Equality/hash/serialization must not read/compare it. Copies may transport padding only when representation-safe and without exposing uninitialized bytes. FFI exports/shared memory/raw byte copies require separate contract-aware leak prevention. P24 verifies and marks padding ranges; it must not automatically zero every padding byte (which could change costs/side effects) nor treat zeroing as an ownership initializer.

## § 28. Explicit layout syntax gate

Existing **MD-019** covers the still-undecided source spelling for packing, explicit alignments and offsets. Do not create `P24-D1` to duplicate it. The resolver must accept **already validated** explicit constraints through its plan API (e.g. platform/FFI metadata) and reject impossible combinations, but **must not** invent new source `@packed`, `alignas`, or offset syntax. No implementation should count parser support for explicit layout as complete until MD-019 is approved and integrated.

## § 29. Unaligned storage

For packed or otherwise unaligned accesses, a safe `ref T` requiring stricter alignment cannot be formed. Materialize target-supported unaligned **value** reads/writes or reject the operation; do not lie about alignment on a safe reference. Hardware MMIO and volatile rules may require exact access size and prohibit replacing one access with bytewise operations; those details stay in platform lowering, with physical requirements recorded now.

## § 30. Representation and ABI boundary

The resolved **storage layout** is a prerequisite to calling-convention lowering, not itself a calling convention. Do not choose sret, register pairs, argument splitting, shadow space, C struct ABI, varargs, symbol naming or foreign wrapper shapes here. Preserve high-level `func.func` semantic parameters/results plus layout refs. ABI lowering later consumes identical `ResolvedLayout` and makes distinct ABI decisions.

## § 31. Semantic IR layout facts

Introduce/reuse `ResolvedLayoutRef` and `LayoutRequirementRef`, with immutable IDs, plan linkage, completeness and proof status. No re-running general Sema from the IR builder. Attach layout identity on materialized local/storage, field/array/variant places, descriptor creation, layout-sensitive copy/move and object construction. Keep source and semantic type identity even when representation type is selected. A generic/unresolved layout may remain before materialization; executable layout-sensitive IR may not.

## § 32. P24 Semantic IR operations

Canonical operations (reuse existing equivalents if semantically identical):

```text
LayoutRequireOp               : semantic type + intended use -> LayoutRequirementRef
LayoutResolveOp               : LayoutRequirementRef + CompilationPlan -> ResolvedLayoutRef
LayoutSizeOfOp                : ResolvedLayoutRef -> uint plan-constant
LayoutAlignOfOp               : ResolvedLayoutRef -> uint plan-constant
LayoutStrideOfOp              : ResolvedLayoutRef -> uint plan-constant
LayoutFieldOffsetOp           : ResolvedLayoutRef + FieldIdentity -> uint plan-constant
RepresentationMaterializeOp   : semantic value + layout -> physical value/storage
RepresentationProjectOp       : verified layout + Place path -> representable projection
RepresentationLoadOp          : validated physical storage -> semantic value
RepresentationStoreOp         : owned/copy-resolved semantic value -> physical storage
RepresentationValidityOp      : physical candidate + validity proof/guard -> validated representation
RepresentationDischargeOp     : explicit proof outcome for an obligation
```

`LayoutResolveOp` is a **compiler plan-resolution stage artifact**, not a runtime allocator/query. Constant-size queries fold after plan selection. Prefer analysis tables instead of gratuitous executable IR ops for static facts.

## § 33. Schema v20 type and attribute design

Keep existing high-level `!sec.struct`, `!sec.array`, `!sec.union`, `!sec.ref`, `!sec.slice`, `!sec.dynamic_array`, `!sec.string`, etc. v20 adds **immutable, typed** `#sec.layout<...>` / `sec.layout_ref` and optional representation-bearing wrapper **only where backend transition demands it**. `LayoutID` is plan-scoped. Any physical-materialization marker must specify the concrete lower representation class, alignment, representation validity and outstanding obligations, not a free-form string guessed from T.

`!sec.layout_value<T, layout_ref>` is a **recommended high-level transition wrapper**, not a second Sec source type; repository implementer may use typed attrs on ops or an existing equivalent. Do not make a runtime token from LayoutID.

## § 34. Schema v20 operation families

Recommended Sec MLIR ops (avoid duplicating earlier canonical spellings):

```text
sec.layout.size_of
sec.layout.align_of
sec.layout.stride_of
sec.layout.field_offset
sec.repr.materialize
sec.repr.project_field
sec.repr.project_element
sec.repr.project_variant
sec.repr.load_validated
sec.repr.store_validated
sec.repr.check_validity
sec.repr.discharge
```

Each operation carries a typed layout ref and concrete semantic type/Place facts. `project_variant` requires P12 variant proof and an active payload; `project_element` requires P14/P16/P18 bounds proof. `load_validated` cannot bypass ownership and active variant checks.

## § 35. Representation-lowering stages

Use precise, separately testable stages:

```text
(1) resolve and verify one canonical layout service
(2) attach layout refs to Semantic IR / high-level Sec MLIR
(3) verify all necessary guards/ownership/cleanup facts
(4) materialize eligible scalar/aggregate storage representations
(5) lower verified address/field/element projections
(6) lower variant tag/payload and descriptor construction where resolved
(7) discharge only the proven or materialized obligations
(8) verify remaining Sec ops are intentionally deferred by feature gate
```

Do not put ABI or LLVM-finalization decisions into stage (4). Pure scalar/aggregate value operations may use standard MLIR Arith/MemRef/CF/Func operations where representation/effects are exactly equivalent. A later LLVM-oriented lowering can use `LLVM` dialect after ABIs are resolved.

## § 36. Concrete field address computation

From a verified base Place and `ResolvedFieldLayout` compute the field location using **the recorded offset** in the correct address space and alignment. A field may be accessed only after prove/store/borrow validity. Do not compute offset from declaration ordinal at backend time. Zero-sized fields have semantic identity even if their physical address coincides. Do not fabricate a writable pointer to a zero-length/nonaddressable object.

## § 37. Concrete fixed array addressing

Compute element byte displacement as checked `index * stride`; use P14/P15 validated bounds and P24 layout metadata. The verifier must require a dominating check on the same array/base/index SSA or a valid static proof. No `inbounds`-like low-level promise without proof. Do not dereference an element of N=0. Unaligned element access must respect type/target constraints.

## § 38. Variant materialization and tag

Select tag/payload according to resolved union/Result/Option layout and current active-variant state; emit tag representation with verified discriminant domain, initialize only active payload, leave inactive bytes unobservable. Destruction uses P17 plan and active state, not raw tag equality guessed by backend. If a niche optimization is not proven and selected by `ResolvedLayout`, retain explicit discriminant. Do not cast inactive payload bytes into another type.

## § 39. Materializing references and descriptors

If a selected `CompilationPlan` supplies a representation policy with fully resolvable runtime metadata, lower P15–P19 safe refs/slices/dynamic owning arrays/strings using exact policy fields. When policy is *not* selected or a required checker has no lower form, leave op high-level and report an implementation/capability gate—not an all-purpose pointer-length tuple. Runtime epoch comparison must use matching storage domain and expected epoch and failure must retain `panic.invalid-reference-generation`, not become `IndexError`. Borrow/provenance checks remain independent.

## § 40. P23 and unresolved string policy

P24 can resolve **physical layout** for known-valid static and materialized string values under a chosen plan; it does not resolve MD-050, P23-D1 (public offset unit), P23-D2 (materialization ownership/errors), P23-D3 (char conversion). Any physical string operation relying on these remains gated. `.Len` scalar count is semantic and value `.SizeOf` byte count is not the descriptor `SizeOf` query. Preserve MD-004 one runtime materialization; no hidden intermediate allocations while building the descriptor.

## § 41. P20–P22 cross-package synchronization

Use October plain `float` width (MD-014), the MD-012 `ConversionError` family rather than obsolete P20 `NumericConversionError`, and P21's actual approved decimal status. P22 named-type contracts require runtime validation before constructing the constrained value; physical storage allocation is not evidence of validity. If predecessor artifacts disagree, canonical October rules and mapped MD decisions win; record a targeted compatibility correction, do not silently rewrite predecessor files.

## § 42. Obligation discharge ledger

Maintain a verifier-visible per-operation obligation set with explicit outcomes:

```text
StaticallyDischarged(proof ID)
RuntimeMaterialized(check/guard ID and dominance)
TransferredToLowerLevel(lower op and verified equivalent contract)
```

Never erase: borrow/epoch validity, bounds, tag/payload active state, representation validity, initialized mask, ownership cleanup or address-space/alignment requirements merely due to lowered bits. Link discharge record to source/IR identity and chosen plan. A low-level physical load/store must not be used as a default proof.

## § 43. P24 verifiers

Register at least:

```text
--sec-verify-resolved-layouts
--sec-verify-representation-lowering
--sec-verify-layout-obligations
```

Layout verifier checks plan/type/cache identity, completeness, checked sizes/alignment, field non-overlap (where required), native order, array stride/count, tag/payload fit, descriptor policy, validity and explicit contract provenance. Representation verifier checks offsets, address space/alignment, typed load/store and no premature ABI. Obligation verifier checks no required obligation is erased without one of the three discharge proofs.

## § 44. Compiler diagnostics

Use the compiler's mentor-style diagnostics with type, plan/target, operation and exact failing path. Distinguish: invalid recursive type, impossible native/explicit layout, target unsupported representation, and compiler implementation gap. Show layout cycles with full field chain and source positions; for overflow show requested extent and target maximum. Do not report a valid-but-unimplemented descriptor policy as a malformed Sec program. Attach stable diagnostics to `sec diagnostics <ID>` when IDs are assigned by the actual compiler.

## § 45. Layout introspection and tooling

`T.SizeOf`/`AlignOf`/`StrideOf` and defined field-offset queries consume the **same** immutable `ResolvedLayout`. LSP hover/`sec analyse` must label plan/target and stability; absent plan report unresolved rather than host-guessing. Do not introduce new public methods lacking canonical compiler-known declaration. For `string`, distinguish type descriptor `SizeOf` from value UTF-8 payload `SizeOf`. Compiler and tooling results for identical plans must match byte-for-byte.

## § 46. Target matrices and negative cases

Required plan matrix: 32-/64-bit, little-/big-endian where target-supported, differing pointer alignments, richer address-space/capability target or mock policy, native and packed explicit constraints through validated metadata, Arena/static/automatic storage. Include active `int128/int256/uint128/uint256/decimal128`; plain `float` changes width while fixed `float32/float64` do not. Use mock plan implementations if actual toolchain lacks a target; do not claim target execution from mock verification.

## § 47. Scalar layout test matrix

Test type sizes/alignments/representation-validity for bool/byte/char/rune, signed/unsigned fixed widths including 128/256, pointer-sized int/uint and plain float, fixed floats, decimal and decimal128, named constrained carriers. Test illegal byte-pattern reads: invalid bool, rune surrogate, enum unknown discriminant (where invalid), invalid ref metadata. Test that all-zero default is not assumed generally.

## § 48. Aggregate layout test matrix

Struct: empty, zero-sized fields, nested, mixed alignment, declaration order, tail/interior padding, optional explicit offsets via metadata, field identity. Fixed array: N=0, N=1, huge N overflow, padded stride, zero-sized element count. Union/Result/Option: unit payloads, very different payload alignments, inactive payload protection, tag fits, niche optimization only with selected proof. Recursive: by-value cycle rejected; ref/RawPtr/dynamic backing recursion accepted where type rules permit.

## § 49. Descriptor and lifetime test matrix

For every enabled representation policy, test P15 proven ref with zero runtime generation metadata, dynamic epoch ref with correct matching guard and 64 logical epoch bits on 32-bit, P16 zero-length slice, P18 empty/move-only dynamic array, P19 Arena domain, P23 known-valid string. Test relocation/reclaim/reference conflict and ensure no runtime borrow checker, mandatory GC/RC/heap/side table is introduced. Test rich pointer/address-space targets with rejection/protection rather than truncation.

## § 50. Lowering and pass regression tests

Roundtrip v20 layout attrs/types/ops. Verify negative operations with missing/wrong `LayoutID`, wrong plan, wrong offsets, incomplete layout, non-dominating validity guard, wrong active variant, mismatched address space, unaligned safe borrow, ABI metadata applied too early, moved-source double destroy. Run P1–P14 regression, then P15–P23 installed subsets according to actual state. Record exact checks (`go test ./...`, MLIR `check-sec-mlir`, selected `mlir-opt` passes) as **not run** unless they truly ran.

## § 51. Incremental and determinism tests

Warm the cache for one plan and reuse under another plan: failure must cause recompute, never stale result. Change generic substitution, field order, explicit contract, reference descriptor policy, float width, endian and target; assert invalidation and equivalence to a clean compile. Run identical plan multiple times and demand stable IDs/order/printer output independent of map iteration. Inject malformed cache and ensure fail closed.

## § 52. Phased implementation instructions for Codex

Perform **one phase at a time and run its tests before starting the next**:

**A. Preflight/audit:** SHA, inventory, read all authoritative books, compare P23 decisions, list existing layout engines and differences, no code edits until inventory.

**B. Shared resolver:** canonical IDs, plan key, exact layout arithmetic, scalars + struct/fixed arrays, typed queries, focused unit tests.

**C. Variants/generics:** enum/union/Result/Option, recursive/generic resolution, validity constraints, deterministic caching, test matrix.

**D. Plan-driven descriptors:** reference/slice/dynamic-array/string/Arena policies; implement only policies chosen by plan; negative missing-policy tests.

**E. Semantic IR/MLIR v20:** add/refine layout refs, ops, verifier; parse-print and malformed IR tests.

**F. Representation-lowering:** materialize eligible pure storage/field/index/tag paths and discharge ledger, test exact offsets and guards.

**G. Cross-pipeline/tooling:** LSP/analysis parity, governance sync, cold/incremental, target mock matrix, regression, report all blocked dependencies.

After each phase: `git diff --check`, appropriate Go/CMake tests, inspect generated IR, and document exact evidence. Never implement another phase while the previous phase is failing without recording a root-cause exception.

## § 53. Do not over-implement

P24 explicitly excludes: final foreign/internal callable ABI, sret/register/stack argument classification, C layout compatibility without explicit contract, universal physical reference descriptor, universal `(ptr,len)` string/slice choice, physical Arena provider ABI, LLVM finalization, FFI marshalling, MMIO/volatile lowering, user source syntax for MD-019, pending P23 decisions, general LLVM optimization/pipeline closure. A valid high-level feature whose prerequisite is not executable remains a recorded gate. No speculative fallbacks.

## § 54. Acceptance criteria

```text
[ ] actual HEAD, dirty state, existing schema, P15-P23 status recorded
[ ] canonical October rules reviewed and cited in implementation report
[ ] existing layout engines identified, single service/convergence established
[ ] immutable ResolvedLayout + ResolvedFieldLayout linked to CompilationPlan
[ ] plan-complete cache key and cold fallback implemented
[ ] checked size/align/stride/offset arithmetic implemented
[ ] 32-/64-bit int/uint/plain float and all active wide types covered
[ ] decimal {i64,i32} / decimal128 {i128,i32} plan layout covered
[ ] struct declaration-order fields and padding verified
[ ] fixed arrays including zero-sized/huge count verified
[ ] enum/union/Result/Option resolved layout and validity verified
[ ] recursive/generic layout resolution and negative cycles verified
[ ] selected reference/slice/owning/string descriptor policies supported or gated
[ ] P15/P17 proof/ownership/cleanup semantics preserved
[ ] typed layout references attached to Semantic IR and Sec MLIR
[ ] schema-v20 parser/printer/verifiers implemented
[ ] physical field/array/variant representation path consumes exact layout
[ ] obligations discharged only with static/runtime/lower proof
[ ] no implicit `reinterpret` or padding comparison/leak introduced
[ ] no ABI classification performed in P24
[ ] MD-019 source syntax remains gated; P23-D1–D3 untouched
[ ] correct governance IDs updated, no duplicate fragment
[ ] compiler/LSP/sec analyse layout query parity
[ ] target matrix, cache mutation, negative/legacy tests run and recorded
[ ] `go test ./...` and `check-sec-mlir` results accurately recorded
[ ] any unimplemented dependency reported without claiming completion
```

## § 55. Required implementation report

Codex must provide: (1) HEAD/dirty state and actual precursor schemas; (2) P15–P23 implementation truth; (3) authoritative rulebook revisions and MD decision mappings; (4) files added/modified; (5) existing layout engine inventory/merges; (6) layout key/ID/canonical service; (7) scalar/float/decimal tests; (8) struct/array layout tests; (9) enum/union/Option/Result tests; (10) descriptor policy proofs/gates; (11) storage and validity facts; (12) Semantic IR changes; (13) schema-v20 ops/attrs; (14) layout/representation/obligation verifier evidence; (15) representation-lowering output; (16) LSP/tooling parity; (17) cache separation and malformed-cache tests; (18) Go/MLIR/CMake commands and exact outputs; (19) expected failures/not-run; (20) governance changes; (21) unresolved MD-019/P23 references; (22) package 25 recommendations.

Do **not** claim all of P24 complete if a required selected descriptor policy lacks a lower form; accurately report phase/subset completion.

## § 56. Next-package boundary

P25 candidate: **Callable ABI Classification and Foreign Representation Boundary**. It consumes P24 `ResolvedLayout` to classify arguments/returns, target calling conventions, sret, hidden compiler arguments, by-value/by-reference ABI rules, C/foreign contract compatibility, platform ABI requirements and wrapper planning. It must still keep source ownership/borrowing/call semantics fixed by earlier packages. Alternative ordering may prioritize final physical reference/descriptor lowering once actual P15–P23 implementation is ready; do not assume calendar plans equal implemented code.
