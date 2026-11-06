// Path-sensitive cleanup verification (dialect v13 sections 15-22, 32, 35).
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — sections 38-47, 82, 115, 131.
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/Dominance.h"
#include "mlir/Pass/Pass.h"
#include "sec/Analysis/Passes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "llvm/ADT/DenseMap.h"
#include <map>

namespace sec {
#define GEN_PASS_DEF_SECVERIFYCLEANUPS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {
// One registration's state on a path. A join keeps every state some path reaches,
// so a transfer on one branch and live cleanup on another stay distinct facts
// rather than one approximate flag.
enum : uint8_t {
  Unregistered = 1 << 0, // No active responsibility: never registered, ended or scope-exited.
  Live = 1 << 1,         // Cleanup will destroy the tracked value, or run the defer.
  Consumed = 1 << 2,     // The value was moved or destroyed; cancel has not run yet.
};

struct Registration {
  Value root;
  bool isDefer = false;
  int32_t scope = 0;
  Operation *site = nullptr;
  SmallVector<Value> captures; // The roots an active defer still reads (section 113).
};

using State = std::map<int32_t, uint8_t>;

uint8_t stateOf(const State &state, int32_t action) {
  auto found = state.find(action);
  return found == state.end() ? uint8_t(Unregistered) : found->second;
}

// rootOf identifies an owner across the fresh Place root each use builds; a
// subobject counts as its whole owner.
Value rootOf(Value value) {
  while (true) {
    if (auto place = value.getDefiningOp<sec::PlaceStorageOp>())
      return place.getStorage();
    if (auto field = value.getDefiningOp<sec::PlaceFieldOp>())
      value = field.getBase();
    else if (auto payload = value.getDefiningOp<sec::PlaceUnionPayloadOp>())
      value = payload.getBase();
    else
      return value;
  }
}

// endsWhole reports a Place that is its whole owner or an active payload, which is
// all a union holds. A moved or destroyed stored field leaves the owner
// registered, and its cleanup destroys only the remaining fields (package17
// sections 12, 58-59).
bool endsWhole(Value place) {
  while (true) {
    if (place.getDefiningOp<sec::PlaceFieldOp>())
      return false;
    if (auto payload = place.getDefiningOp<sec::PlaceUnionPayloadOp>())
      place = payload.getBase();
    else
      return true;
  }
}

// consumedValue is the owner an ownership operation ends, if any.
Value consumedValue(Operation *op) {
  if (auto move = dyn_cast<sec::OwnMoveOp>(op))
    return rootOf(move.getSource());
  if (auto discard = dyn_cast<sec::OwnDiscardOp>(op))
    return rootOf(discard.getValue());
  if (auto destroy = dyn_cast<sec::OwnDestroyValueOp>(op))
    return rootOf(destroy.getValue());
  if (auto move = dyn_cast<sec::OwnMoveFromPlaceOp>(op))
    return endsWhole(move.getPlace()) ? rootOf(move.getPlace()) : Value();
  if (auto destroy = dyn_cast<sec::OwnDestroyPlaceOp>(op))
    return destroy.getSkipped() || !endsWhole(destroy.getPlace()) ? Value() : rootOf(destroy.getPlace());
  return Value();
}

bool isCleanupOp(Operation *op) {
  return isa<sec::CleanupTrackOwnedOp, sec::CleanupCancelOp, sec::CleanupDeferRegisterOp,
             sec::CleanupRunScopeOp, sec::CleanupRunFunctionOp>(op);
}

class Verifier {
public:
  explicit Verifier(func::FuncOp function) : function(function) {}

  LogicalResult run() {
    if (failed(collect()))
      return failure();
    if (registrations.empty() && !hasExitOps)
      return success();
    solve();
    return check();
  }

private:
  LogicalResult collect() {
    llvm::DenseSet<int32_t> ordinals;
    for (Block &block : function.getBody()) {
      for (Operation &op : block) {
        if (isa<sec::CleanupRunFunctionOp, sec::CleanupRunScopeOp, sec::CleanupCancelOp>(op))
          hasExitOps = true;
        if (auto track = dyn_cast<sec::CleanupTrackOwnedOp>(op)) {
          if (failed(add(&op, track.getCleanupActionId(), rootOf(track.getTracked()),
                         false, track.getCleanupScopeId(), track.getCleanupRegistrationOrdinal(), ordinals)))
            return failure();
        } else if (auto defer = dyn_cast<sec::CleanupDeferRegisterOp>(op)) {
          if (failed(add(&op, defer.getCleanupActionId(), Value(), true,
                         defer.getCleanupScopeId(), defer.getCleanupRegistrationOrdinal(), ordinals)))
            return failure();
          for (Value capture : defer.getCaptures())
            registrations[defer.getCleanupActionId()].captures.push_back(rootOf(capture));
          // A defer body holds no registration or exit of its own (section 45).
          WalkResult nested = defer.getBody().walk([&](Operation *inner) {
            if (isCleanupOp(inner))
              return inner->emitOpError("is not valid inside a defer body"), WalkResult::interrupt();
            return WalkResult::advance();
          });
          if (nested.wasInterrupted())
            return failure();
        }
      }
    }
    return success();
  }

