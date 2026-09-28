package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

func analyzeTemporalCoreSource(t *testing.T, source string) (*Analyzer, *ast.Program, []Error) {
	t.Helper()
	const sourceFile = "sec/core/temporal_now_test.sec"
	parsed := parser.New(lexer.NewWithFile(source, sourceFile)).Parse()
	if parsed.HasErrors {
		t.Fatalf("parser errors: %+v", parsed.Diagnostics)
	}
	parsed.Program.SourceProvenance = map[string]ast.SourceProvenance{sourceFile: ast.SourceCore}
	analyzer := NewAnalyzer()
	return analyzer, parsed.Program, analyzer.Analyze(parsed.Program)
}

// Rules:
//   - rules/compiler/compiler_known_members.md — "Private core UTC wall-clock intrinsic"
//   - rules/types/temporal.md — §3 "UTC wall-clock access"
func TestCompilerKnownNowIsTypedAndEffectfulInTrustedCore(t *testing.T) {
	analyzer, program, errors := analyzeTemporalCoreSource(t, `module core

fn ReadTwice() datetime {
    let first: datetime := _now
    return _now
}
`)
	assertSemaErrors(t, errors, nil)

	function, ok := program.Statements[1].(*ast.FunctionDeclaration)
	if !ok || len(function.Body.Statements) != 2 {
		t.Fatalf("unexpected program shape: %#v", program.Statements)
	}
	first := function.Body.Statements[0].(*ast.LetStatement).Value.(*ast.Identifier)
	second := function.Body.Statements[1].(*ast.ReturnStatement).Value.(*ast.Identifier)
	for _, identifier := range []*ast.Identifier{first, second} {
		known, found := analyzer.CompilerKnownValueAt(identifier.Token.File, identifier.Token.Line, identifier.Token.Column)
		if !found || known.ID != "CKV-TEMPORAL-NOW" || known.Result.Name != "datetime" ||
			known.RequiredCapability != "UTCWallClock" || len(known.Effects) != 1 ||
			known.Effects[0] != EffectMayUseNondeterministicInput {
			t.Fatalf("_now fact = %+v, found %v", known, found)
		}
	}

	summary := analyzer.CallGraph().EffectSummary(callGraphNodeIDByName(t, analyzer.CallGraph(), "ReadTwice"))
	if len(summary.DirectEffects) != 2 {
		t.Fatalf("_now effects = %+v, want two distinct reads", summary.DirectEffects)
	}
	for _, effect := range summary.DirectEffects {
		if effect.Kind != EffectMayUseNondeterministicInput || effect.Source.Lexeme != "_now" {
			t.Fatalf("unexpected _now effect: %+v", effect)
		}
	}
}

func TestTemporalCorePropertiesProjectOneNowRead(t *testing.T) {
	_, _, errors := analyzeTemporalCoreSource(t, `module core

type date struct { _epochDays: int32, }
type time struct { _nanosecondsSinceMidnight: uint64, }
type datetime struct { _epochDays: int32, _nanosecondsSinceMidnight: uint64, }

impl datetime {
    static property Now: datetime {
        get { return _now }
    }
}

impl date {
    static property Today: date {
        get {
            let now: datetime := _now
            return date { _epochDays: now._epochDays }
        }
    }
}

impl time {
    static property Now: time {
        get {
            let now: datetime := _now
            return time { _nanosecondsSinceMidnight: now._nanosecondsSinceMidnight }
        }
    }
}
`)
	assertSemaErrors(t, errors, nil)
}

func TestCompilerKnownNowRejectsOrdinarySource(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn Read() datetime {
    return _now
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "_now is a compiler-internal value available only to loader-proven core source") {
		t.Fatalf("errors = %+v", errors)
	}
}

func TestCompilerKnownNowRejectsStaticInitialization(t *testing.T) {
	_, _, errors := analyzeTemporalCoreSource(t, `module core

let Snapshot: datetime := _now
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "static initializer for Snapshot must be compile-time evaluable") {
		t.Fatalf("errors = %+v", errors)
	}
}

func TestCompilerKnownNowCannotBeDeclared(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main

fn _now() datetime {
    panic "reserved"
}
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "function _now is compiler-known and cannot be declared") {
		t.Fatalf("errors = %+v", errors)
	}
}
