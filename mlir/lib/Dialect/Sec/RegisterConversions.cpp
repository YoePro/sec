// Exact detached integer/register conversions use the single MD-114 registry.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2 (MD-110 §8).
#include "RegisterContracts.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Interfaces/DataLayoutInterfaces.h"
#include "llvm/ADT/APInt.h"
using namespace mlir;
using namespace sec;
namespace {
// Semantic signedness remains explicit at this boundary, including P8 bridges.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2, MD-110 §8(2–5).
std::optional<unsigned> integerWidth(Type type,Operation *anchor) {
 if (auto integer=dyn_cast<IntegerType>(type)) {
  if ((!integer.isSigned() && !integer.isUnsigned()) || integer.getWidth()==1) return std::nullopt;
  switch(integer.getWidth()){case 8:case 16:case 32:case 64:case 128:case 256:return integer.getWidth();default:return std::nullopt;}
 }
 if (isa<IntType,UIntType>(type)) {
  auto bits=DataLayout::closest(anchor).getTypeSizeInBits(IndexType::get(anchor->getContext()));
  if (!bits.isScalable()) return bits.getFixedValue();
 }
 return std::nullopt;
}
// Constants are read from represented SSA, with the source's retained signed
// interpretation; neither nominal layout nor byte order changes numeric fit.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2, MD-110 §8(4).
std::optional<llvm::APInt> constantBits(Value value) {
 bool sign=isa<IntType>(value.getType()) || value.getType().isSignedInteger();
 for (unsigned depth=0;depth<64;++depth) {
  auto cast=value.getDefiningOp<UnrealizedConversionCastOp>();if (!cast || cast->getNumOperands()!=1) break;
  value=cast->getOperand(0);
 }
 if (auto constant=value.getDefiningOp<ConstIntOp>()) {
  auto text=constant.getValue();unsigned precision=std::max<unsigned>(257,text.size()*4+2);
  return llvm::APInt(precision,text,10);
 }
 if (auto constant=value.getDefiningOp<arith::ConstantOp>()) if (auto integer=dyn_cast<IntegerAttr>(constant.getValue())) {
  return sign?integer.getValue().sextOrTrunc(257):integer.getValue().zextOrTrunc(257);
 }
 return std::nullopt;
}
}
// Check admitted constant/source-domain fits using the canonical register width.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2, MD-110 §8(3–4).
LogicalResult RegisterFromBitsOp::verify() {
 auto definition=sec::detail::lookupRegister(*this,getResult().getType());
 auto width=integerWidth(getValue().getType(),*this);
 if (!definition || !width) return emitOpError("register.from_bits requires a primitive integer and defined register result");
 auto proof=(*this)->getAttrOfType<DictionaryAttr>("sec.register_fit_proof");
 auto basis=proof?proof.getAs<StringAttr>("basis"):StringAttr();
 if (!proof || proof.size()!=1 || !basis) return emitOpError("register.from_bits requires exactly the register_fit_proof basis");
 if (basis.getValue()=="source-domain") {
  if (*width>definition.getWidth()) return emitOpError("register source-domain proof exceeds register width");
  return success();
 }
 if (basis.getValue()=="constant") {
  auto value=constantBits(getValue());
  if (!value || (value->isNegative()?value->getSignificantBits():value->getActiveBits())>definition.getWidth()) return emitOpError("register constant bit-fit proof is not established by SSA");
  return success();
 }
 if (basis.getValue()=="guarded") return emitOpError("unsupported MD-110 fits_bits destination-identity transport");
 return emitOpError("unknown register bit-fit proof basis");
}
// Preserve nominal source identity and signed/unsigned result interpretation.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2, MD-110 §8(5).
LogicalResult RegisterToBitsOp::verify() {
 auto definition=sec::detail::lookupRegister(*this,getValue().getType());
 auto width=integerWidth(getResult().getType(),*this);
 if (!definition || !width || *width<definition.getWidth()) return emitOpError("register.to_bits requires a defined register and integer result at least its width");
 if ((*this)->hasAttr("sec.register_fit_proof")) return emitOpError("register.to_bits carries no bit-fit proof");
 return success();
}
