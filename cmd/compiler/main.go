package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"sec/internal/ast"
	llvmcodegen "sec/internal/codegen/llvm"
	mlircodegen "sec/internal/codegen/mlir"
	"sec/internal/diagnostics"
	secformatter "sec/internal/formatter"
	semantic "sec/internal/ir/semantic"
	"sec/internal/lexer"
	secmlirlowering "sec/internal/lowering/secmlir"
	mlirtoolchain "sec/internal/mlir"
	"sec/internal/parser"
	"sec/internal/sema"
)

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		printUsage()
		os.Exit(1)
	}

	command := flag.Arg(0)

	if command == "diagnostics" {
		if err := runDiagnosticCatalogCommand(flag.Args()[1:], os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "diagnostics error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if command == "init" {
		runInitCommand(flag.Args()[1:])
		return
	}

	if command == "emit-llvm" {
		runEmitLLVMCommand(flag.Args()[1:])
		return
	}

	if command == "emit-ir" {
		runEmitIRCommand(flag.Args()[1:])
		return
	}

	if command == "emit-sec-mlir" {
		runEmitSecMLIRCommand(flag.Args()[1:])
		return
	}

	if command == "emit-mlir" {
		runEmitMLIRCommand(flag.Args()[1:])
		return
	}

	if command == "build" {
		runBuildCommand(flag.Args()[1:])
		return
	}

	if command == "fmt" {
		if err := runFmtCommand(flag.Args()[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "fmt error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if command == "parse" || command == "ast" || command == "sema" {
		if flag.NArg() < 2 {
			printUsage()
			os.Exit(1)
		}
		inputs := flag.Args()[1:]
		switch command {
		case "parse":
			runParseInputs(inputs)
		case "ast":
			runASTInputs(inputs)
		case "sema":
			runSemaInputs(inputs, hostCompilerTarget())
		}
		return
	}

	if flag.NArg() != 2 {
		printUsage()
		os.Exit(1)
	}

	file := flag.Arg(1)

	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}

	switch command {
	case "lex":
		runLex(string(data))

	case "token", "tokens", "lexer":
		runTokens(string(data))

	case "parse":
		runParse(string(data))

	case "ast":
		runAST(string(data))

	case "sema":
		parseAndAnalyzeFileForTarget(file, hostCompilerTarget())
		fmt.Println("OK")

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: sec <lex|token> <file.sec>")
	fmt.Fprintln(os.Stderr, "       sec diagnostics [--json]")
	fmt.Fprintln(os.Stderr, "       sec init [path] [--name <name>] [--target <os-arch>] [--profile <profile>]")
	fmt.Fprintln(os.Stderr, "       sec <parse|ast|sema> <file.sec|dir|glob>...")
	fmt.Fprintln(os.Stderr, "       sec fmt <file.sec>...")
	fmt.Fprintln(os.Stderr, "       sec emit-llvm <file.sec> -o <file.ll|-> [--target <os-arch>]")
	fmt.Fprintln(os.Stderr, "       sec emit-ir <file.sec> [-o <file.sir|->] [--target <os-arch>]")
	fmt.Fprintln(os.Stderr, "       sec emit-sec-mlir <file.sec> [-o <file.mlir|->] [--target <os-arch>] [--mlir-bin <path>]")
	fmt.Fprintln(os.Stderr, "       sec emit-mlir <file.sec> -o <file.mlir|-> [--target <os-arch>] [--mlir-bin <path>] [--verify]")
	fmt.Fprintln(os.Stderr, "       sec build <file.sec> [-o <program>] [--target <os-arch>] [--pipeline <llvm|mlir>] [--keep-mlir] [--keep-llvm] [--mlir-bin <path>] [--clang <path>]")
}

// runFmtCommand applies the canonical shared formatter in place. LSP document
// formatting calls the same internal/formatter package, as required by
// rules/tooling/formatter.md, Shared implementation.
func runFmtCommand(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("expected at least one source file")
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular source file", path)
		}
		input, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		formatted := secformatter.Format(secformatter.Source{Text: string(input)}, secformatter.Options{}).Text
		if formatted == string(input) {
			continue
		}
		if err := replaceFormattedFile(path, []byte(formatted), info.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func replaceFormattedFile(path string, contents []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".sec-fmt-*")
	if err != nil {
		return fmt.Errorf("%s: create temporary file: %w", path, err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("%s: preserve permissions: %w", path, err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("%s: write formatted source: %w", path, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("%s: close formatted source: %w", path, err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("%s: replace source: %w", path, err)
	}
	removeTemporary = false
	return nil
}

func runLex(input string) {
	l := lexer.New(input)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "LEXEME\tLINE\tCOLUMN")

	summary := diagnosticSummary{}

	for {
		tok := l.NextToken()

		if tok.Type != lexer.EOF {
			fmt.Fprintf(
				w,
				"%q\t%d\t%d\n",
				tok.Lexeme,
				tok.Line,
				tok.Column,
			)
		}

		if tok.Type == lexer.ILLEGAL {
			summary.Errors++
		}

		if tok.Type == lexer.EOF {
			break
		}
	}

	_ = w.Flush()
	printDiagnosticSummary(summary)
	if summary.Errors > 0 {
		os.Exit(2)
	}
}

func runTokens(input string) {
	l := lexer.New(input)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tLEXEME\tLINE\tCOLUMN")

	summary := diagnosticSummary{}

	for {
		tok := l.NextToken()

		fmt.Fprintf(
			w,
			"%s\t%q\t%d\t%d\n",
			tok.Type,
			tok.Lexeme,
			tok.Line,
			tok.Column,
		)

		if tok.Type == lexer.ILLEGAL {
			summary.Errors++
		}

		if tok.Type == lexer.EOF {
			break
		}
	}

	_ = w.Flush()
	printDiagnosticSummary(summary)
	if summary.Errors > 0 {
		os.Exit(2)
	}
}

func runParse(input string) {
	l := lexer.New(input)
	p := parser.New(l)

	result := p.Parse()
	program := result.Program

	printParserWarnings(p)
	summary := diagnosticSummary{Warnings: len(p.Warnings())}

	if result.HasErrors {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		}
		summary.Errors = len(p.Errors())
		printDiagnosticSummary(summary)
		os.Exit(2)
	}

	printProgram(program)
	printDiagnosticSummary(summary)
}

func runParseInputs(inputs []string) {
	program, summary := parseSourceInputs(inputs, CompilerTarget{}, false)
	printProgram(program)
	printDiagnosticSummary(summary)
}

func runAST(input string) {
	l := lexer.New(input)
	p := parser.New(l)

	result := p.Parse()
	program := result.Program

	printParserWarnings(p)
	summary := diagnosticSummary{Warnings: len(p.Warnings())}

	if result.HasErrors {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		}
		summary.Errors = len(p.Errors())
		printDiagnosticSummary(summary)
		os.Exit(2)
	}

	printAST(program)
	printDiagnosticSummary(summary)
}

func runASTInputs(inputs []string) {
	program, summary := parseSourceInputs(inputs, CompilerTarget{}, false)
	printAST(program)
	printDiagnosticSummary(summary)
}

func runSema(input string) {
	parseAndAnalyze(input)
	fmt.Println("OK")
}

func runSemaInputs(inputs []string, target CompilerTarget) {
	program, summary := parseSourceInputs(inputs, target, true)
	analyzeProgramWithSources(program, target, collectSourceRootsFromInputs(inputs), summary)
	fmt.Println("OK")
}

func runEmitLLVMCommand(args []string) {
	inputFile, outputFile, target, ok := parseEmitLLVMCommandArgs(args, hostCompilerTarget())
	if !ok {
		printUsage()
		os.Exit(1)
	}

	targetDefinition, err := requireTargetCanEmitLLVM(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "target error: %s\n", err)
		os.Exit(1)
	}

	program := parseAndAnalyzeFileForTarget(inputFile, target)
	ir, err := llvmcodegen.GenerateWithTriple(program, targetDefinition.LLVMTriple)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen error: %v\n", err)
		os.Exit(4)
	}

	if err := writeCompilerOutput(outputFile, []byte(ir)); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
}

func runEmitIRCommand(args []string) {
	inputFile, outputFile, target, ok := parseEmitIRCommandArgs(args, hostCompilerTarget())
	if !ok {
		printUsage()
		os.Exit(1)
	}
	input, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(string(input), inputFile, target, false)
	module, err := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{SourceFiles: []string{inputFile}})
	if err != nil {
		fmt.Fprintf(os.Stderr, "semantic IR error: %v\n", err)
		os.Exit(4)
	}
	if err := semantic.Verify(module); err != nil {
		fmt.Fprintf(os.Stderr, "semantic IR verification error: %v\n", err)
		os.Exit(4)
	}
	if err := writeCompilerOutput(outputFile, []byte(semantic.Format(module))); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
}

func parseEmitIRCommandArgs(args []string, defaultTarget CompilerTarget) (inputFile, outputFile string, target CompilerTarget, ok bool) {
	outputFile = "-"
	target = defaultTarget
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) {
				return "", "", CompilerTarget{}, false
			}
			i++
			outputFile = args[i]
		case "--target":
			if i+1 >= len(args) {
				return "", "", CompilerTarget{}, false
			}
			i++
			parsed, valid := parseCompilerTarget(args[i])
			if !valid {
				return "", "", CompilerTarget{}, false
			}
			target = parsed
		default:
			if strings.HasPrefix(args[i], "-") || inputFile != "" {
				return "", "", CompilerTarget{}, false
			}
			inputFile = args[i]
		}
	}
	return inputFile, outputFile, target, inputFile != ""
}

type emitSecMLIROptions struct {
	InputFile  string
	OutputFile string
	MLIRBin    string
	Target     CompilerTarget
}

func runEmitSecMLIRCommand(args []string) {
	options, ok := parseEmitSecMLIRCommandArgs(args, hostCompilerTarget())
	if !ok {
		printUsage()
		os.Exit(1)
	}
	input, err := os.ReadFile(options.InputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}
	analyzed := parseAndAnalyzeSourceForTargetWithAnalyzerMode(string(input), options.InputFile, options.Target, false)
	module, err := semantic.Build(analyzed.Program, analyzed.Analyzer, semantic.BuildOptions{SourceFiles: []string{options.InputFile}, MaxPackage: 12})
	if err != nil {
		fmt.Fprintf(os.Stderr, "semantic IR error: %v\n", err)
		os.Exit(4)
	}
	targetDefinition, ok := findTargetDefinition(options.Target)
	if !ok {
		fmt.Fprintf(os.Stderr, "target error: unsupported target %s\n", options.Target.String())
		os.Exit(1)
	}
	scalarPlan, err := targetDefinition.scalarPlan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "target error: %v\n", err)
		os.Exit(1)
	}
	mlirText, err := secmlirlowering.Emit(module, scalarPlan)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Sec MLIR lowering error: %v\n", err)
		os.Exit(4)
	}
	verifyPath, removeVerifyPath, err := createTempOutputPath(".sec.mlir")
	if err != nil {
		fmt.Fprintf(os.Stderr, "temp file error: %v\n", err)
		os.Exit(1)
	}
	if removeVerifyPath {
		defer os.Remove(verifyPath)
	}
	if err := os.WriteFile(verifyPath, mlirText, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
	if err := mlirtoolchain.NewToolchain(options.MLIRBin).VerifySec(verifyPath); err != nil {
		fmt.Fprintf(os.Stderr, "Sec MLIR verification error: %v\n", err)
		os.Exit(4)
	}
	if err := writeCompilerOutput(options.OutputFile, mlirText); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
}

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

func runEmitMLIRCommand(args []string) {
	options, ok := parseEmitMLIRCommandArgs(args, hostCompilerTarget())
	if !ok {
		printUsage()
		os.Exit(1)
	}

	input, err := os.ReadFile(options.InputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}

	targetDefinition, err := requireTargetCanEmitLLVM(options.Target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "target error: %s\n", err)
		os.Exit(1)
	}

	program := parseAndAnalyzeSourceForTarget(string(input), options.InputFile, options.Target)
	mlirText, err := mlircodegen.GenerateWithTriple(program, targetDefinition.LLVMTriple)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen error: %v\n", err)
		os.Exit(4)
	}

	if options.OutputFile != "-" {
		if err := writeCompilerOutput(options.OutputFile, []byte(mlirText)); err != nil {
			fmt.Fprintf(os.Stderr, "write error: %v\n", err)
			os.Exit(1)
		}
	}

	if options.Verify {
		toolchain := mlirtoolchain.NewToolchain(options.MLIRBin)
		verifyPath := options.OutputFile
		if options.OutputFile == "-" {
			tmpPath, removeTmp, err := createTempOutputPath(".mlir")
			if err != nil {
				fmt.Fprintf(os.Stderr, "temp file error: %v\n", err)
				os.Exit(1)
			}
			verifyPath = tmpPath
			if removeTmp {
				defer os.Remove(tmpPath)
			}
			if err := os.WriteFile(verifyPath, []byte(mlirText), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "write error: %v\n", err)
				os.Exit(1)
			}
		}
		if err := toolchain.Verify(verifyPath); err != nil {
			fmt.Fprintf(os.Stderr, "mlir error: %v\n", err)
			os.Exit(4)
		}
	}

	if options.OutputFile == "-" {
		if err := writeCompilerOutput(options.OutputFile, []byte(mlirText)); err != nil {
			fmt.Fprintf(os.Stderr, "write error: %v\n", err)
			os.Exit(1)
		}
	}
}

func writeCompilerOutput(outputFile string, data []byte) error {
	if outputFile == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(outputFile, data, 0644)
}

