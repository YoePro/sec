#include "sec/Analysis/Passes.h"
#include "ReferenceFlow.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "mlir/Interfaces/ControlFlowInterfaces.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/SmallPtrSet.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYSLICEGUARDS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;
using namespace sec;
using namespace sec::detail;

namespace {
// Require every use, including forwarded block arguments, to stay below the
// selected outcome. A branch may forward a result only on that outcome.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§80,105.
LogicalResult confinedUses(Value value, cf::CondBranchOp guard, bool trueEdge,
                          DominanceInfo &dominance) {
  SmallVector<Value> worklist{value};
  llvm::DenseSet<Value> visited;
  while (!worklist.empty()) {
    Value current = worklist.pop_back_val();
    if (!visited.insert(current).second) continue;
    for (OpOperand &use : current.getUses()) {
      Operation *owner = use.getOwner();
      auto branch = dyn_cast<BranchOpInterface>(owner);
      if (owner != guard && !guardedBy(dominance, guard.getCondition(), trueEdge, owner))
        return owner->emitOpError("slice range result used outside its guarded edge");
      if (!branch) continue;
      for (unsigned i = 0; i < branch->getNumSuccessors(); ++i) {
        auto successor = branch.getSuccessorOperands(i);
        auto operands = successor.getForwardedOperands();
        for (auto [index, operand] : llvm::enumerate(operands)) {
          if (operand != current) continue;
          if (owner == guard && i != (trueEdge ? 0u : 1u))
            return owner->emitOpError("slice range result forwarded on the wrong outcome");
          worklist.push_back(branch->getSuccessor(i)->getArgument(
              index + successor.getProducedOperandCount()));
        }
      }
    }
  }
  return success();
}

// A temporal guard checks the exact bounded SSA value and must fail through
// the stale-reference endpoint, independently of spatial recovery handlers.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§107–110.
LogicalResult verifyObservation(Value source, Operation *use, DominanceInfo &dominance) {
  source = sourceValue(source);
  if (!source)
    return use->emitOpError("slice observation has no unambiguous reference facts");
  ReferenceFacts facts = referenceFacts(source);
  if (!facts.known)
    return use->emitOpError("slice observation requires canonical P15 reference facts");
  if (!facts.dynamic) return success();
  bool validated = false;
  use->getParentOfType<func::FuncOp>().walk([&](RefIsValidOp valid) {
    if (sourceValue(valid.getReference()) != source) return;
    auto branch = dyn_cast_or_null<cf::CondBranchOp>(valid->getNextNode());
    if (!branch || branch.getCondition() != valid.getResult() || !valid.getResult().hasOneUse())
      return;
    Block *bad = branch.getFalseDest();
    if (!isa<FailReferenceGenerationOp>(bad->getTerminator()) ||
        bad->getSinglePredecessor() != branch->getBlock())
      return;
    if (guardedBy(dominance, valid.getResult(), true, use)) validated = true;
  });
  if (validated) return success();
  return use->emitOpError("dynamic slice observation requires a dominating same-source validity guard and stale-reference failure");
}

// The false index edge either panics with slice-index provenance or produces
// the canonical IndexError.OutOfBounds for the existing fallible dispatcher.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§53–58,106,108.
bool indexFailure(Block *block, llvm::SmallPtrSetImpl<Block *> &active) {
  if (!active.insert(block).second) return false;
  if (auto panic = dyn_cast<FailBoundsOp>(block->getTerminator()))
    return panic->getAttrOfType<StringAttr>("operation").getValue() == "slice-index";
  for (Operation &op : *block) {
    auto constant = dyn_cast<EnumConstantOp>(op);
    auto error = constant ? dyn_cast<EnumType>(constant.getResult().getType()) : EnumType{};
    if (!error || error.getIdentity().getValue() != "core::IndexError" ||
        error.getCases().size() != 1) continue;
    for (Attribute attr : error.getCases()) {
      auto entry = cast<EnumCaseAttr>(attr);
      if (entry.getOrdinal() == constant.getCaseOrdinal() &&
          entry.getOrdinal() == 0 && entry.getValue().getValue() == "0" &&
          entry.getName().getValue() == "OutOfBounds" && !constant.getResult().use_empty())
        return true;
    }
  }
  auto branch = dyn_cast<BranchOpInterface>(block->getTerminator());
  if (!branch || branch->getNumSuccessors() == 0) return false;
  for (Block *successor : block->getSuccessors()) {
    llvm::SmallPtrSet<Block *, 8> path(active.begin(), active.end());
    if (!indexFailure(successor, path)) return false;
  }
  return true;
}

// Runtime range checks own one branch with true=failure and false=success;
// normalized outputs and errors cannot cross those edge boundaries.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§79–80,105.
LogicalResult verifyRangeGuard(SliceRangeCheckOp check, DominanceInfo &dominance) {
  auto branch = dyn_cast_or_null<cf::CondBranchOp>(check->getNextNode());
  if (!branch || branch.getCondition() != check.getFailed() || !check.getFailed().hasOneUse() ||
      branch.getTrueDest() == branch.getFalseDest())
    return check.emitOpError("range check requires an immediate single-use failed branch with distinct outcomes");
  if (branch.getTrueDest()->getSinglePredecessor() != branch->getBlock() ||
      branch.getFalseDest()->getSinglePredecessor() != branch->getBlock())
    return check.emitOpError("range outcomes require dedicated successors");
  if (failed(confinedUses(check.getStart(), branch, false, dominance)) ||
      failed(confinedUses(check.getLength(), branch, false, dominance)) ||
      failed(confinedUses(check.getError(), branch, true, dominance))) return failure();
  if (auto panic = dyn_cast<FailBoundsOp>(branch.getTrueDest()->getTerminator())) {
    if (panic->getAttrOfType<StringAttr>("operation").getValue() != "slice-range")
      return check.emitOpError("range failure requires slice-range bounds provenance");
  } else if (check.getError().use_empty()) {
    return check.emitOpError("range failure must panic or consume canonical RangeError");
  }
  return success();
}

// Construction consumes one normalization tuple for its exact source extent.
// A reborrow is therefore bounded relative to that source, never its parent or
// a different slice. Original endpoints/proof remain in the range operation.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–46,83,94–97.
LogicalResult verifyConstruction(Operation *op, DominanceInfo &dominance) {
  Value source = sourceValue(op->getOperand(0));
  auto start = dyn_cast_or_null<OpResult>(sourceValue(op->getOperand(1)));
  auto length = dyn_cast_or_null<OpResult>(sourceValue(op->getOperand(2)));
  Operation *range = start ? start.getOwner() : nullptr;
  if (!range || !isa<SliceRangeNormalizeOp, SliceRangeCheckOp>(range) ||
      start.getResultNumber() != 0 || !length || length.getOwner() != range || length.getResultNumber() != 1)
    return op->emitOpError("construction requires the same normalized start/length tuple");
  Value extent = sourceValue(range->getOperand(0));
  if (isa<SliceReborrowSharedOp, SliceReborrowMutOp>(op)) {
    auto len = extent ? extent.getDefiningOp<SliceLenOp>() : SliceLenOp{};
    if (!len || sourceValue(len.getSource()) != source)
      return op->emitOpError("reborrow range must use the same source slice length");
    return verifyObservation(op->getOperand(0), op, dominance);
  }
  auto place = source ? dyn_cast<PlaceType>(source.getType()) : PlaceType{};
  auto array = place ? dyn_cast<ArrayType>(place.getElementType()) : ArrayType{};
  auto constant = extent ? extent.getDefiningOp<ConstIntOp>() : ConstIntOp{};
  if (!array || !constant || constant.getValue() != array.getLength())
    return op->emitOpError("fixed-array range requires its exact source extent");
  return success();
}
} // namespace

