package cbindings

import (
	"fmt"
	"strconv"
	"strings"
)

// tomlEntry is one top-level TOML statement located in the source bytes: a
// table header or a key/value pair. start/end cover whole lines including
// the line break; valueStart/valueEnd cover the value of a key/value pair.
type tomlEntry struct {
	header, arrayHeader  bool
	path                 []string // full key path (header path, or table + key)
	table                []string // enclosing table of a key/value pair
	start, end           int
	valueStart, valueEnd int
}

// tomlDocument is the source text plus its statement index. Edits splice
// bytes; nothing is re-serialized, so comments and formatting survive.
// Rules: rules/projects/projects.md — § 5(4)-(6); MD-021 correction § 5(3).
type tomlDocument struct {
	text    string
	entries []tomlEntry
	newline string
}

// scanDocument indexes statements, skipping strings, arrays, inline tables
// and comments so that their contents can never be mistaken for structure.
// The full TOML parser has already accepted the text.
// Rules: MD-021 correction § 5(3); rules/projects/projects.md — § 5(2).
func scanDocument(text string) (*tomlDocument, error) {
	doc := &tomlDocument{text: text, newline: "\n"}
	if strings.Contains(text, "\r\n") {
		doc.newline = "\r\n"
	}
	var table []string
	pos := 0
	for {
		pos = skipBlank(text, pos, true)
		if pos >= len(text) {
			return doc, nil
		}
		start := lineStart(text, pos)
		switch text[pos] {
		case '#':
			pos = lineEnd(text, pos)
			continue
		case '[':
			array := strings.HasPrefix(text[pos:], "[[")
			pos++
			if array {
				pos++
			}
			path, next, err := parseKey(text, pos)
			if err != nil {
				return nil, err
			}
			pos = skipBlank(text, next, false)
			closing := "]"
			if array {
				closing = "]]"
			}
			if !strings.HasPrefix(text[pos:], closing) {
				return nil, fmt.Errorf("malformed table header at byte %d", start)
			}
			pos = lineEnd(text, pos+len(closing))
			doc.entries = append(doc.entries, tomlEntry{header: true, arrayHeader: array, path: path, start: start, end: pos})
			table = path
		default:
			key, next, err := parseKey(text, pos)
			if err != nil {
				return nil, err
			}
			pos = skipBlank(text, next, false)
			if pos >= len(text) || text[pos] != '=' {
				return nil, fmt.Errorf("expected '=' at byte %d", pos)
			}
			pos = skipBlank(text, pos+1, false)
			valueStart := pos
			if pos, err = scanValue(text, pos); err != nil {
				return nil, err
			}
			valueEnd := pos
			pos = lineEnd(text, pos)
			full := append(append([]string{}, table...), key...)
			doc.entries = append(doc.entries, tomlEntry{path: full, table: table, start: start, end: pos, valueStart: valueStart, valueEnd: valueEnd})
		}
	}
}

func skipBlank(text string, pos int, newlines bool) int {
	for pos < len(text) && (text[pos] == ' ' || text[pos] == '\t' || newlines && (text[pos] == '\n' || text[pos] == '\r')) {
		pos++
	}
	return pos
}

func lineStart(text string, pos int) int { return strings.LastIndexByte(text[:pos], '\n') + 1 }

// lineEnd returns the offset after the line break of the line holding pos.
func lineEnd(text string, pos int) int {
	if index := strings.IndexByte(text[pos:], '\n'); index >= 0 {
		return pos + index + 1
	}
	return len(text)
}

// parseKey reads a dotted key of bare, basic-string or literal-string parts.
func parseKey(text string, pos int) ([]string, int, error) {
	var parts []string
	for {
		pos = skipBlank(text, pos, false)
		if pos >= len(text) {
			return nil, pos, fmt.Errorf("unexpected end of key")
		}
		switch text[pos] {
		case '"':
			end, err := scanBasicString(text, pos)
			if err != nil {
				return nil, pos, err
			}
			value, err := strconv.Unquote(text[pos:end])
			if err != nil {
				return nil, pos, fmt.Errorf("unsupported quoted key %s", text[pos:end])
			}
			parts, pos = append(parts, value), end
		case '\'':
			end := strings.IndexByte(text[pos+1:], '\'')
			if end < 0 {
				return nil, pos, fmt.Errorf("unterminated literal key")
			}
			parts, pos = append(parts, text[pos+1:pos+1+end]), pos+end+2
		default:
			end := pos
			for end < len(text) && isBareKeyChar(text[end]) {
				end++
			}
			if end == pos {
				return nil, pos, fmt.Errorf("invalid key at byte %d", pos)
			}
			parts, pos = append(parts, text[pos:end]), end
		}
		next := skipBlank(text, pos, false)
		if next >= len(text) || text[next] != '.' {
			return parts, pos, nil
		}
		pos = next + 1
	}
}

