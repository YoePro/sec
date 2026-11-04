package main

import (
	"os"
	"strings"

	semantic "sec/internal/ir/semantic"
)

// runEmitIRCommand publishes verified Semantic IR while preserving structured
// source-proof and compiler-capability failures at the requested boundary.
// Rules: rules/compiler/compiler_pipeline.md — §§2(8–9), 32–34;
// rules/compiler/compiler_analysis.md — §58(3).
func runEmitIRCommand(args []string) {
	inputFile, outputFile, target, jsonOutput, ok := parseEmitIRCommandArgs(args, hostCompilerTarget())
	if !ok {
		printUsage()
		exitCLI(1)
	}
	input, err := os.ReadFile(inputFile)
	if err != nil {
		reportToolError("read", "%v", err)
		exitCLI(1)
	}
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(string(input), inputFile, target, false)
	module, err := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{SourceFiles: []string{inputFile}, MaxPackage: inspectionMaxPackage})
	if err != nil {
		reportPipelineError("semantic IR", err)
		exitCLI(4)
	}
	var output []byte
	if jsonOutput {
		output, err = semantic.Serialize(module)
	} else {
		err = semantic.Verify(module)
		if err == nil {
			output = []byte(semantic.Format(module))
		}
	}
	if err != nil {
		reportPipelineError("semantic IR verification/serialization", err)
		exitCLI(4)
	}
	if err := writeCompilerOutput(outputFile, output); err != nil {
		reportToolError("write", "%v", err)
		exitCLI(1)
	}
}

// parseEmitIRCommandArgs selects text or the versioned JSON transport without
// changing target resolution. Rules: rules/tooling/cli.md — §5 emit-ir;
// rules/compiler/semantic_ir.md — §§97,106.
func parseEmitIRCommandArgs(args []string, defaultTarget CompilerTarget) (inputFile, outputFile string, target CompilerTarget, jsonOutput bool, ok bool) {
	outputFile = "-"
	target = defaultTarget
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			if jsonOutput {
				return "", "", CompilerTarget{}, false, false
			}
			jsonOutput = true
		case "-o":
			if i+1 >= len(args) {
				return "", "", CompilerTarget{}, false, false
			}
			i++
			outputFile = args[i]
		case "--target":
			if i+1 >= len(args) {
				return "", "", CompilerTarget{}, false, false
			}
			i++
			parsed, valid := parseCompilerTarget(args[i])
			if !valid {
				return "", "", CompilerTarget{}, false, false
			}
			target = parsed
		default:
			if strings.HasPrefix(args[i], "-") || inputFile != "" {
				return "", "", CompilerTarget{}, false, false
			}
			inputFile = args[i]
		}
	}
	return inputFile, outputFile, target, jsonOutput, inputFile != ""
}
