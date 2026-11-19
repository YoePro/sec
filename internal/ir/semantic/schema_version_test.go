package semantic

import (
	"fmt"
	"strings"
	"testing"
)

// TestSemanticSchemaVersionBoundary checks the compatibility boundary before
// malformed module contents can be used as semantic proof.
// Rules: rules/compiler/semantic_ir.md — §§ 5(3), 106(1)–(4).
func TestSemanticSchemaVersionBoundary(t *testing.T) {
	module := &Module{Version: Version, Identity: "schema-test", Types: NewTypeTable()}
	if err := Verify(module); err != nil {
		t.Fatalf("current schema: %v", err)
	}
	for _, version := range []uint32{0, Version - 1, Version + 1, ^uint32(0)} {
		if version == Version {
			continue
		}
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			// No type table: schema rejection must precede content inspection.
			err := Verify(&Module{Version: version})
			if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("unsupported semantic IR version %d", version)) {
				t.Fatalf("incompatible schema %d: %v", version, err)
			}
		})
	}
}
