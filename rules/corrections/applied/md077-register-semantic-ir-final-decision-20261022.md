# MD-077 — Register values, hardware implementations, Semantic IR, and MLIR

- **Status:** Normatively integrated 2026-10-22; implementation in progress per § 18 and tracked in governance (`frontend.registers`, `platform.hardware-register-access`, `frontend.fixed-address-bindings`, `lowering.sec-mlir-package15`). The register entry has been removed (§ 15(7)).
- **Created:** 2026-10-22
- **Last updated:** 2026-10-22
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical decision ID:** `MD-077`
- **Canonical destination on acceptance:** `rules/corrections/applied/` (without renumbering existing normative paragraphs)
- **Primary normative owners:** `rules/declarations/registers.md`; `rules/declarations/impl.md`; `rules/declarations/properties.md`; `rules/platform/hardware-register-access.md`; `rules/platform/fixed-address-bindings.md`; `rules/compiler/semantic_ir.md`
- **Related:** `rules/types/types.md`; `rules/types/contracts.md`; `rules/declarations/enums.md`; `rules/mlir/packages/sec-mlir-dialect_package15.md` (P15-42/42a); `rules/mlir/sec_mlir_dialect.md` and the matching versioned dialect specification
- **Decision tracking:** `missing-decisions.yaml`, MD-077
- **Replaces:** the unresolved MD-077 register/Place-IR design question; incorporates `MD-077-register-impl-storage-clarification-20261022.md` completely (do not apply that clarification separately)
- **Repository reference:** public `main` rulebook pages checked on 2026-10-22. The repository commit SHA could not be independently verified by the web source and must be recorded by the integrator before applying this file. No repository file has been modified by this correction.

> **Decision authority:** The user approved the language and register-access principles, then explicitly delegated to the assistant the remaining three design choices: implementation-handle typing, Semantic IR operation vocabulary, and MLIR schema encoding. All paragraphs below are part of that completed design decision. Adoption into canonical rulebooks and implementation status remains separately tracked.
---

## § 1. Purpose and scope

**§ 1(1).** A `register[N]` is a nominal fixed-width bit-layout type. It is not inherently a hardware resource, MMIO location, volatile value, `RawPtr`, or ordinary reference. Detached register values remain usable for protocol headers, network packets, file formats, snapshots and ordinary computation.

**§ 1(2).** Hardware binding associates a register-type implementation instance with a resource/endpoint contract. The physical register is distinct from both a detached register value and the ordinary software storage belonging to an implementation instance.

**§ 1(3).** Every already-defined access rule remains authoritative, including permissions; `read-clear`; `read-set`; `write-one-clear`, `write-one-set`, `write-one-toggle`; `write-zero-clear`, `write-zero-set`, `write-zero-toggle`; `shadow`/`noshadow`; reserved-bit preservation; aliases; physical footprints; ordering; completion; access context; and faults. This document neither weakens nor reinterprets them.

**§ 1(4).** The compiler owns the semantics and realization of individual compiler-known register operations. It shall not require the programmer to reproduce masks, read-modify-write, clear/set/toggle sequences or other already-declared field semantics in hand-written functions. No new public core functions for individual access modifiers are required by MD-077.

**§ 1(5).** An ordinary Sec `impl` is owned by a **type**, never by a physical endpoint or runtime instance. Different register kinds that require different type-level access implementations are distinct nominal register types, even where the bit layouts coincide. Different resource instances may use the same register type if the same complete type-level contract applies to both.

**§ 1(6).** This correction establishes the distinction between (a) register value operations, (b) implementation-instance software state, and (c) external hardware operations, and defines the minimum Semantic IR representation that makes those distinctions verifiable.

## § 2. Mandatory `impl` and controlled exposure

**§ 2(1).** Hardware-bound register types require a valid type-owned primary `impl`. A hardware binding whose register type lacks that `impl` is a compile-time error. A generated transaction plan must not silently stand in for the missing source-level `impl`.

**§ 2(2).** Normal user code accesses hardware through the properties and methods expressly exposed by the register type's `impl`. Physical register fields are not implicitly added to the public member surface merely because they appear in a register declaration. The internal compiler-known access primitives are available for resolution **inside the authorized implementation context**, not as a freely callable public function family.

**§ 2(3).** Inside the owning `impl`, ordinary-looking field syntax resolves to the declared logical register operation, subject to full access and target-plan validation. For example, a field annotated `write-one-clear` is an action to clear when written with its declared triggering bit pattern; it is **not** a generic memory assignment and shall not be lowered through an unverified RMW.

**§ 2(4).** A public property may include one getter and a suitable setter according to `properties.md`. A fallible getter can return `Result[T,E]`; no `try get` syntax is introduced. Properties and methods specify the exposed API, not alternative hardware semantics.

**§ 2(5).** The Sema-resolved member-access plan must distinguish `RegisterPhysicalField` from `ImplInstanceMember` and `PublicImplMember`. Accessibility is checked before constructing Semantic IR; IR must not regain private physical field access by re-resolving syntax.

**§ 2(6).** Legacy user-source expressions that directly address a hardware field outside the owning `impl` must receive a focused diagnostic with the permitted public property/method alternative, rather than being silently accepted through older `hardware-register-access.md` §6 examples. These examples and the earlier compiler-known `Read()`/`Write()` surface require an editorial pass: preserve their semantics, but route external calls through the controlled API.

## § 3. Representation: physical resource versus software instance

**§ 3(1).** Physical register bit width, bit ranges, byte order, addressing mechanism, resource identity and endpoint access do **not** include `impl` instance `let` / `let mut` members.

**§ 3(2).** An `impl` may possess distinct per-instance Sec-owned state. That state is regular software storage, located independently of the physical register, and follows Sec initialization, ownership, borrowing, availability and destruction rules. Its allocation policy must not change the register bit-layout type or silently initialize hardware.

**§ 3(3).** The existing statement in `impl.md` §7 that `impl`-declared instance fields contribute to the implemented type's complete storage representation requires a *hardware-bound register clarification*: they contribute to the **software implementation-instance representation**, not to the hardware's `register[N]` bit width or detached value layout. The `impl` remains type-owned; this is not permission to introduce instance-specific implementations.

**§ 3(4).** The canonical internal identities are distinct:

| Semantic identity | Purpose | May be an ordinary memory Place? |
|---|---|---|
| `RegisterTypeID` | Nominal register bit layout and field definitions | Its detached instances may be |
| `HardwareResourceID` | External resource/authority identity | No |
| `HardwareEndpointID` | Actual physical access mechanism/alias | No by default |
| `HardwareBindingID` | Connection between instance and external resource | No |
| `ImplInstanceID` | Specific implementation-instance identity | Its software state may be |
| `ImplStorageID` | Ordinary software storage of `impl` members/shadow state | Yes, if the storage contract permits |
| `RegisterFieldID` | Resolved field or nested field identity | Not a hardware pointer |