  LogicalResult add(Operation *op, int32_t action, Value root, bool isDefer, int32_t scope,
                    int32_t ordinal, llvm::DenseSet<int32_t> &ordinals) {
    if (!registrations.try_emplace(action, Registration{root, isDefer, scope, op}).second)
      return op->emitOpError("reuses a cleanup action identity");
    if (!ordinals.insert(ordinal).second)
      return op->emitOpError("reuses a registration ordinal");
    return success();
  }

  // apply is the transfer of one operation; reporting happens only in check, once
  // the fixed point holds, so a loop's first pass never reports a false state.
  void apply(Operation &op, State &state) {
    if (auto track = dyn_cast<sec::CleanupTrackOwnedOp>(op)) {
      // MD-125: a conditional registration is live only on paths where none
      // of the inherited registrations is.
      if (auto unless = track.getUnlessActive()) {
        int32_t self = track.getCleanupActionId();
        bool otherLive = false, someNot = false, selfLive = false;
        for (int32_t inherited : *unless) {
          uint8_t s = stateOf(state, inherited);
          if (inherited == self)
            selfLive = s & Live;
          else
            otherLive |= bool(s & Live);
          someNot |= bool(s & ~uint8_t(Live));
        }
        uint8_t next = 0;
        if (selfLive)
          next |= Live; // an earlier iteration's instance keeps its position
        if (otherLive)
          next |= Unregistered; // the inherited registration stays responsible
        if (someNot || (!selfLive && !otherLive))
          next |= Live;
        state[self] = next;
        return;
      }
      state[track.getCleanupActionId()] = Live;
      return;
    }
    if (auto defer = dyn_cast<sec::CleanupDeferRegisterOp>(op)) {
      state[defer.getCleanupActionId()] = Live;
      return;
    }
    if (auto cancel = dyn_cast<sec::CleanupCancelOp>(op)) {
      state[cancel.getCleanupActionId()] = Unregistered;
      return;
    }
    if (auto scope = dyn_cast<sec::CleanupRunScopeOp>(op)) {
      for (auto &[action, registration] : registrations)
        if (!registration.isDefer && llvm::is_contained(scope.getScopeIds(), registration.scope))
          state[action] = Unregistered;
      return;
    }
    if (isa<sec::CleanupRunFunctionOp>(op)) {
      for (auto &entry : state)
        entry.second = Unregistered;
      return;
    }
    // MD-125: reinitializing a whole owner whose registration was kept (a
    // replacement after its remaining fields were destroyed) makes that same
    // registration responsible for the new value again.
    if (auto initialize = dyn_cast<sec::OwnInitializePlaceOp>(op)) {
      if (endsWhole(initialize.getPlace())) {
        Value owner = rootOf(initialize.getPlace());
        for (auto &[action, registration] : registrations)
          if (!registration.isDefer && registration.root == owner) {
            uint8_t current = stateOf(state, action);
            if (current & Consumed)
              state[action] = (current & ~uint8_t(Consumed)) | Live;
          }
      }
      return;
    }
    if (Value owner = consumedValue(&op)) {
      for (auto &[action, registration] : registrations)
        if (registration.root == owner) {
          uint8_t current = stateOf(state, action);
          if (current & Live)
            state[action] = (current & ~uint8_t(Live)) | Consumed;
        }
    }
  }

  void solve() {
    entry[&function.getBody().front()] = State{};
    bool changed = true;
    while (changed) {
      changed = false;
      for (Block &block : function.getBody()) {
        auto found = entry.find(&block);
        if (found == entry.end())
          continue;
        State state = found->second;
        for (Operation &op : block)
          apply(op, state);
        for (Block *successor : block.getSuccessors())
          changed |= merge(successor, state);
      }
    }
  }

  // merge unions states; an action missing on one side is Unregistered there.
  bool merge(Block *block, const State &incoming) {
    auto [slot, inserted] = entry.try_emplace(block, incoming);
    if (inserted)
      return true;
    bool changed = false;
    State &target = slot->second;
    for (auto &[action, unused] : registrations) {
      uint8_t joined = stateOf(target, action) | stateOf(incoming, action);
      if (joined != stateOf(target, action)) {
        target[action] = joined;
        changed = true;
      }
    }
    return changed;
  }

  LogicalResult check() {
    for (Block &block : function.getBody()) {
      auto found = entry.find(&block);
      if (found == entry.end())
        continue;
      State state = found->second;
      int32_t lastOrdinal = 0;
      for (Operation &op : block) {
        if (failed(checkOperation(op, state, lastOrdinal)))
          return failure();
        apply(op, state);
      }
    }
    return success();
  }

