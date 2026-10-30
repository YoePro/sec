// Basic Sec MLIR -> LLVM path: the last Sec-owned stages of the single
// compilation track Sec dialect -> standard MLIR -> LLVM dialect. Each stage
// lowers only constructs whose semantics are fully discharged; everything
// else is rejected by sec-finalize-for-llvm with the construct's name, so a
// later package widens the path by lowering more constructs before that gate
// and never by a fallback (MD-101).
//
// Rules:
//   - rules/compiler/initialization.md — §§ 20–23, 28 command entry and status
//   - rules/errors/panic.md — §§ 8.1, 12(2) root panic, minimal endpoint
//   - rules/mlir/sec_mlir_lowering.md — staged lowering, no hidden fallback

#include "sec/Conversion/SecToLLVM/Passes.h"

#include "mlir/Conversion/ArithToLLVM/ArithToLLVM.h"
#include "mlir/Conversion/ControlFlowToLLVM/ControlFlowToLLVM.h"
#include "mlir/Conversion/FuncToLLVM/ConvertFuncToLLVMPass.h"
#include "mlir/Conversion/MemRefToLLVM/MemRefToLLVM.h"
#include "mlir/Conversion/ReconcileUnrealizedCasts/ReconcileUnrealizedCasts.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Dialect/LLVMIR/LLVMDialect.h"
#include "mlir/IR/Builders.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Interfaces/DataLayoutInterfaces.h"
#include "mlir/Interfaces/FunctionInterfaces.h"
#include "mlir/Pass/PassManager.h"
#include "sec/Conversion/SecIntegerToArith/Passes.h"
#include "sec/Conversion/SecToCore/Passes.h"
#include "sec/Dialect/Sec/SecDialect.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "sec/Dialect/Sec/TypeTraversal.h"
#include "llvm/ADT/StringSwitch.h"

