// Whole-function Arena domain, owner and epoch flow (schema 15).
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — sections 35, 90-91, 124, 126-127, 129, 151.
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/Dominance.h"
#include "mlir/Pass/Pass.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/DenseMap.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/STLExtras.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYARENAS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {
struct ArenaStep {
  uint32_t domain = 0;
  std::optional<uint32_t> stateBefore, epochBefore, epochAfter;
  uint32_t stateAfter = 0;
  bool reset = false, terminal = false;
};

template <typename Op>
ArenaStep stepOf(Op op) {
  ArenaStep step;
  step.domain = op.getDomain().value_or(0);
  step.stateBefore = op.getStateBefore();
  step.stateAfter = op.getStateAfter();
  step.epochBefore = op.getEpochBefore();
  step.epochAfter = op.getEpochAfter();
  return step;
}

std::optional<ArenaStep> arenaStep(Operation *op) {
  if (auto create = dyn_cast<sec::ArenaCreateBorrowedOp>(op))
    return stepOf(create);
  if (auto create = dyn_cast<sec::ArenaCreateDefaultOp>(op))
    return stepOf(create);
  if (auto create = dyn_cast<sec::ArenaCreateOwnedFixedOp>(op))
    return stepOf(create);
  if (auto create = dyn_cast<sec::ArenaCreateGrowableOp>(op))
    return stepOf(create);
  if (auto allocate = dyn_cast<sec::ArenaNewOp>(op))
    return stepOf(allocate);
  if (auto allocate = dyn_cast<sec::ArenaAllocOp>(op))
    return stepOf(allocate);
  if (auto reset = dyn_cast<sec::ArenaResetOp>(op)) {
    ArenaStep step = stepOf(reset);
    step.reset = true;
    return step;
  }
  if (auto release = dyn_cast<sec::ArenaReleaseOp>(op)) {
    ArenaStep step = stepOf(release);
    step.terminal = true;
    return step;
  }
  if (auto destroy = dyn_cast<sec::ArenaDestroyOp>(op)) {
    ArenaStep step = stepOf(destroy);
    step.terminal = true;
    return step;
  }
  return std::nullopt;
}

bool consumesOwner(Operation *op) { return isa<sec::ArenaReleaseOp, sec::ArenaDestroyOp>(op); }

