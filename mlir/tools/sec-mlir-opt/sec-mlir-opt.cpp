#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/InitAllPasses.h"
#include "mlir/Tools/mlir-opt/MlirOptMain.h"

#include "sec/Dialect/Sec/SecDialect.h"

int main(int argc, char **argv) {
  mlir::registerAllPasses();

  mlir::DialectRegistry registry;
  registry.insert<sec::SecDialect, mlir::func::FuncDialect>();

  return mlir::asMainReturnCode(
      mlir::MlirOptMain(argc, argv, "Sec MLIR optimizer driver\n", registry));
}
