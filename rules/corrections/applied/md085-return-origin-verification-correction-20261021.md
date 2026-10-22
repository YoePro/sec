# MD-085 — Path- and callee-summary-aware reference-return verification

- **Status:** Applied 2026-10-22 — integrated and implemented; register entry removed.
- **Created:** 2026-10-21
- **Last updated:** 2026-10-21
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical correction destination:** `rules/corrections/applied/` after integration
- **Primary normative owners:** `rules/mlir/sec_mlir_dialect.md`, especially v11.34–37; `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`
- **Related owners:** `rules/mlir/packages/sec-mlir-dialect_package15.md`, `rules/compiler/semantic_ir.md`, `rules/mlir/sec_mlir_lowering.md`
- **Decision tracking:** `missing-decisions.yaml`, MD-085
- **Related decisions:** MD-084 (canonical metadata grammar), MD-065 (borrow-end), MD-076 (temporary borrowing)
- **Replaces:** P15-56's provisional root-only return-origin verification and all-reference-arguments fallback at calls
- **Reference baseline:** public `main` MD-085 and P15-56 descriptions reviewed 2026-10-21

---

## § 1. Scope and allocation of authority

§ 1(1) Sema is the **single semantic authority** for source-level reference origin, borrowing, alias permissions, lifetime validity, ability to return a reference and all source-language constraints.

§ 1(2) Semantic IR carries Sema-established canonical facts, including permitted return-origin summaries and the actual reference provenance transported by operations. The emitter preserves those facts into schema 11 and schema 12, as applicable.

§ 1(3) `--sec-verify-borrow-semantics` checks **structural fidelity** of these facts to the represented MLIR. It must not infer a new independent source-level borrow/lifetime algorithm, infer previously missing permission from pointer values, or silently widen a source contract.

§ 1(4) This correction strengthens the verifier from root-only matching to path-aware matching and replaces the imprecise assumption that any reference argument of a call may be the return origin.

## § 2. Return-summary preconditions

§ 2(1) Every function with a reference result must carry a valid, nonempty `sec.return_origins` attribute in the canonical MD-084 encoding. Each entry names an actual, permitted origin under the function's resolved contract.

§ 2(2) A `param#N` root must designate an existing reference-eligible parameter, with its correct positional identity and type/authority. A `receiver` root must designate an actual eligible receiver. A `static@N` root must be backed by the explicit permitted static-origin contract; the token alone is not lifetime evidence.

§ 2(3) Each `.field#N`, `[K]` or `[*]` projection must be structurally compatible with the root type and projected result type. No invalid field ordinal, non-indexable type, unsupported variant or malformed path may be accepted simply because its root is valid.

§ 2(4) A missing, malformed or self-contradictory origin summary is invalid. An external/trusted declaration requires the separately specified external/trusted contract; absence of a body is not general permission to waive validation.

## § 3. Structural provenance traversal

§ 3(1) The verifier traces each reachable `return` of a reference through the reference-producing/transport operations represented in MLIR, including shared copies, mutable moves, reborrows, dereference-to-Place, Place projections, block arguments and joins, and direct calls with verified callee summaries.

§ 3(2) Traverse SSA definition/use, region boundaries and CFG block-argument edges in a path-sensitive way sufficient to establish a finite conservative set of possible origins. A loop or join is handled by deterministic fixed-point/visited-state logic; failure to prove membership is a verification failure, not a guessed origin.

§ 3(3) A field projection extends the origin with the resolved field ordinal. A constant array-element projection adds its exact index. An unresolved/dynamic element projection uses the canonical wildcard `[*]`, preserving known surrounding structure. An unrelated SSA integer is not interchangeable with the selected projected index.

§ 3(4) Copies/moves/reborrows preserve or appropriately refine the path and identity already attached to the reference. A reborrow does not spontaneously change its storage root, replace a field selection or widen to unrelated storage. A reborrow's authority/lifetime restrictions are still governed by Sema and separately verified metadata rules.

§ 3(5) Local storage origins or source-value roots that are not eligible for escape must be rejected on return. Projecting a field from a local origin does not convert it into a parameter/static origin.

§ 3(6) Exact paths and conservatively summarized dynamic paths are both valid when faithful to the represented operations. The verifier need not solve symbolic equality between arbitrary dynamic indexes; `[*]` is a well-defined conservative representation, not an instruction to ignore field/index structure.