func runBuildCommand(args []string) {
	options, ok := parseBuildCommandOptions(args, hostCompilerTarget())
	if !ok {
		printUsage()
		os.Exit(1)
	}

	input, err := os.ReadFile(options.InputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}

	targetDefinition, err := requireTargetCanLink(options.Target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "target error: %s\n", err)
		os.Exit(1)
	}

	program := parseAndAnalyzeSourceForTarget(string(input), options.InputFile, options.Target)
	llvmPath := ""
	switch options.Pipeline {
	case "llvm":
		ir, err := llvmcodegen.GenerateWithTriple(program, targetDefinition.LLVMTriple)
		if err != nil {
			fmt.Fprintf(os.Stderr, "codegen error: %v\n", err)
			os.Exit(4)
		}
		llvmPath = options.LLVMOutputFile
		removeLLVM := false
		if llvmPath == "" {
			llvmPath, removeLLVM, err = createTempOutputPath(".ll")
			if err != nil {
				fmt.Fprintf(os.Stderr, "temp file error: %v\n", err)
				os.Exit(1)
			}
		}
		if removeLLVM {
			defer os.Remove(llvmPath)
		}
		if err := os.WriteFile(llvmPath, []byte(ir), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write error: %v\n", err)
			os.Exit(1)
		}
	case "mlir":
		var cleanup func()
		llvmPath, cleanup = runMLIRBuildPipeline(program, targetDefinition.LLVMTriple, options)
		defer cleanup()
	default:
		fmt.Fprintf(os.Stderr, "build error: unknown pipeline %q\n", options.Pipeline)
		os.Exit(1)
	}

	// rules/compiler/linking.md sections 8, 29, and 37: this is the legacy
	// direct-driver path. The selected executable and argv are not a canonical
	// LinkEnvironment/LinkPlan and must eventually be materialized from one.
	clangPath, err := exec.LookPath(options.Clang)
	if err != nil {
		fmt.Fprintf(os.Stderr, "build error: clang not found: %v\n", err)
		os.Exit(1)
	}

	clangArgs := []string{"-target", targetDefinition.LLVMTriple, llvmPath, "-o", options.OutputFile}
	cmd := exec.Command(clangPath, clangArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build error: %v\n", err)
		os.Exit(5)
	}
}

func runMLIRBuildPipeline(program *ast.Program, triple string, options buildCommandOptions) (string, func()) {
	cleanupPaths := []string{}
	cleanup := func() {
		for _, path := range cleanupPaths {
			_ = os.Remove(path)
		}
	}

	mlirText, err := mlircodegen.GenerateWithTriple(program, triple)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen error: %v\n", err)
		os.Exit(4)
	}

	mlirPath := options.MLIROutputFile
	removeMLIR := false
	if mlirPath == "" {
		mlirPath, removeMLIR, err = createTempOutputPath(".mlir")
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp file error: %v\n", err)
			os.Exit(1)
		}
	}
	if removeMLIR {
		cleanupPaths = append(cleanupPaths, mlirPath)
	}
	if err := os.WriteFile(mlirPath, []byte(mlirText), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}

	llvmPath := options.LLVMOutputFile
	removeLLVM := false
	if llvmPath == "" {
		llvmPath, removeLLVM, err = createTempOutputPath(".ll")
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp file error: %v\n", err)
			os.Exit(1)
		}
	}
	if removeLLVM {
		cleanupPaths = append(cleanupPaths, llvmPath)
	}

	toolchain := mlirtoolchain.NewToolchain(options.MLIRBin)
	if err := toolchain.Verify(mlirPath); err != nil {
		fmt.Fprintf(os.Stderr, "mlir error: %v\n", err)
		os.Exit(4)
	}
	if err := toolchain.TranslateToLLVMIR(mlirPath, llvmPath); err != nil {
		fmt.Fprintf(os.Stderr, "mlir error: %v\n", err)
		os.Exit(4)
	}
	return llvmPath, cleanup
}

func createTempOutputPath(ext string) (string, bool, error) {
	tmp, err := os.CreateTemp("", "sec-*"+ext)
	if err != nil {
		return "", false, err
	}
	path := tmp.Name()
	if err := tmp.Close(); err != nil {
		return "", false, err
	}
	return path, true, nil
}

func parseOutputCommandArgs(args []string) (inputFile string, outputFile string, ok bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || outputFile != "" {
				return "", "", false
			}
			outputFile = args[i+1]
			if !isValidEmitOutputFile(outputFile) {
				return "", "", false
			}
			i++
		default:
			if inputFile != "" {
				return "", "", false
			}
			inputFile = args[i]
		}
	}

	return inputFile, outputFile, inputFile != "" && outputFile != ""
}

func parseEmitLLVMCommandArgs(args []string, defaultTarget CompilerTarget) (inputFile string, outputFile string, target CompilerTarget, ok bool) {
	target = defaultTarget
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || outputFile != "" {
				return "", "", CompilerTarget{}, false
			}
			outputFile = args[i+1]
			if !isValidEmitOutputFile(outputFile) {
				return "", "", CompilerTarget{}, false
			}
			i++
		case "--target":
			if i+1 >= len(args) {
				return "", "", CompilerTarget{}, false
			}
			parsed, parseOK := parseCompilerTarget(args[i+1])
			if !parseOK {
				return "", "", CompilerTarget{}, false
			}
			target = parsed
			i++
		default:
			if strings.HasPrefix(args[i], "-") || inputFile != "" {
				return "", "", CompilerTarget{}, false
			}
			inputFile = args[i]
		}
	}

	return inputFile, outputFile, target, inputFile != "" && outputFile != ""
}

type emitMLIRCommandOptions struct {
	InputFile  string
	OutputFile string
	Target     CompilerTarget
	MLIRBin    string
	Verify     bool
}

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
		case "--verify":
			options.Verify = true
		default:
			if strings.HasPrefix(args[i], "-") || options.InputFile != "" {
				return emitMLIRCommandOptions{}, false
			}
			options.InputFile = args[i]
		}
	}

	return options, options.InputFile != "" && options.OutputFile != ""
}

func isValidEmitOutputFile(path string) bool {
	return path == "-" || !strings.HasPrefix(filepath.Base(path), "-")
}

type buildCommandOptions struct {
	InputFile      string
	OutputFile     string
	Target         CompilerTarget
	Clang          string
	LLVMOutputFile string
	MLIROutputFile string
	MLIRBin        string
	Pipeline       string
}

func parseBuildCommandArgs(args []string) (inputFile string, outputFile string, ok bool) {
	options, ok := parseBuildCommandOptions(args, CompilerTarget{})
	return options.InputFile, options.OutputFile, ok
}

func parseBuildCommandOptions(args []string, defaultTarget CompilerTarget) (buildCommandOptions, bool) {
	options := buildCommandOptions{
		Target:   defaultTarget,
		Clang:    "clang",
		Pipeline: "llvm",
	}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || options.OutputFile != "" {
				return buildCommandOptions{}, false
			}
			options.OutputFile = args[i+1]
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
			i++
		case "--clang":
			if i+1 >= len(args) || options.Clang != "clang" {
				return buildCommandOptions{}, false
			}
			options.Clang = args[i+1]
			i++
		case "--pipeline":
			if i+1 >= len(args) {
				return buildCommandOptions{}, false
			}
			switch args[i+1] {
			case "llvm", "mlir":
				options.Pipeline = args[i+1]
			default:
				return buildCommandOptions{}, false
			}
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

func isValidBuildOutputFile(path string) bool {
	return path != "" && path != "-" && !strings.HasPrefix(filepath.Base(path), "-")
}

func defaultBuildOutputPath(inputFile string) string {
	base := filepath.Base(filepath.Clean(inputFile))
	ext := filepath.Ext(base)
	if ext == "" {
		return base
	}
	return strings.TrimSuffix(base, ext)
}

func parseAndAnalyze(input string) *ast.Program {
	return parseAndAnalyzeForTarget(input, hostCompilerTarget())
}

func parseAndAnalyzeForTarget(input string, target CompilerTarget) *ast.Program {
	return parseAndAnalyzeSourceForTarget(input, "", target)
}

func parseAndAnalyzeFileForTarget(path string, target CompilerTarget) *ast.Program {
	input, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}
	return parseAndAnalyzeSourceForTarget(string(input), path, target)
}

func parseAndAnalyzeSourceForTarget(input string, sourceFile string, target CompilerTarget) *ast.Program {
	return parseAndAnalyzeSourceForTargetWithAnalyzer(input, sourceFile, target).Program
}

type analyzedProgram struct {
	Program  *ast.Program
	Analyzer *sema.Analyzer
}

func parseAndAnalyzeSourceForTargetWithAnalyzer(input string, sourceFile string, target CompilerTarget) analyzedProgram {
	return parseAndAnalyzeSourceForTargetWithAnalyzerMode(input, sourceFile, target, true)
}

func parseAndAnalyzeSourceForTargetWithAnalyzerMode(input string, sourceFile string, target CompilerTarget, printSuccessSummary bool) analyzedProgram {
	l := lexer.NewWithFile(input, sourceFile)
	p := parser.New(l)

	result := p.Parse()
	program := result.Program

	printParserWarnings(p)
	summary := diagnosticSummary{Warnings: len(p.Warnings())}

	if result.HasErrors {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		}
		summary.Errors = len(p.Errors())
		printDiagnosticSummary(summary)
		os.Exit(2)
	}

	analyzer := analyzeProgramWithSourcesRetained(program, target, []string{sourceFile}, summary, printSuccessSummary)

	return analyzedProgram{Program: program, Analyzer: analyzer}
}

func analyzeProgram(program *ast.Program, target CompilerTarget) {
	analyzeProgramWithSources(program, target, nil, diagnosticSummary{})
}

func analyzeProgramWithSources(program *ast.Program, target CompilerTarget, sourceFiles []string, summary diagnosticSummary) {
	_ = analyzeProgramWithSourcesRetained(program, target, sourceFiles, summary, true)
}

func analyzeProgramWithSourcesRetained(program *ast.Program, target CompilerTarget, sourceFiles []string, summary diagnosticSummary, printSuccessSummary bool) *sema.Analyzer {
	if err := validateProgramTarget(program, target); err != nil {
		fmt.Fprintf(os.Stderr, "target error: %s\n", err)
		summary.Errors++
		printDiagnosticSummary(summary)
		os.Exit(1)
	}

	resolveCoreLibraryWithSources(program, sourceFiles)
	resolveStdlibImportsWithSources(program, target, sourceFiles)

	targetDefinition, ok := findTargetDefinition(target)
	if !ok {
		fmt.Fprintf(os.Stderr, "target error: unsupported target %s\n", target.String())
		summary.Errors++
		printDiagnosticSummary(summary)
		os.Exit(1)
	}
	scalarPlan, err := targetDefinition.scalarPlan()
	if err != nil {
		fmt.Fprintf(os.Stderr, "target error: %s\n", err)
		summary.Errors++
		printDiagnosticSummary(summary)
		os.Exit(1)
	}
	analyzer := sema.NewAnalyzerWithScalarPlan(scalarPlan)
	errors := analyzer.Analyze(program)
	printSemaWarnings(analyzer)
	for _, warning := range analyzer.Warnings() {
		if warning.Severity == diagnostics.SeverityInformation {
			summary.Information++
		} else {
			summary.Warnings++
		}
	}
	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "sema error: %s\n", err)
		}
		summary.Errors += len(errors)
		printDiagnosticSummary(summary)
		os.Exit(3)
	}
	if printSuccessSummary {
		printDiagnosticSummary(summary)
	}
	return analyzer
}

func resolveCoreLibrary(program *ast.Program) {
	resolveCoreLibraryWithSources(program, nil)
}

func resolveCoreLibraryWithSources(program *ast.Program, sourceFiles []string) {
	root := findCompilerSourceRoot(sourceFiles)
	markTrustedCoreSources(program, root)
	if programContainsCoreSource(program) {
		return
	}
	core := parseCoreLibrary(sourceFiles)
	if core == nil || len(core.Statements) == 0 {
		return
	}
	program.Statements = append(append([]ast.Statement{}, core.Statements...), program.Statements...)
	mergeSourceProvenance(program, core)
}

func mergeSourceProvenance(destination, source *ast.Program) {
	if destination == nil || source == nil {
		return
	}
	if destination.SourceProvenance == nil {
		destination.SourceProvenance = map[string]ast.SourceProvenance{}
	}
	for file, provenance := range source.SourceProvenance {
		destination.SourceProvenance[file] = provenance
	}
}

// markTrustedCoreSources is the compiler-loader trust boundary required by
// rules/library/core-library.md and correction9.md. Canonical path containment
// is converted here into explicit provenance consumed by Sema.
func markTrustedCoreSources(program *ast.Program, root string) {
	if program == nil {
		return
	}
	coreRoot, err := filepath.Abs(filepath.Join(root, "sec", "core"))
	if err != nil {
		return
	}
	if resolved, resolveErr := filepath.EvalSymlinks(coreRoot); resolveErr == nil {
		coreRoot = resolved
	}
	if program.SourceProvenance == nil {
		program.SourceProvenance = map[string]ast.SourceProvenance{}
	}
	for _, stmt := range program.Statements {
		file := statementTokenForSource(stmt).File
		if file == "" {
			continue
		}
		absolute, absErr := filepath.Abs(file)
		if absErr != nil {
			continue
		}
		if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
			absolute = resolved
		}
		relative, relErr := filepath.Rel(coreRoot, absolute)
		if relErr == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			program.SourceProvenance[file] = ast.SourceCore
		}
	}
}

func programContainsCoreSource(program *ast.Program) bool {
	if program == nil {
		return false
	}
	for _, stmt := range program.Statements {
		token := statementTokenForSource(stmt)
		path := filepath.ToSlash(filepath.Clean(token.File))
		if strings.Contains(path, "/sec/core/") || strings.HasPrefix(path, "sec/core/") {
			return true
		}
	}
	return false
}

func statementTokenForSource(stmt ast.Statement) lexer.Token {
	switch stmt := stmt.(type) {
	case *ast.ModuleStatement:
		return stmt.Token
	case *ast.TypeDeclStatement:
		return stmt.Token
	case *ast.UnitDeclStatement:
		return stmt.Token
	case *ast.EnumDeclaration:
		return stmt.Token
	case *ast.InterfaceDeclaration:
		return stmt.Token
	case *ast.ImplStatement:
		return stmt.Token
	case *ast.FunctionDeclaration:
		return stmt.Token
	case *ast.LetStatement:
		return stmt.Token
	default:
		return lexer.Token{}
	}
}

