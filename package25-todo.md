# SEC-MLIR Package 25 — implementation TODO

Package: `SEC-MLIR-P25` — Callable ABI Classification and Foreign Representation Boundary  
Primary rulebook: `rules/mlir/packages/sec-mlir-dialect_package25.md`  
Package contract: `rules/mlir/packages/sec-mlir-dialect_package25.yaml`

Normative inputs:

- `rules/mlir/normative-sync/sec_callable_abi_sync_package25.md`
- `rules/mlir/semantic-ir/sec_semantic_ir_callable_abi_package25.md`
- `rules/mlir/dialect-versions/sec_mlir_dialect_v21.md`
- `rules/mlir/lowering-versions/sec_mlir_lowering_v21.md`
- `rules/mlir/packages/sec-mlir-dialect_package25_decision-index.md`
- `rules/platform/abi.md`, `rules/platform/ffi.md` and
  `rules/platform/target_profiles.md`

This is an implementation plan, not a completion report and not a new FFI
language design. P25 consumes legal source declarations and P24 physical
layouts to construct one immutable, target-selected callable ABI plan. It must
not re-decide FFI legality, mutate a source signature, fabricate ownership or
borrow facts, or let a later LLVM stage classify calls a second time.

## Inventory snapshot — 2026-10-18

Audited HEAD:

```text
831f6e1
```

The working tree contained 145 changed entries when this package was audited.
Those intended predecessor changes, including Package 20–24 TODO documents,
must be captured before P25 acceptance can be reproduced.

| Area | Current implementation | P25 consequence |
|---|---|---|
| Target C facts | `layout.CABIModel` models target C fundamental scalar widths, signedness and long-double formats; target plans select a C model. | Reuse as source input, but add a full plan-keyed `ABIModel` for calling transport rather than treating C scalar width as call classification. |
| Source FFI | Parser/Sema recognize `extern` Sec/C/system conventions, concrete extern restrictions, positional foreign legality, c:: scalar identity and unsafe/null constraints. | Preserve resolved legality and diagnostics; ABI code must not reinterpret syntax or repair illegal declarations. |
| Semantic IR | Function and call facts retain `Extern`, ABI, link name, unsafe state, source types and reference metadata. | Extend with immutable ABI-intent/layout/transfer references without replacing semantic callable types by physical parts. |
| Current MLIR emission | The emitter writes high-level `func.func` declarations and Sec source ABI metadata, and rejects unsupported foreign fixed-array boundaries. | It has no `CallABIPlan`, transport ops, hidden-result/stack assignment or ABI verifier. |
| Layout/ownership predecessors | P24's general `ResolvedLayout` service and schema 20 are planned; P17/P19 transfer/hidden-context work is not a completed executable ABI substrate. | P25 must phase-gate classification/lowering by actual predecessor capability, not package number. |
| Current track | The maintained executable dialect/lowering is schema v11; the repository has one Sec → standard MLIR → LLVM dialect track. | Schema 21 and ABI lowering remain planned until all staged prerequisites and verifiers work. |

Primary implementation anchors:

```text
internal/layout/c_abi.go
internal/layout/scalar_plan.go
internal/platform/target/target.go
internal/sema/foreign_c_types.go
internal/sema/ffi.go
internal/sema/ffi_legality.go
internal/sema/stack_external_contracts.go
internal/ir/semantic/model.go
internal/ir/semantic/builder.go
internal/ir/semantic/verifier.go
internal/lowering/secmlir/emitter.go
internal/lowering/secmlir/emitter_test.go
governance/platform.yaml
governance/ffi.yaml
governance/compiler.yaml
```

## Authority and dependency constraints

1. **Three identities remain distinct:** `SemanticDeclarationIdentity`,
   `ABISignature`/`ABIFingerprint`, and `BinarySymbolIdentity` are never
   interchangeable. ABI-equivalent nominal Sec types retain their source
   identity; equal link names never bypass ABI compatibility.
