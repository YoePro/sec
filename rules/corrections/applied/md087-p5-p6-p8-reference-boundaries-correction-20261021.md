# MD-087 — Permanent P5/P6/P8 integration boundaries for Place and reference IR

- **Status:** Applied 2026-10-22 — integrated and implemented; register entry removed.
- **Created:** 2026-10-21
- **Last updated:** 2026-10-21
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical correction destination:** `rules/corrections/applied/` after integration
- **Primary normative owners:** `rules/mlir/sec_mlir_lowering.md` (v11.18–20); `rules/mlir/sec_mlir_dialect.md` (v11.31–34)
- **Historical/versioned owners:** `rules/mlir/lowering-versions/sec_mlir_lowering_v11.md`; `rules/mlir/dialect-versions/sec_mlir_dialect_v11.md`
- **Implementation owner:** `rules/mlir/packages/sec-mlir-dialect_package15.md` and P15-59/P15-60, including P5/P6/P8 passes
- **Decision tracking:** `missing-decisions.yaml`, MD-087
- **Related decisions:** MD-084 (metadata preservation), MD-085 (origin verification), MD-086 (emit capabilities)
- **Replaces:** the unresolved three P5/P6/P8 implementation choices in MD-087; specifically supersedes the P15-60 blanket module rejection for checked-integer lowering
- **Reference baseline:** public `main` rulebooks, P15 TODO, and MLIR Arith/Builtin documentation reviewed 2026-10-21

---

## § 1. Scope and permanent architecture

§ 1(1) This correction governs coexistence of P15's high-level Place/direct-reference representations with P5 storage lowering, P6 scalar resolution/calls and P8 checked-integer lowering. It does not alter Sec source semantics, define a physical reference ABI or impose a single immutable global pass order.

§ 1(2) Higher-level Place/reference wrappers retain semantic type identity and metadata until a dedicated representation-lowering stage can discharge all their obligations. Independent, representable scalar computation is not blocked merely because those wrappers exist elsewhere in the same module.

§ 1(3) A pass must transform only what it can prove legal, preserve untransformed high-level IR faithfully and produce verified, consistent output. Failure to support a required conversion is a specific **lowering capability** failure, not a language-validity failure.

## § 2. P5 storage boundary — accepted existing implementation

§ 2(1) The **single authoritative** implementation query is `sec::isReferenceDependentStorage` (P15-59). Storage is reference-dependent when its `sec.storage.declare` has `sec.address_taken = true` or is consumed by `sec.place.storage`.

§ 2(2) `--sec-lower-trivial-core` preserves reference-dependent storage as high-level `sec.storage` operations/values. It must not turn such storage into `memref.alloca`/MemRef (or otherwise erase storage identity) while an undischargeable reference/Place dependency remains.

§ 2(3) `--sec-resolve-scalar-layout` may resolve the storage **element type** while retaining the storage wrapper, identity, address-taken marker and reference semantics. For example, reference-dependent boolean storage may remain `!sec.storage<i1>` rather than P6's ordinary physically oriented `memref<i8>`.

§ 2(4) Storage with no reference dependency continues through existing ordinary P5/P6 lowering. The presence of unrelated reference-dependent storage cannot change a normal storage object's lowering semantics.

§ 2(5) New implementations must reuse the authoritative query or its normatively equivalent canonical abstraction; competing, subtly different duplicate storage-dependency predicates are forbidden.

## § 3. P6 reference-bearing call boundary — accepted existing implementation

§ 3(1) `sec.call.direct` whose operand **or result** contains `!sec.place`, `!sec.ref`, or `!sec.ref_mut` stays a Sec call until a representation/ABI lowering exists that faithfully carries call argument actions and all required reference facts.

§ 3(2) Trivial-core must **not** prematurely replace that call with `func.call` if the conversion loses `shared-reference` / `mutable-reference` actions, `sec.reference` or origin metadata. `func.call` signatures permitting reference types is not itself evidence of a lossless metadata mapping.

§ 3(3) Ordinary calls without these high-level dependencies continue through their existing lowering. P6 scalar call lowering must preserve all applicable discardable call attributes, even when its current transformation does not inspect them.

§ 3(4) A future representation-aware call lowering may convert such calls after demonstrating lossless authority, reference validity, ownership and origin transport. No permanent requirement to retain `sec.call.direct` after all obligations have been discharged is introduced.

## § 4. P8 selective checked-integer lowering — required correction