func parseCoreLibrary(sourceFiles []string) *ast.Program {
	root := findCompilerSourceRoot(sourceFiles)
	matches, err := filepath.Glob(filepath.Join(root, "sec", "core", "*.sec"))
	if err != nil || len(matches) == 0 {
		return nil
	}
	sort.Strings(matches)
	core := &ast.Program{}
	for _, path := range matches {
		source := parseSourceInclude(path, CompilerTarget{})
		core.Statements = append(core.Statements, source.Statements...)
	}
	markTrustedCoreSources(core, root)
	return core
}

func parseSourceInputs(inputs []string, target CompilerTarget, filterTarget bool) (*ast.Program, diagnosticSummary) {
	files, err := collectSourceFiles(inputs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "source error: %v\n", err)
		os.Exit(1)
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "source error: no .sec files found")
		os.Exit(1)
	}

	combined := &ast.Program{}
	summary := diagnosticSummary{}
	included := 0
	for _, file := range files {
		if filterTarget && !sourcePathMatchesTarget(file, target) {
			continue
		}
		program, fileSummary := parseSourceFileWithDiagnostics(file)
		summary.Errors += fileSummary.Errors
		summary.Warnings += fileSummary.Warnings
		printParserWarningsForFile(file, program.Warnings)
		if fileSummary.Errors > 0 {
			for _, err := range program.Errors {
				fmt.Fprintf(os.Stderr, "%s: parse error: %s\n", file, err)
			}
			continue
		}
		if filterTarget {
			if err := validateProgramTarget(program.Program, target); err != nil {
				continue
			}
		}
		combined.Statements = append(combined.Statements, program.Program.Statements...)
		included++
	}
	if included == 0 {
		if summary.Errors > 0 {
			printDiagnosticSummary(summary)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "source error: no .sec files match target %s\n", target.String())
		os.Exit(1)
	}
	if summary.Errors > 0 {
		printDiagnosticSummary(summary)
		os.Exit(2)
	}
	return combined, summary
}

func collectSourceRootsFromInputs(inputs []string) []string {
	files, err := collectSourceFiles(inputs)
	if err != nil {
		return nil
	}
	return files
}

func sourcePathMatchesTarget(path string, target CompilerTarget) bool {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "platform" {
			continue
		}
		osPart := parts[i+1]
		if target.OS != "" && osPart != target.OS {
			return false
		}
		if i+3 < len(parts) {
			archPart := parts[i+2]
			if isKnownTargetArch(archPart) && target.Arch != "" && archPart != target.Arch {
				return false
			}
		}
		return true
	}
	return true
}

func isKnownTargetArch(part string) bool {
	switch part {
	case "amd64", "arm64", "arm32", "armv7", "x86", "cortex-m4":
		return true
	default:
		return false
	}
}

func collectSourceFiles(inputs []string) ([]string, error) {
	seen := map[string]bool{}
	files := []string{}

	add := func(path string) {
		clean := filepath.Clean(path)
		if seen[clean] {
			return
		}
		seen[clean] = true
		files = append(files, clean)
	}

	for _, input := range inputs {
		matches := []string{input}
		if strings.ContainsAny(input, "*?[") {
			globMatches, err := filepath.Glob(input)
			if err != nil {
				return nil, err
			}
			matches = globMatches
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("%s: no matches", input)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				return nil, err
			}
			if info.IsDir() {
				err := filepath.WalkDir(match, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if d.IsDir() {
						return nil
					}
					if filepath.Ext(path) == ".sec" {
						add(path)
					}
					return nil
				})
				if err != nil {
					return nil, err
				}
				continue
			}
			if filepath.Ext(match) == ".sec" {
				add(match)
			}
		}
	}

	return files, nil
}

func parseSourceFile(path string) *ast.Program {
	program, summary := parseSourceFileWithDiagnostics(path)
	printParserWarningsForFile(path, program.Warnings)
	if summary.Errors > 0 {
		for _, err := range program.Errors {
			fmt.Fprintf(os.Stderr, "%s: parse error: %s\n", path, err)
		}
		printDiagnosticSummary(summary)
		os.Exit(2)
	}
	return program.Program
}

type parsedSourceFile struct {
	Program  *ast.Program
	Errors   []string
	Warnings []string
}

func parseSourceFileWithDiagnostics(path string) (parsedSourceFile, diagnosticSummary) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}

	l := lexer.NewWithFile(string(data), path)
	p := parser.New(l)
	result := p.Parse()
	program := result.Program
	return parsedSourceFile{Program: program, Errors: p.Errors(), Warnings: p.Warnings()}, diagnosticSummary{Errors: len(p.Errors()), Warnings: len(p.Warnings())}
}

func printParserWarnings(p *parser.Parser) {
	for _, warning := range p.Warnings() {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
	}
}

func printParserWarningsForFile(path string, warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "%s: Warning: %s\n", path, warning)
	}
}

// printSemaWarnings preserves the registered information/warning distinction
// for optional semantic diagnostics instead of relabeling every advisory as a
// warning.
//
// Rules:
//   - rules/declarations/static.md — "Diagnostics"
//   - rules/tooling/diagnostics.txt — diagnostic severity
func printSemaWarnings(analyzer *sema.Analyzer) {
	for _, warning := range analyzer.Warnings() {
		label := "Warning"
		if warning.Severity == diagnostics.SeverityInformation {
			label = "Info"
		}
		fmt.Fprintf(os.Stderr, "%s: %s\n", label, warning)
	}
}

type diagnosticSummary struct {
	Errors      int
	Warnings    int
	Information int
}

func printDiagnosticSummary(summary diagnosticSummary) {
	fmt.Fprintf(os.Stderr, "summary: %s, %s", diagnosticCountLabel(summary.Errors, "error"), diagnosticCountLabel(summary.Warnings, "warning"))
	if summary.Information > 0 {
		fmt.Fprintf(os.Stderr, ", %s", diagnosticCountLabel(summary.Information, "information diagnostic"))
	}
	fmt.Fprintln(os.Stderr)
}

func diagnosticCountLabel(count int, singular string) string {
	if count == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %ss", count, singular)
}

func validateProgramTarget(program *ast.Program, current CompilerTarget) error {
	for _, stmt := range program.Statements {
		target, ok := stmt.(*ast.TargetDirective)
		if !ok {
			continue
		}

		fileTarget := CompilerTarget{
			OS:   normalizeTargetOS(target.OS),
			Arch: normalizeTargetArch(target.Arch),
		}
		if fileTarget.OS != current.OS || (fileTarget.Arch != "any" && fileTarget.Arch != current.Arch) {
			return fmt.Errorf("file target %s does not match current target %s", fileTarget.String(), current.String())
		}
	}
	return nil
}

func resolveStdlibImports(program *ast.Program, target CompilerTarget) {
	resolveStdlibImportsWithSources(program, target, nil)
}

func resolveStdlibImportsWithSources(program *ast.Program, target CompilerTarget, sourceFiles []string) {
	// TODO: Replace this source-level stdlib inclusion with compiled library metadata/artifacts.
	seen := map[string]bool{}
	resolveStdlibImportsInto(program, target, seen, sourceFiles)
}

func resolveStdlibImportsInto(program *ast.Program, target CompilerTarget, seen map[string]bool, sourceFiles []string) {
	for _, stmt := range append([]ast.Statement{}, program.Statements...) {
		importStmt, ok := stmt.(*ast.ImportStatement)
		if !ok {
			continue
		}
		sourcePaths, ok := sourceIncludePathsWithSources(importStmt.Path, target, sourceFiles)
		if !ok {
			continue
		}
		imported := &ast.Program{}
		module := ""
		for _, sourcePath := range sourcePaths {
			if seen[sourcePath] {
				if module == "" {
					module = programModulePath(parseSourceInclude(sourcePath, target))
				}
				continue
			}
			seen[sourcePath] = true
			sourceProgram := parseSourceInclude(sourcePath, target)
			if module == "" {
				module = programModulePath(sourceProgram)
			}
			imported.Statements = append(imported.Statements, sourceProgram.Statements...)
		}
		if len(imported.Statements) == 0 {
			rewriteImportQualifier(program, importQualifier(importStmt), module)
			continue
		}

		resolveStdlibImportsInto(imported, target, seen, sourcePaths)
		if module == "" {
			module = programModulePath(imported)
		}
		rewriteImportQualifier(program, importQualifier(importStmt), module)
		qualifyImportedModule(imported, module)
		program.Statements = append(program.Statements, imported.Statements...)
	}
}

func sourceIncludePaths(path string, target CompilerTarget) ([]string, bool) {
	return sourceIncludePathsWithSources(path, target, nil)
}

func sourceIncludePathsWithSources(path string, target CompilerTarget, sourceFiles []string) ([]string, bool) {
	if canSourceIncludePlatform(path) {
		trimmed := strings.Trim(strings.TrimSuffix(path, ".sec"), "/")
		base := filepath.Join("sec", filepath.FromSlash(trimmed))
		if _, err := os.Stat(base); err != nil {
			base = filepath.Join(findCompilerSourceRoot(sourceFiles), base)
		}
		if info, err := os.Stat(base); err == nil && info.IsDir() {
			matches, globErr := filepath.Glob(filepath.Join(base, "*.sec"))
			if globErr != nil {
				return nil, false
			}
			sort.Strings(matches)
			return matches, len(matches) > 0
		}
		if filepath.Ext(base) != ".sec" {
			base += ".sec"
		}
		return []string{base}, true
	}
	module := stdlibModuleName(path)
	if canSourceIncludeModule(module) {
		paths := stdlibModulePaths(module, target, sourceFiles)
		return paths, len(paths) > 0
	}
	sourcePath, ok := sourceIncludePath(path, target)
	if ok {
		sourcePath = resolveCompilerRelativeSourcePath(sourcePath, sourceFiles)
		return []string{sourcePath}, true
	}
	return projectIncludePaths(path, sourceFiles)
}

func resolveCompilerRelativeSourcePath(path string, sourceFiles []string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if _, err := os.Stat(path); err == nil {
		return filepath.Clean(path)
	}
	return filepath.Join(findCompilerSourceRoot(sourceFiles), path)
}

func findCompilerSourceRoot(sourceFiles []string) string {
	starts := []string{}
	for _, sourceFile := range sourceFiles {
		if sourceFile != "" {
			starts = append(starts, filepath.Dir(sourceFile))
		}
	}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(executable))
	}
	for _, start := range starts {
		for current := filepath.Clean(start); ; current = filepath.Dir(current) {
			if info, err := os.Stat(filepath.Join(current, "sec", "platform")); err == nil && info.IsDir() {
				return current
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
		}
	}
	return "."
}

func projectIncludePaths(path string, sourceFiles []string) ([]string, bool) {
	trimmed := strings.Trim(strings.TrimSuffix(path, ".sec"), "/")
	if trimmed == "" || strings.HasPrefix(trimmed, "std/") || strings.HasPrefix(trimmed, "platform/") {
		return nil, false
	}

	for _, root := range projectSourceRoots(sourceFiles) {
		if paths, ok := projectIncludePathsUnderRoot(root, trimmed); ok {
			return paths, true
		}
	}
	return nil, false
}

func projectIncludePathsUnderRoot(root string, importPath string) ([]string, bool) {
	// Project imports are resolved separately from stdlib imports. This keeps
	// ordinary source modules from accidentally becoming permanent library API.
	candidates := []string{
		filepath.Join(root, filepath.FromSlash(importPath)+".sec"),
		filepath.Join(root, filepath.FromSlash(importPath), filepath.Base(importPath)+".sec"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return []string{filepath.Clean(candidate)}, true
		}
	}

	dir := filepath.Join(root, filepath.FromSlash(importPath))
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		matches, globErr := filepath.Glob(filepath.Join(dir, "*.sec"))
		if globErr != nil || len(matches) == 0 {
			return nil, false
		}
		sort.Strings(matches)
		return matches, true
	}
	return nil, false
}

func projectSourceRoots(sourceFiles []string) []string {
	seen := map[string]bool{}
	roots := []string{}
	add := func(root string) {
		if root == "" {
			return
		}
		root = filepath.Clean(root)
		if seen[root] {
			return
		}
		seen[root] = true
		roots = append(roots, root)
	}

	for _, sourceFile := range sourceFiles {
		if sourceFile == "" {
			continue
		}
		add(findProjectRoot(sourceFile))
		add(filepath.Dir(sourceFile))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(findProjectRoot(cwd))
	}
	return roots
}

func findProjectRoot(path string) string {
	current := filepath.Clean(path)
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if info, err := os.Stat(filepath.Join(current, ".sec", "sec.toml")); err == nil && !info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return filepath.Clean(path)
	}
	return filepath.Dir(filepath.Clean(path))
}

func canSourceIncludeModule(name string) bool {
	switch name {
	case "fmt", "io", "unicode":
		return true
	default:
		return false
	}
}

func canSourceIncludePlatform(path string) bool {
	return strings.HasPrefix(path, "platform/")
}

func stdlibModuleName(path string) string {
	if len(path) > 4 && path[:4] == "std/" {
		return path[4:]
	}
	return path
}

func stdlibModulePath(name string, target CompilerTarget) string {
	if target.OS != "" && target.Arch != "" {
		switch name {
		case "io":
			return filepath.Join("sec", "stdlib", name, "write."+target.OS+"."+target.Arch+".sec")
		}
	}
	return filepath.Join("sec", "stdlib", name, name+".sec")
}

func stdlibModulePaths(name string, target CompilerTarget, sourceFiles []string) []string {
	primary := resolveCompilerRelativeSourcePath(stdlibModulePath(name, target), sourceFiles)
	dir := filepath.Dir(primary)
	matches, err := filepath.Glob(filepath.Join(dir, "*.sec"))
	if err != nil || len(matches) == 0 {
		return []string{primary}
	}
	sort.Strings(matches)

	seen := map[string]bool{}
	out := []string{}
	add := func(path string) {
		path = filepath.Clean(path)
		if seen[path] || !sourcePathMatchesTarget(path, target) {
			return
		}
		seen[path] = true
		out = append(out, path)
	}

	add(primary)
	for _, match := range matches {
		add(match)
	}
	return out
}