The names above designate **compiler-internal semantic concepts**, not new public Sec type names or guaranteed struct/class names.

**§ 3(5).** A hardware-bound instance must preserve its instance-to-resource association independently of the implementation's separate storage location. Copy, move or borrow must not manufacture extra resource authority. The compiler-known `HardwareInstance[T]` remains subject to Sec's normal ownership classification, extended with the binding's non-duplicable hardware-authority restrictions. It is not universally copyable. The concrete representation may be zero-sized when the resource is statically identified and no per-instance software state or reference-accessible identity storage is needed, but the distinct semantic identity must remain observable to analyses.

## § 4. Safe references and hardware endpoints

**§ 4(1).** `ref` and `ref mut` to the **physical hardware register or endpoint itself** are forbidden. Hardware endpoints cannot be promoted into ordinary safe memory Places merely because a binding is named or has a numeric address.

**§ 4(2).** Software-state members and the refererable implementation instance/handle remain eligible for ordinary Sec borrowing to the extent that their actual ordinary storage and authority permit it. A software reference never creates a `ref` to the external register's bits and never grants new register access permission.

**§ 4(3).** P15's addressed Place roots are retained for **genuine addressed memory storage** governed by the normal storage/reference rules. They are **not** to be fabricated for `HardwareResourceID` or `HardwareEndpointID` when the binding denotes an external register operation mechanism. A fixed numeric address is not proof of ordinary memory semantics.

**§ 4(4).** The compiler must reject IR trying to use `place.read`, `place.write`, `ref.borrow`, `ref.borrow-mut`, raw safe dereference or an ordinary address-space reinterpretation as an alternative route to a physical register operation. The right path is the explicit semantic hardware operation family (§ 9).

**§ 4(5).** For references to implementation-state objects, Semantic IR must preserve the software state origin (`ImplStorageID`) and the associated implementation-instance identity separately. A function using a reference to software state cannot assume that it thereby possesses unbounded device authority.

**§ 4(6).** The compiler-known, user-nameable type `HardwareInstance[T]` identifies the **software implementation instance** associated with nominal register type `T`. It is distinct from `T` and from all physical endpoints. It is supplied by the compiler; users may not declare a competing type or primary `impl` for it. `impl T` remains the sole owning implementation of the register type and supplies its public properties and methods to `HardwareInstance[T]` instances. This introduces no new grammar beyond ordinary generic type application.

**§ 4(7).** In the existing source form `@address(endpoint) let mut device: T`, the explicit `T` names the nominal hardware register layout **being bound**, while the declaration denotes a hardware implementation instance with resolved semantic type `HardwareInstance[T]`. This is a specific normative rule for the existing `@address` register-binding construct; it is not an implicit conversion of ordinary `T` values into hardware instances. The compiler and LSP shall report both the declared register type and the resolved instance type, rather than falsely presenting the binding as a detached `T`.

**§ 4(8).** `ref device` and `ref mut device`, where `device` denotes a hardware-bound implementation instance, have the types `ref HardwareInstance[T]` and `ref mut HardwareInstance[T]`, respectively, subject to ordinary binding mutability, lifetime and borrowing rules on the **software instance**. They never have types `ref T` or `ref mut T` and never grant direct access to physical register bits. Functions receiving these references may call the public API of `impl T` but cannot dereference them into a detached `T` or reborrow physical hardware storage.

**§ 4(9).** Existing memory-backed detached `T` instances continue to yield `ref T` and `ref mut T`. Neither `HardwareInstance[T]` nor its references implicitly coerce to `T`, `RawPtr[T]`, `ref T`, or `ref mut T`. The compiler rejects attempts to obtain such values by implicit conversion, dereference or Place reclassification. A deliberate hardware observation returning a detached `T` must use a permitted compiler-known operation through `impl T`.

**§ 4(10).** `HardwareInstance[T]` has no public constructors, no public stored fields of its own and no separate source-owned `impl`. It is created through a validated hardware binding. Its public callable/property member surface consists exclusively of the permitted members of `impl T`, checked under the actual hardware authority contract. Compiler-generated resource and shadow descriptors remain private representation, not ordinary user-visible fields. Any general compiler-known `.Ptr` access, where otherwise valid for a borrowable software object, addresses only its software handle Place, never the physical register endpoint.

**§ 4(11).** Members of `impl T` that require compiler-known hardware access have a hardware-instance receiver requirement and cannot be invoked on a detached `T`. Members that operate solely on detached values may be used with detached `T` as already permitted by their receiver contract. Receiver-kind eligibility is determined statically from the method body and Sema's validated effect contract; it does not select an alternative implementation of `impl T` according to individual instances or introduce runtime hardware dispatch.

**§ 4(12).** An implementation instance with no software fields still has nominal instance identity and an associated resource. A borrowable whole instance requires a valid logical software Place identity and a proven lifetime. The compiler may materialize storage only when required by the chosen representation and must preserve distinct identities for distinct instances; zero-size storage does not authorize collapsing their identities or creating a physical register address. No `ref void` is used.

## § 5. Detached register values

**§ 5(1).** For a detached `register[N]` value, ordinary register field extraction/update uses the declared bit layout and produces or consumes normal Sec values. No physical hardware transaction, volatile access, shadow observation or hardware side effect is implied.

**§ 5(2).** Detached value operations must retain `RegisterTypeID`, the field path (`RegisterFieldID` sequence), bit offset/width, nested-register layout and field value type. They can be optimized as pure value calculations subject to ordinary copy/move and target representation constraints.

**§ 5(3).** For a detached register value, a field update produces a new typed register value or updates an ordinary Place using the existing ownership/mutability rules. A bit mask in lower IR is merely a permissible lowering of an already validated *value* operation, not a reconstruction of source-level field meaning.

**§ 5(4).** A detached register snapshot returned from a hardware read remains detached. Reading its fields must not repeat the original device observation or its side effects.

## § 6. Multibit fields, ranges and enum conversions

**§ 6(1).** A field declared `bit[N]` is a field representation, **not** a standalone integer type. An ordinary field read uses the smallest standard unsigned integer carrier supported by Sec which can hold all `N` bits, together with an exact semantic range fact `0..(2^N - 1)`.

**§ 6(2).** For `bit` (one bit), the field value is `bool` according to the applicable field typing rule; `bit[N]` for `N >= 2` uses the carrier types below. The already-defined `bit[N](integerValue)` extraction, including for `N = 1`, continues to return the unsigned carrier specified in `types.md`, **not** `bool`. These are different constructs and their result types must not be conflated.

