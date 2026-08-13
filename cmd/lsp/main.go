package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/formatter"
	"sec/internal/lexer"
	"sec/internal/lsp/features"
	"sec/internal/lsp/protocol"
	lspserver "sec/internal/lsp/server"
	"sec/internal/parser"
	"sec/internal/sema"
)

type rpcMessage = protocol.Message
type rpcError = protocol.Error
type rpcResponseMessage = protocol.ResponseMessage

type textDocumentItem struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
	Text    string `json:"text"`
}

type didOpenParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
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

type hoverParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type definitionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type documentHighlightParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type callHierarchyPrepareParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type callHierarchyIncomingCallsParams struct {
	Item callHierarchyItem `json:"item"`
}

type callHierarchyOutgoingCallsParams struct {
	Item callHierarchyItem `json:"item"`
}

type documentSymbolParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type semanticTokensParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type documentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          lspRange         `json:"range"`
	SelectionRange lspRange         `json:"selectionRange"`
	Children       []documentSymbol `json:"children,omitempty"`
}

type semanticTokens struct {
	Data []int `json:"data"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
	Range    lspRange      `json:"range,omitempty"`
}

type location struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type documentHighlight struct {
	Range lspRange `json:"range"`
	Kind  int      `json:"kind,omitempty"`
}

type callHierarchyData struct {
	AnalysisURI string                     `json:"analysisUri"`
	NodeID      sema.CallableID            `json:"nodeId"`
	Dispatch    sema.CallDispatchKind      `json:"dispatch,omitempty"`
	Execution   sema.CallExecutionRelation `json:"execution,omitempty"`
}

type callHierarchyItem struct {
	Name           string            `json:"name"`
	Kind           int               `json:"kind"`
	Detail         string            `json:"detail,omitempty"`
	URI            string            `json:"uri"`
	Range          lspRange          `json:"range"`
	SelectionRange lspRange          `json:"selectionRange"`
	Data           callHierarchyData `json:"data"`
}

type callHierarchyIncomingCall struct {
	From       callHierarchyItem `json:"from"`
	FromRanges []lspRange        `json:"fromRanges"`
}

type callHierarchyOutgoingCall struct {
	To         callHierarchyItem `json:"to"`
	FromRanges []lspRange        `json:"fromRanges"`
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

type didCloseParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type didSaveParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text,omitempty"`
}

type willSaveParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Reason       int                    `json:"reason"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type formattingParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type codeActionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        lspRange               `json:"range"`
	Context      codeActionContext      `json:"context"`
}

type codeActionContext struct {
	Diagnostics []diagnostic `json:"diagnostics"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

type workspaceEdit struct {
	Changes map[string][]textEdit `json:"changes"`
}

type codeAction struct {
	Title       string        `json:"title"`
	Kind        string        `json:"kind"`
	Diagnostics []diagnostic  `json:"diagnostics,omitempty"`
	Edit        workspaceEdit `json:"edit"`
}

type diagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity"`
	Code     string   `json:"code,omitempty"`
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
	in                *bufio.Reader //
	out               io.Writer     //
	documentSnapshots *lspserver.Documents
	diagnosticTimers  map[string]*time.Timer //
	diagnosticDelay   time.Duration          //
	writeMu           sync.Mutex             //
	timerMu           sync.Mutex             //
	shutdown          bool                   //
}

type formatterBranchContext struct {
	contentDepth int
	branchActive bool
	bodyExtra    bool
}

func main() {
	s := &server{
		in:                bufio.NewReader(os.Stdin),
		out:               os.Stdout,
		documentSnapshots: lspserver.NewDocuments(),
		diagnosticTimers:  map[string]*time.Timer{},
		diagnosticDelay:   600 * time.Millisecond,
	}
	if err := s.run(); err != nil {
		fmt.Fprintf(os.Stderr, "lsp error: %v\n", err)
		os.Exit(1)
	}
}

func (s *server) run() error {
	for {
		message, err := protocol.ReadMessage(s.in)
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
				"textDocumentSync": map[string]any{
					"openClose":         true,
					"change":            1,
					"willSave":          true,
					"willSaveWaitUntil": true,
					"save": map[string]any{
						"includeText": false,
					},
				},
				"documentFormattingProvider": true,
				"documentSymbolProvider":     true,
				"hoverProvider":              true,
				"definitionProvider":         true,
				"documentHighlightProvider":  true,
				"callHierarchyProvider":      true,
				"codeActionProvider":         true,
				"completionProvider": map[string]any{
					"triggerCharacters": []string{"."},
				},
				"semanticTokensProvider": map[string]any{
					"legend": map[string]any{
						"tokenTypes":     semanticTokenTypes,
						"tokenModifiers": semanticTokenModifiers,
					},
					"full":  true,
					"range": false,
				},
			},
			"serverInfo": map[string]any{
				"name":    "sec-lsp",
				"version": "0.1.0",
			},
		})
	case "initialized":
		return nil
	case "workspace/didChangeConfiguration", "workspace/didChangeWatchedFiles":
		// Analysis configuration is read from the project manifest for each
		// analysis. Re-publishing open documents applies a changed depth without
		// requiring a server restart and replaces facts computed at the old depth.
		return s.republishOpenDiagnostics()
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
		s.documentSnapshots.Open(params.TextDocument.URI, params.TextDocument.Version, params.TextDocument.Text)
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
		if _, changed := s.documentSnapshots.Change(params.TextDocument.URI, params.TextDocument.Version, text); !changed {
			return nil
		}
		s.scheduleDiagnostics(params.TextDocument.URI, text)
		return nil
	case "textDocument/didClose":
		var params didCloseParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		s.stopDiagnosticTimer(params.TextDocument.URI)
		s.documentSnapshots.Close(params.TextDocument.URI)
		return s.notify("textDocument/publishDiagnostics", map[string]any{
			"uri":         params.TextDocument.URI,
			"diagnostics": []diagnostic{},
		})
	case "textDocument/didSave":
		var params didSaveParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		snapshot, ok := s.documentSnapshots.Snapshot(params.TextDocument.URI)
		if !ok {
			return nil
		}
		return s.publishDiagnostics(snapshot.URI, snapshot.Text)
	case "textDocument/willSave":
		var params willSaveParams
		return json.Unmarshal(message.Params, &params)
	case "textDocument/willSaveWaitUntil":
		var params willSaveParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		return s.respond(message.ID, []textEdit{})
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
	case "textDocument/codeAction":
		var params codeActionParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		snapshot, ok := s.documentSnapshots.Snapshot(params.TextDocument.URI)
		if !ok {
			return s.respond(message.ID, []codeAction{})
		}
		return s.respond(message.ID, ownershipCodeActions(params.TextDocument.URI, snapshot.Text, params.Context.Diagnostics))
	case "textDocument/completion":
		var params completionParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}

		snapshot, ok := s.documentSnapshots.Snapshot(params.TextDocument.URI)
		if !ok {
			return s.respond(message.ID, []completionItem{})
		}

		offset := lineCharToOffset(snapshot.Text, params.Position.Line, params.Position.Character)
		items := completeSource(params.TextDocument.URI, snapshot.Text, offset)
		return s.respond(message.ID, items)
	case "textDocument/documentSymbol":
		var params documentSymbolParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		snapshot, ok := s.documentSnapshots.Snapshot(params.TextDocument.URI)
		if !ok {
			return s.respond(message.ID, []documentSymbol{})
		}
		return s.respond(message.ID, documentSymbolsForSource(params.TextDocument.URI, snapshot.Text))
	case "textDocument/semanticTokens/full":
		var params semanticTokensParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		snapshot, ok := s.documentSnapshots.Snapshot(params.TextDocument.URI)
		if !ok {
			return s.respond(message.ID, semanticTokens{})
		}
		return s.respond(message.ID, semanticTokensForSource(params.TextDocument.URI, snapshot.Text))
	case "textDocument/hover":
		var params hoverParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		snapshot, ok := s.documentSnapshots.Snapshot(params.TextDocument.URI)
		if !ok {
			return s.respond(message.ID, nil)
		}
		result, ok := hoverForSource(params.TextDocument.URI, snapshot.Text, params.Position)
		if !ok {
			return s.respond(message.ID, nil)
		}
		return s.respond(message.ID, result)
	case "textDocument/definition":
		var params definitionParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		snapshot, ok := s.documentSnapshots.Snapshot(params.TextDocument.URI)
		if !ok {
			return s.respond(message.ID, nil)
		}
		locations := definitionsForSource(params.TextDocument.URI, snapshot.Text, params.Position)
		if len(locations) == 0 {
			return s.respond(message.ID, nil)
		}
		return s.respond(message.ID, locations)
	case "textDocument/documentHighlight":
		var params documentHighlightParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		snapshot, ok := s.documentSnapshots.Snapshot(params.TextDocument.URI)
		if !ok {
			return s.respond(message.ID, []documentHighlight{})
		}
		return s.respond(message.ID, documentHighlightsForSource(params.TextDocument.URI, snapshot.Text, params.Position))
	case "textDocument/prepareCallHierarchy":
		var params callHierarchyPrepareParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		snapshot, ok := s.documentSnapshots.Snapshot(params.TextDocument.URI)
		if !ok {
			return s.respond(message.ID, []callHierarchyItem{})
		}
		return s.respond(message.ID, callHierarchyItemsForSource(params.TextDocument.URI, snapshot.Text, params.Position))
	case "callHierarchy/incomingCalls":
		var params callHierarchyIncomingCallsParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		analysisURI := params.Item.Data.AnalysisURI
		if analysisURI == "" {
			analysisURI = params.Item.URI
		}
		snapshot, ok := s.documentSnapshots.Snapshot(analysisURI)
		if !ok {
			return s.respond(message.ID, []callHierarchyIncomingCall{})
		}
		return s.respond(message.ID, callHierarchyIncomingCallsForSource(analysisURI, snapshot.Text, params.Item.Data.NodeID))
	case "callHierarchy/outgoingCalls":
		var params callHierarchyOutgoingCallsParams
		if err := json.Unmarshal(message.Params, &params); err != nil {
			return err
		}
		analysisURI := params.Item.Data.AnalysisURI
		if analysisURI == "" {
			analysisURI = params.Item.URI
		}
		snapshot, ok := s.documentSnapshots.Snapshot(analysisURI)
		if !ok {
			return s.respond(message.ID, []callHierarchyOutgoingCall{})
		}
		return s.respond(message.ID, callHierarchyOutgoingCallsForSource(analysisURI, snapshot.Text, params.Item.Data.NodeID))
	default:
		if len(message.ID) == 0 {
			return nil
		}
		return s.respondError(message.ID, -32601, "method not found")
	}
}

