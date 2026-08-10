#include "sec/Conversion/SecIntegerToArith/Passes.h"

#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Dialect/Func/Transforms/FuncConversions.h"
#include "mlir/Dialect/MemRef/IR/MemRef.h"
#include "mlir/IR/BuiltinDialect.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Pass/PassManager.h"
#include "mlir/Transforms/DialectConversion.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecDialect.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "sec/Conversion/SecToCore/Passes.h"
#include "llvm/ADT/APInt.h"
#include "llvm/ADT/STLExtras.h"

#include <algorithm>
#include <cctype>

namespace sec {
#define GEN_PASS_DEF_SECLOWERCHECKEDINTEGERS
#include "sec/Conversion/SecIntegerToArith/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {

bool isPlainSemanticInteger(Type type) {
  auto integer = dyn_cast<IntegerType>(type);
  return integer && (integer.isSigned() || integer.isUnsigned());
}

bool containsUnresolvedPlainInteger(Type type) {
  if (isa<sec::IntType, sec::UIntType>(type))
    return true;
  if (auto function = dyn_cast<FunctionType>(type))
    return llvm::any_of(function.getInputs(), containsUnresolvedPlainInteger) ||
           llvm::any_of(function.getResults(), containsUnresolvedPlainInteger);
  if (auto memref = dyn_cast<MemRefType>(type))
    return containsUnresolvedPlainInteger(memref.getElementType());
  if (auto storage = dyn_cast<sec::StorageType>(type))
    return containsUnresolvedPlainInteger(storage.getElementType());
  // Named and distinct types retain their nominal representation in Package 8.
  return false;
}

bool isCheckedIntegerOperation(Operation *operation) {
  return isa<sec::IntUnaryPlusOp, sec::IntNegCheckedOp,
             sec::IntBinaryCheckedOp, sec::IntBitNotOp, sec::IntBitwiseOp,
             sec::IntShiftCheckedOp, sec::IntCmpOp>(operation);
}

IntegerType signlessType(MLIRContext *context, Type type) {
  auto integer = cast<IntegerType>(type);
  return IntegerType::get(context, integer.getWidth());
}

Value integerConstant(ConversionPatternRewriter &rewriter, Location location,
                      IntegerType type, const APInt &value) {
  return arith::ConstantOp::create(
             rewriter, location, type,
             IntegerAttr::get(type, value.zextOrTrunc(type.getWidth())))
      .getResult();
}

Value extendInteger(ConversionPatternRewriter &rewriter, Location location,
                    Value value, IntegerType destination, bool signedValue) {
  auto source = cast<IntegerType>(value.getType());
  if (source.getWidth() == destination.getWidth())
    return value;
  if (source.getWidth() > destination.getWidth())
    return arith::TruncIOp::create(rewriter, location, destination, value);
  if (signedValue)
    return arith::ExtSIOp::create(rewriter, location, destination, value);
  return arith::ExtUIOp::create(rewriter, location, destination, value);
}

Value combineOr(ConversionPatternRewriter &rewriter, Location location,
                Value left, Value right) {
  return arith::OrIOp::create(rewriter, location, left, right);
}

Value arithmeticReasonConstant(ConversionPatternRewriter &rewriter,
                               Location location, StringRef value) {
  OperationState state(location,
      sec::ArithmeticFailureReasonConstantOp::getOperationName());
  state.addAttribute("value", rewriter.getStringAttr(value));
  state.addTypes(sec::ArithmeticFailureReasonType::get(rewriter.getContext()));
  return rewriter.create(state)->getResult(0);
}

Value selectArithmeticReason(ConversionPatternRewriter &rewriter,
                             Location location, Value condition,
                             StringRef trueReason, Value falseReason) {
  Value selected = arithmeticReasonConstant(rewriter, location, trueReason);
  return arith::SelectOp::create(rewriter, location, condition, selected,
                                 falseReason);
}

void replaceCheckedOperation(ConversionPatternRewriter &rewriter,
                             Operation *operation, Value result, Value failed,
                             Value reason) {
  if (operation->getNumResults() == 3)
    rewriter.replaceOp(operation, ValueRange{result, failed, reason});
  else
    rewriter.replaceOp(operation, ValueRange{result, failed});
}

std::optional<APInt> parseInteger(StringRef spelling, unsigned width) {
  bool negative = spelling.consume_front("-");
  spelling.consume_front("+");
  if (spelling.empty() ||
      !llvm::all_of(spelling, [](char value) {
        return std::isdigit(static_cast<unsigned char>(value));
      }))
    return std::nullopt;
  unsigned parseWidth = std::max(width, APInt::getBitsNeeded(spelling, 10));
  APInt value(parseWidth, spelling, 10);
  if (negative)
    value.negate();
  return value.sextOrTrunc(width);
}

bool scalarKindMatchesInteger(StringRef kind, IntegerType type) {
  const bool isSigned = type.isSigned();
  const unsigned width = type.getWidth();
  if (kind == "int")
    return isSigned && (width == 32 || width == 64);
  if (kind == "uint")
    return !isSigned && (width == 32 || width == 64);

  struct FixedKind {
    StringLiteral name;
    unsigned width;
    bool isSigned;
  };
  static constexpr FixedKind fixedKinds[] = {
      {"int8", 8, true},       {"int16", 16, true},
      {"int32", 32, true},     {"int64", 64, true},
      {"int128", 128, true},   {"int256", 256, true},
      {"uint8", 8, false},     {"uint16", 16, false},
      {"uint32", 32, false},   {"uint64", 64, false},
      {"uint128", 128, false}, {"uint256", 256, false},
      {"byte", 8, false},      {"char", 8, false},
      {"rune", 32, false},
  };
  return llvm::any_of(fixedKinds, [&](const FixedKind &candidate) {
    return kind == candidate.name && width == candidate.width &&
           isSigned == candidate.isSigned;
  });
}

LogicalResult verifyScalarKind(Operation *operation, IntegerType type,
                               StringAttr kind, StringRef boundary) {
  if (!kind)
    return operation->emitOpError() << boundary << " requires sec.scalar_kind";
  if (!scalarKindMatchesInteger(kind.getValue(), type))
    return operation->emitOpError()
           << boundary << " has sec.scalar_kind '" << kind.getValue()
           << "' incompatible with " << type;
  return success();
}

bool isSecOriginIntegerStorage(Value value) {
  auto alloca = value.getDefiningOp<memref::AllocaOp>();
  if (!alloca)
    return false;
  auto type = alloca.getType();
  return type.getRank() == 0 && isa<IntegerType>(type.getElementType()) &&
         alloca->hasAttr("sec.storage_id") &&
         alloca->getAttrOfType<StringAttr>("sec.scalar_kind") != nullptr;
}

LogicalResult normalizeSecIntegerStorage(ModuleOp module) {
  WalkResult result = module.walk([&](memref::AllocaOp alloca) {
    auto oldType = alloca.getType();
    if (oldType.getRank() != 0 ||
        !isPlainSemanticInteger(oldType.getElementType()) ||
        !alloca->hasAttr("sec.storage_id"))
      return WalkResult::advance();
    auto elementType = cast<IntegerType>(oldType.getElementType());
    if (failed(verifyScalarKind(
            alloca, elementType,
            alloca->getAttrOfType<StringAttr>("sec.scalar_kind"),
            "Sec-origin integer storage")))
      return WalkResult::interrupt();
    auto converted = MemRefType::get(
        {}, signlessType(module.getContext(), oldType.getElementType()),
        oldType.getLayout(), oldType.getMemorySpace());
    for (Operation *user : alloca.getResult().getUsers()) {
      if (!isa<memref::LoadOp, memref::StoreOp>(user)) {
        alloca.emitOpError(
            "Sec-origin integer storage has an unsupported non-load/store use");
        return WalkResult::interrupt();
      }
    }
    alloca.getResult().setType(converted);
    for (Operation *user : alloca.getResult().getUsers()) {
      if (auto load = dyn_cast<memref::LoadOp>(user))
        load.getResult().setType(converted.getElementType());
    }
    return WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}

class FunctionTypeLowering final : public OpConversionPattern<func::FuncOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(func::FuncOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    FunctionType type = op.getFunctionType();
    TypeConverter::SignatureConversion signature(type.getNumInputs());
    SmallVector<Type> results;
    if (failed(getTypeConverter()->convertSignatureArgs(type.getInputs(),
                                                         signature)) ||
        failed(getTypeConverter()->convertTypes(type.getResults(), results)))
      return failure();
    if (!op.getBody().empty() &&
        failed(rewriter.convertRegionTypes(&op.getBody(), *getTypeConverter(),
                                           &signature)))
      return failure();
    auto converted = FunctionType::get(rewriter.getContext(),
                                       signature.getConvertedTypes(), results);
    rewriter.modifyOpInPlace(op, [&] { op.setType(converted); });
    return success();
  }
};

class SecIntegerStoreLowering final
    : public OpConversionPattern<memref::StoreOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(memref::StoreOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    if (!isSecOriginIntegerStorage(op.getMemref()))
      return failure();
    rewriter.replaceOpWithNewOp<memref::StoreOp>(
        op, adaptor.getValue(), adaptor.getMemref(), adaptor.getIndices(),
        op.getNontemporal());
    return success();
  }
};

class BranchLowering final : public OpConversionPattern<cf::BranchOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult
  matchAndRewrite(cf::BranchOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    rewriter.replaceOpWithNewOp<cf::BranchOp>(op, op.getDest(),
                                               adaptor.getDestOperands());
    return success();
  }
};

