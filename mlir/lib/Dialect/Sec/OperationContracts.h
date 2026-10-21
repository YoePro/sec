#ifndef SEC_DIALECT_LOCAL_OPERATIONCONTRACTS_H
#define SEC_DIALECT_LOCAL_OPERATIONCONTRACTS_H

#include "sec/Dialect/Sec/SecTypes.h"
#include "mlir/IR/Operation.h"
#include "llvm/ADT/STLExtras.h"
#include <initializer_list>

namespace sec::detail {
using namespace mlir;

// Shared canonical struct/type/field and metadata vocabulary checks.
// Rules: rules/mlir/packages/sec-mlir-dialect_package13.md — §§33–39.
inline FailureOr<StructType> requireStructType(Type type, Operation *operation) {
  auto structType = dyn_cast<StructType>(type);
  if (!structType)
    return operation->emitOpError("value must have !sec.struct type");
  return structType;
}

// Validate resolved per-field action/origin vocabulary without reclassifying source.
// Rules: rules/mlir/packages/sec-mlir-dialect_package13.md — §§33–36.
inline LogicalResult verifyStringArrayValues(Operation *operation, ArrayAttr values,
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

// Resolve exactly one canonical declaration-order stored field.
// Rules: rules/mlir/packages/sec-mlir-dialect_package13.md — §§18,39.
inline FailureOr<StructFieldAttr> lookupStructField(StructType type,
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

// Inspect an integer's semantic carrier without erasing its nominal SSA type.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 35,102;
inline Type semanticIntegerCarrier(Type type) {
  while (true) {
    if (auto named = dyn_cast<NamedType>(type)) type = named.getBase();
    else if (auto distinct = dyn_cast<DistinctType>(type)) type = distinct.getBase();
    else return type;
  }
}

// Classify builtin semantic integer carriers without admitting signless values.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v4.md — integer types.
inline bool isBuiltinIntegerSemanticType(Type type) {
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

// Retain the original signedness of a builtin semantic integer.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 35,102.
inline bool isSignedIntegerSemanticType(Type type) {
  if (isa<IntType>(type))
    return true;
  auto integer = dyn_cast<IntegerType>(type);
  return integer && integer.isSigned() && isBuiltinIntegerSemanticType(type);
}

// Check a closed vocabulary of resolved semantic metadata.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v10.md — bounds proofs.
inline bool isOneOf(StringRef value, std::initializer_list<StringRef> allowed) {
  return llvm::is_contained(allowed, value);
}

// Validate proven or guarded bounds metadata shared by arrays and slices.
// Rules: rules/mlir/packages/sec-mlir-dialect_package14.md — sections 32-36;
// rules/mlir/packages/sec-mlir-dialect_package16.md — sections 52-54,103.
inline LogicalResult verifyArrayBoundsMetadata(Operation *operation,
                                        StringRef boundsKind,
                                        StringRef boundsProof) {
  if (boundsKind == "runtime-check") {
    if (boundsProof != "guarded")
      return operation->emitOpError(
          "runtime-check bounds proof must be guarded");
    return success();
  }
  if (boundsKind != "proven-safe")
    return operation->emitOpError(
        "bounds_kind must be proven-safe or runtime-check");
  if (!isOneOf(boundsProof,
               {"constant", "range", "branch", "contract", "analysis"}))
    return operation->emitOpError(
        "proven-safe bounds proof must be constant, range, branch, contract, or analysis");
  return success();
}

// Validate the same P15 borrow/lifetime and validity policy for references
// and bounded slices; no second reference metadata model is introduced.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v11.md — section 7;
// rules/mlir/packages/sec-mlir-dialect_package16.md — sections 44,91.
template <typename OpT>
LogicalResult verifyReferenceMetadata(OpT op) {
  if (op.getBorrowId() == 0 || op.getLifetimeId() == 0)
    return op.emitOpError("borrow_id and lifetime_id must be non-zero");
  if (op.getAddressStability().empty() || op.getProvenance().empty())
    return op.emitOpError("address_stability and provenance must be non-empty");
  StringRef policy = op.getValidityPolicy();
  if (policy == "proven") {
    if (!op.getValidityProof() || op.getValidityProof()->empty())
      return op.emitOpError("a proven reference requires validity_proof");
    if (op.getEpochDependency())
      return op.emitOpError("a proven reference has no epoch_dependency");
    return success();
  }
  if (policy == "dynamic-epoch") {
    if (!op.getInvalidationDomain() || *op.getInvalidationDomain() == 0 ||
        !op.getEpochDependency() || *op.getEpochDependency() == 0)
      return op.emitOpError(
          "a dynamic-epoch reference requires invalidation_domain and epoch_dependency");
    return success();
  }
  return op.emitOpError("validity_policy must be proven or dynamic-epoch");
}

} // namespace sec::detail
#endif
