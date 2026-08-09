set(LLVM_TARGET_DEFINITIONS Passes.td)
mlir_tablegen(Passes.h.inc -gen-pass-decls -name Analysis)
add_public_tablegen_target(SecMLIRAnalysisPassIncGen)

add_mlir_doc(Passes SecAnalysisPasses ./ -gen-pass-doc)