func callHierarchyItemsForSource(uri string, text string, pos position) (items []callHierarchyItem) {
	defer func() {
		if recover() != nil {
			items = []callHierarchyItem{}
		}
	}()
	analyzer := analyzeNavigationSource(uri, text)
	if analyzer == nil {
		return []callHierarchyItem{}
	}
	token, ok := sourceTokenAtPosition(uri, text, pos)
	if !ok {
		return []callHierarchyItem{}
	}
	definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(token.File, token.Line, token.Column))
	if len(definitions) != 1 {
		return []callHierarchyItem{}
	}
	for _, node := range analyzer.CallGraph().NodesForDeclaration(definitions[0]) {
		items = append(items, callHierarchyItemForNode(uri, node))
	}
	return items
}

func callHierarchyIncomingCallsForSource(uri string, text string, nodeID sema.CallableID) (calls []callHierarchyIncomingCall) {
	defer func() {
		if recover() != nil {
			calls = []callHierarchyIncomingCall{}
		}
	}()
	analyzer := analyzeNavigationSource(uri, text)
	if analyzer == nil {
		return []callHierarchyIncomingCall{}
	}
	graph := analyzer.CallGraph()
	grouped := map[string]int{}
	for _, site := range graph.Incoming(nodeID) {
		caller, ok := graph.Node(site.Caller)
		if !ok {
			continue
		}
		rng := definitionTokenRange(site.Source)
		key := callHierarchyRelationshipKey(caller.ID, site.Dispatch, site.Execution)
		if index, exists := grouped[key]; exists {
			calls[index].FromRanges = append(calls[index].FromRanges, rng)
			continue
		}
		grouped[key] = len(calls)
		calls = append(calls, callHierarchyIncomingCall{
			From:       callHierarchyItemForRelationship(uri, caller, site.Dispatch, site.Execution),
			FromRanges: []lspRange{rng},
		})
	}
	return calls
}

func callHierarchyOutgoingCallsForSource(uri string, text string, nodeID sema.CallableID) (calls []callHierarchyOutgoingCall) {
	defer func() {
		if recover() != nil {
			calls = []callHierarchyOutgoingCall{}
		}
	}()
	analyzer := analyzeNavigationSource(uri, text)
	if analyzer == nil {
		return []callHierarchyOutgoingCall{}
	}
	graph := analyzer.CallGraph()
	grouped := map[string]int{}
	for _, site := range graph.Outgoing(nodeID) {
		for _, targetID := range site.Targets {
			target, ok := graph.Node(targetID)
			if !ok || target.Declaration.Line <= 0 {
				continue
			}
			rng := definitionTokenRange(site.Source)
			key := callHierarchyRelationshipKey(target.ID, site.Dispatch, site.Execution)
			if index, exists := grouped[key]; exists {
				calls[index].FromRanges = append(calls[index].FromRanges, rng)
				continue
			}
			grouped[key] = len(calls)
			calls = append(calls, callHierarchyOutgoingCall{
				To:         callHierarchyItemForRelationship(uri, target, site.Dispatch, site.Execution),
				FromRanges: []lspRange{rng},
			})
		}
	}
	return calls
}

func callHierarchyRelationshipKey(node sema.CallableID, dispatch sema.CallDispatchKind, execution sema.CallExecutionRelation) string {
	return fmt.Sprintf("%s|%s|%s", node, dispatch, execution)
}

func callHierarchyItemForRelationship(analysisURI string, node sema.CallableNode, dispatch sema.CallDispatchKind, execution sema.CallExecutionRelation) callHierarchyItem {
	item := callHierarchyItemForNode(analysisURI, node)
	item.Data.Dispatch = dispatch
	item.Data.Execution = execution
	if execution != "" && execution != sema.CallExecutionSynchronous {
		label := strings.ReplaceAll(string(execution), "-", " ")
		if item.Detail == "" {
			item.Detail = label
		} else {
			item.Detail += " | " + label
		}
	}
	return item
}

func analyzeNavigationSource(uri string, text string) *sema.Analyzer {
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return nil
	}
	path := pathFromURI(uri)
	if uri != "" {
		resolveCoreSources(program, path)
		resolveSourceImports(program, map[string]bool{}, path)
	}
	analyzer := newLSPAnalyzer(uri)
	analyzer.Analyze(program)
	return analyzer
}

func callHierarchyItemForNode(analysisURI string, node sema.CallableNode) callHierarchyItem {
	uri := analysisURI
	if node.Declaration.File != "" {
		uri = uriFromPath(node.Declaration.File)
	}
	rng := definitionTokenRange(node.Declaration)
	name := node.Name
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	detail := node.Module
	kind := 12 // SymbolKind.Function
	if node.ImplTarget != "" {
		detail = "impl " + node.ImplTarget
		kind = 6 // SymbolKind.Method
	}
	return callHierarchyItem{
		Name:           name,
		Kind:           kind,
		Detail:         detail,
		URI:            uri,
		Range:          rng,
		SelectionRange: rng,
		Data:           callHierarchyData{AnalysisURI: analysisURI, NodeID: node.ID},
	}
}

const (
	documentHighlightRead  = 2
	documentHighlightWrite = 3
)

func documentHighlightsForSource(uri string, text string, pos position) (highlights []documentHighlight) {
	defer func() {
		if recover() != nil {
			highlights = []documentHighlight{}
		}
	}()

	program := parseProgramForLSP(uri, text)
	if program == nil {
		return []documentHighlight{}
	}
	path := pathFromURI(uri)
	if uri != "" {
		resolveCoreSources(program, path)
		resolveSourceImports(program, map[string]bool{}, path)
	}
	analyzer := newLSPAnalyzer(uri)
	analyzer.Analyze(program)

	use, ok := sourceTokenAtPosition(uri, text, pos)
	if !ok {
		return []documentHighlight{}
	}
	definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(use.File, use.Line, use.Column))
	// A highlight must never combine separate overloads or otherwise ambiguous symbols.
	if len(definitions) != 1 {
		return []documentHighlight{}
	}
	definition := definitions[0]
	tokens := sourceTokens(uri, text)
	for index, token := range tokens {
		bound := uniqueDefinitionTokens(analyzer.DefinitionsAt(token.File, token.Line, token.Column))
		if len(bound) != 1 || !sameSourceToken(bound[0], definition) {
			continue
		}
		kind := documentHighlightRead
		if sameSourceToken(token, definition) || tokenIsDirectAssignmentTarget(tokens, index) {
			kind = documentHighlightWrite
		}
		highlights = append(highlights, documentHighlight{Range: tokenRange(token), Kind: kind})
	}
	return highlights
}

func sourceTokens(uri string, text string) []lexer.Token {
	l := lexer.New(text)
	if uri != "" {
		l = lexer.NewWithFile(text, pathFromURI(uri))
	}
	tokens := make([]lexer.Token, 0)
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			return tokens
		}
		tokens = append(tokens, token)
	}
}

func uniqueDefinitionTokens(tokens []lexer.Token) []lexer.Token {
	unique := make([]lexer.Token, 0, len(tokens))
	seen := map[string]bool{}
	for _, token := range tokens {
		key := fmt.Sprintf("%s:%d:%d", token.File, token.Line, token.Column)
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, token)
	}
	return unique
}

func sameSourceToken(left lexer.Token, right lexer.Token) bool {
	return left.File == right.File && left.Line == right.Line && left.Column == right.Column
}

func tokenIsDirectAssignmentTarget(tokens []lexer.Token, index int) bool {
	if index+1 >= len(tokens) {
		return false
	}
	switch tokens[index+1].Type {
	case lexer.ASSIGN, lexer.MOVE_ASSIGN,
		lexer.PLUS_ASSIGN, lexer.MINUS_ASSIGN, lexer.ASTERISK_ASSIGN,
		lexer.SLASH_ASSIGN, lexer.PERCENT_ASSIGN, lexer.BIT_AND_ASSIGN,
		lexer.BIT_OR_ASSIGN, lexer.BIT_XOR_ASSIGN, lexer.SHIFT_LEFT_ASSIGN,
		lexer.SHIFT_RIGHT_ASSIGN:
		return true
	default:
		return false
	}
}

