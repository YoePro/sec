#include "sec/Analysis/Passes.h"

#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/STLExtras.h"

#include <map>
#include <optional>
#include <set>

namespace sec {
#define GEN_PASS_DEF_SECVERIFYCHECKEDINTEGERGUARDS
#define GEN_PASS_DEF_SECVERIFYRESULTGUARDS
#define GEN_PASS_DEF_SECVERIFYTRYHANDLERS
#define GEN_PASS_DEF_SECVERIFYUNIONGUARDS
#define GEN_PASS_DEF_SECVERIFYMATCHCFG
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

    if (test->hasAttr("sec.match_id")) {
      auto testID = test->getAttrOfType<IntegerAttr>("sec.match_id");
      auto testArm = test->getAttrOfType<IntegerAttr>("sec.match_arm_index");
      auto testStage = test->getAttrOfType<StringAttr>("sec.match_stage");
      auto branchID = branch->getAttrOfType<IntegerAttr>("sec.match_id");
      auto branchArm = branch->getAttrOfType<IntegerAttr>("sec.match_arm_index");
      auto branchStage = branch->getAttrOfType<StringAttr>("sec.match_stage");
      if (!testID || !testArm || !testStage || !branchID || !branchArm ||
          !branchStage || testID != branchID || testArm != branchArm ||
          testStage.getValue() != "pattern" ||
          branchStage.getValue() != "pattern")
        return test.emitOpError("has inconsistent match provenance"),
               WalkResult::interrupt();
      // A Result match arm may reverse the predicate edges for Ok and may
      // intentionally discard either payload. Match CFG and the unwrap ops
      // verify those paths without imposing try's fixed Err/Ok shape.
      return WalkResult::advance();
    }

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
    bool exhaustive = false;
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
    if (auto exhaustive = operation->getAttrOfType<BoolAttr>(
            "sec.try_handler_exhaustive"))
      group.exhaustive = group.exhaustive || exhaustive.getValue();
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
    int64_t schema = 0;
    if (auto module = function->getParentOfType<ModuleOp>())
      if (auto version =
              module->getAttrOfType<IntegerAttr>("sec.dialect_version"))
        schema = version.getInt();
    if (schema >= 7 && !group.exhaustive)
      return merge->getParentOp()->emitError(
          "try handlers require exhaustive Sema provenance");
    if (schema < 7 && !group.catchAll &&
        group.variants !=
            std::set<std::string>{"Overflow", "DivisionByZero",
                                  "InvalidShift"})
      return merge->getParentOp()->emitError(
          "ArithmeticError handlers must be exhaustive");
  }
  return success();
}

LogicalResult sec::verifyUnionGuards(func::FuncOp function) {
  WalkResult result = function.walk([&](Operation *operation) {
    if (auto test = dyn_cast<sec::UnionIsVariantOp>(operation)) {
      auto branch = dyn_cast_or_null<cf::CondBranchOp>(operation->getNextNode());
      if (!branch || branch.getCondition() != test.getResult())
        return test.emitOpError(
                   "must immediately feed a conditional branch"),
               WalkResult::interrupt();
      return WalkResult::advance();
    }
    auto payload = dyn_cast<sec::UnionUnwrapPayloadOp>(operation);
    auto field = dyn_cast<sec::UnionUnwrapFieldOp>(operation);
    if (!payload && !field)
      return WalkResult::advance();

    Value value = payload ? payload.getValue() : field.getValue();
    int64_t variant = payload ? payload.getVariantAttr().getInt()
                              : field.getVariantAttr().getInt();
    Block *matchingBlock = operation->getBlock();
    Block *predecessor = matchingBlock->getSinglePredecessor();
    auto branch = predecessor
                      ? dyn_cast<cf::CondBranchOp>(predecessor->getTerminator())
                      : cf::CondBranchOp();
    auto test = branch && branch.getTrueDest() == matchingBlock
                    ? branch.getCondition().getDefiningOp<sec::UnionIsVariantOp>()
                    : sec::UnionIsVariantOp();
    if (test && test.getValue() == value &&
        test.getVariantAttr().getInt() == variant &&
        test->getNextNode() == branch.getOperation())
      return WalkResult::advance();
    return operation->emitOpError(
               "must be in the true successor of a matching union.is_variant guard on the same SSA value"),
           WalkResult::interrupt();
  });
  return result.wasInterrupted() ? failure() : success();
}

