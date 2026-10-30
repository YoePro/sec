package ast

import (
	"strings"

	"sec/internal/lexer"
)

// HardwareDeclaration is a contextual `hardware region|resource|endpoint`
// fact declaration (MD-111). Its members use restricted metadata values, not
// expressions; Sema validates them against the category's closed schema.
//
// Rules:
//   - rules/foundations/grammar.md — HardwareDeclaration (Amendment MD-111)
//   - rules/corrections/MD-111-hardware-facts-approved-decision-20261030.md — §§ 3(5), 8(1)
type HardwareDeclaration struct {
	Token     lexer.Token // the contextual `hardware`
	KindToken lexer.Token // `region`, `resource` or `endpoint`
	Category  string
	Name      *Identifier
	Members   []*HardwareMember
	// Invalid marks a declaration whose header or member syntax was
	// malformed and only partially retained for recovery.
	Invalid bool
}

func (hd *HardwareDeclaration) statementNode() {}

func (hd *HardwareDeclaration) TokenLiteral() string { return hd.Token.Lexeme }

func (hd *HardwareDeclaration) String() string {
	name := ""
	if hd.Name != nil {
		name = hd.Name.Value
	}
	members := make([]string, 0, len(hd.Members))
	for _, member := range hd.Members {
		members = append(members, member.String())
	}
	return "hardware " + hd.Category + " " + name + " { " + strings.Join(members, ", ") + " }"
}

// HardwareMember is one `name: value` member.
type HardwareMember struct {
	Name  *Identifier
	Value *HardwareValue
}

func (hm *HardwareMember) String() string {
	value := ""
	if hm.Value != nil {
		value = hm.Value.String()
	}
	return hm.Name.Value + ": " + value
}

// HardwareValueKind is the syntactic form of a metadata value.
type HardwareValueKind string

const (
	HardwareValueInteger HardwareValueKind = "integer"
	HardwareValueString  HardwareValueKind = "string"
	HardwareValueBool    HardwareValueKind = "bool"
	// HardwareValuePath is `Name` or `A.B`: a qualified metadata enum case or
	// a hardware symbol selector, told apart by the member's schema type.
	HardwareValuePath HardwareValueKind = "path"
	HardwareValueList HardwareValueKind = "list"
)

// HardwareValue is one restricted metadata value: an integer literal (Text is
// its exact spelling), a string literal (Text is its unquoted value), a
// boolean, a dotted path or a bracketed list of integer literals.
type HardwareValue struct {
	Kind     HardwareValueKind
	Token    lexer.Token
	Text     string
	Bool     bool
	Path     []*Identifier
	Elements []*HardwareValue
}

func (hv *HardwareValue) String() string {
	switch hv.Kind {
	case HardwareValueString:
		return hv.Token.Lexeme
	case HardwareValueBool:
		if hv.Bool {
			return "true"
		}
		return "false"
	case HardwareValuePath:
		parts := make([]string, 0, len(hv.Path))
		for _, part := range hv.Path {
			parts = append(parts, part.Value)
		}
		return strings.Join(parts, ".")
	case HardwareValueList:
		parts := make([]string, 0, len(hv.Elements))
		for _, element := range hv.Elements {
			parts = append(parts, element.String())
		}
		return "[" + strings.Join(parts, ", ") + "]"
	default:
		return hv.Text
	}
}