| Multibit field width | Read carrier | Maximum |
|---|---|---|
| `2..8` | `uint8` | `2^N - 1` |
| `9..16` | `uint16` | `2^N - 1` |
| `17..32` | `uint32` | `2^N - 1` |
| `33..64` | `uint64` | `2^N - 1` |
| `65..128` | `uint128` | `2^N - 1` |
| `129..256` | `uint256` | `2^N - 1` |

**§ 6(3).** The value range is a **Sema proof/refinement fact**; it is not a declaration of a fresh named `range` type on each field. `contracts.md` continues to own named range types. Field reads and ordinary range checks reuse one numeric domain/range analysis.

**§ 6(4).** A field backed by a bit-backed enum retains the enum's nominal type and its full bit-domain semantics. An unlisted bit pattern does not automatically invalidate an open bit-backed enum. Conversions to or from integer carriers remain explicit per the enum/type rules.

**§ 6(5).** A field write from a literal or typed value is checked for carrier type, units (if any), representable bit width and effective field value domain. A compile-time provably out-of-range constant is rejected. An already-typed runtime value whose range has not been established is not silently truncated to fit. It requires the existing checked-conversion/range-proof path or an explicit lossy extraction, as applicable to the type and source-width rules.

**§ 6(6).** Existing compiler-known `bit[N](value)` is the **deliberately lossy** low-bit operation: it produces an unsigned carrier and a proven interval; it is not implicitly performed by a field write. No new syntax, `uint4`/`uint12`-family, or second field-casting system is introduced.

**§ 6(7).** Semantic IR must carry value type, signedness, relevant unit fact, bit width, proven range, and the selected conversion/check/truncation operation **separately from** the physical register access plan. This same conversion contract applies to detached protocol values and hardware-exposed properties.

**§ 6(8).** Do not silently assume that an arbitrary checked cast to a field-width constrained value is expressible as a public `bit[N]` type constructor; no such standalone type exists. Implementation should use the already resolved Sema field-assignment plan and preserve any required failure edge explicitly.

## § 7. Access permissions and effects

**§ 7(1).** A field's permission and behavior axes are independent; `ref mut` to software state and writable property setters never authorize writes to hardware `read-only` fields. No alternative access path may read hardware `write-only` fields.

**§ 7(2).** `read-clear`/`read-set` operations are observations **with hardware effects** and must not be duplicated, speculated, elided, combined or reordered unless a verified physical plan preserves those exact effects.

**§ 7(3).** The six `write-one/zero-{clear,set,toggle}` behaviors express logical actions, not conventional assignment of a stored bit pattern. The trigger value and non-trigger semantics remain the ones defined by the register and hardware access rulebooks.

**§ 7(4).** Signal active-high/active-low interpretation belongs to driver/application semantics. The compiler must not invert logical field meaning or infer asserted/deasserted state automatically.

**§ 7(5).** Hardware operations in Semantic IR are effectful by default to the degree specified by their resolved effect contract, even if an individual operation returns an ordinary scalar value. In particular, ordinary dead-code elimination must not erase an intentionally destructive hardware observation whose return value is unused.

## § 8. Shadow, observation and commits

**§ 8(1).** `shadow` and `noshadow` remain orthogonal to field read/write permission. The effective policy is chosen from field override, register default and finally `noshadow`.

**§ 8(2).** Shadow knowledge is not current-hardware truth. Distinguish `Unknown` / `Known(value)` observed information from pending **logical write intents**. No fabricated zero/default state, spontaneous refresh or implicit write-back at scope exit/destruction is permitted.

**§ 8(3).** A logical shadow commit such as existing `Write()` must translate pending logical intents into a verified hardware plan. If no pending intents exist, it performs no physical write. It must not dump stale register-shaped cached bits.

**§ 8(4).** Semantic IR must provide explicit shadow-state read/observe, write-intent enqueue, invalidate and commit events, together with the implementation instance/shadow-group identity and control-flow state. It shall neither flatten an uncommitted logical intent into ordinary memory writes nor lose the distinction between an observed snapshot and hardware truth.

**§ 8(5).** Verifiers must check shadow availability and knowledge only to the extent already established by Sema's path-sensitive analysis and the immutable operation plan; lower stages must not invent a new source-level shadow analysis.

## § 9. Canonical Semantic IR operation families

**§ 9(1).** The following stable Semantic IR operation identifiers are normative. They name compiler-internal semantic operations and **not** public Sec functions, properties or source syntax. Sema must resolve each source operation and its access plan before constructing the corresponding IR:

| Stable Semantic IR operation ID | Inputs | Outcome | Effects |
|---|---|---|---|
| `register.value.extract` | Detached typed `T`; validated field-path ID | Detached typed field value | Pure |
| `register.value.update` | Detached `T`; field path; range-validated value | Updated detached `T` | Pure |
| `register.hw.bind` | Hardware binding; nominal `T`; resource/endpoint resolution | `HardwareInstance[T]` identity | Establishes authority association; no implicit physical I/O |
| `register.hw.observe` | `HardwareInstance[T]`; field/whole read projection; verified access plan | Observed typed value or explicit fault continuation | External hardware observation; may be destructive |
| `register.hw.request_write` | `HardwareInstance[T]`; field path; range-validated logical request; verified access plan | Write intent or completion / explicit failure continuation | Hardware write or pending logical intent |
| `register.hw.shadow_observe` | Instance/shadow group; result of permitted observation | Updated software shadow knowledge | Software state; cannot invent hardware truth |
| `register.hw.shadow_intent` | Instance/shadow group; validated logical request | Pending intent | Software shadow state |
| `register.hw.shadow_invalidate` | Instance/shadow group; affected knowledge | Updated unknown/known fact | Software shadow state |
| `register.hw.commit` | Instance/shadow group; pending intents; verified plan | Completion or explicit failure continuation | Hardware operation only when pending |

**§ 9(2).** These are distinct semantic categories. The compiler may bundle immutable validation facts in a canonical operation-plan payload, but it must not erase an externally visible effect, failure edge or shadow transition. These identifiers are independent of LLVM operation names or source spelling.

**§ 9(3).** Methods and properties remain ordinary resolved callable operations on the type-owned `impl T`. Their bodies are lowered to the compiler-known register operations where necessary. The compiler shall not expose a second public helper API for access modifiers such as `write-one-clear`.

**§ 9(4).** Each operation is target- and `CompilationPlan`-capability checked. If no semantics-preserving physical plan is available, compilation fails with a diagnostic specific to the operation, resource and unavailable capability; it does not substitute ordinary read-modify-write.

**§ 9(5).** Hardware binding and detached register values are never interchangeable in Semantic IR, even if their numeric bit widths match. `register.hw.bind` produces an instance/authority identity, **not** a copied hardware register snapshot and not a `Place` to the endpoint.

