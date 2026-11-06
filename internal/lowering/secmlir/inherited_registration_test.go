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

// MD-125 inherited registration reaches verified schema 13: an available
// replacement keeps the live registration (equal old/new identities, no cancel
// or new track_owned), reinitialization registers anew, a conditional
// replacement registers only where nothing is live (unless_active), a
// replacement after field moves keeps the owner's registration, and a defer
// reading a replaced owner observes the new value before its destruction.
// Rules: rules/memory/destruction.md — §§ 6.1(4)–(5), 13.2(5)–(8), 13.3(5);
// rules/compiler/semantic_ir.md — § 40(7)–(10).
func TestSchema13InheritsReplacementRegistration(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/inherited_registration.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "inherited.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"inherited.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			functions := map[string]string{}
			for _, part := range strings.Split(text, "\n  func ")[1:] {
				functions[strings.Fields(part)[1]] = part
			}
			body := func(name string) string {
				for key, part := range functions {
					if strings.HasPrefix(key, name+"(") {
						return part
					}
				}
				t.Fatalf("missing function %s", name)
				return ""
			}
			same := regexp.MustCompile(`own\.replace-place .* cleanup=#(\d+)->#(\d+)`)
			for _, name := range []string{"Available", "NestedThenMove", "DeferReadsReplaced", "FallibleReplace", "RecoverReplace", "HandlerReturn"} {
				part := body(name)
				match := same.FindStringSubmatch(part)
				if match == nil || match[1] != match[2] || strings.Contains(part, "reason=replaced") || strings.Count(part, "cleanup.track-owned") != 1 {
					t.Errorf("%s does not inherit its registration:\n%s", name, part)
				}
			}
			if part := body("Reinitialize"); strings.Count(part, "cleanup.track-owned") != 2 || !strings.Contains(part, "own.initialize-place") {
				t.Errorf("Reinitialize does not register anew:\n%s", part)
			}
			for _, name := range []string{"Conditional", "ConditionalInLoop"} {
				if part := body(name); !strings.Contains(part, "unless-active=[") || !strings.Contains(part, "if-owned") {
					t.Errorf("%s lacks its conditional registration:\n%s", name, part)
				}
			}
			// The failure edge of a fallible right-hand side leaves through
			// function cleanup over the untouched old owner; the success edge
			// replaces exactly once (P17-39).
			if part := body("FallibleReplace"); strings.Count(part, "own.replace-place") != 1 || !strings.Contains(part, "cleanup.run-function exit=error-propagation") {
				t.Errorf("FallibleReplace lacks its failure exit or single replacement:\n%s", part)
			}
			if part := body("HandlerReturn"); strings.Count(part, "own.replace-place") != 1 || strings.Count(part, "cleanup.run-function exit=return") != 2 {
				t.Errorf("HandlerReturn does not run function cleanup on its handler return:\n%s", part)
			}
			if part := body("AfterFieldMoves"); strings.Contains(part, "reason=replaced") || strings.Count(part, "cleanup.track-owned") != 1 {
				t.Errorf("AfterFieldMoves does not keep the owner's registration:\n%s", part)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(output, []byte("unless_active = array<i32:")) {
				t.Errorf("missing unless_active in:\n%s", output)
			}
			verify := exec.Command(tool, "--sec-verify-slice-guards", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
