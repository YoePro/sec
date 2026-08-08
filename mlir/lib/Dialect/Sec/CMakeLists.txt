add_mlir_dialect_library(SecMLIRDialect
  SecDialect.cpp
  SecTypes.cpp

  ADDITIONAL_HEADER_DIRS
  ${PROJECT_SOURCE_DIR}/include/sec/Dialect/Sec

  DEPENDS
  SecDialectIncGen
  SecTypesIncGen

  LINK_LIBS PUBLIC
  MLIRIR
)
