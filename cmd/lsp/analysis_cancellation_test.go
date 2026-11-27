package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	lspserver "sec/internal/lsp/server"
)

// checkpointCancellation deterministically cancels compiler work after a fixed
// number of polls; the actual context supplies thread-safe Done/Err behavior.
type checkpointCancellation struct {
	context.Context
	cancel    context.CancelFunc
	remaining atomic.Int64
}

func (c *checkpointCancellation) Err() error {
	if c.remaining.Add(-1) == 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func cancellationSource(t *testing.T) (string, string) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/lsp/cancellation/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "main.sec")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	return uriFromPath(path), string(data)
}

// Cancelled cache producers publish no analyzer or negative cache record, and a
// new request reanalyzes the same source normally. Shared leases are preserved.
// Rules: rules/tooling/lsp.md — Snapshots, Responsiveness model, §6.7(2–3).
func TestAnalysisCancellationCacheProducer(t *testing.T) {
	uri, text := cancellationSource(t)
	enableAnalysisCache()
	defer disableAnalysisCache()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := &checkpointCancellation{Context: ctx, cancel: cancel}
	request.remaining.Store(40)
	result, release, err := cachedLSPAnalysisResultContext(request, uri, text, parseProgramForLSP(uri, text), sourceOverlay{})
	release()
	if !errors.Is(err, context.Canceled) || result.analyzer != nil || result.program != nil {
		t.Fatal(result, err)
	}
	analysisCache.Lock()
	_, exists := analysisCache.entries[uri]
	analysisCache.Unlock()
	if exists {
		t.Fatal("cancelled producer poisoned cache")
	}
	result, release, err = cachedLSPAnalysisResultContext(context.Background(), uri, text, parseProgramForLSP(uri, text), sourceOverlay{})
	defer release()
	if err != nil || result.analyzer == nil || len(result.semanticErrors) != 0 {
		t.Fatal(result.semanticErrors, err)
	}
}

// A cancelled batch must discard parser errors and partial dependencies too,
// for both the single-document cached and multi-document direct Sema paths.
// Rules: rules/tooling/lsp.md — Snapshots, Responsiveness model, §6.7(2–3).
func TestAnalysisCancellationDiagnosticBatch(t *testing.T) {
	for _, count := range []int{1, 3} {
		snapshots, overlay := diagnosticBatchFixture(t, count)
		ctx, cancel := context.WithCancel(context.Background())
		request := &checkpointCancellation{Context: ctx, cancel: cancel}
		request.remaining.Store(40)
		dependencies := map[string]map[string]bool{"prior": {"preserved": true}}
		result, err := analyzeDiagnosticBatchContext(request, snapshots, overlay, pitfallDiagnosticSettings{}, dependencies)
		cancel()
		if !errors.Is(err, context.Canceled) || result != nil {
			t.Fatal(count, result, err)
		}
		if len(dependencies) != 1 || !dependencies["prior"]["preserved"] {
			t.Fatal("cancelled batch published partial dependency facts", dependencies)
		}
		if fresh := analyzeDiagnosticBatch(snapshots, overlay); len(fresh) != count {
			t.Fatal("retry lost parser diagnostics", fresh)
		}
	}
}

// Edits, close and shutdown stop a real running module job even while its
// cache lease is held by a reader. No stale diagnostics escape publication.
// Rules: rules/tooling/lsp.md — Document synchronization, Snapshots, §6.7(2–3).
func TestAnalysisCancellationRunningModuleJob(t *testing.T) {
	for _, action := range []string{"edit", "close", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			uri, text := cancellationSource(t)
			enableAnalysisCache()
			defer disableAnalysisCache()
			_, release := cachedLSPAnalysisResult(uri, text, parseProgramForLSP(uri, text), sourceOverlay{Sources: map[string]string{normalizedSourcePath(pathFromURI(uri)): text}})
			defer release()
			var out bytes.Buffer
			s := &server{out: &out, documentSnapshots: lspserver.NewDocuments(), diagnosticDelay: time.Hour}
			defer s.stopDiagnosticTimers()
			s.documentSnapshots.Open(uri, 1, text)
			s.scheduleModuleDiagnostics(uri)
			dir := normalizedSourcePath(filepath.Dir(pathFromURI(uri)))
			s.timerMu.Lock()
			generation := s.diagnosticGeneration[dir]
			s.timerMu.Unlock()
			done := make(chan error, 1)
			go func() { done <- s.publishDiagnosticBatch(dir, generation) }()
			deadline := time.Now().Add(5 * time.Second)
			for {
				s.timerMu.Lock()
				running := s.diagnosticCancels[dir] != nil
				s.timerMu.Unlock()
				if running {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("module job did not start")
				}
				time.Sleep(time.Millisecond)
			}
			switch action {
			case "edit":
				s.documentSnapshots.Change(uri, 2, text+"\n")
				s.scheduleModuleDiagnostics(uri)
			case "close":
				s.documentSnapshots.Close(uri)
				s.stopDiagnosticTimer(uri)
			case "shutdown":
				s.stopDiagnosticTimers()
			}
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("cancelled job kept waiting for a cache lease")
			}
			if out.Len() != 0 {
				t.Fatal("cancelled job published partial diagnostics", out.String())
			}
		})
	}
}

