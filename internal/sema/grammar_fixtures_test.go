package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// grammarFixtureDirectory holds the canonical grammar fixture suite.
const grammarFixtureDirectory = "../../testdata/grammar"

// requiredGrammarFixtures is the suite grammar.md "Required grammar fixtures"
// names.
var requiredGrammarFixtures = []string{
	"grammar_valid.sec",
	"grammar_invalid.sec",
	"grammar_declarations_valid.sec",
	"grammar_declarations_invalid.sec",
	"grammar_types_valid.sec",
	"grammar_types_invalid.sec",
	"grammar_statements_valid.sec",
	"grammar_statements_invalid.sec",
	"grammar_expressions_valid.sec",
	"grammar_expressions_invalid.sec",
	"grammar_contextual_valid.sec",
	"grammar_contextual_invalid.sec",
	"grammar_recovery.sec",
}

var expectedErrorBlock = regexp.MustCompile(`(?s)/\* Expected error: (.*?)\n \* Reason: (.*?)\*/`)

type fixtureDiagnostic struct {
	line    int
	message string
}

type expectedFixtureError struct {
	text      string
	startLine int // first line after the block
	endLine   int // last line before the next block or end of file
}

// TestGrammarFixtureSuite runs the canonical grammar fixtures. A valid fixture
// parses and analyzes without diagnostics. In an invalid fixture or the
// recovery fixture every `/* Expected error: ... * Reason: ... */` block must
// be matched by a parser or semantic diagnostic containing its text between
// that block and the next one, and no diagnostic may fall outside a block's
// region, so cascades and silently accepted invalid forms both fail. The
// recovery fixture must also retain every top-level function after the
// errors.
//
// Rules:
//   - rules/foundations/grammar.md — "Required grammar fixtures", "Invalid forms", "Legacy, future, and recovery syntax"
//   - rules/compiler/parser_recovery.md — recovery continues and retains later declarations
func TestGrammarFixtureSuite(t *testing.T) {
	for _, name := range requiredGrammarFixtures {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(grammarFixtureDirectory, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("required grammar fixture is missing: %v", err)
			}
			source := string(data)
			program, diagnostics := analyzeGrammarFixture(t, name, source)
			expected := expectedFixtureErrors(source)
			if strings.HasSuffix(name, "_valid.sec") {
				if len(expected) != 0 {
					t.Fatalf("a valid fixture must not contain Expected error blocks")
				}
				for _, diagnostic := range diagnostics {
					t.Errorf("valid fixture %s:%d: %s", name, diagnostic.line, diagnostic.message)
				}
				return
			}
			if len(expected) == 0 {
				t.Fatalf("an invalid fixture must contain Expected error blocks")
			}
			checkExpectedFixtureErrors(t, name, expected, diagnostics)
			if name == "grammar_recovery.sec" {
				checkRecoveredFunctions(t, source, program)
			}
		})
	}
}

// analyzeGrammarFixture parses and analyzes one fixture, returning parser
// and semantic diagnostics with their lines.
func analyzeGrammarFixture(t *testing.T, name string, source string) (program *ast.Program, diagnostics []fixtureDiagnostic) {
	t.Helper()
	result := parser.New(lexer.NewWithFile(source, name)).Parse()
	for _, diagnostic := range result.Diagnostics {
		diagnostics = append(diagnostics, fixtureDiagnostic{line: diagnostic.Primary.Line, message: diagnostic.Message})
	}
	program = result.Program
	if program == nil {
		return program, diagnostics
	}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("semantic analysis panicked on %s: %v", name, recovered)
			}
		}()
		for _, err := range NewAnalyzer().Analyze(program) {
			diagnostics = append(diagnostics, fixtureDiagnostic{line: err.Line, message: err.Message})
		}
	}()
	sort.SliceStable(diagnostics, func(i, j int) bool { return diagnostics[i].line < diagnostics[j].line })
	return program, diagnostics
}

// expectedFixtureErrors reads the Expected error blocks and the source region
// each one governs.
func expectedFixtureErrors(source string) []expectedFixtureError {
	matches := expectedErrorBlock.FindAllStringSubmatchIndex(source, -1)
	lines := strings.Count(source, "\n") + 1
	expected := make([]expectedFixtureError, 0, len(matches))
	for index, match := range matches {
		end := lines
		if index+1 < len(matches) {
			end = strings.Count(source[:matches[index+1][0]], "\n")
		}
		expected = append(expected, expectedFixtureError{
			text:      strings.TrimSpace(source[match[2]:match[3]]),
			startLine: strings.Count(source[:match[1]], "\n") + 1,
			endLine:   end,
		})
	}
	return expected
}

func checkExpectedFixtureErrors(t *testing.T, name string, expected []expectedFixtureError, diagnostics []fixtureDiagnostic) {
	t.Helper()
	for _, block := range expected {
		want := strings.ToLower(strings.TrimSuffix(block.text, "."))
		found := false
		for _, diagnostic := range diagnostics {
			if diagnostic.line >= block.startLine && diagnostic.line <= block.endLine && strings.Contains(strings.ToLower(diagnostic.message), want) {
				found = true
				break
			}
		}
		if !found {
			reported := []string{}
			for _, diagnostic := range diagnostics {
				if diagnostic.line >= block.startLine && diagnostic.line <= block.endLine {
					reported = append(reported, fmt.Sprintf("%d: %s", diagnostic.line, diagnostic.message))
				}
			}
			t.Errorf("%s:%d: expected error %q was not reported in lines %d-%d; reported there: %q", name, block.startLine, block.text, block.startLine, block.endLine, reported)
		}
	}
	for _, diagnostic := range diagnostics {
		inside := false
		for _, block := range expected {
			if diagnostic.line >= block.startLine && diagnostic.line <= block.endLine {
				inside = true
				break
			}
		}
		if !inside {
			t.Errorf("%s:%d: unexpected diagnostic outside every Expected error region: %s", name, diagnostic.line, diagnostic.message)
		}
	}
}

var topLevelFunction = regexp.MustCompile(`(?m)^fn ([A-Za-z_][A-Za-z0-9_]*)`)

// checkRecoveredFunctions requires every top-level function of the source to
// survive parser recovery as a declaration.
func checkRecoveredFunctions(t *testing.T, source string, program *ast.Program) {
	t.Helper()
	retained := map[string]bool{}
	if program != nil {
		for _, statement := range program.Statements {
			if function, ok := statement.(*ast.FunctionDeclaration); ok && function != nil && function.Name != nil {
				retained[function.Name.Value] = true
			}
		}
	}
	for _, match := range topLevelFunction.FindAllStringSubmatch(source, -1) {
		if !retained[match[1]] {
			t.Errorf("recovery lost top-level function %s", match[1])
		}
	}
}
