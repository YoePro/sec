// Type-level destruction plan verification (dialect v13 sections 23, 34).
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 19-27, 116, 123;
// rules/memory/destruction.md — sections 10.3, 15.4.
#include "DestructionFacts.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/Pass/Pass.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/StringMap.h"
#include "llvm/ADT/StringSet.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYDESTRUCTIONPLANS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {
struct Plan {
  StringRef id;
  Type type;
  StringRef kind;
  DictionaryAttr entry;
};

// selfRooted reports a Place projected from the free function's own receiver.
bool selfRooted(Value place, Value self) {
  while (Operation *op = place.getDefiningOp()) {
    if (auto field = dyn_cast<sec::PlaceFieldOp>(op))
      place = field.getBase();
    else if (auto payload = dyn_cast<sec::PlaceUnionPayloadOp>(op))
      place = payload.getBase();
    else if (auto element = dyn_cast<sec::PlaceArrayElementOp>(op))
      place = element.getBase();
    else if (auto deref = dyn_cast<sec::PlaceDerefOp>(op))
      place = deref.getReference();
    else
      return false;
  }
  return place == self;
}

class Verifier {
public:
  explicit Verifier(ModuleOp module) : module(module), facts(module) {}

  LogicalResult run() {
    if (failed(collect()))
      return failure();
    for (const Plan &plan : plans)
      if (failed(checkPlan(plan)))
        return failure();
    WalkResult uses = module.walk([&](Operation *op) {
      return succeeded(checkUse(op)) ? WalkResult::advance() : WalkResult::interrupt();
    });
    return failure(uses.wasInterrupted());
  }

private:
  LogicalResult collect() {
    auto table = module->getAttrOfType<ArrayAttr>("sec.destruction_plans");
    if (!table)
      return success();
    llvm::DenseSet<Type> planned;
    for (Attribute attribute : table) {
      auto entry = dyn_cast<DictionaryAttr>(attribute);
      auto id = entry ? entry.getAs<StringAttr>("id") : StringAttr();
      auto type = entry ? entry.getAs<TypeAttr>("type") : TypeAttr();
      auto kind = entry ? entry.getAs<StringAttr>("kind") : StringAttr();
      if (!id || id.getValue().empty() || !type || !kind)
        return module.emitError("a destruction plan requires an id, a type and a kind");
      if (!byID.try_emplace(id.getValue(), plans.size()).second)
        return module.emitError() << "duplicate destruction plan '" << id.getValue() << "'";
      // Identity is type-level: two plans for one type would let a value's
      // destruction depend on which identity a producer named (§ 20).
      if (!planned.insert(type.getValue()).second)
        return module.emitError() << "destruction plan '" << id.getValue() << "' is a second plan for " << type.getValue();
      plans.push_back({id.getValue(), type.getValue(), kind.getValue(), entry});
    }
    return success();
  }

  bool needsDestruction(Type type) { return facts.needsDestruction(type); }

  InFlightDiagnostic error(const Plan &plan) {
    return module.emitError() << "destruction plan '" << plan.id << "' ";
  }

  LogicalResult checkStep(const Plan &plan, StringAttr id, Type expected) {
    auto target = id ? byID.find(id.getValue()) : byID.end();
    if (target == byID.end())
      return error(plan) << "steps to an unknown plan";
    if (plans[target->second].type != expected)
      return error(plan) << "steps to '" << id.getValue() << "', which does not destroy " << expected;
    return success();
  }

