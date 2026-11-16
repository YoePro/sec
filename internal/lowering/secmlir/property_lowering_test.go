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

// User-defined properties lower to their accessor functions: every read,
// write, fallible write and compound update is a call.direct carrying its
// property operation, with one receiver evaluation (an implicit borrow of the
// owner, or a copy/reborrow of the enclosing method's self). Accessors of a
// generic owner specialize per instance with concrete ownership, a getter's
// owned result is a fresh value, and a `try set` failure propagates through the
// enclosing Result (running owned cleanup) or enters local handlers. The module
// passes the schema-13 verifiers on 32- and 64-bit targets, and `sec.property`
// is emitted on each accessor call.
// Rules: rules/compiler/semantic_ir.md — § 26; rules/declarations/properties.md — §§ 4–9, 15;
// rules/errors/errorhandling.md — §§ 23–24; rules/compiler/monomorphization.md — § 9.3.
func TestSchema13PropertyLowering(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/properties.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "properties.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"properties.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			if err := semantic.Verify(module); err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			body := func(id string) string {
				start := strings.Index(text, `func "main::`+id)
				if start < 0 {
					t.Fatalf("missing function %s", id)
				}
				end := strings.Index(text[start:], "\n  }\n")
				return text[start : start+end]
			}
			for id, wants := range map[string][]string{
				"main()": {
					"property(write main::Counter.Count)", "property(compound-read main::Counter.Count)",
					"property(compound-write main::Counter.Count)", "property(read static main::Counter.Limit)",
					`call.direct "main::Counter.Fresh.get()"`, `plan="destroy:main:struct:Box"`,
				},
				"Counter.Bump()":                         {"ref.reborrow-mut %0", "ref.reborrow-shared", "property(compound-write main::Counter.Count)"},
				"Counter.Read()":                         {"ref.copy-shared %0", "property(read main::Counter.Count)"},
				"Generic()":                              {"property(write main::Cell<main::Box>.Value)", "%8[move]", "property(write main::Cell<int>.Value)"},
				"Cell.Value.set[T=main::Box](main::Box)": {"cleanup.track-owned %1", "own.replace-place"},
				"Propagate(ref mut main::Car,int)":       {"property(fallible-write main::Car.Speed)", "exit=error-propagation", "result.unwrap-err"},
				"Handled(int)":                           {"handler=err-catch-all", "property(compound-write main::Car.Speed)"},
				"Car.Speed.try-set(int)":                 {"result.err", "result.ok"},
			} {
				part := body(id)
				for _, want := range wants {
					if !strings.Contains(part, want) {
						t.Errorf("%s lacks %q:\n%s", id, want, part)
					}
				}
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{`sec.property = {kind = "fallible-write", name = "Speed", owner = "main::Car", static = false}`, `!sec.result<none, `} {
				if !strings.Contains(string(output), want) {
					t.Errorf("emitted module lacks %q", want)
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
