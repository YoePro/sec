package main

import (
	"fmt"
	"os"
	"runtime"
)

// Threaded request handling. Messages are still read and dequeued in order on
// the server loop: document synchronization, formatting and every other
// message that is not a read-only analysis request is handled there, in order.
// Read-only analysis requests (hover, semantic tokens, inlay hints, code
// lenses, completion, ...) run on a bounded pool of worker goroutines, so a
// slow analysis no longer blocks the requests behind it and independent
// requests use several cores. Responses may arrive out of order, which
// JSON-RPC permits.
//
// Shared state: documents, the workspace symbol index, the cross-target store
// and output are already internally synchronized; the analysis cache locks
// each entry for its user. The settings and workspace fields of the server are
// guarded by stateMu: workers hold it shared and the messages in
// stateMutatingMethod hold it exclusively.
//
// Rules:
//   - rules/tooling/lsp.md — "Responsiveness model", "Snapshots"

// analysisWorkerCount bounds concurrent analyses; each analysis is memory
// heavy, so the pool stays small even on large machines.
func analysisWorkerCount() int {
	workers := runtime.NumCPU() / 2
	if workers < 2 {
		workers = 2
	}
	if workers > 4 {
		workers = 4
	}
	return workers
}

// prepareConcurrentState creates the lazily initialized shared stores before
// workers can read them.
func (s *server) prepareConcurrentState() {
	if s.workspaceSymbols == nil {
		s.workspaceSymbols = newWorkspaceSymbolIndex()
	}
	if s.crossTarget == nil {
		s.crossTarget = newCrossTargetStore()
	}
}

// handleConcurrently runs one read-only request on a worker once a slot is
// free; waiting for a slot applies back-pressure to the server loop.
func (s *server) handleConcurrently(message rpcMessage, slots chan struct{}) {
	slots <- struct{}{}
	s.workers.Add(1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				fmt.Fprintf(os.Stderr, "lsp request %s panicked: %v\n", message.Method, recovered)
				_ = s.respondError(message.ID, -32603, fmt.Sprintf("internal error in %s", message.Method))
			}
			<-slots
			s.workers.Done()
		}()
		s.stateMu.RLock()
		defer s.stateMu.RUnlock()
		if err := s.handle(message); err != nil {
			fmt.Fprintf(os.Stderr, "lsp request %s failed: %v\n", message.Method, err)
		}
	}()
}

// handleOrdered handles a message on the server loop, exclusively when it
// changes the settings or workspace fields that workers read.
func (s *server) handleOrdered(message rpcMessage) error {
	if stateMutatingMethod(message.Method) {
		s.stateMu.Lock()
		defer s.stateMu.Unlock()
	}
	return s.handle(message)
}

func stateMutatingMethod(method string) bool {
	switch method {
	case "initialize", "workspace/didChangeWorkspaceFolders", "workspace/didChangeConfiguration", "workspace/didChangeWatchedFiles", "shutdown", "exit":
		return true
	}
	return false
}
