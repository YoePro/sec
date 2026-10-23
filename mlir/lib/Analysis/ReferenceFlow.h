#ifndef SEC_ANALYSIS_REFERENCEFLOW_H
#define SEC_ANALYSIS_REFERENCEFLOW_H
#include "ReferenceFacts.h"
#include "mlir/Interfaces/ControlFlowInterfaces.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/SmallVector.h"
namespace sec::detail {
// Collect represented validity facts from every incoming CFG source. A
// visited set bounds cyclic forwarding. Unknown sources add no inferred
// facts, and conflicting facts are kept distinct rather than merged away.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 19-20,27,36;
// rules/mlir/packages/sec-mlir-dialect_package15.md section 7.
inline llvm::SmallVector<ReferenceFacts> incomingReferenceFacts(Value value) {
  llvm::SmallVector<ReferenceFacts> facts;
  llvm::SmallVector<Value> worklist{value};
  llvm::DenseSet<Value> visited;
  while (!worklist.empty()) {
    Value source = worklist.pop_back_val();
    if (!visited.insert(source).second) continue;
    ReferenceFacts sourceFacts = referenceFacts(source);
    if (sourceFacts.known) facts.push_back(sourceFacts);
    auto argument = dyn_cast<BlockArgument>(source);
    if (!argument || argument.getOwner()->isEntryBlock()) continue;
    for (Block *predecessor : argument.getOwner()->getPredecessors()) {
      auto branch = dyn_cast<BranchOpInterface>(predecessor->getTerminator());
      if (!branch) continue;
      for (unsigned index = 0; index < branch->getNumSuccessors(); ++index) {
        if (branch->getSuccessor(index) != argument.getOwner()) continue;
        auto successor = branch.getSuccessorOperands(index);
        unsigned ordinal = argument.getArgNumber();
        if (ordinal < successor.getProducedOperandCount()) continue;
        unsigned forwarded = ordinal - successor.getProducedOperandCount();
        auto operands = successor.getForwardedOperands();
        if (forwarded < operands.size()) worklist.push_back(operands[forwarded]);
      }
    }
  }
  return facts;
}

// Resolve one concrete SSA source through CFG forwarding, including cycles
// that carry that source unchanged. Conflicting roots and unrepresented
// incoming edges fail closed; a rootless cycle cannot prove its own origin.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.24;
// rules/compiler/semantic_ir.md — section 126.23.
inline Value sourceValue(Value value) {
  if (!value) return {};
  llvm::SmallVector<Value> worklist{value};
  llvm::DenseSet<Value> visited;
  Value source;
  while (!worklist.empty()) {
    Value current = worklist.pop_back_val();
    if (!visited.insert(current).second) continue;
    auto argument = dyn_cast<BlockArgument>(current);
    if (!argument || argument.getOwner()->isEntryBlock()) {
      if (source && source != current) return {};
      source = current;
      continue;
    }
    bool hasIncoming = false;
    for (Block *predecessor : argument.getOwner()->getPredecessors()) {
      auto branch = dyn_cast<BranchOpInterface>(predecessor->getTerminator());
      if (!branch) return {};
      for (unsigned index = 0; index < branch->getNumSuccessors(); ++index) {
        if (branch->getSuccessor(index) != argument.getOwner()) continue;
        auto successor = branch.getSuccessorOperands(index);
        unsigned ordinal = argument.getArgNumber();
        if (ordinal < successor.getProducedOperandCount()) return {};
        auto operands = successor.getForwardedOperands();
        unsigned forwarded = ordinal - successor.getProducedOperandCount();
        if (forwarded >= operands.size()) return {};
        worklist.push_back(operands[forwarded]);
        hasIncoming = true;
      }
    }
    if (!hasIncoming) return {};
  }
  return source;
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

// Copy/move retains an existing dynamic reference's validity dependency.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md section 7;
// rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 19-20,27.
LogicalResult verifyReferenceTransfers(func::FuncOp function);
} // namespace sec::detail
#endif
