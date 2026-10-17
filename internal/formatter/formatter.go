// Package formatter provides the canonical, reusable Sec source formatter.
package formatter

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

type Options struct {
	// Fix enables the opt-in Language Corrections layer (`language_corrections`).
	// Ordinary formatting, including current CLI and LSP entry points, leaves it
	// disabled.
	Fix bool
	// IndentationWidth is `indentation_width`: 2, 4, or 6 spaces. Zero selects
	// the default of 4.
	IndentationWidth int
	// VerticalStyle is `vertical_style`; empty selects structured.
	VerticalStyle VerticalStyle
	// MaxAlignmentPadding is `max_alignment_padding` in spaces. Zero selects
	// the default of four indentation widths (§ 4(14)).
	MaxAlignmentPadding int
}

// VerticalStyle selects whether optional structural blank lines are emitted.
//
// Rules:
//   - rules/tooling/formatter.md — § 4(6)–(7), § 7(3)–(4)
type VerticalStyle string

const (
	VerticalStructured VerticalStyle = "structured"
	VerticalCompact    VerticalStyle = "compact"
)

// DefaultOptions returns the default project formatting configuration of
// format_version = 1.
//
// Rules:
//   - rules/tooling/formatter.md — § 4(1)–(5)
func DefaultOptions() Options {
	return Options{IndentationWidth: 4, VerticalStyle: VerticalStructured}
}

// ValidateOptions rejects configuration values outside the canonical
// format_version = 1 domains.
//
// Rules:
//   - rules/tooling/formatter.md — § 4(6)–(8)
func ValidateOptions(options Options) error {
	switch options.IndentationWidth {
	case 0, 2, 4, 6:
	default:
		return fmt.Errorf("indentation_width must be 2, 4, or 6 spaces, got %d", options.IndentationWidth)
	}
	switch options.VerticalStyle {
	case "", VerticalStructured, VerticalCompact:
	default:
		return fmt.Errorf("vertical_style must be \"structured\" or \"compact\", got %q", options.VerticalStyle)
	}
	if options.MaxAlignmentPadding < 0 {
		return fmt.Errorf("max_alignment_padding must be a positive number of spaces, got %d", options.MaxAlignmentPadding)
	}
	return nil
}

// indentationWidth returns the effective indentation width; values outside
// the canonical domain fall back to the default.
func (options Options) indentationWidth() int {
	switch options.IndentationWidth {
	case 2, 4, 6:
		return options.IndentationWidth
	default:
		return 4
	}
}

// maximumAlignmentPadding is the project's `max_alignment_padding`, or four
// indentation widths when it is not set (§ 4(14), MD-098).
func (options Options) maximumAlignmentPadding() int {
	if options.MaxAlignmentPadding > 0 {
		return options.MaxAlignmentPadding
	}
	return 4 * options.indentationWidth()
}

// compact reports the compact vertical style.
func (options Options) compact() bool {
	return options.VerticalStyle == VerticalCompact
}

type Source struct{ Text string }
type Result struct {
	Text      string
	Comments  []ast.CommentAttachment
	Malformed bool
}

// Format returns canonical source together with parser-owned comment
// attachments whose positions describe the formatted output.
//
// Rules:
//   - rules/tooling/formatter.md — "Comment attachment"
//   - rules/tooling/formatter.md — Appendix A.5 "Build lossless syntax and trivia support"
func Format(source Source, options Options) Result {
	// rules/tooling/formatter.md — §25: lexically uncertain source is
	// preserved byte-for-byte; Language Corrections never repair it either.
	if hasUncertainConcreteSyntax(cst.Build(source.Text, "")) {
		program := parser.New(lexer.New(source.Text)).ParseProgram()
		return Result{
			Text:      source.Text,
			Comments:  append([]ast.CommentAttachment(nil), program.Comments...),
			Malformed: true,
		}
	}
	text := format(source.Text, options)
	program := parser.New(lexer.New(text)).ParseProgram()
	return Result{Text: text, Comments: append([]ast.CommentAttachment(nil), program.Comments...)}
}

