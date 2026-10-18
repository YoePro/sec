// Lower the P20 binary floating subset to standard Arith after scalar layout
// resolved float, float32 and float64 to f32/f64.
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 6–9 (nearest
//     even, no contraction, no fast-math, no denormal flushing), 20–26, 75–77
//   - rules/types/types.md — plain float width (MD-014)

#include "sec/Conversion/SecToLLVM/Passes.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/IR/PatternMatch.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "llvm/ADT/APFloat.h"
#include "llvm/ADT/StringSwitch.h"

namespace sec {
#define GEN_PASS_DEF_SECLOWERFLOATS
#include "sec/Conversion/SecToLLVM/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {

std::optional<arith::CmpFPredicate> floatPredicate(StringRef predicate) {
  // Source != is unordered-not-equal so NaN != NaN holds; every other
  // comparison is ordered and false with a NaN operand (P20 §§ 24–26).
  return llvm::StringSwitch<std::optional<arith::CmpFPredicate>>(predicate)
      .Case("eq", arith::CmpFPredicate::OEQ)
      .Case("ne", arith::CmpFPredicate::UNE)
      .Case("lt", arith::CmpFPredicate::OLT)
      .Case("le", arith::CmpFPredicate::OLE)
      .Case("gt", arith::CmpFPredicate::OGT)
      .Case("ge", arith::CmpFPredicate::OGE)
      .Default(std::nullopt);
}

struct LowerFloatsPass : sec::impl::SecLowerFloatsBase<LowerFloatsPass> {
  void runOnOperation() override {
    ModuleOp module = getOperation();
    bool failed = false;
    SmallVector<Operation *> operations;
    module.walk([&](Operation *operation) {
      if (isa<sec::ConstFloatOp, sec::FloatNegOp, sec::FloatBinaryOp,
              sec::FloatCmpOp>(operation))
        operations.push_back(operation);
    });
    IRRewriter rewriter(module.getContext());
    for (Operation *operation : operations) {
      rewriter.setInsertionPoint(operation);
      Location loc = operation->getLoc();
      if (auto constant = dyn_cast<sec::ConstFloatOp>(operation)) {
        auto type = dyn_cast<FloatType>(constant.getResult().getType());
        auto bits = constant.getBitsAttr();
        if (!type || !bits) {
          constant.emitError() << "the Sec MLIR -> LLVM track does not lower "
                                  "'sec.const.float' without a resolved "
                                  "format and canonical bit pattern yet";
          failed = true;
          continue;
        }
        // The exact canonical pattern from the frontend; never reparse the
        // lexeme here (P20 § 76).
        llvm::APInt pattern(type.getWidth(),
                            static_cast<uint64_t>(bits.getInt()));
        llvm::APFloat value(type.getFloatSemantics(), pattern);
        rewriter.replaceOpWithNewOp<arith::ConstantOp>(
            constant, type, rewriter.getFloatAttr(type, value));
        continue;
      }
      if (auto negate = dyn_cast<sec::FloatNegOp>(operation)) {
        rewriter.replaceOpWithNewOp<arith::NegFOp>(negate, negate.getValue());
        continue;
      }
      if (auto binary = dyn_cast<sec::FloatBinaryOp>(operation)) {
        Value left = binary.getLeft(), right = binary.getRight();
        StringRef kind = binary.getKind();
        // Each source operation rounds independently; no fastmath flags are
        // set, so nothing may contract or reassociate (P20 §§ 7–8, 75).
        if (kind == "add")
          rewriter.replaceOpWithNewOp<arith::AddFOp>(binary, left, right);
        else if (kind == "sub")
          rewriter.replaceOpWithNewOp<arith::SubFOp>(binary, left, right);
        else if (kind == "mul")
          rewriter.replaceOpWithNewOp<arith::MulFOp>(binary, left, right);
        else if (kind == "div")
          rewriter.replaceOpWithNewOp<arith::DivFOp>(binary, left, right);
        else
          // Truncation remainder, like C fmod (P20 § 23).
          rewriter.replaceOpWithNewOp<arith::RemFOp>(binary, left, right);
        continue;
      }
      auto compare = cast<sec::FloatCmpOp>(operation);
      auto predicate = floatPredicate(compare.getPredicate());
      rewriter.replaceOpWithNewOp<arith::CmpFOp>(compare, *predicate,
                                                 compare.getLeft(),
                                                 compare.getRight());
      (void)loc;
    }
    if (failed)
      signalPassFailure();
  }
};

} // namespace
