add_mlir_dialect_library(SecMLIRDialect
  SecAttributes.cpp
  SecDialect.cpp
  SecTypes.cpp
  SecOps.cpp

  ADDITIONAL_HEADER_DIRS
  ${PROJECT_SOURCE_DIR}/include/sec/Dialect/Sec

  DEPENDS
  SecDialectIncGen
  SecTypesIncGen
  SecAttributesIncGen
  SecOpsIncGen

  LINK_LIBS PUBLIC
  MLIRIR
  MLIRFuncDialect
)
