#include "sec/Dialect/Sec/SecTypes.h"

#include "sec/Dialect/Sec/SecDialect.h"
#include "sec/Dialect/Sec/SecAttributes.h"
#include "llvm/ADT/APInt.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/StringSet.h"
#include "llvm/ADT/TypeSwitch.h"
#include "mlir/IR/Builders.h"
#include "mlir/IR/DialectImplementation.h"
#include "mlir/IR/Diagnostics.h"

using namespace mlir;
using namespace sec;

#define GET_TYPEDEF_CLASSES
#include "sec/Dialect/Sec/SecTypes.cpp.inc"

static LogicalResult verifyIdentityType(
    function_ref<InFlightDiagnostic()> emitError, StringAttr identity,
    Type base, StringRef typeName) {
  if (!identity || identity.getValue().empty())
    return emitError() << typeName << " identity must not be empty";
  if (!base)
    return emitError() << typeName << " base type must be present";
  if (isa<NoneType>(base))
    return emitError() << typeName << " base type must not be none";
  return success();
}

LogicalResult NamedType::verify(function_ref<InFlightDiagnostic()> emitError,
                                StringAttr identity, Type base) {
  return verifyIdentityType(emitError, identity, base, "sec.named");
}

LogicalResult
DistinctType::verify(function_ref<InFlightDiagnostic()> emitError,
                     StringAttr identity, Type base) {
  return verifyIdentityType(emitError, identity, base, "sec.distinct");
}

LogicalResult
StorageType::verify(function_ref<InFlightDiagnostic()> emitError,
                    Type elementType) {
  if (!elementType || isa<NoneType>(elementType))
    return emitError() << "sec.storage element type must be non-void";
  if (isa<StorageType>(elementType))
    return emitError() << "sec.storage cannot contain another sec.storage";
  return success();
}

LogicalResult CoreErrorType::verify(
    function_ref<InFlightDiagnostic()> emitError, StringAttr identity) {
  if (!identity || identity.getValue().empty())
    return emitError() << "sec.core_error identity must not be empty";
  return success();
}

LogicalResult ResultType::verify(
    function_ref<InFlightDiagnostic()> emitError, Type successType,
    Type errorType) {
  if (!successType || !errorType || isa<NoneType>(successType) ||
      isa<NoneType>(errorType))
    return emitError() << "sec.result requires non-void success and error types";
  return success();
}

namespace {
bool isEnumUnderlying(Type type) {
  if (isa<IntType, UIntType>(type))
    return true;
  auto integer = dyn_cast<IntegerType>(type);
  return integer && integer.getWidth() >= 1 && integer.getWidth() <= 256 &&
         !integer.isSignless();
}

bool enumValueFits(StringRef spelling, Type underlying, uint32_t bitWidth) {
  bool negative = spelling.consume_front("-");
  spelling.consume_front("+");
  unsigned required = std::max(1u, APInt::getBitsNeeded(spelling, 10));
  APInt magnitude(required, spelling, 10);
  if (bitWidth != 0)
    return !negative && magnitude.getActiveBits() <= bitWidth;
  auto integer = dyn_cast<IntegerType>(underlying);
  if (!integer || isa<IntType, UIntType>(underlying))
    return true;
  if (integer.isUnsigned())
    return !negative && magnitude.getActiveBits() <= integer.getWidth();
  if (!negative)
    return magnitude.getActiveBits() < integer.getWidth();
  return magnitude.getActiveBits() < integer.getWidth() ||
         (magnitude.getActiveBits() == integer.getWidth() &&
          magnitude.isPowerOf2());
}
} // namespace

LogicalResult EnumType::verify(
    function_ref<InFlightDiagnostic()> emitError, StringAttr identity,
    Type underlying, StringAttr representation, uint32_t bitWidth,
    ArrayAttr cases) {
  if (!identity || identity.getValue().empty())
    return emitError() << "sec.enum identity must not be empty";
  if (!isEnumUnderlying(underlying))
    return emitError() << "sec.enum underlying type must be a semantic integer";
  if (!representation)
    return emitError() << "sec.enum representation must be present";
  if (representation.getValue() == "integer") {
    if (bitWidth != 0)
      return emitError() << "integer sec.enum requires bit_width = 0";
  } else if (representation.getValue() == "bit-backed") {
    if (bitWidth == 0 || bitWidth > 256)
      return emitError() << "bit-backed sec.enum width must be 1 through 256";
  } else {
    return emitError() << "sec.enum representation must be integer or bit-backed";
  }
  llvm::StringSet<> names;
  for (auto [ordinal, attribute] : llvm::enumerate(cases)) {
    auto enumCase = dyn_cast<EnumCaseAttr>(attribute);
    if (!enumCase)
      return emitError() << "enum cases must be #sec.enum_case attributes";
    if (enumCase.getOrdinal() != ordinal)
      return emitError() << "sec.enum case ordinals must be contiguous from zero";
    if (!names.insert(enumCase.getName().getValue()).second)
      return emitError() << "sec.enum case names must be unique";
    if (!enumValueFits(enumCase.getValue().getValue(), underlying, bitWidth))
      return emitError() << "sec.enum case value is not representable";
  }
  return success();
}

LogicalResult UnionType::verify(
    function_ref<InFlightDiagnostic()> emitError, StringAttr identity,
    ArrayAttr typeArguments, ArrayAttr variants) {
  if (!identity || identity.getValue().empty())
    return emitError() << "sec.union identity must not be empty";
  if (variants.empty())
    return emitError() << "sec.union requires at least one variant";
  for (Attribute argument : typeArguments)
    if (!isa<TypeAttr>(argument))
      return emitError() << "sec.union type arguments must be type attributes";
  llvm::StringSet<> names;
  for (auto [index, attribute] : llvm::enumerate(variants)) {
    auto variant = dyn_cast<UnionVariantAttr>(attribute);
    if (!variant)
      return emitError() << "union variants must be #sec.union_variant attributes";
    if (variant.getIndex() != index)
      return emitError() << "sec.union variant indices must be contiguous from zero";
    if (!names.insert(variant.getName().getValue()).second)
      return emitError() << "sec.union variant names must be unique";
  }
  return success();
}

void SecDialect::registerTypes() {
  addTypes<
#define GET_TYPEDEF_LIST
#include "sec/Dialect/Sec/SecTypes.cpp.inc"
      >();
}
