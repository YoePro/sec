package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	if versions := validateDiscoveredProjectManifests([]string{options.InputFile}); versions.Errors > 0 {
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
	secMLIR, entry := emitSecDialect(analyzed, options.InputFile, options.Target)
	if entry == "" {
		reportToolError("build", "the entry module of %s defines no `fn main() int`", options.InputFile)
		exitCLI(3)
	}
	llvmDialectPath, cleanup := lowerToLLVMDialect(secMLIR, entry, options.MLIRBin, options.MLIROutputFile)
	defer cleanup()
	llvmPath := options.LLVMOutputFile
	if llvmPath == "" {
		path, remove, err := createTempOutputPath(".ll")
		if err != nil {
			reportToolError("temp file", "%v", err)
			exitCLI(1)
		}
		if remove {
			defer os.Remove(path)
		}
		llvmPath = path
	}
	translateToLLVMIR(llvmDialectPath, llvmPath, options.MLIRBin)
	if invocation != nil {
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

// buildVersionLLVMComment records the invocation's version and artifact identity
// in retained LLVM IR without introducing Sec build-metadata declarations.
// Rules: rules/projects/projects.md — §§30(2),31,34(2); §32(4) API ownership.
func buildVersionLLVMComment(invocation *project.BuildInvocation, variant string) string {
	return fmt.Sprintf("; Sec build version %q artifact %s\n", invocation.Rendered(), invocation.ArtifactIdentity(variant))
}
