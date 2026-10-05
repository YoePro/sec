package formatter

import (
	"reflect"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/operators"
	"sec/internal/parser"
)

type infixLayoutOperator struct {
	token    lexer.Token
	spelling string
}

// formatMultilineInfixLayout lays out a let or assignment value whose infix
// chain is already broken with every operand on its own line in the canonical
// leading-operator form: the value starts on the line after the declaration
// or assignment operator, operands share one stable column two spaces past
// the continuation indentation, and each operator is right-aligned in front of
// its operand. Source with comments, multiline operands, operands sharing a
// line, or a parse error is left unchanged.
//
// Rules:
//   - rules/tooling/formatter.md — § 9(13)–(15) multiline infix expressions, § 6(3) continuation indentation
func formatMultilineInfixLayout(text string, indentationWidth int) string {
	if !strings.Contains(text, "\n") {
		return text
	}
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 || program == nil {
		return text
	}
	replacements := []formatterReplacement{}
	var visit func(value reflect.Value)
	visit = func(value reflect.Value) {
		if !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.Interface:
			if !value.IsNil() {
				visit(value.Elem())
			}
		case reflect.Pointer:
			if value.IsNil() || value.Type().Elem().PkgPath() != "sec/internal/ast" {
				return
			}
			switch node := value.Interface().(type) {
			case *ast.LetStatement:
				if replacement, ok := valueLayoutReplacement(text, node.Token, node.Value, indentationWidth); ok {
					replacements = append(replacements, replacement)
					return
				}
			case *ast.AssignmentStatement:
				if node.Operator == "=" {
					if replacement, ok := valueLayoutReplacement(text, statementStartToken(node), node.Value, indentationWidth); ok {
						replacements = append(replacements, replacement)
						return
					}
				}
			}
			visit(value.Elem())
		case reflect.Struct:
			for index := 0; index < value.NumField(); index++ {
				if value.Type().Field(index).IsExported() {
					visit(value.Field(index))
				}
			}
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				visit(value.Index(index))
			}
		}
	}
	for _, statement := range program.Statements {
		visit(reflect.ValueOf(statement))
	}
	sort.Slice(replacements, func(i, j int) bool { return replacements[i].start > replacements[j].start })
	for _, replacement := range replacements {
		text = text[:replacement.start] + replacement.text + text[replacement.end:]
	}
	return text
}

// statementStartToken returns the first token of an assignment statement.
func statementStartToken(statement *ast.AssignmentStatement) lexer.Token {
	token := statement.Token
	if identifier, ok := statement.Target.(*ast.Identifier); ok && identifier.Token.ByteStart < token.ByteStart {
		token = identifier.Token
	}
	return token
}

