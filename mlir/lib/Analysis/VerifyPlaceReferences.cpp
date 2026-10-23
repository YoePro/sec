// P15 Place/direct-reference verification with P16 bounded slice integration.
//
// Rules:
//   - rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 29, 34-37
//   - rules/mlir/packages/sec-mlir-dialect_package15.md sections 82-85, 131
//   - rules/mlir/packages/sec-mlir-dialect_package16.md sections 83-86, 116
#include "sec/Analysis/Passes.h"
#include "ReferenceFlow.h"

#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/Dominance.h"
#include "mlir/Interfaces/ControlFlowInterfaces.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "sec/Dialect/Sec/SecAttributes.h"
#include "sec/Dialect/Sec/ReturnOrigins.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/IR/SymbolTable.h"
#include <set>
#include <string>
#include "llvm/ADT/DenseSet.h"
#include "llvm/ADT/STLExtras.h"
#include "llvm/ADT/StringExtras.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/SmallVector.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYPLACES
#define GEN_PASS_DEF_SECVERIFYREFERENCEGUARDS
#define GEN_PASS_DEF_SECVERIFYBORROWSEMANTICS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;
using namespace sec::detail;

namespace {

bool isPlace(Type type) { return isa<sec::PlaceType>(type); }

// Recognize scalar and bounded direct safe-reference values.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.5,v12.25.
bool isReference(Type type) {
  return isa<sec::RefType, sec::RefMutType, sec::SliceType, sec::SliceMutType>(type);
}

// The operations that may consume a Place: projections, the bounds
// predicate, reads, writes and borrows (sec_mlir_dialect_v11.md section 29).
bool isPlaceConsumer(Operation *operation) {
  return isa<sec::PlaceFieldOp, sec::PlaceArrayIndexInBoundsOp,
             sec::PlaceArrayElementOp, sec::PlaceUnionPayloadOp,
             sec::PlaceReadOp, sec::PlaceWriteOp, sec::RefBorrowSharedOp,
             sec::RefBorrowMutOp, sec::RefReborrowSharedOp,
             sec::RefReborrowMutOp, sec::SliceBorrowSharedOp,
             sec::SliceBorrowMutOp>(operation);
}

// A canonical Place chain: the root (an SSA value, a storage value or a
// dereferenced reference) and the stored-field path below it.
struct Chain {
  Value root;
  SmallVector<int64_t> fields;
  bool valid = false;

