#include "sec/Dialect/Sec/SecTypes.h"

#include "sec/Dialect/Sec/SecDialect.h"
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

void SecDialect::registerTypes() {
  addTypes<
#define GET_TYPEDEF_LIST
#include "sec/Dialect/Sec/SecTypes.cpp.inc"
      >();
}
