package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"sec/internal/ast"
	llvmcodegen "sec/internal/codegen/llvm"
	"sec/internal/lexer"
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

	if command == "init" {
		runInitCommand(flag.Args()[1:])
		return
	}

	if command == "emit-llvm" {
		runEmitLLVMCommand(flag.Args()[1:])
		return
	}

	if command == "build" {
		runBuildCommand(flag.Args()[1:])
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
		runSema(string(data))

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "usage: sec <lex|token> <file.sec>")
	fmt.Fprintln(os.Stderr, "       sec init [path] [--name <name>] [--target <os-arch>] [--profile <profile>]")
	fmt.Fprintln(os.Stderr, "       sec <parse|ast|sema> <file.sec|dir|glob>...")
	fmt.Fprintln(os.Stderr, "       sec emit-llvm <file.sec> -o <file.ll> [--target <os-arch>]")
	fmt.Fprintln(os.Stderr, "       sec build <file.sec> [-o <program>] [--target <os-arch>] [--keep-llvm] [--clang <path>]")
}

func runLex(input string) {
	l := lexer.New(input)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "LEXEME\tLINE\tCOLUMN")

	exitCode := 0

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
			exitCode = 2
		}

		if tok.Type == lexer.EOF {
			break
		}
	}

	_ = w.Flush()
	os.Exit(exitCode)
}

func runTokens(input string) {
	l := lexer.New(input)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TYPE\tLEXEME\tLINE\tCOLUMN")

	exitCode := 0

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
			exitCode = 2
		}

		if tok.Type == lexer.EOF {
			break
		}
	}

	_ = w.Flush()
	os.Exit(exitCode)
}

func runParse(input string) {
	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()

	printParserWarnings(p)

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		}
		os.Exit(2)
	}

	printProgram(program)
}

func runParseInputs(inputs []string) {
	program := parseSourceInputs(inputs, CompilerTarget{}, false)
	printProgram(program)
}

func runAST(input string) {
	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()

	printParserWarnings(p)

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		}
		os.Exit(2)
	}

	printAST(program)
}

func runASTInputs(inputs []string) {
	program := parseSourceInputs(inputs, CompilerTarget{}, false)
	printAST(program)
}

func runSema(input string) {
	parseAndAnalyze(input)
	fmt.Println("OK")
}

func runSemaInputs(inputs []string, target CompilerTarget) {
	program := parseSourceInputs(inputs, target, true)
	analyzeProgram(program, target)
	fmt.Println("OK")
}

func runEmitLLVMCommand(args []string) {
	inputFile, outputFile, target, ok := parseEmitLLVMCommandArgs(args, hostCompilerTarget())
	if !ok {
		printUsage()
		os.Exit(1)
	}

	input, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}

	targetDefinition, err := requireTargetCanEmitLLVM(target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "target error: %s\n", err)
		os.Exit(1)
	}

	program := parseAndAnalyzeForTarget(string(input), target)
	ir, err := llvmcodegen.GenerateWithTriple(program, targetDefinition.LLVMTriple)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen error: %v\n", err)
		os.Exit(4)
	}

	if err := os.WriteFile(outputFile, []byte(ir), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
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

	program := parseAndAnalyzeForTarget(string(input), options.Target)
	ir, err := llvmcodegen.GenerateWithTriple(program, targetDefinition.LLVMTriple)
	if err != nil {
		fmt.Fprintf(os.Stderr, "codegen error: %v\n", err)
		os.Exit(4)
	}

	llvmPath := options.LLVMOutputFile
	removeLLVM := false
	if llvmPath == "" {
		tmp, err := os.CreateTemp("", "sec-*.ll")
		if err != nil {
			fmt.Fprintf(os.Stderr, "temp file error: %v\n", err)
			os.Exit(1)
		}
		llvmPath = tmp.Name()
		removeLLVM = true
		if err := tmp.Close(); err != nil {
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

func parseOutputCommandArgs(args []string) (inputFile string, outputFile string, ok bool) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 >= len(args) || outputFile != "" {
				return "", "", false
			}
			outputFile = args[i+1]
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

type buildCommandOptions struct {
	InputFile      string
	OutputFile     string
	Target         CompilerTarget
	Clang          string
	LLVMOutputFile string
}

func parseBuildCommandArgs(args []string) (inputFile string, outputFile string, ok bool) {
	options, ok := parseBuildCommandOptions(args, CompilerTarget{})
	return options.InputFile, options.OutputFile, ok
}

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
		case "--keep-llvm":
			if options.LLVMOutputFile != "" {
				return buildCommandOptions{}, false
			}
			options.LLVMOutputFile = "__sec_keep_llvm__"
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

	return options, true
}

func defaultBuildOutputPath(inputFile string) string {
	ext := filepath.Ext(inputFile)
	if ext == "" {
		return inputFile
	}
	return strings.TrimSuffix(inputFile, ext)
}

func parseAndAnalyze(input string) *ast.Program {
	return parseAndAnalyzeForTarget(input, hostCompilerTarget())
}

func parseAndAnalyzeForTarget(input string, target CompilerTarget) *ast.Program {
	l := lexer.New(input)
	p := parser.New(l)

	program := p.ParseProgram()

	printParserWarnings(p)

	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		}
		os.Exit(2)
	}

	analyzeProgram(program, target)

	return program
}

