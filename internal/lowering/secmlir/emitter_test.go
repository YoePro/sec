package secmlir

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/layout"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

// configuredSecMLIROptPath implements the absolute-tool-path acceptance rule
// shared by rules/mlir/packages/sec-mlir-dialect_package13.md section 90 and
// the Package 14 acceptance/report workflow in sections 107-108:
// SEC_MLIR_BIN names an absolute tool directory so tests remain independent of
// the package working directory selected by `go test`.
func configuredSecMLIROptPath(binDir string) (string, error) {
	if !filepath.IsAbs(binDir) {
		return "", fmt.Errorf("SEC_MLIR_BIN must be an absolute path, got %q", binDir)
	}
	return filepath.Join(binDir, "sec-mlir-opt"), nil
}

// requiredSecMLIROptPath resolves the maintained Sec MLIR verifier for package
// acceptance tests or skips when the optional toolchain is not configured.
func requiredSecMLIROptPath(t *testing.T) string {
	t.Helper()
	binDir := os.Getenv("SEC_MLIR_BIN")
	if binDir == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	path, err := configuredSecMLIROptPath(binDir)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfiguredSecMLIRBinRequiresAbsolutePathForPackage13And14(t *testing.T) {
	if _, err := configuredSecMLIROptPath("build/sec-mlir/bin"); err == nil {
		t.Fatal("relative SEC_MLIR_BIN unexpectedly accepted")
	}
	want := filepath.Join(string(filepath.Separator), "tmp", "sec-tools", "sec-mlir-opt")
	got, err := configuredSecMLIROptPath(filepath.Dir(want))
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("configured tool path = %q, want %q", got, want)
	}
}

func TestEmitIsDeterministicAndPreservesSchemaMetadata(t *testing.T) {
	module := representativeModule(t)
	first, err := Emit(module, testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	second, err := Emit(module, testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("Sec MLIR emission is not deterministic")
	}
	text := string(first)
	for _, expected := range []string{
		"sec.dialect_version = 10 : i32",
		"sec.semantic_ir_version = 1 : i32",
		`sec.module_id = "main"`,
		`sec.target_os = "linux"`,
		`sec.target_arch = "amd64"`,
		`sec.target_triple = "x86_64-pc-linux-gnu"`,
		`sec.target_endianness = "little"`,
		`#dlti.dl_entry<index, 64>`,
		`sec.function_id = "main::Foreign(int)"`,
		`@sec_fn_0`, `@sec_fn_1`, `@sec_fn_2`,
		`loc("sample\22name.sec":1:1)`,
		`sec.scalar_kind = "int"`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in:\n%s", expected, text)
		}
	}
}

func TestEmitAttachesScalarProvenanceToFunctionAndStorageBoundaries(t *testing.T) {
	output, err := Emit(representativeModule(t), testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, expected := range []string{
		`!sec.int {sec.scalar_kind = "int"}`,
		`-> (!sec.int {sec.scalar_kind = "int"})`,
		`sec.storage_id = 1 : i32, sec.scalar_kind = "int"`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in:\n%s", expected, text)
		}
	}
}

func TestEmitCarries32BitScalarPlan(t *testing.T) {
	output, err := Emit(boolModule(t), testPlan(32))
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, expected := range []string{`sec.target_arch = "armv7"`, `#dlti.dl_entry<index, 32>`} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in:\n%s", expected, text)
		}
	}
}

func TestEmitMapsSemanticTypesExactly(t *testing.T) {
	types := semantic.NewTypeTable()
	voidType := types.Intern(semantic.Type{Kind: semantic.TypeVoid, Name: "void"})
	baseInt := types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int", Signed: true, TargetSize: true})
	typeIDs := []semantic.TypeID{
		baseInt,
		types.Intern(semantic.Type{Kind: semantic.TypeUint, Name: "uint", TargetSize: true}),
		types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int32", Signed: true, BitWidth: 32}),
		types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int128", Signed: true, BitWidth: 128}),
		types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int256", Signed: true, BitWidth: 256}),
		types.Intern(semantic.Type{Kind: semantic.TypeUint, Name: "uint64", BitWidth: 64}),
		types.Intern(semantic.Type{Kind: semantic.TypeUint, Name: "uint128", BitWidth: 128}),
		types.Intern(semantic.Type{Kind: semantic.TypeUint, Name: "uint256", BitWidth: 256}),
		types.Intern(semantic.Type{Kind: semantic.TypeByte, Name: "byte", BitWidth: 8}),
		types.Intern(semantic.Type{Kind: semantic.TypeBool, Name: "bool", BitWidth: 1}),
		types.Intern(semantic.Type{Kind: semantic.TypeChar, Name: "char"}),
		types.Intern(semantic.Type{Kind: semantic.TypeRune, Name: "rune"}),
		types.Intern(semantic.Type{Kind: semantic.TypeString, Name: "string"}),
		types.Intern(semantic.Type{Kind: semantic.TypeDecimal, Name: "decimal"}),
		types.Intern(semantic.Type{Kind: semantic.TypeDecimal128, Name: "decimal128", BitWidth: 128}),
		types.Intern(semantic.Type{Kind: semantic.TypeFloat, Name: "float", TargetSize: true}),
		types.Intern(semantic.Type{Kind: semantic.TypeFloat, Name: "float32", BitWidth: 32}),
		types.Intern(semantic.Type{Kind: semantic.TypeNever, Name: "never"}),
		types.Intern(semantic.Type{Kind: semantic.TypeNamed, Name: "Count", Identity: "main::Count", Base: baseInt}),
	}
	parameters := make([]semantic.Parameter, len(typeIDs))
	for index, typeID := range typeIDs {
		parameters[index] = semantic.Parameter{Name: "p", Value: semantic.Value{ID: semantic.ValueID(index + 1), Type: typeID, Ownership: semantic.OwnershipImmediate}}
	}
	function := &semantic.Function{ID: "main::Types", Name: "Types", ReturnType: voidType, Parameters: parameters, Entry: 1,
		Blocks: []*semantic.Block{{ID: 1, Operations: []semantic.Operation{{Kind: semantic.OpReturn}}}}}
	output, err := Emit(&semantic.Module{Version: semantic.Version, Identity: "main", Types: types, Functions: []*semantic.Function{function}}, testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, expected := range []string{"!sec.int", "!sec.uint", "si32", "si128", "si256", "ui64", "ui128", "ui256", "ui8", "i1", "!sec.char", "!sec.rune", "!sec.string", "!sec.decimal", "!sec.decimal128", "!sec.float", "f32", "!sec.never", `!sec.named<"main::Count", !sec.int>`} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing mapped type %s in:\n%s", expected, text)
		}
	}
	for _, expected := range []string{
		`si128 {sec.scalar_kind = "int128"}`,
		`ui128 {sec.scalar_kind = "uint128"}`,
		`ui8 {sec.scalar_kind = "byte"}`,
		`!sec.char {sec.scalar_kind = "char"}`,
		`!sec.rune {sec.scalar_kind = "rune"}`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing scalar provenance %s in:\n%s", expected, text)
		}
	}
}

