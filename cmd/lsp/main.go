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

type completionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type completionItem struct {
	Label         string `json:"label"`                   //
	Kind          int    `json:"kind"`                    // 5 = Field, 2 = Method, 13 = Enum, 20 = EnumMember
	Detail        string `json:"detail,omitempty"`        //
	Documentation string `json:"documentation,omitempty"` //
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
	Line      int `json:"line"`      // 0-indexed line number
	Character int `json:"character"` // 0-indexed character offset
}

type server struct {
	in               *bufio.Reader          //
	out              io.Writer              //
	documents        map[string]string      //
	docMu            sync.RWMutex           // Protects the documents map from concurrent read/write operations
	diagnosticTimers map[string]*time.Timer //
	diagnosticDelay  time.Duration          //
	writeMu          sync.Mutex             //
	timerMu          sync.Mutex             //
	shutdown         bool                   //
}

type formatterBranchContext struct {
	contentDepth int
	branchActive bool
	bodyExtra    bool
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
				"completionProvider": map[string]any{
					"triggerCharacters": []string{"."},
				},
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
	case "textDocument/completion":
		var params completionParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}

		// Thread-safe reading from the document storage
		s.docMu.RLock()
		text, ok := s.documents[params.TextDocument.URI]
		s.docMu.RUnlock()
		if !ok {
			return s.respond(message.ID, []completionItem{})
		}

		offset := lineCharToOffset(text, params.Position.Line, params.Position.Character)
		items := completeSource(params.TextDocument.URI, text, offset)
		return s.respond(message.ID, items)
	default:
		if len(message.ID) == 0 {
			return nil
		}
		return s.respondError(message.ID, -32601, "method not found")
	}
}

func completeSource(uri string, text string, offset int) []completionItem {
	context := completionContextAt(text, offset)
	parseText := text
	if context.Member && context.Prefix == "" {
		parseText = text[:offset] + "__sec_completion" + text[offset:]
	}

	l := lexer.New(parseText)
	if uri != "" {
		l = lexer.NewWithFile(parseText, pathFromURI(uri))
	}
	p := parser.New(l)
	fileAST := p.ParseProgram()

	analyzer := sema.NewAnalyzer()
	if fileAST != nil && len(p.Errors()) == 0 {
		if uri != "" {
			resolveCoreSources(fileAST, pathFromURI(uri))
			resolveSourceImports(fileAST, map[string]bool{}, pathFromURI(uri))
		}
		analyzer.Analyze(fileAST)
		if expected, ok := expectedReturnTypeAt(fileAST, analyzer, parseText, offset); ok {
			context.ExpectedType = &expected
		}
	}

	if context.Member {
		if fileAST == nil {
			return []completionItem{}
		}
		targetExpr := findSelectorLHS(fileAST, text, context.DotOffset)
		if targetExpr == nil {
			return []completionItem{}
		}
		exprType, ok := analyzer.TypeOf(targetExpr)
		if !ok {
			return []completionItem{}
		}
		return memberCompletionItems(exprType, analyzer.Functions(), analyzer.Symbols(), context.Prefix)
	}

	return globalCompletionItems(text, analyzer, context)
}

type completionContext struct {
	Prefix              string
	Member              bool
	DotOffset           int
	TypeForm            bool
	ContractModifier    bool
	ReturnValue         bool
	ExpectedType        *sema.Type
	CursorOffset        int
	FunctionStartOffset int
}

func completionContextAt(text string, offset int) completionContext {
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}

	prefixStart := offset
	for prefixStart > 0 && isIdentifierByte(text[prefixStart-1]) {
		prefixStart--
	}

	context := completionContext{
		Prefix:              strings.TrimSpace(text[prefixStart:offset]),
		CursorOffset:        offset,
		FunctionStartOffset: functionStartOffsetBefore(text, offset),
	}
	if prefixStart > 0 && text[prefixStart-1] == '.' {
		context.Member = true
		context.DotOffset = prefixStart - 1
	}
	context.ReturnValue = isReturnValueContext(text[:prefixStart], offset)
	context.TypeForm = isTypeDeclarationFormContext(text[:prefixStart])
	context.ContractModifier = isContractModifierContext(text[:prefixStart])
	return context
}

