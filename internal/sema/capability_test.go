package sema

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// Every Sema diagnostic that reports an implementation limit carries an
// implementation-capability identity, so no current compiler limitation is
// presented as a source semantic error. The check reads the producer calls:
// a message saying something is not implemented, not supported yet or a
// compiler capability is unavailable must go through
// reportCapabilityUnavailable or name a compiler-capability diagnostic.
// Rules: rules/compiler/compiler_pipeline.md — § 32(2)–(4).
func TestSemaCapabilityLimitsUseCapabilityIdentity(t *testing.T) {
	capabilityIDs := map[string]string{
		"CompilerCapabilityUnavailable":           diagnostics.CompilerCapabilityUnavailable,
		"GenericTypePackExpansionUnavailable":     diagnostics.GenericTypePackExpansionUnavailable,
		"HardwareContextUnavailable":              diagnostics.HardwareContextUnavailable,
		"HardwareSchemaUnavailable":               diagnostics.HardwareSchemaUnavailable,
		"SemanticCompileTimeExecutionUnavailable": diagnostics.SemanticCompileTimeExecutionUnavailable,
	}
	for name, id := range capabilityIDs {
		if !diagnostics.IsImplementationCapability(id) {
			t.Errorf("%s (%s) is not an implementation-capability identity", name, id)
		}
	}
	if diagnostics.IsImplementationCapability(diagnostics.DuplicateDeclaration) {
		t.Error("a source error is classified as an implementation capability")
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	limit := func(text string) bool {
		return strings.Contains(text, "not implemented yet") || strings.Contains(text, "not supported yet") || strings.Contains(text, "capability unavailable")
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, source, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(selector.Sel.Name, "addError") {
				return true
			}
			message, identified := "", false
			for _, argument := range call.Args {
				switch argument := argument.(type) {
				case *ast.BasicLit:
					if text, err := strconv.Unquote(argument.Value); err == nil && limit(text) {
						message = text
					}
				case *ast.SelectorExpr:
					if _, capability := capabilityIDs[argument.Sel.Name]; capability {
						identified = true
					}
				}
			}
			if message != "" && !identified {
				t.Errorf("%s: %q reports an implementation limit without a capability identity; use reportCapabilityUnavailable", file, message)
			}
			return true
		})
	}
}

// A capability boundary reaches tooling as the S1239 identity.
func TestCapabilityBoundaryDiagnostic(t *testing.T) {
	errors := analyzeSourceRaw(t, "module main\n\nfn Work() int {\n    return 1\n}\n\nfn main() int {\n    let child := spawn process Work()\n    return 0\n}\n")
	found := false
	for _, err := range errors {
		if err.ID == diagnostics.CompilerCapabilityUnavailable && err.IsImplementationCapability() && strings.Contains(err.Message, "spawn process is not implemented yet") && err.Help != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("errors = %v", errors)
	}
}
