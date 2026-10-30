// Select the native representation of semantic Result and union values whose
// payloads are already resolved scalars, in the final binary track only. A
// value becomes an LLVM literal struct holding the discriminator followed by
// one slot per possible payload position; inactive slots are undefined and
// never read, because every projection sits on its verified guard edge.
// Earlier package pipelines keep the nominal Result/union types.
//
// Rules:
//   - rules/memory/layout.md — §§ 15(1)–15(6), 16(1)–16(4)
//   - rules/mlir/sec_mlir_lowering.md — staged lowering, no hidden fallback
//   - rules/mlir/sec_mlir_dialect.md — v11.41.4 (error-variant patterns)

#include "sec/Conversion/SecToLLVM/Passes.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Dialect/Func/Transforms/FuncConversions.h"
#include "mlir/Dialect/LLVMIR/LLVMDialect.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Transforms/DialectConversion.h"
#include "sec/Dialect/Sec/SecAttributes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"

namespace sec {
#define GEN_PASS_DEF_SECLOWERTAGGEDVALUES
#include "sec/Conversion/SecToLLVM/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {

// The slot type of one resolved scalar payload, or null for a payload this
// slice does not represent (strings, structs, arrays, references, ...).
Type physicalType(Type type);

// The layout of one tagged type: the discriminator type and, per payload
// position, its struct slot.
struct TaggedLayout {
  LLVM::LLVMStructType structType;
  // Result: slot of the Ok and Err payloads; sec.result has no void payload.
  unsigned okSlot = 0, errSlot = 0;
  // Union: slot of each variant's single payload, keyed by variant index,
  // and of each named field, keyed by (variant index, field name).
  DenseMap<uint32_t, unsigned> payloadSlots;
  std::map<std::pair<uint32_t, std::string>, unsigned> fieldSlots;
};

std::optional<TaggedLayout> taggedLayout(Type type) {
  MLIRContext *context = type.getContext();
  TaggedLayout layout;
  SmallVector<Type> elements;
  if (auto result = dyn_cast<sec::ResultType>(type)) {
    // layout.md § 16(1): Err is the set discriminator bit.
    elements.push_back(IntegerType::get(context, 1));
    Type success = physicalType(result.getSuccessType());
    Type error = physicalType(result.getErrorType());
    if (!success || !error)
      return std::nullopt;
    layout.okSlot = 1;
    layout.errSlot = 2;
    elements.append({success, error});
  } else if (auto union_ = dyn_cast<sec::UnionType>(type)) {
    // layout.md § 15(2): the discriminator is the declaration-order index.
    elements.push_back(IntegerType::get(context, 32));
    for (Attribute attribute : union_.getVariants()) {
      auto variant = cast<sec::UnionVariantAttr>(attribute);
      uint32_t index = variant.getIndex();
      if (Type payload = variant.getPayload()) {
        Type slot = physicalType(payload);
        if (!slot)
          return std::nullopt;
        layout.payloadSlots[index] = elements.size();
        elements.push_back(slot);
      }
      for (Attribute fieldAttribute : variant.getFields()) {
        auto field = cast<sec::UnionFieldAttr>(fieldAttribute);
        Type slot = physicalType(field.getType());
        if (!slot)
          return std::nullopt;
        layout.fieldSlots[{index, field.getName().str()}] = elements.size();
        elements.push_back(slot);
      }
    }
  } else {
    return std::nullopt;
  }
  layout.structType = LLVM::LLVMStructType::getLiteral(context, elements);
  return layout;
}

Type physicalType(Type type) {
  if (auto integer = dyn_cast<IntegerType>(type))
    return IntegerType::get(type.getContext(), integer.getWidth());
  if (isa<Float32Type, Float64Type>(type))
    return type;
  if (isa<sec::ResultType, sec::UnionType>(type)) {
    std::optional<TaggedLayout> layout = taggedLayout(type);
    return layout ? Type(layout->structType) : Type();
  }
  return Type();
}

// Bridge a semantic siN/uiN value and its signless slot; the casts pair up
// with the checked-integer bridges and are reconciled before LLVM export.
Value bridge(OpBuilder &builder, Location location, Value value, Type type) {
  if (value.getType() == type)
    return value;
  return builder.create<UnrealizedConversionCastOp>(location, type, value)
      .getResult(0);
}

template <typename Op>
class TaggedPattern : public OpConversionPattern<Op> {
public:
  using OpConversionPattern<Op>::OpConversionPattern;

protected:
  // The layout of a tagged operand or result type, from the unconverted
  // semantic type.
  static std::optional<TaggedLayout> layoutOf(Type type) {
    return taggedLayout(type);
  }

