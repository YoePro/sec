package secmlir

import (
	"fmt"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
)

// TestSemanticSchemaCompatibilityAcrossEmissionSchemas ensures dialect output
// selection never bypasses the independently owned Semantic IR schema boundary.
// Rules: rules/compiler/semantic_ir.md — §§ 103(3), 106(1)–(5).
func TestSemanticSchemaCompatibilityAcrossEmissionSchemas(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		for _, schema := range []uint32{0, 11, 12, 13, 14} {
			t.Run(fmt.Sprintf("width%d/schema%d", width, schema), func(t *testing.T) {
				module := boolModule(t)
				output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: schema})
				if err != nil || !strings.Contains(string(output), fmt.Sprintf("sec.semantic_ir_version = %d : i32", semantic.Version)) {
					t.Fatalf("current schema lost its version: %s; %v", output, err)
				}
				for _, version := range []uint32{0, semantic.Version - 1, semantic.Version + 1, ^uint32(0)} {
					if version == semantic.Version {
						continue
					}
					module.Version = version
					output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: schema})
					if len(output) != 0 || err == nil || !strings.Contains(err.Error(), "unsupported semantic IR version") {
						t.Fatalf("incompatible schema %d emitted %q; %v", version, output, err)
					}
					output, err = Emit(module, testPlan(width))
					if len(output) != 0 || err == nil || !strings.Contains(err.Error(), "unsupported semantic IR version") {
						t.Fatalf("default emitter accepted incompatible schema %d: %q; %v", version, output, err)
					}
				}
			})
		}
	}
}
