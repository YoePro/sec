package main

import (
	"fmt"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	lspserver "sec/internal/lsp/server"
	"sec/internal/sema"
)

type renameParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
	NewName      string                 `json:"newName"`
}

type prepareRenameResult struct {
	Range       lspRange `json:"range"`
	Placeholder string   `json:"placeholder"`
}

// renameTarget is the exact symbol a rename request resolved: the identifier
// under the cursor and its single Sema definition.
type renameTarget struct {
	use        lexer.Token
	definition lexer.Token
	analyzer   *sema.Analyzer
	program    *ast.Program
}

// resolveRenameTarget resolves the symbol under pos semantically and rejects
// what prepare-rename must reject: keywords and non-identifiers, positions
// without exactly one Sema definition, compiler-known members, and symbols
// declared in read-only trusted core sources other than the edited document.
//
// Rules:
//   - rules/tooling/lsp.md — "Rename" (semantic rename; prepare-rename rejections)
func resolveRenameTarget(uri string, text string, pos position, overlay sourceOverlay) (renameTarget, error) {
	use, ok := sourceTokenAtPosition(uri, text, pos)
	if !ok || use.Type != lexer.IDENT {
		return renameTarget{}, fmt.Errorf("only an identifier can be renamed")
	}
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return renameTarget{}, fmt.Errorf("the document cannot be parsed")
	}
	path := pathFromURI(uri)
	prepareProgramForLSP(program, path, overlay)
	analyzer := newLSPAnalyzer(uri, program)
	analyzer.Analyze(program)
	if _, compilerKnown := analyzer.CompilerKnownMemberAt(use.File, use.Line, use.Column); compilerKnown {
		return renameTarget{}, fmt.Errorf("%s is a compiler-known member and cannot be renamed", use.Lexeme)
	}
	definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(use.File, use.Line, use.Column))
	if len(definitions) != 1 {
		return renameTarget{}, fmt.Errorf("%s does not resolve to exactly one declaration", use.Lexeme)
	}
	definition := definitions[0]
	if definition.File != "" && normalizedSourcePath(definition.File) != normalizedSourcePath(path) &&
		program.SourceProvenance[definition.File] == ast.SourceCore {
		return renameTarget{}, fmt.Errorf("%s is declared in read-only trusted core source", use.Lexeme)
	}
	return renameTarget{use: use, definition: definition, analyzer: analyzer, program: program}, nil
}

// prepareRenameForSource answers textDocument/prepareRename with the exact
// identifier range, or an error explaining why the symbol is not renamable.
func prepareRenameForSource(uri string, text string, pos position, overlay sourceOverlay) (prepareRenameResult, error) {
	target, err := resolveRenameTarget(uri, text, pos, overlay)
	if err != nil {
		return prepareRenameResult{}, err
	}
	return prepareRenameResult{Range: tokenRange(text, target.use), Placeholder: target.use.Lexeme}, nil
}

