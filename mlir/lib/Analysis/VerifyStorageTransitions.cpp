// Storage-domain lifecycle and dynamic-array growth invariants that are
// provable within one function. Runtime facts (whether an owner entering the
// function has backing, its length and capacity) stay the operations'
// guarded protocol; this pass never assumes a state it cannot see.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — sections 30-31, 37, 58-67, 129-130.
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/Dominance.h"
#include "mlir/Pass/Pass.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/DenseMap.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYSTORAGETRANSITIONS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {
// ownerKey identifies the owner a Place addresses: its storage or root value,
// so separate place.storage projections of one owner compare equal.
Value ownerKey(Value place) {
  Operation *definition = place.getDefiningOp();
  if (auto storage = dyn_cast_or_null<sec::PlaceStorageOp>(definition))
    return storage.getStorage();
  if (auto value = dyn_cast_or_null<sec::PlaceValueOp>(definition))
    return value.getValue();
  return place;
}

// ownerOf returns the owner Place operand of a P18 operation, or null.
Value ownerOf(Operation *op) {
  if (isa<sec::DynamicArrayLenOp, sec::DynamicArrayLenIntOp, sec::DynamicArrayCapacityInternalOp,
          sec::DynamicArrayHasBackingOp, sec::DynamicArrayHasCapacityOp, sec::DynamicArrayIndexInBoundsOp,
          sec::DynamicArrayElementPlaceOp, sec::DynamicArrayAppendPreparedOp, sec::DynamicArrayGrowBackingOp,
          sec::StorageEstablishDomainOp, sec::StorageAdvanceEpochOp, sec::StorageEndDomainOp,
          sec::StorageReclaimOp>(op))
    return op->getOperand(0);
  if (isa<sec::SliceBorrowSharedOp, sec::SliceBorrowMutOp>(op)) {
    auto place = dyn_cast<sec::PlaceType>(op->getOperand(0).getType());
    if (place && isa<sec::DynamicArrayType>(place.getElementType()))
      return op->getOperand(0);
  }
  return {};
}

std::optional<uint32_t> domainOf(Operation *op) {
  if (auto element = dyn_cast<sec::DynamicArrayElementPlaceOp>(op))
    return element.getBackingDomain();
  if (auto grow = dyn_cast<sec::DynamicArrayGrowBackingOp>(op)) return grow.getDomain();
  if (auto establish = dyn_cast<sec::StorageEstablishDomainOp>(op)) return establish.getDomain();
  if (auto advance = dyn_cast<sec::StorageAdvanceEpochOp>(op)) return advance.getDomain();
  if (auto end = dyn_cast<sec::StorageEndDomainOp>(op)) return end.getDomain();
  if (auto reclaim = dyn_cast<sec::StorageReclaimOp>(op)) return reclaim.getDomain();
  return std::nullopt;
}

struct Establishment {
  Operation *op = nullptr;
  StringRef authority;
  uint32_t reclamation = 0;
};

template <typename Op>
Establishment establishmentOf(Op op) {
  return {op.getOperation(), op.getAuthority().value_or(""), op.getReclamation().value_or(0)};
}