func TestEmitPreservesStorageCallsAndCFG(t *testing.T) {
	output, err := Emit(representativeModule(t), testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, expected := range []string{
		`%s1 = "sec.storage.declare"()`,
		`"sec.storage.init"(%s1, %v1)`,
		`%v3 = "sec.storage.load"(%s1)`,
		`cf.cond_br %v0, ^bb2, ^bb3`,
		`"sec.call.direct"(%v3) <{callee = @sec_fn_1}>`,
		`"sec.call.foreign"(%v3) <{callee = @sec_fn_0}>`,
		`sec.argument_actions = ["copy-trivial"]`,
		`cf.br ^bb4(%v4 : !sec.int)`,
		`^bb4(%v6: !sec.int):`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in:\n%s", expected, text)
		}
	}
}

func TestEmitSchema6CheckedIntegerGuard(t *testing.T) {
	types := semantic.NewTypeTable()
	intType := types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int128", Signed: true, BitWidth: 128})
	boolType := types.Intern(semantic.Type{Kind: semantic.TypeBool, Name: "bool", BitWidth: 1})
	reasonType := types.Intern(semantic.Type{Kind: semantic.TypeArithmeticFailureReason, Name: "ArithmeticFailureReason"})
	left := semantic.Value{ID: 0, Type: intType, Ownership: semantic.OwnershipImmediate}
	right := semantic.Value{ID: 1, Type: intType, Ownership: semantic.OwnershipImmediate}
	result := semantic.Value{ID: 2, Type: intType, Ownership: semantic.OwnershipImmediate}
	failed := semantic.Value{ID: 3, Type: boolType, Ownership: semantic.OwnershipImmediate}
	reason := semantic.Value{ID: 4, Type: reasonType, Ownership: semantic.OwnershipImmediate}
	reasonArgument := semantic.Value{ID: 5, Type: reasonType, Ownership: semantic.OwnershipImmediate}
	function := &semantic.Function{ID: "main::Add(int128,int128)", Name: "Add", ReturnType: intType, Entry: 0,
		Parameters: []semantic.Parameter{{Name: "left", Value: left}, {Name: "right", Value: right}},
		Blocks: []*semantic.Block{
			{ID: 0, Operations: []semantic.Operation{
				{Kind: semantic.OpIntBinaryChecked, Operands: []semantic.ValueID{0, 1}, Results: []semantic.Value{result, failed, reason}, IntegerBinary: semantic.IntegerCheckedAdd, Operator: "+"},
				{Kind: semantic.OpCondBranch, Operands: []semantic.ValueID{3}, Successors: []semantic.BranchTarget{{Block: 1, Arguments: []semantic.ValueID{4}}, {Block: 2}}},
			}},
			{ID: 1, Parameters: []semantic.Value{reasonArgument}, Operations: []semantic.Operation{{Kind: semantic.OpArithmeticFailure, Operands: []semantic.ValueID{5}, FailureCategory: semantic.ArithmeticFailureOverflow, Operator: "+"}}},
			{ID: 2, Operations: []semantic.Operation{{Kind: semantic.OpReturn, Operands: []semantic.ValueID{2}}}},
		},
	}
	output, err := Emit(&semantic.Module{Version: semantic.Version, Identity: "main", Types: types, Functions: []*semantic.Function{function}}, testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, expected := range []string{
		`sec.dialect_version = 10 : i32`,
		`%v2, %v3, %v4 = "sec.int.binary_checked"(%v0, %v1) <{kind = "add"}> : (si128, si128) -> (si128, i1, !sec.arithmetic_failure_reason)`,
		`cf.cond_br %v3, ^bb1(%v4 : !sec.arithmetic_failure_reason), ^bb2`,
		`^bb1(%v5: !sec.arithmetic_failure_reason):`,
		`"sec.fail.arithmetic"(%v5) {sec.operator = "+"} : (!sec.arithmetic_failure_reason) -> ()`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("missing %q in:\n%s", expected, text)
		}
	}
}

func TestEmitInvokesSemanticVerifier(t *testing.T) {
	module := representativeModule(t)
	module.Functions[2].Blocks[0].Operations = nil
	if _, err := Emit(module, testPlan(64)); err == nil || !strings.Contains(err.Error(), "verify Semantic IR") {
		t.Fatalf("error = %v", err)
	}
}

