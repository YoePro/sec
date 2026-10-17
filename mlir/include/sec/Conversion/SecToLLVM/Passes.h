#ifndef SEC_CONVERSION_SECTOLLVM_PASSES_H
#define SEC_CONVERSION_SECTOLLVM_PASSES_H

#include "mlir/Pass/Pass.h"

#include <string>

namespace sec {

#define GEN_PASS_DECL
#include "sec/Conversion/SecToLLVM/Passes.h.inc"

void registerSecToLLVMPipelines();

#define GEN_PASS_REGISTRATION
#include "sec/Conversion/SecToLLVM/Passes.h.inc"

} // namespace sec

#endif // SEC_CONVERSION_SECTOLLVM_PASSES_H