class CondBranchLowering final
    : public OpConversionPattern<cf::CondBranchOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult
  matchAndRewrite(cf::CondBranchOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    ArrayRef<int32_t> weights =
        op.getBranchWeights().value_or(ArrayRef<int32_t>{});
    rewriter.replaceOpWithNewOp<cf::CondBranchOp>(
        op, adaptor.getCondition(), op.getTrueDest(),
        adaptor.getTrueDestOperands(), op.getFalseDest(),
        adaptor.getFalseDestOperands(), weights);
    return success();
  }
};

template <typename CallOp>
class SecCallTypeLowering final : public OpConversionPattern<CallOp> {
public:
  using OpConversionPattern<CallOp>::OpConversionPattern;
  using OpAdaptor = typename OpConversionPattern<CallOp>::OpAdaptor;

  LogicalResult
  matchAndRewrite(CallOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    SmallVector<Type> resultTypes;
    if (failed(this->getTypeConverter()->convertTypes(op.getResultTypes(),
                                                       resultTypes)))
      return failure();
    OperationState state(op.getLoc(), op->getName());
    state.addOperands(adaptor.getOperands());
    state.addTypes(resultTypes);
    state.addAttributes(op->getAttrs());
    Operation *replacement = rewriter.create(state);
    rewriter.replaceOp(op, replacement->getResults());
    return success();
  }
};