// hasUncertainConcreteSyntax implements the first conservative malformed-
// source boundary from the lossless CST: lexical errors and unmatched or
// incomplete real delimiters protect the complete document byte-for-byte.
// Parser recovery ranges will later permit safe formatting around narrower
// uncertain regions without treating every parser diagnostic as malformed.
//
// Rules:
//   - rules/tooling/formatter.md — §25 "Malformed and incomplete source"
func hasUncertainConcreteSyntax(document cst.Document) bool {
	if len(document.Diagnostics) > 0 || len(document.UnmatchedClosers) > 0 {
		return true
	}
	for _, group := range document.Groups {
		if group.Close < 0 {
			return true
		}
	}
	return false
}

type branch struct {
	depth         int
	active, extra bool
}

func format(text string, options Options) string {
	text = strings.TrimPrefix(text, "\uFEFF")
	// rules/foundations/lexical_structure.md §2 "Line endings": the formatter
	// writes LF line endings; no formatter configuration currently selects
	// another convention, so CRLF and bare CR input are normalized to LF.
	normal := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(text)
	if options.Fix {
		normal = fixMissingListSeparators(normal)
		normal = fixRedundantNestedParentheses(normal)
		normal = fixRedundantControlConditionParentheses(normal)
		normal = fixStateTestBadPractice(normal)
		normal = fixLegacyAssignedNamedType(normal)
	}
	normal = formatImportRegion(normal, options.compact())
	normal = formatStructuralBlockLayout(normal)
	normal = formatCallableParameterListLayout(normal)
	normal = formatPropertyAccessorLayout(normal)
	normal = formatCSTTokenSpacing(normal)
	normal = formatCSTBlockComments(normal)
	blockCommentLines := standaloneBlockCommentLines(normal)
	tokenLines := multilineTokenLines(normal)
	lines := strings.Split(normal, "\n")
	hadFinal := strings.HasSuffix(normal, "\n")
	if hadFinal {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, len(lines))
	indent := 0
	blank := false
	// lastOpensBlock records that the previous emitted code line ends with an
	// opening brace, so a following blank line would only decorate the block.
	lastOpensBlock := false
	branches := []branch{}
	for lineIndex, line := range lines {
		commentLine := blockCommentLines[lineIndex+1]
		multiline, inMultiline := tokenLines[lineIndex]
		if inMultiline && multiline.continuation {
			// rules/tooling/formatter.md — §6(8): the continuation of a
			// multiline string literal is program data and is kept verbatim.
			out = append(out, line)
			indent += delimiters(multiline.tail)
			if indent < 0 {
				indent = 0
			}
			lastOpensBlock = strings.HasSuffix(strings.TrimSpace(codeBeforeTrailingComment(multiline.tail)), "{")
			continue
		}
		line = strings.ReplaceAll(line, "\t", "    ")
		line = strings.TrimRight(line, " \t")
		line = strings.TrimSpace(line)
		if line == "" {
			if len(out) > 0 {
				blank = true
			}
			continue
		}
		// rules/declarations/static.md, sections 3 and 25. Module storage is
		// already static; canonical formatting removes the redundant modifier.
		if indent == 0 && strings.HasPrefix(line, "static let ") {
			line = strings.TrimPrefix(line, "static ")
		}
		if options.Fix {
			line = normalizeReversedTypeDeclaration(line)
			line = normalizeFunc(line)
		}
		level := indent
		if !commentLine {
			level -= closing(line)
		}
		if level < 0 {
			level = 0
		}
		for len(branches) > 0 && level < branches[len(branches)-1].depth {
			branches = branches[:len(branches)-1]
		}
		at := -1
		if branchClause(line) {
			for i := len(branches) - 1; i >= 0; i-- {
				if branches[i].depth == level {
					at = i
					break
				}
			}
		}
		extra := 0
		for i, b := range branches {
			if b.extra && b.active && level >= b.depth && i != at {
				extra++
			}
		}
		// rules/tooling/formatter.md — §7(2): no empty line immediately inside
		// an opening or closing brace merely for decoration.
		if blank && (lastOpensBlock || strings.HasPrefix(line, "}")) {
			blank = false
		}
		if blank && len(out) > 0 {
			out = append(out, "")
			blank = false
		}
		out = append(out, strings.Repeat(" ", (level+extra)*options.indentationWidth())+line)
		delimiterLine := line
		if inMultiline && multiline.start {
			delimiterLine = strings.TrimSpace(multiline.masked)
		}
		lastOpensBlock = !commentLine && strings.HasSuffix(strings.TrimSpace(codeBeforeTrailingComment(delimiterLine)), "{")
		delta := 0
		if !commentLine {
			delta = delimiters(delimiterLine)
		}
		indent += delta
		if indent < 0 {
			indent = 0
		}
		if !commentLine && branchStart(line) && delta > 0 {
			branches = append(branches, branch{depth: indent, extra: switchStart(line)})
		}
		if at >= 0 {
			branches[at].active = true
		}
	}
	// rules/tooling/formatter.md, Alignment groups and General trailing-comment
	// alignment. Run after indentation so CLI and LSP observe identical visual
	// columns and comment text never participates in structural indentation.
	out = alignDeclarationTrailingComments(out)
	result := strings.Join(out, "\n")
	result = formatExecutableBlocks(result, options.indentationWidth())
	result = formatCSTRoles(result, options.maximumAlignmentPadding())
	result = formatMultilineInfixLayout(result, options.indentationWidth())
	result = formatExpressionContinuations(result, options.indentationWidth())
	// The role pass may widen field, register, and named-type columns; align
	// trailing comments again against those final columns so a second
	// formatting pass is a fixed point (rules/tooling/formatter.md §29).
	result = strings.Join(alignDeclarationTrailingComments(strings.Split(result, "\n")), "\n")
	result = formatCSTBlockComments(result)
	result = formatCSTLineComments(result)
	if hadFinal || result != "" {
		result += "\n"
	}
	return result
}