// renameForSource answers textDocument/rename with a workspace edit that
// renames every reference bound to the exact declaration across all source
// files of the analyzed program, so the client can preview a multi-file
// change. The new name must be a non-reserved identifier with the same
// visibility prefix, and the edit is rejected when re-analysis shows it would
// introduce new errors such as a collision, forbidden shadowing, or a
// confusable name.
//
// Rules:
//   - rules/tooling/lsp.md — "Rename"
//   - rules/foundations/names_scopes_visibility.md — visibility prefixes and shadowing
func renameForSource(uri string, text string, pos position, newName string, overlay sourceOverlay) (workspaceEdit, error) {
	target, err := resolveRenameTarget(uri, text, pos, overlay)
	if err != nil {
		return workspaceEdit{}, err
	}
	if err := validateRenameName(target.use.Lexeme, newName); err != nil {
		return workspaceEdit{}, err
	}
	// The edited document's current text is authoritative even when the
	// caller's overlay does not carry it.
	current := sourceOverlay{}
	for key, value := range overlay {
		current[key] = value
	}
	if path := pathFromURI(uri); path != "" {
		current[normalizedSourcePath(path)] = text
	}
	overlay = current
	locations := referencesForSource(uri, text, pos, true, overlay)
	if len(locations) == 0 {
		return workspaceEdit{}, fmt.Errorf("no references of %s were found", target.use.Lexeme)
	}
	changes := map[string][]textEdit{}
	for _, reference := range locations {
		changes[reference.URI] = append(changes[reference.URI], textEdit{Range: reference.Range, NewText: newName})
	}

	// Semantic safety: the renamed program must not gain errors.
	renamedOverlay := sourceOverlay{}
	for key, value := range overlay {
		renamedOverlay[key] = value
	}
	renamedText := text
	for fileURI, edits := range changes {
		path := pathFromURI(fileURI)
		original := text
		if fileURI != uri {
			data, readErr := lspserver.ReadSource(path, overlay)
			if readErr != nil {
				return workspaceEdit{}, fmt.Errorf("cannot read %s: %v", path, readErr)
			}
			original = string(data)
		}
		updated := applyTextEdits(original, edits)
		renamedOverlay[normalizedSourcePath(path)] = updated
		if fileURI == uri {
			renamedText = updated
		}
	}
	// Messages name the symbol, so map the new spelling back before
	// comparing: an error that already existed is not introduced by renaming.
	before := errorDiagnosticMessages(analyze(uri, text, overlay))
	for message := range errorDiagnosticMessages(analyze(uri, renamedText, renamedOverlay)) {
		if !before[strings.ReplaceAll(message, newName, target.use.Lexeme)] {
			return workspaceEdit{}, fmt.Errorf("renaming %s to %s would introduce an error: %s", target.use.Lexeme, newName, firstLine(message))
		}
	}
	return workspaceEdit{Changes: changes}, nil
}

// validateRenameName accepts exactly one non-reserved identifier that keeps
// the visibility prefix (leading underscores) of the old name.
func validateRenameName(oldName string, newName string) error {
	tokens := []lexer.Token{}
	l := lexer.New(newName)
	for token := l.NextToken(); token.Type != lexer.EOF; token = l.NextToken() {
		tokens = append(tokens, token)
	}
	if len(tokens) != 1 || tokens[0].Type != lexer.IDENT || tokens[0].Lexeme != newName {
		return fmt.Errorf("%q is not a valid Sec identifier", newName)
	}
	if lexer.IsReservedDeclarationName(newName) {
		return fmt.Errorf("%q is reserved by the language", newName)
	}
	if visibilityPrefix(oldName) != visibilityPrefix(newName) {
		return fmt.Errorf("renaming %s to %s would change its visibility prefix %q", oldName, newName, visibilityPrefix(oldName))
	}
	return nil
}

func visibilityPrefix(name string) string {
	return name[:len(name)-len(strings.TrimLeft(name, "_"))]
}

// applyTextEdits applies non-overlapping edits from the end of the text so
// earlier offsets stay valid.
func applyTextEdits(text string, edits []textEdit) string {
	type span struct {
		start, end int
		text       string
	}
	spans := make([]span, 0, len(edits))
	for _, edit := range edits {
		spans = append(spans, span{
			start: lineCharToOffset(text, edit.Range.Start.Line, edit.Range.Start.Character),
			end:   lineCharToOffset(text, edit.Range.End.Line, edit.Range.End.Character),
			text:  edit.NewText,
		})
	}
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j].start > spans[j-1].start; j-- {
			spans[j], spans[j-1] = spans[j-1], spans[j]
		}
	}
	for _, edit := range spans {
		if edit.start < 0 || edit.end < edit.start || edit.end > len(text) {
			continue
		}
		text = text[:edit.start] + edit.text + text[edit.end:]
	}
	return text
}

func errorDiagnosticMessages(diagnostics []diagnostic) map[string]bool {
	messages := map[string]bool{}
	for _, reported := range diagnostics {
		if reported.Severity == 1 {
			messages[firstLine(reported.Message)] = true
		}
	}
	return messages
}

func firstLine(message string) string {
	if index := strings.IndexByte(message, '\n'); index >= 0 {
		return message[:index]
	}
	return message
}
