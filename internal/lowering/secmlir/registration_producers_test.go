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

// P17-19 registration producers reach verified schema 13: a declaration without
// an initializer starts with its default and is replaced in place; stored and nested
// fields are replaced under their owner's registration; an owned parameter
// replaced whole or through a field lives in storage from entry and inherits
// its registration.
// Rules: rules/memory/destruction.md — §§ 6.1, 13; rules/compiler/semantic_ir.md — § 40.
func TestSchema13RegistrationProducers(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/registration_producers.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "registration.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"registration.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			body := func(name string) string {
				start := strings.Index(text, `func "main::`+name+`(`)
				if start < 0 {
					t.Fatalf("missing function %s", name)
				}
				end := strings.Index(text[start:], "\n  }\n")
				return text[start : start+end]
			}
			for _, name := range []string{"DirectParameter", "ParameterFieldReplace"} {
				part := body(name)
				if !strings.Contains(part, "kind=parameter") || !strings.Contains(part, "storage.declare") || strings.Contains(part, "reason=replaced") {
					t.Errorf("%s does not hold its parameter in storage with an inherited registration:\n%s", name, part)
				}
			}
			// `let mut x: T` of a defaultable T starts with its default value,
			// so the first assignment replaces it under the inherited registration.
			for _, name := range []string{"EmptyThenAssign", "EmptyBranches"} {
				if part := body(name); strings.Count(part, "cleanup.track-owned") != 1 || !strings.Contains(part, "own.replace-place") || strings.Contains(part, "reason=replaced") {
					t.Errorf("%s does not replace its default under one registration:\n%s", name, part)
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
