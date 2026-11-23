# MD-136 — Arena Default Construction and Empty Initial State

- **Document ID:** MD-136
- **Document kind:** Approved language-design correction; normative synchronization instructions
- **Sec language version:** 0.1 (with Sec 0.1.1 design compatibility)
- **Created:** 2026-11-23
- **Updated:** 2026-11-23
- **Document revision:** 1.0
- **Decision status:** **Owner approved on 2026-11-23**
- **Integration status:** **Not integrated into canonical rulebooks or governance; implementation and tests not claimed**
- **Repository baseline reviewed:** `YoePro/sec` `main` at `5b2e5b294488dce2380c019572c67c5229628e20`
- **Canonical owners:** `rules/memory/arena.md`, `rules/types/default_values.md`, `rules/mlir/packages/sec-mlir-dialect_package19.md`
- **Implementation governance:** `lowering.sec-mlir-package19` (`governance/lowering.yaml`), with existing frontend/default-value, Arena, and allocation/target-profile owners where their contracts are implicated
- **Related decisions:** MD-034 (decided), MD-139 (decided, implementation pending), MD-140 (open), Sec 0.1.1 Owner/Arena design (separate work)

> This correction records a concluded **language choice**. It does not claim that source acceptance, Semantic IR, Sec MLIR schema 15, runtime allocation, native execution, governance integration, or conformance tests are complete. Integrate without renumbering existing canonical paragraphs.

---

## § 1. Decision and scope

**§ 1(1)** `Arena` is `Defaultable`. Its canonical default is a **fresh, valid, empty Arena** with zero initial byte capacity and no allocated or borrowed backing storage.

**§ 1(2)** Both of the following source forms are valid and have equivalent default-construction semantics:

```sec
let mut first: Arena
let mut second := Arena {}
```

**§ 1(3)** `Arena {}` is an explicitly recognized **builtin Arena default construction**, not ordinary construction of an exposed user-defined struct and not an invitation to initialize or access implementation fields.

**§ 1(4)** The default constructor is infallible and does not allocate backing storage. It is valid in `@noAlloc`, `noalloc`, freestanding/BareMetal, and other target/profile contexts that support ordinary Arena values and their allocation-free initialization.

## § 2. Identity, ownership and initial state

**§ 2(1)** Each executed Arena default initialization establishes a **distinct, new ArenaDomain**. Two independently default-initialized Arena values never silently share the same domain, even when both have zero capacity.

**§ 2(2)** The default Arena is live and owns/controls its own ArenaDomain. It owns no backing allocation and borrows no backing buffer. Its initial logical capacity is exactly zero bytes; there are no live allocations.

**§ 2(3)** The Arena begins at its normal initial valid epoch and state version. Their concrete representation and the epoch-exhaustion strategy remain governed by existing Arena/reference rules and open MD-140; this correction introduces no epoch counter value, wraparound rule, rekey strategy, or exhaustion trap.

**§ 2(4)** `Arena` remains move-only and non-copyable, with its existing destruction and terminal `Release` behavior. Defaultability does **not** imply copyability or shared ownership. Arena moves preserve the domain identity; independent default initializations do not.

**§ 2(5)** `Reset`, `Release`, and normal scope destruction are legal for an empty default Arena under the existing rules. None may invent backing ownership or reclaim backing that was never acquired. `Release` remains terminal and must not be repeated by automatic destruction.

## § 3. Frozen growth and provider policy

**§ 3(1)** Default construction determines the Arena's immutable growth/backing policy from the **already resolved and frozen CompilationPlan**, including the selected target/profile and positively verified provider capabilities. The Arena does not select a provider during a later allocation attempt.

**§ 3(2)** Where the plan positively establishes a compatible provider and a legal non-relocating growth strategy, the default Arena may acquire backing on a subsequent operation that requests positive storage. Such an operation retains its existing fallible, allocation-effect, capacity, alignment, lifetime and provider-failure contracts.

**§ 3(3)** Where the plan has no active, positively supported compatible provider (including `noalloc`), default construction nevertheless succeeds: the Arena remains a valid **non-growing empty Arena**. A subsequently requested operation requiring unavailable positive backing is rejected at compile time under the existing profile/allocation readiness rules. Zero-storage operations may remain legal where their own established contracts permit them.

**§ 3(4)** `Unknown` provider availability is not evidence of capacity. The compiler must not invent a platform allocator, infer a host heap, silently activate a disabled capability, or switch providers to satisfy an operation.

**§ 3(5)** The policy cannot be changed for a live ArenaDomain. Provider identity, authority, reclamation strategy and growth capability are not inferred from the presence of a lexical `Arena` variable and must remain compatible with MD-034/MD-139.

**§ 3(6)** A provider that requires a fallible allocation or reservation *merely to create the default Arena* cannot be used for that eager operation. The default constructor must remain infallible and backing-allocation-free. Any supported positive backing acquisition occurs later through the separately fallible operation.

## § 4. Relationship to explicit constructors

**§ 4(1)** Existing explicit constructor contracts remain unchanged:

```sec
let mut supplied := Arena.FromBuffer(ref mut buffer)
let mut fixed := try Arena.WithCapacity(4096)
let mut growable := try Arena.Growable(4096)
```

**§ 4(2)** `Arena.FromBuffer` uses explicitly borrowed backing, including its existing valid zero-length case; default `Arena {}` borrows no backing at all.

