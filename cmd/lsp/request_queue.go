package main

import (
	"encoding/json"
	"strings"
	"sync"
)

// requestQueue decouples reading protocol messages from handling them, so a
// formatting request that arrives while analysis requests are still waiting is
// not delayed behind them. Messages are otherwise handled in arrival order.
//
// Rules:
//   - rules/tooling/lsp.md — "Responsiveness model" (formatting must not wait for queued analysis)
type requestQueue struct {
	mu      sync.Mutex
	ready   *sync.Cond
	items   []rpcMessage
	closed  bool
	readErr error
	// cancelled holds the IDs of queued requests the client cancelled with
	// $/cancelRequest before they were handled.
	cancelled map[string]bool
}

// queueDisposition tells the handler loop how to answer a dequeued message.
type queueDisposition int

const (
	// dispositionHandle handles the message normally.
	dispositionHandle queueDisposition = iota
	// dispositionCancelled answers RequestCancelled (-32800): the client
	// cancelled the request while it was queued.
	dispositionCancelled
	// dispositionContentModified answers ContentModified (-32801): a later
	// edit or close of the same document is already queued, so the result
	// would describe a stale document.
	dispositionContentModified
)

func newRequestQueue() *requestQueue {
	queue := &requestQueue{}
	queue.ready = sync.NewCond(&queue.mu)
	return queue
}

func (q *requestQueue) push(message rpcMessage) {
	q.mu.Lock()
	if message.Method == "$/cancelRequest" {
		// A cancellation only matters while the request is still queued; one
		// for a request already handled is dropped, so nothing accumulates.
		var params struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal(message.Params, &params) == nil {
			id := requestKey(params.ID)
			for _, item := range q.items {
				if len(item.ID) > 0 && requestKey(item.ID) == id {
					if q.cancelled == nil {
						q.cancelled = map[string]bool{}
					}
					q.cancelled[id] = true
					break
				}
			}
		}
		q.mu.Unlock()
		return
	}
	q.items = append(q.items, message)
	q.mu.Unlock()
	q.ready.Signal()
}

// close ends the queue after the remaining items; err is nil at end of input.
func (q *requestQueue) close(err error) {
	q.mu.Lock()
	q.closed = true
	q.readErr = err
	q.mu.Unlock()
	q.ready.Broadcast()
}

// next blocks until a message is available and returns the one to handle
// next, or false once the queue is closed and drained.
func (q *requestQueue) next() (rpcMessage, queueDisposition, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.ready.Wait()
	}
	if len(q.items) == 0 {
		return rpcMessage{}, dispositionHandle, false
	}
	index := nextRequestIndex(q.items)
	message := q.items[index]
	q.items = append(q.items[:index], q.items[index+1:]...)
	if len(message.ID) > 0 {
		id := requestKey(message.ID)
		if q.cancelled[id] {
			delete(q.cancelled, id)
			return message, dispositionCancelled, true
		}
		if isDeferrableAnalysisRequest(message.Method) && supersededByQueuedEdit(message, q.items) {
			return message, dispositionContentModified, true
		}
	}
	return message, dispositionHandle, true
}

// requestKey normalizes a JSON-RPC request ID for comparison.
func requestKey(id json.RawMessage) string {
	return strings.TrimSpace(string(id))
}

// supersededByQueuedEdit reports an analysis request for a document whose
// later change or close is already queued: answering it would cost a full
// analysis of a version the editor has already replaced. Requests without a
// document (workspace/symbol, call hierarchy items) are never superseded.
//
// Rules:
//   - rules/tooling/lsp.md — "Responsiveness model", "Document synchronization"
func supersededByQueuedEdit(message rpcMessage, later []rpcMessage) bool {
	uri := requestDocumentURI(message.Params)
	if uri == "" && message.Method == "codeLens/resolve" {
		var lens codeLens
		if json.Unmarshal(message.Params, &lens) == nil && lens.Data != nil {
			uri = lens.Data.URI
		}
	}
	if uri == "" && message.Method == "inlayHint/resolve" {
		var hint inlayHint
		if json.Unmarshal(message.Params, &hint) == nil && hint.Data != nil {
			uri = hint.Data.URI
		}
	}
	if uri == "" {
		return false
	}
	for _, item := range later {
		if item.Method != "textDocument/didChange" && item.Method != "textDocument/didClose" {
			continue
		}
		if requestDocumentURI(item.Params) == uri {
			return true
		}
	}
	return false
}

func requestDocumentURI(params json.RawMessage) string {
	var document struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`
	}
	if json.Unmarshal(params, &document) != nil {
		return ""
	}
	return document.TextDocument.URI
}

func (q *requestQueue) err() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.readErr
}

// nextRequestIndex selects the queue head, except that a formatting request
// may move ahead of read-only analysis requests queued before it. It never
// moves ahead of a document-synchronization notification or any other
// message, so formatting always sees every edit that preceded it.
func nextRequestIndex(items []rpcMessage) int {
	for index, item := range items {
		if isPriorityFormattingRequest(item.Method) {
			return index
		}
		if !isDeferrableAnalysisRequest(item.Method) {
			return 0
		}
	}
	return 0
}

func isPriorityFormattingRequest(method string) bool {
	return method == "textDocument/formatting" || method == "textDocument/willSaveWaitUntil"
}

// isDeferrableAnalysisRequest reports a read-only request whose answer does
// not change document state, so a later formatting request may overtake it.
func isDeferrableAnalysisRequest(method string) bool {
	switch method {
	case "textDocument/hover",
		"textDocument/completion",
		"textDocument/semanticTokens/full",
		"textDocument/definition",
		"textDocument/typeDefinition",
		"textDocument/references",
		"textDocument/documentHighlight",
		"textDocument/documentSymbol",
		"workspace/symbol",
		"workspaceSymbol/resolve",
		"textDocument/inlayHint",
		"inlayHint/resolve",
		"textDocument/codeLens",
		"codeLens/resolve",
		"sec/compilerKnownDefinition",
		"textDocument/signatureHelp",
		"textDocument/codeAction",
		"textDocument/prepareCallHierarchy",
		"callHierarchy/incomingCalls",
		"callHierarchy/outgoingCalls":
		return true
	}
	return false
}
