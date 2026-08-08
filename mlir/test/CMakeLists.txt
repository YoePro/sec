configure_lit_site_cfg(
  ${CMAKE_CURRENT_SOURCE_DIR}/lit.site.cfg.py.in
  ${CMAKE_CURRENT_BINARY_DIR}/lit.site.cfg.py
  MAIN_CONFIG
  ${CMAKE_CURRENT_SOURCE_DIR}/lit.cfg.py
)

set(SEC_MLIR_TEST_DEPENDS
  FileCheck
  count
  not
  sec-mlir-opt
)

add_lit_testsuite(check-sec-mlir "Running the Sec MLIR regression tests"
  ${CMAKE_CURRENT_BINARY_DIR}
  DEPENDS ${SEC_MLIR_TEST_DEPENDS}
)
set_target_properties(check-sec-mlir PROPERTIES FOLDER "Tests")

add_lit_testsuites(SEC_MLIR ${CMAKE_CURRENT_SOURCE_DIR}
  DEPENDS ${SEC_MLIR_TEST_DEPENDS})
