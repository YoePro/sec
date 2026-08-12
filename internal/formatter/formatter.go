// Package formatter provides the canonical, reusable Sec source formatter.
package formatter

import (
	"strings"
	"unicode"
)

type Options struct{ Fix bool }
type Source struct{ Text string }
type Result struct{ Text string }

func Format(source Source, options Options) Result { return Result{Text: format(source.Text)} }

type branch struct {
	depth         int
	active, extra bool
}

func format(text string) string {
	text = strings.TrimPrefix(text, "\uFEFF")
	eol := "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	normal := strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(text)
	lines := strings.Split(normal, "\n")
	hadFinal := strings.HasSuffix(normal, "\n")
	if hadFinal {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, len(lines))
	indent := 0
	blank := false
	branches := []branch{}
	for _, line := range lines {
		line = strings.ReplaceAll(line, "\t", "    ")
		line = strings.TrimRight(line, " \t")
		line = strings.TrimSpace(line)
		if line == "" {
			if len(out) > 0 {
				blank = true
			}
			continue
		}
		if strings.HasPrefix(line, "@noCopy ") {
			if blank && len(out) > 0 {
				out = append(out, "")
				blank = false
			}
			out = append(out, strings.Repeat(" ", indent*4)+"@noCopy")
			line = strings.TrimSpace(strings.TrimPrefix(line, "@noCopy"))
		}
		line = formatLet(formatSignature(normalizeFunc(line)))
		level := indent - closing(line)
		if level < 0 {
			level = 0
		}
		for len(branches) > 0 && level < branches[len(branches)-1].depth {
			branches = branches[:len(branches)-1]
		}
		at := -1
		if branchClause(line) {
			for i := len(branches) - 1; i >= 0; i-- {
				if branches[i].depth == level {
					at = i
					break
				}
			}
		}
		extra := 0
		for i, b := range branches {
			if b.extra && b.active && level >= b.depth && i != at {
				extra++
			}
		}
		if blank && len(out) > 0 {
			out = append(out, "")
			blank = false
		}
		out = append(out, strings.Repeat(" ", (level+extra)*4)+line)
		delta := delimiters(line)
		indent += delta
		if indent < 0 {
			indent = 0
		}
		if branchStart(line) && delta > 0 {
			branches = append(branches, branch{depth: indent, extra: switchStart(line)})
		}
		if at >= 0 {
			branches[at].active = true
		}
	}
	result := strings.Join(out, "\n")
	if hadFinal || result != "" {
		result += "\n"
	}
	if eol != "\n" {
		result = strings.ReplaceAll(result, "\n", eol)
	}
	return result
}

func normalizeFunc(line string) string {
	if !strings.HasPrefix(line, "func ") {
		return line
	}
	open := strings.Index(line, "(")
	if open < 0 || !ident(strings.TrimSpace(line[5:open])) {
		return line
	}
	close := matchingParen(line, open)
	if close < 0 || !strings.Contains(line[close+1:], "{") {
		return line
	}
	return "fn " + line[5:]
}
func ident(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if r != '_' && !unicode.IsLetter(r) && (i == 0 || !unicode.IsDigit(r)) {
			return false
		}
	}
	return true
}
func formatSignature(line string) string {
	if !strings.HasPrefix(line, "fn ") {
		return line
	}
	open := strings.Index(line, "(")
	if open < 0 {
		return line
	}
	close := matchingParen(line, open)
	if close < 0 {
		return line
	}
	parts := split(line[open+1 : close])
	if parts == nil {
		return line
	}
	return line[:open+1] + strings.Join(parts, ", ") + line[close:]
}
func formatLet(line string) string {
	if !strings.HasPrefix(line, "let ") || strings.Contains(line, "//") || strings.Contains(line, "/*") {
		return line
	}
	parts := split(line[4:])
	if len(parts) < 2 {
		return line
	}
	for _, p := range parts {
		if !strings.Contains(p, ":=") {
			return line
		}
	}
	return "let " + strings.Join(parts, ", ")
}
func matchingParen(s string, open int) int {
	depth, angle := 0, 0
	quote := rune(0)
	esc := false
	for i, r := range s[open:] {
		if quote != 0 {
			if esc {
				esc = false
			} else if r == '\\' {
				esc = true
			} else if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		switch r {
		case '<':
			angle++
		case '>':
			if angle > 0 {
				angle--
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 && angle == 0 {
				return open + i
			}
		}
	}
	return -1
}
func split(s string) []string {
	parts := []string{}
	start, paren, bracket, angle := 0, 0, 0, 0
	quote := rune(0)
	esc := false
	for i, r := range s {
		if quote != 0 {
			if esc {
				esc = false
			} else if r == '\\' {
				esc = true
			} else if r == quote {
				quote = 0
			}
			continue
		}
		if r == '"' || r == '\'' {
			quote = r
			continue
		}
		switch r {
		case '(':
			paren++
		case ')':
			paren--
		case '[':
			bracket++
		case ']':
			bracket--
		case '<':
			angle++
		case '>':
			angle--
		case ',':
			if paren == 0 && bracket == 0 && angle == 0 {
				if p := strings.TrimSpace(s[start:i]); p != "" {
					parts = append(parts, p)
				}
				start = i + 1
			}
		}
	}
	if quote != 0 || paren != 0 || bracket != 0 || angle != 0 {
		return nil
	}
	if p := strings.TrimSpace(s[start:]); p != "" {
		parts = append(parts, p)
	}
	return parts
}
func switchStart(s string) bool {
	return strings.HasPrefix(s, "switch ") || strings.HasPrefix(s, "switch{")
}
func branchStart(s string) bool {
	return switchStart(s) || s == "select {" || strings.HasPrefix(s, "select ")
}
func branchClause(s string) bool {
	return s == "case" || strings.HasPrefix(s, "case ") || strings.HasPrefix(s, "case\t") || s == "default:" || strings.HasPrefix(s, "default ") || s == "default => {" || strings.HasPrefix(s, "after ") || strings.HasSuffix(s, "=> {")
}
func closing(s string) int {
	for _, r := range s {
		if r == ' ' || r == '\t' {
			continue
		}
		if r == '}' || r == ')' || r == ']' {
			return 1
		}
		return 0
	}
	return 0
}
func delimiters(s string) int {
	delta := 0
	quote := rune(0)
	esc := false
	lastCode := rune(0)
	for i, r := range s {
		if quote != 0 {
			if esc {
				esc = false
			} else if r == '\\' {
				esc = true
			} else if r == quote {
				quote = 0
			}
			continue
		}
		if i+1 < len(s) && r == '/' && s[i+1] == '/' {
			break
		}
		if r == '"' || r == '\'' {
			quote = r
			lastCode = r
			continue
		}
		if !unicode.IsSpace(r) {
			lastCode = r
		}
		if r == '{' || r == '(' || r == '[' {
			delta++
		}
		if r == '}' || r == ')' || r == ']' {
			delta--
		}
	}
	// A line such as `}) {` closes a multiline call/struct literal and opens
	// the attached try-handler block. The line-oriented formatter intentionally
	// collapses multiple opens on the preceding line to one visual indentation
	// level, so consuming the raw net -1 here would lose the newly opened block.
	// Keep the current level when a closing-heavy continuation hands off to a
	// trailing opening brace.
	if delta < 0 && lastCode == '{' {
		return 0
	}
	if delta < 0 {
		return -1
	}
	if delta > 0 {
		return 1
	}
	return 0
}
