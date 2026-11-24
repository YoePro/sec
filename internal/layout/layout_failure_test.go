package layout

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
)

// Missing/contradictory representation diagnostics retain nominal type and
// selected target context through aggregates, named carriers and fixed arrays.
// Rules: MD-141 correction §10(1); rules/memory/layout.md — §42.
func TestLayoutFailureContext(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		plan := layoutTestPlan(width)
		for _, test := range []struct {
			name string
			req  LayoutRequirement
			want string
		}{
			{"target integer", scalar("main::Count", ScalarSigned, 0, true), "int"},
			{"bool", scalar("bool", ScalarBool, 0, false), "int8"},
			{"rune", scalar("rune", ScalarRune, 0, false), "int32"},
			{"float", scalar("float64", ScalarFloat, 64, false), "float64"},
			{"decimal", scalar("decimal128", ScalarDecimal, 128, false), "coefficient IntegerStorage(128), scale IntegerStorage(32)"},
			{"array", LayoutRequirement{TypeIdentity: "main::Values", Kind: LayoutArray, Element: ptrRequirement(scalar("float32", ScalarFloat, 32, false)), Count: big.NewInt(3)}, "float32"},
			{"named", LayoutRequirement{TypeIdentity: "main::Price", Kind: LayoutNamed, Carrier: ptrRequirement(scalar("float64", ScalarFloat, 64, false))}, "float64"},
			{"struct", LayoutRequirement{TypeIdentity: "main::Record", Kind: LayoutStruct, Fields: []FieldRequirement{{Name: "value", Requirement: scalar("int256", ScalarSigned, 256, false)}}}, "value: int256"},
		} {
			t.Run(fmt.Sprintf("%s/%d", test.name, width), func(t *testing.T) {
				message := DescribeLayoutFailure(plan, test.req, test.req.TypeIdentity, LayoutIncomplete, []string{"missing verified alignment"}, "Arena.Alloc")
				for _, want := range []string{test.req.TypeIdentity, "linux/test", plan.LLVMTriple, "profile hosted", "ABI gnu", test.want, "missing verified alignment", "Arena.Alloc needs a complete layout", "supply verified target data-layout"} {
					if !strings.Contains(message, want) {
						t.Errorf("missing %q: %s", want, message)
					}
				}
			})
		}
	}
	for _, state := range []LayoutCompleteness{LayoutInvalid, LayoutUnsupported} {
		message := DescribeLayoutFailure(ResolvedScalarPlan{}, scalar("int", ScalarSigned, 0, true), "int", state, []string{"contradictory fact"}, "materialization")
		for _, want := range []string{"triple unresolved", "profile unresolved", "ABI unresolved", "unresolved width", "contradictory fact"} {
			if !strings.Contains(message, want) {
				t.Errorf("missing %q: %s", want, message)
			}
		}
		if state == LayoutInvalid && !strings.Contains(message, "correct the contradictory") || state == LayoutUnsupported && !strings.Contains(message, "implement the required P24") {
			t.Fatal(message)
		}
	}
}

func ptrRequirement(requirement LayoutRequirement) *LayoutRequirement { return &requirement }
