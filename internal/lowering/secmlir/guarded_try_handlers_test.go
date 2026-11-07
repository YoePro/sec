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

// P17-37 guarded try handlers on an owned error reach verified schema 13: the
// handler registers or holds the error only in its guard's true block, and a
// false guard reaches the next handler with the error still unowned by any
// handler, so it is destroyed exactly once on every path.
// Rules: rules/errors/errorhandling.md — § 19; rules/memory/destruction.md — § 8(8).
func TestSchema13GuardedTryHandlersOnOwnedErrors(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/guarded_try_handlers.sec")
	if err != nil {
		t.Fatal(err)
	}
	verifySource(t, tool, "../../../testdata/semantic_ir/package17_variants/error_handler_payload.sec")
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "guarded.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"guarded.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			start := strings.Index(text, `func "main::WholeGuard(`)
			whole := text[start : start+strings.Index(text[start:], "\n  }\n")]
			if strings.Count(whole, "cleanup.track-owned") != 1 || strings.Count(whole, "cause=temporary-end") != 1 {
				t.Errorf("the error is not registered once and ended once by the next handler:\n%s", whole)
			}
			for _, block := range strings.Split(whole, "\n    ^") {
				if strings.Contains(block, "cleanup.track-owned") && strings.Contains(block, "int.compare") {
					t.Errorf("the handler takes the error before its guard decides:\n%s", block)
				}
			}
			start = strings.Index(text, `func "main::MovedPayload(`)
			moved := text[start : start+strings.Index(text[start:], "\n  }\n")]
			candidate, move := strings.Index(moved, "place.read %"), strings.Index(moved, "[move] [handler=err-variant index=0")
			if candidate < 0 || !strings.Contains(moved[candidate:], "candidate") || move < candidate {
				t.Errorf("the guard does not read a candidate before the payload moves:\n%s", moved)
			}
			for _, block := range strings.Split(moved, "\n    ^") {
				if strings.Contains(block, "[move] [handler=err-variant index=0") && strings.Contains(block, "int.compare") {
					t.Errorf("the payload moves before its guard decides:\n%s", block)
				}
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

// verifySource builds one fixture at schema 13 and runs every ownership verifier.
func verifySource(t *testing.T, tool, path string) {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: 64})
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{path}, MaxPackage: 17})
	if err != nil {
		t.Fatal(err)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 13})
	if err != nil {
		t.Fatal(err)
	}
	verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", path, err, result)
	}
}