func isBareKeyChar(char byte) bool {
	return char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-'
}

// scanBasicString returns the offset after a single-line basic string.
func scanBasicString(text string, pos int) (int, error) {
	for index := pos + 1; index < len(text); index++ {
		switch text[index] {
		case '\\':
			index++
		case '"':
			return index + 1, nil
		case '\n':
			return 0, fmt.Errorf("unterminated string at byte %d", pos)
		}
	}
	return 0, fmt.Errorf("unterminated string at byte %d", pos)
}

// scanValue returns the offset after one TOML value.
func scanValue(text string, pos int) (int, error) {
	if pos >= len(text) {
		return pos, fmt.Errorf("missing value")
	}
	switch {
	case strings.HasPrefix(text[pos:], `"""`), strings.HasPrefix(text[pos:], `'''`):
		delimiter := text[pos : pos+3]
		for index := pos + 3; index < len(text); index++ {
			if delimiter == `"""` && text[index] == '\\' {
				index++
				continue
			}
			if strings.HasPrefix(text[index:], delimiter) {
				end := index + 3
				for extra := 0; extra < 2 && end < len(text) && text[end] == delimiter[0]; extra++ {
					end++
				}
				return end, nil
			}
		}
		return pos, fmt.Errorf("unterminated multi-line string")
	case text[pos] == '"':
		return scanBasicString(text, pos)
	case text[pos] == '\'':
		end := strings.IndexByte(text[pos+1:], '\'')
		if end < 0 {
			return pos, fmt.Errorf("unterminated literal string")
		}
		return pos + end + 2, nil
	case text[pos] == '[' || text[pos] == '{':
		depth := 0
		for index := pos; index < len(text); index++ {
			switch text[index] {
			case '#':
				index = lineEnd(text, index) - 1
			case '"', '\'':
				end, err := scanValue(text, index)
				if err != nil {
					return pos, err
				}
				index = end - 1
			case '[', '{':
				depth++
			case ']', '}':
				depth--
				if depth == 0 {
					return index + 1, nil
				}
			}
		}
		return pos, fmt.Errorf("unterminated array or inline table")
	}
	end := pos
	for end < len(text) && !strings.ContainsRune(" \t\r\n#", rune(text[end])) {
		end++
	}
	return end, nil
}

func samePath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func (doc *tomlDocument) find(path []string) (tomlEntry, bool) {
	for _, entry := range doc.entries {
		if !entry.header && samePath(entry.path, path) {
			return entry, true
		}
	}
	return tomlEntry{}, false
}

func (doc *tomlDocument) header(path []string) (int, bool) {
	for index, entry := range doc.entries {
		if entry.header && !entry.arrayHeader && samePath(entry.path, path) {
			return index, true
		}
	}
	return 0, false
}

// setValue replaces a key's value in place, or inserts the key after the
// last key of its explicit table (creating the table at the end if absent).
// Rules: MD-021 correction § 5(3).
func (doc *tomlDocument) setValue(table []string, key, value string) (string, error) {
	path := append(append([]string{}, table...), key)
	if entry, found := doc.find(path); found {
		return doc.text[:entry.valueStart] + value + doc.text[entry.valueEnd:], nil
	}
	return doc.insertKey(table, key+" = "+value)
}

// insertKey adds one line to an explicit table, creating the table when
// absent; a [modules] table is placed before the first environment.
func (doc *tomlDocument) insertKey(table []string, line string) (string, error) {
	index, found := doc.header(table)
	if !found {
		if _, defined := doc.findPrefix(table); defined {
			return "", fmt.Errorf("%s is not written as an explicit [table]", strings.Join(table, "."))
		}
		if samePath(table, []string{"modules"}) {
			for _, entry := range doc.entries {
				if entry.header && len(entry.path) > 0 && entry.path[0] == "environments" {
					block := "[modules]" + doc.newline + line + doc.newline + doc.newline
					offset := attachedCommentStart(doc.text, entry.start)
					return doc.text[:offset] + block + doc.text[offset:], nil
				}
			}
		}
		return doc.appendTable(table, []string{line}), nil
	}
	offset := doc.entries[index].end
	for _, entry := range doc.entries[index+1:] {
		if entry.header {
			break
		}
		offset = entry.end
	}
	prefix := doc.text[:offset]
	if !strings.HasSuffix(prefix, "\n") {
		prefix += doc.newline
	}
	return prefix + line + doc.newline + doc.text[offset:], nil
}

