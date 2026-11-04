// RUN: sec-mlir-opt %s --split-input-file --verify-diagnostics

!m = !sec.named<"main::Meters", !sec.int>
!k = !sec.named<"main::Kilometers", !m>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "named", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @same(%value: !m) -> !m {
    // expected-error@+1 {{must change the nominal identity of its operand}}
    %same = "sec.named.convert"(%value) : (!m) -> !m
    return %same : !m
  }
}

// -----

!m = !sec.named<"main::Meters", !sec.int>
!k = !sec.named<"main::Kilometers", !m>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "named", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @carrier(%value: !m) -> !sec.uint {
    // expected-error@+1 {{operand and result must share one builtin Sec integer carrier}}
    %wrong = "sec.named.convert"(%value) : (!m) -> !sec.uint
    return %wrong : !sec.uint
  }
}

// -----

!m = !sec.named<"main::Meters", !sec.int>
!k = !sec.named<"main::Kilometers", !m>
module attributes {sec.dialect_version = 13 : i32, sec.semantic_ir_version = 1 : i32, sec.module_id = "named", sec.source_files = [], sec.target_os = "linux", sec.target_arch = "amd64", sec.target_triple = "x86_64-pc-linux-gnu", sec.target_abi = "gnu", sec.target_profile = "hosted", sec.target_endianness = "little", dlti.dl_spec = #dlti.dl_spec<#dlti.dl_entry<index, 64>>} {
  func.func @mixed(%left: !m, %right: !sec.int) -> i1 {
    // expected-error@+1 {{comparison operands must have the same builtin Sec integer, char or rune semantic type}}
    %less = "sec.int.cmp"(%left, %right) <{predicate = "lt"}> : (!m, !sec.int) -> i1
    return %less : i1
  }
}