// Target cache waiters can cancel independently; a cancelled producer cannot
// store a negative snapshot and a live waiter retries without inheriting its error.
// Rules: rules/tooling/lsp.md — Multi-target diagnostics, Snapshots, §6.7(2–3).
func TestAnalysisCancellationTargetCache(t *testing.T) {
	var cache targetSnapshotCache
	started := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	producer := make(chan error, 1)
	go func() {
		_, err := cache.loadContext(ctx, "key", func() ([]byte, error) { close(started); <-ctx.Done(); return nil, ctx.Err() })
		producer <- err
	}()
	<-started
	waiterCtx, stopWaiter := context.WithCancel(context.Background())
	stopWaiter()
	if data, err := cache.loadContext(waiterCtx, "key", func() ([]byte, error) { t.Error("cancelled waiter computed"); return nil, nil }); !errors.Is(err, context.Canceled) || data != nil {
		t.Fatal(data, err)
	}
	live := make(chan error, 1)
	go func() {
		data, err := cache.loadContext(context.Background(), "key", func() ([]byte, error) { return []byte("complete"), nil })
		if err == nil && string(data) != "complete" {
			err = errors.New("incomplete target snapshot")
		}
		live <- err
	}()
	cancel()
	for _, done := range []chan error{producer, live} {
		select {
		case err := <-done:
			if done == producer {
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("target waiter stranded")
		}
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if string(cache.entries["key"]) != "complete" || len(cache.flights) != 0 {
		t.Fatal("cancelled target retained partial data")
	}
}

// Cross-target supersession/close cancels an active compiler context while
// retaining worker ownership until that worker exits, preventing overlap.
// Rules: rules/tooling/lsp.md — Multi-target diagnostics, Document snapshots.
func TestAnalysisCancellationCrossTargetWork(t *testing.T) {
	store := newCrossTargetStore()
	ctx, cancel := context.WithCancel(context.Background())
	job := &crossTargetWork{running: true, cancel: cancel}
	store.work = map[string]*crossTargetWork{"uri": job}
	store.mu.Lock()
	store.cancelWorkLocked("uri")
	store.mu.Unlock()
	if !errors.Is(ctx.Err(), context.Canceled) || store.work["uri"] != job || !job.running {
		t.Fatal("active target work was not cancelled safely")
	}
}

// Queue-to-worker handoff is atomic with cancellation: a request removed from
// the pending list is already registered as active when a cancel arrives.
// Rules: rules/tooling/lsp.md — Responsiveness model, §6.7(2–3).
func TestAnalysisCancellationQueueHandoff(t *testing.T) {
	s := &server{}
	q := newRequestQueue()
	q.startAnalysis = func(id json.RawMessage) { s.beginAnalysisRequest(id) }
	q.cancelAnalysis = s.cancelAnalysisRequest
	q.invalidateAnalyses = s.cancelAnalysisRequests
	request := rpcMessage{ID: json.RawMessage(`7`), Method: "textDocument/hover"}
	q.push(request)
	message, disposition, ok := q.next()
	if !ok || disposition != dispositionHandle {
		t.Fatal(message, disposition, ok)
	}
	ctx := s.beginAnalysisRequest(message.ID)
	defer s.finishAnalysisRequest(message.ID)
	q.push(queueNotification("$/cancelRequest", map[string]int{"id": 7}))
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("dequeue lost cancellation")
	}
	s.requestMu.Lock()
	if len(s.analysisRequests) != 1 {
		t.Error("unknown cancellation grew active registry")
	}
	s.requestMu.Unlock()
}

// Real worker queries cancel during compiler polling, return exactly one
// protocol error and drop their active registry entry. A stale source produces
// ContentModified, while an explicit cancellation produces RequestCancelled.
// Rules: rules/tooling/lsp.md — Snapshots, Responsiveness model, §6.7(2–3).
func TestAnalysisCancellationWorkerResponse(t *testing.T) {
	for _, cause := range []error{context.Canceled, analysisContentModified} {
		t.Run(cause.Error(), func(t *testing.T) {
			uri, text := cancellationSource(t)
			enableAnalysisCache()
			defer disableAnalysisCache()
			var out bytes.Buffer
			s := &server{out: &out, documentSnapshots: lspserver.NewDocuments()}
			s.prepareConcurrentState()
			s.documentSnapshots.Open(uri, 1, text)
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(context.Canceled)
			request := &checkpointCancellation{Context: ctx, cancel: func() { cancel(cause) }}
			request.remaining.Store(40)
			s.analysisRequests = map[string]runningAnalysisRequest{"17": {ctx: request, cancel: cancel}}
			params, err := json.Marshal(map[string]any{"textDocument": map[string]string{"uri": uri}, "position": offsetPosition(text, strings.Index(text, "local80"))})
			if err != nil {
				t.Fatal(err)
			}
			s.handleConcurrently(rpcMessage{ID: json.RawMessage(`17`), Method: "textDocument/hover", Params: params}, make(chan struct{}, 1))
			done := make(chan struct{})
			go func() { s.workers.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("cancelled foreground analysis did not exit")
			}
			code := `"code":-32800`
			if cause == analysisContentModified {
				code = `"code":-32801`
			}
			if !bytes.Contains(out.Bytes(), []byte(code)) || bytes.Count(out.Bytes(), []byte(`"id":17`)) != 1 || bytes.Contains(out.Bytes(), []byte(`"result"`)) {
				t.Fatal(out.String())
			}
			s.requestMu.Lock()
			remaining := len(s.analysisRequests)
			s.requestMu.Unlock()
			if remaining != 0 {
				t.Fatal("completed request retained cancellation state")
			}
		})
	}
}
