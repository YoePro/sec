package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// P20 §§ 74, 76: float statics, scalar and inside an aggregate, emit their
// canonical "bits:<decimal>" leaves and the dialect verifies them.
func TestFloatStaticEmission(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := "module main\n\ntype Point struct {\n    x: float32,\n    y: float64,\n}\n\nlet Tiny: float32 := 0.1\nlet Origin := Point { x: 1.5, y: 2.0 }\n\nfn Read() float32 {\n    return Tiny + Origin.x\n}\n\nfn main() int {\n    return 0\n}\n"
	module, err := buildPackage19Source(t, source, "amd64", 64)
	if err != nil {
		t.Fatal(err)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 14})
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, want := range []string{`initial = "bits:1036831949"`, `"bits:1069547520"`, `"bits:4611686018427387904"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	verify := exec.Command(tool, "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}
}

// P20 §§ 73-74: schema 16 emits every float constant with its canonical
// bits and verifies; a constant without canonical bits has no schema-16
// form, while schema 15 still accepts it.
func TestSchema16FloatConstants(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	module, err := buildPackage19Source(t, "module main\n\nfn Half() float64 {\n    return 0.5\n}\n\nfn main() int {\n    return 0\n}\n", "amd64", 64)
	if err != nil {
		t.Fatal(err)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 16})
	if err != nil {
		t.Fatal(err)
	}
	if text := string(output); !strings.Contains(text, "sec.dialect_version = 16 : i32") || !strings.Contains(text, `"sec.const.float"() <{bits = 4602678819172646912 : i64, lexeme = "0.5"}>`) {
		t.Fatalf("schema-16 module:\n%s", text)
	}
	verify := exec.Command(tool, "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s", err, result)
	}
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for index := range block.Operations {
				if block.Operations[index].Float != nil {
					block.Operations[index].Float = nil
				}
			}
		}
	}
	if _, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 16}); err == nil || !strings.Contains(err.Error(), "canonical bits") {
		t.Fatalf("schema 16 emitted a lexeme-only float constant: %v", err)
	}
	if _, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 15}); err != nil {
		t.Fatalf("schema 15 rejected a lexeme-only float constant: %v", err)
	}
}
