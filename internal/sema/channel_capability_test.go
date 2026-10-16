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

// TestChannelCapabilityClosedCoreSet verifies that real core source and
// standalone registration share one reserved enum identity with four members.
// Rules: rules/concurrency/channels.md — §§4–5.
func TestChannelCapabilityClosedCoreSet(t *testing.T) {
	const file = "../../sec/core/channel.sec"
	source, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
	a := NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	typ := a.Types()["ChannelCapability"]
	want := []string{"Revocation", "Expiration", "ISRSafeSend", "Statistics"}
	if !typ.Intrinsic || !typ.Named || typ.Module != "core" || typ.Kind != EnumType || !reflect.DeepEqual(typ.EnumValues, want) || !sameConcreteType(typ, builtinType("ChannelCapability")) {
		t.Fatal(typ)
	}
	if len(typ.EnumConsts) != 4 {
		t.Fatal(typ.EnumConsts)
	}
	for _, name := range want {
		if typ.EnumConsts[name].Token.File != file {
			t.Fatal(name, typ.EnumConsts[name])
		}
	}
	for _, module := range []string{"main", "core"} {
		_, errors := analyzeSourceWithAnalyzerRaw(t, "module "+module+"\nenum ChannelCapability { Priority }\n")
		if len(errors) != 1 || !strings.Contains(errors[0].Message, "compiler-known and cannot be redeclared") {
			t.Fatal(errors)
		}
	}
}

// TestChannelCapabilitySource accepts the complete public set without an
// import and rejects Priority through ordinary enum lookup.
// Rules: rules/concurrency/channels.md — §5(1–4).
func TestChannelCapabilitySource(t *testing.T) {
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"channel_capability_valid.sec", 0}, {"channel_capability_invalid.sec", 1},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		_, errors := analyzeSourceWithAnalyzerRaw(t, string(data))
		if len(errors) != test.errors {
			t.Fatal(test.file, errors)
		}
		if test.errors > 0 && !strings.Contains(errors[0].Message, "ChannelCapability.Priority has never been declared") {
			t.Fatal(errors)
		}
	}
}
