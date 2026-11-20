# MD-141 + MD-142 — Canonical Scalar and Native Aggregate Layout

| Field | Value |
|---|---|
| Document | Joint normative correction for MD-141 and MD-142 |
| Date | 2026-11-20 |
| Revision | 1 |
| Language | Sec 0.1 |
| Status | **Specification ready for normative integration; not yet merged or implementation-verified** |
| Repository | `YoePro/sec`, `main` at `d680ce820748224e3527a63699aefa4fd34250e0` |
| Owning rulebooks | `rules/memory/layout.md`, `rules/types/types.md`, `rules/platform/target_profiles.md`, `rules/platform/platform_model.md` |
| Consumers | `rules/mlir/packages/sec-mlir-dialect_package24.md`; P18 and P19; compiler, LSP and `CompilationPlan` |
| Governance owners | `frontend.layout`, `lowering.sec-mlir-package24`; target data-layout producer and project target selection |

## § 1. Purpose, authority and limits

**§ 1(1)** This correction completes the missing normative rules for (a) storage size and alignment of Sec scalar types and (b) ordinary native layout of structs, fixed arrays and the already-defined composite decimal representations. It does not define callable ABI classification, C-compatible struct layout, registers/MMIO, allocator-provider selection, an unrelated numeric error policy or new source layout modifiers.

**§ 1(2)** Existing decisions remain authoritative. In particular, `layout.md` §§ 1(10), 3, 5, 7–9, 11, 18–19, 24 and 27–29, 38–40; `types.md` §§ 9–12; MD-014; and MD-019 must be preserved. This document supplies missing deterministic rules, not replacement language semantics.

**§ 1(3)** Four different concepts must remain distinct: (i) source semantic type and valid value domain; (ii) materialized storage size; (iii) minimum required alignment; and (iv) preferred or incidental physical placement/alignment chosen by an optimizer. Equal storage size never proves equal ABI alignment. A backend's internal SSA/register width is not the storage contract of an addressable value.

**§ 1(4)** `CompilationPlan` is the single authority for target-resolved layout. The compiler's canonical Sec layout service computes `ResolvedLayout` under that plan. LLVM/MLIR target data-layout queries are permitted inputs to the plan producer, but their independently recomputed answers must not replace a resolved Sec layout during lowering (`layout.md` § 39(4)).

**§ 1(5)** No existing numbered paragraphs are renumbered. On integration, append new uniquely numbered paragraphs to the owning sections, or insert clearly labelled subsections with their own new numbers. Source-code rule references must remain valid.

## § 2. MD-141 — Canonical scalar storage representations

**§ 2(1) [OWNER-APPROVED 2026-11-20]** A separately addressable `bool` occupies **1 byte**. Its valid stored representation is exactly `0x00` for `false` and `0x01` for `true`. Other byte values are invalid *stored* `bool` representations. The already-defined explicit conversion `bool(value)` (zero→false, nonzero→true) is not an implicit validity rule for raw memory.

**§ 2(2) [OWNER-APPROVED 2026-11-20]** A separately addressable `rune` occupies **4 bytes**, containing a 32-bit unsigned Unicode code-point number in the target's selected byte order. Its already-decided value domain is `U+0000..U+10FFFF` excluding `U+D800..U+DFFF`; every other 32-bit bit pattern is invalid for a safe typed `rune` value. `rune` is not UTF-8-encoded storage and is not an unrestricted `uint32`.

**§ 2(3) [PRE-EXISTING NORMATIVE]** `byte` and `char` each occupy 1 byte with all 256 bit patterns valid. They remain distinct semantic types. Fixed signed and unsigned integer types occupy their declared width divided by 8 bytes: 1, 2, 4, 8, 16 or 32 for 8, 16, 32, 64, 128 or 256 bits. Every bit pattern is a valid integer representation; endianness determines byte ordering where observable.

**§ 2(4) [PRE-EXISTING NORMATIVE]** Plain `int` and `uint` use the plan-selected 32- or 64-bit width, occupying 4 or 8 bytes respectively. `float32` and `float64` occupy 4 and 8 bytes respectively; plain `float` uses the same width selector as `int`/`uint` (MD-014). No independent target-sized float-width setting is introduced.

