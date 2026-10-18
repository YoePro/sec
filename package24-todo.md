# SEC-MLIR Package 24 — implementation TODO

Package: `SEC-MLIR-P24` — Physical Value Representation and Layout Lowering  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package24.md`  
Package contract: `rules/mlir/packages/sec-mlir-dialect_package24.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_layout_sync_package24.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_layout_package24.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v20.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v20.md`
- `rules/mlir/packages/sec-mlir-dialect_package24_decision-index.md`
- `rules/memory/layout.md`, `rules/memory/storage.md` and
  `rules/memory/reference_model.md`

This is an implementation plan, not a completion report and not an ABI design.
P24 builds one canonical, target-resolved storage-layout service and carries its
facts through Semantic IR and the single Sec MLIR lowering track. Physical
value layout is not calling-convention classification, ownership, a universal
descriptor or permission for another backend.

## Inventory snapshot — 2026-10-18

Audited HEAD:

```text
831f6e1
```

The working tree contained 144 changed entries when this package was audited.
Those intended predecessor changes, including Package 20–23 TODO documents,
must be captured before P24 acceptance can be reproduced.

| Area | Current implementation | P24 consequence |
|---|---|---|
| Target facts | `layout.ResolvedScalarPlan` provides target OS/arch, pointer width, endian, C ABI, reference and atomic facts; platform targets construct these facts. | Reuse it as a starting input, then define the complete plan key required for storage layout. |
| Source scalar semantics | Sema resolves target-sized `int`/`uint` and platform-width plain `float`; fixed widths remain distinct. | Preserve this source truth and extend it with physical size/alignment facts rather than re-inferring host widths. |
| Existing layout analysis | Sema detects by-value struct/union/wrapper/generic recursion and preserves deterministic witness paths. | Retain this validation as an input to concrete layout resolution; it is not a `ResolvedLayout` calculator. |
| Existing physical lowering | Current Sec MLIR supports target scalar resolution (including plain `float` by plan width), ordinary integer enums lowered to their carrier integer, statics as LLVM globals and P15 high-level places/references; installed rulebooks remain schema/lowering v11. | There are no P24 typed layout refs, schema-v20 operations, layout verifiers or general representation-lowering pass. |
| Compiler-known facts | `SizeOf`, `Len`, `Bits` and related surfaces are typed in Sema; string value byte size is distinct from its scalar length. | Route actual target-resolved values through the canonical service and shared tooling rather than hardcoded helpers. |
| Governance | `frontend.layout`, `lowering.layout` and `platform.layout` are partial; Semantic IR/tooling layout are planned. | Update these existing IDs with real evidence only; do not create a parallel namespace. |

Primary implementation anchors:

```text
internal/layout/scalar_plan.go
internal/layout/reference_policy.go
internal/platform/target/target.go
internal/codegen/targetplan/plan.go
internal/sema/analyzer_target_plan.go
internal/sema/analyzer_struct_layout.go
internal/sema/generic_layout_paths.go
internal/sema/compiler_known_members.go
internal/ir/semantic/builder.go
internal/ir/semantic/model.go
internal/lowering/secmlir/emitter.go
internal/lowering/secmlir/target_scalar_parity_test.go
governance/memory.yaml
```

## Authority and dependency constraints

1. **One canonical service:** Sema, Semantic IR, MLIR, lowering, LSP and
   `sec analyse` consume one immutable `ResolvedLayout` keyed by concrete type,
   complete compilation-plan identity and representation contract. No layer may
   recompute offsets/sizes from Go-host data or create a second layout engine.
2. **Layout is not ABI:** P24 determines storage representation only. It must
   not choose sret, register/stack classification, argument splitting, C
   compatibility, symbol shape, foreign wrappers or final LLVM ABI lowering.
3. **MD-014 is settled:** plain `float` is binary32 for 32-bit plans and
   binary64 for 64-bit plans. Fixed `float32`/`float64` keep their widths; old
   P20 binary64-always wording is compatibility debt, not authority.
4. **Canonical decimal carrier layout is fixed:** `decimal` is `{i64,i32}` and
   `decimal128` is `{i128,i32}`; offsets/alignment/padding remain plan facts.
   P21's unresolved precision/scale/API work can gate dependent behavior but
   cannot silently alter those physical carriers.
5. **Representation validity is independent of layout:** aligned bytes are not
   automatically a valid bool/rune/enum/named value/reference/active variant.
   Padding is not semantic data, and zeroing is not a generic constructor.
6. **MD-019 remains open:** P24 may accept already validated layout constraints
   from a plan/FFI metadata source, but cannot invent source syntax for packing,
   alignment or explicit field offsets.
7. **Reconcile stale P23 references before implementation:** current authority
   resolves MD-050, MD-071 and MD-072 for valid UTF-8, public position units
   and core materialization boundaries. MD-073 (char conversion) and MD-109
   (stdlib materialization) remain gated. Descriptor selection must not alter
   any of these source contracts.
8. **One track only:** accepted work is Sema → Semantic IR → Sec dialect MLIR
   → standard MLIR → LLVM dialect → LLVM IR → `llc` object (same LLVM build) → clang link
   (2026-10-18). P24 cannot restore a
   legacy generator or declare schema 20 complete while current executable
   schema/lowering remains v11.

## Completion boundary

P24 is complete only when one deterministic, plan-keyed `ResolvedLayout`
service resolves supported concrete scalar, aggregate, variant and selected
descriptor storage layouts with checked exact arithmetic; Semantic IR and
schema-v20 refer to those facts immutably; eligible representation operations
lower through the single native track; and every omitted high-level obligation
has an explicit static, runtime or lower-level discharge proof.

P24 does not implement final callable/foreign ABI, a universal physical
reference/slice/string/Arena descriptor, a global allocator ABI, source
packing syntax, MMIO/volatile access strategy, raw serialization, implicit
`reinterpret`, final LLVM lowering, MD-073 or MD-109.

## A. Establish baseline and synchronize authority

- [ ] P24-01 — Record exact HEAD, dirty/untracked inventory, installed schema
  and lowering versions, toolchain versions, `SEC_MLIR_BIN` setting and actual
  P15–P23 executable status. Do not equate planning documents with installed
  predecessors.
- [ ] P24-02 — Run and record focused layout/target/Sema/IR/lowering tests,
  `go vet`, dialect controls and full repository checks; separate existing
  failures from P24 failures.
- [ ] P24-03 — Map actual repository paths for memory/layout/storage/reference,
  target plan, type identity, defaults, ownership/destruction, Semantic IR,
  Sec MLIR tablegen/pass registration, codegen and platform ABI documentation.
  Do not invent a missing rulebook or assume a file name.
- [ ] P24-04 — Reconcile P24's package documents and governance wording with
  October corrections: MD-014/MD-012 authority, P21 mapped decisions and the
  settled P23 MD-050/071/072 state. Keep only MD-019, MD-073 and MD-109 as
  string/layout-adjacent decision gates where their behavior actually matters.
- [ ] P24-05 — Define the preflight predecessor/capability matrix: scalar and
  fixed aggregate first; P15 refs/P16 slices/P17 ownership/P18 arrays/P19 Arena
  /P21 decimal/P22 contracts/P23 strings only when their exact plan facts and
  lowering prerequisites are executable.

## B. Credit existing target and frontend foundations

- [x] P24-06 — Use `layout.ResolvedScalarPlan` and platform target definitions
  as authoritative existing target facts for pointer width, endian, selected C
  ABI, reference epoch policy and atomic capability snapshots. Its validation
  already rejects incomplete/inconsistent basic plans.

  Current anchors: `internal/layout/scalar_plan.go`,
  `internal/layout/reference_policy.go` and `internal/platform/target/target.go`.

- [x] P24-07 — Resolve source-level target-sized `int`/`uint` and plain
  `float` from the selected scalar plan, keeping fixed scalar widths distinct.
  Existing parity tests exercise 32-/64-bit target-sized source behavior.

  Current anchors: `internal/sema/analyzer_target_plan.go`,
  `internal/sema/types.go` and
  `internal/lowering/secmlir/target_scalar_parity_test.go`.

- [x] P24-08 — Reject by-value layout cycles across structs, tagged unions,
  wrappers, fixed arrays and generic carriers with deterministic source witness
  paths; indirections terminate the source containment graph.

  Current anchors: `internal/sema/analyzer_struct_layout.go`,
  `internal/sema/generic_layout_paths.go` and their focused tests. This does
  not calculate concrete offsets, sizes, stride, padding or representation.

- [x] P24-09 — Maintain compiler-known semantic surfaces for type/value size
  and sequence facts, including the distinction between string value UTF-8
  payload size and string descriptor type size.

  This is a typed frontend contract, not evidence that the query has a single
  executable `ResolvedLayout` implementation yet.

## C. Build the canonical layout resolver

- [ ] P24-10 — Introduce immutable `LayoutRequirement`, `LayoutContract`,
  `CompilationPlanLayoutKey`, stable snapshot-scoped `LayoutID`,
  `ResolvedLayout` and `ResolvedFieldLayout` records. Include concrete semantic
  TypeID/generic substitution, plan key, exact size/align, kind, fields, array,
  variant, descriptor, validity, stability and provenance without using display
  names or pointer addresses as identity.
- [ ] P24-11 — Converge `ResolvedScalarPlan`, target data-layout facts,
  alignment/address-space capabilities, representation policy, explicit
  validated constraints and compiler layout revision into a complete immutable
  layout key. An incompatible cache entry must be a miss/recompute, never a
  fallback or authority.
- [ ] P24-12 — Implement checked arbitrary-precision nonnegative arithmetic:
  `CheckedAdd`, `CheckedMul`, `AlignUp`, `StrideOf` and array-size calculation.
  Delay APInt/index/host-size casts until target representability is proven;
  diagnose the exact overflowing field/count/instantiation and target bound.
- [ ] P24-13 — Establish resolver states for complete, incomplete, invalid and
  unsupported layouts. Target-independent requirements may remain incomplete;
  no layout-sensitive materialization/lowering may proceed from one.
- [ ] P24-14 — Add deterministic worklist/memoization and an `in-progress`
  containment stack for concrete generic resolution. Reuse existing source
  recursion diagnostics where applicable, but report concrete substituted
  layout paths and keep valid indirect cycles distinct.
- [ ] P24-15 — Add cold-versus-warm cache tests and diagnostics for width,
  endian, alignment, address space, reference/descriptor policy, explicit
  contract and generic-substitution changes. Reject malformed/stale cache
  entries fail-closed and ensure stable IDs/printer ordering across runs.

## D. Resolve scalar and aggregate storage layouts

- [ ] P24-16 — Resolve bool, byte, char, rune, fixed signed/unsigned 8–256-bit
  integers, target-sized integers, fixed floats and plain target-width float.
  Preserve source scalar kind/provenance and separately record valid bit-pattern
  requirements; do not lower semantic sign/type identity to raw bits early.
- [ ] P24-17 — Resolve decimal/decimal128 carrier layouts as `{i64,i32}` and
  `{i128,i32}` under the same plan, including field offsets, aggregate alignment
  and tail padding. Distinguish public decimal scale from its physical i32
  field and reject obsolete/truncating generator shapes.
- [ ] P24-18 — Resolve named and constrained types as carrier layout plus exact
  nominal/contract validity provenance. Representation reuse cannot bypass P22
  validation or erase nominal identity/debug facts.
- [ ] P24-19 — Resolve structs in declared stored-field order: recursively
  resolve field layout, align each offset, record padding, calculate aggregate
  alignment/final size and preserve `FieldIdentity` independently of offset.
  Exclude methods/properties/static/nested declarations and never reorder.
- [ ] P24-20 — Resolve `T[N]` as exactly N inline slots using checked canonical
  stride/count arithmetic. Support zero-length and zero-sized-element layouts
  without manufacturing addresses; avoid N-way construction/destruction
  expansion for huge counts.
- [ ] P24-21 — Resolve fieldless/bit-backed/string-backed enum representation
  from canonical definitions and record discriminant validity. Do not hardcode
  i32, truncate values or treat zero-filled storage as automatically valid.
- [ ] P24-22 — Resolve tagged unions, `Result` and `Option` with one selected
  discriminator/tag encoding, maximal aligned payload, total padding and
  active-state validity. Permit niche/tag elision only when selected and proven
  in `ResolvedLayout`; otherwise retain explicit tag plus payload.
- [ ] P24-23 — Accept already validated packing/alignment/offset constraints
  from a layout contract, validate impossible combinations and model unaligned
  value access requirements. Do not parse or document new user syntax before
  MD-019 is resolved.

## E. Selected descriptors, placement and validity

- [ ] P24-24 — Attach selected P15 safe-reference representation policy, not
  `RawPtr` layout. An address-only shape is permitted only after all temporal,
  spatial, provenance, alignment, address-space, borrow/relocation and
  concurrency obligations are proven; dynamic epoch needs an actual protection
  path and retains 64 logical bits unless a canonical proof permits otherwise.
- [ ] P24-25 — Add P16 non-owning slice, P18 owning dynamic-array, P19 Arena
  and P23 string descriptor layouts only for explicit selected plan policies.
  Keep descriptor size separate from backing storage/ownership and reject
  missing policies instead of selecting universal pointer tuples.
- [ ] P24-26 — Preserve P23's distinctions: string type `SizeOf` is descriptor
  bytes; value `SizeOf` is UTF-8 payload bytes; `Len` is scalar count. A byte
  pointer in a descriptor is not a C NUL-string guarantee, and concat retains
  MD-004 one checked materialization.
- [ ] P24-27 — Resolve raw pointer/address-space representation only from plan
  facts. No implicit safe-ref↔RawPtr conversion, host-word assumption or
  truncation on capability/richer-pointer targets.
- [ ] P24-28 — Define representation validity separately from physical shape:
  initialized field/payload masks, bool/rune/char domain, enum discriminant,
  named contract, safe-reference facts, active variant, alignment and
  provenance. A load from arbitrary bytes needs proof or validation.
- [ ] P24-29 — Record padding ranges and prevent semantic equality/hash/
  serialization/safe loads from observing them. Do not universally zero padding
  or use ordinary copy as an FFI/shared-memory leak proof.
- [ ] P24-30 — Integrate placement with existing ownership/destruction facts:
  retain SSA for eligible non-address-observable trivial values; materialize
  storage only for required identity/borrow/mutation/cleanup/target facts;
  preserve static/TLS/Arena/automatic/allocator origins distinctly.

## F. Semantic IR layout facts and operations

- [ ] P24-31 — Add typed immutable `LayoutRequirementRef` and
  `ResolvedLayoutRef` with plan linkage, completeness and proof state. Attach
  them to materialized storage, field/array/variant places, descriptors,
  layout-sensitive copy/move and object construction without re-running Sema.
- [ ] P24-32 — Add or reuse exact Semantic IR facts/ops for require/resolve,
  size/align/stride/field-offset queries, representation materialize/project/
  load/store/check-validity and obligation discharge. Prefer analysis tables
  for static facts; `LayoutResolve` is a compiler stage, never runtime work.
- [ ] P24-33 — Gate every layout-sensitive IR path on a complete compatible
  layout ref. Preserve source semantic type and Place authority even when a
  physical representation is selected; reject wrong plan, cache or field ID.
- [ ] P24-34 — Model field and array projection from exact field identity/
  recorded offset and base/index/bounds proof; retain zero-sized identities but
  never manufacture an element address for `N=0` or nonaddressable storage.
- [ ] P24-35 — Model variant projection from active-variant/tag proof, layout
  payload position and P17 ownership facts. Inactive payload bytes never become
  a semantic value; use selected layout only after P12/P22 guards are valid.
- [ ] P24-36 — Establish verifier-visible obligation outcomes:
  `StaticallyDischarged`, `RuntimeMaterialized` and
  `TransferredToLowerLevel`. Link each to exact source/IR identity and plan;
  never erase borrow/epoch, bounds, active-state, validity, initialization,
  cleanup, address-space or alignment obligations merely because bits changed.

## G. Schema 20, emission and verification

- [ ] P24-37 — Install schema 20 only after executable schema-19 predecessor
  reconciliation. Add typed plan-scoped `#sec.layout<...>`/`sec.layout_ref`
  metadata and an optional internal transition wrapper only where essential;
  do not create a source/runtime type or duplicate field lists in MLIR.
