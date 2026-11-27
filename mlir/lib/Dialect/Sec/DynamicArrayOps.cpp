#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "OperationContracts.h"
#include "mlir/IR/BuiltinOps.h"
#include "llvm/ADT/STLExtras.h"

using namespace mlir;
using namespace sec;

// Local verifiers of the schema-14 owning dynamic-array and storage
// transition operations. Whole-function protocol relations (a growth tested
// immediately on failed, the transition opening its success edge, append
// only after ensured capacity) are checked by --sec-verify-dynamic-arrays.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — sections 21-31, 58-67, 81-96

namespace {
// requireDynamicArraySchema gates every P18 operation (section 81).
LogicalResult requireDynamicArraySchema(Operation *op) {
  auto module = op->getParentOfType<ModuleOp>();
  auto version = module
      ? module->getAttrOfType<IntegerAttr>("sec.dialect_version") : IntegerAttr{};
  if (!version || version.getInt() < 14)
    return op->emitOpError("dynamic-array and storage-transition operations require schema 14");
  return success();
}

// ownerArray requires the owner Place of a dynamic array, writable when the
// operation changes the owner; the owner is never moved or copied.
FailureOr<DynamicArrayType> ownerArray(Operation *op, Value owner, bool writable) {
  auto place = dyn_cast<PlaceType>(owner.getType());
  if (!place)
    return op->emitOpError("addresses the dynamic-array owner through a Place");
  auto array = dyn_cast<DynamicArrayType>(place.getElementType());
  if (!array)
    return op->emitOpError("requires a Place of a dynamic array");
  if (writable && place.getAuthority().getValue() != "rw")
    return op->emitOpError("changes the owner and requires a writable Place");
  return array;
}

// positiveCount admits a canonical positive decimal element count.
bool positiveCount(StringRef value) {
  return !value.empty() && value.front() != '0' &&
         llvm::all_of(value, [](char c) { return c >= '0' && c <= '9'; });
}

bool isAllocationError(Type type) {
  auto error = dyn_cast<EnumType>(type);
  return error && error.getIdentity().getValue() == "core::AllocationError";
}

bool isUnsignedCount(Type type) {
  if (isa<UIntType>(type))
    return true;
  auto integer = dyn_cast<IntegerType>(type);
  return integer && integer.isUnsigned();
}

bool isSignedCount(Type type) {
  if (isa<IntType>(type))
    return true;
  auto integer = dyn_cast<IntegerType>(type);
  return integer && integer.isSigned();
}

bool isIndexType(Type type) {
  return isa<IntType, UIntType>(type) || isa<IntegerType>(type);
}

bool nonZero(std::optional<uint32_t> value) { return value && *value != 0; }

// verifyEstablishment checks the complete establishment facts (section 59)
// and the reclamation pairing (section 31): owned backing, a known
// reclamation authority, stability and memory space, and a reclamation plan
// exactly for owning-value authority.
template <typename Op>
LogicalResult verifyEstablishment(Op op) {
  auto origin = op.getOrigin(), relation = op.getRelation(), authority = op.getAuthority();
  auto stability = op.getStability(), space = op.getSpace();
  if (!origin || origin->empty() || !relation || *relation != "Owned" || !authority ||
      authority->empty() || *authority == "Unknown" || !stability || !space || space->empty())
    return op.emitOpError("requires origin, owned backing relation, known reclamation authority, stability and memory space");
  if (failed(sec::detail::verifyAddressStability(op, *stability)))
    return failure();
  if (nonZero(op.getReclamation()) != (*authority == "OwningValue"))
    return op.emitOpError("pairs a reclamation plan only with owning-value authority");
  return success();
}

// verifyAllocationFacts requires the backing domain, allocation identity and
// allocation context of an allocating operation.
template <typename Op>
LogicalResult verifyAllocationFacts(Op op) {
  if (!nonZero(op.getDomain()) || !nonZero(op.getAllocation()) || !nonZero(op.getAllocationContext()))
    return op.emitOpError("requires its backing domain, allocation identity and allocation context");
  return success();
}
} // namespace