**§ 2(5) [PRE-EXISTING NORMATIVE]** The canonical storage representation of `decimal` is the ordered fields `{ i64 coefficient, i32 scale }`, and of `decimal128` is `{ i128 coefficient, i32 scale }` (`layout.md` § 19; `types.md` § 12.1). Their overall layout is **not** a raw sum of 8+4 or 16+4 bytes; § 7 provides the deterministic field placement, padding and alignment rule.

**§ 2(6)** A scalar's valid value domain remains independent of its physical width and alignment. A physical load or materialization may not create an invalid `bool`, `rune`, named scalar or enum. Validation, proof and source-specific safety requirements are owned by existing conversion and validity rules, not by target alignment metadata.

**§ 2(7)** Addressable native scalar storage is not silently bit-packed or compressed. A separately specified bit-field/register, explicit packed field access or optimized non-addressable SSA value may have a different representation **only** when that owning rule permits it and it does not change an observable native `T.SizeOf`/`T.AlignOf` or allow an invalid safe reference.

## § 3. MD-141 — Target-derived minimum scalar alignment

**§ 3(1) [PRE-EXISTING NORMATIVE]** Native scalar alignment is selected by the target/profile and `CompilationPlan`, as already required by `layout.md` §§ 1(10), 3 and 8. It must not be deduced from scalar byte width alone or from the compiler host. The minimum alignment observable as `T.AlignOf` is the *required* placement alignment, not the target's optional preferred optimization alignment.

**§ 3(2)** The target-data-layout producer must resolve *at least* a scalar ABI-storage-alignment fact for each relevant representation class and width, subject to the selected target, target ABI/layout identity and applicable address-space storage policy. Its scalar representation identities include:

```text
IntegerStorage(8, 16, 32, 64, 128, 256)
BinaryFloatStorage(32, 64)
```

`bool`, `byte` and `char` use the 8-bit integer *storage representation* alignment; `rune` uses the 32-bit integer storage representation alignment. Signed and unsigned integer types of equal width share a storage representation unless the selected target positively defines a different canonical representation requirement. This does **not** merge the Sec types.

**§ 3(3)** `int`, `uint` and plain `float` resolve to the corresponding width- and class-specific target facts. `decimal`/`decimal128` resolve their component integer facts first; their aggregate alignment is computed under § 7. Named scalar types that share a carrier's representation use its physical layout while preserving nominal identity and constraints.

**§ 3(4)** The canonical fact producer obtains its values from the **selected target's verified data layout / ABI description** (for example, an LLVM TargetMachine data-layout query for the exact selected triple/CPU/feature set when that representation is faithfully supported), or from an explicit compiler-maintained target-specific layout description whose provenance and compatibility are validated. It must not derive facts from the compiler host's `sizeof`, `alignof`, architecture-name heuristics, pointer width alone, or assumptions that every N-byte scalar aligns to N bytes.

**§ 3(5)** A backend-produced data-layout string or table is *evidence*, not a second source of Sec type semantics. The plan producer normalizes the representation-specific ABI alignments into a typed, immutable resolved fact set and checks them against the representation it will actually emit. Data-layout fields that have no corresponding supported representation must not be accepted silently; conversely, absence of an explicit layout-string entry is not permission to guess: the producer must use that toolchain's documented, deterministic default-resolution algorithm **and verify its result**, or declare that fact unresolved.

**§ 3(6)** All required alignment values are positive powers of two in bytes and must be positively supported for the chosen representation and address space. A target that needs a non-power-of-two placement model or an unsupported scalar representation requires a separately specified target rule; it may not be squeezed into the ordinary power-of-two layout table. An over-alignment capability is a *separate* target fact, not an inferred consequence of native scalar alignment.

**§ 3(7)** A target definition may support parsing/type-checking while leaving some physical layouts incomplete, but it must not claim executable or fully supported layout-sensitive lowering for those facts. Missing target facts produce an incomplete `ResolvedLayout` with the target and missing representation identified. A target known not to support the representation produces an unsupported layout. Contradictory facts are invalid; no fallback table is silently substituted.

