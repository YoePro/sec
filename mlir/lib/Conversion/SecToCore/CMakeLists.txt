add_mlir_conversion_library(SecMLIRSecToCore
  SecToCore.cpp

  ADDITIONAL_HEADER_DIRS
  ${SEC_MLIR_SOURCE_DIR}/include/sec/Conversion/SecToCore

  DEPENDS
  SecMLIRSecToCorePassIncGen

  LINK_LIBS PUBLIC
  MLIRArithDialect
  MLIRFuncDialect
  MLIRFuncTransforms
  MLIRIR
  MLIRDataLayoutInterfaces
  MLIRMemRefDialect
  MLIRPass
  MLIRTransforms
  SecMLIRDialect
)
