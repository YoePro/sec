# MD-095 — Canonical atomic capability requirements

| Document metadata | Value |
|---|---|
| Document ID | MD-095 |
| Document revision | 1.0 |
| Date | 2026-11-24 |
| Status | Owner-approved design; normative correction pending integration |
| Language version | Sec 0.1; applicable to Sec 0.1.1 |
| Classification | Blocking — atomic capability requirement keys |
| Repository baseline | `3ed6048c0b9e25b28030a49c1f7cbe62421cd5e1` |
| Primary normative owner | `rules/concurrency/atomics.md` |
| Related owners | Platform model, target profiles, layout, Semantic IR and diagnostics |
| Governance | `concurrency.atomics-v2`, `frontend.atomics-v2`, `platform.atomics-v2`, `compiler.platform-model`, `semantic-ir.atomics-v2`, `lowering.atomics-v2`, `tooling.atomics-v2` |

## § 1. Scope and accepted decisions

**§ 1(1)** MD-095 resolves the canonical identity, execution context, layout and evidence requirements for target-dependent atomic operations. It introduces no new public syntax, atomic-compatible type category, operation, ordering, or runtime API.

**§ 1(2)** The owner has approved four principles:

1. Preserve the complete nominal Sec type; query physical capability using its resolved underlying carrier.
2. Use compiler-known execution-context categories, with additional requirements and target-specific restrictions where relevant.
3. Use the existing canonical target layout authority, including stronger alignment required by atomic storage.
4. Treat unknown capability as an unfulfilled proof obligation, not as permission or proof of impossibility. Analysis may continue; executable emission must fail closed unless the obligation is discharged.

**§ 1(3)** These principles are clarifications of Sec 0.1 and apply equally in Sec 0.1.1. They are consistent with the existing distinctions between language semantics, target evidence, immutable plans and incomplete layout. The correction replaces the unresolved alternatives in MD-095, not the remainder of the atomics rulebook.

**§ 1(4)** This document specifies the semantic interface between producers and consumers. Internal field names below describe logical data, not a required Go struct, serialization format, or public Sec API. No particular native instruction, emulation strategy, target support matrix or numbered MLIR package is approved by this correction.

## § 2. Separate source validity from platform capability

**§ 2(1)** Before querying concrete operation capability, Sema establishes the canonical `Atomic[T]` identity, concrete `T`, applicable method, value and receiver authority, nominal contracts, and legal memory orders. `atomics.md` §§ 8–9, 12–24 and 45 remain authoritative.

**§ 2(2)** The eligible categories remain `bool`, primitive signed and unsigned integers, `RawPtr[T]`, integer-backed enums, and named types whose underlying type and contracts permit the operation. Equal size, equal alignment or equal bit representation never makes an excluded type eligible. No new eligibility is granted to floating-point types, decimals, arbitrary structs or unions, or other currently excluded types.

**§ 2(3)** A legal type does not imply that every operation on it is available. The operation family, both memory orders where applicable, representation, storage alignment, execution context and selected profile must be validated. A target fact cannot legalize an otherwise invalid operation.

**§ 2(4)** An omitted ordinary object-operation order resolves to `SeqCst`. Explicit orders remain required semantic CTE contexts under § 24(4–6). Compare-exchange success and failure orders are preserved independently; no new relative-strength restriction is imposed on source orders. `atomic.Fence` requires its existing explicit order and no contained type.

**§ 2(5)** Language invalidity, unsupported target capability, incomplete target evidence, and unimplemented lowering are distinct. No error in one category is disguised as another.

## § 3. Canonical semantic identity and carrier identity

**§ 3(1)** `TypeID` is the complete canonical resolved identity of the source value type `T`, using the existing compiler type-identity service. It preserves its defining module, nominal declaration identity, kind and resolved generic arguments where applicable. Consumers must not invent another spelling convention from diagnostic names, import aliases, backend type spellings or host-language types.

