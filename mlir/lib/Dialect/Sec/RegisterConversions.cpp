// Exact detached integer/register conversions use the single MD-114 registry.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2 (MD-110 §8).
#include "RegisterContracts.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/Dominance.h"
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
// A guarded conversion is dominated by the true edge of a fits_bits test of
// the same SSA operand and destination register type.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2 amendment (guarded basis).
bool guardedByFitsBits(RegisterFromBitsOp conversion) {
 auto function=conversion->getParentOfType<func::FuncOp>();
 if (!function) return false;
 DominanceInfo dominance(function);
 for (Operation *user : conversion.getValue().getUsers()) {
  auto test=dyn_cast<RegisterFitsBitsOp>(user);
  if (!test || test.getRegisterType()!=conversion.getResult().getType()) continue;
  for (Operation *testUser : test.getResult().getUsers()) {
   auto branch=dyn_cast<cf::CondBranchOp>(testUser);
   if (!branch || branch.getCondition()!=test.getResult() || branch.getTrueDest()==branch.getFalseDest()) continue;
   if (branch.getTrueDest()->getSinglePredecessor()==branch->getBlock() && dominance.dominates(branch.getTrueDest(),conversion->getBlock())) return true;
  }
 }
 return false;
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
 if (basis.getValue()=="guarded") {
  if (!guardedByFitsBits(*this)) return emitOpError("register guarded bit-fit proof requires a dominating successful register.fits_bits of the same operand and register type");
  return success();
 }
 if (basis.getValue()=="bit-extraction") {
  auto extract=getValue().getDefiningOp<IntBitExtractOp>();
  if (!extract || static_cast<int64_t>(extract.getWidth())>static_cast<int64_t>(definition.getWidth())) return emitOpError("register bit-extraction proof requires the direct result of an int.bit_extract no wider than the register");
  return success();
 }
 return emitOpError("unknown register bit-fit proof basis");
}
// Name the destination by TypeAttr and resolve its width in the register table.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2 amendment (2026-10-30).
LogicalResult RegisterFitsBitsOp::verify() {
 if (!isa<RegisterType>(getRegisterType()) || !sec::detail::lookupRegister(*this,getRegisterType())) return emitOpError("register.fits_bits requires register_type naming a defined register");
 if (!integerWidth(getValue().getType(),*this)) return emitOpError("register.fits_bits requires a resolved primitive integer operand");
 return success();
}
// x mod 2^width into the smallest standard unsigned carrier; total and pure.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.2 amendment; semantic_ir.md § 59.
LogicalResult IntBitExtractOp::verify() {
 auto sourceWidth=integerWidth(getValue().getType(),*this);
 auto result=dyn_cast<IntegerType>(getResult().getType());
 if (!sourceWidth) return emitOpError("int.bit_extract requires a standard-width signed or unsigned integer operand");
 int64_t width=getWidth();
 if (width<1 || width>static_cast<int64_t>(*sourceWidth)) return emitOpError("int.bit_extract width must be between 1 and the operand width");
 unsigned carrier=8;
 while (carrier<width) carrier*=2;
 if (!result || !result.isUnsigned() || result.getWidth()!=carrier) return emitOpError("int.bit_extract result must be the smallest standard unsigned integer holding width bits");
 return success();
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