func TestEmitRejectsUnresolvedScalarPlan(t *testing.T) {
	if _, err := Emit(representativeModule(t), layout.ResolvedScalarPlan{}); err == nil ||
		!strings.Contains(err.Error(), "validate scalar plan") {
		t.Fatalf("error = %v", err)
	}
}

func TestEmitRejectsVerifiedUnsupportedType(t *testing.T) {
	types := semantic.NewTypeTable()
	unsupported := types.Intern(semantic.Type{Kind: semantic.TypeKind("future"), Name: "future"})
	function := &semantic.Function{ID: "main::Future", Name: "Future", ReturnType: unsupported, Extern: true}
	_, err := Emit(&semantic.Module{Version: semantic.Version, Identity: "main", Types: types, Functions: []*semantic.Function{function}}, testPlan(64))
	var unsupportedError *UnsupportedLoweringError
	if !errors.As(err, &unsupportedError) {
		t.Fatalf("error = %v", err)
	}
}

func TestEmitterDependencyBoundary(t *testing.T) {
	data, err := os.ReadFile("emitter.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"internal/ast", "internal/lexer", "internal/parser", "internal/sema", "internal/codegen"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("emitter imports forbidden package %s", forbidden)
		}
	}
}

func TestEmittedModuleVerifiesWithRealTool(t *testing.T) {
	tool := requiredSecMLIROptPath(t)
	output, err := Emit(representativeModule(t), testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "module.mlir")
	if err := os.WriteFile(path, output, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(tool, path, "-o", os.DevNull)
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, combined, output)
	}
}

func TestEmittedModuleLowersTrivialCoreWithRealTool(t *testing.T) {
	tool := requiredSecMLIROptPath(t)
	output, err := Emit(boolModule(t), testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bool.mlir")
	if err := os.WriteFile(path, output, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(tool, path,
		"--sec-lower-trivial-core")
	lowered, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, lowered, output)
	}
	text := string(lowered)
	if !strings.Contains(text, "arith.constant true") ||
		strings.Contains(text, "sec.const.bool") || strings.Contains(text, "llvm.") {
		t.Fatalf("unexpected trivial-core output:\n%s", text)
	}
}

// rules/declarations/struct.md section 4 and the maintained schema-v9 struct
// contract, retained by lowering v10, define the verified raw tag-metadata
// bridge from Semantic IR to Sec MLIR structs.
func TestEmitPackage13StructsEndToEnd(t *testing.T) {
	source := `module main
type Pair struct { Wide: int128 ` + "`wire:\"signed\\\"little\" json:\"wide value\"`" + `, Limit: uint256 }
type TargetPair struct { Count: int, Limit: uint }
type Position union { Unknown Known { X: int128, Y: uint256 } }
fn Build(base: Pair) int128 {
  let mut value := Pair { base..., Wide: 5 }
  value.Limit = 9
  return value.Wide
}
fn BuildTarget(base: TargetPair) uint {
  let value := TargetPair { base..., Count: 3 }
  return value.Limit
}
fn Read(position: Position, zero: int128) int128 {
  return match position {
    Unknown => zero
    Known(payload) => payload.X
  }
}`
	p := parser.New(lexer.NewWithFile(source, "package13-emitter.sec"))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzer()
	if errors := analyzer.Analyze(parsed.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{RequestedModule: "main", SourceFiles: []string{"package13-emitter.sec"}, MaxPackage: 13})
	if err != nil {
		t.Fatalf("semantic build: %v", err)
	}
	binDir := os.Getenv("SEC_MLIR_BIN")
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatalf("emit: %v", err)
			}
			text := string(output)
			for _, expected := range []string{
				`sec.dialect_version = 10 : i32`,
				`!sec.struct<identity = "main::Pair"`,
				`!sec.struct<identity = "main::TargetPair"`,
				`#sec.struct_field<ordinal = 0, name = "Count", type = !sec.int`,
				`#sec.struct_field<ordinal = 1, name = "Limit", type = !sec.uint`,
				`#sec.struct_tag<key = "wire", value = "signed\5C\22little">`,
				`#sec.struct_tag<key = "json", value = "wide value">`,
				`"sec.struct.spread_fields"`,
				`"sec.struct.construct"`,
				`"sec.struct.replace_field"`,
				`"sec.struct.extract"`,
				`!sec.storage<!sec.struct`,
				`!sec.struct<identity = "main::Position#1$payload"`,
				`"sec.union.unwrap_field"`,
			} {
				if !strings.Contains(text, expected) {
					t.Errorf("missing %q in:\n%s", expected, text)
				}
			}
			if binDir == "" {
				return
			}
			tool, err := configuredSecMLIROptPath(binDir)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "package13.mlir")
			if err := os.WriteFile(path, output, 0600); err != nil {
				t.Fatal(err)
			}
			// Package 13 sections 82 and 90 require its struct representation
			// to keep verifying under the current schema on both scalar plans.
			command := exec.Command(tool, path,
				"--sec-verify-union-guards", "--sec-verify-match-cfg",
				"--sec-lower-scalar-core", "--sec-lower-trivial-core")
			lowered, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, lowered, output)
			}
			loweredText := string(lowered)
			for _, expected := range []string{
				fmt.Sprintf(`type = si%d`, width),
				fmt.Sprintf(`type = ui%d`, width),
			} {
				if !strings.Contains(loweredText, expected) {
					t.Errorf("lowered %d-bit module missing %q:\n%s", width, expected, loweredText)
				}
			}
			for _, forbidden := range []string{"!sec.int", "!sec.uint", "unrealized_conversion_cast", "llvm."} {
				if strings.Contains(loweredText, forbidden) {
					t.Errorf("lowered %d-bit module contains %q:\n%s", width, forbidden, loweredText)
				}
			}
		})
	}
}

