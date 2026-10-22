package secmlir

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// P17-07/schema 13 is a named unsupported boundary, never a downgrade to the
// existing schema-11/12 track. Rejection leaves module and later output intact.
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
		var unsupported *UnsupportedLoweringError
		if output != nil || !errors.As(err, &unsupported) || !strings.Contains(unsupported.Feature, "P17") {
			t.Fatalf("schema 13 was downgraded or partially emitted: %q %v", output, err)
		}
		if unsupported.ImplementationCapability().Boundary != "Sec MLIR schema 13" || !strings.Contains(err.Error(), "schema 13") {
			t.Fatal("requested schema was replaced by the default diagnostic boundary")
		}
		after, err := Emit(module, testPlan(width))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("schema-13 rejection mutated default emission")
		}
		if output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 12}); err != nil || len(output) == 0 {
			t.Fatalf("P17 gate blocked existing schema 12: %v", err)
		}
	}
}
