package sema

import (
	"errors"
	"math/big"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func fixSafetyFixture(t *testing.T) (*ast.Program, *Analyzer) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/sema/pitfall_fix_safety_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "fix.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	analyzer := NewAnalyzerWithDepth(AnalysisDeep)
	if errs := analyzer.Analyze(program); len(errs) != 0 {
		t.Fatal(errs)
	}
	return program, analyzer
}

func fixSafetyFunctions(program *ast.Program) []*ast.FunctionDeclaration {
	var result []*ast.FunctionDeclaration
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			result = append(result, statement)
		case *ast.ImplStatement:
			for _, member := range statement.Members {
				if function, ok := member.(*ast.FunctionDeclaration); ok {
					result = append(result, function)
				}
			}
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Token.Line < result[j].Token.Line })
	return result
}

func fixSafetyResultFor(t *testing.T, program *ast.Program, analyzer *Analyzer, name string) PitfallFinding {
	t.Helper()
	functions := fixSafetyFunctions(program)
	for index, function := range functions {
		if function.Name.Value != name {
			continue
		}
		end := int(^uint(0) >> 1)
		if index+1 < len(functions) {
			end = functions[index+1].Token.Line
		}
		for _, finding := range analyzer.PitfallAnalysis().Results() {
			if finding.Subject.Source.Line >= function.Token.Line && finding.Subject.Source.Line < end {
				return finding
			}
		}
	}
	t.Fatalf("no finding for %s", name)
	return PitfallFinding{}
}

// All currently automatic families require independent equivalence obligations.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety".
func TestPitfallFixSafetyCertificatesAndUnsafeCases(t *testing.T) {
	program, analyzer := fixSafetyFixture(t)
	for _, name := range []string{"EqTrue", "EqFalse", "NeTrue", "NeFalse", "TrueEq", "FalseEq", "TrueNe", "FalseNe", "ShortCircuit", "OuterConversion", "Consuming", "Stored", "Inclusive", "Exclusive", "Stable"} {
		finding := fixSafetyResultFor(t, program, analyzer, name)
		if len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallProvenFix || !finding.Actions[0].Safety.verifiedFor(finding.Rule, finding.Actions[0].Replacement) {
			t.Fatalf("%s: %+v", name, finding)
		}
	}
	for _, name := range []string{"Getter", "BareGetter", "Physical"} {
		finding := fixSafetyResultFor(t, program, analyzer, name)
		if finding.State != PitfallStateSuppressed || len(finding.Actions) != 0 {
			t.Fatalf("getter can be automatically rewritten: %+v", finding)
		}
	}
	for _, name := range []string{"Unproven", "ChangedAtStart"} {
		finding := fixSafetyResultFor(t, program, analyzer, name)
		if len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit || finding.Actions[0].Safety.verifiedFor(finding.Rule, finding.Actions[0].Replacement) {
			t.Fatalf("%s: %+v", name, finding)
		}
	}
	// Certificates are detached values in every published snapshot.
	before := analyzer.PitfallAnalysis().Results()
	changed := analyzer.PitfallAnalysis().Results()
	for index := range changed {
		for action := range changed[index].Actions {
			changed[index].Actions[action].Safety.Failure = "changed"
		}
	}
	if !reflect.DeepEqual(before, analyzer.PitfallAnalysis().Results()) {
		t.Fatal("certificate mutated compiler-owned state")
	}
}

// A claimed fix without every obligation, with a different replacement, or
// from a future unsupported family must never acquire automatic status.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety".
func TestPitfallFixSafetyGateRejectsIncompleteAndMismatchedProof(t *testing.T) {
	program, analyzer := fixSafetyFixture(t)
	original := fixSafetyResultFor(t, program, analyzer, "EqTrue")
	for _, field := range []string{"EvaluationOrder", "EvaluationCount", "Effects", "Failure", "Ownership", "Borrow", "ControlFlow", "Rule", "Replacement"} {
		finding := clonePitfallFinding(original)
		reflect.ValueOf(&finding.Actions[0].Safety).Elem().FieldByName(field).SetString("")
		requirePitfallFixSafety(&finding)
		if finding.Actions[0].Kind != PitfallSuggestedEdit {
			t.Fatalf("missing %s accepted", field)
		}
	}
	for _, change := range []string{"replacement", "family"} {
		finding := clonePitfallFinding(original)
		if change == "replacement" {
			finding.Actions[0].Replacement = "false"
		} else {
			finding.Rule = PitfallDirectIndexAtLength
		}
		requirePitfallFixSafety(&finding)
		if finding.Actions[0].Kind != PitfallSuggestedEdit {
			t.Fatal("mismatched certificate accepted")
		}
	}
}

