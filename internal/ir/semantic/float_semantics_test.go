package semantic

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// buildFloatPolicyMatrix combines every core operation with arithmetic,
// comparison and compound assignment on a selected target plan.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§18,20–27,63–69.
func buildFloatPolicyMatrix(t *testing.T, width uint16) *Module {
	t.Helper()
	data, err := os.ReadFile("../../../testdata/semantic_ir/float_core_members.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data) + `
fn PolicyOps(x: float64, y: float64) bool {
 let literal: float64 := 1.25
 discard literal
 let mut z := x
 z += y
 discard -z
 discard z % y
 return z != y
}
`
	p := parser.New(lexer.NewWithFile(source, "float_policy.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	m, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main"})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// All currently represented P20 families carry explicit strict facts. Mutating
// any fact must fail verification for every family, not just binary arithmetic.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§18,20–27,63–69,81.
func TestFloatSemanticPolicies(t *testing.T) {
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			m := buildFloatPolicyMatrix(t, width)
			if err := Verify(m); err != nil {
				t.Fatal(err)
			}
			seen := map[OpKind]*Operation{}
			var explicitOp *Operation
			for _, fn := range m.Functions {
				for _, block := range fn.Blocks {
					for i := range block.Operations {
						op := &block.Operations[i]
						if !floatSemanticOperation(op.Kind) {
							continue
						}
						facts := op.FloatSemantics
						source, ok := m.Types.Lookup(facts.SourceType)
						if !ok || source.Kind != TypeFloat || facts.FormatBits != source.BitWidth || facts.Rounding != "nearest-even" || facts.FastMath != "none" || facts.Contraction != "off" || facts.Subnormals != "preserve" {
							t.Fatalf("missing strict facts: %+v", op)
						}
						if op.Kind == OpFloatCompare && facts.ComparisonPredicate != "une" {
							t.Fatalf("!= must preserve unordered truth: %+v", facts)
						}
						if op.Kind == OpFloatBinary && op.FloatBinary == FloatBinaryRem && facts.RemainderKind != "truncation" {
							t.Fatal(facts)
						}
						seen[op.Kind] = op
						if source.Name == fmt.Sprintf("float%d", width) {
							explicitOp = op
						}
					}
				}
			}
			if len(seen) != 11 {
				t.Fatalf("expected all 11 float families, got %v", seen)
			}
			corruptions := map[string]func(*FloatSemantics){
				"missing":     func(f *FloatSemantics) { *f = FloatSemantics{} },
				"source":      func(f *FloatSemantics) { f.SourceType = 0 },
				"format":      func(f *FloatSemantics) { f.FormatBits = 16 },
				"rounding":    func(f *FloatSemantics) { f.Rounding = "toward-zero" },
				"fast-math":   func(f *FloatSemantics) { f.FastMath = "fast" },
				"contraction": func(f *FloatSemantics) { f.Contraction = "on" },
				"subnormals":  func(f *FloatSemantics) { f.Subnormals = "flush" },
				"predicate":   func(f *FloatSemantics) { f.ComparisonPredicate = "one" },
				"remainder":   func(f *FloatSemantics) { f.RemainderKind = "ieee" },
			}
			for kind, op := range seen {
				for name, corrupt := range corruptions {
					t.Run(string(kind)+"/"+name, func(t *testing.T) {
						original := op.FloatSemantics
						corrupt(&op.FloatSemantics)
						err := Verify(m)
						op.FloatSemantics = original
						if err == nil || !strings.Contains(err.Error(), "float semantic facts") {
							t.Fatalf("invalid %s %s accepted or misdiagnosed: %v", kind, name, err)
						}
					})
				}
			}
			text := Format(m)
			for _, want := range []string{"float-policy={source=#", "rounding=nearest-even fast-math=none contraction=off subnormals=preserve", "predicate=une", "remainder=truncation"} {
				if !strings.Contains(text, want) {
					t.Fatalf("IR dump lacks %q", want)
				}
			}
			// Same format is not proof of the same nominal source type.
			var plain TypeID
			for _, typ := range m.Types.All() {
				if typ.Kind == TypeFloat && typ.Name == "float" {
					plain = m.Types.Intern(typ)
				}
			}
			if explicitOp == nil || plain == 0 {
				t.Fatal("missing nominal provenance witnesses")
			}
			old := explicitOp.FloatSemantics
			explicitOp.FloatSemantics.SourceType = plain
			err := Verify(m)
			explicitOp.FloatSemantics = old
			if err == nil {
				t.Fatal("nominal float provenance silently changed")
			}

		})
	}
}

// A float constant's canonical bits and declared scalar format must agree;
// policy metadata cannot launder a binary32 payload as a binary64 constant.
// Rules: rules/mlir/packages/sec-mlir-dialect_package20.md §§13–18,81.
func TestFloatConstantScalarProvenance(t *testing.T) {
	m, err := buildPackage15Source(t, "module main\nfn main() float64 { return 1.25 }", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range m.Functions {
		for _, block := range fn.Blocks {
			for i := range block.Operations {
				op := &block.Operations[i]
				if op.Kind != OpConstFloat {
					continue
				}
				op.Float.Format = 32
				if err := Verify(m); err == nil || !strings.Contains(err.Error(), "payload format") {
					t.Fatalf("mismatched payload accepted: %v", err)
				}
				return
			}
		}
	}
	t.Fatal("no float constant")
}

// Planless platform float retains its unresolved width instead of borrowing
// the host's format. A selected plan is a separate lowering requirement.
// Rules: rules/mlir/normative-sync/sec_p20_october_compatibility_package21.md §2;
// rules/mlir/packages/sec-mlir-dialect_package20.md §§6–9,63–69.
func TestFloatSemanticUnresolvedFormat(t *testing.T) {
	p := parser.New(lexer.New("module main\nfn Plain(x: float) float { return x.Abs() }"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{})
	if errs := a.Analyze(parsed.Program); len(errs) != 0 {
		t.Fatal(errs)
	}
	m, err := Build(parsed.Program, a, BuildOptions{RequestedModule: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	for _, fn := range m.Functions {
		for _, b := range fn.Blocks {
			for _, op := range b.Operations {
				if op.Kind == OpFloatAbs {
					if op.FloatSemantics.FormatBits != 0 {
						t.Fatalf("planless float acquired a host format: %+v", op.FloatSemantics)
					}
					return
				}
			}
		}
	}
	t.Fatal("missing Abs operation")
}
