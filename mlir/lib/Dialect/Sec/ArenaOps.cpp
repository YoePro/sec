#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "OperationContracts.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Interfaces/SideEffectInterfaces.h"

using namespace mlir;
using namespace sec;

// Local verifiers of the schema-15 Arena and allocation-context operations.
// They mirror the Semantic IR contracts: fresh domain and initial epoch on
// creation, epoch-preserving allocation, epoch-advancing Reset, domain-ending
// Release/destroy, resolved backing/growth/provider/capacity/layout/default/
// dependency plans, and Arena-domain references. Cross-operation flow (use
// after Release, double Release, state order) is --sec-verify-arenas.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package19.md — sections 105-123, 126

namespace {
LogicalResult requireArenaSchema(Operation *op) {
  auto module = op->getParentOfType<ModuleOp>();
  auto version = module ? module->getAttrOfType<IntegerAttr>("sec.dialect_version") : IntegerAttr{};
  if (!version || version.getInt() < 15)
    return op->emitOpError("Arena and allocation-context operations require schema 15");
  return success();
}

bool nonZero(std::optional<uint32_t> value) { return value && *value != 0; }
bool present(std::optional<StringRef> value) { return value && !value->empty(); }

bool isAllocationErrorType(Type type) {
  auto error = dyn_cast<EnumType>(type);
  return error && error.getIdentity().getValue() == "core::AllocationError";
}

bool isArenaPlace(Type type) {
  auto place = dyn_cast<PlaceType>(type);
  return place && isa<ArenaType>(place.getElementType()) && place.getAuthority().getValue() == "rw";
}

bool isByte(Type type) {
  auto integer = dyn_cast<IntegerType>(type);
  return integer && integer.getWidth() == 8 && integer.isUnsigned();
}

enum class Shape { Create, Allocate, Reset, Terminal };

// verifyArenaFacts checks the plan facts and the state/epoch transition of
// one operation (sections 110-117, 121-122).
template <typename Op>
LogicalResult verifyArenaFacts(Op op, Shape shape) {
  if (failed(requireArenaSchema(op)))
    return failure();
  StringRef backing = op.getBackingKind(), growth = op.getGrowthKind();
  if ((backing != "borrowed" && backing != "owned" && backing != "none") || (growth != "fixed" && growth != "growable-nonrelocating"))
    return op.emitOpError("requires resolved backing (borrowed|owned|none) and growth (fixed|growable-nonrelocating) policies");
  auto relation = op.getRelation();
  if (backing == "borrowed" && (growth != "fixed" || !relation || *relation != "Borrowed" || !nonZero(op.getRetainedBorrow())))
    return op.emitOpError("borrowed backing is fixed, Borrowed and retains its incoming borrow");
  if (backing == "owned" && (!relation || *relation != "Owned" || !present(op.getProviderPlan())))
    return op.emitOpError("owned backing is Owned and names its resolved provider plan");
  // MD-136: the empty default owns its domain, borrows nothing and names its
  // frozen provider plan (absent, or the interim hosted mapping).
  if (backing == "none" && (!relation || *relation != "Owned" || nonZero(op.getRetainedBorrow()) || !present(op.getProviderPlan())))
    return op.emitOpError("an empty default Arena is Owned, retains no borrow and names its provider plan");
  if (!nonZero(op.getDomain()) || !present(op.getOrigin()) || !present(op.getAuthority()) ||
      !present(op.getStability()) || !present(op.getSpace()))
    return op.emitOpError("requires its Arena domain and canonical storage classification");
  if (failed(sec::detail::verifyAddressStability(op, *op.getStability())))
    return failure();
  auto stateBefore = op.getStateBefore();
  uint32_t stateAfter = op.getStateAfter();
  auto epochBefore = op.getEpochBefore(), epochAfter = op.getEpochAfter();
  StringRef transition = op.getTransition() ? *op.getTransition() : StringRef();
  switch (shape) {
  case Shape::Create:
    if (stateBefore || stateAfter == 0 || epochBefore || !nonZero(epochAfter) || transition != "establish-domain")
      return op.emitOpError("creation establishes a fresh domain, its first state version and initial epoch");
    break;
  case Shape::Allocate:
    if (!nonZero(stateBefore) || stateAfter == 0 || *stateBefore == stateAfter || !nonZero(epochBefore) ||
        epochBefore != epochAfter || transition != "none")
      return op.emitOpError("allocation advances the state version and preserves the validity epoch");
    break;
  case Shape::Reset:
    if (!nonZero(stateBefore) || stateAfter == 0 || *stateBefore == stateAfter || !nonZero(epochBefore) ||
        !nonZero(epochAfter) || epochBefore == epochAfter || transition != "advance-epoch")
      return op.emitOpError("Reset advances the state version and the validity epoch");
    break;
  case Shape::Terminal:
    if (!nonZero(stateBefore) || stateAfter == 0 || *stateBefore == stateAfter || !nonZero(epochBefore) ||
        epochAfter || transition != "end-domain")
      return op.emitOpError("Release ends the domain without publishing another epoch");
    break;
  }
  if ((shape == Shape::Reset || shape == Shape::Terminal) && !present(op.getDependencyProof()))
    return op.emitOpError("requires resolved dependency legality (no live strong dependency)");
  return success();
}

// verifyChecked requires the candidate/failed/AllocationError convention
// with atomic failure (sections 111-114).
template <typename Op>
LogicalResult verifyChecked(Op op) {
  if (!op.getFailureAtomic() || !isAllocationErrorType(op.getError().getType()))
    return op.emitOpError("produces candidate, failed and core::AllocationError with atomic failure");
  return success();
}

// verifyCandidateReference requires a dynamic-epoch reference fact of the
// Arena domain and its current epoch (sections 113-114, 122).
template <typename Op>
LogicalResult verifyCandidateReference(Op op) {
  auto facts = op->template getAttrOfType<DictionaryAttr>("sec.reference");
  auto integer = [&](StringRef key) -> int64_t {
    auto value = facts ? facts.template getAs<IntegerAttr>(key) : IntegerAttr{};
    return value ? value.getInt() : 0;
  };
  auto text = [&](StringRef key) -> StringRef {
    auto value = facts ? facts.template getAs<StringAttr>(key) : StringAttr{};
    return value ? value.getValue() : StringRef();
  };
  if (!facts || text("validity_policy") != "dynamic-epoch" || text("provenance") != "arena" ||
      integer("invalidation_domain") != int64_t(*op.getDomain()) ||
      !op.getEpochAfter() || integer("epoch_dependency") != int64_t(*op.getEpochAfter()))
    return op.emitOpError("candidate reference must retain its Arena domain and current epoch (sec.reference)");
  if (!present(op.getLayoutPlan()) || !present(op.getDefaultPlan()) || !present(op.getCapacityPlan()))
    return op.emitOpError("requires resolved layout, default and capacity plans for T");
  return success();
}
} // namespace

