package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// scheduleModuleDiagnostics coalesces all open siblings into one delayed job.
// Rules: rules/tooling/lsp.md — "Responsiveness model".
func (s *server) scheduleModuleDiagnostics(uri string) {
	if s == nil || s.documentSnapshots == nil {
		return
	}
	dir := normalizedSourcePath(filepath.Dir(pathFromURI(uri)))
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	if s.diagnosticsStopped {
		return
	}
	s.scheduleDiagnosticJobLocked(dir)
}

// scheduleDiagnosticJobLocked requires timerMu and replaces an invalidated job.
// Reanalysis of a changed source universe must not strand old allocation facts.
// Rules: rules/memory/allocation.md — §29(5); rules/tooling/lsp.md — "Snapshots", "Responsiveness model".
func (s *server) scheduleDiagnosticJobLocked(dir string) {
	generation := s.invalidateDiagnosticJob(dir)
	s.diagnosticTimers[dir] = time.AfterFunc(s.diagnosticDelay, func() {
		if err := s.publishDiagnosticBatch(dir, generation); err != nil {
			fmt.Fprintf(os.Stderr, "lsp diagnostics error: %v\n", err)
		}
		s.timerMu.Lock()
		if s.diagnosticGeneration[dir] == generation {
			delete(s.diagnosticTimers, dir)
		}
		s.timerMu.Unlock()
	})
}

// invalidateDiagnosticJob requires timerMu and invalidates queued/running work.
func (s *server) invalidateDiagnosticJob(dir string) uint64 {
	if s.diagnosticTimers == nil {
		s.diagnosticTimers = map[string]*time.Timer{}
	}
	if s.diagnosticGeneration == nil {
		s.diagnosticGeneration = map[string]uint64{}
	}
	if timer := s.diagnosticTimers[dir]; timer != nil {
		timer.Stop()
		delete(s.diagnosticTimers, dir)
	}
	s.diagnosticGeneration[dir]++
	return s.diagnosticGeneration[dir]
}

func (s *server) stopDiagnosticTimers() {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	s.diagnosticsStopped = true
	for dir := range s.diagnosticGeneration {
		s.invalidateDiagnosticJob(dir)
	}
}

func (s *server) stopDiagnosticTimer(uri string) {
	s.timerMu.Lock()
	defer s.timerMu.Unlock()
	s.invalidateDiagnosticJob(normalizedSourcePath(filepath.Dir(pathFromURI(uri))))
}

func (s *server) publishModuleDiagnostics(uri string) error {
	if s == nil || s.documentSnapshots == nil {
		return nil
	}
	dir := normalizedSourcePath(filepath.Dir(pathFromURI(uri)))
	s.timerMu.Lock()
	generation := s.invalidateDiagnosticJob(dir)
	s.timerMu.Unlock()
	return s.publishDiagnosticBatch(dir, generation)
}

// republishOpenDiagnostics invalidates both active and retained cross-plan
// facts before applying watched source/project configuration changes.
// Rules: rules/memory/allocation.md — §29(5); rules/tooling/lsp.md — "Configuration", "Snapshots".
func (s *server) republishOpenDiagnostics() error {
	representatives := map[string]string{}
	for _, snapshot := range s.documentSnapshots.Snapshots() {
		representatives[normalizedSourcePath(filepath.Dir(pathFromURI(snapshot.URI)))] = snapshot.URI
	}
	affected := map[string]bool{}
	for dir := range representatives {
		affected[dir] = true
	}
	s.invalidateCrossTargetDirectories(affected)
	for _, snapshot := range s.documentSnapshots.Snapshots() {
		s.scheduleCrossTargetDiagnostics(snapshot.URI)
	}
	for _, uri := range representatives {
		s.scheduleModuleDiagnostics(uri)
	}
	return nil
}