2. **P24 is the only physical-layout source:** ABI classification consumes a
   complete compatible `ResolvedLayout`; it must not recompute offsets, sizes,
   descriptor shapes or target data layout, nor use Go-host/LLVM defaults.
3. **Legality precedes classification:** Sema determines position-sensitive
   C/system legality, `unsafe extern` obligations, c:: identity, vararg
   promotion facts and call-bounded reference constraints. Matching bytes never
   makes `string`, slice, dynamic array, `Result`, `Option` or a native
   descriptor foreign-legal.
4. **Calling conventions are typed:** `Sec`, `C` and `system` remain distinct
   source/ABI families even when C and system happen to match on a particular
   target. Unknown spellings are source errors; a raw foreign return never
   manufactures a safe reference.
5. **Applied decisions are implementation requirements:** MD-020 (lowercase
   `c::`), MD-023 (foreign integer/wide-scalar position validation) and MD-024
   (unsafe/null distinctions) are settled. They must not become new P25-D
   questions.
6. **Existing gates remain narrow:** MD-019 blocks source explicit-layout
   spelling; MD-021 blocks a new target-binding environment producer; MD-045
   blocks invented external stack/reentry metadata; MD-052 concerns privileged
   low-level bodies. None permits host fallback or a new annotation syntax.
7. **ABI transport never changes semantic ownership:** P17 transfer/cleanup,
   P15 reference validity and P19 allocation-context facts are consumed, not
   created, erased or exposed as source parameters. Bit copies, split parts,
   spills and ABI temporaries are not Sec copies/owners/borrows.
8. **One track only:** accepted work remains Sema → Semantic IR → Sec dialect
   MLIR → standard MLIR → LLVM dialect → LLVM IR → `llc` object (same LLVM
   build) → clang link (2026-10-18). P25 adds no direct
   legacy emitter or ad-hoc `llvm.call` classification path.

## Completion boundary

P25 is complete only when a concrete callable under a selected
`CompilationPlan` has a deterministic, immutable position-aware `CallABIPlan`
with return-first classification, hidden roles, complete resource/stack
assignment, canonical `ABISignature` and `ABIFingerprint`; ABI-lowered
schema-21 entry/call/return transport follows that one plan; and ABI, foreign
boundary and compatibility verifiers prevent mismatches, lost obligations and
stale artifacts.

P25 does not create general C exports, source calling-convention/packing
syntax, target binding files, foreign error normalization, universal descriptor
shapes, GC/runtime policy, callback registry, final LLVM ABI encoding, linker
name mangling or a raw-to-safe-reference conversion.

## A. Establish baseline and synchronize authority

- [ ] P25-01 — Record exact HEAD, dirty/untracked inventory, installed
  dialect/lowering schemas, MLIR/toolchain versions, `SEC_MLIR_BIN` setting and
  actual P15–P24 implementation evidence. Preserve pre-existing local work.
- [ ] P25-02 — Locate canonical ABI/FFI/platform/target/layout/link/debug and
  generic-lowering rulebooks with `git ls-files`; inventory actual platform,
  Sema, Semantic IR, lowering/TableGen/pass and foreign-call tests before any
  ABI change.
- [ ] P25-03 — Run and record focused C ABI, FFI, extern, generic-extern,
  Semantic IR, emitter, target and native-track tests plus vet/full-suite
  controls. Separate pre-existing failure from P25 regression.
- [ ] P25-04 — Reconcile local `missing-decisions.yaml`, corrections and
  governance with P25's decision index. Record MD-019/021/045/052 only at the
  affected capability boundaries; do not reopen applied MD-020/023/024.
- [ ] P25-05 — Build a per-feature predecessor matrix: P24 resolved layout,
  P15 refs, P17 transfer/cleanup, P19 hidden allocation context, generic
  specialization, foreign C declarations, callbacks and varargs. Each missing
  prerequisite must fail closed before ABI transport.

## B. Credit the existing source and target foundation

