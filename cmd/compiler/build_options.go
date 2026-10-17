package main

import (
	"strconv"
	"strings"
)

// buildCommandOptions selects build tools and invocation-only overrides.
// Rules: rules/projects/projects.md — §§25(4–5),30.
type buildCommandOptions struct {
	BuildNumber    *uint64
	TargetSet      bool
	OutputSet      bool
	InputFile      string
	OutputFile     string
	Target         CompilerTarget
	Clang          string
	LLVMOutputFile string
	MLIROutputFile string
	MLIRBin        string
}

func parseBuildCommandArgs(args []string) (inputFile string, outputFile string, ok bool) {
	options, ok := parseBuildCommandOptions(args, CompilerTarget{})
	return options.InputFile, options.OutputFile, ok
}

// parseBuildCommandOptions validates explicit build numbers without changing project state.
// Rules: rules/projects/projects.md — §§8(3),30(1–4).
func parseBuildCommandOptions(args []string, defaultTarget CompilerTarget) (buildCommandOptions, bool) {
	options := buildCommandOptions{
		Target: defaultTarget,
		Clang:  "clang",
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || options.OutputFile != "" {
				return buildCommandOptions{}, false
			}
			options.OutputFile = args[i+1]
			options.OutputSet = true
			if !isValidBuildOutputFile(options.OutputFile) {
				return buildCommandOptions{}, false
			}
			i++
		case "--target":
			if i+1 >= len(args) {
				return buildCommandOptions{}, false
			}
			target, ok := parseCompilerTarget(args[i+1])
			if !ok {
				return buildCommandOptions{}, false
			}
			options.Target = target
			options.TargetSet = true
			i++
		case "--build-number":
			if i+1 >= len(args) || options.BuildNumber != nil {
				return buildCommandOptions{}, false
			}
			raw := args[i+1]
			if raw == "" {
				return buildCommandOptions{}, false
			}
			for _, digit := range raw {
				if digit < '0' || digit > '9' {
					return buildCommandOptions{}, false
				}
			}
			value, err := strconv.ParseUint(raw, 10, 63)
			if err != nil {
				return buildCommandOptions{}, false
			}
			options.BuildNumber = &value
			i++
		case "--clang":
			if i+1 >= len(args) || options.Clang != "clang" {
				return buildCommandOptions{}, false
			}
			options.Clang = args[i+1]
			i++
		case "--mlir-bin":
			if i+1 >= len(args) || options.MLIRBin != "" {
				return buildCommandOptions{}, false
			}
			options.MLIRBin = args[i+1]
			i++
		case "--keep-llvm":
			if options.LLVMOutputFile != "" {
				return buildCommandOptions{}, false
			}
			options.LLVMOutputFile = "__sec_keep_llvm__"
		case "--keep-mlir":
			if options.MLIROutputFile != "" {
				return buildCommandOptions{}, false
			}
			options.MLIROutputFile = "__sec_keep_mlir__"
		default:
			if strings.HasPrefix(args[i], "-") || options.InputFile != "" {
				return buildCommandOptions{}, false
			}
			options.InputFile = args[i]
		}
	}

	if options.InputFile == "" {
		return buildCommandOptions{}, false
	}
	if options.OutputFile == "" {
		options.OutputFile = defaultBuildOutputPath(options.InputFile)
	}
	if options.Clang == "" {
		options.Clang = "clang"
	}
	if options.LLVMOutputFile == "__sec_keep_llvm__" {
		options.LLVMOutputFile = options.OutputFile + ".ll"
	}
	if options.MLIROutputFile == "__sec_keep_mlir__" {
		options.MLIROutputFile = options.OutputFile + ".mlir"
	}

	return options, true
}
