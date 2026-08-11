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
)

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
		"sec.dialect_version = 7 : i32",
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
		`sec.dialect_version = 7 : i32`,
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
	binDir := os.Getenv("SEC_MLIR_BIN")
	if binDir == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	output, err := Emit(representativeModule(t), testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "module.mlir")
	if err := os.WriteFile(path, output, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(filepath.Join(binDir, "sec-mlir-opt"), path, "-o", os.DevNull)
	if combined, err := command.CombinedOutput(); err != nil {
		t.Fatalf("sec-mlir-opt: %v\n%s\nGenerated:\n%s", err, combined, output)
	}
}

func TestEmittedModuleLowersTrivialCoreWithRealTool(t *testing.T) {
	binDir := os.Getenv("SEC_MLIR_BIN")
	if binDir == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
	output, err := Emit(boolModule(t), testPlan(64))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "bool.mlir")
	if err := os.WriteFile(path, output, 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(filepath.Join(binDir, "sec-mlir-opt"), path,
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

func TestEmittedModuleLowersScalarCoreFor32And64BitPlans(t *testing.T) {
	binDir := os.Getenv("SEC_MLIR_BIN")
	if binDir == "" {
		t.Skip("SEC_MLIR_BIN is not set")
	}
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
			command := exec.Command(filepath.Join(binDir, "sec-mlir-opt"), path,
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
