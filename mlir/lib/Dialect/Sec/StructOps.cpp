// Canonical struct value operation contracts and the P16 mutable-slice boundary.
// Rules: rules/mlir/packages/sec-mlir-dialect_package13.md — §§33–39;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §114.
#include "sec/Dialect/Sec/SecOps.h"
#include "OperationContracts.h"

using namespace mlir;
using namespace sec;
using namespace sec::detail;

namespace {
// Inspect only owned aggregate storage; reference/slice referents are not copied.
// Rules: rules/memory/copy_move.md — §§4.1–4.4;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39,114.
bool containsOwnedMutableSlice(Type type) {
  if (isa<SliceMutType>(type)) return true;
  if (auto named = dyn_cast<NamedType>(type)) return containsOwnedMutableSlice(named.getBase());
  if (auto named = dyn_cast<DistinctType>(type)) return containsOwnedMutableSlice(named.getBase());
  if (auto array = dyn_cast<ArrayType>(type)) return containsOwnedMutableSlice(array.getElementType());
  if (auto result = dyn_cast<ResultType>(type))
    return containsOwnedMutableSlice(result.getSuccessType()) || containsOwnedMutableSlice(result.getErrorType());
  if (auto structure = dyn_cast<StructType>(type))
    for (Attribute attr : structure.getFields())
      if (containsOwnedMutableSlice(cast<StructFieldAttr>(attr).getType())) return true;
  if (auto sum = dyn_cast<UnionType>(type)) {
    for (Attribute attr : sum.getVariants()) {
      auto variant = cast<UnionVariantAttr>(attr);
      if (variant.getKind().getValue() == "single" && containsOwnedMutableSlice(variant.getPayload())) return true;
      for (Attribute field : variant.getFields())
        if (containsOwnedMutableSlice(cast<UnionFieldAttr>(field).getType())) return true;
    }
  }
  return false;
}
} // namespace

// Verify declaration-order construction without exclusive-holder duplication.
// Rules: rules/mlir/packages/sec-mlir-dialect_package13.md — §§33–39;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §114.
LogicalResult StructConstructOp::verify() {
  if (containsOwnedMutableSlice(getResult().getType()))
    return emitOpError("mutable slice aggregate construction requires move-aware lowering (P17)");
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

// Verify exact shared/trivial field copies in a struct spread.
// Rules: rules/mlir/packages/sec-mlir-dialect_package13.md — §§33–39;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §114.
LogicalResult StructSpreadFieldsOp::verify() {
  if (containsOwnedMutableSlice(getSource().getType()))
    return emitOpError("cannot copy a mutable slice through struct spread");
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

// Verify field extraction copies only copyable holders.
// Rules: rules/mlir/packages/sec-mlir-dialect_package13.md — §§33–39;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §114.
LogicalResult StructExtractOp::verify() {
  if (containsOwnedMutableSlice(getResult().getType()))
    return emitOpError("cannot copy a mutable slice through struct extract");
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

// Verify trivial replacement preserves exact field and aggregate identities.
// Rules: rules/mlir/packages/sec-mlir-dialect_package13.md — §§33–39;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §114.
LogicalResult StructReplaceFieldOp::verify() {
  if (containsOwnedMutableSlice(getSource().getType()))
    return emitOpError("cannot copy a mutable slice through struct replacement");
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