  bool operator==(const Chain &other) const {
    return valid && other.valid && root == other.root && fields == other.fields;
  }
};

Chain placeChain(Value place) {
  SmallVector<int64_t> reversed;
  while (true) {
    Operation *definition = place.getDefiningOp();
    if (auto field = dyn_cast_or_null<sec::PlaceFieldOp>(definition)) {
      reversed.push_back(field.getFieldAttr().getInt());
      place = field.getBase();
      continue;
    }
    Chain chain;
    if (auto root = dyn_cast_or_null<sec::PlaceValueOp>(definition))
      chain.root = root.getValue();
    else if (auto root = dyn_cast_or_null<sec::PlaceStorageOp>(definition))
      chain.root = root.getStorage();
    else if (auto deref = dyn_cast_or_null<sec::PlaceDerefOp>(definition))
      chain.root = deref.getReference();
    else
      return Chain{};
    chain.fields.assign(reversed.rbegin(), reversed.rend());
    chain.valid = true;
    return chain;
  }
}

// The chain a tested value denotes: a value read from a Place, a stored
// field projected from such a value or from loaded storage, or the value
// itself as a value root.
Chain valueChain(Value value) {
  SmallVector<int64_t> reversed;
  while (true) {
    Operation *definition = value.getDefiningOp();
    if (auto extract = dyn_cast_or_null<sec::StructExtractOp>(definition)) {
      reversed.push_back(extract.getFieldAttr().getInt());
      value = extract.getSource();
      continue;
    }
    Chain chain;
    if (auto read = dyn_cast_or_null<sec::PlaceReadOp>(definition)) {
      chain = placeChain(read.getPlace());
      if (!chain.valid)
        return Chain{};
    } else if (auto load = dyn_cast_or_null<sec::StorageLoadOp>(definition)) {
      chain.root = load.getStorage();
      chain.valid = true;
    } else {
      chain.root = value;
      chain.valid = true;
    }
    for (auto field : llvm::reverse(reversed))
      chain.fields.push_back(field);
    return chain;
  }
}

// Report a reference use after end/move, including unambiguous CFG forwarding.
// Distinct copies/reborrows are separate holders, not forwarding aliases.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 20,25,37;
// rules/mlir/packages/sec-mlir-dialect_package15.md section 7.
Operation *reachableUse(Operation *from, Value value) {
  auto usesReference = [&](Operation *operation) {
    return llvm::any_of(operation->getOperands(), [&](Value operand) {
      return isReference(operand.getType()) && sameSource(operand, value);
    });
  };
  for (Operation *next = from->getNextNode(); next; next = next->getNextNode())
    if (usesReference(next))
      return next;
  SmallVector<Block *> worklist(from->getBlock()->getSuccessors().begin(),
                                from->getBlock()->getSuccessors().end());
  llvm::SmallPtrSet<Block *, 16> visited;
  while (!worklist.empty()) {
    Block *block = worklist.pop_back_val();
    if (!visited.insert(block).second)
      continue;
    for (Operation &operation : *block)
      if (usesReference(&operation))
        return &operation;
    for (Block *successor : block->getSuccessors())
      worklist.push_back(successor);
  }
  return nullptr;
}

// The reference a reborrow or move derives from: the source reference, or
// the reference whose dereference roots the source sub-Place.
Value derivationSource(Value source) {
  if (isReference(source.getType()))
    return source;
  // Follow field, element and payload projections down to the dereference
  // that roots the sub-Place.
  Value place = source;
  while (Operation *definition = place.getDefiningOp()) {
    if (auto deref = dyn_cast<sec::PlaceDerefOp>(definition))
      return deref.getReference();
    if (auto element = dyn_cast<sec::SliceElementPlaceOp>(definition))
      return element.getSource();
    if (!isa<sec::PlaceFieldOp, sec::PlaceArrayElementOp,
             sec::PlaceUnionPayloadOp>(definition))
      break;
    place = definition->getOperand(0);
  }
  return Value();
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
bool admissibleProjections(Type root, const sec::ReturnOrigin &origin) {
  bool sequence = false;
  Type current = originRootReferent(root, &sequence);
  if (!current)
    return false;
  for (const sec::ReturnOriginProjection &projection : origin.projections) {
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
  return true;
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

class ReturnOriginTracer {
public:
  ReturnOriginTracer(func::FuncOp function, func::ReturnOp ret, bool sliceReturn)
      : function(function), ret(ret), entry(&function.getBody().front()),
        sliceReturn(sliceReturn) {}

  // trace collects the origins of value with the projection path tail that
  // lies between value and the returned reference.
  LogicalResult trace(Value value, const std::string &tail,
                      std::set<std::string> &origins) {
    if (!visited.insert({value.getAsOpaquePointer(), tail}).second)
      return success();
    if (auto argument = dyn_cast<BlockArgument>(value)) {
      Block *owner = argument.getOwner();
      if (owner == entry) {
        origins.insert("param#" + std::to_string(argument.getArgNumber()) + tail);
        return success();
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
    if (isa<sec::RefBorrowSharedOp, sec::RefBorrowMutOp, sec::RefCopySharedOp,
            sec::RefMoveOp, sec::RefReborrowSharedOp, sec::RefReborrowMutOp,
            sec::PlaceDerefOp,
            sec::SliceCopySharedOp, sec::SliceMoveMutOp>(definition))
      return trace(definition->getOperand(0), tail, origins);
    // Only the complete source extent retains an unchanged slice origin.
    // An explicit sub-range needs a projection the current grammar cannot
    // encode; do not erase the range merely by following operand zero.
    // Rules: rules/mlir/packages/sec-mlir-dialect_package16.md sections 66-67;
    // rules/corrections/applied/md084-schema11-reference-metadata-correction-20261021.md
    // section 4(6); missing-decisions.yaml MD-115 records the open spelling.
    if (isa<sec::SliceBorrowSharedOp, sec::SliceBorrowMutOp,
            sec::SliceReborrowSharedOp, sec::SliceReborrowMutOp>(definition)) {
      Value start = sourceValue(definition->getOperand(1));
      auto normalize = start ? start.getDefiningOp<sec::SliceRangeNormalizeOp>()
                             : sec::SliceRangeNormalizeOp{};
      if (sliceReturn &&
          (!normalize || normalize.getProofKind() != "source-open-bound" ||
           normalize.getStartPresent() || normalize.getEndPresent()))
        return ret.emitOpError(
            "returned sub-slice origin requires an explicit slice-range projection (MD-115)");
      return trace(definition->getOperand(0), tail, origins);
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

// verifyReturnOrigins requires a function returning a direct reference to
// carry a valid canonical origin summary and every returned reference's
// observed origins to be listed in it (MD-085).
LogicalResult verifyReturnOrigins(func::FuncOp function) {
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
    if (origin->root == sec::ReturnOrigin::Root::Static)
      return function.emitOpError()
             << "sec.return_origins entry " << attribute
             << " names a static root, which has no schema-11 Place root "
                "(unsupported static-origin representation)";
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

} // namespace

// --sec-verify-places: Places never escape, runtime-checked element Places
// are dominated by their bounds predicate, and payload Places by an
// active-variant test of the same Place (sec_mlir_dialect_v11.md section 35).
LogicalResult sec::verifyPlaces(func::FuncOp function) {
  // P16 uses the same normalized-range and element guard contracts.
  // Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §85.
  if (failed(verifySliceGuards(function))) return failure();
  for (Type type : function.getFunctionType().getInputs())
    if (isPlace(type))
      return function.emitOpError("a !sec.place cannot be a function parameter");
  for (Type type : function.getFunctionType().getResults())
    if (isPlace(type))
      return function.emitOpError("a !sec.place cannot be returned");
  DominanceInfo dominance(function);
  WalkResult result = function.walk([&](Operation *operation) {
    if (!isPlaceConsumer(operation))
      for (Value operand : operation->getOperands())
        if (isPlace(operand.getType()))
          return operation->emitOpError(
                     "a !sec.place may only be consumed by Place and borrow operations"),
                 WalkResult::interrupt();
    // Storage a Place root addresses stays high-level until reference
    // representation is discharged, so its declaration must say so
    // (sec-mlir-dialect_package15.md section 67, spelling MD-084).
    if (auto root = dyn_cast<sec::PlaceStorageOp>(operation))
      if (auto declare = root.getStorage().getDefiningOp<sec::StorageDeclareOp>()) {
        auto addressTaken = declare->getAttrOfType<BoolAttr>("sec.address_taken");
        if (!addressTaken || !addressTaken.getValue())
          return operation->emitOpError(
                     "a Place root requires its storage declaration to carry sec.address_taken = true"),
                 WalkResult::interrupt();
      }
    if (auto element = dyn_cast<sec::PlaceArrayElementOp>(operation)) {
      if (element.getBoundsKind() != "runtime-check")
        return WalkResult::advance();
      bool guarded = false;
      for (Operation *user : element.getBase().getUsers()) {
        auto predicate = dyn_cast<sec::PlaceArrayIndexInBoundsOp>(user);
        if (predicate && predicate.getIndex() == element.getIndex() &&
            guardedBy(dominance, predicate.getResult(), true, operation))
          guarded = true;
      }
      if (!guarded)
        return operation->emitOpError(
                   "runtime-check element Place requires a dominating true edge of sec.place.array_index_in_bounds on the same base and index"),
               WalkResult::interrupt();
    }
    if (auto payload = dyn_cast<sec::PlaceUnionPayloadOp>(operation)) {
      Chain base = placeChain(payload.getBase());
      int64_t variant = payload.getVariantAttr().getInt();
      bool guarded = false;
      function.walk([&](Operation *test) {
        if (auto isVariant = dyn_cast<sec::UnionIsVariantOp>(test)) {
          if (isVariant.getVariantAttr().getInt() == variant &&
              valueChain(isVariant.getValue()) == base &&
              guardedBy(dominance, isVariant.getResult(), true, operation))
            guarded = true;
        } else if (auto isErr = dyn_cast<sec::ResultIsErrOp>(test)) {
          // Err is proven on the true edge, Ok on the false edge.
          if ((variant == 0 || variant == 1) &&
              valueChain(isErr.getValue()) == base &&
              guardedBy(dominance, isErr.getResult(), variant == 1, operation))
            guarded = true;
        }
      });
      if (!guarded)
        return operation->emitOpError(
                   "payload Place requires a dominating active-variant test of the same Place"),
               WalkResult::interrupt();
    }
    return WalkResult::advance();
  });
  if (result.wasInterrupted())
    return failure();
  // Places do not flow through block arguments either.
  for (Block &block : function.getBody())
    for (BlockArgument argument : block.getArguments())
      if (isPlace(argument.getType()) && !block.isEntryBlock())
        return function.emitOpError("a !sec.place cannot be a block argument");
  return success();
}

// --sec-verify-reference-guards: every dereference or comparison of a
// dynamic-epoch reference is dominated by the true edge of sec.ref.is_valid
// on the same SSA reference, whose false edge terminates in
// sec.fail.reference_generation (sec_mlir_dialect_v11.md section 36).
LogicalResult sec::verifyReferenceGuards(func::FuncOp function) {
  // A transfer cannot forge a proven result and thereby evade this pass.
  // Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 19-20,27.
  if (failed(verifyReferenceTransfers(function))) return failure();
  // The shared slice verifier owns exact-source temporal/spatial ordering.
  // Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §84.
  if (failed(verifySliceGuards(function))) return failure();
  DominanceInfo dominance(function);
  auto validated = [&](Value reference, Operation *use) {
    for (Operation *user : reference.getUsers()) {
      auto isValid = dyn_cast<sec::RefIsValidOp>(user);
      if (!isValid)
        continue;
      for (Operation *branchUser : isValid.getResult().getUsers()) {
        auto branch = dyn_cast<cf::CondBranchOp>(branchUser);
        if (!branch || branch.getCondition() != isValid.getResult())
          continue;
        Block *failure = branch.getFalseDest();
        if (failure->empty() ||
            !isa<sec::FailReferenceGenerationOp>(failure->back()))
          continue;
        if (guardedBy(dominance, isValid.getResult(), true, use))
          return true;
      }
    }
    return false;
  };
  WalkResult result = function.walk([&](Operation *operation) {
    SmallVector<Value, 2> references;
    // Reads and writes go through place.deref; a reborrow may also consume
    // the reference directly.
    if (auto deref = dyn_cast<sec::PlaceDerefOp>(operation))
      references.push_back(deref.getReference());
    else if (auto compare = dyn_cast<sec::RefCompareOp>(operation))
      references.append({compare.getLhs(), compare.getRhs()});
    else if (auto reborrow = dyn_cast<sec::RefReborrowSharedOp>(operation)) {
      if (isReference(reborrow.getSource().getType()))
        references.push_back(reborrow.getSource());
    } else if (auto reborrow = dyn_cast<sec::RefReborrowMutOp>(operation)) {
      if (isReference(reborrow.getSource().getType()))
        references.push_back(reborrow.getSource());
    }
    for (Value reference : references) {
      auto incoming = incomingReferenceFacts(reference);
      if (!llvm::any_of(incoming, [](const ReferenceFacts &facts) {
            return facts.dynamic;
          }))
        continue;
      if (!validated(reference, operation))
        return operation->emitOpError(
                   "use of a dynamic-epoch reference requires a dominating sec.ref.is_valid true edge whose false edge ends in sec.fail.reference_generation"),
               WalkResult::interrupt();
    }
    return WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}

// --sec-verify-borrow-semantics: derived references name the borrow they
// derive from, end markers name the ended reference's own identities, and
// no use is reachable after a reference's end or move, and returned
// references derive only from the origins their function's summary names
// (sec_mlir_dialect_v11.md section 37).
LogicalResult sec::verifyBorrowSemantics(func::FuncOp function) {
  if (failed(verifyReferenceTransfers(function))) return failure();
  if (failed(verifySliceReferences(function))) return failure();
  auto checkDerivation = [](Operation *operation, Value source,
                            std::optional<uint32_t> sourceBorrow) -> LogicalResult {
    Value reference = derivationSource(source);
    if (!reference)
      return operation->emitOpError(
          "a reborrow derives from a reference or a sub-Place of its dereference");
    for (const ReferenceFacts &facts : incomingReferenceFacts(reference))
      if (!sourceBorrow || *sourceBorrow != facts.borrow)
        return operation->emitOpError(
            "source_borrow must name the borrow_id of the reference it derives from");
    return success();
  };
  WalkResult result = function.walk([&](Operation *operation) {
    if (auto reborrow = dyn_cast<sec::RefReborrowSharedOp>(operation))
      if (failed(checkDerivation(operation, reborrow.getSource(), reborrow.getSourceBorrow())))
        return WalkResult::interrupt();
    if (auto reborrow = dyn_cast<sec::RefReborrowMutOp>(operation))
      if (failed(checkDerivation(operation, reborrow.getSource(), reborrow.getSourceBorrow())))
        return WalkResult::interrupt();
    if (auto end = dyn_cast<sec::RefEndBorrowOp>(operation)) {
      // The selected holder's identities must match every represented
      // incoming direct-reference source, not just a local SSA definition.
      // Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 25,37.
      if (isa<sec::RefType, sec::RefMutType>(end.getReference().getType()))
        for (const ReferenceFacts &incoming :
             incomingReferenceFacts(end.getReference()))
          if (incoming.borrow != end.getBorrowId() ||
              incoming.lifetime != end.getLifetimeId())
            return operation->emitOpError(
                       "end_borrow must name the ended reference's borrow_id and lifetime_id"),
                   WalkResult::interrupt();
      ReferenceFacts facts = referenceFacts(end.getReference());
      if (facts.known && (facts.borrow != end.getBorrowId() ||
                          facts.lifetime != end.getLifetimeId()))
        return operation->emitOpError(
                   "end_borrow must name the ended reference's borrow_id and lifetime_id"),
               WalkResult::interrupt();
      // Slice holder/derived/CFG lifetimes are checked by verifySliceReferences.
      // Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§63,86.
      if (!isa<sec::SliceType, sec::SliceMutType>(end.getReference().getType()))
        if (Operation *later = reachableUse(operation, end.getReference()))
          return later->emitOpError("reference used after its sec.ref.end_borrow"),
                 WalkResult::interrupt();
    }
    if (auto move = dyn_cast<sec::RefMoveOp>(operation)) {
      if (Operation *later = reachableUse(operation, move.getSource()))
        return later->emitOpError("!sec.ref_mut used after it was moved"),
               WalkResult::interrupt();
    }
    return WalkResult::advance();
  });
  if (result.wasInterrupted())
    return failure();
  return verifyReturnOrigins(function);
}

namespace {

class VerifyPlacesPass final
    : public sec::impl::SecVerifyPlacesBase<VerifyPlacesPass> {
public:
  void runOnOperation() override {
    if (failed(sec::verifyPlaces(getOperation())))
      signalPassFailure();
  }
};

class VerifyReferenceGuardsPass final
    : public sec::impl::SecVerifyReferenceGuardsBase<VerifyReferenceGuardsPass> {
public:
  void runOnOperation() override {
    if (failed(sec::verifyReferenceGuards(getOperation())))
      signalPassFailure();
  }
};

class VerifyBorrowSemanticsPass final
    : public sec::impl::SecVerifyBorrowSemanticsBase<VerifyBorrowSemanticsPass> {
public:
  void runOnOperation() override {
    if (failed(sec::verifyBorrowSemantics(getOperation())))
      signalPassFailure();
  }
};

} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyPlacesPass() {
  return std::make_unique<VerifyPlacesPass>();
}

std::unique_ptr<mlir::Pass> sec::createSecVerifyReferenceGuardsPass() {
  return std::make_unique<VerifyReferenceGuardsPass>();
}

std::unique_ptr<mlir::Pass> sec::createSecVerifyBorrowSemanticsPass() {
  return std::make_unique<VerifyBorrowSemanticsPass>();
}