## § 10. Required operation payload and invariants

**§ 10(1).** Every register value extraction/update retains at least: `RegisterTypeID`, field-path identities, logical bit width/offset, bit allocation order where layout-dependent, endian contract at byte-serialization boundaries, field value type, enum/unit/range facts, and source provenance.

**§ 10(2).** Every hardware operation additionally retains or losslessly references:

| Group | Required data |
|---|---|
| Identity | hardware binding, resource, endpoint/alias and implementation instance IDs |
| Field | register/field identity, bit offsets, resolved access permission, trigger behavior |
| Request | observe/read, logical update, read snapshot, write intent, commit; request operand |
| Transfer | physical width/alignment and distinct read/write projections |
| Safety | reserved/collateral effects, physical read/write footprint, coherence, atomicity and failure atomicity |
| Effects | volatility, destructive read, ordering, completion, external effects |
| Shadow | group identity, observed-knowledge change, pending write intent, invalidation |
| Authority | CompilationPlan, privilege/access-context requirement and target capability |
| Failure | fault model, failure continuation/Result policy where specified, partial-failure facts |
| Trace | source provenance and immutable validated operation-plan ID |

**§ 10(3).** Sema/target-aware planning must resolve and validate these facts before declaring an operation valid. Later passes may choose a physical realization but cannot infer missing access semantics from an address, LLVM integer type, field name or a prior bit-mask expression.

**§ 10(4).** Semantic IR must carry sufficient effect dependencies that optimizing one register operation cannot change its order relative to other operations when ordered by their hardware or compiler contracts. Volatile access is not reinterpreted as inter-thread atomic synchronization.

**§ 10(5).** When a logical operation is realized by several physical transactions, the corresponding plan is one semantically coherent operation with explicit sub-operation ordering, completion, collateral effect and partial-failure contract. The optimizer must not freely treat its pieces as unrelated loads/stores.

## § 11. Resolved plans and verification

**§ 11(1).** Hardware register operation planning is compiler-owned and must be semantics preserving. Candidate physical sequences that would accidentally read/clear unrelated fields, trample reserved bits, fail required coherence or violate access contexts must be rejected.

**§ 11(2).** Sema's immutable resolved plans are the input to Semantic IR builders; builders must not independently reinterpret source modifiers or reprobe AST syntax to invent hardware meaning.

**§ 11(3).** The Semantic IR verifier checks operation category, type/field identity, required payload presence, valid enum/value-width facts, correct resource/instance linkage, legal effect/plan capabilities and non-fabrication of ordinary Place roots for physical resources. It is not a second source-level hardware planner.

**§ 11(4).** Each schema-specific MLIR verifier checks that required fields, operands, effects and plan identities survived translation into that schema. If the selected schema lacks a faithful register encoding, `emit-sec-mlir` must fail with an explicit unsupported-capability diagnostic; it must not emit a plausible but semantically incomplete operation.

**§ 11(5).** The MLIR spellings and attributes are defined by the explicit schema-11 early admission in § 19 below. Schema 12 inherits this admission unchanged; no earlier schema may accept these operations. A compiler may claim register MLIR support only after implementing the schema gate, parsing, round-trip printing, verification and all required effects. Dialect-version acceptance alone does not establish lowering capability.

## § 12. Fixed-address bindings and P15-42/42a

**§ 12(1).** `@address` is binding metadata and does not by itself confer lifetime, atomicity, safe RAM semantics or unrestricted referencing.

**§ 12(2).** A hardware-bound register binding participates in Semantic IR as a `register.hw.bind` external resource/instance root, not as an ordinary `Place[T]` whose bytes are safely borrowed. Its endpoint may be MemoryMapped, PortMapped, InstructionEncoded, RuntimeMapped or TargetDefined; only the first is necessarily in a memory-mapped target domain.

**§ 12(3).** A separate addressed ordinary-memory binding (whose target truly provides reference-compatible storage) continues through P15's addressed Place root and reference validity checks. Do not apply the hardware endpoint prohibition to unrelated legitimate fixed-address RAM.

**§ 12(4).** P15-42/42a must split these roots by their **semantic binding category**, not assume that all fixed-address roots are MMIO and not infer a hardware operation from the existence of `Place` projection syntax.

**§ 12(5).** P15 is responsible for the root identity, prohibition on direct hardware borrows, correct typing of `HardwareInstance[T]`, and faithful production of the compiler-known register semantic operations. It need not complete every target-specific hardware implementation; lacking capabilities must fail closed. The schema representation is fixed by § 19.

## § 13. Methods, properties, calls and instance lifetime

**§ 13(1).** The source `impl` stays bound to one nominal register type; no instance-specific `impl` or hidden second public access class is introduced. Different access semantics requiring a different type implementation require different nominal types.

**§ 13(2).** Method/property calls use the existing resolved callable contract. The receiver carries the required implementation instance and, for hardware operations, a verified resource association; calling a property must not silently downgrade external operations into ordinary `Place` projections.

**§ 13(3).** A function that receives an implementation-instance handle may call its exposed API under the same capability/authority restrictions. Any indirect call, interface call or closure must preserve the effects and authority constraints in its callable contract; it may not permit a hardware operation merely because the register bit layout matches.

**§ 13(4).** Regular borrowing of implementation state follows existing Sema `BorrowID`, `LifetimeID`, availability and destruction rules. Those rules control software storage, **not** hardware's independent ability to mutate asynchronously or to side-effect reads.

**§ 13(5).** Whole-instance references are typed `ref HardwareInstance[T]` or `ref mut HardwareInstance[T]` under § 4(8) in all declaration contexts: locals, parameters, return types and callable types. They are **not** typed `ref T`. Sema rejects a signature/call combination that would pass a hardware instance to `fn(ref T)` or pass a detached `T` to `fn(ref HardwareInstance[T])`. The runtime caller need not dispatch by the register's storage kind; the types are distinct before lowering.

## § 14. Interactions with `Read()` / `Write()` and errors

**§ 14(1).** Existing explicit logical `Read()` and shadow `Write()` semantics remain authoritative. `Read()` is one logical hardware observation yielding a detached snapshot where permitted; `Write()` commits pending shadow logical intents, not a raw cached integer image. It does not write when no intent is pending.

**§ 14(2).** Their implementation within an authorized register `impl` uses the same resolved hardware operation machinery as field getters and setters. Public exposure is controlled by the `impl` API instead of automatically generated public members; editorial changes to any former public compiler-known member declaration must preserve internal operation semantics.

