package sourceimports

import "sec/internal/lexer"

type ModuleConflict struct {
	Expected string
	Actual   lexer.Token
}

// ModuleConflicts checks the directory module header before body qualification.
// This detects inconsistent namespaces even when a source body is incomplete.
// Selected paths and overlay reading are supplied by the owning frontend.
// Rules: rules/projects/modules.md — §§3,9,21.
func ModuleConflicts(paths []string, read func(string) ([]byte, error)) []ModuleConflict {
	expected := ""
	var result []ModuleConflict
	for _, path := range paths {
		data, err := read(path)
		if err != nil {
			continue
		} // The source reader owns I/O diagnostics.
		l := lexer.NewWithFile(string(data), path)
		for token := l.NextToken(); token.Type != lexer.EOF; token = l.NextToken() {
			if token.Type != lexer.MODULE {
				continue
			}
			name := l.NextToken()
			if name.Type != lexer.IDENT {
				break
			}
			if expected == "" {
				expected = name.Lexeme
			}
			if name.Lexeme != expected {
				result = append(result, ModuleConflict{Expected: expected, Actual: name})
			}
			break
		}
	}
	return result
}
