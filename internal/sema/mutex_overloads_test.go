package sema

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestMutexAcquisitionOverloads checks the complete source signature matrix,
// exact result identities and contextual time conversion through ordinary Sema.
// Rules: rules/concurrency/mutex.md — §§11–12,20–24;
// rules/types/temporal.md — §5.5.
func TestMutexAcquisitionOverloads(t *testing.T) {
	for _, file := range []string{"mutex_overloads_valid.sec", "mutex_overloads_invalid.sec"} {
		data, err := os.ReadFile("../../testdata/sema/" + file)
		if err != nil {
			t.Fatal(err)
		}
		p := parser.New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if file == "mutex_overloads_invalid.sec" {
			if len(errors) != 11 {
				t.Fatal(errors)
			}
			for _, want := range []string{"got int", "got bool", "got ref Other", "already typed quantity", "dimension of time", "not a whole number", "no timeout/context combination", "does not take type arguments", "TryLock expects 0 arguments", "mutably borrowed", "does not satisfy @noBlock"} {
				found := false
				for _, diagnostic := range errors {
					found = found || strings.Contains(diagnostic.Message, want)
				}
				if !found {
					t.Fatal("missing", want, errors)
				}
			}
			continue
		}
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		want := map[string]string{"Basic": "MutexGuard[int]", "Probe": "Option[MutexGuard[int]]", "Timed": "Option[MutexGuard[int]]", "Literal": "Option[MutexGuard[int]]", "Contextual": "Result[MutexGuard[int], ContextError]", "MutableContext": "Result[MutexGuard[int], ContextError]"}
		for _, statement := range program.Statements {
			fn, ok := statement.(*ast.FunctionDeclaration)
			if !ok || want[fn.Name.Value] == "" {
				continue
			}
			value := fn.Body.Statements[0].(*ast.ReturnStatement).Value
			typ, ok := a.ResolvedTypeOf(value)
			if !ok || typeDisplayName(typ) != want[fn.Name.Value] {
				t.Fatal(fn.Name.Value, typ, ok)
			}
			call := value.(*ast.CallExpression)
			member := call.Callee.(*ast.MemberExpression)
			fact, ok := a.CompilerKnownMemberAt(member.Property.Token.File, member.Property.Token.Line, member.Property.Token.Column)
			if !ok || !sameConcreteType(fact.Result, typ) {
				t.Fatal(fn.Name.Value, fact, ok)
			}
			if fn.Name.Value == "Literal" {
				plan, ok := a.DurationConversionPlanOf(call.Arguments[0])
				if !ok || plan.Nanoseconds == nil || plan.Nanoseconds.String() != "250000000" {
					t.Fatal(plan, ok)
				}
			}
			delete(want, fn.Name.Value)
		}
		if len(want) != 0 {
			t.Fatal("missing functions", want)
		}
		if _, exists := a.Types()["LockError"]; exists {
			t.Fatal("invented LockError")
		}
	}
}

// TestMutexContextCoreIdentity validates the new overload dependencies against
// actual core source, including opaque ownership and the closed error family.
// Rules: rules/concurrency/mutex.md — §§4,7–8,24.
func TestMutexContextCoreIdentity(t *testing.T) {
	const file = "../../sec/core/context.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	context := a.Types()["Context"]
	if !sameConcreteType(context, builtinType("Context")) || !context.ExplicitlyNonCopyable || CopyClassificationOf(context) != CopyMoveOnly || TriviallyDestructible(context) || len(context.Fields) != 0 {
		t.Fatal(context)
	}
	errorType := a.Types()["ContextError"]
	if !sameConcreteType(errorType, builtinType("ContextError")) || !errorType.ErrorAssignable || !reflect.DeepEqual(errorType.EnumValues, []string{"Cancelled", "DeadlineExceeded"}) {
		t.Fatal(errorType)
	}
	p = parser.New(lexer.NewWithFile(string(data), file))
	if errors := NewAnalyzer().Analyze(p.ParseProgram()); len(errors) != 2 {
		t.Fatal(errors)
	}
}