LogicalResult DynamicArrayEmptyOp::verify() {
  if (failed(requireDynamicArraySchema(*this)))
    return failure();
  if (!isa<DynamicArrayType>(getResult().getType()))
    return emitOpError("produces an owning dynamic array");
  return success();
}

LogicalResult DynamicArrayAllocateOp::verify() {
  if (failed(requireDynamicArraySchema(*this)))
    return failure();
  if (!isa<DynamicArrayType>(getArray().getType()) || !isAllocationError(getError().getType()))
    return emitOpError("produces a candidate dynamic array, failed and core::AllocationError");
  if (!positiveCount(getCount()))
    return emitOpError("requires a positive canonical minimum capacity");
  if (failed(verifyAllocationFacts(*this)))
    return failure();
  if (getTransition() != StringRef("establish-domain"))
    return emitOpError("establishes its backing domain (transition establish-domain)");
  return verifyEstablishment(*this);
}

static LogicalResult verifyObservation(Operation *op, Value owner, Type result,
                                       bool (*resultKind)(Type), StringRef expected) {
  if (failed(requireDynamicArraySchema(op)) || failed(ownerArray(op, owner, false)))
    return failure();
  if (!resultKind(result))
    return op->emitOpError("produces ") << expected;
  return success();
}

LogicalResult DynamicArrayLenOp::verify() {
  return verifyObservation(*this, getOwner(), getResult().getType(), isUnsignedCount, "the uint length");
}

LogicalResult DynamicArrayLenIntOp::verify() {
  return verifyObservation(*this, getOwner(), getResult().getType(), isSignedCount, "the int length");
}

LogicalResult DynamicArrayCapacityInternalOp::verify() {
  return verifyObservation(*this, getOwner(), getResult().getType(), isUnsignedCount, "the uint capacity");
}

LogicalResult DynamicArrayHasBackingOp::verify() {
  return verifyObservation(*this, getOwner(), getResult().getType(),
                           [](Type type) { return type.isInteger(1); }, "i1");
}

LogicalResult DynamicArrayHasCapacityOp::verify() {
  if (failed(requireDynamicArraySchema(*this)) || failed(ownerArray(*this, getOwner(), false)))
    return failure();
  if (!positiveCount(getCount()))
    return emitOpError("requires a positive canonical element count");
  return success();
}

LogicalResult DynamicArrayIndexInBoundsOp::verify() {
  if (failed(requireDynamicArraySchema(*this)) || failed(ownerArray(*this, getOwner(), false)))
    return failure();
  if (!isIndexType(getIndex().getType()))
    return emitOpError("requires a built-in integer index");
  return success();
}

LogicalResult DynamicArrayElementPlaceOp::verify() {
  if (failed(requireDynamicArraySchema(*this)))
    return failure();
  auto array = ownerArray(*this, getOwner(), false);
  if (failed(array))
    return failure();
  if (!isIndexType(getIndex().getType()))
    return emitOpError("requires a built-in integer index");
  auto owner = cast<PlaceType>(getOwner().getType());
  auto place = dyn_cast<PlaceType>(getPlace().getType());
  if (!place || place.getElementType() != array->getElementType() ||
      place.getAuthority() != owner.getAuthority())
    return emitOpError("yields an element Place with the owner's authority");
  // Section 41: a runtime length admits the guarded check or a frontend
  // branch/analysis proof of `index < Len`; never a constant or range proof.
  bool guarded = getBoundsKind() == "runtime-check" && getBoundsProof() == "guarded";
  bool proven = getBoundsKind() == "proven-safe" &&
                (getBoundsProof() == "branch" || getBoundsProof() == "analysis");
  if (!guarded && !proven)
    return emitOpError("requires its runtime bounds guard or a branch/analysis length proof");
  if (getBackingDomain() == 0)
    return emitOpError("must carry the owner's backing-domain dependency");
  return success();
}

LogicalResult DynamicArrayAppendPreparedOp::verify() {
  if (failed(requireDynamicArraySchema(*this)))
    return failure();
  auto array = ownerArray(*this, getOwner(), true);
  if (failed(array))
    return failure();
  if (getElement().getType() != array->getElementType())
    return emitOpError("transfers one element of the array's element type");
  return success();
}

