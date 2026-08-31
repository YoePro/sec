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

int64_t schemaVersion(Operation *operation) {
  if (auto module = operation->getParentOfType<ModuleOp>())
    if (auto version = module->getAttrOfType<IntegerAttr>("sec.dialect_version"))
      return version.getInt();
  return 0;
}

LogicalResult verifyCheckedResultTuple(Operation *operation, Value operand) {
  const unsigned expected = schemaVersion(operation) >= 5 ? 3 : 2;
  if (operation->getNumResults() != expected)
    return operation->emitOpError()
           << "schema " << schemaVersion(operation) << " requires " << expected
           << " checked results";
  if (operand.getType() != operation->getResult(0).getType())
    return operation->emitOpError("operand and result types must match exactly");
  auto failed = dyn_cast<IntegerType>(operation->getResult(1).getType());
  if (!failed || failed.getWidth() != 1)
    return operation->emitOpError("failed result must be i1");
  if (expected == 3 &&
      !isa<ArithmeticFailureReasonType>(operation->getResult(2).getType()))
    return operation->emitOpError(
        "reason result must be !sec.arithmetic_failure_reason");
  return success();
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
  if (failed(verifyCheckedResultTuple(*this, getValue())))
    return failure();
  return verifyUnaryInteger(*this, getValue(), (*this)->getResult(0));
}

LogicalResult IntBinaryCheckedOp::verify() {
  if (!isOneOf(getKind(), {"add", "subtract", "multiply", "divide", "remainder"}))
    return emitOpError("kind must be add, subtract, multiply, divide, or remainder");
  if (failed(verifyCheckedResultTuple(*this, getLeft())))
    return failure();
  return verifyBinaryInteger(*this, getLeft(), getRight(), (*this)->getResult(0));
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
  if (failed(verifyCheckedResultTuple(*this, getValue())))
    return failure();
  if (getValue().getType() != (*this)->getResult(0).getType())
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
  if (schemaVersion(*this) >= 5) {
    if ((*this)->getNumOperands() != 1 ||
        !isa<ArithmeticFailureReasonType>((*this)->getOperand(0).getType()))
      return emitOpError(
          "schema 5 requires one !sec.arithmetic_failure_reason operand");
    if ((*this)->hasAttr("category"))
      return emitOpError("schema 5 does not use category attribute");
  if (auto constant = (*this)->getOperand(0).getDefiningOp<
      ArithmeticFailureReasonConstantOp>())
    if (constant.getValue() == "none")
    return emitOpError("cannot consume the none arithmetic failure reason");
  } else {
    auto category = (*this)->getAttrOfType<StringAttr>("category");
    if ((*this)->getNumOperands() != 0 || !category ||
        !isOneOf(category.getValue(),
                 {"overflow", "division", "remainder", "shift"}))
      return emitOpError("schema 4 requires a valid arithmetic category");
  }
  auto sourceOperator = (*this)->getAttrOfType<StringAttr>("sec.operator");
  if (!sourceOperator || sourceOperator.getValue().empty())
    return emitOpError("requires non-empty sec.operator string attribute");
  return success();
}

LogicalResult ArithmeticFailureReasonConstantOp::verify() {
  if (!isa<ArithmeticFailureReasonType>(getResult().getType()))
    return emitOpError("result must be !sec.arithmetic_failure_reason");
  if (!isOneOf(getValue(), {"none", "overflow", "division-by-zero",
                            "invalid-shift"}))
    return emitOpError("invalid arithmetic failure reason");
  return success();
}

