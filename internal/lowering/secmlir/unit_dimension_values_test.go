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

// UnitDimension values that escape compile-time evaluation (MD-062 §§ 7(12),
// 12(8)): metadata reads used as values become canonical constants (sorted
// axis identities with non-zero exponents), and parameters, locals and
// struct fields are compared and read at run time by structural equality,
// AxisCount and IsDimensionless, with no hash, interning, registry or
// allocation. Equal dimensions of different units (m and km) are the same
// constant; a unit without a declared dimension keeps Sema's module-qualified
// default axis. Schema 13 carries !sec.unit_dimension and the four
// operations through every verifier on 32- and 64-bit targets.
// Rules: rules/types/units.md — "Unit metadata value types";
// rules/corrections/applied/md062-unit-metadata-value-types-correction-20261027-20261121.md — §§ 7(2)–(13), 12(7)–(8).
func TestSchema13UnitDimensionValues(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/unit_dimension_values.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "dimensions.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"dimensions.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			if err := semantic.Verify(module); err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			for _, want := range []string{
				"unit-dimension.compare %0 == %1", "unit-dimension.compare %0 != %1",
				"unit-dimension.axis-count %0", "unit-dimension.is-dimensionless %0",
				"unit-metadata=<m>.Dimension dimension=[length^1]",
				"unit-metadata=<km>.Dimension dimension=[length^1]",
				"unit-metadata=<mps>.Dimension dimension=[length^1, time^-1]",
				"unit-metadata=<ratio>.Dimension dimension=[main::ratio^1]",
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
				`"sec.unit_dimension.constant"() <{axes = [{axis = "length", exponent = 1 : i64}, {axis = "time", exponent = -1 : i64}]}> : () -> !sec.unit_dimension`,
				`<{predicate = "ne"}> : (!sec.unit_dimension, !sec.unit_dimension) -> i1`,
				`"sec.unit_dimension.axis_count"`, `"sec.unit_dimension.is_dimensionless"`,
			} {
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
