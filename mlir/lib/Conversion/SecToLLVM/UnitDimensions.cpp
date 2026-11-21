// Native representation of the compiler-known UnitDimension value (MD-062).
//
// A !sec.unit_dimension value becomes the literal struct
//   { ptr descriptor, i64 descriptor-bytes, i64 axis-count }
// whose descriptor is an internal constant byte string holding the canonical
// sorted axis mapping in a collision-free encoding: for every axis
// "<identity-bytes>:<identity><exponent>;". The empty mapping has no
// descriptor (null pointer, zero bytes). Equality compares the lengths and
// then the descriptor bytes, so it is exact structural equality of the two
// canonical mappings whatever object or module materialized them; it is
// never a hash or a pointer-identity comparison (MD-062 § 7(12)-(13)).
// AxisCount reads the count slot; IsDimensionless tests it for zero. No
// allocation, registry or libc routine is used.
//
// Rules:
//   - rules/corrections/applied/md062-unit-metadata-value-types-correction-20261027-20261121.md — §§ 7(5)-(7), 7(12)-(13), 12(8)
//   - rules/mlir/sec_mlir_lowering.md — staged lowering, no hidden fallback

#include "sec/Conversion/SecToLLVM/Passes.h"

#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Dialect/Func/Transforms/FuncConversions.h"
#include "mlir/Dialect/LLVMIR/LLVMDialect.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Transforms/DialectConversion.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/StringMap.h"

namespace sec {
#define GEN_PASS_DEF_SECLOWERUNITDIMENSIONS
#include "sec/Conversion/SecToLLVM/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {

constexpr StringLiteral kEqualHelper = "__sec_unit_dimension_equal";

LLVM::LLVMStructType dimensionStruct(MLIRContext *context) {
  Type i64 = IntegerType::get(context, 64);
  return LLVM::LLVMStructType::getLiteral(
      context, {LLVM::LLVMPointerType::get(context), i64, i64});
}

// The canonical collision-free descriptor bytes of a verified constant.
std::string descriptorBytes(ArrayAttr axes) {
  std::string bytes;
  for (Attribute attribute : axes) {
    auto entry = cast<DictionaryAttr>(attribute);
    StringRef axis = cast<StringAttr>(entry.get("axis")).getValue();
    int64_t exponent = cast<IntegerAttr>(entry.get("exponent")).getInt();
    bytes += std::to_string(axis.size()) + ":" + axis.str() +
             std::to_string(exponent) + ";";
  }
  return bytes;
}

// Module-level state: one descriptor global per distinct canonical mapping
// and one equality helper.
struct DimensionModule {
  ModuleOp module;
  llvm::StringMap<LLVM::GlobalOp> descriptors;
  unsigned next = 0;

  LLVM::GlobalOp descriptor(OpBuilder &builder, Location location,
                            StringRef bytes) {
    auto found = descriptors.find(bytes);
    if (found != descriptors.end())
      return found->second;
    OpBuilder::InsertionGuard guard(builder);
    builder.setInsertionPointToStart(module.getBody());
    MLIRContext *context = module.getContext();
    auto type = LLVM::LLVMArrayType::get(IntegerType::get(context, 8),
                                         bytes.size());
    std::string name = "__sec_unit_dimension." + std::to_string(next++);
    auto global = builder.create<LLVM::GlobalOp>(
        location, type, /*isConstant=*/true, LLVM::Linkage::Internal, name,
        builder.getStringAttr(bytes));
    descriptors[bytes] = global;
    return global;
  }

