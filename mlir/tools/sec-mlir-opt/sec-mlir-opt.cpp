#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlow.h"
#include "mlir/Dialect/DLTI/DLTI.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Dialect/MemRef/IR/MemRef.h"
#include "mlir/InitAllPasses.h"
#include "mlir/Tools/mlir-opt/MlirOptMain.h"

#include "sec/Dialect/Sec/SecDialect.h"
#include "sec/Analysis/Passes.h"
#include "sec/Conversion/SecIntegerToArith/Passes.h"
#include "sec/Conversion/SecToCore/Passes.h"

int main(int argc, char **argv) {
  mlir::registerAllPasses();
  sec::registerAnalysisPasses();
  sec::registerSecIntegerToArithPasses();
  sec::registerSecIntegerToArithPipelines();
  sec::registerSecToCorePasses();
  sec::registerSecToCorePipelines();

  mlir::DialectRegistry registry;
  registry.insert<sec::SecDialect, mlir::arith::ArithDialect,
                  mlir::func::FuncDialect, mlir::cf::ControlFlowDialect,
                  mlir::DLTIDialect, mlir::memref::MemRefDialect>();

  return mlir::asMainReturnCode(
      mlir::MlirOptMain(argc, argv, "Sec MLIR optimizer driver\n", registry));
}
