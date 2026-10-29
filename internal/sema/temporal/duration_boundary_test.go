package temporal_test

import (
	"math/big"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// TestDurationCallBoundariesHaveNoUnitSpellingShortcut verifies that the same
// canonical duration parameter behaves identically for mutex-like, context-like
// and ordinary names, including user-defined member calls. Familiar unit names
// with a non-time dimension must never become a hidden timeout overload.
// Rules: rules/concurrency/mutex.md — §12(2)-(6);
// rules/types/temporal.md — §1 (nominal temporal identities);
// rules/types/units.md — Dimensions;
// rules/types/temporal.md — §5.5(2): already typed quantities need a written
// duration conversion even when their values are provably representable.
func TestDurationCallBoundariesHaveNoUnitSpellingShortcut(t *testing.T) {
	template := fixture(t, "duration_call_boundary.sec.in")
	for _, symbol := range []string{"s", "ms", "us", "ns", "beat", "arbitraryTick"} {
		for _, axis := range []string{"time", "length"} {
			for _, scale := range []string{"1", "1 / 1000"} {
				for _, kind := range []string{"duration", "quantity", "plain_integer"} {
					t.Run(symbol+"/"+axis+"/"+scale+"/"+kind, func(t *testing.T) {
						typ := "int<" + symbol + ">"
						if kind == "duration" {
							typ = "duration"
						}
						if kind == "plain_integer" {
							typ = "int"
						}
						source := strings.NewReplacer("__UNIT__", symbol, "__AXIS__", axis, "__SCALE__", scale, "__TYPE__", typ).Replace(template)
						parsed := parser.New(lexer.NewWithFile(source, "duration_boundary.sec")).Parse()
						if parsed.HasErrors {
							t.Fatal(parsed.Diagnostics)
						}
						analyzer := sema.NewAnalyzer()
						errors := analyzer.Analyze(parsed.Program)
						if kind == "duration" {
							if len(errors) > 0 {
								t.Fatal(errors)
							}
							return
						}
						if len(errors) != 4 {
							t.Fatalf("want four nominal argument errors, got %+v", errors)
						}
						for _, err := range errors {
							if !strings.Contains(err.Message, "must be duration") || err.File != "duration_boundary.sec" {
								t.Fatalf("unexpected rejection: %+v", err)
							}
						}
						// Prove that all metadata was accepted before checking the call boundary;
						// a parser/unit-metadata failure must not masquerade as timeout rejection.
						parameter := analyzer.Functions()["Check"][0].Parameters[1].Type
						if kind == "quantity" {
							expectedScale, ok := new(big.Rat).SetString(strings.ReplaceAll(scale, " ", ""))
							if !ok || parameter.Dimension.Base[axis] != 1 || parameter.UnitSemantics.Scale == nil || parameter.UnitSemantics.Scale.Cmp(expectedScale) != 0 {
								t.Fatalf("quantity facts missing: %+v", parameter)
							}
						}
					})
				}
			}
		}
	}
}

// TestCanonicalMutexCallsDoNotInventUnitOverloads checks the current intrinsic
// API: Lock(duration) accepts nominal duration and exact time literals, never
// already typed quantities by their spelling; TryLock takes no arguments.
// Basic acquisition signatures remain valid throughout the metadata matrix.
// Rules: rules/concurrency/mutex.md — §§3,12(1–6),20–24;
// rules/types/temporal.md — §5.5(1–3).
func TestCanonicalMutexCallsDoNotInventUnitOverloads(t *testing.T) {
	template := fixture(t, "duration_mutex_boundary.sec.in")
	for _, symbol := range []string{"s", "ms", "us", "ns", "beat", "arbitraryTick"} {
		for _, axis := range []string{"time", "length"} {
			for _, scale := range []string{"1", "1 / 1000"} {
				for _, kind := range []string{"duration", "quantity", "plain_integer"} {
					t.Run(symbol+"/"+axis+"/"+scale+"/"+kind, func(t *testing.T) {
						typ := "int<" + symbol + ">"
						if kind == "duration" {
							typ = "duration"
						}
						if kind == "plain_integer" {
							typ = "int"
						}
						source := strings.NewReplacer("__UNIT__", symbol, "__AXIS__", axis, "__SCALE__", scale, "__TYPE__", typ).Replace(template)
						parsed := parser.New(lexer.NewWithFile(source, "mutex_boundary.sec")).Parse()
						if parsed.HasErrors {
							t.Fatal(parsed.Diagnostics)
						}
						analyzer := sema.NewAnalyzer()
						errors := analyzer.Analyze(parsed.Program)
						want := 1 // TryLock(value) is always invalid.
						if kind != "duration" {
							want++
						}
						if axis != "time" {
							want++
						}
						if len(errors) != want {
							t.Fatalf("want %d call-boundary errors, got %v", want, errors)
						}
						lockErrors, tryLockErrors, literalErrors := 0, 0, 0
						for _, err := range errors {
							if err.File != "mutex_boundary.sec" {
								t.Fatal(err)
							}
							switch {
							case err.Message == "Mutex.TryLock expects 0 arguments, got 1":
								tryLockErrors++
							case strings.Contains(err.Message, "does not have the dimension of time"):
								literalErrors++
							case strings.Contains(err.Message, "Mutex.Lock argument must be duration or ref Context"):
								lockErrors++
								if kind == "quantity" && !strings.Contains(err.Message, "already typed quantity") {
									t.Fatal("lost nominal-quantity explanation", err)
								}
							default:
								t.Fatal("unrelated error masked the intended boundary", err)
							}
						}
						if tryLockErrors != 1 || (lockErrors == 0) != (kind == "duration") || (literalErrors == 0) != (axis == "time") {
							t.Fatal("wrong boundary classification", errors)
						}
						var literalArgument ast.Expression
						for _, statement := range parsed.Program.Statements {
							if fn, ok := statement.(*ast.FunctionDeclaration); ok && fn.Name.Value == "Literal" {
								returned := fn.Body.Statements[0].(*ast.ReturnStatement)
								literalArgument = returned.Value.(*ast.CallExpression).Arguments[0]
							}
						}
						plan, converted := analyzer.DurationConversionPlanOf(literalArgument)
						if converted != (axis == "time") {
							t.Fatalf("wrong literal conversion eligibility: %+v", plan)
						}
						if converted {
							wantNanoseconds := "5000000000"
							if scale == "1 / 1000" {
								wantNanoseconds = "5000000"
							}
							if plan.Explicit || plan.Proof != sema.DurationConversionConstant || plan.Nanoseconds == nil || plan.Nanoseconds.String() != wantNanoseconds {
								t.Fatalf("literal conversion ignored the resolved unit scale: %+v", plan)
							}
						}
					})
				}
			}
		}
	}
}
