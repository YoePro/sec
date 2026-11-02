// Facts the sec.destruction_plans table gives the ownership analyses.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 19-27.
#ifndef SEC_ANALYSIS_DESTRUCTIONFACTS_H
#define SEC_ANALYSIS_DESTRUCTIONFACTS_H

#include "mlir/IR/BuiltinOps.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/DenseMap.h"

namespace sec {
// stripNominal is the shape a plan destroys: a named or distinct wrapper has its
// base's fields, element or variants.
inline mlir::Type stripNominal(mlir::Type type) {
  while (true) {
    if (auto named = mlir::dyn_cast<NamedType>(type))
      type = named.getBase();
    else if (auto distinct = mlir::dyn_cast<DistinctType>(type))
      type = distinct.getBase();
    else
      return type;
  }
}

// DestructionFacts derives non-triviality from the table: the types carry no
// custom-free marker, so a leaf is non-trivial only by its own non-trivial plan.
// A reference or slice ends a borrow and never destroys its referent (section 27).
class DestructionFacts {
public:
  explicit DestructionFacts(mlir::ModuleOp module) {
    if (auto table = module->getAttrOfType<mlir::ArrayAttr>("sec.destruction_plans"))
      for (mlir::Attribute attribute : table)
        if (auto entry = mlir::dyn_cast<mlir::DictionaryAttr>(attribute))
          if (auto type = entry.getAs<mlir::TypeAttr>("type"))
            entries[type.getValue()] = entry;
  }

  mlir::DictionaryAttr planOf(mlir::Type type) const { return entries.lookup(type); }

  bool hasCustomFree(mlir::Type type) const {
    mlir::DictionaryAttr entry = planOf(type);
    auto flag = entry ? entry.getAs<mlir::BoolAttr>("custom_free") : mlir::BoolAttr();
    return flag && flag.getValue();
  }

  bool needsDestruction(mlir::Type type) {
    auto known = memo.find(type);
    if (known != memo.end())
      return known->second;
    bool result = false;
    mlir::DictionaryAttr entry = planOf(type);
    auto kind = entry ? entry.getAs<mlir::StringAttr>("kind") : mlir::StringAttr();
    if (kind && kind.getValue() != "trivial") {
      result = true;
    } else if (stripNominal(type) != type) {
      result = needsDestruction(stripNominal(type));
    } else if (auto array = mlir::dyn_cast<ArrayType>(type)) {
      result = array.getLength().getValue() != "0" && needsDestruction(array.getElementType());
    } else if (auto sum = mlir::dyn_cast<ResultType>(type)) {
      result = needsDestruction(sum.getSuccessType()) || needsDestruction(sum.getErrorType());
    } else if (auto structure = mlir::dyn_cast<StructType>(type)) {
      for (mlir::Attribute field : structure.getFields())
        result = result || needsDestruction(mlir::cast<StructFieldAttr>(field).getType());
    } else if (auto sum = mlir::dyn_cast<UnionType>(type)) {
      for (mlir::Attribute attribute : sum.getVariants()) {
        auto variant = mlir::cast<UnionVariantAttr>(attribute);
        result = result || (variant.getPayload() && needsDestruction(variant.getPayload()));
        for (mlir::Attribute field : variant.getFields())
          result = result || needsDestruction(mlir::cast<UnionFieldAttr>(field).getType());
      }
    }
    memo[type] = result;
    return result;
  }

private:
  llvm::DenseMap<mlir::Type, mlir::DictionaryAttr> entries;
  llvm::DenseMap<mlir::Type, bool> memo;
};
} // namespace sec

#endif