**§ 3(8)** The canonical plan must retain the selected target layout identity, ABI identity, scalar alignment facts, address-space distinction where relevant, any native aggregate constraints from § 5 and the compiler layout revision in the layout cache/fingerprint. Two plans with different alignment-relevant facts cannot share a completed cached layout. `LayoutID` remains plan-scoped.

**§ 3(9)** An existing `ScalarAlignments map[uint64]uint64`, keyed only by **size in bytes**, is not a normative representation of these facts because integer and binary-float storage classes with equal size need not have equal required alignment. Replace or extend it with a typed class+width+applicable storage-context lookup; serialize and fingerprint the *normalized typed facts* deterministically. Never introduce an independent fallback scalar layout engine for Sema, LSP or lowering.

## § 4. MD-141 — Coverage for registered targets

**§ 4(1)** Every concrete target advertised as capable of materializing a source scalar type must resolve the applicable alignment facts under its exact selected target layout. The target registry currently includes amd64/x86-64, ARM64, ARMv6/v7, RISC-V 32/64, WebAssembly 32/64, Cortex-M variants and other target/profile entries. Each registered target is checked independently; matching pointer widths do not authorize sharing unverified alignments.

**§ 4(2)** The producer's conformance matrix must cover integer storage 8/16/32/64/128/256, binary32/binary64, `bool`, `rune`, target-sized types and both decimal component families. For targets that cannot lower a particular type, record **unsupported** rather than choosing an invented alignment. An explicit registry status `Planned` or `Experimental` does not authorize an arbitrary concrete alignment.

**§ 4(3)** For each registered triple/profile combination, tests must record which target-data-layout provider/version or maintained target description yielded its values, and compare the normalized resolved facts against the actual backend for all representations claimed supported. This is a *machine-checkable per-target value contract*, not a single cross-target table baked into the language specification.

**§ 4(4)** An alternative verified backend or toolchain version may supply different target facts. Compatibility must be plan-sensitive and fail closed when the concrete layout is not established. A compilation targeting a CPU or ABI variant must not silently inherit layout data from a different variant merely because the architecture spelling matches.

## § 5. MD-142 — Ordinary native aggregate alignment

**§ 5(1)** Sec native structs and arrays use Sec's canonical layout rules. Their similarity to common C/LLVM natural layouts does **not** prove C ABI layout compatibility (`layout.md` § 36). Callable passing conventions and foreign aggregate rules are outside MD-142.

**§ 5(2)** A plan may define *verified target constraints applicable to native Sec aggregate storage* (such as a required minimum alignment, a maximum supported alignment, or a necessary address-space restriction). Such constraints are typed and plan-resolved. The default ordinary native aggregate alignment is derived from the required alignment of its constituents, **not** from an unrelated C struct-layout assumption.

**§ 5(3)** For an ordinary unmodified nonempty struct, let `Afield[i]` be each field's **effective placement alignment**, obtained from the field's resolved type alignment and any applicable, already-validated `pack(N)`, `minAlign(N)` or target rule (§ 8). Its natural aggregate alignment is `max(1, Afield[0], ..., Afield[n−1])`. A verified target-specific *minimum* for native aggregates or a type-level `minAlign` may raise that result; a target-specific maximum is a capability check and may **not** silently lower the field-required alignment. A conflicting requirement makes the layout invalid/unsupported.

**§ 5(4)** An unmodified fixed array has the **element's resolved native alignment** as its natural alignment. A positively established target-native-array minimum may raise it; a maximum or unsupported alignment fails closed rather than clamping. An array's alignment is not inferred from total byte size or element count.

**§ 5(5)** An empty struct with no stored fields has **size 0 and alignment 1** (`layout.md` § 24(6), already decided). A zero-sized field does not disappear from semantic field identity. For a nonempty struct whose members happen to be zero-sized, the ordinary placement algorithm still applies to each field, including any nontrivial explicit minimum alignment, without treating that struct as equivalent to the *fieldless* struct rule.

