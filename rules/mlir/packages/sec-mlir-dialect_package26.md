# Sec MLIR Program — Package 26: Validated Physical Reference and Descriptor Lowering

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-19
- **Last updated:** 2026-10-19
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/packages/sec-mlir-dialect_package26.md` (installed 2026-10-20)
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P26`

---
## § 1. Purpose and exact result

P26 implements **Validated Physical Reference and Descriptor Lowering**. It converts already-resolved Sec safe-reference and descriptor semantics into the physical representation selected by the canonical `CompilationPlan`, discharging every required validity obligation either by a traceable static proof, by a correctly ordered runtime check, or by verified target support. It does **not** decide source borrowing, ownership, descriptor layout, ABI class, public APIs, or FFI legality.

Expected path:

```text
Sema reference/Place/borrow/lifetime facts
  -> verified Semantic IR and high-level Sec MLIR
  -> P24 ResolvedLayout + selected reference/descriptor policy
  -> P26 guarantee-specific physical materialization and validation
  -> standard MLIR (Arith, CF, Func, MemRef only where selected)
  -> later LLVM dialect/ABI finalization
```

**Schema v22 / lowering v22 are planned**, not installed. The repository's runnable dialect description presently identifies schema v11.

## § 2. Verified synchronization basis and uncertain HEAD

The newest available project inventory explicitly reviewed is `package25-todo.md`, inventory date **2026-10-18**, audited commit **`831f6e1`**, reporting 145 dirty working-tree entries at its inventory time. Public current-file views were reviewed on 2026-10-19. An independent current `git rev-parse HEAD` could not be obtained from this environment: **do not assert that `831f6e1` is today's `main` HEAD**. Before implementation capture live `git rev-parse HEAD`, `git status --short`, actual installed MLIR schema, user-uncommitted work and exact toolchain versions.

P25 TODO marks items P25-06..P25-10 complete, primarily frontend C facts/FFI legality/high-level extern preservation; full `CallABIPlan`, schema 21 and ABI transport are still unchecked. Other package TODOs are planning/inventory evidence, not acceptance reports.

## § 3. Authoritative October rulebooks

Read in the implementer's checkout, using `git ls-files` to verify paths:

```text
rules/memory/reference_model.md
rules/memory/references.md
rules/memory/borrowing.md
rules/memory/lifetime_analysis.md
rules/memory/storage.md
rules/memory/memory_model.md
rules/memory/layout.md
rules/memory/raw_pointers.md
rules/memory/ownership.md
rules/memory/copy_move.md
rules/memory/destruction.md
rules/memory/arena.md
rules/platform/abi.md
rules/platform/ffi.md
rules/platform/target_profiles.md
rules/compiler/semantic_ir.md
rules/compiler/compiler_pipeline.md
rules/mlir/sec_mlir_dialect.md
rules/mlir/sec_mlir_lowering.md
missing-decisions.yaml
```

P15–P25 package amendments and **corrected local package versions** take precedence over obsolete package copies only where they have normative approval. Rulebooks, not README examples, own semantics.

## § 4. Decisions and duplicate avoidance

The checked public `missing-decisions.yaml` is a synchronization aid only: locally updated decisions may not be committed. The scope must respect **MD-070** (public spelling/facts for `RawPtr` ↔ safe ref), **MD-019** (explicit source layout spelling), and outstanding P23-related surface gates **MD-073/MD-109**. Applied reference decisions, including **MD-061** reborrow meaning, must be implemented rather than reopened. No new P26-D decision is required to implement selected reference representations: the canonical rulebook already delegates physical policy to the CompilationPlan.

New genuine forks discovered during coding get standalone `P26-D1.md`, `P26-D2.md`, etc. and are later migrated by the user into `missing-decisions.yaml`. Never create duplicate MD IDs.

## § 5. Actual predecessor capability matrix

Codex must populate and test the following gates **before** any dependent lowering, not by checking package numbers:

