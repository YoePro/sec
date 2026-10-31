// MD-077 § 19(5) register.hw.bind roots: verify preservation of the
// Semantic IR binding record, not a second hardware planner (§ 11(3)).
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.5; rules/compiler/semantic_ir.md — § 21(11).
#include "RegisterContracts.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "mlir/IR/BuiltinOps.h"
#include "llvm/ADT/APInt.h"
using namespace mlir;
using namespace sec;

namespace {
// Exact non-negative decimal, so domain addresses beyond 64 bits survive.
bool parseDecimal(StringRef text, llvm::APInt &value) {
  if (text.empty() || !llvm::all_of(text, [](char c) { return c >= '0' && c <= '9'; }) ||
      (text.size() > 1 && text.front() == '0'))
    return false;
  return !text.getAsInteger(10, value);
}
} // namespace

LogicalResult HwBindingOp::verify() {
  auto module = (*this)->getParentOfType<ModuleOp>();
  auto version = module ? module->getAttrOfType<IntegerAttr>("sec.dialect_version") : IntegerAttr();
  if (!version || version.getInt() < 11)
    return emitOpError("sec.hw.binding requires Sec dialect schema 11 or later");
  if ((*this)->getParentOp() != module.getOperation())
    return emitOpError("sec.hw.binding must be a direct child of its module");
  auto instance = dyn_cast<HardwareInstanceType>(getInstanceType());
  if (!instance)
    return emitOpError("instance_type must be !sec.hw_instance<T>");
  auto registerType = RegisterType::get(getContext(), instance.getIdentity());
  auto definition = sec::detail::lookupRegister(*this, registerType);
  if (!definition)
    return emitOpError("bound register type has no definition in sec.registers");
  if (getHardwareBindingId() <= 0)
    return emitOpError("hardware_binding_id must be positive");
  if (getEndpointId().empty() || getHardwareResourceId().empty() || getRegionId().empty() || getDomain().empty())
    return emitOpError("requires resolved endpoint, resource, region and domain identities");
  if (getEndpointKind() != "MemoryMapped")
    return emitOpError("endpoint kind '") << getEndpointKind() << "' has no admitted binding schema";
  llvm::APInt address, size, alignment;
  if (!parseDecimal(getAddress(), address) || !parseDecimal(getSize(), size) || !parseDecimal(getAlignment(), alignment) ||
      size.isZero() || alignment.isZero())
    return emitOpError("requires a decimal address and a positive decimal size and alignment");
  unsigned bitsWidth = std::max(size.getActiveBits() + 4u, 64u);
  llvm::APInt bits = size.zext(bitsWidth) * llvm::APInt(bitsWidth, 8);
  for (ArrayAttr widths : {getReadWidths(), getWriteWidths()})
    for (Attribute attribute : widths) {
      int64_t width = cast<IntegerAttr>(attribute).getInt();
      if (width <= 0 || bits.ult(llvm::APInt(bitsWidth, width)))
        return emitOpError("transfer width ") << width << " is not positive or exceeds the endpoint extent";
    }
  for (Operation *sibling = (*this)->getPrevNode(); sibling; sibling = sibling->getPrevNode())
    if (auto other = dyn_cast<HwBindingOp>(sibling))
      if (other.getHardwareBindingId() == getHardwareBindingId())
        return emitOpError() << "duplicate hardware_binding_id " << getHardwareBindingId();
  return success();
}
