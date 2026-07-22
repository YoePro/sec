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

		if blankPending && len(out) > 0 {
			out = append(out, "")
			blankPending = false
		}

		out = append(out, strings.Repeat(" ", lineIndent*4)+trimmed)
		indent += braceIndentDelta(trimmed)
		if indent < 0 {
			indent = 0
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
	resolveSourceImports(program, map[string]bool{})

	analyzer := sema.NewAnalyzer()
	for _, err := range analyzer.Analyze(program) {
		diagnostics = append(diagnostics, semaDiagnostic(err, 1))
	}
	for _, warning := range analyzer.Warnings() {
		diagnostics = append(diagnostics, semaDiagnostic(warning, 2))
	}
	return diagnostics
}

func resolveSourceImports(program *ast.Program, seen map[string]bool) {
	for _, stmt := range append([]ast.Statement{}, program.Statements...) {
		importStmt, ok := stmt.(*ast.ImportStatement)
		if !ok {
			continue
		}
		sourcePath, ok := sourceIncludePath(importStmt.Path)
		if !ok || seen[sourcePath] {
			continue
		}
		seen[sourcePath] = true

		imported, ok := parseSourceInclude(sourcePath)
		if !ok {
			continue
		}
		resolveSourceImports(imported, seen)
		module := programModulePath(imported)
		qualifyImportedModule(imported, module)
		program.Statements = append(program.Statements, imported.Statements...)
	}
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
	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if ok && fn.Name != nil && !strings.Contains(fn.Name.Value, ".") {
			localFunctions[fn.Name.Value] = true
		}
	}

	for _, stmt := range program.Statements {
		fn, ok := stmt.(*ast.FunctionDeclaration)
		if !ok || fn.Name == nil {
			continue
		}
		if strings.Contains(fn.Name.Value, ".") {
			continue
		}
		qualifyLocalCalls(fn.Body, module, localFunctions)
		fn.Name.Value = module + "." + fn.Name.Value
		fn.Name.Token.Lexeme = fn.Name.Value
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