- [ ] P24-38 — Add the minimal verified operation family:
  `sec.layout.size_of`, `.align_of`, `.stride_of`, `.field_offset`, and
  `sec.repr.materialize`, `.project_field`, `.project_element`,
  `.project_variant`, `.load_validated`, `.store_validated`,
  `.check_validity`, `.discharge`.
- [ ] P24-39 — Emit schema-20 layout refs only from complete canonical facts.
  Preserve plan ID, semantic type, field/variant identity, selected descriptor
  policy, alignment/address space, validity and outstanding obligations as
  typed metadata—not free-form backend guesses.
- [ ] P24-40 — Register `--sec-verify-resolved-layouts`,
  `--sec-verify-representation-lowering` and
  `--sec-verify-layout-obligations` after prior place/ownership/guard
  verifiers and before high-level metadata is discarded.
- [ ] P24-41 — Verify plan/type/cache identity, completeness, exact arithmetic,
  declaration order, non-overlap where required, stride/count, tag/payload
  fit, descriptor policy, validity/provenance, projection guards, alignment,
  address space and no premature ABI metadata.
- [ ] P24-42 — Add deterministic parse/print plus negative fixtures for
  wrong/missing LayoutID, stale plan, incomplete layout, wrong field offset,
  invalid N=0 projection, non-dominating validity/tag/bounds guard, unaligned
  safe reference, erased obligation, malformed descriptor policy and ABI data
  applied at this stage.