```text
P15 -> Sema-resolved Place and direct-ref identities, borrow/validity facts
P16 -> slice provenance, range bounds and lifetime facts
P17 -> move/destruction/cleanup responsibilities
P18 -> dynamic backing identity and storage transition model
P19 -> Arena domain/epoch and allocation context
P23 -> string materialization/storage contracts
P24 -> complete plan-keyed ResolvedLayout and descriptor policy
P25 -> complete immutable CallABIPlan at native/foreign boundaries
```

P26 static proofs and plan records can be implemented/tested before all gates. Actual physical lowering of a feature is **blocked**, with actionable diagnostics, if the selected required plan is incomplete. Do not call a planned schema an executable predecessor.

## § 6. One value, three independent physical contracts

Keep identities separate:

```text
Semantic reference / type identity    (P15 and reference_model.md)
Physical storage representation       (P24 ResolvedLayout)
ABI transport representation          (P25 CallABIPlan)
```

Also keep `StorageIdentity`, `InvalidationDomain`, `EpochDependency`, `BorrowID`, `LifetimeID`, `Provenance`, `AllocationIdentity` and `RawPtr` logically distinct. Equal machine words never imply equal identities. P26 consumes, does not replace, these facts.

## § 7. Reference validity is a conjunction of guarantees

A safe `ref T`/`ref mut T` access requires all applicable guarantees: temporal validity, spatial extent, provenance, initialized valid typed representation, alignment, authority/borrow compatibility, address-space capability, relocation, context/synchronization and trust contract. **One successful generation check is not proof of all the others**. Carry each required guarantee separately through physical lowering and verification.

## § 8. Plan-keyed immutable physical reference decision

Introduce or reuse a read-only plan equivalent to:

```go
type ReferencePhysicalPlan struct {
    SemanticType       TypeID
    ResolvedLayout     LayoutID
    CompilationPlan    CompilationPlanID
    RepresentationID  RepresentationPolicyID
    StorageIdentity   StorageIdentityID
    InvalidationDomain InvalidationDomainID
    EpochDependency   EpochDependencyID
    AddressSpace      AddressSpaceID
    ProofSet          ReferenceProofSetID
    CheckPlan         ReferenceValidationPlanID
}
```

Names/field layout are advisory; do not duplicate existing canonical structures. Cache key includes concrete type, selected representation, target/profile, source authority and all semantic validity domains. No host defaults, no stale/incompatible reuse.

## § 9. Reference representation choices

Only a **CompilationPlan-selected, fully proven** policy may materialize one of:

```text
address only
address + bounds/length
address + expected epoch
address + bounds + expected epoch
slot/domain identity + generation
hardware capability/tagged representation
other registered semantics-preserving policy
```

These are options, not a universal mandatory fat pointer. If no registered policy preserves the reference contract, reject the operation/target rather than silently falling back to raw address.

## § 10. No mandatory runtime infrastructure

No global generation table, slot registry, ownership table, reference count, GC, heap allocation or runtime borrow checker is introduced merely for convenience. If a specific selected representation requires side-table/control state, the plan identifies the provider, storage, synchronization, lifetime and bounded profile legality explicitly. Reference creation itself must not silently allocate.

## § 11. Default logical epoch width

Reference-model `§ 33` fixes a default **64-bit logical validity epoch even on 32-bit targets**. This is independent of pointer width and ABI register size. A narrower physical representation needs the profile's pre-proven retirement/rekey/exhaustion discipline; truncating the 64-bit logical epoch is forbidden. Checks must not read a torn multiword epoch in concurrent contexts.

## § 12. Guarantee discharge taxonomy

Every pending validity obligation is discharged by exactly one explicit kind:

```text
StaticProof        exact Sema/analysis proof reference and invalidation scope
DynamicCheck       validated check operation, order and dominating success edge
TargetGuarantee    registered target capability plus provenance/bounds contract
```

Unknown, invented, expired or wrong-target evidence is a failure. P24 `sec.repr.discharge` is reused; P26 does not create a parallel general proof system. Runtime-check removal must preserve any intentional hardening required by the chosen profile.

## § 13. Static address-only specialization

