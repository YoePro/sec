package main

import (
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
}

func newRequestQueue() *requestQueue {
	queue := &requestQueue{}
	queue.ready = sync.NewCond(&queue.mu)
	return queue
}

func (q *requestQueue) push(message rpcMessage) {
	q.mu.Lock()
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
func (q *requestQueue) next() (rpcMessage, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.ready.Wait()
	}
	if len(q.items) == 0 {
		return rpcMessage{}, false
	}
	index := nextRequestIndex(q.items)
	message := q.items[index]
	q.items = append(q.items[:index], q.items[index+1:]...)
	return message, true
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
