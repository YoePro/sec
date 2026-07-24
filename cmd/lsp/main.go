package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
	"sec/internal/sema"
)

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type textDocumentItem struct {
	URI  string `json:"uri"`
	Text string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type versionedTextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type textDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	CompletionProvider CompletionOptions `json:"completionProvider"`
}

type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}

type didChangeParams struct {
	TextDocument   versionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []textDocumentContentChangeEvent `json:"contentChanges"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type formattingParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

type diagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity"`
	Source   string   `json:"source"`
	Message  string   `json:"message"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type server struct {
	in               *bufio.Reader
	out              io.Writer
	documents        map[string]string
	diagnosticTimers map[string]*time.Timer
	diagnosticDelay  time.Duration
	writeMu          sync.Mutex
	timerMu          sync.Mutex
	shutdown         bool
}

type formatterSwitchContext struct {
	contentDepth int
	caseActive   bool
}

func main() {
	s := &server{
		in:               bufio.NewReader(os.Stdin),
		out:              os.Stdout,
		documents:        map[string]string{},
		diagnosticTimers: map[string]*time.Timer{},
		diagnosticDelay:  600 * time.Millisecond,
	}
	if err := s.run(); err != nil {
		fmt.Fprintf(os.Stderr, "lsp error: %v\n", err)
		os.Exit(1)
	}
}

func (s *server) run() error {
	for {
		message, err := readMessage(s.in)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := s.handle(message); err != nil {
			return err
		}
	}
}

func (s *server) handle(message rpcMessage) error {
	switch message.Method {
	case "initialize":
		return s.respond(message.ID, map[string]any{
			"capabilities": map[string]any{
				"textDocumentSync":           1,
				"documentFormattingProvider": true,
			},
			"serverInfo": map[string]any{
				"name":    "sec-lsp",
				"version": "0.1.0",
			},
		})
	case "initialized":
		return nil
	case "shutdown":
		s.shutdown = true
		s.stopDiagnosticTimers()
		return s.respond(message.ID, nil)
	case "exit":
		if s.shutdown {
			os.Exit(0)
		}
		os.Exit(1)
		return nil
	case "textDocument/didOpen":
		var params didOpenParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		s.documents[params.TextDocument.URI] = params.TextDocument.Text
		return s.publishDiagnostics(params.TextDocument.URI, params.TextDocument.Text)
	case "textDocument/didChange":
		var params didChangeParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		if len(params.ContentChanges) == 0 {
			return nil
		}
		text := params.ContentChanges[len(params.ContentChanges)-1].Text
		s.documents[params.TextDocument.URI] = text
		s.scheduleDiagnostics(params.TextDocument.URI, text)
		return nil
	case "textDocument/formatting":
		var params formattingParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		edits, err := s.formatDocument(params.TextDocument.URI)
		if err != nil {
			return s.respondError(message.ID, -32603, err.Error())
		}
		return s.respond(message.ID, edits)
	default:
		if len(message.ID) == 0 {
			return nil
		}
		return s.respondError(message.ID, -32601, "method not found")
	}
}

func (s *server) scheduleDiagnostics(uri string, text string) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()

	if timer := s.diagnosticTimers[uri]; timer != nil {
		timer.Stop()
	}
	s.diagnosticTimers[uri] = time.AfterFunc(s.diagnosticDelay, func() {
		s.timerMu.Lock()
		delete(s.diagnosticTimers, uri)
		s.timerMu.Unlock()
		_ = s.publishDiagnostics(uri, text)
	})
}

func (s *server) stopDiagnosticTimers() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()

	for uri, timer := range s.diagnosticTimers {
		timer.Stop()
		delete(s.diagnosticTimers, uri)
	}
}

func (s *server) publishDiagnostics(uri string, text string) error {
	diagnostics := analyze(uri, text)
	return s.notify("textDocument/publishDiagnostics", map[string]any{
		"uri":         uri,
		"diagnostics": diagnostics,
	})
}

func (s *server) formatDocument(uri string) ([]textEdit, error) {
	text, ok := s.documents[uri]
	if !ok {
		data, err := os.ReadFile(pathFromURI(uri))
		if err != nil {
			return nil, err
		}
		text = string(data)
	}

	formatted := formatSource(text)
	if formatted == text {
		return []textEdit{}, nil
	}

	return []textEdit{
		{
			Range: lspRange{
				Start: position{Line: 0, Character: 0},
				End:   endPosition(text),
			},
			NewText: formatted,
		},
	}, nil
}

func formatSource(text string) string {
	lineEnding := "\n"
	if strings.Contains(text, "\r\n") {
		lineEnding = "\r\n"
	}

	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	hadFinalNewline := strings.HasSuffix(normalized, "\n")
	if hadFinalNewline && len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}

	out := make([]string, 0, len(lines))
	indent := 0
	blankPending := false
	switches := []formatterSwitchContext{}
	inImportGroup := false

	for _, line := range lines {
		line = strings.ReplaceAll(line, "\t", "    ")
		trimmedRight := strings.TrimRight(line, " \t")
		trimmed := strings.TrimSpace(trimmedRight)

		if trimmed == "" {
			if len(out) > 0 {
				blankPending = true
			}
			continue
		}

		lineIndent := indent
		if startsWithClosingBlock(trimmed) && lineIndent > 0 {
			lineIndent--
		}

		for len(switches) > 0 && lineIndent < switches[len(switches)-1].contentDepth {
			switches = switches[:len(switches)-1]
		}

		caseContext := -1
		if isSwitchCaseClause(trimmed) {
			for i := len(switches) - 1; i >= 0; i-- {
				if switches[i].contentDepth == lineIndent {
					caseContext = i
					break
				}
			}
		}

		extraIndent := 0
		if inImportGroup && trimmed != ")" {
			extraIndent++
		}
		for i, switchContext := range switches {
			if switchContext.caseActive && lineIndent >= switchContext.contentDepth {
				if i == caseContext {
					continue
				}
				extraIndent++
			}
		}

		if blankPending && len(out) > 0 {
			out = append(out, "")
			blankPending = false
		}

		out = append(out, strings.Repeat(" ", (lineIndent+extraIndent)*4)+trimmed)
		indentDelta := braceIndentDelta(trimmed)
		indent += indentDelta
		if indent < 0 {
			indent = 0
		}
		if isSwitchBlockStart(trimmed) && indentDelta > 0 {
			switches = append(switches, formatterSwitchContext{contentDepth: indent})
		}
		if trimmed == "import (" {
			inImportGroup = true
		} else if inImportGroup && trimmed == ")" {
			inImportGroup = false
		}
		if caseContext >= 0 {
			switches[caseContext].caseActive = true
		}
	}

	formatted := strings.Join(out, "\n")
	if hadFinalNewline || formatted != "" {
		formatted += "\n"
	}
	if lineEnding != "\n" {
		formatted = strings.ReplaceAll(formatted, "\n", lineEnding)
	}
	return formatted
}

func isSwitchBlockStart(line string) bool {
	return strings.HasPrefix(line, "switch ") || strings.HasPrefix(line, "switch{")
}

func isSwitchCaseClause(line string) bool {
	return line == "case" ||
		strings.HasPrefix(line, "case ") ||
		strings.HasPrefix(line, "case\t") ||
		line == "default:" ||
		strings.HasPrefix(line, "default ")
}

func startsWithClosingBlock(line string) bool {
	return strings.HasPrefix(line, "}") || strings.HasPrefix(line, "]")
}

func braceIndentDelta(line string) int {
	delta := 0
	inString := false
	escaped := false
	inLineComment := false
	for i, r := range line {
		if inLineComment {
			break
		}
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inString = false
			}
			continue
		}
		if r == '/' && i+1 < len(line) && line[i+1] == '/' {
			inLineComment = true
			continue
		}
		if r == '"' {
			inString = true
			continue
		}
		switch r {
		case '{':
			delta++
		case '}':
			delta--
		}
	}
	return delta
}

func endPosition(text string) position {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) == 0 {
		return position{}
	}
	return position{Line: len(lines) - 1, Character: len([]rune(lines[len(lines)-1]))}
}

func analyze(uri string, text string) []diagnostic {
	path := pathFromURI(uri)
	l := lexer.NewWithFile(text, path)
	p := parser.New(l)
	program := p.ParseProgram()

	diagnostics := []diagnostic{}
	for _, err := range p.Errors() {
		diagnostics = append(diagnostics, parserDiagnostic(err))
	}
	if len(p.Errors()) > 0 {
		return diagnostics
	}
	resolveSourceImports(program, map[string]bool{}, path)

	analyzer := sema.NewAnalyzer()
	for _, err := range analyzer.Analyze(program) {
		diagnostics = append(diagnostics, semaDiagnostic(err, 1))
	}
	for _, warning := range analyzer.Warnings() {
		diagnostics = append(diagnostics, semaDiagnostic(warning, 2))
	}
	return diagnostics
}

func resolveSourceImports(program *ast.Program, seen map[string]bool, sourceFile string) {
	for _, stmt := range append([]ast.Statement{}, program.Statements...) {
		importStmt, ok := stmt.(*ast.ImportStatement)
		if !ok {
			continue
		}
		sourcePaths := sourceIncludePaths(importStmt.Path, sourceFile)
		importedStatements := []ast.Statement{}
		module := ""
		for _, sourcePath := range sourcePaths {
			if seen[sourcePath] {
				if imported, parsed := parseSourceInclude(sourcePath); parsed {
					rewriteImportQualifier(program, importQualifier(importStmt), programModulePath(imported))
				}
				continue
			}
			seen[sourcePath] = true

			imported, ok := parseSourceInclude(sourcePath)
			if !ok {
				continue
			}
			resolveSourceImports(imported, seen, sourcePath)
			if module == "" {
				module = programModulePath(imported)
			}
			importedStatements = append(importedStatements, imported.Statements...)
		}
		if module == "" || len(importedStatements) == 0 {
			continue
		}
		rewriteImportQualifier(program, importQualifier(importStmt), module)
		imported := &ast.Program{Statements: importedStatements}
		qualifyImportedModule(imported, module)
		program.Statements = append(program.Statements, imported.Statements...)
	}
}

func sourceIncludePaths(path string, sourceFile string) []string {
	root := findSecSourceRoot(sourceFile)
	if strings.HasPrefix(path, "platform/") {
		trimmed := strings.Trim(strings.TrimSuffix(path, ".sec"), "/")
		base := filepath.Join(root, "sec", filepath.FromSlash(trimmed))
		if info, err := os.Stat(base); err == nil && info.IsDir() {
			matches, globErr := filepath.Glob(filepath.Join(base, "*.sec"))
			if globErr != nil {
				return nil
			}
			sort.Strings(matches)
			return matches
		}
	}
	relative, ok := sourceIncludePath(path)
	if !ok {
		return nil
	}
	primary := filepath.Clean(filepath.Join(root, relative))

	module := strings.TrimPrefix(path, "std/")
	if module != "fmt" {
		return []string{primary}
	}

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(primary), "*.sec"))
	if err != nil || len(matches) == 0 {
		return []string{primary}
	}
	sort.Strings(matches)
	out := []string{primary}
	for _, match := range matches {
		match = filepath.Clean(match)
		if match != primary {
			out = append(out, match)
		}
	}
	return out
}

func findSecSourceRoot(sourceFile string) string {
	starts := []string{}
	if sourceFile != "" {
		starts = append(starts, filepath.Dir(sourceFile))
	}
	if cwd, err := os.Getwd(); err == nil {
		starts = append(starts, cwd)
	}
	if executable, err := os.Executable(); err == nil {
		starts = append(starts, filepath.Dir(executable))
	}
	for _, start := range starts {
		for current := filepath.Clean(start); ; current = filepath.Dir(current) {
			if info, err := os.Stat(filepath.Join(current, "sec", "stdlib")); err == nil && info.IsDir() {
				return current
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
		}
	}
	return "."
}

func sourceIncludePath(path string) (string, bool) {
	module := strings.TrimPrefix(path, "std/")
	switch module {
	case "fmt":
		return filepath.Join("sec", "stdlib", "fmt", "fmt.sec"), true
	case "io":
		return filepath.Join("sec", "stdlib", "io", "write.linux.amd64.sec"), true
	}
	if strings.HasPrefix(path, "platform/") {
		trimmed := strings.Trim(path, "/")
		trimmed = strings.TrimSuffix(trimmed, ".sec")
		return filepath.Join("sec", trimmed+".sec"), true
	}
	return "", false
}

func importQualifier(stmt *ast.ImportStatement) string {
	if stmt == nil {
		return ""
	}
	if stmt.Alias != "" {
		return stmt.Alias
	}
	trimmed := strings.Trim(strings.TrimSuffix(stmt.Path, ".sec"), "/")
	if index := strings.LastIndex(trimmed, "/"); index >= 0 {
		return trimmed[index+1:]
	}
	return trimmed
}

func parseSourceInclude(path string) (*ast.Program, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	l := lexer.NewWithFile(string(data), path)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, false
	}
	return program, true
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
	localTypes := map[string]bool{}
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localFunctions[stmt.Name.Value] = true
			}
		case *ast.TypeDeclStatement:
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.UnitDeclStatement:
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.EnumDeclaration:
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.InterfaceDeclaration:
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.StructStatement:
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		}
	}

	for _, stmt := range program.Statements {
		qualifyLocalTypeReferencesInStatement(stmt, module, localTypes)
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			if stmt.Name == nil || strings.Contains(stmt.Name.Value, ".") {
				continue
			}
			qualifyLocalCalls(stmt.Body, module, localFunctions)
			stmt.Name.Value = module + "." + stmt.Name.Value
			stmt.Name.Token.Lexeme = stmt.Name.Value
		case *ast.TypeDeclStatement:
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.UnitDeclStatement:
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.EnumDeclaration:
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.InterfaceDeclaration:
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.StructStatement:
			qualifyIdentifierDeclaration(stmt.Name, module)
		}
	}
}

func qualifyIdentifierDeclaration(ident *ast.Identifier, module string) {
	if ident == nil || strings.Contains(ident.Value, ".") {
		return
	}
	ident.Value = module + "." + ident.Value
	ident.Token.Lexeme = ident.Value
}

func rewriteImportQualifier(program *ast.Program, from string, to string) {
	if program == nil || from == "" || to == "" || from == to {
		return
	}
	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			rewriteQualifierInBlock(stmt.Body, from, to)
		case *ast.ImplStatement:
			for _, member := range stmt.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok {
					rewriteQualifierInBlock(fn.Body, from, to)
				}
			}
		default:
			rewriteQualifierInStatement(stmt, from, to)
		}
	}
}

func rewriteQualifierInBlock(block *ast.BlockStatement, from string, to string) {
	if block == nil {
		return
	}
	for _, stmt := range block.Statements {
		rewriteQualifierInStatement(stmt, from, to)
	}
}

func rewriteQualifierInStatement(stmt ast.Statement, from string, to string) {
	switch stmt := stmt.(type) {
	case *ast.LetStatement:
		rewriteQualifierInExpression(stmt.Value, from, to)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			rewriteQualifierInStatement(let, from, to)
		}
	case *ast.AssignmentStatement:
		rewriteQualifierInExpression(stmt.Target, from, to)
		rewriteQualifierInExpression(stmt.Value, from, to)
	case *ast.ExpressionStatement:
		rewriteQualifierInExpression(stmt.Expression, from, to)
	case *ast.ReturnStatement:
		rewriteQualifierInExpression(stmt.Value, from, to)
	case *ast.IfStatement:
		rewriteQualifierInExpression(stmt.Condition, from, to)
		rewriteQualifierInBlock(stmt.Consequence, from, to)
		rewriteQualifierInBlock(stmt.Alternative, from, to)
	case *ast.ForStatement:
		rewriteQualifierInExpression(stmt.Iterable, from, to)
		rewriteQualifierInExpression(stmt.Step, from, to)
		rewriteQualifierInBlock(stmt.Body, from, to)
	case *ast.WhileStatement:
		rewriteQualifierInExpression(stmt.Condition, from, to)
		rewriteQualifierInBlock(stmt.Body, from, to)
	case *ast.UnsafeStatement:
		rewriteQualifierInBlock(stmt.Body, from, to)
	}
}

func rewriteQualifierInExpression(expr ast.Expression, from string, to string) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		if expr.Value == from {
			expr.Value = to
			expr.Token.Lexeme = to
		}
	case *ast.CallExpression:
		if expr.Function != nil {
			expr.Function.Value = rewriteQualifiedName(expr.Function.Value, from, to)
			expr.Function.Token.Lexeme = expr.Function.Value
		}
		rewriteQualifierInExpression(expr.Callee, from, to)
		for _, arg := range expr.Arguments {
			rewriteQualifierInExpression(arg, from, to)
		}
	case *ast.MemberExpression:
		rewriteQualifierInExpression(expr.Object, from, to)
	case *ast.IndexExpression:
		rewriteQualifierInExpression(expr.Left, from, to)
		rewriteQualifierInExpression(expr.Index, from, to)
	case *ast.PrefixExpression:
		rewriteQualifierInExpression(expr.Right, from, to)
	case *ast.InfixExpression:
		rewriteQualifierInExpression(expr.Left, from, to)
		rewriteQualifierInExpression(expr.Right, from, to)
	case *ast.ConversionExpression:
		if expr.Type != nil {
			expr.Type.Name = rewriteQualifiedName(expr.Type.Name, from, to)
		}
		rewriteQualifierInExpression(expr.Value, from, to)
	case *ast.RangeExpression:
		rewriteQualifierInExpression(expr.Start, from, to)
		rewriteQualifierInExpression(expr.End, from, to)
	case *ast.RefExpression:
		rewriteQualifierInExpression(expr.Value, from, to)
	}
}

func rewriteQualifiedName(name string, from string, to string) string {
	if name == from {
		return to
	}
	if strings.HasPrefix(name, from+".") {
		return to + strings.TrimPrefix(name, from)
	}
	return name
}

func qualifyLocalTypeReferencesInStatement(stmt ast.Statement, module string, localTypes map[string]bool) {
	switch stmt := stmt.(type) {
	case *ast.TypeDeclStatement:
		qualifyLocalTypeReference(stmt.BaseType, module, localTypes)
		qualifyLocalTypeReference(stmt.AssignedType, module, localTypes)
		for _, ref := range stmt.Implements {
			qualifyLocalTypeReference(ref, module, localTypes)
		}
		if stmt.StructType != nil {
			for _, field := range stmt.StructType.Fields {
				qualifyLocalTypeReference(field.Type, module, localTypes)
			}
		}
		if stmt.RegisterType != nil {
			for _, field := range stmt.RegisterType.Fields {
				qualifyLocalTypeReference(field.Type, module, localTypes)
			}
		}
		for _, variant := range stmt.UnionVariants {
			qualifyLocalTypeReference(variant.Payload, module, localTypes)
			for _, field := range variant.PayloadFields {
				qualifyLocalTypeReference(field.Type, module, localTypes)
			}
		}
	case *ast.UnitDeclStatement:
		qualifyLocalTypeReference(stmt.BaseType, module, localTypes)
	case *ast.InterfaceDeclaration:
		for _, ref := range stmt.Implements {
			qualifyLocalTypeReference(ref, module, localTypes)
		}
		for _, method := range stmt.Methods {
			qualifyLocalTypeReferencesInFunction(method, module, localTypes)
		}
		for _, property := range stmt.Properties {
			qualifyLocalTypeReference(property.Type, module, localTypes)
		}
	case *ast.StructStatement:
		for _, field := range stmt.Fields {
			qualifyLocalTypeReference(field.Type, module, localTypes)
		}
	case *ast.FunctionDeclaration:
		qualifyLocalTypeReferencesInFunction(stmt, module, localTypes)
	case *ast.ImplStatement:
		qualifyLocalTypeReference(stmt.Target, module, localTypes)
		for _, member := range stmt.Members {
			qualifyLocalTypeReferencesInImplMember(member, module, localTypes)
		}
	case *ast.LetStatement:
		qualifyLocalTypeReference(stmt.Type, module, localTypes)
		qualifyLocalTypesInExpression(stmt.Value, module, localTypes)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			qualifyLocalTypeReferencesInStatement(let, module, localTypes)
		}
	case *ast.AssignmentStatement:
		qualifyLocalTypesInExpression(stmt.Target, module, localTypes)
		qualifyLocalTypesInExpression(stmt.Value, module, localTypes)
	case *ast.ExpressionStatement:
		qualifyLocalTypesInExpression(stmt.Expression, module, localTypes)
	case *ast.ReturnStatement:
		qualifyLocalTypesInExpression(stmt.Value, module, localTypes)
	case *ast.IfStatement:
		qualifyLocalTypesInExpression(stmt.Condition, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Consequence, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Alternative, module, localTypes)
	case *ast.ForStatement:
		qualifyLocalTypesInExpression(stmt.Iterable, module, localTypes)
		qualifyLocalTypesInExpression(stmt.Step, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Body, module, localTypes)
	case *ast.WhileStatement:
		qualifyLocalTypesInExpression(stmt.Condition, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Body, module, localTypes)
	case *ast.SwitchStatement:
		qualifyLocalTypesInExpression(stmt.Subject, module, localTypes)
		for _, clause := range stmt.Cases {
			qualifyLocalTypesInSwitchCase(clause, module, localTypes)
		}
		qualifyLocalTypesInSwitchCase(stmt.Default, module, localTypes)
	case *ast.UnsafeStatement:
		qualifyLocalTypeReferencesInBlock(stmt.Body, module, localTypes)
	}
}

func qualifyLocalTypesInSwitchCase(clause *ast.SwitchCase, module string, localTypes map[string]bool) {
	if clause == nil {
		return
	}
	for _, item := range clause.Items {
		switch item := item.(type) {
		case *ast.SwitchValueCase:
			qualifyLocalTypesInExpression(item.Value, module, localTypes)
		case *ast.SwitchRangeCase:
			qualifyLocalTypesInExpression(item.Range, module, localTypes)
		case *ast.SwitchRelationalCase:
			qualifyLocalTypesInExpression(item.Value, module, localTypes)
		}
	}
	qualifyLocalTypeReferencesInBlock(clause.Body, module, localTypes)
}

func qualifyLocalTypeReferencesInImplMember(member ast.ImplMember, module string, localTypes map[string]bool) {
	switch member := member.(type) {
	case *ast.TypeDeclStatement:
		qualifyLocalTypeReferencesInStatement(member, module, localTypes)
	case *ast.UnitDeclStatement:
		qualifyLocalTypeReferencesInStatement(member, module, localTypes)
	case *ast.EnumDeclaration:
		qualifyLocalTypeReferencesInStatement(member, module, localTypes)
	case *ast.FunctionDeclaration:
		qualifyLocalTypeReferencesInFunction(member, module, localTypes)
	case *ast.PropertyDeclaration:
		qualifyLocalTypeReference(member.Type, module, localTypes)
		qualifyLocalTypeReferencesInBlock(member.Getter, module, localTypes)
		if member.Setter != nil {
			qualifyLocalTypeReferencesInBlock(member.Setter.Body, module, localTypes)
		}
	}
}

func qualifyLocalTypeReferencesInFunction(fn *ast.FunctionDeclaration, module string, localTypes map[string]bool) {
	if fn == nil {
		return
	}
	for _, parameter := range fn.Parameters {
		qualifyLocalTypeReference(parameter.Type, module, localTypes)
	}
	qualifyLocalTypeReference(fn.ReturnType, module, localTypes)
	qualifyLocalTypeReferencesInBlock(fn.Body, module, localTypes)
}

func qualifyLocalTypeReferencesInBlock(block *ast.BlockStatement, module string, localTypes map[string]bool) {
	if block == nil {
		return
	}
	for _, stmt := range block.Statements {
		qualifyLocalTypeReferencesInStatement(stmt, module, localTypes)
	}
}

func qualifyLocalTypeReference(ref *ast.TypeReference, module string, localTypes map[string]bool) {
	if ref == nil {
		return
	}
	if localTypes[ref.Name] {
		ref.Name = module + "." + ref.Name
		ref.Token.Lexeme = ref.Name
	}
	qualifyLocalTypeReference(ref.ElementType, module, localTypes)
	for _, arg := range ref.TypeArgs {
		qualifyLocalTypeReference(arg, module, localTypes)
	}
	for _, param := range ref.FunctionParameterTypes {
		qualifyLocalTypeReference(param, module, localTypes)
	}
	qualifyLocalTypeReference(ref.FunctionReturnType, module, localTypes)
	qualifyLocalTypesInExpression(ref.ArrayLengthExpression, module, localTypes)
}

func qualifyLocalTypesInExpression(expr ast.Expression, module string, localTypes map[string]bool) {
	switch expr := expr.(type) {
	case *ast.CallExpression:
		if expr.Function != nil && localTypes[expr.Function.Value] {
			expr.Function.Value = module + "." + expr.Function.Value
			expr.Function.Token.Lexeme = expr.Function.Value
		}
		for _, arg := range expr.GenericArguments {
			qualifyLocalTypeReference(arg, module, localTypes)
		}
		qualifyLocalTypesInExpression(expr.Callee, module, localTypes)
		for _, arg := range expr.Arguments {
			qualifyLocalTypesInExpression(arg, module, localTypes)
		}
	case *ast.MemberExpression:
		if ident, ok := expr.Object.(*ast.Identifier); ok && localTypes[ident.Value] {
			ident.Value = module + "." + ident.Value
			ident.Token.Lexeme = ident.Value
		} else {
			qualifyLocalTypesInExpression(expr.Object, module, localTypes)
		}
	case *ast.ConversionExpression:
		qualifyLocalTypeReference(expr.Type, module, localTypes)
		qualifyLocalTypesInExpression(expr.Value, module, localTypes)
	case *ast.PrefixExpression:
		qualifyLocalTypesInExpression(expr.Right, module, localTypes)
	case *ast.InfixExpression:
		qualifyLocalTypesInExpression(expr.Left, module, localTypes)
		qualifyLocalTypesInExpression(expr.Right, module, localTypes)
	case *ast.RangeExpression:
		qualifyLocalTypesInExpression(expr.Start, module, localTypes)
		qualifyLocalTypesInExpression(expr.End, module, localTypes)
	case *ast.IndexExpression:
		qualifyLocalTypesInExpression(expr.Left, module, localTypes)
		qualifyLocalTypesInExpression(expr.Index, module, localTypes)
	case *ast.RefExpression:
		qualifyLocalTypesInExpression(expr.Value, module, localTypes)
	case *ast.StructLiteral:
		qualifyLocalTypeReference(expr.Type, module, localTypes)
		for _, field := range expr.Fields {
			qualifyLocalTypesInExpression(field.Value, module, localTypes)
		}
	case *ast.ArrayLiteral:
		for _, element := range expr.Elements {
			qualifyLocalTypesInExpression(element, module, localTypes)
		}
	case *ast.OkExpression:
		qualifyLocalTypesInExpression(expr.Value, module, localTypes)
	case *ast.ErrExpression:
		qualifyLocalTypesInExpression(expr.Value, module, localTypes)
	case *ast.TryExpression:
		qualifyLocalTypesInExpression(expr.Expression, module, localTypes)
	case *ast.MatchExpression:
		qualifyLocalTypesInExpression(expr.Subject, module, localTypes)
		for _, arm := range expr.Arms {
			qualifyLocalTypesInExpression(arm.Pattern, module, localTypes)
			qualifyLocalTypesInExpression(arm.Guard, module, localTypes)
			qualifyLocalTypesInExpression(arm.Body, module, localTypes)
			qualifyLocalTypeReferencesInBlock(arm.BlockBody, module, localTypes)
		}
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

func semaDiagnostic(err sema.Error, severity int) diagnostic {
	line := max(err.Line-1, 0)
	column := max(err.Column-1, 0)
	return diagnostic{
		Range: lspRange{
			Start: position{Line: line, Character: column},
			End:   position{Line: line, Character: column + 1},
		},
		Severity: severity,
		Source:   "sec",
		Message:  err.Error(),
	}
}

var parserPositionPattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./\\:-])(\d+):(\d+)\b`)