LogicalResult ArithmeticErrorFromReasonOp::verify() {
  if (!isa<ArithmeticFailureReasonType>(getReason().getType()))
    return emitOpError("operand must be !sec.arithmetic_failure_reason");
  bool arithmeticError = false;
  if (auto legacy = dyn_cast<CoreErrorType>(getResult().getType()))
    arithmeticError = legacy.getIdentity().getValue() == "core::ArithmeticError";
  if (auto enumType = dyn_cast<EnumType>(getResult().getType()))
    arithmeticError = enumType.getIdentity().getValue() == "core::ArithmeticError";
  if (!arithmeticError)
    return emitOpError(
        "result must be the canonical core::ArithmeticError semantic type");
  if (auto constant = getReason().getDefiningOp<
      ArithmeticFailureReasonConstantOp>())
    if (constant.getValue() == "none")
    return emitOpError("cannot map the none arithmetic failure reason");
  return success();
}

template <typename ResultOp>
LogicalResult verifyResultConstructor(ResultOp operation, bool successValue) {
  auto result = dyn_cast<ResultType>(operation.getResult().getType());
  if (!result)
    return operation.emitOpError("result must have !sec.result type");
  Type expected = successValue ? result.getSuccessType() : result.getErrorType();
  if (operation->getNumOperands() != 1 ||
      operation->getOperand(0).getType() != expected)
    return operation.emitOpError("payload type must exactly match Result component");
  return success();
}

LogicalResult ResultOkOp::verify() {
  return verifyResultConstructor(*this, true);
}

LogicalResult ResultErrOp::verify() {
  return verifyResultConstructor(*this, false);
}

LogicalResult ResultIsErrOp::verify() {
  if (!isa<ResultType>(getValue().getType()))
    return emitOpError("operand must have !sec.result type");
  return success();
}

LogicalResult ResultUnwrapOkOp::verify() {
  auto result = dyn_cast<ResultType>(getValue().getType());
  if (!result)
    return emitOpError("operand must have !sec.result type");
  if (getResult().getType() != result.getSuccessType())
    return emitOpError("result type must exactly match Result success type");
  return success();
}

LogicalResult ResultUnwrapErrOp::verify() {
  auto result = dyn_cast<ResultType>(getValue().getType());
  if (!result)
    return emitOpError("operand must have !sec.result type");
  if (getResult().getType() != result.getErrorType())
    return emitOpError("result type must exactly match Result error type");
  return success();
}

LogicalResult CoreErrorIsVariantOp::verify() {
  auto error = dyn_cast<CoreErrorType>(getValue().getType());
  if (!error || error.getIdentity().getValue() != "core::ArithmeticError")
    return emitOpError(
        "operand must be !sec.core_error<\"core::ArithmeticError\">");
  if (!isOneOf(getVariant(),
               {"Overflow", "DivisionByZero", "InvalidShift"}))
    return emitOpError("unknown ArithmeticError variant");
  return success();
}

namespace {
FailureOr<UnionVariantAttr> lookupUnionVariant(Type type, IntegerAttr index,
                                               Operation *operation) {
  auto unionType = dyn_cast<UnionType>(type);
  if (!unionType)
    return operation->emitOpError("value must have !sec.union type");
  int64_t ordinal = index.getInt();
  if (ordinal < 0 || static_cast<uint64_t>(ordinal) >= unionType.getVariants().size())
    return operation->emitOpError("union variant index is out of range");
  auto variant = dyn_cast<UnionVariantAttr>(unionType.getVariants()[ordinal]);
  if (!variant)
    return operation->emitOpError("union variant table contains an invalid attribute");
  if (variant.getIndex() != static_cast<uint32_t>(ordinal))
    return operation->emitOpError("union variant table is not canonical");
  return variant;
}

LogicalResult verifyCopyTrivialAction(Operation *operation, StringRef action) {
  if (action != "copy-trivial")
    return operation->emitOpError(
        "Package 11 union payload action must be copy-trivial");
  return success();
}
} // namespace

LogicalResult EnumConstantOp::verify() {
  auto enumType = dyn_cast<EnumType>(getResult().getType());
  if (!enumType)
    return emitOpError("result must have !sec.enum type");
  int64_t ordinal = getCaseOrdinalAttr().getInt();
  if (ordinal < 0 || static_cast<uint64_t>(ordinal) >= enumType.getCases().size())
    return emitOpError("case ordinal is out of range for result enum");
  auto enumCase = dyn_cast<EnumCaseAttr>(enumType.getCases()[ordinal]);
  if (!enumCase || enumCase.getOrdinal() != static_cast<uint32_t>(ordinal))
    return emitOpError("result enum case table is not canonical");
  return success();
}

