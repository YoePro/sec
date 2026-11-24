package sema

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
)

// P19 § 150 with § 83 (the sized/unsized/default/destruction rows are
// TestArenaTypedAllocationRequirementsUseDistinctDiagnostics): wide active
// integers and decimal128 are accepted with a resolved layout on linux/amd64;
// a checked count * size beyond the target's maximum object size, or a
// payload larger than the known FromBuffer backing, can never succeed and is
// a compile-time error; fitting and zero-element allocations stay valid.
func TestArenaTypeRestrictionMatrix(t *testing.T) {
	definition, ok := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	if !ok {
		t.Fatal("no linux/amd64 target")
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	analyze := func(body string) (*Analyzer, []string) {
		source := "module main\n\nfn F() Result[int64, AllocationError] {\n    let mut buffer: byte[16]\n    let mut arena := Arena.FromBuffer(ref mut buffer[..])\n" + body + "    return Ok(0)\n}\n\nfn main() int {\n    return 0\n}\n"
		p := parser.New(lexer.NewWithFile(source, "restrictions.sec"))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(plan)
		var messages []string
		for _, err := range a.Analyze(program) {
			messages = append(messages, err.Error())
		}
		return a, messages
	}
	for name, test := range map[string]struct {
		body, want  string
		size, align int64
	}{
		"int128 accepted":     {"    let a := try arena.New[int128]()\n", "", 16, 16},
		"uint256 accepted":    {"    let a := try arena.Alloc[uint256](0)\n", "", 32, 0},
		"decimal128 accepted": {"    let a := try arena.Alloc[decimal128](0)\n", "", 32, 16},
		"fitting allocation":  {"    let a := try arena.Alloc[int64](2)\n", "", 8, 8},
		"count * size overflow": {"    let a := try arena.Alloc[int64](9223372036854775807)\n",
			"can never succeed: 9223372036854775807 elements of int64 need 73786976294838206456 bytes, more than the target's maximum object size", 0, 0},
		"payload beyond buffer": {"    let a := try arena.Alloc[int64](4)\n", "can never succeed: 32 bytes of int64 exceed the 16-byte buffer", 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			a, errs := analyze(test.body)
			if test.want != "" {
				if len(errs) != 1 || !strings.Contains(errs[0], test.want) {
					t.Fatalf("errors %v, want one containing %q", errs, test.want)
				}
				return
			}
			if len(errs) != 0 {
				t.Fatal(errs)
			}
			for _, plan := range a.arenaOperationPlans {
				if plan.Operation != ArenaOperationNew && plan.Operation != ArenaOperationAlloc {
					continue
				}
				if plan.ElementLayout == nil || plan.ElementLayout.SizeBytes.Int64() != test.size || test.align != 0 && plan.ElementLayout.AlignBytes.Int64() != test.align {
					t.Fatalf("layout %+v, want size %d align %d", plan.ElementLayout, test.size, test.align)
				}
			}
		})
	}
}