func isReturnValueContext(prefix string, offset int) bool {
	cursorLine := lineAtOffset(prefix, offset)
	l := lexer.New(prefix)
	var previous lexer.Token
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			break
		}
		if token.Type == lexer.COMMENT {
			continue
		}
		previous = token
	}
	return previous.Type == lexer.RETURN && previous.Line == cursorLine
}

func lineAtOffset(text string, offset int) int {
	if offset > len(text) {
		offset = len(text)
	}
	line := 1
	for i := 0; i < offset; i++ {
		if text[i] == '\n' {
			line++
		}
	}
	return line
}

func functionStartOffsetBefore(text string, offset int) int {
	if offset > len(text) {
		offset = len(text)
	}
	prefix := text[:offset]
	start := strings.LastIndex(prefix, "\nfn ")
	if start >= 0 {
		return start + 1
	}
	if strings.HasPrefix(prefix, "fn ") {
		return 0
	}
	return -1
}

func isIdentifierByte(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_'
}

func isTypeDeclarationFormContext(prefix string) bool {
	l := lexer.New(prefix)
	tokens := []lexer.Token{}
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			break
		}
		if token.Type == lexer.COMMENT {
			continue
		}
		tokens = append(tokens, token)
	}
	for i := len(tokens) - 1; i >= 1; i-- {
		if tokens[i-1].Type == lexer.TYPE && tokens[i].Type == lexer.IDENT {
			return true
		}
		if tokens[i].Type == lexer.ASSIGN || tokens[i].Type == lexer.LBRACE || tokens[i].Type == lexer.RBRACE {
			return false
		}
	}
	return false
}

func isContractModifierContext(prefix string) bool {
	lineStart := strings.LastIndex(prefix, "\n") + 1
	currentLine := strings.TrimSpace(prefix[lineStart:])
	if currentLine == "" {
		return false
	}

	l := lexer.New(currentLine)
	tokens := []lexer.Token{}
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			break
		}
		if token.Type == lexer.COMMENT {
			continue
		}
		if token.Type == lexer.ASSIGN || token.Type == lexer.LBRACE || token.Type == lexer.RBRACE {
			return false
		}
		tokens = append(tokens, token)
	}
	if len(tokens) >= 3 && tokens[0].Type == lexer.TYPE && tokens[1].Type == lexer.IDENT {
		return true
	}
	if len(tokens) >= 4 && tokens[0].Type == lexer.LET {
		for i, token := range tokens {
			if token.Type == lexer.COLON && i+1 < len(tokens) {
				return true
			}
		}
	}
	return false
}