**§ 14(3).** A getter returning `Result[T,E]` is a normal property getter and is suitable where its own API explicitly models recoverable errors. The compiler shall not unilaterally turn every hardware read into `Result`, nor silently suppress a target fault condition; the existing physical fault and mapping contracts decide that behavior.

## § 15. Integration edits to existing books (stable numbering)

**§ 15(1).** Add clarification to **`impl.md` §1 and §7** without changing existing paragraph numbers: type ownership of `impl` never changes; for hardware-bound register instances, implementation-instance `let`/`let mut` storage is separate from the physical register bits and detached register representation.

**§ 15(2).** Add to **`registers.md` §§11–12**: (a) physical hardware fields are available for internal owning-`impl` resolution and (b) public access goes only through implementation-exposed members; (c) `bit[N]` field reads reuse `types.md` unsigned carriers and Sema range proof; (d) explicit lossy `bit[N](value)` remains the only existing low-bit extraction and is not silently inserted.

**§ 15(3).** Amend **`hardware-register-access.md` §§2, 6, 7–9, 30–33 and 39** where needed: explicitly distinguish internal field observation from external public API; require `impl` for hardware-bound types; preserve all physical-plan, shadow, side-effect and transaction requirements. Keep the established §6 safe implicit observation test but make it an internal operation of the owning implementation, not an unrestricted user-access permission.

**§ 15(4).** Amend **`fixed-address-bindings.md` §§2–3** so physical register bindings and ordinary fixed-address storage are separately classified. Do not convert non-MMIO hardware endpoints to numeric memory pointers or create safe `ref` to external resources.

**§ 15(5).** Expand **`semantic_ir.md` §21 and §85** with the resolved register-vs-hardware op families, required identities/effect payloads and verifier contracts. Preserve the existing distinction between source semantics and lowering. Add cross-references to the memory/Place sections and test requirements without renumbering original sections.

**§ 15(6).** Amend **P15-42 and P15-42a** to explicitly state which addressed roots are ordinary memory Places and which are hardware binding roots, and provide the correct positive/negative tests. Install the exact schema-11 early admission from § 19, inherited by schema 12; do not introduce a private unversioned dialect extension.

**§ 15(7).** Use normal governance fragment updates for each owning book. Do not overwrite `implementation-status.yaml`, duplicate existing IDs or mark incomplete tests implemented. Close `MD-077` in `missing-decisions.yaml` only when this correction has been accepted and the normative integration completed.

## § 16. Required positive tests

**§ 16(1).** Detached `register[N]` value field extraction and update are pure typed value operations; nested `msb-first`, byte order and reserved-bit layout are preserved.

**§ 16(2).** `bit[2]` reads as `uint8` with Sema-known `0..3`; `bit[12]` as `uint16` with `0..4095`; a bit-backed enum retains nominal type; named `range` contracts are not fabricated on the field.

**§ 16(3).** A proven-fitting literal assigns to a multibit field; a proof from an existing named range or path analysis is accepted; `bit[N](value)` explicitly extracts low bits without an implicit narrowing conversion.

**§ 16(4).** A hardware-bound type with `impl` exposes only declared public properties/methods; inside `impl`, field observation and W1C/W0C, set and toggle produce the correct compiler-known semantic hardware operations.

**§ 16(5).** `read-clear` and `read-set` retain their external effects even if the observed value is discarded; shadow keeps observed knowledge distinct from hardware truth.

**§ 16(6).** A shadow-intent write remains pending until explicit commit; no-pending commit is a physical no-op; destroying the implementation state does not write back hardware.

**§ 16(7).** One type-level implementation applies to multiple distinct hardware instances with the same contract; each instance has its own permitted software state while retaining separate resource identity.

**§ 16(8).** An ordinary fixed-address RAM object may form a valid P15 Place/reference root; an external addressed hardware register resolves as a hardware resource root instead.

**§ 16(9).** Safe observations through valid read aliases, coherence-preserving multi-step observations and legal target-specific instruction endpoints produce equivalent, verified logical effects.

**§ 16(10).** Compiler/LSP diagnostics and Semantic IR inspection identify the field, access behavior, instance/resource, relevant CompilationPlan and exact missing target capability.

**§ 16(11).** `ref hardware` produces `ref HardwareInstance[T]`, whereas `ref snapshot` produces `ref T`; a parameter `ref HardwareInstance[T]` accepts the former and not the latter. An instance with no software fields retains distinct semantic identity and remains borrowable only with a valid logical software Place/lifetime.

**§ 16(12).** Parse/print/verify round trips for all § 19 register/hardware types and operation families succeed in schema 11 and schema 12 with stable canonical identities, immutable plan references and correct side effects. An operation using schema-12-specific unrelated slice features remains separately version-gated.

## § 17. Required negative tests

**§ 17(1).** Hardware binding with no primary type-owned `impl` fails at compile time; adding a generic register compiler intrinsic does not silence it.

**§ 17(2).** Direct physical register `ref` / `ref mut` (including via nested fields, unsafe-free RawPtr conversion and accidental `Place` roots) is rejected. Borrowing ordinary implementation-state storage remains separately tested.

**§ 17(3).** Public source access to a physical register field not explicitly exposed through the `impl` is rejected, even if old syntax or names would have accepted it.

**§ 17(4).** `read-only` writes, `write-only` reads and contradictory access modifiers fail; no property or method changes these permissions by renaming the operation.

**§ 17(5).** Unsafe physical RMW where collateral `read-clear`, `write-one-clear`, reserved-bit corruption, illegal width or ordering effects arise is rejected.

**§ 17(6).** Missing target endpoint capability, misalignment, unsupported access context, unverified alias, illegal partial failure or incoherent multi-step read produces a precise compilation/lowering failure, not a fabricated operation.

**§ 17(7).** A runtime value too wide for `bit[N]` is not silently truncated; a known out-of-range constant is rejected; an explicit low-bit extraction remains visibly lossy.

**§ 17(8).** No shadow state is fabricated as known at first use; an invalid branch-dependent unknown shadow access is rejected; effects remain explicit through CFG merge and calls.

**§ 17(9).** A schema lacking required register operation semantics may not accept a partial or physically guessed MLIR encoding. No successful emission may carry unresolved required hardware meaning as merely an integer load/store.

**§ 17(10).** Two register types with identical bit widths but different type-level access implementations remain nominally distinct; no implicit cross-cast or implementation substitution is performed.

**§ 17(11).** A `ref T` parameter cannot accept an instance of `HardwareInstance[T]`; `ref HardwareInstance[T]` cannot be used to read or write the physical register bits. A forged resource Place and a missing logical software-instance lifetime are rejected.

**§ 17(12).** Schemas 1–10 reject all § 19 admitted register/hardware types and operations. Schemas 11–12 reject missing/unknown required attribute fields, noncanonical field paths, bogus resource/instance IDs, mismatched register type identities, missing operation plans, and unauthorized reordering/elimination of effectful hardware observations.

