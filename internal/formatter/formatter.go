// Package formatter provides the canonical, reusable Sec source formatter.
package formatter

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type Options struct{ Fix bool }
type Source struct{ Text string }
type Result struct{ Text string }

func Format(source Source, options Options) Result { return Result{Text: format(source.Text, options)} }

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
	lines := strings.Split(normal, "\n")
	hadFinal := strings.HasSuffix(normal, "\n")
	if hadFinal {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, len(lines))
	indent := 0
	blank := false
	branches := []branch{}
	for _, line := range lines {
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
		}
		line = formatUnitExpressions(formatSingleLineCallSpacing(formatMatchArm(formatLet(formatSignature(formatInitSignature(normalizeFunc(line)))))))
		level := indent - closing(line)
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
		delta := delimiters(line)
		indent += delta
		if indent < 0 {
			indent = 0
		}
		if branchStart(line) && delta > 0 {
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

func trailingLineCommentIndex(line string) int {
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

// formatUnitExpressions implements rules/types/units.md and
// rules/tooling/formatter.md compact structural-unit spacing. It only edits a
// balanced type-annotation angle group and never reorders factors or removes
// source grouping.
func formatUnitExpressions(line string) string {
	if strings.Contains(line, "//") || strings.Contains(line, "/*") || strings.Contains(line, "*/") {
		return line
	}
	var out strings.Builder
	for cursor := 0; cursor < len(line); {
		open := strings.IndexByte(line[cursor:], '<')
		if open < 0 {
			out.WriteString(line[cursor:])
			break
		}
		open += cursor
		close := strings.IndexByte(line[open+1:], '>')
		if close < 0 {
			out.WriteString(line[cursor:])
			break
		}
		close += open + 1
		content := line[open+1 : close]
		if !strings.ContainsAny(content, "*/^()") || !looksLikeUnitAnnotation(line, open) {
			out.WriteString(line[cursor : open+1])
			cursor = open + 1
			continue
		}
		out.WriteString(line[cursor : open+1])
		out.WriteString(compactUnitOperators(content))
		out.WriteByte('>')
		cursor = close + 1
	}
	return out.String()
}

func looksLikeUnitAnnotation(line string, open int) bool {
	spaced := open > 0 && unicode.IsSpace(rune(line[open-1]))
	for i := open - 1; i >= 0; i-- {
		if unicode.IsSpace(rune(line[i])) {
			continue
		}
		if spaced {
			// A whitespace-led '<' after an operand is a comparison, not a
			// unit-only type annotation.
			return line[i] == ':' || line[i] == '[' || line[i] == ',' || line[i] == '(' || line[i] == '='
		}
		return line[i] == '_' || line[i] == ']' || unicode.IsLetter(rune(line[i])) || unicode.IsDigit(rune(line[i]))
	}
	return true
}

func compactUnitOperators(content string) string {
	fields := strings.Fields(content)
	compact := strings.Join(fields, " ")
	for _, operator := range []string{"*", "/", "^", "(", ")"} {
		compact = strings.ReplaceAll(compact, " "+operator, operator)
		compact = strings.ReplaceAll(compact, operator+" ", operator)
	}
	return compact
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

func formatMatchArm(line string) string {
	arrow := strings.Index(line, "=>")
	if arrow < 0 {
		return line
	}
	left := strings.TrimSpace(line[:arrow])
	right := strings.TrimSpace(line[arrow+2:])
	guard := ""
	if where := strings.Index(left, " where "); where >= 0 {
		guard = " where " + strings.TrimSpace(left[where+7:])
		left = strings.TrimSpace(left[:where])
	}
	pattern, ok := normalizeCanonicalMatchPattern(left)
	if !ok {
		return line
	}
	if right == "" {
		return pattern + guard + " =>"
	}
	return pattern + guard + " => " + right
}

func normalizeCanonicalMatchPattern(text string) (string, bool) {
	text = strings.TrimSpace(text)
	if text == "_" || text == "empty" {
		return text, true
	}
	if open := strings.Index(text, "{"); open >= 0 {
		if !strings.HasSuffix(text, "}") {
			return "", false
		}
		name := strings.TrimSpace(text[:open])
		if !qualifiedIdentifier(name) {
			return "", false
		}
		parts := split(text[open+1 : len(text)-1])
		if parts == nil {
			return "", false
		}
		fields := make([]string, 0, len(parts))
		for _, part := range parts {
			field := strings.TrimSpace(part)
			colon := strings.Index(field, ":")
			if colon < 0 {
				if !ident(field) {
					return "", false
				}
				fields = append(fields, field)
				continue
			}
			fieldName := strings.TrimSpace(field[:colon])
			binding, ok := normalizeMatchBinding(field[colon+1:])
			if !ident(fieldName) || !ok {
				return "", false
			}
			fields = append(fields, fieldName+": "+binding)
		}
		return name + " { " + strings.Join(fields, ", ") + " }", true
	}
	if open := strings.Index(text, "("); open >= 0 {
		if !strings.HasSuffix(text, ")") {
			return "", false
		}
		name := strings.TrimSpace(text[:open])
		binding, ok := normalizeMatchBinding(text[open+1 : len(text)-1])
		if !qualifiedIdentifier(name) || !ok {
			return "", false
		}
		return name + "(" + binding + ")", true
	}
	return text, qualifiedIdentifier(text)
}

func normalizeMatchBinding(text string) (string, bool) {
	parts := strings.Fields(text)
	switch {
	case len(parts) == 1 && (parts[0] == "_" || ident(parts[0])):
		return parts[0], true
	case len(parts) == 2 && parts[0] == "ref" && ident(parts[1]):
		return "ref " + parts[1], true
	case len(parts) == 3 && parts[0] == "ref" && parts[1] == "mut" && ident(parts[2]):
		return "ref mut " + parts[2], true
	default:
		return "", false
	}
}

func qualifiedIdentifier(text string) bool {
	parts := strings.Split(text, ".")
	if len(parts) == 0 {
		return false
	}
	for _, part := range parts {
		if !ident(part) {
			return false
		}
	}
	return true
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
func formatSignature(line string) string {
	prefix := "fn "
	for _, candidate := range []string{"mut fn ", "-> fn ", "static fn "} {
		if strings.HasPrefix(line, candidate) {
			prefix = candidate
			break
		}
	}
	if !strings.HasPrefix(line, prefix) {
		return line
	}
	open := strings.Index(line, "(")
	if open < 0 {
		return line
	}
	close := matchingParen(line, open)
	if close < 0 {
		return line
	}
	parts := split(line[open+1 : close])
	if parts == nil {
		return line
	}
	return line[:open+1] + strings.Join(parts, ", ") + line[close:]
}

func formatInitSignature(line string) string {
	if !strings.HasPrefix(line, "init(") && !strings.HasPrefix(line, "init (") {
		return line
	}
	open := strings.Index(line, "(")
	if open < 0 {
		return line
	}
	close := matchingParen(line, open)
	if close < 0 {
		return line
	}
	parts := split(line[open+1 : close])
	if parts == nil {
		return line
	}
	return "init(" + strings.Join(parts, ", ") + line[close:]
}
func formatLet(line string) string {
	if !strings.HasPrefix(line, "let ") || strings.Contains(line, "//") || strings.Contains(line, "/*") {
		return line
	}
	parts := split(line[4:])
	if len(parts) < 2 {
		return line
	}
	for _, p := range parts {
		if !strings.Contains(p, ":=") {
			return line
		}
	}
	return "let " + strings.Join(parts, ", ")
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
	start, paren, bracket, angle := 0, 0, 0, 0
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
		case '<':
			angle++
		case '>':
			if angle > 0 {
				angle--
			}
		case ',':
			if paren == 0 && bracket == 0 && angle == 0 {
				if p := strings.TrimSpace(s[start:i]); p != "" {
					parts = append(parts, p)
				}
				start = i + 1
			}
		}
	}
	if quote != 0 || paren != 0 || bracket != 0 || angle != 0 {
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
