#ifndef SEC_CONVERSION_FLOATLOWERING_H
#define SEC_CONVERSION_FLOATLOWERING_H
namespace mlir { class IRRewriter; }
namespace sec {
class FloatBinaryOp;
// Lower exact truncation remainder without an ambient math runtime.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md — §§23,75–80.
void lowerFloatRemainder(FloatBinaryOp operation, mlir::IRRewriter &rewriter);
}
#endif
