package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

const (
	completionKindProperty = 10
	completionKindValue    = 12
	completionKindKeyword  = 14
)

var (
	attributeNamePrefix = regexp.MustCompile(`^[ \t]*@([A-Za-z_]*)$`)
	attributeOpenCall   = regexp.MustCompile(`@([A-Za-z_]+)\(([^()]*)$`)
	selectorValueSite   = regexp.MustCompile(`^\s*([A-Za-z_]+)\s*:\s*"?([A-Za-z0-9_-]*)$`)
	argumentNameSite    = regexp.MustCompile(`^\s*([A-Za-z_]*)$`)
	usedArgumentName    = regexp.MustCompile(`([A-Za-z_]+)\s*:`)
)

// attributeCompletionItems completes an attribute name after `@` with only
// the compiler-known attributes valid on the declaration that follows, and
// completes @target selector names and known os/arch values and the
// @interrupt vector name inside an argument list. Device, board, cpu,
// interrupt-vector, address, and config values need target knowledge packs
// and compile-time parameters that do not exist yet, so none are invented.
//
// Rules:
//   - rules/foundations/attributes.md — "LSP completion", "Target selectors"
//   - rules/tooling/lsp.md — "Completion"
func attributeCompletionItems(text string, offset int) ([]completionItem, bool) {
	if offset < 0 || offset > len(text) {
		return nil, false
	}
	lineStart := strings.LastIndexAny(text[:offset], "\r\n") + 1
	line := text[lineStart:offset]
	if match := attributeNamePrefix.FindStringSubmatch(line); match != nil {
		target, known := attributeTargetAfter(text, offset)
		items := []completionItem{}
		for _, info := range parser.CompilerKnownAttributes() {
			if known && !info.AllowedOn(target) {
				continue
			}
			if !completionLabelMatches(info.Name, match[1]) {
				continue
			}
			items = append(items, completionItem{Label: info.Name, Kind: completionKindKeyword, Detail: "attribute · " + info.TargetText, Documentation: info.Summary})
		}
		return items, true
	}
	match := attributeOpenCall.FindStringSubmatch(line)
	if match == nil {
		return nil, false
	}
	name, arguments := match[1], match[2]
	info, ok := parser.CompilerKnownAttribute(name)
	if !ok || len(info.NamedArguments) == 0 {
		return []completionItem{}, true
	}
	current := arguments[strings.LastIndex(arguments, ",")+1:]
	if value := selectorValueSite.FindStringSubmatch(current); value != nil {
		if name != "target" {
			return []completionItem{}, true
		}
		items := []completionItem{}
		for _, candidate := range targetSelectorValues(value[1]) {
			if completionLabelMatches(candidate, value[2]) {
				items = append(items, completionItem{Label: candidate, Kind: completionKindValue, Detail: "@target " + value[1]})
			}
		}
		return items, true
	}
	prefix := argumentNameSite.FindStringSubmatch(current)
	if prefix == nil {
		return []completionItem{}, true
	}
	used := map[string]bool{}
	for _, previous := range usedArgumentName.FindAllStringSubmatch(arguments, -1) {
		used[previous[1]] = true
	}
	items := []completionItem{}
	for _, argument := range info.NamedArguments {
		if !used[argument] && completionLabelMatches(argument, prefix[1]) {
			items = append(items, completionItem{Label: argument, Kind: completionKindProperty, Detail: "@" + name + " argument"})
		}
	}
	return items, true
}

