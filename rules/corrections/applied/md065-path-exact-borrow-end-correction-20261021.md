# MD-065 — Path-Exact Borrow-End Placement

- **Status:** Applied 2026-10-21 — normative integration: `rules/memory/borrowing.md` rev 2.3 § 8.1(9)–(14) (§ 8.1(7c) pointer updated), `rules/mlir/packages/sec-mlir-dialect_package15.md` § 50. Implemented 2026-10-22, with a recorded conservative limitation. Sema publishes each last-required-use end with its binding and the statement it precedes (`ReferenceLifetimeEnd.Binding/Before/HolderDepth/EndDepth`). Semantic IR (`internal/ir/semantic/borrow_ends.go`) emits `ref.end-borrow` there in the holder's own statement list, and an arm exit already dominated by that end gets no second marker. Tests: `TestBorrowEndsFollowLastRequiredUse`, `TestLastUseBorrowEndsEmitSchema11` (32/64-bit, with all verifiers) and the updated `TestPackage15PayloadBorrowsEndOnEveryArmExit`. Known hole (verified 2026-10-22): an end that Sema records inside a nested block of the holder's scope is not placed there; that borrow ends conservatively at its scope exit, which § 8.1(8) permits as a tracked implementation limitation. MD-065 removed from missing-decisions.yaml.
- **Created:** 2026-10-21
- **Last updated:** 2026-10-21
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Decision ID:** `MD-065`
- **Canonical correction path:** `rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md`
- **Affected canonical rulebooks:** `rules/memory/borrowing.md`; `rules/memory/lifetime_analysis.md`; `rules/mlir/packages/sec-mlir-dialect_package15.md`
- **Related implementation:** `package15-todo.md` (especially P15-49 and P15-63), P16 reference consumers
- **Replaces:** No existing rulebook; resolves the placement choice recorded as MD-065 in `missing-decisions.yaml`.
- **Integration rule:** Preserve existing paragraph numbers in every affected rulebook. Add this decision as new numbered subparagraphs or revise wording in place without renumbering cited provisions.

---

## § 1. Purpose and scope

**§ 1(1)** This correction closes MD-065, the choice between exact non-lexical borrow-end placement and conservative arm/scope-exit placement of `ref.end-borrow` markers in Semantic IR.

**§ 1(2)** It governs direct borrows, payload borrows created within `match` arms or guards, reborrows, copied shared-reference dependencies, and any other P15 borrow represented by a `BorrowID` and a `LifetimeID`.

**§ 1(3)** It does **not** change Sec source syntax, create runtime borrow tracking, redefine ownership availability, or select a physical reference representation. Sema remains authoritative for borrowing; verified Semantic IR and MLIR consume that decision.

## § 2. Normative decision — last required use

**§ 2(1)** On each reachable control-flow path, a borrow's semantic live range extends through its **last required use**, including uses by surviving dependent holders or obligations, and **ends immediately thereafter** when no future reachable use or retention dependency requires it.

**§ 2(2)** The last required use is not necessarily the last textual use of the original reference variable. It includes derived references, copied shared references, stored/aggregated holders, captures, deferred uses, call retention contracts, and valid returned-reference dependencies.

**§ 2(3)** For an authority whose lifetime is bounded by a local scope (including a non-escaping union/Option/Result payload borrow in a match arm), its live range must end **no later than the relevant scope or arm exit**. The scope exit is a *maximum bound*, **not** the normal chosen end point when the last required use occurs earlier.

**§ 2(4)** If a reference dependency is legally returned or otherwise transferred to a surviving holder, the underlying required authority follows the existing escape/retention rules; a local end marker must **not** falsely terminate that surviving dependency. An illegal escape remains a source-semantic error. This does not relax § 2(3) for non-escaping arm-local payload borrows.

**§ 2(5)** A borrow that is established but is never subsequently used or retained ends at the first semantic point after its creation at which no remaining dependency exists. It must not be artificially kept live until scope exit.

## § 3. Path sensitivity and joins

**§ 3(1)** Sema must derive end points separately for distinct continuing control-flow paths. A borrow may end at different positions in the branches of an `if`, `switch`, `match`, `try`, `select`, short-circuit expression, or another control-flow construct.

**§ 3(2)** If the last required use occurs before an arm exits, Semantic IR must represent an end at that resolved point. A marker placed only on the arm-exit edge is **not conforming** merely because arm exit is a safe upper bound.