func memberCompletionItems(exprType sema.Type, functions map[string][]sema.Function, symbols map[string]sema.Symbol, prefix string) []completionItem {
	items := []completionItem{}
	seen := map[string]bool{}
	add := func(item completionItem) {
		if item.Label == "" || seen[item.Label] || !completionLabelMatches(item.Label, prefix) {
			return
		}
		seen[item.Label] = true
		items = append(items, item)
	}

	switch exprType.Kind {
	case sema.StructType:
		for _, field := range exprType.Fields {
			add(completionItem{Label: field.Name, Kind: 5, Detail: lspTypeName(field.Type)})
		}
	case sema.RegisterType:
		for _, field := range exprType.RegisterFields {
			add(completionItem{Label: field.Name, Kind: 5, Detail: lspTypeName(field.Type)})
		}
	case sema.EnumType:
		for _, variant := range exprType.EnumValues {
			add(completionItem{Label: variant, Kind: 20, Detail: lspTypeName(exprType)})
		}
	case sema.UnionType:
		for _, variant := range exprType.UnionVariants {
			add(completionItem{Label: variant.Name, Kind: 20, Detail: lspTypeName(exprType)})
		}
	}

	for _, property := range exprType.Properties {
		add(completionItem{Label: property.Name, Kind: 10, Detail: lspTypeName(property.Type)})
	}

	typeName := exprType.Name
	if typeName != "" {
		methodPrefix := typeName + "."
		for name, overloads := range functions {
			if !strings.HasPrefix(name, methodPrefix) {
				continue
			}
			methodName := strings.TrimPrefix(name, methodPrefix)
			add(completionItem{Label: methodName, Kind: 2, Detail: functionCompletionDetail(overloads)})
		}
		staticPrefix := typeName + "."
		for name, symbol := range symbols {
			if !strings.HasPrefix(name, staticPrefix) {
				continue
			}
			memberName := strings.TrimPrefix(name, staticPrefix)
			add(completionItem{Label: memberName, Kind: 6, Detail: lspTypeName(symbol.Type)})
		}
	}

	sortCompletionItems(items)
	return items
}

func globalCompletionItems(text string, analyzer *sema.Analyzer, context completionContext) []completionItem {
	items := []completionItem{}
	seen := map[string]bool{}
	add := func(item completionItem) {
		if item.Label == "" || seen[item.Label] || !completionLabelMatches(item.Label, context.Prefix) {
			return
		}
		seen[item.Label] = true
		items = append(items, item)
	}

	if context.ReturnValue && context.ExpectedType != nil {
		addReturnValueCompletionItems(text, analyzer, context, add)
		sortCompletionItems(items)
		return items
	}

	keywords := secKeywords
	if context.ContractModifier {
		keywords = []string{"range", "in", "multipleOf", "notEmpty", "unique", "finite", "odd", "even"}
	} else if context.TypeForm {
		keywords = []string{"struct", "union", "enum", "interface", "register"}
	}
	for _, keyword := range keywords {
		add(completionItem{Label: keyword, Kind: 14})
	}

	if !context.TypeForm {
		for name, overloads := range analyzer.Functions() {
			add(completionItem{Label: name, Kind: 3, Detail: functionCompletionDetail(overloads)})
		}
		for name, symbol := range analyzer.Symbols() {
			if strings.Contains(name, ".") {
				continue
			}
			if symbol.Local && !symbolVisibleAtCompletion(textPositionOffset(text, symbol.Token.Line, symbol.Token.Column), context) {
				continue
			}
			add(completionItem{Label: name, Kind: 6, Detail: lspTypeName(symbol.Type)})
		}
	}
	for name, typ := range analyzer.Types() {
		add(completionItem{Label: name, Kind: typeCompletionKind(typ), Detail: string(typ.Kind)})
	}

	sortCompletionItems(items)
	return items
}

func addReturnValueCompletionItems(text string, analyzer *sema.Analyzer, context completionContext, add func(completionItem)) {
	expected := *context.ExpectedType
	if expected.Kind == sema.BoolType {
		add(completionItem{Label: "false", Kind: 14, Detail: "bool"})
		add(completionItem{Label: "true", Kind: 14, Detail: "bool"})
	}

	for name, overloads := range analyzer.Functions() {
		for _, function := range overloads {
			if !completionTypeMatches(expected, function.ReturnType) {
				continue
			}
			add(completionItem{Label: name, Kind: 3, Detail: lspTypeName(function.ReturnType)})
			break
		}
	}

	for name, symbol := range analyzer.Symbols() {
		if strings.Contains(name, ".") {
			continue
		}
		if symbol.Local && !symbolVisibleAtCompletion(textPositionOffset(text, symbol.Token.Line, symbol.Token.Column), context) {
			continue
		}
		if completionTypeMatches(expected, symbol.Type) {
			add(completionItem{Label: name, Kind: 6, Detail: lspTypeName(symbol.Type)})
		}
	}
}

