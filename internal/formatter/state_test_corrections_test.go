package formatter

import (
	"strings"
	"testing"
)

// Rules:
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §§ 7.9, 7.15, 7.25
//   - rules/tooling/formatter.md — § 26 "Language Corrections model"
func TestFixRewritesBadPracticeStateTests(t *testing.T) {
	input := "fn F(state: State, option: Option[int], ready: bool) bool {\n    let a := (state is Idle) == true\n    let b := !(state is Idle)\n    let c := ready && !(option is None)\n    let d := !(state is not Idle)\n    return true == (state is Running)\n}\n"
	want := "fn F(state: State, option: Option[int], ready: bool) bool {\n    let a := state is Idle\n    let b := state is not Idle\n    let c := ready && option is not None\n    let d := !(state is not Idle)\n    return state is Running\n}\n"
	if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != want {
		t.Fatalf("Format(Fix) =\n%s\nwant:\n%s", got, want)
	}
	if got := Format(Source{Text: input}, Options{}).Text; got != input {
		t.Fatalf("ordinary formatting rewrote state tests:\n%s", got)
	}
}

func TestFixKeepsStateTestsWhoseGroupingMatters(t *testing.T) {
	input := "fn F(state: State, option: Option[int], flag: bool) bool {\n    let a := flag == !(state is Idle)\n    let b := !(option is Some(value))\n    return a\n}\n"
	if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != input {
		t.Fatalf("Format(Fix) changed grouping-sensitive or binding forms:\n%s", got)
	}
}

// Rules:
//   - rules/tooling/formatter.md — § 27(33)–(35)
func TestFixRewritesLegacyAssignedNamedType(t *testing.T) {
	input := "type UserID = uint64\n\ntype Port=int range 1..65535\n\ntype IOError = FileNotFound AccessDenied\n\ntype Pair[T] = Box[T]\n"
	want := "type UserID uint64\n\ntype Port int range 1..65535\n\ntype IOError = FileNotFound AccessDenied\n\ntype Pair[T] Box[T]\n"
	if got := Format(Source{Text: input}, Options{Fix: true}).Text; got != want {
		t.Fatalf("Format(Fix) =\n%s\nwant:\n%s", got, want)
	}
	ordinary := Format(Source{Text: input}, Options{}).Text
	if !strings.Contains(ordinary, "type UserID = uint64") || !strings.Contains(ordinary, "type Pair[T] = Box[T]") {
		t.Fatalf("ordinary formatting converted legacy syntax:\n%s", ordinary)
	}
}