func sourceIncludePath(path string, target CompilerTarget) (string, bool) {
	module := stdlibModuleName(path)
	if canSourceIncludeModule(module) {
		return stdlibModulePath(module, target), true
	}
	if canSourceIncludePlatform(path) {
		trimmed := strings.Trim(path, "/")
		if strings.HasSuffix(trimmed, ".sec") {
			trimmed = strings.TrimSuffix(trimmed, ".sec")
		}
		return filepath.Join("sec", trimmed+".sec"), true
	}
	return "", false
}

func importQualifier(stmt *ast.ImportStatement) string {
	if stmt == nil {
		return ""
	}
	if stmt.Alias != "" {
		return stmt.Alias
	}
	trimmed := strings.Trim(strings.TrimSuffix(stmt.Path, ".sec"), "/")
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		return trimmed[index+1:]
	}
	return trimmed
}

func parseSourceInclude(path string, target CompilerTarget) *ast.Program {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "source import error: %v\n", err)
		os.Exit(1)
	}

	l := lexer.NewWithFile(string(data), path)
	p := parser.New(l)
	program := p.Parse().Program
	printParserWarnings(p)
	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "stdlib parse error: %s\n", err)
		}
		os.Exit(2)
	}
	if err := validateProgramTarget(program, target); err != nil {
		fmt.Fprintf(os.Stderr, "stdlib target error: %s\n", err)
		os.Exit(1)
	}

	return program
}

func programModulePath(program *ast.Program) string {
	for _, stmt := range program.Statements {
		module, ok := stmt.(*ast.ModuleStatement)
		if ok {
			return module.Path
		}
	}
	return ""
}

func qualifyImportedModule(program *ast.Program, module string) {
	localFunctions := map[string]bool{}
	localTypes := map[string]bool{}
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			if stmt != nil && stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localFunctions[stmt.Name.Value] = true
			}
		case *ast.TypeDeclStatement:
			if stmt != nil && stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.UnitDeclStatement:
			if stmt != nil && stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.EnumDeclaration:
			if stmt != nil && stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.InterfaceDeclaration:
			if stmt != nil && stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.StructStatement:
			if stmt != nil && stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		}
	}

	for _, stmt := range program.Statements {
		if stmt == nil {
			continue
		}
		qualifyLocalTypeReferencesInStatement(stmt, module, localTypes)
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			if stmt.Name == nil || strings.Contains(stmt.Name.Value, ".") {
				continue
			}
			qualifyLocalCalls(stmt.Body, module, localFunctions)
			stmt.Name.Value = module + "." + stmt.Name.Value
			stmt.Name.Token.Lexeme = stmt.Name.Value
		case *ast.ImplStatement:
			qualifyLocalCallsInImplMembers(stmt.Members, module, localFunctions)
		case *ast.TypeDeclStatement:
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.UnitDeclStatement:
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.EnumDeclaration:
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.InterfaceDeclaration:
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.StructStatement:
			qualifyIdentifierDeclaration(stmt.Name, module)
		}
	}
}

func qualifyIdentifierDeclaration(ident *ast.Identifier, module string) {
	if ident == nil || strings.Contains(ident.Value, ".") {
		return
	}
	ident.Value = module + "." + ident.Value
	ident.Token.Lexeme = ident.Value
}

func rewriteImportQualifier(program *ast.Program, from string, to string) {
	if program == nil || from == "" || to == "" || from == to {
		return
	}
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			rewriteQualifierInBlock(stmt.Body, from, to)
		case *ast.ImplStatement:
			for _, member := range stmt.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok {
					rewriteQualifierInBlock(fn.Body, from, to)
				}
			}
		default:
			rewriteQualifierInStatement(stmt, from, to)
		}
	}
}

func rewriteQualifierInBlock(block *ast.BlockStatement, from string, to string) {
	if block == nil {
		return
	}
	for _, stmt := range block.Statements {
		rewriteQualifierInStatement(stmt, from, to)
	}
}

func rewriteQualifierInStatement(stmt ast.Statement, from string, to string) {
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		rewriteQualifierInExpression(stmt.Value, from, to)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			rewriteQualifierInStatement(let, from, to)
		}
	case *ast.AssignmentStatement:
		rewriteQualifierInExpression(stmt.Target, from, to)
		rewriteQualifierInExpression(stmt.Value, from, to)
	case *ast.ExpressionStatement:
		rewriteQualifierInExpression(stmt.Expression, from, to)
	case *ast.ReturnStatement:
		rewriteQualifierInExpression(stmt.Value, from, to)
	case *ast.MatchStatement:
		if stmt.Match != nil {
			rewriteQualifierInExpression(stmt.Match, from, to)
		}
	case *ast.IfStatement:
		rewriteQualifierInExpression(stmt.Condition, from, to)
		rewriteQualifierInBlock(stmt.Consequence, from, to)
		rewriteQualifierInBlock(stmt.Alternative, from, to)
	case *ast.ForStatement:
		rewriteQualifierInExpression(stmt.Iterable, from, to)
		rewriteQualifierInExpression(stmt.Step, from, to)
		rewriteQualifierInBlock(stmt.Body, from, to)
	case *ast.WhileStatement:
		rewriteQualifierInExpression(stmt.Condition, from, to)
		rewriteQualifierInBlock(stmt.Body, from, to)
	case *ast.UnsafeStatement:
		rewriteQualifierInBlock(stmt.Body, from, to)
	}
}

func rewriteQualifierInExpression(expr ast.Expression, from string, to string) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		if expr.Value == from {
			expr.Value = to
			expr.Token.Lexeme = to
		}
	case *ast.CallExpression:
		if expr.Function != nil {
			expr.Function.Value = rewriteQualifiedName(expr.Function.Value, from, to)
			expr.Function.Token.Lexeme = expr.Function.Value
		}
		rewriteQualifierInExpression(expr.Callee, from, to)
		for _, arg := range expr.Arguments {
			rewriteQualifierInExpression(arg, from, to)
		}
	case *ast.MemberExpression:
		rewriteQualifierInExpression(expr.Object, from, to)
	case *ast.IndexExpression:
		rewriteQualifierInExpression(expr.Left, from, to)
		rewriteQualifierInExpression(expr.Index, from, to)
	case *ast.PrefixExpression:
		rewriteQualifierInExpression(expr.Right, from, to)
	case *ast.InfixExpression:
		rewriteQualifierInExpression(expr.Left, from, to)
		rewriteQualifierInExpression(expr.Right, from, to)
	case *ast.ConversionExpression:
		if expr.Type != nil {
			expr.Type.Name = rewriteQualifiedName(expr.Type.Name, from, to)
		}
		rewriteQualifierInExpression(expr.Value, from, to)
	case *ast.RangeExpression:
		rewriteQualifierInExpression(expr.Start, from, to)
		rewriteQualifierInExpression(expr.End, from, to)
	case *ast.RefExpression:
		rewriteQualifierInExpression(expr.Value, from, to)
	case *ast.MatchExpression:
		rewriteQualifierInExpression(expr.Subject, from, to)
		for _, arm := range expr.Arms {
			rewriteQualifierInMatchPattern(arm.Pattern, from, to)
			rewriteQualifierInExpression(arm.Guard, from, to)
			rewriteQualifierInExpression(arm.Body, from, to)
			if arm.ReturnBody != nil {
				rewriteQualifierInStatement(arm.ReturnBody, from, to)
			}
			rewriteQualifierInBlock(arm.BlockBody, from, to)
		}
	}
}

func rewriteQualifiedName(name string, from string, to string) string {
	if name == from {
		return to
	}
	if strings.HasPrefix(name, from+".") {
		return to + strings.TrimPrefix(name, from)
	}
	return name
}

func rewriteQualifierInMatchPattern(pattern *ast.MatchPattern, from string, to string) {
	if pattern == nil {
		return
	}
	pattern.Name = rewriteQualifiedName(pattern.Name, from, to)
}

func qualifyLocalTypesInMatchPattern(pattern *ast.MatchPattern, module string, localTypes map[string]bool) {
	if pattern == nil {
		return
	}
	separator := strings.LastIndex(pattern.Name, ".")
	if separator > 0 && localTypes[pattern.Name[:separator]] {
		pattern.Name = module + "." + pattern.Name
	}
}

func qualifyLocalTypeReferencesInStatement(stmt ast.Statement, module string, localTypes map[string]bool) {
	switch stmt := stmt.(type) {
	case *ast.TypeDeclStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypeReference(stmt.BaseType, module, localTypes)
		qualifyLocalTypeReference(stmt.AssignedType, module, localTypes)
		for _, ref := range stmt.Implements {
			qualifyLocalTypeReference(ref, module, localTypes)
		}
		if stmt.StructType != nil {
			for _, field := range stmt.StructType.Fields {
				qualifyLocalTypeReference(field.Type, module, localTypes)
			}
		}
		if stmt.RegisterType != nil {
			for _, field := range stmt.RegisterType.Fields {
				qualifyLocalTypeReference(field.Type, module, localTypes)
			}
		}
		for _, variant := range stmt.UnionVariants {
			qualifyLocalTypeReference(variant.Payload, module, localTypes)
			for _, field := range variant.PayloadFields {
				qualifyLocalTypeReference(field.Type, module, localTypes)
			}
		}
	case *ast.UnitDeclStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypeReference(stmt.BaseType, module, localTypes)
	case *ast.InterfaceDeclaration:
		if stmt == nil {
			return
		}
		for _, ref := range stmt.Implements {
			qualifyLocalTypeReference(ref, module, localTypes)
		}
		for _, method := range stmt.Methods {
			qualifyLocalTypeReferencesInFunction(method, module, localTypes)
		}
		for _, property := range stmt.Properties {
			if property != nil {
				qualifyLocalTypeReference(property.Type, module, localTypes)
			}
		}
		for _, event := range stmt.Events {
			if event != nil {
				qualifyLocalTypeReference(event.Payload, module, localTypes)
			}
		}
	case *ast.StructStatement:
		if stmt == nil {
			return
		}
		for _, field := range stmt.Fields {
			qualifyLocalTypeReference(field.Type, module, localTypes)
		}
	case *ast.FunctionDeclaration:
		qualifyLocalTypeReferencesInFunction(stmt, module, localTypes)
	case *ast.ImplStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypeReference(stmt.Target, module, localTypes)
		for _, member := range stmt.Members {
			qualifyLocalTypeReferencesInImplMember(member, module, localTypes)
		}
	case *ast.LetStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypeReference(stmt.Type, module, localTypes)
		qualifyLocalTypesInExpression(stmt.Value, module, localTypes)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			qualifyLocalTypeReferencesInStatement(let, module, localTypes)
		}
	case *ast.AssignmentStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Target, module, localTypes)
		qualifyLocalTypesInExpression(stmt.Value, module, localTypes)
	case *ast.ExpressionStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Expression, module, localTypes)
	case *ast.ReturnStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Value, module, localTypes)
	case *ast.MatchStatement:
		if stmt != nil {
			qualifyLocalTypesInExpression(stmt.Match, module, localTypes)
		}
	case *ast.IfStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Condition, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Consequence, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Alternative, module, localTypes)
	case *ast.ForStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Iterable, module, localTypes)
		qualifyLocalTypesInExpression(stmt.Step, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Body, module, localTypes)
	case *ast.WhileStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Condition, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Body, module, localTypes)
	case *ast.SwitchStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Subject, module, localTypes)
		for _, clause := range stmt.Cases {
			qualifyLocalTypesInSwitchCase(clause, module, localTypes)
		}
		qualifyLocalTypesInSwitchCase(stmt.Default, module, localTypes)
	case *ast.SelectStatement:
		if stmt == nil {
			return
		}
		for _, branch := range stmt.Branches {
			if branch == nil {
				continue
			}
			qualifyLocalTypesInExpression(branch.Value, module, localTypes)
			qualifyLocalTypeReferencesInBlock(branch.Body, module, localTypes)
		}
	case *ast.UnsafeStatement:
		if stmt != nil {
			qualifyLocalTypeReferencesInBlock(stmt.Body, module, localTypes)
		}
	}
}

func qualifyLocalTypesInSwitchCase(clause *ast.SwitchCase, module string, localTypes map[string]bool) {
	if clause == nil {
		return
	}
	for _, item := range clause.Items {
		switch item := item.(type) {
		case *ast.SwitchValueCase:
			qualifyLocalTypesInExpression(item.Value, module, localTypes)
		case *ast.SwitchRangeCase:
			qualifyLocalTypesInExpression(item.Range, module, localTypes)
		case *ast.SwitchRelationalCase:
			qualifyLocalTypesInExpression(item.Value, module, localTypes)
		}
	}
	qualifyLocalTypeReferencesInBlock(clause.Body, module, localTypes)
}

func qualifyLocalTypeReferencesInImplMember(member ast.ImplMember, module string, localTypes map[string]bool) {
	switch member := member.(type) {
	case *ast.TypeDeclStatement:
		qualifyLocalTypeReferencesInStatement(member, module, localTypes)
	case *ast.UnitDeclStatement:
		qualifyLocalTypeReferencesInStatement(member, module, localTypes)
	case *ast.EnumDeclaration:
		qualifyLocalTypeReferencesInStatement(member, module, localTypes)
	case *ast.FunctionDeclaration:
		qualifyLocalTypeReferencesInFunction(member, module, localTypes)
	case *ast.PropertyDeclaration:
		if member == nil {
			return
		}
		qualifyLocalTypeReference(member.Type, module, localTypes)
		qualifyLocalTypeReferencesInBlock(member.Getter, module, localTypes)
		if member.Setter != nil {
			qualifyLocalTypeReferencesInBlock(member.Setter.Body, module, localTypes)
		}
	}
}