  LogicalResult checkPlan(const Plan &plan) {
    DictionaryAttr entry = plan.entry;
    if ((plan.kind != "struct" && entry.contains("fields")) ||
        (plan.kind != "fixed-array" && (entry.contains("element") || entry.contains("element_count"))) ||
        (plan.kind != "active-variant" && entry.contains("variants")) ||
        (plan.kind != "compiler-known-resource" && entry.contains("resource")))
      return error(plan) << "carries steps of another kind";
    if (plan.kind != "struct" && (entry.contains("custom_free") || entry.contains("free_type") || entry.contains("free_function")))
      return error(plan) << "has custom free, which belongs to a struct plan";
    if (plan.kind == "trivial") {
      if (needsDestruction(plan.type))
        return error(plan) << "is trivial for a type that needs destruction";
      return success();
    }
    if (plan.kind == "struct")
      return checkStruct(plan);
    if (plan.kind == "fixed-array")
      return checkArray(plan);
    if (plan.kind == "active-variant")
      return checkVariants(plan);
    if (plan.kind == "compiler-known-resource") {
      auto resource = entry.getAs<StringAttr>("resource");
      if (!isa<sec::StructType>(sec::stripNominal(plan.type)) || !resource || resource.getValue().empty())
        return error(plan) << "needs a struct type and a named release contract";
      return success();
    }
    return error(plan) << "has unsupported kind '" << plan.kind << "'";
  }

  // Remaining fields run in reverse declaration order and every non-trivial
  // field is covered exactly once (§ 23).
  LogicalResult checkStruct(const Plan &plan) {
    auto structure = dyn_cast<sec::StructType>(sec::stripNominal(plan.type));
    if (!structure)
      return error(plan) << "destroys a non-struct";
    auto steps = plan.entry.getAs<ArrayAttr>("fields");
    llvm::SmallDenseSet<uint32_t> listed;
    int64_t previous = structure.getFields().size();
    for (Attribute attribute : steps ? steps.getValue() : ArrayRef<Attribute>()) {
      auto step = dyn_cast<DictionaryAttr>(attribute);
      auto index = step ? step.getAs<IntegerAttr>("field") : IntegerAttr();
      if (!index || index.getInt() < 0 || index.getInt() >= previous)
        return error(plan) << "must list existing fields once in reverse declaration order";
      previous = index.getInt();
      listed.insert(previous);
      auto field = cast<sec::StructFieldAttr>(structure.getFields()[previous]);
      if (failed(checkStep(plan, step.getAs<StringAttr>("plan"), field.getType())))
        return failure();
    }
    for (auto [position, attribute] : llvm::enumerate(structure.getFields()))
      if (!listed.contains(position) && needsDestruction(cast<sec::StructFieldAttr>(attribute).getType()))
        return error(plan) << "omits non-trivial field #" << position;
    bool customFree = false;
    if (auto flag = plan.entry.getAs<BoolAttr>("custom_free"))
      customFree = flag.getValue();
    if (!customFree) {
      if (plan.entry.contains("free_type") || plan.entry.contains("free_function"))
        return error(plan) << "names a free function without custom free";
      if (listed.empty())
        return error(plan) << "has no custom free and no field steps, so it is trivial";
      return success();
    }
    return checkFree(plan);
  }

  // A free takes exactly a mutable reference to the destroyed type. Owned fields
  // stay compiler-managed: the body cannot destroy or move them out, or release its
  // own receiver again through a recursive free (destruction.md § 15.4).
  LogicalResult checkFree(const Plan &plan) {
    auto freeType = plan.entry.getAs<StringAttr>("free_type");
    auto symbol = plan.entry.getAs<FlatSymbolRefAttr>("free_function");
    if (!freeType || freeType.getValue().empty() || !symbol)
      return error(plan) << "has custom free without its source type and function";
    auto free = module.lookupSymbol<func::FuncOp>(symbol);
    if (!free)
      return error(plan) << "names an unknown free function @" << symbol.getValue();
    FunctionType signature = free.getFunctionType();
    auto receiver = signature.getNumInputs() == 1 ? dyn_cast<sec::RefMutType>(signature.getInput(0)) : sec::RefMutType();
    if (!receiver || receiver.getElementType() != plan.type || signature.getNumResults() != 0)
      return error(plan) << "needs a free function over a mutable reference to its type, returning nothing";
    if (free.isExternal())
      return success();
    Value self = free.getArgument(0);
    WalkResult body = free.walk([&](Operation *op) {
      Value place;
      if (auto destroy = dyn_cast<sec::OwnDestroyPlaceOp>(op))
        place = destroy.getPlace();
      else if (auto move = dyn_cast<sec::OwnMoveFromPlaceOp>(op))
        place = move.getPlace();
      if (place && selfRooted(place, self)) {
        op->emitOpError("ends a part of the value its custom free is destroying");
        return WalkResult::interrupt();
      }
      auto call = dyn_cast<func::CallOp>(op);
      if (call && call.getCallee() == free.getSymName() && call.getNumOperands() == 1 && call.getOperand(0) == self) {
        op->emitOpError("frees the value its custom free is already destroying");
        return WalkResult::interrupt();
      }
      return WalkResult::advance();
    });
    return failure(body.wasInterrupted());
  }

