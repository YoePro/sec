package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// A variant test on an immutable binding whose active variant is proven — by
// construction from a variant constructor or by a dominating test of the
// path — is decided, and the excluded branch is S3001. Mutable bindings are
// never decided.
//
// Rules:
//   - rules/declarations/unions.md — §8.1 "Active variant test", §8.4 "Impossible state tests"
//   - rules/tooling/diagnostics.md — § 21 "Proven unreachable and dead code"
func TestKnownActiveVariantDecidesStateTests(t *testing.T) {
	const prelude = `module main

type Figure union {
    Dot,
    Line(int),
}

fn Make(flag: bool) Figure {
    if flag {
        return Figure.Dot
    }
    return Figure.Line(1)
}
`
	tests := []struct {
		name     string
		body     string
		want     int
		wantHelp string
	}{
		{name: "constructed other variant", want: 1, wantHelp: "always false", body: `
fn F() int {
    let figure := Figure.Dot
    if figure is Figure.Line {
        return 1
    }
    return 0
}`},
		{name: "constructed payload variant leaves else impossible", want: 1, wantHelp: "always true", body: `
fn F() int {
    let figure := Figure.Line(3)
    if figure is Line {
        return 1
    } else {
        return 2
    }
}`},
		{name: "constructed result", want: 1, wantHelp: "always false", body: `
fn F() int {
    let result: Result[int, error] := Ok(1)
    if result is Err {
        return 1
    }
    return 0
}`},
		{name: "nested contradicting test", want: 1, wantHelp: "always false", body: `
fn F(flag: bool) int {
    let figure := Make(flag)
    if figure is Figure.Dot {
        if figure is Figure.Line {
            return 1
        }
    }
    return 0
}`},
		{name: "else branch excludes the tested variant", want: 1, wantHelp: "always false", body: `
fn F(flag: bool) int {
    let figure := Make(flag)
    if figure is Figure.Dot {
        return 1
    } else {
        if figure is Figure.Dot {
            return 2
        }
    }
    return 0
}`},
		{name: "after exit", want: 1, wantHelp: "always false", body: `
fn F(flag: bool) int {
    let figure := Make(flag)
    if figure is Figure.Dot {
        return 1
    }
    if figure is Figure.Dot {
        return 2
    }
    return 0
}`},
		{name: "unknown variant stays runtime", body: `
fn F(flag: bool) int {
    let figure := Make(flag)
    if figure is Figure.Dot {
        return 1
    }
    return 0
}`},
		{name: "mutable binding stays runtime", body: `
fn F(flag: bool) int {
    let mut figure := Figure.Dot
    figure = Make(flag)
    if figure is Figure.Line {
        return 1
    }
    return 0
}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program := parser.New(lexer.New(prelude + test.body + "\n")).ParseProgram()
			analyzer := NewAnalyzer()
			errors := analyzer.Analyze(program)
			unreachable := 0
			for _, err := range errors {
				if err.ID != diagnostics.UnreachableStatement {
					t.Fatalf("unexpected error: %v", err)
				}
				unreachable++
				if test.wantHelp != "" && !strings.Contains(err.Help, test.wantHelp) {
					t.Fatalf("help = %q, want %q", err.Help, test.wantHelp)
				}
			}
			if unreachable != test.want {
				t.Fatalf("S3001 count = %d, want %d: %v", unreachable, test.want, errors)
			}
		})
	}
}