func qualifyLocalTypeReferencesInFunction(fn *ast.FunctionDeclaration, module string, localTypes map[string]bool) {
	if fn == nil {
		return
	}
	for _, parameter := range fn.Parameters {
		qualifyLocalTypeReference(parameter.Type, module, localTypes)
	}
	qualifyLocalTypeReference(fn.ReturnType, module, localTypes)
	qualifyLocalTypeReferencesInBlock(fn.Body, module, localTypes)
}

func qualifyLocalTypeReferencesInBlock(block *ast.BlockStatement, module string, localTypes map[string]bool) {
	if block == nil {
		return
	}
	for _, stmt := range block.Statements {
		qualifyLocalTypeReferencesInStatement(stmt, module, localTypes)
	}
}

func qualifyLocalTypeReference(ref *ast.TypeReference, module string, localTypes map[string]bool) {
	if ref == nil {
		return
	}
	if localTypes[ref.Name] {
		ref.Name = module + "." + ref.Name
		ref.Token.Lexeme = ref.Name
	}
	qualifyLocalTypeReference(ref.ElementType, module, localTypes)
	for _, arg := range ref.TypeArgs {
		qualifyLocalTypeReference(arg, module, localTypes)
	}
	for _, param := range ref.FunctionParameterTypes {
		qualifyLocalTypeReference(param, module, localTypes)
	}
	qualifyLocalTypeReference(ref.FunctionReturnType, module, localTypes)
	qualifyLocalTypesInExpression(ref.ArrayLengthExpression, module, localTypes)
}

func qualifyLocalTypesInExpression(expr ast.Expression, module string, localTypes map[string]bool) {
	switch expr := expr.(type) {
	case *ast.CallExpression:
		if expr.Function != nil && localTypes[expr.Function.Value] {
			expr.Function.Value = module + "." + expr.Function.Value
			expr.Function.Token.Lexeme = expr.Function.Value
		}
		for _, arg := range expr.GenericArguments {
			qualifyLocalTypeReference(arg, module, localTypes)
		}
		qualifyLocalTypesInExpression(expr.Callee, module, localTypes)
		for _, arg := range expr.Arguments {
			qualifyLocalTypesInExpression(arg, module, localTypes)
		}
	case *ast.MemberExpression:
		if ident, ok := expr.Object.(*ast.Identifier); ok && localTypes[ident.Value] {
			ident.Value = module + "." + ident.Value
			ident.Token.Lexeme = ident.Value
		} else {
			qualifyLocalTypesInExpression(expr.Object, module, localTypes)
		}
	case *ast.ConversionExpression:
		qualifyLocalTypeReference(expr.Type, module, localTypes)
		qualifyLocalTypesInExpression(expr.Value, module, localTypes)
	case *ast.PrefixExpression:
		qualifyLocalTypesInExpression(expr.Right, module, localTypes)
	case *ast.InfixExpression:
		qualifyLocalTypesInExpression(expr.Left, module, localTypes)
		qualifyLocalTypesInExpression(expr.Right, module, localTypes)
	case *ast.RangeExpression:
		qualifyLocalTypesInExpression(expr.Start, module, localTypes)
		qualifyLocalTypesInExpression(expr.End, module, localTypes)
	case *ast.IndexExpression:
		qualifyLocalTypesInExpression(expr.Left, module, localTypes)
		qualifyLocalTypesInExpression(expr.Index, module, localTypes)
	case *ast.RefExpression:
		qualifyLocalTypesInExpression(expr.Value, module, localTypes)
	case *ast.StructLiteral:
		qualifyLocalTypeReference(expr.Type, module, localTypes)
		for _, field := range expr.Fields {
			qualifyLocalTypesInExpression(field.Value, module, localTypes)
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			qualifyLocalTypesInExpression(element, module, localTypes)
		}
	case *ast.OkExpression:
		qualifyLocalTypesInExpression(expr.Value, module, localTypes)
	case *ast.ErrExpression:
		qualifyLocalTypesInExpression(expr.Value, module, localTypes)
	case *ast.TryExpression:
		qualifyLocalTypesInExpression(expr.Expression, module, localTypes)
	case *ast.MatchExpression:
		qualifyLocalTypesInExpression(expr.Subject, module, localTypes)
		for _, arm := range expr.Arms {
			qualifyLocalTypesInMatchPattern(arm.Pattern, module, localTypes)
			qualifyLocalTypesInExpression(arm.Guard, module, localTypes)
			qualifyLocalTypesInExpression(arm.Body, module, localTypes)
			if arm.ReturnBody != nil {
				qualifyLocalTypeReferencesInStatement(arm.ReturnBody, module, localTypes)
			}
			qualifyLocalTypeReferencesInBlock(arm.BlockBody, module, localTypes)
		}
	}
}

func qualifyLocalCalls(block *ast.BlockStatement, module string, localFunctions map[string]bool) {
	if block == nil {
		return
	}
	for _, stmt := range block.Statements {
		qualifyLocalCallsInStatement(stmt, module, localFunctions)
	}
}

func qualifyLocalCallsInImplMembers(members []ast.ImplMember, module string, localFunctions map[string]bool) {
	for _, member := range members {
		switch member := member.(type) {
		case *ast.FunctionDeclaration:
			qualifyLocalCalls(member.Body, module, localFunctions)
		case *ast.PropertyDeclaration:
			if member == nil {
				continue
			}
			qualifyLocalCalls(member.Getter, module, localFunctions)
			if member.Setter != nil {
				qualifyLocalCalls(member.Setter.Body, module, localFunctions)
			}
		}
	}
}

func qualifyLocalCallsInStatement(stmt ast.Statement, module string, localFunctions map[string]bool) {
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		qualifyLocalCallsInExpression(stmt.Value, module, localFunctions)
	case *ast.AssignmentStatement:
		qualifyLocalCallsInExpression(stmt.Target, module, localFunctions)
		qualifyLocalCallsInExpression(stmt.Value, module, localFunctions)
	case *ast.TryAssignmentStatement:
		if stmt.Assignment != nil {
			qualifyLocalCallsInStatement(stmt.Assignment, module, localFunctions)
		}
	case *ast.ExpressionStatement:
		qualifyLocalCallsInExpression(stmt.Expression, module, localFunctions)
	case *ast.ReturnStatement:
		qualifyLocalCallsInExpression(stmt.Value, module, localFunctions)
	case *ast.MatchStatement:
		if stmt.Match != nil {
			qualifyLocalCallsInExpression(stmt.Match, module, localFunctions)
		}
	case *ast.IfStatement:
		qualifyLocalCallsInExpression(stmt.Condition, module, localFunctions)
		qualifyLocalCalls(stmt.Consequence, module, localFunctions)
		qualifyLocalCalls(stmt.Alternative, module, localFunctions)
	case *ast.ForStatement:
		qualifyLocalCallsInExpression(stmt.Iterable, module, localFunctions)
		qualifyLocalCallsInExpression(stmt.Step, module, localFunctions)
		qualifyLocalCalls(stmt.Body, module, localFunctions)
	case *ast.WhileStatement:
		qualifyLocalCallsInExpression(stmt.Condition, module, localFunctions)
		qualifyLocalCalls(stmt.Body, module, localFunctions)
	case *ast.UnsafeStatement:
		qualifyLocalCalls(stmt.Body, module, localFunctions)
	}
}

func qualifyLocalCallsInExpression(expr ast.Expression, module string, localFunctions map[string]bool) {
	switch expr := expr.(type) {
	case *ast.CallExpression:
		if expr.Function != nil && localFunctions[expr.Function.Value] {
			expr.Function.Value = module + "." + expr.Function.Value
			expr.Function.Token.Lexeme = expr.Function.Value
		}
		qualifyLocalCallsInExpression(expr.Callee, module, localFunctions)
		for _, arg := range expr.Arguments {
			qualifyLocalCallsInExpression(arg, module, localFunctions)
		}
	case *ast.PrefixExpression:
		qualifyLocalCallsInExpression(expr.Right, module, localFunctions)
	case *ast.InfixExpression:
		qualifyLocalCallsInExpression(expr.Left, module, localFunctions)
		qualifyLocalCallsInExpression(expr.Right, module, localFunctions)
	case *ast.RangeExpression:
		qualifyLocalCallsInExpression(expr.Start, module, localFunctions)
		qualifyLocalCallsInExpression(expr.End, module, localFunctions)
	case *ast.MemberExpression:
		qualifyLocalCallsInExpression(expr.Object, module, localFunctions)
	case *ast.ConversionExpression:
		qualifyLocalCallsInExpression(expr.Value, module, localFunctions)
	case *ast.TryExpression:
		qualifyLocalCallsInExpression(expr.Expression, module, localFunctions)
	case *ast.OkExpression:
		if expr.Value != nil {
			qualifyLocalCallsInExpression(expr.Value, module, localFunctions)
		}
	case *ast.ErrExpression:
		qualifyLocalCallsInExpression(expr.Value, module, localFunctions)
	case *ast.MatchExpression:
		qualifyLocalCallsInExpression(expr.Subject, module, localFunctions)
		for _, arm := range expr.Arms {
			qualifyLocalCallsInExpression(arm.Guard, module, localFunctions)
			qualifyLocalCallsInExpression(arm.Body, module, localFunctions)
			if arm.ReturnBody != nil {
				qualifyLocalCallsInStatement(arm.ReturnBody, module, localFunctions)
			}
			qualifyLocalCalls(arm.BlockBody, module, localFunctions)
		}
	}
}

func printProgram(program *ast.Program) {
	for i, stmt := range program.Statements {
		if i > 0 {
			fmt.Println()
		}

		printStatement(stmt)
	}
}

func printAST(program *ast.Program) {
	fmt.Println("Program")

	for i, stmt := range program.Statements {
		printASTStatement(stmt, "", i == len(program.Statements)-1)
	}
}

func printASTStatement(stmt ast.Statement, prefix string, last bool) {
	switch stmt := stmt.(type) {
	case *ast.TargetDirective:
		printASTBranch(prefix, last, "Target")
		children := []string{
			"OS: " + stmt.OS,
			"Arch: " + stmt.Arch,
		}
		printASTLeaves(childPrefix(prefix, last), children)

	case *ast.ModuleStatement:
		printASTBranch(prefix, last, "Module")
		printASTLeaf(childPrefix(prefix, last), true, "Path: "+stmt.Path)

	case *ast.ImportStatement:
		printASTBranch(prefix, last, "Import")
		children := []string{}
		if stmt.Alias != "" {
			children = append(children, "Alias: "+stmt.Alias)
		}
		children = append(children, fmt.Sprintf("Path: %q", stmt.Path))
		printASTLeaves(childPrefix(prefix, last), children)

	case *ast.TypeDeclStatement:
		printASTTypeDecl(stmt, prefix, last)

	case *ast.UnitDeclStatement:
		printASTBranch(prefix, last, "Unit")
		children := []string{
			"Name: " + stmt.Name.Value,
			"Base: " + formatTypeRef(stmt.BaseType),
		}
		if stmt.Category != "" {
			children = append(children, "Category: "+stmt.Category)
		}
		printASTLeaves(childPrefix(prefix, last), children)

	case *ast.EnumDeclaration:
		printASTEnum(stmt, prefix, last)

	case *ast.InterfaceDeclaration:
		printASTInterface(stmt, prefix, last)

	case *ast.FunctionDeclaration:
		printASTFunction(stmt, prefix, last)

	case *ast.StructStatement:
		printASTBranch(prefix, last, "Struct")
		childrenPrefix := childPrefix(prefix, last)
		printASTLeaf(childrenPrefix, len(stmt.Fields) == 0, "Name: "+stmt.Name.Value)
		for i, field := range stmt.Fields {
			printASTField(childrenPrefix, field, i == len(stmt.Fields)-1)
		}

	case *ast.LetStatement:
		printASTLet(stmt, prefix, last)

	case *ast.LetGroupStatement:
		printASTBranch(prefix, last, "LetGroup")
		childrenPrefix := childPrefix(prefix, last)
		for i, let := range stmt.Lets {
			printASTLet(let, childrenPrefix, i == len(stmt.Lets)-1)
		}

	case *ast.AssignmentStatement:
		printASTAssignment(stmt, prefix, last)

	case *ast.TryAssignmentStatement:
		printASTBranch(prefix, last, "TryAssignment")
		if stmt.Assignment != nil {
			printASTAssignment(stmt.Assignment, childPrefix(prefix, last), true)
		}

	case *ast.DeferStatement:
		printASTBranch(prefix, last, "Defer")
		if stmt.Body != nil {
			printASTBlock(childPrefix(prefix, last), true, "Body", stmt.Body)
		}

	case *ast.DiscardStatement:
		printASTBranch(prefix, last, "Discard")
		value := "<nil>"
		if stmt.Value != nil {
			value = stmt.Value.String()
		}
		printASTLeaf(childPrefix(prefix, last), true, "Value: "+value)

	case *ast.DetachStatement:
		printASTBranch(prefix, last, "Detach")
		value := "<nil>"
		if stmt.Value != nil {
			value = stmt.Value.String()
		}
		children := []string{"Value: " + value}
		if stmt.DiscardResult {
			children = append(children, "DiscardResult: true")
		}
		printASTLeaves(childPrefix(prefix, last), children)

	case *ast.CancelStatement:
		printASTBranch(prefix, last, "Cancel")

	case *ast.ExpressionStatement:
		printASTExpression(prefix, last, "Expression", stmt.Expression)

	case *ast.ReturnStatement:
		printASTReturn(stmt, prefix, last)

	case *ast.IfStatement:
		printASTIf(stmt, prefix, last)

	case *ast.ForStatement:
		printASTFor(stmt, prefix, last)

	case *ast.WhileStatement:
		printASTWhile(stmt, prefix, last)

	case *ast.SwitchStatement:
		printASTSwitch(stmt, prefix, last)

	case *ast.MatchStatement:
		printASTExpression(prefix, last, "Match", stmt.Match)

	case *ast.ImplStatement:
		printASTImpl(stmt, prefix, last)

	case *ast.CommentStatement:
		printASTBranch(prefix, last, "Comment")
		printASTLeaf(childPrefix(prefix, last), true, fmt.Sprintf("Text: %q", stmt.Text))

	case *ast.InvalidStatement:
		printASTBranch(prefix, last, "Invalid")
		printASTLeaf(childPrefix(prefix, last), true, "Token: "+stmt.TokenLiteral())

	case *ast.FallthroughStatement:
		printASTBranch(prefix, last, "Fallthrough")

	case *ast.BreakStatement:
		printASTBranch(prefix, last, "Break")

	case *ast.ContinueStatement:
		printASTBranch(prefix, last, "Continue")

	case *ast.UnsafeStatement:
		printASTUnsafe(stmt, prefix, last)

	case *ast.AsmStatement:
		printASTBranch(prefix, last, "Asm")
		childrenPrefix := childPrefix(prefix, last)
		if stmt.Block != nil {
			printASTAsmBlock(childrenPrefix, stmt.Block)
			return
		}
		template := "<nil>"
		if stmt.Template != nil {
			template = fmt.Sprintf("%q", stmt.Template.Value)
		}
		printASTLeaf(childrenPrefix, true, "Template: "+template)

	default:
		printASTBranch(prefix, last, fmt.Sprintf("%T", stmt))
		printASTLeaf(childPrefix(prefix, last), true, "Token: "+stmt.TokenLiteral())
	}
}

