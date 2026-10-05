package sema

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// The entry contract of command and firmware Targets: exactly one concrete,
// non-generic, module-level main with no parameters returning int (command)
// or void (firmware) in the entry module. Library and test Targets are not
// checked.
//
// Rules:
//   - rules/compiler/initialization.md — § 21, § 22, § 25, § 26, § 27, § 45.1 "Entry validation"
func TestProgramEntryContract(t *testing.T) {
	tests := []struct {
		name   string
		kind   ProgramTargetKind
		source string
		want   []string
	}{
		{name: "valid command", kind: ProgramTargetCommand, source: "fn main() int {\n    return 0\n}"},
		{name: "missing command entry", kind: ProgramTargetCommand, source: "fn Run() int {\n    return 0\n}",
			want: []string{diagnostics.ProgramEntryMissing + ": command target has no entry: module main declares no `fn main() int`"}},
		{name: "wrong command return type", kind: ProgramTargetCommand, source: "fn main() void {\n}",
			want: []string{diagnostics.ProgramEntryInvalid + ": command entry must be `fn main() int`; this main returns void"}},
		{name: "command entry with parameters", kind: ProgramTargetCommand, source: "fn main(args: string[]) int {\n    return 0\n}",
			want: []string{diagnostics.ProgramEntryInvalid + ": command entry must be `fn main() int`; this main takes parameters"}},
		{name: "result-returning command entry", kind: ProgramTargetCommand, source: "type Failure enum error {\n    Bad\n}\n\nfn main() Result[int, Failure] {\n    return Ok(0)\n}",
			want: []string{diagnostics.ProgramEntryInvalid + ": command entry must be `fn main() int`; this main returns Result[int, Failure]"}},
		{name: "generic entry", kind: ProgramTargetCommand, source: "fn main[T]() int {\n    return 0\n}",
			want: []string{diagnostics.ProgramEntryInvalid + ": command entry must be `fn main() int`; this main is generic"}},
		{name: "overloaded main", kind: ProgramTargetCommand, source: "fn main() int {\n    return 0\n}\n\nfn main(value: int) int {\n    return value\n}",
			want: []string{diagnostics.ProgramEntryDuplicate + ": duplicate entry main in module main"}},
		{name: "valid firmware", kind: ProgramTargetFirmware, source: "fn main() void {\n}"},
		{name: "invalid firmware entry", kind: ProgramTargetFirmware, source: "fn main() int {\n    return 0\n}",
			want: []string{diagnostics.ProgramEntryInvalid + ": firmware entry must be `fn main() void`; this main returns int"}},
		{name: "library without entry", kind: ProgramTargetLibrary, source: "fn Helper() int {\n    return 1\n}"},
		{name: "library may name a function main", kind: ProgramTargetLibrary, source: "fn main(value: int) int {\n    return value\n}"},
		{name: "test target uses the generated harness", kind: ProgramTargetTest, source: "fn Helper() int {\n    return 1\n}"},
		{name: "method named main is not the entry", kind: ProgramTargetCommand, source: "type Tool struct {\n    id: int,\n}\n\nimpl Tool {\n    fn main() int {\n        return id\n    }\n}\n\nfn main() int {\n    return 0\n}"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := parser.New(lexer.NewWithFile("module main\n\n"+test.source+"\n", "entry.sec")).Parse()
			if len(result.Diagnostics) != 0 {
				t.Fatalf("parser diagnostics = %+v", result.Diagnostics)
			}
			analyzer := NewAnalyzer()
			if errors := analyzer.Analyze(result.Program); len(errors) != 0 {
				t.Fatalf("analysis errors = %v", errors)
			}
			anchor := lexer.Token{File: "entry.sec", Line: 1, Column: 1}
			got := []string{}
			for _, err := range analyzer.ValidateProgramEntry(test.kind, "main", anchor) {
				if err.Line == 0 || err.Help == "" {
					t.Errorf("entry diagnostic lacks a location or help: %+v", err)
				}
				got = append(got, err.ID+": "+err.Message)
			}
			if len(got) != len(test.want) {
				t.Fatalf("entry diagnostics = %q, want %q", got, test.want)
			}
			for index := range got {
				if !strings.HasPrefix(got[index], test.want[index]) {
					t.Errorf("entry diagnostic %d = %q, want prefix %q", index, got[index], test.want[index])
				}
			}
		})
	}
}
