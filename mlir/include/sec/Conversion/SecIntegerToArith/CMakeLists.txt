set(LLVM_TARGET_DEFINITIONS Passes.td)
mlir_tablegen(Passes.h.inc -gen-pass-decls -name SecIntegerToArith)
add_public_tablegen_target(SecMLIRSecIntegerToArithPassIncGen)

add_mlir_doc(Passes SecIntegerToArithPasses ./ -gen-pass-doc)
