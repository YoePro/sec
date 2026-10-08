package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/layout"
	lspserver "sec/internal/lsp/server"
	platformtarget "sec/internal/platform/target"
	"sec/internal/sema"
)

type lspTargetSelection struct {
	LogicalTarget string `json:"logicalTarget"`
	Variant       string `json:"variant"`
}

type lspTargetOption struct {
	lspTargetSelection
	OS               string `json:"os"`
	Arch             string `json:"arch"`
	PointerWidthBits uint16 `json:"pointerWidthBits"`
	Active           bool   `json:"active"`
}

// selectedLSPPlan resolves an immutable request's explicit project selection
// against current manifest declarations and the shared compiler target registry.
// Rules: rules/tooling/lsp.md — Target-aware analysis, Snapshots;
// rules/projects/projects.md — Targets, Variants, Compilation plans and target lowering.
func selectedLSPPlan(path string, overlay sourceOverlay) (layout.ResolvedScalarPlan, bool) {
	if path == "" {
		return layout.ResolvedScalarPlan{}, false
	}
	root := normalizedSourcePath(findProjectRoot(path))
	selection, ok := overlay.Targets[root]
	if !ok {
		return layout.ResolvedScalarPlan{}, false
	}
	variants, targets, err := readLSPProjectTargets(filepath.Join(root, ".sec", "sec.toml"))
	if err != nil {
		return layout.ResolvedScalarPlan{}, false
	}
	target, ok := targets[selection.LogicalTarget]
	if !ok {
		return layout.ResolvedScalarPlan{}, false
	}
	for _, name := range target.variants {
		if name != selection.Variant {
			continue
		}
		variant, ok := variants[name]
		if !ok {
			return layout.ResolvedScalarPlan{}, false
		}
		definition, ok := platformtarget.Find(platformtarget.Target{OS: platformtarget.NormalizeOS(variant.os), Arch: platformtarget.NormalizeArch(variant.arch)})
		if !ok {
			return layout.ResolvedScalarPlan{}, false
		}
		plan, err := definition.ScalarPlan()
		return plan, err == nil
	}
	return layout.ResolvedScalarPlan{}, false
}

// lspTargetOptions exposes canonical project variants without silently picking
// a scalar plan from a project with distinct variants. Selection is per project.
// Rules: rules/tooling/lsp.md — Target-aware analysis, Target status;
// rules/projects/projects.md — Targets and Variants.
func lspTargetOptions(path string, overlay sourceOverlay) ([]lspTargetOption, error) {
	root := normalizedSourcePath(findProjectRoot(path))
	variants, targets, err := readLSPProjectTargets(filepath.Join(root, ".sec", "sec.toml"))
	if err != nil {
		return nil, err
	}
	selection := overlay.Targets[root]
	options := []lspTargetOption{}
	names := []string{}
	for name := range targets {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		target := targets[name]
		if target.source == "" {
			continue
		}
		for _, variantName := range target.variants {
			variant, ok := variants[variantName]
			if !ok {
				return nil, fmt.Errorf("target %q references unknown variant %q", name, variantName)
			}
			definition, ok := platformtarget.Find(platformtarget.Target{OS: platformtarget.NormalizeOS(variant.os), Arch: platformtarget.NormalizeArch(variant.arch)})
			if !ok {
				return nil, fmt.Errorf("variant %q has unsupported target %s-%s", variantName, variant.os, variant.arch)
			}
			plan, err := definition.ScalarPlan()
			if err != nil {
				return nil, err
			}
			candidate := lspTargetSelection{LogicalTarget: name, Variant: variantName}
			options = append(options, lspTargetOption{lspTargetSelection: candidate, OS: definition.OS, Arch: definition.Arch, PointerWidthBits: plan.PointerWidthBits, Active: selection == candidate})
		}
	}
	return options, nil
}

