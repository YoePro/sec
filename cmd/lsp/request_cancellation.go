package main

import (
	"context"
	"encoding/json"
	"errors"
)

var analysisContentModified = errors.New("analysis inputs changed")

type runningAnalysisRequest struct {
	ctx       context.Context
	cancel    context.CancelCauseFunc
	responded bool
}

// beginAnalysisRequest transfers a request atomically from the protocol queue
// to the running registry. Cancellation arriving after dequeue cannot be lost.
// Rules: rules/tooling/lsp.md — Document synchronization, §6.7(2–3), Snapshots.
func (s *server) beginAnalysisRequest(id json.RawMessage) context.Context {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	key := requestKey(id)
	if request, ok := s.analysisRequests[key]; ok {
		return request.ctx
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	if s.analysisRequests == nil {
		s.analysisRequests = map[string]runningAnalysisRequest{}
	}
	s.analysisRequests[key] = runningAnalysisRequest{ctx: ctx, cancel: cancel}
	return ctx
}

func (s *server) finishAnalysisRequest(id json.RawMessage) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	key := requestKey(id)
	if request, ok := s.analysisRequests[key]; ok {
		request.cancel(context.Canceled)
		delete(s.analysisRequests, key)
	}
}

func (s *server) cancelAnalysisRequest(id json.RawMessage) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	if request, ok := s.analysisRequests[requestKey(id)]; ok && !request.responded {
		request.cancel(context.Canceled)
	}
}

// cancelAnalysisRequests withdraws running snapshots before ordered mutation
// waits for reader locks. Any open document may be an imported dependency.
// Rules: rules/tooling/lsp.md — Snapshots, Responsiveness model, §6.7(2–3).
func (s *server) cancelAnalysisRequests(cause error) {
	s.requestMu.Lock()
	defer s.requestMu.Unlock()
	for _, request := range s.analysisRequests {
		if !request.responded {
			request.cancel(cause)
		}
	}
}

type cancelledLSPAnalysis struct{ err error }

func (s *server) respondAnalysisCancellation(ctx context.Context, id json.RawMessage) error {
	if errors.Is(context.Cause(ctx), analysisContentModified) {
		return s.respondError(id, -32801, "content modified by a newer analysis snapshot")
	}
	return s.respondError(id, -32800, "request cancelled")
}

// respondAnalysis prevents even recovery-oriented handlers from publishing an
// empty or partial success after their compiler request was abandoned.
// Rules: rules/tooling/lsp.md — Snapshots, §6.7(2–3).
func (s *server) respondAnalysis(ctx context.Context, id json.RawMessage, result any) error {
	// Commit publication against protocol cancellation under the same registry
	// lock. Once committed, a later cancellation cannot retroactively win.
	s.requestMu.Lock()
	cancelled := ctx.Err() != nil
	if !cancelled {
		key := requestKey(id)
		if request, ok := s.analysisRequests[key]; ok {
			request.responded = true
			s.analysisRequests[key] = request
		}
	}
	s.requestMu.Unlock()
	if cancelled {
		return s.respondAnalysisCancellation(ctx, id)
	}
	return s.respond(id, result)
}

// analysisInputCancellation classifies ordered snapshot changes independently
// of queue arrival, so requests started after a notification was queued are
// still withdrawn before that notification mutates shared input state.
// Rules: rules/tooling/lsp.md — Snapshots, Document synchronization, §6.7(2–3).
func analysisInputCancellation(method string) (error, bool) {
	switch method {
	case "textDocument/didOpen", "textDocument/didChange", "textDocument/didClose", "textDocument/didSave", "workspace/didChangeConfiguration", "workspace/didChangeWatchedFiles", "workspace/didChangeWorkspaceFolders", "workspace/executeCommand":
		return analysisContentModified, true
	case "shutdown", "exit":
		return context.Canceled, true
	}
	return nil, false
}