When the compiler has complete static proof of lifetime, bounds, provenance, initialization, borrow and relocation correctness, a short-lived direct reference may lower to an address-only value. Record the `StaticProof` on the emitted representation even if its runtime payload has no metadata. Backend `nonnull`, `dereferenceable`, `noalias`, lifetime and alignment attributes must be no stronger than that proof. Fail closed if the proof ceases to apply after a pass.

## § 14. Shared and mutable authority

`ref T` remains non-owning shared read authority; `ref mut T` is non-owning exclusive mutable authority and a move-only holder. Materializing a reference cannot create a new Sec owner, extend referent lifetime, upgrade read authority or turn a mutable holder copyable. For `ref holder`, apply **MD-061** reborrow-the-referent semantics, not holder-address semantics. Underlying physical load/store may use an address only after all relevant authority obligations remain valid.

## § 15. Place derivation and projection

Derive physical field, fixed-array element, dynamic element, active variant and slice-subrange addresses from P15 `Place` identities and P24 exact layout offsets/strides. Preserve parent provenance, storage incarnation, bounds and authority. Prohibit pointer arithmetic on a safe reference as an ordinary source operation. Zero-sized elements still have distinct semantic indexes/Place identities despite lack of distinct bytes.

## § 16. Spatial and bounds checks

Guard every unproven element or range access using the canonical P14–P16 and P18 checks. Check signed negative indexes, checked index arithmetic, overflow and `start <= end <= length` before forming an out-of-range pointer. A byte address cannot be fabricated before the necessary bound check merely because it is later discarded. Empty slice bases may have hidden sentinel/null-like physical representation, but never become safe scalar references.

## § 17. Reference validation before use

A dynamic use requiring an epoch check must verify the current domain incarnation and expected epoch **before** a dereference. Guard must dominate the access on every CFG predecessor and remain valid after intervening invalidation events. Bounds/provenance/typed-value obligations remain separate. A check on one domain cannot discharge a guard on another domain or an old incarnation.

## § 18. Stale direct reference failure

A detected stale ordinary direct reference follows canonical `panic.invalid-reference-generation` (`InvalidReferenceGeneration`) according to `reference_model.md § 76` and the active panic profile. It is **not** an ordinary `Result`, not caught with `try`, not LLVM undefined behavior and not a new error family. Targets without a general runtime still need the correct deterministic panic/trap endpoint.

## § 19. Generation exhaustion

Never silently wrap an epoch and revalidate an old reference. Consume the domain's established overflow/rekey/retirement strategy from its resolved storage/profile plan; Arena exhaustion policy belongs to Arena. If the required strategy is missing, fail compile/plan validation. A counter increment emitted without checked exhaustion behavior is incorrect.

## § 20. Explicit invalidation transitions

Consume P18/P19/P24 canonical storage facts:

```text
EstablishDomain
AdvanceEpoch
EndDomain
ReclaimStorage
```

Arena Reset advances epoch; Arena Release ends the domain; owning dynamic-array relocation either advances backing epoch or ends old/establishes new depending on its plan. Ordinary Arena allocation does not advance epoch. Do not invent new transitions or assume every descriptor write relocates backing.

## § 21. Relocation and pinned/direct addresses

A live direct reference blocks relocation of referenced storage unless an equivalent valid transformation is proven by the canonical analysis. A selected descriptor relocation must first prove the borrow/invalidation relationship and then emit the exact domain transition. Pinned/MMIO/static contexts obey their dedicated platform/storage contracts. Do not move storage behind a physical pointer and silently retarget the reference.

## § 22. Slice representation

For `ref T[]` and `ref mut T[]`, materialize the P16 non-owning slice value using the P24 selected descriptor plan. Store/derive exact logical range length, selected address/provenance and optional runtime epoch/bounds metadata. A slice never owns or destroys backing storage. Shared slices are copyable, mutable slices move-only. Preserve view origin through reborrow, subslice, indexing, call and return paths.

## § 23. Slices from owning arrays