**§ 3(3)** A path that returns, propagates an error, terminates, breaks, or continues must end or correctly transfer each applicable live borrow **on that path**. An edge that never reaches a merge contributes no live state to that merge.

**§ 3(4)** When a borrow remains required along at least one *continuing* path into a join, the merged state must conservatively preserve the obligation necessary for subsequent uses. When **no** continuing path has any future required use, it must not remain live at the join merely because different branches were previously visited.

**§ 3(5)** Loops require the same rule at iteration boundaries, `continue`, `break`, backedges, and zero-iteration joins. A borrow from one iteration must not spuriously conflict with a subsequent iteration after its last required use; genuinely loop-carried holders must remain accounted for.

## § 4. Borrow dependencies and reborrowing

**§ 4(1)** Sema must account for every authority-preserving dependency before declaring a borrow dead: `defer`, a live closure capture, a live copied shared reference, an aggregate holder, a derived borrow, a returned reference, and any relevant retention/effect contract.

**§ 4(2)** A child reborrow ends under its own `BorrowID`/`LifetimeID` when its last required use ends. Ending it restores access through a valid parent reference *when* the existing exclusivity, liveness and validity rules allow; the event must not end the parent borrow or create new authority.

**§ 4(3)** Source origin/provenance information alone does not prove that two holders have the same liveness. Neither the existence of a parent reference nor shared origin by itself keeps an otherwise dead child reborrow alive.

**§ 4(4)** `ref.end-borrow` does not destroy the referent, move it, discard it, invalidate its storage, or discharge an ownership or linear-resource obligation.

## § 5. Availability is a different dimension

**§ 5(1)** Ownership availability and borrow liveness are independent facts. A source `Place` may be `Available`, `Unavailable`, `PartiallyAvailable`, or `ConditionallyAvailable` (the last sometimes informally described as “maybe available”) without that fact, by itself, establishing whether a particular borrow remains live.

**§ 5(2)** If continuing branches disagree over whether an owned value was consumed, ownership analysis must preserve `ConditionallyAvailable` as prescribed by `ownership.md`. That uncertainty **must not** cause Semantic IR to keep an unrelated borrow live after every required use and dependency has ended.

**§ 5(3)** Conversely, an owned Place being `Available` does not authorize conflicting access while a genuinely live exclusive or shared borrow restricts it.

**§ 5(4)** Borrow endings must never be used to “repair” ownership availability after a move. Availability is refined by ownership-flow facts, not by `ref.end-borrow`.

## § 6. Sema-to-Semantic-IR contract

**§ 6(1)** Sema is the sole authority for the source-semantic borrow live range. It must make resolved, path-specific borrow-end information available to Semantic IR in a deterministic compiler-owned representation. A normal IR builder must not infer end points independently from a lexical arm boundary, textual occurrence, or a second approximate liveness pass.

**§ 6(2)** Every represented end must identify the borrow being ended and its applicable lifetime identity (`BorrowID` and `LifetimeID`), with a resolved control-flow position or edge. Conditional endings may require explicit edge blocks or multiple path-specific end markers.

**§ 6(3)** The builder may choose another *instruction location* only when it is proven equivalent to the exact semantic boundary on all reachable paths: no intervening operation must see an additional active borrow or a prematurely ended borrow, and no verifier or later lowering may observe different borrowing rights. This permission is not a fallback to routine scope-exit markers.

**§ 6(4)** An end marker is required on each applicable path, but no path may end a borrow more than once before the relevant redefinition/new borrow lifetime. In particular, `match` guard-failure edges must use their own resolved state and must not end a borrow required by an alternate continuing edge.

**§ 6(5)** If Sema cannot establish the necessary path-specific facts, compilation or emission must fail clearly at the relevant semantic boundary, rather than emit a guessed, apparently valid `ref.end-borrow` schedule. No lowering stage may silently supply missing language semantics.

**§ 6(6)** The compiler may retain analysis bookkeeping after the borrow has ended, provided that bookkeeping does not extend source-semantic borrowed authority, reject otherwise valid code, or alter verified IR semantics.

## § 7. Verification and lowering

**§ 7(1)** The Semantic IR verifier must check structural consistency of resolved end markers, identity correspondence, dominating borrow creation where required, path coverage, absence of premature ends, absence of double ends, and absence of use after end. It must reject an IR representation that keeps an otherwise ended borrow active across a conflicting operation.

