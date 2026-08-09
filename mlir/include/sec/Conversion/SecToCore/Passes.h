#ifndef SEC_CONVERSION_SECTOCORE_PASSES_H
#define SEC_CONVERSION_SECTOCORE_PASSES_H

#include "mlir/Pass/Pass.h"

namespace sec {

#define GEN_PASS_DECL
#include "sec/Conversion/SecToCore/Passes.h.inc"

std::unique_ptr<mlir::Pass> createSecLowerTrivialCorePass();
std::unique_ptr<mlir::Pass> createSecResolveScalarLayoutPass();
void registerSecToCorePipelines();

#define GEN_PASS_REGISTRATION
#include "sec/Conversion/SecToCore/Passes.h.inc"

} // namespace sec

#endif // SEC_CONVERSION_SECTOCORE_PASSES_H