**§ 5(6)** No implicit target-default aggregate over-alignment is introduced merely for optimization. An optimizer may choose stricter incidental physical placement when it does not change the type's canonical native layout or observable layout queries. The backend may not use a changed padding/offset/stride as if it were an optimized equivalent of a `ResolvedLayout` with different facts.

## § 6. MD-142 — Canonical stored-field placement algorithm

**§ 6(1)** For a concrete, acyclic ordinary struct with stored fields in source declaration order, initialize a cursor `C=0` and a required aggregate alignment `A=1`. Resolve every field's complete materialized layout first. For field `i`, determine its effective placement alignment `Ai` under § 8. Then compute:

```text
Offset[i] = AlignUp(C, Ai)
C         = CheckedAdd(Offset[i], SizeOf(Field[i]))
A         = max(A, Ai)
```

The field's semantic identity, source order, offset and required alignment remain recorded separately. All arithmetic is exact and checked under the selected plan.

**§ 6(2)** After all fields, apply any positively supported target-native-aggregate minimum and type-level explicit `minAlign` to obtain the final `A`. Require that this alignment is legal for the target/representation. The final storage size is `AlignUp(C, A)`. The bytes from a previous field end to the next field start are interior padding; bytes from the last field extent through `SizeOf` are tail padding. Padding is not automatically initialized or semantic data.

**§ 6(3)** If a field has size zero, the cursor may advance solely to honor required placement alignment but its payload occupies no bytes. Multiple zero-sized fields may share an offset when the canonical placement rules yield that result; they retain separate field identities and may not be treated as a proof of overlapping addressable storage or interchangeable safe references (`layout.md` § 24).

**§ 6(4)** Explicit `byteOffset(N)` overrides the automatically computed offset of that field, subject to the pre-existing MD-019 rules: exact offset, effective alignment, no overlap of nonzero-sized stored regions, representation bounds and target capability. It cannot be used to skip validation. The cursor for subsequent fields is the maximum byte extent of fields placed so far; a zero-sized explicitly placed field may raise that extent as a placement boundary.

**§ 6(5)** Native structs preserve stored field declaration order, not alphabetical order or backend convenience order. Properties, methods, static members and nonstored declarations contribute no instance fields. A `ResolvedLayout` must describe exactly the representation consumed by lowering.

**§ 6(6)** `AlignUp(X,A)` is defined only for positive, supported `A` and nonnegative exact `X`, with `X + ((A − (X mod A)) mod A)` using checked arithmetic. All offsets, alignment-adjusted sizes and strides obey the existing maximum byte extent `2^(W−1) − 1` for plan pointer width `W` (`layout.md` § 7(7)). There is no host-width overflow or modulo wraparound.

## § 7. MD-142 — Fixed arrays and decimal composite scalars

**§ 7(1)** For materialized `T[N]`, `N` is the already-validated fixed count and `T` has one complete resolved element layout. The **element stride** is `AlignUp(SizeOf(T), AlignOf(T))` when `SizeOf(T) > 0`, and **zero** when `SizeOf(T) == 0`. `N == 0` yields an array of size zero. The array's required alignment is selected under § 5(4), and its total size is `AlignUp(CheckedMul(N, StrideOf(T)), AlignOf(T[N]))` for `N > 0`; it is zero for `N == 0`. Under the ordinary native rule with no extra target array alignment, this is simply `N × StrideOf(T)`.

**§ 7(2)** A zero-sized array element still represents an element at each logical index. `StrideOf(T)==0` does not authorize unchecked pointer arithmetic, address-identity claims, mutable aliasing or dereferencing of an out-of-range index. Existing reference, indexing and zero-sized-type rules remain authoritative (`layout.md` §§ 9, 24).

**§ 7(3)** Arrays have no hidden length, capacity, pointer or heap header in their fixed inline materialized layout. Size/stride overflow or unsupported alignment fails compilation when the layout is required. Runtime dynamic arrays and slices are different descriptor/ownership models and are not redefined here.

**§ 7(4)** Resolve `decimal` and `decimal128` using the **same ordinary native stored-field algorithm** as a two-field struct with fields in this fixed semantic order:

```text
decimal:    coefficient: i64;  scale: i32
decimal128: coefficient: i128; scale: i32
```

