// Pure register contracts; physical hardware authority is intentionally absent.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a (MD-114 §§2–9).
#include "RegisterContracts.h"
#include "sec/Dialect/Sec/SecOps.h"
#include "sec/Dialect/Sec/SecTypes.h"
#include "sec/Dialect/Sec/TypeTraversal.h"
#include "llvm/ADT/APInt.h"
#include "llvm/ADT/StringMap.h"
#include "llvm/ADT/StringSet.h"
#include "llvm/ADT/StringSwitch.h"
#include "llvm/ADT/STLExtras.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include <functional>
using namespace mlir;
using namespace sec;

namespace {
// Canonical arbitrary-precision natural bounds, never host-width conversion.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a; MD-114 §7(6).
bool natural(StringAttr text) {
  if (!text) return false;
  StringRef s=text.getValue();
  return !s.empty() && (s.size()==1 || s.front()!='0') &&
    llvm::all_of(s,[](char c){return c>='0' && c<='9';});
}
// Compare canonical natural decimal values without host-width narrowing.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §7(6).
bool less(StringRef a,StringRef b) {
  return a.size()!=b.size()?a.size()<b.size():a<b;
}
// Accept only resolved register access policies, without applying hardware effects.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §3(6).
bool accessValid(StringRef value) {
  return llvm::StringSwitch<bool>(value)
    .Cases({"read-write","read-only","write-only","read-clear","read-set"},true)
    .Cases({"write-one-clear","write-one-set","write-one-toggle"},true)
    .Cases({"write-zero-clear","write-zero-set","write-zero-toggle"},true).Default(false);
}
// Select the exact standard unsigned carrier for a multi-bit field.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §3(5).
unsigned carrierWidth(unsigned width) {
  for (unsigned n:{8,16,32,64,128,256}) if (width<=n) return n;
  return 0;
}
}

// Validate required field metadata and reserved/name agreement.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §3.
LogicalResult RegisterFieldAttr::verify(function_ref<InFlightDiagnostic()> error,
 uint32_t ordinal,StringAttr name,uint32_t offset,uint32_t width,Type type,
 StringAttr access,bool shadow,StringAttr unit,bool reserved) {
 if (!name || name.getValue().empty() || !type || !width || !access || !accessValid(access.getValue())) return error()<<"invalid register field name/type/width/access";
 if ((name.getValue()=="_")!=reserved) return error()<<"register reserved flag must agree with name '_'";
 if (unit && unit.getValue().empty()) return error()<<"register unit identity must be nonempty when present";
 return success();
}
// Validate declaration-order identity and exact logical allocation coverage.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §2–3.
LogicalResult RegisterDefAttr::verify(function_ref<InFlightDiagnostic()> error,
 StringAttr identity,uint32_t width,StringAttr order,StringAttr byteOrder,ArrayAttr fields) {
 if (!identity || identity.getValue().empty() || !width || !order || (order.getValue()!="lsb-first" && order.getValue()!="msb-first") || !fields) return error()<<"invalid register identity/width/order/fields";
 if (byteOrder && byteOrder.getValue()!="little-endian" && byteOrder.getValue()!="big-endian") return error()<<"invalid register byte_order";
 uint64_t allocated=0;llvm::StringSet<> names;
 for (auto [index,attribute]:llvm::enumerate(fields)) {
  auto field=dyn_cast<RegisterFieldAttr>(attribute);
  if (!field || field.getOrdinal()!=index || allocated+field.getWidth()>width) return error()<<"register field ordinals/layout must be contiguous and fit the register";
  uint64_t offset=order.getValue()=="lsb-first"?allocated:width-allocated-field.getWidth();
  if (field.getOffset()!=offset) return error()<<"register field offset disagrees with declaration allocation order";
  if (!field.getReserved() && !names.insert(field.getName().getValue()).second) return error()<<"duplicate register field name";
  allocated+=field.getWidth();
 }
 if (allocated!=width) return error()<<"register fields must cover the complete width including reserved bits";
 return success();
}
// Validate exact/interval proof shape and canonical arbitrary-precision bounds.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §7.
LogicalResult RegisterWriteProofAttr::verify(function_ref<InFlightDiagnostic()> error,
 StringAttr basis,StringAttr low,StringAttr high) {
 if (!basis) return error()<<"register write proof requires basis";
 if (basis.getValue()=="typed") {
  if (low || high) return error()<<"typed register write proof cannot carry bounds";
 } else if (basis.getValue()=="constant" || basis.getValue()=="interval") {
  if (!natural(low) || !natural(high) || less(high.getValue(),low.getValue())) return error()<<"register write proof requires canonical ordered nonnegative bounds";
  if (basis.getValue()=="constant" && low!=high) return error()<<"constant register write proof bounds must be equal";
 } else return error()<<"unknown register write proof basis";
 return success();
}

// Resolve nominal identity in the one owning module registry.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §2.
RegisterDefAttr sec::detail::lookupRegister(Operation *anchor,Type type) {
 auto reg=dyn_cast<RegisterType>(type);if (!reg) return {};
 auto module=anchor->getParentOfType<ModuleOp>();
 auto table=module?module->getAttrOfType<ArrayAttr>("sec.registers"):ArrayAttr();
 if (table) for (auto attribute:table) if (auto def=dyn_cast<RegisterDefAttr>(attribute))
  if (def.getIdentity()==reg.getIdentity()) return def;
 return {};
}