func definitionsForSource(uri string, text string, pos position) (locations []location) {
	defer func() {
		if recover() != nil {
			locations = nil
		}
	}()

	program := parseProgramForLSP(uri, text)
	if program == nil {
		return nil
	}
	path := pathFromURI(uri)
	if uri != "" {
		resolveCoreSources(program, path)
		resolveSourceImports(program, map[string]bool{}, path)
	}
	analyzer := newLSPAnalyzer(uri)
	analyzer.Analyze(program)

	use, ok := sourceTokenAtPosition(uri, text, pos)
	if !ok {
		return nil
	}
	definitions := analyzer.DefinitionsAt(use.File, use.Line, use.Column)
	seen := map[string]bool{}
	for _, definition := range definitions {
		definitionURI := uri
		if definition.File != "" {
			definitionURI = uriFromPath(definition.File)
		}
		rng := definitionTokenRange(definition)
		key := fmt.Sprintf("%s:%d:%d", definitionURI, rng.Start.Line, rng.Start.Character)
		if seen[key] {
			continue
		}
		seen[key] = true
		locations = append(locations, location{URI: definitionURI, Range: rng})
	}
	return locations
}

func sourceTokenAtPosition(uri string, text string, pos position) (lexer.Token, bool) {
	l := lexer.New(text)
	if uri != "" {
		l = lexer.NewWithFile(text, pathFromURI(uri))
	}
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			return lexer.Token{}, false
		}
		rng := tokenRange(token)
		if comparePosition(pos, rng.Start) >= 0 && comparePosition(pos, rng.End) < 0 {
			return token, true
		}
	}
}

func definitionTokenRange(token lexer.Token) lspRange {
	rng := tokenRange(token)
	// Imported declarations are semantically qualified in the combined AST.
	// Their source token still starts at the unqualified declaration name.
	if index := strings.LastIndex(token.Lexeme, "."); index >= 0 && index+1 < len(token.Lexeme) {
		rng.End.Character = rng.Start.Character + len([]rune(token.Lexeme[index+1:]))
	}
	return rng
}

func completeSource(uri string, text string, offset int) []completionItem {
	context := completionContextAt(text, offset)
	parseText := completionParseText(text, offset, context)

	l := lexer.New(parseText)
	if uri != "" {
		l = lexer.NewWithFile(parseText, pathFromURI(uri))
	}
	p := parser.New(l)
	parseResult := p.Parse()
	fileAST := parseResult.Program

	analyzer := newLSPAnalyzer(uri)
	analyzed := false
	if fileAST != nil && !parseResult.HasErrors {
		if uri != "" {
			resolveCoreSources(fileAST, pathFromURI(uri))
			resolveSourceImports(fileAST, map[string]bool{}, pathFromURI(uri))
		}
		analyzer.Analyze(fileAST)
		analyzed = true
		if expected, ok := expectedReturnTypeAt(fileAST, analyzer, parseText, offset); ok {
			context.ExpectedType = &expected
		}
	}

	if context.Member {
		if fileAST == nil || !analyzed {
			return []completionItem{}
		}
		targetExpr := findSelectorLHS(fileAST, text, context.DotOffset)
		if targetExpr == nil {
			return []completionItem{}
		}
		if identifier, ok := targetExpr.(*ast.Identifier); ok {
			if staticType, exists := analyzer.Types()[identifier.Value]; exists {
				return memberCompletionItems(staticType, analyzer.Functions(), analyzer.Symbols(), context.Prefix, true)
			}
		}
		exprType, ok := analyzer.TypeOf(targetExpr)
		if !ok {
			return []completionItem{}
		}
		return memberCompletionItems(exprType, analyzer.Functions(), analyzer.Symbols(), context.Prefix, false)
	}

	return globalCompletionItems(text, analyzer, context)
}

// completionParseText supplies the smallest missing syntax needed to analyze a
// member selector while the user is still typing an incomplete control statement.
func completionParseText(text string, offset int, context completionContext) string {
	if !context.Member {
		return text
	}
	if offset < 0 {
		offset = 0
	}
	if offset > len(text) {
		offset = len(text)
	}

	insertion := ""
	if context.Prefix == "" {
		insertion = "__sec_completion"
	}
	if incompleteIfConditionAt(text, offset, context.DotOffset) {
		insertion += " {}"
	}
	if insertion == "" {
		return text
	}
	return text[:offset] + insertion + text[offset:]
}

func incompleteIfConditionAt(text string, offset int, dotOffset int) bool {
	if dotOffset < 0 || dotOffset > len(text) || offset < dotOffset || offset > len(text) {
		return false
	}
	lineStart := strings.LastIndex(text[:dotOffset], "\n") + 1
	beforeSelector := strings.TrimSpace(text[lineStart:dotOffset])
	if !strings.HasPrefix(beforeSelector, "if ") {
		return false
	}
	lineEnd := strings.IndexByte(text[offset:], '\n')
	if lineEnd < 0 {
		lineEnd = len(text)
	} else {
		lineEnd += offset
	}
	return !strings.Contains(text[offset:lineEnd], "{")
}

var semanticTokenTypes = features.SemanticTokenTypes
var semanticTokenModifiers = features.SemanticTokenModifiers

var semanticTokenTypeIndex = func() map[string]int {
	indexes := map[string]int{}
	for i, tokenType := range semanticTokenTypes {
		indexes[tokenType] = i
	}
	return indexes
}()

