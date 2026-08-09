#include "sec/Dialect/Sec/SecOps.h"

#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/APInt.h"
#include "llvm/ADT/STLExtras.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/BuiltinAttributes.h"
#include "mlir/IR/BuiltinTypes.h"
#include "mlir/IR/SymbolTable.h"

#include <cctype>

using namespace mlir;
using namespace sec;

#define GET_OP_CLASSES
#include "sec/Dialect/Sec/SecOps.cpp.inc"

namespace {

Type unwrapIdentityType(Type type) {
  while (true) {
    if (auto named = dyn_cast<NamedType>(type)) {
      type = named.getBase();
      continue;
    }
    if (auto distinct = dyn_cast<DistinctType>(type)) {
      type = distinct.getBase();
      continue;
    }
    return type;
  }
}

struct ParsedInteger {
  APInt magnitude;
  bool negative;
};

FailureOr<ParsedInteger> parseDecimalInteger(StringRef spelling) {
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

bool fitsSigned(const ParsedInteger &value, unsigned width) {
  if (width == 0)
    return false;
  unsigned activeBits = value.magnitude.getActiveBits();
  if (!value.negative)
    return activeBits <= width - 1;
  if (activeBits < width)
    return true;
  return activeBits == width && value.magnitude.isPowerOf2();
}

bool fitsUnsigned(const ParsedInteger &value, unsigned width) {
  return !value.negative && value.magnitude.getActiveBits() <= width;
}

bool isBuiltinIntegerSemanticType(Type type) {
  if (isa<IntType, UIntType>(type))
    return true;
  auto integer = dyn_cast<IntegerType>(type);
  if (!integer || integer.getWidth() == 1 || integer.isSignless())
    return false;
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

bool isSignedIntegerSemanticType(Type type) {
  if (isa<IntType>(type))
    return true;
  auto integer = dyn_cast<IntegerType>(type);
  return integer && integer.isSigned() && isBuiltinIntegerSemanticType(type);
}

LogicalResult verifyUnaryInteger(Operation *operation, Value operand,
                                 Value result) {
  if (!isBuiltinIntegerSemanticType(operand.getType()))
    return operation->emitOpError("operand must be a builtin Sec integer semantic type");
  if (operand.getType() != result.getType())
    return operation->emitOpError("operand and result types must match exactly");
  return success();
}

LogicalResult verifyBinaryInteger(Operation *operation, Value left, Value right,
                                  Value result) {
  if (!isBuiltinIntegerSemanticType(left.getType()))
    return operation->emitOpError("operands must have a builtin Sec integer semantic type");
  if (left.getType() != right.getType() || left.getType() != result.getType())
    return operation->emitOpError("operand and result types must match exactly");
  return success();
}

bool isOneOf(StringRef value, std::initializer_list<StringRef> allowed) {
  return llvm::is_contained(allowed, value);
}

LogicalResult verifyIntegerResult(Operation *operation, StringAttr valueAttr,
                                  Type resultType) {
  auto parsed = parseDecimalInteger(valueAttr.getValue());
  if (failed(parsed))
    return operation->emitOpError("value must be a base-10 integer string");

  Type base = unwrapIdentityType(resultType);
  if (isa<IntType>(base))
    return success();
  if (isa<UIntType, CharType, RuneType>(base)) {
    if (parsed->negative)
      return operation->emitOpError("unsigned result cannot contain a negative value");
    return success();
  }
  auto integer = dyn_cast<IntegerType>(base);
  if (!integer || integer.getWidth() == 1)
    return operation->emitOpError("result must have a Sec integer semantic type");
  if (integer.isUnsigned() && parsed->negative)
    return operation->emitOpError(
        "unsigned result cannot contain a negative value");
  bool fits = integer.isSigned()
                  ? fitsSigned(*parsed, integer.getWidth())
                  : integer.isUnsigned()
                        ? fitsUnsigned(*parsed, integer.getWidth())
                        : parsed->negative
                              ? fitsSigned(*parsed, integer.getWidth())
                              : fitsUnsigned(*parsed, integer.getWidth());
  if (!fits)
    return operation->emitOpError("integer value is not representable by result type");
  return success();
}

LogicalResult verifyStoragePair(Operation *operation, Value storage,
                                Value value) {
  auto storageType = dyn_cast<StorageType>(storage.getType());
  if (!storageType)
    return operation->emitOpError("storage operand must have !sec.storage type");
  if (storageType.getElementType() != value.getType())
    return operation->emitOpError("storage element and value types must match exactly");
  return success();
}

LogicalResult verifyArgumentActions(Operation *operation, unsigned arity) {
  auto actions = operation->getAttrOfType<ArrayAttr>("sec.argument_actions");
  if (!actions)
    return operation->emitOpError("requires sec.argument_actions array attribute");
  if (actions.size() != arity)
    return operation->emitOpError("argument action count must equal operand count");
  for (Attribute action : actions) {
    auto string = dyn_cast<StringAttr>(action);
    if (!string || string.getValue() != "copy-trivial")
      return operation->emitOpError("only copy-trivial argument actions are valid in schema 2");
  }
  return success();
}

template <typename CallOp>
LogicalResult verifyCall(CallOp call, bool requireExtern) {
  auto target = SymbolTable::lookupNearestSymbolFrom<func::FuncOp>(
      call, call.getCalleeAttr());
  if (!target)
    return call.emitOpError("callee must resolve to func.func");
  auto externAttr = target->template getAttrOfType<BoolAttr>("sec.extern");
  if (!externAttr)
    return call.emitOpError("callee func.func requires sec.extern");
  if (externAttr.getValue() != requireExtern)
    return call.emitOpError(requireExtern
                                ? "foreign call target must be extern"
                                : "direct call target must not be extern");
  FunctionType signature = target.getFunctionType();
  if (!llvm::equal(call->getOperandTypes(), signature.getInputs()))
    return call.emitOpError("operand types must match callee inputs");
  if (!llvm::equal(call->getResultTypes(), signature.getResults()))
    return call.emitOpError("result types must match callee results");
  return verifyArgumentActions(call, call->getNumOperands());
}

} // namespace

LogicalResult ConstIntOp::verify() {
  return verifyIntegerResult(*this, getValueAttr(), getResult().getType());
}

LogicalResult ConstBoolOp::verify() {
  Type base = unwrapIdentityType(getResult().getType());
  auto integer = dyn_cast<IntegerType>(base);
  if (!integer || integer.getWidth() != 1)
    return emitOpError("result must be i1 or an identity wrapper over i1");
  return success();
}

LogicalResult ConstFloatOp::verify() {
  if (getLexeme().empty())
    return emitOpError("lexeme must not be empty");
  Type base = unwrapIdentityType(getResult().getType());
  if (isa<sec::FloatType>(base))
    return success();
  auto builtin = dyn_cast<mlir::FloatType>(base);
  if (!builtin || (builtin.getWidth() != 32 && builtin.getWidth() != 64))
    return emitOpError("result must be !sec.float, f32, or f64");
  return success();
}

LogicalResult ConstDecimalOp::verify() {
  if (failed(parseDecimalInteger(getCoefficient())))
    return emitOpError("coefficient must be a base-10 integer string");
  if (getScaleAttr().getValue().isNegative())
    return emitOpError("scale must be non-negative");
  if (getLexeme().empty())
    return emitOpError("lexeme must not be empty");
  if (!isa<DecimalType, Decimal128Type>(
          unwrapIdentityType(getResult().getType())))
    return emitOpError(
        "result must be !sec.decimal, !sec.decimal128, or an identity wrapper");
  return success();
}

LogicalResult ConstStringOp::verify() {
  if (!isa<StringType>(unwrapIdentityType(getResult().getType())))
    return emitOpError("result must be !sec.string or an identity wrapper");
  return success();
}

LogicalResult StorageDeclareOp::verify() {
  if (!isa<StorageType>(getStorage().getType()))
    return emitOpError("result must have !sec.storage type");
  auto id = (*this)->getAttrOfType<IntegerAttr>("sec.storage_id");
  auto idType = id ? dyn_cast<IntegerType>(id.getType()) : IntegerType{};
  if (!id || !idType || idType.getWidth() != 32 || id.getInt() <= 0)
    return emitOpError("sec.storage_id must be a positive i32 integer");
  auto name = (*this)->getAttrOfType<StringAttr>("sec.source_name");
  auto synthesized = (*this)->getAttrOfType<BoolAttr>("sec.synthesized");
  if (!name || (name.getValue().empty() &&
                (!synthesized || !synthesized.getValue())))
    return emitOpError("sec.source_name must be non-empty unless synthesized");
  auto storageClass =
      (*this)->getAttrOfType<StringAttr>("sec.storage_class");
  if (!storageClass || storageClass.getValue() != "local-automatic")
    return emitOpError("sec.storage_class must be local-automatic");
  if (!(*this)->getAttrOfType<BoolAttr>("sec.mutable"))
    return emitOpError("requires sec.mutable boolean attribute");
  return success();
}

LogicalResult StorageInitOp::verify() {
  return verifyStoragePair(*this, getStorage(), getValue());
}

LogicalResult StorageLoadOp::verify() {
  auto storageType = dyn_cast<StorageType>(getStorage().getType());
  if (!storageType)
    return emitOpError("operand must have !sec.storage type");
  if (storageType.getElementType() != getResult().getType())
    return emitOpError("storage element and result types must match exactly");
  return success();
}

LogicalResult StorageStoreOp::verify() {
  return verifyStoragePair(*this, getStorage(), getValue());
}

LogicalResult CallDirectOp::verify() { return verifyCall(*this, false); }

LogicalResult CallForeignOp::verify() { return verifyCall(*this, true); }

LogicalResult IntUnaryPlusOp::verify() {
  return verifyUnaryInteger(*this, getValue(), getResult());
}

LogicalResult IntNegCheckedOp::verify() {
  if (!isSignedIntegerSemanticType(getValue().getType()))
    return emitOpError("operand must be a signed builtin Sec integer semantic type");
  return verifyUnaryInteger(*this, getValue(), getResult());
}

LogicalResult IntBinaryCheckedOp::verify() {
  if (!isOneOf(getKind(), {"add", "subtract", "multiply", "divide", "remainder"}))
    return emitOpError("kind must be add, subtract, multiply, divide, or remainder");
  return verifyBinaryInteger(*this, getLeft(), getRight(), getResult());
}

LogicalResult IntBitNotOp::verify() {
  return verifyUnaryInteger(*this, getValue(), getResult());
}

LogicalResult IntBitwiseOp::verify() {
  if (!isOneOf(getKind(), {"and", "or", "xor"}))
    return emitOpError("kind must be and, or, or xor");
  return verifyBinaryInteger(*this, getLeft(), getRight(), getResult());
}

LogicalResult IntShiftCheckedOp::verify() {
  if (!isOneOf(getKind(), {"left_unsigned", "left_signed", "right_unsigned", "right_signed"}))
    return emitOpError("invalid checked shift kind");
  if (!isBuiltinIntegerSemanticType(getValue().getType()) ||
      !isBuiltinIntegerSemanticType(getCount().getType()))
    return emitOpError("value and count must be builtin Sec integer semantic types");
  if (getValue().getType() != getResult().getType())
    return emitOpError("value and result types must match exactly");
  bool signedKind = getKind().contains("_signed");
  if (signedKind != isSignedIntegerSemanticType(getValue().getType()))
    return emitOpError("shift kind signedness must match value type");
  return success();
}

LogicalResult IntCmpOp::verify() {
  if (!isOneOf(getPredicate(), {"eq", "ne", "lt", "le", "gt", "ge"}))
    return emitOpError("invalid integer comparison predicate");
  if (!isBuiltinIntegerSemanticType(getLeft().getType()) ||
      getLeft().getType() != getRight().getType())
    return emitOpError("comparison operands must have the same builtin Sec integer semantic type");
  return success();
}

LogicalResult FailArithmeticOp::verify() {
  if (!isOneOf(getCategory(), {"overflow", "division", "remainder", "shift"}))
    return emitOpError("invalid arithmetic failure category");
  auto sourceOperator = (*this)->getAttrOfType<StringAttr>("sec.operator");
  if (!sourceOperator || sourceOperator.getValue().empty())
    return emitOpError("requires non-empty sec.operator string attribute");
  return success();
}