func analyzeProgram(program *ast.Program, target CompilerTarget) {
	if err := validateProgramTarget(program, target); err != nil {
		fmt.Fprintf(os.Stderr, "target error: %s\n", err)
		os.Exit(1)
	}

	resolveStdlibImports(program, target)

	analyzer := sema.NewAnalyzer()
	errors := analyzer.Analyze(program)
	printSemaWarnings(analyzer)
	if len(errors) > 0 {
		for _, err := range errors {
			fmt.Fprintf(os.Stderr, "sema error: %s\n", err)
		}
		os.Exit(3)
	}
}

func parseSourceInputs(inputs []string, target CompilerTarget, filterTarget bool) *ast.Program {
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
	included := 0
	for _, file := range files {
		if filterTarget && !sourcePathMatchesTarget(file, target) {
			continue
		}
		program := parseSourceFile(file)
		if filterTarget {
			if err := validateProgramTarget(program, target); err != nil {
				continue
			}
		}
		combined.Statements = append(combined.Statements, program.Statements...)
		included++
	}
	if included == 0 {
		fmt.Fprintf(os.Stderr, "source error: no .sec files match target %s\n", target.String())
		os.Exit(1)
	}
	return combined
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
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}

	l := lexer.New(string(data))
	p := parser.New(l)
	program := p.ParseProgram()
	printParserWarnings(p)
	if len(p.Errors()) > 0 {
		for _, err := range p.Errors() {
			fmt.Fprintf(os.Stderr, "%s: parse error: %s\n", path, err)
		}
		os.Exit(2)
	}
	return program
}

func printParserWarnings(p *parser.Parser) {
	for _, warning := range p.Warnings() {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
	}
}

func printSemaWarnings(analyzer *sema.Analyzer) {
	for _, warning := range analyzer.Warnings() {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
	}
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
	// TODO: Replace this source-level stdlib inclusion with compiled library metadata/artifacts.
	seen := map[string]bool{}
	resolveStdlibImportsInto(program, target, seen)
}

func resolveStdlibImportsInto(program *ast.Program, target CompilerTarget, seen map[string]bool) {
	for _, stmt := range append([]ast.Statement{}, program.Statements...) {
		importStmt, ok := stmt.(*ast.ImportStatement)
		if !ok {
			continue
		}
		sourcePath, ok := sourceIncludePath(importStmt.Path, target)
		if !ok {
			continue
		}
		if seen[sourcePath] {
			continue
		}
		seen[sourcePath] = true

		imported := parseSourceInclude(sourcePath, target)
		resolveStdlibImportsInto(imported, target, seen)
		module := programModulePath(imported)
		qualifyImportedModule(imported, module)
		program.Statements = append(program.Statements, imported.Statements...)
	}
}

