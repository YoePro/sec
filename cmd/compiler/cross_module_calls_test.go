package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
)

// writeCrossModuleProgram writes a Project command whose entry calls util,
// which in turn calls mathx: two imported modules, one of them only
// transitively.
func writeCrossModuleProgram(t *testing.T) string {
	t.Helper()
	return writeCrossModuleSources(t, true)
}

func writeCrossModuleSources(t *testing.T, project bool) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"mathx/mathx.sec": "module mathx\n\nfn Add(a: int, b: int) int {\n    return a + b\n}\n",
		"util/util.sec":   "module util\n\nimport \"mathx\"\n\nfn Twice(x: int) int {\n    return mathx.Add(x, x)\n}\n",
		"main.sec":        "module main\n\nimport \"util\"\n\nfn main() int {\n    return util.Twice(21)\n}\n",
	}
	if project {
		files[".sec/sec.toml"] = projectMoveManifest
	}
	for path, text := range files {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "main.sec")
}

// A call into an imported module has the callee's canonical identity, and
// the entry module's Semantic IR and Sec MLIR carry a signature-only
// imported declaration instead of failing verification.
// Rules: rules/compiler/semantic_ir.md — §§ 5(1), 6; rules/compiler/linking.md — §§ 9, 15.
func TestCrossModuleCallIRAndSecMLIR(t *testing.T) {
	mainFile := writeCrossModuleProgram(t)
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", mainFile, "-o", "-")
	if code != 0 {
		t.Fatalf("emit-ir: %d %s", code, stderr)
	}
	for _, want := range []string{`call.direct "util::Twice(int)"`, `func "util::Twice(int)"`, `imported module="util"`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("emit-ir lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "util::util.") || strings.Contains(stdout, "mathx::") {
		t.Errorf("emit-ir doubles the module qualifier or declares an indirect import:\n%s", stdout)
	}
	if os.Getenv("SEC_MLIR_BIN") == "" {
		return
	}
	stdout, stderr, code = runCLIForDiagnostics(t, "emit-sec-mlir", mainFile, "-o", "-")
	if code != 0 {
		t.Fatalf("emit-sec-mlir: %d %s", code, stderr)
	}
	if !strings.Contains(stdout, "sec.imported = true") || !strings.Contains(stdout, "func.func private @sec_sym_") {
		t.Errorf("Sec MLIR lacks the imported declaration:\n%s", stdout)
	}
}

// `sec build` compiles each called imported module, transitively, into its
// own object and links them; the program runs and returns 42.
// Rules: rules/compiler/linking.md — §§ 9, 15, 18–19.
func TestCrossModuleBuildRuns(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	mainFile := writeCrossModuleProgram(t)
	program := filepath.Join(t.TempDir(), "program")
	if _, stderr, code := runCLIForDiagnostics(t, "build", mainFile, "-o", program); code != 0 {
		t.Fatalf("build: %d %s", code, stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, program).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42", err)
	}
}

// Without a Project, the entry directory is the anonymous import root of
// every source (modules.md § 2.2): util's own import of mathx resolves there
// even when the compiler runs in another working directory with an absolute
// entry path.
func TestCrossModuleNestedImportWithoutProject(t *testing.T) {
	mainFile := writeCrossModuleSources(t, false)
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", mainFile, "-o", "-")
	if code != 0 {
		t.Fatalf("emit-ir: %d %s", code, stderr)
	}
	if !strings.Contains(stdout, `call.direct "util::Twice(int)"`) {
		t.Fatalf("emit-ir lacks the imported call:\n%s", stdout)
	}
}

// writeCrossModuleMethodProgram writes a Project command that calls a static
// constructor and an instance method of a struct declared in util.
func writeCrossModuleMethodProgram(t *testing.T) string {
	t.Helper()
	impl := "    static fn Make(v: int) Counter {\n        return Counter { value: v }\n    }\n    fn Value() int {\n        return value\n    }\n"
	return writeCrossModuleMethodSources(t, impl, "fn main() int {\n    let c := util.Counter.Make(41)\n    return c.Value() + 1\n}\n")
}

func writeCrossModuleMethodSources(t *testing.T, impl string, entry string) string {
	t.Helper()
	dir := t.TempDir()
	for path, text := range map[string]string{
		".sec/sec.toml": projectMoveManifest,
		"util/util.sec": "module util\n\ntype Counter struct { value: int }\n\nimpl Counter {\n" + impl + "}\n",
		"main.sec":      "module main\n\nimport \"util\"\n\n" + entry,
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "main.sec")
}

// Static and instance methods of an imported type are declared like free
// functions, and the type and callables keep the canonical identities the
// declaring module uses (util::Counter, not util::util.Counter).
// Rules: rules/compiler/semantic_ir.md — §§ 5(1), 6, 60–61; rules/declarations/impl.md — §§ 8, 11.
func TestCrossModuleMethodsIR(t *testing.T) {
	mainFile := writeCrossModuleMethodProgram(t)
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", mainFile, "-o", "-")
	if code != 0 {
		t.Fatalf("emit-ir: %d %s", code, stderr)
	}
	for _, want := range []string{
		`struct "util::Counter"`,
		`call.direct "util::Counter.Make(int)"`, `call.direct "util::Counter.Value()"`,
		`func "util::Counter.Make(int)"`, `func "util::Counter.Value()"`, `imported module="util"`,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("emit-ir lacks %q:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "util::util.") {
		t.Errorf("emit-ir doubles the module qualifier:\n%s", stdout)
	}
}

// A static method of an imported type is compiled into its module's own
// object, linked, and the program returns 42. (Instance methods need safe
// references and struct construction, which the native track does not lower
// for any module yet; an imported module's object holds all its functions.)
// Rules: rules/compiler/linking.md — §§ 9, 15.
func TestCrossModuleMethodsBuildRuns(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "program")
	mainFile := writeCrossModuleMethodSources(t, "    static fn Double(v: int) int {\n        return v * 2\n    }\n", "fn main() int {\n    return util.Counter.Double(21)\n}\n")
	if _, stderr, code := runCLIForDiagnostics(t, "build", mainFile, "-o", program); code != 0 {
		t.Fatalf("build: %d %s", code, stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, program).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42", err)
	}
}

// A property of an imported type lowers to a call of its accessor, which the
// importing module declares and the defining module, built as an exported
// unit, defines under the same identity even though it never reads the
// property itself.
// Rules: rules/compiler/semantic_ir.md — §§ 5(1), 26; rules/declarations/properties.md — §§ 4–9.
func TestCrossModulePropertyAccessorIdentity(t *testing.T) {
	impl := "    static fn Make(v: int) Counter {\n        return Counter { value: v }\n    }\n    property Doubled: int {\n        get {\n            return value * 2\n        }\n    }\n"
	mainFile := writeCrossModuleMethodSources(t, impl, "fn main() int {\n    let c := util.Counter.Make(21)\n    return c.Doubled\n}\n")
	source, err := os.ReadFile(mainFile)
	if err != nil {
		t.Fatal(err)
	}
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(string(source), mainFile, hostCompilerTarget(), false)
	const accessor = "util::Counter.Doubled.get()"
	entry, err := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{SourceFiles: []string{mainFile}, MaxPackage: inspectionMaxPackage})
	if err != nil {
		t.Fatal(err)
	}
	unit, err := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{RequestedModule: "util", SourceFiles: []string{mainFile}, MaxPackage: inspectionMaxPackage, Exported: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		module   *semantic.Module
		imported bool
	}{{entry, true}, {unit, false}} {
		if err := semantic.Verify(check.module); err != nil {
			t.Fatalf("%s: %v", check.module.Identity, err)
		}
		found := false
		for _, function := range check.module.Functions {
			if string(function.ID) == accessor {
				found = true
				if function.Imported != check.imported || check.imported == (len(function.Blocks) != 0) {
					t.Errorf("%s: accessor imported=%t blocks=%d", check.module.Identity, function.Imported, len(function.Blocks))
				}
			}
		}
		if !found {
			t.Errorf("%s lacks %s:\n%s", check.module.Identity, accessor, semantic.Format(check.module))
		}
	}
}

