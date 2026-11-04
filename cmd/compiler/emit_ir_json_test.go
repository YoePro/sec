package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	semantic "sec/internal/ir/semantic"
	secmlir "sec/internal/lowering/secmlir"
)

// The public command produces one versioned document on stdout or in the
// requested artifact, while its default human output remains unchanged.
// Rules: rules/tooling/cli.md — §5 emit-ir;
// rules/compiler/semantic_ir.md — §§97,106–107.
func TestEmitIRJSONCLI(t *testing.T) {
	fixture := "../../testdata/semantic_ir/wide_scalars.sec"
	for _, target := range []string{"linux-amd64", "linux-armv7"} {
		t.Run(target, func(t *testing.T) {
			text, diagnostics, code := runCLIForDiagnostics(t, "emit-ir", fixture, "--target", target)
			if code != 0 || diagnostics != "" || !strings.HasPrefix(text, "semantic-ir 1\n") {
				t.Fatalf("text: %d %s %s", code, diagnostics, text)
			}
			stdout, diagnostics, code := runCLIForDiagnostics(t, "emit-ir", fixture, "--json", "--target", target, "--diagnostic-format=json")
			if code != 0 {
				t.Fatalf("JSON: %d %s", code, diagnostics)
			}
			if report := decodeOccurrenceDocument(t, diagnostics); len(report.Occurrences) != 0 {
				t.Fatal("unexpected diagnostic occurrences", report)
			}
			module, err := semantic.Deserialize([]byte(stdout))
			if err != nil {
				t.Fatal(err)
			}
			if got := semantic.Format(module); got != text {
				t.Fatal("JSON and text expose different Semantic IR")
			}
			width := uint16(64)
			if target == "linux-armv7" {
				width = 32
			}
			if module.PointerWidthBits != width {
				t.Fatal("lost target scalar plan", module.PointerWidthBits)
			}
			output := filepath.Join(t.TempDir(), "module.sir.json")
			out, diagnostics, code := runCLIForDiagnostics(t, "emit-ir", "--json", fixture, "--target", target, "-o", output)
			if code != 0 || diagnostics != "" || out != "" {
				t.Fatalf("file: %d %s %s", code, diagnostics, out)
			}
			data, err := os.ReadFile(output)
			if err != nil || !bytes.Equal(data, []byte(stdout)) {
				t.Fatalf("file/stdout transport differs: %v", err)
			}
		})
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "help", "emit-ir")
	if code != 0 || stderr != "" || !strings.Contains(stdout, "--json") {
		t.Fatalf("help: %d %s %s", code, stderr, stdout)
	}
}

// A failed frontend or unsupported Semantic IR construction publishes no JSON
// and never overwrites an existing output with a partial serialization.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/compiler/semantic_ir.md — §97.
func TestEmitIRJSONDoesNotPublishFailure(t *testing.T) {
	for _, fixture := range []string{
		"../../testdata/ir/unsupported_for_step.sec",
		"../../testdata/tooling/borrow_authority_invalid.sec",
	} {
		output := filepath.Join(t.TempDir(), "existing.json")
		if err := os.WriteFile(output, []byte("previous output"), 0600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", fixture, "--json", "-o", output)
		if code == 0 || stdout != "" || stderr == "" {
			t.Fatalf("failure published IR: %d %s %s", code, stdout, stderr)
		}
		if data, err := os.ReadFile(output); err != nil || string(data) != "previous output" {
			t.Fatalf("failure changed output: %q %v", data, err)
		}
	}
}

// JSON selection composes with target/output selection in either order and
// rejects malformed requests rather than quietly falling back to text.
// Rules: rules/tooling/cli.md — §5 emit-ir.
func TestParseEmitIRJSONArgs(t *testing.T) {
	target := CompilerTarget{OS: "linux", Arch: "amd64"}
	for _, args := range [][]string{{"--json", "main.sec"}, {"main.sec", "--json", "-o", "out.json", "--target", "linux-amd64"}} {
		input, _, actual, jsonOutput, ok := parseEmitIRCommandArgs(args, target)
		if !ok || !jsonOutput || input != "main.sec" || actual != target {
			t.Fatal(args, input, actual, jsonOutput, ok)
		}
	}
	for _, args := range [][]string{{"--json"}, {"main.sec", "--json", "--json"}, {"main.sec", "--json=true"}, {"main.sec", "--json", "--target"}, {"main.sec", "--json", "-o"}} {
		if _, _, _, _, ok := parseEmitIRCommandArgs(args, target); ok {
			t.Fatal("accepted malformed request", args)
		}
	}
}

// A module loaded through the transport enters the existing Sec MLIR lowering
// boundary unchanged; serialization neither retargets it nor adds a new track.
// Rules: rules/compiler/semantic_ir.md — §§97,103,106–107.
func TestSemanticIRJSONFeedsLowering(t *testing.T) {
	for _, targetName := range []CompilerTarget{{OS: "linux", Arch: "armv7"}, {OS: "linux", Arch: "amd64"}} {
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/semantic_ir/basic_return.sec", "--json", "--target", targetName.String())
		if code != 0 || stderr != "" {
			t.Fatal(code, stderr)
		}
		module, err := semantic.Deserialize([]byte(stdout))
		if err != nil {
			t.Fatal(err)
		}
		target, ok := findTargetDefinition(targetName)
		if !ok {
			t.Fatal("missing target", targetName)
		}
		plan, err := target.scalarPlan()
		if err != nil {
			t.Fatal(err)
		}
		if output, err := secmlir.Emit(module, plan); err != nil || !bytes.Contains(output, []byte("sec.const.int")) {
			t.Fatalf("decoded IR failed existing lowering: %s %v", output, err)
		}
	}
}