func printASTAsmBlock(prefix string, block *ast.AsmBlock) {
	template := "<nil>"
	if block.Template != nil {
		template = fmt.Sprintf("%q", block.Template.Value)
	}
	printASTLeaf(prefix, false, "Template: "+template)
	printASTLeaf(prefix, len(block.Outputs) == 0 && len(block.Clobbers) == 0, "Inputs: "+formatAsmInputs(block.Inputs))
	if len(block.Outputs) > 0 {
		printASTLeaf(prefix, len(block.Clobbers) == 0, "Outputs: "+formatAsmOutputs(block.Outputs))
	}
	if len(block.Clobbers) > 0 {
		printASTLeaf(prefix, true, "Clobbers: "+formatStringList(block.Clobbers))
	}
}

func formatAsmInputs(inputs []ast.AsmOperand) string {
	out := ""
	for i, input := range inputs {
		if i > 0 {
			out += ", "
		}
		value := "<nil>"
		if input.Value != nil {
			value = input.Value.String()
		}
		out += input.Register + "(" + value + ")"
	}
	return out
}

func formatAsmOutputs(outputs []ast.AsmOutput) string {
	out := ""
	for i, output := range outputs {
		if i > 0 {
			out += ", "
		}
		out += output.Register
		if output.Name != "" {
			out += "(" + output.Name + ")"
		}
	}
	return out
}

func formatStringList(values []string) string {
	out := ""
	for i, value := range values {
		if i > 0 {
			out += ", "
		}
		out += value
	}
	return out
}

func printASTIf(stmt *ast.IfStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "If")
	childrenPrefix := childPrefix(prefix, last)
	printASTExpression(childrenPrefix, false, "Condition", stmt.Condition)

	hasAlternative := stmt.Alternative != nil
	printASTBranch(childrenPrefix, !hasAlternative, "Then")
	thenPrefix := childPrefix(childrenPrefix, !hasAlternative)
	for i, bodyStmt := range stmt.Consequence.Statements {
		printASTStatement(bodyStmt, thenPrefix, i == len(stmt.Consequence.Statements)-1)
	}

	if hasAlternative {
		printASTBranch(childrenPrefix, true, "Else")
		elsePrefix := childPrefix(childrenPrefix, true)
		for i, bodyStmt := range stmt.Alternative.Statements {
			printASTStatement(bodyStmt, elsePrefix, i == len(stmt.Alternative.Statements)-1)
		}
	}
}

func printASTFor(stmt *ast.ForStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "For")
	childrenPrefix := childPrefix(prefix, last)

	hasBindings := len(stmt.Bindings) > 0
	hasIterable := stmt.Iterable != nil
	hasStep := stmt.Step != nil
	hasBody := stmt.Body != nil && len(stmt.Body.Statements) > 0

	if hasBindings {
		printASTBranch(childrenPrefix, !(hasIterable || hasStep || hasBody), "Bindings")
		bindingsPrefix := childPrefix(childrenPrefix, !(hasIterable || hasStep || hasBody))
		for i, binding := range stmt.Bindings {
			label := "Name: " + binding.Name
			if binding.Discard {
				label = "Discard: _"
			}
			printASTLeaf(bindingsPrefix, i == len(stmt.Bindings)-1, label)
		}
	}

	if hasIterable {
		printASTExpression(childrenPrefix, !(hasStep || hasBody), "Iterable", stmt.Iterable)
	}

	if hasStep {
		printASTExpression(childrenPrefix, !hasBody, "Step", stmt.Step)
	}

	if stmt.Body != nil {
		printASTBranch(childrenPrefix, true, "Body")
		bodyPrefix := childPrefix(childrenPrefix, true)
		for i, bodyStmt := range stmt.Body.Statements {
			printASTStatement(bodyStmt, bodyPrefix, i == len(stmt.Body.Statements)-1)
		}
	}
}

func printASTWhile(stmt *ast.WhileStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "While")
	childrenPrefix := childPrefix(prefix, last)
	hasBody := stmt.Body != nil && len(stmt.Body.Statements) > 0
	printASTExpression(childrenPrefix, !hasBody, "Condition", stmt.Condition)
	if stmt.Body != nil {
		printASTBranch(childrenPrefix, true, "Body")
		bodyPrefix := childPrefix(childrenPrefix, true)
		for i, bodyStmt := range stmt.Body.Statements {
			printASTStatement(bodyStmt, bodyPrefix, i == len(stmt.Body.Statements)-1)
		}
	}
}

func printASTSwitch(stmt *ast.SwitchStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Switch")
	childrenPrefix := childPrefix(prefix, last)
	hasClauses := len(stmt.Cases) > 0 || stmt.Default != nil
	printASTExpression(childrenPrefix, !hasClauses, "Subject", stmt.Subject)

	for i, caseClause := range stmt.Cases {
		printASTSwitchCase(childrenPrefix, caseClause, stmt.Default == nil && i == len(stmt.Cases)-1)
	}
	if stmt.Default != nil {
		printASTSwitchCase(childrenPrefix, stmt.Default, true)
	}
}

func printASTSwitchCase(prefix string, caseClause *ast.SwitchCase, last bool) {
	label := "Case"
	if caseClause.Default {
		label = "Default"
	}
	printASTBranch(prefix, last, label)
	childrenPrefix := childPrefix(prefix, last)

	hasBody := caseClause.Body != nil && len(caseClause.Body.Statements) > 0
	if !caseClause.Default {
		printASTBranch(childrenPrefix, !hasBody, "Items")
		itemsPrefix := childPrefix(childrenPrefix, !hasBody)
		for i, item := range caseClause.Items {
			printASTSwitchCaseItem(itemsPrefix, item, i == len(caseClause.Items)-1)
		}
	}

	if hasBody {
		printASTBranch(childrenPrefix, true, "Body")
		bodyPrefix := childPrefix(childrenPrefix, true)
		for i, bodyStmt := range caseClause.Body.Statements {
			printASTStatement(bodyStmt, bodyPrefix, i == len(caseClause.Body.Statements)-1)
		}
	}
}

func printASTSwitchCaseItem(prefix string, item ast.SwitchCaseItem, last bool) {
	switch item := item.(type) {
	case *ast.SwitchValueCase:
		printASTExpression(prefix, last, "Value", item.Value)
	case *ast.SwitchRangeCase:
		printASTExpression(prefix, last, "Range", item.Range)
	case *ast.SwitchRelationalCase:
		printASTBranch(prefix, last, "Relational("+item.Operator+")")
		printASTExpression(childPrefix(prefix, last), true, "Value", item.Value)
	default:
		printASTLeaf(prefix, last, fmt.Sprintf("%T", item))
	}
}

func printASTBlock(prefix string, last bool, label string, block *ast.BlockStatement) {
	printASTBranch(prefix, last, label)
	if block == nil {
		return
	}
	bodyPrefix := childPrefix(prefix, last)
	for i, bodyStmt := range block.Statements {
		printASTStatement(bodyStmt, bodyPrefix, i == len(block.Statements)-1)
	}
}

func printASTUnsafe(stmt *ast.UnsafeStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Unsafe")
	childrenPrefix := childPrefix(prefix, last)
	printASTBranch(childrenPrefix, true, "Body")
	bodyPrefix := childPrefix(childrenPrefix, true)
	if stmt.Body == nil {
		return
	}
	for i, bodyStmt := range stmt.Body.Statements {
		printASTStatement(bodyStmt, bodyPrefix, i == len(stmt.Body.Statements)-1)
	}
}

func printASTImpl(stmt *ast.ImplStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Impl")

	childrenPrefix := childPrefix(prefix, last)
	printASTLeaf(childrenPrefix, len(stmt.Members) == 0, "Target: "+formatTypeRef(stmt.Target))

	for i, member := range stmt.Members {
		printASTImplMember(childrenPrefix, member, i == len(stmt.Members)-1)
	}
}

func printASTImplMember(prefix string, member ast.ImplMember, last bool) {
	switch member := member.(type) {
	case *ast.TypeDeclStatement:
		printASTTypeDecl(member, prefix, last)
	case *ast.UnitDeclStatement:
		printASTBranch(prefix, last, "Unit")
		children := []string{
			"Name: " + member.Name.Value,
			"Base: " + formatTypeRef(member.BaseType),
		}
		if member.Category != "" {
			children = append(children, "Category: "+member.Category)
		}
		printASTLeaves(childPrefix(prefix, last), children)
	case *ast.EnumDeclaration:
		printASTEnum(member, prefix, last)
	case *ast.FunctionDeclaration:
		printASTFunction(member, prefix, last)
	case *ast.PropertyDeclaration:
		printASTBranch(prefix, last, "Property")
		children := []string{
			"Name: " + member.Name.Value,
			"Type: " + formatTypeRef(member.Type),
		}
		if member.Getter != nil {
			children = append(children, "Getter: true")
		}
		if member.Setter != nil {
			children = append(children, fmt.Sprintf("Setter: %s fallible=%t", member.Setter.Parameter.Value, member.Setter.Fallible))
		}
		printASTLeaves(childPrefix(prefix, last), children)
	default:
		printASTBranch(prefix, last, fmt.Sprintf("%T", member))
		printASTLeaf(childPrefix(prefix, last), true, "Token: "+member.TokenLiteral())
	}
}

func printASTLet(stmt *ast.LetStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Let")

	childrenPrefix := childPrefix(prefix, last)
	children := []string{
		fmt.Sprintf("Mutable: %t", stmt.Mutable),
		"Name: " + stmt.Name.Value,
	}
	if stmt.Ownership == ast.OwnershipMove {
		children = append(children, "Ownership: move")
	}

	if stmt.Type != nil {
		children = append(children, "Type: "+formatTypeRef(stmt.Type))
	}
	if stmt.Contract != nil {
		children = append(children, formatASTContract(stmt.Contract))
	}
	if stmt.Address != nil {
		children = append(children, "Address: "+stmt.Address.String())
	}

	if stmt.Value == nil {
		printASTLeaves(childrenPrefix, children)
		return
	}

	for _, child := range children {
		printASTLeaf(childrenPrefix, false, child)
	}

	printASTExpression(childrenPrefix, true, "Value", stmt.Value)
}

func printASTAssignment(stmt *ast.AssignmentStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Assignment")

	childrenPrefix := childPrefix(prefix, last)
	printASTExpression(childrenPrefix, false, "Target", stmt.Target)
	printASTLeaf(childrenPrefix, false, "Operator: "+stmt.Operator)
	if stmt.Ownership == ast.OwnershipMove {
		printASTLeaf(childrenPrefix, false, "Ownership: move")
	}
	printASTExpression(childrenPrefix, true, "Value", stmt.Value)
}

func printASTTypeDecl(stmt *ast.TypeDeclStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Type")

	children := []string{
		"Name: " + stmt.Name.Value,
		"GenericParameters: " + formatGenericParameters(stmt.GenericParameters),
	}

	if stmt.BaseType != nil {
		children = append(children, "Base: "+formatTypeRef(stmt.BaseType))
	}

	if stmt.AssignedType != nil {
		children = append(children, "Assigned: "+formatTypeRef(stmt.AssignedType))
	}

	if len(stmt.Implements) > 0 {
		children = append(children, "Implements: "+formatTypeRefs(stmt.Implements))
	}

	if len(stmt.Variants) > 0 {
		children = append(children, "Variants: "+formatVariants(stmt.Variants))
	}

	if stmt.Contract != nil {
		children = append(children, formatASTContract(stmt.Contract))
	}
	if stmt.Default != nil {
		children = append(children, "Default: "+stmt.Default.String())
	}

	childrenPrefix := childPrefix(prefix, last)

	if stmt.StructType == nil && stmt.RegisterType == nil {
		printASTLeaves(childrenPrefix, children)
		return
	}

	for _, child := range children {
		printASTLeaf(childrenPrefix, false, child)
	}

	if stmt.StructType != nil {
		printASTBranch(childrenPrefix, stmt.RegisterType == nil, "Struct")
		structPrefix := childPrefix(childrenPrefix, stmt.RegisterType == nil)
		for i, field := range stmt.StructType.Fields {
			printASTField(structPrefix, field, i == len(stmt.StructType.Fields)-1)
		}
	}
	if stmt.RegisterType != nil {
		printASTBranch(childrenPrefix, true, "Register["+formatRegisterWidth(stmt.RegisterType.WidthExpression, stmt.RegisterType.Width)+"]")
		registerPrefix := childPrefix(childrenPrefix, true)
		for i, field := range stmt.RegisterType.Fields {
			printASTRegisterField(registerPrefix, field, i == len(stmt.RegisterType.Fields)-1)
		}
	}
}

