#include "ReferenceFlow.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/DenseMap.h"

using namespace mlir;
using namespace sec;
using namespace sec::detail;

namespace {
// Slice holders are direct safe references, independently of their elements.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31,84–86.
bool isSlice(Type type) { return isa<SliceType, SliceMutType>(type); }

// Canonical P15 metadata is reused, with parameter dictionaries and operation
// properties exposed under the same keys. Absence keeps the established
// ordinary-address/unpinned/empty-storage defaults (MD-084 § 3(4)).
// Rules: rules/mlir/sec_mlir_dialect.md — v11.7,v12.5,v12.25.
DictionaryAttr metadata(Value value) {
  value = sourceValue(value);
  if (!value) return {};
  if (auto arg = dyn_cast<BlockArgument>(value)) {
    auto fn = dyn_cast<func::FuncOp>(arg.getOwner()->getParentOp());
    return fn ? fn.getArgAttrOfType<DictionaryAttr>(arg.getArgNumber(), "sec.reference") : DictionaryAttr{};
  }
  Operation *op = value.getDefiningOp();
  if (!op) return {};
  auto facts = op->getAttrOfType<DictionaryAttr>("sec.reference");
  auto properties = dyn_cast_or_null<DictionaryAttr>(op->getPropertiesAsAttribute());
  if (!facts && (!properties || !properties.get("borrow_id"))) return {};
  NamedAttrList attrs;
  if (properties) attrs.append(properties.getValue());
  // MD-084 section 2(5): dictionary and properties are compatible views of
  // the same facts, not mutually exclusive sources. Supplemental facts still
  // apply when a dictionary omits their redundant/default entries.
  if (facts)
    for (NamedAttribute entry : facts) attrs.set(entry.getName(), entry.getValue());
  for (auto [key, extra] : {std::pair{"origin_storages", "sec.origin_storages"},
                            {"pinned", "sec.pinned"},
                            {"address_space", "sec.reference_address_space"}})
    if (Attribute attr = op->getAttr(extra)) attrs.set(key, attr);
  return attrs.getDictionary(op->getContext());
}

// Missing optional facts equal their canonical omitted defaults only.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.7,v12.5; MD-084.
Attribute fact(DictionaryAttr dict, StringRef key) {
  if (!dict) return {};
  Attribute attr = dict.get(key);
  if (key == "pinned") {
    if (auto value = dyn_cast_or_null<BoolAttr>(attr); value && !value.getValue()) return {};
  }
  if (key == "address_space") {
    if (auto value = dyn_cast_or_null<StringAttr>(attr); value && value.getValue() == "ordinary") return {};
  }
  if (key == "origin_storages") {
    if (auto value = dyn_cast_or_null<ArrayAttr>(attr); value && value.empty()) return {};
  }
  return attr;
}

// Derived holders may get new borrow/lifetime identities, but cannot change
// their parent's validity, storage, epoch, stability, pin or address contract.
// Copies/moves preserve both identities and do not observe backing storage.
// Rules: rules/compiler/semantic_ir.md — §126.8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§40–46,63,86.
LogicalResult preserveFacts(Operation *op, Value source, bool derived) {
  DictionaryAttr before = metadata(source), after = metadata(op->getResult(0));
  if (!before || !after)
    return op->emitOpError("slice derivation/transfer requires canonical source reference facts");
  for (StringRef key : {"validity_policy", "validity_proof", "invalidation_domain", "epoch_dependency", "address_stability", "provenance", "origin_storages", "pinned", "address_space"})
    if (fact(before, key) != fact(after, key))
      return op->emitOpError() << "slice operation changes source reference fact " << key;
  if (!derived) {
    for (StringRef key : {"borrow_id", "lifetime_id"})
      if (before.get(key) != after.get(key))
        return op->emitOpError() << "slice transfer/element Place changes source " << key;
  } else if (before.get("borrow_id") == after.get("borrow_id")) {
    return op->emitOpError("slice derivation requires a distinct borrow_id");
  }
  // P15 element borrows may derive implicitly through their Place operand;
  // explicit reborrows and slice transfers carry source_borrow.
  // Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58,116.
  bool implicitPlaceBorrow = isa<RefBorrowSharedOp, RefBorrowMutOp>(op) && !after.get("source_borrow");
  // MD-084 section 3(3): copy/move may omit source_borrow when it equals
  // borrow_id. The exact borrow/lifetime identity checks above still apply.
  bool implicitTransfer = isa<SliceCopySharedOp, SliceMoveMutOp>(op) && !after.get("source_borrow");
  if (!isa<SliceElementPlaceOp>(op) && !implicitPlaceBorrow && !implicitTransfer &&
      after.get("source_borrow") != before.get("borrow_id"))
    return op->emitOpError("source_borrow must name the granting source borrow_id");
  if (isa<SliceElementPlaceOp>(op) && fact(before,"source_borrow") != fact(after,"source_borrow"))
    return op->emitOpError("slice element Place changes source_borrow");
  if (isa<SliceCopySharedOp, SliceMoveMutOp>(op)) {
    Operation *definition = sourceValue(source).getDefiningOp();
    for (StringRef key : {"sec.slice_static_start", "sec.slice_static_end_exclusive", "sec.slice_range_id"})
        if (op->getAttr(key) && (!definition || op->getAttr(key) != definition->getAttr(key)))
          return op->emitOpError("slice transfer changes represented range facts");
  }
  return success();
}

// Follow only projection/derivation edges to the granting reference, or the
// owned Place root. Element Places keep the same bounded-source contract.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§44,50,85–86,115–116.
Value grantingReference(Value place) {
  while (Operation *op = place.getDefiningOp()) {
    if (auto deref = dyn_cast<PlaceDerefOp>(op)) return deref.getReference();
    if (auto element = dyn_cast<SliceElementPlaceOp>(op)) return element.getSource();
    if (!isa<PlaceFieldOp, PlaceArrayElementOp, PlaceUnionPayloadOp>(op)) break;
    place = op->getOperand(0);
  }
  return {};
}

// Fixed-array Places preserve any represented root storage/address facts.
// Projection chains cannot manufacture a source borrow or dynamic validity.
// Rules: rules/compiler/semantic_ir.md — §126.7;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§44,85,94–95,115.
LogicalResult verifyOwnedPlace(Operation *borrow) {
  Value place = borrow->getOperand(0);
  while (Operation *op = place.getDefiningOp()) {
    if (isa<PlaceFieldOp, PlaceArrayElementOp, PlaceUnionPayloadOp>(op)) {
      place = op->getOperand(0);
      continue;
    }
    if (!isa<PlaceValueOp, PlaceStorageOp>(op)) break;
    auto props = cast<DictionaryAttr>(op->getPropertiesAsAttribute());
    auto facts = metadata(borrow->getResult(0));
    if (Attribute origins = fact(facts, "origin_storages")) {
      auto list = dyn_cast<ArrayAttr>(origins);
      if (!list || list.size() != 1 || list[0] != props.get("storage_identity"))
        return borrow->emitOpError("fixed-array slice borrow changes its Place storage identity");
    }
    auto space = dyn_cast_or_null<StringAttr>(props.get("address_space"));
    auto recorded = dyn_cast_or_null<StringAttr>(fact(facts, "address_space"));
    if ((recorded ? recorded.getValue() : "ordinary") != (space ? space.getValue() : "ordinary"))
      return borrow->emitOpError("fixed-array slice borrow changes its Place address space");
    return success();
  }
  return borrow->emitOpError("slice borrow requires a canonical owned or dereferenced Place root");
}

// Reachability after an end/move follows CFG aliases and derived references.
// A shared copy starts an independent holder; descendants of a reborrow keep
// the parent dependency even when subsequently copied/moved. Loop redefinition
// starts a new instance, rather than making the previous instance live again.
// Rules: rules/compiler/semantic_ir.md — §66;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–43,60–63,86.
Operation *useAfterBoundary(Operation *boundary, Value ended) {
  struct Edge { Value value; bool derived; bool transfer; };
  llvm::DenseMap<Value, SmallVector<Edge>> graph;
  func::FuncOp fn = boundary->getParentOfType<func::FuncOp>();
  fn.walk([&](Operation *op) {
    if (isa<PlaceDerefOp, PlaceFieldOp, PlaceArrayElementOp, PlaceUnionPayloadOp,
            SliceElementPlaceOp, RefBorrowSharedOp, RefBorrowMutOp,
            RefReborrowSharedOp, RefReborrowMutOp, SliceBorrowSharedOp,
            SliceBorrowMutOp, SliceReborrowSharedOp, SliceReborrowMutOp,
            SliceCopySharedOp, SliceMoveMutOp, RefCopySharedOp, RefMoveOp>(op)) {
      bool derived = isa<RefBorrowSharedOp, RefBorrowMutOp, RefReborrowSharedOp,
                         RefReborrowMutOp, SliceBorrowSharedOp, SliceBorrowMutOp,
                         SliceReborrowSharedOp, SliceReborrowMutOp>(op);
      bool transfer = isa<SliceCopySharedOp, SliceMoveMutOp, RefCopySharedOp, RefMoveOp>(op);
      graph[op->getOperand(0)].push_back({op->getResult(0), derived, transfer});
    }
    if (auto branch = dyn_cast<BranchOpInterface>(op))
      for (unsigned i = 0; i < branch->getNumSuccessors(); ++i) {
        auto succ = branch.getSuccessorOperands(i);
        for (auto [index, operand] : llvm::enumerate(succ.getForwardedOperands())) {
          Value arg = branch->getSuccessor(i)->getArgument(index + succ.getProducedOperandCount());
          if (!isSlice(arg.getType()) && !isa<RefType, RefMutType>(arg.getType())) continue;
          graph[operand].push_back({arg,false,false});
          graph[arg].push_back({operand,false,false});
        }
      }
  });
  SmallVector<std::pair<Value,unsigned>> pending{{ended,false}};
  llvm::DenseSet<std::pair<Value,unsigned>> visited;
  llvm::DenseSet<Value> aliases;
  while (!pending.empty()) {
    auto state = pending.pop_back_val();
    if (!visited.insert(state).second) continue;
    aliases.insert(state.first);
    for (Edge edge : graph[state.first]) {
      if (!state.second && edge.transfer) continue;
      pending.push_back({edge.value,state.second || edge.derived});
    }
  }
  SmallVector<std::pair<Block *, Operation *>> work{{boundary->getBlock(),boundary->getNextNode()}};
  llvm::SmallPtrSet<Block *,16> blocks;
  while (!work.empty()) {
    auto [block, first] = work.pop_back_val();
    bool initial = block == boundary->getBlock() && first == boundary->getNextNode();
    if (!initial && !blocks.insert(block).second) continue;
    if (!initial)
      if (auto arg = dyn_cast<BlockArgument>(ended); arg && arg.getOwner() == block) continue;
    bool fresh = false;
    for (Operation *op = first; op; op = op->getNextNode()) {
      if (llvm::is_contained(op->getResults(),ended)) { fresh=true; break; }
      if (llvm::any_of(op->getOperands(),[&](Value value){return aliases.contains(value);})) return op;
    }
    if (!fresh)
      for (Block *next : block->getSuccessors()) work.push_back({next,&next->front()});
  }
  return nullptr;
}
} // namespace

