package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// MD-083: TemporalArithmeticError is the public error type of
// temporal/arithmetic; core ArithmeticError keeps its own identity, stays
// usable next to the temporal type, and no temporal declaration reintroduces
// the name.
//
// Rules:
//   - stdlib/temporal/arithmetic/std-temporal-arithmetic.md — § 3(3)
//   - stdlib/temporal/std-temporal.md — § 18(3a)
func TestTemporalArithmeticErrorKeepsCoreIdentity(t *testing.T) {
	redeclaration := regexp.MustCompile(`(?m)^\s*(enum|type|union)\s+ArithmeticError\b`)
	fallible := regexp.MustCompile(`Result\[[^\]]*,\s*TemporalArithmeticError\]`)
	err := filepath.WalkDir("../../sec/stdlib/temporal", func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".sec" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if redeclaration.Match(data) {
			t.Errorf("%s redeclares ArithmeticError", path)
		}
		if strings.HasSuffix(path, filepath.Join("arithmetic", "apply.sec")) && len(fallible.FindAll(data, -1)) != 2 {
			t.Errorf("%s: AddMonths and AddPeriod must return TemporalArithmeticError", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "main.sec")
	source := "module main\n\nimport \"temporal/arithmetic\"\n\n" +
		"fn Core() ArithmeticError {\n    return ArithmeticError.Overflow\n}\n\n" +
		"fn Temporal() arithmetic.TemporalArithmeticError {\n    return arithmetic.TemporalArithmeticError.NotImplemented\n}\n\n" +
		"fn main() int {\n    return 0\n}\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if stdout, stderr, code := runCLIForDiagnostics(t, "check", path); code != 0 {
		t.Fatalf("core and temporal arithmetic errors do not coexist: %d %s %s", code, stdout, stderr)
	}
}
