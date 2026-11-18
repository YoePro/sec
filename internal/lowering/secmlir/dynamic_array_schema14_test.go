package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// Owning dynamic arrays lower from verified Semantic IR to schema 14 on 32-
// and 64-bit plans: the growth protocol (has_backing, grow_backing,
// establish_domain/advance_epoch, has_capacity, append_prepared), guarded
// element Places with their backing-domain dependency, len, owner slicing
// whose range uses the runtime length, and the dynamic-array destruction plan
// (end-domain, then reclaim per plan). Every verifier, including
// --sec-verify-dynamic-arrays, accepts the module; schema 13 rejects the
// dynamic-array type by name.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 81–101, 128–131
func TestSchema14DynamicArrays(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/dynamic_array_schema14.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		arch  string
		width uint16
	}{{"armv7", 32}, {"amd64", 64}} {
		t.Run(fmt.Sprint(target.width), func(t *testing.T) {
			parsed := parser.New(lexer.NewWithFile(string(source), "arrays.sec")).Parse()
			if parsed.HasErrors {
				t.Fatal(parsed.Diagnostics)
			}
			analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: target.arch, Profile: "hosted", PointerWidthBits: target.width})
			analyzer.SetAllocationProfile("hosted")
			if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
				t.Fatal(errors)
			}
			module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"arrays.sec"}, MaxPackage: 18})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := EmitWithOptions(module, testPlan(target.width), EmitOptions{DialectVersion: 13}); err == nil || !strings.Contains(err.Error(), "dynamic-array") {
				t.Fatalf("schema 13 accepted a dynamic array: %v", err)
			}
			output, err := EmitWithOptions(module, testPlan(target.width), EmitOptions{DialectVersion: 14})
			if err != nil {
				t.Fatal(err)
			}
			text := string(output)
			for _, want := range []string{
				"sec.dialect_version = 14 : i32",
				`kind = "dynamic-array", element = "destroy::int:int", backing = ["end-domain", "reclaim-per-allocation-plan"]`,
				`"sec.dynamic_array.empty"() : () -> !sec.dynamic_array<!sec.int>`,
				`"sec.dynamic_array.has_backing"`, `"sec.dynamic_array.has_capacity"`, `<{count = "1"}>`,
				`transition = "establish-domain"`, `transition = "advance-epoch"`, `allocation_context = `,
				`"sec.storage.establish_domain"`, `"sec.storage.advance_epoch"`, `"sec.dynamic_array.append_prepared"`,
				`"sec.dynamic_array.index_in_bounds"`, `bounds_kind = "runtime-check", bounds_proof = "guarded", backing_domain = `,
				`"sec.dynamic_array.len"`, `"sec.slice.borrow_shared"`, `"sec.fail.bounds"() {operation = "dynamic-array-index"}`,
			} {
				if !strings.Contains(text, want) {
					t.Errorf("schema-14 module lacks %q", want)
				}
			}
			verify := exec.Command(tool, "--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-places", "--sec-verify-reference-guards", "--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
}
