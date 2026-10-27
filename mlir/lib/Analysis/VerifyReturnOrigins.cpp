// Canonical returned-reference/slice origin verification (MD-084/085/115/116).
// Rules: rules/mlir/sec_mlir_dialect.md — v11.37, v11.41.3, v12.34;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§66–67,83–86.
#include "ReferenceFlow.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/SymbolTable.h"
#include "mlir/Interfaces/ControlFlowInterfaces.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "sec/Dialect/Sec/SecAttributes.h"
#include "sec/Dialect/Sec/ReturnOrigins.h"
#include "llvm/ADT/SmallString.h"
#include <set>
#include <string>
using namespace mlir;
using namespace sec::detail;
namespace {
bool isReference(Type type) {
  return isa<sec::RefType,sec::RefMutType,sec::SliceType,sec::SliceMutType>(type);
}
// MD-085: structural, path-aware return-origin verification. Origins use the
// canonical MD-084 grammar; a receiver is parameter 0 of an instance method
// (sec.parameter_names[0] == "self") and is compared as param#0. The walk
// mirrors the Semantic IR verifier: copies, moves, borrows, reborrows and
// dereferences keep the path; a field Place adds .field#N, a proven-safe
// constant element [K] and any other element [*]; a slice view keeps its
// parent's origin and its element is [*]; block arguments follow every
// predecessor with visited states; a direct call contributes only the callee's
// own summary instantiated over the actual operands (caller prefix + callee
// suffix). Every observed origin must be listed exactly. Sema stays the only
// source-level borrow/lifetime authority; nothing is guessed or patched.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.37 amendment (MD-085);
// rules/mlir/packages/sec-mlir-dialect_package15.md — §§ 61–62, 85;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§ 63–69.

bool hasReceiver(func::FuncOp function) {
  auto names = function->getAttrOfType<ArrayAttr>("sec.parameter_names");
  if (!names || names.empty())
    return false;
  auto first = dyn_cast<StringAttr>(names[0]);
  return first && first.getValue() == "self";
}

std::string projectionText(const sec::ReturnOriginProjection &projection) {
  switch (projection.kind) {
  case sec::ReturnOriginProjection::Kind::Field:
    return ".field#" + std::to_string(projection.value);
  case sec::ReturnOriginProjection::Kind::Index:
    return "[" + std::to_string(projection.value) + "]";
  case sec::ReturnOriginProjection::Kind::AnyIndex:
    return "[*]";
  case sec::ReturnOriginProjection::Kind::Range:
    return "[" + projection.start + "..<" + projection.end + "]";
  case sec::ReturnOriginProjection::Kind::UnknownRange:
    return "[?]";
  }
  return "";
}

std::string projectionsText(const sec::ReturnOrigin &origin) {
  std::string text;
  for (const sec::ReturnOriginProjection &projection : origin.projections)
    text += projectionText(projection);
  return text;
}

// The referent a projection path starts from: the element of a direct
// reference, or the element sequence of a slice view.
Type originRootReferent(Type type, bool *sequence) {
  *sequence = false;
  if (auto reference = dyn_cast<sec::RefType>(type))
    return reference.getElementType();
  if (auto reference = dyn_cast<sec::RefMutType>(type))
    return reference.getElementType();
  if (auto slice = dyn_cast<sec::SliceType>(type)) {
    *sequence = true;
    return slice.getElementType();
  }
  if (auto slice = dyn_cast<sec::SliceMutType>(type)) {
    *sequence = true;
    return slice.getElementType();
  }
  return Type();
}

// The type-aware admissibility of a projection path (MD-084 § 4(4),
// MD-085 § 2(3)): fields of structs, indexes of fixed arrays within their
// extent (or of a slice view's element sequence).
bool admissiblePath(Type current, bool sequence,
                    const sec::ReturnOrigin &origin, Type *finalType = nullptr,
                    bool *finalSequence = nullptr) {
  if (!current)
    return false;
  for (const sec::ReturnOriginProjection &projection : origin.projections) {
    if (auto slice = dyn_cast<sec::SliceType>(current)) {current=slice.getElementType();sequence=true;}
    if (auto slice = dyn_cast<sec::SliceMutType>(current)) {current=slice.getElementType();sequence=true;}
    if (projection.kind == sec::ReturnOriginProjection::Kind::Range || projection.kind == sec::ReturnOriginProjection::Kind::UnknownRange) {
      auto array = dyn_cast<sec::ArrayType>(current);
      if (!sequence && !array) return false;
      if (array && projection.kind == sec::ReturnOriginProjection::Kind::Range && sec::detail::naturalLess(array.getLength().getValue(), projection.end)) return false;
      if (array && !sequence) current = array.getElementType();
      sequence = true;
      continue;
    }
    if (projection.kind == sec::ReturnOriginProjection::Kind::Field) {
      auto structure = dyn_cast<sec::StructType>(current);
      if (sequence || !structure)
        return false;
      Type fieldType;
      for (Attribute attribute : structure.getFields())
        if (auto field = dyn_cast<sec::StructFieldAttr>(attribute))
          if (field.getOrdinal() == projection.value)
            fieldType = field.getType();
      if (!fieldType)
        return false;
      current = fieldType;
      continue;
    }
    if (sequence) {
      sequence = false;
      continue;
    }
    auto array = dyn_cast<sec::ArrayType>(current);
    if (!array)
      return false;
    if (projection.kind == sec::ReturnOriginProjection::Kind::Index) {
      uint64_t extent = 0;
      if (!array.getLength().getValue().getAsInteger(10, extent) &&
          projection.value >= extent)
        return false;
    }
    current = array.getElementType();
  }
  if (finalType) *finalType = current;
  if (finalSequence) *finalSequence = sequence;
  return true;
}

bool admissibleProjections(Type root, const sec::ReturnOrigin &origin) {
  bool sequence = false;
  Type current = originRootReferent(root, &sequence);
  return admissiblePath(current, sequence, origin);
}

// The static an explicit sec.static_id names in the function's module
// (MD-116 § 2(4)); symbol names and source names are never parsed.
sec::StaticGlobalOp staticWithID(Operation *anchor, uint32_t id) {
  sec::StaticGlobalOp found;
  if (auto module = anchor->getParentOfType<ModuleOp>())
    module.walk([&](sec::StaticGlobalOp global) {
      auto attribute = global->getAttrOfType<IntegerAttr>("sec.static_id");
      if (attribute && attribute.getValue().getZExtValue() == id)
        found = global;
    });
  return found;
}

// The parameter an origin root names (receiver = parameter 0).
std::optional<unsigned> originRootParameter(const sec::ReturnOrigin &origin) {
  if (origin.root == sec::ReturnOrigin::Root::Receiver)
    return 0u;
  if (origin.root == sec::ReturnOrigin::Root::Parameter)
    return origin.rootIndex;
  return std::nullopt;
}

std::optional<uint64_t> constantIndex(Value index) {
  while (auto cast = index.getDefiningOp<UnrealizedConversionCastOp>()) {
    if (cast->getNumOperands() != 1)
      return std::nullopt;
    index = cast->getOperand(0);
  }
  if (auto constant = index.getDefiningOp<sec::ConstIntOp>()) {
    uint64_t value = 0;
    if (constant.getValue().getAsInteger(10, value))
      return std::nullopt;
    return value;
  }
  if (auto constant = index.getDefiningOp<arith::ConstantOp>())
    if (auto integer = dyn_cast<IntegerAttr>(constant.getValue()))
      if (!integer.getValue().isNegative())
        return integer.getValue().getZExtValue();
  return std::nullopt;
}

// MD-115 §§4,7: recover arbitrary-precision endpoint constants from the
// canonical normalization/check tuple, including post-P6 integer constants.
std::optional<std::string> originConstant(Value value, unsigned depth);

// MD-115 §7(5): only unanimous, verified call contracts or actual normalized
// slice lengths establish the current extent. No dynamic bound is fabricated.
std::optional<std::string> originSliceExtent(Value value, unsigned depth) {
  if (depth > 64) return std::nullopt;
  value=sourceValue(value); if (!value) return std::nullopt;
  auto definition=value.getDefiningOp();
  if (isa_and_nonnull<sec::SliceCopySharedOp,sec::SliceMoveMutOp>(definition)) return originSliceExtent(definition->getOperand(0),depth+1);
  if (isa_and_nonnull<sec::SliceBorrowSharedOp,sec::SliceBorrowMutOp,sec::SliceReborrowSharedOp,sec::SliceReborrowMutOp>(definition)) return originConstant(definition->getOperand(2),depth+1);
  if (auto call=dyn_cast_or_null<sec::CallDirectOp>(definition)) {
    auto callee=SymbolTable::lookupNearestSymbolFrom<func::FuncOp>(call,call->getAttrOfType<FlatSymbolRefAttr>("callee"));
    auto origins=callee ? callee->getAttrOfType<ArrayAttr>("sec.return_origins") : ArrayAttr();
    if (!origins || origins.empty()) return std::nullopt;
    std::optional<std::string> common;
    for (auto attribute:origins) {
      auto text=dyn_cast<StringAttr>(attribute);auto origin=text?sec::parseReturnOrigin(text.getValue()):std::nullopt;
      if (!origin) return std::nullopt;
      std::optional<std::string> extent;
      if (!origin->projections.empty()) {
        const auto &last=origin->projections.back();
        if (last.kind==sec::ReturnOriginProjection::Kind::Range) extent=sec::detail::subtractNatural(last.end,last.start);
      } else if (origin->root!=sec::ReturnOrigin::Root::Static) {
        unsigned index=origin->root==sec::ReturnOrigin::Root::Receiver?0:origin->rootIndex;
        if (index<call->getNumOperands()) extent=originSliceExtent(call->getOperand(index),depth+1);
      }
      if (!extent || (common && common!=extent)) return std::nullopt;
      common=extent;
    }
    return common;
  }
  return std::nullopt;
}

std::optional<std::string> originConstant(Value value, unsigned depth = 0) {
  if (depth > 64) return std::nullopt;
  value = sourceValue(value);
  if (!value) return std::nullopt;
  if (auto cast = value.getDefiningOp<UnrealizedConversionCastOp>())
    return cast->getNumOperands() == 1 ? originConstant(cast->getOperand(0), depth+1) : std::nullopt;
  if (auto constant = value.getDefiningOp<sec::ConstIntOp>()) {
    auto text = constant.getValue(); auto copy = text;
    auto result = sec::detail::consumeNatural(copy);
    if (result && copy.empty()) return result;
  }
  if (auto constant = value.getDefiningOp<arith::ConstantOp>())
    if (auto integer = dyn_cast<IntegerAttr>(constant.getValue())) {
      // Unsigned bit patterns remain positive after scalar resolution.
      if (!integer.getType().isSignedInteger() || !integer.getValue().isNegative()) {
        llvm::SmallString<40> text; integer.getValue().toString(text,10,integer.getType().isSignedInteger());
        return text.str().str();
      }
    }
  auto next = [&](Value v) { return originConstant(v,depth+1); };
  if (auto len = value.getDefiningOp<sec::SliceLenOp>()) return originSliceExtent(len->getOperand(0),depth+1);
  auto result = dyn_cast<OpResult>(value);
  Operation *range = value.getDefiningOp();
  if (result && isa_and_nonnull<sec::SliceRangeNormalizeOp,sec::SliceRangeCheckOp>(range)) {
    bool sp = range->getAttrOfType<BoolAttr>("start_present").getValue();
    bool ep = range->getAttrOfType<BoolAttr>("end_present").getValue();
    auto lower = sp ? next(range->getOperand(1)) : std::optional<std::string>("0");
    if (result.getResultNumber() == 0) return lower;
    if (result.getResultNumber() != 1) return std::nullopt;
    auto upper = next(range->getOperand(ep ? (sp ? 2 : 1) : 0));
    if (ep && upper && !range->getAttrOfType<BoolAttr>("end_exclusive").getValue()) upper=sec::detail::addNatural(*upper,"1");
    if (upper && lower) return sec::detail::subtractNatural(*upper,*lower);
  }
  return std::nullopt;
}

// Trace a relative slice projection from represented normalized SSA, never
// from declared return metadata. verifySliceGuards separately requires the
// same-source tuple and dominating success/validity proof (MD-115 §7(2–3)).
std::optional<std::string> originRange(Operation *borrow) {
  Value start = sourceValue(borrow->getOperand(1));
  Value length = sourceValue(borrow->getOperand(2));
  Operation *range = start ? start.getDefiningOp() : nullptr;
  if (!range || !isa<sec::SliceRangeNormalizeOp,sec::SliceRangeCheckOp>(range) || range->getResult(0) != start || range->getResult(1) != length) return std::nullopt;
  bool startPresent = range->getAttrOfType<BoolAttr>("start_present").getValue();
  bool endPresent = range->getAttrOfType<BoolAttr>("end_present").getValue();
  if (!startPresent && !endPresent) return std::string();
  auto lower = startPresent ? originConstant(range->getOperand(1)) : std::optional<std::string>("0");
  auto upper = originConstant(range->getOperand(endPresent ? (startPresent ? 2 : 1) : 0));
  if (endPresent && upper && !range->getAttrOfType<BoolAttr>("end_exclusive").getValue()) upper = sec::detail::addNatural(*upper,"1");
  if (lower && upper) {
    if (sec::detail::naturalLess(*upper,*lower)) return std::nullopt;
    return "[" + *lower + "..<" + *upper + "]";
  }
  return std::string("[?]");
}

class ReturnOriginTracer {
public:
  ReturnOriginTracer(func::FuncOp function, func::ReturnOp ret, bool sliceReturn)
      : function(function), ret(ret), entry(&function.getBody().front()),
        sliceReturn(sliceReturn) {}