class SecConstIntLowering final
    : public OpConversionPattern<sec::ConstIntOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult
  matchAndRewrite(sec::ConstIntOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    if (!isPlainSemanticInteger(op.getResult().getType()))
      return failure();
    auto type = signlessType(rewriter.getContext(), op.getResult().getType());
    auto value = parseInteger(op.getValue(), type.getWidth());
    if (!value)
      return op.emitOpError("cannot preserve exact integer value during signless lowering");
    rewriter.replaceOp(op, integerConstant(rewriter, op.getLoc(), type, *value));
    return success();
  }
};

class ArithConstantTypeLowering final
    : public OpConversionPattern<arith::ConstantOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult
  matchAndRewrite(arith::ConstantOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    if (!isPlainSemanticInteger(op.getType()))
      return failure();
    auto value = dyn_cast<IntegerAttr>(op.getValue());
    if (!value)
      return failure();
    auto type = signlessType(rewriter.getContext(), op.getType());
    rewriter.replaceOp(op,
                       integerConstant(rewriter, op.getLoc(), type,
                                       value.getValue()));
    return success();
  }
};

class IntegerSemanticOpLowering final : public ConversionPattern {
public:
  IntegerSemanticOpLowering(TypeConverter &converter, MLIRContext *context,
                            StringRef operationName)
      : ConversionPattern(converter, operationName, 1, context) {}