func completionTypeMatches(expected sema.Type, actual sema.Type) bool {
	if expected.Kind == sema.InvalidType || actual.Kind == sema.InvalidType {
		return false
	}
	if expected.Name != "" || actual.Name != "" {
		return expected.Name != "" && expected.Name == actual.Name
	}
	return expected.Kind == actual.Kind
}

func expectedReturnTypeAt(program *ast.Program, analyzer *sema.Analyzer, text string, offset int) (sema.Type, bool) {
	if program == nil {
		return sema.Type{}, false
	}
	functions := analyzer.Functions()
	var best sema.Type
	bestStart := -1
	var visitFunction func(fn *ast.FunctionDeclaration, qualifiedName string)
	visitFunction = func(fn *ast.FunctionDeclaration, qualifiedName string) {
		if fn == nil || fn.Body == nil {
			return
		}
		start := textPositionOffset(text, fn.Body.Token.Line, fn.Body.Token.Column)
		end := matchingBraceOffset(text, start)
		if start < 0 || end < 0 || offset < start || offset > end || start < bestStart {
			return
		}
		overloads := functions[qualifiedName]
		if len(overloads) == 0 && fn.Name != nil {
			overloads = functions[fn.Name.Value]
		}
		if len(overloads) == 0 {
			return
		}
		best = overloads[0].ReturnType
		bestStart = start
	}

	for _, stmt := range program.Statements {
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			if stmt.Name != nil {
				visitFunction(stmt, stmt.Name.Value)
			}
		case *ast.ImplStatement:
			target := ""
			if stmt.Target != nil {
				target = stmt.Target.Name
			}
			for _, member := range stmt.Members {
				fn, ok := member.(*ast.FunctionDeclaration)
				if !ok || fn.Name == nil {
					continue
				}
				name := fn.Name.Value
				if target != "" {
					name = target + "." + name
				}
				visitFunction(fn, name)
			}
		}
	}
	if bestStart < 0 {
		return sema.Type{}, false
	}
	return best, true
}