func printASTEnum(stmt *ast.EnumDeclaration, prefix string, last bool) {
	printASTBranch(prefix, last, "Enum")

	children := []string{"Name: " + stmt.Name.Value}
	if stmt.BitUnderlying {
		children = append(children, fmt.Sprintf("Underlying: bit[%d]", stmt.UnderlyingBitWidth))
	} else if stmt.UnderlyingType != nil {
		children = append(children, "Underlying: "+formatTypeRef(stmt.UnderlyingType))
	}
	if len(stmt.Values) > 0 {
		values := ""
		for i, value := range stmt.Values {
			if i > 0 {
				values += ", "
			}
			values += formatEnumValue(value)
		}
		children = append(children, "Values: "+values)
	}

	printASTLeaves(childPrefix(prefix, last), children)
}

func printASTInterface(stmt *ast.InterfaceDeclaration, prefix string, last bool) {
	printASTBranch(prefix, last, "Interface")

	children := []string{
		"Name: " + stmt.Name.Value,
		"GenericParameters: " + formatGenericParameters(stmt.GenericParameters),
	}
	if len(stmt.Implements) > 0 {
		children = append(children, "Implements: "+formatTypeRefs(stmt.Implements))
	}

	childrenPrefix := childPrefix(prefix, last)
	if len(stmt.Methods) == 0 && len(stmt.Properties) == 0 {
		printASTLeaves(childrenPrefix, children)
		return
	}

	for _, child := range children {
		printASTLeaf(childrenPrefix, false, child)
	}
	for _, method := range stmt.Methods {
		printASTBranch(childrenPrefix, false, "Method")
		methodPrefix := childPrefix(childrenPrefix, false)
		printASTLeaf(methodPrefix, false, "Name: "+method.Name.Value)
		printASTLeaf(methodPrefix, false, "Parameters: "+formatParameters(method.Parameters))
		printASTLeaf(methodPrefix, true, "Return: "+formatTypeRef(method.ReturnType))
	}
	for i, property := range stmt.Properties {
		printASTBranch(childrenPrefix, i == len(stmt.Properties)-1, "Property")
		propertyPrefix := childPrefix(childrenPrefix, i == len(stmt.Properties)-1)
		children := []string{
			"Name: " + property.Name.Value,
			"Type: " + formatTypeRef(property.Type),
			fmt.Sprintf("Get: %t", property.RequiresGet),
			fmt.Sprintf("Set: %t", property.RequiresSet),
		}
		printASTLeaves(propertyPrefix, children)
	}
}

func printASTFunction(stmt *ast.FunctionDeclaration, prefix string, last bool) {
	printASTBranch(prefix, last, "Function")
	childrenPrefix := childPrefix(prefix, last)
	printASTLeaf(childrenPrefix, false, fmt.Sprintf("Unsafe: %t", stmt.Unsafe))
	printASTLeaf(childrenPrefix, false, fmt.Sprintf("Extern: %t", stmt.Extern))
	if stmt.Extern {
		printASTLeaf(childrenPrefix, false, "ABI: "+stmt.ABI)
		printASTLeaf(childrenPrefix, false, "LinkName: "+stmt.LinkName)
	}
	printASTLeaf(childrenPrefix, false, "Name: "+stmt.Name.Value)
	printASTLeaf(childrenPrefix, false, "GenericParameters: "+formatGenericParameters(stmt.GenericParameters))
	printASTLeaf(childrenPrefix, false, "Parameters: "+formatParameters(stmt.Parameters))
	printASTLeaf(childrenPrefix, stmt.Body == nil, "Return: "+formatTypeRef(stmt.ReturnType))
	if stmt.Body == nil {
		return
	}
	printASTBranch(childrenPrefix, true, "Body")
	bodyPrefix := childPrefix(childrenPrefix, true)
	for i, bodyStmt := range stmt.Body.Statements {
		printASTStatement(bodyStmt, bodyPrefix, i == len(stmt.Body.Statements)-1)
	}
}

func printASTReturn(stmt *ast.ReturnStatement, prefix string, last bool) {
	printASTBranch(prefix, last, "Return")
	if stmt.Value == nil {
		printASTLeaf(childPrefix(prefix, last), true, "Value: <nil>")
		return
	}
	printASTExpression(childPrefix(prefix, last), true, "Value", stmt.Value)
}

func printASTField(prefix string, field *ast.StructField, last bool) {
	printASTBranch(prefix, last, "Field")
	children := []string{
		"Name: " + field.Name.Value,
		"Type: " + formatTypeRef(field.Type),
	}
	if field.Contract != nil {
		children = append(children, formatASTContract(field.Contract))
	}
	printASTLeaves(childPrefix(prefix, last), children)
}

func printASTRegisterField(prefix string, field *ast.RegisterField, last bool) {
	printASTBranch(prefix, last, "Field")
	fieldType := "bit[" + formatRegisterWidth(field.WidthExpression, field.Width) + "]"
	if field.Type != nil {
		fieldType = formatTypeRef(field.Type)
	}
	children := []string{
		"Name: " + field.Name.Value,
		"Type: " + fieldType,
	}
	if field.Unit != "" {
		children = append(children, "Unit: "+field.Unit)
	}
	printASTLeaves(childPrefix(prefix, last), children)
}

func printASTExpression(prefix string, last bool, role string, expr ast.Expression) {
	if expr == nil {
		printASTLeaf(prefix, last, role+": <nil>")
		return
	}

	switch expr := expr.(type) {
	case *ast.LambdaExpression:
		printASTLambda(prefix, last, role, expr)

	case *ast.PrefixExpression:
		printASTBranch(prefix, last, role+": Prefix("+expr.Operator+")")
		printASTExpression(childPrefix(prefix, last), true, "Right", expr.Right)

	case *ast.InfixExpression:
		printASTBranch(prefix, last, role+": Infix("+expr.Operator+")")
		childrenPrefix := childPrefix(prefix, last)
		printASTExpression(childrenPrefix, false, "Left", expr.Left)
		printASTExpression(childrenPrefix, true, "Right", expr.Right)

	case *ast.TryExpression:
		printASTBranch(prefix, last, role+": Try")
		childrenPrefix := childPrefix(prefix, last)
		hasHandlers := len(expr.Handlers) > 0
		printASTExpression(childrenPrefix, !hasHandlers, "Expression", expr.Expression)
		for i, handler := range expr.Handlers {
			printASTTryHandler(childrenPrefix, handler, i == len(expr.Handlers)-1)
		}

	case *ast.MatchExpression:
		printASTBranch(prefix, last, role+": Match")
		childrenPrefix := childPrefix(prefix, last)
		printASTExpression(childrenPrefix, len(expr.Arms) == 0, "Subject", expr.Subject)
		for i, arm := range expr.Arms {
			printASTMatchArm(childrenPrefix, arm, i == len(expr.Arms)-1)
		}

	default:
		printASTLeaf(prefix, last, role+": "+formatASTExpression(expr))
	}
}

func printASTLambda(prefix string, last bool, role string, expr *ast.LambdaExpression) {
	printASTBranch(prefix, last, role+": Lambda")
	childrenPrefix := childPrefix(prefix, last)
	hasCaptures := len(expr.Captures) > 0
	printASTLeaf(childrenPrefix, false, "Parameters: "+formatParameters(expr.Parameters))
	printASTLeaf(childrenPrefix, false, "Return: "+formatTypeRef(expr.ReturnType))
	if hasCaptures {
		printASTLeaf(childrenPrefix, false, "Captures: "+formatLambdaCaptures(expr.Captures))
	}
	printASTBranch(childrenPrefix, true, "Body")
	bodyPrefix := childPrefix(childrenPrefix, true)
	for i, bodyStmt := range expr.Body.Statements {
		printASTStatement(bodyStmt, bodyPrefix, i == len(expr.Body.Statements)-1)
	}
}

func printASTTryHandler(prefix string, handler *ast.TryHandler, last bool) {
	printASTBranch(prefix, last, "Handler")
	childrenPrefix := childPrefix(prefix, last)
	printASTExpression(childrenPrefix, false, "Pattern", handler.Pattern)

	switch {
	case handler.ReturnBody != nil:
		printASTReturn(handler.ReturnBody, childrenPrefix, true)
	case handler.BlockBody != nil:
		printASTLeaf(childrenPrefix, true, "Body: Block")
	case handler.Body != nil:
		printASTExpression(childrenPrefix, true, "Body", handler.Body)
	default:
		printASTLeaf(childrenPrefix, true, "Body: <nil>")
	}
}

func printASTMatchArm(prefix string, arm *ast.MatchArm, last bool) {
	printASTBranch(prefix, last, "Arm")
	childrenPrefix := childPrefix(prefix, last)
	hasGuard := arm.Guard != nil
	printASTExpression(childrenPrefix, false, "Pattern", arm.Pattern.Expression())
	if hasGuard {
		printASTExpression(childrenPrefix, false, "Guard", arm.Guard)
	}

	switch {
	case arm.ReturnBody != nil:
		printASTReturn(arm.ReturnBody, childrenPrefix, true)
	case arm.BlockBody != nil:
		printASTLeaf(childrenPrefix, true, "Body: Block")
	case arm.Body != nil:
		printASTExpression(childrenPrefix, true, "Body", arm.Body)
	default:
		printASTLeaf(childrenPrefix, true, "Body: <nil>")
	}
}

func printASTLeaves(prefix string, leaves []string) {
	for i, leaf := range leaves {
		printASTLeaf(prefix, i == len(leaves)-1, leaf)
	}
}

func printASTBranch(prefix string, last bool, label string) {
	fmt.Printf("%s%s %s\n", prefix, branch(last), label)
}

func printASTLeaf(prefix string, last bool, label string) {
	fmt.Printf("%s%s %s\n", prefix, branch(last), label)
}

func branch(last bool) string {
	if last {
		return "└─"
	}

	return "├─"
}

func childPrefix(prefix string, last bool) string {
	if last {
		return prefix + "   "
	}

	return prefix + "│  "
}

func formatASTContract(contract ast.Contract) string {
	switch contract := contract.(type) {
	case *ast.ContractList:
		return "Contracts: " + formatContract(contract)
	case *ast.RangeContract:
		return "Range: " + formatRangeContract(contract)
	case *ast.MembershipContract:
		return "In: " + formatMembershipContract(contract)
	case *ast.MarkerContract:
		return "Contract: " + formatMarkerContract(contract)
	default:
		return fmt.Sprintf("Contract: %T", contract)
	}
}

func formatASTExpression(expr ast.Expression) string {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return "Identifier(" + expr.Value + ")"
	case *ast.IntegerLiteral:
		return "Int(" + expr.Token.Lexeme + ")"
	case *ast.FloatLiteral:
		return fmt.Sprintf("Float(%g)", expr.Value)
	case *ast.StringLiteral:
		return fmt.Sprintf("String(%q)", expr.Value)
	case *ast.CharLiteral:
		return fmt.Sprintf("Char(%q)", expr.Value)
	case *ast.BooleanLiteral:
		return fmt.Sprintf("Bool(%t)", expr.Value)
	case *ast.InterpolatedStringLiteral:
		return fmt.Sprintf("InterpolatedString(%q)", expr.Value)
	case *ast.PrefixExpression:
		return "Prefix(" + expr.Operator + ")"
	case *ast.InfixExpression:
		return "Infix(" + expr.Operator + ")"
	case *ast.MemberExpression:
		return "Member(" + formatASTExpression(expr.Object) + "." + expr.Property.Value + ")"
	case *ast.CallExpression:
		return "Call(" + expr.String() + ")"
	case *ast.RuntimeCallExpression:
		return "RuntimeCall(" + expr.String() + ")"
	case *ast.OkExpression:
		return "Ok(" + formatASTExpression(expr.Value) + ")"
	case *ast.ErrExpression:
		return "Err(" + formatASTExpression(expr.Value) + ")"
	case *ast.TryExpression:
		return "Try(" + formatASTExpression(expr.Expression) + ")"
	case *ast.MatchExpression:
		return "Match(" + formatASTExpression(expr.Subject) + ")"
	case *ast.RangeExpression:
		return "Range(" + expr.String() + ")"
	case *ast.StructLiteral:
		return "StructLiteral(" + formatTypeRef(expr.Type) + ")"
	case *ast.LambdaExpression:
		return "Lambda(" + formatParameters(expr.Parameters) + ") " + formatTypeRef(expr.ReturnType)
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func formatVariants(variants []*ast.Identifier) string {
	out := ""

	for i, variant := range variants {
		if i > 0 {
			out += ", "
		}
		out += variant.Value
	}

	return out
}

func printStatement(stmt ast.Statement) {
	switch stmt := stmt.(type) {
	case *ast.TargetDirective:
		fmt.Printf("#target(os: %q, arch: %q)\n", stmt.OS, stmt.Arch)

	case *ast.ModuleStatement:
		fmt.Printf("Module %s\n", stmt.Path)

	case *ast.ImportStatement:
		if stmt.Alias != "" {
			fmt.Printf("Import %s %q\n", stmt.Alias, stmt.Path)
			return
		}
		fmt.Printf("Import %q\n", stmt.Path)

	case *ast.TypeDeclStatement:
		printTypeDecl(stmt)

	case *ast.UnitDeclStatement:
		fmt.Printf("Unit %s %s", stmt.Name.Value, formatTypeRef(stmt.BaseType))
		if stmt.Category != "" {
			fmt.Printf(" %s", stmt.Category)
		}
		fmt.Println()

	case *ast.EnumDeclaration:
		printEnum(stmt)

	case *ast.FunctionDeclaration:
		printFunction(stmt)

	case *ast.StructStatement:
		fmt.Printf("Struct %s\n", stmt.Name.Value)
		printStructFields(stmt.Fields)

	case *ast.LetStatement:
		printLet(stmt)

	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			printLet(let)
		}

	case *ast.AssignmentStatement:
		printAssignment(stmt)

	case *ast.TryAssignmentStatement:
		if stmt.Assignment == nil {
			fmt.Println("TryAssignment")
			return
		}
		fmt.Print("Try ")
		printAssignment(stmt.Assignment)

	case *ast.DeferStatement:
		fmt.Println("Defer")
		if stmt.Body != nil {
			for _, bodyStmt := range stmt.Body.Statements {
				fmt.Print("  ")
				printStatement(bodyStmt)
			}
		}

	case *ast.DiscardStatement:
		value := "<nil>"
		if stmt.Value != nil {
			value = stmt.Value.String()
		}
		fmt.Printf("Discard %s\n", value)

	case *ast.DetachStatement:
		value := "<nil>"
		if stmt.Value != nil {
			value = stmt.Value.String()
		}
		if stmt.DiscardResult {
			fmt.Printf("Detach %s discard\n", value)
			return
		}
		fmt.Printf("Detach %s\n", value)

	case *ast.CancelStatement:
		fmt.Println("Cancel")

	case *ast.ExpressionStatement:
		fmt.Printf("Expression %s\n", stmt.Expression.String())

	case *ast.ReturnStatement:
		printReturn(stmt)

	case *ast.FallthroughStatement:
		fmt.Println("Fallthrough")

	case *ast.BreakStatement:
		fmt.Println("Break")

	case *ast.ContinueStatement:
		fmt.Println("Continue")

	case *ast.ForStatement:
		printFor(stmt)

	case *ast.WhileStatement:
		printWhile(stmt)

	case *ast.SwitchStatement:
		fmt.Println("Switch")

	case *ast.UnsafeStatement:
		fmt.Println("Unsafe")
		if stmt.Body != nil {
			for _, bodyStmt := range stmt.Body.Statements {
				fmt.Print("  ")
				printStatement(bodyStmt)
			}
		}

	case *ast.AsmStatement:
		if stmt.Template == nil {
			fmt.Println("Asm")
			return
		}
		fmt.Printf("Asm %q\n", stmt.Template.Value)

	case *ast.ImplStatement:
		printImpl(stmt)

	case *ast.CommentStatement:
		fmt.Printf("Comment %q\n", stmt.Text)

	case *ast.InvalidStatement:
		fmt.Printf("Invalid %q\n", stmt.TokenLiteral())

	default:
		fmt.Printf("%T %q\n", stmt, stmt.TokenLiteral())
	}
}

