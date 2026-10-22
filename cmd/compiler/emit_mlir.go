package main

import (
	"fmt"
	"os"
	mlirtoolchain "sec/internal/mlir"
	"strings"
)

type emitSecMLIROptions struct {
	InputFile  string
	OutputFile string
	MLIRBin    string
	Target     CompilerTarget
}

// runEmitSecMLIRCommand lowers resolved Semantic IR under the selected scalar
// plan and preserves proof/capability failures before publishing Sec MLIR.
// Rules: rules/compiler/compiler_pipeline.md — §§2(8–9), 32–34;
// rules/compiler/compiler_analysis.md — §58(3).
// rules/compiler/compiler_pipeline.md — §§59(1–3),60(1–3) documented emission boundaries.
func runEmitSecMLIRCommand(args []string) {
	options, ok := parseEmitSecMLIRCommandArgs(args, hostCompilerTarget())
	if !ok {
		printUsage()
		exitCLI(1)
	}
	input, err := os.ReadFile(options.InputFile)
	if err != nil {
		reportToolError("read", "%v", err)
		exitCLI(1)
	}
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(string(input), options.InputFile, options.Target, false)
	mlirText, _ := emitSecDialect(analyzed, options.InputFile, options.Target, inspectionMaxPackage)
	verifyPath, removeVerifyPath, err := createTempOutputPath(".sec.mlir")
	if err != nil {
		reportToolError("temp file", "%v", err)
		exitCLI(1)
	}
	if removeVerifyPath {
		defer os.Remove(verifyPath)
	}
	if err := os.WriteFile(verifyPath, mlirText, 0600); err != nil {
		reportToolError("write", "%v", err)
		exitCLI(1)
	}
	if err := mlirtoolchain.NewToolchain(options.MLIRBin).VerifySecInspection(verifyPath); err != nil {
		reportPipelineError("Sec MLIR verification", err)
		exitCLI(4)
	}
	if err := writeCompilerOutput(options.OutputFile, mlirText); err != nil {
		reportToolError("write", "%v", err)
		exitCLI(1)
	}
}

// parseEmitSecMLIRCommandArgs selects the canonical Sec dialect inspection path.
// Rules: rules/compiler/compiler_pipeline.md — §59.
func parseEmitSecMLIRCommandArgs(args []string, defaultTarget CompilerTarget) (emitSecMLIROptions, bool) {
	options := emitSecMLIROptions{OutputFile: "-", Target: defaultTarget}
	outputSet := false
	targetSet := false
	mlirBinSet := false
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-o":
			if outputSet || index+1 >= len(args) {
				return emitSecMLIROptions{}, false
			}
			outputSet = true
			index++
			options.OutputFile = args[index]
		case "--target":
			if targetSet || index+1 >= len(args) {
				return emitSecMLIROptions{}, false
			}
			targetSet = true
			index++
			target, ok := parseCompilerTarget(args[index])
			if !ok {
				return emitSecMLIROptions{}, false
			}
			options.Target = target
		case "--mlir-bin":
			if mlirBinSet || index+1 >= len(args) {
				return emitSecMLIROptions{}, false
			}
			mlirBinSet = true
			index++
			options.MLIRBin = args[index]
		default:
			if strings.HasPrefix(args[index], "-") || options.InputFile != "" {
				return emitSecMLIROptions{}, false
			}
			options.InputFile = args[index]
		}
	}
	return options, options.InputFile != ""
}

// runEmitMLIRCommand publishes the LLVM dialect boundary of the single
// compilation track: Sec dialect MLIR lowered through standard MLIR by
// sec-mlir-opt --sec-lower-to-llvm (MD-101).
// Rules: rules/compiler/compiler_pipeline.md — §§32–34,60(1–3).
func runEmitMLIRCommand(args []string) {
	options, ok := parseEmitMLIRCommandArgs(args, hostCompilerTarget())
	if !ok {
		printUsage()
		exitCLI(1)
	}

	input, err := os.ReadFile(options.InputFile)
	if err != nil {
		reportToolError("read", "%v", err)
		exitCLI(1)
	}

	if _, err := requireTargetCanEmitLLVM(options.Target); err != nil {
		reportToolError("target", "%s", err)
		exitCLI(1)
	}

	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(string(input), options.InputFile, options.Target, false)
	requireOutputLoweringReadiness(analyzed, options.InputFile)
	secMLIR, entry := emitSecDialect(analyzed, options.InputFile, options.Target, nativeTrackMaxPackage)
	llvmDialectPath, cleanup := lowerToLLVMDialect(secMLIR, entry, options.MLIRBin, "")
	defer cleanup()
	lowered, err := os.ReadFile(llvmDialectPath)
	if err != nil {
		reportToolError("read", "%v", err)
		exitCLI(1)
	}
	mlirText := mlirEmissionHeader("LLVM dialect MLIR", options.Target) + stripMLIRHeaderComments(string(lowered))
	if err := writeCompilerOutput(options.OutputFile, []byte(mlirText)); err != nil {
		reportToolError("write", "%v", err)
		exitCLI(1)
	}
}

type emitMLIRCommandOptions struct {
	InputFile  string
	OutputFile string
	Target     CompilerTarget
	MLIRBin    string
}

// parseEmitMLIRCommandArgs selects the LLVM dialect inspection path and optional verification.
// Rules: rules/compiler/compiler_pipeline.md — §60(1–3).
func parseEmitMLIRCommandArgs(args []string, defaultTarget CompilerTarget) (emitMLIRCommandOptions, bool) {
	options := emitMLIRCommandOptions{Target: defaultTarget}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || options.OutputFile != "" {
				return emitMLIRCommandOptions{}, false
			}
			options.OutputFile = args[i+1]
			if !isValidEmitOutputFile(options.OutputFile) {
				return emitMLIRCommandOptions{}, false
			}
			i++
		case "--target":
			if i+1 >= len(args) {
				return emitMLIRCommandOptions{}, false
			}
			target, ok := parseCompilerTarget(args[i+1])
			if !ok {
				return emitMLIRCommandOptions{}, false
			}
			options.Target = target
			i++
		case "--mlir-bin":
			if i+1 >= len(args) || options.MLIRBin != "" {
				return emitMLIRCommandOptions{}, false
			}
			options.MLIRBin = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") || options.InputFile != "" {
				return emitMLIRCommandOptions{}, false
			}
			options.InputFile = args[i]
		}
	}

	return options, options.InputFile != "" && options.OutputFile != ""
}

// mlirEmissionHeader labels the CLI inspection boundary and selected target in
// ordinary MLIR comments, preserving parser and translation compatibility.
// Rules: rules/compiler/compiler_pipeline.md — §59(3),§60(1–3).
func mlirEmissionHeader(boundary string, target CompilerTarget) string {
	return fmt.Sprintf("// Sec compiler MLIR boundary: %s\n// Selected target: %s\n", boundary, target.String())
}
