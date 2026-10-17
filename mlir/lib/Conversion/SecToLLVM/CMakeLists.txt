add_mlir_conversion_library(SecMLIRSecToLLVM
  SecToLLVM.cpp
  IntegerEnums.cpp

  ADDITIONAL_HEADER_DIRS
  ${SEC_MLIR_SOURCE_DIR}/include/sec/Conversion/SecToLLVM

  DEPENDS
  SecMLIRSecToLLVMPassIncGen

  LINK_LIBS PUBLIC
  MLIRArithDialect
  MLIRArithToLLVM
  MLIRControlFlowToLLVM
  MLIRControlFlowDialect
  MLIRFuncDialect
  MLIRFuncToLLVM
  MLIRFuncTransforms
  MLIRIR
  MLIRLLVMDialect
  MLIRMemRefToLLVM
  MLIRPass
  MLIRReconcileUnrealizedCasts
  MLIRTransforms
  SecMLIRDialect
  SecMLIRSecIntegerToArith
  SecMLIRSecToCore
)
