#ifndef SEC_ANALYSIS_REFERENCEFLOW_H
#define SEC_ANALYSIS_REFERENCEFLOW_H
#include "ReferenceFacts.h"
#include "mlir/Interfaces/ControlFlowInterfaces.h"
#include "llvm/ADT/DenseSet.h"
namespace sec::detail {
// Follow unambiguous CFG forwarding without inventing facts for a join whose
// incoming values differ. Cycles cannot prove their own origin.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.24; rules/compiler/semantic_ir.md §126.23.
inline Value sourceValue(Value value, llvm::DenseSet<Value> &active) {
  auto arg = dyn_cast<BlockArgument>(value);
  if (!arg || arg.getOwner()->isEntryBlock())
    return value;
  if (!active.insert(value).second)
    return {};
  Value source;
  for (Block *pred : arg.getOwner()->getPredecessors()) {
    auto branch = dyn_cast<BranchOpInterface>(pred->getTerminator());
    if (!branch) return {};
    for (unsigned i = 0; i < branch->getNumSuccessors(); ++i) {
      if (branch->getSuccessor(i) != arg.getOwner()) continue;
      auto successor = branch.getSuccessorOperands(i);
      if (arg.getArgNumber() < successor.getProducedOperandCount()) return {};
      auto operands = successor.getForwardedOperands();
      unsigned index = arg.getArgNumber() - successor.getProducedOperandCount();
      if (index >= operands.size()) return {};
      Value incoming = sourceValue(operands[index], active);
      if (!incoming || (source && source != incoming)) return {};
      source = incoming;
    }
  }
  active.erase(value);
  return source;
}

// Return the same originating SSA value across proved forwarding edges.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.24.
inline Value sourceValue(Value value) {
  llvm::DenseSet<Value> active;
  return sourceValue(value, active);
}

// Exact SSA equality is sufficient even for a loop parameter; otherwise two
// values match only when forwarding establishes one non-ambiguous source.
// Unknown origins can never make unrelated phi values compare equal.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§83,106.
inline bool sameSource(Value left, Value right) {
  if (left == right) return true;
  Value source = sourceValue(left);
  return source && source == sourceValue(right);
}


// Integrate represented slice identities, transfers and lifetimes into P15.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§84–86,116.
LogicalResult verifySliceReferences(func::FuncOp function);
} // namespace sec::detail
#endif