  // One element plan and the exact count: a large array stays one record (§ 24).
  LogicalResult checkArray(const Plan &plan) {
    auto array = dyn_cast<sec::ArrayType>(sec::stripNominal(plan.type));
    auto count = plan.entry.getAs<StringAttr>("element_count");
    if (!array || !count || count != array.getLength())
      return error(plan) << "needs a fixed-array type and its exact element count";
    return checkStep(plan, plan.entry.getAs<StringAttr>("element"), array.getElementType());
  }

  // Only the active payload runs, so every variant that may hold a non-trivial
  // payload is covered once. A Result's variants are Ok and Err, in that order.
  // A named-field variant has no single payload plan yet; its producer is a later
  // P17 slice, so it is neither accepted nor silently skipped (§§ 25, 26).
  LogicalResult checkVariants(const Plan &plan) {
    SmallVector<Type> payloads;
    SmallVector<bool> fieldsNeedDestruction;
    Type shape = sec::stripNominal(plan.type);
    if (auto result = dyn_cast<sec::ResultType>(shape)) {
      payloads = {result.getSuccessType(), result.getErrorType()};
      fieldsNeedDestruction.assign(2, false);
    } else if (auto sum = dyn_cast<sec::UnionType>(shape)) {
      for (Attribute attribute : sum.getVariants()) {
        auto variant = cast<sec::UnionVariantAttr>(attribute);
        payloads.push_back(variant.getPayload());
        bool fields = false;
        for (Attribute field : variant.getFields())
          fields = fields || needsDestruction(cast<sec::UnionFieldAttr>(field).getType());
        fieldsNeedDestruction.push_back(fields);
      }
    } else {
      return error(plan) << "destroys neither a union nor a Result";
    }
    llvm::SmallDenseSet<uint32_t> listed;
    auto steps = plan.entry.getAs<ArrayAttr>("variants");
    for (Attribute attribute : steps ? steps.getValue() : ArrayRef<Attribute>()) {
      auto step = dyn_cast<DictionaryAttr>(attribute);
      auto index = step ? step.getAs<IntegerAttr>("variant") : IntegerAttr();
      if (!index || index.getInt() < 0 || index.getInt() >= int64_t(payloads.size()) ||
          !payloads[index.getInt()] || !listed.insert(index.getInt()).second)
        return error(plan) << "names an invalid, repeated or payload-free variant";
      if (failed(checkStep(plan, step.getAs<StringAttr>("plan"), payloads[index.getInt()])))
        return failure();
    }
    for (auto [position, payload] : llvm::enumerate(payloads))
      if (!listed.contains(position) && (fieldsNeedDestruction[position] || (payload && needsDestruction(payload))))
        return error(plan) << "omits non-trivial variant #" << position;
    return success();
  }

  // A use names a plan of exactly the destroyed value's type.
  LogicalResult resolveUse(Operation *op, StringRef id, Type type, const Plan *&plan) {
    auto found = byID.find(id);
    if (found == byID.end())
      return op->emitOpError("names unknown destruction plan '") << id << "'";
    plan = &plans[found->second];
    if (plan->type != type)
      return op->emitOpError("names destruction plan '") << id << "', which does not destroy " << type;
    return success();
  }

