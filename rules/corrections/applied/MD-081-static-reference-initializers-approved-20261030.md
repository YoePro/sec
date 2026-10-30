# MD-081 — Static References in Compile-Time Static Initializers — Approved Decision

- **Status:** Applied 2026-10-30 — normatively integrated (static.md §§ 5, 15-16, 20, 27; compile_time_evaluation.md §§ 28(6), 36(5); lifetime_analysis.md § 19.2(3)); Sema implemented; Semantic IR rejects a static holding a reference explicitly (§ 9(3)), with materialization tracked under P15-42/P24 and cross-module roots under module static access
- **Created:** 2026-10-30
- **Last updated:** 2026-10-30
- **Decision revision:** 1.0
- **Sec language version:** 0.1
- **Decision ID:** MD-081
- **Intended repository location (pending):** `rules/corrections/md081-static-reference-initializers-correction.md`
- **Decision authority:** Sec language owner; discussion approved 2026-10-30
- **Repository baseline checked:** `YoePro/sec`, `main` at `aa59b4c18c91ed04eef69c7dd0028e41630067f5` (2026-10-30)
- **Primary normative owners:** `rules/declarations/static.md` §§ 5, 15–17, 20, 26–28; `rules/compiler/compile_time_evaluation.md` §§ 9, 28, 35–39, 62–65; `rules/memory/lifetime_analysis.md` §§ 6–9, 12, 19, 26
- **Supporting normative owners:** `rules/memory/references.md` §§ 5–6, 10–11; `rules/memory/borrowing.md` §§ 8, 12–14, 27; `rules/compiler/semantic_ir.md`; `rules/mlir/sec_mlir_dialect.md` (only as required by the established MD-116 contracts)
- **Related accepted decisions:** MD-080 (explicit reference creation), MD-084/MD-085 (reference/Place metadata and returned origins), MD-115/MD-116 (static origin and `sec.place.static`)
- **Tracking:** `missing-decisions.yaml` MD-081; existing governance integration IDs `frontend.static-declarations-members`, `compiler.compile-time-evaluation`, `frontend.lifetime-analysis`, `frontend.borrowing` (and existing Semantic IR/MLIR owner if affected)
- **Replaces:** No previous approved MD-081 decision; supersedes the provisional source-level limitation to module-static roots where it is narrower than the verified static-storage contract below

---

## § 1. Authority and scope

**§ 1(1).** This document records the approved Sec 0.1 resolution of **MD-081**. It defines when an explicit safe reference can be the compile-time-materialized initial value of static storage. It is authoritative for this subject pending integration into the canonical owning rulebooks; those books remain the permanent homes of their respective rules.

**§ 1(2).** The decision covers explicit shared and mutable references, type inference, statically known field and element sub-Places, origin and authority verification, static materialization, and interactions with existing initialization and destruction rules.

**§ 1(3).** It does **not** add implicit reference conversion, a special static reference type, source-level lifetime syntax, a new initialization mechanism, an exception to ownership/borrowing/concurrency rules, or a new runtime startup action. It does not redefine linear types planned for Sec 0.2.

**§ 1(4).** Existing referenced paragraph numbers must remain stable. Amend an existing numbered section in place, or append a new uniquely numbered subsection/paragraph; do not renumber earlier provisions or duplicate their normative owners.

## § 2. Core decision: references are materializable static initializer values

**§ 2(1).** `ref S` is a legal `SemanticCompileTimeRequiredContext` initializer expression when `S` resolves to a verifiably addressable, initialized static-storage Place whose storage identity and reference authority can be established for the resulting program artifact.

**§ 2(2).** `ref mut S` is likewise legal when `S` is mutable static storage **and** the compiler proves the exclusive mutable borrow valid over its required live range, including all uses that can be reached through the resulting static reference holder.

