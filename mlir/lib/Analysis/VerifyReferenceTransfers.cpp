// Direct-reference transfers retain validity facts, independently of referent
// representation. No reference copy/move is an epoch-validation operation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md section 7;
// rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 19-20,27.
#include "ReferenceFlow.h"

using namespace mlir;
using namespace sec::detail;

// Reject validity strengthening or dependency replacement on a copied/moved
// direct reference against every represented incoming CFG source. A visited
// set bounds traversal of loop-carried references; a cycle cannot hide its
// initial dynamic dependency. Unknown sources do not supply invented facts.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md section 7;
// rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 19-20,27.
static LogicalResult verifyTransferValidity(Operation *operation) {
  ReferenceFacts target = referenceFacts(operation->getResult(0));
  for (const ReferenceFacts &source :
       incomingReferenceFacts(operation->getOperand(0))) {
    if (source.known && source.dynamic &&
        (!target.dynamic ||
         source.invalidationDomain != target.invalidationDomain ||
         source.epochDependency != target.epochDependency))
      return operation->emitOpError(
          "reference copy/move must preserve dynamic-epoch validity, invalidation_domain and epoch_dependency");
  }
  return success();
}

// Verify direct-reference transfers using the existing parameter/SSA fact
// model, including each path into joined and loop-carried block arguments.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md section 7;
// rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 19-20,27.
LogicalResult sec::detail::verifyReferenceTransfers(func::FuncOp function) {
  WalkResult result = function.walk([&](Operation *operation) {
    if (isa<sec::RefCopySharedOp, sec::RefMoveOp>(operation) &&
        failed(verifyTransferValidity(operation)))
      return WalkResult::interrupt();
    return WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}
