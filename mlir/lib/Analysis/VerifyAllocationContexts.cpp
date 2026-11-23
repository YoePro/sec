// Hidden allocation-context arguments and their propagation (schema 15).
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — sections 107-109, 125, 128.
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/SymbolTable.h"
#include "mlir/Pass/Pass.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYALLOCATIONCONTEXTS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {
constexpr StringLiteral kHidden = "sec.hidden_allocation_context";

bool hiddenContext(func::FuncOp function) {
  return function.getNumArguments() != 0 && isa<sec::AllocContextType>(function.getArgument(0).getType());
}

bool isForeign(func::FuncOp function) {
  auto external = function->getAttrOfType<BoolAttr>("sec.extern");
  return (external && external.getValue()) || function->hasAttr("sec.abi");
}

LogicalResult verifyModule(ModuleOp module) {
  bool failed = false;
  auto fail = [&](Operation *op, const Twine &message) {
    op->emitOpError(message);
    failed = true;
  };
  SymbolTable symbols(module);
  module.walk([&](func::FuncOp function) {
    for (unsigned index = 0; index < function.getNumArguments(); ++index) {
      bool context = isa<sec::AllocContextType>(function.getArgument(index).getType());
      auto marked = function.getArgAttrOfType<BoolAttr>(index, kHidden);
      if (context != (marked && marked.getValue()))
        fail(function, "marks exactly its !sec.alloc_context argument as sec.hidden_allocation_context");
      if (context && index != 0)
        fail(function, "carries its hidden allocation context as the first argument, once");
    }
    // Section 130: the callable summary agrees with the signature.
    auto requires = function->getAttrOfType<BoolAttr>("sec.requires_allocation_context");
    if (bool(requires && requires.getValue()) != hiddenContext(function))
      fail(function, "sec.requires_allocation_context must agree with its hidden allocation context");
    if (auto may = function->getAttrOfType<StringAttr>("sec.may_allocate")) {
      StringRef knowledge = may.getValue();
      if (knowledge != "allocation-free" && knowledge != "may-allocate" && knowledge != "unknown")
        fail(function, "sec.may_allocate must be allocation-free, may-allocate or unknown");
      else if (knowledge == "allocation-free" && hiddenContext(function))
        fail(function, "an allocation-free function cannot require an allocation context");
    }
    if (hiddenContext(function) && isForeign(function))
      fail(function, "is a foreign/extern ABI function and cannot gain a hidden allocation context");
    for (Type result : function.getFunctionType().getResults())
      if (isa<sec::AllocContextType>(result))
        fail(function, "cannot return an allocation context");
  });
  module.walk([&](Operation *op) {
    for (Region &region : op->getRegions())
      for (Block &block : region)
        if (!block.isEntryBlock())
          for (BlockArgument argument : block.getArguments())
            if (isa<sec::AllocContextType>(argument.getType()))
              fail(op, "passes an allocation context through a block argument");
    if (auto call = dyn_cast<sec::CallDirectOp>(op)) {
      auto callee = symbols.lookup<func::FuncOp>(call.getCallee());
      bool required = callee && hiddenContext(callee);
      bool forwarded = !call.getArguments().empty() && isa<sec::AllocContextType>(call.getArguments().front().getType());
      if (required != forwarded)
        fail(op, required ? "must forward the callee's hidden allocation context as its first operand"
                          : "forwards an allocation context the callee does not require");
    }
    for (OpOperand &use : op->getOpOperands()) {
      if (!isa<sec::AllocContextType>(use.get().getType()))
        continue;
      bool allowed = (isa<sec::CallDirectOp>(op) && use.getOperandNumber() == 0) ||
                     (isa<sec::ArenaCreateOwnedFixedOp, sec::ArenaCreateGrowableOp>(op) && use.getOperandNumber() == 1);
      if (!allowed)
        fail(op, "uses an allocation context outside owned Arena creation or hidden-context forwarding");
    }
  });
  return failure(failed);
}

class VerifyAllocationContextsPass final
    : public sec::impl::SecVerifyAllocationContextsBase<VerifyAllocationContextsPass> {
public:
  void runOnOperation() override {
    if (failed(verifyModule(getOperation())))
      signalPassFailure();
  }
};
} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyAllocationContextsPass() {
  return std::make_unique<VerifyAllocationContextsPass>();
}