§ 4(1) P8 **must support** modules containing high-level Place/reference types and operations. It is nonconforming to reject an entire otherwise supported module *solely because* `!sec.place`, `!sec.ref`, `!sec.ref_mut` or a corresponding operation appears anywhere within it.

§ 4(2) P8 lowers integer computations wherever all required representations and proofs are known, even if unrelated functions contain references. An operation still requiring semantic integer/Place representation stays high-level until its own semantics can be discharged.

§ 4(3) P8 must **not recursively normalize signedness** or erase semantic referent identity within `!sec.place<T,authority>`, `!sec.ref<T>` or `!sec.ref_mut<T>`. It must preserve wrapper authority, layout/projection facts, `BorrowID`, `LifetimeID`, origin facts, validity policies and related metadata.

§ 4(4) A target-resolved semantic integer `siN`/`uiN` encountered in a Place/reference boundary is distinct as an **MLIR type** from a lowered signless `iN` computation value, although their bit patterns can represent the same width. This difference must be handled explicitly where an SSA value crosses the boundary.

§ 4(5) At a `sec.place.read`/`sec.place.write` boundary, P8 must preserve the operation's current *semantic* input/output type contract; it must materialize any necessary intermediate SSA representation conversion rather than silently changing the referent/wrapper type or implicitly changing an operation signature. The same requirement applies to relevant calls, storage values, CFG block arguments, merges and checked-operation results.

§ 4(6) The boundary conversion must preserve exact bit width and bit pattern and must not implicitly truncate, extend, round, change signed/unsigned interpretation, alter Sec overflow policy or bypass a runtime error path. Semantic signedness and checked-operation behavior are determined by the Sema-resolved operation and must be carried through to the final signless operation selection.

§ 4(7) P6/`CompilationPlan` target scalar resolution must establish the exact bit width before a width-dependent bridge is materialized. No host-size guess or hard-coded pointer-width assumption is permitted. Incompatible widths or missing representation evidence are diagnosed before publication.

§ 4(8) **A failed boundary conversion must not leave partially rewritten published IR.** The pass must either preflight its full intended rewrite and commit only verified results, use an atomic/rollback-capable rewrite strategy, or execute on a disposable staged IR. Successful output must satisfy standard MLIR verification plus the applicable Sec verifiers.

§ 4(9) A specific unsupported operation or unresolved conversion requirement may legitimately block the *requested lowering*. It must be diagnosed with the exact operation and missing capability; it must not be disguised as a module-wide prohibition on the presence of references. Supported independent computations remain lowerable in the same mixed module.

## § 5. MLIR materialization details — correct the earlier example

§ 5(1) **Important technical correction to the design discussion:** `arith.bitcast %v : si32 to i32` is not a legal generic MLIR bridge in the currently documented Arith dialect. `arith.bitcast` requires signless integer or floating-point-like inputs/outputs (and specified signless memref cases), not arbitrary signed/unsigned integer types. Its equal-width description alone is insufficient. Do **not** implement or test the invalid example.

§ 5(2) The *approved architectural decision* remains: produce an explicit, verifiable boundary between the semantic integer and the signless computation representation. The particular MLIR materialization primitive is subordinate to MLIR's actual operation type constraints.

§ 5(3) Where useful during dialect conversion, an MLIR `TypeConverter` may use a `builtin.unrealized_conversion_cast` **only as a temporary type-system bridge**. The Builtin dialect specifies that this operation has no intrinsic representation or execution semantics. It must not be mistaken for a complete checked-integer conversion or shipped as an unresolved executable backend contract.

§ 5(4) The conversion pipeline must reconcile/erase such temporary bridges or replace them with verified, semantically equivalent legal operations before the lower-dialect/LLVM boundary that requires resolved representations. A bridge whose bit pattern, width or required legality cannot be discharged must cause lowering failure.

§ 5(5) An implementation may instead use another legal MLIR conversion materialization mechanism, if it provides the same exact-value/bit-pattern and verification guarantees. This is an implementation choice, **not** a new source-language decision or authority to create ad hoc `sec.*` operations without schema specification.

§ 5(6) No reference field, epoch encoding, raw pointer, side table, MemRef-based reference ABI or LLVM pointer representation is selected by this correction.

## § 6. Example integration scenario

§ 6(1) Given a module with a reference-free scalar function and a function operating through `ref mut int`, P8 must be able to transform the supported scalar function while preserving the higher-level reference function's references and metadata.

