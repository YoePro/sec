package secmlir

import (
	"os"
	"os/exec"
	"path/filepath"
	"sec/internal/testsupport"
	"strconv"
	"strings"
	"testing"
)

// Rules: rules/foundations/operators.md — "Boolean equality";
// rules/compiler/semantic_ir.md — §67(2) (typed value-producing merges).
func TestEmitBooleanEqualityUsesExistingTypedCFG(t *testing.T) {
	module := booleanFixtureModule(t, "boolean_equality")
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatal(err)
			}
			text := string(output)
			if !strings.Contains(text, "cf.cond_br") || !strings.Contains(text, `"sec.const.bool"`) || strings.Contains(text, `"sec.int.cmp"`) || strings.Contains(text, `"sec.bool.not"`) {
				t.Fatalf("lost bool representation or introduced an unsupported operation:\n%s", text)
			}
			tool, configured, err := testsupport.SecMLIROptPathFromEnvironment()
			if err != nil {
				t.Fatal(err)
			}
			if !configured {
				return
			}
			path := filepath.Join(t.TempDir(), "bool.mlir")
			if err := os.WriteFile(path, output, 0600); err != nil {
				t.Fatal(err)
			}
			if data, err := exec.Command(tool, path, "--sec-lower-scalar-core", "--sec-lower-trivial-core", "--verify-each", "-o", os.DevNull).CombinedOutput(); err != nil {
				t.Fatalf("Sec MLIR verification/lowering: %v\n%s", err, data)
			}
		})
	}
}

// Rules: rules/foundations/operators.md — "Boolean equality", "Evaluation order".
func TestExecuteBooleanEqualityThroughSecMLIR(t *testing.T) {
	executeBooleanFixture(t, "boolean_equality")
}