LogicalResult verifyFunction(func::FuncOp function) {
  DominanceInfo dominance(function);
  llvm::DenseMap<uint32_t, Operation *> created;
  bool failed = false;
  auto fail = [&](Operation *op, const Twine &message) {
    op->emitOpError(message);
    failed = true;
  };
  // Fresh domains and per-block state/epoch chains.
  for (Block &block : function.getBody()) {
    llvm::DenseMap<uint32_t, ArenaStep> last;
    for (Operation &op : block) {
      auto step = arenaStep(&op);
      if (!step)
        continue;
      if (isa<sec::ArenaCreateBorrowedOp, sec::ArenaCreateDefaultOp, sec::ArenaCreateOwnedFixedOp, sec::ArenaCreateGrowableOp>(op)) {
        if (auto [it, inserted] = created.try_emplace(step->domain, &op); !inserted)
          fail(&op, "must establish a fresh Arena domain; another creation in this function already established it");
        last[step->domain] = *step;
        continue;
      }
      if (auto previous = last.find(step->domain); previous != last.end()) {
        const ArenaStep &before = previous->second;
        if (before.terminal)
          fail(&op, "uses an Arena domain after its Release ended it");
        else if (step->stateBefore != before.stateAfter || step->epochBefore != before.epochAfter)
          fail(&op, "breaks the state-version or epoch chain of its Arena domain");
      }
      last[step->domain] = *step;
    }
  }
  // Section 130: sec.arena_demand_summary is compile-time metadata, one
  // entry per Arena domain this function creates.
  if (Attribute summary = function->getAttr("sec.arena_demand_summary")) {
    auto entries = dyn_cast<ArrayAttr>(summary);
    llvm::DenseSet<uint32_t> summarized;
    auto decimal = [](Attribute attribute) {
      auto text = dyn_cast_or_null<StringAttr>(attribute);
      return text && !text.getValue().empty() && llvm::all_of(text.getValue(), [](char c) { return c >= '0' && c <= '9'; });
    };
    if (!entries)
      fail(function, "sec.arena_demand_summary must be an array of domain summaries");
    else
      for (Attribute entry : entries) {
        auto fields = dyn_cast<DictionaryAttr>(entry);
        auto domain = fields ? fields.getAs<IntegerAttr>("domain") : IntegerAttr();
        auto kind = fields ? fields.getAs<StringAttr>("kind") : StringAttr();
        auto expression = fields ? fields.getAs<StringAttr>("expression") : StringAttr();
        if (!domain || domain.getInt() <= 0 || !kind || !expression || expression.getValue().empty() ||
            (kind.getValue() != "constant" && kind.getValue() != "unknown" && kind.getValue() != "unbounded")) {
          fail(function, "an Arena demand summary needs a domain, kind constant|unknown|unbounded and an expression");
          continue;
        }
        if ((kind.getValue() == "constant") != decimal(fields.get("bytes")) ||
            (fields.get("capacity_bytes") && !decimal(fields.get("capacity_bytes"))))
          fail(function, "constant Arena demand carries decimal bytes; unknown or unbounded demand carries none");
        uint32_t id = static_cast<uint32_t>(domain.getInt());
        if (!summarized.insert(id).second)
          fail(function, "summarizes an Arena domain twice");
        if (!created.count(id))
          fail(function, "summarizes an Arena domain it does not create");
      }
  }
  // Owner consumption: at most one consumer on a path, no use after it.
  function.walk([&](Operation *op) {
    for (Value result : op->getResults())
      if (isa<sec::ArenaType>(result.getType())) {
        SmallVector<Operation *> consumers;
        for (Operation *user : result.getUsers())
          if (consumesOwner(user))
            consumers.push_back(user);
        for (Operation *consumer : consumers) {
          for (Operation *other : consumers)
            if (other != consumer && dominance.dominates(consumer, other))
              fail(other, "releases an Arena owner that was already released");
          for (Operation *user : result.getUsers())
            if (user != consumer && dominance.properlyDominates(consumer, user) && !consumesOwner(user))
              fail(user, "uses an Arena owner after its Release");
        }
      }
  });
  // Sections 35, 151 and v15 section 14: failure atomicity. A checked
  // creation, New or Alloc is followed by the branch on its own failed flag;
  // the candidate never crosses the failure edge, and every other use is
  // dominated by the success block, which only that edge enters. A failed
  // allocation therefore never publishes a partial reference or slice.
  function.walk([&](Operation *op) {
    if (!isa<sec::ArenaNewOp, sec::ArenaAllocOp, sec::ArenaCreateOwnedFixedOp, sec::ArenaCreateGrowableOp>(op) || op->getNumResults() != 3)
      return;
    Value candidate = op->getResult(0), flag = op->getResult(1);
    auto branch = dyn_cast_or_null<cf::CondBranchOp>(op->getNextNode());
    if (!branch || branch.getCondition() != flag || branch->getNextNode()) {
      fail(op, "must end its block with the branch on its own failed flag before publishing its candidate");
      return;
    }
    if (llvm::is_contained(branch.getTrueDestOperands(), candidate))
      fail(op, "passes its candidate across the allocation failure edge");
    Block *success = branch.getFalseDest();
    if (!success->getSinglePredecessor()) {
      fail(op, "success edge must be the only way into its publication block");
      return;
    }
    for (Operation *user : candidate.getUsers()) {
      if (user == branch.getOperation())
        continue;
      if (!dominance.dominates(success, user->getBlock()))
        fail(user, "uses an Arena allocation candidate where the allocation may have failed");
    }
  });
  // Section 127: Arena references depend on their domain epoch.
  SmallVector<Operation *> invalidations;
  function.walk([&](Operation *op) {
    if (isa<sec::ArenaResetOp, sec::ArenaReleaseOp, sec::ArenaDestroyOp>(op))
      invalidations.push_back(op);
  });
  function.walk([&](Operation *op) {
    if (!isa<sec::ArenaNewOp, sec::ArenaAllocOp>(op))
      return;
    auto step = arenaStep(op);
    Value candidate = op->getResult(0);
    for (Operation *invalidation : invalidations) {
      auto after = arenaStep(invalidation);
      if (!after || after->domain != step->domain || after->epochBefore != step->epochAfter ||
          !dominance.properlyDominates(op, invalidation))
        continue;
      for (Operation *user : candidate.getUsers())
        if (dominance.properlyDominates(invalidation, user))
          fail(user, "uses an Arena reference after Reset or Release invalidated its epoch");
    }
  });
  return failure(failed);
}

class VerifyArenasPass final : public sec::impl::SecVerifyArenasBase<VerifyArenasPass> {
public:
  void runOnOperation() override {
    if (failed(verifyFunction(getOperation())))
      signalPassFailure();
  }
};
} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyArenasPass() {
  return std::make_unique<VerifyArenasPass>();
}