Their coefficient and scale retain existing semantic types and widths. The offset of `scale`, interior/tail padding, total size and alignment come from target-resolved `i64`/`i128`/`i32` storage alignments, not from a hardcoded 12-byte or 20-byte packed record. No source-level change to the decimal API or value semantics is introduced.

**§ 7(5)** As an **illustrative mock target only**, let `AlignOf(i32)=4`, `AlignOf(i64)=8`, `AlignOf(i128)=16`. The algorithm yields `decimal` offsets `{0,8}`, alignment 8 and size 16; `decimal128` offsets `{0,16}`, alignment 16 and size 32. These examples are not evidence that every x86, ARM, RISC-V or WebAssembly target has those alignments.

**§ 7(6)** These composite decimal layouts remain scalar **semantic** types. Their internal ordered physical fields are a layout implementation fact, not an invitation to treat `decimal` or `decimal128` as publicly declared Sec structs, to expose their padding, or to claim ordinary C ABI equivalence.

## § 8. MD-019 interaction and precedence — no new source syntax

**§ 8(1) [PRE-EXISTING]** The source modifiers are `pack(N)`, `minAlign(N)` and `byteOffset(N)`, as already defined by MD-019 and `layout.md` §§ 27–29. This correction adds no alternate attribute and no implicit repr(C).

**§ 8(2)** For each stored struct field, determine the effective placement alignment by: (a) resolving the **field type's own** native alignment and nested type-level minimum; (b) applying the enclosing struct's `pack(N)` cap to the *ordinary natural* part; (c) preserving any independently declared field `minAlign` and nested type-level `minAlign`; (d) applying verified target requirements that cannot be weakened. At no stage may a cap reduce an explicit or target-mandated minimum. The `pack` cap affects placement in its enclosing struct; it does not rewrite the nested type's internal offsets or type identity.

**§ 8(3)** A type-level `minAlign` raises the enclosing type's own alignment and therefore can add tail padding. A field-level `minAlign` raises that field's effective placement alignment and can affect the containing struct's alignment and offsets. Each remains independently source-attributed for diagnostics and IR verification.

**§ 8(4)** A field placed at insufficient alignment may not form a safe `ref T` requiring stronger alignment; where permitted, value-form unaligned operations are handled under the pre-existing packed/unaligned rules. This correction does not declare every unaligned access safe or force unsafe pointer operations to succeed.

**§ 8(5)** Explicit layout conflicts fail with a diagnostic identifying the exact source contract, required alignment, requested placement or bound, and incompatible verified target fact. Neither packing nor explicit offsets can silently override target-imposed legality or change scalar representation validity.

## § 9. Canonical resolved facts and lowering contract

**§ 9(1)** Target resolution produces one immutable normalized native-layout fact set. It must contain target layout identity/provenance, ABI/representation class alignments (including wide integers), applicable aggregate and address-space constraints, and the already-defined width/endian/profile/reference facts. `ResolvedLayout` contains type identity, plan key, completeness, exact size and alignment, field offsets, stride/element/variants where applicable, representation validity, explicit contract provenance and diagnostics.

**§ 9(2)** Sema, Semantic IR, LSP and MLIR lowering consume **the same** canonical layout service or its verified immutable results. They must not compute separate host-derived sizes, assume C struct layout, perform an independent default-width selection, or replace a missing fact with an LLVM backend guess. A target fact changing the answer invalidates layout cache reuse.

**§ 9(3)** Layout resolution may produce `complete`, `incomplete`, `invalid`, or `unsupported` as already modeled. Only `complete` results may satisfy layout-sensitive emission or materialization. For an unresolved scalar, every dependent struct/array/decimal layout stays incomplete with the underlying missing fact traceable; a failure is not converted to zero size or default alignment.

**§ 9(4)** Physical lowering must preserve all recorded field offsets, alignments, array strides, representation and semantic validity constraints. `SizeOf`/`AlignOf`/`StrideOf`/field-offset queries must agree with the emitted representation. Backends must reject a type they cannot materialize accordingly rather than silently selecting an incompatible ABI/default representation.

