package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// lineWidth is the target width of folded `>-` prose, indentation included.
const lineWidth = 80

// render writes a fragment in the canonical governance layout:
//
//   - two spaces per nesting level, with list items indented below their key;
//   - strings unquoted when YAML reads the plain spelling back as the same
//     string, otherwise double-quoted;
//   - folded `>-` prose kept folded and re-wrapped at lineWidth;
//   - non-string scalars (numbers, booleans, null, timestamps) written with
//     their original spelling, so formatting never changes a value's type;
//   - one blank line between the entries of the integrations list.
//
// Mapping key order is never changed.
func render(root *yaml.Node) ([]byte, error) {
	var lines []string
	var err error
	switch root.Kind {
	case yaml.MappingNode:
		lines, err = renderMapping(root, 0, true)
	case yaml.SequenceNode:
		lines, err = renderSequence(root, 0, true)
	default:
		return nil, fmt.Errorf("a governance fragment must be a mapping or a list")
	}
	if err != nil {
		return nil, err
	}
	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func renderMapping(node *yaml.Node, indent int, topLevel bool) ([]string, error) {
	if len(node.Content) == 0 {
		return []string{spaces(indent) + "{}"}, nil
	}
	var lines []string
	for i := 0; i+1 < len(node.Content); i += 2 {
		key, err := inlineScalar(node.Content[i])
		if err != nil {
			return nil, err
		}
		value := node.Content[i+1]
		prefix := spaces(indent) + key + ":"
		switch value.Kind {
		case yaml.ScalarNode:
			if folded, ok := foldedLines(value, indent+2); ok {
				lines = append(lines, prefix+" >-")
				lines = append(lines, folded...)
				continue
			}
			text, err := inlineScalar(value)
			if err != nil {
				return nil, err
			}
			lines = append(lines, prefix+" "+text)
		case yaml.SequenceNode:
			if len(value.Content) == 0 {
				lines = append(lines, prefix+" []")
				continue
			}
			nested, err := renderSequence(value, indent+2, topLevel && key == "integrations")
			if err != nil {
				return nil, err
			}
			lines = append(lines, prefix)
			lines = append(lines, nested...)
		case yaml.MappingNode:
			if len(value.Content) == 0 {
				lines = append(lines, prefix+" {}")
				continue
			}
			nested, err := renderMapping(value, indent+2, false)
			if err != nil {
				return nil, err
			}
			lines = append(lines, prefix)
			lines = append(lines, nested...)
		default:
			return nil, fmt.Errorf("line %d: unsupported YAML node", value.Line)
		}
	}
	return lines, nil
}

// renderSequence writes `- item` entries. A nested block is rendered two
// columns deeper and its first line is overlaid with the `- ` marker, so a
// mapping item starts on the dash line. integrationList separates entries with
// one blank line.
func renderSequence(node *yaml.Node, indent int, integrationList bool) ([]string, error) {
	var lines []string
	for index, item := range node.Content {
		if integrationList && index > 0 {
			lines = append(lines, "")
		}
		marker := spaces(indent) + "- "
		switch item.Kind {
		case yaml.ScalarNode:
			if folded, ok := foldedLines(item, indent+2); ok {
				lines = append(lines, marker+">-")
				lines = append(lines, folded...)
				continue
			}
			text, err := inlineScalar(item)
			if err != nil {
				return nil, err
			}
			lines = append(lines, marker+text)
		case yaml.SequenceNode, yaml.MappingNode:
			if len(item.Content) == 0 {
				empty := "[]"
				if item.Kind == yaml.MappingNode {
					empty = "{}"
				}
				lines = append(lines, marker+empty)
				continue
			}
			var nested []string
			var err error
			if item.Kind == yaml.MappingNode {
				nested, err = renderMapping(item, indent+2, false)
			} else {
				nested, err = renderSequence(item, indent+2, false)
			}
			if err != nil {
				return nil, err
			}
			nested[0] = marker + nested[0][indent+2:]
			lines = append(lines, nested...)
		default:
			return nil, fmt.Errorf("line %d: unsupported YAML node", item.Line)
		}
	}
	return lines, nil
}

// inlineScalar spells a scalar on one line.
func inlineScalar(node *yaml.Node) (string, error) {
	if node.Kind != yaml.ScalarNode {
		return "", fmt.Errorf("line %d: expected a scalar", node.Line)
	}
	if node.Style&yaml.TaggedStyle != 0 {
		return "", fmt.Errorf("line %d: explicit YAML tags are not supported", node.Line)
	}
	if node.ShortTag() != "!!str" {
		if node.ShortTag() == "!!null" && node.Value == "" {
			return "null", nil
		}
		return node.Value, nil
	}
	if plainSafe(node.Value) {
		return node.Value, nil
	}
	return doubleQuoted(node.Value), nil
}

// foldedLines re-wraps a string written in folded `>-` style. Only single-line
// prose with single spaces round-trips through folding; anything else falls
// back to an inline scalar.
func foldedLines(node *yaml.Node, indent int) ([]string, bool) {
	value := node.Value
	if node.Style&yaml.FoldedStyle == 0 || node.ShortTag() != "!!str" || value == "" ||
		strings.ContainsAny(value, "\n\r\t") || strings.Contains(value, "  ") ||
		strings.TrimSpace(value) != value {
		return nil, false
	}
	width := lineWidth - indent
	if width < 40 {
		width = 40
	}
	var lines []string
	current := ""
	for _, word := range strings.Split(value, " ") {
		if current != "" && utf8.RuneCountInString(current)+1+utf8.RuneCountInString(word) > width {
			lines = append(lines, spaces(indent)+current)
			current = word
			continue
		}
		if current == "" {
			current = word
		} else {
			current += " " + word
		}
	}
	lines = append(lines, spaces(indent)+current)
	return lines, true
}

// plainSafe reports whether YAML reads s written without quotes back as the
// same string in block context.
func plainSafe(s string) bool {
	if s == "" || strings.TrimSpace(s) != s || strings.ContainsAny(s, "\n\r\t") ||
		strings.Contains(s, ": ") || strings.Contains(s, " #") || strings.HasSuffix(s, ":") {
		return false
	}
	first, _ := utf8.DecodeRuneInString(s)
	if strings.ContainsRune("-?:,[]{}#&*!|>'\"%@`", first) {
		return false
	}
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	var decoded map[string]interface{}
	if err := yaml.Unmarshal([]byte("v: "+s), &decoded); err != nil {
		return false
	}
	value, ok := decoded["v"].(string)
	return ok && value == s
}

func doubleQuoted(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\r':
			b.WriteString(`\r`)
		default:
			if !unicode.IsPrint(r) {
				fmt.Fprintf(&b, `\u%04X`, r)
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func spaces(n int) string { return strings.Repeat(" ", n) }

// verifyRendered confirms that rendered text decodes to exactly the values of
// the node tree it was produced from.
func verifyRendered(root *yaml.Node, rendered []byte) error {
	var want, got interface{}
	if err := root.Decode(&want); err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(rendered))
	if err := decoder.Decode(&got); err != nil {
		return fmt.Errorf("canonical output does not parse: %w", err)
	}
	if !reflect.DeepEqual(want, got) {
		return fmt.Errorf("canonical output changes a value; nothing was written")
	}
	return nil
}
