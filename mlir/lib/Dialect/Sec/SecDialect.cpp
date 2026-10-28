#include "sec/Dialect/Sec/SecDialect.h"
#include "sec/Dialect/Sec/SecAttributes.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/ReturnOrigins.h"
#include "sec/Dialect/Sec/TypeTraversal.h"
#include "OperationContracts.h"
#include "RegisterContracts.h"

#include "llvm/ADT/StringSet.h"
#include "llvm/ADT/StringSwitch.h"
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

namespace {
// isReferenceValueType recognizes bounded slices as safe reference values
// for the existing P15 metadata family without treating them as element refs.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v12.md sections 5,21-22;
// rules/mlir/packages/sec-mlir-dialect_package16.md sections 44,91.
bool isReferenceValueType(Type type) {
  return isa<RefType, RefMutType, SliceType, SliceMutType>(type);
}

// Reference facts of a value no schema-11 operation defines: a reference
// function parameter or call result (`sec.reference`, implementation choice
// MD-084). The keys mirror the reference operation properties of
// sec_mlir_dialect_v11.md section 7.
LogicalResult verifyReferenceDictionary(Operation *operation,
                                        Attribute value) {
  auto facts = dyn_cast<DictionaryAttr>(value);
  if (!facts)
    return operation->emitError("sec.reference must be a dictionary attribute");
  // MD-084 section 3(1): exact schema, not free-form descriptive metadata.
  for (NamedAttribute fact : facts) {
    StringRef key = fact.getName().getValue();
    bool known = llvm::StringSwitch<bool>(key)
        .Cases({"borrow_id", "lifetime_id", "validity_policy"}, true)
        .Cases({"validity_proof", "invalidation_domain", "epoch_dependency"}, true)
        .Cases({"address_stability", "provenance", "source_borrow"}, true)
        .Cases({"origin_storages", "pinned", "address_space"}, true)
        .Default(false);
    if (!known)
      return operation->emitError() << "unknown sec.reference field '" << key << "'";
  }
  // A proof is optional for dynamic policy, but retains its declared type
  // when present. MD-084 section 3(2); schema-11 reference properties.
  if (Attribute proof = facts.get("validity_proof"))
    if (!isa<StringAttr>(proof))
      return operation->emitError("sec.reference validity_proof must be a string");
  auto identity = [&](StringRef key, bool required) -> LogicalResult {
    Attribute entry = facts.get(key);
    if (!entry)
      return required ? operation->emitError()
                            << "sec.reference requires " << key
                      : success();
    auto integer = dyn_cast<IntegerAttr>(entry);
    auto type = integer ? dyn_cast<IntegerType>(integer.getType())
                        : IntegerType{};
    if (!integer || !type || type.getWidth() != 32 || integer.getInt() <= 0)
      return operation->emitError()
             << "sec.reference " << key << " must be a non-zero i32";
    return success();
  };
  auto text = [&](StringRef key) -> StringRef {
    auto string = facts.getAs<StringAttr>(key);
    return string ? string.getValue() : StringRef();
  };
  if (failed(identity("borrow_id", true)) ||
      failed(identity("lifetime_id", true)) ||
      failed(identity("invalidation_domain", false)) ||
      failed(identity("epoch_dependency", false)) ||
      failed(identity("source_borrow", false)))
    return failure();
  if (text("address_stability").empty() || text("provenance").empty())
    return operation->emitError(
        "sec.reference requires non-empty address_stability and provenance");
  if (failed(sec::detail::verifyAddressStability(operation, text("address_stability"))))
    return failure();
  StringRef policy = text("validity_policy");
  if (policy == "proven") {
    if (text("validity_proof").empty() || facts.get("epoch_dependency"))
      return operation->emitError(
          "a proven sec.reference names validity_proof and no epoch_dependency");
  } else if (policy == "dynamic-epoch") {
    if (!facts.get("invalidation_domain") || !facts.get("epoch_dependency"))
      return operation->emitError(
          "a dynamic-epoch sec.reference requires invalidation_domain and epoch_dependency");
  } else {
    return operation->emitError(
        "sec.reference validity_policy must be proven or dynamic-epoch");
  }
  if (Attribute space = facts.get("address_space"))
    if (!isa<StringAttr>(space) || cast<StringAttr>(space).getValue() != "mmio")
      return operation->emitError(
          "sec.reference address_space is omitted for ordinary memory and otherwise mmio");
  if (Attribute pinned = facts.get("pinned"))
    if (!isa<BoolAttr>(pinned))
      return operation->emitError("sec.reference pinned must be a boolean");
  if (Attribute origins = facts.get("origin_storages")) {
    auto array = dyn_cast<ArrayAttr>(origins);
    if (!array || !llvm::all_of(array, [](Attribute origin) {
          auto integer = dyn_cast<IntegerAttr>(origin);
          return integer && integer.getType().isInteger(32);
        }))
      return operation->emitError(
          "sec.reference origin_storages must be an array of i32");
  }
  return success();
}
} // namespace