LogicalResult ArenaCreateDefaultOp::verify() {
  if (failed(verifyArenaFacts(*this, Shape::Create)))
    return failure();
  if (getBackingKind() != "none" || !getCapacityPlan() || *getCapacityPlan() != "empty-default")
    return emitOpError("creates an Arena with no backing and the empty-default capacity plan");
  if (!isa<ArenaType>(getArena().getType()))
    return emitOpError("produces one owned !sec.arena");
  return success();
}

LogicalResult ArenaCreateBorrowedOp::verify() {
  if (failed(verifyArenaFacts(*this, Shape::Create)))
    return failure();
  auto slice = dyn_cast<SliceMutType>(getBacking().getType());
  if (!slice || !isByte(slice.getElementType()) || getBackingKind() != "borrowed")
    return emitOpError("transfers one mutable byte-slice borrow into borrowed backing");
  if (!isa<ArenaType>(getArena().getType()))
    return emitOpError("produces one owned !sec.arena");
  if (nonZero(getParentStorage()) != nonZero(getParentDomain()) || nonZero(getParentDomain()) != nonZero(getParentEpoch()))
    return emitOpError("a nested child names parent storage, domain and epoch together");
  if (nonZero(getParentDomain()) && *getParentDomain() == *getDomain())
    return emitOpError("a nested child has a domain distinct from its parent");
  return success();
}

template <typename Op>
static LogicalResult verifyOwnedCreation(Op op, bool growable) {
  if (failed(verifyArenaFacts(op, Shape::Create)) || failed(verifyChecked(op)))
    return failure();
  if (!isa<UIntType>(op.getCapacity().getType()) || !isa<AllocContextType>(op.getContext().getType()) ||
      op.getBackingKind() != "owned" || !present(op.getCapacityPlan()))
    return op.emitOpError("takes a uint byte capacity, the selected !sec.alloc_context and a capacity plan");
  if (growable != (op.getGrowthKind() == "growable-nonrelocating"))
    return op.emitOpError("has an inconsistent growth policy");
  if (!isa<ArenaType>(op.getArena().getType()))
    return op.emitOpError("produces a candidate !sec.arena");
  return success();
}

LogicalResult ArenaCreateOwnedFixedOp::verify() { return verifyOwnedCreation(*this, false); }
LogicalResult ArenaCreateGrowableOp::verify() { return verifyOwnedCreation(*this, true); }