**§ 3(2)** A separate `CarrierTypeID` identifies the underlying eligible primitive carrier of `T`. Resolve named-type layers recursively and resolve an integer-backed enum to its canonical backing integer type. Use the same canonical identity authority for the carrier as for source types. Keep the semantic type and its contracts independently available throughout analysis and lowering.

**§ 3(3)** Generic nominal instantiations retain their full semantic identity. A concrete specialization obtains its carrier only after all required substitutions and type resolution have completed. An unresolved generic type is not a concrete atomic capability request. Symbolic requirements may be retained until specialization; no arbitrary default carrier is permitted.

**§ 3(4)** `int` and `uint` retain their source primitive identities. Their physical width and storage facts are resolved by the selected plan. Any producer normalization of target-dependent integer carriers must be canonical and shared by all consumers; it may not use the compiler host's integer width.

**§ 3(5)** `RawPtr[T]` preserves the complete semantic pointer type, including its pointee identity. Its physical capability is queried using the resolved raw-pointer carrier representation, not by requiring a target definition to enumerate user-defined pointee types. Pointer representation distinctions that affect implementation must remain distinguishable. Do not infer pointer capability from an integer of equal width alone. This creates no new raw-pointer representation or address-domain feature.

**§ 3(6)** Capability lookup does not require the platform to enumerate arbitrary user-defined nominal types. For example, assuming the named contracts permit the operation:

```sec
type RequestCount uint64
type UserID uint64

let requests: Atomic[RequestCount] := Atomic(0)
let users: Atomic[UserID] := Atomic(0)
let count: Atomic[uint64] := Atomic(0)
```

All three source types remain distinct. Their corresponding physical operation requirements can use the same underlying `uint64` carrier and the same target capability when all other relevant requirements agree. There is no implicit conversion between the source types and no weakening of their nominal contracts.

**§ 3(7)** Carrier identity is not merely a byte count. The canonical physical representation facts are retained as well, including distinctions relevant to operation semantics. Sharing an implementation across integer carriers, `bool`, or other eligible representations is allowed only by an explicit validated producer rule that proves the full applicable operation. A lookup miss must not trigger an ad-hoc fallback to a similarly sized type.

**§ 3(8)** The former `AtomicRequirement.TypeID`-only exact-match contract must be revised consistently on producer and consumer sides. Semantic identity belongs to the source requirement and provenance; the physical capability key uses carrier identity and the remaining resolved physical/context requirements. An implementation may use a typed key rather than string concatenation. Legacy test strings such as `core::uint64` and `test::Count` are not normative identity spellings.

## § 4. Canonical execution context

**§ 4(1)** `ContextID` denotes the compiler-known execution category defined in `target_profiles.md` § 19:

```text
Ordinary
Task
Thread
Interrupt
Startup
Shutdown
```

These categories are not arbitrary project-defined strings. Do not use a task ID, thread ID, source function name or interrupt vector as a replacement category. `thread::main`, `isr::timer` and `test::ordinary` are not canonical categories.

**§ 4(2)** Determine the category from the existing execution-context analysis and the actual invocation contract. Do not classify logical tasks as ordinary threads merely because they use physical workers. Ordinary execution does not acquire a task identity by default.

**§ 4(3)** Additional requirements such as `NoBlocking` and `InterruptSafe` belong to the resolved invocation contract. They are requirements on the implementation, not claims that it has those properties. Existing effect, interrupt and profile rules determine when they apply. Interrupt execution must prove both nonblocking execution and interrupt safety; a claim of one is not evidence of the other.

**§ 4(4)** Platform-specific restrictions may refine the category using canonical target facts, such as a concrete interrupt vector, priority, masking domain, core or execution domain, when the selected target distinguishes those cases. Such facts restrict or qualify the proof; they do not create arbitrary new language contexts or enable a globally forbidden capability.

**§ 4(5)** A category-wide capability may discharge a concrete use only if its proof covers that use's additional restrictions. Where the category fact is insufficient, retain the unresolved obligation or reject a proven incompatible use. A validated refinement may select a more specific implementation without duplicating the entire capability table for every root.