## § 18. Implementation order and review gates

**§ 18(1).** Implement one tested unit at a time. Begin with Sema's detached `bit[N]` carrier/range/value conversion tests because they unblock non-hardware network/protocol code independently of hardware lowering.

**§ 18(2).** Next correct register/`impl` representation and mandatory-implementation diagnostics. Check software state and resource roots are never merged; add explicit restrictions on safe hardware references.

**§ 18(3).** Introduce Semantic IR detached register operations and then hardware operation/plan payload, using current compiler naming conventions. Reuse the existing resolved hardware register planner; do not invent a second planner.

**§ 18(4).** Implement controlled public exposure, shadow/side-effect operation emission and missing target-plan failure. Add all negative tests before enabling the corresponding CLI output path.

**§ 18(5).** Implement § 19 in schema 11 and inherited schema 12, including types, operations, verifier contracts and round trips, before enabling P15-42/42a as complete.

**§ 18(6).** Reconcile `governance` fragments and `missing-decisions.yaml` last. Do not mark MD-077 implemented solely because the design decision is finalized.

## § 19. Canonical Sec MLIR representation and versioning

**§ 19(1).** The canonical MLIR spellings for MD-077 are listed below. On normative integration they shall be installed as **explicit schema-11 early admissions** under `sec_mlir_dialect.md` v11.41 and inherited verbatim by schema 12. The schema-11 specification and its versioned snapshot must be updated together as one controlled revision. Schemas 1–10 must reject these constructs. The default emitted schema version is not changed by this decision.

**§ 19(2).** Two new compiler-owned MLIR semantic types are introduced:

```text
!sec.register<"module::TypeName">
!sec.hw_instance<"module::TypeName">
```

The first represents a **detached nominal register value**, the second a **software implementation instance/authority descriptor**. Neither is equivalent to the other or to an ordinary LLVM integer. The quoted type name must resolve to the same canonical nominal register `TypeID` in the module's verified register-definition table. Instance state layout and hardware endpoint physical representation are not implicit in either type spelling.

**§ 19(3).** The MLIR operation spellings are fixed by the following mapping:

| Semantic IR operation | Sec MLIR operation |
|---|---|
| `register.value.extract` | `sec.register.extract` |
| `register.value.update` | `sec.register.update` |
| `register.hw.bind` | `sec.hw.bind` |
| `register.hw.observe` | `sec.hw.observe` |
| `register.hw.request_write` | `sec.hw.request_write` |
| `register.hw.shadow_observe` | `sec.hw.shadow_observe` |
| `register.hw.shadow_intent` | `sec.hw.shadow_intent` |
| `register.hw.shadow_invalidate` | `sec.hw.shadow_invalidate` |
| `register.hw.commit` | `sec.hw.commit` |

**§ 19(4).** `sec.register.extract` consumes `!sec.register<T>` and a resolved field path encoded in `sec.field_path`, and produces the exact resolved Sec field value type. `sec.register.update` consumes `!sec.register<T>`, one value of the field's resolved carrier/nominal enum type, and the same validated `sec.field_path`; it produces `!sec.register<T>`. Both operations are pure and must satisfy range, register layout, reserved-bit and field-type verification; no memory or hardware effect may be attached. The module-level register definition contains ordered field names/IDs, bit widths/offsets, representation modes, effective access contracts and nominal type identity.

**§ 19(5).** `sec.hw.bind` establishes a verified instance-to-resource association. Its result is `!sec.hw_instance<T>` and it carries canonical instance, binding and resource IDs, endpoint kind, implementation identity and target-plan applicability. For runtime-mapped resources, the resolved runtime mapping operand is explicit; it is not manufactured from a numeric address. This operation does not read, write or initialize the physical register. It must reject a hardware binding without the mandatory primary `impl T`.

**§ 19(6).** `sec.hw.observe` consumes a compatible `!sec.hw_instance<T>` and a validated read projection, field/whole-register path, and immutable verified operation-plan reference. It returns the resolved detached field/register value on its successful path. Observable failures use the program's already resolved Result/failure CFG rather than being silently suppressed. `sec.hw.request_write` consumes a compatible instance, path, validated request value, and immutable verified plan reference. Both operations carry the full effect, ordering and failure contract; both are forbidden outside validated `impl T` access paths.

**§ 19(7).** `sec.hw.shadow_observe`, `sec.hw.shadow_intent`, `sec.hw.shadow_invalidate`, and `sec.hw.commit` apply to identified per-instance shadow groups. They preserve observed knowledge (`Unknown` / `Known(value)`), pending **logical intents**, paths, physical plan references, and failure/control-flow behavior. `sec.hw.commit` may have no physical effect when there are no intents; it must never be synthesized automatically during destruction or borrow end. Shadow state is software-side data and must not masquerade as the current hardware value.

**§ 19(8).** A hardware operation has **mandatory, structured and verifier-checked** metadata: `sec.register_type_id`, `sec.impl_instance_id`, `sec.hardware_binding_id`, `sec.hardware_resource_id`, `sec.endpoint_id`, `sec.field_path` or an explicitly marked whole-register projection, `sec.access_behavior`, `sec.effect_contract`, `sec.verified_plan_id`, `sec.compilation_plan_identity`, `sec.source_provenance`, and the required physical width/alignment/footprint, shadow, ordering, completion, privilege/access-context and fault-plan facts. The keys may reference an immutable, module-scoped structured plan record, but the referenced record must exist and satisfy every required constraint. Do not use unparsed descriptive strings as a substitute for canonical IDs/typed lists.

**§ 19(9).** The register-definition table, instance/resource identities and verified operation-plan records are module-scoped and canonical; each operation resolves all references during verification. Their serialized ordering is deterministic, identities are unique in the declared scope, and every layout/field path must be valid for the declared nominal `TypeID`. A serializer must not drop unknown required fields or silently reinterpret missing semantics as ordinary memory behavior.

**§ 19(10).** `!sec.ref<!sec.hw_instance<T>>` and `!sec.ref_mut<!sec.hw_instance<T>>` are ordinary schema-11 reference categories when built from an actually referable **software implementation-instance Place**. Their normal `sec.ref.*` and `sec.place.*` operations may manipulate software storage and handles, subject to the existing ownership rules. `!sec.ref<!sec.register<T>>` remains a reference to an ordinary detached register value where valid. A verifier must reject any attempt to use ordinary `sec.place.read`, `sec.place.write` or `sec.ref.borrow_*` directly on a physical `HardwareResourceID`/`HardwareEndpointID`.

