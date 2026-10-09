package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/sema"
)

// testCommand is `sec test [--list] [--run <pattern>] [--json]
// [--target <os-arch>] [<file_test.sec|dir|glob>...]`.
type testCommand struct {
	inputs []string
	run    *regexp.Regexp
	list   bool
	json   bool
	target CompilerTarget
}

// testStatus keeps completed outcomes (§3.3) apart from selection and
// infrastructure states, which are never reported as Skipped (§3.3, §30.4).
type testStatus string

const (
	testPassed               testStatus = "passed"
	testFailed               testStatus = "failed"
	testSkipped              testStatus = "skipped"
	testCompilationFailed    testStatus = "compilation-failed"
	testExecutionUnavailable testStatus = "execution-unavailable"
	testListed               testStatus = "listed"
)

// executionUnavailableReason explains why a compiled test is not executed:
// the static harness and the lowering of test bodies and test-boundary
// outcomes (testing.md §§32–33) do not exist yet.
const executionUnavailableReason = "test execution is not available yet: the static test harness and lowering of test bodies are not implemented"

// testResult is the structured record of one selected top-level test
// (testing.md §31.2, §31.5).
type testResult struct {
	Module   string     `json:"module"`
	Name     string     `json:"name"`
	Identity []string   `json:"identity"`
	File     string     `json:"file"`
	Line     int        `json:"line"`
	Column   int        `json:"column"`
	Status   testStatus `json:"status"`
	Message  string     `json:"message,omitempty"`
}

// testModuleResult is one test compilation: the selected *_test.sec files of
// one directory and module together with their production sources.
type testModuleResult struct {
	Module      string       `json:"module"`
	Directory   string       `json:"directory"`
	Files       []string     `json:"files"`
	Compiled    bool         `json:"compiled"`
	Diagnostics int          `json:"diagnostics"`
	Tests       []testResult `json:"tests"`
}

type testRunSummary struct {
	Selected             int `json:"selected"`
	Passed               int `json:"passed"`
	Failed               int `json:"failed"`
	Skipped              int `json:"skipped"`
	CompilationFailed    int `json:"compilationFailedModules"`
	ExecutionUnavailable int `json:"executionUnavailable"`
}

type testRunReport struct {
	Modules []testModuleResult `json:"modules"`
	Summary testRunSummary     `json:"summary"`
	// Filter is the --run pattern; FilteredOut counts discovered tests it
	// did not select, so an empty selection is reported precisely (§29.4).
	Filter      string `json:"filter,omitempty"`
	FilteredOut int    `json:"filteredOut,omitempty"`
}

// Exit statuses of sec test. A run that cannot be completed is unsuccessful
// (§30.3) and each failure category keeps its own status (§30.4).
const (
	testExitSuccess              = 0
	testExitTestFailure          = 1
	testExitCompilationFailure   = 3
	testExitExecutionUnavailable = 4
)

// runTestCommand discovers, compiles, selects and reports source tests.
//
// Rules:
//   - rules/tooling/testing.md — §4 "Test source files", §26 "TestCompilationPlan"
//   - rules/tooling/testing.md — §27 "Test discovery", §28 "sec test", §29 "Runner behavior"
//   - rules/tooling/testing.md — §30 "Exit status and aggregate result", §31 "Test result reporting"
func runTestCommand(args []string, out io.Writer) int {
	command, err := parseTestCommandArgs(args, hostCompilerTarget())
	if err != nil {
		reportToolError("usage", "%v", err)
		return 1
	}
	groups, err := selectTestSourceGroups(command.inputs)
	if err != nil {
		reportToolError("test", "%v", err)
		return 1
	}
	report := testRunReport{}
	if command.run != nil {
		report.Filter = command.run.String()
	}
	for _, group := range groups {
		report.Modules = append(report.Modules, compileTestGroup(group, command, &report))
	}
	report.summarize()
	if command.json {
		encoder := json.NewEncoder(out)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			reportToolError("test", "%v", err)
			return 1
		}
	} else {
		report.writeHuman(out, command.list)
	}
	return report.exitStatus(command.list)
}

func parseTestCommandArgs(args []string, host CompilerTarget) (testCommand, error) {
	command := testCommand{target: host}
	for index := 0; index < len(args); index++ {
		switch arg := args[index]; {
		case arg == "--list":
			command.list = true
		case arg == "--json":
			command.json = true
		case arg == "--run" || arg == "--target":
			if index+1 >= len(args) {
				return testCommand{}, fmt.Errorf("%s requires a value", arg)
			}
			index++
			if arg == "--target" {
				target, ok := parseCompilerTarget(args[index])
				if !ok {
					return testCommand{}, fmt.Errorf("invalid target %q", args[index])
				}
				command.target = target
				continue
			}
			pattern, err := regexp.Compile(args[index])
			if err != nil {
				return testCommand{}, fmt.Errorf("invalid --run pattern: %v", err)
			}
			command.run = pattern
		case strings.HasPrefix(arg, "-"):
			return testCommand{}, fmt.Errorf("unknown test option %s", arg)
		default:
			command.inputs = append(command.inputs, arg)
		}
	}
	return command, nil
}

