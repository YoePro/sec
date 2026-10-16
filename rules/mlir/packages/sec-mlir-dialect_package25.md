# Sec MLIR Program — Package 25: Callable ABI Classification and Foreign Representation Boundary

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec-mlir-dialect_package25.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P25`

---

## § 1. Mission and authoritative ownership

P25 implements **physical callable ABI classification and ABI-lowered call/function boundaries** for already semantically valid Sec code. The canonical authority is `rules/platform/abi.md` (normative, rev 1, 2026-08-16), with foreign legality defined separately by `rules/platform/ffi.md` (rev 2.0, 2026-09-27). P24 provides the one `ResolvedLayout` source. P25 does **not** choose ownership, infer raw pointers from safe refs, or define exports, symbol mangling, data layout or calling syntax.

Intended sequence:

```text
Semantic IR with resolved call/ownership facts
    -> high-level Sec MLIR (schema v20 from P24)
    -> canonical resolved ABIModel from CompilationPlan
    -> position-specific classification / complete CallABIPlan
    -> ABI-lowered Sec MLIR (planned schema v21)
    -> later standard MLIR / LLVM dialect / backend
```

P25 is a **planned schema v21 and lowering v21** specification, not evidence that any preceding package is already implemented.

## § 2. Implementation status and repository preflight

User-confirmed last status: P1–P14 implemented; P15 underway; P16/P17 corrected plans; P18–P24 actual implementation status must be checked. P24 is a layout specification, not a completed layout implementation.

Before modifying the checkout, record `git rev-parse HEAD`, `git status --short`, `git ls-files` for rules and dialect sources, `git log -1 --format=%H`, existing dialect/lowering version, MLIR version and test commands. Preserve user's corrected P16–P18 plans and any uncommitted changes. Neither a package number nor a governance planned status proves code was merged. Phase-gate ABI lowering when P24 selected physical representation is not available.

## § 3. Canonical repository sources

Read and reconcile these **current October paths**, not the pre-refactor August paths:

```text
rules/platform/abi.md
rules/platform/ffi.md
rules/platform/platform_model.md
rules/platform/target_profiles.md
rules/memory/layout.md
rules/memory/storage.md
rules/memory/reference_model.md
rules/memory/copy_move.md
rules/compiler/semantic_ir.md
rules/compiler/compiler_pipeline.md
rules/compiler/generics_lowering.md
rules/compiler/linking.md
rules/compiler/debug_information.md
rules/mlir/sec_mlir.md
rules/mlir/sec_mlir_dialect.md
rules/mlir/sec_mlir_lowering.md
rules/declarations/functions.md
rules/declarations/lambda_functions.md
missing-decisions.yaml
```

Resolve exact paths with `git ls-files`. Read `governance/platform.yaml`, the actual per-area implementation fragments, and all existing call/ABI/foreign tests. Do not invent unavailable files or rewrite existing rulebook paragraph numbers.

## § 4. Decision registry and stale-check rule

The GitHub `missing-decisions.yaml` snapshot observed on 2026-10-16 has `updated: 2026-10-15`; it may lag the user's uncommitted work. Consult it to avoid duplicate P25-D decisions, but recheck local canonical files and ask for a decision only if a genuine unsolved fork remains.

Existing relevant gates: MD-019 (explicit source layout syntax), MD-021 (serialization/selection of target `c::` binding environment), MD-045 (optional external callable/stack/reentry contract producers), MD-052 (low-level privileged core body syntax; not an ABI source). Already applied MD-020/MD-023/MD-024 must **not** be reopened. See decision index. No new P25-D# is required for the core ABI classification problem.

## § 5. Three separate identities

Keep distinct at every stage:

```text
SemanticDeclarationIdentity
ABISignature / ABIFingerprint
BinarySymbolIdentity
```

The first belongs to source/modules, the second to ABI, the third to linking. ABI equivalence must never make two nominal Sec types semantically interchangeable; equal symbol spelling must never bypass ABIFingerprint checks.

## § 6. Resolved ABIModel comes from CompilationPlan

Each concrete `CompilationPlan` resolves one authoritative `ABIModel` with typed identity/version and family-specific Sec/C/system classification under the selected target requirements. Reuse the immutable platform resolver and canonical plan cache. Do not select by host OS, target triple string alone, function name, LLVM defaults or C compiler guesses. Missing/unverified model fails closed with an actionable target/ABI diagnostic.

## § 7. Calling conventions

Recognize source families `Sec`, `C`, `system`; exact family spelling and case are normative. Native ordinary Sec functions use Sec ABI. Imported extern declarations may be `extern "Sec"`, `extern "C"` or `extern "system"`. The system ABI is allowed to be physically identical to C in a particular plan but never globally aliased in the type system. Unknown convention strings are compile-time errors.

## § 8. ABI legality precedes physical classification

Pipeline stage 1 is Sema/FFI **legality**: validate source types, complete layout where required, `unsafe extern` call obligations, call-bounded borrow, `c::` binding identity, vararg promotion, provenance and target support. Stage 2 is ABI physical classification of the already-approved declaration. A matching layout does not authorize a Sec `string`, `Result`, `Option` or native descriptor in an `extern "C"` signature. No ABI pass repairs an illegal foreign source signature by silently inventing conversions.

## § 9. Ordinary extern and unsafe extern

Per current `rules/platform/ffi.md` §3: ordinary `extern` denotes a compiler-unverified foreign trust boundary, **not** a mandatory `unsafe {}` at every call. Only `unsafe extern` carries additional caller proof obligations and requires an unsafe call context. Do not regress to obsolete pre-September blanket `unsafe` rules. `unsafe` does not waive ABI legality, null restrictions, effects, lifetime, or prohibition on unwind.

## § 10. Position-sensitive classification interface

Provide read-only canonical operations equivalent to:

```go
type ABIPositionKind string

