#ifndef SEC_ANALYSIS_REFERENCEFACTS_H
#define SEC_ANALYSIS_REFERENCEFACTS_H

#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/Dominance.h"
#include "sec/Dialect/Sec/SecOps.h"

namespace sec::detail {
using namespace mlir;

// guardedBy reports whether `operation` is dominated by the selected edge
// of a conditional branch on `predicate`, made exclusive by a dedicated
// successor block (no other path enters it).
// Rules: rules/mlir/sec_mlir_dialect.md — v11.36, v12.24.
inline bool guardedBy(DominanceInfo &dominance, Value predicate, bool trueEdge,
               Operation *operation) {
  for (Operation *user : predicate.getUsers()) {
    auto branch = dyn_cast<cf::CondBranchOp>(user);
    if (!branch || branch.getCondition() != predicate ||
        branch.getTrueDest() == branch.getFalseDest())
      continue;
    Block *edge = trueEdge ? branch.getTrueDest() : branch.getFalseDest();
    if (edge->getSinglePredecessor() != branch->getBlock())
      continue;
    if (dominance.dominates(edge, operation->getBlock()))
      return true;
  }
  return false;
}

// The shared P15 reference identity/validity model, also used by bounded slices.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.7, v12.5, v12.21, v12.24.
struct ReferenceFacts {
  bool known = false;
  uint32_t borrow = 0;
  uint32_t lifetime = 0;
  bool dynamic = false;
};

// Read canonical operation-owned P15 reference facts.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.7, v12.5.
template <typename OpT>
inline ReferenceFacts factsOf(OpT op) {
  return ReferenceFacts{true, op.getBorrowId(), op.getLifetimeId(),
                        op.getValidityPolicy() == "dynamic-epoch"};
}

// Facts recorded as a `sec.reference` dictionary on a reference function
// parameter or call result (implementation choice MD-084).
// Rules: rules/mlir/sec_mlir_dialect.md — v11.7, v12.5.
inline ReferenceFacts dictionaryFacts(DictionaryAttr facts) {
  if (!facts)
    return ReferenceFacts{};
  auto borrow = facts.getAs<IntegerAttr>("borrow_id");
  auto lifetime = facts.getAs<IntegerAttr>("lifetime_id");
  auto policy = facts.getAs<StringAttr>("validity_policy");
  if (!borrow || !lifetime || !policy)
    return ReferenceFacts{};
  return ReferenceFacts{true, static_cast<uint32_t>(borrow.getInt()),
                        static_cast<uint32_t>(lifetime.getInt()),
                        policy.getValue() == "dynamic-epoch"};
}

// Resolve canonical facts of a reference/slice SSA definition or parameter.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.7, v12.5, v12.24.
inline ReferenceFacts referenceFacts(Value reference) {
  if (!reference) return ReferenceFacts{};
  if (auto argument = dyn_cast<BlockArgument>(reference)) {
    Block *owner = argument.getOwner();
    auto function = dyn_cast_or_null<func::FuncOp>(owner->getParentOp());
    if (function && owner->isEntryBlock())
      return dictionaryFacts(function.getArgAttrOfType<DictionaryAttr>(
          argument.getArgNumber(), "sec.reference"));
    return ReferenceFacts{};
  }
  Operation *definition = reference.getDefiningOp();
  if (!definition)
    return ReferenceFacts{};
  if (auto facts = definition->getAttrOfType<DictionaryAttr>("sec.reference"))
    return dictionaryFacts(facts);
  if (auto op = dyn_cast_or_null<sec::RefBorrowSharedOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefBorrowMutOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefCopySharedOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefMoveOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefReborrowSharedOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefReborrowMutOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::SliceBorrowSharedOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::SliceBorrowMutOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::SliceReborrowSharedOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::SliceReborrowMutOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::SliceCopySharedOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::SliceElementPlaceOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::SliceMoveMutOp>(definition))
    return factsOf(op);
  return ReferenceFacts{};
}

} // namespace sec::detail
#endif
