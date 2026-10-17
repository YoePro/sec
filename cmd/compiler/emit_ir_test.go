package main

import (
	"strings"
	"testing"

	semantic "sec/internal/ir/semantic"
)

func TestParseEmitIRCommandArgs(t *testing.T) {
	target := CompilerTarget{OS: "linux", Arch: "amd64"}
	input, output, gotTarget, ok := parseEmitIRCommandArgs([]string{"sample.sec", "-o", "sample.sir", "--target", "linux-amd64"}, target)
	if !ok || input != "sample.sec" || output != "sample.sir" || gotTarget != target {
		t.Fatalf("got %q %q %#v %t", input, output, gotTarget, ok)
	}
	_, output, _, ok = parseEmitIRCommandArgs([]string{"sample.sec"}, target)
	if !ok || output != "-" {
		t.Fatalf("default output = %q, ok=%t", output, ok)
	}
}

func TestFrontendRetainsAnalyzerForSemanticIR(t *testing.T) {
	source := "module main\nfn Answer() int { return 42 }\n"
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(source, "sample.sec", hostCompilerTarget(), false)
	if analyzed.Program == nil || analyzed.Analyzer == nil {
		t.Fatal("frontend discarded program or analyzer")
	}
	module, err := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"sample.sec"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatal(err)
	}
	if text := semantic.Format(module); !strings.HasPrefix(text, "semantic-ir 1\n") {
		t.Fatalf("unexpected output:\n%s", text)
	}
}