**§ 4(6)** For a callable reachable from multiple execution contexts, check every applicable resolved invocation contract. A proof for an ordinary caller cannot discharge an ISR obligation. Unresolved context reachability is an outstanding analysis obligation, not permission to assume `Ordinary`. Existing transitive execution-context and effect analysis owns the determination of those contexts; this correction adds no alternative callable requirement syntax.

## § 5. Canonical storage layout and alignment

**§ 5(1)** The canonical layout service defined by `rules/memory/layout.md`, including § 47 (MD-141/MD-142), is the sole authority for concrete size, alignment, placement and stride. It consumes the selected frozen plan, including `ResolvedScalarPlan`, verified target layout and typed scalar alignment facts. Atomic analysis must not implement a second layout calculator.

**§ 5(2)** Distinguish the source carrier's size and natural alignment from the atomic object's effective storage alignment. The target may impose a stronger alignment for atomic access. That minimum is part of the canonical atomic storage layout, not an ordinary field exposed to the source program.

**§ 5(3)** The capability request's `SizeBytes` identifies the concrete atomic value access extent, not unrelated aggregate padding or the total size of a wrapper with private implementation storage. `AlignmentBytes` identifies the guaranteed alignment of the accessed atomic storage, after canonical placement and target requirements have been resolved. Capability producers and consumers must use the same meaning. A separate complete layout describes `Atomic[T]` itself, including any target-required private storage and placement constraints.

**§ 5(4)** Required atomic alignment is automatically provided for valid native storage. It applies to standalone values, statics, aggregate fields, arrays, and storage obtained through approved allocation mechanisms. It is not selected by the processor word size alone. An illustrative, non-normative target could require:

| Type | Access/value size | Required alignment |
|---|---:|---:|
| `uint32` | 4 bytes | 4 bytes |
| `uint64` | 8 bytes | 4 bytes |
| `Atomic[uint64]` | 8 bytes | 8 bytes |

The table makes no claim about any registered target. The complete atomic object's size is separately resolved.

**§ 5(5)** Atomic storage alignment is a verified target minimum under `layout.md` § 47.3(4). An enclosing `pack(N)` cannot reduce it. An explicit placement, offset, external storage contract or other layout constraint that cannot satisfy this minimum must fail under the applicable layout and capability rules. Do not silently use an unaligned atomic operation or change the programmer's explicit layout contract.

**§ 5(6)** A target producer's atomic storage-layout requirements must be resolvable before the final concrete operation capability query. In particular, do not infer an atomic object's alignment independently from whichever method is first used, or retroactively change a published layout after operation selection. A selected plan may describe stronger requirements or alternative implementations, but all used implementations must be compatible with the same canonical storage contract for that object.

**§ 5(7)** When a required size, alignment or representation fact is missing, the dependent layout is incomplete. Do not encode unknown size or alignment as a usable zero or guess a default from width, architecture name or host alignment. Retain the missing fact and its provenance. Contradictory facts are invalid; proven unsupported representations remain unsupported. Layout-sensitive executable lowering requires a complete layout.

**§ 5(8)** `atomic.Fence` has no value carrier, size or alignment requirement. The existing zero/empty internal sentinel representation for a fence is acceptable only as an explicit not-applicable case, never as a substitute for unknown object layout.

## § 6. Capability producer/consumer contract

**§ 6(1)** A resolved source operation retains at least the following logical information. This notation is descriptive, not a new Sec declaration:

```text
AtomicOperationRequirement
    SemanticTypeID             // Not applicable to Fence.
    CarrierTypeID              // Not applicable to Fence.
    ConcreteStorageRepresentation
    SizeBytes
    AlignmentBytes
    Operation
    Order
    FailureOrder               // CompareExchange only.
    ContextID
    NoBlocking
    InterruptSafe
    RelevantTargetContextFacts
    PlanIdentity
    SourceProvenance
```

The capability key is the carrier, representation, access size, guaranteed alignment, operation, requested orders, canonical context and applicable additional requirements, within the selected plan. Semantic type identity and source location are retained for validation and diagnostics, not used to demand separate target entries for equivalent user-defined carriers.

