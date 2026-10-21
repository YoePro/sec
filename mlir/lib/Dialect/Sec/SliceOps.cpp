#include "sec/Dialect/Sec/SecOps.h"
#include "OperationContracts.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Interfaces/DataLayoutInterfaces.h"
#include <string>
#include <utility>

using namespace mlir;
using namespace sec;
using namespace sec::detail;

namespace {
// requireSliceSchema gates every P16 operation, including scalar-only range
// checks whose operand types alone cannot expose an obsolete module schema.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 87.
LogicalResult requireSliceSchema(Operation *op) {
  auto module = op->getParentOfType<ModuleOp>();
  auto version = module
      ? module->getAttrOfType<IntegerAttr>("sec.dialect_version") : IntegerAttr{};
  if (!version || version.getInt() != 12)
    return op->emitOpError("slice operations require schema 12");
  return success();
}

// isCountType admits uint and its P6-resolved target-width unsigned carrier,
// never a signed result or an integer chosen from the element's width.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 47-49,112.
bool isCountType(Operation *op, Type type) {
  if (isa<UIntType>(type))
    return true;
  auto integer = dyn_cast<IntegerType>(type);
  auto module = op->getParentOfType<ModuleOp>();
  if (!integer || !integer.isUnsigned() || !module)
    return false;
  auto width = DataLayout(module).getTypeSizeInBits(
      IndexType::get(op->getContext()));
  return !width.isScalable() && integer.getWidth() == width.getFixedValue();
}

// canonicalNonnegative retains exact decimal provenance without a host-sized
// parse, sign, leading zero or precision cap.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v12.md — section 6.
bool canonicalNonnegative(StringRef value) {
  return !value.empty() && (value.size() == 1 || value.front() != '0') &&
         llvm::all_of(value, [](char c) { return c >= '0' && c <= '9'; });
}

// verifySliceProvenance validates optional symbolic/exact range metadata;
// dominance, origin binding and lifetime flow remain whole-function checks.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 25-30,91;
// rules/mlir/dialect-versions/sec_mlir_dialect_v12.md — section 6.
LogicalResult verifySliceProvenance(Operation *op) {
  StringAttr start, end;
  for (StringRef key : {"sec.slice_static_start", "sec.slice_static_end_exclusive"}) {
    if (auto value = op->getAttr(key)) {
      auto text = dyn_cast<StringAttr>(value);
      if (!text || !canonicalNonnegative(text.getValue()))
        return op->emitOpError("static slice bounds must be canonical non-negative decimal strings");
      (key == "sec.slice_static_start" ? start : end) = text;
    }
  }
  if (start && end) {
    StringRef a = start.getValue(), b = end.getValue();
    if (a.size() > b.size() || (a.size() == b.size() && a > b))
      return op->emitOpError("static slice start exceeds its exclusive end");
  }
  if (auto value = op->getAttr("sec.slice_range_id")) {
    auto id = dyn_cast<IntegerAttr>(value);
    if (!id || !id.getType().isInteger(32) || id.getValue().isZero())
      return op->emitOpError("slice_range_id must be a non-zero i32");
  }
  return success();
}

// sliceElement identifies bounded references only; arrays, raw pointers and
// direct element references cannot acquire slice operations implicitly.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 31-32,88-90.
Type sliceElement(Type type) {
  if (auto shared = dyn_cast<SliceType>(type)) return shared.getElementType();
  if (auto mut = dyn_cast<SliceMutType>(type)) return mut.getElementType();
  return {};
}

// verifyRange preserves the original present endpoint carriers/signedness and
// exact uint outputs. This validates the operation, not Sema's range proof.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 35,78-80,92-93.
template <typename Op> LogicalResult verifyRange(Op op) {
  if (failed(requireSliceSchema(op)) || failed(verifySliceProvenance(op)))
    return failure();
  if (!isCountType(op, op.getSourceLength().getType()) ||
      op.getStart().getType() != op.getSourceLength().getType() ||
      op.getLength().getType() != op.getSourceLength().getType())
    return op.emitOpError("source length and normalized outputs must have the same uint count type");
  if (op.getBounds().size() != unsigned(op.getStartPresent()) + unsigned(op.getEndPresent()))
    return op.emitOpError("bound presence must match the original endpoint operands");
  unsigned next = 0;
  for (auto [present, signedValue] : {std::pair{op.getStartPresent(), op.getStartSigned()}, std::pair{op.getEndPresent(), op.getEndSigned()}}) {
    if (!present) {
      if (signedValue)
        return op.emitOpError("an omitted endpoint has no signedness");
      continue;
    }
    Type type = op.getBounds()[next++].getType();
    if (!isBuiltinIntegerSemanticType(type) || isSignedIntegerSemanticType(type) != signedValue)
      return op.emitOpError("endpoint signedness must match its original semantic integer type");
  }
  return success();
}

// canonicalRangeError retains the P11 enum identity, declared cases and values,
// preserving P16's precedence classes without a parallel BoundsError type.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 18-19,93.
bool canonicalRangeError(Type type) {
  auto error = dyn_cast<EnumType>(type);
  if (!error || error.getIdentity().getValue() != "core::RangeError" || error.getCases().size() != 3)
    return false;
  const char *names[] = {"InvalidRange", "StartAfterEnd", "OutOfBounds"};
  for (auto [i, attr] : llvm::enumerate(error.getCases())) {
    auto entry = dyn_cast<EnumCaseAttr>(attr);
    if (!entry || entry.getOrdinal() != i || entry.getName().getValue() != names[i] || entry.getValue().getValue() != std::to_string(i))
      return false;
  }
  return true;
}

// verifyBorrow checks the contiguous source boundary, element/authority and
// canonical P15 identities. Actual narrowing/guard/lifetime relations are P43.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 91,94-97.
template <typename Op>
LogicalResult verifyBorrow(Op op, bool mutableResult, bool reborrow) {
  if (failed(requireSliceSchema(op)) || failed(verifyReferenceMetadata(op)) ||
      failed(verifySliceProvenance(op)))
    return failure();
  Type element;
  bool writable;
  if (reborrow) {
    element = sliceElement(op.getSource().getType());
    writable = isa<SliceMutType>(op.getSource().getType());
    if (!element)
      return op.emitOpError("reborrow source must be a bounded slice");
    if (!op.getSourceBorrow() || *op.getSourceBorrow() == 0 || *op.getSourceBorrow() == op.getBorrowId())
      return op.emitOpError("reborrow requires a distinct non-zero source_borrow");
  } else {
    auto place = dyn_cast<PlaceType>(op.getSource().getType());
    auto array = place ? dyn_cast<ArrayType>(place.getElementType()) : ArrayType{};
    if (!array)
      return op.emitOpError("slice borrow source must be a fixed-array Place");
    element = array.getElementType();
    writable = place.getAuthority().getValue() == "rw";
    if (op.getSourceBorrow())
      return op.emitOpError("a fresh slice borrow has no source_borrow");
  }
  Type result = op.getSlice().getType();
  if (sliceElement(result) != element ||
      isa<SliceMutType>(result) != mutableResult || (mutableResult && !writable))
    return op.emitOpError("slice borrow must preserve element type and cannot increase authority");
  if (!isCountType(op, op.getStart().getType()) || op.getStart().getType() != op.getLength().getType())
    return op.emitOpError("normalized start and length must have the same uint count type");
  return success();
}

// verifyTransfer preserves the bounded value's type/authority and P15 borrow
// identity; copying elements or manufacturing mutable copies is not admitted.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 98-99.
template <typename Op> LogicalResult verifyTransfer(Op op, bool mut) {
  if (failed(requireSliceSchema(op)) || failed(verifyReferenceMetadata(op)) ||
      failed(verifySliceProvenance(op)))
    return failure();
  Type source = op.getSource().getType();
  if (!sliceElement(source) || isa<SliceMutType>(source) != mut || op.getSlice().getType() != source)
    return op.emitOpError("holder transfer requires matching slice type and authority");
  if (!op.getSourceBorrow() || *op.getSourceBorrow() != op.getBorrowId())
    return op.emitOpError("holder transfer must preserve source_borrow");
  return success();
}

// verifyIndex keeps an original signed/unsigned integer beside its slice
// source; no element width or target count conversion narrows that index.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 52,102-103.
template <typename Op> LogicalResult verifyIndex(Op op) {
  if (failed(requireSliceSchema(op)) || failed(verifySliceProvenance(op)))
    return failure();
  if (!sliceElement(op.getSource().getType()))
    return op.emitOpError("index source must be a bounded slice");
  Type index = op.getIndex().getType();
  if (!isBuiltinIntegerSemanticType(index) || op.getIndexSigned() != isSignedIntegerSemanticType(index))
    return op.emitOpError("index_signed must match the original semantic integer type");
  return success();
}
} // namespace