- [x] P25-06 — Select target C fundamental scalar facts from an immutable
  `layout.CABIModel`, including data model, char signedness, C widths and
  long-double representation. C scalar identity remains distinct from a Sec
  scalar even if their physical widths match.

  Current anchors: `internal/layout/c_abi.go`, `internal/layout/scalar_plan.go`
  and `internal/platform/target/target.go`.

- [x] P25-07 — Resolve compiler-known lowercase `c::` fundamentals into the
  active Sema target plan, preserve nominal identity and literal-shaping rules,
  and give unresolved C binding types target-aware diagnostics instead of host
  guesses.

  Current anchor: `internal/sema/foreign_c_types.go`.

- [x] P25-08 — Validate extern declarations positionally before lowering:
  distinguish Sec/C/system, reject unresolved generic externs, enforce raw
  foreign type restrictions and reject unsupported wide scalar, native
  descriptor, foreign-return reference and array cases.

  Current anchors: `internal/sema/ffi.go`, `internal/sema/ffi_legality.go` and
  their source/LSP/compiler tests. This is legality analysis, not ABI part or
  stack classification.

- [x] P25-09 — Preserve high-level extern/call source facts through Semantic
  IR and Sec MLIR emission: source ABI, link identity, unsafe provenance,
  semantic parameter/result types and reference metadata remain visible.

  Current anchors: `internal/ir/semantic/model.go`, `builder.go`, `verifier.go`
  and `internal/lowering/secmlir/emitter.go`.

- [x] P25-10 — Keep unimplemented foreign aggregate ABI requests explicit at
  the existing emitter boundary rather than guessing an aggregate transport.

  Current anchor: `validatePackage14ArrayBoundaries` and its focused emitter
  tests. It is a fail-closed predecessor boundary, not P25 ABI lowering.

## C. Canonical ABI model and position classification

- [ ] P25-11 — Define immutable, plan-scoped `ABIModelRef`/`ABIModelID` and
  ABI revision/schema identity. Resolve one authoritative family-specific Sec,
  C and system classifier from a complete `CompilationPlan`; reject missing or
  unverified target model with an actionable diagnostic.
- [ ] P25-12 — Define a complete ABI cache key over ABI model/revision,
  compilation-plan ABI identity, convention, position, concrete monomorphized
  TypeID, P24 LayoutID/representation policy and foreign-legal contract. Cache
  is an optimization only: stale/incompatible entries recompute or fail closed.
- [ ] P25-13 — Introduce read-only position-aware requests and classifiers for
  parameter, return and foreign vararg. Forbid a position-blind `GetABIType(T)`
  API; unresolved generic/target-layout facts are unavailable, not guessed.
- [ ] P25-14 — Define immutable `ABIArgInfo`/`ABIReturnInfo`: passing kind
  `Direct`/`Indirect`/`Ignore`, ordered abstract ABI parts, coercion/extension,
  required alignment, indirect-storage role, transport attributes, source
  position identity and classification provenance. Do not encode LLVM types or
  hardware register names here.
- [ ] P25-15 — Implement one registered target ABI vertical slice for fixed
  scalars, target-sized integers, plain/fixed floats, RawPtr and legal c::
  scalars in parameter/return positions. Preserve extension as machine
  transport, never a Sec semantic conversion.
- [ ] P25-16 — Extend classification to bool/byte/char/rune, int128/int256,
  decimal/decimal128, enums and aggregates only where the selected ABIModel
  explicitly supports their concrete position and P24 layout/validity facts.
  Width existence does not prove foreign legality.
- [ ] P25-17 — Classify structs, fixed arrays, C aggregates/bitfields/flexible
  headers, unions, Result/Option, native descriptors and nested aggregates only
  according to model-specific rules. Do not use size-threshold or universal
  register-splitting heuristics; native tagged union never becomes a C union.
- [ ] P25-18 — Keep native safe-reference transport bound to the complete P24
  selected representation. Adapt an approved C/system reference only at a
  verified call-bounded non-retained parameter boundary; raw foreign returns
  remain raw pointers.

