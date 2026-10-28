// Structural range-proof preservation for pure detached field updates.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §7).
#include "RegisterContracts.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/IR/Dominance.h"
#include "mlir/Interfaces/DataLayoutInterfaces.h"
#include "llvm/ADT/APInt.h"
#include <optional>
using namespace mlir;
using namespace sec;
namespace {
using Range=std::pair<llvm::APInt,llvm::APInt>;
// Exact conversions preserve mathematical values; no source name participates.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §7(4–8).
Value original(Value value) {
 for (unsigned depth=0;depth<64;++depth) {
  Operation *op=value.getDefiningOp();
  if (!op || !isa<NumericConvertExactOp,UnrealizedConversionCastOp,arith::ExtUIOp>(op) || op->getNumOperands()!=1) break;
  value=op->getOperand(0);
 }
 return value;
}
// Read actual integer/bool constant evidence through exact bridges.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §7(4).
std::optional<llvm::APInt> constant(Value value,unsigned precision) {
 value=original(value);
 if (auto c=value.getDefiningOp<ConstIntOp>()) {
  StringRef text=c.getValue(); if (text.empty() || text.front()=='-' || text.size()*4+1>precision) return std::nullopt;
  llvm::APInt result; if (text.getAsInteger(10,result)) return std::nullopt;
  return result.zextOrTrunc(precision);
 }
 if (auto c=value.getDefiningOp<ConstBoolOp>()) return llvm::APInt(precision,c.getValue()?1:0);
 if (auto c=value.getDefiningOp<arith::ConstantOp>()) if (auto attr=dyn_cast<IntegerAttr>(c.getValue())) {
  if (attr.getValue().getActiveBits()>precision || (attr.getType().isSignedInteger() && attr.getValue().isNegative())) return std::nullopt;
  return attr.getValue().zextOrTrunc(precision);
 }
 return std::nullopt;
}
// Evidence is confined to existing types, constants, field extraction and
// explicit masks. Unknown operations/CFG sources grant no fabricated range.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §7(5–8).
std::optional<Range> rangeOf(Value value,Operation *use,unsigned precision,unsigned depth=0) {
 if (depth>64) return std::nullopt;
 bool unsignedBoundary=false;
 if (auto semantic=dyn_cast<IntegerType>(value.getType())) unsignedBoundary=semantic.isUnsigned();
 value=original(value);
 if (auto number=constant(value,precision)) return Range{*number,*number};
 if (auto extract=value.getDefiningOp<RegisterExtractOp>()) {
  auto field=sec::detail::lookupRegisterField(extract);
  if (failed(field) || field->getWidth()>=precision) return std::nullopt;
  return Range{llvm::APInt(precision,0),llvm::APInt::getLowBitsSet(precision,field->getWidth())};
 }
 if (auto mask=value.getDefiningOp<IntBitwiseOp>()) if (mask.getKind()=="and") {
  auto c=constant(mask.getRight(),precision);if (!c) c=constant(mask.getLeft(),precision);
  if (c) return Range{llvm::APInt(precision,0),*c};
 }
 if (auto integer=dyn_cast<IntegerType>(value.getType())) {
  if (integer.isUnsigned() || integer.getWidth()==1 || (unsignedBoundary && integer.isSignless())) {
   if (integer.getWidth()>=precision) return std::nullopt;
   return Range{llvm::APInt(precision,0),llvm::APInt::getLowBitsSet(precision,integer.getWidth())};
  }
 }
 if (isa<UIntType>(value.getType())) {
  auto bits=DataLayout::closest(use).getTypeSizeInBits(IndexType::get(use->getContext()));
  if (bits.isScalable() || bits.getFixedValue()>=precision) return std::nullopt;
  return Range{llvm::APInt(precision,0),llvm::APInt::getLowBitsSet(precision,bits.getFixedValue())};
 }
 return std::nullopt;
}
// Only an actual successful edge dominating the update can refine the same
// immutable SSA value. Mutable loads or unknown phi relations are not guessed.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §7(5).
void refine(Range &range,Value value,Operation *use,unsigned precision) {
 auto function=use->getParentOfType<func::FuncOp>();if (!function) return;
 DominanceInfo dominance(function);
 value=original(value);
 function.walk([&](cf::CondBranchOp branch) {
  bool yes=dominance.dominates(branch.getTrueDest(),use->getBlock());
  bool no=dominance.dominates(branch.getFalseDest(),use->getBlock());
  if (yes==no || !dominance.dominates(branch.getOperation(),use)) return;
  Value left,right;std::string predicate;
  if (auto compare=branch.getCondition().getDefiningOp<IntCmpOp>()) {
    left=compare.getLeft();right=compare.getRight();predicate=compare.getPredicate().str();
  } else if (auto compare=branch.getCondition().getDefiningOp<arith::CmpIOp>()) {
    left=compare.getLhs();right=compare.getRhs();
    switch(compare.getPredicate()) {
    case arith::CmpIPredicate::eq:predicate="eq";break;
    case arith::CmpIPredicate::ne:predicate="ne";break;
    case arith::CmpIPredicate::ult:predicate="lt";break;
    case arith::CmpIPredicate::ule:predicate="le";break;
    case arith::CmpIPredicate::ugt:predicate="gt";break;
    case arith::CmpIPredicate::uge:predicate="ge";break;
    default:return; // No signed interpretation is inferred from signless bits.
    }
  } else return;
  Value variable=original(left);auto c=constant(right,precision);
  if (variable!=value) {
   variable=original(right);c=constant(left,precision);
   if (predicate=="lt") predicate="gt";else if(predicate=="le")predicate="ge";else if(predicate=="gt")predicate="lt";else if(predicate=="ge")predicate="le";
  }
  if (variable!=value || !c) return;
  if (no) {
   if(predicate=="lt")predicate="ge";else if(predicate=="le")predicate="gt";else if(predicate=="gt")predicate="le";else if(predicate=="ge")predicate="lt";else if(predicate=="eq")predicate="ne";else predicate="eq";
  }
  if(predicate=="eq") {range.first=llvm::APIntOps::umax(range.first,*c);range.second=llvm::APIntOps::umin(range.second,*c);}
  if(predicate=="lt" && !c->isZero()) range.second=llvm::APIntOps::umin(range.second,*c-1);
  if(predicate=="le") range.second=llvm::APIntOps::umin(range.second,*c);
  if(predicate=="gt") range.first=llvm::APIntOps::umax(range.first,*c+1);
  if(predicate=="ge") range.first=llvm::APIntOps::umax(range.first,*c);
 });
}
}
// Match transported bounds to the selected leaf and actual replacement SSA.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §7.
LogicalResult sec::detail::verifyRegisterWrite(Operation *operation,RegisterFieldAttr field) {
 auto proof=operation->getAttrOfType<RegisterWriteProofAttr>("sec.write_proof");
 unsigned precision=std::max<unsigned>(257,std::max(proof.getLow().getValue().size(),proof.getHigh().getValue().size())*4+2);
 llvm::APInt low(precision,proof.getLow().getValue(),10),high(precision,proof.getHigh().getValue(),10);
 if (high.getActiveBits()>field.getWidth()) return operation->emitOpError("register write proof exceeds selected field width");
 Value replacement=operation->getOperand(1);
 auto range=rangeOf(replacement,operation,precision);
 if (range && proof.getBasis().getValue()=="interval") refine(*range,replacement,operation,precision);
 if (!range || range->first.ugt(range->second) || range->first.ult(low) || range->second.ugt(high)) return operation->emitOpError("register write proof is not established by replacement SSA evidence");
 if (proof.getBasis().getValue()=="constant" && (range->first!=range->second || range->first!=low)) return operation->emitOpError("register constant proof contradicts replacement SSA");
 return success();
}
