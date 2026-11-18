// Whole-function dynamic-array growth and storage-transition protocol.
// Rules: rules/mlir/packages/sec-mlir-dialect_package18.md — sections 37, 63-67, 128-130.
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Pass/Pass.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "llvm/ADT/DenseMap.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYDYNAMICARRAYS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {
// testedBy returns the conditional branch that immediately follows op in its
// block and tests `condition`, or null.
cf::CondBranchOp testedBy(Operation *op, Value condition) {
  Operation *next = op->getNextNode();
  auto branch = dyn_cast_or_null<cf::CondBranchOp>(next);
  if (!branch || branch->getNextNode() || branch.getCondition() != condition)
    return {};
  return branch;
}

struct Guard {
  Value owner;
  StringRef transition;
  uint32_t domain = 0;
};

LogicalResult verifyFunction(func::FuncOp function) {
  // Failed-edge targets of state tests, keyed by the block they enter.
  llvm::DenseMap<Block *, Guard> backingFalse, capacityFalse, growthSuccess;
  // True edges of has_capacity, keyed by the block they enter.
  llvm::DenseMap<Block *, SmallVector<Value>> capacityTrue;
  WalkResult walk = function.walk([&](Operation *op) -> WalkResult {
    if (auto allocate = dyn_cast<sec::DynamicArrayAllocateOp>(op)) {
      auto branch = testedBy(op, allocate.getFailed());
      if (!branch)
        return op->emitOpError("must be immediately tested on failed"), WalkResult::interrupt();
      for (Value argument : branch.getTrueDestOperands())
        if (argument == allocate.getArray())
          return op->emitOpError("candidate escapes on its failure edge"), WalkResult::interrupt();
      Block *failure = branch.getTrueDest();
      if (failure->getSinglePredecessor())
        for (Operation &use : *failure)
          for (Value operand : use.getOperands())
            if (operand == allocate.getArray())
              return op->emitOpError("candidate is used on its failure edge"), WalkResult::interrupt();
      return WalkResult::advance();
    }
    if (auto test = dyn_cast<sec::DynamicArrayHasBackingOp>(op)) {
      auto branch = testedBy(op, test.getResult());
      if (!branch)
        return op->emitOpError("must be immediately tested"), WalkResult::interrupt();
      backingFalse[branch.getFalseDest()] = {test.getOwner()};
      return WalkResult::advance();
    }
    if (auto test = dyn_cast<sec::DynamicArrayHasCapacityOp>(op)) {
      auto branch = testedBy(op, test.getResult());
      if (!branch)
        return op->emitOpError("must be immediately tested"), WalkResult::interrupt();
      capacityFalse[branch.getFalseDest()] = {test.getOwner()};
      capacityTrue[branch.getTrueDest()].push_back(test.getOwner());
      return WalkResult::advance();
    }
    if (auto grow = dyn_cast<sec::DynamicArrayGrowBackingOp>(op)) {
      auto branch = testedBy(op, grow.getFailed());
      if (!branch)
        return op->emitOpError("must be immediately tested on failed"), WalkResult::interrupt();
      if (&op->getBlock()->front() != op)
        return op->emitOpError("must open its block after its state test"), WalkResult::interrupt();
      growthSuccess[branch.getFalseDest()] = {grow.getOwner(), *grow.getTransition(), grow.getDomain().value_or(0)};
    }
    return WalkResult::advance();
  });
  if (walk.wasInterrupted())
    return failure();

  walk = function.walk([&](Operation *op) -> WalkResult {
    if (auto grow = dyn_cast<sec::DynamicArrayGrowBackingOp>(op)) {
      Block *block = op->getBlock();
      auto &entry = grow.getTransition() == StringRef("establish-domain") ? backingFalse : capacityFalse;
      auto test = entry.find(block);
      if (test == entry.end() || test->second.owner != grow.getOwner() || !block->getSinglePredecessor())
        return op->emitOpError("must follow the failed state test of the same owner"), WalkResult::interrupt();
      return WalkResult::advance();
    }
    auto transition = [&](Value owner, StringRef want, std::optional<uint32_t> domain) -> WalkResult {
      Block *block = op->getBlock();
      auto growth = growthSuccess.find(block);
      if (&block->front() != op || growth == growthSuccess.end() || growth->second.transition != want ||
          growth->second.domain != domain.value_or(0) || growth->second.owner != owner ||
          !block->getSinglePredecessor())
        return op->emitOpError("must open the success edge of the matching sec.dynamic_array.grow_backing"),
               WalkResult::interrupt();
      return WalkResult::advance();
    };
    if (auto establish = dyn_cast<sec::StorageEstablishDomainOp>(op))
      return transition(establish.getOwner(), "establish-domain", establish.getDomain());
    if (auto advance = dyn_cast<sec::StorageAdvanceEpochOp>(op))
      return transition(advance.getOwner(), "advance-epoch", advance.getDomain());
    if (auto append = dyn_cast<sec::DynamicArrayAppendPreparedOp>(op)) {
      Block *block = op->getBlock();
      if (&block->front() != op || block->hasNoPredecessors())
        return op->emitOpError("must open a block entered after capacity is ensured"), WalkResult::interrupt();
      for (Block *predecessor : block->getPredecessors()) {
        bool ensured = false;
        if (auto owners = capacityTrue.find(block); owners != capacityTrue.end())
          for (Value owner : owners->second) {
            auto *terminator = predecessor->getTerminator();
            Operation *test = terminator ? terminator->getPrevNode() : nullptr;
            auto capacity = dyn_cast_or_null<sec::DynamicArrayHasCapacityOp>(test);
            ensured |= owner == append.getOwner() && capacity && capacity.getOwner() == owner;
          }
        if (!ensured && !predecessor->empty()) {
          Operation &first = predecessor->front();
          bool applied = (isa<sec::StorageEstablishDomainOp>(first) || isa<sec::StorageAdvanceEpochOp>(first)) &&
                         first.getOperand(0) == append.getOwner() &&
                         isa<cf::BranchOp>(predecessor->getTerminator());
          ensured = applied;
        }
        if (!ensured)
          return op->emitOpError("is reachable without ensured capacity"), WalkResult::interrupt();
      }
    }
    return WalkResult::advance();
  });
  return failure(walk.wasInterrupted());
}

class VerifyDynamicArraysPass final
    : public sec::impl::SecVerifyDynamicArraysBase<VerifyDynamicArraysPass> {
public:
  void runOnOperation() override {
    if (failed(verifyFunction(getOperation())))
      signalPassFailure();
  }
};
} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyDynamicArraysPass() {
  return std::make_unique<VerifyDynamicArraysPass>();
}