Borrowing from P18 owning `T[]` uses the **current backing-storage incarnation**, not merely owner binding identity. A structural mutation that may invalidate the slice is rejected or delayed until the direct borrow ends. In-place non-invalidating element writes follow borrow authority. Descriptor materialization itself neither copies elements nor allocates.

## § 24. Owning dynamic array descriptors

P18 `T[]` remains an owning move-only descriptor with runtime length/internal capacity and backing/reclamation plans. P26 lowers only its **selected** P24 physical descriptor shape and validates backing generation at ref/slice derivation; P17 still owns moves/destruction and P19 allocator facts. Descriptor bit splitting is not a source-level copy or a separate owner. Do not expose `capacity` as a new source member or implement an unapproved `Push`/`Reserve` API.

## § 25. String descriptors

P23 `string` representation is profile-selected and may be literal/static, borrowed/view or owning materialized as permitted by its source contract. P26 must not assume all strings own heap bytes. UTF-8 validity and scalar/byte lengths stay semantic; `.Len` (scalar count) and value `.SizeOf` (UTF-8 payload bytes) do not equal the descriptor type's layout `SizeOf`. Preserve single fallible runtime materialization per MD-004. Gates for public `ToCharArray` and stdlib byte-to-string materialization stay with existing MD-073/MD-109, not P26.

## § 26. Arena and allocation-context representation

P19 `Arena` may own/borrow backing and has domain identity, validity epoch and separate state version. P26 materializes only P24 selected Arena/reference representation; it does not choose a cursor/segment layout or allocator provider. `Arena.New` returns a direct mutable ref and `Arena.Alloc` a mutable slice carrying ArenaDomain/epoch/bounds facts. `Reset` is an invalidation barrier; `Release` is terminal. Hidden `!sec.alloc_context` is not a public descriptor.

## § 27. Variants, interfaces and other descriptors

References into active union/Result/Option payloads must retain proven tag/active-member validity; P24 repr ops may project only the active valid payload. Interface/closure/callable descriptors, if materialized, require their own already-resolved P24 and semantic contracts; a single universal `(ptr,len)` or data-plus-vtable format is not invented. Unsupported selected policy yields a specific capability gap.

## § 28. Stable and weak handles boundary

The reference model distinguishes direct references from stable handles and weak handles. It defines slot/generation and fallible resolution concepts but **does not finalize a public Sec 0.1 stable/weak-handle API**. P26 may preserve and verify handle-domain facts or lower a separately approved internal handle source; it must not introduce a mandatory runtime registry, a public `Handle[T]` or implicit relocation-surviving direct refs. Ordinary stale direct refs panic; missing weak handle resolution remains ordinary fallibility.

## § 29. Raw pointer boundary and MD-070

`RawPtr[T]` is not a safe reference. The public conversion spellings in both directions and result proof facts are still tracked under **MD-070**; do not create P26-D1 for them. P26 may lower *existing verified internal* reference-to-address adaptations at P25 approved call-bounded foreign parameter boundaries. Never fabricate `ref T` from a foreign raw return, integer address or `bitcast` without the canonical unsafe/trusted proof.

## § 30. Address-space and capability targets

Carry target memory space, raw-pointer address space, capability provenance, required tag preservation, alignment and access context from the canonical CompilationPlan. Standard MLIR type conversion may not silently collapse non-flat capabilities to integers or AS0 pointers. Unsupported address-space mapping is a target-capability diagnostic, not a new ABI policy.

## § 31. Concurrency and atomic generation reads

An epoch match is not sufficient when concurrent invalidation/reclamation can race with use. A valid selected concurrency protocol is required (ownership exclusion, atomic operations, lock/critical section, versioned slot, etc.). On 32-bit targets, multiword 64-bit logical epochs need non-tearing validation where sharing permits concurrency. Do not implement threading/synchronization policy inside P26; consume verified plans or fail closed. Volatile is not synchronization.

## § 32. Runtime check ordering and CFG

Emit checks in Sema-resolved evaluation order. Load/compute source operands once. Check before invalid physical address derivation or dereference. A checked success edge dominates the guarded operation; failure routes to exact Sec panic/Result endpoint as specified by the **particular** guard. Moving checks across writes, Reset, Release, deallocation, foreign calls with unknown retention/effects or iterator invalidation is forbidden absent complete proof.

