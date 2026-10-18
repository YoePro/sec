// Discharge the P11 nominal enum boundary only in the final binary track.
// P11/P12 and scalar-layout pipelines continue to preserve enum identity.
// Rules:
//   - rules/mlir/lowering-versions/sec_mlir_lowering_v7.md — sections 1–6
//   - rules/memory/layout.md — § 14(1–2, 6)
//   - rules/declarations/enums.md — sections 7, 8, 13–14

#include "sec/Conversion/SecToLLVM/Passes.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlow.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Dialect/Func/Transforms/FuncConversions.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Transforms/DialectConversion.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/APSInt.h"

namespace sec {
#define GEN_PASS_DEF_SECLOWERINTEGERENUMS
#include "sec/Conversion/SecToLLVM/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {

// Select the already resolved native integer representation; no enum domain
// or checked conversion is inferred here (layout.md § 14, lowering v7 § 3).
bool isOrdinaryIntegerEnum(sec::EnumType type) {
  return type.getRepresentation() == "integer" &&
         isa<IntegerType>(type.getUnderlying());
}

// Use the case's exact value, including aliases and negative/wide values,
// rather than its declaration ordinal (enums.md sections 7, 8, 13).
class EnumConstantLowering : public OpConversionPattern<sec::EnumConstantOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult matchAndRewrite(sec::EnumConstantOp op, OpAdaptor,
                                ConversionPatternRewriter &rewriter) const override {
    auto type = cast<sec::EnumType>(op.getResult().getType());
    if (!isOrdinaryIntegerEnum(type))
      return rewriter.notifyMatchFailure(op, "requires a resolved ordinary integer enum");
    auto member = cast<sec::EnumCaseAttr>(type.getCases()[op.getCaseOrdinal()]);
    rewriter.replaceOpWithNewOp<sec::ConstIntOp>(
        op, type.getUnderlying(), member.getValue());
    return success();
  }
};

// Same-enum equality is numeric equality, so aliased cases compare equal
// (enums.md sections 7, 13; lowering v7 section 6).
class EnumCompareLowering : public OpConversionPattern<sec::EnumCmpOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult matchAndRewrite(sec::EnumCmpOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    if (!isOrdinaryIntegerEnum(cast<sec::EnumType>(op.getLeft().getType())))
      return failure();
    rewriter.replaceOpWithNewOp<sec::IntCmpOp>(
        op, rewriter.getI1Type(), op.getPredicate(), adaptor.getLeft(),
        adaptor.getRight());
    return success();
  }
};

// enumToIntegerTarget is the resolved integer an explicit enum-to-integer
// conversion produces, when that conversion is exact: the target is the
// enum's carrier, or every declared case value fits the target. An enum
// value is always one of its declared cases (enums.md sections 7, 13), so
// this decides exactness without a runtime check (MD-104).
IntegerType enumToIntegerTarget(sec::EnumToIntegerOp op) {
  auto type = cast<sec::EnumType>(op.getValue().getType());
  auto target = dyn_cast<IntegerType>(op.getResult().getType());
  if (!isOrdinaryIntegerEnum(type) || !target ||
      (!target.isSigned() && !target.isUnsigned()))
    return IntegerType();
  if (target == type.getUnderlying())
    return target;
  for (Attribute attribute : type.getCases()) {
    llvm::APSInt value(cast<sec::EnumCaseAttr>(attribute).getValue().getValue());
    llvm::APSInt minimum = llvm::APSInt::getMinValue(target.getWidth(), target.isUnsigned());
    llvm::APSInt maximum = llvm::APSInt::getMaxValue(target.getWidth(), target.isUnsigned());
    if (llvm::APSInt::compareValues(value, minimum) < 0 ||
        llvm::APSInt::compareValues(value, maximum) > 0)
      return IntegerType();
  }
  return target;
}

class EnumToIntegerLowering : public OpConversionPattern<sec::EnumToIntegerOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult matchAndRewrite(sec::EnumToIntegerOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    IntegerType target = enumToIntegerTarget(op);
    if (!target)
      return failure();
    if (adaptor.getValue().getType() == target)
      rewriter.replaceOp(op, adaptor.getValue());
    else
      rewriter.replaceOpWithNewOp<sec::NumericConvertExactOp>(op, target,
                                                              adaptor.getValue());
    return success();
  }
};