**§ 2(3).** Both explicitly annotated and inferred reference types are supported. The initializer determines the actual reference type under ordinary Sec inference; `ref` produces `ref T`, and `ref mut` produces `ref mut T`.

```sec
let Configuration: int := 42
let ConfigurationRef: ref int := ref Configuration
let ConfigurationRef2 := ref Configuration

let mut Counter: int := 0
let CounterAccess := ref mut Counter
```

**§ 2(4).** The reference initializer denotes the **program static's storage identity**, not the address of a temporary object inside the compiler's CTE evaluator. Its materialization is permitted through verified program symbols/relocations or a target-defined equivalent, never through copying an evaluator-local host pointer.

**§ 2(5).** A reference to an immutable static cannot grant mutable authority. A shared reference to mutable static storage grants no permission to mutate through that reference and does not establish freedom from races.

## § 3. Explicit syntax and inference

**§ 3(1).** In every static initializer, a reference to a Place MUST be written explicitly as `ref place` or `ref mut place`. MD-080's error `S1169` / `borrowing.explicit-ref-required` remains mandatory where an owned Place is incorrectly supplied as an implicit reference outside call arguments.

**§ 3(2).** Type inference of a static reference expression is ordinary Sec type inference. Explicitly writing `ref int` on the destination is optional when inference can establish that exact type. Nothing in this decision permits the inferred type to erase reference authority, origin, or mutability.

**§ 3(3).** No general CTE-created temporary is made static merely because its address is taken in a static initializer. An expression whose target is a computed temporary, invocation-local automatic Place, runtime-mapped resource, invalidated object, or unmaterializable evaluator allocation is rejected.

## § 4. Valid static origins and named storage

**§ 4(1).** A legal root denotes an existing static-storage declaration with a canonical and unambiguous identity. Module-level declarations qualify because their storage is static whether or not the redundant `static` keyword is written.

**§ 4(2).** A type-owned `static let`, or function-local `static let`, is not excluded solely because of its source declaration position. It qualifies where the compiler resolves its persistent, unique static-storage identity and establishes all applicable visibility, initialization, materialization, and borrow requirements. In particular, a function-local static may **not** use invocation-local automatic storage as its referenced root.

**§ 4(3).** Cross-module references require ordinary visibility and unambiguous canonical module-qualified static identity. A compiler stage that cannot yet preserve such linkage must reject its unsupported representation rather than guess or merge identities. Source-name equality does not establish common storage identity.

**§ 4(4).** Static *storage duration* alone is insufficient to authorize a borrow: the target Place must be initialized, available, addressable, and backed by an object whose lifetime and access authority remain valid for the borrow's entire required live range.

## § 5. Field and array-element sub-Places

**§ 5(1).** An initializer may use `ref StaticValue.Field`, `ref mut MutableStatic.Field`, `ref StaticArray[index]`, or corresponding mutable form when the complete Place path can be resolved to a verified static-storage sub-Place during compilation.

**§ 5(2).** For a fixed array, `index` must be established by semantic CTE and proven within bounds. A runtime-dependent index cannot initialize a static reference. A compile-time-known but out-of-bounds index is invalid.

**§ 5(3).** The compiler preserves the root's canonical static identity **and** the complete field/element projection, not merely the source spelling or a numeric offset. Accessing two disjoint struct fields or constant array elements may be proved nonoverlapping under ordinary Place rules; overlapping or unknown projections must remain conservative.

**§ 5(4).** The ordinary addressability, representation, active-union-variant, layout, target, and lifetime rules still apply. A constant index into a dynamically movable or otherwise non-address-stable backing collection is not enough by itself to establish a valid materialized reference.

**§ 5(5).** An indirect, runtime-dependent, or not-yet-materializable reference origin cannot be laundered into a statically valid Place by projecting a field or index from it.

## § 6. Mutable borrowing and exclusive authority