LogicalResult SecDialect::verifyRegionArgAttribute(Operation *operation,
                                                   unsigned regionIndex,
                                                   unsigned argIndex,
                                                   NamedAttribute attribute) {
  (void)regionIndex;
  StringRef name = attribute.getName().getValue();
  if (name != "sec.reference")
    return success();
  auto function = dyn_cast<func::FuncOp>(operation);
  if (!function || argIndex >= function.getNumArguments() ||
      !isReferenceValueType(function.getArgumentTypes()[argIndex]))
    return operation->emitError(
        "sec.reference annotates a !sec.ref or !sec.ref_mut function parameter, or a schema-12 slice parameter");
  return verifyReferenceDictionary(operation, attribute.getValue());
}

// SecDialect::verifyOperationAttribute validates versioned module and operation
// metadata while retaining schemas 1-11 as compatibility inputs for P16.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md section 58
//   - rules/mlir/dialect-versions/sec_mlir_dialect_v10.md section 1
//   - rules/mlir/dialect-versions/sec_mlir_dialect_v12.md section 1
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
        number != 5 && number != 6 && number != 7 && number != 8 &&
        number != 9 && number != 10 && number != 11 && number != 12)
      return operation->emitError("unsupported Sec dialect schema version");
    // Reject MD-077 identities recursively, including type attributes and
    // signatures, before schema 11. No default schema change is implied.
    // Rules: rules/mlir/sec_mlir_dialect.md, v11.41.1 Types and admission.
    if (number < 11) {
      bool hasRegister = false;
      module.walk([&](Operation *nested) {
        hasRegister |= bool(findOperationType(nested, [](Type type) {
          return isa<RegisterType, HardwareInstanceType>(type);
        }));
      });
      if (hasRegister)
        return operation->emitError()
               << "register and hardware-instance types require schema 11, module declares schema "
               << number;
    }
    // Slice types (including nested type attributes, signatures and block
    // parameters) belong to schema 12. An older header cannot hide a P16 value
    // inside an aggregate or nominal wrapper.
    // Rules: rules/mlir/packages/sec-mlir-dialect_package16.md sections 87-90;
    // rules/mlir/dialect-versions/sec_mlir_dialect_v12.md sections 1-4.
    if (number < 12) {
      bool hasSlice = false;
      module.walk([&](Operation *nested) {
        hasSlice |= bool(findOperationType(nested, [](Type type) {
          return isa<SliceType, SliceMutType>(type);
        }));
      });
      if (hasSlice)
        return operation->emitError()
               << "slice types require schema 12, module declares schema "
               << number;
    }
    // Places and direct references exist from schema 11 (SEC-MLIR-P15).
    if (number < 11) {
      auto usesSchema11Type = [](Type type) {
        return isa<PlaceType, RefType, RefMutType>(type);
      };
      WalkResult walk = module->walk([&](Operation *nested) {
        if (isa<FailReferenceGenerationOp>(nested))
          return WalkResult::interrupt();
        if (findOperationType(nested, usesSchema11Type))
          return WalkResult::interrupt();
        return WalkResult::advance();
      });
      if (walk.wasInterrupted())
        return operation->emitError()
               << "Places, direct references and their operations require schema 11, module declares schema "
               << number;
    }
    if (number>=11 && failed(detail::verifyRegisterModule(module))) return failure();
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

  if (name=="sec.register_fit_proof") {
    if (!isa<RegisterFromBitsOp>(operation)) return operation->emitError("sec.register_fit_proof belongs only on register.from_bits");
    return success();
  }
  if (name == "sec.registers") {
    auto module=dyn_cast<ModuleOp>(operation);
    if (!module) return operation->emitError("sec.registers belongs on a module");
    auto version=module->getAttrOfType<IntegerAttr>("sec.dialect_version");
    if (!version || version.getInt()<11) return operation->emitError("sec.registers requires schema 11");
    return detail::verifyRegisterModule(module);
  }
  if (name=="sec.field_path" || name=="sec.write_proof") {
    if (!isa<RegisterExtractOp,RegisterUpdateOp>(operation)) return operation->emitError("register field metadata requires detached field operation");
    if (name=="sec.write_proof" && !isa<RegisterUpdateOp>(operation)) return operation->emitError("sec.write_proof belongs only on register.update");
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
  // Schema-11 discardable facts (implementation choice MD-084).
  auto definesReference = [&]() {
    return operation->getNumResults() == 1 &&
           (isReferenceValueType(operation->getResult(0).getType()) ||
            isa<SliceElementPlaceOp>(operation));
  };
  // MD-116 § 2(4): the explicit nonzero module-local StaticID of a static.
  if (name == "sec.static_id") {
    auto integer = dyn_cast<IntegerAttr>(value);
    auto type = integer ? dyn_cast<IntegerType>(integer.getType())
                        : IntegerType{};
    if (!isa<StaticGlobalOp>(operation) || !integer || !type ||
        type.getWidth() != 32 || integer.getValue().isZero())
      return operation->emitError(
          "sec.static_id must be a nonzero i32 on sec.static.global");
    return success();
  }
  if (name == "sec.address_taken") {
    if (!isa<StorageDeclareOp, StaticGlobalOp>(operation))
      return operation->emitError(
          "sec.address_taken annotates sec.storage.declare or sec.static.global");
    // MD-084 section 2(4): absence means no marker; false is not a marker.
    if (failed(requireBool())) return failure();
    if (!cast<BoolAttr>(value).getValue())
      return operation->emitError("sec.address_taken must be the boolean marker true");
    return success();
  }
  if ((name == "sec.pinned" || name == "sec.origin_storages" ||
       name == "sec.reference_address_space") &&
      !definesReference())
    return operation->emitError()
           << name << " annotates an operation defining one !sec.ref or !sec.ref_mut, or a schema-12 slice";
  if (name == "sec.pinned")
    return requireBool();
  if (name == "sec.reference") {
    if (operation->getNumResults() != 1 ||
        !isReferenceValueType(operation->getResult(0).getType()))
      return operation->emitError(
          "sec.reference annotates an operation with one !sec.ref or !sec.ref_mut result, or a schema-12 slice result");
    if (failed(verifyReferenceDictionary(operation, value))) return failure();
    // MD-084 section 2(5): a dictionary cannot override the operation's
    // own semantic properties. In particular, it cannot hide a dynamic epoch
    // from the shared reference-fact reader by claiming proven validity.
    for (NamedAttribute fact : cast<DictionaryAttr>(value)) {
      StringRef key = fact.getName().getValue();
      if (Attribute property = operation->getAttr(key))
        if (property != fact.getValue())
          return operation->emitError()
                 << "sec.reference field '" << key
                 << "' conflicts with the operation property";
      StringRef supplemental;
      if (key == "pinned") supplemental = "sec.pinned";
      else if (key == "origin_storages") supplemental = "sec.origin_storages";
      else if (key == "address_space") supplemental = "sec.reference_address_space";
      if (!supplemental.empty())
        if (Attribute extra = operation->getAttr(supplemental))
          if (extra != fact.getValue())
            return operation->emitError()
                   << "sec.reference field '" << key
                   << "' conflicts with " << supplemental;
    }
    return success();
  }
  if (name == "sec.reference_address_space") {
    auto space = dyn_cast<StringAttr>(value);
    if (!space || space.getValue() != "mmio")
      return operation->emitError(
          "sec.reference_address_space is omitted for ordinary memory and otherwise mmio");
    return success();
  }
  if (name == "sec.origin_storages") {
    auto array = dyn_cast<ArrayAttr>(value);
    if (!array || !llvm::all_of(array, [](Attribute origin) {
          auto integer = dyn_cast<IntegerAttr>(origin);
          return integer && integer.getType().isInteger(32);
        }))
      return operation->emitError("sec.origin_storages must be an array of i32");
    return success();
  }
  if (name == "sec.return_origins") {
    auto function = dyn_cast<func::FuncOp>(operation);
    auto array = dyn_cast<ArrayAttr>(value);
    if (!function || !array || array.empty() ||
        !llvm::all_of(array, [](Attribute origin) {
          auto string = dyn_cast<StringAttr>(origin);
          return string && !string.getValue().empty();
        }))
      return operation->emitError(
          "sec.return_origins must be a non-empty array of origin strings on func.func");
    // MD-084 section 2(3): a non-reference result has no reference origins.
    if (!llvm::any_of(function.getResultTypes(), isReferenceValueType))
      return operation->emitError(
          "sec.return_origins requires a function with a reference result");
    // MD-084 § 4: every entry follows the canonical origin grammar and the
    // array is a finite set without duplicate entries.
    llvm::StringSet<> seen;
    for (Attribute origin : array) {
      StringRef text = cast<StringAttr>(origin).getValue();
      auto parsedOrigin = sec::parseReturnOrigin(text);
      if (!parsedOrigin)
        return operation->emitError()
               << "sec.return_origins entry \"" << text
               << "\" is not a canonical origin (param#N, receiver or "
                  "static@N followed by .field#N, [N] or [*] projections)";
      // MD-115 §1(4),§6(5): only schema 12 admits range projections.
      auto module = operation->getParentOfType<ModuleOp>();
      auto version = module ? module->getAttrOfType<IntegerAttr>("sec.dialect_version") : IntegerAttr();
      if ((!version || version.getInt() < 12) && llvm::any_of(parsedOrigin->projections, [](const sec::ReturnOriginProjection &p) {
          return p.kind == sec::ReturnOriginProjection::Kind::Range || p.kind == sec::ReturnOriginProjection::Kind::UnknownRange;
        })) return operation->emitError("returned slice range projections require schema 12");
      if (!seen.insert(text).second)
        return operation->emitError()
               << "sec.return_origins contains duplicate entry \"" << text
               << "\"";
    }
    return success();
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