// alignDeclarationTrailingComments aligns local groups inside nominal
// declaration blocks. A blank line, standalone comment, multiline item, or
// nested block ends a group; a trailing comment does not. Trailing comments
// occupy the next structural column, which begins one standard space after
// the widest code cell of the group, the same spacing that separates
// `identifier:` from its type in the widest row.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 5.12–5.19
//   - rules/tooling/formatter.md — § 9(2), § 12(4)–(6)
func alignDeclarationTrailingComments(lines []string) []string {
	for opener := 0; opener < len(lines); opener++ {
		if !isDeclarationAlignmentOpener(lines[opener]) {
			continue
		}
		baseIndent := leadingSpaces(lines[opener])
		end := declarationBlockEnd(lines, opener, baseIndent)
		if end < 0 {
			continue
		}
		alignDeclarationBodyGroups(lines, opener+1, end, baseIndent+4)
	}
	return lines
}

func isDeclarationAlignmentOpener(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasSuffix(codeBeforeTrailingComment(trimmed), "{") {
		return false
	}
	if strings.HasPrefix(trimmed, "enum ") || strings.HasPrefix(trimmed, "interface ") {
		return true
	}
	if !strings.HasPrefix(trimmed, "type ") {
		return false
	}
	return strings.Contains(trimmed, " struct {") ||
		strings.Contains(trimmed, " register[") ||
		strings.Contains(trimmed, " union {") ||
		strings.Contains(trimmed, " union error {") ||
		strings.Contains(trimmed, " enum {")
}

func declarationBlockEnd(lines []string, opener int, baseIndent int) int {
	for index := opener + 1; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		if leadingSpaces(lines[index]) == baseIndent && strings.HasPrefix(trimmed, "}") {
			return index
		}
	}
	return -1
}

func alignDeclarationBodyGroups(lines []string, start int, end int, itemIndent int) {
	group := []int{}
	flush := func() {
		alignTrailingCommentGroup(lines, group)
		group = group[:0]
	}
	for index := start; index < end; index++ {
		line := lines[index]
		trimmed := strings.TrimSpace(line)
		code, _, _ := splitTrailingComment(trimmed)
		if trimmed == "" || leadingSpaces(line) != itemIndent || isStandaloneComment(trimmed) || delimiters(strings.TrimSpace(code)) > 0 {
			flush()
			continue
		}
		group = append(group, index)
	}
	flush()
}

// structuralColumnSpacing is the formatter's standard spacing between
// structural columns: the single space after `identifier:` in the widest row
// of an aligned group (formatter.md § 9(3), § 9(5), § 22(3)).
const structuralColumnSpacing = 1

func alignTrailingCommentGroup(lines []string, group []int) {
	maxCodeWidth := 0
	commented := 0
	for _, index := range group {
		code, _, hasComment := splitTrailingComment(lines[index])
		if width := utf8.RuneCountInString(code); width > maxCodeWidth {
			maxCodeWidth = width
		}
		if hasComment {
			commented++
		}
	}
	if commented == 0 {
		return
	}
	for _, index := range group {
		code, comment, hasComment := splitTrailingComment(lines[index])
		if !hasComment {
			continue
		}
		padding := maxCodeWidth - utf8.RuneCountInString(code) + structuralColumnSpacing
		lines[index] = code + strings.Repeat(" ", padding) + comment
	}
}

