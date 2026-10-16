# P25 Sec MLIR Lowering Amendment — Planned v21

- **Status:** Implementation specification; not an implementation claim
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/packages/sec_mlir_lowering_package25.md`
- **Replaces:** None; additive amendment, preserve existing numbered paragraphs
- **Package:** `SEC-MLIR-P25`

---

## § 1. Version and preconditions

Planned lowering v21 follows P24 planned lowering v20. Preconditions: frozen CompilationPlan/ABIModel, legal concrete Sema function/call, P17 ownership, P19 context metadata, P24 resolved representation/layout. No physical ABI materialization from unresolved generic, opaque by-value or unsupported foreign type.

## § 2. Pass ordering

Recommended staged pipeline:

```text
verify Semantic IR source ownership/foreign legality
resolve P24 physical layout/repr and prove validity
sec-resolve-call-abi
sec-verify-call-abi-plans
sec-lower-call-abi
sec-verify-abi-lowering
sec-verify-ffi-boundaries
sec-verify-abi-compatibility
later standard/LLVM dialect conversion
```

Implement the actual pass names as declared; register equivalent testable stage gates.

## § 3. Build a plan once

Lowering consumes only one immutable `CallABIPlan` derived by platform ABIModel. Caller and callee must use compatible plan references/ABISignatures. Never ask LLVM to derive aggregate pass/return classification a second time.

## § 4. Parameter marshalling

Evaluate arguments once, preserve semantic effects/try and P17 transfers. Split/coerce/register/stack transport per ordered ABI parts and call resource assignment. Encode no source copy or new owner. Track temporary storage and lifetimes separately from source lifetime.

## § 5. Callee entry

Translate ABI transport parts or indirect temp into one initialized semantic parameter/place, respecting source ownership and ABI validity. Hidden params and source ordinal map are distinct. Function source type unchanged.

## § 6. Return

After semantic result transfer, create matching ABI return parts or initialize sret storage. P17 callee cleanup excludes returned ownership. Caller reconstructs one result and assumes its proper cleanup responsibility. Never destroy both result temp and returned owner.

## § 7. Foreign reference adaptation

Approved call-bounded `ref`/`ref mut` to C/system may be packed to expected foreign address, bounded to call, non-retained and aligned. Native Sec references carry full safe-reference representation. Foreign raw returns never synthesize safe refs.

## § 8. C aggregate and pointer cases

Use foreign C layout and C model-specific part classification; no native tagged discriminant added to C union. RawPtr uses selected address space. Source-provenance stays available to wrapper and diagnostics. Incomplete by-value fails before lowering.

## § 9. Variadics

Lower only already-C-typed promoted vararg list, classify each through ABIModel vararg position and assign ABI resources including stack/register-save rules. No Sec typed variadic pack/spread reuse.

## § 10. Callbacks

Generate verified private C-facing bridge, translate foreign ABI to native Sec ABI, use runtime thread attachment where required, propagate declared effects, translate allowed result contract, forbid unwind and double detach. Do not export arbitrary Sec function.

## § 11. Alloc-context and hidden args

Honor P19 hidden context only on eligible native Sec ABI paths; order with receiver/environment/sret according to complete plan. Foreign calls/exports get no hidden Sec context unless a specific validated wrapper protocol provides context internally.

## § 12. Fingerprint and stale artifacts

Emit deterministic `ABISignature`/`ABIFingerprint` with plan identity. Check against imported persisted ABI metadata or reject/recompile. ABI-dependent caches invalidate by semantic dependency; stale warm cache cannot override cold verified classification.

## § 13. Diagnostic and debug map

Keep source declaration, position, selected ABI and plan when rejecting. Debug maps expose source params/results, not transport temporaries. Source locations and names do not alter ABI fingerprint.

## § 14. Test and phase gates

Implement positive/negative unit and MLIR tests described in P25 package. Physical LLVM emission is deferred; when selected target cannot support a required ABI part or P24 representation, reject/mark blocked and report actual test status. Do not count planned operations as passing.
