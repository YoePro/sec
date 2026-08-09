set(LLVM_TARGET_DEFINITIONS SecDialect.td)
mlir_tablegen(SecDialect.h.inc -gen-dialect-decls)
mlir_tablegen(SecDialect.cpp.inc -gen-dialect-defs)
add_public_tablegen_target(SecDialectIncGen)

set(LLVM_TARGET_DEFINITIONS SecTypes.td)
mlir_tablegen(SecTypes.h.inc -gen-typedef-decls)
mlir_tablegen(SecTypes.cpp.inc -gen-typedef-defs)
add_public_tablegen_target(SecTypesIncGen)

set(LLVM_TARGET_DEFINITIONS SecOps.td)
mlir_tablegen(SecOps.h.inc -gen-op-decls)
mlir_tablegen(SecOps.cpp.inc -gen-op-defs)
add_public_tablegen_target(SecOpsIncGen)

add_mlir_doc(SecDialect SecDialect Sec/ -gen-dialect-doc)
add_mlir_doc(SecTypes SecTypes Sec/ -gen-typedef-doc)
add_mlir_doc(SecOps SecOps Sec/ -gen-op-doc)