**§ 9(5)** `LayoutRevision` and normalized target data-layout facts participate in the layout-cache key. Any modification of the layout algorithm, scalar alignment classes, array rules or aggregate constraints that can change results must update the compatibility revision and invalidate/recompute stale cache entries.

## § 10. Diagnostics and mandatory conformance tests

**§ 10(1)** A diagnostic for missing layout facts names: affected semantic type; target/triple/profile/ABI when available; required representation class/width; missing/contradictory fact; why the operation needs a complete layout; and the responsible target/layout configuration or implementation step. Use existing compiler diagnostic taxonomy; this correction does not allocate a new diagnostic identifier without governance.

**§ 10(2)** Test approved scalar values and sizes: `bool` 1 byte with valid `0`/`1` encodings only; `rune` 4 bytes with valid Unicode scalars only; `byte`/`char` 1 byte; every fixed integer width through 256; `int`/`uint` and `float` on 32-bit and 64-bit plans; `float32`/`float64`; named constrained carriers. Invalid stored bool/rune patterns cannot be treated as safe values.

**§ 10(3)** Test target alignment *by representation class*, not only by byte size. Use synthetic verified plans in which a 4-byte binary float and a 4-byte integer intentionally have different legal ABI alignments; both must remain distinct in the cache key and resolved layout. Test 16- and 32-byte integer storage on targets where alignment differs from size. Separate target-integration tests must verify actual facts against the selected toolchain; mock plans are not counted as real-target evidence.

**§ 10(4)** Test struct placement in declaration order, mixed scalar widths, trailing padding, empty struct (size 0, align 1), zero-sized fields, all-zero-sized-field structs, explicit field minAlign, type minAlign, pack interaction, exact byteOffset, nested aggregates, conflicting contracts and target limits. Assert offset and padding ranges as well as size and alignment.

**§ 10(5)** Test `T[0]`, `T[1]`, `T[3]`, zero-sized `T`, embedded arrays, array alignment greater than element alignment in a verified synthetic plan, huge `N` overflow and stride/size upper-bound checks. Logical indexing identity remains correct where physical stride is zero.

**§ 10(6)** Test decimal internal coefficient/scale order and offsets, including plans with distinct `i64`/`i128` alignments; `decimal` and `decimal128` tail padding; no fixed packed 12/20 byte assumption; consistent conversion between resolved layout and emitted fields.

**§ 10(7)** For every registered target/triple/profile that claims the supported operation, verify all required scalar class+width alignments and aggregate constraints from a canonical target data-layout provider or validated maintained data; unsupported combinations must fail closed. Include registered 32-bit/64-bit and bare-metal/WebAssembly cases without claiming execution tests for planned targets.

**§ 10(8)** Cache regression: query the same semantic type across different targets, ABIs, address spaces, scalar class alignments, aggregate constraints and compiler layout revisions. A differing plan forces recomputation. Cold and cached resolved results must agree. Assert that absent/malformed target facts never silently reuse results from a previous plan.

**§ 10(9)** Do not mark compilation tests as passed until actually executed. Governance closure requires normative integration, target value coverage, unit tests, compiler/LSP path tests, P24 lowering checks, and affected P18/P19 conformance paths to the extent supported by their implementation packages.

## § 11. Required source and governance integration

**§ 11(1)** `rules/types/types.md`: append scalar materialized-storage statements without altering § 9.1 `bool` conversion semantics, § 9.4 `rune` Unicode validity, target-sized rules or fixed integer widths.

**§ 11(2)** `rules/memory/layout.md`: append uniquely numbered text in §§ 8, 9, 11, 18, 19 and 24 or equivalent new dedicated subsections, incorporating §§ 2–8 above. Keep existing § 7(7), § 24(6) and MD-019 §§ 27–29 intact. No existing numbered paragraph may be renumbered.

**§ 11(3)** `rules/platform/target_profiles.md` and the target/CompilationPlan owning model: specify the typed and independently validated source of scalar class+width required ABI alignments, native aggregate constraints, exact target data-layout provenance, completeness and failure modes. A plan may be analysis-only if a concrete physical layout cannot be resolved.

