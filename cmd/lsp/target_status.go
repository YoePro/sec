package main

import (
	"encoding/json"
	"net/url"
	"os"
)

type targetStatusRow struct {
	Name        string           `json:"name"`
	Active      bool             `json:"active"`
	Current     bool             `json:"current"`
	Diagnostics []diagnostic     `json:"diagnostics"`
	Option      *lspTargetOption `json:"option,omitempty"`
}

type targetStatusResult struct {
	URI     string            `json:"uri"`
	Version int               `json:"version"`
	State   string            `json:"state"`
	Active  string            `json:"active"`
	Options []lspTargetOption `json:"options"`
	Targets []targetStatusRow `json:"targets"`
}

// targetStatusFor projects the existing canonical comparison snapshot without
// running Sema or manufacturing a successful empty result. Only diagnostics
// for the current open text may be shown; target/manifest invalidation remains
// owned by the existing comparison scheduler.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Document snapshots.
func (s *server) targetStatusFor(uri string, options []lspTargetOption) targetStatusResult {
	status := targetStatusResult{URI: uri, State: "unavailable", Options: options, Targets: []targetStatusRow{}}
	for _, option := range options {
		option := option
		status.Targets = append(status.Targets, targetStatusRow{Name: option.diagnosticName(), Active: option.Active, Option: &option})
	}
	if s.documentSnapshots == nil {
		return status
	}
	snapshot, ok := s.documentSnapshots.Snapshot(uri)
	if !ok {
		return status
	}
	status.Version = snapshot.Version
	result, ok := s.crossTargetResultFor(uri, snapshot.Text)
	if !ok {
		return status
	}
	// A comparison must include every declared target before any row can
	// imply a completed comparison. Empty-but-present diagnostics are valid.
	if len(result.targets) == 0 {
		return status
	}
	for _, name := range result.targets {
		if _, exists := result.perTarget[name]; !exists {
			return status
		}
	}
	if len(options) != 0 {
		if len(options) != len(result.targets) {
			return status
		}
		for _, option := range options {
			if _, exists := result.perTarget[option.diagnosticName()]; !exists {
				return status
			}
		}
	}
	status.State, status.Active = "current", result.active
	status.Targets = nil
	for _, name := range result.targets {
		// Allocate a detached slice, including [] for a completed clean row.
		diagnostics := append([]diagnostic{}, result.perTarget[name]...)
		row := targetStatusRow{Name: name, Active: name == result.active, Current: true, Diagnostics: diagnostics}
		for _, option := range options {
			if option.diagnosticName() == name {
				option := option
				row.Option = &option
				break
			}
		}
		status.Targets = append(status.Targets, row)
	}
	return status
}

// handleTargetStatusRequest exposes compiler-owned target options and stored
// per-target diagnostics to clients. It does not select a target or start an
// analysis; invalid requests cannot change the current project selection.
// Rules: rules/tooling/lsp.md — §§23.1–23.2, Responsiveness model.
func (s *server) handleTargetStatusRequest(message rpcMessage) error {
	var params struct {
		TextDocument textDocumentIdentifier `json:"textDocument"`
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		return s.respondError(message.ID, -32602, err.Error())
	}
	uri := params.TextDocument.URI
	parsed, err := url.Parse(uri)
	path := pathFromURI(uri)
	if err != nil || parsed.Scheme != "file" || path == "" {
		return s.respondError(message.ID, -32602, "a file document URI is required")
	}
	options, err := lspTargetOptions(path, s.sourceOverlay())
	if err != nil && !os.IsNotExist(err) {
		return s.respondError(message.ID, -32602, err.Error())
	}
	if options == nil {
		options = []lspTargetOption{}
	}
	return s.respond(message.ID, s.targetStatusFor(uri, options))
}