func documentSymbolsForSource(uri string, text string) []documentSymbol {
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return []documentSymbol{}
	}
	symbols := []documentSymbol{}
	for _, stmt := range program.Statements {
		if symbol, ok := documentSymbolForStatement(stmt); ok {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

func documentSymbolForStatement(stmt ast.Statement) (documentSymbol, bool) {
	switch stmt := stmt.(type) {
	case *ast.ModuleStatement:
		if stmt == nil {
			return documentSymbol{}, false
		}
		return namedDocumentSymbol(stmt.Path, "module", 2, stmt.Token, stmt.Token), true
	case *ast.TypeDeclStatement:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		kind := 5
		detail := "type"
		if stmt.StructType != nil {
			kind = 23
			detail = "struct"
		} else if stmt.Union {
			kind = 23
			detail = "union"
		} else if len(stmt.Variants) > 0 {
			kind = 10
			detail = "enum"
		}
		return namedDocumentSymbol(stmt.Name.Value, detail, kind, stmt.Token, stmt.Name.Token), true
	case *ast.UnitDeclStatement:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		return namedDocumentSymbol(stmt.Name.Value, "unit", 14, stmt.Token, stmt.Name.Token), true
	case *ast.EnumDeclaration:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		symbol := namedDocumentSymbol(stmt.Name.Value, "enum", 10, stmt.Token, stmt.Name.Token)
		for _, value := range stmt.Values {
			if value != nil && value.Name != nil {
				symbol.Children = append(symbol.Children, namedDocumentSymbol(value.Name.Value, "enum member", 22, value.Token, value.Name.Token))
			}
		}
		return symbol, true
	case *ast.InterfaceDeclaration:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		symbol := namedDocumentSymbol(stmt.Name.Value, "interface", 11, stmt.Token, stmt.Name.Token)
		for _, method := range stmt.Methods {
			if method != nil && method.Name != nil {
				symbol.Children = append(symbol.Children, functionDocumentSymbol(method, 6))
			}
		}
		for _, property := range stmt.Properties {
			if property != nil && property.Name != nil {
				symbol.Children = append(symbol.Children, namedDocumentSymbol(property.Name.Value, typeReferenceName(property.Type), 7, property.Token, property.Name.Token))
			}
		}
		for _, event := range stmt.Events {
			if event != nil && event.Name != nil {
				symbol.Children = append(symbol.Children, namedDocumentSymbol(event.Name.Value, typeReferenceName(event.Payload), 24, event.Token, event.Name.Token))
			}
		}
		return symbol, true
	case *ast.FunctionDeclaration:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		return functionDocumentSymbol(stmt, 12), true
	case *ast.StructStatement:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		symbol := namedDocumentSymbol(stmt.Name.Value, "struct", 23, stmt.Token, stmt.Name.Token)
		for _, field := range stmt.Fields {
			if field.Name != nil {
				symbol.Children = append(symbol.Children, namedDocumentSymbol(field.Name.Value, typeReferenceName(field.Type), 8, field.Token, field.Name.Token))
			}
		}
		return symbol, true
	case *ast.LetStatement:
		if stmt == nil || stmt.Name == nil {
			return documentSymbol{}, false
		}
		return namedDocumentSymbol(stmt.Name.Value, typeReferenceName(stmt.Type), 13, stmt.Token, stmt.Name.Token), true
	case *ast.LetGroupStatement:
		if stmt == nil || len(stmt.Lets) == 0 {
			return documentSymbol{}, false
		}
		group := namedDocumentSymbol("let", "variables", 13, stmt.Token, stmt.Token)
		for _, let := range stmt.Lets {
			if let != nil && let.Name != nil {
				group.Children = append(group.Children, namedDocumentSymbol(let.Name.Value, typeReferenceName(let.Type), 13, let.Token, let.Name.Token))
			}
		}
		return group, true
	case *ast.ImplStatement:
		if stmt == nil || stmt.Target == nil {
			return documentSymbol{}, false
		}
		name := "impl " + stmt.Target.Name
		if stmt.Extends {
			name = "impl extends " + stmt.Target.Name
		} else if len(stmt.Implements) > 0 {
			interfaces := make([]string, 0, len(stmt.Implements))
			for _, ref := range stmt.Implements {
				interfaces = append(interfaces, typeReferenceName(ref))
			}
			name += " implements " + strings.Join(interfaces, ", ")
		}
		symbol := namedDocumentSymbol(name, "impl", 3, stmt.Token, stmt.Target.Token)
		for _, member := range stmt.Members {
			if child, ok := documentSymbolForImplMember(member); ok {
				symbol.Children = append(symbol.Children, child)
			}
		}
		return symbol, true
	default:
		return documentSymbol{}, false
	}
}

func documentSymbolForImplMember(member ast.ImplMember) (documentSymbol, bool) {
	switch member := member.(type) {
	case *ast.FunctionDeclaration:
		if member == nil || member.Name == nil {
			return documentSymbol{}, false
		}
		return functionDocumentSymbol(member, 6), true
	case *ast.InitDeclaration:
		if member == nil {
			return documentSymbol{}, false
		}
		detail := "constructs"
		if member.ErrorType != nil {
			detail += "; error " + typeReferenceName(member.ErrorType)
		}
		return namedDocumentSymbol("init", detail, 9, member.Token, member.Token), true
	case *ast.PropertyDeclaration:
		if member == nil || member.Name == nil {
			return documentSymbol{}, false
		}
		return namedDocumentSymbol(member.Name.Value, typeReferenceName(member.Type), 7, member.Token, member.Name.Token), true
	case *ast.EventDeclaration:
		if member == nil || member.Name == nil {
			return documentSymbol{}, false
		}
		return namedDocumentSymbol(member.Name.Value, "event", 24, member.Token, member.Name.Token), true
	case *ast.TypeDeclStatement:
		return documentSymbolForStatement(member)
	case *ast.UnitDeclStatement:
		return documentSymbolForStatement(member)
	case *ast.EnumDeclaration:
		return documentSymbolForStatement(member)
	case *ast.LetStatement:
		return documentSymbolForStatement(member)
	default:
		return documentSymbol{}, false
	}
}

func functionDocumentSymbol(fn *ast.FunctionDeclaration, kind int) documentSymbol {
	detail := "fn"
	if fn.ReturnType != nil {
		detail += " " + typeReferenceName(fn.ReturnType)
	}
	return namedDocumentSymbol(fn.Name.Value, detail, kind, fn.Token, fn.Name.Token)
}

func namedDocumentSymbol(name string, detail string, kind int, token lexer.Token, selectionToken lexer.Token) documentSymbol {
	selectionRange := tokenRange(selectionToken)
	return documentSymbol{
		Name:           name,
		Detail:         detail,
		Kind:           kind,
		Range:          rangeContaining(tokenRange(token), selectionRange),
		SelectionRange: selectionRange,
	}
}

func semanticTokensForSource(uri string, text string) semanticTokens {
	tokenTypes := semanticTokenClassification(uri, text)
	return semanticTokens{Data: features.SemanticTokens(text, pathFromURI(uri), tokenTypes)}
}

func semanticTokenClassification(uri string, text string) (classification map[string]string) {
	classification = map[string]string{}
	defer func() {
		if recover() != nil {
			// Semantic analysis is best-effort for an actively edited document.
			// Keep the server alive and let lexical token classification continue.
			classification = map[string]string{}
		}
	}()

	program := parseProgramForLSP(uri, text)
	if program == nil {
		return classification
	}
	path := pathFromURI(uri)
	resolveCoreSources(program, path)
	resolveSourceImports(program, map[string]bool{}, path)
	analyzer := newLSPAnalyzer(uri)
	analyzer.Analyze(program)
	for name := range analyzer.Types() {
		classification[name] = "type"
	}
	for name := range analyzer.Functions() {
		classification[name] = "function"
		if dot := strings.LastIndex(name, "."); dot >= 0 && dot+1 < len(name) {
			classification[name[dot+1:]] = "method"
		}
	}
	for name, symbol := range analyzer.Symbols() {
		if strings.Contains(name, ".") {
			continue
		}
		if symbol.ImplicitMember {
			classification[name] = "property"
		} else if !symbol.Mutable {
			classification[name] = "variable readonly"
		} else if symbol.Local {
			classification[name] = "variable"
		} else {
			classification[name] = "variable"
		}
	}
	for key, kind := range contextualKeywordClassifications(program) {
		classification[key] = kind
	}
	declarationKinds := semanticDeclarationKinds(analyzer)
	for _, token := range sourceTokens(uri, text) {
		if token.Type != lexer.IDENT && token.Type != lexer.SELF {
			continue
		}
		positionKey := features.ClassificationKey(token.File, token.Line, token.Column)
		if classification[positionKey] == "keyword" {
			continue
		}
		if member, ok := analyzer.CompilerKnownMemberAt(token.File, token.Line, token.Column); ok {
			kind := "method"
			if member.Kind == sema.CompilerKnownProperty {
				kind = "property"
			}
			classification[positionKey] = kind
			continue
		}
		binding, ok := analyzer.ResolvedBindingAt(token.File, token.Line, token.Column)
		if ok {
			kind := "variable"
			if !binding.Mutable {
				kind += " readonly"
			}
			classification[positionKey] = kind
			continue
		}
		if kind := declarationKinds[positionKey]; kind != "" {
			classification[positionKey] = kind
			continue
		}
		definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(token.File, token.Line, token.Column))
		if len(definitions) != 1 {
			continue
		}
		definition := definitions[0]
		if kind := declarationKinds[features.ClassificationKey(definition.File, definition.Line, definition.Column)]; kind != "" {
			classification[positionKey] = kind
		}
	}
	return classification
}

func contextualKeywordClassifications(program *ast.Program) map[string]string {
	classification := map[string]string{}
	if program == nil {
		return classification
	}
	setKeyword := func(token lexer.Token) {
		if token.Line <= 0 || token.Column <= 0 {
			return
		}
		classification[features.ClassificationKey(token.File, token.Line, token.Column)] = "keyword"
	}
	setParameter := func(identifier *ast.Identifier) {
		if identifier == nil || identifier.Token.Line <= 0 || identifier.Token.Column <= 0 {
			return
		}
		classification[features.ClassificationKey(identifier.Token.File, identifier.Token.Line, identifier.Token.Column)] = "parameter"
	}
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.InterfaceDeclaration:
			if statement == nil {
				continue
			}
			for _, property := range statement.Properties {
				if property != nil && property.RequiresSet {
					setKeyword(property.SetToken)
					setParameter(property.SetterParameter)
				}
			}
		case *ast.ImplStatement:
			if statement == nil {
				continue
			}
			for _, member := range statement.Members {
				switch member := member.(type) {
				case *ast.PropertyDeclaration:
					if member != nil && member.Setter != nil {
						setKeyword(member.Setter.Token)
					}
				case *ast.InitDeclaration:
					if member != nil {
						setKeyword(member.Token)
					}
				}
			}
		}
	}
	return classification
}

func semanticDeclarationKinds(analyzer *sema.Analyzer) map[string]string {
	kinds := map[string]string{}
	set := func(token lexer.Token, kind string) {
		if token.Line <= 0 || token.Column <= 0 {
			return
		}
		kinds[features.ClassificationKey(token.File, token.Line, token.Column)] = kind
	}
	for _, typ := range analyzer.Types() {
		for _, field := range typ.Fields {
			set(field.Token, "property")
		}
		for _, field := range typ.RegisterFields {
			set(field.Token, "property")
		}
		for _, property := range typ.Properties {
			set(property.Token, "property")
		}
		for _, event := range typ.Events {
			set(event.Token, "event")
		}
		for _, value := range typ.EnumConsts {
			set(value.Token, "enumMember")
		}
	}
	for _, overloads := range analyzer.Functions() {
		for _, function := range overloads {
			kind := "function"
			if function.ImplTarget != "" {
				kind = "method"
			}
			set(function.Token, kind)
		}
	}
	return kinds
}

func semanticTokenType(token lexer.Token, classification map[string]string) string {
	switch token.Type {
	case lexer.COMMENT:
		return "comment"
	case lexer.STRING, lexer.CHAR, lexer.RAW_STRING, lexer.INTERPSTRING:
		return "string"
	case lexer.INT, lexer.FLOAT:
		return "number"
	case lexer.IDENT, lexer.SELF:
		if classified := classification[token.Lexeme]; classified != "" {
			return classified
		}
		return "variable"
	case lexer.ASSIGN, lexer.DECLARE, lexer.MOVE_ASSIGN, lexer.MOVE_DECLARE, lexer.ARROW, lexer.CONSUME_ARROW, lexer.PLUS, lexer.MINUS, lexer.ASTERISK, lexer.SLASH, lexer.PERCENT,
		lexer.PLUS_ASSIGN, lexer.MINUS_ASSIGN, lexer.ASTERISK_ASSIGN, lexer.SLASH_ASSIGN, lexer.PERCENT_ASSIGN,
		lexer.EQ, lexer.NEQ, lexer.LT, lexer.LTE, lexer.GT, lexer.GTE, lexer.AND, lexer.OR, lexer.NOT,
		lexer.BIT_AND, lexer.BIT_OR, lexer.BIT_XOR, lexer.BIT_NOT, lexer.SHIFT_LEFT, lexer.SHIFT_RIGHT,
		lexer.BIT_AND_ASSIGN, lexer.BIT_OR_ASSIGN, lexer.BIT_XOR_ASSIGN, lexer.SHIFT_LEFT_ASSIGN, lexer.SHIFT_RIGHT_ASSIGN,
		lexer.DOT, lexer.RANGE, lexer.RANGE_EXCLUSIVE, lexer.SPREAD, lexer.COLON:
		return "operator"
	case lexer.COMMA, lexer.SEMICOLON, lexer.QUESTION, lexer.UNDERSCORE, lexer.AT, lexer.HASH,
		lexer.LPAREN, lexer.RPAREN, lexer.LBRACE, lexer.RBRACE, lexer.LBRACKET, lexer.RBRACKET:
		return ""
	default:
		return "keyword"
	}
}

func semanticTokenLength(token lexer.Token) int {
	if strings.Contains(token.Lexeme, "\n") {
		return len([]rune(strings.Split(token.Lexeme, "\n")[0]))
	}
	return len([]rune(token.Lexeme))
}

