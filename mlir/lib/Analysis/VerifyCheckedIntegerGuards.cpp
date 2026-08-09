#include "sec/Analysis/Passes.h"

#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "llvm/ADT/STLExtras.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYCHECKEDINTEGERGUARDS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

LogicalResult sec::verifyCheckedIntegerGuards(func::FuncOp function) {
  WalkResult result = function.walk([&](Operation *operation) {
      Value checkedResult;
      Value failedValue;
      StringRef expectedCategory;
      StringRef expectedOperator;

      if (auto neg = dyn_cast<sec::IntNegCheckedOp>(operation)) {
        checkedResult = neg.getResult();
        failedValue = neg.getFailed();
        expectedCategory = "overflow";
        expectedOperator = "-";
      } else if (auto binary = dyn_cast<sec::IntBinaryCheckedOp>(operation)) {
        checkedResult = binary.getResult();
        failedValue = binary.getFailed();
        StringRef kind = binary.getKind();
        expectedCategory = kind == "divide"     ? "division"
                           : kind == "remainder" ? "remainder"
                                                  : "overflow";
        expectedOperator = kind == "add"        ? "+"
                           : kind == "subtract" ? "-"
                           : kind == "multiply" ? "*"
                           : kind == "divide"   ? "/"
                                                  : "%";
      } else if (auto shift = dyn_cast<sec::IntShiftCheckedOp>(operation)) {
        checkedResult = shift.getResult();
        failedValue = shift.getFailed();
        expectedCategory = "shift";
        expectedOperator = shift.getKind().starts_with("left_") ? "<<" : ">>";
      } else {
        return WalkResult::advance();
      }

      auto branch = dyn_cast_or_null<cf::CondBranchOp>(operation->getNextNode());
      if (!branch || branch.getCondition() != failedValue)
        return operation->emitOpError(
                   "must be immediately followed by cf.cond_br on failed result"),
               WalkResult::interrupt();
      if (!failedValue.hasOneUse())
        return operation->emitOpError("failed result must have exactly one use"),
               WalkResult::interrupt();
      Block *failureBlock = branch.getTrueDest();
      if (failureBlock == branch.getFalseDest() ||
          failureBlock->getNumArguments() != 0)
        return operation->emitOpError(
                   "true successor must be a dedicated arithmetic failure block"),
               WalkResult::interrupt();
      auto failure = dyn_cast<sec::FailArithmeticOp>(failureBlock->getTerminator());
      if (!failure)
        return operation->emitOpError(
                   "true successor must end in sec.fail.arithmetic"),
               WalkResult::interrupt();
      if (llvm::any_of(checkedResult.getUses(), [&](OpOperand &use) {
            return use.getOwner()->getBlock() == failureBlock;
          }))
        return operation->emitOpError(
                   "checked result must not be used in failure block"),
               WalkResult::interrupt();
      if (!llvm::hasSingleElement(*failureBlock))
        return operation->emitOpError(
                   "true successor must be a dedicated one-operation failure block"),
               WalkResult::interrupt();
      if (failure.getCategory() != expectedCategory)
        return operation->emitOpError(
                   "failure category does not match checked operation"),
               WalkResult::interrupt();
      auto sourceOperator = failure->getAttrOfType<StringAttr>("sec.operator");
      if (!sourceOperator || sourceOperator.getValue() != expectedOperator)
        return operation->emitOpError(
                   "sec.operator does not match checked operation"),
               WalkResult::interrupt();
      if (!llvm::hasSingleElement(failureBlock->getPredecessors()))
        return operation->emitOpError(
                   "failure block must have exactly one predecessor"),
               WalkResult::interrupt();
      return WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}

namespace {

class VerifyCheckedIntegerGuardsPass final
    : public sec::impl::SecVerifyCheckedIntegerGuardsBase<
          VerifyCheckedIntegerGuardsPass> {
public:
  void runOnOperation() override {
    if (failed(sec::verifyCheckedIntegerGuards(getOperation())))
      signalPassFailure();
  }
};

} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyCheckedIntegerGuardsPass() {
  return std::make_unique<VerifyCheckedIntegerGuardsPass>();
}