  LogicalResult checkOperation(Operation &op, const State &state, int32_t &lastOrdinal) {
    int32_t ordinal = 0;
    if (auto track = dyn_cast<sec::CleanupTrackOwnedOp>(op))
      ordinal = track.getCleanupRegistrationOrdinal();
    else if (auto defer = dyn_cast<sec::CleanupDeferRegisterOp>(op))
      ordinal = defer.getCleanupRegistrationOrdinal();
    if (ordinal != 0) {
      if (ordinal <= lastOrdinal)
        return op.emitOpError("registers out of source registration order");
      lastOrdinal = ordinal;
    }
    // An active defer still reads what it captured (section 113).
    if (Value owner = consumedValue(&op))
      for (auto &[action, registration] : registrations)
        if (registration.isDefer && (stateOf(state, action) & Live) && llvm::is_contained(registration.captures, owner))
          return op.emitOpError("consumes a value an active defer captures");
    if (auto cancel = dyn_cast<sec::CleanupCancelOp>(op)) {
      auto registration = registrations.find(cancel.getCleanupActionId());
      if (registration == registrations.end())
        return op.emitOpError("cancels an action that is never registered");
      if (registration->second.isDefer)
        return op.emitOpError("cancels a defer registration, which always runs");
      if (!cancel.getIfActive() && (stateOf(state, cancel.getCleanupActionId()) & Unregistered))
        return op.emitOpError("cancels an action that is not active on every path");
    }
    if (auto track = dyn_cast<sec::CleanupTrackOwnedOp>(op)) {
      // A conditional registration that names itself keeps an earlier
      // iteration's live instance instead of registering again (MD-125).
      auto unless = track.getUnlessActive();
      bool selfInherited = unless && llvm::is_contained(*unless, track.getCleanupActionId());
      if (!selfInherited && (stateOf(state, track.getCleanupActionId()) & (Live | Consumed)))
        return op.emitOpError("registers an action that is still active on a path");
      // Its cleanup would run before an earlier defer that reads the same owner
      // (destruction.md sections 6.3(3), 35.6(2)).
      Value owner = rootOf(track.getTracked());
      for (auto &[action, registration] : registrations)
        if (registration.isDefer && (stateOf(state, action) & Live) && llvm::is_contained(registration.captures, owner))
          return op.emitOpError("registers an owner an active defer reads, whose cleanup would run before the defer");
    }
    if (auto scope = dyn_cast<sec::CleanupRunScopeOp>(op))
      for (auto &[action, registration] : registrations) {
        if (registration.isDefer || !llvm::is_contained(scope.getScopeIds(), registration.scope))
          continue;
        if (stateOf(state, action) & Consumed)
          return op.emitOpError("would destroy a moved value");
        // A defer still pending at function exit reads this owner
        // (defer.md section 8).
        if (stateOf(state, action) & Live)
          for (auto &[deferAction, deferred] : registrations)
            if (deferred.isDefer && (stateOf(state, deferAction) & Live) &&
                llvm::is_contained(deferred.captures, registration.root))
              return op.emitOpError("would destroy an owner an active defer reads before the defer runs");
      }
    if (isa<sec::CleanupRunFunctionOp>(op)) {
      if (!isa<func::ReturnOp>(op.getNextNode()))
        return op.emitOpError("must immediately precede the function's return");
      for (auto &[action, registration] : registrations)
        if (stateOf(state, action) & Consumed)
          return op.emitOpError("would destroy a moved value");
    }
    if (auto ret = dyn_cast<func::ReturnOp>(op)) {
      Operation *previous = op.getPrevNode();
      bool ran = previous && isa<sec::CleanupRunFunctionOp>(previous);
      if (!ran)
        for (auto &[action, registration] : registrations)
          if (stateOf(state, action) & (Live | Consumed))
            return op.emitOpError("returns with a live cleanup registration and no sec.cleanup.run_function");
    }
    // The cleanup at a function exit never destroys the returned value.
    if (auto run = dyn_cast<sec::CleanupRunFunctionOp>(op))
      if (auto ret = dyn_cast<func::ReturnOp>(op.getNextNode()))
        for (Value operand : ret.getOperands())
          for (auto &[action, registration] : registrations)
            if (registration.root == rootOf(operand) && (stateOf(state, action) & Live))
              return op.emitOpError("would destroy the returned value");
    return success();
  }

  func::FuncOp function;
  bool hasExitOps = false;
  llvm::DenseMap<int32_t, Registration> registrations;
  llvm::DenseMap<Block *, State> entry;
};

class VerifyCleanupsPass final : public sec::impl::SecVerifyCleanupsBase<VerifyCleanupsPass> {
public:
  void runOnOperation() override {
    if (failed(Verifier(getOperation()).run()))
      signalPassFailure();
  }
};
} // namespace

std::unique_ptr<mlir::Pass> sec::createSecVerifyCleanupsPass() {
  return std::make_unique<VerifyCleanupsPass>();
}
