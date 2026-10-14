package debuginfo

import (
	"encoding/json"
	"testing"
)

// TestSourceContentMismatch catches provenance-only and byte-level edits without
// embedded source, including empty files and canonically equivalent Unicode.
// Rules: rules/compiler/debug_information.md — §55;
// rules/compiler/incremental_compilation.md — §59(1).
func TestSourceContentMismatch(t *testing.T) {
	for _, source := range []string{"", "module main\n// compiled\n", "å", "\xef\xbb\xbfmodule main\r\n"} {
		identity := IdentifySourceContent([]byte(source))
		data, err := json.Marshal(identity)
		if err != nil {
			t.Fatal(err)
		}
		var decoded SourceContentIdentity
		if err := json.Unmarshal(data, &decoded); err != nil || decoded != identity {
			t.Fatal(decoded, err)
		}
		if matches, err := decoded.Matches([]byte(source)); err != nil || !matches {
			t.Fatal(matches, err)
		}
		if matches, err := decoded.Matches([]byte(source + " ")); err != nil || matches {
			t.Fatal(matches, err)
		}
	}
	for _, pair := range [][2]string{{"// old", "// new"}, {"a\n", "a\r\n"}, {"a", "\xef\xbb\xbfa"}, {"å", "a\u030a"}} {
		if matches, err := IdentifySourceContent([]byte(pair[0])).Matches([]byte(pair[1])); err != nil || matches {
			t.Fatal(pair, matches, err)
		}
	}
	empty, err := IdentifySourceContent(nil).MarshalText()
	if err != nil || string(empty) != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatal(string(empty), err)
	}
}

// TestSourceContentMetadataFailsClosed distinguishes unavailable metadata from
// empty compiled source and retains existing identity after a malformed read.
// Rules: rules/compiler/debug_information.md — §55(1)-(3).
func TestSourceContentMetadataFailsClosed(t *testing.T) {
	var unknown SourceContentIdentity
	if match, err := unknown.Matches(nil); err == nil || match {
		t.Fatal(match, err)
	}
	if _, err := unknown.MarshalText(); err == nil {
		t.Fatal("unknown identity serialized")
	}
	expected := IdentifySourceContent([]byte("compiled"))
	for _, invalid := range []string{"", "sha1:123", "sha256:123", "sha256:" + string(make([]byte, 64))} {
		identity := expected
		if err := identity.UnmarshalText([]byte(invalid)); err == nil || identity != expected {
			t.Fatal(invalid, identity, err)
		}
	}
}