LogicalResult EnumFromIntegerOp::verify() {
  auto enumType = dyn_cast<EnumType>(getResult().getType());
  if (!enumType)
    return emitOpError("result must have !sec.enum type");
  if (!isBuiltinIntegerSemanticType(getValue().getType()))
    return emitOpError("operand must have an integer semantic type");
  return success();
}

LogicalResult EnumToIntegerOp::verify() {
  if (!isa<EnumType>(getValue().getType()))
    return emitOpError("operand must have !sec.enum type");
  if (!isBuiltinIntegerSemanticType(getResult().getType()))
    return emitOpError("result must have an integer semantic type");
  return success();
}

LogicalResult EnumCmpOp::verify() {
  if (!isa<EnumType>(getLeft().getType()) ||
      getLeft().getType() != getRight().getType())
    return emitOpError("operands must have the same !sec.enum type");
  if (getPredicate() != "eq" && getPredicate() != "ne")
    return emitOpError("predicate must be eq or ne");
  return success();
}

LogicalResult UnionConstructOp::verify() {
  auto variant = lookupUnionVariant(getResult().getType(), getVariantAttr(), *this);
  if (failed(variant))
    return failure();
  if (getPayloadActions().size() != getPayloads().size())
    return emitOpError("payload action count must equal payload count");
  for (Attribute action : getPayloadActions()) {
    auto string = dyn_cast<StringAttr>(action);
    if (!string || failed(verifyCopyTrivialAction(*this, string.getValue())))
      return failure();
  }
  StringRef kind = variant->getKind().getValue();
  if (kind == "empty") {
    if (!getPayloads().empty() || !getFieldNames().empty())
      return emitOpError("empty variant must not have payloads or field names");
  } else if (kind == "single") {
    if (getPayloads().size() != 1 || !getFieldNames().empty() ||
        getPayloads()[0].getType() != variant->getPayload())
      return emitOpError("single variant payload type must match exactly");
  } else {
    if (getPayloads().size() != variant->getFields().size() ||
        getFieldNames().size() != variant->getFields().size())
      return emitOpError("fields variant payload arity must match exactly");
    for (auto [position, attribute] : llvm::enumerate(variant->getFields())) {
      auto field = cast<UnionFieldAttr>(attribute);
      auto name = dyn_cast<StringAttr>(getFieldNames()[position]);
      if (!name || name.getValue() != field.getName().getValue() ||
          getPayloads()[position].getType() != field.getType())
        return emitOpError(
            "fields variant operands must use declaration order and exact types");
    }
  }
  return success();
}

LogicalResult UnionIsVariantOp::verify() {
  if (failed(lookupUnionVariant(getValue().getType(), getVariantAttr(), *this)))
    return failure();
  return success();
}

LogicalResult UnionUnwrapPayloadOp::verify() {
  auto variant = lookupUnionVariant(getValue().getType(), getVariantAttr(), *this);
  if (failed(variant))
    return failure();
  if (variant->getKind().getValue() != "single" ||
      variant->getPayload() != getResult().getType())
    return emitOpError("projection requires matching single-payload variant");
  return verifyCopyTrivialAction(*this, getPayloadAction());
}

LogicalResult UnionUnwrapFieldOp::verify() {
  auto variant = lookupUnionVariant(getValue().getType(), getVariantAttr(), *this);
  if (failed(variant))
    return failure();
  if (variant->getKind().getValue() != "fields")
    return emitOpError("field projection requires a fields variant");
  if (failed(verifyCopyTrivialAction(*this, getPayloadAction())))
    return failure();
  for (Attribute attribute : variant->getFields()) {
    auto field = cast<UnionFieldAttr>(attribute);
    if (field.getName().getValue() == getField() &&
        field.getType() == getResult().getType())
      return success();
  }
  return emitOpError("field does not exist with the projected result type");
}

