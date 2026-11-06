// Path-sensitive ownership verification (dialect v13 section 33).
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 9-18, 111-114, 117.
// Borrow validity stays with the P15 verifiers (section 117); this pass follows who
// owns a value, including which subobjects remain.
#include "DestructionFacts.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Interfaces/ControlFlowInterfaces.h"
#include "mlir/Pass/Pass.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/DenseSet.h"
#include <map>
#include <string>

namespace sec {
#define GEN_PASS_DEF_SECVERIFYOWNERSHIP
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {
// One subobject's state on a path. A join keeps every state some path reaches.
enum : uint8_t {
  Available = 1 << 0,
  Gone = 1 << 1,       // Moved, destroyed or not yet initialized.
  Pending = 1 << 2,    // Needs destruction and nothing is responsible for it yet.
  Registered = 1 << 3, // A cleanup registration is responsible for it.
};

// A subobject is its root plus a path in the destruction `skipped` spelling; the
// empty path is the whole value.
using Key = std::pair<void *, std::string>;
using State = std::map<Key, uint8_t>;

struct Subobject {
  Value root;
  std::string path;
  bool borrowed = false; // Reached through a reference or a slice.
  bool element = false;  // Reached through an array element.
};

bool isWithin(const std::string &path, const std::string &ancestor) {
  return ancestor.empty() || path == ancestor ||
         (path.size() > ancestor.size() && path.compare(0, ancestor.size(), ancestor) == 0 && path[ancestor.size()] == '.');
}

std::string joinPath(const std::string &path, std::string step) {
  return path.empty() ? step : step + "." + path;
}

Subobject resolve(Value place) {
  Subobject result;
  while (true) {
    Operation *op = place.getDefiningOp();
    if (auto field = dyn_cast_or_null<sec::PlaceFieldOp>(op)) {
      result.path = joinPath(result.path, "field#" + std::to_string(field.getField()));
      place = field.getBase();
    } else if (auto payload = dyn_cast_or_null<sec::PlaceUnionPayloadOp>(op)) {
      result.path = joinPath(result.path, "payload#" + std::to_string(payload.getVariant()));
      place = payload.getBase();
    } else if (auto element = dyn_cast_or_null<sec::PlaceArrayElementOp>(op)) {
      result.element = true;
      result.path.clear();
      place = element.getBase();
    } else if (auto deref = dyn_cast_or_null<sec::PlaceDerefOp>(op)) {
      result.borrowed = true;
      place = deref.getReference();
    } else if (auto storage = dyn_cast_or_null<sec::PlaceStorageOp>(op)) {
      place = storage.getStorage();
    } else if (auto value = dyn_cast_or_null<sec::PlaceValueOp>(op)) {
      place = value.getValue();
    } else {
      result.borrowed |= isa_and_nonnull<sec::SliceElementPlaceOp>(op);
      result.root = place;
      return result;
    }
  }
}

bool isProjection(Operation &op) {
  return isa<sec::PlaceStorageOp, sec::PlaceValueOp, sec::PlaceStaticOp, sec::PlaceFieldOp,
             sec::PlaceUnionPayloadOp, sec::PlaceArrayElementOp, sec::PlaceDerefOp>(op);
}

Value ownedPlaceOperand(Operation &op) {
  if (auto move = dyn_cast<sec::OwnMoveFromPlaceOp>(op))
    return move.getPlace();
  if (auto destroy = dyn_cast<sec::OwnDestroyPlaceOp>(op))
    return destroy.getPlace();
  if (auto initialize = dyn_cast<sec::OwnInitializePlaceOp>(op))
    return initialize.getPlace();
  if (auto replace = dyn_cast<sec::OwnReplacePlaceOp>(op))
    return replace.getPlace();
  if (auto copy = dyn_cast<sec::OwnCopyFromPlaceOp>(op))
    return copy.getPlace();
  if (auto track = dyn_cast<sec::CleanupTrackOwnedOp>(op))
    return isa<sec::PlaceType>(track.getTracked().getType()) ? track.getTracked() : Value();
  return Value();
}

class Verifier {
public:
  Verifier(Region &region, sec::DestructionFacts &facts) : region(region), facts(facts) {}

  LogicalResult run() {
    if (region.empty())
      return success();
    collect();
    entry[&region.front()] = State{};
    bool changed = true;
    while (changed) {
      changed = false;
      for (Block &block : region) {
        auto found = entry.find(&block);
        if (found == entry.end())
          continue;
        State state = found->second;
        enter(block, state);
        for (Operation &op : block)
          apply(op, state);
        changed |= flow(block, state);
      }
    }
    for (Block &block : region) {
      auto found = entry.find(&block);
      if (found == entry.end())
        continue;
      State state = found->second;
      enter(block, state);
      for (Operation &op : block) {
        if (failed(check(op, state)))
          return failure();
        apply(op, state);
      }
    }
    return nested();
  }

private:
  // A local storage starts uninitialized once ownership operations manage it; any
  // other root is available from its definition.
  void collect() {
    for (Block &block : region)
      for (Operation &op : block) {
        if (Value place = ownedPlaceOperand(op)) {
          Subobject target = resolve(place);
          if (isa<sec::StorageType>(target.root.getType()))
            managedStorage.insert(target.root);
        }
        if (auto track = dyn_cast<sec::CleanupTrackOwnedOp>(op)) {
          Subobject target = resolve(track.getTracked());
          registrations[track.getCleanupActionId()] = {target.root, target.path, track.getCleanupScopeId()};
        }
      }
  }

  // A defer body is its own isolated region whose captures start available.
  LogicalResult nested() {
    for (Block &block : region)
      for (Operation &op : block)
        if (auto defer = dyn_cast<sec::CleanupDeferRegisterOp>(op))
          if (failed(Verifier(defer.getBody(), facts).run()))
            return failure();
    return success();
  }

  uint8_t defaultFlags(Value root) const { return managedStorage.contains(root) ? uint8_t(Gone) : uint8_t(Available); }

  // flagsAt is the explicit state of the subobject or its nearest explicit ancestor.
  uint8_t flagsAt(const State &state, Value root, const std::string &path) const {
    std::string current = path;
    while (true) {
      auto found = state.find({root.getAsOpaquePointer(), current});
      if (found != state.end())
        return found->second;
      if (current.empty())
        return defaultFlags(root);
      size_t dot = current.rfind('.');
      current = dot == std::string::npos ? std::string() : current.substr(0, dot);
    }
  }

  // Descendants holds every explicit state strictly below the subobject.
  template <typename Fn>
  void forDescendants(const State &state, Value root, const std::string &path, Fn fn) const {
    void *key = root.getAsOpaquePointer();
    for (auto it = state.lower_bound({key, std::string()}); it != state.end() && it->first.first == key; ++it)
      if (it->first.second != path && isWithin(it->first.second, path))
        fn(it->first.second, it->second);
  }

  bool anyGone(const State &state, Value root, const std::string &path) const {
    bool gone = flagsAt(state, root, path) & Gone;
    forDescendants(state, root, path, [&](const std::string &, uint8_t flags) { gone |= (flags & Gone) != 0; });
    return gone;
  }

  bool allGone(const State &state, Value root, const std::string &path) const {
    bool available = flagsAt(state, root, path) & Available;
    forDescendants(state, root, path, [&](const std::string &, uint8_t flags) { available |= (flags & Available) != 0; });
    return !available;
  }

  void set(State &state, Value root, const std::string &path, uint8_t flags) const {
    SmallVector<std::string> below;
    forDescendants(state, root, path, [&](const std::string &item, uint8_t) { below.push_back(item); });
    for (const std::string &item : below)
      state.erase({root.getAsOpaquePointer(), item});
    state[{root.getAsOpaquePointer(), path}] = flags;
  }

  uint8_t freshFlags(Type type) { return Available | (facts.needsDestruction(type) ? uint8_t(Pending) : uint8_t(0)); }

  void define(Value value, State &state) {
    if (managedStorage.contains(value))
      set(state, value, "", Gone);
    else
      set(state, value, "", freshFlags(value.getType()));
  }

  void enter(Block &block, State &state) {
    for (BlockArgument argument : block.getArguments())
      define(argument, state);
  }

  // transferAction is how an operation takes its operand: "" for a read, otherwise
  // the action. A value that needs destruction always transfers unless it is copied.
  StringRef transferAction(Operation &op, unsigned index) {
    ArrayAttr actions;
    if (isa<sec::CallDirectOp, sec::CallForeignOp>(op))
      actions = op.getAttrOfType<ArrayAttr>("sec.argument_actions");
    else if (auto structure = dyn_cast<sec::StructConstructOp>(op))
      actions = structure.getFieldActions();
    else if (auto array = dyn_cast<sec::ArrayConstructOp>(op))
      actions = array.getSegmentActions();
    else if (auto sum = dyn_cast<sec::UnionConstructOp>(op))
      actions = sum.getPayloadActions();
    StringRef action;
    if (actions && index < actions.size())
      if (auto text = dyn_cast<StringAttr>(actions[index]))
        action = text.getValue();
    if (actions) {
      if (action.starts_with("copy"))
        return action;
      if (action == "move" || facts.needsDestruction(op.getOperand(index).getType()))
        return action.empty() ? StringRef("move") : action;
      return "";
    }
    if (isa<sec::OwnInitializePlaceOp, sec::OwnReplacePlaceOp>(op))
      return index == 1 ? StringRef("move") : StringRef();
    if (isa<func::ReturnOp, sec::ResultOkOp, sec::ResultErrOp>(op))
      return facts.needsDestruction(op.getOperand(index).getType()) ? StringRef("move") : StringRef();
    // Projecting a payload that needs destruction takes the whole container.
    if (isPayloadProjection(op))
      return facts.needsDestruction(op.getResult(0).getType()) ? StringRef("move") : StringRef();
    return "";
  }

  static bool isPayloadProjection(Operation &op) {
    return isa<sec::ResultUnwrapOkOp, sec::ResultUnwrapErrOp, sec::UnionUnwrapPayloadOp, sec::UnionUnwrapFieldOp>(op);
  }

  void apply(Operation &op, State &state) {
    for (auto [index, operand] : llvm::enumerate(op.getOperands())) {
      StringRef action = transferAction(op, index);
      if (!action.empty() && !action.starts_with("copy"))
        set(state, operand, "", Gone);
    }
    // On its proven path a whole-payload projection saw all the container holds,
    // so one that needs no destruction ends an owner no registration still covers.
    // A field projection sees one field of several and ends nothing.
    if (isPayloadProjection(op) && !isa<sec::UnionUnwrapFieldOp>(op) && facts.needsDestruction(op.getOperand(0).getType()) &&
        !(flagsAt(state, op.getOperand(0), "") & Registered))
      set(state, op.getOperand(0), "", Gone);
    if (auto move = dyn_cast<sec::OwnMoveOp>(op))
      set(state, move.getSource(), "", Gone);
    else if (auto discard = dyn_cast<sec::OwnDiscardOp>(op))
      set(state, discard.getValue(), "", Gone);
    else if (auto destroy = dyn_cast<sec::OwnDestroyValueOp>(op))
      set(state, destroy.getValue(), "", Gone);
    else if (auto move = dyn_cast<sec::OwnMoveFromPlaceOp>(op)) {
      Subobject target = resolve(move.getPlace());
      // An active payload is all its container holds, so the container ends with
      // it, and so on outward through nested active payloads.
      std::string path = target.path;
      while (true) {
        size_t dot = path.rfind('.');
        StringRef last = StringRef(path).substr(dot == std::string::npos ? 0 : dot + 1);
        if (path.empty() || !last.starts_with("payload#"))
          break;
        path = dot == std::string::npos ? std::string() : path.substr(0, dot);
      }
      set(state, target.root, path, Gone);
    } else if (auto destroy = dyn_cast<sec::OwnDestroyPlaceOp>(op)) {
      Subobject target = resolve(destroy.getPlace());
      // The owner's registration stays with the Place (MD-125): a following
      // reinitialization is covered by it again.
      uint8_t kept = flagsAt(state, target.root, target.path) & Registered;
      set(state, target.root, target.path, Gone | kept);
    } else if (auto initialize = dyn_cast<sec::OwnInitializePlaceOp>(op)) {
      Subobject target = resolve(initialize.getPlace());
      // A reinitialized subobject joins its owner, whose terminal action covers it;
      // a whole owner whose registration was kept is covered by it again (MD-125).
      uint8_t kept = flagsAt(state, target.root, target.path) & Registered;
      set(state, target.root, target.path, target.path.empty() ? uint8_t(kept ? (freshFlags(initialize.getValue().getType()) & ~uint8_t(Pending)) | kept : freshFlags(initialize.getValue().getType())) : uint8_t(Available));
    } else if (auto replace = dyn_cast<sec::OwnReplacePlaceOp>(op)) {
      // The old value ends and the new one takes its place, whole again; the old
      // registration's cancel and the new registration follow.
      Subobject target = resolve(replace.getPlace());
      if (replace.getIfOwned())
        set(state, target.root, target.path, Available | Registered);
      else
        set(state, target.root, target.path, flagsAt(state, target.root, target.path));
    } else if (auto store = dyn_cast<sec::StorageInitOp>(op)) {
      if (managedStorage.contains(store.getStorage()))
        set(state, store.getStorage(), "", Available);
    } else if (auto store = dyn_cast<sec::StorageStoreOp>(op)) {
      if (managedStorage.contains(store.getStorage()))
        set(state, store.getStorage(), "", Available);
    } else if (auto write = dyn_cast<sec::PlaceWriteOp>(op)) {
      Subobject target = resolve(write.getPlace());
      if (!target.element)
        set(state, target.root, target.path, flagsAt(state, target.root, target.path) | Available);
    } else if (auto track = dyn_cast<sec::CleanupTrackOwnedOp>(op)) {
      Subobject target = resolve(track.getTracked());
      uint8_t flags = flagsAt(state, target.root, target.path);
      state[{target.root.getAsOpaquePointer(), target.path}] = (flags & ~uint8_t(Pending)) | Registered;
    } else if (auto cancel = dyn_cast<sec::CleanupCancelOp>(op)) {
      // A value still available after its cancel has lost its only responsibility.
      auto found = registrations.find(cancel.getCleanupActionId());
      if (found != registrations.end()) {
        Value root = found->second.root;
        uint8_t flags = flagsAt(state, root, found->second.path) & ~uint8_t(Registered);
        if (flags & Available)
          flags |= Pending;
        state[{root.getAsOpaquePointer(), found->second.path}] = flags;
      }
    } else if (auto scope = dyn_cast<sec::CleanupRunScopeOp>(op)) {
      for (auto &[action, registration] : registrations)
        if (llvm::is_contained(scope.getScopeIds(), registration.scope) &&
            (flagsAt(state, registration.root, registration.path) & Registered))
          set(state, registration.root, registration.path, Gone);
    }
    for (Value result : op.getResults())
      define(result, state);
    // An observing read owns nothing.
    if (auto read = dyn_cast<sec::PlaceReadOp>(op); read && read.getAction() != "copy-trivial")
      set(state, read.getResult(), "", Available);
  }

  // flow moves a terminator's state along each edge. Exclusive successors may
  // transfer the same owner; each edge consumes only its own operands.
  bool flow(Block &block, const State &state) {
    bool changed = false;
    Operation *terminator = block.getTerminator();
    auto branch = dyn_cast<BranchOpInterface>(terminator);
    for (unsigned index = 0; index < terminator->getNumSuccessors(); ++index) {
      State edge = state;
      if (branch)
        for (Value operand : branch.getSuccessorOperands(index).getForwardedOperands())
          if (facts.needsDestruction(operand.getType()))
            set(edge, operand, "", Gone);
      changed |= merge(terminator->getSuccessor(index), edge);
    }
    return changed;
  }

  bool merge(Block *block, const State &incoming) {
    auto [slot, inserted] = entry.try_emplace(block, incoming);
    if (inserted)
      return true;
    State joined;
    auto add = [&](const State &side) {
      for (auto &[key, unused] : side) {
        Value root = Value::getFromOpaquePointer(key.first);
        joined[key] = flagsAt(slot->second, root, key.second) | flagsAt(incoming, root, key.second);
      }
    };
    add(slot->second);
    add(incoming);
    bool changed = joined != slot->second;
    slot->second = std::move(joined);
    return changed;
  }

  // A value that needs destruction crosses a call result, a return or a branch
  // edge only as a move, and that move is written down (dialect v13 §§ 25-27).
  LogicalResult checkAction(Operation &op, Attribute action, Type type, StringRef what) {
    auto text = dyn_cast_or_null<StringAttr>(action);
    if (facts.needsDestruction(type) && (!text || text.getValue() != "move"))
      return op.emitOpError() << what << " needs destruction, so its ownership action must be \"move\"";
    if (text && text.getValue() == "non-owning" && !isa<sec::RefType, sec::RefMutType, sec::SliceType, sec::SliceMutType, sec::StringViewType>(type))
      return op.emitOpError() << what << " is not a reference or slice holder, so it cannot be \"non-owning\"";
    return success();
  }

  LogicalResult checkActionMetadata(Operation &op) {
    if (isa<sec::CallDirectOp, sec::CallForeignOp>(op) && op.getNumResults() == 1)
      return checkAction(op, op.getAttr("sec.result_action"), op.getResult(0).getType(), "the call result");
    if (isa<func::ReturnOp>(op) && op.getNumOperands() == 1)
      return checkAction(op, op.getAttr("sec.return_action"), op.getOperand(0).getType(), "the returned value");
    auto branch = dyn_cast<BranchOpInterface>(op);
    if (!branch)
      return success();
    auto lists = op.getAttrOfType<ArrayAttr>("sec.successor_actions");
    for (unsigned index = 0; index < op.getNumSuccessors(); ++index) {
      ArrayAttr actions = lists && index < lists.size() ? dyn_cast<ArrayAttr>(lists[index]) : ArrayAttr();
      for (auto [position, operand] : llvm::enumerate(branch.getSuccessorOperands(index).getForwardedOperands())) {
        Attribute action = actions && position < actions.size() ? actions[position] : Attribute();
        if (failed(checkAction(op, action, operand.getType(), "a forwarded branch operand")))
          return failure();
      }
    }
    return success();
  }

  LogicalResult requireAvailable(Operation &op, const State &state, Value root, const std::string &path) {
    if (!anyGone(state, root, path))
      return success();
    return op.emitOpError("uses a value that is moved, destroyed or uninitialized on some path");
  }

  LogicalResult checkCopy(Operation &op, Type type) {
    if (facts.needsDestruction(type))
      return op.emitOpError("copies a value that needs destruction");
    if (isa<sec::RefMutType, sec::SliceMutType>(type))
      return op.emitOpError("copies a mutable reference");
    return success();
  }

  // A registered value leaves through own.move, so its cancel is explicit.
  LogicalResult checkTransfer(Operation &op, const State &state, Value operand) {
    if (flagsAt(state, operand, "") & Registered)
      return op.emitOpError("transfers a value still registered for cleanup without sec.own.move");
    return success();
  }

  LogicalResult checkMoveFromPlace(Operation &op, const Subobject &target) {
    if (target.borrowed)
      return op.emitOpError("moves out of a borrowed referent");
    if (target.element)
      return op.emitOpError("moves out of an array element, which has no partial state in this package");
    // A custom free relies on complete fields (destruction.md section 10.3).
    if (!target.path.empty())
      for (Value place = cast<sec::OwnMoveFromPlaceOp>(op).getPlace();;) {
        Operation *def = place.getDefiningOp();
        Value base;
        if (auto field = dyn_cast_or_null<sec::PlaceFieldOp>(def))
          base = field.getBase();
        else if (auto payload = dyn_cast_or_null<sec::PlaceUnionPayloadOp>(def))
          base = payload.getBase();
        if (!base)
          break;
        if (facts.hasCustomFree(cast<sec::PlaceType>(base.getType()).getElementType()))
          return op.emitOpError("moves a field out of a value with custom free");
        place = base;
      }
    return success();
  }

  LogicalResult checkDestroyPlace(sec::OwnDestroyPlaceOp op, const State &state, const Subobject &target) {
    if (flagsAt(state, target.root, target.path) & Gone)
      return op.emitOpError("destroys a value that is moved or uninitialized on some path");
    SmallVector<StringRef> skipped;
    if (auto list = op.getSkipped())
      for (Attribute attribute : *list)
        skipped.push_back(cast<StringAttr>(attribute).getValue());
    std::string missing;
    forDescendants(state, target.root, target.path, [&](const std::string &path, uint8_t flags) {
      if ((flags & Gone) && !llvm::is_contained(skipped, relative(path, target.path)))
        missing = path;
    });
    if (!missing.empty())
      return op.emitOpError("destroys moved subobject '") << missing << "' without skipping it";
    for (StringRef path : skipped) {
      std::string full = target.path.empty() ? path.str() : target.path + "." + path.str();
      if (flagsAt(state, target.root, full) & Available)
        return op.emitOpError("skips subobject '") << path << "', which is available on some path";
    }
    return success();
  }

  static std::string relative(const std::string &path, const std::string &ancestor) {
    return ancestor.empty() ? path : path.substr(ancestor.size() + 1);
  }

  LogicalResult check(Operation &op, const State &state) {
    for (Value result : op.getResults())
      if (flagsAt(state, result, "") & Pending)
        return op.emitOpError("redefines a value that was never destroyed or transferred on the previous pass");
    if (isProjection(op))
      return success();
    // A candidate views its Place, so it ends when the payload leaves it.
    for (Value operand : op.getOperands())
      if (auto read = operand.getDefiningOp<sec::PlaceReadOp>(); read && read.getAction() == "candidate") {
        Subobject target = resolve(read.getPlace());
        if (failed(requireAvailable(op, state, target.root, target.path)))
          return failure();
      }
    if (auto copy = dyn_cast<sec::OwnCopyOp>(op))
      if (failed(checkCopy(op, copy.getSource().getType())))
        return failure();
    if (auto copy = dyn_cast<sec::OwnCopyFromPlaceOp>(op))
      if (copy.getCopyMode() == "trivial" && failed(checkCopy(op, copy.getResult().getType())))
        return failure();
    if (auto read = dyn_cast<sec::PlaceReadOp>(op)) {
      if (read.getAction() != "copy-trivial") {
        Subobject target = resolve(read.getPlace());
        return requireAvailable(op, state, target.root, target.path);
      }
      if (facts.needsDestruction(read.getResult().getType()))
        return op.emitOpError("copies a value that needs destruction");
    }
    if (auto extract = dyn_cast<sec::StructExtractOp>(op))
      if (failed(checkCopy(op, extract.getResult().getType())))
        return failure();
    if (auto initialize = dyn_cast<sec::OwnInitializePlaceOp>(op)) {
      Subobject target = resolve(initialize.getPlace());
      if (!target.path.empty()) {
        size_t dot = target.path.rfind('.');
        if (flagsAt(state, target.root, dot == std::string::npos ? std::string() : target.path.substr(0, dot)) & Gone)
          return op.emitOpError("initializes a subobject of an unavailable value");
      }
      if (!allGone(state, target.root, target.path))
        return op.emitOpError("initializes a Place that may still own a value; a live old value needs sec.own.replace_place");
    }
    if (auto replace = dyn_cast<sec::OwnReplacePlaceOp>(op); replace && !replace.getIfOwned()) {
      Subobject target = resolve(replace.getPlace());
      if (failed(requireAvailable(op, state, target.root, target.path)))
        return failure();
    }
    if (auto move = dyn_cast<sec::OwnMoveFromPlaceOp>(op)) {
      Subobject target = resolve(move.getPlace());
      if (failed(checkMoveFromPlace(op, target)) || failed(requireAvailable(op, state, target.root, target.path)))
        return failure();
    }
    if (auto destroy = dyn_cast<sec::OwnDestroyPlaceOp>(op); destroy && !destroy.getIfOwned()) {
      Subobject target = resolve(destroy.getPlace());
      if (failed(checkDestroyPlace(destroy, state, target)))
        return failure();
    }
    for (auto [index, operand] : llvm::enumerate(op.getOperands())) {
      if (isa<sec::OwnInitializePlaceOp, sec::OwnReplacePlaceOp, sec::OwnMoveFromPlaceOp, sec::OwnDestroyPlaceOp,
              sec::PlaceWriteOp, sec::StorageInitOp, sec::StorageStoreOp>(op) && index == 0)
        continue;
      if (auto discard = dyn_cast<sec::OwnDiscardOp>(op); discard && discard.getIfOwned())
        continue;
      Subobject target = isa<sec::PlaceType>(operand.getType()) ? resolve(operand) : Subobject{operand, "", false, false};
      if (failed(requireAvailable(op, state, target.root, target.path)))
        return failure();
      StringRef action = transferAction(op, index);
      if (action == "copy-trivial" && failed(checkCopy(op, operand.getType())))
        return failure();
      if (!action.empty() && !action.starts_with("copy") && failed(checkTransfer(op, state, operand)))
        return failure();
    }
    if (failed(checkActionMetadata(op)))
      return failure();
    if (auto branch = dyn_cast<BranchOpInterface>(op))
      for (unsigned index = 0; index < op.getNumSuccessors(); ++index) {
        llvm::SmallDenseSet<Value> seen;
        for (Value operand : branch.getSuccessorOperands(index).getForwardedOperands()) {
          if (!facts.needsDestruction(operand.getType()))
            continue;
          if (!seen.insert(operand).second)
            return op.emitOpError("transfers one owner twice along one edge");
          if (failed(checkTransfer(op, state, operand)))
            return failure();
        }
      }
    if (isa<func::ReturnOp, sec::CleanupDeferYieldOp>(op)) {
      State after = state;
      for (auto [index, operand] : llvm::enumerate(op.getOperands()))
        if (!transferAction(op, index).empty())
          set(after, operand, "", Gone);
      for (auto &[key, flags] : after)
        if (flags & Pending)
          return op.emitOpError("leaves an owned value with no terminal ownership action");
    }
    return success();
  }

  struct Tracked {
    Value root;
    std::string path;
    int32_t scope = 0;
  };

  Region &region;
  sec::DestructionFacts &facts;
  llvm::DenseSet<Value> managedStorage;
  llvm::DenseMap<int32_t, Tracked> registrations;
  llvm::DenseMap<Block *, State> entry;
};

class VerifyOwnershipPass final : public sec::impl::SecVerifyOwnershipBase<VerifyOwnershipPass> {
public:
  void runOnOperation() override {
    ModuleOp module = getOperation();
    auto version = module->getAttrOfType<IntegerAttr>("sec.dialect_version");
    if (!version || version.getInt() < 13)
      return;
    sec::DestructionFacts facts(module);
    for (auto function : module.getOps<func::FuncOp>())
      if (failed(Verifier(function.getBody(), facts).run()))
        return signalPassFailure();
  }
};
} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyOwnershipPass() {
  return std::make_unique<VerifyOwnershipPass>();
}
