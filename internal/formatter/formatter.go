// Package formatter provides the canonical, reusable Sec source formatter.
package formatter

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/cst"
	"sec/internal/lexer"
	"sec/internal/parser"
)

type Options struct {
	// Fix enables the opt-in Language Corrections layer. Ordinary formatting,
	// including current CLI and LSP entry points, leaves it disabled.
	Fix bool
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
	if !options.Fix {
		syntax := cst.Build(source.Text, "")
		if hasUncertainConcreteSyntax(syntax) {
			program := parser.New(lexer.New(source.Text)).ParseProgram()
			return Result{
				Text:      source.Text,
				Comments:  append([]ast.CommentAttachment(nil), program.Comments...),
				Malformed: true,
			}
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
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	normal := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(text)
	if options.Fix {
		normal = fixRedundantNestedParentheses(normal)
		normal = fixRedundantControlConditionParentheses(normal)
	}
	normal = formatCSTBlockComments(normal)
	blockCommentLines := standaloneBlockCommentLines(normal)
	lines := strings.Split(normal, "\n")
	hadFinal := strings.HasSuffix(normal, "\n")
	if hadFinal {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, len(lines))
	indent := 0
	blank := false
	branches := []branch{}
	for lineIndex, line := range lines {
		commentLine := blockCommentLines[lineIndex+1]
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
		// static.md section 6: impl static let is distinct from instance let.
		if strings.HasPrefix(line, "@noCopy ") {
			if blank && len(out) > 0 {
				out = append(out, "")
				blank = false
			}
			out = append(out, strings.Repeat(" ", indent*4)+"@noCopy")
			line = strings.TrimSpace(strings.TrimPrefix(line, "@noCopy"))
		}
		if options.Fix {
			line = normalizeReversedTypeDeclaration(line)
			line = normalizeFunc(line)
		}
		line = formatPanic(formatAssert(formatSingleLineDelimiterSpacing(formatSingleLineCallSpacing(line))))
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
		if blank && len(out) > 0 {
			out = append(out, "")
			blank = false
		}
		out = append(out, strings.Repeat(" ", (level+extra)*4)+line)
		delta := 0
		if !commentLine {
			delta = delimiters(line)
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
	result = formatCSTRoles(result)
	result = formatCSTBlockComments(result)
	result = formatCSTLineComments(result)
	if hadFinal || result != "" {
		result += "\n"
	}
	if eol != "\n" {
		result = strings.ReplaceAll(result, "\n", eol)
	}
	return result
}

// alignDeclarationTrailingComments aligns local groups inside nominal
// declaration blocks. A blank line, standalone comment, multiline item, or
// nested block ends a group. The comment column starts four spaces after the
// widest code item, as required by rules/tooling/formatter.md, Line comments.
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
		if trimmed == "" || leadingSpaces(line) != itemIndent || isStandaloneComment(trimmed) || delimiters(codeBeforeTrailingComment(trimmed)) > 0 {
			flush()
			continue
		}
		group = append(group, index)
	}
	flush()
}

func alignTrailingCommentGroup(lines []string, group []int) {
	maxCodeWidth := 0
	commented := 0
	for _, index := range group {
		code, _, hasComment := splitTrailingLineComment(lines[index])
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
		code, comment, hasComment := splitTrailingLineComment(lines[index])
		if !hasComment {
			continue
		}
		padding := maxCodeWidth - utf8.RuneCountInString(code) + 4
		lines[index] = code + strings.Repeat(" ", padding) + comment
	}
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
	quote := byte(0)
	escaped := false
	for index := 0; index+1 < len(line); index++ {
		character := line[index]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if character == '\\' && quote != '`' {
				escaped = true
				continue
			}
			if character == quote {
				quote = 0
			}
			continue
		}
		if character == '"' || character == '\'' || character == '`' {
			quote = character
			continue
		}
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

// formatSingleLineCallSpacing implements the single-line call rule in
// rules/tooling/formatter.md. A newline is source layout chosen by the
// programmer and is never removed by this pass.
func formatSingleLineCallSpacing(line string) string {
	if strings.Contains(line, "//") || strings.Contains(line, "/*") || strings.Contains(line, "*/") {
		return line
	}
	return normalizeSingleLineCalls(line)
}

// formatSingleLineDelimiterSpacing is the line-based migration step toward the
// lossless CST required by rules/tooling/formatter.md. It treats balanced
// delimiter groups as structural nodes, retains quoted contents verbatim, and
// never joins or rewrites a group whose matching delimiter is on another line.
func formatSingleLineDelimiterSpacing(line string) string {
	if strings.Contains(line, "//") || strings.Contains(line, "/*") || strings.Contains(line, "*/") {
		return line
	}
	return normalizeSingleLineDelimiters(line)
}

func normalizeSingleLineDelimiters(text string) string {
	var out strings.Builder
	for cursor := 0; cursor < len(text); {
		open := nextStructuralDelimiter(text, cursor)
		if open < 0 {
			out.WriteString(text[cursor:])
			break
		}
		close := matchingDelimiter(text, open)
		if close < 0 {
			// The group continues on another physical line (or is incomplete).
			// Preserve the rest of this line byte-for-byte for the future CST.
			out.WriteString(text[cursor:])
			break
		}

		opener := text[open]
		inner := normalizeSingleLineDelimiters(text[open+1 : close])
		inner = strings.TrimSpace(inner)
		if opener == '(' || opener == '[' {
			if parts := split(inner); parts != nil && strings.Contains(inner, ",") {
				inner = strings.Join(parts, ", ")
			}
		}

		out.WriteString(text[cursor : open+1])
		if opener == '{' && inner != "" {
			out.WriteByte(' ')
			out.WriteString(inner)
			out.WriteByte(' ')
		} else {
			out.WriteString(inner)
		}
		out.WriteByte(text[close])
		cursor = close + 1
	}
	return out.String()
}

func nextStructuralDelimiter(text string, start int) int {
	quote := byte(0)
	escaped := false
	for index := start; index < len(text); index++ {
		character := text[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' && quote != '`' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		if character == '"' || character == '\'' || character == '`' {
			quote = character
			continue
		}
		if character == '(' || character == '[' || character == '{' {
			return index
		}
	}
	return -1
}

func matchingDelimiter(text string, open int) int {
	if open < 0 || open >= len(text) || !isOpeningDelimiter(text[open]) {
		return -1
	}
	stack := []byte{}
	quote := byte(0)
	escaped := false
	for index := open; index < len(text); index++ {
		character := text[index]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if character == '\\' && quote != '`' {
				escaped = true
			} else if character == quote {
				quote = 0
			}
			continue
		}
		if character == '"' || character == '\'' || character == '`' {
			quote = character
			continue
		}
		if isOpeningDelimiter(character) {
			stack = append(stack, character)
			continue
		}
		if !isClosingDelimiter(character) {
			continue
		}
		if len(stack) == 0 || closingDelimiter(stack[len(stack)-1]) != character {
			return -1
		}
		stack = stack[:len(stack)-1]
		if len(stack) == 0 {
			return index
		}
	}
	return -1
}

func isOpeningDelimiter(character byte) bool {
	return character == '(' || character == '[' || character == '{'
}

func isClosingDelimiter(character byte) bool {
	return character == ')' || character == ']' || character == '}'
}

func closingDelimiter(open byte) byte {
	switch open {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	default:
		return 0
	}
}

// normalizeSingleLineCalls recursively normalizes balanced call argument lists
// on one line while leaving grouping parentheses and incomplete syntax intact.
func normalizeSingleLineCalls(text string) string {
	var out strings.Builder
	for cursor := 0; cursor < len(text); {
		open := nextStructuralParen(text, cursor)
		if open < 0 {
			out.WriteString(text[cursor:])
			break
		}
		close := matchingParen(text, open)
		if close < 0 {
			out.WriteString(text[cursor:])
			break
		}

		out.WriteString(text[cursor : open+1])
		inner := normalizeSingleLineCalls(text[open+1 : close])
		if callParen(text, open) && !strings.ContainsAny(inner, "{}") {
			if parts := split(inner); parts != nil {
				inner = strings.Join(parts, ", ")
			}
		}
		out.WriteString(inner)
		out.WriteByte(')')
		cursor = close + 1
	}
	return out.String()
}

// nextStructuralParen finds an opening parenthesis outside quoted literals.
func nextStructuralParen(text string, start int) int {
	quote := byte(0)
	escaped := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		if ch == '(' {
			return i
		}
	}
	return -1
}

// callParen distinguishes an adjacent callee delimiter from whitespace-led
// grouping or declaration-group parentheses.
func callParen(text string, open int) bool {
	if open == 0 {
		return false
	}
	last := rune(0)
	for _, r := range text[:open] {
		last = r
	}
	if unicode.IsSpace(last) {
		return false
	}
	return last == '_' || last == ')' || last == ']' || unicode.IsLetter(last) || unicode.IsDigit(last)
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

// formatAssert canonicalizes the comma separator before an assertion message
// while preserving commas nested in the condition and inside quoted literals.
//
// Rules:
//   - rules/errors/panic.md — § 15.1 "Canonical syntax"
//   - rules/tooling/formatter.md — "Assertion statements"
func formatAssert(line string) string {
	if !strings.HasPrefix(line, "assert ") || strings.Contains(line, "/*") {
		return line
	}
	code, comment, hasComment := splitTrailingLineComment(line)
	depth := 0
	quote := byte(0)
	escaped := false
	separator := -1
	for i := 0; i < len(code); i++ {
		ch := code[i]
		if quote != 0 {
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'':
			quote = ch
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 && strings.HasPrefix(strings.TrimSpace(code[i+1:]), "\"") {
				separator = i
			}
		}
	}
	if separator < 0 {
		return line
	}
	formatted := strings.TrimSpace(code[:separator]) + ", " + strings.TrimSpace(code[separator+1:])
	if hasComment {
		formatted += " " + comment
	}
	return formatted
}

// formatPanic emits exactly one space between the statement keyword and its
// static string-literal payload without rewriting invalid call-like spelling.
//
// Rules:
//   - rules/errors/panic.md — § 17 "Explicit panic"
func formatPanic(line string) string {
	if !strings.HasPrefix(line, "panic ") {
		return line
	}
	payload := strings.TrimSpace(strings.TrimPrefix(line, "panic"))
	if !strings.HasPrefix(payload, "\"") {
		return line
	}
	return "panic " + payload
}

func matchingParen(s string, open int) int {
	depth, angle := 0, 0
	quote := rune(0)
	esc := false
	for i, r := range s[open:] {
		if quote != 0 {
			if esc {
				esc = false
			} else if r == '\\' {
				esc = true
			} else if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		switch r {
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
	quote := rune(0)
	esc := false
	for i, r := range s {
		if quote != 0 {
			if esc {
				esc = false
			} else if r == '\\' {
				esc = true
			} else if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		switch r {
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
	if quote != 0 || paren != 0 || bracket != 0 || brace != 0 || angle != 0 {
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
	quote := rune(0)
	esc := false
	lastCode := rune(0)
	for i, r := range s {
		if quote != 0 {
			if esc {
				esc = false
			} else if r == '\\' {
				esc = true
			} else if r == quote {
				quote = 0
			}
			continue
		}
		if i+1 < len(s) && r == '/' && s[i+1] == '/' {
			break
		}
		if r == '"' || r == '\'' {
			quote = r
			lastCode = r
			continue
		}
		if !unicode.IsSpace(r) {
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
