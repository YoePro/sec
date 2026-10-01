package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"
	"testing"
	"time"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Recovery robustness harness for the canonical parser-recovery invariants.
// Every generated input must terminate, avoid panics, keep diagnostics
// bounded, recover deterministically, and leave a structurally valid partial
// AST.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Recovery invariants": "Progress", "Determinism", "Bounded damage"
//   - rules/compiler/parser_recovery.md — "Diagnostic limit"
//   - rules/compiler/parser_recovery.md — "Tests": "Progress tests", "Truncation tests", "Mutation tests", "Fuzz tests"
//   - rules/compiler/parser_recovery.md — "A.19 Add fuzz and mutation tests"

const robustnessParseTimeout = 10 * time.Second

type robustnessOutcome struct {
	result    ParseResult
	panicked  any
	stack     string
	tokens    int
	lexerFail string
}

// robustParse lexes and parses source under a watchdog so a progress violation
// is reported as a failing input instead of hanging the test binary.
func robustParse(source string) (robustnessOutcome, bool) {
	done := make(chan robustnessOutcome, 1)
	go func() {
		var outcome robustnessOutcome
		defer func() {
			if recovered := recover(); recovered != nil {
				outcome.panicked = recovered
				outcome.stack = string(debug.Stack())
			}
			done <- outcome
		}()
		outcome.tokens, outcome.lexerFail = checkLexerStream(source)
		outcome.result = New(lexer.NewWithFile(source, "robustness.sec")).Parse()
	}()
	select {
	case outcome := <-done:
		return outcome, true
	case <-time.After(robustnessParseTimeout):
		return robustnessOutcome{}, false
	}
}

// checkLexerStream verifies lexer progress and token-range sanity: the stream
// reaches EOF within a bound proportional to the input, and byte ranges stay
// ordered and inside the source.
func checkLexerStream(source string) (int, string) {
	l := lexer.NewWithFile(source, "robustness.sec")
	limit := 4*len(source) + 16
	previousStart := 0
	for count := 0; count <= limit; count++ {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			return count, ""
		}
		if token.ByteStart < previousStart {
			return count, fmt.Sprintf("token %q starts at byte %d before previous token start %d", token.Lexeme, token.ByteStart, previousStart)
		}
		if token.ByteEnd < token.ByteStart || token.ByteEnd > len(source) {
			return count, fmt.Sprintf("token %q has byte range [%d,%d) outside source length %d", token.Lexeme, token.ByteStart, token.ByteEnd, len(source))
		}
		previousStart = token.ByteStart
	}
	return limit, fmt.Sprintf("lexer did not reach EOF within %d tokens", limit)
}

// checkRecoveryInvariants reports the first violated parser-recovery
// invariant for source, or the empty string when every invariant holds.
func checkRecoveryInvariants(source string) string {
	outcome, finished := robustParse(source)
	if !finished {
		return fmt.Sprintf("parser did not terminate within %s", robustnessParseTimeout)
	}
	if outcome.panicked != nil {
		return fmt.Sprintf("panic: %v\n%s", outcome.panicked, outcome.stack)
	}
	if outcome.lexerFail != "" {
		return "lexer: " + outcome.lexerFail
	}
	result := outcome.result
	if result.Program == nil {
		return "Parse returned a nil Program"
	}
	if limit := maxParserDiagnostics + outcome.tokens + 1; len(result.Diagnostics) > limit {
		return fmt.Sprintf("diagnostics are unbounded: %d > %d", len(result.Diagnostics), limit)
	}
	parserDiagnostics := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.ID == "" {
			return fmt.Sprintf("diagnostic without ID: %q", diagnostic.Message)
		}
		if !strings.HasPrefix(diagnostic.ID, "L") {
			parserDiagnostics++
		}
	}
	if parserDiagnostics > maxParserDiagnostics {
		return fmt.Sprintf("parser diagnostics exceed limit: %d > %d", parserDiagnostics, maxParserDiagnostics)
	}
	if len(result.Diagnostics) > 0 && !result.HasErrors {
		return "diagnostics were reported but HasErrors is false"
	}
	if problem := findInvalidASTShape(result.Program); problem != "" {
		return "partial AST: " + problem
	}
	second, finished := robustParse(source)
	if !finished || second.panicked != nil {
		return "second parse of identical source diverged in termination or panic behavior"
	}
	if first, again := diagnosticFingerprint(result), diagnosticFingerprint(second.result); first != again {
		return fmt.Sprintf("recovery is not deterministic:\nfirst:\n%s\nsecond:\n%s", first, again)
	}
	return ""
}

