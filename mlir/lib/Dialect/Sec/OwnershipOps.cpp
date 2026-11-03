#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/BuiltinOps.h"
#include "llvm/ADT/StringSet.h"

using namespace mlir;
using namespace sec;

namespace {
// requireOwnershipSchema gates every v13 operation; older modules keep their
// predecessor meaning. Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v13.md — section 1.
LogicalResult requireOwnershipSchema(Operation *op) {
  auto module = op->getParentOfType<ModuleOp>();
  auto version = module
      ? module->getAttrOfType<IntegerAttr>("sec.dialect_version") : IntegerAttr{};
  if (!version || version.getInt() < 13)
    return op->emitOpError("ownership and cleanup operations require schema 13");
  return success();
}

// placeOf returns the operand's Place, requiring its element and any authority.
LogicalResult placeOf(Operation *op, Value place, Type element, bool writable) {
  auto type = dyn_cast<PlaceType>(place.getType());
  if (!type)
    return op->emitOpError("requires a Place operand");
  if (element && type.getElementType() != element)
    return op->emitOpError("requires a Place of the value's type");
  if (writable && type.getAuthority().getValue() != "rw")
    return op->emitOpError("requires a writable Place");
  return success();
}

bool isBorrowType(Type type) {
  return isa<RefType, RefMutType, SliceType, SliceMutType>(type);
}

bool oneOf(StringRef value, std::initializer_list<StringRef> allowed) {
  return llvm::is_contained(allowed, value);
}

bool isDestroyCause(StringRef cause) {
  return oneOf(cause, {"scope-exit", "function-exit", "temporary-end", "cleanup", "compiler-generated"});
}

// verifyExitKind admits the dialect's exit kinds; a function exit is only a
// language-controlled one, and panic never runs cleanup (v13 sections 22, 35).
LogicalResult verifyExitKind(Operation *op, StringRef kind, bool function) {
  if (oneOf(kind, {"fallthrough", "return", "error-propagation"}))
    return success();
  if (!function && oneOf(kind, {"break", "continue", "match-branch-exit", "switch-branch-exit"}))
    return success();
  return op->emitOpError("has an invalid exit kind '") << kind << "'";
}
} // namespace

LogicalResult OwnCopyOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  if (getSource().getType() != getResult().getType())
    return emitOpError("copy preserves the value type");
  auto type = getSource().getType();
  if (isa<RefMutType, SliceMutType>(type))
    return emitOpError("a mutable reference is never copied");
  return success();
}

LogicalResult OwnSemanticCopyOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  if (getSource().getType() != getResult().getType())
    return emitOpError("copy preserves the value type");
  if (isBorrowType(getSource().getType()))
    return emitOpError("a reference has no semantic copy");
  if (getSemanticCopyPlan().empty())
    return emitOpError("requires a resolved semantic-copy plan");
  return success();
}

LogicalResult OwnMoveOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  if (getSource().getType() != getResult().getType())
    return emitOpError("move preserves the value type");
  return success();
}

LogicalResult OwnCopyFromPlaceOp::verify() {
  if (failed(requireOwnershipSchema(*this)) ||
      failed(placeOf(*this, getPlace(), getResult().getType(), false)))
    return failure();
  StringRef mode = getCopyMode();
  if (mode != "trivial" && mode != "semantic-infallible")
    return emitOpError("has an invalid copy mode '") << mode << "'";
  if ((mode == "semantic-infallible") != getSemanticCopyPlan().has_value())
    return emitOpError("a semantic copy, and only it, names a semantic-copy plan");
  return success();
}

LogicalResult OwnMoveFromPlaceOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  return placeOf(*this, getPlace(), getResult().getType(), true);
}

LogicalResult OwnInitializePlaceOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  return placeOf(*this, getPlace(), getValue().getType(), true);
}

LogicalResult OwnReplacePlaceOp::verify() {
  if (failed(requireOwnershipSchema(*this)) ||
      failed(placeOf(*this, getPlace(), getValue().getType(), true)))
    return failure();
  if (getDestructionPlan().empty())
    return emitOpError("requires the old value's destruction plan");
  // A replaced stored field stays under its owner's registration (package17 section 64).
  bool field = getPlace().getDefiningOp<PlaceFieldOp>() != nullptr;
  if (field && getOldCleanupActionId() == 0 && getNewCleanupActionId() == 0)
    return success();
  if (getOldCleanupActionId() == 0 || getNewCleanupActionId() == 0 ||
      getOldCleanupActionId() == getNewCleanupActionId())
    return emitOpError("requires distinct old and new cleanup action identities");
  return success();
}

