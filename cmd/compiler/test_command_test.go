package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "calc")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

const calcProduction = "module calc\n\nfn Add(a: int, b: int) int {\n    return a + b\n}\n"

const calcTests = "module calc\n\ntest \"Add sums\" {\n    testing.ExpectEqual(3, Add(1, 2))\n}\n\ntest \"Add keeps zero\" {\n    testing.ExpectEqual(0, Add(0, 0))\n}\n"

// sec test compiles the selected *_test.sec files with their white-box
// production module, discovers top-level tests from compiler metadata, and
// keeps execution unavailability distinct from passing (testing.md §§27–31).
func TestSecTestDiscoversCompilesAndReportsUnavailableExecution(t *testing.T) {
	dir := writeTestModule(t, map[string]string{"calc.sec": calcProduction, "calc_test.sec": calcTests})

	var out bytes.Buffer
	if status := runTestCommand([]string{"--list", dir}, &out); status != testExitSuccess {
		t.Fatalf("--list status = %d\n%s", status, out.String())
	}
	if !strings.Contains(out.String(), `TEST          calc "Add sums"`) || !strings.Contains(out.String(), "2 tests in 1 modules") {
		t.Fatalf("--list output:\n%s", out.String())
	}

	out.Reset()
	if status := runTestCommand([]string{dir}, &out); status != testExitExecutionUnavailable {
		t.Fatalf("run status = %d\n%s", status, out.String())
	}
	if !strings.Contains(out.String(), `UNAVAILABLE   calc "Add keeps zero"`) || !strings.Contains(out.String(), "2 selected: 0 passed, 0 failed, 0 skipped, 2 execution unavailable") {
		t.Fatalf("run output:\n%s", out.String())
	}

	out.Reset()
	if status := runTestCommand([]string{"--json", "--run", "sums$", dir}, &out); status != testExitExecutionUnavailable {
		t.Fatalf("json status = %d", status)
	}
	var report testRunReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json output: %v\n%s", err, out.String())
	}
	tests := report.Modules[0].Tests
	if len(tests) != 1 || tests[0].Name != "Add sums" || tests[0].Status != testExecutionUnavailable ||
		tests[0].Line != 3 || len(tests[0].Identity) != 1 || report.FilteredOut != 1 {
		t.Fatalf("json report = %+v", report)
	}

	out.Reset()
	if status := runTestCommand([]string{"--run", "nothing", dir}, &out); status != testExitSuccess ||
		!strings.Contains(out.String(), `no tests match --run "nothing" (2 discovered)`) {
		t.Fatalf("empty filter status = %d\n%s", status, out.String())
	}
}

func TestSecTestReportsCompilationFailureSeparately(t *testing.T) {
	dir := writeTestModule(t, map[string]string{
		"calc.sec":      calcProduction,
		"calc_test.sec": "module calc\n\ntest \"wrong type\" {\n    let text: string := Add(1, 2)\n    discard text\n}\n",
	})
	var out bytes.Buffer
	if status := runTestCommand([]string{dir}, &out); status != testExitCompilationFailure {
		t.Fatalf("status = %d\n%s", status, out.String())
	}
	if !strings.Contains(out.String(), "COMPILE FAIL  calc") || !strings.Contains(out.String(), "1 modules failed to compile") {
		t.Fatalf("output:\n%s", out.String())
	}
}

// A test body may use bodyless try; the test is the propagation boundary.
func TestSecTestAcceptsTryAtTheTestBoundary(t *testing.T) {
	dir := writeTestModule(t, map[string]string{
		"calc.sec":      "module calc\n\ntype CalcError enum error {\n    Overflow,\n}\n\nfn Checked(a: int) Result[int, CalcError] {\n    return Ok(a)\n}\n",
		"calc_test.sec": "module calc\n\ntest \"try propagates to the test\" {\n    let value := try Checked(1)\n    testing.ExpectEqual(1, value)\n}\n",
	})
	var out bytes.Buffer
	if status := runTestCommand([]string{"--list", dir}, &out); status != testExitSuccess {
		t.Fatalf("status = %d\n%s", status, out.String())
	}
}

func TestSecTestSelectionErrors(t *testing.T) {
	dir := writeTestModule(t, map[string]string{"calc.sec": calcProduction})
	var out bytes.Buffer
	if status := runTestCommand([]string{filepath.Join(dir, "calc.sec")}, &out); status != 1 {
		t.Fatalf("production file status = %d", status)
	}
	if status := runTestCommand([]string{"--bogus"}, &out); status != 1 {
		t.Fatalf("unknown option status = %d", status)
	}
	if status := runTestCommand([]string{dir}, &out); status != testExitSuccess || !strings.Contains(out.String(), "no tests found") {
		t.Fatalf("no tests status = %d\n%s", status, out.String())
	}
}
