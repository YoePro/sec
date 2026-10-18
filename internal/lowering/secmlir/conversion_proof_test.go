package secmlir

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// Exactness proofs must cross both the emitter and enum-erasure boundaries.
// Missing contracts and contracts from another plan fail before lowering;
// enum erasure cannot replace Sema's decision with its own domain analysis.
// Rules: rules/compiler/semantic_ir.md — § 124(2–3).
func TestConversionProofTransport(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/ir/conversion_proof_transport.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			plan := testPlan(width)
			module := scalarParityBuild(t, string(source), plan)
			output, err := Emit(module, plan)
			if err != nil {
				t.Fatal(err)
			}
			portable := `sec.conversion_proof = {basis = "closed-enum-domain", plan_pointer_width = 0 : i32}`
			dependent := fmt.Sprintf(`sec.conversion_proof = {basis = "source-domain", plan_pointer_width = %d : i32}`, width)
			for _, want := range []string{portable, dependent} {
				if !strings.Contains(string(output), want) {
					t.Fatalf("missing %s:\n%s", want, output)
				}
			}
			path := filepath.Join(t.TempDir(), "proof.mlir")
			run := func(text string, args ...string) ([]byte, error) {
				t.Helper()
				if err := os.WriteFile(path, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
				return exec.Command(tool, append([]string{path}, args...)...).CombinedOutput()
			}
			lowered, err := run(string(output), "--sec-resolve-scalar-layout", "--sec-lower-integer-enums")
			if err != nil {
				t.Fatalf("enum lowering: %v\n%s", err, lowered)
			}
			if strings.Contains(string(lowered), "sec.enum.to_integer") ||
				!strings.Contains(string(lowered), `basis = "closed-enum-domain"`) ||
				!strings.Contains(string(lowered), "sec.numeric.convert_exact") {
				t.Fatalf("enum erasure lost its proof:\n%s", lowered)
			}
			if result, err := run(string(output), "--sec-lower-to-llvm=entry="); err != nil {
				t.Fatalf("LLVM lowering: %v\n%s", err, result)
			}
			missing := strings.Replace(string(output), " {"+portable+"}", "", 1)
			if result, err := run(missing); err == nil || !strings.Contains(string(result), "requires a sec.conversion_proof dictionary") {
				t.Fatalf("accepted missing enum proof: %v\n%s", err, result)
			}
			other := uint16(32)
			if width == 32 {
				other = 64
			}
			wrong := strings.ReplaceAll(string(output), fmt.Sprintf("plan_pointer_width = %d : i32", width), fmt.Sprintf("plan_pointer_width = %d : i32", other))
			if result, err := run(wrong); err == nil || !strings.Contains(string(result), "incompatible with module plan width") {
				t.Fatalf("accepted wrong plan proof: %v\n%s", err, result)
			}
		})
	}
}
