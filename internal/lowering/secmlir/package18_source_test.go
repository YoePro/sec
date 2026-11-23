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

// buildPackage18Source analyzes and builds one source program at package 18
// on the given plan width, returning the module or the build error.
func buildPackage18Source(t *testing.T, source string, arch string, width uint16) (*semantic.Module, error) {
	t.Helper()
	parsed := parser.New(lexer.NewWithFile(source, "source.sec")).Parse()
	if parsed.HasErrors {
		t.Fatal(parsed.Diagnostics)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(layout.ResolvedScalarPlan{TargetOS: "linux", TargetArch: arch, Profile: "hosted", PointerWidthBits: width})
	analyzer.SetAllocationProfile("hosted")
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatal(errors)
	}
	return semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"source.sec"}, MaxPackage: 18})
}

// P18-43 (§ 131): the existing owning dynamic-array source surface lowers end
// to end to verified schema 14 on 32- and 64-bit plans: a mutable byte[]
// owner, move into another owner, consuming parameter, Result and Option
// return ownership, a union payload, a struct field owner, Len, IsEmpty,
// len(...), index read and replacement, ref values[..], ref mut values[..] and
// Append. The rows whose operation has no schema-14 form stop at named
// boundaries instead of an unresolved call: SizeOf (element stride, P24),
// unsafe Ptr (P18-25), Clear and RemoveAt (no length-reducing operation) and
// ToString (P23).
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — § 131
func TestPackage18SourceEndToEnd(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source, err := os.ReadFile("../../../testdata/semantic_ir/dynamic_array_source.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []struct {
		arch  string
		width uint16
	}{{"armv7", 32}, {"amd64", 64}} {
		t.Run(fmt.Sprint(target.width), func(t *testing.T) {
			module, err := buildPackage18Source(t, string(source), target.arch, target.width)
			if err != nil {
				t.Fatal(err)
			}
			text := semantic.Format(module)
			for _, want := range []string{
				"dynamic-array.empty", "dynamic-array.append-prepared", "dynamic-array.length", "int.compare eq",
				"dynamic-array.element-place", "place.write", "slice.borrow-shared", "slice.borrow-mut",
				"own.move-from-place", "place.field", "union.construct",
			} {
				if !strings.Contains(text, want) {
					t.Errorf("Semantic IR lacks %q", want)
				}
			}
			output, err := EmitWithOptions(module, testPlan(target.width), EmitOptions{DialectVersion: 14})
			if err != nil {
				t.Fatal(err)
			}
			verify := exec.Command(tool, "--sec-verify-dynamic-arrays", "--sec-verify-storage-transitions", "--sec-verify-places",
				"--sec-verify-slice-guards", "--sec-verify-reference-guards", "--sec-verify-borrow-semantics", "--sec-verify-cleanups",
				"--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
			verify.Stdin = bytes.NewReader(output)
			if result, err := verify.CombinedOutput(); err != nil {
				t.Fatalf("%v\n%s", err, result)
			}
		})
	}
	for name, test := range map[string]struct{ body, want string }{
		"SizeOf":     {"fn F() uint {\n    let mut values: byte[]\n    return values.SizeOf\n}", "dynamic-array SizeOf (Len * element layout stride needs the resolved layout of P24)"},
		"unsafe Ptr": {"fn F() RawPtr[byte] {\n    let mut values: byte[]\n    unsafe {\n        return values.Ptr\n    }\n}", "raw pointer to dynamic-array backing (P18-25 raw-pointer boundary)"},
		"Clear":      {"fn F() void {\n    let mut values: byte[]\n    values.Clear()\n}", "dynamic-array Clear (no schema-14 operation removes elements or shrinks the length)"},
		"RemoveAt":   {"fn F() void {\n    let mut values: byte[]\n    let removed := values.RemoveAt(0)\n    discard removed\n}", "dynamic-array RemoveAt (no schema-14 operation"},
		"ToString":   {"fn F() Result[string, StringError] {\n    let mut values: byte[]\n    return Ok(try values.ToString())\n}", "ToString() needs P23"},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := buildPackage18Source(t, "module main\n\n"+test.body+"\n", "amd64", 64)
			if module != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want named boundary %q, got %v", test.want, err)
			}
		})
	}
}

