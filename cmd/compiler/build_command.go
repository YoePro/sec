package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	llvmcodegen "sec/internal/codegen/llvm"
	mlircodegen "sec/internal/codegen/mlir"
	mlirtoolchain "sec/internal/mlir"
	"sec/internal/project"
)

// runBuildCommand validates entry and lowering prerequisites before selecting
// a backend and producing target artifacts.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/compiler/initialization.md — §20 Target entry contracts;
// rules/projects/projects.md — §§28–31,48(1–2) counter validation and commit.
func runBuildCommand(args []string) {
	options, ok := parseBuildCommandOptions(args, hostCompilerTarget())
	if !ok {
		printUsage()
		exitCLI(1)
	}
	if versions := validateDiscoveredProjectVersions([]string{options.InputFile}); versions.Errors > 0 {
		printDiagnosticSummary(versions)
		exitCLI(3)
	}
	selection, err := selectBuildJobs(options)
	if err != nil {
		reportToolError("project", "%v", err)
		exitCLI(1)
	}
	var invocation *project.BuildInvocation
	if selection.manifest != "" {
		invocation, err = project.BeginBuildInvocation(context.Background(), selection.manifest, selection.target, options.BuildNumber)
		if err != nil {
			reportToolError("project", "%v", err)
			exitCLI(1)
		}
		defer invocation.Close()
		// A waiting parallel build can observe new manifest facts when it gets
		// the lock. Replan Variants from that snapshot before compiling them.
		refreshed, err := selectBuildJobs(options)
		if err != nil {
			reportToolError("project", "%v", err)
			exitCLI(1)
		}
		if refreshed.snapshot != invocation.Manifest().Text() || refreshed.target != selection.target {
			reportToolError("project", "manifest changed during build selection")
			exitCLI(1)
		}
		selection = refreshed
	}
	for _, job := range selection.jobs {
		runBuildVariant(job.options, invocation, job.name)
	}
	if invocation != nil {
		if err := invocation.Commit(); err != nil {
			reportToolError("build commit", "%v", err)
			exitCLI(5)
		}
		fmt.Printf("built %s version %s\n", selection.target, invocation.Rendered())
	}
}

// runBuildVariant compiles one Variant using its logical build's resolved version.
// Rules: rules/compiler/compiler_pipeline.md — §§32–34;
// rules/projects/projects.md — §§29(3–7),30(2),31(1–2),34(2).
func runBuildVariant(options buildCommandOptions, invocation *project.BuildInvocation, variant string) {
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
	if options.OutputSet == false && invocation != nil {
		if err := os.MkdirAll(filepath.Dir(options.OutputFile), 0755); err != nil {
			reportToolError("write", "%v", err)
			exitCLI(1)
		}
	}
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
		if invocation != nil {
			ir = buildVersionLLVMComment(invocation, variant) + ir
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

	if options.Pipeline == "mlir" && invocation != nil {
		data, err := os.ReadFile(llvmPath)
		if err != nil {
			reportToolError("read", "%v", err)
			exitCLI(1)
		}
		if err := os.WriteFile(llvmPath, append([]byte(buildVersionLLVMComment(invocation, variant)), data...), 0644); err != nil {
			reportToolError("write", "%v", err)
			exitCLI(1)
		}
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

// buildVersionLLVMComment records the invocation's version and artifact identity
// in retained LLVM IR without introducing Sec build-metadata declarations.
// Rules: rules/projects/projects.md — §§30(2),31,34(2); §32(4) API ownership.
func buildVersionLLVMComment(invocation *project.BuildInvocation, variant string) string {
	return fmt.Sprintf("; Sec build version %q artifact %s\n", invocation.Rendered(), invocation.ArtifactIdentity(variant))
}
