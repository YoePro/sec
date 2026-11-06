package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// P17-25: interleaved owned-local and defer registrations keep source order;
// a replacement keeps its position before a defer and a reinitialization
// registers after it; branch, loop and jump exits run scope cleanup; returned
// and propagated values are protected; discard converges. Every function
// passes all cleanup/ownership verifiers on 32- and 64-bit plans, and the
// defer-body restrictions stay source errors.
// Rules: rules/memory/destruction.md — §§ 6, 9, 13, 35; rules/control-flow/defer.md — §§ 8–13.
func TestSchema13CleanupInterleaving(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/cleanup_interleaving.sec")
	if err != nil {
		t.Fatal(err)
	}
	ordinal := regexp.MustCompile(`cleanup\.(track-owned|defer-register) %\d+ action=#(\d+) scope=#\d+ ordinal=(\d+)`)
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "interleaving.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"interleaving.sec"}, MaxPackage: 17})
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
			kinds := func(name string) []string {
				var order []string
				for _, match := range ordinal.FindAllStringSubmatch(body(name), -1) {
					order = append(order, match[1])
				}
				return order
			}
			if got := strings.Join(kinds("Interleaved"), ","); got != "track-owned,defer-register,track-owned,defer-register,track-owned" {
				t.Errorf("Interleaved registration order %s", got)
			}
			// kept (inherited) and the first moved incarnation precede the
			// defer; the reinitialized moved value registers after it.
			if got := strings.Join(kinds("ReplaceAndReinitAroundDefer"), ","); got != "track-owned,track-owned,defer-register,track-owned" {
				t.Errorf("ReplaceAndReinitAroundDefer registration order %s", got)
			}
			for name, want := range map[string]string{
				"Branches":         "cleanup.run-scope",
				"LoopExits":        "exit=continue",
				"ReturnProtected":  "cleanup.cancel",
				"ErrProtected":     "exit=error-propagation",
				"DiscardConverges": "own.discard",
				"DiscardOnOnePath": "reason=discarded",
			} {
				if !strings.Contains(body(name), want) {
					t.Errorf("%s lacks %q:\n%s", name, want, body(name))
				}
			}
			if !strings.Contains(body("LoopExits"), "exit=break") {
				t.Errorf("LoopExits lacks its break cleanup:\n%s", body("LoopExits"))
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
	invalid, err := os.ReadFile("../../../testdata/sema/cleanup_interleaving_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	parsed := parser.New(lexer.NewWithFile(string(invalid), "invalid.sec")).Parse()
	var ids []string
	for _, diagnostic := range sema.NewAnalyzer().Analyze(parsed.Program) {
		ids = append(ids, diagnostic.ID)
	}
	if got := strings.Join(ids, ","); got != "S1203,S1093,S1203" {
		t.Errorf("defer-body restrictions reported %s, want S1203,S1093,S1203", got)
	}
}
