package collectionshape_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../../testdata/sema/collection_contracts", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func analyzeSourceWithAnalyzer(t *testing.T, input string) (*sema.Analyzer, []sema.Error) {
	t.Helper()
	p := parser.New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	return a, a.Analyze(program)
}

func analyzeSource(t *testing.T, input string) []sema.Error {
	t.Helper()
	_, errors := analyzeSourceWithAnalyzer(t, input)
	return errors
}

func analyzeSourceRaw(t *testing.T, input string) []sema.Error {
	t.Helper()
	return analyzeSource(t, input)
}

func errorsContainMessage(errors []sema.Error, fragment string) bool {
	for _, err := range errors {
		if strings.Contains(err.Error(), fragment) {
			return true
		}
	}
	return false
}

func assertSemaErrors(t *testing.T, errors []sema.Error, expected []string) {
	t.Helper()
	if len(errors) != len(expected) {
		t.Fatalf("errors = %v, want %v", errors, expected)
	}
	for i, err := range errors {
		if err.Error() != expected[i] {
			t.Fatalf("error = %v, want %s", err, expected[i])
		}
	}
}