func hoverForSource(uri string, text string, pos position) (hoverResult, bool) {
	program := parseProgramForLSP(uri, text)
	if program == nil {
		return hoverResult{}, false
	}
	offset := lineCharToOffset(text, pos.Line, pos.Character)
	if offset < 0 {
		return hoverResult{}, false
	}

	path := pathFromURI(uri)
	if uri != "" {
		resolveCoreSources(program, path)
		resolveSourceImports(program, map[string]bool{}, path)
	}
	analyzer := newLSPAnalyzer(uri)
	analyzer.Analyze(program)

	name, nameStart, nameEnd, ok := identifierAtOffset(text, offset)
	if !ok {
		return hoverResult{}, false
	}
	nameRange := offsetsRange(text, nameStart, nameEnd)

	if target, ok := implTargetAtOffset(program, analyzer, text, path, offset); ok {
		if name == "self" {
			return typedHover(nameRange, "self", target), true
		}
		if isSelfMemberSelector(text, nameStart) {
			if contents, ok := selfMemberHoverContents(target, name, analyzer.Functions(), text, path); ok {
				contents += callGraphHoverSuffix(analyzer, uri, text, pos)
				return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: nameRange}, true
			}
		}
	}
	if token, found := sourceTokenAtPosition(uri, text, pos); found {
		if member, resolved := analyzer.CompilerKnownMemberAt(token.File, token.Line, token.Column); resolved {
			return compilerKnownMemberHover(nameRange, member), true
		}
		definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(token.File, token.Line, token.Column))
		if len(definitions) == 1 {
			if contents, found := memberHoverContentsForDefinition(analyzer.Types(), definitions[0]); found {
				return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: nameRange}, true
			}
		}
	}

	if functions := analyzer.Functions()[name]; len(functions) > 0 {
		contents := functionHoverContents(functions, text, path)
		contents += callGraphHoverSuffix(analyzer, uri, text, pos)
		return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: nameRange}, true
	}
	if symbol, ok := analyzer.Symbols()[name]; ok {
		return typedHover(nameRange, symbol.Name, symbol.Type), true
	}
	if typ, ok := analyzer.Types()[name]; ok {
		return typedHover(nameRange, "type "+name, typ), true
	}

	return hoverResult{}, false
}

func compilerKnownMemberHover(sourceRange lspRange, member sema.CompilerKnownMember) hoverResult {
	kind := string(member.Kind)
	if member.Unsafe {
		kind = "unsafe " + kind
	}
	result := lspTypeName(member.Result)
	if member.Result.Kind == sema.InvalidType || member.Result.Kind == "" {
		result = "context-dependent"
	}
	contents := fmt.Sprintf("```sec\n%s %s: %s\n```\n\nCompiler-known `%s`.", kind, member.Name, result, member.ID)
	return hoverResult{Contents: markupContent{Kind: "markdown", Value: contents}, Range: sourceRange}
}

func memberHoverContentsForDefinition(types map[string]sema.Type, definition lexer.Token) (string, bool) {
	for _, typ := range types {
		for _, field := range typ.Fields {
			if sameSourceToken(field.Token, definition) {
				return fmt.Sprintf("```sec\nfield %s: %s\n```", field.Name, lspTypeName(field.Type)), true
			}
		}
		for _, field := range typ.RegisterFields {
			if sameSourceToken(field.Token, definition) {
				return fmt.Sprintf("```sec\nregister field %s: %s\n```", field.Name, lspTypeName(field.Type)), true
			}
		}
		for _, property := range typ.Properties {
			if sameSourceToken(property.Token, definition) {
				return fmt.Sprintf("```sec\nproperty %s: %s\n```", property.Name, lspTypeName(property.Type)), true
			}
		}
		for _, event := range typ.Events {
			if sameSourceToken(event.Token, definition) {
				return fmt.Sprintf("```sec\nevent %s: %s\n```", event.Name, lspTypeName(event.Type)), true
			}
		}
	}
	return "", false
}

func callGraphHoverSuffix(analyzer *sema.Analyzer, uri string, text string, pos position) string {
	if analyzer == nil {
		return ""
	}
	token, ok := sourceTokenAtPosition(uri, text, pos)
	if !ok {
		return ""
	}
	definitions := uniqueDefinitionTokens(analyzer.DefinitionsAt(token.File, token.Line, token.Column))
	if len(definitions) != 1 {
		return ""
	}
	graph := analyzer.CallGraph()
	nodes := graph.NodesForDeclaration(definitions[0])
	if len(nodes) != 1 {
		return ""
	}
	node := nodes[0]
	incoming := graph.Incoming(node.ID)
	outgoing := graph.Outgoing(node.ID)
	callerCount := distinctCallers(incoming)
	calleeCount := distinctCallees(outgoing)

	lines := []string{
		"**Call graph**",
		fmt.Sprintf("Incoming: `%d` call sites from `%d` callables", len(incoming), callerCount),
		fmt.Sprintf("Outgoing: `%d` call sites to `%d` callables", len(outgoing), calleeCount),
	}
	roots := graph.RootsReaching(node.ID)
	if len(roots) == 0 {
		lines = append(lines, "Reachability: no active root in the current analysis")
	} else {
		rootNames := make([]string, 0, len(roots))
		for _, root := range roots {
			rootNames = append(rootNames, string(root.Kind))
		}
		lines = append(lines, "Reachable from: `"+strings.Join(rootNames, "`, `")+"`")
	}
	if graph.IsSameStackRecursive(node.ID) {
		members := graph.SameStackSCC(node.ID)
		names := make([]string, 0, len(members))
		for _, member := range members {
			names = append(names, member.Name)
		}
		lines = append(lines, "Same-stack recursion: `"+strings.Join(names, "`, `")+"`")
	}
	spawnCounts := map[sema.CallExecutionRelation]int{}
	for _, site := range outgoing {
		switch site.Execution {
		case sema.CallExecutionSpawnTask, sema.CallExecutionSpawnThread, sema.CallExecutionSpawnProcess:
			spawnCounts[site.Execution]++
		}
	}
	if len(spawnCounts) > 0 {
		parts := make([]string, 0, len(spawnCounts))
		for _, execution := range []sema.CallExecutionRelation{
			sema.CallExecutionSpawnTask,
			sema.CallExecutionSpawnThread,
			sema.CallExecutionSpawnProcess,
		} {
			if count := spawnCounts[execution]; count > 0 {
				parts = append(parts, fmt.Sprintf("%s: `%d`", strings.ReplaceAll(string(execution), "-", " "), count))
			}
		}
		lines = append(lines, "Execution edges: "+strings.Join(parts, ", "))
	}
	arenaSummary := graph.ArenaSummary(node.ID)
	if len(arenaSummary.DirectEffects) > 0 {
		effects := make([]string, 0, len(arenaSummary.DirectEffects))
		for _, effect := range arenaSummary.DirectEffects {
			name := string(effect.Kind)
			if effect.Arena != "" {
				name += "(" + effect.Arena + ")"
			}
			effects = append(effects, name)
		}
		lines = append(lines, "Direct Arena effects: `"+strings.Join(effects, "`, `")+"`")
	}
	if arenaSummary.MayAllocate {
		path := make([]string, 0, len(arenaSummary.AllocationPath))
		for _, id := range arenaSummary.AllocationPath {
			if member, ok := graph.Node(id); ok {
				path = append(path, member.Name)
			}
		}
		lines = append(lines, "May allocate: `yes`")
		if len(path) > 0 {
			lines = append(lines, "Allocation path: `"+strings.Join(path, "` -> `")+"`")
		}
	}
	return "\n\n" + strings.Join(lines, "\n\n")
}

func distinctCallers(sites []sema.CallSite) int {
	callers := map[sema.CallableID]bool{}
	for _, site := range sites {
		callers[site.Caller] = true
	}
	return len(callers)
}

func distinctCallees(sites []sema.CallSite) int {
	callees := map[sema.CallableID]bool{}
	for _, site := range sites {
		for _, target := range site.Targets {
			callees[target] = true
		}
	}
	return len(callees)
}

func typedHover(rng lspRange, name string, typ sema.Type) hoverResult {
	contents := fmt.Sprintf("```sec\n%s: %s\n```", name, lspTypeName(typ))
	if value, source, ok := sema.DefaultValuePreview(typ, 8); ok {
		contents += fmt.Sprintf("\n\nDefault: `%s`\n\nSource: `%s`", value, source)
	} else {
		contents += "\n\nDefault: _none_"
	}
	return hoverResult{
		Contents: markupContent{Kind: "markdown", Value: contents},
		Range:    rng,
	}
}

func selfMemberHoverContents(target sema.Type, name string, functions map[string][]sema.Function, text string, sourcePath string) (string, bool) {
	for _, field := range target.Fields {
		if field.Name == name {
			return fmt.Sprintf("```sec\nfield %s: %s\n```", name, lspTypeName(field.Type)), true
		}
	}
	for _, field := range target.RegisterFields {
		if field.Name == name {
			return fmt.Sprintf("```sec\nregister field %s: %s\n```", name, lspTypeName(field.Type)), true
		}
	}
	for _, property := range target.Properties {
		if property.Name == name {
			return fmt.Sprintf("```sec\nproperty %s: %s\n```", name, lspTypeName(property.Type)), true
		}
	}
	for _, event := range target.Events {
		if event.Name == name {
			return fmt.Sprintf("```sec\nevent %s: %s\n```", name, lspTypeName(event.Type)), true
		}
	}
	if overloads := functions[target.Name+"."+name]; len(overloads) > 0 {
		return functionHoverContents(overloads, text, sourcePath), true
	}
	return "", false
}

