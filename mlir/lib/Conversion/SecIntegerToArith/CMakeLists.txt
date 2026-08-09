add_mlir_conversion_library(SecMLIRSecIntegerToArith
  SecIntegerToArith.cpp

  ADDITIONAL_HEADER_DIRS
  ${SEC_MLIR_SOURCE_DIR}/include/sec/Conversion/SecIntegerToArith

  DEPENDS
  SecMLIRSecIntegerToArithPassIncGen

  LINK_LIBS PUBLIC
  MLIRArithDialect
  MLIRControlFlowDialect
  MLIRFuncDialect
  MLIRFuncTransforms
  MLIRIR
  MLIRMemRefDialect
  MLIRPass
  MLIRTransforms
  SecMLIRAnalysis
  SecMLIRDialect
  SecMLIRSecToCore
)