namespace {
FailureOr<StructType> requireStructType(Type type, Operation *operation) {
  auto structType = dyn_cast<StructType>(type);
  if (!structType)
    return operation->emitOpError("value must have !sec.struct type");
  return structType;
}

LogicalResult verifyStringArrayValues(Operation *operation, ArrayAttr values,
                                      ArrayRef<StringRef> allowed,
                                      StringRef description) {
  for (Attribute attribute : values) {
    auto value = dyn_cast<StringAttr>(attribute);
    if (!value || !llvm::is_contained(allowed, value.getValue()))
      return operation->emitOpError()
             << description << " contains an invalid value";
  }
  return success();
}

FailureOr<StructFieldAttr> lookupStructField(StructType type,
                                             IntegerAttr ordinal,
                                             Operation *operation) {
  int64_t index = ordinal.getInt();
  if (index < 0 || static_cast<uint64_t>(index) >= type.getFields().size())
    return operation->emitOpError("struct field ordinal is out of range");
  auto field = dyn_cast<StructFieldAttr>(type.getFields()[index]);
  if (!field || field.getOrdinal() != static_cast<uint32_t>(index))
    return operation->emitOpError("struct field table is not canonical");
  return field;
}
} // namespace

LogicalResult StructConstructOp::verify() {
  auto type = requireStructType(getResult().getType(), *this);
  if (failed(type))
    return failure();
  if (getFields().size() != type->getFields().size() ||
      getFieldOrigins().size() != getFields().size() ||
      getFieldActions().size() != getFields().size())
    return emitOpError("field operands, origins, and actions must match the stored-field count");
  if (failed(verifyStringArrayValues(
          *this, getFieldOrigins(), {"explicit", "spread", "default"},
          "field origins")) ||
      failed(verifyStringArrayValues(
          *this, getFieldActions(), {"construct-direct", "copy-trivial"},
          "field actions")))
    return failure();
  for (auto [index, attribute] : llvm::enumerate(type->getFields())) {
    auto field = cast<StructFieldAttr>(attribute);
    if (getFields()[index].getType() != field.getType())
      return emitOpError("field operands must use declaration order and exact types");
  }
  return success();
}

LogicalResult StructSpreadFieldsOp::verify() {
  auto type = requireStructType(getSource().getType(), *this);
  if (failed(type))
    return failure();
  if (getFields().size() != type->getFields().size() ||
      getActions().size() != getFields().size())
    return emitOpError("results and actions must match the stored-field count");
  if (failed(verifyStringArrayValues(*this, getActions(), {"copy-trivial"},
                                     "spread actions")))
    return failure();
  for (auto [index, attribute] : llvm::enumerate(type->getFields())) {
    auto field = cast<StructFieldAttr>(attribute);
    if (getFields()[index].getType() != field.getType())
      return emitOpError("results must use declaration-order field types");
  }
  return success();
}

LogicalResult StructExtractOp::verify() {
  auto type = requireStructType(getSource().getType(), *this);
  if (failed(type))
    return failure();
  auto field = lookupStructField(*type, getFieldAttr(), *this);
  if (failed(field))
    return failure();
  if (getAction() != "copy-trivial")
    return emitOpError("schema 9 struct extract action must be copy-trivial");
  if (getResult().getType() != field->getType())
    return emitOpError("result type must exactly match the selected field type");
  return success();
}

LogicalResult StructReplaceFieldOp::verify() {
  auto type = requireStructType(getSource().getType(), *this);
  if (failed(type))
    return failure();
  auto field = lookupStructField(*type, getFieldAttr(), *this);
  if (failed(field))
    return failure();
  if (getResult().getType() != getSource().getType())
    return emitOpError("source and result struct types must match exactly");
  if (getReplacement().getType() != field->getType())
    return emitOpError("replacement type must exactly match the selected field type");
  return success();
}

