package authority_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/codegen/llvm"
	legacy "sec/internal/codegen/mlir"
	"sec/internal/codegen/targetplan"
	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// load keeps compiler authority distinct from forgeable paths and prefixes.
// Rules: rules/compiler/compiler_known_members.md — Internal core string-slice helper;
// rules/foundations/names_scopes_visibility.md — §19.
func load(t *testing.T, fixture, file string, trusted bool) *ast.Program {
	t.Helper()
	data, err := os.ReadFile("../../../../testdata/codegen/compiler_authority/" + fixture)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	if trusted {
		program.SourceProvenance = map[string]ast.SourceProvenance{file: ast.SourceCore}
	}
	return program
}

// TestCompilerAuthorityAcrossLegacyEntryPoints covers loader authority, owner
// privacy, source signature validation and ordinary private names on both plans.
// Rules: rules/compiler/compiler_known_members.md — Internal intrinsic operations;
// rules/compiler/compiler_pipeline.md — Sema before lowering.
func TestCompilerAuthorityAcrossLegacyEntryPoints(t *testing.T) {
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		for _, test := range []struct {
			name, file, fixture string
			trusted, success    bool
		}{
			{"user", "user.sec", "helper.sec", false, false},
			{"forged owner path", "/tmp/project/sec/core/string.sec", "helper.sec", false, false},
			{"other trusted core", "sec/core/task.sec", "helper.sec", true, false},
			{"trusted owner", "/tmp/project/sec/core/string.sec", "helper.sec", true, true},
			{"ordinary private", "user.sec", "ordinary.sec", false, true},
			{"user builtin impl", "user.sec", "user_impl_invalid.sec", false, false},
			{"forged builtin impl", "sec/core/spoof.sec", "user_impl_invalid.sec", false, false},
			{"internal callable value", "sec/core/string.sec", "value_invalid.sec", true, false},
		} {
			t.Run(triple+"/"+test.name, func(t *testing.T) {
				for _, backend := range []struct {
					name     string
					generate func(*ast.Program, string) (string, error)
				}{{"LLVM", llvm.GenerateWithTriple}, {"legacy MLIR", legacy.GenerateWithTriple}} {
					program := load(t, test.fixture, test.file, test.trusted)
					out, err := backend.generate(program, triple)
					if (err == nil) != test.success {
						t.Fatalf("%s: %v", backend.name, err)
					}
					if !test.success {
						var unsupported *semantic.UnsupportedFeatureError
						if out != "" || !errors.As(err, &unsupported) || unsupported.Location.File != test.file || unsupported.Location.Line == 0 {
							t.Fatalf("%s partial/unmapped failure: %q %v", backend.name, out, err)
						}
					} else if out == "" {
						t.Fatalf("%s empty success", backend.name)
					}
				}
			})
		}
	}
}

// TestAnalyzedAuthorityDoesNotAcceptRevokedOrChangedSource validates the current
// input rather than trusting a stale analyzer or earlier successful generation.
// Rules: rules/compiler/compiler_known_members.md — loader-proven core authority;
// rules/compiler/compiler_pipeline.md — validation before emission.
func TestAnalyzedAuthorityDoesNotAcceptRevokedOrChangedSource(t *testing.T) {
	triple := "x86_64-pc-linux-gnu"
	plan, _ := targetplan.Plan(triple)
	for _, mutation := range []string{"provenance", "argument"} {
		program := load(t, "helper.sec", "sec/core/string.sec", true)
		a := sema.NewAnalyzerWithScalarPlan(plan)
		if errs := a.Analyze(program); len(errs) != 0 {
			t.Fatal(errs)
		}
		if mutation == "provenance" {
			program.SourceProvenance = nil
		} else {
			for _, statement := range program.Statements {
				if fn, ok := statement.(*ast.FunctionDeclaration); ok {
					call := fn.Body.Statements[0].(*ast.ExpressionStatement).Expression.(*ast.CallExpression)
					call.Arguments[1] = &ast.BooleanLiteral{Token: call.Token, Value: true}
				}
			}
		}
		// LLVM intentionally omits core output roots; use its raw boundary for
		// changed core bodies and both analyzed APIs for revoked output roots.
		generators := []func() (string, error){
			func() (string, error) { return llvm.GenerateWithTriple(program, triple) },
			func() (string, error) { return legacy.GenerateAnalyzed(program, a, triple) },
		}
		if mutation == "provenance" {
			generators = append(generators, func() (string, error) { return llvm.GenerateAnalyzed(program, a, triple) })
		}
		for _, generate := range generators {
			out, err := generate()
			if out != "" || err == nil || !strings.Contains(err.Error(), "compiler authority validation") {
				t.Fatalf("stale %s authority: %q %v", mutation, out, err)
			}
		}
	}
}

