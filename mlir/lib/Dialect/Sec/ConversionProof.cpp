#include "sec/Dialect/Sec/SecOps.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Interfaces/DataLayoutInterfaces.h"
#include "llvm/ADT/StringSwitch.h"

using namespace mlir;

// Verify the transported Sema contract, not the mathematical conversion.
// Zero plan width denotes a plan-independent proof; a plan-specific proof
// must match this module's authoritative index layout.
// Rules: rules/compiler/semantic_ir.md — § 124(2–3).
LogicalResult sec::verifyConversionProof(Operation *operation,
                                          bool allowLegacyAbsence) {
  Attribute attribute = operation->getAttr("sec.conversion_proof");
  if (!attribute && allowLegacyAbsence)
    return success();
  auto proof = dyn_cast_or_null<DictionaryAttr>(attribute);
  if (!proof)
    return operation->emitOpError("requires a sec.conversion_proof dictionary");
  auto basis = proof.getAs<StringAttr>("basis");
  if (!basis || !llvm::StringSwitch<bool>(basis.getValue())
                         .Cases({"constant", "source-domain", "closed-enum-domain",
                                 "open-enum-domain", "compiler-widening"}, true)
                         .Default(false))
    return operation->emitOpError("unknown exactness proof basis");
  auto width = proof.getAs<IntegerAttr>("plan_pointer_width");
  if (!width || !width.getType().isInteger(32) ||
      (width.getInt() != 0 && width.getInt() != 32 && width.getInt() != 64))
    return operation->emitOpError("exactness proof plan_pointer_width must be 0, 32 or 64 : i32");
  if (proof.size() != 2)
    return operation->emitOpError("unexpected exactness proof fields");
  if (width.getInt() != 0) {
    auto module = operation->getParentOfType<ModuleOp>();
    if (!module || !module->hasAttr("dlti.dl_spec"))
      return operation->emitOpError("plan-specific exactness proof requires module data layout");
    DataLayout layout(module);
    auto bits = layout.getTypeSizeInBits(IndexType::get(operation->getContext()));
    if (bits.isScalable() || bits.getFixedValue() != static_cast<uint64_t>(width.getInt()))
      return operation->emitOpError("exactness proof is incompatible with module plan width");
  }
  return success();
}