namespace {
FailureOr<APInt> parseCanonicalArrayLength(Attribute attribute,
                                           Operation *operation) {
  auto string = dyn_cast<StringAttr>(attribute);
  if (!string)
    return operation->emitOpError(
        "array segment lengths must be string attributes");
  StringRef spelling = string.getValue();
  if (spelling.empty() ||
      (spelling.size() > 1 && spelling.front() == '0') ||
      !llvm::all_of(spelling, [](char value) {
        return value >= '0' && value <= '9';
      }))
    return operation->emitOpError(
        "array segment lengths must be canonical unsigned decimal");
  APInt value;
  if (spelling.getAsInteger(10, value))
    return operation->emitOpError("array segment length is not decimal");
  return value;
}

APInt addUnsignedExact(APInt left, const APInt &right) {
  unsigned width = std::max(left.getBitWidth(), right.getBitWidth()) + 1;
  return left.zext(width) + right.zext(width);
}
} // namespace

// ArrayConstructOp::verify enforces compact source-ordered fixed-array
// construction without expanding spreads. Rules:
// rules/mlir/packages/sec-mlir-dialect_package14.md sections 62-63 and
// rules/mlir/dialect-versions/sec_mlir_dialect_v10.md sections 7-10.
LogicalResult ArrayConstructOp::verify() {
  auto resultType = dyn_cast<ArrayType>(getResult().getType());
  if (!resultType)
    return emitOpError("result must have !sec.array type");
  if (getSegmentKinds().size() != getSegments().size() ||
      getSegmentLengths().size() != getSegments().size() ||
      getSegmentActions().size() != getSegments().size())
    return emitOpError(
        "segment operands, kinds, lengths, and actions must have equal counts");

  APInt total(1, 0);
  for (auto [index, operand] : llvm::enumerate(getSegments())) {
    auto kind = dyn_cast<StringAttr>(getSegmentKinds()[index]);
    auto action = dyn_cast<StringAttr>(getSegmentActions()[index]);
    if (!kind || !action)
      return emitOpError("array segment kinds and actions must be strings");
    auto length = parseCanonicalArrayLength(getSegmentLengths()[index], *this);
    if (failed(length))
      return failure();

    if (kind.getValue() == "element") {
      if (action.getValue() != "construct-direct")
        return emitOpError(
            "element segment action must be construct-direct");
      if (*length != 1)
        return emitOpError("element segment length must be 1");
      if (operand.getType() != resultType.getElementType())
        return emitOpError(
            "element segment type must match the result element type");
    } else if (kind.getValue() == "spread") {
      if (action.getValue() != "copy-trivial")
        return emitOpError("spread segment action must be copy-trivial");
      auto spreadType = dyn_cast<ArrayType>(operand.getType());
      if (!spreadType ||
          spreadType.getElementType() != resultType.getElementType())
        return emitOpError(
            "spread segment must be an array of the result element type");
      auto spreadLength =
          parseCanonicalArrayLength(spreadType.getLength(), *this);
      if (failed(spreadLength))
        return failure();
      if (*length != *spreadLength)
        return emitOpError(
            "spread segment length must match its operand array length");
    } else {
      return emitOpError("array segment kind must be element or spread");
    }
    total = addUnsignedExact(std::move(total), *length);
  }

  auto expected = parseCanonicalArrayLength(resultType.getLength(), *this);
  if (failed(expected))
    return failure();
  unsigned width = std::max(total.getBitWidth(), expected->getBitWidth());
  if (total.zext(width) != expected->zext(width))
    return emitOpError(
        "exact segment length sum must match the result array length");
  return success();
}

LogicalResult UnreachableOp::verify() {
  auto synthesized = (*this)->getAttrOfType<BoolAttr>("sec.synthesized");
  if (!synthesized || !synthesized.getValue())
    return emitOpError("requires sec.synthesized = true");
  if (getReason().empty())
    return emitOpError("requires a non-empty reason");
  return success();
}