// A generic function of an imported module specializes in each module that
// demands it. When both the entry and the imported module instantiate
// util::Id[T=int], the imported unit's copy has private linkage, so the
// objects link without a duplicate definition and the program returns 42.
// Rules: rules/compiler/monomorphization.md — §§ 4, 9; rules/compiler/linking.md — §§ 15, 23–24.
func TestCrossModuleGenericSpecializationBuildRuns(t *testing.T) {
	dir := t.TempDir()
	for path, text := range map[string]string{
		".sec/sec.toml": projectMoveManifest,
		"util/util.sec": "module util\n\nfn Id[T](value: T) T {\n    return value\n}\n\nfn Answer() int {\n    return Id[int](40) + 2\n}\n",
		"main.sec":      "module main\n\nimport \"util\"\n\nfn main() int {\n    return util.Id[int](util.Answer())\n}\n",
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mainFile := filepath.Join(dir, "main.sec")
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", mainFile, "-o", "-")
	if code != 0 || !strings.Contains(stdout, `func "util::Id[T=int](int)"`) || !strings.Contains(stdout, "specialization(util.Id") {
		t.Fatalf("emit-ir: %d %s\n%s", code, stderr, stdout)
	}
	testsupport.RequireSecMLIROptPath(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	program := filepath.Join(t.TempDir(), "program")
	if _, stderr, code := runCLIForDiagnostics(t, "build", mainFile, "-o", program); code != 0 {
		t.Fatalf("build: %d %s", code, stderr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, program).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 42 {
		t.Fatalf("status = %v, want 42", err)
	}
}

// Equivalent multi-module source produces byte-identical Semantic IR, Sec
// MLIR, LLVM IR and executables across runs and Project locations, and the
// LLVM IR names the Sec module it was compiled from.
// Rules: rules/compiler/compiler_pipeline.md — § 86; rules/projects/projects.md — § 34.
func TestCrossModuleArtifactsAreDeterministic(t *testing.T) {
	testsupport.RequireSecMLIROptPath(t)
	first, second := writeCrossModuleProgram(t), writeCrossModuleProgram(t)
	for _, command := range []string{"emit-ir", "emit-sec-mlir", "emit-llvm"} {
		emit := func(file string) string {
			stdout, stderr, code := runCLIForDiagnostics(t, command, file, "-o", "-")
			if code != 0 {
				t.Fatalf("%s: %d %s", command, code, stderr)
			}
			return stdout
		}
		a, b, c := emit(first), emit(first), emit(second)
		if a != b || a != c {
			t.Errorf("%s output differs between runs or locations", command)
		}
		if command == "emit-llvm" && (!strings.Contains(a, "; ModuleID = 'main'") || !strings.Contains(a, `source_filename = "main"`)) {
			t.Errorf("LLVM IR does not name its Sec module:\n%s", a)
		}
	}
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang unavailable")
	}
	build := func(file string) []byte {
		program := filepath.Join(t.TempDir(), "program")
		if _, stderr, code := runCLIForDiagnostics(t, "build", file, "-o", program); code != 0 {
			t.Fatalf("build: %d %s", code, stderr)
		}
		data, err := os.ReadFile(program)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	if a, b := build(first), build(second); string(a) != string(b) {
		t.Errorf("executables differ between Project locations (%d and %d bytes)", len(a), len(b))
	}
}