func canSourceIncludeModule(name string) bool {
	switch name {
	case "fmt", "io":
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

func parseSourceInclude(path string, target CompilerTarget) *ast.Program {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "source import error: %v\n", err)
		os.Exit(1)
	}

	l := lexer.New(string(data))
	p := parser.New(l)
	program := p.ParseProgram()
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
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if ok && fn.Name != nil && !strings.Contains(fn.Name.Value, ".") {
			localFunctions[fn.Name.Value] = true
		}
	}

	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok || fn.Name == nil {
			continue
		}
		if strings.Contains(fn.Name.Value, ".") {
			continue
		}
		qualifyLocalCalls(fn.Body, module, localFunctions)
		fn.Name.Value = module + "." + fn.Name.Value
		fn.Name.Token.Lexeme = fn.Name.Value
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
			qualifyLocalCallsInExpression(arm.Pattern, module, localFunctions)
			qualifyLocalCallsInExpression(arm.Guard, module, localFunctions)
			qualifyLocalCallsInExpression(arm.Body, module, localFunctions)
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
		name := "<nil>"
		if stmt.Name != nil {
			name = stmt.Name.Value
		}
		printASTLeaf(childPrefix(prefix, last), true, "Name: "+name)

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

	if stmt.Type != nil {
		children = append(children, "Type: "+formatTypeRef(stmt.Type))
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

	if len(stmt.Variants) > 0 {
		children = append(children, "Variants: "+formatVariants(stmt.Variants))
	}

	if stmt.Contract != nil {
		children = append(children, formatASTContract(stmt.Contract))
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
		printASTBranch(childrenPrefix, true, fmt.Sprintf("Register[%d]", stmt.RegisterType.Width))
		registerPrefix := childPrefix(childrenPrefix, true)
		for i, field := range stmt.RegisterType.Fields {
			printASTRegisterField(registerPrefix, field, i == len(stmt.RegisterType.Fields)-1)
		}
	}
}

func printASTEnum(stmt *ast.EnumDeclaration, prefix string, last bool) {
	printASTBranch(prefix, last, "Enum")

	children := []string{"Name: " + stmt.Name.Value}
	if stmt.UnderlyingType != nil {
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

func printASTFunction(stmt *ast.FunctionDeclaration, prefix string, last bool) {
	printASTBranch(prefix, last, "Function")
	childrenPrefix := childPrefix(prefix, last)
	printASTLeaf(childrenPrefix, false, fmt.Sprintf("Unsafe: %t", stmt.Unsafe))
	printASTLeaf(childrenPrefix, false, fmt.Sprintf("Extern: %t", stmt.Extern))
	if stmt.Extern {
		printASTLeaf(childrenPrefix, false, "ABI: "+stmt.ABI)
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
	children := []string{
		"Name: " + field.Name.Value,
		fmt.Sprintf("Type: bit[%d]", field.Width),
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
	printASTExpression(childrenPrefix, false, "Pattern", arm.Pattern)
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
	case *ast.RangeContract:
		return "Range: " + formatRangeContract(contract)
	default:
		return fmt.Sprintf("Contract: %T", contract)
	}
}

func formatASTExpression(expr ast.Expression) string {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return "Identifier(" + expr.Value + ")"
	case *ast.IntegerLiteral:
		return fmt.Sprintf("Int(%d)", expr.Value)
	case *ast.FloatLiteral:
		return fmt.Sprintf("Float(%g)", expr.Value)
	case *ast.StringLiteral:
		return fmt.Sprintf("String(%q)", expr.Value)
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
		name := "<nil>"
		if stmt.Name != nil {
			name = stmt.Name.Value
		}
		fmt.Printf("Discard %s\n", name)

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
		fmt.Printf(" register[%d]", stmt.RegisterType.Width)
	}

	if stmt.Contract != nil {
		fmt.Printf(" %s", formatContract(stmt.Contract))
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
	if stmt.UnderlyingType != nil {
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

	if stmt.Value != nil {
		fmt.Printf(" := %s", stmt.Value.String())
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
			if member.UnderlyingType != nil {
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
		out += param.Name.Value + ": " + formatTypeRef(param.Type)
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

func printStructFields(fields []*ast.StructField) {
	for _, field := range fields {
		fmt.Printf("  Field %s %s\n", field.Name.Value, formatTypeRef(field.Type))
	}
}

func printRegisterFields(fields []*ast.RegisterField) {
	for _, field := range fields {
		fmt.Printf("  Field %s bit", field.Name.Value)
		if field.Width != 1 {
			fmt.Printf("[%d]", field.Width)
		}
		if field.Unit != "" {
			fmt.Printf("<%s>", field.Unit)
		}
		fmt.Println()
	}
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
		if ref.ArrayLength > 0 {
			return refPrefix + fmt.Sprintf("[%d]%s", ref.ArrayLength, formatTypeRef(ref.ElementType))
		}
		return refPrefix + "[]" + formatTypeRef(ref.ElementType)
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
	case *ast.RangeContract:
		return "range " + formatRangeContract(contract)
	default:
		return fmt.Sprintf("%T", contract)
	}
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