// targetSelectorValues returns the known values of one @target selector from
// the canonical target registry shared with the compiler.
func targetSelectorValues(selector string) []string {
	seen := map[string]bool{}
	values := []string{}
	for _, definition := range platformtarget.Definitions() {
		value := ""
		switch selector {
		case "os":
			value = definition.OS
		case "arch":
			value = definition.Arch
		}
		if value == "" || value == "any" || seen[value] {
			continue
		}
		seen[value] = true
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

// attributeTargetAfter classifies the declaration that an attribute written
// at offset would attach to: the first line after it that is not blank, a
// comment, or another attribute. A function inside an open brace is a
// method.
func attributeTargetAfter(text string, offset int) (parser.AttributeTarget, bool) {
	rest := text[offset:]
	if newline := strings.IndexAny(rest, "\r\n"); newline >= 0 {
		rest = rest[newline:]
	} else {
		return "", false
	}
	for _, line := range strings.FieldsFunc(rest, func(r rune) bool { return r == '\n' || r == '\r' }) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "@") {
			continue
		}
		fields := strings.Fields(trimmed)
		word := fields[0]
		if word == "pub" && len(fields) > 1 {
			word = fields[1]
		}
		if word == "unsafe" && len(fields) > 1 {
			word = fields[1]
		}
		switch word {
		case "fn":
			if braceDepth(text[:offset]) > 0 {
				return parser.AttributeTargetMethod, true
			}
			return parser.AttributeTargetFunction, true
		case "extern":
			return parser.AttributeTargetExtern, true
		case "type", "enum":
			return parser.AttributeTargetType, true
		case "let":
			return parser.AttributeTargetLet, true
		}
		return "", false
	}
	return "", false
}

func braceDepth(text string) int {
	depth := 0
	for _, token := range sourceTokens("", text) {
		switch token.Type {
		case lexer.LBRACE:
			depth++
		case lexer.RBRACE:
			depth--
		}
	}
	return depth
}

// attributeHover documents a written attribute: its meaning, allowed
// targets, argument form, status on this declaration, the declaration's
// effective guarantees with their implication sources, and, for a verified
// guarantee on a Sec function, the compiler's panic, allocation, or blocking
// cause path.
//
// Rules:
//   - rules/foundations/attributes.md — "LSP behavior", "Attribute implications", "Sec code versus foreign declarations"
//   - rules/tooling/lsp.md — "Hover"
func attributeHover(text string, program *ast.Program, analyzer *sema.Analyzer, token lexer.Token) (hoverResult, bool) {
	attribute, attributes, function := writtenAttributeAt(program, token)
	if attribute == nil {
		return hoverResult{}, false
	}
	name := attribute.Name.Value
	info, ok := parser.CompilerKnownAttribute(name)
	if !ok {
		return hoverResult{}, false
	}
	lines := []string{"**@" + name + "**", info.Summary, "Allowed on: " + info.TargetText, "Arguments: " + info.Arguments}
	switch {
	case !sema.AttributeImplemented(name):
		lines = append(lines, "Status: not implemented yet; the compiler rejects it rather than accept it without its meaning")
	case function != nil && function.Extern && isGuaranteeAttribute(name):
		lines = append(lines, "Status: trusted foreign contract; the compiler cannot verify the foreign body")
	case isGuaranteeAttribute(name):
		lines = append(lines, "Status: verified by the compiler over every reachable synchronous path")
	}
	if guarantees := sema.EffectiveGuarantees(attributes); len(guarantees) > 0 {
		parts := []string{}
		for _, guarantee := range guarantees {
			part := "`" + string(guarantee.Guarantee) + "`"
			if guarantee.Explicit() {
				part += " (written)"
			} else {
				part += " (implied by @" + strings.Join(guarantee.ImpliedBy, " via @") + ")"
			}
			parts = append(parts, part)
		}
		lines = append(lines, "Effective guarantees: "+strings.Join(parts, ", "))
	}
	if function != nil && !function.Extern && function.Name != nil && analyzer != nil {
		graph := analyzer.CallGraph()
		if nodes := graph.NodesForDeclaration(function.Name.Token); len(nodes) == 1 {
			lines = append(lines, guaranteeCauseLines(graph, nodes[0].ID, attributes)...)
		}
	}
	return hoverResult{Contents: markupContent{Kind: "markdown", Value: strings.Join(lines, "\n\n")}, Range: tokenRange(text, attribute.Name.Token)}, true
}

func isGuaranteeAttribute(name string) bool {
	switch name {
	case "noPanic", "noAlloc", "noBlock", "isr", "interruptSafe":
		return true
	}
	return false
}