// TestInternalFunctionValueAuthority keeps source access separate from the
// deferred callable representation: the trusted owner may bind the operation,
// whereas ordinary or sibling sources cannot acquire its authority.
// Rules: rules/compiler/compiler_known_members.md — Internal core string-slice helper;
// rules/foundations/names_scopes_visibility.md — §§12.3, 19.
func TestInternalFunctionValueAuthority(t *testing.T) {
	for _, test := range []struct {
		file             string
		trusted, allowed bool
	}{
		{"sec/core/string.sec", true, true},
		{"sec/core/string.sec", false, false},
		{"sec/core/task.sec", true, false},
	} {
		a := sema.NewAnalyzer()
		errs := a.Analyze(load(t, "value_invalid.sec", test.file, test.trusted))
		if (len(errs) == 0) != test.allowed {
			t.Fatalf("%s trusted=%v: %v", test.file, test.trusted, errs)
		}
	}
}

// TestAuthorizedLLVMCompiles preserves the accepted existing LLVM body on both
// scalar plans and executes the native fixture without adding a new operation.
// Rules: rules/compiler/compiler_known_members.md — Internal core string-slice helper.
func TestAuthorizedLLVMCompiles(t *testing.T) {
	clang, err := exec.LookPath("clang")
	if err != nil {
		t.Skip("clang unavailable")
	}
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		program := load(t, "helper.sec", "sec/core/string.sec", true)
		out, err := llvm.GenerateWithTriple(program, triple)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		source := filepath.Join(dir, "helper.ll")
		output := filepath.Join(dir, "helper")
		if err := os.WriteFile(source, []byte(out), 0600); err != nil {
			t.Fatal(err)
		}
		args := []string{"-target", triple, "-x", "ir", source, "-o", output}
		if strings.HasPrefix(triple, "armv7") {
			args = append(args, "-c")
		}
		if log, err := exec.Command(clang, args...).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v %s", triple, err, log)
		}
		if strings.HasPrefix(triple, "x86_64") {
			if log, err := exec.Command(output).CombinedOutput(); err != nil {
				t.Fatalf("native execution: %v %s", err, log)
			}
		}
	}
}

// Rules: rules/compiler/compiler_known_members.md — Lookup order;
// rules/compiler/compiler_pipeline.md — Sema before lowering; temporal.md — §2.
func TestRefinedTypeExtensionsCannotBypassBackendAuthority(t *testing.T) {
	for _, triple := range []string{"x86_64-pc-linux-gnu", "armv7-unknown-linux-gnueabihf"} {
		for _, stale := range []bool{false, true} {
			program := &ast.Program{SourceProvenance: map[string]ast.SourceProvenance{}}
			for _, name := range []string{"refined.sec.in", "inject.sec.in"} {
				data, err := os.ReadFile("../../../../testdata/core/extension_authority/" + name)
				if err != nil {
					t.Fatal(err)
				}
				source := strings.ReplaceAll(strings.ReplaceAll(string(data), "TYPE", "duration"), "EXTEND", "extends")
				path := "sec/core/" + name
				p := parser.New(lexer.NewWithFile(source, path))
				parsed := p.ParseProgram()
				if len(p.Errors()) > 0 {
					t.Fatal(p.Errors())
				}
				program.Statements = append(program.Statements, parsed.Statements...)
				if name == "refined.sec.in" || stale {
					program.SourceProvenance[path] = ast.SourceCore
				}
			}
			plan, err := targetplan.Plan(triple)
			if err != nil {
				t.Fatal(err)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(plan)
			if stale {
				if errors := analyzer.Analyze(program); len(errors) != 0 {
					t.Fatal(errors)
				}
				delete(program.SourceProvenance, "sec/core/inject.sec.in")
			}
			generators := []func() (string, error){
				func() (string, error) { return llvm.GenerateWithTriple(program, triple) },
				func() (string, error) { return legacy.GenerateWithTriple(program, triple) },
			}
			if stale {
				generators = append(generators,
					func() (string, error) { return llvm.GenerateAnalyzed(program, analyzer, triple) },
					func() (string, error) { return legacy.GenerateAnalyzed(program, analyzer, triple) },
				)
			}
			for _, generate := range generators {
				out, err := generate()
				var failure *semantic.UnsupportedFeatureError
				if err == nil || out != "" || !errors.As(err, &failure) || failure.Location.File != "sec/core/inject.sec.in" || failure.Location.Line != 3 {
					t.Fatalf("%s stale=%v: %q %v", triple, stale, out, err)
				}
			}
		}
	}
}