func printTypeDecl(stmt *ast.TypeDeclStatement) {
	fmt.Printf("Type %s%s", stmt.Name.Value, formatGenericParameters(stmt.GenericParameters))

	switch {
	case stmt.BaseType != nil:
		fmt.Printf(" %s", formatTypeRef(stmt.BaseType))
	case stmt.AssignedType != nil:
		fmt.Printf(" = %s", formatTypeRef(stmt.AssignedType))
	case len(stmt.Variants) > 0:
		fmt.Print(" =")
		for _, variant := range stmt.Variants {
			fmt.Printf(" %s", variant.Value)
		}
	case stmt.StructType != nil:
		fmt.Print(" struct")
	case stmt.RegisterType != nil:
		fmt.Printf(" register[%s]", formatRegisterWidth(stmt.RegisterType.WidthExpression, stmt.RegisterType.Width))
	}

	if stmt.Contract != nil {
		fmt.Printf(" %s", formatContract(stmt.Contract))
	}
	if stmt.Default != nil {
		fmt.Printf(" default %s", stmt.Default.String())
	}

	fmt.Println()

	if stmt.StructType != nil {
		printStructFields(stmt.StructType.Fields)
	}
	if stmt.RegisterType != nil {
		printRegisterFields(stmt.RegisterType.Fields)
	}
}

func printEnum(stmt *ast.EnumDeclaration) {
	fmt.Printf("Enum %s", stmt.Name.Value)
	if stmt.BitUnderlying {
		fmt.Printf(": bit[%d]", stmt.UnderlyingBitWidth)
	} else if stmt.UnderlyingType != nil {
		fmt.Printf(" %s", formatTypeRef(stmt.UnderlyingType))
	}
	fmt.Println()
	for _, value := range stmt.Values {
		fmt.Printf("  Value %s\n", formatEnumValue(value))
	}
}

func printFunction(stmt *ast.FunctionDeclaration) {
	prefix := "Function"
	if stmt.Extern {
		prefix = "Extern " + stmt.ABI + " Function"
	}
	if stmt.Unsafe {
		prefix = "Unsafe Function"
	}
	if stmt.LinkName != "" {
		fmt.Printf("@link_name(%q)\n", stmt.LinkName)
	}
	fmt.Printf("%s %s%s(%s) %s\n", prefix, stmt.Name.Value, formatGenericParameters(stmt.GenericParameters), formatParameters(stmt.Parameters), formatTypeRef(stmt.ReturnType))
	if stmt.Body == nil {
		return
	}
	for _, bodyStmt := range stmt.Body.Statements {
		fmt.Print("  ")
		printStatement(bodyStmt)
	}
}

func printLet(stmt *ast.LetStatement) {
	if stmt.Address != nil {
		fmt.Printf("@address(%s)\n", stmt.Address.String())
	}
	fmt.Print("Let ")

	if stmt.Mutable {
		fmt.Print("mut ")
	}

	fmt.Print(stmt.Name.Value)

	if stmt.Type != nil {
		fmt.Printf(": %s", formatTypeRef(stmt.Type))
	}
	if stmt.Contract != nil {
		fmt.Printf(" %s", formatContract(stmt.Contract))
	}

	if stmt.Value != nil {
		operator := ":="
		if stmt.Ownership == ast.OwnershipMove {
			operator = ":<-"
			if stmt.Type != nil {
				operator = "<-"
			}
		}
		fmt.Printf(" %s %s", operator, stmt.Value.String())
	}

	fmt.Println()
}

func printAssignment(stmt *ast.AssignmentStatement) {
	fmt.Printf("Assignment %s %s %s\n", stmt.Target.String(), stmt.Operator, stmt.Value.String())
}

func printFor(stmt *ast.ForStatement) {
	if len(stmt.Bindings) == 0 && stmt.Iterable == nil {
		fmt.Println("For")
		return
	}

	fmt.Print("For ")
	for i, binding := range stmt.Bindings {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(binding.Name)
	}
	if stmt.Iterable != nil {
		fmt.Printf(" in %s", stmt.Iterable.String())
	}
	if stmt.Step != nil {
		fmt.Printf(" step %s", stmt.Step.String())
	}
	fmt.Println()
}

func printWhile(stmt *ast.WhileStatement) {
	condition := "<nil>"
	if stmt.Condition != nil {
		condition = stmt.Condition.String()
	}
	fmt.Printf("While %s\n", condition)
}

func printReturn(stmt *ast.ReturnStatement) {
	if stmt.Value == nil {
		fmt.Println("Return")
		return
	}
	fmt.Printf("Return %s\n", stmt.Value.String())
}

func printImpl(stmt *ast.ImplStatement) {
	fmt.Printf("Impl %s\n", formatTypeRef(stmt.Target))
	for _, member := range stmt.Members {
		switch member := member.(type) {
		case *ast.TypeDeclStatement:
			fmt.Print("  ")
			printTypeDecl(member)
		case *ast.UnitDeclStatement:
			fmt.Printf("  Unit %s %s", member.Name.Value, formatTypeRef(member.BaseType))
			if member.Category != "" {
				fmt.Printf(" %s", member.Category)
			}
			fmt.Println()
		case *ast.EnumDeclaration:
			fmt.Printf("  Enum %s", member.Name.Value)
			if member.BitUnderlying {
				fmt.Printf(": bit[%d]", member.UnderlyingBitWidth)
			} else if member.UnderlyingType != nil {
				fmt.Printf(" %s", formatTypeRef(member.UnderlyingType))
			}
			fmt.Println()
			for _, value := range member.Values {
				fmt.Printf("    Value %s\n", formatEnumValue(value))
			}
		case *ast.PropertyDeclaration:
			fmt.Printf("  Property %s: %s\n", member.Name.Value, formatTypeRef(member.Type))
		case *ast.FunctionDeclaration:
			fmt.Print("  ")
			printFunction(member)
		default:
			fmt.Printf("  %T %q\n", member, member.TokenLiteral())
		}
	}
}

func formatEnumValue(value *ast.EnumValue) string {
	out := value.Name.Value
	if value.Initializer != nil {
		out += " = " + value.Initializer.String()
	}
	return out
}

func formatParameters(parameters []*ast.Parameter) string {
	out := ""
	for i, param := range parameters {
		if i > 0 {
			out += ", "
		}
		if param.Ref {
			if param.MutableRef {
				out += "ref mut "
			} else {
				out += "ref "
			}
		}
		out += param.Name.Value
		if param.Type != nil && param.Type.Name == "self" {
			continue
		}
		out += ": "
		if param.Variadic {
			// rules/declarations/functions.md section 28: `...` is parameter
			// shape and must remain visible in the compiler AST presentation.
			out += "..."
		}
		out += formatTypeRef(param.Type)
	}
	return out
}

func formatGenericParameters(parameters []*ast.GenericParameter) string {
	if len(parameters) == 0 {
		return ""
	}
	out := "["
	for i, param := range parameters {
		if i > 0 {
			out += ", "
		}
		if param.Name != nil {
			out += param.Name.Value
		}
		if param.Constraint != nil {
			out += ": " + formatTypeRef(param.Constraint)
		}
	}
	out += "]"
	return out
}

func formatTypeRefs(refs []*ast.TypeReference) string {
	out := ""
	for i, ref := range refs {
		if i > 0 {
			out += ", "
		}
		out += formatTypeRef(ref)
	}
	return out
}

func printStructFields(fields []*ast.StructField) {
	for _, field := range fields {
		fmt.Printf("  Field %s %s\n", field.Name.Value, formatTypeRef(field.Type))
	}
}

func printRegisterFields(fields []*ast.RegisterField) {
	for _, field := range fields {
		if field.Type != nil {
			fmt.Printf("  Field %s %s\n", field.Name.Value, formatTypeRef(field.Type))
			continue
		}
		fmt.Printf("  Field %s bit", field.Name.Value)
		if field.WidthExpression != nil || field.Width != 1 {
			fmt.Printf("[%s]", formatRegisterWidth(field.WidthExpression, field.Width))
		}
		if field.Unit != "" {
			fmt.Printf("<%s>", field.Unit)
		}
		fmt.Println()
	}
}

// formatRegisterWidth prints the preserved source expression introduced by
// rules/declarations/registers.md section 3, falling back to the literal cache
// for older AST producers.
func formatRegisterWidth(expression ast.Expression, width int64) string {
	if expression != nil {
		return expression.String()
	}
	return fmt.Sprintf("%d", width)
}

func formatTypeRef(ref *ast.TypeReference) string {
	if ref == nil {
		return "<nil>"
	}

	refPrefix := ""
	if ref.Ref {
		if ref.MutableRef {
			refPrefix = "ref mut "
		} else {
			refPrefix = "ref "
		}
	}

	if ref.Name == "fn" || ref.FunctionReturnType != nil {
		out := "fn("
		for i, param := range ref.FunctionParameterTypes {
			if i > 0 {
				out += ", "
			}
			out += formatTypeRef(param)
		}
		out += ") " + formatTypeRef(ref.FunctionReturnType)
		return refPrefix + out
	}

	if ref.ElementType != nil {
		element := formatTypeRef(ref.ElementType)
		if ref.Slice {
			return refPrefix + element + "[]"
		}
		if ref.ArrayLengthExpression != nil {
			return refPrefix + element + "[" + ref.ArrayLengthExpression.String() + "]"
		}
		return refPrefix + fmt.Sprintf("%s[%d]", element, ref.ArrayLength)
	}

	out := ref.Name

	if ref.Unit != "" {
		out += "<" + ref.Unit + ">"
	}

	if len(ref.TypeArgs) > 0 {
		out += "["
		for i, arg := range ref.TypeArgs {
			if i > 0 {
				out += ", "
			}
			out += formatTypeRef(arg)
		}
		if ref.EventCapacitySet {
			if len(ref.TypeArgs) > 0 {
				out += ", "
			}
			out += fmt.Sprintf("%d", ref.EventCapacity)
		}
		out += "]"
	}

	return refPrefix + out
}

func formatLambdaCaptures(captures []ast.LambdaCapture) string {
	out := ""
	for i, capture := range captures {
		if i > 0 {
			out += ", "
		}
		if capture.Name != nil {
			out += capture.Name.Value
		}
	}
	return out
}

func formatContract(contract ast.Contract) string {
	switch contract := contract.(type) {
	case *ast.ContractList:
		parts := make([]string, 0, len(contract.Contracts))
		for _, item := range contract.Contracts {
			parts = append(parts, formatContract(item))
		}
		return strings.Join(parts, " ")
	case *ast.RangeContract:
		return "range " + formatRangeContract(contract)
	case *ast.MembershipContract:
		return "in " + formatMembershipContract(contract)
	case *ast.MarkerContract:
		return formatMarkerContract(contract)
	default:
		return fmt.Sprintf("%T", contract)
	}
}

func formatMembershipContract(contract *ast.MembershipContract) string {
	values := make([]string, 0, len(contract.Values))
	for _, value := range contract.Values {
		values = append(values, value.String())
	}
	return "[" + strings.Join(values, ", ") + "]"
}

func formatMarkerContract(contract *ast.MarkerContract) string {
	if contract.Value != nil {
		return contract.Name + " " + contract.Value.String()
	}
	return contract.Name
}

func formatRangeContract(contract *ast.RangeContract) string {
	operator := ".."
	if contract.Exclusive {
		operator = "..<"
	}

	return formatRangeBound(contract.Min) + operator + formatRangeBound(contract.Max)
}

func formatRangeBound(expr ast.Expression) string {
	if expr == nil {
		return ""
	}

	if prefix, ok := expr.(*ast.PrefixExpression); ok {
		if prefix.Operator == "-" && prefix.Right != nil {
			return "-" + formatExpression(prefix.Right)
		}
	}

	return formatExpression(expr)
}

func formatExpression(expr ast.Expression) string {
	if expr == nil {
		return "<nil>"
	}

	return expr.String()
}
