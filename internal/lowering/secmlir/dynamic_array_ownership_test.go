package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// An owning T[] participates in P17 ownership through schema 14: a move into
// another owner and a consuming argument cancel the source cleanup, Result
// carries the returned owner, and a replacement is the transactional
// own.replace_place under the dynamic-array destruction plan, so the old owner
// is destroyed only after the new value is prepared. Every ownership, cleanup,
// destruction-plan, dynamic-array and storage-transition verifier accepts it.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 97–99, 128–130
func TestSchema14DynamicArrayOwnership(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/dynamic_array_ownership.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		arch  string
		width uint16
	}{{"armv7", 32}, {"amd64", 64}} {
		t.Run(fmt.Sprint(target.width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "ownership.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: target.arch, Profile: "hosted", PointerWidthBits: target.width})
			analyzer.SetAllocationProfile("hosted")
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"ownership.sec"}, MaxPackage: 18})
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(target.width), EmitOptions{DialectVersion: 14})
			if err != nil {
				t.Fatal(err)
			}
			text := string(output)
			for _, want := range []string{
				`"sec.own.replace_place"(`, `destruction_plan = "destroy:array:dynamic::int:int"`,
				`"sec.own.move_from_place"(`, `"sec.cleanup.cancel"(`, `"sec.cleanup.track_owned"(`,
			} {
				if !strings.Contains(text, want) {
					t.Errorf("schema-14 module lacks %q", want)
				}
			}
			verify := exec.Command(tool, "--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-places", "--sec-verify-reference-guards",
				"--sec-verify-borrow-semantics", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
