#include "sec/Analysis/Passes.h"
#include "ReferenceFacts.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/IR/Verifier.h"
#include "sec/Dialect/Sec/SecTypes.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYFLOATSEMANTICS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {
// Validate explicit policies, including inherited module/function policies;
// omitted attributes retain the strict semantics of the Sec operations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§6–9,64,81.
LogicalResult verifyPolicies(Operation *op) {
  for (NamedAttribute attribute : op->getAttrs()) {
    StringRef name = attribute.getName().getValue();
    name.consume_front("sec.");
    Attribute value = attribute.getValue();
    auto text = dyn_cast<StringAttr>(value);
    if (name == "fastmath" || name == "fast_math") {
      auto flags = dyn_cast<arith::FastMathFlagsAttr>(value);
      if ((flags && flags.getValue() == arith::FastMathFlags::none) ||
          (text && text.getValue() == "none")) continue;
      return op->emitOpError("Sec float semantics require fast-math none");
    }
    if (name == "rounding" || name == "roundingmode") {
      auto mode = dyn_cast<arith::RoundingModeAttr>(value);
      if ((mode && mode.getValue() == arith::RoundingMode::to_nearest_even) ||
          (text && text.getValue() == "nearest-even")) continue;
      return op->emitOpError("Sec float semantics require nearest-even rounding");
    }
    if (name == "contraction") {
      auto flag = dyn_cast<BoolAttr>(value);
      if ((flag && !flag.getValue()) || (text && text.getValue() == "off")) continue;
      return op->emitOpError("Sec float semantics forbid contraction");
    }
    if (name == "denormal" || name == "denormal-fp-math" ||
        name == "denormal-fp-math-f32") {
      if (text && (text.getValue() == "preserve" ||
                   text.getValue() == "ieee" || text.getValue() == "ieee,ieee")) continue;
      return op->emitOpError("Sec float semantics require subnormal preservation");
    }
    if (name == "remainder_kind" &&
        (!text || text.getValue() != "truncation"))
      return op->emitOpError("Sec float semantics require truncation remainder");
  }
  if (op->getName().getStringRef() == "arith.flush_denormals" ||
      op->getName().getStringRef() == "math.fma" ||
      op->getName().getStringRef() == "llvm.intr.fma")
    return op->emitOpError("operation weakens ordinary Sec float semantics");
  return success();
}

// Clamp's candidate exists only on success; edge dominance also rejects
// forwarding it to failure or a merge reachable without success.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§31,69,81.
LogicalResult verifyClamp(sec::FloatClampCheckedOp clamp) {
  auto function = clamp->getParentOfType<func::FuncOp>();
  if (!function) return clamp.emitOpError("Clamp requires function control flow");
  auto branch = dyn_cast_or_null<cf::CondBranchOp>(clamp->getNextNode());
  if (!branch || branch.getCondition() != clamp.getFailed() ||
      !clamp.getFailed().hasOneUse() || branch.getTrueDest() == branch.getFalseDest())
    return clamp.emitOpError("Clamp requires an immediate single-use failed branch with distinct outcomes");
  DominanceInfo dominance(function);
  for (OpOperand &use : clamp.getResult().getUses()) {
    if (use.getOwner() == branch) {
      auto operands = branch.getFalseDestOperands();
      // Operand 0 is the predicate; the true operands precede false operands.
      unsigned firstFalse = 1 + branch.getTrueDestOperands().size();
      if (use.getOperandNumber() >= firstFalse && !operands.empty()) continue;
    }
    if (!sec::detail::guardedBy(dominance, clamp.getFailed(), false, use.getOwner()))
      return clamp.emitOpError("Clamp candidate must be confined to the success edge");
  }
  return success();
}

// Run registered operation verifiers as well as the cross-operation policy
// checks. This P20 boundary requires canonical constant bits; historical
// lexeme-only constants can still round-trip through the dialect itself.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§15,63–75,81.
LogicalResult verifyFloats(ModuleOp module) {
  WalkResult result = module.walk([&](Operation *op) {
    if (failed(verifyPolicies(op))) return WalkResult::interrupt();
    if (isa<sec::ConstFloatOp, sec::FloatNegOp, sec::FloatBinaryOp,
            sec::FloatCmpOp, sec::FloatAbsOp, sec::FloatMinimumOp,
            sec::FloatMaximumOp, sec::FloatClassifyOp, sec::FloatRoundOp,
            sec::FloatPropertyOp, sec::FloatClampCheckedOp>(op)) {
      if (failed(verify(op, false))) return WalkResult::interrupt();
      if (auto constant = dyn_cast<sec::ConstFloatOp>(op)) {
        if (!constant.getBitsAttr()) {
          constant.emitOpError("P20 float constants require canonical bits");
          return WalkResult::interrupt();
        }
      }
      if (auto clamp = dyn_cast<sec::FloatClampCheckedOp>(op))
        if (failed(verifyClamp(clamp))) return WalkResult::interrupt();
    }
    return WalkResult::advance();
  });
  return failure(result.wasInterrupted());
}

// Explicit package verifier, usable independently of lowering/schema upgrade.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §81 (P20-22).
class VerifyFloatSemanticsPass final
    : public sec::impl::SecVerifyFloatSemanticsBase<VerifyFloatSemanticsPass> {
  void runOnOperation() override {
    if (failed(sec::verifyFloatSemantics(getOperation()))) signalPassFailure();
  }
};
} // namespace

// Register the standalone P20 float-semantics verification boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §81.
std::unique_ptr<Pass> sec::createSecVerifyFloatSemanticsPass() {
  return std::make_unique<VerifyFloatSemanticsPass>();
}

// Share the exact explicit verifier with the native float-lowering boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§77–81.
LogicalResult sec::verifyFloatSemantics(ModuleOp module) {
  return verifyFloats(module);
}
