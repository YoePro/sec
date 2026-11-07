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

// P17-40 fresh match subjects reach verified schema 13. A borrowed payload of a
// fresh subject is taken through the subject's compiler storage and the borrow
// ends before the arm destroys that storage, on fallthrough, return, value arms
// and a failing guard alike. A constructed subject moves its operand when it is
// evaluated and cancels the operand's registration.
// Rules: rules/control-flow/flowcontrol_match.md — § 13;
// rules/memory/destruction.md — § 8(8).
func TestSchema13FreshSubjectBorrows(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/fresh_subject_borrows.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "fresh.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"fresh.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			function := func(name string) string {
				start := strings.Index(text, `func "main::`+name+`(`)
				if start < 0 {
					t.Fatalf("missing function %s", name)
				}
				return text[start : start+strings.Index(text[start:], "\n  }\n")]
			}
			for _, name := range []string{"Shared", "Mutable", "FallsOut", "Expression", "TrivialSubject", "ResultSubject", "Guarded", "FieldOfFresh"} {
				part := function(name)
				if !strings.Contains(part, "own.initialize-place") || !strings.Contains(part, "ref.borrow-") {
					t.Errorf("%s does not borrow through compiler storage:\n%s", name, part)
				}
				// In every block, a borrow end precedes the subject's destruction.
				for _, block := range strings.Split(part, "\n    ^") {
					end, destroy := strings.Index(block, "ref.end-borrow"), strings.Index(block, "cause=temporary-end")
					if end >= 0 && destroy >= 0 && destroy < end {
						t.Errorf("%s destroys the subject before its payload borrow ends:\n%s", name, block)
					}
				}
			}
			taken := function("ConstructedTaken")
			move, cancel, construct := strings.Index(taken, "own.move"), strings.Index(taken, "cleanup.cancel action=#"), strings.Index(taken, "union.construct")
			if move < 0 || cancel < move || construct < cancel {
				t.Errorf("a constructed subject does not take its operand before it is built:\n%s", taken)
			}
			if !strings.Contains(function("ConstructedLeft"), "cause=temporary-end") {
				t.Errorf("an arm that leaves a constructed subject does not destroy it")
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