**§ 6(2)** Target producers may derive concrete facts from validated reusable representation/operation rules instead of listing every concrete key in handwritten target files. The derived result must establish the exact request, including the applicable context refinements. Consumers cannot derive unsupported or supported status from a partial resemblance to another key. Any matching, normalization or refinement is owned by the canonical plan capability service and must have deterministic, validated semantics.

**§ 6(3)** Supported facts retain implementation identity, provenance, reason where applicable, and the required implementation guarantees. The physical realization may be a native operation, compiler intrinsic, verified platform primitive or permitted emulation as already specified in `atomics.md` § 38. Blocking emulation is not implicitly authorized.

**§ 6(4)** Requested ordering may be implemented by an observably compatible stronger order only as allowed by `atomics.md` § 47. The complete proof must preserve the original requested order and both compare-exchange paths. Backend acceptance of an instruction, or existence of an equally sized scalar, is not by itself a proof of the source operation.

**§ 6(5)** The same validated query service is consumed by frontend, analyses, Semantic IR validation, lowering and tooling. Producers and consumers use the same identity and normalization authority. The selected plan remains immutable; no consumer mutates target facts to make a requirement pass.

## § 7. Supported, Unsupported and Unknown

**§ 7(1)** The availability states retain the meanings of `platform_model.md` § 14:

| Availability | Consequence |
|---|---|
| `Supported` | A verified implementation satisfies the concrete capability request, subject to all other language and profile requirements. |
| `Unsupported` | Canonical evidence proves the requested capability unavailable or forbidden. Report a focused target capability error. |
| `Unknown` | Evidence is insufficient. Retain an unfulfilled proof obligation; do not accept it as a verified operation or report proven impossibility. |

**§ 7(2)** Global capability availability and profile activation remain independent. A disabled capability cannot be enabled by program use or a more permissive context. A missing optional capability entry can legitimately be unknown in a frozen plan. This is distinct from constructing a plan with required unresolved internal state, which remains invalid under `platform_model.md` § 21.

**§ 7(3)** Sema may continue language checking, type resolution, and other analyses for semantically valid operations with outstanding target proofs. An LSP or analysis-only result may retain these obligations as unproven. It must not claim executable target validity. A complete build must discharge every required concrete proof before executable emission. If still unknown at that boundary, report insufficient capability evidence and stop.

**§ 7(4)** The same rule applies to interrupt and nonblocking contexts. The analysis may retain an unknown obligation, but an ISR operation is not verified legal for the selected plan without positive proof of its concrete interrupt safety and nonblocking behavior. This preserves `atomics.md` § 39(2); there is no deferred runtime safety check or provisional permission to emit unsafe code.

**§ 7(5)** Positive evidence may be established by later canonical target-dependent analyses using verified target information and lowering contracts. LLVM may contribute target facts through the canonical platform/layout authority. It may not replace the selected plan, silently consult the host, or infer nonblocking or interrupt-safe behavior merely because it can produce an instruction or library call. A change to the resolved plan's facts requires a newly verified plan/snapshot and revalidation of dependent proofs, not mutation of the frozen plan.

**§ 7(6)** A purportedly supported implementation with a missing or incompatible required progress or interrupt-safety property does not satisfy the request. The existing `AtomicContextViolation` validation remains mandatory. Preserve the distinction between an explicitly incompatible property and missing proof. Do not weaken the requirement to obtain a match.

**§ 7(7)** If a target capability is supported but the current compiler or MLIR package does not implement its lowering, report an implementation-capability limitation. It is not a source-language prohibition or evidence that the selected platform lacks the capability. Existing fail-closed emission rules remain in force.

## § 8. Semantic IR, diagnostics and verification boundaries

**§ 8(1)** Semantic IR preserves source type, concrete carrier, operation and orders, storage identity, complete or outstanding layout facts, context requirements, capability evidence and provenance. The existing `semantic_ir.md` §§ 123 and 123A own operation names and representation. Do not introduce a second opcode inventory or infer atomics from ordinary load/store patterns.

