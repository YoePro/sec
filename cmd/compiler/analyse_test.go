package main

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const analyseFixture = `module main
fn Consume(value: int) void {}
fn Walk(values: ref int[], unused: int) void {
    for i in RANGE {
        Consume(values[i])
    }
}
fn main() void {
    let values: int[] := [1, 2, 3]
    Walk(ref values, 4)
}
`

// `sec analyse` runs every available analysis by default at Deep depth,
// reports only the selected sources, distinguishes the §61(3) result classes,
// and keeps proven invalidity an error.
//
// Rules:
//   - rules/compiler/compiler_analysis.md — § 61(1–3) `sec analyse`
//   - rules/compiler/compiler_pipeline.md — § 65 analysis-only mode
//   - rules/analysis/pitfall_analysis.md — "Proven invalidity is not a warning"; sec analyse
//   - rules/analysis/parameter_usage_analysis.md — `sec analyse --all`
func TestAnalyseCLI(t *testing.T) {
	dir := t.TempDir()
	advisory := filepath.Join(dir, "advisory.sec")
	invalid := filepath.Join(dir, "invalid.sec")
	for path, rangeText := range map[string]string{advisory: "uint(1)..<values.Len", invalid: "uint(0)..values.Len"} {
		if err := os.WriteFile(path, []byte(strings.Replace(analyseFixture, "RANGE", rangeText, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	recommendations := filepath.Join(dir, "recommendations.sec")
	fixture, err := os.ReadFile("../../testdata/sema/parameter_recommendations_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(recommendations, fixture, 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		args    []string
		code    int
		want    []string
		without []string
	}{
		{
			name: "parameter recommendation explanations", args: []string{recommendations}, code: 0,
			want: []string{
				"Read(frame): candidate ref Frame, recommended", "confidence strong", "reason: AvoidCopyCost",
				"ForwardRead(frame): candidate ref Frame, recommended",
				"ForwardSink(frame): candidate ref Frame, blocked", "blocked: ownership demand is consumption-required",
				"Small(value): candidate ref int, not-preferred", "reason: value is below the large-value cost threshold",
				"First(values): candidate ref int[100], recommended", "estimated size:",
			},
			without: []string{"confidence certain", "Borrowed(frame): candidate", "sec/core"},
		},
		{
			name: "default runs all analyses", args: []string{advisory}, code: 0,
			want: []string{
				"analysis report (depth deep,",
				"call graph:\n  3 callables, 2 call sites\n  root program-entry main",
				"main -> Walk [direct, synchronous]",
				"effects:", "escape:",
				"Walk(values ref int[]): access read, mutation no-mutation, ownership borrow-sufficient",
				"Walk(unused int): access unused",
				"advisory pitfall.bounds.skipped-zero at " + advisory + ":5:",
				"suggested-edit: start complete traversal at index 0",
				"results: 0 errors, 0 unproven, 0 warnings, 1 advisory",
			},
			without: []string{"sec/core", "ToString"},
		},
		{name: "explicit all", args: []string{"--all", advisory}, code: 0, want: []string{"results: 0 errors, 0 unproven, 0 warnings, 1 advisory"}},
		{
			name: "proven invalidity stays an error", args: []string{invalid}, code: 3,
			want: []string{"error pitfall.bounds.inclusive-length-index at " + invalid + ":5:", "(proven-invalid, proven, owning rule bounds)", "results: 1 errors,"},
		},
		{name: "explicit target", args: []string{"--target", "linux-arm64", advisory}, code: 0, want: []string{"target linux-arm64)"}},
		{name: "no inputs", args: []string{"--all"}, code: 1, want: []string{"analyse requires at least one"}},
		{name: "unknown option", args: []string{"--pitfalls", advisory}, code: 1, want: []string{"unknown analyse option --pitfalls"}},
		{name: "missing target", args: []string{advisory, "--target"}, code: 1, want: []string{"--target requires <os-arch>"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-test.run=^TestAnalyseCLIProcess$", "--", "analyse"}, tt.args...)
			cmd := exec.Command(os.Args[0], args...)
			cmd.Env = append(os.Environ(), "SEC_ANALYSE_TEST_PROCESS=1")
			output, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				exit, ok := err.(*exec.ExitError)
				if !ok {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tt.code {
				t.Fatalf("exit code = %d, want %d; output: %s", code, tt.code, output)
			}
			for _, want := range tt.want {
				if !strings.Contains(string(output), want) {
					t.Errorf("output does not contain %q:\n%s", want, output)
				}
			}
			for _, unwanted := range tt.without {
				if strings.Contains(string(output), unwanted) {
					t.Errorf("output reports unselected source %q:\n%s", unwanted, output)
				}
			}
		})
	}
}

func TestAnalyseCLIProcess(t *testing.T) {
	if os.Getenv("SEC_ANALYSE_TEST_PROCESS") != "1" {
		return
	}
	os.Args = append(os.Args[:1], os.Args[3:]...)
	flag.CommandLine = flag.NewFlagSet("sec", flag.ExitOnError)
	main()
	os.Exit(0)
}

// Every registered target's C ABI model is coherent with its scalar plan, and
// C:: widths follow the --target CompilationPlan end to end.
//
// Rules:
//   - rules/platform/ffi.md — §5, §49 "Target and ABI validation"
//   - rules/platform/abi.md — § 19 "C scalar representation"
func TestTargetsCarryCoherentCABIModels(t *testing.T) {
	for _, definition := range targets {
		if definition.PointerWidthBits == 0 {
			continue
		}
		plan, err := definition.scalarPlan()
		if err != nil {
			t.Fatalf("%s-%s: %v", definition.OS, definition.Arch, err)
		}
		if definition.Profile != "" && definition.Arch != "any" && !plan.CABI.Defined() {
			t.Fatalf("%s-%s has no C ABI model", definition.OS, definition.Arch)
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "clong.sec")
	if err := os.WriteFile(path, []byte("module main\nfn main() void {\n    let wide: C::long := 3000000000\n    discard wide\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		target string
		code   int
		want   string
	}{
		{"linux-amd64", 0, "results: 0 errors"},
		{"windows-amd64", 3, "value 3000000000 overflows C::long"},
	} {
		args := []string{"-test.run=^TestAnalyseCLIProcess$", "--", "analyse", "--target", test.target, path}
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "SEC_ANALYSE_TEST_PROCESS=1")
		output, err := cmd.CombinedOutput()
		code := 0
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		}
		if code != test.code || !strings.Contains(string(output), test.want) {
			t.Fatalf("%s exit %d output:\n%s\nwant exit %d containing %q", test.target, code, output, test.code, test.want)
		}
	}
}
