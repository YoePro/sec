// Restores dialect-scoped branch ownership metadata (dialect v13 section 26)
// that a branch-rewriting optimization dropped. The action of a forwarded
// operand is a function of its type, so the restored metadata is exactly what
// the emitter writes; existing metadata is left for --sec-verify-ownership.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v13.md — section 26;
// rules/mlir/packages/sec-mlir-dialect_package17.md — section 57.
#include "DestructionFacts.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Interfaces/ControlFlowInterfaces.h"
#include "mlir/Pass/Pass.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"

namespace sec {
#define GEN_PASS_DEF_SECANNOTATESUCCESSORACTIONS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {
class AnnotateSuccessorActionsPass final
    : public sec::impl::SecAnnotateSuccessorActionsBase<AnnotateSuccessorActionsPass> {
public:
  void runOnOperation() override {
    ModuleOp module = getOperation();
    auto version = module->getAttrOfType<IntegerAttr>("sec.dialect_version");
    if (!version || version.getInt() < 13)
      return;
    sec::DestructionFacts facts(module);
    Builder builder(module.getContext());
    module.walk([&](BranchOpInterface branch) {
      Operation *op = branch.getOperation();
      if (op->hasAttr("sec.successor_actions"))
        return;
      bool sensitive = false;
      SmallVector<Attribute> lists;
      for (unsigned index = 0; index < op->getNumSuccessors(); ++index) {
        SmallVector<Attribute> actions;
        for (Value operand : branch.getSuccessorOperands(index).getForwardedOperands()) {
          Type type = operand.getType();
          StringRef action = "copy";
          if (isa<sec::RefType, sec::RefMutType, sec::SliceType, sec::SliceMutType, sec::StringViewType>(type))
            action = "non-owning";
          else if (facts.needsDestruction(type))
            action = "move";
          sensitive |= action != "copy";
          actions.push_back(builder.getStringAttr(action));
        }
        lists.push_back(builder.getArrayAttr(actions));
      }
      if (sensitive)
        op->setAttr("sec.successor_actions", builder.getArrayAttr(lists));
    });
  }
};
} // namespace

std::unique_ptr<mlir::Pass> sec::createSecAnnotateSuccessorActionsPass() {
  return std::make_unique<AnnotateSuccessorActionsPass>();
}