## H. Lower through the single native track

- [ ] P24-43 — Add `--sec-resolve-layouts` to resolve/attach/verify canonical
  facts and `--sec-lower-value-representations` for eligible intra-procedural
  representation lowering. Keep unsupported operations high-level with an
  explicit capability/deferred marker.
- [ ] P24-44 — Lower scalar representations using plan-selected widths and
  decimal composites; preserve semantic validity/provenance until explicit
  discharge and never derive values from compiler-host integer sizes.
- [ ] P24-45 — Lower eligible trivial value structs/arrays using exact resolved
  offsets, alignment and stride. Retain typed storage for address-observable or
  nontrivial values until P15/P17 authority and physical placement are proven;
  no field reorder, hidden length/capacity or large-array unroll.
- [ ] P24-46 — Lower verified field/element address/place computation only
  after matching bounds, provenance, address-space and alignment facts. For
  packed/unaligned storage, use target-supported unaligned value access or
  reject; never create a falsely aligned safe `ref`.
- [ ] P24-47 — Lower selected union/Result/Option tag and payload layouts with
  guards, active payload initialization and P17 destruction facts. Preserve
  explicit discriminant unless `ResolvedLayout` contains a complete selected
  niche proof.
- [ ] P24-48 — Lower refs, slices, dynamic arrays, strings and Arenas only when
  their selected descriptor policy plus runtime protection/lifetime/provider
  form exists. Do not synthesize `(ptr,epoch)` or `(ptr,len)` defaults, hidden
  heap allocation or C-string contracts.