  LogicalResult
  matchAndRewrite(Operation *operation, ArrayRef<Value> operands,
                  ConversionPatternRewriter &rewriter) const override {
    Location location = operation->getLoc();
    auto originalValueType = dyn_cast<IntegerType>(operation->getOperand(0).getType());
    if (!originalValueType ||
        (!originalValueType.isSigned() && !originalValueType.isUnsigned()))
      return operation->emitOpError(
          "requires resolved signed or unsigned builtin integer operands");
    auto type = cast<IntegerType>(operands[0].getType());
    unsigned width = type.getWidth();
    bool signedValue = originalValueType.isSigned();

    if (isa<sec::IntUnaryPlusOp>(operation)) {
      rewriter.replaceOp(operation, operands[0]);
      return success();
    }
    if (isa<sec::IntBitNotOp>(operation)) {
      Value ones = integerConstant(rewriter, location, type,
                                   APInt::getAllOnes(width));
      rewriter.replaceOp(
          operation,
          arith::XOrIOp::create(rewriter, location, operands[0], ones)
              .getResult());
      return success();
    }
    if (auto bitwise = dyn_cast<sec::IntBitwiseOp>(operation)) {
      Value result;
      if (bitwise.getKind() == "and")
        result = arith::AndIOp::create(rewriter, location, operands[0],
                                      operands[1]);
      else if (bitwise.getKind() == "or")
        result = arith::OrIOp::create(rewriter, location, operands[0],
                                     operands[1]);
      else
        result = arith::XOrIOp::create(rewriter, location, operands[0],
                                      operands[1]);
      rewriter.replaceOp(operation, result);
      return success();
    }
    if (auto comparison = dyn_cast<sec::IntCmpOp>(operation)) {
      arith::CmpIPredicate predicate;
      StringRef kind = comparison.getPredicate();
      if (kind == "eq")
        predicate = arith::CmpIPredicate::eq;
      else if (kind == "ne")
        predicate = arith::CmpIPredicate::ne;
      else if (kind == "lt")
        predicate = signedValue ? arith::CmpIPredicate::slt
                                : arith::CmpIPredicate::ult;
      else if (kind == "le")
        predicate = signedValue ? arith::CmpIPredicate::sle
                                : arith::CmpIPredicate::ule;
      else if (kind == "gt")
        predicate = signedValue ? arith::CmpIPredicate::sgt
                                : arith::CmpIPredicate::ugt;
      else
        predicate = signedValue ? arith::CmpIPredicate::sge
                                : arith::CmpIPredicate::uge;
      rewriter.replaceOp(
          operation, arith::CmpIOp::create(rewriter, location, predicate,
                                           operands[0], operands[1])
                         .getResult());
      return success();
    }
    if (isa<sec::IntNegCheckedOp>(operation)) {
      Value zero = integerConstant(rewriter, location, type, APInt(width, 0));
      Value minimum = integerConstant(rewriter, location, type,
                                      APInt::getSignedMinValue(width));
      Value failed = arith::CmpIOp::create(
          rewriter, location, arith::CmpIPredicate::eq, operands[0], minimum);
      Value result =
          arith::SubIOp::create(rewriter, location, zero, operands[0]);
      Value reason = selectArithmeticReason(
          rewriter, location, failed, "overflow",
          arithmeticReasonConstant(rewriter, location, "none"));
      replaceCheckedOperation(rewriter, operation, result, failed, reason);
      return success();
    }
    if (auto binary = dyn_cast<sec::IntBinaryCheckedOp>(operation))
      return lowerBinary(binary, operands, rewriter, type, signedValue);
    if (auto shift = dyn_cast<sec::IntShiftCheckedOp>(operation))
      return lowerShift(shift, operands, rewriter, type, signedValue);
    return failure();
  }

private:
  LogicalResult lowerBinary(sec::IntBinaryCheckedOp operation,
                            ArrayRef<Value> operands,
                            ConversionPatternRewriter &rewriter,
                            IntegerType type, bool signedValue) const {
    Location location = operation.getLoc();
    unsigned width = type.getWidth();
    StringRef kind = operation.getKind();
    Value left = operands[0];
    Value right = operands[1];

    if (kind == "divide" || kind == "remainder") {
      Value zero = integerConstant(rewriter, location, type, APInt(width, 0));
      Value one = integerConstant(rewriter, location, type, APInt(width, 1));
      Value divisionByZero = arith::CmpIOp::create(
          rewriter, location, arith::CmpIPredicate::eq, right, zero);
      Value overflow = arith::ConstantIntOp::create(rewriter, location, 0, 1);
      if (signedValue) {
        Value minimum = integerConstant(rewriter, location, type,
                                        APInt::getSignedMinValue(width));
        Value minusOne = integerConstant(rewriter, location, type,
                                         APInt::getAllOnes(width));
        Value leftMinimum = arith::CmpIOp::create(
            rewriter, location, arith::CmpIPredicate::eq, left, minimum);
        Value rightMinusOne = arith::CmpIOp::create(
            rewriter, location, arith::CmpIPredicate::eq, right, minusOne);
    overflow = arith::AndIOp::create(rewriter, location, leftMinimum,
                                       rightMinusOne);
      }
    Value failed = combineOr(rewriter, location, divisionByZero, overflow);
      Value safeRight =
          arith::SelectOp::create(rewriter, location, failed, one, right);
      Value result;
      if (kind == "divide")
        result = signedValue
                     ? Value(arith::DivSIOp::create(rewriter, location, left,
                                                    safeRight))
                     : Value(arith::DivUIOp::create(rewriter, location, left,
                                                    safeRight));
      else
        result = signedValue
                     ? Value(arith::RemSIOp::create(rewriter, location, left,
                                                    safeRight))
                     : Value(arith::RemUIOp::create(rewriter, location, left,
                                                    safeRight));
    Value reason = selectArithmeticReason(
      rewriter, location, overflow, "overflow",
      arithmeticReasonConstant(rewriter, location, "none"));
    reason = selectArithmeticReason(rewriter, location, divisionByZero,
                                  "division-by-zero", reason);
    replaceCheckedOperation(rewriter, operation, result, failed, reason);
      return success();
    }

    if (kind == "subtract" && !signedValue) {
      Value result = arith::SubIOp::create(rewriter, location, left, right);
      Value failed = arith::CmpIOp::create(
          rewriter, location, arith::CmpIPredicate::ult, left, right);
    Value reason = selectArithmeticReason(
      rewriter, location, failed, "overflow",
      arithmeticReasonConstant(rewriter, location, "none"));
    replaceCheckedOperation(rewriter, operation, result, failed, reason);
      return success();
    }

    unsigned wideWidth = kind == "multiply" ? width * 2 : width + 1;
    auto wideType = IntegerType::get(rewriter.getContext(), wideWidth);
    Value wideLeft =
        extendInteger(rewriter, location, left, wideType, signedValue);
    Value wideRight =
        extendInteger(rewriter, location, right, wideType, signedValue);
    Value wideResult;
    if (kind == "add")
      wideResult =
          arith::AddIOp::create(rewriter, location, wideLeft, wideRight);
    else if (kind == "subtract")
      wideResult =
          arith::SubIOp::create(rewriter, location, wideLeft, wideRight);
    else
      wideResult =
          arith::MulIOp::create(rewriter, location, wideLeft, wideRight);
    Value result = arith::TruncIOp::create(rewriter, location, type, wideResult);
    APInt minimum = signedValue ? APInt::getSignedMinValue(width).sext(wideWidth)
                                : APInt(wideWidth, 0);
    APInt maximum = signedValue ? APInt::getSignedMaxValue(width).sext(wideWidth)
                                : APInt::getMaxValue(width).zext(wideWidth);
    Value minimumValue = integerConstant(rewriter, location, wideType, minimum);
    Value maximumValue = integerConstant(rewriter, location, wideType, maximum);
    Value below = arith::CmpIOp::create(
        rewriter, location,
        signedValue ? arith::CmpIPredicate::slt : arith::CmpIPredicate::ult,
        wideResult, minimumValue);
    Value above = arith::CmpIOp::create(
        rewriter, location,
        signedValue ? arith::CmpIPredicate::sgt : arith::CmpIPredicate::ugt,
        wideResult, maximumValue);
    Value failed = combineOr(rewriter, location, below, above);
  Value reason = selectArithmeticReason(
    rewriter, location, failed, "overflow",
    arithmeticReasonConstant(rewriter, location, "none"));
  replaceCheckedOperation(rewriter, operation, result, failed, reason);
    return success();
  }

