package sema

import (
	"fmt"
	"os"
	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestPitfallConstantFixes distinguishes semantic equivalence from intent repair
// and from removing getters, calls, failures or ownership transfers. Every
// automatic replacement is rechecked in its original source context.
// Rules: rules/analysis/pitfall_analysis.md — "Corrective actions", "Fix safety",
// "Required corrective-action tests".
func TestPitfallConstantFixes(t *testing.T) {
	path := "../../testdata/sema/pitfall_constant_fixes_valid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	p := parser.New(lexer.NewWithFile(source, path))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzerWithDepth(AnalysisDeep)
	if errs := a.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	for _, test := range []struct {
		name  string
		kind  PitfallActionKind
		value string
	}{
		{"Tautology", PitfallProvenFix, "true"}, {"Impossible", PitfallProvenFix, "false"},
		{"Chain", PitfallProvenFix, "false"}, {"Unsigned", PitfallProvenFix, "true"},
		{"Mirrored", PitfallProvenFix, "false"}, {"Stored", PitfallProvenFix, "true"},
		{"Getter", PitfallSuggestedEdit, "true"}, {"Call", PitfallSuggestedEdit, "true"},
		{"Effect", PitfallSuggestedEdit, "true"}, {"Failure", PitfallSuggestedEdit, "false"},
		{"Consume", PitfallSuggestedEdit, "false"},
		{"Physical", PitfallSuggestedEdit, "false"},
		{"Index", PitfallSuggestedEdit, "true"},
	} {
		t.Run(test.name, func(t *testing.T) {
			finding := fixSafetyResultFor(t, program, a, test.name)
			action := finding.Actions[len(finding.Actions)-1]
			if action.Kind != test.kind || action.Replacement != test.value || action.Safety.verifiedFor(finding.Rule, test.value) != (test.kind == PitfallProvenFix) {
				t.Fatal(action)
			}
			if test.kind != PitfallProvenFix {
				return
			}
			// Interpret the original source AST and actual emitted replacement,
			// exercising interval endpoints and both short-circuit branches.
			for _, fn := range fixSafetyFunctions(program) {
				if fn.Name.Value != test.name {
					continue
				}
				expr := fn.Body.Statements[0].(*ast.ReturnStatement).Value
				replacement := parseFixBoolean(t, action.Replacement)
				for value := -20; value <= 20; value++ {
					for _, flag := range []bool{false, true} {
						vars := map[string]any{"value": int64(value), "FLOOR": int64(0), "LIMIT": int64(10), "flag": flag, "count": int64(value + 20), "reading.value": int64(value + 20)}
						if constantFixEval(expr, vars) != constantFixEval(replacement, vars) {
							t.Fatal(test.name, value, flag)
						}
					}
				}
			}
			start := strings.Index(source, "fn "+test.name+"(")
			start += strings.Index(source[start:], "return ") + len("return ")
			end := start + strings.Index(source[start:], " }")
			rewritten := source[:start] + action.Replacement + source[end:]
			p := parser.New(lexer.New(rewritten))
			replacement := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			if errs := NewAnalyzerWithDepth(AnalysisDeep).Analyze(replacement); len(errs) != 0 {
				t.Fatal(errs)
			}
			for _, field := range []string{"EvaluationOrder", "EvaluationCount", "Effects", "Failure", "Ownership", "Borrow", "ControlFlow"} {
				broken := clonePitfallFinding(finding)
				last := len(broken.Actions) - 1
				// Mutating one obligation must revoke automatic status.
				switch field {
				case "EvaluationOrder":
					broken.Actions[last].Safety.EvaluationOrder = ""
				case "EvaluationCount":
					broken.Actions[last].Safety.EvaluationCount = ""
				case "Effects":
					broken.Actions[last].Safety.Effects = ""
				case "Failure":
					broken.Actions[last].Safety.Failure = ""
				case "Ownership":
					broken.Actions[last].Safety.Ownership = ""
				case "Borrow":
					broken.Actions[last].Safety.Borrow = ""
				case "ControlFlow":
					broken.Actions[last].Safety.ControlFlow = ""
				}
				requirePitfallFixSafety(&broken)
				if broken.Actions[last].Kind != PitfallSuggestedEdit {
					t.Fatal(field, broken)
				}
			}
			// The likely intended membership rewrite must never inherit the
			// certificate proving the original tautology equals true.
			if test.name == "Tautology" && (len(finding.Actions) != 2 || finding.Actions[0].Kind != PitfallSuggestedEdit || finding.Actions[0].Replacement != "value in FLOOR..LIMIT") {
				t.Fatal(finding)
			}
		})
	}
}

// constantFixEval interprets primitive source expressions for independent truth
// comparisons against the emitted replacement; it does not call Sema's interval
// solver or safety predicate.
// Rules: rules/foundations/operators.md — ordered comparisons, "Logical AND", "Logical OR".
func constantFixEval(expr ast.Expression, values map[string]any) any {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return values[expr.Value]
	case *ast.MemberExpression:
		return values[expr.String()]
	case *ast.BooleanLiteral:
		return expr.Value
	case *ast.IntegerLiteral:
		value, ok := ast.ParseIntegerLiteralLexeme(expr.Token.Lexeme)
		if !ok {
			panic(expr.String())
		}
		return value.Int64()
	case *ast.InfixExpression:
		left := constantFixEval(expr.Left, values)
		if expr.Operator == "&&" && !left.(bool) {
			return false
		}
		if expr.Operator == "||" && left.(bool) {
			return true
		}
		right := constantFixEval(expr.Right, values)
		switch expr.Operator {
		case "&&":
			return left.(bool) && right.(bool)
		case "||":
			return left.(bool) || right.(bool)
		case "<":
			return left.(int64) < right.(int64)
		case "<=":
			return left.(int64) <= right.(int64)
		case ">":
			return left.(int64) > right.(int64)
		case ">=":
			return left.(int64) >= right.(int64)
		}
	}
	panic(fmt.Sprintf("unsupported evaluation: %T", expr))
}
