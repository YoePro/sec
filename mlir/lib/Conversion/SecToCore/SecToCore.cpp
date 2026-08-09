#include "sec/Conversion/SecToCore/Passes.h"

#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlow.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Dialect/Func/Transforms/FuncConversions.h"
#include "mlir/Dialect/MemRef/IR/MemRef.h"
#include "mlir/IR/BuiltinDialect.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Interfaces/DataLayoutInterfaces.h"
#include "mlir/Pass/PassManager.h"
#include "mlir/Transforms/DialectConversion.h"
#include "sec/Dialect/Sec/SecDialect.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/APInt.h"

#include <cctype>

namespace sec {
#define GEN_PASS_DEF_SECLOWERTRIVIALCORE
#define GEN_PASS_DEF_SECRESOLVESCALARLAYOUT
#include "sec/Conversion/SecToCore/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {

bool isLowerableElementType(Type type) {
  if (auto integer = dyn_cast<IntegerType>(type)) {
    switch (integer.getWidth()) {
    case 8:
    case 16:
    case 32:
    case 64:
    case 128:
    case 256:
      return true;
    default:
      return false;
    }
  }
  return type.isF32() || type.isF64();
}

bool isLowerableStorageType(Type type) {
  auto storage = dyn_cast<sec::StorageType>(type);
  return storage && isLowerableElementType(storage.getElementType());
}

struct ParsedInteger {
  APInt magnitude;
  bool negative;
};

FailureOr<ParsedInteger> parseExactInteger(StringRef spelling) {
  bool negative = spelling.consume_front("-");
  spelling.consume_front("+");
  if (spelling.empty() ||
      !llvm::all_of(spelling, [](char value) {
        return std::isdigit(static_cast<unsigned char>(value));
      }))
    return failure();
  unsigned bits = std::max(1u, APInt::getBitsNeeded(spelling, 10));
  return ParsedInteger{APInt(bits, spelling, 10), negative};
}

bool exactIntegerFits(const ParsedInteger &value, IntegerType type) {
  unsigned width = type.getWidth();
  if (type.isUnsigned())
    return !value.negative && value.magnitude.getActiveBits() <= width;
  if (type.isSignless()) {
    if (!value.negative)
      return value.magnitude.getActiveBits() <= width;
  } else if (!type.isSigned()) {
    return false;
  }
  unsigned activeBits = value.magnitude.getActiveBits();
  if (!value.negative)
    return activeBits <= width - 1;
  return activeBits < width ||
         (activeBits == width && value.magnitude.isPowerOf2());
}

FailureOr<IntegerAttr> exactIntegerAttribute(MLIRContext *context,
                                             IntegerType type,
                                             StringRef spelling) {
  auto parsed = parseExactInteger(spelling);
  if (failed(parsed) || !exactIntegerFits(*parsed, type))
    return failure();
  APInt value = parsed->magnitude.zextOrTrunc(type.getWidth());
  if (parsed->negative)
    value.negate();
  return IntegerAttr::get(type, value);
}

class ConstBoolLowering final
    : public OpConversionPattern<sec::ConstBoolOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::ConstBoolOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    auto value = rewriter.getIntegerAttr(rewriter.getI1Type(),
                                         op.getValue() ? 1 : 0);
    rewriter.replaceOpWithNewOp<arith::ConstantOp>(op, value);
    return success();
  }
};

class StorageDeclareLowering final
    : public OpConversionPattern<sec::StorageDeclareOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::StorageDeclareOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    auto converted = dyn_cast_or_null<MemRefType>(
        getTypeConverter()->convertType(op.getStorage().getType()));
    if (!converted || converted.getRank() != 0)
      return failure();

    auto alloca = memref::AllocaOp::create(rewriter, op.getLoc(), converted);
    for (StringRef name : {"sec.storage_id", "sec.source_name",
                           "sec.storage_class", "sec.mutable",
                           "sec.scalar_kind"}) {
      if (Attribute attribute = op->getAttr(name))
        alloca->setAttr(name, attribute);
    }
    rewriter.replaceOp(op, alloca.getResult());
    return success();
  }
};

class StorageInitLowering final
    : public OpConversionPattern<sec::StorageInitOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::StorageInitOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    if (!isa<MemRefType>(adaptor.getStorage().getType()))
      return failure();
    memref::StoreOp::create(rewriter, op.getLoc(), adaptor.getValue(),
                            adaptor.getStorage(), ValueRange{});
    rewriter.eraseOp(op);
    return success();
  }
};

