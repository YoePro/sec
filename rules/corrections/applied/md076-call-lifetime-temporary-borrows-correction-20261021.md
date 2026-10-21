# MD-076 — Call-lifetime borrowing of owned temporaries and function chaining

- **Status:** Applied 2026-10-21 — integrated into the owning rulebooks and implemented in Sema and package-15 Semantic IR/schema-11; see the application note below.
- **Created:** 2026-10-21
- **Last updated:** 2026-10-21
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/corrections/pending/md076-call-lifetime-temporary-borrows-correction-20261021.md`
- **Replaces:** None. Adds an explicit specialized exception to existing borrowing and lifetime rules.
- **Decision ID:** `MD-076`
- **Decision authority:** Sec language owner, accepted 2026-10-21
- **Implementation focus:** SEC-MLIR Package 15; preserve compatibility with subsequent packages
- **Repository status:** Applied 2026-10-21; the MD-076 register entry has been removed.

---

## § 1. Purpose and decision

**§ 1(1)** Sec 0.1 permits a freshly produced, owned temporary value to be borrowed for a function argument or instance-method receiver when the selected callable requires `ref T` or `ref mut T`. This permission includes nested calls and method/function chaining.

**§ 1(2)** This is a narrowly scoped *call/operation lifetime* exception to the general requirement that explicit source-level borrowing target a reusable addressable Place. It does not make arbitrary computed expressions generally borrowable by explicit `ref` / `ref mut` syntax.

**§ 1(3)** Both shared and mutable borrowing are permitted. A method may mutate a freshly created temporary even when the temporary is not bound to a named mutable local. The operation's usefulness or externally observable effects are not a condition of borrow legality.

**§ 1(4)** The seven decisions approved by the owner are normative in §§ 2–9 below: compiler-materialized temporary Place; shared/mutable authority; last-required-use borrow termination; full-expression temporary ownership/cleanup; no escaping dependency; no generic `ref Factory()` syntax; and explicit Semantic IR ownership/borrow/cleanup facts.

**§ 1(5)** No new surface syntax, user-written lifetime annotations, borrow locks, garbage collection, mandatory heap allocation, or general temporary-lifetime extension is introduced.

## § 2. Scope and canonical source behavior

**§ 2(1)** The exception applies only when an already resolved function or instance-method call requires an owned expression result as a borrowed argument or receiver. The call's selected parameter/receiver authority must be known before Sema commits the borrow.

**§ 2(2)** For example, these expressions are permitted when their selected signatures are compatible:

```sec
Read(CreateCounter())
Bump(CreateCounter())
CreateCounter().GetCount()
CreateCounter().Increment()
let row := QuerySingle().GetRow()
```

The function and type names in this section are illustrative application APIs, not declarations introduced into Sec core or the standard library.

**§ 2(3)** The rule is identical for a free-function borrowed parameter, an explicitly written method receiver, an implicit `self` receiver, and a receiver forwarded through ordinary instance-method dispatch, subject to the existing receiver authority rules.

**§ 2(4)** A temporary may be an intermediate result in a longer, nested call expression. Each temporary that needs borrowed addressability has its own semantic owner, Place identity, and lifetime boundary. Evaluation does not duplicate a subexpression or invoke a factory twice.

**§ 2(5)** Owned results in a chain and borrowed results in a chain are not equivalent. The return type and return-origin summary of each call determine whether the next stage works on a new owner or on storage borrowed from an earlier stage.

## § 3. Materialization and Place identity

**§ 3(1)** Sema may designate a fresh, owned expression result as a compiler-materialized temporary Place for the sole purpose of the selected call/receiver borrow. This makes the value addressable for the borrow without introducing a user-visible local binding.

**§ 3(2)** The temporary Place is an owned semantic object with a stable identity for the duration of that evaluation. The reference denotes that Place and carries its source provenance, storage/validity dependencies, borrow kind, and lifetime bound.

**§ 3(3)** Materialization does not permit Sema or a lowering pass to manufacture a safe reference into an SSA-only value that has no valid backing Place or lifetime. A compiler may optimize away physical storage only if the language-level Place, aliasing, evaluation, and cleanup semantics remain equivalent.

**§ 3(4)** Temporary materialization is not itself a request for heap allocation. Existing allocation and effect contracts of the value-producing and called functions remain unchanged.

## § 4. Shared and mutable authority

**§ 4(1)** A temporary passed to `value: ref T` provides the ordinary shared authority. A fresh, exclusively owned temporary passed to `value: ref mut T`, or used as the receiver of a mutating method, may provide ordinary exclusive mutable authority.

**§ 4(2)** A mutable borrow of a fresh temporary does not confer ownership on the callee, permit moving an owned move-only value out through the borrow, or elevate authority from an immutable existing Place or shared reference.

**§ 4(3)** For an existing named binding, the normal mutability, source-Place availability, and borrow compatibility requirements continue to apply. This exception does not change `ref mut existingImmutable` into a valid borrow.

**§ 4(4)** Borrow overlap, reborrowing, call-site borrow rules, parameter contracts, and returned-reference origins are checked exactly as for an ordinary borrowed Place.

## § 5. Borrow live range: MD-065 compatibility

**§ 5(1)** Each borrow of a materialized temporary remains live through its last required use and any derived dependence on the path being analyzed. Its live range is bounded by the validity of the temporary referent.

**§ 5(2)** Sema determines path-specific borrow ends, including nested calls, branches, short-circuit paths, and any valid derived borrow. Semantic IR records the semantically required end points; lexical exit is not a substitute for last-required-use analysis.

**§ 5(3)** A borrow may end before the temporary owned object's full-expression cleanup. Borrow-end and owner destruction are distinct events and must not be conflated.

**§ 5(4)** `defer`, retained captures, returned references, views, and other dependent holders do not override the temporary's maximum allowed lifetime. If those dependencies require the temporary to remain valid beyond that boundary, the source use is invalid.

## § 6. Owned lifetime and cleanup

**§ 6(1)** A still-owned temporary survives through its enclosing full expression, subject to the existing ownership-transfer rules. Borrowing the temporary neither destroys it immediately after the called method returns nor extends it into the next unrelated statement.

**§ 6(2)** Remaining successfully constructed owned temporaries are cleaned in reverse successful creation order at the full-expression boundary, preserving left-to-right argument evaluation and every observable cleanup ordering requirement of `destruction.md` § 8.

**§ 6(3)** If a temporary's ownership is legally transferred into a consuming receiver/argument or an owned return result, its former owner must not destroy it. Borrowed calls by themselves do not transfer ownership.

**§ 6(4)** Failure during argument preparation, error propagation, a branching expression, or another abnormal control-flow edge cleans exactly the still-owned successfully created temporaries whose lifetimes end on that edge. No duplicated evaluation, premature transfer, leaked owner, or double destruction is permitted.

**§ 6(5)** The compiler must preserve observable effects, initialization success boundaries, storage-validity checks, and destructor ordering even when optimizing away temporary storage or borrow bookkeeping.

## § 7. Returned results and chaining

**§ 7(1)** Function chaining is an explicit language goal and must not be blocked solely because an intermediate owned result lacks a user-visible binding.

**§ 7(2)** In:

```sec
let row := QuerySingle().GetRow()
```

if `GetRow()` returns an **owned `Row`**, the owned result is received by `row` and follows ordinary return/ownership-transfer rules. The temporary receiver created by `QuerySingle()` is independently cleaned when its remaining ownership lifetime ends.

**§ 7(3)** If `GetRow()` returns `ref Row`, `ref mut Row`, or a non-owning view, Sema must apply the instantiated returned-reference origin and storage-dependency facts. When the returned object depends on the temporary query receiver, that result cannot be retained in `row` beyond the receiver's valid lifetime.

**§ 7(4)** A borrowed result of an intermediate call may be used by a later operation *within the same enclosing full expression* provided every stage is valid, the original temporary survives until the last required use, and no reference/view dependency escapes the permitted lifetime.

**§ 7(5)** Conversely, when a returned borrow provably denotes independent, longer-lived storage rather than the temporary receiver, existing reference-origin and lifetime rules govern that independent storage. A borrowed result must not be rejected merely for originating in a call expression if its proven referent does not depend on the temporary.

**§ 7(6)** The exact result of `QuerySingle().GetRow()` depends on the declared API. MD-076 guarantees that the receiver can be temporarily borrowed; it does not promise that every conceivable `GetRow()` signature produces a safely storable result.

## § 8. Explicit-borrow restriction retained

**§ 8(1)** The existing general restriction is unchanged. For example, a standalone attempt to create and retain a borrow of a computed owned temporary is invalid:

```sec
let borrowed := ref CreateCounter()      // Invalid
let writable := ref mut CreateCounter()  // Invalid
```

**§ 8(2)** Compiler materialization for a call does not make `ref (left + right)` or a computed property generally addressable. Existing non-addressable-Place diagnostics remain applicable outside this exception.

**§ 8(3)** Source must not obtain a persistent safe reference by disguising temporary borrowing as a nested call, a field projection, a reference-returning method, a closure capture, or a returned view.

**§ 8(4)** Named local values remain the ordinary explicit way to obtain a borrow that needs to outlive a single call expression, without extending the general borrow syntax.

## § 9. Sema and Semantic IR contract

**§ 9(1)** Sema is authoritative for selected callable signature, temporary ownership, Place identity, receiver/argument borrow kind, return-origin dependencies, last-required-use ends, and full-expression cleanup boundary.

**§ 9(2)** The resolved use plan must distinguish (a) owned temporary creation/materialization, (b) ordinary reference borrow from that Place, (c) argument/receiver passing and any reborrows, (d) dependent returned reference/view results, (e) borrow termination, (f) ownership transfers, and (g) remaining-owner cleanup.

**§ 9(3)** Semantic IR lowers all ordinary supported call/receiver cases from these resolved facts. It must not continue rejecting a valid source-level temporary receiver merely because its original AST expression was not a reusable named Place.

**§ 9(4)** The Semantic IR builder and verifier must use the existing canonical Place, reference, ownership, and cleanup vocabulary where adequate; this correction does not itself invent a new public operation or change a dialect schema.

**§ 9(5)** The verifier checks that each borrowed temporary has a valid originating owner and Place, that calls use compatible authority, that dependent uses are in range, and that every still-owned temporary has correct path-sensitive terminal cleanup. It does not redo the entire source-language borrow checker or invent returned-reference origins.

**§ 9(6)** P15 implements the safe Place/reference semantic core according to its package boundaries. Later implementation packages retain ownership of non-trivial whole-object copying, replacement, destruction details, and capabilities outside P15. A deferred implementation feature must not be presented as forbidden Sec source.

## § 10. Diagnostics

**§ 10(1)** Do not issue `S1168` (`borrowing.non-addressable-place`) merely because a compatible call/receiver borrow originates in a fresh owned temporary. The diagnostic remains valid for prohibited general explicit borrowing and genuinely non-addressable expressions.

**§ 10(2)** A temporary escape that would outlive its referent must be reported as a reference-lifetime/origin error. The message must identify the creating expression, the bounded lifetime, and the retention or use requiring a longer lifetime, with a viable local-binding or owned-return remedy where applicable.

**§ 10(3)** Reject lack of mutable authority, borrow conflicts, invalid returned-reference origins, and unsupported target capabilities under their existing respective diagnostic categories. Do not reuse `S1168` for unrelated failures.

**§ 10(4)** Diagnostic and recovery behavior must remain stable through call chaining and nested calls; a later error must not cause a misleading earlier error asserting that temporary borrowing is categorically illegal.

## § 11. Conformance and regression matrix

**§ 11(1)** Required positive cases, assuming compatible test-declared functions/methods:

| ID | Scenario | Required result |
|---|---|---|
| MD076-P01 | `Read(CreateCounter())`, shared parameter | Accepted; one temporary and shared borrow |
| MD076-P02 | `Bump(CreateCounter())`, mutable parameter | Accepted; mutable temporary authority |
| MD076-P03 | `CreateCounter().GetCount()` | Accepted shared receiver |
| MD076-P04 | `CreateCounter().Increment()` | Accepted mutable receiver even if result unused |
| MD076-P05 | `QuerySingle().GetRow()` returning owned `Row` | Accepted; transferred owned result |
| MD076-P06 | Borrowed intermediate result consumed by next call within one full expression | Accepted if lifetime/origin valid |
| MD076-P07 | Fresh temporary as nested argument in deeper call chain | Accepted; evaluate producers once |
| MD076-P08 | Multiple temporary arguments | Left-to-right production; reverse remaining cleanup |
| MD076-P09 | A returned borrow proven to refer to independent live storage | Accepted on existing origin/lifetime rules |
| MD076-P10 | Full-expression chain with branch/short-circuit | Only reached temporaries are created/cleaned |

**§ 11(2)** Required negative cases:

| ID | Scenario | Required result |
|---|---|---|
| MD076-N01 | `let borrowed := ref CreateCounter()` | Reject general explicit computed-temp borrow |
| MD076-N02 | `let writable := ref mut CreateCounter()` | Reject general explicit computed-temp borrow |
| MD076-N03 | Retain `ref Row` from `QuerySingle().GetRow()` when origin is the temporary query | Reject escaping dependency |
| MD076-N04 | Return a reference into a receiver temporary | Reject escaping dependency |
| MD076-N05 | Store a view of temporary backing storage beyond its lifetime | Reject escaping dependency |
| MD076-N06 | Capture a temporary-dependent borrow in a surviving closure | Reject escaping dependency |
| MD076-N07 | Borrow conflicting overlapping Places in the same chain | Reject borrow conflict |
| MD076-N08 | Treat an existing immutable owner as mutable only because call uses `ref mut` | Reject authority elevation |
| MD076-N09 | Use borrow after temporary cleanup via a derived reference | Reject lifetime invalidity |
| MD076-N10 | A computed property masquerading as stored addressable Place | Reject unless the expression itself returns a valid safe reference according to its declared contract |

**§ 11(3)** Required lowering/invariant checks:

1. Sema, verified Semantic IR, MLIR, and execution agree on acceptance and effects.
2. No duplicate evaluation of the source expression or additional observable constructor invocation.
3. Canonical temporary Place/owner identities and valid borrow provenance survive through lowering.
4. Path-specific borrow ends follow MD-065; no borrow ends early or blocks a subsequent valid use through overlong authority.
5. No premature destruction, missing cleanup, double destruction, or destruction after ownership transfer.
6. Fallible later argument preparation preserves the earlier temporary's cleanup obligation until the appropriate terminal edge.
7. No hidden heap allocation, physical layout assumption, or runtime borrow tracking is introduced by temporary materialization.
8. P15 source → Sema → Semantic IR → schema-appropriate MLIR evidence covers function arguments and method receivers.

## § 12. Normative integration instructions

**§ 12(1)** Integrate the exception into `rules/memory/borrowing.md` without replacing the general rule in § 5.2(1)–(3). Add a stable numbered cross-reference subordinate to § 5.2(4), or another non-renumbering insertion, defining the exact call/receiver exception in §§ 1–8 of this correction.

**§ 12(2)** Synchronize `rules/memory/lifetime_analysis.md` § 10 with the same temporary-owner/full-expression boundary, returned-origin check, and path-sensitive use requirements. Preserve existing section and paragraph identities.

**§ 12(3)** Synchronize `rules/memory/destruction.md` § 8 only as needed to clarify that allowed call borrows do not alter full-expression destruction or its ordering; avoid duplicating conflicting cleanup rules.

**§ 12(4)** Synchronize the relevant call rules in `rules/declarations/functions.md`, and instance-method/receiver rules in the owning implementation rulebook, consistently across local and parameter references. Do not invent different borrowing semantics solely for methods.

**§ 12(5)** Synchronize the P15 package implementation contract, its tests, the canonical Semantic IR/MLIR normative surfaces, and the current governance fragment(s). Respect the package boundary for P17 and later work.

**§ 12(6)** Update `missing-decisions.yaml` only after this correction has been placed under the repository's canonical *applied* correction path or integrated directly into the owning books, and all dependent governance entries have been synchronized. The language decision is accepted, but the backlog record must not be removed prematurely.

**§ 12(7)** Do not treat the illustrative `QuerySingle`, `GetRow`, `CreateCounter`, `Read`, or `Bump` symbols as newly defined language or standard-library APIs. Tests shall declare concrete compatible signatures and fully defined implementations or use established fixtures.

## § 13. Explicitly unchanged contracts

**§ 13(1)** General explicit `ref place` / `ref mut place` syntax, existing reborrow semantics (MD-061), and implicit borrowing at call sites remain unchanged except for this newly specified source-Place category.

**§ 13(2)** The distinction between borrow authority and ownership responsibility is unchanged. `ref mut` is not a whole-owner move capability.

**§ 13(3)** Return-reference provenance, storage generation/epoch validity, caller-versus-callee cleanup responsibilities, and full-expression cleanup ordering remain owned by their current rulebooks.

**§ 13(4)** MD-063 (explicit `*` dereference of complete referenced values) is an independent decision/review. MD-076 does not require MD-063 to be integrated and does not decide whole-value dereference syntax.

**§ 13(5)** Syntax and semantics of fallible chaining, `try`, short-circuiting, consuming receivers, target-specific operation capabilities, and future allocation features remain governed by their respective rulebooks; this correction only authorizes call-lifetime borrowed temporary Places within those rules.

---

## References examined

- `missing-decisions.yaml`, MD-076.
- `rules/memory/borrowing.md`, §§ 5.2, 8.1, 11, 15.
- `rules/memory/lifetime_analysis.md`, §§ 9–11, 14.
- `rules/memory/destruction.md`, § 8 and its ownership-transfer/cleanup provisions.
- `rules/declarations/functions.md`, §§ 16–18 and 22.
- `package15-todo.md`, P15-47 and P15-49.

**Recorded decision:** Approved by the Sec language owner on 2026-10-21, including the express requirement that function chaining is supported. **Integration state:** pending; no repository files were modified by producing this document.

---

## Application note (2026-10-21)

**Integrated into:**
- `rules/memory/borrowing.md` rev 2.4: § 5.2(4) cross-reference and new § 5.4(1)–(7).
- `rules/memory/lifetime_analysis.md` rev 2.1: § 10.2(3).
- `rules/memory/destruction.md` rev 2.2: § 8(7).
- `rules/declarations/functions.md` § 18.
- `rules/declarations/impl.md` § 8.
- `rules/compiler/semantic_ir.md` § 45(9b).

**Implementation:**
- **Sema (new file):** `internal/sema/temporary_borrows.go` handles the temporary Place, the borrow plan, the `$temporary:<site>` returned-reference origin and the retention check.
- **Sema hooks:** `reference_return_origins.go`, `reference_plans.go` (new `ResolvedReferencePlan.Temporary`), `receiver_facts.go`, `resolved_calls.go`, `analyzer.go` (`methodReceiverInfo.Expr`, let/assignment retention) and `escape_diagnostics.go`.
- **Semantic IR:** `internal/ir/semantic/reference_lowering.go` (`lowerTemporaryBorrow`), `reference_use.go` (dereference of a reference value) and `builder.go`.

**Tests:**
- `internal/sema/temporary_borrows_test.go`
- `internal/ir/semantic/temporary_borrows_test.go`
- `internal/lowering/secmlir/temporary_borrows_emit_test.go`, which passes the three schema-11 verifiers.

**Matrix coverage:**
- **Accepted:** P01–P05, P06 (`Use(Field(CreateCounter()))`, `*Field(CreateCounter())`, `*CreateCounter().CountRef()`), P07 and P08.
- **Rejected:** N01 and N02 (S1168); N03, N04 and N05 (S1128, store into a field); N08 (S1165).
- **Indirect:** N06 is covered because the dependent reference cannot be bound to a name before it is captured. N07 and N09 follow the existing borrow and lifetime checks.

**Defects found while applying:**
1. A mutating method on a fresh temporary was not accepted. The overload was rejected with an empty message, so `CreateCounter().Increment()` typed as invalid with no diagnostic. The register entry had recorded this as "accepted".
2. The same check ignored the authority of projected receivers, so `self.Channel.SetDutyPermille(...)` was silently invalid.

Both are fixed. A rejected mutable or consuming receiver now always reports an error.

**Known holes (verified 2026-10-21):**
- Full-expression cleanup of a temporary that is not trivially destructible is an explicit Semantic IR lowering boundary until P17 (§ 9(6)).
- The native track (`emit-sec-mlir`, `build`) still stops at P14 (MD-086), so these programs reach schema-11 only through package-15 builds.
- P10 (branches and short-circuits) relies on the ordinary per-path evaluation of the producing expression and has no dedicated regression test yet.