  // A fresh value of the struct with only its discriminator and the active
  // payload slots defined (layout.md § 15(5)).
  static Value construct(ConversionPatternRewriter &rewriter, Location location,
                         const TaggedLayout &layout, Value tag,
                         ArrayRef<std::pair<unsigned, Value>> payloads) {
    Value value = rewriter.create<LLVM::UndefOp>(location, layout.structType);
    value = rewriter.create<LLVM::InsertValueOp>(location, value, tag,
                                                 ArrayRef<int64_t>{0});
    for (auto [slot, payload] : payloads) {
      Type slotType = layout.structType.getBody()[slot];
      value = rewriter.create<LLVM::InsertValueOp>(
          location, value, bridge(rewriter, location, payload, slotType),
          ArrayRef<int64_t>{static_cast<int64_t>(slot)});
    }
    return value;
  }

  // Read one slot and present it with the converted type of the semantic
  // result.
  Value extract(ConversionPatternRewriter &rewriter, Location location,
                const TaggedLayout &layout, Value value, unsigned slot,
                Type semanticType) const {
    Value element = rewriter.create<LLVM::ExtractValueOp>(
        location, value, ArrayRef<int64_t>{static_cast<int64_t>(slot)});
    Type converted = this->getTypeConverter()->convertType(semanticType);
    return bridge(rewriter, location, element, converted);
  }
};

class ResultOkLowering : public TaggedPattern<sec::ResultOkOp> {
public:
  using TaggedPattern::TaggedPattern;
  LogicalResult matchAndRewrite(sec::ResultOkOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    std::optional<TaggedLayout> layout = layoutOf(op.getResult().getType());
    if (!layout)
      return failure();
    Location location = op.getLoc();
    Value tag = rewriter.create<LLVM::ConstantOp>(location, rewriter.getI1Type(),
                                                  rewriter.getBoolAttr(false));
    SmallVector<std::pair<unsigned, Value>> payloads;
    for (Value payload : adaptor.getPayload())
      payloads.push_back({layout->okSlot, payload});
    rewriter.replaceOp(op, construct(rewriter, location, *layout, tag, payloads));
    return success();
  }
};

class ResultErrLowering : public TaggedPattern<sec::ResultErrOp> {
public:
  using TaggedPattern::TaggedPattern;
  LogicalResult matchAndRewrite(sec::ResultErrOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    std::optional<TaggedLayout> layout = layoutOf(op.getResult().getType());
    if (!layout)
      return failure();
    Location location = op.getLoc();
    Value tag = rewriter.create<LLVM::ConstantOp>(location, rewriter.getI1Type(),
                                                  rewriter.getBoolAttr(true));
    SmallVector<std::pair<unsigned, Value>> payloads;
    for (Value payload : adaptor.getPayload())
      payloads.push_back({layout->errSlot, payload});
    rewriter.replaceOp(op, construct(rewriter, location, *layout, tag, payloads));
    return success();
  }
};

class ResultIsErrLowering : public TaggedPattern<sec::ResultIsErrOp> {
public:
  using TaggedPattern::TaggedPattern;
  LogicalResult matchAndRewrite(sec::ResultIsErrOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    rewriter.replaceOpWithNewOp<LLVM::ExtractValueOp>(
        op, adaptor.getValue(), ArrayRef<int64_t>{0});
    return success();
  }
};

template <typename Op, bool Err>
class ResultUnwrapLowering : public TaggedPattern<Op> {
public:
  using TaggedPattern<Op>::TaggedPattern;
  LogicalResult matchAndRewrite(Op op, typename Op::Adaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    std::optional<TaggedLayout> layout = this->layoutOf(op.getValue().getType());
    if (!layout)
      return failure();
    unsigned slot = Err ? layout->errSlot : layout->okSlot;
    rewriter.replaceOp(op, this->extract(rewriter, op.getLoc(), *layout,
                                         adaptor.getValue(), slot,
                                         op.getResult().getType()));
    return success();
  }
};

class UnionConstructLowering : public TaggedPattern<sec::UnionConstructOp> {
public:
  using TaggedPattern::TaggedPattern;
  LogicalResult matchAndRewrite(sec::UnionConstructOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    std::optional<TaggedLayout> layout = layoutOf(op.getResult().getType());
    if (!layout)
      return failure();
    uint32_t variant = op.getVariant();
    SmallVector<std::pair<unsigned, Value>> payloads;
    ArrayAttr fieldNames = op.getFieldNames();
    if (fieldNames.empty()) {
      if (!adaptor.getPayloads().empty()) {
        auto slot = layout->payloadSlots.find(variant);
        if (slot == layout->payloadSlots.end())
          return failure();
        payloads.push_back({slot->second, adaptor.getPayloads().front()});
      }
    } else {
      for (auto [name, payload] : llvm::zip(fieldNames, adaptor.getPayloads())) {
        auto slot = layout->fieldSlots.find(
            {variant, cast<StringAttr>(name).getValue().str()});
        if (slot == layout->fieldSlots.end())
          return failure();
        payloads.push_back({slot->second, payload});
      }
    }
    Location location = op.getLoc();
    Value tag = rewriter.create<LLVM::ConstantOp>(
        location, rewriter.getI32Type(), rewriter.getI32IntegerAttr(variant));
    rewriter.replaceOp(op, construct(rewriter, location, *layout, tag, payloads));
    return success();
  }
};

class UnionIsVariantLowering : public TaggedPattern<sec::UnionIsVariantOp> {
public:
  using TaggedPattern::TaggedPattern;
  LogicalResult matchAndRewrite(sec::UnionIsVariantOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    Location location = op.getLoc();
    Value tag = rewriter.create<LLVM::ExtractValueOp>(location, adaptor.getValue(),
                                                      ArrayRef<int64_t>{0});
    Value expected = rewriter.create<LLVM::ConstantOp>(
        location, rewriter.getI32Type(),
        rewriter.getI32IntegerAttr(op.getVariant()));
    rewriter.replaceOpWithNewOp<LLVM::ICmpOp>(op, LLVM::ICmpPredicate::eq, tag,
                                              expected);
    return success();
  }
};

class UnionUnwrapPayloadLowering
    : public TaggedPattern<sec::UnionUnwrapPayloadOp> {
public:
  using TaggedPattern::TaggedPattern;
  LogicalResult matchAndRewrite(sec::UnionUnwrapPayloadOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    std::optional<TaggedLayout> layout = layoutOf(op.getValue().getType());
    if (!layout)
      return failure();
    auto slot = layout->payloadSlots.find(op.getVariant());
    if (slot == layout->payloadSlots.end())
      return failure();
    rewriter.replaceOp(op, extract(rewriter, op.getLoc(), *layout,
                                   adaptor.getValue(), slot->second,
                                   op.getResult().getType()));
    return success();
  }
};

class UnionUnwrapFieldLowering : public TaggedPattern<sec::UnionUnwrapFieldOp> {
public:
  using TaggedPattern::TaggedPattern;
  LogicalResult matchAndRewrite(sec::UnionUnwrapFieldOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    std::optional<TaggedLayout> layout = layoutOf(op.getValue().getType());
    if (!layout)
      return failure();
    auto slot = layout->fieldSlots.find({op.getVariant(), op.getField().str()});
    if (slot == layout->fieldSlots.end())
      return failure();
    rewriter.replaceOp(op, extract(rewriter, op.getLoc(), *layout,
                                   adaptor.getValue(), slot->second,
                                   op.getResult().getType()));
    return success();
  }
};

// A mutable Result/union binding lives in one stack slot of the selected
// struct, allocated in the entry block so that a declaration inside a loop
// does not grow the stack per iteration.
class TaggedStorageDeclareLowering
    : public OpConversionPattern<sec::StorageDeclareOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult matchAndRewrite(sec::StorageDeclareOp op, OpAdaptor,
                                ConversionPatternRewriter &rewriter) const override {
    auto storage = cast<sec::StorageType>(op.getStorage().getType());
    Type element = getTypeConverter()->convertType(storage.getElementType());
    auto function = op->getParentOfType<func::FuncOp>();
    if (!element || !function)
      return failure();
    OpBuilder::InsertionGuard guard(rewriter);
    rewriter.setInsertionPointToStart(&function.getBody().front());
    Value count = rewriter.create<LLVM::ConstantOp>(
        op.getLoc(), rewriter.getI64Type(), rewriter.getI64IntegerAttr(1));
    rewriter.replaceOpWithNewOp<LLVM::AllocaOp>(
        op, LLVM::LLVMPointerType::get(rewriter.getContext()), element, count);
    return success();
  }
};