**§ 11(4)** `rules/mlir/packages/sec-mlir-dialect_package24.md`: reconcile §§ 9–13 and 47–50 with the resolved algorithms, no second layout authority and conformance gates. P18 and P19 consume the same result without inventing array/arena memory alignment.

**§ 11(5)** `internal/layout/resolved_layout.go`: migrate size-only `ScalarAlignments` into representation-keyed target facts and canonical deterministic fingerprinting. `internal/layout/scalar_plan.go`, `internal/platform/target/target.go` and target-plan construction supply resolved values and provenance. `internal/layout/resolver.go` implements scalar sizes/alignments, struct placement, fixed arrays and decimal carriers with shared checked arithmetic. Avoid duplicate layout calculators in other compiler phases.

**§ 11(6)** Once the correction is integrated into owning rulebooks, change `missing-decisions.yaml` statuses of MD-141 and MD-142 from `open` to **`decided-implementation-pending`**, retaining the IDs and implementation-gate descriptions. Do **not** delete entries before governance and implementation/test closure required by that file's purpose. Synchronize `governance/memory.yaml` (`frontend.layout`), the P24 owner fragment and the target-plan owner; retain their existing IDs and status semantics.

## § 12. Decision provenance and closure ledger

| Item | Source of authority | State | Closure criteria |
|---|---|---|---|
| MD-141: storage representation of `bool`/`rune` | Owner approved 2026-11-20; pre-existing semantic domain in `types.md` | **Approved** | Rulebook integration and compiler/tests |
| MD-141: target/plan owns alignment; alignment ≠ size | Already normative in `layout.md` §§ 1, 3, 8, 39; reaffirmed by owner | **Already approved; not reopened** | Typed plan facts, target coverage, tests |
| MD-141: concrete class+width plan facts and target verification | Direct completion of existing alignment authority | **Normative correction proposed here** | Resolve facts for claimed supported targets; negative unknown tests |
| MD-142: empty struct size 0/align 1 | `layout.md` § 24(6), owner decision 2026-11-19 | **Already approved** | Retain and test |
| MD-142: maximum byte extent `2^(W−1)−1` | `layout.md` § 7(7), owner decision 2026-11-19 | **Already approved** | Checked overflow tests |
| MD-142: ordinary struct/array alignment, placement and stride | Open MD-142; derived proposal in §§ 5–8 | **Normative correction proposed here; not pre-approved** | Integrate rule and verify mock/real plans |
| MD-142: decimal composite physical layout | Existing `{ i64,i32 }` / `{ i128,i32 }` plus proposed ordinary aggregate algorithm | **Existing representation; placement specified here** | Offset/size/padding tests and P24-17 |

**§ 12(1)** This document is the **proposed joint normative fix**. It records already-approved decisions distinctly from the remaining precise, implementable layout rules. It does not represent a GitHub commit, a successful implementation, or completion of the target-specific conformance matrix.

**§ 12(2)** After normative integration, MD-141 and MD-142 should no longer require another open design discussion. Their implementation remains tracked by the existing governance owners. Any contradiction discovered during integration must be reported against the exact existing rule, not silently settled through a backend heuristic.

## § 13. Source inventory (read at repository baseline)

- `missing-decisions.yaml`, MD-141 and MD-142.
- `rules/memory/layout.md` §§ 1, 3–9, 11, 18–19, 24, 27–29, 38–40.
- `rules/types/types.md` §§ 9–12.
- `rules/platform/target_profiles.md`; `rules/platform/platform_model.md`.
- `rules/mlir/packages/sec-mlir-dialect_package24.md` §§ 9–13, 47–50.
- `internal/layout/scalar_plan.go`, `internal/layout/resolved_layout.go`, `internal/layout/resolver.go`, `internal/platform/target/target.go`.

No source files in the repository have been modified by generating this correction document.

## § 14. Registered-target coverage checklist (repository baseline)

**§ 14(1)** At the inspected repository baseline, the target registry contains the following distinct registrations. This inventory is **not** a claim that required scalar alignment facts are already available. Implementers must record the resolved target-specific ABI alignment for every required scalar representation class/width and verify it with the selected target toolchain before marking the registration's physical layout complete. Entries with no concrete target triple cannot resolve a concrete layout merely from their operating-system/profile family.

