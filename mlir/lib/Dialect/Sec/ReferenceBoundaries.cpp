// Local operation contracts shared by direct references and bounded slices.
// Guard dominance and ownership/borrow-flow verification remain in analysis
// passes; this file selects no physical layout, epoch representation or trap.
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "mlir/IR/BuiltinOps.h"

using namespace mlir;
using namespace sec;

// supportsSliceValues confines P16 operation extensions to schema 12 inputs.
// Unknown versions are rejected by the dialect's module verifier.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md section 87;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md section 1.
static bool supportsSliceValues(Operation *operation) {
  if (auto module = operation->getParentOfType<ModuleOp>())
    if (auto version = module->getAttrOfType<IntegerAttr>("sec.dialect_version"))
      return version.getInt() >= 12;
  return false;
}

// FailBoundsOp::verify retains the single panic.bounds endpoint while
// distinguishing fixed-array indexing from slice range/index failure. Slice
// provenance is a schema-12 contract, not a new trap or panic category.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md section 104;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md section 23.
LogicalResult FailBoundsOp::verify() {
  auto operation = (*this)->getAttrOfType<StringAttr>("operation");
  if (!operation || (operation.getValue() != "fixed-array-index" &&
                     operation.getValue() != "slice-index" &&
                     operation.getValue() != "slice-range"))
    return emitOpError("operation must be fixed-array-index, slice-index or slice-range");
  if (operation.getValue() != "fixed-array-index" && !supportsSliceValues(*this))
    return emitOpError("slice bounds provenance requires schema 12");
  return success();
}

// verifyReferenceValueOperand admits slices only for the two canonical
// reference-value operations extended by P16. It deliberately does not extend
// referentOf: a slice is not a direct reference to one element, and cannot
// acquire ref.compare, dereference or ordinary reborrow semantics implicitly.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v12.md sections 21-22;
// rules/mlir/packages/sec-mlir-dialect_package16.md sections 44,63,70,84.
static LogicalResult verifyReferenceValueOperand(Operation *operation,
                                                 Value reference) {
  Type type = reference.getType();
  if (isa<SliceType, SliceMutType>(type)) {
    if (!supportsSliceValues(operation))
      return operation->emitOpError("slice reference operations require schema 12");
    return success();
  }
  if (!isa<RefType, RefMutType>(type))
    return operation->emitOpError("operand must be !sec.ref or !sec.ref_mut, or a schema-12 !sec.slice or !sec.slice_mut");
  return success();
}

// RefIsValidOp::verify accepts the complete bounded reference value without
// observing its elements or inventing a slice-specific generation system.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v12.md section 21.
LogicalResult RefIsValidOp::verify() {
  return verifyReferenceValueOperand(*this, getReference());
}

// RefEndBorrowOp::verify ends the named holder obligation without destroying
// elements, releasing backing storage or resetting an arena. Identity/use
// matching remains the separate borrow-semantics verifier's responsibility.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md section 63;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md section 22.
LogicalResult RefEndBorrowOp::verify() {
  if (failed(verifyReferenceValueOperand(*this, getReference())))
    return failure();
  if (getBorrowId() == 0 || getLifetimeId() == 0)
    return emitOpError("borrow_id and lifetime_id must be non-zero");
  return success();
}