// P15's borrow pass consumes the same P16 spatial proof and canonical reference
// facts; it does not rerun overlap analysis, invent metadata or choose layout.
// Static disjointness remains Sema's proof, never a runtime borrow check.
// Rules: rules/mlir/sec_mlir_dialect.md — v12.25;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§83–86,111,116.
LogicalResult sec::detail::verifySliceReferences(func::FuncOp fn) {
  if (failed(sec::verifySliceGuards(fn))) return failure();
  WalkResult result = fn.walk([&](Operation *op) {
    Value source;
    bool derived = false;
    if (isa<SliceCopySharedOp, SliceMoveMutOp, SliceElementPlaceOp>(op)) source=op->getOperand(0);
    if (isa<SliceReborrowSharedOp, SliceReborrowMutOp>(op)) { source=op->getOperand(0); derived=true; }
    if (isa<SliceBorrowSharedOp, SliceBorrowMutOp>(op)) {
      source=grantingReference(op->getOperand(0)); derived=bool(source);
      if (!source) {
        auto props=cast<DictionaryAttr>(op->getPropertiesAsAttribute());
        if (props.get("source_borrow") || cast<StringAttr>(props.get("validity_policy")).getValue()!="proven") {
          op->emitOpError("owned fixed-array Place requires a proven borrow without source_borrow");
          return WalkResult::interrupt();
        }
        if (failed(verifyOwnedPlace(op))) return WalkResult::interrupt();
      }
    }
    // Element reference derivations retain the same bounded source facts.
    if (isa<RefBorrowSharedOp, RefBorrowMutOp, RefReborrowSharedOp, RefReborrowMutOp>(op)) {
      Value parent=grantingReference(op->getOperand(0));
      if (parent && isSlice(parent.getType())) {source=parent;derived=true;}
    }
    if (source && failed(preserveFacts(op,source,derived))) return WalkResult::interrupt();
    Value ended;
    if (auto end=dyn_cast<RefEndBorrowOp>(op)) {
      ended=end.getReference();
      if (isSlice(ended.getType())) {
        ReferenceFacts facts=referenceFacts(sourceValue(ended));
        if (!facts.known || facts.borrow!=end.getBorrowId() || facts.lifetime!=end.getLifetimeId()) {
          op->emitOpError("slice end_borrow must match source borrow_id and lifetime_id");
          return WalkResult::interrupt();
        }
      }
    } else if (auto move=dyn_cast<SliceMoveMutOp>(op)) ended=move.getSource();
    else if (auto move=dyn_cast<RefMoveOp>(op)) ended=move.getSource();
    if (ended)
      if (Operation *later=useAfterBoundary(op,ended)) {
        later->emitOpError(isa<SliceMoveMutOp>(op) ? "slice used after mutable move" :
                           isa<RefMoveOp>(op) ? "!sec.ref_mut used after it was moved" :
                           "reference used after its sec.ref.end_borrow");
        return WalkResult::interrupt();
      }
    return WalkResult::advance();
  });
  return result.wasInterrupted()?failure():success();
}