**§ 6(1).** An active `ref mut` borrow conflicts with another active `ref mut` or `ref` borrow of the same or overlapping Place, and with direct access that conflicts under Sec's existing borrowing rules. Two mutable references to **provably disjoint** sub-Places may coexist.

**§ 6(2).** The compiler tracks at least canonical storage identity, Place/projection, reference origin, borrower/holder authority, and borrow live range. The fact that two roots are reached via different names, modules, or aggregate paths does not erase an established overlap.

**§ 6(3).** A mutable reference retained in a static holder is not automatically regarded as expired merely because its declaration's initializer has finished. Borrow lifetime is governed by the holder's actual potential uses and escapes. Where analysis cannot prove safe exclusivity — especially through globally reachable state, separate compilation, or concurrent execution — compilation fails closed rather than treating the borrow as absent.

**§ 6(4).** Ordinary Sec nonlexical liveness permits a new borrow once the prior borrowing authority has *actually ended* and no holder can use it again. This rule must not be used to create two simultaneously usable exclusive references.

**§ 6(5).** A mutable static reference is **not** a synchronization mechanism. Thread/task/interrupt safety remains subject to the established concurrency and execution-context rules.

**§ 6(6).** The following pair of globally retained mutable references is invalid when their required live ranges overlap:

```sec
let mut Counter: int := 0
let First := ref mut Counter
let Second := ref mut Counter // Error: overlapping exclusive borrow
```

**§ 6(7).** By contrast, separately proven sub-Places may be borrowed:

```sec
type Settings struct {
    Width: int
    Height: int
}

let mut Config: Settings := Settings { Width: 100, Height: 200 }
let WidthAccess := ref mut Config.Width
let HeightAccess := ref mut Config.Height
```

The latter requires the usual proof that the two fields are distinct stored Places, not computed properties or overlapping storage.

## § 7. Object lifetime, ownership availability, and destruction

**§ 7(1).** Three separate obligations remain: the storage Place must exist; its currently referenced object/value must remain initialized and valid; and any resource represented by that object must remain valid for every use requiring it. Static storage duration proves none of the latter two by itself.

**§ 7(2).** A referenced value cannot be moved, invalidated, replaced incompatibly, or destroyed while an active borrow requires its existing identity and validity. Standard ownership and partial-availability rules apply equally to static roots and sub-Places.

**§ 7(3).** A static reference holder's destruction and the referent's destruction must follow validated dependency/use relationships. A referent must not be destroyed while a live reference can still be used. Existing static shutdown/target termination rules remain unchanged.

**§ 7(4).** Future Sec 0.2 linear-type features must preserve these same lifetime and ownership guarantees; MD-081 does not postpone current Sec 0.1 reference safety until linear types are introduced.

## § 8. CTE evaluation and materialization boundary

**§ 8(1).** Computing a static *reference identity* is not equivalent to reading the target static's runtime value. Forming `ref SomeMutableStatic` does not authorize semantic CTE to read its mutable runtime contents.

**§ 8(2).** Semantic CTE may evaluate expressions needed to resolve a static Place path (e.g. a constant index) and return a typed symbolic reference target. The compiler materializes it only after validating static root, projection, borrow authority, lifetime, and artifact representability.

**§ 8(3).** Each referenced static's program storage identity is distinct from any evaluator-local object identity. Materialization MUST preserve a valid symbol/relocation or equivalent established by the selected CompilationPlan and target. It MUST NOT embed evaluator host addresses, fabricate a static heap, or perform a hidden runtime initializer to repair a failure.

**§ 8(4).** If semantic evaluation succeeds but target materialization cannot represent the reference, report a materialization/unsupported-feature diagnostic at the applicable stage. Never silently convert it to a raw pointer or a runtime-created borrow.

**§ 8(5).** The pre-existing static initialization dependency and cycle rules in `static.md` § 16 and `compile_time_evaluation.md` §§ 38–39 remain authoritative. This decision does **not** introduce a new exception allowing cyclic initializer graphs, whether or not one edge takes an address.