// findPrefix reports whether any key lives below path through another form
// (dotted keys or an inline table), which a table edit must not duplicate.
func (doc *tomlDocument) findPrefix(path []string) (tomlEntry, bool) {
	for _, entry := range doc.entries {
		if len(entry.path) >= len(path) && samePath(entry.path[:len(path)], path) {
			return entry, true
		}
		if !entry.header && len(entry.path) < len(path) && samePath(entry.path, path[:len(entry.path)]) {
			return entry, true
		}
	}
	return tomlEntry{}, false
}

// appendTable adds a new explicit table, separated by one blank line.
func (doc *tomlDocument) appendTable(table []string, lines []string) string {
	prefix := strings.TrimRight(doc.text, " \t\r\n")
	var out strings.Builder
	out.WriteString(prefix)
	if prefix != "" {
		out.WriteString(doc.newline + doc.newline)
	}
	out.WriteString("[" + strings.Join(table, ".") + "]" + doc.newline)
	for _, line := range lines {
		out.WriteString(line + doc.newline)
	}
	return out.String()
}

// removeKey deletes one key/value line (with its trailing comment).
func (doc *tomlDocument) removeKey(table []string, key string) (string, error) {
	entry, found := doc.find(append(append([]string{}, table...), key))
	if !found {
		return doc.text, nil
	}
	return doc.text[:entry.start] + doc.text[entry.end:], nil
}

// attachedCommentStart moves a line offset up over the comment lines that
// directly precede it (no blank line between), which document that table.
func attachedCommentStart(text string, offset int) int {
	for offset > 0 {
		previous := lineStart(text, offset-1)
		if !strings.HasPrefix(strings.TrimSpace(text[previous:offset]), "#") {
			break
		}
		offset = previous
	}
	return offset
}

// removeTable deletes an explicit table, its keys, its attached leading
// comments and the blank lines up to the next table's own comment block.
// Rules: MD-021 correction §§ 4(4), 5(3).
func (doc *tomlDocument) removeTable(table []string) (string, error) {
	index, found := doc.header(table)
	if !found {
		return "", fmt.Errorf("%s is not written as an explicit [table]", strings.Join(table, "."))
	}
	start := attachedCommentStart(doc.text, doc.entries[index].start)
	end := len(doc.text)
	for _, entry := range doc.entries[index+1:] {
		if entry.header {
			if len(entry.path) > len(table) && samePath(entry.path[:len(table)], table) {
				return "", fmt.Errorf("%s has sub-tables", strings.Join(table, "."))
			}
			end = attachedCommentStart(doc.text, entry.start)
			break
		}
	}
	if end == len(doc.text) {
		prefix := strings.TrimRight(doc.text[:start], " \t\r\n")
		if prefix == "" {
			return "", nil
		}
		return prefix + doc.newline, nil
	}
	return doc.text[:start] + doc.text[end:], nil
}

// arrayElement is one string element of an array value.
type arrayElement struct {
	start, end int
	comma      int // offset of the following comma, or -1
	value      string
}

// scanStringArray lists the string elements of an array value.
func (doc *tomlDocument) scanStringArray(entry tomlEntry) ([]arrayElement, int, error) {
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

// addArrayString appends a string to an array value in the style of the
// existing list (single line, or one element per line with the same indent
// and trailing-comma convention); a missing key is inserted.
// Rules: MD-021 correction §§ 4(4), 5(3).
func (doc *tomlDocument) addArrayString(table []string, key, value string) (string, error) {
	entry, found := doc.find(append(append([]string{}, table...), key))
	quoted := tomlString(value)
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

// removeArrayString removes one exact string element. An element alone on
// its line is removed with that line (and its comment); the list keeps its
// trailing-comma convention.
// Rules: MD-021 correction §§ 4(4), 5(3)-(4).
func (doc *tomlDocument) removeArrayString(table []string, key, value string) (string, error) {
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
