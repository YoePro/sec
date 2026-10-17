// Schema-11 (SEC-MLIR-P15) Place and direct-reference verifiers.
//
// Rules:
//   - rules/mlir/dialect-versions/sec_mlir_dialect_v11.md sections 29, 34-37
//   - rules/mlir/packages/sec-mlir-dialect_package15.md sections 82-85, 131
#include "sec/Analysis/Passes.h"

#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/Dominance.h"
#include "mlir/Interfaces/ControlFlowInterfaces.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
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

namespace {

bool isPlace(Type type) { return isa<sec::PlaceType>(type); }

bool isReference(Type type) {
  return isa<sec::RefType, sec::RefMutType>(type);
}

// The operations that may consume a Place: projections, the bounds
// predicate, reads, writes and borrows (sec_mlir_dialect_v11.md section 29).
bool isPlaceConsumer(Operation *operation) {
  return isa<sec::PlaceFieldOp, sec::PlaceArrayIndexInBoundsOp,
             sec::PlaceArrayElementOp, sec::PlaceUnionPayloadOp,
             sec::PlaceReadOp, sec::PlaceWriteOp, sec::RefBorrowSharedOp,
             sec::RefBorrowMutOp, sec::RefReborrowSharedOp,
             sec::RefReborrowMutOp>(operation);
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

// guardedBy reports whether `operation` is dominated by the selected edge
// of a conditional branch on `predicate`, made exclusive by a dedicated
// successor block (no other path enters it).
bool guardedBy(DominanceInfo &dominance, Value predicate, bool trueEdge,
               Operation *operation) {
  for (Operation *user : predicate.getUsers()) {
    auto branch = dyn_cast<cf::CondBranchOp>(user);
    if (!branch || branch.getCondition() != predicate ||
        branch.getTrueDest() == branch.getFalseDest())
      continue;
    Block *edge = trueEdge ? branch.getTrueDest() : branch.getFalseDest();
    if (edge->getSinglePredecessor() != branch->getBlock())
      continue;
    if (dominance.dominates(edge, operation->getBlock()))
      return true;
  }
  return false;
}

// reachableUses reports a use of `value` reachable after `from` in its own
// block or in any successor block (cycles included).
Operation *reachableUse(Operation *from, Value value) {
  for (Operation *next = from->getNextNode(); next; next = next->getNextNode())
    if (llvm::is_contained(next->getOperands(), value))
      return next;
  SmallVector<Block *> worklist(from->getBlock()->getSuccessors().begin(),
                                from->getBlock()->getSuccessors().end());
  llvm::SmallPtrSet<Block *, 16> visited;
  while (!worklist.empty()) {
    Block *block = worklist.pop_back_val();
    if (!visited.insert(block).second)
      continue;
    for (Operation &operation : *block)
      if (llvm::is_contained(operation.getOperands(), value))
        return &operation;
    for (Block *successor : block->getSuccessors())
      worklist.push_back(successor);
  }
  return nullptr;
}

// Reference metadata of the operation defining a reference value.
struct ReferenceFacts {
  bool known = false;
  uint32_t borrow = 0;
  uint32_t lifetime = 0;
  bool dynamic = false;
};

template <typename OpT>
ReferenceFacts factsOf(OpT op) {
  return ReferenceFacts{true, op.getBorrowId(), op.getLifetimeId(),
                        op.getValidityPolicy() == "dynamic-epoch"};
}

// Facts recorded as a `sec.reference` dictionary on a reference function
// parameter or call result (implementation choice MD-084).
ReferenceFacts dictionaryFacts(DictionaryAttr facts) {
  if (!facts)
    return ReferenceFacts{};
  auto borrow = facts.getAs<IntegerAttr>("borrow_id");
  auto lifetime = facts.getAs<IntegerAttr>("lifetime_id");
  auto policy = facts.getAs<StringAttr>("validity_policy");
  if (!borrow || !lifetime || !policy)
    return ReferenceFacts{};
  return ReferenceFacts{true, static_cast<uint32_t>(borrow.getInt()),
                        static_cast<uint32_t>(lifetime.getInt()),
                        policy.getValue() == "dynamic-epoch"};
}

ReferenceFacts referenceFacts(Value reference) {
  if (auto argument = dyn_cast<BlockArgument>(reference)) {
    Block *owner = argument.getOwner();
    auto function = dyn_cast_or_null<func::FuncOp>(owner->getParentOp());
    if (function && owner->isEntryBlock())
      return dictionaryFacts(function.getArgAttrOfType<DictionaryAttr>(
          argument.getArgNumber(), "sec.reference"));
    return ReferenceFacts{};
  }
  Operation *definition = reference.getDefiningOp();
  if (auto facts = definition->getAttrOfType<DictionaryAttr>("sec.reference"))
    return dictionaryFacts(facts);
  if (auto op = dyn_cast_or_null<sec::RefBorrowSharedOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefBorrowMutOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefCopySharedOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefMoveOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefReborrowSharedOp>(definition))
    return factsOf(op);
  if (auto op = dyn_cast_or_null<sec::RefReborrowMutOp>(definition))
    return factsOf(op);
  return ReferenceFacts{};
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
    if (!isa<sec::PlaceFieldOp, sec::PlaceArrayElementOp,
             sec::PlaceUnionPayloadOp>(definition))
      break;
    place = definition->getOperand(0);
  }
  return Value();
}

// The function parameter a `sec.return_origins` entry is rooted at: param#N,
// or the receiver, which is parameter 0. Static origins have no schema-11
// root and yield std::nullopt with *isStatic set.
std::optional<unsigned> originParameter(StringRef origin, bool *isStatic) {
  *isStatic = origin.starts_with("static@");
  if (origin.starts_with("receiver"))
    return 0u;
  if (!origin.consume_front("param#"))
    return std::nullopt;
  StringRef digits = origin.take_while([](char c) { return llvm::isDigit(c); });
  unsigned index = 0;
  if (digits.empty() || digits.getAsInteger(10, index))
    return std::nullopt;
  return index;
}

// verifyReturnOrigins requires a function returning a direct reference to
// carry its origin summary, and every returned reference to derive only from
// the reference parameters that summary names: through copies, moves,
// reborrows, dereferences, Place projections, block arguments and calls whose
// reference arguments derive from them. A Place rooted at local storage or a
// local value is never an allowed origin (package15.md sections 61-62, 85).
// The projection paths of the summary are Sema facts and are not
// re-derived here.
LogicalResult verifyReturnOrigins(func::FuncOp function) {
  ArrayRef<Type> results = function.getFunctionType().getResults();
  if (function.isExternal() || !llvm::any_of(results, isReference))
    return success();
  auto origins = function->getAttrOfType<ArrayAttr>("sec.return_origins");
  if (!origins || origins.empty())
    return function.emitOpError(
        "a function returning a direct reference requires sec.return_origins");
  llvm::SmallDenseSet<unsigned> allowed;
  for (Attribute attribute : origins) {
    auto origin = dyn_cast<StringAttr>(attribute);
    bool isStatic = false;
    std::optional<unsigned> parameter =
        origin ? originParameter(origin.getValue(), &isStatic) : std::nullopt;
    if (isStatic)
      continue;
    if (!parameter || *parameter >= function.getNumArguments() ||
        !isReference(function.getArgumentTypes()[*parameter]))
      return function.emitOpError()
             << "sec.return_origins entry " << attribute
             << " must name a reference parameter";
    allowed.insert(*parameter);
  }
  Block &entry = function.getBody().front();
  WalkResult result = function.walk([&](func::ReturnOp ret) {
    SmallVector<Value> worklist;
    llvm::SmallPtrSet<Operation *, 16> visitedOps;
    llvm::DenseSet<Value> visited;
    for (Value operand : ret.getOperands())
      if (isReference(operand.getType()))
        worklist.push_back(operand);
    while (!worklist.empty()) {
      Value value = worklist.pop_back_val();
      if (!visited.insert(value).second)
        continue;
      if (auto argument = dyn_cast<BlockArgument>(value)) {
        Block *owner = argument.getOwner();
        if (owner == &entry) {
          if (!allowed.contains(argument.getArgNumber()))
            return ret.emitOpError()
                       << "returns a reference derived from parameter "
                       << argument.getArgNumber()
                       << ", which sec.return_origins does not name",
                   WalkResult::interrupt();
          continue;
        }
        for (Block *predecessor : owner->getPredecessors()) {
          auto branch =
              dyn_cast<BranchOpInterface>(predecessor->getTerminator());
          if (!branch)
            return ret.emitOpError("returned reference has an unknown origin"),
                   WalkResult::interrupt();
          for (unsigned index = 0; index < branch->getNumSuccessors(); ++index)
            if (branch->getSuccessor(index) == owner)
              if (Value incoming = branch.getSuccessorOperands(index)
                                       [argument.getArgNumber()])
                worklist.push_back(incoming);
        }
        continue;
      }
      Operation *definition = value.getDefiningOp();
      if (isa<sec::PlaceStorageOp, sec::PlaceValueOp>(definition))
        return ret.emitOpError(
                   "returns a reference to local storage or a local value"),
               WalkResult::interrupt();
      if (isa<sec::RefBorrowSharedOp, sec::RefBorrowMutOp,
              sec::RefCopySharedOp, sec::RefMoveOp, sec::RefReborrowSharedOp,
              sec::RefReborrowMutOp, sec::PlaceDerefOp, sec::PlaceFieldOp,
              sec::PlaceArrayElementOp, sec::PlaceUnionPayloadOp>(definition)) {
        worklist.push_back(definition->getOperand(0));
        continue;
      }
      if (isa<sec::CallDirectOp>(definition)) {
        if (!visitedOps.insert(definition).second)
          continue;
        bool any = false;
        for (Value operand : definition->getOperands())
          if (isReference(operand.getType())) {
            worklist.push_back(operand);
            any = true;
          }
        if (any)
          continue;
      }
      return ret.emitOpError("returned reference has an unknown origin"),
             WalkResult::interrupt();
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
      if (!referenceFacts(reference).dynamic)
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
  auto checkDerivation = [](Operation *operation, Value source,
                            std::optional<uint32_t> sourceBorrow) -> LogicalResult {
    Value reference = derivationSource(source);
    if (!reference)
      return operation->emitOpError(
          "a reborrow derives from a reference or a sub-Place of its dereference");
    ReferenceFacts facts = referenceFacts(reference);
    if (facts.known && (!sourceBorrow || *sourceBorrow != facts.borrow))
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
      ReferenceFacts facts = referenceFacts(end.getReference());
      if (facts.known && (facts.borrow != end.getBorrowId() ||
                          facts.lifetime != end.getLifetimeId()))
        return operation->emitOpError(
                   "end_borrow must name the ended reference's borrow_id and lifetime_id"),
               WalkResult::interrupt();
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