  static Type placeElement(Value value) {
    if (auto place = dyn_cast<sec::PlaceType>(value.getType()))
      return place.getElementType();
    return value.getType();
  }

  // A skipped path follows the destroyed type's own fields and active payloads,
  // through a plan that permits skipping: custom free rests on complete fields
  // (destruction.md § 10.3).
  LogicalResult checkSkipped(sec::OwnDestroyPlaceOp op, const Plan &plan) {
    auto skipped = op.getSkipped();
    if (!skipped)
      return success();
    auto custom = plan.entry.getAs<BoolAttr>("custom_free");
    if ((custom && custom.getValue()) || (plan.kind != "struct" && plan.kind != "active-variant"))
      return op.emitOpError("skips subobjects through plan '") << plan.id << "', which cannot skip";
    for (Attribute attribute : *skipped) {
      Type current = plan.type;
      SmallVector<StringRef> parts;
      cast<StringAttr>(attribute).getValue().split(parts, '.');
      for (StringRef part : parts) {
        current = sec::stripNominal(current);
        uint32_t index = 0;
        Type next;
        if (part.consume_front("field#") && !part.getAsInteger(10, index)) {
          auto structure = dyn_cast<sec::StructType>(current);
          if (structure && index < structure.getFields().size())
            next = cast<sec::StructFieldAttr>(structure.getFields()[index]).getType();
        } else if (part.consume_front("payload#") && !part.getAsInteger(10, index)) {
          if (auto result = dyn_cast<sec::ResultType>(current)) {
            next = index == 0 ? result.getSuccessType() : index == 1 ? result.getErrorType() : Type();
          } else if (auto sum = dyn_cast<sec::UnionType>(current)) {
            if (index < sum.getVariants().size())
              next = cast<sec::UnionVariantAttr>(sum.getVariants()[index]).getPayload();
          }
        }
        if (!next)
          return op.emitOpError("skips '") << cast<StringAttr>(attribute).getValue() << "', which is not a subobject of the destroyed type";
        current = next;
      }
    }
    return success();
  }

  LogicalResult checkUse(Operation *op) {
    const Plan *plan = nullptr;
    if (auto replace = dyn_cast<sec::OwnReplacePlaceOp>(op))
      return resolveUse(op, replace.getDestructionPlan(), placeElement(replace.getPlace()), plan);
    if (auto destroy = dyn_cast<sec::OwnDestroyValueOp>(op))
      return resolveUse(op, destroy.getDestructionPlan(), destroy.getValue().getType(), plan);
    if (auto track = dyn_cast<sec::CleanupTrackOwnedOp>(op))
      return resolveUse(op, track.getDestructionPlan(), placeElement(track.getTracked()), plan);
    if (auto destroy = dyn_cast<sec::OwnDestroyPlaceOp>(op)) {
      if (failed(resolveUse(op, destroy.getDestructionPlan(), placeElement(destroy.getPlace()), plan)))
        return failure();
      return checkSkipped(destroy, *plan);
    }
    if (auto discard = dyn_cast<sec::OwnDiscardOp>(op)) {
      if (auto id = discard.getDestructionPlan())
        return resolveUse(op, *id, discard.getValue().getType(), plan);
      if (needsDestruction(discard.getValue().getType()))
        return op->emitOpError("discards a value that needs destruction without its plan");
    }
    return success();
  }

  ModuleOp module;
  SmallVector<Plan> plans;
  llvm::StringMap<size_t> byID;
  sec::DestructionFacts facts;
};

class VerifyDestructionPlansPass final
    : public sec::impl::SecVerifyDestructionPlansBase<VerifyDestructionPlansPass> {
public:
  void runOnOperation() override {
    if (failed(Verifier(getOperation()).run()))
      signalPassFailure();
  }
};
} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyDestructionPlansPass() {
  return std::make_unique<VerifyDestructionPlansPass>();
}
