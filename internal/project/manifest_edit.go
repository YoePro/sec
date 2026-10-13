package project

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// ManifestDocument retains source bytes while using a complete TOML parser to
// distinguish real tables/keys from text, comments and inline values.
// Rules: rules/projects/projects.md — §§5(2,4–6),41,49(5).
type ManifestDocument struct {
	Text     string
	Values   map[string]any
	metadata toml.MetaData
}

// ParseManifestDocument validates TOML syntax without serializing user content.
// Semantic section validation remains owned by the corresponding project model.
// Rules: rules/projects/projects.md — §§5,41(1–2),43(1).
func ParseManifestDocument(text string) (*ManifestDocument, error) {
	doc := &ManifestDocument{Text: text}
	metadata, err := toml.Decode(text, &doc.Values)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest TOML: %w", err)
	}
	doc.metadata = metadata
	return doc, nil
}

// Table returns a typed table rather than treating a scalar or array as one.
// Rules: rules/projects/projects.md — §§6,17,19,48(2).
func (doc *ManifestDocument) Table(path ...string) (map[string]any, error) {
	current := doc.Values
	for _, key := range path {
		value, found := current[key]
		if !found {
			return nil, nil
		}
		table, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s must be a TOML table", strings.Join(path, "."))
		}
		current = table
	}
	return current, nil
}

type ManifestField struct{ Name, Value string }

// AddMissingFields inserts only absent requested fields into their actual table,
// or appends a new table. All existing bytes keep their order and spelling. The
// proposal is parsed before publication, so unsupported edit forms fail safely.
// Rules: rules/projects/projects.md — §§5(4–6),40–43,49(5).
func (doc *ManifestDocument) AddMissingFields(path []string, fields []ManifestField) (string, error) {
	newline := "\n"
	if strings.Contains(doc.Text, "\r\n") {
		newline = "\r\n"
	}
	var addition strings.Builder
	for _, field := range fields {
		key := append(append([]string(nil), path...), field.Name)
		if !doc.metadata.IsDefined(key...) {
			addition.WriteString(field.Name + " = " + field.Value + newline)
		}
	}
	if addition.Len() == 0 {
		return doc.Text, nil
	}
	offset, found := manifestTableInsertion(doc.Text, path)
	var proposal string
	if found {
		prefix := doc.Text[:offset]
		if !strings.HasSuffix(prefix, "\n") {
			prefix += newline
		}
		proposal = prefix + addition.String() + doc.Text[offset:]
	} else {
		prefix := doc.Text
		if prefix != "" && !strings.HasSuffix(prefix, "\n") {
			prefix += newline
		}
		proposal = prefix + newline + "[" + manifestTablePath(path) + "]" + newline + addition.String()
	}
	if _, err := ParseManifestDocument(proposal); err != nil {
		return "", fmt.Errorf("cannot add fields to %s without changing existing syntax: %w", strings.Join(path, "."), err)
	}
	return proposal, nil
}

// manifestTableInsertion finds an explicit table header outside values. A
// synthetic header is parsed by the TOML library, so quoted/dotted keys follow
// the same syntax as the document. Multiline strings and arrays cannot forge it.
// Rules: rules/projects/projects.md — §§5(2,4–6),49(5).
func manifestTableInsertion(text string, path []string) (int, bool) {
	quote := byte(0)
	multiline := false
	depth := 0
	offset := 0
	for _, line := range strings.SplitAfter(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if quote == 0 && depth == 0 && strings.HasPrefix(trimmed, "[") {
			var values map[string]any
			metadata, err := toml.Decode(line+"\n__sec_init_marker = true\n", &values)
			if err == nil {
				keys := metadata.Keys()
				key := keys[len(keys)-1]
				if !strings.HasPrefix(trimmed, "[[") && reflect.DeepEqual([]string(key[:len(key)-1]), path) {
					return offset + len(line), true
				}
				offset += len(line)
				continue
			}
		}
		for i := 0; i < len(line); i++ {
			char := line[i]
			if quote != 0 {
				if quote == '"' && char == '\\' {
					i++
					continue
				}
				if char == quote {
					if multiline {
						count := 1
						for i+count < len(line) && line[i+count] == quote {
							count++
						}
						if count >= 3 {
							quote = 0
							multiline = false
							i += count - 1
						}
					} else {
						quote = 0
					}
				}
				continue
			}
			switch char {
			case '#':
				i = len(line)
			case '"', '\'':
				quote = char
				if i+2 < len(line) && line[i+1] == char && line[i+2] == char {
					multiline = true
					i += 2
				}
			case '[', '{':
				depth++
			case ']', '}':
				depth--
			}
		}
		offset += len(line)
	}
	return 0, false
}

// ValidProjectKey implements the machine-facing key grammar without trimming
// or case folding human-facing ProjectNames.
// Rules: rules/projects/projects.md — §6(4–6).
func ValidProjectKey(name string) bool {
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	for _, char := range name {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '_' || char == '-') {
			return false
		}
	}
	return true
}

// ManifestString emits a basic TOML string for generated ASCII metadata.
func ManifestString(value string) string { return strconv.Quote(value) }

// manifestTablePath retains key-component boundaries, including quoted names.
// Rules: rules/projects/projects.md — §5(2),§49(5).
func manifestTablePath(path []string) string {
	parts := make([]string, len(path))
	for i, key := range path {
		bare := key != ""
		for _, char := range key {
			if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_' || char == '-') {
				bare = false
			}
		}
		if bare {
			parts[i] = key
		} else {
			parts[i] = strconv.Quote(key)
		}
	}
	return strings.Join(parts, ".")
}
