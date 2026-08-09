#ifndef SEC_DIALECT_SEC_SECOPS_H
#define SEC_DIALECT_SEC_SECOPS_H

#include "mlir/Bytecode/BytecodeOpInterface.h"
#include "mlir/IR/OpDefinition.h"
#include "mlir/IR/SymbolTable.h"
#include "mlir/Interfaces/SideEffectInterfaces.h"

#define GET_OP_CLASSES
#include "sec/Dialect/Sec/SecOps.h.inc"

#endif // SEC_DIALECT_SEC_SECOPS_H
