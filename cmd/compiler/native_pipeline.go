package main

import (
	"os"
	"strings"

	semantic "sec/internal/ir/semantic"
	secmlirlowering "sec/internal/lowering/secmlir"
	mlirtoolchain "sec/internal/mlir"
)

// The single compilation track (MD-101, user decision 2026-10-17):
//
//	Sema -> Semantic IR -> Sec dialect MLIR -> standard MLIR -> LLVM dialect
//	     -> LLVM IR -> clang link
//
// Sec dialect operations are used where Sec semantics need them and standard
// MLIR where it suffices; sec-mlir-opt --sec-lower-to-llvm lowers both to the
// LLVM dialect. emit-sec-mlir, emit-mlir, emit-llvm and build all run this
// one track and stop at different boundaries. A construct the track does not
// lower yet is rejected with its name; there is no second backend.
//
// Rules:
//   - rules/compiler/compiler_pipeline.md — §§ 2(8–9), 32–34, 59–60
//   - rules/compiler/initialization.md — §§ 21, 28 command entry adapter

// nativeTrackMaxPackage is the Semantic IR package level the Sec MLIR
// emitter accepts.
const nativeTrackMaxPackage = 14

// emitSecDialect builds verified-ready Sec dialect MLIR text for one analyzed
// source file and reports the Sec function identity of its command entry, or
// "" when the module defines none.
func emitSecDialect(analyzed analyzedProgram, inputFile string, target CompilerTarget) ([]byte, string) {
	module, err := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{SourceFiles: []string{inputFile}, MaxPackage: nativeTrackMaxPackage})
	if err != nil {
		reportPipelineError("semantic IR", err)
		exitCLI(4)
	}
	targetDefinition, ok := findTargetDefinition(target)
	if !ok {
		reportToolError("target", "unsupported target %s", target.String())
		exitCLI(1)
	}
	scalarPlan, err := targetDefinition.scalarPlan()
	if err != nil {
		reportToolError("target", "%v", err)
		exitCLI(1)
	}
	mlirText, err := secmlirlowering.Emit(module, scalarPlan)
	if err != nil {
		reportPipelineError("Sec MLIR lowering", err)
		exitCLI(4)
	}
	return append([]byte(mlirEmissionHeader("Sec dialect MLIR", target)), mlirText...), commandEntryIdentity(module, analyzed, inputFile)
}

// commandEntryIdentity is the Sec function identity of `main` in the input
// file's module (initialization.md § 22), or "" when it is absent.
func commandEntryIdentity(module *semantic.Module, analyzed analyzedProgram, inputFile string) string {
	entryModule, _ := entryModuleOf(analyzed.Program, inputFile)
	want := entryModule + "::main()"
	for _, function := range module.Functions {
		if string(function.ID) == want {
			return want
		}
	}
	return ""
}

// lowerToLLVMDialect verifies the Sec dialect module and lowers it to the
// LLVM dialect, returning the path of the LLVM dialect file. The caller
// removes temporary files through cleanup.
func lowerToLLVMDialect(secMLIR []byte, entry string, mlirBin string, keepSecPath string) (string, func()) {
	paths := []string{}
	cleanup := func() {
		for _, path := range paths {
			_ = os.Remove(path)
		}
	}
	secPath := keepSecPath
	if secPath == "" {
		path, remove, err := createTempOutputPath(".sec.mlir")
		if err != nil {
			reportToolError("temp file", "%v", err)
			exitCLI(1)
		}
		secPath = path
		if remove {
			paths = append(paths, path)
		}
	}
	if err := os.WriteFile(secPath, secMLIR, 0644); err != nil {
		cleanup()
		reportToolError("write", "%v", err)
		exitCLI(1)
	}
	llvmDialectPath, remove, err := createTempOutputPath(".llvm.mlir")
	if err != nil {
		cleanup()
		reportToolError("temp file", "%v", err)
		exitCLI(1)
	}
	if remove {
		paths = append(paths, llvmDialectPath)
	}
	if err := mlirtoolchain.NewToolchain(mlirBin).LowerSecToLLVM(secPath, llvmDialectPath, entry); err != nil {
		cleanup()
		reportPipelineError("Sec MLIR to LLVM lowering", err)
		exitCLI(4)
	}
	return llvmDialectPath, cleanup
}

// translateToLLVMIR translates LLVM dialect MLIR to textual LLVM IR.
func translateToLLVMIR(llvmDialectPath string, llvmPath string, mlirBin string) {
	if err := mlirtoolchain.NewToolchain(mlirBin).TranslateToLLVMIR(llvmDialectPath, llvmPath); err != nil {
		reportPipelineError("LLVM IR translation", err)
		exitCLI(4)
	}
}

// stripMLIRHeaderComments removes the leading `//` boundary comments so a
// follow-on boundary can add its own header.
func stripMLIRHeaderComments(text string) string {
	for strings.HasPrefix(text, "//") {
		newline := strings.IndexByte(text, '\n')
		if newline < 0 {
			return ""
		}
		text = text[newline+1:]
	}
	return text
}
