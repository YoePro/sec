add_mlir_library(SecMLIRAnalysis
  VerifyCheckedIntegerGuards.cpp

  ADDITIONAL_HEADER_DIRS
  ${SEC_MLIR_SOURCE_DIR}/include/sec/Analysis

  DEPENDS
  SecMLIRAnalysisPassIncGen

  LINK_LIBS PUBLIC
  MLIRControlFlowDialect
  MLIRFuncDialect
  MLIRIR
  MLIRPass
  SecMLIRDialect
)