// Retain storage/call/CFG metadata while substituting only the resolved
// enum representation (lowering v7 sections 1–3; layout.md § 14).
class EnumTransportLowering : public ConversionPattern {
public:
  EnumTransportLowering(TypeConverter &converter, MLIRContext *context,
                        StringRef name)
      : ConversionPattern(converter, name, 1, context) {}
  LogicalResult matchAndRewrite(Operation *op, ArrayRef<Value> operands,
                                ConversionPatternRewriter &rewriter) const override {
    SmallVector<Type> types;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(), types)))
      return failure();
    OperationState state(op->getLoc(), op->getName());
    state.addOperands(operands);
    state.addTypes(types);
    state.addAttributes(op->getAttrs());
    state.addSuccessors(op->getSuccessors());
    Operation *replacement = rewriter.create(state);
    if (auto declaration = dyn_cast<sec::StorageDeclareOp>(op)) {
      auto element = cast<sec::StorageType>(declaration.getStorage().getType())
                         .getElementType();
      if (auto type = dyn_cast<sec::EnumType>(element);
          type && isOrdinaryIntegerEnum(type)) {
        auto integer = cast<IntegerType>(type.getUnderlying());
        // P8 storage normalization needs the resolved carrier's signedness,
        // independently of the enum's source name (layout.md § 14(6)).
        std::string kind = (integer.isSigned() ? "int" : "uint") +
                           std::to_string(integer.getWidth());
        replacement->setAttr("sec.scalar_kind", rewriter.getStringAttr(kind));
      }
    }
    rewriter.replaceOp(op, replacement->getResults());
    return success();
  }
};

// Convert the complete CFG, including enum-valued match merge arguments,
// together with its signature (lowering v7 sections 1–3; enums.md § 14).
class EnumFunctionLowering : public OpConversionPattern<func::FuncOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult matchAndRewrite(func::FuncOp op, OpAdaptor,
                                ConversionPatternRewriter &rewriter) const override {
    TypeConverter::SignatureConversion signature(op.getNumArguments());
    SmallVector<Type> results;
    if (failed(getTypeConverter()->convertSignatureArgs(
            op.getArgumentTypes(), signature)) ||
        failed(getTypeConverter()->convertTypes(op.getResultTypes(), results)))
      return failure();
    if (!op.getBody().empty() &&
        failed(rewriter.convertRegionTypes(&op.getBody(), *getTypeConverter(),
                                           &signature)))
      return failure();
    rewriter.modifyOpInPlace(op, [&] {
      op.setType(FunctionType::get(rewriter.getContext(),
                                   signature.getConvertedTypes(), results));
    });
    return success();
  }
};

// Erase ordinary integer enums after target resolution and before P5/P8
// storage/integer lowering, keeping all other package representation boundaries
// intact (lowering v7 sections 1–6; compiler_pipeline.md §§ 32–34).
struct LowerIntegerEnumsPass
    : sec::impl::SecLowerIntegerEnumsBase<LowerIntegerEnumsPass> {
  void runOnOperation() override {
    MLIRContext *context = &getContext();
    TypeConverter converter;
    converter.addConversion([](Type type) { return type; });
    converter.addConversion([](sec::EnumType type) -> Type {
      return isOrdinaryIntegerEnum(type) ? type.getUnderlying() : type;
    });
    converter.addConversion([&](sec::StorageType type) -> Type {
      return sec::StorageType::get(context, converter.convertType(type.getElementType()));
    });
    ConversionTarget target(*context);
    target.markUnknownOpDynamicallyLegal(
        [&](Operation *op) { return converter.isLegal(op); });
    target.addDynamicallyLegalOp<func::FuncOp>([&](func::FuncOp op) {
      return converter.isSignatureLegal(op.getFunctionType()) &&
             converter.isLegal(&op.getBody());
    });
    target.addDynamicallyLegalOp<sec::EnumConstantOp>([](sec::EnumConstantOp op) {
      return !isOrdinaryIntegerEnum(cast<sec::EnumType>(op.getResult().getType()));
    });
    // A conversion this erasure cannot prove exact is named, not left to a
    // generic legalization failure.
    bool unsupported = false;
    getOperation().walk([&](sec::EnumToIntegerOp op) {
      if (!enumToIntegerTarget(op) &&
          isOrdinaryIntegerEnum(cast<sec::EnumType>(op.getValue().getType()))) {
        op.emitError() << "the Sec MLIR -> LLVM track does not lower "
                          "'sec.enum.to_integer' to "
                       << op.getResult().getType()
                       << " (a case value does not fit) yet";
        unsupported = true;
      }
    });
    if (unsupported)
      return signalPassFailure();
    RewritePatternSet patterns(context);
    patterns.add<EnumConstantLowering, EnumCompareLowering, EnumFunctionLowering,
                 EnumToIntegerLowering>(converter, context);
    populateCallOpTypeConversionPattern(patterns, converter);
    populateReturnOpTypeConversionPattern(patterns, converter);
    for (StringRef name : {sec::StorageDeclareOp::getOperationName(),
                           sec::StorageInitOp::getOperationName(),
                           sec::StorageLoadOp::getOperationName(),
                           sec::StorageStoreOp::getOperationName(),
                           sec::CallDirectOp::getOperationName(),
                           cf::BranchOp::getOperationName(),
                           cf::CondBranchOp::getOperationName()})
      patterns.add<EnumTransportLowering>(converter, context, name);
    if (failed(applyPartialConversion(getOperation(), target, std::move(patterns))))
      signalPassFailure();
  }
};
} // namespace