// handleTargetRequest lists or changes the selected project variant. Invalid
// requests preserve the prior selection; switching invalidates diagnostic jobs
// and recomputes every open source and per-variant result without a restart.
// Rules: rules/tooling/lsp.md — Target-aware analysis, Multi-target diagnostics,
// Snapshots, Configuration; rules/platform/platform_model.md — source selection by target.
func (s *server) handleTargetRequest(message rpcMessage) error {
	var params struct {
		URI string `json:"uri"`
		lspTargetSelection
	}
	if err := json.Unmarshal(message.Params, &params); err != nil {
		return s.respondError(message.ID, -32602, err.Error())
	}
	documentURI, uriErr := url.Parse(params.URI)
	path := pathFromURI(params.URI)
	if uriErr != nil || documentURI.Scheme != "file" || path == "" {
		return s.respondError(message.ID, -32602, "a file document URI is required")
	}
	options, err := lspTargetOptions(path, s.sourceOverlay())
	if err != nil {
		return s.respondError(message.ID, -32602, err.Error())
	}
	if message.Method == "sec/targets" {
		return s.respond(message.ID, options)
	}
	reset := params.LogicalTarget == "" && params.Variant == ""
	found := reset
	for _, option := range options {
		if option.lspTargetSelection == params.lspTargetSelection {
			found = true
		}
	}
	if !found {
		return s.respondError(message.ID, -32602, "selection is not a declared target variant")
	}
	root := normalizedSourcePath(findProjectRoot(path))
	s.timerMu.Lock()
	if s.targetSelections == nil {
		s.targetSelections = map[string]lspTargetSelection{}
	}
	if reset {
		delete(s.targetSelections, root)
	} else {
		s.targetSelections[root] = params.lspTargetSelection
	}
	s.timerMu.Unlock()
	if err := s.republishOpenDiagnostics(); err != nil {
		return err
	}
	options, _ = lspTargetOptions(path, s.sourceOverlay())
	return s.respond(message.ID, options)
}

// newLSPAnalyzerWithOverlay applies the same immutable selected variant as
// source assembly, while retaining a document's explicit #target precedence.
// Rules: rules/tooling/lsp.md — Target-aware analysis, Snapshots;
// rules/types/types.md — int and uint, Binary floating-point types.
func newLSPAnalyzerWithOverlay(uri string, program *ast.Program, overlay sourceOverlay) *sema.Analyzer {
	return newLSPAnalyzerForInputs(uri, program, overlay)
}

// sourceOverlay captures source text and selected project targets together.
// Rules: rules/tooling/lsp.md — Snapshots and Target-aware analysis.
type sourceOverlay struct {
	Sources lspserver.SourceOverlay
	Targets map[string]lspTargetSelection
}

// sourceOverlay copies mutable server configuration into an immutable request view.
// Rules: rules/tooling/lsp.md — Snapshots, Target-aware analysis.
func (s *server) sourceOverlay() sourceOverlay {
	overlay := sourceOverlay{Sources: map[string]string{}}
	if s == nil {
		return overlay
	}
	s.timerMu.Lock()
	overlay.Targets = map[string]lspTargetSelection{}
	for root, selection := range s.targetSelections {
		overlay.Targets[root] = selection
	}
	s.timerMu.Unlock()
	if s.documentSnapshots == nil {
		return overlay
	}
	for _, snapshot := range s.documentSnapshots.Snapshots() {
		path := pathFromURI(snapshot.URI)
		if path != "" {
			overlay.Sources[normalizedSourcePath(path)] = snapshot.Text
		}
	}
	return overlay
}

// lspDiagnosticTargetOptions selects all declared variants of the active logical
// product, retaining variant identity even when two share a platform plan.
// Rules: rules/tooling/lsp.md — Multi-target diagnostics;
// rules/projects/projects.md — §§17(10),19(10),20(5–6).
func lspDiagnosticTargetOptions(path string, overlay sourceOverlay) []lspTargetOption {
	root := normalizedSourcePath(findProjectRoot(path))
	selection, selected := overlay.Targets[root]
	if !selected {
		_, targets, err := readLSPProjectTargets(filepath.Join(root, ".sec", "sec.toml"))
		if err != nil {
			return nil
		}
		owned, err := selectLSPManifestTarget(root, path, targets)
		if err != nil {
			return nil
		}
		for name, target := range targets {
			if target.source == owned.source && strings.Join(target.variants, "\x00") == strings.Join(owned.variants, "\x00") {
				selection.LogicalTarget = name
				break
			}
		}
	}
	options, err := lspTargetOptions(path, overlay)
	if err != nil {
		return nil
	}
	out := []lspTargetOption{}
	for _, option := range options {
		if option.LogicalTarget == selection.LogicalTarget {
			out = append(out, option)
		}
	}
	return out
}

// diagnosticName preserves both logical product and output-variant applicability.
// Rules: rules/tooling/lsp.md — Multi-target diagnostics; projects/projects.md §19(10).
func (option lspTargetOption) diagnosticName() string {
	return option.LogicalTarget + "/" + option.Variant + " (" + option.OS + "-" + option.Arch + ")"
}