// TestEmitPackage14ArraysEndToEnd covers the complete verified Semantic IR to
// schema-v10 fixed-array mapping while keeping arrays high-level and compact.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 58-85 and 102
//   - rules/mlir/lowering-versions/sec_mlir_lowering_v10.md — sections 1-23
func TestEmitPackage14ArraysEndToEnd(t *testing.T) {
	module := package14EmitterModule(t)
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatalf("emit: %v", err)
			}
			text := string(output)
			for _, expected := range []string{
				`sec.dialect_version = 10 : i32`,
				`!sec.array<!sec.int, "0">`,
				`!sec.array<!sec.int, "2">`,
				`!sec.array<!sec.int, "3">`,
				`"sec.array.construct"`,
				`segment_actions = ["construct-direct", "copy-trivial"]`,
				`segment_kinds = ["element", "spread"]`,
				`segment_lengths = ["1", "2"]`,
				`"sec.array.default"()`,
				`"sec.array.len"`,
				`"sec.array.index_in_bounds"`,
				`index_signed = true`,
				`"sec.array.extract"`,
				`action = "copy-trivial", bounds_kind = "runtime-check", bounds_proof = "guarded"`,
				`"sec.array.replace"`,
				`bounds_kind = "proven-safe", bounds_proof = "analysis"`,
				`"sec.fail.bounds"() {operation = "fixed-array-index"}`,
			} {
				if !strings.Contains(text, expected) {
					t.Errorf("missing %q in:\n%s", expected, text)
				}
			}
			for _, forbidden := range []string{"!llvm.array", "llvm.", "memref<", "getelementptr", "undef", "poison", "@malloc", "@calloc"} {
				if strings.Contains(text, forbidden) {
					t.Errorf("schema-v10 array module contains %q:\n%s", forbidden, text)
				}
			}

			binDir := os.Getenv("SEC_MLIR_BIN")
			if binDir == "" {
				return
			}
			tool, err := configuredSecMLIROptPath(binDir)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "package14.mlir")
			if err := os.WriteFile(path, output, 0600); err != nil {
				t.Fatal(err)
			}
			verified, err := exec.Command(tool, path, "--sec-verify-array-index-guards").CombinedOutput()
			if err != nil {
				t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, verified, output)
			}
		})
	}
}

func TestEmitPackage14ArrayLengthUsesTargetUintBoundary(t *testing.T) {
	types := semantic.NewTypeTable()
	voidType := types.Intern(semantic.Type{Kind: semantic.TypeVoid, Name: "void"})
	intType := types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int32", Signed: true, BitWidth: 32})
	arrayType := types.Intern(semantic.Type{Kind: semantic.TypeArray, Name: "int32[4294967296]", Element: intType, Length: "4294967296"})
	function := &semantic.Function{
		ID: "main::Boundary(int32[4294967296])", Name: "Boundary", ReturnType: voidType, Entry: 0,
		Parameters: []semantic.Parameter{{Name: "value", Value: semantic.Value{ID: 1, Type: arrayType, Ownership: semantic.OwnershipImmediate}}},
		Blocks:     []*semantic.Block{{ID: 0, Operations: []semantic.Operation{{Kind: semantic.OpReturn}}}},
	}
	module := &semantic.Module{Version: semantic.Version, Identity: "main", Types: types, Functions: []*semantic.Function{function}}
	if _, err := Emit(module, testPlan(32)); err == nil || !strings.Contains(err.Error(), "fixed-array length 4294967296 overflows target uint32") {
		t.Fatalf("32-bit emit error = %v", err)
	}
	if _, err := Emit(module, testPlan(64)); err != nil {
		t.Fatalf("64-bit emit: %v", err)
	}
}

