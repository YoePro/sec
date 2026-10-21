#ifndef SEC_CONVERSION_SECTOCORE_SCALARCONTAINERS_H
#define SEC_CONVERSION_SECTOCORE_SCALARCONTAINERS_H

namespace mlir {
class TypeConverter;
class MLIRContext;
} // namespace mlir

namespace sec {
// Install P6 scalar resolution through semantic containers without selecting
// their physical representation. P16 section 112 preserves slice authority.
void addScalarContainerConversions(mlir::TypeConverter &converter,
                                   mlir::MLIRContext *context);
} // namespace sec

#endif