## D. Whole-call plans, resources and compatibility identity

- [ ] P25-19 — Define immutable `CallABIPlan`, `CallABIPlanID`,
  `ABISignature`, `ABIFingerprint` and semantic-to-physical correspondence.
  Include semantic callable/instantiation identity, model/schema, convention,
  return/parameter/vararg classifications, hidden roles, resource assignment,
  stack requirements and P24 representation identity.
- [ ] P25-20 — Build the plan once in deterministic order: resolve concrete
  callable/model/layouts; validate precomputed legality; classify return;
  introduce hidden return storage if required; classify source arguments in
  semantic order; classify promoted C varargs; collect native hidden roles;
  assign resources; finalize stack; serialize/verify/publish.
- [ ] P25-21 — Assign complete call resources under ABIModel state: abstract
  registers/classes, direct multi-part values, indirect roles, register/stack
  fallback, stack offsets/size/alignment, shadow/home and vararg save-area
  requirements. Use P24 checked arithmetic, never host `sizeof` or backend
  reclassification.
- [ ] P25-22 — Model hidden indirect return (`sret`) storage after return
  classification. It is compiler transport storage, not source RawPtr; callee
  initializes once, caller receives one semantic owned result and P17 cleanup
  never destroys a moved returned value twice.
- [ ] P25-23 — Reconcile P19 allocation context, receiver/environment and
  hidden result roles in one ABI-model-selected deterministic ordering. Retain
  original source arity/ordinals and debug provenance; never inject Sec hidden
  context into ordinary C/system ABI.
- [ ] P25-24 — Serialize ABI meaning canonically and deterministically without
  source locations/display names: model/schema, convention, positions/parts,
  hidden roles/order, vararg state, stack requirements and representation IDs.
  Fingerprint is an accelerator only; structural equality remains authoritative.
- [ ] P25-25 — Persist ABI signature/fingerprint, plan identity, semantic
  callable identity and producer/trust provenance for separate compilation.
  Invalidate ABI artifacts precisely when target/layout/model/schema/convention/
  hidden role/foreign contract changes; test cold and warm compilation.

## E. Preserve semantic IR while attaching ABI intent

- [ ] P25-26 — Attach typed calling-convention intent, target ABIModelRef,
  concrete plan identity, resolved foreign-legality proof, concrete generic
  identity, P24 layout ref and P17 transfer/cleanup facts to Semantic IR calls
  and functions without reinterpreting source syntax.
- [ ] P25-27 — Differentiate native direct/indirect calls, imported foreign
  calls, C function-pointer calls, callback creation, native typed variadics
  and C varargs. Preserve source function type, Result/error channels,
  parameter names/ordinals and semantic evaluation order.
- [ ] P25-27a — MD-117 (added 2026-10-27): classify heterogeneous native
  variadic type packs (`[...Types]` with `...Types` / `...ref Types` /
  `...ref mut Types`) position by position as ordinary native parameters of
  each concrete specialization's ordered element types and modes; include the
  ordered type sequence in callable identity, `ABISignature` and fingerprint;
  keep homogeneous runtime-length `...T` transport; never route either through
  C varargs, `va_list`, promotions, boxing, erasure or heap allocation; treat
  `<-values...` as a call-committed ownership transfer, not ABI transport
  (package25 § 22 amendment; functions.md § 38A; Semantic IR § 27A).
- [ ] P25-28 — Add read-only `CallABIPlanRef` after successful resolution. Do
  not rewrite source `sec.call`/function signature into physical parts in
  Semantic IR; ABI materialization begins only at the P25 Sec MLIR boundary.
- [ ] P25-29 — Extend Semantic IR verification for concrete generics, exact
  transfer/cleanup action, legal foreign facts, compatible complete layout,
  no implicit descriptor conversion, no generic foreign export and no captured
  callback candidate. Preserve reference validity/provenance through the ABI
  boundary.

## F. Schema 21, ABI transport and verifiers