// TestEmitPackage14SourceIntegrationMatrix proves every Package 14 section-102
// case through source parsing, Sema, verified Semantic IR, and schema-v10 text.
// No generated IR is patched or reconstructed by the test.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — section 102
//   - rules/mlir/lowering-versions/sec_mlir_lowering_v10.md — section 23
func TestEmitPackage14SourceIntegrationMatrix(t *testing.T) {
	module := package14SourceIntegrationModule(t)

	type operationMinimums map[semantic.OpKind]int
	want := map[string]operationMinimums{
		"Literal":       {semantic.OpArrayConstruct: 1},
		"Spread":        {semantic.OpArrayConstruct: 1},
		"Zero":          {semantic.OpArrayConstruct: 1},
		"Defaulted":     {semantic.OpArrayDefault: 1},
		"ArrayInStruct": {semantic.OpArrayConstruct: 1, semantic.OpStructConstruct: 1},
		"StructInArray": {semantic.OpArrayConstruct: 1, semantic.OpStructConstruct: 2},
		"Nested":        {semantic.OpArrayConstruct: 3},
		"Identity":      {},
		"Length":        {semantic.OpArrayLength: 1},
		"Constant":      {semantic.OpArrayExtract: 1},
		"Dynamic":       {semantic.OpArrayIndexInBounds: 1, semantic.OpArrayExtract: 1, semantic.OpBoundsFailure: 1},
		"Fallible":      {semantic.OpArrayIndexInBounds: 1, semantic.OpArrayExtract: 1, semantic.OpEnumConstant: 1, semantic.OpResultErr: 1},
		"LocalHandler":  {semantic.OpArrayIndexInBounds: 1, semantic.OpArrayExtract: 1, semantic.OpEnumConstant: 1},
		"Replace":       {semantic.OpArrayReplace: 1, semantic.OpStorageStore: 1},
		"NestedReplace": {semantic.OpArrayIndexInBounds: 2, semantic.OpArrayExtract: 1, semantic.OpArrayReplace: 2, semantic.OpStorageStore: 1},
	}
	seen := map[string]bool{}
	for _, function := range module.Functions {
		expected, relevant := want[function.Name]
		if !relevant {
			continue
		}
		seen[function.Name] = true
		counts := map[semantic.OpKind]int{}
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				counts[operation.Kind]++
			}
		}
		for kind, minimum := range expected {
			if counts[kind] < minimum {
				t.Errorf("%s has %d %s operations, want at least %d\n%s", function.Name, counts[kind], kind, minimum, semantic.Format(module))
			}
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("missing source-built function %s", name)
		}
	}

	output, err := Emit(module, testPlan(64))
	if err != nil {
		t.Fatalf("schema-v10 emit: %v", err)
	}
	text := string(output)
	for _, expected := range []string{
		`sec.dialect_version = 10 : i32`,
		`!sec.array<si32, "0">`,
		`!sec.array<si32, "3">`,
		`!sec.array<!sec.struct<identity = "main::Pair"`,
		`!sec.struct<identity = "main::Holder"`,
		`type = !sec.array<si128, "2">`,
		`!sec.array<!sec.array<si32, "2">, "2">`,
		`segment_kinds = ["element", "spread", "element"]`,
		`"sec.array.default"`,
		`"sec.array.len"`,
		`bounds_kind = "proven-safe", bounds_proof = "constant"`,
		`bounds_kind = "runtime-check", bounds_proof = "guarded"`,
		`"sec.fail.bounds"`,
		`"sec.result.err"`,
		`"sec.array.replace"`,
		`!sec.storage<!sec.array`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("schema-v10 integration output missing %q:\n%s", expected, text)
		}
	}
}

func package14SourceIntegrationModule(t *testing.T) *semantic.Module {
	t.Helper()
	sourcePath := filepath.Join("..", "..", "..", "testdata", "semantic_ir", "package14_schema10_integration.sec")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(source), sourcePath))
	parsed := p.Parse()
	if parsed.HasErrors {
		t.Fatalf("parse: %v", p.Errors())
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(testPlan(64))
	if diagnostics := analyzer.Analyze(parsed.Program); len(diagnostics) != 0 {
		t.Fatalf("sema: %v", diagnostics)
	}
	module, err := semantic.Build(parsed.Program, analyzer, semantic.BuildOptions{
		RequestedModule: "main", SourceFiles: []string{sourcePath}, MaxPackage: 14,
	})
	if err != nil {
		t.Fatalf("semantic build: %v", err)
	}
	if err := semantic.Verify(module); err != nil {
		t.Fatalf("semantic verify: %v\n%s", err, semantic.Format(module))
	}
	return module
}

// TestEmitPackage14SourceModuleVerifiesOn32And64BitPlans proves that the same
// source-built Package 14 module survives the maintained absolute verifier and
// scalar-layout pipeline without selecting an array ABI or runtime strategy.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 102, 107
//   - rules/mlir/lowering-versions/sec_mlir_lowering_v10.md — sections 23, 25
func TestEmitPackage14SourceModuleVerifiesOn32And64BitPlans(t *testing.T) {
	tool := requiredSecMLIROptPath(t)
	module := package14SourceIntegrationModule(t)
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatalf("schema-v10 emit: %v", err)
			}
			path := filepath.Join(t.TempDir(), "package14.mlir")
			if err := os.WriteFile(path, output, 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(tool, path,
				"--sec-verify-array-index-guards", "--sec-lower-scalar-core")
			lowered, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, lowered, output)
			}
			text := string(lowered)
			for _, expected := range []string{
				`sec.dialect_version = 10 : i32`,
				fmt.Sprintf("ui%d", width),
				`!sec.array<si32, "3">`,
				`!sec.array<!sec.array<si32, "2">, "2">`,
				`"sec.array.construct"`,
				`"sec.array.len"`,
				`"sec.array.index_in_bounds"`,
				`"sec.array.extract"`,
				`"sec.array.replace"`,
			} {
				if !strings.Contains(text, expected) {
					t.Errorf("lowered schema-v10 module missing %q:\n%s", expected, text)
				}
			}
			for _, forbidden := range []string{
				"!sec.int", "!sec.uint", "unrealized_conversion_cast",
				"memref<", "!llvm.array", "llvm.", "getelementptr",
				"@malloc", "@calloc", "sec.runtime", "runtime.",
			} {
				if strings.Contains(text, forbidden) {
					t.Errorf("unexpected physical/runtime lowering %q in:\n%s", forbidden, text)
				}
			}
		})
	}
}