**§ 8(2)** Before executable lowering, verify that every relevant concrete operation has a complete layout, resolved context constraints and an applicable supported implementation under the selected plan. Later stages consume the validated evidence and preserve alignment, no-tearing behavior, memory order, modification order and exact compare-exchange outcomes. No silent fallback to ordinary, volatile, weaker-order or unverified blocking operations is allowed.

**§ 8(3)** Diagnostics identify, when available, the selected target/profile, semantic type, carrier, operation and orders, concrete layout, required context properties, source location and relevant proof or missing fact. Distinguish these categories:

```text
MemoryOrder.Release is not valid for Atomic.Load.
Selected target/profile does not support Atomic[RequestCount].CompareExchange
with the requested success and failure orders.
Atomic[RequestCount].Load cannot be verified: the selected target has no
capability fact for the required carrier and interrupt context.
Atomic[RequestCount] has incomplete layout: the required atomic alignment
is not resolved for the selected target.
Atomic operation is supported by the selected target but its lowering is
not implemented by the active compiler/dialect package.
```

These are illustrative English messages, not allocated diagnostic IDs. Reuse existing identities only where their registered contract matches; allocate new identities through the diagnostics registry when necessary.

**§ 8(4)** Analysis-only reporting follows the canonical diagnostics and analysis-policy rules. No advisory or LSP presentation policy makes an outstanding required proof sufficient for executable lowering. A proven source error remains mandatory regardless of analysis level. An unresolved proof is not reported as a proven unsupported capability.

**§ 8(5)** Capability and layout caches must be bound to the selected plan and their canonical dependency fingerprints, including relevant carrier/layout/context normalization and proof schema revisions. Changed facts, targets, ABI, profile, or applicable implementation guarantees invalidate dependent proofs. Host facts are never cache substitutes. Existing exact-match snapshots must not be reused across incompatible key schemas.

## § 9. Integration instructions

**§ 9(1)** Integrate the approved semantics into existing numbered sections or append stable lettered subsections. Do not renumber existing paragraphs referenced by implementation or governance. Update document revisions, dates and correction provenance according to each book's established header conventions.

| File | Required integration |
|---|---|
| `rules/concurrency/atomics.md` | §§ 7–8, 37–39, 45–48: carrier distinction, canonical requirements, layout, context, evidence and diagnostics. Preserve order matrix and public API. |
| `rules/platform/target_profiles.md` | §§ 19, 29–32: category identity, refinements, proof status and common consumer behavior. |
| `rules/platform/platform_model.md` | §§ 9, 14, 21: validated capability producers, immutable plan and unknown optional evidence versus unresolved required plan facts. |
| `rules/memory/layout.md` | §§ 8, 28, 47: atomic storage layout uses canonical service; verified atomic minimum alignment cannot be capped. |
| `rules/compiler/semantic_ir.md` | §§ 123, 123A: preserve required operation evidence or unresolved obligation without changing existing opcodes or assigning a new MLIR package. |
| `rules/tooling/diagnostics.md` | Align registered diagnostics and transport with the distinctions in § 8; no invented numeric IDs. |
| `missing-decisions.yaml` | Record the accepted MD-095 resolution and correction provenance when integrated, using the existing ledger schema and status conventions. |

**§ 9(2)** Migrate `internal/layout/atomic_capabilities.go`, its producers, consumers and tests together. Existing exact-match lookup and `atomic_context.go` validation are useful foundations, but the old semantic-type-only key and arbitrary string contexts must not remain alternate authoritative paths. Use the canonical type and layout services rather than embedding duplicate normalization in Sema. Preserve immutable snapshot validation.

**§ 9(3)** Establish target capability and storage-layout facts through verified producer data or derivation rules. The absence of registered positive facts is not a reason to insert fabricated default support. The correction makes no claim that any particular registered platform now implements a concrete operation.

**§ 9(4)** Synchronize the relevant existing governance fragments using `governance/index.yaml` and the current owner mappings. Distinguish a resolved normative decision from implemented frontend integration, actual target facts, verified layout, context analysis, diagnostics and executable lowering. Do not promote implementation status solely because this correction is accepted.