| Target OS | Architecture | Canonical triple / identity | Registry state | Required action |
|---|---|---|---|---|
| linux | amd64 | `x86_64-pc-linux-gnu` | Implemented | Resolve and verify all supported scalar alignments |
| linux | arm64 | `aarch64-unknown-linux-gnu` | Experimental | Resolve and verify before layout-sensitive emission |
| linux | armv6 | `armv6-unknown-linux-gnueabihf` | Experimental | Resolve and verify before layout-sensitive emission |
| linux | armv7 | `armv7-unknown-linux-gnueabihf` | Experimental | Resolve and verify before layout-sensitive emission |
| linux | riscv64 | `riscv64-unknown-linux-gnu` | Planned | Record supported facts or remain unresolved |
| macos | amd64 | `x86_64-apple-darwin` | Experimental | Resolve and verify before layout-sensitive emission |
| macos | arm64 | `aarch64-apple-darwin` | Experimental | Resolve and verify before layout-sensitive emission |
| windows | amd64 | `x86_64-pc-windows-msvc` | Planned | Record supported facts or remain unresolved |
| windows | arm64 | `aarch64-pc-windows-msvc` | Planned | Record supported facts or remain unresolved |
| freebsd | amd64 | `x86_64-unknown-freebsd` | Planned | Record supported facts or remain unresolved |
| freebsd | arm64 | `aarch64-unknown-freebsd` | Planned | Record supported facts or remain unresolved |
| freebsd | armv7 | `armv7-unknown-freebsd-gnueabihf` | Planned | Record supported facts or remain unresolved |
| openbsd | amd64 | `x86_64-unknown-openbsd` | Planned | Record supported facts or remain unresolved |
| openbsd | arm64 | `aarch64-unknown-openbsd` | Planned | Record supported facts or remain unresolved |
| netbsd | amd64 | `x86_64-unknown-netbsd` | Planned | Record supported facts or remain unresolved |
| netbsd | arm64 | `aarch64-unknown-netbsd` | Planned | Record supported facts or remain unresolved |
| unknown | wasm32 | `wasm32-unknown-unknown` | Planned | Record supported facts or remain unresolved; MD-075 independent |
| unknown | wasm64 | `wasm64-unknown-unknown` | Planned | Record supported facts or remain unresolved; MD-075 independent |
| baremetal | cortex-m0 | Target/ABI-specific AAPCS identity | Planned | Resolve concrete toolchain target and verify |
| baremetal | cortex-m3 | Target/ABI-specific AAPCS identity | Planned | Resolve concrete toolchain target and verify |
| baremetal | cortex-m4 | Target/ABI-specific AAPCS identity | Planned | Resolve concrete toolchain target and verify |
| baremetal | cortex-m7 | Target/ABI-specific AAPCS identity | Planned | Resolve concrete toolchain target and verify |
| baremetal | riscv32 | Target/ABI-specific ILP32 identity | Planned | Resolve concrete toolchain target and verify |
| freertos | cortex-m4 | Target/ABI-specific AAPCS identity | Planned | Resolve concrete toolchain target and verify |
| freertos | cortex-m7 | Target/ABI-specific AAPCS identity | Planned | Resolve concrete toolchain target and verify |
| rtems | any | No concrete triple / target selected | Planned | Select concrete architecture/ABI before layout resolution |
| zephyr | any | No concrete triple / target selected | Planned | Select concrete architecture/ABI before layout resolution |

**§ 14(2)** A 27-row target registry checklist is deliberately independent of the numeric alignment tables: the latter must be obtained from **the exact selected ABI/data layout** for every supported representation. A static table of guessed values for these entries would violate `layout.md` § 3(4) and § 5(5).

**§ 14(3)** The target producer should emit, for each registered materializable target/plan and scalar representation, a machine-checkable record of `{TargetPlanIdentity, RepresentationClass, WidthBits, RequiredAlignBytes, ProviderIdentity, ValidationStatus}`. `ValidationStatus` must distinguish actual verified facts from mock data, unavailable backend, planned support and unsupported representation. Record both successful and rejected plan-resolution paths in tests.