// TestPackage14SuccessfulModuleContainsNoPhysicalArrayShortcut consolidates the
// Package 14 full-initialization and representation boundary on the maintained
// source-built integration module. Verification proves readable construction;
// schema emission must retain semantic arrays without inventing layout/runtime.
//
// Rules:
//   - rules/mlir/packages/sec-mlir-dialect_package14.md — sections 27, 104, 106
//   - rules/mlir/semantic-ir/sec_semantic_ir_fixed_array_v1.md — "Full initialization"
func TestPackage14SuccessfulModuleContainsNoPhysicalArrayShortcut(t *testing.T) {
	module := package14SourceIntegrationModule(t)
	constructs, defaults := 0, 0
	for _, function := range module.Functions {
		for _, block := range function.Blocks {
			for _, operation := range block.Operations {
				switch operation.Kind {
				case semantic.OpArrayConstruct:
					constructs++
					total := new(big.Int)
					for _, length := range operation.ArraySegmentLengths {
						part, ok := new(big.Int).SetString(length, 10)
						if !ok {
							t.Fatalf("non-decimal construction segment %q", length)
						}
						total.Add(total, part)
					}
					if total.String() != operation.ArrayLength || len(operation.Operands) != len(operation.ArraySegmentLengths) {
						t.Fatalf("partial array construction escaped verification: %#v", operation)
					}
					for _, action := range operation.ArrayActions {
						if action != semantic.ArrayActionConstructDirect && action != semantic.ArrayActionCopyTrivial {
							t.Fatalf("deferred array construction action %q escaped Package 14", action)
						}
					}
				case semantic.OpArrayDefault:
					defaults++
					if len(operation.Operands) != 0 {
						t.Fatalf("array.default is not compact and complete: %#v", operation)
					}
				}
			}
		}
	}
	if constructs == 0 || defaults == 0 {
		t.Fatalf("integration module lacks construction/default coverage: constructs=%d defaults=%d", constructs, defaults)
	}

	semanticText := strings.ToLower(semantic.Format(module))
	for _, forbidden := range []string{"undef", "poison", "partial-readable", "uninitialized"} {
		if strings.Contains(semanticText, forbidden) {
			t.Errorf("verified Semantic IR contains forbidden state %q:\n%s", forbidden, semanticText)
		}
	}

	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(module, testPlan(width))
			if err != nil {
				t.Fatalf("emit: %v", err)
			}
			text := strings.ToLower(string(output))
			for _, forbidden := range []string{
				"undef", "poison", "memref<", "memref.alloc", "!llvm.array",
				"llvm.", "llvm.insertvalue", "getelementptr", "alloca", "@malloc",
				"@calloc", "sec.alloc", "sec.runtime", "@sec_runtime", "runtime.",
				"call @", "func.call", "llvm.intr.trap",
			} {
				if strings.Contains(text, forbidden) {
					t.Errorf("schema-v10 module contains forbidden shortcut %q:\n%s", forbidden, text)
				}
			}
			for _, line := range strings.Split(text, "\n") {
				if (strings.Contains(line, `"sec.array.index_in_bounds"`) ||
					strings.Contains(line, `"sec.array.extract"`) ||
					strings.Contains(line, `"sec.array.replace"`)) &&
					strings.Contains(line, ", i64") {
					t.Errorf("array operation hard-codes a signless semantic i64 index: %s", line)
				}
			}
		})
	}
}

func TestEmittedModuleLowersScalarCoreFor32And64BitPlans(t *testing.T) {
	tool := requiredSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		t.Run(strconv.Itoa(int(width)), func(t *testing.T) {
			output, err := Emit(representativeModule(t), testPlan(width))
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "module.mlir")
			if err := os.WriteFile(path, output, 0600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command(tool, path,
				"--sec-lower-scalar-core", "--sec-lower-scalar-core")
			lowered, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, lowered, output)
			}
			text := string(lowered)
			for _, expected := range []string{
				fmt.Sprintf("memref<si%d>", width),
				"call @sec_fn_1",
				`"sec.call.foreign"`,
			} {
				if !strings.Contains(text, expected) {
					t.Errorf("missing %q in:\n%s", expected, text)
				}
			}
			for _, forbidden := range []string{"!sec.int", "sec.call.direct", "unrealized_conversion_cast", "llvm."} {
				if strings.Contains(text, forbidden) {
					t.Errorf("unexpected %q in:\n%s", forbidden, text)
				}
			}
		})
	}
}

func boolModule(t *testing.T) *semantic.Module {
	t.Helper()
	types := semantic.NewTypeTable()
	boolType := types.Intern(semantic.Type{Kind: semantic.TypeBool, Name: "bool", BitWidth: 1})
	value := true
	result := semantic.Value{ID: 0, Type: boolType, Ownership: semantic.OwnershipImmediate}
	function := &semantic.Function{
		ID: "main::True()", Name: "True", ReturnType: boolType, Entry: 1,
		Blocks: []*semantic.Block{{ID: 1, Operations: []semantic.Operation{
			{Kind: semantic.OpConstBool, Results: []semantic.Value{result}, Bool: &value},
			{Kind: semantic.OpReturn, Operands: []semantic.ValueID{result.ID}},
		}}},
	}
	module := &semantic.Module{Version: semantic.Version, Identity: "main", Types: types, Functions: []*semantic.Function{function}}
	if err := semantic.Verify(module); err != nil {
		t.Fatalf("test module: %v", err)
	}
	return module
}

