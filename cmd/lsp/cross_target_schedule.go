package main

import (
	"context"
	"time"
)

type crossTargetRequest struct {
	generation uint64
	text       string
	overlay    sourceOverlay
}

type crossTargetWork struct {
	timer   *time.Timer
	pending *crossTargetRequest
	ready   bool
	running bool
	cancel  context.CancelFunc
}

// scheduleCrossTargetDiagnostics coalesces open/change/save and dependency
// refreshes against the latest immutable text/overlay. At most one comparison
// runs per open document; a running worker retains only the latest pending
// request and may publish only its still-current generation.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Document snapshots, Responsiveness model;
// rules/compiler/incremental_compilation.md — §§4,7,13.
func (s *server) scheduleCrossTargetDiagnostics(uri string) {
	if s == nil || s.documentSnapshots == nil || isProjectManifest(pathFromURI(uri)) {
		return
	}
	s.timerMu.Lock()
	stopped := s.diagnosticsStopped
	delay := s.diagnosticDelay
	s.timerMu.Unlock()
	if stopped {
		return
	}
	snapshot, ok := s.documentSnapshots.Snapshot(uri)
	if !ok {
		return
	}
	if s.crossTarget == nil {
		s.crossTarget = newCrossTargetStore()
	}
	store := s.crossTarget
	overlay := s.sourceOverlay()
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.stopped {
		return
	}
	if store.work == nil {
		store.work = map[string]*crossTargetWork{}
	}
	job := store.work[uri]
	if job == nil {
		job = &crossTargetWork{}
		store.work[uri] = job
	}
	if job.timer != nil {
		job.timer.Stop()
	}
	store.generation[uri]++
	delete(store.results, uri)
	if job.cancel != nil {
		job.cancel()
	}
	job.pending = &crossTargetRequest{generation: store.generation[uri], text: snapshot.Text, overlay: overlay}
	job.ready = false
	generation := store.generation[uri]
	job.timer = time.AfterFunc(delay, func() { s.runCrossTargetWork(uri, job, generation) })
}

// runCrossTargetWork marks a debounced request ready and serializes comparisons.
// A newer edit supersedes both pending and running results. New edits wait for
// their own debounce before the existing worker may pick them up.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Document snapshots, Responsiveness model;
// rules/compiler/incremental_compilation.md — §§4,7,13.
func (s *server) runCrossTargetWork(uri string, job *crossTargetWork, generation uint64) {
	store := s.crossTarget
	store.mu.Lock()
	if store.stopped || store.work[uri] != job || store.generation[uri] != generation || job.pending == nil {
		store.mu.Unlock()
		return
	}
	job.ready = true
	if job.running {
		store.mu.Unlock()
		return
	}
	job.running = true
	for {
		request := job.pending
		job.pending = nil
		job.ready = false
		ctx, cancel := context.WithCancel(context.Background())
		job.cancel = cancel
		store.mu.Unlock()
		result, ok := computeCrossTargetDiagnosticsContext(ctx, uri, request.text, request.overlay)
		cancel()
		store.mu.Lock()
		job.cancel = nil
		current := !store.stopped && store.work[uri] == job && store.generation[uri] == request.generation
		if current && ok {
			store.results[uri] = result
		}
		if current && !ok {
			delete(store.results, uri)
		}
		store.mu.Unlock()
		if current && ok {
			s.scheduleModuleDiagnostics(uri)
		}
		store.mu.Lock()
		if store.stopped || store.work[uri] != job || !job.ready || job.pending == nil {
			job.running = false
			if job.pending == nil && store.work[uri] == job {
				delete(store.work, uri)
			}
			store.mu.Unlock()
			return
		}
	}
}

// cancelWorkLocked withdraws queued comparisons and supersedes running results.
// The caller holds store.mu. A running job stays registered until it exits, so
// closing/reopening a URI cannot start a second concurrent worker for that URI.
// Rules: rules/tooling/lsp.md — Document synchronization, Document snapshots.
func (store *crossTargetStore) cancelWorkLocked(uri string) {
	store.generation[uri]++
	delete(store.results, uri)
	if job := store.work[uri]; job != nil {
		if job.cancel != nil {
			job.cancel()
		}
		if job.timer != nil {
			job.timer.Stop()
		}
		job.pending = nil
		job.ready = false
		if !job.running {
			delete(store.work, uri)
		}
	}
}