func diagnosticFingerprint(result ParseResult) string {
	var builder strings.Builder
	for _, diagnostic := range result.Diagnostics {
		fmt.Fprintf(&builder, "%s %d:%d %s\n", diagnostic.ID, diagnostic.Primary.Line, diagnostic.Primary.Column, diagnostic.Message)
	}
	for _, event := range result.Recovery {
		fmt.Fprintf(&builder, "recovery %s %s %d:%d skipped=%d\n", event.Kind, event.DiagnosticID, event.Start.Line, event.Start.Column, event.Skipped)
	}
	fmt.Fprintf(&builder, "statements=%d\n", len(result.Program.Statements))
	return builder.String()
}

// findInvalidASTShape walks the retained AST and rejects nil or typed-nil
// nodes stored in node lists. Recovery represents missing members through
// invalid nodes or omission, never through nil list entries.
func findInvalidASTShape(program *ast.Program) string {
	visited := map[uintptr]struct{}{}
	var walk func(value reflect.Value, path string) string
	walk = func(value reflect.Value, path string) string {
		switch value.Kind() {
		case reflect.Pointer:
			if value.IsNil() {
				return ""
			}
			address := value.Pointer()
			if _, seen := visited[address]; seen {
				return ""
			}
			visited[address] = struct{}{}
			return walk(value.Elem(), path)
		case reflect.Interface:
			if value.IsNil() {
				return ""
			}
			return walk(value.Elem(), path)
		case reflect.Struct:
			valueType := value.Type()
			if valueType.PkgPath() != reflect.TypeOf(ast.Program{}).PkgPath() {
				return ""
			}
			for index := 0; index < value.NumField(); index++ {
				field := valueType.Field(index)
				if !field.IsExported() {
					continue
				}
				if problem := walk(value.Field(index), path+"."+field.Name); problem != "" {
					return problem
				}
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				element := value.Index(index)
				elementPath := fmt.Sprintf("%s[%d]", path, index)
				if element.Kind() == reflect.Interface && isNilNode(element) {
					return elementPath + " is a nil " + value.Type().Elem().String()
				}
				if problem := walk(element, elementPath); problem != "" {
					return problem
				}
			}
		}
		return ""
	}
	return walk(reflect.ValueOf(program), "Program")
}

func isNilNode(value reflect.Value) bool {
	if value.IsNil() {
		return true
	}
	inner := value.Elem()
	switch inner.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Func, reflect.Chan, reflect.Interface:
		return inner.IsNil()
	}
	return false
}

// robustnessCorpus returns the accepted (non-_invalid) Sec fixtures used as
// mutation and truncation seeds, ordered deterministically.
func robustnessCorpus(t testing.TB) map[string]string {
	t.Helper()
	root := filepath.Join("..", "..", "testdata")
	corpus := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".sec" || strings.HasSuffix(path, "_invalid.sec") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		corpus[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatalf("read robustness corpus: %v", err)
	}
	if len(corpus) == 0 {
		t.Fatal("robustness corpus is empty")
	}
	return corpus
}

func sortedCorpusNames(corpus map[string]string) []string {
	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type sourceToken struct {
	typ   lexer.TokenType
	start int
	end   int
}

func lexSourceTokens(source string) []sourceToken {
	l := lexer.NewWithFile(source, "robustness.sec")
	var tokens []sourceToken
	for limit := 4*len(source) + 16; limit > 0; limit-- {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			break
		}
		tokens = append(tokens, sourceToken{typ: token.Type, start: token.ByteStart, end: token.ByteEnd})
	}
	return tokens
}