// Verify represented CFG/SSA proof and temporal ordering without repeating
// source range analysis, choosing layout, or reclassifying failure precedence.
// RangeError's canonical classes/order are checked by the range op verifier.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.24;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§83,105–110.
LogicalResult sec::verifySliceGuards(func::FuncOp function) {
  DominanceInfo dominance(function);
  WalkResult result = function.walk([&](Operation *op) {
    LogicalResult checked = success();
    if (auto range = dyn_cast<SliceRangeCheckOp>(op))
      checked = verifyRangeGuard(range, dominance);
    if (failed(checked)) return WalkResult::interrupt();
    if (isa<SliceRangeNormalizeOp, SliceRangeCheckOp>(op)) {
      Value extent = sourceValue(op->getOperand(0));
      if (auto len = extent ? extent.getDefiningOp<SliceLenOp>() : SliceLenOp{})
        checked = verifyObservation(len.getSource(), op, dominance);
    } else if (isa<SliceBorrowSharedOp, SliceBorrowMutOp, SliceReborrowSharedOp, SliceReborrowMutOp>(op)) {
      checked = verifyConstruction(op, dominance);
    } else if (auto len = dyn_cast<SliceLenOp>(op)) {
      checked = verifyObservation(len.getSource(), op, dominance);
    } else if (auto index = dyn_cast<SliceIndexInBoundsOp>(op)) {
      checked = verifyObservation(index.getSource(), op, dominance);
      auto branch = dyn_cast_or_null<cf::CondBranchOp>(index->getNextNode());
      llvm::SmallPtrSet<Block *, 8> failurePath;
      if (succeeded(checked) && (!branch || branch.getCondition() != index.getResult() ||
          !index.getResult().hasOneUse() || branch.getTrueDest() == branch.getFalseDest() ||
          branch.getFalseDest()->getSinglePredecessor() != branch->getBlock() ||
          !indexFailure(branch.getFalseDest(), failurePath)))
        checked = index.emitOpError("index guard requires true access and a dedicated bounds/IndexError failure edge");
    } else if (auto element = dyn_cast<SliceElementPlaceOp>(op)) {
      checked = verifyObservation(element.getSource(), op, dominance);
      if (succeeded(checked) && element.getBoundsKind() == "runtime-check") {
        bool guarded = false;
        function.walk([&](SliceIndexInBoundsOp index) {
          if (sameSource(index.getSource(), element.getSource()) &&
              sameSource(index.getIndex(), element.getIndex()) &&
              guardedBy(dominance, index.getResult(), true, op)) guarded = true;
        });
        if (!guarded) checked = element.emitOpError("element Place requires the same slice/index bounds-true edge");
      }
    }
    return failed(checked) ? WalkResult::interrupt() : WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}

namespace {
// Register the schema-12 function-level safety pass.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.24.
class VerifySliceGuardsPass final : public sec::impl::SecVerifySliceGuardsBase<VerifySliceGuardsPass> {
public:
  void runOnOperation() override {
    if (failed(sec::verifySliceGuards(getOperation()))) signalPassFailure();
  }
};
} // namespace

// Construct the same pass for explicit verifier pipelines.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.24.
std::unique_ptr<Pass> sec::createSecVerifySliceGuardsPass() {
  return std::make_unique<VerifySliceGuardsPass>();
}
