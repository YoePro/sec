package main

import (
	"os"
	"os/exec"

	llvmcodegen "sec/internal/codegen/llvm"
	mlircodegen "sec/internal/codegen/mlir"
	mlirtoolchain "sec/internal/mlir"
)

// runBuildCommand validates entry and lowering prerequisites before selecting
// a backend and producing target artifacts.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/compiler/initialization.md — §20 Target entry contracts.
func runBuildCommand(args []string) {
	options, ok := parseBuildCommandOptions(args, hostCompilerTarget())
	if !ok {
		printUsage()
		exitCLI(1)
	}

	input, err := os.ReadFile(options.InputFile)
	if err != nil {
		reportToolError("read", "%v", err)
		exitCLI(1)
	}

	targetDefinition, err := requireTargetCanLink(options.Target)
	if err != nil {
		reportToolError("target", "%s", err)
		exitCLI(1)
	}

	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(string(input), options.InputFile, options.Target, false)
	program := analyzed.Program
	entryErrors := validateBuildEntry(analyzed, options.InputFile)
	for _, err := range entryErrors {
		printSemaError(os.Stderr, err)
	}
	if len(entryErrors) > 0 {
		printDiagnosticSummary(diagnosticSummary{Errors: len(entryErrors), Warnings: len(analyzed.Analyzer.Warnings())})
		exitCLI(3)
	}
	requireOutputLoweringReadiness(analyzed, options.InputFile)
	llvmPath := ""
	switch options.Pipeline {
	case "llvm":
		ir, err := llvmcodegen.GenerateAnalyzed(program, analyzed.Analyzer, targetDefinition.LLVMTriple)
		if err != nil {
			reportPipelineError("codegen", err)
			exitCLI(4)
		}
		llvmPath = options.LLVMOutputFile
		removeLLVM := false
		if llvmPath == "" {
			llvmPath, removeLLVM, err = createTempOutputPath(".ll")
			if err != nil {
				reportToolError("temp file", "%v", err)
				exitCLI(1)
			}
		}
		if removeLLVM {
			defer os.Remove(llvmPath)
		}
		if err := os.WriteFile(llvmPath, []byte(ir), 0644); err != nil {
			reportToolError("write", "%v", err)
			exitCLI(1)
		}
	case "mlir":
		var cleanup func()
		llvmPath, cleanup = runMLIRBuildPipeline(analyzed, targetDefinition.LLVMTriple, options)
		defer cleanup()
	default:
		reportToolError("build", "unknown pipeline %q", options.Pipeline)
		exitCLI(1)
	}

	// rules/compiler/linking.md sections 8, 29, and 37: this is the legacy
	// direct-driver path. The selected executable and argv are not a canonical
	// LinkEnvironment/LinkPlan and must eventually be materialized from one.
	clangPath, err := exec.LookPath(options.Clang)
	if err != nil {
		reportToolError("build", "clang not found: %v", err)
		exitCLI(1)
	}

	clangArgs := []string{"-target", targetDefinition.LLVMTriple, llvmPath, "-o", options.OutputFile}
	cmd := exec.Command(clangPath, clangArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		reportToolError("build", "%v", err)
		exitCLI(5)
	}
	printDiagnosticSummary(diagnosticSummary{Warnings: len(analyzed.Analyzer.Warnings())})
}

// runMLIRBuildPipeline generates and verifies target MLIR before LLVM
// translation, preserving structured analysis failures through build reporting.
// Rules: rules/compiler/compiler_pipeline.md — §§2(8–9), 32–34;
// rules/compiler/compiler_analysis.md — §58(3).
func runMLIRBuildPipeline(analyzed analyzedProgram, triple string, options buildCommandOptions) (string, func()) {
	cleanupPaths := []string{}
	cleanup := func() {
		for _, path := range cleanupPaths {
			_ = os.Remove(path)
		}
	}

	mlirText, err := mlircodegen.GenerateAnalyzed(analyzed.Program, analyzed.Analyzer, triple)
	if err != nil {
		reportPipelineError("codegen", err)
		exitCLI(4)
	}

	mlirPath := options.MLIROutputFile
	removeMLIR := false
	if mlirPath == "" {
		mlirPath, removeMLIR, err = createTempOutputPath(".mlir")
		if err != nil {
			reportToolError("temp file", "%v", err)
			exitCLI(1)
		}
	}
	if removeMLIR {
		cleanupPaths = append(cleanupPaths, mlirPath)
	}
	if err := os.WriteFile(mlirPath, []byte(mlirText), 0644); err != nil {
		reportToolError("write", "%v", err)
		exitCLI(1)
	}

	llvmPath := options.LLVMOutputFile
	removeLLVM := false
	if llvmPath == "" {
		llvmPath, removeLLVM, err = createTempOutputPath(".ll")
		if err != nil {
			reportToolError("temp file", "%v", err)
			exitCLI(1)
		}
	}
	if removeLLVM {
		cleanupPaths = append(cleanupPaths, llvmPath)
	}

	toolchain := mlirtoolchain.NewToolchain(options.MLIRBin)
	if err := toolchain.Verify(mlirPath); err != nil {
		reportPipelineError("mlir", err)
		exitCLI(4)
	}
	if err := toolchain.TranslateToLLVMIR(mlirPath, llvmPath); err != nil {
		reportPipelineError("mlir", err)
		exitCLI(4)
	}
	return llvmPath, cleanup
}