// splitTrailingComment splits a declaration-table row at its trailing
// comment: a line comment, or a single-line block comment that ends the line
// after code. Both occupy the same trailing-comment column; a block comment
// followed by more code on the line is inline, not trailing (MD-097).
//
// Rules:
//   - rules/tooling/formatter.md — § 9(8), § 9(8b), § 12(5), § 12(9)
func splitTrailingComment(line string) (string, string, bool) {
	if index := trailingBlockCommentIndex(line); index >= 0 {
		return strings.TrimRight(line[:index], " \t"), strings.TrimRight(line[index:], " \t"), true
	}
	return splitTrailingLineComment(line)
}

// trailingBlockCommentIndex returns the start of a single-line block comment
// that is the last lexical element of a line containing code before it, or -1.
func trailingBlockCommentIndex(line string) int {
	if !strings.Contains(line, "/*") {
		return -1
	}
	syntax := cst.Build(line, "")
	if len(syntax.Diagnostics) != 0 {
		return -1
	}
	last := -1
	for index, element := range syntax.Elements {
		if element.Kind != cst.Whitespace {
			last = index
		}
	}
	if last <= 0 {
		return -1
	}
	element := syntax.Elements[last]
	if element.Kind != cst.Comment || !strings.HasPrefix(element.Text, "/*") || strings.Contains(element.Text, "\n") {
		return -1
	}
	for index := 0; index < last; index++ {
		if syntax.Elements[index].Kind == cst.Token {
			return element.Span.Start
		}
	}
	return -1
}

func splitTrailingLineComment(line string) (string, string, bool) {
	index := trailingLineCommentIndex(line)
	if index < 0 {
		return strings.TrimRight(line, " \t"), "", false
	}
	return strings.TrimRight(line[:index], " \t"), line[index:], true
}

func codeBeforeTrailingComment(line string) string {
	code, _, _ := splitTrailingLineComment(line)
	return strings.TrimSpace(code)
}

// trailingLineCommentIndex uses lexer-backed CST elements to distinguish a
// real line comment from comment-like text inside literals or same-line block
// comments.
//
// Rules:
//   - rules/foundations/lexical_structure.md — § 5 "Comments"
//   - rules/tooling/formatter.md — "Source model", "Line comments"
func trailingLineCommentIndex(line string) int {
	if !strings.Contains(line, "//") {
		return -1
	}
	syntax := cst.Build(line, "")
	if len(syntax.Diagnostics) == 0 {
		for _, element := range syntax.Elements {
			if element.Kind == cst.Comment && strings.HasPrefix(element.Text, "//") {
				return element.Span.Start
			}
		}
		return -1
	}
	// On malformed source, retain the existing conservative line scanner until
	// parser recovery nodes can carry structural formatting decisions.
	return trailingLineCommentIndexFallback(line)
}

// trailingLineCommentIndexFallback keeps malformed single-line source on the
// prior conservative path until parser recovery is represented in the CST.
//
// Rules:
//   - rules/tooling/formatter.md — "Malformed and incomplete source"
func trailingLineCommentIndexFallback(line string) int {
	literals := literalEnds(line)
	for index := 0; index+1 < len(line); index++ {
		if end, ok := literals[index]; ok {
			index = end - 1
			continue
		}
		character := line[index]
		if character == '/' && line[index+1] == '/' {
			return index
		}
	}
	return -1
}

func isStandaloneComment(trimmed string) bool {
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "*/")
}

func leadingSpaces(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

func normalizeReversedTypeDeclaration(line string) string {
	for _, kind := range []string{"struct", "union"} {
		prefix := "type " + kind + " "
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		name, rest := leadingIdentifier(line[len(prefix):])
		if name != "" && (rest == "" || strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "{")) {
			return "type " + name + " " + kind + rest
		}
	}

	const registerPrefix = "type register "
	if strings.HasPrefix(line, registerPrefix) {
		name, rest := leadingIdentifier(line[len(registerPrefix):])
		if name != "" && strings.HasPrefix(rest, "[") {
			return "type " + name + " register" + rest
		}
	}
	return line
}