  LogicalResult lowerShift(sec::IntShiftCheckedOp operation,
                           ArrayRef<Value> operands,
                           ConversionPatternRewriter &rewriter,
                           IntegerType type, bool signedValue) const {
    Location location = operation.getLoc();
    unsigned width = type.getWidth();
    auto originalCountType =
        cast<IntegerType>(operation.getCount().getType());
    auto countType = cast<IntegerType>(operands[1].getType());
    unsigned checkWidth =
        std::max(width + 1, countType.getWidth() + 1);
    auto checkType = IntegerType::get(rewriter.getContext(), checkWidth);
    bool signedCount = originalCountType.isSigned();
    Value countWide = extendInteger(rewriter, location, operands[1], checkType,
                                    signedCount);
    Value zeroWide =
        integerConstant(rewriter, location, checkType, APInt(checkWidth, 0));
    Value widthWide = integerConstant(rewriter, location, checkType,
                                      APInt(checkWidth, width));
    Value tooLarge = arith::CmpIOp::create(
        rewriter, location, arith::CmpIPredicate::uge, countWide, widthWide);
    Value invalid = tooLarge;
    if (signedCount) {
      Value negative = arith::CmpIOp::create(
          rewriter, location, arith::CmpIPredicate::slt, countWide, zeroWide);
      invalid = combineOr(rewriter, location, negative, tooLarge);
    }
    Value safeWide = arith::SelectOp::create(rewriter, location, invalid,
                                             zeroWide, countWide);
    Value safeCount =
        extendInteger(rewriter, location, safeWide, type, false);
    StringRef kind = operation.getKind();
    Value result;
    Value failed = invalid;
  Value overflow = arith::ConstantIntOp::create(rewriter, location, 0, 1);
    if (kind == "left_signed") {
      auto wideType = IntegerType::get(rewriter.getContext(), width * 2);
      Value valueWide =
          extendInteger(rewriter, location, operands[0], wideType, true);
      Value countForWide =
          extendInteger(rewriter, location, safeCount, wideType, false);
      Value shifted = arith::ShLIOp::create(rewriter, location, valueWide,
                                           countForWide);
      Value minimum = integerConstant(
          rewriter, location, wideType,
          APInt::getSignedMinValue(width).sext(width * 2));
      Value maximum = integerConstant(
          rewriter, location, wideType,
          APInt::getSignedMaxValue(width).sext(width * 2));
      Value below = arith::CmpIOp::create(
          rewriter, location, arith::CmpIPredicate::slt, shifted, minimum);
      Value above = arith::CmpIOp::create(
          rewriter, location, arith::CmpIPredicate::sgt, shifted, maximum);
    overflow = combineOr(rewriter, location, below, above);
      failed = combineOr(rewriter, location, invalid, overflow);
      result = arith::TruncIOp::create(rewriter, location, type, shifted);
    } else if (kind == "left_unsigned") {
      result = arith::ShLIOp::create(rewriter, location, operands[0], safeCount);
    } else if (kind == "right_signed") {
      result =
          arith::ShRSIOp::create(rewriter, location, operands[0], safeCount);
    } else {
      result =
          arith::ShRUIOp::create(rewriter, location, operands[0], safeCount);
    }
  Value reason = selectArithmeticReason(
    rewriter, location, overflow, "overflow",
    arithmeticReasonConstant(rewriter, location, "none"));
  reason = selectArithmeticReason(rewriter, location, invalid,
                                  "invalid-shift", reason);
  replaceCheckedOperation(rewriter, operation, result, failed, reason);
    return success();
  }
};

