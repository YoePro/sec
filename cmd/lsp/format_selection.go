package main

import (
	"encoding/json"

	"sec/internal/formatter"
)

type selectionFormattingParams struct {
	formattingParams
	Range    *lspRange `json:"range"`
	Position *position `json:"position"`
	Ch       string    `json:"ch"`
}

// handleSelectionFormatting projects range/on-type requests onto the shared
// formatter. Complete syntax units can expand the selection; unrelated source
// stays byte-exact. Unsupported triggers and uncertain syntax produce no edits.
// Rules: rules/tooling/formatter.md §§4(22),5,25,28,29;
// rules/tooling/lsp.md Appendix A.8.
func (s *server) handleSelectionFormatting(message rpcMessage) error {
	var params selectionFormattingParams
	if err := json.Unmarshal(message.Params, &params); err != nil || params.TextDocument.URI == "" {
		return s.respondError(message.ID, -32602, "invalid formatting request")
	}
	text, err := s.formattingText(params.TextDocument.URI)
	if err != nil {
		return s.respondError(message.ID, -32603, err.Error())
	}
	start, end, cursor := 0, 0, 0
	if message.Method == "textDocument/rangeFormatting" {
		if params.Range == nil {
			return s.respondError(message.ID, -32602, "missing formatting range")
		}
		var validStart, validEnd bool
		start, validStart = formattingOffset(text, params.Range.Start)
		end, validEnd = formattingOffset(text, params.Range.End)
		if !validStart || !validEnd || end < start {
			return s.respondError(message.ID, -32602, "invalid formatting range")
		}
	} else {
		if params.Position == nil || params.Ch == "" {
			return s.respondError(message.ID, -32602, "missing on-type position or character")
		}
		var valid bool
		cursor, valid = formattingOffset(text, *params.Position)
		if !valid {
			return s.respondError(message.ID, -32602, "invalid on-type position")
		}
	}
	options, err := formattingOptionsForDocument(params.TextDocument.URI, params.Options.PresentationWidth)
	if err != nil {
		return s.respondError(message.ID, -32603, err.Error())
	}
	var edit formatter.Edit
	var changed bool
	if message.Method == "textDocument/onTypeFormatting" {
		edit, changed = formatter.OnTypeEdit(formatter.Source{Text: text}, options, cursor, params.Ch)
	} else {
		edit, changed = formatter.RangeEdit(formatter.Source{Text: text}, options, start, end)
	}
	return s.respond(message.ID, formattingEdits(text, edit, changed))
}

// formattingOffset rejects positions that would otherwise clamp to a different
// line or split a UTF-16 surrogate pair, before the shared byte-boundary check.
// Rules: rules/tooling/lsp.md — protocol positions; formatter.md §29.
func formattingOffset(text string, pos position) (int, bool) {
	if pos.Line < 0 || pos.Character < 0 {
		return 0, false
	}
	offset := lineCharToOffset(text, pos.Line, pos.Character)
	return offset, offsetPosition(text, offset) == pos
}