// Validate the complete table before nested references, including cycles and
// all register identities in module attributes/signatures/values/regions.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a; MD-114 §§2–3,9(2).
LogicalResult sec::detail::verifyRegisterModule(ModuleOp module) {
 auto raw=module->getAttr("sec.registers");auto table=dyn_cast_or_null<ArrayAttr>(raw);
 if (raw && !table) return module.emitOpError("sec.registers must be an array of #sec.register_def");
 llvm::StringMap<RegisterDefAttr> definitions;
 if (table) for (Attribute attribute:table) {
  auto def=dyn_cast<RegisterDefAttr>(attribute);
  if (!def || !definitions.try_emplace(def.getIdentity().getValue(),def).second) return module.emitOpError("sec.registers requires unique typed register definitions");
 }
 llvm::StringMap<unsigned> state;
 std::function<LogicalResult(RegisterDefAttr)> visit=[&](RegisterDefAttr def)->LogicalResult {
  auto identity=def.getIdentity().getValue();
  if (state[identity]==1) return module.emitOpError("recursive register layout");
  if (state[identity]==2) return success();
  state[identity]=1;
  for (auto attribute:def.getFields()) {
   auto field=cast<RegisterFieldAttr>(attribute);Type type=field.getType();unsigned width=field.getWidth();
   if (auto nested=dyn_cast<RegisterType>(type)) {
    auto child=definitions.lookup(nested.getIdentity().getValue());
    if (!child || child.getWidth()!=width) return module.emitOpError("nested register field needs a matching definition and width");
    if (failed(visit(child))) return failure();
   } else if (auto enumeration=dyn_cast<EnumType>(type)) {
    if (enumeration.getRepresentation().getValue()!="bit-backed" || enumeration.getBitWidth()!=width) return module.emitOpError("register enum field requires exact bit-backed width");
   } else if (auto integer=dyn_cast<IntegerType>(type)) {
    bool valid=width==1?integer.isSignless() && integer.getWidth()==1:integer.isUnsigned() && integer.getWidth()==carrierWidth(width) && carrierWidth(width)!=0;
    if (!valid) return module.emitOpError("register field requires its smallest unsigned carrier (i1 for bit)");
   } else return module.emitOpError("unsupported register field carrier");
   // Quantity types are not yet represented by this schema; never erase a unit.
   if (field.getUnit()) return module.emitOpError("unsupported register unit-qualified field representation");
  }
  state[identity]=2;return success();
 };
 if (table) for (auto attribute:table) if (failed(visit(cast<RegisterDefAttr>(attribute)))) return failure();
 bool missing=false;
 module.walk([&](Operation *operation){
  if (operation!=module.getOperation() && (isa<ModuleOp>(operation) || operation->getParentOfType<ModuleOp>()!=module)) return;
  missing |= bool(findOperationType(operation,[&](Type type){
   if (auto reg=dyn_cast<RegisterType>(type)) return !definitions.count(reg.getIdentity().getValue());
   return false;
  }));
 });
 if (missing) return module.emitOpError("register type has no definition in sec.registers");
 return success();
}

// Resolve every ordinal and compose the logical slice through nested registers.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §5.
FailureOr<RegisterFieldAttr> sec::detail::lookupRegisterField(Operation *operation) {
 auto definition=lookupRegister(operation,operation->getOperand(0).getType());
 auto path=operation->getAttrOfType<ArrayAttr>("sec.field_path");
 if (!definition || !path || path.empty()) return operation->emitOpError("requires a defined register and nonempty sec.field_path");
 RegisterFieldAttr field;
 uint64_t logicalOffset=0;
 uint64_t rootWidth=definition.getWidth();
 for (auto [index,attribute]:llvm::enumerate(path)) {
  auto ordinal=dyn_cast<IntegerAttr>(attribute);
  if (!ordinal || !ordinal.getType().isSignlessInteger(32) || ordinal.getValue().isNegative() || ordinal.getValue().getZExtValue()>=definition.getFields().size()) return operation->emitOpError("invalid register field ordinal");
  field=cast<RegisterFieldAttr>(definition.getFields()[ordinal.getValue().getZExtValue()]);
  if (field.getReserved()) return operation->emitOpError("cannot access reserved register field");
  logicalOffset+=field.getOffset();
  if (logicalOffset+field.getWidth()>rootWidth) return operation->emitOpError("composed register field path exceeds its root slice");
  if (index+1<path.size()) {
   definition=lookupRegister(operation,field.getType());
   if (!definition) return operation->emitOpError("register field path descends into a scalar");
  }
 }
 return field;
}

// Verify a pure field extraction of its exact declared carrier.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §6(2).
LogicalResult RegisterExtractOp::verify() {
 auto field=detail::lookupRegisterField(*this);if (failed(field)) return failure();
 if (getResult().getType()!=field->getType() || (*this)->hasAttr("sec.write_proof")) return emitOpError("register extract result/proof mismatch");
 return success();
}
// Verify nominal replacement/result identity and field-write evidence.
// Rules: rules/mlir/sec_mlir_dialect.md — v11.41.1a, MD-114 §6(3),7.
LogicalResult RegisterUpdateOp::verify() {
 auto field=detail::lookupRegisterField(*this);if (failed(field)) return failure();
 if (getResult().getType()!=getSource().getType() || getReplacement().getType()!=field->getType()) return emitOpError("register update must retain nominal type and exact field carrier");
 auto proof=(*this)->getAttrOfType<RegisterWriteProofAttr>("sec.write_proof");
 if (!proof) return emitOpError("register update requires #sec.register_write_proof");
 if (proof.getBasis().getValue()=="typed") {
  if (isa<EnumType,RegisterType>(field->getType()) || field->getType().isSignlessInteger(1)) return success();
  return emitOpError("typed proof cannot justify a plain multi-bit unsigned carrier");
 }
 return detail::verifyRegisterWrite(*this,*field);
}