- [ ] P24-49 — Fold plan-resolved static type layout queries to exact `uint`
  constants and route LSP/`sec analyse` through the same facts. Preserve the
  semantic distinction between string value payload size and descriptor type
  size.
- [ ] P24-50 — Keep call ABI, foreign ABI/FFI marshalling, sret/register/stack
  decisions, final LLVM lowering, MMIO/volatile strategy and source packing
  syntax out of all P24 passes.

## I. Required test matrix

- [ ] P24-51 — Test plan keys, snapshot IDs, cache invalidation and cold/warm
  equivalence across 32/64-bit widths, endian, pointer alignment/address space,
  descriptor/reference policy, explicit contract, generic substitution and
  malformed cache injection.
- [ ] P24-52 — Test scalar sizes/alignments/validity for bool/byte/char/rune,
  signed/unsigned 8–256 bits, target-sized ints, plain/fixed floats, decimal/
  decimal128 and named carriers. Include invalid bool/rune/enum/reference bit
  patterns and reject generic all-zero validity assumptions.
- [ ] P24-53 — Test struct field declaration order, empty/nested/zero-sized
  fields, mixed alignment, interior/tail padding, field identity, validated
  metadata constraints and checked overflow diagnostics.
- [ ] P24-54 — Test fixed arrays N=0/N=1, padded stride, zero-sized elements,
  huge-count overflow and exact index displacement without element address
  creation for zero length.