## § 9. Semantic IR, MLIR, and identity preservation

**§ 9(1).** A materialized reference carries a canonical static origin with exact Place projections and authority (shared or mutable), along with the borrow/lifetime and static-storage facts required by the existing Semantic IR model. Do not rederive identity from a textual identifier, address equality, or an evaluator pointer.

**§ 9(2).** The existing MD-116 `sec.place.static`, `sec.static.global`, `sec.static_id`, and `sec.address_taken` contracts remain authoritative. Address-taking a static must be preserved through the existing source → Sema → Semantic IR → Sec MLIR → lowering pipeline where supported; no second static root mechanism is introduced.

**§ 9(3).** A static sub-Place is represented by canonical Place projection from the verified static root, using already-defined field/element projection and reference-borrow semantics. Where a selected current schema or lowering pass cannot represent the particular projection, reject that path explicitly; do not invent an undocumented operation.

**§ 9(4).** Imported/cross-module static identity and static-origin reference summaries retain their module-qualified provenance. An unsupported cross-module linkage fails closed without falsifying the source-level decision.

## § 10. Compiler diagnostics

**§ 10(1).** Preserve `S1169` for implicit reference creation outside call arguments. An explicit but invalid static reference should receive the existing most specific error family for reference origin, non-addressability, immutability, borrow overlap, lifetime, static CTE legality, or materialization, rather than being reduced to an unhelpful generic "runtime initializer" error.

**§ 10(2).** Diagnostics SHOULD identify the static root and projected Place, why the reference cannot be held, and, for borrow conflicts, the conflicting holder and its relevant live range. Where authority is insufficient, explain how to obtain a legal shared reference or restructure ownership without promising an unsafe workaround.

**§ 10(3).** Preserve existing stable diagnostic identities where already defined. Any new diagnostic identity must be added to the single canonical diagnostics registry and governance, not invented independently by Sema and lowering.

## § 11. Required positive conformance tests

**§ 11(1).** Static shared `ref` to immutable module static; typed and inferred destination; two simultaneously retained shared references with identical canonical origin.

**§ 11(2).** Static shared reference to mutable module static subject to borrow authority; statically retained exclusive `ref mut` to mutable static with proven nonconflicting program uses; reference to a type-owned or function-local static when the compiler has established its canonical persistent storage identity.

**§ 11(3).** Statically addressed struct field and fixed-array element with compile-time-known in-bounds index; distinct nonoverlapping fields/elements with separate mutable references.

**§ 11(4).** Compile-time index calculation, source-file declaration-order independence, valid module-qualified static resolution when the existing import/linkage contract supports it, and preservation of exact static identity and projections into supported Semantic IR/MLIR stages.

**§ 11(5).** Materialization through a valid target symbol/relocation without hidden startup execution or evaluator-address escape; deterministic repeated compilation.

## § 12. Required negative and regression tests

**§ 12(1).** Rejected implicit reference initialization (`S1169`); `ref mut` of immutable static; two overlapping live mutable borrows; shared/mutable conflicts; direct conflicting access; mutation without synchronization where required.

**§ 12(2).** Borrow of automatic local, call temporary, uninitialized/unavailable static, runtime-generated Place, non-addressable computed property, and evaluator-local object that lacks valid program-static materialization.

**§ 12(3).** Runtime-dependent or out-of-bounds static array index; unknown/overlapping projections; non-stable backing storage; invalid union projection; unsupported/nonordinary address-space or target relocation; erroneous cross-module root identity.

**§ 12(4).** Destruction, move, partial invalidation, and replacement while borrowed; static reference retained past a target's permissible teardown; static storage mistaken for thread-local per-thread values.

**§ 12(5).** Existing scalar static initializers and dependency-cycle diagnostics remain valid. Existing function-local static persistence, explicit runtime initialization requirement, no-hidden-startup rule, MD-080 explicit reference diagnostics, and MD-116 static-origin identity verification must not regress.