## § 33. Safe load and store

Refine P24 `sec.repr.load_validated` and `.store_validated`. They may lower to typed standard MLIR operations only after reference validity and representation obligations are discharged; no check/metadata is silently dropped. Preserve P17 replacement-before-destruction rules, typed representation validity, volatile/atomic distinctions and stable Place identity.

## § 34. No hidden ownership or allocation

Splitting a descriptor into machine values, materializing a reference or emitting generation validation is ABI/representation machinery, not an implicit Sec copy, move, allocation or destructor. P17 cleanup retains single logical owner identity. P19 allocation context is used only by operations already semantically allocation-capable. No new heap/side-table allocation is introduced without a selected provider and explicit effect.

## § 35. P25 call ABI handoff

P26 consumes complete P25 `CallABIPlan` at a physical call boundary, or retains high-level callable forms until P25 is executable. No `GetABIType(T)` or argument position-blind guess. Physical safe-reference parts, borrow lifetime and hidden validity components correspond to exactly one ABI plan; split machine values do not become new Sec reference/owners. Native Sec calls preserve the complete reference representation. No reclassification in P26/LLVM.

## § 36. FFI call-bounded adaptation

Only P25-approved foreign `ref`/`ref mut` **parameter** adaptation may extract the call-bounded address portion after validation, and only under non-retention and compatible authority/target contract. The foreign boundary never constructs a safe `ref` from raw result, never automatically adapts `string`/slice/owning-array/`Result` to C and never adds hidden Sec context parameters to ordinary C/system ABI. `unsafe extern` and plain `extern` retain their current distinct source rules.

## § 37. Separate compilation and caching

Reference/descriptor representation decisions and ABI correspondence must be serialized using canonical structural meaning (not source names or raw hash only) and linked to CompilationPlan, LayoutID, representation version, safety proof provenance and CallABIPlan where used. Stale/incompatible cached proof or representation fails closed or is recomputed. Same-address reused allocation cannot revive an old domain identity.

## § 38. Sema/Semantic IR contract

Extend existing P15/P16/P18/P19 SemIR **without a second borrow checker**. Introduce/refine read-only `ResolvedReferencePhysicalPlan` and `ResolvedDescriptorMaterializationPlan` facts referencing canonical Sema proof sets and P24 layouts. Builder queries are pure, no target guessing or generation policy selection. IR printer preserves deterministic domain/layout/proof references until discharge.

## § 39. SemIR reference lowering operations

Reuse P15/P16 operations (borrow, reborrow, derive Place/reference, slice projection) and P24 physical operations (`Materialize`, `Project*`, `CheckValidity`, `LoadValidated`, `StoreValidated`, `Discharge`). P26 may add the minimal typed validity/epoch-check operations in schema v22 but must not replicate source-level borrow/ownership rules. Store traceable `ReferenceValidationPlanRef` for each check.

## § 40. Schema v22

Install only after predecessor schema support exists. Planned schema v22 extends P24 `sec.repr.*`, P15/P16 ref/slice and P18 `sec.storage.*`, adding at most:

```text
sec.ref_repr.check_epoch
sec.ref_repr.check_spatial
sec.ref_repr.guarded_access
sec.descriptor.materialize_selected
sec.descriptor.project_selected
```

Reuse existing operations rather than register synonyms where actual in-tree dialect already expresses the identical contract. Introduce typed plan/proof attributes, not ad-hoc stringly typed flags. There is no universal pointer descriptor type.

## § 41. Physical lowering passes

Register staged passes or equivalent deterministic boundaries:

```text
--sec-resolve-reference-representation
--sec-verify-reference-physical-plans
--sec-lower-reference-validity
--sec-lower-selected-descriptors
```

Do not run these merely because a package number is high. Require actual verified P24 layouts and P15/P16/P18/P19 facts; require P25 plan only for its ABI crossing. Reject any residual unsupported selected high-level operation before physical emission.

## § 42. Required verifiers