  // trace collects the origins of value with the projection path tail that
  // lies between value and the returned reference.
  LogicalResult trace(Value value, const std::string &tail,
                      std::set<std::string> &origins) {
    if (tail.size() > 65536 || visited.size() > 4096)
      return ret.emitOpError("unsupported cyclic returned-range composition");
    if (!visited.insert({value.getAsOpaquePointer(), tail}).second)
      return success();
    if (auto argument = dyn_cast<BlockArgument>(value)) {
      Block *owner = argument.getOwner();
      if (owner == entry) {
        return record("param#" + std::to_string(argument.getArgNumber()) + tail, origins);
      }
      for (Block *predecessor : owner->getPredecessors()) {
        auto branch = dyn_cast<BranchOpInterface>(predecessor->getTerminator());
        if (!branch)
          return ret.emitOpError("returned reference has an unknown origin");
        for (unsigned index = 0; index < branch->getNumSuccessors(); ++index)
          if (branch->getSuccessor(index) == owner)
            if (Value incoming =
                    branch.getSuccessorOperands(index)[argument.getArgNumber()])
              if (failed(trace(incoming, tail, origins)))
                return failure();
      }
      return success();
    }
    Operation *definition = value.getDefiningOp();
    if (isa<sec::PlaceStorageOp, sec::PlaceValueOp>(definition))
      return ret.emitOpError(
          "returns a reference to local storage or a local value");
    // A static root contributes static@N from its declaration's explicit
    // identity, never from the symbol spelling (MD-116 § 6(3)).
    if (auto root = dyn_cast<sec::PlaceStaticOp>(definition)) {
      auto global = SymbolTable::lookupNearestSymbolFrom<sec::StaticGlobalOp>(
          root, root.getGlobalAttr());
      auto id = global ? global->getAttrOfType<IntegerAttr>("sec.static_id")
                       : IntegerAttr();
      if (!id)
        return ret.emitOpError(
            "returns a reference to a static without sec.static_id");
      return record("static@" + std::to_string(id.getValue().getZExtValue()) + tail, origins);
    }
    if (isa<sec::RefBorrowSharedOp, sec::RefBorrowMutOp, sec::RefCopySharedOp,
            sec::RefMoveOp, sec::RefReborrowSharedOp, sec::RefReborrowMutOp,
            sec::PlaceDerefOp,
            sec::SliceCopySharedOp, sec::SliceMoveMutOp>(definition))
      return trace(definition->getOperand(0), tail, origins);
    // MD-115: a complete view preserves its current path, not its root;
    // narrowing contributes a proven exact or conservative range projection.
    if (isa<sec::SliceBorrowSharedOp, sec::SliceBorrowMutOp,
            sec::SliceReborrowSharedOp, sec::SliceReborrowMutOp>(definition)) {
      auto range = originRange(definition);
      if (!range) return ret.emitOpError("returned slice lacks a canonical normalized range tuple");
      return trace(definition->getOperand(0), *range + tail, origins);
    }
    if (auto field = dyn_cast<sec::PlaceFieldOp>(definition))
      return trace(field.getBase(),
                   ".field#" + std::to_string(field.getField()) + tail, origins);
    if (auto element = dyn_cast<sec::PlaceArrayElementOp>(definition)) {
      std::string step = "[*]";
      if (element.getBoundsKind() == "proven-safe")
        if (std::optional<uint64_t> index = constantIndex(element.getIndex()))
          step = "[" + std::to_string(*index) + "]";
      return trace(element.getBase(), step + tail, origins);
    }
    if (isa<sec::SliceElementPlaceOp>(definition))
      return trace(definition->getOperand(0), "[*]" + tail, origins);
    if (isa<sec::PlaceUnionPayloadOp>(definition))
      return ret.emitOpError(
          "returns a reference through a union payload, which has no "
          "canonical return-origin projection");
    if (auto call = dyn_cast<sec::CallDirectOp>(definition))
      return traceCall(call, tail, origins);
    return ret.emitOpError("returned reference has an unknown origin");
  }

private:
  // MD-115 §4: canonicalize composed paths only after their original root
  // has been proved. Locals/unknown callees cannot acquire an external root.
  LogicalResult record(const std::string &text, std::set<std::string> &origins) {
    auto parsed = sec::parseReturnOrigin(text);
    auto normalized = parsed ? sec::normalizeReturnOrigin(*parsed) : std::nullopt;
    if (!normalized) return ret.emitOpError("invalid returned range composition");
    std::string root = normalized->root == sec::ReturnOrigin::Root::Static ? "static@" : "param#";
    origins.insert(root + std::to_string(normalized->rootIndex) + projectionsText(*normalized));
    return success();
  }
  // A direct call contributes only its callee's listed origins (MD-085 § 4).
  LogicalResult traceCall(sec::CallDirectOp call, const std::string &tail,
                          std::set<std::string> &origins) {
    auto callee = SymbolTable::lookupNearestSymbolFrom<func::FuncOp>(
        call, call->getAttrOfType<FlatSymbolRefAttr>("callee"));
    auto summary = callee ? callee->getAttrOfType<ArrayAttr>("sec.return_origins")
                          : ArrayAttr();
    if (!callee || !summary || summary.empty())
      return ret.emitOpError()
             << "returns a reference from a call to "
             << (callee ? callee.getSymName() : StringRef("an unresolved callee"))
             << " without a sec.return_origins summary";
    for (Attribute attribute : summary) {
      auto text = dyn_cast<StringAttr>(attribute);
      std::optional<sec::ReturnOrigin> origin =
          text ? sec::parseReturnOrigin(text.getValue()) : std::nullopt;
      // A callee defined in this module names this module's statics; a
      // declaration's static origin cannot be linked and fails closed
      // (MD-116 § 6(4)).
      if (origin && origin->root == sec::ReturnOrigin::Root::Static &&
          !callee.isExternal()) {
        if (failed(record("static@" + std::to_string(origin->rootIndex) + projectionsText(*origin) + tail, origins))) return failure();
        continue;
      }
      std::optional<unsigned> parameter =
          origin ? originRootParameter(*origin) : std::nullopt;
      if (!parameter || *parameter >= call->getNumOperands() ||
          (origin->root == sec::ReturnOrigin::Root::Receiver &&
           !hasReceiver(callee)))
        return ret.emitOpError()
               << "cannot instantiate origin " << attribute << " of callee @"
               << callee.getSymName() << " at this call";
      if (failed(trace(call->getOperand(*parameter),
                       projectionsText(*origin) + tail, origins)))
        return failure();
    }
    return success();
  }

