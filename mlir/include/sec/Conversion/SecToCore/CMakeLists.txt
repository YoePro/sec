set(LLVM_TARGET_DEFINITIONS Passes.td)
mlir_tablegen(Passes.h.inc -gen-pass-decls -name SecToCore)
add_public_tablegen_target(SecMLIRSecToCorePassIncGen)

add_mlir_doc(Passes SecToCorePasses ./ -gen-pass-doc)