## § 4. Callee-summary instantiation at calls

§ 4(1) When a direct call returns a reference, obtain the **actual callee's** canonical `sec.return_origins`. Instantiate each callee origin against the caller's corresponding reference operand, receiver or eligible static origin.

§ 4(2) Preserve any callee suffix projection when translating through the actual argument's origin path. For example, if the actual first reference argument comes from `param#2.field#1`, and the callee promises `param#0.field#3`, the composed caller-side path contains both field steps in that order, subject to type verification.

§ 4(3) Only callee-listed origins may contribute to the call result. The P15-56 provisional rule of treating **all** reference arguments as possible origins is forbidden for accepted, verified calls.

§ 4(4) Direct calls with unavailable, incompatible, malformed or uninstantiable callee summaries must fail this verifier; they cannot silently substitute the all-arguments approximation. External/indirect calls may be supported only by an explicit separately verified origin-summary contract for that call form.

§ 4(5) Any summaries crossing module boundaries are matched to the actual callee identity and signature (including compatible declared result/reference type and receiver mapping). No unresolved symbol, positional mismatch or signature mismatch may produce a successful origin proof.

## § 5. Return-summary inclusion and conservatism

§ 5(1) For every reachable reference return, the structurally derivable set of actual possible origins must be included in the function's declared `sec.return_origins` set under the canonical abstraction rules.

§ 5(2) A summary may be **conservative** when Sema legitimately chooses a broader set, including declared alternative parameter roots or a dynamic index wildcard. It must never omit an actually possible origin. A verifier does not demand artificially exact runtime branch selection.

§ 5(3) Conservative approximation is **not** permission to erase a known field selector or to name a nonexistent source. Whether a broader prefix represents a valid conservative summary must be defined by the origin abstraction contract; a bare root must not be assumed to validate every projected path unless that abstraction is explicitly supported.

§ 5(4) A return from `param#0.field#1` must not match metadata that claims exclusively `param#0.field#0`. Identical result types do not make distinct field origins equivalent. A mismatch is an IR correctness error.

§ 5(5) Distinct predecessors may contribute different origins to a join. The resulting set is the union of **reachable, admissible** origins. Verifier acceptance may not depend on branch ordering or SSA block enumeration order.

## § 6. Boundaries with Sema and other verification passes

§ 6(1) Do not rebuild Sema's full borrow-conflict graph, solve lifetime outlives constraints from scratch, or establish aliasing permissions by recomputing source rules. That would introduce a competing semantic authority.

§ 6(2) Keep the existing `--sec-verify-places` checks (Place authority, projection validity and non-escape), `--sec-verify-reference-guards` checks (dynamic validity), and `--sec-verify-borrow-semantics` checks (copy/move/reborrow/end-borrow ordering) as separate cooperating gates. Strengthened return-origin verification complements rather than displaces them.

§ 6(3) Follow MD-065 for Sema-established end-borrow points and MD-076 for bounded borrow lifetimes of temporaries. The return-origin verifier must reject escaping a call-bound temporary when Sema's supplied facts prohibit it; it does not extend the temporary's storage lifetime.

§ 6(4) Returned references rooted in admissible static storage are not globally prohibited by this amendment. The verifier must use the explicit static-root semantics of the selected schema and implementation capability; unsupported static representation is diagnosed as a capability gap, not converted into a false source-language prohibition.

## § 7. Failure rules and diagnostics

§ 7(1) Invalid metadata or structural provenance must fail closed. The verifier may not patch the IR with guessed `param#N` origins to make it pass.

§ 7(2) Diagnostics identify, where available, the return operation, claimed origin path, observed origin path or mismatched projection, callee symbol/summary, and originating source span. Report an IR contract violation distinctly from a valid Sec feature not yet supported by the installed emitter.

§ 7(3) Compiler-generated IR that fails this check is a compiler correctness defect; the verifier must not silently mark it as a user error.

## § 8. Required implementation synchronization

§ 8(1) Update `--sec-verify-borrow-semantics` and the schema-11 contract to perform complete structural projection tracking and callee-summary instantiation; keep source-language analysis in Sema and do not duplicate its algorithm.

