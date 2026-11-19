// The P20 numeric-conversion verification boundary
// (sec-mlir-dialect_package20.md §§ 41, 44-56, 70-72, 79, 81): exact
// conversions are only those a proof can make exact, a checked conversion's
// candidate lives only on its success edge and its reason flags only on the
// failure edge, and the failure edge tests the reasons in priority order and
// maps each to its own ConversionError variant.

#include "sec/Analysis/Passes.h"
#include "ReferenceFacts.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/Dominance.h"
#include "sec/Dialect/Sec/SecAttributes.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"

namespace sec {
#define GEN_PASS_DEF_SECVERIFYNUMERICCONVERSIONS
#include "sec/Analysis/Passes.h.inc"
} // namespace sec

using namespace mlir;

namespace {

bool isFloat(Type type) { return isa<Float32Type, Float64Type, sec::FloatType>(type); }

// The significand precision of a resolved binary format; plain float has
// none until the plan resolves it.
std::optional<unsigned> significandBits(Type type) {
  if (isa<Float32Type>(type))
    return 24;
  if (isa<Float64Type>(type))
    return 53;
  return std::nullopt;
}

// Magnitude bits of an integer-family source; target-sized int/uint have
// none until the plan resolves them.
std::optional<unsigned> magnitudeBits(Type type) {
  if (auto integer = dyn_cast<IntegerType>(type))
    return integer.getWidth() - (integer.isSigned() ? 1 : 0);
  if (isa<sec::CharType>(type))
    return 8;
  if (isa<sec::RuneType>(type))
    return 21;
  return std::nullopt;
}

// A structural proof (source domain or identity) cannot make a narrowing,
// float-to-integer or significand-exceeding integer-to-float conversion
// exact; those need a constant or value-interval proof (§§ 44-50).
LogicalResult verifyExact(sec::NumericConvertExactOp op) {
  Type source = op.getValue().getType(), target = op.getResult().getType();
  if (!isFloat(source) && !isFloat(target))
    return success();
  auto proof = op->getAttrOfType<DictionaryAttr>("sec.conversion_proof");
  auto basis = proof ? proof.getAs<StringAttr>("basis") : StringAttr();
  if (!basis || (basis.getValue() != "source-domain" && basis.getValue() != "identity"))
    return success();
  if (isFloat(source) && isFloat(target)) {
    auto from = significandBits(source), to = significandBits(target);
    if (from && to && *from > *to)
      return op.emitOpError("a narrowing float conversion is exact only by a constant proof");
    return success();
  }
  if (isFloat(source))
    return op.emitOpError("a float-to-integer conversion is exact only by a constant proof");
  auto magnitude = magnitudeBits(source);
  auto precision = significandBits(target);
  if (magnitude && precision && *magnitude > *precision)
    return op.emitOpError("an integer-to-float conversion wider than the significand needs a value proof");
  return success();
}

// The union identity and variant name a failure block constructs, if it
// starts with a union construction.
std::optional<std::pair<StringRef, StringRef>> constructedVariant(Block *block) {
  auto construct = dyn_cast<sec::UnionConstructOp>(&block->front());
  if (!construct)
    return std::nullopt;
  auto union_ = dyn_cast<sec::UnionType>(construct.getResult().getType());
  if (!union_)
    return std::nullopt;
  for (Attribute attribute : union_.getVariants()) {
    auto variant = cast<sec::UnionVariantAttr>(attribute);
    if (variant.getIndex() == construct.getVariant())
      return std::make_pair(union_.getIdentity().getValue(), variant.getName().getValue());
  }
  return std::nullopt;
}

LogicalResult verifyChecked(sec::NumericConvertCheckedOp op) {
  auto function = op->getParentOfType<func::FuncOp>();
  if (!function)
    return success();
  auto branch = dyn_cast_or_null<cf::CondBranchOp>(op->getNextNode());
  if (!branch || branch.getCondition() != op.getFailed() ||
      branch.getTrueDest() == branch.getFalseDest())
    return op.emitOpError("requires an immediate branch on its failed flag with distinct outcomes");
  DominanceInfo dominance(function);
  for (OpOperand &use : op.getCandidate().getUses())
    if (!sec::detail::guardedBy(dominance, op.getFailed(), false, use.getOwner()))
      return op.emitOpError("candidate must be confined to the success edge");
  for (Value reason : op.getReasons())
    for (OpOperand &use : reason.getUses())
      if (!sec::detail::guardedBy(dominance, op.getFailed(), true, use.getOwner()))
        return op.emitOpError("reason flags may only be used on the failure edge");
  // The failure edge tests the reasons in priority order; each reason (the
  // last one by elimination) constructs its own ConversionError variant.
  SmallVector<StringRef> failures;
  if (std::optional<ArrayAttr> list = op.getFailures()) {
    for (Attribute attribute : *list)
      failures.push_back(cast<StringAttr>(attribute).getValue());
  } else if (std::optional<StringRef> failure = op.getFailure()) {
    failures.push_back(*failure);
  }
  Block *block = branch.getTrueDest();
  for (unsigned index = 0; index < failures.size(); ++index) {
    Block *reasonBlock = block;
    if (index + 1 < failures.size()) {
      auto dispatch = dyn_cast<cf::CondBranchOp>(&block->front());
      if (!dispatch || dispatch.getCondition() != op.getReasons()[index])
        return op.emitOpError("the failure edge must test the reasons in priority order");
      reasonBlock = dispatch.getTrueDest();
      block = dispatch.getFalseDest();
    }
    auto constructed = constructedVariant(reasonBlock);
    if (!constructed)
      continue;
    // The exact error family, never a hidden wrapper union (§§ 39-40, 57).
    if (!constructed->first.ends_with("::ConversionError"))
      return op.emitOpError("the failure edge must construct ConversionError, not ")
             << constructed->first;
    if (constructed->second != failures[index])
      return op.emitOpError("reason ") << failures[index]
                                       << " maps to ConversionError." << constructed->second;
  }
  return success();
}

LogicalResult verifyConversions(ModuleOp module) {
  WalkResult result = module.walk([&](Operation *operation) {
    if (auto exact = dyn_cast<sec::NumericConvertExactOp>(operation))
      if (failed(verifyExact(exact)))
        return WalkResult::interrupt();
    if (auto checked = dyn_cast<sec::NumericConvertCheckedOp>(operation))
      if (failed(verifyChecked(checked)))
        return WalkResult::interrupt();
    return WalkResult::advance();
  });
  return failure(result.wasInterrupted());
}

class VerifyNumericConversionsPass final
    : public sec::impl::SecVerifyNumericConversionsBase<VerifyNumericConversionsPass> {
  void runOnOperation() override {
    if (failed(verifyConversions(getOperation())))
      signalPassFailure();
  }
};
} // namespace

// Register the P20 numeric-conversion verification boundary (§ 81, P20-23).
std::unique_ptr<Pass> sec::createSecVerifyNumericConversionsPass() {
  return std::make_unique<VerifyNumericConversionsPass>();
}