LogicalResult verifyFunction(func::FuncOp function) {
  DominanceInfo dominance(function);
  llvm::DenseMap<Value, uint32_t> ownerDomain;
  llvm::DenseMap<uint32_t, Establishment> established;
  SmallVector<Operation *> ends, grows, holders;
  bool replacesDomain = false;
  function.walk([&](sec::DynamicArrayGrowBackingOp grow) {
    replacesDomain |= grow.getTransition() == StringRef("replace-domain");
  });

  WalkResult walk = function.walk([&](Operation *op) -> WalkResult {
    Value owner = ownerOf(op);
    if (!owner)
      return WalkResult::advance();
    Value key = ownerKey(owner);
    // One owner names one backing domain in a function, unless a growth
    // replaces the domain (section 30).
    if (auto domain = domainOf(op); domain && *domain != 0 && !replacesDomain) {
      auto [entry, inserted] = ownerDomain.try_emplace(key, *domain);
      if (!inserted && entry->second != *domain)
        return op->emitOpError("names backing domain #") << *domain << " but its owner uses #" << entry->second,
               WalkResult::interrupt();
    }
    // Establishment facts of one domain agree (sections 31, 59).
    Establishment facts;
    if (auto grow = dyn_cast<sec::DynamicArrayGrowBackingOp>(op)) facts = establishmentOf(grow);
    if (auto establish = dyn_cast<sec::StorageEstablishDomainOp>(op)) facts = establishmentOf(establish);
    if (facts.op) {
      uint32_t domain = domainOf(op).value_or(0);
      auto [entry, inserted] = established.try_emplace(domain, facts);
      if (!inserted && (entry->second.authority != facts.authority || entry->second.reclamation != facts.reclamation))
        return op->emitOpError("disagrees with the reclamation authority or plan of its domain"), WalkResult::interrupt();
    }
    if (isa<sec::StorageEndDomainOp>(op))
      ends.push_back(op);
    if (isa<sec::DynamicArrayGrowBackingOp>(op))
      grows.push_back(op);
    if (isa<sec::DynamicArrayElementPlaceOp, sec::SliceBorrowSharedOp, sec::SliceBorrowMutOp>(op))
      holders.push_back(op);

    // Reclamation directly follows the domain end or the epoch advance that
    // relocated the backing, for the same owner and domain (sections 30-31, 62).
    if (auto reclaim = dyn_cast<sec::StorageReclaimOp>(op)) {
      Operation *previous = op->getPrevNode();
      bool paired = previous && isa<sec::StorageEndDomainOp, sec::StorageAdvanceEpochOp>(previous) &&
                    ownerKey(previous->getOperand(0)) == key && domainOf(previous) == reclaim.getDomain();
      if (!paired)
        return op->emitOpError("must directly follow the end_domain or advance_epoch of the same owner and domain"),
               WalkResult::interrupt();
    }
    // AdvanceEpoch grows only a live domain: its has_capacity test sits on
    // the true edge of a has_backing test of the same owner (section 61).
    if (auto grow = dyn_cast<sec::DynamicArrayGrowBackingOp>(op);
        grow && grow.getTransition() == StringRef("advance-epoch")) {
      Block *capacityBlock = op->getBlock()->getSinglePredecessor();
      Block *backingBlock = capacityBlock ? capacityBlock->getSinglePredecessor() : nullptr;
      auto branch = backingBlock ? dyn_cast<cf::CondBranchOp>(backingBlock->getTerminator()) : cf::CondBranchOp();
      auto test = branch ? branch.getCondition().getDefiningOp<sec::DynamicArrayHasBackingOp>() : sec::DynamicArrayHasBackingOp();
      if (!test || branch.getTrueDest() != capacityBlock || ownerKey(test.getOwner()) != key)
        return op->emitOpError("advances an epoch only after a true has_backing test of the same owner"),
               WalkResult::interrupt();
    }
    // A failed growth leaves the owner unchanged: its failure edge applies no
    // storage transition (section 37).
    if (auto grow = dyn_cast<sec::DynamicArrayGrowBackingOp>(op)) {
      auto branch = dyn_cast_or_null<cf::CondBranchOp>(op->getNextNode());
      if (branch && !branch.getTrueDest()->empty()) {
        Operation &first = branch.getTrueDest()->front();
        if (isa<sec::StorageEstablishDomainOp, sec::StorageAdvanceEpochOp, sec::StorageReclaimOp>(first) &&
            ownerKey(first.getOperand(0)) == key)
          return op->emitOpError("failure edge applies a storage transition; a failed growth leaves the owner unchanged"),
                 WalkResult::interrupt();
      }
    }
    return WalkResult::advance();
  });
  if (walk.wasInterrupted())
    return failure();

  // No dependent access after the domain ends (section 130), except the
  // paired reclamation.
  for (Operation *end : ends) {
    Value key = ownerKey(end->getOperand(0));
    WalkResult after = function.walk([&](Operation *op) -> WalkResult {
      Value owner = ownerOf(op);
      if (!owner || op == end || ownerKey(owner) != key || !dominance.properlyDominates(end, op))
        return WalkResult::advance();
      if (isa<sec::StorageReclaimOp>(op) && op->getPrevNode() == end)
        return WalkResult::advance();
      return op->emitOpError("depends on an owner after its storage.end_domain"), WalkResult::interrupt();
    });
    if (after.wasInterrupted())
      return failure();
  }
  // Growth borrow exclusion: an element Place or slice of the owner formed
  // before its grow_backing is not used after it (sections 38, 77, 129).
  for (Operation *grow : grows) {
    Value key = ownerKey(grow->getOperand(0));
    for (Operation *holder : holders) {
      if (ownerKey(ownerOf(holder)) != key || !dominance.properlyDominates(holder, grow))
        continue;
      for (Operation *user : holder->getResult(0).getUsers())
        if (dominance.properlyDominates(grow, user))
          return user->emitOpError("uses an element Place or slice of an owner across its grow_backing"),
                 failure();
    }
  }
  return success();
}

class VerifyStorageTransitionsPass final
    : public sec::impl::SecVerifyStorageTransitionsBase<VerifyStorageTransitionsPass> {
public:
  void runOnOperation() override {
    if (failed(verifyFunction(getOperation())))
      signalPassFailure();
  }
};
} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyStorageTransitionsPass() {
  return std::make_unique<VerifyStorageTransitionsPass>();
}