// guaranteeCauseLines states, for each effective verified guarantee, whether
// the callable holds it and otherwise the compiler's cause path.
func guaranteeCauseLines(graph *sema.CallGraph, id sema.CallableID, attributes []*ast.Attribute) []string {
	pathText := func(path []sema.CallableID) string {
		names := []string{}
		for _, step := range path {
			if node, ok := graph.Node(step); ok {
				names = append(names, node.Name)
			}
		}
		return "`" + strings.Join(names, "` -> `") + "`"
	}
	lines := []string{}
	for _, guarantee := range sema.EffectiveGuarantees(attributes) {
		switch guarantee.Guarantee {
		case sema.GuaranteeNoPanic:
			if summary := graph.EffectSummary(id); summary.MayPanic {
				lines = append(lines, "May panic: `yes` via "+pathText(summary.PanicPath))
			} else {
				lines = append(lines, "May panic: `no`")
			}
		case sema.GuaranteeNoAlloc:
			summary := graph.ArenaSummary(id)
			switch {
			case summary.MayAllocate:
				lines = append(lines, "May allocate: `yes` via "+pathText(summary.AllocationPath))
			case summary.AllocationUnknown:
				lines = append(lines, "May allocate: `unknown` via "+pathText(summary.UnknownAllocationPath))
			default:
				lines = append(lines, "May allocate: `no`")
			}
		case sema.GuaranteeNoBlock:
			lines = append(lines, blockingHoverLine(graph, id))
		}
	}
	return lines
}

// blockingHoverLine presents the compiler-owned blocking summary.
//
// Rules:
//   - rules/concurrency/blocking.md — "Call graph analysis"
func blockingHoverLine(graph *sema.CallGraph, id sema.CallableID) string {
	summary := graph.BlockSummary(id)
	if !summary.MayBlock {
		return "May block: `no`"
	}
	names := []string{}
	for _, step := range summary.BlockPath {
		if node, ok := graph.Node(step); ok {
			names = append(names, node.Name)
		}
	}
	line := "May block: `yes` via `" + strings.Join(names, "` -> `") + "`"
	if direct := graph.BlockSummary(summary.BlockPath[len(summary.BlockPath)-1]).DirectEffects; len(direct) > 0 {
		cause := string(direct[0].Kind)
		if direct[0].Operation != "" {
			cause = direct[0].Operation
		}
		line += fmt.Sprintf(" (%s at %d:%d)", cause, direct[0].Source.Line, direct[0].Source.Column)
	}
	return line
}

// writtenAttributeAt finds the attribute whose name token is at token, with
// the attachment set it belongs to and its function declaration, if any.
func writtenAttributeAt(program *ast.Program, token lexer.Token) (*ast.Attribute, []*ast.Attribute, *ast.FunctionDeclaration) {
	if program == nil {
		return nil, nil, nil
	}
	match := func(attributes []*ast.Attribute) *ast.Attribute {
		for _, attribute := range attributes {
			if attribute != nil && attribute.Name != nil && attribute.Name.Token.Line == token.Line && attribute.Name.Token.Column == token.Column && attribute.Name.Token.File == token.File {
				return attribute
			}
		}
		return nil
	}
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.FunctionDeclaration:
			if statement != nil {
				if found := match(statement.Attributes); found != nil {
					return found, statement.Attributes, statement
				}
			}
		case *ast.TypeDeclStatement:
			if statement != nil {
				if found := match(statement.Attributes); found != nil {
					return found, statement.Attributes, nil
				}
			}
		case *ast.EnumDeclaration:
			if statement != nil {
				if found := match(statement.Attributes); found != nil {
					return found, statement.Attributes, nil
				}
			}
		case *ast.LetStatement:
			if statement != nil {
				if found := match(statement.Attributes); found != nil {
					return found, statement.Attributes, nil
				}
			}
		case *ast.ImplStatement:
			if statement == nil {
				continue
			}
			for _, member := range statement.Members {
				if function, ok := member.(*ast.FunctionDeclaration); ok && function != nil {
					if found := match(function.Attributes); found != nil {
						return found, function.Attributes, function
					}
				}
			}
		}
	}
	return nil, nil, nil
}

