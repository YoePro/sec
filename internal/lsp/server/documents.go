// Package server contains reusable state for the Sec language server.
package server

import "sync"

// Snapshot is an immutable view of one open document at a specific LSP version.
type Snapshot struct {
	URI     string
	Version int
	Text    string
}

// Documents owns open-document overlays. It accepts full-text changes for now;
// incremental range edits can be added without changing callers' snapshot API.
type Documents struct {
	mu   sync.RWMutex
	open map[string]Snapshot
}

func NewDocuments() *Documents { return &Documents{open: map[string]Snapshot{}} }

func (d *Documents) Open(uri string, version int, text string) Snapshot {
	snapshot := Snapshot{URI: uri, Version: version, Text: text}
	d.mu.Lock()
	d.open[uri] = snapshot
	d.mu.Unlock()
	return snapshot
}

// Change ignores stale versions and returns the snapshot that remains current.
func (d *Documents) Change(uri string, version int, text string) (Snapshot, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	current, exists := d.open[uri]
	if !exists || version <= current.Version {
		return current, false
	}
	next := Snapshot{URI: uri, Version: version, Text: text}
	d.open[uri] = next
	return next, true
}

func (d *Documents) Snapshot(uri string) (Snapshot, bool) {
	d.mu.RLock()
	snapshot, ok := d.open[uri]
	d.mu.RUnlock()
	return snapshot, ok
}

func (d *Documents) Close(uri string) {
	d.mu.Lock()
	delete(d.open, uri)
	d.mu.Unlock()
}