  func::FuncOp function;
  func::ReturnOp ret;
  Block *entry;
  bool sliceReturn;
  std::set<std::pair<const void *, std::string>> visited;
};

} // namespace

// verifyReturnOrigins requires a function returning a direct reference to
// carry a valid canonical origin summary and every returned reference's
// observed origins to be listed in it (MD-085).
LogicalResult sec::detail::verifyReturnOrigins(func::FuncOp function) {
  ArrayRef<Type> results = function.getFunctionType().getResults();
  if (function.isExternal() || !llvm::any_of(results, isReference))
    return success();
  auto origins = function->getAttrOfType<ArrayAttr>("sec.return_origins");
  if (!origins || origins.empty())
    return function.emitOpError(
        "a function returning a direct reference requires sec.return_origins");
  std::set<std::string> declared;
  for (Attribute attribute : origins) {
    auto text = dyn_cast<StringAttr>(attribute);
    std::optional<sec::ReturnOrigin> origin =
        text ? sec::parseReturnOrigin(text.getValue()) : std::nullopt;
    if (!origin)
      return function.emitOpError()
             << "sec.return_origins entry " << attribute
             << " is not a canonical origin";
    // MD-115 §3: a terminal range is a compatible slice, and a shared
    // source cannot justify a mutable result. Type paths remain structural.
    if (llvm::any_of(origin->projections, [](const auto &p) {
          return p.kind == sec::ReturnOriginProjection::Kind::Range || p.kind == sec::ReturnOriginProjection::Kind::UnknownRange;
        })) {
      auto parameter = originRootParameter(*origin);
      Type rootType;
      if (parameter && *parameter < function.getNumArguments()) rootType = function.getArgument(*parameter).getType();
      else if (auto global = staticWithID(function,origin->rootIndex)) rootType = global.getType();
      if (llvm::any_of(results, [&](Type t) { return isa<sec::SliceMutType>(t); }) && !isa_and_nonnull<sec::RefMutType,sec::SliceMutType>(rootType))
        return function.emitOpError("returned mutable slice requires mutable origin authority");
      bool sequence=false; Type element=originRootReferent(rootType,&sequence);
      if (!element && origin->root==sec::ReturnOrigin::Root::Static) element=rootType;
      Type finalType; bool finalSequence=false;
      if (!admissiblePath(element,sequence,*origin,&finalType,&finalSequence))
        return function.emitOpError("returned range has an inadmissible typed projection");
      for (Type result : results) {
        bool resultSequence=false; Type resultElement=originRootReferent(result,&resultSequence);
        if (resultElement != finalType || resultSequence != finalSequence)
          return function.emitOpError("returned range requires a compatible result element type");
      }
      auto last = origin->projections.back().kind;
      if ((last == sec::ReturnOriginProjection::Kind::Range || last == sec::ReturnOriginProjection::Kind::UnknownRange) && !llvm::all_of(results, [](Type t) { return isa<sec::SliceType,sec::SliceMutType>(t); }))
        return function.emitOpError("terminal range origin requires a compatible slice result");
    }
    // static@N names the address-taken static whose explicit sec.static_id
    // is N in this module (MD-116 §§ 6(2), 7(2)).
    if (origin->root == sec::ReturnOrigin::Root::Static) {
      sec::StaticGlobalOp global = staticWithID(function, origin->rootIndex);
      auto addressTaken =
          global ? global->getAttrOfType<BoolAttr>("sec.address_taken")
                 : BoolAttr();
      if (!addressTaken || !addressTaken.getValue())
        return function.emitOpError()
               << "sec.return_origins entry " << attribute
               << " must name an address-taken static with that sec.static_id";
      if (!admissiblePath(global.getType(), false, *origin))
        return function.emitOpError()
               << "sec.return_origins entry " << attribute
               << " has a projection that the static's type does not admit";
      declared.insert("static@" + std::to_string(origin->rootIndex) +
                      projectionsText(*origin));
      continue;
    }
    if (origin->root == sec::ReturnOrigin::Root::Receiver &&
        !hasReceiver(function))
      return function.emitOpError()
             << "sec.return_origins entry " << attribute
             << " names a receiver, but the function has none";
    unsigned parameter = *originRootParameter(*origin);
    if (parameter >= function.getNumArguments() ||
        !isReference(function.getArgumentTypes()[parameter]))
      return function.emitOpError()
             << "sec.return_origins entry " << attribute
             << " must name a reference parameter";
    if (!admissibleProjections(function.getArgumentTypes()[parameter], *origin))
      return function.emitOpError()
             << "sec.return_origins entry " << attribute
             << " has a projection that the parameter's referent type does "
                "not admit";
    declared.insert("param#" + std::to_string(parameter) +
                    projectionsText(*origin));
  }
  WalkResult result = function.walk([&](func::ReturnOp ret) {
    for (Value operand : ret.getOperands()) {
      if (!isReference(operand.getType()))
        continue;
      std::set<std::string> observed;
      ReturnOriginTracer tracer(function, ret,
          isa<sec::SliceType, sec::SliceMutType>(operand.getType()));
      if (failed(tracer.trace(operand, "", observed)))
        return WalkResult::interrupt();
      if (observed.empty()) {
        ret.emitOpError("returned reference has no proven external origin");
        return WalkResult::interrupt();
      }
      for (const std::string &origin : observed)
        if (!declared.count(origin)) {
          ret.emitOpError() << "returns a reference derived from " << origin
                            << ", which sec.return_origins does not list";
          return WalkResult::interrupt();
        }
    }
    return WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}