  // Byte-wise equality of two descriptors of equal length.
  LLVM::LLVMFuncOp equalHelper(OpBuilder &builder, Location location) {
    if (auto existing = module.lookupSymbol<LLVM::LLVMFuncOp>(kEqualHelper))
      return existing;
    OpBuilder::InsertionGuard guard(builder);
    builder.setInsertionPointToEnd(module.getBody());
    MLIRContext *context = module.getContext();
    Type pointer = LLVM::LLVMPointerType::get(context);
    Type i64 = IntegerType::get(context, 64);
    Type i8 = IntegerType::get(context, 8);
    Type i1 = IntegerType::get(context, 1);
    auto function = builder.create<LLVM::LLVMFuncOp>(
        location, kEqualHelper,
        LLVM::LLVMFunctionType::get(i1, {pointer, i64, pointer, i64}),
        LLVM::Linkage::Internal);
    Block *entry = function.addEntryBlock(builder);
    Region &body = function.getBody();
    Block *loop = builder.createBlock(&body, body.end(), {i64}, {location});
    Block *step = builder.createBlock(&body, body.end());
    Block *exit = builder.createBlock(&body, body.end(), {i1}, {location});

    builder.setInsertionPointToStart(entry);
    Value left = entry->getArgument(0), leftBytes = entry->getArgument(1);
    Value right = entry->getArgument(2), rightBytes = entry->getArgument(3);
    Value zero = builder.create<LLVM::ConstantOp>(location, i64, 0);
    Value one = builder.create<LLVM::ConstantOp>(location, i64, 1);
    Value no = builder.create<LLVM::ConstantOp>(location, i1, 0);
    Value yes = builder.create<LLVM::ConstantOp>(location, i1, 1);
    Value sameLength = builder.create<LLVM::ICmpOp>(
        location, LLVM::ICmpPredicate::eq, leftBytes, rightBytes);
    builder.create<LLVM::CondBrOp>(location, sameLength, loop, ValueRange{zero},
                                   exit, ValueRange{no});

    builder.setInsertionPointToStart(loop);
    Value index = loop->getArgument(0);
    Value done = builder.create<LLVM::ICmpOp>(location, LLVM::ICmpPredicate::eq,
                                              index, leftBytes);
    builder.create<LLVM::CondBrOp>(location, done, exit, ValueRange{yes}, step,
                                   ValueRange{});

    builder.setInsertionPointToStart(step);
    Value leftAddress = builder.create<LLVM::GEPOp>(
        location, pointer, i8, left, ValueRange{index});
    Value rightAddress = builder.create<LLVM::GEPOp>(
        location, pointer, i8, right, ValueRange{index});
    Value leftByte = builder.create<LLVM::LoadOp>(location, i8, leftAddress);
    Value rightByte = builder.create<LLVM::LoadOp>(location, i8, rightAddress);
    Value sameByte = builder.create<LLVM::ICmpOp>(
        location, LLVM::ICmpPredicate::eq, leftByte, rightByte);
    Value nextIndex = builder.create<LLVM::AddOp>(location, index, one);
    builder.create<LLVM::CondBrOp>(location, sameByte, loop,
                                   ValueRange{nextIndex}, exit, ValueRange{no});

    builder.setInsertionPointToStart(exit);
    builder.create<LLVM::ReturnOp>(location, exit->getArgument(0));
    return function;
  }
};

class ConstantLowering
    : public OpConversionPattern<sec::UnitDimensionConstantOp> {
public:
  ConstantLowering(const TypeConverter &converter, MLIRContext *context,
                   DimensionModule &state)
      : OpConversionPattern(converter, context), state(state) {}
  LogicalResult
  matchAndRewrite(sec::UnitDimensionConstantOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    MLIRContext *context = op.getContext();
    Location location = op.getLoc();
    Type pointer = LLVM::LLVMPointerType::get(context);
    Type i64 = IntegerType::get(context, 64);
    std::string bytes = descriptorBytes(op.getAxes());
    Value address;
    if (bytes.empty()) {
      address = rewriter.create<LLVM::ZeroOp>(location, pointer);
    } else {
      LLVM::GlobalOp global = state.descriptor(rewriter, location, bytes);
      address = rewriter.create<LLVM::AddressOfOp>(location, global);
    }
    auto type = dimensionStruct(context);
    Value value = rewriter.create<LLVM::UndefOp>(location, type);
    value = rewriter.create<LLVM::InsertValueOp>(location, value, address, 0);
    value = rewriter.create<LLVM::InsertValueOp>(
        location, value,
        rewriter.create<LLVM::ConstantOp>(location, i64, bytes.size()), 1);
    value = rewriter.create<LLVM::InsertValueOp>(
        location, value,
        rewriter.create<LLVM::ConstantOp>(location, i64, op.getAxes().size()), 2);
    rewriter.replaceOp(op, value);
    return success();
  }

private:
  DimensionModule &state;
};

class CompareLowering : public OpConversionPattern<sec::UnitDimensionCompareOp> {
public:
  CompareLowering(const TypeConverter &converter, MLIRContext *context,
                  DimensionModule &state)
      : OpConversionPattern(converter, context), state(state) {}
  LogicalResult
  matchAndRewrite(sec::UnitDimensionCompareOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    Location location = op.getLoc();
    LLVM::LLVMFuncOp helper = state.equalHelper(rewriter, location);
    auto field = [&](Value value, int64_t index) -> Value {
      return rewriter.create<LLVM::ExtractValueOp>(location, value, index);
    };
    Value left = adaptor.getLeft(), right = adaptor.getRight();
    Value equal =
        rewriter
            .create<LLVM::CallOp>(location, helper,
                                  ValueRange{field(left, 0), field(left, 1),
                                             field(right, 0), field(right, 1)})
            .getResult();
    if (op.getPredicate() == "ne") {
      Value yes = rewriter.create<LLVM::ConstantOp>(
          location, IntegerType::get(op.getContext(), 1), 1);
      equal = rewriter.create<LLVM::XOrOp>(location, equal, yes);
    }
    rewriter.replaceOp(op, equal);
    return success();
  }

private:
  DimensionModule &state;
};

// The count slot fits the plan's uint; a narrower uint is the truncation of
// an already bounded small count.
class AxisCountLowering
    : public OpConversionPattern<sec::UnitDimensionAxisCountOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult
  matchAndRewrite(sec::UnitDimensionAxisCountOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    Location location = op.getLoc();
    auto result = dyn_cast<IntegerType>(op.getResult().getType());
    if (!result)
      return rewriter.notifyMatchFailure(op, "AxisCount needs a resolved uint");
    Value count =
        rewriter.create<LLVM::ExtractValueOp>(location, adaptor.getValue(), 2);
    auto signless = IntegerType::get(op.getContext(), result.getWidth());
    if (result.getWidth() < 64)
      count = rewriter.create<LLVM::TruncOp>(location, signless, count);
    if (!result.isSignless())
      count = rewriter.create<UnrealizedConversionCastOp>(location, result, count)
                  .getResult(0);
    rewriter.replaceOp(op, count);
    return success();
  }
};