LogicalResult ArenaNewOp::verify() {
  if (failed(verifyArenaFacts(*this, Shape::Allocate)) || failed(verifyChecked(*this)))
    return failure();
  if (!isArenaPlace(getArena().getType()) || !isa<RefMutType>(getReference().getType()))
    return emitOpError("allocates through a writable Arena Place into one !sec.ref_mut<T>");
  return verifyCandidateReference(*this);
}

LogicalResult ArenaAllocOp::verify() {
  if (failed(verifyArenaFacts(*this, Shape::Allocate)) || failed(verifyChecked(*this)))
    return failure();
  if (!isArenaPlace(getArena().getType()) || !isa<UIntType>(getCount().getType()) || !isa<SliceMutType>(getSlice().getType()))
    return emitOpError("allocates uint count elements through a writable Arena Place into !sec.slice_mut<T>");
  return verifyCandidateReference(*this);
}

LogicalResult ArenaResetOp::verify() {
  if (failed(verifyArenaFacts(*this, Shape::Reset)))
    return failure();
  if (!isArenaPlace(getArena().getType()))
    return emitOpError("resets through a writable Arena Place");
  return success();
}

LogicalResult ArenaReleaseOp::verify() {
  if (failed(verifyArenaFacts(*this, Shape::Terminal)))
    return failure();
  if (!isa<ArenaType>(getArena().getType()))
    return emitOpError("consumes one owned !sec.arena");
  return success();
}

LogicalResult ArenaDestroyOp::verify() {
  if (failed(verifyArenaFacts(*this, Shape::Terminal)))
    return failure();
  if (!isa<ArenaType>(getArena().getType()) || getDestructionPlan().empty() || getCause().empty())
    return emitOpError("consumes one owned !sec.arena with its destruction plan and cause");
  return success();
}

template <typename Op>
static LogicalResult verifyContextProducer(Op op) {
  if (failed(requireArenaSchema(op)))
    return failure();
  if (!isa<AllocContextType>(op.getContext().getType()) || op.getOriginPlan().empty())
    return op.emitOpError("produces one !sec.alloc_context with its resolved origin plan");
  return success();
}

LogicalResult AllocContextFromArenaOp::verify() {
  if (failed(verifyContextProducer(*this)))
    return failure();
  if (!isArenaPlace(getArena().getType()) || getArenaDomain() == 0)
    return emitOpError("selects explicitly named writable Arena authority and its domain");
  return success();
}
LogicalResult AllocContextTargetOp::verify() { return verifyContextProducer(*this); }
LogicalResult AllocContextCompilerLocalOp::verify() { return verifyContextProducer(*this); }

// Section 123: the ordered Sec Arena resource. Standard Allocate/Free alone
// do not describe Reset, so Reset is a read and write of the resource.
namespace sec::detail {
// TypeID requires a named namespace for the resource singleton.
struct ArenaResource : public SideEffects::Resource::Base<ArenaResource> {
  StringRef getName() const final { return "<SecArena>"; }
};
} // namespace sec::detail

namespace {
using sec::detail::ArenaResource;

using EffectList = SmallVectorImpl<SideEffects::EffectInstance<MemoryEffects::Effect>>;

void arenaEffects(EffectList &effects, std::initializer_list<MemoryEffects::Effect *> kinds) {
  for (MemoryEffects::Effect *kind : kinds)
    effects.emplace_back(kind, ArenaResource::get());
}
} // namespace

void ArenaCreateDefaultOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Allocate::get(), MemoryEffects::Write::get()});
}
void ArenaCreateBorrowedOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Allocate::get(), MemoryEffects::Write::get()});
}
void ArenaCreateOwnedFixedOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Allocate::get(), MemoryEffects::Write::get()});
}
void ArenaCreateGrowableOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Allocate::get(), MemoryEffects::Write::get()});
}
void ArenaNewOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Read::get(), MemoryEffects::Write::get(), MemoryEffects::Allocate::get()});
}
void ArenaAllocOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Read::get(), MemoryEffects::Write::get(), MemoryEffects::Allocate::get()});
}
void ArenaResetOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Read::get(), MemoryEffects::Write::get()});
}
void ArenaReleaseOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Read::get(), MemoryEffects::Free::get()});
}
void ArenaDestroyOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Read::get(), MemoryEffects::Free::get()});
}
void AllocContextFromArenaOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Read::get()});
}
void AllocContextTargetOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Read::get()});
}
void AllocContextCompilerLocalOp::getEffects(EffectList &effects) {
  arenaEffects(effects, {MemoryEffects::Read::get()});
}