Register:

```text
--sec-verify-physical-references
--sec-verify-physical-descriptors
--sec-verify-reference-discharges
```

Extend P15 reference guards, P16 slice guards, P18 storage transitions, P19 Arenas, P24 layout obligations and P25 ABI verifier where those passes are installed. A malformed imported v22 module fails verification even if Sema would never have produced it.

## § 43. Physical reference verifier

Check representation policy identity, correct layout/target, source reference kind, nonnull scalar safety, alignment, valid type/active variant, borrow authority, exact provenance, spatial narrowing, epoch/domain agreement, pin/relocation dependencies, safe address space, check dominance and invalidation barriers. Forbid ref fabricated from RawPtr, copy of mutable holder, status-null encoding, obsolete generation reused after domain end and metadata stronger than proved.

## § 44. Physical descriptor verifier

Check exact selected LayoutID and policy; member/length/capacity projections do not invent semantics; slice bounds/owner-domain linkage; empty views cannot yield safe scalar ref; owned backing reclamation remains P17/P18; string literal vs allocated backing not conflated; Arena epoch distinct from Arena state version; no fabricated universal `(ptr,len)`; ABI parts correspond to immutable P25 plan.

## § 45. Proof-discharge verifier

Verify each obligation has exactly one sufficient discharge; dynamic-check operands and success edge dominate access and no intervening invalidation; static proof is in scope for **all** possible origins; target proof identifies actual registered capability; no raw/unsafe trust escalation. After lowering each remaining required guarantee is statically proven or embodied by a remaining runtime mechanism. Diagnostics identify missing guarantee, origin, invalidating event and selected target plan.

## § 46. Check-elimination soundness

Optimization may remove a runtime check only with a persisted, independently verifiable proof for precisely that guarantee. Distinct paths, loops, concurrent invalidation, foreign calls, aliasing and generation advance invalidate reuse assumptions. Deliberately warm caches with incompatible plan/epoch/representation state; verify cold-run equivalence. Never rely on SSA use counts or LLVM's optimizer as a semantic borrow validator.

## § 47. Panic and profile behavior

Dynamic stale check routes to registered `panic.invalid-reference-generation`. Bounds failure uses the existing bounds failure policy, not a stale-reference panic. A profile that traps without unwinding does not promise P17 cleanup on panic; a cleanup-capable profile follows its chosen panic policy. No universal unwinder or runtime panic registry is added. `try` cannot catch an ordinary stale direct reference.

## § 48. Errors and diagnostics

Diagnostic contract: include source Place/projection, reference kind, required guarantee, invalidation provenance, relevant type, target/profile and how to resolve or reduce the borrow. Separate malformed Sec source, valid-but-unsupported target representation, imported IR invariant violation and stale metadata. Do not claim source is invalid merely because v22 or a selected profile backend is not yet implemented. Link to `sec diagnostics <ID>` where the repository has assigned an ID.

## § 49. Testing: profile matrix

Test at least 32-/64-bit target plans, capability/address-space-dependent variants, hosted optimized address-only proof, hardened epoch-carrying ref, constrained bare-metal proven ref with zero runtime manager, and missing selected policy rejection. Check logical epoch default 64 bits on 32-bit general purpose, no silent truncation, verified compact representation only with exhaustion proof, checked non-wrapping epoch advancement and concurrent non-tearing where required.

## § 50. Testing: reference origin and derived views

Positive and negative: shared/mutable ref, copy shared, move mutable, reborrow referent per MD-061, field Place, fixed/dynamic element Place, nested subslice, finite multi-origin returned refs, mutable overlap rejection, type/active-variant validity, zero-sized elements, null/sentinel empty slice, bounds-before-address, aligned/misaligned storage and dangling owner at scope end.

## § 51. Testing: invalidation, relocation and ownership

Arena repeated allocation does **not** invalidate old refs; Reset advances epoch; Release ends domain; backing relocation invalidates direct slices/refs; in-place replacement does not relocate; same address reused after deallocation is a new identity; moved `T[]` owner retains backing identity; P17 destruction ends/reclaims in correct order; captured deferred ref blocks early invalidation; epoch overflow does not wrap; concurrent invalidation requires proper protocol.

