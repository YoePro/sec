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

// TestChannelRevokeTypes preserves the complete disposition domain in the
// generic result payload and in nested exhaustive matches, with no error root.
// Rules: rules/concurrency/channels.md — §§13(1–4),14(1–3).
func TestChannelRevokeTypes(t *testing.T) {
	for _, tc := range []struct {
		file   string
		errors int
	}{
		{"channel_revoke_types_valid.sec", 0}, {"channel_revoke_types_invalid.sec", 4},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + tc.file)
		if err != nil {
			t.Fatal(err)
		}
		a, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != tc.errors {
			t.Fatal(tc.file, errors)
		}
		typ := a.Types()["ChannelRevokeResult"]
		if !typ.Named || !typ.Intrinsic || typ.Module != "core" || typ.ErrorAssignable || len(typ.UnionVariants) != 2 {
			t.Fatal(typ)
		}
		disposition := typ.UnionVariants[1].Payload
		if disposition == nil || !disposition.Named || disposition.Module != "core" || disposition.ErrorAssignable || !reflect.DeepEqual(disposition.EnumValues, []string{"Received", "Expired", "Discarded"}) || !sameConcreteType(*disposition, a.Types()["MessageDisposition"]) {
			t.Fatal(disposition)
		}
	}
}

// TestChannelRevokeTypesCore refines real source tokens while requiring exact
// non-error declarations and preserving the complete disposition payload facts.
// Rules: rules/concurrency/channels.md — §§13–14;
// rules/library/core-library.md — compiler-known core declarations.
func TestChannelRevokeTypesCore(t *testing.T) {
	const file = "../../testdata/sema/channel_revoke_types_core.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	for _, tc := range []struct{ old, replacement string }{
		{"", ""}, {"Received\n", "Orphaned\n"}, {"Revoked(T)", "Revoked(ref T)"},
		{"Unavailable(MessageDisposition)", "Unavailable(int)"}, {" union {", " union error {"},
	} {
		input := source
		if tc.old != "" {
			input = strings.Replace(source, tc.old, tc.replacement, 1)
		}
		p := parser.New(lexer.NewWithFile(input, file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
		a := NewAnalyzer()
		errors := a.Analyze(program)
		if tc.old == "" {
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			typ := a.Types()["ChannelRevokeResult"]
			if typ.UnionVariants[1].Token.File != file || typ.UnionVariants[1].Payload.EnumConsts["Received"].Token.File != file {
				t.Fatal("lost source identity", typ)
			}
		} else if len(errors) != 1 || !strings.Contains(errors[0].Message, "canonical core") {
			t.Fatal(tc, errors)
		}
	}
	_, errors := analyzeSourceWithAnalyzerRaw(t, strings.Replace(source, "module core", "module main", 1))
	if len(errors) != 2 {
		t.Fatal(errors)
	}
}
