# MD-064 and MD-069 — Non-Addressable Borrow Diagnostics and Origin-Aware Last-Use Liveness

- **Status:** Applied 2026-10-16 — integrated into `rules/tooling/diagnostics.md` § 24(4b)–(4c), `rules/memory/borrowing.md` §§ 5.2(5)–(7), 8.1(7a)–(7c) and `rules/memory/lifetime_analysis.md` § 14.7; S1168 help refined; tests `TestMD064NonAddressableBorrowGuidance`, `TestMD069OriginRootAndLastUse`; governance `lifetime.yaml`; MD-064 and MD-069 closed. MD-065 (end-borrow marker placement) remains open.
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1 (compatible in principle with planned Sec 0.2 linear types; no 0.2 obligations decided)
- **Proposed canonical path:** `rules/corrections/pending/md064-md069-borrow-diagnostics-origin-liveness-correction-20261016.md`
- **Decision IDs:** `MD-064`, `MD-069`
- **Primary rulebooks:** `rules/tooling/diagnostics.md`, `rules/memory/borrowing.md`
- **Related rulebooks:** `rules/memory/references.md`, `rules/memory/reference_model.md`, `rules/memory/lifetime_analysis.md`, `rules/compiler/semantic_ir.md`, `rules/declarations/properties.md` (where applicable), match/payload lifetime rules, compiler testing and LSP rules
- **Amends prior correction:** `rules/corrections/applied/md015-md061-duration-reborrow-correction-20261015.md` (use the repository's actual current path) — especially MD-061 § 4. The present correction narrows the interpretation of origin-sharing holders; it does not supersede explicit-reborrow syntax or general optimizer safety.
- **Replaces:** No existing numbered paragraphs. Amend normative rulebooks in place, preserving all stable paragraph IDs and existing valid rules.
- **Governance:** Reconcile existing per-book governance fragment IDs; do not duplicate implementation entries or equate a language-owner decision with implementation validation.
- **Source baseline:** User-provided `missing-decisions.yaml`, updated 2026-10-16 (locally ahead of repository synchronization), entries MD-064 and MD-069.

---

## § 1. Authority, scope, and terminology

**§ 1(1).** This correction locks the language-owner confirmations for the two identified missing decisions. MD-064 concerns the diagnostic identity for a non-addressable borrowing target and the match-payload escape grouping. MD-069 concerns the interpretation of source-origin summaries and the liveness of references sharing an origin.

**§ 1(2).** A **Place** is an addressable storage location in the Sec semantic model. A computed value is not automatically an addressable Place merely because member syntax is used to obtain it. A **borrow** is temporary access authority; a **reference holder** is a value carrying such authority; an **origin** identifies possible referent Places/provenance. These concepts must not be conflated.

**§ 1(3).** A borrow's semantic live range, an owned value's lifetime, and the backing storage's lifetime are different concepts. Ending borrow authority does not destroy, move, or consume the referenced value, nor does it release storage.

**§ 1(4).** This correction adds no keyword, new public reference type, runtime borrow counter, explicit lifetime annotation, or special treatment for planned Sec 0.2 linear obligations.

## § 2. MD-064 — Confirm diagnostic S1168

**§ 2(1).** When an explicit `ref` or `ref mut` targets a computed expression that is not an independently addressable Place, the compiler shall reject the source with the following canonical identity:

| Field | Required value |
|---|---|
| Code | `S1168` |
| Registry name | `borrowing.non-addressable-place` |
| Diagnostic family | `borrowing` |
| Default and mandatory severity | `ERROR` |
| Rule | Only reusable addressable Places with sufficient authority may be borrowed. |

**§ 2(2).** Properties must not be treated as addressable stored fields merely because they are written with dot notation. A getter that produces a value does not grant a reference to the getter's backing implementation state. The same principle applies to other computed, non-Place expressions.

```sec
// Assume counter.Total is a computed property of type int.
let view := ref counter.Total  // ERROR S1168: property is not an addressable Place.
```

**§ 2(3).** A useful corrective example may copy the property result into an independently stored local and borrow **that local**, subject to the existing ownership and reference rules:

```sec
let total := counter.Total
let view := ref total        // Valid borrow of the local storage, if other rules permit.
```

**§ 2(4).** The diagnostic must make the semantic difference explicit: `view` in § 2(3) refers to `total`; it **does not** refer to `counter.Total` or to the stored member that its getter may read. The compiler must not suggest a direct borrow of an inaccessible/private backing field as though it were always legal.

**§ 2(5).** A property whose declared result is itself a valid `ref T` or `ref mut T` is a separate case. Accessing its returned reference is governed by the returned reference's own provenance, authority and lifetime contract; the property expression does not thereby become an addressable stored Place. This correction does not invent a new getter return or borrowing syntax.

## § 3. S1168 help text and mandatory error behavior

**§ 3(1).** Keep the code, registry name, family and mandatory-error status selected in the 2026-10-16 implementation. Refine the guidance to make the difference between local-copy storage and original property storage unmistakable. The following is a canonical *message intent*, not a mandatory exact punctuation/line-wrap template:

```text
S1168: Cannot borrow a non-addressable Place.

This expression computes a value but has no independently addressable
storage to borrow. Only stored Places may be borrowed, subject to their
authority and lifetime rules.

If a copy is appropriate, store the result in a local variable and
borrow that local. Such a reference points to the local copy, not to
the original property's underlying storage.

Alternatively, borrow an accessible stored field or use a property
that explicitly returns a reference with a valid lifetime contract.
```

**§ 3(2).** A mandatory borrow-safety error cannot be suppressed or demoted by the INFO-to-WARNING/ERROR escalation policy. Optional educational information may supplement the error in the CLI and LSP without changing the error's severity.

**§ 3(3).** Diagnostic registration, `sec diagnostics S1168`, structured/JSON output and LSP publication must refer to one stable identity, not independently invented aliases. Source ranges should identify the attempted borrow target, not merely the containing function or file.

## § 4. MD-064 — Confirm existing S1130 grouping

**§ 4(1).** An attempt to make a branch-scoped match/union payload borrow escape its permitted scope shall use the already established **`S1130`**, registry category **`escape.match-payload`**, rather than inventing a new error identity. This confirms the grouping reported in MD-064.

**§ 4(2).** The compiler shall explain the branch/payload lifetime that prevents the escape. The error remains mandatory and is separate from S1168: a match payload may be addressable within its permitted branch scope while still being forbidden to escape that scope.

**§ 4(3).** This classification does not decide every possible match-payload addressability or escape rule. It only selects the correct diagnostic identity when the existing rules establish that a payload reference is escaping illegally.

## § 5. MD-069 — Origin of call-returned references

**§ 5(1).** When a callable returns a safe reference, the returned reference holder shall inherit the canonical possible referent origin(s), provenance constraints and lifetime dependencies established by the **instantiated return-origin summary** of the callee. A call-returned reference must not be treated as originless solely because its source Place is not a direct expression at the call site.

**§ 5(2).** The summary is instantiated at the actual call and generic specialization, preserving any proven relation to the caller's arguments, their sub-Places, and allowed alternatives. A name used inside the callee is not itself the caller's origin identity; the call-site mapping must be performed.

**§ 5(3).** Origin information identifies **which Places may be referred to**; it does not by itself decide **how long a particular borrow is live**. Borrow authority, holder identity and downstream retention/escape dependencies remain separately tracked.

**§ 5(4).** When the instantiated origin is unknown, incomplete, incompatible or exceeds the analyzer's justified provenance limit, the compiler must remain conservative and may not fabricate a precise origin or assume an early last use. A function's declared return-reference contract and any legitimate unknown-retention restrictions still apply.

## § 6. MD-069 — Last use and origin-sharing holders

**§ 6(1).** On each feasible control-flow path, a borrow ends at its **last use**, including all still-reachable uses and dependencies through derived references, other live reference holders, aggregate retention, closures, returns, deferred execution and function-call retention contracts. It does **not** inherently extend to the lexical end of the containing block.

**§ 6(2).** The phrase *last use* is semantic rather than textual: an earlier `defer` or closure capture may require the borrowed authority at a later point, and a returned/retained reference may carry the obligation beyond the local call. Branches and loop backedges are analyzed using their actual possible control flow.

**§ 6(3).** Where a distinct live local carries the **same borrow origin and authority dependency** without an independently represented borrow record, it must still be counted as a surviving dependent holder. A compiler representation detail must not allow such a holder's actual borrow use to be discarded.

**§ 6(4).** The **origin root itself**, for example the `ref mut` parameter `outer` through which a new child borrow of `outer.Count` is created, is **not** counted merely by virtue of sharing the origin as an additional *child-borrow holder*. Otherwise the parent would artificially keep its already-unused child borrow live and produce false conflicts.

**§ 6(5).** The origin root remains bound by its **own** borrowing, referent validity and exclusivity rules. Excluding the root as a *child holder* does not grant it authority to bypass a child borrow that is genuinely still live.

**§ 6(6).** When a child borrow has no remaining uses or dependent holders, the origin root's previously suspended compatible access becomes usable again, provided the original reference and storage remain valid and no other conflicting borrow remains. This is a recovery of existing authority, not creation of a new borrow right or consumption of a resource.

**§ 6(7).** Equality of origin/provenance alone does not prove that holders are aliases with identical liveness or identical permission. Borrow identity and parent/child authority edges remain distinct. Proven disjoint sub-Places may be used independently under the existing overlap rules.

## § 7. Illustrative last-use cases

**§ 7(1).** The following examples assume `Container` and `Counter` are ordinary stored structs with fields shown; their declarations are illustrative rather than new standard-library types:

```sec
type Counter struct {
    Value: int
}

type Container struct {
    Count: Counter
}

fn LastUseAllowsParent(outer: ref mut Container) void {
    let count := ref mut outer.Count
    count.Value = 10                // Last use of child borrow.
    outer.Count.Value = 20          // Valid: parent may be used again.
}
```

**§ 7(2).** A later child use makes an intervening overlapping parent access illegal:

```sec
fn ChildStillLive(outer: ref mut Container) void {
    let count := ref mut outer.Count
    count.Value = 10
    outer.Count.Value = 20          // ERROR: child is needed again below.
    count.Value = 30
}
```

**§ 7(3).** A call-returned reference retains the returned origin and is analyzed through its last use; it must not be treated as an unrelated temporary:

```sec
fn GetCounter(outer: ref mut Container) ref mut Counter {
    return ref mut outer.Count
}

fn UseReturnedOrigin(outer: ref mut Container) void {
    let count := GetCounter(outer)
    count.Value = 10                // Last use of returned borrow.
    outer.Count.Value = 20          // Valid if no other dependency remains.
}
```

**§ 7(4).** `defer`, escaping return values and captured holders may extend a borrow even when its last textual mention precedes an otherwise conflicting statement. Do not accept such cases merely because a source-level variable appears unused in the intervening text.

## § 8. Separation from ownership, Sec 0.2 and optimization

**§ 8(1).** Ending a borrow does not trigger destruction of the referent and does not perform an implicit owner move, `discard`, storage reclamation or future linear consumption. The owner remains responsible for the owned value under existing 0.1 rules.

**§ 8(2).** Sec 0.2 may introduce linear obligations for owned values and types with lifecycle behavior. Those future obligations are **not** satisfied merely because an observer's last borrow use was reached; they are tracked by the corresponding ownership/consumption rules. This correction does not fix the syntax or discharge policy for 0.2 linearity.

**§ 8(3).** Sema determines source-level borrowing legality, origin, access authority, and path-sensitive last-use boundaries. Semantic IR must preserve equivalent safety-relevant facts. Sec MLIR/LLVM lowering and optimization must neither manufacture overlapping mutable authority nor prolong a *source-level* borrow simply because bookkeeping markers remain in a larger physical scope.

**§ 8(4).** This correction **does not decide MD-065**, which asks where Semantic IR `ref.end-borrow` markers must be placed relative to Sema's exact liveness endpoints. That separate question remains open. Any permitted marker placement must preserve the already-decided source semantics and must not cause false borrow conflicts or unsafe transformations.

## § 9. Required regression coverage

**§ 9(1). MD-064 identity.** Confirm `ref` and `ref mut` of computed properties/non-addressable expressions report `S1168` with the `borrowing.non-addressable-place` identity and mandatory ERROR severity; legal borrowing of stored locals/fields/array elements remains accepted under usual borrowing rules.

**§ 9(2). MD-064 guidance.** Verify the help text explicitly distinguishes a borrowed local copy from the original property's backing storage, provides no unsafe suggestion to bypass access controls, and treats reference-returning properties by their own reference contract.

**§ 9(3). Match-payload grouping.** Verify an actual, independently invalid escaping match payload borrow reports `S1130` (`escape.match-payload`), not `S1168` or an unregistered new diagnostic. Valid branch-scoped borrowing remains accepted.

**§ 9(4). Returned-origin mapping.** Test direct and nested field origins through ordinary calls, generic specialization, finite alternative returned origins and unknown/incomplete summaries. Confirm Sema, Semantic IR, diagnostics and code generation agree on the permitted referents. Do not fabricate a single origin for multi-origin cases.

**§ 9(5). Parent and child.** Accept access through `outer.Count` following the genuine last use of a child `ref mut outer.Count`, including cases where `outer` is itself a reference parameter. Reject the same parent access if a future reachable use, copied shared holder, nested reborrow, aggregate, `defer`, capture, or retention contract still requires the child authority.

**§ 9(6). Control flow.** Exercise branch-local loans, merge points, early return, guard-false paths, loops/backedges and disjoint fields. Use path-dependent last-use facts rather than a blanket block-end rule.

**§ 9(7). Pipeline conformance.** Compare accepted and rejected source between cold and incremental builds and across debug/optimized Semantic IR, Sec MLIR and LLVM modes. Verify provenance and borrow-authority invariants survive optimizations, and ensure markers/bookkeeping do not make otherwise valid source invalid.

**§ 9(8). Diagnostics interfaces.** Verify `sec diagnostics S1168` and `sec diagnostics S1130`, JSON and LSP report stable names, categories, source ranges and remedies. Mandatory errors may not be downgraded by optional informational-diagnostic policy.

## § 10. Integration and closure

**§ 10(1).** Integrate the S1168 and S1130 decisions into `rules/tooling/diagnostics.md`, and the addressable Place/property rules into the corresponding borrowing/property owner. Preserve the original numbered paragraphs and insert numbered subordinate clarifications where necessary. Do not rewrite unrelated reference or match rules.

**§ 10(2).** Integrate the return-origin and origin-root/child-holder distinctions into `rules/memory/borrowing.md`, `rules/memory/lifetime_analysis.md`, and the Semantic IR contracts, reconciling the actual MD-061 § 4 wording without altering the previously approved reborrow behavior or INFO-by-default diagnostic for *legal* reborrows.

**§ 10(3).** Review the implementation of S1168 and the return-origin/root-holder analysis as **reported implementation choices awaiting validation**, not as verified compiler behavior. Add or update regression and negative tests before claiming conformance.

**§ 10(4).** Reconcile existing governance implementation-status fragments by **canonical item ID**, preserving established implementations, partial progress, known bugs and validation status. Do not create new duplicate governance categories or overwrite the legacy historical status file.

**§ 10(5).** Keep the relevant `missing-decisions.yaml` entries pending until the owning rulebooks, implementation evidence, tests and governance have been synchronized. Once those conditions are demonstrated, the two design-answered entries may be closed; otherwise they are decided but **not yet validated or closed**.

**§ 10(6).** The central follow-up concerning marker placement in Semantic IR remains MD-065 and shall be discussed independently. The string/Unicode questions (MD-050, MD-071–MD-073) are separate and are unaffected by this correction.