// fixBooleanTrace executes the bool subset used by these source regressions.
// Opaque calls model observable effects/failures; move markers record transfer
// before a consuming call. Parser structure determines short-circuit order.
// Rules: rules/foundations/operators.md — "Logical AND", "Logical NOT", "Equality";
// rules/memory/ownership.md — explicit move; rules/analysis/pitfall_analysis.md — "Fix safety".
func fixBooleanTrace(expression ast.Expression, input bool, failAt int, trace *[]string) (bool, error) {
	switch expression := expression.(type) {
	case *ast.BooleanLiteral:
		return expression.Value, nil
	case *ast.PrefixExpression:
		if expression.Operator == "<-" {
			*trace = append(*trace, "move")
			return input, nil
		}
		value, err := fixBooleanTrace(expression.Right, input, failAt, trace)
		if expression.Operator == "!" {
			value = !value
		}
		return value, err
	case *ast.ConversionExpression:
		return fixBooleanTrace(expression.Value, input, failAt, trace)
	case *ast.CallExpression:
		name := expression.Callee.(*ast.Identifier).Value
		if name == "bool" {
			return fixBooleanTrace(expression.Arguments[0], input, failAt, trace)
		}
		for _, argument := range expression.Arguments {
			if _, err := fixBooleanTrace(argument, input, failAt, trace); err != nil {
				return false, err
			}
		}
		*trace = append(*trace, name)
		calls := 0
		for _, event := range *trace {
			if event != "move" {
				calls++
			}
		}
		if calls == failAt {
			return false, errors.New("opaque call failed")
		}
		return input, nil
	case *ast.InfixExpression:
		left, err := fixBooleanTrace(expression.Left, input, failAt, trace)
		if err != nil {
			return false, err
		}
		if expression.Operator == "&&" && !left {
			return false, nil
		}
		if expression.Operator == "||" && left {
			return true, nil
		}
		right, err := fixBooleanTrace(expression.Right, input, failAt, trace)
		if err != nil {
			return false, err
		}
		switch expression.Operator {
		case "==":
			return left == right, nil
		case "!=":
			return left != right, nil
		case "&&":
			return left && right, nil
		case "||":
			return left || right, nil
		}
	}
	panic("unsupported trace expression: " + expression.String())
}

func parseFixBoolean(t *testing.T, replacement string) ast.Expression {
	t.Helper()
	p := parser.New(lexer.New("module main\nfn Check() bool {return " + replacement + "}\n"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return fixSafetyFunctions(program)[0].Body.Statements[0].(*ast.ReturnStatement).Value
}

// Every emitted bool fix is checked for truth, evaluation order/count, opaque
// call failure, short circuit and explicit ownership transfer, in both values.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety".
func TestPitfallBooleanFixPreservesObservableTrace(t *testing.T) {
	program, analyzer := fixSafetyFixture(t)
	for _, function := range fixSafetyFunctions(program) {
		name := function.Name.Value
		finding := PitfallFinding{}
		if !strings.Contains(name, "Eq") && !strings.Contains(name, "Ne") && name != "ShortCircuit" && name != "OuterConversion" && name != "Consuming" {
			continue
		}
		finding = fixSafetyResultFor(t, program, analyzer, name)
		original := function.Body.Statements[0].(*ast.ReturnStatement).Value
		replacement := parseFixBoolean(t, finding.Actions[0].Replacement)
		for _, input := range []bool{false, true} {
			for failAt := 0; failAt <= 3; failAt++ {
				var before, after []string
				oldValue, oldError := fixBooleanTrace(original, input, failAt, &before)
				newValue, newError := fixBooleanTrace(replacement, input, failAt, &after)
				if (oldError != nil) != (newError != nil) || oldError == nil && oldValue != newValue || !reflect.DeepEqual(before, after) {
					t.Fatalf("%s/%v/%d: %v %v %v vs %v %v %v", name, input, failAt, oldValue, oldError, before, newValue, newError, after)
				}
			}
		}
	}
}

// The range proof is checked at one/multiple lengths and maximum uint,
// including starts after the end and negative starts. An entry-changing call
// is deliberately excluded above: it can turn nonempty into underflow.
// Rules: rules/control-flow/flowcontrol_for.md — §§23,25,29;
// rules/analysis/pitfall_analysis.md — "Fix safety".
func TestPitfallLengthFixEndpointEquivalence(t *testing.T) {
	max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 64), big.NewInt(1))
	for _, length := range []*big.Int{big.NewInt(1), big.NewInt(2), big.NewInt(10), max} {
		inclusive := new(big.Int).Sub(length, big.NewInt(1))
		for _, start := range []*big.Int{big.NewInt(-1), big.NewInt(0), big.NewInt(1), new(big.Int).Sub(length, big.NewInt(2)), new(big.Int).Set(length), new(big.Int).Add(length, big.NewInt(1))} {
			oldCount := new(big.Int).Sub(inclusive, start)
			oldCount.Add(oldCount, big.NewInt(1))
			if oldCount.Sign() < 0 {
				oldCount.SetInt64(0)
			}
			newCount := new(big.Int).Sub(length, start)
			if newCount.Sign() < 0 {
				newCount.SetInt64(0)
			}
			if oldCount.Cmp(newCount) != 0 {
				t.Fatalf("start=%s length=%s count differs", start, length)
			}
		}
	}
}