func package14EmitterModule(t *testing.T) *semantic.Module {
	t.Helper()
	types := semantic.NewTypeTable()
	intType := types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int", Signed: true, TargetSize: true})
	indexType := types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int128", Signed: true, BitWidth: 128})
	uintType := types.Intern(semantic.Type{Kind: semantic.TypeUint, Name: "uint", TargetSize: true})
	boolType := types.Intern(semantic.Type{Kind: semantic.TypeBool, Name: "bool", BitWidth: 1})
	array0 := types.Intern(semantic.Type{Kind: semantic.TypeArray, Name: "int[0]", Element: intType, Length: "0"})
	array2 := types.Intern(semantic.Type{Kind: semantic.TypeArray, Name: "int[2]", Element: intType, Length: "2"})
	array3 := types.Intern(semantic.Type{Kind: semantic.TypeArray, Name: "int[3]", Element: intType, Length: "3"})
	location := semantic.Location{File: "package14-emitter.sec", Line: 1, Column: 1}

	build := &semantic.Function{
		ID: "main::Build(int[2],int)", Name: "Build", ReturnType: array3, Entry: 0, Location: location,
		Parameters: []semantic.Parameter{
			{Name: "source", Value: semantic.Value{ID: 1, Type: array2, Ownership: semantic.OwnershipImmediate}},
			{Name: "element", Value: semantic.Value{ID: 2, Type: intType, Ownership: semantic.OwnershipImmediate}},
		},
		Blocks: []*semantic.Block{{ID: 0, Operations: []semantic.Operation{
			{Kind: semantic.OpArrayConstruct, Operands: []semantic.ValueID{2, 1}, Results: []semantic.Value{{ID: 3, Type: array3, Ownership: semantic.OwnershipImmediate}}, ArrayElementType: intType, ArrayLength: "3", ArraySegmentKinds: []semantic.ArrayConstructSegmentKind{semantic.ArraySegmentElement, semantic.ArraySegmentSpread}, ArraySegmentLengths: []string{"1", "2"}, ArrayActions: []semantic.ArrayTransferAction{semantic.ArrayActionConstructDirect, semantic.ArrayActionCopyTrivial}, Location: location},
			{Kind: semantic.OpArrayDefault, Results: []semantic.Value{{ID: 4, Type: array0, Ownership: semantic.OwnershipImmediate}}, ArrayElementType: intType, ArrayLength: "0", Location: location},
			{Kind: semantic.OpArrayDefault, Results: []semantic.Value{{ID: 5, Type: array3, Ownership: semantic.OwnershipImmediate}}, ArrayElementType: intType, ArrayLength: "3", Location: location},
			{Kind: semantic.OpArrayLength, Operands: []semantic.ValueID{3}, Results: []semantic.Value{{ID: 6, Type: uintType, Ownership: semantic.OwnershipImmediate}}, ArrayLength: "3", Location: location},
			{Kind: semantic.OpReturn, Operands: []semantic.ValueID{3}, Location: location},
		}}},
	}

	runtime := &semantic.Function{
		ID: "main::Update(int[3],int128,int)", Name: "Update", ReturnType: array3, Entry: 0, Location: location,
		Parameters: []semantic.Parameter{
			{Name: "array", Value: semantic.Value{ID: 1, Type: array3, Ownership: semantic.OwnershipImmediate}},
			{Name: "index", Value: semantic.Value{ID: 2, Type: indexType, Ownership: semantic.OwnershipImmediate}},
			{Name: "newValue", Value: semantic.Value{ID: 3, Type: intType, Ownership: semantic.OwnershipImmediate}},
		},
		Blocks: []*semantic.Block{
			{ID: 0, Operations: []semantic.Operation{
				{Kind: semantic.OpArrayIndexInBounds, Operands: []semantic.ValueID{1, 2}, Results: []semantic.Value{{ID: 4, Type: boolType, Ownership: semantic.OwnershipImmediate}}, ArrayIndexSigned: true, Location: location},
				{Kind: semantic.OpCondBranch, Operands: []semantic.ValueID{4}, Successors: []semantic.BranchTarget{{Block: 1}, {Block: 2}}, Location: location},
			}},
			{ID: 1, Operations: []semantic.Operation{
				{Kind: semantic.OpArrayExtract, Operands: []semantic.ValueID{1, 2}, Results: []semantic.Value{{ID: 5, Type: intType, Ownership: semantic.OwnershipImmediate}}, ArrayCheckKind: semantic.ArrayIndexRuntimeCheck, ArrayProofKind: semantic.ArrayIndexProofGuarded, ArrayGuard: 4, ArrayActions: []semantic.ArrayTransferAction{semantic.ArrayActionCopyTrivial}, Location: location},
				{Kind: semantic.OpArrayReplace, Operands: []semantic.ValueID{1, 2, 3}, Results: []semantic.Value{{ID: 6, Type: array3, Ownership: semantic.OwnershipImmediate}}, ArrayCheckKind: semantic.ArrayIndexRuntimeCheck, ArrayProofKind: semantic.ArrayIndexProofGuarded, ArrayGuard: 4, Location: location},
				{Kind: semantic.OpReturn, Operands: []semantic.ValueID{6}, Location: location},
			}},
			{ID: 2, Operations: []semantic.Operation{{Kind: semantic.OpBoundsFailure, ArrayOperation: "fixed-array-index", Location: location}}},
		},
	}

	proven := &semantic.Function{
		ID: "main::Read(int[3],int128)", Name: "Read", ReturnType: intType, Entry: 0, Location: location,
		Parameters: []semantic.Parameter{
			{Name: "array", Value: semantic.Value{ID: 1, Type: array3, Ownership: semantic.OwnershipImmediate}},
			{Name: "index", Value: semantic.Value{ID: 2, Type: indexType, Ownership: semantic.OwnershipImmediate}},
		},
		Blocks: []*semantic.Block{{ID: 0, Operations: []semantic.Operation{
			{Kind: semantic.OpArrayExtract, Operands: []semantic.ValueID{1, 2}, Results: []semantic.Value{{ID: 3, Type: intType, Ownership: semantic.OwnershipImmediate}}, ArrayCheckKind: semantic.ArrayIndexProvenSafe, ArrayProofKind: semantic.ArrayIndexProofAnalysis, ArrayActions: []semantic.ArrayTransferAction{semantic.ArrayActionCopyTrivial}, Location: location},
			{Kind: semantic.OpReturn, Operands: []semantic.ValueID{3}, Location: location},
		}}},
	}

	module := &semantic.Module{Version: semantic.Version, Identity: "main", SourceFiles: []string{location.File}, Types: types, Functions: []*semantic.Function{build, runtime, proven}}
	if err := semantic.Verify(module); err != nil {
		t.Fatalf("test module: %v\n%s", err, semantic.Format(module))
	}
	return module
}