**§ 19(11).** All hardware operations carry MLIR memory/effect information sufficient to prevent invalid elimination, duplication, speculation, movement, coalescing or reordering. Physical accesses that are destructive or externally observable must be considered effectful even when the resulting scalar value is unused. Pure detached register operations may be optimized as ordinary typed computations.

**§ 19(12).** The schema-11/12 verifier requires exact operand/result types, registered operation names, required structured attributes, valid field width/range/enum contracts, valid impl-instance/resource bindings, trusted plan and target eligibility, proper control-flow/fault edges and correct side-effect declarations. It verifies *preservation* of already established semantic contracts, not a second source-level register planner. An absent verified plan or an unsupported access capability is a fail-closed diagnostic.

**§ 19(13).** `emit-ir` emits verified Semantic IR according to its independently completed P15 support. `emit-sec-mlir` may emit these schema-11 operations only after the dialect supports their complete parse/print/verify contract. The existence of schema 12 does not authorize implicit use of a new operation family before its explicit admission. Emission must never publish partial output after a failed verifier or capability check. Lowering to physical memory, port I/O, instruction-encoded endpoints, runtime mappings and target-defined mechanisms stays separately capability-gated.

**§ 19(14).** This clause supersedes the three undecided review items of the October 22 MD-077 review draft: the compiler-known `HardwareInstance[T]` is the source typing rule; the names in § 9 are canonical Semantic IR operations; and § 19(1)–(13) defines the admitted MLIR schema-11/12 contract. They are decided through the owner's explicit delegation. There are no design questions reserved for implementation agents in this correction.

---

## Appendix A. Editorial replacement text for `impl.md` §7

> **Hardware-bound register implementations.** An `impl` belongs to the nominal register type and is never defined separately for individual hardware bindings. The `impl` of a hardware-bound `register[N]` may declare ordinary per-instance software state using `let` and `let mut`. Such state belongs to the implementation-instance representation, which is distinct from the physical register and from the detached `register[N]` bit layout. It contributes to normal software initialization, ownership, borrowing and destruction, but neither widens the hardware register nor authorizes direct ordinary memory access to its external endpoint. Its declarations do not create an implicit implementation for any hardware-bound register type lacking a valid type-owned `impl`.

## Appendix B. Editorial replacement text for `semantic_ir.md` §21

> **Register value and hardware operation distinction.** A nominal `register[N]` used as a detached value is represented by typed, effect-free field extraction and update operations preserving register type identity, field paths, bit layout, enum and range facts. A hardware-bound register is represented by a distinct resolved binding/resource identity and effectful logical hardware operations through an authorized type-owned implementation. These operations preserve field permissions, specialized read/write behaviors, shadow intent/knowledge, physical access plan, resource/endpoint aliases, footprints, ordering, completion, access authority, fault behavior and provenance. External hardware register endpoints are not ordinary `Place` roots and do not permit `ref` or `ref mut` to their bits. Separately allocated ordinary implementation state remains eligible for regular Place/reference operations. The compiler validates logical register operations before target-aware lowering; lower stages shall not reconstruct field behavior from integer bit masks or numeric addresses.

## Appendix C. Cross-reference inventory

- `rules/declarations/registers.md`: nominal register values, `bit[N]` field and access modifiers, nested layouts, conversions.
- `rules/types/types.md`: MD-056 `bit[N](value)` low-bit extraction, standard unsigned carrier widths and proof intervals.
- `rules/types/contracts.md`: named-type-only range contracts (distinct from value-range refinements).
- `rules/declarations/enums.md`: nominal open bit-backed enum domain.
- `rules/declarations/impl.md`: type-owned implementations, per-instance software members and implicit `self`.
- `rules/declarations/properties.md`: ordinary getters/setters, `Result[T,E]` return values.
- `rules/platform/hardware-register-access.md`: physical transaction planning, specialized reads/writes, shadow, endpoints, failure, ordered effects.
- `rules/platform/fixed-address-bindings.md`: explicit fixed binding identity without implicit memory/reference guarantee.
- `rules/compiler/semantic_ir.md`: §§21 and 85 register IR preservation and planning, §59 conversions, §§60–63 calls and indirect calls.
- `rules/mlir/packages/sec-mlir-dialect_package15.md`: P15-42/42a addressed register roots, reference integration tests.
- `missing-decisions.yaml`: MD-077 acceptance/closure must follow integration and verification.

## Appendix D. Concrete source and conformance examples

The examples below illustrate the normative source-level model. Tests must use the latest canonical Sec grammar and verify that the compiler performs no implicit hardware-as-memory conversion.

### D.1. Detached protocol value: no hardware effects

```sec
// Layout used for network packet fields, not hardware access.
type IPv4FirstByte register[8] msb-first {
    Version: bit[4],
    HeaderLength: bit[4],
}

fn ParseVersion(raw: uint8) uint8 {
    let header := IPv4FirstByte(raw)
    return header.Version
}
```

Expected: `header.Version` has unsigned carrier `uint8` and proven interval `0..15`. `ParseVersion` requires no hardware binding, implementation, endpoint or shadow state.

### D.2. Explicit low-bit extraction, not an implicit cast

```sec
fn LowNibble(word: uint16) uint8 {
    return bit[4](word)
}
```

Expected: the result is `uint8`, known to be within `0..15`. A field assignment consuming this result uses the existing range proof. No implicit truncation of `word` occurs anywhere else.

### D.3. Hardware register and controlled public API

```sec
type InterruptStatus register[8] {
    Ready: bit read-only,
    Pending: bit write-one-clear,
    _: bit[6],
}

impl InterruptStatus {
    property IsReady: bool {
        get {
            return self.Ready
        }
    }

    fn ClearPending() void {
        self.Pending = true
    }
}

@address(Platform.InterruptStatus)
let mut interruptStatus: InterruptStatus

fn CheckInterrupt() bool {
    return interruptStatus.IsReady
}

fn AcknowledgeInterrupt() void {
    interruptStatus.ClearPending()
}
```

Expected: `self.Ready` requests a validated logical hardware observation; it does not imply a one-bit bus read. `self.Pending = true` requests the W1C clear action rather than storing the bit `1`. Whether either operation can be physically realized is decided by the target's access contract. The public API contains `IsReady` and `ClearPending`, not an automatically public hardware `Pending` field. If the selected endpoint cannot safely observe `Ready` without an unintended collateral destructive read, `CheckInterrupt` fails to compile for that plan.

### D.4. Deliberately invalid direct hardware access and borrowing

```sec
// Invalid: no public field access is exposed for Pending.
interruptStatus.Pending = true

// Invalid: hardware bits cannot be taken as ordinary safe storage.
let physicalReference := ref mut interruptStatus.Pending
```