func parserDiagnostic(message string) diagnostic {
	line, column := 0, 0
	matches := parserPositionPattern.FindAllStringSubmatch(message, -1)
	if len(matches) > 0 {
		last := matches[len(matches)-1]
		if parsedLine, err := strconv.Atoi(last[1]); err == nil && parsedLine > 0 {
			line = parsedLine - 1
		}
		if parsedColumn, err := strconv.Atoi(last[2]); err == nil && parsedColumn > 0 {
			column = parsedColumn - 1
		}
	}
	return diagnostic{
		Range: lspRange{
			Start: position{Line: line, Character: column},
			End:   position{Line: line, Character: column + 1},
		},
		Severity: 1,
		Source:   "sec",
		Message:  message,
	}
}

func readMessage(reader *bufio.Reader) (rpcMessage, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return rpcMessage{}, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(key), "Content-Length") {
			length, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return rpcMessage{}, err
			}
			contentLength = length
		}
	}
	if contentLength < 0 {
		return rpcMessage{}, fmt.Errorf("missing Content-Length header")
	}

	body := make([]byte, contentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return rpcMessage{}, err
	}

	var message rpcMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return rpcMessage{}, err
	}
	return message, nil
}

func (s *server) respond(id json.RawMessage, result any) error {
	return s.writeMessage(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *server) respondError(id json.RawMessage, code int, message string) error {
	return s.writeMessage(rpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func (s *server) notify(method string, params any) error {
	rawParams, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return s.writeMessage(rpcMessage{
		JSONRPC: "2.0",
		Method:  method,
		Params:  rawParams,
	})
}

func (s *server) writeMessage(message rpcMessage) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	body, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.out, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err = s.out.Write(body)
	return err
}

func pathFromURI(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil || parsed.Scheme != "file" {
		return uri
	}
	path, err := url.PathUnescape(parsed.Path)
	if err != nil {
		return parsed.Path
	}
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}
	return filepath.Clean(path)
}