namespace sec {
#define GEN_PASS_DEF_SECCHECKLOWERINGSUPPORT
#define GEN_PASS_DEF_SECLOWERFAILUREENDPOINTS
#define GEN_PASS_DEF_SECLOWERCOMMANDENTRY
#define GEN_PASS_DEF_SECLOWERSTATICSTORAGE
#define GEN_PASS_DEF_SECFINALIZEFORLLVM
#include "sec/Conversion/SecToLLVM/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {

bool isSecType(Type type) {
  bool found = false;
  type.walk([&](Type nested) {
    if (isa<sec::SecDialect>(nested.getDialect())) {
      found = true;
      return WalkResult::interrupt();
    }
    return WalkResult::advance();
  });
  return found;
}

// The i8 code of an arithmetic failure reason. Reporting the reason is a
// later panic-policy stage; the code keeps it available until then.
std::optional<int64_t> failureReasonCode(StringRef reason) {
  return llvm::StringSwitch<std::optional<int64_t>>(reason)
      .Case("none", 0)
      .Case("overflow", 1)
      .Case("division-by-zero", 2)
      .Case("invalid-shift", 3)
      .Default(std::nullopt);
}

bool isFailureTerminator(Operation *operation) {
  return isa<sec::FailArithmeticOp, sec::FailBoundsOp, sec::FailPanicOp,
             sec::FailReferenceGenerationOp, sec::UnreachableOp>(operation);
}

// The complete currently represented P20 float operation set. Registration
// grants this native track only; schema/conversion/target-capability gates
// remain independent responsibilities.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§63–69,77–84;
// codex.md — "One compilation track" (MD-101).
bool isLoweredSecFloatOperation(Operation *operation) {
  return isa<sec::ConstFloatOp, sec::FloatNegOp, sec::FloatBinaryOp,
             sec::FloatCmpOp, sec::FloatAbsOp, sec::FloatMinimumOp,
             sec::FloatMaximumOp, sec::FloatClassifyOp, sec::FloatRoundOp,
             sec::FloatPropertyOp, sec::FloatClampCheckedOp>(operation);
}

// A Result or union this track represents natively: every payload position
// is a resolved scalar, an ordinary integer enum, or again such a Result or
// union (layout.md §§ 15-16; sec-lower-tagged-values).
bool isScalarTaggedPayload(Type type);

bool isScalarTaggedType(Type type) {
  if (auto result = dyn_cast<sec::ResultType>(type))
    return isScalarTaggedPayload(result.getSuccessType()) &&
           isScalarTaggedPayload(result.getErrorType());
  auto union_ = dyn_cast<sec::UnionType>(type);
  if (!union_)
    return false;
  for (Attribute attribute : union_.getVariants()) {
    auto variant = cast<sec::UnionVariantAttr>(attribute);
    if (Type payload = variant.getPayload(); payload && !isScalarTaggedPayload(payload))
      return false;
    for (Attribute field : variant.getFields())
      if (!isScalarTaggedPayload(cast<sec::UnionFieldAttr>(field).getType()))
        return false;
  }
  return true;
}

bool isScalarTaggedPayload(Type type) {
  if (auto enumType = dyn_cast<sec::EnumType>(type))
    return enumType.getRepresentation() == "integer" &&
           isa<IntegerType, sec::IntType, sec::UIntType>(enumType.getUnderlying());
  return isa<sec::IntType, sec::UIntType, sec::FloatType, sec::CharType,
             sec::RuneType, IntegerType, Float32Type, Float64Type>(type) ||
         isScalarTaggedType(type);
}

// The Sec operations the track lowers today. Adding lowering for a construct
// adds its operations here; nothing outside this set reaches conversion.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§63–75,79–84;
// rules/mlir/sec_mlir_lowering.md — staged lowering, no hidden fallback.
bool isLoweredSecOperation(Operation *operation) {
  // Only ordinary integer enums have their representation discharged by this
  // slice (rules/memory/layout.md § 14; lowering v7 sections 1–6).
  if (isa<sec::EnumConstantOp, sec::EnumCmpOp, sec::EnumToIntegerOp>(operation)) {
    Type valueType = isa<sec::EnumConstantOp>(operation)
                         ? operation->getResult(0).getType()
                         : operation->getOperand(0).getType();
    auto type = cast<sec::EnumType>(valueType);
    return type.getRepresentation() == "integer";
  }
  // Result and union values with scalar payloads (sec-lower-tagged-values).
  if (isa<sec::ResultOkOp, sec::ResultErrOp, sec::ResultIsErrOp,
          sec::ResultUnwrapOkOp, sec::ResultUnwrapErrOp, sec::UnionConstructOp,
          sec::UnionIsVariantOp, sec::UnionUnwrapPayloadOp,
          sec::UnionUnwrapFieldOp>(operation)) {
    Type tagged = isa<sec::ResultOkOp, sec::ResultErrOp, sec::UnionConstructOp>(operation)
                      ? operation->getResult(0).getType()
                      : operation->getOperand(0).getType();
    return isScalarTaggedType(tagged);
  }
  // Storage of a Result or union needs its value representation too.
  if (isa<sec::StorageDeclareOp, sec::StorageInitOp, sec::StorageLoadOp,
          sec::StorageStoreOp>(operation)) {
    Value storage = isa<sec::StorageDeclareOp>(operation)
                        ? operation->getResult(0)
                        : operation->getOperand(0);
    Type element = cast<sec::StorageType>(storage.getType()).getElementType();
    if (isa<sec::ResultType, sec::UnionType>(element))
      return isScalarTaggedType(element);
  }
  return isLoweredSecFloatOperation(operation) ||
         isa<sec::ConstIntOp, sec::ConstBoolOp, sec::StorageDeclareOp,
             sec::StorageInitOp, sec::StorageLoadOp, sec::StorageStoreOp,
             sec::CallDirectOp, sec::IntUnaryPlusOp, sec::IntNegCheckedOp,
             sec::IntBinaryCheckedOp, sec::IntBitNotOp, sec::IntBitwiseOp,
             sec::IntShiftCheckedOp, sec::IntCmpOp, sec::IntBitExtractOp,
             sec::NumericConvertExactOp,
             sec::NumericConvertCheckedOp,
             sec::ArithmeticFailureReasonConstantOp, sec::FailArithmeticOp,
             sec::FailPanicOp, sec::StaticGlobalOp, sec::StaticLoadOp,
             sec::StaticStoreOp,
             sec::UnreachableOp>(operation);
}

struct CheckLoweringSupportPass
    : sec::impl::SecCheckLoweringSupportBase<CheckLoweringSupportPass> {
  void runOnOperation() override {
    bool failed = false;
    getOperation().walk([&](Operation *operation) {
      // Recognizing the MD-077 type syntax does not select a physical
      // representation or enable hardware lowering. Check signatures, nested
      // attributes and block arguments as well as SSA operands/results before
      // conversion can fail with a materialization error.
      // Rules: rules/mlir/sec_mlir_dialect.md, v11.41.1 "Types" and
      // "Effects and verification"; codex.md, "One compilation track".
      Type unsupportedRegisterType = sec::findOperationType(
          operation, [](Type type) {
            return isa<sec::RegisterType, sec::HardwareInstanceType>(type);
          });
      if (unsupportedRegisterType) {
        operation->emitError()
            << "[unsupported-lowering] the Sec MLIR -> LLVM track does not lower type "
            << unsupportedRegisterType << " yet; register/hardware physical lowering requires a verified representation and target plan";
        failed = true;
      }
      // Lexeme-only constants can round-trip historically, but have no
      // canonical physical value for P20. Reject at entry, before rewriting.
      // Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§18,76,84.
      if (auto constant = dyn_cast<sec::ConstFloatOp>(operation)) {
        if (!constant.getBitsAttr()) {
          constant.emitError()
              << "[unsupported-lowering] sec.const.float requires canonical bits before native lowering";
          failed = true;
        }
      }
      if (isa<sec::SecDialect>(operation->getDialect()) &&
          !isLoweredSecOperation(operation)) {
        operation->emitError()
            << "[unsupported-lowering] the Sec MLIR -> LLVM track does not lower '"
            << operation->getName() << "' yet";
        failed = true;
      }
    });
    if (failed)
      signalPassFailure();
  }
};

struct LowerFailureEndpointsPass
    : sec::impl::SecLowerFailureEndpointsBase<LowerFailureEndpointsPass> {
  void runOnOperation() override {
    ModuleOp module = getOperation();
    MLIRContext *context = module.getContext();
    Type code = IntegerType::get(context, 8);

    // A reason may flow only to a failure endpoint here; any other Sec
    // consumer (for example a Result error mapping) is not lowered yet.
    bool failed = false;
    auto checkReasonUses = [&](Value value) {
      for (Operation *user : value.getUsers()) {
        if (isa<sec::SecDialect>(user->getDialect()) &&
            !isa<sec::FailArithmeticOp>(user)) {
          user->emitError() << "the Sec MLIR -> LLVM track does not "
                               "lower '"
                            << user->getName()
                            << "' over an arithmetic failure reason yet";
          failed = true;
        }
      }
    };
    module.walk([&](Operation *operation) {
      for (Region &region : operation->getRegions())
        for (Block &block : region)
          for (BlockArgument argument : block.getArguments())
            if (isa<sec::ArithmeticFailureReasonType>(argument.getType()))
              checkReasonUses(argument);
      for (Value result : operation->getResults())
        if (isa<sec::ArithmeticFailureReasonType>(result.getType()))
          checkReasonUses(result);
    });
    if (failed)
      return signalPassFailure();

    SmallVector<sec::ArithmeticFailureReasonConstantOp> constants;
    module.walk([&](sec::ArithmeticFailureReasonConstantOp constant) {
      constants.push_back(constant);
    });
    for (sec::ArithmeticFailureReasonConstantOp constant : constants) {
      std::optional<int64_t> value = failureReasonCode(constant.getValue());
      if (!value) {
        constant.emitError("unknown arithmetic failure reason");
        return signalPassFailure();
      }
      OpBuilder builder(constant);
      Value replacement = builder.create<arith::ConstantOp>(
          constant.getLoc(), code, builder.getIntegerAttr(code, *value));
      constant.getResult().replaceAllUsesWith(replacement);
      constant.erase();
    }
    module.walk([&](Operation *operation) {
      for (Region &region : operation->getRegions())
        for (Block &block : region)
          for (BlockArgument argument : block.getArguments())
            if (isa<sec::ArithmeticFailureReasonType>(argument.getType()))
              argument.setType(code);
      for (Value result : operation->getResults())
        if (isa<sec::ArithmeticFailureReasonType>(result.getType()))
          result.setType(code);
    });

    SmallVector<Operation *> endpoints;
    module.walk([&](Operation *operation) {
      if (isFailureTerminator(operation))
        endpoints.push_back(operation);
    });
    for (Operation *endpoint : endpoints) {
      OpBuilder builder(endpoint);
      builder.create<LLVM::Trap>(endpoint->getLoc());
      builder.create<LLVM::UnreachableOp>(endpoint->getLoc());
      endpoint->erase();
    }
  }
};

struct LowerCommandEntryPass
    : sec::impl::SecLowerCommandEntryBase<LowerCommandEntryPass> {
  using SecLowerCommandEntryBase::SecLowerCommandEntryBase;

  void runOnOperation() override {
    ModuleOp module = getOperation();
    func::FuncOp source;
    module.walk([&](func::FuncOp function) {
      auto identity = function->getAttrOfType<StringAttr>("sec.function_id");
      if (identity && identity.getValue() == entry)
        source = function;
    });
    if (!source) {
      module.emitError() << "command entry '" << entry
                         << "' is not defined in this module";
      return signalPassFailure();
    }
    FunctionType type = source.getFunctionType();
    auto status = type.getNumResults() == 1
                      ? dyn_cast<IntegerType>(type.getResult(0))
                      : IntegerType();
    if (type.getNumInputs() != 0 || !status) {
      source.emitError() << "the Sec MLIR -> LLVM track supports only "
                            "the command entry `fn main() int`, got "
                         << type;
      return signalPassFailure();
    }
    if (SymbolTable::lookupSymbolIn(module, "main")) {
      module.emitError("the platform entry symbol 'main' is already defined");
      return signalPassFailure();
    }

    // The hosted startup adapter maps the logical termination status to the
    // C int of the target (initialization.md § 23 leaves the physical
    // mapping to the execution environment; MD-101).
    MLIRContext *context = module.getContext();
    Type cInt = IntegerType::get(context, 32);
    OpBuilder builder = OpBuilder::atBlockEnd(module.getBody());
    auto adapter = builder.create<func::FuncOp>(
        source.getLoc(), "main", FunctionType::get(context, {}, {cInt}));
    Block *body = adapter.addEntryBlock();
    builder.setInsertionPointToStart(body);
    Value result =
        builder.create<func::CallOp>(source.getLoc(), source, ValueRange{})
            .getResult(0);
    if (status.getWidth() > 32)
      result = builder.create<arith::TruncIOp>(source.getLoc(), cInt, result);
    else if (status.getWidth() < 32)
      result = builder.create<arith::ExtSIOp>(source.getLoc(), cInt, result);
    builder.create<func::ReturnOp>(source.getLoc(), result);
  }
};

// The resolved representation of a static: signless builtin integers keep
// their width, bool is i1, and target-sized int/uint take the plan's index
// width (rules/memory/layout.md; the data layout carries the plan).
Type resolvedStaticType(Type type, unsigned indexBits) {
  MLIRContext *context = type.getContext();
  if (auto integer = dyn_cast<IntegerType>(type))
    return IntegerType::get(context, integer.getWidth());
  if (isa<sec::IntType, sec::UIntType>(type))
    return IntegerType::get(context, indexBits);
  return Type();
}

struct LowerStaticStoragePass
    : sec::impl::SecLowerStaticStorageBase<LowerStaticStoragePass> {
  void runOnOperation() override {
    ModuleOp module = getOperation();
    MLIRContext *context = module.getContext();
    unsigned indexBits = DataLayout(module).getTypeSizeInBits(IndexType::get(context));
    auto pointer = LLVM::LLVMPointerType::get(context);
    SmallVector<sec::StaticGlobalOp> globals;
    module.walk([&](sec::StaticGlobalOp op) { globals.push_back(op); });
    for (sec::StaticGlobalOp global : globals) {
      // A reference-dependent static keeps its Place until the physical
      // reference representation exists; it is rejected, never erased
      // (MD-116 § 7(6)).
      auto addressTaken = global->getAttrOfType<BoolAttr>("sec.address_taken");
      if (addressTaken && addressTaken.getValue()) {
        global.emitError() << "the Sec MLIR -> LLVM track does not lower "
                              "reference-dependent static storage yet";
        return signalPassFailure();
      }
      Type type = resolvedStaticType(global.getType(), indexBits);
      if (!type) {
        global.emitError() << "the Sec MLIR -> LLVM track does not lower static "
                              "storage of "
                           << global.getType() << " yet";
        return signalPassFailure();
      }
      unsigned width = cast<IntegerType>(type).getWidth();
      StringRef initial = global.getInitial();
      APInt value = initial == "true"    ? APInt(1, 1)
                    : initial == "false" ? APInt(1, 0)
                                         : APInt(width, initial, 10);
      OpBuilder builder(global);
      builder.create<LLVM::GlobalOp>(global.getLoc(), type, !global.getIsMutable(),
                                     LLVM::Linkage::Internal, global.getSymName(),
                                     builder.getIntegerAttr(type, value));
      global.erase();
    }
    SmallVector<Operation *> accesses;
    module.walk([&](Operation *op) {
      if (isa<sec::StaticLoadOp, sec::StaticStoreOp>(op))
        accesses.push_back(op);
    });
    for (Operation *op : accesses) {
      OpBuilder builder(op);
      if (auto load = dyn_cast<sec::StaticLoadOp>(op)) {
        Value address = builder.create<LLVM::AddressOfOp>(load.getLoc(), pointer, load.getGlobal());
        Value value = builder.create<LLVM::LoadOp>(load.getLoc(), load.getResult().getType(), address);
        load.getResult().replaceAllUsesWith(value);
      } else {
        auto store = cast<sec::StaticStoreOp>(op);
        Value address = builder.create<LLVM::AddressOfOp>(store.getLoc(), pointer, store.getGlobal());
        builder.create<LLVM::StoreOp>(store.getLoc(), store.getValue(), address);
      }
      op->erase();
    }
  }
};

struct FinalizeForLLVMPass
    : sec::impl::SecFinalizeForLLVMBase<FinalizeForLLVMPass> {
  void runOnOperation() override {
    ModuleOp module = getOperation();
    bool failed = false;
    auto reject = [&](Operation *operation, const Twine &what) {
      operation->emitError() << "the Sec MLIR -> LLVM track does not "
                                "lower "
                             << what << " yet";
      failed = true;
    };
    module.walk([&](Operation *operation) {
      if (isa<sec::SecDialect>(operation->getDialect())) {
        reject(operation, "'" + operation->getName().getStringRef() + "'");
        return;
      }
      for (Type type : operation->getResultTypes())
        if (isSecType(type)) {
          std::string text;
          llvm::raw_string_ostream(text) << type;
          reject(operation, "values of type " + text);
          return;
        }
      for (Region &region : operation->getRegions())
        for (Block &block : region)
          for (BlockArgument argument : block.getArguments())
            if (isSecType(argument.getType())) {
              std::string text;
              llvm::raw_string_ostream(text) << argument.getType();
              reject(operation, "values of type " + text);
              return;
            }
      if (auto function = dyn_cast<FunctionOpInterface>(operation))
        if (isSecType(function.getFunctionType())) {
          reject(operation, "this function signature");
          return;
        }
    });
    if (failed)
      return signalPassFailure();

    MLIRContext *context = module.getContext();
    if (auto triple = module->getAttrOfType<StringAttr>("sec.target_triple"))
      module->setAttr(LLVM::LLVMDialect::getTargetTripleAttrName(), triple);
    auto stripped = [](Operation *operation) {
      SmallVector<StringAttr> names;
      for (NamedAttribute attribute : operation->getDiscardableAttrs())
        if (attribute.getName().strref().starts_with("sec."))
          names.push_back(attribute.getName());
      for (StringAttr name : names)
        operation->removeDiscardableAttr(name);
    };
    auto strippedDictionary = [&](DictionaryAttr dictionary) {
      SmallVector<NamedAttribute> kept;
      for (NamedAttribute attribute : dictionary)
        if (!attribute.getName().strref().starts_with("sec."))
          kept.push_back(attribute);
      return DictionaryAttr::get(context, kept);
    };
    module.walk([&](Operation *operation) {
      stripped(operation);
      if (auto function = dyn_cast<func::FuncOp>(operation)) {
        for (unsigned index = 0; index < function.getNumArguments(); ++index)
          if (DictionaryAttr attributes = function.getArgAttrDict(index))
            function.setArgAttrs(index, strippedDictionary(attributes));
        for (unsigned index = 0; index < function.getNumResults(); ++index)
          if (DictionaryAttr attributes = function.getResultAttrDict(index))
            function.setResultAttrs(index, strippedDictionary(attributes));
      }
    });
  }
};

} // namespace

