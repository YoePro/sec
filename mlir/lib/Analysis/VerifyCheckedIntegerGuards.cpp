#include "sec/Analysis/Passes.h"

#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/STLExtras.h"

#include <map>
#include <set>

namespace sec {
#define GEN_PASS_DEF_SECVERIFYCHECKEDINTEGERGUARDS
#define GEN_PASS_DEF_SECVERIFYRESULTGUARDS
#define GEN_PASS_DEF_SECVERIFYTRYHANDLERS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

LogicalResult sec::verifyCheckedIntegerGuards(func::FuncOp function) {
  int64_t schema = 0;
  if (auto module = function->getParentOfType<ModuleOp>())
    if (auto version = module->getAttrOfType<IntegerAttr>("sec.dialect_version"))
      schema = version.getInt();
  WalkResult result = function.walk([&](Operation *operation) {
      Value checkedResult;
      Value failedValue;
      Value reasonValue;
      StringRef expectedCategory;
      StringRef expectedOperator;

      if (auto neg = dyn_cast<sec::IntNegCheckedOp>(operation)) {
    if (operation->getNumResults() < 2)
      return WalkResult::interrupt();
        checkedResult = operation->getResult(0);
        failedValue = operation->getResult(1);
        expectedCategory = "overflow";
        expectedOperator = "-";
      } else if (auto binary = dyn_cast<sec::IntBinaryCheckedOp>(operation)) {
        checkedResult = operation->getResult(0);
        failedValue = operation->getResult(1);
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
        checkedResult = operation->getResult(0);
        failedValue = operation->getResult(1);
        expectedCategory = "shift";
        expectedOperator = shift.getKind().starts_with("left_") ? "<<" : ">>";
      } else {
        return WalkResult::advance();
      }
    if (schema >= 5) {
    if (operation->getNumResults() != 3 ||
      !isa<sec::ArithmeticFailureReasonType>(operation->getResult(2).getType()))
      return operation->emitOpError(
         "schema 5 checked operation requires typed reason result"),
       WalkResult::interrupt();
    reasonValue = operation->getResult(2);
    } else if (operation->getNumResults() != 2) {
    return operation->emitOpError("schema 4 checked operation requires two results"),
         WalkResult::interrupt();
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
      unsigned expectedArguments = schema >= 5 ? 1 : 0;
      if (failureBlock == branch.getFalseDest() ||
          failureBlock->getNumArguments() != expectedArguments)
        return operation->emitOpError(
                   "true successor must be a dedicated arithmetic failure block"),
               WalkResult::interrupt();
    if (schema >= 5) {
    if (branch.getTrueDestOperands().size() != 1 ||
      branch.getTrueDestOperands().front() != reasonValue ||
      failureBlock->getArgument(0).getType() != reasonValue.getType())
      return operation->emitOpError(
         "true successor must receive the checked reason"),
       WalkResult::interrupt();
    }
      if (llvm::any_of(checkedResult.getUses(), [&](OpOperand &use) {
            return use.getOwner()->getBlock() == failureBlock;
          }))
        return operation->emitOpError(
                   "checked result must not be used in failure block"),
               WalkResult::interrupt();
    if (auto failure = dyn_cast<sec::FailArithmeticOp>(failureBlock->getTerminator())) {
    if (!llvm::hasSingleElement(*failureBlock))
      return operation->emitOpError(
         "ordinary failure successor must contain only sec.fail.arithmetic"),
       WalkResult::interrupt();
    if (schema >= 5) {
      if (failure->getNumOperands() != 1 ||
        failure->getOperand(0) != failureBlock->getArgument(0))
      return operation->emitOpError("sec.fail.arithmetic must consume block reason"),
           WalkResult::interrupt();
    } else {
      auto category = failure->getAttrOfType<StringAttr>("category");
      if (!category || category.getValue() != expectedCategory)
      return operation->emitOpError(
             "failure category does not match checked operation"),
           WalkResult::interrupt();
    }
    auto sourceOperator = failure->getAttrOfType<StringAttr>("sec.operator");
    if (!sourceOperator || sourceOperator.getValue() != expectedOperator)
      return operation->emitOpError(
         "sec.operator does not match checked operation"),
       WalkResult::interrupt();
    } else if (schema >= 6 &&
               isa<sec::ArithmeticErrorFromReasonOp>(failureBlock->front()) &&
               (isa<cf::BranchOp>(failureBlock->getTerminator()) ||
                isa<cf::CondBranchOp>(failureBlock->getTerminator()) ||
                isa<func::ReturnOp>(failureBlock->getTerminator()))) {
      auto mapping = cast<sec::ArithmeticErrorFromReasonOp>(&failureBlock->front());
      if (mapping.getReason() != failureBlock->getArgument(0))
        return operation->emitOpError(
                   "local handler failure mapping must consume block reason"),
               WalkResult::interrupt();
    } else if (schema >= 5) {
    auto returnOp = dyn_cast<func::ReturnOp>(failureBlock->getTerminator());
    if (!returnOp || returnOp.getNumOperands() != 1 ||
      failureBlock->getOperations().size() != 3)
      return operation->emitOpError(
         "fallible failure successor must map reason, construct Result.err, and return"),
       WalkResult::interrupt();
    auto mapping = dyn_cast<sec::ArithmeticErrorFromReasonOp>(&failureBlock->front());
    auto resultErr = dyn_cast<sec::ResultErrOp>(mapping ? mapping->getNextNode() : nullptr);
    if (!mapping || !resultErr || mapping.getReason() != failureBlock->getArgument(0) ||
      resultErr->getNumOperands() != 1 || resultErr->getOperand(0) != mapping.getResult() ||
      returnOp.getOperand(0) != resultErr.getResult())
      return operation->emitOpError("invalid fallible arithmetic failure flow"),
       WalkResult::interrupt();
    } else {
    return operation->emitOpError("true successor must end in sec.fail.arithmetic"),
         WalkResult::interrupt();
    }
      if (!llvm::hasSingleElement(failureBlock->getPredecessors()))
        return operation->emitOpError(
                   "failure block must have exactly one predecessor"),
               WalkResult::interrupt();
      return WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}

LogicalResult sec::verifyResultGuards(func::FuncOp function) {
  WalkResult result = function.walk([&](sec::ResultIsErrOp test) {
    auto branch = dyn_cast_or_null<cf::CondBranchOp>(test->getNextNode());
    if (!branch || branch.getCondition() != test.getResult())
      return test.emitOpError(
                 "must be immediately followed by cf.cond_br on its result"),
             WalkResult::interrupt();
    if (!test.getResult().hasOneUse())
      return test.emitOpError("predicate must have exactly one use"),
             WalkResult::interrupt();

    Block *errBlock = branch.getTrueDest();
    Block *okBlock = branch.getFalseDest();
    auto err = errBlock->empty()
                   ? sec::ResultUnwrapErrOp{}
                   : dyn_cast<sec::ResultUnwrapErrOp>(&errBlock->front());
    auto ok = okBlock->empty()
                  ? sec::ResultUnwrapOkOp{}
                  : dyn_cast<sec::ResultUnwrapOkOp>(&okBlock->front());
    if (!err || err.getValue() != test.getValue())
      return test.emitOpError(
                 "true successor must begin by unwrapping Err from the tested Result"),
             WalkResult::interrupt();
    auto resultType = cast<sec::ResultType>(test.getValue().getType());
    if (!isa<NoneType>(resultType.getSuccessType()) &&
        (!ok || ok.getValue() != test.getValue()))
      return test.emitOpError(
                 "false successor must begin by unwrapping Ok from the tested Result"),
             WalkResult::interrupt();
    return WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}

LogicalResult sec::verifyTryHandlers(func::FuncOp function) {
  struct Group {
    std::set<std::string> variants;
    bool catchAll = false;
    int64_t catchAllIndex = -1;
    int64_t highestVariantIndex = -1;
  };
  std::map<Block *, Group> groups;
  WalkResult result = function.walk([&](Operation *operation) {
    auto kind = operation->getAttrOfType<StringAttr>("sec.try_handler_kind");
    if (!kind)
      return WalkResult::advance();
    auto index = operation->getAttrOfType<IntegerAttr>("sec.try_handler_index");
    if (!index || index.getInt() < -1)
      return operation->emitOpError(
                 "handler provenance requires sec.try_handler_index >= -1"),
             WalkResult::interrupt();
    if (kind.getValue() != "ok" && kind.getValue() != "err-variant" &&
        kind.getValue() != "err-catch-all" && kind.getValue() != "merge")
      return operation->emitOpError("invalid sec.try_handler_kind"),
             WalkResult::interrupt();

    if (auto test = dyn_cast<sec::CoreErrorIsVariantOp>(operation)) {
      if (kind.getValue() != "err-variant")
        return test.emitOpError("variant test must be an err-variant handler"),
               WalkResult::interrupt();
      auto variant = operation->getAttrOfType<StringAttr>(
          "sec.try_handler_variant");
      if (!variant || variant.getValue() != test.getVariant())
        return test.emitOpError(
                   "handler variant provenance must match tested variant"),
               WalkResult::interrupt();
    }

    auto branch = dyn_cast<cf::BranchOp>(operation);
    if (!branch || (kind.getValue() != "err-variant" &&
                    kind.getValue() != "err-catch-all"))
      return WalkResult::advance();
    Group &group = groups[branch.getDest()];
    if (kind.getValue() == "err-catch-all") {
      if (group.catchAll)
        return branch.emitOpError("duplicate Err catch-all for try merge"),
               WalkResult::interrupt();
      group.catchAll = true;
      group.catchAllIndex = index.getInt();
    } else {
      auto variant = operation->getAttrOfType<StringAttr>(
          "sec.try_handler_variant");
      if (!variant || !group.variants.insert(variant.getValue().str()).second)
        return branch.emitOpError("duplicate or missing Err variant provenance"),
               WalkResult::interrupt();
      group.highestVariantIndex =
          std::max(group.highestVariantIndex, index.getInt());
    }
    return WalkResult::advance();
  });
  if (result.wasInterrupted())
    return failure();
  for (const auto &[merge, group] : groups) {
    if (group.catchAll && group.catchAllIndex <= group.highestVariantIndex)
      return merge->getParentOp()->emitError(
          "Err catch-all must follow all specific handlers for a try merge");
    if (!group.catchAll &&
        group.variants !=
            std::set<std::string>{"Overflow", "DivisionByZero",
                                  "InvalidShift"})
      return merge->getParentOp()->emitError(
          "ArithmeticError handlers must be exhaustive");
  }
  return success();
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

class VerifyResultGuardsPass final
    : public sec::impl::SecVerifyResultGuardsBase<VerifyResultGuardsPass> {
public:
  void runOnOperation() override {
    if (failed(sec::verifyResultGuards(getOperation())))
      signalPassFailure();
  }
};

class VerifyTryHandlersPass final
    : public sec::impl::SecVerifyTryHandlersBase<VerifyTryHandlersPass> {
public:
  void runOnOperation() override {
    if (failed(sec::verifyTryHandlers(getOperation())))
      signalPassFailure();
  }
};

} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyCheckedIntegerGuardsPass() {
  return std::make_unique<VerifyCheckedIntegerGuardsPass>();
}

std::unique_ptr<mlir::Pass> sec::createSecVerifyResultGuardsPass() {
  return std::make_unique<VerifyResultGuardsPass>();
}

std::unique_ptr<mlir::Pass> sec::createSecVerifyTryHandlersPass() {
  return std::make_unique<VerifyTryHandlersPass>();
}