class IsDimensionlessLowering
    : public OpConversionPattern<sec::UnitDimensionIsDimensionlessOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult
  matchAndRewrite(sec::UnitDimensionIsDimensionlessOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    Location location = op.getLoc();
    Value count =
        rewriter.create<LLVM::ExtractValueOp>(location, adaptor.getValue(), 2);
    Value zero = rewriter.create<LLVM::ConstantOp>(
        location, IntegerType::get(op.getContext(), 64), 0);
    rewriter.replaceOpWithNewOp<LLVM::ICmpOp>(op, LLVM::ICmpPredicate::eq,
                                              count, zero);
    return success();
  }
};

// Convert the complete CFG together with the signature, like the tagged
// value lowering; the upstream function pattern converts only the entry.
class DimensionFunctionLowering : public OpConversionPattern<func::FuncOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult
  matchAndRewrite(func::FuncOp op, OpAdaptor,
                  ConversionPatternRewriter &rewriter) const override {
    TypeConverter::SignatureConversion signature(op.getNumArguments());
    SmallVector<Type> results;
    if (failed(getTypeConverter()->convertSignatureArgs(op.getArgumentTypes(),
                                                        signature)) ||
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

bool isDimensionOperation(Operation *op) {
  return isa<sec::UnitDimensionConstantOp, sec::UnitDimensionCompareOp,
             sec::UnitDimensionAxisCountOp,
             sec::UnitDimensionIsDimensionlessOp>(op);
}

// A dimension may flow only through its own operations, calls, returns,
// branches and signatures here; storage, aggregates and other consumers wait
// for their physical representation and are rejected, never erased.
bool isDimensionCarrier(Operation *op) {
  return isDimensionOperation(op) ||
         isa<func::FuncOp, func::CallOp, func::ReturnOp, cf::BranchOp,
             cf::CondBranchOp>(op);
}

bool mentionsDimension(Type type) {
  bool found = false;
  type.walk([&](Type nested) {
    if (isa<sec::UnitDimensionType>(nested))
      found = true;
  });
  return found;
}

struct LowerUnitDimensionsPass
    : sec::impl::SecLowerUnitDimensionsBase<LowerUnitDimensionsPass> {
  void runOnOperation() override {
    ModuleOp module = getOperation();
    MLIRContext *context = &getContext();
    bool rejected = false;
    module.walk([&](Operation *op) {
      if (isDimensionCarrier(op))
        return;
      auto mentions = [](TypeRange types) {
        return llvm::any_of(types, mentionsDimension);
      };
      if (mentions(op->getOperandTypes()) || mentions(op->getResultTypes())) {
        op->emitError() << "[unsupported-lowering] the Sec MLIR -> LLVM track "
                           "does not lower '"
                        << op->getName() << "' over UnitDimension values yet";
        rejected = true;
      }
    });
    if (rejected)
      return signalPassFailure();

    DimensionModule state{module};
    TypeConverter converter;
    converter.addConversion([](Type type) { return type; });
    converter.addConversion([](sec::UnitDimensionType type) -> Type {
      return dimensionStruct(type.getContext());
    });
    auto materialize = [](OpBuilder &builder, Type type, ValueRange inputs,
                          Location location) -> Value {
      if (inputs.size() != 1)
        return Value();
      return builder.create<UnrealizedConversionCastOp>(location, type, inputs)
          .getResult(0);
    };
    converter.addSourceMaterialization(materialize);
    converter.addTargetMaterialization(materialize);

    ConversionTarget target(*context);
    target.addLegalDialect<LLVM::LLVMDialect>();
    target.addLegalOp<UnrealizedConversionCastOp>();
    target.markUnknownOpDynamicallyLegal([&](Operation *op) {
      return !isDimensionOperation(op) && converter.isLegal(op);
    });
    target.addDynamicallyLegalOp<func::FuncOp>([&](func::FuncOp op) {
      return converter.isSignatureLegal(op.getFunctionType()) &&
             converter.isLegal(&op.getBody());
    });

    RewritePatternSet patterns(context);
    patterns.add<ConstantLowering, CompareLowering>(converter, context, state);
    patterns.add<AxisCountLowering, IsDimensionlessLowering,
                 DimensionFunctionLowering>(converter, context);
    populateCallOpTypeConversionPattern(patterns, converter);
    populateReturnOpTypeConversionPattern(patterns, converter);
    populateBranchOpInterfaceTypeConversionPattern(patterns, converter);
    if (failed(applyPartialConversion(module, target, std::move(patterns))))
      signalPassFailure();
  }
};

} // namespace