namespace {
// An empty entry lowers a module without a program entry (library or
// firmware targets are not supported by the basic path yet).
struct LowerToLLVMOptions : public PassPipelineOptions<LowerToLLVMOptions> {
  Option<std::string> entry{*this, "entry",
                            llvm::cl::desc("Sec function identity of the "
                                           "command entry; empty for none"),
                            llvm::cl::init("main::main()")};
};
} // namespace

void sec::registerSecToLLVMPipelines() {
  static PassPipelineRegistration<LowerToLLVMOptions> pipeline(
      "sec-lower-to-llvm",
      "Lower verified Sec MLIR through standard MLIR to the LLVM dialect",
      [](OpPassManager &manager, const LowerToLLVMOptions &options) {
        manager.addPass(sec::createSecCheckLoweringSupport());
        manager.addPass(sec::createSecLowerTrivialCorePass());
        manager.addPass(sec::createSecResolveScalarLayoutPass());
        manager.addPass(sec::createSecLowerIntegerEnums());
        manager.addPass(sec::createSecLowerFloats());
        manager.addPass(sec::createSecLowerTrivialCorePass());
        manager.addPass(sec::createSecLowerCheckedIntegersPass());
        manager.addPass(sec::createSecLowerStaticStorage());
        manager.addPass(sec::createSecLowerFailureEndpoints());
        if (!options.entry.getValue().empty())
          manager.addPass(sec::createSecLowerCommandEntry(
              sec::SecLowerCommandEntryOptions{options.entry.getValue()}));
        manager.addPass(sec::createSecLowerTaggedValues());
        manager.addPass(sec::createSecFinalizeForLLVM());
        manager.addPass(createArithToLLVMConversionPass());
        manager.addPass(createConvertControlFlowToLLVMPass());
        manager.addPass(createFinalizeMemRefToLLVMConversionPass());
        manager.addPass(createConvertFuncToLLVMPass());
        manager.addPass(createReconcileUnrealizedCastsPass());
      });
  (void)pipeline;
}