class StorageLoadLowering final
    : public OpConversionPattern<sec::StorageLoadOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::StorageLoadOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    if (!isa<MemRefType>(adaptor.getStorage().getType()))
      return failure();
    rewriter.replaceOpWithNewOp<memref::LoadOp>(op, adaptor.getStorage(),
                                                ValueRange{});
    return success();
  }
};

class StorageStoreLowering final
    : public OpConversionPattern<sec::StorageStoreOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::StorageStoreOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    if (!isa<MemRefType>(adaptor.getStorage().getType()))
      return failure();
    memref::StoreOp::create(rewriter, op.getLoc(), adaptor.getValue(),
                            adaptor.getStorage(), ValueRange{});
    rewriter.eraseOp(op);
    return success();
  }
};

class DirectCallLowering final
    : public OpConversionPattern<sec::CallDirectOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::CallDirectOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    auto actions = op->getAttrOfType<ArrayAttr>("sec.argument_actions");
    if (!actions || actions.size() != adaptor.getArguments().size())
      return rewriter.notifyMatchFailure(op, "invalid argument action count");
    if (llvm::any_of(actions, [](Attribute action) {
          auto text = dyn_cast<StringAttr>(action);
          return !text || text.getValue() != "copy-trivial";
        }))
      return rewriter.notifyMatchFailure(op,
                                         "unsupported argument action");

    rewriter.replaceOpWithNewOp<func::CallOp>(
        op, op.getCallee(), op.getResultTypes(), adaptor.getArguments());
    return success();
  }
};

class ScalarConstIntLowering final
    : public OpConversionPattern<sec::ConstIntOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::ConstIntOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    Type converted = getTypeConverter()->convertType(op.getResult().getType());
    if (!converted)
      return failure();
    if (auto integer = dyn_cast<IntegerType>(converted);
        integer && integer.isSignless()) {
      auto value = exactIntegerAttribute(rewriter.getContext(), integer,
                                         op.getValue());
      if (failed(value))
        return rewriter.notifyMatchFailure(
            op, "integer is not representable after scalar resolution");
      rewriter.replaceOpWithNewOp<arith::ConstantOp>(op, *value);
      return success();
    }
    rewriter.replaceOpWithNewOp<sec::ConstIntOp>(op, converted,
                                                 op.getValueAttr());
    return success();
  }
};

class ScalarConstFloatLowering final
    : public OpConversionPattern<sec::ConstFloatOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::ConstFloatOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    Type converted = getTypeConverter()->convertType(op.getResult().getType());
    if (!converted)
      return failure();
    rewriter.replaceOpWithNewOp<sec::ConstFloatOp>(op, converted,
                                                   op.getLexemeAttr());
    return success();
  }
};

class ScalarStorageDeclareLowering final
    : public OpConversionPattern<sec::StorageDeclareOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::StorageDeclareOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    Type converted = getTypeConverter()->convertType(op.getStorage().getType());
    if (!converted)
      return failure();
    if (auto memref = dyn_cast<MemRefType>(converted)) {
      auto alloca = memref::AllocaOp::create(rewriter, op.getLoc(), memref);
      for (NamedAttribute attribute : op->getAttrs())
        alloca->setAttr(attribute.getName(), attribute.getValue());
      rewriter.replaceOp(op, alloca.getResult());
      return success();
    }
    auto replacement = sec::StorageDeclareOp::create(
        rewriter, op.getLoc(), TypeRange{converted}, ValueRange{},
        op->getAttrs());
    rewriter.replaceOp(op, replacement.getResult());
    return success();
  }
};

template <typename StorageOp>
LogicalResult lowerBoolStorageWrite(StorageOp op, Value storage, Value value,
                                    ConversionPatternRewriter &rewriter) {
  if (!isa<MemRefType>(storage.getType()))
    return failure();
  auto byte = arith::ExtUIOp::create(rewriter, op.getLoc(),
                                     rewriter.getI8Type(), value);
  memref::StoreOp::create(rewriter, op.getLoc(), byte, storage, ValueRange{});
  rewriter.eraseOp(op);
  return success();
}