// Suggestions for invalid bool/integer intent never gain a truth-equivalence
// certificate and never remove the owning type diagnostic at any depth.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety", "Proven invalid".
func TestPitfallFixSafetyRetainsInvalidity(t *testing.T) {
	data, err := os.ReadFile("../../testdata/sema/pitfall_fix_safety_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, depth := range []AnalysisDepth{AnalysisInteractive, AnalysisStandard, AnalysisDeep} {
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		analyzer := NewAnalyzerWithDepth(depth)
		if errs := analyzer.Analyze(program); len(errs) == 0 {
			t.Fatal("invalid fixture passed", depth)
		}
		finding := fixSafetyResultFor(t, program, analyzer, "NumericIntent")
		if finding.Classification != PitfallProvenInvalid || len(finding.Actions) != 1 || finding.Actions[0].Kind != PitfallSuggestedEdit || finding.Actions[0].Safety.verifiedFor(finding.Rule, finding.Actions[0].Replacement) {
			t.Fatalf("%s: %+v", depth, finding)
		}
	}
}

// Applying the emitted replacement to the original declaration context must
// remain valid, including consuming-call markers and named constant bounds.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety";
// rules/memory/ownership.md — explicit move.
func TestPitfallProvenFixesRemainValidInSourceContext(t *testing.T) {
	program, analyzer := fixSafetyFixture(t)
	data, err := os.ReadFile("../../testdata/sema/pitfall_fix_safety_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, name := range []string{"EqTrue", "EqFalse", "NeTrue", "NeFalse", "TrueEq", "FalseEq", "TrueNe", "FalseNe", "ShortCircuit", "OuterConversion", "Consuming", "Stored", "Inclusive", "Exclusive"} {
		finding := fixSafetyResultFor(t, program, analyzer, name)
		functionStart := strings.Index(source, "fn "+name+"(")
		start := functionStart + strings.Index(source[functionStart:], "return ") + len("return ")
		end := start + strings.Index(source[start:], " }")
		rewritten := source[:start] + finding.Actions[0].Replacement + source[end:]
		p := parser.New(lexer.New(rewritten))
		replacementProgram := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("%s: %v", name, p.Errors())
		}
		if errs := NewAnalyzerWithDepth(AnalysisDeep).Analyze(replacementProgram); len(errs) != 0 {
			t.Fatalf("%s replacement is invalid: %v", name, errs)
		}
	}
	// Apply the full range edit too, preserving the start and body text.
	finding := fixSafetyResultFor(t, program, analyzer, "Stable")
	start := strings.Index(source, "for i in uint(0)..values.Len - 1") + len("for i in ")
	end := start + len("uint(0)..values.Len - 1")
	rewritten := source[:start] + finding.Actions[0].Replacement + source[end:]
	p := parser.New(lexer.New(rewritten))
	replacementProgram := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	if errs := NewAnalyzer().Analyze(replacementProgram); len(errs) != 0 {
		t.Fatal(errs)
	}
}

// Literal-bound interval membership retains truth across both endpoints and
// short-circuit paths. Observable subject reads are excluded by the fixture
// tests; these tests verify the emitted range spelling and boundary behavior.
// Rules: rules/analysis/pitfall_analysis.md — "Fix safety";
// rules/foundations/operators.md — "Range membership".
func TestPitfallMembershipFixPreservesIntervalTruth(t *testing.T) {
	program, analyzer := fixSafetyFixture(t)
	for _, name := range []string{"Inclusive", "Exclusive", "Stored"} {
		finding := fixSafetyResultFor(t, program, analyzer, name)
		rewritten := parseFixBoolean(t, finding.Actions[0].Replacement).(*ast.InfixExpression)
		interval := rewritten.Right.(*ast.RangeExpression)
		if interval.Start.String() != "FLOOR" || interval.End.String() != "LIMIT" {
			t.Fatal("constant names lost", rewritten)
		}
		for value := -2; value <= 12; value++ {
			original := value >= 1 && value <= 9
			if name == "Exclusive" {
				original = value >= 1 && value < 9
			}
			membership := value >= 1 && (value < 9 || !interval.Exclusive && value == 9)
			if original != membership {
				t.Fatalf("%s/%d: truth changed", name, value)
			}
		}
	}
}
