package configedit

import (
	"fmt"
	"strconv"
	"strings"
)

type arrayElement struct {
	start, end int
	comma      int // offset of the following comma, or -1
	value      string
}

// scanStringArray lists the string elements of an array value.
func (doc *Document) scanStringArray(entry tomlEntry) ([]arrayElement, int, error) {
	text := doc.text
	if text[entry.valueStart] != '[' {
		return nil, 0, fmt.Errorf("value is not an array")
	}
	var elements []arrayElement
	for index := entry.valueStart + 1; index < entry.valueEnd; index++ {
		switch char := text[index]; {
		case char == '#':
			index = lineEnd(text, index) - 1
		case char == ',':
			if len(elements) == 0 || elements[len(elements)-1].comma >= 0 {
				return nil, 0, fmt.Errorf("malformed array")
			}
			elements[len(elements)-1].comma = index
		case char == '"' || char == '\'':
			end, err := scanValue(text, index)
			if err != nil {
				return nil, 0, err
			}
			raw := text[index:end]
			value := raw[1 : len(raw)-1]
			if char == '"' {
				if value, err = strconv.Unquote(raw); err != nil {
					return nil, 0, fmt.Errorf("unsupported string %s", raw)
				}
			}
			elements = append(elements, arrayElement{start: index, end: end, comma: -1, value: value})
			index = end - 1
		case char == ']':
			return elements, index, nil
		case char == ' ' || char == '\t' || char == '\r' || char == '\n':
		default:
			return nil, 0, fmt.Errorf("array holds a non-string value")
		}
	}
	return nil, 0, fmt.Errorf("unterminated array")
}

// AddArrayString appends a string to an array value in the style of the
// existing list (single line, or one element per line with the same indent
// and trailing-comma convention); a missing key is inserted.
// Rules: MD-021 correction §§ 4(4), 5(3).
func (doc *Document) AddArrayString(table []string, key, value string) (string, error) {
	entry, found := doc.find(append(append([]string{}, table...), key))
	quoted := String(value)
	if !found {
		return doc.insertKey(table, key+" = ["+quoted+"]")
	}
	elements, closing, err := doc.scanStringArray(entry)
	if err != nil {
		return "", err
	}
	text := doc.text
	multiline := strings.Contains(text[entry.valueStart:entry.valueEnd], "\n")
	if len(elements) == 0 {
		if strings.TrimSpace(text[entry.valueStart+1:closing]) == "" {
			return text[:entry.valueStart] + "[" + quoted + "]" + text[entry.valueEnd:], nil
		}
		at := lineStart(text, closing)
		return text[:at] + "    " + quoted + "," + doc.newline + text[at:], nil
	}
	last := elements[len(elements)-1]
	if !multiline || lineStart(text, last.start) == lineStart(text, closing) {
		if multiline {
			indent := leadingIndent(text, last.start)
			return text[:last.end] + "," + doc.newline + indent + quoted + text[last.end:], nil
		}
		return text[:last.end] + ", " + quoted + text[last.end:], nil
	}
	indent := leadingIndent(text, last.start)
	after := lineEnd(text, last.end)
	line := indent + quoted
	if last.comma >= 0 {
		line += ","
	}
	edited := text[:after] + line + doc.newline + text[after:]
	if last.comma < 0 {
		edited = edited[:last.end] + "," + edited[last.end:]
	}
	return edited, nil
}

func leadingIndent(text string, pos int) string {
	start := lineStart(text, pos)
	end := start
	for end < pos && (text[end] == ' ' || text[end] == '\t') {
		end++
	}
	return text[start:end]
}

// RemoveArrayString removes one exact string element. An element alone on
// its line is removed with that line (and its comment); the list keeps its
// trailing-comma convention.
// Rules: MD-021 correction §§ 4(4), 5(3)-(4).
func (doc *Document) RemoveArrayString(table []string, key, value string) (string, error) {
	entry, found := doc.find(append(append([]string{}, table...), key))
	if !found {
		return doc.text, nil
	}
	elements, closing, err := doc.scanStringArray(entry)
	if err != nil {
		return "", err
	}
	target := -1
	for index, element := range elements {
		if element.value == value {
			target = index
			break
		}
	}
	if target < 0 {
		return doc.text, nil
	}
	text := doc.text
	element := elements[target]
	isLast := target == len(elements)-1
	start, end := lineStart(text, element.start), lineEnd(text, element.end)
	rest := element.end
	if element.comma >= 0 {
		rest = element.comma + 1
	}
	alone := strings.TrimSpace(text[start:element.start]) == "" &&
		(strings.TrimSpace(text[rest:end]) == "" || strings.HasPrefix(strings.TrimSpace(text[rest:end]), "#")) &&
		closing >= end
	if alone {
		edited := text[:start] + text[end:]
		if isLast && element.comma < 0 && target > 0 && elements[target-1].comma >= 0 {
			comma := elements[target-1].comma
			edited = edited[:comma] + edited[comma+1:]
		}
		return edited, nil
	}
	switch {
	case !isLast:
		return text[:element.start] + text[elements[target+1].start:], nil
	case target > 0:
		previous := elements[target-1]
		return text[:previous.end] + text[rest:], nil
	}
	return text[:element.start] + text[rest:], nil
}
