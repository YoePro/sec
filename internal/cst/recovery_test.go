package cst

import (
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

func recoveredDocument(t *testing.T, source string) Document {
	t.Helper()
	document := Build(source, "")
	document.ApplyRecovery(parser.New(lexer.New(source)).Parse().Recovery)
	if got := document.Text(); got != source {
		t.Fatalf("recovery changed the lossless text:\n%q\nwant\n%q", got, source)
	}
	return document
}

// Rules:
//   - rules/compiler/parser_recovery.md — "Virtual missing tokens", "Missing comma"
func TestRecoveryAttachesProvenMissingSeparatorAfterItsAnchor(t *testing.T) {
	source := "module main\n\ntype P struct {\n    x: int // first\n    y: int\n}\n"
	document := recoveredDocument(t, source)
	if len(document.Recovery) != 1 {
		t.Fatalf("recovery = %+v", document.Recovery)
	}
	node := document.Recovery[0]
	anchor := len("module main\n\ntype P struct {\n    x: int")
	if node.Kind != MissingToken || !node.Synthetic() || !node.Located || !node.AnchorExact ||
		node.Span != (Span{anchor, anchor}) || len(node.Expected) != 1 || node.Expected[0] != lexer.COMMA {
		t.Fatalf("missing separator = %+v", node)
	}
	if element := document.Elements[node.First]; element.Text != "int" || element.Span.End != anchor {
		t.Fatalf("anchor element = %+v", element)
	}
	if node.DiagnosticID == "" || node.Confidence != parser.RecoveryUnambiguous {
		t.Fatalf("diagnostic identity = %+v", node)
	}
}

// A missing token without a proven insertion point keeps the reported token
// as its location but is not presented as an exact anchor.
func TestRecoveryKeepsUnprovenMissingTokenAnchorsInexact(t *testing.T) {
	source := "module main\n\nfn F() void {\n    let x := (1 + 2\n}\n"
	document := recoveredDocument(t, source)
	if len(document.Recovery) != 1 {
		t.Fatalf("recovery = %+v", document.Recovery)
	}
	node := document.Recovery[0]
	closer := len(source) - len("}\n")
	if node.Kind != MissingToken || node.AnchorExact || !node.Located || node.Span != (Span{closer, closer}) ||
		document.Elements[node.First].Text != "}" || node.Expected[0] != lexer.RPAREN {
		t.Fatalf("missing ')' = %+v", node)
	}
}

// Rules:
//   - rules/compiler/parser_recovery.md — "Skipped tokens are only partly preserved"
//   - rules/tooling/formatter.md — § 25 "Malformed and incomplete source"
func TestRecoveryAttachesSkippedRangesToRealElements(t *testing.T) {
	source := "module main\n\nfn F() void {\n    let x := 1 ) ) 2\n    let y := 3\n}\n"
	document := recoveredDocument(t, source)
	var skipped []string
	for _, node := range document.Recovery {
		if node.Kind != SkippedTokens {
			continue
		}
		if !node.Located || node.Synthetic() || node.Skipped == 0 {
			t.Fatalf("skipped node = %+v", node)
		}
		skipped = append(skipped, source[node.Span.Start:node.Span.End])
		for index := node.First; index <= node.Last; index++ {
			if !document.SkippedElement(index) {
				t.Fatalf("element %d inside %+v is not reported as skipped", index, node)
			}
		}
	}
	if len(skipped) != 2 || skipped[0] != ")" || skipped[1] != ") 2" {
		t.Fatalf("skipped ranges = %q", skipped)
	}
	for index, element := range document.Elements {
		if element.Text == "y" && document.SkippedElement(index) {
			t.Fatal("the following valid statement is marked skipped")
		}
	}
}

func TestRecoveryLeavesUnmatchedEventTokensUnlocated(t *testing.T) {
	document := Build("module main\n", "")
	document.ApplyRecovery([]parser.RecoveryEvent{
		{Kind: parser.RecoverySkipTokens, Start: lexer.Token{Type: lexer.IDENT, Lexeme: "ghost", ByteStart: 7, Line: 1, Column: 8}, End: lexer.Token{Type: lexer.IDENT, Lexeme: "ghost", ByteStart: 7, Line: 1, Column: 8}},
		{Kind: parser.RecoveryInsertMissingToken, Expected: []lexer.TokenType{lexer.COMMA}},
	})
	for _, node := range document.Recovery {
		if node.Located || node.First != -1 || node.Span != (Span{}) {
			t.Fatalf("unmatched event was located: %+v", node)
		}
	}
	document.ApplyRecovery(nil)
	if len(document.Recovery) != 0 {
		t.Fatal("ApplyRecovery did not replace earlier recovery")
	}
}

func TestValidSourceHasNoRecoveryNodes(t *testing.T) {
	if document := recoveredDocument(t, "module main\n\nfn F(a: int, b: int) int {\n    return a + b\n}\n"); len(document.Recovery) != 0 {
		t.Fatalf("recovery = %+v", document.Recovery)
	}
}