// attributeCodeActions offers source edits for attribute diagnostics: remove
// a duplicate, keep the first of two conflicting values, remove an attribute
// from a declaration it cannot modify, and remove arguments from an
// argument-free attribute. An implied guarantee is never written for the
// user, and a redundant explicit one is never removed.
//
// Rules:
//   - rules/foundations/attributes.md — "Duplicate attributes", "Conflicting attributes", "Redundant attributes", "LSP behavior"
//   - rules/tooling/lsp.md — "Code actions"
func attributeCodeActions(uri string, text string, reported []diagnostic) []codeAction {
	actions := []codeAction{}
	for _, reportedDiagnostic := range reported {
		start := lineCharToOffset(text, reportedDiagnostic.Range.Start.Line, reportedDiagnostic.Range.Start.Character)
		var title string
		var edit textEdit
		ok := false
		switch reportedDiagnostic.Code {
		case diagnostics.AttributeDuplicate:
			edit, title, ok = removeAttributeEdit(text, start, "Remove duplicate @%s")
		case diagnostics.AttributeConflict:
			edit, title, ok = removeAttributeEdit(text, start, "Keep the first @%s and remove this one")
		case diagnostics.AttributeNotAllowedOnTarget:
			edit, title, ok = removeAttributeEdit(text, start, "Remove @%s")
		case diagnostics.AttributeInvalidArgument:
			if strings.Contains(reportedDiagnostic.Message, "does not take arguments") {
				edit, title, ok = removeArgumentsEdit(text, start)
			}
		}
		if !ok {
			continue
		}
		actions = append(actions, codeAction{
			Title:       title,
			Kind:        "quickfix",
			Diagnostics: []diagnostic{reportedDiagnostic},
			Edit:        workspaceEdit{Changes: map[string][]textEdit{uri: {edit}}},
		})
	}
	return actions
}

// removeAttributeEdit removes the attribute starting at the `@` at start,
// with its argument list, and its line when nothing else remains on it.
func removeAttributeEdit(text string, start int, titleFormat string) (textEdit, string, bool) {
	if start < 0 || start >= len(text) || text[start] != '@' {
		return textEdit{}, "", false
	}
	end := start + 1
	for end < len(text) && (text[end] == '_' || isASCIIAlphaNumeric(text[end])) {
		end++
	}
	name := text[start+1 : end]
	if name == "" {
		return textEdit{}, "", false
	}
	if end < len(text) && text[end] == '(' {
		closing, ok := matchingParenthesis(text, end)
		if !ok {
			return textEdit{}, "", false
		}
		end = closing + 1
	}
	lineStart := strings.LastIndexAny(text[:start], "\r\n") + 1
	lineEnd := len(text)
	if newline := strings.IndexAny(text[end:], "\r\n"); newline >= 0 {
		lineEnd = end + newline
	}
	if strings.TrimSpace(text[lineStart:start]) == "" && strings.TrimSpace(text[end:lineEnd]) == "" {
		start, end = lineStart, lineEnd
		if end < len(text) && text[end] == '\r' {
			end++
		}
		if end < len(text) && text[end] == '\n' {
			end++
		}
	}
	return textEdit{Range: offsetsRange(text, start, end), NewText: ""}, fmt.Sprintf(titleFormat, name), true
}

// removeArgumentsEdit removes the argument list whose `(` is at start.
func removeArgumentsEdit(text string, start int) (textEdit, string, bool) {
	if start < 0 || start >= len(text) || text[start] != '(' {
		return textEdit{}, "", false
	}
	closing, ok := matchingParenthesis(text, start)
	if !ok {
		return textEdit{}, "", false
	}
	nameStart := start
	for nameStart > 0 && (text[nameStart-1] == '_' || isASCIIAlphaNumeric(text[nameStart-1])) {
		nameStart--
	}
	return textEdit{Range: offsetsRange(text, start, closing+1), NewText: ""}, "Remove the arguments of @" + text[nameStart:start], true
}

func matchingParenthesis(text string, open int) (int, bool) {
	depth := 0
	inString := false
	for index := open; index < len(text); index++ {
		switch character := text[index]; {
		case inString && character == '\\':
			index++
		case character == '"':
			inString = !inString
		case inString:
		case character == '(':
			depth++
		case character == ')':
			depth--
			if depth == 0 {
				return index, true
			}
		}
	}
	return 0, false
}

func isASCIIAlphaNumeric(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9'
}
