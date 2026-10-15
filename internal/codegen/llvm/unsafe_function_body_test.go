package llvm

import (
	"os"
	"strings"
	"testing"
)

// Named asm outputs remain inside an explicit unsafe block through the return;
// unsafe callable status grants no lexical permission to its implementation.
// Rules: rules/memory/unsafe.md — §7(1–3);
// rules/platform/inline_assembly.md — §7, §30.
func TestGenerateUnsafeFunctionWithNamedAsmOutput(t *testing.T) {
	input, err := os.ReadFile("../../../testdata/codegen/unsafe_named_asm_output.sec")
	if err != nil {
		t.Fatal(err)
	}
	program := parseAndAnalyze(t, string(input))
	got, err := GenerateWithTriple(program, "x86_64-pc-linux-gnu")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	expectedParts := []string{
		`define i64 @_rawSyscall3(i64 %number, i64 %arg1, i64 %arg2, i64 %arg3)`,
		`call i64 asm sideeffect "syscall", "={rax},{rax},{rdi},{rsi},{rdx},~{rcx},~{r11},~{memory}"`,
		`ret i64`,
		`ret i64`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(got, part) {
			t.Fatalf("generated LLVM IR missing %q.\nIR:\n%s", part, got)
		}
	}
}
