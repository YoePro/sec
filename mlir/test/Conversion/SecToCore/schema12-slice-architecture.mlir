// RUN: split-file %S/../../Dialect/Sec/schema12-slice-ops.mlir %t
// RUN: sec-mlir-opt %t/32.mlir --sec-resolve-scalar-layout | FileCheck %s --implicit-check-not='{{(llvm\.|memref\.|!llvm|!memref|!sec\.struct|!sec\.storage|func\.call|arith\.muli|arith\.addi|alloc|atomic|undef|poison)}}'
// RUN: sec-mlir-opt %t/64.mlir --sec-resolve-scalar-layout | FileCheck %s --implicit-check-not='{{(llvm\.|memref\.|!llvm|!memref|!sec\.struct|!sec\.storage|func\.call|arith\.muli|arith\.addi|alloc|atomic|undef|poison)}}'

// P16-52: target scalar resolution cannot choose a
// slice descriptor, expand signatures, allocate backing/runtime state, call
// helpers, count borrows, or form physical element addresses. The implicit
// negatives cover the entire output, including between CHECK regions.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md sections 7,112-113,143;
// rules/mlir/sec_mlir_dialect.md v12.3-5,27-31;
// rules/mlir/sec_mlir_lowering.md v12.27-30.
// Checked integer lowering coexists with slice and reference semantics
// (MD-087): it lowers the module's representable integer computation while
// every slice/Place/reference operation stays high-level with its facts. The
// physical gates remain --sec-check-lowering-support and --sec-lower-to-llvm.
// These handwritten contracts are not verifier evidence (below), so the
// post-P8 verifier run is covered by the source-emitted Go tests.
// RUN: sec-mlir-opt %t/32.mlir --sec-resolve-scalar-layout --sec-lower-checked-integers | FileCheck %s --check-prefix=P8
// RUN: sec-mlir-opt %t/64.mlir --sec-resolve-scalar-layout --sec-lower-checked-integers | FileCheck %s --check-prefix=P8
// P8: "sec.slice.borrow_shared"
// P8: "sec.slice.element_place"
// These are handwritten local-operation contracts, not source-emitter or
// complete guard/borrow verification evidence (P16-42..P16-45 remain separate).

// CHECK-LABEL: func.func @shared(
// CHECK-SAME: %{{[^:]+}}: !sec.place<!sec.array<si128, "4">, "ro">, %{{[^:]+}}: ui{{32|64}}, %{{[^:]+}}: si256) -> ui{{32|64}} {
// CHECK: "sec.slice.range_normalize"
// CHECK: "sec.slice.borrow_shared"
// CHECK: "sec.slice.copy_shared"
// CHECK: "sec.slice.reborrow_shared"
// CHECK: "sec.slice.len"
// CHECK: "sec.slice.index_in_bounds"
// CHECK: "sec.slice.element_place"
// CHECK-SAME: (!sec.slice<si128>, si256) -> !sec.place<si128, "ro">
// CHECK: "sec.place.read"
// CHECK: "sec.ref.end_borrow"
// CHECK-LABEL: func.func @mutable(
// CHECK-SAME: %{{[^:]+}}: !sec.place<!sec.array<si128, "4">, "rw">, %{{[^:]+}}: ui{{32|64}}, %{{[^:]+}}: ui{{32|64}}) {
// CHECK: "sec.slice.borrow_mut"
// CHECK: "sec.slice.reborrow_mut"
// CHECK: "sec.ref.end_borrow"
// CHECK: "sec.slice.move_mut"
// CHECK-LABEL: func.func @checked(
// CHECK-SAME: %{{[^:]+}}: ui{{32|64}}, %{{[^:]+}}: si128, %{{[^:]+}}: ui256) -> ui{{32|64}} {
// CHECK: "sec.slice.range_check"
// CHECK: "sec.fail.bounds"
// CHECK: return