LogicalResult DynamicArrayPayloadSizeOp::verify() {
  if (failed(requireDynamicArraySchema(*this)))
    return failure();
  if (failed(ownerArray(*this, getOwner(), false)))
    return failure();
  if (getStride() != "0" && !positiveCount(getStride()))
    return emitOpError("requires a canonical non-negative stride");
  if (!isa<UIntType>(getSize().getType()))
    return emitOpError("produces the target-sized uint payload size");
  return success();
}

LogicalResult DynamicArrayClearOp::verify() {
  if (failed(requireDynamicArraySchema(*this)))
    return failure();
  if (failed(ownerArray(*this, getOwner(), true)))
    return failure();
  return success();
}

LogicalResult DynamicArrayRemoveAtOp::verify() {
  if (failed(requireDynamicArraySchema(*this)))
    return failure();
  auto array = ownerArray(*this, getOwner(), true);
  if (failed(array))
    return failure();
  if (!isIndexType(getIndex().getType()))
    return emitOpError("requires a built-in integer index");
  if (getElement().getType() != array->getElementType())
    return emitOpError("yields one element of the array's element type");
  // Section 13C: only the runtime index_in_bounds guard admits extraction.
  if (getBoundsKind() != "runtime-check" || getBoundsProof() != "guarded")
    return emitOpError("requires its runtime index_in_bounds guard");
  return success();
}

LogicalResult DynamicArrayAppendRangePreparedOp::verify() {
  if (failed(requireDynamicArraySchema(*this)))
    return failure();
  auto array = ownerArray(*this, getOwner(), true);
  if (failed(array))
    return failure();
  Type element = array->getElementType();
  if (getLower().getType() != element)
    return emitOpError("takes a lower bound of the array's element type");
  if (!isIndexType(element) && !isa<RuneType>(element))
    return emitOpError("requires a built-in integer or rune element");
  if (!positiveCount(getCount()))
    return emitOpError("requires a positive canonical value count");
  return success();
}

LogicalResult DynamicArrayGrowBackingOp::verify() {
  if (failed(requireDynamicArraySchema(*this)) || failed(ownerArray(*this, getOwner(), true)))
    return failure();
  if (!isAllocationError(getError().getType()))
    return emitOpError("produces failed and core::AllocationError");
  if (!positiveCount(getCount()))
    return emitOpError("requires a positive canonical element count");
  if (failed(verifyAllocationFacts(*this)))
    return failure();
  auto transition = getTransition();
  if (!transition || (*transition != "establish-domain" && *transition != "advance-epoch" &&
                      *transition != "replace-domain"))
    return emitOpError("transition must be establish-domain, advance-epoch or replace-domain");
  return verifyEstablishment(*this);
}

template <typename Op>
static LogicalResult verifyTransition(Op op) {
  if (failed(requireDynamicArraySchema(op)) || failed(ownerArray(op, op.getOwner(), true)))
    return failure();
  if (!nonZero(op.getDomain()))
    return op.emitOpError("requires its invalidation domain");
  return success();
}

LogicalResult StorageEstablishDomainOp::verify() {
  if (failed(verifyTransition(*this)))
    return failure();
  if (getTransition() != StringRef("establish-domain"))
    return emitOpError("applies an establish-domain transition");
  return verifyEstablishment(*this);
}

LogicalResult StorageAdvanceEpochOp::verify() {
  if (failed(verifyTransition(*this)))
    return failure();
  if (getTransition() != StringRef("advance-epoch"))
    return emitOpError("applies an advance-epoch transition");
  return success();
}

LogicalResult StorageEndDomainOp::verify() { return verifyTransition(*this); }

LogicalResult StorageReclaimOp::verify() {
  if (failed(verifyTransition(*this)))
    return failure();
  if (!nonZero(getReclamation()) || getAuthority() != StringRef("OwningValue"))
    return emitOpError("consumes a resolved owning-value reclamation plan");
  return success();
}
