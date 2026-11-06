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

// P17-40 temporaries reach verified schema 13: a mutably borrowed owned
// temporary is initialized in compiler storage, borrowed for the call and
// destroyed in place at the end of its full expression; `discard try` and a
// discarded stored field end through their destruction plans. Temporaries of
// a short-circuit operand or a match arm end with that region.
// Rules: rules/memory/destruction.md — § 8; rules/memory/borrowing.md — § 5.4.
func TestSchema13TemporaryEnds(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/temporary_ends.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "temporaries.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"temporaries.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			for _, name := range []string{"SomePaths", "ArmTemporary"} {
				start := strings.Index(text, `func "main::`+name+`(`)
				part := text[start : start+strings.Index(text[start:], "\n  }\n")]
				// The region's temporary ends in the region block, before the
				// branch that carries the region's value to the merge.
				destroy := strings.Index(part, "cause=temporary-end")
				if destroy < 0 || !strings.Contains(part[destroy:], "branch ^") {
					t.Errorf("%s does not end its region temporary before the merge:\n%s", name, part)
				}
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`"sec.ref.borrow_mut"(`, `cause = "temporary-end"`, `"sec.own.destroy_place"(`, `"sec.own.discard"(`} {
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