func functionHoverContents(functions []sema.Function, text string, sourcePath string) string {
	lines := make([]string, 0, len(functions)+2)
	for _, function := range functions {
		params := make([]string, 0, len(function.Parameters))
		for _, parameter := range function.Parameters {
			prefix := ""
			if parameter.Ref {
				prefix = "ref "
				if parameter.MutableRef {
					prefix = "ref mut "
				}
			}
			params = append(params, fmt.Sprintf("%s%s: %s", prefix, parameter.Name, lspTypeName(parameter.Type)))
		}
		lines = append(lines, fmt.Sprintf("fn %s(%s) %s", function.Name, strings.Join(params, ", "), lspTypeName(function.ReturnType)))
	}
	contents := "```sec\n" + strings.Join(lines, "\n") + "\n```"
	if len(functions) == 1 && functionSourceMatches(functions[0], sourcePath) {
		if doc := functionDocCommentAbove(text, functions[0].Token.Line); doc != "" {
			contents += "\n\n" + doc
		}
	}
	return contents
}

func functionSourceMatches(function sema.Function, sourcePath string) bool {
	if sourcePath == "" || function.Token.File == "" {
		return true
	}
	return filepath.Clean(function.Token.File) == filepath.Clean(sourcePath)
}

func implTargetAtOffset(program *ast.Program, analyzer *sema.Analyzer, text string, sourcePath string, offset int) (sema.Type, bool) {
	if program == nil {
		return sema.Type{}, false
	}
	for _, stmt := range program.Statements {
		impl, ok := stmt.(*ast.ImplStatement)
		if !ok || impl == nil || impl.Target == nil || !statementSourceMatches(impl.Token, sourcePath) {
			continue
		}
		start := textPositionOffset(text, impl.Token.Line, impl.Token.Column)
		if start < 0 {
			continue
		}
		brace := strings.Index(text[start:], "{")
		if brace < 0 {
			continue
		}
		bodyStart := start + brace
		bodyEnd := matchingBraceOffset(text, bodyStart)
		if bodyEnd < 0 || offset < bodyStart || offset > bodyEnd {
			continue
		}
		target, ok := analyzer.Types()[impl.Target.Name]
		return target, ok
	}
	return sema.Type{}, false
}

func statementSourceMatches(token lexer.Token, sourcePath string) bool {
	if sourcePath == "" || token.File == "" {
		return true
	}
	return filepath.Clean(token.File) == filepath.Clean(sourcePath)
}

func identifierAtOffset(text string, offset int) (string, int, int, bool) {
	if offset < 0 || offset > len(text) {
		return "", 0, 0, false
	}
	index := offset
	if index == len(text) || !isIdentifierByte(text[index]) {
		index--
	}
	if index < 0 || !isIdentifierByte(text[index]) {
		return "", 0, 0, false
	}
	start := index
	for start > 0 && isIdentifierByte(text[start-1]) {
		start--
	}
	end := index + 1
	for end < len(text) && isIdentifierByte(text[end]) {
		end++
	}
	return text[start:end], start, end, true
}

func isSelfMemberSelector(text string, nameStart int) bool {
	index := nameStart - 1
	for index >= 0 && (text[index] == ' ' || text[index] == '\t' || text[index] == '\r' || text[index] == '\n') {
		index--
	}
	if index < 0 || text[index] != '.' {
		return false
	}
	index--
	for index >= 0 && (text[index] == ' ' || text[index] == '\t' || text[index] == '\r' || text[index] == '\n') {
		index--
	}
	end := index + 1
	for index >= 0 && isIdentifierByte(text[index]) {
		index--
	}
	return text[index+1:end] == "self"
}

func offsetsRange(text string, start int, end int) lspRange {
	return lspRange{Start: offsetPosition(text, start), End: offsetPosition(text, end)}
}

func offsetPosition(text string, offset int) position {
	line := 0
	column := 0
	for index, value := range text {
		if index >= offset {
			return position{Line: line, Character: column}
		}
		if value == '\n' {
			line++
			column = 0
		} else {
			column++
		}
	}
	return position{Line: line, Character: column}
}

func functionDocCommentAbove(text string, functionLine int) string {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"), "\n")
	lineIndex := functionLine - 2
	for lineIndex >= 0 && strings.TrimSpace(lines[lineIndex]) == "" {
		lineIndex--
	}
	if lineIndex < 0 || !strings.HasSuffix(strings.TrimSpace(lines[lineIndex]), "*/") {
		return ""
	}
	end := lineIndex
	for lineIndex >= 0 && !strings.Contains(lines[lineIndex], "/**") {
		lineIndex--
	}
	if lineIndex < 0 {
		return ""
	}
	if strings.TrimSpace(lines[lineIndex]) == "/**" && end == lineIndex {
		return ""
	}
	docLines := []string{}
	for i := lineIndex; i <= end; i++ {
		line := strings.TrimSpace(lines[i])
		line = strings.TrimPrefix(line, "/**")
		line = strings.TrimSuffix(line, "*/")
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "*")
		line = strings.TrimSpace(line)
		if line != "" {
			docLines = append(docLines, line)
		}
	}
	return strings.Join(docLines, "\n")
}

func parseProgramForLSP(uri string, text string) *ast.Program {
	l := lexer.New(text)
	if uri != "" {
		l = lexer.NewWithFile(text, pathFromURI(uri))
	}
	p := parser.New(l)
	return p.Parse().Program
}

func tokenRange(token lexer.Token) lspRange {
	line := max(token.Line-1, 0)
	start := max(token.Column-1, 0)
	return lspRange{
		Start: position{Line: line, Character: start},
		End:   position{Line: line, Character: start + semanticTokenLength(token)},
	}
}

func rangeContaining(rng lspRange, contained lspRange) lspRange {
	if comparePosition(contained.Start, rng.Start) < 0 {
		rng.Start = contained.Start
	}
	if comparePosition(contained.End, rng.End) > 0 {
		rng.End = contained.End
	}
	return rng
}

func comparePosition(left position, right position) int {
	if left.Line != right.Line {
		return left.Line - right.Line
	}
	return left.Character - right.Character
}

func positionInRange(pos position, rng lspRange) bool {
	if pos.Line < rng.Start.Line || pos.Line > rng.End.Line {
		return false
	}
	if pos.Line == rng.Start.Line && pos.Character < rng.Start.Character {
		return false
	}
	if pos.Line == rng.End.Line && pos.Character > rng.End.Character {
		return false
	}
	return true
}

