package formatter

import "testing"

// MD-029 error patterns keep their qualified or contextual spelling, payload
// and field braces; an empty field pattern is written `{}` and formatting is
// idempotent (M029-15).
// Rules: rules/tooling/formatter.md — § 20(11).
func TestFormatErrorPatterns(t *testing.T) {
	input := `module main

fn Use(flag: bool) int {
    match Open(flag) {
        Ok(value) => return value
        Err(FileError.NotFound { Code: code,Path: ref path }) => return code
        Err(.NotFound { }) => return 1
        Err(.InvalidEncoding( position )) => return 2
        Err(.PermissionDenied) => return 3
    }
}
`
	want := `module main

fn Use(flag: bool) int {
    match Open(flag) {
        Ok(value) => return value
        Err(FileError.NotFound { Code: code, Path: ref path }) => return code
        Err(.NotFound {}) => return 1
        Err(.InvalidEncoding(position)) => return 2
        Err(.PermissionDenied) => return 3
    }
}
`
	got := Format(Source{Text: input}, Options{}).Text
	if got != want {
		t.Fatalf("formatted:\n%s\nwant:\n%s", got, want)
	}
	if again := Format(Source{Text: got}, Options{}).Text; again != got {
		t.Fatalf("not idempotent:\n%s", again)
	}
}