LogicalResult OwnDiscardOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  StringRef origin = getDiscardOrigin();
  if (!oneOf(origin, {"explicit", "implicit-call-result", "compiler-temporary"}))
    return emitOpError("has an invalid discard origin '") << origin << "'";
  if (isBorrowType(getValue().getType()) && getDestructionPlan())
    return emitOpError("discarding a reference never destroys its referent");
  return success();
}

LogicalResult OwnDestroyValueOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  if (isBorrowType(getValue().getType()))
    return emitOpError("a reference is ended, never destroyed");
  if (getDestructionPlan().empty() || !isDestroyCause(getCause()))
    return emitOpError("requires a destruction plan and a valid cause");
  return success();
}

LogicalResult OwnDestroyPlaceOp::verify() {
  if (failed(requireOwnershipSchema(*this)) ||
      failed(placeOf(*this, getPlace(), Type{}, true)))
    return failure();
  if (isBorrowType(cast<PlaceType>(getPlace().getType()).getElementType()))
    return emitOpError("a reference is ended, never destroyed");
  if (getDestructionPlan().empty() || !isDestroyCause(getCause()))
    return emitOpError("requires a destruction plan and a valid cause");
  return success();
}

LogicalResult CleanupTrackOwnedOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  Type tracked = getTracked().getType();
  if (auto place = dyn_cast<PlaceType>(tracked))
    tracked = place.getElementType();
  if (isBorrowType(tracked))
    return emitOpError("a borrow holder is not owned");
  if (getCleanupActionId() == 0 || getCleanupScopeId() == 0 || getCleanupRegistrationOrdinal() == 0)
    return emitOpError("requires action, scope and ordinal identities");
  if (getDestructionPlan().empty())
    return emitOpError("requires a destruction plan");
  return success();
}

LogicalResult CleanupCancelOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  if (getCleanupActionId() == 0 || !oneOf(getReason(), {"moved", "replaced", "transferred", "discarded"}))
    return emitOpError("requires an action identity and a valid reason");
  return success();
}

// A defer body is isolated: its entry arguments are exactly the capture Places, it
// reaches no outer SSA value and yields to cleanup execution (v13 sections 17-19).
LogicalResult CleanupDeferRegisterOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  if (getCleanupActionId() == 0 || getCleanupScopeId() == 0 ||
      getCleanupRegistrationOrdinal() == 0 || getDeferId() == 0)
    return emitOpError("requires action, scope, ordinal and defer identities");
  for (Value capture : getCaptures())
    if (!isa<PlaceType>(capture.getType()))
      return emitOpError("captures only Places");
  Region &body = getBody();
  if (body.empty())
    return emitOpError("requires a body region");
  if (!llvm::equal(body.front().getArgumentTypes(), getCaptures().getTypes()))
    return emitOpError("body arguments must match the capture types exactly");
  for (Block &block : body) {
    Operation *last = block.empty() ? nullptr : &block.back();
    if (!last || (last->getNumSuccessors() == 0 && !isa<CleanupDeferYieldOp>(last) &&
                  !last->hasTrait<OpTrait::IsTerminator>()))
      return emitOpError("every body path ends with sec.cleanup.defer_yield");
    WalkResult walked = block.walk([&](Operation *inner) {
      if (isa<func::ReturnOp>(inner))
        return WalkResult::interrupt();
      for (Value operand : inner->getOperands())
        if (!body.isAncestor(operand.getParentRegion()))
          return WalkResult::interrupt();
      return WalkResult::advance();
    });
    if (walked.wasInterrupted())
      return emitOpError("a defer body neither returns nor uses an outer value");
  }
  return success();
}

LogicalResult CleanupDeferYieldOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  if (!(*this)->getParentOfType<CleanupDeferRegisterOp>())
    return emitOpError("must be inside a defer body");
  return success();
}

LogicalResult CleanupRunScopeOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  if (getScopeIds().empty() || llvm::is_contained(getScopeIds(), 0))
    return emitOpError("requires exited scope identities");
  return verifyExitKind(*this, getExitKind(), false);
}

LogicalResult CleanupRunFunctionOp::verify() {
  if (failed(requireOwnershipSchema(*this)))
    return failure();
  return verifyExitKind(*this, getExitKind(), true);
}