func typeReferenceName(ref *ast.TypeReference) string {
	if ref == nil {
		return ""
	}
	name := ref.Name
	if name == "" && ref.Unit != "" {
		name = ref.Unit
	}
	if name == "" && ref.ElementType != nil {
		name = typeReferenceName(ref.ElementType)
	}
	if ref.Ref {
		if ref.MutableRef {
			name = "ref mut " + name
		} else {
			name = "ref " + name
		}
	}
	if ref.Slice {
		name += "[]"
	}
	if ref.ArrayLength > 0 {
		name += fmt.Sprintf("[%d]", ref.ArrayLength)
	}
	if len(ref.TypeArgs) > 0 {
		args := make([]string, 0, len(ref.TypeArgs))
		for _, arg := range ref.TypeArgs {
			args = append(args, typeReferenceName(arg))
		}
		name += "[" + strings.Join(args, ", ") + "]"
	}
	return name
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

func memberCompletionItems(exprType sema.Type, functions map[string][]sema.Function, symbols map[string]sema.Symbol, prefix string, static bool) []completionItem {
	items := []completionItem{}
	seen := map[string]bool{}
	add := func(item completionItem) {
		if item.Label == "" || seen[item.Label] || !completionLabelMatches(item.Label, prefix) {
			return
		}
		seen[item.Label] = true
		items = append(items, item)
	}
	for _, member := range sema.CompilerKnownMembersForType(exprType, static) {
		kind := 2
		if member.Kind == sema.CompilerKnownProperty {
			kind = 10
		}
		detail := lspTypeName(member.Result)
		if member.Result.Kind == sema.InvalidType || member.Result.Kind == "" {
			detail = string(member.Kind)
		}
		if member.Unsafe {
			detail = "unsafe " + detail
		}
		add(completionItem{Label: member.Name, Kind: kind, Detail: detail})
	}

	if !static {
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
	}

	for _, property := range exprType.Properties {
		add(completionItem{Label: property.Name, Kind: 10, Detail: lspTypeName(property.Type)})
	}
	for _, event := range exprType.Events {
		add(completionItem{Label: event.Name, Kind: 24, Detail: lspTypeName(event.Type)})
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
		add(completionItem{Label: name, Kind: typeCompletionKind(typ), Detail: typeCompletionDetail(typ)})
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
	"defer", "detach", "else", "enum", "even", "extern", "fallthrough", "false", "finite", "fn", "for",
	"free", "get", "if", "impl", "implements", "import", "in", "interface",
	"let", "match", "module", "multipleOf", "mut", "notEmpty", "odd", "panic", "process",
	"new", "property", "ref", "return", "select", "self", "set", "spawn", "static", "struct",
	"require", "switch", "task", "thread", "true", "try", "type", "unique", "unit", "union",
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

func typeCompletionDetail(typ sema.Type) string {
	detail := string(typ.Kind)
	if value, _, ok := sema.DefaultValueDisplay(typ); ok {
		detail += " = " + value
	}
	return detail
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

func (s *server) stopDiagnosticTimer(uri string) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if timer := s.diagnosticTimers[uri]; timer != nil {
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

func (s *server) republishOpenDiagnostics() error {
	for _, snapshot := range s.documentSnapshots.Snapshots() {
		if err := s.publishDiagnostics(snapshot.URI, snapshot.Text); err != nil {
			return err
		}
	}
	return nil
}

func (s *server) formatDocument(uri string) ([]textEdit, error) {
	snapshot, ok := s.documentSnapshots.Snapshot(uri)
	text := snapshot.Text
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

func ownershipCodeActions(uri string, text string, reported []diagnostic) []codeAction {
	actions := make([]codeAction, 0, len(reported))
	for _, reportedDiagnostic := range reported {
		if reportedDiagnostic.Code != diagnostics.ImplicitMoveDisallowed {
			continue
		}
		edit, title, ok := ownershipMoveSyntaxEdit(text, reportedDiagnostic)
		if !ok {
			continue
		}
		actions = append(actions, codeAction{
			Title:       title,
			Kind:        "quickfix",
			Diagnostics: []diagnostic{reportedDiagnostic},
			Edit: workspaceEdit{Changes: map[string][]textEdit{
				uri: {edit},
			}},
		})
	}
	return actions
}

// ownershipMoveSyntaxEdit turns only the operator belonging to a verified
// implicit-move diagnostic into its explicit-move equivalent. The diagnostic
// is attached to the named source, so the immediately preceding token must be
// the ordinary declaration or assignment operator.
func ownershipMoveSyntaxEdit(text string, reported diagnostic) (textEdit, string, bool) {
	targetLine := reported.Range.Start.Line + 1
	targetColumn := reported.Range.Start.Character + 1
	l := lexer.New(text)
	previous := lexer.Token{}
	for {
		token := l.NextToken()
		if token.Type == lexer.EOF {
			return textEdit{}, "", false
		}
		if token.Line == targetLine && token.Column == targetColumn {
			var replacement, title string
			switch previous.Type {
			case lexer.DECLARE:
				replacement = ":<-"
				title = "Change := to :<- to move the value (source becomes unavailable)"
			case lexer.ASSIGN:
				replacement = "<-"
				title = "Change = to <- to move the value (source becomes unavailable)"
			default:
				return textEdit{}, "", false
			}
			return textEdit{
				Range: lspRange{
					Start: position{Line: previous.Line - 1, Character: previous.Column - 1},
					End:   position{Line: previous.Line - 1, Character: previous.Column - 1 + len([]rune(previous.Lexeme))},
				},
				NewText: replacement,
			}, title, true
		}
		previous = token
	}
}

func formatSource(text string) string {
	return formatter.Format(formatter.Source{Text: text}, formatter.Options{}).Text
}

// formatSourceLegacy is retained temporarily while the existing formatter
// helpers are removed in a follow-up mechanical cleanup. LSP uses the shared
// formatter package above.
func formatSourceLegacy(text string) string {
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
		trimmed = formatSingleLineLetDeclaration(formatSingleLineFunctionSignature(normalizeFunctionKeyword(trimmed)))

		lineIndent := indent - leadingClosingDelimiterCount(trimmed)
		if lineIndent < 0 {
			lineIndent = 0
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
		indentDelta := delimiterIndentDelta(trimmed)
		indent += indentDelta
		if indent < 0 {
			indent = 0
		}
		if isBranchBlockStart(trimmed) && indentDelta > 0 {
			branchBlocks = append(branchBlocks, formatterBranchContext{contentDepth: indent, bodyExtra: isSwitchBlockStart(trimmed)})
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

func formatSingleLineFunctionSignature(line string) string {
	if !strings.HasPrefix(line, "fn ") {
		return line
	}

	open := strings.Index(line, "(")
	if open < 0 {
		return line
	}
	close := matchingSignatureParenthesis(line, open)
	if close < 0 {
		return line
	}

	parameters := splitTopLevelCommaList(line[open+1 : close])
	if parameters == nil {
		return line
	}
	return line[:open+1] + strings.Join(parameters, ", ") + line[close:]
}

// normalizeFunctionKeyword corrects only a complete, body-bearing function
// declaration. Other uses of the identifier "func" are left untouched.
func normalizeFunctionKeyword(line string) string {
	if !strings.HasPrefix(line, "func ") {
		return line
	}

	open := strings.Index(line, "(")
	if open < 0 || !plausibleFunctionName(strings.TrimSpace(line[len("func "):open])) {
		return line
	}
	close := matchingSignatureParenthesis(line, open)
	if close < 0 || !strings.Contains(line[close+1:], "{") {
		return line
	}
	return "fn " + strings.TrimPrefix(line, "func ")
}

func plausibleFunctionName(name string) bool {
	if name == "" {
		return false
	}
	for index, r := range name {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func formatSingleLineLetDeclaration(line string) string {
	if !strings.HasPrefix(line, "let ") || strings.Contains(line, "//") || strings.Contains(line, "/*") {
		return line
	}

	declarations := splitTopLevelCommaList(strings.TrimPrefix(line, "let "))
	if len(declarations) < 2 {
		return line
	}
	for _, declaration := range declarations {
		if !strings.Contains(declaration, ":=") {
			return line
		}
	}
	return "let " + strings.Join(declarations, ", ")
}

func matchingSignatureParenthesis(line string, open int) int {
	depth := 0
	angleDepth := 0
	inString := false
	inChar := false
	escaped := false
	for index, r := range line[open:] {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
			continue
		}
		if inChar {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
			} else if r == '\'' {
				inChar = false
			}
			continue
		}

		switch r {
		case '"':
			inString = true
		case '\'':
			inChar = true
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && angleDepth == 0 {
				return open + index
			}
		}
	}
	return -1
}

func splitTopLevelCommaList(value string) []string {
	parts := []string{}
	start := 0
	parenDepth := 0
	bracketDepth := 0
	angleDepth := 0
	inString := false
	inChar := false
	escaped := false

	for index, r := range value {
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
			} else if r == '"' {
				inString = false
			}
			continue
		}
		if inChar {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
			} else if r == '\'' {
				inChar = false
			}
			continue
		}

		switch r {
		case '"':
			inString = true
		case '\'':
			inChar = true
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case ',':
			if parenDepth == 0 && bracketDepth == 0 && angleDepth == 0 {
				part := strings.TrimSpace(value[start:index])
				if part != "" {
					parts = append(parts, part)
				}
				start = index + 1
			}
		}
	}

	if inString || inChar || parenDepth != 0 || bracketDepth != 0 || angleDepth != 0 {
		return nil
	}
	if part := strings.TrimSpace(value[start:]); part != "" {
		parts = append(parts, part)
	}
	return parts
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

func isTypedDeclarationGroupStart(line string, lines []string, index int) bool {
	if line == "import (" || !strings.HasSuffix(line, "(") {
		return false
	}
	prefix := strings.TrimSpace(strings.TrimSuffix(line, "("))
	if !isPlausibleTypeReferencePrefix(prefix) {
		return false
	}
	for i := index + 1; i < len(lines); i++ {
		next := strings.TrimSpace(strings.TrimRight(strings.ReplaceAll(lines[i], "\t", "    "), " \t"))
		if next == "" || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/*") || strings.HasPrefix(next, "*") {
			continue
		}
		if next == ")" {
			return false
		}
		return strings.Contains(next, ":=")
	}
	return false
}

func isPlausibleTypeReferencePrefix(prefix string) bool {
	fields := strings.Fields(prefix)
	if len(fields) == 0 {
		return false
	}
	if isFormatterStatementKeyword(fields[0]) {
		return false
	}
	for _, r := range strings.ReplaceAll(prefix, " ", "") {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			continue
		}
		switch r {
		case '_', '[', ']', '<', '>', ',', '.', '?', '&', '*':
			continue
		default:
			return false
		}
	}
	return true
}

func isFormatterStatementKeyword(word string) bool {
	switch word {
	case "if", "else", "for", "while", "switch", "match", "select", "case", "default",
		"fn", "return", "let", "try", "spawn", "await", "defer", "discard",
		"impl", "type", "struct", "enum", "interface", "property", "extern", "unsafe",
		"asm", "import", "module":
		return true
	default:
		return false
	}
}

func leadingClosingDelimiterCount(line string) int {
	for _, r := range line {
		switch r {
		case ' ', '\t':
			continue
		case '}', ')', ']':
			return 1
		default:
			return 0
		}
	}
	return 0
}

func delimiterIndentDelta(line string) int {
	delta := 0
	inString := false
	inChar := false
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
		if inChar {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '\'' {
				inChar = false
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
		if r == '\'' {
			inChar = true
			continue
		}
		switch r {
		case '{', '(', '[':
			delta++
		case '}', ')', ']':
			delta--
		}
	}
	switch {
	case delta < 0:
		return -1
	case delta > 0:
		return 1
	default:
		return 0
	}
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
	parseResult := p.Parse()
	program := parseResult.Program

	diagnostics := []diagnostic{}
	for _, parserError := range parseResult.Diagnostics {
		diagnostics = append(diagnostics, structuredParserDiagnostic(parserError))
	}
	if parseResult.HasErrors {
		return diagnostics
	}
	resolveCoreSources(program, path)
	resolveSourceImports(program, map[string]bool{}, path)

	analyzer := newLSPAnalyzer(uri)
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
	if ok {
		primary := filepath.Clean(filepath.Join(root, relative))
		module := strings.TrimPrefix(path, "std/")
		if module != "fmt" && module != "io" && module != "unicode" {
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
	return lspProjectIncludePaths(path, sourceFile)
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
	case "unicode":
		return filepath.Join("sec", "stdlib", "unicode", "unicode.sec"), true
	}
	if strings.HasPrefix(path, "platform/") {
		trimmed := strings.Trim(path, "/")
		trimmed = strings.TrimSuffix(trimmed, ".sec")
		return filepath.Join("sec", trimmed+".sec"), true
	}
	return "", false
}

func lspProjectIncludePaths(path string, sourceFile string) []string {
	trimmed := strings.Trim(strings.TrimSuffix(path, ".sec"), "/")
	if trimmed == "" || strings.HasPrefix(trimmed, "std/") || strings.HasPrefix(trimmed, "platform/") {
		return nil
	}
	for _, root := range lspProjectSourceRoots(sourceFile) {
		if paths, ok := lspProjectIncludePathsUnderRoot(root, trimmed); ok {
			return paths
		}
	}
	return nil
}

func lspProjectIncludePathsUnderRoot(root string, importPath string) ([]string, bool) {
	// Project imports are resolved separately from stdlib imports. This keeps
	// ordinary source modules from accidentally becoming permanent library API.
	candidates := []string{
		filepath.Join(root, filepath.FromSlash(importPath)+".sec"),
		filepath.Join(root, filepath.FromSlash(importPath), filepath.Base(importPath)+".sec"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return []string{filepath.Clean(candidate)}, true
		}
	}

	dir := filepath.Join(root, filepath.FromSlash(importPath))
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		matches, globErr := filepath.Glob(filepath.Join(dir, "*.sec"))
		if globErr != nil || len(matches) == 0 {
			return nil, false
		}
		sort.Strings(matches)
		return matches, true
	}
	return nil, false
}

func lspProjectSourceRoots(sourceFile string) []string {
	seen := map[string]bool{}
	roots := []string{}
	add := func(root string) {
		if root == "" {
			return
		}
		root = filepath.Clean(root)
		if seen[root] {
			return
		}
		seen[root] = true
		roots = append(roots, root)
	}
	if sourceFile != "" {
		add(findProjectRoot(sourceFile))
		add(filepath.Dir(sourceFile))
	}
	if cwd, err := os.Getwd(); err == nil {
		add(findProjectRoot(cwd))
	}
	return roots
}

func findProjectRoot(path string) string {
	current := filepath.Clean(path)
	if info, err := os.Stat(current); err == nil && !info.IsDir() {
		current = filepath.Dir(current)
	}
	for {
		if info, err := os.Stat(filepath.Join(current, ".sec", "sec.toml")); err == nil && !info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return filepath.Clean(path)
	}
	return filepath.Dir(filepath.Clean(path))
}

func newLSPAnalyzer(uri string) *sema.Analyzer {
	return sema.NewAnalyzerWithDepth(lspAnalysisDepth(pathFromURI(uri)))
}

func lspAnalysisDepth(sourcePath string) sema.AnalysisDepth {
	depth := sema.AnalysisInteractive
	if sourcePath == "" {
		return depth
	}
	manifest := filepath.Join(findProjectRoot(sourcePath), ".sec", "sec.toml")
	configured, err := readLSPAnalysisDepth(manifest)
	if err == nil {
		return configured
	}
	return depth
}

func readLSPAnalysisDepth(manifest string) (sema.AnalysisDepth, error) {
	file, err := os.Open(manifest)
	if err != nil {
		return "", err
	}
	defer file.Close()

	section := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		if section != "analysis" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "lsp_depth" {
			continue
		}
		value = strings.TrimSpace(value)
		if comment := strings.Index(value, "#"); comment >= 0 {
			value = strings.TrimSpace(value[:comment])
		}
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid analysis.lsp_depth in %s: %w", manifest, err)
		}
		return sema.ParseAnalysisDepth(unquoted)
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return sema.AnalysisInteractive, nil
}

// findSelectorLHS walks the AST and returns the expression that matches the selector
// written immediately before the cursor, such as the "foo" in "foo.".
func findSelectorLHS(node any, text string, dotOffset int) ast.Expression {
	if isNilASTValue(node) {
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
	case *ast.ImplStatement:
		if n == nil {
			return nil
		}
		for _, member := range n.Members {
			if found := findSelectorLHS(member, text, dotOffset); found != nil {
				return found
			}
		}
	case *ast.PropertyDeclaration:
		if n == nil {
			return nil
		}
		if found := findSelectorLHS(n.Getter, text, dotOffset); found != nil {
			return found
		}
		if n.Setter != nil {
			return findSelectorLHS(n.Setter.Body, text, dotOffset)
		}
	case *ast.InitDeclaration:
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

func isNilASTValue(node any) bool {
	if node == nil {
		return true
	}
	value := reflect.ValueOf(node)
	return value.Kind() == reflect.Ptr && value.IsNil()
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
	parseResult := p.Parse()
	program := parseResult.Program
	if parseResult.HasErrors {
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
		case *ast.ImplStatement:
			if stmt == nil {
				continue
			}
			qualifyLocalCallsInImplMembers(stmt.Members, module, localFunctions)
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
				switch member := member.(type) {
				case *ast.FunctionDeclaration:
					if member != nil {
						rewriteQualifierInBlock(member.Body, from, to)
					}
				case *ast.InitDeclaration:
					if member != nil {
						rewriteQualifierInBlock(member.Body, from, to)
					}
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
	case *ast.MatchStatement:
		if stmt.Match != nil {
			rewriteQualifierInExpression(stmt.Match, from, to)
		}
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
	case *ast.NewExpression:
		if expr.Type != nil {
			expr.Type.Name = rewriteQualifiedName(expr.Type.Name, from, to)
		}
		for _, argument := range expr.Arguments {
			rewriteQualifierInExpression(argument, from, to)
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
	case *ast.MatchExpression:
		rewriteQualifierInExpression(expr.Subject, from, to)
		for _, arm := range expr.Arms {
			rewriteQualifierInExpression(arm.Pattern, from, to)
			rewriteQualifierInExpression(arm.Guard, from, to)
			rewriteQualifierInExpression(arm.Body, from, to)
			if arm.ReturnBody != nil {
				rewriteQualifierInStatement(arm.ReturnBody, from, to)
			}
			rewriteQualifierInBlock(arm.BlockBody, from, to)
		}
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
	case *ast.MatchStatement:
		if stmt == nil || stmt.Match == nil {
			return
		}
		qualifyLocalTypesInExpression(stmt.Match, module, localTypes)
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
	case *ast.InitDeclaration:
		if member == nil {
			return
		}
		for _, parameter := range member.Parameters {
			qualifyLocalTypeReference(parameter.Type, module, localTypes)
		}
		qualifyLocalTypeReference(member.ErrorType, module, localTypes)
		qualifyLocalTypeReferencesInBlock(member.Body, module, localTypes)
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
	case *ast.NewExpression:
		qualifyLocalTypeReference(expr.Type, module, localTypes)
		for _, argument := range expr.Arguments {
			qualifyLocalTypesInExpression(argument, module, localTypes)
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
			if arm.ReturnBody != nil {
				qualifyLocalTypeReferencesInStatement(arm.ReturnBody, module, localTypes)
			}
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

func qualifyLocalCallsInImplMembers(members []ast.ImplMember, module string, localFunctions map[string]bool) {
	for _, member := range members {
		switch member := member.(type) {
		case *ast.FunctionDeclaration:
			qualifyLocalCalls(member.Body, module, localFunctions)
		case *ast.InitDeclaration:
			qualifyLocalCalls(member.Body, module, localFunctions)
		case *ast.PropertyDeclaration:
			if member == nil {
				continue
			}
			qualifyLocalCalls(member.Getter, module, localFunctions)
			if member.Setter != nil {
				qualifyLocalCalls(member.Setter.Body, module, localFunctions)
			}
		}
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
	case *ast.MatchStatement:
		if stmt.Match != nil {
			qualifyLocalCallsInExpression(stmt.Match, module, localFunctions)
		}
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
	case *ast.NewExpression:
		for _, argument := range expr.Arguments {
			qualifyLocalCallsInExpression(argument, module, localFunctions)
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
			if arm.ReturnBody != nil {
				qualifyLocalCallsInStatement(arm.ReturnBody, module, localFunctions)
			}
			qualifyLocalCalls(arm.BlockBody, module, localFunctions)
		}
	}
}

func semaDiagnostic(err sema.Error, severity int) diagnostic {
	line := max(err.Line-1, 0)
	column := max(err.Column-1, 0)
	message := err.Error()
	if err.Help != "" {
		message += "\n\nhelp: " + err.Help
	}
	return diagnostic{
		Range: lspRange{
			Start: position{Line: line, Character: column},
			End:   position{Line: line, Character: column + 1},
		},
		Severity: lspSeverity(err.Severity, severity),
		Code:     err.ID,
		Source:   "sec",
		Message:  message,
	}
}

func lspSeverity(severity diagnostics.Severity, fallback int) int {
	switch severity {
	case diagnostics.SeverityError:
		return 1
	case diagnostics.SeverityWarning:
		return 2
	case diagnostics.SeverityInformation:
		return 3
	default:
		return fallback
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
		Code:     diagnostics.ParserSyntaxError,
		Source:   "sec",
		Message:  message,
	}
}

func structuredParserDiagnostic(value parser.Diagnostic) diagnostic {
	result := parserDiagnostic(value.Message)
	result.Code = value.ID
	if value.ID == diagnostics.ParserSyntaxError {
		return result
	}
	primary := value.Primary
	if value.Unexpected != nil {
		primary = *value.Unexpected
	}
	if primary.Line > 0 && primary.Column > 0 {
		width := len([]rune(primary.Lexeme))
		if width == 0 {
			width = 1
		}
		result.Range = lspRange{
			Start: position{Line: primary.Line - 1, Character: primary.Column - 1},
			End:   position{Line: primary.Line - 1, Character: primary.Column - 1 + width},
		}
	}
	return result
}

func (s *server) respond(id json.RawMessage, result any) error {
	return s.writeResponseMessage(rpcResponseMessage{
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
	return s.writeJSONMessage(message)
}

func (s *server) writeResponseMessage(message rpcResponseMessage) error {
	return s.writeJSONMessage(message)
}

func (s *server) writeJSONMessage(message any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return protocol.WriteMessage(s.out, message)
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

func uriFromPath(path string) string {
	if path == "" {
		return ""
	}
	if parsed, err := url.Parse(path); err == nil && parsed.Scheme != "" {
		return path
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
}
