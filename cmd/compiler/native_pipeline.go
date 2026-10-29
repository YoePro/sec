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
//	     -> LLVM IR -> llc object (same LLVM build) -> clang link
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

// nativeTrackMaxPackage is the Semantic IR package level whose Sec MLIR the
// track lowers on to standard MLIR, LLVM and an object (emit-mlir, emit-llvm,
// build). Package-15 Places and references have no physical representation
// yet, so those routes reject them as an unsupported lowering.
const nativeTrackMaxPackage = 14

// inspectionMaxPackage is the Semantic IR package level of the high-level
// inspection routes emit-ir and emit-sec-mlir, which stop at verified
// Semantic IR and verified schema-11 Sec MLIR (MD-086).
//
// Rules: rules/compiler/compiler_pipeline.md — § 59(4)–(7) (MD-086).
const inspectionMaxPackage = 15

// emitSecDialect builds verified-ready Sec dialect MLIR text for one analyzed
// source file and reports the Sec function identity of its command entry, or
// "" when the module defines none. maxPackage is the route's Semantic IR
// package ceiling.
func emitSecDialect(analyzed analyzedProgram, inputFile string, target CompilerTarget, maxPackage uint8) ([]byte, string) {
	module, err := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{SourceFiles: []string{inputFile}, MaxPackage: maxPackage})
	if err != nil {
		reportPipelineError("semantic IR", physicalReferenceBoundary(analyzed, inputFile, maxPackage, err))
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
// removes temporary files through cleanup. Retained Sec MLIR is published only
// after successful verification/lowering.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34,59–60,88(1–3).
func lowerToLLVMDialect(secMLIR []byte, entry string, mlirBin string, keepSecPath string) (string, func()) {
	paths := []string{}
	cleanup := func() {
		for _, path := range paths {
			_ = os.Remove(path)
		}
	}
	// A retained file is a published artifact. Verify/lower a temporary
	// module first, so rejected input cannot overwrite a previous artifact.
	secPath, removeSecPath, err := createTempOutputPath(".sec.mlir")
	if err != nil {
		reportToolError("temp file", "%v", err)
		exitCLI(1)
	}
	if removeSecPath {
		paths = append(paths, secPath)
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
	if keepSecPath != "" {
		if err := os.WriteFile(keepSecPath, secMLIR, 0644); err != nil {
			cleanup()
			reportToolError("write", "%v", err)
			exitCLI(1)
		}
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

// physicalReferenceBoundary names the real limitation when a lowering route
// rejects a program that the package-15 inspection routes accept: Places and
// safe references have no physical representation or ABI yet (MD-086 § 2(4)).
// It keeps the original location, and the error stays an unsupported-lowering
// capability failure, never a source error.
func physicalReferenceBoundary(analyzed analyzedProgram, inputFile string, maxPackage uint8, err error) error {
	unsupported, ok := err.(*semantic.UnsupportedFeatureError)
	if !ok || maxPackage >= inspectionMaxPackage {
		return err
	}
	if _, inspectable := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{SourceFiles: []string{inputFile}, MaxPackage: inspectionMaxPackage}); inspectable != nil {
		return err
	}
	return &semantic.UnsupportedFeatureError{
		Feature:  "physical representation of Places and safe references (" + unsupported.Feature + "); inspect this program with emit-ir or emit-sec-mlir",
		Package:  unsupported.Package,
		Location: unsupported.Location,
	}
}