class LowerCheckedIntegersPass final
    : public sec::impl::SecLowerCheckedIntegersBase<
          LowerCheckedIntegersPass> {
public:
  void runOnOperation() override {
    ModuleOp module = getOperation();
    WalkResult unresolvedTypes = module.walk([&](Operation *operation) {
      bool unresolved = containsUnresolvedPlainInteger(
          FunctionType::get(&getContext(), operation->getOperandTypes(),
                            operation->getResultTypes()));
      if (auto function = dyn_cast<func::FuncOp>(operation))
        unresolved |= containsUnresolvedPlainInteger(function.getFunctionType());
      for (Region &region : operation->getRegions())
        for (Block &block : region)
          unresolved |= llvm::any_of(block.getArgumentTypes(),
                                     containsUnresolvedPlainInteger);
      if (!unresolved)
        return WalkResult::advance();
      operation->emitOpError(
          "target-sized !sec.int/!sec.uint must be resolved before checked "
          "integer lowering");
      return WalkResult::interrupt();
    });
    if (unresolvedTypes.wasInterrupted())
      return signalPassFailure();

    WalkResult preconditions = module.walk([&](func::FuncOp function) {
      if (failed(sec::verifyCheckedIntegerGuards(function)))
        return WalkResult::interrupt();
      if (!function.isExternal())
        return WalkResult::advance();
      for (auto [index, type] : llvm::enumerate(function.getArgumentTypes())) {
        auto integer = dyn_cast<IntegerType>(type);
        if (integer && (integer.isSigned() || integer.isUnsigned()) &&
            failed(verifyScalarKind(
                function, integer,
                function.getArgAttrOfType<StringAttr>(index, "sec.scalar_kind"),
                (Twine("extern integer argument ") + Twine(index)).str())))
          return WalkResult::interrupt();
      }
      for (auto [index, type] : llvm::enumerate(function.getResultTypes())) {
        auto integer = dyn_cast<IntegerType>(type);
        if (integer && (integer.isSigned() || integer.isUnsigned()) &&
            failed(verifyScalarKind(
                function, integer,
                function.getResultAttrOfType<StringAttr>(index,
                                                          "sec.scalar_kind"),
                (Twine("extern integer result ") + Twine(index)).str())))
          return WalkResult::interrupt();
      }
      return WalkResult::advance();
    });
    if (preconditions.wasInterrupted())
      return signalPassFailure();
    if (failed(normalizeSecIntegerStorage(module)))
      return signalPassFailure();

    MLIRContext *context = &getContext();
    TypeConverter typeConverter;
    typeConverter.addConversion([](Type type) { return type; });
    typeConverter.addConversion([&](IntegerType type) -> Type {
      if (!type.isSigned() && !type.isUnsigned())
        return type;
      return IntegerType::get(context, type.getWidth());
    });

    ConversionTarget target(*context);
    target.addLegalDialect<BuiltinDialect, arith::ArithDialect,
                           func::FuncDialect, cf::ControlFlowDialect,
                           memref::MemRefDialect>();
    target.addLegalDialect<sec::SecDialect>();
    target.addIllegalOp<sec::IntUnaryPlusOp, sec::IntNegCheckedOp,
                        sec::IntBinaryCheckedOp, sec::IntBitNotOp,
                        sec::IntBitwiseOp, sec::IntShiftCheckedOp,
                        sec::IntCmpOp>();
    target.addDynamicallyLegalOp<func::FuncOp>([&](func::FuncOp op) {
      if (!typeConverter.isSignatureLegal(op.getFunctionType()))
        return false;
      return llvm::all_of(op.getBody(), [&](Block &block) {
        return llvm::all_of(block.getArgumentTypes(), [&](Type type) {
          return typeConverter.isLegal(type);
        });
      });
    });
    target.addDynamicallyLegalOp<func::CallOp, func::ReturnOp,
                                 cf::BranchOp, cf::CondBranchOp,
                                 sec::CallDirectOp, sec::CallForeignOp>(
        [&](Operation *op) { return typeConverter.isLegal(op); });
    target.addDynamicallyLegalOp<sec::ConstIntOp>([](sec::ConstIntOp op) {
      return !isPlainSemanticInteger(op.getResult().getType());
    });
    target.addDynamicallyLegalOp<arith::ConstantOp>([&](arith::ConstantOp op) {
      return typeConverter.isLegal(op.getType());
    });
    target.addDynamicallyLegalOp<memref::StoreOp>([](memref::StoreOp op) {
      if (!isSecOriginIntegerStorage(op.getMemref()))
        return true;
      return op.getValue().getType() == op.getMemRefType().getElementType();
    });
    RewritePatternSet patterns(context);
    patterns.add<FunctionTypeLowering, BranchLowering, CondBranchLowering,
                 SecIntegerStoreLowering,
                 SecCallTypeLowering<sec::CallDirectOp>,
                 SecCallTypeLowering<sec::CallForeignOp>, SecConstIntLowering,
                 ArithConstantTypeLowering>(typeConverter, context);
    for (StringRef operationName : {
             sec::IntUnaryPlusOp::getOperationName(),
             sec::IntNegCheckedOp::getOperationName(),
             sec::IntBinaryCheckedOp::getOperationName(),
             sec::IntBitNotOp::getOperationName(),
             sec::IntBitwiseOp::getOperationName(),
             sec::IntShiftCheckedOp::getOperationName(),
             sec::IntCmpOp::getOperationName()})
      patterns.add<IntegerSemanticOpLowering>(typeConverter, context,
                                              operationName);
    populateCallOpTypeConversionPattern(patterns, typeConverter);
    populateReturnOpTypeConversionPattern(patterns, typeConverter);

    if (failed(applyPartialConversion(module, target, std::move(patterns))))
      return signalPassFailure();

    WalkResult remaining = module.walk([&](Operation *operation) {
      if (!isCheckedIntegerOperation(operation))
        return WalkResult::advance();
      operation->emitOpError(
          "checked integer operation remained after Package 8 lowering");
      return WalkResult::interrupt();
    });
    if (remaining.wasInterrupted())
      signalPassFailure();
  }
};

} // namespace

std::unique_ptr<mlir::Pass> sec::createSecLowerCheckedIntegersPass() {
  return std::make_unique<LowerCheckedIntegersPass>();
}

void sec::registerSecIntegerToArithPipelines() {
  static PassPipelineRegistration<> pipeline(
      "sec-lower-integer-core",
      "Resolve Sec scalar layout and lower checked integer semantics to Arith",
      [](OpPassManager &manager) {
        manager.addPass(sec::createSecLowerTrivialCorePass());
        manager.addPass(sec::createSecResolveScalarLayoutPass());
        manager.addPass(sec::createSecLowerTrivialCorePass());
        manager.addPass(sec::createSecLowerCheckedIntegersPass());
      });
  (void)pipeline;
}
