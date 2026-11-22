// Fold copy-trivial field projections of struct constructions before the
// entry gate of the LLVM track. BuildInfo.Current() is materialized as a
// constant sec.struct.construct (MD-143 § 113B(2)); reading one scalar field
// of it needs no struct or string representation, only the field operand.
// Nothing outside a folded construction tree changes, so every remaining
// struct, string or union value still reaches the entry gate and is
// reported there.
//
// Rules:
//   - rules/compiler/compiler_known_members.md — § 113B(2)
//   - rules/mlir/sec_mlir_lowering.md — staged lowering, no hidden fallback
//   - rules/mlir/sec_mlir_dialect.md — sec.struct.construct/extract

#include "sec/Conversion/SecToLLVM/Passes.h"
#include "mlir/IR/BuiltinOps.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "llvm/ADT/SetVector.h"

namespace sec {
#define GEN_PASS_DEF_SECFOLDSTRUCTPROJECTIONS
#include "sec/Conversion/SecToLLVM/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {

bool allCopyTrivial(ArrayAttr actions) {
  for (Attribute action : actions) {
    auto text = dyn_cast<StringAttr>(action);
    if (!text || text.getValue() != "copy-trivial")
      return false;
  }
  return true;
}

// A producer that may disappear once unused: constants and constructions
// whose every operand was copied trivially, so no owner is dropped.
bool erasableWhenDead(Operation *operation) {
  if (isa<sec::ConstStringOp, sec::ConstIntOp, sec::ConstBoolOp>(operation))
    return true;
  if (auto construct = dyn_cast<sec::StructConstructOp>(operation))
    return allCopyTrivial(construct.getFieldActions());
  if (auto construct = dyn_cast<sec::UnionConstructOp>(operation))
    return allCopyTrivial(construct.getPayloadActions());
  return false;
}

struct FoldStructProjectionsPass
    : sec::impl::SecFoldStructProjectionsBase<FoldStructProjectionsPass> {
  void runOnOperation() override {
    llvm::SetVector<Operation *> candidates;
    SmallVector<sec::StructExtractOp> extracts;
    getOperation().walk([&](sec::StructExtractOp extract) { extracts.push_back(extract); });
    for (sec::StructExtractOp extract : extracts) {
      auto construct = extract.getSource().getDefiningOp<sec::StructConstructOp>();
      if (!construct || extract.getAction() != "copy-trivial")
        continue;
      uint32_t field = extract.getField();
      ArrayAttr actions = construct.getFieldActions();
      if (field >= construct.getFields().size() || field >= actions.size())
        continue;
      auto action = dyn_cast<StringAttr>(actions[field]);
      if (!action || action.getValue() != "copy-trivial")
        continue;
      Value operand = construct.getFields()[field];
      if (operand.getType() != extract.getType())
        continue;
      extract.getResult().replaceAllUsesWith(operand);
      extract.erase();
      candidates.insert(construct);
    }
    // Erase only the folded constructions and the producers they alone kept.
    while (!candidates.empty()) {
      Operation *operation = candidates.pop_back_val();
      if (!operation->use_empty() || !erasableWhenDead(operation))
        continue;
      SmallVector<Value> operands(operation->getOperands());
      operation->erase();
      for (Value operand : operands)
        if (Operation *producer = operand.getDefiningOp())
          candidates.insert(producer);
    }
  }
};

} // namespace