// Verify proven versus checked normalization using the same local contract.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 92-93.
LogicalResult SliceRangeNormalizeOp::verify() {
  if (failed(verifyRange(*this)))
    return failure();
  if (!isOneOf(getProofKind(), {"constant", "range", "branch", "contract", "analysis", "source-open-bound"}))
    return emitOpError("range normalization requires a resolved proof_kind");
  if (getProofKind() == "source-open-bound" && (getStartPresent() || getEndPresent()))
    return emitOpError("source-open-bound proof requires both endpoints omitted");
  return success();
}
// Checked normalization is total and cannot carry an assumed safe proof.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 93.
LogicalResult SliceRangeCheckOp::verify() {
  if (failed(verifyRange(*this)))
    return failure();
  if ((*this)->getAttr("proof_kind"))
    return emitOpError("checked range cannot carry an assumed proof_kind");
  if (!canonicalRangeError(getError().getType()))
    return emitOpError("checked range requires canonical core::RangeError");
  return success();
}
// Require a compatible fixed-array Place and shared bounded result.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 94.
LogicalResult SliceBorrowSharedOp::verify() {
  return verifyBorrow(*this, false, false);
}
// Require a writable fixed-array Place and exclusive bounded result.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 95.
LogicalResult SliceBorrowMutOp::verify() {
  return verifyBorrow(*this, true, false);
}
// Shared reborrow cannot change elements or increase source authority.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 96.
LogicalResult SliceReborrowSharedOp::verify() {
  return verifyBorrow(*this, false, true);
}
// Exclusive reborrow requires an already exclusive source.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 97.
LogicalResult SliceReborrowMutOp::verify() {
  return verifyBorrow(*this, true, true);
}
// Shared holder copying has no element ownership operation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 98.
LogicalResult SliceCopySharedOp::verify() {
  return verifyTransfer(*this, false);
}
// An exclusive holder transfers only with its existing type/borrow identity.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 99.
LogicalResult SliceMoveMutOp::verify() {
  return verifyTransfer(*this, true);
}
// The only slice length observation is uint, before or after P6 resolution.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — sections 100-101,112.
LogicalResult SliceLenOp::verify() {
  if (failed(requireSliceSchema(*this)) || failed(verifySliceProvenance(*this)))
    return failure();
  if (!sliceElement(getSource().getType()) || !isCountType(*this, getLength().getType()))
    return emitOpError("slice length requires a bounded slice and uint count result");
  return success();
}
// The bounds predicate does not read elements or compute physical addresses.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 102.
LogicalResult SliceIndexInBoundsOp::verify() {
  return verifyIndex(*this);
}
// Element Places preserve exact element type and source read/write authority.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — section 103.
LogicalResult SliceElementPlaceOp::verify() {
  if (failed(verifyIndex(*this)) || failed(verifyReferenceMetadata(*this)) ||
      failed(verifyArrayBoundsMetadata(*this, getBoundsKind(), getBoundsProof())))
    return failure();
  auto place = dyn_cast<PlaceType>(getPlace().getType());
  bool mut = isa<SliceMutType>(getSource().getType());
  if (!place || place.getElementType() != sliceElement(getSource().getType()) ||
      (place.getAuthority().getValue() == "rw") != mut)
    return emitOpError("element Place must preserve slice element type and authority");
  return success();
}
