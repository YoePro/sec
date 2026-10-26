add_mlir_library(SecMLIRAnalysis
  VerifyFloatSemantics.cpp
  VerifyCheckedIntegerGuards.cpp
  VerifyPlaceReferences.cpp
  VerifyReferenceTransfers.cpp
  VerifySliceGuards.cpp
  VerifySliceReferences.cpp

  ADDITIONAL_HEADER_DIRS
  ${SEC_MLIR_SOURCE_DIR}/include/sec/Analysis

  DEPENDS
  SecMLIRAnalysisPassIncGen

  LINK_LIBS PUBLIC
  MLIRArithDialect
  MLIRControlFlowDialect
  MLIRFuncDialect
  MLIRIR
  MLIRPass
  SecMLIRDialect
)
