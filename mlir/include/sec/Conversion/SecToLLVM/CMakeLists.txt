set(LLVM_TARGET_DEFINITIONS Passes.td)
mlir_tablegen(Passes.h.inc -gen-pass-decls -name SecToLLVM)
add_public_tablegen_target(SecMLIRSecToLLVMPassIncGen)

add_mlir_doc(Passes SecToLLVMPasses ./ -gen-pass-doc)