func testPlan(width uint16) layout.ResolvedScalarPlan {
	arch := "amd64"
	triple := "x86_64-pc-linux-gnu"
	if width == 32 {
		arch = "armv7"
		triple = "armv7-unknown-linux-gnueabihf"
	}
	return layout.ResolvedScalarPlan{
		TargetOS: "linux", TargetArch: arch, LLVMTriple: triple,
		ABI: "gnu", Profile: "hosted", PointerWidthBits: width,
		Endianness: layout.LittleEndian,
	}
}

func representativeModule(t *testing.T) *semantic.Module {
	t.Helper()
	types := semantic.NewTypeTable()
	voidType := types.Intern(semantic.Type{Kind: semantic.TypeVoid, Name: "void"})
	intType := types.Intern(semantic.Type{Kind: semantic.TypeInt, Name: "int", Signed: true, TargetSize: true})
	boolType := types.Intern(semantic.Type{Kind: semantic.TypeBool, Name: "bool", BitWidth: 1})
	location := semantic.Location{File: `sample"name.sec`, Line: 1, Column: 1}
	foreign := &semantic.Function{ID: "main::Foreign(int)", Name: "Foreign", LinkName: "foreign", ReturnType: intType, Extern: true, ABI: "C", Location: location,
		Parameters: []semantic.Parameter{{Name: "value", Value: semantic.Value{ID: 0, Type: intType, Ownership: semantic.OwnershipImmediate}}}}
	calleeParameter := semantic.Value{ID: 0, Type: intType, Ownership: semantic.OwnershipImmediate}
	callee := &semantic.Function{ID: "main::Identity(int)", Name: "Identity", ReturnType: intType, Entry: 1, Location: location,
		Parameters: []semantic.Parameter{{Name: "value", Value: calleeParameter}},
		Blocks:     []*semantic.Block{{ID: 1, Operations: []semantic.Operation{{Kind: semantic.OpReturn, Operands: []semantic.ValueID{0}, Location: location}}}}}
	condition := semantic.Value{ID: 0, Type: boolType, Ownership: semantic.OwnershipImmediate}
	input := semantic.Value{ID: 1, Type: intType, Ownership: semantic.OwnershipImmediate}
	constant := semantic.Value{ID: 2, Type: intType, Ownership: semantic.OwnershipImmediate}
	loaded := semantic.Value{ID: 3, Type: intType, Ownership: semantic.OwnershipImmediate}
	directResult := semantic.Value{ID: 4, Type: intType, Ownership: semantic.OwnershipImmediate}
	foreignResult := semantic.Value{ID: 5, Type: intType, Ownership: semantic.OwnershipImmediate}
	merged := semantic.Value{ID: 6, Type: intType, Ownership: semantic.OwnershipImmediate}
	caller := &semantic.Function{ID: "main::Choose(bool,int)", Name: "Choose", ReturnType: intType, Entry: 1, Location: location,
		Parameters: []semantic.Parameter{{Name: "condition", Value: condition}, {Name: "input", Value: input}},
		Storages:   []semantic.Storage{{ID: 1, Name: "value", Type: intType, Mutable: true, Class: semantic.StorageLocalAutomatic, Location: location}},
		Blocks: []*semantic.Block{
			{ID: 1, Operations: []semantic.Operation{
				{Kind: semantic.OpConstInt, Results: []semantic.Value{constant}, Integer: big.NewInt(7), Location: location},
				{Kind: semantic.OpStorageDeclare, Storage: 1, Location: location},
				{Kind: semantic.OpStorageInit, Storage: 1, Operands: []semantic.ValueID{1}, Location: location},
				{Kind: semantic.OpStorageLoad, Storage: 1, Results: []semantic.Value{loaded}, Location: location},
				{Kind: semantic.OpCondBranch, Operands: []semantic.ValueID{0}, Successors: []semantic.BranchTarget{{Block: 2}, {Block: 3}}, Location: location},
			}},
			{ID: 2, Operations: []semantic.Operation{
				{Kind: semantic.OpDirectCall, Callee: callee.ID, Operands: []semantic.ValueID{3}, Results: []semantic.Value{directResult}, ArgumentActions: []semantic.ArgumentAction{semantic.ArgumentCopyTrivial}, Location: location},
				{Kind: semantic.OpBranch, Successors: []semantic.BranchTarget{{Block: 4, Arguments: []semantic.ValueID{4}}}, Location: location},
			}},
			{ID: 3, Operations: []semantic.Operation{
				{Kind: semantic.OpForeignCall, Callee: foreign.ID, Operands: []semantic.ValueID{3}, Results: []semantic.Value{foreignResult}, ArgumentActions: []semantic.ArgumentAction{semantic.ArgumentCopyTrivial}, Location: location},
				{Kind: semantic.OpBranch, Successors: []semantic.BranchTarget{{Block: 4, Arguments: []semantic.ValueID{5}}}, Location: location},
			}},
			{ID: 4, Parameters: []semantic.Value{merged}, Operations: []semantic.Operation{{Kind: semantic.OpReturn, Operands: []semantic.ValueID{6}, Location: location}}},
		},
	}
	module := &semantic.Module{Version: semantic.Version, Identity: "main", SourceFiles: []string{location.File}, Types: types, Functions: []*semantic.Function{foreign, callee, caller}}
	if err := semantic.Verify(module); err != nil {
		t.Fatalf("test module: %v", err)
	}
	_ = voidType
	return module
}