class ScalarStorageInitLowering final
    : public OpConversionPattern<sec::StorageInitOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::StorageInitOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    if (isa<MemRefType>(adaptor.getStorage().getType()))
      return lowerBoolStorageWrite(op, adaptor.getStorage(), adaptor.getValue(),
                                   rewriter);
    rewriter.replaceOpWithNewOp<sec::StorageInitOp>(
        op, adaptor.getStorage(), adaptor.getValue());
    return success();
  }
};

class ScalarStorageStoreLowering final
    : public OpConversionPattern<sec::StorageStoreOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::StorageStoreOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    if (isa<MemRefType>(adaptor.getStorage().getType()))
      return lowerBoolStorageWrite(op, adaptor.getStorage(), adaptor.getValue(),
                                   rewriter);
    rewriter.replaceOpWithNewOp<sec::StorageStoreOp>(
        op, adaptor.getStorage(), adaptor.getValue());
    return success();
  }
};

class ScalarStorageLoadLowering final
    : public OpConversionPattern<sec::StorageLoadOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::StorageLoadOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    Type convertedResult =
        getTypeConverter()->convertType(op.getResult().getType());
    if (!convertedResult)
      return failure();
    if (auto memref = dyn_cast<MemRefType>(adaptor.getStorage().getType())) {
      auto byte = memref::LoadOp::create(rewriter, op.getLoc(),
                                         adaptor.getStorage(), ValueRange{});
      rewriter.replaceOpWithNewOp<arith::TruncIOp>(op, convertedResult, byte);
      return success();
    }
    rewriter.replaceOpWithNewOp<sec::StorageLoadOp>(
        op, convertedResult, adaptor.getStorage());
    return success();
  }
};

template <typename CallOp>
class ScalarSecCallLowering final : public OpConversionPattern<CallOp> {
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
    auto replacement = CallOp::create(rewriter, op.getLoc(), resultTypes,
                                      op.getCalleeAttr(),
                                      adaptor.getArguments());
    if (Attribute actions = op->getAttr("sec.argument_actions"))
      replacement->setAttr("sec.argument_actions", actions);
    rewriter.replaceOp(op, replacement.getResults());
    return success();
  }
};

