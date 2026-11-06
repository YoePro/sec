package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// Value-producing arm blocks reach verified schema 13: the arm value is built,
// then the arm scope's owned locals are cleaned up, then the arm branches to the
// merge; a returning path leaves through function cleanup.
// Rules: rules/control-flow/flowcontrol_match.md — § 21; rules/memory/destruction.md — § 6.2.
func TestSchema13MatchArmBlocks(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/match_arm_blocks.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "arms.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{PointerWidthBits: width})
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"arms.sec"}, MaxPackage: 17})
			if err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			start := strings.Index(text, `func "main::ArmLocal(`)
			part := text[start : start+strings.Index(text[start:], "\n  }\n")]
			track, scope, merge := strings.Index(part, "cleanup.track-owned"), strings.Index(part, "cleanup.run-scope"), strings.Index(part[max(strings.Index(part, "cleanup.run-scope"), 0):], "branch ^")
			if track < 0 || scope < track || merge < 0 {
				t.Errorf("ArmLocal does not clean its arm local before the merge:\n%s", part)
			}
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 13})
			if err != nil {
				t.Fatal(err)
			}
			verify := exec.Command(tool, "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-match-cfg", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