const (
    ABIParameter ABIPositionKind = "parameter"
    ABIReturn    ABIPositionKind = "return"
    ABIVarArg    ABIPositionKind = "foreign-vararg"
)
```

`ClassifyParameter`, `ClassifyReturn` and `ClassifyVarArg` consume concrete semantic type, `ResolvedLayout` and `ABIModel`. Do not implement one position-blind `GetABIType(T)` that loses return/vararg differences. Reject unresolved generic or target-dependent position facts.

## § 11. Direct, indirect, ignored transport

Classification must distinguish **Direct**, **Indirect** and **Ignore** as defined by ABI §9. Direct may consist of multiple ABI parts; Indirect may refer to caller-owned outgoing argument temporary or hidden return storage; Ignore is permitted only under model-specific rules (including valid zero-sized transport). None of these operations introduces a Sec source pointer, new owner, borrow or semantic copy.

## § 12. ABIValuePart and resource classes

Each directly transported part records representation offset/width, machine class, coercion, required extension, alignment and transport attrs. Classes are abstract (integer, floating, vector, memory/other as selected model). Do not record hard-coded physical register names in the canonical target-neutral plan. Keep a part-to-semantic-field mapping for debugging and verification.

## § 13. Scalar classification including wide types

Cover fixed `int8`..`int256`/`uint8`..`uint256`, plan-sized `int`/`uint`, `byte`, `bool`, `char`, `rune`, `float32`, `float64`, plan-sized plain `float`, `decimal`, `decimal128` and compiler-known C scalar types when legal. Width existence in code generation does **not** automatically make int128/int256 foreign-legal; require the selected foreign ABI to support that exact call position. Plain `float` follows the October platform-width correction. ABI sign/zero extension is machine transport, not a Sec conversion.

## § 14. Foreign C scalar identities

Use lowercase `c::` only, per applied MD-020; examples include `c::int`, `c::long`, `c::bool`, `c::double`, `c::long_double`, `c::stddef::size_t`. `c::int` and `int32` stay semantically distinct even if transport bits match. `c::long` and `c::long_double` are target-specific. MD-021 blocks the **new target-binding metadata producer**, not fundamental c:: types already supplied by a valid resolved plan. Never substitute host C sizes.

## § 15. Aggregate classification is model-specific

Consume P24 resolved offsets, field categories, packing, alignment, size/stride, representation validity and homogeneous-aggregate eligibility. Cover structs, fixed arrays, enums, unions, Result/Option where legal, descriptors and nested aggregates. No universal `size <= N -> registers` heuristic. Same semantic type may classify differently in a parameter and return. Machine classes and part splits come solely from `ABIModel`. Reuse the same classifier for call and callee.

## § 16. Foreign aggregates and bitfields

C-compatible `extern "C" type ... struct/union/enum` follow `rules/platform/ffi.md` and the target C ABI. C unions have overlapping representation **without** hidden Sec active-tag state; native tagged unions do not become C unions. Field order is representation significant for foreign structs. Bitfield allocation units, placement and signedness follow resolved C ABI; `c::flex[T]` is final member, contributes only fixed header to SizeOf, never an implicit slice. An incomplete foreign struct cannot be passed/returned by value.

## § 17. Reference and RawPtr transport

Native `ref T` and `ref mut T` may have generation/provenance/bounds metadata and must use the **complete selected safe-reference representation**. A native safe reference is never assumed to be one pointer. `RawPtr[T]` uses the selected address-space pointer representation without becoming non-null, owned or bounded. FFI accepts call-bounded non-null `ref T`/`ref mut T` parameters only when explicitly legal and proves the borrow cannot escape; it lowers to the foreign address representation at that boundary. A raw extern must not return a safe `ref`/`ref mut`.

## § 18. Native descriptors versus foreign surfaces

Sec ABI may transport native `string`, slices, owning `T[]`, collection descriptors, `Result`/`Option` and native callable values using P24 representation. None automatically acquires C compatibility. No implicit `string -> char*`, `T[] -> ptr+len`, `ref T[] -> C array` or `Result -> C status integer` conversion. Explicit checked wrapper code is the only such adaptation boundary.

## § 19. Native callable and C function pointer

`fn(...) R` and `c::fn(...) R` are distinct semantic and representation families. A C function pointer uses the resolved C function-pointer ABI, including nullability when supplied from foreign data and required non-null proof before call. Generic Sec callable values and lambda environments are not automatically thin C function pointers. Function-pointer indirect calls preserve the exact resolved ABI signature and effect/ownership facts.

## § 20. Callback adapter scope

Support `c::callback(callable)` for *environment-free, non-capturing, reusable* `fn` callables whose parameters/results match the C callback contract. `mut fn`, `-> fn` and capturing closures are invalid direct candidates. Generate a **private** C ABI callback thunk with explicit foreign-facing and Sec-facing call plans; do not generalize this to user-controlled C exports. No global callback registry or implicit environment boxing.

## § 21. Callback thread attachment and effects

Foreign callback entry may require `ThreadAttachment` when entering Sec from an unattached foreign physical thread, as defined by the selected runtime. Reuse an existing attachment; detach only one created by the wrapper. User TLS factories are not run merely because of attachment. Preserve callback call-graph, FFI trust, no-reentry/effect/ISR facts; pending MD-045 external metadata must not be fabricated. Attachment runtime mechanics can be phase-gated until corresponding runtime package exists.

## § 22. C variadics and promotions

Only `unsafe extern "C"`/compatible `c::fn` may have a final bare `...` under the FFI rule. Require already-C-typed extra arguments; the frontend resolves C default promotions, then `ABIModel.ClassifyVarArg` performs position-specific transport including register/stack/save-area rules. Native `...T` typed variadics are not C varargs. No Sec spread into C varargs. No untyped `null` vararg.

## § 23. Whole-call planning must include register pressure

Classify **return first**; derive required hidden result storage; classify formal arguments; consume already legal promoted varargs; assign *all* hidden and explicit arguments using whole-call resource state; finalize physical stack placement. A descriptor/classification alone is not a complete call-site plan. Resource limits and stack fallback must follow selected `ABIModel`, not generic codegen defaults. Split aggregate register+stack mixing is allowed only where the model permits it.

## § 24. Outgoing stack and alignment

Record stack offset, size, alignment, shadow/home requirements, register-save/variadic requirements and outgoing stack alignment when selected model requires. Use P24 exact checked layout arithmetic and model rules. Avoid unguarded `offset += sizeof(arg)`, host `size_t` overflows, or reclassification in LLVM backend.

## § 25. Hidden return storage

`sret`/indirect return is compiler ABI machinery. It is created after return classification, lives in ABI-generated storage, and maps to one semantic owned return when required. It does not alter the source result type or introduce a source-level `RawPtr`. Ensure the callee initializes returned storage once, caller takes one semantic returned owner, and P17 does not destroy a moved result. Return/storage alias assumptions are ABI-model-specific.

## § 26. Hidden compiler contexts and ordering

P19 introduced an internal hidden `!sec.alloc_context` for Sec functions that require an allocation context. Compiler-hidden environment, receiver, result-storage or context parameters must be reconciled in **one** `CallABIPlan` with deterministic ABI-model-selected ordering. P25 must not treat an internal context argument as a public source parameter nor inject it into an ordinary foreign ABI. Preserve source argument ordinal and debug provenance independently of physical ABI position.

## § 27. Ownership transfer is not ABI bit transport

Consume P17 transfer plans per parameter/return (copy, move, borrow, construct-direct) before ABI marshalling. Bit copies, ABI coercion, spills, indirect temporaries, register splits and stack homes never create a new Sec semantic copy or owner. Generated copies of representation bytes may only occur under the existing ownership-preserving transport plan. For nontrivial owning values passed indirectly, cleanup responsibility moves only when the source call semantics consume it.

## § 28. Call evaluation order

Preserve the source call's resolved receiver/argument evaluation order and exactly-once effects. ABI assignment may physically reorder/register-classify already-evaluated values; it may not reorder source evaluation, `try`, destructor/cleanup registration, or hidden context creation in a way that changes program semantics. CallABIPlan records semantic-to-physical mapping.

## § 29. Error and panic semantics at foreign boundary

Raw extern calls return raw foreign codes/pointers; ABI lowering does not construct `Result`/`Option` or read errno automatically. A Sec wrapper does that explicitly. No non-local transfer/unwind may cross an active foreign boundary in Sec 0.1, including foreign exceptions/longjmp over Sec frames or Sec unwinding through callbacks. Enforce required foreign shim/error protocol; `unsafe` is not an escape hatch. Preserve P17's ordinary cleanup and platform-specific panic policy as an **independent** concern.

## § 30. Foreign ownership/retention

Raw foreign pointer/scalar handle transport does not transfer Sec ownership. Owned foreign resources use nominal Sec wrappers with explicit success/failure transfer protocol; fallible foreign close is separate from mandatory `free`. Foreign retained addresses require a declared, verified contract, not inferred from pointer shape. MD-045 metadata producer remains pending: conservatively classify unknown effects/retention/reentry and reject positive proofs depending on unavailable contracts.

## § 31. Foreign symbol and link identity

Keep the named foreign symbol/link dependency and library-resolution provenance through the ABI pass, but do not decide linker name mangling or export policy. `rules/compiler/linking.md` owns symbol identities and link roots. Private callback thunks get canonical, collision-resistant internal symbol identities tied to concrete ABI contracts. General C-export syntax is not added in Sec 0.1.

## § 32. Canonical ABIModel API and cache

Recommended API shape (adapt names to actual compiler):

```go
type ABIModelRef struct {
    Identity    ABIModelID
    Revision    ABIRevision
    PlanKey     CompilationPlanABIKey
}

