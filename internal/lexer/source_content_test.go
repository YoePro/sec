package lexer

import "testing"

// TestCompiledSourceContent binds metadata to lexer input rather than a host
// path or a decoded/normalized token stream. Rewind and consumers cannot mutate
// it, and two editor versions under the same URI remain distinguishable.
// Rules: rules/compiler/debug_information.md — §§54(2),55(1)-(5).
func TestCompiledSourceContent(t *testing.T) {
	source := "\xef\xbb\xbfmodule main\r\n// editor overlay\r\n"
	l := NewWithFile(source, "/first/root/main.sec")
	before := l.SourceContent()
	if before.File != "/first/root/main.sec" {
		t.Fatal(before)
	}
	if match, err := before.Identity.Matches([]byte(source)); err != nil || !match {
		t.Fatal(match, err)
	}
	other := NewWithFile(source, "/relocated/root/main.sec")
	if other.SourceContent().Identity != before.Identity {
		t.Fatal("host path changed content identity")
	}
	changed := NewWithFile(source+"// unsaved edit", before.File)
	if changed.SourceContent().Identity == before.Identity {
		t.Fatal("overlay edit reused old identity")
	}
	state := l.Snapshot()
	first := l.NextToken()
	l.Restore(state)
	if replay := l.NextToken(); replay != first || l.SourceContent() != before {
		t.Fatal("rewind changed token or content provenance", first, replay)
	}
	for token := l.NextToken(); token.Type != EOF; token = l.NextToken() {
	}
	if l.SourceContent() != before {
		t.Fatal("lexing changed provenance")
	}
	before.File = "remapped"
	if l.SourceContent().File == before.File {
		t.Fatal("consumer mutated lexer provenance")
	}
}