// TestParserTerminatesOnRepeatedMalformedTokens covers the rulebook's progress
// tests: long runs of one malformed token in every recovery context must
// terminate with bounded diagnostics.
func TestParserTerminatesOnRepeatedMalformedTokens(t *testing.T) {
	runs := []string{"}", ",", "?", "+", "(", ")", "[", "]", "{", ":", "->", "<-", "=>", "|", "..", "@", "#", ".", ";", "fn", "let", "match", "type", "\"", "'", "/*", "<", ">"}
	contexts := []struct {
		name   string
		prefix string
		suffix string
	}{
		{"top-level", "module main\n", "\nfn After() void {}\n"},
		{"block", "module main\nfn Test() void {\n", "\n}\nfn After() void {}\n"},
		{"expression", "module main\nfn Test() void {\n\tlet value := ", "\n}\nfn After() void {}\n"},
		{"call-arguments", "module main\nfn Test() void {\n\tCall(1, ", "\n}\nfn After() void {}\n"},
		{"struct-members", "module main\ntype Point struct {\n\tx: int,\n", "\n}\nfn After() void {}\n"},
		{"parameters", "module main\nfn Test(value: int, ", ") void {}\nfn After() void {}\n"},
		{"match-arms", "module main\nfn Test(value: int) int {\n\treturn match value {\n", "\n\t}\n}\nfn After() void {}\n"},
		{"switch-body", "module main\nfn Test(value: int) void {\n\tswitch value {\n", "\n\t}\n}\nfn After() void {}\n"},
		{"type-reference", "module main\nfn Test(value: ", ") void {}\nfn After() void {}\n"},
		{"generic-arguments", "module main\nfn Test(value: Map[", "]) void {}\nfn After() void {}\n"},
	}
	for _, context := range contexts {
		for _, run := range runs {
			for _, separator := range []string{"", " ", "\n"} {
				body := strings.Repeat(run+separator, 256)
				source := context.prefix + body + context.suffix
				if problem := checkRecoveryInvariants(source); problem != "" {
					t.Errorf("%s: %q repeated with separator %q: %s", context.name, run, separator, problem)
				}
			}
		}
	}
}

// TestParserTerminatesOnDeepAndLongMalformedInput covers nested delimiters,
// long malformed lists, and long malformed expressions from the fuzz-test
// categories without depending on random input.
func TestParserTerminatesOnDeepAndLongMalformedInput(t *testing.T) {
	cases := map[string]string{
		"nested-parens":         "module main\nfn Test() void {\n\tlet value := " + strings.Repeat("(", 200) + "1" + "\n}\n",
		"nested-brackets":       "module main\nfn Test() void {\n\tlet value := " + strings.Repeat("[", 200) + "\n}\n",
		"nested-braces":         "module main\nfn Test() void " + strings.Repeat("{", 200) + "\n",
		"nested-closed":         "module main\nfn Test() void {\n\tlet value := " + strings.Repeat("(", 150) + "1" + strings.Repeat(")", 150) + "\n}\n",
		"mismatched-closers":    "module main\nfn Test() void {\n\tlet value := " + strings.Repeat("([{", 60) + strings.Repeat(")]}", 60) + "\n}\n",
		"long-argument-list":    "module main\nfn Test() void {\n\tCall(" + strings.Repeat("1 2, ", 400) + ")\n}\n",
		"long-parameter-list":   "module main\nfn Test(" + strings.Repeat("value int, ", 400) + ") void {}\n",
		"long-struct-fields":    "module main\ntype Point struct {\n" + strings.Repeat("\tx int\n", 400) + "}\n",
		"long-binary-chain":     "module main\nfn Test() int {\n\treturn " + strings.Repeat("1 + * ", 500) + "1\n}\n",
		"long-unary-chain":      "module main\nfn Test() int {\n\treturn " + strings.Repeat("-!", 500) + "1\n}\n",
		"long-member-chain":     "module main\nfn Test() int {\n\treturn value" + strings.Repeat(".", 500) + "\n}\n",
		"unterminated-comment":  "module main\nfn Test() void {\n/* " + strings.Repeat("comment ", 100),
		"unterminated-string":   "module main\nfn Test() void {\n\tlet text := \"" + strings.Repeat("text ", 100),
		"unterminated-interp":   "module main\nfn Test() void {\n\tlet text := $\"{" + strings.Repeat("value + ", 100),
		"unterminated-raw":      "module main\nfn Test() void {\n\tlet text := `" + strings.Repeat("raw ", 100),
		"unterminated-char":     "module main\nfn Test() void {\n\tlet c := '",
		"unicode-identifiers":   "module main\nfn Tëst(välue: ïnt) void {\n\tlet 名前 := välue\n\tlet a\u0301 := 名前\n}\n",
		"invalid-scalars":       "module main\nfn Test() void {\n\tlet value := \x00\x01\u200b\ufeff 1\n}\n",
		"invalid-utf8":          "module main\nfn Test() void {\n\tlet value := \xff\xfe\xc3 1\n}\n",
		"only-newlines":         strings.Repeat("\n", 1000),
		"empty":                 "",
		"bare-module":           "module",
		"nested-match-try":      "module main\nfn Test() int {\n\treturn " + strings.Repeat("try match x {\n", 80) + "\n}\n",
		"nested-generic-type":   "module main\nfn Test(value: " + strings.Repeat("Map[", 200) + ") void {}\n",
		"nested-lambda":         "module main\nfn Test() void {\n\tlet f := " + strings.Repeat("fn() int { return ", 120) + "\n}\n",
		"long-attribute-chain":  "module main\n" + strings.Repeat("@inline(", 200) + "\nfn Test() void {}\n",
		"long-import-group":     "module main\nimport (\n" + strings.Repeat("\t\"core\" as\n", 300) + "\nfn Test() void {}\n",
		"long-target-directive": "#target " + strings.Repeat("os == ", 300) + "\nmodule main\n",
	}
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if problem := checkRecoveryInvariants(cases[name]); problem != "" {
			t.Errorf("%s: %s", name, problem)
		}
	}
}

