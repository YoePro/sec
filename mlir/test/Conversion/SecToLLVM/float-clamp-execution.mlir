// RUN: sec-mlir-opt %s --sec-verify-float-semantics --sec-lower-to-llvm=entry= | FileCheck %s
// P20 §§31,69,78,80: runtime candidate/failure and signed-zero cases.
// Also executed as a linked program by internal/mlir/float_clamp_native_test.go.
// CHECK-NOT: llvm.call @{{(fmin|fmax|floor|ceil|round|trunc)}}
// CHECK-NOT: "sec.
// CHECK: llvm.func @clamp32
// CHECK: llvm.func @clamp64
// CHECK: llvm.func @clampPlatform
// CHECK: llvm.func @main
module attributes {sec.dialect_version = 11 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "main", sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", sec.source_files = ["float-clamp-execution.mlir"], dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @clamp32(%x: f32 {sec.scalar_kind = "float32"}, %lo: f32 {sec.scalar_kind = "float32"}, %hi: f32 {sec.scalar_kind = "float32"}, %expected: f32 {sec.scalar_kind = "float32"}, %nan: i1 {sec.scalar_kind = "bool"}, %failure: i1 {sec.scalar_kind = "bool"}, %negativeZero: i1 {sec.scalar_kind = "bool"}) -> i1 attributes {sec.function_id = "main::clamp32(float32,float32,float32,float32,bool,bool,bool)", sec.source_name = "clamp32", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x", "lo", "hi", "expected", "nan", "failure", "negativeZero"]} {
    %v, %failed = "sec.float.clamp_checked"(%x, %lo, %hi) : (f32, f32, f32) -> (f32, i1)
    cf.cond_br %failed, ^bad, ^ok
  ^bad:
    return %failure : i1
  ^ok:
    %true = arith.constant true
    %zero = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %one = "sec.float.property"() {kind = "epsilon"} : () -> f32
    %equal = "sec.float.cmp"(%v, %expected) {predicate = "eq"} : (f32, f32) -> i1
    %isnan = "sec.float.classify"(%v) {kind = "nan"} : (f32) -> i1
    %valueOK = arith.select %nan, %isnan, %equal : i1
    %reciprocal = "sec.float.binary"(%one, %v) {kind = "div"} : (f32, f32) -> f32
    %negative = "sec.float.cmp"(%reciprocal, %zero) {predicate = "lt"} : (f32, f32) -> i1
    %signOK = arith.cmpi eq, %negative, %negativeZero : i1
    %iszero = "sec.float.cmp"(%v, %zero) {predicate = "eq"} : (f32, f32) -> i1
    %zeroOK = arith.select %iszero, %signOK, %true : i1
    %successExpected = arith.xori %failure, %true : i1
    %goodValue = arith.andi %valueOK, %zeroOK : i1
    %good = arith.andi %goodValue, %successExpected : i1
    return %good : i1
  }
  func.func @clamp64(%x: f64 {sec.scalar_kind = "float64"}, %lo: f64 {sec.scalar_kind = "float64"}, %hi: f64 {sec.scalar_kind = "float64"}, %expected: f64 {sec.scalar_kind = "float64"}, %nan: i1 {sec.scalar_kind = "bool"}, %failure: i1 {sec.scalar_kind = "bool"}, %negativeZero: i1 {sec.scalar_kind = "bool"}) -> i1 attributes {sec.function_id = "main::clamp64(float64,float64,float64,float64,bool,bool,bool)", sec.source_name = "clamp64", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x", "lo", "hi", "expected", "nan", "failure", "negativeZero"]} {
    %v, %failed = "sec.float.clamp_checked"(%x, %lo, %hi) : (f64, f64, f64) -> (f64, i1)
    cf.cond_br %failed, ^bad, ^ok
  ^bad:
    return %failure : i1
  ^ok:
    %true = arith.constant true
    %zero = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %one = "sec.float.property"() {kind = "epsilon"} : () -> f64
    %equal = "sec.float.cmp"(%v, %expected) {predicate = "eq"} : (f64, f64) -> i1
    %isnan = "sec.float.classify"(%v) {kind = "nan"} : (f64) -> i1
    %valueOK = arith.select %nan, %isnan, %equal : i1
    %reciprocal = "sec.float.binary"(%one, %v) {kind = "div"} : (f64, f64) -> f64
    %negative = "sec.float.cmp"(%reciprocal, %zero) {predicate = "lt"} : (f64, f64) -> i1
    %signOK = arith.cmpi eq, %negative, %negativeZero : i1
    %iszero = "sec.float.cmp"(%v, %zero) {predicate = "eq"} : (f64, f64) -> i1
    %zeroOK = arith.select %iszero, %signOK, %true : i1
    %successExpected = arith.xori %failure, %true : i1
    %goodValue = arith.andi %valueOK, %zeroOK : i1
    %good = arith.andi %goodValue, %successExpected : i1
    return %good : i1
  }
  func.func @clampPlatform(%x: !sec.float {sec.scalar_kind = "float"}, %lo: !sec.float {sec.scalar_kind = "float"}, %hi: !sec.float {sec.scalar_kind = "float"}, %expected: !sec.float {sec.scalar_kind = "float"}, %nan: i1 {sec.scalar_kind = "bool"}, %failure: i1 {sec.scalar_kind = "bool"}, %negativeZero: i1 {sec.scalar_kind = "bool"}) -> i1 attributes {sec.function_id = "main::clampPlatform(float,float,float,float,bool,bool,bool)", sec.source_name = "clampPlatform", sec.extern = false, sec.unsafe = false, sec.parameter_names = ["x", "lo", "hi", "expected", "nan", "failure", "negativeZero"]} {
    %v, %failed = "sec.float.clamp_checked"(%x, %lo, %hi) : (!sec.float, !sec.float, !sec.float) -> (!sec.float, i1)
    cf.cond_br %failed, ^bad, ^ok
  ^bad:
    return %failure : i1
  ^ok:
    %true = arith.constant true
    %zero = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %one = "sec.float.property"() {kind = "epsilon"} : () -> !sec.float
    %equal = "sec.float.cmp"(%v, %expected) {predicate = "eq"} : (!sec.float, !sec.float) -> i1
    %isnan = "sec.float.classify"(%v) {kind = "nan"} : (!sec.float) -> i1
    %valueOK = arith.select %nan, %isnan, %equal : i1
    %reciprocal = "sec.float.binary"(%one, %v) {kind = "div"} : (!sec.float, !sec.float) -> !sec.float
    %negative = "sec.float.cmp"(%reciprocal, %zero) {predicate = "lt"} : (!sec.float, !sec.float) -> i1
    %signOK = arith.cmpi eq, %negative, %negativeZero : i1
    %iszero = "sec.float.cmp"(%v, %zero) {predicate = "eq"} : (!sec.float, !sec.float) -> i1
    %zeroOK = arith.select %iszero, %signOK, %true : i1
    %successExpected = arith.xori %failure, %true : i1
    %goodValue = arith.andi %valueOK, %zeroOK : i1
    %good = arith.andi %goodValue, %successExpected : i1
    return %good : i1
  }
  func.func @main() -> i32 attributes {sec.function_id = "main::main()", sec.source_name = "main", sec.extern = false, sec.unsafe = false, sec.parameter_names = []} {
    %true = arith.constant true
    %false = arith.constant false
    %status0 = arith.constant 0 : i32
    // Case 1: clamp32, value/bounds/expected = (-2, -1, 1, -1).
    %v1_0 = "sec.const.float"() {lexeme = "-2", bits = 3221225472 : i64} : () -> f32
    %v1_1 = "sec.const.float"() {lexeme = "-1", bits = 3212836864 : i64} : () -> f32
    %v1_2 = "sec.const.float"() {lexeme = "1", bits = 1065353216 : i64} : () -> f32
    %v1_3 = "sec.const.float"() {lexeme = "-1", bits = 3212836864 : i64} : () -> f32
    %ok1 = func.call @clamp32(%v1_0, %v1_1, %v1_2, %v1_3, %false, %false, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code1 = arith.constant 1 : i32
    %status1 = arith.select %ok1, %status0, %code1 : i32
    // Case 2: clamp32, value/bounds/expected = (0.5, -1, 1, 0.5).
    %v2_0 = "sec.const.float"() {lexeme = "0.5", bits = 1056964608 : i64} : () -> f32
    %v2_1 = "sec.const.float"() {lexeme = "-1", bits = 3212836864 : i64} : () -> f32
    %v2_2 = "sec.const.float"() {lexeme = "1", bits = 1065353216 : i64} : () -> f32
    %v2_3 = "sec.const.float"() {lexeme = "0.5", bits = 1056964608 : i64} : () -> f32
    %ok2 = func.call @clamp32(%v2_0, %v2_1, %v2_2, %v2_3, %false, %false, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code2 = arith.constant 2 : i32
    %status2 = arith.select %ok2, %status1, %code2 : i32
    // Case 3: clamp32, value/bounds/expected = (2, -1, 1, 1).
    %v3_0 = "sec.const.float"() {lexeme = "2", bits = 1073741824 : i64} : () -> f32
    %v3_1 = "sec.const.float"() {lexeme = "-1", bits = 3212836864 : i64} : () -> f32
    %v3_2 = "sec.const.float"() {lexeme = "1", bits = 1065353216 : i64} : () -> f32
    %v3_3 = "sec.const.float"() {lexeme = "1", bits = 1065353216 : i64} : () -> f32
    %ok3 = func.call @clamp32(%v3_0, %v3_1, %v3_2, %v3_3, %false, %false, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code3 = arith.constant 3 : i32
    %status3 = arith.select %ok3, %status2, %code3 : i32
    // Case 4: clamp32, value/bounds/expected = (0, 1, -1, 0).
    %v4_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %v4_1 = "sec.const.float"() {lexeme = "1", bits = 1065353216 : i64} : () -> f32
    %v4_2 = "sec.const.float"() {lexeme = "-1", bits = 3212836864 : i64} : () -> f32
    %v4_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %ok4 = func.call @clamp32(%v4_0, %v4_1, %v4_2, %v4_3, %false, %true, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code4 = arith.constant 4 : i32
    %status4 = arith.select %ok4, %status3, %code4 : i32
    // Case 5: clamp32, value/bounds/expected = (0, nan, 1, 0).
    %v5_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %v5_1 = "sec.const.float"() {lexeme = "nan", bits = 2143289344 : i64} : () -> f32
    %v5_2 = "sec.const.float"() {lexeme = "1", bits = 1065353216 : i64} : () -> f32
    %v5_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %ok5 = func.call @clamp32(%v5_0, %v5_1, %v5_2, %v5_3, %false, %true, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code5 = arith.constant 5 : i32
    %status5 = arith.select %ok5, %status4, %code5 : i32
    // Case 6: clamp32, value/bounds/expected = (0, -1, nan, 0).
    %v6_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %v6_1 = "sec.const.float"() {lexeme = "-1", bits = 3212836864 : i64} : () -> f32
    %v6_2 = "sec.const.float"() {lexeme = "nan", bits = 2143289344 : i64} : () -> f32
    %v6_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %ok6 = func.call @clamp32(%v6_0, %v6_1, %v6_2, %v6_3, %false, %true, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code6 = arith.constant 6 : i32
    %status6 = arith.select %ok6, %status5, %code6 : i32
    // Case 7: clamp32, value/bounds/expected = (nan, -1, 1, 0).
    %v7_0 = "sec.const.float"() {lexeme = "nan", bits = 2143289344 : i64} : () -> f32
    %v7_1 = "sec.const.float"() {lexeme = "-1", bits = 3212836864 : i64} : () -> f32
    %v7_2 = "sec.const.float"() {lexeme = "1", bits = 1065353216 : i64} : () -> f32
    %v7_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %ok7 = func.call @clamp32(%v7_0, %v7_1, %v7_2, %v7_3, %true, %false, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code7 = arith.constant 7 : i32
    %status7 = arith.select %ok7, %status6, %code7 : i32
    // Case 8: clamp32, value/bounds/expected = (-0.0, -inf, inf, 0).
    %v8_0 = "sec.const.float"() {lexeme = "-0.0", bits = 2147483648 : i64} : () -> f32
    %v8_1 = "sec.const.float"() {lexeme = "-inf", bits = 4286578688 : i64} : () -> f32
    %v8_2 = "sec.const.float"() {lexeme = "inf", bits = 2139095040 : i64} : () -> f32
    %v8_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %ok8 = func.call @clamp32(%v8_0, %v8_1, %v8_2, %v8_3, %false, %false, %true) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code8 = arith.constant 8 : i32
    %status8 = arith.select %ok8, %status7, %code8 : i32
    // Case 9: clamp32, value/bounds/expected = (0, -0.0, 0, 0).
    %v9_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %v9_1 = "sec.const.float"() {lexeme = "-0.0", bits = 2147483648 : i64} : () -> f32
    %v9_2 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %v9_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %ok9 = func.call @clamp32(%v9_0, %v9_1, %v9_2, %v9_3, %false, %false, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code9 = arith.constant 9 : i32
    %status9 = arith.select %ok9, %status8, %code9 : i32
    // Case 10: clamp32, value/bounds/expected = (-0.0, -0.0, 0, 0).
    %v10_0 = "sec.const.float"() {lexeme = "-0.0", bits = 2147483648 : i64} : () -> f32
    %v10_1 = "sec.const.float"() {lexeme = "-0.0", bits = 2147483648 : i64} : () -> f32
    %v10_2 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %v10_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %ok10 = func.call @clamp32(%v10_0, %v10_1, %v10_2, %v10_3, %false, %false, %true) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code10 = arith.constant 10 : i32
    %status10 = arith.select %ok10, %status9, %code10 : i32
    // Case 11: clamp32, value/bounds/expected = (-1, 0, 0, 0).
    %v11_0 = "sec.const.float"() {lexeme = "-1", bits = 3212836864 : i64} : () -> f32
    %v11_1 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %v11_2 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %v11_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %ok11 = func.call @clamp32(%v11_0, %v11_1, %v11_2, %v11_3, %false, %false, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code11 = arith.constant 11 : i32
    %status11 = arith.select %ok11, %status10, %code11 : i32
    // Case 12: clamp32, value/bounds/expected = (1, -0.0, -0.0, 0).
    %v12_0 = "sec.const.float"() {lexeme = "1", bits = 1065353216 : i64} : () -> f32
    %v12_1 = "sec.const.float"() {lexeme = "-0.0", bits = 2147483648 : i64} : () -> f32
    %v12_2 = "sec.const.float"() {lexeme = "-0.0", bits = 2147483648 : i64} : () -> f32
    %v12_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %ok12 = func.call @clamp32(%v12_0, %v12_1, %v12_2, %v12_3, %false, %false, %true) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code12 = arith.constant 12 : i32
    %status12 = arith.select %ok12, %status11, %code12 : i32
    // Case 13: clamp32, value/bounds/expected = (inf, -inf, inf, inf).
    %v13_0 = "sec.const.float"() {lexeme = "inf", bits = 2139095040 : i64} : () -> f32
    %v13_1 = "sec.const.float"() {lexeme = "-inf", bits = 4286578688 : i64} : () -> f32
    %v13_2 = "sec.const.float"() {lexeme = "inf", bits = 2139095040 : i64} : () -> f32
    %v13_3 = "sec.const.float"() {lexeme = "inf", bits = 2139095040 : i64} : () -> f32
    %ok13 = func.call @clamp32(%v13_0, %v13_1, %v13_2, %v13_3, %false, %false, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code13 = arith.constant 13 : i32
    %status13 = arith.select %ok13, %status12, %code13 : i32
    // Case 14: clamp32, value/bounds/expected = (-inf, -inf, inf, -inf).
    %v14_0 = "sec.const.float"() {lexeme = "-inf", bits = 4286578688 : i64} : () -> f32
    %v14_1 = "sec.const.float"() {lexeme = "-inf", bits = 4286578688 : i64} : () -> f32
    %v14_2 = "sec.const.float"() {lexeme = "inf", bits = 2139095040 : i64} : () -> f32
    %v14_3 = "sec.const.float"() {lexeme = "-inf", bits = 4286578688 : i64} : () -> f32
    %ok14 = func.call @clamp32(%v14_0, %v14_1, %v14_2, %v14_3, %false, %false, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code14 = arith.constant 14 : i32
    %status14 = arith.select %ok14, %status13, %code14 : i32
    // Case 15: clamp32, value/bounds/expected = (0, -inf, -inf, -inf).
    %v15_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f32
    %v15_1 = "sec.const.float"() {lexeme = "-inf", bits = 4286578688 : i64} : () -> f32
    %v15_2 = "sec.const.float"() {lexeme = "-inf", bits = 4286578688 : i64} : () -> f32
    %v15_3 = "sec.const.float"() {lexeme = "-inf", bits = 4286578688 : i64} : () -> f32
    %ok15 = func.call @clamp32(%v15_0, %v15_1, %v15_2, %v15_3, %false, %false, %false) : (f32, f32, f32, f32, i1, i1, i1) -> i1
    %code15 = arith.constant 15 : i32
    %status15 = arith.select %ok15, %status14, %code15 : i32
    // Case 16: clamp64, value/bounds/expected = (-2, -1, 1, -1).
    %v16_0 = "sec.const.float"() {lexeme = "-2", bits = -4611686018427387904 : i64} : () -> f64
    %v16_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> f64
    %v16_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> f64
    %v16_3 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> f64
    %ok16 = func.call @clamp64(%v16_0, %v16_1, %v16_2, %v16_3, %false, %false, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code16 = arith.constant 16 : i32
    %status16 = arith.select %ok16, %status15, %code16 : i32
    // Case 17: clamp64, value/bounds/expected = (0.5, -1, 1, 0.5).
    %v17_0 = "sec.const.float"() {lexeme = "0.5", bits = 4602678819172646912 : i64} : () -> f64
    %v17_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> f64
    %v17_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> f64
    %v17_3 = "sec.const.float"() {lexeme = "0.5", bits = 4602678819172646912 : i64} : () -> f64
    %ok17 = func.call @clamp64(%v17_0, %v17_1, %v17_2, %v17_3, %false, %false, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code17 = arith.constant 17 : i32
    %status17 = arith.select %ok17, %status16, %code17 : i32
    // Case 18: clamp64, value/bounds/expected = (2, -1, 1, 1).
    %v18_0 = "sec.const.float"() {lexeme = "2", bits = 4611686018427387904 : i64} : () -> f64
    %v18_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> f64
    %v18_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> f64
    %v18_3 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> f64
    %ok18 = func.call @clamp64(%v18_0, %v18_1, %v18_2, %v18_3, %false, %false, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code18 = arith.constant 18 : i32
    %status18 = arith.select %ok18, %status17, %code18 : i32
    // Case 19: clamp64, value/bounds/expected = (0, 1, -1, 0).
    %v19_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %v19_1 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> f64
    %v19_2 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> f64
    %v19_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %ok19 = func.call @clamp64(%v19_0, %v19_1, %v19_2, %v19_3, %false, %true, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code19 = arith.constant 19 : i32
    %status19 = arith.select %ok19, %status18, %code19 : i32
    // Case 20: clamp64, value/bounds/expected = (0, nan, 1, 0).
    %v20_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %v20_1 = "sec.const.float"() {lexeme = "nan", bits = 9221120237041090560 : i64} : () -> f64
    %v20_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> f64
    %v20_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %ok20 = func.call @clamp64(%v20_0, %v20_1, %v20_2, %v20_3, %false, %true, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code20 = arith.constant 20 : i32
    %status20 = arith.select %ok20, %status19, %code20 : i32
    // Case 21: clamp64, value/bounds/expected = (0, -1, nan, 0).
    %v21_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %v21_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> f64
    %v21_2 = "sec.const.float"() {lexeme = "nan", bits = 9221120237041090560 : i64} : () -> f64
    %v21_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %ok21 = func.call @clamp64(%v21_0, %v21_1, %v21_2, %v21_3, %false, %true, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code21 = arith.constant 21 : i32
    %status21 = arith.select %ok21, %status20, %code21 : i32
    // Case 22: clamp64, value/bounds/expected = (nan, -1, 1, 0).
    %v22_0 = "sec.const.float"() {lexeme = "nan", bits = 9221120237041090560 : i64} : () -> f64
    %v22_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> f64
    %v22_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> f64
    %v22_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %ok22 = func.call @clamp64(%v22_0, %v22_1, %v22_2, %v22_3, %true, %false, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code22 = arith.constant 22 : i32
    %status22 = arith.select %ok22, %status21, %code22 : i32
    // Case 23: clamp64, value/bounds/expected = (-0.0, -inf, inf, 0).
    %v23_0 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> f64
    %v23_1 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> f64
    %v23_2 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> f64
    %v23_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %ok23 = func.call @clamp64(%v23_0, %v23_1, %v23_2, %v23_3, %false, %false, %true) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code23 = arith.constant 23 : i32
    %status23 = arith.select %ok23, %status22, %code23 : i32
    // Case 24: clamp64, value/bounds/expected = (0, -0.0, 0, 0).
    %v24_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %v24_1 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> f64
    %v24_2 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %v24_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %ok24 = func.call @clamp64(%v24_0, %v24_1, %v24_2, %v24_3, %false, %false, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code24 = arith.constant 24 : i32
    %status24 = arith.select %ok24, %status23, %code24 : i32
    // Case 25: clamp64, value/bounds/expected = (-0.0, -0.0, 0, 0).
    %v25_0 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> f64
    %v25_1 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> f64
    %v25_2 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %v25_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %ok25 = func.call @clamp64(%v25_0, %v25_1, %v25_2, %v25_3, %false, %false, %true) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code25 = arith.constant 25 : i32
    %status25 = arith.select %ok25, %status24, %code25 : i32
    // Case 26: clamp64, value/bounds/expected = (-1, 0, 0, 0).
    %v26_0 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> f64
    %v26_1 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %v26_2 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %v26_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %ok26 = func.call @clamp64(%v26_0, %v26_1, %v26_2, %v26_3, %false, %false, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code26 = arith.constant 26 : i32
    %status26 = arith.select %ok26, %status25, %code26 : i32
    // Case 27: clamp64, value/bounds/expected = (1, -0.0, -0.0, 0).
    %v27_0 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> f64
    %v27_1 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> f64
    %v27_2 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> f64
    %v27_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %ok27 = func.call @clamp64(%v27_0, %v27_1, %v27_2, %v27_3, %false, %false, %true) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code27 = arith.constant 27 : i32
    %status27 = arith.select %ok27, %status26, %code27 : i32
    // Case 28: clamp64, value/bounds/expected = (inf, -inf, inf, inf).
    %v28_0 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> f64
    %v28_1 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> f64
    %v28_2 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> f64
    %v28_3 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> f64
    %ok28 = func.call @clamp64(%v28_0, %v28_1, %v28_2, %v28_3, %false, %false, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code28 = arith.constant 28 : i32
    %status28 = arith.select %ok28, %status27, %code28 : i32
    // Case 29: clamp64, value/bounds/expected = (-inf, -inf, inf, -inf).
    %v29_0 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> f64
    %v29_1 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> f64
    %v29_2 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> f64
    %v29_3 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> f64
    %ok29 = func.call @clamp64(%v29_0, %v29_1, %v29_2, %v29_3, %false, %false, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code29 = arith.constant 29 : i32
    %status29 = arith.select %ok29, %status28, %code29 : i32
    // Case 30: clamp64, value/bounds/expected = (0, -inf, -inf, -inf).
    %v30_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> f64
    %v30_1 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> f64
    %v30_2 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> f64
    %v30_3 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> f64
    %ok30 = func.call @clamp64(%v30_0, %v30_1, %v30_2, %v30_3, %false, %false, %false) : (f64, f64, f64, f64, i1, i1, i1) -> i1
    %code30 = arith.constant 30 : i32
    %status30 = arith.select %ok30, %status29, %code30 : i32
    // Case 31: clampPlatform, value/bounds/expected = (-2, -1, 1, -1).
    %v31_0 = "sec.const.float"() {lexeme = "-2", bits = -4611686018427387904 : i64} : () -> !sec.float
    %v31_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> !sec.float
    %v31_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> !sec.float
    %v31_3 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> !sec.float
    %ok31 = func.call @clampPlatform(%v31_0, %v31_1, %v31_2, %v31_3, %false, %false, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code31 = arith.constant 31 : i32
    %status31 = arith.select %ok31, %status30, %code31 : i32
    // Case 32: clampPlatform, value/bounds/expected = (0.5, -1, 1, 0.5).
    %v32_0 = "sec.const.float"() {lexeme = "0.5", bits = 4602678819172646912 : i64} : () -> !sec.float
    %v32_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> !sec.float
    %v32_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> !sec.float
    %v32_3 = "sec.const.float"() {lexeme = "0.5", bits = 4602678819172646912 : i64} : () -> !sec.float
    %ok32 = func.call @clampPlatform(%v32_0, %v32_1, %v32_2, %v32_3, %false, %false, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code32 = arith.constant 32 : i32
    %status32 = arith.select %ok32, %status31, %code32 : i32
    // Case 33: clampPlatform, value/bounds/expected = (2, -1, 1, 1).
    %v33_0 = "sec.const.float"() {lexeme = "2", bits = 4611686018427387904 : i64} : () -> !sec.float
    %v33_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> !sec.float
    %v33_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> !sec.float
    %v33_3 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> !sec.float
    %ok33 = func.call @clampPlatform(%v33_0, %v33_1, %v33_2, %v33_3, %false, %false, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code33 = arith.constant 33 : i32
    %status33 = arith.select %ok33, %status32, %code33 : i32
    // Case 34: clampPlatform, value/bounds/expected = (0, 1, -1, 0).
    %v34_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %v34_1 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> !sec.float
    %v34_2 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> !sec.float
    %v34_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %ok34 = func.call @clampPlatform(%v34_0, %v34_1, %v34_2, %v34_3, %false, %true, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code34 = arith.constant 34 : i32
    %status34 = arith.select %ok34, %status33, %code34 : i32
    // Case 35: clampPlatform, value/bounds/expected = (0, nan, 1, 0).
    %v35_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %v35_1 = "sec.const.float"() {lexeme = "nan", bits = 9221120237041090560 : i64} : () -> !sec.float
    %v35_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> !sec.float
    %v35_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %ok35 = func.call @clampPlatform(%v35_0, %v35_1, %v35_2, %v35_3, %false, %true, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code35 = arith.constant 35 : i32
    %status35 = arith.select %ok35, %status34, %code35 : i32
    // Case 36: clampPlatform, value/bounds/expected = (0, -1, nan, 0).
    %v36_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %v36_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> !sec.float
    %v36_2 = "sec.const.float"() {lexeme = "nan", bits = 9221120237041090560 : i64} : () -> !sec.float
    %v36_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %ok36 = func.call @clampPlatform(%v36_0, %v36_1, %v36_2, %v36_3, %false, %true, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code36 = arith.constant 36 : i32
    %status36 = arith.select %ok36, %status35, %code36 : i32
    // Case 37: clampPlatform, value/bounds/expected = (nan, -1, 1, 0).
    %v37_0 = "sec.const.float"() {lexeme = "nan", bits = 9221120237041090560 : i64} : () -> !sec.float
    %v37_1 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> !sec.float
    %v37_2 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> !sec.float
    %v37_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %ok37 = func.call @clampPlatform(%v37_0, %v37_1, %v37_2, %v37_3, %true, %false, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code37 = arith.constant 37 : i32
    %status37 = arith.select %ok37, %status36, %code37 : i32
    // Case 38: clampPlatform, value/bounds/expected = (-0.0, -inf, inf, 0).
    %v38_0 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> !sec.float
    %v38_1 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> !sec.float
    %v38_2 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> !sec.float
    %v38_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %ok38 = func.call @clampPlatform(%v38_0, %v38_1, %v38_2, %v38_3, %false, %false, %true) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code38 = arith.constant 38 : i32
    %status38 = arith.select %ok38, %status37, %code38 : i32
    // Case 39: clampPlatform, value/bounds/expected = (0, -0.0, 0, 0).
    %v39_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %v39_1 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> !sec.float
    %v39_2 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %v39_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %ok39 = func.call @clampPlatform(%v39_0, %v39_1, %v39_2, %v39_3, %false, %false, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code39 = arith.constant 39 : i32
    %status39 = arith.select %ok39, %status38, %code39 : i32
    // Case 40: clampPlatform, value/bounds/expected = (-0.0, -0.0, 0, 0).
    %v40_0 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> !sec.float
    %v40_1 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> !sec.float
    %v40_2 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %v40_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %ok40 = func.call @clampPlatform(%v40_0, %v40_1, %v40_2, %v40_3, %false, %false, %true) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code40 = arith.constant 40 : i32
    %status40 = arith.select %ok40, %status39, %code40 : i32
    // Case 41: clampPlatform, value/bounds/expected = (-1, 0, 0, 0).
    %v41_0 = "sec.const.float"() {lexeme = "-1", bits = -4616189618054758400 : i64} : () -> !sec.float
    %v41_1 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %v41_2 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %v41_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %ok41 = func.call @clampPlatform(%v41_0, %v41_1, %v41_2, %v41_3, %false, %false, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code41 = arith.constant 41 : i32
    %status41 = arith.select %ok41, %status40, %code41 : i32
    // Case 42: clampPlatform, value/bounds/expected = (1, -0.0, -0.0, 0).
    %v42_0 = "sec.const.float"() {lexeme = "1", bits = 4607182418800017408 : i64} : () -> !sec.float
    %v42_1 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> !sec.float
    %v42_2 = "sec.const.float"() {lexeme = "-0.0", bits = -9223372036854775808 : i64} : () -> !sec.float
    %v42_3 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %ok42 = func.call @clampPlatform(%v42_0, %v42_1, %v42_2, %v42_3, %false, %false, %true) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code42 = arith.constant 42 : i32
    %status42 = arith.select %ok42, %status41, %code42 : i32
    // Case 43: clampPlatform, value/bounds/expected = (inf, -inf, inf, inf).
    %v43_0 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> !sec.float
    %v43_1 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> !sec.float
    %v43_2 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> !sec.float
    %v43_3 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> !sec.float
    %ok43 = func.call @clampPlatform(%v43_0, %v43_1, %v43_2, %v43_3, %false, %false, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code43 = arith.constant 43 : i32
    %status43 = arith.select %ok43, %status42, %code43 : i32
    // Case 44: clampPlatform, value/bounds/expected = (-inf, -inf, inf, -inf).
    %v44_0 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> !sec.float
    %v44_1 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> !sec.float
    %v44_2 = "sec.const.float"() {lexeme = "inf", bits = 9218868437227405312 : i64} : () -> !sec.float
    %v44_3 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> !sec.float
    %ok44 = func.call @clampPlatform(%v44_0, %v44_1, %v44_2, %v44_3, %false, %false, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code44 = arith.constant 44 : i32
    %status44 = arith.select %ok44, %status43, %code44 : i32
    // Case 45: clampPlatform, value/bounds/expected = (0, -inf, -inf, -inf).
    %v45_0 = "sec.const.float"() {lexeme = "0", bits = 0 : i64} : () -> !sec.float
    %v45_1 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> !sec.float
    %v45_2 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> !sec.float
    %v45_3 = "sec.const.float"() {lexeme = "-inf", bits = -4503599627370496 : i64} : () -> !sec.float
    %ok45 = func.call @clampPlatform(%v45_0, %v45_1, %v45_2, %v45_3, %false, %false, %false) : (!sec.float, !sec.float, !sec.float, !sec.float, i1, i1, i1) -> i1
    %code45 = arith.constant 45 : i32
    %status45 = arith.select %ok45, %status44, %code45 : i32
    return %status45 : i32
  }
}