template <typename Op>
class TaggedStorageWriteLowering : public OpConversionPattern<Op> {
public:
  using OpConversionPattern<Op>::OpConversionPattern;
  LogicalResult matchAndRewrite(Op op, typename Op::Adaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    rewriter.replaceOpWithNewOp<LLVM::StoreOp>(op, adaptor.getValue(),
                                               adaptor.getStorage());
    return success();
  }
};

class TaggedStorageLoadLowering : public OpConversionPattern<sec::StorageLoadOp> {
public:
  using OpConversionPattern::OpConversionPattern;
  LogicalResult matchAndRewrite(sec::StorageLoadOp op, OpAdaptor adaptor,
                                ConversionPatternRewriter &rewriter) const override {
    Type element = getTypeConverter()->convertType(op.getResult().getType());
    if (!element)
      return failure();
    rewriter.replaceOpWithNewOp<LLVM::LoadOp>(op, element, adaptor.getStorage());
    return success();
  }
};

// Convert the complete CFG, including Result/union merge arguments of a value
// match or try, together with the signature; the upstream function pattern
// converts only the entry block.
class TaggedFunctionLowering : public OpConversionPattern<func::FuncOp> {
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

bool isTaggedOperation(Operation *op) {
  return isa<sec::ResultOkOp, sec::ResultErrOp, sec::ResultIsErrOp,
             sec::ResultUnwrapOkOp, sec::ResultUnwrapErrOp,
             sec::UnionConstructOp, sec::UnionIsVariantOp,
             sec::UnionUnwrapPayloadOp, sec::UnionUnwrapFieldOp>(op);
}

// After checked-integer lowering and before the LLVM finalization gate,
// replace semantic Result/union values with their selected struct
// representation across construction, tests, projections, calls, returns,
// branches and function signatures (layout.md §§ 15–16).
struct LowerTaggedValuesPass
    : sec::impl::SecLowerTaggedValuesBase<LowerTaggedValuesPass> {
  void runOnOperation() override {
    MLIRContext *context = &getContext();
    TypeConverter converter;
    converter.addConversion([](Type type) { return type; });
    converter.addConversion([](sec::ResultType type) -> std::optional<Type> {
      std::optional<TaggedLayout> layout = taggedLayout(type);
      if (!layout)
        return std::nullopt;
      return Type(layout->structType);
    });
    converter.addConversion([](sec::UnionType type) -> std::optional<Type> {
      std::optional<TaggedLayout> layout = taggedLayout(type);
      if (!layout)
        return std::nullopt;
      return Type(layout->structType);
    });
    converter.addConversion([](sec::StorageType type) -> std::optional<Type> {
      if (!isa<sec::ResultType, sec::UnionType>(type.getElementType()))
        return type;
      if (!taggedLayout(type.getElementType()))
        return std::nullopt;
      return Type(LLVM::LLVMPointerType::get(type.getContext()));
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
      return !isTaggedOperation(op) && converter.isLegal(op);
    });
    target.addDynamicallyLegalOp<func::FuncOp>([&](func::FuncOp op) {
      return converter.isSignatureLegal(op.getFunctionType()) &&
             converter.isLegal(&op.getBody());
    });

    RewritePatternSet patterns(context);
    patterns.add<ResultOkLowering, ResultErrLowering, ResultIsErrLowering,
                 ResultUnwrapLowering<sec::ResultUnwrapOkOp, false>,
                 ResultUnwrapLowering<sec::ResultUnwrapErrOp, true>,
                 UnionConstructLowering, UnionIsVariantLowering,
                 UnionUnwrapPayloadLowering, UnionUnwrapFieldLowering,
                 TaggedStorageDeclareLowering,
                 TaggedStorageWriteLowering<sec::StorageInitOp>,
                 TaggedStorageWriteLowering<sec::StorageStoreOp>,
                 TaggedStorageLoadLowering, TaggedFunctionLowering>(converter,
                                                                    context);
    populateCallOpTypeConversionPattern(patterns, converter);
    populateReturnOpTypeConversionPattern(patterns, converter);
    populateBranchOpInterfaceTypeConversionPattern(patterns, converter);
    if (failed(applyPartialConversion(getOperation(), target, std::move(patterns))))
      signalPassFailure();
  }
};
} // namespace