## § 52. Testing: FFI/ABI and descriptors

Native call/return of safe ref/slice through matching P25 ABI plan; ordinary vs `unsafe extern`; legal call-bounded foreign ref param -> raw address; invalid foreign raw return -> safe ref; illegal retained foreign reference; descriptor string/slice cannot silently cross C ABI; wrong hidden ABI part order; stale ABI fingerprint; `MD-070` public conversion API remains gated, without disabling internal verified adapters.

## § 53. Testing: verifier mutations and differential pipeline

Malformed schema v22 cases: unguarded deref, wrong domain epoch, check after access, erased proof, wrong target LayoutID, forged noalias/non-null, raw pointer provenance fabrication, concurrent torn epoch, double-descriptor ownership, invalid reborrow, stale cache, wrong ABI plan. Exercise cold/warm reproducibility and deterministic printer. Verify high-level vs lowered behavior, and legacy path parity only on actually supported operations.

## § 54. Implementation stages and one-point-at-a-time rule

Codex must execute **one numbered action at a time and test it immediately**, recording results before continuing:

```text
A. Audit: repo/dirty state, P15–P25 capability matrix, exact tests.
B. Plan: one proof/representation service referencing P24, static specialization.
C. Ref: scalar direct ref materialization, runtime epoch guards, CFG verification.
D. Views: slices, fields, owner backing, string/Arena only for ready policies.
E. Calls: consume P25 CallABIPlan when actually executable.
F. Regressions: malformed IR, profile matrix, caches, governance/report.
```

Never replace a working predecessor subsystem simply to fit the proposed operation names. If a prerequisite is missing, implement independent subfeatures and leave exact blocker evidence.

## § 55. Repository implementation anchors

Inspect/extend actual existing code before creating new modules:

```text
internal/layout/reference_policy.go
internal/layout/scalar_plan.go
internal/platform/target/target.go
internal/sema/place.go
internal/ir/semantic/model.go
internal/ir/semantic/builder.go
internal/ir/semantic/verifier.go
internal/lowering/secmlir/emitter.go
mlir/ (TableGen, verifiers and conversion passes)
governance/memory.yaml
governance/compiler.yaml
governance/platform.yaml
governance/ffi.yaml
```

These paths are inventory hints; verify current existence by `git ls-files`. Preserve `governance` stable IDs and use the existing fragment schema. Never write a new parallel `implementation_state` namespace.

## § 56. Out of scope

P26 does **not** define public stable/weak handle APIs, `RawPtr` ↔ safe conversion source syntax (MD-070), a runtime borrow checker, a mandatory generation manager, GC, a general heap allocator, user-visible lifetimes, arbitrary FFI descriptor compatibility, standard C exports, new pointer arithmetic, target-specific sync rules, full threading/interrupt lowering, final LLVM reference/ABI encoding, generic hardware MMIO access lowering, or a general compiler pipeline closure. Keep all such gates explicit.

## § 57. Architecture invariants

Non-negotiable:

```text
Physical representation never defines Sec reference semantics.
P24 is the only source of physical layout.
P25 is the only source of callable ABI classification.
A generation match alone is insufficient for safe access.
RawPtr is never silently a safe ref.
Address-only ref requires complete static proof.
Epoch width is independent of pointer width.
No stale epoch may wrap into validity.
Generation checks precede dependent dereference.
Stale ordinary ref -> InvalidReferenceGeneration panic, not Result/UB.
Static proof can remove runtime check without weakening guarantees.
Direct ref does not silently survive physical relocation.
Stable/weak handles do not become ordinary refs by default.
No implicit reference ownership or heap allocation.
No fabricated noalias, nonnull, dereferenceable or lifetime attributes.
Source FFI legality remains Sema's decision.
No universal physical descriptor is assumed.
No LLVM dialect is required by P26.
```

## § 58. Acceptance criteria

P26 completion requires independent evidence for each:

```text
[ ] live checkout HEAD and dirty tree recorded
[ ] executable P15–P25 predecessor evidence recorded
[ ] corrected local package specs preserved
[ ] current decisions/governance checked without duplicates
[ ] canonical reference-policy/layout query reused
[ ] complete guarantee-specific proof obligations modeled
[ ] source and physical reference identity remain distinct
[ ] address-only statically-proven ref works
[ ] checked epoch-carrying ref works under chosen policy
[ ] 64-bit logical epochs retained on 32-bit general-purpose target
[ ] no torn concurrent epoch reads where applicable
[ ] stale direct ref uses InvalidReferenceGeneration panic
[ ] proper generation overflow behavior
[ ] checked bounds and reference use dominate access
[ ] no dynamic raw-to-safe fabrication
[ ] shared/mutable reborrow and move semantics preserved
[ ] slice from fixed/dynamic/Arena storage works where ready
[ ] owner/string/Arena descriptor policies lowered only when ready
[ ] storage transition/invalidation barriers preserved
[ ] P17 cleanup/ownership single-owner identity preserved
[ ] P25 call ABI handoff tested when executable
[ ] schema v22 parse/print/verifier installed after prerequisites
[ ] lowering v22 passes staged/registered
[ ] three P26 verifiers registered
[ ] static proof/check-elimination soundness demonstrated
[ ] malformed IR negative fixtures added
[ ] 32/64-bit and constrained target controls run
[ ] cold/warm/cache mutation equivalence tested
[ ] no public MD-070 syntax invented
[ ] no mandatory runtime, GC or LLVM dependency
[ ] existing governance IDs updated with evidence
[ ] check-sec-mlir / llvm-lit controls run where available
[ ] go test ./... and go vet ./... results recorded
```

Record failed/not-run tests accurately. Do **not** claim package acceptance when a selected required representation still has no physical lowering.

## § 59. Required implementation report

Codex must report: exact HEAD/worktree; normative source revisions/MD gates; predecessor capability matrix; affected files; selected profile reference policies; canonical LayoutIDs and cache keys; guarantee/proof taxonomy; exact retained/elided checks; invalidation sources; checked epoch and overflow policy; scalar refs/slices/owner/string/Arena descriptor support; P17 ownership; P25 ABI correspondence; schema v22 ops/attrs/passes/verifiers; malformed-module tests; target profiles; cold/warm validation; exact MLIR/LLVM toolchain version; commands/results for Go, vet, CMake, sec-mlir-opt/lit; governance updates; unsupported gates; deviations; recommendation for P27. No hand-wavy “tests passed” claim without commands and outputs.

## § 60. P27 boundary

Possible next package: **Reference-Aware LLVM Dialect Convergence and Verified Memory Operations**, but only after an audit proves selected v22 representation/lifetime/ABI paths are executable. A target-critical unfinished P24 layout policy or P25 ABIModel should take precedence over chronological package numbers. P27 would lower verified physical checks, descriptors and ABI transport to LLVM dialect/LLVM IR while preserving Sec safety semantics and selected target ABI; it must not decide reference semantics anew.

---

## Synchronization note — 2026-10-20

Installed 2026-10-20. Live-checkout corrections to this book: (1) since
2026-10-19 `sec-mlir-opt` also accepts schema 12 (P16 slices) and the emitter
can select it; default emission remains schema 11, and schemas 13–21 are not
installed. (2) P24 `ResolvedLayout`/`LayoutID`/`sec.repr.*` and P25
`CallABIPlan` do not exist yet, so § 12's "P24 `sec.repr.discharge` is reused"
and every LayoutID reference are dependencies, not reusable code. (3) The
stale-reference path already exists in part (Sema effect, Semantic IR
`ref.is-valid`/`fail.reference-generation`, dialect ops, trap endpoint); P26
reuses it (`package26-todo.md` P26-11, P26-28). (4) § 4's standalone
`P26-D#.md` files are replaced by the project practice: simple choices are
asked of the owner; substantial forks become `MD-###` entries. (5) MD-108
(interface view representation) is an additional open gate for § 27.