// P18-44 (§ 133): string.ToByteArray and ToRuneArray of a temporary
// string stop at named P23 boundaries; ToRuneArray of a string Place lowers
// (TestRuneArrayMaterializationSemanticIR). The § 132 invented-surface rows are
// TestPackage18UnsupportedSurface in internal/sema.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 72–73, 133
func TestPackage18CoreMaterializationBoundary(t *testing.T) {
	for _, producer := range []string{"ToByteArray", "ToRuneArray"} {
		body := "fn F() Result[uint, CollectionError] {\n    let values := try \"abc\"." + producer + "()\n    return Ok(values.Len)\n}"
		module, err := buildPackage18Source(t, "module main\n\n"+body+"\n", "amd64", 64)
		want := map[string]string{
			"ToByteArray": "string.ToByteArray materialization (no Semantic IR operation reads UTF-8 bytes before P23 string lowering)",
			"ToRuneArray": "string.ToRuneArray of a string that is not a Place",
		}[producer]
		if module != nil || err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: want %q, got %v", producer, want, err)
		}
	}
}

// P18-23..26 (§§ 102–112): lowering consumes the P18 plans and every unplanned
// form is a package-18 unsupported error with no partial module; a struct
// field, union/Result/Option payloads and by-value parameters and results
// nest T[] through P17 ownership, with the struct plan stepping to the
// dynamic-array plan; the iterator and membership boundaries stay named
// (FFI and equality are rejected by Sema, TestPackage18BoundarySurface); and
// zero-sized elements keep length, destruction and index semantics.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package18.md — §§ 102–112
func TestPackage18NestingAndBoundaries(t *testing.T) {
	text, _ := package18Module(t, "../../../testdata/semantic_ir/dynamic_array_source.sec")
	if !strings.Contains(text, `"destroy:main:struct:Bag" !16 struct field#0="destroy:array:dynamic::uint:byte"`) {
		t.Error("the struct plan does not step to its dynamic-array field plan (§ 103)")
	}
	for _, want := range []string{`func "main::Consume(own byte[])"`, `func "main::Maybe()"`, `func "main::Wrap()"`, "union.construct"} {
		if !strings.Contains(text, want) {
			t.Errorf("nesting/by-value transfer lacks %q (§§ 104–105)", want)
		}
	}
	for name, test := range map[string]struct{ body, want string }{
		"for over T[] (§ 108)": {"fn F() int {\n    let mut values: int[]\n    let mut total := 0\n    for value in values {\n        total += value\n    }\n    return total\n}", "for iteration over a collection"},
		"membership (§ 110)":   {"fn F() bool {\n    let mut values: int[]\n    return 1 in values\n}", "membership over an owning dynamic array (no P18 membership lowering)"},
		"unsafe Ptr (§ 107)":   {"fn F() RawPtr[int] {\n    let mut values: int[]\n    unsafe {\n        return values.Ptr\n    }\n}", "raw pointer to dynamic-array backing"},
	} {
		t.Run(name, func(t *testing.T) {
			module, err := buildPackage18Source(t, "module main\n\n"+test.body+"\n", "amd64", 64)
			if module != nil || err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want named boundary %q and no partial module, got module=%t err=%v", test.want, module != nil, err)
			}
		})
	}
	zero := "module main\n\ntype Unit struct {\n}\n\nfn F() Result[uint, CollectionError] {\n    let mut values: Unit[]\n    try values.Append(Unit {})\n    try values.Append(Unit {})\n    return Ok(values.Len)\n}\n"
	path := t.TempDir() + "/zero.sec"
	if err := os.WriteFile(path, []byte(zero), 0o600); err != nil {
		t.Fatal(err)
	}
	zeroText, _ := package18Module(t, path)
	if strings.Count(zeroText, "dynamic-array.append-prepared") != 2 || !strings.Contains(zeroText, "dynamic-array.length") {
		t.Errorf("zero-sized elements lost their append or length semantics (§ 112):\n%s", zeroText)
	}
}
