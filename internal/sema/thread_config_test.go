package sema

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestThreadConfigSource exercises the exact six fields through ordinary
// aggregate/union typing and consuming arguments, including contextual sugar.
// Rules: rules/concurrency/threads.md — §§10–11.
func TestThreadConfigSource(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		data, err := os.ReadFile("../../testdata/sema/thread_config_" + mode + ".sec")
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if mode == "invalid" {
			if len(errors) != 10 {
				t.Fatalf("invalid (%d): %v", len(errors), errors)
			}
			continue
		}
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		typ := a.Types()["ThreadConfig"]
		if !typ.Intrinsic || typ.Module != "core" || !typ.ExplicitlyNonCopyable || implicitlyCopyable(typ) || len(typ.Fields) != 6 {
			t.Fatal(typ)
		}
		defaults := DefaultValueOf(typ)
		if defaults.Kind != StructDefault || len(defaults.Fields) != 6 {
			t.Fatal(defaults)
		}
		for index, name := range []string{"Name", "Stack", "Affinity", "Priority", "Start", "Storage"} {
			field := defaults.Fields[index]
			if field.Name != name {
				t.Fatal(field)
			}
			want := "Default"
			if name == "Storage" {
				want = "None"
			}
			if name == "Start" {
				if field.Value.Kind != EnumDefault || field.Value.Value.Lexeme != "Eager" {
					t.Fatal(field)
				}
			} else if field.Value.Kind != UnionDefault || field.Value.Variant != want {
				t.Fatal(field)
			}
		}
		// Configuration defaults do not silently make arbitrary ThreadSetting
		// values defaultable (threads.md §4).
		if DefaultValueOf(typ.Fields[0].Type).Kind != NoDefault {
			t.Fatal("leaked configuration default")
		}
		if typ.Fields[5].Type.TypeArgs[0].Kind != ReferenceType || !typ.Fields[5].Type.TypeArgs[0].ReferenceMutable {
			t.Fatal(typ.Fields[5])
		}
	}
}

// TestThreadConfigLiteralPlans checks the shared immutable construction plans,
// including default values when legacy AST materialization is disabled.
// Rules: rules/concurrency/threads.md — §§10(1),11(2–3);
// rules/mlir/packages/sec-mlir-dialect_package13.md — §26.
func TestThreadConfigLiteralPlans(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_config_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{true, false} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		a.SetLegacyASTDefaultMaterialization(legacy)
		if errors := a.Analyze(program); len(errors) != 0 {
			t.Fatal(errors)
		}
		configured, defaults := 0, 0
		ast.Visit(program, func(node any) bool {
			literal, ok := node.(*ast.StructLiteral)
			if !ok || literal.Type.Name != "ThreadConfig" {
				return true
			}
			plan, ok := a.ResolvedStructLiteralPlanOf(literal)
			if !ok || !plan.FullyInitialized || len(plan.FinalFields) != 6 {
				t.Fatal("missing initialization plan", plan, ok)
			}
			for _, field := range plan.FinalFields {
				if field.SourceKind == StructFieldSourceDefault {
					defaults++
					want := threadConfigFieldDefault(plan.StructType, plan.StructType.Fields[field.FieldID])
					if field.Default.Kind != want.Kind || field.Default.Variant != want.Variant || field.Default.Value.Lexeme != want.Value.Lexeme {
						t.Fatal(field)
					}
				}
			}
			configured++
			return true
		})
		if configured != 5 || defaults != 23 {
			t.Fatal(configured, defaults)
		}
	}
}

// TestThreadConfigCoreDeclaration rejects malformed trusted replacements while
// preserving canonical identity and real member declaration locations.
// Rules: rules/concurrency/threads.md — §10(1–2).
func TestThreadConfigCoreDeclaration(t *testing.T) {
	file := "../../testdata/sema/thread_config_core.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ old, new string }{
		{"", ""}, {"@noCopy\n", ""}, {"ThreadConfig struct", "ThreadConfig[T] struct"},
		{"Name: ThreadSetting[string]", "Name: string"},
		{"Stack: ThreadSetting[uint]", "Stack: ThreadSetting[int]"},
		{"Storage: Option[ref mut ThreadStorage]", "Storage: Option[ref ThreadStorage]"},
		{"Start: ThreadStartMode", "Start: ThreadPriority"},
		{"Affinity: ThreadSetting[CpuSet]", "Unknown: ThreadSetting[CpuSet]"},
	} {
		p := parser.New(lexer.NewWithFile(strings.Replace(string(data), tc.old, tc.new, 1), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if tc.old == "" {
			if len(errors) != 0 || a.Types()["ThreadConfig"].Fields[0].Token.File != file {
				t.Fatal(errors, "missing real field definition")
			}
			program.SourceProvenance = nil
			if errors := NewAnalyzer().Analyze(program); len(errors) != 1 {
				t.Fatal("untrusted replacement", errors)
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "canonical non-generic @noCopy") {
			t.Fatal(tc, errors)
		}
	}
}

// TestThreadConfigTargetStack validates the Required uint payload against the
// CompilationPlan, never a host width or an unspecialized generic placeholder.
// Rules: rules/concurrency/threads.md — §§4,10(1);
// rules/memory/layout.md — target scalar plan.
func TestThreadConfigTargetStack(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/thread_config_target.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
		errors := a.Analyze(program)
		if width == 32 && len(errors) != 1 || width == 64 && len(errors) != 0 {
			t.Fatal(width, errors)
		}
	}
}