// infixLayoutReplacement computes the canonical layout of one broken value.
func infixLayoutReplacement(text string, start lexer.Token, value ast.Expression, indentationWidth int) (formatterReplacement, bool) {
	root, ok := value.(*ast.InfixExpression)
	if !ok {
		return formatterReplacement{}, false
	}
	level, ok := operators.BinaryPrecedence(root.Operator)
	if !ok {
		return formatterReplacement{}, false
	}
	operatorTokens := []infixLayoutOperator{}
	for node := ast.Expression(root); ; {
		infix, isInfix := node.(*ast.InfixExpression)
		if !isInfix {
			break
		}
		candidate, known := operators.BinaryPrecedence(infix.Operator)
		if !known || candidate != level {
			break
		}
		operatorTokens = append([]infixLayoutOperator{{token: infix.Token, spelling: infix.Operator}}, operatorTokens...)
		node = infix.Left
	}
	header, headerOperatorEnd, ok := valueHeader(text, start, operatorTokens[0].token.ByteStart)
	if !ok {
		return formatterReplacement{}, false
	}
	if headerOperatorEnd > operatorTokens[0].token.ByteStart {
		return formatterReplacement{}, false
	}
	// The last operand starts at the first non-space byte after the last
	// operator, possibly on the next line, and ends with its line.
	lastStart := infixLayoutOperatorEnd(text, operatorTokens[len(operatorTokens)-1])
	for lastStart < len(text) && (text[lastStart] == ' ' || text[lastStart] == '\t' || text[lastStart] == '\n' || text[lastStart] == '\r') {
		lastStart++
	}
	lastEnd := strings.IndexByte(text[lastStart:], '\n')
	if lastEnd < 0 {
		lastEnd = len(text)
	} else {
		lastEnd += lastStart
	}
	boundaries := []int{headerOperatorEnd}
	for _, operator := range operatorTokens {
		boundaries = append(boundaries, operator.token.ByteStart, infixLayoutOperatorEnd(text, operator))
	}
	boundaries = append(boundaries, lastEnd)
	operands := []string{}
	contentStarts, contentEnds := []int{}, []int{}
	for index := 0; index+1 < len(boundaries); index += 2 {
		segment := text[boundaries[index]:boundaries[index+1]]
		if strings.Contains(segment, "//") || strings.Contains(segment, "/*") {
			return formatterReplacement{}, false
		}
		operand := strings.TrimSpace(segment)
		if operand == "" || strings.ContainsAny(operand, "\n\r") {
			return formatterReplacement{}, false
		}
		leading := len(segment) - len(strings.TrimLeft(segment, " \t\r\n"))
		contentStarts = append(contentStarts, boundaries[index]+leading)
		contentEnds = append(contentEnds, boundaries[index]+leading+len(operand))
		operands = append(operands, operand)
	}
	// Every operand must already sit on its own line: the gap between two
	// consecutive operands, which holds their operator, crosses exactly one
	// line break.
	for index := range operatorTokens {
		if strings.Count(text[contentEnds[index]:contentStarts[index+1]], "\n") != 1 {
			return formatterReplacement{}, false
		}
	}
	indent := header[:len(header)-len(strings.TrimLeft(header, " "))]
	operandColumn := len(indent) + indentationWidth + 2
	var out strings.Builder
	out.WriteString("\n")
	out.WriteString(strings.Repeat(" ", operandColumn))
	out.WriteString(operands[0])
	for index, operator := range operatorTokens {
		padding := operandColumn - 1 - len(operator.spelling)
		if padding < len(indent) {
			return formatterReplacement{}, false
		}
		out.WriteString("\n")
		out.WriteString(strings.Repeat(" ", padding))
		out.WriteString(operator.spelling)
		out.WriteString(" ")
		out.WriteString(operands[index+1])
	}
	return formatterReplacement{start: headerOperatorEnd, end: lastEnd, text: out.String()}, true
}

// infixLayoutOperatorEnd returns the complete source extent of a binary
// operator. Most operators are one lexer token; contextual `not in` spans the
// parser-owned `not` token, separating trivia, and the following `in` token.
// Token spacing has already canonicalized that trivia before this pass.
//
// Rules:
//   - rules/foundations/operators.md — "Membership operators `in` and `not in`"
//   - rules/tooling/formatter.md — § 9(13)–(15) multiline infix expressions
func infixLayoutOperatorEnd(text string, operator infixLayoutOperator) int {
	end := operator.token.ByteEnd
	if operator.spelling == operator.token.Lexeme || !strings.HasPrefix(operator.spelling, operator.token.Lexeme) {
		return end
	}
	remainder := strings.TrimSpace(operator.spelling[len(operator.token.Lexeme):])
	for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
		end++
	}
	if remainder != "" && strings.HasPrefix(text[end:], remainder) {
		end += len(remainder)
	}
	return end
}

// valueHeader returns the declaration or assignment header line that starts
// at start and the byte offset just past its `:=` or `=` operator, which must
// precede valueStart.
func valueHeader(text string, start lexer.Token, valueStart int) (string, int, bool) {
	if start.ByteEnd == 0 || start.ByteStart >= valueStart {
		return "", 0, false
	}
	lineStart := strings.LastIndexByte(text[:start.ByteStart], '\n') + 1
	headerLineEnd := strings.IndexByte(text[start.ByteStart:], '\n')
	if headerLineEnd < 0 {
		return "", 0, false
	}
	header := text[lineStart : start.ByteStart+headerLineEnd]
	limit := valueStart - lineStart
	if limit > len(header) {
		limit = len(header)
	}
	for index := 0; index < limit; index++ {
		switch {
		case strings.HasPrefix(header[index:], ":="):
			return header, lineStart + index + 2, true
		case header[index] == '=' && (index+1 >= len(header) || header[index+1] != '=') &&
			(index == 0 || !strings.ContainsRune("=!<>:", rune(header[index-1]))):
			return header, lineStart + index + 1, true
		}
	}
	return "", 0, false
}