// truncationPoints chooses token-boundary prefixes. Small fixtures are cut
// after every token as required; larger fixtures use a deterministic stride so
// the full corpus stays inside an ordinary test budget.
func truncationPoints(tokens []sourceToken, budget int) []int {
	if len(tokens) == 0 {
		return nil
	}
	stride := 1
	if len(tokens) > budget {
		stride = (len(tokens) + budget - 1) / budget
	}
	points := make([]int, 0, len(tokens)/stride+1)
	for index := 0; index < len(tokens); index += stride {
		points = append(points, tokens[index].end)
	}
	if last := tokens[len(tokens)-1].end; points[len(points)-1] != last {
		points = append(points, last)
	}
	return points
}

// TestParserRecoveryOnTruncatedFixtures truncates every accepted fixture at
// token boundaries. Every prefix must satisfy the recovery invariants.
func TestParserRecoveryOnTruncatedFixtures(t *testing.T) {
	budget := 160
	if testing.Short() {
		budget = 24
	}
	corpus := robustnessCorpus(t)
	for _, name := range sortedCorpusNames(corpus) {
		source := corpus[name]
		for _, cut := range truncationPoints(lexSourceTokens(source), budget) {
			if problem := checkRecoveryInvariants(source[:cut]); problem != "" {
				t.Errorf("%s truncated at byte %d: %s\n--- prefix tail ---\n%s", name, cut, problem, sourceTail(source[:cut]))
				break
			}
		}
	}
}

type tokenMutation struct {
	name  string
	apply func(source string, tokens []sourceToken, index int) (string, bool)
}

