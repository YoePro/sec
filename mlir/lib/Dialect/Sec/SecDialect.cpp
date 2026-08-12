#include "sec/Dialect/Sec/SecDialect.h"
#include "sec/Dialect/Sec/SecAttributes.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "sec/Dialect/Sec/SecOps.h"

#include "llvm/ADT/StringSet.h"
#include "llvm/ADT/STLExtras.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/BuiltinOps.h"

using namespace mlir;
using namespace sec;

#include "sec/Dialect/Sec/SecDialect.cpp.inc"

void SecDialect::initialize() {
  registerAttributes();
  registerTypes();
  registerOperations();
}

void SecDialect::registerOperations() {
  addOperations<
#define GET_OP_LIST
#include "sec/Dialect/Sec/SecOps.cpp.inc"
      >();
}

LogicalResult SecDialect::verifyOperationAttribute(Operation *operation,
                                                   NamedAttribute attribute) {
  StringRef name = attribute.getName().getValue();
  Attribute value = attribute.getValue();
  auto requireString = [&]() -> LogicalResult {
    if (!isa<StringAttr>(value))
      return operation->emitError() << name << " must be a string attribute";
    return success();
  };
  auto requireBool = [&]() -> LogicalResult {
    if (!isa<BoolAttr>(value))
      return operation->emitError() << name << " must be a boolean attribute";
    return success();
  };

  if (name == "sec.dialect_version") {
    auto module = dyn_cast<ModuleOp>(operation);
    auto version = dyn_cast<IntegerAttr>(value);
    auto integerType = version ? dyn_cast<IntegerType>(version.getType())
                               : IntegerType{};
    if (!module || !version || !integerType || integerType.getWidth() != 32)
      return operation->emitError(
          "sec.dialect_version must be an i32 module attribute");
    int64_t number = version.getInt();
    if (number != 1 && number != 2 && number != 3 && number != 4 &&
        number != 5 && number != 6 && number != 7 && number != 8)
      return operation->emitError("unsupported Sec dialect schema version");
    if (number == 1)
      return success();
    auto semanticVersion =
        module->getAttrOfType<IntegerAttr>("sec.semantic_ir_version");
    auto semanticType = semanticVersion
                            ? dyn_cast<IntegerType>(semanticVersion.getType())
                            : IntegerType{};
    if (!semanticVersion || !semanticType || semanticType.getWidth() != 32 ||
        semanticVersion.getInt() != 1)
      return operation->emitError()
             << "schema " << number
             << " requires sec.semantic_ir_version = 1 : i32";
    auto moduleID = module->getAttrOfType<StringAttr>("sec.module_id");
    if (!moduleID || moduleID.getValue().empty())
      return operation->emitError()
             << "schema " << number
             << " requires a non-empty sec.module_id string";
    auto sources = module->getAttrOfType<ArrayAttr>("sec.source_files");
    if (!sources)
      return operation->emitError()
             << "schema " << number << " requires sec.source_files array";
    llvm::StringSet<> seen;
    for (Attribute source : sources) {
      auto string = dyn_cast<StringAttr>(source);
      if (!string)
        return operation->emitError(
            "sec.source_files elements must be strings");
      if (!seen.insert(string.getValue()).second)
        return operation->emitError(
            "sec.source_files must not contain duplicates");
    }
    if (number >= 3) {
      for (StringRef targetName : {
               "sec.target_os", "sec.target_arch", "sec.target_triple",
               "sec.target_abi", "sec.target_profile",
               "sec.target_endianness"}) {
        if (!module->getAttrOfType<StringAttr>(targetName))
          return operation->emitError()
                 << "schema " << number << " requires " << targetName
                 << " string attribute";
      }
      StringRef endianness =
          module->getAttrOfType<StringAttr>("sec.target_endianness")
              .getValue();
      if (endianness != "little" && endianness != "big")
        return operation->emitError(
            "sec.target_endianness must be little or big");
      if (!module->hasAttr("dlti.dl_spec"))
        return operation->emitError()
               << "schema " << number << " requires explicit dlti.dl_spec";
    }
    return success();
  }

  if (name == "sec.semantic_ir_version") {
    auto integer = dyn_cast<IntegerAttr>(value);
    auto type = integer ? dyn_cast<IntegerType>(integer.getType())
                        : IntegerType{};
    if (!isa<ModuleOp>(operation) || !integer || !type ||
        type.getWidth() != 32 || integer.getInt() != 1)
      return operation->emitError(
          "sec.semantic_ir_version must be 1 : i32 on a module");
    return success();
  }

  if (name == "sec.function_id") {
    if (!isa<func::FuncOp>(operation))
      return operation->emitError(
          "sec.function_id is only valid on func.func");
    if (failed(requireString()) || cast<StringAttr>(value).getValue().empty())
      return failure();
    if (!operation->getAttrOfType<StringAttr>("sec.source_name") ||
        !operation->getAttrOfType<BoolAttr>("sec.extern") ||
        !operation->getAttrOfType<BoolAttr>("sec.unsafe"))
      return operation->emitError(
          "schema 2 or 3 function metadata is incomplete");
    return success();
  }

  if (name == "sec.source_name" || name == "sec.link_name" ||
      name == "sec.abi" || name == "sec.module_id" ||
      name == "sec.storage_class" || name == "sec.symbol_id" ||
      name == "sec.type_id" || name == "sec.layout_ref" ||
      name == "sec.target_os" || name == "sec.target_arch" ||
      name == "sec.target_triple" || name == "sec.target_abi" ||
      name == "sec.target_profile" || name == "sec.target_endianness")
    return requireString();
  if (name == "sec.match_stage") {
    if (failed(requireString()))
      return failure();
    StringRef stage = cast<StringAttr>(value).getValue();
    if (stage != "pattern" && stage != "guard" && stage != "body-exit" &&
        stage != "merge" && stage != "residual")
      return operation->emitError("invalid sec.match_stage");
    return success();
  }
  if (name == "sec.match_pattern_kind") {
    if (failed(requireString()))
      return failure();
    StringRef kind = cast<StringAttr>(value).getValue();
    if (kind != "enum-value" && kind != "union-variant" &&
        kind != "result-ok" && kind != "result-err" &&
        kind != "option-some" && kind != "option-none" &&
        kind != "catch-all")
      return operation->emitError("invalid sec.match_pattern_kind");
    return success();
  }
  if (name == "sec.operator" || name == "sec.try_handler_kind" ||
      name == "sec.try_handler_variant")
    return requireString();
  if (name == "sec.extern" || name == "sec.unsafe" ||
      name == "sec.mutable" || name == "sec.synthesized" ||
      name == "sec.try_handler_exhaustive")
    return requireBool();
  if (name == "sec.parameter_names" || name == "sec.source_files" ||
      name == "sec.argument_actions") {
    auto values = dyn_cast<ArrayAttr>(value);
    if (!values || !llvm::all_of(values, [](Attribute item) {
          return isa<StringAttr>(item);
        }))
      return operation->emitError() << name << " must be an array of strings";
  }
  if (name == "sec.storage_id") {
    auto integer = dyn_cast<IntegerAttr>(value);
    auto type = integer ? dyn_cast<IntegerType>(integer.getType())
                        : IntegerType{};
    if (!integer || !type || type.getWidth() != 32)
      return operation->emitError(
          "sec.storage_id must be an i32 integer attribute");
  }
  if (name == "sec.try_handler_index") {
    auto integer = dyn_cast<IntegerAttr>(value);
    auto type = integer ? dyn_cast<IntegerType>(integer.getType())
                        : IntegerType{};
    if (!integer || !type || type.getWidth() != 32 || integer.getInt() < -1)
      return operation->emitError(
          "sec.try_handler_index must be an i32 integer >= -1");
  }
  if (name == "sec.match_id" || name == "sec.match_arm_index") {
    auto integer = dyn_cast<IntegerAttr>(value);
    auto type = integer ? dyn_cast<IntegerType>(integer.getType())
                        : IntegerType{};
    int64_t minimum = name == "sec.match_id" ? 1 : 0;
    if (!integer || !type || type.getWidth() != 32 ||
        integer.getInt() < minimum)
      return operation->emitError()
             << name << " must be an i32 integer >= " << minimum;
  }
  return success();
}