Expected: targeted diagnostics distinguish forbidden public access from forbidden physical borrowing. If user code borrows software state owned by the `impl` instead, the existing normal memory/reference rules apply.

### D.5. Same bit layout, different register implementations

```sec
type InterruptStatusA register[8] {
    Pending: bit write-one-clear,
    _: bit[7],
}

type InterruptStatusB register[8] {
    Pending: bit write-one-clear,
    _: bit[7],
}
```

Expected: the nominal types are distinct despite matching bit layout. A type-owned `impl` may have a different internally compiled hardware-access behavior for each type. An instance cannot add a competing `impl` to either type.

### D.6. Suggested Semantic IR inspection assertions

The following are **test assertions**, not a literal Sec/MLIR textual assembly syntax:

```text
ParseVersion:
    one register.value.extract
    zero register.hw.observe
    zero register.hw.request-write

CheckInterrupt:
    one authorized implementation getter call
    hardware operation carries ReadOnly and resolved read footprint
    no ordinary place.read against the hardware endpoint

AcknowledgeInterrupt:
    one authorized implementation method call
    register.hw.request-write has WriteOneClear effect
    no implicit general read-modify-write

Hardware binding:
    HardwareBindingID exists
    HardwareResourceID exists
    implementation-instance software storage is distinct
    no safe ref to HardwareEndpointID
```

The operation-ID spellings above are normative under §9(1).

### D.7. Distinct whole-instance reference typing

```sec
fn InspectHardware(device: ref HardwareInstance[InterruptStatus]) bool {
    return device.IsReady
}

fn UseHardware() bool {
    return InspectHardware(interruptStatus)
}
```

Expected: argument preparation obtains a borrow of the ordinary implementation instance, never of physical register bytes. The normal borrowing rules determine legality. A detached `InterruptStatus` value is not type-compatible with this function. The public getter's hardware effect is preserved.



---

## Application note (2026-10-22)

**Normative integration (no renumbering):**
- `rules/declarations/registers.md`: new § 11.7 and § 12.x.
- `rules/declarations/impl.md`: § 1 amendment, and Appendix A verbatim in § 7.
- `rules/declarations/properties.md`: closing amendment.
- `rules/types/types.md`: bit-extraction cross-reference.
- `rules/platform/hardware-register-access.md`: amendments in §§ 2, 6, 7, 8, 9, 30, 31, 32, 33 and 39.
- `rules/platform/fixed-address-bindings.md`: amendments in §§ 2–3.
- `rules/compiler/semantic_ir.md`: § 21(4) (Appendix B verbatim), § 21(5) and § 85(4).
- `rules/mlir/sec_mlir_dialect.md`: a v11.41 table row, the new v11.41.1 subsection, and the inheritance note in v12.1.
- `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`: amendment entry.
- `rules/mlir/packages/sec-mlir-dialect_package15.md`: P15-42/42a amendment before § 64.

**Implementation audit (2026-10-22), divergences to close in the § 18 order:**
- Detached values: field reads already give the § 6 carriers (`bit[4]` → `uint8`), and `bit[N](v)` gives the carrier. Semantic IR rejects every register type.
- Hardware-bound instances: a binding without `impl` is accepted, direct public field access outside the `impl` is accepted, `ref mut device` is accepted with the register type instead of `HardwareInstance[T]`, and no register Semantic IR or MLIR exists.
- About 11 `@address` fixtures and 7 Go test files use direct access. They migrate with step 2.

**Implementation progress: steps 1–2 done (2026-10-22); step 3 detached part done (2026-10-23).**

- **Step 1:**
  - Detached field carriers, proven writes and no-truncation are locked by `TestMD077DetachedFieldCarriersAndProvenWrites` and `TestMD077DetachedFieldWritesNeverTruncate`.
  - Sema already conformed.
- **Step 2:**
  - The compiler-known `HardwareInstance[T]` is non-copyable and trivially destructible, and applies only to register `T`. `dereferenceType` unwraps it to reach `impl T`.
  - An `@address` binding resolves to that type, and a type-owned primary `impl` is mandatory.
  - Outside `impl T`, a physical field gets a diagnostic naming the public members. Inside `impl T`, including static members that use a global binding, it resolves.
  - `ref`/`ref mut` of physical bits are rejected, both through an instance and through `self` in the `impl` of a register type. `ref` of the instance is `ref HardwareInstance[T]` and never crosses with `ref T` or with a same-layout type.
  - LSP completion lists `impl` members, not fields. Hover shows `HardwareInstance[T]`.
  - Tests: `TestMD077HardwareInstancesUseTheImplAPI`, `TestMD077HardwareInstanceMisuseIsRejected`, `TestHardwareInstanceCompletionAndHover`.
- **Migration:** 17 Sema tests and the `@address` fixtures now use the `impl` API. In `register_invalid.sec`, every intended error is still triggered inside the owning `impl`, plus the new MD-077 cases. `register3/4/5/6_valid.sec` still contain earlier, unrelated errors (units used as types, unknown members) that no test exercises.
- **Remaining:** steps 3–5. Semantic IR register operations with their payload, shadow and effect emission, and fail-closed capability checks come first, followed by MLIR v11.41.1 and P15-42/42a.
- **Step 3, detached part (2026-10-23):**
  - Sema records immutable register field plans (`ResolvedRegisterFieldOf`) with the root kind, plus write proofs (`ResolvedRegisterWriteOf`).
  - Semantic IR has the register type, the module register-definition table and the pure `register.value.extract`/`register.value.update`, including nested registers and registers inside structs (`register_values_test.go`).
  - Hardware roots, `HardwareInstance[T]` and integer-to-register conversion fail closed with specific diagnostics. The conversion is open as MD-110.
  - `impl T` members used on a software receiver lower `self.Field` as ordinary register value operations through the receiver Place (owner clarification 2026-10-23, `registers.md` § 11.7). A member that borrows a receiver field is rejected on a hardware instance.
  - `shadow`/`noshadow` modifiers are implemented in the parser, Sema (inheritance), formatter, LSP hover and the IR definition table.
- **Step 4, logical requests (2026-10-23):** hardware and impl-receiver field steps resolve to logical requests (observe, destructive-observe, write-value, clear/set/toggle action). The safe implicit read rule, the compound-update restriction and the trigger-value rule (MD-112) are enforced on hardware and make impl members software-only.
- **Step 4, `Read()`/`Write()` (2026-10-23):** these are compiler-known, impl-only operations on a hardware receiver. A member that uses them is hardware-only, and `Read`/`Write` are reserved member names in a register `impl` (owner decision). `Write()` requires a writable shadow field.
- **Steps 3–4, physical part:** blocked by MD-111, which has no canonical endpoint/DeviceModel fact source, so no verified physical plan can exist. Hardware uses fail closed.
