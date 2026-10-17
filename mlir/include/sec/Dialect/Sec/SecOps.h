#ifndef SEC_DIALECT_SEC_SECOPS_H
#define SEC_DIALECT_SEC_SECOPS_H

#include "mlir/Bytecode/BytecodeOpInterface.h"
#include "mlir/IR/OpDefinition.h"
#include "mlir/IR/SymbolTable.h"
#include "mlir/Interfaces/SideEffectInterfaces.h"

#define GET_OP_CLASSES
#include "sec/Dialect/Sec/SecOps.h.inc"

namespace sec {
/// The one authoritative P5/P15 storage query (sec_mlir_lowering_v11.md
/// section 18, sec-mlir-dialect_package15.md section 67): storage whose
/// declaration carries `sec.address_taken = true`, or that a
/// `sec.place.storage` root consumes, participates in Place/reference
/// semantics and must stay high-level until reference representation
/// lowering discharges it.
bool isReferenceDependentStorage(mlir::Value storage);

/// True when an operation has a !sec.place, !sec.ref or !sec.ref_mut operand
/// or result.
bool hasPlaceOrReferenceType(mlir::Operation *operation);
} // namespace sec

#endif // SEC_DIALECT_SEC_SECOPS_H