§ 6(2) When it lowers a checked integer computation that consumes a `sec.place.read` result, it must preserve the read's semantic result type (e.g., target-resolved `si32`), materialize a legal transition to a signless `i32` computation SSA value, perform the checked computation with the correct signed/unsigned interpretation, and transition back only as required by the semantic `sec.place.write` contract.

§ 6(3) The following diagram expresses the **type relationship**, not a literal list of necessarily emitted operations:

```text
!sec.ref_mut<si32>  ->  !sec.place<si32, "rw">
                        ->  sec.place.read : si32
                        ->  verified type-system materialization : i32
                        ->  checked arithmetic : i32
                        ->  verified type-system materialization : si32
                        ->  sec.place.write : !sec.place<si32, "rw">, si32
```

§ 6(4) On a 64-bit `CompilationPlan`, substitute exact corresponding widths where the source `int` type is target-sized; fixed-width types retain their declared widths. Neither plan may change memory semantics or borrow rights.

## § 7. Inter-pass invariants and pass ordering

§ 7(1) P5 and P6 preservation rules apply independently of whether P8 executes before or after another legal scalar-only transformation. A requested pass schedule must satisfy each pass's real prerequisites, not assume one globally fixed list solely to avoid mixed representations.

§ 7(2) A pass that cannot prove it will preserve `sec.address_taken`, reference-origin summaries, validity facts, reference argument actions or address stability must leave the affected high-level operation untouched or reject that specific lowering path before mutation.

§ 7(3) Verifiers for Places, reference guards and borrow semantics must run at specified checkpoints after transformations that might affect their invariants. The checker must not silently reinterpret missing metadata as a successful proof.

§ 7(4) The updated behavior must not regress non-reference P8 arithmetic, schema-9/10 constructs or ordinary P5/P6 storage/call conversions.

## § 8. Implementation migration instructions

§ 8(1) Retain the accepted P15-59 authoritative storage query and P6 reference-bearing call behavior. Update their comments/specifications to cite this correction and the MD-084 metadata contract.

§ 8(2) **Replace** P15-60's `--sec-lower-checked-integers` module-wide pre-rewrite rejection with a selective, type-aware legality and materialization strategy. Correct the P15 TODO status/evidence, any rulebook text and tests that previously asserted rejection as the desired long-term behavior.

§ 8(3) Implement legal representation transitions using the available MLIR conversion infrastructure. Never copy the invalid `arith.bitcast siN -> iN` example from the prior design discussion into tests or lowering code.

§ 8(4) Update the consolidated v11 P5/P6/P8 interaction paragraphs by dated amendments, and synchronize the historical version snapshots without renumbering referenceable sections.

§ 8(5) Update the existing `governance/` implementation entry/fragment by stable ID and evidence. Do not create parallel package ownership or treat legacy `implementation-status.yaml` as writable authority.

§ 8(6) Remove MD-087 from the open decision ledger only after the normative and implementation/governance synchronization is complete, including the P8 behavior change and passing regression evidence.

## § 9. Required regression tests

§ 9(1) Positive: mixed module with independent scalar checked-integer operations and reference functions lowers scalar operations successfully; ordinary P5/P6 storage and calls retain existing behavior.

§ 9(2) Positive: read `ref int`, perform checked arithmetic and write through `ref mut int`; correct signed and unsigned arithmetic; preserved overflow/trap/error handling; no whole-object reference copy or unintended physical reference lowering.

§ 9(3) Positive: nested struct/array referent and target-sized `int`/`uint` resolve under 32-bit and 64-bit plans without wrapper erasure; `sec.address_taken`/origin/action metadata survive permitted passes.

§ 9(4) Positive: intermediate casts (if used) are legal according to MLIR's actual typed operation constraints; all required temporary unrealized materializations are reconciled before physical/backend lowering.

§ 9(5) Negative: unsupported width or unknown target resolution; illegal cast between signed and signless types; missing materialization contract; unexpected reference-wrapper normalization; prematurely lowered referenced storage; missing required call metadata.

§ 9(6) Negative: operation requiring unsupported reference ABI is diagnosed at the affected boundary; no partial output is published. A module containing an unrelated reference is not rejected solely for its presence.

§ 9(7) Verify the applicable Sec passes, standard MLIR verifier and full non-reference regression matrix. Where current P15 tests expect a blanket P8 failure, replace them with accurate mixed-IR positive and specific illegal-conversion negative tests.