type ABIClassificationRequest struct {
    Type       TypeID
    Layout     LayoutID
    Position   ABIPositionKind
    Convention CallingConventionID
    ABIModel   ABIModelRef
}
```

Cache only by complete canonical identity, including ABI position, concrete monomorphized type, P24 layout/repr policy, ABI model+schema, relevant CompilationPlan details and foreign-legal contract. A cache is not canonical truth; invalid/stale cache falls back to cold classification.

## § 33. ABIArgInfo / ABIReturnInfo

Define immutable classification records with:

```text
PassingKind { Direct, Indirect, Ignore }
ABIValueParts[]
TransportExtension
RequiredAlignment
CoercionPlan
IndirectStorageRole
TransportAttributes
SourcePositionIdentity
ClassificationProvenance
```

Use a distinct return result or position parameter to encode hidden return requirements. Avoid invented LLVM types at this stage. No ABI classifier may mutate Sema ownership state.

## § 34. Resolved CallABIPlan

Recommended plan facts:

```text
CallABIPlanID
SemanticCallableID
ConcreteInstantiationID
ABIModelID / ABISchemaID
CallingConvention
ReturnClassification
ParameterClassifications[]
ForeignVariadicClassifications[]
HiddenABIParameters[]
AssignmentResources[]
OutgoingStackRequirements
SemanticToPhysicalCorrespondence[]
ABISignatureRef
ABIFingerprint
```

Stable IDs are compilation-snapshot identities. The plan is immutable and read-only to every lowering/verifier stage.

## § 35. CallABIPlan build algorithm

Implement a single deterministic pipeline:

```text
resolve concrete callable and selected ABIModel
validate FFI legality and all required physical layouts
classify return and materialize any hidden return parameter
classify parameters in semantic order
classify already-promoted C varargs in vararg positions
collect compiler-hidden internal Sec operands/roles
assign all ABI parts using model-specific resource state
finalize stack alignment/home/shadow/save areas
construct ABI signature and stable fingerprint
verify call and callee contracts against the same plan
publish immutable CallABIPlan
```

Do not perform source semantic checks during the ABI part-assignment stage.

## § 36. Canonical ABISignature and fingerprint

Serialize canonical physical ABI meaning in deterministic order; include ABI model/schema, return/parameter part classifications, hidden roles+order, variadic state, stack requirements, relevant P24 representation identity and ABI calling convention. Exclude source locations and display names. Fingerprint equality may accelerate compatibility checks but must not substitute for structural equality where hash collision/metadata errors are possible.

## § 37. Separate compilation and incremental cache

Expose `ABISignature`, `ABIFingerprint`, CompilationPlan ABI identity, semantic callable identity and producer/trust provenance to persisted binary metadata. A stale ABI artifact must rebuild or reject even if linker symbol names match. Cache invalidation is dependency-based: layout, ABI model/schema, calling convention, hidden params, return transport, foreign binding contract, target and profile changes invalidate only dependent artifacts. Compare warm incremental and cold compilation.

## § 38. Semantic IR amendment boundary

Semantic IR retains semantic callable type and source argument/result types. Add read-only references to `ResolvedABIIntent`, foreign legality/ABI origin, call-site identity, concrete monomorph identity, P24 `ResolvedLayoutRef`, return/argument ownership actions and optional approved callback/variadic metadata. **Do not rewrite** source function type into the physical call type inside Semantic IR. P25 materializes physical ABI only after semantic verification.

## § 39. Schema v21 staging

Planned Sec MLIR schema **v21** follows P24 planned v20. Before ABI lowering, high-level `sec.call`/function declarations retain semantic shapes (existing operations may be reused). After ABI planning, represent **ABI-specific** transport with `sec.abi.*` ops and typed plan attributes; do not modify `sec.func` semantic types in place without an explicit bridge. The schema must print both semantic and physical plan correspondence for diagnostics/debug.

## § 40. Planned ABI operations

Proposed ops (exact TableGen spelling may adapt to existing dialect conventions):

```text
sec.abi.pack_arg          (semantic value -> one/more verified ABI parts)
sec.abi.unpack_arg        (ABI parts -> semantic callee value)
sec.abi.prepare_indirect  (ABI temporary, transport-only)
sec.abi.prepare_return    (hidden result storage)
sec.abi.pack_return       (semantic result -> verified ABI parts/storage)
sec.abi.unpack_return     (ABI parts/storage -> semantic result)
sec.abi.call              (ABI-lowered direct/indirect/foreign call)
sec.abi.func              (ABI-lowered entry/wrapper contract)
sec.abi.callback_bridge   (private callback thunk linkage contract)
sec.abi.vararg_promote    (consume already-resolved promotion plan; no inference)
```

Do not invent a runtime source-level ABI value type. Multiple physical parts may lower through standard MLIR arity and typed attrs. No blanket `llvm.call` creation in P25.

## § 41. Typed metadata and no invented free-form semantics

Typed attrs/reference identities should cover:

```text
#sec.abi.model<...>
#sec.abi.signature<...>
#sec.abi.classification<...>
#sec.abi.call_plan<...>
#sec.abi.fingerprint<...>
sec.abi.semantic_decl_id
sec.abi.source_ordinal
sec.abi.part_ordinal
sec.abi.hidden_role
sec.abi.convention
sec.abi.foreign_legality_proof
sec.abi.layout_ref
sec.abi.source_transfer_action
```

These are proposals for **representation syntax**, not new source-language constructs. Prefer existing typed attrs if implemented. Every plan ref must resolve in the same frozen CompilationPlan snapshot.

## § 42. ABI lowering pass

Register a phase/pipeline equivalent to:

```text
--sec-resolve-call-abi
--sec-lower-call-abi
```

Stage 1 obtains canonical `CallABIPlan`s; stage 2 materializes transport ops around entries, returns and calls, preserving source semantics. Calls requiring a physical representation not yet supported by selected P24 policy must fail closed or remain explicitly deferred; never choose pointer+length, bool coercion or aggregate splitting ad hoc.

## § 43. Verifier: ABI plans

Register `--sec-verify-call-abi-plans`. Verify unique resolved ABIModel for the plan, concrete monomorph type, position-aware classification, P24 layout consistency, hidden arg ordering, full resource assignment, supported vararg promotions, stack offset/alignment, canonical ABISignature/fingerprint, and failure on unknown/unsupported target plan. Forbid C/`system` equivalence without explicit selected-model proof.

## § 44. Verifier: ABI-lowered MLIR

Register `--sec-verify-abi-lowering`. Check `sec.abi.call` and ABI-lowered function are from the same plan; parameter part counts/types/order; return transport including sret; indirect storage roles and alignment; hidden context/env/result roles; foreign fixed/variadic distinction; physical stack constraints and required ABI attrs; no semantic ownership creation, no lost valid-reference metadata, no pre-guard raw pointer fabrication.

## § 45. Verifier: foreign boundaries

Register `--sec-verify-ffi-boundaries`. Recheck **previously resolved** FFI legality/provenance, normal versus unsafe extern distinction, C namespace/type identity, unsafe-only null semantics, call-bounded borrow adaptation, no raw extern safe-reference returns, C function pointer non-null proof, callback restrictions, no C export, vararg legality/promotions, no implicit string/descriptor adaptation, no unwind and foreign effect trust provenance. This is not a second source checker; malformed imported/stale MLIR must be rejected.

## § 46. Verifier: fingerprint and persisted compatibility

Register `--sec-verify-abi-compatibility`. Compare caller/callee ABISignature structurally within one build, check imported/frozen metadata and fingerprint/schema/CompilationPlan identity, validate callbacks' two-sided plans, fail closed on unknown foreign ABI metadata. Internal inconsistency from same declaration is a compiler invariant failure with exact plan evidence; stale binary metadata is a deterministic compatibility diagnostic.

## § 47. ABI-lowering legality for safe refs

For native Sec call: keep complete P15 physical safe-reference representation selected by P24. For approved foreign `ref` parameter: use explicit call-bounded address adaptation, with legality proof, lifetime limited to call, no retained foreign address and required alignment/representation. Do not let an ABI `bitcast` silently erase validity metadata for native paths or reconstruct safe `ref` from a foreign return.

## § 48. Callback thunk lowering

Generate exactly one private bridge per concrete `(Sec callable identity, target C callback ABISignature, CompilationPlan, adapter revision)` as applicable; verify no captures and exact foreign-facing signature. The entry decodes foreign parts, establishes required verified Sec execution attachment/context, invokes native Sec callable under its own ABI plan, encodes approved result, closes only newly opened attachment. No unwinding across bridge; fallible callback must already define foreign status contract. Avoid user-visible exported names.

## § 49. No direct foreign null/Option/Result rewriting

`null` exists only in the declared unsafe foreign/raw pointer context. ABI lowering transports raw null representation where legal; no automatic `Option` wrapping. `== null` / `!= null` do not become supported; canonical tests use `is null`/`is not null` in unsafe contexts only. Foreign sentinel return normalization is explicit Sec wrapper code and may have allocation/effects. Do not equate C NUL byte terminator with null pointer.

## § 50. Debug/tooling correspondence

For each source parameter/result, retain semantic name/type and map to zero/one/many physical ABI parts with part ordinal, offset/register class/stack slot and hidden purpose. LSP/diagnostics should display source signature, not `sret`/environment/alloc-context plumbing as source params. Debug machinery consumes the map, doesn't recompute ABI. A native function pointer and C pointer must remain distinguishable in debug type identity.

## § 51. Target matrix and cross compilation

Test at least 32-/64-bit targets and ABI families with materially different rules, where registered in the chosen platform snapshot. Include C vs system same and different, aggregate multi-part/indirect, varying `c::long`, target `long_double`, `int128`/`int256` accepted/rejected positions, stack fallback, alignment, float target width. All expected facts must come from target `CompilationPlan`; do not run host C sizeof/offsetof or architecture-specific C code to decide cross-target classification.

## § 52. Known decision gates — no new P25-D file

On the public 2026-10-15 `missing-decisions.yaml` snapshot: MD-021 blocks source/metadata **producer and selection** for target bindings; MD-019 blocks explicit layout source syntax; MD-045 blocks optional external stack/no-reentry metadata source; MD-052 may constrain future privileged low-level bodies. None blocks **intrinsic ABI classification** for approved concrete types under a valid ABIModel. Do not create duplicate P25-D1 for any of those. The user's local `missing-decisions.yaml` may be newer; reconcile before deciding another source-language fork.

## § 53. Phase 0 — inventory and proof of predecessor readiness

Codex tasks, one at a time, test after each:

1. Save preflight HEAD/dirty status and rulebook versions.
2. Inventory `internal/platform`, `internal/layout`, `internal/sema`, `internal/ir/semantic`, `internal/lowering`, MLIR/TableGen and C tests using `rg`/`git ls-files`.
3. Identify existing `ABIModel`, c-binding IDs, call planning, string/closure transport, variadic handling, P24 layout hooks and legacy LLVM calls.
4. Create a subsystem integration table with `implemented`, `partial`, `planned`, `blocked`, `not checked`; no optimistic status.
5. Run baseline tests and record exact commands/outputs; do not claim P24 implementation absent proof.

## § 54. Phase 1 — model and position classification

Codex tasks:

1. Reuse/finish immutable `ABIModel` selection from CompilationPlan.
2. Introduce canonical positional classification records and errors.
3. Implement scalar/RawPtr and known C scalar position rules for *one* registered target ABI vertical slice first.
4. Add aggregate direct/multipart/indirect/ignore under exact P24 layout.
5. Add target matrix and negative incomplete/generic/unsupported tests.
6. Validate target-dependent cache keys and deterministic cold-vs-warm behavior.
7. Run Go/unit and TableGen verifier tests after each milestone.

## § 55. Phase 2 — whole-call resource assignment

Codex tasks:

1. Classify return first; emit hidden result role where ABI needs it.
2. Classify explicit params and legal C varargs.
3. Assign register/resource classes, stack fallback and mixed-part constraints exactly as selected model.
4. Reconcile P19 hidden allocation context and any environment/receiver ABI-hidden roles without modifying source signatures.
5. Build immutable `CallABIPlan`, ABISignature and deterministic fingerprint.
6. Test caller/callee equality, return-vs-param differences, many args, stack alignment and stable plan serialization.

## § 56. Phase 3 — high-level ABI boundary MLIR

Codex tasks:

1. Add planned schema v21 types/attrs and `sec.abi.*` operations without changing source op identity.
2. Add parsing/printing/round trips for plans and multi-part transport.
3. Implement `--sec-resolve-call-abi` and ABI marshalling for direct Sec call/function entry/return.
4. Verify source ownership/P17 lifetime/cleanup and P24 layout maps remain intact.
5. Implement indirect/sret paths and hidden contexts.
6. Run semantic-to-v21 and negative MLIR tests; do not go to LLVM in this phase.

## § 57. Phase 4 — foreign calls and callbacks

Codex tasks:

1. Lower imported `extern "C"` and `extern "system"` for existing legal C types; preserve trust/unsafe extern distinction.
2. Handle foreign C structs/unions/enums, incomplete/bitfields/flexible arrays, call-bounded refs, raw pointers where supported.
3. Apply prevalidated C default promotions; assign vararg resources.
4. Lower C function pointer calls and approved `c::callback` private bridges.
5. Verify no implicit source conversion, no unwind, no general exports, no hidden context on foreign ABI.
6. Mark target-binding metadata serialization as MD-021-gated rather than inventing it.

## § 58. Phase 5 — persistence, debug and integration

Codex tasks:

1. Add canonical ABI metadata/fingerprint to persisted call boundaries and debug source-to-part correspondence.
2. Reject stale/mismatched dependent artifacts; maintain cold compile fallback.
3. Integrate all four ABI/FFI/compatibility verifiers and target smoke tests.
4. Test direct/indirect/foreign callback path consistency and legacy backend regressions.
5. Update existing governance IDs after comparing actual existing records; do not duplicate historical YAML.
6. Produce required implementation report; leave unready P15–P24-dependent cases explicitly blocked.

## § 59. Canonical positive tests

Include test suites with named fixtures for:

```text
Sec call simple scalar + return
Sec multi-part aggregate direct parameter/return
Sec aggregate indirect parameter and hidden return
zero-sized Ignore only where target permits
named nominal representation equal but semantic types distinct
native string/dynamic-array/slice/Result with selected Sec ABI
native ref carries all selected metadata
C/system call matching active ABI
foreign RawPtr, c:: scalar and C struct
C union without hidden Sec discriminant
incomplete C type only by pointer/ref
C bitfields/flexible-array fixed header
C vararg default promotions and stack rules
C function pointer call
callback private thunk (no capture)
callback entry attachment when supported
implicit alloc-context hidden only in Sec call
move-only argument/return transfers exactly once
```

Every test asserts both canonical CallABIPlan and ABI-lowered IR/ABI fingerprint.

## § 60. Required negative tests

At minimum:

```text
unknown calling convention
C and system treated same without target proof
position-blind return/param classification
unsupported wide foreign scalar
host c::int / c::long sizing used under cross-target plan
incomplete C struct by-value
native string/slice/Result passed as foreign raw type
ordinary Sec safe ref collapsed to one RawPtr
raw extern returning ref T
foreign call retention from unproven call-bounded ref
C vararg with Sec scalar lacking explicit c:: conversion
C vararg using ref/ref mut or untyped null
ordinary extern incorrectly forced into unsafe
unsafe extern allowed without unsafe context
capturing / mut / once callable adapted to c::callback
user export added as side effect of c::callback
foreign exception/longjmp across Sec frame
foreign function gains P19 hidden allocation-context parameter
ABI caller/callee part count/ordering mismatch
invalid sret ownership
stale ABI fingerprint accepted
unknown target model guessed from host
```

Diagnostics must name source declaration, exact argument/return/vararg position, target/ABI and rule owner.

## § 61. Metamorphic, fuzz and cross-backend tests

Property/fuzz tests: random legal aggregate shapes under canonical LayoutID; compare direct classification to independently recomputed cold model; mutated ABI schema/target/hidden params/calling conventions; permutation of source traversal and map order; repeated parallel compilation; varied caller vs callee acquisition order. Cross-check with a compiled C fixture on *the intended target only*, when runner/toolchain available; otherwise report test as not-run, not passed. For unsupported target/ABI combinations expect explicit early rejection. P24 layout and ABI must never diverge.

## § 62. Commands and reporting policy

Use commands actually present in the checkout; discover with CMake and `rg` rather than assuming. Typical commands include:

```bash
git rev-parse HEAD
git status --short
git ls-files rules/platform rules/mlir governance
rg -n 'ABIModel|CallABIPlan|ABISignature|extern|c::callback|vararg|sret' internal rules testdata
go test ./...
```

For MLIR, run the checkout's real `check-sec-mlir` target and dialect pass test runner, recording LLVM/MLIR exact version, commands and outputs. A command mentioned in this specification is not evidence it exists yet.

## § 63. Acceptance criteria

All must be checked truthfully:

```text
[ ] actual HEAD and dirty state recorded
[ ] P1–P24 integration state audited, no future package assumed implemented
[ ] rules/platform/abi.md and ffi.md current revisions respected
[ ] one authoritative ABIModel per concrete CompilationPlan, including family-specific classification
[ ] positional parameter/return/vararg classification
[ ] direct/indirect/ignore and ABI parts
[ ] P24 layout reused without duplicate layout engine
[ ] scalar, wide, C scalar and aggregate classifications
[ ] safe ref vs RawPtr preservation and foreign call-bounded adaptation
[ ] whole-call resource and stack assignment
[ ] hidden sret, receiver/env/context ordering
[ ] Source-to-ABI parameter correspondence
[ ] immutable CallABIPlan + ABISignature + ABIFingerprint
[ ] caller/callee structural consistency
[ ] ABI caches plan/schema-sensitive and cold fallback
[ ] schema-v21 sec.abi.* operations parse/print/verify
[ ] --sec-resolve-call-abi and --sec-lower-call-abi registered
[ ] four ABI/FFI/compatibility verifiers registered
[ ] native Sec call and return lower through ABI plan
[ ] foreign C/system calls and C varargs as supported
[ ] c::fn and private c::callback bridges as supported
[ ] no general Sec-to-C exports added
[ ] ordinary vs unsafe extern semantics correct
[ ] no unwind across FFI
[ ] P17 ownership, P19 context, P24 validity preserved
[ ] MD-019/MD-021/MD-045 recognized as existing gates
[ ] no new normative fork silently invented
[ ] prior schema v20 regressions green where executable
[ ] go test ./... evidence recorded
[ ] check-sec-mlir evidence recorded
[ ] target matrix / C interoperability test evidence recorded
[ ] actual governance entries updated without duplication
[ ] blocked/partial/not-run cases explicitly named
```

Do not claim P25 complete merely because the classifier exists while call marshalling, verifier or selected target profile remains absent.

## § 64. Required Codex implementation report

Report with explicit evidence: (1) HEAD/dirty status; (2) actual P15–P24 dependencies; (3) rulebook SHA/revisions; (4) existing ABI/platform/layout/FFI inventory; (5) files changed; (6) chosen canonical model ID, concrete plans; (7) scalar and C scalar rules; (8) aggregate direct/multipart/indirect tests; (9) whole-call resource assignment; (10) stack/hidden/sret; (11) P17/P19 semantics; (12) CallABIPlan schema; (13) fingerprint/caching; (14) Semantic IR preservation; (15) schema-v21 ops; (16) dialect verifiers; (17) pass pipeline; (18) foreign calls; (19) `c::` binding environment gate; (20) varargs; (21) `c::fn`; (22) callback thunk; (23) no unwind; (24) cross-target matrix; (25) debug/source mapping; (26) separate compilation/cold cache; (27) legacy parity; (28) exact commands, versions, test output; (29) failures/not-run; (30) governance edits; (31) MD decisions linked; (32) recommendation for P26. This is an implementation report template, not proof the work has been done.

## § 65. Next package boundary

Proposed **P26: Validated Physical Reference and Descriptor Lowering** (confirm after actual P15–P25 implementation audit). It should lower the selected safe-reference/generation and descriptor policies through P24 and P25 into standard MLIR without erasing ownership/validity. Alternatively, if P25 reveals an unimplemented target ABIModel or runtime dependency, prioritize that concrete gap. Avoid promising LLVM-convergence until the actual ABI and layout passes are executable.
