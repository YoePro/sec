package mlir

import (
	"fmt"
	"strings"
)

// emitStringRuneLength counts Unicode scalars from valid UTF-8 without allocation.
// ByteLen retains the descriptor byte count. NUL is ordinary length-counted data.
// Rules: rules/compiler/compiler_known_members.md — Len, RuneLen, ByteLen on strings;
// rules/types/contracts.md — String and collection contracts (revision 2.1).
func (g *Generator) emitStringRuneLength(text value) value {
	for _, declarations := range g.functions {
		for _, declaration := range declarations {
			if declaration.LinkName == ".sec.generated.string_rune_length" {
				g.failure = fmt.Errorf("generated rune-length helper conflicts with source linkage")
				return value{}
			}
		}
	}
	if !strings.Contains(g.globals.String(), `llvm.func internal @".sec.generated.string_rune_length"`) {
		g.globals.WriteString(runeLengthMLIR)
	}
	result := g.nextTemp()
	g.write("    %s = llvm.call @\".sec.generated.string_rune_length\"(%s, %s) : (!llvm.ptr, i64) -> i64\n", result, text.ref, text.len)
	return value{typ: "i64", ref: result, unsigned: true}
}

const runeLengthMLIR = `
  // sec-generated string rune-length helper; rules/types/contracts.md — string length contracts
  llvm.func internal @".sec.generated.string_rune_length"(%data: !llvm.ptr, %length: i64) -> i64 {
    %zero = llvm.mlir.constant(0 : i64) : i64
    %one = llvm.mlir.constant(1 : i64) : i64
    %mask = llvm.mlir.constant(-64 : i8) : i8
    %continuation = llvm.mlir.constant(-128 : i8) : i8
    llvm.br ^scan(%zero, %zero : i64, i64)
  ^scan(%index: i64, %count: i64):
    %done = llvm.icmp "eq" %index, %length : i64
    llvm.cond_br %done, ^finish, ^byte
  ^byte:
    %address = llvm.getelementptr %data[%index] : (!llvm.ptr, i64) -> !llvm.ptr, i8
    %value = llvm.load %address : !llvm.ptr -> i8
    %prefix = llvm.and %value, %mask : i8
    %leading = llvm.icmp "ne" %prefix, %continuation : i8
    %increment = llvm.zext %leading : i1 to i64
    %updated = llvm.add %count, %increment : i64
    %next = llvm.add %index, %one : i64
    llvm.br ^scan(%next, %updated : i64, i64)
  ^finish:
    llvm.return %count : i64
  }
`
