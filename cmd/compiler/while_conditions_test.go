package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/formatter"
	"sec/internal/testsupport"
)

// Side-effect counters distinguish one condition evaluation per attempt from
// eager logical operands, duplicate try evaluation and incorrect loop targets.
// Run both source and formatter output through the single native track.
// Rules: rules/control-flow/flowcontrol_while.md — §§3–7,13–15,29;
// rules/tooling/formatter.md — §§25,29.
func TestWhileConditionEffectsNative(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	source := "../../testdata/control_flow/while_conditions/effects.sec"
	input, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	formatted := formatter.Format(formatter.Source{Text: string(input)}, formatter.Options{}).Text
	path := filepath.Join(t.TempDir(), "formatted.sec")
	if err := os.WriteFile(path, []byte(formatted), 0600); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{source, path} {
		t.Run(filepath.Base(source), func(t *testing.T) {
			var exit *exec.ExitError
			if err := runNativeProgram(t, source); !errors.As(err, &exit) || exit.ExitCode() != 42 {
				t.Fatalf("native result = %v; want 42 (1–10 name failing checks)", err)
			}
		})
	}
}

// Plain and fallible integer conditions are rejected consistently at the CLI.
// Rules: rules/control-flow/flowcontrol_while.md — §§3,7,30.
func TestWhileConditionDiagnosticsCLI(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/control_flow/while_conditions/non_bool_invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 3 || len(doc.Occurrences) != 2 {
		t.Fatal(code, stderr)
	}
	for _, occurrence := range doc.Occurrences {
		if occurrence.Primary == nil || !strings.Contains(occurrence.Message.Text, "while condition must") || !strings.Contains(occurrence.Message.Text, "bool, got int") {
			t.Fatal(occurrence)
		}
	}
}
