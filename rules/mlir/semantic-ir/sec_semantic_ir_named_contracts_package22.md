# Semantic IR Amendment — Named-Type Contracts

- **Status:** Proposed additive v1 amendment
- **Created:** 2026-10-16
- **Last updated:** 2026-10-16
- **Document revision:** 1.0
- **Sec language version:** 0.1
- **Canonical path (proposed):** `rules/mlir/semantic-ir/sec_semantic_ir_named_contracts_package22.md`
- **Replaces:** None; additive package amendment only
- **Package:** `SEC-MLIR-P22`

---

## § 1. Scope

Add target-independent contract validation for *named* value construction. Semantic IR does not infer contracts from layout or rerun frontend CTE. Existing nominal type identity and checked Result semantics remain authoritative.

## § 2. Resolved plans

A `ResolvedNamedContractConversionPlan` comes from immutable Sema analysis. Its stable identity refers to source and target types, underlying intrinsic conversion plan, ordered effective contracts, declared origin/type ordinal, canonical pre-evaluated operands, predicate/equality/length analysis, proof class, and `core::ConversionError`. Optional source assignment destination Place/ownership plans are references to P15/P17 facts, not reinterpretations.

## § 3. Contract predicate representation

`ContractCheckOp` consumes a checked underlying source value plus a resolved predicate/operand plan, returns `bool`, and carries `ContractKind`, declaring TypeID, zero-based `DeclarationIndex`, proof and source location. It may later expand to existing integer, numeric, string, collection or bool operations. It never dynamically interprets source AST.

## § 4. Ordered contract sequence

The sequence preserves canonical inherited/local effective order. Proven checks may be skipped only when invariant/error precedence is preserved. The first failed runtime check terminates the sequence. No contract check may run on the intrinsic-failure path.

## § 5. Nominal construction

`ConstructNamedValidatedOp` accepts the underlying typed value only on the success-dominated path. It returns the canonical nominal TypeID. There is no unchecked public nominal cast. Aggregate default construction with a constrained field requires prevalidated default facts.

## § 6. Error formation

Use the existing core enum/union type identities from `sec/core/error.sec` to form `ContractKind`, `ContractError.Violation { Kind, DeclarationIndex }` and `ConversionError.Contract(ContractError)`. Error payload must not include compiler-only `DeclaringTypeID`.

## § 7. Assignment

`CheckedNamedAssignmentCommitPlan` records destination Place and P17 initialization/replacement action. New source value materialization and all guards happen before commit. Failure leaves old ownership/storage/cleanup unchanged. The builder cannot choose copy vs move.

## § 8. CTE / constant path

An invalid statically known contract value produces a compile-time diagnostic. A valid known value becomes a proven validated named constant, with origin and contract proof. No guaranteed Err is generated to defer a known error.

## § 9. Families and readiness

Supported predicates: resolved `range`, `in`, `odd`, `even`, `multipleOf`, `finite`, `minLen`, `maxLen`, `exactLen`, `notEmpty`, `minByteLen`, `maxByteLen`, `exactByteLen`, `unique`. Actual dynamic IR emission requires the underlying family/equality/length operation to be available. Regex is blocked by MD-010; invalid-string policy by MD-050 where relevant. Treat unsupported as compile-time capability feedback.

## § 10. Verification

Check typed inputs, ordinal/provenance, intrinsic-success dominance, branch polarity, failure guard identity, no candidate use before total success, exact core union payload, no unsupported family claiming proof, and source/target nominal identity preservation. Diagnostic reports include source plus declaration locations.

## § 11. Deterministic printing

Print TypeIDs, predicate plan identity, proof, ContractKind, declaration index, SSA operand IDs, checked branch targets and error nominal type. Do not print pointers to AST nodes, physical string layout or target ABI fields.

## § 12. Physical boundary

No LLVM-specific representation, no custom contract runtime, no physical union layout, no byte-order inference, and no new error universe. Standard arithmetic/control-flow predicates may be emitted after semantic validation.