func leadingIdentifier(text string) (string, string) {
	end := 0
	for index, r := range text {
		if r != '_' && !unicode.IsLetter(r) && (index == 0 || !unicode.IsDigit(r)) {
			break
		}
		end = index + len(string(r))
	}
	if end == 0 {
		return "", text
	}
	return text[:end], text[end:]
}

func normalizeFunc(line string) string {
	if !strings.HasPrefix(line, "func ") {
		return line
	}
	open := strings.Index(line, "(")
	if open < 0 || !ident(strings.TrimSpace(line[5:open])) {
		return line
	}
	close := matchingParen(line, open)
	if close < 0 || !strings.Contains(line[close+1:], "{") {
		return line
	}
	return "fn " + line[5:]
}
func ident(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r != '_' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}

func matchingParen(s string, open int) int {
	depth, angle := 0, 0
	literals := literalEnds(s)
	for index := 0; index < len(s); index++ {
		if end, ok := literals[index]; ok {
			index = end - 1
			continue
		}
		if index < open {
			continue
		}
		i := index - open
		switch s[index] {
		case '<':
			angle++
		case '>':
			if angle > 0 {
				angle--
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && angle == 0 {
				return open + i
			}
		}
	}
	return -1
}
func split(s string) []string {
	parts := []string{}
	start, paren, bracket, brace, angle := 0, 0, 0, 0, 0
	literals := literalEnds(s)
	unterminated := false
	for i := 0; i < len(s); i++ {
		if end, ok := literals[i]; ok {
			if end >= len(s) && !literalClosed(s[i:end]) {
				unterminated = true
			}
			i = end - 1
			continue
		}
		switch s[i] {
		case '(':
			paren++
		case ')':
			paren--
		case '[':
			bracket++
		case ']':
			bracket--
		case '{':
			brace++
		case '}':
			brace--
		case '<':
			angle++
		case '>':
			if angle > 0 {
				angle--
			}
		case ',':
			if paren == 0 && bracket == 0 && brace == 0 && angle == 0 {
				if p := strings.TrimSpace(s[start:i]); p != "" {
					parts = append(parts, p)
				}
				start = i + 1
			}
		}
	}
	if unterminated || paren != 0 || bracket != 0 || brace != 0 || angle != 0 {
		return nil
	}
	if p := strings.TrimSpace(s[start:]); p != "" {
		parts = append(parts, p)
	}
	return parts
}
func switchStart(s string) bool {
	return strings.HasPrefix(s, "switch ") || strings.HasPrefix(s, "switch{")
}
func branchStart(s string) bool {
	return switchStart(s) || s == "select {" || strings.HasPrefix(s, "select ")
}
func branchClause(s string) bool {
	return s == "case" || strings.HasPrefix(s, "case ") || strings.HasPrefix(s, "case\t") || s == "default:" || strings.HasPrefix(s, "default ") || s == "default => {" || strings.HasPrefix(s, "after ") || strings.HasSuffix(s, "=> {")
}
func closing(s string) int {
	for _, r := range s {
		if r == ' ' || r == '\t' {
			continue
		}
		if r == '}' || r == ')' || r == ']' {
			return 1
		}
		return 0
	}
	return 0
}
func delimiters(s string) int {
	delta := 0
	lastCode := byte(0)
	literals := literalEnds(s)
	for i := 0; i < len(s); i++ {
		if end, ok := literals[i]; ok {
			lastCode = s[i]
			i = end - 1
			continue
		}
		r := s[i]
		if i+1 < len(s) && r == '/' && s[i+1] == '/' {
			break
		}
		if r != ' ' && r != '\t' && r != '\n' && r != '\r' {
			lastCode = r
		}
		if r == '{' || r == '(' || r == '[' {
			delta++
		}
		if r == '}' || r == ')' || r == ']' {
			delta--
		}
	}
	// A line such as `}) {` closes a multiline call/struct literal and opens
	// the attached try-handler block. The line-oriented formatter intentionally
	// collapses multiple opens on the preceding line to one visual indentation
	// level, so consuming the raw net -1 here would lose the newly opened block.
	// Keep the current level when a closing-heavy continuation hands off to a
	// trailing opening brace.
	if delta < 0 && lastCode == '{' {
		return 0
	}
	if delta < 0 {
		return -1
	}
	if delta > 0 {
		return 1
	}
	return 0
}
