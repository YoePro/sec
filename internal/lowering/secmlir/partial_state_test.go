package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// P17-15 per-path partial state reaches verified schema 13: fields moved on
// some paths and reinitialized, every field moved and reinitialized, field
// moves in loops, an active payload moved on one path and replaced, and a
// nested field moved on some paths, read in place after reinitialization.
// Rules: rules/memory/copy_move.md — §§ 12–15; rules/memory/destruction.md — §§ 12–13.3.
func TestSchema13PerPathPartialState(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/partial_state.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "partial.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"partial.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`"sec.own.move_from_place"(`, `"sec.own.initialize_place"(`, `"sec.place.field"(`, `if_owned`} {
				if !bytes.Contains(output, []byte(want)) {
					t.Errorf("missing %s", want)
				}
			}
			verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
