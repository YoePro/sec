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
#include "llvm/ADT/APFloat.h"
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

// namedCarrier is the type a named type's base chain ends in; a named integer
// lowers as its carrier, which erases the nominal identity (semantic_ir.md
// § 7(5)).
Type namedCarrier(Type type) {
  while (auto named = dyn_cast<sec::NamedType>(type))
    type = named.getBase();
  return type;
}

bool isPlainSemanticInteger(Type type) {
  auto integer = dyn_cast<IntegerType>(namedCarrier(type));
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

// A bit extraction that proves a register.from_bits stays with the detached
// register family, which keeps its high-level form (v11.41.2 amendment).
bool feedsRegisterConversion(Operation *operation) {
  return llvm::any_of(operation->getUsers(), [](Operation *user) {
    return isa<sec::RegisterFromBitsOp>(user);
  });
}

bool isCheckedIntegerOperation(Operation *operation) {
  return isa<sec::IntUnaryPlusOp, sec::IntNegCheckedOp,
             sec::IntBinaryCheckedOp, sec::IntBitNotOp, sec::IntBitwiseOp,
             sec::IntShiftCheckedOp, sec::IntCmpOp, sec::NamedConvertOp>(operation) ||
         (isa<sec::IntBitExtractOp>(operation) &&
          !feedsRegisterConversion(operation));
}

IntegerType signlessType(MLIRContext *context, Type type) {
  auto integer = cast<IntegerType>(namedCarrier(type));
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

// isSecOriginIntegerStorage recognizes scalar slots with source provenance,
// including named integer carriers, for physical load/store normalization.
// Rules: rules/compiler/semantic_ir.md, section 7(3)-(5);
// rules/corrections/applied/md087-p5-p6-p8-reference-boundaries-correction-20261021.md,
// section 5(3)-(4).
bool isSecOriginIntegerStorage(Value value) {
  auto alloca = value.getDefiningOp<memref::AllocaOp>();
  if (!alloca)
    return false;
  auto type = alloca.getType();
  return type.getRank() == 0 &&
         isa<IntegerType>(namedCarrier(type.getElementType())) &&
         alloca->hasAttr("sec.storage_id") &&
         alloca->getAttrOfType<StringAttr>("sec.scalar_kind") != nullptr;
}

// normalizeSecIntegerStorage erases nominal integer identity only at physical
// storage lowering, retaining carrier width/signedness for provenance checks
// and semantic load bridges. Named chains obey the same rule as plain carriers.
// Rules: rules/compiler/semantic_ir.md, section 7(3)-(5);
// rules/types/types.md, section 22.1(1)-(6);
// rules/corrections/applied/md087-p5-p6-p8-reference-boundaries-correction-20261021.md,
// sections 4(6), 5(3)-(4).
LogicalResult normalizeSecIntegerStorage(ModuleOp module) {
  WalkResult result = module.walk([&](memref::AllocaOp alloca) {
    auto oldType = alloca.getType();
    if (oldType.getRank() != 0 ||
        !isPlainSemanticInteger(oldType.getElementType()) ||
        !alloca->hasAttr("sec.storage_id"))
      return WalkResult::advance();
    auto elementType = cast<IntegerType>(namedCarrier(oldType.getElementType()));
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
    // A load keeps its semantic signed/unsigned type for its users through a
    // materialization cast, so checked integer lowering still sees the source
    // signedness; the conversion folds the cast away.
    for (Operation *user :
         llvm::to_vector(alloca.getResult().getUsers())) {
      auto load = dyn_cast<memref::LoadOp>(user);
      if (!load)
        continue;
      Type semantic = load.getResult().getType();
      load.getResult().setType(converted.getElementType());
      OpBuilder builder(load->getContext());
      builder.setInsertionPointAfter(load);
      auto cast = builder.create<UnrealizedConversionCastOp>(
          load.getLoc(), semantic, load.getResult());
      load.getResult().replaceAllUsesExcept(cast.getResult(0), cast);
    }
    return WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}

// foldRoundTripCasts removes the signless -> semantic -> signless cast pairs
// that keep a normalized storage load's source signedness visible during
// conversion (normalizeSecIntegerStorage); no cast survives the pass.
void foldRoundTripCasts(ModuleOp module) {
  SmallVector<UnrealizedConversionCastOp> casts;
  module.walk([&](UnrealizedConversionCastOp cast) { casts.push_back(cast); });
  for (UnrealizedConversionCastOp cast : casts) {
    if (cast->getNumOperands() != 1 || cast->getNumResults() != 1)
      continue;
    Value source = cast.getOperand(0);
    for (Operation *user : llvm::to_vector(cast->getUsers())) {
      auto inverse = dyn_cast<UnrealizedConversionCastOp>(user);
      if (inverse && inverse->getNumOperands() == 1 &&
          inverse->getNumResults() == 1 &&
          inverse.getResult(0).getType() == source.getType()) {
        inverse.getResult(0).replaceAllUsesWith(source);
        inverse.erase();
      }
    }
  }
  for (UnrealizedConversionCastOp cast : casts)
    if (cast->use_empty())
      cast.erase();
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

// namedIntegerStorage selects scalar local storage whose named carrier has
// already been resolved by the plan. Reference-dependent storage stays at P15.
// Rules: rules/compiler/semantic_ir.md, section 7(3)-(5);
// rules/mlir/packages/sec-mlir-dialect_package15.md, section 67.
Value namedIntegerStorage(Operation *op) {
  if (!isa<sec::StorageDeclareOp, sec::StorageInitOp, sec::StorageLoadOp,
           sec::StorageStoreOp>(op))
    return {};
  Value value = isa<sec::StorageDeclareOp>(op) ? op->getResult(0)
                                              : op->getOperand(0);
  auto storage = dyn_cast<sec::StorageType>(value.getType());
  return storage && isa<sec::NamedType>(storage.getElementType()) &&
                 isPlainSemanticInteger(storage.getElementType())
             ? value
             : Value{};
}

// NamedIntegerStorageLowering materializes named scalar locals as carrier
// memrefs together with their accesses. Semantic named values are converted
// by the same P8 type converter, preserving checked arithmetic signedness.
// Rules: rules/compiler/semantic_ir.md, section 7(3)-(5);
// rules/types/types.md, section 22.1(1)-(6);
// rules/corrections/applied/md087-p5-p6-p8-reference-boundaries-correction-20261021.md,
// sections 4(6), 5(3)-(4).
class NamedIntegerStorageLowering final : public ConversionPattern {
public:
  NamedIntegerStorageLowering(TypeConverter &converter, MLIRContext *context)
      : ConversionPattern(converter, MatchAnyOpTypeTag(), 1, context) {}

  LogicalResult matchAndRewrite(Operation *op, ArrayRef<Value> operands,
                                ConversionPatternRewriter &rewriter) const override {
    Value storage = namedIntegerStorage(op);
    if (!storage || sec::isReferenceDependentStorage(storage))
      return failure();
    if (isa<sec::StorageDeclareOp>(op)) {
      Type element = cast<sec::StorageType>(storage.getType()).getElementType();
      if (failed(verifyScalarKind(op, cast<IntegerType>(namedCarrier(element)),
                                 op->getAttrOfType<StringAttr>("sec.scalar_kind"),
                                 "Sec-origin named integer storage")))
        return failure();
      auto type = cast<MemRefType>(
          getTypeConverter()->convertType(storage.getType()));
      auto alloca = memref::AllocaOp::create(rewriter, op->getLoc(), type);
      for (StringRef name : {"sec.storage_id", "sec.source_name",
                             "sec.storage_class", "sec.mutable",
                             "sec.scalar_kind"})
        if (Attribute attribute = op->getAttr(name))
          alloca->setAttr(name, attribute);
      rewriter.replaceOp(op, alloca.getResult());
    } else if (isa<sec::StorageLoadOp>(op)) {
      rewriter.replaceOpWithNewOp<memref::LoadOp>(op, operands[0], ValueRange{});
    } else {
      memref::StoreOp::create(rewriter, op->getLoc(), operands[1], operands[0],
                             ValueRange{});
      rewriter.eraseOp(op);
    }
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

// An exact integer conversion is a plain width change: the frontend proved
// the value fits, so widening extends by the source signedness, narrowing
// truncates, and equal widths reuse the bits (package20.md § 44).
class NumericConvertExactLowering final
    : public OpConversionPattern<sec::NumericConvertExactOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::NumericConvertExactOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    // A proven-exact P20 float conversion (package20.md §§ 44–55, 70): the
    // frontend proved the value representable, so the single standard
    // conversion is exact and no guard is needed.
    Type sourceSemantic = op.getValue().getType();
    Type targetSemantic = op.getResult().getType();
    if (isa<FloatType>(sourceSemantic) || isa<FloatType>(targetSemantic)) {
      Value value = adaptor.getValue();
      Type target = getTypeConverter()->convertType(targetSemantic);
      Location location = op.getLoc();
      auto sourceInteger = dyn_cast<IntegerType>(sourceSemantic);
      auto targetInteger = dyn_cast<IntegerType>(targetSemantic);
      auto sourceFloat = dyn_cast<FloatType>(sourceSemantic);
      auto targetFloat = dyn_cast_or_null<FloatType>(target);
      Value result;
      if (sourceFloat && targetFloat) {
        if (targetFloat.getWidth() > sourceFloat.getWidth())
          result = arith::ExtFOp::create(rewriter, location, targetFloat, value);
        else if (targetFloat.getWidth() < sourceFloat.getWidth())
          result = arith::TruncFOp::create(rewriter, location, targetFloat, value);
        else
          result = value;
      } else if (sourceInteger && targetFloat &&
                 (sourceInteger.isSigned() || sourceInteger.isUnsigned())) {
        result = sourceInteger.isSigned()
                     ? arith::SIToFPOp::create(rewriter, location, targetFloat, value).getResult()
                     : arith::UIToFPOp::create(rewriter, location, targetFloat, value).getResult();
      } else if (sourceFloat && targetInteger && target &&
                 (targetInteger.isSigned() || targetInteger.isUnsigned())) {
        result = targetInteger.isSigned()
                     ? arith::FPToSIOp::create(rewriter, location, target, value).getResult()
                     : arith::FPToUIOp::create(rewriter, location, target, value).getResult();
      } else {
        return op.emitOpError(
            "requires resolved binary32/binary64 floats and signed or unsigned "
            "builtin integers");
      }
      rewriter.replaceOp(op, result);
      return success();
    }
    auto sourceType = dyn_cast<IntegerType>(op.getValue().getType());
    auto value = adaptor.getValue();
    auto from = dyn_cast<IntegerType>(value.getType());
    auto to = dyn_cast_or_null<IntegerType>(
        getTypeConverter()->convertType(op.getResult().getType()));
    if (!sourceType || !from || !to ||
        (!sourceType.isSigned() && !sourceType.isUnsigned()))
      return op.emitOpError(
          "requires resolved signed or unsigned builtin integer types");
    Value result = value;
    if (to.getWidth() > from.getWidth())
      result = sourceType.isSigned()
                   ? arith::ExtSIOp::create(rewriter, op.getLoc(), to, value)
                         .getResult()
                   : arith::ExtUIOp::create(rewriter, op.getLoc(), to, value)
                         .getResult();
    else if (to.getWidth() < from.getWidth())
      result =
          arith::TruncIOp::create(rewriter, op.getLoc(), to, value).getResult();
    rewriter.replaceOp(op, result);
    return success();
  }
};

// A checked integer conversion compares the source, extended one bit beyond
// both widths, with the target's domain; the candidate is the same width
// change as the exact form and is consumed only on the success edge
// (package20.md §§ 48–54, 71).
class NumericConvertCheckedLowering final
    : public OpConversionPattern<sec::NumericConvertCheckedOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(sec::NumericConvertCheckedOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    if (op.getFailures())
      return lowerFloatForm(op, adaptor, rewriter);
    auto sourceType = dyn_cast<IntegerType>(op.getValue().getType());
    auto targetType = dyn_cast<IntegerType>(op.getCandidate().getType());
    Value value = adaptor.getValue();
    auto from = dyn_cast<IntegerType>(value.getType());
    auto to = dyn_cast_or_null<IntegerType>(
        getTypeConverter()->convertType(op.getCandidate().getType()));
    if (!sourceType || !targetType || !from || !to ||
        (!sourceType.isSigned() && !sourceType.isUnsigned()) ||
        (!targetType.isSigned() && !targetType.isUnsigned()))
      return op.emitOpError(
          "requires resolved signed or unsigned builtin integer types");
    Location location = op.getLoc();
    unsigned width = std::max(from.getWidth(), to.getWidth()) + 1;
    auto wide = IntegerType::get(rewriter.getContext(), width);
    Value extended = extendInteger(rewriter, location, value, wide,
                                   sourceType.isSigned());
    unsigned bits = to.getWidth();
    APInt minimum = targetType.isSigned()
                        ? APInt::getSignedMinValue(bits).sext(width)
                        : APInt::getZero(width);
    APInt maximum = targetType.isSigned()
                        ? APInt::getSignedMaxValue(bits).sext(width)
                        : APInt::getMaxValue(bits).zext(width);
    Value below = arith::CmpIOp::create(
        rewriter, location, arith::CmpIPredicate::slt, extended,
        integerConstant(rewriter, location, wide, minimum));
    Value above = arith::CmpIOp::create(
        rewriter, location, arith::CmpIPredicate::sgt, extended,
        integerConstant(rewriter, location, wide, maximum));
    if (op.getFailure() == "InvalidScalar") {
      // A rune target (resolved to its carrier): only the Unicode scalar
      // values 0..0x10FFFF without the surrogates 0xD800..0xDFFF.
      below = arith::CmpIOp::create(
          rewriter, location, arith::CmpIPredicate::slt, extended,
          integerConstant(rewriter, location, wide, APInt::getZero(width)));
      above = arith::CmpIOp::create(
          rewriter, location, arith::CmpIPredicate::sgt, extended,
          integerConstant(rewriter, location, wide, APInt(width, 0x10FFFF)));
      Value surrogate = arith::AndIOp::create(
          rewriter, location,
          arith::CmpIOp::create(rewriter, location, arith::CmpIPredicate::sge, extended,
                                integerConstant(rewriter, location, wide, APInt(width, 0xD800))),
          arith::CmpIOp::create(rewriter, location, arith::CmpIPredicate::sle, extended,
                                integerConstant(rewriter, location, wide, APInt(width, 0xDFFF))));
      above = combineOr(rewriter, location, above, surrogate);
    }
    Value failed = combineOr(rewriter, location, below, above);
    Value candidate = extendInteger(rewriter, location, value, to,
                                    sourceType.isSigned());
    rewriter.replaceOp(op, ValueRange{candidate, failed});
    return success();
  }

private:
  // The P20 binary-float form as a total guard sequence
  // (sec-mlir-dialect_package20.md §§ 41, 47-55, 79): every reason is
  // computed from the source alone, in priority order and mutually
  // exclusive, and the final conversion only ever sees a sanitized operand,
  // so no invalid fptosi/fptoui executes.
  LogicalResult lowerFloatForm(sec::NumericConvertCheckedOp op,
                               OpAdaptor adaptor,
                               ConversionPatternRewriter &rewriter) const {
    Location loc = op.getLoc();
    Type sourceSemantic = op.getValue().getType();
    Type targetSemantic = op.getCandidate().getType();
    Value value = adaptor.getValue();
    Type target = getTypeConverter()->convertType(targetSemantic);
    auto sourceFloat = dyn_cast<FloatType>(sourceSemantic);
    auto targetFloat = dyn_cast<FloatType>(targetSemantic);
    auto sourceInteger = dyn_cast<IntegerType>(sourceSemantic);
    auto targetInteger = dyn_cast<IntegerType>(targetSemantic);
    auto i1 = rewriter.getI1Type();
    auto falseValue = [&]() -> Value {
      return arith::ConstantOp::create(rewriter, loc, i1, rewriter.getBoolAttr(false));
    };
    auto fconst = [&](FloatType type, const APFloat &number) -> Value {
      return arith::ConstantOp::create(rewriter, loc, type,
                                       rewriter.getFloatAttr(type, number));
    };
    auto cmpf = [&](arith::CmpFPredicate predicate, Value left, Value right) -> Value {
      return arith::CmpFOp::create(rewriter, loc, predicate, left, right);
    };
    auto andNot = [&](Value value, Value excluded) -> Value {
      Value notExcluded = arith::XOrIOp::create(
          rewriter, loc, excluded,
          arith::ConstantOp::create(rewriter, loc, i1, rewriter.getBoolAttr(true)));
      return arith::AndIOp::create(rewriter, loc, value, notExcluded);
    };
    auto isInfinite = [&](Value number) -> Value {
      auto type = cast<FloatType>(number.getType());
      const llvm::fltSemantics &semantics = type.getFloatSemantics();
      return combineOr(rewriter, loc,
                       cmpf(arith::CmpFPredicate::OEQ, number,
                            fconst(type, APFloat::getInf(semantics))),
                       cmpf(arith::CmpFPredicate::OEQ, number,
                            fconst(type, APFloat::getInf(semantics, true))));
    };
    // 2^exponent in a binary format; beyond its range it is infinity, which
    // makes the bound vacuous for every finite value (§ 53).
    auto power = [&](FloatType type, int exponent, bool negative) -> Value {
      APFloat number = scalbn(APFloat(type.getFloatSemantics(), 1), exponent,
                              APFloat::rmNearestTiesToEven);
      if (negative)
        number.changeSign();
      return fconst(type, number);
    };
    Value nonFinite = falseValue(), outOfRange = falseValue(),
          precisionLoss = falseValue(), candidate;
    if (sourceFloat && targetFloat) {
      // Narrowing (§§ 47-48): NaN and infinities succeed unchanged.
      if (targetFloat.getWidth() >= sourceFloat.getWidth())
        return op.emitOpError("checked float conversion must narrow");
      candidate = arith::TruncFOp::create(rewriter, loc, targetFloat, value);
      Value finite = andNot(cmpf(arith::CmpFPredicate::ORD, value, value),
                            isInfinite(value));
      outOfRange = arith::AndIOp::create(rewriter, loc, finite, isInfinite(candidate));
      Value widened = arith::ExtFOp::create(rewriter, loc, sourceFloat, candidate);
      precisionLoss = andNot(
          arith::AndIOp::create(rewriter, loc, finite,
                                cmpf(arith::CmpFPredicate::ONE, widened, value)),
          outOfRange);
    } else if (sourceInteger && targetFloat &&
               (sourceInteger.isSigned() || sourceInteger.isUnsigned())) {
      // Integer to float (§§ 49-50): exact iff the rounded value converts
      // back to the same integer; one extra bit holds every rounded result.
      bool isSigned = sourceInteger.isSigned();
      candidate = isSigned
                      ? arith::SIToFPOp::create(rewriter, loc, targetFloat, value).getResult()
                      : arith::UIToFPOp::create(rewriter, loc, targetFloat, value).getResult();
      outOfRange = isInfinite(candidate);
      auto wide = IntegerType::get(rewriter.getContext(), sourceInteger.getWidth() + 1);
      Value safe = arith::SelectOp::create(
          rewriter, loc, outOfRange,
          fconst(targetFloat, APFloat::getZero(targetFloat.getFloatSemantics())),
          candidate);
      Value back = isSigned
                       ? arith::FPToSIOp::create(rewriter, loc, wide, safe).getResult()
                       : arith::FPToUIOp::create(rewriter, loc, wide, safe).getResult();
      Value original = extendInteger(rewriter, loc, value, wide, isSigned);
      precisionLoss = andNot(arith::CmpIOp::create(rewriter, loc, arith::CmpIPredicate::ne,
                                                   back, original),
                             outOfRange);
    } else if (sourceFloat && targetInteger && target &&
               (targetInteger.isSigned() || targetInteger.isUnsigned())) {
      // Float to integer (§§ 51-54): finite, then exact power-of-two range,
      // then integral; the cast sees only a value that passed all three.
      bool isSigned = targetInteger.isSigned();
      unsigned bits = targetInteger.getWidth();
      nonFinite = combineOr(rewriter, loc,
                            cmpf(arith::CmpFPredicate::UNO, value, value),
                            isInfinite(value));
      Value lower = isSigned ? power(sourceFloat, bits - 1, true)
                             : fconst(sourceFloat, APFloat::getZero(sourceFloat.getFloatSemantics()));
      Value upper = power(sourceFloat, isSigned ? bits - 1 : bits, false);
      outOfRange = andNot(combineOr(rewriter, loc,
                                    cmpf(arith::CmpFPredicate::OLT, value, lower),
                                    cmpf(arith::CmpFPredicate::OGE, value, upper)),
                          nonFinite);
      // Only |x| < 2^p can have a fraction; such a value fits i64 exactly.
      int precision = sourceFloat.getWidth() == 32 ? 24 : 53;
      Value small = arith::AndIOp::create(
          rewriter, loc,
          cmpf(arith::CmpFPredicate::OLT, value, power(sourceFloat, precision, false)),
          cmpf(arith::CmpFPredicate::OGT, value, power(sourceFloat, precision, true)));
      Value zero = fconst(sourceFloat, APFloat::getZero(sourceFloat.getFloatSemantics()));
      Value probe = arith::SelectOp::create(rewriter, loc, small, value, zero);
      Value truncated = arith::SIToFPOp::create(
          rewriter, loc, sourceFloat,
          arith::FPToSIOp::create(rewriter, loc, rewriter.getI64Type(), probe));
      Value fractional = arith::AndIOp::create(
          rewriter, loc, small, cmpf(arith::CmpFPredicate::ONE, truncated, value));
      precisionLoss = andNot(andNot(fractional, nonFinite), outOfRange);
      Value invalid = combineOr(rewriter, loc,
                                combineOr(rewriter, loc, nonFinite, outOfRange),
                                precisionLoss);
      Value safe = arith::SelectOp::create(rewriter, loc, invalid, zero, value);
      candidate = isSigned
                      ? arith::FPToSIOp::create(rewriter, loc, target, safe).getResult()
                      : arith::FPToUIOp::create(rewriter, loc, target, safe).getResult();
    } else {
      return op.emitOpError(
          "requires resolved binary32/binary64 floats and signed or unsigned "
          "builtin integers");
    }
    SmallVector<Value> results{candidate, falseValue()};
    Value failed = results[1];
    for (Attribute attribute : *op.getFailures()) {
      StringRef failure = cast<StringAttr>(attribute).getValue();
      Value reason = failure == "NonFinite"    ? nonFinite
                     : failure == "OutOfRange" ? outOfRange
                                               : precisionLoss;
      results.push_back(reason);
      failed = combineOr(rewriter, loc, failed, reason);
    }
    results[1] = failed;
    rewriter.replaceOp(op, results);
    return success();
  }
};

// Static storage access keeps its symbol and attributes while its value type
// becomes the signless integer of the resolved representation.
class StaticAccessTypeLowering final : public ConversionPattern {
public:
  StaticAccessTypeLowering(TypeConverter &converter, MLIRContext *context,
                           StringRef name)
      : ConversionPattern(converter, name, 1, context) {}
  LogicalResult
  matchAndRewrite(Operation *op, ArrayRef<Value> operands,
                  ConversionPatternRewriter &rewriter) const override {
    SmallVector<Type> results;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(), results)))
      return failure();
    OperationState state(op->getLoc(), op->getName(), operands, results,
                         op->getAttrs());
    rewriter.replaceOp(op, rewriter.create(state)->getResults());
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
    // A named float conversion changes only nominal identity.
    if (isa<sec::NamedConvertOp>(operation) &&
        isa<FloatType>(namedCarrier(operation->getOperand(0).getType()))) {
      rewriter.replaceOp(operation, operands[0]);
      return success();
    }
    // A named integer applies its carrier's semantics (semantic_ir.md § 7(4)).
    auto originalValueType = dyn_cast<IntegerType>(namedCarrier(operation->getOperand(0).getType()));
    if (!originalValueType ||
        (!originalValueType.isSigned() && !originalValueType.isUnsigned()))
      return operation->emitOpError(
          "requires resolved signed or unsigned builtin integer operands");
    auto type = cast<IntegerType>(operands[0].getType());
    unsigned width = type.getWidth();
    bool signedValue = originalValueType.isSigned();

    // The carrier is unchanged; erasing the identity leaves the operand.
    if (isa<sec::IntUnaryPlusOp, sec::NamedConvertOp>(operation)) {
      rewriter.replaceOp(operation, operands[0]);
      return success();
    }
    // v11.41.2 amendment: x mod 2^N as the smallest standard unsigned width.
    if (auto extract = dyn_cast<sec::IntBitExtractOp>(operation)) {
      auto resultType = dyn_cast_or_null<IntegerType>(
          getTypeConverter()->convertType(extract.getResult().getType()));
      if (!resultType)
        return failure();
      unsigned carrier = resultType.getWidth();
      unsigned bits = extract.getWidth();
      Value value = operands[0];
      if (carrier < width)
        value = arith::TruncIOp::create(rewriter, location, resultType, value)
                    .getResult();
      if (bits < carrier)
        value = arith::AndIOp::create(
                    rewriter, location, value,
                    integerConstant(rewriter, location, resultType,
                                    APInt::getLowBitsSet(carrier, bits)))
                    .getResult();
      rewriter.replaceOp(operation, value);
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

    // Places and references coexist with this lowering (MD-087): the type
    // converter rewrites only top-level semantic integers, so it never
    // recurses into !sec.place / !sec.ref / !sec.ref_mut referents, and their
    // operations stay high-level with every fact. Where an SSA value crosses
    // between a lowered signless computation and such a Sec operation, the
    // conversion materializes an exact-width bridge (verified below).
    // Rules: rules/mlir/sec_mlir_dialect.md — v11.33 amendment (MD-087);
    // rules/mlir/sec_mlir_lowering.md — v11.20 amendment.
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
    // Nominal identity is erased here, in physical lowering (semantic_ir.md
    // § 7(5)): a named integer becomes its carrier's signless integer.
    typeConverter.addConversion([&](sec::NamedType type) -> std::optional<Type> {
      // A named binary float is its resolved f32/f64 carrier.
      if (auto floating = dyn_cast<FloatType>(namedCarrier(type)))
        return Type(floating);
      auto integer = dyn_cast<IntegerType>(namedCarrier(type));
      if (!integer || (!integer.isSigned() && !integer.isUnsigned()))
        return std::nullopt;
      return IntegerType::get(context, integer.getWidth());
    });
    // Named local storage becomes physical in the same pass as its values;
    // P5 cannot place semantic !sec.named elements in builtin memrefs.
    // Rules: rules/compiler/semantic_ir.md, section 7(5).
    typeConverter.addConversion([&](sec::StorageType type) -> std::optional<Type> {
      if (!isa<sec::NamedType>(type.getElementType()) ||
          !isPlainSemanticInteger(type.getElementType()))
        return type;
      return MemRefType::get({}, signlessType(context, type.getElementType()));
    });
    // MD-087 § 5(3): the only bridge is a temporary same-width
    // builtin.unrealized_conversion_cast between a semantic siN/uiN and the
    // signless iN; it carries no conversion semantics and is reconciled or
    // rejected before any physical lowering (never arith.bitcast on signed
    // integer types, which is not legal Arith).
    auto bridge = [](OpBuilder &builder, Type resultType, ValueRange inputs,
                     Location location) -> Value {
      if (inputs.size() != 1)
        return Value();
      // A named float and its f32/f64 carrier bridge the same way (P22).
      Type fromFloat = namedCarrier(inputs.front().getType());
      if (isa<FloatType>(fromFloat) && fromFloat == namedCarrier(resultType))
        return builder
            .create<UnrealizedConversionCastOp>(location, resultType, inputs)
            .getResult(0);
      auto from = dyn_cast<IntegerType>(namedCarrier(inputs.front().getType()));
      auto to = dyn_cast<IntegerType>(namedCarrier(resultType));
      if (!from || !to || from.getWidth() != to.getWidth())
        return Value();
      return builder
          .create<UnrealizedConversionCastOp>(location, resultType, inputs)
          .getResult(0);
    };
    typeConverter.addSourceMaterialization(bridge);
    typeConverter.addTargetMaterialization(bridge);

    ConversionTarget target(*context);
    target.addLegalDialect<BuiltinDialect, arith::ArithDialect,
                           func::FuncDialect, cf::ControlFlowDialect,
                           memref::MemRefDialect>();
    target.addLegalDialect<sec::SecDialect>();
    target.addDynamicallyLegalOp<sec::StorageDeclareOp, sec::StorageInitOp,
                                 sec::StorageLoadOp, sec::StorageStoreOp>(
        [](Operation *op) {
          Value storage = namedIntegerStorage(op);
          return !storage || sec::isReferenceDependentStorage(storage);
        });
    target.addIllegalOp<sec::NumericConvertExactOp,
                        sec::NumericConvertCheckedOp>();
    target.addDynamicallyLegalOp<sec::StaticLoadOp, sec::StaticStoreOp>(
        [&](Operation *op) { return typeConverter.isLegal(op); });
    target.addIllegalOp<sec::IntUnaryPlusOp, sec::IntNegCheckedOp,
                        sec::IntBinaryCheckedOp, sec::IntBitNotOp,
                        sec::IntBitwiseOp, sec::IntShiftCheckedOp,
                        sec::IntCmpOp, sec::NamedConvertOp>();
    target.addDynamicallyLegalOp<sec::IntBitExtractOp>(
        [](Operation *op) { return feedsRegisterConversion(op); });
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
    patterns.add<NamedIntegerStorageLowering>(typeConverter, context);
    patterns.add<FunctionTypeLowering, BranchLowering, CondBranchLowering,
                 SecIntegerStoreLowering,
                 SecCallTypeLowering<sec::CallDirectOp>,
                 SecCallTypeLowering<sec::CallForeignOp>, SecConstIntLowering,
                 ArithConstantTypeLowering, NumericConvertExactLowering,
                 NumericConvertCheckedLowering>(
        typeConverter, context);
    for (StringRef name : {sec::StaticLoadOp::getOperationName(),
                           sec::StaticStoreOp::getOperationName()})
      patterns.add<StaticAccessTypeLowering>(typeConverter, context, name);
    for (StringRef operationName : {
             sec::IntUnaryPlusOp::getOperationName(),
             sec::IntNegCheckedOp::getOperationName(),
             sec::IntBinaryCheckedOp::getOperationName(),
             sec::IntBitNotOp::getOperationName(),
             sec::IntBitwiseOp::getOperationName(),
             sec::IntShiftCheckedOp::getOperationName(),
             sec::IntCmpOp::getOperationName(),
             sec::NamedConvertOp::getOperationName(),
             sec::IntBitExtractOp::getOperationName()})
      patterns.add<IntegerSemanticOpLowering>(typeConverter, context,
                                              operationName);
    populateCallOpTypeConversionPattern(patterns, typeConverter);
    populateReturnOpTypeConversionPattern(patterns, typeConverter);

    if (failed(applyPartialConversion(module, target, std::move(patterns))))
      return signalPassFailure();
    foldRoundTripCasts(module);

    // A bridge that survives the round-trip folding must be an exact-width
    // signed/unsigned <-> signless integer cast at a Place, reference or
    // high-level storage boundary; anything else is a lowering defect
    // (MD-087 §§ 4(6), 5(4)).
    WalkResult bridges = module.walk([&](UnrealizedConversionCastOp cast) {
      // A named float and its own f32/f64 carrier differ only in nominal
      // identity; the tagged-value pass and reconciliation fold the pair.
      if (cast->getNumOperands() == 1 && cast->getNumResults() == 1) {
        Type fromType = cast->getOperand(0).getType();
        Type toType = cast->getResult(0).getType();
        if ((isa<sec::NamedType>(fromType) || isa<sec::NamedType>(toType)) &&
            isa<FloatType>(namedCarrier(fromType)) &&
            namedCarrier(fromType) == namedCarrier(toType))
          return WalkResult::advance();
      }
      // A named integer storage boundary bridges like its carrier.
      auto from = cast->getNumOperands() == 1
                      ? dyn_cast<IntegerType>(namedCarrier(cast->getOperand(0).getType()))
                      : IntegerType{};
      auto to = cast->getNumResults() == 1
                    ? dyn_cast<IntegerType>(namedCarrier(cast->getResult(0).getType()))
                    : IntegerType{};
      if (from && to && from.getWidth() == to.getWidth() &&
          from.isSignless() != to.isSignless())
        return WalkResult::advance();
      cast.emitOpError(
          "checked integer lowering left a bridge that is not an exact-width "
          "semantic/signless integer representation transition");
      return WalkResult::interrupt();
    });
    if (bridges.wasInterrupted())
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