**§ 12(6).** Regression tests must distinguish source-level rejection from an independently unsupported IR/backend lowering path; do not label the entire language feature implemented because source Sema accepts it.

## § 13. Canonical rulebook integration

**§ 13(1).** Amend `rules/declarations/static.md` §§ 5 and 15 to explicitly admit materializable static `ref` and conditionally `ref mut` initializers; extend §§ 16–17, 20, 26–28 only to connect static dependency, exclusive borrow, destruction, compiler analysis, Semantic IR, and diagnostics. Keep paragraph titles and existing references stable.

**§ 13(2).** Amend `rules/compiler/compile_time_evaluation.md` §§ 9, 28, 35–39, 62–65 as necessary to distinguish symbolic static-reference formation from runtime mutable-static reads, to specify validated static-reference materialization, and to require fail-closed relocation/identity checks. Do not reopen CTE semantics outside this scope.

**§ 13(3).** Amend `rules/memory/lifetime_analysis.md` § 19 and applicable lifetime/reference provisions to state that static origin plus valid object/borrow authority is required; cross-reference existing ownership/borrowing rules. Add supporting cross-references to `references.md` / `borrowing.md` only if needed; avoid duplicating their primary contracts.

**§ 13(4).** Keep MD-080 and MD-116 authoritative for explicit source reference spelling and verified static Place representation. Preserve all existing Semantic IR and MLIR schema numbers and paragraph numbers; add only contract-compatible amendments when needed by supported lowering.

## § 14. Implementation and governance instructions for Claude/Codex

**§ 14(1).** Work **one small implementation point at a time and test it before starting the next**. First synchronize the owning language rulebooks; then update Sema's static initializer validation and borrow-origin proofs, static materialization and dependency handling, and downstream IR/lowering only to the extent of independently verified support.

**§ 14(2).** Reconcile the existing provisional logic at `internal/sema/static.go`, especially the `*ast.RefExpression` case in `validateStaticInitializerExpression`, which currently permits certain module-level static roots under a comment explicitly awaiting MD-081. A syntactic root-name test is not itself a substitute for the canonical Place, sub-Place, and borrow/identity proof required here.

**§ 14(3).** Reuse the existing governance integration records by exact stable ID: `frontend.static-declarations-members` in `governance/declarations.yaml`, `compiler.compile-time-evaluation` in `governance/compiler.yaml`, and `frontend.lifetime-analysis` / `frontend.borrowing` in `governance/lifetime.yaml`; update an existing Semantic IR/MLIR owner if that implementation is touched. No duplicate `MD-081` governance owner should be created.

**§ 14(4).** Record evidence per increment: affected source paths, positive and negative tests, commands executed and their actual results, remaining unsupported cases, and source/IR/backend status. Retain honest `partial`/`remaining` status where necessary; no assertion that P15 or the full static infrastructure is complete follows from this decision.

**§ 14(5).** Only after normative rulebook integration **and** governance synchronization should `MD-081` be removed/closed from `missing-decisions.yaml` according to its current policy. Once applied, move this correction to `rules/corrections/applied/` and append the real application date (`-YYYYMMDD.md`); do not archive it merely because the owner approved it.

## § 15. Final accepted decision

**§ 15(1).** **Approved:** explicit static `ref`/`ref mut` initializers to verified static roots and statically addressable sub-Places, including type inference, subject to full existing CTE, borrow, lifetime, ownership, synchronization, destruction, and target materialization constraints. Concurrent overlapping exclusive references remain forbidden; disjoint projections may have distinct mutable references.

**§ 15(2).** **Not claimed:** completed code implementation, new source-level lifetimes or linear-type semantics, general CTE pointer escape, dynamic-address materialization, hidden startup initialization, relaxed static dependency cycles, or an unconditional guarantee that all backends already support the approved cases.
