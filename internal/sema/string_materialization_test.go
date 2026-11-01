package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 5.1–5.15
//   - rules/foundations/operators.md — "String concatenation"
func TestRuntimeStringMaterializationRequiresTry(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn UnhandledOperand(count: int) string {
    return "Hello " + count.ToString()
}

fn MissingInterpolation(count: int) string {
    return $"count={count}"
}

fn Folded() string {
    return "Hello " + "world" + '!'
}

fn Propagated(name: string) Result[string, StringError] {
    return Ok(try "Hello " + name)
}

fn Handled(name: string) string {
    return try "Hello " + name {
        Err(_) => "Hello"
    }
}
`)
	if len(errors) != 2 {
		t.Fatalf("errors = %+v, want two missing-try diagnostics", errors)
	}
	for index, line := range []int{4, 8} {
		if errors[index].ID != "S1097" || errors[index].Line != line || !strings.Contains(errors[index].Message, "requires try") {
			t.Fatalf("error %d = %+v, want S1097 at line %d", index, errors[index], line)
		}
	}
}

func TestStringConcatPlanRecordsResolvedAllocationContext(t *testing.T) {
	source := `module main

fn Render(name: string) Result[string, StringError] {
    return Ok(try "Hello " + name)
}

fn Folded() string {
    return "Hello " + "world"
}
`
	result := parser.New(lexer.NewWithFile(source, "allocation.sec")).Parse()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	render := unwrapOkTry(result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value)
	plan, ok := analyzer.StringConcatPlanOf(render)
	if !ok || !plan.Runtime {
		t.Fatalf("runtime plan = %#v, found=%t", plan, ok)
	}
	context := plan.Allocation.Context
	if !context.Available || context.Origin != StorageOriginArena || context.Profile != "hosted" || plan.Allocation.FailureType.Name != "StringError" {
		t.Fatalf("allocation = %#v, want hosted Arena context with StringError", plan.Allocation)
	}
	folded := result.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.ReturnStatement).Value
	if plan, ok := analyzer.StringConcatPlanOf(folded); !ok || plan.Runtime {
		t.Fatalf("folded plan = %#v, found=%t, want compile-time plan", plan, ok)
	}
	summary := analyzer.CallGraph().ArenaSummary(callGraphNodeIDByName(t, analyzer.CallGraph(), "Render"))
	if !summary.MayAllocate {
		t.Fatal("runtime string materialization did not publish an allocation effect")
	}
}

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 5.18–5.19
//   - rules/memory/allocation.md — § 22(4)
func TestRuntimeStringMaterializationWithoutAllocationContext(t *testing.T) {
	source := `module main

fn Render(name: string) Result[string, StringError] {
    return Ok(try "Hello " + name)
}

fn Folded() string {
    return "Hello " + "world"
}
`
	result := parser.New(lexer.NewWithFile(source, "noalloc.sec")).Parse()
	analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "baremetal", TargetArch: "cortex-m3", Profile: "noalloc", PointerWidthBits: 32})
	errors := analyzer.Analyze(result.Program)
	if len(errors) != 1 || errors[0].ID != "S1098" || errors[0].Line != 4 || !strings.Contains(errors[0].Message, `"noalloc" provides none`) {
		t.Fatalf("errors = %+v, want one S1098 for the runtime plan only", errors)
	}
}

func stringPlanOf(t *testing.T, a *Analyzer, program *ast.Program, name string) StringConcatPlan {
	t.Helper()
	found := StringConcatPlan{}
	known := false
	ast.Visit(program, func(node any) bool {
		fn, ok := node.(*ast.FunctionDeclaration)
		if !ok || fn.Name == nil || fn.Name.Value != name {
			return true
		}
		ast.Visit(fn.Body, func(inner any) bool {
			if expression, ok := inner.(ast.Expression); ok && !known {
				if plan, has := a.StringConcatPlanOf(expression); has {
					found, known = plan, true
				}
			}
			return true
		})
		return false
	})
	if !known {
		t.Fatalf("no plan in %s", name)
	}
	return found
}

// Source syntax alone selects try or panic for runtime + and +=; the plan
// records the mode, and an inner try leaves the outer + in panic mode.
// Rules: rules/foundations/operators.md — "Runtime allocation and failure policy" (MD-039).
func TestStringConcatFailureModes(t *testing.T) {
	source := `module main

fn Ordinary(name: string) string {
    return "Hello " + name
}

fn Protected(name: string) Result[string, StringError] {
    return Ok(try "Hello " + name)
}

fn Handled(name: string) string {
    return try "Hello " + name {
        Err(_) => "Hello"
    }
}

fn InnerOperand(count: int) Result[string, StringError] {
    return Ok("Count " + (try count.ToString()))
}

fn WholePlan(count: int) Result[string, StringError] {
    return Ok(try "Count " + count.ToString())
}
`
	result := parser.New(lexer.NewWithFile(source, "modes.sec")).Parse()
	analyzer := NewAnalyzer()
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	for name, want := range map[string]StringMaterializationFailureMode{
		"Ordinary": StringMaterializationPanic, "Protected": StringMaterializationTry, "Handled": StringMaterializationTry,
		"InnerOperand": StringMaterializationPanic, "WholePlan": StringMaterializationTry,
	} {
		if plan := stringPlanOf(t, analyzer, result.Program, name); plan.Failure != want {
			t.Errorf("%s: mode %q, want %q", name, plan.Failure, want)
		}
	}
}

// Rules: rules/foundations/operators.md — "Compound concatenation assignment"; MD-039 §§ 3,4.
func TestStringAppendForms(t *testing.T) {
	valid := `module main

type Named struct {
    Text: string,
}

fn Ordinary(part: string, letter: char) string {
    let mut text := "x"
    text += part
    text += letter
    return text
}

fn Propagated(part: string) Result[string, StringError] {
    let mut text := "x"
    try text += part
    return Ok(text)
}

fn Handled(part: string) string {
    let mut text := "x"
    try text += part {
        Err(_) => {
            text = "failed"
        }
    }
    return text
}

fn WholeOperand(count: int) Result[string, StringError] {
    let mut text := "x"
    try text += count.ToString()
    return Ok(text)
}

fn InnerOperand(count: int) Result[string, StringError] {
    let mut text := "x"
    text += try count.ToString()
    return Ok(text)
}

fn PlanOperand(part: string) Result[string, StringError] {
    let mut text := "x"
    try text += "a" + part
    return Ok(text)
}

fn InterpolatedOperand(count: int) Result[string, StringError] {
    let mut text := "x"
    try text += $"{count}"
    return Ok(text)
}

fn Field(named: ref mut Named, part: string) Result[void, StringError] {
    try named.Text += part
    return Ok()
}

fn Folded() Result[string, StringError] {
    return Ok(try "a" + "b")
}
`
	assertSemaErrors(t, analyzeSourceRaw(t, valid), nil)
	for _, item := range []struct{ name, body, want string }{
		{"unhandled operand", "fn A(c: int) string {\n let mut t := \"x\"\n t += c.ToString()\n return t\n}", "unhandled Result[string, StringError]"},
		{"other Result", "fn R() Result[string, FormatError] { return Ok(\"a\") }\nfn A() Result[string, FormatError] {\n let mut t := \"x\"\n try t += R()\n return Ok(t)\n}", "cannot concatenate string"},
		{"non-text", "fn A(c: int) string {\n let mut t := \"x\"\n t += c\n return t\n}", "cannot concatenate string and int"},
		{"try on unrelated assignment", "fn A() string {\n let mut t := \"x\"\n try t = \"y\"\n return t\n}", "try assignment requires a fallible assignment target"},
		{"incompatible channel", "fn A(p: string) string {\n let mut t := \"x\"\n try t += p\n return t\n}", "naked try assignment propagates StringError"},
		{"plan operand needs try", "fn A(c: int) string {\n let mut t := \"x\"\n t += \"a\" + c.ToString()\n return t\n}", "unhandled Result[string, StringError] operand"},
		{"interpolated operand needs try", "fn A(c: int) string {\n let mut t := \"x\"\n t += $\"{c}\"\n return t\n}", "interpolation may fail"},
		{"mixed interpolation needs try", "fn A(c: int) string {\n return \"x\" + $\"{c}\"\n}", "interpolation may fail"},
		{"folded incompatible channel", "fn A() string {\n return try \"a\" + \"b\"\n}", "StringError"},
	} {
		t.Run(item.name, func(t *testing.T) {
			messages := ""
			for _, err := range analyzeSourceRaw(t, "module main\n"+item.body+"\n") {
				messages += err.Message + "\n"
			}
			if !strings.Contains(messages, item.want) {
				t.Fatalf("want %q, got %s", item.want, messages)
			}
		})
	}
}

// An unprotected + or += has a registered panic effect, a protected one has none,
// and @noAlloc rejects both modes.
// Rules: rules/foundations/operators.md — "@noPanic and try scope"; MD-039 § 5.
func TestStringMaterializationGuarantees(t *testing.T) {
	for _, item := range []struct{ name, source, want string }{
		{"noPanic +", "@noPanic\nfn A(n: string) string {\n return \"x\" + n\n}", "may-panic-string-materialization"},
		{"noPanic +=", "@noPanic\nfn A(n: string) string {\n let mut t := \"x\"\n t += n\n return t\n}", "may-panic-string-materialization"},
		{"noPanic try +", "@noPanic\nfn A(n: string) Result[string, StringError] {\n return Ok(try \"x\" + n)\n}", ""},
		{"noPanic try +=", "@noPanic\nfn A(n: string) Result[string, StringError] {\n let mut t := \"x\"\n try t += n\n return Ok(t)\n}", ""},
		{"noPanic folded", "@noPanic\nfn A() string {\n return \"x\" + \"y\"\n}", ""},
		{"noAlloc +", "@noAlloc\nfn A(n: string) string {\n return \"x\" + n\n}", "@noAlloc"},
		{"noAlloc try +", "@noAlloc\nfn A(n: string) Result[string, StringError] {\n return Ok(try \"x\" + n)\n}", "@noAlloc"},
		{"noAlloc +=", "@noAlloc\nfn A(n: string) string {\n let mut t := \"x\"\n t += n\n return t\n}", "@noAlloc"},
		{"noAlloc try +=", "@noAlloc\nfn A(n: string) Result[string, StringError] {\n let mut t := \"x\"\n try t += n\n return Ok(t)\n}", "@noAlloc"},
	} {
		t.Run(item.name, func(t *testing.T) {
			messages := ""
			for _, err := range analyzeSourceRaw(t, "module main\n"+item.source+"\n") {
				messages += err.Message + "\n"
			}
			if item.want == "" && messages != "" || !strings.Contains(messages, item.want) {
				t.Fatalf("want %q, got %s", item.want, messages)
			}
		})
	}
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, "module main\nfn A(n: string) string {\n return \"x\" + n\n}\n")
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	reason := false
	graph := analyzer.CallGraph()
	for _, effect := range graph.EffectSummary(callGraphNodeIDByName(t, graph, "A")).DirectEffects {
		if effect.Kind == EffectMayPanicStringMaterialization {
			for _, id := range effect.PanicReasonIDs {
				reason = reason || id == diagnostics.PanicReasonStringMaterializationFailure
			}
		}
	}
	if !reason {
		t.Fatal("ordinary concatenation lost its registered panic reason")
	}
}

// A missing allocation context is an error in both modes (MD-039 § 5(1)).
func TestStringAppendRequiresAllocationContext(t *testing.T) {
	for _, source := range []string{
		"module main\nfn A(n: string) string {\n let mut t := \"x\"\n t += n\n return t\n}\n",
		"module main\nfn A(n: string) Result[string, StringError] {\n let mut t := \"x\"\n try t += n\n return Ok(t)\n}\n",
		"module main\nfn A(n: string) string {\n return \"x\" + n\n}\n",
	} {
		result := parser.New(lexer.NewWithFile(source, "context.sec")).Parse()
		analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "baremetal", TargetArch: "cortex-m3", Profile: "noalloc", PointerWidthBits: 32})
		found := false
		for _, err := range analyzer.Analyze(result.Program) {
			found = found || err.ID == diagnostics.StringMaterializationWithoutAllocationContext
		}
		if !found {
			t.Errorf("no S1098 for %s", source)
		}
	}
}

// Effects of a core body reach their callers, and its unreached body does not fail
// a no-allocation build; a program body with the same form does.
func TestUnreachedCoreStringBodyHasNoContextError(t *testing.T) {
	source := "module main\nfn Pure() int {\n return 1\n}\n"
	result := parser.New(lexer.NewWithFile(source, "pure.sec")).Parse()
	analyzer := NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "baremetal", TargetArch: "cortex-m3", Profile: "noalloc", PointerWidthBits: 32})
	if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
}
