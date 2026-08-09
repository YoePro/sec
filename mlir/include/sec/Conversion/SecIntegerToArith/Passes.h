#ifndef SEC_CONVERSION_SECINTEGERTOARITH_PASSES_H
#define SEC_CONVERSION_SECINTEGERTOARITH_PASSES_H

#include "mlir/Pass/Pass.h"

namespace sec {

#define GEN_PASS_DECL
#include "sec/Conversion/SecIntegerToArith/Passes.h.inc"

std::unique_ptr<mlir::Pass> createSecLowerCheckedIntegersPass();
void registerSecIntegerToArithPipelines();

#define GEN_PASS_REGISTRATION
#include "sec/Conversion/SecIntegerToArith/Passes.h.inc"

} // namespace sec

#endif // SEC_CONVERSION_SECINTEGERTOARITH_PASSES_H
