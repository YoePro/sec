// Consume explicit P15 borrow ends before P17 destroys their referent storage.
// Rules: rules/memory/destruction.md §18(3); sec-mlir-dialect_package17.md §117.
#include "ReferenceFlow.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "mlir/IR/SymbolTable.h"
#include "llvm/ADT/SmallPtrSet.h"
#include "llvm/ADT/STLExtras.h"
#include <algorithm>
#include <set>
#include <tuple>
#include <vector>

using namespace mlir;
using namespace sec;
using namespace sec::detail;

namespace {
// Field, active-payload and conservatively indexed element projection steps.
using Step = std::pair<unsigned, int64_t>;
using Path = SmallVector<Step>;
using Visit = std::tuple<void *, std::vector<Step>, bool>;
struct Origin {
  Value root;
  Operation *global = nullptr;
  Path path;
  bool payload = false;
};

// Only represented Place/reference derivations are followed. CFG alternatives
// remain separate; cycles terminate without inventing a new source or lifetime.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md §§7,82–85;
// rules/mlir/packages/sec-mlir-dialect_package17.md §117.
void collectOrigins(Value value, Path path, bool payload,
                    llvm::DenseSet<Value> &visiting, std::set<Visit> &seen, SmallVectorImpl<Origin> &out) {
  if (!value || visiting.contains(value)) return;
  // Repeated CFG diamonds do not repeat the same query exponentially. The
  // projection path remains part of the key so joined distinct fields survive.
  Visit query{value.getAsOpaquePointer(), {path.begin(), path.end()}, payload};
  if (!seen.insert(std::move(query)).second) return;
  visiting.insert(value);
  auto follow = [&](Value source) { collectOrigins(source, path, payload, visiting, seen, out); };
  Operation *op = value.getDefiningOp();
  if (auto arg = dyn_cast<BlockArgument>(value); arg && !arg.getOwner()->isEntryBlock()) {
    for (Block *pred : arg.getOwner()->getPredecessors())
      if (auto branch = dyn_cast<BranchOpInterface>(pred->getTerminator()))
        for (unsigned i = 0; i < branch->getNumSuccessors(); ++i) {
          if (branch->getSuccessor(i) != arg.getOwner()) continue;
          auto operands = branch.getSuccessorOperands(i);
          unsigned n = arg.getArgNumber();
          if (n >= operands.getProducedOperandCount()) {
            n -= operands.getProducedOperandCount();
            if (n < operands.getForwardedOperands().size()) follow(operands.getForwardedOperands()[n]);
          }
        }
  } else if (auto field = dyn_cast_or_null<PlaceFieldOp>(op)) {
    path.insert(path.begin(), {0, field.getField()}); follow(field.getBase());
  } else if (auto arm = dyn_cast_or_null<PlaceUnionPayloadOp>(op)) {
    path.insert(path.begin(), {1, arm.getVariant()}); payload = true; follow(arm.getBase());
  } else if (isa_and_nonnull<PlaceArrayElementOp, SliceElementPlaceOp>(op)) {
    path.insert(path.begin(), {2, 0}); follow(op->getOperand(0));
  } else if (auto root = dyn_cast_or_null<PlaceValueOp>(op)) {
    follow(root.getValue());
  } else if (auto root = dyn_cast_or_null<PlaceStorageOp>(op)) {
    follow(root.getStorage());
  } else if (auto root = dyn_cast_or_null<PlaceStaticOp>(op)) {
    out.push_back({{}, SymbolTable::lookupNearestSymbolFrom(op, root.getGlobalAttr()), path, payload});
  } else if (auto deref = dyn_cast_or_null<PlaceDerefOp>(op)) {
    follow(deref.getReference());
  } else if (isa_and_nonnull<RefBorrowSharedOp, RefBorrowMutOp, RefReborrowSharedOp,
                           RefReborrowMutOp, RefCopySharedOp, RefMoveOp,
                           SliceBorrowSharedOp, SliceBorrowMutOp, SliceReborrowSharedOp,
                           SliceReborrowMutOp, SliceCopySharedOp, SliceMoveMutOp,
                           StringViewBorrowOp>(op)) {
    follow(op->getOperand(0));
  } else {
    out.push_back({value, nullptr, path, payload});
  }
  visiting.erase(value);
}

// Canonical origins are detached for one loan/terminal action; no source AST,
// numeric address, inferred last use or target layout participates.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md §117.
SmallVector<Origin> origins(Value value) {
  SmallVector<Origin> result;
  llvm::DenseSet<Value> visiting;
  std::set<Visit> seen;
  collectOrigins(value, {}, false, visiting, seen, result);
  return result;
}

// Distinct stored fields are disjoint. Active variants share storage; array
// elements without a supplied disjointness proof remain conservative.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v11.md §35; P17 §117.
bool overlaps(const Origin &left, const Origin &right) {
  if (left.global || right.global) {
    if (!left.global || left.global != right.global) return false;
  } else if (!sameSource(left.root, right.root)) return false;
  for (unsigned i = 0, n = std::min(left.path.size(), right.path.size()); i < n; ++i) {
    auto a = left.path[i], b = right.path[i];
    if (a.first != 0 || b.first != 0) {
      if (a.first == 1 && b.first == 1 && a.second == b.second) continue;
      return true;
    }
    if (a.second != b.second) return false;
  }
  return true;
}

// These are explicit destruction transitions, not reads or projections. Partial
// destruction is tested at its actual projected Place, not at the whole root.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v13.md §§10,15,33.
Value destroyedOperand(Operation *op) {
  if (auto destroy = dyn_cast<OwnDestroyPlaceOp>(op)) return destroy.getPlace();
  if (auto replace = dyn_cast<OwnReplacePlaceOp>(op)) return replace.getPlace();
  if (auto destroy = dyn_cast<OwnDestroyValueOp>(op)) return destroy.getValue();
  if (auto discard = dyn_cast<OwnDiscardOp>(op)) return discard.getValue();
  return {};
}

// Search every represented CFG path until THIS holder ends/moves or a loop
// redefines its SSA producer. Copies/children are checked as independent holders,
// so ending a parent or another holder cannot authorize destruction. End markers
// must match both source forwarding and the loan's borrow/lifetime facts.
// Rules: rules/mlir/dialect-versions/sec_mlir_dialect_v11.md §§25,37;
// rules/mlir/packages/sec-mlir-dialect_package16.md §§39–43,60–63;
// rules/mlir/packages/sec-mlir-dialect_package17.md §117.
LogicalResult checkLoan(Operation *producer, Value reference,
                        ArrayRef<Origin> referents) {
  ReferenceFacts facts = referenceFacts(reference);
  SmallVector<std::pair<Block *, Operation *>> work{{producer->getBlock(), producer->getNextNode()}};
  llvm::SmallPtrSet<Block *, 16> visited;
  while (!work.empty()) {
    auto [block, first] = work.pop_back_val();
    bool initial = block == producer->getBlock() && first == producer->getNextNode();
    if (!initial && !visited.insert(block).second) continue;
    bool ended = false;
    for (Operation *op = first; op; op = op->getNextNode()) {
      if (op == producer) { ended = true; break; }
      if (auto end = dyn_cast<RefEndBorrowOp>(op)) {
        if (facts.known && facts.borrow == end.getBorrowId() && facts.lifetime == end.getLifetimeId() &&
            sameSource(end.getReference(), reference)) { ended = true; break; }
      }
      if (isa<RefMoveOp, SliceMoveMutOp>(op) && sameSource(op->getOperand(0), reference)) {
        ended = true; break;
      }
      if (Value destroyed = destroyedOperand(op)) {
        auto targets = origins(destroyed);
        for (const Origin &borrowed : referents)
          for (const Origin &target : targets)
            if (overlaps(borrowed, target)) {
              auto diagnostic = op->emitOpError("cannot destroy storage while a payload borrow is live");
              diagnostic.attachNote(producer->getLoc()) << "payload borrow holder begins here";
              return failure();
            }
      }
    }
    if (!ended)
      for (Block *successor : block->getSuccessors())
        work.push_back({successor, &successor->front()});
  }
  return success();
}
} // namespace

// Both Place and ownership verification consume one explicit borrow/destruction
// ordering contract. This does not add a second source-level borrow checker.
// Rules: rules/memory/destruction.md §18(3);
// rules/mlir/packages/sec-mlir-dialect_package17.md §117.
LogicalResult sec::detail::verifyBorrowDestruction(func::FuncOp function) {
  WalkResult result = function.walk([&](Operation *op) {
    for (Value reference : op->getResults()) {
      if (!isa<RefType, RefMutType, SliceType, SliceMutType, StringViewType>(reference.getType())) continue;
      auto referents = origins(reference);
      llvm::erase_if(referents, [](const Origin &origin) { return !origin.payload; });
      if (!referents.empty() && failed(checkLoan(op, reference, referents)))
        return WalkResult::interrupt();
    }
    return WalkResult::advance();
  });
  return result.wasInterrupted() ? failure() : success();
}
