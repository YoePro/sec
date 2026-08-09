set(LIBS
  MLIRArithDialect
  MLIRFuncDialect
  MLIRControlFlowDialect
  MLIRDLTIDialect
  MLIRMemRefDialect
  MLIROptLib
  MLIRRegisterAllPasses
  SecMLIRAnalysis
  SecMLIRDialect
  SecMLIRSecIntegerToArith
  SecMLIRSecToCore
)

add_llvm_executable(sec-mlir-opt sec-mlir-opt.cpp)
llvm_update_compile_flags(sec-mlir-opt)
target_link_libraries(sec-mlir-opt PRIVATE ${LIBS})
mlir_check_all_link_libraries(sec-mlir-opt)