- [ ] P24-55 — Test enum/union/Option/Result tag/payload layout, unit and
  mismatched payload alignment, inactive-payload protection, chosen niche
  proof and negative unproven optimization paths; test direct/indirect and
  generic recursive containment paths.
- [ ] P24-56 — Test each enabled descriptor policy: proven/dynamic epoch refs,
  zero-length slices, move-only dynamic arrays, Arena identity and known-valid
  strings. Include rich pointer/address-space plans, relocation/reclamation
  conflicts and rejection rather than metadata truncation.
- [ ] P24-57 — Test schema-20 parse/print/verifiers and lowering negatives for
  layout mismatch, missing proof, wrong guard/offset/address space/alignment,
  moved-source cleanup, premature ABI and erased obligation.
- [ ] P24-58 — Test source/LSP/`sec analyse` layout-query parity, deterministic
  printer output and target mock matrix. Run focused Go/vet/dialect/lowering/
  lit suites plus full repository checks, recording exact commands/results and
  clearly marking work not run.

## J. Acceptance, governance and handoff

- [ ] P24-59 — Reconcile P24 acceptance by phase. Existing scalar-plan and
  recursive-layout source analysis do not satisfy the canonical resolver,
  layout-sensitive IR, schema 20 or representation-lowering boundary.
