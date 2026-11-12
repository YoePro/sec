package sema

import (
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"

	"sec/internal/diagnostics"
)

// Executed concurrency, atomic, barrier and runtime I/O operations are
// forbidden effects of semantic CTE, classified from Sema facts and reported
// at the operation with the executed call chain; the same functions remain
// valid runtime code.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 22, 47(3).
func TestSemanticCTEForbiddenConcurrencyAndAtomicEffects(t *testing.T) {
	source := `module main

static let Ready: Atomic[bool] := Atomic(false)
static let Shared: Mutex[int] := Mutex(1)

fn Work() int {
    return 1
}

fn Spawned() int {
    let job := try spawn task Work() { Err(_) => { return 0 } }
    await job
    return 1
}

fn Barrier() int {
    atomic.Fence(MemoryOrder.SeqCst)
    return 1
}

fn Flag() int {
    if Ready.Load() {
        return 1
    }
    return 2
}

fn Locked() int {
    let guard := Shared.Lock()
    return 1
}

type A int range 0..Spawned()
type B int range 0..Barrier()
type C int range 0..Flag()
type D int range 0..Locked()
`
	errors := analyzeSourceRaw(t, source)
	want := []string{
		"spawn starts concurrent execution, which semantic CTE never performs",
		"atomic.Fence(MemoryOrder.SeqCst) is a memory barrier",
		"Ready.Load() is an atomic operation",
		"Shared.Lock() is a concurrency operation",
	}
	got := []Error{}
	for _, err := range errors {
		if err.ID == diagnostics.CompileTimeForbiddenEffect {
			got = append(got, err)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("errors = %v", errors)
	}
	for index, text := range want {
		if !strings.Contains(got[index].Message, text) || got[index].PreviousLine == 0 {
			t.Errorf("error %d = %q (stopped at %d), want %q", index, got[index].Message, got[index].PreviousLine, text)
		}
	}
}

// A successful required evaluation is a typed Sec value: the canonical
// TypeIdentity (module-qualified nominal identity, never the display
// spelling) plus the SemanticValue, for scalars, named types and structs.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 11, 31(3), 73.
func TestCompileTimeValueIsTyped(t *testing.T) {
	source := `module main

type Point struct {
    X: int,
    Y: int,
}

type Port uint16

fn Origin() Point {
    return Point { X: 10, Y: 20 }
}

fn Base() Port {
    return Port(8000)
}

let Home: Point := Origin()
let Listen: Port := Base()
let Count: uint32 := 42
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	got := map[string]CompileTimeValue{}
	roots := map[string]ast.Expression{}
	for _, static := range analyzer.ResolvedStaticDeclarations() {
		if static.Declaration == nil || static.Declaration.Value == nil {
			continue
		}
		if value, ok := analyzer.CompileTimeValueOf(static.Declaration.Value); ok {
			got[static.Name] = value
			roots[static.Name] = static.Declaration.Value
		}
	}
	home, ok := got["Home"]
	if !ok || home.TypeIdentity != "main:struct:Point" || renderCompileTimeValue(home.Value) != "Point { X: 10, Y: 20 }" {
		t.Errorf("Home = %+v (%v)", home, ok)
	}
	listen, ok := got["Listen"]
	if !ok || !strings.HasPrefix(listen.TypeIdentity, "main:") || !strings.Contains(listen.TypeIdentity, "Port") || renderCompileTimeValue(listen.Value) != "8000" {
		t.Errorf("Listen = %+v (%v)", listen, ok)
	}
	if _, literal := got["Count"]; literal {
		t.Error("a literal initializer is not a semantic CTE request")
	}
	start := expressionStartToken(roots["Home"])
	fact, ok := analyzer.CompileTimeValueAt(start.File, start.Line, start.Column)
	if !ok || fact.Type != "Point" || fact.Value != "Point { X: 10, Y: 20 }" {
		t.Errorf("tooling fact = %+v (%v)", fact, ok)
	}
}

// Every required evaluation records conservative dependencies: executed
// declarations, generic concrete instances, immutable values, nominal type
// facts and the CompilationPlan's platform scalar width. A memoized call
// keeps the dependencies of its execution, so a second root reusing it
// lists the same transitive inputs.
// Rules: rules/compiler/compile_time_evaluation.md — §§ 58, 60(2)–(3).
func TestSemanticCTEDependencies(t *testing.T) {
	source := `module main

type Point struct {
    X: int,
    Y: int,
}

fn Pick[T](a: T, b: T) T {
    return a
}

fn Twice(n: int) int {
    return n * 2
}

let Base: int := 40

fn Limit() int {
    let p := Point { X: 1, Y: 2 }
    return Pick(Base, 0) + p.X + Twice(1)
}

let First: int := Limit()
let Second: int := Limit()
`
	analyzer, errors := analyzeSourceWithAnalyzerRaw(t, source)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	kinds := func(name string) map[CompileTimeDependencyKind][]string {
		for _, static := range analyzer.ResolvedStaticDeclarations() {
			if static.Name != name {
				continue
			}
			dependencies, ok := analyzer.CompileTimeDependencies(static.Declaration.Value)
			if !ok {
				t.Fatalf("%s has no dependencies", name)
			}
			out := map[CompileTimeDependencyKind][]string{}
			for _, dependency := range dependencies {
				out[dependency.Kind] = append(out[dependency.Kind], dependency.Identity)
			}
			return out
		}
		t.Fatalf("%s not found", name)
		return nil
	}
	first := kinds("First")
	if len(first[CTEDependsOnDeclaration]) != 3 ||
		!reflect.DeepEqual(first[CTEDependsOnGenericInstance], []string{"main.Pick[T = int]"}) ||
		!reflect.DeepEqual(first[CTEDependsOnImmutableValue], []string{"main.Base"}) ||
		!reflect.DeepEqual(first[CTEDependsOnTypeFact], []string{"main:struct:Point"}) ||
		len(first[CTEDependsOnCompilationPlan]) != 1 || len(first[CTEDependsOnImportedModule]) != 0 {
		t.Errorf("First dependencies = %v", first)
	}
	if second := kinds("Second"); !reflect.DeepEqual(second, first) {
		t.Errorf("memoized Limit() lost dependencies:\n%v\nvs\n%v", second, first)
	}
}
