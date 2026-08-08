set(LLVM_TARGET_DEFINITIONS SecDialect.td)
mlir_tablegen(SecDialect.h.inc -gen-dialect-decls)
mlir_tablegen(SecDialect.cpp.inc -gen-dialect-defs)
add_public_tablegen_target(SecDialectIncGen)

set(LLVM_TARGET_DEFINITIONS SecTypes.td)
mlir_tablegen(SecTypes.h.inc -gen-typedef-decls)
mlir_tablegen(SecTypes.cpp.inc -gen-typedef-defs)
add_public_tablegen_target(SecTypesIncGen)

add_mlir_doc(SecDialect SecDialect Sec/ -gen-dialect-doc)
add_mlir_doc(SecTypes SecTypes Sec/ -gen-typedef-doc)