- [ ] P25-30 — Install schema 21 only after executable schema-20/P24
  predecessor reconciliation. Add immutable typed ABI model/signature/
  classification/call-plan/fingerprint references and required semantic↔ABI
  correspondence; no new source or runtime ABI value type.
- [ ] P25-31 — Add the minimal ABI transport family: `sec.abi.pack_arg`,
  `.unpack_arg`, `.prepare_indirect`, `.prepare_return`, `.pack_return`,
  `.unpack_return`, `.call`, `.func`, `.callback_bridge` and
  `.vararg_promote`. Reuse exact existing operations if semantically identical.
- [ ] P25-32 — Ensure every pack/unpack part count/type/order/extension and
  transport temporary comes from exactly one classification/CallABIPlan. Parts
  do not become independent Sec values/owners, and bridge operations keep the
  high-level declaration/call identity for diagnostics and debug.
- [ ] P25-33 — Register `--sec-verify-call-abi-plans` for model/layout/position
  identity, concrete types, return-first hidden roles, resource assignment,
  stack alignment, vararg promotion and canonical signature/fingerprint.
- [ ] P25-34 — Register `--sec-verify-abi-lowering` for caller/callee plan
  match, part structure/order, indirect/sret storage roles/alignment, hidden
  role ordering, source transfer preservation and no raw-pointer/reference
  metadata fabrication.
- [ ] P25-35 — Register `--sec-verify-ffi-boundaries` and
  `--sec-verify-abi-compatibility` for positional legality/unsafe provenance,
  c:: identity, no unwind/implicit conversion, callback two-sided plans and
  persisted signature/fingerprint/plan compatibility. Unknown metadata fails
  closed; same-build inconsistency is an internal invariant diagnostic.
- [ ] P25-36 — Add deterministic parse/print and negative schema fixtures for
  missing/wrong plan/model/layout, C/system false equivalence, position-blind
  classification, bad hidden order, stack misalignment, stale fingerprint,
  illegal descriptor crossing, raw foreign ref return, generic export and
  unverified callback capture.

## G. Lower calls and native ABI boundaries through one track

- [ ] P25-37 — Add staged passes `--sec-resolve-call-abi` and
  `--sec-lower-call-abi` after source/foreign-legality and P24 layout/validity
  verification. Lowering consumes one immutable plan; it never asks LLVM to
  decide aggregate pass/return classification again.
- [ ] P25-38 — Lower native Sec direct calls, indirect native function calls,
  entries and returns by evaluating receiver/arguments once in semantic order,
  applying P17 action first, then marshalling exact ABI parts/temporaries and
  reconstructing exactly one semantic parameter/result on the other side.
- [ ] P25-39 — Lower indirect returns, multi-part returns and stack fallback
  using verified transport storage/resource plans. Preserve initialization,
  alias limits, transfer/cleanup and semantic return identity; do not double
  destroy result temporaries or owners.
- [ ] P25-40 — Lower approved C/system calls only after source legality and
  selected foreign ABI classification. Keep raw pointer/address space, C
  aggregate layout, trust/unsafe provenance and source link identity; no
  implicit string/slice/array/Result/null/status adaptation or automatic errno.
- [ ] P25-41 — Lower C varargs only for already-C-typed promoted final varargs
  under a verified model-specific resource plan. Native typed variadics/spread
  never reuse this path; untyped null and unclassified extras are rejected.
- [ ] P25-42 — Lower a `c::callback` only as one private noncapturing reusable
  bridge per concrete callable/C-signature/plan/adapter revision. Decode C
  parts, establish only required runtime attachment, invoke native plan, encode
  approved result, detach only a newly created attachment and prohibit unwind.
- [ ] P25-43 — Preserve external stack/no-reentry/trust facts conservatively.
  Consume verified MD-045 producers where available; otherwise reject only
  positive proof-dependent paths rather than inventing metadata or pretending
  unknown foreign behavior is safe.
