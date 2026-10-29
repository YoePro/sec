package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type inlayHintLabelPart struct {
	Value    string         `json:"value"`
	Tooltip  *markupContent `json:"tooltip,omitempty"`
	Location *location      `json:"location,omitempty"`
}

type inlayHintResolveData struct {
	Version     int    `json:"version"`
	URI         string `json:"uri"`
	Fingerprint string `json:"fingerprint"`
}

// MarshalJSON preserves the protocol's string-or-parts label union while local
// hint producers and sorting retain a plain display string. Part text is stable
// from listing through resolve; only tooltip/location properties are filled.
// Rules: rules/tooling/lsp.md — "Inlay hints"; LSP 3.17 InlayHint/LabelPart.
func (hint inlayHint) MarshalJSON() ([]byte, error) {
	type plain inlayHint
	var label any = hint.Label
	if len(hint.LabelParts) != 0 {
		label = hint.LabelParts
	}
	return json.Marshal(struct {
		plain
		Label any `json:"label"`
	}{plain: plain(hint), Label: label})
}

// UnmarshalJSON accepts both LSP label forms, retaining part boundaries for
// validation. No client-supplied tooltip or location becomes semantic evidence.
// Rules: rules/tooling/lsp.md — "Inlay hints"; LSP 3.17 InlayHint/LabelPart.
func (hint *inlayHint) UnmarshalJSON(data []byte) error {
	type plain inlayHint
	var wire struct {
		plain
		Label json.RawMessage `json:"label"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	*hint = inlayHint(wire.plain)
	if err := json.Unmarshal(wire.Label, &hint.Label); err == nil && hint.Label != "" {
		return nil
	}
	if err := json.Unmarshal(wire.Label, &hint.LabelParts); err != nil {
		return err
	}
	if len(hint.LabelParts) == 0 {
		return fmt.Errorf("empty inlay hint label")
	}
	var label strings.Builder
	for _, part := range hint.LabelParts {
		if part.Value == "" {
			return fmt.Errorf("empty inlay hint label part")
		}
		label.WriteString(part.Value)
	}
	hint.Label = label.String()
	return nil
}

// handleInlayHintResolve regenerates a hint from the current immutable snapshot
// and compiler facts. A stale hint cannot relocate itself by spelling, and a
// client cannot inject a navigation destination. Work uses the read-only queue.
// Rules: rules/tooling/lsp.md — "Inlay hints", "Document snapshots",
// "Responsiveness model"; LSP 3.17 inlayHint/resolve.
func (s *server) handleInlayHintResolve(message rpcMessage) error {
	var hint inlayHint
	if err := json.Unmarshal(message.Params, &hint); err != nil || hint.Data == nil || hint.Data.Version != 1 || hint.Data.URI == "" || hint.Data.Fingerprint == "" {
		return s.respondError(message.ID, -32602, "invalid inlay hint resolve data")
	}
	if s.documentSnapshots == nil {
		return s.respondError(message.ID, -32801, "inlay hint document is no longer open")
	}
	snapshot, ok := s.documentSnapshots.Snapshot(hint.Data.URI)
	if !ok || sourceContentFingerprint(snapshot.Text) != hint.Data.Fingerprint {
		return s.respondError(message.ID, -32801, "inlay hint source changed; repeat textDocument/inlayHint")
	}
	overlay := s.sourceOverlay()
	requested := lspRange{Start: hint.Position, End: hint.Position}
	for _, current := range inlayHintsForSource(hint.Data.URI, snapshot.Text, requested, s.inlayHintSettings(), overlay) {
		if !sameInlayHintIdentity(hint, current) {
			continue
		}
		enrichInlayHint(&current, hint.Data.URI, snapshot.Text, overlay)
		latest, open := s.documentSnapshots.Snapshot(hint.Data.URI)
		if !open || latest.Version != snapshot.Version || latest.Text != snapshot.Text {
			return s.respondError(message.ID, -32801, "inlay hint source changed during analysis")
		}
		return s.respond(message.ID, current)
	}
	return s.respondError(message.ID, -32602, "inlay hint no longer matches a compiler-resolved hint")
}

// sameInlayHintIdentity compares the immutable display fields, including label
// partitioning, rather than trusting round-tripped decorations.
// Rules: rules/tooling/lsp.md — "Inlay hints"; LSP 3.17 inlayHint/resolve.
func sameInlayHintIdentity(left, right inlayHint) bool {
	if left.Position != right.Position || left.Label != right.Label || left.Kind != right.Kind || left.PaddingLeft != right.PaddingLeft || left.PaddingRight != right.PaddingRight || len(left.LabelParts) != len(right.LabelParts) {
		return false
	}
	for index := range left.LabelParts {
		if left.LabelParts[index].Value != right.LabelParts[index].Value {
			return false
		}
	}
	return true
}

// enrichInlayHint adds Markdown and exact Sema declaration provenance. Built-in,
// generic or compound types with no single physical declaration stay unlinked.
// Ownership/default hints describe the already selected effect/default preview;
// they do not introduce another type, effect or default-selection engine.
// Rules: rules/tooling/lsp.md — "Inlay hints", "Navigation and references".
func enrichInlayHint(hint *inlayHint, uri, text string, overlay sourceOverlay) {
	detail := hint.detail
	if detail == "" {
		switch hint.Label {
		case "ref":
			detail = "Implicit shared borrow for this call."
		case "ref mut":
			detail = "Implicit mutable borrow for this call."
		case "copy":
			detail = "Semantic copy selected by the compiler."
		case "moves if selected":
			detail = "The payload moves only if this match arm is selected; the subject is not moved by pattern testing."
		default:
			detail = "Compiler-resolved omitted field defaults:\n\n" + hint.Label
		}
	} else {
		detail = "```sec\n" + detail + "\n```"
	}
	hint.Tooltip = &markupContent{Kind: "markdown", Value: detail}
	hint.LabelParts[0].Tooltip = hint.Tooltip
	token := hint.declaration
	if !validDefinitionTokenForHint(token) {
		return
	}
	targetURI := uri
	if token.File != "" {
		targetURI = uriFromPath(token.File)
	}
	targetText := sourceTextForToken(uri, text, overlay, token)
	if targetText == "" {
		return
	}
	hint.LabelParts[0].Location = &location{URI: targetURI, Range: definitionTokenRange(targetText, token)}
}

type inlayResolveSupport struct {
	tooltip       bool
	labelTooltip  bool
	labelLocation bool
}

// configureInlayResolve records the exact properties the client can request
// lazily. Clients without resolve support receive the same details eagerly.
// Rules: rules/tooling/lsp.md — "Inlay hints"; LSP 3.17 resolveSupport.
func (s *server) configureInlayResolve(raw json.RawMessage) {
	var params struct {
		Capabilities struct {
			TextDocument struct {
				InlayHint struct {
					ResolveSupport struct {
						Properties []string `json:"properties"`
					} `json:"resolveSupport"`
				} `json:"inlayHint"`
			} `json:"textDocument"`
		} `json:"capabilities"`
	}
	s.inlayResolve = inlayResolveSupport{}
	if json.Unmarshal(raw, &params) != nil {
		return
	}
	for _, property := range params.Capabilities.TextDocument.InlayHint.ResolveSupport.Properties {
		switch property {
		case "tooltip":
			s.inlayResolve.tooltip = true
		case "label.tooltip":
			s.inlayResolve.labelTooltip = true
		case "label.location":
			s.inlayResolve.labelLocation = true
		}
	}
}

// documentInlayHints respects per-property lazy support without making clients
// that cannot resolve lose tooltip or navigation information.
// Rules: rules/tooling/lsp.md — "Inlay hints"; LSP 3.17 resolveSupport.
func (s *server) documentInlayHints(uri, text string, requested lspRange) []inlayHint {
	overlay := s.sourceOverlay()
	hints := inlayHintsForSource(uri, text, requested, s.inlayHintSettings(), overlay)
	if s.inlayResolve.tooltip && s.inlayResolve.labelTooltip && s.inlayResolve.labelLocation {
		return hints
	}
	for index := range hints {
		hint := &hints[index]
		enrichInlayHint(hint, uri, text, overlay)
		if s.inlayResolve.tooltip {
			hint.Tooltip = nil
		}
		if s.inlayResolve.labelTooltip {
			hint.LabelParts[0].Tooltip = nil
		}
		if s.inlayResolve.labelLocation {
			hint.LabelParts[0].Location = nil
		}
	}
	return hints
}