func matchingBraceOffset(text string, openOffset int) int {
	if openOffset < 0 || openOffset >= len(text) || text[openOffset] != '{' {
		return -1
	}
	depth := 0
	inString := false
	escaped := false
	inLineComment := false
	for i := openOffset; i < len(text); i++ {
		ch := text[i]
		if inLineComment {
			if ch == '\n' {
				inLineComment = false
			}
			continue
		}
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '/' && i+1 < len(text) && text[i+1] == '/' {
			inLineComment = true
			i++
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		switch ch {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(text)
}

func symbolVisibleAtCompletion(symbolOffset int, context completionContext) bool {
	if !contextHasFunction(context) {
		return !context.Member
	}
	return symbolOffset >= context.FunctionStartOffset && symbolOffset <= context.CursorOffset
}

func contextHasFunction(context completionContext) bool {
	return context.FunctionStartOffset >= 0
}

var secKeywords = []string{
	"after", "asm", "assert", "await", "break", "case", "capture", "continue", "default",
	"defer", "else", "enum", "even", "extern", "fallthrough", "false", "finite", "fn", "for",
	"free", "get", "if", "impl", "implements", "import", "in", "interface",
	"let", "match", "module", "multipleOf", "mut", "notEmpty", "odd", "panic", "process",
	"property", "ref", "return", "select", "self", "set", "spawn", "static", "struct",
	"switch", "task", "thread", "true", "try", "type", "unique", "unit", "union",
	"unsafe", "where", "while",
}

func typeCompletionKind(typ sema.Type) int {
	switch typ.Kind {
	case sema.EnumType:
		return 13
	case sema.InterfaceType:
		return 8
	default:
		return 7
	}
}

func functionCompletionDetail(functions []sema.Function) string {
	if len(functions) == 0 {
		return ""
	}
	if len(functions) > 1 {
		return fmt.Sprintf("%d overloads", len(functions))
	}
	return lspTypeName(functions[0].ReturnType)
}

func lspTypeName(typ sema.Type) string {
	if typ.Name != "" {
		return typ.Name
	}
	if typ.Kind != "" {
		return string(typ.Kind)
	}
	return ""
}

func completionLabelMatches(label string, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(label), strings.ToLower(prefix))
}

func sortCompletionItems(items []completionItem) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Label == items[j].Label {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Label < items[j].Label
	})
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
	branchBlocks := []formatterBranchContext{}
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

		for len(branchBlocks) > 0 && lineIndent < branchBlocks[len(branchBlocks)-1].contentDepth {
			branchBlocks = branchBlocks[:len(branchBlocks)-1]
		}

		branchContext := -1
		if isBranchClause(trimmed) {
			for i := len(branchBlocks) - 1; i >= 0; i-- {
				if branchBlocks[i].contentDepth == lineIndent {
					branchContext = i
					break
				}
			}
		}

		extraIndent := 0
		if inImportGroup && trimmed != ")" {
			extraIndent++
		}
		for i, context := range branchBlocks {
			if context.bodyExtra && context.branchActive && lineIndent >= context.contentDepth {
				if i == branchContext {
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
		if isBranchBlockStart(trimmed) && indentDelta > 0 {
			branchBlocks = append(branchBlocks, formatterBranchContext{contentDepth: indent, bodyExtra: isSwitchBlockStart(trimmed)})
		}
		if trimmed == "import (" {
			inImportGroup = true
		} else if inImportGroup && trimmed == ")" {
			inImportGroup = false
		}
		if branchContext >= 0 {
			branchBlocks[branchContext].branchActive = true
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

func isBranchBlockStart(line string) bool {
	return isSwitchBlockStart(line) || line == "select {" || strings.HasPrefix(line, "select ")
}

func isBranchClause(line string) bool {
	return isSwitchCaseClause(line) || isSelectBranchClause(line)
}

func isSwitchCaseClause(line string) bool {
	return line == "case" ||
		strings.HasPrefix(line, "case ") ||
		strings.HasPrefix(line, "case\t") ||
		line == "default:" ||
		strings.HasPrefix(line, "default ")
}

func isSelectBranchClause(line string) bool {
	return line == "default => {" ||
		strings.HasPrefix(line, "after ") ||
		strings.HasSuffix(line, "=> {")
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

func lineCharToOffset(text string, line, char int) int {
	currentLine := 0
	currentCol := 0

	for i, r := range text {
		if currentLine == line && currentCol == char {
			return i
		}
		if r == '\n' {
			currentLine++
			currentCol = 0
		} else {
			currentCol++
		}
	}
	return len(text)
}

func textPositionOffset(text string, line int, column int) int {
	if line <= 0 || column <= 0 {
		return -1
	}
	return lineCharToOffset(text, line-1, column-1)
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
	resolveCoreSources(program, path)
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

func resolveCoreSources(program *ast.Program, sourceFile string) {
	if program == nil || lspProgramContainsCoreSource(program) {
		return
	}
	root := findSecSourceRoot(sourceFile)
	matches, err := filepath.Glob(filepath.Join(root, "sec", "core", "*.sec"))
	if err != nil || len(matches) == 0 {
		return
	}
	sort.Strings(matches)
	coreStatements := []ast.Statement{}
	for _, match := range matches {
		imported, ok := parseSourceInclude(match)
		if !ok {
			continue
		}
		coreStatements = append(coreStatements, imported.Statements...)
	}
	if len(coreStatements) == 0 {
		return
	}
	program.Statements = append(append([]ast.Statement{}, coreStatements...), program.Statements...)
}

func lspProgramContainsCoreSource(program *ast.Program) bool {
	for _, stmt := range program.Statements {
		token, ok := lspStatementTokenForSource(stmt)
		if !ok || token.File == "" {
			continue
		}
		path := filepath.ToSlash(filepath.Clean(token.File))
		if strings.Contains(path, "/sec/core/") || strings.HasPrefix(path, "sec/core/") {
			return true
		}
	}
	return false
}

func lspStatementTokenForSource(stmt ast.Statement) (lexer.Token, bool) {
	switch stmt := stmt.(type) {
	case *ast.ModuleStatement:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	case *ast.TypeDeclStatement:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	case *ast.UnitDeclStatement:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	case *ast.EnumDeclaration:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	case *ast.InterfaceDeclaration:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	case *ast.ImplStatement:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	case *ast.FunctionDeclaration:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	case *ast.LetStatement:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	case *ast.StructStatement:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	case *ast.ImportStatement:
		if stmt == nil {
			return lexer.Token{}, false
		}
		return stmt.Token, true
	default:
		return lexer.Token{}, false
	}
}

func resolveSourceImports(program *ast.Program, seen map[string]bool, sourceFile string) {
	for _, stmt := range append([]ast.Statement{}, program.Statements...) {
		importStmt, ok := stmt.(*ast.ImportStatement)
		if !ok || importStmt == nil {
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

// findSelectorLHS walks the AST and returns the expression that matches the selector
// written immediately before the cursor, such as the "foo" in "foo.".
func findSelectorLHS(node any, text string, dotOffset int) ast.Expression {
	if node == nil {
		return nil
	}

	selectorText := selectorTextBeforeCursor(text, dotOffset)
	if selectorText == "" {
		return nil
	}
	if selectorText == "" {
		return nil
	}

	switch n := node.(type) {
	case *ast.Program:
		for _, stmt := range n.Statements {
			if found := findSelectorLHS(stmt, text, dotOffset); found != nil {
				return found
			}
		}
	case *ast.FunctionDeclaration:
		if n == nil {
			return nil
		}
		return findSelectorLHS(n.Body, text, dotOffset)
	case *ast.BlockStatement:
		for _, stmt := range n.Statements {
			if found := findSelectorLHS(stmt, text, dotOffset); found != nil {
				return found
			}
		}
	case *ast.LetStatement:
		return findSelectorLHS(n.Value, text, dotOffset)
	case *ast.ExpressionStatement:
		return findSelectorLHS(n.Expression, text, dotOffset)
	case *ast.AssignmentStatement:
		return findSelectorLHS(n.Value, text, dotOffset)
	case *ast.ReturnStatement:
		return findSelectorLHS(n.Value, text, dotOffset)
	case *ast.ImportStatement:
		return nil
	case *ast.IfStatement:
		if found := findSelectorLHS(n.Condition, text, dotOffset); found != nil {
			return found
		}
		if found := findSelectorLHS(n.Consequence, text, dotOffset); found != nil {
			return found
		}
		return findSelectorLHS(n.Alternative, text, dotOffset)
	case *ast.ForStatement:
		if found := findSelectorLHS(n.Iterable, text, dotOffset); found != nil {
			return found
		}
		if found := findSelectorLHS(n.Step, text, dotOffset); found != nil {
			return found
		}
		return findSelectorLHS(n.Body, text, dotOffset)
	case *ast.WhileStatement:
		if found := findSelectorLHS(n.Condition, text, dotOffset); found != nil {
			return found
		}
		return findSelectorLHS(n.Body, text, dotOffset)
	case *ast.SwitchStatement:
		if found := findSelectorLHS(n.Subject, text, dotOffset); found != nil {
			return found
		}
		for _, clause := range n.Cases {
			if clause != nil {
				if found := findSelectorLHS(clause.Body, text, dotOffset); found != nil {
					return found
				}
			}
		}
		if n.Default != nil {
			return findSelectorLHS(n.Default.Body, text, dotOffset)
		}
	case *ast.SelectStatement:
		for _, branch := range n.Branches {
			if branch == nil {
				continue
			}
			if found := findSelectorLHS(branch.Value, text, dotOffset); found != nil {
				return found
			}
			if found := findSelectorLHS(branch.Body, text, dotOffset); found != nil {
				return found
			}
		}
	case *ast.UnsafeStatement:
		return findSelectorLHS(n.Body, text, dotOffset)
	}

	if expr, ok := node.(ast.Expression); ok {
		if expr == nil {
			return nil
		}
		if ident, ok := expr.(*ast.Identifier); ok && ident != nil && ident.Value == selectorText {
			return expr
		}
		if member, ok := expr.(*ast.MemberExpression); ok && member != nil {
			if member.Object != nil && member.Object.String() == selectorText {
				return member.Object
			}
			if member.String() == selectorText {
				return expr
			}
		}
		if expr.String() == selectorText {
			return expr
		}
	}

	return nil
}

func selectorTextBeforeCursor(text string, dotOffset int) string {
	if dotOffset < 0 || dotOffset > len(text) {
		return ""
	}
	prefix := text[:dotOffset]
	start := len(prefix)
	for start > 0 {
		ch := prefix[start-1]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || ch == '.' {
			start--
			continue
		}
		break
	}
	selector := strings.TrimSpace(prefix[start:])
	if selector == "" {
		return ""
	}
	if lastDot := strings.LastIndex(selector, "."); lastDot >= 0 {
		return selector[:lastDot]
	}
	return selector
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
			if stmt == nil {
				continue
			}
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localFunctions[stmt.Name.Value] = true
			}
		case *ast.TypeDeclStatement:
			if stmt == nil {
				continue
			}
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.UnitDeclStatement:
			if stmt == nil {
				continue
			}
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.EnumDeclaration:
			if stmt == nil {
				continue
			}
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.InterfaceDeclaration:
			if stmt == nil {
				continue
			}
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		case *ast.StructStatement:
			if stmt == nil {
				continue
			}
			if stmt.Name != nil && !strings.Contains(stmt.Name.Value, ".") {
				localTypes[stmt.Name.Value] = true
			}
		}
	}

	for _, stmt := range program.Statements {
		if stmt == nil {
			continue
		}
		qualifyLocalTypeReferencesInStatement(stmt, module, localTypes)
		switch stmt := stmt.(type) {
		case *ast.FunctionDeclaration:
			if stmt == nil {
				continue
			}
			if stmt.Name == nil || strings.Contains(stmt.Name.Value, ".") {
				continue
			}
			qualifyLocalCalls(stmt.Body, module, localFunctions)
			stmt.Name.Value = module + "." + stmt.Name.Value
			stmt.Name.Token.Lexeme = stmt.Name.Value
		case *ast.TypeDeclStatement:
			if stmt == nil {
				continue
			}
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.UnitDeclStatement:
			if stmt == nil {
				continue
			}
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.EnumDeclaration:
			if stmt == nil {
				continue
			}
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.InterfaceDeclaration:
			if stmt == nil {
				continue
			}
			qualifyIdentifierDeclaration(stmt.Name, module)
		case *ast.StructStatement:
			if stmt == nil {
				continue
			}
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
			if stmt == nil {
				continue
			}
			rewriteQualifierInBlock(stmt.Body, from, to)
		case *ast.ImplStatement:
			if stmt == nil {
				continue
			}
			for _, member := range stmt.Members {
				if fn, ok := member.(*ast.FunctionDeclaration); ok && fn != nil {
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
		if stmt == nil {
			return
		}
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
		if stmt == nil {
			return
		}
		qualifyLocalTypeReference(stmt.BaseType, module, localTypes)
	case *ast.InterfaceDeclaration:
		if stmt == nil {
			return
		}
		for _, ref := range stmt.Implements {
			qualifyLocalTypeReference(ref, module, localTypes)
		}
		for _, method := range stmt.Methods {
			qualifyLocalTypeReferencesInFunction(method, module, localTypes)
		}
		for _, property := range stmt.Properties {
			if property == nil {
				continue
			}
			qualifyLocalTypeReference(property.Type, module, localTypes)
		}
		for _, event := range stmt.Events {
			if event == nil {
				continue
			}
			qualifyLocalTypeReference(event.Payload, module, localTypes)
		}
	case *ast.StructStatement:
		if stmt == nil {
			return
		}
		for _, field := range stmt.Fields {
			qualifyLocalTypeReference(field.Type, module, localTypes)
		}
	case *ast.FunctionDeclaration:
		if stmt == nil {
			return
		}
		qualifyLocalTypeReferencesInFunction(stmt, module, localTypes)
	case *ast.ImplStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypeReference(stmt.Target, module, localTypes)
		for _, member := range stmt.Members {
			qualifyLocalTypeReferencesInImplMember(member, module, localTypes)
		}
	case *ast.LetStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypeReference(stmt.Type, module, localTypes)
		qualifyLocalTypesInExpression(stmt.Value, module, localTypes)
	case *ast.LetGroupStatement:
		for _, let := range stmt.Lets {
			qualifyLocalTypeReferencesInStatement(let, module, localTypes)
		}
	case *ast.AssignmentStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Target, module, localTypes)
		qualifyLocalTypesInExpression(stmt.Value, module, localTypes)
	case *ast.ExpressionStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Expression, module, localTypes)
	case *ast.ReturnStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Value, module, localTypes)
	case *ast.IfStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Condition, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Consequence, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Alternative, module, localTypes)
	case *ast.ForStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Iterable, module, localTypes)
		qualifyLocalTypesInExpression(stmt.Step, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Body, module, localTypes)
	case *ast.WhileStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Condition, module, localTypes)
		qualifyLocalTypeReferencesInBlock(stmt.Body, module, localTypes)
	case *ast.SwitchStatement:
		if stmt == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Subject, module, localTypes)
		for _, clause := range stmt.Cases {
			qualifyLocalTypesInSwitchCase(clause, module, localTypes)
		}
		qualifyLocalTypesInSwitchCase(stmt.Default, module, localTypes)
	case *ast.SelectStatement:
		if stmt == nil {
			return
		}
		for _, branch := range stmt.Branches {
			if branch == nil {
				continue
			}
			qualifyLocalTypesInExpression(branch.Value, module, localTypes)
			qualifyLocalTypeReferencesInBlock(branch.Body, module, localTypes)
		}
	case *ast.UnsafeStatement:
		if stmt == nil {
			return
		}
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
		if member == nil {
			return
		}
		qualifyLocalTypeReference(member.Type, module, localTypes)
		qualifyLocalTypeReferencesInBlock(member.Getter, module, localTypes)
		if member.Setter != nil {
			qualifyLocalTypeReferencesInBlock(member.Setter.Body, module, localTypes)
		}
	case *ast.EventDeclaration:
		return
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
	case *ast.SwitchStatement:
		qualifyLocalCallsInExpression(stmt.Subject, module, localFunctions)
		for _, clause := range stmt.Cases {
			if clause != nil {
				for _, item := range clause.Items {
					switch item := item.(type) {
					case *ast.SwitchValueCase:
						qualifyLocalCallsInExpression(item.Value, module, localFunctions)
					case *ast.SwitchRangeCase:
						qualifyLocalCallsInExpression(item.Range, module, localFunctions)
					case *ast.SwitchRelationalCase:
						qualifyLocalCallsInExpression(item.Value, module, localFunctions)
					}
				}
				qualifyLocalCalls(clause.Body, module, localFunctions)
			}
		}
		if stmt.Default != nil {
			qualifyLocalCalls(stmt.Default.Body, module, localFunctions)
		}
	case *ast.SelectStatement:
		for _, branch := range stmt.Branches {
			if branch == nil {
				continue
			}
			qualifyLocalCallsInExpression(branch.Value, module, localFunctions)
			qualifyLocalCalls(branch.Body, module, localFunctions)
		}
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