## § 10. Decision record

§ 10(1) **Owner-approved 2026-10-21:** (1) accept the single P5 storage-dependency query and preservation behavior; (2) retain `sec.call.direct` for reference-bearing calls until reference ABI lowering can preserve all facts; (3) require permanent, selective P8 coexistence with Place/reference operations and explicit verified integer-representation boundaries. There was no fourth subdecision.

§ 10(2) The earlier proposed use of `arith.bitcast` between signed and signless MLIR integer types is **corrected as technically invalid**, without changing the approved architectural outcome. The concrete legal conversion materialization must meet §§ 4–5 and pass the mandated tests.

§ 10(3) The decision is closed at the owner/design level; implementation and governance work remain outstanding. The existing P15-60 blanket-rejection implementation is a known divergence from the newly approved normative behavior.

## § 11. External technical references (implementation only)

§ 11(1) MLIR Arith dialect, `arith.bitcast` documented operand/result restrictions: <https://mlir.llvm.org/docs/Dialects/ArithOps/#arithbitcast-arithbitcastop>.

§ 11(2) MLIR Builtin dialect, temporary `builtin.unrealized_conversion_cast` semantics: <https://mlir.llvm.org/docs/Dialects/Builtin/#builtinunrealized_conversion_cast-builtinunrealizedconversioncastop>.

---

## Application note (2026-10-22)

**Integrated into:**
- `rules/mlir/sec_mlir_dialect.md` v11.31–v11.33 amendments.
- `rules/mlir/sec_mlir_lowering.md` v11.18–v11.20 amendments.
- The dated amendment sections of both v11 snapshots.
- `rules/mlir/packages/sec-mlir-dialect_package15.md` § 67 amendment.
- The P15-60 note in `package15-todo.md`.

**Implementation status:**
- P5 (`sec::isReferenceDependentStorage`) and P6 (a reference-bearing `sec.call.direct` stays high-level) are accepted as implemented.
- Known divergence (verified 2026-10-22): `--sec-lower-checked-integers` still rejects any module that contains Places or references. The selective, type-aware lowering with legal verified integer-representation transitions, and its § 9 tests, remain to be implemented.


## Implementation (2026-10-22)

**Code:** `mlir/lib/Conversion/SecIntegerToArith/SecIntegerToArith.cpp` removes the module-wide rejection.
- **Type conversion:** the converter still rewrites only top-level semantic integers, so it never enters `!sec.place` / `!sec.ref` / `!sec.ref_mut` / slice referents.
- **Bridges:** source and target materializations create only exact-width `builtin.unrealized_conversion_cast` bridges between `siN`/`uiN` and `iN`; this is never `arith.bitcast`.
- **Bridge check:** after round-trip folding, any surviving cast that is not such an exact-width semantic ↔ signless bridge is rejected.
- **Signedness:** checked operations keep the signed or unsigned interpretation of their original operand type.

**Audit of P5/P6 (conforming):**
- Both `--sec-lower-trivial-core` and `--sec-resolve-scalar-layout` use `sec::isReferenceDependentStorage`.
- Reference-bearing `sec.call.direct` stays legal in trivial-core.
- P6's `ScalarSecCallLowering` preserves the discardable call attributes.

**Superseded tests and texts updated:**
- lit: `schema11-reference-scalar-layout.mlir` and `schema12-slice-architecture.mlir`, now positive.
- Go: `package15_emit_test.go` `verifyPackage15MLIR` now requires successful `--sec-lower-integer-core` plus verifiers and no `arith.bitcast`; `slice_unsupported_test.go` drops the P8 gate case.
- Documentation: the P16 implementation report § 30, `package16-todo.md` (P16-51, P16-52), `package15-todo.md` (P15-60) and governance.

**New tests:**
- `TestCheckedIntegerLoweringCoexistsWithReferences` (32/64-bit). It covers a mixed scalar/reference module, read–checked arithmetic–write through `ref mut`, unsigned `divui`, address-taken storage and reference calls kept, and exact-width bridges only.
- lit `Conversion/SecIntegerToArith/invalid-reference-bridges.mlir`.

**Known hole (verified 2026-10-22):** the bridges remain in the output until a physical reference-representation lowering exists to reconcile them. Until then, every route beyond `emit-sec-mlir` rejects reference programs (MD-086).