- [ ] P24-60 — Update only `frontend.layout`, `semantic-ir.layout`,
  `lowering.layout`, `platform.layout` and `tooling.layout` in
  `governance/memory.yaml`, plus `language-rulebook-status.md`, with exact
  evidence, capability boundaries and no duplicate IDs.
- [ ] P24-61 — Produce the required implementation report: snapshot,
  predecessor truth, source authority, resolver/key/cache inventory, supported
  layouts/policies, IR/schema/pass behavior, verifier and test evidence,
  unresolved MD-019/MD-073/MD-109 gates, deviations and P25 handoff.

## Recommended execution order

1. A: capture the real execution baseline, reconcile stale P24/P23 decision
   prose and map existing target/layout code before adding a service.
2. B/C: converge plan facts into one immutable resolver with exact arithmetic,
   deterministic identity and cache safety; finish scalar, struct and fixed
   array layouts before descriptor policies.
3. D/E: add variants, validity and only explicitly selected descriptor policies
   while retaining all ownership/borrow/contract boundaries.
4. F/G/H: attach verified layout refs, then schema 20 and one native-track
   representation lowering for individually eligible operations.
5. I/J: prove target/cache/tooling parity and leave ABI, MD-019, MD-073,
   MD-109 and unsupported descriptor policies explicit rather than speculative.