- [ ] P25-44 — Leave final LLVM call attributes/IR ABI encoding, linker export
  policy, arbitrary C export, callback runtime registry, MD-021 binding
  serialization and unsupported P24 descriptor policies out of P25 lowering.

## H. Debug, tooling and required test matrix

- [ ] P25-45 — Provide source-to-ABI debug/tooling correspondence for every
  source parameter/result: semantic name/type/ordinal to zero/one/many parts,
  offset/register class/stack slot and hidden purpose. LSP/diagnostics show
  source signatures, not synthetic sret/context plumbing.
- [ ] P25-46 — Test ABIModel/classifier on at least 32-/64-bit plans and
  materially different registered ABI families; C versus system equal/different
  cases; target `c::long`/`long_double`; plain/fixed float; legal/rejected wide
  integer positions; scalar extension and RawPtr address spaces.
- [ ] P25-47 — Test direct, multi-part, indirect and Ignore transport for
  supported aggregate/array/enum cases, return-versus-parameter differences,
  hidden sret/context/receiver roles, register pressure/stack fallback,
  alignment and source-to-part maps. P24 LayoutID must agree in all cases.
- [ ] P25-48 — Test C ABI aggregates, C unions/bitfields/flexible headers,
  opaque by-value rejection, native tagged-union rejection, illegal native
  descriptors, call-bounded reference adaptation and raw-return no-safe-ref
  rule as feature support becomes executable.
- [ ] P25-49 — Test ordinary versus unsafe extern, no-unwind behavior, legal
  c:: type identity, callback eligibility/capture rejection/private thunk,
  attachment reuse/detach, C vararg promotions and rejection of native spread
  or untyped null varargs.
- [ ] P25-50 — Test signature/fingerprint determinism, stale persisted
  artifacts, warm/cold cache equivalence, changed model/schema/target/layout/
  convention/hidden roles, caller/callee mismatch and independent map/traversal
  order. Fuzz legal aggregate shapes and malformed ABI metadata.
- [ ] P25-51 — Add schema/verifier/lowering negatives and cross-target smoke
  controls. Cross-check a C fixture only on the intended target with available
  runner/toolchain; otherwise record it as not run. Run focused Go/vet/dialect/
  lit and full repository suites with exact commands/results.

## I. Acceptance, governance and handoff

- [ ] P25-52 — Reconcile acceptance by phase. Existing source FFI legality,
  C scalar resolution and high-level extern emission do not establish an
  ABIModel, `CallABIPlan`, schema 21, transport lowering or persisted ABI
  compatibility.
- [ ] P25-53 — Find and update only existing governance IDs for platform ABI,
  FFI, compiler/lowering/tooling and layout dependencies, plus
  `language-rulebook-status.md`. Record exact supported ABI families/tests and
  fail-closed boundaries; do not create duplicate IDs or declare source FFI
  complete merely from ABI work.
- [ ] P25-54 — Produce the required implementation report: snapshot and
  predecessor evidence; authoritative rules/MD mapping; existing ABI inventory;
  model/key/classifier/call plan; layouts/transfers/hidden roles; signatures/
  caches; Semantic IR/schema/passes/verifiers; foreign/vararg/callback/debug
  state; target tests; unresolved gates; and P26 recommendation.

## Recommended execution order

1. A: audit actual P24/P15/P17/P19 readiness and local decision/governance
   state before adding any physical ABI form.
2. B/C: construct one plan-keyed ABIModel and position-aware classifier for a
   narrow registered scalar/C vertical slice, with cold-cache proof.
3. D/E: add whole-call resource planning, immutable signature/fingerprint and
   Semantic IR intent while preserving source identities and transfers.
4. F/G: after executable P24 schema/layout exists, add schema 21 verifiers and
   lower native ABI transport before widening cautiously to foreign calls,
   varargs and callbacks.
5. H/I: prove debug/tooling/cross-target/persistence behavior, retaining every
   unsupported ABI family and MD-019/021/045/052 boundary explicitly.