**§ 9(5)** Implementation should proceed one verified step at a time: canonical identity and key migration; layout integration; context requirement derivation; plan producer and lookup validation; Sema/IR evidence propagation; diagnostic and lowering gates; real-target verification. Test each step before advancing. Synthetic target fixtures test the model but do not establish support on real targets.

## § 10. Conformance matrix

**§ 10(1)** Each capability fixture must state its target plan, resolved layout, applicable operation/order/context facts and implementation guarantees. Positive fixtures are synthetic unless verified against a registered target. Tests must distinguish a language error from a capability error and an unresolved proof from an unsupported lowering.

| ID | Case | Expected outcome |
|---|---|---|
| T01 | `Atomic[uint64]`, legal operation, complete supported request | Accepted when other requirements hold. |
| T02 | Named `RequestCount uint64`, same physical request | Same carrier capability; distinct source type retained. |
| T03 | Two distinct named types with the same carrier | No implicit conversion or nominal identity collapse. |
| T04 | Multiple named-type layers | Resolve canonical underlying carrier; retain outer identity. |
| T05 | Integer-backed enum, legal load | Backing integer capability; preserve enum value validity. |
| T06 | Enum `FetchAdd` | Source operation error regardless of target evidence. |
| T07 | Named scalar contract forbids requested operation | Source semantic error; target support cannot legalize it. |
| T08 | Generic named instantiations with equivalent carrier | Distinct semantic instantiations; reusable validated capability. |
| T09 | Unresolved generic parameter | Symbolic requirement; no fabricated concrete carrier. |
| T10 | `RawPtr` to two different pointee types, same proven representation | Preserve pointer types; share capability only under identical applicable physical requirements. |
| T11 | Raw pointer and integer of equal size | No automatic cross-carrier support inference. |
| T12 | `bool` carrier and integer with equal size | Preserve type semantics; require validated producer coverage. |
| T13 | Same-sized ordinary struct or float | Source type error even if hardware supports that width. |
| T14 | Target-dependent `uint` on two distinct plans | Plan-derived width/layout; no host dependency. |
| T15 | `Load(Release)` with positive target fixture | Source order error. |
| T16 | CompareExchange with `Release` failure order | Source order error. |
| T17 | Legal independently selected compare-exchange orders | Retain both; no extra relative-strength source restriction. |
| T18 | Omitted object-operation order | Query `SeqCst`. |
| T19 | Runtime-dependent explicit order | Required CTE error, not a capability lookup miss. |
| T20 | Fence with valid explicit order | Query without carrier/layout fields. |
| T21 | Fence with `Relaxed` or omitted order | Existing source error. |
| T22 | Canonical `Ordinary`, `Task`, `Thread`, `Interrupt`, `Startup`, `Shutdown` | Correct distinct context categories. |
| T23 | Arbitrary string-named context in producer key | Reject noncanonical producer input. |
| T24 | Task executes on physical worker | Do not substitute `Thread` category merely from worker identity. |
| T25 | Same helper reachable from ordinary and ISR roots | Both invocation requirements validated. |
| T26 | Category-wide interrupt proof covers concrete vector | Applicable supported evidence. |
| T27 | Vector restriction excludes category's implementation | Capability failure for that use; no category-only bypass. |
| T28 | Refinement facts missing | Unfulfilled proof where necessary; do not assume permissive case. |
| T29 | Nonblocking proof absent for `NoBlocking` request | Supported claim cannot discharge requirement. |
| T30 | ISR-safe property yes, blocking property yes | Reject implementation proof. |
| T31 | Nonblocking yes, interrupt safety unknown in ISR | No verified ISR operation. |
| T32 | Native implementation with unknown required property | No automatic exemption. |
| T33 | Local interrupt masking without cross-core proof where needed | Insufficient evidence for requested atomic contract. |
| T34 | Enabled global capability restricted by profile context | Restriction enforced. |
| T35 | Disabled global capability with permissive context | Cannot enable by context or source requirement. |
| T36 | 32-bit target with verified 64-bit atomic support | Do not reject by CPU width alone. |
| T37 | Equal-width targets with different capability facts | Independent results. |
| T38 | Load supported, CompareExchange unsupported | Operation-specific results, not blanket width support. |
| T39 | Natural alignment 4, atomic minimum 8 | Canonical object/placement alignment satisfies 8. |
| T40 | Atomic field in ordinary struct | Field offset and aggregate alignment respect required minimum. |
| T41 | Atomic field in packed struct | Packing cannot lower target-mandated alignment. |
| T42 | Explicit incompatible field offset | Reject invalid placement rather than silently changing offset. |
| T43 | Array of atomic values | Every element's stride and placement satisfy atomic minimum. |
| T44 | Missing scalar or atomic layout fact | Incomplete layout; no guessed width/alignment. |
| T45 | Object request with zero size or alignment used for unknown | Not a valid concrete key. |
| T46 | Complete object layout contains private padding/storage | Query access extent, not arbitrary wrapper size. |
| T47 | Multiple methods on the same object | One compatible canonical storage contract. |
| T48 | Known supported complete request | Applicable proof retained. |
| T49 | Known unsupported request | Focused target capability error. |
| T50 | Missing optional capability entry | Unknown, not supported or proven unsupported. |
| T51 | Unknown operation in analysis-only mode | Language analysis may continue; retain unfulfilled obligation. |
| T52 | Unknown operation at executable emission | Fail closed with missing-evidence diagnostic. |
| T53 | Unknown ISR operation during analysis | No claim of verified ISR legality. |
| T54 | Unknown ISR operation at emission | Fail closed; no interrupt-safety bypass. |
| T55 | Later canonical proof discharges unknown obligation | Emission permitted only after all remaining checks pass. |
| T56 | LLVM can emit a library call but lacks required nonblocking proof | Insufficient evidence for restricted context. |
| T57 | Stronger implementation order proven compatible | Allowed under existing § 47; preserve source order metadata. |
| T58 | Weaker-order or volatile fallback | Rejected. |
| T59 | Supported operation with absent MLIR lowering | Explicit implementation limitation; no executable emission. |
| T60 | Changed target/profile/layout fingerprint | Invalidate incompatible cached proofs. |
| T61 | Legacy semantic-type-only table reused under new key schema | Reject incompatible snapshot or migrate and revalidate. |
| T62 | Required unresolved internal plan fact | Invalid frozen plan; distinguish from optional unknown capability. |
| T63 | Failure of compare-exchange | Return actual observed value under preserved ordering; no later substitute load. |
| T64 | Invalid source operation and missing target facts | Source invalidity is not hidden behind unknown capability. |

