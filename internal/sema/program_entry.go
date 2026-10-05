package sema

import (
	"fmt"
	"sort"
	"strings"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// ProgramTargetKind is the kind of a project Target, which selects its entry
// contract.
//
// Rules:
//   - rules/compiler/initialization.md — § 20 "Target entry contracts"
//   - rules/projects/projects.md — Target kinds
type ProgramTargetKind string

const (
	ProgramTargetCommand  ProgramTargetKind = "command"
	ProgramTargetFirmware ProgramTargetKind = "firmware"
	ProgramTargetLibrary  ProgramTargetKind = "library"
	ProgramTargetTest     ProgramTargetKind = "test"
)

// ValidateProgramEntry checks the source entry of an analyzed program against
// its Target's entry contract: a command Target has exactly one concrete,
// non-generic, module-level `fn main() int` without parameters in its entry
// module, and a firmware Target exactly one `fn main() void`. `main` in the
// entry module is the entry identity, not an overload family. A library
// Target has no entry, and a test Target's entry is the compiler-generated
// harness, so neither is checked here. anchor locates the missing-entry
// diagnostic, normally the entry module's declaration.
//
// Rules:
//   - rules/compiler/initialization.md — § 21 "Command entry", § 22 "Entry name in the entry module", § 25 "Firmware entry", § 26 "Library targets", § 27 "Test targets", § 42 "Diagnostics"
func (a *Analyzer) ValidateProgramEntry(kind ProgramTargetKind, entryModule string, anchor lexer.Token) []Error {
	var want string
	switch kind {
	case ProgramTargetCommand:
		want = "fn main() int"
	case ProgramTargetFirmware:
		want = "fn main() void"
	default:
		return nil
	}
	entries := []Function{}
	for _, function := range a.functions["main"] {
		if function.Module == entryModule && function.ImplTarget == "" {
			entries = append(entries, function)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		left, right := entries[i].Token, entries[j].Token
		if left.File != right.File {
			return left.File < right.File
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Column < right.Column
	})
	if len(entries) == 0 {
		return []Error{entryError(anchor, lexer.Token{}, diagnostics.ProgramEntryMissing,
			fmt.Sprintf("Declare `%s { ... }` at module level in module %s.", want, entryModule),
			"%s target has no entry: module %s declares no `%s`", kind, entryModule, want)}
	}
	errors := []Error{}
	for index, entry := range entries {
		if index > 0 {
			errors = append(errors, entryError(entry.Token, entries[0].Token, diagnostics.ProgramEntryDuplicate,
				"main is the program entry and cannot be overloaded; rename this function.",
				"duplicate entry main in module %s; the entry module has exactly one `%s`", entryModule, want))
			continue
		}
		if problems := entryProblems(kind, entry); len(problems) > 0 {
			errors = append(errors, entryError(entry.Token, lexer.Token{}, diagnostics.ProgramEntryInvalid,
				fmt.Sprintf("Write `%s`. Command-line arguments and the environment are read through library facilities, not entry parameters.", want),
				"%s entry must be `%s`; this main %s", kind, want, strings.Join(problems, ", ")))
		}
	}
	return errors
}

// entryProblems lists how an entry declaration departs from its contract.
func entryProblems(kind ProgramTargetKind, entry Function) []string {
	problems := []string{}
	if len(entry.GenericParameters) > 0 {
		problems = append(problems, "is generic")
	}
	if entry.Extern {
		problems = append(problems, "is an extern declaration")
	}
	if len(entry.Parameters) > 0 {
		problems = append(problems, "takes parameters")
	}
	returnsInt := entry.ReturnType.Kind == IntType && entry.ReturnType.Name == "int"
	switch {
	case kind == ProgramTargetCommand && !returnsInt:
		problems = append(problems, "returns "+typeDisplayName(entry.ReturnType))
	case kind == ProgramTargetFirmware && entry.ReturnType.Kind != VoidType:
		problems = append(problems, "returns "+typeDisplayName(entry.ReturnType))
	}
	return problems
}

func entryError(token lexer.Token, previous lexer.Token, id string, help string, format string, args ...any) Error {
	endLine, endColumn := token.EndPosition()
	return Error{
		ID:             id,
		Severity:       diagnostics.SeverityError,
		Help:           help,
		Message:        fmt.Sprintf(format, args...),
		File:           token.File,
		Line:           token.Line,
		Column:         token.Column,
		EndLine:        endLine,
		EndColumn:      endColumn,
		PreviousFile:   previous.File,
		PreviousLine:   previous.Line,
		PreviousColumn: previous.Column,
	}
}