// chainLayoutReplacement lays out a let or assignment value that is a member
// or call chain already broken across lines: the receiver moves to the
// continuation line after the header operator and every chain segment
// starts with its leading dot on its own line one level deeper. Trailing
// dots become leading dots, and segments that shared a line of an already
// multiline chain get their own line (§ 17(7)). A chain with a segment that
// itself spans several lines, a blank line, or comments is left unchanged.
//
// Rules:
//   - rules/tooling/formatter.md — § 17(6)–(7) multiline member chains, § 6(3)
func chainLayoutReplacement(text string, start lexer.Token, value ast.Expression, indentationWidth int) (formatterReplacement, bool) {
	dots := []lexer.Token{}
	for node := value; ; {
		switch current := node.(type) {
		case *ast.CallExpression:
			member, ok := current.Callee.(*ast.MemberExpression)
			if !ok {
				node = nil
				break
			}
			dots = append([]lexer.Token{member.Token}, dots...)
			node = member.Object
			continue
		case *ast.MemberExpression:
			dots = append([]lexer.Token{current.Token}, dots...)
			node = current.Object
			continue
		}
		break
	}
	if len(dots) == 0 {
		return formatterReplacement{}, false
	}
	header, headerOperatorEnd, ok := valueHeader(text, start, dots[0].ByteStart)
	if !ok {
		return formatterReplacement{}, false
	}
	lastStart := dots[len(dots)-1].ByteEnd
	for lastStart < len(text) && strings.ContainsRune(" \t\r\n", rune(text[lastStart])) {
		lastStart++
	}
	lastEnd := strings.IndexByte(text[lastStart:], '\n')
	if lastEnd < 0 {
		lastEnd = len(text)
	} else {
		lastEnd += lastStart
	}
	region := text[headerOperatorEnd:lastEnd]
	if strings.Contains(region, "//") || strings.Contains(region, "/*") || !strings.Contains(region, "\n") {
		return formatterReplacement{}, false
	}
	receiver := strings.TrimSpace(text[headerOperatorEnd:dots[0].ByteStart])
	if receiver == "" || strings.ContainsAny(receiver, "\n\r") {
		return formatterReplacement{}, false
	}
	segments := []string{}
	previousContentEnd := headerOperatorEnd + strings.Index(text[headerOperatorEnd:], receiver) + len(receiver)
	for index, dot := range dots {
		end := lastEnd
		if index+1 < len(dots) {
			end = dots[index+1].ByteStart
		}
		rest := strings.TrimSpace(text[dot.ByteEnd:end])
		if rest == "" || strings.ContainsAny(rest, "\n\r") {
			return formatterReplacement{}, false
		}
		// At most one line break separates consecutive segments; a blank line
		// inside a chain is not a layout this pass rewrites.
		restStart := dot.ByteEnd + strings.Index(text[dot.ByteEnd:end], rest)
		if strings.Count(text[previousContentEnd:restStart], "\n") > 1 {
			return formatterReplacement{}, false
		}
		segments = append(segments, "."+rest)
		previousContentEnd = restStart + len(rest)
	}
	indent := header[:len(header)-len(strings.TrimLeft(header, " "))]
	receiverIndent := strings.Repeat(" ", len(indent)+indentationWidth)
	segmentIndent := strings.Repeat(" ", len(indent)+2*indentationWidth)
	var out strings.Builder
	out.WriteString("\n" + receiverIndent + receiver)
	for _, segment := range segments {
		out.WriteString("\n" + segmentIndent + segment)
	}
	return formatterReplacement{start: headerOperatorEnd, end: lastEnd, text: out.String()}, true
}

// valueLayoutReplacement applies the multiline infix or member-chain layout.
func valueLayoutReplacement(text string, start lexer.Token, value ast.Expression, indentationWidth int) (formatterReplacement, bool) {
	if replacement, ok := infixLayoutReplacement(text, start, value, indentationWidth); ok {
		return replacement, true
	}
	return chainLayoutReplacement(text, start, value, indentationWidth)
}