class ScalarBranchLowering final : public OpConversionPattern<cf::BranchOp> {
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

class ScalarCondBranchLowering final
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

class ScalarFunctionLowering final
    : public OpConversionPattern<func::FuncOp> {
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

class ScalarSemanticIntegerLowering final : public ConversionPattern {
public:
  ScalarSemanticIntegerLowering(TypeConverter &converter, MLIRContext *context,
                                StringRef operationName)
      : ConversionPattern(converter, operationName, 1, context) {}

  LogicalResult
  matchAndRewrite(Operation *operation, ArrayRef<Value> operands,
                  ConversionPatternRewriter &rewriter) const override {
    SmallVector<Type> resultTypes;
    if (failed(getTypeConverter()->convertTypes(operation->getResultTypes(),
                                                 resultTypes)))
      return failure();
    OperationState state(operation->getLoc(), operation->getName());
    state.addOperands(operands);
    state.addTypes(resultTypes);
    state.addAttributes(operation->getAttrs());
    Operation *replacement = rewriter.create(state);
    rewriter.replaceOp(operation, replacement->getResults());
    return success();
  }
};

class LowerTrivialCorePass final
    : public sec::impl::SecLowerTrivialCoreBase<LowerTrivialCorePass> {
public:
  void runOnOperation() override {
    MLIRContext *context = &getContext();
    TypeConverter typeConverter;
    typeConverter.addConversion([](Type type) { return type; });
    typeConverter.addConversion([](sec::StorageType storage)
                                    -> std::optional<Type> {
      if (!isLowerableElementType(storage.getElementType()))
        return std::nullopt;
      return MemRefType::get({}, storage.getElementType());
    });

    ConversionTarget target(*context);
    target.addLegalDialect<BuiltinDialect, arith::ArithDialect,
                           func::FuncDialect, cf::ControlFlowDialect,
                           memref::MemRefDialect>();
    target.addLegalDialect<sec::SecDialect>();
    target.addIllegalOp<sec::ConstBoolOp, sec::CallDirectOp>();
    target.addDynamicallyLegalOp<sec::StorageDeclareOp>(
        [](sec::StorageDeclareOp op) {
          return !isLowerableStorageType(op.getStorage().getType());
        });
    target.addDynamicallyLegalOp<sec::StorageInitOp>(
        [](sec::StorageInitOp op) {
          return !isLowerableStorageType(op.getStorage().getType());
        });
    target.addDynamicallyLegalOp<sec::StorageLoadOp>(
        [](sec::StorageLoadOp op) {
          return !isLowerableStorageType(op.getStorage().getType());
        });
    target.addDynamicallyLegalOp<sec::StorageStoreOp>(
        [](sec::StorageStoreOp op) {
          return !isLowerableStorageType(op.getStorage().getType());
        });

    RewritePatternSet patterns(context);
    patterns.add<ConstBoolLowering, StorageDeclareLowering,
                 StorageInitLowering, StorageLoadLowering,
                 StorageStoreLowering, DirectCallLowering>(typeConverter,
                                                            context);
    if (failed(applyPartialConversion(getOperation(), target,
                                      std::move(patterns))))
      signalPassFailure();
  }
};

class ResolveScalarLayoutPass final
    : public sec::impl::SecResolveScalarLayoutBase<ResolveScalarLayoutPass> {
public:
  void runOnOperation() override {
    ModuleOp module = getOperation();
    if (!module->hasAttr("dlti.dl_spec")) {
      module.emitError("scalar layout resolution requires explicit dlti.dl_spec");
      return signalPassFailure();
    }
    WalkResult legacyBoolStorage = module.walk([&](memref::AllocaOp op) {
      auto type = op.getType();
      auto integer = dyn_cast<IntegerType>(type.getElementType());
      if (type.getRank() == 0 && integer && integer.isSignless() &&
          integer.getWidth() == 1 && op->hasAttr("sec.storage_id")) {
        op.emitError(
            "legacy Sec bool storage memref<i1> is invalid; addressable bool storage requires memref<i8>");
        return WalkResult::interrupt();
      }
      return WalkResult::advance();
    });
    if (legacyBoolStorage.wasInterrupted())
      return signalPassFailure();

    MLIRContext *context = &getContext();
    DataLayout dataLayout(module);
    llvm::TypeSize indexSize =
        dataLayout.getTypeSizeInBits(IndexType::get(context));
    if (indexSize.isScalable()) {
      module.emitError("scalar layout index width must be fixed");
      return signalPassFailure();
    }
    uint64_t pointerWidth = indexSize.getFixedValue();
    if (pointerWidth != 32 && pointerWidth != 64) {
      module.emitError("scalar layout index width must be 32 or 64");
      return signalPassFailure();
    }

    TypeConverter typeConverter;
    typeConverter.addConversion([](Type type) { return type; });
    typeConverter.addConversion([&](sec::IntType) -> Type {
      return IntegerType::get(context, pointerWidth,
                              IntegerType::SignednessSemantics::Signed);
    });
    typeConverter.addConversion([&](sec::UIntType) -> Type {
      return IntegerType::get(context, pointerWidth,
                              IntegerType::SignednessSemantics::Unsigned);
    });
    typeConverter.addConversion(
        [&](sec::FloatType) -> Type { return Float64Type::get(context); });
    typeConverter.addConversion([&](sec::CharType) -> Type {
      return IntegerType::get(context, 8,
                              IntegerType::SignednessSemantics::Unsigned);
    });
    typeConverter.addConversion([&](sec::RuneType) -> Type {
      return IntegerType::get(context, 32,
                              IntegerType::SignednessSemantics::Unsigned);
    });
    typeConverter.addConversion(
        [&](sec::NamedType type) -> std::optional<Type> {
          Type base = typeConverter.convertType(type.getBase());
          if (!base)
            return std::nullopt;
          return sec::NamedType::get(context, type.getIdentity(), base);
        });
    typeConverter.addConversion(
        [&](sec::DistinctType type) -> std::optional<Type> {
          Type base = typeConverter.convertType(type.getBase());
          if (!base)
            return std::nullopt;
          return sec::DistinctType::get(context, type.getIdentity(), base);
        });
    typeConverter.addConversion(
        [&](sec::StorageType type) -> std::optional<Type> {
          Type element = type.getElementType();
          if (auto integer = dyn_cast<IntegerType>(element);
              integer && integer.isSignless() && integer.getWidth() == 1)
            return MemRefType::get({}, rewriterI8Type(context));
          Type converted = typeConverter.convertType(element);
          if (!converted)
            return std::nullopt;
          if (converted == element)
            return type;
          return sec::StorageType::get(context, converted);
        });

    ConversionTarget target(*context);
    target.addLegalDialect<BuiltinDialect, arith::ArithDialect,
                           func::FuncDialect, cf::ControlFlowDialect,
                           memref::MemRefDialect>();
    target.addLegalDialect<sec::SecDialect>();
    target.addDynamicallyLegalOp<func::FuncOp>([&](func::FuncOp op) {
      if (!typeConverter.isSignatureLegal(op.getFunctionType()))
        return false;
      return llvm::all_of(op.getBody(), [&](Block &block) {
        return llvm::all_of(block.getArgumentTypes(),
                            [&](Type type) { return typeConverter.isLegal(type); });
      });
    });
    target.addDynamicallyLegalOp<func::CallOp, func::ReturnOp,
                                 cf::BranchOp, cf::CondBranchOp>(
        [&](Operation *op) { return typeConverter.isLegal(op); });
    target.addDynamicallyLegalOp<sec::ConstIntOp>([&](sec::ConstIntOp op) {
      auto integer = dyn_cast<IntegerType>(op.getResult().getType());
      return typeConverter.isLegal(op.getOperation()) &&
             (!integer || !integer.isSignless());
    });
    target.addDynamicallyLegalOp<sec::ConstFloatOp>(
        [&](sec::ConstFloatOp op) {
          return typeConverter.isLegal(op.getOperation());
        });
    target.addDynamicallyLegalOp<sec::StorageDeclareOp, sec::StorageInitOp,
                                 sec::StorageLoadOp, sec::StorageStoreOp,
                                 sec::CallDirectOp, sec::CallForeignOp>(
        [&](Operation *op) { return typeConverter.isLegal(op); });
    target.addDynamicallyLegalOp<
        sec::IntUnaryPlusOp, sec::IntNegCheckedOp,
        sec::IntBinaryCheckedOp, sec::IntBitNotOp, sec::IntBitwiseOp,
        sec::IntShiftCheckedOp, sec::IntCmpOp>(
        [&](Operation *op) { return typeConverter.isLegal(op); });

    RewritePatternSet patterns(context);
    patterns.add<ScalarConstIntLowering, ScalarConstFloatLowering,
                 ScalarStorageDeclareLowering, ScalarStorageInitLowering,
                 ScalarStorageLoadLowering, ScalarStorageStoreLowering,
                 ScalarSecCallLowering<sec::CallDirectOp>,
                 ScalarSecCallLowering<sec::CallForeignOp>,
                 ScalarBranchLowering,
                 ScalarCondBranchLowering,
                 ScalarFunctionLowering>(typeConverter, context);
    for (StringRef operationName : {
             sec::IntUnaryPlusOp::getOperationName(),
             sec::IntNegCheckedOp::getOperationName(),
             sec::IntBinaryCheckedOp::getOperationName(),
             sec::IntBitNotOp::getOperationName(),
             sec::IntBitwiseOp::getOperationName(),
             sec::IntShiftCheckedOp::getOperationName(),
             sec::IntCmpOp::getOperationName()})
      patterns.add<ScalarSemanticIntegerLowering>(typeConverter, context,
                                                  operationName);
    populateCallOpTypeConversionPattern(patterns, typeConverter);
    populateReturnOpTypeConversionPattern(patterns, typeConverter);

    if (failed(applyPartialConversion(module, target, std::move(patterns))))
      signalPassFailure();
  }

private:
  static IntegerType rewriterI8Type(MLIRContext *context) {
    return IntegerType::get(context, 8);
  }
};

} // namespace

std::unique_ptr<mlir::Pass> sec::createSecLowerTrivialCorePass() {
  return std::make_unique<LowerTrivialCorePass>();
}

std::unique_ptr<mlir::Pass> sec::createSecResolveScalarLayoutPass() {
  return std::make_unique<ResolveScalarLayoutPass>();
}

void sec::registerSecToCorePipelines() {
  static PassPipelineRegistration<> pipeline(
      "sec-lower-scalar-core",
      "Lower trivial and target-resolved Sec scalar semantics to core MLIR",
      [](OpPassManager &manager) {
        manager.addPass(sec::createSecLowerTrivialCorePass());
        manager.addPass(sec::createSecResolveScalarLayoutPass());
        manager.addPass(sec::createSecLowerTrivialCorePass());
      });
  (void)pipeline;
}
