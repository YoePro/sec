package mlir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Translated LLVM IR takes the LLVM dialect module's symbol name as its
// ModuleID and source_filename; an unnamed module keeps the translator's
// fixed spelling.
// Rules: rules/compiler/compiler_pipeline.md — § 86.
func TestNameLLVMModule(t *testing.T) {
	translated := "; ModuleID = 'LLVMDialectModule'\nsource_filename = \"LLVMDialectModule\"\ntarget triple = \"x86_64-pc-linux-gnu\"\n"
	for name, check := range map[string]struct{ mlir, module string }{
		"plain":   {"module @util attributes {llvm.target_triple = \"x\"} {\n}\n", "util"},
		"quoted":  {"module @\"drivers/gpio\" {\n}\n", "drivers/gpio"},
		"unnamed": {"module attributes {} {\n}\n", "LLVMDialectModule"},
	} {
		dir := t.TempDir()
		mlirPath, llvmPath := filepath.Join(dir, "in.mlir"), filepath.Join(dir, "out.ll")
		if err := os.WriteFile(mlirPath, []byte(check.mlir), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(llvmPath, []byte(translated), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := nameLLVMModule(mlirPath, llvmPath); err != nil {
			t.Fatal(err)
		}
		text, _ := os.ReadFile(llvmPath)
		if !strings.Contains(string(text), "; ModuleID = '"+check.module+"'") || !strings.Contains(string(text), `source_filename = "`+check.module+`"`) {
			t.Errorf("%s:\n%s", name, text)
		}
	}
}