LogicalResult sec::verifyMatchCFG(func::FuncOp function) {
  struct Group {
    int64_t lastArm = -1;
    int64_t catchAllArm = -1;
    Block *merge = nullptr;
    Value subject;
    std::optional<unsigned> mergeArity;
    std::set<int64_t> guardedArms;
    bool hasPattern = false;
    bool hasBodyExit = false;
    bool hasResidual = false;
    bool hasCatchAll = false;
  };
  std::map<int64_t, Group> groups;
  WalkResult result = function.walk([&](Operation *operation) {
    auto id = operation->getAttrOfType<IntegerAttr>("sec.match_id");
    auto arm = operation->getAttrOfType<IntegerAttr>("sec.match_arm_index");
    auto stage = operation->getAttrOfType<StringAttr>("sec.match_stage");
    auto kind = operation->getAttrOfType<StringAttr>("sec.match_pattern_kind");
    if (!id && !arm && !stage && !kind)
      return WalkResult::advance();
    if (!id || !arm || !stage || !kind)
      return operation->emitOpError("incomplete Sec match provenance"),
             WalkResult::interrupt();
    Group &group = groups[id.getInt()];
    if (arm.getInt() < group.lastArm)
      return operation->emitOpError("match arm provenance is not source ordered"),
             WalkResult::interrupt();
    if (group.catchAllArm >= 0 && arm.getInt() > group.catchAllArm &&
        group.guardedArms.count(group.catchAllArm) == 0)
      return operation->emitOpError("match arm follows an unguarded catch-all"),
             WalkResult::interrupt();
    group.lastArm = arm.getInt();
    if (stage.getValue() == "pattern") {
      group.hasPattern = true;
      if (kind.getValue() == "catch-all") {
        group.hasCatchAll = true;
        group.catchAllArm = arm.getInt();
      }
      if (auto compare = dyn_cast<sec::EnumCmpOp>(operation)) {
        if (group.subject && group.subject != compare.getLeft())
          return compare.emitOpError("match tests must reuse one enum subject"),
                 WalkResult::interrupt();
        group.subject = compare.getLeft();
        auto branch = dyn_cast_or_null<cf::CondBranchOp>(operation->getNextNode());
        if (!branch || branch.getCondition() != compare.getResult())
          return compare.emitOpError(
                     "enum match comparison must immediately feed cf.cond_br"),
                 WalkResult::interrupt();
      } else if (auto test = dyn_cast<sec::UnionIsVariantOp>(operation)) {
        if (group.subject && group.subject != test.getValue())
          return test.emitOpError("match tests must reuse one union subject"),
                 WalkResult::interrupt();
        group.subject = test.getValue();
        auto branch = dyn_cast_or_null<cf::CondBranchOp>(operation->getNextNode());
        if (!branch || branch.getCondition() != test.getResult())
          return test.emitOpError(
                     "union match test must immediately feed cf.cond_br"),
                 WalkResult::interrupt();
      } else if (auto test = dyn_cast<sec::ResultIsErrOp>(operation)) {
        if (group.subject && group.subject != test.getValue())
          return test.emitOpError("match tests must reuse one Result subject"),
                 WalkResult::interrupt();
        group.subject = test.getValue();
        auto branch = dyn_cast_or_null<cf::CondBranchOp>(operation->getNextNode());
        if (!branch || branch.getCondition() != test.getResult())
          return test.emitOpError(
                     "Result match test must immediately feed cf.cond_br"),
                 WalkResult::interrupt();
      }
    } else if (stage.getValue() == "guard") {
      if (!isa<cf::CondBranchOp>(operation))
        return operation->emitOpError("match guard must be cf.cond_br"),
               WalkResult::interrupt();
      group.guardedArms.insert(arm.getInt());
    } else if (stage.getValue() == "body-exit") {
      auto branch = dyn_cast<cf::BranchOp>(operation);
      auto returning = dyn_cast<func::ReturnOp>(operation);
      auto unreachable = dyn_cast<sec::UnreachableOp>(operation);
      auto failure = dyn_cast<sec::FailArithmeticOp>(operation);
      if (!branch && !returning && !unreachable && !failure)
        return operation->emitOpError(
                   "match body exit must branch or terminate"),
               WalkResult::interrupt();
      if (branch) {
        if (group.merge && group.merge != branch.getDest())
          return branch.emitOpError("match body exits must share one merge"),
                 WalkResult::interrupt();
        if (group.mergeArity &&
            *group.mergeArity != branch.getDestOperands().size())
          return branch.emitOpError("match body exits must share merge arity"),
                 WalkResult::interrupt();
        group.merge = branch.getDest();
        group.mergeArity = branch.getDestOperands().size();
      }
      group.hasBodyExit = true;
    } else if (stage.getValue() == "residual") {
      auto unreachable = dyn_cast<sec::UnreachableOp>(operation);
      if (!unreachable || unreachable.getReason() !=
                              "exhaustive-match-fallthrough")
        return operation->emitOpError(
                   "match residual must be exhaustive sec.unreachable"),
               WalkResult::interrupt();
      group.hasResidual = true;
    }
    return WalkResult::advance();
  });
  if (result.wasInterrupted())
    return failure();
  for (const auto &[id, group] : groups)
    if (!group.hasPattern || !group.hasBodyExit ||
        (!group.hasResidual && !group.hasCatchAll))
      return function.emitError()
             << "match " << id
             << " lacks pattern, body-exit, or exhaustive residual provenance";
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

class VerifyUnionGuardsPass final
    : public sec::impl::SecVerifyUnionGuardsBase<VerifyUnionGuardsPass> {
public:
  void runOnOperation() override {
    if (failed(sec::verifyUnionGuards(getOperation())))
      signalPassFailure();
  }
};

class VerifyMatchCFGPass final
    : public sec::impl::SecVerifyMatchCFGBase<VerifyMatchCFGPass> {
public:
  void runOnOperation() override {
    if (failed(sec::verifyMatchCFG(getOperation())))
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

std::unique_ptr<mlir::Pass> sec::createSecVerifyUnionGuardsPass() {
  return std::make_unique<VerifyUnionGuardsPass>();
}

std::unique_ptr<mlir::Pass> sec::createSecVerifyMatchCFGPass() {
  return std::make_unique<VerifyMatchCFGPass>();
}
