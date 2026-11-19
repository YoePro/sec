add_mlir_library(SecMLIRAnalysis
  AnnotateSuccessorActions.cpp
  VerifyFloatSemantics.cpp
  VerifyNumericConversions.cpp
  VerifyCheckedIntegerGuards.cpp
  VerifyCleanups.cpp
  VerifyDestructionPlans.cpp
  VerifyDynamicArrays.cpp
  VerifyStorageTransitions.cpp
  VerifyOwnership.cpp
  VerifyBorrowDestruction.cpp
  VerifyPlaceReferences.cpp
  VerifyReturnOrigins.cpp
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
