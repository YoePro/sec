// Lower the P20 binary floating subset to standard Arith after scalar layout
// resolved float, float32 and float64 to f32/f64.
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package20.md — §§ 6–9 (nearest
//     even, no contraction, no fast-math, no denormal flushing), 20–26, 75–77
//   - rules/types/types.md — plain float width (MD-014)

#include "sec/Conversion/SecToLLVM/Passes.h"
#include "sec/Analysis/Passes.h"
#include "FloatLowering.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
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

// The equal-width integer view of a binary float and its field masks
// (P20 § 78).
struct FloatLayout {
  IntegerType integer;
  APInt sign, exponent, fraction;
  unsigned mantissaBits;
};

FloatLayout layoutOf(FloatType type, MLIRContext *context) {
  unsigned width = type.getWidth();
  unsigned mantissa = width == 32 ? 23 : 52;
  APInt fraction = APInt::getLowBitsSet(width, mantissa);
  APInt sign = APInt::getSignMask(width);
  APInt exponent = ~(sign | fraction);
  return {IntegerType::get(context, width), sign, exponent, fraction, mantissa};
}

Value integerConstant(OpBuilder &builder, Location loc, IntegerType type,
                      const APInt &value) {
  return arith::ConstantOp::create(builder, loc, type,
                                   builder.getIntegerAttr(type, value));
}

Value floatConstant(OpBuilder &builder, Location loc, FloatType type,
                    const APFloat &value) {
  return arith::ConstantOp::create(builder, loc, type,
                                   builder.getFloatAttr(type, value));
}

// The value with its sign bit replaced by that of `signSource`.
Value withSignOf(OpBuilder &builder, Location loc, const FloatLayout &layout,
                 Value value, Value signSource) {
  Value magnitude = arith::AndIOp::create(
      builder, loc, arith::BitcastOp::create(builder, loc, layout.integer, value),
      integerConstant(builder, loc, layout.integer, ~layout.sign));
  Value sign = arith::AndIOp::create(
      builder, loc,
      arith::BitcastOp::create(builder, loc, layout.integer, signSource),
      integerConstant(builder, loc, layout.integer, layout.sign));
  Value bits = arith::OrIOp::create(builder, loc, magnitude, sign);
  return arith::BitcastOp::create(builder, loc, value.getType(), bits);
}

// Integral rounding without a runtime library (P20 §§ 34, 80): only a value
// with |x| < 2^p can have a fraction, so NaN, infinities and large values
// pass through unchanged; the conversion only ever sees the guarded value,
// and the result takes the sign of x, preserving signed zero.
Value lowerRound(OpBuilder &builder, Location loc, FloatType type,
                 StringRef kind, Value value) {
  FloatLayout layout = layoutOf(type, builder.getContext());
  IntegerType i64 = builder.getI64Type();
  Value zero = floatConstant(builder, loc, type, APFloat::getZero(type.getFloatSemantics()));
  APFloat limit(type.getFloatSemantics(), 1);
  limit = scalbn(limit, layout.mantissaBits, APFloat::rmNearestTiesToEven);
  Value magnitude = withSignOf(builder, loc, layout, value, zero);
  Value fractional = arith::CmpFOp::create(builder, loc, arith::CmpFPredicate::OLT,
                                           magnitude, floatConstant(builder, loc, type, limit));
  Value guarded = arith::SelectOp::create(builder, loc, fractional, value, zero);
  Value integer = arith::FPToSIOp::create(builder, loc, i64, guarded);
  Value truncated = arith::SIToFPOp::create(builder, loc, type, integer);
  auto i64Constant = [&](int64_t number) {
    return integerConstant(builder, loc, i64, APInt(64, number, true));
  };
  Value adjustment = i64Constant(0);
  if (kind == "floor") {
    Value below = arith::CmpFOp::create(builder, loc, arith::CmpFPredicate::OLT, guarded, truncated);
    adjustment = arith::SelectOp::create(builder, loc, below, i64Constant(-1), adjustment);
  } else if (kind == "ceiling") {
    Value above = arith::CmpFOp::create(builder, loc, arith::CmpFPredicate::OGT, guarded, truncated);
    adjustment = arith::SelectOp::create(builder, loc, above, i64Constant(1), adjustment);
  } else if (kind == "nearest-even") {
    // x - trunc(x) is exact here; ties go to the even neighbour.
    Value distance = withSignOf(builder, loc, layout,
                                arith::SubFOp::create(builder, loc, guarded, truncated), zero);
    Value half = floatConstant(builder, loc, type, APFloat(type.getFloatSemantics(), "0.5"));
    Value beyond = arith::CmpFOp::create(builder, loc, arith::CmpFPredicate::OGT, distance, half);
    Value tie = arith::CmpFOp::create(builder, loc, arith::CmpFPredicate::OEQ, distance, half);
    Value odd = arith::CmpIOp::create(
        builder, loc, arith::CmpIPredicate::ne,
        arith::AndIOp::create(builder, loc, integer, i64Constant(1)), i64Constant(0));
    Value away = arith::OrIOp::create(builder, loc, beyond,
                                      arith::AndIOp::create(builder, loc, tie, odd));
    Value negative = arith::CmpFOp::create(builder, loc, arith::CmpFPredicate::OLT, guarded, zero);
    Value step = arith::SelectOp::create(builder, loc, negative, i64Constant(-1), i64Constant(1));
    adjustment = arith::SelectOp::create(builder, loc, away, step, adjustment);
  }
  Value rounded = arith::SIToFPOp::create(
      builder, loc, type, arith::AddIOp::create(builder, loc, integer, adjustment));
  Value signedResult = withSignOf(builder, loc, layout, rounded, value);
  return arith::SelectOp::create(builder, loc, fractional, signedResult, value);
}

// Bitwise classification with the binary field masks (P20 §§ 32–33, 78).
Value lowerClassify(OpBuilder &builder, Location loc, FloatType type,
                    StringRef kind, Value value) {
  FloatLayout layout = layoutOf(type, builder.getContext());
  Value bits = arith::BitcastOp::create(builder, loc, layout.integer, value);
  Value exponentMask = integerConstant(builder, loc, layout.integer, layout.exponent);
  Value zero = integerConstant(builder, loc, layout.integer, APInt::getZero(type.getWidth()));
  Value exponent = arith::AndIOp::create(builder, loc, bits, exponentMask);
  Value fraction = arith::AndIOp::create(
      builder, loc, bits, integerConstant(builder, loc, layout.integer, layout.fraction));
  auto compare = [&](arith::CmpIPredicate predicate, Value left, Value right) -> Value {
    return arith::CmpIOp::create(builder, loc, predicate, left, right);
  };
  Value allOnes = compare(arith::CmpIPredicate::eq, exponent, exponentMask);
  if (kind == "finite")
    return compare(arith::CmpIPredicate::ne, exponent, exponentMask);
  if (kind == "infinite")
    return arith::AndIOp::create(builder, loc, allOnes,
                                 compare(arith::CmpIPredicate::eq, fraction, zero));
  if (kind == "nan")
    return arith::AndIOp::create(builder, loc, allOnes,
                                 compare(arith::CmpIPredicate::ne, fraction, zero));
  Value exponentZero = compare(arith::CmpIPredicate::eq, exponent, zero);
  if (kind == "normal")
    return arith::AndIOp::create(
        builder, loc, compare(arith::CmpIPredicate::ne, exponent, zero),
        compare(arith::CmpIPredicate::ne, exponent, exponentMask));
  return arith::AndIOp::create(builder, loc, exponentZero,
                               compare(arith::CmpIPredicate::ne, fraction, zero));
}

// The exact associated property values (P20 § 28); nan is the canonical
// quiet NaN.
std::optional<APFloat> propertyValue(FloatType type, StringRef kind) {
  const llvm::fltSemantics &semantics = type.getFloatSemantics();
  if (kind == "max")
    return APFloat::getLargest(semantics);
  if (kind == "min")
    return APFloat::getLargest(semantics, /*Negative=*/true);
  if (kind == "infinity")
    return APFloat::getInf(semantics);
  if (kind == "negative-infinity")
    return APFloat::getInf(semantics, /*Negative=*/true);
  if (kind == "nan")
    return APFloat::getQNaN(semantics);
  if (kind == "epsilon") {
    APFloat one(semantics, 1);
    return scalbn(one, -static_cast<int>(type.getFPMantissaWidth() - 1),
                  APFloat::rmNearestTiesToEven);
  }
  return std::nullopt;
}

struct LowerFloatsPass : sec::impl::SecLowerFloatsBase<LowerFloatsPass> {
  void runOnOperation() override {
    ModuleOp module = getOperation();
    if (mlir::failed(sec::verifyFloatSemantics(module))) {
      signalPassFailure();
      return;
    }
    bool failed = false;
    SmallVector<Operation *> operations;
    module.walk([&](Operation *operation) {
      if (isa<sec::ConstFloatOp, sec::FloatNegOp, sec::FloatBinaryOp,
              sec::FloatCmpOp, sec::FloatAbsOp, sec::FloatMinimumOp,
              sec::FloatMaximumOp, sec::FloatClassifyOp, sec::FloatRoundOp,
              sec::FloatPropertyOp, sec::FloatClampCheckedOp>(operation))
        operations.push_back(operation);
    });
    IRRewriter rewriter(module.getContext());
    for (Operation *operation : operations) {
      // The explicit pass may be invoked without its scalar-layout predecessor.
      // Fail closed rather than casting an unresolved Sec scalar to builtin
      // FloatType. Rules: rules/mlir/packages/sec-mlir-dialect_package20.md
      // §§75–80; rules/types/types.md (MD-014).
      Type scalar = operation->getNumOperands() ? operation->getOperand(0).getType()
                                                 : operation->getResult(0).getType();
      if (!isa<Float32Type, Float64Type>(scalar)) {
        operation->emitError()
            << "[unsupported-lowering] " << operation->getName()
            << " requires resolved binary32/binary64 scalar layout before float lowering";
        failed = true;
        continue;
      }
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
          // Exact binary reduction avoids an undeclared fmod runtime call.
          sec::lowerFloatRemainder(binary, rewriter);
        continue;
      }
      if (auto abs = dyn_cast<sec::FloatAbsOp>(operation)) {
        auto type = cast<FloatType>(abs.getValue().getType());
        Value zero = floatConstant(rewriter, loc, type,
                                   APFloat::getZero(type.getFloatSemantics()));
        rewriter.replaceOp(abs, withSignOf(rewriter, loc,
                                           layoutOf(type, rewriter.getContext()),
                                           abs.getValue(), zero));
        continue;
      }
      if (auto minimum = dyn_cast<sec::FloatMinimumOp>(operation)) {
        // NaN-propagating; never minnumf (P20 § 36).
        rewriter.replaceOpWithNewOp<arith::MinimumFOp>(minimum, minimum.getLeft(),
                                                       minimum.getRight());
        continue;
      }
      if (auto maximum = dyn_cast<sec::FloatMaximumOp>(operation)) {
        rewriter.replaceOpWithNewOp<arith::MaximumFOp>(maximum, maximum.getLeft(),
                                                       maximum.getRight());
        continue;
      }
      if (auto classify = dyn_cast<sec::FloatClassifyOp>(operation)) {
        auto type = cast<FloatType>(classify.getValue().getType());
        rewriter.replaceOp(classify, lowerClassify(rewriter, loc, type,
                                                   classify.getKind(),
                                                   classify.getValue()));
        continue;
      }
      if (auto round = dyn_cast<sec::FloatRoundOp>(operation)) {
        auto type = cast<FloatType>(round.getValue().getType());
        rewriter.replaceOp(round, lowerRound(rewriter, loc, type, round.getKind(),
                                             round.getValue()));
        continue;
      }
      if (auto clamp = dyn_cast<sec::FloatClampCheckedOp>(operation)) {
        // A NaN bound (unordered) or minimum > maximum is invalid; a NaN
        // value propagates through minimumf/maximumf to Ok(NaN) (P20 § 31).
        Value minimum = clamp.getMinimum(), maximum = clamp.getMaximum();
        Value unordered = arith::CmpFOp::create(
            rewriter, loc, arith::CmpFPredicate::UNO, minimum, maximum);
        Value inverted = arith::CmpFOp::create(
            rewriter, loc, arith::CmpFPredicate::OGT, minimum, maximum);
        Value failedFlag =
            arith::OrIOp::create(rewriter, loc, unordered, inverted);
        Value upper =
            arith::MinimumFOp::create(rewriter, loc, clamp.getValue(), maximum);
        Value clamped = arith::MaximumFOp::create(rewriter, loc, upper, minimum);
        rewriter.replaceOp(clamp, ValueRange{clamped, failedFlag});
        continue;
      }
      if (auto property = dyn_cast<sec::FloatPropertyOp>(operation)) {
        auto type = dyn_cast<FloatType>(property.getResult().getType());
        std::optional<APFloat> value =
            type ? propertyValue(type, property.getKind()) : std::nullopt;
        if (!value) {
          property.emitError() << "the Sec MLIR -> LLVM track does not lower "
                                  "'sec.float.property' without a resolved "
                                  "binary format";
          failed = true;
          continue;
        }
        rewriter.replaceOp(property, floatConstant(rewriter, loc, type, *value));
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