**§ 4(3)** `Arena.WithCapacity` expressly requests owned fixed backing and remains fallible. `Arena.Growable` expressly requests an initial growable backing configuration and remains fallible. `Arena {}` requests neither initial backing nor initial positive capacity, and is **not** a source-level alias of `Arena.Growable(0)`.

**§ 4(4)** Existing rules for Arena.New/Alloc, Reset, Release, borrowing, ownership, validity, effects, default element initialization and cleanup are retained. This correction adds no allocator or reclamation API and does not settle the separate Sec 0.1.1 Owner/Box prepared-reservation protocol.

## § 5. Default-value integration

**§ 5(1)** `rules/types/default_values.md` must classify `Arena` as `Defaultable` with exactly the default described in §§ 1–3.

**§ 5(2)** Ordinary default-initialization contexts, including an omitted defaultable struct field, may construct their own fresh ArenaDomain according to canonical default rules. They do not share an Arena or copy one from another default initializer.

**§ 5(3)** Compiler-internal representation may differ by target and implementation, but it must preserve a fresh domain, zero capacity, allocation-free creation, frozen growth capability and exact-once lifecycle semantics.

## § 6. Compiler and lowering contract

**§ 6(1)** Parser/Sema must recognize the builtin `Arena {}` form and omitted Arena initialization without treating Arena as a normal user struct, synthesizing unverified physical fields, or selecting backing through a heuristic.

**§ 6(2)** Sema must publish a resolved default-Arena creation plan with at least the fresh ArenaDomain, initial epoch/state, no backing allocation, zero capacity, frozen profile/growth classification, provider evidence or explicitly absent provider, and cleanup obligations. Do not fabricate a positive provider fact.

**§ 6(3)** Semantic IR and Sec MLIR P19 must represent this default creation distinctly enough to preserve the allocation-free initial state and profile-controlled future growth. Any missing layout/provider/ABI/schema facts remain explicit unresolved lowering boundaries; no partially accepted native module or silent fallback.

**§ 6(4)** Compiler and LSP diagnostics must distinguish an **allowed empty default** from an **invalid positive-storage operation on that default**, showing the selected Target/Variant, policy, requested operation and missing/inactive provider capability according to canonical diagnostics rules.

## § 7. Required conformance test descriptions (not run)

- **MD-136-T01:** `let mut arena: Arena` and `Arena {}` both type-check and produce equivalent zero-capacity, allocation-free state.
- **MD-136-T02:** Two independent defaults have distinct ArenaDomain identities; moving an Arena preserves its own identity and invalidates the moved-from source Place.
- **MD-136-T03:** `Arena {}` is legal under a resolved `noalloc` profile and in `@noAlloc` code without hidden heap/runtime use.
- **MD-136-T04:** Under `noalloc`, a positive-storage Arena allocation without compatible explicitly supplied backing is diagnosed at compile time; default construction itself remains legal.
- **MD-136-T05:** Under a resolved provider-capable profile, default construction performs no backing allocation; a later positive-storage request uses exactly the verified frozen provider and existing fallible result channel.
- **MD-136-T06:** Unknown, disabled, incompatible and ambiguous providers cannot become implicit backing. Explicit profile errors and operation errors remain distinct.
- **MD-136-T07:** Independent default Arena fields in distinct default-initialized structs establish distinct domains; no implicit copy or shared backing.
- **MD-136-T08:** Empty default Reset, Release, move and scope destruction preserve existing lifecycle and exact-once cleanup; there is no invented backing reclamation.
- **MD-136-T09:** `Arena.FromBuffer` with zero-length backing remains valid and semantically distinct from default Arena with no backing; explicit `WithCapacity`/`Growable` retain their own fallibility.
- **MD-136-T10:** Source → Semantic IR → schema-15 MLIR and native acceptance once these stages are supported; until then the missing stage fails closed and is not reported as implemented.
- **MD-136-T11:** No dependency on a chosen MD-140 exhaustion strategy is silently introduced by default creation; Reset's exhaustive behavior remains independently blocked where required.

## § 8. Normative and governance synchronization

**§ 8(1)** Append (do not renumber) authoritative clauses to `rules/memory/arena.md` for default construction, fresh ArenaDomain, no backing, no allocation, frozen policy, and interaction with the explicit constructors and lifecycle.

**§ 8(2)** Append an `Arena` builtin default entry and associated struct-default consequence to `rules/types/default_values.md`; retain its existing requirements that defaults are valid, deterministic and fully initialized.

**§ 8(3)** Synchronize `rules/mlir/packages/sec-mlir-dialect_package19.md`, P19's normative synchronization inputs and `package19-todo.md` with the decided default constructor behavior. Do not imply that schema 15 or physical backing allocation is now implemented.

**§ 8(4)** Register/adjust implementation actions and the test descriptions in existing governance owners, especially `lowering.sec-mlir-package19`; update `missing-decisions.yaml` MD-136 from open to **decided-implementation-pending** only after the canonical normative integration is performed and tracked.

**§ 8(5)** MD-034 and MD-139 remain **decided**, with their separate implementation/provider-catalog obligations. MD-140 remains **open**, and must not be implicitly decided by this correction. Preserve existing Sec 0.1.1 ownership/slot design separately.

**§ 8(6)** A correction may be moved to `rules/corrections/applied/` after (i) all normative text is synchronized into canonical books, (ii) implementation actions are recorded in the appropriate governance fragments, and (iii) specified tests are documented. Code need not already implement the requirements and tests need not pass for this housekeeping move; all implementation/test statuses must be truthful.
