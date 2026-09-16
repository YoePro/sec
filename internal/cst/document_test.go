package cst

import (
	"strings"
	"testing"

	"sec/internal/lexer"
)

// rules/tooling/formatter.md "Source model" requires every real source byte,
// including invalid bytes and trivia, to survive the lossless syntax layer.
func TestBuildRetainsEverySourceByteAndLexerToken(t *testing.T) {
	source := "\ufeffmodule main\r\n\tlet π := `a//b`  // note\r\n\xff\r\n"
	document := Build(source, "lossless.sec")
	if got := document.Text(); got != source {
		t.Fatalf("round trip = %q, want %q", got, source)
	}
	if document.Elements[0].Kind != BOM || document.Elements[0].Span != (Span{0, 3}) {
		t.Fatalf("initial BOM = %+v", document.Elements[0])
	}
	if document.EOF.Type != lexer.EOF || len(document.Diagnostics) == 0 {
		t.Fatalf("EOF = %+v, diagnostics = %+v", document.EOF, document.Diagnostics)
	}

	previousEnd := 0
	var foundRaw, foundComment, foundError, foundCRLF bool
	for _, element := range document.Elements {
		if element.Span.Start != previousEnd || element.Span.End <= element.Span.Start ||
			element.Text != source[element.Span.Start:element.Span.End] {
			t.Fatalf("non-lossless element after byte %d: %+v", previousEnd, element)
		}
		previousEnd = element.Span.End
		if element.Kind == Token && element.Token.Type == lexer.RAW_STRING && element.Text == "`a//b`" {
			foundRaw = true
		}
		if element.Kind == Comment && element.Text == "// note" {
			foundComment = true
		}
		if element.Kind == Error && element.Text == "\xff" {
			foundError = true
		}
		if element.Kind == Whitespace && strings.Contains(element.Text, "\r\n") {
			foundCRLF = true
		}
	}
	if previousEnd != len(source) || !foundRaw || !foundComment || !foundError || !foundCRLF {
		t.Fatalf("incomplete source coverage: end=%d, elements=%+v", previousEnd, document.Elements)
	}
}

func TestBuildRetainsNestedCommentsAndInterpolatedLiteralAsSingleTokens(t *testing.T) {
	source := "/* outer /* inner */ end */\nlet value := $\"// {Call(1)}\" // real\n"
	document := Build(source, "nested.sec")
	if got := document.Text(); got != source {
		t.Fatalf("round trip = %q, want %q", got, source)
	}
	if len(document.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", document.Diagnostics)
	}
	var comments, interpolated int
	for _, element := range document.Elements {
		if element.Kind == Comment {
			comments++
		}
		if element.Token.Type == lexer.INTERPSTRING {
			interpolated++
		}
	}
	if comments != 2 || interpolated != 1 {
		t.Fatalf("comments = %d, interpolated strings = %d; elements = %+v", comments, interpolated, document.Elements)
	}
}

func TestBuildRetainsUnexpectedBOMAsErrorElement(t *testing.T) {
	source := "let value := 1 \ufeff // trailing\n"
	document := Build(source, "bom.sec")
	if document.Text() != source {
		t.Fatalf("round trip = %q, want %q", document.Text(), source)
	}
	found := false
	for _, element := range document.Elements {
		if element.Text == "\ufeff" && element.Kind == Error {
			found = true
		}
	}
	if !found || len(document.Diagnostics) == 0 {
		t.Fatalf("later BOM not retained as diagnosed error: %+v, %+v", document.Elements, document.Diagnostics)
	}
}
