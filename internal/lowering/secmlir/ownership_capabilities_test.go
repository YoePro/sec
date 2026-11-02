package secmlir

import (
	"bytes"
	"testing"
)

// Schema 13 is explicit: selecting it never changes the default or schema-12 tracks,
// and a later unknown schema is an error with no output.
// Rules: rules/mlir/packages/sec-mlir-dialect_package17.md — §§86,128–129,135;
// rules/compiler/compiler_pipeline.md — §§32–35.
func TestPackage17SchemaCapabilityBoundary(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		module := representativeModule(t)
		before, err := Emit(module, testPlan(width))
		if err != nil {
			t.Fatal(err)
		}
		output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
		if err != nil || !bytes.Contains(output, []byte("sec.dialect_version = 13 : i32")) {
			t.Fatalf("schema 13 was not honored: %q %v", output, err)
		}
		if output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 14}); output != nil || err == nil {
			t.Fatalf("schema 14 was accepted: %q", output)
		}
		after, err := Emit(module, testPlan(width))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("explicit schema selection mutated default emission")
		}
		if output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 12}); err != nil || len(output) == 0 {
			t.Fatalf("P17 gate blocked existing schema 12: %v", err)
		}
	}
}
