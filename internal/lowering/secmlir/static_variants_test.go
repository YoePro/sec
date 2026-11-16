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

// Static union, Option and Result values keep Sema's active variant and payload
// through Semantic IR into schema-13 sec.static.global (SVV-2): direct and
// executed constructors, nested Result[Option[Point]], error-enum payloads,
// named-field and fixed-array payloads, string payloads, empty variants, None
// and Ok of Result[void, E]. The dialect checks each tree against the union
// definition or the Result's components on 32- and 64-bit targets.
// Rules: rules/compiler/compile_time_evaluation.md — § 32(3);
// rules/declarations/static.md — §§ 15–16; rules/compiler/semantic_ir.md — § 28.
func TestSchema13StaticVariantValues(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/sema/static_variants_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "static_variants_valid.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "static_variants_valid", SourceFiles: []string{"static_variants_valid.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			if err := semantic.Verify(module); err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			for _, want := range []string{
				"::Nested\": !", "init=Ok(Some({42, true}))", "init=Err(.Bad)", "init=Record({{42, true}, 7})",
				`init=Text("Sec")`, "init=Empty", "init=None", "init=Ok ", "init=Some(3)", "init=Bytes({1, 2})",
				`function="Local" init=Number(9)`,
			} {
				if !strings.Contains(text, want) {
					t.Errorf("Semantic IR lacks %q", want)
				}
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{
				`initial = "variant"`,
				`sec.static_initializer = {payload = {payload = ["42", "true"], variant = "Some"}, variant = "Ok"}`,
				`{payload = {enum = "Bad"}, variant = "Err"}`,
				`{payload = {string = "Sec"}, variant = "Text"}`,
				`{payload = [["42", "true"], "7"], variant = "Record"}`,
			} {
				if !strings.Contains(string(output), want) {
					t.Errorf("emitted module lacks %q", want)
				}
			}
			verify := exec.Command(tool, "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
