package cst

import (
	"sec/internal/lexer"
	"sec/internal/parser"
)

// RecoveryKind distinguishes the two parser repairs the concrete syntax model
// retains next to the real token tape.
type RecoveryKind string

const (
	// MissingToken is a zero-width virtual token the parser assumed in order
	// to keep the surrounding node. It is synthetic and never part of Text.
	MissingToken RecoveryKind = "missing-token"
	// SkippedTokens is a range of real source elements the parser consumed
	// without assigning them grammar.
	SkippedTokens RecoveryKind = "skipped-tokens"
)

// RecoveryNode attaches one parser recovery event to exact CST elements.
//
// For a SkippedTokens node, First and Last are the element indexes of the
// first and last skipped real tokens, and Span covers them including the
// trivia between them.
//
// For a MissingToken node, Span is zero-width at the insertion anchor.
// AnchorExact reports that the parser proved the insertion point: the
// virtual token sits directly after the real token at element First (Last is
// equal). Otherwise the parser only reported where it noticed the absence;
// First is the element of that reported token and Span is zero-width at its
// start, which tooling must not treat as a proven insertion point.
//
// Located is false when an event token has no matching real element, for
// example a repair inside a re-lexed interpolation hole; First, Last and Span
// are then unset rather than guessed.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Virtual missing tokens", "Skipped tokens are only partly preserved", "Initial recovery event stream"
//   - rules/tooling/formatter.md — § 5 "Required syntax representation", § 25 "Malformed and incomplete source"
type RecoveryNode struct {
	Kind         RecoveryKind
	Span         Span
	First        int
	Last         int
	Located      bool
	AnchorExact  bool
	Expected     []lexer.TokenType
	Skipped      int
	DiagnosticID string
	Confidence   parser.RecoveryConfidence
	Context      parser.RecoveryContext
	Episode      int
}

// Synthetic reports whether the node stands for source text that is absent.
func (n RecoveryNode) Synthetic() bool { return n.Kind == MissingToken }

// ApplyRecovery attaches parser recovery events, in event order, to the
// document's real elements. The events must come from parsing exactly the
// source the document was built from. Applying replaces earlier recovery.
func (d *Document) ApplyRecovery(events []parser.RecoveryEvent) {
	elements := map[int]int{}
	for index, element := range d.Elements {
		if element.Kind == Token || element.Kind == Error {
			elements[element.Span.Start] = index
		}
	}
	find := func(token lexer.Token) (int, bool) {
		index, ok := elements[token.ByteStart]
		if !ok || token.Type == "" {
			return -1, false
		}
		element := d.Elements[index].Token
		if element.Lexeme != token.Lexeme || element.Line != token.Line || element.Column != token.Column {
			return -1, false
		}
		return index, true
	}
	d.Recovery = d.Recovery[:0]
	for _, event := range events {
		node := RecoveryNode{
			First: -1, Last: -1,
			Expected:     append([]lexer.TokenType(nil), event.Expected...),
			Skipped:      event.Skipped,
			DiagnosticID: event.DiagnosticID,
			Confidence:   event.Confidence,
			Context:      event.Context,
			Episode:      event.Episode,
		}
		switch event.Kind {
		case parser.RecoverySkipTokens:
			node.Kind = SkippedTokens
			first, firstOK := find(event.Start)
			last, lastOK := find(event.End)
			if firstOK && lastOK && first <= last {
				node.First, node.Last, node.Located = first, last, true
				node.Span = Span{d.Elements[first].Span.Start, d.Elements[last].Span.End}
			}
		case parser.RecoveryInsertMissingToken:
			node.Kind = MissingToken
			if event.After.Type != "" {
				if after, ok := find(event.After); ok {
					end := d.Elements[after].Span.End
					node.First, node.Last, node.Located, node.AnchorExact = after, after, true, true
					node.Span = Span{end, end}
				}
			} else if reported, ok := find(event.Start); ok {
				start := d.Elements[reported].Span.Start
				node.First, node.Last, node.Located = reported, reported, true
				node.Span = Span{start, start}
			}
		default:
			continue
		}
		d.Recovery = append(d.Recovery, node)
	}
}

// SkippedElement reports whether the element at index lies inside a located
// skipped-token range, including trivia between its first and last token.
func (d Document) SkippedElement(index int) bool {
	for _, node := range d.Recovery {
		if node.Kind == SkippedTokens && node.Located && index >= node.First && index <= node.Last {
			return true
		}
	}
	return false
}