§ 8(2) Synchronize MD-084 origin parsing/validation, the Semantic IR origin printer and the Go schema-11 emitter. A verifier may assume only facts that the emitter guarantees to preserve and that parser/verifier agreements explicitly define.

§ 8(3) Update P15-56 and related regression tests from the provisional verification depth. Update P16-compatible reference uses without inventing a separate schema-12 origin format.

§ 8(4) Synchronize the existing governance fragment/entry by its stable ID. Remove MD-085 from the outstanding decision ledger only after the normative owners and test evidence have been updated.

## § 9. Required test matrix

§ 9(1) Positive: return from a parameter, receiver and permitted static root; exact nested field paths; fixed-index element paths; a dynamic-index wildcard; shared-copy and mutable-move paths; reborrow chains; branch and loop joins; calls returning a declared subset of reference arguments.

§ 9(2) Positive: caller argument origin prefix and callee origin suffix compose correctly; reordering unrelated reference arguments does not alter the mapped callee summary; cross-module callee summaries verify when explicit and compatible.

§ 9(3) Negative: summary names wrong field ordinal while types match; wrong constant element; missing dynamic projection; local-storage escape; illegitimate root; malformed or missing summary; unresolved callee; wrong signature/argument ordinal; callee summary excludes the actual return; falsely invented static origin.

§ 9(4) Negative: an invalid reference returned from an escaping temporary; a call that previously passed solely by the all-reference-arguments fallback; truncated/contradictory summary at a CFG merge.

§ 9(5) Verify deterministic round-trip across 32- and 64-bit `CompilationPlan` values and both schema 11 and supported schema-12 reference forms. Earlier schema-9/10 and ordinary P15 references without return origins remain unaffected.

## § 10. Decision record

§ 10(1) **Owner decision accepted 2026-10-21:** seven points are locked — Sema authority; structural MLIR origin and projection verification; callee-summary instantiation; conservative but complete summaries; invalid local-origin rejection; fail-closed metadata with explicit external/trusted contracts; no second source-level borrow/lifetime checker.

§ 10(2) Implemented P15-56 behavior described in the repository is **not** deemed compliant merely because this owner decision is accepted. Verification depth and tests must be updated before MD-085 can be removed from governance.

---

## Application note (2026-10-22)

**Integrated into:**
- `rules/mlir/sec_mlir_dialect.md` v11.37 amendment.
- The dated amendment section of `dialect-versions/sec_mlir_dialect_v11.md`.
- `rules/mlir/packages/sec-mlir-dialect_package15.md` § 85 amendment.
- The P15-56 note in `package15-todo.md`.

**Known divergence (verified 2026-10-22):** `verifyReturnOrigins` in `mlir/lib/Analysis/VerifyPlaceReferences.cpp` still matches only parameter roots and treats every reference argument of a direct call as a possible origin. The canonical origin parser `sec::parseReturnOrigin` (MD-084) is available to the new implementation.


## Implementation (2026-10-22)

**Code:** `mlir/lib/Analysis/VerifyPlaceReferences.cpp` replaces the root-only `verifyReturnOrigins` with `ReturnOriginTracer`, which mirrors the Semantic IR verifier (`internal/ir/semantic/return_origin.go`):
- canonical parsing through `sec::parseReturnOrigin` and type-aware projection admissibility;
- receiver = parameter 0 of an instance method (`sec.parameter_names[0] == "self"`);
- `[K]` only for a proven-safe constant index, `[*]` otherwise; slice views are transparent and their element is `[*]`;
- deterministic visited states over (value, path);
- callee-summary instantiation, with no fallback to "all reference arguments";
- exact inclusion of every observed origin in the declared set.

Static roots (no schema-11 Place root) and union payloads (no grammar projection) are rejected as capability gaps, and a receiver origin on a non-method is rejected.

**Tests:** lit `Analysis/return-origins-paths.mlir` (fields, constant and dynamic elements through a join, caller-prefix + callee-suffix composition, receiver) and `Analysis/invalid-return-origins-paths.mlir` (10 negative cases from § 9(3)–(4)). Two older message expectations are updated. All P15/P16 source fixtures pass through `verifyPackage15MLIR`.

**Known hole (verified 2026-10-22):** conservative summaries are matched exactly: a declared `[*]` does not cover an observed `[K]`, because the origin abstraction contract defines no such coverage (§ 5(3)). Sema emits exactly the matching form today.
