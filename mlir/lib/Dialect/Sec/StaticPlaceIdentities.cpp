// Static/root/storage identities are separate, module-local facts.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.3 (MD-116 §§2,4(6),7).
#include "OperationContracts.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/IR/SymbolTable.h"

using namespace mlir;

LogicalResult sec::detail::verifyStaticPlaceIdentity(Operation *operation) {
  auto root = cast<sec::PlaceStaticOp>(operation);
  auto module = root->getParentOfType<ModuleOp>();
  if (!module) return root.emitOpError("static Place requires a module");
  Operation *global = SymbolTable::lookupNearestSymbolFrom(root, root.getGlobalAttr());
  LogicalResult result = success();
  // Like duplicate declaration checks, report a contradiction only at the
  // later root. Resolve declarations, never source names or symbol suffixes.
  module.walk<WalkOrder::PreOrder>([&](sec::PlaceStaticOp prior) {
    if (prior == root) return WalkResult::interrupt();
    if (prior->getParentOfType<ModuleOp>() != module) return WalkResult::advance();
    Operation *other = SymbolTable::lookupNearestSymbolFrom(prior, prior.getGlobalAttr());
    if (!other) return WalkResult::advance(); // Its own symbol check diagnoses it.
    if (other == global && (prior.getStorageIdentity() != root.getStorageIdentity() ||
                            prior.getAddressSpace() != root.getAddressSpace())) {
      result = root.emitOpError("static Place roots have conflicting storage identity or address space");
    } else if (other != global && prior.getStorageIdentity() == root.getStorageIdentity()) {
      result = root.emitOpError("distinct static Place roots share storage identity");
    } else if (other != global && prior.getPlaceRootId() == root.getPlaceRootId()) {
      result = root.emitOpError("distinct static Place roots share root identity");
    }
    return failed(result) ? WalkResult::interrupt() : WalkResult::advance();
  });
  return result;
}
