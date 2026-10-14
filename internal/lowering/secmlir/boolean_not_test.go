package secmlir

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
)

// Rules: rules/foundations/operators.md — "Logical negation", "Logical operators".
func TestEmitBooleanNotPreservesTypedResultAndOperand(t *testing.T) {
	module := booleanFixtureModule(t, "boolean_negation")
	want := 0
	for _, fn := range module.Functions {
		for _, block := range fn.Blocks {
			for _, op := range block.Operations {
				if op.Kind == semantic.OpBoolNot {
					want++
				}
			}
		}
	}
	if want == 0 {
		t.Fatal("fixture did not exercise bool.not")
	}
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatal(err)
			}
			text := string(output)
			if got := strings.Count(text, "  ^bool_not_merge_"); got != want {
				t.Fatalf("negation merges = %d, want %d", got, want)
			}
			if strings.Contains(text, `"sec.bool.not"`) || strings.Contains(text, "arith.xori") {
				t.Fatal("unexpected new operation or premature arithmetic lowering")
			}
			tool, configured, err := testsupport.SecMLIROptPathFromEnvironment()
			if err != nil {
				t.Fatal(err)
			}
			if !configured {
				return
			}
			path := filepath.Join(t.TempDir(), "not.mlir")
			if err := os.WriteFile(path, output, 0600); err != nil {
				t.Fatal(err)
			}
			if data, err := exec.Command(tool, path, "--sec-lower-scalar-core", "--sec-lower-trivial-core", "--verify-each", "-o", os.DevNull).CombinedOutput(); err != nil {
				t.Fatalf("Sec MLIR: %v\n%s", err, data)
			}
		})
	}
}

func TestExecuteBooleanNotThroughSecMLIR(t *testing.T) {
	executeBooleanFixture(t, "boolean_negation")
}