## § 11. Resolution and remaining implementation work

**§ 11(1)** MD-095 is resolved at the design level by the accepted four decisions. The implementation is not completed by this document.

**§ 11(2)** The integration details in §§ 3–8 make the accepted decisions usable at compiler boundaries: separate access extent from complete object layout, resolve storage minimums before capability selection, preserve multiple context obligations, and prevent plan mutation or schema-incompatible cache reuse. They are derived constraints, not additional public language features. Exact internal data structures, producer encoding and diagnostic IDs remain implementation choices under the existing authorities.

**§ 11(3)** Registered target facts, real-target evidence, remaining MLIR package work and compiler implementation status must be verified independently. This correction does not close MD-096 or any separate lowering decision. Do not mark unsupported implementation paths as implemented merely because MD-095 no longer blocks the design.

**§ 11(4)** Baseline source evidence: `atomics.md` §§ 7–9, 24–26, 37–40, 45–48; `target_profiles.md` §§ 19, 29–33; `platform_model.md` §§ 14, 21; `layout.md` § 47; `semantic_ir.md` §§ 123, 123A; `internal/layout/atomic_capabilities.go`, `atomic_context.go`, `scalar_plan.go`; MD-095 in `missing-decisions.yaml`. Repository baseline checked against `main` on 2026-11-24. No repository files are modified by delivery of this correction.
