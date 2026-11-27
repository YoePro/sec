package secmlir

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/platform/target"
	"sec/internal/sema"
	"sec/internal/testsupport"
)

// buildRegisteredTargetSource analyzes source under the registered linux
// target plan for arch, so layout facts come from the canonical resolver.
func buildRegisteredTargetSource(t *testing.T, source, arch string) (*semantic.Module, error) {
	t.Helper()
	parsed := parser.New(lexer.NewWithFile(source, "source.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	definition, ok := target.Find(target.Target{OS: "linux", Arch: arch})
	if !ok {
		t.Fatalf("no target linux-%s", arch)
	}
	plan, err := definition.ScalarPlan()
	if err != nil {
		t.Fatal(err)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(plan)
	analyzer.SetAllocationProfile("hosted")
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	return semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"source.sec"}, MaxPackage: 18})
}

// Instance SizeOf on an owning dynamic array is Len times the element's
// layout stride under the target plan: byte[] has stride 1 on every plan and
// int[] the plan's int stride (4 on 32-bit, 8 on 64-bit). It lowers to one
// sec.dynamic_array.payload_size (v14 § 13D) that passes the real dialect.
// Rules: rules/collections/collections.md — § 3.4; rules/memory/layout.md — § 9.
func TestDynamicArrayPayloadSizeVerifiesWithRealDialect(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := "module main\n\nfn Bytes() uint {\n    let mut values: byte[]\n    return values.SizeOf\n}\n\nfn Ints() uint {\n    let mut values: int[]\n    return values.SizeOf\n}\n\nfn main() int {\n    return 0\n}\n"
	for _, target := range []struct {
		width     uint16
		arch      string
		intStride string
	}{{32, "armv7", "4"}, {64, "amd64", "8"}} {
		t.Run(fmt.Sprint(target.width), func(t *testing.T) {
			module, err := buildRegisteredTargetSource(t, source, target.arch)
			if err != nil {
				t.Fatal(err)
			}
			if err := semantic.Verify(module); err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			if !strings.Contains(text, "dynamic-array.payload-size %") || !strings.Contains(text, "stride=1") || !strings.Contains(text, "stride="+target.intStride) {
				t.Fatalf("want strides 1 and %s:\n%s", target.intStride, text)
			}
			output, err := EmitWithOptions(module, testPlan(target.width), EmitOptions{DialectVersion: 14})
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(output), `"sec.dynamic_array.payload_size"`) != 2 || !strings.Contains(string(output), `stride = "`+target.intStride+`"`) {
				t.Fatalf("want two payload_size operations:\n%s", output)
			}
			verify := exec.Command(tool, "--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-places", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s\n%s", err, result, output)
			}
		})
	}
	if _, err := buildRegisteredTargetSource(t, "module main\n\nfn F() uint {\n    let mut values: string[]\n    return values.SizeOf\n}\n", "amd64"); err == nil || !strings.Contains(err.Error(), "without complete layout") {
		t.Fatalf("descriptor element SizeOf: %v", err)
	}
}

// Type-form `T.SizeOf` and value-form SizeOf of a statically sized value are
// the canonical layout size of the type under the target plan: int32 is 4,
// int16 2 and int[3] three int strides on every plan, so int[3] is 12 bytes
// on 32-bit and 24 on 64-bit targets.
// Rules: rules/compiler/compiler_known_members.md — §§ 14(4), 15(4), 16(4);
// rules/memory/layout.md — § 7.
func TestStaticSizeOfUsesTargetLayout(t *testing.T) {
	source := "module main\n\nfn T() uint {\n    return int32.SizeOf\n}\n\nfn V() uint {\n    let x: int16 := 1\n    let a: int[3] := [1, 2, 3]\n    return x.SizeOf + a.SizeOf\n}\n\nfn main() int {\n    return 0\n}\n"
	for _, target := range []struct{ arch, array string }{{"armv7", "12"}, {"amd64", "24"}} {
		module, err := buildRegisteredTargetSource(t, source, target.arch)
		if err != nil {
			t.Fatal(err)
		}
		if err := semantic.Verify(module); err != nil {
			t.Fatal(err)
		}
		text := semantic.Format(module)
		for _, want := range []string{"const.int 4 :", "const.int 2 :", "const.int " + target.array + " :"} {
			if !strings.Contains(text, want) {
				t.Fatalf("%s: missing %q in:\n%s", target.arch, want, text)
			}
		}
	}
}
