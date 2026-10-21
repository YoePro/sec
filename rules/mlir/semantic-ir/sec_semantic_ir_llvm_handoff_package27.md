# P27 Semantic IR Handoff Amendment

- **Status:** Implementation specification; not implementation evidence
- **Created:** 2026-10-20
- **Last updated:** 2026-10-20
- **Revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path:** `rules/mlir/semantic-ir/sec_semantic_ir_llvm_handoff_package27.md` (installed 2026-10-20 at the established location instead of the proposed `rules/compiler/amendments/sec_semantic_ir_llvm_handoff_package27.md`)
- **Replaces:** None; additive amendment. Preserve all existing paragraph references.
- **Package:** `SEC-MLIR-P27`

---

## § 1. Semantic IR is unchanged in meaning

P27 introduces **no new source semantic IR operation family**. It verifies that the existing normalized Semantic IR operation set can lower through the selected requested terminal boundary.

## § 2. Read-only lowering readiness

Reuse existing lowering readiness facts and expose a read-only query equivalent to:

```go
type LoweringReadiness struct {
    OperationID  SemanticOpID
    TargetStage  LoweringStage
    PlanID       CompilationPlanID
    CapabilityID string
    Ready        bool
    Reason       string
}
```

The exact Go names are advisory. Do not construct a second readiness authority or mutate Sema from this query.

## § 3. Capability provenance

For each emitted op, retain canonical Semantic IR identity, resolved types, failure/ownership/storage obligations, source location, target plan, selected layout/ABI only if needed and the required stage. No 'ready' result may be inferred from a target LLVM op's existence.

## § 4. SemIR-to-LLVM correspondence

A testable trace maps each emitted stage artifact to its SemIR operation/proof outcome. Erasure of a contract is justified by existing static discharge, lower-level representation or runtime materialization. Unknown proof is a hard gate.

## § 5. No fallback

Unsupported SemIR constructs receive `[unsupported-lowering]` and do not enter retired backends. The earlier valid SemIR output remains available for inspection.

## § 6. Test obligations

Successful scalar/CFG lowering, unsupported structural op, known deterministic failure, source-provenance diagnostics, foreign ABI lacking CallABIPlan, and invalid context/Place all have tests. Error categories invalid source/unsupported target/unsupported implementation/internal bug remain distinct.
