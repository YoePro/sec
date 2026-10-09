package corehelpers_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// extensionProgram preserves per-file loader provenance independently of the
// module name and the source path chosen by the caller.
// Rules: rules/library/core-library.md — §1.2; compiler_known_members.md — Lookup order.
func extensionProgram(t *testing.T, kind, primaryPath, extensionPath string, extends, trusted bool) *ast.Program {
	t.Helper()
	program := &ast.Program{SourceProvenance: map[string]ast.SourceProvenance{}}
	for index, name := range []string{"refined.sec.in", "inject.sec.in"} {
		data, err := os.ReadFile(filepath.Join("../../../testdata/core/extension_authority", name))
		if err != nil {
			t.Fatal(err)
		}
		source := strings.ReplaceAll(string(data), "TYPE", kind)
		modifier := ""
		if extends {
			modifier = "extends"
		}
		source = strings.ReplaceAll(source, "EXTEND", modifier)
		path := primaryPath
		if index == 1 {
			path = extensionPath
		}
		p := parser.New(lexer.NewWithFile(source, path))
		parsed := p.ParseProgram()
		if len(p.Errors()) > 0 {
			t.Fatal(p.Errors())
		}
		program.Statements = append(program.Statements, parsed.Statements...)
		if index == 0 || trusted {
			program.SourceProvenance[path] = ast.SourceCore
		}
	}
	return program
}

// Rules: rules/compiler/compiler_known_members.md — Lookup order;
// rules/types/temporal.md — §2; rules/library/core-library.md — §1.2.
func TestRefinedCompilerTypesRejectUserExtension(t *testing.T) {
	for _, kind := range []string{"duration", "date", "time", "datetime"} {
		for _, extends := range []bool{false, true} {
			for _, path := range []string{"ordinary.sec", "sec/core/spoof.sec"} {
				t.Run(kind+"/"+path+"/"+map[bool]string{false: "primary", true: "extends"}[extends], func(t *testing.T) {
					program := extensionProgram(t, kind, "sec/core/owner.sec", path, extends, false)
					a := sema.NewAnalyzer()
					errors := a.Analyze(program)
					if len(errors) != 1 || errors[0].File != path || errors[0].Line != 3 || !strings.Contains(errors[0].Message, "compiler-owned") {
						t.Fatalf("errors: %#v", errors)
					}
					if _, ok := a.Types()[kind+".Injected"]; ok {
						t.Fatal("unauthorized nested type registered")
					}
					if len(a.Functions()[kind+".Hijack"]) != 0 {
						t.Fatal("unauthorized method registered")
					}
					for _, property := range a.Types()[kind].Properties {
						if property.Name == "Added" {
							t.Fatal("unauthorized property registered")
						}
					}
				})
			}
		}
	}
}

// Rules: rules/library/core-library.md — §1.2; names_scopes_visibility.md — §12.2;
// rules/types/types.md — Named types; compiler_known_members.md — Lookup order.
func TestCompilerExtensionRetainsCoreAndNominalAuthority(t *testing.T) {
	for _, kind := range []string{"duration", "date", "time", "datetime"} {
		a := sema.NewAnalyzer()
		if errors := a.Analyze(extensionProgram(t, kind, "sec/core/owner.sec", "sec/core/extension.sec", true, true)); len(errors) != 0 {
			t.Fatal(errors)
		}
		if len(a.Functions()[kind+".Hijack"]) != 1 {
			t.Fatal("trusted core extension lost")
		}
	}
	data, err := os.ReadFile("../../../testdata/core/extension_authority/nominal.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), "user.sec"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := sema.NewAnalyzer()
	if errors := a.Analyze(program); len(errors) != 0 {
		t.Fatal(errors)
	}
	method := a.Functions()["UserDuration.Inspect"]
	if len(method) != 1 || method[0].CompilerKnownID != "" || method[0].CompilerInternal {
		t.Fatalf("ordinary method: %#v", method)
	}
}

// Rules: rules/compiler/compiler_known_members.md — Private core UTC wall-clock intrinsic.
func TestUnderscoreCompilerServicesRemainCoreOnly(t *testing.T) {
	for _, function := range sema.CompilerKnownFunctions() {
		if strings.HasPrefix(function.Name, "_") && !function.Internal {
			t.Fatalf("public underscore compiler function: %#v", function)
		}
	}
	for _, value := range sema.CompilerKnownValues() {
		if strings.HasPrefix(value.Name, "_") && !value.Internal {
			t.Fatalf("public underscore compiler value: %#v", value)
		}
	}
}

// Rules: rules/library/core-library.md §1.2; names_scopes_visibility.md §§12.2–12.3.
func TestCoreUnderscoreHelperRejectsForgedModuleCaller(t *testing.T) {
	program := load(t, "helpers.sec", "sibling.sec")
	delete(program.SourceProvenance, "sibling.sec")
	a := sema.NewAnalyzer()
	errors := a.Analyze(program)
	if len(errors) != 1 || errors[0].File != "sibling.sec" || !strings.Contains(errors[0].Message, "_Shared") {
		t.Fatalf("errors: %#v", errors)
	}
	helper := a.Functions()["_Shared"][0]
	if a.FunctionVisibleFromSource(helper, "sibling.sec") || !a.FunctionVisibleFromSource(helper, "helpers.sec") {
		t.Fatal("core helper visibility lost authority")
	}
}