var recoveryMutations = []tokenMutation{
	{"delete", func(source string, tokens []sourceToken, index int) (string, bool) {
		token := tokens[index]
		return source[:token.start] + source[token.end:], true
	}},
	{"duplicate", func(source string, tokens []sourceToken, index int) (string, bool) {
		token := tokens[index]
		return source[:token.end] + " " + source[token.start:token.end] + source[token.end:], true
	}},
	{"replace-delimiter", func(source string, tokens []sourceToken, index int) (string, bool) {
		replacements := map[lexer.TokenType]string{
			lexer.LPAREN: "[", lexer.RPAREN: "]", lexer.LBRACKET: "{", lexer.RBRACKET: "}",
			lexer.LBRACE: "(", lexer.RBRACE: ")", lexer.COMMA: ";", lexer.COLON: ",",
		}
		token := tokens[index]
		replacement, ok := replacements[token.typ]
		if !ok {
			return "", false
		}
		return source[:token.start] + replacement + source[token.end:], true
	}},
	{"move-closing-brace", func(source string, tokens []sourceToken, index int) (string, bool) {
		token := tokens[index]
		if token.typ != lexer.RBRACE {
			return "", false
		}
		without := source[:token.start] + source[token.end:]
		target := tokens[(index*7+3)%len(tokens)].start
		if target > token.start {
			target -= token.end - token.start
		}
		return without[:target] + "}" + without[target:], true
	}},
	{"insert-keyword", func(source string, tokens []sourceToken, index int) (string, bool) {
		keywords := []string{"fn", "let", "return", "match", "type", "if", "else", "try", "struct", "impl", "while", "for"}
		token := tokens[index]
		return source[:token.start] + keywords[index%len(keywords)] + " " + source[token.start:], true
	}},
}

// TestParserRecoveryOnMutatedFixtures applies the rulebook mutation families to
// every accepted fixture at deterministic token positions.
func TestParserRecoveryOnMutatedFixtures(t *testing.T) {
	perMutation := 24
	if testing.Short() {
		perMutation = 4
	}
	corpus := robustnessCorpus(t)
	for _, name := range sortedCorpusNames(corpus) {
		source := corpus[name]
		tokens := lexSourceTokens(source)
		if len(tokens) == 0 {
			continue
		}
		for mutationIndex, mutation := range recoveryMutations {
			applied := 0
			stride := len(tokens)/perMutation + 1
			for offset := 0; offset < len(tokens) && applied < perMutation; offset++ {
				index := (offset*stride + mutationIndex) % len(tokens)
				mutated, ok := mutation.apply(source, tokens, index)
				if !ok {
					continue
				}
				applied++
				if problem := checkRecoveryInvariants(mutated); problem != "" {
					t.Errorf("%s %s at token %d: %s\n--- mutated tail ---\n%s", name, mutation.name, index, problem, sourceTail(mutated[:min(len(mutated), tokens[index].end+16)]))
					break
				}
			}
		}
	}
}

// TestParserRecoveryReplaysRegressionSeeds replays inputs that previously
// violated a recovery invariant. Add every fixed robustness bug here.
func TestParserRecoveryReplaysRegressionSeeds(t *testing.T) {
	for name, source := range parserRecoveryRegressionSeeds {
		if problem := checkRecoveryInvariants(source); problem != "" {
			t.Errorf("%s: %s", name, problem)
		}
	}
}

var parserRecoveryRegressionSeeds = map[string]string{
	// default without ':' retried the same DEFAULT token and grew without bound.
	"switch-default-at-eof":           "module main\nfn f(v: int) int {\n    switch v {\n        case 1:\n            return 1\n        default",
	"switch-default-without-colon":    "module main\nfn f(v: int) int {\n    switch v {\n        default return 1\n    }\n}\n",
	"switch-case-body-invalid-fn":     "module main\nfn f(v: int) int {\n    switch v {\n        case 1:\n            fn result = 10\n        default:\n            return 0\n    }\n}\n",
	"generic-list-trailing-comma-eof": "module main\ntype Pair[A,",
	"generic-list-foreign-closer":     "module main\nfn F[\n    a: A,\n) int {\n    return 0\n}\n",
}

// FuzzParserRecovery is the native Go fuzz target for lexer input and parser
// token streams. Seeds are the accepted fixtures plus the regression seeds; run
// it with `go test ./internal/parser -run '^$' -fuzz FuzzParserRecovery`.
func FuzzParserRecovery(f *testing.F) {
	corpus := robustnessCorpus(f)
	for _, name := range sortedCorpusNames(corpus) {
		f.Add(corpus[name])
	}
	for _, source := range parserRecoveryRegressionSeeds {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 1<<16 {
			t.Skip("input larger than fuzz budget")
		}
		if problem := checkRecoveryInvariants(source); problem != "" {
			t.Fatal(problem)
		}
	})
}

func sourceTail(source string) string {
	const width = 240
	if len(source) <= width {
		return source
	}
	return source[len(source)-width:]
}