**§ 7(2)** The verifier does **not** become a second source-level borrow checker: it checks the Sema-provided facts and the integrity of their IR representation. The selected MLIR schema and later lowering must preserve the same semantics without adding runtime borrow counters or inventing new alias permissions.

**§ 7(3)** Existing P15-63 arm-exit markers are an implementation stage, not permission to replace MD-065's non-lexical end rule. Migrate them to Sema-published last-required-use end points where the two differ; keep an arm-exit marker when that is genuinely the semantic end.

## § 8. Required regression and conformance cases

**§ 8(1)** Extend the P15-49 and P15-63 coverage to demonstrate the following **positive** cases:

1. A reference's final required use occurs before the end of its `match` arm; an overlapping mutable access after that use is accepted and the IR borrow has ended before the access.
2. Both branches continue, but each has a different final use; their path-specific ends preserve a valid later join.
3. One branch returns and the other continues; the terminated path does not keep a borrow live at the merge.
4. A guard reads a payload, and its false edge ends the guard/payload borrow at the resolved point without corrupting the next arm's state.
5. Loop iteration-local borrows end before the next incompatible iteration access; genuinely carried dependencies stay live.
6. A child reborrow ends after its last required use; the parent reference regains only its previously valid authority.
7. A copied shared holder, closure capture, `defer` use, or returned dependency retains authority until its actual last required use.
8. A `ConditionallyAvailable` owned Place after a branch does not prolong a borrow whose dependencies are all dead.

**§ 8(2)** Add **negative** tests that reject:

1. Ending the wrong `BorrowID` or `LifetimeID`.
2. Ending a borrow before a reachable dependent use.
3. A missing end on a reachable terminating or continuing edge where one is required.
4. Double end of a single borrow lifetime on a path.
5. Treating a retained/escaping dependency as already dead.
6. Retaining a borrow past its Sema-resolved end so as to create a false later conflict.
7. Conflating conditional owned-value availability with borrow liveness or using an end marker to re-establish a moved value.

**§ 8(3)** Require deterministic Semantic IR emission and preserved high-level reference identity on 32-bit and 64-bit `CompilationPlan` variants. Compare the resolved Sema end-point facts with the emitted `ref.end-borrow` operations, not merely the existence of some end marker per arm.

## § 9. Integration and closure instructions

**§ 9(1)** Integrate § 2–§ 5 into `borrowing.md` § 8 without renumbering its current cited paragraphs; cross-reference `lifetime_analysis.md` §§ 3 and 11 for scope/flow constraints and `ownership.md` § 5 for conditional availability. Preserve the MD-061 and MD-069 decisions already incorporated in borrowing rules.

**§ 9(2)** Integrate § 6–§ 8 into Package 15's authoritative contract for `ref.end-borrow` (notably §§ 49–50 and the union-payload interaction referenced as § 82), the resolved Sema-to-Semantic-IR interface, and its test TODOs. If implementation differs, mark the implementation work remaining rather than weakening this normative decision.

**§ 9(3)** Synchronize the relevant implementation-status fragments and P15/P16 dependency records. Update or remove the `MD-065` entry in `missing-decisions.yaml` **only after** this decision is present in the owning canonical rulebooks and the governing synchronization contract is satisfied.

**§ 9(4)** This correction resolves the **design decision** MD-065. It does not claim that existing code already emits path-exact end markers or that P15/P16 are complete. Such completion requires the regression evidence in § 8.

## § 10. Sources reviewed

**§ 10(1)** `missing-decisions.yaml`, entry MD-065 (repository raw `main` view retrieved 2026-10-21); `package15-todo.md`, P15-49 and P15-63 (retrieved 2026-10-21).

**§ 10(2)** `rules/memory/borrowing.md` revision 2.2, particularly §§ 8.1–8.3 and 10–11; `rules/memory/lifetime_analysis.md` revision 2.0, particularly §§ 3 and 11; `rules/memory/ownership.md` revision 2.1, particularly §§ 4–5 (retrieved 2026-10-21).

**§ 10(3)** This correction records the user's decision on 2026-10-21. A complete Git commit SHA for the current remote `main` was not independently verified; consumers must reconcile these source snapshots with the integration-time `main` HEAD before editing canonical files.
