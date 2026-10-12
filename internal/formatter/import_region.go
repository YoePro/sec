package formatter

import (
	"sort"
	"strings"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// importEntry is one parser-confirmed import together with the comments that
// the parser attached to it, so reordering moves them with the import.
type importEntry struct {
	alias    string
	path     string
	leading  []string
	trailing string
	order    int
}

// importDeclaration is one top-level single import or import group and the
// complete physical line range it owns, including attached leading comments.
type importDeclaration struct {
	firstLine int
	lastLine  int
	entries   []*importEntry
}

// formatImportRegion gathers every top-level import into one canonical import
// region after the module declaration: one import stays in single-import
// form, two or more form one import group, and imports are sorted
// alphabetically by logical import path (never by alias, never by a guessed
// standard-library, project or platform origin), with parser-attached
// leading and trailing comments moving with their import. Explicit
// blank-line-separated groups of one import group, with or without heading
// comments, are kept and sorted internally (headedImportGroup). No import is
// ever removed. Any source the pass cannot attribute exactly, such as a parse
// error, a detached comment inside a group, or several imports on one line,
// is left unchanged.
//
// Rules:
//   - rules/tooling/formatter.md — § 13 "Imports", § 14(1)–(5) "Top-level file layout"
//   - rules/foundations/grammar.md — "Import declarations"
//   - rules/corrections/applied/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md — § 3
func formatImportRegion(text string, compact bool) string {
	if !strings.Contains(text, "import") {
		return text
	}
	p := parser.New(lexer.New(text))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 || program == nil {
		return text
	}
	lines := strings.Split(text, "\n")
	tokens := lexImportRegionTokens(text)
	tokensByLine := map[int][]lexer.Token{}
	for _, token := range tokens {
		tokensByLine[token.Line] = append(tokensByLine[token.Line], token)
	}

	moduleLine := 0
	firstDeclarationLine := 0
	declarations := []*importDeclaration{}
	byImportToken := map[int]*importDeclaration{}
	entryByAnchor := map[int]*importEntry{}
	entryByLine := map[int]*importEntry{}
	ownedAnchors := map[int]bool{}
	order := 0
	for _, statement := range program.Statements {
		switch statement := statement.(type) {
		case *ast.ModuleStatement:
			if moduleLine != 0 || len(declarations) > 0 {
				return text
			}
			moduleLine = statement.Token.Line
		case *ast.ImportStatement:
			if moduleLine == 0 {
				return text
			}
			declaration, grouped := byImportToken[statement.Token.ByteStart]
			if !grouped {
				declaration = &importDeclaration{firstLine: statement.Token.Line}
				byImportToken[statement.Token.ByteStart] = declaration
				declarations = append(declarations, declaration)
				ownedAnchors[statement.Token.ByteStart] = true
			}
			first := statement.PathToken
			if statement.Alias != "" {
				if statement.AliasToken.Line != statement.PathToken.Line {
					return text
				}
				first = statement.AliasToken
			}
			entry := &importEntry{alias: statement.Alias, path: statement.Path, order: order}
			order++
			declaration.entries = append(declaration.entries, entry)
			entryByAnchor[first.ByteStart] = entry
			ownedAnchors[first.ByteStart] = true
			ownedAnchors[statement.PathToken.ByteStart] = true
			entryByAnchor[-1-statement.PathToken.ByteStart] = entry
			entryByLine[statement.PathToken.Line] = entry
		}
	}
	if len(declarations) == 0 {
		return text
	}

	// Establish each declaration's exact line span and require that its lines
	// hold nothing but its own tokens.
	for _, declaration := range declarations {
		importToken, ok := tokenStartingAt(tokensByLine, declaration.firstLine, "import")
		if !ok {
			return text
		}
		lineTokens := tokensByLine[declaration.firstLine]
		if len(lineTokens) >= 2 && lineTokens[1].Type == lexer.LPAREN {
			if len(lineTokens) != 2 || lineTokens[0].ByteStart != importToken.ByteStart {
				return text
			}
			closeLine := 0
			for line := declaration.firstLine + 1; line <= len(lines); line++ {
				candidates := tokensByLine[line]
				if len(candidates) == 1 && candidates[0].Type == lexer.RPAREN {
					closeLine = line
					break
				}
				if len(candidates) == 0 {
					continue
				}
				if !importItemLine(candidates) {
					return text
				}
			}
			if closeLine == 0 {
				return text
			}
			ownedAnchors[lineTokens[1].ByteStart] = true
			ownedAnchors[tokensByLine[closeLine][0].ByteStart] = true
			declaration.lastLine = closeLine
		} else {
			if len(declaration.entries) != 1 || !importItemLine(lineTokens[1:]) || lineTokens[0].ByteStart != importToken.ByteStart {
				return text
			}
			declaration.lastLine = declaration.firstLine
		}
	}

	// The first real token after the module line that no import owns starts
	// the first ordinary declaration, including any attribute line.
	for _, token := range tokens {
		if token.Line <= moduleLine || token.Line >= declarations[0].firstLine && commentInsideImportDeclaration(declarations, token.Line, token.Line) {
			continue
		}
		firstDeclarationLine = token.Line
		break
	}

	// MD-055: a group whose blank-line-separated runs carry heading comments
	// keeps its runs; the comments inside it are attributed by its lines.
	headed, headedOK := headedImportGroup(lines, tokensByLine, declarations, entryByLine)
	if !headedOK {
		return text
	}

	// Attach parser-owned comment groups; a comment the pass cannot attribute
	// to exactly one import keeps the whole region unchanged.
	for _, attachment := range program.Comments {
		if len(attachment.Comments) == 0 {
			continue
		}
		first := attachment.Comments[0]
		last := attachment.Comments[len(attachment.Comments)-1]
		lastLine := tokenLastLine(last)
		inside := commentInsideImportDeclaration(declarations, first.Line, lastLine)
		if headed != nil && inside {
			continue
		}
		switch attachment.Placement {
		case ast.CommentTrailing:
			if entry, ok := entryByAnchor[-1-attachment.Anchor.ByteStart]; ok && strings.HasPrefix(first.Lexeme, "//") {
				entry.trailing = first.Lexeme
				continue
			}
			if ownedAnchors[attachment.Anchor.ByteStart] || inside {
				return text
			}
		case ast.CommentLeading, ast.CommentDocumentation:
			if entry, ok := entryByAnchor[attachment.Anchor.ByteStart]; ok {
				entry.leading = append(entry.leading, lines[first.Line-1:lastLine]...)
				continue
			}
			if declaration, ok := byImportToken[attachment.Anchor.ByteStart]; ok && len(declaration.entries) == 1 && attachment.Anchor.Line == declaration.firstLine && tokensByLine[declaration.firstLine][0].Type == lexer.IMPORT && !declarationIsGroup(tokensByLine, declaration) {
				declaration.entries[0].leading = append(declaration.entries[0].leading, lines[first.Line-1:lastLine]...)
				if first.Line < declaration.firstLine {
					declaration.firstLine = first.Line
				}
				continue
			}
			if ownedAnchors[attachment.Anchor.ByteStart] || inside {
				return text
			}
		default:
			if inside || ownedAnchors[attachment.Anchor.ByteStart] {
				return text
			}
		}
	}

	entries := []*importEntry{}
	removed := map[int]bool{}
	for _, declaration := range declarations {
		entries = append(entries, declaration.entries...)
		for line := declaration.firstLine; line <= declaration.lastLine; line++ {
			removed[line] = true
		}
	}
	sortImportEntries(entries)
	region := renderImportRegion(entries)
	if headed != nil {
		region = renderHeadedImportGroup(headed)
	}

	insertBefore := declarations[0].firstLine
	if firstDeclarationLine != 0 && firstDeclarationLine < insertBefore {
		insertBefore = leadingCommentStart(program.Comments, firstDeclarationLine)
	}
	out := make([]string, 0, len(lines)+2)
	for index, line := range lines {
		number := index + 1
		if number == insertBefore {
			out = append(out, "")
			out = append(out, region...)
			out = append(out, "")
		}
		if removed[number] {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// importRun is one blank-line-separated run of a grouped import declaration
// with its optional group heading.
type importRun struct {
	heading []string
	entries []*importEntry
}

// headedImportGroup recognizes a single grouped import declaration whose
// imports form explicit blank-line-separated groups (MD-033), optionally
// introduced by standalone `//` heading comments (MD-055). A heading is the
// comment block that starts a run, directly above or separated by blank
// lines from the run's first import; a comment between two imports of a run
// leads the next import, and an end-of-line `//` comment trails its import.
// It returns nil, true for a group of one run without a heading (the
// ordinary region pass applies) and false when a comment's owner or the
// group layout is ambiguous: a comment that ends a run or the group, a block
// comment, or explicit groups combined with further import declarations.
// Such a region is left unchanged.
//
// Rules:
//   - rules/tooling/formatter.md — § 13 "Imports"
//   - rules/corrections/applied/md028-md032-md055-try-switch-import-comments-correction-20261012.md — § 4
//   - rules/corrections/applied/md027-md033-md036-md054-contextual-literals-formatter-configuration-correction-20261012.md — § 3
func headedImportGroup(lines []string, tokensByLine map[int][]lexer.Token, declarations []*importDeclaration, entryByLine map[int]*importEntry) ([]importRun, bool) {
	hasHeading := false
	var runs []importRun
	leading := map[*importEntry][]string{}
	trailing := map[*importEntry]string{}
	for _, declaration := range declarations {
		if !declarationIsGroup(tokensByLine, declaration) {
			continue
		}
		current := importRun{}
		var pending []string
		blankSinceItem := true
		flush := func() bool {
			if len(pending) > 0 {
				return false
			}
			if len(current.entries) > 0 {
				runs = append(runs, current)
			} else if len(current.heading) > 0 {
				return false
			}
			current = importRun{}
			blankSinceItem = true
			return true
		}
		for line := declaration.firstLine + 1; line < declaration.lastLine; line++ {
			trimmed := strings.TrimSpace(lines[line-1])
			switch {
			case trimmed == "":
				if len(current.entries) > 0 {
					if !flush() {
						return nil, false
					}
				}
			case strings.HasPrefix(trimmed, "//"):
				if len(current.entries) == 0 && blankSinceItem {
					current.heading = append(current.heading, trimmed)
					if line > declaration.firstLine+1 && strings.TrimSpace(lines[line-2]) == "" && len(current.heading) > 1 {
						return nil, false
					}
					continue
				}
				pending = append(pending, trimmed)
			case strings.Contains(trimmed, "/*"):
				if len(current.heading) > 0 || hasHeading {
					return nil, false
				}
				return nil, true
			default:
				entry, ok := entryByLine[line]
				if !ok {
					return nil, true
				}
				leading[entry] = pending
				pending = nil
				if tokens := tokensByLine[line]; len(tokens) > 0 {
					end := tokens[len(tokens)-1].ByteEnd - tokenLineStart(lines, line)
					if end >= 0 && end <= len(lines[line-1]) {
						if rest := strings.TrimSpace(lines[line-1][end:]); rest != "" {
							if !strings.HasPrefix(rest, "//") {
								return nil, false
							}
							trailing[entry] = rest
						}
					}
				}
				current.entries = append(current.entries, entry)
				blankSinceItem = false
			}
			if len(current.heading) > 0 {
				hasHeading = true
			}
		}
		if !flush() {
			return nil, false
		}
	}
	if !hasHeading && len(runs) < 2 {
		return nil, true
	}
	if len(declarations) != 1 {
		return nil, false
	}
	for _, run := range runs {
		for _, entry := range run.entries {
			entry.leading, entry.trailing = leading[entry], trailing[entry]
		}
	}
	return runs, true
}

// renderHeadedImportGroup keeps every run and its heading, sorts imports
// within each run only, places each heading directly above its run's first
// import and separates runs with one blank line.
func renderHeadedImportGroup(runs []importRun) []string {
	region := []string{"import ("}
	for index, run := range runs {
		if index > 0 {
			region = append(region, "")
		}
		for _, heading := range run.heading {
			region = append(region, "    "+heading)
		}
		entries := append([]*importEntry(nil), run.entries...)
		sortImportEntries(entries)
		for _, entry := range entries {
			for _, comment := range entry.leading {
				region = append(region, "    "+comment)
			}
			spelled := "\"" + entry.path + "\""
			if entry.alias != "" {
				spelled = entry.alias + " " + spelled
			}
			if entry.trailing != "" {
				spelled += " " + entry.trailing
			}
			region = append(region, "    "+spelled)
		}
	}
	return append(region, ")")
}

// tokenLineStart returns the byte offset at which line starts.
func tokenLineStart(lines []string, line int) int {
	offset := 0
	for index := 0; index < line-1; index++ {
		offset += len(lines[index]) + 1
	}
	return offset
}

// sortImportEntries orders imports alphabetically by logical import path,
// then alias, then source order; it never classifies a path (MD-033).
func sortImportEntries(entries []*importEntry) {
	sort.SliceStable(entries, func(left, right int) bool {
		a, b := entries[left], entries[right]
		if a.path != b.path {
			return a.path < b.path
		}
		if a.alias != b.alias {
			return a.alias < b.alias
		}
		return a.order < b.order
	})
}

// renderImportRegion emits the single-import form for one import and one
// import group of a single alphabetic sequence otherwise.
func renderImportRegion(entries []*importEntry) []string {
	item := func(entry *importEntry) string {
		spelled := "\"" + entry.path + "\""
		if entry.alias != "" {
			spelled = entry.alias + " " + spelled
		}
		if entry.trailing != "" {
			spelled += " " + entry.trailing
		}
		return spelled
	}
	if len(entries) == 1 {
		region := append([]string(nil), trimmedLines(entries[0].leading)...)
		return append(region, "import "+item(entries[0]))
	}
	region := []string{"import ("}
	for _, entry := range entries {
		for _, comment := range trimmedLines(entry.leading) {
			region = append(region, "    "+comment)
		}
		region = append(region, "    "+item(entry))
	}
	return append(region, ")")
}

func trimmedLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, strings.TrimSpace(line))
	}
	return out
}

// importItemLine reports whether one physical line holds exactly one import
// item: an optional alias followed by its path.
func importItemLine(tokens []lexer.Token) bool {
	switch len(tokens) {
	case 1:
		return tokens[0].Type == lexer.STRING
	case 2:
		return tokens[0].Type == lexer.IDENT && tokens[1].Type == lexer.STRING
	default:
		return false
	}
}

func declarationIsGroup(tokensByLine map[int][]lexer.Token, declaration *importDeclaration) bool {
	tokens := tokensByLine[declaration.firstLine]
	return len(tokens) >= 2 && tokens[1].Type == lexer.LPAREN
}

func commentInsideImportDeclaration(declarations []*importDeclaration, first, last int) bool {
	for _, declaration := range declarations {
		if last >= declaration.firstLine && first <= declaration.lastLine {
			return true
		}
	}
	return false
}

func tokenStartingAt(tokensByLine map[int][]lexer.Token, line int, lexeme string) (lexer.Token, bool) {
	tokens := tokensByLine[line]
	if len(tokens) == 0 || tokens[0].Lexeme != lexeme {
		return lexer.Token{}, false
	}
	return tokens[0], true
}

func tokenLastLine(token lexer.Token) int {
	if token.EndLine >= token.Line {
		return token.EndLine
	}
	return token.Line
}

// leadingCommentStart returns the first line of a comment group the parser
// attached as leading to the declaration starting on line, so the import
// region is inserted above that declaration's comments rather than between.
func leadingCommentStart(comments []ast.CommentAttachment, line int) int {
	for _, attachment := range comments {
		if (attachment.Placement == ast.CommentLeading || attachment.Placement == ast.CommentDocumentation) && attachment.Anchor.Line == line && len(attachment.Comments) > 0 {
			return attachment.Comments[0].Line
		}
	}
	return line
}

// lexImportRegionTokens returns the real (non-comment) tokens of text.
func lexImportRegionTokens(text string) []lexer.Token {
	source := lexer.New(text)
	tokens := []lexer.Token{}
	for {
		token := source.NextToken()
		if token.Type == lexer.EOF {
			return tokens
		}
		if token.Type != lexer.COMMENT {
			tokens = append(tokens, token)
		}
	}
}