// testSourceGroup is the set of selected test files of one directory.
type testSourceGroup struct {
	dir   string
	files []string
}

// selectTestSourceGroups selects *_test.sec files. Explicit files must be
// test files; directories and globs are searched recursively. Without
// inputs the active project (.sec/sec.toml) is the default scope (§28.2);
// tests of imported standard-library modules are never added implicitly.
func selectTestSourceGroups(inputs []string) ([]testSourceGroup, error) {
	if len(inputs) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		root := findProjectRoot(cwd)
		if _, err := os.Stat(filepath.Join(root, ".sec", "sec.toml")); err != nil {
			return nil, fmt.Errorf("no project manifest (.sec/sec.toml) found; pass test files or directories")
		}
		inputs = []string{root}
	}
	byDir := map[string][]string{}
	seen := map[string]bool{}
	add := func(path string) {
		clean := filepath.Clean(path)
		if !seen[clean] {
			seen[clean] = true
			byDir[filepath.Dir(clean)] = append(byDir[filepath.Dir(clean)], clean)
		}
	}
	for _, input := range inputs {
		matches := []string{input}
		if strings.ContainsAny(input, "*?[") {
			globbed, err := filepath.Glob(input)
			if err != nil {
				return nil, err
			}
			matches = globbed
		}
		if len(matches) == 0 {
			return nil, fmt.Errorf("%s: no matches", input)
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				return nil, err
			}
			if !info.IsDir() {
				if !strings.HasSuffix(match, "_test.sec") {
					return nil, fmt.Errorf("%s is not a test file; test files are named *_test.sec", match)
				}
				add(match)
				continue
			}
			err = filepath.WalkDir(match, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if path != match && strings.HasPrefix(entry.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(path, "_test.sec") {
					add(path)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}
	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	groups := make([]testSourceGroup, 0, len(dirs))
	for _, dir := range dirs {
		files := byDir[dir]
		sort.Strings(files)
		groups = append(groups, testSourceGroup{dir: dir, files: files})
	}
	return groups, nil
}

// compileTestGroup builds one test compilation: the selected test files plus
// the production files of their directory module (white-box tests, §7), then
// discovers the top-level tests declared in the selected files (§27.2).
func compileTestGroup(group testSourceGroup, command testCommand, report *testRunReport) testModuleResult {
	result := testModuleResult{Directory: group.dir, Files: group.files, Tests: []testResult{}}
	program := &ast.Program{}
	for _, file := range group.files {
		parsed, counts := parseSourceFileWithDiagnostics(file)
		printParserWarningsForFile(file, parsed.Warnings)
		if result.Module == "" && parsed.Program != nil {
			result.Module = programModulePath(parsed.Program)
		}
		if counts.Errors > 0 {
			cliDiagnostics.parserDiagnostics(file, parsed.Diagnostics, parsed.Errors)
			result.Diagnostics += counts.Errors
			continue
		}
		program.Statements = append(program.Statements, parsed.Program.Statements...)
	}
	if result.Diagnostics > 0 {
		return result
	}
	analyzer, diagnostics := analyzeTestProgram(program, command.target, group.files)
	result.Diagnostics += diagnostics
	if analyzer == nil || diagnostics > 0 {
		return result
	}
	result.Compiled = true
	selected := map[string]bool{}
	for _, file := range group.files {
		selected[sameSourceFileKey(file)] = true
	}
	for _, metadata := range analyzer.ResolvedTests() {
		if !selected[sameSourceFileKey(metadata.Source.File)] {
			continue
		}
		if command.run != nil && !command.run.MatchString(metadata.Name) {
			report.FilteredOut++
			continue
		}
		status, message := testExecutionUnavailable, executionUnavailableReason
		if command.list {
			status, message = testListed, ""
		}
		result.Tests = append(result.Tests, testResult{
			Module: metadata.Identity.Module, Name: metadata.Name,
			Identity: append([]string(nil), metadata.Identity.Path...),
			File:     metadata.Source.File, Line: metadata.Source.Line, Column: metadata.Source.Column,
			Status: status, Message: message,
		})
	}
	sort.SliceStable(result.Tests, func(i, j int) bool {
		left, right := result.Tests[i], result.Tests[j]
		if left.File != right.File {
			return left.File < right.File
		}
		return left.Line < right.Line
	})
	return result
}

// analyzeTestProgram runs the shared CLI compilation steps without exiting,
// so one failing test module does not hide the results of the others
// (§29.1). It returns nil and a nonzero count when compilation fails.
func analyzeTestProgram(program *ast.Program, target CompilerTarget, sourceFiles []string) (*sema.Analyzer, int) {
	siblings := assembleCLIModuleSources(program, target)
	if siblings.Errors > 0 {
		return nil, siblings.Errors
	}
	if err := validateProgramTarget(program, target); err != nil {
		reportToolError("target", "%s", err)
		return nil, 1
	}
	resolveCoreLibraryWithSources(program, sourceFiles)
	resolveStdlibImportsWithSources(program, target, sourceFiles)
	definition, ok := findTargetDefinition(target)
	if !ok {
		reportToolError("target", "unsupported target %s", target.String())
		return nil, 1
	}
	plan, err := definition.scalarPlan()
	if err != nil {
		reportToolError("target", "%s", err)
		return nil, 1
	}
	analyzer := sema.NewAnalyzerWithScalarPlanAndDepth(plan, sema.AnalysisStandard)
	errors := analyzer.Analyze(program)
	printSemaWarnings(analyzer)
	for _, err := range errors {
		printSemaError(os.Stderr, err)
	}
	return analyzer, len(errors)
}

func sameSourceFileKey(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		return absolute
	}
	return filepath.Clean(path)
}

func (r *testRunReport) summarize() {
	for _, module := range r.Modules {
		if !module.Compiled {
			r.Summary.CompilationFailed++
		}
		for _, test := range module.Tests {
			r.Summary.Selected++
			switch test.Status {
			case testPassed:
				r.Summary.Passed++
			case testFailed:
				r.Summary.Failed++
			case testSkipped:
				r.Summary.Skipped++
			case testExecutionUnavailable:
				r.Summary.ExecutionUnavailable++
			}
		}
	}
}

func (r testRunReport) exitStatus(list bool) int {
	switch {
	case r.Summary.CompilationFailed > 0:
		return testExitCompilationFailure
	case r.Summary.Failed > 0:
		return testExitTestFailure
	case !list && r.Summary.ExecutionUnavailable > 0:
		return testExitExecutionUnavailable
	}
	return testExitSuccess
}

// writeHuman prints one line per test in deterministic order (§29.3) and a
// summary that keeps every category visible (§30.2, §30.4).
func (r testRunReport) writeHuman(out io.Writer, list bool) {
	for _, module := range r.Modules {
		name := module.Module
		if name == "" {
			name = module.Directory
		}
		if !module.Compiled {
			fmt.Fprintf(out, "COMPILE FAIL  %s (%s): %d diagnostics\n", name, module.Directory, module.Diagnostics)
			continue
		}
		for _, test := range module.Tests {
			label := map[testStatus]string{
				testPassed: "PASS", testFailed: "FAIL", testSkipped: "SKIP",
				testExecutionUnavailable: "UNAVAILABLE", testListed: "TEST",
			}[test.Status]
			fmt.Fprintf(out, "%-12s  %s %q  %s:%d:%d\n", label, test.Module, test.Name, test.File, test.Line, test.Column)
		}
	}
	switch {
	case r.Summary.Selected == 0 && r.Filter != "" && r.FilteredOut > 0:
		fmt.Fprintf(out, "no tests match --run %q (%d discovered)\n", r.Filter, r.FilteredOut)
	case r.Summary.Selected == 0 && r.Summary.CompilationFailed == 0:
		fmt.Fprintln(out, "no tests found")
	}
	if list {
		fmt.Fprintf(out, "%d tests in %d modules", r.Summary.Selected, len(r.Modules))
		if r.Summary.CompilationFailed > 0 {
			fmt.Fprintf(out, "; %d modules failed to compile", r.Summary.CompilationFailed)
		}
		fmt.Fprintln(out)
		return
	}
	fmt.Fprintf(out, "%d selected: %d passed, %d failed, %d skipped, %d execution unavailable; %d modules failed to compile\n",
		r.Summary.Selected, r.Summary.Passed, r.Summary.Failed, r.Summary.Skipped, r.Summary.ExecutionUnavailable, r.Summary.CompilationFailed)
	if r.Summary.ExecutionUnavailable > 0 {
		fmt.Fprintln(out, "note: "+executionUnavailableReason+"; use --list to only discover and compile tests")
	}
}
